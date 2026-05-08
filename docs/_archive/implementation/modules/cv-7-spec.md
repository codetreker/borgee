# CV-7 spec brief — artifact comment edit / delete / reaction (CV-5 续)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) L24 字面 "Linear issue + comment" + CV-5 #530 messages 表单源 + thinking 5-pattern 第 4 处链承袭. CV-7 是 CV-5 续 — comment edit/delete/reaction 三动作 byte-identical 复用 messages 表既有 PUT/DELETE/reactions endpoint, **0 新表**.

## 0. 关键约束 (4 项立场, 蓝图字面 + 跨链)

1. **edit/delete/reaction 走 messages 表既有 endpoint, 0 新 endpoint** (CV-5 立场 ① 单源延伸 — comment 是 message, message PUT/DELETE/reactions 既有自动覆盖): PUT `/api/v1/messages/{messageId}` (existing) + DELETE `/api/v1/messages/{messageId}` (existing) + PUT `/api/v1/messages/{messageId}/reactions` (existing) 三 endpoint **不动 server signature**, CV-7 仅加 thinking validator hook + 新 WS frame fan-out. **反约束**: 不开 `/api/v1/artifacts/:id/comments/:cid` 别名 endpoint / 不开 `artifact_comment_edited` 单独 row 类型 / 不开 `artifact_reactions` 表; 反向 grep `PATCH.*artifacts.*comments\|DELETE.*artifacts.*comments\|artifact_reactions.*PRIMARY` 在 internal/ count==0.

2. **owner-only ACL byte-identical 跟既有 messages 同源 (10+ 处一致, admin god-mode 不挂)**: edit/delete = sender_id==user.id (existing handleUpdateMessage line 335 + handleDeleteMessage 同; CV-7 不收紧不放宽). reaction = channel-member (任何 member 可 +1 -1). admin god-mode (admin_sessions cookie) 不入 `/api/v1/messages/*` rail (跟 ADM-0 §1.3 + CV-5 立场 ④ 同源). **反向 grep**: `admin.*messages.*\/edit\|admin.*PATCH.*messages\|admin.*DELETE.*messages` 在 internal/api/admin*.go count==0.

3. **edit 后必重新 validate thinking subject (5-pattern 第 5 处链)** (CV-5 立场 ③ 延伸; 蓝图 realtime §1.1 ⭐ + RT-3 #488 + BPP-2.2 #485 + AL-1b #482 + CV-5 #530 同链, CV-7 是第 5 处): handleUpdateMessage 在 sender_role==agent + content_type=='artifact_comment' 时, 复用 CV-5 既有 `violatesThinkingSubject` 函数, 命中 5-pattern → reject 400 `comment.thinking_subject_required` (错误码 byte-identical 跟 CV-5 同). **反向 grep**: 5-pattern 字面 (`body.*"thinking"$|defaultSubject|fallbackSubject|"AI is thinking"|subject\s*=\s*""`) 在 internal/api/ 排除 _test.go count==0; **5-pattern 改 = 改 5 处** (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 byte-identical).

4. **delete confirm + reaction 文案 byte-identical 跨链** (跟 CV-2.3 / DM-* 既有删除/反应 UI 同源, content-lock 必锁 DOM): delete confirm 字面 "确认删除这条评论?" (跟 CV-2.3 anchor comment delete 同源若有, 否则新锁) / reaction button data-attr `data-cv7-reaction-target="<msg_id>"` (跟 CM-5.3 透明协作 hover data-attr 同模式). **反约束**: 不重写既有 delete confirm 文本 / 不另起 emoji picker (复用现有 reactions UI).

## 1. 拆段实施 (3 段, 一 milestone 一 PR 协议)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-7.1 server | `internal/api/messages.go::handleUpdateMessage` 改 (≤10 行 — 加 thinking validator hook) + `internal/api/artifact_comments.go` export `ViolatesThinkingSubject` (lower→upper) | 仅在 existing.ContentType=='artifact_comment' && existing.SenderID role=='agent' 时跑 5-pattern; 错误码 byte-identical CV-5; reaction/delete path 不加新逻辑 (existing 自动覆盖); 0 schema 改 |
| CV-7.2 client | `packages/client/src/components/ArtifactComments.tsx` 改 + `packages/client/src/components/ArtifactCommentEditModal.tsx` (新) + content-lock | edit 按钮仅 sender==user 渲染 (反约束 grep `data-cv7-edit-btn` count≥1) + delete confirm "确认删除这条评论?" byte-identical + reaction button (复用既有 emoji picker) data-attr `data-cv7-reaction-target` |
| CV-7.3 e2e + closure | `packages/e2e/tests/cv-7-comment-edit-delete.spec.ts` (新) + REG-CV7-001..006 + acceptance template + PROGRESS [x] | 6 case: human edit own comment OK / agent edit thinking 5-pattern reject (5 sub-case) / delete own OK / delete other 403 / reaction +1 -1 round-trip / cross-channel reject (REG-INV-002 fail-closed 跟 CV-5 同源) |

## 2. 错误码 byte-identical (跟 CV-5 #530 复用, 不另起命名)

- `comment.thinking_subject_required` — agent edit 后命中 5-pattern reject (跟 CV-5 同字符串第 5 处)
- 既有 `Can only edit your own messages` / `Cannot edit deleted message` — messages.go 字面不改

## 3. 反向 grep 锚 (CV-7 实施 PR 必跑)

```
git grep -nE 'PATCH.*artifacts.*comments|DELETE.*artifacts.*comments|artifact_reactions.*PRIMARY' packages/server-go/internal/   # 0 hit (单源不裂)
git grep -nE 'admin.*messages.*edit|admin.*PATCH.*messages' packages/server-go/internal/api/admin   # 0 hit (ADM-0 §1.3)
git grep -nE 'body.*"thinking"$|defaultSubject|fallbackSubject|"AI is thinking"|subject\s*=\s*""' packages/server-go/internal/api/ --include='*.go' --exclude='*_test.go'  # 0 hit (5-pattern 第 5 处链)
git grep -nE 'data-cv7-edit-btn|data-cv7-reaction-target' packages/client/src/  # ≥ 2 hit (content-lock DOM 锚)
git grep -nE '确认删除这条评论\?' packages/client/src/  # ≥ 1 hit (delete confirm byte-identical)
```

## 4. 不在本轮范围 (deferred)

- ❌ comment thread reply (CV-7.4+ Phase 6, 跟 anchor reply 复用)
- ❌ comment edit history audit (forward-only, edit 即覆写 content + edited_at; 跟 admin_actions 不挂)
- ❌ admin god-mode 看/改 comment (ADM-0 §1.3 红线)
- ❌ reaction 自定义 emoji (复用现有 message reaction unicode 集)
- ❌ 新 schema migration (0 schema 改, 既有 messages.edited_at + deleted_at 既有列覆盖)
