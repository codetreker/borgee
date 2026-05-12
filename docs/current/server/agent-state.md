# AL-1a Implementation Note — agent runtime three-state model

> Workstream A · #249 implementation review card for reviewer handoff; does not replace [`agent-runtime-state.md`](agent-runtime-state.md) (full five-section version).

**Three-state enum** (`internal/agent/state.go`): `Online` / `Offline` / `Error`. Blueprint §2.3 removed busy/idle from the four-state draft (four-person review #5 decision: without BPP, do not ship a stub that v1 would later remove).

**6 reason codes** (literals locked with client `lib/agent-state.ts`):

| code | Trigger (`ClassifyProxyError`) |
|------|------|
| `api_key_invalid` | status 401 / err contains "api key" / "unauthorized" |
| `quota_exceeded` | status 429 |
| `runtime_crashed` | status ≥ 500 |
| `runtime_timeout` | err contains "timeout" / "deadline exceeded" |
| `network_unreachable` | err contains "not connected" / "connection refused" / "unreachable" |
| `unknown` | fallback for any other non-empty err |

**Tracker** — in-memory `map[agentID]Snapshot`, storing only error rows (online/offline are derived from `hub.GetPlugin` presence). It is not persisted and is cleared on restart; AL-3 Phase 4 moves storage behind the `Tracker` interface so callers do not change when the SQL backend is added.

**API** — `GET /api/v1/agents` / `GET /api/v1/agents/{id}` 返回:
```
state              : "online" | "offline" | "error"   (always)
reason             : string  (仅 error)
state_updated_at   : Unix ms (仅 error)
```
disabled agents are always offline (blueprint §2.4: disabled means no longer accepting messages).

**Copy lock** (#190 §11 + onboarding-journey.md §11): "在线" / "已离线" / "故障 (API key 失效)" and related labels. Changing a reason string requires changing server `Reason*` constants, client `REASON_LABELS`, and test assertions in the same PR.

**Error-side trigger** — when `handleGetAgentFiles` calls the plugin proxy and the classifier returns a non-empty reason, the handler calls `setter.SetAgentError(id, reason)`. The owner sees the error banner and repair entry on the next GET. AL-1b (Phase 4 BPP) adds a dedicated push frame.
