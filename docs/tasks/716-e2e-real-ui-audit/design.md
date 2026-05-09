# 716 实施设计 — e2e 真 UI 审计 + 重写

> Issue: gh#716 (P0 / current-iteration)
> Author: zhanma-c (Dev, 主笔)
> Worktree: `/workspace/borgee/.worktrees/716-e2e-real-ui-audit/` (`feat/716-e2e-real-ui-audit`)
> 待 4 签: feima (架构) / yema (PM) / heima (Security) / liema (QA)

## §0 范围

全量审计 `packages/e2e/tests/**/*.spec.ts` (46 spec), 按 4 类反模式 (F1 fs lint / F2 page.evaluate fetch / F3 纯 REST / F4 源码 grep / F5 noop) 标 PASS / REWRITE / DELETE.

详情见 `audit.md` 同目录.

不在范围:
- `packages/e2e/playwright.config.ts` 不动 (双 server 编排合理)
- `packages/e2e/fixtures/` 不动 (helper 不是 spec, fs.* 在这里允)
- 不改 `client/` 或 `server/` 任何产品代码 (e2e 重写不允许带产品改动)

## §1 数据流

```
Dev 跑 audit → 标 46 spec PASS/REWRITE/DELETE → 一 PR 内并行做:

  ┌─ DELETE 3 → git rm
  ├─ PASS 24 → git mv 重命名 + 头部注释 refine 去黑话
  ├─ PASS+fix 3 → git mv + 局部去 page.evaluate(注 mock)
  └─ REWRITE 16 → 重写真 UI (admin login + page.click + DOM 断)
                  ↓
              push to feat/716-e2e-real-ui-audit
                  ↓
              CI e2e 真跑 → 全绿
                  ↓
              反向 grep 守卫: F1 / F2 / F5 = 0 hit
                  ↓
              teamlead 开 PR + 4 角色 review + 三签合
```

## §2 数据模型

无 schema 改动. 此 PR 仅改 e2e spec 文件本身.

## §3 API contract

无 API 改动. 改的是 e2e 怎么测 API + UI. server endpoint 不动.

## §4 边界 / 错误处理

| 场景 | 处理 |
|---|---|
| REWRITE spec 真 UI 跑了真 server, 但 client 没有对应 UI 路径 (e.g. cv-12 search 还没 client 入口) | 留 follow-up issue 标 client UI 实施缺失, **不**降级回 REST e2e (issue §3 反模式 F3). 此 PR 只做"现存 UI 真测", 不做"假 UI 凑数". |
| PASS+fix 改 cv-4-iterate 的 page.evaluate(mock) 后, server 真数据 seed 不出来 | 调 server-side fixture (admin POST artifact + iteration), 不行就标 follow-up + 暂保留 page.evaluate 加注释 |
| DELETE 后 CI 失败 (有人在别处 import 这 spec) | grep 引用面后再删. spec 文件互不 import 是 Playwright 默认, 但 fixtures/ 可能有引用 |
| REWRITE 多用 admin login 导致 testing 数据库膨胀 | 每 spec 用唯一 invite + 唯一 user (现有 `Date.now()` 后缀已防, 不动) |
| REWRITE 双 tab 测试 (cm-4 / dm-3 / rt-3 / cm-5) 时一边 disconnect | 用 `browser.newContext()` 起两个独立 context, 不共 cookie |
| 内联截图比对 chn-4-screenshots-followup §1-§3 仍要写 PNG 到 `docs/qa/screenshots/` | 这条路径之前 PR #715 删过 PNG, 跟此 PR 无关, 不动 |

## §5 多方案对比

### 方案 A: 一 PR 全做 (推荐)

PR 内 commit 序:
1. delete 3 死 spec
2. rename + refine 24 PASS spec (描述去黑话)
3. fix 3 PASS+fix spec
4. rewrite 16 spec
5. 反向 grep 守卫 + audit doc

**Pro**:
- issue §"P0 一次做干净不留尾" 跟用户 "一 milestone 一 PR" 铁律一致
- 跨 spec 重命名是 atomic, 不会出现"一半旧名一半新名"中间态

**Con**:
- PR diff 大 (~1500 行净改动估)
- review 重 (4 角色都要扫 46 spec)

### 方案 B: 拆三 PR (按动作)

PR1 = DELETE, PR2 = PASS rename, PR3 = REWRITE.

**Pro**:
- 每 PR diff 小好审

**Con**:
- 撞 "一 milestone 一 PR" 铁律 (memory `strict_one_milestone_one_pr`)
- PASS rename 跟 REWRITE 撞同 spec (e.g. cv-7 rename 后 REWRITE 又改) 必须串行, 不 atomic
- 跨 PR 周期长, P0 拖

### 方案 C: 只删假 + 重命名, REWRITE 拆 follow-up issue

**Pro**:
- 此 PR 极小

**Con**:
- 留 16 个 follow-up issue, 16 个 PR — 跟 issue body §"不允许留着以后改, P0 一次做干净" 直撞
- 假 e2e 只是改名没改实质

### 选择: **方案 A** (issue 命令 + 用户铁律)

issue 描述 §3 末段 + §"处理动作" 明文要求 "P0 一次做干净, 不允许 '留着以后改'". 跟用户铁律 `strict_one_milestone_one_pr` + `dispatch_grep_first_no_assumptions` 一致.

PR diff 大可以接受, 是 e2e spec 重写不是产品改动, review 重点在 "新 spec 跑得通 + 反模式 = 0 hit", 不是逐行 logic review.

## §6 集成 (反向 grep 锚)

| Grep | 命中 | 处理 |
|---|---|---|
| `fs\.(existsSync\|stat\|readFileSync\|readdirSync)` 在 `packages/e2e/tests/` | 9 hit (3 文件) | DELETE hb-1b-installer (7 hit), 删 chn-4-screenshots-followup §4§5 注释 (1 hit), hb-2-v0d 不是 fs (重检) |
| `expect\(true\)\.toBe\(true\)` 在 spec | 2 hit (cv-3-3-deferred / g2.4-adm-0-stance) | DELETE 两个 noop 占位 |
| `page\.evaluate\([^)]*=>[^)]*fetch` (cookie fetch) | 0 hit (好) | 已无 — issue 描述的反模式 F2 当前其实没人写, 守卫加上防回归 |
| `page\.evaluate` 总数 | 28 hit (8 文件) | 检查每 hit: localStorage / DOM 探 / WS 注入 / mock 注入. mock 注入改产品 fixture; 其它合理保留 |
| `apiRequest\.newContext` | 35 hit (大量 spec seed) | seed 不动; 测试主体走 REST 的 16 spec 重写为 UI |
| 文件名 `^[a-z]{2}-\d` (milestone-prefixed) | 38 文件 | 重命名为功能名 (见 audit.md "重命名建议" 列) |
| 文件名 `^gh-\d+` | 2 文件 (gh-684 / gh-698) | 重命名为功能名 (gh#684 → agent-detail-credentials-display, gh#698 → agent-config-form-layout) |
| 文件名 `^smoke|^cm-onboarding[^-]` | 留语义 | smoke.spec.ts 重命名 smoke-app-loads, cm-onboarding 重命名 chat-first-time-onboarding |

### 反向影响

- **e2e CI workflow** (`.github/workflows/deploy-test.yml` 等): grep `e2e/tests/` 路径 = 整个目录, 不引用具体文件名 → 重命名不破 CI ✅
- **playwright.config.ts** `testDir: './tests'` 整目录, 不引用具体文件 → 不破 ✅
- **package.json scripts**: grep 后无具体文件名引用 → 不破 ✅
- **fixtures/**: spec 内只 import `../fixtures/stopwatch` 这种相对路径, fixture 自己不 import spec → 不破 ✅
- **REG-* 锚** 在 `docs/qa/` 引用 spec 名: grep 找出后同 PR 改名 → 不破

## §7 测试策略 (元测试 — e2e spec 自己怎么验)

| 层 | 覆盖 |
|---|---|
| **CI e2e 跑全绿** | testing 环境真 deploy + 跑 46 - 3 = 43 spec, 全绿 |
| **反向 grep 守卫** | PR 合并前: F1 / F5 = 0 hit (F2 已 0 hit 加防回归); 文件名 `^[a-z]{2}-\d` 命中数减少 (DELETE + REWRITE 后期望 ≈ 0) |
| **反向证测产品**: 关 backend 跑必 fail | 此 PR 只验"绿是真的过", 反向证留 follow-up 由 liema 拍 (issue §验收提的, 不在此 PR 第一轮做完, 二轮加) |
| **手工真验 (testing)** | liema 拉 PR 分支 → push 触发 Deploy Test → testing-borgee.codetrek.cn 浏览器抽 5 个 REWRITE spec 真验流程 |

## §8 风险

| 风险 | 缓解 |
|---|---|
| REWRITE 16 spec 工作量过大 (5 天估) 拖 P0 | 分批 commit, 每 4-6 个 spec 一 commit, 逐步 push, CI 增量证 |
| 重写后 spec flaky | playwright config 已 retries=1 in CI; 加 `await expect(...).toBeVisible()` 不用 `waitForTimeout` |
| client UI 实际没对应入口给重写 (e.g. comment search) | 标 follow-up issue + 当前 spec 留 `test.skip` 加 todo 注释 (允, 因为是"产品 UI 缺失"非"e2e 偷懒") |
| 一 PR 大被驳回拆 | 跟 teamlead 提前沟通 P0 + issue 明文要求"一次做干净", 拒拆 |
| 重命名跟 docs/qa/REG-* 锚链接撞 | 反向 grep `docs/qa/` 找 spec 名引用同 PR 改 |
| 有人合 main 后撞我 worktree | push 前 `git fetch && git log HEAD..origin/main` 看 diverge, 必 rebase (memory `dev_push_must_rebase_main_first`) |

## §9 实施步骤 (4 签后)

1. **DELETE** 3 spec: `git rm packages/e2e/tests/{cv-3-3-deferred,g2.4-adm-0-stance,hb-1b-installer}.spec.ts`
2. **PASS rename** 24 spec: `git mv` 按 audit.md 列表; 每个文件头部注释重写自然语言 (去 "立场承袭" / "byte-identical" / "锚" 等黑话, 改 "跟 X 一致" / "字面相等" / "关联")
3. **PASS+fix** 3 spec: cv-4-iterate / hb-2-v0d / rt-1-2-backfill — 局部去 page.evaluate mock 注入, 改真数据 seed; 不行就保留 + 加注释说明为何用 page.evaluate
4. **REWRITE** 16 spec: 按 audit.md 列表逐个重写 (admin seed 留, 主体改 page.click + DOM 断)
5. **反向 grep 守卫**: 加到 `audit.md` 末尾, PR description 也写一份, 让 review 容易扫
6. **rebase main**: push 前 `git fetch && git rebase origin/main` (memory `dev_push_must_rebase_main_first`)
7. **push**: `git push -u origin feat/716-e2e-real-ui-audit` 触发 Deploy Test workflow
8. **真验**: testing-borgee.codetrek.cn 浏览器抽 5 个 REWRITE spec 真跑确认绿
9. **flip**: progress.md 翻 [x], regression.md 状态翻 ✅, acceptance.md 翻 [x]
10. **teamlead 开 PR** (Closes gh#716), 等 4 签 + 三签 (PR review flow) → squash merge

## §10 跟其它在飞 PR 关联

- gh#717 (zhanma-e release-gate): 互不影响, e2e spec 重写不动 release workflow
- gh#718 (yema 黑话整治): 文档黑话整治在 docs/, 此 PR 整治 e2e spec 注释 — 同方向不冲突. spec 注释里的黑话由此 PR 处理.
- gh#698 (zhanma-d form 排版, 已合): spec gh-698-agent-config-form-layout 重命名为 agent-config-form-layout 不破

## §11 留账 (followup, 不在本 PR)

| Followup | 关联 | 备注 |
|---|---|---|
| 反向证 e2e 真在测产品 (关 backend 后必 fail) | issue §验收 | 此 PR 第一轮做"消除假 e2e"; 反向证机制 (kill backend → 全 fail) 由 liema follow-up 立基础设施 |
| client UI 缺失导致 REWRITE 改 test.skip 的 spec | follow-up issue | comment-search / dm-reaction-summary 等 client 没真 UI 路径的, 单立产品 issue 由 yema 排 |
| docs/qa/ 内引用旧 spec 名的链接 | 同 PR 改 | grep `docs/qa/` 内 `*.spec.ts` 引用, 同 PR 改名一并改 |

---

**等待 4 签**:
- [ ] **feima** (架构): 数据流 §1 + 多方案选 A 合理性 + 反向 grep 锚 §6 完整
- [ ] **yema** (PM): e2e 整治带的产品价值 (CI 信任度) + 描述去黑话方向
- [ ] **heima** (Security): 无 auth/permission 路径变化, REWRITE 用 admin login seed 跟现有模式一致, 不引入新凭据/路径
- [ ] **liema** (QA): audit.md 46 行 1:1 覆盖 + 反向 grep 守卫够防回归 + 反向证留 followup 合理
