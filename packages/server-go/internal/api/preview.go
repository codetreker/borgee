// Package api — preview.go: CV-2 v2 server handler for artifact preview
// thumbnail / media URL recording (Phase 5).
//
// Blueprint: docs/blueprint/current/canvas-vision.md §1.4 (artifact 集合: Markdown
// / 代码片段 / image_link / video_link / pdf_link; preview 是首屏快读).
// Spec brief: docs/implementation/modules/cv-2-v2-media-preview-spec.md
// (战马D v0). 原则: 3 条 (① server CDN thumbnail 不 inline / ② HTML5
// native player 不引入 video.js / ③ kind enum 跟 CV-3 #396 共 schema 单一来源).
//
// Endpoint:
//
//	POST /api/v1/artifacts/{artifactId}/preview        owner-only generate/record preview_url
//
// 设计反查:
//   - ① owner-only ACL — channel.created_by gate (跟 CV-1.2 rollback 设计 ⑦
//     同 path; admin god-mode → 401, non-owner → 403).
//   - ② preview_url MUST be https — 反向约束 javascript: / data: / http: /
//     file: / 任何非 https scheme 全 reject (跟 ValidateImageLinkURL XSS
//     红线一致, content-lock §1).
//   - ③ kind enum 闸 — 仅 image_link / video_link / pdf_link 才能 generate
//     preview (markdown / code 走 CV-1 既有 head body 渲染, 不需 preview_url);
//     其他 kind 调此 endpoint → 400.
//
// v0 scope: this handler records a client-supplied preview_url. For example,
// an out-of-band server-side ffmpeg/ImageMagick/pdf2image worker may post the
// resulting CDN URL back here. Full CDN integration (ffmpeg first frame or
// pdf2image first page) is deferred to v1+; this PR locks only the server
// invariants for ACL, HTTPS-only URLs, and previewable artifact kinds.
package api

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"borgee-server/internal/auth"
)

// PreviewURLErrCode constants stay byte-identical with spec §0 designs ②③.
const (
	PreviewErrCodeNotOwner          = "preview.not_owner"
	PreviewErrCodeURLInvalid        = "preview.url_invalid"
	PreviewErrCodeURLNotHTTPS       = "preview.url_must_be_https"
	PreviewErrCodeKindNotPreviewable = "preview.kind_not_previewable"
	PreviewErrCodeArtifactNotFound  = "preview.artifact_not_found"
)

// PreviewableKinds is the allowlist for POST /preview artifact kinds (design ③).
// markdown / code use the existing CV-1 head/body rendering and do not need
// preview_url.
var PreviewableKinds = []string{
	ArtifactKindImageLink,
	ArtifactKindVideoLink,
	ArtifactKindPDFLink,
}

// IsPreviewableKind reports whether kind k requires a preview thumbnail
// path. It uses the same gate as PreviewableKinds; grep checks keep
// `markdown.*preview_url|code.*preview_url` at count==0 because markdown/code
// do not use preview.
func IsPreviewableKind(k string) bool {
	for _, v := range PreviewableKinds {
		if k == v {
			return true
		}
	}
	return false
}

// previewRequest is the POST body shape — server accepts a pre-computed
// thumbnail / media URL. Real CDN worker integration is deferred to v1+.
type previewRequest struct {
	PreviewURL string `json:"preview_url"`
}

// handlePreview implements POST /api/v1/artifacts/{artifactId}/preview.
//
// 反向约束守 (设计 ①②③):
//   - admin (no auth user) → 401 (admin god-mode 不入业务路径).
//   - non-owner authenticated user → 403 + preview.not_owner.
//   - artifact kind ∉ PreviewableKinds → 400 + preview.kind_not_previewable.
//   - preview_url empty / unparseable → 400 + preview.url_invalid.
//   - preview_url scheme ≠ https → 400 + preview.url_must_be_https (XSS
//     红线第一道, 跟 ValidateImageLinkURL 字面一致).
//   - artifact not found → 404 + preview.artifact_not_found.
//
// Side-effect: UPDATE artifacts SET preview_url = ? WHERE id = ?.
// 反向约束: 不发 system message (preview 是 owner action, 不污染 fanout —
// 跟 CV-1.2 rollback 设计 ⑦ "system message 不发" 同精神).
// 反向约束: 不 push WS frame (preview_url 静态 CDN, client 下次 GET
// /artifacts/:id pull 就拿到 — spec §3 不在范围 "实时刷新").
func (h *ArtifactHandler) handlePreview(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		// 设计 ① admin → 401 (跟 CV-1.2 rollback 同 path).
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	id := r.PathValue("artifactId")
	art, err := h.loadArtifact(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSONError(w, http.StatusNotFound, PreviewErrCodeArtifactNotFound+": artifact not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "load artifact failed")
		return
	}

	// 设计 ① owner = channel.created_by (跟 CV-1.2 rollback 同源).
	ownerID, err := h.channelOwnerID(art.ChannelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, PreviewErrCodeArtifactNotFound+": channel not found")
		return
	}
	if user.ID != ownerID {
		writeJSONError(w, http.StatusForbidden, PreviewErrCodeNotOwner+": only the channel owner may set preview_url")
		return
	}
	// Channel access defense-in-depth (跟 CV-1.2 既有 path).
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, PreviewErrCodeNotOwner+": forbidden")
		return
	}

	// 设计 ③ kind 闸.
	if !IsPreviewableKind(art.Type) {
		writeJSONError(w, http.StatusBadRequest,
			PreviewErrCodeKindNotPreviewable+": kind "+art.Type+" does not support preview (must be one of [image_link video_link pdf_link])")
		return
	}

	var req previewRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, PreviewErrCodeURLInvalid+": "+err.Error())
		return
	}

	// 设计 ② https-only 红线. 复用 ValidateImageLinkURL — same XSS gate.
	if err := ValidateImageLinkURL(req.PreviewURL); err != nil {
		// errInvalidImageLinkURL.Error() 已带 "artifact.invalid_url:" 前缀.
		// 我们想要 preview.url_must_be_https / preview.url_invalid 命名;
		// 简单 path: scheme-mismatch 抓 "scheme must be https" 子串, 其它
		// 全归 url_invalid.
		msg := err.Error()
		if containsHTTPSDirective(msg) {
			writeJSONError(w, http.StatusBadRequest, PreviewErrCodeURLNotHTTPS+": "+msg)
		} else {
			writeJSONError(w, http.StatusBadRequest, PreviewErrCodeURLInvalid+": "+msg)
		}
		return
	}

	// Persist.
	if err := h.Store.DB().Exec(`UPDATE artifacts SET preview_url = ? WHERE id = ?`,
		req.PreviewURL, id).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "update preview_url failed")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"artifact_id": id,
		"preview_url": req.PreviewURL,
	})
}

// containsHTTPSDirective scans an err.Error() for the literal "https"
// directive substring our validator emits ("url scheme must be https").
// 反向约束: 不引入 strings 多 import — handler 局部 helper.
func containsHTTPSDirective(s string) bool {
	const needle = "must be https"
	if len(s) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
