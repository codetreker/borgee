# App Shell State

This document covers the user SPA entry, root providers, auth bootstrap, top-level view state, and `AppContext`. Realtime frame handling is covered in `realtime-sync.md`; feature-specific component ownership is covered in `feature-surfaces.md`.

## Module Overview

```text
index.html
  -> src/main.tsx
    -> App
      -> ThemeProvider
        -> AppProvider
          -> ToastProvider
            -> AppInner
              -> auth check + init loads
              -> Sidebar
              -> ChannelView or sidepane selected by mainView
```

The user entry is `packages/client/src/main.tsx`. It mounts `<App />` into `#root`, wraps it in `React.StrictMode`, and registers `/sw.js` after the window `load` event when `navigator.serviceWorker` is available (`packages/client/src/main.tsx`, `packages/client/index.html`).

`App` provides `ThemeProvider`, `AppProvider`, and `ToastProvider` around `AppInner`. The admin entry does not participate in this provider tree; it mounts `AdminAuthProvider` and `AdminApp` separately (`packages/client/src/App.tsx`, `packages/client/src/admin/main.tsx`).

## Responsibilities

This module is responsible for bootstrapping user auth state, initializing shared user data, owning the top-level `mainView`, wiring the WebSocket send/ack functions into `AppContext`, rendering the responsive sidebar wrapper, and choosing the active shell surface (`packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts`).

This module is not responsible for route matching. The user SPA does not use `react-router-dom` in `App.tsx`; it uses reducer state plus a `mainView` string. React Router is used by the admin SPA only (`packages/client/src/App.tsx`, `packages/client/src/lib/mainView.ts`, `packages/client/src/admin/AdminApp.tsx`).

This module is not responsible for backend authorization or persistence. It calls `fetchMe`, `fetchMyPermissions`, `fetchChannels`, `fetchOnlineUsers`, message APIs, and related user endpoints through `lib/api.ts` (`packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/lib/api.ts`).

## Auth And Initialization

`AppInner` first calls `fetchMe` to decide whether to render login/register or the authenticated app. On successful login it waits briefly for `fetchMe` to become ready, then sets `authenticated` so the post-auth initialization effect can run (`packages/client/src/App.tsx`, `packages/client/src/lib/api.ts`).

After authentication, `AppInner` calls `actions.loadCurrentUser()`, `actions.loadPermissions()`, `actions.loadChannels()`, and `actions.loadOnlineUsers()` before dispatching `SET_INITIALIZED`. It also refreshes online users every 30 seconds while authenticated (`packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx`).

If no channel is selected after initialization, the shell auto-selects the first `system` channel, otherwise the first channel. If channel loading leaves no selected channel, the fallback UI offers a retry that calls `actions.loadChannels()` (`packages/client/src/App.tsx`).

## Top-Level View State

The user shell uses a single `mainView` value rather than independent booleans. The active values are `channel`, `agents`, `invitations`, `workspaces`, `remote-nodes`, and `settings`; `MAIN_VIEW_DEFAULT` is defined in `packages/client/src/lib/mainView.ts` and used by `App.tsx` (`packages/client/src/App.tsx`, `packages/client/src/lib/mainView.ts`).

Sidebar callbacks call `requestMainView`. Before replacing `mainView`, `requestMainView` runs `runUnsavedGuards()` so dirty editors/forms can block navigation (`packages/client/src/App.tsx`, `packages/client/src/hooks/useUnsavedChangesGuard.ts`).

The shell renders sidepanes directly from `mainView`: `AgentManager`, `InvitationsInbox`, `WorkspaceManager`, `NodeManager`, and `SettingsPage`. When `mainView === 'channel'`, the shell renders `ChannelView` for `state.currentChannelId` (`packages/client/src/App.tsx`, `packages/client/src/components/AgentManager.tsx`, `packages/client/src/components/InvitationsInbox.tsx`, `packages/client/src/components/WorkspaceManager.tsx`, `packages/client/src/components/NodeManager.tsx`, `packages/client/src/components/Settings/SettingsPage.tsx`, `packages/client/src/components/ChannelView.tsx`).

## AppContext State

`AppProvider` owns the user reducer and exposes `state`, `dispatch`, `sendWsMessage`, `registerAckTimer`, setter functions used by `AppInner`, and an `actions` object for common REST-backed operations (`packages/client/src/context/AppContext.tsx`, `packages/client/src/App.tsx`).

| State | Responsibility | Evidence |
| --- | --- | --- |
| `channels`, `groups`, `dmChannels` | User channel rail, grouped channels, and DM rail data. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/types.ts`, `packages/client/src/components/Sidebar.tsx` |
| `currentChannelId` | Selected channel or DM for `ChannelView`. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/App.tsx` |
| `messages`, `hasMore`, `loadingMessages` | Per-channel message cache and pagination state. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/components/MessageList.tsx` |
| `pendingMessages` | Optimistic outbound messages awaiting WS ack/nack or timeout. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/components/MessageInput.tsx`, `packages/client/src/hooks/useWebSocket.ts` |
| `currentUser`, `permissions` | Authenticated user identity and capability details. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/usePermissions.ts`, `packages/client/src/lib/api.ts` |
| `onlineUserIds`, `connectionState`, `typingUsers` | Online presence, WS connection banner state, and transient typing indicators. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/components/ConnectionStatus.tsx`, `packages/client/src/components/TypingIndicator.tsx` |
| `channelMembersVersion` | Version counter used to wake member-dependent UI after membership frames. | `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/components/Sidebar.tsx` |
| `initialized` | Gate that prevents rendering the authenticated app before bootstrap data loads. | `packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx` |

Context actions are thin orchestration around `lib/api.ts`. They load channels/groups, load messages and older pages, load current user, load permissions, load online users, select a channel, send a message through the REST fallback API, create a channel, load DM channels, and open a DM (`packages/client/src/context/AppContext.tsx`, `packages/client/src/lib/api.ts`).

`selectChannel` is both navigation and read-state coordination: it sets `currentChannelId`, clears unread counts locally, and fire-and-forgets `markChannelRead` to the server (`packages/client/src/context/AppContext.tsx`, `packages/client/src/lib/api.ts`).

## Interfaces To Other Modules

| Interface | Used by this module | Contract | Evidence |
| --- | --- | --- | --- |
| REST API client | `AppInner` and `AppContext` actions | Auth/init and state loading use `packages/client/src/lib/api.ts`. | `packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/lib/api.ts` |
| WebSocket hook | `AppInner` | `useWebSocket` returns `subscribe`, `sendWsMessage`, and `registerAckTimer`; `AppInner` stores send/ack functions in context. | `packages/client/src/App.tsx`, `packages/client/src/hooks/useWebSocket.ts` |
| Unsaved guard registry | `requestMainView` | Dirty feature forms can register guards; shell navigation runs them before switching sidepanes. | `packages/client/src/App.tsx`, `packages/client/src/hooks/useUnsavedChangesGuard.ts`, `packages/client/src/components/AgentConfigPanel.tsx`, `packages/client/src/components/NodeManager.tsx` |
| Sidebar/component surfaces | Rendered by shell | Sidebar owns channel/DM rail interactions; sidepane components own feature-specific local state. | `packages/client/src/App.tsx`, `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/*` |
