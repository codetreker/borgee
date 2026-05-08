# CHN-10 spec brief — channel description / topic UI (战马D v0)

> Phase 6 channel description (= topic 既有列) UI 闭环 — channels.topic
> 既有列 (CM-onboarding-welcome 既有 ALTER TEXT NOT NULL DEFAULT '' 跟
> CHN-2 既有 PUT /channels/:id/topic member-level path 不动). CHN-10
> 收尾: **owner-only** PUT /channels/:id/description endpoint (跟既有
> /topic 互补 — 不动既有 path 反向 byte-identical) + ChannelHeader 头部
> 展示 + DescriptionEditor 编辑 modal + 文案 byte-identical 跟 DM-7
> EditHistoryModal 同模式.

## §0 立场 (3 + 3 边界)

- **①** 0 schema 改 (复用 channels.topic 既有列 cm-onboarding-welcome
  ALTER TEXT NOT NULL DEFAULT '' 路径承袭). 反向 grep `migrations/chn_10_\d+
  | ALTER channels.*description` 在 internal/migrations/ 0 hit.
- **②** owner-only ACL 锁链第 20 处 (DM-7 #19 + CHN-9 #14 承袭) — PUT
  /api/v1/channels/{channelId}/description owner-only (creator_id ==
  user.id 或 channel.manage_topic permission 既有 ABAC; 跟 CHN-9 visibility
  manage path 同模式 — 反 member-level), 长度上限 500 字符 (跟 channels.
  topic GORM size:500 byte-identical). 既有 PUT /topic member-level path
  byte-identical 不动 (CHN-2 #406 既有, 不破现网行为). admin god-mode
  不挂 PATCH/PUT/POST 在 admin-api/v1/.../description (ADM-0 §1.3 红线
  — admin 看不能改, 反向 grep 0 hit).
- **③** 文案 byte-identical 跟 DM-7 EditHistoryModal 同模式: title
  `频道说明` 4 字 + edit button `编辑` 2 字 + save `保存` 2 字 + cancel
  `取消` 2 字 + length counter `{n}/500`; 同义词反向 reject (`topic /
  about / intro / 简介 / 主题 / 关于 / 介绍`).

边界:
- **④** 既有 PUT /topic byte-identical 不变 — CHN-2 #406 既有 endpoint
  member-level + 250 字符上限 不动 (反向 grep dm_2 / cm_2 / chn_2
  既有 unit 全套不破); CHN-10 加新 endpoint /description **owner-only**
  500 字符上限 互补 (实际写入相同 channels.topic 列 — store.UpdateChannel
  单源 byte-identical).
- **⑤** AL-1a reason 锁链不漂 — CHN-10 read/write description 不引入新
  reason (反向 grep `chn10.*reason\|description_reason` 0 hit); reason
  锁链停在 HB-6 #19 不延伸.
- **⑥** AST 锁链延伸第 17 处 forbidden 3 token (`pendingDescription /
  descriptionQueue / deadLetterDescription`) 在 internal/api 0 hit.

## §1 拆段

**CHN-10.1 — schema**: 0 行 (复用 channels.topic).

**CHN-10.2 — server**: `internal/api/chn_10_description.go` PUT
/api/v1/channels/{channelId}/description owner-only + 500 字符上限 + 写
channels.topic 列 (复用 store.UpdateChannel 单源). server.go 加
chn10Handler.RegisterUserRoutes (admin-rail 不挂).

**CHN-10.3 — client**: `lib/api.ts::setChannelDescription` thin wrapper
+ `components/ChannelHeader.tsx` 加 description 展示行 (≤500 char ellipsis
overflow) + `components/DescriptionEditor.tsx` modal (textarea + 字符
counter + 保存/取消; 同义词反向 reject).

**CHN-10.4 — closure**: REG-CHN10-001..006 6 🟢.

## §2 反约束 grep 锚

- 0 schema: 反向 grep `migrations/chn_10_\d+\|ALTER channels.*description` 0 hit.
- 既有 /topic byte-identical: CHN-2 既有 unit (TestCH2*.go) 全套不破 — 反向
  grep `chn_10` 在 channels.go::handleSetTopic block 0 hit.
- owner-only ACL 锁链第 20 处: PUT /description handler 走 IsChannelOwner /
  channel.manage_topic ABAC 反向断 member-level reject 403.
- admin god-mode 不挂: 反向 grep `admin-api/v[0-9]+/.*description` 0 hit.
- 同义词反向 reject (client UI): `topic / about / intro / 简介 / 主题 /
  关于 / 介绍` 在 ChannelHeader.tsx + DescriptionEditor.tsx user-visible
  text 0 hit.
- AST 锁链延伸第 17 处 forbidden 3 token 0 hit.

## §3 不在范围

- description Markdown 渲染 (留 v3, 文本 only v0).
- description 历史回放 (留 v3 — DM-7 edit history 不延伸到 description).
- description push 通知 (留 v3 — 跟 RT-3 fan-out 不联动 v0).
- per-language i18n description (留 v4).
- description 全文搜 (留 v3, FTS 同期).
- admin god-mode description override (永久不挂 ADM-0 §1.3).
- description retention sweeper (留 v3 — forward-only).
