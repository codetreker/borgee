# CV-8 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · 立场 review checklist (跟 CV-5 #530 + CV-7 #535 续 + thinking 5-pattern 第 6 处链)
> **关联**: spec `docs/implementation/modules/cv-8-spec.md` (战马E v0 4f28e82) + acceptance `docs/qa/acceptance-templates/cv-8.md` + content-lock `docs/qa/cv-8-content-lock.md`.

## §0 立场总表 (4 立场 + 3 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | reply 走 messages 表既有 endpoint + reply_to_id 既有列, **0 新 endpoint** + **0 schema 改** | CV-5 #530 立场 ① + CV-7 #535 立场 ① | 反向 grep `\/comments\/.*\/replies\|comment_threads.*PRIMARY\|parent_comment_id` 在 internal/ count==0; git diff migrations/ 0 行 |
| ② | owner-only ACL byte-identical 跟既有 messages 同源; admin god-mode 不挂 | ADM-0 §1.3 + CV-5 立场 ④ + CV-7 立场 ② | 反向 grep `admin.*messages.*reply\|admin.*\/messages\/.*\/replies` 在 admin*.go count==0 |
| ③ | agent reply 必 validate thinking subject — 5-pattern 第 6 处链 byte-identical | RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 | 反向 grep `body.*"thinking"$\|defaultSubject\|fallbackSubject\|"AI is thinking"\|subject\s*=\s*""` count==0; 改 = 改 6 处 |
| ④ | thread depth 1 层强制 — reply on reply 必拒 (server `parent.ReplyToID` 必 nil); content-lock 必锁 collapse/expand 文案 + DOM | content-lock 必锁 (UI) + 反约束 N-deep recursion | 反向 grep `cv8.*depth.*[2-9]\|cv8.*recursive\|cv8.*nested.*reply` count==0; `data-cv8-thread-toggle\|data-cv8-reply-target` count≥2 hit; "隐藏/显示 N 条回复" 文案 ≥2 hit |

## §1 立场 ① 0 新 endpoint + 0 schema 改 (CV-8.1 守)

**反约束清单**:

- [ ] POST `/api/v1/channels/{channelId}/messages` 既有 + body `reply_to_id` 既有, 不动 signature
- [ ] CV-8 仅加 ≤15 行 thinking validator + 1-level depth gate hook 在 handleCreateMessage
- [ ] 0 新表 — git diff packages/server-go/internal/migrations/ 0 production 行
- [ ] 反向 grep `\/comments\/.*\/replies\|comment_threads.*PRIMARY\|parent_comment_id` 0 hit

## §2 立场 ② owner-only + admin god-mode 不挂

**反约束清单**:

- [ ] reply create = channel-member ACL (existing handleCreateMessage 不动)
- [ ] reply edit/delete 走 CV-7 #535 既有 sender 自检 (CV-8 不动)
- [ ] 反向 grep `admin.*messages.*reply\|admin.*\/messages\/.*\/replies` 在 admin*.go 0 hit
- [ ] cross-org 既有 `store.CrossOrg` 不动

## §3 立场 ③ agent reply thinking 5-pattern 第 6 处链

**反约束清单**:

- [ ] handleCreateMessage 在 content_type=='artifact_comment' && reply_to_id != nil 时, 查 sender User.Role
- [ ] sender Role=='agent' + new content 命中 5-pattern → reject 400 `comment.thinking_subject_required`
- [ ] 错误码 byte-identical 跟 CV-5/CV-7 同字符串
- [ ] human reply (Role!='agent') 不走此 validator
- [ ] 5-pattern reverse-grep 守门 0 hit
- [ ] 5-pattern 改 = 改 6 处 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8) byte-identical

## §4 立场 ④ thread depth 1 层 + 文案 + DOM 锁

**反约束清单**:

- [ ] reply 路径必查 parent.ContentType=='artifact_comment'; 否则 400 `comment.reply_target_invalid`
- [ ] reply 路径必查 parent.ReplyToID==nil (即 parent 本身不是 reply); 否则 400 `comment.thread_depth_exceeded`
- [ ] DOM `data-cv8-thread-toggle="<parent_id>"` (反向 grep ≥1)
- [ ] DOM `data-cv8-reply-target="<parent_id>"` (反向 grep ≥1)
- [ ] 文案 "▼ 隐藏 N 条回复" + "▶ 显示 N 条回复" byte-identical (反向 grep ≥1 各)
- [ ] 反向 grep `cv8.*depth.*[2-9]\|cv8.*recursive\|cv8.*nested.*reply` 0 hit

## §5 蓝图边界 ⑤⑥⑦ — fail-closed / forward-only / 不裂表

- [ ] cross-channel reject 跟 CV-5/CV-7 同源 (REG-INV-002 fail-closed)
- [ ] forward-only — reply edit 走 CV-7 既有路径; 不留 thread history
- [ ] 不裂表 — 0 schema 改

## §6 退出条件

- §1 (4) + §2 (4) + §3 (6) + §4 (6) + §5 (3) 全 ✅
- 反向 grep 6 锚: 4 处 0 hit + DOM/文案 ≥1 hit
- e2e 6 case 全 PASS
- 0 schema 改
- 5-pattern 第 6 处链 byte-identical
