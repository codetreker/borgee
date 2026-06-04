package outbound

import (
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type PrereqConfig struct {
	ServerOrigin    string
	AllowedOrigins  string
	QueueStateDir   string
	StatusStateDir  string
	AuditHandoffDir string
}

type ValidationOptions struct {
	AllowedStateRoots []string
	AllowLoopbackHTTP bool
	// AllowDevContainerWS, when true, permits ws:// (and http://) to
	// non-routable destinations beyond the strict loopback set:
	// RFC1918 private addresses, link-local, the wildcard 0.0.0.0
	// family, and single-label "container DNS" hostnames (no dot, no
	// public TLD shape — e.g. `borgee-server` resolved by the docker
	// embedded DNS). It is OFF by default; daemon.go opts in only
	// when BORGEE_DEV_ALLOW_INSECURE_WS=1 is set. This is a dev-stack
	// escape hatch — production daemons leave it off and the
	// existing https/wss-only rule continues to bind.
	AllowDevContainerWS bool
}

type PreparedConfig struct {
	Enabled         bool
	ServerOrigin    string
	AllowedOrigins  []string
	QueueStateDir   string
	StatusStateDir  string
	AuditHandoffDir string
}

func ValidateAndPrepare(cfg PrereqConfig, opts ValidationOptions) (PreparedConfig, error) {
	if !cfg.configured() {
		return PreparedConfig{}, nil
	}
	if !cfg.complete() {
		return PreparedConfig{}, fmt.Errorf("partial outbound prerequisite config: server origin, allowed origins, queue state dir, status state dir, and audit handoff dir are required together")
	}

	serverOrigin, err := normalizeOrigin(cfg.ServerOrigin, opts)
	if err != nil {
		return PreparedConfig{}, fmt.Errorf("outbound server origin: %w", err)
	}
	allowedOrigins, err := normalizeAllowedOrigins(cfg.AllowedOrigins, opts)
	if err != nil {
		return PreparedConfig{}, err
	}
	if !contains(allowedOrigins, serverOrigin) {
		return PreparedConfig{}, fmt.Errorf("outbound server origin %q is not in allowed origins", serverOrigin)
	}

	allowedRoots := normalizeRoots(opts.AllowedStateRoots)
	if len(allowedRoots) == 0 {
		allowedRoots = DefaultStateRoots()
	}
	queueDir, err := normalizeStateDir("queue state dir", cfg.QueueStateDir, allowedRoots)
	if err != nil {
		return PreparedConfig{}, err
	}
	statusDir, err := normalizeStateDir("status state dir", cfg.StatusStateDir, allowedRoots)
	if err != nil {
		return PreparedConfig{}, err
	}
	auditDir, err := normalizeStateDir("audit handoff dir", cfg.AuditHandoffDir, allowedRoots)
	if err != nil {
		return PreparedConfig{}, err
	}
	for _, dir := range []string{queueDir, statusDir, auditDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return PreparedConfig{}, fmt.Errorf("create outbound state dir %q: %w", dir, err)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return PreparedConfig{}, fmt.Errorf("chmod outbound state dir %q: %w", dir, err)
		}
	}

	return PreparedConfig{
		Enabled:         true,
		ServerOrigin:    serverOrigin,
		AllowedOrigins:  allowedOrigins,
		QueueStateDir:   queueDir,
		StatusStateDir:  statusDir,
		AuditHandoffDir: auditDir,
	}, nil
}

func DefaultStateRoots() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/Library/Application Support/Borgee/Helper"}
	case "linux":
		// Amend gap #5: the internal setup helper (invoked by `borgee
		// install`) provisions state dirs under
		// /var/lib/borgee/{queue,status,audit-handoff,...} and the
		// systemd unit's ExecStart points to the same root. The legacy
		// helper-specific path /var/lib/borgee-helper is no longer
		// created on a fresh install — defaulting to that here caused
		// every state-dir validation to fail with "outside allowed
		// Helper-owned state roots". Keep /var/lib/borgee-helper in the
		// list too so daemons upgraded in-place from older packages
		// (whose existing state dirs may still sit under the old root)
		// continue to validate cleanly.
		return []string{"/var/lib/borgee", "/var/lib/borgee-helper"}
	default:
		return nil
	}
}

func (c PrereqConfig) configured() bool {
	return strings.TrimSpace(c.ServerOrigin) != "" ||
		strings.TrimSpace(c.AllowedOrigins) != "" ||
		strings.TrimSpace(c.QueueStateDir) != "" ||
		strings.TrimSpace(c.StatusStateDir) != "" ||
		strings.TrimSpace(c.AuditHandoffDir) != ""
}

func (c PrereqConfig) complete() bool {
	return strings.TrimSpace(c.ServerOrigin) != "" &&
		strings.TrimSpace(c.AllowedOrigins) != "" &&
		strings.TrimSpace(c.QueueStateDir) != "" &&
		strings.TrimSpace(c.StatusStateDir) != "" &&
		strings.TrimSpace(c.AuditHandoffDir) != ""
}

func normalizeAllowedOrigins(raw string, opts ValidationOptions) ([]string, error) {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		normalized, err := normalizeOrigin(part, opts)
		if err != nil {
			return nil, fmt.Errorf("allowed origin %q: %w", part, err)
		}
		if !contains(out, normalized) {
			out = append(out, normalized)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("outbound allowed origins: at least one origin is required")
	}
	return out, nil
}

func normalizeOrigin(raw string, opts ValidationOptions) (string, error) {
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
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("query and fragment are not allowed")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("path is not allowed")
	}
	scheme := strings.ToLower(u.Scheme)
	// PR-2 #1038: accept wss:// (the production WS transport) alongside
	// the legacy https:// (retained for one-shot claim calls).
	// AllowLoopbackHTTP keeps the ws:// + http:// loopback escape hatch
	// for e2e tests.
	switch scheme {
	case "https", "wss":
		// secure schemes: accept
	case "http", "ws":
		loopbackOK := opts.AllowLoopbackHTTP && isLoopbackHost(u.Hostname())
		// Dev-stack escape hatch (#1050 blocker #1): permit ws:// to
		// docker-network DNS names (no dot, single label) and to
		// non-routable / private addresses when the operator explicitly
		// opts in via BORGEE_DEV_ALLOW_INSECURE_WS=1. Production
		// daemons run with AllowDevContainerWS=false so the strict
		// https/wss-only rule continues to bind.
		devOK := opts.AllowDevContainerWS && isDevPrivateOrContainerHost(u.Hostname())
		if !(loopbackOK || devOK) {
			return "", fmt.Errorf("https/wss is required")
		}
	default:
		return "", fmt.Errorf("scheme %q is not supported", u.Scheme)
	}
	if (scheme == "https" || scheme == "wss") && isLocalOrPrivateHost(u.Hostname()) {
		return "", fmt.Errorf("local/private origins are not allowed")
	}
	u.Scheme = scheme
	u.Host = strings.ToLower(u.Host)
	u.Path = ""
	u.RawPath = ""
	u.ForceQuery = false
	return u.String(), nil
}

func isLoopbackHost(host string) bool {
	host = canonicalOriginHost(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	addr, ok := parseOriginHostAddr(host)
	return ok && addr.IsLoopback()
}

func isLocalOrPrivateHost(host string) bool {
	host = canonicalOriginHost(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	addr, ok := parseOriginHostAddr(host)
	if !ok {
		return false
	}
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified()
}

// isDevPrivateOrContainerHost decides whether `host` looks like a
// non-routable destination acceptable under the dev-stack
// BORGEE_DEV_ALLOW_INSECURE_WS opt-in. Three buckets accepted:
//
//  1. Loopback literals + `*.localhost`.
//  2. RFC1918 / link-local / unspecified IP addresses (parsed via
//     netip), matching the existing isLocalOrPrivateHost set.
//  3. Single-label hostnames (no dot) — docker embedded DNS uses
//     bare service names like `borgee-server`. Public TLDs always
//     carry at least one dot, so this gate cannot accidentally let
//     `example` pass when there is no public `example` TLD record
//     reachable from the daemon (and the operator has already opted
//     in with the env var anyway).
//
// Anything else (public DNS name with at least one dot, public IP)
// is rejected even with the opt-in.
func isDevPrivateOrContainerHost(host string) bool {
	host = canonicalOriginHost(host)
	if host == "" {
		return false
	}
	if isLocalOrPrivateHost(host) {
		return true
	}
	// Hostnames containing a dot may resolve to public DNS — refuse
	// even with the dev opt-in. Single-label tokens are container DNS.
	if !strings.Contains(host, ".") {
		return true
	}
	return false
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

func normalizeRoots(roots []string) []string {
	var out []string
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" || !filepath.IsAbs(root) {
			continue
		}
		out = append(out, filepath.Clean(root))
	}
	return out
}

func normalizeStateDir(label, raw string, allowedRoots []string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("%s must be absolute", label)
	}
	cleaned := filepath.Clean(raw)
	for _, root := range allowedRoots {
		rel, err := filepath.Rel(root, cleaned)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		return cleaned, nil
	}
	return "", fmt.Errorf("%s %q is outside allowed Helper-owned state roots", label, raw)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
