# Stance

1. Channel management authority is server-owned. Client rules are a truthful preview, not authorization.
2. Broad permission rows such as `*` or scoped `channel.delete` are necessary but not sufficient for ownership actions. Delete and archive also require the authenticated user to be the channel creator.
3. Creator-owned channels cannot be left through the user rail. A creator must transfer ownership in a future task or delete/archive where allowed.
4. Member management must not remove the channel creator, and managers must themselves be members of the channel they manage.
5. Cross-org callers fail closed before management mutations can change channel membership or ownership state.
6. Settings remains a read-only action-availability surface in this task. Existing mutation controls outside Settings are tightened to match the same authority boundary.

## Reverse Checks

- `ChannelManagementSurface.tsx` still must not call `leaveChannel`, `deleteChannel`, or `archiveChannel`.
- Owner transfer remains unavailable.
- Task9 sidebar collision regression and private-indicator work remain untouched.
