# Acceptance: Helper Credential Rotation And Revoke

## Source Alignment

- Task: `task-2-helper-credential-rotation-and-revoke`
- Milestone: `phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status`
- Blueprint anchors: `remote-actuator-design.md` §1.2, §5, and §10; `migration-analysis.md` §6.1
- Dependency base: task 1 accepted in PR #934 at merge commit `547f869`.

## Segment A: Credential Rotation Lifecycle

Acceptance checks:

- A claimed Helper enrollment can rotate its persistent credential only by presenting the current valid Helper credential and matching `helper_device_id`.
- Rotation returns the new raw credential once and stores only a digest plus lifecycle metadata.
- The new rotated credential plus the same `helper_device_id` can authenticate Helper lifecycle status/heartbeat after rotation and can be used for later task-2 lifecycle authority.
- The previous credential is immediately stale and cannot authenticate heartbeat, rotation, or uninstall.

Negative checks:

- Pending/unclaimed enrollments cannot rotate.
- Rotation does not create a job queue, polling loop, service operation, or Configure OpenClaw execution path.
- Rotation does not accept Remote Agent tokens, host grant ids, user tokens, admin sessions, or user permissions as fallback authority.

## Segment B: Stale Credential And Device Semantics

Acceptance checks:

- Stale credentials fail without mutating `last_seen_at`, credential metadata, revoke/uninstall timestamps, or device binding.
- Wrong or stale `helper_device_id` values fail without rotating credentials or rebinding the enrolled device.
- After rotation, the same enrolled device remains valid with the new credential and can update heartbeat/freshness through the Helper lifecycle rail.
- Failure responses are distinguishable enough for server/helper policy review without leaking raw credential material.

Negative checks:

- A stale device cannot claim to be a new valid Helper through rotation.
- Stale authority cannot collapse into connected, successful, or indefinitely pending visible state.

## Segment C: Revoke Authority

Acceptance checks:

- Owner/org-scoped user revoke is terminal for Helper authority.
- After revoke, old and current Helper credentials cannot heartbeat, rotate, uninstall, or prepare future Helper host-management authority.
- Revoke keeps `revoked` visibly distinct from `offline` and `uninstalled`.

Negative checks:

- Revoke does not rely on best-effort UI-only state.
- Revoke does not grant or reuse Remote Agent, host grant, or user permission authority.

## Segment D: Helper-Originated Uninstall Authority

Acceptance checks:

- Helper-originated uninstall requires the current valid Helper credential and matching enrolled device id.
- Uninstall is terminal for future Helper heartbeat and rotation attempts.
- User-initiated revoke and helper-originated uninstall precedence is deterministic and reviewer-checkable.

Negative checks:

- A revoked enrollment cannot be converted into uninstalled by a stale Helper credential unless design review explicitly chooses and tests a different terminal precedence.
- Uninstall does not perform service lifecycle actions in this task.

## Segment E: API/Data-Model And Rail Separation

Acceptance checks:

- Data-layer and API contracts expose only the metadata needed for lifecycle review and hide raw credentials/digests from serializers.
- Tests cover API and store/datalayer authority paths introduced by the task.
- Reverse checks show Helper credential logic remains separate from `remote_nodes`, host grants, and user permissions.

Negative checks:

- No shared token, shared grant, merged permission check, or Remote Agent credential extension is introduced.
- No typed job envelope, lease, result schema, arbitrary shell, or service manager operation is accepted by this task.

## Segment F: Current-Doc Sync And Progress State

Acceptance checks:

- If accepted behavior changes current architecture, relevant `docs/current` pages are updated for credential rotation, stale semantics, revoke/uninstall authority, and rail separation.
- `progress.md` records implementation evidence, focused verification, docs/current sync status, and final acceptance state in this same task PR.
- The shared task resume state no longer depends on a closure/status follow-up PR for task 1.

Negative checks:

- Current docs must not describe Helper credentials as Remote Agent credentials, runtime-owner credentials, arbitrary command credentials, or post-install sudo authority.
- Task 2 must not be marked accepted until its own implementation and acceptance evidence are complete.
