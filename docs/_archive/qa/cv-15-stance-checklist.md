# CV-15 立场反查清单 (战马C v0)

> 战马C · 2026-04-30 · CV-15 artifact comment edit history audit 立场反查 (跟 DM-7 #558 / CV-7 #535 / CHN-15 #587 同模式)

## §0 立场总表 (3 + 3 边界)

| # | 立场 | 蓝图字面 | 反约束 |
|---|---|---|---|
| ① | 0 schema 改 — artifact comments 走 messages 表 (CV-5 立场 ①), edit_history 列 DM-7.1 v=34 既有 复用 | CV-5 #530 立场 ① + DM-7.1 #558 + CV-7 #535 PATCH 既有 path | 反向 grep `CREATE TABLE.*artifact_comments|artifact_comment_history|cv_15_\d+\.go` 0 hit (架构核查 — 不存在 artifact_comments 表, 不需 v=45 migration) |
| ② | owner-only sender + admin readonly admin-rail | 跟 DM-7 owner-only ACL 锁链第 19 处 + admin god-mode ADM-0 §1.3 红线 | user-rail GET sender == current user (反断 别 user 403); admin-rail GET only (反向 grep `admin-api.*comment-edit-history.*(POST|DELETE|PATCH|PUT)` 0 hit) |
| ③ | content_type filter 强制 artifact_comment scope | 跟 DM-7 GET /messages/{id}/edit-history 区分 (DM-7 泛 message, CV-15 artifact comment scoped) | 非 artifact_comment 调本 endpoint → 404 reject (反向断言 message.ContentType == "artifact_comment") |
| ④ (边界) | 文案 byte-identical 跟 DM-7 EditHistoryModal 同源 | content-lock §1 (`编辑历史 / 暂无编辑记录 / 共 N 次编辑 / RFC3339 ts`) | 同义词反向 reject `changes/revisions/版本/修订/变更/回退` (跟 DM-7 同精神) |
| ⑤ (边界) | UpdateMessage SSOT 不漂 (reuse DM-7.2 既有 path) | DM-7.2 #558 改 content 前 SELECT old + JSON marshal append; CV-7 #535 既有 PATCH /messages/{id} 走 UpdateMessage SSOT 自动经 | 反向 grep `inline.*UPDATE.*messages.*content` 在 production 0 hit (DM-7 既有 path 不破); CV-15 read-only view 不另起 update path |
| ⑥ (边界) | AST 锁链延伸 forbidden 3 token + AL-1a reason 锁链不漂 | best-effort 立场代码层守 (跟 BPP-4..8 + DM-7/8 + CHN-15 同模式); reason 复用 DM-7 'unknown' byte-identical | `pendingCommentEdit/commentEditQueue/deadLetterCommentEdit` 在 internal/ 0 hit; CV-15 不写 admin_actions 不另起 reason 字典 |

## §1 立场 ① 0 schema 改 (架构核查)

- [ ] 反向 grep `migrations/cv_15_\d+|ALTER TABLE artifact_comments|CREATE TABLE.*artifact_comments|artifact_comment_history` count==0
- [ ] 复用 messages.edit_history 列 (DM-7.1 v=34 既有, 不重新加)
- [ ] CV-5 #530 立场 ① "comments 走 messages 表单源 不裂表" 锁链承袭

## §2 立场 ② owner-only sender + admin readonly

- [ ] GET /api/v1/channels/{channelId}/messages/{messageId}/comment-edit-history user-rail (sender == current user 反断, 别 user → 403; 空 history 返 `[]`)
- [ ] GET /admin-api/v1/messages/{messageId}/comment-edit-history admin readonly (admin god-mode 不挂 PATCH/DELETE/PUT — ADM-0 §1.3 红线)
- [ ] 反向 grep `mux.Handle("(POST|DELETE|PATCH|PUT).*admin-api/v[0-9]+/.*comment-edit-history` 在 internal/api/+server/ 0 hit
- [ ] owner-only ACL 锁链第 22 处

## §3 立场 ③ content_type='artifact_comment' filter 强制

- [ ] handler 加 `if msg.ContentType != "artifact_comment" → 404 comment.not_artifact_comment`
- [ ] 反向断言: 普通 message (text/image) 调本 endpoint → 404 (避免跟 DM-7 既有 GET /messages/{id}/edit-history 混淆)

## §4 立场 ④ 文案 byte-identical 跟 DM-7 同源

- [ ] `编辑历史` 4 字 modal title byte-identical 跟 DM-7 EditHistoryModal §1 同源
- [ ] `暂无编辑记录` 6 字 空 history 文案
- [ ] `共 N 次编辑` 5 字 count
- [ ] RFC3339 ts byte-identical 跟 DM-7 server-side time.Format("2006-01-02 15:04") 同精神
- [ ] 同义词反向 reject `changes/revisions/revs/版本/修订/变更/回退`

## §5 立场 ⑤ UpdateMessage SSOT 不漂

- [ ] CV-15 read-only view, 0 write path 新增
- [ ] 反向 grep `inline.*UPDATE.*messages.*content` 在 production 0 hit (DM-7 既有 path 不破)
- [ ] CV-7 #535 既有 PATCH /messages/{id} 自动 wrap UpdateMessage SSOT (artifact comments 编辑产生 edit_history 早已自动)

## §6 立场 ⑥ AST forbidden + AL-1a reason 锁链不漂

- [ ] AST scan `pendingCommentEdit/commentEditQueue/deadLetterCommentEdit` 0 hit
- [ ] CV-15 不写 admin_actions row (read-only view), 不另起 reason 字典 (复用 DM-7 既有 'unknown')

## §7 联签 (实施 PR 时填)

- [ ] 飞马 (spec ↔ 立场对齐 + 架构核查 v=45 → 0 schema 改 决策): _(签)_
- [ ] 烈马 (反向 grep + cov ≥84% + 6 反约束 0 hit + 0 schema reverse + content_type filter 守门): _(签)_
- [ ] 战马C (实施代码 ↔ 立场反查 6 项全过): _(签)_
