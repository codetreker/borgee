# Realtime Sync

This document covers the user SPA data-sync model: REST as authority, WebSocket as live delivery and wake-up signal, reconnect/backfill, and pending-message reconciliation. Shell state is covered in `app-shell-state.md`.

## Module Overview

```text
User action or mount
  -> lib/api.ts pulls / mutates /api/v1 authoritative data
  -> useWebSocket receives /ws frames
     -> reducer-updating frames mutate AppContext directly
     -> signal-only frames dispatch window CustomEvents
        -> feature component refetches REST state
  -> reconnect uses cursor backfill and per-channel message pull
```

The client sync contract is REST-first. `packages/client/src/lib/api.ts` contains same-origin `/api/v1/*` calls with `credentials: 'include'`; components and context actions use those calls for initial state, list refresh, file/artifact content, comments, permissions, workspace data, remote data, admin-awareness rows, and mutations (`packages/client/src/lib/api.ts`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/components/*`).

`useWebSocket` connects to `ws://<host>/ws` or `wss://<host>/ws` and optionally appends `user_id` from the dev user helper. It exposes `subscribe`, `unsubscribe`, `sendWsMessage`, `registerAckTimer`, and `connectionState` (`packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/api.ts`).

## Responsibilities

This module is responsible for documenting how live frames update user state, how dropped connections are reconciled, how message acks map to optimistic pending messages, and which frame types are only wake-up signals (`packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/context/AppContext.tsx`).

This module is not responsible for server frame schema definitions or backend event storage. The client consumes frame shapes in `packages/client/src/types/ws-frames.ts` and REST response shapes in `packages/client/src/lib/api.ts`; server ownership is outside the frontend docs (`packages/client/src/types/ws-frames.ts`, `packages/client/src/hooks/useWebSocket.ts`).

This module is not responsible for admin realtime behavior. The admin SPA does not mount `useWebSocket`; admin pages are REST-driven through `/admin-api/v1` (`packages/client/src/admin/main.tsx`, `packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/api.ts`, `packages/client/src/hooks/useWebSocket.ts`).

## REST API Client

`lib/api.ts` uses `BASE = ''`, so production and Vite dev proxy both use same-origin requests. It includes cookies, adds JSON `Content-Type` only for non-`FormData` bodies, and lets the browser provide multipart boundaries for uploads (`packages/client/src/lib/api.ts`, `packages/client/vite.config.ts`).

Important user REST groups:

| Group | Responsibility | Evidence |
| --- | --- | --- |
| Auth/user | Login, logout, register, current user, profile. | `packages/client/src/lib/api.ts`, `packages/client/src/App.tsx` |
| Channels/groups/members | Channel list, create/update/join/leave/delete/reorder, groups, members, read markers, visibility, pin/mute/notification prefs. | `packages/client/src/lib/api.ts`, `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/ChannelView.tsx` |
| Messages/DMs | Message pages, sends, edits, deletes, reactions, DMs, DM edit/history/search helpers. | `packages/client/src/lib/api.ts`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/components/MessageInput.tsx` |
| Backfill | `fetchEventsBackfill(since)` calls `/api/v1/events?since=` after reconnect with a stored cursor. | `packages/client/src/lib/api.ts`, `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/lastSeenCursor.ts` |
| Agents/runtime/config | Agent CRUD, permissions, API key rotation, runtime start/stop, config, recovery. | `packages/client/src/lib/api.ts`, `packages/client/src/components/AgentManager.tsx`, `packages/client/src/components/AgentConfigPanel.tsx` |
| Workspace/uploads | Message image upload plus channel/global workspace file operations. | `packages/client/src/lib/api.ts`, `packages/client/src/components/MessageInput.tsx`, `packages/client/src/components/WorkspacePanel.tsx`, `packages/client/src/components/WorkspaceManager.tsx` |
| Remote nodes | Remote node CRUD/status/token bindings, channel bindings, remote `ls` and read-only file reads. | `packages/client/src/lib/api.ts`, `packages/client/src/components/NodeManager.tsx`, `packages/client/src/components/RemotePanel.tsx`, `packages/client/src/components/RemoteTree.tsx` |
| Artifacts/comments | Artifact create/get/version/commit/rollback, iterations, anchors, comments, comment search, edit history. | `packages/client/src/lib/api.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/ArtifactComments.tsx`, `packages/client/src/components/IteratePanel.tsx` |
| User admin-awareness | Own admin action history and own impersonation grant. | `packages/client/src/lib/api.ts`, `packages/client/src/components/Settings/SettingsPage.tsx`, `packages/client/src/components/Settings/BannerImpersonate.tsx` |

## WebSocket Connection

`useWebSocket` keeps a set of subscribed channel IDs. On open, it resubscribes every stored channel and marks connection state connected. `AppInner` subscribes all joined channels after initialization, and the hook also auto-subscribes DM channels from context state (`packages/client/src/App.tsx`, `packages/client/src/hooks/useWebSocket.ts`).

The hook sends a ping every 25 seconds while the socket is open. On close, it schedules reconnects with delays from 1 second up to 30 seconds, except auth failure close codes `4001` and `4003`, which leave the connection disconnected (`packages/client/src/hooks/useWebSocket.ts`).

The hook normalizes both direct frames and hub envelope frames through `flattenWsFrame`, so handlers read a flattened `{ type, ...payload }` shape regardless of whether the server sent `{type, data: payload}` or a direct payload frame (`packages/client/src/hooks/useWebSocket.ts`).

## Reconnect Backfill

When reconnecting after a dropped socket, `useWebSocket` loads the stored cursor from `lastSeenCursor`. If the cursor is greater than zero, it calls `fetchEventsBackfill(since)`, dispatches each returned event through the same message handler, and persists the returned cursor if it advanced (`packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/lastSeenCursor.ts`, `packages/client/src/lib/api.ts`).

Backfill is not a cold-start full history load. If `loadLastSeenCursor()` returns `0`, the event backfill is skipped; normal state comes from REST list/message loads and per-channel reconciliation (`packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/context/AppContext.tsx`).

The hook also reconciles missed messages per subscribed channel. It tracks the latest message timestamp per channel and calls `fetchMessages(channelId, { after: lastTs, limit: 50 })` after reconnect, then dispatches `ADD_MESSAGE` for each returned message and clears matching pending messages (`packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/api.ts`).

## Direct Reducer Frames

Some frames are treated as state updates and dispatch directly into `AppContext`:

| Frame family | Reducer effect | Evidence |
| --- | --- | --- |
| `new_message`, `message_ack`, `message_nack`, `message_edited`, `message_deleted` | Add/edit/delete messages and resolve pending messages. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/context/AppContext.tsx` |
| `presence`, `presence.changed` | Update user online set or agent runtime presence cache. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/hooks/usePresence.ts` |
| `channel_created`, `channel_added`, `channel_removed`, `channel_deleted`, `visibility_changed`, `channel_updated`, `channels_reordered` | Add/remove/update channel rows and subscriptions. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/context/AppContext.tsx` |
| `user_joined`, `user_left` | Update member count and bump `channelMembersVersion`. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/components/Sidebar.tsx` |
| `typing`, `reaction_update` | Set transient typing state and replace reaction aggregates. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/context/AppContext.tsx` |
| Group frames | Add/update/reorder/delete channel groups. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/context/AppContext.tsx` |
| `commands_updated` | Dispatch a browser event that `ChannelView` debounces before reloading commands. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/components/ChannelView.tsx` |

## Pending Message Flow

```text
MessageInput or retry in MessageList
  -> ADD_PENDING_MESSAGE
  -> sendWsMessage({ type: "send_message", channel_id, content, content_type, client_message_id })
  -> register 10s timeout
  -> message_ack: ACK_PENDING_MESSAGE and insert server message
  -> message_nack or timeout: FAIL_PENDING_MESSAGE
```

Text messages and uploaded image messages both follow the pending-message path. For images, `MessageInput` first calls `uploadImage`, then sends the returned URL as an image message over WS (`packages/client/src/components/MessageInput.tsx`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/api.ts`).

## Signal Then Pull

Signal-only frames are bridged to browser `CustomEvent`s in `useWsHubFrames`; consumers refetch REST state instead of rendering frame previews as authoritative data (`packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/hooks/useWebSocket.ts`).

| WS signal | Client bridge | Follow-up pull | Consumer evidence |
| --- | --- | --- | --- |
| `agent_invitation_pending`, `agent_invitation_decided` | `useInvitationFrames` / `borgee:invitation-*` | `listAgentInvitations` refreshes inbox and bell badge. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/InvitationsInbox.tsx` |
| `artifact_updated` | `useArtifactUpdated` | `getArtifact` and `listArtifactVersions` reload artifact head and versions. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/lib/api.ts` |
| `mention_pushed` | `useMentionPushed` | Matching channel calls `actions.loadMessages`; body preview is not rendered as message body. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/types/ws-frames.ts` |
| `anchor_comment_added` | `useAnchorCommentAdded` | Artifact anchors/comments are refetched. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/AnchorThreadPanel.tsx`, `packages/client/src/lib/api.ts` |
| `iteration_state_changed` | `useIterationStateChanged` | Iteration state/body is refetched by artifact iteration UI. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/IteratePanel.tsx`, `packages/client/src/types/ws-frames.ts` |
| `artifact_comment_added` | `useArtifactCommentAdded` | Artifact comments are refetched from `/api/v1/artifacts/:id/comments`; body preview is not the rendered comment body. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/ArtifactComments.tsx`, `packages/client/src/lib/api.ts`, `packages/client/src/types/ws-frames.ts` |

## Interfaces To Other Modules

| Interface | Contract | Evidence |
| --- | --- | --- |
| `AppContext` reducer | Receives direct frame actions and stores pending messages, connection state, typing, channels, messages, and DMs. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts` |
| Feature components | Listen to `useWsHubFrames` hooks and pull REST data after signal frames. | `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/InvitationsInbox.tsx` |
| Backend REST rail | Source of truth for state after mount, reconnect, and signal frames. | `packages/client/src/lib/api.ts` |
| Vite dev proxy | `/ws` is proxied with WebSocket support; `/api` and `/uploads` use the same target. | `packages/client/vite.config.ts` |
