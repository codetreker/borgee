# UI Map

This map describes the user SPA's surface hierarchy. It is an architectural locator, not a component inventory and not a design specification.

## Surface Hierarchy

```text
User SPA
  Auth gate
  Authenticated shell
    Global banner area
    Navigation rail
      Channel groups and channels
      Direct messages
      Global sidepane actions
    Main workspace
      Channel host
        Chat
        Canvas
        Workspace
        Remote
      Sidepanes
        Agents
        Invitations
        All workspaces
        Remote nodes
        Settings
```

## Responsibilities

The UI map explains where a maintainer should place or reason about a user-facing capability in the architecture: rail, channel host, channel tab, global sidepane, or global shell concern.

It does not specify visual design, CSS layout, copy, keyboard behavior, or exact component ownership. Those are implementation details and test concerns.

## Surface Placement Rules

| Question | Architectural answer |
| --- | --- |
| Does it affect the entire authenticated session? | Place it in the shell layer. Examples: auth state, initialization, active impersonation banner, connection wiring. |
| Does it choose where the user is working? | Place it in the navigation rail or view selector. Examples: channel selection, DM selection, sidepane buttons. |
| Is it scoped to the selected channel? | Place it under the channel host as a tab or chat capability. Examples: messages, canvas artifact, channel workspace, channel remote bindings. |
| Does it span channels but remain user-owned? | Place it as a global sidepane. Examples: all workspaces, remote nodes, agent management, invitations, settings. |
| Does it need admin authority? | It belongs outside the user SPA unless it is user-owned admin-awareness metadata. |

## Surface Map

| Surface | Layer | State owner | Data owner |
| --- | --- | --- | --- |
| Login/register | Auth gate | Local auth UI plus shell auth flags | User auth REST rail |
| Channel and DM rail | Navigation rail | Shared app state | User channel, DM, member, layout, and presence endpoints |
| Chat | Channel host | Shared message and pending-message state plus local composer state | User message REST and WS rails |
| Canvas/artifact | Channel tab | Local artifact/edit state | Artifact REST rail, refreshed by signals |
| Channel workspace | Channel tab | Local file navigation/editor state | Workspace REST rail |
| Channel remote | Channel tab | Local binding/tree/viewer state | Remote user REST rail |
| Agents | Global sidepane | Local agent/detail/config state | User agent REST rail and runtime presence |
| Invitations | Global sidepane | Local list/filter state | Agent invitation REST rail, refreshed by signals |
| All workspaces | Global sidepane | Local grouping/filter/preview state | Workspace REST rail |
| Remote nodes | Global sidepane | Local node/detail/binding state | Remote user REST rail |
| Settings | Global sidepane | Local settings tab state | User admin-awareness REST endpoints |

## Cross-Surface Signals

| Signal source | Surfaces affected | Design rule |
| --- | --- | --- |
| Channel selection | Rail, channel host, read markers | Selection is global because multiple surfaces must agree on the active channel. |
| WebSocket connection | Chat, rail, presence indicators | Connection state is global; feature-specific refresh remains local. |
| Unsaved-change guards | Sidepanes and shell navigation | Feature forms register guards; shell navigation respects them. |
| Invitation signal | Rail badge and invitation sidepane | Signal wakes both surfaces; REST remains authoritative. |
| Artifact/comment signal | Canvas and comment surfaces | Signal wakes scoped artifact surfaces; bodies are pulled. |
| Admin-awareness grant | Global banner and settings | User-owned grant state can affect the whole authenticated shell. |

## Interfaces To Other Modules

| Interface | Contract |
| --- | --- |
| App shell | Owns auth, initialization, global banner, navigation state, and active surface selection. |
| Realtime sync | Provides connection state, direct chat/presence updates, and signal wake-ups. |
| Feature surfaces | Own workflow state below the shell and call REST for authoritative data. |
| Build/PWA | Determines which entry registers service-worker behavior and which rail is installable. |
| Admin SPA | Separate application; not part of this surface hierarchy. |

## Implementation Anchors

| Concern | Anchors |
| --- | --- |
| Shell and view selector | `packages/client/src/App.tsx`, `MainView` |
| App state | `packages/client/src/context/AppContext.tsx`, `AppState` |
| Navigation rail | `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/ChannelList.tsx` |
| Channel host | `packages/client/src/components/ChannelView.tsx` |
| Chat | `packages/client/src/components/MessageList.tsx`, `packages/client/src/components/MessageInput.tsx` |
| Channel tabs | `packages/client/src/components/ArtifactPanel.tsx`, `packages/client/src/components/WorkspacePanel.tsx`, `packages/client/src/components/RemotePanel.tsx` |
| Sidepanes | `packages/client/src/components/AgentManager.tsx`, `packages/client/src/components/InvitationsInbox.tsx`, `packages/client/src/components/WorkspaceManager.tsx`, `packages/client/src/components/NodeManager.tsx`, `packages/client/src/components/Settings/SettingsPage.tsx` |
| Global signals | `packages/client/src/hooks/useWebSocket.ts`, `packages/client/src/hooks/useWsHubFrames.ts`, `packages/client/src/hooks/useUnsavedChangesGuard.ts` |
