// helper_job_plugin_connection_decode_test.go — #1049 unit coverage
// for validBorgeePluginConnectionID + decodeBorgeePluginRemovePayload.
//
// These functions are exercised end-to-end via the API package
// integration tests but coverage is measured per-package, so the
// store-level haystack threshold for the file flagged them as 0%.
// White-box tests in this same package (store) close the gap.

package store

import (
	"errors"
	"strings"
	"testing"
)

func TestValidBorgeePluginConnectionID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"valid simple", "borgee-plugin:test-1", true},
		{"valid all-allowed chars", "borgee-plugin:abc_def.GHI-123", true},
		{"valid max suffix len 200", "borgee-plugin:" + strings.Repeat("a", 200), true},
		{"missing prefix", "test-1", false},
		{"wrong prefix", "borgee:test-1", false},
		{"empty suffix", "borgee-plugin:", false},
		{"suffix too long 201", "borgee-plugin:" + strings.Repeat("a", 201), false},
		{"suffix equals .", "borgee-plugin:.", false},
		{"suffix equals ..", "borgee-plugin:..", false},
		{"suffix contains ..", "borgee-plugin:foo..bar", false},
		{"forward slash", "borgee-plugin:foo/bar", false},
		{"backslash", "borgee-plugin:foo\\bar", false},
		{"nul byte", "borgee-plugin:foo\x00bar", false},
		{"space char", "borgee-plugin:foo bar", false},
		{"unicode char", "borgee-plugin:fooé", false},
		{"colon in suffix", "borgee-plugin:foo:bar", false},
		{"plus char", "borgee-plugin:foo+bar", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := validBorgeePluginConnectionID(tc.in)
			if got != tc.ok {
				t.Fatalf("validBorgeePluginConnectionID(%q) = %v, want %v", tc.in, got, tc.ok)
			}
		})
	}
}

func TestDecodeBorgeePluginRemovePayload(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		raw := `{"agent_id":"agent-1","connection_id":"borgee-plugin:test-1"}`
		p, err := decodeBorgeePluginRemovePayload(raw)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.AgentID != "agent-1" || p.ConnectionID != "borgee-plugin:test-1" {
			t.Fatalf("payload = %+v", p)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Parallel()
		raw := `{"agent_id":"  agent-1  ","connection_id":"  borgee-plugin:test-1  "}`
		p, err := decodeBorgeePluginRemovePayload(raw)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if p.AgentID != "agent-1" || p.ConnectionID != "borgee-plugin:test-1" {
			t.Fatalf("payload after trim = %+v", p)
		}
	})

	t.Run("missing agent_id", func(t *testing.T) {
		t.Parallel()
		raw := `{"connection_id":"borgee-plugin:test-1"}`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("expected ErrHelperJobSchemaInvalid, got %v", err)
		}
	})

	t.Run("empty agent_id after trim", func(t *testing.T) {
		t.Parallel()
		raw := `{"agent_id":"   ","connection_id":"borgee-plugin:test-1"}`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("expected ErrHelperJobSchemaInvalid, got %v", err)
		}
	})

	t.Run("agent_id too long", func(t *testing.T) {
		t.Parallel()
		raw := `{"agent_id":"` + strings.Repeat("a", 256) + `","connection_id":"borgee-plugin:test-1"}`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("expected ErrHelperJobSchemaInvalid, got %v", err)
		}
	})

	t.Run("invalid connection_id (no prefix)", func(t *testing.T) {
		t.Parallel()
		raw := `{"agent_id":"agent-1","connection_id":"foo"}`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("expected ErrHelperJobSchemaInvalid, got %v", err)
		}
	})

	t.Run("invalid connection_id (path traversal)", func(t *testing.T) {
		t.Parallel()
		raw := `{"agent_id":"agent-1","connection_id":"borgee-plugin:../etc"}`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("expected ErrHelperJobSchemaInvalid, got %v", err)
		}
	})

	t.Run("forbidden field shell", func(t *testing.T) {
		t.Parallel()
		raw := `{"agent_id":"agent-1","connection_id":"borgee-plugin:test-1","shell":"/bin/sh"}`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobForbiddenField) {
			t.Fatalf("expected ErrHelperJobForbiddenField, got %v", err)
		}
	})

	t.Run("forbidden field credential", func(t *testing.T) {
		t.Parallel()
		raw := `{"agent_id":"agent-1","connection_id":"borgee-plugin:test-1","credential":"x"}`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobForbiddenField) {
			t.Fatalf("expected ErrHelperJobForbiddenField, got %v", err)
		}
	})

	t.Run("connection_id roundtrip is allowed (not flagged as forbidden)", func(t *testing.T) {
		t.Parallel()
		// connection_id passes the forbidden-field filter intentionally
		// per #1049 — the client round-trips the server-derived id.
		raw := `{"agent_id":"agent-1","connection_id":"borgee-plugin:test-roundtrip"}`
		if _, err := decodeBorgeePluginRemovePayload(raw); err != nil {
			t.Fatalf("connection_id round-trip rejected: %v", err)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		t.Parallel()
		raw := `not-json`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("expected ErrHelperJobSchemaInvalid, got %v", err)
		}
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		t.Parallel()
		// DisallowUnknownFields → unknown but non-forbidden key fails decode.
		raw := `{"agent_id":"agent-1","connection_id":"borgee-plugin:test-1","extra_unrecognized":"x"}`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("expected ErrHelperJobSchemaInvalid, got %v", err)
		}
	})

	t.Run("null body", func(t *testing.T) {
		t.Parallel()
		raw := `null`
		_, err := decodeBorgeePluginRemovePayload(raw)
		if !errors.Is(err, ErrHelperJobSchemaInvalid) {
			t.Fatalf("expected ErrHelperJobSchemaInvalid, got %v", err)
		}
	})
}
