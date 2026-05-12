# e2e — Playwright E2E test suite

代码位置: `/workspace/borgee/packages/e2e/`

> INFRA-2 (Phase 2 R3 解封前置) — Playwright scaffold. RT-0 (#40 latency stopwatch), CM-onboarding (#42 注册→Welcome 流), CM-4 第 4 个 demo (G2.1/G2.2/G2.4 邀请审批 / 离线 fallback / 用户感知签字) 都由这个 package 覆盖。

## 1. 设计

- **独立 pnpm workspace 包** `@borgee/e2e` (不挂在 `@borgee/client` 下), 避免把 `@playwright/test` ~150MB 拖进 client build。
- **真实进程启动**, 不使用 mock: server-go 跑 `go run ./cmd/collab`, client 跑真实 vite dev, 行为跟开发环境一致 (CM-4.2 60s polling 那种集成 bug 单测抓不出, 只能 E2E)。
- **端口隔离**: E2E 跑 server-go 在 `4901`, vite 在 `5174` (开发默认 4900/5173 留给开发者, `pnpm test` 不占用这些端口)。
- **数据隔离**: sqlite db 写入 `packages/e2e/.playwright-data/collab-e2e.db`, 每次 run 复用 (CI runner workspace 自动清空)。

## 2. 目录

```
packages/e2e/
├── package.json              # @borgee/e2e
├── playwright.config.ts      # 双 webServer 编排 + projects
├── tsconfig.json
├── fixtures/
│   ├── auth.ts               # 占位, CM-onboarding/RT-0 真接
│   └── stopwatch.ts          # G2.4 ≤ 3s latency 测量 + HTML 报告附件
└── tests/
    ├── smoke.spec.ts                     # 3 条: server health / client title / vite proxy
    ├── chat-first-time-onboarding.spec.ts             # CM-onboarding 注册→Welcome 流
    ├── chat-realtime-message-fanout.spec.ts             # CM-4 邀请审批 / 离线 fallback
    ├── realtime-backfill-on-reconnect.spec.ts  # RT-1.2 断线后 backfill
    ├── channel-list-sidebar.spec.ts      # CHN-1.3 频道列表 DnD
    ├── agent-list-presence-indicator.spec.ts       # AL-3.3 agent 在线点
    └── canvas-modal-open-close.spec.ts             # CV-1.3 Canvas tab markdown+WS push (§3.1-§3.3)
```

`canvas-modal-open-close.spec.ts` 覆盖 cv-1.md §3 的验收项：第 ④ 项要求只渲染 Markdown 内容；第 ③/⑦ 项要求 rollback 控件只对 owner 出现在 DOM 中，并且版本 label 必须与 `"v{N+1} (rollback from v{M})"` 字节级一致；第 ②/⑤ 项要求页面通过 WS push 在 3 秒内刷新，并在 409 场景显示 toast `内容已更新, 请刷新查看`。这两个测试用例使用真实 server-go + vite，耗时约 3.7s；其中一次提交通过 REST 以 other-user 身份创建 commit，用来触发 push。

## 3. 双 server 编排

`playwright.config.ts` 的 `webServer: [...]` 起两个进程:

| Server | 命令 | URL | 健康检查 |
|---|---|---|---|
| server-go | `go run ./cmd/collab` (cwd=server-go) | `http://127.0.0.1:4901` | `GET /health` |
| client | `pnpm --filter @borgee/client dev --port 5174 --strictPort` | `http://127.0.0.1:5174` | `GET /` |

server-go 通过环境变量切换:
- `PORT=4901 HOST=127.0.0.1` 隔离开发端口
- `DATABASE_PATH` 落 e2e tmp 目录
- `JWT_SECRET=e2e-test-secret-not-for-prod` 固定 (跨 run 复用 token 没意义)
- `ADMIN_USER` / `ADMIN_PASSWORD` env bootstrap (ADM-0 之后改吃 `admins` 表)
- `BORGEE_ADMIN_LOGIN=e2e-admin` + `BORGEE_ADMIN_PASSWORD_HASH=$2a$10$...` — ADM-0.1 startup must fail visibly when required env vars are missing: env 任一缺 → server panic, 所以 e2e webServer 必须显式提供。bcrypt cost=10, 明文 `e2e-admin-pass-12345` (e2e 专用, 永不进 prod)。改 hash = 同步改 `playwright.config.ts` 的 `BORGEE_ADMIN_PASSWORD_HASH` 字面。

client 通过 **`VITE_E2E_API_TARGET`** 把 vite proxy 从写死的 `localhost:4900` 切到 e2e server 端口。`packages/client/vite.config.ts` 读这个 env, 默认仍是 `4900` (开发流不变)。

## 4. 关键 fixture

### `fixtures/stopwatch.ts`

用于验证 G2.4 ≤ 3s 条件。RT-0 (#40) 是第一个接入方。

```ts
const sw = stopwatch();
await senderPage.click('[data-testid=invite-send]');
await receiverPage.waitForSelector('[data-testid=invitation-toast]');
sw.stop();
await sw.attach(testInfo, '邀请→通知 latency');  // 进 HTML 报告 (野马读)
expect(sw.ms).toBeLessThanOrEqual(3000);
```

`attach()` 把毫秒数写进 Playwright HTML 报告附件, reviewer 跑 demo 时不用打开 trace viewer 就能读延迟。

### `fixtures/auth.ts`

**故意是占位**。`seedUser` / `login` 都直接 throw, 防止其他测试在 invite-code seed env 落地之前 (CM-onboarding #42) 提前接入 auth；失败必须显式暴露。pattern 内联在文件注释里。

## 5. CI 集成

`.github/workflows/ci.yml` 加 `e2e` job:

- runs-on `ubuntu-latest`
- 装 pnpm + node22 + go1.25
- `pnpm install --filter @borgee/e2e --filter @borgee/client` (2 个 workspace, lockfile frozen)
- `pnpm --filter @borgee/e2e exec playwright install --with-deps chromium` (仅 chromium ≈ 200MB, 其它 browser 留到 RT-0 需要时再装)
- `pnpm --filter @borgee/client build` 一次 (vite dev 不需要, 但保险 + 跑 tsc 严格模式)
- `pnpm --filter @borgee/e2e test`
- 失败时上传 `packages/e2e/playwright-report/` 作为 artifact (reviewer 直接看)

CI 总耗时预估: 装 chromium ~1min + go build ~10s + smoke 3 条 ~5s + teardown = **~2min**。后续 RT-0 + CM-onboarding 加 5-10 条会涨到 4-5min, 仍在可接受范围。

## 6. 不在范围 (留给后续 milestone)

- 真实 auth fixture (CM-onboarding #42 invite-code seed 接入后再接 RT-0)
- WebSocket 测试工具 (RT-0 加 `ws.frame()` helper, 验 `agent_invitation_pending` schema)
- 移动布局快照 (CV-* 阶段)
- 第二浏览器 (firefox / webkit) — 现在只 chromium, 见 G2.4 截屏只需 1 个 browser
- 跨 PR retry orchestration / shard — 测试条数 < 50 之前不需要

## 7. 本地跑

```sh
pnpm --filter @borgee/e2e install-browsers  # 一次性, 安装 chromium
pnpm --filter @borgee/e2e test              # 跑全套
pnpm --filter @borgee/e2e test:headed       # 看浏览器跑 (debug)
pnpm --filter @borgee/e2e test:ui           # Playwright UI mode
pnpm --filter @borgee/e2e report            # 看上次 HTML 报告
```

如果端口被占: `E2E_SERVER_PORT=4902 E2E_CLIENT_PORT=5175 pnpm --filter @borgee/e2e test`.
