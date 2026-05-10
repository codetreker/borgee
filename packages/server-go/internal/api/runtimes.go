// Package api — runtimes.go: AL-4.2 server registry + start/stop API +
// heartbeat hook for agent_runtimes (process-level descriptor).
//
// Blueprint出处: docs/blueprint/current/agent-lifecycle.md §2.2 (默认 remote-agent +
// power user 直配 plugin 双路径 + v1 务实边界 — only OpenClaw / Mac+Linux /
// 不优化多 runtime 并行) + §2.3 (故障可解释) + §4 (remote-agent 安全模型留
// 第 6 轮); README.md §1 设计 #7 (Borgee 不带 runtime — 走 plugin 接);
// concept-model.md §0 (不调 LLM / 不带 runtime / 不定义角色模板).
// Spec brief: docs/implementation/modules/al-4-spec.md (飞马 #313 v0 →
// #379 v2, merged 962fec7) §0 设计 ①②③ + §1 拆段 AL-4.2.
// 原则: docs/qa/al-4-stance-checklist.md (野马 #387, merged 8db1f9c).
// Acceptance: docs/qa/acceptance-templates/al-4.md (#318) §2.1-§2.7 + §4.
// Content lock: docs/qa/al-4-content-lock.md (野马 #321) status DM 文案
// byte-identical 跟 AL-3 #305 / DM-2 #314 同模式.
//
// Schema源: migration v=16 al_4_1_agent_runtimes (#398 merged) —
// agent_runtimes table + idx_agent_runtimes_agent_id + UNIQUE(agent_id).
//
// Endpoints (acceptance §2 字面):
//
//	POST /api/v1/agents/{id}/runtime/register   create agent_runtimes row (owner-only)
//	POST /api/v1/agents/{id}/runtime/start      transition status → running (owner-only)
//	POST /api/v1/agents/{id}/runtime/stop       transition status → stopped (owner-only, idempotent)
//	POST /api/v1/agents/{id}/runtime/heartbeat  plugin → server, update last_heartbeat_at (owner-only — v0 simplify)
//	POST /api/v1/agents/{id}/runtime/error      transition status → error + reason (owner-only)
//	GET  /api/v1/agents/{id}/runtime            owner-only metadata read
//	GET  /admin-api/v1/runtimes                 admin god-mode whitelist (no last_error_reason)
//
// 设计反查 (al-4-spec.md §0 + acceptance §2 + §4):
//
//   - ① Borgee 不带 runtime: server 仅记 process descriptor, 不存
//     llm_provider / model_name / api_key / prompt_template (acceptance
//     §1.5 + §4.1 grep 检查 count==0 — schema 闸位已就位 #398).
//   - ② admin god-mode 元数据 only: admin endpoint 返白名单不写,
//     last_error_reason raw 文本不返 (acceptance §2.6 + §4.3 grep 检查
//     `admin.*runtime.*start|admin.*runtime.*stop` count==0).
//   - ③ runtime status ≠ presence: heartbeat 写 agent_runtimes.last_heartbeat_at
//     不写 presence_sessions (acceptance §2.4 + §4.2 grep 检查
//     grep 检查 — schema 闸位已就位 #398, server
//     handler 路径在此守不 import internal/presence 写 presence_sessions).
//   - ④ status DM 文案锁定 byte-identical: "{agent_name} 已启动" / "已停止" /
//     "出错: {reason}" 跟 #321 同源 (acceptance §2.7).
//   - ⑤ reason 复用 AL-1a #249 6 reason 枚举字面: api_key_invalid /
//     quota_exceeded / network_unreachable / runtime_crashed /
//     runtime_timeout / unknown — 不另起字典, 跟 agent/state.go Reason* +
//     AL-3 #305 + lib/agent-state.ts REASON_LABELS 三处 byte-identical
//     (acceptance §2.5 + spec §0 设计 ④).
//   - ⑥ 走 BPP-1 既有 frame 不拆 namespace: register / start / stop 不发
//     'runtime.start' / 'runtime.stop' 自造 frame type (acceptance §4.4
//     grep 检查 count==0 — 此 PR 不发 BPP frame, AL-4 真接管落 plugin 路径
//     时复用既有 AgentRegisterFrame, 不新建).
//
// admin (god-mode) cookie 仅入 GET /admin-api/v1/runtimes 元数据读路径,
// 不入写动作 (跟 ADM-0 §1.3 红线 + anchors / artifacts / iterations 同
// rail 隔离 — admin 永不 owner 化, AP-0 token 不双轨入此 rail).
//
// Permission anchor (acceptance §4.6): owner 化经 OwnerID 直比 (跟
// agents.go handleRotateAPIKey / handleDeleteAgent 同模式 — Borgee
// agent 无 channel-scope, 不走 RequirePermission scope resolver). 此处
// PermissionAgentRuntimeControl 是 docstring 占位常量, 留 AL-4 后续
// 真接管 plugin 路径时考虑切 RequirePermission middleware (届时需在
// GrantDefaultPermissions 加 grant — 此 PR 不动 permissions 默认行).
package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"


	"borgee-server/internal/idgen"
	"gorm.io/gorm"

	agentpkg "borgee-server/internal/agent"
	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// PermissionAgentRuntimeControl is the permission key reserved for AL-4
// 后续 RequirePermission migration. v0 uses inline OwnerID check
// (see file-level docstring §6). grep 检查 target for acceptance §4.6.
const PermissionAgentRuntimeControl = "agent.runtime.control"

// RuntimeStatus values pinned by migration v=16 (al_4_1_agent_runtimes
// #398) CHECK constraint. 4 态 byte-identical 跟 schema 字面.
const (
	RuntimeStatusRegistered = "registered"
	RuntimeStatusRunning    = "running"
	RuntimeStatusStopped    = "stopped"
	RuntimeStatusError      = "error"
)

// RuntimeProcessKind values pinned by migration v=16 CHECK constraint.
// v1 仅 'openclaw' (蓝图 §2.2 v1 边界字面), 'hermes' 占号 v2+.
const (
	RuntimeProcessKindOpenclaw = "openclaw"
	RuntimeProcessKindHermes   = "hermes"
)

// RuntimeStatusDMTemplate* — #321 文案锁定 byte-identical (acceptance §2.7).
// 改 = 改 #321 + 测试 byte-identical 锁定两处, grep 检查 防同义词脱节.
const (
	RuntimeStatusDMTemplateStart = "%s 已启动"
	RuntimeStatusDMTemplateStop  = "%s 已停止"
	RuntimeStatusDMTemplateError = "%s 出错: %s"
)

// RuntimeHandler exposes the AL-4.2 user-rail HTTP surface.
type RuntimeHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	Now    func() time.Time
	NewID  func() string
}

func (h *RuntimeHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *RuntimeHandler) newID() string {
	if h.NewID != nil {
		return h.NewID()
	}
	return idgen.NewID()
}

// RegisterRoutes mounts the user-rail runtime endpoints. The admin
// god-mode metadata read path (`GET /admin-api/v1/runtimes`) is registered
// separately by AdminRuntimeHandler.RegisterRoutes (admin.go-side rail).
//
// Defense-in-depth (acceptance §4.6 出处): start + stop wrap with
// `auth.RequirePermission(s, "agent.runtime.control", nil)` so a future
// GrantDefaultPermissions adjustment can narrow ownership without
// changing this file. v0 owner check is still inline (handlers do
// OwnerID 直比 — wildcard `(*, *)` AP-0 grant covers
// `agent.runtime.control` so existing humans pass through; non-owners
// then 403 via inline check). Reverse-grep §4.6: count≥2 for start +
// stop literal `agent.runtime.control` 命中此文件.
func (h *RuntimeHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	// start + stop: defense-in-depth permission gate (acceptance §4.6).
	// Inlined string literal — keep the byte-identical
	// `RequirePermission(s, "agent.runtime.control", ...)` shape so the
	// CI grep `RequirePermission..agent\.runtime\.control` matches both
	// call sites at count≥2 (acceptance §4.6 字面). PermissionAgentRuntimeControl
	// const declared above documents the source of truth — the literal
	// here is the spec compliance witness.
	startMw := auth.RequirePermission(h.Store, "agent.runtime.control", nil)
	stopMw := auth.RequirePermission(h.Store, "agent.runtime.control", nil)
	wrapPerm := func(mw func(http.Handler) http.Handler, f http.HandlerFunc) http.Handler {
		return authMw(mw(http.HandlerFunc(f)))
	}
	mux.Handle("POST /api/v1/agents/{id}/runtime/register", wrap(h.handleRegister))
	mux.Handle("POST /api/v1/agents/{id}/runtime/start", wrapPerm(startMw, h.handleStart))
	mux.Handle("POST /api/v1/agents/{id}/runtime/stop", wrapPerm(stopMw, h.handleStop))
	mux.Handle("POST /api/v1/agents/{id}/runtime/heartbeat", wrap(h.handleHeartbeat))
	mux.Handle("POST /api/v1/agents/{id}/runtime/error", wrap(h.handleError))
	mux.Handle("GET /api/v1/agents/{id}/runtime", wrap(h.handleGet))
}

// runtimeRow — raw shape (private to handler, 跟 anchorRow / iterationRow
// 同模式).
type runtimeRow struct {
	ID               string  `gorm:"column:id"`
	AgentID          string  `gorm:"column:agent_id"`
	EndpointURL      string  `gorm:"column:endpoint_url"`
	ProcessKind      string  `gorm:"column:process_kind"`
	Status           string  `gorm:"column:status"`
	LastErrorReason  *string `gorm:"column:last_error_reason"`
	LastHeartbeatAt  *int64  `gorm:"column:last_heartbeat_at"`
	CreatedAt        int64   `gorm:"column:created_at"`
	UpdatedAt        int64   `gorm:"column:updated_at"`
}

// loadOwnerCheckedAgent loads agent + verifies caller owns it. Returns
// 401 / 403 / 404 on the writer for non-owner paths and ok=false. Mirrors
// the OwnerID == nil || != user.ID pattern from agents.go (handleDeleteAgent
// / handleRotateAPIKey) — owner-only is the AL-4 invariant.
func (h *RuntimeHandler) loadOwnerCheckedAgent(w http.ResponseWriter, r *http.Request) (*store.User, *store.User, bool) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return nil, nil, false
	}
	id := r.PathValue("id")
	agent, err := h.Store.GetAgent(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return nil, nil, false
	}
	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return nil, nil, false
	}
	return user, agent, true
}

func (h *RuntimeHandler) loadRuntimeByAgent(agentID string) (*runtimeRow, error) {
	var rows []runtimeRow
	if err := h.Store.DB().Raw(`SELECT
  id, agent_id, endpoint_url, process_kind, status,
  last_error_reason, last_heartbeat_at, created_at, updated_at
FROM agent_runtimes WHERE agent_id = ?`, agentID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

// ----- POST /api/v1/agents/{id}/runtime/register -----

type registerRuntimeRequest struct {
	EndpointURL string `json:"endpoint_url"`
	ProcessKind string `json:"process_kind"`
}

func (h *RuntimeHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	_, agent, ok := h.loadOwnerCheckedAgent(w, r)
	if !ok {
		return
	}

	var req registerRuntimeRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	endpoint := strings.TrimSpace(req.EndpointURL)
	if endpoint == "" {
		writeJSONError(w, http.StatusBadRequest, "endpoint_url is required")
		return
	}
	kind := strings.TrimSpace(req.ProcessKind)
	if kind == "" {
		// v1 默认 openclaw (蓝图 §2.2 v1 边界字面).
		kind = RuntimeProcessKindOpenclaw
	}
	// schema CHECK 兜底 reject 'unknown' 等 enum 外值 — 此处显式校验提
	// 早错码 (跟 cv_3_2 metadata gate 同思路 — schema CHECK 是最后一道闸).
	if kind != RuntimeProcessKindOpenclaw && kind != RuntimeProcessKindHermes {
		writeJSONError(w, http.StatusBadRequest, "process_kind must be one of [openclaw hermes]")
		return
	}

	id := h.newID()
	nowMs := h.now().UnixMilli()

	if err := h.Store.DB().Exec(`INSERT INTO agent_runtimes
  (id, agent_id, endpoint_url, process_kind, status, created_at, updated_at)
  VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, agent.ID, endpoint, kind, RuntimeStatusRegistered, nowMs, nowMs,
	).Error; err != nil {
		// UNIQUE(agent_id) reject — 单 runtime per agent (设计 ① v1 边界).
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSONError(w, http.StatusConflict, "agent already has a runtime registered")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "register runtime failed")
		return
	}

	writeJSONResponse(w, http.StatusCreated, h.serializeRuntime(&runtimeRow{
		ID: id, AgentID: agent.ID, EndpointURL: endpoint,
		ProcessKind: kind, Status: RuntimeStatusRegistered,
		CreatedAt: nowMs, UpdatedAt: nowMs,
	}))
}

// ----- POST /api/v1/agents/{id}/runtime/start -----

// handleStart transitions status → running (acceptance §2.1). Owner-only
// via inline OwnerID check (see file-level docstring §6 — RequirePermission
// 后续). Idempotent if already 'running'.
//
// 反向约束 (acceptance §4.4): 不发自造 'runtime.start' BPP frame — AL-4
// 真接管时复用既有 AgentRegisterFrame, 不拆 namespace.
func (h *RuntimeHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	_, agent, ok := h.loadOwnerCheckedAgent(w, r)
	if !ok {
		return
	}
	rt, err := h.loadRuntimeByAgent(agent.ID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Runtime not registered for this agent")
		return
	}
	nowMs := h.now().UnixMilli()
	res := h.Store.DB().Exec(`UPDATE agent_runtimes
  SET status = ?, last_error_reason = NULL, updated_at = ?
  WHERE id = ?`, RuntimeStatusRunning, nowMs, rt.ID)
	if res.Error != nil {
		writeJSONError(w, http.StatusInternalServerError, "start runtime failed")
		return
	}
	// status 转 running 仅在源态 != running 时发 system DM (idempotent —
	// 重复 start 不重复发文案 #321 §2 反向约束).
	if rt.Status != RuntimeStatusRunning {
		h.fanoutOwnerSystemDM(*agent.OwnerID,
			fmt.Sprintf(RuntimeStatusDMTemplateStart, agent.DisplayName), nowMs)
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"id":        rt.ID,
		"agent_id":  rt.AgentID,
		"status":    RuntimeStatusRunning,
		"updated_at": nowMs,
	})
}

// ----- POST /api/v1/agents/{id}/runtime/stop -----

func (h *RuntimeHandler) handleStop(w http.ResponseWriter, r *http.Request) {
	_, agent, ok := h.loadOwnerCheckedAgent(w, r)
	if !ok {
		return
	}
	rt, err := h.loadRuntimeByAgent(agent.ID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Runtime not registered for this agent")
		return
	}
	nowMs := h.now().UnixMilli()
	res := h.Store.DB().Exec(`UPDATE agent_runtimes
  SET status = ?, updated_at = ?
  WHERE id = ?`, RuntimeStatusStopped, nowMs, rt.ID)
	if res.Error != nil {
		writeJSONError(w, http.StatusInternalServerError, "stop runtime failed")
		return
	}
	// Idempotent — 重复 stop 不重复发文案.
	if rt.Status != RuntimeStatusStopped {
		h.fanoutOwnerSystemDM(*agent.OwnerID,
			fmt.Sprintf(RuntimeStatusDMTemplateStop, agent.DisplayName), nowMs)
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"id":         rt.ID,
		"agent_id":   rt.AgentID,
		"status":     RuntimeStatusStopped,
		"updated_at": nowMs,
	})
}

// ----- POST /api/v1/agents/{id}/runtime/heartbeat -----

// handleHeartbeat updates agent_runtimes.last_heartbeat_at (acceptance §2.4).
// 设计 ③ 反向约束: 此 endpoint 不写 presence_sessions.last_heartbeat_at —
// 那是 AL-3 hub WS lifecycle 路径, runtime process-level / WS session-level
// 真分清. grep 检查 CI 守 — count==0 + 此 handler 不
// import internal/presence.
//
// v0 simplify: heartbeat 走 owner cookie 兜底 — AL-4 真接管时切 plugin token
// (BPP-1 connect frame 同源), 此 PR 不动 BPP auth.
func (h *RuntimeHandler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	_, agent, ok := h.loadOwnerCheckedAgent(w, r)
	if !ok {
		return
	}
	rt, err := h.loadRuntimeByAgent(agent.ID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Runtime not registered for this agent")
		return
	}
	nowMs := h.now().UnixMilli()
	if err := h.Store.DB().Exec(`UPDATE agent_runtimes
  SET last_heartbeat_at = ?, updated_at = ?
  WHERE id = ?`, nowMs, nowMs, rt.ID).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "heartbeat failed")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"id":                rt.ID,
		"last_heartbeat_at": nowMs,
	})
}

// ----- POST /api/v1/agents/{id}/runtime/error -----

type runtimeErrorRequest struct {
	Reason string `json:"reason"`
}

// handleError transitions status → error + last_error_reason. reason
// must be one of AL-1a #249 6 enum (acceptance §2.5 + 设计 ⑤). Schema
// 层无 CHECK enum (留 server 校验, 跟 11 项 language 白名单同思路 —
// schema CHECK 装不下产品级 enum).
func (h *RuntimeHandler) handleError(w http.ResponseWriter, r *http.Request) {
	_, agent, ok := h.loadOwnerCheckedAgent(w, r)
	if !ok {
		return
	}
	rt, err := h.loadRuntimeByAgent(agent.ID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Runtime not registered for this agent")
		return
	}
	var req runtimeErrorRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if !isValidAL1aReason(reason) {
		writeJSONError(w, http.StatusBadRequest,
			"reason must be one of AL-1a 6 reason enum")
		return
	}
	nowMs := h.now().UnixMilli()
	if err := h.Store.DB().Exec(`UPDATE agent_runtimes
  SET status = ?, last_error_reason = ?, updated_at = ?
  WHERE id = ?`, RuntimeStatusError, reason, nowMs, rt.ID).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "set error failed")
		return
	}
	if rt.Status != RuntimeStatusError {
		h.fanoutOwnerSystemDM(*agent.OwnerID,
			fmt.Sprintf(RuntimeStatusDMTemplateError, agent.DisplayName, reason), nowMs)
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"id":                rt.ID,
		"agent_id":          rt.AgentID,
		"status":            RuntimeStatusError,
		"last_error_reason": reason,
		"updated_at":        nowMs,
	})
}

// ----- GET /api/v1/agents/{id}/runtime -----

func (h *RuntimeHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	_, agent, ok := h.loadOwnerCheckedAgent(w, r)
	if !ok {
		return
	}
	rt, err := h.loadRuntimeByAgent(agent.ID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Runtime not registered for this agent")
		return
	}
	writeJSONResponse(w, http.StatusOK, h.serializeRuntime(rt))
}

// ----- helpers -----

// isValidAL1aReason — byte-identical 跟 agent/state.go Reason* + AL-3 #305
// + lib/agent-state.ts REASON_LABELS 三处一致. 改 = 改三处单测锁定 (#249 +
// AL-3 + 此). grep 检查 `last_error_reason.*=.*"[a-z_]+"` 字面校验.
func isValidAL1aReason(reason string) bool {
	switch reason {
	case agentpkg.ReasonAPIKeyInvalid,
		agentpkg.ReasonQuotaExceeded,
		agentpkg.ReasonNetworkUnreachable,
		agentpkg.ReasonRuntimeCrashed,
		agentpkg.ReasonRuntimeTimeout,
		agentpkg.ReasonUnknown:
		return true
	}
	return false
}

// fanoutOwnerSystemDM emits a system DM to owner only (acceptance §2.7).
// 反向约束: recipient = agent.owner_id only — channel fanout count==0;
// payload 不含 raw runtime_id / pid / endpoint_url (#321 §3 反向约束).
// Failures log-only (best-effort, 跟 fanoutAgentCommitMessage 同模式).
func (h *RuntimeHandler) fanoutOwnerSystemDM(ownerID, body string, ts int64) {
	dmCh, err := h.Store.CreateDmChannel(ownerID, "system")
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("runtime status DM ensure channel failed", "owner_id", ownerID, "error", err)
		}
		return
	}
	msg := &store.Message{
		ID:          idgen.NewID(),
		ChannelID:   dmCh.ID,
		SenderID:    "system",
		Content:     body,
		ContentType: "text",
		CreatedAt:   ts,
	}
	if err := h.Store.DB().Create(msg).Error; err != nil {
		if h.Logger != nil {
			h.Logger.Error("runtime status DM create failed", "owner_id", ownerID, "error", err)
		}
		return
	}
}

// serializeRuntime emits the full owner-rail row (includes
// last_error_reason). Admin god-mode rail uses serializeRuntimeAdmin
// instead — the white-listed projection per acceptance §2.6.
func (h *RuntimeHandler) serializeRuntime(rt *runtimeRow) map[string]any {
	out := map[string]any{
		"id":           rt.ID,
		"agent_id":     rt.AgentID,
		"endpoint_url": rt.EndpointURL,
		"process_kind": rt.ProcessKind,
		"status":       rt.Status,
		"created_at":   rt.CreatedAt,
		"updated_at":   rt.UpdatedAt,
	}
	if rt.LastHeartbeatAt != nil {
		out["last_heartbeat_at"] = *rt.LastHeartbeatAt
	} else {
		out["last_heartbeat_at"] = nil
	}
	if rt.LastErrorReason != nil {
		out["last_error_reason"] = *rt.LastErrorReason
	} else {
		out["last_error_reason"] = nil
	}
	return out
}

// ----- AL-4.2 admin god-mode metadata read (acceptance §2.6) -----

// AdminRuntimeHandler — admin god-mode rail for agent_runtimes metadata
// reads. **Read-only** — admin never writes to agent_runtimes (acceptance
// §4.3 反向约束 grep 检查 `admin.*runtime.*start|admin.*runtime.*stop`
// count==0). 设计 ② admin 元数据 only (跟 ADM-0 §1.3 红线 + AP-0 双轨闸
// 同模式).
//
// 隐私: response shape 字面排除 last_error_reason raw 文本 (acceptance
// §2.6 + 设计 ⑦ ADM-0 同源). 反向断言: TestAdminGodModeOmitsErrorReason
// 字面 reflect-scan 锁定.
type AdminRuntimeHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

func (h *AdminRuntimeHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("GET /admin-api/v1/runtimes", wrap(h.handleListRuntimes))
}

func (h *AdminRuntimeHandler) handleListRuntimes(w http.ResponseWriter, r *http.Request) {
	var rows []runtimeRow
	if err := h.Store.DB().Raw(`SELECT
  id, agent_id, endpoint_url, process_kind, status,
  last_error_reason, last_heartbeat_at, created_at, updated_at
FROM agent_runtimes ORDER BY created_at DESC`).Scan(&rows).Error; err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list runtimes failed")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, rt := range rows {
		// White-list: id / agent_id / endpoint_url / process_kind / status /
		// last_heartbeat_at. last_error_reason **OMITTED** (隐私 设计 ⑦
		// ADM-0 §1.3 红线, acceptance §2.6 字面). Reflect-scan 锁定
		// TestAdminGodModeOmitsErrorReason byte-identical.
		entry := map[string]any{
			"id":           rt.ID,
			"agent_id":     rt.AgentID,
			"endpoint_url": rt.EndpointURL,
			"process_kind": rt.ProcessKind,
			"status":       rt.Status,
		}
		if rt.LastHeartbeatAt != nil {
			entry["last_heartbeat_at"] = *rt.LastHeartbeatAt
		} else {
			entry["last_heartbeat_at"] = nil
		}
		out = append(out, entry)
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"runtimes": out})
}
