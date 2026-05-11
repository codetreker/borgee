# Messages API + sender messages do not count as unread for that sender (gh#687) — implementation note

> gh#687 (PR #704) — 自己发的消息不应该让自己的侧边栏闪 unread badge.
> 蓝图: `channel-model.md` §2.1 (channel = 协作场) + §4.2 (未读语义) + §4.6 (mark-read) + §4.8 + §4.9 (multi-device).

## 1. 设计

Messages sent by the current user use three safeguards so they do not count as unread for that same user:
- **Layer 1 (server/API after-send fallback)**: the API handler `handleCreateMessage` calls store method `h.Store.CreateMessageFull(...)`, then immediately best-effort calls `h.Store.MarkChannelRead(channelID, user.ID)` for the sender. This covers the after-send window before the next channel aggregation response; failure is logged and does not block message creation.
- **Layer 2 (server SQL)**: `GetChannelsForUser` 聚合 unread_count 时用 `WHERE m.sender_id != ?` 只统计其他用户发来的消息. Prevents server unread aggregation from counting the sender's own messages in multi-device scenarios.
- **Layer 3 (client reducer)**: ws push frame 来时 reducer 判 `if (frame.sender_id === currentUser.id) return; // skip bump` — prevents sidebar unread increments when another device receives a message from the current user.

Required constraints:
- ① 三层都必须有 (单层失效另两层保底, avoids a single point of failure)
- ② Layer 2 SQL `sender_id != ?` 不能改成 `sender_id == ?` (that would count only the sender's own messages as unread and stop counting unread messages from other users)
- ③ 其他用户发来的消息仍算 unread (regression assertion: Layer 2 still counts messages whose sender differs from the current user)
- ④ multi-device self-send (设备 A 发, 设备 B 收) 在设备 B 也不算 unread (Layer 3 走 sender_id 比对)

## 2. Layer 1 (server/API after-send fallback)

| Scope | Location | Behavior | Failure semantics |
|---|---|---|---|
| API handler fallback | `packages/server-go/internal/api/messages.go::handleCreateMessage` | Calls `h.Store.MarkChannelRead(channelID, user.ID)` immediately after `h.Store.CreateMessageFull(...)` returns the created message. | Logs the failure and still returns the created message. |
| Store message creation | `packages/server-go/internal/store/queries.go::CreateMessageFull` | Persists the message and related store-side records. | Not the fallback location; the API handler performs the after-send mark-read call. |

Client `markChannelRead(channelId)` remains a channel selection/open behavior in
`packages/client/src/context/AppContext.tsx`; it is not an after-send safeguard.

`POST /api/v1/channels/:id/messages` 路径:

```go
msg, err := h.Store.CreateMessageFull(channelID, user.ID, content, ct, body.ReplyToID, body.Mentions)
...

// API handler 写消息后, server/API 端尽力 mark current channel read for the sender.
// 避免下一次 channel 聚合返回前, sender 自己发的消息短暂显示 unread.
if mrErr := h.Store.MarkChannelRead(channelID, user.ID); mrErr != nil {
  // 不阻塞消息创建 — 失败仅 log; Layer 2 SQL 和 Layer 3 reducer 继续保底.
  h.Logger.Error(...)
}
```

Server/API fallback is best-effort: 失败仅 log 不阻塞消息创建. Layer 3 reducer skips own messages, and the Layer 2 SQL filter remains as fallback. Client `markChannelRead(channelId)` is still used for channel selection/open, not after send.

## 3. Layer 2 (server SQL) — `packages/server-go/internal/store/queries.go`

5 个 SQL locations calculate channel unread_count; all add `AND m.sender_id != ?`:

```sql
-- L862, L885, L1117, L1138, L1581 (queries.go 五处算 unread 的 query)
SELECT COUNT(*) FROM messages m
JOIN channels c ON c.id = m.channel_id
WHERE c.id = ?
  AND m.created_at > COALESCE(read_marker.last_read_at, 0)
  AND m.sender_id != ?  -- ← gh#687 Layer 2: 排除自己发的
```

All 5 locations must match; missing one would make unread counts differ between GET /channels list view and GET /channels/:id detail view.

## 4. Layer 3 (client reducer) — `packages/client/src/context/AppContext.tsx`

ws push 收到新消息 frame 时:

```ts
case 'NEW_MESSAGE': {
  const msg = action.payload;
  // gh#687 Layer 3: current user's message 跨设备到达时不 bump unread
  if (msg.sender_id === state.currentUser?.id) {
    return state;  // skip bump
  }
  // 其他用户发来的消息走原 path bump unread
  ...
}
```

跟 Layer 1 联动: 当前用户发完消息后, server/API fallback attempts to mark the sender's channel read after accepting the message; ws push 到当前设备时 Layer 3 avoids another unread increment. 跨设备时 Layer 3 继续根据 sender_id 判断 (设备 A 发, 设备 B 收 ws push 走 Layer 3 不增加 unread).

## 5. Regression assertion (other users' messages still count as unread)

Layer 2 SQL 的 `sender_id != ?` 关键约束: 其他用户发来的消息仍算 unread. Regression assertion:

```
test: user B 发消息到 shared channel
expect: owner (user A) 视角 GET /channels 返 shared channel unread_count >= 1
```

Prevents accidentally changing Layer 2 SQL to `sender_id == ?`, which would exclude messages from other users.

## 6. 测试

- vitest unit (`packages/client/src/__tests__/`):
  - `gh-687-own-unread-reducer.test.ts` — Layer 3 reducer ignores own messages (≥3 case: 自己发不增加 unread / 别人发增加 unread / 当前 channel 不增加 unread)
- go test unit (`packages/server-go/internal/store/`):
  - `gh-687-unread-sql.test.go` — Layer 2 SQL 5 locations 全过 + 其他用户发来的消息仍算 unread (≥6 case)
- e2e (`packages/e2e/tests/self-message-unread-counter.spec.ts`, gh#700 / PR #711):
  - §7.3 主路径 5 步: own message 切走切回 unread=0
  - §7.2 其他用户发来的消息仍算 unread
  - §4.2 multi-device self-send 设备 B 不闪 unread

## 7. 参考资料

- 蓝图: `channel-model.md` §2.1 / §4.2 / §4.6 / §4.8 / §4.9
- design: `docs/implementation/design/687-self-message-unread-design.md`
- PR: #704 (Closes gh#687, 三层防御实施) + #711 (Closes gh#700, e2e regression spec)
