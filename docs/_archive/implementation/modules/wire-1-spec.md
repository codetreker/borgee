# WIRE-1 spec brief — wire-up 死代码 3 处 production 接通 (≤80 行)

> 飞马 · 2026-05-01 · post-Phase 4+ closure follow-up wave (zhanma-c dev 视角抓真值, 飞马 audit 漏抓反思)
> **关联**: DL-2 #615 cold consumer + DL-3 #618 archive offloader + DL-4 #485 AgentTaskNotifier — 3 处 spec 字面合格但 production 0 callsite 死代码
> **命名**: WIRE-1 = 第一件 wire-up follow-up milestone (post-closure 死代码兑现, 跟 BPP-3 plugin frame dispatcher wire-up 同精神)

> ⚠️ Server wire-up milestone — **0 schema 改 / 0 endpoint URL 改 / ~30 行 production wire**.
> 真接通 spec 字面立场 (DL-2/DL-3/DL-4 立场字面合格但路径死代码).

## 0. 关键约束 (3 条立场)

1. **3 处 wire-up 真接通 + 跨 milestone 立场字面 byte-identical 不破** (post-closure 死代码兑现): 
   - **wire-1**: `factory.go:35` `NewInProcessEventBus()` → `NewInProcessEventBusWithStore(store)` (DL-2 #615 cold consumer 真接, channel_events / global_events 表真 INSERT; mustPersistKinds 4 类真落)
   - **wire-2**: `server.go:460` 加 `NewEventsArchiveOffloader(store.DB(), dl.EventBus, logger).Start(ctx)` 跟 ThresholdMonitor 同精神 ctx-aware (DL-3 #618 cold archive 真触发)
   - **wire-3**: `bpp/task_lifecycle.go::handleTaskFinished` 加 `agentTaskNotifier.NotifyAgentTask(...)` 调 (DL-4 #485 deferred BPP-2.2 task lifecycle frame → push 真接, RT-3.2 派生 hook 真落)
   反约束: 反向 grep `NewInProcessEventBus\(\)|NewInProcessEventBusWithStore` 在 production 路径 (factory.go) 字面映射: hot-only 0 hit + with-store ==1 hit; `EventsArchiveOffloader.*Start` 在 server.go ==1 hit; `AgentTaskNotifier.*Notify` 在 task_lifecycle.go ≥1 hit.

2. **0 schema / 0 endpoint URL / 0 routes.go / 0 user-facing API 改** (wire 立场, 跟 INFRA-3/4 / REFACTOR-1/2 / NAMING-1 / RT-3 / DL-2/3 / HB-2 v0(D) wrapper 系列承袭): PR diff 仅 (a) factory.go 1 行改 (b) server.go 6 行加 (c) task_lifecycle.go ~10 行 wire (d) ~3 unit test 真测 production callsite 不死. 反约束: 0 schema column / 0 migration v 号 / 0 endpoint URL.

3. **haystack gate 三轨 + 既有 test 全 PASS + ctx-aware 不 leak** (跟 DL-2/DL-3 sweeper / TEST-FIX-2 #608 立场承袭): wire 走既有 ctx 真传 (s.ctx) 反 goroutine leak; mention_notifier nil-safe 路径不破 (production 真值 Gateway 注入 vs nil-test path).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **W1.1 DL-2 cold consumer wire** | `internal/datalayer/factory.go:35` 改 (NewInProcessEventBus → NewInProcessEventBusWithStore + store 注入); `factory_wire_test.go` 新 ~20 行 (production callsite 真测 1 Publish → channel_events INSERT 真验) | 战马 / 飞马 review |
| **W1.2 DL-3 offloader wire + W1.3 AgentTaskNotifier wire** | `internal/server/server.go` +6 行 EventsArchiveOffloader.Start(ctx) (跟 ThresholdMonitor 同精神); `internal/bpp/task_lifecycle.go::handleTaskFinished` ~10 行 wire AgentTaskNotifier (Gateway 注入 + idempotent NotifyAgentTask + nil-safe 兜底); `wire_test.go` 真测 task_finished → push.NotifyAgentTask 调 ≥1 (mockGateway counter ≥1) | 战马 / 飞马 review |
| **W1.3 closure** | REG-WIRE1-001..006 (6 反向 grep + 3 wire 真接通 + 0 schema/endpoint 改 + ctx-aware 真守 + haystack 三轨过 + 既有 test 全 PASS) + acceptance + content-lock 不需 (server-only wire) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) DL-2 cold consumer 真接 (反 hot-only)
grep -nE 'EventBus:.*NewInProcessEventBusWithStore' packages/server-go/internal/datalayer/factory.go  # ==1 hit
grep -nE 'EventBus:.*NewInProcessEventBus\(\)' packages/server-go/internal/datalayer/factory.go  # 0 hit (反 hot-only stale)

# 2) DL-3 offloader 真启 (server.go production wire)
grep -nE 'EventsArchiveOffloader.*Start\(' packages/server-go/internal/server/server.go  # ==1 hit
grep -nE 'NewEventsArchiveOffloader' packages/server-go/internal/server/server.go  # ==1 hit

# 3) AgentTaskNotifier 真接 (task_lifecycle production wire)
grep -nE 'AgentTaskNotifier|NotifyAgentTask' packages/server-go/internal/bpp/task_lifecycle.go  # ≥1 hit

# 4) 0 endpoint URL / 0 routes.go / 0 migration
git diff origin/main -- packages/server-go/internal/server/server.go | grep -cE '^\+.*HandleFunc|^\+.*Handle\('  # 0 hit
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+\s*Version:'  # 0 hit

# 5) ctx-aware 真守 (反 leak, 跟 DL-2/DL-3/TEST-FIX-2 立场承袭)
grep -nE 'Start\(ctx\)|Start\(s\.ctx\)' packages/server-go/internal/server/server.go  # ≥3 hit (RetentionSweeper + ThresholdMonitor + EventsArchiveOffloader)

# 6) haystack gate 三轨 + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./...  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **新功能 / 新 endpoint / 新 schema** — 0 行为改铁律
- ❌ **events 接 RT-3 fanout 上游 hook** — 真接 wire-2 留下半 (DL-2 cold → RT-3 hub.PushFrame 桥接), v1.x follow-up
- ❌ **HB-2 v0(D) Borgee Helper SQLite consumer 阈值哨 wire** — 留 HB-2 v1 升级 (跟 P1 半漏项 #2 同精神)
- ❌ **ADM-3 v1 host_bridge placeholder 真接** — 留 ADM-3.bis HB source 接入 PR (P1 半漏项)

## 4. 跨 milestone byte-identical 锁

- DL-2 #615 EventStore + mustPersistKinds + retention sweeper 字面 byte-identical 不破
- DL-3 #618 ThresholdMonitor / EventsArchiveOffloader 字面 byte-identical 不破
- DL-4 #485 AgentTaskNotifier nil-safe 模式 byte-identical 承袭
- TEST-FIX-2 #608 ctx-aware shutdown 立场承袭 (反 goroutine leak)
- 跨 audit forward-only 链 ≥18 处 (events.archive_offload kind 走 DL-2 mustPersistKinds 必落 prefix)

## 5. 派活 + 6. 飞马自审

派 **zhanma-c** (DL-2/DL-3 主战续作熟手). 飞马 review.

✅ **APPROVED with 1 必修条件**:
🟡 必修: production callsite 真测 (反"spec 字面合格但 0 callsite 死代码" 教训承袭) — wire_test.go 必 mockGateway counter ≥1 / channel_events INSERT count ≥1 真值 verify.

担忧 (1 项): factory.go 改 1 行影响整 server boot 路径 — 战马实施跑 full server 启动 e2e 1 次 verify wire 真接通不 panic.
