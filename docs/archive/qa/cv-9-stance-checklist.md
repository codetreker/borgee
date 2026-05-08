# CV-9 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · 立场 review checklist (跟 CV-5/CV-7/CV-8 续 + DM-2.2 mention router 复用 + thinking 5-pattern 第 7 处链)
> **关联**: spec `docs/implementation/modules/cv-9-spec.md` (1e57005) + acceptance `docs/qa/acceptance-templates/cv-9.md` + content-lock `docs/qa/cv-9-content-lock.md`.

## §0 立场总表 (4 立场 + 3 边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | mention fan-out 复用 DM-2.2 既有 path, **0 server 实施改动** — POST /channels/:id/messages 已挂 MentionDispatcher (messages.go:249); content_type=='artifact_comment' 走同 path (CV-7 whitelist 已盖) | DM-2.2 #372 + CV-5/7/8 单源 | 反向 grep `cv9.*fanout\|cv9.*dispatch\|comment_mentions.*PRIMARY\|/comments/.*/mention` count==0; CV-9 server diff 0 production 行 |
| ② | owner-only ACL byte-identical 12+ 处一致, admin god-mode 不挂 | ADM-0 §1.3 + CV-5/7/8 + DM-2.2 同源 | 反向 grep `admin.*mention.*comment\|admin.*comment.*mention` 在 admin*.go count==0 |
| ③ | agent comment mention 仍走 thinking 5-pattern 第 7 处链 byte-identical | RT-3 + BPP-2.2 + AL-1b + CV-5/7/8/9 同链 | 反向 grep 5-pattern 字面在 internal/api/ 排除 _test.go count==0; 改 = 改 7 处 |
| ④ | client unread badge UI — DOM `data-cv9-unread-count` + `data-cv9-mention-toast` + 文案 "你被 @ 在 N 条评论中" byte-identical; 复用 useMentionPushed hook 既有, 不另起 state | content-lock 必锁 + DM-2.2 客户端复用 | 反向 grep `data-cv9-*` ≥2 hit + `你被 @ 在 N 条评论中` ≥1 hit; `useCV9MentionState\|cv9-mention-state` count==0 |

## §1 立场 ① 0 server production code

- [ ] CV-9 不开新 endpoint, 不开新表 (git diff packages/server-go/ 0 production 行)
- [ ] artifact_comment-typed message 经 POST /channels/:id/messages 时 MentionDispatcher 真跑 (反向断 unit)
- [ ] 反向 grep `cv9.*fanout\|cv9.*dispatch\|comment_mentions.*PRIMARY\|/comments/.*/mention` 0 hit

## §2 立场 ② owner-only + admin 不挂

- [ ] mention.target_not_in_channel 既有 reject 路径覆盖 artifact_comment-typed mention
- [ ] 反向 grep `admin.*mention.*comment\|admin.*comment.*mention` 在 admin*.go 0 hit
- [ ] cross-org 既有 store.CrossOrg 不动

## §3 立场 ③ thinking 5-pattern 第 7 处链 byte-identical

- [ ] CV-8 既有 hook (messages.go::handleCreateMessage 当 reply_to_id != nil + agent + content_type='artifact_comment' 命中 5-pattern → reject) 覆盖 mention-in-body 路径 (mention 不豁免 thinking guard)
- [ ] 反向 grep 5-pattern 字面在 internal/api/ 排除 _test.go count==0
- [ ] 5-pattern 改 = 改 7 处 byte-identical

## §4 立场 ④ DOM 锚 + 文案 byte-identical

- [ ] `data-cv9-unread-count` 在 ArtifactCommentsMentionBadge.tsx 渲染 (反向 grep ≥1)
- [ ] `data-cv9-mention-toast` (反向 grep ≥1)
- [ ] 文案 "你被 @ 在 N 条评论中" byte-identical (N 占位符, count==0 时不渲染)
- [ ] 复用 useMentionPushed hook 既有 (反向断 import 真路径), 不另起 state — `useCV9MentionState\|cv9-mention-state` 0 hit

## §5 边界 ⑤⑥⑦ — fail-closed / forward-only / 不裂表

- [ ] cross-channel reject 跟 DM-2.2 同源 (REG-INV-002 fail-closed)
- [ ] forward-only — mention dispatch 是 best-effort fanout (失败仅 log 不阻断)
- [ ] 不裂表 — 0 schema 改

## §6 退出条件

- §1 (3) + §2 (3) + §3 (3) + §4 (4) + §5 (3) 全 ✅
- 反向 grep 6 锚: 4 处 0 hit + DOM/文案 ≥1 hit
- e2e 5 case 全 PASS
- 0 schema 改 + 0 server production code (0 行 git diff packages/server-go/internal/{api,store,migrations}/*.go production 文件)
- 5-pattern 第 7 处链 byte-identical
