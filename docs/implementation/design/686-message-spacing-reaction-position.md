# 686 — 消息间距 + 添加表情按钮位置 — 实施设计

> milestone: bug fix #686
> 蓝图: `blueprint/current/channel-model.md` (沉默 — 视觉细节属于实施层, 蓝图不写视觉规则)
> 产品方向: yema 已审过 (跟 Slack / Discord / Telegram 同模式)
> Dev 主笔: zhanma-c
> 4 签状态: ✅ feima R1 (架构师, picker 锚定澄清纳入 §1 + 集成测建议纳入 §6) → ✅ feima R2 (复审 81204ae 抓事实错: project 里没 `ADD_REACTION` action 真路径是 `UPDATE_REACTIONS` 整体替换, §4 #11a 已纠正 + 加 §4 #11b 防 REMOVE_REACTION_OPTIMISTIC 误删别人 reaction) / ✅ yema (PM, §6 .reaction-bar-empty 清纳入 + 拍 §4 #11 = 乐观 + 失败 toast) / ✅ liema R1 (NOT-LGTM 5 条 → 全部纳入 §4 #11-15) → ✅ liema R2 (81204ae LGTM, §6.1 文案锁 + 反近义词 grep 守得好) / ✅ heima Security (12 类 + 4 重点全过, §5 client throttle 留账不 block)

## §1 数据流

只动 client 端 React 组件渲染逻辑, 不动 server / 不动数据库 schema / 不动 API.

```
用户打开 channel
  → MessageList 拉一批 messages
  → 每条 message 进 MessageItem 组件
  → MessageItem 看 message.reactions 长度:
      ├─ length === 0  (没人 reaction)
      │     ├─ 不渲染 ReactionBar (返回 null)
      │     └─ .message-actions 浮起工具栏里加一个 "添加表情" (➕) 按钮,
      │        跟 ✏️ 编辑 / 🗑️ 删除 一组
      └─ length > 0  (已经有人 reaction)
            ├─ 渲染 ReactionBar (横排 emoji 药丸 + 末尾一个 ➕)
            └─ .message-actions 工具栏不再放 ➕ (避免重复, 用户在 reaction
               那行点 ➕ 就够了)

点 ➕ → 弹 emoji 选择面板 → 选 emoji
       → POST /api/v1/messages/{id}/reactions  (复用 lib/api.addReaction)
       → WebSocket 推 reactions 更新
       → AppContext reducer 更新 message.reactions
       → MessageItem 重渲染 → length > 0 路径
       → ➕ 按钮位置自动从工具栏切到 ReactionBar 末尾
```

**picker 锚定** (按 feima review 澄清): ReactionAddButton 自己内联 emoji
picker (跟现有 ReactionBar 模式一致), picker 在 DOM 上锚到自己的 ➕ 按钮
位置 (`position: absolute; bottom: 100%`). 两个挂载点 (`.message-actions`
工具栏里的 ➕ + ReactionBar 末尾的 ➕) 各自一个 ReactionAddButton 实例,
各自一份 `useState(open)` + outside-click + escape 处理, picker state
互不串扰. 不存在"两个挂载点共享一个 picker"的情况.

## §2 数据模型

不改. message.reactions 字段已经存在 (类型 `{emoji, count, user_ids}[]`),
此次只换 client 端的渲染条件, 不动后端.

## §3 API contract

不改. 复用既有:
- `POST /api/v1/messages/{id}/reactions` (走 `lib/api.ts::addReaction`)
- `DELETE /api/v1/messages/{id}/reactions/{emoji}` (走 `lib/api.ts::removeReaction`)

## §4 边界条件 + 错误处理

| # | 场景 | 行为 | 怎么处理 |
|---|---|---|---|
| 1 | 消息已删除 (`deleted_at != null`) | 不出 ReactionBar 也不出工具栏 | 既有 `!isDeleted` gate 已挡, 维持 |
| 2 | 消息正在编辑 (`editing == true`) | 同上不出 | 既有 `!editing` gate 已挡, 维持 |
| 3 | 消息发送中 (`_pending`) 或失败 (`_failed`) | 不出 ➕ (✏️/🗑️ 也已经被既有逻辑挡) | 新加 `canAddReaction = !pending && !failed && !deleted && !hasReactions` |
| 4 | 加上第一个 reaction 后, 工具栏的 ➕ 应消失, ReactionBar 末尾的 ➕ 应出现 | 通过 `hasReactions = (reactions?.length ?? 0) > 0` 推算; React 重渲染自动处理 | 不加平滑动画, 跟 Slack/Discord 一致, 切换是离散的 |
| 5 | 移除最后一个 reaction 后, ReactionBar 消失, ➕ 又回到工具栏 | 同 #4 反向 | 同上 |
| 6 | hover 状态时切换 (#4 场景) | hover 由 CSS `:hover` 维持; 工具栏 ➕ 消失 + ReactionBar 出现是离散切换 | CSS 不加 transition 防 "➕ 闪烁两次" (一次淡出工具栏 + 一次淡入 reaction 行) |
| 7 | 同时点 ➕ 时另一个 emoji 选择面板已经开 | 既有 useEffect outside-click 关掉旧面板; 新面板只跟自己的按钮挂 | 复用 ReactionAddButton 自己管 open/close 的模式, 不串扰 |
| 8 | 没 user (`currentUserId == null`) | 不挡 ➕ 出现 (服务端会 401) | 跟 ReactionBar 既有行为一致, 服务端拒 |
| 9 | mobile 长按 | 既有 `useLongPress` 弹 mobile-action-sheet, gate 改为 `canEdit \|\| canDelete \|\| canAddReaction` | 已加 |
| 10 | 系统消息 (`sender_id === 'system'`) | 走 SystemMessageBubble 分支 (return early), 跟 reaction 无关 | 既有逻辑维持 |
| 11 | reaction POST 失败 (5xx / timeout / 4xx) | **乐观渲染 + 失败 toast** (按 yema 拍 X 方案: B 乐观渲染 + 失败 toast 提示, 不是纯 A 悲观 spinner 也不是纯 B 静默撤回) | 实施步骤: ① 用户点 emoji → optimistic dispatch (`ADD_REACTION_OPTIMISTIC` reducer action 立刻改 message.reactions, 把 currentUserId 加到对应 emoji 的 user_ids 数组, count + 1; emoji 在 reactions 里没有就新建一条 `{emoji, count: 1, user_ids: [currentUserId]}`) → UI 立刻显新 pill ② await `api.addReaction` ③ 成功 → 不动 (等 WS 推到走 §11a 路径整体替换) ④ 失败 → dispatch `REMOVE_REACTION_OPTIMISTIC` 撤回 (按 currentUserId 移除而非按 emoji 删整条 — 见 §11b race 修法) + 调 `useToast().showToast('添加 reaction 失败, 请重试')` |
| 11a | WS 推 reaction 跟乐观渲染撞 (yema 提, feima R2 事实纠正) | 不重复加 pill | **真路径是 `UPDATE_REACTIONS` reducer 整体替换** (AppContext.tsx L75 type + L300 case + useWebSocket.ts:356 dispatch), 不是按 emoji 增量去重. 乐观 dispatch `ADD_REACTION_OPTIMISTIC` 加 pill 后, WS 推到时 message.reactions 整体替换为服务器返的完整数组, 服务器端已经 dedupe (按 user_id + emoji). 实施时反向 grep `grep -n "case 'UPDATE_REACTIONS'" packages/client/src/context/AppContext.tsx` 确认这是唯一的 reaction 路径 (project 里**没有** `ADD_REACTION` action) |
| 11b | REMOVE_REACTION_OPTIMISTIC 跟 WS race (feima R2 实施细节) | 不误删别人的 reaction | 时间序列: t0 用户点 emoji → `ADD_REACTION_OPTIMISTIC` → t1 API await → t2 **WS 推到比 HTTP 4xx 早到** (常见情况) → `UPDATE_REACTIONS` 整体替换为服务器版完整 reactions (含别人同 emoji 的 reaction) → t3 API reject → 走 REMOVE_REACTION_OPTIMISTIC. 此时 reactions 是服务器版, 如果 reducer 写 "按 emoji 删整条" 会**误删别人的 thumbs-up**. **修法**: REMOVE_REACTION_OPTIMISTIC reducer 应该是"按 emoji 找到 user_ids 含 currentUserId 的那一条, 从 user_ids 移除 currentUserId, count - 1, 如果 count == 0 删整条" (跟既有 `ReactionBar.handleToggle` 走 `removeReaction` API 的 server 端语义同源). 单测必覆盖: ① ADD_REACTION_OPTIMISTIC ② UPDATE_REACTIONS (WS 推, reactions=[{thumbs-up, count: 3, user_ids: [A, B, currentUser]}]) ③ REMOVE_REACTION_OPTIMISTIC (API fail) → 期望 reactions=[{thumbs-up, count: 2, user_ids: [A, B]}] ④ 反向断言 A / B 的 thumbs-up 没被误删 |
| 12 | 双击 / 快速连点 ➕ (picker race / 双 ➕ DOM 状态) | 第一次点开 picker, 第二次点关 (toggle), 不弹两个 | ReactionAddButton 内部维护 `busy` state: 点击中 (await addReaction) `disabled=true` 防 race; picker open 期间再点同一个 ➕ → toggle 关; 选完 emoji 后 picker 闭合 + 同一时刻 reactions 切到 ≥1, MessageItem 重渲染让工具栏 ➕ 不再 mount, 不存在"双 ➕ 同时活" |
| 13 | Touch target 大小 (WCAG 2.1 AA ≥ 44x44px CSS) | 视觉保持原字号 (11px), 命中区撑大 | `.message-action-btn.message-action-react` 加 `min-width: 24px; min-height: 24px; padding: 6px;` 桌面命中区 ~24-30px (鼠标够); 触摸设备走既有 `useLongPress` 弹 `.mobile-action-sheet` (按钮高 ≥44px 已合规); 反约束: 不为了凑 44px 把视觉撑到 44px 破排版紧凑修法 |
| 14 | Picker 溢出 viewport (➕ 在屏幕底部 / 短屏 mobile) | 自动翻向上不溢出 | CSS-only 方案 (不引 floating-ui / popper.js 加包重量): picker 默认 `position: absolute; bottom: 100%; left: 0; max-height: min(360px, 50vh)` 永远向上展开 (跟既有 `.reaction-picker-popover` 行为一致, 见 index.css:2767-2773); 短屏 mobile (`max-height: 50vh` 兜底) + picker 内部既有 emoji-mart 自带滚动; 反约束: 不引 floating-ui 第三方 (本次只是 bug fix, 加包要单独 milestone 评估) |
| 15 | 键盘 a11y (Tab / Enter / Space / Escape / focus trap / aria-label) | ➕ 是真 button 默认 Tab/Enter/Space 都行; Escape 关 picker; aria-label 写"添加表情" | ① ReactionAddButton render `<button type="button" aria-label="添加表情" aria-haspopup="dialog" aria-expanded={open}>` ② Escape 已经在既有 useEffect 处理 ③ Tab 进 picker — emoji-mart 自带键盘导航 (上下左右 + Enter), 不另起 focus trap (避免引 react-focus-lock); 反约束: 不强制 focus trap, 让用户 Shift+Tab 出去也行 (跟既有 ReactionBar picker 行为一致) ④ ➕ 按钮文字是 emoji "➕", aria-label "添加表情" 给 screen reader 读, title="添加表情" 给鼠标悬停提示 |

## §5 多个方案

### 方案 A — 抽 `ReactionAddButton` 组件, 两种 variant (inline-pill / toolbar-btn), 两处复用 (本次选)

- Pro: emoji 选择面板逻辑 (open/close, outside-click, escape, addReaction 调用) 只一份
- Pro: 两个挂载位置共享 picker DOM 锚 + key/aria 行为
- Pro: 测试只测一个组件就锁两种 variant
- Con: 多一个新文件 (`ReactionAddButton.tsx`)

### 方案 B — 不抽组件, MessageItem 自己起 picker state, ReactionBar 自己起 picker state

- Pro: 不新加文件
- Con: 同一份逻辑写两份, 容易 drift (一处改另一处忘改)
- Con: 两个 picker state 需要互相协调 "我开你关", 复杂度反而上升

### 方案 C — 不挪 ➕, 只把 ReactionBar 在 reactions=[] 时返回 null 让消息容器收缩

- Pro: 改动最小 (只删 ReactionBar 里的空 bar 占位)
- Con: 用户没 reaction 状态下没法添加表情了 — 跟 yema 拍的产品方向 (➕ 跟 edit/delete 一组) 直接冲突
- Con: 退化, 不能走

**选 A**, 真实原因: yema 拍的产品方向要求 ➕ 在两个位置真出现 + 切换. 方案 B 维护成本高 + 容易 drift. 方案 C 不做 yema 要的功能, 等于绕过产品方向.

## §6 跟现有代码的接合

反向 grep 锚 (本次改动需要确认的接口):

- `packages/client/src/components/MessageItem.tsx` — 主改文件, 加 `canAddReaction` 推算 + 调整 ReactionBar / ReactionAddButton 渲染分支
- `packages/client/src/components/ReactionBar.tsx` — `reactions=[]` 时直接 `return null` (历史行为是 `return <div className="reaction-bar reaction-bar-empty">` 占位); pills 末尾用 `<ReactionAddButton variant="inline-pill" />` 替既有 inline `<button>`
- `packages/client/src/components/ReactionAddButton.tsx` — **新文件**, ➕ 按钮 + emoji 选择面板. 实施细节按 §4 #11-15:
  - 内部 `useState(busy)` 防双击 race (#12)
  - 调 `useToast().showToast('添加 reaction 失败, 请重试')` 走失败 toast 路径 (#11, 复用 `Toast.tsx` 既有 facility)
  - 乐观渲染走 dispatch `ADD_REACTION_OPTIMISTIC` action (新增 reducer action, 见 AppContext.tsx 改动条目)
  - aria-label="添加表情" + aria-haspopup="dialog" + aria-expanded={open} (#15)
  - picker 永远向上 (CSS `bottom: 100%; max-height: min(360px, 50vh)`) (#14)
- `packages/client/src/context/AppContext.tsx` — 加 `ADD_REACTION_OPTIMISTIC` + `REMOVE_REACTION_OPTIMISTIC` 两个 reducer action, 跟既有 `UPDATE_REACTIONS` (整体替换语义, 见 L75 type + L300 case) **共存不冲突**. **REMOVE_REACTION_OPTIMISTIC 关键语义** (按 feima R2 §4 #11b): 不能"按 emoji 删整条" (race 中 WS 已经把 reactions 整列替换为服务器版含别人 reaction 时会误删), 必须 "按 emoji 找到 user_ids 含 currentUserId 的那一条, 从 user_ids 移除 currentUserId, count - 1, count == 0 删整条" — 跟既有 `ReactionBar.handleToggle` 走 `removeReaction` API 的 server 端语义同源
- `packages/client/src/index.css` — 删 `.reaction-add-hidden` 规则 (悬浮空 bar 占位用的, 不再需要) + 删 `.reaction-bar-empty` 规则 (按 yema review: 既然 ReactionBar 在 reactions=[] 时 `return null`, 那个修饰类也没东西匹配了, 一起清干净); 加 `.message-action-btn.message-action-react` (字号 11px + min-width/height 24px + padding 6px 给桌面命中区 #13); `.reaction-picker-popover` 加 `max-height: min(360px, 50vh)` (短屏 mobile 防溢出 #14)
- `packages/client/src/__tests__/ReactionBar.test.tsx` — **新文件**, 锁 `reactions=[] return null` + 反向断言 `.reaction-bar-empty` / `.reaction-add-hidden` 都不再出现
- `packages/client/src/__tests__/ReactionAddButton.test.tsx` — **新文件**, 锁:
  1. 两种 variant 各自的 className + ➕ 文字 + title + 点击开关 picker (原有)
  2. 失败时调 `showToast` (mock useToast) + 撤回乐观 pill (mock dispatch) (#11)
  3. busy 期间二次 click 不发第二次请求 (#12)
  4. aria-label / aria-haspopup / aria-expanded 字面 (#15)
- `packages/client/src/__tests__/reaction-reducer-race.test.ts` — **新文件** (按 feima R2 §4 #11b 实施细节), 锁 reducer 跟 WS race 路径:
  1. 初始 `state.messages[].reactions=[]`
  2. dispatch `ADD_REACTION_OPTIMISTIC{emoji='👍', userId='currentUser'}` → 期望 `[{emoji:'👍', count:1, user_ids:['currentUser']}]`
  3. dispatch `UPDATE_REACTIONS{reactions:[{emoji:'👍', count:3, user_ids:['A','B','currentUser']}]}` (模拟 WS 推到含别人 reaction)
  4. dispatch `REMOVE_REACTION_OPTIMISTIC{emoji='👍', userId='currentUser'}` (模拟 API fail)
  5. **期望** `[{emoji:'👍', count:2, user_ids:['A','B']}]` — A / B 的 reaction 没被误删
  6. **反向断言** A 的 user_id 仍在 user_ids 数组里, count 不是 0
- `packages/client/src/__tests__/MessageItem-reaction-toggle.test.tsx` — **新文件** (按 feima review: 需要集成测覆盖组合点, 单测两个子组件不够), 锁两路径:
  1. `reactions=[]` → 工具栏 ➕ 出现 (toolbar-btn variant), ReactionBar 不渲染 (DOM 没 `.reaction-bar`)
  2. `reactions=[一个]` → 工具栏 ➕ 不出 (避免重复), ReactionBar 渲染 + 末尾 ➕ (inline-pill variant)
  > 写法: mock useAppContext + useToast 减依赖, 直接渲染 MessageItem 验 DOM

复用既有不动:
- `lib/api.addReaction / removeReaction` — 不改
- `@emoji-mart/react` + `@emoji-mart/data` — 不改
- `useToast()` (`packages/client/src/components/Toast.tsx`) — 复用既有 toast facility 显失败提示, 不引新组件 (#11)
- `EmojiPickerPopover` (DM-9 那个 5-emoji preset picker) — 不复用, 那是 DM 系统的 preset picker, 跟 channel 的 full picker 是两条产品路径

新组件 ReactionAddButton **挂 useAppContext (dispatch) + useToast (showToast)** — 跟前一版 design (说 "不挂 AppContext") 不一样, 因为现在要走乐观 dispatch + 失败 toast (按 yema 拍 X 方案 + liema #11). 单元测试需要 mock useAppContext + useToast (见 §6 测试清单).

跟现有架构没冲突点: 既有 ReactionBar 已经在 picker 路径里调 `addReaction`, 不破契约.

跨模块影响:
- `MessageList.tsx` 不动 (它只 map 出 `<MessageItem>`)
- DM 系统的 `DMMessageReactionPicker` / `EmojiPickerPopover` 不动 (是另一条产品路径)
- 不动 server / 不动 WebSocket / 不动 schema

## §6.1 文案锁 (按 yema review + team-lead 提醒)

#686 引入一条新用户可见文案 (失败 toast), byte-identical 锁住, 改 = 改三处
(此 design doc + ReactionAddButton.tsx 调用现场 + 单元测试断言):

```
add_reaction_failed_toast → "添加 reaction 失败, 请重试"
```

**反向 grep** (count==0 反近义词漂移): 在 `packages/client/src/components/ReactionAddButton.tsx` + `packages/client/src/__tests__/ReactionAddButton.test.tsx` 内, 不允许出现 `添加失败|reaction 失败|重试一下|失败了|Add failed` 这类近义词 — 仅上面字面允许.

## 反查 grep (写代码前自检, 防漏)

- `grep -rn "reaction-bar-empty\|reaction-add-hidden" packages/ docs/` → 应该 0 hit (除新文件自己的注释)
- `grep -rn "ReactionAddButton" packages/client/src` → 应在 MessageItem + ReactionBar + 测试文件出现
- `grep -rn "canAddReaction" packages/client/src` → 应只在 MessageItem 出现
- `grep -rn "message.reactions" packages/client/src` → 历史路径 + 新加的 `hasReactions` 推算; 不破
- `grep -rn "ADD_REACTION_OPTIMISTIC\|REMOVE_REACTION_OPTIMISTIC" packages/client/src` → 应在 AppContext.tsx (reducer 定义) + ReactionAddButton.tsx (dispatch 调用) 出现, 单测 mock
- `grep -n "case 'UPDATE_REACTIONS'" packages/client/src/context/AppContext.tsx` → 验证现有 reducer 是整体替换语义 (跟 §4 #11a 假设一致, 不是按 emoji 增量去重); 同时反向 `grep -n "ADD_REACTION[^_]\|case 'ADD_REACTION'$" packages/client/src/context/AppContext.tsx` 应 0 hit (project 里**不存在** `ADD_REACTION` action, 之前 design 误引)
- `grep -rnE "添加失败|reaction 失败|重试一下|失败了|Add failed" packages/client/src/components/ReactionAddButton.tsx packages/client/src/__tests__/ReactionAddButton.test.tsx` → 应该 0 hit (反 §6.1 文案锁近义词漂移; 仅 "添加 reaction 失败, 请重试" byte-identical 字面允许出现)

## 不在范围

- 不改 reaction 的服务端逻辑 (count / user_ids 聚合)
- 不改 emoji 选择面板的库 (维持 `@emoji-mart`)
- 不改 mobile-action-sheet 的视觉 (只把 gate 加 canAddReaction)
- 不改 DM 系统的 reaction 路径 (DMMessageReactionPicker / DM-9 preset)
- 不动 channel 分组 / 折叠 (那是 #689, 已合)
- **client 端 reaction throttle 不做** (heima Security R1 §5 留账): 乐观渲染 + 失败 toast 路径下用户快速点 ➕ 多个 emoji 会每次都立刻 POST, 加重 server 负载. server 端 reactions.go 没看到现成 reaction rate limit. 本次只是 bug fix scope 不引 throttle, 留 backlog 单独 issue 跟 server 端 rate limit 一起讨论

## 命名澄清 (yema R2 提)

`ADD_REACTION_OPTIMISTIC` / `REMOVE_REACTION_OPTIMISTIC` 跟既有 `UPDATE_REACTIONS` (WS 推到走) 在同一 reducer 命名空间. yema 提示有混淆风险, 建议 `OPTIMISTIC_ADD_REACTION` / `LOCAL_ADD_REACTION_PENDING` 等前缀分组写法. 维持当前命名 (`<动词>_<对象>_OPTIMISTIC` 后缀模式), 因为:

1. **跟既有 reducer 风格一致**: 现有 reducer action (`UPDATE_REACTIONS` / `EDIT_MESSAGE` / `REMOVE_GROUP`) 都是 `<VERB>_<OBJECT>` 大写下划线, 后缀 `_OPTIMISTIC` 加在末尾不破整体风格
2. **后缀分组比前缀分组更易 grep**: `grep "_OPTIMISTIC$"` 一把抓到所有乐观 action; 前缀 `OPTIMISTIC_*` 散在不同对象类型里反而难统计
3. **同模式可复用**: 之后如果要给消息发送 / 频道置顶等也加乐观更新, 都用 `_OPTIMISTIC` 后缀, project 里有一致命名约定

如果 yema 复审后还是觉得有混淆风险, 实施 PR review 时可以再讨论改名, 不是 design 阶段的阻塞.
