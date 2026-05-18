# Stance: Channel Management Surface

1. The task creates a place to inspect channel ownership and membership; it does not create channel action authority.
2. Settings is the chosen entry point because the blueprint allows Settings placement and it avoids Milestone 3 sidebar/footer ownership.
3. The current user can see channels they created separately from channels they joined. Created channels are not duplicated in the joined-only list.
4. The server-authoritative channel list remains the source of truth. Client grouping is display-only and does not broaden ACL, membership, ownership, or privacy semantics.
5. The surface may show channel name, visibility, topic, and member count when those fields already arrive in the authorized channel list. It must not expose message bodies, private file content, or hidden channel names.
6. Leave/delete/archive/owner-transfer behavior is explicitly deferred to tasks 5 and 6. This task must not add disabled action buttons that imply unresolved policy.
7. Notification, collapse, sort, private indicator, and broad visual redesign work stay out unless a later task reopens them.

Blacklist grep for review:

- `leaveChannel(` under the new management component should not appear.
- `deleteChannel(` under the new management component should not appear.
- `archiveChannel(` under the new management component should not appear.
- `sidebar-footer` should not be modified for this task.
