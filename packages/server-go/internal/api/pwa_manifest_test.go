// Package api_test — pwa_manifest_test.go: DL-4.4 PWA Web App Manifest
// endpoint tests.
//
// Pins:
//   - Content-Type "application/manifest+json" (W3C 标准 MIME)
//   - Required fields per W3C App Manifest spec subset (name / short_name
//     / start_url / display / icons)
//   - display=standalone (蓝图 L22 字面)
//   - 公开 endpoint (无 auth)
//   - 约束 endpoint 字面不含 'plugin-manifest' / 'manifest/plugins'
//     (DL-4 vs HB-1 #491 命名分立)
package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/testutil"
)

// TestDL_PWAManifest_PublicEndpoint pins acceptance — GET /api/v1/pwa/manifest
// 不需 auth (浏览器 install prompt 在 login 前 fetch).
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
// application/manifest+json (浏览器 install prompt trigger 识别).
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

	// display=standalone 蓝图 L22 字面
	if d, _ := m["display"].(string); d != "standalone" {
		t.Errorf("display = %q, want %q (蓝图 L22 字面)", d, "standalone")
	}

	// icons 至少 1 个 + 含 192x192 / 512x512 W3C 推荐基线
	icons, _ := m["icons"].([]any)
	if len(icons) < 2 {
		t.Errorf("icons count = %d, want ≥2 (192x192 + 512x512 W3C 基线)", len(icons))
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
		t.Error("icons missing 192x192 (W3C 基线)")
	}
	if !hasSize("512x512") {
		t.Error("icons missing 512x512 (W3C 基线)")
	}
}

// TestDL_PWAManifest_NoSecretsLeak pins 约束 — manifest 内容不含
// secret / token / api_key / vapid 等字面 (公开 endpoint 隐私防御).
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
			t.Errorf("manifest leaks forbidden substring %q (公开 endpoint 隐私防御 broken)", forbidden)
		}
	}
}

// TestDL_PWAManifest_NameNotPluginManifest pins ⚠️ 命名分立锚 — DL-4
// endpoint 路径不含 'plugin-manifest' (HB-1 #491 独占字面). zhanma-a
// mismatch audit 锚源.
func TestDL_PWAManifest_NameNotPluginManifest(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	// Verify the WRONG path (HB-1 字面) returns 404 — DL-4 不冒充该 endpoint.
	resp, err := http.Get(ts.URL + "/api/v1/plugin-manifest")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Either 404 (no such route) or 501 (placeholder). Anything 2xx
	// means DL-4 squatted on HB-1 字面 — mismatch broken.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		t.Errorf("/api/v1/plugin-manifest returned %d — DL-4 must not squat HB-1 字面 (zhanma-a mismatch audit)",
			resp.StatusCode)
	}

	// Verify also the legacy bad name (manifest/plugins) is NOT served by DL-4.
	resp2, err := http.Get(ts.URL + "/api/v1/manifest/plugins")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 200 && resp2.StatusCode < 300 {
		t.Errorf("/api/v1/manifest/plugins returned %d — old DL-4 命名应回退 (Option A 改名后)",
			resp2.StatusCode)
	}
}
