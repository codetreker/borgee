// Package api — cv_15_comment_edit_history.go: CV-15 artifact comment
// edit history audit GET endpoints.
//
// Blueprint: canvas-vision.md §1.4 artifact 集合 + dm-model.md §3
// forward-only audit. Spec: docs/implementation/modules/cv-15-spec.md
// (战马C v0).
//
// Architecture: artifact comments live in the messages table with
// content_type='artifact_comment' (CV-5 #530 设计 ① — comments 走 messages
// 表单源 不裂表). messages.edit_history column is provided by DM-7.1 v=34
// migration; CV-7 #535 既有 PATCH /api/v1/messages/{id} → UpdateMessage
// SSOT (DM-7.2) → artifact comments 编辑早已自动产生 edit_history JSON.
//
// CV-15 is therefore **0 schema 改 + GET endpoint scoped** — exposes the
// existing edit_history JSON to the comment author (user-rail sender-only)
// and admin (admin-rail readonly), filtered to content_type='artifact_comment'
// to avoid confusion with DM-7's generic /messages/{id}/edit-history.
//
// Public surface:
//   - CanvasCommentEditHistoryHandler{Store, Logger}
//   - (h *CanvasCommentEditHistoryHandler) RegisterUserRoutes(mux, authMw)
//   - (h *CanvasCommentEditHistoryHandler) RegisterAdminRoutes(mux, adminMw)
//
// 反约束 (cv-15-spec.md §0):
//   - 设计 ① 0 schema — 复用 messages.edit_history (DM-7.1 v=34); 反向
//     grep `cv_15_\d+|artifact_comments` 在 migrations/ 0 hit.
//   - 设计 ② owner-only sender + admin readonly; admin god-mode 不挂
//     PATCH/DELETE/PUT (ADM-0 §1.3 红线).
//   - 设计 ③ content_type='artifact_comment' filter 强制 — 非 artifact
//     comment 调本 endpoint → 404 (跟 DM-7 既有 /edit-history 区分).
package api

import (
	"log/slog"
	"net/http"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"
)

// 3 错码字面单源 (跟 DM-7 / DM-8 / CHN-15 / AL-9 / CV-6 const 同模式).
// 改 = 改三处: server const + client COMMENT_EDIT_HISTORY_ERR_TOAST +
// content-lock §3.
const (
	CommentEditHistoryErrCodeNotArtifactComment = "comment.not_artifact_comment"
	CommentEditHistoryErrCodeNotOwner           = "comment.not_owner"
	CommentEditHistoryErrCodeMessageNotFound    = "comment.message_not_found"
)

// CanvasCommentEditHistoryHandler hosts the user-rail and admin-rail GET
// endpoints for artifact comment edit history. user-rail is sender-only
// (the comment author); admin-rail is readonly only (no PATCH/DELETE —
// admin god-mode 不挂, ADM-0 §1.3 红线).
type CanvasCommentEditHistoryHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterUserRoutes wires GET /api/v1/channels/{channelId}/messages/
// {messageId}/comment-edit-history behind authMw. user-rail sender-only
// (设计 ② owner-only ACL 锁链第 22 处).
func (h *CanvasCommentEditHistoryHandler) RegisterUserRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/channels/{channelId}/messages/{messageId}/comment-edit-history",
		authMw(http.HandlerFunc(h.handleUserGet)))
}

// RegisterAdminRoutes wires GET /admin-api/v1/messages/{messageId}/comment-edit-history
// behind adminMw. admin readonly — no PATCH/DELETE/PUT on this path
// (grep 守门; admin god-mode ADM-0 §1.3 红线).
func (h *CanvasCommentEditHistoryHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("GET /admin-api/v1/messages/{messageId}/comment-edit-history",
		adminMw(http.HandlerFunc(h.handleAdminGet)))
}

// handleUserGet — GET /api/v1/channels/{channelId}/messages/{messageId}/comment-edit-history.
//
// Validation order (acceptance §2.1):
//  1. Auth (user-rail).
//  2. Message exists (else 404 comment.message_not_found).
//  3. content_type == "artifact_comment" (else 404 comment.not_artifact_comment).
//  4. Owner-only — sender == current user (else 403 comment.not_owner).
//  5. Returns {history: [...]} with [] for empty/null.
func (h *CanvasCommentEditHistoryHandler) handleUserGet(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	messageID := r.PathValue("messageId")
	msg, err := h.Store.GetMessageByID(messageID)
	if err != nil || msg == nil {
		writeJSONError(w, http.StatusNotFound, CommentEditHistoryErrCodeMessageNotFound)
		return
	}
	// 设计 ③ content_type filter 强制 — 跟 DM-7 既有 /edit-history 区分.
	if msg.ContentType != "artifact_comment" {
		writeJSONError(w, http.StatusNotFound, CommentEditHistoryErrCodeNotArtifactComment)
		return
	}
	// 设计 ② sender-only — 反向断言 sender == current user.
	if msg.SenderID != user.ID {
		writeJSONError(w, http.StatusForbidden, CommentEditHistoryErrCodeNotOwner)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"history": parseMessageEditHistory(msg.EditHistory),
	})
}

// handleAdminGet — GET /admin-api/v1/messages/{messageId}/comment-edit-history.
//
// admin readonly. 设计 ② 反约束: 不挂 PATCH/DELETE/PUT (grep 守门).
// content_type filter still applies — admin querying a non-artifact_comment
// message via this endpoint gets 404 (so DM-7 vs CV-15 endpoints stay
// disjoint by message kind).
func (h *CanvasCommentEditHistoryHandler) handleAdminGet(w http.ResponseWriter, r *http.Request) {
	a := admin.AdminFromContext(r.Context())
	if a == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	messageID := r.PathValue("messageId")
	msg, err := h.Store.GetMessageByID(messageID)
	if err != nil || msg == nil {
		writeJSONError(w, http.StatusNotFound, CommentEditHistoryErrCodeMessageNotFound)
		return
	}
	if msg.ContentType != "artifact_comment" {
		writeJSONError(w, http.StatusNotFound, CommentEditHistoryErrCodeNotArtifactComment)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"history": parseMessageEditHistory(msg.EditHistory),
	})
}

// REFACTOR-1 R1.2: parseMessageEditHistory SSOT 移到 message_edit_history.go
// (helper-2). DM-7 ↔ CV-15 11 行 duplicate 合一, byte-identical 不破.
