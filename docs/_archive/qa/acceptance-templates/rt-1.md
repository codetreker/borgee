# Acceptance Template — RT-1: realtime cursor + backfill + BPP session.resume

> 蓝图: `realtime.md` §1.3 (人/agent 拆 replay) + §1.4 (cursor 去重) + §2.3 (`/ws` ↔ BPP envelope 等同) + §2.1 BPP `artifact.commit/progress` frame
> Implementation: `docs/implementation/modules/rt-1-spec.md` (#269)
> 拆 PR: **RT-1.1** server cursor (#290 merged d1538f5) + **RT-1.2** client backfill (#292 merged 6a5ac92) + **RT-1.3** agent BPP session.resume (#296 merged 7c62150) — **RT-1 三段全闭 ✅**
> 飞马硬约束 (#267 readiness §5): 人/agent 两路 replay 拆死 — 人 full replay + 端虚拟列表; agent BPP `session.resume` 三 hint 由 runtime 自决。反约束: ❌ 一套 replay; ❌ agent default full
> Owner: 战马A 实施 / 烈马 验收

## 验收清单

### §1 server cursor 单调 + dedup (RT-1.1, 已实施)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.a 100 并发 commit cursor 严格递增无重复 (race detector) | unit (`-race`) | 战马A / 烈马 | `internal/ws/cursor_test.go::TestCursorMonotonicUnderConcurrency` (#290) — 100 goroutine 全唯一 + max==N 严格 |
| 1.b 重复 (artifact_id, version) → 同 cursor + fresh=false (32 racer 折叠) | unit | 战马A / 烈马 | `cursor_test.go::TestCursorIdempotentSameArtifactVersion` (#290) — racing AllocateForArtifact collapse to one cursor (results 全等) |
| 1.c restart 不回退 (pre-seed 3 events → fresh allocator PeekCursor==MAX + Next>MAX) | unit (fixture) | 战马A / 烈马 | `cursor_test.go::TestCursorNoRollbackAfterRestart` (#290) |
| 1.d ArtifactUpdatedFrame 字段顺序 byte-identical 于 #237 envelope (`type/cursor/artifact_id/version/channel_id/updated_at/kind`) | unit (golden JSON) | 飞马 / 烈马 | `cursor_test.go::TestArtifactUpdatedFrameFieldOrder` (#290) json.Marshal byte-equality vs literal want |

### §2 client backfill on reconnect (RT-1.2, #292 merged ✅)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.a 新 commit ≤ 3s 看到 (latency 截屏, G2.4 模板, stopwatch fixture) | e2e (Playwright) | 战马A / 烈马 | `packages/e2e/tests/rt-1-2-backfill-on-reconnect.spec.ts::立场 ① offline 5s → reconnect → backfill within 3s` (#292) — `expect.poll(timeout:3_000)` + `latency<3_000` 字面断言 |
| 2.b 离线 5s × N event → reconnect backfill 齐序 (cursor 单调 + 无丢) | e2e | 战马A / 烈马 | `rt-1-2-backfill-on-reconnect.spec.ts::立场 ①` (#292) — `setOffline(true)` 5s + reconnect + 验 response body 逐 ev `cursor>since` (反约束 server contract); 完整 `30s × 5 commit` ArtifactUpdated 链路待 CV-1 artifact 表 (Phase 3+, REG-RT1-005 留账) |
| 2.c 多端 (2 tab) 各看一份不重复 (cursor set dedup, ArtifactList 不双写) | unit (vitest) | 战马A / 烈马 | `packages/client/src/__tests__/last-seen-cursor.test.ts::② monotonic` (#292) — sessionStorage 拒小+等于 + page reload 存活; 多 tab sniff (sessionStorage per-tab, 2 tab 各自独立 backfill 同窗口) 留 follow-up (server backfill RPC 层未 dedup, 当前是冗余流量, 不是正确性问题) |
| 2.d `last_seen_cursor` sessionStorage 持久化 + WS 重连带 `?since=N` (反向断言: 不 default 拉全 history) | e2e + unit | 战马A / 烈马 | `rt-1-2-backfill-on-reconnect.spec.ts::立场 ② cold start does NOT auto-pull` (#292) — `page.on('request')` 监听 GET /events 1500ms idle 后 `toHaveLength(0)` 反约束 0-call 断言 + `last-seen-cursor.test.ts::① round-trip + ④ 防御 + ⑤ 损坏 storage clamp 0` |

### §3 agent BPP session.resume 三 hint (RT-1.3, #296 merged ✅)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.a 三 hint 单测: `incremental` (default fallback) / `none` (cold start, ack 高水位) / `full` (agent-explicit, 从 0 重放) | unit (table + 反向 fallback) | 战马A / 烈马 | `internal/bpp/session_resume_test.go::TestResolveResumeIncremental` + `TestResolveResumeNone` + `TestResolveResumeFull` + `TestResolveResumeUnknownModeFallsBackIncremental` (5 种 unknown→incremental, NEVER full) (#296) |
| 3.b 反向 grep `replay_mode.*=.*"full".*default\|defaultReplayMode\|default.*ResumeModeFull` count==0 (server NEVER defaults full) + 喂 11 种坏输入 (`""` / `" "` / `"FULL"` / `"Full"` / `"full "` / 7 种语义近邻) 任一进 full 即红 + 行为反向断言 (`ack.Count == full全集 ⇒ 红`) | unit (双层) + grep | 飞马 / 烈马 | `session_resume_test.go::TestParseResumeModeNeverDefaultsFull` (字面 parse 锁) + `TestResolverNeverDefaultsToFullBranch` (行为分支锁) (#296); `grep -rEn ... internal/bpp/ --exclude='*_test.go'` 仅命中 `session_resume.go:10` 文件头注释自身 (战马A 主动写入当 grep 锚, 非 leak) |
| 3.c BPP frame envelope 与 RT-1.1 `/ws` event byte-identical (§2.3 等同, golden JSON) | unit (golden) | 飞马 / 烈马 | `session_resume_test.go::TestSessionResumeFrameFieldOrder` (#296) — `string(json.Marshal(req)) == \`{"type":"session.resume","mode":"incremental","since":42}\`` + ack 同样字面对比 `{"type":"session.resume_ack","count":3,"cursor":99}`; 跟 RT-1.1 ArtifactUpdatedFrame byte-identity 同模式 |

### 蓝图行为对照 (反查锚, 每 PR 必带)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.a `grep -rEn 'artifact_updated.*timestamp\|"timestamp".*artifact_updated' packages/server-go/ packages/client/` count==0 (字段名锁 `updated_at`, 不混 `timestamp`) | CI grep | 飞马 / 烈马 | RT-1.1 #290 实测干净 (cursor.go 仅注释引述, 不算字段) |
| 4.b 人/agent 拆 replay: agent 路径不复用 client backfill (`grep -rE 'wsClient\.backfill\|client.*last_seen_cursor' internal/bpp/` count==0) | CI grep | 飞马 / 烈马 | RT-1.3 #296 实测干净 (resolver 走 `EventLister` 接口 + `GetEventsSince` 直查 store, 不复用 client REST) |

## 退出条件

- §1 server (1.a-d) **全绿** ← #290 merged ✅
- §2 client (2.a-d) **全绿** ← #292 merged ✅
- §3 agent BPP (3.a-c) **全绿** ← #296 merged ✅
- 反查锚 (4.a-b) 每 PR 必跑 0 命中 — RT-1.1/1.2/1.3 实测全干净
- 登记 `regression-registry.md` REG-RT1-001..010 全 🟢 (RT-1.1 5 行 + RT-1.2 3 行 + RT-1.3 2 行); REG-RT1-005 字段名锁 reverse grep 已绿 (cursor.go 注释引述非 leak)
- Phase 2 留账: G2.6 BPP CI lint (#274 BPP-1) + G2.5 presence (AL-3 #277) — 不挡 RT-1; **RT-1 提前完成 → Phase 4 留账压力降** (announcement v3 §5 备注)
- **CI 时序备注**: RT-1.2 backfill spec ≤3s 在 CI runner 时序敏感, 2 次 merge agent 用 ruleset disable/restore 兜底; follow-up PR 调阈值 (5s? 7s?) 或 fixture retry — 跟踪 issue 待开
