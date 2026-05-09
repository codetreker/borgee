# server-go — agent runtime 三态 (AL-1a)

> AL-1a (#R3 Phase 2 起步) · 蓝图 `agent-lifecycle.md §2.3` · 不持久化, AL-3 才落表

## 1. 适用范围

Phase 2 只承诺 **online / offline + error 旁路** 三态. busy / idle 跟 BPP 同期 (Phase 4 AL-1) 落地——因为 source 必须是 plugin 上行 frame, 没 BPP 就只能 stub, stub 上 v1 要拆掉 = 白写 (2026-04-28 4 人 review #5 决议).

## 2. 服务端 API

| 文件 | 角色 |
|------|------|
| `internal/agent/state.go` | `RuntimeState` enum + `Reason*` 常量 + `Tracker` (error map) + `ClassifyProxyError` |
| `internal/api/agents.go` | `AgentRuntimeProvider` interface + `withState` JSON 折入 + plugin 调用故障旁路 |
| `internal/server/server.go` | `agentRuntimeAdapter` 把 `*ws.Hub.GetPlugin` + `*agent.Tracker` 合成单次查询 |

行为:

- **online**: `hub.GetPlugin(agentID) != nil` 且无 error 记录.
- **offline**: 没 plugin 在线, 也没 error 记录. 默认值.
- **error**: `Tracker.SetError(id, reason)` 写入. 优先级最高 (有 error 记录无视 plugin presence, 防 owner 看到"绿点 + 实际不通").
- **disabled**: `users.disabled = true` 时强制 offline (蓝图 §2.4 禁用 = 停接消息).

## 3. 故障旁路触发点

`handleGetAgentFiles` 调 `Hub.ProxyPluginRequest`, 失败时 `ClassifyProxyError(status, err)` 分类:

| 信号 | reason |
|------|--------|
| `status == 401` 或 err 含 "api key" / "unauthorized" | `api_key_invalid` |
| `status == 429` | `quota_exceeded` |
| `status >= 500` | `runtime_crashed` |
| err 含 "timeout" / "deadline exceeded" | `runtime_timeout` |
| err 含 "not connected" / "connection refused" / "unreachable" | `network_unreachable` |
| 其它非空 err | `unknown` |

非空 reason → `setter.SetAgentError(id, reason)`. owner 下次 GET 立即看到红条 + 修复入口.

## 4. JSON wire schema

GET `/api/v1/agents` / GET `/api/v1/agents/{id}` 在原 sanitize 字段上多加:

```
state              : "online" | "offline" | "error"   (always emit)
reason             : string (仅 error 态)
state_updated_at   : Unix ms (仅 error 态, error 时刻)
```

文案锁定见 `packages/client/src/lib/agent-state.ts` (野马 #190 §11): "在线" / "已离线" / "故障 (api_key_invalid)" 等. 改 reason 字符串 = 改两边 + 改 `__tests__/agent-state.test.ts` 锁定断言.

## 5. 不在范围

- 不带 migration. 状态全部驻 `Tracker` map (内存). 重启全清, owner 触发任意 plugin 调用即重新分类.
- 不实现 busy / idle. 没 BPP 不能 stub.
- 不主动 push state 变更. 客户端依赖 RT-0 (#40) 已有的 `/events` long-poll wakeup 路径; AL-1b (Phase 4 BPP cutover) 再考虑专属 frame.

## 6. AL-4.2 — runtime process descriptor API (PR #414)

> AL-1a 三态是内存瞬时态 (online/offline/error); AL-4.2 落 `agent_runtimes` 表 (`schema_migrations` v=16, PR #398) — plugin process descriptor 持久化, 跟 AL-1a 内存态拆死 (蓝图 `agent-lifecycle.md §2.2`)。

文件: `internal/api/runtimes.go` (`RuntimeHandler` user-rail + `AdminRuntimeHandler` admin-rail 双 mux 隔离)。

Endpoints (acceptance §2 字面, owner-only 除标注):

```
POST /api/v1/agents/{id}/runtime/register   create agent_runtimes row
POST /api/v1/agents/{id}/runtime/start      transition status → running   (Permission: agent.runtime.control)
POST /api/v1/agents/{id}/runtime/stop       transition status → stopped (idempotent) (Permission: agent.runtime.control)
POST /api/v1/agents/{id}/runtime/heartbeat  plugin → server, update last_heartbeat_at (v0 简化为 owner-only)
POST /api/v1/agents/{id}/runtime/error      transition status → error + reason
GET  /api/v1/agents/{id}/runtime            owner-only metadata read
GET  /admin-api/v1/runtimes                 admin god-mode whitelist (no last_error_reason raw)
```

start + stop 二次防护 = `auth.RequirePermission(s, "agent.runtime.control", nil)` middleware (acceptance §4.6 字面 grep `RequirePermission..agent\.runtime\.control` count≥2 锁定两路命中)。

设计原则反查 (al-4-spec.md §0 + acceptance §4):

- ① Borgee 不带 runtime: server 仅记 process descriptor, 不存 `llm_provider` / `model_name` / `api_key` / `prompt_template` (schema 闸位已就位 #398).
- ② admin god-mode 元数据 only: admin endpoint 返白名单不写; `last_error_reason` raw 不返 (admin-rail grep 检查 `admin.*runtime.*start|admin.*runtime.*stop` count==0).
- ③ runtime status ≠ presence: heartbeat 写 `agent_runtimes.last_heartbeat_at` 不写 `presence_sessions` (跟 AL-3 SessionsTracker 边界真分清 — schema 闸位已就位 #398, handler 不 import `internal/presence` 写表).
- ④ status DM 文案锁定 byte-identical: "{agent_name} 已启动" / "已停止" / "出错: {reason}" 跟野马 #321 三处单测同源.
- ⑤ reason 复用 AL-1a #249 6 reason 枚举字面 + AL-4 stub fail-closed 加 `runtime_not_registered` 第 7 reason — 不另起字典 (跟 `agent/state.go Reason*` + `lib/agent-state.ts REASON_LABELS` byte-identical).
- ⑥ 走 BPP-1 既有 frame 不拆 namespace: register / start / stop **不发** `runtime.start` / `runtime.stop` 自造 frame type (acceptance §4.4 grep 检查 count==0).

