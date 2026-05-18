# Spec: Helper Credential Rotation And Revoke

## Source Alignment

- Task: `task-2-helper-credential-rotation-and-revoke`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Blueprint anchors: `remote-actuator-design.md` §1.2, §5, and §10; `migration-analysis.md` §6.1
- Dependency base: task 1 accepted Helper enrollment/status foundation in PR #934, merge commit `547f869`.

## Goal

Make Helper credential lifecycle, stale credential/device handling, revoke authority, and helper-originated uninstall authority enforceable before any typed job execution exists.

The task extends the accepted Helper enrollment rail. It must keep Helper credentials distinct from Remote Agent file-proxy credentials, host grants, user permissions, and any future job queue credential.

## In Scope

- Rotate the persistent Helper credential for a claimed enrollment and ensure the prior credential becomes unusable.
- Represent stale credential and stale device outcomes as explicit authority failures at the Helper rail.
- Preserve same-device heartbeat freshness after successful rotation.
- Make user-initiated revoke terminal for future Helper authority, including old and current credentials.
- Keep helper-originated uninstall authority bound to the current valid Helper credential and matching helper device id.
- Update API/data-model contracts needed for credential versioning, rotation timestamps, stale outcomes, and terminal revoke/uninstall state.
- Add focused tests before implementation during `bf-task-execute`.
- Sync `docs/current` after accepted behavior changes; if no current-doc change is needed, record the no-op rationale in `progress.md` before acceptance.

## Out Of Scope

- Typed job execution, queue/lease/result behavior, helper polling loop, bounded logs, service lifecycle actions, or OpenClaw configuration closure.
- Any arbitrary shell, argv, executable path, script, service unit name, or local service manager operation.
- Any Remote Agent token reuse, host grant fallback, user permission fallback, or merged Helper/Remote Agent enforcement rail.
- New user-facing privacy/compliance product surfaces, audit dashboards, legal-copy expansion, or admin impact records.
- UI copy or DOM literals. `content-lock.md` is not required for this tasking baseline.

## Behavioral Contract

- A claimed Helper can request credential rotation only with the current persistent Helper credential and the matching `helper_device_id`.
- Rotation returns the new raw credential exactly once, stores only its digest, updates credential lifecycle metadata, and invalidates the previous credential immediately.
- A stale credential must not update `last_seen_at`, rotate again, uninstall, or revive a terminal enrollment.
- A wrong or stale device id must not update `last_seen_at`, rotate, uninstall, or replace the enrolled device through rotation.
- User revoke remains owner/org-scoped and terminal. After revoke, Helper heartbeat, rotate, and uninstall attempts fail without falling back to any other rail.
- Helper-originated uninstall remains terminal and requires the current valid credential plus matching device id.
- Pending/unclaimed enrollments cannot rotate because no persistent Helper credential exists yet.
- Revoked and uninstalled states stay visibly distinct from offline freshness.

## Data And API Boundaries

- Extend `helper_enrollments` rather than creating a parallel authority table unless design review finds rotation history needs a separate table for audit/security reasons.
- Keep raw enrollment secrets and Helper credentials out of serializers, logs, stored rows, and docs/current examples.
- Keep internal owner/org fields server-side; user APIs remain owner/org scoped.
- If exposed over HTTP, rotation belongs on the Helper credential rail, not the user rail, Remote Agent rail, host grant rail, or admin rail.

## Acceptance Shape

Acceptance requires reviewer-checkable evidence that stale, revoked, or uninstalled Helper authority cannot be used for future host-management work and cannot fall back to Remote Agent credentials. Because no job execution exists in this task, evidence focuses on the enrollment/credential authority gates that later enqueue and poll tasks must depend on.
