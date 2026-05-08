# BPP-1 Implementation Review Prep — 5 分钟过审 checklist

> 战马 A · Phase 4 第一周 BPP-1 协议骨架 PR 预备 · 参考 `docs/qa/rt-0-server-review-prep.md` 同模板
> 引: `docs/blueprint/bpp-protocol.md` (R3-4 锁) + `docs/current/server/bpp-protocol-overview.md` (#255) + #237 RT-0 子集

## 1. 5 条盯点

| # | 盯点 | 看文件 | 通过条件 |
|---|------|--------|---------|
| B1 | frame schema byte-identical 4 处 | 蓝图 §schemas / `internal/ws/event_schemas.go` / `packages/client/src/types/ws-frames.ts` / `docs/current/server/bpp-protocol-overview.md` | 字段顺序 + 字面 + Unix ms `int64` ↔ client `number` 对齐; CI lint G2.6 (Phase 4 加) `go vet` + reflect 比对 4 处 fail-closed |
| B2 | typed Push 接口 | `internal/ws/hub.go` | `PushAgentRuntimeState(owner_id, frame AgentRuntimeStateFrame)` 等 typed 签名 (非 `interface{}` / `map[string]any`); 复用 RT-0 #237 模式; 编译期 schema 锁 |
| B3 | reverse channel `POST /ws/upstream` handler | `internal/api/ws_upstream.go` (新) + 蓝图 §reverse | runtime → server frame 入口; auth via plugin token; dispatch by `type` 字段; BPP-2 升 WS bidirectional 时 handler 接口不动 |
| B4 | WS close + reconnect (战马C #204 雷) | `internal/ws/hub.go` + `internal/ws/plugin.go` | per-conn send chan `defer close`; Broadcast 走 `select { case: default: }` 非阻塞; runtime reconnect 后 Tracker.Clear (#249 hook) 重置 error 态; 双 close 防御单 owner goroutine |
| B5 | 不带 migration | (无 schema 改) | BPP 协议层只动 frame schema + handler; AL-3 落表 hook 已在 #249 `Tracker` 接口形参化, 不在本 PR 范围 |

## 2. 拒收红线

❌ schema drift 4 处任一 (CI lint G2.6 fail-closed) · ❌ frame 用 `map[string]any` / `interface{}` 取代 typed struct · ❌ 带 migration (BPP 是协议层) · ❌ hub Broadcast 阻塞或 send chan 双 close (#204 雷) · ❌ busy/idle 没 BPP frame 直接 stub (4 人 review #5 决议 — frame 落齐才解锁 #249 deferred 子态)

## 3. 烈马 acceptance template hook

`docs/qa/acceptance-templates/` Phase 4 templates index (#254) 加 BPP-1 item: 引本文件 §1 5 盯点 + §2 红线; e2e latency ≤ 3s stopwatch 复用 #218 模板 (RT-0 已绿); G2.6 lint CI 行落地后翻 audit row.
