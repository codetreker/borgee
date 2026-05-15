# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-7-local-policy-manifest-and-sandbox-profile` |
| Branch | `feat/task-7-local-policy-manifest-and-sandbox-profile` |
| PR | N/A; local task/design commits only |
| Owner | Blueprintflow tasking worker under Teamlead |
| State | READY_FOR_LOCAL_COMMIT |
| Blocker | none; design gate green via `PM_QA_DESIGN_LGTM_BOTH` and `ARCH_SECURITY_TASK7_DESIGN_LGTM` |

## Checkpoints

- [x] Worktree/branch created from `origin/main` at `10e79bf`
- [x] `AGENTS.md` reviewed
- [x] Canonical Phase 1, Milestone 1, task 7, accepted history, and adjacent task 6 docs reviewed
- [x] Blueprint anchors reviewed: `remote-actuator-design.md` sections 1.2, 7, and 8; `migration-analysis.md` section 6.1
- [x] Accepted dependencies confirmed from canonical milestone docs: PR #934 (`547f869`), PR #936 (`1ca5f95`), PR #937 (`2872905`), PR #938 (`64d56f1`), and PR #939 (`96dc0dc`)
- [x] Four-piece task-start docs created: `spec.md`, `stance.md`, `acceptance.md`, `progress.md`
- [x] `content-lock.md` checked N/A because task-start scope has no UI copy, DOM selectors, or product-facing content literals
- [x] Shared milestone index intentionally not edited during task-start prep
- [x] Product code intentionally not changed in task-start commit scope
- [x] Dev/Security scouting completed against Helper sandbox/outbound, server Helper jobs/enrollment, manifest verifier, installer docs, plugin docs, and current docs
- [x] `design.md` drafted with policy schema, manifest/artifact binding, signature/digest boundary, allowlists, sandbox alignment, auth/state rejection, test plan, docs/current sync, and non-goals
- [x] Task state advanced to `READY_FOR_DESIGN_REVIEW`
- [x] Design gate accepted: `PM_QA_DESIGN_LGTM_BOTH` and `ARCH_SECURITY_TASK7_DESIGN_LGTM`
- [x] Task state advanced through `READY_FOR_IMPL` to `IMPLEMENTING`
- [x] RED tests written first for `packages/borgee-helper/internal/jobpolicy`
- [x] RED evidence captured with missing policy API/types before production implementation
- [x] Helper-side local policy evaluator implemented without task 6 transport or action execution
- [x] Focused and broader Helper verification passed locally
- [x] Docs/current synced for landed local policy behavior and remaining no-poll/no-action limits

## Task-Prep Evidence

| Item | Evidence | Result |
|---|---|---|
| Base | `origin/main` resolved to `10e79bf` for task-start worktree creation | PASS |
| Worktree/branch | Created `/workspace/borgee/.worktrees/task-7-local-policy-manifest-and-sandbox-profile` on `feat/task-7-local-policy-manifest-and-sandbox-profile` | PASS |
| Required instructions | Read `AGENTS.md`; worker owns git operations for this task, while parent Teamlead remains orchestration-only | PASS |
| Required task docs | Read canonical phase plan, Milestone 1 doc, accepted history, task 7 skeleton, task 6 skeleton, and accepted task 4/task 5 prep docs for boundary alignment | PASS |
| Blueprint anchors | Reviewed locked guardrails and execution-contract planning scope for local policy, closed typed jobs, sandbox, and privacy-scope guard | PASS |
| Dependency state | Canonical docs show task 7 READY after accepted PR #934/#936/#937/#938/#939 | PASS |
| Scope split | Task 7 owns local policy/manifest/sandbox profile; task 6 owns pull/lease/result transport and settlement mechanics | PASS |
| Shared index | No milestone index or shared task index edit made in this task-start commit | PASS |
| Content lock | N/A; no UI copy, selectors, or product-facing text literals are part of task-start scope | PASS |
| Product code | No product code changes made in task-start commit scope | PASS |

## Design-Prep Evidence

| Item | Evidence | Result |
|---|---|---|
| Helper daemon boundary | Read `packages/borgee-helper/cmd/borgee-helper/main.go`; daemon validates outbound prereqs, opens grants DB, applies sandbox, and serves UDS without polling jobs or local policy today | PASS |
| Outbound prereq boundary | Read `packages/borgee-helper/internal/outbound/prereq.go`, tests, and install assets; exact public HTTPS origin and Helper-owned state root validation exist, with documented DNS/CNAME residual risk | PASS |
| Sandbox/profile boundary | Read `packages/borgee-helper/internal/sandbox/*`, `install/borgee-helper.service`, `install/cloud.borgee.host-bridge.plist`, `install/borgee-helper.sb`, and asset tests | PASS |
| Existing Helper ACL rail | Read `internal/acl`, `internal/grants`, `internal/fileio`, `internal/ipc`, `internal/audit`, and `internal/reasons`; these are current host-grant-backed IPC patterns, not Helper job policy authority | PASS |
| Server Helper jobs | Read `packages/server-go/internal/store/helper_job_queries.go`, `internal/api/helper_jobs.go`, `internal/datalayer/helper_jobs*.go`, migration/tests, and model fields; enqueue is server-only and task 6 endpoints remain unmounted | PASS |
| Helper enrollment/auth state | Read `helper_enrollment_queries.go`, API handler, migrations, and docs; owner/org/enrollment/device/current credential generation and terminal revoke/uninstall states are available as policy inputs | PASS |
| Manifest/artifact baseline | Read `packages/borgee-installer/internal/manifest`, installer commands/docs, and `server-go/internal/api/host_manifest.go`; existing installer manifest trust remains partial and envelope shapes differ | PASS |
| Plugin/OpenClaw context | Read OpenClaw plugin config/setup/docs enough to verify task 7 should not write plugin config or execute OpenClaw actions | PASS |
| Design doc | Created `design.md` with explicit Dev and Security scouting inputs plus requested design sections | PASS |
| Product code | No product code changes made in design-prep commit scope | PASS |

## Interface Assumptions For Dev Design

- Task 7 may assume a server-owned typed job envelope exists after task 4 and is delivered/settled by task 6.
- Task 7 may expose local policy allow/deny decisions and failure reasons that task 6 can later report, but task 7 does not own transport, lease, result upload, retry, backoff, or cancellation mechanics.
- Task 7 policy inputs should include Helper enrollment identity, owner/org binding, job type/schema version, manifest/artifact reference, declared path/domain/service authority, and current revocation/stale-authority state.

## Scope Locks

- In scope: fixed schema validation, signed manifest/artifact binding, allowlisted paths/domains, declared service IDs, revoked/stale/wrong-owner/wrong-org rejection, sandbox/profile alignment, and Helper/Remote Agent rail separation.
- Out of scope: Helper pull/lease/result transport, OpenClaw action execution, service lifecycle restart/boot/crash behavior, Configure OpenClaw terminal UI, sudo cache, persistent privileged service behavior, and Remote Agent rail reuse.

## Design Review Handoff

- Proposed implementation unit: new Helper-side `internal/jobpolicy` package that evaluates a delivered server-owned job candidate and returns allow/deny with reason, without owning HTTP polling or action execution.
- Existing installer manifest verifier is not treated as sufficient runtime policy trust. Design calls out envelope mismatch and partial artifact-binding/key-wiring gaps, then defines the runtime manifest/artifact binding boundary for review.
- Task 6 interface assumption stays narrow: task 6 may call policy and later report the returned reason; task 7 does not design task 6 endpoints, lease/result shape, retry/backoff, cancellation, or result upload.
- Security review should focus on signed manifest canonical bytes, artifact byte/cache digest binding, no rail reuse, path/domain/service fail-closed checks, and sandbox/profile alignment.

## Implementation Evidence

| Item | Evidence | Result |
|---|---|---|
| RED test-first run | `GOTMPDIR=$PWD/.gotmp go test ./internal/jobpolicy` failed before production implementation with undefined API/types including `JobTypeOpenClawConfigureAgent`, `ArtifactDeclaration`, `PathDeclaration`, `ServiceDeclaration`, `EvaluationInput`, `ManifestBinding`, `Decision`, and `Reason` | PASS |
| Additional RED hardening | Added artifact-origin binding test; `GOTMPDIR=$PWD/.gotmp go test ./internal/jobpolicy` failed with `artifact_origin_not_bound_as_allowed_domain` returning `allow=true reason=ok` instead of `domain_denied` before the fix | PASS |
| Focused GREEN | `GOTMPDIR=$PWD/.gotmp go test ./internal/jobpolicy` -> `ok borgee-helper/internal/jobpolicy 0.008s` | PASS |
| Focused Helper verification | `GOTMPDIR=$PWD/.gotmp go test ./internal/jobpolicy ./internal/outbound ./install` -> `ok` for all three packages | PASS |
| Broader Helper verification | `GOTMPDIR=$PWD/.gotmp go test ./cmd/borgee-helper ./internal/...` -> `ok` for helper internal packages and `cmd/borgee-helper` no test files | PASS |
| Whitespace check | `git diff --check` -> no output, exit 0 | PASS |
| Scope boundary | Added pure `internal/jobpolicy` package only; no Helper poll/lease/result routes, result upload, OpenClaw action execution, service-manager calls, sudo, Remote Agent credential reuse, or server transport implementation | PASS |
| Docs/current sync | Updated Host Bridge helper, Host Bridge overview, Security boundaries, and Known Gaps to document the pure evaluator and remaining no-poll/no-action/no-settlement limits | PASS |

## Acceptance State

Task 7 implementation is ready for local commit. The Helper-side pure local policy evaluator is implemented and verified for fixed schema validation, signed manifest and artifact binding, path/domain/service allowlists, revoked/stale/wrong-owner/wrong-org denial, and sandbox/profile mismatch denial. Task 6 transport, result upload, OpenClaw action execution, and service lifecycle execution remain out of scope and unimplemented.
