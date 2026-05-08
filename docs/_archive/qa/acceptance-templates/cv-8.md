# Acceptance Template — CV-8: artifact comment thread reply (1-level) ✅

> 蓝图 `canvas-vision.md` L24 字面 + CV-5 #530 + CV-7 #535 续 + thinking 5-pattern 第 6 处链 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8). Spec `cv-8-spec.md` (战马E v0 4f28e82) + Stance `cv-8-stance-checklist.md` + Content-lock `cv-8-content-lock.md`. 拆 PR: 整 milestone 一 PR (`feat/cv-8`).

## 验收清单

### §1 CV-8.1 — server thinking validator + 1-level depth gate (POST /channels/:id/messages)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 立场 ① 0 新 endpoint + 0 schema 改 — 复用 POST `/api/v1/channels/{channelId}/messages` + body `reply_to_id`; CV-8 仅加 ≤15 行 hook | grep + git diff | 战马E / 飞马 / 烈马 | `messages.go` diff ≤15 行 + git diff migrations/ 0 production 行 |
| 1.2 立场 ③ agent reply thinking 5-pattern reject — content_type=='artifact_comment' + reply_to_id 非 nil + sender Role=='agent' + 5-pattern → 400 `comment.thinking_subject_required` byte-identical CV-5/CV-7 | unit (4 sub-case) | 战马E / 飞马 / 烈马 | `messages_thread_test.go::TestCV8_AgentReplyThinking_Reject` (4 sub-case 全 reject byte-identical) |
| 1.3 立场 ④ depth 1 层强制 — parent.ReplyToID 非 nil → 400 `comment.thread_depth_exceeded` (reply on reply 拒) | unit | 战马E / 飞马 / 烈马 | `TestCV8_ReplyOnReply_Reject` (parent 是 reply 时再 reply → 400) |
| 1.4 立场 ④ reply target 校验 — parent.ContentType != 'artifact_comment' → 400 `comment.reply_target_invalid` | unit | 战马E / 烈马 | `TestCV8_ReplyOnNonComment_Reject` |
| 1.5 立场 ② human reply 不走 validator — sanity 反向断 | unit | 战马E / 烈马 | `TestCV8_HumanReply_AnyBodyOK` |

### §2 CV-8.2 — client ArtifactCommentThread.tsx + DOM/文案锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 thread collapse/expand toggle — `data-cv8-thread-toggle="<parent_id>"` DOM 锚 + 文案 "▼ 隐藏 N 条回复" / "▶ 显示 N 条回复" byte-identical | vitest (3 case) | 战马E / 烈马 | `ArtifactCommentThread.test.tsx::collapse_expand` (3 case: 默认折叠 / click expand 文案切换 / DOM data-attr 反向锁) |
| 2.2 reply button — `data-cv8-reply-target="<parent_id>"` DOM 锚 + click 打开 reply input | vitest | 战马E / 烈马 | `ArtifactCommentThread.test.tsx::reply_button` |
| 2.3 立场 ④ depth 1 层 client 反约束 — reply 渲染中不再渲染 reply button (反向 grep DOM ≥0 in nested reply) | vitest | 战马E / 烈马 | `ArtifactCommentThread.test.tsx::no_recursive_reply` (reply 内 data-cv8-reply-target count==0) |

### §3 CV-8.3 — e2e + REG-CV8-001..006

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e: human owner reply on artifact_comment-typed message → POST 200 + GET 见 reply_to_id | E2E | 战马E / 烈马 | `cv-8-comment-thread-reply.spec.ts::human reply` |
| 3.2 e2e: agent reply thinking 5-pattern → 4 sub-case reject 400 (5-pattern 第 6 处链) | E2E | 战马E / 飞马 / 烈马 | `cv-8-comment-thread-reply.spec.ts::agent reply thinking reject` |
| 3.3 e2e: reply on reply (depth 2) → 400 `comment.thread_depth_exceeded` (1-level 强制) | E2E | 战马E / 烈马 | `cv-8-comment-thread-reply.spec.ts::depth 2 reject` |
| 3.4 e2e: reply on plain-text message (非 artifact_comment) → 400 `comment.reply_target_invalid` | E2E | 战马E / 烈马 | `cv-8-comment-thread-reply.spec.ts::reply on non-comment reject` |
| 3.5 e2e: cross-channel reject — non-member reply → 403 byte-identical | E2E | 战马E / 烈马 | `cv-8-comment-thread-reply.spec.ts::cross-channel reject` |
| 3.6 反向 grep 6 锚: 4 处 0 hit + DOM/文案 ≥1 hit (cv-8-spec.md §3 字面) | CI grep | 飞马 / 烈马 | CI lint 每 CV-8 PR 必跑 |

## 边界

- CV-5 #530 (artifact_comments handler) / CV-7 #535 (edit/delete + thinking 5-pattern 第 5 处链) / messages.go (POST/PUT/DELETE existing) / ADM-0 §1.3 admin rail 红线 / REG-INV-002 fail-closed / 5-pattern thinking 锁链 RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 byte-identical 6 处

## 退出条件

- §1 (5) + §2 (3) + §3 (6) 全绿
- 0 schema 改
- 反向 grep 6 锚通过
- 5-pattern 第 6 处链 byte-identical
- 登记 REG-CV8-001..006
