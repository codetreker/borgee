# UI Wireframes

These sketches are architecture-level layout maps for the user SPA. They were recovered from the older `docs/current/client/ui/*` sketches and normalized to the current architecture. They are not a pixel spec, copy spec, or component inventory.

Use them to understand which UI surfaces coexist in the shell, which surfaces are channel-scoped, and which surfaces are global sidepanes.

## Recovered Sources

| Old sketch | Restored value |
| --- | --- |
| `client/ui/main-desktop.md` | Desktop shell, rail, channel host, tab strip, composer. |
| `client/ui/main-mobile.md` | Mobile shell and rail overlay relationship. |
| `client/ui/dm.md` | DM host as a channel-like chat surface without non-DM tabs. |
| `client/ui/workspace.md` | Channel workspace tab: tree plus file viewer. |
| `client/ui/channel-sort-groups.md` | Rail grouping, owner-only reorder affordance, DM separation. |
| `client/ui/message.md` | Message row, system action row, reaction placement. |
| `client/ui/slash-commands.md` | Composer-owned command panel and command result placement in chat. |
| `client/ui/agent-manager.md` and `client/ui/settings.md` | Global sidepane surfaces under the single `mainView` state. |
| `remote-agent/ui/README.md` | Remote node and binding management shape, now represented as user remote sidepanes and channel remote browsing. |

Older sketches that specified exact visual copy, modal flows, or implementation-only acceptance details were not restored here. Those details belong in tests or implementation notes, not in the current architecture map.

## Desktop Shell

```text
+----------------------+------------------------------------------------------+
| User rail            | Channel host                                         |
|                      | [ Chat ] [ Canvas ] [ Workspace ] [ Remote ]        |
| Channels             +------------------------------------------------------+
|   # general          | # general                                controls   |
|   # dev              +------------------------------------------------------+
|   # design           |                                                      |
|                      | Message stream, artifact canvas, workspace, or      |
| Direct messages      | remote binding browser, depending on selected tab.  |
|   Bob                |                                                      |
|   Carol              |                                                      |
|                      |                                                      |
| Global sidepanes     +------------------------------------------------------+
| [settings agents     | Composer or selected feature action area             |
|  invites files       |                                                      |
|  remote]             |                                                      |
+----------------------+------------------------------------------------------+
```

Architectural reading:

- The left rail owns channel, DM, and global sidepane selection.
- The channel host owns selected-channel tabs. Chat is the default channel surface; Canvas, Workspace, and Remote are channel-scoped siblings.
- Global sidepanes are outside the selected channel tab strip and use the shell-level `mainView` state.
- The composer belongs to chat/DM, not to non-chat feature tabs.

## Mobile Shell

```text
+----------------------------------+
| [menu]  # general       controls |
+----------------------------------+
| [Chat] [Canvas] [Files] [Remote] |
+----------------------------------+
|                                  |
| Selected channel surface         |
|                                  |
| Chat shows message stream.       |
| Other tabs replace this region   |
| with their channel-scoped        |
| surface.                         |
|                                  |
+----------------------------------+
| Composer or feature action area  |
+----------------------------------+

Rail overlay:

+----------------------+------------+
| User rail            | dimmed app |
| Channels             | backdrop   |
| Direct messages      |            |
| Global sidepanes     |            |
+----------------------+------------+
```

Architectural reading:

- Mobile keeps the same shell layers as desktop; it changes rail presentation, not ownership.
- The rail overlay selects channels, DMs, or global sidepanes, then returns to the active surface.
- Feature tabs remain channel-scoped and should not become bottom-navigation global routes.

## Rail Grouping

```text
+----------------------+
| Borgee           add |
|                      |
|   # general          |
|   # random           |
|                      |
| v Engineering        |
|   # dev              |
|   # ci               |
|   # infra            |
|                      |
| > Archived           |
|                      |
| Direct messages      |
|   Bob                |
|   Carol              |
|                      |
| settings agents      |
| invites files remote |
+----------------------+
```

Architectural reading:

- Channel grouping belongs to the channel rail and never applies to DMs.
- Owner-only reorder and group-management affordances are rail capabilities; non-owners see the same grouping as read-only navigation.
- Collapsed group state is local presentation state; channel order and group assignment are server-owned channel metadata.

## Channel And DM Hosts

```text
Channel host:

+----------------------------------------------------------------+
| # general                                           controls   |
+----------------------------------------------------------------+
| [Chat] [Canvas] [Workspace] [Remote]                           |
+----------------------------------------------------------------+
| Message stream or selected channel feature surface              |
+----------------------------------------------------------------+
| Composer when Chat is selected                                  |
+----------------------------------------------------------------+

DM host:

+----------------------------------------------------------------+
| Direct message with Alice                            controls  |
+----------------------------------------------------------------+
| Message stream                                                   |
+----------------------------------------------------------------+
| Composer                                                         |
+----------------------------------------------------------------+
```

Architectural reading:

- DMs converge on the same selected-channel model but do not expose non-DM channel tabs.
- Chat and DM share message rendering, optimistic send state, reactions, edit/delete, and retry behavior.
- Canvas, Workspace, and Remote remain absent from DM surfaces unless the server model explicitly adds such a capability.

## Message Row And System Actions

```text
+----+  Author                         time
| AV |  Message text can wrap across lines.
+----+
       reaction chips                      add reaction

+----+  System                         time
| SY |  Agent needs a user-owned decision.
+----+  +----------+ +----------+ +----------+
       | grant    | | reject   | | later    |
       +----------+ +----------+ +----------+
```

Architectural reading:

- Normal, agent, and system messages share the message stream surface.
- Action buttons inside system messages are user decisions over server-owned payloads; they are not separate global sidepanes.
- Reactions are message-scoped UI state that reconciles with server aggregates.

## Composer Command Panel

```text
+----------------------+------------------------------------------------------+
| User rail            | # general                                           |
|                      +------------------------------------------------------+
|                      | Message stream                                      |
|                      |                                                      |
|                      | +--------------------------------------------------+ |
|                      | | Slash commands                                   | |
|                      | | System                                           | |
|                      | |   /status       channel metadata                 | |
|                      | | Agents                                           | |
|                      | |   /deploy       selected agent action            | |
|                      | +--------------------------------------------------+ |
|                      +------------------------------------------------------+
|                      | /de                                      Send       |
+----------------------+------------------------------------------------------+
```

Architectural reading:

- The command panel is composer-owned chat UI, not a global route or sidepane.
- Command discovery can group by system and agent sources while final execution still writes into the chat/message path.
- Command results render inside the message stream as message-adjacent state, so reconnect and history can reconcile through the normal message and signal rails.

## Workspace And Remote Tabs

```text
Workspace tab:

+----------------------+------------------------------------------------------+
| File tree            | File viewer or editor                                |
| docs/                |                                                      |
|   README.md          | Markdown preview, source view, text, or image body   |
| src/                 | loaded through workspace REST endpoints.            |
|   app.ts             |                                                      |
+----------------------+------------------------------------------------------+

Remote tab:

+----------------------+------------------------------------------------------+
| Channel bindings     | Remote file viewer                                   |
| dev-server:/repo     |                                                      |
| staging:/logs        | Read-only directory listing and file reads mediated  |
|                      | by the user's remote node connection.               |
+----------------------+------------------------------------------------------+
```

Architectural reading:

- Workspace is channel-scoped file work backed by workspace REST endpoints.
- Remote tab is channel-scoped browsing of bindings to user-owned remote nodes.
- Remote browsing is read-only in the current UI architecture and does not grant host helper authority.

## Global Sidepanes

```text
+----------------------+------------------------------------------------------+
| User rail            | Active global sidepane                               |
|                      |                                                      |
| Channels             | +--------------------------------------------------+ |
| Direct messages      | | Agents, invitations, all workspaces, remote      | |
|                      | | nodes, or settings. Only one is active at a     | |
| Global sidepanes     | | time through shell `mainView`.                  | |
| > agents             | +--------------------------------------------------+ |
|   invites            |                                                      |
|   files              | Back returns to the selected channel surface.        |
|   remote             | Dirty forms register unsaved-change guards.          |
|   settings           |                                                      |
+----------------------+------------------------------------------------------+
```

Architectural reading:

- Sidepanes are global user workflows, not channel tabs.
- The shell prevents sidepane stacking with one active `mainView` value.
- Feature drafts, dirty guards, reveal/copy state, and local filters stay inside the active sidepane.

## Remote Nodes Sidepane

```text
+----------------------------------------------------------------+
| Remote nodes                                                    |
+----------------------------------------------------------------+
| Node            Status       Last seen       Actions            |
| dev-server      online       just now        remove             |
| staging         offline      yesterday       remove             |
+----------------------------------------------------------------+
| Bindings for selected node                                      |
| Alias           Remote path                 Channels            |
| project-src     /home/user/project/src      #dev               |
+----------------------------------------------------------------+
| Connection token and setup command, hidden until the user        |
| explicitly reveals or copies it.                                |
+----------------------------------------------------------------+
```

Architectural reading:

- Node lifecycle and binding management are user-owned global workflows.
- Channel remote tabs consume bindings; they do not own node tokens or setup commands.
- Token reveal/copy belongs to local UI state and should not leak into shared app state.

## Related Documents

| Document | Relationship |
| --- | --- |
| `ui-map.md` | Surface hierarchy and ownership rules. |
| `feature-surfaces.md` | Feature-level responsibilities and REST/realtime boundaries. |
| `app-shell-state.md` | Shell lifecycle, `mainView`, selection, and shared app state. |
| `realtime-sync.md` | Which surfaces update directly from WebSocket and which use signal-then-pull. |
