# CHN-9 spec brief — channel privacy 三态 (战马D v0)

> Phase 6 channel privacy / member visibility — 谁可以看到这个 channel.
> **0 schema 改** (复用 CHN-1.1 #267 既有 channels.visibility TEXT 列,
> 加第 3 enum 值 `creator_only` 跟既有 `private`/`public` 共三态;
> visibility 列 CHECK 在 app 层不在 schema 层, 跟 CHN-7/8 bitmap 0
> schema 同精神). 跟 CHN-5/6/7/8 #542/544/550/552 0 schema 模式承袭.

## §0 立场 (3 + 3 边界)

- **①** **0 schema 改** — channels.visibility TEXT 列 byte-identical
  跟 CHN-1.1 #267 不动. CHECK 校验在 app 层 (handler) 不在 schema 层
  (反向 grep `migrations/chn_9_\d+|ALTER TABLE channels.*visibility`
  在 internal/migrations/ 0 hit). 既有 `private`/`public` byte-identical
  保留 — 现有行行为零变. 加第 3 enum `creator_only` (creator-only 视图,
  非 creator member 也看不到 — 跟 ChannelMembersModal CHN-1.2 既有
  channel.manage_visibility 权限路径同源 ACL).
- **②** 三向锁 visibility 字面 — server const VisibilityCreatorOnly /
  Members (alias of "private") / OrgPublic (alias of "public"); client
  const 字面 byte-identical; ListChannelsWithUnread 既有 SQL `visibility
  = 'public'` filter byte-identical 不动 (新 enum `creator_only` 不入
  org-public preview, 反向断言 — 防 leak).
- **③** owner-only ACL — visibility 切换走既有 `channel.manage_visibility`
  permission (CHN-1.2 既有 ACL byte-identical 不动); admin god-mode **不挂
  PATCH** (反向 grep `admin.*visibility\|admin.*channel.*visibility`
  在 admin*.go 0 hit) — owner-only ACL 锁链第 17 处 (CHN-8 #16 承袭).
  反向 leak 断言: creator_only channel preview 走 CHN-1.2 既有
  GET /preview owner-only 立场 byte-identical 同源.

边界:
- **④** 入参字面验证 — PATCH /api/v1/channels/{channelId} body.visibility
  ∈ {`creator_only`, `private`, `public`} 三 case 反向 reject 外值 → 400
  invalid_value byte-identical 跟 CHN-1.2 既有 reject path 同源 (反
  reject 现有报错 `Visibility must be 'public' or 'private'` 字面替成
  `Visibility must be 'creator_only', 'private', or 'public'` 三态).
- **⑤** 文案 byte-identical 跟 content-lock §1 — VisibilityBadge 三态
  显示 `🔒 仅创建者` 5 字 / `👥 成员可见` 4 字 / `🌐 组织内可见` 5 字;
  同义词反向 reject (`secret/exclusive/team-only/外部/外公/绝密`).
- **⑥** AST 锁链延伸第 14 处 forbidden 3 token (`pendingVisibility /
  visibilityChangeQueue / deadLetterVisibility`) 在 internal/api 0 hit.

## §1 拆段

**CHN-9.1 — server**:
- `internal/api/chn_9_visibility.go` const VisibilityCreatorOnly =
  "creator_only" / VisibilityMembers = "private" (alias) /
  VisibilityOrgPublic = "public" (alias) + IsValidVisibility 谓词.
- `internal/api/channels.go::handleUpdateChannel` body.Visibility 反向
  validation 扩 3-tuple (反向 reject 报错文案改).
- `internal/api/channels.go::handleCreateChannel` 同步扩 3-tuple.
- ListChannelsWithUnread 既有 SQL `visibility = 'public'` byte-identical
  不动 (creator_only 不 leak preview); GetChannelByID 既有 ACL 路径不动.

**CHN-9.2 — client**:
- `lib/api.ts::setChannelVisibility(channelId, visibility)` 单源.
- `lib/visibility.ts::VISIBILITY_*` const 字面 byte-identical 跟 server
  + isValidVisibility 谓词.
- `components/VisibilityBadge.tsx` 三态显示 byte-identical 跟 content-lock.

**CHN-9.3 — closure**: REG-CHN9-001..006 6 🟢 + AST scan + audit 5 字段
链第 14 处.

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_9_\d+|ALTER TABLE channels.*visibility` 0 hit.
- admin 不挂: `admin.*visibility\|admin.*channel.*visibility` 在 admin*.go 0 hit.
- creator_only 不 leak preview: `creator_only.*public\|public.*creator_only`
  在 SQL/JOIN 0 hit; 反向 unit `creator_only` channel non-creator user
  GET /channels 列表不见.
- AST 锁链延伸第 14 处 forbidden 3 token 0 hit.
- Visibility const 三向锁 byte-identical (server + client + DB 字面).

## §3 不在范围

- per-message visibility (永久不挂 — message 走 channel ACL 单源).
- visibility 切换审计 (audit 5 字段链复用 既有 PATCH path).
- admin god-mode visibility override (永久不挂 ADM-0 §1.3).
- DM visibility (永久不挂 — DM 走 CHN-4 dm-only).
- visibility-based 跨 org 邀请 (留 v3 跟 AP-3 同期).
