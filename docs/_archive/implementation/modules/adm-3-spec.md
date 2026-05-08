# ADM-3 spec brief — multi-source audit 合并查询 + source enum 4 类 (≤80 行) [v1 推断 scope]

> 飞马 · 2026-05-01 · 用户拍板待 PR review (推断 scope: 来源 C 混合 = multi-source audit 合并查询) · zhanma 主战 + 飞马 review
> **关联**: ADM-2 #484 ✅ admin_actions table · ADM-3 rename #586 ✅ admin_actions→audit_events · BPP-8 #532 ✅ plugin lifecycle audit · HB-1 #491 ✅ install-butler audit · DL-2 #615 ✅ events 双流 · ADM-0 §1.3 admin god-mode 红线
> **命名**: ADM-3 = admin 第三件 (multi-source audit 合并); 跟 #586 rename milestone 同等级但不同 scope (本 v1 是 query 层合并, #586 是 schema rename)

> ⚠️ 推断 scope (PROGRESS [ ] **ADM-3** 来源 C 混合一句话, 用户无明确细则) — 本 spec v1 按 (a) multi-source audit 合并查询 写, PR review 用户拍板再调.
> ⚠️ Server-side query layer + admin UI milestone — **0 schema 改 / 0 endpoint URL 改 / 0 user-facing API 行为改** (复用 #586 audit_events 表 + DL-2 channel_events / global_events).

## 0. 关键约束 (3 条立场)

1. **复用既有 4 源 audit 表 + DL-2 events_archive byte-identical 不破** (跨 audit 链锁定): 4 来源 enum SSOT:
   - `server` — `audit_events` (#586 rename 后, ADM-2 既有, server-side action audit)
   - `plugin` — `audit_events` source='plugin' (BPP-8 #532 lifecycle)
   - `host_bridge` — install-butler audit log (HB-1 #491 5-field SSOT) + HB-2 v0(D) 真 IO audit
   - `agent` — DL-2 #615 channel_events / global_events (agent.state 等必落 kind)
   
   反约束: 反向 grep 4 source enum const SSOT count==4 hit (跟 reasons.IsValid #496 / NAMING-1 enum 模式承袭) + audit_events / channel_events / global_events / hb-1 audit 表 schema 不动 (0 column add / 0 migration v 号).

2. **multi-source 合并查询 SSOT + admin UI source enum 4 类 + admin god-mode 永久独立**:
   - **server query helper SSOT**: `internal/api/admin_audit_query.go` 新 (~120 行) — `MultiSourceAuditQuery(ctx, filter)` UNION ALL across 4 source tables + 统一 ResponseShape (source enum 4 类标识 + ts + actor + action + payload), 走 admin /admin-api/audit/multi-source endpoint **(NEW endpoint OK, 走 admin god-mode 独立 ACL, 跟 ADM-0 §1.3 红线 byte-identical 不破)**
   - **client admin UI**: `packages/client/src/components/admin/MultiSourceAuditView.tsx` 新 (~80 行) — source enum 4 类 filter dropdown + table view 走 capabilityLabel-style i18n (跟 AP-2 风格承袭)
   - **admin god-mode 路径独立**: 仅 `/admin-api/audit/multi-source` 暴露, **不挂 user-rail**, 反向 grep `/api/v1/audit/multi-source` 0 hit (ADM-0 §1.3 红线)
   - 反约束: 反向 grep `auditSource` enum 4 const SSOT count==4 hit + admin /admin-api/audit/multi-source endpoint 在 user-rail handler 0 hit + UNION ALL 跨 4 表查询单源.

3. **0 schema / 0 column add / 0 migration v 号 + admin god-mode 独立 ACL** (ADM-3 立场, 跟 INFRA-3/4 / REFACTOR-1/2 / NAMING-1 / RT-3 / DL-2/3 / HB-2 v0(D) / AP-2 系列承袭): PR diff 仅 (a) `internal/api/admin_audit_query.go` 新 query helper (b) `internal/api/admin_audit_query_test.go` ~10 unit (4 source × 2 case + filter + UNION) (c) `MultiSourceAuditView.tsx` client (d) i18n 4 source label (跟 AP-2 capability 同模式) (e) admin /admin-api/audit/multi-source endpoint route. 反约束: 0 schema column 改 + 0 migration v 号 + 0 user-rail endpoint 加 (仅 admin /admin-api/* 加).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 范围 |
|---|---|
| **ADM3.1 server query helper + admin endpoint** | `internal/api/admin_audit_query.go` 新 ~120 行 (MultiSourceAuditQuery + 4 source UNION ALL + ResponseShape SSOT) + admin /admin-api/audit/multi-source 新 endpoint + admin ACL gate (走 ADM-2 既有 admin auth middleware byte-identical) + ~10 unit test |
| **ADM3.2 client admin UI** | `packages/client/src/components/admin/MultiSourceAuditView.tsx` 新 ~80 行 + source enum 4 类 i18n key (admin/audit-sources.ts SSOT 跟 AP-2 capability i18n 模式承袭) + vitest ~6 case (4 source render + filter + admin path 独立 + 反 user-rail 漂) |
| **ADM3.3 closure** | REG-ADM3-001..008 (8 反向 grep + 4 source enum SSOT + 4 既有 audit 表 schema 不动 + admin god-mode 独立 + UNION 查询单源 + 0 user-rail endpoint + haystack 三轨过 + 既有 test 全 PASS) + acceptance + content-lock §1+§2 (4 source enum 字面 + i18n 4 key 锁) + 4 件套 |

## 2. 反向 grep 锚 (8 反约束)

```bash
# 1) 4 source enum SSOT 单源 (反 inline 字面漂)
grep -rcE 'AuditSourceServer|AuditSourcePlugin|AuditSourceHostBridge|AuditSourceAgent' packages/server-go/internal/api/admin_audit_query.go  # ==4 hit

# 2) 既有 4 audit 表 schema byte-identical 不破
git diff origin/main -- packages/server-go/internal/migrations/ | grep -cE '^\+\s*Version:|^\+.*ALTER TABLE.*audit'  # 0 hit

# 3) admin god-mode 独立 (ADM-0 §1.3 红线)
grep -rE '/api/v1/audit/multi-source|user-rail.*multi-source' packages/server-go/internal/api/  # 0 hit (仅 /admin-api/audit/multi-source)
grep -rE '/admin-api/audit/multi-source' packages/server-go/internal/server/server.go  # ==1 hit (admin route 单源)

# 4) UNION ALL 跨 4 表查询单源
grep -rcE 'UNION ALL' packages/server-go/internal/api/admin_audit_query.go  # ≥3 hit (4 表 UNION 至少 3 个 UNION ALL)
grep -rE 'audit_events|channel_events|global_events|install_butler_audit' packages/server-go/internal/api/admin_audit_query.go  | wc -l  # ≥4 hit (4 source 表名)

# 5) admin auth middleware 复用 (ADM-2 既有)
grep -rE 'AdminFromContext|adminAuth' packages/server-go/internal/api/admin_audit_query.go  # ≥1 hit (admin gate 真守)

# 6) DL-2 #615 mustPersistKinds 不破 (agent source 走 DL-2 cold consumer)
git diff origin/main -- packages/server-go/internal/datalayer/must_persist_kinds.go | grep -cE '^[-+]'  # 0 hit

# 7) i18n 4 source key SSOT (跟 AP-2 capability i18n 模式)
grep -cE 'audit\.source\.server|audit\.source\.plugin|audit\.source\.host_bridge|audit\.source\.agent' packages/client/src/i18n/  # ≥4 hit per 语言

# 8) haystack gate 三轨 + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./... && pnpm vitest run --testTimeout=10000  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **新 schema audit 表合并** — 0 schema 立场, 走 UNION ALL query 层合并
- ❌ **跨 source 反向追溯链** (e.g. agent action → host_bridge syscall trace) — 留 v3+
- ❌ **audit FTS 搜索** — 留 v3+ (本 v1 走 LIKE 简单 filter)
- ❌ **audit retention 跨 source 统一** — 走 DL-2 #615 既有 retention sweeper per-source 既有阈值, 不强制统一
- ❌ **user-rail audit feed (per-user 隐私视图)** — 永久不挂 (ADM-0 §1.3 红线 + 蓝图 §3.4 必落 kind 跟 user feed 不同 concern)
- ❌ **audit_events external export** (Splunk/Datadog) — 留 v2+ (跟 DL-3 #618 Prometheus 留账同精神)

## 4. 跨 milestone byte-identical 锁

- 复用 ADM-2 #484 + ADM-3 rename #586 audit_events 表 schema (字面不动)
- 复用 BPP-8 #532 plugin lifecycle audit (source='plugin')
- 复用 HB-1 #491 install-butler audit log 5-field SSOT (source='host_bridge')
- 复用 DL-2 #615 channel_events / global_events + mustPersistKinds (source='agent')
- 复用 ADM-2 既有 admin auth middleware (admin god-mode 路径独立)
- 复用 NAMING-1 #614 命名规范 + AP-4-enum #591 enum SSOT 模式
- 复用 AP-2 (并行) capability i18n SSOT 模式 (audit source 4 类 i18n 同精神)
- 0-schema-改 wrapper 决策树**变体**: 跟 RT-3 / DL-2/3 / HB-2 v0(D) / AP-2 同源

## 5. 派活 + 双签 + 飞马自审

派 **zhanma-c** (DL-2 #615 / DL-3 主战熟手, audit / events 域续作). 飞马 review.

✅ **APPROVED with 3 必修条件**:
🟡 必修-1: scope 推断 (multi-source audit 合并), PR body 必明示"等用户拍板再调"
🟡 必修-2: 4 既有 audit 表 schema byte-identical 不破 (反约束 grep #2 真守)
🟡 必修-3: admin god-mode 独立 (反约束 grep #3 真守, 仅 /admin-api/audit/multi-source 暴露, user-rail 0 hit)

担忧 (1 项, 中度): UNION ALL 跨 4 表性能 — v1 阈值哨 (DL-3 #618 events_row_count) 触发后人工决策切 (留 v2+ index hint 调优), 本 v1 简单 LIMIT 100 + ORDER BY ts DESC 即够.

**ROI 拍**: ADM-3 ⭐⭐ — multi-source audit 合并 + admin 视图统一, 跨 4 audit 链 (ADM-2/BPP-8/HB-1/DL-2) 真接, 后续 admin 操作 trace 解锁基座.
