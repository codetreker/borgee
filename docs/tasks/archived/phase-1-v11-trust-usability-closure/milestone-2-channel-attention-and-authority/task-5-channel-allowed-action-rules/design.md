# Design

## Surface

Task 5 extends the existing Settings `频道` tab created by Task 4. Each non-DM channel row keeps its name, visibility, topic, and member count, then adds a read-only action availability list.

The availability list is deliberately not a toolbar. It does not render buttons and does not call mutation APIs. It gives users a truthful preview of the channel-management rules that Task 6 will enforce before any mutation is wired.

## Client Rule Helper

`packages/client/src/lib/channelManagement.ts` owns both grouping and action-rule derivation:

- `buildChannelManagementSections(...)`: existing created/joined grouping.
- `canLeaveChannel(...)`: shared leave predicate used by Settings rows and the active channel header.
- `buildChannelAllowedActionRules(...)`: ordered read-only rules for `leave`, `delete`, `archive`, and `owner-transfer`.

Keeping this in the existing channel-management helper avoids duplicating ownership/membership logic across Settings and ChannelView.

## Rule Mapping

| Action | Available when | Unavailable examples |
|---|---|---|
| `leave` | channel is non-DM, joined, non-general, and not created by current user | creator-owned channel, general, not joined |
| `delete` | channel is non-general and created by current user | general, joined-only channel |
| `archive` | channel is non-general and created by current user | general, joined-only channel |
| `owner-transfer` | never in this task | all rows |

## Existing Channel Header

`ChannelView` already has a leave button. Task 5 updates that button to use `canLeaveChannel(...)`, so creator-owned channels stop exposing a misleading leave affordance outside Settings as well.

## Testing

- Helper tests cover created/self-owned, joined-only, and general-channel rule cases.
- Settings surface tests cover read-only row rendering and confirm action availability is not rendered as mutation buttons.
- Existing focused channel management tests are reused so Task 4 grouping stays intact.

## Non-Goals

- No server route, permission, or ACL changes.
- No owner-transfer implementation.
- No destructive mutation control in Settings.
- No notification/collapse/sort/private-indicator/sidebar/footer work.
