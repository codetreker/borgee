# RT-0 Server Review Prep — 5 分钟过审 checklist

> 飞马 · 2026-04-28 · 战马A RT-0 server PR 预备 (≤ 500 LOC)
> 引: `docs/blueprint/current/realtime.md §2.3` + #218 client review (schema 已 lock)

## 1. 5 条盯点

| # | 盯点 | 看文件 | 通过条件 |
|---|------|--------|---------|
| S1 | frame schema byte-identical w/ #218 client | `internal/ws/event_schemas.go` (新) | `pending` 字段顺序: `invitation_id, requester_user_id, agent_id, channel_id, created_at, expires_at`; `decided`: `invitation_id, state, decided_at`; CI lint vs `bpp/frame_schemas.go` byte-identical (G2.6) |
| S2 | hub.PushAgentInvitationFrame 签名 | `internal/ws/hub.go` | 飞马倾向: `PushPending(invitee_id string, frame AgentInvitationPendingFrame)` + `PushDecided(requester_id string, frame AgentInvitationDecidedFrame)` typed (非 `interface{}`) — 编译期 schema 锁; 复用 `commands_updated` broadcast 通路 |
| S3 | handler 触发点 | `internal/api/agent_invitations.go` POST + PATCH | POST 后调 `hub.PushPending(invitation.OwnerUserID, ...)`; PATCH (approve/reject) 调 `hub.PushDecided(invitation.RequesterUserID, ...)`; expire 后台扫描同样调 PushDecided |
| S4 | e2e ≤ 3s stopwatch 解 skip | `packages/e2e/tests/cm-4-realtime.spec.ts` | 战马B #218 已写 skip 形, 本 PR 1-line 改 `describe.skip` → `describe`; fixture TODO 填真 IDs; CI 跑过 latency ≤ 3000ms |
| S5 | hub goroutine cleanup (战马C #204 雷) | `hub.go` | per-client send chan close 走 `defer close(ch)` 路径; `Broadcast` 用 `select { case ch <- frame: default: }` 非阻塞; client unregister 必走单一 owner goroutine, 不双 close |

## 2. 数据契约 + 接口契约

frame Go struct 字段顺序与 client `ws-frames.ts` 字面对应 (state enum `approved/rejected/expired`); `created_at/expires_at/decided_at` 为 Unix ms `int64` (与 client `number` 对齐); 不带 migration (RT-0 无 schema 改); `internal/presence/contract.go` 路径锁 (G2.5 一并落或 audit 跟踪)

## 3. 行为不变量 + LOC

- 邀请发出 → owner 端 latency ≤ 3s (e2e 4.1) · 跨 tab approve/reject 同步 (decided frame) · ≤ 500 LOC · 不动 schema (无 migration) · CI lint G2.6 必须本 PR 加 (`go vet` + reflect 比对 `bpp/frame_schemas.go` ↔ `ws/event_schemas.go`)

## 4. 拒收红线

❌ schema drift 跟 #218 client (CI lint 必须 fail-closed) · ❌ hub goroutine 没 cleanup (战马C #204 修过的雷不能再踩) · ❌ 带任何 migration (RT-0 无 schema 改) · ❌ frame 用 `map[string]interface{}` 或 `interface{}` 取代 typed struct (编译期 schema 锁丢) · ❌ Broadcast 阻塞 (慢 client 拖死整个 hub)
