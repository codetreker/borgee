// mainView — App 主区域视图状态机 (sidepane 切换语义).
//
// 解决 #682 的核心 bug: 之前 App.tsx 用 5 个独立 boolean (showAgents /
// showInvitations / showWorkspaces / showRemoteNodes / showSettings) 管 5
// 个管理页, 点新按钮只 setX(true) 不关其它, render 用 if/else if 链显第
// 一个 true. 结果: 点 Settings 时 showAgents 还 true, 屏幕上是 Agents,
// sidebar 上 Settings active, 关掉 Agents 后 Settings 露出来 — 堆栈语义.
//
// 修法: 5 个 boolean 合并成 1 个字符串状态 (discriminated union). 只有一
// 个视图能 active. 切换 = 替换值, 自动关掉前一个. 从 5D 状态空间 (32 种
// 组合, 31 种是 bug) 缩到 6 种合法状态.
//
// 状态:
//   'channel'      默认, 显当前 channel (state.currentChannelId)
//   'agents'       My Agents 管理页
//   'invitations'  Agent 邀请收件箱
//   'workspaces'   Workspace 管理页
//   'remote-nodes' Remote nodes 管理页
//   'helper-status' Helper status visibility
//   'settings'     Settings 页
//
// 只有 'channel' 时才显 ChannelView; 其它都是 sidepane 独占主区域. 'channel'
// 是隐含默认 — 任何 sidepane 关闭按钮直接 setMainView('channel').

export type MainView =
  | 'channel'
  | 'agents'
  | 'invitations'
  | 'workspaces'
  | 'remote-nodes'
  | 'helper-status'
  | 'settings';

export const MAIN_VIEW_DEFAULT: MainView = 'channel';

export const ALL_MAIN_VIEWS: ReadonlyArray<MainView> = [
  'channel',
  'agents',
  'invitations',
  'workspaces',
  'remote-nodes',
  'helper-status',
  'settings',
];

/**
 * 是不是 sidepane (非 channel)? 用来决定要不要 hide ChannelView.
 */
export function isSidepane(view: MainView): boolean {
  return view !== 'channel';
}
