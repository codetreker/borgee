# UI Map

This is a maintainer locator for the user SPA. It maps visible areas to source files, state owners, and API hooks. It is not a UI design spec and should not be used as a source of visual styling requirements.

## Module Overview

```text
User window
  Sidebar: channels, DMs, footer actions
  Main content:
    ChannelView chat/canvas/workspace/remote
    or one mainView sidepane
  Global authenticated banner: BannerImpersonate
```

The source of truth for current layout behavior is the rendered component tree in `App.tsx`: `BannerImpersonate`, optional mobile hamburger/overlay, `Sidebar`, and `main-content` containing either a `mainView` sidepane or `ChannelView` (`packages/client/src/App.tsx`).

## Responsibilities

This document is responsible for helping maintainers find the component and API boundary for a user-visible area (`packages/client/src/App.tsx`, `packages/client/src/components/*`, `packages/client/src/hooks/*`, `packages/client/src/lib/api.ts`).

This document is not responsible for specifying spacing, color, copy, visual hierarchy, interaction design, or accessibility acceptance criteria. Those must be read from components, CSS, tests, and product specs where applicable (`packages/client/src/index.css`, `packages/client/src/components/*`, `packages/client/src/__tests__/*`).

This document is not responsible for admin SPA UI. Admin routes and pages are documented in `../admin/spa.md` and live under `packages/client/src/admin/*` (`packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/pages/*`).

## Shell Locator

| UI area | Primary files | State/API boundary | Evidence |
| --- | --- | --- | --- |
| Root entry | `src/main.tsx`, `App.tsx` | Mounts user app and service worker; no router. | `packages/client/src/main.tsx`, `packages/client/src/App.tsx` |
| Providers | `ThemeContext.tsx`, `AppContext.tsx`, `Toast.tsx` | Theme, user state reducer, toast context. | `packages/client/src/App.tsx`, `packages/client/src/context/ThemeContext.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/components/Toast.tsx` |
| Mobile shell | `App.tsx` | `isMobile`, `sidebarOpen`, resize listener, overlay/hamburger. | `packages/client/src/App.tsx` |
| Sidepane switching | `App.tsx`, `mainView.ts`, `useUnsavedChangesGuard.ts` | Single `mainView` string; guard registry before navigation. | `packages/client/src/App.tsx`, `packages/client/src/lib/mainView.ts`, `packages/client/src/hooks/useUnsavedChangesGuard.ts` |
| Active impersonation banner | `BannerImpersonate.tsx` | Polls and revokes user-owned grant. | `packages/client/src/App.tsx`, `packages/client/src/components/Settings/BannerImpersonate.tsx`, `packages/client/src/lib/api.ts` |

## Navigation And Rail Locator

| UI area | Primary files | State/API boundary | Evidence |
| --- | --- | --- | --- |
| Channel list | `Sidebar.tsx`, `ChannelList.tsx`, `SortableChannelItem.tsx`, `ChannelGroupComponent.tsx` | `state.channels`, `state.groups`, channel reorder/group APIs. | `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/ChannelList.tsx`, `packages/client/src/lib/api.ts` |
| Create channel/group | `Sidebar.tsx`, `CreateGroupModal.tsx` | `actions.createChannel`, `createChannelGroup`, `useCan('channel.create')`. | `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/CreateGroupModal.tsx`, `packages/client/src/hooks/usePermissions.ts` |
| DM list | `Sidebar.tsx` | `state.dmChannels`, `actions.loadDmChannels`, `actions.openDm`, online status. | `packages/client/src/components/Sidebar.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/lib/api.ts` |
| Sidebar footer actions | `Sidebar.tsx` | Opens `agents`, `invitations`, `workspaces`, `remote-nodes`, `settings`; hides some actions for agent users. | `packages/client/src/components/Sidebar.tsx`, `packages/client/src/App.tsx` |
| Invitation bell | `Sidebar.tsx`, `InvitationsInbox.tsx` | `listAgentInvitations`, `useInvitationFrames`, 60s fallback poll. | `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/InvitationsInbox.tsx`, `packages/client/src/hooks/useWsHubFrames.ts` |

## Channel View Locator

| UI area | Primary files | State/API boundary | Evidence |
| --- | --- | --- | --- |
| Channel header | `ChannelView.tsx`, `ChannelMembersModal.tsx`, `MemberList.tsx` | Channel/DM lookup, member modal, join/leave, preview mode. | `packages/client/src/components/ChannelView.tsx`, `packages/client/src/components/ChannelMembersModal.tsx`, `packages/client/src/lib/api.ts` |
| Tabs | `ChannelView.tsx` | Non-DM joined channels show `chat`, `canvas`, `workspace`, `remote`; only `chat`/`workspace` sync to URL. | `packages/client/src/components/ChannelView.tsx` |
| Connection banner | `ConnectionStatus.tsx`, `SyncStatusIndicator.tsx` | `state.connectionState` from `useWebSocket`. | `packages/client/src/components/ConnectionStatus.tsx`, `packages/client/src/components/SyncStatusIndicator.tsx`, `packages/client/src/hooks/useWebSocket.ts` |
| Message list | `MessageList.tsx`, `MessageItem.tsx`, reaction components, edit/history components | `state.messages`, `state.pendingMessages`, members, reactions, mention-pushed refresh. | `packages/client/src/components/MessageList.tsx`, `packages/client/src/components/MessageItem.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWsHubFrames.ts` |
| Message composer | `MessageInput.tsx`, `MentionPicker.tsx`, `SlashCommandPicker.tsx`, `EmojiPickerPopover.tsx` | WS send, upload, mentions, slash commands, typing. | `packages/client/src/components/MessageInput.tsx`, `packages/client/src/commands/registry.ts`, `packages/client/src/hooks/useSlashCommands.ts`, `packages/client/src/extensions/mention.ts` |
| Public preview | `ChannelView.tsx`, `MessageList.tsx` | `fetchChannelPreview`, join channel, no composer until joined. | `packages/client/src/components/ChannelView.tsx`, `packages/client/src/lib/api.ts` |

## Feature Sidepanes And Tabs

| UI area | Primary files | State/API boundary | Evidence |
| --- | --- | --- | --- |
| Canvas/artifact tab | `ArtifactPanel.tsx`, `DiffView.tsx`, `AnchorThreadPanel.tsx`, `IteratePanel.tsx` | Artifact head/version/commit/rollback/anchor/iteration APIs; WS signal pull. | `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/lib/api.ts`, `packages/client/src/hooks/useWsHubFrames.ts` |
| Artifact comments | `ArtifactComments.tsx`, comment body/item/thread/search/edit-history components | Comment REST APIs and `artifact_comment_added` signal pull. | `packages/client/src/components/ArtifactComments.tsx`, `packages/client/src/components/ArtifactCommentThread.tsx`, `packages/client/src/lib/api.ts` |
| Workspace tab | `WorkspacePanel.tsx`, `FileViewer.tsx`, `MarkdownEditor.tsx`, viewer components | Channel workspace file APIs and download/viewer selection. | `packages/client/src/components/WorkspacePanel.tsx`, `packages/client/src/components/FileViewer.tsx`, `packages/client/src/components/viewers/*` |
| Workspaces sidepane | `WorkspaceManager.tsx` | `fetchAllWorkspaces`, grouping/filtering, preview, rename. | `packages/client/src/components/WorkspaceManager.tsx`, `packages/client/src/lib/api.ts` |
| Remote tab | `RemotePanel.tsx`, `RemoteTree.tsx`, `RemoteFileViewer.tsx` | Channel remote bindings, read-only remote `ls` and file reads. | `packages/client/src/components/RemotePanel.tsx`, `packages/client/src/components/RemoteTree.tsx`, `packages/client/src/components/RemoteFileViewer.tsx` |
| Remote nodes sidepane | `NodeManager.tsx` | Node CRUD/status/token/start command/binding APIs. | `packages/client/src/components/NodeManager.tsx`, `packages/client/src/lib/api.ts` |
| Agent sidepane | `AgentManager.tsx`, `AgentConfigPanel.tsx`, `RuntimeCard.tsx`, `HostGrantsPanel.tsx` | Agent CRUD, permissions, key, runtime, config APIs. | `packages/client/src/components/AgentManager.tsx`, `packages/client/src/components/AgentConfigPanel.tsx`, `packages/client/src/lib/api.ts` |
| Invitations sidepane | `InvitationsInbox.tsx` | Owner invitation list/approve/reject plus invitation frame refresh. | `packages/client/src/components/InvitationsInbox.tsx`, `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/lib/api.ts` |
| Settings sidepane | `Settings/SettingsPage.tsx`, `PrivacyPromise.tsx`, `ImpersonateGrantSection.tsx`, `AdminActionsList.tsx` | User-owned admin-awareness endpoints and grant operations. | `packages/client/src/components/Settings/SettingsPage.tsx`, `packages/client/src/components/Settings/*`, `packages/client/src/lib/api.ts` |

## Hooks Locator

| Hook/module | Main users | Responsibility | Evidence |
| --- | --- | --- | --- |
| `useWebSocket` | `App.tsx`, `AppContext`, chat components indirectly | `/ws` connection, subscribe, frame dispatch, pending ack timers, reconnect/backfill. | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/App.tsx` |
| `useWsHubFrames` | Invitations, messages, artifact/comment/iteration surfaces | Convert signal-only WS frames to window events and hooks. | `packages/client/src/hooks/useWsHubFrames.ts` |
| `usePermissions` | Sidebar and permission-gated controls | Check current user's permissions from `AppContext`. | `packages/client/src/hooks/usePermissions.ts`, `packages/client/src/context/AppContext.tsx` |
| `useUnsavedChangesGuard` | Shell sidepane switching, agent config, remote forms | Register dirty guards and run them before navigation. | `packages/client/src/hooks/useUnsavedChangesGuard.ts`, `packages/client/src/App.tsx` |
| `usePresence` / `useRT3Presence` | Agent/DM presence displays | Local presence cache updated by WS runtime frames. | `packages/client/src/hooks/usePresence.ts`, `packages/client/src/hooks/useRT3Presence.ts`, `packages/client/src/components/AgentManager.tsx` |
| `useSlashCommands` | Message composer | Filter built-in and remote slash commands. | `packages/client/src/hooks/useSlashCommands.ts`, `packages/client/src/components/MessageInput.tsx`, `packages/client/src/commands/registry.ts` |
| `useVisualViewport` | Channel view | Adjust channel viewport when virtual keyboard changes. | `packages/client/src/hooks/useVisualViewport.ts`, `packages/client/src/components/ChannelView.tsx` |

## Interfaces To Other Modules

The UI map interfaces with `app-shell-state.md` for shell ownership, `realtime-sync.md` for WS/REST behavior, `feature-surfaces.md` for deeper feature boundaries, and `build-pwa-cache.md` for entry/build/cache behavior. Source evidence remains the code paths listed in each row above.
