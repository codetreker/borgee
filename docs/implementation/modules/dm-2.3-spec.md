# DM-2.3 spec brief — client SPA mention 渲染 (DM-2 主线最后一段)

> 飞马 · 2026-04-29 · ≤80 行 spec lock (DM-2.2 #372 server 实施全闭后续作; DM-2 主线收口)
> **蓝图锚**: [`concept-model.md`](../../blueprint/current/concept-model.md) §4 (mention 路由 — 显示 display_name 不漏 raw UUID) + §4.1 (离线 fallback 是 owner 后台事不污染发送方); [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) §1.5 (agent 默认允许参与, 二元 🤖↔👤)
> **关联**: 上游全闭 — DM-2.1 #361 schema v=15 ✅ / DM-2.2 #372 server parser + WS push + offline fallback ✅ (MentionPushedFrame 8 字段 byte-identical 落地); 锚 #312 飞马 DM-2 spec brief (merged 7de76f9) §1 CV-2.3 + #293 烈马 acceptance §3 + #314 野马文案锁 (display_name 渲染字面) + 既有 `lib/markdown.ts:55-58` `<@user_id>` token 渲染同模式 (CV-1 既有路径, DM-2.3 复用扩 mention 候选 + WS 实时刷)
> **章程闸**: G3.4 协作场骨架 demo 收口路径之一 (CHN-4 #374 e2e 链含 mention 流 + 离线 fallback owner DM 截屏)

> ⚠️ 锚说明: client `lib/markdown.ts` 已有 mention render 路径 (CV-1 既有, hljs/markdown 内 `<@user_id>` token); DM-2.3 **不动** 既有 markdown render 路径, 仅加 (a) textarea `@` 候选列表 + (b) MentionPushedFrame WS 实时刷新触发 + (c) raw UUID 防漏 e2e 反向断言; 反约束: 不裂 mention render 路径

## 0. 关键约束 (3 条立场, 蓝图字面 + #312/#293/#314 byte-identical 承袭)

1. **textarea `@` 候选 = channel member 人+agent 同列, 选中回填 `@<user_id>` token 非 display_name** (#312 立场 ① + #314 文案锁): 输入 `@` 触发 `<MentionPicker>` 浮层, 列 channel member (`GET /channels/:id/members` 既有 endpoint) 人 + agent 同列 (agent 行加 🤖 角标 byte-identical 跟 CV-1 #347 立场 ⑥ 二元 🤖↔👤 + #364 `data-kind` 同源); 上下键导航 + 回车选中 → textarea 回填 `@<user_id>` token (非 `@<display_name>`, 防同名歧义跟 #312 立场 ① 字面); **反约束**: 候选列表不含 channel 外用户 / 不含 admin (admin 不入 channel ADM-0 §1.3 红线); textarea 字面存 `@<user_id>` server-side parser regex `@([0-9a-f-]{36})` 命中 (#372 已落)
2. **消息流渲染 token → `<span data-mention-id="...">@{display_name}</span>` 蓝色高亮, raw UUID 仅 attr 不进文本节点** (#293 §3.1 + #314 byte-identical): 复用 `lib/markdown.ts:55-58` 现有 `<@user_id>` token 渲染路径 (CV-1 既有), DM-2 mention token 走同函数; class `mention` byte-identical 跟既有, 蓝色高亮走 CSS (`.mention { color: var(--blue-link); }`); **反约束**: DOM 文本节点 grep raw UUID `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}` count==0 (跟 ADM-0 #211 §1.1 隐私红线同源 + #312 §3 反向 grep 同模式)
3. **MentionPushedFrame WS 实时刷新 + 离线 agent 发送方 UI 无任何提示** (#293 §3.3 反向断言 + #312 立场 ② + 蓝图 §4.1 字面): client `wsClient.ts` switch type='mention_pushed' → 触发 channel 视图 message list 重渲 + (target=本人时) 浏览器 notification (Notification API 复用既有 channel notification 路径); 反约束: **离线 agent 发送方 UI 无任何提示** — 发送方提交 message 后**不感知** target 是否在线 / 是否触发 fallback / 是否 owner 收到 system DM (隐私 + 发送方语境干净); fallback 是 owner 收到 system DM (走既有 messages.type='system' 路径 #372 落地, 跟 DM 流内 system message 同源渲染 byte-identical)

## 1. 拆段实施 (单 PR, 3 文件; **无 schema, 不抢 v 号**)

| 文件 | 范围 |
|---|---|
| `packages/client/src/components/MentionPicker.tsx` (新) | textarea `@` 触发浮层 + 候选列表 (props: `channelId`, `onSelect(userId)`); 列 channel members (`GET /channels/:id/members` 拉, useSWR 缓存); agent 行 `data-kind="agent"` + 🤖 角标; 人行 `data-kind="user"` + 👤 (跟 CV-1 #347 + #364 byte-identical); 上下键导航 + 回车选中 → 回填 `@<user_id>` 到 textarea cursor 位; placeholder `"输入 @ 提到 channel 成员…"` byte-identical (跟 #314 文案锁) |
| `packages/client/src/components/MessageList.tsx` (微调 ~10 行) | message body 渲染走 `lib/markdown.ts:renderMarkdown(body, mentionedUserIds, userMap)` 既有路径 (CV-1 已落); MentionPushedFrame WS push 命中本 channel 时, useSWR mutate `/channels/:id/messages` 重拉 (跟 RT-1 #292 backfill 同模式); target=本人时触发浏览器 notification (复用既有 channel notification API 路径); 反约束: 不另起 mention 专用 message component, 走同 message 渲染路径 |
| `packages/client/src/realtime/wsClient.ts` (微调 ~5 行) | switch case `'mention_pushed'` → dispatch `mentionPushedHandler(frame)` (frame 8 字段 byte-identical 跟 #372 server 端); handler 触发上述 message list mutate + notification; 反约束: 不在 wsClient 解 body / 不在 wsClient 渲染 — 只 dispatch (跟 ArtifactUpdated/AnchorCommentAdded 既有 wsClient 模式同源) |

**owner**: 战马A (CV-2.3 后接) **或** 战马C (DM-2.2 #372 续作) — team-lead 拍

## 2. 与 DM-2.1/2.2 + CV-1 + RT-1 + CHN-1/4 + ADM-0 留账冲突点

- **DM-2.2 #372 server 路径** (核心承袭): MentionPushedFrame 8 字段 byte-identical 跟 #372 anchor 一致 (`type/cursor/message_id/channel_id/sender_id/mention_target_id/body_preview/created_at`); body_preview 80 rune 截断走 `TruncateBodyPreview` 已落 (反约束: client 不重新解析 body_preview, 直接显示)
- **CV-1 既有 `lib/markdown.ts` mention 渲染** (复用核心): 现有 `<@user_id>` 渲染路径已支持 display_name 替换 (line 55-58), DM-2.3 不改 lib/markdown.ts 仅消费; 反约束: 不裂 mention render 路径
- **CHN-1 channel members API** (复用): MentionPicker 拉 `GET /channels/:id/members` 既有 endpoint, 不开新; agent 候选过滤走既有 member.kind 字段 (CHN-1 schema)
- **CHN-2 DM 拆死** (字面承袭): DM 视图 (channel.type='dm') textarea 仍可触发 `<MentionPicker>` (DM 双方都是 channel member 候选列表 2 项), 但 mention DM 对方等于 ping 自己 (无 fallback 因 DM 私密路径); 反约束: DM 内不开 `@<3rd_user_id>` token (server 端 #372 cross-channel reject 400 已落)
- **CHN-4 协作场骨架** (e2e 整合): MentionPicker 在 chat tab textarea 显, workspace tab artifact body 编辑器**不显** (workspace 不混 mention 流, CHN-4 立场 ② 字面); 反约束: workspace tab 内 textarea 不引入 MentionPicker
- **RT-1 cursor 共序** (非冲突): MentionPushedFrame 走 #290 cursor 单调发号 client 端 wsClient.ts switch dispatch
- **ADM-0 god-mode** (字面承袭): admin 不入 channel, MentionPicker `GET /channels/:id/members` 不返 admin (CHN-1 #286 已落); 反约束: 候选列表反向断言无 admin 行
- **CV-3 D-lite kind 三态** (非冲突): mention 仅在 chat tab message body 内, 不挂 artifact body / code body / image_link (CV-3 立场 #370 ①+② 字面); 反约束: workspace tab artifact 渲染路径不读 message_mentions 表

## 3. 反查 grep 锚 (Phase 3 章程退出闸 + Phase 4 验收)

```
git grep -nE 'MentionPicker|<MentionPicker'                      packages/client/src/components/   # ≥ 1 hit (DM-2.3 候选列表组件)
git grep -nE "case\\s+['\"]mention_pushed['\"]"                  packages/client/src/realtime/wsClient.ts   # ≥ 1 hit (WS frame dispatch)
git grep -nE 'data-mention-id'                                   packages/client/src/lib/markdown.ts packages/client/src/components/   # ≥ 1 hit (DOM attr 字面锁, raw UUID 仅 attr)
git grep -nE '输入 @ 提到 channel 成员'                          packages/client/src/components/MentionPicker.tsx   # ≥ 1 hit (#314 文案锁 placeholder)
# 反约束 (5 条 0 hit)
git grep -rnE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' packages/client/src/components/MessageList.tsx | grep -v _test | grep -v data-   # 0 hit (raw UUID 不进文本节点, 跟 #312 §3 + ADM-0 #211 同源)
git grep -nE 'fallback.*toast|offline.*sender.*notify|owner.*system.*DM.*sender' packages/client/src/   # 0 hit (立场 ③ 离线 agent 发送方 UI 无提示)
git grep -nE 'MentionPicker.*admin|admin.*member.*list'          packages/client/src/components/MentionPicker.tsx   # 0 hit (admin 不入候选, ADM-0 红线)
git grep -nE 'WorkspaceTab.*MentionPicker|artifact.*editor.*mention' packages/client/src/components/   # 0 hit (workspace tab 不混 mention 流, CHN-4 立场 ② 同源)
git grep -nE '@\\$\\{.*display_name\\}|insert.*\\@\\$\\{.*\\.name' packages/client/src/components/MentionPicker.tsx   # 0 hit (回填 token 是 user_id 非 display_name, 立场 ①)
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束)

- ❌ `@channel` / `@everyone` 全员 (留 DM-3, #293 §4.b + #312 §4 字面)
- ❌ mention 撤回 / edit (跟 message edit 一起留 Phase 5+)
- ❌ mention 历史聚合视图 ("我被 @ 列表") — Phase 5+
- ❌ 跨 channel mention 候选 (#372 server 已 reject 400, client 候选不显示其他 channel 成员)
- ❌ mention 候选模糊匹配 / 拼音搜索 (Phase 5+ UX 增强, v1 走前缀字符匹配 display_name 够用)
- ❌ workspace tab artifact body 内 mention (CHN-4 立场 ② 字面, mention 仅 chat 流)
- ❌ admin SPA 看 mention 数据 (ADM-0 §1.3 红线, admin 不入业务路径)
- ❌ 离线 agent 发送方 UI 任何提示 (隐私 + 发送方语境干净, #293 §3.3 反向断言永久锁)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- vitest MentionPicker: textarea `@` 触发浮层 + channel member 列 (人+agent) + agent 🤖 角标 byte-identical + 上下键 + 回车回填 `@<user_id>` token + placeholder 字面锁 + admin 不入候选反向断言
- vitest MessageList: MentionPushedFrame 命中 mutate 重渲 + display_name 替换走 lib/markdown.ts 既有路径 + DOM `data-mention-id` attr 存在 + 文本节点反向 grep raw UUID 0 hit (5 个隐私红线 byte-identical 跟 #211)
- vitest wsClient: switch case 'mention_pushed' dispatch handler + frame 8 字段反序列化 (跟 #372 server 端 byte-identical)
- e2e: textarea `@` 候选 → 选中 agent → 提交 message → 在线 target → MentionPushedFrame ≤3s 实时刷 + 离线 target → 发送方 UI 无任何提示 (反向断言无 toast / 无 banner / 无 status); G3.4 协作场骨架 demo mention 流截屏归档 `g3.4-collab-mention-flow.png` (撑 #374 CHN-4 demo 5 张截屏)
