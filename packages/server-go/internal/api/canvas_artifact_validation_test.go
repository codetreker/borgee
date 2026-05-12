// Package api — cv_3_2_artifact_validation_test.go: CV-3.2 (#363/#397)
// server validation acceptance tests.
//
// Covered checks:
//   - acceptance §1.3 — code MUST carry metadata.language ∈ 11 allowed values
//   - 'text' fallback (12 values); short-code uniqueness rejects full-name synonyms
//     ('golang'/'typescript'/'python'/'shell'/'bash'/'plaintext') (#370 §1 第 2 条).
//   - acceptance §1.4 — image_link MUST carry metadata.kind ∈ ('image',
//     'link') + URL must be https; javascript:/data:/data:image/http:/file:
//     all reject as the first XSS guard (#370 §1 第 4 条).
//   - constraint — old CV-1.2 400 text 'type must be markdown (v1)' was removed
//     (spec #397 mismatch 3 literal).
package api

import (
	"strings"
	"testing"
)

// ---- IsValidArtifactKind / enum (acceptance §1.1 mirror) -----------

func TestIsValidArtifactKind(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"markdown", "code", "image_link"} {
		if !IsValidArtifactKind(k) {
			t.Errorf("kind=%q rejected — should be in enum", k)
		}
	}
	for _, bad := range []string{"", "pdf", "kanban", "mindmap", "MARKDOWN", "code_image", "imageLink"} {
		if IsValidArtifactKind(bad) {
			t.Errorf("kind=%q accepted — should NOT be in enum", bad)
		}
	}
}

// ---- IsValidCodeLanguage 11 values + text (#370 §1 第 2 条) ---------------

func TestIsValidCodeLanguage_11WhitelistPlusText(t *testing.T) {
	t.Parallel()
	want := []string{"go", "ts", "js", "py", "md", "sh", "sql", "yaml", "json", "html", "css", "text"}
	if len(ValidCodeLanguages) != 12 {
		t.Fatalf("ValidCodeLanguages length: got %d, want 12 (11 项允许集合 + text fallback)", len(ValidCodeLanguages))
	}
	for _, lang := range want {
		if !IsValidCodeLanguage(lang) {
			t.Errorf("language=%q rejected — should be in 12 项允许集合", lang)
		}
	}
}

// TestIsValidCodeLanguage_RejectsFullNameSynonyms verifies #370 §1 第 2 条:
// short-code uniqueness rejects full-name synonyms ('golang' / 'typescript' /
// 'python' / 'shell' / 'bash' / 'plaintext').
func TestIsValidCodeLanguage_RejectsFullNameSynonyms(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		// Full-name synonyms rejected by #370 §1 第 2 条.
		"golang", "typescript", "python", "shell", "bash", "plaintext",
		// Case variants — short codes are ASCII case-sensitive.
		"GO", "TS", "Py", "MD",
		// Languages outside the spec are rejected to keep the allowed set narrow.
		"rust", "c", "cpp", "java", "kotlin", "swift", "ruby", "php",
		"yml", // 'yaml' 已收, 'yml' 不接 (短码唯一)
		// 空字串.
		"",
	} {
		if IsValidCodeLanguage(bad) {
			t.Errorf("language=%q accepted — should NOT be in allowed language set (#370 §1 第 2 条 短码唯一)", bad)
		}
	}
}

// ---- ValidateImageLinkURL — first XSS guard (#370 §1 第 4 条) ------------

func TestValidateImageLinkURL_AcceptsHttpsAbsolute(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{
		"https://example.com/foo.png",
		"https://cdn.example.com/path/to/image.jpg",
		"https://example.com/",
		"https://EXAMPLE.com/x", // host case is irrelevant — DNS handles it
		"HTTPS://example.com/x", // scheme case-insensitive (RFC 3986)
	} {
		if err := ValidateImageLinkURL(ok); err != nil {
			t.Errorf("url=%q rejected: %v", ok, err)
		}
	}
}

// TestValidateImageLinkURL_RejectsNonHttpsSchemes verifies the first XSS guard
// in #370 §1 第 4 条: javascript: / data: / data:image / http: / file: /
// chrome: all reject.
func TestValidateImageLinkURL_RejectsNonHttpsSchemes(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		// XSS vectors.
		"javascript:alert(1)",
		"javascript://example.com/a",
		"data:image/png;base64,AAA",
		"data:text/html,<script>",
		// Mixed-content downgrade — http: rejects.
		"http://example.com/img.png",
		"http://insecure.test/",
		// File scheme — local resource access risk.
		"file:///etc/passwd",
		"file://share/x",
		// Chrome / browser-internal schemes.
		"chrome://settings",
		"chrome-extension://abcdef/x",
		// FTP — out of scope for this phase.
		"ftp://files.example.com/x.zip",
	} {
		if err := ValidateImageLinkURL(bad); err == nil {
			t.Errorf("url=%q accepted — XSS 红线检查失败 (#370 §1 第 4 条)", bad)
		}
	}
}

func TestValidateImageLinkURL_RejectsMalformed(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		"",                // empty
		"  ",              // whitespace
		"example.com/x",   // no scheme
		"//example.com/x", // scheme-relative — inherits page's scheme, bypasses gate
		"https://",        // no host
		"https:///nohost", // empty host
	} {
		if err := ValidateImageLinkURL(bad); err == nil {
			t.Errorf("url=%q accepted — malformed should reject", bad)
		}
	}
}

// ---- ValidateArtifactMetadata per-kind contract --------------------

// TestValidateArtifactMetadata_Markdown_NoContract 验证 CV-1 v1 path —
// markdown kind has no per-kind metadata contract; empty / arbitrary
// metadata both pass (forward-compat for future markdown-specific fields).
func TestValidateArtifactMetadata_Markdown_NoContract(t *testing.T) {
	t.Parallel()
	if err := ValidateArtifactMetadata(ArtifactKindMarkdown, ArtifactMetadata{}); err != nil {
		t.Errorf("markdown empty metadata rejected: %v", err)
	}
	if err := ValidateArtifactMetadata(ArtifactKindMarkdown, ArtifactMetadata{Language: "ignored"}); err != nil {
		t.Errorf("markdown stray metadata rejected: %v", err)
	}
}

// TestValidateArtifactMetadata_Code_RequiresLanguage 验证 acceptance §1.3 —
// code MUST carry metadata.language; missing → 400 artifact.invalid_language.
func TestValidateArtifactMetadata_Code_RequiresLanguage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		meta    ArtifactMetadata
		wantOK  bool
		wantSub string // substring expected in error.Error() if !wantOK
	}{
		{"missing language", ArtifactMetadata{}, false, "language is required"},
		{"empty language", ArtifactMetadata{Language: ""}, false, "language is required"},
		{"go OK", ArtifactMetadata{Language: "go"}, true, ""},
		{"text fallback OK", ArtifactMetadata{Language: "text"}, true, ""},
		{"golang full-name reject", ArtifactMetadata{Language: "golang"}, false, "must be one of"},
		{"typescript reject", ArtifactMetadata{Language: "typescript"}, false, "must be one of"},
		{"rust outside allowed set", ArtifactMetadata{Language: "rust"}, false, "must be one of"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateArtifactMetadata(ArtifactKindCode, tc.meta)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("expected OK, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.HasPrefix(err.Error(), "artifact.invalid_language:") {
				t.Errorf("error code prefix wrong: %v", err)
			}
			if tc.wantSub != "" && !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error substring mismatch: got %q want substring %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestValidateArtifactMetadata_ImageLink_RequiresHttpsOnly 验证
// acceptance §1.4 + #370 §1 第 4 条 — image_link MUST carry metadata.kind ∈
// ('image','link') + URL 必 https.
func TestValidateArtifactMetadata_ImageLink_RequiresHttpsOnly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		meta      ArtifactMetadata
		wantOK    bool
		wantCode  string // error prefix
		wantSub   string // error substring
	}{
		{"image https OK", ArtifactMetadata{Kind: "image", URL: "https://example.com/x.png"}, true, "", ""},
		{"link https OK", ArtifactMetadata{Kind: "link", URL: "https://example.com/page"}, true, "", ""},
		{"missing kind",
			ArtifactMetadata{URL: "https://example.com/x"},
			false, "artifact.invalid_image_link_kind:", "kind is required"},
		{"empty kind",
			ArtifactMetadata{Kind: "", URL: "https://example.com/x"},
			false, "artifact.invalid_image_link_kind:", "kind is required"},
		{"invalid kind 'video'",
			ArtifactMetadata{Kind: "video", URL: "https://example.com/x"},
			false, "artifact.invalid_image_link_kind:", "must be 'image' or 'link'"},
		// XSS reject branches.
		{"javascript:",
			ArtifactMetadata{Kind: "image", URL: "javascript:alert(1)"},
			false, "artifact.invalid_url:", "https"},
		{"data:image",
			ArtifactMetadata{Kind: "image", URL: "data:image/png;base64,AAA"},
			false, "artifact.invalid_url:", "https"},
		{"http: mixed content",
			ArtifactMetadata{Kind: "image", URL: "http://example.com/x"},
			false, "artifact.invalid_url:", "https"},
		// Thumbnail must also gate.
		{"thumbnail javascript:",
			ArtifactMetadata{Kind: "image", URL: "https://ok.com/x", ThumbnailURL: "javascript:alert(2)"},
			false, "artifact.invalid_url:", "thumbnail_url"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateArtifactMetadata(ArtifactKindImageLink, tc.meta)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("expected OK, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q...%q, got nil", tc.wantCode, tc.wantSub)
			}
			if !strings.HasPrefix(err.Error(), tc.wantCode) {
				t.Errorf("error code prefix mismatch: got %q want %q", err.Error(), tc.wantCode)
			}
			if tc.wantSub != "" && !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error substring mismatch: got %q want substring %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestValidateArtifactMetadata_UnknownKind 验证未知类型必须显式报错 — if a
// future caller forgets the IsValidArtifactKind gate, ValidateArtifactMetadata
// MUST surface the unknown kind rather than silently passing.
func TestValidateArtifactMetadata_UnknownKind(t *testing.T) {
	t.Parallel()
	if err := ValidateArtifactMetadata("pdf", ArtifactMetadata{}); err == nil {
		t.Error("unknown kind 'pdf' accepted — should fail explicitly")
	}
}
