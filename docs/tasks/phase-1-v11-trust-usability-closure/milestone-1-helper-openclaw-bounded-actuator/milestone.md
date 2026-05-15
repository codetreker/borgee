# Milestone 1: Helper / OpenClaw Bounded Actuator

## Capability Goal

Complete one user-facing Helper/OpenClaw loop: enroll Helper once, authorize bounded typed jobs, let Helper pull/lease/result through local policy, settle logs/revoke truthfully, install/configure OpenClaw, bind the Borgee plugin channel, keep services reliable, and show Configure OpenClaw terminal UI states.

## Canonical Task Home

This milestone is the canonical execution home for the Helper/OpenClaw bounded actuator chain. Accepted task artifacts and progress from the previous finer-grained Helper/OpenClaw planning shape have been moved here so that no old phase directory remains live.

Historical PR evidence is summarized in `accepted-history.md`. Accepted task execution is recorded in the task directories listed below.

## Acceptance Boundary

Accepted by this milestone:

- Helper enrollment/status, credential lifecycle, status UI, enqueue authority, outbound service prerequisites, Helper pull/lease/result, and local policy/manifest/sandbox accepted through PR #934, #936, #937, #938, #939, #943, and #942.
- Configure OpenClaw terminal UI closure on top of accepted terminal logs/revoke settlement, OpenClaw install/config jobs, plugin channel binding, and service lifecycle reliability.
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
| Bounded status logs and revoke settlement | ACCEPTED | `task-8-bounded-status-logs-and-revoke-settlement` | PR #954 (`419c5bf`) | complete |
| OpenClaw install and agent config jobs | ACCEPTED | `task-9-openclaw-install-and-agent-config-jobs` | PR #956 (`5575b53`) | complete |
| Borgee plugin channel binding job | ACCEPTED | `task-10-borgee-plugin-channel-binding-job` | PR #958 (`ad50575`) | complete |
| Service lifecycle reliability | ACCEPTED | `task-11-service-lifecycle-boot-crash` | PR #963 (`d8d179e`) | complete |
| Configure OpenClaw terminal UI | ACCEPTED | `task-12-configure-openclaw-terminal-ui` | PR #964 (`3450d8c`) | complete |

Milestone 1 is accepted. `task-8-bounded-status-logs-and-revoke-settlement` is accepted through PR #954 (`419c5bf`), `task-9-openclaw-install-and-agent-config-jobs` is accepted through PR #956 (`5575b53`), `task-10-borgee-plugin-channel-binding-job` is accepted through PR #958 (`ad50575`), `task-11-service-lifecycle-boot-crash` is accepted through PR #963 (`d8d179e`), and `task-12-configure-openclaw-terminal-ui` is accepted through PR #964 (`3450d8c`).

## Exit Gates

- Configure OpenClaw uses only schema-bound typed jobs and pre-authorized bounded categories.
- Helper pull/lease/result and local policy both reject stale credentials, revoked enrollment, unknown job types, extra fields, and out-of-allowlist paths/domains/service IDs.
- Status, logs, and revoke/uninstall race settlement cannot make failed, denied, or revoked work look successful.
- Long-lived Helper/OpenClaw services stay non-sudo; any privileged installer remains short-lived and visible.
