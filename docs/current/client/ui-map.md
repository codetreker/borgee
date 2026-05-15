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
        Primary footer: avatar/account identity, Agents, Workspaces, Settings
        Secondary footer overflow: Invitations, Remote nodes, Helper status, Logout
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
        Helper status
        Settings
```

## Responsibilities

The UI map explains where a maintainer should place or reason about a user-facing capability in the architecture: rail, channel host, channel tab, global sidepane, or global shell concern.

It does not specify visual design, CSS layout, copy, keyboard behavior, or exact component ownership. Those are implementation details and test concerns. The [ui/](ui/) directory keeps representative ASCII interaction sketches as reference/layout sketches.

The sketches are an Interaction And Layout Reference. They help maintainers recognize surface shape and navigation flow, but they do not define product behavior, verification status, or design-system rules.

## Surface Placement Rules

| Question | Architectural answer |
| --- | --- |
| Does it affect the entire authenticated session? | Place it in the shell layer. Examples: auth state, initialization, active impersonation banner, connection wiring. |
| Does it choose where the user is working? | Place it in the navigation rail or view selector. Examples: channel selection, DM selection, sidepane buttons. |
| Is it scoped to the selected channel? | Place it under the channel host as a tab or chat capability. Examples: messages, canvas artifact, channel workspace, channel remote bindings. |
| Does it span channels but remain user-owned? | Place it as a global sidepane. Examples: all workspaces, remote nodes, Helper status, agent management, invitations, settings. |
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
| Helper status | Global sidepane | Local enrollment list/detail/refresh state | User Helper enrollment REST rail |
| Settings | Global sidepane | Local settings tab state | User admin-awareness REST endpoints |

## Sketch Coverage

| Surface | Sketch reference | Notes |
| --- | --- | --- |
| Auth gate | [ui/login.md](ui/login.md) | Login/register placement reference only. |
| Shell and channel host | [ui/main-desktop.md](ui/main-desktop.md), [ui/main-mobile.md](ui/main-mobile.md) | Desktop and mobile workspace layout references. |
| Channel rail | [ui/channel-sort-groups.md](ui/channel-sort-groups.md) | Channel grouping/sorting interaction reference. |
| Chat and DM | [ui/message.md](ui/message.md), [ui/dm.md](ui/dm.md), [ui/slash-commands.md](ui/slash-commands.md) | Message shape, DM shape, and command-panel references. |
| Public preview | [ui/preview.md](ui/preview.md) | Read-only preview and join prompt reference. |
| Canvas/artifact | [ui/canvas-modal.md](ui/canvas-modal.md) | In-app decision flow reference for canvas actions. |
| Workspace | [ui/workspace.md](ui/workspace.md) | File tree and viewer reference. |
| Agents | [ui/agent-manager.md](ui/agent-manager.md), [ui/agent-config.md](ui/agent-config.md), [ui/agent-collab.md](ui/agent-collab.md) | Owner-side agent management and collaboration references. |
| Sidepanes and settings | [ui/sidepane.md](ui/sidepane.md), [ui/settings.md](ui/settings.md) | Sidepane switching, Helper status placement, and admin-awareness references. |
| Remote surfaces | [../remote-agent/ui/README.md](../remote-agent/ui/README.md) | Combined Remote Explorer reference sketch; current client architecture splits remote nodes from channel remote browsing. |

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
| Surface hosts | `packages/client/src/components/Sidebar.tsx`, `packages/client/src/components/ChannelView.tsx` |
| Feature surfaces | `packages/client/src/components/`, `packages/client/src/components/Settings/` |
| Global hooks | `packages/client/src/hooks/` |
