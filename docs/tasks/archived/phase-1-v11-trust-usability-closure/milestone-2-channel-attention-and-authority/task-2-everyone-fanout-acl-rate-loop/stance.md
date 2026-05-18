# Stance: Everyone Fanout ACL Rate Loop

## Decisions

- Server computes all `@Everyone` recipients from channel membership. Client-supplied recipient ids are rejected.
- `@Everyone` is exact-case and reserved. It is not a display-name mention.
- Human/member senders may broadcast. Agent senders may not broadcast.
- The first implementation uses process-local rate limiting, matching the existing mention fallback throttle pattern.
- Computed broadcast recipients are persisted to `message_mentions` for reviewability.

## Rejected Options

- Accepting a request-body recipient list. Rejected because the task explicitly forbids hidden or unauthorized fanout.
- Letting agents broadcast. Rejected because it creates recursive agent-to-channel loop risk.
- Adding client UI controls in this task. Rejected because Task3 owns client mention controls and Task4 owns channel management UI.
- Adding a new notification subsystem. Rejected as outside the milestone slice.

## Follow-On Boundaries

- Task3 can wire client mention controls against the now-testable server behavior.
- Task4 remains the channel management surface and should not inherit mention fanout implementation details.
- Task7 and later private/sidebar tasks remain separate visual-state work.
