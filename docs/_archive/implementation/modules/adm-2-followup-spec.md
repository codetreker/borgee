# ADM-2 follow-up spec brief — REG-010 grant wire + REG-011 audit-log 页 (≤80 行)

> 飞马 · 2026-05-01 · post-Phase 4+ closure ⏸️ deferred 兑现 (ADM-2 #484 留账之二)
> **关联**: ADM-2 #484 ✅ admin_actions schema v=22 + impersonation_grants v=23 + 5 REST endpoints · ADM-3 v1 #619 multi-source audit query · ADM-0 §1.3 admin god-mode 红线
> **命名**: ADM-2-FOLLOWUP = ADM-2 deferred 兑现 (跟 G3.audit fill v1 / TEST-FIX-3-COV deferred follow-up 同模式承袭)

> ⚠️ Server wire-up + client admin SPA milestone — **0 schema 改 / 0 endpoint URL 改 / ~30 行 wire + ~150 行 client SPA**.
> ADM-2 #484 deferred 2 项 (REG-ADM2-010 grant 校验 wire + REG-ADM2-011 admin SPA audit-log 页 + e2e + G4.2 双截屏).

## 0. 关键约束 (3 条立场)

1. **ADM-2 #484 既有 schema + endpoint byte-identical 不破** (post-merge follow-up): admin_actions 表 / impersonation_grants 表 / 5 REST endpoint 字面不动, 仅 (a) `start_impersonation` handler 加 audit hook (REG-010 wire) (b) admin SPA `audit-log` 页新 + e2e (REG-011 兑现) (c) G4.2 双截屏 (5-state UI / busy-idle BPP). 反约束: 反向 grep `git diff -- internal/migrations/adm_2_*` 0 hit.

2. **REG-010 grant 校验 wire + REG-011 admin SPA audit-log 页**:
   - **REG-010**: `internal/api/adm_2_2_endpoints.go::startImpersonation` 加 `InsertAdminAction("impersonate.start", ...)` audit hook (跟 4/5 既有 admin handler audit 同精神, 反向断言 5/5 全挂)
   - **REG-011**: `packages/client/src/admin/pages/AuditLogPage.tsx` 新 (~150 行) + admin nav 加 `/admin/audit-log` route + 复用 ADM-3 v1 multi-source query 模式 (admin-rail 独立路径) + DOM data-attr SSOT (`data-page="admin-audit-log"` + `data-audit-row` + `data-audit-actor-kind`) + 4 vitest + 1 Playwright e2e
   - **G4.2 双截屏**: `docs/qa/screenshots/adm-2-{audit-log-page,impersonate-banner}.png` (yema 签字门槛)
   反约束: 反向 grep `audit-log.*user-rail|/api/v1/audit-log` 0 hit (admin god-mode 路径独立 ADM-0 §1.3 红线).

3. **0 schema / 0 endpoint URL 改 + admin god-mode 独立路径** (跟 ADM-2 #484 + ADM-3 v1 #619 立场承袭): PR diff 仅 (a) `start_impersonation` ~5 行 audit hook (b) AuditLogPage.tsx + admin nav route (c) 1 vitest + 1 e2e + 2 截屏. 反约束: 0 schema column / 0 migration v 号 / 0 user-rail endpoint 加.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **A2F.1 REG-010 grant wire** | `internal/api/adm_2_2_endpoints.go::startImpersonation` 加 InsertAdminAction("impersonate.start", ...) ~5 行 (跟既有 4/5 admin handler 同精神 byte-identical) + 1 unit test 真测 5/5 admin handler 全挂 audit hook | 战马 / 飞马 review |
| **A2F.2 REG-011 admin SPA audit-log + G4.2 截屏** | `packages/client/src/admin/pages/AuditLogPage.tsx` ~150 行 (复用 ADM-3 multi-source query API + 4 source badge byte-identical 跟 #619 + filter + DOM data-attr SSOT) + admin nav route + 4 vitest (render / filter / impersonate banner / data-attr) + Playwright e2e (admin login → /admin/audit-log → 真渲染) + 2 截屏 (audit-log-page 全状态 + impersonate-banner 红色横幅) | 战马 / 飞马 review |
| **A2F.3 closure** | REG-ADM2-010 ⏸️→🟢 + REG-ADM2-011 ⏸️→🟢 + 6 反向 grep + 0 schema 改 + admin 路径独立守 + post-#619 haystack 三轨过 + acceptance template ⏸️ 翻 🟢 + yema G4.2 双截屏签字 + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) start_impersonation audit hook 真挂 (REG-010)
grep -nE 'InsertAdminAction.*impersonate\.start|InsertAdminAction.*"impersonate' packages/server-go/internal/api/adm_2_2_endpoints.go  # ≥1 hit

# 2) admin SPA audit-log 页真建 (REG-011)
test -f packages/client/src/admin/pages/AuditLogPage.tsx  # exists
grep -nE 'data-page="admin-audit-log"' packages/client/src/admin/pages/AuditLogPage.tsx  # ≥1 hit

# 3) admin god-mode 独立 (反 user-rail 漂)
grep -rE '/api/v1/audit-log|"user-audit-log"' packages/server-go/internal/api/ packages/client/src/  # 0 hit

# 4) 0 schema 改
git diff origin/main -- packages/server-go/internal/migrations/adm_2_* | grep -cE '^\+|^-'  # 0 hit
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+\s*Version:'  # 0 hit

# 5) G4.2 双截屏
ls docs/qa/screenshots/adm-2-{audit-log-page,impersonate-banner}.png  # 2 文件

# 6) post-#619 haystack gate + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./... && pnpm vitest run && pnpm exec playwright test -g 'adm-2'  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **per-user audit feed (user-rail)** — 永久不挂 (ADM-0 §1.3 红线)
- ❌ **audit retention 跨 source 统一** — 留 v2+ (走 DL-2 既有 retention sweeper per-source)
- ❌ **audit FTS / external export (Splunk/Datadog)** — 留 v2+

## 4. 跨 milestone byte-identical 锁

- ADM-2 #484 5 REST endpoint + impersonation_grants schema byte-identical 不破
- ADM-3 v1 #619 multi-source query API + 4 source enum byte-identical 不破
- ADM-0 §1.3 admin god-mode 路径独立红线
- AP-2 #620 capability i18n SSOT 模式 (audit source label 同精神)
- RT-3 #616 e2e + screenshot 模式承袭
