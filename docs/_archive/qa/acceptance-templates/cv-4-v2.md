# Acceptance Template — CV-4 v2: canvas iteration history list + timeline UI 续

> 蓝图 `canvas-vision.md` §1.4 + RT-1 #290 fan-out + CV-4 v1 #398/#414/#417. Spec `cv-4-v2-spec.md` (战马D v0 c4e2c25) + Stance `cv-4-v2-stance-checklist.md` (战马D v0). 不需 content-lock — server 仅加 limit query + client UI state badge label 跟 CV-4 v1 #380 已锁. 拆 PR: 整 milestone 一 PR (`spec/cv-4-v2`). Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 CV-4.1 v2 — server GET /iterations + ?limit query (CV-4 v1 endpoint 复用 + 加 limit)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 GET /api/v1/artifacts/{artifactId}/iterations 复用 CV-4 v1 #414 既有 endpoint + ACL channel-member 不变, v2 加 `?limit=N` query (默认 50, max 200, 0/负 → 50, >200 → 200) | unit (4 limit clamp sub-case) | 战马D / 烈马 | `internal/api/cv_4_2_iterations_test.go::TestCV4V2_ListIterations_LimitClamp` (limit=0/-1/999/100/empty → 50/50/200/100/50 5 sub-case) |
| 1.2 0 schema 改 — git diff packages/server-go/internal/migrations/ 仅 _test.go (反向 grep `ALTER TABLE artifact_iterations\|CREATE TABLE.*iteration_history` 0 hit) | grep + git diff | 战马D / 飞马 / 烈马 | `TestCV4V2_NoSchemaChange` (反向 grep production migration 文件 + 反向 grep `iteration_history_event\|artifact_iteration_log\|iteration_history_table` count==0) |
| 1.3 立场 ③ admin god-mode 不挂 — 反向 grep `admin.*\/iterations\|admin.*CV4` 在 internal/api/admin*.go count==0 (CV-4 v1 stance ⑥ + ADM-0 §1.3 同源) | grep | 飞马 / 烈马 | `TestCV4V2_AdminGodModeNotMounted` (反向 grep admin*.go 反向 0 hit) |

### §2 CV-4.2 — client IterationTimeline.tsx + 4 vitest

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 IterationTimeline.tsx 渲染 DESC list (4 态 badge pending/running/completed/failed + intent_text + thumbnail 复用 artifact_versions.preview_url 字段 + click jump callback) | vitest (4 case) | 战马D / 烈马 | `packages/client/src/__tests__/IterationTimeline.test.tsx` 4 vitest PASS (4 态 badge 渲染 + thumbnail src 复用 + 空状态 / 点击 onJump callback) |
| 2.2 立场 ③ cursor 复用 RT-1.1 — IterationTimeline 不写独立 sessionStorage cursor; 反向 grep `borgee.cv4.cursor:\|useCV4Cursor\|cv4.*sessionStorage` 0 hit (跟 DM-4 useDMEdit 同精神) | grep + vitest | 战马D / 烈马 | `TestIterationTimeline_DoesNotWriteOwnCursor` (反向断言 sessionStorage borgee.cv4.cursor:* 未写, 跟 DM-4 useDMEdit DoesNotWriteOwnCursor 同模式) |

### §3 CV-4.3 — e2e + REG-CV4V2 + AST 兜底

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e: 创 artifact + 触 3 iteration → GET ?limit=2 真返 2 行 DESC + UI 渲染 4 态 badge + thumbnail (复用 preview_url) | E2E (Playwright) | 战马D / 烈马 / 野马 | `packages/e2e/tests/cv-4-v2-iteration-history.spec.ts` REST-driven seed 3 iteration + UI 渲染 |
| 3.2 反向 grep 5 锚 0 hit (不另起 history endpoint / 不另起 sequence / 不另起 thumbnail snapshot / admin god-mode / 不裂 schema) | CI grep | 飞马 / 烈马 | CI lint 每 CV-4 v2 PR 必跑 + `TestCV4V2_NoHistoryEventTable` AST scan |

## 边界

- CV-4 v1 #398/#414/#417 (artifact_iterations 表 + GET endpoint 复用) / CV-1 #348 commits endpoint (路径单源不动) / RT-1 #290 ArtifactUpdated frame (cursor 复用) / CV-3 v2 #517 preview_url (thumbnail 字段复用) / AL-2a/BPP-3.2/AL-1/AL-5/DM-4 owner-only 6 处 / ADM-0 §1.3 红线

## 退出条件

- §1 (3) + §2 (2) + §3 (2) 全绿 — 一票否决
- 0 schema 改 (反向 grep `ALTER TABLE artifact_iterations\|CREATE TABLE.*iteration_history` 0 hit)
- 反向 grep 5 锚 0 hit (history endpoint / sequence / thumbnail snapshot / admin / schema)
- 登记 REG-CV4V2-001..005
