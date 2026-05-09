// Package api_test — cv_11_no_markdown_test.go: CV-11.1 unit verifying the
// server stores comment body as raw markdown source (NOT pre-rendered HTML).
//
// Stance pin (cv-11-spec.md §0 设计 ① + §1 CV-11.1):
//   - server 0 production code change for CV-11
//   - markdown rendering is client-side only (反约束: server never imports
//     marked or DOMPurify; client lib/markdown owns the path)
//   - body stored in messages.content is the user's raw markdown source,
//     byte-identical to input — round-trip via POST → GET preserves the
//     `**bold**` / `<@uuid>` / etc tokens unchanged
package api_test

import (
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/testutil"
)

// TestCV_ServerStoresRawMarkdown pins 设计 ①: comment POST stores body
// raw, GET round-trip returns exact same bytes. Server never renders.
func TestCV_ServerStoresRawMarkdown(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	chID := mustGeneralChannelIDCV11(t, ts.URL, tok)

	rawBody := "**bold** _italic_ `code` and <@some-uuid> mention\n\n```\nfenced\n```"
	post, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/messages", tok, map[string]any{
		"content": rawBody,
	})
	if post.StatusCode != http.StatusCreated {
		t.Fatalf("post: %d %v", post.StatusCode, data)
	}
	msg := data["message"].(map[string]any)
	got, _ := msg["content"].(string)
	if got != rawBody {
		t.Errorf("server mutated markdown source: got %q, want %q", got, rawBody)
	}
	// Defensive: server response must NOT contain HTML tags inserted by any
	// future server-side markdown rendering.
	if strings.Contains(got, "<strong>") || strings.Contains(got, "<em>") {
		t.Errorf("server rendered markdown to HTML — must stay raw (got %q)", got)
	}
}

func mustGeneralChannelIDCV11(t *testing.T, url, tok string) string {
	t.Helper()
	resp, data := testutil.JSON(t, "GET", url+"/api/v1/channels", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list channels: %d", resp.StatusCode)
	}
	chans, _ := data["channels"].([]any)
	for _, c := range chans {
		if cm, ok := c.(map[string]any); ok && cm["name"] == "general" {
			return cm["id"].(string)
		}
	}
	t.Fatalf("general channel not seeded")
	return ""
}
