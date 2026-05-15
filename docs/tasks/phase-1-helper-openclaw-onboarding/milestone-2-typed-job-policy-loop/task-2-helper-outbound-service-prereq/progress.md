# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-2-helper-outbound-service-prereq` |
| Branch | `feat/task-2-helper-outbound-service-prereq` |
| PR | #939, merged at `96dc0dc` |
| Owner | Blueprintflow tasking worker under Teamlead |
| State | ACCEPTED |
| Blocker | none; residual DNS/CNAME resolution risk is documented as future hardening/runtime network-policy scope |

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
- [x] Security repair added after review: default HTTPS local/private/link-local/metadata origin rejection
- [x] `docs/current` sync checked after implementation
- [x] `docs/current` sync checked after security repair
- [x] Acceptance evidence recorded after implementation
- [x] Security repair evidence recorded
- [x] Security docs/progress repair recorded residual DNS/CNAME risk without overstating validator coverage
- [x] Final PR readiness verification run by operator worker
- [x] PR opened as #939
- [x] PR merged at `96dc0dca19c243bfc53c8e8a4af56dbd33214a26`

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

## Security Repair Evidence

| Item | Evidence | Result |
|---|---|---|
| Review blocker reproduced | `GOTMPDIR=$PWD/.gotmp go test ./internal/outbound` failed before production repair: HTTPS `localhost`, `localhost.`, `127.0.0.1`, `::1`, IPv4-mapped loopback/private, RFC1918, link-local, metadata `169.254.169.254`, IPv6 unique-local, and `fe80::1` cases were accepted; allowed-origin list containing metadata IP also returned nil | PASS |
| Default fail-closed origin validation | `normalizeOrigin` now rejects local/private origins for default HTTPS validation using canonical host/IP classification for localhost, loopback, private, link-local, multicast link-local, unspecified, and IPv4-mapped IP addresses | PASS |
| Test/dev loopback constraint | `ValidationOptions.AllowLoopbackHTTP` remains limited to explicit HTTP loopback; HTTP private and HTTPS loopback remain rejected | PASS |
| Focused GREEN | `GOTMPDIR=$PWD/.gotmp go test ./internal/outbound ./install` from `packages/borgee-helper` -> `ok borgee-helper/internal/outbound`; `ok borgee-helper/install` | PASS |
| Helper module GREEN | `GOTMPDIR=$PWD/.gotmp go test ./...` from `packages/borgee-helper` -> helper module passed | PASS |
| Installer module GREEN | `GOTMPDIR=$PWD/.gotmp go test ./...` from `packages/borgee-installer` -> installer module passed | PASS |
| Diff check GREEN | `git diff --check` -> no whitespace errors | PASS |
| docs/current sync | Updated host bridge and security docs to state that outbound prerequisite origins must match the exact public HTTPS allowlist and reject localhost/private/link-local/metadata literal origins by default even over HTTPS | PASS |

## Security Docs Repair Evidence

| Item | Evidence | Result |
|---|---|---|
| Literal-origin scope | Docs now state the validator classifies literal host/IP input with `netip` and rejects localhost/private/link-local/metadata literal origins by default | PASS |
| DNS residual risk | Docs now state allowed hostnames are not resolved and DNS answers/CNAME chains resolving to private, link-local, or metadata addresses are not guarded by this prerequisite validator | PASS |
| Production allowlist boundary | Docs now state production assets use exact `https://app.borgee.io`, while DNS resolution/rebinding remains future hardening or runtime network-policy scope | PASS |
| Diff check GREEN | `git diff --check` -> no whitespace errors | PASS |

## Final PR Readiness Evidence

| Item | Evidence | Result |
|---|---|---|
| Branch/worktree | `git status --short --branch` -> `## feat/task-2-helper-outbound-service-prereq...origin/main [ahead 5]` with no file changes | PASS |
| Diff check | `git diff --check` -> no whitespace errors | PASS |
| Focused helper tests | `GOTMPDIR=/tmp/borgee-gotmp-helper-prereq go test ./internal/outbound ./install` from `packages/borgee-helper` -> both packages passed | PASS |
| Helper module tests | `GOTMPDIR=/tmp/borgee-gotmp-helper-all go test ./...` from `packages/borgee-helper` -> all helper packages passed | PASS |
| Installer tests | `GOTMPDIR=/tmp/borgee-gotmp-installer-all go test ./...` from `packages/borgee-installer` -> all installer packages passed | PASS |
| PR lint rehearsal | Local current-sync rehearsal against the PR diff exited cleanly; no mapped module trigger fired because the workflow maps `packages/helper/`, not `packages/borgee-helper/` | PASS WITH LIMITATION |
| OpenClaw plugin PR lint | `BASE_SHA=$(git merge-base origin/main HEAD) HEAD_SHA=$(git rev-parse HEAD) bash scripts/check-openclaw-plugin-version-bump.sh` -> no OpenClaw plugin files changed | PASS |
| OpenClaw plugin self-test | `bash scripts/check-openclaw-plugin-version-bump.test.sh` -> version-bump script self-test passed | PASS |

## Scope Locks

- In scope: service/sandbox prerequisites for outbound Helper polling, Linux AF_UNIX-only restriction resolution boundary, allowed outbound domains, queue/status write paths, non-sudo service permission boundary, and Helper/Remote Agent rail separation.
- Out of scope: job lease/result/poll contract implementation beyond service prerequisite, local policy execution, OpenClaw action, service lifecycle restart, boot restart, crash restart, sudo cache, and Remote Agent rail reuse.

## Acceptance State

Task 2 is accepted through PR #939, merged at `96dc0dc`. Acceptance evidence above covers Linux service, macOS plist/sandbox, Helper config validation, explicit state/write paths, docs/current sync, and out-of-scope locks. `content-lock.md` remains N/A for this scope.
