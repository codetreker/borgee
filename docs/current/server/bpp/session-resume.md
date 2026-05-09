# BPP `session.resume` — implementation note

> RT-1.3 (#293) · Phase 4 BPP-1 子集 · 跟 RT-1.1 (#290) cursor + RT-1.2 (#292) client backfill 同语义, 错段在 wire (BPP runtime ↔ server) 不在 REST.

## 1. 设计

Agent runtime ↔ server 重连后的 replay 握手. 三 mode:

- **`incremental`** (default) — replay events 严格 `cursor > since`, 跟 RT-1.2 client backfill (`GET /api/v1/events?since=N`) byte-equivalent. 任何 well-formed reconnect 走这条.
- **`none`** — cold start. runtime 显式不要补; server 回 ack `count=0 + cursor=high_water`, 不查 store.
- **`full`** — agent 显式要求 (runtime 丢了 durable state). 从 cursor=0 在 channel scope 内重放. **人 client 不许走这条**.

## 2. 反约束 (hardline)

> server **NEVER defaults** caller into `full`. 空字符串 / 未知 / 大小写错 (`FULL`, `Full`) 一律 fallthrough 到 `incremental`. 唯一进 `Full` 分支的输入是字面量 `"full"`.

实现锁:

- `ParseResumeMode` (`session_resume.go`) — `default:` 返回 `ResumeModeIncremental`, **不**返回 `ResumeModeFull`.
- grep 检查 `replay_mode.*=.*"full"|defaultReplayMode|default.*ResumeModeFull` 在 `internal/bpp/` (排除 `_test.go`) 必须为空, 仅 `case ResumeModeFull: return ResumeModeFull` 命中 (字面量解析路径, **不是** default).
- 单测 `TestParseResumeModeNeverDefaultsFull` + `TestResolverNeverDefaultsToFullBranch` — 喂 11 种坏输入, 任何一个进 full 分支立即红.

## 3. Frame schema (byte-locked)

```go
// session_resume request
{"type":"session.resume","mode":"incremental","since":42}

// session.resume_ack response (events follow as separate frames)
{"type":"session.resume_ack","count":3,"cursor":99}
```

字段顺序锁跟 #237 invitation envelope + RT-1.1 `artifact_updated` 一致 (`type` 在前, semantic IDs 紧跟). 加字段必须 client / agent SDK 同 PR 同步, CI lint G2.6 (Phase 4 加) catch drift.

## 4. 实现 surface

- `internal/bpp/frame_schemas.go` — `SessionResumeRequest` / `SessionResumeAck` / `ResumeMode` enum.
- `internal/bpp/session_resume.go` — `ResolveResume(es, req, channelIDs, limit)` 是 wire-layer-agnostic resolver. plugin WS handler 调用即可, 不在此包写 IO.
- `EventLister` interface (内部 alias of `*store.Store`) — 单测用 fake, 不需要 SQLite fixture.
- limit 钳制 — `<=0` → `DefaultResumeLimit (200)`, `>500` → `MaxResumeLimit (500)`. 跟 RT-1.2 REST endpoint 同值.
- `since < 0` 当 0 处理 (incremental 模式), 防 underflow 触发 full scan.
- 空 channel scope → `ErrNoChannelScope`, ack(0, high-water), 不查 store.

## 5. 跟 RT-1.1 / RT-1.2 的关系

- **cursor 单调** — 由 RT-1.1 `CursorAllocator` 保证. 此包不分配 cursor, 只读.
- **client backfill** — 人 client 走 RT-1.2 `GET /api/v1/events?since=N`, **不**走 BPP. BPP 只给 agent runtime.
- **events 永不按 timestamp 排** — 反约束 跟 RT-1.x 一致 (cursor IS the order).

## 6. 不动什么

- 没 migration. BPP 是协议层.
- 没 wire IO. plugin WS handler (`internal/ws/plugin.go`) 调 resolver, IO 在 ws package.
- 没 client UI. agent runtime SDK 在 Phase 4 接.
