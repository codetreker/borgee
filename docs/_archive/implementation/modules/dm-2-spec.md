# DM-2 spec brief — mention (@user / @agent) 实施 spec

> 飞马 · 2026-04-28 · ≤80 行 spec lock (实施视角 3 段拆 PR 由战马B 落)
> **蓝图锚**: [`concept-model.md`](../../blueprint/current/concept-model.md) §4 (agent 代表自己 — mention 只 ping target, 不抄送 owner) + §4.1 (离线 fallback — owner 系统 DM + 节流 5 分钟/channel + ❌ 不转发原始内容) + §13 隐私默认
> **关联**: 烈马 [`docs/qa/acceptance-templates/dm-2.md`](../../qa/acceptance-templates/dm-2.md) (#293 LGTM merged, 11 验收 + 反查锚) + 野马 #211 ADM-0 反查表 (mention 按 sender_id 不展开 owner) + AL-3 #277/#310 PresenceTracker (在线判断真接, 不再 stub)

> ⚠️ 锚说明: `@channel` 不在 DM-2 范围 (留 DM-3), 此 spec 只锁 `@<user_id>` / `@<agent_id>` (同表同语义, 立场 ⑥ agent=同事)

## 0. 关键约束 (3 条立场, 蓝图字面 + #293 acceptance)

1. **parse `@<id>` ↔ display_name 拆死, raw UUID 不进 DOM 文本节点**: server 端 body 存 `@<user_id>` token 原样 (parser 抓; 持久化到 `message_mentions` 表); client 渲染时按 `users.display_name` 替换文本节点, raw UUID 仅留 `data-mention-id` attr (反约束: DOM 文本节点 grep raw UUID count==0, 立场对齐 #293 §3.1)
2. **在线 ping target, 离线 fallback owner DM, 不转发原 body** (蓝图 §4 + §4.1 字面): \`presenceTracker.IsOnline(target_id)\` 真接 AL-3 #310 (不再 stub 永远 false placeholder); 离线触发 owner system DM 文案 `{agent_name} 当前离线，#{channel} 中有人 @ 了它，你可能需要处理` byte-identical (#293 §2.2 已锁); **反约束**: system DM payload 仅 `{agent_name}` + `{channel_name}` 占位, 不含 raw body 字符串 (隐私 §13)
3. **mention 路由按 sender_id, 永不写 target_user_id = agent.owner_id** (除 §4.1 offline fallback 走独立 `type=system` message): \`message_mentions(message_id, target_user_id)\` 一行一目标 + dedup 同 target; **反约束**: \`grep mention.*owner_id\` count==0 (立场 ⑥ agent=同事, owner 不被 cc)

## 1. 拆段实施 (DM-2.1 / 2.2 / 2.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **DM-2.1** schema migration v=14 | `message_mentions` 表 (`id` PK / `message_id NOT NULL FK` / `target_user_id NOT NULL FK users.id` / `created_at`); UNIQUE(message_id, target_user_id) (dedup 同 target); 索引 `idx_message_mentions_target_user_id` (mention 路由热路径); migration v=13 (CV-1.1) → v=14 双向 | 待 PR (战马B) | 战马B |
| **DM-2.2** server parser + push + offline fallback | parser regex `@([0-9a-f-]{36})` 抓 token 写 `message_mentions` (人 / agent 同表同语义, 立场 ⑥); WS push `MentionFrame` 到 target (在线 = `IsOnline(target_id)==true`, 走 #310 SessionsTracker); 离线 → fanout owner system DM 文案锁 (#293 §2.2 byte-identical) + 节流 5min/(agent, channel) (clock fixture, 跟 G2.3 节流模式同) | 待 PR (战马B) | 战马B |
| **DM-2.3** client SPA mention 渲染 | textarea `@` 触发候选列表 (channel members 人+agent 同列, agent 🤖 badge, 立场 ⑥); 选中回填 `@<user_id>` token (非 display_name, 防同名歧义); 消息流渲染 token → `<span data-mention-id="...">@{display_name}</span>` 蓝色高亮; **离线 agent UI 无任何提示** (#293 §3.3 反向断言, fallback 是 owner 后台事不污染发送方) | 待 PR (战马B) | 战马B |

## 2. 与 RT-1 / CHN-1 / AL-3 / CV-1 留账冲突点

- **RT-1 cursor 复用**: `MentionFrame` 套 #237 envelope + RT-1.1 #290 cursor 单调发号; 不另起 mention-only 推送通道 (走 `/ws` hub 同路径)
- **CHN-1 channel 范围**: mention 仅在 channel 内 (`message_mentions.message_id` FK `messages.id`, message 已挂 channel_id); 不允许跨 channel mention
- **AL-3 IsOnline 真接 #310**: DM-2.2 fallback 触发条件 = `!presenceTracker.IsOnline(target_id)` 走 \`SessionsTracker.IsOnline\` (人 走 user_id 路径, agent 走 agent_id 路径, 跟 #310 OR 查询路径字面对齐); 跨 org agent → #310 §4 默认 false (隐私), 触发 fallback 不报错
- **CV-1 不依赖**: mention 走 message body, 跟 artifact 不交叉 (artifact 的 `@` 引用走 §1.4 不同路径, 留 v2)
- **AP-0 不变**: mention 路由不需要新 perm (channel member 可发 message → 自然可 mention 同 channel 任何 member, 立场 ⑥)

## 3. 反查 grep 锚 (Phase 4 验收)

```
git grep -nE 'message_mentions.*UNIQUE'        packages/server-go/internal/migrations/   # ≥ 1 hit (DM-2.1)
git grep -nE '当前离线，#.*中有人 @ 了它'        packages/server-go/internal/api/messages.go # ≥ 1 hit (DM-2.2 文案锁)
git grep -nE 'mention.*owner_id|owner_id.*mention' packages/server-go/internal/api/messages.go # 0 hit (反约束 立场 ③ 不抄送 owner)
git grep -nE 'cc.*owner|notify.*owner_id'      packages/server-go/internal/api/messages.go # 0 hit (反约束 #293 §4.a)
git grep -rnE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' packages/client/src/components/MessageList.tsx # 0 hit (反约束 立场 ① raw UUID 不进文本节点, 仅 data-* attr)
git grep -rnE '@channel\b'                     packages/server-go/ packages/client/src/   # 命中必带 `TODO: DM-3` 或 `unsupported` 注释 (#293 §4.b)
```

任一 0 hit (除反约束行) → CI fail, 视作蓝图 §4 / §4.1 立场被弱化.

## 4. 不在本轮范围 (反约束)

- ❌ `@channel` 全员 fanout (留 DM-3, #293 §4.b 锚)
- ❌ mention 搜索 / 历史聚合视图 (\"我被 @ 列表\") — 留 Phase 5+
- ❌ batch mention (单 message 多 `@` 已支持, 但不做批量发起 / template) — Phase 5+
- ❌ mention 撤回 / edit (蓝图无, message edit 本身留账 Phase 5+)
- ❌ admin SPA mention god-mode (admin 不入 channel, ADM-0 §1.3 红线; mention 路由本身不需 admin 路径)
- ❌ mention 跨 org 邀请审批 (走 §4.2 `agent_invitations` 表, ADM-1/CHN-2 落地, DM-2 仅锁 mention 路由本身 — #293 退出条件字面)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- DM-2.1: migration v=13 → v=14 双向 + UNIQUE(message_id, target_user_id) 反向 (重复 mention 同 target reject) + 反约束 owner_id 列不存在
- DM-2.2: parser table-driven (人/agent/混合/无 mention/重复 dedup) + IsOnline true → WS push only (owner sniff 0) + IsOnline false → fallback DM + 5min 节流 (clock fixture) + 反约束 system DM body 不含 raw message body 字符串
- DM-2.3: e2e textarea `@` 候选 + 渲染 display_name + 反向 DOM raw UUID grep + 离线 agent 发送方 UI 无提示 (#293 §3.3 反向断言)
