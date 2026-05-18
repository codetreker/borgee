# PM Stance: Client Mention Controls

## Scope Position

This task turns the server authority from Tasks 1 and 2 into visible client controls without moving authority into the browser.

## Stances

1. Server authority is explicit.
   - Constraint: client copy must say `@Everyone` recipients are server-computed and cannot be widened by the client.
   - Reviewer signal: the settings panel exposes the server-computed broadcast boundary and does not render a recipient picker.

2. Agent attention policy is controllable only through the user rail.
   - Constraint: the UI calls the existing require-mention policy endpoint and relies on server rejection for owner-ceiling failures.
   - Reviewer signal: users with `channel.manage_members` can choose inherit/on/off; users without it see disabled controls.

3. Current state is truthful.
   - Constraint: member listing includes `effective_require_mention` so the UI can distinguish "needs @" from "ordinary messages also deliver".
   - Reviewer signal: the panel renders the effective state returned by the server.

4. No client recipient authority.
   - Constraint: message sends do not include client-supplied mention recipient ids in HTTP or websocket payloads.
   - Reviewer signal: tests assert `sendMessage` omits `mentions`, and `MessageInput` sends content only.

5. Channel management stays narrow.
   - Constraint: no leave, delete, archive, owner-transfer, allowed-action, notification, group, sort, or broad settings redesign enters this task.
   - Reviewer signal: touched UI files stay under settings channel mention controls and message send payload cleanup.
