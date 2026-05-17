// removed_user_rail_routes_test.go — #975 route-registration absence lock (RM-3).
//
// Per skeptic-owner contract C2: assert ROUTE-REGISTRATION absence, NOT
// HTTP 404. A future PR re-adding `/api/v1/me/impersonation-grant` for a
// different purpose (say, self-service rotation) would legitimately flip a
// 404 test to 200/204 — the wrong axis of failure. The actual invariant
// #975 cemented is "no user-rail handler is wired under these paths."
//
// Go's `http.ServeMux.Handler(req)` returns the registered pattern that
// would handle the request. The catch-all `mux.HandleFunc("/api/v1/",
// respondNotImplemented)` is a deliberate 501 placeholder; a real
// registration of the deleted routes would produce a more specific pattern.
// We assert that for each (method, path) pair, the matched pattern is
// either empty (no match — falls through to default) OR the catch-all
// "/api/v1/" 501 placeholder. Any specific pattern is a #975 regression.
package server

import (
	"net/http"
	"testing"
)

func TestRoutes_NoDeletedUserRailPatterns(t *testing.T) {
	t.Parallel()
	srv, _ := testServer(t)

	// The 4 user-rail routes deleted in #975 (see server.go:429 comment and
	// internal/api/admin_endpoints.go:7 header for the cleanup record).
	cases := []struct {
		method, path string
	}{
		{"GET", "/api/v1/me/admin-actions"},
		{"GET", "/api/v1/me/impersonation-grant"},
		{"POST", "/api/v1/me/impersonation-grant"},
		{"DELETE", "/api/v1/me/impersonation-grant"},
	}

	// Allowed match patterns: empty (no registration) OR the catch-all 501
	// placeholder. Anything else means a real route is wired again.
	allowed := map[string]bool{
		"":           true, // no match at all (shouldn't happen given catch-all, but kept defensive)
		"/api/v1/":   true, // the respondNotImplemented placeholder
	}

	for _, c := range cases {
		req, err := http.NewRequest(c.method, c.path, nil)
		if err != nil {
			t.Fatalf("NewRequest %s %s: %v", c.method, c.path, err)
		}
		_, pattern := srv.mux.Handler(req)
		if !allowed[pattern] {
			t.Errorf(
				"#975 regression: %s %s is registered under pattern %q "+
					"(expected empty or '/api/v1/' catch-all). A real handler "+
					"for the deleted user-rail surface is wired again.",
				c.method, c.path, pattern,
			)
		}
	}
}
