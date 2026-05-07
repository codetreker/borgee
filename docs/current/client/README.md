# client — React SPA 设计

代码位置：`/workspace/borgee/packages/client/`

## 1. 构建与入口

- 包名 `@borgee/client`，纯 ESM，React 18 + TypeScript 5.7 + Vite 6 + Vitest 4。
- 关键运行时依赖：`react-router-dom@7`（仅 admin 用）、`@tiptap/react` + `tiptap-markdown`（消息编辑器）、`@dnd-kit/*`（频道拖拽排序）、`emoji-mart`、`marked` + `highlight.js` + `dompurify`（消息渲染）。
- **双 SPA 单构建**：`vite.config.ts` 配置两个 Rollup 入口
  - `index.html` → `src/main.tsx` → `<App/>`（用户端）
  - `admin.html` → `src/admin/main.tsx` → `<AdminApp/>`（admin 端，用 React Router）
- Dev 时 `/api`、`/admin-api`、`/ws`、`/uploads` 全部 proxy 到 `localhost:4900`。
- `main.tsx` 在 `load` 事件后注册 `/sw.js`（PWA service worker）。

## 2. 顶层结构 (`src/App.tsx`)

```
<ThemeProvider>            # 浅/深色，localStorage 持久化
  <AppProvider>            # useReducer 中央 store
    <ToastProvider>
      <AppInner/>          # 真正的 layout
```

**没有 React Router**——用户端通过 `AppContext.currentChannelId` 这个状态字段做"路由"。`AppInner` 启动时：

1. `useEffect` 里调一次 `fetchMe()` 识别登录态（`waitForAuthReady` 的 500ms 轮询只在登录表单提交后用，不是启动）。
2. 串行加载 user / permissions / channels / online users → `SET_INITIALIZED`。
3. 每 30s `loadOnlineUsers()`。
4. 没选频道时优先选 `type='system'` (#welcome / CM-onboarding)，没有再 fallback 到 `channels[0]`。空列表时主面板显示降级文案 "正在准备你的工作区, 稍候刷新…" + [重试] 按钮（触发 `loadChannels()`），不再渲染老的 "👈 选择一个频道开始聊天"。
5. 已加入的频道挨个 `useWebSocket().subscribe()`。
6. 渲染 `<Sidebar/>` + 当前主面板（`AgentManager` / `InvitationsInbox` / `WorkspaceManager` / `NodeManager` / `ChannelView` / 启动屏）。
7. 768px 以下走移动端布局，hamburger + overlay。

## 3. 目录职责

### `context/`
- `AppContext.tsx` — `useReducer` 中央 store，承载 `channels`/`groups`/`dmChannels`/`currentChannelId`、`messages: Map<channelId, Message[]>`、`hasMore`、`loadingMessages`、`currentUser`、`permissions`、`onlineUserIds`、`connectionState`、`typingUsers`、`pendingMessages`、`channelMembersVersion`、`initialized`。reducer 处理 38 个 action。
  - 通过两个 ref 注入 WS 能力：`sendWsMessageRef`、`registerAckTimerRef`。`AppInner` 在 mount 后从 `useWebSocket()` 拿到函数，再调 context 的 `setSendWsMessage` / `setRegisterAckTimer` 写入 ref，避免循环依赖。
- `ThemeContext.tsx` — 主题切换。

### `lib/`
- `api.ts` — 全部 REST 调用集中在这里。`request<T>()` 用 `fetch` + `credentials: 'include'`（cookie auth）；dev 时塞 `X-Dev-User-Id` 头；非 2xx 抛 `ApiError`。`Agent` interface 含 `state` / `reason` / `state_updated_at` (AL-1a Phase 2 三态 online/offline/error + AL-1b #453 Phase 4 解封 busy/idle 两态 = 5-state) + `last_task_id` / `last_task_started_at` / `last_task_finished_at` (AL-1b BPP frame busy/idle 态时 server 填的 task 元数据).
- `markdown.ts` — CV-1.3 artifact 渲染复用同 `marked + DOMPurify` 管线 (立场 ④ Markdown ONLY); ArtifactPanel 直接 `dangerouslySetInnerHTML={{ __html: renderMarkdown(body) }}`, 不接 HTML 直插 / 不接 type 切换。
- `api.ts` (CV-1.3) — Artifact 5 endpoints: `createArtifact(channelId, {title, body})` / `getArtifact(id)` / `listArtifactVersions(id)` / `commitArtifact(id, {expected_version, body})` / `rollbackArtifact(id, toVersion)`. 类型 `Artifact` / `ArtifactVersion` (含 `committer_kind: 'agent'|'human'`, `rolled_back_from_version?`) / `CommitArtifactResponse` / `RollbackArtifactResponse`. 409 全部抛 `ApiError`, 调用方自决文案 (ArtifactPanel 锁 `'内容已更新, 请刷新查看'`)。
- `agent-state.ts` (AL-1a + AL-1b) — `describeAgentState(state, reason)` 把 server 下发的 `state` + `reason` 折成 `{text, tone}`；AL-1a 三态 `online`/`offline`/`error` (`REASON_LABELS` 锁 6 reason — api_key_invalid/quota_exceeded/network_unreachable/runtime_crashed/runtime_timeout/unknown); AL-1b (#453, Phase 4 解封) 加 `busy → "在工作"` tone='ok' / `idle → "空闲"` tone='muted' (acceptance al-1b.md §3.1+§3.2 byte-identical, 反约束 §3.4 grep 反 "活跃"/"running"/"Standing by"/"等待中" 模糊词). 详见 `docs/current/server/agent-runtime-state.md` wire schema + `docs/current/server/data-model.md` agent_status v=21 表 + GET /api/v1/agents/:id/status 5-state 合并 (error > busy > idle > online > offline).
- `markdown.ts`、`file-links.ts` — `marked + highlight.js + dompurify` 渲染。

### `hooks/usePresence.ts` (AL-3.3)
- `markPresence(agentID, state, reason)` — WS `presence.changed` frame 入口；cache 总是写最新值，**通知 (UI 重渲染) 5s 节流**: 距上次 notify ≥ 5s 立即派发，窗口内 burst 安排 trailing flush（同 server §2.4 PresenceChange5sCoalesce 锁）。
- `usePresence(agentID)` — React hook，订阅指定 agent 的实时 cached state；返回 `undefined` 时 `<PresenceDot/>` 走 `describeAgentState(undefined,...)` 兜底为 `已离线` (野马 §11)。
- `__resetPresenceStoreForTest(now)` / `flushPendingForTest()` — 仅单测用；`presence.test.ts` 注入 fake clock 推进时间线，不依赖 wall time。
- 反约束: cache 仅 `{state, reason, updatedAt}` 三元组，不存 IP / 心跳 / 连接数（acceptance §2.5 frame 字段白名单）。

### `components/PresenceDot.tsx` (AL-3.3 + AL-1b)
- AL-3 三态 DOM 字面锁（acceptance §3.1）：`data-presence="online"` 绿点 + `在线`、`data-presence="offline"` 灰点 + `已离线`、`data-presence="error"` 红点 + `故障 (REASON_LABEL)`。
- AL-1b (#453, Phase 4) 扩 busy/idle 两态 — `data-task-state` 槽位独立 attr (busy/idle 时填字面, 其他态填空 string); `presence-task-busy` / `presence-task-idle` CSS class; busy/idle 时 `data-presence` 仍为 'online' (busy/idle = 连着, 跟 AL-3 hub session 同源 — task-state 是独立维度); 文案 `在工作` / `空闲` (acceptance al-1b.md §3.1+§3.2 byte-identical).
- 反约束 §5.4：`.presence-dot` 永远跟 sibling 文本（或 compact 模式下 sr-only + title），不出现裸灰点。
- 反约束 §5.1 (AL-3 Phase 2 旧条) Phase 4 解封 — busy/idle 字面 AL-1b 合法 describeAgentState() 输出 + PresenceDot data-task-state attr 字面; 仅守 server-side enum 名 leak (StateBusy / StateIdle Go 字面) 不漂入 client (presence-reverse-grep.test.ts §5.1 修).
- 反约束 §3.4 (AL-1b) — agent-state.ts 不出现 "活跃"/"running"/"Standing by"/"等待中" 模糊词 (presence-reverse-grep.test.ts §3.4 反查锚 + PresenceDot.test.tsx 双层闸).
- 反约束 §3.2：组件不判 role；调用方仅在 agent 行渲染（`Sidebar.tsx` `DmItem` 用 `peer.role === 'agent'` gate；`ChannelMembersModal.tsx` 用 `m.role === 'agent'` gate；row 写 `data-role` 属性供 e2e 反查 `[data-role="user"][data-presence]` count==0）。

### `hooks/useWsHubFrames.ts` (RT-0 / CV-1.2-client)
- `dispatchInvitationPending` / `dispatchInvitationDecided` + `useInvitationFrames({onPending,onDecided})` — RT-0 邀请 push → CustomEvent (`borgee:invitation-pending` / `borgee:invitation-decided`) → InvitationsInbox / Sidebar 铃铛 listener。
- `dispatchArtifactUpdated(frame)` + `useArtifactUpdated(handler)` (CV-1.3) — `useWebSocket.ts` 的 `case 'artifact_updated'` 调 dispatch, 派发 `borgee:artifact-updated` CustomEvent (字面锁, 见 `__tests__/ws-artifact-updated.test.ts`); ArtifactPanel 用 hook 订阅, handler 自决是否 re-fetch。立场 ⑤: 7-field envelope `{type, cursor, artifact_id, version, channel_id, updated_at, kind}` 仅信号, **不**带 body / committer (反向断言已锁在单测)。

### `hooks/`
- `useWebSocket.ts` — 单连接 `/ws`，关键行为：
  - 重连退避 `[1s, 2s, 4s, 8s, 16s, 30s]`。
  - 每 25s `ping`。
  - 重连后重新 `subscribe` 所有频道，并对每个频道用最后已知时间戳调 `fetchMessages({after: lastTs})` 拉漏掉的消息。
  - **RT-1.2 (#290 follow)** — 重连时还会调 `fetchEventsBackfill(last_seen_cursor)` (`GET /api/v1/events?since=N`) 拉断线期间的 event 缺洞，按 server 单调 cursor 排序透传给 `handleMessage`。`onmessage` 入口先把 frame 上的 `cursor`（RT-1.1 `ArtifactUpdatedFrame` 起始）持久化到 `lib/lastSeenCursor.ts` (sessionStorage `borgee.rt1.last_seen_cursor`)，再 dispatch handler；持久化函数 `persistLastSeenCursor` 单调（小值 / NaN / 负数 / Infinity 全 no-op），page reload 后 `loadLastSeenCursor` 恢复。**反约束**: cold start (`since=0`) 不触发 backfill — 不拉全 history（与 RT-1.3 agent `session.resume{full}` 区别）；事件**不**按 `updated_at` / `created_at` 排序，cursor 即顺序。
  - 把所有服务端 push 类型 dispatch 到 `AppContext`。
- `useSlashCommands.ts` — 跟踪 editor 文本，前缀 `/` + 无空格时激活；委托 `commandRegistry.search(prefix)` 出选项；管理键盘导航。
- `useCommandTracking.ts` — 监听自定义事件 `commands_updated` 重新拉远端命令。
- `useMention.ts`、`usePermissions.ts`、`useLongPress.ts`、`useVisualViewport.ts` — 小工具 hook。

### `components/`
- `Sidebar.tsx`、`ChannelList.tsx`（`@dnd-kit` 拖拽 → `api.reorderChannel`）。
  - **CHN-1.3 (#265 拆段 3/3)** — 创建对话框默认 `visibility=public` + 不预选成员（creator-only，配合 server CHN-1.2 立场 ①）；`SortableChannelItem` 根据 `Channel.archived_at` 显示 `📦` + `已归档` badge + `channel-item-archived` 类（灰显 + 删除线）；`ChannelMembersModal` 危险区域新增 归档/恢复 按钮（PATCH `archived: true|false`，server 标 timestamp + 系统 DM "channel #{name} 已被 ... 关闭于 ..."）；agent member 行额外渲染 `🔕 silent` badge（CHN-1.2 schema `channel_members.silent=true`）。
- `ChannelView.tsx` — 频道主区，组合 `MessageList` + `MessageInput` + `TypingIndicator` + 工具栏。
- `MessageList.tsx` — 合并 `messages + pendingMessages` 渲染；scroll 到顶触发 `loadOlderMessages`；新消息自动 scroll 到底。
- `MessageItem.tsx` — 单条消息：avatar、displayName、时间、`marked + dompurify` markdown、edit/delete、`<ReactionBar/>`。`sender_id==='system'` 走简化分支（无头像）；若 `message.quick_action` 为 `{kind:"button",label,action}` JSON，渲染按钮，点击 `window.dispatchEvent(new CustomEvent('borgee:quick-action',{detail:{action}}))`。`App.tsx` 监听该事件：`open_agent_manager` → `setShowAgents(true)`（CM-onboarding）。
- `MessageInput.tsx` — TipTap 编辑器（`StarterKit + Markdown + MentionExtension`），Enter 发送、Ctrl+Enter 换行、文件拖放、图片粘贴、emoji 选择器、mention 选择器、slash command 选择器。
- `ArtifactPanel.tsx` (CV-1.3) — channel 维度 Markdown artifact 协作面板 (Canvas tab)。立场反查在文件头注释 (① 归属=channel / ② 单文档锁 30s TTL → 409 toast 文案锁 `'内容已更新, 请刷新查看'` / ③ 版本线性 asc, rollback 也是新增 row / ④ Markdown ONLY 走 `renderMarkdown` / ⑤ frame 仅信号, body 走 GET pull / ⑥ committer_kind 决定 🤖/👤 badge / ⑦ rollback 按钮 owner-only — `channel.created_by===currentUser.id`)。状态机: 空 → `handleCreate` (`window.prompt` 标题) → 拿到 head + version list → 渲染。`handleSubmit` 走 `commitArtifact({expected_version, body})`, 409 → `showToast(CONFLICT_TOAST)` + `reload()` 让 expected_version 前进。`handleRollback(toVersion)` confirm 后调 `rollbackArtifact`, 同样 409 共用 toast 文案。WS push 接入: `useArtifactUpdated((frame)=>{ if(frame.channel_id!==channelId) return; if(frame.artifact_id!==artifact?.id) return; void reload(artifact.id) })` — 立场 ⑤ pull-after-signal, 不消费 frame 的 body/committer (envelope 里也没有)。反约束: 不上 CRDT, 不自造 envelope, 不用 client timestamp 排序。
- `ReactionBar.tsx`、`SlashCommandPicker.tsx`、`AgentManager.tsx`、`InvitationsInbox.tsx`、`WorkspaceManager.tsx`、`NodeManager.tsx`、`ConnectionStatus.tsx`、`Toast.tsx`、`TypingIndicator.tsx`。
  - `InvitationsInbox.tsx`（CM-4.2）— 业主侧 agent 邀请收件箱：`listAgentInvitations('owner')` 拉列表，pending 行带 同意/拒绝 quick action（PATCH `/api/v1/agent_invitations/{id}` `{state}`），同意成功后 `actions.loadChannels()` 然后 `onJumpToChannel(channel_id)` 切到目标频道；409 → "该邀请已被处理或状态已变更，请刷新"。`Sidebar` 右下 🔔 铃铛每 60s 轮询 owner-role 邀请数（agent 角色跳过），CM-4.3 会替换成 BPP push frame。Bug-029 后渲染 `agent_name` / `channel_name`（前缀 `#`）/ `requester_name`，server-resolved label 缺失则 fallback 到 raw id；raw UUID 始终保留在 `title` hover 上（debug / log 引用）。`AgentInvitation` 类型见 `lib/api.ts`：`agent_name?` / `channel_name?` / `requester_name?` 三字段 optional（向后兼容旧 server）。

### `components/Settings/`
- 用户设置页，**v1 仅 "隐私" tab**（ADM-1 起步, Phase 4 启动 milestone）。详见 [`ui/settings.md`](ui/settings.md)。
- `SettingsPage.tsx` — 1 page 骨架, 顶部嵌 ⚙️ 按钮（Sidebar `data-action="open-settings"` → `App.tsx::requestMainView('settings')`，跟 agents / invitations / workspaces / remote-nodes 共用 `mainView: MainView` 字符串状态切视图，无 react-router；切换前跑 `useUnsavedChangesGuard` 守卫防丢改动）。
- `PrivacyPromise.tsx` — 三承诺字面 1:1 跟 `admin-model.md §4.1` 同源（drift test CI 拦, vite `?raw` import）+ 八行 ✅/❌ 表格三色锁（allow gray / deny `#d33` 加粗 / impersonate `#d97706` amber）。**默认展开不可折叠**（野马 R3, 反 `<details>` 包裹源码 0 hit）。
- 路径分叉：跟 `admin/pages/SettingsPage.tsx` 同名共存不混用（ADM-0 红线: cookie 拆 + `/admin-api/*` 独立 route）。

### `extensions/`
- `mention.ts` — 包装 TipTap 的 mention 扩展，suggestion 用 `<MentionList/>` 渲染。`MessageInput` 发送前用 `extractMentionIds()` 把 mention node 的 `id` 收集出来传给 server。

### `commands/`
- `registry.ts` — 单例 `CommandRegistry`：内置命令 `Map<name, CommandDefinition>`，远端命令 `RemoteCommand[]`。`resolve(name)` 返回 `builtin / remote / ambiguous / null`；`search(prefix)` 输出按 group 分类。
- `builtins.ts` — 内置 8 个 slash command：`/help /leave /topic /invite /dm /status /clear /nick`。每个 `execute(ctx)` 拿到 `{channelId, currentUserId, args, dispatch, api, actions}`。

### `admin/`
- 独立 SPA，用 React Router v7。`useAdminAuth()` 处理 `/admin-api/v1/auth/*` 的 cookie session。
- 页面：`DashboardPage`（统计）、`UsersPage` + `UserDetailPage`（账号、权限、API key）、`ChannelsPage`、`InvitesPage`、`SettingsPage`、**`AdminAuditLogPage`** (ADM-2 PR #484, `GET /admin-api/v1/audit-log` 全 admin 互可见 audit log + ?actor_id/?action/?target_user_id 三 filter; 跟 user-rail 中文动词字面**拆死**, admin SPA 用英文 `action` enum, stance §6 反向 lint 锁)。
- `admin/api.ts` 镜像 `lib/api.ts`，base URL 为 `/admin-api/v1`。

> ADM-2 client (PR #484) 详见 [`ui/settings.md §7`](ui/settings.md) — user 设置页 ADM-2 三段 (PrivacyPromise + ImpersonateGrantSection + AdminActionsList) + App-level `BannerImpersonate.tsx` 顶部红横幅。

## 4. 与 server 的通信

- **REST**：同源（dev 走 vite proxy），cookie 即 auth。
- **WebSocket**：单连 `/ws`，事件类型见 [`server` §6](../server/README.md#6-realtime)。
- **乐观发送**（`MessageInput.tsx`）：
  1. `dispatch(ADD_PENDING_MESSAGE)` 生成 `client_message_id`（`crypto.randomUUID()`）。
  2. WS 发 `{type:'chat_message', client_message_id, channel_id, content}`。
  3. `registerAckTimer` 起 10s 计时器；超时 → `dispatch(FAIL_PENDING_MESSAGE)` 把这条消息标记为发送失败（**不会**自动 fallback 到 REST，由用户手动重试）。
  4. `message_ack` → `ACK_PENDING_MESSAGE`，把 pending 替换为已确认行。
  5. `message_nack` → `FAIL_PENDING_MESSAGE`。
- **文件上传**：`api.uploadImage(file)` → `POST /api/v1/upload`，回传 URL，作为 markdown image 嵌入消息内容，`content_type: 'image'`。

## 5. 状态模型

- 全部状态在 `AppContext` 的 `useReducer` 里，**没有 Redux/Zustand/Recoil**。
- 消息按 channel 缓存：`Map<channelId, Message[]>`，进入频道时拉最近 100，向上翻 `PREPEND_MESSAGES` 50 条；`hasMore` 控制"加载更早"按钮。
- pending 消息单独 `Map<channelId, PendingMessage[]>`，`MessageList` 合并后按时间戳排序。
- 未读数：`ADD_MESSAGE` 时如果不是当前频道就 `unread++`，`selectChannel` 时清零。
- typing 指示 3s 过期，`AppProvider` 里 1s 一次 interval 清理。

## 6. 关键用户流

| 流程 | 触发 | 涉及文件 |
|------|------|----------|
| 登录 | `<LoginPage/>` 提交 | `lib/api.login` → cookie，刷新 `fetchMe` |
| 选频道 | Sidebar 点击 | `dispatch(SELECT_CHANNEL)`，懒加载 messages |
| 发消息 | `MessageInput` 回车 | 见 §4 乐观发送 |
| 创建 DM | 点用户名 | `api.openDM(userId)` → 新 channel 出现并选中 |
| 加 reaction | `ReactionBar` 表情 | `api.addReaction/removeReaction` + WS push 同步 |
| 上传图片 | drag/paste/选择 | `api.uploadImage` → markdown 注入编辑器 |
| 拖拽频道 | `ChannelList` dnd-kit | 计算新 LexoRank → `api.reorderChannel` |
| 输入 `/x` | `MessageInput` | `useSlashCommands` 显示 `<SlashCommandPicker/>`，回车 → 内置 `execute()` 或派发到 agent |

## 7. Slash Command 模型

- 内置命令在 `commands/builtins.ts` 注册到 `commandRegistry`。
- 远端命令来自 server `GET /api/v1/commands`（plugin 通过 WS `register_commands` 上报），`commands_updated` 事件触发 `useCommandTracking` 重拉。
- 同名冲突：内置优先；多个 agent 同名 → `ambiguous`，需要用 `/agent:cmd` 限定。

## 8. 测试

`src/__tests__/`，全部 Vitest，**只覆盖纯逻辑模块**：

- `command-registry.test.ts` — resolve 优先级、ambiguous、search 前缀过滤、`setRemoteCommands` 替换语义。
- `channel-sort.test.ts` — position 字符串 lex 排序 + `last_message_at` fallback。
- `channel-groups-ui.test.ts` — 分组展示逻辑。
- `GroupHeader.test.tsx` — 分组头排版锁: 折叠时三角字符 ▶ (朝右) / 展开 ▼ (朝下) 不再用 CSS 旋转; drag-handle 跟 ⋯ 按钮统一 20px 方块 (`group-header-drag-handle` / `group-header-menu-btn`), 不再混 `.icon-btn` 32x32 撑高 header. 修 issue #689.
- `agent-invitations.test.ts` — CM-4.2 client：`createAgentInvitation` / `listAgentInvitations(role)` / `fetchAgentInvitation` / `decideAgentInvitation` 的请求形状、`{invitation}` / `{invitations}` 解包、409 → `ApiError`、`stateToLabel` 4 状态中文映射。
- `agent-state.test.ts` (AL-1a) — `describeAgentState` 三态文案锁 + 6 reason code 表覆盖, 防退化。
- `presence.test.ts` (AL-3.3) — `markPresence` cache + 5s 节流单测：跨窗口立即通知 / 窗口内 burst trailing flush / 多 agent anchor 独立 / 空 agentID 防御 / `PRESENCE_THROTTLE_MS===5000` 字面锁。fake clock 走 `__resetPresenceStoreForTest(()=>nowMs)` 注入。
- `PresenceDot.test.tsx` (AL-3.3) — DOM 字面锁: 三态 `data-presence` 属性 + `.presence-online/.presence-offline/.presence-error` class + 6 reason 文案 byte-identical 跟 `agent-state.ts` 绑定; compact 模式 title fallback; 反约束 — 任意状态文本反查无 busy/idle/忙/空闲。

- `ws-artifact-updated.test.ts` (CV-1.3) — `dispatchArtifactUpdated` 派发 `borgee:artifact-updated` CustomEvent + 7-field key 顺序字面锁 (`['type','cursor','artifact_id','version','channel_id','updated_at','kind']`, 跟 server `cursor.go::ArtifactUpdatedFrame` 锁; BPP-1 #304 envelope CI lint 反查 server 侧) + commit/rollback 双 kind round-trip + 反向断言 frame 不漏 `body|committer_id|committer_kind` (立场 ⑤) + event-name 字面锁 `'borgee:artifact-updated'`。

没有组件级 React Testing Library 测试。
