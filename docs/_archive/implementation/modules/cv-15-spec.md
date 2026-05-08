# CV-15 spec brief — artifact comment edit history audit (战马C v0)

> Phase 6+ wrapper milestone — artifact comment edit history audit, GET-only readonly view (跟 DM-7 #558 messages.edit_history 模式承袭). **0 schema 改** — artifact comments 走 messages 表 (CV-5 #530 立场 ①), `edit_history` 列 DM-7.1 v=34 已加 (复用 byte-identical), CV-15 仅加 GET endpoint scoped 到 `content_type='artifact_comment'` filter.
> **蓝图锚**: [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) §1.4 artifact 集合 + [`dm-model.md`](../../blueprint/dm-model.md) §3 forward-only audit history 同精神.
> **关联**: CV-5 #530 artifact comments 走 messages 表单源 (立场 ①, 不裂 artifact_comments 表) + CV-7 #535 既有 PATCH /api/v1/messages/{id} edit path (sender owner-only ACL) + DM-7 #558 messages.edit_history schema v=34 + UpdateMessage SSOT history append.

> ⚠️ **架构发现**: team-lead 派活原案是 "v=45 ALTER artifact_comments ADD COLUMN edit_history". 真状况:
>   1. 不存在 `artifact_comments` 表 — CV-5 #530 立场 ① "comment 走 messages 表单源 不裂表" 已锁;
>   2. messages.edit_history 列已由 DM-7.1 v=34 (#558) 加, CV-7 #535 PATCH /api/v1/messages/{id} 已自动 wrap UpdateMessage SSOT → artifact comments 编辑早已自动产生 edit_history JSON.
>
> 因此 CV-15 = **0 schema 改 + GET endpoint scoped** (filter messages.content_type='artifact_comment'), 无需 v=45 migration. 留 v=45 号给后续 milestone.

## §0 关键约束 (3 条立场, 蓝图字面承袭)

1. **0 schema 改 — artifact comments 复用 messages.edit_history (DM-7.1 v=34 既有)** (跟 CV-5 #530 立场 ① "comments 走 messages 表单源 不裂表" + DM-7.1 #558 edit_history 列既有 同模式承袭): artifact comments = messages WHERE content_type='artifact_comment' (CV-5/CV-7 既有约定). CV-7 #535 既有 PATCH /api/v1/messages/{id} 走 UpdateMessage SSOT (DM-7.2 #558) 已自动 history append → artifact comments 编辑产生 edit_history JSON. CV-15 仅加 GET endpoint 暴露 history 给 owner. 反约束: 不另起 artifact_comments 表 (反向 grep `CREATE TABLE.*artifact_comments|artifact_comment_history` 在 internal/migrations/ count==0); 不另起 v= migration (CV-15 0 schema, sequencing 不占 v=45).

2. **owner-only sender + admin readonly admin-rail** (跟 DM-7 #558 owner-only ACL 锁链第 19 处 + CV-1.2/CV-2 v2/CV-3 v2/CV-4/AL-5/AP-3/CV-6/AL-9/DM-7/DM-8/CHN-15 owner-only 锁链同模式 — 第 22 处): GET /api/v1/channels/{channelId}/messages/{messageId}/comment-edit-history user-rail sender-only (sender == current user 反断, 别 user → 403); GET /admin-api/v1/messages/{messageId}/comment-edit-history admin readonly admin-rail (admin god-mode ADM-0 §1.3 红线 — admin 看不能改, 反向 grep `mux\.Handle\("(POST|DELETE|PATCH|PUT).*admin-api/v[0-9]+/.*comment-edit-history` 0 hit). content_type filter 强制: 非 artifact_comment 调本 endpoint → 404 (避免跟 DM-7 既有 GET /messages/{id}/edit-history 混淆).

3. **3 文案 byte-identical + AL-1a reason 锁链不漂** (跟 DM-7 / DM-8 / CHN-15 content-lock 同模式): 文案锁三 (`编辑历史` 4 字 modal title byte-identical 跟 DM-7 EditHistoryModal 同源 / `暂无编辑记录` 6 字 空 history 文案 / `共 N 次编辑` 5 字 count). reason 字段复用 DM-7 既有 `'unknown'` byte-identical (AL-1a 锁链不漂, CV-15 inline JSON read-only view 不入 admin_actions). RFC3339 ts 跟 DM-7 同源.

## §1 拆段实施 (CV-15.1 / 15.2 / 15.3, 一 milestone 一 PR)

| 段 | 范围 | 闭锁 |
|---|---|---|
| **CV-15.1** server endpoint + content_type filter | `internal/api/cv_15_comment_edit_history.go` GET /api/v1/channels/{channelId}/messages/{messageId}/comment-edit-history user-rail sender-only + GET /admin-api/v1/messages/{messageId}/comment-edit-history admin readonly + content_type='artifact_comment' filter (非 artifact_comment → 404 reject); 复用 messages.edit_history JSON (DM-7.1 v=34 既有列) + parseCommentEditHistory helper byte-identical 跟 DM-7 同精神 | 战马C |
| **CV-15.2** server tests + reverse-grep | 9 unit (HappyPath sender owner + history JSON shape + sender ≠ → 403 + non-artifact_comment → 404 + admin readonly GET happy + admin god-mode 不挂 PATCH/DELETE/PUT 反向 grep + 0 schema 反向 + 401 + 错码 byte-identical) | 战马C |
| **CV-15.3** client + closure | `lib/api.ts::getArtifactCommentEditHistory(messageId)` thin wrapper (复用既有 fetch + RFC3339 + EditHistoryEntry type) + `components/ArtifactCommentEditHistoryModal.tsx` 文案 byte-identical 跟 DM-7 EditHistoryModal §1 文案锁同源; REG-CV15-001..006 6 🟢 + acceptance flip + PROGRESS [x] CV-15 | 战马C |

## §2 留账边界

- v2 anchor_comments edit history (anchor_comments 表 CV-2 #359 独立, 不在 messages 表; 留 v2+ — 加 PATCH endpoint + 加 edit_history 列 单独 milestone)
- v2 edit history rollback (creator 一键回退到任一历史版本) — 留 v2+, forward-only 立场承袭 (DM-7 同精神)
- v2 admin god-mode comment edit override (永久不挂 ADM-0 §1.3)
- diff syntax highlight (留 v3 client only, 跟 DM-7 同精神)

## §3 反查 grep 锚 (5 反约束, count==0)

```bash
# 1) 0 schema 反向 — 不裂 artifact_comments / artifact_comment_history 表 + 不加 v=45
git grep -nE 'CREATE TABLE.*artifact_comments|artifact_comment_history|cv_15_\d+\.go' \
  packages/server-go/internal/migrations/  # 0 hit
# 2) admin god-mode 不挂 PATCH/DELETE/PUT (ADM-0 §1.3 红线)
git grep -nE '/admin-api/v[0-9]+/.*comment-edit-history|admin-api.*comment-edit-history.*PATCH' \
  packages/server-go/internal/  # 仅 GET 1 hit, 无 PATCH/DELETE/PUT
# 3) content_type filter 强制 — 非 artifact_comment 调本 endpoint 404 (跟 DM-7 既有 /edit-history 区分)
git grep -nE 'content_type\s*!=\s*"artifact_comment"|comment\.not_artifact' \
  packages/server-go/internal/api/cv_15_comment_edit_history.go  # ≥1 hit
# 4) UpdateMessage SSOT 不漂 — 反向 inline UPDATE messages content 0 hit production
git grep -nE 'inline.*UPDATE.*messages.*content' \
  packages/server-go/internal/  # 0 hit (DM-7 既有 SSOT 不破)
# 5) AST 锁链延伸 forbidden 3 token
git grep -nE 'pendingCommentEdit|commentEditQueue|deadLetterCommentEdit' \
  packages/server-go/internal/  # 0 hit
```

## §4 不在范围

- v2 anchor_comments edit history (跟本 milestone 拆死, 留单独 milestone)
- v2 edit rollback / diff syntax highlight / admin god-mode override (留 v2+/v3+)
- v2 audit log row to admin_actions (CV-15 read-only view, 不写 admin_actions; reason 字典不漂)

## §5 跨 milestone byte-identical 锁

- 跟 CV-5 #530 立场 ① "comments 走 messages 表单源" 同源 (CV-15 复用 messages, 不裂 artifact_comments 表)
- 跟 DM-7.1 #558 messages.edit_history 列既有 (CV-15 0 schema 改 复用)
- 跟 DM-7.2 #558 UpdateMessage SSOT history append (CV-7 #535 PATCH /messages/{id} 自动经过)
- 跟 DM-7.3 #558 EditHistoryModal `编辑历史` 文案 byte-identical 跟 content-lock §1 同源
- 跟 owner-only ACL 锁链第 22 处 (CV-1.2/CV-2 v2/CV-3 v2/CV-4/AL-5/AP-3/CV-6/AL-9/DM-7/DM-8/CHN-15 同模式)
- 跟 ADM-0 §1.3 admin god-mode 不挂 PATCH/DELETE/PUT
- 跟 AL-1a reason 锁链不漂 (复用 DM-7 既有 `'unknown'` reason, CV-15 read-only view 不另起字典)
