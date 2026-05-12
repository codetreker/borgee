// Package api — preview.go: CV-2 v2 server handler for artifact preview
// thumbnail / media URL recording (Phase 5).
//
// Blueprint: docs/blueprint/current/canvas-vision.md §1.4 (artifact collection:
// Markdown / code snippets / image_link / video_link / pdf_link; preview is the
// first-screen quick read).
// Spec brief: docs/implementation/modules/cv-2-v2-media-preview-spec.md
// principles: ① server CDN thumbnail, not inline data; ② HTML5 native player,
// no video.js; ③ kind enum shares the schema source with CV-3 #396.
//
// Endpoint:
//
//	POST /api/v1/artifacts/{artifactId}/preview        owner-only generate/record preview_url
//
// Design references:
//   - ① Owner-only ACL: channel.created_by gate, matching the CV-1.2 rollback
//     path. Admin without user auth gets 401; non-owner gets 403.
//   - ② preview_url MUST be https: reject javascript:, data:, http:, file:, and
//     any non-https scheme. This matches ValidateImageLinkURL's XSS constraint
//     and content-lock §1.
//   - ③ Kind enum gate: only image_link / video_link / pdf_link can generate a
//     preview. markdown / code use existing CV-1 head/body rendering and do not
//     need preview_url; other kinds calling this endpoint return 400.
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
	PreviewErrCodeNotOwner           = "preview.not_owner"
	PreviewErrCodeURLInvalid         = "preview.url_invalid"
	PreviewErrCodeURLNotHTTPS        = "preview.url_must_be_https"
	PreviewErrCodeKindNotPreviewable = "preview.kind_not_previewable"
	PreviewErrCodeArtifactNotFound   = "preview.artifact_not_found"
)

// PreviewableKinds is the allowlist for POST /preview artifact kinds (design ③).
// markdown / code use the existing CV-1 head/body rendering and do not need
// preview_url.
var PreviewableKinds = []string{
	ArtifactKindImageLink,
	ArtifactKindVideoLink,
	ArtifactKindPDFLink,
}

// IsPreviewableKind reports whether kind k requires a preview URL. It uses
// the same gate as PreviewableKinds; grep checks keep
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
// Constraints enforced here (designs ①②③):
//   - admin (no auth user) → 401; admin path is separate from business writes.
//   - non-owner authenticated user → 403 + preview.not_owner.
//   - artifact kind ∉ PreviewableKinds → 400 + preview.kind_not_previewable.
//   - preview_url empty / unparseable → 400 + preview.url_invalid.
//   - preview_url scheme ≠ https → 400 + preview.url_must_be_https, matching
//     ValidateImageLinkURL's first XSS constraint.
//   - artifact not found → 404 + preview.artifact_not_found.
//
// Side-effect: UPDATE artifacts SET preview_url = ? WHERE id = ?.
// Constraint: do not emit a system message. Preview is an owner action and does
// not enter fanout, matching CV-1.2 rollback design ⑦.
// Constraint: do not push a WS frame. preview_url is a static CDN URL and the
// client gets it on the next GET /artifacts/:id; realtime refresh is outside
// spec §3.
func (h *ArtifactHandler) handlePreview(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		// Design ①: admin without user auth gets 401, matching the CV-1.2 rollback path.
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

	// Design ①: owner = channel.created_by, shared with CV-1.2 rollback.
	ownerID, err := h.channelOwnerID(art.ChannelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, PreviewErrCodeArtifactNotFound+": channel not found")
		return
	}
	if user.ID != ownerID {
		writeJSONError(w, http.StatusForbidden, PreviewErrCodeNotOwner+": only the channel owner may set preview_url")
		return
	}
	// Channel access defense-in-depth, matching the existing CV-1.2 path.
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, PreviewErrCodeNotOwner+": forbidden")
		return
	}

	// Design ③: kind gate.
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

	// Design ②: https-only constraint. Reuse ValidateImageLinkURL's XSS gate.
	if err := ValidateImageLinkURL(req.PreviewURL); err != nil {
		// errInvalidImageLinkURL.Error() already includes the "artifact.invalid_url:"
		// prefix. This endpoint maps it to preview.url_must_be_https or
		// preview.url_invalid.
		// Simple path: scheme mismatch is identified by the "scheme must be https"
		// substring; all other validator errors map to url_invalid.
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
// Constraint: avoid adding a strings import for this handler-local helper.
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
