# AL-8 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 AL-7/BPP-8/HB-3 v2 stance 同模式)
> **目的**: AL-8 三段实施 (8.1 0 schema / 8.2 server filter / 8.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/al-8-spec.md` (战马D v0) + acceptance `docs/qa/acceptance-templates/al-8.md` (战马D v0)
> **不需 content-lock** — admin-rail API 无 client UI v1 (admin dashboard 留 v3); 跟 AL-7 / BPP-3..8 / HB-3 v2 server-only 同模式.
> **0 schema / 0 新 endpoint** — 仅复用 ADM-2.2 既有 GET /admin-api/v1/audit-log + AL-7.1 archived_at 列.

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | 不另起 endpoint — GET /admin-api/v1/audit-log 路径单源 (ADM-2.2 既有), 加 since/until/archived/actions additive filter, 既有 3-filter (actor_id/action/target_user_id) byte-identical 不动 | admin-model.md §1.4 audit log 单视图原则 + ADM-2.2 #484 既有 path | 反向 grep `audit-log/query\|audit-log/search\|/admin-api/v1/audit/` 在 internal/api/ 除 ADM-2.2 既有 0 hit |
| ② | admin-rail only — ADM-0 §1.3 红线 admin 互可见 + user-rail 不挂 | admin-model.md ADM-0 §1.3 + ADM-2.2 stance ⑥ admin/user 二轨拆死 | 反向 grep `/api/v1/.*audit-log` user-rail handler 0 hit; 反向 grep `RegisterUserRoutes.*audit-log` 0 hit |
| ③ | archived filter 三态 (active/archived/all) — active 默认 (archived_at IS NULL, 跟 AL-7 sparse idx 反向同源 — active 行不入 idx 现网零开销); archived (archived_at IS NOT NULL — 走 idx_admin_actions_archived_at sparse); all (无 WHERE) | al-7-spec.md §0 立场 ① archived_at 列单源 + ADM-2.2 audit 互可见 | spec 外值 (?archived=foo) 反向 reject 400 `audit_log.archived_view_invalid`; 跟 AL-7.1 archived_at 字段 byte-identical |
| ④ (边界) | since/until 区间 — int64 ms epoch; clamp 反 negative / non-int → 400 `audit_log.time_range_invalid`; since>until → 400 `audit_log.time_range_inverted` | acceptance §1.4 字面单源 | 反 0/负/字符串/反向 4 case 全 reject; 错码字面 byte-identical 跟 acceptance 同源 |
| ⑤ (边界) | actions 多值 query — r.URL.Query()["action"] Slice (重复 ?action=a&action=b); 单值 ADM-2.2 backward-compat byte-identical 不动 | ADM-2.2 既有 ?action=foo 单值 path | 单值 ADM-2.2 既有 unit 不破 (TestADM22_GetAdminAuditLog_FullVisibility 通过); 多值 INSERT IN slice 0 hit hardcode |
| ⑥ (边界) | limit clamp 跟 ADM-2.2 既有 default 100/max 500 字面单源 (parseLimit helper byte-identical 不动) | ADM-2.2 stance ⑦ limit clamp | parseLimit helper 0 改 (反向 grep `parseLimit.*100.*500` 复用既有 site) |
| ⑦ (边界) | AST 锁链延伸第 8 处 forbidden 3 token (`pendingAuditQuery / auditQueryRetryQueue / deadLetterAuditQuery`) 在 internal/auth + internal/api production 0 hit (跟 BPP-4/5/6/7/8 + HB-3 v2 + AL-7 同模式) | bpp-4.md §0.3 best-effort 立场 + AL-7 锁链延伸第 7 处承袭 | AST scan forbidden tokens count==0 |

## §1 立场 ① 不另起 endpoint (AL-8.2 守)

**反约束清单**:

- [ ] 既有 GET /admin-api/v1/audit-log 路径字面 byte-identical 不动 (ADM-2.2 #484 mux.Handle 行 0 改)
- [ ] 反向 grep `audit-log/query\|audit-log/search\|/admin-api/v1/audit/` 在 internal/api/ 0 hit (除 ADM-2.2 既有单源)
- [ ] 既有 3-filter (actor_id/action/target_user_id) ADM-2.2 单测全 PASS — TestADM22_GetAdminAuditLog_FullVisibility 不破
- [ ] AdminActionListFilters struct ADM-2.2 既有 3 字段顺序 byte-identical 不动 (ActorID/Action/TargetUserID 在前; Since/Until/ArchivedView/Actions 顺位 append)

## §2 立场 ② admin-rail only (AL-8.2 守)

**反约束清单**:

- [ ] 反向 grep `/api/v1/.*audit-log` user-rail handler 0 hit
- [ ] ADM2Handler.RegisterUserRoutes 不挂 audit-log 路径 (反向 grep `RegisterUserRoutes.*audit-log` 0 hit)
- [ ] user cookie 调 GET /admin-api/v1/audit-log?archived=archived → 401 (admin.RequireAdmin middleware 兜底, REG-ADM0-002 共享底线 byte-identical)
- [ ] user-rail GET /api/v1/me/admin-actions 不挂 archived/since/until filter (user 只见自己, 立场 ⑤ ADM-2.2 字面承袭, 反向 grep 0 hit)

## §3 立场 ③ archived 三态 (AL-8.2 守)

**反约束清单**:

- [ ] active 视图 (archived_at IS NULL) — 默认行为 (无 ?archived 参数 = active, 跟 AL-7 sparse idx 反向同源 — active 行不入 idx)
- [ ] archived 视图 (archived_at IS NOT NULL) — 走 idx_admin_actions_archived_at sparse idx (现网零开销, AL-7 字面承袭)
- [ ] all 视图 (无 WHERE) — 显式 ?archived=all 才走
- [ ] spec 外值 reject — ?archived=foo / ?archived=xxx → 400 错码 `audit_log.archived_view_invalid` 字面 byte-identical
- [ ] 跟 AL-7.1 admin_actions.archived_at 列 byte-identical (改 = 改 al_7_1 migration + AL-8 query handler 双源)

## §4 蓝图边界 ④⑤⑥⑦ — 不漂

**反约束清单**:

- [ ] since/until int64 ms epoch — negative / non-int → 400 `audit_log.time_range_invalid`
- [ ] since > until → 400 `audit_log.time_range_inverted` (反 0/负/字符串/反向区间 4 case 全 reject)
- [ ] actions 多值 r.URL.Query()["action"] Slice — 重复 ?action=a&action=b 走 IN 子句
- [ ] 单值 ?action=foo 字面 byte-identical 跟 ADM-2.2 既有 unit (TestADM22_GetAdminAuditLog_FullVisibility 不破)
- [ ] limit parseLimit helper 0 改 (default 100 / max 500 单源)
- [ ] AST 锁链延伸第 8 处 — forbidden 3 token (`pendingAuditQuery / auditQueryRetryQueue / deadLetterAuditQuery`) 在 internal/auth + internal/api production *.go 0 hit

## §5 退出条件

- §1 (4) + §2 (4) + §3 (5) + §4 (6) 全 ✅
- 反向 grep 5 项全 0 hit (新 endpoint / user-rail / 裂表 / cron / hardcode)
- 0 schema / 0 新 endpoint / 0 新 path (registry.go + mux.Handle 字面 byte-identical 不动)
- audit 5 字段链第 8 处不漂 (id/actor/target/action/metadata 字段集不动)
- AL-1a reason 锁链 AL-8 = 第 16 处 (AL-7 = 第 15 处承袭) — 复用 reasons.Unknown 字面 byte-identical 跟 AL-7 SweeperReason 同源
- AST 锁链延伸第 8 处
- admin-rail only (ADM-0 §1.3 红线)
- 登记 REG-AL8-001..006
