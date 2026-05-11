# CHN-10 + CHN-14 — channel description + edit history endpoints contract

> **单一来源 pointer.** Schema in
> `packages/server-go/internal/migrations/chn_14_1_channels_description_edit_history.go` (v=44).
> Owner-only PUT handler in `packages/server-go/internal/api/chn_10_description.go`.
> Owner-only + admin readonly history GET handlers in
> `packages/server-go/internal/api/chn_14_description_history.go`.
> Routes are registered at server boot via `CHN10DescriptionHandler.RegisterUserRoutes` +
> `CHN14DescriptionHistoryHandler.RegisterUserRoutes/RegisterAdminRoutes`
> in `packages/server-go/internal/server/server.go`.
> Store single-source entry point `Store.UpdateChannelDescription` in
> `packages/server-go/internal/store/queries.go`.

## Why

CHN-10 #561 ships owner-only channel description (= channels.topic 列, 复
用 CHN-2 既有列). CHN-14 续 — forward-only audit history
JSON array on the same row (channels.description_edit_history TEXT NULL),
reusing the nullable `ALTER ADD COLUMN` pattern already used by eight
milestones: AL-7.1, HB-5.1, AP-1.1, AP-3.1, AP-2.1, DM-7.1, CV-6.1, and
CHN-14.1. 不另起 history table.

## 原则 (chn-10-spec.md §0 + chn-14-spec.md §0)

- **① schema v=44 ALTER ADD nullable.** description_edit_history TEXT
  NULL on channels (no separate table). Migration `chn_14_1_channels_
  description_edit_history` registry name is part of the migration contract.
  老 channel 行保持原值; `NULL` means no history.
- **② UpdateChannelDescription 单一来源.** PUT /channels/:id/description 走
  store.UpdateChannelDescription entry point: SELECT old topic + edit_history
  → JSON append `{old_content, ts, reason='unknown'}` → UPDATE atomic.
  Production writes to `channels.topic` for this API should stay behind that
  entry point; see QA notes for repository-search coverage.
- **③ owner-only ACL.** PUT + GET history user-rail 走
  `channel.CreatedBy == user.ID` authorization rule (member-level → 403); admin-rail
  GET history readonly (admin access does not add PATCH/DELETE; ADM-0 §1.3).
- **④ UI copy contract** (chn-14-content-lock.md §1):
  - modal title `编辑历史` (matches DM-7 #558 EditHistoryModal
    跨 milestone)
  - empty state `暂无编辑记录` (CHN-14 设计 ⑥ 显式空态; DM-7 设计是空
    return null — 真分歧)
  - 行 action `: 修改了说明` (CHN-14 独有, per-edit 显式)
  - reject 同义词 `History/Audit/Log/记录/日志/审计/回退/恢复`
- **⑤ Reason value compatibility.** `reason='unknown'` uses the same literal
  value as DM-7 #558 / AL-7 SweeperReason / HB-5, so CHN-14 does not
  introduce a new reason value.
- **⑥ No async audit queue for this API.** `pendingDescriptionAudit`,
  `descriptionHistoryQueue`, and `deadLetterDescriptionHistory` are not part
  of the design; see QA notes for repository-search coverage.

## Schema (v=44 ALTER ADD)

| Column | Type | Notes |
|---|---|---|
| ... existing columns ... | (CHN-1.1 + CM-1 + CHN-3.1 + ...) | unchanged |
| `topic` | `TEXT NOT NULL DEFAULT '' size:500` | CHN-2 既有 — 实际持有 description (CHN-10 写, CHN-2 既有 PUT /topic member-level path 不动) |
| `description_edit_history` | `TEXT NULL` | CHN-14.1 v=44 — JSON array `[{old_content, ts, reason}]`; NULL = 无历史 / existing rows keep their prior data |

Migration is forward-only, idempotent via `hasColumn` guard. Existing rows
preserve verbatim with `description_edit_history=NULL`.

## Endpoints

### PUT /api/v1/channels/{channelId}/description (CHN-10)

```
PUT /api/v1/channels/{channelId}/description
Authorization: <session cookie>
Content-Type: application/json

{
  "description": "<= 500 chars"
}
```

ACL:
- No auth → **401 Unauthorized**
- Authenticated non-owner (channel.CreatedBy != user.ID) → **403** `Only
  the channel owner can update description`
- channel not found → **404** `Channel not found`

Validation:
- `description.length > 500` → **400** `Description must be 500 characters
  or less` (DescriptionMaxLength const + GORM size:500 + client
  DESCRIPTION_MAX_LENGTH share the same 500-character limit)

Side-effects on success (200):
- `Store.UpdateChannelDescription(channelID, newDescription)` single-source entry point:
  SELECT old topic + edit_history → JSON append `{old_content, ts,
  reason='unknown'}` → UPDATE topic + description_edit_history.
- **idempotent** — same-content PUT 不入 history (跟 DM-7 #558 同精神).
- 不发 system message (owner action 不进入 message broadcast flow).
- 不 push WS frame (CHN-10 设计 ⑤ — client 下次 GET pull).

Response body: existing channel JSON payload (含 topic 新值).

### GET /api/v1/channels/{channelId}/description/history (CHN-14 owner-only)

```
GET /api/v1/channels/{channelId}/description/history
Authorization: <session cookie>
```

ACL:
- No auth → **401 Unauthorized**
- Authenticated non-owner → **403** `Only the channel owner can view edit
  history`
- channel not found → **404** `Channel not found`

Response body:
```json
{
  "history": [
    {"old_content": "<previous topic>", "ts": 1700000000000, "reason": "unknown"}
  ]
}
```

- `history` is forward-only JSON array, append-only.
- Empty / NULL → `[]` (server-side store layer pre-normalized).
- `reason='unknown'` stays unchanged from DM-7 #558 / AL-7 / HB-5.

### GET /admin-api/v1/channels/{channelId}/description/history (CHN-14 admin readonly)

Same response payload as user-rail GET, no owner-only check (admin
可见全 org). Admin access does not add PATCH/DELETE; admin can view audit
history but cannot directly modify it through this API (ADM-0 §1.3).

## Cross-Milestone Compatibility

- ALTER ADD COLUMN nullable follows the same pattern across eight milestones
  (DM-7.1 + AL-7.1 + HB-5.1 + AP-1.1 + AP-3.1 + AP-2.1 + CV-6.1 + CHN-14.1).
- UpdateChannelDescription 单一来源模式跟 DM-7 #558 UpdateMessage 单一来源 一致.
- owner-only ACL follows the same channel-owner authorization pattern as
  CHN-10 #20 + DM-7 #19.
- audit inline JSON 列模式 (跟 DM-7 #558 设计 ⑤ 同精神, 不入 admin_actions).
- 文案 `编辑历史` matches DM-7 EditHistoryModal + CHN-14
  DescriptionHistoryModal (跨 modal 一致).

## QA Notes

- Repository search should show no production `inline UPDATE channels.*topic`
  writes outside the CHN-10/CHN-14 single-source path.
- Repository search should show no production references to
  `pendingDescriptionAudit`, `descriptionHistoryQueue`, or
  `deadLetterDescriptionHistory`.
- Admin API coverage should verify GET-only behavior for description history;
  PATCH/DELETE routes remain out of scope.

## 不在范围

- 单条 history 删/编 (forward-only 设计).
- 非 description 字段 audit (CHN-2 既有 PUT /topic member-level path 不挂).
- 跨 org admin 全局 history (留 v3 — 仅同 org admin readonly).
- audit retention 自动清理 (留 v3 跟 AL-7 同期统一).
- diff render 新旧字符串对比 (留 v3 — v0 仅 stored `old_content` value).
