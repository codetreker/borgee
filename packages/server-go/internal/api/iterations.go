// Package api — iterations.go: CV-4.2 server API for artifact iterate
// orchestration: POST /iterate (owner-only) creates an iteration row,
// fail-closed when AL-4 runtime is not running; GET /iterations lists
// history; CV-1 commit single-source via ?iteration_id= query in
// artifacts.go's handleCommit.
//
// Blueprint: docs/blueprint/current/canvas-vision.md §1.4 (artifact 自带版本
// 历史: agent 每次修改产生一个版本) + §1.5 (agent 写内容默认允许).
// Spec brief: docs/implementation/modules/cv-4-spec.md (飞马 #365 v0,
// merged 9720a66) §0 立场 ① 域隔离 + ② commit 单源 + ③ client 算 diff +
// §1 拆段 CV-4.2.
// Stance: docs/qa/cv-4-stance-checklist.md (野马 #385).
// Acceptance: docs/qa/acceptance-templates/cv-4.md (#384) §2.1-§2.7 + §4.
// Content lock: docs/qa/cv-4-content-lock.md (野马 #380) state 4 态
// byte-identical + reason 三处单测锁 byte-identical.
//
// Schema源: migration v=18 cv_4_1_artifact_iterations (#405) —
// artifact_iterations table + 双索引.
//
// Endpoints (cv-4-spec.md §1 字面 + acceptance §2):
//
//	POST /api/v1/artifacts/{artifactId}/iterate         create iteration (owner-only, body intent_text + target_agent_id)
//	GET  /api/v1/artifacts/{artifactId}/iterations      list history (ORDER BY created_at DESC)
//	(commit single-source: POST /api/v1/artifacts/{id}/commits?iteration_id=<id> — handleCommit in artifacts.go)
//
// 立场反查 (cv-4-spec.md §0 + acceptance §2 + §4):
//
//   - ① 域隔离: messages 表无 iteration 反指列, artifact_versions schema
//     不动 (acceptance §1.5 + §4.2 字面). 反向 grep 0 hit.
//   - ② CV-1 commit 单源: 不开旁路 endpoint — commit 走
//     ?iteration_id= query atomic UPDATE (acceptance §2.2 + §4.1, 反向 grep
//     `POST.*\\/iterations\\/.*\\/commit` count==0).
//   - ③ server 不算 diff: 不下沉 jsdiff (acceptance §2.6 + §4.4, 反向 grep
//     server-side diff 模式 count==0).
//   - ④ state machine: 4 态合法转移 (pending→running / pending→failed /
//     running→completed / running→failed); 反断 completed→running /
//     completed→pending / failed→pending 等回退 (acceptance §2.3 + §4.3
//     反向 grep 0 hit).
//   - ⑤ AL-4 stub fail-closed: agent_runtimes.status != 'running' →
//     state='failed' + error_reason='runtime_not_registered' byte-identical
//     跟 AL-1a #249 6 reason 同源 (acceptance §2.5).
//   - ⑥ admin god-mode 不返 intent_text raw (ADM-0 §1.3 红线). admin cookie
//     不入此 rail — admin 走 /admin-api/* (跟 anchors.go / artifacts.go
//     同 rail 隔离).
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"


	"borgee-server/internal/idgen"
	"gorm.io/gorm"

	"borgee-server/internal/store"
)

// IterationState 4 态 byte-identical 跟 migration v=18 CHECK + 野马 #380
// 文案锁 ③ + ws.IterationState* 同源.
const (
	IterationStatePending   = "pending"
	IterationStateRunning   = "running"
	IterationStateCompleted = "completed"
	IterationStateFailed    = "failed"
)

// IterationErrorReasonRuntimeNotRegistered is the AL-4 stub fail-closed
// reason byte-identical 跟 AL-1a #249 6 reason 同源 — 不另起 reason
// 字典 (acceptance §2.5 + §4 字面). AL-4 落地后真路径切真 reason
// (api_key_invalid / quota_exceeded / network_unreachable /
// runtime_crashed / runtime_timeout / unknown).
const IterationErrorReasonRuntimeNotRegistered = "runtime_not_registered"

// IterationErrCodeTargetNotInChannel — target_agent_id 不是 channel
// member 时返回 (acceptance §2.1 字面). 反向 grep target.
const IterationErrCodeTargetNotInChannel = "iteration.target_not_in_channel"

// IterationStatePusher is the seam between the api package and ws.Hub
// for the IterationStateChanged frame (mirrors AnchorCommentPusher /
// ArtifactPusher pattern).
type IterationStatePusher interface {
	PushIterationStateChanged(
		iterationID string,
		artifactID string,
		channelID string,
		state string,
		errorReason string,
		createdArtifactVersionID int64,
		completedAt int64,
	) (cursor int64, sent bool)
}

// IterationHandler exposes the CV-4.2 HTTP surface.
type IterationHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	Pusher IterationStatePusher
	Now    func() time.Time
	NewID  func() string
}

func (h *IterationHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *IterationHandler) newID() string {
	if h.NewID != nil {
		return h.NewID()
	}
	return idgen.NewID()
}

func (h *IterationHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("POST /api/v1/artifacts/{artifactId}/iterate", wrap(h.handleIterate))
	mux.Handle("GET /api/v1/artifacts/{artifactId}/iterations", wrap(h.handleListIterations))
}

// iterationRow is the raw shape we read back via gorm.Raw.Scan (跟
// anchorRow / artifactRow 同模式 — private to handler).
type iterationRow struct {
	ID                       string  `gorm:"column:id"`
	ArtifactID               string  `gorm:"column:artifact_id"`
	RequestedBy              string  `gorm:"column:requested_by"`
	IntentText               string  `gorm:"column:intent_text"`
	TargetAgentID            string  `gorm:"column:target_agent_id"`
	State                    string  `gorm:"column:state"`
	CreatedArtifactVersionID *int64  `gorm:"column:created_artifact_version_id"`
	ErrorReason              *string `gorm:"column:error_reason"`
	CreatedAt                int64   `gorm:"column:created_at"`
	CompletedAt              *int64  `gorm:"column:completed_at"`
}

func (h *IterationHandler) loadArtifact(id string) (*artifactRow, error) {
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

// canAccessChannel mirrors anchors / artifacts handlers (CHN-1 双轴隔离).
func (h *IterationHandler) canAccessChannel(channelID, userID string) bool {
	if !h.Store.IsChannelMember(channelID, userID) {
		return h.Store.CanAccessChannel(channelID, userID)
	}
	return true
}

// channelOwnerID returns channel.created_by (跟 ArtifactHandler.channelOwnerID
// 同模式 — owner = channel-model §1.4 字面).
func (h *IterationHandler) channelOwnerID(channelID string) (string, error) {
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		return "", err
	}
	return ch.CreatedBy, nil
}

// agentRuntimeRunning returns true iff agent_runtimes row exists for
// agentID with status='running'. AL-4 stub fail-closed 立场 ⑤: 任何其它
// 状态 (registered / stopped / error) 或行不存在 → 不可派 → state='failed'
// + reason='runtime_not_registered'. AL-4 落地后切真路径不破此函数语义 —
// runtime 跑着 = status='running' 是 AL-4.1 字面.
func (h *IterationHandler) agentRuntimeRunning(agentID string) bool {
	var row struct {
		Status string `gorm:"column:status"`
	}
	res := h.Store.DB().Raw(`SELECT status FROM agent_runtimes WHERE agent_id = ?`, agentID).Scan(&row)
	if res.Error != nil {
		return false
	}
	return row.Status == "running"
}

// ----- POST /api/v1/artifacts/{artifactId}/iterate -----

type iterateRequest struct {
	IntentText    string `json:"intent_text"`
	TargetAgentID string `json:"target_agent_id"`
}

func (h *IterationHandler) handleIterate(w http.ResponseWriter, r *http.Request) {
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
	// 立场 ⑦ channel-scope (跟 anchors / artifacts 同).
	if !h.canAccessChannel(art.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	// owner-only (acceptance §2.1) — owner = channel.created_by.
	ownerID, err := h.channelOwnerID(art.ChannelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	if user.ID != ownerID {
		writeJSONError(w, http.StatusForbidden, "Only the channel owner may iterate")
		return
	}

	var req iterateRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	intent := strings.TrimSpace(req.IntentText)
	if intent == "" {
		writeJSONError(w, http.StatusBadRequest, "intent_text is required")
		return
	}
	if strings.TrimSpace(req.TargetAgentID) == "" {
		writeJSONError(w, http.StatusBadRequest, "target_agent_id is required")
		return
	}
	// target must be channel member + role='agent' (acceptance §2.1 字面).
	if !h.Store.IsChannelMember(art.ChannelID, req.TargetAgentID) {
		writeJSONErrorCode(w, http.StatusBadRequest, IterationErrCodeTargetNotInChannel,
			"target agent is not a member of the artifact's channel")
		return
	}
	target, err := h.Store.GetUserByID(req.TargetAgentID)
	if err != nil || target == nil || target.Role != "agent" {
		writeJSONErrorCode(w, http.StatusBadRequest, IterationErrCodeTargetNotInChannel,
			"target_agent_id must reference a role='agent' user")
		return
	}

	id := h.newID()
	nowMs := h.now().UnixMilli()

	// Create as pending. Even when AL-4 stub fail-closes immediately, we
	// persist the pending row first so audit history is intact (acceptance
	// §3.6 history inline 5 条). Then we transition pending→failed in a
	// second UPDATE — same row, atomic state machine path.
	if err := h.Store.DB().Exec(`INSERT INTO artifact_iterations
  (id, artifact_id, requested_by, intent_text, target_agent_id, state, created_at)
  VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, art.ID, user.ID, intent, req.TargetAgentID,
		IterationStatePending, nowMs,
	).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "create iteration failed")
		return
	}
	// Push pending state.
	if h.Pusher != nil {
		h.Pusher.PushIterationStateChanged(id, art.ID, art.ChannelID,
			IterationStatePending, "", 0, 0)
	}

	// AL-4 dispatch — stub fail-closed when runtime not 'running'. AL-4
	// 落地后真路径在此处切到 plugin/remote dispatch (此 PR 仅留 stub fork).
	if !h.agentRuntimeRunning(req.TargetAgentID) {
		failedAt := h.now().UnixMilli()
		// State machine pending → failed (合法转移).
		res := h.Store.DB().Exec(`UPDATE artifact_iterations
  SET state = ?, error_reason = ?, completed_at = ?
  WHERE id = ? AND state = ?`,
			IterationStateFailed, IterationErrorReasonRuntimeNotRegistered,
			failedAt, id, IterationStatePending)
		if res.Error == nil && res.RowsAffected == 1 {
			if h.Pusher != nil {
				h.Pusher.PushIterationStateChanged(id, art.ID, art.ChannelID,
					IterationStateFailed, IterationErrorReasonRuntimeNotRegistered,
					0, failedAt)
			}
		}
		writeJSONResponse(w, http.StatusCreated, map[string]any{
			"id":              id,
			"artifact_id":     art.ID,
			"requested_by":    user.ID,
			"intent_text":     intent,
			"target_agent_id": req.TargetAgentID,
			"state":           IterationStateFailed,
			"error_reason":    IterationErrorReasonRuntimeNotRegistered,
			"created_at":      nowMs,
			"completed_at":    failedAt,
		})
		return
	}
	// AL-4 live path placeholder: pending → running. Real dispatch lands
	// when AL-4 runtime hub plugin path is wired (acceptance §2.5 second
	// branch). Not in CV-4.2 scope — leave running as the durable state.
	res := h.Store.DB().Exec(`UPDATE artifact_iterations
  SET state = ?
  WHERE id = ? AND state = ?`,
		IterationStateRunning, id, IterationStatePending)
	if res.Error == nil && res.RowsAffected == 1 {
		if h.Pusher != nil {
			h.Pusher.PushIterationStateChanged(id, art.ID, art.ChannelID,
				IterationStateRunning, "", 0, 0)
		}
	}

	writeJSONResponse(w, http.StatusCreated, map[string]any{
		"id":              id,
		"artifact_id":     art.ID,
		"requested_by":    user.ID,
		"intent_text":     intent,
		"target_agent_id": req.TargetAgentID,
		"state":           IterationStateRunning,
		"created_at":      nowMs,
	})
}

// ----- GET /api/v1/artifacts/{artifactId}/iterations -----

func (h *IterationHandler) handleListIterations(w http.ResponseWriter, r *http.Request) {
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

	// CV-4 v2 — clamp ?limit query (default 50, max 200, 0/negative → default).
	// 立场 ① — endpoint shape unchanged from v1; only the optional limit
	// query is new. cursor reuse goes via existing events sequence.
	limit := cv4v2ClampLimit(r.URL.Query().Get("limit"))

	var rows []iterationRow
	if err := h.Store.DB().Raw(`SELECT
  id, artifact_id, requested_by, intent_text, target_agent_id, state,
  created_artifact_version_id, error_reason, created_at, completed_at
FROM artifact_iterations WHERE artifact_id = ?
ORDER BY created_at DESC LIMIT ?`, art.ID, limit).Scan(&rows).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list iterations failed")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, it := range rows {
		out = append(out, h.serializeIteration(&it))
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"iterations": out})
}

// cv4v2ClampLimit parses the ?limit query string per CV-4 v2 立场 ①
// (default 50, max 200, 0/negative/empty → 50). Exposed for unit tests
// that want to cover the clamp matrix without booting an HTTP server.
func cv4v2ClampLimit(raw string) int {
	const (
		def = 50
		max = 200
	)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// ClampCV4V2LimitForTest exposes cv4v2ClampLimit to api_test test files
// (test-only export, prefixed with ForTest to keep the public API surface
// clean — same pattern as other ForTest helpers in this package).
func ClampCV4V2LimitForTest(raw string) int { return cv4v2ClampLimit(raw) }

// serializeIteration emits the API row shape. intent_text is included on
// the channel-member rail (this handler is gated by canAccessChannel).
// admin god-mode (ADM-0 §1.3) does not enter this rail — admin path lives
// at /admin-api/* and is the responsibility of admin.go to never read
// artifact_iterations.intent_text into its response (acceptance §2.7
// 反断). 反向 grep `admin.*intent_text|intent_text.*admin` count==0 in
// internal/api/admin*.go enforced at PR review.
func (h *IterationHandler) serializeIteration(it *iterationRow) map[string]any {
	out := map[string]any{
		"id":              it.ID,
		"artifact_id":     it.ArtifactID,
		"requested_by":    it.RequestedBy,
		"intent_text":     it.IntentText,
		"target_agent_id": it.TargetAgentID,
		"state":           it.State,
		"created_at":      it.CreatedAt,
	}
	if it.CreatedArtifactVersionID != nil {
		out["created_artifact_version_id"] = *it.CreatedArtifactVersionID
	} else {
		out["created_artifact_version_id"] = nil
	}
	if it.ErrorReason != nil {
		out["error_reason"] = *it.ErrorReason
	} else {
		out["error_reason"] = nil
	}
	if it.CompletedAt != nil {
		out["completed_at"] = *it.CompletedAt
	} else {
		out["completed_at"] = nil
	}
	return out
}

// ----- CV-1 commit single-source: ?iteration_id= atomic UPDATE -----

// errIterationStateMachineReject is returned when an iteration state
// transition violates the 4-态 forward-only state machine (acceptance §2.3
// 反断 — 反 completed→running / completed→pending / failed→pending).
var errIterationStateMachineReject = errors.New("iteration state machine reject")

// CompleteIterationOnCommit is invoked by ArtifactHandler.handleCommit when
// the request carries `?iteration_id=<uuid>`. It performs a single atomic
// UPDATE that is the 立场 ② "CV-1 commit 单源" — there is no
// POST bypass endpoint (acceptance §2.2 + §4.1
// 反向 grep `POST.*\/iterations\/.*\/commit` count==0).
//
// State machine: only `running` → `completed` is legal here. Any other
// source state (pending / completed / failed) returns
// errIterationStateMachineReject and the caller MUST NOT silently swallow
// the rejection — acceptance §2.3 requires the commit path itself to
// surface 409 conflict so a stale client cannot replay a completed
// iteration_id (反约束: completed → running 等回退绝对 reject).
//
// This function is exposed package-private for artifacts.go to call after
// a successful commit Tx. It writes through h.Store.DB() (no nested Tx)
// because commit's outer Tx already closed; the iteration UPDATE is its
// own atomic statement (the WHERE state='running' clause guards 4-态
// machine reject).
func CompleteIterationOnCommit(s *store.Store, iterationID, artifactID string,
	createdArtifactVersionID int64, completedAt int64) error {
	if iterationID == "" {
		return nil
	}
	res := s.DB().Exec(`UPDATE artifact_iterations
  SET state = ?, created_artifact_version_id = ?, completed_at = ?
  WHERE id = ? AND artifact_id = ? AND state = ?`,
		IterationStateCompleted, createdArtifactVersionID, completedAt,
		iterationID, artifactID, IterationStateRunning)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// State machine reject: source was not 'running' — could be
		// pending (race), completed (replay), failed (forbidden) or
		// wrong artifact_id. All collapse to the same 409.
		return errIterationStateMachineReject
	}
	return nil
}

// IsIterationStateMachineReject lets artifacts.go map the package-private
// error to a 409 status code without exporting the var directly (跟
// errArtifactConflict 同模式 — error sentinel 不外露).
func IsIterationStateMachineReject(err error) bool {
	return errors.Is(err, errIterationStateMachineReject)
}
