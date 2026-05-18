# Design: Borgee Plugin Channel Binding Job

## Dependency Decision

Task10 can proceed on current `origin/main`. Task9 is merged, and the existing store channel access helpers are sufficient for binding authorization. M2 Task6 is not required because it owns channel management action enforcement, while Task10 owns a Helper job enqueue boundary that reuses current channel membership/access checks and adds no channel mutation controls.

## Server Enqueue

Enable `borgee_plugin.configure_connection` in the Helper job taxonomy as an `openclaw_config` manifest-required job. Decode a strict payload with only `agent_id` and `channel_id`. Reject unknown or authority-bearing payload fields through the same store/API preflight used by the other Helper jobs.

The store validates:

- caller is a human/member owner in the requested org;
- enrollment belongs to that owner/org, is claimed, fresh, and delegated for `openclaw_config`;
- target user is an agent owned by the caller in the same org;
- target channel exists in the org, is a normal channel, and both owner and target agent can access it.

The effective payload contains only a deterministic server-owned `connection_id`, `agent_id`, and `channel_id`. The manifest binding contains the server-owned manifest digest and approved `borgee_plugin_config` path id. User enqueue responses continue to hide payload, manifest digest, manifest binding, owner/org internals, credentials, and logs.

## Helper Policy

Update `internal/jobpolicy` so `borgee_plugin.configure_connection` validates the server-owned payload schema: `connection_id` must use the server prefix, and `agent_id`/`channel_id` must be present. Existing policy flow already validates payload hash, enrollment state, manifest authority, and path binding for plugin connection jobs.

## Tests

Add RED-first coverage for:

- store enqueue rejects missing agent channel access and allows only after the target agent is a channel member;
- store enqueue derives connection id and manifest/path binding, converges idempotency, and rejects client-supplied connection/base URL/API-key authority;
- API enqueue/lease exposes only safe metadata and safe effective payload;
- Helper policy allows the server-bound plugin payload and rejects extra authority fields.

## Docs

Sync Task10 docs and `docs/current` to say plugin channel binding typed jobs are current behavior while local plugin config execution, service lifecycle, raw logs, sudo, Remote Agent rail reuse, and Configure OpenClaw terminal closure remain future work.
