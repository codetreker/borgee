# Acceptance Template — CV-9: artifact comment notification (mention fan-out) ✅ closed

> 蓝图 `canvas-vision.md` L24 + DM-2.2 #372 mention router 复用 + thinking 5-pattern 第 7 处链 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 + CV-9). Spec `cv-9-spec.md` (1e57005) + Stance + Content-lock. **0 server production code + 0 schema 改 + 0 新 endpoint**. 拆 PR: 整 milestone 一 PR (`feat/cv-9`).

## 验收清单

### §1 CV-9.1 — server 0 production code + 1 unit 验证 dispatch 真触发

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 立场 ① 0 server production code — CV-9 git diff packages/server-go/ 0 production 行 (仅 _test.go) | git diff | 战马E / 飞马 / 烈马 | `git diff main..feat/cv-9 -- 'packages/server-go/internal/**/*.go' ':!**/*_test.go'` 0 行 |
| 1.2 unit 验证 mention dispatch 在 artifact_comment-typed message 路径真跑 (反向断 PushMentionPushed 真调用) | unit | 战马E / 烈马 | `internal/api/cv_9_mention_dispatch_test.go::TestCV9_ArtifactComment_TriggersMentionDispatch` PASS |
| 1.3 立场 ③ agent + 5-pattern + mention body → 仍 reject (mention 不豁免 thinking guard) | unit | 战马E / 烈马 | `TestCV9_AgentMentionThinking_StillReject` (agent 含 mention 但 5-pattern 命中 → 400) |

### §2 CV-9.2 — client ArtifactCommentsMentionBadge.tsx + DOM/文案 byte-identical

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 立场 ④ unread badge — `data-cv9-unread-count` DOM 锚 + 文案 "你被 @ 在 N 条评论中" byte-identical (N=0 时不渲染) | vitest (3 case) | 战马E / 烈马 | `ArtifactCommentsMentionBadge.test.tsx` (count==0 不渲染 / count==N 渲染 + 文案 byte-identical / DOM data-attr) |
| 2.2 立场 ④ 复用 useMentionPushed 既有 hook — 反向断 不另起 state (反向 grep `useCV9MentionState\|cv9-mention-state` 0 hit) | vitest + grep | 战马E / 烈马 | `ArtifactCommentsMentionBadge.test.tsx::ReusesUseMentionPushed` |
| 2.3 click → scroll-to-message + reset count | vitest | 战马E / 烈马 | `ArtifactCommentsMentionBadge.test.tsx::click_handler` |

### §3 CV-9.3 — e2e + REG-CV9-001..006

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e: human posts artifact_comment with `<@user_id>` → MentionDispatcher 真跑, mention row 真写 | E2E | 战马E / 烈马 | `cv-9-comment-mention.spec.ts::human mention dispatch` |
| 3.2 e2e: agent posts artifact_comment with mention + 5-pattern body → 400 byte-identical (5-pattern 第 7 处链) | E2E | 战马E / 飞马 / 烈马 | `cv-9-comment-mention.spec.ts::agent thinking with mention reject` |
| 3.3 e2e: mention non-channel-member → 400 `mention.target_not_in_channel` (DM-2.2 既有错码复用) | E2E | 战马E / 烈马 | `cv-9-comment-mention.spec.ts::mention cross-channel reject` |
| 3.4 e2e: artifact_comment-typed mention 跟 text-typed mention dispatch 等价 (反向断 mention row 都写) | E2E | 战马E / 烈马 | `cv-9-comment-mention.spec.ts::dispatch parity` |
| 3.5 反向 grep 6 锚: 4 处 0 hit + DOM/文案 ≥1 hit (cv-9-spec.md §3 字面) | CI grep | 飞马 / 烈马 | CI lint 每 CV-9 PR 必跑 |

## 边界

- DM-2.2 #372 (MentionDispatcher + MentionPushed frame) / CV-5 #530 + CV-7 #535 + CV-8 #537 (artifact_comment namespace 单源 + thinking 5-pattern 链) / messages.go (POST 路径既有) / ADM-0 §1.3 admin rail 红线 / REG-INV-002 fail-closed / 5-pattern 锁链 7 处 byte-identical

## 退出条件

- §1 (3) + §2 (3) + §3 (5) 全绿
- 0 schema 改 + 0 server production code (CV-9.1 验收 1.1)
- 反向 grep 6 锚通过
- 5-pattern 第 7 处链 byte-identical
- 登记 REG-CV9-001..006
