# CHN-11 stance checklist (战马D v0)

战马D · 2026-04-30 · 立场守门 (3+3 边界).

## §0 立场 3 项

- [x] **① 0 schema 改** — 复用 channel_members 既有表; 反向 grep
  `migrations/chn_11_\d+\|ALTER channel_members` 0 hit.
- [x] **② 0 server production code** — POST/DELETE/GET /channels/:id/members
  CHN-1 #276 既有 path byte-identical 不动 (manage_members ABAC + agent
  cross-owner guard + agent_join system DM 文案锁); git diff 0 行
  production; 反向 grep `chn_11` 在 internal/api/*.go 非 _test.go 0 hit.
- [x] **③ 文案 byte-identical** — title `成员` + add button `添加成员`
  + remove `移除` + KickConfirm `确认移除 {user}?` byte-identical;
  同义词反向 reject `invite/kick/remove/expel/逐出/踢出/邀请`.

## §0.边界 3 项

- [x] **④ 既有 server endpoint byte-identical** — handleAddMember +
  handleRemoveMember block 内反向 grep `chn_11` 0 hit.
- [x] **⑤ AL-1a reason 锁链不漂** — CHN-11 不引入新 reason (反向 grep
  `chn11.*reason\|member.*removed_reason` 0 hit, 锁链停在 HB-6 #19);
  audit 5 字段链不延伸 (复用 user_joined/user_left events).
- [x] **⑥ AST 锁链延伸第 19 处** — forbidden 3 token (`pendingMemberAdmin
  / memberAdminQueue / deadLetterMemberAdmin`) 0 hit.

## §1 测试

- [x] REG-CHN11-001 0 schema (`TestCHN111_NoSchemaChange`).
- [x] REG-CHN11-002 0 server prod (`TestCHN111_NoServerProductionCode`
  反向 grep `chn_11` 在 internal/api/*.go 非 _test.go production count==0).
- [x] REG-CHN11-003 既有 handleAddMember + handleRemoveMember block
  byte-identical (`TestCHN111_HandlersByteIdentical` 反向 grep chn_11
  在 block 0 hit).
- [x] REG-CHN11-004 AST 锁链延伸第 19 处 (`TestCHN113_NoMemberAdminQueue`).
- [x] REG-CHN11-005 client MemberList + AddMemberModal + KickConfirmModal
  文案 byte-identical (`成员` / `添加成员` / `移除` / `确认移除 {user}?`)
  + 同义词反向 reject + 5 vitest.
- [x] REG-CHN11-006 admin god-mode 不挂 PATCH/POST/PUT/DELETE 在 admin-
  api/v1/.../members (`TestCHN113_NoAdminMembersPath`).

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_11_\d+|ALTER channel_members` 0 hit.
- 0 server prod: `chn_11` 在 internal/api/*.go 非 _test.go 0 hit.
- 同义词反向 (user-visible): `逐出|踢出|邀请` 在 client user-visible
  Chinese text 0 hit (我们用 `成员/添加成员/移除`).
- AL-1a reason 锁链不漂: `chn11.*reason|member.*removed_reason` 0 hit.
- AST 锁链延伸第 19 处: 3 forbidden token 0 hit.
- admin-rail 不挂: `admin-api/v[0-9]+/.*members` 0 hit.
