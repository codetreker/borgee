# HB-3 — host_grants schema source + contextual authorization

> **Source-of-truth pointer.** Schema in
> `packages/server-go/internal/migrations/hb_3_1_host_grants.go` (v=27).
> REST endpoints in `packages/server-go/internal/api/host_grants.go`.
> Client SPA in `packages/client/src/components/HostGrantsPanel.tsx`.
> Route registration at server boot in
> `packages/server-go/internal/server/server.go`.

## Why

Plugin runtime needs OS-level resources (filesystem read, network egress)
that platform permissions (`user_permissions`) do not model. HB-3 ships
`host_grants` as a separate source for host-level authorization, so daemon
(install-butler + host-bridge) consumers have one read-only path
without polluting the platform-level permission schema.

## Principles (host-bridge.md §1.3 + §1.5 + §2)

| Constraint | Contract |
|---|---|
| Schema source | HB-3 owns the schema. HB-2 daemon (Go module `packages/borgee-helper/`) and install-butler are read-only consumers. server-go `internal/api/host_grants.go` is the only INSERT/UPDATE/DELETE path. |
| Separate dictionaries (host vs runtime) | `host_grants` and AP-1 `user_permissions` have disjoint field sets. AST scan check: the handler must not reference the `user_permissions` identifier; the schema must not add `permission` / `is_admin` / `cursor` / `org_id` / `runtime_id` columns. |
| Audit log 5-field source | `actor / action / target / when / scope` must stay aligned with HB-1 install audit, HB-2 host-IPC audit, and BPP-4 #499 dead-letter. A schema change must update related tests for HB-1, HB-2, BPP-4, and HB-3. This matches the HB-4 §1.5 release criteria line 4 check for the audit-log JSON schema. |
| Revoke < 100ms | HB-4 §1.5 release criteria line 5. v1 implementation: REST DELETE sets `revoked_at` NOT NULL, and the daemon rechecks on every SELECT with no cache. This follows the HB-1 manifest no-cache and HB-2 §4.3 pattern. |
| Forward-only revoke | DELETE does not hard-delete rows. It stamps `revoked_at` for audit retention, matching host-bridge.md §2 trust pillar 3. |
| No admin-wide access | User authorization remains user-sovereign (host-bridge.md §1.3 + ADM-0 §1.3 guardrail). Handler code must not add admin host-grant paths. |
| Attempt once, no retry queue | Follow BPP-4 #499 §0.3. The grant path must not add `pendingGrants` / `grantQueue` / `deadLetterGrants`; it shares constraints with BPP-4 dead_letter_test and BPP-5 reconnect_handler_test. |

## Schema (migration v=27)

```sql
CREATE TABLE host_grants (
  id          TEXT    PRIMARY KEY,
  user_id     TEXT    NOT NULL,
  agent_id    TEXT,                                          -- NULL for install/exec
  grant_type  TEXT    NOT NULL CHECK (grant_type IN
              ('install','exec','filesystem','network')),
  scope       TEXT    NOT NULL,                              -- opaque to server; exact helper lookup value
  ttl_kind    TEXT    NOT NULL CHECK (ttl_kind IN
              ('one_shot','always')),
  granted_at  INTEGER NOT NULL,                              -- Unix ms
  expires_at  INTEGER,                                       -- one_shot: now+1h; always: NULL
  revoked_at  INTEGER                                        -- NULL until revoked
);
CREATE INDEX idx_host_grants_user_id  ON host_grants(user_id);
CREATE INDEX idx_host_grants_agent_id ON host_grants(agent_id) WHERE agent_id IS NOT NULL;
```

## Endpoints

| Method | Path                                | Purpose                          | ACL          |
|--------|-------------------------------------|----------------------------------|--------------|
| POST   | `/api/v1/host-grants`               | Create grant (insert row)        | owner-only   |
| GET    | `/api/v1/host-grants`               | List active grants for caller    | owner-only   |
| DELETE | `/api/v1/host-grants/{id}`          | Revoke (stamp `revoked_at`)      | owner-only   |

POST body:
```json
{
  "agent_id": "<uuid>",        // optional; install/exec is user-level
  "grant_type": "filesystem",  // install | exec | filesystem | network
  "scope": "fs:/home/user/code",  // opaque to server; must exactly match helper lookup scope
  "ttl_kind": "always"         // one_shot | always
}
```

## DOM ↔ DB Enum Mapping

| Button label | data-action          | data-hb3-button | DB ttl_kind |
|--------------|----------------------|-----------------|-------------|
| 拒绝         | `deny`               | `danger`        | (none, no row written) |
| 仅这一次     | `grant_one_shot`     | `primary`       | `one_shot`  |
| 始终允许     | `grant_always`       | `primary`       | `always`    |

DOM data-action values map to enum literals directly: `grant_one_shot` ↔
`one_shot`, `grant_always` ↔ `always`. A frontend change also requires a schema
CHECK change and related content-rule updates.

## Audit log keys

| Key                       | Trigger                              |
|---------------------------|--------------------------------------|
| `host_grants.granted`     | POST success                         |
| `host_grants.revoked`     | DELETE success                       |

Each log includes `actor / action / target / when / scope` keys aligned with
HB-1/HB-2/BPP-4 audit schema.

## Tests

- `internal/migrations/hb_3_1_host_grants_test.go` — 7 unit tests
  (table shape + 4-enum CHECK + 2-enum CHECK + no platform-domain columns +
  indexes + idempotent + version=27).
- `internal/api/host_grants_test.go` — 8 unit tests (POST success
  filesystem + one_shot expires_at + grant_type/ttl_kind reject +
  GET list + DELETE revoke + cross-user 403 + AST scan
  user_permissions absence + AST scan grant-queue absence + AST scan
  audit 5-field).
- `packages/client/src/__tests__/HostGrantsPanel.test.tsx` — 5
  vitest cases (data-action + hb3-button + button text alignment
  + actionLabel 4-enum + synonym absence + three-value onDecide).

Regression rows: `REG-HB3-001..011` in
`docs/qa/regression-registry.md`.

## HB-2 daemon read-path contract (Go, packages/borgee-helper/)

HB-2 host-bridge daemon (Go module `packages/borgee-helper/`)
looks up the exact scoped value stored in `host_grants.scope`. The server treats
`scope` as opaque data, but helper lookup expects values such as `fs:<path>` or
`egress:<host>` and uses one SELECT:

```sql
SELECT id, scope, expires_at, granted_at, revoked_at
FROM host_grants
WHERE agent_id = ? AND scope = ? AND revoked_at IS NULL
ORDER BY granted_at DESC LIMIT 1;
```

Expiration is checked afterward in Go. Revoked rows are filtered out by
`revoked_at IS NULL`, so helper lookup treats them as not found; server revoke
still stamps `revoked_at`. Daemon does not write or cache. CI expects
`host_grants.*INSERT|host_grants.*UPDATE` in `packages/borgee-helper/`
must find no matches. Package entry point → [`../../borgee-helper.md`](../../borgee-helper.md).

## Adding a new grant_type

1. Update the actionLabel map in server prose.
2. Update CHECK constraint in `hb_3_1_host_grants.go` migration —
   actually NO, migration is immutable; ship a new migration that
   ALTERs (forward-only).
3. Update `hostGrantTypeWhitelist` in `host_grants.go`.
4. Update `actionLabel` map in `HostGrantsPanel.tsx`.
5. Update server prose, spec §1, and acceptance §1.2 in sync.
6. CI catches divergence through reflect (existing PRAGMA test) and enum
   coverage (`TestHB31_GrantTypeEnumReject` enumerates 4-list).
