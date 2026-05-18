# PM Stance: requireMention Policy Model

## Scope Position

This task gives channel context a bounded way to reduce or require agent attention while preserving agent-owner authority as the ceiling. The policy is server-owned and privacy-preserving; UI affordances can follow in a later task.

## Stances

1. Agent owner authority is the ceiling.
   - Constraint: a channel policy cannot make an external agent listen more broadly than the agent owner allows.
   - Reviewer signal: setting `off` is rejected when the agent's global `require_mention` is true.

2. Channel owners may reduce attention.
   - Constraint: `on` can force mention-required behavior because it narrows delivery.
   - Reviewer signal: channel-level `on` works even when the agent owner globally allows broader listening.

3. `inherit` is compatibility-preserving.
   - Constraint: existing memberships and missing values resolve through the agent's existing global `require_mention` setting.
   - Reviewer signal: legacy rows do not change behavior after migration.

4. Server policy owns enforcement.
   - Constraint: clients may request a policy update, but message routing and recipient eligibility are resolved server-side.
   - Reviewer signal: tests prove storage/API/message behavior without relying on client hints.

5. Mention routing remains explicit.
   - Constraint: explicit `@agent` mention behavior and cross-channel mention rejection remain unchanged.
   - Reviewer signal: no history backfill, no client recipient IDs, and no `@Everyone` fanout enter this task.

6. Privacy scope stays internal.
   - Constraint: this task preserves existing privacy/security controls without adding dashboards, compliance copy, legal promises, or audit product surfaces.

## Out-Of-Scope Locks

- No `@Everyone` fanout.
- No client control UI.
- No notification, collapse, sort, or history backfill rewrite.
- No owner-transfer, hard-delete, archive, or channel management surface expansion.
- No user-facing privacy/compliance product surface.
