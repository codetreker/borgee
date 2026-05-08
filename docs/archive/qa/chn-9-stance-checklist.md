# CHN-9 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 CHN-8/CHN-7/CHN-6/CHN-5 stance 同模式)
> **目的**: CHN-9 三段实施 (9.1 server / 9.2 client / 9.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off.
> **关联**: spec `chn-9-spec.md` + acceptance `acceptance-templates/chn-9.md` + content-lock `chn-9-content-lock.md`
> **content-lock 必锁** — VisibilityBadge 三态 DOM + 文案 + 同义词反向 + Visibility const 三向锁.

## §0 立场总表 (3 立场 + 3 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | **0 schema 改** — channels.visibility TEXT 列 byte-identical 跟 CHN-1.1 #267 不动. CHECK 在 app 层不在 schema 层. 加第 3 enum `creator_only` 跟既有 `private`/`public` 共三态 (现有行 byte-identical 保留, 行为零变) | CHN-1.1 #267 channels.visibility 单源 + CHN-5/6/7/8 0 schema 立场承袭 | 反向 grep `migrations/chn_9_\d+\|ALTER TABLE channels.*visibility` 0 hit |
| ② | 三向锁 visibility 字面 — server const VisibilityCreatorOnly='creator_only' / Members='private' (alias) / OrgPublic='public' (alias); client const byte-identical; DB 字面 byte-identical (改一处 = 改三处) | CHN-7 MuteBit + CHN-8 NotifPref 双/三向锁模式承袭 | server const + client const + DB 字面 byte-identical 反向 reflect 检查 |
| ③ | owner-only ACL — visibility 切换走既有 `channel.manage_visibility` permission (CHN-1.2 既有 ACL byte-identical 不动); admin god-mode **不挂 PATCH**; creator_only channel 反 leak preview (反向断言非 creator GET /channels 列表不见) — owner-only ACL 锁链第 17 处 (CHN-8 #16 承袭) | admin-model.md ADM-0 §1.3 红线 + CHN-1.2 既有 channel.manage_visibility 权限承袭 | 反向 grep `admin.*visibility\|admin.*channel.*visibility` 在 admin*.go 0 hit + 反向断言 unit creator_only 不 leak |

边界:
- **④** PATCH /api/v1/channels/{channelId} body.visibility ∈ {`creator_only`, `private`, `public`} 三 case 反向 reject 外值 → 400 (报错文案 byte-identical `Visibility must be 'creator_only', 'private', or 'public'`); existing PATCH 现网行为 byte-identical 保留 (`public`/`private` 仍 OK).
- **⑤** 文案 byte-identical 跟 content-lock §1 — `🔒 仅创建者` / `👥 成员可见` / `🌐 组织内可见` 三态显示 + 同义词反向 reject (`secret/exclusive/team-only/外部/外公/绝密`).
- **⑥** AST 锁链延伸第 14 处 forbidden 3 token (`pendingVisibility / visibilityChangeQueue / deadLetterVisibility`) 在 internal/api 0 hit.

## §1 立场 ① 0 schema 改 (CHN-9.1 守)

- [ ] migrations/ 0 新文件 (反向 grep `migrations/chn_9_` 0 hit)
- [ ] registry.go byte-identical 跟 main 不动
- [ ] channels.visibility 列复用 CHN-1.1 既有 (不另起 privacy_tier 列)
- [ ] CHN-1.2 既有 PATCH /channels/{channelId} byte-identical 不动 (仅扩 visibility validation)
- [ ] 现有 'public'/'private' 行 byte-identical 保留 — 行为零变

## §2 立场 ② 三向锁 (CHN-9.1+9.2 守)

- [ ] server const VisibilityCreatorOnly/Members/OrgPublic 字面单源
- [ ] client const VISIBILITY_CREATOR_ONLY/MEMBERS/ORG_PUBLIC 字面 byte-identical
- [ ] DB 字面 'creator_only' / 'private' / 'public' byte-identical
- [ ] IsValidVisibility 谓词单源 (server + client byte-identical)
- [ ] 改一处 = 改三处 (server const + client const + DB 字面)

## §3 立场 ③ owner-only + admin god-mode 不挂 (CHN-9.1 守)

- [ ] visibility PATCH 走既有 channel.manage_visibility permission (CHN-1.2 既有 ACL)
- [ ] admin god-mode 不挂 PATCH (反向 grep 0 hit)
- [ ] creator_only channel 不 leak preview — 反向断言非 creator GET /channels 列表不见
- [ ] ListChannelsWithUnread 既有 `visibility = 'public'` filter byte-identical 不动 (creator_only 不入 org-public preview)
- [ ] owner-only ACL 锁链第 17 处一致 (AL-2a/BPP-3.2/AL-1/AL-5/DM-4/CV-4 v2/BPP-7/BPP-8/CHN-5/CHN-6/CHN-7/CHN-8/CHN-9)

## §4 蓝图边界 ④⑤⑥ — 不漂

- [ ] PATCH body.visibility 三 case validation
- [ ] 报错文案 byte-identical
- [ ] existing public/private PATCH 现网行为不破
- [ ] VisibilityBadge 三态 emoji + 文字字面 byte-identical
- [ ] 同义词反向 (`secret/exclusive/team-only/外部/外公/绝密`) 0 hit
- [ ] AST 锁链延伸第 14 处 forbidden 3 token 0 hit

## §5 退出条件

- §1 (5) + §2 (5) + §3 (5) + §4 (6) 全 ✅
- 反向 grep 5 项全 0 hit
- audit 5 字段链 CHN-9 = 第 14 处
- AST 锁链延伸第 14 处
- owner-only ACL 锁链第 17 处一致
- Visibility 三向锁 (server + client + DB byte-identical)
- creator_only 不 leak (反向 unit + ListChannelsWithUnread filter byte-identical)
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN9-001..006
