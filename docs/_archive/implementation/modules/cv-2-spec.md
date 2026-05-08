# CV-2 spec brief — 锚点对话 (anchor comments on artifacts)

> 飞马 · 2026-04-29 · ≤80 行 spec lock (实施视角 3 段拆 PR 由战马A 落)
> **蓝图锚**: [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) §1.4 (artifact 集合) + §1.6 (锚点对话 = owner review agent 产物的工具) + §2 v1 不做清单第 5 条 (\"段落锚点对话, v2 加\")
> **关联**: CV-1 三段四件全闭 ✅ (#334+#342+#346+#348) — artifact + version 表 + Markdown ONLY + RT-1 envelope 接管已就位; RT-1 #290 cursor 单调 envelope 复用; CHN-1 #276 channel 权限继承; AL-3 不依赖
> **章程闸**: G3.2 锚点对话 E2E 强依赖 — Phase 3 退出公告硬约束

> ⚠️ 锚说明: CV-1 v1 故意把锚点对话留 v2; 用户 2026-04-29 拍板严守 Phase 3 章程, CV-2 升格为 Phase 3 续作第一波 (野马 ⭐ 签字 milestone, CV-1 后顺位)

## 0. 关键约束 (3 条立场, 蓝图字面 + CV-1 边界对齐)

1. **锚点 = 人审 agent 产物 (人机界面, 非 agent 间通信)** (蓝图 §1.6 字面锁): 锚点对话**仅** owner / channel 成员对 agent commit 产物的 review 工具; agent 之间互相通信走普通 channel message + artifact `@` 引用, **不挂锚点**; **反约束**: 不开 \"agent 互发锚点\" 路径 (避免 AI 自跟 AI 锚点对话的诡异场景)
2. **锚点挂 artifact_version 不挂 artifact** (CV-1 立场 ③ 线性版本承袭): 一个锚点 thread 锁死在创时的 `artifact_version_id` + `anchor_range` (start_offset / end_offset 字符索引) 上, artifact 滚到下个版本不自动迁移锚点 (锚跟产物绑死, 否则 review 语境漂移); **反约束**: 不做锚点跨版本自动 \"携带\" 智能 (留 v3+, 真做要解决 diff/范围漂移)
3. **AnchorCommentAdded 套 #237 envelope** (RT-1.1 #290 envelope 锁同源): `AnchorCommentAdded{type, cursor, anchor_id, comment_id, artifact_id, artifact_version_id, channel_id, author_id, author_kind, created_at}` 10 字段 byte-identical 于 ArtifactUpdated 同模式 (`type` discriminator + `cursor` 第二字段共一根 hub.cursors 单调 sequence; 注: 第 9 字段 `author_kind` 跟 §1 表 `anchor_comments.author_kind` 列名一致 — anchor 是评论作者非 commit 提交者, 不复用 CV-1 ArtifactUpdated 的 `committer_kind` 命名; 第 6 字段 `artifact_version_id` 用 §1 schema FK 列名而非 ArtifactUpdated 的 `version` 用户号 — anchor 钉死 PK 行非用户版本号, 立场 ② 字面); 走 BPP-1 #304 envelope CI lint 自动闸; **反约束**: 不自造 envelope, 不 client timestamp 排序

## 1. 拆段实施 (CV-2.1 / 2.2 / 2.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CV-2.1** schema migration v=14 | `artifact_anchors` 表 (`id` / `artifact_id FK` / `artifact_version_id FK NOT NULL` / `start_offset INT` / `end_offset INT CHECK end>=start` / `created_by` / `created_at` / `resolved_at NULL`); `anchor_comments` 表 (`id` / `anchor_id FK` / `body TEXT` / `author_kind CHECK in ('agent','human')` / `author_id` / `created_at`); 索引 `idx_anchors_artifact_version` + `idx_anchor_comments_anchor`; v=13 (CV-1.1) → v=14 双向 | 待 PR (战马A) | 战马A |
| **CV-2.2** server API + WS push | `POST /artifacts/:id/anchors` 创锚 (body: `{version, start_offset, end_offset}`, 校验 version == current 或 ≤ current 但 immutable; channel 成员可见, 走 CHN-1 权限) + `POST /anchors/:id/comments` 加评 + `POST /anchors/:id/resolve` 标已审 (owner / 创建者) + WS push `AnchorCommentAdded` frame 套 RT-1.1 cursor (立场 ③); 反断 \"agent → agent 锚点对话\" 路径 — 校验 thread 至少有一 `author_kind='human'` 锚点 (立场 ①, 防 AI 自循环) | 待 PR (战马A) | 战马A |
| **CV-2.3** client SPA anchor UI | CV-1.3 markdown preview 上挂选区 → \"在此选区评论\" 按钮 (调 CV-2.2 创锚); artifact 右侧栏列 active anchor thread (按 `start_offset` 排), 点 thread 高亮选区 + 滚视图; 评论气泡内联 (跟 channel chat 风格区分: 浅灰底, 行高密); WS `AnchorCommentAdded` 实时刷; resolve 后 thread 折叠 | 待 PR (战马A) | 战马A |

## 2. 与 CV-1 / RT-1 / CHN-1 留账冲突点

- **CV-1 artifact_version 复用** (非冲突): 锚 FK `artifact_version_id` 指 CV-1.1 #334 `artifact_versions.id`; rollback 触发新 version (CV-1 立场 ⑦) → 老 version 锚保留 (immutable, 立场 ②), 不自动迁移
- **RT-1 cursor 复用**: AnchorCommentAdded 走 #290 cursor + #292 client backfill, 不另起 channel; CHN-4 collab demo (G3.4) 同共用
- **CHN-1 channel 权限继承**: anchor 创/读权限 = artifact 所属 channel 成员权限 (CHN-1 #286 API 同源校验); 反约束: 不另起 anchor-level 权限层
- **AL-3 不依赖**: anchor presence (\"谁在看这个 thread\") 留 v3+; v1 不挂在线状态
- **v=14 三方撞号 sequencing 锁** (DM-2.1 战马B / CV-2.1 飞马 spec / CHN-2.1 飞马 spec 全挤 v=14): 真 sequencing **谁先 merge 谁拿 v=14, 后顺延**; 起手优先级 — DM-2 战马B (~6h 起手, 30min 阈值临过) 优先抢 v=14; 若战马B 未回报转活给战马A, 则 CV-2.1 拿 v=14, DM-2.1 顺延 v=15, CHN-2.1 v=16 (CHN-2.1 实际无 schema 改, 软约束在 server, 不抢号)

## 3. 反查 grep 锚 (Phase 3 验收)

```
git grep -nE 'artifact_anchors.*artifact_version_id'   packages/server-go/internal/migrations/   # ≥ 1 hit (CV-2.1 立场 ②)
git grep -nE 'AnchorCommentAdded\{[^}]*cursor'          packages/server-go/internal/ws/           # ≥ 1 hit (立场 ③ envelope 锁)
git grep -nE 'anchor.*end_offset.*CHECK'                packages/server-go/internal/migrations/   # ≥ 1 hit (range 反向校验)
git grep -nE 'anchor.*author_kind.*=.*"agent".*author_kind.*=.*"agent"' packages/server-go/internal/   # 0 hit (立场 ① 反 agent→agent thread)
git grep -nE 'anchor.*migrate.*to.*next_version|anchor.*follow.*version' packages/server-go/internal/   # 0 hit (反约束 立场 ② 不跨版本迁移)
# #355 野马反约束三连 (锚点钉死人审, 立场 ① 字面落)
git grep -nE 'anchor-comment-btn|data-anchor-id'        packages/client/src/components/ArtifactPanel.tsx   # ≥ 1 hit (① hover 入口 — owner 视角); agent 视角 e2e 反断 0
git grep -nE 'createAnchor.*kind.*=.*"agent"|kind=="agent".*POST.*anchors' packages/server-go/internal/   # 0 hit (server agent role POST 锚 → 403, 错码 anchor.create_owner_only)
git grep -nE 'agent.*reply.*new_anchor|cross.*anchor.*agent' packages/server-go/internal/                   # 0 hit (cross-anchor agent→agent 同 403)
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束)

- ❌ 锚点跨版本自动迁移 / diff 范围漂移智能 (立场 ② v3+)
- ❌ agent → agent 锚点对话 (立场 ① 蓝图 §1.6 字面禁)
- ❌ anchor presence (\"谁在看 thread\") / typing indicator (留 AL-3 后续 + v3+)
- ❌ 锚点挂 PDF / 图片 / 代码 (CV-1 Markdown ONLY 锁同源, 留 CV-3 D-lite 后)
- ❌ 锚点 emoji 反应 / 点赞 (留 v3+)
- ❌ admin 看 anchor body (走 god-mode endpoint 不返回 body, ADM-0 §1.3 红线同源)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- CV-2.1: migration v=13 → v=14 双向 + UNIQUE 反向 + end_offset CHECK reject (start>end → reject) + FK cascade (artifact_version 删 → anchors 删)
- CV-2.2: 创锚 version != head 接受 (immutable v3 立场 ②) + agent-only thread 反断 403 (立场 ①) + AnchorCommentAdded envelope byte-identical 跟 ArtifactUpdated #342 (反向 grep 字段顺序) + resolve 权限 owner / creator
- CV-2.3: e2e 选区创锚 + 锚气泡列表 + WS 实时刷 (跟 CV-1.3 #348 §3.3 同 e2e 模式) + resolve UI 折叠 → **G3.2 闸 e2e 直撑**
