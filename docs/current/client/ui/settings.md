# Settings, Channel Management, And Admin-Awareness Sketch

## Purpose

This sketch is an Interaction And Layout Reference for the user settings sidepane, channel-management overview, and admin-awareness content. It does not define product behavior, implementation contracts, privacy policy, copy authority, or verification status.

## Surface

Settings is a global sidepane in the user SPA. It has local tabs for privacy/admin-awareness and channel management. The privacy tab can show user-owned admin-impact metadata, impersonation grant state, and the user's current capability grants without creating an admin session or mounting the admin SPA. The channel tab shows channels the current user created and channels they joined, visible allowed-action rules, and expandable mention delivery controls for agent channel members.

## Interaction Model

- The user opens settings from the shell navigation rail.
- Settings uses the same sidepane navigation model as agents, invitations, workspaces, and remote nodes.
- The Settings tab state is local to the settings sidepane; switching between Privacy and Channel does not alter app-level sidepane routing.
- Channel management reads the authorized channel list already held in app state. It groups non-DM channels by explicit `created_by` and `is_member` fields.
- Mention delivery controls load channel members on demand, show `@Everyone` as server-computed, and update agent `requireMention` policy through the user rail when the signed-in user has `channel.manage_members`.
- Channel management derives read-only leave/delete/archive/owner-transfer availability from the current user, channel ownership, membership, and the default-channel rule.
- Admin-awareness content is scoped to the signed-in user.
- Capability visibility is scoped to the signed-in user and is rendered by the same `PermissionsView` surface that reads `/api/v1/me/permissions`. If that endpoint denies access, Settings shows a local forbidden state without rendering response-body details or turning the client view into an authorization decision.
- Grant state can affect a shell-level banner, but the settings form state remains local to the surface.

## Layout Sketch

```
+──────────────────────────────────────────────+
│  Settings                              [Back] │
├──────────────────────────────────────────────┤
│  [Privacy] [Channels]                         │
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
+──────────────────────────────────────────────+
```

## Architecture Notes

- This is a user rail surface backed by user endpoints, not an admin SPA page.
- The capability section is visibility only. Server capability checks remain authoritative; Settings does not make authorization decisions.
- The channel-management tab exposes mention delivery controls and read-only action availability. It does not execute leave, delete, archive, owner-transfer, notification, collapse, sort, pin, group, or private-indicator controls.
- `@Everyone` has explanatory copy only. The client cannot select broadcast recipients; message send payloads omit recipient id arrays and the server computes recipients from membership.
- Agent `requireMention` selects are disabled unless the user has channel member management authority. Server policy checks remain authoritative and can reject `off` when the agent owner has not allowed broader delivery.
- Created channels appear in the created section only; joined channels created by someone else appear in the joined section. DM channels are outside this surface.
- Self-created or owned channels do not expose leave as an available action. Joined-only non-general channels can show leave as available. Owner transfer is unavailable for v1.
- Notification, collapse, sort, pin, group, and private-indicator controls are outside this Settings channel-management surface.
- The admin privacy/audit module owns the durable audit projection and current limitations.
- The shell may show a global banner when user-owned grant state is active.
- Settings should not become a viewer for admin-wide audit data.

## Implementation Anchors

- `packages/client/src/components/Settings/SettingsPage.tsx`: Settings sidepane composition.
- `packages/client/src/components/Settings/ChannelManagementSurface.tsx`: created/joined channel overview with row-local mention settings entry points.
- `packages/client/src/components/Settings/ChannelMentionControls.tsx`: channel member policy controls and `@Everyone` authority copy.
- `packages/client/src/components/PermissionsView.tsx`: signed-in user's capability visibility states and capability-row rendering.
- `packages/client/src/lib/channelManagement.ts`: non-DM created/joined grouping helper and read-only allowed-action rules.
- `packages/client/src/lib/api.ts`: user rail request helper for signed-in user permission data.

## Related Docs

- [../feature-surfaces.md](../feature-surfaces.md)
- [../ui-map.md](../ui-map.md)
- [../../admin/privacy-audit.md](../../admin/privacy-audit.md)
- [../../admin/spa.md](../../admin/spa.md)
