# CHN-5 spec brief — channel archived UI + 列表 (战马D v0)

> Phase 6 channel archive 闭环 — CHN-1.2/.3 #265/#288 既有 archive/unarchive
> PATCH + 列表过滤 + badge UI 已落. 本 milestone 收尾: (1) **0 schema 改**
> (channels.archived_at 列由 chn_1_1 #267 已落), (2) GET 列表 endpoint 给
> 业主看自己已归档频道, (3) unarchive 加 system DM fanout (archive 已有
> CHN-1.2 既有), (4) 客户端 ArchivedChannelsPanel 折叠区, (5) admin-rail
> readonly GET endpoint admin 看全 org archived channels (admin god-mode
> 不挂 PATCH — ADM-0 §1.3).

## §0 立场 (3 + 3 边界)

- **①** **0 schema 改** — channels.archived_at 列 + cross-CHN-1 既有
  ListChannelsWithUnread 过滤 (`archived_at IS NOT NULL` filter) byte-
  identical 不动. 反向 grep `migrations/chn_5_\d+|ALTER TABLE channels
  ADD COLUMN.*archived` 在 internal/migrations/ 0 hit.
- **②** owner-only — GET /api/v1/me/archived-channels 只见 user 自己
  member 的 archived 频道 (跟 ChannelMembersModal 既有 archive 按钮 owner-
  only ACL byte-identical 同源, 13 处 owner-only ACL 锁链不漂); admin god-
  mode 不挂 PATCH 路径 (反向 grep `admin.*archive_channel\|admin.*unarchive`
  在 admin*.go 0 hit) — admin 只读 audit-log 看事件.
- **③** unarchive system DM fanout — handleUpdateChannel 加 unarchive
  分支补 fanoutUnarchiveSystemMessage (archive fanoutArchiveSystemMessage
  既有 CHN-1.2). 文案 byte-identical 跟 content-lock §1: archive `channel
  #{name} 已被 {actor} 归档` / unarchive `channel #{name} 已被 {actor} 恢复`
  二式互补.

边界:
- **④** admin-rail readonly — GET /admin-api/v1/channels/archived admin
  cookie middleware 必经 (admin god-mode 看不能改, 反向 grep PATCH/PUT/DELETE
  在 admin handler 0 hit).
- **⑤** client ArchivedChannelsPanel — 折叠区 (默认 collapsed) 渲染当前
  user 的 archived 频道; 行右键 / hover button "恢复" 调 archiveChannel(id,
  false) (既有 lib/api.ts 单源不动); 文案 `已归档` badge byte-identical 跟
  CHN-1.3 既有 #288 SortableChannelItem 同源.
- **⑥** AST 锁链延伸第 10 处 forbidden 3 token (`pendingChannelArchive /
  channelArchiveQueue / deadLetterChannelArchive`) 在 internal/api 0 hit
  (跟 BPP-4/5/6/7/8/HB-3 v2/AL-7/AL-8/HB-5 同模式).

## §1 拆段

**CHN-5.1 — schema 0 行**: 复用 CHN-1.1 #267 既有 channels.archived_at.

**CHN-5.2 — server**:
- `internal/store/queries.go::ListArchivedChannelsForUser(userID)` —
  `WHERE c.archived_at IS NOT NULL AND cm.user_id=?` (cross-org 过滤跟
  既有 ListChannelsWithUnread 同精神 + 跨 org 不可见 立场承袭 CM-3 #208).
- `internal/store/queries.go::ListAllArchivedChannelsForAdmin()` — admin-
  rail readonly 全 org archived 视图 (跟 ListAllChannelsForAdmin 同模式).
- `internal/api/channels.go` 加 `handleListMyArchived` 用户路由 + 加
  `handleAdminListArchived` admin 路由; 加 `fanoutUnarchiveSystemMessage`
  跟既有 fanoutArchiveSystemMessage 同模式 + handleUpdateChannel 分支
  调用.

**CHN-5.3 — client**: `packages/client/src/components/Archived
ChannelsPanel.tsx` 折叠区 + `lib/api.ts::listArchivedChannels()` 单源
+ vitest 4 PASS (列表渲染 / 恢复 button / 折叠状态 / 空态).

**CHN-5.4 — closure**: REG-CHN5-001..006 6 🟢 + AST scan 反向 + audit
fanout 文案锁 跟 content-lock byte-identical.

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_5_\d+|chn5_\d+_archive` 0 hit.
- admin god-mode 不挂 PATCH: `admin.*archive_channel\|admin.*unarchive`
  在 admin*.go 0 hit.
- 不开 user-rail unarchive 旁路: `/api/v1/.*unarchive[^-]` 0 hit (走
  既有 PATCH archived:false 单源).
- AST 锁链延伸第 10 处 forbidden 3 token 0 hit.

## §3 不在范围

- 自动 archive 策略 (留 v3) / channel age-based auto-archive cron (留 v3).
- archived channel 强制 hard-delete (永久不挂 — forward-only 跟 AL-1 锁).
- archived row 跨 org 同步 (留 v3 跟 AP-3 同期).
- admin god-mode PATCH archive (永久不挂 — ADM-0 §1.3 红线 admin 只观察).
- archived channel 恢复后历史消息 cursor 重置 (留 v3, 现网行为零变).
