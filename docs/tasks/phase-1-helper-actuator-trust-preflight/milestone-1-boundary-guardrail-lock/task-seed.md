# Task Seed: task-0-hb-ra-1a-planning-preflight

## Purpose

Prepare the first post-breakdown docs-only task for `HB-RA-1A` boundary guardrail lock without starting helper actuator implementation. The task proves the team can use a concrete guardrail preflight checklist before any code, API, queue, credential, sandbox, or service work begins.

## Source Anchor

- Locked anchor: `docs/blueprint/next/README.md` `HB-RA-1A`, lines 13 and 31-45.
- Explicitly excluded anchor: `docs/blueprint/next/README.md` `HB-RA-1B`, lines 14 and 47-56.

## Expected Post-Breakdown Task

`bf-milestone-breakdown` should first create a reviewed task contract for `task-0-hb-ra-1a-planning-preflight`. After that `task.md` exists, the first executable PR atom is a docs-only preflight evidence task.

Expected post-breakdown task folder:

```text
docs/tasks/phase-1-helper-actuator-trust-preflight/
`-- milestone-1-boundary-guardrail-lock/
    `-- task-0-hb-ra-1a-planning-preflight/
        `-- task.md
```

That task's PR should add a reviewer-usable evidence artifact, for example:

```text
docs/tasks/phase-1-helper-actuator-trust-preflight/
`-- milestone-1-boundary-guardrail-lock/
    `-- task-0-hb-ra-1a-planning-preflight/
        |-- task.md
        `-- guardrail-preflight.md
```

The evidence artifact should contain a checklist and reverse-check table that reviewers can apply before any implementation task starts. It should prove the bounded Configure OpenClaw promise, list the visible user trust signals, reject fake-green outcomes, and keep `HB-RA-1B` execution-contract blockers out of scope.

## Expected PR Atom

One task PR should add the preflight evidence artifact and any task-local progress note required by the task contract. It should not add code, API definitions, queue behavior, credential models, sandbox profiles, service lifecycle behavior, or product-blueprint changes.

## Pre-Conditions

- `HB-RA-1A` remains locked and still says Phase planning is guardrail-only.
- `HB-RA-1B` remains excluded from the task contract unless it has separately locked in `docs/blueprint/next/README.md`.
- The legacy `681-remote-agent-openclaw/` intake folder is not used as the execution path.

## First Acceptance Check

The first task PR passes when the new preflight evidence artifact lets a reviewer answer all of these before implementation tasks start:

- What can the user approve? Bounded Web-side Configure OpenClaw delegation after explicit local enrollment.
- What can the user perceive? Helper connected state, allowed job categories, progress, truthful terminal status, failure reasons, bounded redacted logs, revoke, and uninstall.
- What is still prohibited? No command channel, no credential reuse, no sudo cache, no Helper/Remote Agent rail merge, and no `HB-RA-1B` execution-contract decisions.
- What blocks fake green? Denied, revoked, stale, cancelled, expired, queued, leased, running, or failed work cannot be represented as success or left spinning indefinitely.
- Where is carry-over parked? Manifest/artifact signing, helper credential shape, sandbox/Linux poll details, revoke race mechanics, service permissions, and exact queue/lease/result semantics remain in `HB-RA-1B` until separately locked.

## Non-Goals

- Do not change code.
- Do not change `docs/blueprint/current/` or `docs/blueprint/next/`.
- Do not decide manifest signing, artifact binding, helper credential shape, sandbox paths, Linux poll permissions, revoke race mechanics, service permissions, queue states, lease duration, or result schema.
- Do not create implementation acceptance for Configure OpenClaw behavior.
