# CS-1 三栏 + Artifact 分级 — content lock (DOM 锚 byte-identical)

> Anchor: `docs/implementation/modules/cs-1-spec.md` v0 + `docs/qa/acceptance-templates/cs-1.md` §1+§4
> Mode: 改 = 改三处 (此 lock + AppShell.tsx + AppShell.test.tsx; ArtifactDrawer 同模式)

## §1 AppShell DOM 锚 byte-identical

| Selector | Where | Required attrs |
|---|---|---|
| `[data-testid="app-shell"]` | AppShell root | `data-artifact-mode={closed\|drawer\|split\|fullscreen}` + `data-mobile={true\|false}` |
| `[data-testid="app-shell-sidebar"]` | sidebar column | (always rendered) |
| `[data-testid="app-shell-main"]` | main column | (always rendered) |
| `[data-testid="app-shell-artifact-column"]` | desktop artifact (drawer/split) | (only when `!isMobile && (mode==='drawer' \|\| mode==='split')`) |
| `[data-testid="app-shell-artifact-fullscreen"]` | fullscreen overlay | `role="dialog"` + `aria-modal="true"` |
| `[data-testid="app-shell-sidebar-overlay"]` | mobile sidebar overlay | (only when `isMobile && sidebarOpen`) |

## §2 ArtifactDrawer DOM 锚 byte-identical

| Selector | Where | Required attrs |
|---|---|---|
| `[data-testid="artifact-drawer"]` | drawer root | `data-mode={drawer\|split\|fullscreen}` + `data-artifact-id={id}` |
| `[data-testid="artifact-drawer-close"]` | close button | `aria-label="关闭"` |
| `[data-testid="artifact-drawer-promote"]` | drawer-only promote button | `aria-label="展开"` (mode==='drawer' 才渲染) |
| `[data-testid="artifact-drawer-drag-handle"]` | drawer→split trigger | `role="separator"` + `aria-orientation="vertical"` (mode==='drawer' 才渲染) |

## §3 字面 byte-identical (3 const + 4 mode)

- `APP_SHELL_DESKTOP_SIDEBAR = 240` (px)
- `APP_SHELL_DESKTOP_DRAWER = 380` (px)
- `APP_SHELL_MOBILE_BREAKPOINT = 768` (px)
- ArtifactPanelMode = `'closed' | 'drawer' | 'split' | 'fullscreen'` (字面顺序锁)

## §4 同义词反向 reject (反"split" 直接打开)

反向 grep count==0:
- `SplitView.*directOpen`
- `artifact.*autoSplit`
- `setMode\("split"\)` 在 `useArtifactPanel.ts` 之外的源 (仅 hook + ArtifactDrawer drag handler 可触发)
- 中文同义词: `自动劈开 / 自动展开 / 直接 split` 0 hit
