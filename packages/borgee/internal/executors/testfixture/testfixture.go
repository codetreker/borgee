// Package testfixture builds minimal manifest + binding bytes for executor
// tests. The bytes are NOT cryptographically signed — executor tests don't
// re-validate the signature (jobpolicy.Evaluate covers that path under
// internal/jobpolicy). Tests that need a signed manifest should use
// internal/jobpolicy/policy_test.go's signedManifest helper directly.
package testfixture

import (
	"encoding/json"
	"strings"
	"testing"
)

// PathSpec — a single (id, root) entry to emit into the manifest's `paths`.
type PathSpec struct {
	ID   string
	Root string
	Mode string // defaults to "write_config" if empty
}

// Build returns (manifestJSON, bindingJSON) with the given paths declared
// in the manifest and PathIDs listed in the binding (a subset). Manifest
// digest is a stable placeholder so tests don't accidentally couple to a
// real signing key.
func Build(t *testing.T, paths []PathSpec, boundPathIDs []string) ([]byte, []byte) {
	t.Helper()
	declared := make([]map[string]string, 0, len(paths))
	for _, p := range paths {
		mode := p.Mode
		if mode == "" {
			mode = "write_config"
		}
		declared = append(declared, map[string]string{
			"id":   p.ID,
			"root": p.Root,
			"mode": mode,
		})
	}
	manifest, err := json.Marshal(map[string]any{
		"manifest_version": 1,
		"issued_at":        "2026-01-01T00:00:00Z",
		"expires_at":       "2027-01-01T00:00:00Z",
		"paths":            declared,
		"signature":        "stub", // jobpolicy verifies upstream; manifestpath does not
	})
	if err != nil {
		t.Fatalf("manifest marshal: %v", err)
	}
	binding, err := json.Marshal(map[string]any{
		"manifest_digest": "sha256:" + strings.Repeat("0", 64),
		"path_ids":        boundPathIDs,
	})
	if err != nil {
		t.Fatalf("binding marshal: %v", err)
	}
	return manifest, binding
}
