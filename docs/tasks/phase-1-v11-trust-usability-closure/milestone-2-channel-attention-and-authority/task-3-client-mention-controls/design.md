# Design: Client Mention Controls

## Decision

Use the existing Settings channel-management tab as the control surface. Each non-DM channel row gets an expandable mention settings panel. The panel loads channel members on demand, shows `@Everyone` as server-computed, filters policy controls to agent members, and disables policy selects unless the current user has `channel.manage_members` for that channel.

## Files

- `packages/server-go/internal/store/queries.go`
  - Extend `ChannelMemberInfo` and `GetChannelDetail` with `effective_require_mention`.
- `packages/client/src/lib/api.ts`
  - Add `RequireMentionPolicy`, policy update API, and member state fields.
  - Stop serializing message `mentions` recipient arrays.
- `packages/client/src/components/MessageInput.tsx`
  - Stop sending websocket `mentions` recipient arrays; keep local pending mention ids only for optimistic rendering.
- `packages/client/src/components/Settings/ChannelMentionControls.tsx`
  - New expandable settings-row panel for `@Everyone` authority copy and agent policy select controls.
- `packages/client/src/components/Settings/ChannelManagementSurface.tsx`
  - Mount the mention controls in channel rows.
- `packages/client/src/index.css`
  - Add compact settings-row styles.

## Boundary Handling

- The server remains authoritative for target membership, manager permission, agent-only policy targets, and owner-ceiling rejection.
- `off` remains selectable because only the server can know and enforce the agent owner's current global authorization.
- Human members are displayed through existing channel member APIs but are not rendered as policy targets.
- `@Everyone` has explanatory copy only; no broadcast recipient selection or fanout control is added.

## Test Design

Tests were written first and observed failing before production edits:

- Client API test failed because the policy endpoint helper did not exist and `sendMessage` still serialized `mentions` recipient ids.
- Settings surface test failed because the mention controls were not mounted.
- Server API test failed because member listing did not return `effective_require_mention`.

The implementation then made those tests pass without changing Task 2's fanout logic.
