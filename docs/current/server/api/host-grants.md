# HB-3 — host_grants schema 单一来源 + 情境化授权

> **单一来源 pointer.** Schema in
> `packages/server-go/internal/migrations/hb_3_1_host_grants.go` (v=27).
> REST endpoints in `packages/server-go/internal/api/host_grants.go`.
> Client SPA in `packages/client/src/components/HostGrantsPanel.tsx`.
> Route registration at server boot in
> `packages/server-go/internal/server/server.go`.

## Why

Plugin runtime needs OS-level resources (filesystem read, network egress)
that platform权限 (`user_permissions`) doesn't model. HB-3 ships
`host_grants` — a separate 单一来源 for host-level授权 — so daemon
(install-butler + host-bridge) consumers have a single read-only path
without polluting the platform-level permission schema.

## 原则 (蓝图 host-bridge.md §1.3 + §1.5 + §2 原文)

- **schema 单一来源.** HB-3 持 ownership; HB-2 daemon (Go module
  `packages/borgee-helper/`, #617 已 merged) + install-butler
  read-only consumer. server-go `internal/api/host_grants.go` 是唯一
  INSERT/UPDATE/DELETE 路径.
- **字典分立 (host vs runtime).** `host_grants` 跟 AP-1
  `user_permissions` 字段集不交. AST scan反向断言: handler 不引用
  `user_permissions` identifier; schema 不挂 `permission` / `is_admin`
  / `cursor` / `org_id` / `runtime_id` 列.
- **audit log 5 字段跨四 milestone 同源.** `actor / action / target /
  when / scope` byte-identical 跟 HB-1 install audit + HB-2 host-IPC
  audit + BPP-4 #499 dead-letter. 改 = 改四处单测锁定 (HB-1 + HB-2 +
  BPP-4 + HB-3 = 第 4 处锁定). 跟 HB-4 §1.5 release criteria 第 4
  行 "审计日志格式锁定 JSON schema" 检查同源.
- **撤销 < 100ms** (HB-4 §1.5 release criteria 第 5 行) — v1 实现:
  REST DELETE → `revoked_at` NOT NULL + daemon 每次 SELECT 守
  (不缓存; 跟 HB-1 manifest 不缓存 + HB-2 §4.3 同模式).
- **forward-only revoke.** DELETE 不真删行 — stamp `revoked_at` 留作
  audit (蓝图 §2 信任五支柱第 3 条).
- **admin-wide access 不入** — 用户授权是用户主权 (蓝图 §1.3 + ADM-0 §1.3
  红线). grep 检查 `admin.*host_grant` 0 hit.
- **attempt once, no retry queue** (跟 BPP-4 #499 §0.3 设计沿用). AST
  scan reverse-grep 检查 forbids `pendingGrants` / `grantQueue` /
  `deadLetterGrants` (第 3 处相关锁定, 跟 BPP-4 dead_letter_test +
  BPP-5 reconnect_handler_test 测试约束同源).

## Schema (migration v=27)

```sql
CREATE TABLE host_grants (
  id          TEXT    PRIMARY KEY,
  user_id     TEXT    NOT NULL,
  agent_id    TEXT,                                          -- NULL for install/exec
  grant_type  TEXT    NOT NULL CHECK (grant_type IN
              ('install','exec','filesystem','network')),
  scope       TEXT    NOT NULL,                              -- JSON-opaque
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
  "scope": "/home/user/code",  // JSON-opaque, daemon interprets
  "ttl_kind": "always"         // one_shot | always
}
```

## DOM ↔ DB enum 双向锁定 (content-lock §1.①)

| Button label | data-action          | data-hb3-button | DB ttl_kind |
|--------------|----------------------|-----------------|-------------|
| 拒绝         | `deny`               | `danger`        | (none, no row written) |
| 仅这一次     | `grant_one_shot`     | `primary`       | `one_shot`  |
| 始终允许     | `grant_always`       | `primary`       | `always`    |

DOM data-action map to enum literal byte-identical: `grant_one_shot` ↔
`one_shot`, `grant_always` ↔ `always`. 改前端 = 改 schema CHECK = 改
content-lock §1.①+§1.② (三处单测锁定).

## Audit log keys

| Key                       | Trigger                              |
|---------------------------|--------------------------------------|
| `host_grants.granted`     | POST success                         |
| `host_grants.revoked`     | DELETE success                       |

Each log includes `actor / action / target / when / scope` keys
byte-identical with HB-1/HB-2/BPP-4 audit schema.

## Tests

- `internal/migrations/hb_3_1_host_grants_test.go` — 7 unit tests
  (table shape + 4-enum CHECK + 2-enum CHECK + no platform-domain columns +
  indexes + idempotent + version=27).
- `internal/api/host_grants_test.go` — 8 unit tests (POST success
  filesystem + one_shot expires_at + grant_type/ttl_kind reject +
  GET list + DELETE revoke + cross-user 403 + AST scan
  user_permissions 0 hit + AST scan grant-queue 0 hit + AST scan
  audit 5-field).
- `packages/client/src/__tests__/HostGrantsPanel.test.tsx` — 5
  vitest cases (data-action + hb3-button + button text byte-identical
  + actionLabel 4-enum + 同义词 0 occurrence + onDecide 三值).

Regression rows: `REG-HB3-001..011` in
`docs/qa/regression-registry.md`.

## HB-2 daemon read-path contract (Go, packages/borgee-helper/, #617 merged)

HB-2 host-bridge daemon (Go module `packages/borgee-helper/`, #617 已 merged)
走单一 SELECT:

```sql
SELECT scope, expires_at FROM host_grants
WHERE user_id = ? AND agent_id = ? AND grant_type = ?
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > strftime('%s','now') * 1000)
LIMIT 1;
```

Daemon 不写, 不缓存. CI lint reverse-grep
`host_grants.*INSERT|host_grants.*UPDATE` in `packages/borgee-helper/`
must be 0 hit. 真包入口 → [`../../borgee-helper.md`](../../borgee-helper.md).

## Adding a new grant_type

1. Update content-lock §1.① actionLabel map (server prose).
2. Update CHECK constraint in `hb_3_1_host_grants.go` migration —
   actually NO, migration is immutable; ship a new migration that
   ALTERs (forward-only).
3. Update `hostGrantTypeWhitelist` in `host_grants.go`.
4. Update `actionLabel` map in `HostGrantsPanel.tsx`.
5. Update content-lock §1 + spec §1 + acceptance §1.2 byte-identical.
6. CI lint catches divergence via reflect (existing PRAGMA test) +
   reverse-grep (`TestHB31_GrantTypeEnumReject` enumerates 4-list).
