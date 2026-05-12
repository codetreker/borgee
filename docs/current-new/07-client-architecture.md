# 07. Client Architecture

This document describes the current user-facing SPA from source. It intentionally does not copy the legacy `docs/current` client docs.

Source set checked for this page: `packages/client/vite.config.ts`, `packages/client/src/main.tsx`, `packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/lib/api.ts`, `packages/client/src/hooks/*`, and `packages/client/src/components/*`.

## Scope And Entry

| Area | Current behavior | Code evidence |
| --- | --- | --- |
| Build tool | The user SPA is a Vite React app. The Vite build has two HTML inputs, `index.html` for the user app and `admin.html` for the admin app. | `packages/client/vite.config.ts`, `packages/client/package.json` |
| Dev proxy | In dev, `/api`, `/admin-api`, `/health`, `/uploads`, and `/ws` are proxied to `VITE_E2E_API_TARGET` or `http://localhost:4900`; `/ws` uses the corresponding `ws:` target. | `packages/client/vite.config.ts` |
| User entry | `src/main.tsx` mounts `<App />` into `#root` under `React.StrictMode` and registers `/sw.js` after window load when service workers are available. | `packages/client/src/main.tsx` |
| Root providers | `App` wraps the user app with `ThemeProvider`, `ToastProvider`, and `AppProvider`; the admin provider is not mounted in the user entry. | `packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/admin/main.tsx` |

```mermaid
flowchart LR
  index[index.html] --> main[src/main.tsx]
  main --> App[App.tsx]
  App --> Theme[ThemeProvider]
  App --> Toast[ToastProvider]
  App --> Ctx[AppProvider]
  Ctx --> UI[Sidebar + ChannelView + sidepane views]
  App --> WS[useWebSocket]
  UI --> API[lib/api.ts /api/v1]
  WS --> Hub[/ws]
```

## App Shell And View State

`AppInner` owns authentication bootstrapping, responsive sidebar state, and the top-level `mainView` state. Login state is checked with `fetchMe`; after authentication it loads current user, permissions, channels, and online users, then marks the app initialized. It also periodically refreshes online users every 30 seconds. Evidence: `packages/client/src/App.tsx`, `packages/client/src/lib/api.ts`.

The main area is a single-string view state, not a stack of booleans. Valid values are `channel`, `agents`, `invitations`, `workspaces`, `remote-nodes`, and `settings`. Sidebar buttons call `requestMainView`, which runs unsaved-change guards before switching. Evidence: `packages/client/src/App.tsx`, `packages/client/src/lib/mainView.ts`, `packages/client/src/hooks/useUnsavedChangesGuard.ts`.

The rendered top-level user views are:

| View | Trigger / route model | Main components | Code evidence |
| --- | --- | --- | --- |
| Auth | unauthenticated `AppInner` branch | `LoginPage`, `RegisterPage` | `packages/client/src/App.tsx`, `packages/client/src/components/LoginPage.tsx`, `packages/client/src/components/RegisterPage.tsx` |
| Channel | `mainView === 'channel'` and `state.currentChannelId` | `Sidebar`, `ChannelView`, `MessageList`, `MessageInput` | `packages/client/src/App.tsx`, `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/ChannelView.tsx` |
| Agents | sidebar agent button | `AgentManager` | `packages/client/src/App.tsx`, `packages/client/src/components/AgentManager.tsx` |
| Invitations | sidebar bell | `InvitationsInbox` | `packages/client/src/App.tsx`, `packages/client/src/components/InvitationsInbox.tsx` |
| All workspaces | sidebar workspace button | `WorkspaceManager` | `packages/client/src/App.tsx`, `packages/client/src/components/WorkspaceManager.tsx` |
| Remote nodes | sidebar remote button | `NodeManager` | `packages/client/src/App.tsx`, `packages/client/src/components/NodeManager.tsx` |
| Settings | sidebar settings button | `SettingsPage`, `PrivacyPromise`, `ImpersonateGrantSection`, `AdminActionsList` | `packages/client/src/App.tsx`, `packages/client/src/components/Settings/SettingsPage.tsx` |

There is also an `AppShell` and `ArtifactDrawer` implementation for a three-column artifact drawer/split/fullscreen state machine, but the current root `App.tsx` does not import or mount it. The active channel canvas path renders `ArtifactPanel` directly inside the `ChannelView` `canvas` tab. Evidence: `packages/client/src/components/AppShell.tsx`, `packages/client/src/components/ArtifactDrawer.tsx`, `packages/client/src/lib/use_artifact_panel.ts`, `packages/client/src/App.tsx`, `packages/client/src/components/ChannelView.tsx`.

## AppContext State

`AppProvider` is the user SPA state container. It uses `useReducer` with an exported reducer and initial state for tests; production consumers go through `useAppContext`. Evidence: `packages/client/src/context/AppContext.tsx`.

Major state fields:

| State field | Purpose | Code evidence |
| --- | --- | --- |
| `channels`, `groups`, `dmChannels` | Channel list, channel groups, and DM rail data. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/types.ts` |
| `currentChannelId` | The selected channel or DM. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/components/Sidebar.tsx` |
| `messages`, `hasMore`, `loadingMessages` | Paginated per-channel message cache and load state. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/components/MessageList.tsx` |
| `pendingMessages` | Optimistic outbound messages waiting for WS ack/nack or timeout. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/components/MessageInput.tsx`, `packages/client/src/hooks/useWebSocket.ts` |
| `currentUser`, `permissions` | Authenticated user and capability details. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/usePermissions.ts` |
| `onlineUserIds`, `connectionState`, `typingUsers` | Presence, WS connection banner state, and typing indicators. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/components/ConnectionStatus.tsx` |
| `channelMembersVersion` | Version counter that forces member-dependent UI to refetch after membership changes. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/components/Sidebar.tsx` |
| `initialized` | Gates post-auth app rendering after current user, permissions, channels, and online users load. | `packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx` |

Context actions are intentionally thin REST orchestration: load channels/groups, load messages and older pages, load current user, load permissions, load online users, select channel, send message, create channel, load DMs, and open DMs. Evidence: `packages/client/src/context/AppContext.tsx`, `packages/client/src/lib/api.ts`.

## REST API Client

The user REST client lives in `packages/client/src/lib/api.ts`. It uses same-origin relative URLs, `credentials: 'include'`, JSON `Content-Type` only when needed, and lets the browser set multipart boundaries for `FormData`. In dev it can add `X-Dev-User-Id` when `setDevUserId` has been called. Evidence: `packages/client/src/lib/api.ts`.

Important API groups:

| Group | Representative functions | Paths / behavior | Code evidence |
| --- | --- | --- | --- |
| Auth/user | `login`, `logout`, `register`, `fetchMe`, `updateProfile` | `/api/v1/auth/*`, `/api/v1/users/me` | `packages/client/src/lib/api.ts` |
| Channels/groups | `fetchChannels`, `createChannel`, `updateChannel`, `joinChannel`, `leaveChannel`, `reorderChannel`, channel group CRUD | `/api/v1/channels*`, `/api/v1/channel-groups*` | `packages/client/src/lib/api.ts`, `packages/client/src/components/Sidebar.tsx` |
| Messages | `fetchMessages`, `sendMessage`, `editMessage`, `deleteMessage`, reactions, read markers | `/api/v1/channels/:id/messages`, `/api/v1/messages/:id*` | `packages/client/src/lib/api.ts`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/components/MessageInput.tsx` |
| Realtime backfill | `fetchEventsBackfill` | `/api/v1/events?since=`; used only after dropped WS reconnect with a stored cursor. | `packages/client/src/lib/api.ts`, `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/lastSeenCursor.ts` |
| Agents/runtime/config | agent CRUD, runtime start/stop, config GET/PATCH, permissions | `/api/v1/agents*` | `packages/client/src/lib/api.ts`, `packages/client/src/components/AgentManager.tsx`, `packages/client/src/components/AgentConfigPanel.tsx` |
| Uploads | `uploadImage` | `POST /api/v1/upload` with `FormData`; returns URL used as an image message. | `packages/client/src/lib/api.ts`, `packages/client/src/components/MessageInput.tsx` |
| Workspace | `listWorkspaceFiles`, `uploadWorkspaceFile`, `downloadWorkspaceFile`, `updateWorkspaceFile`, `deleteWorkspaceFile`, `mkdirWorkspace`, `moveWorkspaceFile`, `renameWorkspaceFile`, `fetchAllWorkspaces` | Channel-scoped workspace endpoints and an all-workspaces list. | `packages/client/src/lib/api.ts`, `packages/client/src/components/WorkspacePanel.tsx`, `packages/client/src/components/WorkspaceManager.tsx` |
| Remote nodes | node CRUD, bindings, directory listing, file reads, node status | `/api/v1/remote/*` and `/api/v1/channels/:id/remote-bindings` | `packages/client/src/lib/api.ts`, `packages/client/src/components/NodeManager.tsx`, `packages/client/src/components/RemotePanel.tsx`, `packages/client/src/components/RemoteTree.tsx` |
| Artifacts | create/get/versions/commit/rollback, iterations, anchors, comments, search | `/api/v1/artifacts*`, `/api/v1/channels/:id/artifacts` | `packages/client/src/lib/api.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/ArtifactComments.tsx`, `packages/client/src/components/IteratePanel.tsx` |
| User admin-awareness | own admin action history and own impersonation grant | `/api/v1/me/admin-actions`, `/api/v1/me/impersonation-grant` | `packages/client/src/lib/api.ts`, `packages/client/src/components/Settings/SettingsPage.tsx`, `packages/client/src/components/Settings/BannerImpersonate.tsx` |

## WebSocket Hook And Pull-After-Signal Pattern

`useWebSocket` connects to `ws(s)://<current-host>/ws`, optionally adding `?user_id=<dev-user>` from the dev user helper. It exposes `subscribe`, `unsubscribe`, `sendWsMessage`, `registerAckTimer`, and the current connection state. Evidence: `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/api.ts`.

Connection behavior:

| Behavior | Code evidence |
| --- | --- |
| Pings every 25 seconds while open. | `packages/client/src/hooks/useWebSocket.ts` |
| Reconnects with backoff delays from 1s to 30s unless the close code is an auth failure (`4001`, `4003`). | `packages/client/src/hooks/useWebSocket.ts` |
| Re-subscribes to previously subscribed channels after reconnect. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/App.tsx` |
| Persists numeric frame cursors before dispatch and uses `/api/v1/events?since=` on dropped reconnect when a stored cursor exists. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/lastSeenCursor.ts`, `packages/client/src/lib/api.ts` |
| Also reconciles missed messages per subscribed channel via `fetchMessages(..., { after: lastTs })`. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/api.ts` |
| Flattens both `{type, data: payload}` hub frames and direct `{type, cursor, ...}` frames before handling. | `packages/client/src/hooks/useWebSocket.ts` |

Message send flow:

```text
MessageInput
  -> ADD_PENDING_MESSAGE in AppContext
  -> sendWsMessage({ type: "send_message", client_message_id, ... })
  -> useWebSocket receives message_ack or message_nack
  -> ACK_PENDING_MESSAGE / FAIL_PENDING_MESSAGE
  -> timeout marks pending message failed after 10s if no ack
```

Evidence: `packages/client/src/components/MessageInput.tsx`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts`.

The WS layer mixes reducer-updating frames with signal-only frames. Reducer-updating frames include new messages, presence, channel/group changes, typing, reaction updates, message edits/deletes, and pending-message ack/nack. Signal-only frames are bridged through window `CustomEvent`s in `useWsHubFrames`; consumers then refetch authoritative REST state. Evidence: `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/types/ws-frames.ts`.

Pull-after-signal cases:

| WS frame | Client event/hook | Authoritative follow-up | Consumer evidence |
| --- | --- | --- | --- |
| `agent_invitation_pending`, `agent_invitation_decided` | `borgee:invitation-*`, `useInvitationFrames` | `listAgentInvitations` refreshes inbox/bell state. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/InvitationsInbox.tsx` |
| `artifact_updated` | `useArtifactUpdated` | `getArtifact` and `listArtifactVersions` reload body, committer, and versions. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/lib/api.ts` |
| `mention_pushed` | `useMentionPushed` | Matching channel reloads messages; `body_preview` is not rendered as message body. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/types/ws-frames.ts` |
| `anchor_comment_added` | `useAnchorCommentAdded` | Artifact anchors and anchor comments are refetched. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/AnchorThreadPanel.tsx`, `packages/client/src/lib/api.ts` |
| `iteration_state_changed` | `useIterationStateChanged` | Iteration list/body is refetched; frame does not carry `intent_text`. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/IteratePanel.tsx`, `packages/client/src/types/ws-frames.ts` |
| `artifact_comment_added` | `useArtifactCommentAdded` | Artifact comments are refetched from `/api/v1/artifacts/:id/comments`; `body_preview` is not used for rendered comment text. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/ArtifactComments.tsx`, `packages/client/src/types/ws-frames.ts` |

## Channel UI Boundaries

`Sidebar` renders non-DM channels through `ChannelList`, renders DMs separately, exposes create channel/group UI, shows agent invitations, and conditionally shows Agents, Remote Nodes, and Settings buttons only for non-agent users. Workspaces are available from the sidebar regardless of the non-agent checks around adjacent buttons. Evidence: `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/ChannelList.tsx`.

`ChannelView` distinguishes channel, DM, and public preview states. DMs do not render the tab switcher. Joined non-DM channels render tabs for `chat`, `canvas`, `workspace`, and `remote`; only `chat` and `workspace` are synchronized to `?tab=`. Evidence: `packages/client/src/components/ChannelView.tsx`.

Chat mode renders connection state, messages, and composer. Public preview mode uses `fetchChannelPreview` and a join button instead of `MessageInput`. Evidence: `packages/client/src/components/ChannelView.tsx`, `packages/client/src/components/ConnectionStatus.tsx`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/components/MessageInput.tsx`.

`MessageInput` is the boundary for rich text, mentions, slash commands, emoji, typing signals, and image upload. Text messages go over WS with optimistic state; image paste/drop/file-select first uploads to REST and then sends the returned URL as an image WS message. Evidence: `packages/client/src/components/MessageInput.tsx`, `packages/client/src/commands/registry.ts`, `packages/client/src/hooks/useSlashCommands.ts`, `packages/client/src/extensions/mention.ts`, `packages/client/src/lib/api.ts`.

## Uploads, Workspace, Artifact, And Remote Boundaries

### Uploads

Image upload is only a message-composer concern in the current user flow. `MessageInput` accepts dropped, pasted, or file-selected images, calls `uploadImage`, then sends a pending image message with the returned URL. The Vite dev proxy also forwards `/uploads` to the backend target, but upload creation goes through `/api/v1/upload`. Evidence: `packages/client/src/components/MessageInput.tsx`, `packages/client/src/lib/api.ts`, `packages/client/vite.config.ts`.

### Workspace

The workspace has two UI surfaces:

| Surface | Boundary | Code evidence |
| --- | --- | --- |
| Channel workspace tab | `WorkspacePanel` is channel-scoped. It lists a folder, supports drag/drop upload, mkdir, delete, rename, Markdown edit, and file preview. | `packages/client/src/components/ChannelView.tsx`, `packages/client/src/components/WorkspacePanel.tsx`, `packages/client/src/components/FileViewer.tsx`, `packages/client/src/components/MarkdownEditor.tsx` |
| Global workspace manager | `WorkspaceManager` fetches all workspace files, groups by channel, supports channel filtering, preview, and rename. It is a sidepane selected by `mainView === 'workspaces'`. | `packages/client/src/App.tsx`, `packages/client/src/components/WorkspaceManager.tsx`, `packages/client/src/lib/api.ts` |

`FileViewer` downloads through the workspace API, then renders images through `ImageViewer`, Markdown through `MarkdownViewer`, code through `CodeViewer`, text through `TextViewer`, and a binary unsupported fallback otherwise. Evidence: `packages/client/src/components/FileViewer.tsx`, `packages/client/src/components/viewers/*`, `packages/client/src/lib/api.ts`.

### Artifact / Canvas

The active canvas surface is `ChannelView` tab `canvas` rendering `ArtifactPanel`. `ArtifactPanel` is channel-scoped and manages one visible artifact head, version list, edit/commit, rollback, anchor review threads, diff view, and owner-only iteration controls. Evidence: `packages/client/src/components/ChannelView.tsx`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/AnchorThreadPanel.tsx`, `packages/client/src/components/IteratePanel.tsx`.

Artifacts support `markdown`, `code`, `image_link`, `video_link`, and `pdf_link` kinds in the client type and renderer switch. Markdown goes through `renderMarkdown`; code, image, video, and PDF link kinds use dedicated renderers. Unsupported kinds render an explicit fallback. Evidence: `packages/client/src/lib/api.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/CodeRenderer.tsx`, `packages/client/src/components/ImageLinkRenderer.tsx`, `packages/client/src/components/MediaPreview.tsx`.

Artifact content changes follow the pull-after-signal contract: `artifact_updated` wakes the panel, and the panel then calls `getArtifact` plus `listArtifactVersions`. Anchor and comment frames likewise refetch comments instead of rendering frame previews as content. Evidence: `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/ArtifactComments.tsx`, `packages/client/src/types/ws-frames.ts`.

### Remote Nodes

Remote nodes have an admin-like user-side manager and a channel browsing tab, but they still use the user REST rail (`/api/v1/remote/*`), not `/admin-api`. Evidence: `packages/client/src/components/NodeManager.tsx`, `packages/client/src/components/RemotePanel.tsx`, `packages/client/src/lib/api.ts`.

| Surface | Boundary | Code evidence |
| --- | --- | --- |
| Node manager sidepane | Lists remote nodes, polls status per node, creates/deletes nodes, displays connection token on demand, builds a remote-agent start command from `VITE_AGENT_WS_SERVER`, and manages channel/path bindings. | `packages/client/src/App.tsx`, `packages/client/src/components/NodeManager.tsx`, `packages/client/src/lib/api.ts` |
| Channel remote tab | Lists bindings for the current channel and opens a selected binding into a remote tree rooted at that path. | `packages/client/src/components/ChannelView.tsx`, `packages/client/src/components/RemotePanel.tsx`, `packages/client/src/lib/api.ts` |
| Remote tree / file viewer | `RemoteTree` calls `remoteLs` and `remoteReadFile`, sorts directories before files, and previews content through `RemoteFileViewer`. | `packages/client/src/components/RemoteTree.tsx`, `packages/client/src/components/RemoteFileViewer.tsx`, `packages/client/src/lib/api.ts` |

Remote file viewing is read-only in the current UI. Node connection tokens are hidden by default and only shown after the user presses the token toggle. Evidence: `packages/client/src/components/NodeManager.tsx`, `packages/client/src/components/RemoteTree.tsx`, `packages/client/src/components/RemoteFileViewer.tsx`.

## Admin Boundary From User SPA

The user SPA knows about admin impact only through user-owned endpoints: own audit entries and own impersonation grant. It does not mount the admin router, admin auth provider, or `/admin-api/v1` client. Evidence: `packages/client/src/App.tsx`, `packages/client/src/lib/api.ts`, `packages/client/src/components/Settings/SettingsPage.tsx`, `packages/client/src/admin/main.tsx`, `packages/client/src/admin/api.ts`.

The user settings privacy UI states the intended boundary explicitly: admins see metadata such as usernames, channel names, counts, and login times, but not message bodies, files, artifact content, built-in owner-agent DMs, or raw API keys unless the user grants temporary impersonation. This is UI documentation inside the product and should be kept aligned with server behavior. Evidence: `packages/client/src/components/Settings/PrivacyPromise.tsx`, `packages/client/src/components/Settings/BannerImpersonate.tsx`, `packages/client/src/lib/api.ts`.
