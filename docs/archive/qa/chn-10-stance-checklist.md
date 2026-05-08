# CHN-10 stance checklist (战马D v0)

> 战马D · 2026-04-30 · CHN-10 立场守门 (3 + 3 边界).

## §0 立场 3 项

- [x] **① 0 schema 改** — 复用 channels.topic 既有列 (cm-onboarding-welcome
  既有 ALTER TEXT NOT NULL DEFAULT '' + GORM size:500). 反向 grep
  `migrations/chn_10_\d+\|ALTER channels.*description` 0 hit.
- [x] **② owner-only ACL 锁链第 20 处** — PUT /api/v1/channels/{channelId}
  /description owner-only (creator_id == user.id 或 channel.manage_topic
  ABAC; 反 member-level reject 403); 长度上限 500 (channels.topic GORM
  size:500 byte-identical); 既有 PUT /topic member-level + 250 字符
  byte-identical 不动 (CHN-2 #406 既有 path 不破); admin god-mode 不挂
  PATCH/PUT/POST/DELETE 在 admin-api/v1/.../description (ADM-0 §1.3 红线).
- [x] **③ 文案 byte-identical 锁** — title `频道说明` 4 字 + edit button
  `编辑` 2 字 + save `保存` 2 字 + cancel `取消` 2 字 + length counter
  `{n}/500`; 同义词反向 reject (`topic / about / intro / 简介 / 主题 /
  关于 / 介绍`) — DescriptionEditor.tsx + ChannelHeader.tsx user-visible
  text 反向 grep 0 hit (data-testid + className + import paths 例外).

## §0.边界 3 项

- [x] **④ 既有 /topic byte-identical 不变** — CHN-2 #406 既有 PUT /topic
  member-level + 250 字符 path 不动; CHN-10 加新 endpoint /description
  owner-only + 500 字符 互补 (实际写入相同 channels.topic 列, store.
  UpdateChannel 单源 byte-identical 不漂).
- [x] **⑤ AL-1a reason 锁链不漂** — CHN-10 read/write description 不引入
  新 reason (反向 grep `chn10.*reason\|description_reason` 0 hit); reason
  锁链停在 HB-6 #19 不延伸.
- [x] **⑥ AST 锁链延伸第 17 处** — forbidden 3 token (`pendingDescription
  / descriptionQueue / deadLetterDescription`) 在 internal/api 0 hit.

## §1 测试覆盖

- [x] REG-CHN10-001 0 schema 反向断言 (`TestCHN101_NoSchemaChange`
  filepath.Walk migrations/ 反向 chn_10_* 0 hit).
- [x] REG-CHN10-002 PUT /description owner-only HappyPath + member non-owner
  403 + 401 (`TestCHN102_PutDescription_OwnerHappyPath` + `_NonOwnerRejected`
  + `_Unauthorized401`).
- [x] REG-CHN10-003 length cap 500 (`TestCHN102_PutDescription_LengthCap500`
  500 字符 OK / 501 reject 400).
- [x] REG-CHN10-004 既有 PUT /topic byte-identical 不变 (`TestCHN102_TopicPath
  ByteIdentical` 反向 grep dm_10 字面在 channels.go::handleSetTopic block
  0 hit).
- [x] REG-CHN10-005 admin god-mode 不挂 (`TestCHN103_NoAdminDescriptionPath`
  反向 grep admin-api/v[0-9]+/.../description PUT/PATCH/POST/DELETE 0 hit).
- [x] REG-CHN10-006 client DescriptionEditor 文案 byte-identical + 同义词
  反向 reject (`vitest 5 case`: 文案 byte-identical + 字符 counter +
  500 上限 + 同义词反向 + 空 description fallback) + AST 锁链延伸第 17 处
  (`TestCHN103_NoDescriptionQueue` 反向 grep 3 forbidden 0 hit).

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_10_\d+|ALTER channels.*description` 0 hit.
- owner-only: PUT /description handler 走 IsChannelOwner / manage_topic ABAC.
- admin-rail 不挂: `admin-api/v[0-9]+/.*description` 0 hit.
- 同义词反向 (user-visible): `topic|about|intro|简介|主题|关于|介绍` 0 hit.
- AST 锁链延伸第 17 处: 3 forbidden token 0 hit.
- AL-1a reason 锁链不漂: `chn10.*reason|description_reason` 0 hit.
