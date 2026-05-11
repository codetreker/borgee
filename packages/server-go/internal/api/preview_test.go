// Package api_test — preview_test.go: CV-2 v2 acceptance tests for the
// POST /api/v1/artifacts/:id/preview endpoint (Phase 5, #cv-2-v2).
//
// 覆盖的设计约束:
//   - ① owner-only ACL (admin → 401, non-owner → 403).
//   - ② preview_url MUST be https — XSS 第一层检查, 反向检查 javascript: /
//     data: / http: / file: 全 reject.
//   - ③ kind 检查 — 仅 image_link / video_link / pdf_link 才能 generate
//     preview; markdown / code → 400 preview.kind_not_previewable.
package api_test

import (
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/testutil"
)

// REG-CV2V2-002 (acceptance §1.1 happy + §1.2 owner-only) — owner posts
// an https preview_url for an image_link artifact, server persists it.
func TestCV_PreviewHappyPathImageLink(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	resp, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Hero",
		"type":  "image_link",
		"body":  "https://cdn.example/hero.png",
		"metadata": map[string]any{
			"kind": "image",
			"url":  "https://cdn.example/hero.png",
		},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create image_link: got %d (%v)", resp.StatusCode, art)
	}
	id := art["id"].(string)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/preview", tok, map[string]any{
		"preview_url": "https://cdn.example/hero-thumb.jpg",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview: got %d (%v)", resp.StatusCode, data)
	}
	if data["preview_url"] != "https://cdn.example/hero-thumb.jpg" {
		t.Errorf("preview_url echo: got %v", data["preview_url"])
	}
	if data["artifact_id"] != id {
		t.Errorf("artifact_id echo: got %v", data["artifact_id"])
	}
}

// REG-CV2V2-002b — POST /preview accepts video_link + pdf_link kinds.
func TestCV_PreviewAcceptsVideoAndPDFKinds(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	for _, kind := range []string{"video_link", "pdf_link"} {
		resp, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
			"title": "M-" + kind,
			"type":  kind,
			"body":  "https://cdn.example/a." + kind,
		})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create %s: got %d (%v)", kind, resp.StatusCode, art)
		}
		id := art["id"].(string)
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/preview", tok, map[string]any{
			"preview_url": "https://cdn.example/a-thumb.jpg",
		})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("preview %s: got %d (%v)", kind, resp.StatusCode, data)
		}
	}
}

// REG-CV2V2-003 (acceptance §1.2 owner-only) — non-owner authenticated
// user → 403 + preview.not_owner.
func TestCV_PreviewNonOwner403(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	chID := cv12General(t, ts.URL, ownerTok)

	resp, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", ownerTok, map[string]any{
		"title": "Hero",
		"type":  "image_link",
		"body":  "https://cdn.example/x.png",
		"metadata": map[string]any{"kind": "image", "url": "https://cdn.example/x.png"},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	id := art["id"].(string)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/preview", memberTok, map[string]any{
		"preview_url": "https://cdn.example/x-thumb.jpg",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-owner preview: got %d, want 403 (%v)", resp.StatusCode, data)
	}
	errStr, _ := data["error"].(string)
	if !strings.Contains(errStr, "preview.not_owner") {
		t.Errorf("error code: got %q, want substring 'preview.not_owner'", errStr)
	}
}

// REG-CV2V2-004 (acceptance §1.3 https-only XSS 检查) — non-https
// schemes rejected: javascript: / data: / http: / file: / scheme-relative.
func TestCV_PreviewURLHttpsOnly(t *testing.T) {
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

	for _, bad := range []string{
		"javascript:alert(1)",
		"data:image/png;base64,AAA",
		"http://cdn.example/x.png",
		"file:///etc/passwd",
		"//cdn.example/x.png",
		"",
	} {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/preview", tok, map[string]any{
			"preview_url": bad,
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("preview_url=%q: got %d, want 400 (%v)", bad, resp.StatusCode, data)
		}
		errStr, _ := data["error"].(string)
		// http: / scheme-relative trip the "must be https" branch; the
		// rest trip url_invalid (empty / unparseable / non-http scheme).
		if !strings.Contains(errStr, "preview.url_") {
			t.Errorf("preview_url=%q: error %q lacks preview.url_ prefix", bad, errStr)
		}
	}
}

// REG-CV2V2-005 (设计 ③) — kind ∉ PreviewableKinds → 400
// preview.kind_not_previewable.
func TestCV_PreviewKindNotPreviewable(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)

	// Default kind=markdown via empty type — not previewable.
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Doc",
		"body":  "# heading",
	})
	id := art["id"].(string)

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/preview", tok, map[string]any{
		"preview_url": "https://cdn.example/x.png",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("markdown preview: got %d, want 400 (%v)", resp.StatusCode, data)
	}
	errStr, _ := data["error"].(string)
	if !strings.Contains(errStr, "preview.kind_not_previewable") {
		t.Errorf("error: got %q, want 'preview.kind_not_previewable'", errStr)
	}
}

// REG-CV2V2-006 — admin (no auth user) → 401.
func TestCV_PreviewAdmin401(t *testing.T) {
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

	// No auth token → 401.
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/preview", "", map[string]any{
		"preview_url": "https://cdn.example/x-thumb.jpg",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no-token preview: got %d, want 401", resp.StatusCode)
	}
}

// REG-CV2V2-007 — preview_url persisted, GET /artifacts/:id reads it back
// (round-trip via current artifact handler — we don't yet expose preview_url
// in serializeArtifact response, but the field IS persisted; subsequent
// preview calls overwrite).
func TestCV_PreviewOverwrite(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := cv12General(t, ts.URL, tok)
	_, art := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/artifacts", tok, map[string]any{
		"title": "Hero", "type": "video_link", "body": "https://cdn.example/x.mp4",
	})
	id := art["id"].(string)

	for _, u := range []string{
		"https://cdn.example/x-thumb-v1.jpg",
		"https://cdn.example/x-thumb-v2.jpg",
	} {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/artifacts/"+id+"/preview", tok, map[string]any{
			"preview_url": u,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("preview %q: got %d (%v)", u, resp.StatusCode, data)
		}
		if data["preview_url"] != u {
			t.Errorf("preview_url echo: got %v, want %q", data["preview_url"], u)
		}
	}
}
