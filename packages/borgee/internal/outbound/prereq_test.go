package outbound

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrereqValidateDisabledWhenUnconfigured(t *testing.T) {
	got, err := ValidateAndPrepare(PrereqConfig{}, ValidationOptions{})
	if err != nil {
		t.Fatalf("ValidateAndPrepare disabled config: %v", err)
	}
	if got.Enabled {
		t.Fatalf("zero outbound config should stay disabled")
	}
}

func TestPrereqValidateAllowsExactOriginAndCreatesStateDirs(t *testing.T) {
	root := t.TempDir()
	cfg := PrereqConfig{
		ServerOrigin:    "https://app.borgee.io/",
		AllowedOrigins:  "https://app.borgee.io",
		QueueStateDir:   filepath.Join(root, "queue"),
		StatusStateDir:  filepath.Join(root, "status"),
		AuditHandoffDir: filepath.Join(root, "audit-handoff"),
	}
	got, err := ValidateAndPrepare(cfg, ValidationOptions{AllowedStateRoots: []string{root}})
	if err != nil {
		t.Fatalf("ValidateAndPrepare configured exact origin: %v", err)
	}
	if !got.Enabled {
		t.Fatalf("configured outbound prerequisite should be enabled")
	}
	if got.ServerOrigin != "https://app.borgee.io" {
		t.Fatalf("normalized origin: got %q", got.ServerOrigin)
	}
	for _, dir := range []string{got.QueueStateDir, got.StatusStateDir, got.AuditHandoffDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("state dir %q not created: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("state path %q is not a directory", dir)
		}
		if mode := info.Mode().Perm(); mode != 0o700 {
			t.Fatalf("state dir %q mode: got %o want 700", dir, mode)
		}
	}
}

func TestPrereqValidateRejectsUnknownOrigin(t *testing.T) {
	root := t.TempDir()
	cfg := PrereqConfig{
		ServerOrigin:    "https://evil.example",
		AllowedOrigins:  "https://app.borgee.io",
		QueueStateDir:   filepath.Join(root, "queue"),
		StatusStateDir:  filepath.Join(root, "status"),
		AuditHandoffDir: filepath.Join(root, "audit-handoff"),
	}
	_, err := ValidateAndPrepare(cfg, ValidationOptions{AllowedStateRoots: []string{root}})
	if err == nil || !strings.Contains(err.Error(), "not in allowed origins") {
		t.Fatalf("expected fail-closed unknown origin error, got %v", err)
	}
}

func TestPrereqValidateRejectsMalformedOrigins(t *testing.T) {
	root := t.TempDir()
	for _, origin := range []string{
		"http://app.borgee.io",
		"ftp://app.borgee.io",
		"https://user:pass@app.borgee.io",
		"https://app.borgee.io/path",
		"https://app.borgee.io#fragment",
		"https://",
	} {
		t.Run(origin, func(t *testing.T) {
			cfg := PrereqConfig{
				ServerOrigin:    origin,
				AllowedOrigins:  "https://app.borgee.io",
				QueueStateDir:   filepath.Join(root, "queue"),
				StatusStateDir:  filepath.Join(root, "status"),
				AuditHandoffDir: filepath.Join(root, "audit-handoff"),
			}
			if _, err := ValidateAndPrepare(cfg, ValidationOptions{AllowedStateRoots: []string{root}}); err == nil {
				t.Fatalf("expected malformed origin %q to fail closed", origin)
			}
		})
	}
}

func TestPrereqValidateRejectsLocalAndPrivateHTTPSOriginsByDefault(t *testing.T) {
	root := t.TempDir()
	for _, origin := range []string{
		"https://localhost",
		"https://localhost.",
		"https://127.0.0.1",
		"https://[::1]",
		"https://[::ffff:127.0.0.1]",
		"https://10.0.0.1",
		"https://172.16.0.1",
		"https://172.31.255.255",
		"https://192.168.1.10",
		"https://169.254.1.1",
		"https://169.254.169.254",
		"https://[fc00::1]",
		"https://[fd12:3456:789a::1]",
		"https://[::ffff:10.0.0.1]",
		"https://[fe80::1]",
		"https://[fe80::1%25lo0]",
	} {
		t.Run(origin, func(t *testing.T) {
			cfg := PrereqConfig{
				ServerOrigin:    origin,
				AllowedOrigins:  origin,
				QueueStateDir:   filepath.Join(root, "queue"),
				StatusStateDir:  filepath.Join(root, "status"),
				AuditHandoffDir: filepath.Join(root, "audit-handoff"),
			}
			if _, err := ValidateAndPrepare(cfg, ValidationOptions{AllowedStateRoots: []string{root}}); err == nil {
				t.Fatalf("expected local/private origin %q to fail closed", origin)
			}
		})
	}
}

func TestPrereqValidateRejectsLocalAndPrivateAllowedOriginsByDefault(t *testing.T) {
	root := t.TempDir()
	cfg := PrereqConfig{
		ServerOrigin:    "https://app.borgee.io",
		AllowedOrigins:  "https://app.borgee.io, https://169.254.169.254",
		QueueStateDir:   filepath.Join(root, "queue"),
		StatusStateDir:  filepath.Join(root, "status"),
		AuditHandoffDir: filepath.Join(root, "audit-handoff"),
	}
	_, err := ValidateAndPrepare(cfg, ValidationOptions{AllowedStateRoots: []string{root}})
	if err == nil || !strings.Contains(err.Error(), "local/private origins are not allowed") {
		t.Fatalf("expected private allowed origin to fail closed, got %v", err)
	}
}

func TestPrereqValidateLoopbackHTTPAllowanceStaysConstrained(t *testing.T) {
	root := t.TempDir()
	for name, tc := range map[string]struct {
		origin  string
		allowed bool
	}{
		"http loopback allowed":         {origin: "http://127.0.0.1:4900", allowed: true},
		"http localhost allowed":        {origin: "http://localhost:4900", allowed: true},
		"http private remains denied":   {origin: "http://10.0.0.1:4900", allowed: false},
		"https loopback remains denied": {origin: "https://127.0.0.1:4900", allowed: false},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := PrereqConfig{
				ServerOrigin:    tc.origin,
				AllowedOrigins:  tc.origin,
				QueueStateDir:   filepath.Join(root, "queue"),
				StatusStateDir:  filepath.Join(root, "status"),
				AuditHandoffDir: filepath.Join(root, "audit-handoff"),
			}
			_, err := ValidateAndPrepare(cfg, ValidationOptions{
				AllowedStateRoots: []string{root},
				AllowLoopbackHTTP: true,
			})
			if tc.allowed && err != nil {
				t.Fatalf("expected explicit loopback allowance for %q, got %v", tc.origin, err)
			}
			if !tc.allowed && err == nil {
				t.Fatalf("expected constrained loopback allowance to reject %q", tc.origin)
			}
		})
	}
}

func TestPrereqValidateRejectsPartialConfig(t *testing.T) {
	_, err := ValidateAndPrepare(PrereqConfig{ServerOrigin: "https://app.borgee.io"}, ValidationOptions{})
	if err == nil || !strings.Contains(err.Error(), "partial outbound prerequisite config") {
		t.Fatalf("expected partial config error, got %v", err)
	}
}

// TestPrereqValidateDevContainerWSAllowance (#1050 blocker #1) — the
// dev-stack escape hatch lets ws:// flow to docker-network DNS like
// `borgee-server` and to RFC1918 / localhost destinations when the
// operator explicitly opts in via BORGEE_DEV_ALLOW_INSECURE_WS=1
// (plumbed as ValidationOptions.AllowDevContainerWS). Without the
// opt-in the strict https/wss-only rule continues to bind. With the
// opt-in, attacker.com (public DNS shape) still rejects.
func TestPrereqValidateDevContainerWSAllowance(t *testing.T) {
	root := t.TempDir()
	for name, tc := range map[string]struct {
		origin   string
		devOptIn bool
		allowed  bool
	}{
		"ws localhost without opt-in":           {origin: "ws://localhost:4900", devOptIn: false, allowed: false},
		"ws localhost with opt-in":              {origin: "ws://localhost:4900", devOptIn: true, allowed: true},
		"ws docker DNS without opt-in":          {origin: "ws://borgee-server:4900", devOptIn: false, allowed: false},
		"ws docker DNS with opt-in":             {origin: "ws://borgee-server:4900", devOptIn: true, allowed: true},
		"ws RFC1918 with opt-in":                {origin: "ws://10.0.0.5:4900", devOptIn: true, allowed: true},
		"ws public dotted host with opt-in":     {origin: "ws://attacker.example.com:4900", devOptIn: true, allowed: false},
		"ws public dotted host without opt-in":  {origin: "ws://attacker.example.com:4900", devOptIn: false, allowed: false},
		"http loopback IP with opt-in":          {origin: "http://127.0.0.1:4900", devOptIn: true, allowed: true},
		"wss public host stays allowed default": {origin: "wss://app.borgee.io", devOptIn: false, allowed: true},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := PrereqConfig{
				ServerOrigin:    tc.origin,
				AllowedOrigins:  tc.origin,
				QueueStateDir:   filepath.Join(root, "queue"),
				StatusStateDir:  filepath.Join(root, "status"),
				AuditHandoffDir: filepath.Join(root, "audit-handoff"),
			}
			_, err := ValidateAndPrepare(cfg, ValidationOptions{
				AllowedStateRoots:   []string{root},
				AllowDevContainerWS: tc.devOptIn,
			})
			if tc.allowed && err != nil {
				t.Fatalf("expected dev-stack allowance for %q (optIn=%v), got %v", tc.origin, tc.devOptIn, err)
			}
			if !tc.allowed && err == nil {
				t.Fatalf("expected reject for %q (optIn=%v), got nil error", tc.origin, tc.devOptIn)
			}
		})
	}
}

func TestPrereqValidateRejectsStatePathsOutsideAllowedRoots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	base := PrereqConfig{
		ServerOrigin:    "https://app.borgee.io",
		AllowedOrigins:  "https://app.borgee.io",
		QueueStateDir:   filepath.Join(root, "queue"),
		StatusStateDir:  filepath.Join(root, "status"),
		AuditHandoffDir: filepath.Join(root, "audit-handoff"),
	}
	for name, mutate := range map[string]func(*PrereqConfig){
		"relative queue": func(c *PrereqConfig) { c.QueueStateDir = "queue" },
		"outside status": func(c *PrereqConfig) { c.StatusStateDir = filepath.Join(outside, "status") },
		"root audit":     func(c *PrereqConfig) { c.AuditHandoffDir = root },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := base
			mutate(&cfg)
			if _, err := ValidateAndPrepare(cfg, ValidationOptions{AllowedStateRoots: []string{root}}); err == nil {
				t.Fatalf("expected invalid state path to fail closed")
			}
		})
	}
}

// TestDefaultStateRoots_LinuxMatchesSetup (amend gap #5) — the internal
// setup helper (invoked by `borgee install`) provisions state dirs under
// /var/lib/borgee/{queue,status,...} and the
// systemd unit's ExecStart points there. If DefaultStateRoots() returned
// only the legacy /var/lib/borgee-helper root, every daemon-startup
// state-dir validation would fail "outside allowed Helper-owned state
// roots" — the gap that blocked Stage 2 e2e. The default keeps the
// legacy root too so in-place upgrades from older packages don't lose
// their existing state.
func TestDefaultStateRoots_LinuxMatchesSetup(t *testing.T) {
	roots := DefaultStateRoots()
	if len(roots) == 0 {
		t.Skip("DefaultStateRoots empty on this platform — see GOOS switch")
	}
	want := map[string]bool{"/var/lib/borgee": false, "/Library/Application Support/Borgee/Helper": false}
	// Either Linux or Darwin must show up in want; missing both indicates
	// the setup default root drifted from the GOOS we're running on.
	matched := false
	for _, r := range roots {
		if _, ok := want[r]; ok {
			matched = true
			want[r] = true
		}
	}
	if !matched {
		t.Fatalf("DefaultStateRoots %v did not match setup.go state root for this platform", roots)
	}
}
