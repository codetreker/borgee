# CV-4 spec brief — artifact iterate 完整流 (agent 多版本协作 orchestration)

> 飞马 · 2026-04-29 · ≤80 行 spec lock (实施视角 3 段拆 PR 由战马A 落, CV 主线收口跟 CV-3 demo 双栈)
> **蓝图锚**: [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) §1.4 ("artifact 自带版本历史: agent 每次修改产生一个版本, 人可以回滚") + §1.5 ("agent 写内容默认允许 / 创建新 artifact 默认允许") + §2 v1 做清单 ("agent 可 iterate, 再次写入触发新版本") + §3 差距 ("Agent iterate / 版本历史: 无 → 需要新表 + 写入策略")
> **关联**: 已闭 3/4 前置 — CV-1 #334+#342+#346+#348 (artifact + version + commit/rollback ✅) / RT-1 #290+#292+#296 (ArtifactUpdated frame ✅) / CV-2 #359+#360 (anchor + comment, server 进行中) / CM-4 minimal presence (`IsOnline` 接口契约 ✅, agent-lifecycle.md §0 字面); AL-4 runtime 待 (本 spec 不强依赖 AL-4 落地, 走接口存根 + AL-4 后接管真启停)
> **章程闸**: G3.4 协作场骨架 demo 双栈撑 — CV-3 是"渲染广度" (markdown/code/image), CV-4 是"协作纵深" (人 → agent iterate → 版本演进 → review 闭环)

> ⚠️ 锚说明: CV-1 已落 commit/rollback 端点 (`POST /artifacts/:id/commits` + `POST /artifacts/:id/rollback`), 但**缺 agent 触发 iterate 的请求生命周期** — 当前是 owner 手动 commit 路径, agent 无 server-side 入口. CV-4 加 iteration request 表 + 异步 orchestrate, 不重写 commit endpoint (CV-1 端点保留)

## 0. 关键约束 (3 条立场, 蓝图字面 + CV-1 边界对齐)

1. **iterate 请求是独立 entity, 不复用 messages / artifacts 表** (蓝图 §1.4 字面 "agent 每次修改产生一个版本"): `artifact_iterations` 表锁 request lifecycle (`{id, artifact_id, requested_by, intent_text, state, created_artifact_version_id NULL, error_reason NULL, created_at, completed_at NULL}`); state ENUM `('pending','running','completed','failed')` 4 态字面; **反约束**: 不在 messages 表加 `iteration_id` 列 (mention 路径走 DM-2.2 已锁, iterate 走 artifact 域内独立路径); 不在 artifact_versions 加 iteration_id 反指 (artifact_versions 是 v0 immutable append, 不动 schema)
2. **owner 触发 iterate, agent 完成时 commit 走 CV-1 既有端点 (server-side 同源)** (立场 ⑤ owner grant + 蓝图 §1.5): `POST /artifacts/:id/iterate` body `{intent_text, target_agent_id}` 创 iteration_id state='pending' → 同步 emit BPP-1 frame (跟 #304 envelope CI lint 同源, AL-4 runtime 接管前 stub 自动 fail-closed); agent runtime (AL-4 未落则用接口存根) commit artifact 时**走 CV-1 既有 `POST /artifacts/:id/commits` 端点带 query `?iteration_id=<uuid>`** server 反查回填 `created_artifact_version_id` + state='completed' 一原子事务 (反约束: 不开 `/iterations/:id/commit` 旁路 endpoint, CV-1 commit 路径单源)
3. **diff view = client 纯 markdown 行级 diff, 不裂 schema 不裂 endpoint** (蓝图 §1.4 字面 "可回滚到前一版" 隐含的版本对比): `<ArtifactPanel>` 加 "对比" tab, 走客户端 `diff` lib (jsdiff 行级, 跟 CV-1.3 markdown renderer 共组件); 加 `?diff=v3..v2` query 拼 URL (deep-link 支持); **反约束**: 不在 server 算 diff (本地 jsdiff 够, CRDT 留 v3+, 蓝图 §2 字面禁); 不存 diff 缓存 (查时即算, ≤500ms 实测够 markdown 数 KB)

## 1. 拆段实施 (CV-4.1 / 4.2 / 4.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CV-4.1** schema migration v=18 | `artifact_iterations` 表 (`id` PK / `artifact_id` NOT NULL FK / `requested_by` NOT NULL FK users / `intent_text` TEXT NOT NULL / `target_agent_id` NOT NULL FK agents / `state` TEXT NOT NULL CHECK in ('pending','running','completed','failed') / `created_artifact_version_id` INTEGER NULL FK artifact_versions / `error_reason` TEXT NULL (复用 AL-1a 6 reason 枚举字面) / `created_at` / `completed_at` NULL); 索引 `idx_iterations_artifact_id_state` (per-artifact pending/running 热路径) + `idx_iterations_target_agent` (agent 工作队列查); v=17 (CV-3.1) → v=18 双向 | 待 PR (战马A) | 战马A |
| **CV-4.2** server iterate endpoint + state machine + WS push | `POST /api/v1/artifacts/:id/iterate` (owner-only, body `{intent_text, target_agent_id}` + 校验 agent 是 channel member, INSERT iteration state='pending' + emit BPP-1 `agent_register`-同模式 control frame); CV-1 `POST /commits` 加 query `?iteration_id` 可选 — 命中则 atomic UPDATE iterations.state='completed' + created_artifact_version_id (反约束: 旧无 iteration_id 路径不破); WS push `IterationStateChangedFrame{type, cursor, iteration_id, artifact_id, channel_id, state, error_reason, created_artifact_version_id, completed_at}` 9 字段套 RT-1.1 cursor (跟 ArtifactUpdated/AnchorCommentAdded/MentionPushed 同模式 type/cursor 头位); state 转移图反断 (pending→running, pending→failed, running→completed, running→failed; 反 completed→running 等回退, server 拒) | 待 PR (战马A) | 战马A |
| **CV-4.3** client SPA iterate UI + diff view | artifact panel "请求 agent 迭代" 按钮 (channel member 可见, owner 才能触发 — 跟 CV-1 commit owner-only 同源); intent 输入框 + agent picker (channel member.kind='agent' 列表); 状态进度 inline (pending spinner / running 进度条 / failed reason badge / completed → 自动跳新版本 view); "对比 v(N) ↔ v(N-1)" tab 走 jsdiff 行级 + 蓝绿增删高亮; deep-link `?diff=v3..v2` 解 query 进对比模式 | 待 PR (战马A) | 战马A |

## 2. 与 CV-1 / CV-2 / CV-3 / RT-1 / AL-4 / DM-2 留账冲突点

- **CV-1 commit/rollback 端点复用** (核心): iterate 完成时走 CV-1 既有 `POST /commits` 加 query, **不开旁路**; rollback 仍 owner-only, 跟 iterate 互不干涉 (rollback 创新 version 但不挂 iteration_id, 跟手动 commit 同语义)
- **CV-2 锚点对话** (非冲突): iteration completed 后 owner/成员可对新 version 加锚, 走 CV-2.2 既有路径 (anchor pinned to artifact_version_id 立场 ②); 反约束: 不开 "在 iteration 上挂锚" 旁路, 锚是 artifact 域内
- **CV-3 D-lite kind 扩展** (协调): kind='code'/'image_link' 也可 iterate (intent 文本同适用); diff view 对 code 走 prism + jsdiff 行级 (CV-3.2 CodeRenderer 共组件), 对 image_link 走前后缩略图并排 (jsdiff 不适用)
- **RT-1 ArtifactUpdated frame**: iteration completed 时 commit 路径已 emit ArtifactUpdated, 跟 IterationStateChangedFrame 共一根 hub.cursors 单调 (反约束: 两 frame 不发同 version 重复, AcrtifactUpdated 由 commit 自然 emit 一次)
- **AL-4 runtime 接管** (非阻塞): CV-4.2 emit 的 BPP-1 frame 走 #304 whitelist; AL-4 未落时 runtime stub 永远 fail-closed (state='failed', error_reason='runtime_not_registered' 跟 AL-1a 6 reason 同源); AL-4 落地后 runtime 真接管 commit 路径 — CV-4 不锁 AL-4 顺序
- **DM-2.2 mention** (非冲突): iterate 不走 mention 路径 (mention 是 channel 协作通信, iterate 是 artifact 域内编辑请求); 反约束: client 不混排 (iterate 是 artifact panel 内 button + WS frame, 不进 messages 流)
- **v=14/15/16/17/18 sequencing 锁** (字面延续): CV-2.1 v=14 ✅ / DM-2.1 v=15 ✅ / AL-4.1 v=16 待 / CV-3.1 v=17 待 / **CV-4.1 v=18** (本 spec)

## 3. 反查 grep 锚 (Phase 3 续作 / Phase 4 验收)

```
git grep -nE 'CREATE TABLE.*artifact_iterations'              packages/server-go/internal/migrations/   # ≥ 1 hit (CV-4.1)
git grep -nE 'POST /api/v1/artifacts/.*\/iterate'             packages/server-go/internal/api/          # ≥ 1 hit (CV-4.2 endpoint)
git grep -nE 'IterationStateChangedFrame\{|type.*iteration_state_changed' packages/server-go/internal/ws/   # ≥ 1 hit (envelope 锁)
git grep -nE 'iteration_id.*[?&]|query.*iteration_id'         packages/server-go/internal/api/          # ≥ 1 hit (CV-1 commit 复用)
git grep -nE 'jsdiff|diffLines'                               packages/client/                          # ≥ 1 hit (CV-4.3 client diff)
# 反约束 (5 条 0 hit)
git grep -nE 'POST.*\/iterations\/.*\/commit|iteration_commit_endpoint' packages/server-go/internal/api/   # 0 hit (立场 ② 不开旁路 commit)
git grep -nE 'messages.*iteration_id|ALTER TABLE messages.*iteration' packages/server-go/internal/migrations/   # 0 hit (立场 ① 不污染 messages)
git grep -nE 'ALTER TABLE artifact_versions ADD|artifact_versions.*iteration_id' packages/server-go/internal/migrations/   # 0 hit (立场 ① 不动 artifact_versions)
git grep -nE 'completed.*-> *running|state.*backward|UPDATE.*iterations.*state.*pending.*WHERE.*completed' packages/server-go/internal/api/   # 0 hit (state machine 不回退)
git grep -nE 'serverDiff|computeDiff.*server|\/api\/v1\/diff'  packages/server-go/internal/api/         # 0 hit (立场 ③ server 不算 diff)
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束)

- ❌ CRDT 多人实时编辑 (蓝图 §2 字面 "CRDT 巨坑"; iterate 仍是顺序 append, 一人一锁)
- ❌ iterate 取消 / pause / resume (留 v3+, 失败重试由 owner 重新触发新 iteration)
- ❌ iterate 历史聚合视图 ("我的 iteration 列表") — Phase 5+
- ❌ multi-agent 协作 iterate (一 iteration = 一 target_agent_id; 多 agent 协作留 v3+)
- ❌ iterate intent 模板 / 预设 prompt — Phase 5+
- ❌ server 端 diff 算法 (立场 ③ client jsdiff 够, server diff 留 CRDT 路径 — 不做)
- ❌ admin SPA iteration god-mode (admin 不入 channel, ADM-0 §1.3 红线; intent_text 含 user 输入不返回 admin)
- ❌ iterate 跨 artifact (一 iteration 锁单 artifact_id, batch iterate 留 v3+)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- CV-4.1: migration v=17 → v=18 双向 + state CHECK reject 'unknown' + idx 双索引 hit + FK cascade (artifact_version 删 → iterations.created_artifact_version_id 置 NULL)
- CV-4.2: owner 触发 iterate 200 (非 owner 403) + agent 非 channel member reject 400 + state machine 反向 (completed → running reject) + commit `?iteration_id` 命中 atomic UPDATE + WS IterationStateChangedFrame 9 字段顺序 byte-identical (跟 ArtifactUpdated/AnchorCommentAdded/MentionPushed 同模式) + AL-4 stub fail-closed reason='runtime_not_registered'
- CV-4.3: e2e 触发 iterate + state inline 进度 (pending spinner → completed 自动跳新版本) + diff tab 蓝绿高亮 + deep-link `?diff=v3..v2` 进对比 + image_link kind 走前后缩略图并排 (jsdiff fallback)
