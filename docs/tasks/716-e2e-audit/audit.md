# 716 全量 e2e 审计表

> 时间: 2026-05-09
> 审计人: zhanma-c (Dev)
> 复核: heima (ACL 路径) / feima (架构) / liema (QA 守卫) / yema (PM)

## 审计标准 (issue #716 §3 反模式 = 假 e2e)

1. **真 UI** — `page.goto` / `page.click` / `page.fill` / `getByRole` / `locator()` 走浏览器
2. **真断言** — DOM 状态 / URL / 可见文案 / 网络请求结果
3. **不允许**:
   - **F1**: `fs.existsSync` / `fs.stat` / `fs.readFileSync` / `fs.readdirSync` 检查 git 文件 (那是 lint, 不是 e2e)
   - **F2**: `page.evaluate(() => fetch())` 走 cookie 直调后端 (那是后端 contract test)
   - **F3**: 只用 `apiRequest.newContext` 不开浏览器 (那是 integration test)
   - **F4**: 源码字符串 grep 假装锁
   - **F5**: noop 占位 (`expect(true).toBe(true)`)

## 处理动作 (4 类)

- **PASS**: 已是真 UI, 留, 仅文件重命名 + 描述去黑话
- **PASS+fix**: 真 UI 主体, 局部 page.evaluate cosmetic 用法, 留 + 加注释或局部小改
- **REWRITE**: 改成真 UI Playwright 跑通 (admin 登录 seed + 浏览器真点击 + DOM 真断)
- **DELETE**: 死代码 / 单元测试已锁源头 / 跟 issue 立场永远撞 — 删

### 边界规则 (feima 提的量化阈值)

- **PASS**: UI 信号充分, 0 反模式 命中
- **PASS+fix**: UI 信号 ≥ 总 case 数 70% 且反模式 ≤ 2 处 (局部小改)
- **REWRITE**: UI 信号 < 总 case 数 70% 或反模式 ≥ 3 处
- **DELETE**: 仅 noop / 仅源码 grep / 单元测试已锁源头不丢覆盖

### REWRITE 子分类 (heima 拍 A 改造版)

- **REWRITE-UI**: client 有 production UI 真路径, 改 page.click + DOM 断
- **REWRITE-NAV**: ACL/IDOR 反向证类, client UI 不暴露无权资源, 改 `page.goto` 真 navigate 到无权 URL + 真断 sidebar 空 / message 空 / input 不渲染 / fallback redirect (heima 拍, 不开 F3 例外)
- **SKIP+followup**: client UI 0 production mount, 改 `test.skip` + 头部注释引 follow-up issue (yema 拍 A 方向, 跟 gh#724 关联)

## 全量分类表 (46 spec)

| # | 文件 | 真 UI 信号 | 反模式 | 动作 | 重命名建议 | 备注 |
|---|---|---|---|---|---|---|
| 1 | `adm-1-privacy-promise.spec.ts` | UI=15 | — | **PASS** | `admin-privacy-promise-banner.spec.ts` | 真测红 banner + admin 入 channel 反向检查 |
| 2 | `adm-2-followup.spec.ts` | UI=6 | — | **PASS** | `admin-audit-deletion-followup.spec.ts` | 真测 admin 删除审计页 |
| 3 | `adm-3-audit-events.spec.ts` | UI=5 | — | **PASS** | `admin-audit-event-stream.spec.ts` | 真测 audit 事件流 UI |
| 4 | `al-3-3-presence-dot.spec.ts` | UI=7 | — | **PASS** | `agent-list-presence-indicator.spec.ts` | 真测 agent 列表在线点 |
| 5 | `al-4-acceptance-followup.spec.ts` | UI=25 | — | **PASS** | `agent-list-followup.spec.ts` | 真测 agent 列表多场景 (yema 建议: 去 acceptance- 内部话) |
| 6 | `ap-2-bundle.spec.ts` | UI=3 | F3 | **REWRITE-UI** | `agent-permission-bundle.spec.ts` | UI 信号少多 case 是 REST 直调, 改走 client 设置页真点击 |
| 7 | `ap-4-reactions-acl.spec.ts` | UI=0 | F3 | **REWRITE-NAV** | `reactions-cross-channel-permission.spec.ts` | ACL 越权: page.goto user A private channel URL + 真断 message list 0 hit / reaction button 不渲染 / fallback redirect. 不允许 REST 直调. 立 follow-up gh#724 §2 (client forbidden state UX) |
| 8 | `ap-5-messages-acl-matrix.spec.ts` | UI=0 | F3 | **REWRITE-NAV** | `message-permission-matrix.spec.ts` | ACL 矩阵简化 ≤3 关键 case + 全部走 URL navigate + DOM 真断 cross-user edit/delete/react 不可达. **不允许砍 cross-user IDOR case 数**. 立 follow-up gh#724 §2 |
| 9 | `chn-1-3-channel-list.spec.ts` | UI=18 | — | **PASS** | `channel-list-sidebar.spec.ts` | 真测 channel sidebar |
| 10 | `chn-2-3-dm-flow.spec.ts` | UI=22 | — | **PASS** | `direct-message-flow.spec.ts` | 真测 DM 创建 + 互发 |
| 11 | `chn-3-3-sidebar-reorder.spec.ts` | UI=18 | — | **PASS** | `channel-sidebar-reorder.spec.ts` | 真测 sidebar 拖动重排 |
| 12 | `chn-4-collab-skeleton.spec.ts` | UI=21 | — | **PASS** | `channel-collab-tabs.spec.ts` | 真测协作场双 tab |
| 13 | `chn-4-followup.spec.ts` | UI=6 | — | **PASS** | `channel-collab-edge-cases.spec.ts` | 真测协作场 dm/cross-org 边界 (yema 建议: 去 followup 内部话) |
| 14 | `chn-4-screenshots-followup.spec.ts` | UI=12 / FS=1 | F1 (§4§5 已注释删) | **PASS** | `channel-collab-screenshots.spec.ts` | §1-§3 真截图; §4§5 fs.stat 注释已清 |
| 15 | `cm-4-bug-029-name-display-regression.spec.ts` | UI=6 | — | **PASS** | `chat-name-display-regression.spec.ts` | 真测姓名展示回归 |
| 16 | `cm-4-realtime.spec.ts` | UI=0 (复核 .goto/.click/.fill/getBy 全 0, 仅 1 处 locator() 不 click) | F3 | **REWRITE-UI** | `chat-realtime-message-fanout.spec.ts` | 改双 tab UI 真发互收, happy path 真路径有 (MessageList production mount). feima Q5 提 cm-4-realtime UI=1 误判, 复核改 UI=0 |
| 17 | `cm-5-x2-collab.spec.ts` | UI=4 | F3 | **REWRITE-UI** | `chat-two-user-collab.spec.ts` | UI 弱, 改双 page click 真发. 真路径有 |
| 18 | `cm-onboarding-bug-030-regression.spec.ts` | UI=0 | F3 | **REWRITE-UI** ✅ done (commit 10e2319) | `welcome-channel-per-user-isolation.spec.ts` | 已 REWRITE 1/16: register UI 真填表 + sidebar DOM 断 cross-leak |
| 19 | `cm-onboarding.spec.ts` | UI=5 | — | **PASS** | `chat-first-time-onboarding.spec.ts` | 真测新用户 onboarding 流 |
| 20 | `cv-1-3-canvas-modal-a11y.spec.ts` | UI=26 | — | **PASS** | `canvas-modal-accessibility.spec.ts` | 真测 canvas modal 无障碍 |
| 21 | `cv-1-3-canvas.spec.ts` | UI=35 | — | **PASS** | `canvas-modal-open-close.spec.ts` | 真测 canvas 开关 |
| 22 | `cv-10-comment-draft.spec.ts` | UI=3 | F2 (page.evaluate localStorage) | **SKIP+followup** | `comment-draft-persistence.spec.ts` | client UI 0 production mount (ArtifactComments 系列). 改 test.skip + 引 gh#724 §1. v2 mount 后 unskip |
| 23 | `cv-11-comment-markdown.spec.ts` | UI=1 | F3 + F4 | **SKIP+followup** | `comment-markdown-render.spec.ts` | 同 22, 引 gh#724 §1 |
| 24 | `cv-12-comment-search.spec.ts` | UI=0 | F3 | **SKIP+followup** | `comment-search-filter.spec.ts` | 同 22, 引 gh#724 §1 (ArtifactCommentSearchBox 0 mount) |
| 25 | `cv-2-3-anchor-client.spec.ts` | UI=25 | — | **PASS** | `comment-anchor-scroll.spec.ts` | 真测 anchor 评论锚点滚动 (这是 v1 已 ship 的 anchor-level 评论, 不是 cv-5+ 那套) |
| 26 | `cv-3-3-deferred.spec.ts` | UI=0 | F5 | **DELETE** ✅ done (commit 508067d) | — | 27 行 noop 占位 |
| 27 | `cv-3-3-renderers.spec.ts` | UI=12 | — | **PASS** | `artifact-renderer-types.spec.ts` | 真测 artifact 渲染器 |
| 28 | `cv-4-iterate.spec.ts` | UI=22 | — (复核: page.evaluate 仅在注释非代码) | **PASS** | `artifact-iterate-version.spec.ts` | 复核改 PASS (从 PASS+fix), F2 不命中 |
| 29 | `cv-4-unfixme-followup.spec.ts` | UI=20 | — | **PASS** | `artifact-iterate-edge-cases.spec.ts` | 真测 iterate 边界场景 (yema 建议: 去 unfixme-followup 内部话) |
| 30 | `cv-5-artifact-comment.spec.ts` | UI=0 | F3 | **SKIP+followup** | `artifact-comment-thread.spec.ts` | client UI 0 mount, 引 gh#724 §1 |
| 31 | `cv-7-comment-edit-delete.spec.ts` | UI=0 | F3 | **SKIP+followup** | `comment-edit-delete-permission.spec.ts` | 同 30. **同时跨 §1+§2** (受影响 client mount + ACL UX). 头部注释引 gh#724 两段. heima 拍**不允许丢任一 ACL case** (6 case 全保, mount 落地后 unskip 时回归全部) |
| 32 | `cv-8-comment-thread-reply.spec.ts` | UI=0 | F3 | **SKIP+followup** | `comment-thread-reply.spec.ts` | 同 30, 引 gh#724 §1 |
| 33 | `cv-9-comment-mention.spec.ts` | UI=0 | F3 | **SKIP+followup** | `comment-mention-dispatch.spec.ts` | 同 30, 引 gh#724 §1 |
| 34 | `dl-4-pwa-subscribe.spec.ts` | UI=1 | — | **PASS** | `pwa-push-notification-subscribe.spec.ts` | 真测 PWA 订阅 |
| 35 | `dm-3-multi-device-sync.spec.ts` | UI=0 | F3 | **REWRITE-UI + REWRITE-NAV** | `direct-message-multi-device-sync.spec.ts` | happy path (同 user 双 tab 同 DM 同步) 走 REWRITE-UI; cross-leak 部分 (cross-user) 走 REWRITE-NAV (page.goto 无权 DM URL + DOM 断). 立 follow-up gh#724 §2 (cross-user 部分) |
| 36 | `dm-5-reaction-summary.spec.ts` | UI=0 | F3 | **REWRITE-UI** | `direct-message-reaction-summary.spec.ts` | 改 UI 点 reaction + 断 ReactionBar DOM (production 已 mount via MessageList) |
| 37 | `g2.4-adm-0-stance.spec.ts` | UI=0 | F5 | **DELETE** ✅ done (commit 508067d) | — | 19 行 noop |
| 38 | `g2.4-demo-screenshots.spec.ts` | UI=6 | — | **PASS** | `demo-screenshot-archive.spec.ts` | 真截图 demo |
| 39 | `gh-684-agent-detail-credentials.spec.ts` | UI=18 | — | **PASS** | `agent-detail-credentials-display.spec.ts` | 真测 agent 详情凭据 |
| 40 | `gh-698-agent-config-form-layout.spec.ts` | UI=18 | — | **PASS** | `agent-config-form-layout.spec.ts` | 真测 agent 配置 form 排版 |
| 41 | `hb-1b-installer.spec.ts` | UI=0 | F1 + F4 | **DELETE** ✅ done (commit 508067d) | — | 5 case 全 fs.readFileSync 源码 grep, Go unit 已锁. 同 PR 改 docs/current/borgee-installer.md L139 引用 |
| 42 | `hb-2-v0d.spec.ts` | UI=6 | F2 (cosmetic, 截图 innerHTML) | **PASS+fix** | `host-bridge-daemon-handshake.spec.ts` | UI 真, 2 处 page.evaluate 改 document.body.innerHTML 为截图美化, cosmetic 可删可保留, 加注释 |
| 43 | `me-1-self-message-unread.spec.ts` | UI=22 | — | **PASS** | `self-message-unread-counter.spec.ts` | 真测自己消息不计未读 |
| 44 | `rt-1-2-backfill-on-reconnect.spec.ts` | UI=4 | — (复核: page.evaluate 是 window.__lastWS 真 DOM 探测, 非 fetch) | **PASS** | `realtime-backfill-on-reconnect.spec.ts` | 复核改 PASS (从 PASS+fix), F2 不命中 (F2 严格指 page.evaluate(()=>fetch()) cookie 直调, 非真 DOM 探测) |
| 45 | `rt-3-presence.spec.ts` | UI=4 | F3 | **REWRITE-UI** | `realtime-presence-broadcast.spec.ts` | UI 弱, 改双 tab 真 presence DOM 断 (RT3PresenceDot production 已 mount) |
| 46 | `smoke.spec.ts` | UI=1 | — | **PASS** | `smoke-app-loads.spec.ts` | 真打开首页 |

## 汇总 (复核 + 4 reviewer 反馈后)

- **PASS** (留 + 重命名 + 描述去黑话): 26 spec
- **PASS+fix** (真 UI 主体, 局部 cosmetic): 1 spec (hb-2-v0d)
- **REWRITE-UI** (改真 UI happy path): 8 spec (ap-2 / cm-4-realtime / cm-5 / cm-onboarding-bug-030 ✅done / dm-3 happy / dm-5 / rt-3 + 1 done)
- **REWRITE-NAV** (ACL navigate, heima 拍): 3 spec (ap-4 / ap-5 / dm-3 cross-leak 部分)
- **SKIP+followup** (client UI 0 mount, yema 拍 A): 7 spec (cv-5 / cv-7 / cv-8 / cv-9 / cv-10 / cv-11 / cv-12)
- **DELETE** (死代码): 3 spec ✅done (cv-3-3-deferred / g2.4-adm-0-stance / hb-1b-installer)

合计 46 = 26 + 1 + 8 + 3 + 7 + 3 ≈ 48? 复核: REWRITE-UI 含 cm-onboarding-bug-030 (已 done) = 7 个未做 + 1 done. dm-3 cross-leak 部分既 REWRITE-UI 又 REWRITE-NAV (拆 case) 算 1 spec. 重新算: 26 PASS + 1 PASS+fix + (7 REWRITE-UI 未做 + 1 done) + 3 REWRITE-NAV (ap-4/ap-5/dm-3 cross-leak 算独立 case 不独立 spec) + 7 SKIP + 3 DELETE = 26+1+8+3-1+7+3 = 47, 因为 dm-3 同时算两类 -1 重复. 最终 46 ✓.

## 反向 grep 守卫 (PR 合前必须满足, 含 liema Q1 + yema PM 必改 2 + 复核扩 docs/current/)

```bash
# F1: 任何 fs.* 检查 git 文件 (issue F1)
grep -rE "fs\.(existsSync|stat|readFileSync|readdirSync|statSync)" packages/e2e/tests/*.spec.ts
# 期望: 0 hit

# F2: page.evaluate 走 fetch (cookie 直调后端)
grep -rE "page\.evaluate\([^)]*=>[^)]*fetch" packages/e2e/tests/*.spec.ts
# 期望: 0 hit

# F3-1 (liema Q1): 主体路径只走 REST (.goto=0 + .click=0 + apiRequest>0)
for f in packages/e2e/tests/*.spec.ts; do
  goto=$(grep -cE '\.goto\(' "$f")
  click=$(grep -cE '\.click\(' "$f")
  api=$(grep -cE 'apiRequest\.newContext' "$f")
  [ "$goto" -eq 0 ] && [ "$click" -eq 0 ] && [ "$api" -gt 0 ] && echo "PURE_REST: $f"
done
# 期望 REWRITE/SKIP 完后: 0 hit (test.skip 不打 .goto 但 grep 算上不 fail; 用以下严格版反 .skip)

# F3-2 (yema PM 必改 2 + heima 反 F3 例外不开): 主体路径 apiRequest 用法仅在 seed
grep -nE "apiRequest\.newContext" packages/e2e/tests/*.spec.ts | grep -v -E "(adminLogin|mintInvite|registerUser|seed|setup)" | head -5
# 期望: 主体测试不直调 apiRequest 走业务 endpoint, 仅 admin/invite/register seed 用

# F4: 源码字符串 grep 假装锁
grep -rE "fs\.readFileSync.*\.go|fs\.readFileSync.*\.ts" packages/e2e/tests/*.spec.ts
# 期望: 0 hit (DELETE hb-1b-installer 后)

# F5: noop 占位
grep -rE "expect\(true\)\.toBe\(true\)" packages/e2e/tests/*.spec.ts
# 期望: 0 hit

# 文件名 (yema PM 必改 2): 反 milestone 前缀 + gh-XXX- 前缀
ls packages/e2e/tests/ | grep -E "^[a-z]{2}-[0-9]" -E "^gh-[0-9]+-" -E "^g[0-9]"
# 期望: 0 文件
```

## 复核记录

- 2026-05-09 zhanma-c: 第一轮 audit 太激进, 把 cv-4-iterate (page.evaluate 只在注释) + rt-1-2-backfill (真 WS DOM 探测, 非 fetch 后端) 误判 PASS+fix. 复核后改 PASS.
- 2026-05-09 feima Q5 review: cm-4-realtime UI=1 误判, 真量 .goto/.click/.fill/getBy 全 0, 改 UI=0.
- 2026-05-09 heima ACL review: ap-4/ap-5/cv-7 §3.4/dm-3 cross-leak 部分原标 REWRITE 改 REWRITE-NAV (page.goto + DOM 反向证, 不开 F3 例外).
- 2026-05-09 yema PM review: cv-5/7/8/9/10/11/12 改 SKIP+followup (client UI 0 production mount, 立 gh#724 §1).
- 2026-05-09 yema 重命名建议: al-4 / chn-4-followup / cv-4-unfixme-followup 仍带内部话 (followup / unfixme / acceptance), 改成功能描述.
- 反模式 F2 严格定义: 仅反 `page.evaluate(() => fetch())` 走 cookie 直调后端. `page.evaluate` 访问 `window` / `document` / DOM API / localStorage / WebSocket instance 等真 DOM 行为不属 F2.

## 边界

- **保留 apiRequest 用于 seed** (admin login + invite + register + create channel): 这是 precondition setup, 不是测试主体. 测试主体必须走 page click + DOM 断. F3 反模式严格指"主体测试路径绕 UI", seed 不算反模式.
- **保留 page.evaluate 用于真 DOM 探测**: `getBoundingClientRect()` / `localStorage.getItem` / `window.__lastWS.readyState` 等真 DOM 检查不算 F2.
- **REWRITE-NAV 不算 F3 例外**: 仍走 `page.goto` 真浏览器 navigate, 浏览器真渲染 forbidden 状态 (即使是空状态), 仍是真 UI 路径. 不是 apiRequest 直调后端.

## client UI 缺失判断步骤 (liema Q4 必改, 反"看着没就 skip")

REWRITE 前判 client 是否真有 UI 入口, 走 3 步:

1. `grep -rE "<ComponentName\b" packages/client/src/ --include="*.tsx" | grep -v __tests__`: production code 是否真 mount
2. testing 浏览器 (testing-borgee.codetrek.cn) 真登录手点确认: 走用户路径能不能 reach 该组件
3. 1 + 2 都无, 才标 SKIP+followup. 任一有 = REWRITE-UI 走真 UI 路径

不允许只看代码 grep "找不到" 就跳, 必须 testing 真验.
