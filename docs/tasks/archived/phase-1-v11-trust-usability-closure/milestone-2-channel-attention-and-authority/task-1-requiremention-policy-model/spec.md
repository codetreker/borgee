# Spec Brief: requireMention Policy Model

## 0. Constraints

Task contract: add the server-side per-channel `requireMention` policy model for agent channel attention. This task owns the data model, API authority checks, policy resolution, tests, and current-doc sync needed for inherit/on/off semantics.

Blueprint anchors:

- `MR-1` (`docs/blueprint/next/migration-analysis.md` section 3.3): per-channel `requireMention` uses inherit / on / off semantics; channel owners cannot expand external-agent attention or capability; agent owners may opt into broader delivery; setting changes do not backfill history.
- `PS-1` (`docs/blueprint/next/migration-analysis.md` section 6.1): preserve server-side privacy/security controls and agent capability owner authority without adding user-facing privacy/compliance product scope.

Dependency base:

- Canonical Milestone 2 start is authorized by the user as a parallel slot.
- This task is independent of Milestone 1 task 6 Helper pull/lease/result and the remaining Helper/OpenClaw implementation because it only touches channel/message/mention policy surfaces.

## 1. Segmentation

Segment A: policy storage.
Add a durable per-channel member policy slot with exactly `inherit`, `on`, or `off`. Existing memberships default to `inherit` so current behavior remains unchanged until an explicit policy is set.

Segment B: policy resolution.
Resolve effective attention for an agent in a channel from the agent-owner global `users.require_mention` ceiling plus the channel member override. `inherit` follows the agent owner setting. `on` requires mention and is always allowed because it reduces delivery. `off` is allowed only when the agent owner has globally opted into broader delivery; otherwise it must be rejected or ignored fail-closed.

Segment C: API authority.
Expose a user-rail API for setting the channel member policy. The caller must be a channel manager for that channel. The target must be an agent member of the channel. Cross-owner or cross-channel access must fail safely, and the API must not let channel owners broaden external-agent attention beyond owner authorization.

Segment D: mention/message integration.
Use the effective policy at message creation so unmentioned agent targets are not broadened unless their owner permits it and the channel policy resolves to off. Explicit mention routing remains valid and cross-channel mention validation remains unchanged.

Segment E: migration and current docs.
Add a forward-only migration and legacy bootstrap compatibility column. Update current server/client/security docs to describe current implemented policy authority and the remaining client-control work.

## 2. Carry-Over

Carry into later tasks, but do not solve here:

- `@Everyone` fanout, rate limits, loop prevention, and recipient computation.
- Client settings UI for users to view or change the policy.
- Channel management surface, allowed-action rules, and broader channel authority work.
- Notification, collapse, sort, history backfill, or broad visual redesign.
- Offline fallback behavior beyond preserving current mention privacy rules.

## 3. Reverse Checks

- If a channel owner can set `off` for an agent whose owner globally requires mention, the task violates the owner-ceiling boundary.
- If a non-manager can update another member's attention policy, the task violates channel authority.
- If policy changes backfill historical messages or mutate old mention rows, the task exceeds scope.
- If the implementation treats `@Everyone` as part of this policy, the task exceeds scope.
- If user-facing privacy/compliance product surfaces are added, the task violates `PS-1`.

## 4. Out Of Scope

- No `@Everyone` implementation.
- No client mention-control UI.
- No channel management surface or owner-transfer/delete/archive rules.
- No notification/collapse/sort rewrite.
- No history sweep or mention backfill.
- No admin route or user-facing privacy/compliance expansion.
