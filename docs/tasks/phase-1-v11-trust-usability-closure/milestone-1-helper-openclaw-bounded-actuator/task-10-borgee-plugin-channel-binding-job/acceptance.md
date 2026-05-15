# Acceptance: Borgee Plugin Channel Binding Job

## Source Alignment

- Task: `task-10-borgee-plugin-channel-binding-job`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Blueprint anchors: `remote-actuator-design.md` sections 1.2 and 7; `migration-analysis.md` sections 4.3 and 6.1
- Dependencies: Task9 PR #956 (`5575b53f657276c57ba319b144281286865db630`)
- Dependency decision: no M2 Task6 blocker; existing channel access checks are sufficient because this task does not implement channel management actions.

## Segment A: Typed Plugin Binding Job

Acceptance checks:

- `borgee_plugin.configure_connection` can enqueue for a fresh claimed Helper with `openclaw_config` delegation.
- Stored payload is server-derived and includes only server-owned `connection_id`, target `agent_id`, and target `channel_id`.
- Stored manifest binding includes the server-owned manifest digest and approved Borgee plugin config path id.

## Segment B: Channel Authority

Acceptance checks:

- Enqueue rejects inaccessible, wrong-org, non-channel, or target-agent-inaccessible channels.
- Owner/org/enrollment alone cannot bind a plugin job to a channel.
- Rejected enqueue attempts do not create executable job rows.

## Segment C: Closed Client Authority

Acceptance checks:

- Client-supplied connection ids, base URLs, API keys, credentials, commands, shells, argv, scripts, executable paths, service units, paths, domains, manifests, artifacts, services, TTLs, and expiry fields are rejected.
- Public enqueue responses do not expose payload body, payload hash, manifest digest, manifest binding, owner/org internals, credentials, or logs.

## Segment D: Helper Policy Alignment

Acceptance checks:

- Helper policy accepts the server-bound plugin connection payload only when payload hash, enrollment state, signed manifest, and approved config path binding validate.
- Helper policy rejects plugin payload schema drift and authority fields.

## Segment E: Docs And Scope

Acceptance checks:

- Task docs and `docs/current` record that the plugin channel binding typed job is current behavior.
- Docs do not claim local plugin config execution, OpenClaw execution, service lifecycle, raw log upload, sudo behavior, terminal UI closure, or Remote Agent rail reuse.

## Evidence

| Segment | Evidence | Result |
|---|---|---|
| A: Typed Plugin Binding Job | Store/API tests prove enqueue for `borgee_plugin.configure_connection`, server-owned connection id, category `openclaw_config`, safe lease projection, and `borgee_plugin_config` manifest path binding. | PASS |
| B: Channel Authority | Store/API tests prove inaccessible target-agent channel binding is rejected before any job row is created and succeeds only after target agent channel access is granted. | PASS |
| C: Closed Client Authority | Store/API tests reject client-supplied connection/base URL/API-key authority and keep public responses free of internal payload/digest/binding fields. | PASS |
| D: Helper Policy Alignment | Helper policy tests prove server-bound plugin payload is allowed with signed manifest/path binding and extra authority fields are schema-invalid. | PASS |
| E: Docs And Scope | Task10 docs and current host/security/server docs updated to describe plugin binding typed jobs without claiming local execution or Configure OpenClaw closure. | PASS |

Verification commands:

- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-server go test -tags sqlite_fts5 -count=1 ./internal/store -run 'TestHelperJobPluginConfigureConnectionIsServerBound|TestHelperJobEnqueueRejectsInactiveDelegationAndClosedTaxonomy'` from `packages/server-go`.
- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-server go test -tags sqlite_fts5 -count=1 ./internal/api -run 'TestHelperJobsEnqueuePluginConfigureConnectionRequiresChannelAuthority|TestHelperJobsEnqueueRejectsUnauthorizedRailsAndInvalidEnvelopes'` from `packages/server-go`.
- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-helper go test -count=1 ./internal/jobpolicy -run 'TestEvaluateAllowsPluginConfigureConnectionWithServerBoundChannelPayload'` from `packages/borgee-helper`.
- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-server go test -tags sqlite_fts5 -count=1 ./internal/store ./internal/api ./internal/datalayer` from `packages/server-go`.
- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task10-helper go test -count=1 ./internal/jobpolicy ./internal/outbound` from `packages/borgee-helper`.
