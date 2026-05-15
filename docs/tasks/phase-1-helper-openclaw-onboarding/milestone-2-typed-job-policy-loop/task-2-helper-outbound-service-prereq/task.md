# task-2-helper-outbound-service-prereq

Purpose:
- Make the long-lived Helper service capable of outbound polling without weakening sandbox or privilege boundaries.

Scope:
- Resolve service permission and sandbox prerequisites for Helper outbound poll/long-poll, including the Linux AF_UNIX-only restriction called out by the blueprint.
- Define allowed network domains, write paths needed for queue/status state, and service-level permission boundaries for the later pull loop.

Out of scope:
- No job lease/result implementation, local policy execution, OpenClaw action, service lifecycle restart feature, or sudo cache.

Depends on:
- `task-1-job-envelope-and-enqueue-authority`

Blueprint anchors:
- `HB-RA-1A`: `docs/blueprint/next/remote-actuator-design.md` §1.2
- `HB-RA-1B`: `docs/blueprint/next/remote-actuator-design.md` §8, §9, and §14
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can verify the Helper service has the minimum sandbox/network/service permissions needed for outbound poll while preserving non-sudo long-lived service behavior and denying inbound server dial or arbitrary host control.

Parallelism:
- Runs after task 1. Blocks Helper pull and local policy tasks because both depend on the service/sandbox permission shape.

Sensitive paths:
- sandbox, network boundary, service authority, privilege boundary, credentials
