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

func TestPrereqValidateRejectsPartialConfig(t *testing.T) {
	_, err := ValidateAndPrepare(PrereqConfig{ServerOrigin: "https://app.borgee.io"}, ValidationOptions{})
	if err == nil || !strings.Contains(err.Error(), "partial outbound prerequisite config") {
		t.Fatalf("expected partial config error, got %v", err)
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
