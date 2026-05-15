package outbound

import (
	"fmt"
	"net"
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
	if u.Scheme != "https" {
		if !(opts.AllowLoopbackHTTP && u.Scheme == "http" && isLoopbackHost(u.Hostname())) {
			return "", fmt.Errorf("https is required")
		}
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = ""
	u.RawPath = ""
	u.ForceQuery = false
	return u.String(), nil
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
