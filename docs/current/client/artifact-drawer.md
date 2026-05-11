# Client ArtifactDrawer — 右侧 drawer / split / fullscreen 渲染 (current)

> 出处: 蓝图 [`client-shape.md §1.2`](../../blueprint/current/client-shape.md) Artifact 分级展开 · 实现 PR #601
> 落点: `packages/client/src/components/ArtifactDrawer.tsx`
> 关联: 4-态 state machine + grid 布局 [`app-shell.md`](app-shell.md)

## 文件清单

| 文件 | 角色 |
|---|---|
| `packages/client/src/components/ArtifactDrawer.tsx` | 右侧容器；`mode='closed'` 时返回 `null`，不挂载 DOM；header 包含 promote / close 按钮；drawer 模式额外挂载 drag handle |

## Artifact 分级展开 (与蓝图 §1.2 保持一致)

Artifact 引用遵循两级展开，避免首次点击就进入 split view:

| 操作 | 行为 |
|---|---|
| 首次点击 artifact 引用 | 展开右侧**抽屉**，用于轻量预览 — `mode='drawer'` |
| 显式动作 (拖拽 / 二次点击 promote 按钮) | 升级为 **split view**，聊天与 artifact 并存 — `mode='split'` |

`ArtifactDrawer` 仅渲染 `mode ∈ {drawer, split, fullscreen}`。`closed` 时 React 返回 `null`，AppShell grid 同步收缩到 2 栏。

## Props 契约

| prop | 类型 | 约束 |
|---|---|---|
| `mode`             | `'drawer' \| 'split' \| 'fullscreen' \| 'closed'` | 当前 iteration 状态；`closed` 时不挂载 DOM |
| `artifactId`       | `string \| null` | 当前 artifact id；写入 `data-artifact-id`，供 e2e/QA 定位 |
| `onClose`          | `() => void` | close button → `useArtifactPanel.close()` |
| `onPromoteToSplit` | `() => void` | drawer 升级 split；由按钮或 drag handle 触发 |
| `children`         | `ReactNode` | 复用既有 ArtifactPanel 内容；CS-1 不改内部渲染 |

## DOM 出处 (改 = 改两处: 此组件 + acceptance template)

| 出处 | 触发条件 | 备注 |
|---|---|---|
| `div[data-testid="artifact-drawer"]`              | mode != closed | 根容器 |
| `div[data-mode="drawer\|split\|fullscreen"]`      | 同上 | mode 真实值 attr |
| `div[data-artifact-id="<ulid>"]`                  | 同上 | 当前 artifact id；空字符串表示 null |
| `button[data-testid="artifact-drawer-promote"]`   | 仅 mode='drawer' | aria-label `"展开"`, 字面 `⇔` |
| `button[data-testid="artifact-drawer-close"]`     | mode != closed | aria-label `"关闭"`, 字面 `×` |
| `div[data-testid="artifact-drawer-drag-handle"]`  | 仅 mode='drawer' | `role="separator" aria-orientation="vertical"`; mouseUp → onPromoteToSplit |

## drawer → split transition 三入口

| 入口 | DOM | 调用 |
|---|---|---|
| 显式按钮 | `button[data-testid="artifact-drawer-promote"]` | `onPromoteToSplit()` |
| drag handle | `div[data-testid="artifact-drawer-drag-handle"]` mouseUp | `onPromoteToSplit()` |
| (二次点击 artifact 引用) | 来自 caller 上层 ArtifactReference | `onPromoteToSplit()` |

三个入口都调用同一个 `onPromoteToSplit` callback，并由 `useArtifactPanel.promoteToSplit()` 统一处理 transition。

| 禁止行为 | 依据 |
|---|---|
| `closed → split` 不可由这三个入口直接到达 | `closed` 态下 ArtifactDrawer 不挂载 DOM，promote 按钮和 drag handle 都不存在 |

## QA / grep 检查

| 出处 | 期望 |
|---|---|
| `mode === 'closed'` 时仍挂载 DOM | 无匹配；`mode==='closed'` 直接 `return null` |
| `setMode\(['"]split['"]\)` 直调 | 无匹配；caller 必须走 `onPromoteToSplit` callback |
| drag handle 字面 `cursor` style 脱节 | 仅在 `artifact-drawer-drag-handle` className 一处 |

## 不在范围 (留尾)

- artifact body 渲染；继续走 `children` ArtifactPanel，CS-1 不改
- mobile drag handle 触摸事件支持；现仅 mouse，touch 走 fullscreen 直 modal
- drawer ↔ split 中间态；拖拽过程中宽度 % 锁定不在当前实现，现为一次性切换，留 v2 微交互
