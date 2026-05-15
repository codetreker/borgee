# Stance: Borgee Plugin Channel Binding Job

1. Existing channel access is enough for this task.
   - Task10 does not add channel management actions. It only binds a Helper job to a channel after checking owner/org/channel/agent access. M2 Task6 remains the owner for management action authority and is not a prerequisite here.

2. Browser intent is not binding authority.
   - The browser can name `agent_id` and `channel_id`; it cannot supply connection ids, base URLs, API keys, credentials, paths, domains, manifests, TTLs, commands, service units, or config hashes.

3. Server-owned effective payloads are the executable boundary.
   - Stored jobs use server-derived owner/org/enrollment/category/device/TTL/idempotency metadata plus a deterministic server-owned Borgee plugin connection id.

4. Helper policy remains a second gate.
   - Enqueue approval is not enough for local action. The Helper-side evaluator must still validate schema, payload hash, enrollment state, signed manifest, and approved config path binding.

5. This is not execution closure.
   - The task enables typed job coverage and safe lease projection. It does not write local plugin config, execute OpenClaw, manage services, upload logs, cache sudo, or complete Configure OpenClaw terminal UI.
