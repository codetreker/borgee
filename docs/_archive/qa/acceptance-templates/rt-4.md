# RT-4 acceptance — channel presence indicator

战马D · 2026-04-30 · spec `rt-4-spec.md` + stance.

## §1 schema

- §1.1 ✅ 0 schema 改 (复用 AL-3.1 #277 presence_sessions).

## §2 server GET /channels/{id}/presence

- §2.1 ✅ member HappyPath → 200 + `{online_user_ids: [...], counted_at}`.
- §2.2 ✅ non-member 403.
- §2.3 ✅ 401 unauthorized.
- §2.4 ✅ 既有 typing WS path byte-identical 不变 (RT-2).

## §3 client ChannelPresenceList

- §3.1 ✅ `当前在线 N 人` 文案 byte-identical (N 计数动态).
- §3.2 ✅ 头像列表 ≤5 + `+N` overflow when N > 5.
- §3.3 ✅ 同义词反向 reject (`presence/typing/composing/在线状态/上线/在线人员`).
- §3.4 ✅ 空 → 整个 list 不渲染 (return null).

## §4 反约束

- §4.1 ✅ 0 schema.
- §4.2 ✅ 0 新 WS frame (presence-change push 留 v3).
- §4.3 ✅ admin god-mode 不挂.
- §4.4 ✅ AL-1a reason 锁链不漂 (停在 HB-6 #19).
- §4.5 ✅ AST 锁链延伸第 18 处.
- §4.6 ✅ 既有 RT-2 typing path byte-identical 不变.

## §5 测试矩阵

- TestRT41_NoSchemaChange ✅
- TestRT41_TypingPathByteIdentical ✅
- TestRT42_GetPresence_MemberHappyPath ✅
- TestRT42_GetPresence_NonMemberRejected ✅
- TestRT42_GetPresence_Unauthorized401 ✅
- TestRT43_NoAdminPresencePath ✅
- TestRT43_NoPresenceQueue ✅
- ChannelPresenceList.test.tsx 5 vitest ✅
