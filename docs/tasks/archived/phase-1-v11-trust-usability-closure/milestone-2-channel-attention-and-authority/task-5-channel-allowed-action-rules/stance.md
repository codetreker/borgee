# Stance

1. Allowed-action display is a truthfulness layer, not authorization. Server authority remains the source of truth and is deferred to Task 6.
2. Owners and channel creators must not see a leave affordance for channels they created. The UI should explain that creator-owned channels cannot be left instead of showing a misleading leave control.
3. Joined-only members may see leave as available, but Settings does not execute the leave mutation in this task.
4. Delete and archive availability are explicit owner-created-channel rules, but Settings still does not expose destructive mutation buttons in this task.
5. Owner transfer is explicitly unavailable for v1.
6. The channel management surface stays inside Settings. No sidebar/footer production entry or private-indicator treatment is part of this task.

## Reverse Checks

- `leaveChannel(` should not appear in `packages/client/src/components/Settings/ChannelManagementSurface.tsx`.
- `deleteChannel(` should not appear in `packages/client/src/components/Settings/ChannelManagementSurface.tsx`.
- `archiveChannel(` should not appear in `packages/client/src/components/Settings/ChannelManagementSurface.tsx`.
- `task-6-channel-authority-checks`, `task-8-private-indicator-visual-treatment`, and `task-9-sidebar-state-collision-regression` should not be edited by this PR.
