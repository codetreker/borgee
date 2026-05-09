# Client AppShell — 三栏布局 + Artifact 分级展开 (current)

> 锚: 蓝图 [`client-shape.md §1.2`](../../blueprint/current/client-shape.md) (主界面三栏 + Artifact 分级) · 实现 PR #601
> 落点: `packages/client/src/components/AppShell.tsx` + `packages/client/src/lib/use_artifact_panel.ts`
> 关联: 抽屉细节 [`artifact-drawer.md`](artifact-drawer.md) (右栏 drawer/split/fullscreen 渲染)

## 文件清单

| 文件 | 角色 |
|---|---|
| `packages/client/src/components/AppShell.tsx` | 三栏 grid container; 据 `artifactMode` 派生 `grid-template-columns`; mobile (≤768px) 降级 |
| `packages/client/src/lib/use_artifact_panel.ts` | 4-态 state machine hook (`useArtifactPanel`); transition 谓词单源 |
| `packages/client/src/components/ArtifactDrawer.tsx` | 右栏渲染容器 (mode != 'closed' 才挂 DOM); 详见 [`artifact-drawer.md`](artifact-drawer.md) |

## 三栏布局 (蓝图 §1.2 byte-identical)

```
┌──────────────────────────────────────────────────┐
│ [顶部团队栏：agent 头像 + 状态] (永久首屏感知)   │
├──────────┬──────────────────┬───────────────────┤
│  侧栏    │  主区（聊天默认）│ artifact          │
│ channel  │                  │（触发分级展开）   │
│  + DM    │                  │                   │
└──────────┴──────────────────┴───────────────────┘
```

three-pane 字面单源, 反 inline drift.

桌面 grid-template-columns (`computeGridColumns(mode, isMobile=false)`):

| `artifactMode` | grid-template-columns |
|---|---|
| `closed`     | `240px 1fr` |
| `drawer`     | `240px 1fr 380px` |
| `split`      | `240px 1fr 1fr` |
| `fullscreen` | `240px 1fr` (artifact overlay 覆盖) |

字面常量 SSOT (改 = 改一处, 反 inline 散落):

| const | 字面 | 来源 |
|---|---|---|
| `APP_SHELL_DESKTOP_SIDEBAR`    | `240` (px) | 蓝图 §1.2 |
| `APP_SHELL_DESKTOP_DRAWER`     | `380` (px) | 蓝图 §1.2 |
| `APP_SHELL_MOBILE_BREAKPOINT`  | `768` (px) | 蓝图 §1.2 |

## 4-态 state machine (蓝图 §1.2 Artifact 分级展开 byte-identical)

```
ArtifactPanelMode = 'closed' | 'drawer' | 'split' | 'fullscreen'
```

| 态 | 触发 | 渲染 |
|---|---|---|
| `closed`     | 初始 / `close()` | 右栏不渲染 (grid 仅 2 列) |
| `drawer`     | 首次点击 artifact 引用 → `open(id)` | 右侧 380px 抽屉 (轻量预览) |
| `split`      | drawer 拖拽 OR 二次点击 → `promoteToSplit()` | 主区 + artifact 各 50/50 |
| `fullscreen` | mobile (≤768px) 降级 → `setFullscreen(true)` | 全屏 modal (overlay) |

### 合法 transition 单源 (`useArtifactPanel` hook)

| transition | API | 行为 |
|---|---|---|
| `closed → drawer`     | `open(artifactId)` | 仅当 `mode==='closed'`, 切到 `drawer` + 设 artifactId |
| `drawer → split`      | `promoteToSplit()` | 仅当 `mode==='drawer'` 才升级; 返 `true` 表已 promoted |
| `split → drawer`      | `demoteToDrawer()` | 仅当 `mode==='split'` 才降级 |
| `* → closed`          | `close()` | 任意态 → closed, 清 artifactId |
| `drawer/split/fullscreen → fullscreen` | `setFullscreen(true)` | mobile 降级; closed 态保持 closed |
| `fullscreen → drawer` | `setFullscreen(false)` | mobile 退出全屏 |

**反约束** (spec §0 ②):
- `closed → split` 直接 reject (`promoteToSplit()` 在 `mode==='closed'` 时 no-op + 返 `false`) — 必先经 drawer
- 反向 grep: `SplitView.*directOpen|artifact.*autoSplit|setMode\(['"]split['"]\)` 仅命中 ArtifactDrawer drag handler 一处

### `open(artifactId)` 行为细则

| `prev.mode` | 新 `mode` | artifactId |
|---|---|---|
| `closed` | `drawer` | 设新 id |
| `drawer` / `split` / `fullscreen` | 不变 | 仅切 id (复用既有 mode, 不退栈) |

## 移动 (≤768px) 降级

`AppShell` 接 `isMobile: boolean` prop (caller 据 viewport breakpoint 算):
- 三栏 → 单栏 (`grid-template-columns: 1fr`); 主区 full width
- 侧栏 → overlay drawer; 走 `sidebarOpen` + `onSidebarClose` props 控制
- artifact split → 全屏 modal (`mode='fullscreen'`); `role="dialog" aria-modal="true"`

## DOM 锚 (改 = 改两处: 此组件 + acceptance template)

| 锚 | 来源 |
|---|---|
| `div.app-shell[data-testid="app-shell"]`              | AppShell 根 |
| `div[data-artifact-mode="closed\|drawer\|split\|fullscreen"]` | 4-态 真值 attr |
| `div[data-mobile="true\|false"]`                      | viewport 真值 |
| `div[data-testid="app-shell-sidebar"]`                | 侧栏 |
| `div[data-testid="app-shell-main"]`                   | 主区 |
| `div[data-testid="app-shell-artifact-column"]`        | drawer/split 时挂 |
| `div[data-testid="app-shell-artifact-fullscreen"]`    | fullscreen 时挂 (`role="dialog"`) |
| `div[data-testid="app-shell-sidebar-overlay"]`        | mobile sidebar overlay |

## 反向 grep 守门

| 锚 | 期望 |
|---|---|
| `useArtifactPanel` 调用 SSOT | 仅 AppShell 顶层 caller 一处 (反 hook 散落) |
| `setMode\(['"]split['"]\)` 直调 | 仅 useArtifactPanel.ts 内部一处 (caller 走 `promoteToSplit()`) |
| inline `'240px 1fr 380px'` 字面 | 仅 `computeGridColumns` 内一处 (反 inline 漂) |
| `closed → split` 直接 transition | 0 hit (反约束硬锁) |
