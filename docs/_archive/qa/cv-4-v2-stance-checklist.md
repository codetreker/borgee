# CV-4 v2 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 CV-4 v1 stance + DM-3/DM-4 同模式)
> **目的**: CV-4 v2 三段实施 (CV-4.1 v2 server limit query / 4.2 client IterationTimeline / 4.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/cv-4-v2-spec.md` (战马D v0 c4e2c25) + acceptance `docs/qa/acceptance-templates/cv-4-v2.md` (战马D v0)
> **不需 content-lock** — server 仅加 limit query 参数 + client 时间轴 UI 无固定文案锁 (state badge label 跟 CV-4 v1 #380 content-lock 已锁), 跟 DM-3/DM-4/BPP-3/4/5/6 同模式.

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | iteration history 复用既有 path — `GET /api/v1/artifacts/{id}/iterations` (CV-4 v1 #414 已挂, channel-member ACL); v2 仅加 `?limit=N` query (默认 50, max 200), **不另起 history endpoint** / **不另起 sequence** | canvas-vision.md §1.4 + CV-4 v1 path 复用 | 反向 grep `iteration_history_event\|artifact_iteration_log\|iteration_history_table\|new.*iteration.*endpoint` 在 internal/ count==0; 不另开 `artifact_iteration_history` 表 |
| ② | thumbnail history 不存 (复用 artifact_versions.preview_url) — CV-3 v2 #517 已加 preview_url 列, CV-4 v2 timeline UI 直接读 artifact_versions 行字段; **不另存历史快照** / **不另起 thumbnail_history 表**; client 通过 created_artifact_version_id 反查 artifact_versions 行取 preview_url | canvas-vision.md §1.4 immutable append + CV-1.4 立场承袭 | 反向 grep `thumbnail_history\|preview_history\|version_thumbnail_snapshot` 在 internal/ count==0; client 反向 grep `cv4.*thumbnail.*cache\|iteration.*preview.*snapshot` 0 hit |
| ③ | iteration timeline UI cursor 复用 RT-1.1 — client `IterationTimeline.tsx` 复用 useArtifactUpdated hook (RT-1.1 cursor 进展归 hub.cursors); **不写独立 sessionStorage cursor**; admin god-mode 不挂 GET endpoint (ADM-0 §1.3 红线 — admin /admin-api/* rail 隔离, 跟 CV-4 v1 立场 ⑥ 同精神) | RT-1 #290 + CV-4 v1 stance ⑥ admin rail 隔离 | client 反向 grep `borgee\.cv4\.cursor:\|useCV4Cursor\|cv4.*sessionStorage` 0 hit; server 反向 grep `admin.*\/iterations\|admin.*CV4` 在 admin*.go count==0 |

## §1 立场 ① iteration history 复用既有 path (CV-4.1 v2 守)

**蓝图字面源**: `canvas-vision.md` §1.4 + CV-4 v1 #414 GET endpoint 已落 channel-member ACL

**反约束清单**:

- [ ] v2 endpoint signature 不变 — `GET /api/v1/artifacts/{artifactId}/iterations` (CV-4 v1 #414 既有), 仅加 `?limit=N` 可选 query (默认 50, max 200, 0/负 → 默认)
- [ ] 不另开 history endpoint — 反向 grep 在 internal/api/ 不含新 path 字面 `\/iteration_history\|\/iteration_log`
- [ ] 不另起 sequence — events 表既有 cursor 复用 (跟 RT-1.3 同精神)
- [ ] limit clamp 真测 (0 / 负 / 999 / 200 / 50 default) 4 case 全 PASS
- [ ] 反向 grep `iteration_history_event\|artifact_iteration_log\|iteration_history_table` 0 hit

## §2 立场 ② thumbnail history 不存 — 复用 artifact_versions.preview_url (CV-4.2 守)

**蓝图字面源**: `canvas-vision.md` §1.4 immutable append + CV-3 v2 #517 preview_url 加列

**反约束清单**:

- [ ] client `IterationTimeline.tsx` 通过 `iteration.created_artifact_version_id` 反查 artifact_versions GET endpoint, 直接显 `version.preview_url` (不缓存历史 thumbnail)
- [ ] 不另起 thumbnail_history 表 — 反向 grep `thumbnail_history\|preview_history\|version_thumbnail_snapshot` 0 hit
- [ ] 旧版本 preview 走既有 artifact_versions 行 (不裂表)
- [ ] client 反向 grep `cv4.*thumbnail.*cache` 0 hit

## §3 立场 ③ cursor 复用 RT-1.1 + admin god-mode 不挂 (CV-4.1+4.2 守)

**蓝图字面源**: RT-1 #290 cursor 单源 + CV-4 v1 stance ⑥ admin rail 隔离 + ADM-0 §1.3 红线

**反约束清单**:

- [ ] client `IterationTimeline.tsx` 不写独立 sessionStorage cursor — 反向 grep `borgee\.cv4\.cursor:\|useCV4Cursor` 0 hit (跟 DM-4 useDMEdit 同精神 — cursor 子集复用上层 hook)
- [ ] 复用 useArtifactUpdated (RT-1.1 既有 hook) — 反向断 IterationTimeline.tsx import useArtifactUpdated 真路径
- [ ] admin god-mode 不挂 GET endpoint — 反向 grep `admin.*\/iterations\|admin.*CV4` 在 internal/api/admin*.go count==0 (跟 CV-4 v1 stance ⑥ + ADM-0 §1.3 同精神)

## §4 蓝图边界 ④⑤⑥⑦ — 跟 CV-4 v1 ACL / 不裂 sequence / 不裂表 / forward-only 不漂

**反约束清单**:

- [ ] CV-4 v1 ACL 不变 — channel-member 视图 (跟 v1 #414 byte-identical, v2 不收紧不放宽)
- [ ] 不裂 sequence — cursor 跟 RT-1 / CV-* / DM-* / BPP-* 共一根 (反向 grep 立场 ①)
- [ ] 不裂表 — 0 schema 改 (反向 grep `ALTER TABLE artifact_iterations\|CREATE TABLE.*iteration_history` 在 internal/migrations/ 0 hit)
- [ ] forward-only — 不挂 UPDATE / DELETE 路径 (CV-4 v1 已有写入路径不动, v2 仅加 read-side limit)

## §5 退出条件

- §1 (5) + §2 (4) + §3 (3) + §4 (4) 全 ✅
- 反向 grep 5 项全 0 hit (history endpoint / sequence / thumbnail snapshot / admin / schema)
- e2e: 创 artifact + 触 ≥3 iteration → GET ?limit=2 真返 2 行 + UI 渲染 thumbnail badge
- 0 schema 改 (git diff packages/server-go/internal/migrations/ 仅 _test.go 或为空)
- client cursor 复用 RT-1.1 (跟 DM-4 useDMEdit 同精神不写独立 cursor)
