// Package api — agent_config.go: AL-2a.2 agent_configs REST endpoints.
//
// Spec: docs/qa/acceptance-templates/al-2a.md (#264, 7 验收项).
// Blueprint: agent-lifecycle.md §2.1 (用户完全自主决定 agent 的
// name/prompt/能力/model) + plugin-protocol.md §1.4 (Borgee=单一来源 字段
// 划界) + §1.5 (热更新分级 — 字段下发, AL-2a 不含 BPP frame, 走轮询
// reload).
// R3 决议: AL-2 拆 a/b — AL-2a 只落 config 表 + REST update API; agent
// 端 reload 走轮询; BPP `agent_config_update` frame 留 AL-2b + BPP-3
// 同合 (战马 D5 锁定紧).
//
// Endpoint surface:
//   - GET   /api/v1/agents/:id/config         return agent's current config
//     ({schema_version, blob})
//   - PATCH /api/v1/agents/:id/config         atomic blob 整体替换 + version++
//     (acceptance §4.1.a 并发 update
//     末次胜出 + schema_version 严格
//     递增 + 无丢失)
//
// 原则 reverse-grep targets (蓝图 §1.4 单一来源 + §1.5 BPP frame 反向约束):
//   - 蓝图 §1.4 单一来源 设计: blob 仅 Borgee 管字段 (name / avatar / prompt /
//     model / capabilities / enabled / memory_ref). Runtime-only 字段
//     (api_key / temperature / token_limit / retry_policy) **fail-closed**
//     reject by allowedConfigKeys whitelist (acceptance §4.1.c reflect
//     scan 同源).
//   - 蓝图 §1.5 BPP frame `agent_config_update` 不在 AL-2a 范围: 本文件
//     无 hub.Broadcast 调用 (grep 检查 `agent_config_update` 在 ws/ 和
//     bpp/ count==0; AL-2a 走轮询 reload, agent 端 GET 周期性).
//   - Owner-only ACL: 跨 agent owner 调用 PATCH → 403 (跟 agents.go ACL
//     pattern 同源, acceptance §4.1.b).
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"borgee-server/internal/store"

	"gorm.io/gorm"
)

// AgentConfigPusher is the interface for fanning out agent_config_update
// BPP frames to the target plugin connection (AL-2b §2.1 server→plugin).
// Implemented by *ws.Hub.PushAgentConfigUpdate; injected via interface
// seam to keep the api package from depending on internal/ws (跟
// IterationStatePusher / ArtifactPusher / AgentInvitationPusher 同模式).
//
// 反向约束: AL-2a-only callers (历史 poll path) leave Pusher nil — PATCH
// handler skips fanout silently. AL-2b wire-up sets Pusher at server
// boot, single source of truth.
type AgentConfigPusher interface {
	PushAgentConfigUpdate(agentID string, schemaVersion int64, blob string,
		idempotencyKey string, createdAt int64) (cursor int64, sent bool)
}

// AgentConfigHandler handles agent config 单一来源 endpoints (AL-2a.2) +
// AL-2b BPP fanout (when Pusher is wired).
type AgentConfigHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	Now    func() time.Time // injectable clock for tests; defaults to time.Now.
	// Pusher is the AL-2b BPP fanout seam. Nil-safe — handler logs +
	// continues if not wired (best-effort, plugin reconnect 后 GET
	// /agents/:id/config 主动拉最新, 跟蓝图 §1.5 "runtime 不缓存" 同源).
	Pusher AgentConfigPusher
}

func (h *AgentConfigHandler) now() int64 {
	if h.Now != nil {
		return h.Now().UnixMilli()
	}
	return time.Now().UnixMilli()
}

func (h *AgentConfigHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("GET /api/v1/agents/{id}/config", wrap(h.handleGetAgentConfig))
	mux.Handle("PATCH /api/v1/agents/{id}/config", wrap(h.handlePatchAgentConfig))
}

// agentConfigRow mirrors migration v=20 (acceptance §数据契约 row 1).
type agentConfigRow struct {
	AgentID       string `gorm:"column:agent_id"       json:"-"`
	SchemaVersion int64  `gorm:"column:schema_version" json:"schema_version"`
	Blob          string `gorm:"column:blob"           json:"-"`
	CreatedAt     int64  `gorm:"column:created_at"     json:"-"`
	UpdatedAt     int64  `gorm:"column:updated_at"     json:"updated_at"`
}

// allowedConfigKeys is the 单一来源 whitelist of keys that may live in
// agent_configs.blob (蓝图 §1.4 字段划界, fail-closed).
//
// Reject (acceptance §4.1.c reflect scan 同源, runtime-only):
//   - api_key / api_secret / token / credentials  → secrets
//   - temperature / top_p / token_limit / max_tokens → runtime tuning
//   - retry_policy / timeout_ms / backoff           → runtime tuning
//   - latency_budget_ms / circuit_breaker          → runtime tuning
//
// 字面跟随 acceptance §4.1.c reflect scan:
// `不返回 api_key/temperature/retry_policy 等 runtime-only 字段 (fail-closed)`.
var allowedConfigKeys = map[string]bool{
	"name":         true, // 蓝图 §1.4 "归 Borgee 管"
	"avatar":       true, // 蓝图 §1.4 "归 Borgee 管"
	"prompt":       true, // 蓝图 §1.4 "归 Borgee 管"
	"model":        true, // 蓝图 §1.4 "归 Borgee 管" (model identifier 字符串, 非 LLM 调用参数)
	"capabilities": true, // 蓝图 §1.4 能力开关
	"enabled":      true, // 蓝图 §1.4 启用状态
	"memory_ref":   true, // 蓝图 §1.4 单一来源 设计
}

// ----- GET /api/v1/agents/:id/config -----
//
// Acceptance §4.1.d: 200 with {schema_version, blob} + agent 端轮询 reload
// 脱节 test 防 cache 不刷.
func (h *AgentConfigHandler) handleGetAgentConfig(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	agent, id, ok := loadAgentByPath(w, r, h.Store)
	if !ok {
		return
	}
	// Owner-only ACL (acceptance §4.1.b 同源 cross-owner reject 403).
	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var row agentConfigRow
	err := h.Store.DB().Raw(`SELECT agent_id, schema_version, blob, created_at, updated_at
		FROM agent_configs WHERE agent_id = ?`, id).Scan(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		h.logErr("agent_config get", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to load agent config")
		return
	}
	if row.AgentID == "" {
		// 无 row — 返 schema_version=0 + blob {} (空 单一来源, agent 端轮询初始).
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"schema_version": int64(0),
			"blob":           map[string]any{},
		})
		return
	}
	// Parse blob 为 map (反向约束: schema 存 TEXT JSON, response 反序列化返
	// map; 不能裸返 string, 否则 client 双重解码).
	var blobMap map[string]any
	if err := json.Unmarshal([]byte(row.Blob), &blobMap); err != nil {
		h.logErr("agent_config blob unmarshal", err)
		writeJSONError(w, http.StatusInternalServerError, "Corrupt agent config")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"schema_version": row.SchemaVersion,
		"blob":           blobMap,
		"updated_at":     row.UpdatedAt,
	})
}

// ----- PATCH /api/v1/agents/:id/config -----
//
// Acceptance §4.1.a: 并发 2 写 → 末次胜出 + schema_version 严格递增 + 无
// 丢失. Implementation: SQLite ON CONFLICT(agent_id) DO UPDATE atomic
// UPSERT, schema_version = excluded.schema_version + 1 (server-stamp,
// monotonic per-row).
//
// Body shape: {"blob": {...}}. Idempotent: 同 payload 重发**仍递增 version**
// (避免 "幂等" 误读 — acceptance §4.1.a "末次胜出" 含义是无丢失, 不是
// dedup; client 不应重发同 payload).
//
// Failure surface:
//   - 400 `agent_config.invalid_payload` (空 body / 非 JSON / blob field 缺)
//   - 400 `agent_config.runtime_field_rejected` (blob 含 allowedConfigKeys
//     白名单外字段, fail-closed reject — acceptance §4.1.c reflect scan)
//   - 403 (cross-owner — acceptance §4.1.b)
//   - 500 with msg "agent 配置保存失败, 请重试" (跟 layout.go 失败 toast
//     同模式, AL-2a 文案锁定待 #264 后续; 暂用此 msg).
const agentConfigSaveErrorMsg = "agent 配置保存失败, 请重试"

type agentConfigPatchRequest struct {
	Blob map[string]any `json:"blob"`
}

func (h *AgentConfigHandler) handlePatchAgentConfig(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	agent, id, ok := loadAgentByPath(w, r, h.Store)
	if !ok {
		return
	}
	// Owner-only ACL (acceptance §4.1.b).
	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var req agentConfigPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorCode(w, http.StatusBadRequest, "agent_config.invalid_payload", "invalid JSON body")
		return
	}
	if req.Blob == nil {
		writeJSONErrorCode(w, http.StatusBadRequest, "agent_config.invalid_payload", "blob field required")
		return
	}

	// 蓝图 §1.4 单一来源 设计 fail-closed: blob 仅含白名单字段, runtime-only 字段
	// (api_key / temperature / token_limit / retry_policy) 一律 reject.
	// acceptance §4.1.c reflect scan 同源.
	for k := range req.Blob {
		if !allowedConfigKeys[k] {
			writeJSONErrorCode(w, http.StatusBadRequest, "agent_config.runtime_field_rejected",
				"runtime-only field not allowed in agent config: "+k)
			return
		}
	}

	blobBytes, err := json.Marshal(req.Blob)
	if err != nil {
		h.logErr("agent_config blob marshal", err)
		writeJSONError(w, http.StatusInternalServerError, agentConfigSaveErrorMsg)
		return
	}

	now := h.now()
	// SQLite ON CONFLICT(agent_id) DO UPDATE atomic UPSERT — schema_version
	// monotonic per-row (server-stamp, 不接受 client 传 version, 防 race
	// 条件 last-write-wins 失效). acceptance §4.1.a 并发 2 写末次胜出 + 严格
	// 递增 + 无丢失.
	if err := h.Store.DB().Exec(`INSERT INTO agent_configs
		(agent_id, schema_version, blob, created_at, updated_at)
		VALUES (?, 1, ?, ?, ?)
		ON CONFLICT(agent_id) DO UPDATE SET
		  schema_version = agent_configs.schema_version + 1,
		  blob           = excluded.blob,
		  updated_at     = excluded.updated_at`,
		id, string(blobBytes), now, now).Error; err != nil {
		h.logErr("agent_config upsert", err)
		writeJSONError(w, http.StatusInternalServerError, agentConfigSaveErrorMsg)
		return
	}

	// Read back the new schema_version for response.
	var row agentConfigRow
	if err := h.Store.DB().Raw(`SELECT agent_id, schema_version, blob, created_at, updated_at
		FROM agent_configs WHERE agent_id = ?`, id).Scan(&row).Error; err != nil {
		h.logErr("agent_config readback", err)
		writeJSONError(w, http.StatusInternalServerError, agentConfigSaveErrorMsg)
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"schema_version": row.SchemaVersion,
		"blob":           req.Blob,
		"updated_at":     row.UpdatedAt,
	})

	// AL-2b §2.1 BPP fanout — server→plugin push of the new config delta.
	// Best-effort: failure here MUST NOT block the PATCH response (跟
	// DM-2.2 #372 mention dispatch 失败不阻 message 落库 同模式; plugin
	// reconnect 后 GET /agents/:id/config 主动拉最新, 跟蓝图 §1.5
	// "runtime 不缓存" 同源).
	//
	// idempotency_key 走 server-side stable derivation (agent_id +
	// schema_version) — plugin 端按此 key 去重 reload (acceptance §2.2
	// 字面跟随). 同 (agent_id, schema_version) 重发 N 次 → plugin reload
	// 行为只触发 1 次.
	if h.Pusher != nil {
		idemKey := id + ":" + strconv.FormatInt(row.SchemaVersion, 10)
		_, sent := h.Pusher.PushAgentConfigUpdate(id, row.SchemaVersion,
			row.Blob, idemKey, now)
		if !sent && h.Logger != nil {
			// plugin offline — frame dropped (反向约束: 不入队列). Audit
			// trail only, not an error from PATCH's POV.
			h.Logger.Info("agent_config push skipped (plugin offline)",
				"agent_id", id, "schema_version", row.SchemaVersion)
		}
	}
}

func (h *AgentConfigHandler) logErr(op string, err error) {
	if h.Logger != nil {
		h.Logger.Error("agent_config error", "op", op, "err", err)
	}
}
