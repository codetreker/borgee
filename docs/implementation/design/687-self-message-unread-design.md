# 687 — 自己消息未读修 (implementation design)

> blueprintflow `implementation-design` skill 输出. Dev (zhanma 战马) 写, 4 角色 review: Architect (feima 飞马) / PM (yema 野马) / QA (liema 烈马) / Security (heima 黑马). 按 `blueprintflow-team-roles` skill 硬性规定 Architect/PM 都不准兼 Security, heima 是独立角色.
>
> 关联:
> - GitHub issue: #687
> - 蓝图: `blueprint/current/channel-model.md` (沉默 — 蓝图层不写未读计算细节, 实施层 bug 修)
> - PR: 待 open (本 design doc 4 签过才推 + 开 PR)

## §0 现象

用户在 testing-borgee.codetrek.cn 一个 channel 里发条消息, 然后刷新页面, 左侧 sidebar 看到这个 channel 显示未读提示. liema 复现路径: 创建第二个 channel 让 welcome 失焦 → 在 welcome 发消息 → 切到第二 channel → 切回 welcome 看到未读. 自己发的消息不该让自己未读.

## §1 根本原因 (走完代码看到的)

两个独立但互相加强的因素:

### 1.1 server `handleCreateMessage` 没更新 sender 的 last_read_at

`packages/server-go/internal/api/messages.go::handleCreateMessage` 创建消息后只做三件事:
1. 写库 `CreateMessageFull`
2. 写 event 表 `Kind: "new_message"` 给 backfill 路径
3. WebSocket `BroadcastEventToChannel("new_message", {message})` 给在线频道

**不动 `channel_members.last_read_at`**. 而 `last_read_at` 只在三处更新: 客户端 `selectChannel(channelId)` / `openDm(userId)` 调 `PUT /api/v1/channels/:id/read` 走 `handleMarkRead` → `Store.MarkChannelRead`. 也就是说: 用户在当前 channel 里发消息, 不切走也不切回, last_read_at 保持发消息**之前**的值.

### 1.2 unread_count SQL 没排除 own message

`packages/server-go/internal/store/queries.go` 5 处用 `m.created_at > COALESCE(cm3.last_read_at, 0)` 算 unread:
- L862 `ListChannelsForUser` (q == "")
- L884 `ListChannelsForUser` (q != "")
- L1115 `ListAllChannelsForAdmin`
- L1135 `GetChannelWithCounts`
- L1577 `ListDmChannelsForUser`

(L1004 / L1025 是已归档列表, hardcode `0 AS unread_count`, 不涉.)

5 处都不带 `AND m.sender_id != ?` 过滤. 因此即便 1.1 的 last_read_at 更新对了, 任何让 last_read_at 落后的边角 (race / 多设备 / 网络抖动 / 旧客户端刷新拿不到 ack) 都会让 own message 算成未读.

### 1.3 client reducer 也没排除 own message (次要)

`packages/client/src/context/AppContext.tsx::ADD_MESSAGE` (L147-182) 收到 ws frame 时, 如果 channel 不是当前选中的, `unread_count + 1`. 不检查 `message.sender_id === currentUser.id`. 实际场景中 UI 流程让用户只在当前 channel 发消息 (MessageInput 在 ChannelView 里挂), 所以 currentChannel == 发消息 channel, skip 掉 unread bump. 但: 多设备同时在线时, 设备 A 发消息广播给设备 B, 设备 B 当前在别的 channel, 设备 B 的 reducer 会把 own message 算成未读, 显示为通知泡. 不大但是同根因.

## §2 修法

按 "double belt + suspenders" 思路, 三层都修:

### 2.1 Layer 1 — server `handleCreateMessage` 顺手 mark-read sender (语义层)

`messages.go::handleCreateMessage` 在 `CreateMessageFull` 成功后, 加一行 `h.Store.MarkChannelRead(channelID, user.ID)`. 失败仅 log 不阻断 message 创建 (best-effort, 反约束: unread 是 UX, 不能挡住主流程).

**为什么 best-effort 不返错**: message 已经落库 + 广播了, 即便 last_read_at 更新失败也不要让前端拿到 5xx (前端会重试, 重复发消息更糟). Layer 2 SQL 防御兜底.

**契约**: `MarkChannelRead` 已存在 (queries.go:1311), 改 `channel_members.last_read_at = now`. sender 一定是 channel member (L234 已 gate `IsChannelMember`), 所以 update 一定影响 1 行.

### 2.2 Layer 2 — unread_count SQL 加 `m.sender_id != ?` (属性层)

5 处都加 `AND m.sender_id != ?` 在 `m.deleted_at IS NULL` 之后, 多传一个 `userID` 参数. 这是 by-construction 防御: own message 永远不可能对 sender 自己算未读, 不管 last_read_at 是什么状态.

性能: `m.sender_id` 没 index, 但 unread 子查询本来就在 channel + deleted_at 上扫小集合 (一般 channel 消息数 <1000), 加一个等值过滤不构成瓶颈. 如果未来 channel 消息上百万, 复合 index `(channel_id, deleted_at, sender_id, created_at)` 可考虑, 但不在本次修范围.

### 2.3 Layer 3 — client reducer ADD_MESSAGE 排除 own message (UX 层)

`AppContext.tsx::ADD_MESSAGE` 加 `const isOwnMessage = action.message.sender_id === state.currentUser?.id`, 然后两处 unread bump 加 `&& !isOwnMessage`. 修多设备场景的 UI flicker.

## §3 数据流

### 3.1 修复后 — 单设备同 session 发消息流

```
用户在 ChannelView 输入 → MessageInput.send()
  ↓
client api.sendMessage POST /api/v1/channels/:id/messages
  ↓
server handleCreateMessage:
  1. ACL 检查
  2. CreateMessageFull → INSERT messages
  3. [新加] MarkChannelRead(chID, userID) → UPDATE channel_members.last_read_at = now
  4. WriteEvent kind=new_message
  5. BroadcastEventToChannel new_message frame
  ↓
HTTP 200 + {message} 返回 → client api.sendMessage resolve
  ↓
ws.onmessage 收到 new_message frame (自己的, 因为同 session 也收)
  ↓
flattenWsFrame 平铺 → handleMessage 'new_message' case
  ↓
dispatch ADD_MESSAGE
  ↓
reducer:
  - 加进 messages map
  - 更新 channel.last_message_at
  - [新加] isOwnMessage 跳 unread bump
```

### 3.2 修复后 — 刷新流

```
用户刷新 → AppInner mount
  ↓
auth check + actions.loadChannels
  ↓
GET /api/v1/channels (走 ListChannelsForUser SQL)
  ↓
SQL [新加] AND m.sender_id != ? 过滤掉 own message
  ↓
unread_count = 0 (即便 last_read_at 还旧)
  ↓
sidebar 不显未读提示 ✅
```

### 3.3 多设备场景 (修后)

设备 A 发 → 设备 B (在同 user 不同 session) 收到 ws new_message frame:
- Layer 3 reducer 看 sender_id == currentUser.id → 跳 unread bump → B 也不显 ✅
- 即便 reducer 没拦, B 刷新后 SQL Layer 2 兜底 → unread 为 0

## §4 边角情况

### 4.1 离线发消息

发消息走的是 `POST /api/v1/channels/:id/messages` 同步请求. 如果发送时网络断, 客户端不会得到 200, 消息显示为 pending (`pendingMessages` 状态). 服务器永远不会收到这条, 也就不会更新 last_read_at, 也不会广播. 所以**离线发消息不会触发未读 bug** (消息根本没存到 server).

如果未来加离线 queue + 后续重传, 重传时走同一个 `handleCreateMessage`, Layer 1+2 一样兜.

### 4.2 多设备同时在线

设备 A 发完, 服务器:
1. `MarkChannelRead(chID, sender)` 更新 last_read_at = now
2. 广播 new_message 给 channel 所有 subscriber

设备 B (同 user 不同 ws connection) 收 ws frame, reducer Layer 3 拦截 own. 设备 B 视图状态正确.

如果设备 B 此时正在 channel 里看 (currentChannel == 该 channel), 收到自己的消息会插进 messages list 显示出来 (这是 desired 行为: 用户在 A 发的, B 上也要看到).

如果设备 B 此时不在该 channel, reducer 跳 unread bump, sidebar 不显未读. 跟 4.3 一致.

### 4.3 ack race: 发了 → 立即刷新但还没收 ack

理论上: 用户 click send → POST in-flight → 用户立即 reload → 浏览器丢 in-flight POST → 但 server 端 POST 已经处理完?

实际: HTTP request 的取消是浏览器告诉自己 socket 关了, 但 server-side handler 仍然跑完 (Go `http.Request` 不会自动 abort). 所以即便用户秒刷, server 端 `MarkChannelRead` 已经跑完, 刷新拉数据时 last_read_at 已经更新, Layer 1 起作用. Layer 2 也兜.

### 4.4 ws push 比 HTTP 200 快到 (or 反过来)

WS 广播跟 HTTP 200 是两个独立路径. 哪个先到都行:
- WS 先到: reducer ADD_MESSAGE (Layer 3 跳 own unread); HTTP 200 后只是 resolver 拿到完整 message struct, ADD_MESSAGE dedupe by id 跳过.
- HTTP 先到: client 用 sendMessage 返回的 message 走可选的 optimistic add (我没动这个路径); WS 后到 ADD_MESSAGE dedupe by id 跳过.

两种顺序都不会重复加 unread.

### 4.5 ack timer + REMOVE_PENDING_MESSAGE

`reconcilePendingMessages` 在 `selectChannel` 时跑, 用 `clientMessageId` 匹配 pending. 跟 unread 计算不重叠 — pending 是发送中的 client-only 状态, 不进 server unread SQL. 不影响.

### 4.6 channel 切换 vs Mark-read race

用户在 channel A 发完消息, 立刻切到 channel B. 两步 race:
- 发消息 server-side `MarkChannelRead` 跑
- selectChannel B 走 `markChannelRead(B)` (不是 A)

切到 B 不动 A 的 last_read_at. A 的 last_read_at 由发消息那条 `MarkChannelRead` 更新. 两步独立, 不 race.

### 4.7 已删除消息

unread SQL 已经过滤 `m.deleted_at IS NULL`. 加 `m.sender_id != ?` 是 AND, 不受影响. 删除自己的消息 → 删后不再算未读 (本来就如此, 没动).

### 4.8 Agent 发消息

agent 也是一种 user (有 user.ID). agent 发消息走同一个 `handleCreateMessage`, Layer 1+2 同样适用 — agent 自己看不到自己的消息算成未读. 没特殊路径.

agent 跟 owner 1:1 DM 的特殊场景: peer 是 agent, agent 发消息给 owner, owner 看. 这种情况 owner 是 receiver, sender_id != owner.ID, Layer 2 不过滤, owner 正常看到未读. 行为符合预期.

### 4.9 system message (welcome 等)

system message 在创建时通常没有人类 sender (用一个 system user ID). 用户看 system 消息算未读, 跟自己消息无关. Layer 2 `sender_id != ?` 是 user.ID != system_user_id, 不过滤, 正常算未读. 行为符合预期.

## §5 SQL 性能注意

`messages` 表索引 (没看到 schema dump 里, 但 gorm tag 提到):
- `(channel_id, created_at)` 隐含 (常用 query)
- 加了 `sender_id` 过滤后 SQLite query planner 仍走 channel_id + created_at, 在 row 集合内做 sender_id 等值检查. 一般 channel 几百到几千条消息, 不构成瓶颈.

如果将来观察到 unread 子查询慢, 可以加复合 index. 不在本次修范围.

## §6 兼容 / 反约束

- 不动 schema (无新字段, 无新表, 无 migration). v 不动.
- 不动 ws frame envelope (跟 #680 修过的 flattenWsFrame 同一种 frame 形状).
- 不动 API 形状 (`POST /messages` 仍返 `{message}`, `GET /channels` 仍返 `{channels[].unread_count}`).
- 不动客户端跟服务器的 mark-read 显式 endpoint (`PUT /channels/:id/read` 还在, 仍由 selectChannel 触发).
- 不动 Agent 特殊路径 (agent / owner DM 行为不变).
- 反约束: 不在客户端 reducer 里把 own message 当成 read 同时副作用调 server `markChannelRead` — 那会引入网络副作用进 reducer, 反 reducer 应该是 pure 函数的规矩.

## §7 测试

### 7.1 server 单测 (Go)

`packages/server-go/internal/api/messages_self_unread_test.go` (新文件):
1. `TestSelfMessageUpdatesLastReadAt` — owner 发消息 → 立刻拉 channel 列表 → unread_count == 0
2. `TestPeerMessageStillUnread` — 反向断言: peer 发消息对 owner 算未读 (Layer 2 没误伤别人消息)
3. `TestOwnMessageDoesNotMarkOtherChannel` — 反过度过滤: owner 在 channel A 发消息不影响 channel B 的 unread_count (B 里 peer 发的消息仍算未读). feima Architect review 加的, 防 Layer 2 改写跨 channel 漏过滤.

### 7.2 client 单测 (vitest)

加进现有 `__tests__/AppContext.test.tsx` 或新建 `app-context-add-message.test.tsx`:
1. 收到自己的 ws message 在 non-current channel 时不 bump unread
2. 收到别人的 ws message 在 non-current channel 时正常 bump unread
3. currentChannel == messageChannel 时不管谁发都不 bump (现有行为不变)

### 7.3 真验 (liema testing 环境)

5 步复现路径:
1. 登录 testing-borgee.codetrek.cn
2. 创建第二个 channel 让 welcome 失焦
3. 在 welcome 发消息
4. 切到第二 channel
5. 切回 welcome — sidebar 不应再显未读提示

## §8 Rule 6 docs/current 同步

server: `docs/current/server/` 没有 unread 计算的现有专门文档. handler 的 docstring 加 `// #687` 注释就够了, 不需要新建 markdown 文档.

client: `docs/current/client/` 没有 reducer 的专门文档.

不需要 docs/current 同步.

## §9 失败模式 / rollback

- 如果 Layer 1 (handleCreateMessage 加 MarkChannelRead) 跑失败, 仅 log warn, 主流程不阻断. 用户感知: 偶尔自己发消息后又看到自己未读 (Layer 2 兜底, 应该不会发生).
- 如果 Layer 2 SQL 改有语法错误, server 启动后第一个 GET /channels 就 500. 上线前有 go test 拦.
- 如果 Layer 3 reducer 改有 bug (例如 currentUser null 时), 现有断言 `state.currentUser?.id` 走 optional chaining, undefined 跟 sender_id 不等, 不 skip → 跟改前行为一致. 不引入新 crash 路径.

rollback: revert PR. 三层独立, 每层都能单独 revert 不破其它两层.

## §10 已知留账

- 多设备 last_read_at 并发场景: 设备 A 发消息时 server `MarkChannelRead` 把 last_read_at 推到 now=t1. 同时设备 B (同 user) 也在 channel A, 设备 B 已经在 t2>t1 的位置 (通过别的方式更新过 last_read_at, 比如 scroll-to-bottom 触发 mark-read). 如果服务器没做 max(now, existing) 而是直接覆盖, 可能让设备 B 的 last_read_at 退回到 t1, 引入设备 B 已读消息又显未读. 看 `Store.MarkChannelRead` 当前实现是 unconditional `Update("last_read_at", now)`, 真有这个回退风险.
  - **Layer 2 SQL 只兜 own message** (`m.sender_id != userID`): 设备 A 发的消息不会对自己未读. 但**别人的消息** race 不兜 — 比如另一个用户 X 在 t1.5 (t1 < t1.5 < t2) 发了一条消息, 设备 B 原本读到 t2 已经标记 X 这条已读, 设备 A 一发自己消息把 last_read_at 推回 t1, 设备 B 刷新 / 拉 channel 列表时 X 的消息 created_at=t1.5 > t1 → 又算未读. 这条不在 Layer 2 防御覆盖范围内.
  - **本次修不展开**, 留账. fix 方向: `MarkChannelRead` 改成 `UPDATE ... SET last_read_at = MAX(last_read_at, ?)` (SQLite `MAX(coalesce(last_read_at, 0), ?)`) 即可, 但需要单独评估 SQL 兼容性 + 加测试. 在新 followup issue 处理.
- Security 视角: 没有引入新 auth 路径. unread 信息暴露的是用户自己 channel 的元数据, 不跨 user. 不引 IDOR.

## 4 角色 review checklist

请 feima/yema/liema 同步看以下角度. 任何一 ❌ block, 我立刻改 doc + 重新征签:

- [ ] **Architect (feima)**: 三层修法是否最小手术 / 跟现有 ws envelope (flattenWsFrame, #680) 协调 / SQL 5 处改是否有遗漏 / 性能影响是否可接受 / 边角情况 §4 是否覆盖完整
- [ ] **PM (yema)**: 产品体验对不对 (自己发的不该自己未读, 别人发的该未读, 这是直觉合理的); rule 6 docs/current 真的不需要同步? 4.4 ack race / 4.6 channel 切换的 UX 边角符合预期吗
- [ ] **QA (liema)**: §7.3 testing 真验 5 步路径能跑吗; §7.1 server 单测覆盖 / §7.2 client 单测覆盖够不够; 反向 acceptance 想得到吗 (peer 发的还是要算未读)
- [ ] **Security (heima)**: §10 IDOR (unread 元数据不跨 user); agent §4.8 (agent 跟普通 user 同路径); §4.9 (system message 不引特殊 sender 路径); 反向 agent / owner DM 行为不变; 没有引入新 auth 路径.

签字格式: 任意角色 SendMessage zhanma 写 "design 687 LGTM (角色名)" 或 "design 687 NACK: 具体问题".
