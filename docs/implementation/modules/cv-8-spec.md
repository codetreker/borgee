# CV-8 spec brief — artifact comment thread reply (CV-5/CV-7 续, 1-level thread)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) L24 字面 "Linear issue + comment" + CV-5 #530 messages 表单源 + CV-7 #535 thinking 5-pattern 锁链第 5 处. CV-8 是 1-level thread reply — 复用 `messages.reply_to_id` 既有列 (schema 既有, 0 ALTER), 复用 POST `/api/v1/channels/{channelId}/messages` 既有 endpoint (字段 reply_to_id 已支持).

## 0. 关键约束 (4 项立场, 蓝图字面 + 跨链)

1. **reply 走 messages 表既有 endpoint + reply_to_id 既有列, 0 新 endpoint + 0 schema 改** (CV-5/CV-7 单源延伸): POST `/api/v1/channels/{channelId}/messages` 既有 + body 字段 `reply_to_id` 既有 (messages.go 行 199 + Store.CreateMessageFull 行 258). CV-8 仅:
   - 在 message create 路径加 thinking validator hook 当 content_type=='artifact_comment' && reply_to_id 非 nil && sender_role=='agent';
   - 客户端渲染 thread collapse/expand (≤1 层 thread, 反约束 不开 N-deep recursion).
   **反约束**: 不开 `/api/v1/comments/:id/replies` 别名 endpoint / 不开 `comment_threads` 表 / 不开 `parent_comment_id` 新列. 反向 grep `\/comments\/.*\/replies\|comment_threads.*PRIMARY\|parent_comment_id` 在 internal/ count==0.

2. **owner-only ACL byte-identical 跟既有 messages 同源 (11+ 处一致, admin god-mode 不挂)**: reply create = channel-member ACL (existing), reply edit/delete = sender_id==user.id (CV-7 #535 已盖). admin god-mode 不入 `/api/v1/messages/*` rail (跟 CV-5 立场 ④ + CV-7 立场 ② + ADM-0 §1.3 同源). **反向 grep**: `admin.*messages.*reply\|admin.*\/messages\/.*\/replies` count==0.

3. **agent reply 必 validate thinking subject (5-pattern 第 6 处链)** (CV-5/CV-7 延伸; 蓝图 realtime §1.1 ⭐ + RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 同链, CV-8 是第 6 处): handleCreateMessage 在 content_type=='artifact_comment' + reply_to_id 非 nil + sender Role=='agent' 时, 跑 5-pattern; 命中 → reject 400 `comment.thinking_subject_required` byte-identical (跟 CV-5/CV-7 同字符串). **反向 grep**: 5-pattern 字面 (`body.*"thinking"$|defaultSubject|fallbackSubject|"AI is thinking"|subject\s*=\s*""`) 在 internal/api/ 排除 _test.go count==0; **5-pattern 改 = 改 6 处** byte-identical.

4. **thread collapse/expand 文案 + DOM 锁 byte-identical 跨链** (content-lock 必锁): collapse 字面 "▼ 隐藏 N 条回复" / expand "▶ 显示 N 条回复" (跟 RT-2 既有 thread 文案同源若有, 否则新锁) + DOM `data-cv8-thread-toggle="<parent_id>"` (跟 CV-7 `data-cv7-*` 同模式承袭) + reply button `data-cv8-reply-target="<parent_id>"`. **反约束**: thread depth 1 层 (反向 grep `cv8.*depth.*[2-9]\|cv8.*recursive\|cv8.*nested.*reply` 0 hit) — reply on a reply 必拒 (server 验 reply_to_id 指向的 message.reply_to_id 必须为 nil).

## 1. 拆段实施 (3 段, 一 milestone 一 PR 协议)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-8.1 server | `internal/api/messages.go::handleCreateMessage` 改 (≤15 行 — 加 thinking validator hook + 1-level depth gate) + `internal/api/cv_8_thread_validator.go` (新, 复用 CV-5/CV-7 5-pattern regex byte-identical) | content_type=='artifact_comment' + reply_to_id 非 nil 时: (a) 查 parent.ContentType 必须 'artifact_comment', 否则 400 `comment.reply_target_invalid`; (b) 查 parent.ReplyToID 必须 nil (1-level depth 强制), 否则 400 `comment.thread_depth_exceeded`; (c) sender Role=='agent' 跑 5-pattern. 错误码 byte-identical CV-5/CV-7 复用 + 2 新错码. 0 schema 改 |
| CV-8.2 client | `packages/client/src/components/ArtifactCommentThread.tsx` (新) + content-lock | thread 渲染 collapse/expand toggle (`data-cv8-thread-toggle`) + reply button (`data-cv8-reply-target`) + reply input modal + 文案 "▼ 隐藏 N 条回复" / "▶ 显示 N 条回复" byte-identical |
| CV-8.3 e2e + closure | `packages/e2e/tests/cv-8-comment-thread-reply.spec.ts` (新) + REG-CV8-001..006 + acceptance template + PROGRESS [x] | 6 case: human reply on comment OK / agent reply thinking 4-pattern reject / depth 2 reject (reply on reply 拒) / reply on non-comment-typed reject / cross-channel reject (REG-INV-002 fail-closed) / collapse/expand 文案 byte-identical |

## 2. 错误码 byte-identical (跟 CV-5 #530 + CV-7 #535 复用 + 2 新)

- `comment.thinking_subject_required` — agent reply 命中 5-pattern reject (跟 CV-5 + CV-7 同字符第 6 处链)
- `comment.reply_target_invalid` — reply 目标 message 非 artifact_comment 类型 (新)
- `comment.thread_depth_exceeded` — reply on reply 拒 (1-level thread, 新, 立场 ④)

## 3. 反向 grep 锚 (CV-8 实施 PR 必跑)

```
git grep -nE '\/comments\/.*\/replies|comment_threads.*PRIMARY|parent_comment_id' packages/server-go/internal/  # 0 hit (单源不裂)
git grep -nE 'admin.*messages.*reply|admin.*\/messages\/.*\/replies' packages/server-go/internal/api/admin   # 0 hit (ADM-0 §1.3)
git grep -nE 'body.*"thinking"$|defaultSubject|fallbackSubject|"AI is thinking"|subject\s*=\s*""' packages/server-go/internal/api/ --include='*.go' --exclude='*_test.go'  # 0 hit (5-pattern 第 6 处链)
git grep -nE 'cv8.*depth.*[2-9]|cv8.*recursive|cv8.*nested.*reply' packages/server-go/  # 0 hit (1-level thread 强制)
git grep -nE 'data-cv8-thread-toggle|data-cv8-reply-target' packages/client/src/  # ≥ 2 hit (DOM 锚)
git grep -nE '隐藏 N 条回复|显示 N 条回复' packages/client/src/  # ≥ 2 hit (collapse/expand 文案 byte-identical)
```

## 4. 不在本轮范围 (deferred)

- ❌ N-level deep thread (1-level only — 立场 ④ 反约束强制; v2+ 再议)
- ❌ thread @-mention 在 reply 内 (DM-2.2 mention 走既有, 跟 reply 路径不耦合; CV-8 不另开 reply-mention 路径)
- ❌ thread reaction (复用 CV-7 #535 reaction)
- ❌ admin god-mode 看/改 thread (ADM-0 §1.3 红线)
- ❌ 新 schema migration (0 schema 改, messages.reply_to_id 既有列覆盖)
