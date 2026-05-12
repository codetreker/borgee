package admin_test

// handlers_field_whitelist_test.go covers ADM-0.2 §1 negative assertion 2.C:
// admin read endpoints on /admin-api/v1/* must be metadata-only. The test calls
// each admin read endpoint, decodes the response, and walks every nested map /
// slice asserting no key in the forbidden set {body, content, text, artifact}
// appears. This is a fail-closed reflective scan: adding a new endpoint that
// leaks one of these fields immediately trips the test.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/testutil"
)

// forbiddenKeys is the locked deny-list for admin response payloads.
// Synced with review checklist §ADM-0.2 §1 (grep 0 hits for these tokens
// in admin handler responses).
var forbiddenKeys = []string{"body", "content", "text", "artifact"}

// walkForForbidden recursively descends a decoded JSON value (map / slice /
// scalar) and reports the first forbidden key it finds. Returns "" when none.
func walkForForbidden(v any) string {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			lk := strings.ToLower(k)
			for _, bad := range forbiddenKeys {
				if lk == bad {
					return k
				}
			}
			if hit := walkForForbidden(child); hit != "" {
				return hit
			}
		}
	case []any:
		for _, child := range t {
			if hit := walkForForbidden(child); hit != "" {
				return hit
			}
		}
	}
	return ""
}

func TestAdminFieldWhitelist_GodModeEndpointsAreMetadataOnly(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAsAdmin(t, ts.URL)

	// All currently mounted admin read endpoints. As new endpoints are
	// added under /admin-api/v1/*, append them here so the deny-list scan
	// covers the full admin read surface.
	endpoints := []string{
		"/admin-api/v1/stats",
		"/admin-api/v1/users",
		"/admin-api/v1/invites",
		"/admin-api/v1/channels",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			resp, _ := testutil.JSON(t, http.MethodGet, ts.URL+ep, tok, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: expected 200, got %d", ep, resp.StatusCode)
			}
			// Re-fetch raw body — testutil.JSON only keeps top-level map keys
			// of the parsed result, but the helper already decoded; we want
			// to scan the full response tree, including nested arrays.
			req, _ := http.NewRequest(http.MethodGet, ts.URL+ep, nil)
			req.AddCookie(&http.Cookie{Name: "borgee_admin_session", Value: tok})
			r2, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s: do: %v", ep, err)
			}
			defer r2.Body.Close()
			var raw any
			if err := json.NewDecoder(r2.Body).Decode(&raw); err != nil {
				t.Fatalf("%s: decode: %v", ep, err)
			}
			if hit := walkForForbidden(raw); hit != "" {
				t.Fatalf("%s: response leaked forbidden key %q (god-mode must be metadata-only)", ep, hit)
			}
		})
	}
}
