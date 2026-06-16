package store

import "strings"

// IsAllowedImageContentURL reports whether content (an image-typed message
// body) is a URL the client may safely render as an <img src>/<a href>.
//
// borgee #1108 finding F5: content_type=image previously accepted any
// arbitrary string. There is NO server-side fetch (no SSRF), so this is a
// client-render-only concern — a stored "javascript:alert(1)" or
// "data:text/html,<script>" turns into a phishing / inert-anchor vector when
// the message list renders it. The WS send rail and the REST create rail both
// call this at write time so they agree, and the client carries the same
// guard for already-stored rows.
//
// Allowlist (identical to the client guard in MessageItem.tsx ImageContent):
//   - starts with http:// or https:// (case-insensitive), OR
//   - a same-origin relative path starting with a single '/' but NOT '//'
//     (protocol-relative `//host/path` inherits the page scheme and is unsafe).
//
// The content is trimmed first to match the trim handlers do before persisting.
func IsAllowedImageContentURL(content string) bool {
	s := strings.TrimSpace(content)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	// Leading single slash (same-origin relative) but not protocol-relative.
	if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") {
		return true
	}
	return false
}
