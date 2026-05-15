# Blueprint Next State

Target version: v1.1 candidate
Last updated: 2026-05-15
Resume from: execute task-start/four-piece for `docs/tasks/phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop/task-1-job-envelope-and-enqueue-authority/`.

This directory tracks planned or in-discussion blueprint work that is not yet accepted into `docs/blueprint/current/`. `current/` remains the implemented-and-accepted product truth. `docs/tasks/` is used only after a next anchor is locked for execution.

## §0 Status Ledger

| Anchor | Topic | Decision | Work | Source issues | Reference | Milestone path | Next action |
|---|---|---|---|---|---|---|---|
| `HB-RA-1A` | Helper bounded actuator product guardrails | LOCKED | IMPLEMENTING | gh#681, gh#659 | `remote-actuator-design.md` §1.1-§1.2; `migration-analysis.md` §2.1 | `docs/tasks/phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status`; `docs/tasks/phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop`; `docs/tasks/phase-1-helper-openclaw-onboarding/milestone-3-configure-openclaw-closure` | Execute through Phase 1 milestones; do not inherit whole-doc draft scope beyond locked guardrails. |
| `HB-RA-1B` | Helper actuator execution contract | LOCKED | IMPLEMENTING | gh#681, gh#659 | `remote-actuator-design.md` §5-§14; `migration-analysis.md` §2.2 | `docs/tasks/phase-1-helper-openclaw-onboarding/milestone-1-helper-enrollment-status`; `docs/tasks/phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop`; `docs/tasks/phase-1-helper-openclaw-onboarding/milestone-3-configure-openclaw-closure` | Carry execution contract into Helper enrollment, typed job policy loop, and Configure OpenClaw closure milestones. |
| `MR-1` | Mention routing granularity and `@Everyone` broadcast | LOCKED | IMPLEMENTING | gh#674, gh#693 | `migration-analysis.md` §3 | `docs/tasks/phase-2-collaboration-channel-control/milestone-1-mention-delivery-controls` | Implement owner-safe per-channel mention delivery and server-authoritative `@Everyone`. |
| `CH-1` | Channel authority and user-side channel management | LOCKED | IMPLEMENTING | gh#685, gh#688, gh#690 | `migration-analysis.md` §4 | `docs/tasks/phase-2-collaboration-channel-control/milestone-2-channel-management-authority`; `docs/tasks/phase-2-collaboration-channel-control/milestone-3-channel-visual-truth` | Implement channel management authority and private-channel visual truth milestones. |
| `CT-1` | Client truthfulness and forbidden-state visibility | LOCKED | IMPLEMENTING | gh#724 | `migration-analysis.md` §5 | `docs/tasks/phase-3-client-truth-navigation/milestone-1-production-surface-truthfulness` | Implement ArtifactComments/ArtifactPanel reachability, Settings `PermissionsView` reachability, and non-leaky forbidden states. |
| `PS-1` | Privacy scope guard | LOCKED | IMPLEMENTING | gh#654 | `migration-analysis.md` §6.1 | all v1.1 milestone folders under `docs/tasks/phase-*` | Carry as locked guardrail: exclude new user-facing privacy/compliance product expansion while preserving existing admin, privacy, security, impersonation, audit/enforcement, data-minimization, capability, and rail-separation controls. |
| `IA-1` | Sidebar footer and account entry IA | LOCKED | IMPLEMENTING | gh#669, gh#670 | `migration-analysis.md` §7 | `docs/tasks/phase-3-client-truth-navigation/milestone-2-sidebar-account-entry` | Implement calmer footer IA and avatar/account entry without rail merge. |

Decision values are `OPEN`, `LOCKED`, or `REOPENED`. Work values are `PENDING`, `IMPLEMENTING`, or `COMPLETED`. Only `LOCKED` anchors may move into `docs/tasks/` Phase/Milestone planning.

The v1.1 selected anchors fit into 3 Phases and 8 user-facing milestones. Each Phase stays within the default limit of 3 milestones; milestone breakdown was accepted in one PR across all planned milestones.

## §1 Iteration Positioning

This next iteration does not rewrite Borgee's product identity. It closes v1 usability and trust gaps discovered after first real use: Helper / remote actuator onboarding, mention routing, channel authority, client truthfulness, privacy scope discipline, and sidebar/account IA.

Default version judgment is minor continuation. The only major-trigger cluster is the helper bounded-actuator work: if the current helper sandbox/isolation model cannot support declared, schema-bound, pre-authorized host-management jobs, the trust pillar must be rewritten before Phase 1 can be accepted into current. Removing helper isolation, adding a host command channel, or making Borgee the runtime owner is a major decision, not a minor continuation.

## §2 Locked Planning Scope And Task-Level Decisions

### §2.1 `HB-RA-1A` Helper bounded actuator product guardrails

Locked product guardrails:

- After explicit local enrollment, Web-side Configure OpenClaw may enqueue bounded, pre-authorized typed jobs without asking the user to SSH again.
- Enrollment-time delegation is not blanket preauthorization; it covers only a closed v1 taxonomy for OpenClaw / Helper lifecycle and config.
- The helper uses outbound poll / long-poll. The server never dials the host.
- Server enqueue authorization and helper local policy both validate owner, org, enrollment, delegation, job type, manifest/artifact, paths/domains, service IDs, and revocation state.
- Web sends schema-bound typed jobs, not arbitrary shell commands, argv, executable paths, scripts, or service unit names.
- Long-lived Helper / OpenClaw services stay non-sudo. `install-butler` remains short-lived, visible, and never caches sudo.
- Revoke / uninstall prevents future jobs, deterministically settles queued or leased jobs, invalidates helper auth, disables in-scope services, and is visible in UI.
- Status and logs are bounded and redacted; failed jobs cannot look successful or spin indefinitely.
- Helper UI placement may move, but Remote Agent credentials, grants, and enforcement rails remain separate from Helper actuator credentials, grants, and enforcement rails.

These guardrails do not lock the execution contract in `HB-RA-1B`. Phase planning for this anchor must preserve the closed schema-bound job model, outbound-only server relationship, server-plus-helper validation, non-sudo long-lived services, revoke/uninstall/status/log guardrails, and separate Remote Agent rails.

### §2.2 `HB-RA-1B` Helper actuator execution contract planning scope

Locked planning scope carried into milestone breakdown and task-level Dev design:

- Manifest signing and artifact binding: signing authority, digest scope, cache invalidation, and replay handling.
- Helper credential model: token shape, rotation cadence, stale-device semantics, and local storage rules.
- Sandbox and Linux outbound poll: exact macOS/Linux write paths, network domains, outbound polling permission, and the current Linux AF_UNIX-only service restriction.
- Revoke race mechanics: safe action boundaries, lease cancellation behavior, terminal status precedence, and what a running helper must do when revocation wins.
- Service permissions: allowed service manager operations, long-lived service privilege level, restart/crash-recovery boundaries, and install-time privilege handoff.
- Exact queue/lease/result contract: job states, lease duration and renewal, idempotency keys, result schema, retry rules, terminal failure shape, and server/helper clock authority.

`docs/tasks/681-remote-agent-openclaw/` is a legacy intake folder. The v1.1 execution path is now the Phase 1 Helper/OpenClaw Phase/Milestone plan under `docs/tasks/phase-1-helper-openclaw-onboarding/`.

### §2.3 `MR-1` Mention routing

Safe guardrails:

- `@Everyone` fanout is server-authoritative and computed from channel membership and ACL. The client may send only the token, not recipient IDs.
- `@Everyone` has rate limits and loop prevention. Agents cannot recursively trigger broadcast fanout.
- Per-channel `requireMention` cannot let a channel owner broaden an external agent's attention or capability. The agent owner may opt into broader delivery; channel owners can only reduce, mute, or remove.

Locked planning choices:

- Per-channel `requireMention` uses tri-state inherit / on / off semantics.
- Setting changes do not backfill historical messages by default.

### §2.4 `CH-1` Channel authority

Safe guardrails:

- Self-created or owned channels do not expose `leave`; the owner manages deletion through channel management.
- Owner transfer and hard delete/archive are not default v1 commitments.
- Private channel lock UI must not collide with unread, fault, or presence badges.

Task-level choices inside this milestone boundary:

- Channel management may live in Settings or as an in-channel settings/index surface if the task keeps membership/ownership authority clear.
- The management surface focuses on membership/ownership actions; notification, collapse, and sort rewrites stay out unless a task explicitly scopes them.

### §2.5 `CT-1` Client truthfulness

Safe guardrails:

- ArtifactComments must be production-mounted if the product claims the surface exists.
- Forbidden ACL states must be visible and must not leak private channel, artifact, or message names/bodies before authorization succeeds.
- Security/permission AP bundle UI remains in scope; RT-3 presence polish and broad e2e platform expansion remain backlog unless reopened.

Locked planning choices:

- Forbidden state is local/in-surface by default unless a task proves redirect or full-page state is better.
- ArtifactComments/ArtifactPanel and Settings `PermissionsView` require e2e reverse proof as milestone acceptance. This is not a broad quality-platform expansion and not a global blueprint invariant.

### §2.6 `PS-1` Privacy scope guard

Locked scope:

- gh#654 means no new user-facing privacy/compliance product expansion. It preserves existing admin, privacy, and security controls; impersonation consent and visible impersonation state; audit and enforcement logs; data minimization; capability boundaries; and Helper / Remote Agent rail separation.

Locked guardrail:

- No new user-facing privacy/compliance product expansion enters v1 through gh#654.
- Existing admin/privacy/security controls, impersonation safeguards, audit/enforcement logs, data minimization, capability boundaries, and Helper / Remote Agent rail separation remain in scope. Avoiding compliance-product scope cannot be used to weaken those controls.

### §2.7 `IA-1` Sidebar and account IA

Safe guardrails:

- Sidebar footer should expose only a small set of primary entries. Default candidate: avatar, Agents, Workspace, Settings.
- Avatar opens an account panel; logout moves into that panel.
- Moving Remote Nodes / Helper entry points in IA does not merge Remote Agent, Helper, credentials, grants, or enforcement rails.

Locked planning choices:

- Account panel v1 is account summary plus logout unless a task explicitly adds account settings.
- Remote Nodes / Helper placement may move into Settings or another host-runtime management surface, but credentials, grants, and enforcement rails remain separate.

## §3 Source Issues

Selected issue traceability lives in `docs/blueprint/_meta/v1.1/source-issues.md`. After selection, issue labels are no longer the workflow state; this ledger and `docs/tasks/` own resume state.

## §4 Backlog And Conditional Inputs

- gh#702: bring in only when agent config / onboarding copy is reopened.
- gh#707 and gh#697: quality gate / a11y follow-up stays backlog unless explicitly pulled into `CT-1`.
- gh#607: file naming maintenance stays backlog.
- gh#675: visual redesign stays backlog unless the user opens a separate visual redesign discussion.

## §5 Next Workflow Step

Continue Phase 1 Milestone 2 from `docs/tasks/phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop/task-1-job-envelope-and-enqueue-authority/`. Phase 1 Milestone 1 is accepted through PR #934 (`547f869`), PR #936 (`1ca5f95`), and PR #937 (`2872905`). Milestone 2 task 1 is unlocked for task-start/four-piece review; product implementation has not started.
