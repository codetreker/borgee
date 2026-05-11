# 716 实施设计 — e2e 真 UI 审计 + 重写

> Issue: gh#716 (P0 / current-iteration)
> Author: zhanma-c (Dev, 主笔); zhanma-d 接手重写 (zhanma-c hung)
> Worktree: `/workspace/borgee/.worktrees/716-e2e-real-ui-audit/` (`feat/716-e2e-real-ui-audit`)
> 关联: `audit.md` (4 reviewer 复核版, commit 8103745) / `spec.md` / `stance.md` / `content-lock.md` / `acceptance.md`
> 待 4 签: feima (架构) / yema (PM) / heima (Security) / liema (QA)

## §0 范围

全量审计 `packages/e2e/tests/**/*.spec.ts` (46 spec), 按 5 类反模式 (F1 fs lint / F2 page.evaluate fetch / F3 纯 REST 绕 UI / F4 源码 grep / F5 noop) 分类:

- **PASS** (26 spec): 已是真 UI, 留 + 重命名 + 头部注释去黑话
- **PASS+fix** (1 spec): 真 UI 主体, 局部 cosmetic page.evaluate, 留 + 注释
- **REWRITE-UI** (8 spec): client 有真 UI 路径, 改 page.click + DOM 断
- **REWRITE-NAV** (3 spec): ACL 越权类, client UI 不暴露无权资源, 改 page.goto + DOM 反向证 forbidden 状态
- **SKIP+followup** (7 spec): client UI 0 production mount, 改 test.describe.skip + 引 gh#724 §1
- **DELETE** (3 spec): 死代码 / noop 占位 / 源码 grep 假 e2e, 直接 git rm

详细分类表 + 4 reviewer 反馈 + 复核记录: 同目录 `audit.md`.

不在范围:
- `packages/e2e/playwright.config.ts` 不动 (双 server 编排合理)
- `packages/e2e/fixtures/` 不动 (helper 不是 spec, fs.* 在这里允许)
- 不改 client/server 产品代码 (e2e 重写不允许带产品改动; REWRITE-NAV 反向证 forbidden state 若 client 没 UX 渲染, 走 gh#724 §2 followup)

## §1 数据流

```
Dev 跑 audit → 标 46 spec 6 类 → 一 PR 内并行做:

  ┌─ DELETE 3 → git rm
  ├─ PASS 26 → git mv + 头部注释去黑话 (zhanma-d 已 push 4 组 + 3 yema rename)
  ├─ PASS+fix 1 (hb-2-v0d) → git mv + 加注释说明 page.evaluate 是 cosmetic
  ├─ REWRITE-UI 8 → 重写真 UI (admin seed + page.click + DOM 断)
  ├─ REWRITE-NAV 3 → page.goto 无权资源 URL + DOM 反向断 sidebar 空 / message 空 / fallback redirect
  └─ SKIP+followup 7 → git mv + test.describe.skip + 头部引 gh#724 §1 (zhanma-d 已 push)
                  ↓
              push to feat/716-e2e-real-ui-audit
                  ↓
              CI e2e 真跑 (39 active + 7 skip)
                  ↓
              反向 grep 守卫: F1 / F2 / F4 / F5 = 0 hit; F3-1 PURE_REST = 0 hit
                  ↓
              teamlead 开 PR + 4 角色 review + 三签合
```

## §2 数据模型

无 schema 改动. 此 PR 仅改 e2e spec 文件本身 + CI workflow 阈值 + docs/current/ 引用.

## §3 API contract

无 API 改动. 改的是 e2e 怎么测 API + UI. server endpoint 不动.

## §4 边界 / 错误处理

| 场景 | 处理 |
|---|---|
| REWRITE-UI 真 UI 跑了真 server, 但 client 没有对应入口 | 改 SKIP+followup (audit §"client UI 缺失判断步骤" 3 步走), 不降级回 REST e2e |
| REWRITE-NAV 跑了 page.goto 无权 URL, client 没 forbidden state UX | 立 gh#724 §2 follow-up; 当前 spec 真断"什么都不渲染" (sidebar 空 / message 空 / input 不出现) |
| PASS+fix 改 hb-2-v0d 的 page.evaluate (注 innerHTML 美化截图) | 加注释说明 cosmetic 不影响测试主体, 不强删 |
| DELETE 后 CI 失败 (有人 import 这 spec) | grep 引用面后再删 (Playwright spec 默认不互 import, fixtures 不引 spec) |
| REWRITE 双 tab (cm-4 / dm-3 / rt-3 / cm-5) 一边 disconnect | 用 `browser.newContext()` 起两独立 context, 不共 cookie |
| docs/qa/screenshots/ PNG 文件 | 跟此 PR 无关 (PR #715 已删过), 不动 |
| 反向 grep 守卫破阈 (release-gate.yml e2e-fixme-skip-guard) | 已扩 4→11 (容 7 cv-* SKIP+followup), zhanma-d commit 36dcfba |

## §5 多方案对比

### 方案 A: 一 PR 全做 (选定)

PR 内 commit 序 (zhanma-d 已 push 部分, zhanma-c REWRITE 在做):
1. ✅ docs (audit / stance / content-lock / progress / spec / acceptance / regression / design)
2. ✅ DELETE 3 (commit 508067d)
3. ✅ PASS rename 26 + 头部去黑话 (commit 9d31840 / 642be70 / 67cb7b6 / a0fc337 + 3 yema rename 6e56366)
4. ✅ SKIP+followup 7 cv-* (commit abc7394)
5. ✅ CI release-gate 阈值 4→11 (commit 36dcfba)
6. ✅ docs/current/ 引用改新名 (commit 6e56366)
7. ⏳ REWRITE-UI 8 (zhanma-c 做, welcome-channel-per-user-isolation 已 done 10e2319, 剩 7 个)
8. ⏳ REWRITE-NAV 3 (zhanma-c 做)
9. ⏳ PASS+fix 1 (zhanma-c 做)

**Pro**:
- issue 描述 §"P0 一次做干净不留尾" + 用户 "一 milestone 一 PR" 铁律一致
- 跨 spec 重命名 atomic, 不出现"一半旧名一半新名"中间态
- docs/current/ 引用预改, REWRITE 后真 git mv 直接对位

**Con**:
- PR diff 大 (~1500 行净改动), review 重 (4 角色扫 46 spec)
- 缓解: review 重点不是逐行 logic, 而是 "spec 跑通 + 反模式 0 hit + audit.md 1:1 覆盖"

### 方案 B: 拆三 PR (按动作: DELETE / PASS rename / REWRITE)

撞 "一 milestone 一 PR" 铁律. PASS rename 跟 REWRITE 撞同 spec (e.g. cv-7 rename 后 REWRITE 又改) 必须串行, 不 atomic. 跨 PR 周期长, P0 拖. **不选**.

### 方案 C: 只删假 + 重命名, REWRITE 拆 16 follow-up issue

跟 issue body §"不允许留着以后改, P0 一次做干净" 直撞. 假 e2e 只改名没改实质, 没解决问题. **不选**.

### 选 A

issue 描述 §3 末段 + §"处理动作" 明文要求 "P0 一次做干净, 不允许 '留着以后改'". 跟用户铁律 `strict_one_milestone_one_pr` + `dispatch_grep_first_no_assumptions` 一致.

## §6 集成 (反向 grep 守卫 — PR 合前必须满足)

跟 audit.md §"反向 grep 守卫" 同源 (这里只列 design 视角的接口面):

| Grep 检查 | 阈值 | 含义 |
|---|---|---|
| F1 `fs.(existsSync\|stat\|readFileSync\|readdirSync\|statSync)` | 0 hit | 不允许 spec 内查 git 文件 (那是 lint 不是 e2e) |
| F2 `page.evaluate(() => fetch())` | 0 hit | 不允许 cookie 直调后端绕 UI |
| F3-1 PURE_REST (`.goto=0 && .click=0 && apiRequest>0`) | 0 hit (除 test.skip 文件) | 主体路径必须开浏览器 |
| F3-2 apiRequest.newContext 仅 seed | 检查 hit 不在测试主体 | seed 走 REST 允许, 业务 endpoint 直调不允许 |
| F4 `fs.readFileSync.*\.(go\|ts)` | 0 hit (DELETE hb-1b-installer 后) | 不允许源码 grep 假装锁定 |
| F5 `expect(true).toBe(true)` | 0 hit | 不允许 noop 占位 |
| 文件名 `^[a-z]{2}-\d` / `^gh-\d+` / `^g[0-9]` | 0 文件 | 反 milestone / issue / gate 前缀, 按功能命名 |

### F2 严格定义 (liema 复核 + heima 确认)

F2 仅反 `page.evaluate(() => fetch())` 走 cookie 直调后端. `page.evaluate` 访问 `window` / `document` / DOM API / localStorage / WebSocket instance 等真 DOM 行为不属 F2 (audit row 28 cv-4-iterate / row 44 rt-1-2-backfill 复核改 PASS 即依此).

### REWRITE-NAV 不开 F3 例外 (heima 拍)

仍走 `page.goto` 真浏览器 navigate, 浏览器真渲染 forbidden 状态 (即使是空状态), 仍是真 UI 路径. 不是 apiRequest 直调后端. 区别于 REWRITE-UI: NAV 测的是"看不到", UI 测的是"操作得到".

### 反向影响 (不破)

- e2e CI workflow `.github/workflows/ci.yml` / `deploy-test.yml`: 路径引用整目录 `packages/e2e/tests/`, 不引具体文件名 → 重命名不破 ✅
- `playwright.config.ts` `testDir: './tests'` 整目录 → 不破 ✅
- `package.json` scripts: 无具体文件名引用 → 不破 ✅
- `fixtures/`: spec 引 `../fixtures/<helper>`, fixture 不引 spec → 不破 ✅
- `release-gate.yml` e2e-fixme-skip-guard: 阈值 4→11 已改 (commit 36dcfba) 容 7 cv-* SKIP ✅
- `docs/current/` 19 处 spec 引用: zhanma-d 已改 8 文件 11 处 (commit 6e56366), 含 zhanma-c REWRITE 后真 git mv 的预改 ✅

## §7 测试策略 (元测试 — e2e spec 自己怎么验)

| 层 | 覆盖 |
|---|---|
| CI e2e 跑全绿 | testing 环境 deploy + 跑 39 active spec 全绿 + 7 skip 不阻塞 |
| 反向 grep 守卫 | PR 合前: F1 / F2 / F4 / F5 = 0 hit; F3-1 PURE_REST = 0 hit; 文件名前缀守卫 = 0 hit |
| 反向证测产品 (kill backend 必 fail) | 此 PR 第一轮做"消除假 e2e"; 反向证机制 (kill backend → 全 fail) 由 liema follow-up 立基础设施 (gh#724 §3) |
| 手工真验 (testing) | liema 拉 PR 分支 → push 触发 Deploy Test → testing-borgee.codetrek.cn 浏览器抽 5 个 REWRITE spec 真验流程 |
| client UI 缺失判断 3 步 | audit.md §"client UI 缺失判断步骤" (liema Q4 拍): grep production mount → testing 真登录手点 → 1+2 都无才 SKIP. 反 "看着没就 skip" |

## §8 风险

| 风险 | 缓解 |
|---|---|
| REWRITE-UI 8 + REWRITE-NAV 3 工作量大 (3-4d) 拖 P0 | 分批 commit (zhanma-d 协助清掉 6 件机械活, zhanma-c 专注 REWRITE) |
| 重写后 spec flaky | playwright config retries=1; 用 `await expect(...).toBeVisible()` 不用 `waitForTimeout` |
| REWRITE-NAV 真渲染 forbidden state 但 client 没 UX | 立 gh#724 §2 follow-up; 当前 spec 真断"什么都不渲染" |
| 一 PR 大被驳拆 | issue 明文要求"一次做干净" + 用户铁律 strict_one_milestone_one_pr, 跟 teamlead 提前 align |
| 重命名跟 docs/qa/REG-* 锚链接撞 | 反向 grep `docs/qa/` 找 spec 名引用同 PR 改 (audit.md §反向 grep 守卫覆盖) |
| 有人合 main 后撞 worktree | push 前 `git fetch && git rebase origin/feat/716-e2e-real-ui-audit && git rebase origin/main` (memory `dev_push_must_rebase_main_first`) |
| 同 worktree 多 dev 并行撞 race | atomic commit chain (memory `worktree_atomic_commit`): 每动一组 `git add <files> && git commit && git fetch && git rebase && git push` 一行链 |
| zhanma-c hung 中断 | zhanma-d 接 6 件机械活 + design.md 重写, REWRITE 主体仍 zhanma-c (本人 unhung 后继续) |

## §9 实施步骤 (4 签后)

zhanma-d 已做 (commit chain 在 worktree):
1. ✅ DELETE 3 (508067d)
2. ✅ PASS rename 26 4 组 (9d31840 / 642be70 / 67cb7b6 / a0fc337)
3. ✅ SKIP+followup 7 cv-* (abc7394)
4. ✅ CI release-gate 阈值 4→11 (36dcfba)
5. ✅ docs/current/ 引用 + 3 yema rename (6e56366)
6. ✅ design.md 重写 (本 commit)

zhanma-c 做中 (REWRITE 主体):
7. ⏳ REWRITE-UI 8 (welcome-channel-per-user-isolation 已 done 10e2319, 剩 ap-2 / cm-4-realtime / cm-5 / dm-3 happy / dm-5 / rt-3 / hb-2-v0d)
8. ⏳ REWRITE-NAV 3 (ap-4 / ap-5 / dm-3 cross-leak)
9. ⏳ PASS+fix 1 (hb-2-v0d 注释 cosmetic)

合 PR 前:
10. rebase main: `git fetch && git rebase origin/main` (memory `dev_push_must_rebase_main_first`)
11. push 触发 Deploy Test workflow
12. testing 浏览器抽 5 个 REWRITE spec 真验绿
13. flip progress.md [x] / acceptance.md [x] / regression.md ✅
14. teamlead 开 PR (Closes gh#716), 等 4 签 + 三签 (pr-review-flow) → squash merge

## §10 跟其它在飞 PR 关联

- gh#717 (zhanma-e release-gate): 互不影响. zhanma-d 已改 release-gate.yml 阈值, 跟 #717 释放路径不同, 提前对账避撞
- gh#718 (yema 黑话整治): 文档黑话整治在 docs/, 此 PR 整治 e2e spec 注释 — 同方向不冲突. spec 注释黑话由此 PR 处理
- gh#698 (已合): spec gh-698-agent-config-form-layout 重命名 agent-config-form-layout 不破
- gh#724 (follow-up, 已立): §1 ArtifactComments 系列 v2 mount (7 cv-* unskip 触发) / §2 ACL forbidden state UX (REWRITE-NAV 真验渲染时用) / §3 反向证 e2e 基础设施 (liema follow-up)

## §11 留账 (followup, gh#724)

| Followup | 关联 | 状态 |
|---|---|---|
| §1 ArtifactComments 系列 v2 mount | 7 cv-* SKIP+followup | gh#724 已立, v2 brainstorm 后排 |
| §2 ACL forbidden state UX (page.goto 无权 URL 后 client 应渲染 forbidden state) | REWRITE-NAV 3 spec | gh#724 已立, 跟 v2 ACL UX 设计走 |
| §3 反向证 e2e 真在测产品 (kill backend 后全 fail) | liema Q5 复核扩 | gh#724 已立, infra 任务 |
| §4 cm-4-realtime 真 UI 路径补全 (audit 复核改 UI=0) | REWRITE-UI 1/8 | zhanma-c 实施中 |

## §12 4 reviewer 反馈对账 (audit.md 8103745 复核版收录)

| Reviewer | 反馈 | 收录位置 |
|---|---|---|
| **feima** (架构) | Q5: cm-4-realtime UI=1 误判, 真量 .goto/.click/.fill/getBy 全 0, 改 UI=0; 量化阈值边界 (PASS ≥0 反模式 / PASS+fix ≤2 / REWRITE <70% UI 或 ≥3 反模式 / DELETE noop+源码 grep) | audit.md §边界规则 + §复核记录 |
| **liema** (QA) | Q1: 主体路径 .goto=0 + .click=0 + apiRequest>0 = PURE_REST 反模式 (F3-1); Q4: client UI 缺失判断 3 步走 反 "看着没就 skip"; Q5: 反向证 kill backend 必 fail (gh#724 §3 立) | audit.md §反向 grep 守卫 F3-1 + §client UI 缺失判断步骤 + §复核记录 |
| **yema** (PM) | A 方向: cv-5/7/8/9/10/11/12 改 SKIP+followup, 立 gh#724 §1 (client UI 0 mount, 不假装真测); 重命名建议: al-4 / chn-4-followup / cv-4-unfixme-followup 去内部话 (followup/unfixme/acceptance 改功能描述) | audit.md §复核记录 + §SKIP+followup 行 + zhanma-d commit 6e56366 (3 yema rename) |
| **heima** (Security) | ACL 改 REWRITE-NAV (ap-4 / ap-5 / cv-7 §3.4 / dm-3 cross-leak): page.goto 无权 URL + DOM 反向证 forbidden state, 不开 F3 例外; cv-7 6 case ACL 全保 (mount 落地后 unskip 时回归全部, 不允许丢任一 IDOR case); ap-5 不允许砍 cross-user IDOR case 数 | audit.md §REWRITE 子分类 + §复核记录 + zhanma-d commit abc7394 (cv-7 头部跨 §1+§2 引 gh#724 两段) |

---

**等待 4 签**:
- [ ] **feima** (架构): §1 数据流 + 方案 A 选 + 反向 grep 守卫 §6 接口完整 + Q5 量化阈值收录
- [ ] **yema** (PM): SKIP+followup 7 (gh#724 §1) 方向 + 3 重命名去内部话 + e2e 整治带的 CI 信任度产品价值
- [ ] **heima** (Security): REWRITE-NAV 3 不开 F3 例外 + cv-7 6 case ACL 全保 + ap-5 cross-user IDOR 不砍
- [ ] **liema** (QA): audit.md 46 行 1:1 覆盖 + 反向 grep 守卫 (F1-F5 + 文件名前缀 + client UI 3 步) + 反向证 follow-up (gh#724 §3) 合理
