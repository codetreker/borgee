# Host Bridge

Host Bridge is the local host capability path. It is designed for actions that need stronger host mediation than Remote Agent provides: user-granted capabilities, helper-side ACL decisions, local IPC, local audit, Helper enrollment/status identity, and platform sandboxing where available. It is not the remote filesystem WebSocket path.

## Overview

**Role**
Host Bridge lets Borgee-controlled agents request limited host capabilities through a local helper. The server owns user consent records and Helper enrollment/status rows, the helper owns local enforcement and local job policy decisions, and the installer owns deployment of the helper runtime.

**Boundary**
The current request boundary is a grant-backed helper request. A request must identify the agent, match the connection's agent identity, normalize to a grant scope, pass grant lookup, and then pass local OS/process constraints before host IO is attempted. Helper enrollment is a separate server-side identity/status boundary: it binds owner, org, enrollment id, helper device id, host label, allowed categories, and terminal revoke/uninstall state before later host-management work can rely on a Helper identity. Server-side Helper jobs now have a user-rail typed enqueue boundary plus a Helper-credential outbound poll/lease/ack/result boundary. The Helper service has outbound HTTPS and local state prerequisites, with exact public-origin startup validation that rejects localhost/private/link-local/metadata literal origins by default, explicit Helper-owned state roots, and bounded boot/crash restart settings. A pure local Helper job-policy evaluator validates delivered server-owned job views against schema, enrollment state, signed manifest/artifact binding, declared paths/domains/services, and supplied sandbox/profile affordances; the current stack still does not add OpenClaw execution, bounded log upload, service-manager action execution, or sudo-cache behavior.

**Collaborators**
Host Bridge collaborates with the user API for grants and Helper enrollment management, server storage for grant and enrollment state, the helper daemon for enforcement, the installer for deployment, and admin audit views for limited visibility. It does not collaborate with the remote-agent WebSocket token path.

The user SPA includes a read-only Helper status sidepane backed by the user Helper enrollment API. It shows server-known Helper enrollment status, last seen, and allowed categories without exposing Helper credentials or treating Helper status as Remote Agent status.

**Internal Architecture**

- Grant control plane: user-owned rows describing host capability consent.
- Helper enrollment control plane: owner/org-scoped rows describing enrolled Helper identity, allowed category visibility, device id, current credential lifecycle metadata, last seen, revoke, and helper-originated uninstall status.
- Helper job enqueue control plane: human/member owner/org/enrollment-scoped typed `helper_jobs` rows with server TTL, category gate, internal normalized payload digest, server-owned manifest binding, and active-window idempotency. Public enqueue responses expose queued metadata only, not payload or manifest digests. OpenClaw Configure can enqueue `openclaw.configure_agent`, OpenClaw install can enqueue `openclaw.install_from_manifest`, Borgee plugin channel binding can enqueue `borgee_plugin.configure_connection`, and OpenClaw service restart intent can enqueue `service.lifecycle`; these use server-derived effective payloads and approved manifest/path, artifact/domain, or service-ID bindings. Plugin binding jobs recheck owner/org/channel and target-agent channel access before queueing. State write, status collect, delegation revoke, and helper uninstall job types remain disabled until their task-owned authority exists.
- Helper job transport rail: Helper-credential `poll`, `ack`, and `result` routes atomically lease one queued job, move receipt to running, and settle terminal statuses with bounded redacted metadata. The rail checks current Helper credential, helper device id, enrollment status, lease token, TTL, lease expiry, non-success reason codes, redacted failure messages, bounded audit/log references, and terminal idempotency.
- Helper data plane: local UDS IPC carrying agent-scoped requests, plus outbound-only Helper job HTTP client construction from validated prerequisite config.
- Helper local job-policy gate: pure pre-action evaluator for delivered server-owned job views. It returns allow/deny reasons and does not perform IO, OpenClaw actions, service-manager calls, bounded log upload, or terminal settlement on its own.
- Enforcement stack: handshake identity, action allowlist, path/scope normalization, grant lookup, read-only IO, audit, sandbox.
- Installer path: current manifest verifier path, local operator confirmation, and platform service deployment.

**Key Flows**

```text
Grant flow:
  user grants capability -> server stores host grant -> helper sees it on next lookup

Helper enrollment flow:
  user creates enrollment -> local helper claims with one-time secret/device id
  -> server returns persistent Helper credential once -> helper can rotate the credential
  -> helper heartbeat updates last seen with the current credential
  -> user revoke or helper-originated uninstall makes the enrollment terminal

Helper status UI flow:
  user opens Helper Status -> browser lists redacted Helper enrollments
  -> UI renders connected/offline/revoked/uninstalled/pending, last seen, and allowed categories
  -> no browser claim/status/uninstall Helper credential call and no Configure OpenClaw success claim

Helper job enqueue flow:
  human/member user posts typed job envelope -> server derives owner/org/enrollment
  -> server validates fresh claimed Helper, category, job type, payload, channel target-agent access when binding, config/plugin binding, install runtime intent, TTL, idempotency
  -> server stores queued metadata plus server-owned manifest binding only; user response does not expose payload, manifest digest, credentials, owner/org internals, or logs

Helper pull/lease/result flow:
  helper polls outbound with current Helper credential + helper_device_id
  -> server atomically leases one queued job and returns a safe effective payload, manifest digest/binding when present, and opaque lease token
  -> helper acks receipt only -> later local policy/action handoff remains outside this task
  -> helper uploads terminal transport metadata with closed reason codes, redacted bounded failure message, and opaque audit/log refs only
  -> repeated matching ack/result is idempotent and conflicting terminal replay is rejected

Helper outbound prerequisite flow:
  installed service starts helper with exact Borgee server origin, allowed origin list, and Helper-owned state dirs
  -> daemon normalizes and validates origin/state roots fail-closed, rejecting local/private/link-local/metadata literal origins in production defaults
  -> Linux systemd allows AF_UNIX plus IPv4/IPv6 only; macOS sandbox permits remote TCP while preserving local UDS-only inbound
  -> no HTTP poll request is made in this task

Helper local policy flow:
  delivered server-owned Helper job view -> strict typed payload validation
  -> local owner/org/enrollment/device/credential/category/revocation/expiry recheck
  -> signed runtime manifest digest and artifact cache digest validation when required
  -> declared path/domain/service binding checked against supplied sandbox/profile affordances
  -> deterministic allow/deny reason only; no action, poll, lease, result upload, or settlement

Helper request flow:
  local client connects -> handshake agent id -> request action/target
  -> ACL decision -> SQLite grant lookup -> IO or rejection -> local audit

Install flow:
  installer fetches manifest -> runs current verifier path -> user confirms
  -> package manager installs the local artifact path -> platform service starts daemon
```

**Invariants**

- User consent is represented as host grants, not as generic user API capabilities.
- Helper enrollment is represented as `helper_enrollments`, not as Remote Agent nodes, host grants, or user permissions.
- Helper enforcement is per request; grant state is not cached in the helper decision path.
- Helper enrollment status and credential rotation are identity/status only; they do not execute jobs or prove Configure OpenClaw success.
- Helper job enqueue stores typed queued metadata only. Helper job transport can lease, ack, and settle terminal metadata with redacted failure text and opaque audit/log references; it does not execute jobs, evaluate local policy, collect raw logs, call service managers, or prove Configure OpenClaw success.
- Helper local job policy is a second authority check after enqueue and before any future action. It rejects unknown job types, schema drift, extra or forbidden payload fields, invalid signed manifests, artifact digest mismatches, undeclared paths/domains/services, revoked/stale state, wrong owner/org, and sandbox/profile mismatches.
- Helper outbound service prerequisites are Helper-rail only: they use Helper startup config and Helper-owned state paths, not Remote Agent credentials, reverse WebSocket transport, host grants, file-proxy status, or permission fallbacks.
- Helper status UI is read-only enrollment visibility; it is not job progress, bounded logs, OpenClaw connectivity, or service lifecycle status.
- Helper filesystem IO is read-only in the current capability set.
- Remote Agent and Host Bridge are separate capabilities with separate credentials, transports, and boundaries.
- Server-side host grant ownership does not imply admin-wide override.

## Submodules

- [helper-daemon.md](helper-daemon.md) defines local enforcement: UDS IPC, ACL, SQLite grant lookup, audit, sandbox, and read-only IO.
- [host-grants.md](host-grants.md) defines the server-side consent model and its invariants.
- [installer.md](installer.md) defines package installation, the manifest verifier path, and deployment responsibilities.

## Out Of Scope

Host Bridge does not provide Remote Agent browsing, plugin WebSocket API tunneling, unrestricted command execution, Configure OpenClaw execution status, bounded log upload, service-manager action execution, sudo cache, or admin-owned host consent. Local job policy exists as a pure pre-action evaluator; transport can deliver and settle job metadata, but no host action path is wired here.

## Known Gaps

- Runtime authorization and platform sandboxing do not have identical update lifecycles; [helper-daemon.md](helper-daemon.md) owns the daemon-level details.
- Deployment trust and runtime authorization are separate boundaries; [installer.md](installer.md) owns installer trust details.
- Helper outbound validation does not resolve allowed hostnames or inspect DNS answers/CNAMEs. The installed production allowlist is exactly `https://app.borgee.io`, but DNS rebinding or private/link-local/metadata resolution remains future hardening or runtime network-policy scope.
- Helper enrollment has identity/status and current-credential rotation handling. Helper job enqueue, outbound poll-and-lease, ack, bounded redacted terminal result settlement, server-owned OpenClaw install/config, Borgee plugin channel binding, service lifecycle service-ID binding, bounded Helper boot/crash restart assets, and pure local job-policy evaluation are current behavior. Local plugin config write execution, service-manager action execution, local uninstall action execution, raw/bulk log upload, policy-to-action wiring, and OpenClaw execution are not current behavior.

## Implementation Anchors

- `packages/server-go/internal/api/host_grants.go` (`HostGrantsHandler`)
- `packages/server-go/internal/api/helper_enrollments.go` (`HelperEnrollmentHandler`)
- `packages/server-go/internal/api/helper_jobs.go` (`HelperJobsHandler`)
- `packages/server-go/internal/migrations/host_grants.go` (`host_grants` schema)
- `packages/server-go/internal/migrations/helper_enrollments.go` (`helper_enrollments` schema)
- `packages/server-go/internal/migrations/helper_jobs.go` (`helper_jobs` schema)
- `packages/server-go/internal/store/helper_enrollment_queries.go`
- `packages/server-go/internal/store/helper_job_queries.go`
- `packages/borgee-helper/cmd/borgee-helper/main.go`
- `packages/borgee-helper/internal/ipc` (`Request`, `Response`, `Handler`)
- `packages/borgee-helper/internal/acl` (`Gate`, `Decision`)
- `packages/borgee-helper/internal/grants` (`SQLiteConsumer`)
- `packages/borgee-helper/internal/fileio`
- `packages/borgee-helper/internal/audit`
- `packages/borgee-helper/internal/sandbox`
- `packages/borgee-helper/internal/jobpolicy`
- `packages/borgee-installer/cmd`
- `packages/borgee-installer/internal`
