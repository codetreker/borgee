# RT-1 Spec — realtime push protocol (artifact 推送 + cursor 单调)

> **范围**: blueprint [`realtime.md`](../../blueprint/current/realtime.md) §1.3 (人/agent 拆 replay) + §1.4 (cursor 去重) + §2.3 (`/ws` ↔ BPP envelope 等同) + §2.1 BPP `artifact.commit/progress` frame。
> **不在本文件**: artifact 表 / 版本 (CV-1) / channel schema (CHN-1) / BPP CI lint (BPP-1)。
> **依赖**: Phase 2 闸 4 ✅ + CHN-1 + CV-1.1 merged。**总工期**: 6-7 天 (server 4-5 + client 2-3), ≤ 3 PR / 每 PR ≤ 3 天。

## 0. 关键约束 (飞马硬约束, readiness review §5 锁)

- **`ArtifactUpdated` envelope = #237 同款** (字段名/顺序 byte-identical, server cursor 单调 + client dedup); 反约束: ❌ 自造 envelope; ❌ client timestamp 排序。
- **BPP-1 envelope CI lint 必须先落** (战马B 的活); 否则 RT-1 frame 不合规, BPP-1 上线整片 RT-* 翻车。RT-1 期间走**飞马 review 闸位人工 lint**。
- **人/agent 两路 replay 拆死** (§1.3 ⭐): 人 full replay + 端虚拟列表; agent BPP `session.resume` 三 hint 由 runtime 自决。反约束: ❌ 一套 replay; ❌ agent default full。

## 1. 拆 PR

### RT-1.1 — server cursor 单调 + dedup (2d, 战马A / 飞马 / 烈马)

- **Touches**: `internal/ws/cursor.go` (新, atomic + persist) + `event_schemas.go` 加 `artifact_updated` payload + handler + 单测
- **范围**: (1) cursor 单调发号 (atomic int64, persist `events.cursor`, restart 不回退); (2) `ArtifactUpdated{cursor, artifact_id, version, channel_id, updated_at, kind}` 与 #237 同序; (3) hub.Broadcast 全推; (4) 同 `(artifact_id, version)` 重发 → 同 cursor (idempotent)
- **Acceptance — 4.1 行为不变量 + cursor 单调单测**: (a) 100 并发 commit cursor 严格递增无重复 (race detector); (b) 重复 commit 同 cursor (fail-closed); (c) restart 不回退 (fixture); (d) frame 字段顺序反向 grep byte-identical 于 #237

### RT-1.2 — client backfill on reconnect (2d, 战马A / 烈马)

- **Touches**: `packages/client/src/realtime/wsClient.ts` + `pages/Channel/ArtifactList.tsx` + `api/events.ts` (GET `/api/events?since=N`) + e2e
- **范围**: (1) `last_seen_cursor` localStorage, WS open 带 `?since=N`; (2) 缺洞 → backfill 拉, 按 cursor 排序; (3) 已渲染 cursor set dedup; (4) ArtifactList 局部刷新
- **Acceptance — e2e (Playwright)**: (1) 新 commit ≤ 3s 看到 (latency 截屏, G2.4 模板); (2) 离线 30s × 5 commit → reconnect backfill 齐序; (3) 多端 (2 tab) 各看一份不重复

### RT-1.3 — agent BPP `session.resume` hint (2d, 与 1.2 可并行)

- **Touches**: `internal/bpp/session_resume.go` (新) + `bpp/frame_schemas.go` (与 RT-1.1 envelope 一致) + 单测
- **范围**: server 接 `session.resume{replay_mode, since_cursor}` 路由三 hint: `full` 推全; `summary` 推 `{missed_count}`; `latest_n` 推最近 N。**不**走 client backfill 路径 (人/agent 拆, §1.3)。runtime 自决 hint, server **不** default `full`。
- **Acceptance — 4.1 行为不变量**: (a) 三 hint table-driven 单测; (b) grep `replay_mode.*=.*\"full\".*default|defaultReplayMode` count==0; (c) BPP frame envelope 与 RT-1.1 ws event byte-identical

## 2. Phase 2 留账冲突点

- **G2.6 BPP CI lint 留账 (Phase 4 BPP-1)**: RT-1.1/1.3 envelope 走飞马人工 lint 闸位, 不接受"等以后"
- **G2.5 presence/contract.go 留账 (AL-3)**: RT-1 不碰 presence (拆干净)
- **#237 invitation envelope 已落**: RT-1.1 `artifact_updated` 套同模板字段顺序对齐 (反向 grep 验)

## 3. 反查锚 (lint-able, 每 PR 必带, 全 0 命中)

```bash
grep -rnE "artifact_updated.*timestamp|sort.*ArtifactUpdated.*time" packages/server-go/internal/ws/ | grep -v _test.go
grep -rnE "events\.sort\(.*createdAt|sort.*events.*timestamp" packages/client/src/realtime/ | grep -v _test.
grep -rnE "replay_mode.*=.*\"full\"|defaultReplayMode" packages/server-go/internal/bpp/ | grep -v _test.go
```

## 4. 不在范围 / 验收挂钩

- ❌ BPP-1 CI lint (战马B) / §1.4 (B) 智能推 / §1.4 (C) per-device / artifact 表 (CV-1) / presence (AL-3)
- 三 PR merge + §3 grep 0 + #237 envelope byte-identical → REG-RT1-001..009 → Phase 3 G3.1 闸接 RT-1.2 e2e 三用例
