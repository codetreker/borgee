package api

import (
	"net/url"
	"strings"
)

// ----- CV-3.2 (#363 v1 / #397 patch) artifact kind validation -----
//
// Blueprint出处: docs/blueprint/current/canvas-vision.md §1.4 (artifact 集合:
// Markdown / 代码片段带语言标注 / 设计稿图片或链接 / 看板 v2+) + §2
// v1 不做清单. Spec: docs/implementation/modules/cv-3-spec.md (飞马
// #363 v0 → #397 v1 后续) §0 设计 ①②③ + §1 拆段 CV-3.2 +
// acceptance docs/qa/acceptance-templates/cv-3.md (烈马 #376) §1.3 / §1.4.
// Content lock: docs/qa/cv-3-content-lock.md (野马 #370) §1 ②④⑤ +
// §2 grep 检查 (XSS 红线两道闸).
//
// Three validation seams locked here:
//
//  1. ArtifactKind enum — 'markdown' (CV-1 既有) / 'code' / 'image_link'
//     (CV-3.1 #396 schema CHECK 已锁定 — server 端镜像 enum 用于 fast-fail
//     400 在 schema 拒前).
//  2. Code metadata.language ∈ 11 项白名单 + 'text' fallback (12 项
//     合法). 短码唯一 (#370 §1 ② 反向约束 — 不接 'golang' / 'typescript' /
//     'python' / 'shell' / 'bash' / 'plaintext' 全名同义词).
//  3. Image/link metadata.kind ∈ ('image','link') + URL **必 https**
//     (XSS 红线第一道, #370 §1 ④ + spec §3 grep 检查项 javascript: /
//     data:image / http: 全 reject).
//
// 反向约束:
//   - 不引入 schema 改 (本 PR 仅 server validation; metadata 持久化
//     由 CV-3.2 schema 后续 PR 决定 — 加 metadata TEXT NULL 列
//     vs body 头部 JSON embedding, 等飞马 spec 后续 拍). 此 PR
//     下: metadata 走请求-时验, 不持久化, 由客户端 reload 后按 kind
//     默认 (code 默认 'text', image_link 默认无 metadata 因 body 即 URL).
//     PR body Acceptance 段已明示此留作后续.
//   - 不删 ArtifactType 常量 (CV-1 既有 'markdown' 写路径仍引, 删会
//     扫飞 9 处 call-site, 留 CV-3 全闭 audit 一次性删 — 本 PR
//     仅扩 enum 不缩老路径).

// ArtifactKindMarkdown / ArtifactKindCode / ArtifactKindImageLink — CV-3
// 三态 enum, byte-identical 跟 cv_3_1_artifact_kinds.go schema CHECK
// + cv-3-content-lock.md §1 ① + spec §0 设计 ①.
// ArtifactKindVideoLink / ArtifactKindPDFLink — CV-2 v2 #cv-2-v2 扩 5 项,
// byte-identical 跟 cv_2_v2_media_preview.go schema CHECK 同源.
const (
	ArtifactKindMarkdown  = "markdown"
	ArtifactKindCode      = "code"
	ArtifactKindImageLink = "image_link"
	ArtifactKindVideoLink = "video_link"
	ArtifactKindPDFLink   = "pdf_link"
)

// ValidArtifactKinds is the closed enum the server accepts at request
// time. Mirrors the schema CHECK constraint installed by migration v=17
// (cv_3_1_artifact_kinds) + extended by v=27 (cv_2_v2_media_preview).
// Drift between this slice and the migration CHECKs is caught by the
// server validation tests + the schema test.
var ValidArtifactKinds = []string{
	ArtifactKindMarkdown,
	ArtifactKindCode,
	ArtifactKindImageLink,
	ArtifactKindVideoLink,
	ArtifactKindPDFLink,
}

// IsValidArtifactKind reports whether k is one of the three accepted
// kinds. Empty string returns false (caller must default to 'markdown'
// before calling,跟 CV-1 既有 default 路径保持兼容).
func IsValidArtifactKind(k string) bool {
	for _, v := range ValidArtifactKinds {
		if k == v {
			return true
		}
	}
	return false
}

// ValidCodeLanguages is the 11 项 code-language whitelist + 'text'
// fallback (12 项), byte-identical 跟 cv-3-content-lock.md §1 ② 同源.
// 短码唯一 — 反向约束: 不接 'golang' / 'typescript' / 'python' / 'shell' /
// 'bash' / 'plaintext' 全名同义词 (#370 §1 ② 字面禁 + spec §3 grep 检查
// 出处 4.4).
var ValidCodeLanguages = []string{
	"go", "ts", "js", "py", "md", "sh",
	"sql", "yaml", "json", "html", "css",
	"text", // fallback (12 项)
}

// IsValidCodeLanguage reports whether lang is one of the 12 accepted
// short codes. Drift between this slice and the client CodeRenderer
// LANG_LABEL map (CV-3.2 client PR) is caught by:
//   - vitest table-driven (#370 §1 ②)
//   - reverse grep `'golang'|'typescript'|'python'|'shell'|'bash'|'plaintext'`
//     count==0 (#370 §2 + acceptance §4.4)
func IsValidCodeLanguage(lang string) bool {
	for _, v := range ValidCodeLanguages {
		if lang == v {
			return true
		}
	}
	return false
}

// ValidImageLinkKinds is the binary 'image' | 'link' enum that branches
// the ImageLinkRenderer (CV-3.2 client). 'image' → <img>, 'link' → <a>.
// 反向约束: 不开第三态 (跟 #370 §1 ④⑤ 二元真分清同源).
var ValidImageLinkKinds = []string{"image", "link"}

// IsValidImageLinkKind reports whether k is one of the two accepted
// image_link sub-kinds. Empty string returns false; missing metadata.kind
// maps to HTTP 400 (acceptance §1.4).
func IsValidImageLinkKind(k string) bool {
	for _, v := range ValidImageLinkKinds {
		if k == v {
			return true
		}
	}
	return false
}

// ValidateImageLinkURL parses rawURL and returns nil iff it's a syntactically
// valid absolute https URL. Constraint (first XSS gate, #370 §1 ④ +
// content-lock §2 grep checks + spec §3): javascript: / data: / data:image /
// http: / file: / chrome: / any non-HTTPS scheme must be rejected. Also rejects
// non-absolute / empty / scheme-relative URLs (`//host/path`) which would
// inherit the page's scheme and bypass the gate.
//
// Note: we DO NOT do DNS / reachability / file-extension checks here —
// that's a Phase 5+ network policy concern. Scheme-only is the security
// invariant; everything else is render-time concern.
func ValidateImageLinkURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return errInvalidImageLinkURL("url is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return errInvalidImageLinkURL("url unparseable")
	}
	// Require an absolute URL with both scheme and host. url.Parse accepts
	// scheme-relative forms such as `//host` with Scheme=="" and Host!="";
	// reject them explicitly.
	if u.Scheme == "" {
		return errInvalidImageLinkURL("url scheme is required")
	}
	// Enforce HTTPS only. EqualFold allows case variants such as 'HTTPS'
	// because RFC 3986 schemes are ASCII-case-insensitive.
	if !strings.EqualFold(u.Scheme, "https") {
		return errInvalidImageLinkURL("url scheme must be https")
	}
	if u.Host == "" {
		return errInvalidImageLinkURL("url host is required")
	}
	return nil
}

// errInvalidImageLinkURL wraps a reason in a stable error type so
// callers (handleCreate) can map all to HTTP 400 with the same error
// code `artifact.invalid_url` (acceptance §1.4 + spec §0 设计 ③ XSS
// 红线一致 4xx code).
type errInvalidImageLinkURL string

func (e errInvalidImageLinkURL) Error() string {
	return "artifact.invalid_url: " + string(e)
}

// ArtifactMetadata is the request-time validation shape — NOT a
// persisted struct. Per CV-3.2 server-only PR scope: validate then
// discard (留作后续 metadata persistence by schema — column
// add vs body JSON header embedding, 飞马 spec 后续 拍).
//
// Layout matches spec §0 ① 字面: code 头部 JSON `{language}` /
// image_link 头部 JSON `{kind, url}` + (optional) `{thumbnail_url}`.
//
// JSON tags use omitempty so missing fields surface as zero values,
// which the per-kind validator below distinguishes from explicit ""
// (empty-but-present) — both fail validation, the error message hints
// which case fired.
type ArtifactMetadata struct {
	// Code-only.
	Language string `json:"language,omitempty"`

	// Image_link-only.
	Kind         string `json:"kind,omitempty"`
	URL          string `json:"url,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

// ValidateArtifactMetadata applies the per-kind metadata contract
// from spec §0 ① + acceptance §1.3 / §1.4. Returns:
//   - nil iff the metadata satisfies the kind's contract.
//   - error with stable prefix `artifact.invalid_language` /
//     `artifact.invalid_image_link_kind` / `artifact.invalid_url`
//     so handler maps to the same 400 error code (跟 spec §3 反向
//     grep 出处 4xx 同源).
//
// Markdown kind: metadata is currently optional (no per-kind contract
// yet — CV-1 v1 path). Future: maybe `outline_level` etc; not in CV-3.2
// 范围.
func ValidateArtifactMetadata(kind string, m ArtifactMetadata) error {
	switch kind {
	case ArtifactKindMarkdown:
		// CV-1 v1 path: no metadata contract.
		return nil

	case ArtifactKindCode:
		// 设计 ① + acceptance §1.3 — code MUST carry language.
		if m.Language == "" {
			return errInvalidLanguage("metadata.language is required for kind='code'")
		}
		if !IsValidCodeLanguage(m.Language) {
			return errInvalidLanguage(
				"metadata.language must be one of [go ts js py md sh sql yaml json html css text]",
			)
		}
		return nil

	case ArtifactKindImageLink:
		// 设计 ① + acceptance §1.4 — image_link MUST carry kind + url.
		if m.Kind == "" {
			return errInvalidImageLinkKind("metadata.kind is required for kind='image_link'")
		}
		if !IsValidImageLinkKind(m.Kind) {
			return errInvalidImageLinkKind("metadata.kind must be 'image' or 'link'")
		}
		if err := ValidateImageLinkURL(m.URL); err != nil {
			return err
		}
		// thumbnail_url is optional; if provided, it MUST also be https
		// (反向约束: 防 thumbnail-XSS leak via the same vector).
		if m.ThumbnailURL != "" {
			if err := ValidateImageLinkURL(m.ThumbnailURL); err != nil {
				return errInvalidImageLinkURL("metadata.thumbnail_url: " + err.Error())
			}
		}
		return nil

	case ArtifactKindVideoLink, ArtifactKindPDFLink:
		// CV-2 v2 设计 ① + ② — body=URL (跟 image_link 同精神, body 即 URL),
		// preview_url generated lazily by POST /artifacts/:id/preview owner-only.
		// metadata 当前无 per-kind contract (留作后续 v3+ 加 'duration'/'pages' 等);
		// URL 合法性 validated at preview generation time (https-only 红线
		// 跟 ValidateImageLinkURL 同源).
		return nil

	default:
		// Should be caught earlier by IsValidArtifactKind, but fail-loud
		// if a future caller forgets the kind gate.
		return errInvalidArtifactKind(kind)
	}
}

type errInvalidLanguage string

func (e errInvalidLanguage) Error() string { return "artifact.invalid_language: " + string(e) }

type errInvalidImageLinkKind string

func (e errInvalidImageLinkKind) Error() string {
	return "artifact.invalid_image_link_kind: " + string(e)
}

type errInvalidArtifactKind string

func (e errInvalidArtifactKind) Error() string {
	return "artifact.invalid_kind: " + string(e)
}
