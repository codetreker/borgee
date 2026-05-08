# DM-2.2 spec brief — mention server parser + WS push + offline fallback

> 飞马 · 2026-04-29 · ≤80 行 spec lock (DM-2.1 #361 schema 已 merge, 续作 DM-2.2 server)
> **蓝图锚**: [`concept-model.md`](../../blueprint/current/concept-model.md) §4 (mention 只 ping target 不抄 owner) + §4.1 (离线 fallback owner system DM, ❌ 不转发原 body, 5min/(agent,channel) 节流) + §13 隐私默认
> **关联**: 上游 DM-2.1 #361 (战马B 实施 message_mentions v=15 已 merge) + DM-2 spec brief #312 (飞马, merged 7de76f9) + acceptance template #293 (烈马, merged) + 文案锁 #314 (野马, merged) + AL-3 #310 PresenceTracker.IsOnline (真接, 不再 stub) + RT-1.1 #290 envelope cursor 单调
> **章程闸**: G3.4 协作场骨架 demo 隐含依赖 (mention 在 channel 协作场是核心 UX 路径之一)

> ⚠️ 锚说明: DM-2 spec brief #312 §1 把 v=14 写成 schema 号已过期 — 真 sequencing CV-2.1 抢 v=14 / DM-2.1 #361 顺延 v=15 (字面落); DM-2.2 不动 schema, 仅 server parser + WS push + offline fallback, 不抢号

## 0. 关键约束 (3 条立场, 蓝图字面 + #312/#293/#314 byte-identical)

1. **parser regex `@([0-9a-f-]{36})` 抓 token 写表 (人/agent 同表同语义, 立场 ⑥)** (#312 立场 ① + #314 文案锁): `internal/api/messages.go` POST /messages handler 解 body 抓所有 UUID-shaped mention token → INSERT `message_mentions(message_id, target_user_id, created_at)` (UNIQUE (message_id, target_user_id) 自动 dedup, schema 层 #361 已锁); **反约束**: 不解 `@<display_name>` (防同名歧义, 客户端 textarea 候选回填只填 `@<user_id>` token, #312 立场 ①)
2. **在线 ping target → WS push only; 离线 → owner system DM fallback + 5min 节流** (蓝图 §4 + §4.1 字面): `presenceTracker.IsOnline(target_id)` 真接 #310 SessionsTracker (人 走 user_id, agent 走 agent_id 同 OR 查询路径); `IsOnline==true` → push `MentionPushedFrame` 到 target 一人 (cursor 走 hub.cursors 同 RT-1.1 单调, 反约束: 不另起 channel); `IsOnline==false` → INSERT system `messages(type='system', body=<文案锁>)` 到 owner DM channel + 节流键 `(agent_id, channel_id)` 5min (clock fixture, 跟 G2.3 节流模式同源)
3. **system DM 文案锁字面 byte-identical** (#293 §2.2 + #314 锁): `{agent_name} 当前离线，#{channel} 中有人 @ 了它，你可能需要处理` — payload **仅** `{agent_name}` + `{channel_name}` 占位, **不含** raw message body 字符串 (隐私 §13 红线); **反约束**: `grep raw body 字符串拼接` count==0; system message 走 `messages.type='system'` 同既有 system message 路径 (CHN-1 onboarding welcome 同模式), 不开新 endpoint

## 1. 拆段实施 (单 PR, ~3 文件)

| 文件 | 范围 |
|---|---|
| `internal/api/messages.go` | POST /messages handler 增 parse mention 段 (regex `@([0-9a-f-]{36})` table-driven 抓 token, dedup 同 message 多 token); 校验 target 是 channel member (反约束: cross-channel mention reject 400 `mention.target_not_in_channel`); INSERT `message_mentions` rows; 调 `dispatchMention(targetID, agentID)` 同步 (route 在线 → push / 离线 → fallback) |
| `internal/api/mention_dispatch.go` (新) | `dispatchMention` 函数: `IsOnline(target)` true → `hub.PushMentionPushed(target, frame)`; false → `enqueueOwnerSystemDM(agent_id, channel_id, agent_name, channel_name)` (节流键 `(agent_id, channel_id)` 5min, 用 in-memory `mentionFallbackThrottle` map + `sync.Mutex` 跟 AL-1a 节流同模式 — Phase 5+ 才考虑 Redis); 文案 `fmt.Sprintf` 锁字面常量 `MentionFallbackTemplate` 不裂 |
| `internal/ws/mention_pushed_frame.go` (新) | `MentionPushedFrame{type, cursor, message_id, channel_id, sender_id, mention_target_id, body_preview, created_at}` 8 字段 byte-identical 跟 ArtifactUpdated/AnchorCommentAdded 同模式 (type/cursor 头位 共 hub.cursors 单调 sequence); `body_preview` 截断前 80 字符 (反约束: client 拿不到完整 body — 隐私; 完整 body 走 fetch /messages 走 channel ACL 同源); BPP-1 #304 envelope CI lint 自动闸 |

**owner**: 战马 (战马B 已 #361 落 schema, 续作可同 owner; 也可派战马A 接, team-lead 拍)

## 2. 与 RT-1 / CHN-1 / AL-3 / CM-4 留账冲突点

- **RT-1 cursor 复用** (非冲突): `MentionPushedFrame` 套 #237 envelope + RT-1.1 #290 cursor 单调发号, 走 `/ws` hub 同路径; 不另起 mention-only 推送通道
- **CHN-1 channel ACL** (非冲突): mention target 必须是 channel member (mention_dispatch 前置校验), 反约束 cross-channel mention reject 400 `mention.target_not_in_channel`; 跟 anchor cross-channel 403 同源拒绝模式
- **AL-3 IsOnline 真接 #310** (核心耦合): `presenceTracker.IsOnline(target_id)` 走 SessionsTracker.IsOnline OR 查询路径 (人 user_id / agent agent_id 共一根); 跨 org agent → #310 §4 默认 false (隐私), 自动 fallback 不报错
- **CM-4 system message 复用**: fallback owner DM 走 `messages.type='system'` 同 CHN-1 onboarding welcome / agent invitation system 行同源, 不开新 endpoint / 新表
- **ADM-0 god-mode**: admin 不入 channel (§ADM-0 红线); admin 看 message_mentions 元数据可见 (target_user_id 时间戳), 不能反推 fallback 路径 (schema #361 已无 owner_id 列)

## 3. 反查 grep 锚 (Phase 3 续作 / Phase 4 验收)

```
git grep -nE 'message_mentions.*INSERT|INSERT.*message_mentions' packages/server-go/internal/api/   # ≥ 1 hit (DM-2.2 parser 写表)
git grep -nF '当前离线，#'                              packages/server-go/internal/api/             # ≥ 1 hit (#293 §2.2 + #314 文案锁)
git grep -nE 'MentionPushedFrame\{|type.*mention_pushed' packages/server-go/internal/ws/             # ≥ 1 hit (envelope 锁)
git grep -nE 'IsOnline\(.*target.*\)|IsOnline\(targetID' packages/server-go/internal/api/             # ≥ 1 hit (立场 ② AL-3 真接, 反 stub-always-false)
# 反约束 (4 条 0 hit)
git grep -nE 'mention.*owner_id|owner_id.*mention|cc.*owner|notify.*owner_id' packages/server-go/internal/api/   # 0 hit (立场 ③ 不抄 owner)
git grep -nE 'system.*DM.*body|fmt.Sprintf.*%s.*body|body_preview.*system_dm' packages/server-go/internal/api/   # 0 hit (立场 ③ system DM 不含 raw body)
git grep -nE 'parseMention.*display_name|@\$\{.*display' packages/server-go/internal/api/                       # 0 hit (立场 ① parser 不解 display_name token)
git grep -nE 'cross.*channel.*mention.*allow|skipChannelCheck.*mention' packages/server-go/internal/api/         # 0 hit (反约束 cross-channel mention reject)
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束)

- ❌ `@channel` 全员 fanout (留 DM-3, #293 §4.b 锚)
- ❌ mention 撤回 / edit (蓝图无, 跟 message edit 一起留 Phase 5+)
- ❌ mention 历史聚合 (\"我被 @ 列表\") — Phase 5+
- ❌ mention 跨 org 邀请审批 (走 §4.2 `agent_invitations`, ADM-1/CHN-2 落地)
- ❌ system DM 节流持久化 (in-memory 足, Redis 留 Phase 5+)
- ❌ admin SPA mention god-mode (§ADM-0 红线, mention 路由不需 admin 入)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- parser table-driven (人 / agent / 混合 / 无 mention / 重复 dedup / cross-channel target reject)
- IsOnline true → MentionPushedFrame push only, owner system DM 不触发 (反向 sniff messages.type='system' count==0)
- IsOnline false → fallback system DM 触发 + body 字面 byte-identical (#293/§314 文案锁 t.Errorf 显式断 raw body 不混入)
- 5min 节流 (clock fixture: 第 1 次触发 / +1min 不触发 / +5min1ms 第 2 次触发, 跟 G2.3 / AL-1a 节流模式同)
- MentionPushedFrame 8 字段顺序 byte-identical (跟 ArtifactUpdated/AnchorCommentAdded 同模式 type/cursor 头位)
- BPP-1 #304 envelope CI lint 自动闸 + body_preview 80 字符截断反向断言
