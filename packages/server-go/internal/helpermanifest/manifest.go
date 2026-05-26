// Package helpermanifest defines the canonical server-side helper-policy
// manifest body that scopes what helper jobs may touch on the helper
// host. Lives in its own package because both `store` (which stamps each
// helper_jobs row's manifest_digest + binding) and `api` (which signs the
// body + injects it into leased-job WS frames) consume it, and neither
// should depend on the other.
//
// JSON tag bytes here MUST stay byte-identical to
// packages/borgee/internal/jobpolicy.PolicyManifest — the daemon
// recomputes canonical bytes for signature verification and any tag
// drift produces ReasonManifestInvalid silently. Tests in api/
// (TestBuildCanonicalHelperManifest_*) lock the shape; the helper-side
// fixture builder (jobpolicy/policy_test.go::signedManifest) is the
// other end of the same byte-contract.
//
// Why fixed IssuedAt / ExpiresAt: the canonical digest must stay stable
// across server reboots so helper_jobs rows persisted before a restart
// remain dischargeable after. Pinning IssuedAt to epoch + ExpiresAt to
// 2099 also sidesteps clock-skew false-negatives on first-boot helpers
// whose system clock has not yet synced.
//
// PR-4 final amend: per-platform manifest variants. Linux + Darwin
// share the same PathID / ServiceID symbols but declare platform-
// specific filesystem roots + service managers. The daemon-side
// jobpolicy resolves binding PathIDs against the manifest's declared
// roots, so the platform-correct manifest must reach the daemon. The
// daemon's WS upgrade sends X-Helper-Platform; the server picks the
// matching builder per session.
package helpermanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// PolicyManifest mirrors jobpolicy.PolicyManifest JSON shape exactly.
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

// Platform identifies the helper deployment target. The canonical
// manifest declares platform-specific filesystem roots + service
// managers. Daemon sends its runtime.GOOS in the X-Helper-Platform WS
// upgrade header; server picks the matching builder.
type Platform string

const (
	PlatformLinux  Platform = "linux"
	PlatformDarwin Platform = "darwin"
)

// String returns the platform token used in headers / cache keys / JSON.
func (p Platform) String() string { return string(p) }

// ParsePlatform validates the daemon-supplied platform token. Empty or
// unknown tokens return ok=false — the WS upgrade handler closes the
// connection on this.
func ParsePlatform(s string) (Platform, bool) {
	switch s {
	case string(PlatformLinux):
		return PlatformLinux, true
	case string(PlatformDarwin):
		return PlatformDarwin, true
	default:
		return "", false
	}
}

// Canonical Path / Service IDs — string consts the store package's
// binding emitter + the daemon-side executors agree on. Both ends are
// locked against drift by TestBuildLinux_DeclaresAllRequiredIDs in api.
const (
	PathIDOpenClawInstall     = "openclaw_install"
	PathIDOpenClawAgentConfig = "openclaw_agent_config"
	PathIDBorgeePluginConfig  = "borgee_plugin_config"
	PathIDBorgeeStateConfig   = "borgee_state_config"
	PathIDHelperState         = "helper_state"
	PathIDHelperRuntime       = "helper_runtime"

	ServiceIDOpenClawUser = "openclaw-user"
	ServiceIDBorgeeHelper = "borgee-helper-service"

	ArtifactIDOpenClawPlugin = "openclaw-plugin"

	DomainCDN = "https://cdn.borgee.io"
)

var (
	epoch   = time.Unix(0, 0).UTC()
	horizon = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
)

// BuildLinux returns the canonical Linux helper manifest. Deterministic:
// repeated calls produce byte-identical output (sorted slices, fixed
// timestamps) so the digest is stable.
//
// #1050 blocker #3 dev-stack override: if BORGEE_DEV_MANIFEST_ORIGIN_BASE
// is set (e.g. "http://borgee-server:4900/dev-artifacts"), the openclaw
// artifact's Origin + the Domains list are rewritten to that base URL
// in place of the production https://cdn.borgee.io placeholder. If
// BORGEE_DEV_MANIFEST_SHA256_OVERRIDE is set to a JSON object mapping
// artifact_id → sha256 hex, those values replace the zero-sha
// placeholder. Production runs leave both env vars unset and the
// manifest body is unchanged. The override is read at every BuildLinux
// call (and LinuxDigest is computed from that snapshot at init time);
// dev-stack must set the env vars BEFORE the server boots so the
// digest is consistent across the run.
func BuildLinux() PolicyManifest {
	origin, domains := devOriginAndDomains()
	sha := devSHA256For(ArtifactIDOpenClawPlugin, "0000000000000000000000000000000000000000000000000000000000000000")
	return PolicyManifest{
		ManifestVersion: 1,
		IssuedAt:        epoch,
		ExpiresAt:       horizon,
		Artifacts: []ArtifactDeclaration{
			{
				ID:       ArtifactIDOpenClawPlugin,
				Platform: "linux-x64",
				Version:  "1.0.0",
				// Placeholder SHA256 — release-helper.yml will flip this
				// to the real artifact digest in a follow-up. Daemon-side
				// validateArtifacts checks SHA256 against the cached
				// bytes only when binding.ArtifactIDs is non-empty, so
				// placeholder is safe for the install-from-manifest path
				// that never lands without a real release pipeline.
				SHA256: sha,
				Origin: origin,
			},
		},
		Paths: []PathDeclaration{
			{ID: PathIDOpenClawInstall, Root: "/usr/local/lib/borgee/openclaw", Mode: "write_install"},
			{ID: PathIDOpenClawAgentConfig, Root: "/var/lib/borgee/openclaw", Mode: "write_config"},
			{ID: PathIDBorgeePluginConfig, Root: "/var/lib/borgee/plugins", Mode: "write_config"},
			{ID: PathIDBorgeeStateConfig, Root: "/var/lib/borgee/state", Mode: "write_config"},
			{ID: PathIDHelperState, Root: "/var/lib/borgee", Mode: "write_state"},
			{ID: PathIDHelperRuntime, Root: "/usr/local/lib/borgee", Mode: "write_runtime"},
		},
		Domains: domains,
		Services: []ServiceDeclaration{
			{ID: ServiceIDOpenClawUser, Platform: "linux", Manager: "systemd", Unit: "openclaw.service"},
			{ID: ServiceIDBorgeeHelper, Platform: "linux", Manager: "systemd", Unit: "borgee.service"},
		},
	}
}

// BuildDarwin returns the canonical macOS helper manifest. Path roots
// match the constants in packages/borgee/internal/cli/setup/setup.go
// (darwinAppSupport / darwinRuntimeDir family); service IDs reuse the
// Linux symbols so the store's binding emitter stays platform-agnostic.
// Service Manager / Unit switch to launchd labels.
//
// Note on PathIDHelperState: setup.go declares `darwinStateRoot =
// /Library/Application Support/Borgee/Helper` (with /Helper suffix) for
// the queue/status/audit handoff subdirs, but the manifest declares the
// parent `/Library/Application Support/Borgee` so writes to
// /Library/Application Support/Borgee/Helper/* are descendants of an
// allowed root. This mirrors how PathIDHelperRuntime covers both
// /usr/local/libexec/borgee/openclaw (openclaw_install) and the binary
// itself.
func BuildDarwin() PolicyManifest {
	origin, domains := devOriginAndDomains()
	sha := devSHA256For(ArtifactIDOpenClawPlugin, "0000000000000000000000000000000000000000000000000000000000000000")
	return PolicyManifest{
		ManifestVersion: 1,
		IssuedAt:        epoch,
		ExpiresAt:       horizon,
		Artifacts: []ArtifactDeclaration{
			{
				ID:       ArtifactIDOpenClawPlugin,
				Platform: "darwin-arm64",
				Version:  "1.0.0",
				SHA256:   sha,
				Origin:   origin,
			},
		},
		Paths: []PathDeclaration{
			{ID: PathIDOpenClawInstall, Root: "/usr/local/libexec/borgee/openclaw", Mode: "write_install"},
			{ID: PathIDOpenClawAgentConfig, Root: "/Library/Application Support/Borgee/openclaw", Mode: "write_config"},
			{ID: PathIDBorgeePluginConfig, Root: "/Library/Application Support/Borgee/plugins", Mode: "write_config"},
			{ID: PathIDBorgeeStateConfig, Root: "/Library/Application Support/Borgee/state", Mode: "write_config"},
			{ID: PathIDHelperState, Root: "/Library/Application Support/Borgee", Mode: "write_state"},
			{ID: PathIDHelperRuntime, Root: "/usr/local/libexec/borgee", Mode: "write_runtime"},
		},
		Domains: domains,
		Services: []ServiceDeclaration{
			{ID: ServiceIDOpenClawUser, Platform: "darwin", Manager: "launchd", Unit: "cloud.borgee.openclaw"},
			{ID: ServiceIDBorgeeHelper, Platform: "darwin", Manager: "launchd", Unit: "cloud.borgee.host-bridge"},
		},
	}
}

// devOriginAndDomains reads BORGEE_DEV_MANIFEST_ORIGIN_BASE and, when
// non-empty, returns it as the openclaw artifact Origin + the single
// Domains entry in place of the production https://cdn.borgee.io
// placeholder. Production runs leave the env unset and the manifest
// continues to declare cdn.borgee.io. The override is the only thing
// that lets the dev-stack `borgee-vm` reach a reachable artifact
// endpoint (the public CDN is not provisioned yet — #1003 follow-up).
// See scripts/dev-stack/.env.example for the configured value.
func devOriginAndDomains() (string, []string) {
	if base := strings.TrimSpace(os.Getenv("BORGEE_DEV_MANIFEST_ORIGIN_BASE")); base != "" {
		return base, []string{base}
	}
	return DomainCDN, []string{DomainCDN}
}

// devSHA256For honors BORGEE_DEV_MANIFEST_SHA256_OVERRIDE, a JSON map
// of artifact_id → sha256 hex string. Missing keys or empty / invalid
// JSON falls back to the supplied placeholder. The override lets the
// dev-stack stamp the real sha256 of a sentinel artifact dropped into
// the server container at build time (#1050 blocker #3); production
// runs leave the env unset and the canonical placeholder is preserved.
func devSHA256For(artifactID, fallback string) string {
	raw := strings.TrimSpace(os.Getenv("BORGEE_DEV_MANIFEST_SHA256_OVERRIDE"))
	if raw == "" {
		return fallback
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return fallback
	}
	if sha, ok := m[artifactID]; ok && strings.TrimSpace(sha) != "" {
		return strings.TrimSpace(sha)
	}
	return fallback
}

// CanonicalManifest returns the canonical manifest body for the given
// platform. PR-4 final amend public API surface; replaces single
// BuildLinux for callers that need platform-awareness.
func CanonicalManifest(p Platform) (PolicyManifest, error) {
	switch p {
	case PlatformLinux:
		return BuildLinux(), nil
	case PlatformDarwin:
		return BuildDarwin(), nil
	default:
		return PolicyManifest{}, fmt.Errorf("helpermanifest: unsupported platform %q", string(p))
	}
}

// CanonicalBytes returns the deterministic JSON bytes (Signature stripped)
// — input to both signature production AND digest. Daemon side
// recomputes via jobpolicy.CanonicalManifestBytes which strips Signature
// identically. Marshal field order = struct field order = canonical.
func CanonicalBytes(m PolicyManifest) ([]byte, error) {
	m.Signature = ""
	return json.Marshal(m)
}

// Digest returns the canonical-bytes sha256 digest in "sha256:<hex>"
// form. Matches jobpolicy.digestBytes (it prefixes "sha256:" too).
func Digest(m PolicyManifest) (string, error) {
	canonical, err := CanonicalBytes(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// CanonicalDigest returns the canonical digest for the given platform's
// manifest. Memoized via the package-level LinuxDigest / DarwinDigest
// vars so the hot path (every helper-job enqueue) pays sha256 once at
// startup per platform.
func CanonicalDigest(p Platform) (string, error) {
	switch p {
	case PlatformLinux:
		return LinuxDigest, nil
	case PlatformDarwin:
		return DarwinDigest, nil
	default:
		return "", fmt.Errorf("helpermanifest: unsupported platform %q", string(p))
	}
}

// LinuxDigest returns the canonical digest of the v1 Linux manifest.
// Memoized as a package-level value so callers in the hot path (every
// helper job enqueue) pay sha256 once at startup.
var LinuxDigest = func() string {
	d, _ := Digest(BuildLinux())
	return d
}()

// DarwinDigest mirrors LinuxDigest for the macOS manifest. Distinct
// value (different paths + service Manager) — daemon's manifest-vs-
// binding digest check rejects cross-platform delivery silently.
var DarwinDigest = func() string {
	d, _ := Digest(BuildDarwin())
	return d
}()
