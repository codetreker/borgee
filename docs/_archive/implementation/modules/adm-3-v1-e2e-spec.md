# ADM-3 v1 e2e spec brief — multi-source audit Playwright 3 case (≤80 行)

> 飞马 · 2026-05-01 · post-Phase 4+ closure follow-up (ADM-3 v1 #619 acceptance §3.3 e2e 漏件兑现)
> **关联**: ADM-3 v1 #619 ✅ multi-source audit query API + admin UI · ADM-2 #484 audit_events · DL-2 #615 channel_events/global_events · HB-1 #491 install-butler audit
> **命名**: ADM-3-V1-E2E = ADM-3 v1 e2e follow-up (acceptance §3.3 真兑现, 跟 RT-3 #616 e2e 5 case 同模式承袭)

> ⚠️ E2E + acceptance flip milestone — **0 production code 改 / 0 schema / 0 endpoint** (ADM-3 v1 实施已 merged, 本 PR 仅补 e2e 真测 + acceptance §3.3 ⚪→🟢).

## 0. 关键约束 (3 条立场)

1. **ADM-3 v1 #619 实施 byte-identical 不破** (post-merge follow-up): 0 production .go 改 + 0 client .tsx 改, 仅加 `packages/e2e/tests/adm-3-audit-events.spec.ts` 3 case + acceptance §3.3 ⚪→🟢 翻牌. 反约束: 反向 grep server-go diff `internal/api/admin_audit_query.go` + client diff `admin/api.ts` + `MultiSourceAuditPage.tsx` 0 hit (post-#619 不动).

2. **e2e 3 case (acceptance §3.3 字面 byte-identical)**:
   - **case-1 admin /admin/audit-multi-source 渲染**: admin login → 走 /admin/audit-multi-source → page 真渲染 + 4 source badge 真显 (server/plugin/host_bridge/agent) + table 真有 row
   - **case-2 4 source filter dropdown**: filter dropdown 选 plugin → 表只显 plugin source 行 + 反向断 server/host_bridge/agent 行 0 hit
   - **case-3 admin god-mode 路径独立**: user-rail (普通 user) 走 /api/v1/audit/multi-source 反向断言 404 / 403 (路径仅 /admin-api/v1/audit/multi-source 暴露, 跟 ADM-0 §1.3 红线 byte-identical) + page-level reverse-grep `data-page="user-audit-multi-source"` 0 hit
   反约束: e2e 文件名 byte-identical `packages/e2e/tests/adm-3-audit-events.spec.ts` 跟 acceptance §3.3 字面 byte-identical (改 = 改两处).

3. **0 production change + post-#619 haystack gate 三轨守 + 4 source enum 不破** (跟 ADM-3 v1 #619 立场承袭): PR diff 仅 (a) e2e 1 文件 (~150 行 3 case) (b) acceptance template §3.3 ⚪→🟢 翻牌 (c) regression-registry REG-ADM3-007/008 真翻 🟢 (post-PR merge 真测). 反约束: 0 schema / 0 endpoint URL / 0 routes.go / 0 4 source enum 改.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **A3VE.1 e2e 3 case** | `packages/e2e/tests/adm-3-audit-events.spec.ts` 新 (~150 行 3 case 走既有 admin login + page screenshot 模式跟 cv-4-unfixme-followup.spec.ts byte-identical 套用) | 战马 / 飞马 review |
| **A3VE.2 acceptance flip** | `docs/qa/acceptance-templates/adm-3.md` §3.3 ⚪→🟢 翻牌 (e2e 真过后 PR body 示 PASS 输出) + REG-ADM3-007 真测 audit Playwright PR 锚 | 战马 / 烈马 |
| **A3VE.3 closure** | REG-ADM3-007/008 ⚪→🟢 + 6 反向 grep + 0 production 改 + post-#619 haystack 三轨过 + 既有 test 全 PASS + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (6 反约束)

```bash
# 1) e2e 文件真建
test -f packages/e2e/tests/adm-3-audit-events.spec.ts  # exists
grep -cE 'test\(.*case' packages/e2e/tests/adm-3-audit-events.spec.ts  # ≥3 hit

# 2) 0 production 改 (post-#619 byte-identical)
git diff origin/main -- packages/server-go/internal/api/admin_audit_query.go packages/client/src/admin/ | grep -cE '^\+|^-'  # ≤2 hit (允许 import 微调)

# 3) admin god-mode 路径独立守 (反 user-rail 漂 ADM-0 §1.3 红线)
grep -rE '/api/v1/audit/multi-source' packages/server-go/internal/api/  # 0 hit (仅 /admin-api/v1/audit/multi-source)
grep -rE 'data-page="user-audit-multi-source"' packages/client/src/  # 0 hit

# 4) 4 source enum byte-identical 不破
grep -rcE 'AuditSourceServer|AuditSourcePlugin|AuditSourceHostBridge|AuditSourceAgent' packages/server-go/internal/api/admin_audit_query.go  # ==4 hit

# 5) acceptance §3.3 翻 🟢
grep -E '3\.3 .*🟢|3\.3 .*PASS' docs/qa/acceptance-templates/adm-3.md  # ≥1 hit

# 6) post-#619 haystack gate + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
pnpm exec playwright test --timeout=30000 -g 'adm-3'  # 3 case PASS
```

## 3. 不在范围 (留账)

- ❌ **跨 source 反向追溯链** (e.g. agent action → host_bridge syscall trace) — 留 v3+
- ❌ **audit FTS 搜索** — 留 v3+
- ❌ **host_bridge source placeholder 真接** — 留 ADM-3-host-bridge-wire follow-up (HB-2 v0(D) Helper 端真出 audit 流后接)
- ❌ **per-user audit feed (user-rail)** — 永久不挂 (ADM-0 §1.3 红线)

## 4. 跨 milestone byte-identical 锁

- ADM-3 v1 #619 production code byte-identical 不破
- RT-3 #616 e2e + screenshot 模式承袭 (browser.newContext + page.screenshot)
- ADM-2 #484 admin auth middleware 不破
- DL-2 #615 mustPersistKinds + channel_events / global_events schema 不破
- ADM-0 §1.3 admin god-mode 路径独立 (e2e case-3 真测)
