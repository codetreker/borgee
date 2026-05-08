# CHN-11 spec brief — channel member admin UI (战马D v0)

> Phase 6 channel member admin UI 闭环 — POST /api/v1/channels/{channelId}
> /members + DELETE /api/v1/channels/{channelId}/members/{userId} + GET
> /members 既有 server-side 三 endpoint (CHN-1 #276 manage_members
> ABAC + cross-owner agent guard + system DM agent_join 字面锁) byte-
> identical 不动. 本 milestone 收尾: client SPA MemberList.tsx + Add
> MemberModal + KickConfirm 模态 + 文案 byte-identical. **0 server
> production code** (跟 DM-6 #556 / CV-12 #545 / DM-5 #549 / CV-9..12 系列
> 0-server 模式承袭).

## §0 立场 (3 + 3 边界)

- **①** 0 schema 改 (复用 channel_members 既有表). 反向 grep
  `migrations/chn_11_\d+\|ALTER channel_members` 在 internal/migrations/
  0 hit.
- **②** **0 server production code** — POST/DELETE /channels/:id/members
  既有 ABAC `channel.manage_members` (= owner-default) + agent cross-owner
  guard + agent_join system DM byte-identical 不动 (反向 grep `chn_11`
  在 internal/api/*.go 非 _test.go production 0 hit). git diff main..HEAD
  -- 'packages/server-go/**/*.go' ':!**/*_test.go' 0 行.
- **③** 文案 byte-identical 锁: 列表 title `成员` 2 字 + add button
  `添加成员` 4 字 + 行 remove `移除` 2 字 + 确认 modal title `确认移除
  {user}?` byte-identical (user 占位); 同义词反向 reject (`invite/kick/
  remove/expel/逐出/踢出/邀请`) 在 user-visible 0 hit.

边界:
- **④** 既有 server endpoint byte-identical — CHN-1 #276 既有 POST/DELETE
  /members + manage_members ABAC + agent cross-owner reject + agent_join
  system DM 文案锁 全套不动 (反向 grep `chn_11` 在 channels.go::
  handleAddMember + handleRemoveMember block 0 hit).
- **⑤** AL-1a reason 锁链不漂 — CHN-11 不引入新 reason (反向 grep
  `chn11.*reason\|member.*removed_reason` 0 hit, 锁链停在 HB-6 #19).
  audit 5 字段链不延伸 — 复用既有 `user_joined / user_left` events 路径
  byte-identical (CHN-1 既有 path 不动).
- **⑥** AST 锁链延伸第 19 处 forbidden 3 token (`pendingMemberAdmin /
  memberAdminQueue / deadLetterMemberAdmin`) 在 internal/api 0 hit.

## §1 拆段

**CHN-11.1 — schema**: 0 行 (复用 channel_members).

**CHN-11.2 — server**: **0 行 production** — 既有 POST/DELETE/GET
/api/v1/channels/{channelId}/members byte-identical 不动. 反向 grep守门
test (`chn_11_no_server_prod_test.go::TestCHN111_NoServerProductionCode`).

**CHN-11.3 — client**:
- `lib/api.ts::addChannelMember(channelId, userId)` 已有 ChannelHandler
  接 POST 既有, 加 thin wrapper.
- `lib/api.ts::removeChannelMember(channelId, userId)` thin wrapper
  for existing DELETE.
- `lib/api.ts::listChannelMembers(channelId)` thin wrapper for existing
  GET.
- `components/MemberList.tsx` 列表 + 加/移除 button.
- `components/AddMemberModal.tsx` (input userId/email + submit).
- `components/KickConfirmModal.tsx` (title byte-identical + 确认 移除).

**CHN-11.4 — closure**: REG-CHN11-001..006 6 🟢.

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_11_\d+\|ALTER channel_members` 0 hit.
- 0 server prod: `git diff main..HEAD -- 'packages/server-go/**/*.go'
  ':!**/*_test.go'` 0 行 + 反向 grep `chn_11` 在 internal/api/*.go 非
  _test.go 0 hit.
- 既有 endpoint byte-identical: handleAddMember / handleRemoveMember
  block 内反向 grep `chn_11` 0 hit.
- 同义词反向 (user-visible Chinese): `逐出/踢出/邀请` 在 client/src/ user-
  visible Chinese text 0 hit.
- AL-1a reason 锁链不漂: `chn11.*reason\|member.*removed_reason` 0 hit.
- AST 锁链延伸第 19 处 forbidden 3 token 0 hit.

## §3 不在范围

- 角色管理 (admin/moderator) — 留 v3, 当前 channel_members 无 role 列.
- bulk add/remove (留 v3, 单条 v0).
- member 历史回放 (留 v3 — DM-7 edit history 不延伸).
- pending invitations (留 v3 — 跟 INV-* 同期).
- admin god-mode kick override (永久不挂 ADM-0 §1.3).
- cross-org member add (CM-3 cross-org 既有禁, 不变).
- audit log entry per add/remove (留 v3 — events `user_joined/user_left`
  既有 audit 路径承袭, 不另起 admin_actions enum).
