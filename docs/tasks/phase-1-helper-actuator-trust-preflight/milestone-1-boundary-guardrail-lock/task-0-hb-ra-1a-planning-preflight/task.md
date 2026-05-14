# task-0-hb-ra-1a-planning-preflight

Purpose:
- Create a docs-only guardrail preflight artifact that reviewers can use before any Helper actuator implementation task starts.
- Prove the bounded Configure OpenClaw promise from `HB-RA-1A` stays reviewable without pulling in `HB-RA-1B` execution-contract blockers.

Scope:
- Add task-local preflight evidence for the `HB-RA-1A` boundary guardrail lock.
- Record what a user may approve after explicit local enrollment: bounded Web-side Configure OpenClaw delegation through closed, schema-bound Helper jobs.
- Record user-perceivable trust signals: Helper connected state, allowed job categories, progress, truthful terminal status, failure reasons, bounded redacted logs, revoke, and uninstall.
- Carry `PS-1` as a reverse-check guardrail for privacy, security, admin, audit/enforcement, data-minimization, capability, and Helper/Remote Agent rail separation.
- Keep `HB-RA-1B` blockers parked until separately locked: manifest/artifact signing, helper credential shape, sandbox/Linux poll details, revoke race mechanics, service permissions, and exact queue/lease/result semantics.

Out of scope:
- Code, API definitions, queue behavior, credential models, sandbox profiles, service lifecycle behavior, product-blueprint changes, and `docs/blueprint/current/` changes.
- Arbitrary shell, argv, executable path, script, service-unit dispatch, or any host command channel.
- Remote Agent credential reuse, grant reuse, enforcement-rail merge, or shared token assumptions.
- Sudo caching, privileged long-lived Helper/OpenClaw services, or hidden privilege escalation.
- Treating `HB-RA-1B` execution-contract blockers as accepted design.
- Using `PS-1` to weaken existing admin, privacy, security, impersonation, audit/enforcement, data-minimization, capability, or rail-separation controls.

Depends on:
- none

Blueprint anchors:
- `HB-RA-1A`: locked Helper bounded actuator product guardrails in `docs/blueprint/next/README.md` lines 13 and 31-45.
- `PS-1`: locked privacy scope guard in `docs/blueprint/next/README.md` lines 18 and 99-108.
- `HB-RA-1B` exclusion: open Helper actuator execution contract blockers in `docs/blueprint/next/README.md` lines 14 and 47-56.

Acceptance slice:
- A reviewer can answer what the user may approve: bounded Web-side Configure OpenClaw delegation after explicit local enrollment.
- A reviewer can answer what the user can perceive: Helper connected state, allowed job categories, progress, truthful terminal status, failure reasons, bounded redacted logs, revoke, and uninstall.
- A reviewer can point to prohibited outcomes: no command channel, no credential reuse, no sudo cache, no Helper/Remote Agent rail merge, and no `HB-RA-1B` execution-contract decisions.
- A reviewer can point to the `PS-1` boundary: no new user-facing privacy/compliance product expansion, while existing controls remain intact.
- The preflight evidence rejects fake green: denied, revoked, stale, cancelled, expired, queued, leased, running, or failed work cannot be represented as success or left spinning indefinitely.
- The preflight evidence parks carry-over in `HB-RA-1B`: manifest/artifact signing, helper credential shape, sandbox/Linux poll details, revoke race mechanics, service permissions, and exact queue/lease/result semantics.

Parallelism:
- First product task for this milestone; blocks Helper actuator implementation-task breakdown until reviewed.
- Can run after milestone breakdown reaches `TASK_SET_READY`.

Sensitive paths:
- Required Security review: auth and authorization boundary, helper credentials, Remote Agent credential separation, privacy/redacted logs, admin/privacy/security control preservation, dangerous-command exclusion, service privilege boundary, revoke/uninstall trust boundary, and project-sensitive Helper/Remote Agent rail separation.
