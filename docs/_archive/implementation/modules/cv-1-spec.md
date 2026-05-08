# CV-1 spec brief — canvas vision artifact (Phase 3 v1 解封后第一波 spec)

> 飞马 · 2026-04-28 · ≤80 行 spec lock (实施视角 3 段拆 PR 由战马A 落)
> **蓝图锚**: [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) §0 (一句话 — channel 围 artifact 协作) + §1.1-§1.6 (五条立场 — D-lite + workspace per channel + Markdown ONLY v1) + §2 (v1 做/不做)
> **关联**: 野马 [`docs/qa/cv-1-stance-checklist.md`](../../qa/cv-1-stance-checklist.md) v0+v0.1 (7 立场 + 5 黑名单 grep + v0/v1 切换三条件) + 飞马 #295 v1 transition 三条件; RT-1 三段闭环 (#290+#292+#296) ✅; AL-3 三轨闭环 (#301+#302+#303+#305) ✅; BPP-1 envelope CI lint #304 ✅ (G2.6 真落)

> ⚠️ 锚说明: v1 解封三条件 (RT-1 ✅ + AL-3 ✅ + BPP-1 lint ✅) Phase 3 解封后已全满足, 此 spec 直开 v1 实施视角拆段, 不再走 v0 占号路径

## 0. 关键约束 (3 条立场, 蓝图字面 + v1 解封后)

1. **artifact 归属 = channel, 不是 author** (野马立场 ①): `artifacts.channel_id NOT NULL` 唯一归属键, 无 `owner_id` 主权列; channel 软删 → artifact 软删随; **反约束**: 不跨 channel 共享, 不 author-owned (author 离开不带走)
2. **Markdown ONLY v1** (野马立场 ④, 蓝图 §2 \"v1 做\" 字面锁): `artifacts.type` CHECK = `'markdown'` 唯一枚举; 代码片段 / 图片 / PDF / 看板 留 v2+; 上传非 markdown → 走老 `workspace_files` 附件路径 (与 artifact 分轨, 不混)
3. **ArtifactUpdated 套 #237 envelope** (野马立场 ⑤, 蓝图 realtime §2.3 + RT-1.1 #290 锁): `ArtifactUpdated{cursor, artifact_id, version, channel_id, updated_at, kind}` 字段名/顺序 byte-identical 于 RT-0/RT-1.1, 走 BPP-1 #304 envelope CI lint 自动闸; **反约束**: 不自造 envelope, 不 client timestamp 排序

## 1. 拆段实施 (CV-1.1 / 1.2 / 1.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CV-1.1** schema migration v=13 | `artifacts` 表 (`id` / `channel_id NOT NULL FK` / `type CHECK='markdown'` / `title` / `body` / `current_version` / `created_at` / `archived_at NULL`); `artifact_versions` 表 (`artifact_id FK` / `version` / `body` / `committer_kind CHECK in ('agent','human')` / `committer_id` / `created_at`); 索引 `idx_artifacts_channel_id` + `UNIQUE(artifact_id, version)`; migration v=12 (AL-3.1) → v=13 双向 | 待 PR (战马A) | 战马A |
| **CV-1.2** server API + WS push | `POST /channels/:id/artifacts` 创 (默认 channel 成员可见, 跟 channel 权限继承) + `PATCH /artifacts/:id` 编辑 (单文档锁 30s TTL + conflict 409, 立场 ②) + `POST /artifacts/:id/rollback` owner-only 触发新 commit (立场 ⑦) + WS push `ArtifactUpdated` frame 套 RT-1.1 cursor 单调 (立场 ⑤); agent commit fanout system message `\"{agent_name} 更新 {artifact_name} v{n}\"` (立场 ⑥) | 待 PR (战马A) | 战马A |
| **CV-1.3** client SPA canvas UI | channel 内 `Workspace` tab (跟 chat 平级, 蓝图 §1.3 \"channel 自带\"); markdown 编辑器 + `<MarkdownPreview>` 渲染; 版本列表侧栏 (线性, 立场 ③) + 点 \"回滚到此版本\" → 触发 rollback POST (立场 ⑦); WS `ArtifactUpdated` 实时刷; conflict 409 toast \"内容已更新, 请刷新查看\" | 待 PR (战马A) | 战马A |

## 2. 与 RT-1 / AL-3 / CHN-1 留账冲突点

- **RT-1 cursor 复用** (非冲突): CV-1.2 ArtifactUpdated 走 RT-1.1 #290 \`cursor\` 单调发号 + RT-1.2 #292 client backfill — 共用 \`/ws\` hub + \`/api/events?since=N\` 路径, 不另起
- **CHN-1 channel_id FK 复用**: `artifacts.channel_id` FK 锁 CHN-1.1 #276 \`channels.id\`; channel \`archived_at\` → CV-1.2 list 过滤继承 (artifact 不 surface 已归档 channel 下)
- **AL-3 不依赖**: artifact 归属 channel 非 agent, presence 状态变更不触发 artifact 推送; CV-1 单文档锁 ②.v1 切换才挂 AL-3 presence (lock-holder 头像 + 在线状态, 留 v2+)
- **BPP-1 #304 envelope CI lint 接管**: ArtifactUpdated frame 走 #304 反射闸位自动 lint, 飞马人工 lint 闸位卸任

## 3. 反查 grep 锚 (Phase 4 验收)

```
git grep -n 'artifacts.*channel_id'            packages/server-go/internal/migrations/   # ≥ 1 hit (CV-1.1)
git grep -nE "type.*CHECK.*'markdown'"          packages/server-go/internal/migrations/   # ≥ 1 hit (CV-1.1 立场 ④ 唯一枚举)
git grep -nE 'ArtifactUpdated\{[^}]*cursor'     packages/server-go/internal/ws/           # ≥ 1 hit (立场 ⑤ 字段顺序锁)
git grep -nE 'artifacts.*owner_id|workspace_artifacts.*owner_id' packages/server-go/internal/store/ # 0 hit (反约束 立场 ① 无 owner_id 主权列)
git grep -nE "artifact\.type.*=.*\"(code|image|pdf|kanban)\"" packages/server-go/internal/ # 0 hit (反约束 立场 ④ Markdown ONLY v1)
git grep -nE 'yjs|automerge|y-protocols'        packages/client/ packages/server-go/      # 0 hit (反约束 立场 ② 不上 CRDT)
git grep -nE 'rollback.*RequirePermission.*[^o]wner' packages/server-go/internal/server/   # 0 hit (反约束 立场 ⑦ owner-only)
```

任一 0 hit (除反约束行) → CI fail, 视作蓝图立场被弱化 / 跟 RT-1 envelope 边界混淆.

## 4. 不在本轮范围 (反约束)

- ❌ 段落锚点对话 (蓝图 §2 \"v1 不做\", v2+; 立场 ⑥ 仅锁 v2 形态)
- ❌ CRDT 实时多人编辑 (蓝图 §2 显式不做, 立场 ②; v1 走单文档锁 30s TTL)
- ❌ 多 artifact 关联视图 / 拖拽连线 / 跨 channel 共享 (v2+)
- ❌ 代码 / 图片 / PDF / 看板 artifact 类型 (立场 ④, v2+)
- ❌ 删除中间版本 / 版本 GC 策略 (立场 ③ v0 不限期, v2+)
- ❌ admin 看 artifact body (走 god-mode endpoint 不返回 body, ADM-0 §1.3 红线)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- CV-1.1: migration v=12 → v=13 双向 + UNIQUE(artifact_id, version) 反向 + type CHECK='markdown' reject 反向 (插非 markdown → reject)
- CV-1.2: 30s TTL 锁 conflict 409 (clock fixture, 跟 G2.3 节流模式同) + rollback owner-only 反断 (非 owner → 403) + ArtifactUpdated envelope byte-identical 跟 RT-1.1 #290 (反向 grep 字段顺序) + agent commit system message 文案锁
- CV-1.3: e2e markdown 编辑 + 版本列表 + rollback owner-only UI + WS 实时刷新 (跟 RT-1.2 #292 reconnect→backfill 同 e2e 模式) + conflict 409 toast
