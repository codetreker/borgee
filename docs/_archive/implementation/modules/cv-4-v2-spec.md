# CV-4 v2 spec brief — canvas iteration history list + timeline UI 续 (≤80 行)

> 战马D · Phase 5+ · ≤80 行 · 蓝图 [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) §1.4 (artifact 多版本 + iterate 历史可见) + CV-4 v1 (#398/#414/#417) iteration request lifecycle 落. 模块锚 [`canvas-vision.md`](canvas-vision.md) §CV-4. 依赖 CV-4 v1 `artifact_iterations` 表 v=18 + CV-1 既有 `POST /artifacts/:id/commits` + RT-1 `ArtifactUpdated` frame + CV-3 v2 `preview_url` (#517) + ADM-0 §1.3 红线.

## 0. 关键约束 (3 条立场, 蓝图 §1.4 + CV-4 v1 字面承袭)

1. **iteration history 复用 CV-1.4 既有 path** — owner 看 artifact 历史走 `GET /api/v1/artifacts/{id}/versions` 既有 endpoint (CV-1.4 已有 artifact_versions list); CV-4 v2 仅加 `GET /api/v1/artifacts/{id}/iterations` **复用 events 表既有 cursor sequence** 走 ListArtifactIterations store helper, 不另起 history endpoint / 不另起 sequence. **反约束**: 反向 grep `iteration_history_event\|artifact_iteration_log\|iteration_history_table` 在 internal/ count==0; 不另开 `artifact_iteration_history` 表 (artifact_iterations 表 + state-log 反向 derive 已盖).

2. **thumbnail history 不存 (复用 CV-3 v2 thumbnail_url)** — artifact_versions 当前每行 immutable append (CV-1.4 立场), 含 `preview_url` (CV-3 v2 #517 加列). CV-4 v2 timeline 直接显 `preview_url` 字段值, **不另存历史快照** / **不另起 thumbnail_history 表**. 旧版本 preview 走 artifact_versions 既有行读取 (artifact 域内单源). **反约束**: 反向 grep `thumbnail_history\|preview_history\|version_thumbnail_snapshot` 0 hit (跟 CV-1.4 immutable append 同精神, 不裂表).

3. **iteration timeline UI 仅 owner-only 视图层** — 非 owner 走 GET /artifacts/{id}/iterations 返 403 `cv4.iterations_owner_only` (跟 AL-2a/BPP-3.2/AL-1/AL-5/DM-4 owner-only 6 处同模式); admin god-mode 不挂此路径 (ADM-0 §1.3 红线). client SPA `IterationTimeline.tsx` 复用 useArtifactUpdated hook (cursor 进展归 RT-1.1, 不写独立 cursor). **反约束**: 反向 grep `admin.*iterations\|admin.*CV4` 在 admin*.go count==0; client 反向 grep `borgee.cv4.cursor:*` sessionStorage 0 hit (cursor 复用 RT-1.1).

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 DM-3/DM-4/BPP-6 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-4.1 v2 server iteration list endpoint | `internal/api/cv_4_v2_iterations_list.go` 新 (GET /api/v1/artifacts/{id}/iterations owner-only ACL + DESC created_at + limit 100; 复用 ListArtifactIterations store helper, 0 schema 改) + `internal/api/cv_4_v2_iterations_list_test.go` 新 (5 unit: HappyPath / NonOwnerRejected 403 cv4.iterations_owner_only / Unauthorized 401 / ArtifactNotFound 404 / NoEventLogTableSpawn 反向 grep) | 复用 artifact_iterations 表既有, 0 schema 新增 |
| CV-4.2 v2 client iteration timeline | `packages/client/src/components/IterationTimeline.tsx` 新 (DESC 列表 + state badge 4 态 pending/running/completed/failed + intent_text + preview_url thumbnail 复用 CV-3 v2 字段 + click jump artifact version) + `__tests__/IterationTimeline.test.tsx` 4 vitest (4 态 badge 渲染 + thumbnail src 复用 + 空状态 + 点击 callback) | 不写独立 cursor, useArtifactUpdated 复用 RT-1.1 路径 |
| CV-4.3 e2e + REG-CV4V2 + acceptance + PROGRESS [x] + closure | `packages/e2e/tests/cv-4-v2-iteration-history.spec.ts` 新 (创 artifact + 触 3 iteration → GET endpoint 返 3 行 DESC + UI 渲染 thumbnail + state badge 真测) + REG-CV4V2-001..005 + acceptance/cv-4-v2.md + docs/current sync (server/canvas-iterations-list.md + client/components/iteration-timeline.md) | RT-1.1 cursor 复用兜底真测 |

## 2. 留账边界

- **iteration history 跨 user comment / 评论** (留 v3) — CV-2 锚点 vs iteration timeline 是不同维度; v3 加 iteration-level comment 跟 anchor 同模式
- **iteration retry / 重试** (留 v3) — failed → 用户手动重 POST /iterate 同 endpoint 即可, server 端不挂 retry queue (跟 BPP-4/5/6 best-effort 立场承袭)
- **iteration audit forward-only** (留 v3 真严格) — v2 仅 ListArtifactIterations DESC 读, 不挂 UPDATE/DELETE 路径; v3 加 server-side audit trail 严验证
- **thumbnail 历史快照** (永久不挂, §0.2 立场) — preview_url 是 artifact_versions 既有行字段, 旧版本 preview 走既有行读, 不另存 history snapshot

## 3. 反查 grep 锚 (Phase 5+ 验收 + CV-4 v2 实施 PR 必跑)

```
git grep -nE 'GET.*\/artifacts\/.*\/iterations' packages/server-go/internal/api/   # ≥ 1 hit (endpoint 真挂, CV-4.1 v2)
git grep -nE 'IterationTimeline' packages/client/src/components/                    # ≥ 1 hit (CV-4.2 v2 component)
# 反约束 (5 条 0 hit)
git grep -nE 'iteration_history_event|artifact_iteration_log|iteration_history_table' packages/server-go/internal/   # 0 hit (复用 events, §0.1)
git grep -nE 'thumbnail_history|preview_history|version_thumbnail_snapshot' packages/server-go/internal/   # 0 hit (复用 CV-3 v2 preview_url, §0.2)
git grep -nE 'admin.*iterations|admin.*CV4' packages/server-go/internal/api/admin*.go   # 0 hit (ADM-0 §1.3 红线)
git grep -nE 'borgee\.cv4\.cursor:|useCV4Cursor' packages/client/src/   # 0 hit (cursor 复用 RT-1.1, §0.3)
git grep -nE 'ALTER TABLE artifact_iterations|CREATE TABLE.*iteration_history' packages/server-go/internal/migrations/   # 0 hit (0 schema 改, v2 范围)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ iteration retry server queue (留 v3, 跟 BPP-4/5/6 best-effort 同精神)
- ❌ iteration-level comment / 评论 (CV-2 锚点维度不同, 留 v3)
- ❌ thumbnail 历史快照表 (§0.2 立场, 永久不挂)
- ❌ schema 改 (CV-4 v1 表已盖, v2 仅加 list endpoint)
- ❌ admin god-mode 看 iteration history (ADM-0 §1.3 红线)
- ❌ 跨 owner / cross-org iteration history (留 AP-3 cross-org 维度)
- ❌ 独立 cursor 字典 (cursor 复用 RT-1.1, useArtifactUpdated 已订阅)
