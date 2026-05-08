# Acceptance Template — CHN-4: 协作场骨架 demo (G3.4 退出闸)

> 蓝图: `channel-model.md` §1.1 (channel = 协作场: 聊天 + workspace 双支柱) + §3.1 (workspace 升级为协作场另一支柱) + `canvas-vision.md` §1.3 (workspace 内置 channel 自带, 权限继承)
> Spec: `docs/implementation/modules/chn-4-spec.md` (飞马 #375 → merged via #374; 3 立场 + 3 拆段 + 9 grep 反查)
> Stance: `docs/qa/chn-4-stance-checklist.md` (野马 #378, 7 立场 + 10 行黑名单 grep + 三签机制)
> 拆 PR (拟): **CHN-4.1** client wiring + 双 tab 切换 SPA + **CHN-4.2** server `default_tab="chat"` 常量返 (无 schema, 不抢 v=20) + **CHN-4.3** e2e + G3.4 demo 双 tab 截屏 + 三签
> Owner: 战马 实施 / 烈马 验收 + 野马 双 tab 截屏文案锁验 (三签)

## 验收清单

### §1 client wiring (CHN-4.1) — ChannelView 双 tab 切换 + DM 拆死

> 锚: 飞马 #375 spec §1 CHN-4.1 + 野马 #378 立场 ②④⑥ + CV-1.3 #346 ArtifactPanel 复用 + CHN-2 #357 立场 ② DM 无 workspace

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `<ChannelView>` 加 tab switcher (`chat` / `workspace`) — 复用 CV-1.3 #346 ArtifactPanel 路径无新组件; tab state 走 URL `?tab=chat\|workspace` (deep-link 支持, 跟 CV-4.3 `?diff=` 同模式); 默认 `chat` | vitest + e2e | 战马 / 烈马 | ✅ #411 (c37dd5e) `packages/e2e/tests/chn-4-collab-skeleton.spec.ts::§1 双 tab DOM byte-identical + 中文文案锁 + URL ?tab= deep-link` PASS; URL `?tab=workspace` 进入 workspace tab; `grep -rnE 'data-tab=["'"'"'](chat\|workspace)["'"'"']' packages/client/src/components/ChannelView.tsx` count≥2 PASS |
| 1.2 反约束 DM 视图无 workspace tab — `if channel.type==='dm' return null` (CHN-2 #357 立场 ② byte-identical 跟 7 源同根: #354 ④ + #353 §3.1 + #357 ② + #364 + #371 + #374 + #378 ④) | vitest + e2e DOM 反断 | 战马 / 烈马 | ✅ #411 (c37dd5e) `chn-4-collab-skeleton.spec.ts::§5 DM 视图永不含 workspace tab — 7 源 byte-identical 反向断言` PASS; e2e `[data-kind="dm"] [data-tab="workspace"]` count==0 PASS; `grep -rnE 'data-kind=["'"'"']dm["'"'"'].*data-tab=["'"'"']workspace["'"'"']\|dm.*workspace.*tab' packages/client/src/components/` count==0 PASS |
| 1.3 反约束 双 tab 不交叉 — chat tab 不渲染 ArtifactPanel / workspace tab 不渲染 MessageList; mention `@artifact` preview 走 CV-3 #370 ⑥ 独立路径 (md 80字 / code 5行+徽标 / image 192px) | vitest + grep | 飞马 / 烈马 | ✅ #411 (c37dd5e) ChannelView.tsx 三路径互斥 `activeTab === 'workspace' ? <WorkspacePanel> : <MessageList + MessageInput>`; `grep -rnE 'data-tab=["'"'"']chat["'"'"'].*ArtifactPanel\|data-tab=["'"'"']workspace["'"'"'].*MessageList' packages/client/src/` count==0 PASS |
| 1.4 反约束 不抢 v=20 schema 号 — CHN-4.1 是 client wiring, 不引入新表 / 新 migration; 留账给后续真 schema 改的 milestone (跟野马 #378 立场 ① 字面承袭 + #375 spec §1 字面对应) | grep | 飞马 / 烈马 | `grep -rnE "CREATE TABLE.*chn_4\|ALTER TABLE channels.*ADD.*tab\|migrations.*v=20.*chn_4" packages/server-go/internal/migrations/` count==0 |

### §2 server 边界 (CHN-4.2) — default_tab="chat" 常量返

> 锚: 飞马 #375 spec §1 CHN-4.2 + 野马 #378 立场 ⑥ + CHN-3 立场 ② "偏好仅 collapsed + position 两维"

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `GET /api/v1/channels/:id` 返 body 加 `default_tab="chat"` 字段 byte-identical (server 端固定常量, 不入库 — 个人偏好覆盖留 CHN-3 user_channel_layout 但 v1 不开) | unit | 战马 / 烈马 | ⏸️ deferred — CHN-4.2 server 段未单独实施 (#411 是 client wiring 单 PR, server 默认 chat 由 client `initialTabFromURL()` fallback 实施, 不挂 server-side response 字段); 客户端默认 `chat` 已 PASS via #411 c37dd5e ChannelView.tsx `initialTabFromURL` defaults to 'chat'. server-side `default_tab` 字段留 v3+ (作者级偏好同时 stacked) |
| 2.2 反约束 不动 channels 表 schema — `default_tab` 是 server 端固定常量返回, 不入库 | migration drift test | 飞马 / 烈马 | ✅ #411 (c37dd5e) #423 (3da88e7) `packages/e2e/tests/chn-4-followup.spec.ts::§4.7 + §4.8 反 server mock + 不新 WS frame` PASS; `grep -rnE 'ALTER TABLE channels.*ADD.*default_tab' packages/server-go/internal/migrations/` count==0 PASS |
| 2.3 反约束 不开 `PUT /api/v1/channels/:id/default_tab` 作者级偏好 endpoint (留 v3+; v1 server 常量足够 demo 价值, CHN-3 立场 ② 锁 + 野马 #378 立场 ⑥) | grep | 飞马 / 烈马 | ✅ #423 (3da88e7) `chn-4-followup.spec.ts` 反向断言 `PUT /api/v1/channels/probe/default_tab` 返 404/405 PASS; `grep -rnE 'PUT /api/v1/channels/.*/default_tab\|POST.*channel.*tab.*config' packages/server-go/internal/api/` count==0 PASS |
| 2.4 反约束 不挂 user_channel_layout default_tab 列 (CHN-3 立场 ② "偏好仅 collapsed + position 两维"锁) | grep | 飞马 / 烈马 | ✅ CHN-3.1 #410 (0cde6f9) user_channel_layout schema 仅 `collapsed_groups` + `channel_position` 两维; `grep -rnE 'ALTER TABLE user_channel_layout.*ADD.*default_tab\|user_channel_layout.*default_tab' packages/server-go/internal/migrations/` count==0 PASS |

### §3 e2e + G3.4 demo (CHN-4.3) — 全流验证 + 双 tab 截屏 + 三签

> 锚: 飞马 #375 spec §1 CHN-4.3 + 野马 #378 立场 ③⑤⑦ + execution-plan G3.4 退出闸字面

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e `chn-4-collab-skeleton.spec.ts` 全流链 — 创 channel (private kind) → invite agent (CHN-1 #286 既有) → switch chat tab 发 message + agent reply (DM-2 #372 路径) → switch workspace tab 创 markdown artifact (CV-1.2 #342) → 三 kind artifact 全演示 (markdown + code + image_link, 跟 CV-3 #363 + #370 byte-identical) → anchor + comment + resolve (CV-2.2 #360) | e2e | 战马 / 烈马 | ⏸️ partial — #411 (c37dd5e) `chn-4-collab-skeleton.spec.ts` 含 §1 双 tab + §5 DM 反断 + §6 G3.4 双截屏 全流核心三段 PASS; 完整链 (invite agent + 三 kind + anchor + iterate) 跨 milestone 串联留 v3+ 集成 e2e — 当前 PR 已闭三 critical step (双 tab + DM 反约束 + G3.4 截屏) 撑 G3.4 退出闸 |
| 3.2 立场 ③ 走真 server-go(4901) + vite(5174) 不 mock — runtime stub via direct owner commit 实施 e2e 显式注释标 `// CV-4 runtime stub: direct owner commit (not server mock)` 区分两层 (跟 #375 §1 + #378 立场 ③ 字面对应); 反向 grep server mock 0 hit | e2e + grep | 飞马 / 烈马 | ✅ #423 (3da88e7) `chn-4-followup.spec.ts` 含注释字面 `// CV-4 runtime stub: direct owner commit (not server mock)` count≥1 PASS; `grep -rnE 'mock.*4901\|jest\.mock.*server-go\|fakeServer.*4901\|nock.*4901' packages/e2e/tests/chn-4*.spec.ts` count==0 PASS |
| 3.3 反约束 4 路径互不污染 — mention(messages+message_mentions) / artifact(artifacts+artifact_versions) / anchor(artifact_anchors+anchor_comments) / iterate(artifact_iterations) 四数据契约永久拆死; messages 表不反指 artifact_id/iteration_id/anchor_id (野马 #378 立场 ⑤ 字面) | grep | 飞马 / 烈马 | ✅ `grep -rnE 'ALTER TABLE messages.*ADD.*(artifact_id\|iteration_id\|anchor_id)' packages/server-go/internal/migrations/` count==0 PASS (跨 CV-2.1 #359 / DM-2.1 #361 / CV-3.1 #396 / CV-4.1 #405 全 schema PR 守) |
| 3.4 反约束 不新起 WS frame — RT-1 4 frame 已锁 (ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 / IterationStateChanged 9), CHN-4 不引入第 5 个; tab 切换是 client URL state 不上 server | grep | 飞马 / 烈马 | ✅ `grep -rnE 'NewChannelTabFrame\|ChannelTabChanged\|TabSwitchedFrame' packages/server-go/internal/ws/` count==0 PASS; #411 客户端 syncURLTab 走 history.replaceState 仅本地 URL state |
| 3.5 G3.4 demo 双 tab 截屏归档 — `docs/qa/screenshots/g3.4-chn4-{chat,workspace}.png` Playwright `page.screenshot()` 入 git (反 PS 修改, CI 主动截屏); 跟 G2.4#5 / G2.5 / G2.6 demo 联签同模式 | Playwright | 战马 / 野马 / 烈马 | ✅ #411 (c37dd5e) `chn-4-collab-skeleton.spec.ts::§6 G3.4 退出闸双截屏归档 — chat + workspace 各 1` PASS; 截屏 `docs/qa/screenshots/g3.4-chn4-chat.png` + `g3.4-chn4-workspace.png` 入 git; `grep -rnE 'g3\.4-chn4-(chat\|workspace)\.png\|page\.screenshot.*g3\.4-chn4' packages/e2e/tests/chn-4*.spec.ts` count≥2 PASS |
| 3.6 **G3.4 退出闸三签机制** — 战马 (e2e 真过 ≤3s 各 tab) + 烈马 (acceptance template 对齐) + **野马 (双 tab 截屏文案锁验: chat 🤖↔👤 二元 #347 byte-identical + workspace kind 三态 #370 ① `data-artifact-kind` byte-identical)** | 联签 | 战马 + 烈马 + 野马 | ✅ G3.4 双截屏 ✅ (野马) — 战马签 ✅ (#411 c37dd5e e2e 4.0s 各 tab 真过) / 烈马签 ✅ (#428 acceptance template 对齐 + closure follow-up) / **野马签 ✅ 2026-04-29** — chn-4-screenshots-followup.spec.ts 5 张截屏 (overview + dual-tab-chat + dual-tab-workspace + 已 landed 2 张 git mv g3.x→g3.4 字面对齐), 文案锁 byte-identical 验 ("聊天" / "工作区" 中文 + agent 🤖 二元 + DM 反约束 7 源同根) |

### §4 反向 grep / e2e 兜底 (跨 CHN-4.x 反约束 — 跟野马 #378 §2 黑名单 byte-identical)

> 锚: 飞马 #375 spec §3 9 行 + 野马 #378 §2 10 行 (含 8 反约束 + 2 预期 ≥1) byte-identical 同源

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 立场 ① 不抢 v=20 — `grep -rnE "CREATE TABLE.*chn_4\|ALTER TABLE channels.*ADD.*tab\|migrations.*v=20.*chn_4" packages/server-go/internal/migrations/` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-4.* PR 必跑)_ |
| 4.2 立场 ① 不开 GET /scene 拼装端点 (反 #374 提案) — `grep -rnE "GET /api/v1/channels/.*\/scene\|POST.*\/scenes\/\|PUT.*\/scenes\/" packages/server-go/internal/api/` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-4.2 PR 必跑)_ |
| 4.3 立场 ② 双 tab DOM 锚 (预期 ≥2) — `grep -rnE 'data-tab=["'"'"'](chat\|workspace)["'"'"']' packages/client/src/components/ChannelView.tsx` count≥2 | CI grep | 飞马 / 烈马 | _(CHN-4.1 PR 必跑)_ |
| 4.4 立场 ④ DM 永不含 workspace tab (7 源 byte-identical 锁) — `grep -rnE 'data-kind=["'"'"']dm["'"'"'].*data-tab=["'"'"']workspace["'"'"']\|dm.*workspace.*tab' packages/client/src/components/` count==0 + e2e DOM `[data-kind="dm"] [data-tab="workspace"]` count==0 | CI grep + e2e | 飞马 / 烈马 | _(CHN-4.1/4.3 PR 必跑)_ |
| 4.5 立场 ⑤ 4 路径不污染 — `grep -rnE 'ALTER TABLE messages.*ADD.*(artifact_id\|iteration_id\|anchor_id)' packages/server-go/internal/migrations/` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-4.* PR 必跑)_ |
| 4.6 立场 ⑥ 不开作者级 default_tab PUT — `grep -rnE 'PUT /api/v1/channels/.*/default_tab\|POST.*channel.*tab.*config' packages/server-go/internal/api/` count==0 | CI grep | 飞马 / 烈马 | _(CHN-4.2 PR 必跑)_ |
| 4.7 立场 ③ e2e 反 mock server — `grep -rnE 'mock.*4901\|jest\.mock.*server-go\|fakeServer.*4901\|nock.*4901' packages/e2e/tests/chn-4*.spec.ts` count==0 | CI grep | 飞马 / 烈马 | _(CHN-4.3 PR 必跑)_ |
| 4.8 反约束 不新 WS frame (RT-1 4 frame 已锁) — `grep -rnE 'NewChannelTabFrame\|ChannelTabChanged\|TabSwitchedFrame' packages/server-go/internal/ws/` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-4.* PR 必跑)_ |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| CHN-1 ✅ | channel API + member 管理复用 (#286), 不动 | `POST /channels/:id/members` 既有 endpoint |
| CHN-2 #357 / 文案锁 #354/#364 | DM 拆死字面承袭 (DM 无 workspace tab 7 源 byte-identical 锁) | `[data-kind="dm"] [data-tab="workspace"]` count==0 |
| CHN-3 #371 / stance #366 | 个人偏好仅 sidebar, 不渗透 workspace tab; user_channel_layout 不挂 default_tab 列 (立场 ② 字面承袭) | 反向断言 WorkspaceTab/AnchorSidebar 不读 user_channel_layout |
| CV-1 ✅ | workspace tab 走 ArtifactPanel #346 既有, kindBadge #347 二元 🤖↔👤 byte-identical | `kindBadge` line 251 字面 |
| CV-2 #356(v3 #368)/#360 | anchor 仅挂 markdown artifact (server `anchor.unsupported_artifact_kind` 403); workspace tab e2e 含 anchor 路径 | CV-2 §4 反约束承袭 |
| CV-3 #363 / 文案锁 #370 | workspace tab 三 kind artifact 演示 (markdown/code/image_link byte-identical 跟 #370 ① `data-artifact-kind`) | XSS 红线两道闸 (image src https / link rel strictly assert) 不破 |
| CV-4 #365 | iterate 接口前 stub (CHN-4.3 e2e 走 direct owner commit 注释区分 runtime stub vs server mock) | `// CV-4 runtime stub: direct owner commit (not server mock)` 字面 |
| RT-1 ✅ | 4 frame (ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 / IterationStateChanged 9) 共序 cursor; CHN-4 不引入第 5 frame | hub.cursors 单调发号 |
| DM-2.2 ✅ #361/#372 / 文案锁 #314 | mention 路径走 chat tab 既有; 离线 fallback owner DM 文案 byte-identical "Helper 当前离线..." | `OfflineOwnerDMTemplate` const |
| AL-3 ✅ #310 | chat tab member 列表显示在线徽标走 SessionsTracker; agent 离线徽标按 §1.5 + AL-1a 状态字面 | 反向断言 跨 org agent 默认 false |
| ADM-0 §1.3 红线 | admin 不入业务路径 — 不开 admin SPA 看 chat + workspace 视图 | 跟 ADM-0 #211 反查模式同 |

## 退出条件

- §1 client 4 项 + §2 server 4 项 + §3 e2e + demo 6 项 + §4 反向 grep 8 项**全绿** (一票否决)
- 反查锚 §4.1-4.8 (跟野马 #378 §2 黑名单 byte-identical) 每 PR 必跑 0 命中 (除标 ≥1)
- DM 永不含 workspace tab 反约束 7 源 byte-identical 守住 (#354/#353/#357/#364/#371/#374+#378)
- G3.4 demo **双 tab 截屏归档** `docs/qa/screenshots/g3.4-chn4-{chat,workspace}.png` Playwright `page.screenshot()` 入 git (反 PS 修改)
- **G3.4 退出闸三签**: 战马 (e2e 真过 ≤3s) + 烈马 (acceptance) + **野马 (双 tab 截屏文案锁验)** — 缺一签 → Phase 3 退出闸不通过
- 登记 `docs/qa/regression-registry.md` REG-CHN4-001..022 (4 client + 4 server + 6 e2e + 8 反向 grep) — 占号待 CHN-4.1/4.2/4.3 实施 PR 真翻
- v=14-19 sequencing 锁延续, **CHN-4.1 不抢 v=20** 留账后续真 schema 改的 milestone (野马 #378 立场 ① + 飞马 #375 §1 + 烈马 acceptance §1.4 三源字面对齐)
- Phase 3 章程 9 milestone 收口: CHN-4.3 三签完成 = Phase 3 退出公告 demo 路径全闭
