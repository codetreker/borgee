# Spec Brief: Client Mention Controls

## 0. Constraints

Task contract: make channel mention delivery behavior understandable and controllable in the client after the server-side `requireMention` policy and `@Everyone` fanout behavior are available.

Dependency base:

- Task 1 is satisfied by PR #949 (`c25ef60`), which added per-channel agent `requireMention` policy and the policy update API.
- Task 2 is satisfied by PR #951 (`3659ce1`), which made `@Everyone` server-computed, ACL-filtered, rate-limited, and agent-loop guarded.
- Task 4 is already merged as PR #948 and provides the settings channel-management surface used here, but Task 3 does not depend on unmerged channel authority work.

## 1. Product Slice

Users can open the Settings channel tab, expand a channel's mention settings, see that `@Everyone` is server-computed from current members, and view agent attention policy state. Users with `channel.manage_members` can update an agent member's policy among `inherit`, `on`, and `off`; server authority remains final and rejects owner-ceiling violations.

## 2. Implementation Scope

- Add client API/types for `PUT /api/v1/channels/{channelId}/members/{userId}/require-mention`.
- Include `effective_require_mention` in channel member listing so the client can render truthful current delivery state.
- Add an expandable mention settings panel to the settings channel-management row.
- Stop sending client-supplied recipient id arrays from client message sends; the server parses explicit `<@id>` content and computes `@Everyone` recipients.
- Update current docs and task evidence.

## 3. Out Of Scope

- No new server fanout semantics, rate-limit changes, ACL changes, notification center rewrite, owner transfer, archive/delete/leave controls, history backfill, or broad visual redesign.
- No client-side recipient picker for `@Everyone`; the server remains the source of recipient truth.
