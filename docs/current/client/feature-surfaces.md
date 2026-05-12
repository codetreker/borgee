# Feature Surfaces

This document maps the user SPA feature surfaces to their owning components, hooks, and API boundaries. It does not define product UX copy or visual design rules; `ui-map.md` is the shorter locator index.

## Module Overview

```text
App mainView
  channel -> ChannelView
    chat -> MessageList + MessageInput
    DM -> same chat stack without channel tabs
    canvas -> ArtifactPanel
    workspace -> WorkspacePanel
    remote -> RemotePanel + RemoteTree
  agents -> AgentManager + AgentConfigPanel
  invitations -> InvitationsInbox
  workspaces -> WorkspaceManager
  remote-nodes -> NodeManager
  settings -> SettingsPage + privacy/impersonation components
```

Feature surfaces sit below the user shell and above `lib/api.ts`, `AppContext`, and `useWebSocket`. The shell decides which surface is visible; each surface owns its local form/list/detail state and calls the user REST client for authoritative data (`packages/client/src/App.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/lib/api.ts`, `packages/client/src/components/*`).

## Responsibilities

This module is responsible for describing user-facing surfaces that run on the user rail: chat, channel/DM rail, artifact/canvas, workspace, remote nodes, settings/admin-awareness, agents, and invitations (`packages/client/src/components/ChannelView.tsx`, `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/WorkspacePanel.tsx`, `packages/client/src/components/NodeManager.tsx`, `packages/client/src/components/AgentManager.tsx`, `packages/client/src/components/Settings/SettingsPage.tsx`).

This module is not responsible for admin SPA pages, admin auth, or `/admin-api/v1`. Admin pages live in `packages/client/src/admin/pages/*` and use `packages/client/src/admin/api.ts` (`packages/client/src/admin/AdminApp.tsx`, `packages/client/src/admin/api.ts`).

This module is not responsible for server-side ACL enforcement, file storage, remote execution, artifact persistence, or audit-log generation. Components call `lib/api.ts`; the backend owns persistence and authorization (`packages/client/src/lib/api.ts`).

## Channel And DM Surface

`Sidebar` renders non-DM channels through `ChannelList`, DMs through `MergedDmList`, create channel/group controls gated by `useCan('channel.create')`, a theme toggle, logout, and sidepane buttons. Agent, invitation, remote-node, and settings buttons are hidden for `role === 'agent'`; the Workspaces button is rendered whenever the shell passes `onWorkspacesOpen` (`packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/ChannelList.tsx`, `packages/client/src/hooks/usePermissions.ts`).

`ChannelView` distinguishes normal channel, DM, and public-preview states. DMs do not render the tab switcher; joined non-DM channels render `chat`, `canvas`, `workspace`, and `remote` tabs, with only `chat` and `workspace` synchronized to `?tab=` (`packages/client/src/components/ChannelView.tsx`).

Public preview mode uses `fetchChannelPreview` and a join button. Joined chat mode renders `ConnectionStatus`, `MessageList`, and `MessageInput` (`packages/client/src/components/ChannelView.tsx`, `packages/client/src/components/ConnectionStatus.tsx`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/components/MessageInput.tsx`, `packages/client/src/lib/api.ts`).

DM channels reuse the chat components but have DM-specific rail handling: `Sidebar` loads `dmChannels`, `openDm` creates or selects a DM, and `useWebSocket` auto-subscribes DM channels (`packages/client/src/components/Sidebar.tsx`, `packages/client/src/context/AppContext.tsx`, `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/lib/api.ts`).

## Chat Surface

`MessageList` renders fetched messages plus pending pseudo-messages, handles older-page loading, member-name lookup, retry for failed pending messages, and `mention_pushed` refresh via `useMentionPushed` (`packages/client/src/components/MessageList.tsx`, `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/context/AppContext.tsx`).

`MessageInput` owns rich message composition. It uses TipTap with mention support, slash-command suggestions, emoji insertion, typing signals, image paste/drop/file selection, pending-message creation, WS send, and ack timeout registration (`packages/client/src/components/MessageInput.tsx`, `packages/client/src/extensions/mention.ts`, `packages/client/src/hooks/useSlashCommands.ts`, `packages/client/src/commands/registry.ts`, `packages/client/src/lib/api.ts`).

Image uploads are a composer boundary. `MessageInput` calls `uploadImage` via `POST /api/v1/upload`, then sends a `content_type: 'image'` message over WS with the returned URL (`packages/client/src/components/MessageInput.tsx`, `packages/client/src/lib/api.ts`).

## Artifact / Canvas Surface

The current canvas tab is `ChannelView` tab `canvas`, which renders `ArtifactPanel` directly. `AppShell` and `ArtifactDrawer` exist in the codebase, but the current root app does not import or mount them (`packages/client/src/components/ChannelView.tsx`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/AppShell.tsx`, `packages/client/src/components/ArtifactDrawer.tsx`, `packages/client/src/App.tsx`).

`ArtifactPanel` is channel-scoped. It loads the current artifact head and versions, can create an artifact, edit and commit body changes, rollback versions, enter a diff view, manage anchors, and show owner-only iteration controls where applicable (`packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/DiffView.tsx`, `packages/client/src/components/AnchorThreadPanel.tsx`, `packages/client/src/components/IteratePanel.tsx`, `packages/client/src/lib/api.ts`).

Artifact rendering supports the client artifact kinds declared in `lib/api.ts`: Markdown, code, image link, video link, and PDF link. Rendering is handled by `ArtifactPanel`, `CodeRenderer`, `ImageLinkRenderer`, and `MediaPreview` (`packages/client/src/lib/api.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/CodeRenderer.tsx`, `packages/client/src/components/ImageLinkRenderer.tsx`, `packages/client/src/components/MediaPreview.tsx`).

Artifact updates follow signal-then-pull. `artifact_updated` triggers `getArtifact` and `listArtifactVersions`; `anchor_comment_added` and `artifact_comment_added` wake comment-related UI, which refetches comment bodies through REST instead of using WS previews as rendered content (`packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/ArtifactComments.tsx`, `packages/client/src/lib/api.ts`).

## Workspace Surface

The workspace has a channel tab and a global sidepane. `WorkspacePanel` is channel-scoped; it lists files/folders, supports drag/drop upload, file input upload, mkdir, delete, move/rename, Markdown edit/preview, and file viewing (`packages/client/src/components/WorkspacePanel.tsx`, `packages/client/src/components/FileViewer.tsx`, `packages/client/src/components/MarkdownEditor.tsx`, `packages/client/src/lib/api.ts`).

`WorkspaceManager` is the all-workspaces sidepane selected by `mainView === 'workspaces'`. It calls `fetchAllWorkspaces`, groups by channel, supports channel filtering, file preview, and rename (`packages/client/src/App.tsx`, `packages/client/src/components/WorkspaceManager.tsx`, `packages/client/src/lib/api.ts`).

Workspace file display is delegated to `FileViewer`, which downloads file content through the workspace API and then selects image, Markdown, code, text, or unsupported-binary rendering through viewer components (`packages/client/src/components/FileViewer.tsx`, `packages/client/src/components/viewers/ImageViewer.tsx`, `packages/client/src/components/viewers/MarkdownViewer.tsx`, `packages/client/src/components/viewers/CodeViewer.tsx`, `packages/client/src/components/viewers/TextViewer.tsx`, `packages/client/src/lib/api.ts`).

## Remote Surface

Remote nodes have a user-side node manager and a channel browsing tab. Both use the user REST rail (`/api/v1/remote/*` and `/api/v1/channels/:id/remote-bindings`), not the admin API (`packages/client/src/lib/api.ts`, `packages/client/src/components/NodeManager.tsx`, `packages/client/src/components/RemotePanel.tsx`).

`NodeManager` lists nodes, fetches status, creates/deletes nodes, displays connection tokens only after the token toggle, builds a remote-agent start command from `VITE_AGENT_WS_SERVER`, and manages channel/path bindings. Its create and binding forms use unsaved-change guards (`packages/client/src/components/NodeManager.tsx`, `packages/client/src/hooks/useUnsavedChangesGuard.ts`, `packages/client/src/lib/api.ts`).

`RemotePanel` lists bindings for the current channel and opens a selected binding into `RemoteTree`. `RemoteTree` calls `remoteLs` and `remoteReadFile`, sorts directories before files, and previews files through `RemoteFileViewer`. The current remote browsing UI is read-only (`packages/client/src/components/RemotePanel.tsx`, `packages/client/src/components/RemoteTree.tsx`, `packages/client/src/components/RemoteFileViewer.tsx`, `packages/client/src/lib/api.ts`).

## Agent And Invitation Surfaces

`AgentManager` is the user-side agent owner surface. It lists agents, creates/deletes agents, fetches an agent detail for key masking/copy, rotates API keys, manages permissions, joins agents to channels, shows runtime state, and embeds `AgentConfigPanel` for config edits (`packages/client/src/components/AgentManager.tsx`, `packages/client/src/components/AgentConfigPanel.tsx`, `packages/client/src/components/RuntimeCard.tsx`, `packages/client/src/lib/api.ts`).

Agent key handling is intentionally local to the agent surface: key material is fetched on demand, copied to the clipboard, masked to last-four display, and not stored in global context (`packages/client/src/components/AgentManager.tsx`).

`InvitationsInbox` is the owner-side invitation surface. It lists owner invitations, filters by state, approves/rejects pending requests, optionally jumps to a channel after approval, and refreshes on invitation WS CustomEvents (`packages/client/src/components/InvitationsInbox.tsx`, `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/lib/api.ts`).

## Settings / Admin-Awareness Surface

The user settings surface is separate from the admin SPA. `SettingsPage` currently exposes the privacy tab and composes `PrivacyPromise`, `ImpersonateGrantSection`, and `AdminActionsList` (`packages/client/src/components/Settings/SettingsPage.tsx`, `packages/client/src/components/Settings/PrivacyPromise.tsx`, `packages/client/src/components/Settings/ImpersonateGrantSection.tsx`, `packages/client/src/components/Settings/AdminActionsList.tsx`).

`BannerImpersonate` is mounted at the top of the authenticated user app. It polls the user-owned impersonation grant every 30 seconds and lets the user revoke the grant; it does not depend on a WS frame (`packages/client/src/App.tsx`, `packages/client/src/components/Settings/BannerImpersonate.tsx`, `packages/client/src/lib/api.ts`).

The settings surface uses `/api/v1/me/admin-actions` and `/api/v1/me/impersonation-grant`. It does not use `/admin-api/v1` and does not mount admin routes (`packages/client/src/lib/api.ts`, `packages/client/src/components/Settings/SettingsPage.tsx`, `packages/client/src/admin/api.ts`, `packages/client/src/admin/AdminApp.tsx`).

## Interfaces To Other Modules

| Surface | Inputs | Outputs / calls | Evidence |
| --- | --- | --- | --- |
| Channel/DM | `AppContext` channels, DMs, current user, permissions, online users. | Select channel/DM, load messages, mark read, join/leave, subscribe WS. | `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/ChannelView.tsx`, `packages/client/src/context/AppContext.tsx` |
| Chat | Current channel, members, pending messages, WS send/ack functions. | Message send/retry, typing, uploads, mentions, slash commands. | `packages/client/src/components/MessageInput.tsx`, `packages/client/src/components/MessageList.tsx`, `packages/client/src/hooks/useWebSocket.ts` |
| Artifact | Channel ID and current user. | Artifact REST create/get/commit/rollback/iteration/comment calls; signal-triggered reload. | `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/lib/api.ts`, `packages/client/src/hooks/useWsHubFrames.ts` |
| Workspace | Channel ID or all-workspaces sidepane state. | Workspace file REST operations and viewer downloads. | `packages/client/src/components/WorkspacePanel.tsx`, `packages/client/src/components/WorkspaceManager.tsx`, `packages/client/src/lib/api.ts` |
| Remote | User remote nodes and channel bindings. | Remote node CRUD/status/binding plus read-only remote tree reads. | `packages/client/src/components/NodeManager.tsx`, `packages/client/src/components/RemotePanel.tsx`, `packages/client/src/components/RemoteTree.tsx`, `packages/client/src/lib/api.ts` |
| Settings/admin-awareness | Current user session and user-owned admin impact endpoints. | User privacy promise, grant create/revoke, grant banner, own admin actions list. | `packages/client/src/components/Settings/*`, `packages/client/src/lib/api.ts` |
