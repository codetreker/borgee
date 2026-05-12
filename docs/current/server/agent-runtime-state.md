# server-go — agent runtime three-state model (AL-1a)

> AL-1a (#R3 Phase 2 start) · blueprint `agent-lifecycle.md §2.3` · not persisted until AL-3

## 1. Scope

Phase 2 only commits to the **online / offline + error-side** three-state model. busy / idle land with BPP (Phase 4 AL-1), because their source must be an upstream plugin frame; without BPP they would be stubs that v1 would later remove (2026-04-28 four-person review #5 decision).

## 2. Server API

| File | Role |
|------|------|
| `internal/agent/state.go` | `RuntimeState` enum + `Reason*` constants + `Tracker` (error map) + `ClassifyProxyError` |
| `internal/api/agents.go` | `AgentRuntimeProvider` interface + `withState` JSON merge + plugin-call error side path |
| `internal/server/server.go` | `agentRuntimeAdapter` combines `*ws.Hub.GetPlugin` + `*agent.Tracker` into a single query |

Behavior:

- **online**: `hub.GetPlugin(agentID) != nil` and there is no error record.
- **offline**: no plugin is online and there is no error record. This is the default.
- **error**: written by `Tracker.SetError(id, reason)`. This has highest priority; an error record overrides plugin presence so the owner does not see a green status when the runtime is actually unreachable.
- **disabled**: `users.disabled = true` forces offline (blueprint §2.4: disabled means no longer accepting messages).

## 3. Error-Side Trigger

`handleGetAgentFiles` calls `Hub.ProxyPluginRequest`; on failure, `ClassifyProxyError(status, err)` classifies the result:

| Signal | reason |
|------|--------|
| `status == 401` or err contains "api key" / "unauthorized" | `api_key_invalid` |
| `status == 429` | `quota_exceeded` |
| `status >= 500` | `runtime_crashed` |
| err contains "timeout" / "deadline exceeded" | `runtime_timeout` |
| err contains "not connected" / "connection refused" / "unreachable" | `network_unreachable` |
| any other non-empty err | `unknown` |

Non-empty reason → `setter.SetAgentError(id, reason)`. The owner sees the error banner and repair entry on the next GET.

## 4. JSON wire schema

GET `/api/v1/agents` / GET `/api/v1/agents/{id}` add these fields to the existing sanitized payload:

```
state              : "online" | "offline" | "error"   (always emit)
reason             : string (仅 error 态)
state_updated_at   : Unix ms (仅 error 态, error 时刻)
```

Copy lock is in `packages/client/src/lib/agent-state.ts` (#190 §11): "在线" / "已离线" / "故障 (api_key_invalid)" and related labels. Changing a reason string requires changing both sides plus `__tests__/agent-state.test.ts` lock assertions.

## 5. Out of Scope

- No migration. State lives only in the in-memory `Tracker` map. Restart clears it, and any owner-triggered plugin call reclassifies it.
- No busy / idle implementation. Without BPP, do not ship stubs.
- No active state-change push. The client relies on the existing RT-0 (#40) `/events` long-poll wakeup path; AL-1b (Phase 4 BPP cutover) can add a dedicated frame.

## 6. AL-4.2 — runtime process descriptor API (PR #414)

> AL-1a's online/offline/error state is in-memory and transient; AL-4.2 adds the `agent_runtimes` table (`schema_migrations` v=16, PR #398) for persistent plugin process descriptors, keeping that data separate from AL-1a's transient state (blueprint `agent-lifecycle.md §2.2`).

File: `internal/api/runtimes.go` (`RuntimeHandler` user rail + `AdminRuntimeHandler` admin rail, separated by mux).

Endpoints (acceptance §2 literals; owner-only unless noted):

```
POST /api/v1/agents/{id}/runtime/register   create agent_runtimes row
POST /api/v1/agents/{id}/runtime/start      transition status → running   (Permission: agent.runtime.control)
POST /api/v1/agents/{id}/runtime/stop       transition status → stopped (idempotent) (Permission: agent.runtime.control)
POST /api/v1/agents/{id}/runtime/heartbeat  plugin → server, update last_heartbeat_at (v0 简化为 owner-only)
POST /api/v1/agents/{id}/runtime/error      transition status → error + reason
GET  /api/v1/agents/{id}/runtime            owner-only metadata read
GET  /admin-api/v1/runtimes                 admin god-mode whitelist (no last_error_reason raw)
```

start + stop use a second guard: `auth.RequirePermission(s, "agent.runtime.control", nil)` middleware (acceptance §4.6 literal grep `RequirePermission..agent\.runtime\.control` count≥2 locks both paths).

Design cross-checks (al-4-spec.md §0 + acceptance §4):

- ① Borgee does not host the runtime: server stores only the process descriptor, not `llm_provider` / `model_name` / `api_key` / `prompt_template` (schema guard already in place in #398).
- ② Admin metadata only: the admin endpoint returns a whitelist and does not write; raw `last_error_reason` is not returned (admin-rail grep check `admin.*runtime.*start|admin.*runtime.*stop` count==0).
- ③ Runtime status is not presence: heartbeat writes `agent_runtimes.last_heartbeat_at`, not `presence_sessions` (separate from the AL-3 SessionsTracker boundary; schema guard already in place in #398, and the handler does not import `internal/presence` for writes).
- ④ Status DM copy is byte-identical: "{agent_name} 已启动" / "已停止" / "出错: {reason}" share the same lock as the three #321 tests.
- ⑤ Reasons reuse the AL-1a #249 six reason literals, plus AL-4 fail-closed stub reason `runtime_not_registered`; do not create another dictionary (`agent/state.go Reason*` + `lib/agent-state.ts REASON_LABELS` stay byte-identical).
- ⑥ Use the existing BPP-1 frame and do not split the namespace: register / start / stop **do not send** custom `runtime.start` / `runtime.stop` frame types (acceptance §4.4 grep check count==0).
