package store

import "testing"

// borgee #1108 F5: IsAllowedImageContentURL allowlist — http(s):// (any case)
// OR a single-leading-slash same-origin path; reject everything else
// (javascript:/data:/protocol-relative/bare-relative/empty). The WS + REST
// rails both gate image content_type on this, so the table is the contract.
func TestIsAllowedImageContentURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		// allowed
		{"https://example.com/x.png", true},
		{"http://example.com/x.png", true},
		{"HTTPS://example.com/x.png", true},
		{"HtTp://example.com/x.png", true},
		{"/api/uploads/x.png", true},
		{"  https://example.com/x.png  ", true}, // trimmed
		// rejected
		{"javascript:alert(1)", false},
		{"data:text/html,<script>alert(1)</script>", false},
		{"data:image/png;base64,AAAA", false},
		{"//evil.com/x.png", false}, // protocol-relative
		{"image.png", false},        // bare relative, no leading slash
		{"ftp://example.com/x.png", false},
		{"file:///etc/passwd", false},
		{"", false},
		{"   ", false},
	}
	for _, c := range cases {
		if got := IsAllowedImageContentURL(c.in); got != c.want {
			t.Errorf("IsAllowedImageContentURL(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
