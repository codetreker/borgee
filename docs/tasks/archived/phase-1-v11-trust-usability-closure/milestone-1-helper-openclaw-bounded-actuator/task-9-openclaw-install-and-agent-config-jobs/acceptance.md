# Acceptance: OpenClaw Install And Agent Config Jobs

## Source Alignment

- Task: `task-9-openclaw-install-and-agent-config-jobs`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Blueprint anchors: `remote-actuator-design.md` sections 1.2, 7, and 9
- Dependencies: Task6 PR #943 (`c2c61e6e8500218ae0e841a9edde3f1187c78c7d`), Task7 PR #942 (`642fb5761b141a633169f39e31f77931bf85f0c1`), and Task8 PR #954 (`419c5bf57637941df5670f08615304e4a9ef8277`)

## Segment A: OpenClaw Install Job

Acceptance checks:

- `openclaw.install_from_manifest` can enqueue for a fresh claimed Helper with `openclaw_lifecycle` delegation.
- Stored payload is server-derived and includes only the server-owned install plan ID.
- Stored manifest binding includes the server-owned manifest digest, OpenClaw plugin artifact ID, approved install/config path IDs, and approved artifact origin.
- Binding does not include service IDs in Task9.

## Segment B: OpenClaw Agent Config Job

Acceptance checks:

- `openclaw.configure_agent` continues to enqueue for `openclaw_config` delegation.
- Stored payload is server-derived from `agent_configs` and includes the target agent, optional channel id, config schema version, and config hash.
- Stored manifest binding includes server-owned approved OpenClaw config path binding.

## Segment C: Closed Client Authority

Acceptance checks:

- Client-supplied command, shell, argv, script, executable path, path, URL, domain, manifest ID/digest/binding, artifact IDs, service IDs/units, credentials, config hashes, install plan IDs, TTLs, and expiry fields are rejected.
- Rejected enqueue attempts do not create executable job rows.

## Segment D: Helper Policy Alignment

Acceptance checks:

- Helper policy requires signed manifest plus approved config path binding for `openclaw.configure_agent`.
- Helper policy still requires signed manifest, artifact digest, approved paths, approved origin, and sandbox affordances for `openclaw.install_from_manifest`.

## Segment E: Docs And Scope

Acceptance checks:

- `docs/current` describes current OpenClaw install/config job records and Helper policy requirements.
- `docs/current` does not claim OpenClaw execution, Borgee plugin channel binding, service lifecycle, raw log upload, sudo behavior, terminal UI closure, or Remote Agent rail reuse.

## Evidence

| Segment | Evidence | Result |
|---|---|---|
| A: OpenClaw Install Job | Store/API tests prove install enqueue for `openclaw_lifecycle`, server-derived install payload, manifest/artifact/path/domain binding, Helper lease projection, and no Task9 service IDs. | PASS |
| B: OpenClaw Agent Config Job | Store tests prove configure-agent still enqueues for `openclaw_config` and stores server-owned config-path manifest binding alongside server-derived config payload. | PASS |
| C: Closed Client Authority | Store/API tests reject client-supplied manifest, artifact, path, domain, URL, service, command, credential, install plan, config hash, TTL, and expiry authority. | PASS |
| D: Helper Policy Alignment | Helper policy tests prove configure-agent now requires signed manifest plus approved config path binding; install manifest/artifact/path/domain policy tests remain green. | PASS |
| E: Docs And Scope | Updated `docs/current` host-bridge, helper-daemon, known-gaps, security, and server docs to describe OpenClaw install/config job records without claiming execution, channel binding, service lifecycle, sudo, raw log upload, or Remote Agent rail reuse. | PASS |

Verification commands:

- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-server go test -tags sqlite_fts5 -count=1 ./internal/store ./internal/api ./internal/datalayer` from `packages/server-go`.
- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-server go test -tags sqlite_fts5 -count=1 ./...` from `packages/server-go`.
- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-helper go test -count=1 ./internal/jobpolicy ./internal/outbound` from `packages/borgee-helper`.
- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-helper go test -count=1 ./...` from `packages/borgee-helper`.
- `git diff --check` from repo root.
