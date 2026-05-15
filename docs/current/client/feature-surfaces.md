# Feature Surfaces

Feature surfaces are the user SPA's task-oriented areas. They are arranged by shell view and channel tab, but they share the same state, REST, and realtime rails.

## Surface Architecture

```mermaid
flowchart TB
  Shell[User shell]
  Rail[Channel and DM rail]
  Channel[Channel view]
  Sidepanes[Global sidepanes]
  Chat[Chat and DM]
  Canvas[Artifact canvas]
  ChannelWorkspace[Channel workspace tab]
  AllWorkspaces[All workspaces sidepane]
  ChannelRemote[Channel remote tab]
  RemoteNodes[Remote nodes sidepane]
  HelperStatus[Helper status sidepane]
  Agents[Agent ownership]
  Settings[Settings and admin-awareness]

  Shell --> Rail
  Shell --> Channel
  Shell --> Sidepanes
  Channel --> Chat
  Channel --> Canvas
  Channel --> ChannelWorkspace
  Channel --> ChannelRemote
  Sidepanes --> Agents
  Sidepanes --> AllWorkspaces
  Sidepanes --> RemoteNodes
  Sidepanes --> HelperStatus
  Sidepanes --> Settings
```

| Surface layer | Design role | Data boundary |
| --- | --- | --- |
| Rail | Navigate channels, DMs, and global sidepanes. | Reads shared channel/DM/current-user state and emits shell navigation. |
| Channel view | Host a selected channel or DM. | Owns tab selection and delegates content to chat/canvas/workspace/remote. |
| Chat/DM | Conversation, optimistic send, mentions, reactions, typing. | Uses shared message state and REST/WS send contracts. |
| Canvas/artifact | Channel-scoped durable artifact work. | Pulls artifact heads, versions, anchors, iterations, and comments from REST. |
| Channel workspace tab | File work in the active channel context. | Pulls channel-scoped file trees and file bodies from REST; editor drafts stay local. |
| All workspaces sidepane | Cross-channel workspace index and preview. | Pulls the all-workspaces projection from REST; grouping/filtering stays local. |
| Channel remote tab | Browse remote bindings attached to the active channel. | Pulls channel binding metadata and read-only remote tree/file data from REST. |
| Remote nodes sidepane | Manage user-owned remote nodes and channel bindings. | Pulls node, status, token, and binding data from the user remote API. |
| Helper status sidepane | Inspect user-owned Helper enrollment status. | Pulls redacted Helper enrollment status from the user Helper enrollment API. |
| Agent/invitation | Owner-side agent management and join approval. | Uses user agent APIs and signal-then-pull invitation updates. |
| Settings | User privacy, admin-impact history, impersonation grant. | Uses user-owned admin-awareness endpoints only. |

## Responsibilities

Feature surfaces own user workflows and local UI state. They coordinate with shared app state only when another surface needs the same information.

They do not own backend ACLs, persistence, admin visibility policy, or realtime frame schemas. They consume shared rails and server-enforced contracts.

The [ui/](ui/) directory provides Interaction And Layout Reference sketches for these surfaces. Treat those sketches as surface-orientation aids; they do not define product behavior, verification status, or design-system rules.

## Channel, Chat, And DM

The channel rail separates public/private channels from DMs, but both converge on the selected channel model. A DM is a channel-like conversation without the non-DM tab strip; normal joined channels can switch between chat, canvas, workspace, and remote browsing.

Chat is the only surface that writes messages through the realtime send path. It keeps optimistic pending state globally because message retry, ack/nack, reconnect, and render order cross component boundaries.

Mentions, slash commands, emoji, typing, reactions, edit/delete, and upload are chat capabilities layered around the same message stream. Public channel preview is read-only until join succeeds.

Sketch references: [ui/message.md](ui/message.md), [ui/dm.md](ui/dm.md), [ui/slash-commands.md](ui/slash-commands.md), and [ui/preview.md](ui/preview.md) show interaction shapes after this architecture boundary is understood.

## Artifact Canvas

The artifact surface treats the channel canvas as a durable collaborative document area. It works with a current artifact head, version history, rollback, diff, anchors, comments, and iteration state.

Artifact and comment bodies are not accepted from realtime as authority. Realtime signals only wake the panel or comment surface; content is pulled through REST so version, ACL, and privacy rules stay centralized.

Sketch reference: [ui/canvas-modal.md](ui/canvas-modal.md) shows the in-app decision flow around canvas actions; REST authority remains defined here.

## Workspace Surfaces

Workspace has two projections over the same file domain, but they have different state ownership:

| Surface | State owner | Data owner |
| --- | --- | --- |
| Channel workspace tab | Channel-scoped file navigation, selected file, edit draft, drag/drop state, and local loading/error state. | Workspace REST endpoints scoped by channel. |
| All workspaces sidepane | Cross-channel grouping, selected channel filter, selected preview file, and local context-menu state. | All-workspaces REST projection plus channel workspace file endpoints for mutations. |

File upload, rename, move, delete, directory creation, Markdown edit, and preview are REST-backed. File viewer selection is local presentation logic; persisted file content remains server-owned.

Sketch reference: [ui/workspace.md](ui/workspace.md) shows the file tree and viewer layout for the workspace surface.

## Remote Surfaces

Remote has two separate user surfaces: browsing a channel binding and managing nodes/bindings.

| Surface | State owner | Data owner |
| --- | --- | --- |
| Channel remote tab | Selected channel binding, current remote path, viewed file, and local tree/error state. | Channel remote binding endpoint plus read-only remote `ls` and file-read endpoints. |
| Remote nodes sidepane | Node list, selected node, status snapshot, token visibility, create/binding forms, and local dirty guards. | User remote node, node status, connection token, and node binding endpoints. |

The remote browsing surface reads directory listings and file content through user APIs. It does not provide an admin bypass and does not write remote files in the current UI architecture.

Sketch reference: [../remote-agent/ui/README.md](../remote-agent/ui/README.md) preserves a combined Remote Explorer reference sketch. Current user SPA architecture splits that concept into the channel remote tab and the remote nodes sidepane.

## Helper Status Surface

Helper status is a user-owned global sidepane for Host Bridge enrollment visibility. It lists Helper enrollments from the user Helper enrollment REST rail and renders connected, offline, revoked, uninstalled, and pending enrollment states with last-seen and allowed-category details.

| Surface | State owner | Data owner |
| --- | --- | --- |
| Helper status sidepane | Enrollment list, selected enrollment, refresh/error state, status display, last-seen display, and allowed-category display. | User Helper enrollment list/detail endpoints. |

The surface is read-only status. It does not call Helper credential claim, heartbeat/status, or uninstall endpoints from the browser, and it does not display raw enrollment secrets, Helper credentials, Remote Agent connection tokens, org internals, private file content, or local environment details.

Helper connected means the enrolled Helper/device was recently seen by the server. It is not OpenClaw connected status, Configure OpenClaw success, job progress, job logs, or service lifecycle completion. Remote nodes remain the separate Remote Agent filesystem proxy surface.

## Agent And Invitation Surfaces

Agent management is an owner workflow: create/delete agents, control permissions, reveal or rotate API keys, add agents to channels, observe runtime state, and edit agent config. Sensitive key material is handled locally and not stored in shared app state.

Invitation handling is a separate owner inbox. Realtime invitation frames do not replace REST state; they wake the inbox and badge to refresh authoritative invitation status.

Sketch references: [ui/agent-manager.md](ui/agent-manager.md), [ui/agent-config.md](ui/agent-config.md), and [ui/agent-collab.md](ui/agent-collab.md) show owner-side agent management and collaboration surfaces.

## Settings And Admin-Awareness

The settings surface is the user-visible privacy boundary. It shows what admin impact the user is allowed to inspect and lets the user create or revoke a temporary impersonation grant.

This is not the admin SPA. It is a user rail surface backed by user endpoints, so it can be visible in the normal shell without granting admin session capabilities.

Sketch reference: [ui/settings.md](ui/settings.md) shows the user-facing admin-awareness layout. Admin architecture remains owned by [../admin/](../admin/).

## Interfaces To Other Modules

| Interface | Contract |
| --- | --- |
| App shell | Selects which sidepane or channel view is visible; surfaces do not own app-level navigation. |
| App state | Supplies shared rail, identity, permission, connection, message, and pending-message state. |
| REST rail | Supplies durable state for all feature domains. |
| Realtime rail | Supplies direct chat/presence updates and wake-up signals for pull refresh. |
| Admin rail | Isolated from user feature surfaces except for user-owned admin-awareness endpoints. |

## Implementation Anchors

| Surface | Anchors |
| --- | --- |
| Surface host | `packages/client/src/components/ChannelView.tsx`, `packages/client/src/components/Sidebar.tsx` |
| Feature components | `packages/client/src/components/`, `packages/client/src/components/Settings/` |
| Feature hooks and commands | `packages/client/src/hooks/`, `packages/client/src/commands/` |
| User API surface | `packages/client/src/lib/api.ts` |
