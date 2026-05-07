# Messages API + own message 不计未读 (gh#687) — implementation note

> gh#687 (PR #704) — 自己发的消息不应该让自己的 sidebar 闪 unread badge.
> 蓝图: `channel-model.md` §2.1 (channel = 协作场) + §4.2 (未读语义) + §4.6 (mark-read) + §4.8 + §4.9 (multi-device).

## 1. 立场

own message 三层防御 (defense-in-depth) 反 sidebar 闪 unread:
- **Layer 1 (client)**: 发完 own message 立刻调 `markChannelRead(channelID, currentUser.id)` 标当前 channel 已读. 反 server 端聚合 ack 来之前的窗口期 unread badge 闪.
- **Layer 2 (server SQL)**: `GetChannelsForUser` 聚合 unread_count 时 `WHERE m.sender_id != ?` 排除自己发的. 反 multi-device 场景下 server 算 unread 把自己发的算进去.
- **Layer 3 (client reducer)**: ws push frame 来时 reducer 判 `if (frame.sender_id === currentUser.id) return; // skip bump` — 反非当前 channel 收到自己 (跨设备) 发的消息时 sidebar bump.

反约束:
- ① 三层都必须有 (单层失效另两层兜底, 反 single point of failure)
- ② Layer 2 SQL `sender_id != ?` 不能改成 `sender_id == ?` (反误反 — 那会让自己看不到自己发的)
- ③ peer 发的消息仍算 unread (反向断言: Layer 2 不误伤别人)
- ④ multi-device own (设备 A 发, 设备 B 收) 在设备 B 也不算 unread (Layer 3 走 sender_id 比对)

## 2. Layer 1 (client) — `packages/server-go/internal/api/messages.go::CreateMessageFull`

`POST /api/v1/channels/:id/messages` 路径:

```go
// CreateMessageFull 写消息后, server 端尽力 mark current channel read for the sender.
// 反 client 端等 server 端聚合 ack 来再 mark 的窗口期 (用户 ws push 看到自己发的
// 消息但 sidebar 还没 mark-read, badge 闪一下).
err := h.store.MarkChannelRead(ctx, channelID, user.ID)
if err != nil {
  // 反阻塞 — 失败仅 log, 走 client Layer 1 兜底 (跟 client mark 重复幂等).
  log.Warn(...)
}
```

best-effort: 失败仅 log 不阻塞消息创建 — Layer 3 reducer skip own + Layer 2 SQL filter 兜底.

## 3. Layer 2 (server SQL) — `packages/server-go/internal/store/queries.go`

5 个 SQL spot 算 channel unread_count, 全加 `AND m.sender_id != ?`:

```sql
-- L862, L885, L1117, L1138, L1581 (queries.go 五处算 unread 的 query)
SELECT COUNT(*) FROM messages m
JOIN channels c ON c.id = m.channel_id
WHERE c.id = ?
  AND m.created_at > COALESCE(read_marker.last_read_at, 0)
  AND m.sender_id != ?  -- ← gh#687 Layer 2: 排除自己发的
```

5 处全改齐 — 缺一处会造成 GET /channels list 视图跟 GET /channels/:id 详情视图 unread 不一致, 反 server 端聚合不一致漂.

## 4. Layer 3 (client reducer) — `packages/client/src/context/AppContext.tsx`

ws push 收到新消息 frame 时:

```ts
case 'NEW_MESSAGE': {
  const msg = action.payload;
  // gh#687 Layer 3: own message 跨设备到达时不 bump unread
  if (msg.sender_id === state.currentUser?.id) {
    return state;  // skip bump
  }
  // 非 own 走原 path bump unread
  ...
}
```

跟 Layer 1 联动: own 在当前设备发完, Layer 1 立刻 mark-read; ws push 到当前设备时 Layer 3 跳 bump (反双 mark/双 bump 噪声). 跨设备时 Layer 3 仍守 (设备 A 发, 设备 B 收 ws push 走 Layer 3 跳 bump).

## 5. 反向断言 (peer 仍算 unread)

Layer 2 SQL 的 `sender_id != ?` 关键反约束: peer 发的消息仍算 unread. 反向断言:

```
test: peer (user B) 发消息到 shared channel
expect: owner (user A) 视角 GET /channels 返 shared channel unread_count >= 1
```

防 Layer 2 SQL 误反成 `sender_id == ?` (那会让别人发的也排除).

## 6. 测试

- vitest unit (`packages/client/src/__tests__/`):
  - `gh-687-own-unread-reducer.test.ts` — Layer 3 reducer skip own (≥3 case: 自己发跳 / 别人发 bump / 当前 channel 不 bump)
- go test unit (`packages/server-go/internal/store/`):
  - `gh-687-unread-sql.test.go` — Layer 2 SQL 5 spot 全过 + 反向 peer 仍算 (≥6 case)
- e2e (`packages/e2e/tests/me-1-self-message-unread.spec.ts`, gh#700 / PR #711):
  - §7.3 主路径 5 步: own message 切走切回 unread=0
  - §7.2 反向 peer 发的仍算 unread
  - §4.2 multi-device own 设备 B 不闪 unread

## 7. 锚

- 蓝图: `channel-model.md` §2.1 / §4.2 / §4.6 / §4.8 / §4.9
- design: `docs/implementation/design/687-self-message-unread-design.md`
- PR: #704 (Closes gh#687, 三层防御实施) + #711 (Closes gh#700, e2e regression spec)
