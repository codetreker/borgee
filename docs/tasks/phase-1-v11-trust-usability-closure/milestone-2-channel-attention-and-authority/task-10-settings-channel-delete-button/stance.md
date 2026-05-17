# Stance

1. The Settings 频道 tab is the canonical user-rail home for channel delete. The button is shown only to creators whose server-side permission set includes `channel.delete` for that channel; the server remains the authority and re-checks on every request.
2. Soft delete via `Store.SoftDeleteChannel` is sufficient as the user-rail action. There is no restore endpoint today, so the UI must not promise recovery.
3. `leave`, `archive`, `owner-transfer` are deliberately absent from Settings. They are either out of milestone-2 scope (transfer) or already reachable through other entry points (leave / archive via channel header). Re-exposing them in Settings needs its own task.
4. The display-only 4-action availability matrix (task-5) is removed wholesale. A button that does the action is more truthful than a chip that explains why something couldn't happen.

## Reverse Checks

- `ChannelManagementSurface.tsx` may import and call `deleteChannel`. It must NOT import `leaveChannel` or `archiveChannel`.
- The Settings 频道 tab MUST NOT render mutation buttons for `leave`, `archive`, or `owner-transfer`.
- Header copy MUST NOT claim restore/undelete is available unless a restore endpoint exists in `packages/server-go`.
- The 4-action `data-action="leave|delete|archive|owner-transfer"` matrix is gone; new tests rely on a single `[data-action="delete"][data-channel-id="..."]` button selector.
- `canLeaveChannel` export remains (consumed by `ChannelView.tsx:69`); `buildChannelAllowedActionRules` and friends are removed.
