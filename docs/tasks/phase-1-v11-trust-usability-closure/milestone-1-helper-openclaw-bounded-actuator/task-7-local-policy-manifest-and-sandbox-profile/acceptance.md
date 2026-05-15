# Acceptance: Local Policy / Manifest / Sandbox Profile

## Source Alignment

- Task: `task-7-local-policy-manifest-and-sandbox-profile`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Blueprint anchors: `remote-actuator-design.md` section 1.2, section 7, and section 8; `migration-analysis.md` section 6.1
- Dependencies: accepted PR #934 (`547f869`), PR #936 (`1ca5f95`), PR #937 (`2872905`), PR #938 (`64d56f1`), and PR #939 (`96dc0dc`).

## Segment A: Fixed Schema Validation

Acceptance checks:

- Helper local policy validates the closed typed-job schema before action.
- Unknown job types, unsupported schema versions, malformed payloads, extra fields, and client-supplied execution authority are denied.
- Failure reasons distinguish at least schema-invalid and unknown-job-type denials for later terminal settlement.

Negative checks:

- No shell, argv, executable path, script body, arbitrary service unit, arbitrary local path, or arbitrary domain field can pass local policy.
- No transport implementation from task 6 is required to prove local schema denial.

## Segment B: Signed Manifest And Artifact Binding

Acceptance checks:

- Jobs that require install/config/service authority bind to a signed manifest and verified artifact reference or digest before action.
- Missing, mismatched, stale, replayed, revoked, wrong-owner, or wrong-org manifest/artifact authority fails closed.
- Review evidence shows artifact authority is not accepted from client payload fields alone.

Negative checks:

- No unsigned artifact, mismatched digest, untrusted signing authority, or wrong-scope manifest can authorize host changes.

## Segment C: Path And Domain Allowlists

Acceptance checks:

- Local policy allows only declared OpenClaw/Borgee config, cache, state, and audit/status paths needed by bounded typed jobs.
- Local policy allows only domains already permitted by signed manifest/enrollment state and task 5 prerequisite configuration.
- Out-of-allowlist paths/domains fail with explicit local policy denial.

Negative checks:

- No arbitrary file write, client-supplied path, private content dump, full environment dump, arbitrary network destination, localhost/private/link-local/metadata detour, or job-payload-added domain is accepted.

## Segment D: Declared Service IDs

Acceptance checks:

- Local policy recognizes only service IDs declared by signed manifest or enrollment state as eligible for later service operations.
- Undeclared, arbitrary, client-supplied, or wrong-owner/wrong-org service IDs fail before any service-manager call.

Negative checks:

- No service lifecycle action, boot/crash restart, arbitrary unit execution, sudo cache, or privileged long-lived service behavior is introduced by this task.

## Segment E: Revoked, Stale, Wrong Owner, And Wrong Org Rejection

Acceptance checks:

- Local policy denies revoked enrollment/delegation, stale Helper credential/device state, wrong owner, wrong org, and policy state that changes between delivery and pre-action validation.
- Denials are shaped so task 6 can settle terminal outcomes later without making denied work look successful.

Negative checks:

- No enqueue-time approval bypasses local revocation, owner/org, or stale-authority checks.
- No Remote Agent credential, host grant, reverse-WS status, file-proxy status, or user permission fallback authorizes Helper policy.

## Segment F: Sandbox/Profile Alignment

Acceptance checks:

- Policy-approved categories map to task 5 sandbox/profile affordances without broadening host authority.
- If sandbox/profile and local policy disagree, the safer denial wins and is visible as a policy/sandbox failure for later settlement.

Negative checks:

- Sandbox permissions cannot silently permit paths, domains, service IDs, or artifact cache access that policy would deny.

## Segment G: Task-Start Completion

Acceptance checks for this task-start commit:

- `spec.md`, `stance.md`, `acceptance.md`, and `progress.md` exist and match the canonical task/milestone boundary.
- `content-lock.md` is recorded as N/A in `progress.md` because task-start scope has no UI copy, DOM selectors, or product-facing text literals.
- Shared milestone index is not edited during task-start prep.
- No product code is implemented in the task-start commit.
