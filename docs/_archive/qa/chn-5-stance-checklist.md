# CHN-5 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 AL-7/AL-8/HB-5 stance 同模式)
> **目的**: CHN-5 四段实施 (5.1 schema 0 行 / 5.2 server / 5.3 client / 5.4 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/chn-5-spec.md` (战马D v0) + acceptance `docs/qa/acceptance-templates/chn-5.md` + content-lock `docs/qa/chn-5-content-lock.md`
> **content-lock 必锁** — 客户端 ArchivedChannelsPanel + 恢复 button + system DM 文案 (archive/unarchive 互补二式).

## §0 立场总表 (3 立场 + 3 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | **0 schema 改** — channels.archived_at 列 + ListChannelsWithUnread 过滤 byte-identical 跟 CHN-1.1 #267 / CHN-1.2 #288 不动 | channel-model.md §2 不变量 + CHN-1.1 既有 archive_at 列单源 | 反向 grep `migrations/chn_5_\d+\|chn5_\d+_archive\|ALTER TABLE channels ADD COLUMN.*archived` 在 internal/migrations/ 0 hit |
| ② | owner-only ACL — GET /api/v1/me/archived-channels 只见 user 自己 member 的 archived 频道 (跟 ChannelMembersModal archive 按钮 owner-only byte-identical 同源, 13 处 owner-only ACL 锁链不漂); admin god-mode **不挂 PATCH** 路径 | admin-model.md ADM-0 §1.3 红线 + AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8 owner-only 12 处承袭 | 反向 grep `admin.*archive_channel\|admin.*unarchive` 在 admin*.go 0 hit; admin handler 必只暴露 GET (反向 grep PATCH/PUT/DELETE 在 hb5/chn5 admin handler 0 hit) |
| ③ | unarchive system DM fanout — fanoutUnarchiveSystemMessage 跟既有 fanoutArchiveSystemMessage 同模式; 文案 byte-identical 跟 content-lock §1 互补二式 (archive `channel #{name} 已被 {actor} 归档` / unarchive `channel #{name} 已被 {actor} 恢复`) | channel-model.md §2 不变量 #3 + CHN-1.2 archive system DM 立场承袭 | 反向 grep handler 内 hardcode 文案 0 hit (走 store helper 单源) |

边界:
- **④** admin-rail readonly — GET /admin-api/v1/channels/archived admin cookie middleware 必经; 反向 grep PATCH/PUT/DELETE 在此 path 0 hit.
- **⑤** client ArchivedChannelsPanel — 折叠区 (默认 collapsed); 行 hover button "恢复" 调 archiveChannel(id, false) (lib/api.ts 单源不动); `已归档` badge byte-identical 跟 CHN-1.3 #288 SortableChannelItem 同源.
- **⑥** AST 锁链延伸第 10 处 forbidden 3 token (`pendingChannelArchive / channelArchiveQueue / deadLetterChannelArchive`) 在 internal/api 0 hit (跟 BPP-4/5/6/7/8 + HB-3 v2 + AL-7/8 + HB-5 同模式).

## §1 立场 ① 0 schema 改 (CHN-5.1 守)

**反约束清单**:

- [ ] migrations/ 0 新文件 (反向 grep `migrations/chn_5_` 0 hit)
- [ ] registry.go 字面 byte-identical 跟 main 不动 (al71/hb51 后无新条目)
- [ ] channels.archived_at 列复用 CHN-1.1 #267 既有, 不另起 archived_state column
- [ ] ListChannelsWithUnread WHERE archived_at IS NULL 字面 byte-identical 跟 #288 既有不动

## §2 立场 ② owner-only + admin god-mode 不挂 (CHN-5.2 守)

**反约束清单**:

- [ ] GET /api/v1/me/archived-channels owner-only (cm.user_id = ? 跟 ListChannelsWithUnread 同精神)
- [ ] cross-org 过滤承袭 CM-3 #208 — 别 org 用户 0 行
- [ ] admin god-mode 不挂 PATCH (反向 grep `admin.*archive_channel\|admin.*unarchive` 在 admin*.go 0 hit)
- [ ] admin GET endpoint 仅 readonly — 反向 grep `mux\.Handle\("(PATCH|PUT|DELETE).*admin-api/v1/channels/archived` 0 hit
- [ ] admin god-mode 调 PATCH /api/v1/channels/{id} archive — 反向断言 401 (admin cookie 走 user-rail middleware 走 401, ADM-0 §1.3 红线)

## §3 立场 ③ unarchive system DM 互补 (CHN-5.2+5.3 守)

**反约束清单**:

- [ ] fanoutUnarchiveSystemMessage 跟 fanoutArchiveSystemMessage 同模式 — 都是每 member 一行 system DM (channel-model.md §2 不变量 #3)
- [ ] 文案 byte-identical 跟 content-lock §1 (archive: `channel #{name} 已被 {actor} 归档` / unarchive: `channel #{name} 已被 {actor} 恢复` 字面)
- [ ] handleUpdateChannel unarchive 分支 (body.Archived=false 且 ch.ArchivedAt!=nil) 必调 fanoutUnarchiveSystemMessage
- [ ] 反向 grep handler 内 hardcode 文案 0 hit (走 store.RenderArchive*Body 单源)
- [ ] WS frame `channel_unarchived` 跟既有 `channel_archived` 字面互补二式

## §4 蓝图边界 ④⑤⑥ — 不漂

**反约束清单**:

- [ ] admin-rail readonly — adminMw 真挂 + 反向 grep PATCH/PUT/DELETE 在 admin handler 0 hit
- [ ] client ArchivedChannelsPanel — 折叠 toggle button DOM byte-identical 跟 content-lock §2
- [ ] `已归档` badge 字面跟 CHN-1.3 #288 SortableChannelItem byte-identical
- [ ] AST 锁链延伸第 10 处 — `pendingChannelArchive\|channelArchiveQueue\|deadLetterChannelArchive` 0 hit

## §5 退出条件

- §1 (4) + §2 (5) + §3 (5) + §4 (4) 全 ✅
- 反向 grep 5 项全 0 hit (新 schema / new endpoint variant / admin PATCH / cron / channel-archive queue)
- audit 5 字段链 CHN-5 = 第 10 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8+HB-5+CHN-5)
- AST 锁链延伸第 10 处
- owner-only ACL 锁链 13 处一致
- 文案 byte-identical 跟 content-lock + CHN-1.3 同源
- 登记 REG-CHN5-001..006
