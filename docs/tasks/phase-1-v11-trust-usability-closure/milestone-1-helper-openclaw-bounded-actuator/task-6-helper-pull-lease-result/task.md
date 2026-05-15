# task-6-helper-pull-lease-result

Purpose:
- Make enrolled Helper retrieve and settle server-authorized jobs through outbound-only transport.

Scope:
- Add Helper outbound poll/long-poll, lease, ack, result upload, retry/backoff, idempotency, cancellation, and stale credential handling using the service/sandbox prerequisite from task 5.
- Preserve that the server never dials the host.

Out of scope:
- No local policy execution beyond envelope handling.
- No OpenClaw install/config action or service lifecycle operation.

Depends on:
- `task-5-helper-outbound-service-prereq`

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §6, §8, and §10

Acceptance slice:
- A reviewer can verify Helper uses outbound-only transport, leases work once, reports terminal results, and stops on stale or revoked authority.

Parallelism:
- Blocks task 8. Can run alongside task 7 after task 5 if Helper transport and local policy files are separable.

Sensitive paths:
- credentials, host authority, revocation, network boundary
