package manifestpath

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func manifestJSON(t *testing.T, paths ...map[string]string) []byte {
	t.Helper()
	declared := make([]map[string]string, 0, len(paths))
	declared = append(declared, paths...)
	return mustJSON(t, map[string]any{
		"manifest_version": 1,
		"issued_at":        "2026-01-01T00:00:00Z",
		"expires_at":       "2027-01-01T00:00:00Z",
		"paths":            declared,
		"signature":        "stub", // not verified here; jobpolicy verifies upstream
	})
}

func bindingJSON(t *testing.T, pathIDs ...string) []byte {
	t.Helper()
	return mustJSON(t, map[string]any{
		"manifest_digest": "sha256:" + strings.Repeat("0", 64),
		"path_ids":        pathIDs,
	})
}

func TestResolve_HappyPath(t *testing.T) {
	m := manifestJSON(t, map[string]string{
		"id":   "openclaw_agent_config",
		"root": "/var/lib/openclaw/agents",
		"mode": "write_config",
	})
	b := bindingJSON(t, "openclaw_agent_config")

	got, err := Resolve(m, b, "openclaw_agent_config")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "/var/lib/openclaw/agents" {
		t.Fatalf("Resolve = %q, want /var/lib/openclaw/agents", got)
	}
}

func TestResolve_PathIDNotInBinding(t *testing.T) {
	m := manifestJSON(t, map[string]string{"id": "openclaw_agent_config", "root": "/var/lib/openclaw", "mode": "write_config"})
	b := bindingJSON(t, "borgee_plugin_config")
	if _, err := Resolve(m, b, "openclaw_agent_config"); !errors.Is(err, ErrPathIDNotInBinding) {
		t.Fatalf("err = %v, want ErrPathIDNotInBinding", err)
	}
}

func TestResolve_PathIDNotInManifest(t *testing.T) {
	m := manifestJSON(t, map[string]string{"id": "other", "root": "/var/lib/other", "mode": "write_config"})
	b := bindingJSON(t, "openclaw_agent_config")
	if _, err := Resolve(m, b, "openclaw_agent_config"); !errors.Is(err, ErrPathIDNotInManifest) {
		t.Fatalf("err = %v, want ErrPathIDNotInManifest", err)
	}
}

func TestResolve_RelativePathInManifest(t *testing.T) {
	m := manifestJSON(t, map[string]string{"id": "x", "root": "var/lib/x", "mode": "write_config"})
	b := bindingJSON(t, "x")
	if _, err := Resolve(m, b, "x"); !errors.Is(err, ErrPathNotAbsolute) {
		t.Fatalf("err = %v, want ErrPathNotAbsolute", err)
	}
}

func TestResolve_DotDotInManifestRoot(t *testing.T) {
	m := manifestJSON(t, map[string]string{"id": "x", "root": "/var/lib/../etc", "mode": "write_config"})
	b := bindingJSON(t, "x")
	if _, err := Resolve(m, b, "x"); !errors.Is(err, ErrPathNotAbsolute) {
		t.Fatalf("err = %v, want ErrPathNotAbsolute", err)
	}
}

func TestResolve_MalformedManifest(t *testing.T) {
	if _, err := Resolve([]byte(`{not json`), bindingJSON(t, "x"), "x"); !errors.Is(err, ErrManifestParse) {
		t.Fatalf("err = %v, want ErrManifestParse", err)
	}
}

func TestResolve_MalformedBinding(t *testing.T) {
	m := manifestJSON(t, map[string]string{"id": "x", "root": "/x", "mode": "write_config"})
	if _, err := Resolve(m, []byte(`{not json`), "x"); !errors.Is(err, ErrBindingParse) {
		t.Fatalf("err = %v, want ErrBindingParse", err)
	}
}

func TestResolve_EmptyManifest(t *testing.T) {
	if _, err := Resolve(nil, bindingJSON(t, "x"), "x"); !errors.Is(err, ErrManifestParse) {
		t.Fatalf("err = %v, want ErrManifestParse", err)
	}
}

func TestResolve_EmptyBinding(t *testing.T) {
	m := manifestJSON(t, map[string]string{"id": "x", "root": "/x", "mode": "write_config"})
	if _, err := Resolve(m, nil, "x"); !errors.Is(err, ErrBindingParse) {
		t.Fatalf("err = %v, want ErrBindingParse", err)
	}
}

func TestResolve_BindingStrictRejectsUnknownField(t *testing.T) {
	m := manifestJSON(t, map[string]string{"id": "x", "root": "/x", "mode": "write_config"})
	b := []byte(`{"manifest_digest":"sha256:` + strings.Repeat("0", 64) + `","path_ids":["x"],"unknown":1}`)
	if _, err := Resolve(m, b, "x"); !errors.Is(err, ErrBindingParse) {
		t.Fatalf("err = %v, want ErrBindingParse for unknown field", err)
	}
}

func TestJoinUnderResolved_HappyPath(t *testing.T) {
	got, err := JoinUnderResolved("/x/y", "agents/foo.json")
	if err != nil {
		t.Fatalf("JoinUnderResolved: %v", err)
	}
	if got != "/x/y/agents/foo.json" {
		t.Fatalf("got %q, want /x/y/agents/foo.json", got)
	}
}

func TestJoinUnderResolved_RejectsParentEscape(t *testing.T) {
	if _, err := JoinUnderResolved("/x/y", "../etc/passwd"); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("err = %v, want ErrPathEscape", err)
	}
}

func TestJoinUnderResolved_RejectsAbsoluteRel(t *testing.T) {
	if _, err := JoinUnderResolved("/x/y", "/etc/passwd"); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("err = %v, want ErrPathEscape", err)
	}
}

func TestJoinUnderResolved_RejectsNUL(t *testing.T) {
	if _, err := JoinUnderResolved("/x/y", "a\x00b"); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("err = %v, want ErrPathEscape", err)
	}
}

func TestJoinUnderResolved_RejectsEmpty(t *testing.T) {
	if _, err := JoinUnderResolved("/x/y", ""); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("err = %v, want ErrPathEscape", err)
	}
}

func TestJoinUnderResolved_CleanRedundantSeparators(t *testing.T) {
	got, err := JoinUnderResolved("/x/y", "a//b/./c")
	if err != nil {
		t.Fatalf("JoinUnderResolved: %v", err)
	}
	if got != "/x/y/a/b/c" {
		t.Fatalf("got %q, want /x/y/a/b/c", got)
	}
}

// TestJoinUnderResolved_RealpathSymlinkEscape — placeholder. Realpath-based
// TOCTOU enforcement (resolving symlinks before deciding "still under root")
// is tracked at #1028 and not implemented here. The runtime landlock /
// sandbox-exec layers cover the hardening today.
func TestJoinUnderResolved_RealpathSymlinkEscape(t *testing.T) {
	t.Skip("realpath-based escape check tracked at #1028 follow-up; landlock/sandbox-exec covers TOCTOU at runtime")
}
