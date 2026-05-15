# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-2-helper-outbound-service-prereq` |
| Branch | `feat/task-2-helper-outbound-service-prereq` |
| PR | not opened |
| Owner | Blueprintflow tasking worker under Teamlead |
| State | VERIFIED_LOCAL_READY_FOR_COMMIT |
| Blocker | none |

## Checkpoints

- [x] Worktree/branch created from `origin/main` at `64d56f1d6b326bc3ceabd93412717c85aa0e0506`
- [x] `AGENTS.md` reviewed
- [x] Task, milestone, shared task, and blueprint anchor docs reviewed
- [x] Shared Blueprintflow state refreshed for task 1 accepted/merged through PR #938 and task 2 unlocked
- [x] Four-piece task-start docs created: `spec.md`, `stance.md`, `acceptance.md`, `progress.md`
- [x] `content-lock.md` checked N/A because task-start scope has no UI copy, DOM selectors, or product-facing content literals
- [x] Dev design drafted for review
- [x] Dev design checked against parent scout constraints and kept at task-design granularity
- [x] Dev design reviewed
- [x] Design gate green: `ARCHITECT_LGTM`, `PM_LGTM`, `QA_LGTM`, `SECURITY_LGTM` at `558934bafa9bf41af8b2f8457f83a690c51e0b36`
- [x] Implementation worker started strict TDD pass
- [x] RED tests/static checks written before production/config changes
- [x] Product implementation complete
- [x] `docs/current` sync checked after implementation
- [x] Acceptance evidence recorded after implementation
- [ ] PR opened
- [ ] PR merged

## Task-Prep Evidence

| Item | Evidence | Result |
|---|---|---|
| Base | `origin/main` resolved to `64d56f1d6b326bc3ceabd93412717c85aa0e0506`, matching PR #938 merge state | PASS |
| Worktree/branch | Created `/workspace/borgee/.worktrees/task-2-helper-outbound-service-prereq` on `feat/task-2-helper-outbound-service-prereq` | PASS |
| Required instructions | Read `AGENTS.md`; kept parent Teamlead git/gh restriction as worker-owned git operations | PASS |
| Required task docs | Read task 2 `task.md`, milestone 2 docs, shared `docs/tasks/README.md`, and `docs/blueprint/next/README.md` | PASS |
| Blueprint anchors | Read `remote-actuator-design.md` sections 1.2, 8, 9, and 14; read `migration-analysis.md` section 6.1 | PASS |
| Task 1 state | Refreshed shared state to show PR #938 (`64d56f1`) accepted and merged | PASS |
| Milestone 2 unlock | Refreshed milestone 2 task index so task 1 is `ACCEPTED` and task 2 is `TASKING` | PASS |
| Four-piece | Created task-start `spec.md`, `stance.md`, and `acceptance.md`; this file records progress | PASS |
| Product code | No product code changes made in task-start commit scope | PASS |
| Dev design | Drafted `design.md` from the task four-piece, helper service assets, sandbox code, daemon startup, and parent scout constraints; kept to service/sandbox/config/write-path/verification boundaries without implementation micro-detail | PASS |
| Design gate | Parent task handoff reports `ARCHITECT_LGTM`, `PM_LGTM`, `QA_LGTM`, and `SECURITY_LGTM` with latest design commit `558934bafa9bf41af8b2f8457f83a690c51e0b36` | PASS |

## Implementation Evidence

| Item | Evidence | Result |
|---|---|---|
| RED validator tests | `go test ./internal/outbound ./install` failed before implementation with missing `ValidateAndPrepare`, `PrereqConfig`, and `ValidationOptions` symbols | PASS |
| RED asset tests | `GOTMPDIR=$PWD/.gotmp go test ./install` failed before asset changes with missing `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6` and missing `--outbound-server-origin=https://app.borgee.io` | PASS |
| Linux service prerequisite | `packages/borgee-helper/install/borgee-helper.service` now keeps `User=borgee-helper`, `Group=borgee-helper`, `NoNewPrivileges=yes`, widens only to `AF_UNIX AF_INET AF_INET6`, and names explicit Helper-owned queue/status/audit-handoff write paths | PASS |
| macOS plist/sandbox prerequisite | `cloud.borgee.host-bridge.plist` passes exact origin/state flags; `borgee-helper.sb` keeps local Unix bind/outbound and adds remote TCP outbound only with explicit Helper state write paths | PASS |
| Helper config validation | `internal/outbound.ValidateAndPrepare` disables when unconfigured, rejects partial/malformed origins, enforces exact allowed HTTPS origin matching, normalizes state dirs under Helper-owned roots, and creates dirs with `0700` | PASS |
| Scope guard | No poll loop, lease/result/ack endpoint, local policy execution, OpenClaw action, service lifecycle restart, sudo cache, installer trust change, or Remote Agent rail reuse was implemented | PASS |
| Focused GREEN | `GOTMPDIR=$PWD/.gotmp go test ./internal/outbound ./install` -> `ok borgee-helper/internal/outbound`; `ok borgee-helper/install` | PASS |
| Helper breadth GREEN | `GOTMPDIR=$PWD/.gotmp go test ./cmd/borgee-helper ./internal/...` -> helper cmd/internal packages passed | PASS |
| Helper module GREEN | `GOTMPDIR=$PWD/.gotmp go test ./...` from `packages/borgee-helper` -> helper module passed, including install asset package | PASS |
| Installer breadth GREEN | `GOTMPDIR=$PWD/.gotmp go test ./...` from `packages/borgee-installer` -> installer packages passed after creating repo-local `.gotmp`; `/tmp` is not executable in this runtime | PASS |
| Diff check GREEN | `git diff --check` -> no whitespace errors | PASS |
| docs/current sync | Updated `docs/current/host-bridge/helper-daemon.md`, `docs/current/host-bridge/README.md`, `docs/current/security/README.md`, and `docs/current/known-gaps.md` for the prerequisite boundary and remaining pull-loop gaps | PASS |

## Scope Locks

- In scope: service/sandbox prerequisites for outbound Helper polling, Linux AF_UNIX-only restriction resolution boundary, allowed outbound domains, queue/status write paths, non-sudo service permission boundary, and Helper/Remote Agent rail separation.
- Out of scope: job lease/result/poll contract implementation beyond service prerequisite, local policy execution, OpenClaw action, service lifecycle restart, boot restart, crash restart, sudo cache, and Remote Agent rail reuse.

## Acceptance State

Implementation is locally verified and ready for commit. Acceptance evidence above covers Linux service, macOS plist/sandbox, Helper config validation, explicit state/write paths, docs/current sync, and out-of-scope locks. `content-lock.md` remains N/A for this scope, and no PR has been opened per worker assignment.
