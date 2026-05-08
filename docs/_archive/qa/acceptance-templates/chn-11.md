# CHN-11 acceptance — channel member admin UI

战马D · 2026-04-30 · spec `chn-11-spec.md` + stance.

## §1 schema / server

- §1.1 ✅ 0 schema 改 (复用 channel_members 既有表).
- §1.2 ✅ 0 server production code — POST/DELETE/GET /channels/:id/members
  CHN-1 #276 既有 path byte-identical 不动.
- §1.3 ✅ 既有 manage_members ABAC + agent cross-owner guard + agent_join
  system DM 文案锁 byte-identical 不变.

## §2 client MemberList + AddMemberModal + KickConfirmModal

- §2.1 ✅ MemberList title `成员` 2 字 byte-identical.
- §2.2 ✅ AddMember button `添加成员` 4 字 byte-identical.
- §2.3 ✅ row remove button `移除` 2 字 byte-identical.
- §2.4 ✅ KickConfirmModal title `确认移除 {user}?` byte-identical (user
  占位).
- §2.5 ✅ 同义词反向 reject (`invite/kick/remove/expel/逐出/踢出/邀请`).
- §2.6 ✅ 空 members → MemberList 不渲染 (return null).

## §3 反约束

- §3.1 ✅ 0 schema.
- §3.2 ✅ 0 server prod.
- §3.3 ✅ 既有 handleAddMember + handleRemoveMember block byte-identical.
- §3.4 ✅ admin god-mode 不挂 (反向 grep admin-api/v1/.../members 0 hit).
- §3.5 ✅ AL-1a reason 锁链不漂 (停在 HB-6 #19).
- §3.6 ✅ AST 锁链延伸第 19 处.

## §4 测试矩阵

- TestCHN111_NoSchemaChange ✅
- TestCHN111_NoServerProductionCode ✅
- TestCHN111_HandlersByteIdentical ✅
- TestCHN113_NoMemberAdminQueue ✅
- TestCHN113_NoAdminMembersPath ✅
- MemberList.test.tsx 5 vitest ✅
