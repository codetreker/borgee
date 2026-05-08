# Acceptance Template — RT-3 ⭐ multi-device fanout + 活物感 + thinking subject 反约束

> Spec brief `rt-3-spec.md` (飞马 v0). Owner: 战马E 实施 / 飞马 review / 烈马 验收. v0 server hook 已 merge (PR #588), 本 v1 batch 接 RT-3.3 client UI + RT-3.4 DL-4 fallback + RT-3.5 e2e + 5 截屏 demo follow-up.
>
> **RT-3 ⭐ 范围**: 多端全推 + 活物感 + thinking subject 必带非空反约束. **0 schema 改 + 0 新错码** (复用 BPP-2.2 ValidateTask* SSOT). 立场承袭 RT-1.1 #290 cursor opaque + BPP-2.2 #485 task_lifecycle SSOT + AL-1a #249 6-dict reason + thinking 5-pattern 锁链第 11 处源头.

## 验收清单

### §1 schema 验收 (0 改, 复用既有)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 0 schema 改 — RT-3 全 milestone 0 migration / 0 新表 / 0 column ALTER | git diff | `git diff main -- packages/server-go/internal/migrations/` ==0 行 ✅ |
| 1.2 复用既有 frame schema (`AgentTaskStateChangedFrame` 在 RT-3.1 已 land) — RT-3.2 hook + RT-3.3 client + RT-3.5 e2e 全走既有 frame byte-identical | grep | reverse grep `agent_task_state_changed` frame schema 字面 byte-identical 跨 server/client/e2e |

### §2 server 验收 (cursor 复用 + multi-device fanout + presence 持久化)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 cursor 复用 RT-1.1 #290 hub.cursors.NextCursor 单源 (RT-3 第 6 个 frame 共序锁, 不另起 allocator) | unit | `agent_task_state_changed_frame_test.go::TestRT3_PushAgentTaskStateChanged_NoCursorAllocator` + `_SharedSequence_WithRT1_CV2_DM2_CV4_AL2b` PASS ✅ (v0 已 🟢) |
| 2.2 multi-device fanout 单源 hub.onlineUsers (`Hub.PushAgentTaskStateChanged` 走 BroadcastToChannel 自动 multi-device, 反向 grep `device.*route\|device_id.*push\|device_session.*broadcast` 在 internal/ws 0 hit) | unit + grep | reverse grep 3 pattern 0 hit ✅ + P1MultiDeviceWebSocket #197 fanout 模式承袭 (v0 已 🟢) |
| 2.3 thinking subject 必带非空 fail-closed (蓝图 §1.1 ⭐ 关键纪律) — `TaskLifecycleHandler.HandleStarted` 走 BPP-2.2 ValidateTaskStarted SSOT, errSubjectEmpty 同源, empty subject reject + pusher.calls == 0 锁 | unit | `task_lifecycle_handler_test.go::TestRT3_HandleStarted_EmptySubjectRejected` + `_StartedAdapter_EmptySubject_PreservesSentinelChain` PASS ✅ (v0 已 🟢) |
| 2.4 反向断言 0 endpoint 行为改 + admin god-mode 不下发 (ADM-0 §1.3) | grep | reverse grep `admin.*PushAgentTaskStateChanged` admin*.go 0 hit ✅ (v0 已 🟢) |

### §3 client 验收 (presence DOM + e2e + 反 typing-indicator 漂)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 RT-3.3 client UI — `wsClient.ts` switch case `agent_task_state_changed` + `AgentActivityDot.tsx` busy/idle/error 三态 DOM (data-attr `data-agent-state` byte-identical content-lock 锁) | vitest | `RT3PresenceDot.test.tsx` 9 case PASS + `rt3-content-lock-reverse-grep.test.ts` 4 case PASS + DOM data-attr `data-rt3-{presence-dot,last-seen,cursor-user}` byte-identical ✅ |
| 3.2 RT-3.5 Playwright e2e 多 tab presence 真测 — 1 user 多 tab fanout + busy-idle + offline-fallback + reject + multi-device (5 case) | E2E | `packages/e2e/tests/rt-3-presence.spec.ts` 5 case PASS (Playwright `--max-failures=2 --workers=1` 真跑通 server-go 4901 + vite 5174) ✅ |
| 3.3 反 typing-indicator 同义词漂 — 反向 grep `typing\|isTyping\|onTyping\|typing-indicator\|正在输入\|输入中` 在 client/src/ + e2e/ 全清 0 hit (除 user-typing 域 anchor 合规白名单) | grep | `rt3-content-lock-reverse-grep.test.ts::§3` 9 同义词 (英 5 + 中 4) 0 hit + `rt3_presence_reverse_grep_test.go::TestRT3_NoTypingIndicator_InRT3Paths` PASS ✅ |

### §4 closure 验收 (REG + cov gate + 5 截屏 demo)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 既有全包 unit + e2e + vitest 全绿不破 + post-#614 haystack gate 三轨过 (Func=50/Pkg=70/Total=85) | full test + CI | `go test -tags sqlite_fts5 -timeout=300s ./...` + Playwright + vitest 全 PASS + go-test-cov SUCCESS |
| 4.2 RT-3.4 DL-4 Web Push fallback 接 (DL-4 #490 已 merge) — offline recipient 走 Web Push 不靠 ws | unit + e2e | `dl_4_push_test.go::TestDL4_RT3OfflineFallback` + e2e offline-fallback case PASS |
| 4.3 ⭐ 5 截屏 demo (yema G4.x) — multi-device / subject / busy-idle / reject / offline-fallback 各 1 PNG | yema sign | `docs/qa/screenshots/rt-3-{multi-device,subject,busy-idle,reject,offline-fallback}.png` × 5 ✅ (Playwright e2e 真跑生成, 入 git 反 PS 修改; yema G4.x signoff 待补) |
| 4.4 thinking 5-pattern 锁链 RT-3 = 第 11 处源头落地 — 反向 grep 5 字面 (`subject\s*=\s*""` / `defaultSubject` / `fallbackSubject` / `"thinking"` / `"AI is thinking"`) 在 internal/ws + internal/bpp + client/src/ 0 hit | CI grep | `TestRT3_ReverseGrep_NoSubjectFallback` + git grep CI 守门 0 hit ✅ (v0 已 🟢) |

## REG-RT3-* (v0 server hook 已 🟢 / v1 client+e2e 待翻)

- REG-RT3-001 🟢 (v0) multi-device fanout 单源 hub.onlineUsers + cursor 第 6 frame 共序 + 反 device-id 路由
- REG-RT3-002 🟢 (v0) thinking subject 必带非空 fail-closed (BPP-2.2 ValidateTaskStarted SSOT 同源)
- REG-RT3-003 🟢 (v0) thinking 5-pattern 锁链第 11 处源头 + 反向 grep 5 字面 0 hit
- REG-RT3-004 🟢 (v0) task_started→busy / completed→idle / failed+reason 透传 / 中间态 reject / dict 污染 reject
- REG-RT3-005 🟢 (v0) BPP-3 #489 PluginFrameDispatcher boundary 集成 + HubAgentTaskPusherAdapter 跨包胶水
- REG-RT3-006 🟢 (v0) admin god-mode 不下发 + 0 schema/migration 改

**v1 新增** (待 PR 翻):
- REG-RT3-007 ⚪ RT-3.3 client UI + AgentActivityDot 三态 DOM byte-identical + 反 typing-indicator 同义词漂
- REG-RT3-008 ⚪ RT-3.5 Playwright 多 tab e2e 5 case PASS + RT-3.4 DL-4 offline fallback + ⭐ 5 截屏 demo (yema G4.x signoff)

## 退出条件

- §1 (2) + §2 (4) + §3 (3) + §4 (4) 全绿 — 一票否决
- 多端 cursor 真同步 (Playwright 多 tab e2e PASS)
- presence 三态 DOM byte-identical (content-lock 锁)
- typing 类同义词反向 grep 0 hit
- thinking 5-pattern 锁链第 11 处源头 (5 字面 0 hit)
- post-#614 haystack gate 三轨过 (Func=50/Pkg=70/Total=85)
- 0 schema 改 + 0 新错码 + admin god-mode 不下发
- ⭐ 5 截屏 demo + yema G4.x signoff
- 登记 REG-RT3-007..008 (v0 001..006 已 🟢)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-30 | 烈马 | v0 — server hook acceptance (REG-RT3-001..006 全 🟢, PR #588 merged). |
| 2026-05-01 | 烈马 | v1 — 扩 4 段验收覆盖 RT-3.3 client + RT-3.5 e2e + RT-3.4 DL-4 fallback + 5 截屏 demo. REG-RT3-007..008 ⚪ 占号. 立场承袭 thinking 5-pattern 锁链第 11 处源头 + RT-1.1 cursor 共序 + BPP-2.2 ValidateTask* SSOT + AL-1a 6-dict reason + post-#614 haystack gate. |
