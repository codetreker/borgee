# Design

## Server Authority

Task 6 keeps the existing user-rail routes and adds domain predicates inside `ChannelHandler` after authentication and coarse permission checks:

- `POST /api/v1/channels/{channelId}/leave`: reject DM/general channels as before, then reject channel creators and non-members before deleting a membership row.
- `PUT /api/v1/channels/{channelId}` with `archived`: require same-org, `channel.manage_visibility`, non-DM/non-general channel, and `channel.created_by == current user`.
- `DELETE /api/v1/channels/{channelId}`: retain `channel.delete` middleware, then require same-org and `channel.created_by == current user`.
- `POST /api/v1/channels/{channelId}/members`: retain `channel.manage_members` middleware, then require same-org and caller channel membership before adding a target.
- `DELETE /api/v1/channels/{channelId}/members/{userId}`: require caller membership, protect the channel creator, require same-org and `channel.manage_members` when removing another user, and reject missing target membership.
- `PUT /api/v1/channels/{channelId}/members/{userId}/require-mention`: require caller channel membership before applying the existing manager and agent-owner ceiling checks.

These checks intentionally layer ownership/membership semantics on top of permission rows because permission rows cannot express creator leave, creator removal, or cross-org management leakage by themselves.

## Client Truthfulness

`buildChannelAllowedActionRules(...)` accepts delete/archive permission hints from `useCan(...)`. It still derives leave and owner-transfer rules from channel/current-user state, but delete/archive are only marked available when both ownership and the matching permission state are present.

`ChannelManagementSurface` passes `channel.delete` and `channel.manage_visibility` permission state into the rule helper. `ChannelMembersModal` uses the same owner boundary for destructive controls and does not render a remove button for the channel creator.

## Testing

Focused Go tests cover the red/green authority paths where broad permission rows used to be enough: creator leave, non-member leave, non-creator delete, non-creator archive, and creator removal. Existing channel lifecycle tests are updated so normal leave/delete coverage uses the correct actor.

Focused client tests cover Settings action availability when server permission state is present or absent. Existing channel-management tests keep the Task 4/5 grouping and read-only action contract intact.

## Non-Goals

No new mutation UI is added to Settings. No owner transfer, admin force-delete, notification, collapse, sort, pin, group, private-indicator, or sidebar regression work is included.
