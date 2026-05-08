# AL-3 spec brief — presence (在线/离线 真状态) 配套 #277 stub 接力

> 飞马 · 2026-04-28 · ≤80 行 spec lock (实施视角拆 PR 由战马A 落)
> **蓝图锚**: [`agent-lifecycle.md`](../../blueprint/current/agent-lifecycle.md) §2.3 (四态机 — online/offline/error/busy-idle 含离线态) + §2.2 (默认 remote-agent — 走 /ws hub 心跳决定 reach); [`realtime.md`](../../blueprint/current/realtime.md) §2.3 (`/ws` ↔ BPP envelope 等同 — presence 状态变更不进 envelope, 走独立 push)
> **关联**: G2.5 留账闸 #277 (`internal/presence/contract.go` PresenceTracker 接口 stub merged) + #267 readiness §5 (presence 与 RT-1 拆死, 不复用 events 路径)

> ⚠️ 锚说明: 蓝图 agent-lifecycle.md 章节到 §5 为止, 无独立 §3 presence 段; 此 spec 按字面对齐 §2.2 默认远程 + §2.3 四态机的 \"reach\" 语义, 不重新编号蓝图。

## 0. 关键约束 (3 条立场, 蓝图字面 + #277 接力)

1. **接口已锁, 写端待加**: PresenceTracker 接口 read-only 端 (`IsOnline` + `Sessions`) 在 #277 占号 merged; AL-3 真实施加写端 (`TrackOnline(userID, sessionID)` / `TrackOffline(userID, sessionID)`) — **不改已锁 read 签名**, 仅在同包扩
2. **Presence 路径与 RT-1 拆死**: presence 状态变更不进 `/ws` ArtifactUpdated envelope (那是 store events), 走独立 `presence.changed` frame; **反约束**: 不复用 cursor 序列 (presence 是瞬时态, 不持久化历史; RT-1 cursor 是不可回退序列, 两者数据特性不同)
3. **多 session per user 合法**: 一人多端 (web tab + mobile + plugin) 各占一行 `presence_sessions`, `IsOnline=COUNT(*)>0`; 关闭最后一个 session 才标 offline (蓝图 §2.2 \"plugin/web 双轨\")

## 1. 拆段实施 (AL-3.1 / 3.2 / 3.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **AL-3.1** schema | `presence_sessions` 表 (`user_id` / `session_id` UNIQUE / `connected_at` / `last_heartbeat_at`); migration v=12; 索引 `idx_presence_sessions_user_id` (IsOnline O(1) lookup 必需) | 待 PR (战马A) | 战马A |
| **AL-3.2** server hub WS lifecycle hook | `internal/ws/hub.go` 连接建立 → `TrackOnline`; 关闭 → `TrackOffline`; heartbeat 周期更 `last_heartbeat_at`; PresenceTracker 真实施 (替占号 stub 实现, 接口签名不变) | 待 PR (战马A) | 战马A |
| **AL-3.3** client UI presence dot | sidebar 用户列表 + `🟢/⚫` dot; agent 列表同语义 (立场 ⑥ — agent 跟人同 UI); WS 收 `presence.changed` frame 实时刷 | 待 PR (战马A) | 战马A |

## 2. 与 RT-1 + CHN-1 + DM-2 留账冲突点

- **RT-1 cursor 不复用**: `presence_sessions` 不挂 cursor 列, 不进 `events` 表; 客户端 reconnect 不走 `?since=N` 拉 presence 历史 — 走 \"reconnect → 重新订阅当前 snapshot\" 路径 (反 cursor backfill 模式)
- **CHN-1 agent silent**: `silent=true` (CHN-1.1) 跟 presence 状态独立 — silent 锁的是 \"不发 system message\", 不锁 \"是否在线\"; 即 silent agent 也有 online/offline 状态走 presence dot UI
- **DM-2 离线 fallback**: DM-2.2 system DM 触发条件 = `!presenceTracker.IsOnline(agent.id)` — AL-3 落地后 DM-2 不再依赖 stub 永远 false 的 placeholder, 真实施前 DM-2 fallback 永远不触发 (蓝图行为差距, 留账)

## 3. 反查 grep 锚 (Phase 4 验收)

```
git grep -n 'presence_sessions'                packages/server-go/internal/migrations/    # ≥ 1 hit (AL-3.1)
git grep -n 'TrackOnline\|TrackOffline'        packages/server-go/internal/presence/      # ≥ 2 hit (AL-3.2 写端方法)
git grep -n 'TrackOnline\|TrackOffline'        packages/server-go/internal/ws/hub.go      # ≥ 2 hit (lifecycle hook)
git grep -nE 'presence_sessions.*cursor'       packages/server-go/                         # 0 hit (反约束: presence 不挂 cursor, 跟 RT-1 拆死)
git grep -nE 'IsOnline\([^)]+\.id\)'           packages/server-go/internal/api/messages.go # ≥ 1 hit (DM-2 fallback 真接 AL-3, 不再 placeholder)
git grep -n 'presence\.changed\|presence_changed' packages/server-go/internal/ws/event_schemas.go # ≥ 1 hit (独立 frame, 不进 ArtifactUpdated envelope)
```

任一 0 hit (除反约束行) → CI fail, 视作蓝图立场被弱化 / 跟 RT-1 边界混淆。

## 4. 不在本轮范围 (反约束)

- ❌ Typing indicator (\"someone is typing...\") — 是不同 subsystem, 走临时 ephemeral signal, 不进 presence 表
- ❌ Read receipt (消息已读时间戳) — 走 `message_reads` 独立表 (DM-2 后续), 不复用 presence
- ❌ Last seen timestamp UI (\"last active 3 hours ago\") — 立场争议 (隐私 vs 体验), 留 Phase 5+ 业主反馈
- ❌ Cross-org presence visibility — 默认 org-scoped, 跨 org agent 走 §4.2 邀请审批 (DM-2 留账); 跨 org IsOnline 查询返回 false (隐私默认)
- ❌ Presence history / audit log — 瞬时态不持久化历史 (跟 RT-1 cursor 不可回退序列拆死)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- AL-3.1: migration v=11 → v=12 双向 + UNIQUE(session_id) 反向 (重复 session reject)
- AL-3.2: hub onConnect/onClose unit (mock WS) + heartbeat 周期 unit (clock fixture, 跟 G2.3 节流模式同) + 多 session per user `IsOnline=true if COUNT≥1` (table-driven)
- AL-3.3: e2e dot 渲染 + WS `presence.changed` frame 触发 dot 翻态 (跟 RT-1.2 #292 的 reconnect→backfill 同 e2e 模式, 但走 presence 独立 frame)
