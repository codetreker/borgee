# CV-9 spec brief — artifact comment notification (mention fan-out, CV-5..CV-8 续)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) L24 字面 "Linear issue + comment" + DM-2.2 #372 mention router 同精神 + RT-3 #488 cursor 共序锚 + thinking 5-pattern 锁链第 7 处 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 + CV-9). CV-9 让 artifact comment 内的 `@<user_id>` mention 复用 DM-2.2 既有 fan-out (无须任何 server 实施改动 — POST /channels/:id/messages 已经跑 MentionDispatcher), 仅加 client unread badge UI.

## 0. 关键约束 (4 项立场, 蓝图字面 + 跨链)

1. **mention fan-out 复用 DM-2.2 既有 path, 0 server 实施改动** (CV-5..CV-8 单源延伸 + DM-2.2 复用): POST /api/v1/channels/{channelId}/messages 已经在落库前调用 `MentionDispatcher.MentionTargetsFromBody` 解 `@<uuid>` token (messages.go:249); content_type=='artifact_comment' 也走同 path (CV-7 #535 whitelist 已加). Dispatch (online → MentionPushed frame, offline agent → owner system DM fallback) 已挂.
   **反约束**: 不开 `/api/v1/artifacts/:id/comments/:cid/mention` 别名 endpoint / 不开 `comment_mentions` 新表 / 不另写 fan-out 路径; 反向 grep `cv9.*fanout\|cv9.*dispatch\|comment_mentions.*PRIMARY` 在 internal/ count==0.

2. **owner-only ACL byte-identical 12+ 处一致, admin god-mode 不挂**: comment-mention dispatch 走既有 `cross-channel reject` (mention.target_not_in_channel 400) + REG-INV-002 fail-closed; admin 不入 mention 路径 (跟 ADM-0 §1.3 + CV-5/CV-7/CV-8 同源). **反向 grep**: `admin.*mention.*comment\|admin.*comment.*mention` 在 admin*.go count==0.

3. **agent comment 内的 mention 仍走 thinking 5-pattern 第 7 处链** (CV-5/CV-7/CV-8 延伸; 5-pattern 在 POST 路径已挂 CV-8.1 hook): handleCreateMessage 在 content_type=='artifact_comment' + reply_to_id != nil + agent + 5-pattern 命中 → reject 400 byte-identical (CV-8 #537 既有), 即使 body 含 mention 也照 reject. **反向 grep**: 5-pattern 字面 count==0 在 internal/api/ 排除 _test.go; **5-pattern 改 = 改 7 处** byte-identical (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 + CV-9).

4. **client unread badge UI — DOM `data-cv9-unread-count` + `data-cv9-mention-toast` + 文案 byte-identical**: ArtifactComments header 渲染 unread count badge (基于 RT-3 + DM-2.2 既有 mention frame 推送), toast 字面 "你被 @ 在 N 条评论中" / 0 时不渲染 (跟 DM-2.2 既有 mention badge 文案同源). **反约束**: 不另写 mention badge state 管理 (复用 RT-2/DM-2.2 既有 mention frame listener), 反向 grep `useCV9MentionState\|cv9-mention-state` 0 hit.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-9.1 server | (无 server 实施 — 既有 POST /channels/:id/messages 已挂 MentionDispatcher; CV-7 whitelist 加 'artifact_comment' 已盖); 仅加 unit 验证 mention dispatch 在 artifact_comment-typed message 真触发 (跟 text-typed 等价) | server-side 0 行新增 production code; 加 1 unit `TestCV9_ArtifactComment_TriggersMentionDispatch` 反向断 mention dispatch 真跑 |
| CV-9.2 client | `packages/client/src/components/ArtifactCommentsMentionBadge.tsx` (新) + content-lock | header 渲染 unread mention count badge (`data-cv9-unread-count`) + 复用 useMentionPushed hook 既有, click → scroll-to-message; 文案 "你被 @ 在 N 条评论中" byte-identical |
| CV-9.3 e2e + closure | `packages/e2e/tests/cv-9-comment-mention.spec.ts` (新) + REG-CV9-001..006 + acceptance + PROGRESS [x] | 5 case: human comment with @user → mention frame 真到 / agent comment with @ + 5-pattern body → reject 400 / non-channel-member 被 mention → 400 mention.target_not_in_channel / artifact_comment-typed 跟 text-typed mention dispatch 等价 / cross-channel reject |

## 2. 错误码 byte-identical (跟 DM-2.2 + CV-5..CV-8 复用, 0 新错码)

- `mention.target_not_in_channel` — DM-2.2 既有, mention 目标非 channel member reject (反约束 cross-channel)
- `comment.thinking_subject_required` — CV-5/CV-7/CV-8/CV-9 同字符 (5-pattern 第 7 处链)

## 3. 反向 grep 锚 (CV-9 实施 PR 必跑)

```
git grep -nE 'cv9.*fanout|cv9.*dispatch|comment_mentions.*PRIMARY|/comments/.*/mention' packages/server-go/internal/  # 0 hit (单源)
git grep -nE 'admin.*mention.*comment|admin.*comment.*mention' packages/server-go/internal/api/admin   # 0 hit (ADM-0 §1.3)
git grep -nE 'body.*"thinking"$|defaultSubject|fallbackSubject|"AI is thinking"|subject\s*=\s*""' packages/server-go/internal/api/ --include='*.go' --exclude='*_test.go'  # 0 hit (第 7 处链)
git grep -nE 'data-cv9-unread-count|data-cv9-mention-toast' packages/client/src/  # ≥ 2 hit (DOM 锚)
git grep -nE '你被 @ 在 N 条评论中' packages/client/src/  # ≥ 1 hit (文案 byte-identical, "N" 是占位符)
git grep -nE 'useCV9MentionState|cv9-mention-state' packages/client/src/  # 0 hit (反约束: 不另起 state)
```

## 4. 不在本轮范围 (deferred)

- ❌ comment mention 反向频率限流 (Phase 6 spam 防护单独 milestone)
- ❌ admin god-mode 看 mention 列表 (ADM-0 §1.3 红线)
- ❌ comment mention webhook (Phase 7+)
- ❌ 新 schema migration (0 schema 改, mentions 表 + messages.content 既有覆盖)
- ❌ comment 内 `@everyone` / `@here` 群广播 (DM-2.2 立场不开, CV-9 承袭)
