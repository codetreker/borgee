# TEST-FIX-3 spec brief — internal/api race time 系统性解 (≤80 行)

> 飞马 · 2026-04-30 · 用户解冻拍板 (全局优化) · zhanma-c 主战
> **关联**: TEST-FIX-2 #608 ✅ merged (server.New(ctx) + middleware.go evictStale 提取); TestClosedStoreInternalErrorBranches 11 sub-test race-heavy
> **命名**: TEST-FIX-3 = 第三件 test infra milestone, 跟 TEST-FIX-1 #596 / TEST-FIX-2 #608 同等级

> ⚠️ Test-infra milestone — 0 schema / 0 endpoint / 0 production 行为改; 仅 test 拆 + fixture 共享 + 可选 CI timeout bump.

## 0. 关键约束 (3 条立场)

1. **race-heavy sub-test 隔离 (build tag 路径优先, 反包拆漂)**:
   - 立场: `TestClosedStoreInternalErrorBranches` 11 sub-test 走 **`//go:build race_heavy`** build tag, 走独立 `_race_test.go` 文件 (跟主 `_test.go` 同包同 dir 不另起 package), 默认 `go test ./...` 不跑, CI 矩阵加 `-tags=race_heavy` job 单独跑 (timeout 180s 单独, 跟主 race job 不互拖)
   - 反路径: **不**拆到独立 package (`internal/api/racetests/`) — 包拆漂导致 internal symbol export, 违封装; 反 `t.Skip` if testing.Short — Short 跟 race 正交, 不解决真问题
   - 反约束: 反向 grep `//go:build race_heavy` count==1 (新加) + race_heavy tag 文件名 `*_race_test.go` 命名约束

2. **共享 fixture 单源 (跟 BPP-3 PluginFrameDispatcher SSOT 同精神)**:
   - `internal/api/testfixture_test.go` 新 (~80 行) — `newTestServerWithClosedStore(t)` / `newTestServerWithFaultStore(t, mode)` 单 helper, 11 sub-test 全走单源 (反每 sub-test 内 inline server.New + s.Close 重复 ~30 行 boilerplate)
   - 复用 TEST-FIX-2 #608 既有 `server.New(ctx)` ctor (ctx-aware shutdown 真闭) — fixture 内 `ctx, cancel := context.WithCancel(t.Context())` + `t.Cleanup(cancel)` 确保 leak 0 (Go 1.25 t.Context() 自动 cancel 兜底)
   - 反约束: 反向 grep `s := server.New` in `internal/api/*_test.go` 单源化后 ≤ baseline (战马C 实施时 grep before/after 报数)

3. **CI race timeout bump 180s 仅 race_heavy job, 主 race job 不动**:
   - 立场: 主 `go test -race ./...` job 维持现 90s timeout (反 mask flaky); 加 sub-job `go test -race -tags=race_heavy ./internal/api/...` timeout 180s (race-heavy sub-test 真需更长, 不污染主路径阈值)
   - 反路径: **不**全局 bump 180s — 全局 bump 是 mask, 真因 (race-heavy sub-test serialize 长) 应隔离不应吞下
   - 反约束: `.github/workflows/ci.yml` race step 加 sub-step 不改既有 timeout (grep `-timeout=90s` 既有 ≥1 hit 不破)

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **TF3.1** build tag + race-heavy 拆文件 | `internal/api/closed_store_race_test.go` 新 (从既有 `*_test.go` 迁 11 sub-test, `//go:build race_heavy` 头) + 既有 `_test.go` 删 race-heavy 11 sub-test (byte-identical 迁不收缩); CI ci.yml 加 sub-job race_heavy timeout 180s | 战马C / 飞马 review |
| **TF3.2** 共享 fixture 单源 + 11 sub-test 走 helper | `internal/api/testfixture_test.go` 新 ~80 行 (`newTestServerWithClosedStore` + `newTestServerWithFaultStore` 2 helper, ctx-aware + t.Cleanup); 11 sub-test 内 inline boilerplate 删走 helper (净减 ≥150 行) | 战马C / 飞马 review |
| **TF3.3** closure | REG-TF3-001..005 (5 反向 grep + race_heavy tag 真挂 + fixture 单源 ≥1 hit + 主 race timeout 不破 + leak 0 + 净减行数 verify) + acceptance + content-lock 不需 (test-only) + 4 件套 spec 第一件 | 战马C / 烈马 |

## 2. 反向 grep 锚 (5 反约束, count==0)

```bash
# 1) race_heavy build tag 真挂 (1 处, 反多处漂)
grep -cE '^//go:build race_heavy' packages/server-go/internal/api/  # ==1

# 2) 主 race job timeout 90s 不破 (反全局 bump)
grep -cE '\-timeout=90s' .github/workflows/ci.yml  # ≥1 (既有 baseline 不减)

# 3) race_heavy sub-job timeout 180s 真加 (限定 race_heavy tag)
grep -cE 'tags=race_heavy.*timeout=180s|timeout=180s.*tags=race_heavy' .github/workflows/ci.yml  # ≥1

# 4) fixture helper 单源 (newTestServerWithClosedStore 全测试走单源)
inline=$(git grep -cE 'server\.New\(ctx,' packages/server-go/internal/api/*_test.go | grep -v testfixture | awk -F: '{s+=$NF}END{print s}')
[ "$inline" -le 5 ]  # baseline ≤5 (TEST-FIX-2 既有最多 5 处, 11 sub-test 单源化后不增)

# 5) ctx leak 0 (t.Cleanup 真挂)
grep -cE 't\.Cleanup\(cancel\)' packages/server-go/internal/api/testfixture_test.go  # ≥1
```

## 3. 不在范围 (留账)

- ❌ **全局 bump race timeout 180s** (立场 ③ 反 mask) — race-heavy 隔离够用
- ❌ **拆独立 package `internal/api/racetests/`** (违封装, 立场 ① 反路径)
- ❌ **重写 11 sub-test 行为** — byte-identical 迁移 (跟 INFRA-3 子文件迁同精神)
- ❌ sqlmock / DATA-DOG dep 引入 (#597 (e') PRAGMA+DROP 路径已立 idiom, 不为 TF3 引入)
- ❌ test parallelism 全局调 (`t.Parallel` 已在 TEST-FIX-2 局部加, 不延伸)

## 4. 跨 milestone byte-identical 锁

- 复用 TEST-FIX-2 #608 既有 `server.New(ctx)` ctor (ctx-aware) byte-identical 不破
- 复用 TEST-FIX-2 既有 `evictStale(now)` helper 模式 (helper 提取 SSOT)
- 复用 #597 (e') PRAGMA+DROP idiom (state-based fault injection, fixture helper 内承载)
- 复用 release-gate.yml CI 守门链 (BPP-4/HB-3/AP-4-enum/HB-4/INFRA-3/INFRA-4/DL-1 同模式) — TF3 加 race_heavy job **不算第 7 处守门链** (是 CI 矩阵扩, 不是 grep 守门 step)
- 0-test-infra milestone 决策树**变体**: 0 schema / 0 endpoint / 0 production 行为改 (跟 TEST-FIX-1 #596 / TEST-FIX-2 #608 同源)

## 5. 派活 + 双签

派 **zhanma-c** (TEST-FIX-2 #608 主战熟手, 续作减学习成本) + 飞马 review.

双签流程: spec brief → team-lead → 飞马自审 ✅ APPROVED 0 必修条件 → zhanma-c 起 worktree `.worktrees/test-fix-3` 实施 (TF3.1+2+3 三段一次合).

## 6. 飞马 (架构师) 自审表态

✅ **APPROVED with 0 必修条件** — TEST-FIX-2 续作 byte-identical 模式承袭, 0 风险.

担忧 (1 项, 轻度):
- 🟡 build tag race_heavy 是新 idiom (项目内首处), 可读性需在 `_race_test.go` 文件头加 ~5 行注释 "为何走 build tag (race-heavy serialize 长隔离, 不污染主 race job timeout 阈值)"

留账接受度全 ✅: 全局 bump / 包拆 / 重写 sub-test / sqlmock / 全局 parallelism 全留账.
