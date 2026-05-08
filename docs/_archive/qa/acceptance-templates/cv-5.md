# Acceptance Template — CV-5: artifact comments (canvas vision) ✅

> 蓝图 `canvas-vision.md` L24 字面 "Linear issue + comment" + DM-2.2 #372 namespace 承袭 + RT-3 #488 cursor 共序 + thinking 5-pattern 第 4 处链 (RT-3/BPP-2.2/AL-1b/CV-5). Spec `cv-5-spec.md` (战马E v0 857170d) + Stance `cv-5-stance-checklist.md` (战马E v0). 不需 content-lock — comment 是用户文本, 视觉栈承袭 CV-2.3 anchor comments. 拆 PR: 整 milestone 一 PR (`spec/cv-5`). Owner: 战马E 实施 / 飞马 review / 烈马 验收.

## 验收清单 (✅ 三段全绿)

### §1 CV-5.1 — server POST /api/v1/artifacts/:id/comments + thinking validator

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 POST /api/v1/artifacts/:id/comments body `{body, agent_id?}` → 创建 message row, channel_id=`artifact:<id>`, 复用 Store.CreateMessage; ACL 复用 channel-member (artifact channel auto-include channel.created_by + agent_id) | unit (handleCreateComment happy) | 战马E / 烈马 | `internal/api/artifact_comments_test.go::TestArtifactComments_HumanCreate_OK` |
| 1.2 立场 ③ agent thinking subject 必带 — sender_role==agent + body 字面 5-pattern 任一 → reject 400 `comment.thinking_subject_required` | unit (5 sub-case) | 战马E / 飞马 / 烈马 | `TestArtifactComments_AgentThinkingSubject_Reject` (5 sub-case: thinking$ / defaultSubject / fallbackSubject / "AI is thinking" / subject="" 全 reject) |
| 1.3 立场 ④ cross-channel reject — 非 artifact channel 成员 403 `comment.cross_channel_reject` (REG-INV-002 fail-closed) | unit | 战马E / 烈马 | `TestArtifactComments_CrossChannelReject_403` |
| 1.4 立场 ① 不开新表 — git diff packages/server-go/internal/migrations/ 0 改 + 反向 grep `CREATE TABLE.*artifact_comments\|artifact_comments.*PRIMARY` 0 hit | grep + git diff | 飞马 / 烈马 | `TestArtifactComments_NoSchemaChange` (反向 grep production migration) |
| 1.5 立场 ② frame `artifact_comment_added` 推 hub.cursors.NextCursor 共序 + body_preview 80 rune cap | unit (cursor monotonic + cap) | 战马E / 烈马 | `TestArtifactComments_FrameCursorMonotonic` + `TestArtifactComments_BodyPreviewCap80Rune` |

### §2 CV-5.2 — client ArtifactComments.tsx + WS frame switch case

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 ArtifactComments.tsx 渲染 comment list (sender 头像 + body + timestamp + agent/human 区分 badge), hover anchor `data-cv5-author-link` (跟 CM-5.3 透明协作 hover 同源) | vitest (3 case) | 战马E / 烈马 | `packages/client/src/__tests__/ArtifactComments.test.tsx` (空状态 / 渲染 list / hover badge data-attr) |
| 2.2 wsClient.ts switch case `artifact_comment_added` → 增量 append (不刷全列表) | vitest | 战马E / 烈马 | `TestWSClient_ArtifactCommentAddedIncremental` |
| 2.3 lib/api.ts postArtifactComment(artifactId, body, agentId?) → POST endpoint | vitest | 战马E / 烈马 | `TestLibApi_PostArtifactComment` |

### §3 CV-5.3 — e2e 5 case + REG-CV5-001..006 + AST 兜底

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e: 创 artifact + human POST comment → WS frame 真到 + UI 渲染 (round-trip) | E2E (Playwright) | 战马E / 烈马 / 野马 | `packages/e2e/tests/cv-5-artifact-comment.spec.ts::human round-trip` |
| 3.2 e2e: agent POST comment 带 thinking 字面无 subject → 400 reject (5-pattern 第 4 处链) | E2E | 战马E / 烈马 | `cv-5-artifact-comment.spec.ts::agent thinking subject reject` |
| 3.3 e2e: 非 artifact channel member 调 POST → 403 `comment.cross_channel_reject` | E2E | 战马E / 烈马 | `cv-5-artifact-comment.spec.ts::cross-channel reject` |
| 3.4 e2e: cursor 共序锁 — artifact_comment_added 跟 RT-3 frame 严格递增 (跟 RT-3 / BPP-2 / DM-2.2 共一根) | E2E (cursor monotonic across frame types) | 战马E / 烈马 / 野马 | `cv-5-artifact-comment.spec.ts::cursor monotonic with rt3 frames` |
| 3.5 e2e: admin god-mode 不消费 frame (ADM-0 §1.3 红线) | E2E | 战马E / 烈马 | `cv-5-artifact-comment.spec.ts::admin god-mode does not consume` |
| 3.6 反向 grep 4 锚 0 hit + 1 锚 ≥1 (artifact_comments 表 / cursor 抄送 / thinking 5-pattern / admin / `channel_id\s*=\s*"artifact:` ≥1) | CI grep | 飞马 / 烈马 | CI lint 每 CV-5 PR 必跑 + `TestArtifactComments_ReverseGrepAnchors` |

## 边界

- DM-2.2 #372 mention 单源 fan-out (namespace + cursor 同源) / RT-3 #488 hub.cursors (cursor 共序根) / CV-2.3 #404 anchor comments (UI 视觉栈承袭) / BPP-2.2 #485 + AL-1b #482 thinking 5-pattern 同链 / ADM-0 §1.3 admin rail 红线 / REG-INV-002 fail-closed

## 退出条件

- §1 (5) + §2 (3) + §3 (6) 全绿 — 一票否决
- 0 schema 改 (反向 grep `CREATE TABLE.*artifact_comments` 0 hit)
- 反向 grep 4 锚 0 hit + 1 锚 ≥1 hit (spec §3 字面)
- 5-pattern thinking 第 4 处链 byte-identical (RT-3 / BPP-2.2 / AL-1b / CV-5 errcode + literal 同字符)
- 登记 REG-CV5-001..006
