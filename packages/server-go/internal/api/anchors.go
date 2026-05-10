// Package api — anchors.go: CV-2.2 server API for anchor-thread CRUD +
// comment + resolve + WS push (`anchor_comment_added`).
//
// Blueprint: docs/blueprint/current/canvas-vision.md §1.4 (artifact 集合) +
// §1.6 (锚点对话 = owner review agent 产物的工具).
// Spec brief: docs/implementation/modules/cv-2-spec.md (飞马 v0/v1/v2,
// 3 条原则 + 3 拆段). Schema源: migration v=14 cv_2_1_anchor_comments
// (#359 stacked, artifact_anchors + anchor_comments tables).
//
// Endpoints (cv-2-spec.md §1 字面):
//
//	POST /api/v1/artifacts/{artifactId}/anchors          create anchor (channel-scoped)
//	GET  /api/v1/artifacts/{artifactId}/anchors          list active anchor threads
//	POST /api/v1/anchors/{anchorId}/comments             reply on a thread
//	POST /api/v1/anchors/{anchorId}/resolve              mark resolved (owner / creator)
//
// 设计反查 (cv-2-spec.md §0):
//
//   - ① 锚点 = 人审 agent 产物 (人机界面, 非 agent 间通信). server kind=='agent'
//     POST 锚 → 403 错码 `anchor.create_owner_only`. 反向约束: 不开 agent → agent
//     锚点对话 (CV-2.2 在 reply 路径上一段同样校验 — agent 只能回复 thread 里
//     至少有一 author_kind='human' 的锚点; 反向约束 grep 0 hit).
//   - ② 锚点挂 artifact_version 不挂 artifact. 创锚校验 version (传入或默认 head),
//     版本 immutable 不会自动迁移 (artifact 滚下个 version 锚点不跟过去).
//   - ③ AnchorCommentAdded 套 #237 envelope, 走 RT-1.1 #290 cursor 单调发号
//     (10 字段 byte-identical 锁定 spec v2 字面, 字段名 `author_kind` 不复用
//     `committer_kind`).
//   - ⑦ channel 权限继承: 创/读 anchor = artifact 所属 channel 成员权限
//     (CHN-1 双轴隔离同源, 反向约束: 不另起 anchor-level 权限层).
//
// admin (god-mode) cookie 不入此 rail (跟 ADM-0 §1.3 红线一致).
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"


	"borgee-server/internal/idgen"
	"gorm.io/gorm"

	"borgee-server/internal/store"
)

// AnchorAuthorKind values per migration v=14 CHECK constraint (注: 不复用
// CommitterKind* 命名 — anchor 是评论作者非 commit 提交者; 飞马 spec v2 字面).
const (
	AnchorAuthorKindAgent = "agent"
	AnchorAuthorKindHuman = "human"
)

// AnchorErrCodeCreateOwnerOnly is the byte-identical error code returned
// by the server when a role='agent' user POSTs to /anchors or /comments.
// Pinned by cv-2-spec.md §3 反查出处 + 野马 #355 文案锁定设计 ⑤. Client UI 反断
// 0 hit on agent path (CV-2.3).
const AnchorErrCodeCreateOwnerOnly = "anchor.create_owner_only"

// AnchorCommentPusher is the seam between the api package and ws.Hub for
// the AnchorCommentAdded frame (mirrors ArtifactPusher for #290 lock).
type AnchorCommentPusher interface {
	PushAnchorCommentAdded(
		anchorID string,
		commentID int64,
		artifactID string,
		artifactVersionID int64,
		channelID string,
		authorID string,
		authorKind string,
		createdAt int64,
	) (cursor int64, sent bool)
}

// AnchorHandler exposes the CV-2.2 HTTP surface.
type AnchorHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	Hub    EventBroadcaster
	Pusher AnchorCommentPusher
	Now    func() time.Time
	NewID  func() string
}

func (h *AnchorHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *AnchorHandler) newID() string {
	if h.NewID != nil {
		return h.NewID()
	}
	return idgen.NewID()
}

func (h *AnchorHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("POST /api/v1/artifacts/{artifactId}/anchors", wrap(h.handleCreateAnchor))
	mux.Handle("GET /api/v1/artifacts/{artifactId}/anchors", wrap(h.handleListAnchors))
	mux.Handle("POST /api/v1/anchors/{anchorId}/comments", wrap(h.handleAddComment))
	mux.Handle("GET /api/v1/anchors/{anchorId}/comments", wrap(h.handleListComments))
	mux.Handle("POST /api/v1/anchors/{anchorId}/resolve", wrap(h.handleResolveAnchor))
}

// ----- raw row shapes (private to handler) -----

type anchorRow struct {
	ID                string `gorm:"column:id"`
	ArtifactID        string `gorm:"column:artifact_id"`
	ArtifactVersionID int64  `gorm:"column:artifact_version_id"`
	StartOffset       int64  `gorm:"column:start_offset"`
	EndOffset         int64  `gorm:"column:end_offset"`
	CreatedBy         string `gorm:"column:created_by"`
	CreatedAt         int64  `gorm:"column:created_at"`
	ResolvedAt        *int64 `gorm:"column:resolved_at"`
}

type anchorCommentRow struct {
	ID         int64  `gorm:"column:id"`
	AnchorID   string `gorm:"column:anchor_id"`
	Body       string `gorm:"column:body"`
	AuthorKind string `gorm:"column:author_kind"`
	AuthorID   string `gorm:"column:author_id"`
	CreatedAt  int64  `gorm:"column:created_at"`
}

// loadAnchor fetches the anchor row + parent artifact row in one shot
// so 设计 ⑦ channel-scope check + 设计 ② version pin can be applied.
func (h *AnchorHandler) loadAnchor(id string) (*anchorRow, *artifactRow, error) {
	var rows []anchorRow
	if err := h.Store.DB().Raw(`SELECT
  id, artifact_id, artifact_version_id, start_offset, end_offset,
  created_by, created_at, resolved_at
FROM artifact_anchors WHERE id = ?`, id).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, gorm.ErrRecordNotFound
	}
	a := &rows[0]
	var arts []artifactRow
	if err := h.Store.DB().Raw(`SELECT
  id, channel_id, type, title, body, current_version, created_at,
  archived_at, lock_holder_user_id, lock_acquired_at
FROM artifacts WHERE id = ?`, a.ArtifactID).Scan(&arts).Error; err != nil {
		return nil, nil, err
	}
	if len(arts) == 0 {
		return nil, nil, gorm.ErrRecordNotFound
	}
	return a, &arts[0], nil
}

// loadArtifactForAnchor reuses the artifact row shape from artifacts.go
// (private to package — same struct).
func (h *AnchorHandler) loadArtifact(id string) (*artifactRow, error) {
	var rows []artifactRow
	if err := h.Store.DB().Raw(`SELECT
  id, channel_id, type, title, body, current_version, created_at,
  archived_at, lock_holder_user_id, lock_acquired_at
FROM artifacts WHERE id = ?`, id).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

// authorKindForUser mirrors committerKindForUser in artifacts.go but
// returns the AnchorAuthorKind* constants — column naming is distinct
// per spec v2 (anchor 是评论作者非 commit 提交者).
func (h *AnchorHandler) authorKindForUser(u *store.User) string {
	if u == nil {
		return AnchorAuthorKindHuman
	}
	if u.Role == "agent" {
		return AnchorAuthorKindAgent
	}
	return AnchorAuthorKindHuman
}

// canAccessChannel — 设计 ⑦ channel-scope ACL (CHN-1 双轴隔离同).
func (h *AnchorHandler) canAccessChannel(channelID, userID string) bool {
	if !h.Store.IsChannelMember(channelID, userID) {
		return h.Store.CanAccessChannel(channelID, userID)
	}
	return true
}

// versionExists confirms (artifact_id, version) tuple is real before we
// pin an anchor to it (设计 ② immutability — pin to a version that exists,
// not arbitrary numbers).
func (h *AnchorHandler) lookupVersionPK(artifactID string, version int64) (int64, error) {
	var row struct {
		ID int64 `gorm:"column:id"`
	}
	res := h.Store.DB().Raw(`SELECT id FROM artifact_versions
WHERE artifact_id = ? AND version = ?`, artifactID, version).Scan(&row)
	if res.Error != nil {
		return 0, res.Error
	}
	if row.ID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return row.ID, nil
}

// threadHasHumanAuthor returns true iff any comment in the anchor thread
// has author_kind='human'. 设计 ① 反向约束: agent reply 必须落在已含 human
// 的 thread, 不允许 agent 自循环新建 / 在 agent-only thread 接龙.
//
// The anchor itself was created by a human (server enforces 创锚 owner-only)
// but the anchor row records `created_by` (user_id), not author_kind. We
// therefore look up the creator's role to seed the determination — if the
// creator is human OR any prior comment is human, agent reply allowed.
func (h *AnchorHandler) threadHasHumanAuthor(anchorID string, anchorCreatorID string) (bool, error) {
	// Creator role check first — anchor creation is owner-only enforced
	// at handleCreateAnchor, so creator should always be human, but we
	// re-verify against the user row in case of role change post-create.
	creator, err := h.Store.GetUserByID(anchorCreatorID)
	if err == nil && creator != nil && creator.Role != "agent" {
		return true, nil
	}
	// Fallback: scan comments for any human author.
	var rows []anchorCommentRow
	if err := h.Store.DB().Raw(`SELECT id, anchor_id, body, author_kind, author_id, created_at
FROM anchor_comments WHERE anchor_id = ? AND author_kind = ? LIMIT 1`,
		anchorID, AnchorAuthorKindHuman).Scan(&rows).Error; err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

// ----- POST /api/v1/artifacts/{artifactId}/anchors -----

type createAnchorRequest struct {
	// Version is the artifact_versions.version to pin the anchor to.
	// 0 / omitted means "current head version" (most common case).
	Version     int64 `json:"version"`
	StartOffset int64 `json:"start_offset"`
	EndOffset   int64 `json:"end_offset"`
}

func (h *AnchorHandler) handleCreateAnchor(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	// 设计 ① owner-only: agent role 创锚 → 403 anchor.create_owner_only.
	if user.Role == "agent" {
		writeJSONErrorCode(w, http.StatusForbidden, AnchorErrCodeCreateOwnerOnly, "anchor creation restricted to human reviewers")
		return
	}

	artifactID := r.PathValue("artifactId")
	art, err := h.loadArtifact(artifactID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Artifact not found")
		return
	}
	// 设计 ⑦ channel-scope.
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var req createAnchorRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.StartOffset < 0 || req.EndOffset < req.StartOffset {
		writeJSONError(w, http.StatusBadRequest, "end_offset must be >= start_offset and start_offset >= 0")
		return
	}
	// Default to current head version (设计 ② anchor pinned to a real version).
	v := req.Version
	if v == 0 {
		v = art.CurrentVersion
	}
	if v < 1 || v > art.CurrentVersion {
		writeJSONError(w, http.StatusBadRequest, "version out of range")
		return
	}
	versionPK, err := h.lookupVersionPK(art.ID, v)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "version not found")
		return
	}

	id := h.newID()
	nowMs := h.now().UnixMilli()

	if err := h.Store.DB().Exec(`INSERT INTO artifact_anchors
  (id, artifact_id, artifact_version_id, start_offset, end_offset, created_by, created_at)
  VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, art.ID, versionPK, req.StartOffset, req.EndOffset, user.ID, nowMs).Error; err != nil {
		// CHECK (end_offset >= start_offset) is the schema's last gate.
		if strings.Contains(err.Error(), "CHECK") {
			writeJSONError(w, http.StatusBadRequest, "end_offset must be >= start_offset")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "create anchor failed")
		return
	}

	writeJSONResponse(w, http.StatusCreated, map[string]any{
		"id":                  id,
		"artifact_id":         art.ID,
		"artifact_version_id": versionPK,
		"version":             v,
		"start_offset":        req.StartOffset,
		"end_offset":          req.EndOffset,
		"created_by":          user.ID,
		"created_at":          nowMs,
		"resolved_at":         nil,
	})
}

// ----- GET /api/v1/artifacts/{artifactId}/anchors -----

func (h *AnchorHandler) handleListAnchors(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	artifactID := r.PathValue("artifactId")
	art, err := h.loadArtifact(artifactID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Artifact not found")
		return
	}
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusNotFound, "Artifact not found")
		return
	}
	var rows []anchorRow
	if err := h.Store.DB().Raw(`SELECT
  id, artifact_id, artifact_version_id, start_offset, end_offset,
  created_by, created_at, resolved_at
FROM artifact_anchors WHERE artifact_id = ?
ORDER BY artifact_version_id ASC, start_offset ASC`, art.ID).Scan(&rows).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list anchors failed")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, h.serializeAnchor(&a))
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"anchors": out})
}

// ----- POST /api/v1/anchors/{anchorId}/comments -----

type addCommentRequest struct {
	Body string `json:"body"`
}

func (h *AnchorHandler) handleAddComment(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	anchorID := r.PathValue("anchorId")
	anchor, art, err := h.loadAnchor(anchorID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Anchor not found")
		return
	}
	// 设计 ⑦ channel-scope.
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	if anchor.ResolvedAt != nil {
		writeJSONError(w, http.StatusConflict, "Anchor already resolved")
		return
	}

	authorKind := h.authorKindForUser(user)

	// 设计 ① 反向约束: agent reply 只允许在 thread 已含 human author_kind 的情况下
	// (防 AI 自循环). human reply 始终允许.
	if authorKind == AnchorAuthorKindAgent {
		hasHuman, err := h.threadHasHumanAuthor(anchor.ID, anchor.CreatedBy)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "thread author check failed")
			return
		}
		if !hasHuman {
			writeJSONErrorCode(w, http.StatusForbidden, AnchorErrCodeCreateOwnerOnly, "agents cannot reply on agent-only anchor threads")
			return
		}
	}

	var req addCommentRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		writeJSONError(w, http.StatusBadRequest, "body is required")
		return
	}

	nowMs := h.now().UnixMilli()
	var commentID int64

	err = h.Store.DB().Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`INSERT INTO anchor_comments
  (anchor_id, body, author_kind, author_id, created_at)
  VALUES (?, ?, ?, ?, ?)`,
			anchor.ID, body, authorKind, user.ID, nowMs)
		if res.Error != nil {
			return res.Error
		}
		// SQLite last_insert_rowid → AUTOINCREMENT id.
		var idRow struct {
			ID int64 `gorm:"column:id"`
		}
		if err := tx.Raw(`SELECT last_insert_rowid() AS id`).Scan(&idRow).Error; err != nil {
			return err
		}
		commentID = idRow.ID
		return nil
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "add comment failed")
		return
	}

	// Push WS frame (设计 ③ envelope cursor 单调发号 byte-identical 跟
	// ArtifactUpdated 同 hub.cursors).
	if h.Pusher != nil {
		h.Pusher.PushAnchorCommentAdded(
			anchor.ID, commentID,
			art.ID, anchor.ArtifactVersionID,
			art.ChannelID, user.ID, authorKind, nowMs,
		)
	}

	writeJSONResponse(w, http.StatusCreated, map[string]any{
		"id":          commentID,
		"anchor_id":   anchor.ID,
		"body":        body,
		"author_kind": authorKind,
		"author_id":   user.ID,
		"created_at":  nowMs,
	})
}

// ----- GET /api/v1/anchors/{anchorId}/comments -----
//
// CV-2.3 client SPA pull path: after the anchor_comment_added WS frame
// lands (signal-only, 设计 ③), AnchorThreadPanel calls this endpoint to
// hydrate the thread body. channel-scoped ACL same as create/list anchors.

func (h *AnchorHandler) handleListComments(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	anchorID := r.PathValue("anchorId")
	_, art, err := h.loadAnchor(anchorID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Anchor not found")
		return
	}
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	var rows []anchorCommentRow
	if err := h.Store.DB().Raw(`SELECT
  id, anchor_id, body, author_kind, author_id, created_at
FROM anchor_comments WHERE anchor_id = ?
ORDER BY id ASC`, anchorID).Scan(&rows).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list comments failed")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		out = append(out, map[string]any{
			"id":          c.ID,
			"anchor_id":   c.AnchorID,
			"body":        c.Body,
			"author_kind": c.AuthorKind,
			"author_id":   c.AuthorID,
			"created_at":  c.CreatedAt,
		})
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"comments": out})
}

// ----- POST /api/v1/anchors/{anchorId}/resolve -----

func (h *AnchorHandler) handleResolveAnchor(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	anchorID := r.PathValue("anchorId")
	anchor, art, err := h.loadAnchor(anchorID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Anchor not found")
		return
	}
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	// Resolve permission: anchor creator OR channel owner (cv-2-spec.md
	// §1 "owner / 创建者"). Channel owner = channel.created_by.
	ch, err := h.Store.GetChannelByID(art.ChannelID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "channel not found")
		return
	}
	if user.ID != anchor.CreatedBy && user.ID != ch.CreatedBy {
		writeJSONError(w, http.StatusForbidden, "Only the anchor creator or channel owner may resolve")
		return
	}
	if anchor.ResolvedAt != nil {
		// idempotent: already resolved; return current state.
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"id":          anchor.ID,
			"resolved_at": *anchor.ResolvedAt,
		})
		return
	}

	nowMs := h.now().UnixMilli()
	res := h.Store.DB().Exec(`UPDATE artifact_anchors
SET resolved_at = ?
WHERE id = ? AND resolved_at IS NULL`, nowMs, anchor.ID)
	if res.Error != nil {
		writeJSONError(w, http.StatusInternalServerError, "resolve anchor failed")
		return
	}
	if res.RowsAffected == 0 {
		// Race: another caller resolved between load and update.
		writeJSONError(w, http.StatusConflict, "anchor state changed; reload")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"id":          anchor.ID,
		"resolved_at": nowMs,
	})
}

// ----- helpers -----

func (h *AnchorHandler) serializeAnchor(a *anchorRow) map[string]any {
	out := map[string]any{
		"id":                  a.ID,
		"artifact_id":         a.ArtifactID,
		"artifact_version_id": a.ArtifactVersionID,
		"start_offset":        a.StartOffset,
		"end_offset":          a.EndOffset,
		"created_by":          a.CreatedBy,
		"created_at":          a.CreatedAt,
	}
	if a.ResolvedAt != nil {
		out["resolved_at"] = *a.ResolvedAt
	} else {
		out["resolved_at"] = nil
	}
	return out
}

// writeJSONErrorCode emits {"error": msg, "code": code} so client UI can
// switch on a stable error code (vs string match on message). 设计 ①
// reverse-grep target: `anchor.create_owner_only`.
func writeJSONErrorCode(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg, "code": code})
}
