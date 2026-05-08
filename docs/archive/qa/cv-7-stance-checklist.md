# CV-7 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · 立场 review checklist (跟 CV-5 #530 续 + thinking 5-pattern 第 5 处链 RT-3/BPP-2.2/AL-1b/CV-5/CV-7)
> **目的**: CV-7 三段实施 PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off.
> **关联**: spec `docs/implementation/modules/cv-7-spec.md` (战马E v0 192743e) + acceptance `docs/qa/acceptance-templates/cv-7.md` + content-lock `docs/qa/cv-7-content-lock.md`.

## §0 立场总表 (4 立场 + 3 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | edit/delete/reaction 走 messages 表既有 endpoint — PUT/DELETE `/api/v1/messages/{id}` + PUT/DELETE `/api/v1/messages/{id}/reactions` 既有, **0 新 endpoint** / **0 新表** | CV-5 #530 立场 ① + canvas-vision.md L24 字面 | 反向 grep `PATCH.*artifacts.*comments\|DELETE.*artifacts.*comments\|artifact_reactions.*PRIMARY` 在 internal/ count==0 |
| ② | owner-only ACL byte-identical 跟既有 messages 同源 — edit/delete = sender_id==user.id; admin god-mode 不挂 | ADM-0 §1.3 + CV-5 立场 ④ + concept-model §4 | 反向 grep `admin.*messages.*\/edit\|admin.*PATCH.*messages\|admin.*DELETE.*messages` 在 internal/api/admin*.go count==0 |
| ③ | edit 后必重新 validate thinking subject — sender_role==agent + content_type=='artifact_comment' → 5-pattern reject 400 byte-identical CV-5 (5-pattern 第 5 处链) | 蓝图 realtime §1.1 ⭐ + RT-3/BPP-2.2/AL-1b/CV-5 同链 | 反向 grep `body.*"thinking"$\|defaultSubject\|fallbackSubject\|"AI is thinking"\|subject\s*=\s*""` 在 internal/api/ 排除 _test.go count==0; 5-pattern 改 = 改 5 处 byte-identical |
| ④ | delete confirm + reaction button 文案 byte-identical 跨链 — DOM `data-cv7-edit-btn` / `data-cv7-reaction-target` 必锚, 文本 "确认删除这条评论?" byte-identical | content-lock 必锁 (UI) | 反向 grep `data-cv7-edit-btn\|data-cv7-reaction-target` 在 client/src/ count≥2; `确认删除这条评论\?` count≥1 |

## §1 立场 ① edit/delete/reaction 走既有 endpoint (CV-7.1 守)

**蓝图字面源**: CV-5 #530 立场 ① + canvas-vision.md L24

**反约束清单**:

- [ ] PUT `/api/v1/messages/{id}` (existing handleUpdateMessage) 不改 signature, 仅加 thinking validator hook (≤10 行)
- [ ] DELETE `/api/v1/messages/{id}` (existing) 完全不改 — comment 删除走既有 sender 自检 + soft-delete (deleted_at)
- [ ] PUT/DELETE `/api/v1/messages/{id}/reactions` (existing) 完全不改 — reaction 走既有 channel-member ACL
- [ ] 0 新表 — git diff packages/server-go/internal/migrations/ 0 改
- [ ] 反向 grep `PATCH.*artifacts.*comments\|DELETE.*artifacts.*comments\|artifact_reactions.*PRIMARY` 0 hit

## §2 立场 ② owner-only + admin god-mode 不挂

**反约束清单**:

- [ ] handleUpdateMessage 行 335 既有 `existing.SenderID != user.ID → 403` 不动
- [ ] handleDeleteMessage 同 sender 自检不动
- [ ] reaction PUT/DELETE 走既有 channel-member ACL 不动
- [ ] 反向 grep `admin.*messages.*\/edit\|admin.*PATCH.*messages\|admin.*DELETE.*messages` 在 admin*.go 0 hit
- [ ] cross-org 既有 `store.CrossOrg(user.OrgID, existing.OrgID) → 403` 不动

## §3 立场 ③ edit thinking subject 5-pattern 第 5 处链

**反约束清单**:

- [ ] handleUpdateMessage 在 existing.ContentType=='artifact_comment' 时, 查 sender User.Role
- [ ] sender Role=='agent' + 新 content 命中 5-pattern → reject 400 `comment.thinking_subject_required`
- [ ] 错误码 byte-identical 跟 CV-5 #530 同字符串 (`comment.thinking_subject_required`)
- [ ] human edit (sender Role!='agent') 不走此 validator — body 文本自由
- [ ] 5-pattern reverse-grep 守门: `body.*"thinking"$\|defaultSubject\|fallbackSubject\|"AI is thinking"\|subject\s*=\s*""` 在 internal/api/ 排除 _test.go count==0
- [ ] 5-pattern 改 = 改 5 处 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7) byte-identical

## §4 立场 ④ DOM 锁 + 文案锁 (content-lock)

**反约束清单**:

- [ ] edit 按钮渲染 `data-cv7-edit-btn` (反向 grep ≥1 hit, 仅 sender==current user 时渲染)
- [ ] reaction button 渲染 `data-cv7-reaction-target="<msg_id>"` (反向 grep ≥1 hit)
- [ ] delete confirm 字面 "确认删除这条评论?" byte-identical (反向 grep ≥1 hit)
- [ ] reaction 不引入新 emoji picker (复用现有 message reaction unicode 集)

## §5 蓝图边界 ⑤⑥⑦ — fail-closed / forward-only / 不裂表

**反约束清单**:

- [ ] cross-channel reject 跟 CV-5 同源 (REG-INV-002 fail-closed; comment edit/delete 都需 channel member)
- [ ] forward-only — comment edit 即覆写 content + edited_at, 不留 history table
- [ ] 不裂表 — 0 schema 改 (git diff internal/migrations/ 0)

## §6 退出条件

- §1 (5) + §2 (5) + §3 (6) + §4 (4) + §5 (3) 全 ✅
- 反向 grep 5 锚: 4 处 0 hit + DOM/文案 ≥1 hit
- e2e 6 case 全 PASS
- 0 schema 改 (git diff packages/server-go/internal/migrations/ 仅 _test.go 或空)
- 5-pattern 第 5 处链 byte-identical (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 errcode 同字符)
