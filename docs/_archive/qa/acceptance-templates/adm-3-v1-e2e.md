# Acceptance Template — ADM-3-V1-E2E (ADM-3 v1 multi-source #619 follow-up admin Playwright e2e)

> Spec brief `adm-3-v1-e2e-spec.md` (飞马 v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收.
>
> **ADM-3-V1-E2E 范围**: ADM-3 v1 multi-source UNION ALL #619 已 land server unit 11 + vitest 7, 但缺 admin login → /admin/audit-multi-source Playwright e2e (G4.audit closure P0.1 漏件). 立场承袭 ADM-3 v1 + AL-8 reverse-grep 白名单单一例外 + ADM-0 §1.3 admin god-mode 红线. **0 production code 改 (仅 e2e)**.

## 验收清单

### §1 行为不变量 (ADM-3 v1 #619 立场承袭)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 4 source UNION ALL byte-identical (server / plugin / host_bridge / agent) | E2E | Playwright `_AdminLogin_AllSourcesQuery` PASS, response 4 源 |
| 1.2 admin god-mode 路径独立 — admin login → /admin-api/v1/audit/multi-source 通; user 走 → 403 reject (反 user-rail 漂) | E2E | `_AdminLogin_FullQuery` + `_UserLogin_403Rejected` PASS |
| 1.3 source filter / time range / limit clamp 真测 | E2E | `_SourceFilterQuery` + `_TimeRangeFilter` + `_LimitClamp` PASS |

### §2 E2E (admin Playwright 真测 + 反 cross-user leak)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 Playwright `adm-3-multi-source.spec.ts` 5 case PASS (admin login + user reject + source filter + time range + limit clamp) | E2E | `packages/e2e/tests/adm-3-multi-source.spec.ts` 5 case PASS |
| 2.2 反 user-rail audit feed 真测 — user 真 cookie 无法访问 /admin-api/v1/audit/multi-source (永不挂, ADM-0 §1.3 红线立场承袭) | E2E | `_NoUserRailAuditFeed` PASS, 403 byte-identical |

### §3 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有全包 unit + e2e + vitest 全绿不破 + post-#621 haystack gate 三轨过 | full test + CI | go-test-cov SUCCESS |
| 3.2 0 production code 改 (仅 e2e 加) | git diff | `git diff main -- packages/server-go/internal/api/` 0 行 |
| 3.3 立场承袭 4 source enum SSOT byte-identical (跨层锁 server const + client AUDIT_SOURCES + i18n SOURCE_LABEL) | grep | reverse grep test PASS |

## REG-ADM3E2E-* 真翻 🟢

- REG-ADM3E2E-001 🟢 admin /admin/audit-multi-source UI 真渲染 + 4 source REST filter (sources byte-identical + 反 cross-source leak + invalid → 400 audit.source_invalid)
- REG-ADM3E2E-002 🟢 admin god-mode 路径独立 (user-rail 404 + user cookie 调 admin-api 401, ADM-0 §1.3 + ADM-0.2 红线)
- REG-ADM3E2E-003 🟢 time range filter (since/until happy + since-only + invalid since → 400 audit.time_range_invalid)
- REG-ADM3E2E-004 🟢 limit clamp (999 → 500 / 0 → default / 非整数 → default, 反 silent reject 漂)
- REG-ADM3E2E-005 🟢 ⭐ 反 user-rail audit feed Go reverse-grep (ADM-0 §1.3 红线核心断言, 反 v2+ 借口推)
- REG-ADM3E2E-006 🟢 0 production code 改 + Playwright 5 case PASS 3.7s + 4 source enum 跨层锁不破

## 退出条件

- §1 (3) + §2 (2) + §3 (3) 全绿 — 一票否决
- 5 case Playwright 真测 PASS
- 反 user-rail audit feed (ADM-0 §1.3 红线立场承袭)
- 0 production code 改 + post-#621 haystack gate 三轨过
- 登记 REG-ADM3E2E-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template. 立场承袭 ADM-3 v1 #619 + G4.audit closure P0.1 漏件 + 跨四 milestone audit 反转锁链 + ADM-0 §1.3 红线 + AL-8 reverse-grep 白名单单一例外承袭. |
| 2026-05-01 | 战马D | v1 实施 — 真补 post-#623 liema CONDITIONAL LGTM 三抓: §1.3 time range + limit clamp 2 case (3→5 case) + §2.2 反 user-rail audit feed Go reverse-grep (ADM-0 §1.3 红线核心断言, 反 v2+ 借口推; 跟 RT-3 #616 + AP-2 #620 reverse-grep 同模式承袭) + REG-ADM3E2E-001..006 ⚪→🟢 全翻. Playwright 5 case PASS 3.7s + Go reverse-grep test PASS. 0 production code 改 (post-#619 byte-identical). |
