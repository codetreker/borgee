# CHN-4 spec brief — 协作场骨架收口 (channel × artifact × anchor × iterate × mention 跨视图整合)

> 烈马 · 2026-04-29 · ≤80 行 spec lock (Phase 3 章程 9 milestone 收口锚, G3.4 demo 路径全闭)
> **蓝图锚**: [`channel-model.md`](../../blueprint/current/channel-model.md) §1.1 (channel = 协作场, 不是单纯聊天容器) + §3.1 (channel 作为协作场, workspace 升级为协作场另一支柱) + §1.4 (作者定义 + 个人微调) + `canvas-vision.md` §1.2 (D-lite) + §1.4 (artifact 集合) + §1.6 (锚点 = 人审 agent 产物)
> **关联**: 已锁 spec — CV-1 ✅ #334+#342+#346+#348 / CV-2 #356 (v3 #368) / CV-3 #363 / CV-4 #365 / CHN-1 ✅ / CHN-2 #357 / CHN-3 #371; 已锁文案锁 — CV-1/CV-2 #355/#368 / CV-3 #370 / CHN-2 #354/#364 / CHN-3 (待); CM-4 minimal presence ✅ / RT-1 #290+#292+#296 ✅ envelope cursor 单调 / DM-2 #361+#372 mention 路径
> **章程闸**: G3.4 协作场骨架 demo **收口闸位** — Phase 3 退出公告"协作场可见路径"全闭依赖此 spec, 是 9 milestone 拼图最后一块

> ⚠️ 锚说明: CHN-4 不引入新 entity (无新表), 仅整合既有 channel × artifact × anchor × iterate × mention 五栈跨视图协调, 让"协作场"作为完整 UX 浮现 — 反 "再开一个表/endpoint" 路径

## 0. 关键约束 (3 条立场, 蓝图字面 + 跨 milestone 边界对齐)

1. **协作场 = channel + workspace 双支柱整合视图, 不开新 entity** (蓝图 §3.1 字面 "workspace 升级为协作场的另一支柱"): channel 视图保持 messages 流主体不变, 加 workspace tab 入 artifact 集合 (CV-1/CV-3 kind 三态), workspace tab 内见 artifact list + version history + iterate 入口 (CV-4) + anchor thread 侧栏 (CV-2); **反约束**: 不新建 `collab_scene` / `workspace_view` 表, 不开 `/api/v1/scenes/:id` 旁路 endpoint (走既有 `/channels/:id` + `/artifacts` + `/iterations` + `/anchors` 四源拼装, client 端组合)
2. **mention × artifact × anchor 三路径互不污染** (各 milestone spec 反约束承袭): mention 路径走 `messages` 流 + `message_mentions` 表 (DM-2); artifact 路径走 `artifacts` + `artifact_versions` (CV-1/3); anchor 路径走 `artifact_anchors` + `anchor_comments` (CV-2); iterate 路径走 `artifact_iterations` (CV-4); **反约束**: 不在 messages 表加 artifact_id / iteration_id / anchor_id 反指 (DM-2 / CV-4 / CV-2 立场各自字面禁); 不在 artifact_versions 加 mention 反指 (CV-4 立场 ① 字面)
3. **个人偏好层 (CHN-3) 仅作用于 channel sidebar, 不渗透 workspace tab / artifact / anchor 视图** (CHN-3 立场承袭): user_channel_layout (CHN-3) 锁 sidebar 折叠/排序; workspace tab 内 artifact list 顺序由 channel-scoped 既有 ORDER BY (CV-1.1 schema), 不查 user_channel_layout; anchor thread 顺序按 created_at, 不查个人偏好; **反约束**: workspace tab / artifact panel / anchor sidebar 内不读 user_channel_layout (跟 CHN-3 立场 ⑥ "ordering 是 client 端事但仅 sidebar" 同模式)

## 1. 拆段实施 (CHN-4.1 / 4.2 / 4.3, ≤ 3 PR; **无 schema 改, v 号 v=20 仅占位 sequencing 锁**)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CHN-4.1** server endpoint 拼装层 (无 schema, v=20 占位 sequencing 字面延续) | 加 `GET /api/v1/channels/:id/scene` 单端点 — 服务端拼装返 `{channel, artifacts: [...], pending_iterations: [...], recent_anchors: [...]}` 一次拉 (减 client 4 次 round-trip); ACL 跟 channel-scoped 同源 (CHN-1 #286 模式); workspace tab artifact list 走 `GET /channels/:id/artifacts` (CV-1.2 #342 既有) 或拼装端点二选一 (client 选); **反约束**: 不开 PUT/POST /scene (协作场状态由各 entity 自管, 这是只读视图聚合); v=20 占位锁 (CHN-3.1 v=19 后, sequencing 字面延续 14/15/16/17/18/19/**20**) | 待 PR (战马A/B 协调) | 战马 |
| **CHN-4.2** client 协作场视图整合 (Channel.tsx + WorkspaceTab.tsx + AnchorSidebar.tsx 三组件 wiring) | `<Channel.tsx>` 现 messages 流 + workspace tab 切换 (CHN-2 已锁 DM 视图无 workspace, CHN-4 仅 channel.type IN ('private','public') 显 tab); `<WorkspaceTab.tsx>` 列 artifact list (CV-1/3 kind 三态 byte-identical 跟 #370 字面锁) + 行尾 iterate 按钮 (CV-4 owner-only) + version dropdown (CV-1 既有 ArtifactPanel 复用); `<AnchorSidebar.tsx>` 折叠侧栏列 recent anchor thread (CV-2 既有 anchor_comments query 复用), 点击跳 ArtifactPanel 锚视图 | 待 PR (战马A) | 战马A |
| **CHN-4.3** e2e 协作场骨架 demo + G3.4 截屏归档 | e2e: open channel → workspace tab 切换 → 创 markdown artifact + commit v1 (CV-1) → 触发 iterate (CV-4) → completed 自动跳新版本 → owner 加 anchor + comment (CV-2) → 在 messages 流 mention `@agent` (DM-2) + 离线 fallback owner DM 触发 → 全流路径 e2e ≤30s 完成; G3.4 demo 截屏 5 张归档 `g3.4-collab-{channel-with-tabs, workspace-artifact-list, iterate-flow, anchor-thread, mention-flow}.png` (撑章程 Phase 3 退出公告) | 待 PR (战马A) | 战马A |

## 2. 与 CHN-1/2/3 / CV-1/2/3/4 / DM-2 / RT-1 / AL-4 留账冲突点

- **CHN-1 channel 视图骨架** (核心): channel.type IN ('private','public','dm') 三态 — DM 不显 workspace tab (CHN-2 立场守); 跟 CHN-1 #288 ChannelGroupComponent 共存不破
- **CHN-2 DM 拆死** (字面承袭): DM 视图无 workspace tab + 无 artifact + 无 anchor + 无 iterate (跟 #354 立场 ④ + #353 §3.1 + #364 data-kind 同源); 反约束 e2e DOM `[data-kind="dm"] [data-tab="workspace"]` count==0
- **CHN-3 sidebar 偏好** (非冲突): CHN-3 仅作用于 sidebar, 不渗透 workspace tab; CHN-4.2 不读 user_channel_layout
- **CV-1/3 artifact kind 三态**: workspace tab artifact list 渲染走 `data-artifact-kind` 三态 (跟 #370 文案锁 byte-identical); kind switch DOM 反约束 #338 cross-grep 无新字面池
- **CV-2 anchor 仅 markdown** (字面承袭): AnchorSidebar 仅显示 markdown artifact 上的 anchor (CV-2 §4 反约束 + #363 `anchor.unsupported_artifact_kind` server 端守); 反约束 client 不渲染 code/image artifact 上的 anchor 入口
- **CV-4 iterate flow** (核心整合): iterate 按钮在 WorkspaceTab artifact 行尾 (owner-only, 跟 CV-4 立场 ② 单源); 状态 inline 进度走 IterationStateChangedFrame WS push; 反约束: 不在 messages 流显示 iterate 进度 (域隔离, CV-4 立场 ① 字面)
- **DM-2 mention** (非冲突): mention 在 messages 流, 不进 workspace tab; iterate completed 触发 ArtifactUpdated WS frame (RT-1), 不通过 mention 路径
- **RT-1 cursor 共序** (核心): 4 frame (ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 / IterationStateChanged 9) 共一根 hub.cursors 单调发号, CHN-4 不引入新 frame
- **AL-4 runtime stub** (非阻塞): CV-4 iterate 走 AL-4 stub fail-closed 路径, CHN-4 demo 用 mock runtime 演示 happy path (`runtime_not_registered` reason 在 demo 截屏外标 v3+ 真接管)

## 3. 反查 grep 锚 (Phase 3 章程退出闸 + Phase 4 验收)

```
git grep -nE 'GET /api/v1/channels/.*/scene'                   packages/server-go/internal/api/   # ≥ 1 hit (CHN-4.1)
git grep -nE 'WorkspaceTab|AnchorSidebar'                      packages/client/src/components/    # ≥ 1 hit (CHN-4.2)
git grep -nE 'data-tab="workspace"'                            packages/client/src/components/    # ≥ 1 hit (channel.type 非 dm 才渲染)
# 反约束 (5 条 0 hit)
git grep -nE 'CREATE TABLE.*collab_scene|CREATE TABLE.*workspace_view' packages/server-go/internal/migrations/   # 0 hit (立场 ① 不开新 entity)
git grep -nE 'POST.*\/scenes\/|PUT.*\/scenes\/'                packages/server-go/internal/api/   # 0 hit (立场 ① 只读视图聚合)
git grep -nE 'data-kind="dm".*data-tab="workspace"|dm.*workspace' packages/client/src/components/   # 0 hit (CHN-2 DM 拆死字面承袭)
git grep -nE 'WorkspaceTab.*user_channel_layout|AnchorSidebar.*user_channel_layout' packages/client/src/   # 0 hit (CHN-3 偏好不渗透)
git grep -nE 'messages.*iterate_progress|messages.*iteration_state' packages/client/src/components/   # 0 hit (CV-4 域隔离立场 ① 字面)
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束)

- ❌ 新表 / 新 schema (立场 ① — workspace tab 走既有四源拼装)
- ❌ 协作场全文搜索 (cross-channel artifact/anchor 搜索留 Phase 5+)
- ❌ artifact 跨 channel 共享 (canvas-vision §2 v1 字面禁; CV-2 §4 字面禁)
- ❌ iterate 跨 artifact (CV-4 立场反约束承袭)
- ❌ admin SPA 协作场视图 god-mode (ADM-0 §1.3 红线; admin 走元数据白名单, 不入业务路径)
- ❌ 协作场偏好 (workspace tab 默认开/关, anchor sidebar 折叠状态等) — CHN-3 仅锁 sidebar, workspace 内偏好留 v3+
- ❌ multi-channel collab view (一次看多个 channel 的协作场) — Phase 5+
- ❌ 协作场实时活动流 (谁在编辑哪个 artifact) — CM-4 minimal presence 已够, 完整 activity feed 留 v3+

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- CHN-4.1: GET /channels/:id/scene 200 拼装 4 源数据 byte-identical 跟单端点结果一致 + DM channel → 仅返 channel 元数据无 artifacts (CHN-2 拆死) + 非 channel member → 403 (CHN-1 ACL 同源)
- CHN-4.2: vitest WorkspaceTab kind 三态渲染 (跟 #370 文案锁字面) + iterate 按钮 owner-only DOM 反断 (非 owner DOM omit, CV-4 立场 ② 同源) + AnchorSidebar 仅 markdown artifact 显 (CV-2 §4 反约束) + DM 视图 DOM `[data-tab="workspace"]` count==0 (CHN-2 拆死)
- CHN-4.3: e2e 全流路径 30s 闭环 + G3.4 demo 5 张截屏 Playwright `page.screenshot()` 入 `docs/qa/screenshots/g3.4-collab-*.png` (撑章程退出公告)
