# Milestone 1: Helper / OpenClaw Bounded Actuator

## Capability Goal

Complete one user-facing Helper/OpenClaw loop: enroll Helper once, authorize bounded typed jobs, let Helper pull/lease/result through local policy, settle logs/revoke truthfully, install/configure OpenClaw, bind the Borgee plugin channel, keep services reliable, and show Configure OpenClaw terminal UI states.

## Remapped Prior Structure

This milestone replaces the old Phase 1 milestone chain:

- `phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status`
- `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop`
- `phase-1-helper-openclaw-onboarding/milestone-3-configure-openclaw-closure`

Those folders remain the detailed task history and task execution homes. This file is the authoritative coarse milestone grouping.

## Acceptance Boundary

Accepted by this milestone:

- Helper enrollment/status, credential lifecycle, status UI, enqueue authority, and outbound service prerequisites accepted through PR #934, #936, #937, #938, and #939.
- Remaining Helper pull/lease/result, local policy/manifest/sandbox, terminal logs/revoke settlement, OpenClaw install/config jobs, plugin channel binding, service lifecycle reliability, and Configure OpenClaw terminal UI.
- Strict Helper/Remote Agent rail separation across credentials, grants, enforcement, and UI placement.

Rejected by this milestone:

- Arbitrary host command channel, shell, argv, executable path, script, arbitrary service unit, arbitrary path, or arbitrary network domain.
- Sudo cache, privileged long-lived services, Remote Agent rail reuse, or new privacy/compliance product surface.

## Task Index

| Task | Status | Prior path | Accepted PR / dependency | Parallel? |
|---|---|---|---|---|
| Helper enrollment model and status | ACCEPTED | `phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status/task-1-helper-enrollment-model-and-status` | PR #934 (`547f869`) | no |
| Helper credential rotation and revoke | ACCEPTED | `phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status/task-2-helper-credential-rotation-and-revoke` | PR #936 (`1ca5f95`) | complete |
| Helper status UI and current sync | ACCEPTED | `phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status/task-3-helper-status-ui-and-current-sync` | PR #937 (`2872905`) | complete |
| Job envelope and enqueue authority | ACCEPTED | `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop/task-1-job-envelope-and-enqueue-authority` | PR #938 (`64d56f1`) | complete |
| Helper outbound service prerequisites | ACCEPTED | `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop/task-2-helper-outbound-service-prereq` | PR #939 (`96dc0dc`) | complete |
| Helper pull / lease / result | READY | `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop/task-3-helper-pull-lease-result` | After PR #939 | yes, with local policy if file ownership is clean |
| Local policy / manifest / sandbox profile | READY | `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop/task-4-local-policy-manifest-and-sandbox-profile` | After PR #939 | yes, with pull/lease/result if file ownership is clean |
| Bounded status logs and revoke settlement | BLOCKED | `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop/task-5-bounded-status-logs-and-revoke-settlement` | After pull/lease/result and local policy | no |
| OpenClaw install and agent config jobs | BLOCKED | `phase-1-helper-openclaw-onboarding/milestone-3-configure-openclaw-closure/task-1-openclaw-install-and-agent-config-jobs` | After typed job/policy substrate | no |
| Borgee plugin channel binding job | BLOCKED | `phase-1-helper-openclaw-onboarding/milestone-3-configure-openclaw-closure/task-2-borgee-plugin-channel-binding-job` | After install/config jobs; coordinate with channel authority if needed | yes, after dependency clears |
| Service lifecycle reliability | BLOCKED | `phase-1-helper-openclaw-onboarding/milestone-3-configure-openclaw-closure/task-3-service-lifecycle-boot-crash` | After typed job/policy substrate | yes, after dependency clears |
| Configure OpenClaw terminal UI | BLOCKED | `phase-1-helper-openclaw-onboarding/milestone-3-configure-openclaw-closure/task-4-configure-openclaw-terminal-ui` | After install/config, binding, and lifecycle work | no |

Next execution should start from the two READY tasks when Teamlead confirms disjoint write ownership. If only one task can be dispatched first, use `task-3-helper-pull-lease-result` as the first resume point because terminal settlement depends on the pull/lease/result contract.

## Exit Gates

- Configure OpenClaw uses only schema-bound typed jobs and pre-authorized bounded categories.
- Helper pull/lease/result and local policy both reject stale credentials, revoked enrollment, unknown job types, extra fields, and out-of-allowlist paths/domains/service IDs.
- Status, logs, and revoke/uninstall race settlement cannot make failed, denied, or revoked work look successful.
- Long-lived Helper/OpenClaw services stay non-sudo; any privileged installer remains short-lived and visible.
