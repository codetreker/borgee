# CHN-13 stance checklist (战马D v0)

战马D · 2026-04-30 · 立场守门 (3+3 边界).

## §0 立场 3 项

- [x] **① 0 schema 改** — 复用 channels 既有表 + name 既有 index;
  反向 grep `migrations/chn_13_\d+\|ALTER channels` 0 hit.
- [x] **② server 最小补丁** (≤15 行 production) — `handleListChannels`
  加 `q := strings.TrimSpace(r.URL.Query().Get("q"))` + 传给
  `ListChannelsWithUnread(userID, q)`; q="" 走既有 path byte-identical,
  q!="" 加 `AND c.name LIKE '%' || ? || '%' COLLATE NOCASE`. 既有
  callsites (admin-rail + welcome / fanout) 全填 `q=""` byte-identical.
- [x] **③ 文案 byte-identical** — input placeholder `搜索频道` 4 字 +
  空态 `未找到匹配` 5 字 + count `共 N 个频道` (N 占位) byte-identical;
  同义词反向 reject `find/lookup/locate/查找/检索/查询`.

## §0.边界 3 项

- [x] **④ 既有 ListChannelsWithUnread byte-identical** — 空 q 路径不破
  (反向 grep `q == ""` 走原 SQL path byte-identical); 既有 admin-rail
  `/admin-api/v1/channels` 不动 (CHN-13 仅 user-rail).
- [x] **⑤ AL-1a reason 锁链不漂** — CHN-13 不引入新 reason (反向 grep
  `chn13.*reason\|search.*reason` 0 hit, 锁链停在 HB-6 #19);
  search 是 read-only filter 不 audit (跟 CHN-7 mute / CHN-8 notif-pref
  立场 ⑥ "per-user UI preference 不入 admin_actions" 同精神).
- [x] **⑥ AST 锁链延伸第 21 处** — forbidden 3 token (`pendingSearch
  / searchQueue / deadLetterSearch`) 0 hit.

## §1 测试

- [x] REG-CHN13-001 0 schema (`TestCHN131_NoSchemaChange`).
- [x] REG-CHN13-002 server 加 q query param happy + empty q byte-identical
  (`TestCHN132_ListChannelsWithQuery`).
- [x] REG-CHN13-003 LIKE 大小写不敏感 + 子串匹配 (`TestCHN132_QueryCaseInsensitive`
  + `TestCHN132_QuerySubstringMatch`).
- [x] REG-CHN13-004 AST 锁链延伸第 21 处 (`TestCHN133_NoSearchQueue`).
- [x] REG-CHN13-005 admin god-mode 不挂 search (`TestCHN133_NoAdminSearchPath`)
  反向 grep admin-api/v1/channels?q= 0 hit.
- [x] REG-CHN13-006 client ChannelSearchInput 文案 byte-identical
  (`搜索频道` placeholder + `未找到匹配` 空态 + `共 N 个频道` count) +
  同义词反向 reject + debounce 200ms.

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_13_\d+|ALTER channels` 0 hit.
- 0 新 endpoint: `mux.Handle.*channels.*search|channels/search` 0 hit
  (复用既有 GET /api/v1/channels).
- 既有 ListChannelsWithUnread call sites 全填 q="" byte-identical.
- 同义词反向 (user-visible): `find|lookup|locate|查找|检索|查询` 在
  client user-visible Chinese/English text 0 hit.
- AL-1a reason 锁链不漂: `chn13.*reason|search.*reason` 0 hit.
- AST 锁链延伸第 21 处: 3 forbidden token 0 hit.
- admin-rail 不挂 search: `admin-api/v[0-9]+/.*\?q=` 0 hit.
