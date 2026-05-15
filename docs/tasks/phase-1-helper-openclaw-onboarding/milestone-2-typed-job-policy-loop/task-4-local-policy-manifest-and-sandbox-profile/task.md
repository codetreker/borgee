# task-4-local-policy-manifest-and-sandbox-profile

Purpose:
- Ensure Helper revalidates every job locally before any host-management action can happen.

Scope:
- Add fixed schema validation, signed manifest/artifact binding, allowed paths/domains, and declared service ID checks on top of the sandbox/network permission shape from task 2.
- Preserve helper isolation while permitting declared typed jobs only.

Out of scope:
- No arbitrary host command channel, shell execution, sudo cache, or client-supplied service unit execution.
- No Configure OpenClaw closure UI.

Depends on:
- `task-2-helper-outbound-service-prereq`

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §7 and §8
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can prove local policy rejects unknown schemas, invalid manifests/artifacts, out-of-allowlist paths/domains, undeclared service IDs, revoked state, and wrong owner/org.

Parallelism:
- Can run after task 2. Can run alongside task 3 if transport and policy files are separable.

Sensitive paths:
- dangerous-commands, sandbox, credentials, host file/network authority, privacy
