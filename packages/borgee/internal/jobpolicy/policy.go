package jobpolicy

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type Reason string

const (
	ReasonOK              Reason = "ok"
	ReasonSchemaInvalid   Reason = "schema_invalid"
	ReasonUnknownJobType  Reason = "unknown_job_type"
	ReasonManifestInvalid Reason = "manifest_invalid"
	ReasonArtifactInvalid Reason = "artifact_invalid"
	ReasonPathDenied      Reason = "path_denied"
	ReasonDomainDenied    Reason = "domain_denied"
	ReasonServiceDenied   Reason = "service_denied"
	ReasonRevoked         Reason = "revoked"
	ReasonStaleCredential Reason = "stale_credential"
	ReasonWrongOwner      Reason = "wrong_owner"
	ReasonWrongOrg        Reason = "wrong_org"
	ReasonPolicyDenied    Reason = "policy_denied"
)

const (
	JobTypeOpenClawConfigureAgent      = "openclaw.configure_agent"
	JobTypeOpenClawInstallFromManifest = "openclaw.install_from_manifest"
	JobTypePluginConfigureConnection   = "borgee_plugin.configure_connection"
	JobTypeServiceLifecycle            = "service.lifecycle"
	JobTypeStateWrite                  = "state.write"
	JobTypeStatusCollect               = "status.collect"
	JobTypeDelegationRevoke            = "delegation.revoke"
	JobTypeHelperUninstall             = "helper.uninstall"

	CategoryOpenClaw          = "openclaw_config"
	CategoryOpenClawLifecycle = "openclaw_lifecycle"
	CategoryServiceLifecycle  = "service.lifecycle"
	CategoryHelperLifecycle   = "helper.lifecycle"
)

type Decision struct {
	Allow  bool
	Reason Reason
}

type EvaluationInput struct {
	Now        time.Time
	TrustRoots []ed25519.PublicKey
	Platform   string
	Job        Job
	Enrollment EnrollmentState
	Sandbox    SandboxProfile
	Artifacts  map[string][]byte
}

type Job struct {
	JobID                string
	OwnerUserID          string
	OrgID                string
	EnrollmentID         string
	HelperDeviceID       string
	CredentialGeneration int
	JobType              string
	Category             string
	SchemaVersion        int
	PayloadJSON          []byte
	PayloadHash          string
	ManifestDigest       string
	ManifestJSON         []byte
	ManifestBindingJSON  []byte
	ExpiresAt            time.Time
}

type EnrollmentState struct {
	OwnerUserID          string
	OrgID                string
	EnrollmentID         string
	HelperDeviceID       string
	CredentialGeneration int
	Status               string
	Revoked              bool
	Uninstalled          bool
	StaleCredential      bool
	AllowedCategories    []string
}

type SandboxProfile struct {
	ReadRoots      []string
	WriteRoots     []string
	AllowedOrigins []string
	ServiceIDs     []string
}

type PolicyManifest struct {
	ManifestVersion int                   `json:"manifest_version"`
	IssuedAt        time.Time             `json:"issued_at"`
	ExpiresAt       time.Time             `json:"expires_at"`
	Artifacts       []ArtifactDeclaration `json:"artifacts"`
	Paths           []PathDeclaration     `json:"paths"`
	Domains         []string              `json:"domains"`
	Services        []ServiceDeclaration  `json:"services"`
	Signature       string                `json:"signature,omitempty"`
}

type ArtifactDeclaration struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
	SHA256   string `json:"sha256"`
	Origin   string `json:"origin"`
	Size     int64  `json:"size,omitempty"`
}

type PathDeclaration struct {
	ID   string `json:"id"`
	Root string `json:"root"`
	Mode string `json:"mode"`
}

type ServiceDeclaration struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Manager  string `json:"manager"`
	Unit     string `json:"unit"`
}

type ManifestBinding struct {
	ManifestDigest string   `json:"manifest_digest"`
	ArtifactIDs    []string `json:"artifact_ids,omitempty"`
	PathIDs        []string `json:"path_ids,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	ServiceIDs     []string `json:"service_ids,omitempty"`
}

type manifestAuthority struct {
	manifest PolicyManifest
	binding  ManifestBinding
}

func Evaluate(input EvaluationInput) Decision {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	if reason := validateJobSchema(input.Job); reason != ReasonOK {
		return deny(reason)
	}
	if reason := validatePayloadHash(input.Job); reason != ReasonOK {
		return deny(reason)
	}
	if reason := validatePayload(input.Job); reason != ReasonOK {
		return deny(reason)
	}
	if reason := validateLocalState(input.Job, input.Enrollment, now); reason != ReasonOK {
		return deny(reason)
	}

	if !requiresManifest(input.Job.JobType) {
		return allow()
	}

	authority, reason := verifyManifestAuthority(input, now)
	if reason != ReasonOK {
		return deny(reason)
	}
	if reason := validateManifestRequirements(input.Job.JobType, authority.binding); reason != ReasonOK {
		return deny(reason)
	}
	if reason := validateArtifacts(authority, input.Artifacts, input.Platform); reason != ReasonOK {
		return deny(reason)
	}
	if reason := validatePaths(input.Job.JobType, authority, input.Sandbox); reason != ReasonOK {
		return deny(reason)
	}
	if reason := validateDomains(authority, input.Sandbox); reason != ReasonOK {
		return deny(reason)
	}
	if reason := validateServices(authority, input.Sandbox, input.Platform); reason != ReasonOK {
		return deny(reason)
	}

	return allow()
}

func CanonicalManifestBytes(m PolicyManifest) ([]byte, error) {
	m.Signature = ""
	return json.Marshal(m)
}

func allow() Decision { return Decision{Allow: true, Reason: ReasonOK} }

func deny(reason Reason) Decision { return Decision{Allow: false, Reason: reason} }

func validateJobSchema(job Job) Reason {
	if strings.TrimSpace(job.JobID) == "" || strings.TrimSpace(job.OwnerUserID) == "" || strings.TrimSpace(job.OrgID) == "" ||
		strings.TrimSpace(job.EnrollmentID) == "" || strings.TrimSpace(job.HelperDeviceID) == "" || strings.TrimSpace(job.Category) == "" ||
		job.SchemaVersion == 0 || len(job.PayloadJSON) == 0 || strings.TrimSpace(job.PayloadHash) == "" || job.ExpiresAt.IsZero() {
		return ReasonSchemaInvalid
	}
	if !knownJobType(job.JobType) {
		return ReasonUnknownJobType
	}
	if job.SchemaVersion != 1 {
		return ReasonSchemaInvalid
	}
	return ReasonOK
}

func validatePayloadHash(job Job) Reason {
	if digestBytes(job.PayloadJSON) != job.PayloadHash {
		return ReasonSchemaInvalid
	}
	return ReasonOK
}

func knownJobType(jobType string) bool {
	switch jobType {
	case JobTypeOpenClawConfigureAgent, JobTypeOpenClawInstallFromManifest, JobTypePluginConfigureConnection,
		JobTypeServiceLifecycle, JobTypeStateWrite, JobTypeStatusCollect, JobTypeDelegationRevoke, JobTypeHelperUninstall:
		return true
	default:
		return false
	}
}

func validatePayload(job Job) Reason {
	switch job.JobType {
	case JobTypeOpenClawConfigureAgent:
		var payload struct {
			AgentID             string `json:"agent_id"`
			ChannelID           string `json:"channel_id,omitempty"`
			ConfigSchemaVersion int64  `json:"config_schema_version"`
			ConfigHash          string `json:"config_hash"`
		}
		if err := decodeStrict(job.PayloadJSON, &payload); err != nil || payload.AgentID == "" || payload.ConfigSchemaVersion <= 0 || !strings.HasPrefix(payload.ConfigHash, "sha256:") || strings.TrimSpace(payload.ConfigHash) != payload.ConfigHash {
			return ReasonSchemaInvalid
		}
	case JobTypeOpenClawInstallFromManifest:
		var payload struct {
			InstallPlanID string `json:"install_plan_id"`
		}
		if err := decodeStrict(job.PayloadJSON, &payload); err != nil || payload.InstallPlanID == "" {
			return ReasonSchemaInvalid
		}
	case JobTypePluginConfigureConnection:
		var payload struct {
			ConnectionID string `json:"connection_id"`
			AgentID      string `json:"agent_id"`
			ChannelID    string `json:"channel_id"`
		}
		if err := decodeStrict(job.PayloadJSON, &payload); err != nil || !strings.HasPrefix(payload.ConnectionID, "borgee-plugin:") || payload.AgentID == "" || payload.ChannelID == "" {
			return ReasonSchemaInvalid
		}
	case JobTypeServiceLifecycle:
		var payload struct {
			Operation string `json:"operation"`
		}
		if err := decodeStrict(job.PayloadJSON, &payload); err != nil || !allowedServiceOperation(payload.Operation) {
			return ReasonSchemaInvalid
		}
	case JobTypeStateWrite:
		var payload struct {
			StateKey string `json:"state_key"`
			ValueSHA string `json:"value_sha256,omitempty"`
		}
		if err := decodeStrict(job.PayloadJSON, &payload); err != nil || payload.StateKey == "" {
			return ReasonSchemaInvalid
		}
	case JobTypeStatusCollect:
		var payload struct {
			Scope string `json:"scope"`
		}
		if err := decodeStrict(job.PayloadJSON, &payload); err != nil || !allowedStatusScope(payload.Scope) {
			return ReasonSchemaInvalid
		}
	case JobTypeDelegationRevoke:
		var payload struct {
			TargetCategory string `json:"target_category"`
		}
		if err := decodeStrict(job.PayloadJSON, &payload); err != nil || payload.TargetCategory == "" {
			return ReasonSchemaInvalid
		}
	case JobTypeHelperUninstall:
		var payload struct {
			Scope         string `json:"scope"`
			PreserveState bool   `json:"preserve_state,omitempty"`
		}
		if err := decodeStrict(job.PayloadJSON, &payload); err != nil || payload.Scope != "helper" {
			return ReasonSchemaInvalid
		}
	}
	return ReasonOK
}

func decodeStrict(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("extra json value")
	}
	return nil
}

func allowedServiceOperation(op string) bool {
	switch op {
	case "start", "stop", "restart", "reload", "enable", "disable":
		return true
	default:
		return false
	}
}

func allowedStatusScope(scope string) bool {
	switch scope {
	case "helper", "openclaw", "service":
		return true
	default:
		return false
	}
}

func validateLocalState(job Job, enrollment EnrollmentState, now time.Time) Reason {
	if job.OwnerUserID != enrollment.OwnerUserID {
		return ReasonWrongOwner
	}
	if job.OrgID != enrollment.OrgID {
		return ReasonWrongOrg
	}
	if job.EnrollmentID != enrollment.EnrollmentID || job.HelperDeviceID != enrollment.HelperDeviceID {
		return ReasonPolicyDenied
	}
	if enrollment.Revoked || enrollment.Uninstalled {
		return ReasonRevoked
	}
	if enrollment.StaleCredential || job.CredentialGeneration != enrollment.CredentialGeneration {
		return ReasonStaleCredential
	}
	if enrollment.Status != "active" {
		return ReasonPolicyDenied
	}
	if !contains(enrollment.AllowedCategories, job.Category) {
		return ReasonPolicyDenied
	}
	if !now.Before(job.ExpiresAt) {
		return ReasonPolicyDenied
	}
	return ReasonOK
}

func requiresManifest(jobType string) bool {
	switch jobType {
	case JobTypeOpenClawConfigureAgent, JobTypeOpenClawInstallFromManifest, JobTypePluginConfigureConnection, JobTypeServiceLifecycle, JobTypeStateWrite, JobTypeHelperUninstall:
		return true
	default:
		return false
	}
}

func verifyManifestAuthority(input EvaluationInput, now time.Time) (manifestAuthority, Reason) {
	if len(input.Job.ManifestJSON) == 0 || strings.TrimSpace(input.Job.ManifestDigest) == "" || len(input.Job.ManifestBindingJSON) == 0 {
		return manifestAuthority{}, ReasonManifestInvalid
	}
	if len(input.TrustRoots) == 0 {
		return manifestAuthority{}, ReasonManifestInvalid
	}

	var manifest PolicyManifest
	if err := decodeStrict(input.Job.ManifestJSON, &manifest); err != nil {
		return manifestAuthority{}, ReasonManifestInvalid
	}
	if manifest.ManifestVersion != 1 || manifest.Signature == "" {
		return manifestAuthority{}, ReasonManifestInvalid
	}
	if manifest.IssuedAt.IsZero() || manifest.ExpiresAt.IsZero() || manifest.ExpiresAt.Before(manifest.IssuedAt) {
		return manifestAuthority{}, ReasonManifestInvalid
	}
	const skew = 5 * time.Minute
	if now.Before(manifest.IssuedAt.Add(-skew)) || now.After(manifest.ExpiresAt.Add(skew)) {
		return manifestAuthority{}, ReasonManifestInvalid
	}

	canonical, err := CanonicalManifestBytes(manifest)
	if err != nil {
		return manifestAuthority{}, ReasonManifestInvalid
	}
	if digestBytes(canonical) != input.Job.ManifestDigest {
		return manifestAuthority{}, ReasonManifestInvalid
	}
	sig, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return manifestAuthority{}, ReasonManifestInvalid
	}
	verified := false
	for _, root := range input.TrustRoots {
		if len(root) == ed25519.PublicKeySize && ed25519.Verify(root, canonical, sig) {
			verified = true
			break
		}
	}
	if !verified {
		return manifestAuthority{}, ReasonManifestInvalid
	}

	var binding ManifestBinding
	if err := decodeStrict(input.Job.ManifestBindingJSON, &binding); err != nil {
		return manifestAuthority{}, ReasonManifestInvalid
	}
	if binding.ManifestDigest == "" || binding.ManifestDigest != input.Job.ManifestDigest {
		return manifestAuthority{}, ReasonManifestInvalid
	}
	return manifestAuthority{manifest: manifest, binding: binding}, ReasonOK
}

func validateManifestRequirements(jobType string, binding ManifestBinding) Reason {
	switch jobType {
	case JobTypeOpenClawConfigureAgent:
		if len(binding.PathIDs) == 0 {
			return ReasonPathDenied
		}
	case JobTypeOpenClawInstallFromManifest:
		if len(binding.ArtifactIDs) == 0 {
			return ReasonArtifactInvalid
		}
		if len(binding.PathIDs) == 0 {
			return ReasonPathDenied
		}
		if len(binding.Domains) == 0 {
			return ReasonDomainDenied
		}
	case JobTypePluginConfigureConnection, JobTypeStateWrite:
		if len(binding.PathIDs) == 0 {
			return ReasonPathDenied
		}
	case JobTypeServiceLifecycle:
		if len(binding.ServiceIDs) == 0 {
			return ReasonServiceDenied
		}
	case JobTypeHelperUninstall:
		if len(binding.PathIDs) == 0 && len(binding.ServiceIDs) == 0 {
			return ReasonPolicyDenied
		}
	}
	return ReasonOK
}

func validateArtifacts(authority manifestAuthority, cache map[string][]byte, platform string) Reason {
	byID := make(map[string]ArtifactDeclaration, len(authority.manifest.Artifacts))
	for _, artifact := range authority.manifest.Artifacts {
		if artifact.ID == "" || artifact.SHA256 == "" || artifact.Origin == "" {
			return ReasonArtifactInvalid
		}
		byID[artifact.ID] = artifact
	}
	boundOrigins := make(map[string]struct{}, len(authority.binding.Domains))
	for _, domain := range authority.binding.Domains {
		normalized, err := normalizeOrigin(domain)
		if err != nil {
			return ReasonDomainDenied
		}
		boundOrigins[normalized] = struct{}{}
	}
	for _, id := range authority.binding.ArtifactIDs {
		artifact, ok := byID[id]
		if !ok {
			return ReasonArtifactInvalid
		}
		if artifact.Platform != "" && platform != "" && artifact.Platform != platform {
			return ReasonArtifactInvalid
		}
		origin, err := normalizeOrigin(artifact.Origin)
		if err != nil {
			return ReasonDomainDenied
		}
		if _, ok := boundOrigins[origin]; !ok {
			return ReasonDomainDenied
		}
		bytes, ok := cache[id]
		if !ok {
			return ReasonArtifactInvalid
		}
		if digestBytes(bytes) != artifact.SHA256 {
			return ReasonArtifactInvalid
		}
	}
	return ReasonOK
}

func validatePaths(jobType string, authority manifestAuthority, sandbox SandboxProfile) Reason {
	byID := make(map[string]PathDeclaration, len(authority.manifest.Paths))
	for _, path := range authority.manifest.Paths {
		if path.ID == "" {
			return ReasonPathDenied
		}
		byID[path.ID] = path
	}
	for _, id := range authority.binding.PathIDs {
		decl, ok := byID[id]
		if !ok {
			return ReasonPathDenied
		}
		root, ok := normalizePolicyPath(decl.Root)
		if !ok || decl.Mode == "" {
			return ReasonPathDenied
		}
		if jobRequiresWritePath(jobType) && !pathModeAllowsWrite(decl.Mode) {
			return ReasonPolicyDenied
		}
		if !sandboxHasPath(root, sandbox, decl.Mode) {
			return ReasonPolicyDenied
		}
	}
	return ReasonOK
}

func jobRequiresWritePath(jobType string) bool {
	switch jobType {
	case JobTypeOpenClawConfigureAgent, JobTypeOpenClawInstallFromManifest, JobTypePluginConfigureConnection, JobTypeStateWrite:
		return true
	default:
		return false
	}
}

func pathModeAllowsWrite(mode string) bool {
	return strings.HasPrefix(mode, "write") || strings.Contains(mode, "write")
}

func normalizePolicyPath(raw string) (string, bool) {
	if strings.ContainsRune(raw, '\x00') || !filepath.IsAbs(raw) || hasDotDotSegment(raw) {
		return "", false
	}
	cleaned := filepath.Clean(raw)
	if cleaned == string(filepath.Separator) || strings.ContainsRune(cleaned, '\x00') || !filepath.IsAbs(cleaned) || hasDotDotSegment(cleaned) {
		return "", false
	}
	return cleaned, true
}

func hasDotDotSegment(path string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if part == ".." {
			return true
		}
	}
	return false
}

func sandboxHasPath(path string, sandbox SandboxProfile, mode string) bool {
	roots := sandbox.ReadRoots
	if pathModeAllowsWrite(mode) {
		roots = sandbox.WriteRoots
	}
	for _, root := range roots {
		cleaned, ok := normalizePolicyPath(root)
		if ok && pathWithin(path, cleaned) {
			return true
		}
	}
	return false
}

func pathWithin(path, root string) bool {
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

func validateDomains(authority manifestAuthority, sandbox SandboxProfile) Reason {
	manifestDomains := make(map[string]struct{}, len(authority.manifest.Domains))
	for _, domain := range authority.manifest.Domains {
		normalized, err := normalizeOrigin(domain)
		if err != nil {
			return ReasonDomainDenied
		}
		manifestDomains[normalized] = struct{}{}
	}
	sandboxOrigins := make(map[string]struct{}, len(sandbox.AllowedOrigins))
	for _, origin := range sandbox.AllowedOrigins {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return ReasonPolicyDenied
		}
		sandboxOrigins[normalized] = struct{}{}
	}
	for _, domain := range authority.binding.Domains {
		normalized, err := normalizeOrigin(domain)
		if err != nil {
			return ReasonDomainDenied
		}
		if _, ok := manifestDomains[normalized]; !ok {
			return ReasonDomainDenied
		}
		if _, ok := sandboxOrigins[normalized]; !ok {
			return ReasonPolicyDenied
		}
	}
	return ReasonOK
}

func normalizeOrigin(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("scheme and host are required")
	}
	if u.User != nil {
		return "", fmt.Errorf("userinfo is not allowed")
	}
	if u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("origin must not include path, query, or fragment")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "https" {
		return "", fmt.Errorf("https is required")
	}
	host := canonicalOriginHost(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return "", fmt.Errorf("local origins are not allowed")
	}
	if addr, ok := parseOriginHostAddr(host); ok {
		if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() {
			return "", fmt.Errorf("local/private origins are not allowed")
		}
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = ""
	u.RawPath = ""
	u.ForceQuery = false
	return u.String(), nil
}

func canonicalOriginHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func parseOriginHostAddr(host string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func validateServices(authority manifestAuthority, sandbox SandboxProfile, platform string) Reason {
	byID := make(map[string]ServiceDeclaration, len(authority.manifest.Services))
	for _, service := range authority.manifest.Services {
		if !validLogicalServiceID(service.ID) || !validServiceDeclaration(service, platform) {
			return ReasonServiceDenied
		}
		if _, exists := byID[service.ID]; exists {
			return ReasonServiceDenied
		}
		byID[service.ID] = service
	}
	seenBinding := map[string]struct{}{}
	for _, id := range authority.binding.ServiceIDs {
		if !validLogicalServiceID(id) {
			return ReasonServiceDenied
		}
		if _, exists := seenBinding[id]; exists {
			return ReasonServiceDenied
		}
		seenBinding[id] = struct{}{}
		service, ok := byID[id]
		if !ok {
			return ReasonServiceDenied
		}
		if service.Platform != "" && platform != "" && !platformMatches(platform, service.Platform) {
			return ReasonServiceDenied
		}
		if !contains(sandbox.ServiceIDs, id) {
			return ReasonPolicyDenied
		}
	}
	return ReasonOK
}

func validLogicalServiceID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 96 || id == "." || id == ".." || strings.Contains(id, "..") {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func validServiceDeclaration(service ServiceDeclaration, platform string) bool {
	manager := strings.TrimSpace(service.Manager)
	unit := strings.TrimSpace(service.Unit)
	if service.Manager != manager || service.Unit != unit || unit == "" || strings.ContainsAny(unit, "/\\\x00\t\n\r ") {
		return false
	}
	declaredPlatform := strings.TrimSpace(service.Platform)
	effectivePlatform := platform
	if effectivePlatform == "" {
		effectivePlatform = declaredPlatform
	}
	switch {
	case strings.HasPrefix(effectivePlatform, "linux"):
		return manager == "systemd" && validSystemdServiceUnit(unit)
	case strings.HasPrefix(effectivePlatform, "darwin"):
		return manager == "launchd" && validLaunchdServiceLabel(unit)
	default:
		return (manager == "systemd" && validSystemdServiceUnit(unit)) || (manager == "launchd" && validLaunchdServiceLabel(unit))
	}
}

func validSystemdServiceUnit(unit string) bool {
	if !strings.HasSuffix(unit, ".service") || strings.HasPrefix(unit, ".") || strings.Contains(unit, "..") || strings.Contains(unit, "@") {
		return false
	}
	for _, r := range unit {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func validLaunchdServiceLabel(label string) bool {
	if strings.HasSuffix(label, ".service") || strings.HasSuffix(label, ".plist") || strings.HasPrefix(label, ".") || strings.HasSuffix(label, ".") || !strings.Contains(label, ".") || strings.Contains(label, "..") {
		return false
	}
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func platformMatches(input, declared string) bool {
	return input == declared || strings.HasPrefix(input, declared+"-")
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
