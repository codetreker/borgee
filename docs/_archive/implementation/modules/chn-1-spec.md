# CHN-1 spec brief — 蓝图锁配套战马A #265 拆段

> 飞马 · 2026-04-28 · ≤80 行 spec lock (实施视角 #265 拆 PR 由战马A 落)
> **蓝图锚**: [`channel-model.md`](../../blueprint/current/channel-model.md) §1.1 (Channel = 协作场) + §1.4 (作者分组 + 个人微调) + §2 (关键不变量); [`concept-model.md`](../../blueprint/current/concept-model.md) §1.4 (主体验 — 团队感知 + DM 对话)
> **关联**: 战马A #265 (CHN-1 拆段实施) + #276 (CHN-1.1 schema migration v=11)

> ⚠️ 锚说明: 业主原派活引 channel-model.md §1.6, 现行蓝图章节只到 §1.4 + §2 不变量, 此处按字面立场对齐 §1.4 (作者分组) + §2 (不变量), 不重新编号蓝图。

## 0. 关键约束 (3 条立场, 蓝图字面)

1. **Channel default creator-only**: 新建 channel 默认仅创建者可见; 邀请扩散走 invitation (#237 envelope) — 不允许 org 全员自动 join (蓝图 §1.1 + §2 不变量 #1)
2. **Agent silent join**: agent 进 channel 不发 system message, `silent=true` 列锁; 人类成员加入仍发 system DM fanout (蓝图 §1.4 隐式 + concept-model.md §1.4 主体验)
3. **Archived not deleted**: channel 退役走 `archived_at` 列 (`INTEGER NULL`, NULL=活 / 非NULL=归档时间戳), 历史 artifact / message 保留可回溯; DROP TABLE 路径禁 (蓝图 §2 不变量 #3)

## 1. 拆段实施 (CHN-1.1 / 1.2 / 1.3, 与 #265 一致)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CHN-1.1** schema migration v=11 | `channels` 表 rebuild + `archived_at` / `silent` / `org_id_at_join` 列 + UNIQUE(org_id, name) | #276 (LGTM) | 战马A |
| **CHN-1.2** API (create / archive / list) | `POST /channels` 默认 creator-only + `POST /channels/:id/archive` 软退役 + `GET /channels` 过滤 archived | 待 PR (战马A) | 战马A |
| **CHN-1.3** client (创建 / 归档 UI) | 创建对话框 + archived 灰显 + agent silent 不打扰 | 待 PR (战马A) | 战马A |

## 1.4 spec 漂移记录 (#288 后置补丁)

- **WS `channel_added` payload schema 锁**: 必须送 full `{channel}` 对象 (含 `archived_at` / `members` count / `silent` flags), **不只** `{channel_id}` — #288 (commit adaf521) 抓出 sidebar crash 链 (客户端拿 channel_id 反查 store, store 未 backfill → undefined.name 崩), 战马A 同 PR fix server `channels.go` + `queries.go` 改送 full channel + 客户端 `useWebSocket.ts` 加 undefined guard 双层防护
- **客户端 handler 反约束**: `channel_added` / `channel_archived` handler 必须对 undefined channel guard (drop frame + warn), 不允许直接 dereference — 防 server schema 回潮窄 payload 导致 sidebar 再炸

## 2. 与 Phase 2 留账无冲突

- **G2.5 presence contract** (留账 #277): channel 成员表 `channel_members` 与 presence 表独立, 不冲突
- **G2.6 BPP envelope lint** (留账 #274): channel CRUD 走 REST, 不进 `/ws` envelope, 不触 BPP frame schema
- **AL-3 (Phase 4)**: presence 按 channel 分桶留 AL-3 占号 PR, CHN-1 不前置 AL-3

## 3. 反查 grep 锚 (Phase 4 验收)

```
git grep -n 'archived_at.*INTEGER'    packages/server-go/internal/migrations/  # ≥ 1 hit (CHN-1.1)
git grep -n 'silent.*INTEGER'         packages/server-go/internal/migrations/  # ≥ 1 hit (CHN-1.1, SQLite 无 BOOL)
git grep -n 'UNIQUE.*org_id.*name'    packages/server-go/internal/migrations/  # ≥ 1 hit (CHN-1.1, 列序对齐 #276)
git grep -n 'archived_at IS NULL'     packages/server-go/internal/channels/    # list 过滤 (CHN-1.2)
git grep -n 'channel_added'           packages/client/src/hooks/useWebSocket.ts # ≥ 1 hit + 同 handler 必含 undefined guard (`if (!msg.channel)` 等价) — 防 #288 sidebar crash 回归
```

任一 0 hit → CI fail, 视作蓝图立场被弱化。

## 4. 不在本轮范围 (反约束)

- ❌ 多 org 共享 channel (跨 org 关联, 留 Phase 5+)
- ❌ Channel template / preset (留 Phase 4+ 业主反馈后)
- ❌ Channel hierarchy (父子 channel / sub-channel, 不在 v1 蓝图)
- ❌ Per-channel notification policy (走 AL-1b busy/idle 全局, 不下沉)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- CHN-1.1: migration v=10 → v=11 双向 + UNIQUE 反向 (重名 reject)
- CHN-1.2: creator-only 反向断言 (非创建者 list count=0) + archive 后 list filter
- CHN-1.3: e2e 创建 + archive 灰显截屏 (野马 G2.4 子签可选)
