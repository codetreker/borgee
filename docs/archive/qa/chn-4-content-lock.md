# CHN-4 协作场骨架文案锁 (野马 G3.4 demo + Phase 3 退出闸双 tab 截屏字面验)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CHN-4 client UI 实施前锁双 tab (chat + workspace) 视觉文案 + DOM 字面 + DM 视图反约束 — **Phase 3 章程退出闸三签机制野马签锚** (战马 e2e 真过 + 烈马 acceptance + 野马双 tab 截屏文案锁验). 跟 AL-3 #305 / DM-2 #314 / AL-4 #321 / CHN-2 #354 / CV-2 #355 / CV-3 #370 / CV-4 #380 同模式, **集成 demo 不臆想新字面, 跟既有 7 milestone 文案锁 byte-identical 同源**.
> **关联**: 飞马 spec brief #375 §0 ①②③ (集成 e2e 反再造 / 双 tab byte-identical / e2e 走真不 mock); 野马 stance #378 7 立场 (DM 永不含 workspace 7 源 byte-identical); CHN-1 #288 ChannelView/ChannelGroupComponent + CV-1 #346 ArtifactPanel + #347 kindBadge + #364 data-kind="dm" + #354 ⑤ 私信字面 + #357 立场 ③ + #366/#371 立场 ④ + #374 立场 ② 同源.
> **#338 cross-grep 反模式遵守**: CHN-4 是集成 demo, 既有 ChannelView (#288) + Sidebar (#288) + ArtifactPanel (#346) + kindBadge (#347) + 私信字面 (#354) + data-kind (#364) + 各 milestone 文案锁字面池**已全部稳定**, 本锁直接 byte-identical 引用既有, 不重新定义.

---

## 1. 6 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **双 tab 切换器** (ChannelView 顶部, channel.type IN ('private','public') 才渲染) | DOM: `<div class="channel-tabs"><button data-tab="chat">聊天</button><button data-tab="workspace">工作区</button></div>` byte-identical (`"聊天"` / `"工作区"` 中文 2 字面锁); URL `?tab=chat\|workspace` deep-link byte-identical 跟 #375 §1 + CV-4 #380 ⑤ deep-link 同精神 | ❌ 不准 "Chat" / "Workspace" / "消息" / "画布" / "Canvas" 同义词 (中文 byte-identical 锁); ❌ 不准在 DM 视图渲染双 tab (跟 stance #378 立场 ④ "DM 永不含 workspace 7 源" byte-identical, e2e DOM `[data-kind="dm"] [data-tab="workspace"]` count==0); ❌ 不准 tab 状态走 user_channel_layout 表 (是 server 常量 + URL state, 跟 stance #378 ⑥ 同源) |
| ② | **chat tab agent 🤖 角标** (member 列表 / sender) | DOM 二元锁: `<span class="member-kind-badge" data-kind="{agent\|user}">{🤖\|👤}</span>` byte-identical 跟 **CV-1 #347 line 251 kindBadge + CV-2 #355 ④ + DM-2 #314 ② + CV-4 #380 ④ 四源 byte-identical** (改 = 改五处: #347 + #355 + #314 + #380 + 此锁) | ❌ 不准 "Bot" / "AI" / "Assistant" / "机器人" 同义词 (跟 #355 ④ + #380 ④ 反向 grep 同源); ❌ 不准漏 agent 角标 (二元锁视觉永久); ❌ 不准 chat tab member 列表混 admin 行 (admin 不入 channel ADM-0 §1.3 红线) |
| ③ | **workspace tab artifact list** (channel scoped, kind 三态) | DOM: `<div class="artifact-list"><a data-artifact-kind="markdown\|code\|image_link" data-artifact-id="{id}">{title}</a></div>` byte-identical 跟 **CV-3 #370 ① + CV-1 #346 + CV-2 #358 §3.4 三源 byte-identical**; agent 创 artifact 行尾 kindBadge 🤖 跟 ② 同源; iterate 触发按钮 🔄 跟 CV-4 #380 ① byte-identical (owner-only DOM omit); rollback / 锚点入口 跟 CV-1 #347 line 254 + CV-2 #355 ① 既有路径同源 | ❌ 不准 workspace tab 渲染 message body (双 tab 不交叉 跟 stance #378 立场 ② + #375 spec §0 ② 同源); ❌ 不准 DM 视图加 workspace tab (7 源 byte-identical 永久锁); ❌ 不准 artifact list 渲染 anchor 行 (CV-2 anchor 仅 markdown artifact 内挂, 跟 #363 server 端 `anchor.unsupported_artifact_kind` 同源); ❌ 不准 admin SPA 看 workspace artifact list (admin 不入业务路径) |
| ④ | **default_tab server 常量 = "chat"** (新建 channel 默认进 chat) | server `GET /channels/:id` body 字段 `"default_tab": "chat"` byte-identical (跟 #375 spec §1 CHN-4.2 + stance #378 ⑥ 同源); client 进 channel 无 URL `?tab=` 时默认 navigate 到 chat tab; URL 显式 `?tab=workspace` 优先 server 常量 (deep-link 用户主动切) | ❌ 不准 server 常量是其他值 (如 "workspace" — 章程 §1.1 字面 "channel = 协作场 但 chat 是双支柱第一支柱"); ❌ 不准开 `PUT /channels/:id/default_tab` 作者级偏好 endpoint (留 v3+, 跟 stance #378 ⑥ 反约束 + #375 反约束同源); ❌ 不准把 default_tab 写入 user_channel_layout (跟 CHN-3 立场 ② "偏好仅 collapsed + position" 同源) |
| ⑤ | **DM 视图永不含 workspace tab** (7 源 byte-identical 锁, Phase 3 全 milestone 最稳反约束) | DM 视图 (channel.type==='dm') DOM 不渲染 `data-tab="workspace"` 元素; 仅渲染 chat 流; 反约束 e2e DOM `[data-kind="dm"] [data-tab="workspace"]` count==0 跟 **#354 ④ + #353 §3.1 + #357 立场 ② + #364 patch + #371 + #374 + #375 七源 byte-identical** | ❌ 不准 DM 视图任何路径下出现 workspace tab; ❌ 不准 DM 视图渲染 anchor / iterate / artifact 入口 (跟 stance #378 ④ "DM 是 1v1 私聊不是协作场" 同源); ❌ 不准 conditional render 写错让 workspace tab 在 DM 视图 disabled (必须 omit, defense-in-depth 跟 CV-1 #347 showRollbackBtn 同模式) |
| ⑥ | **G3.4 退出闸双 tab 截屏文案验** (野马签锚) | 截屏归档 byte-identical 路径锁: `docs/qa/screenshots/g3.4-chn4-{chat,workspace}.png` 双张 (Playwright `page.screenshot()`, CI 主动入 git 防伪造); chat 截屏验: `"聊天"` tab active + agent 🤖 行 + 私信不混排 (跟 #354 ① 私信分组 byte-identical); workspace 截屏验: `"工作区"` tab active + artifact list `data-artifact-kind` 三态各 ≥1 + iterate 按钮 🔄 owner 视角可见 | ❌ 不准截屏后期 PS 修改 (CI 主动 page.screenshot 入 git, 防伪造); ❌ 不准截屏漏 agent 🤖 角标 (二元锁视觉验); ❌ 不准截屏 admin 视角 (admin 不入 channel god-mode 字段白名单不含 chat/workspace body); ❌ 缺一签则退出闸不通过 (野马签 = 截屏文案锁验 byte-identical, 跟 G2.4#5/G2.5/G2.6 demo 联签同模式) |

---

## 2. 反向 grep — CHN-4 实施 PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# ① 双 tab DOM byte-identical (预期 ≥2 — chat + workspace 各 1)
grep -nE 'data-tab=["'"'"'](chat|workspace)["'"'"']' packages/client/src/components/ChannelView.tsx | grep -v _test  # 预期 ≥2
# ① 双 tab 中文文案 byte-identical (反 "Chat/Workspace/消息/画布/Canvas" 同义词)
grep -rnE "['\"](Chat|Workspace|消息|画布|Canvas|Tabs)['\"]" packages/client/src/components/ChannelView.tsx 2>/dev/null | grep -v _test
# ② kindBadge 二元锁 (跟 CV-1 #347 line 251 同源, 预期 ≥1)
grep -rnE "data-kind=['\"]agent['\"]" packages/client/src/components/ChannelView.tsx packages/client/src/components/MemberList*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
# ② agent 角标同义词漂移防御
grep -rnE "['\"](Bot|AI|Assistant|机器人)['\"]" packages/client/src/components/ChannelView.tsx packages/client/src/components/MemberList*.tsx 2>/dev/null | grep -v _test
# ③ workspace tab artifact-kind 三态 (预期 ≥3 — 跟 #370 ① 同源)
grep -nE 'data-artifact-kind=["'"'"'](markdown|code|image_link)["'"'"']' packages/client/src/components/Workspace*.tsx 2>/dev/null | grep -v _test  # 预期 ≥3
# ③ workspace tab 不渲染 message (双 tab 不交叉)
grep -rnE 'WorkspaceTab.*MessageList|workspace.*messages.*render' packages/client/src/components/ 2>/dev/null | grep -v _test
# ④ default_tab server 常量字面锁 (预期 ≥1)
grep -rnE "default_tab.*=.*['\"]chat['\"]|defaultTab.*=.*['\"]chat['\"]" packages/server-go/internal/api/ 2>/dev/null | grep -v _test.go  # 预期 ≥1
# ④ 反约束作者级 default_tab PUT endpoint
grep -rnE 'PUT /api/v1/channels/.*/default_tab|POST.*channel.*tab.*config' packages/server-go/internal/api/ 2>/dev/null | grep -v _test.go
# ⑤ DM 视图永不含 workspace tab (7 源 byte-identical, 跟 stance #378 ④ 同源)
grep -nE 'data-kind=["'"'"']dm["'"'"'].*data-tab=["'"'"']workspace["'"'"']|dm.*workspace.*tab' packages/client/src/components/ 2>/dev/null | grep -v _test
# ⑤ DM 视图反 anchor / iterate / artifact 入口
grep -rnE "channel\\.type.*===.*['\"]dm['\"].*\\.(IterateBtn|AnchorBtn|ArtifactPanel)" packages/client/src/components/ 2>/dev/null | grep -v _test
# ⑥ G3.4 双截屏归档路径锁 (预期 ≥2)
grep -rnE 'g3\\.4-chn4-(chat|workspace)\\.png|page\\.screenshot.*g3\\.4-chn4' packages/e2e/tests/chn-4*.spec.ts 2>/dev/null | grep -v _test  # 预期 ≥2
```

---

## 3. 验收挂钩 (CHN-4.x PR 必带)

- ① 双 tab 切换 e2e: ChannelView channel.type IN ('private','public') 视图 DOM `[data-tab="chat"]` + `[data-tab="workspace"]` 各 ≥1 + 中文文案 byte-identical + URL `?tab=` deep-link 切换
- ② chat tab kindBadge e2e: agent 行 `data-kind="agent"` + 🤖 byte-identical 跟 CV-1 #347 line 251 同源单测 + admin 反向断言不在 member 列表
- ③ workspace tab artifact list e2e: `data-artifact-kind` 三态各 ≥1 (跟 CV-3 #370 ① 同源) + iterate 按钮 owner-only DOM omit (跟 CV-4 #380 ① 同源) + 反向断言无 message body 渲染
- ④ default_tab e2e: server `GET /channels/:id` body `"default_tab": "chat"` 字面 + client 无 URL `?tab=` 时默认 chat + 反向断言无 PUT endpoint
- ⑤ DM 视图反向断言 e2e: `[data-kind="dm"] [data-tab="workspace"]` count==0 + `[data-kind="dm"]` 内无 anchor/iterate/artifact 入口 (7 源 byte-identical 锁)
- ⑥ G3.4 退出闸双截屏 e2e: Playwright `page.screenshot({path: 'docs/qa/screenshots/g3.4-chn4-{chat,workspace}.png'})` 主动入 git 各 1 张 + 字面验 (chat = "聊天" tab active + agent 🤖 + workspace = "工作区" tab active + artifact 三态)
- **章程 Phase 3 退出闸三签**: 战马 (e2e ≤3s 真过) + 烈马 (acceptance 模板对齐) + **野马 (双 tab 截屏文案锁验 byte-identical)** — 缺一不通过

---

## 4. 不在范围

- ❌ 双 tab 交叉 (chat 渲染 artifact body / workspace 渲染 message — 跟 stance #378 立场 ② + #375 spec §0 ② 同源)
- ❌ DM 视图加 workspace / anchor / iterate / artifact 入口 (7 源 byte-identical 永久锁)
- ❌ admin SPA 协作场视图 (admin 不入业务路径 ADM-0 §1.3 红线; admin 仅看元数据)
- ❌ 作者级 default_tab 偏好 endpoint (留 v3+, 跟 stance #378 ⑥ + #375 反约束同源)
- ❌ 协作场偏好 (workspace tab 默认开/关 / anchor sidebar 折叠状态等) — CHN-3 仅锁 sidebar, workspace 内偏好留 v3+
- ❌ multi-channel 视图 / channel 切换器 (蓝图 §3.1 v1 不做)
- ❌ 协作场实时活动流 (CM-4 minimal presence 已够, 完整 activity feed 留 v3+)
- ❌ tab 切换 WS push frame (tab 切换是 client URL state, 不上 server, 跟 #375 反约束 + #378 立场 ① 同源 — RT-1 4 frame 已锁不引入第 5 个)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 6 处文案锁 (双 tab 切换器 "聊天"/"工作区" + chat agent 🤖 byte-identical 跟 #347/#355/#314/#380 四源 + workspace artifact-kind 三态 跟 #370/#346/#358 三源 + default_tab server 常量 "chat" + DM 视图永不含 workspace 7 源 byte-identical + G3.4 双截屏文案锁验) + 11 行反向 grep (含 5 预期 ≥1 + 6 反约束) + G3.4 退出闸三签机制 (野马签 = 双 tab 截屏文案锁验 byte-identical). #338 cross-grep 反模式遵守: 集成 demo 不臆想新字面, 跟既有 7 milestone 文案锁字面池 (#288/#346/#347/#354/#364/#370/#380) byte-identical 引用 |
