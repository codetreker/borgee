# AL-1a Implementation Note — agent runtime 三态

> 战马 A · #249 implement 后给野马 / 飞马 review 的速读卡, 不替代 [`agent-runtime-state.md`](agent-runtime-state.md) (5 节详版).

**三态 enum** (`internal/agent/state.go`): `Online` / `Offline` / `Error`. 蓝图 §2.3 四态去 busy/idle (4 人 review #5 决议 — 没 BPP 不准 stub).

**6 reason codes** (字面跟客户端 `lib/agent-state.ts` 锁定):

| code | 触发 (`ClassifyProxyError`) |
|------|------|
| `api_key_invalid` | status 401 / err 含 "api key" / "unauthorized" |
| `quota_exceeded` | status 429 |
| `runtime_crashed` | status ≥ 500 |
| `runtime_timeout` | err 含 "timeout" / "deadline exceeded" |
| `network_unreachable` | err 含 "not connected" / "connection refused" / "unreachable" |
| `unknown` | 其它非空 err 保底 |

**Tracker** — in-memory `map[agentID]Snapshot`, 仅存 error 行 (online/offline 由 `hub.GetPlugin` presence 推导). 不持久化, 重启全清; AL-3 Phase 4 落表的 hook 是 `Tracker` 接口形参化, 换 SQL backend 不动调用方.

**API** — `GET /api/v1/agents` / `GET /api/v1/agents/{id}` 返回:
```
state              : "online" | "offline" | "error"   (always)
reason             : string  (仅 error)
state_updated_at   : Unix ms (仅 error)
```
disabled agent 永远 offline (蓝图 §2.4 禁用 = 停接消息).

**文案锁定** (野马 #190 §11 + onboarding-journey.md §11): "在线" / "已离线" / "故障 (API key 失效)" 等. 改 reason 字符串 = 改 server `Reason*` 常量 + client `REASON_LABELS` + 单测断言, 三处同 PR.

**故障旁路触发点** — `handleGetAgentFiles` 调 plugin proxy 失败时, classifier 非空 reason → `setter.SetAgentError(id, reason)`. owner 下次 GET 立即看到红条 + 修复入口. AL-1b (Phase 4 BPP) 加专属 push frame.
