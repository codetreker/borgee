# TEST-FIX-3 stance checklist — race-heavy 测试拆分 (test infra refactor only)

> 4 立场 byte-identical 跟 TEST 类 milestone 红线 (不降覆盖度 / 不 mask race / 不动 production code / 跟 INFRA-3 拆分同精神). content-lock 不需 (test infra refactor, 跟 TEST-FIX-1 #596 + TEST-FIX-2 #608 同精神). 跟 TEST-FIX-1 (`t.Parallel()` 加速) + TEST-FIX-2 (ctx-aware shutdown 解 leak) 互补不替代, **TEST-FIX-3 = race-heavy 文件按主题拆分 (test 文件级别基础设施)**.

## 1. 不降覆盖度铁律 (永不 skip / 永不降阈值)

- [ ] **永不 skip 任何 test** — 反 `t.Skip` / `t.SkipNow` 在本 PR diff 0 hit (拆分仅迁移 test func 跨文件, 不删不 skip)
- [ ] **永不降覆盖度阈值** — `cov ≥84%` 不动 (跟 user memory `no_lower_test_coverage` 铁律承袭)
- [ ] 反 `// TODO re-enable` / `// flaky skip` / `t.Skipf("flaky")` 反向 grep 0 hit
- [ ] go-test-cov CI step 既有阈值 byte-identical 不动
- [ ] 拆分前后 `go test -coverprofile` 对比 — 行覆盖率 byte-identical (拆文件不改测试逻辑)

## 2. 不 mask race fail (真因优化 ≠ rerun-and-pray)

- [ ] **真因拆主题分文件** — 按 race-heavy 测试主题 (例: subscriber lifecycle / pubsub fanout / connection pool / closed-store branches) 拆到独立 _test.go (反"加 retry / 加 sleep / 提 timeout" mask 路径)
- [ ] **反向 grep mask 模式 0 hit**:
  - 反 `time.Sleep(.*ms)` 在 _test.go 新增 0 hit (反 sleep mask race)
  - 反 `for i := 0; i < retries; i++` 在 _test.go 新增 0 hit (反 retry mask)
  - 反 `t.Skip("flaky")` / `t.Skip("race")` 0 hit
  - 反 timeout 提升 (180s → 300s 等) 反 mask 真因
- [ ] **拆分逻辑保 byte-identical** — test func 跨文件迁移, 单测断言/setup/cleanup 字面 byte-identical
- [ ] 拆分后 race CI 单 _test.go ≤ 既有 budget (跟 TEST-FIX-1 race budget step 同精神承袭)
- [ ] 跟 TEST-FIX-2 #608 ctx-aware shutdown root-cause 互补 — 本 PR 是 **基础设施层** (test 文件组织), TEST-FIX-2 已修真因 leak, TEST-FIX-3 拆分让 CI 信号更清晰 (反 monolithic _test.go 一处 fail 阻塞全主题)

## 3. 不动 production code (test infra 改 only)

- [ ] **0 production diff** — 反向断言: PR diff 仅含 `*_test.go` + `internal/testutil/` (跟 TEST-FIX-1 同精神)
- [ ] 反向 grep `packages/server-go/internal/api/*.go` (非 _test) 0 行改 (production handler 不动)
- [ ] 反向 grep `packages/server-go/internal/server/server.go` / `middleware.go` 0 行改 (TEST-FIX-2 已动, 本 PR 不重叠)
- [ ] 反向 grep `packages/server-go/cmd/` 0 行改 (production binary entry 不动)
- [ ] 0 schema / 0 migration / 0 endpoint / 0 client / 0 acceptance template 改 (跟 TEST-FIX-1 #596 + TEST-FIX-2 #608 同精神)
- [ ] 反 `interface{}` / 反 `var _ X = (*Y)(nil)` 等 production assertion 改 (拆分仅 test 文件, 反"顺手改 production")

## 4. 跟 INFRA-3 phase-* 拆分同精神 (无功能变更, 仅基础设施)

- [ ] **拆分协议承袭 INFRA-3 #594** — INFRA-3 PROGRESS.md 拆 5 子文件 (主 ≤100 行 + 单行 ≤200 字符), TEST-FIX-3 race-heavy _test.go 拆按主题 (单文件 ≤N test func / 单文件 ≤M 行 待 spec brief 拍板真值)
- [ ] **0 功能变更** — test 行为字面 byte-identical (跟 INFRA-3 翻牌机制不变 + 单源切路径同模式)
- [ ] **拆分锚** — race-heavy 主题 (建议: subscriber lifecycle / pubsub fanout / closed-store branches / DB connection pool 等) 各自单源不混 (跟 PR #574 naming-map module group 拆死同精神)
- [ ] **CI 守门链承袭** — go-test-race + go-test-cov 既有 step byte-identical 不动 (反新加 step, 跟 TEST-FIX-1 + TEST-FIX-2 同精神)
- [ ] **subagent rebase 不污染** — PR scope 严守 test infra refactor (跟 user memory `one_milestone_one_pr` 铁律 + `parallel_default_protocol` 协同)

## 反约束 — TEST-FIX-3 真不在范围

- ❌ 改 production code (留 TEST-FIX-2 类 root-cause fix milestone)
- ❌ 加 retry / sleep / timeout 提升 (反 mask, 反真因优化)
- ❌ skip 任何 test (反铁律)
- ❌ 降覆盖度阈值 (反铁律)
- ❌ 0 schema / 0 endpoint / 0 client / 0 acceptance template / 0 content-lock 改 (test infra refactor scope)
- ❌ 加新 CI step (跟 TEST-FIX-1 + TEST-FIX-2 同精神, 既有 step byte-identical 不动)
- ❌ admin god-mode 不挂 test infra (反向 grep `admin.*test|admin.*infra` 0 hit, ADM-0 §1.3 红线 + PR #571 §2 ⑥ 精神延伸)

## 反约束 — TEST-FIX-1/2/3 三 PR 互补不替代锁链

- TEST-FIX-1 #596 — `t.Parallel()` 加速 sub-test (减总耗时 12x)
- TEST-FIX-2 #608 — ctx-aware shutdown 解 leak (真因 root-cause fix, 7 PR 解锁)
- **TEST-FIX-3 (本 PR)** — race-heavy _test.go 按主题拆分 (test 文件组织基础设施, CI 信号清晰)
- 三 PR scope 各自单源不混 (反"一 PR 全包", 跟 user memory `one_milestone_one_pr` + INFRA-3 拆分协议同精神)

## 跨 milestone byte-identical 锁链 (5 链)

- **TEST-FIX-1 #596** — `t.Parallel()` 加速 sub-test 模式 (本 PR 拆分后单 _test.go race budget 自然降, 跟 #596 race budget step 协同)
- **TEST-FIX-2 #608** — ctx-aware shutdown root-cause fix (本 PR 拆分让 #608 真因优化效果在 CI 信号上更可见, 反 monolithic _test.go 单点 fail 遮罩)
- **INFRA-3 #594 拆分协议** — phase-* 子文件拆分模式承袭 (主文件瘦身 + 单源切路径 + 单行字符 budget; 本 PR test 文件版)
- **user memory `no_lower_test_coverage` 铁律** — cov 阈值是铁律, 本 PR 真守 byte-identical
- **user memory `no_admin_merge_bypass` 铁律** — flaky 真修不 bypass, 本 PR 拆分让 race 信号清晰反 mask, CI 必须真过承袭

## PM 立场拆死决策

**race-heavy 拆分 vs mask 5 模式拆死**:
- ✅ 按主题拆 _test.go (本 PR 选) — 单文件 race 信号清晰, 反 monolithic 一处 fail 遮罩主题
- ❌ 加 `time.Sleep(Nms)` race 让步 — 反 mask, 不解真因
- ❌ 加 retry loop — 反 mask, 隐藏真 race
- ❌ 提 timeout (180s → 300s) — 反 mask, 不优化
- ❌ skip flaky test — 反铁律
- ❌ 降 race budget 阈值 — 反铁律

**test infra refactor vs production refactor 拆死**:
- ✅ TEST-FIX-3 = test 文件级别拆分 (本 PR 选, 0 production diff)
- ❌ TEST-FIX-2 = production server.New ctx 入参 (root-cause fix, 已落 #608)
- 各自单源不混, 跟 user memory `one_milestone_one_pr` 铁律承袭

**0 功能变更 vs 拆分锚 byte-identical 拆死**:
- ✅ test func 跨文件迁移 (字面 byte-identical) + 主题分文件锚清晰 (本 PR 选)
- ❌ 反"拆分顺手改测试逻辑" — 反 INFRA-3 拆分协议精神 (单源切路径不改逻辑)

## 用户主权红线锚 (5 项)

- ✅ **永不降测试覆盖度铁律** (cov ≥84% 不动, 跟 user memory 协议)
- ✅ **永不 skip / 永不 mask race 真因** (真修拆分让信号清晰, 反 retry-pray)
- ✅ **0 user-facing change** (test infra 改, 无 user UI / 文案 / 翻译键 / DOM 改)
- ✅ **0 production code 改** (反向断言 PR diff 仅 _test.go + testutil/, 跟 TEST-FIX-1 同精神)
- ✅ **admin god-mode 不挂 test infra** (反向 grep 0 hit, ADM-0 §1.3 红线 + PR #571 §2 ⑥ 精神延伸)

## PR 出来 4 核对疑点 (PM 真测)

1. **0 production diff** — `git diff --name-only origin/main..HEAD | grep -vE '_test\.go$|/testutil/|/qa/|/implementation/'` 应 0 hit
2. **拆分前后 cov byte-identical** — `go test -coverprofile=cov.out ./internal/...` 拆分前后行覆盖率 byte-identical (反"拆分丢测试")
3. **race CI 单 _test.go ≤ budget** — go-test-race CI run 单 _test.go 时间 ≤ TEST-FIX-1 既有 budget (反"拆分后某文件超 budget")
4. **mask 5 模式反向 grep 0 hit** — `time.Sleep` / `for retries` / `t.Skip("flaky")` / `t.Skip("race")` / timeout 提升 在本 PR diff 0 hit
