# task-10-settings-channel-delete-button

Purpose:
- Reopen the Settings 频道 tab to expose a real `删除` (soft delete) button for channel creators, replacing the read-only action-availability matrix from task-5/task-6.

Scope:
- `ChannelManagementSurface.tsx` renders a per-row `删除` button gated by `canDeleteChannel(channel, currentUserId, useCan('channel.delete', channel.id))`.
- Click → `ConfirmDeleteModal` → existing `deleteChannel()` (`DELETE /api/v1/channels/{id}`, server already soft-delete + broadcasts `channel_deleted`).
- Drop the 4-row read-only matrix (`leave` / `delete` / `archive` / `owner-transfer`) and the supporting `buildChannelAllowedActionRules` helper.
- Row layout: header (`#name` + visibility chip + member count) left, delete button `margin-left:auto` anchored right.
- WS race guard: auto-close `ConfirmDeleteModal` if `pendingDelete` disappears from `state.channels` (server `channel_deleted` push between modal open and confirm).

Out of scope:
- `leave`, `archive`, `owner-transfer` actions — still not exposed (no backend for transfer; leave / archive intentionally left to non-Settings entry points).
- ConfirmDeleteModal a11y rewrite (no `role="dialog"` / focus trap) — pre-existing, shared component, separate PR.
- `Toast` `aria-live` rewrite — pre-existing, separate PR.
- `--danger` token contrast lift to WCAG AA — theme-level, separate PR.

Depends on:
- `task-4-channel-management-surface` (surface skeleton, kept)
- `task-5-channel-allowed-action-rules` (matrix removed by this task — see Lock Overrides below)
- `task-6-channel-authority-checks` (server enforcement, reused unchanged)

Blueprint anchors:
- `CH-1`: `docs/blueprint/next/migration-analysis.md` §4.3 (channel management v1)
- Server endpoint: `packages/server-go/internal/api/channels.go:839` `handleDeleteChannel` (general/dm/non-creator/cross-org rejections + `store.SoftDeleteChannel` + `channel_deleted` event)

Acceptance slice:
- Creator with `channel.delete` permission on a non-general non-dm channel sees `删除` button; confirms in modal; row disappears, toast `#name 已删除` fires, server stamps `deleted_at`. Non-owner / `#general` / dm channels: no button. Failure: toast surfaces server error, modal stays closeable.

Parallelism:
- Runs after task-4 / task-5 / task-6. Sole code edit lives in the user-rail Settings surface.

Sensitive paths:
- user-facing destructive action (channel delete). Server re-checks ownership; UI button visibility is a UX hint only, not authorization.

## Lock Overrides

This task explicitly supersedes the following reverse locks from earlier accepted tasks. The locks were correct for their milestones; this task narrows them to "delete is now a real button; leave/archive/owner-transfer remain absent":

- task-5 `content-lock.md` §31 ("No Settings action item should be a `button` in this task") — superseded for `delete` only; leave/archive/owner-transfer chips are removed entirely rather than promoted to buttons.
- task-5 `content-lock.md` §32 ("No Settings channel-management component should import `leaveChannel`, `deleteChannel`, or `archiveChannel`") — superseded for `deleteChannel` only; `leaveChannel` / `archiveChannel` still must not be imported by Settings.
- task-6 `stance.md` §6 ("Settings remains a read-only action-availability surface in this task") — superseded: Settings now executes channel delete for creators with permission.
- task-6 `stance.md` §12 ("`ChannelManagementSurface.tsx` still must not call `leaveChannel`, `deleteChannel`, or `archiveChannel`") — superseded for `deleteChannel`.
- task-6 `content-lock.md` §14 ("Settings channel management remains read-only and must not render mutation buttons for `leave`, `delete`, `archive`, or `owner-transfer`") — superseded for `delete`; the other three remain locked out.
