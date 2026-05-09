// Package api — thumbnail.go: CV-3 v2 server handler for code/markdown
// artifact thumbnail recording (Phase 5+, #cv-3-v2).
//
// Blueprint: docs/blueprint/current/canvas-vision.md §1.4 (artifact 集合: 多类型,
// "首屏快读不是浏览器内全量解码" 字面). Spec brief:
// docs/implementation/modules/cv-3-v2-spec.md (战马C v0, 484ec08).
// 原则: 3 条 (① server CDN thumbnail 不 inline / ② https only 复用
// ValidateImageLinkURL / ③ ThumbnailableKinds 跟 PreviewableKinds 二端
// 互斥 + thumbnail_url 跟 preview_url 字段拆).
//
// Endpoint:
//
//	POST /api/v1/artifacts/{artifactId}/thumbnail   owner-only generate/record thumbnail_url
//
// 设计反查:
//   - ① owner-only ACL — channel.created_by gate (跟 CV-1.2 rollback +
//     CV-2 v2 preview.go 设计 ① 同 path; admin god-mode → 401, non-owner
//     → 403).
//   - ② thumbnail_url MUST be https — 复用 ValidateImageLinkURL XSS 红线
//     单一来源 (跟 CV-2 v2 preview.go 设计 ② 同 helper).
//   - ③ kind 闸 — 仅 markdown / code 才能 generate thumbnail (二端互斥
//     跟 PreviewableKinds — image_link/video_link/pdf_link 走 CV-2 v2
//     既有 preview 路径); 其他 kind 调此 endpoint → 400.

// v0 原则: 此 handler 是 thin recording shim — accepts client/worker-
// supplied thumbnail_url (跟 CV-2 v2 preview.go 同精神 — 真 CDN worker
// 集成 syntax-highlight render 留 v1+).

// 跨 milestone byte-identical 锁定: thumbnail_url 跟 preview_url 字段拆;
// ThumbnailableKinds 跟 PreviewableKinds 二端互斥; 5 错码字面单一来源 (跟
// PreviewErrCode* + AP-1/AP-2/AP-3 const 同模式).
package api

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"borgee-server/internal/auth"
)

// ThumbnailURLErrCode constants — byte-identical 跟 cv-3-v2-spec.md §0
// 设计 ②③ + 跟 PreviewErrCode* 同模式.
const (
	ThumbnailErrCodeNotOwner            = "thumbnail.not_owner"
	ThumbnailErrCodeURLInvalid          = "thumbnail.url_invalid"
	ThumbnailErrCodeURLNotHTTPS         = "thumbnail.url_must_be_https"
	ThumbnailErrCodeKindNotThumbnailable = "thumbnail.kind_not_thumbnailable"
	ThumbnailErrCodeArtifactNotFound    = "thumbnail.artifact_not_found"
)

// ThumbnailableKinds 是允许调 POST /thumbnail 的 kind 白名单 (设计 ③
// 二端互斥 — 跟 PreviewableKinds [image/video/pdf] 互斥; markdown/code
// 是 text kind, 走 thumbnail; image/video/pdf 是 media kind, 走 preview).
var ThumbnailableKinds = []string{
	ArtifactKindMarkdown,
	ArtifactKindCode,
}

// IsThumbnailableKind reports whether kind k requires a thumbnail path.
// 跟 ThumbnailableKinds slice 共闸; 跟 IsPreviewableKind 互斥单测锁定
// (TestCV3V22_ThumbnailableVsPreviewableMutuallyExclusive 守).
func IsThumbnailableKind(k string) bool {
	for _, v := range ThumbnailableKinds {
		if k == v {
			return true
		}
	}
	return false
}

// thumbnailRequest is the POST body shape — server accepts a pre-computed
// thumbnail URL (跟 previewRequest 同模式 thin recording shim).
type thumbnailRequest struct {
	ThumbnailURL string `json:"thumbnail_url"`
}

// handleThumbnail implements POST /api/v1/artifacts/{artifactId}/thumbnail.
//
// 反向约束守 (设计 ①②③):
//   - admin (no auth user) → 401 (admin god-mode 不入业务路径).
//   - non-owner authenticated user → 403 + thumbnail.not_owner.
//   - artifact kind ∉ ThumbnailableKinds → 400 + thumbnail.kind_not_thumbnailable
//     (二端互斥 — image_link/video_link/pdf_link 走 CV-2 v2 preview 路径).
//   - thumbnail_url empty / unparseable → 400 + thumbnail.url_invalid.
//   - thumbnail_url scheme ≠ https → 400 + thumbnail.url_must_be_https
//     (XSS 红线第一道, 复用 ValidateImageLinkURL 同源).
//   - artifact not found → 404 + thumbnail.artifact_not_found.
//
// Side-effect: UPDATE artifacts SET thumbnail_url = ? WHERE id = ?.
// 反向约束: 不发 system message + 不 push WS frame (跟 CV-2 v2 preview.go
// 设计 同精神 — thumbnail 静态 CDN, client 下次 GET pull 拿到).
func (h *ArtifactHandler) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		// 设计 ① admin → 401 (跟 CV-1.2 rollback + CV-2 v2 preview.go 同 path).
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	id := r.PathValue("artifactId")
	art, err := h.loadArtifact(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSONError(w, http.StatusNotFound, ThumbnailErrCodeArtifactNotFound+": artifact not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "load artifact failed")
		return
	}

	// 设计 ① owner = channel.created_by (跟 CV-1.2 rollback + CV-2 v2 同源).
	ownerID, err := h.channelOwnerID(art.ChannelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, ThumbnailErrCodeArtifactNotFound+": channel not found")
		return
	}
	if user.ID != ownerID {
		writeJSONError(w, http.StatusForbidden, ThumbnailErrCodeNotOwner+": only the channel owner may set thumbnail_url")
		return
	}
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, ThumbnailErrCodeNotOwner+": forbidden")
		return
	}

	// 设计 ③ kind 闸 (二端互斥 — markdown/code 走 thumbnail, 其他走 preview).
	if !IsThumbnailableKind(art.Type) {
		writeJSONError(w, http.StatusBadRequest,
			ThumbnailErrCodeKindNotThumbnailable+": kind "+art.Type+" does not support thumbnail (must be one of [markdown code]; image_link/video_link/pdf_link 走 /preview 路径)")
		return
	}

	var req thumbnailRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, ThumbnailErrCodeURLInvalid+": "+err.Error())
		return
	}

	// 设计 ② https-only 红线 — 复用 ValidateImageLinkURL (XSS 红线单一来源,
	// 跟 CV-2 v2 preview.go 同 helper).
	if err := ValidateImageLinkURL(req.ThumbnailURL); err != nil {
		msg := err.Error()
		if containsHTTPSDirective(msg) {
			writeJSONError(w, http.StatusBadRequest, ThumbnailErrCodeURLNotHTTPS+": "+msg)
		} else {
			writeJSONError(w, http.StatusBadRequest, ThumbnailErrCodeURLInvalid+": "+msg)
		}
		return
	}

	// Persist.
	if err := h.Store.DB().Exec(`UPDATE artifacts SET thumbnail_url = ? WHERE id = ?`,
		req.ThumbnailURL, id).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "update thumbnail_url failed")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"artifact_id":   id,
		"thumbnail_url": req.ThumbnailURL,
	})
}
