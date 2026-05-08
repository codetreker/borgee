# Acceptance Template — CV-2: 锚点对话 (artifact 段落 anchor + comment thread)

> 蓝图: `canvas-vision.md` §1.4 (workspace = artifact 集合) + §1.6 (锚点对话: 人机界面, 不是 agent 间通信 — owner review agent 产物的工具) + §2 v1 不做行 "❌ 段落锚点对话 (v2 加, v1 验证文档形态够不够)" → CV-2 是 v2 兑现 + `concept-model.md` §4 (mention 路由按 sender_id, agent 不展开 owner)
> Implementation: `docs/implementation/modules/cv-2-spec.md` (飞马 spec **TBD**)
> 拆 PR (拟, 等飞马 spec 确定): **CV-2.1** schema migration (`artifact_anchors` + `artifact_comments` 双表) + **CV-2.2** server API (POST anchor / POST comment / GET thread + WS push) + **CV-2.3** client SPA (artifact 段落选中 → 创建 anchor + comment thread UI 渲染)
> Owner: 战马A 实施 (待 spawn, 跟 CV-1 ✅ closed 顺位接力) / 烈马 验收
> Status: ⚪ skeleton (跟 #318 AL-4 + #293 DM-2 + #353 CHN-2 acceptance skeleton 同模式, 4 件套并行 — spec 飞马 / 文案锁 野马 / acceptance 烈马 / 战马A 等三件套到位实施)

---

## §0 关键约束 (蓝图立场, 实施 PR 不可绕)

> 锚: `canvas-vision.md` §1.6 + §1.5 + §2 v1 不做行 → CV-2 v2 兑现

| # | 立场 | 反约束 | 蓝图源 |
|---|---|---|---|
| ① | 锚点 = 人机界面, **不是 agent 间通信** — anchor + comment 仅供 owner review agent 产物 | ❌ agent 不能在 anchor 上发起 root comment (反向断言: agent role POST anchor → 403, agent 仅能在已有 thread 内 reply); ❌ 不复用 anchor 做 agent-to-agent 内部协议 (走普通 channel message + artifact 引用, 不带 anchor) | §1.6 "agent 之间通信走普通 channel message + artifact 引用, 不需要锚点" |
| ② | anchor 指向 artifact 段落 (range 范围), **不是** artifact 整体 — 段落语义清晰 | anchor schema 必带 `start_offset` + `end_offset` (markdown body byte offset) + `version` (当时 artifact version, 防版本漂移); ❌ 不允许 NULL offset 表示 "整个 artifact" (走普通 message reply 路径) | §1.6 "段落锚点对话" + §1.4 "artifact 自带版本历史" |
| ③ | 评论独立 entity — `artifact_comments` 不复用 `messages` 表 | 数据层拆: 跟 message 流不混; comment 不进 channel fanout (反向 fanout — comment 仅推订阅 anchor 的客户端, 不走 channel WS hub) | §1.4 + §1.6 "owner review" 边界 + 跟 channel-model §1.1 "聊天流" 拆死 |
| ④ | anchor **不改 artifact body** — 不可变内容 | ❌ POST anchor 路径不写 `artifacts.body` 列 (反向 grep server-go 不在 anchors handler 触 body update); ❌ artifact rollback 不删 anchor (anchor `version` 字段记录原版本, rollback 后 anchor 仍可见但标 "stale (artifact at v3, anchor on v1)") | §1.5 "agent 默认能写内容, 修改布局需 owner grant" → CV-2 anchor 是评论层不是结构层 |
| ⑤ | anchor version 漂移处理 — artifact commit 后 anchor 不重新映射 (v1 不做 OT/CRDT-like rebase) | anchor `version` 字段冻结创建时 artifact version; UI 渲染时若 artifact 当前 version > anchor version → 显示 "stale" 标签 + 仍可访问原 thread; ❌ 不做段落自动 follow (留 v3+) | §1.4 + canvas-vision.md §2 v1 不做 "❌ realtime CRDT" 延伸到 CV-2 |
| ⑥ | 通知路径 — anchor 上 @ mention 走 DM-2 mention 路由 (按 sender_id, 不展开 owner) | comment body 中 `@<user_id>` token 复用 DM-2 `message_mentions` 解析路径 (跟 DM-2.1 schema 一致, 表名 anchor_mentions 还是复用 message_mentions 由飞马 spec 决议); ❌ anchor comment fanout 不抄送 channel owner (跟 concept-model §4 一致) | `concept-model.md` §4 + DM-2 acceptance §1.3 反向 |
| ⑦ | anchor channel-scoped — 跟 artifact 同 channel 范围, 不跨 channel | 跨 channel 访问 anchor → 403 (反向断言, 跟 CV-1.2 #342 cross-channel 模式一致, `cv-1.md` §0 立场 ① 延伸) | `canvas-vision.md` §1.3 "channel 自带" + cv-1.md §0 ① channel-scoped artifact |

---

## 验收清单

### §1 schema (CV-2.1) — artifact_anchors + artifact_comments 数据契约

> 锚: 飞马 spec §1 (TBD) + CV-1.1 #334 schema 模板 (artifacts + artifact_versions 双表) + DM-2.1 #312 spec (message_mentions 表) + 立场 ②③⑥⑦

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 [TBD] `artifact_anchors` 表三轴: PK `id` AUTOINCREMENT + `artifact_id` NOT NULL FK + `version` NOT NULL int + `start_offset` / `end_offset` NOT NULL int + `created_by` NOT NULL FK users + `created_at` NOT NULL — pragma 字面 列断言 (立场 ②⑤) | migration drift test | 战马A / 烈马 | _(待 CV-2.1 PR + 飞马 spec)_ — 跟 CV-1.1 #334 `TestCV11_CreatesArtifactsTable` pragma 模板一致 |
| 1.2 [TBD] `artifact_comments` 表三轴: PK + `anchor_id` NOT NULL FK + `body` NOT NULL TEXT + `author_id` NOT NULL FK users + `author_kind` ('user'/'agent') + `created_at` NOT NULL — 跟 messages 表**字段不重叠** (立场 ③ 数据层拆) | migration drift test | 战马A / 烈马 | _(待 CV-2.1 PR)_; 反向 grep `messages.*INNER JOIN.*artifact_comments` count==0 (拆死) |
| 1.3 [TBD] CHECK `start_offset >= 0 AND end_offset > start_offset` (立场 ② 段落范围语义) — INSERT 异值 reject | migration test | 战马A / 烈马 | _(待 CV-2.1 PR)_ |
| 1.4 [TBD] INDEX `idx_artifact_anchors_artifact_id` (热查路径 — artifact 渲染时拉所有 anchor) + INDEX `idx_artifact_comments_anchor_id` | migration test | 战马A / 烈马 | _(待 CV-2.1 PR)_ |
| 1.5 [TBD] migration v=N → v=N+1 串行号 + idempotent rerun no-op + forward-only — registry.go 字面锁 | migration drift test | 战马A / 烈马 | _(待 CV-2.1 PR)_; `grep -nE "v=N\|N:" packages/server-go/internal/migrations/registry.go` count==1 |
| 1.6 反向 — `artifact_anchors` 表无 `body` 列 (立场 ④ anchor 不改 artifact body, body 在 artifact_versions); 表无 `parent_anchor_id` 列 (v1 anchor 单层, 不做 nested anchor) | migration drift test + CI grep | 飞马 / 烈马 | _(待 CV-2.1 PR)_; `grep -nE 'artifact_anchors.*body\|parent_anchor_id' packages/server-go/internal/migrations/cv_2_1_*.go` count==0 |

### §2 server 行为 (CV-2.2) — REST + WS push + 立场 ①④⑥⑦ 反约束

> 锚: 飞马 spec §2 (TBD) + CV-1.2 #342 server 模板 + 立场 ①④⑥⑦

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 [TBD] POST `/artifacts/:id/anchors` (创建 anchor) — sender 必须是 channel member; cross-channel 403 (立场 ⑦); body 必带 `version` + `start_offset` + `end_offset` (立场 ②) | unit + e2e | 战马A / 烈马 | _(待 CV-2.2 PR)_ — 跟 CV-1.2 #342 `TestArtifactCrossChannel403` 同模式 |
| 2.2 [TBD] POST `/anchors/:id/comments` (在 anchor 内 reply) — channel member 任何 role 可 reply (立场 ① reply 不限) | unit | 战马A / 烈马 | _(待 CV-2.2 PR)_ |
| 2.3 [TBD] POST `/artifacts/:id/anchors` agent role (author_kind='agent') — 反向断言 root anchor 创建 → 403 (立场 ① anchor 是人机界面, agent 不发起 root) | unit | 战马A / 烈马 | _(待 CV-2.2 PR)_ — 反向 model: 跟 CV-1.2 RollbackOwnerOnly admin 401 同模式 |
| 2.4 [TBD] POST `/anchors/:id/comments` agent reply 合法 — agent 仅在已有 thread 内 reply, 走 author_kind='agent' (立场 ①) | unit | 战马A / 烈马 | _(待 CV-2.2 PR)_ |
| 2.5 [TBD] WS push — anchor created / comment created → 推 `ArtifactAnchorUpdated` frame (复用 BPP-1 #304 envelope CI lint, 7 字段 byte-identical 跟 RT-1.1 #290 cursor 序; frame `kind` ∈ {'anchor.created', 'comment.created'}) — 立场 ③ 不进 channel fanout, 仅推订阅该 artifact 的客户端 | unit + WS sniff | 战马A / 烈马 | _(待 CV-2.2 PR)_ — 跟 CV-1.2 #342 `PushFrameOnCreateAndCommit` 同模式; BPP-1 #304 lint 自动 enforce 字段顺序 |
| 2.6 [TBD] artifact rollback (CV-1.2 #342 路径) → anchor 不删, 仅 anchor.version 标 stale (立场 ④⑤) | unit | 战马A / 烈马 | _(待 CV-2.2 PR)_; 反向 grep `DELETE FROM artifact_anchors.*rollback` count==0 |
| 2.7 [TBD] anchor comment body 中 `@<user_id>` mention → 复用 DM-2 mention 路由 (按 sender_id 不展开 owner, 立场 ⑥) | unit + e2e | 战马A / 烈马 | _(待 CV-2.2 PR + DM-2 联动)_ |
| 2.8 反向 grep — anchors handler 不应触 `artifacts.body` 列 (立场 ④); 不应裂 `anchor.created` 跟 `comment.created` 走两个 frame namespace (复用 ArtifactAnchorUpdated 单 frame `kind` 区分) | CI grep | 飞马 / 烈马 | `grep -rnE 'UPDATE artifacts.*body.*anchor\|type:.*"anchor\\." packages/server-go/internal/api/anchors.go packages/server-go/internal/ws/' count==0 |
| 2.9 反向 grep — admin god-mode 不入 anchor 写路径 (跟 ADM-0 §1.3 红线, CV-1.2 #342 admin 401 模式延伸) | unit | 飞马 / 烈马 | _(待 CV-2.2 PR)_ |

### §3 用户感知 (CV-2.3 client SPA) — anchor 创建 + thread 渲染 + stale 标签

> 锚: 立场 ②⑤⑥ + CV-1.3 #346 ArtifactPanel 模板

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 [TBD] artifact body 段落选中 (text selection) → 浮 "添加评论" 按钮 → 点击 → POST anchor 创建 + thread modal 打开 (立场 ②) | e2e + DOM | 战马A / 烈马 | _(待 CV-2.3 PR)_ |
| 3.2 [TBD] anchor 高亮 — 已有 anchor 的段落 DOM `data-anchor-id="<uuid>"` 字面锁 + 黄色高亮 background; 鼠标 hover → 显示 thread preview tooltip | e2e + DOM | 战马A / 烈马 | _(待 CV-2.3 PR)_ |
| 3.3 [TBD] thread 渲染 — 时序 ASC 列出 comment, author display name + avatar; agent comment 加 🤖 badge (跟 DM-2 §3.2 mention 候选列表 agent 标识同模式) | e2e | 战马A / 烈马 | _(待 CV-2.3 PR)_ |
| 3.4 [TBD] stale 标签 — anchor.version < artifact 当前 version → DOM 渲染 `data-anchor-stale="true"` + 文案锁 byte-identical: `锚点指向 v{N}, 文档已更新到 v{M}` (立场 ⑤; 文案野马锁) | e2e + DOM grep | 战马A / 烈马 | _(待 CV-2.3 PR + 野马文案锁)_ |
| 3.5 [TBD] WS push 实时 — 双 tab 一窗口 POST comment, 另一窗口 thread modal ≤3s 接收并 append 新 comment (立场 ③ 仅推订阅 artifact 的客户端, ≤3s budget 跟 RT-1 + CV-1.3 #348 一致) | e2e | 战马A / 烈马 | _(待 CV-2.3 PR)_ |
| 3.6 [TBD] agent 不见 "添加评论" 按钮 (立场 ① — agent 不发起 root anchor; 客户端 DOM 反约束 — agent 登录态 button count==0) | e2e + DOM | 战马A / 烈马 | _(待 CV-2.3 PR)_ |
| 3.7 [TBD] anchor body 中 `@user_id` 渲染 — 复用 DM-2.3 mention 渲染 (`@{display_name}` 蓝色高亮, raw UUID 不漏) — 立场 ⑥ | e2e + DOM grep | 战马A / 烈马 | _(待 CV-2.3 PR + DM-2.3 联动)_ |

### §4 蓝图行为对照 (反查锚, 每 PR 必带)

> 锚: 立场 ①④⑥⑦ 反约束横切

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.a 反向 grep — `messages` 表跟 `artifact_comments` 表无 JOIN (立场 ③ 数据层拆死) | CI grep | 飞马 / 烈马 | `grep -rnE 'messages.*JOIN.*artifact_comments\|artifact_comments.*JOIN.*messages' packages/server-go/internal/store/` count==0 |
| 4.b 反向 grep — anchor body 不写 artifacts.body (立场 ④) | CI grep | 飞马 / 烈马 | `grep -rnE 'UPDATE artifacts.*body.*WHERE.*anchor\|anchors.*UPDATE artifacts' packages/server-go/internal/api/anchors.go` count==0 |
| 4.c 反向 grep — agent role 不能 POST root anchor (立场 ①) | CI grep + unit | 飞马 / 烈马 | `grep -nE 'author_kind.*=.*"agent".*INSERT.*artifact_anchors\|RequirePermission.*"anchor.create"' packages/server-go/internal/api/anchors.go` 命中需带 owner role check |
| 4.d 反向 grep — anchor frame 不裂出 `anchor.created` 跟 `comment.created` 双 namespace (立场 ③ + BPP-1 #304 envelope 单 frame `kind` 区分) | CI grep | 飞马 / 烈马 | `grep -rnE "type:.*'anchor\\.\|type:.*'comment\\." packages/server-go/internal/ws/` count==0; 仅命中 `ArtifactAnchorUpdated` 单 frame |
| 4.e e2e — 跨 channel anchor 访问 → 403 (立场 ⑦, 跟 CV-1.2 cross-channel 模式镜像) | e2e | 战马A / 烈马 | _(待 CV-2.3 PR)_ |

---

## 退出条件

- §0 关键约束 7 立场入册 (跟 cv-1.md §0 立场反查 同模式)
- §1 schema 6 项 (5 TBD + 1 反向 grep) 全绿
- §2 server 9 项 (7 TBD + 2 反向 grep) 全绿
- §3 用户感知 7 项 (7 TBD client SPA) 全绿
- §4 蓝图行为对照 5 项 (4 grep + 1 e2e) 全绿
- 登记 `docs/qa/regression-registry.md` REG-CV2-001..N (server schema + server behavior + client + 反向 兜底)
- 蓝图引用区 `canvas-vision.md` §1.6 + §2 v1 不做行 "❌ 段落锚点对话 (v2 加)" 翻 ✅ #CV-2 closure (v2 兑现)

## 跟其他 milestone 的边界

| Milestone | 关系 | 备注 |
|---|---|---|
| **CV-1** ✅ closed | CV-2 anchor 指向 CV-1 artifact 段落; 复用 artifact_id FK; 复用 cross-channel 403 模式 | CV-1.1 #334 schema (artifacts + artifact_versions) 已落; CV-2 在其上加 anchors + comments 双表 |
| **DM-2** in-flight | CV-2 anchor comment body 中 `@user` mention 走 DM-2 mention 路由 (concept-model §4) | 复用 message_mentions 解析 (飞马 spec 决议是新表 anchor_mentions 还是复用 message_mentions) |
| **RT-1** ✅ closed | CV-2 ArtifactAnchorUpdated frame 复用 RT-1.1 server cursor + RT-1.2 backfill + RT-1.3 BPP session.resume | BPP-1 #304 envelope CI lint 自动 enforce 7 字段 byte-identical |
| **CHN-2** TBD (本批次同时起 #353) | DM channel 无 workspace → DM 内 artifact 不存在 → CV-2 anchor 不入 DM 路径 | 立场 ⑦ channel-scoped 自动反约束 DM (CHN-2 §1.3 server 拒 DM artifact API → CV-2 anchor 也走不到) |
| **CV-3 / CV-4** TBD (Phase 3 章程未启) | CV-2 闭 → CV-3 D-lite 渲染 + CV-4 artifact iterate 完整流 | CV-3/4 spec 飞马 TBD |

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 烈马 | v0 skeleton — §0 7 立场 + §1-§4 验收清单 (TBD 占位等飞马 spec) + 边界表 (CV-1/DM-2/RT-1/CHN-2/CV-3/CV-4 关系); 跟 #318 AL-4 + #293 DM-2 + #353 CHN-2 acceptance skeleton 同模式 4 件套并行 (spec 飞马 / 文案锁 野马 / acceptance 烈马 / 战马A 等三件套到位接力) |
