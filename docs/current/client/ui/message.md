# 4. 消息气泡

## 4a. 普通消息

```
┌──┐  Username                      2:30 PM
│AV│  This is a normal text message that can span
└──┘  multiple lines if the content is long enough.
                                      [😀] [✏️] [🗑]  ← hover 才显示
      👍 3   🎉 1                     ← Reactions
```

## 4b. 代码块消息

```
┌──┐  🤖 AgentX                     2:31 PM
│AV│  Here are the test results:
└──┘
      ┌─────────────────────────────────────────────────────────┐
      │ ```typescript                                           │
      │ describe('auth', () => {                                │
      │   it('should login with valid credentials', () => {     │
      │     expect(result.status).toBe(200);                    │
      │   });                                                   │
      │ });                                                     │
      │ ```                                                     │
      └─────────────────────────────────────────────────────────┘
```

## 4c. @Mention

```
┌──┐  Bob                           2:33 PM
│AV│  @Alice great work on the PR! The @AgentX
└──┘  review comments were helpful too.
       ↑                                ↑
       高亮蓝色背景                      Agent mention 带 🤖 标记
```

- **头像（AV）**：圆角方形，32px，Agent 头像带 🤖 标记
- **用户名 + 时间戳**：用户名加粗，时间戳灰色右对齐
- **Reactions**：消息底部，emoji + 计数，点击可切换
- **Hover 操作**：鼠标悬浮时右上角浮现 React / Edit / Delete 按钮

---

## 4f. BPP-3.2.2 — Capability grant DM 三按钮 (Phase 5)

> BPP-3.2.2 (#494 后续) · 蓝图 [`auth-permissions.md`](../../../blueprint/current/auth-permissions.md) §1.3 主入口 + content-lock [`bpp-3.2-content-lock.md`](../../../qa/bpp-3.2-content-lock.md) §3 DOM 字面锁定.

owner 收 system DM (BPP-3.2.1 server 写) → SystemMessageBubble (`packages/client/src/components/SystemMessageBubble.tsx`) 检测 quick_action JSON 是 BPP-3.2 shape (含 `action ∈ grant/reject/snooze` + 4 必填字段) → 渲染**三按钮**:

```
┌──┐  System                          2:30 PM
│SY│  AgentX 想 commit_artifact 但缺权限 commit_artifact
└──┘  ┌──────┐ ┌──────┐ ┌──────┐
      │ 授权 │ │ 拒绝 │ │ 稍后 │
      └──────┘ └──────┘ └──────┘
```

DOM 字面锁定 byte-identical 跟 content-lock §3 (改 = 改两处: content-lock + SystemMessageBubble.tsx):

| label | data-action | data-bpp32-button | 视觉 |
|---|---|---|---|
| `授权` | `grant` | `primary` | 主按钮 (绿色) |
| `拒绝` | `reject` | `danger` | 次按钮 (红色) |
| `稍后` | `snooze` | `ghost` | 弱按钮 (灰色) |

点击 → `postMeGrant({...payload, action})` → POST `/api/v1/me/grants` → server 真改 user_permissions (action='grant') 或 audit-only (reject/snooze, v1 不持久化 deny list).

**反向约束 (content-lock §3 同义词grep 检查)**: 12 词禁出现在 button label — 批准/授予/同意/许可 (替 "授权") / 驳回/拒接/否决/不允许 (替 "拒绝") / 稍候/延后/推迟/暂缓/过会儿 (替 "稍后"). 单测 `SystemMessageBubble.bpp32.test.tsx` 守.

CM-onboarding 既有单按钮 (`{kind: 'button', label, action}`) 路径不变 — `isBPP32GrantPayload` type guard 区分 shape.

---

## 4g. AL-5.2 — Agent error recovery DM 单 "重连" button (Phase 5)

> AL-5 (#516) · 蓝图 [`agent-lifecycle.md`](../../../blueprint/current/agent-lifecycle.md) §2.3 (5-state error → online edge) + spec [`al-5-spec.md`](../../../implementation/modules/al-5-spec.md) §1 AL-5.2 byte-identical.

agent 翻 error 后, owner 收 system DM (AL-5.1 server 写, 后续 PR) → `SystemMessageBubble` 检测 quick_action JSON 是 AL-5 shape (`action='recover'` + `agent_id` + `reason` + `request_id`) → 渲染**单按钮**:

```
┌──┐  System                          2:30 PM
│SY│  AgentX 状态变更: error (api_key_invalid)
└──┘  ┌──────┐
      │ 重连 │
      └──────┘
```

DOM 字面锁定 byte-identical 跟 spec §1 AL-5.2 (改 = 改两处: al-5-spec.md + SystemMessageBubble.tsx):

| label | data-action | data-al5-button | container marker |
|---|---|---|---|
| `重连` | `recover` | `recover` | `data-al5-recover="true"` |

点击 → `postAgentRecover({action, agent_id, reason, request_id})` → POST `/api/v1/agents/{id}/recover` → server 走 AL-1 #492 single-gate `AppendAgentStateTransition(error→online, lastReason)`, 不拆状态机, 不另起 reason 字典.

**反向约束 (AL-5 spec §0 同义词grep 检查)**: 5 词禁出现在 button label — 重启 / reset / restart / 重新启动 / 重置 (替 "重连"). 单测 `SystemMessageBubble.al5.test.tsx::reverse — no synonym buttons` 守.

`isAL5RecoverPayload` type guard 区分 shape, BPP-3.2 + AL-5 共用 SystemMessageBubble — 两 payload 都给时按 BPP-3.2 优先 (al5 ∧ ¬bpp32 才渲 重连按钮). CM-onboarding 单按钮 fallback 在两者都无时渲.

---

## 5. 消息间距 + Reaction 布局 (gh#686 / PR #705)

### 5.1 消息间距

消息气泡之间 `margin-block: 8px` (反默认 0 间距挤在一起) — `packages/client/src/index.css` `.message-item` 选择器. 同 sender 连续消息之间 `margin-block: 4px` (紧凑视觉分组).

### 5.2 ReactionAddButton (新组件)

```
┌──┐  Username                      2:30 PM
│AV│  Message text...
└──┘                                        [➕]   ← hover 才显示, 点开 emoji picker
      👍 3   🎉 1                     ← Reactions chip 在消息下方独立行
```

之前 reaction chip + add 按钮挤在 hover toolbar 里 (跟 ✏️/🗑 编辑/删除按钮同行), 视觉拥挤. PR #705 拆出 `ReactionAddButton.tsx` 独立组件 + reaction chip 移到消息下方独立行.

DOM 出处:
- `<button data-testid="reaction-add-button" aria-label="添加 reaction">` — hover 才显示
- `<div className="reaction-chips">` — chip 容器, 在 `.message-item` 下方独立行

点击 → 打开 emoji picker → 选 emoji → POST `/api/v1/messages/:id/reactions` → reducer 派发.

### 5.3 三态共存 reducer (#705)

`packages/client/src/context/AppContext.tsx` reducer 同时支持三态 action:
- `ADD_REACTION_OPTIMISTIC` — 用户点选 emoji 立刻乐观渲染 (反 ws push 来之前 UI 空)
- `REMOVE_REACTION_OPTIMISTIC` — 用户取消 reaction 立刻乐观移除 (反 ws push 来之前 chip 还在)
- `UPDATE_REACTIONS` — ws push 用服务端聚合结果整体替换 (server 端聚合后下发, 避免多设备并发竞态)

REMOVE 走 user_id 移除 (不是 emoji-wide 删除) — 反 N 用户给同 emoji 时, 一个用户取消把别人 reaction 也清掉.

竞态修复:
- 乐观 ADD 后 ws push 来时 UPDATE_REACTIONS 整体替换, 不双计
- 跨设备 (设备 A 加 reaction, 设备 B 收 ws push) → UPDATE_REACTIONS 同步, last-writer-wins
- HTTP 失败 → reducer rollback (走 REMOVE_REACTION_OPTIMISTIC) 反留乐观状态

详见 design `docs/implementation/design/686-message-spacing-reaction-position.md`.
