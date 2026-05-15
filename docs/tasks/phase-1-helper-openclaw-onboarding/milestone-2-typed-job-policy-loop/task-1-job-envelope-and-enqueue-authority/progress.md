# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-1-job-envelope-and-enqueue-authority` |
| Branch | `feat/task-1-job-envelope-and-enqueue-authority` |
| PR | not opened |
| Owner | Blueprintflow tasking worker under Teamlead |
| State | DESIGN_REVIEW_REPAIRED |
| Blocker | Design review blockers repaired locally; awaiting re-review. Phase 1 milestone 1 is accepted through PR #934, PR #936, and PR #937. |

## Checkpoints

- [x] Worktree/branch created from `origin/main` at `2872905`
- [x] `AGENTS.md` reviewed
- [x] Task, milestone, shared task, and blueprint anchor docs reviewed
- [x] Shared Blueprintflow state refreshed for milestone 1 accepted and milestone 2 task 1 unlocked
- [x] Four-piece task-start docs created: `spec.md`, `stance.md`, `acceptance.md`, `progress.md`
- [x] `content-lock.md` checked N/A because task-start scope has no UI copy, DOM selectors, or product-facing content literals
- [x] Dev design drafted for review
- [x] Dev design reviewed: PM_LGTM, ARCHITECT_BLOCKED, QA_BLOCKED, SECURITY_BLOCKED
- [x] Design blockers repaired in `design.md`
- [ ] TDD RED tests written before implementation
- [ ] Product implementation complete
- [ ] `docs/current` sync checked after implementation or no-op rationale recorded
- [ ] Acceptance evidence recorded after implementation
- [ ] PR opened
- [ ] PR merged

## Task-Prep Evidence

| Item | Evidence | Result |
|---|---|---|
| Base | `origin/main` resolved to `2872905392db136789d08fc650a7e246bab4463b`, matching PR #937 merge state | PASS |
| Worktree/branch | Created `/workspace/borgee/.worktrees/task-1-job-envelope-and-enqueue-authority` on `feat/task-1-job-envelope-and-enqueue-authority` | PASS |
| Required instructions | Read `AGENTS.md`; kept parent Teamlead git/gh restriction as worker-owned git operations | PASS |
| Required task docs | Read task 1 `task.md`, milestone 1 and milestone 2 docs, shared `docs/tasks/README.md`, and `docs/blueprint/next/README.md` | PASS |
| Blueprint anchors | Read `remote-actuator-design.md` sections 1.2, 6, and 7; read `migration-analysis.md` section 6.1 | PASS |
| Milestone 1 state | Refreshed shared state to show PR #934 (`547f869`), PR #936 (`1ca5f95`), and PR #937 (`2872905`) accepted | PASS |
| Milestone 2 unlock | Refreshed milestone 2 task index so task 1 is `TASKING` and no longer blocked by milestone 1 | PASS |
| Four-piece | Created task-start `spec.md`, `stance.md`, and `acceptance.md`; this file records progress | PASS |
| Product code | No product code changes made in task-start commit scope | PASS |
| Dev inventory | Reviewed Helper enrollment API, datalayer, store queries, migration registry, route wiring, tests, and current docs; highest current migration is v50, so design reserves likely v51 with re-check instruction | PASS |
| Security pre-scan | Design records user-rail-only enqueue, strict typed JSON validation, closed job taxonomy, category mapping, idempotency/TTL, redaction, and explicit non-goals for poll/lease/result/execution/UI | PASS |

## Design Review Repair Evidence

| Reviewer | Blocker | Repair |
|---|---|---|
| Architect | `openclaw.configure_agent` referenced nonexistent `agent_config_id` | Replaced payload with `agent_id`; design now binds to existing `agent_configs(agent_id, schema_version, blob)` and captures server-derived config version/hash in the effective payload. |
| Architect | Store/datalayer must verify referenced config/channel against owner/org and capture effective payload for idempotency | Store responsibilities now require owner/org agent validation, channel authority validation, config row lookup, and server-derived effective payload before idempotency hashing. |
| QA | RED stale enrollment coverage | Test plan now requires API/store stale enrollment coverage: missing or older-than-five-minute `last_seen_at` returns `403 stale_enrollment` and creates no job row. |
| QA | TTL coverage | Design now requires server-generated bounded `expires_at`, rejects client `ttl`/`expires_at`/`deadline`/`lease_expires_at` with no job row, and treats expired queued rows as terminal/non-executable. |
| QA | Route negative tests | Test plan now requires proving only the task 1 enqueue route is mounted and poll/lease/result/ack/log/service/local-policy/install/uninstall/execution endpoints remain unmounted. |
| Security | Explicit task 1 enabled job type set | Taxonomy now says task 1 enables exactly `openclaw.configure_agent`; install, service, local-policy/write, status/log, revoke, and uninstall job types are recognized-but-rejected until owning tasks. |
| Security | Mandatory stale enrollment rejection | Enrollment checks now require fail-closed freshness: `last_seen_at` present and within five minutes, otherwise `stale_enrollment` and no job row. |
| Security | Idempotency/TTL must not globally block same job after expiry | Migration/model guidance now uses an active-window idempotency mechanism and forbids a permanent global unique index over `idempotency_scope`; expired/terminal rows stop participating in convergence/conflict. |

## Repair Verification

| Check | Result |
|---|---|
| Stale design wording scan | PASS: no remaining enqueue payload dependency on nonexistent `agent_config_id`; remaining mention is an explicit rejection note. |
| `git diff --check` | PASS |

## Scope Locks

- In scope: typed job envelope boundary, server enqueue authority, closed job type handling at enqueue, idempotency/TTL seeds, and enqueue-time failure truthfulness.
- Out of scope: Helper polling/lease execution, Linux service lifecycle, local policy manifest/sandbox profile, OpenClaw closure UI, job progress/log UI, and merged Helper/Remote Agent rails.

## Acceptance State

Dev design blockers from ARCHITECT, QA, and SECURITY review are repaired in `design.md`. Product implementation has not started and this task is not accepted until re-review, TDD, implementation, verification, docs/current sync, PR review, and merge complete.
