// Package api — artifacts.go: CV-1.2 server API for artifact CRUD +
// commit + rollback + WS push.
//
// Blueprint: docs/blueprint/current/canvas-vision.md §0 (channel 围 artifact 协作)
// + §1.1-§1.6 (D-lite + workspace per channel + Markdown ONLY v1) + §2
// (v1 做/不做). Spec brief: docs/implementation/modules/cv-1-spec.md
// (3 条原则 + 3 拆段). 原则: docs/qa/cv-1-stance-checklist.md (v0, 7 条原则)
// + docs/qa/cv-1-stance-v1-supplement.md (v1, ②③⑤⑦ 字段 + 边界 + REST + 反断).
//
// Schema源: migration v=13 cv_1_1_artifacts (#334 merged) — artifacts +
// artifact_versions tables + lock_holder_user_id / lock_acquired_at /
// rolled_back_from_version 列.
//
// Endpoints:
//
//	POST /api/v1/channels/{channelId}/artifacts        create artifact (channel-scoped)
//	GET  /api/v1/artifacts/{artifactId}                fetch current state (head body)
//	GET  /api/v1/artifacts/{artifactId}/versions       list version history
//	POST /api/v1/artifacts/{artifactId}/commits        commit a new version (acquires lock; lazy expire)
//	POST /api/v1/artifacts/{artifactId}/rollback       owner-only rollback to a prior version
//
// 设计反查 (v0+v1):
//
//   - ① 归属 = channel — channel membership 是唯一 ACL, 无 owner_id 主权列;
//     archive 随 channel.
//   - ② 单文档锁定 30s TTL — commit 路径走 lazy expire 抢锁定; 旧 holder 后写收
//     409 conflict + reload hint. 反向约束: 不上 CRDT, 不 range lock.
//   - ③ 版本线性 — `version` 单调每艺术品自增, UNIQUE(artifact_id, version)
//     由 schema 保护; 每 commit 写新 row, 旧版本不删.
//   - ④ Markdown ONLY v1 — type 锁定 'markdown' 由 CHECK 约束 (#334 schema 锁定).
//   - ⑤ ArtifactUpdated frame 走 RT-1.1 #290 既有 envelope (7 字段:
//     type/cursor/artifact_id/version/channel_id/updated_at/kind), envelope 内
//     不带 body, push 仅信号; committer_kind / committer_id 走 GET pull.
//   - ⑥ committer_kind 'agent'|'human' — schema CHECK 锁定; agent commit fanout
//     system message `"{agent_name} 更新 {artifact_name} v{n}"` 文案锁定.
//   - ⑦ rollback owner-only — POST /rollback {to_version:N} action endpoint
//     (非 PATCH body 字段); admin → 401, 非 owner → 403, 锁定持有 = 别人 → 409;
//     成功 = INSERT 新 row body=旧版本 + rolled_back_from_version=N.
//
// admin (god-mode) cookie 不入此 rail (跟 ADM-0 §1.3 红线一致, admin 走
// /admin-api/* 单独入口, 不写 artifact 行为).
package api

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"


	"borgee-server/internal/idgen"
	"gorm.io/gorm"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// ArtifactLockTTL is the single-doc lock TTL — 30s lazy expire window
// (v1 supplement ②). After 30s any caller may steal the lock; the
// previous holder's next write returns 409 conflict.
const ArtifactLockTTL = 30 * time.Second

// ArtifactType locks the v1 enum (设计 ④, mirrored by the CHECK constraint
// on artifacts.type in migration v=13). Any 脱节 here is caught by the
// schema test (TestCV11_RejectsNonMarkdownType).
const ArtifactType = "markdown"

// CommitterKind values per migration v=13 CHECK constraint (设计 ⑥).
const (
	CommitterKindAgent = "agent"
	CommitterKindHuman = "human"
)

// FrameKindCommit / FrameKindRollback distinguish the two cursor-bearing
// push paths on the existing RT-1.1 ArtifactUpdatedFrame. Both reuse the
// same envelope (反向约束: 不另造 envelope) — the `kind` tail field is the
// only differentiator the client switches on.
const (
	FrameKindCommit   = "commit"
	FrameKindRollback = "rollback"
)

// ArtifactPusher is the seam between the api package and ws.Hub for the
// RT-1.1 ArtifactUpdated frame (mirrors AgentInvitationPusher pattern in
// agent_invitations.go so the api package doesn't import internal/ws).
//
// The hub.PushArtifactUpdated signature is (artifactID, version,
// channelID, updatedAt, kind) → (cursor, sent). We re-export it via
// this interface so unit tests can inject a recording fake.
type ArtifactPusher interface {
	PushArtifactUpdated(artifactID string, version int64, channelID string, updatedAt int64, kind string) (cursor int64, sent bool)
}

// ArtifactHandler exposes the CV-1.2 HTTP surface. Hub may be nil in
// unit tests that don't assert push behaviour; nil-safe at call sites.
//
// IterationPusher is the CV-4.2 seam — when a commit carries
// `?iteration_id=<uuid>` (设计 ② commit 单一来源) we transition the
// iteration row from running→completed and emit IterationStateChanged.
// Optional: nil disables the iteration completion path (历史 commit
// behavior is unchanged, acceptance §2.2 反断).
type ArtifactHandler struct {
	Store           *store.Store
	Logger          *slog.Logger
	Hub             EventBroadcaster
	Pusher          ArtifactPusher
	IterationPusher IterationStatePusher
	Now             func() time.Time
	NewID           func() string
	clockFn         func() time.Time
}

func (h *ArtifactHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *ArtifactHandler) newID() string {
	if h.NewID != nil {
		return h.NewID()
	}
	return idgen.NewID()
}

func (h *ArtifactHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("POST /api/v1/channels/{channelId}/artifacts", wrap(h.handleCreate))
	mux.Handle("GET /api/v1/artifacts/{artifactId}", wrap(h.handleGet))
	mux.Handle("GET /api/v1/artifacts/{artifactId}/versions", wrap(h.handleListVersions))
	mux.Handle("POST /api/v1/artifacts/{artifactId}/commits", wrap(h.handleCommit))
	mux.Handle("POST /api/v1/artifacts/{artifactId}/rollback", wrap(h.handleRollback))
	// CV-2 v2 (#cv-2-v2): owner-only preview thumbnail generation.
	mux.Handle("POST /api/v1/artifacts/{artifactId}/preview", wrap(h.handlePreview))
	// CV-3 v2 (#cv-3-v2): owner-only code/markdown thumbnail generation
	// (二端互斥 跟 /preview — markdown/code 走此, image/video/pdf 走 /preview).
	mux.Handle("POST /api/v1/artifacts/{artifactId}/thumbnail", wrap(h.handleThumbnail))
	// CV-6 (#cv-6): owner-only artifact full-text search via SQLite FTS5.
	mux.Handle("GET /api/v1/artifacts/search", wrap(h.handleArtifactSearch))
}

// artifactRow is the raw shape we read back via gorm.Raw.Scan. We don't
// add a model in store/models.go because CV-1.2 is API-only (#334 owns
// the schema) and this struct is private to the handler.
type artifactRow struct {
	ID                string
	ChannelID         string  `gorm:"column:channel_id"`
	Type              string
	Title             string
	Body              string
	CurrentVersion    int64   `gorm:"column:current_version"`
	CreatedAt         int64   `gorm:"column:created_at"`
	ArchivedAt        *int64  `gorm:"column:archived_at"`
	LockHolderUserID  *string `gorm:"column:lock_holder_user_id"`
	LockAcquiredAt    *int64  `gorm:"column:lock_acquired_at"`
	// CV-2 v2 (#cv-2-v2) — server-recorded thumbnail/poster URL (https only).
	PreviewURL        *string `gorm:"column:preview_url"`
	// CV-3 v2 (#cv-3-v2) — server-recorded code/markdown thumbnail URL
	// (https only). 跟 PreviewURL 字段拆: PreviewURL 给 image/video/pdf
	// media kind, ThumbnailURL 给 markdown/code text kind (二端互斥).
	ThumbnailURL      *string `gorm:"column:thumbnail_url"`
}

type versionRow struct {
	ID                    int64  `gorm:"column:id"`
	ArtifactID            string `gorm:"column:artifact_id"`
	Version               int64
	Body                  string
	CommitterKind         string `gorm:"column:committer_kind"`
	CommitterID           string `gorm:"column:committer_id"`
	CreatedAt             int64  `gorm:"column:created_at"`
	RolledBackFromVersion *int64 `gorm:"column:rolled_back_from_version"`
}

func (h *ArtifactHandler) loadArtifact(id string) (*artifactRow, error) {
	var rows []artifactRow
	if err := h.Store.DB().Raw(`SELECT
  id, channel_id, type, title, body, current_version, created_at,
  archived_at, lock_holder_user_id, lock_acquired_at, preview_url, thumbnail_url
FROM artifacts WHERE id = ?`, id).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

func (h *ArtifactHandler) committerKindForUser(u *store.User) string {
	if u == nil {
		return CommitterKindHuman
	}
	if u.Role == "agent" {
		return CommitterKindAgent
	}
	return CommitterKindHuman
}

// resolveChannelOwner returns the channel.created_by user. CV-1 设计 ⑦
// rollback owner-only — channel-model §1.4 字面.
func (h *ArtifactHandler) channelOwnerID(channelID string) (string, error) {
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		return "", err
	}
	return ch.CreatedBy, nil
}

// canAccessChannel — channel membership (incl. private channel ACL).
// Mirrors the messages handler's gate. 设计 ① 归属 = channel.
func (h *ArtifactHandler) canAccessChannel(channelID, userID string) bool {
	if !h.Store.IsChannelMember(channelID, userID) {
		// public channels: we still defer to CanAccessChannel so the
		// CHN-1 双轴隔离 (org / channel) rules apply uniformly.
		return h.Store.CanAccessChannel(channelID, userID)
	}
	return true
}

// ----- POST /api/v1/channels/{channelId}/artifacts -----

type createArtifactRequest struct {
	Title    string           `json:"title"`
	Body     string           `json:"body"`
	Type     string           `json:"type"`
	Metadata ArtifactMetadata `json:"metadata"`
}

func (h *ArtifactHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	channelID := r.PathValue("channelId")
	if channelID == "" {
		writeJSONError(w, http.StatusBadRequest, "Channel ID is required")
		return
	}
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	if ch.ArchivedAt != nil {
		writeJSONError(w, http.StatusForbidden, "Channel is archived")
		return
	}
	// CHN-2.1 设计 ② DM 无 workspace (蓝图 §1.2 字面禁; #353 acceptance §2.3
	// 同源 — DM channel cross-type 反向约束). DM channel 创 artifact → 403
	// `dm.workspace_not_supported` 保底, 防 client UI bug 漏检.
	// grep 检查项: `dm.workspace_not_supported` count≥1 (本行).
	if ch.Type == "dm" {
		writeJSONErrorCode(w, http.StatusForbidden, "dm.workspace_not_supported", "DM 无 workspace, 跟 channel 拆")
		return
	}
	if !h.canAccessChannel(channelID, user.ID) {
		// 反向约束: cross-channel + cross-org → 403 (CHN-1 双轴隔离同).
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var req createArtifactRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeJSONError(w, http.StatusBadRequest, "title is required")
		return
	}
	// CV-3.2 (#363 / #397): kind enum extended to {markdown, code,
	// image_link} per migration v=17 (cv_3_1_artifact_kinds, #396 merged).
	// CV-1.2 设计 ④ Markdown ONLY 锁定已废 — 旧的 `400 "type must be 'markdown' (v1)"`
	// 文案此处删 (grep 检查 `type must be 'markdown' \(v1\)` count==0,
	// spec #397 §3 脱节 3 字面). 默认值仍 'markdown' 兼容旧 client (CV-1
	// 既有 POST /artifacts 不带 type 字段的路径走 markdown 默认, 不破).
	if req.Type == "" {
		req.Type = ArtifactKindMarkdown
	}
	if !IsValidArtifactKind(req.Type) {
		writeJSONError(w, http.StatusBadRequest, "artifact.invalid_kind: type must be one of [markdown code image_link video_link pdf_link]")
		return
	}
	// CV-3.2 metadata gate (acceptance §1.3 / §1.4 + 文案锁定 §1 ②④⑤):
	//   - kind='code'       MUST carry metadata.language ∈ 11 项白名单 + 'text'
	//   - kind='image_link' MUST carry metadata.kind ∈ ('image','link')
	//                       AND metadata.url 是合法 https URL
	// 反向约束 — javascript: / data: / data:image / http: / file: / 任何
	// 非 https scheme 全 reject (XSS 红线第一道, #370 §1 ④ + spec §3 出处).
	//
	// Note: metadata 本 PR **不持久化** (留作后续 — CV-3.2 schema 后续
	// 决定 add metadata column vs body JSON header). 服务端验完后丢弃,
	// client reload 时按 kind 默认 (code 默认 'text', image_link body
	// 即 URL). PR body Acceptance 段已明示此后续边界.
	if err := ValidateArtifactMetadata(req.Type, req.Metadata); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := h.now().UnixMilli()
	id := h.newID()
	committerKind := h.committerKindForUser(user)

	err = h.Store.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO artifacts
  (id, channel_id, type, title, body, current_version, created_at)
  VALUES (?, ?, ?, ?, ?, 1, ?)`,
			id, channelID, req.Type, req.Title, req.Body, now,
		).Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO artifact_versions
  (artifact_id, version, body, committer_kind, committer_id, created_at)
  VALUES (?, 1, ?, ?, ?, ?)`,
			id, req.Body, committerKind, user.ID, now,
		).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "create artifact failed")
		return
	}

	// Push: cursor allocated via RT-1.1 hub; reuses ArtifactUpdatedFrame
	// (#290 byte-identical, kind=commit).
	if h.Pusher != nil {
		h.Pusher.PushArtifactUpdated(id, 1, channelID, now, FrameKindCommit)
	}

	writeJSONResponse(w, http.StatusCreated, h.serializeArtifact(&artifactRow{
		ID: id, ChannelID: channelID, Type: req.Type,
		Title: req.Title, Body: req.Body, CurrentVersion: 1,
		CreatedAt: now,
	}, committerKind, user.ID))
}

// ----- GET /api/v1/artifacts/{artifactId} -----

func (h *ArtifactHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("artifactId")
	art, err := h.loadArtifact(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Artifact not found")
		return
	}
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusNotFound, "Artifact not found")
		return
	}
	// Resolve head version's committer for the response (设计 ⑤: push frame
	// 不含 committer; pull GET 才含).
	var head versionRow
	if err := h.Store.DB().Raw(`SELECT artifact_id, version, body, committer_kind, committer_id, created_at, rolled_back_from_version
FROM artifact_versions WHERE artifact_id = ? AND version = ?`,
		art.ID, art.CurrentVersion).Scan(&head).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "load head version failed")
		return
	}
	writeJSONResponse(w, http.StatusOK, h.serializeArtifact(art, head.CommitterKind, head.CommitterID))
}

// ----- GET /api/v1/artifacts/{artifactId}/versions -----

func (h *ArtifactHandler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("artifactId")
	art, err := h.loadArtifact(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Artifact not found")
		return
	}
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusNotFound, "Artifact not found")
		return
	}
	var rows []versionRow
	if err := h.Store.DB().Raw(`SELECT id, artifact_id, version, body, committer_kind, committer_id, created_at, rolled_back_from_version
FROM artifact_versions WHERE artifact_id = ? ORDER BY version ASC`, id).Scan(&rows).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list versions failed")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, h.serializeVersion(&v))
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"versions": out})
}

// ----- POST /api/v1/artifacts/{artifactId}/commits -----

type commitRequest struct {
	// ExpectedVersion is the version the client edited from. If it
	// differs from the artifact's current_version, return 409 — the
	// client must reload (设计 ② lock conflict + reload hint).
	ExpectedVersion int64  `json:"expected_version"`
	Body            string `json:"body"`
}

func (h *ArtifactHandler) handleCommit(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("artifactId")
	art, err := h.loadArtifact(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Artifact not found")
		return
	}
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	// AP-1 设计 ②③: ABAC capability check 单一来源 — agent 严格 (蓝图
	// §1.4 不享 wildcard); human 享 wildcard 短路. grep 检查 守 const
	// 字面单一来源 (spec §2 #1).
	if !auth.HasCapability(r.Context(), h.Store, auth.CommitArtifact, auth.ArtifactScopeStr(id)) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, `{"error":"Forbidden","required_capability":%q,"current_scope":%q}`,
			auth.CommitArtifact, auth.ArtifactScopeStr(id))
		return
	}

	var req commitRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := h.now()
	nowMs := now.UnixMilli()
	expireBefore := now.Add(-ArtifactLockTTL).UnixMilli()

	// 设计 ② lazy lock acquire: a commit takes the lock if (a) lock free
	// or (b) held by us, or (c) prior holder's window has lapsed beyond
	// 30s. Otherwise → 409 conflict (someone else is mid-edit).
	if art.LockHolderUserID != nil && *art.LockHolderUserID != user.ID {
		if art.LockAcquiredAt == nil || *art.LockAcquiredAt > expireBefore {
			writeJSONError(w, http.StatusConflict, "Artifact is locked by another editor")
			return
		}
		// else: lock expired, current caller may steal it.
	}

	// Optimistic concurrency: client must base the commit on the head
	// version. expected_version omitted (== 0) means "latest".
	if req.ExpectedVersion != 0 && req.ExpectedVersion != art.CurrentVersion {
		writeJSONError(w, http.StatusConflict, "version mismatch — reload before commit")
		return
	}

	committerKind := h.committerKindForUser(user)
	newVersion := art.CurrentVersion + 1

	err = h.Store.DB().Transaction(func(tx *gorm.DB) error {
		// Re-check current_version under tx so two concurrent commits
		// don't both see the same head and both bump.
		res := tx.Exec(`UPDATE artifacts SET
  body = ?, current_version = ?,
  lock_holder_user_id = ?, lock_acquired_at = ?
  WHERE id = ? AND current_version = ?`,
			req.Body, newVersion, user.ID, nowMs, id, art.CurrentVersion)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errArtifactConflict
		}
		return tx.Exec(`INSERT INTO artifact_versions
  (artifact_id, version, body, committer_kind, committer_id, created_at)
  VALUES (?, ?, ?, ?, ?, ?)`,
			id, newVersion, req.Body, committerKind, user.ID, nowMs,
		).Error
	})
	if errors.Is(err, errArtifactConflict) {
		writeJSONError(w, http.StatusConflict, "version mismatch — reload before commit")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "commit failed")
		return
	}

	// Push WS frame (RT-1.1 envelope, kind=commit).
	if h.Pusher != nil {
		h.Pusher.PushArtifactUpdated(id, newVersion, art.ChannelID, nowMs, FrameKindCommit)
	}

	// CV-4.2 设计 ② commit 单一来源: when ?iteration_id=<uuid> present, atomic
	// UPDATE iteration row running→completed + push IterationStateChanged.
	// State machine reject (source state not 'running') → 409 conflict
	// (acceptance §2.3 反断 — completed→running / failed→pending 等回退
	// 全 reject). 反向约束: 不开旁路 commit endpoint
	// (acceptance §4.1 字面). The version row + artifact row are already
	// committed; iteration UPDATE is a separate atomic stmt — if it
	// rejects we still surface 409 so client knows the iteration is stale.
	iterationID := r.URL.Query().Get("iteration_id")
	if iterationID != "" {
		// Need the artifact_versions.id PK (autoincrement) for
		// created_artifact_version_id. Lookup by (artifact_id, version).
		var versionPKRow struct {
			ID int64 `gorm:"column:id"`
		}
		_ = h.Store.DB().Raw(`SELECT id FROM artifact_versions
WHERE artifact_id = ? AND version = ?`, id, newVersion).Scan(&versionPKRow).Error
		ierr := CompleteIterationOnCommit(h.Store, iterationID, id,
			versionPKRow.ID, nowMs)
		if IsIterationStateMachineReject(ierr) {
			writeJSONError(w, http.StatusConflict, "iteration state changed; reload")
			return
		}
		if ierr != nil {
			writeJSONError(w, http.StatusInternalServerError, "iteration completion failed")
			return
		}
		// Look up channel_id for push (we already have art.ChannelID).
		if h.IterationPusher != nil {
			h.IterationPusher.PushIterationStateChanged(
				iterationID, id, art.ChannelID,
				IterationStateCompleted, "",
				versionPKRow.ID, nowMs)
		}
	}

	// 设计 ⑥: agent commit fanout system message. Format byte-identical:
	// "{agent_name} 更新 {artifact_name} v{n}". Human commits silent.
	if committerKind == CommitterKindAgent {
		h.fanoutAgentCommitMessage(art.ChannelID, user.DisplayName, art.Title, newVersion, nowMs)
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"artifact_id":     id,
		"version":         newVersion,
		"committer_id":    user.ID,
		"committer_kind":  committerKind,
		"updated_at":      nowMs,
	})
}

// ----- POST /api/v1/artifacts/{artifactId}/rollback -----

type rollbackRequest struct {
	ToVersion int64 `json:"to_version"`
}

func (h *ArtifactHandler) handleRollback(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		// 设计 ⑦ admin → 401 (admin god-mode 不入写动作).
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	id := r.PathValue("artifactId")
	art, err := h.loadArtifact(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Artifact not found")
		return
	}
	// Owner = channel.created_by (channel-model §1.4 字面).
	ownerID, err := h.channelOwnerID(art.ChannelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	if user.ID != ownerID {
		// 设计 ⑦ 非 owner → 403.
		writeJSONError(w, http.StatusForbidden, "Only the channel owner may rollback")
		return
	}
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var req rollbackRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ToVersion <= 0 || req.ToVersion >= art.CurrentVersion {
		writeJSONError(w, http.StatusBadRequest, "to_version must reference a prior version")
		return
	}

	now := h.now()
	nowMs := now.UnixMilli()
	expireBefore := now.Add(-ArtifactLockTTL).UnixMilli()

	// 设计 ⑦ rollback 也走加锁定路径: 锁定持有 = 别人 (未过 30s TTL) → 409.
	if art.LockHolderUserID != nil && *art.LockHolderUserID != user.ID {
		if art.LockAcquiredAt == nil || *art.LockAcquiredAt > expireBefore {
			writeJSONError(w, http.StatusConflict, "Artifact is locked by another editor")
			return
		}
	}

	// Fetch the source version's body.
	var src versionRow
	if err := h.Store.DB().Raw(`SELECT artifact_id, version, body, committer_kind, committer_id, created_at, rolled_back_from_version
FROM artifact_versions WHERE artifact_id = ? AND version = ?`,
		id, req.ToVersion).Scan(&src).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "load source version failed")
		return
	}
	if src.ArtifactID == "" {
		writeJSONError(w, http.StatusBadRequest, "to_version not found")
		return
	}

	committerKind := h.committerKindForUser(user)
	newVersion := art.CurrentVersion + 1
	rollbackFrom := req.ToVersion

	err = h.Store.DB().Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`UPDATE artifacts SET
  body = ?, current_version = ?,
  lock_holder_user_id = ?, lock_acquired_at = ?
  WHERE id = ? AND current_version = ?`,
			src.Body, newVersion, user.ID, nowMs, id, art.CurrentVersion)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errArtifactConflict
		}
		return tx.Exec(`INSERT INTO artifact_versions
  (artifact_id, version, body, committer_kind, committer_id, created_at, rolled_back_from_version)
  VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, newVersion, src.Body, committerKind, user.ID, nowMs, rollbackFrom,
		).Error
	})
	if errors.Is(err, errArtifactConflict) {
		writeJSONError(w, http.StatusConflict, "version mismatch — reload before rollback")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "rollback failed")
		return
	}

	// Push WS frame (RT-1.1 envelope, kind=rollback).
	if h.Pusher != nil {
		h.Pusher.PushArtifactUpdated(id, newVersion, art.ChannelID, nowMs, FrameKindRollback)
	}
	// 设计 ⑦ 反向约束: rollback 不发 system message (rollback 是 owner 行为,
	// 不污染 fanout, v1 supplement ⑦ "system message 不发").

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"artifact_id":             id,
		"version":                 newVersion,
		"rolled_back_from_version": rollbackFrom,
		"updated_at":              nowMs,
	})
}

// ----- helpers -----

var errArtifactConflict = errors.New("artifact version conflict")

// fanoutAgentCommitMessage emits the system message anchored by 设计 ⑥
// + cv-1 acceptance §2.4. Format byte-identical:
//
//	"{agent_name} 更新 {artifact_name} v{n}"
//
// sender_id='system' / content_type='text'. Failures log-only.
func (h *ArtifactHandler) fanoutAgentCommitMessage(channelID, agentName, artifactTitle string, version int64, ts int64) {
	content := fmt.Sprintf("%s 更新 %s v%d", agentName, artifactTitle, version)
	msg := &store.Message{
		ID:          idgen.NewID(),
		ChannelID:   channelID,
		SenderID:    "system",
		Content:     content,
		ContentType: "text",
		CreatedAt:   ts,
	}
	if err := h.Store.CreateMessage(msg); err != nil {
		if h.Logger != nil {
			h.Logger.Error("artifact agent-commit system message failed", "channel_id", channelID, "error", err)
		}
		return
	}
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(channelID, "system_message", map[string]any{
			"channel_id": channelID,
			"content":    content,
			"sender_id":  "system",
			"created_at": ts,
		})
	}
}

func (h *ArtifactHandler) serializeArtifact(a *artifactRow, committerKind, committerID string) map[string]any {
	out := map[string]any{
		"id":               a.ID,
		"channel_id":       a.ChannelID,
		"type":             a.Type,
		"title":            a.Title,
		"body":             a.Body,
		"current_version":  a.CurrentVersion,
		"created_at":       a.CreatedAt,
		"committer_kind":   committerKind,
		"committer_id":     committerID,
	}
	if a.ArchivedAt != nil {
		out["archived_at"] = *a.ArchivedAt
	}
	if a.LockHolderUserID != nil {
		out["lock_holder_user_id"] = *a.LockHolderUserID
	}
	if a.LockAcquiredAt != nil {
		out["lock_acquired_at"] = *a.LockAcquiredAt
	}
	// CV-2 v2 (#cv-2-v2): preview_url echoed when set; null/missing when absent.
	if a.PreviewURL != nil {
		out["preview_url"] = *a.PreviewURL
	}
	// CV-3 v2 (#cv-3-v2): thumbnail_url echoed when set (markdown/code only).
	if a.ThumbnailURL != nil {
		out["thumbnail_url"] = *a.ThumbnailURL
	}
	return out
}

func (h *ArtifactHandler) serializeVersion(v *versionRow) map[string]any {
	out := map[string]any{
		"version":         v.Version,
		"body":            v.Body,
		"committer_kind":  v.CommitterKind,
		"committer_id":    v.CommitterID,
		"created_at":      v.CreatedAt,
	}
	if v.RolledBackFromVersion != nil {
		out["rolled_back_from_version"] = *v.RolledBackFromVersion
	}
	return out
}

// compile-time guard: catch unused import slip-ups across iteration.
var _ = sql.ErrNoRows
