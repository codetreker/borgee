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
		return []string{"/var/lib/borgee-helper"}
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
		if !(opts.AllowLoopbackHTTP && isLoopbackHost(u.Hostname())) {
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
