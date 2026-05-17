# Settings, Channel Management, And Admin-Awareness Sketch

## Purpose

This sketch is an Interaction And Layout Reference for the user settings sidepane, channel-management overview, and admin-awareness content. It does not define product behavior, implementation contracts, privacy policy, copy authority, or verification status.

## Surface

Settings is a global sidepane in the user SPA. It has local tabs for privacy/admin-awareness, channel management, and runtime launch points. The privacy tab can show user-owned admin-impact metadata, impersonation grant state, and the user's current capability grants without creating an admin session or mounting the admin SPA. The channel tab shows channels the current user created and channels they joined, a per-row `删除` button for creators with `channel.delete` permission (soft delete via `Store.SoftDeleteChannel`), and expandable mention delivery controls for agent channel members. The runtime tab launches Remote Nodes and Helper Status without merging their rails.

## Interaction Model

- The user opens settings from the shell navigation rail.
- Settings uses the same sidepane navigation model as agents, invitations, workspaces, remote nodes, and Helper status.
- The Settings tab state is local to the settings sidepane; switching between Privacy, Channels, and Runtime does not alter app-level sidepane routing.
- Runtime entries open the existing Remote Nodes and Helper Status sidepanes through the shell view selector. They are distinct launch points, not a shared credential or host-management surface.
- Channel management reads the authorized channel list already held in app state. It groups non-DM channels by explicit `created_by` and `is_member` fields.
- Mention delivery controls load channel members on demand, show `@Everyone` as server-computed, and update agent `requireMention` policy through the user rail when the signed-in user has `channel.manage_members`.
- Channel management surfaces a per-row `删除` button for the current user when the row represents a non-DM non-general channel they created AND `useCan('channel.delete', channelId)` resolves true. Clicking opens the shared `ConfirmDeleteModal`; confirm calls `deleteChannel()` (`DELETE /api/v1/channels/{id}`, soft delete, broadcasts `channel_deleted`). Failure surfaces as a toast and leaves the modal closeable. Success dispatches `REMOVE_CHANNEL`, toasts `#name 已删除`, and (if the user was viewing the deleted channel) dispatches `SET_CURRENT_CHANNEL` to `#general`. If the channel disappears from `state.channels` between modal-open and confirm (e.g. server `channel_deleted` push from another session), the modal auto-closes.
- Admin-awareness content is scoped to the signed-in user.
- Capability visibility is scoped to the signed-in user and is rendered by the same `PermissionsView` surface that reads `/api/v1/me/permissions`. If that endpoint denies access, Settings shows a local forbidden state without rendering response-body details or turning the client view into an authorization decision.
- Grant state can affect a shell-level banner, but the settings form state remains local to the surface.

## Layout Sketch

```
+──────────────────────────────────────────────+
│  Settings                              [Back] │
├──────────────────────────────────────────────┤
│  [Privacy] [Channels] [Runtime]               │
│                                              │
│  Privacy                                      │
│                                              │
│  Admin visibility                            │
│  - Account and channel metadata              │
│  - No message, file, or artifact body view   │
│    unless a user-controlled grant is active  │
│                                              │
│  Temporary support grant                     │
│  Current status: not granted                 │
│  [Grant 24h]                                 │
│                                              │
│  Admin impact history                        │
│  No recent admin impact records              │
│                                              │
│  Capability grants                           │
│  No grants / granted capability rows         │
│                                              │
│  Channels tab                                │
│  Created by me                               │
│  - #ops         private      3 members       │
│    [Mention settings]                         │
│    @Everyone: server computed                 │
│    BuildBot: needs @ mention       [inherit]  │
│    Leave: creator cannot leave               │
│    Delete: available                         │
│    Archive: available                        │
│    Transfer: unavailable                     │
│  Joined by me                                │
│  - #support     public       8 members       │
│    Leave: available                          │
│    Delete/archive/transfer: unavailable      │
│                                              │
│  Runtime                                      │
│  [Remote Nodes]  Remote Agent file proxy     │
│  [Helper Status] Helper actuator enrollment  │
+──────────────────────────────────────────────+
```

## Architecture Notes

- This is a user rail surface backed by user endpoints, not an admin SPA page.
- The capability section is visibility only. Server capability checks remain authoritative; Settings does not make authorization decisions.
- The channel-management tab exposes mention delivery controls and a single per-row `删除` button (soft delete for channel creators with `channel.delete` permission). It does not execute leave, archive, owner-transfer, notification, collapse, sort, pin, group, or private-indicator controls.
- Server-side user-rail mutations remain authoritative: creator-owned channels cannot be left, delete/archive require the channel creator as well as permission state, managers must be channel members, creator removal is rejected, and cross-org management attempts fail closed.
- `@Everyone` has explanatory copy only. The client cannot select broadcast recipients; message send payloads omit recipient id arrays and the server computes recipients from membership.
- Agent `requireMention` selects are disabled unless the user has channel member management authority. Server policy checks remain authoritative and can reject `off` when the agent owner has not allowed broader delivery.
- Created channels appear in the created section only; joined channels created by someone else appear in the joined section. DM channels are outside this surface.
- Self-created or owned channels do not expose leave as an available action anywhere. Joined-only non-general channels can leave via the channel header (outside Settings). Settings only exposes `删除` for owned non-general non-dm channels with `channel.delete` permission; `archive` and `owner-transfer` are not exposed in Settings.
- Notification, collapse, sort, pin, group, and private-indicator controls are outside this Settings channel-management surface.
- Runtime entries are navigation-only. Remote Agent node tokens, Helper enrollment credentials, host grants, and enforcement checks remain owned by their existing rails.
- The admin privacy/audit module owns the durable audit projection and current limitations.
- The shell may show a global banner when user-owned grant state is active.
- Settings should not become a viewer for admin-wide audit data.

## Implementation Anchors

- `packages/client/src/components/Settings/SettingsPage.tsx`: Settings sidepane composition.
- `packages/client/src/App.tsx`: Shell view selector callbacks for Settings Runtime launch entries.
- `packages/client/src/components/Settings/ChannelManagementSurface.tsx`: created/joined channel overview with row-local mention settings entry points.
- `packages/client/src/components/Settings/ChannelMentionControls.tsx`: channel member policy controls and `@Everyone` authority copy.
- `packages/client/src/components/PermissionsView.tsx`: signed-in user's capability visibility states and capability-row rendering.
- `packages/client/src/lib/channelManagement.ts`: non-DM created/joined grouping helper, `canLeaveChannel` (consumed by `ChannelView`), and `canDeleteChannel` (consumed by the Settings 频道 tab).
- `packages/client/src/lib/api.ts`: user rail request helper for signed-in user permission data.

## Related Docs

- [../feature-surfaces.md](../feature-surfaces.md)
- [../ui-map.md](../ui-map.md)
- [../../admin/privacy-audit.md](../../admin/privacy-audit.md)
- [../../admin/spa.md](../../admin/spa.md)
