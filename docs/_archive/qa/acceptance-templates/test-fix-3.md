# Acceptance Template — TEST-FIX-3 (race-heavy build tag 隔离 + shared fixture ctx-aware + CI sub-job)

> 跟 TEST-FIX-1 #596 (`t.Parallel()` sub-test 加速) + TEST-FIX-2 #608 (server.New ctx + 3 处 goroutine leak) 互补; TEST-FIX-3 = race-heavy build tag 隔离 + 共用 fixture 单源 + CI sub-job 真验三轨. Spec brief `test-fix-3-spec.md` v0 (飞马 ✅ APPROVED 0 必修). Owner: 战马C 实施 / 飞马 review / 烈马 验收.
>
> **TEST-FIX-3 范围**: test infra refactor — 0 production code 改, 不 skip / 不 mask / 不降 cov, 走"真因隔离不吞下"立场承袭 TEST-FIX-1/2 路径.

## 验收清单

### §1 数据契约 — race-heavy build tag 隔离 + shared fixture 单源 + CI sub-job 字面锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 race-heavy build tag 真挂 — TestClosedStoreInternalErrorBranches 11 sub-test 整段 byte-identical 迁到 `closed_store_race_test.go` 走 `//go:build race_heavy` (默认 `go test ./...` 不跑, 主 race job 不再贴近 120s ceiling) | unit + grep | 战马C / 烈马 | ✅ `grep -cE '^//go:build race_heavy' packages/server-go/internal/api/closed_store_race_test.go` ==1 + `grep -c "TestClosedStoreInternalErrorBranches" packages/server-go/internal/api/error_branches_test.go` ==0 + go test -tags=race_heavy 单测 6.6s PASS |
| 1.2 shared fixture 单源 ctx-aware — `testfixture_test.go::closedStoreFixtureContext(t)` (t.Context() + WithCancel + t.Cleanup 双保险, #608 leak 不复发) | unit + grep | 战马C / 烈马 | ✅ `grep -c 'closedStoreFixtureContext' packages/server-go/internal/api/testfixture_test.go` ≥1 + `grep -cE 't\.Cleanup\(cancel\)' packages/server-go/internal/api/testfixture_test.go` ≥1 |
| 1.3 CI 字面锁 — `ci.yml::go-test-race` 维持 `-timeout=120s` 不破 (反全局 bump mask) + 新 `go-test-race-heavy` sub-job 真加 `-tags 'sqlite_fts5 race_heavy' -timeout=180s` + `go-test-cov` 加 `race_heavy` tag (cov 路径不带 -race, 安全包含, 不破 84% 阈值) | CI yml | 战马C / 飞马 | ✅ `grep -cE '\-timeout=120s' .github/workflows/ci.yml` ≥1 (主 race 不破) + `grep -cE "tags 'sqlite_fts5 race_heavy'.*timeout=180s" .github/workflows/ci.yml` ≥1 (sub-job 真加) |

### §2 行为不变量 — race PASS + cov ≥84% 不降 + 主 race 不撞 ceiling

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 主 race job 不撞 120s ceiling — race-heavy 11 sub-test 隔离后, ./internal/api 主路径 race 49.8s ≤120s 余量充足 | full race | 战马C / 烈马 | ✅ 实测 `go test -tags=sqlite_fts5 -timeout=120s -race ./internal/api/` 49.8s PASS |
| 2.2 race-heavy sub-job PASS ≤180s — `closed_store_race_test.go` 11 sub-test 走独立 sub-job timeout 180s | race+tag | 战马C / 飞马 | ✅ 实测 `go test -tags='sqlite_fts5 race_heavy' -timeout=180s -race -run TestClosedStoreInternalErrorBranches ./internal/api/` 6.6s PASS |
| 2.3 cov ≥84.0% 不降 — go-test-cov 加 race_heavy tag 包含 11 sub-test 覆盖 (永不降测试覆盖度铁律) | go test -cover | 战马C / 烈马 | ✅ 实测 `go test -tags='sqlite_fts5 race_heavy' -timeout=180s -coverprofile=coverage.out ./...` cov 84.0% ≥84% |
| 2.4 既有 server-go non-race ./... 全绿不破 (Wrapper 立场 — TEST-FIX-3 是 test infra refactor 0 production behavior 改) | full test | 战马C / 烈马 | ✅ `go test -tags='sqlite_fts5 race_heavy' ./...` PASS (cov run 已验) |

### §3 蓝图行为对照 — 不 skip / 不 mask + ctx-aware 立场承袭 + Go 1.25 t.Context()

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场承袭 TEST-FIX-1/2 — 不 skip 任何 test (11 sub-test byte-identical 迁, 不缩) + 不加 retry/sleep mask + 不全局 bump race timeout (隔离不吞下) | grep | 飞马 / 烈马 | ✅ closed_store_race_test.go 11 sub-test 计数 ≥11 + 主 race timeout 维持 120s 字面不破 |
| 3.2 ctx-aware shutdown 立场承袭 (TEST-FIX-2 server.New(ctx)) — fixture helper `closedStoreFixtureContext` 走 `t.Context() + WithCancel + t.Cleanup(cancel)` 双保险 (Go 1.25 自动 cancel + 显式 Cleanup) | grep | 飞马 / 烈马 | ✅ testfixture_test.go 内 `t.Context()` + `context.WithCancel` + `t.Cleanup(cancel)` 三件齐 |
| 3.3 race target test byte-identical 迁 — 11 sub-test 字面 0 改, 仅文件名变 (closed_store_race_test.go) + build tag header 加 | inspect | 飞马 / 烈马 | ✅ `git diff origin/main...HEAD` 显示 closed_store_race_test.go 是新增 + error_branches_test.go 是删除该函数 (byte-identical 迁) |

### §4 反向断言 — 0 production code 改 + drift 守门 + 跨 milestone 锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 0 production code 改 (仅 *_test.go + ci.yml + docs) — `git diff origin/main...HEAD --stat` 0 production .go 文件改 | grep | 飞马 / 烈马 | ✅ diff stat 仅 _test.go + ci.yml + docs/qa/regression-registry.md + docs/implementation/progress/phase-4.md + docs/implementation/modules/test-fix-3-spec.md + docs/qa/acceptance-templates/test-fix-3.md |
| 4.2 反向 grep 主 race job timeout 不破 — `grep -cE '\-timeout=120s' .github/workflows/ci.yml` ≥1 (反全局 bump 180s mask) | CI grep | 飞马 / 烈马 | ✅ ci.yml::go-test-race 仍 -timeout=120s |
| 4.3 反向 grep 跨 PR 解锁验证 — #584 CHN-14 + #597 DM-10 race 共性债 post-merge rebase 后 race 自然过 | post-merge | 战马C / team-lead | post-merge verify 待 PR rebase 后 |

## REG-TESTFIX3-* 占号 (initial ⚪ → 实施后 🟢)

- REG-TESTFIX3-001 🟢 race-heavy build tag 真挂 (closed_store_race_test.go::`//go:build race_heavy` 1 处)
- REG-TESTFIX3-002 🟢 主 race timeout 维持 120s (反全局 bump) + race_heavy sub-job 180s 真加 + cov 加 race_heavy tag (不降 84%)
- REG-TESTFIX3-003 🟢 fixture 单源 ctx-aware (closedStoreFixtureContext 双保险) + cov ≥84.0%
- REG-TESTFIX3-004 ⚪ 跨 PR 解锁: #584 + #597 race post-merge 自然过 (待 rebase verify)
- REG-TESTFIX3-005 🟢 0 production code 改 (仅 *_test.go + ci.yml + docs)

## 边界

- TEST-FIX-1 #596 t.Parallel() sub-test 加速 (跟 TEST-FIX-3 互补 — 不重复 sub-test 加速, TEST-FIX-3 是 build tag 隔离 + fixture helper)
- TEST-FIX-2 #608 server.New(ctx) + 3 处 goroutine leak 修 (调用方 ctx-aware shutdown 立场承袭, fixture helper 复用此 ctor)
- AL-7 RetentionSweeper #533 + HB-5 HeartbeatRetentionSweeper #607 既有 ctx-aware shutdown (TEST-FIX-3 不动 sweeper 内部)
- Go 1.25+ `t.Context()` 自动 cancel pattern (TEST-FIX-2 引入, TEST-FIX-3 在 fixture helper 显式 WithCancel + Cleanup 双保险)
- 永不降测试覆盖度铁律 (CLAUDE.md `no_lower_test_coverage.md`) — cov 84.0% ≥84%
- 跑 test 必须加 timeout 铁律 (主 race 120s / race_heavy 180s / cov 180s)
- 不允许 admin merge bypass / flaky retry mask (CLAUDE.md `no_admin_merge_bypass.md`) — 真因隔离不 mask

## 退出条件

- §1 (3) + §2 (4) + §3 (3) + §4 (3) 全绿 — 一票否决
- race-heavy build tag 隔离 (closed_store_race_test.go 1 文件 + ci.yml sub-job 1 个)
- shared fixture 单源 ctx-aware (testfixture_test.go::closedStoreFixtureContext 1 helper)
- CI 字面锁 (主 race -timeout=120s 不破 + race_heavy sub-job -timeout=180s + cov 加 race_heavy tag)
- 主 race ./internal/api PASS ≤120s (实测 49.8s 余量充足) + race_heavy sub-job PASS ≤180s (实测 6.6s)
- cov ≥84.0% 不降 (实测 84.0% 阈值 byte-identical 不动)
- 0 production code 改 (仅 *_test.go + ci.yml + docs)
- 立场承袭 TEST-FIX-1/2 (不 skip / 不 mask / 不全局 bump / 真因隔离)
- 登记 REG-TESTFIX3-001..005 (4 已 🟢, 1 待 post-merge verify)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-30 | 烈马 | v0 — acceptance template 草稿 (4 选 1 验收框架 + REG-TESTFIX3-001..006 6 行占号 ⚪). |
| 2026-04-30 | 战马C | v1 — 实施完毕 acceptance flip ⚪→🟢. 跟随 spec brief v0 (飞马 ✅ APPROVED) 实施: race-heavy build tag 隔离 (closed_store_race_test.go) + fixture helper ctx-aware (testfixture_test.go::closedStoreFixtureContext) + CI sub-job (go-test-race-heavy) + cov 加 race_heavy tag. 实测 (本地): 主 race 49.8s ≤120s + race_heavy 6.6s ≤180s + cov 84.0% ≥84%. 0 production code 改, 仅 *_test.go + ci.yml + docs. REG-TESTFIX3-001/002/003/005 🟢, REG-TESTFIX3-004 ⚪ post-merge verify (#584 + #597 rebase). |
