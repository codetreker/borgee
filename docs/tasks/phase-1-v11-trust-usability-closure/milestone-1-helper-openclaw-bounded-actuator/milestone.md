# Milestone 1: Helper / OpenClaw Bounded Actuator

## Capability Goal

Complete one user-facing Helper/OpenClaw loop: enroll Helper once, authorize bounded typed jobs, let Helper pull/lease/result through local policy, settle logs/revoke truthfully, install/configure OpenClaw, bind the Borgee plugin channel, keep services reliable, and show Configure OpenClaw terminal UI states.

## Canonical Task Home

This milestone is the canonical execution home for the Helper/OpenClaw bounded actuator chain. Accepted task artifacts and progress from the previous finer-grained Helper/OpenClaw planning shape have been moved here so that no old phase directory remains live.

Historical PR evidence is summarized in `accepted-history.md`. Future task execution resumes from the task directories listed below.

## Acceptance Boundary

Accepted by this milestone:

- Helper enrollment/status, credential lifecycle, status UI, enqueue authority, outbound service prerequisites, Helper pull/lease/result, and local policy/manifest/sandbox accepted through PR #934, #936, #937, #938, #939, #943, and #942.
- Remaining terminal logs/revoke settlement, OpenClaw install/config jobs, plugin channel binding, service lifecycle reliability, and Configure OpenClaw terminal UI.
- Strict Helper/Remote Agent rail separation across credentials, grants, enforcement, and UI placement.

Rejected by this milestone:

- Arbitrary host command channel, shell, argv, executable path, script, arbitrary service unit, arbitrary path, or arbitrary network domain.
- Sudo cache, privileged long-lived services, Remote Agent rail reuse, or new privacy/compliance product surface.

## Task Index

| Task | Status | Canonical path | Accepted PR / dependency | Parallel? |
|---|---|---|---|---|
| Helper enrollment model and status | ACCEPTED | `task-1-helper-enrollment-model-and-status` | PR #934 (`547f869`) | no |
| Helper credential rotation and revoke | ACCEPTED | `task-2-helper-credential-rotation-and-revoke` | PR #936 (`1ca5f95`) | complete |
| Helper status UI and current sync | ACCEPTED | `task-3-helper-status-ui-and-current-sync` | PR #937 (`2872905`) | complete |
| Job envelope and enqueue authority | ACCEPTED | `task-4-job-envelope-and-enqueue-authority` | PR #938 (`64d56f1`) | complete |
| Helper outbound service prerequisites | ACCEPTED | `task-5-helper-outbound-service-prereq` | PR #939 (`96dc0dc`) | complete |
| Helper pull / lease / result | ACCEPTED | `task-6-helper-pull-lease-result` | PR #943 (`c2c61e`) | complete |
| Local policy / manifest / sandbox profile | ACCEPTED | `task-7-local-policy-manifest-and-sandbox-profile` | PR #942 (`642fb57`) | complete |
| Bounded status logs and revoke settlement | IMPLEMENTING | `task-8-bounded-status-logs-and-revoke-settlement` | After PR #943 (`c2c61e`) and PR #942 (`642fb57`) | no |
| OpenClaw install and agent config jobs | BLOCKED | `task-9-openclaw-install-and-agent-config-jobs` | After typed job/policy substrate | no |
| Borgee plugin channel binding job | BLOCKED | `task-10-borgee-plugin-channel-binding-job` | After install/config jobs; coordinate with channel authority if needed | yes, after dependency clears |
| Service lifecycle reliability | BLOCKED | `task-11-service-lifecycle-boot-crash` | After typed job/policy substrate | yes, after dependency clears |
| Configure OpenClaw terminal UI | BLOCKED | `task-12-configure-openclaw-terminal-ui` | After install/config, binding, and lifecycle work | no |

Current execution is implementation for `task-8-bounded-status-logs-and-revoke-settlement`. `task-6-helper-pull-lease-result` is accepted through PR #943 (`c2c61e`), and `task-7-local-policy-manifest-and-sandbox-profile` is accepted through PR #942 (`642fb57`).

## Exit Gates

- Configure OpenClaw uses only schema-bound typed jobs and pre-authorized bounded categories.
- Helper pull/lease/result and local policy both reject stale credentials, revoked enrollment, unknown job types, extra fields, and out-of-allowlist paths/domains/service IDs.
- Status, logs, and revoke/uninstall race settlement cannot make failed, denied, or revoked work look successful.
- Long-lived Helper/OpenClaw services stay non-sudo; any privileged installer remains short-lived and visible.
