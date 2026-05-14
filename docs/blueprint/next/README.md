# Blueprint Next State

Target version: v1.1 candidate
Last updated: 2026-05-14
Resume from: route locked `HB-RA-1A` guardrails to Phase planning while continuing `HB-RA-1B` execution-contract discussion.

This directory tracks planned or in-discussion blueprint work that is not yet accepted into `docs/blueprint/current/`. `current/` remains the implemented-and-accepted product truth. `docs/tasks/` is used only after a next anchor is locked for execution.

## Status Ledger

| Anchor | Topic | Decision | Work | Source issues | Reference | Milestone path | Next action |
|---|---|---|---|---|---|---|---|
| `HB-RA-1A` | Helper bounded actuator product guardrails | LOCKED | PENDING | gh#681, gh#659 | `remote-actuator-design.md`; `migration-analysis.md` §2 | - | Route only the bounded-helper guardrails below to Phase planning. |
| `HB-RA-1B` | Helper actuator execution contract blockers | OPEN | PENDING | gh#681, gh#659 | `remote-actuator-design.md`; `migration-analysis.md` §2 | - | Resolve manifest/artifact signing, helper credentials, sandbox/Linux poll, revoke races, service permissions, and queue contract. |
| `MR-1` | Mention routing granularity and `@Everyone` broadcast | OPEN | PENDING | gh#674, gh#693 | `migration-analysis.md` §3 | - | Decide per-channel `requireMention` model and history behavior. |
| `CH-1` | Channel authority and user-side channel management | OPEN | PENDING | gh#685, gh#688, gh#690 | `migration-analysis.md` §4 | - | Decide management page placement and private badge interaction rules. |
| `CT-1` | Client truthfulness and forbidden-state visibility | OPEN | PENDING | gh#724 | `migration-analysis.md` §5 | - | Decide forbidden-state shape and selected-surface e2e reverse-proof acceptance. |
| `PS-1` | Privacy scope guard | OPEN | PENDING | gh#654 | `migration-analysis.md` §6 | - | Treat as lock candidate after Security confirms user-facing scope does not expand and existing controls stay intact. |
| `IA-1` | Sidebar footer and account entry IA | OPEN | PENDING | gh#669, gh#670 | `migration-analysis.md` §7 | - | Decide account panel scope and Remote Nodes / Helper entry placement. |

Decision values are `OPEN`, `LOCKED`, or `REOPENED`. Work values are `PENDING`, `IMPLEMENTING`, or `COMPLETED`. Only `LOCKED` anchors may move into `docs/tasks/` Phase/Milestone planning.

## Iteration Positioning

This next iteration does not rewrite Borgee's product identity. It closes v1 usability and trust gaps discovered after first real use: Helper / remote actuator onboarding, mention routing, channel authority, client truthfulness, privacy scope discipline, and sidebar/account IA.

Default version judgment is minor continuation. The only major-trigger cluster is the helper bounded-actuator work: if the current helper sandbox/isolation model cannot support declared, schema-bound, pre-authorized host-management jobs, the trust pillar must be rewritten before execution lock. Removing helper isolation, adding a host command channel, or making Borgee the runtime owner is a major decision, not a minor continuation.

## Lock Candidates And Open Blockers

### `HB-RA-1A` Helper bounded actuator product guardrails

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

### `HB-RA-1B` Helper actuator execution contract blockers

Open blockers before execution lock:

- Manifest signing and artifact binding: signing authority, digest scope, cache invalidation, and replay handling.
- Helper credential model: token shape, rotation cadence, stale-device semantics, and local storage rules.
- Sandbox and Linux outbound poll: exact macOS/Linux write paths, network domains, outbound polling permission, and the current Linux AF_UNIX-only service restriction.
- Revoke race mechanics: safe action boundaries, lease cancellation behavior, terminal status precedence, and what a running helper must do when revocation wins.
- Service permissions: allowed service manager operations, long-lived service privilege level, restart/crash-recovery boundaries, and install-time privilege handoff.
- Exact queue/lease/result contract: job states, lease duration and renewal, idempotency keys, result schema, retry rules, terminal failure shape, and server/helper clock authority.

`docs/tasks/681-remote-agent-openclaw/` is a legacy intake folder. It must not be treated as an execution path until a locked helper anchor has a Phase/Milestone path.

### `MR-1` Mention routing

Safe guardrails:

- `@Everyone` fanout is server-authoritative and computed from channel membership and ACL. The client may send only the token, not recipient IDs.
- `@Everyone` has rate limits and loop prevention. Agents cannot recursively trigger broadcast fanout.
- Per-channel `requireMention` cannot let a channel owner broaden an external agent's attention or capability. The agent owner may opt into broader delivery; channel owners can only reduce, mute, or remove.

Open blockers:

- Decide whether per-channel `requireMention` is tri-state: inherit / on / off.
- Decide whether setting changes ever backfill historical messages. Default candidate: no history sweep.

### `CH-1` Channel authority

Safe guardrails:

- Self-created or owned channels do not expose `leave`; the owner manages deletion through channel management.
- Owner transfer and hard delete/archive are not default v1 commitments.
- Private channel lock UI must not collide with unread, fault, or presence badges.

Open blockers:

- Decide whether channel management lives in Settings or as an in-channel settings/index surface.
- Decide whether the management surface includes notification, collapse, and sort settings or only membership/ownership actions.

### `CT-1` Client truthfulness

Safe guardrails:

- ArtifactComments must be production-mounted if the product claims the surface exists.
- Forbidden ACL states must be visible and must not leak private channel, artifact, or message names/bodies before authorization succeeds.
- Security/permission AP bundle UI remains in scope; RT-3 presence polish and broad e2e platform expansion remain backlog unless reopened.

Open blockers:

- Decide whether forbidden state is redirect, full-page state, or in-surface empty/error state.
- Decide which selected `CT-1` surfaces require e2e reverse proof as milestone acceptance. This is not a broad quality-platform expansion and not a global blueprint invariant.

### `PS-1` Privacy scope guard

Lock candidate:

- gh#654 means no new user-facing privacy/compliance product expansion. It preserves existing admin, privacy, and security controls; impersonation consent and visible impersonation state; audit and enforcement logs; data minimization; capability boundaries; and Helper / Remote Agent rail separation.

Security blocker:

- Reconcile any cleanup language with the current admin privacy contract. Existing admin/privacy/security controls, impersonation safeguards, audit/enforcement logs, data minimization, and rail separation remain in scope under the label of avoiding compliance-product scope.

### `IA-1` Sidebar and account IA

Safe guardrails:

- Sidebar footer should expose only a small set of primary entries. Default candidate: avatar, Agents, Workspace, Settings.
- Avatar opens an account panel; logout moves into that panel.
- Moving Remote Nodes / Helper entry points in IA does not merge Remote Agent, Helper, credentials, grants, or enforcement rails.

Open blockers:

- Decide whether account panel v1 is display name / account summary / logout only, or includes account settings.
- Decide whether Remote Nodes belongs in Settings, Helper panel, or another host-runtime management surface.

## Source Issues

Selected issue traceability lives in `docs/blueprint/_meta/v1.1/source-issues.md`. After selection, issue labels are no longer the workflow state; this ledger and `docs/tasks/` own resume state.

## Backlog And Conditional Inputs

- gh#702: bring in only when agent config / onboarding copy is reopened.
- gh#707 and gh#697: quality gate / a11y follow-up stays backlog unless explicitly pulled into `CT-1`.
- gh#607: file naming maintenance stays backlog.
- gh#675: visual redesign stays backlog unless the user opens a separate visual redesign discussion.

## Next Workflow Step

Route `HB-RA-1A` to `bf-phase-plan` for guardrail-only Phase/Milestone planning. Keep `HB-RA-1B` and the other `OPEN / PENDING` rows in lock review until their blockers are resolved or split. Do not run milestone breakdown or task execution before the relevant anchor lock.
