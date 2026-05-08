# Acceptance Template — CV-7: artifact comment edit / delete / reaction ✅

> 蓝图 `canvas-vision.md` L24 字面 + CV-5 #530 续 + thinking 5-pattern 第 5 处链 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7). Spec `cv-7-spec.md` (战马E v0 192743e) + Stance `cv-7-stance-checklist.md` + Content-lock `cv-7-content-lock.md`. 拆 PR: 整 milestone 一 PR (`feat/cv-7`). Owner: 战马E 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 CV-7.1 — server thinking validator hook 加挂 (PUT /messages/{id})

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 立场 ① 0 新 endpoint — PUT/DELETE `/api/v1/messages/{id}` + reactions endpoint 既有不动, CV-7 仅 hook 加 thinking validator (≤10 行 messages.go diff) | grep + git diff | 战马E / 飞马 / 烈马 | `internal/api/messages.go` diff ≤10 行 + 反向 grep `PATCH.*artifacts.*comments\|DELETE.*artifacts.*comments` 0 hit |
| 1.2 立场 ③ agent edit 5-pattern reject — content_type=='artifact_comment' + sender Role=='agent' + new content 命中 5-pattern → 400 `comment.thinking_subject_required` byte-identical CV-5 (5-pattern 第 5 处链) | unit (5 sub-case) | 战马E / 飞马 / 烈马 | `messages_test.go::TestCV7_AgentEditThinkingSubject_Reject` (5 sub-case 全 reject + error code byte-identical) |
| 1.3 立场 ② human edit 不走 validator — sender Role=='human' edit 任意 body 通过 (反约束 sanity) | unit | 战马E / 烈马 | `TestCV7_HumanEditAnyBody_OK` |
| 1.4 立场 ② owner-only — 非 sender edit/delete → 403 既有不变 | unit (existing) | 战马E / 烈马 | 既有 `TestPutMessage_ForbiddenOtherUser` 类 PASS (CV-7 不破) |
| 1.5 立场 ① 0 schema 改 — git diff packages/server-go/internal/migrations/ 仅 _test.go 或空 | grep + git diff | 飞马 / 烈马 | `git diff main..HEAD -- packages/server-go/internal/migrations/` 0 production 行 |

### §2 CV-7.2 — client edit modal + delete confirm + reaction button

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 立场 ④ edit 按钮 `data-cv7-edit-btn` DOM 锁 — 仅 sender==current user 渲染; click → modal 打开 + textarea + 保存 调 PUT | vitest (3 case) | 战马E / 烈马 | `ArtifactComments.test.tsx::edit_btn` (own 渲染 / other 不渲染 / click 触发 modal) |
| 2.2 立场 ④ delete confirm 文案 byte-identical "确认删除这条评论?" | vitest | 战马E / 烈马 | `ArtifactComments.test.tsx::delete_confirm` (window.confirm spy + literal byte-identical) |
| 2.3 立场 ④ reaction button `data-cv7-reaction-target="<msg_id>"` DOM 锁 — click → PUT /reactions 调 | vitest | 战马E / 烈马 | `ArtifactComments.test.tsx::reaction_btn` (DOM data-attr + click action) |

### §3 CV-7.3 — e2e + REG-CV7-001..006 + content-lock 反向锚

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e: human owner edit 自己 comment → PUT 200 + GET 见新 body + edited_at 非空 | E2E | 战马E / 烈马 | `cv-7-comment-edit-delete.spec.ts::human edit own` |
| 3.2 e2e: agent edit thinking 5-pattern → 5 sub-case reject 400 (5-pattern 第 5 处链) | E2E | 战马E / 飞马 / 烈马 | `cv-7-comment-edit-delete.spec.ts::agent thinking edit reject` |
| 3.3 e2e: delete own → 200 + GET 不再出现 (deleted_at 非空) | E2E | 战马E / 烈马 | `cv-7-comment-edit-delete.spec.ts::delete own` |
| 3.4 e2e: 非 sender edit/delete 别人 comment → 403 byte-identical | E2E | 战马E / 烈马 | `cv-7-comment-edit-delete.spec.ts::edit other 403` |
| 3.5 e2e: reaction +1 -1 round-trip — PUT 200 + GET reactions count==1 + DELETE 200 + count==0 | E2E | 战马E / 烈马 | `cv-7-comment-edit-delete.spec.ts::reaction roundtrip` |
| 3.6 反向 grep 5 锚: 4 处 0 hit + DOM/文案 ≥1 hit (cv-7-spec.md §3 字面) | CI grep | 飞马 / 烈马 | CI lint 每 CV-7 PR 必跑 |

## 边界

- CV-5 #530 (artifact_comments handler messages 表单源 + thinking validator) / messages.go (PUT/DELETE existing endpoint) / reactions.go (existing) / ADM-0 §1.3 admin rail 红线 / REG-INV-002 fail-closed / 5-pattern thinking 锁链 RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 byte-identical 5 处

## 退出条件

- §1 (5) + §2 (3) + §3 (6) 全绿 — 一票否决
- 0 schema 改
- 反向 grep 5 锚通过 (4 处 0 hit + DOM/文案 ≥1 hit)
- 5-pattern 第 5 处链 byte-identical (5 处 errcode 同字符)
- 登记 REG-CV7-001..006
