# Client AppShell — three-pane layout + Artifact staged expansion (current)

> Source: blueprint [`client-shape.md §1.2`](../../blueprint/current/client-shape.md) (main three-pane UI + Artifact staged expansion) · implementation PR #601
> Implementation: `packages/client/src/components/AppShell.tsx` + `packages/client/src/lib/use_artifact_panel.ts`
> Related: drawer details in [`artifact-drawer.md`](artifact-drawer.md) (right-pane drawer/split/fullscreen rendering)

## File List

| File | Role |
|---|---|
| `packages/client/src/components/AppShell.tsx` | Three-pane grid layout container; derives `grid-template-columns` from `artifactMode`; mobile fallback at ≤768px |
| `packages/client/src/lib/use_artifact_panel.ts` | 4-state state-machine hook (`useArtifactPanel`); single source for transition predicates |
| `packages/client/src/components/ArtifactDrawer.tsx` | Right-pane render container (mounts DOM only when mode != 'closed'); see [`artifact-drawer.md`](artifact-drawer.md) |

## Three-Pane Layout (blueprint §1.2 byte-identical)

```
┌──────────────────────────────────────────────────┐
│ [顶部团队栏：agent 头像 + 状态] (永久首屏感知)   │
├──────────┬──────────────────┬───────────────────┤
│  侧栏    │  主区（聊天默认）│ artifact          │
│ channel  │                  │（触发分级展开）   │
│  + DM    │                  │                   │
└──────────┴──────────────────┴───────────────────┘
```

The three-pane literal is the single source, avoiding inconsistent scattered literal copy.

Desktop grid-template-columns (`computeGridColumns(mode, isMobile=false)`):

| `artifactMode` | grid-template-columns |
|---|---|
| `closed`     | `240px 1fr` |
| `drawer`     | `240px 1fr 380px` |
| `split`      | `240px 1fr 1fr` |
| `fullscreen` | `240px 1fr` (artifact overlay covers it) |

Single source for literal constants (change one place, avoiding scattered literal values):

| const | Literal | Source |
|---|---|---|
| `APP_SHELL_DESKTOP_SIDEBAR`    | `240` (px) | blueprint §1.2 |
| `APP_SHELL_DESKTOP_DRAWER`     | `380` (px) | blueprint §1.2 |
| `APP_SHELL_MOBILE_BREAKPOINT`  | `768` (px) | blueprint §1.2 |

## 4-State State Machine (blueprint §1.2 Artifact staged expansion byte-identical)

```
ArtifactPanelMode = 'closed' | 'drawer' | 'split' | 'fullscreen'
```

| State | Trigger | Rendering |
|---|---|---|
| `closed`     | initial / `close()` | right pane does not render (grid has only 2 columns) |
| `drawer`     | first click on artifact reference → `open(id)` | right-side 380px drawer (lightweight preview) |
| `split`      | drawer drag OR second click → `promoteToSplit()` | main area + artifact are each 50/50 |
| `fullscreen` | mobile fallback (≤768px) → `setFullscreen(true)` | fullscreen modal (overlay) |

### Legal Transition Single Source (`useArtifactPanel` hook)

| transition | API | Behavior |
|---|---|---|
| `closed → drawer`     | `open(artifactId)` | only when `mode==='closed'`; switch to `drawer` + set artifactId |
| `drawer → split`      | `promoteToSplit()` | only upgrades when `mode==='drawer'`; returns `true` when upgraded |
| `split → drawer`      | `demoteToDrawer()` | only demotes when `mode==='split'` |
| `* → closed`          | `close()` | any state → closed, clear artifactId |
| `drawer/split/fullscreen → fullscreen` | `setFullscreen(true)` | mobile fallback; closed state remains closed |
| `fullscreen → drawer` | `setFullscreen(false)` | mobile exits fullscreen |

**Reverse constraint** (spec §0 ②):
- Direct `closed → split` is rejected (`promoteToSplit()` is a no-op and returns `false` when `mode==='closed'`) — must pass through drawer first
- Grep check: `SplitView.*directOpen|artifact.*autoSplit|setMode\(['"]split['"]\)` only matches the ArtifactDrawer drag handler

### `open(artifactId)` Behavior Details

| `prev.mode` | New `mode` | artifactId |
|---|---|---|
| `closed` | `drawer` | set new id |
| `drawer` / `split` / `fullscreen` | unchanged | only switch id (reuse existing mode, do not pop state) |

## Mobile Fallback (≤768px)

`AppShell` accepts `isMobile: boolean` prop (calling component calculates it from viewport breakpoint):
- three-pane → single-pane (`grid-template-columns: 1fr`); main area is full width
- sidebar → overlay drawer; controlled by `sidebarOpen` + `onSidebarClose` props
- artifact split → fullscreen modal (`mode='fullscreen'`); `role="dialog" aria-modal="true"`

## DOM Sources (change two places: this component + acceptance template)

| Source | Origin |
|---|---|
| `div.app-shell[data-testid="app-shell"]`              | AppShell root |
| `div[data-artifact-mode="closed\|drawer\|split\|fullscreen"]` | real 4-state attribute value |
| `div[data-mobile="true\|false"]`                      | real viewport value |
| `div[data-testid="app-shell-sidebar"]`                | sidebar |
| `div[data-testid="app-shell-main"]`                   | main area |
| `div[data-testid="app-shell-artifact-column"]`        | mounted during drawer/split |
| `div[data-testid="app-shell-artifact-fullscreen"]`    | mounted during fullscreen (`role="dialog"`) |
| `div[data-testid="app-shell-sidebar-overlay"]`        | mobile sidebar overlay |

## Grep Checks

| Source | Expected |
|---|---|
| `useArtifactPanel` call single source | only one top-level AppShell calling component (avoid scattered hook calls) |
| direct `setMode\(['"]split['"]\)` call | only one internal useArtifactPanel.ts call (caller uses `promoteToSplit()`) |
| inline `'240px 1fr 380px'` literal | only one occurrence inside `computeGridColumns` (avoid inconsistent scattered literal values) |
| direct `closed → split` transition | 0 hits (hard reverse constraint) |
