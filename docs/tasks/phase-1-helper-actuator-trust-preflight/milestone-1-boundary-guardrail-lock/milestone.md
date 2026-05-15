# Milestone 1: HB-RA-1A Boundary Guardrail Lock

## Source Anchor

- Source: `docs/blueprint/next/README.md` `HB-RA-1A`, lines 13 and 31-45.
- Reverse-check guardrail: `docs/blueprint/next/README.md` `PS-1`, lines 18 and 99-108.
- Exclusion: `docs/blueprint/next/README.md` `HB-RA-1B`, lines 14 and 47-56, remains open for execution-contract blockers.

## Capability Goal

Make the bounded Configure OpenClaw delegation promise reviewable before helper actuator implementation begins. The milestone records what must be true for the team to approve the `HB-RA-1A` guardrail boundary and what must remain blocked until `HB-RA-1B` locks.

## Acceptance Boundary

Accepted by this milestone:

- The product promise is bounded to explicit local enrollment plus closed, schema-bound Configure OpenClaw delegation.
- The server relationship is outbound-only from Helper poll/long-poll. The server never dials the host.
- Server enqueue authorization and Helper local policy both remain part of the trust boundary.
- Status, logs, revocation, uninstall, failure states, and bounded redaction remain user-perceivable requirements.
- Security wording is explicit: no command channel, no credential reuse, no sudo cache, and Helper rails remain separate from Remote Agent rails.
- `PS-1` privacy scope is preserved: no new user-facing privacy/compliance product expansion, and existing admin/privacy/security controls stay intact.

Rejected by this milestone:

- Treating enrollment as blanket preauthorization.
- Treating Web Configure OpenClaw as arbitrary shell, argv, executable path, script, or service-unit dispatch.
- Reusing Remote Agent credentials, grants, or enforcement rails for Helper actuator work.
- Caching sudo or making long-lived Helper/OpenClaw services privileged.
- Advancing `HB-RA-1B` blockers as accepted design.
- Using `PS-1` to remove admin, privacy, security, impersonation, audit/enforcement, data-minimization, capability, or rail-separation controls.

## Dependencies

| Dependency | Status | Handling |
|---|---|---|
| `HB-RA-1A` locked next anchor | READY | Cited as the source for this milestone |
| `PS-1` privacy scope guard | READY | Required reverse-check for privacy/security/admin/rail boundaries |
| `HB-RA-1B` blockers | BLOCKED OUT OF SCOPE | Kept in next-blueprint discussion until lock |
| Legacy `docs/tasks/681-remote-agent-openclaw/` intake | HISTORICAL | Not an execution path |
| `bf-milestone-breakdown` | TASK_SET_READY | Task skeleton review is recorded; first ready task can enter `bf-task-execute` |

## Exit Gates

Strict:

- `HB-RA-1A` citation is present in `phase-plan.md`, this milestone, and the task seed.
- `PS-1` citation is present in `phase-plan.md`, this milestone, and the task seed.
- `HB-RA-1B` is named as out of scope in `phase-plan.md`, this milestone, and the task seed.
- No placeholder markers for unfinished work remain in the touched planning files.
- Forbidden implementation-implying phrases are absent outside reject/non-goal language.
- `git show --check` passes.

User-perceivable:

- The reviewed plan says what a user sees before trust is granted: Helper connected, allowed job categories, Configure OpenClaw progress, truthful terminal state, bounded redacted logs, and revoke/uninstall controls.
- The reviewed plan says what a user is protected from: arbitrary command dispatch, shared Remote Agent credentials, silent sudo reuse, and merged enforcement rails.
- The reviewed plan says privacy scope is not expanding and existing controls are not weakened.

Carry-over:

- Every unresolved execution detail is parked at `HB-RA-1B`, not hidden in this milestone.
- Every task contract produced by breakdown carries `PS-1` as a reverse-check guardrail when relevant.
- Any later implementation work must receive a task folder from `bf-milestone-breakdown` and its own task contract.

Fake-green:

- Failure, policy denial, revoked, stale, cancelled, expired, queued, leased, and running states cannot be summarized as success.
- `PS-1` cannot be summarized as permission to delete existing security/privacy controls.
- A green check cannot be based on docs existing alone; reviewers must verify the guardrail boundary and exclusions.

## Task-Split Trigger

Run `bf-milestone-breakdown` when this milestone plan has been reviewed and the team agrees the first executable task is still planning-preflight only. Do not split implementation tasks while `HB-RA-1B` remains open.

## First Task Seed

See `task-seed.md` for `task-0-hb-ra-1a-planning-preflight`.

## Breakdown State

State: `TASK_SET_READY`

Publication status:
- Published as `TASK_SET_READY`; Architect, PM, QA, Dev, and Security review decisions are recorded below.

## Task Index

| Task | Status | Purpose | Depends on | Parallel? | First ready? |
|---|---|---|---|---|---|
| `task-0-hb-ra-1a-planning-preflight` | READY | Add docs-only guardrail preflight evidence before Helper actuator implementation starts | none | no | yes |

Dependency order:
- Route `task-0-hb-ra-1a-planning-preflight` to `bf-task-execute` next.
- Do not create implementation tasks while `HB-RA-1B` remains open.

Parallelism:
- No parallel product tasks in this milestone; the single product task is the planning preflight.

First ready task:
- `task-0-hb-ra-1a-planning-preflight`.
- Current state: ready for `bf-task-execute`; not started here.

## Breakdown Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Boundaries, anchors, dependency order, and `HB-RA-1B` exclusion align at task-skeleton granularity. |
| PM | LGTM | User-value slice, trust promise, `PS-1` discipline, and docs-only scope remain aligned. |
| QA | LGTM | Acceptance slice is checkable, fake-green rejection is preserved, and the first ready task has no hidden prerequisite. |
| Dev | LGTM | One-PR feasibility is clear and no hidden code/API/queue/credential/sandbox/service work is introduced. |
| Security | LGTM | Required security review passed for auth, credentials, privacy, admin/security controls, command exclusion, service privilege, revoke/uninstall, and Helper/Remote Agent rail separation. |
