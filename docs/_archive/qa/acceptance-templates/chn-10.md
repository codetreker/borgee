# CHN-10 acceptance — channel description owner-only

战马D · 2026-04-30 · spec `chn-10-spec.md` + stance.

## §1 schema

- §1.1 ✅ 0 schema 改 (复用 channels.topic 既有列, GORM size:500).

## §2 server PUT /description owner-only

- §2.1 ✅ owner-only HappyPath PUT /api/v1/channels/{channelId}/description
  → 200 + topic 列写入.
- §2.2 ✅ member non-owner 403 (channel member 但 != creator).
- §2.3 ✅ length cap 500 (501+ → 400).
- §2.4 ✅ 401 unauthorized.
- §2.5 ✅ 既有 PUT /topic member-level path byte-identical 不变.

## §3 client DescriptionEditor

- §3.1 ✅ title `频道说明` 4 字 byte-identical.
- §3.2 ✅ edit `编辑` / save `保存` / cancel `取消` 2 字 byte-identical.
- §3.3 ✅ length counter `{n}/500` 字面.
- §3.4 ✅ 同义词反向 reject (`topic/about/intro/简介/主题/关于/介绍`).
- §3.5 ✅ 空 description → ChannelHeader 不渲染描述行 (fallback null).

## §4 反约束

- §4.1 ✅ 0 schema.
- §4.2 ✅ admin god-mode 不挂.
- §4.3 ✅ AL-1a reason 锁链不漂 (停在 HB-6 #19).
- §4.4 ✅ AST 锁链延伸第 17 处.
- §4.5 ✅ 既有 /topic byte-identical 不变.

## §5 测试矩阵

- TestCHN101_NoSchemaChange ✅
- TestCHN102_PutDescription_OwnerHappyPath ✅
- TestCHN102_PutDescription_NonOwnerRejected ✅
- TestCHN102_PutDescription_LengthCap500 ✅
- TestCHN102_PutDescription_Unauthorized401 ✅
- TestCHN102_TopicPathByteIdentical ✅
- TestCHN103_NoAdminDescriptionPath ✅
- TestCHN103_NoDescriptionQueue ✅
- DescriptionEditor.test.tsx 5 vitest ✅
