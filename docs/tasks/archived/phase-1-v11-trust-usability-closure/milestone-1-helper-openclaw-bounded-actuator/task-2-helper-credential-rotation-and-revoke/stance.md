# Stance: Helper Credential Rotation And Revoke

## Decision

Helper credential rotation and revoke are authority changes, not status decoration. The server must treat the current Helper credential plus enrolled device id as the only Helper-originated authority for lifecycle status, rotation, and uninstall until later typed-job work adds queue/poll contracts.

## Product Boundary

Task 2 keeps the Phase 1 boundary narrow: a user can trust that stale or revoked Helper authority cannot be used later, but this task does not make Web-side Configure OpenClaw execute. It prepares the authority rail that later enqueue/poll work will consume.

## Security Stance

- Helper credentials remain separate from Remote Agent file-proxy tokens, host grants, user permissions, admin sessions, and plugin/API keys.
- A stale credential is an authentication failure on the Helper rail, not a reason to try a user token, host grant, Remote Agent token, or broader permission check.
- A stale device id is an authority mismatch. Rotation must not silently rebind the enrolled helper device.
- Revoke and uninstall are terminal for future Helper authority unless a future explicitly scoped task designs a new enrollment/re-enrollment flow.
- Raw credential material is returned only at claim/rotation time and is never serialized afterward.

## Privacy Stance

The task preserves `migration-analysis.md` §6.1: no new user-facing privacy/compliance product surface enters this milestone. Backend security boundaries, data minimization, audit/enforcement hooks where already present, and rail separation remain in scope.

## Implementation Posture

Implementation must wait for Dev design review and start with TDD. Tests should prove the negative authority paths first: old credential after rotation, wrong device, revoked enrollment, uninstalled enrollment, Remote Agent token, host grant id, and user token must not authorize Helper lifecycle operations.
