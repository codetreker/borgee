# Sidepane mainView 状态机 (gh#682) — implementation note

> gh#682 (PR #695) — 5 个 sidepane (settings / agents / invitations / workspaces / remote-nodes) 切换从 5 boolean 合并成单 `mainView` 字符串状态机, 反 sidepane stacking bug.
> 蓝图: `client-shape.md` § sidepane.

## 1. 设计

5 个 sidepane (settings / agents / invitations / workspaces / remote-nodes) 同时只有一个能 active — 之前用 5 个独立 boolean (showSettings / showAgents / ...), 切换之间状态相互踩 (打开 settings 没关 agents → stacking bug, 显示叠 sidepane). 改成单一字符串 `mainView: MainView` 状态机, 反 stacking + 反落差 state.

反约束:
- ① 5 boolean 永不再分开存 — 单 `mainView` 字符串 state
- ② sidepane 切换前必跑 `runUnsavedGuards()` (跟 useUnsavedChangesGuard 联动)
- ③ 反 react-router (一份 SPA 跨 sidepane 状态切, 不挂 URL)

## 2. State 设计 (`packages/client/src/lib/mainView.ts`)

```ts
export type MainView = 'channel' | 'settings' | 'agents' | 'invitations' | 'workspaces' | 'remote-nodes';

// channel = 默认主视图; 其它 5 个值是 sidepane 视图
```

App 顶层:
```tsx
const [mainView, setMainView] = useState<MainView>('channel');

function requestMainView(target: MainView) {
  // 反约束: 切换前跑守卫, 用户决定要不要丢未保存改动
  if (!runUnsavedGuards()) return;
  setMainView(target);
}

function closeAllViews() {
  setMainView('channel');
}
```

## 3. 跟 useUnsavedChangesGuard 联动

`requestMainView()` 在 `setMainView()` 前跑 `runUnsavedGuards()`:
- 任一注册的 form 守卫 isDirty=true → window.confirm 弹消息
- 用户 OK → 继续 setMainView (丢改动, 用户选了)
- 用户 Cancel → return, 停在原 sidepane

5 form 自动获益 (useUnsavedChangesGuard hook 注册到 module-level guards Set, requestMainView 跑全套).

详见 [`../hooks/useUnsavedChangesGuard.md`](../hooks/useUnsavedChangesGuard.md).

## 4. 入口

| 入口 | 动作 |
|---|---|
| Sidebar 底部 ⚙️ 按钮 (`data-action="open-settings"`) | `requestMainView('settings')` |
| Sidebar 顶部 "Agents" 链接 | `requestMainView('agents')` |
| Sidebar 顶部 "Invitations" 链接 | `requestMainView('invitations')` |
| Sidebar 顶部 "Workspaces" 链接 | `requestMainView('workspaces')` |
| Sidebar 顶部 "Remote Nodes" 链接 | `requestMainView('remote-nodes')` |
| sidepane Back 按钮 | `setMainView('channel')` (closeAllViews) |
| ESC 关 sidepane (沿用) | `setMainView('channel')` |

## 5. 渲染

```tsx
{mainView === 'channel' && <ChannelView ... />}
{mainView === 'settings' && <SettingsPage onBack={closeAllViews} />}
{mainView === 'agents' && <AgentManager onBack={closeAllViews} />}
{mainView === 'invitations' && <InvitationsPage onBack={closeAllViews} />}
{mainView === 'workspaces' && <WorkspacesPage onBack={closeAllViews} />}
{mainView === 'remote-nodes' && <NodeManager onBack={closeAllViews} />}
```

同时只有一个能 active, 反堆栈.

## 6. 测试

- `packages/client/src/__tests__/main-view.test.tsx` (≥6 case): mainView 默认 'channel' / requestMainView 切换 / runUnsavedGuards 拦截 / closeAllViews 回 'channel' / 反 5 boolean 同时 true / sidepane 切换 ESC 关
- e2e: `gh-682-sidepane-mainview.spec.ts` — 真 UI 切 sidepane + dirty form 拦截 + Back 按钮回主视图

## 7. 锚

- 蓝图: `client-shape.md` § sidepane
- spec: 无单独 spec (gh#682 直接 PR)
- PR: #695 (Closes gh#682 + gh#682-sidepane stacking)
