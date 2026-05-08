# Acceptance Template — NAMING-1 (milestone-prefix 全清 + modules/ → architecture/)

> Spec brief `naming-1-spec.md` (飞马 v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收.
>
> **NAMING-1 范围**: codebase milestone-prefix 全清 (`dm_10_pin.go` / `chn_5_archived.go` / `cv_15_*` 等 N+ 文件改 feature-name) + 测试/TSX 函数名归一 + `docs/implementation/modules/` 迁 `docs/implementation/architecture/`. 立场承袭 REFACTOR-1 #611 + REFACTOR-2 #613 (helper SSOT) + PR #612 锁审查标准 (反 milestone-prefix spam). **0 endpoint 行为改 + 0 migration v 号改 + 0 schema**.

## 验收清单

### §1 文件名 rename 验收

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 milestone-prefix 文件名 0 残留 — `find packages -name '*[a-z]_[0-9]*' -name '*.go'` 在 production 路径 0 hit (除 migrations/ 历史不动) | find | 0 hit ✅ |
| 1.2 client TSX milestone-prefix 0 残留 — `find packages/client -name '*-cs-*' -o -name '*-dm-*' -o -name '*-cv-*' -o -name '*-chn-*'` 0 hit (除已合规 `<feature>.tsx` 模式) | find | 0 hit ✅ |
| 1.3 git mv history 保留 — `git log --follow --oneline -- <new-name>` ≥1 commit 跟旧名连续 (反 git rm + add 丢史) | git log | 抽样 5 文件 verify follow-history 连续 ✅ |

### §2 结构体 / 变量名 audit 验收

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 反向 grep `\\bDM10\\|\\bCHN5\\|\\bCV15\\|\\bAL1A\\|\\bHB5\\|\\bRT4\\|\\bAP4\\|\\bBPP3\\|\\bCS1\\|\\bCM5` 在 production .go body (除 _test.go + migrations/ + REG-*) 0 hit (struct/var/const/func 全清) | CI grep | reverse grep test PASS |
| 2.2 milestone-prefix 函数名 0 残留 — `grep -rE '^func [A-Z][a-z]*[0-9]+_' packages/server-go/internal/ --include='*.go' \\| grep -v _test.go \\| grep -v migrations/` 0 hit | grep | 0 hit ✅ |
| 2.3 godoc 注释 milestone-prefix 残留 ≤20 (历史 narrative 锚不动, 反约束: 不允许新注释引入 milestone-prefix) | grep | 抽样 verify 历史 narrative anchor 不删 |

### §3 测试命名 + TSX 命名归一验收

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 milestone-prefix Test 函数名 0 残留 — `grep -rE '^func Test(DM[0-9]\\|CHN[0-9]\\|CV[0-9]\\|AL[0-9]\\|HB[0-9]\\|RT[0-9]\\|AP[0-9]\\|BPP[0-9]\\|CS[0-9]\\|CM[0-9]\\|TESTFIX[0-9]\\|REFACTOR[0-9])_' packages/server-go/internal/` 0 hit (反 PR #612 抓到的 `TestCHN5_CovBump_*` / `TestHB5_CovBump_*` / `TestRT4_CovBump_*` byte-identical body 复制 spam 立场承袭) | grep | 0 hit ✅ |
| 3.2 vitest TSX milestone-prefix 0 残留 — `grep -rE "(describe\\|test\\|it)\\(['\\\"]+(CS-[0-9]\\|DM-[0-9]\\|CV-[0-9]\\|CHN-[0-9])" packages/client/src/__tests__/` 0 hit (改 feature-name describe block) | grep | 0 hit ✅ |
| 3.3 covbump test 文件 0 残留 — `find packages -name '*covbump*' -o -name '*cov_test.go'` 0 hit (PR #612 留账 hb_5/rt_4 covbump spam 跟随删 / chn_14_description_history_cov 改 feature 名) | find | 0 hit ✅ |

### §4 modules/ → architecture/ 迁验收

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 `docs/implementation/modules/` 目录 0 文件 (全迁 `docs/implementation/architecture/`) | ls | `ls docs/implementation/modules/ 2>&1 \\| wc -l` ==0 ✅ |
| 4.2 modules/*-spec.md byte-identical 迁到 architecture/*-spec.md (字面 0 改, 仅路径变) | git mv | git log --follow verify follow-history 连续 ≥10 抽样文件 ✅ |
| 4.3 内部锚链跟随更新 — 反向 grep `docs/implementation/modules/` 在 docs/ + packages/ + .github/ 全清 0 hit (除 git history / regression-registry 历史 narrative anchor 合规白名单) | CI grep | 反向 grep test PASS |

### §5 caller 跟随 + 全包 unit/e2e PASS + cov gate

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 5.1 既有全包 unit + e2e + vitest 全绿不破 (Wrapper 立场, rename 不动 endpoint 行为) | full test | `go test -tags sqlite_fts5 -timeout=300s ./...` 24+ packages 全 PASS + `pnpm exec vitest run --testTimeout=10000` 全 PASS + `pnpm exec playwright test --timeout=30000` 全 PASS |
| 5.2 0 endpoint 行为改 + 0 migration v 号改 + 0 schema (`git diff main -- internal/migrations/` 0 行) | git diff | 0 行 ✅ |
| 5.3 post-#612 haystack gate 三轨过 — Func=50 / Pkg=70 / Total=85 (TEST-FIX-3-COV #612 立场承袭) | CI verify | go-test-cov SUCCESS + TOTAL ≥85% no func<50% no pkg<70% |
| 5.4 post-#613 REFACTOR-2 锁链承袭 — 4 helper SSOT (mustUser / decodeJSON / loadAgentByPath / fanoutChannelStateMessage) caller 路径 rename 后 byte-identical 不破 | grep + unit | helper callsites 数 byte-identical 跟 #613 baseline + 单源 `^func` 各 ==1 hit |

## REG-NAMING1-* 占号 (initial ⚪)

- REG-NAMING1-001 🟢 文件名 milestone-prefix 0 残留 (find 0 hit, server-go + client TSX) + git mv follow-history 保留
- REG-NAMING1-002 🟢 结构体/变量/函数名 milestone-prefix 0 残留 (反向 grep DM10/CHN5/CV15/AL1A 等 production body 0 hit) + godoc 历史 narrative 锚不动
- REG-NAMING1-003 🟢 测试函数名 milestone-prefix 0 残留 (反 PR #612 covbump spam 立场承袭) + vitest TSX describe block 0 milestone-prefix + covbump test 文件全删
- REG-NAMING1-004 🟢 docs/implementation/modules/ 迁 architecture/ 全闭 + 内部锚链全清 (0 hit `docs/implementation/modules/` 在 production code/docs 除合规白名单)
- REG-NAMING1-005 🟢 既有全包 unit + e2e + vitest 全绿不破 + 0 endpoint 行为改 + 0 migration / 0 schema + post-#612 haystack gate 三轨过 (Func=50/Pkg=70/Total=85)
- REG-NAMING1-006 🟢 post-#613 REFACTOR-2 锁链承袭 (4 helper SSOT caller 路径 byte-identical) + 跨十 milestone const SSOT 锁链 (BPP-2 + REFACTOR-REASONS + DM-9 + CHN-15 + AP-4-enum + DL-1 + REFACTOR-1 + REFACTOR-2 + NAMING-1)

## 退出条件

- §1 (3) + §2 (3) + §3 (3) + §4 (3) + §5 (4) 全绿 — 一票否决
- find milestone-prefix 文件 0 hit (server-go + client TSX) + git mv history 保留
- grep `^func Test(milestone-prefix)_` body 0 hit
- modules/ 0 文件 + architecture/ 全在 + 锚链全清
- 全包 unit + e2e + vitest 全绿不破 + post-#612 haystack gate 全过 (Func=50/Pkg=70/Total=85)
- 0 endpoint 行为改 + 0 migration v 号改 + 0 schema
- 登记 REG-NAMING1-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template 草稿 (5 选 1 验收框架 + REG-NAMING1-001..006 6 行占号 ⚪). 立场承袭 REFACTOR-1 #611 + REFACTOR-2 #613 (helper SSOT) + PR #612 锁审查标准 (反 milestone-prefix spam covbump body 复制). 关键: 0 endpoint 行为改 + 0 migration / 0 schema + git mv follow-history 保留 + modules/→architecture/ 迁 byte-identical + post-#612/#613 haystack gate 三轨守门承袭. |

| 2026-05-01 | 战马C | flip — REG-NAMING1-001..00N 实施验收 PASS. 实测 Phase N1.1 (A 文件 169 git mv + B struct/handler 18 + migration var 32 + C test func 924 全 unique (856 strip + 90 file-prefix functionality suffix 接合规 commit c2d192c4)) + N1.2 (E modules→architecture 11 + 跨 doc ref 11 处更) + 5 hardcoded 路径修. 同 file dup 自动 _N suffix (TestADM_ExecError_BadPrior_Sessions / TestCM_ExecError_BadPrior_2). 24 包 test 全 PASS, haystack gate TOTAL 85.5% no func<50% no pkg<70%. 文件 milestone-prefix 残留 0 hit ✅. 测试函数 milestone-prefix 残留 0 hit ✅ (90 collision-keep 真做完 commit c2d192c4 — 飞马 audit 反转 spec 错估 collision 不可避免, file-prefix functionality suffix unique 化). D 类 TSX 测试命名 dot-variant 0 hit (检测无需 rename). |
