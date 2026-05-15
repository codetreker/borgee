# PM Stance: Local Policy / Manifest / Sandbox Profile

## Scope Position

This task is the Helper-side trust gate for bounded host-management jobs. It should make it possible for a reviewer to prove that even after server enqueue and transport delivery, the local Helper refuses work that is malformed, unbound to an approved manifest/artifact, outside path/domain/service authority, revoked, stale, or for the wrong owner/org.

## Stances

1. Local policy is a second authority check, not a rubber stamp.
   - Constraint: the Helper must revalidate the job envelope and policy inputs before any action can start.
   - Reviewer signal: enqueue approval alone is insufficient; local denial reasons exist for schema, manifest, artifact, path, domain, service, revocation, stale credential, wrong owner, and wrong org failures.

2. Typed jobs stay closed and schema-bound.
   - Constraint: unknown job types, schema-version drift, extra fields, and client-supplied execution fields are denied locally.
   - Reviewer signal: there is still no arbitrary command, shell, argv, executable path, script body, service unit, path, or domain channel.

3. Manifest/artifact binding carries host authority.
   - Constraint: install/config/service authority must come from a signed manifest and verified artifact binding, not from browser payload fields.
   - Reviewer signal: missing, mismatched, revoked, replayed, or wrong-scope manifest/artifact material fails before action.

4. Allowlists are narrow and explainable.
   - Constraint: eligible paths, domains, and service IDs come from signed manifest, enrollment state, and task 5 prerequisite configuration.
   - Reviewer signal: jobs cannot add new local paths, network destinations, or service identifiers through payload fields.

5. Sandbox and policy must agree.
   - Constraint: sandbox/profile permissions cannot become broader than the policy surface needed for declared jobs.
   - Reviewer signal: Dev design can map policy-approved path/domain/service categories to sandbox affordances and fail closed when either side denies.

6. Task 6 is an interface consumer, not owned scope.
   - Constraint: this task may assume a received server-owned envelope and may return local policy decisions/failure reasons, but does not design or implement pull, lease, ack, retry, cancellation, or result upload.
   - Reviewer signal: task 7 docs and future design stay transport-neutral except for the minimum decision handoff needed by task 6.

7. Helper and Remote Agent rails remain separate.
   - Constraint: no Remote Agent credential, reverse-WS transport, host grant, file-proxy status, or Remote Agent permission fallback can authorize Helper policy.
   - Reviewer signal: local policy uses Helper enrollment/credential/delegation state only.

8. Privacy scope stays internal and bounded.
   - Constraint: keep data minimization and rail separation; do not add privacy dashboards, compliance surfaces, legal promise copy, or broad admin-facing product changes.

## Out-Of-Scope Locks

- No Helper pull/lease/result transport implementation.
- No OpenClaw install/config/plugin/channel-binding action.
- No service lifecycle restart/boot/crash feature.
- No Configure OpenClaw terminal UI or action closure.
- No sudo cache, persistent privileged service, or silent escalation.
- No Remote Agent rail reuse.
- No arbitrary host command, service unit, file path, or network domain authority.
