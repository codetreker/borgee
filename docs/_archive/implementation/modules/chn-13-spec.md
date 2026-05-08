# CHN-13 spec brief — channel list search/filter (战马D v0)

> Phase 6 channel sidebar 搜索/过滤闭环 — `GET /api/v1/channels?q=<query>`
> 既有 endpoint 加 optional `q` query param + ListChannelsWithUnread 加
> name 子串匹配 (case-insensitive, 单字段 LIKE). client `ChannelSearchInput.
> tsx` debounce 200ms + filter list + 空态文案. **0 schema 改** (复用
> channels 既有表). server 改 = +1 query param + ListChannelsWithUnread
> filter 参数 (10-15 行).

## §0 立场 (3 + 3 边界)

- **①** 0 schema 改 (复用 channels 既有表). 反向 grep
  `migrations/chn_13_\d+\|ALTER channels` 在 internal/migrations/ 0 hit.
- **②** server 加最小 — handleListChannels 加 `q := r.URL.Query().Get("q")`
  +传给 ListChannelsWithUnread; store 层 fn signature 加 `query string` 参数,
  生成 `WHERE c.name LIKE ?` 子句 (空 q → 不加 WHERE 既有路径
  byte-identical). 反向 grep `chn_13` 在 channels.go::handleListChannels
  block 内 0 hit (只动 query param 解析与传参, 不另起 handler).
- **③** 文案 byte-identical 锁: input placeholder `搜索频道` 4 字 + 空态
  `未找到匹配` 5 字 + count `共 N 个频道` (N 占位) byte-identical;
  同义词反向 reject (`find/lookup/locate/查找/检索/查询`) 在 user-visible
  Chinese / English 0 hit.

边界:
- **④** 既有 ListChannelsWithUnread byte-identical 行为 — 空 `q` (默认)
  返回完整 membership 列表 (跟现有 path byte-identical 不变); 仅 `q != ""`
  时加 LIKE 子句. 既有 admin-rail `/admin-api/v1/channels` 不动.
- **⑤** AL-1a reason 锁链不漂 — CHN-13 不引入新 reason (反向 grep
  `chn13.*reason\|search.*reason` 0 hit, 锁链停在 HB-6 #19).
  search 是 read-only filter, 不 audit (跟 CHN-7 mute / CHN-8 notif-pref
  立场 ⑥ "per-user UI preference 不入 admin_actions" 同精神).
- **⑥** AST 锁链延伸第 21 处 forbidden 3 token (`pendingSearch /
  searchQueue / deadLetterSearch`) 在 internal/api 0 hit.

## §1 拆段

**CHN-13.1 — schema**: 0 行 (复用 channels 既有表 + name 列既有 index).

**CHN-13.2 — server**: 最小补丁 (≤15 行 production):
- `handleListChannels`: 加 `q := strings.TrimSpace(r.URL.Query().Get("q"))`.
- `ListChannelsWithUnread(userID, q string)`: q 空走既有 path
  byte-identical; q 非空加 `AND c.name LIKE '%' || ? || '%'` (case-
  insensitive 走 SQLite COLLATE NOCASE).
- 反向 grep守门: 既有 ListChannelsWithUnread call sites (admin-rail
  + 系统 fanout) 不漂, 全部填空 `q=""` 不破语义.

**CHN-13.3 — client**:
- `lib/api.ts::listChannels(q?: string)` thin wrapper 加 optional q;
  既有 call site (空) byte-identical.
- `components/ChannelSearchInput.tsx` controlled input + debounce 200ms +
  onChange → re-fetch listChannels(q).
- `components/ChannelList.tsx` 接 search 结果 + 空态 `未找到匹配` + count
  `共 N 个频道`.

**CHN-13.4 — closure**: REG-CHN13-001..006 6 🟢. AST 锁链延伸第 21 处.

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_13_\d+|ALTER channels` 0 hit.
- 0 新 endpoint: 反向 grep `mux.Handle.*channels.*search\|channels/search`
  0 hit (复用既有 GET /api/v1/channels?q=).
- 既有 ListChannelsWithUnread call sites 全填 q="" byte-identical.
- 同义词反向 (user-visible): `find|lookup|locate|查找|检索|查询` 在 client
  user-visible Chinese/English text 0 hit (我们用 `搜索/未找到匹配/共 N 个`).
- AL-1a reason 锁链不漂: `chn13.*reason|search.*reason` 0 hit.
- AST 锁链延伸第 21 处: 3 forbidden token 0 hit.
- admin-rail 不挂 search: `admin-api/v[0-9]+/.*\?q=` 0 hit.

## §3 不在范围

- 全文搜索 (message body / artifact content) — 留 v3 (CHN-13 仅 channel
  name 子串).
- 模糊匹配 / pinyin / 分词 — 留 v3 (单 LIKE 子串足 v0).
- 跨 org 搜索 — 永不 (CM-3 cross-org 既有禁, 不变).
- admin-rail 搜索 — 留 v3 (admin /admin-api/v1/channels 既有列表已含全
  org, admin UI 自己 filter).
- audit log per search — 永不 (read-only filter, 跟 CHN-7/CHN-8 立场 ⑥).
- 搜索历史 / 推荐 / 热门 — 留 v3+.
- 搜索结果排序定制 — v0 走既有 ordering (position ASC), 留 v3+ 加 score.
