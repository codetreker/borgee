# 716 全量 e2e 审计表

> 时间: 2026-05-09
> 审计人: zhanma-c (Dev)
> 待 review: feima (架构) / liema (QA) / yema (PM) / heima (Security)

## 审计标准 (issue §3 反模式 = 假 e2e)

1. **真 UI** — `page.goto` / `page.click` / `page.fill` / `getByRole` / `locator()` 走浏览器
2. **真断言** — DOM 状态 / URL / 可见文案 / 网络请求结果
3. **不允许**:
   - **F1**: `fs.existsSync` / `fs.stat` / `fs.readFileSync` 检查 git 文件 (那是 lint)
   - **F2**: `page.evaluate(() => fetch())` 走 cookie 直调后端 (那是后端 contract)
   - **F3**: 只用 `apiRequest.newContext` 不开浏览器 (那是 integration test)
   - **F4**: 源码字符串 grep 假装锁
   - **F5**: noop 占位 (`expect(true).toBe(true)`)

## 三种处理动作

- **PASS**: 已是真 UI, 留, 仅文件重命名 + 描述去黑话
- **REWRITE**: 改成真 UI Playwright 跑通 (admin 登录 + click input + 断 DOM)
- **DELETE**: 死代码 / 单元测试已锁源头 / 跟 issue 立场永远撞 — 删

## 全量分类表 (46 spec)

| # | 文件 | 真 UI 信号 | 反模式 | 动作 | 重命名建议 | 备注 |
|---|---|---|---|---|---|---|
| 1 | `adm-1-privacy-promise.spec.ts` | UI=15 | — | **PASS** | `admin-privacy-promise-banner.spec.ts` | 真测红横幅 + admin 入 channel 反约束 |
| 2 | `adm-2-followup.spec.ts` | UI=6 | — | **PASS** | `admin-audit-deletion-followup.spec.ts` | 真测 admin 删除审计页 |
| 3 | `adm-3-audit-events.spec.ts` | UI=5 | — | **PASS** | `admin-audit-event-stream.spec.ts` | 真测 audit 事件流 UI |
| 4 | `al-3-3-presence-dot.spec.ts` | UI=7 | — | **PASS** | `agent-list-presence-indicator.spec.ts` | 真测 agent 列表在线点 |
| 5 | `al-4-acceptance-followup.spec.ts` | UI=25 | — | **PASS** | `agent-list-acceptance-followup.spec.ts` | 真测 agent 列表多场景 |
| 6 | `ap-2-bundle.spec.ts` | UI=3 | — | **REWRITE** | `agent-permission-bundle.spec.ts` | UI 信号少, 多 case 是 REST 直调; 改走 client 设置页 |
| 7 | `ap-4-reactions-acl.spec.ts` | UI=0 | F3 | **REWRITE** | `reactions-cross-channel-permission.spec.ts` | 纯 REST, 改走 client message 反应 UI |
| 8 | `ap-5-messages-acl-matrix.spec.ts` | UI=0 | F3 | **REWRITE** | `message-permission-matrix.spec.ts` | 纯 REST 矩阵, 改走 UI 选择 ≤3 关键 case + 真点击发送 |
| 9 | `chn-1-3-channel-list.spec.ts` | UI=18 | — | **PASS** | `channel-list-sidebar.spec.ts` | 真测 channel sidebar |
| 10 | `chn-2-3-dm-flow.spec.ts` | UI=22 | — | **PASS** | `direct-message-flow.spec.ts` | 真测 DM 创建 + 互发 |
| 11 | `chn-3-3-sidebar-reorder.spec.ts` | UI=18 | — | **PASS** | `channel-sidebar-reorder.spec.ts` | 真测 sidebar 拖动重排 |
| 12 | `chn-4-collab-skeleton.spec.ts` | UI=21 | — | **PASS** | `channel-collab-tabs.spec.ts` | 真测协作场双 tab |
| 13 | `chn-4-followup.spec.ts` | UI=6 | — | **PASS** | `channel-collab-followup.spec.ts` | 真测协作场 dm/cross-org 边界 |
| 14 | `chn-4-screenshots-followup.spec.ts` | UI=12 / FS=1 | F1 (§4§5 已注释删) | **PASS** | `channel-collab-screenshots.spec.ts` | §1-§3 真截图; §4§5 已删的 fs.stat 注释清掉 |
| 15 | `cm-4-bug-029-name-display-regression.spec.ts` | UI=6 | — | **PASS** | `chat-name-display-regression.spec.ts` | 真测姓名展示回归 |
| 16 | `cm-4-realtime.spec.ts` | UI=1 | F3 | **REWRITE** | `chat-realtime-message-fanout.spec.ts` | 1 个 page.goto 后全 REST, 改双 tab UI 真互发 |
| 17 | `cm-5-x2-collab.spec.ts` | UI=4 | F3 | **REWRITE** | `chat-two-user-collab.spec.ts` | UI 弱, 改双 page click 真发 |
| 18 | `cm-onboarding-bug-030-regression.spec.ts` | UI=0 | F3 | **REWRITE** | `welcome-channel-per-user-isolation.spec.ts` | 纯 REST, 改 register 后 client 真看 sidebar 有无 #welcome |
| 19 | `cm-onboarding.spec.ts` | UI=5 | — | **PASS** | `chat-first-time-onboarding.spec.ts` | 真测新用户 onboarding 流 |
| 20 | `cv-1-3-canvas-modal-a11y.spec.ts` | UI=26 | — | **PASS** | `canvas-modal-accessibility.spec.ts` | 真测 canvas modal 无障碍 |
| 21 | `cv-1-3-canvas.spec.ts` | UI=35 | — | **PASS** | `canvas-modal-open-close.spec.ts` | 真测 canvas 开关 |
| 22 | `cv-10-comment-draft.spec.ts` | UI=3 | F2 (page.evaluate localStorage) | **REWRITE** | `comment-draft-persistence.spec.ts` | 全用 page.evaluate(localStorage) 模拟; 改真 textarea input + reload + DOM 断 |
| 23 | `cv-11-comment-markdown.spec.ts` | UI=1 | F3 + F4 (§3.2 自己注释承认 sanity smoke) | **REWRITE** | `comment-markdown-render.spec.ts` | §3.1/§3.3 server raw 是 contract 删; §3.2 真测 client DOM 渲染 strong/em/code |
| 24 | `cv-12-comment-search.spec.ts` | UI=0 | F3 | **REWRITE** | `comment-search-filter.spec.ts` | 纯 REST, 改 client search input + DOM 断结果 |
| 25 | `cv-2-3-anchor-client.spec.ts` | UI=25 | — | **PASS** | `comment-anchor-scroll.spec.ts` | 真测评论锚点滚动 |
| 26 | `cv-3-3-deferred.spec.ts` | UI=0 | F5 (`expect(true).toBe(true)`) | **DELETE** | — | 27 行 noop 占位, 死代码 |
| 27 | `cv-3-3-renderers.spec.ts` | UI=12 | — | **PASS** | `artifact-renderer-types.spec.ts` | 真测 artifact 渲染器 |
| 28 | `cv-4-iterate.spec.ts` | UI=22 | F2 (1 处 page.evaluate 注 mock) | **PASS+fix** | `artifact-iterate-version.spec.ts` | UI 真但 1 处 page.evaluate 注 mock — 改成真 server 数据 seed |
| 29 | `cv-4-unfixme-followup.spec.ts` | UI=20 | — | **PASS** | `artifact-iterate-followup.spec.ts` | 真测 iterate followup |
| 30 | `cv-5-artifact-comment.spec.ts` | UI=0 | F3 | **REWRITE** | `artifact-comment-thread.spec.ts` | 纯 REST, 改 client 真发评论 + DOM 断帖出 |
| 31 | `cv-7-comment-edit-delete.spec.ts` | UI=0 | F3 | **REWRITE** | `comment-edit-delete-permission.spec.ts` | 纯 REST 6 case, 改 client UI 真编辑 + 真删 + 反应; 反约束 sanity 留 server 单测 |
| 32 | `cv-8-comment-thread-reply.spec.ts` | UI=0 | F3 | **REWRITE** | `comment-thread-reply.spec.ts` | 纯 REST, 改 client 真点回复 + 真发 |
| 33 | `cv-9-comment-mention.spec.ts` | UI=0 | F3 | **REWRITE** | `comment-mention-dispatch.spec.ts` | 纯 REST, 改 client 真打 @ + 真选 + 真发 + DOM 断 |
| 34 | `dl-4-pwa-subscribe.spec.ts` | UI=1 | — | **PASS** | `pwa-push-notification-subscribe.spec.ts` | 真测 PWA 订阅 (单 case) |
| 35 | `dm-3-multi-device-sync.spec.ts` | UI=0 | F3 (注释自己说"REST-driven, 不走 dual-page UI") | **REWRITE** | `direct-message-multi-device-sync.spec.ts` | 改双 browser context 真 DM + 断对端 DOM 出现 |
| 36 | `dm-5-reaction-summary.spec.ts` | UI=0 | F3 | **REWRITE** | `direct-message-reaction-summary.spec.ts` | 改 UI 点 reaction + 断 summary DOM |
| 37 | `g2.4-adm-0-stance.spec.ts` | UI=0 | F5 (`expect(true).toBe(true)`) | **DELETE** | — | 19 行 noop, 自己注释承认 "audit真删, 单元测试已锁" |
| 38 | `g2.4-demo-screenshots.spec.ts` | UI=6 | — | **PASS** | `demo-screenshot-archive.spec.ts` | 真截图 demo |
| 39 | `gh-684-agent-detail-credentials.spec.ts` | UI=18 | — | **PASS** | `agent-detail-credentials-display.spec.ts` | 真测 agent 详情凭据 |
| 40 | `gh-698-agent-config-form-layout.spec.ts` | UI=18 | — | **PASS** | `agent-config-form-layout.spec.ts` | 真测 agent 配置 form 排版 |
| 41 | `hb-1b-installer.spec.ts` | UI=0 | F1 + F4 (5 case 全 fs.readFileSync 源码 grep) | **DELETE** | — | 全 5 case 源码 grep 假装 e2e; Go unit 已锁 |
| 42 | `hb-2-v0d.spec.ts` | UI=6 | F2 (page.evaluate WS msg inject) | **PASS+fix** | `host-bridge-daemon-handshake.spec.ts` | UI 真但 2 处 page.evaluate 注 WS — 检查能否走 server 真推; 不行就保留并加注释 |
| 43 | `me-1-self-message-unread.spec.ts` | UI=22 | — | **PASS** | `self-message-unread-counter.spec.ts` | 真测自己消息不计未读 |
| 44 | `rt-1-2-backfill-on-reconnect.spec.ts` | UI=4 | F2 (page.evaluate WS) | **PASS+fix** | `realtime-backfill-on-reconnect.spec.ts` | UI 弱; 改双 tab 真 disconnect/reconnect |
| 45 | `rt-3-presence.spec.ts` | UI=4 | F3 | **REWRITE** | `realtime-presence-broadcast.spec.ts` | UI 弱, 改双 tab 真 presence DOM 断 |
| 46 | `smoke.spec.ts` | UI=1 | — | **PASS** | `smoke-app-loads.spec.ts` | 真打开首页 |

## 汇总

- **PASS** (留 + 重命名 + 描述 refine): 24 spec
- **PASS+fix** (留但小改 page.evaluate 注 mock): 3 spec (cv-4-iterate / hb-2-v0d / rt-1-2-backfill)
- **REWRITE** (重写真 UI): 16 spec
- **DELETE** (死代码 / noop / 源码 grep 占位): 3 spec (cv-3-3-deferred / g2.4-adm-0-stance / hb-1b-installer)

合计 46 = 24 + 3 + 16 + 3 ✅

## 反向 grep 守卫 (合 PR 时必须 0 hit)

```bash
# F1: 任何 fs.* 检查 git 文件
grep -rE "fs\.(existsSync|stat|readFileSync|readdirSync|statSync)" packages/e2e/tests/*.spec.ts
# 期望: 0 hit (除 fixtures/ 内 helper, 不在 tests/)

# F2: page.evaluate 走 fetch (cookie 直调后端)
grep -rE "page\.evaluate\([^)]*=>[^)]*fetch" packages/e2e/tests/*.spec.ts
# 期望: 0 hit

# F5: noop 占位
grep -rE "expect\(true\)\.toBe\(true\)" packages/e2e/tests/*.spec.ts
# 期望: 0 hit
```

## 边界

- **保留 apiRequest 用于 seed** (admin login + invite + register + create channel): 这是 precondition setup, 不是测试主体. 测试主体必须走 page click + DOM 断. e2e 反模式 F3 指 "只打 API 不开浏览器", **如果 spec 主测试路径走真 UI, seed 用 REST 不算反模式**.
- **保留 page.evaluate 用于真 DOM 探测** (e.g. `getBoundingClientRect()` 量排版, `localStorage.getItem` 验存储): 这是真 DOM 检查, 不是绕过 UI. 反模式 F2 仅指 `page.evaluate(() => fetch())` 走 cookie 直调后端.
