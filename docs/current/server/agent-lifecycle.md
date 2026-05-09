# Agent Lifecycle Implementation Note — presence + WS lifecycle hook

> 战马 A · #317 implement 后给野马 / 飞马 review 的速读卡 (规则 6 docs/current 留账补丁).
> 关联: 蓝图 `docs/blueprint/current/agent-lifecycle.md` §2.3 / acceptance `docs/qa/acceptance-templates/al-3.md` / 数据 `docs/current/server/data-model.md` (`presence_sessions` v=12).
> 范围: AL-3.1 schema (#310) + AL-3.2 hub WS lifecycle hook 写端 (#317). AL-3.3 client UI dot + 5s+60s 节流 + presence.changed frame 留下一阶段.

## §AL-3 presence_sessions + PresenceWriter / PresenceTracker

**接口拆 (write/read 双 side)** — `internal/presence/`:

| 接口 | 方法 | 实现 | 调用方 |
|---|---|---|---|
| `PresenceTracker` (read, 锁 #277 byte-identical) | `IsOnline(userID) bool` + `Sessions(userID) []string` | `*SessionsTracker` | DM-2 mention fallback / sidebar / admin god-mode |
| `PresenceWriter` (write, AL-3.2 新增) | `TrackOnline(userID, sessionID, agentID *string)` + `TrackOffline(sessionID)` | `*SessionsTracker` | `ws.Hub` 唯一调用方 |

编译期双锁: `var _ PresenceTracker = (*SessionsTracker)(nil)` + `var _ PresenceWriter = (*SessionsTracker)(nil)`. 接口拆 → 调用方按职责依赖, 不绕表读 (反约束: AST grep `internal/ws/` 非测试 .go 不出现 `presence_sessions` 字面量).

**`presence_sessions` 表** (migration v=12, #310):
- `id` PK + `session_id` UNIQUE NOT NULL + `user_id` NOT NULL + `connected_at` NOT NULL + `last_heartbeat_at` NOT NULL + `agent_id` nullable
- INDEX `idx_presence_sessions_user_id` (full, IsOnline O(1) lookup)
- INDEX `idx_presence_sessions_agent_id` (partial WHERE agent_id IS NOT NULL — DM-2 mention 路径热查, 仅 role='agent' 行入索引)

**Hub WS lifecycle hook** (`internal/ws/hub.go` AL-3.2 #317):
- `Hub.SetPresenceWriter(w PresenceWriter)` — boot wiring 在 `server.go::New` (`hub.SetPresenceWriter(presence.NewSessionsTracker(s.DB()))`)
- `Hub.Register(client)` → `presenceWriter.TrackOnline(userID, sessionID, agentID)` (`agentID` 仅 `client.user.Role == "agent"` 时填指针, 否则 nil → partial index 不入)
- `Hub.Unregister(client)` → `presenceWriter.TrackOffline(sessionID)` (defer-based, 唯一 teardown 入口 → panic / ctx-cancel / normal close 三路均走)

**多端 last-wins** — `web+mobile+plugin` 多 session 同 user 共存合法 (UNIQUE 在 session_id 而非 user_id), `IsOnline` 仅 last `TrackOffline` 才返 false. 反约束: 单 session close 不误判 offline; 多端是实施细节, 上层 API 仍单 bool (设计 ⑥).

**容错策略**:
- `TrackOnline` 失败仅 log, **不阻断** in-memory broadcast — DB 抖动不能拒服务 (`TestPresenceLifecycle_TrackOnlineFailureDoesNotAbort`).
- `TrackOffline` 对未知 sessionID soft no-op (`TestTrackOffline_UnknownSessionIsSoftNoop`).
- `Hub.SetPresenceWriter` 未调用时 (nil writer) 全 no-op, 老 fixture 直跑不 panic (`TestPresenceLifecycle_NilWriterIsNoop`).

**测试覆盖** (#317 8 test):
- `presence/tracker_test.go` 6 test: WritesRow / AgentIDPartialIndex / MultiSessionLastWins / UnknownSessionIsSoftNoop / DuplicateSessionIDIsUnique / RejectsEmptyArgs.
- `ws/hub_presence_test.go` 6 test (含 fakePresenceWriter): HumanRegisterTrackOnline / AgentRoleSetsAgentID / MultiSessionLastWins / DeferUntrackOnPanic / TrackOnlineFailureDoesNotAbort / NilWriterIsNoop.
- `ws/hub_presence_grep_test.go` 1 AST scan: TestPresenceLifecycle_NoDirectTableRead (go/parser, 非测试 .go 不出现 `presence_sessions` 字面量, 强制走 PresenceWriter 接口).

## 后续留账 (AL-3.3)

- 5s presence 变更节流 (clock fixture, 跟 G2.3 同模式) + 60s 心跳超时 → 标 offline (REG-AL3-008).
- `presence.changed` frame 独立路径 + 字段白名单 `{agent_id, status, reason?}` (REG-AL3-008, 设计 ⑥ 不带 last_heartbeat_at / connection_count / endpoints[]).
- Client SPA agent 行 dot DOM lock + 文案守 §11 "已离线" 不准灰点不说原因 (REG-AL3-010).
- Admin god-mode 元数据白名单 (`/admin-api/agents/:id` 无 in-flight / active_channel_ids), 等 ADM-2 实施 (REG-AL3-010).
