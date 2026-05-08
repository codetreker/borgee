# Acceptance Template — CHN-8: channel notification preferences

> 蓝图 channel-model.md §3 layout per-user. Spec `chn-8-spec.md` (战马D v0). Stance `chn-8-stance-checklist.md`. Content-lock `chn-8-content-lock.md`. **0 schema 改** — collapsed bitmap bits 2-3 (CHN-3 bit 0 + CHN-7 bit 1 + CHN-8 bits 2-3 拆死). Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 CHN-8.1 — server REST endpoint + bitmap 三态 + push integration

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 0 schema 改反向断言 — migrations/ 0 新文件 + registry.go byte-identical 不动 | grep | 战马D / 飞马 / 烈马 | `internal/api/chn_8_notif_pref_test.go::TestCHN81_NoSchemaChange` 1 unit PASS |
| 1.2 PUT /api/v1/channels/{channelId}/notification-pref owner-only 三态 — body `{pref:'all'/'mention'/'none'}`; collapsed bits 2-3 set 跟 NotifPref* 三态 byte-identical; non-member 403; DM 400; spec 外值 → 400 `notification_pref.invalid_value` | unit (5 sub-case) | 战马D / 烈马 | `TestCHN81_SetPref_All` (collapsed bits 2-3 = 0) + `_SetPref_Mention` (= 1) + `_SetPref_None` (= 2) + `_SetPref_RejectsInvalidValue` (4 case `xxx/3/null/empty` → 400) + `_SetPref_NonMemberRejected` (403) |
| 1.3 NotifPrefShift=2 / NotifPrefMask=3 / NotifPrefAll=0/Mention=1/None=2 const + GetNotifPref(collapsed) 谓词单源 — server const byte-identical 跟 client `NOTIF_PREF_*` 三向锁 (server + client + bitmap) | unit + grep | 战马D / 飞马 / 烈马 | `_NotifPrefConsts_ByteIdentical` (NotifPrefShift==2 + NotifPrefMask==3 + NotifPrefAll==0 + NotifPrefMention==1 + NotifPrefNone==2 + GetNotifPref(0)==All + GetNotifPref(4)==Mention + GetNotifPref(8)==None) + 双向锁 vitest |
| 1.4 admin god-mode 不挂 反向断言 — `admin.*notification.*pref\|admin.*notif_pref\b\|/admin-api/.*notification` 在 admin*.go 0 hit | grep | 战马D / 飞马 / 烈马 | `TestCHN81_NoAdminNotifPrefPath` (双反向 pattern 0 hit) |
| 1.5 bitmap 不互扰 — 改 pref 不动 collapsed bit 0 (CHN-3) 也不动 mute bit 1 (CHN-7); 反向断言 collapse → mute → pref → unmute → uncollapse 链 round-trip | unit | 战马D / 烈马 | `_BitmapIsolation_PreservesOtherBits` (PUT collapsed=1 → POST mute → PUT pref=mention → DELETE mute → PUT collapsed=0 chain, 中途 GetNotifPref 仍 mention) |

### §2 CHN-8.2 — client NotificationPrefDropdown + 文案锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 NotificationPrefDropdown.tsx `<select>` 三选一 DOM byte-identical 跟 content-lock §1 (option value="all"/"mention"/"none" + 文案 `所有消息` 4 字 / `仅@提及` 4 字 / `不打扰` 3 字 + data-testid="notification-pref-dropdown") + onChange → setNotificationPref API call | vitest (3 PASS) | 战马D / 野马 / 烈马 | `packages/client/src/__tests__/NotificationPrefDropdown.test.tsx` (三选一 DOM byte-identical / change → API call / 同义词反向) |
| 2.2 NOTIF_PREF_SHIFT=2 / NOTIF_PREF_MASK=3 / NOTIF_PREF_ALL/MENTION/NONE 字面 byte-identical 跟 server const + getNotifPref 谓词单源 | vitest (1 PASS) | 战马D / 飞马 / 烈马 | `_NotifPrefConsts_ByteIdentical` (跟 server reflect 双向锁) |
| 2.3 同义词反向 reject (`subscribe/follow/unsubscribe/snooze/订阅/关注/取消订阅` 0 hit user-visible text) | vitest (1 PASS) | 战马D / 野马 / 烈马 | `_NoSynonyms` |
| 2.4 lib/api.ts::setNotificationPref 单源 + lib/notif_pref.ts NOTIF_PREF_* 单源 | grep + vitest | 战马D / 烈马 | `_APIClientSingleSource` |

### §3 CHN-8.3 — closure + AST 锁链延伸第 13 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ AST 锁链延伸第 13 处 forbidden 3 token (`pendingNotifPref / notifPrefQueue / deadLetterNotifPref`) 在 internal/api+push 0 hit + 立场 ③ mute 不 drop messages 反向 grep `notif_pref.*skip.*broadcast\|notif_pref.*drop.*message` 0 hit | AST scan + grep | 飞马 / 烈马 | `TestCHN83_NoNotifPrefQueue` + `TestCHN81_NotifPrefDoesNotDropMessages` 2 unit PASS |

## 边界

- CHN-3.1 #410 user_channel_layout 列复用 / CHN-3.2 #412 既有 endpoint byte-identical 不动 / CHN-6 #544 PinThreshold + CHN-7 #550 MuteBit 双向锁模式承袭 / DL-4 push gateway 立场承袭 / ADM-0 §1.3 红线 admin god-mode 不挂 / owner-only ACL 锁链 16 处一致 / audit 5 字段链第 13 处 / AST 锁链延伸第 13 处 / **0 schema 改** / NotifPref 三向锁 (server + client + bitmap byte-identical) / bitmap bit 0/1/2-3 拆死 (CHN-3/CHN-7/CHN-8 互不干扰)

## 退出条件

- §1 (5) + §2 (4) + §3 (1) 全绿 — 一票否决
- 0 schema 改 / 0 新 migration
- CHN-3.2/CHN-6/CHN-7 既有 unit 不破
- audit 5 字段链 CHN-8 = 第 13 处
- AST 锁链延伸第 13 处
- owner-only ACL 锁链 16 处一致
- NotifPref 三向锁 (server + client + bitmap byte-identical)
- bitmap 不互扰 (改 pref 不动 collapsed/mute)
- 文案 byte-identical 跟 content-lock + 同义词反向
- 登记 REG-CHN8-001..006
