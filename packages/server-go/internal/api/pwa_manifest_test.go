// Package api_test — pwa_manifest_test.go: DL-4.4 PWA Web App Manifest
// endpoint tests.
//
// Pins:
//   - Content-Type "application/manifest+json" (W3C standard MIME)
//   - Required fields per W3C App Manifest spec subset (name / short_name
//     / start_url / display / icons)
//   - display=standalone (blueprint L22 literal)
//   - Public endpoint (no auth)
//   - Endpoint naming stays separate from 'plugin-manifest' / 'manifest/plugins'
//     (DL-4 vs HB-1 #491)
package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/testutil"
)

// TestDL_PWAManifest_PublicEndpoint pins acceptance — GET /api/v1/pwa/manifest
// requires no auth so browser install prompts can fetch before login.
func TestDL_PWAManifest_PublicEndpoint(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/pwa/manifest")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (public endpoint), got %d", resp.StatusCode)
	}
}

// TestDL_PWAManifest_ContentType pins W3C MIME — Content-Type:
// application/manifest+json, which browser install prompts recognize.
func TestDL_PWAManifest_ContentType(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/pwa/manifest")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	got := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(got, "application/manifest+json") {
		t.Errorf("Content-Type = %q, want prefix application/manifest+json (W3C standard)", got)
	}
}

// TestDL_PWAManifest_RequiredFields pins W3C App Manifest spec subset
// — required + recommended fields (name / short_name / start_url /
// display / icons).
func TestDL_PWAManifest_RequiredFields(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/pwa/manifest")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("manifest decode: %v", err)
	}

	for _, key := range []string{"name", "short_name", "start_url", "display", "theme_color", "background_color", "scope", "icons"} {
		if _, ok := m[key]; !ok {
			t.Errorf("manifest missing required field %q (W3C App Manifest spec)", key)
		}
	}

	// display=standalone is the blueprint L22 literal.
	if d, _ := m["display"].(string); d != "standalone" {
		t.Errorf("display = %q, want %q (blueprint L22 literal)", d, "standalone")
	}

	// Icons include at least the W3C-recommended 192x192 and 512x512 sizes.
	icons, _ := m["icons"].([]any)
	if len(icons) < 2 {
		t.Errorf("icons count = %d, want >=2 (192x192 + 512x512 W3C baseline)", len(icons))
	}
	hasSize := func(target string) bool {
		for _, i := range icons {
			ic, _ := i.(map[string]any)
			if s, _ := ic["sizes"].(string); s == target {
				return true
			}
		}
		return false
	}
	if !hasSize("192x192") {
		t.Error("icons missing 192x192 (W3C baseline)")
	}
	if !hasSize("512x512") {
		t.Error("icons missing 512x512 (W3C baseline)")
	}
}

// TestDL_PWAManifest_NoSecretsLeak pins the privacy constraint: manifest
// content must not contain secret / token / api_key / vapid literals.
func TestDL_PWAManifest_NoSecretsLeak(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/pwa/manifest")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}

	// JSON-marshal back + scan for forbidden substrings.
	body, _ := json.Marshal(m)
	bodyStr := strings.ToLower(string(body))
	for _, forbidden := range []string{
		"vapid_secret", "vapid_private", "private_key",
		"api_key", "secret", "token",
		"borgee_token", "borgee_admin_session",
	} {
		if strings.Contains(bodyStr, strings.ToLower(forbidden)) {
			t.Errorf("manifest leaks forbidden substring %q (public endpoint privacy guard broken)", forbidden)
		}
	}
}

// TestDL_PWAManifest_NameNotPluginManifest pins the naming split: the DL-4
// endpoint path must not use 'plugin-manifest' (reserved by HB-1 #491).
func TestDL_PWAManifest_NameNotPluginManifest(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	// Verify the wrong HB-1 path returns 404, so DL-4 does not impersonate it.
	resp, err := http.Get(ts.URL + "/api/v1/plugin-manifest")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Either 404 (no such route) or 501 (placeholder). Anything 2xx
	// means DL-4 used the HB-1 literal.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		t.Errorf("/api/v1/plugin-manifest returned %d — DL-4 must not use the HB-1 literal",
			resp.StatusCode)
	}

	// Verify also the legacy bad name (manifest/plugins) is NOT served by DL-4.
	resp2, err := http.Get(ts.URL + "/api/v1/manifest/plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
		t.Errorf("/api/v1/manifest/plugins returned %d — legacy DL-4 naming should stay retired",
			resp2.StatusCode)
	}
}
