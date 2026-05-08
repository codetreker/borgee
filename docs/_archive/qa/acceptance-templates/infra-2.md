# Acceptance Template — INFRA-2: Playwright E2E scaffold

> Implementation: `docs/implementation/PROGRESS.md` Phase 2 解封前置 (R3)
> 依赖: 无 (基础设施, 不依赖任何 milestone)
> 阻塞: **RT-0 / CM-onboarding / G2.4 野马 ≤3s 硬条件 / G2.audit 全部依赖此 PR**
> 烈马 R3 review 立场 (#188): "INFRA-2 必须前置到 CM-4.3a 之前, 不能等 G2.audit"

## 验收清单

### Playwright 安装与配置

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `packages/client/playwright.config.ts` (或 `playwright.config.{js,mjs}`) 文件存在 | CI grep | 战马 (实施) / 烈马 (验) | _(待填)_ |
| Config `projects` 数组至少含 `chromium` (单浏览器即可, multi-browser 留 v1) | unit (config import + assert) | 战马 / 烈马 | _(待填)_ |
| Config `webServer` 项配置 (启 vite dev or preview server) 或文档说明手动启 | 人眼 (PR review) | 飞马 / 烈马 | _(待填)_ |
| `package.json` `devDependencies` 含 `@playwright/test` (锁定版本) | CI grep | 战马 / 飞马 | _(待填)_ |
| `pnpm-lock.yaml` 同步更新 (锁版本) | 人眼 | 飞马 | _(待填)_ |

### Smoke test (≥ 1 个能跑通)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 至少 1 个 `*.spec.ts` 文件存在 (建议路径 `packages/client/e2e/` 或 `packages/e2e/`) | CI grep | 战马 / 烈马 | _(待填)_ |
| Smoke test 内容: 登录页加载 + DOM 含 "登录" / "Login" 任一字面 (蓝图无关, 仅验 runner 通) | E2E | 战马 / 烈马 | _(待填)_ |
| 本地 `pnpm --filter client exec playwright test` (or 同义命令) 退出 0 | 人眼 + CI 跑 | 战马 / 烈马 | _(待填, 本地 / CI 任一)_ |
| 测试可在 sandbox/CI Linux 跑通 (Chrome binary 路径自动 resolve 或 doc) | CI | 飞马 / 烈马 | _(待填)_ |

### 命令入口

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `npm run test:e2e` 或 `pnpm test:e2e` 或 `make test-e2e` 任一存在 | CI grep | 战马 / 飞马 | _(待填, package.json scripts 或 Makefile)_ |
| 命令 fail-fast: 测试失败时退出码非 0 (不吞错) | 人眼 (跑一次 force fail) | 烈马 | _(待填)_ |

### CI workflow 集成

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `.github/workflows/` 含 playwright job (新文件 or 加进现有 client.yml) | CI grep | 飞马 / 战马 | _(待填)_ |
| Job step 含 `playwright install --with-deps chromium` (或缓存命中) | 人眼 (PR review) | 飞马 | _(待填)_ |
| Browser binary cache (actions/cache) 命中后避免重复下载 (≥ 第二次 CI 跑时间 < 第一次 - 60s) | 人眼 | 飞马 | _(待填, 跑 2 次 CI 比 timing)_ |
| Test 失败时上传 trace / screenshot artifact (debug 兜底) | 人眼 (PR review) | 飞马 | _(待填)_ |
| Job 在 PR CI 检查列表里展示 (与 go-test / lint 并列) | CI | 飞马 | _(待填)_ |

### testutil 桥 (server 端启 fixture)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| Playwright 测试可启动一个 backed-by-`testutil.OpenSeeded` 的 server 实例 (or doc 说明) | 人眼 (PR review) | 战马 / 飞马 | _(待填)_ |
| 每个测试隔离 db (per-test fresh sqlite or 清表) | 人眼 + unit | 战马 / 烈马 | _(待填)_ |

### 文档

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `docs/current/client/` 或 `packages/client/README.md` 加 "如何跑 E2E" 1 节 | 人眼 (PR review) | 战马 / 飞马 | _(待填)_ |
| 文档含: 命令 / 选浏览器 / 看 trace 路径 / 跑单个 spec 命令 | 人眼 | 飞马 | _(待填)_ |

### 不在范围 (避免 PR 膨胀)

- ❌ 真实业务 E2E (RT-0 / CM-onboarding / G2.4 各自负责)
- ❌ 多浏览器 matrix (firefox / webkit) — v1 范围
- ❌ Visual regression (screenshot diff) — v1 范围
- ❌ Accessibility / lighthouse — 单独 milestone

### 退出条件

- 上表 16 项全绿 (CI grep 5 + unit 1 + E2E 1 + 人眼 9)
- Smoke test 在 CI 跑通至少 1 次 (PR check 绿)
- 战马 / 飞马 / 烈马三方任一 (野马不强制) PR review +1
- README "如何跑 E2E" 1 节 merge 后 1 周内野马至少自跑过 1 次 (验文档可读)

### 后续 milestone 立刻能用

INFRA-2 merge 后, 立刻**解锁**:
- RT-0 latency ≤3s stopwatch (野马 G2.4 硬条件)
- CM-onboarding happy path E2E (步骤 1 → 5 + 4 条 error 文案)
- ADM-0.3 cookie 串扰跨页跳转 E2E (G2.0 一票否决补强)
- §11 反约束 fault injection E2E

烈马预计 INFRA-2 merge 后 2 天内交付 RT-0 / CM-onboarding 第一批 spec。
