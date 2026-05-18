# Spec Brief: Borgee Plugin Channel Binding Job

Task contract: enable the closed Helper job type `borgee_plugin.configure_connection` so Configure OpenClaw can enqueue Borgee plugin connection/channel binding intent after Task9 install/config jobs. The job remains a bounded typed job: the browser may identify the target agent and channel, but the server derives the effective connection identifier, manifest digest, approved config path binding, owner/org/enrollment fields, category, TTL, and idempotency scope.

Dependencies: Task9 PR #956 is merged at `5575b53f657276c57ba319b144281286865db630`. Current code already has channel access checks sufficient for this task: binding rechecks the channel exists in the owner org and that both owner and target agent can access the channel. M2 Task6 channel authority checks are not a blocker because this task does not expose channel management actions; it only authorizes a Helper job against existing channel membership/access state.

Implement `borgee_plugin.configure_connection` under the existing `openclaw_config` delegation category. The input payload is closed to `agent_id` and `channel_id`. The target agent must be an agent owned by the human/member owner in the same org. The channel must be an ordinary channel in that org, not a DM, and both owner and agent must pass existing channel access checks. Rejected binding attempts create no executable `helper_jobs` row.

The stored effective payload includes `connection_id`, `agent_id`, and `channel_id`. `connection_id` is server-owned and deterministic for org, agent, and channel; clients cannot provide or override it. The stored manifest binding uses the existing runtime manifest digest mechanism and binds only the approved Borgee plugin config path id. User enqueue responses remain metadata-only. Helper poll responses may expose the safe effective payload and manifest binding needed by local policy.

Helper local policy accepts the new job only when the server-owned payload schema, payload hash, enrollment/category state, signed manifest, and approved config path binding all validate. It does not execute the plugin write, install OpenClaw, start services, upload raw logs, or claim Configure OpenClaw terminal success.

Out of scope: channel management actions, owner transfer, leave/delete/archive authority, service lifecycle, OpenClaw local execution, sudo behavior, raw/bulk log upload, Remote Agent rail reuse, and Configure OpenClaw terminal UI closure.
