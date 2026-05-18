# Content Lock

Task 10 turns the Settings 频道 tab from a read-only availability surface into a single-action mutation surface (delete only).

## User-Facing Copy

- Tab header subtitle: `查看你创建或加入的频道；创建者可以删除自己的频道 (soft delete)。`
- Per-row delete button label: `删除`
- Per-row delete button `aria-label`: `删除频道 #{channelName}`
- Confirm modal: reuses shared `ConfirmDeleteModal` ("确定删除 #channelname？此操作不可恢复。")
- Success toast: `#{channelName} 已删除`
- Failure toast: server error message, or `删除失败` fallback

The phrases `可由管理员恢复` / `restore` / `undelete` MUST NOT appear in user copy until a restore endpoint exists. Soft delete is true (`deleted_at` stamp) but not user-recoverable today.

## Reverse Locks

- Settings 频道 tab MUST NOT render mutation buttons for `leave`, `archive`, or `owner-transfer`. Only `delete` is exposed.
- The `data-action="leave"`, `data-action="archive"`, `data-action="owner-transfer"` rule chips and their reason text MUST NOT be reintroduced.
- The `ChannelManagementActionId` / `ChannelAllowedActionRule` / `buildChannelAllowedActionRules` symbols MUST NOT be reintroduced.
