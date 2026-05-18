# Dev Design: OpenClaw Install And Agent Config Jobs

## 1. Data Flow

1. A human/member user posts a Helper job envelope to `POST /api/v1/helper/enrollments/{enrollmentId}/jobs`.
2. The server derives owner/org/enrollment from authenticated state and route path, validates fresh claimed Helper state, category delegation, closed job type, and typed payload.
3. For `openclaw.configure_agent`, the server validates target agent ownership/org and optional channel access, reads the server-owned `agent_configs` row, and stores an effective payload containing `agent_id`, optional `channel_id`, `config_schema_version`, and `config_hash`.
4. For `openclaw.install_from_manifest`, the server accepts only `{ "runtime": "openclaw" }` intent and stores an effective payload containing a server-owned `install_plan_id`.
5. The server stores a manifest digest plus manifest binding JSON selected by job type: config path for configure-agent; install/config paths, artifact ID, and artifact origin for install.
6. Helper poll/lease returns the safe effective payload and manifest binding to the Helper credential rail. It does not expose owner/org internals, raw credentials, raw config, arbitrary paths/domains, service IDs, or logs.
7. Helper local policy revalidates signed manifest/artifact/path/domain/sandbox authority before any future action can run.

## 2. Data Model

No migration is needed. Existing `helper_jobs.manifest_digest` and `helper_jobs.manifest_binding_json` are now populated for the enabled OpenClaw job types.

Server-owned binding shape:

```json
{
  "manifest_digest": "sha256:...",
  "artifact_ids": ["openclaw-plugin"],
  "path_ids": ["openclaw_install", "openclaw_agent_config"],
  "domains": ["https://cdn.borgee.io"]
}
```

`service_ids` remains absent in Task9 so this task does not grant service lifecycle authority.

## 3. API Contract

Accepted enqueue payloads:

```json
{"job_type":"openclaw.configure_agent","schema_version":1,"payload":{"agent_id":"agent-id"}}
```

```json
{"job_type":"openclaw.install_from_manifest","schema_version":1,"payload":{"runtime":"openclaw"}}
```

Rejected payload authority includes `manifest_id`, `manifest_digest`, `manifest_binding`, `artifact_id`, `artifact_ids`, `path`, `path_ids`, `domain`, `url`, `service_id`, `service_unit`, `config_hash`, `install_plan_id`, `credential`, `token`, `shell`, `argv`, `command`, `script`, `ttl`, `deadline`, and expiry fields.

## 4. Security Boundary

- Enqueue authority stays on the user rail only; Helper credentials cannot enqueue.
- Helper credential rail can poll/ack/result only for the current Helper credential/device and matching lease token.
- Remote Agent tokens, host grants, plugin API keys, admin sessions, and user permissions do not authorize Helper jobs.
- Local policy remains the second authority check. It requires signed manifest and approved config path binding for configure-agent and signed manifest/artifact/path/domain binding for install.
- Task9 records intent and policy material only; it does not run local OpenClaw actions.

## 5. Test Plan

RED first:

- Store/API tests proving `openclaw.install_from_manifest` is enabled with server-owned effective payload and manifest binding, and client manifest/artifact/path/domain authority is rejected.
- Store tests proving `openclaw.configure_agent` now stores manifest/path binding.
- Helper policy tests proving configure-agent requires signed manifest and approved config path binding.

GREEN verification:

- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-server go test -tags sqlite_fts5 -count=1 ./internal/store ./internal/api ./internal/datalayer` from `packages/server-go`.
- `GOTMPDIR=/workspace/borgee/.worktrees/.gotmp-task9-helper go test -count=1 ./internal/jobpolicy ./internal/outbound` from `packages/borgee-helper`.
- Broader server/helper package tests as feasible before PR.
- `git diff --check` from repo root.
