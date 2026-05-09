// Package api_test — thumbnail_test.go: CV-3 v2 acceptance tests for the
// POST /api/v1/artifacts/:id/thumbnail endpoint (Phase 5+, #cv-3-v2).
//
// 原则 pins exercised:
//   - ① owner-only ACL (admin → 401, non-owner → 403).
//   - ② thumbnail_url MUST be https — XSS 红线第一道.
//   - ③ kind 闸 — 仅 markdown / code (二端互斥跟 PreviewableKinds).
package api_test

import (
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/testutil"
)

// REG-CV3V2-002 — owner posts an https thumbnail_url for a markdown
// artifact, server persists it.
func TestCV_HappyPathMarkdown(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	resp, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Doc",
		"body":  "# heading",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create markdown: got %d (%v)", resp.StatusCode, art)
	}
	id := art["id"].(string)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/thumbnail", tok, map[string]any{
		"thumbnail_url": "https://cdn.example/doc-thumb.png",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("thumbnail: got %d (%v)", resp.StatusCode, data)
	}
	if data["thumbnail_url"] != "https://cdn.example/doc-thumb.png" {
		t.Errorf("thumbnail_url echo: got %v", data["thumbnail_url"])
	}
	if data["artifact_id"] != id {
		t.Errorf("artifact_id echo: got %v", data["artifact_id"])
	}
}

// REG-CV3V2-002b — code kind also accepted.
func TestCV_HappyPathCode(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	resp, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Snippet",
		"type":  "code",
		"body":  "func main() {}",
		"metadata": map[string]any{"language": "go"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create code: got %d (%v)", resp.StatusCode, art)
	}
	id := art["id"].(string)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/thumbnail", tok, map[string]any{
		"thumbnail_url": "https://cdn.example/snippet-thumb.png",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("thumbnail code: got %d (%v)", resp.StatusCode, data)
	}
}

// REG-CV3V2-003 — non-owner → 403 + thumbnail.not_owner.
func TestCV_NonOwner403(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Doc", "body": "# h",
	})
	id := art["id"].(string)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/thumbnail", memberTok, map[string]any{
		"thumbnail_url": "https://cdn.example/x.png",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-owner: got %d, want 403 (%v)", resp.StatusCode, data)
	}
	errStr, _ := data["error"].(string)
	if !strings.Contains(errStr, "thumbnail.not_owner") {
		t.Errorf("error code: got %q, want substring 'thumbnail.not_owner'", errStr)
	}
}

// REG-CV3V2-003b — admin (no auth user) → 401.
func TestCV_Admin401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Doc", "body": "# h",
	})
	id := art["id"].(string)

	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/thumbnail", "", map[string]any{
		"thumbnail_url": "https://cdn.example/x.png",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-token: got %d, want 401", resp.StatusCode)
	}
}

// REG-CV3V2-004 — https only XSS 红线 (复用 ValidateImageLinkURL).
func TestCV_URLHttpsOnly(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Doc", "body": "# h",
	})
	id := art["id"].(string)

	for _, bad := range []string{
		"javascript:alert(1)",
		"data:image/png;base64,AAA",
		"http://cdn.example/x.png",
		"file:///etc/passwd",
		"//cdn.example/x.png",
		"",
	} {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/thumbnail", tok, map[string]any{
			"thumbnail_url": bad,
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("thumbnail_url=%q: got %d, want 400 (%v)", bad, resp.StatusCode, data)
		}
		errStr, _ := data["error"].(string)
		if !strings.Contains(errStr, "thumbnail.url_") {
			t.Errorf("thumbnail_url=%q: error %q lacks 'thumbnail.url_' prefix", bad, errStr)
		}
	}
}

// REG-CV3V2-005 — kind 闸: image_link → 400 thumbnail.kind_not_thumbnailable
// (二端互斥跟 PreviewableKinds).
func TestCV_KindNotThumbnailable_ImageLink(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Hero",
		"type":  "image_link",
		"body":  "https://cdn.example/x.png",
		"metadata": map[string]any{"kind": "image", "url": "https://cdn.example/x.png"},
	})
	id := art["id"].(string)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/thumbnail", tok, map[string]any{
		"thumbnail_url": "https://cdn.example/x-thumb.jpg",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("image_link thumbnail: got %d, want 400 (%v)", resp.StatusCode, data)
	}
	errStr, _ := data["error"].(string)
	if !strings.Contains(errStr, "thumbnail.kind_not_thumbnailable") {
		t.Errorf("error: got %q, want 'thumbnail.kind_not_thumbnailable'", errStr)
	}
}

// REG-CV3V2-005b — video_link + pdf_link 同样 reject.
func TestCV_KindNotThumbnailable_VideoAndPDF(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	for _, kind := range []string{"video_link", "pdf_link"} {
		_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
			"title": "M-" + kind, "type": kind, "body": "https://cdn.example/a." + kind,
		})
		id := art["id"].(string)
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/thumbnail", tok, map[string]any{
			"thumbnail_url": "https://cdn.example/x-thumb.jpg",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("kind=%s thumbnail: got %d, want 400 (互斥跟 preview)", kind, resp.StatusCode)
		}
	}
}

// REG-CV3V2-005c — ThumbnailableKinds vs PreviewableKinds mutually exclusive
// (single source of truth byte-identical 锁定).
func TestCV_ThumbnailableVsPreviewableMutuallyExclusive(t *testing.T) {
	t.Parallel()
	for _, k := range api.ThumbnailableKinds {
		if api.IsPreviewableKind(k) {
			t.Errorf("kind %q in BOTH Thumbnailable and Previewable — 设计 ③ broken", k)
		}
	}
	for _, k := range api.PreviewableKinds {
		if api.IsThumbnailableKind(k) {
			t.Errorf("kind %q in BOTH Previewable and Thumbnailable — 设计 ③ broken", k)
		}
	}
	// And union covers all 5 known kinds (markdown/code/image_link/video_link/pdf_link).
	want := map[string]bool{
		"markdown": true, "code": true, "image_link": true, "video_link": true, "pdf_link": true,
	}
	got := map[string]bool{}
	for _, k := range api.ThumbnailableKinds {
		got[k] = true
	}
	for _, k := range api.PreviewableKinds {
		got[k] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("kind %q not covered by Thumbnailable ∪ Previewable", k)
		}
	}
}

// REG-CV3V2-006 — overwrite 接受.
func TestCV_Overwrite(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)
	resp, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Doc", "body": "# h",
	})
	// flaky-fix (REG-CV3V2-006-GUARD): when CI happens to schedule POST
	// artifact under load and the response body is non-2xx (no `id`
	// field), the bare `art["id"].(string)` type assertion panics
	// (`interface conversion: interface {} is nil, not string`),
	// hiding the real status code. Same guard pattern used by every
	// other TestCV_* sibling in this file (lines 31-34 / 63-66 / etc).
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create artifact: got %d (%v)", resp.StatusCode, art)
	}
	id := art["id"].(string)

	for _, u := range []string{
		"https://cdn.example/v1.png",
		"https://cdn.example/v2.png",
	} {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/thumbnail", tok, map[string]any{
			"thumbnail_url": u,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("thumbnail %q: got %d (%v)", u, resp.StatusCode, data)
		}
		if data["thumbnail_url"] != u {
			t.Errorf("thumbnail_url echo: got %v, want %q", data["thumbnail_url"], u)
		}
	}

	// GET /artifacts/:id roundtrips thumbnail_url.
	resp, head := testutil.JSON(t, "GET", ts.URL+"/api/v1/artifacts/"+id, tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET: %d", resp.StatusCode)
	}
	if head["thumbnail_url"] != "https://cdn.example/v2.png" {
		t.Errorf("GET thumbnail_url: got %v, want v2", head["thumbnail_url"])
	}
}
