# Design: Channel Management Surface

## 1. Data Flow

User opens Settings from the existing sidebar footer settings control. Settings renders a new `频道` tab next to the existing privacy tab. Selecting it renders `ChannelManagementSurface`, which reads `state.channels` and `state.currentUser` from `AppContext`. App initialization already calls `actions.loadChannels()`, so the first implementation does not introduce a new API route or fetch loop.

If the channel list is refreshed elsewhere, this surface reflects the same state as the sidebar. It does not mutate channel membership, ownership, sorting, grouping, notification, archive, delete, or visibility.

## 2. Data Model

No database or server schema change. The display model is derived from existing `Channel` fields:

- `created_by` equals `currentUser.id` for created channels.
- `is_member !== false` means the current user is joined.
- Joined-only rows are joined channels whose `created_by` is not the current user.
- DM channels are excluded from management rows because the task scope is channel management, not direct messages.

The implementation may add a small pure classifier, `buildChannelManagementSections(channels, currentUserId)`, to make API/client behavior testable without rendering the entire app.

## 3. API Contract

No new API endpoint. Existing `fetchChannels()` keeps using `GET /api/v1/channels` and returns `{ channels, groups }`. API/client tests verify the management-relevant metadata in a representative channel payload is preserved by `fetchChannels()`.

This task does not add mutation endpoints. Later tasks can add or wire action APIs once allowed-action rules and authority checks are scoped.

## 4. Edge Cases

- No current user: render an empty management surface rather than guessing ownership.
- Created channel that is also joined: show only in created section, not duplicated in joined-only section.
- Non-member preview channel: exclude from joined-only section; include in created only if created by the current user.
- DM channel in app state: exclude from both sections.
- Empty sections: render the locked empty text for each section.
- Private channels: show only metadata already present in the authorized channel list. Do not inspect messages or private content.

## 5. Options Considered

Option A: Settings tab. Chosen. It is explicitly allowed by the blueprint, provides a route/entry through existing Settings navigation, and avoids sidebar/footer ownership reserved for Milestone 3.

Option B: New sidebar/footer button. Rejected because the user explicitly told this task to avoid M3 sidebar/footer production edits.

Option C: In-channel header management entry. Rejected for this first task because it risks coupling management visibility to the selected channel and would make the joined/created overview harder to reach.

## 6. Integration

Files expected to change:

- `packages/client/src/lib/channelManagement.ts`: pure classification helper.
- `packages/client/src/components/Settings/ChannelManagementSurface.tsx`: display-only management surface.
- `packages/client/src/components/Settings/SettingsPage.tsx`: add the `频道` tab and render the surface.
- `packages/client/src/__tests__/ChannelManagementSurface.test.tsx`: component/client behavior.
- `packages/client/src/__tests__/channel-management-api.test.ts`: API metadata and classifier tests.
- `packages/client/src/lib/mainView.ts`: no change expected; Settings remains the sidepane route.
- `packages/client/src/components/Sidebar.tsx`: no sidebar/footer production changes expected.

Docs/current sync should update the Settings/UI current documentation to mention that Settings now owns Privacy and Channel tabs, and that channel management is display-only until allowed-action/authority tasks land.

## Sensitive Task Threat Model

Sensitive paths: channel ACL, ownership visibility, privacy.

Controls:

- The client displays only channels already delivered by the authorized channel list.
- The surface does not introduce mutation controls or bypass server authority.
- The component groups by explicit `created_by` and `is_member` fields; it does not infer hidden memberships or owner powers.
- No message/file/body content is fetched or rendered.
- Later task 6 remains responsible for server/client enforcement for any management action.
