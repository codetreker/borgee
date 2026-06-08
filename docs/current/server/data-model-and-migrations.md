# Data Model And Migrations

## Role

The server data model is the authoritative persisted memory of the product. It stores identity, channel collaboration, messages, permissions, admin audit, remote-node registration, artifacts, agent runtime descriptors, presence-backed reachability, and event streams. The store is not just a database wrapper; it defines which concepts are durable, which are live-only, and which are append-only audit records.

Migrations define how that durable model evolves. The current architecture keeps an older baseline schema path and a numbered forward-only migration registry. The baseline keeps existing bootstraps and tests stable; the forward-only registry is the schema change mechanism for additive product work.

```mermaid
flowchart TB
  store[(SQLite store)]
  baseline[Baseline schema]
  forward[Forward-only migrations]
  aggregates[Core aggregates]
  hot[Hot realtime events]
  cold[Cold data-layer events]
  lifecycle[Retention and archive lifecycle]

  baseline --> store
  forward --> store
  store --> aggregates
  aggregates --> hot
  aggregates --> cold
  cold --> lifecycle
```

## Boundary

The store owns durable state. The realtime hub owns live socket presence, in-memory connection maps, and transient delivery buffers. The data layer owns abstraction boundaries over store-backed repositories, presence reads, storage, and cold events. These boundaries overlap deliberately, but they are not interchangeable.

Core user collaboration state is persisted in relational aggregates: users/agents, channels/memberships, messages/mentions/reactions, permissions, files, remote nodes, artifacts/versions/comments/iterations, admin records, and agent state tables.

Channel membership carries agent attention policy. `channel_members.require_mention_policy` is a tri-state durable field with `inherit`, `on`, and `off`. `inherit` resolves through the agent user's global `require_mention` flag, `on` forces explicit mention in that channel, and `off` allows non-mention delivery only when the agent's global owner-controlled setting already permits broader delivery. Existing and legacy memberships default to `inherit`; policy changes do not rewrite historical messages or mention rows.

Event state is split. Hot events use a numeric cursor stream for user-facing realtime replay. Cold data-layer events use lexicographic ids and can carry row-level retention metadata for longer-lived event records. A feature must choose the correct stream explicitly.

## Collaborators

The REST layer reads and writes aggregates through store helpers or direct database access where a typed model has not been extracted. It also owns much of the validation around aggregate transitions.

The realtime hub depends on store state for authentication, channel access checks, remote-node lookup, and cursor seeding. It should not become the durable source of truth for collaboration state.

The auth layer depends on user and permission rows. Capability checks interpret permission rows and resource scope, including organization boundaries.

The admin layer depends on its own admin tables and audit tables. Admin sessions and user sessions are intentionally different aggregates. Canonical server audit storage is `audit_events`; `admin_actions` remains the compatibility view and store facade used by existing helpers.

The data layer wraps selected store behavior behind interfaces and provides the cold event writer. It gives newer code a stable seam without requiring every legacy handler to migrate at once.

## Internal Architecture

The storage runtime is SQLite through GORM. File-backed databases run with WAL, busy-timeout, and foreign-key pragmas attached to the SQLite DSN so every pooled connection gets the same concurrency and integrity settings. In-memory test databases use a single connection to avoid isolated per-connection databases. Runtime config can also cap SQLite open connections with `SQLITE_MAX_OPEN_CONNS` or set SQLite transaction locking with `SQLITE_TXLOCK`; the Playwright e2e server uses immediate transaction locking to avoid deferred read-to-write lock upgrade failures under multi-worker load without changing the production default pool behavior.

Server API tests use the same route wiring as production but disable unrelated production background workers in the test harness. Heartbeat, retention, threshold, and archive ticker workers keep their own package-level race coverage; disabling them for generic API handler tests avoids hundreds of duplicate idle goroutines competing with the race detector while preserving the handler, store, auth, websocket, and data-layer signal those tests are meant to cover.

Realtime channel fanout treats channel-scoped frames, including `new_message`, as reliable within a bounded send window instead of silently dropping them when a browser's websocket buffer is briefly full. Global presence-style frames remain best-effort.

The baseline migration creates the original core tables, applies guarded column additions, creates indexes, performs backfills, and cleans up legacy direct-message state. It remains part of boot because the server still supports databases that were born before the numbered migration registry.

The forward-only migration engine is the additive schema mechanism. Each migration has a positive unique version, a name, and an `Up` function. Applied versions are recorded so startup can safely run the registry more than once. There is no rollback path in the engine; corrections are expressed as later migrations.

Core aggregates are intentionally not normalized into one generic resource table. Users, channels, messages, remote nodes, artifacts, admin rows, and agent state each retain domain-specific tables because they carry different ownership, privacy, and retention rules.

Agent state is deliberately multi-part: runtime process metadata, plugin socket liveness, presence sessions, busy/idle task state, and append-only state transitions are separate concepts. Collapsing them would lose information about whether an agent process is registered, connected, reachable, executing work, or historically failed.

## Key Flows

Boot migration flow: opening the store prepares SQLite runtime settings, baseline migration ensures the legacy schema shape, forward-only migrations apply additive schema, and backfills reconcile older rows with current invariants.

Write flow: a handler validates the operation, writes one or more aggregate rows, and then chooses side effects such as hot event rows, WebSocket fanout, cold event publication, audit rows, or push notification. Persistence and fanout are related but not automatically coupled.

Channel member attention-policy flow: a channel manager can update an agent member's per-channel policy through the user rail. The target must be an agent member of the same channel, DM channels are rejected, and cross-org callers fail before permission checks. Setting `off` is rejected when the agent's global `require_mention` remains true, so channel management can reduce or require attention but cannot broaden agent delivery beyond owner authorization. Channel member listing returns both the stored `require_mention_policy` and server-derived `effective_require_mention` so clients can display current delivery state without recomputing authority.

Channel membership and ownership mutations layer domain checks on top of permission rows. Leaving requires the caller to be a current member and not the channel creator. Adding/removing members and changing member attention policy require the manager to be a current channel member, and member removal cannot target the channel creator. Delete/archive require the authenticated user to be the channel creator after the relevant permission check, and cross-org channel management attempts fail closed. Automatic public-channel joins are organization-scoped: registering a user or creating an agent joins only public channels in that user's or agent's org, not public channels owned by other orgs.

`@Everyone` message flow: message creation treats `@Everyone` as a reserved content token and does not accept client-supplied recipient ids. The server computes recipients from channel membership, excludes the sender and soft-deleted users, records the computed targets in `message_mentions`, and dispatches through the same mention fanout path as explicit mentions. Explicit mentions are also parsed from persisted message content, not trusted from client recipient arrays. The flow adds no schema table or migration; it uses existing `channel_members`, `users`, `messages`, and `message_mentions` rows.

Hot event flow: user-facing realtime replay is based on an autoincrement cursor. Polling, streaming, and backfill clients consume cursor-ordered state, while WebSocket frame producers may allocate cursors for live delivery.

Cold event flow: data-layer publishers write to an in-process bus first and asynchronously persist to channel-scoped or global cold event tables. The cold event retention job is started by the server runtime, but its current sweeper only reaps rows with an explicit non-negative `retention_days`. The ordinary cold event writers currently insert without `retention_days`, so the per-kind default policy is not effective for those rows. Archive offload remains a separate cold table lifecycle path.

Admin audit flow: admin actions and impersonation grants are durable audit-oriented records. User-facing audit views and admin-facing audit views are different projections over related audit data.

## Invariants

- The SQLite store is the canonical persisted source for server-owned state.
- Baseline migration may remain for compatibility, but additive schema belongs in numbered forward-only migrations.
- Forward migrations are immutable once applied; changes are made by appending a later migration.
- Admin identity is stored outside the user aggregate.
- Agents are users for ownership and API-key purposes, but agent runtime state is stored in separate runtime/state aggregates.
- Hot cursor events and cold data-layer events are separate streams with different identifiers and retention behavior; default per-kind cold retention is policy intent, not current behavior for rows written without `retention_days`.
- Append-only audit/state-log tables should not be rewritten to hide history.
- Organization and ownership fields are part of authorization, not merely display metadata.
- Channel-level agent attention policy is membership-scoped. It can narrow attention locally, but it cannot override the agent owner's global require-mention ceiling.
- `@Everyone` recipient history is server-computed from channel membership. Request-body recipient ids are rejected on message create.

## Non-Goals

The data model does not model plugin-local runtime secrets, LLM provider configuration, or a universal event table for all delivery paths.

## Implementation Anchors

- `packages/server-go/internal/store/db.go`
- `packages/server-go/internal/store/models.go`
- `packages/server-go/internal/store/migrations.go`
- `packages/server-go/internal/store/queries.go`
- `packages/server-go/internal/store/require_mention_policy.go`
- `packages/server-go/internal/api/messages.go`
- `packages/server-go/internal/api/mention_dispatch.go`
- `packages/server-go/internal/store/admin_actions.go`
- `packages/server-go/internal/store/agent_state_log.go`
- `packages/server-go/internal/migrations/migrations.go`
- `packages/server-go/internal/migrations/registry.go`
- `packages/server-go/internal/migrations/admin_admins.go`
- `packages/server-go/internal/migrations/admin_sessions.go`
- `packages/server-go/internal/migrations/agent_runtimes.go`
- `packages/server-go/internal/migrations/agent_state_log.go`
- `packages/server-go/internal/migrations/canvas_artifacts.go`
- `packages/server-go/internal/migrations/canvas_artifact_iterations.go`
- `packages/server-go/internal/migrations/channel_events.go`
- `packages/server-go/internal/migrations/global_events.go`
- `packages/server-go/internal/migrations/channel_member_require_mention_policy.go`
- `packages/server-go/internal/datalayer/factory.go`
- `packages/server-go/internal/datalayer/v1_sqlite.go`
- `packages/server-go/internal/datalayer/events_store.go`
- `packages/server-go/internal/datalayer/events_retention.go`
- `packages/server-go/internal/datalayer/events_archive_offloader.go`
- `store.Store`
- `migrations.Engine`
- `datalayer.DataLayer`
