# ADM-3 stance checklist — 来源 C 混合 (蓝图 §1.4 来源透明: 人 / agent / admin / 混合)

> 7 立场 byte-identical 跟 adm-3-spec.md §0+§2 (飞马 v0 待 commit, 蓝图 §1.4 来源透明立场). **真有 prod code (audit_events.actor_kind 4 enum + 混合来源 helper SSOT) 但 0 schema 字段加 (复用 ADM-3 #586 RENAME 后 audit_events 表) / 0 endpoint 行为改 / 0 client UI**. 跟 ADM-3 #586 admin_actions → audit_events RENAME + alias view + AL-7/8 + DL-1 #609 4 interface + REFACTOR-1/2 字面锁同模式承袭. content-lock 不需 (server-only audit, 0 user-visible 字面改).

## 1. 4 actor_kind enum 分立 (蓝图 §1.4 byte-identical)
- [ ] **4 enum 分立** — `human` / `agent` / `admin` / `mixed` (来源 C 混合 = mixed) byte-identical 跟蓝图 §1.4 立场
- [ ] enum 命名锁 byte-identical, 反 `user / bot / system / hybrid` 同义词漂
- [ ] 反第 5 enum 漂入 (反 5/3 偷工减料, 跟 DL-1 4 interface count==4 + DL-2 retention 3-enum + AP-4-enum 14-cap 单源同精神)
- [ ] 跟字典分立锁链第 7 处承袭 (AL-1a 6-dict + HB-1 7-dict + HB-2 8-dict + AP-4-enum 14-cap + DL-2 3-enum + DL-3 3-const + AP-2 bundle + ADM-3 4-enum)

## 2. 混合来源 (mixed) 单源 helper SSOT
- [ ] **`actor_kind = "mixed"` 真兑现** — 反 `combined / multi_source / mixed_actor / hybrid_kind` 同义词漂
- [ ] mixed 判定 helper 单源 — 反多处散布 (反 SSOT)
- [ ] mixed 来源场景明示 (例: agent 代 user 执行 + admin 推翻 user 决策 = mixed) 跟蓝图 §1.4 字面承袭
- [ ] 反向 grep `combined|multi_source|hybrid_kind|mixed_actor` 在 packages/server-go/ 0 hit

## 3. 0 schema 字段加 (复用 ADM-3 #586 audit_events 表)
- [ ] 反向 grep `migrations/adm_3_2_|adm_3_3_` 在 packages/server-go/ 0 hit (反新 migration 漂)
- [ ] `currentSchemaVersion` 不动 (反向断 0 行改)
- [ ] 复用 ADM-3 #586 既有 audit_events 表 + alias view (RENAME 后 backward compat)
- [ ] 反 ALTER 加 `actor_kind` 字段 (蓝图 §1.4 actor 字段隐式表达, 反加显式 actor_kind 列漂)

## 4. ADM-2 / AL-7 / AL-8 既有 audit 路径 byte-identical 不破
- [ ] **ADM-2 #484 audit 写路径** byte-identical 不破 (跟 ADM-3 #586 alias view backward compat 承袭)
- [ ] **AL-7 #533 archived_at sweeper** + **AL-8 audit-forward-only** 既有立场字面 + 行为 byte-identical
- [ ] 既有 unit + e2e 全 PASS byte-identical (反 race-flake, 跟 #612/#613 cov 85% 协议承袭)
- [ ] audit-forward-only 立场延伸 (audit 不可 DELETE / UPDATE, 跟 ADM-3 #586 ADM-0 §1.3 红线扩展段承袭)

## 5. 5-field audit JSON-line schema byte-identical (跨 HB-1/HB-2/BPP-4/HB-4/HB-3/ADM-3 锁链)
- [ ] **5 字段 byte-identical** — `actor / action / target / when / scope` 跨 HB-1 install + HB-2 IPC + BPP-4 dead-letter + HB-4 release-gate + HB-3 grants + ADM-3 audit_events 同源 (改一处 = 改六处单测锁)
- [ ] 反向 grep AST scan 跨六源 struct 字段名 byte-identical (反新字段第 6 项漂入)
- [ ] 跟 HB-2 v0(D) `audit 写状态` 5 支柱字面承袭 (HB-4 release-gate 锁链)
- [ ] 跟 audit-forward-only 锁链跨 11+ 处立场延伸

## 6. 0 endpoint 行为改 (audit log 内部)
- [ ] 0 endpoint shape 改 — `git diff origin/main -- internal/api/server.go | grep -E '\\+.*Method|\\+.*Register'` 0 hit
- [ ] 0 response body / 0 error code 字面改 — 既有错码 byte-identical
- [ ] ADM-3 #586 alias view backward compat 守 (反 admin_actions read 路径漂)
- [ ] 0 client UI 改 (server-only audit, 跟 ADM-* + AL-* 同精神)

## 7. admin god-mode 不挂 audit (ADM-0 §1.3 红线)
- [ ] 反向 grep `admin.*audit_events|admin.*actor_kind` 在 packages/server-go/ 0 hit
- [ ] 反向 grep `/admin-api.*audit_events` 0 hit (admin 仅走 /admin-api/* 既有路径, 反额外端点漂)
- [ ] audit 走 user-rail / system-internal, 反 admin 跨用户写 audit (anchor #360 owner-only ACL 锁链 22+ PRs 立场延伸)
- [ ] ADM-0 §1.3 红线 + ADM-3 #586 audit-forward-only 段承袭

## 反约束 — 真不在范围
- ❌ 加 actor_kind 字段 / 加 migration v 号 / 加 schema 改
- ❌ 加新 endpoint / 改既有 endpoint shape / 0 client UI 改
- ❌ admin god-mode 加挂 audit_events 写路径 (永久不挂, ADM-0 §1.3 红线)
- ❌ 加新 CI step (跟 DL-1/2/3 + REFACTOR-1/2 + INFRA-3 + TEST-FIX-* 同精神)
- ❌ 引入 actor_kind 第 5 enum (反 4 enum byte-identical 锁)
- ❌ audit DELETE / UPDATE 路径 (永久反, audit-forward-only 红线)

## 跨 milestone byte-identical 锁链 (5 链)
- **ADM-3 #586** admin_actions → audit_events RENAME + alias view backward compat 承袭真兑现
- **ADM-2 #484 + AL-7 #533 + AL-8** audit 写路径 + archived_at sweeper + audit-forward-only 立场字面 byte-identical
- **5-field audit schema 锁链** — `actor/action/target/when/scope` 跨 HB-1/HB-2/BPP-4/HB-4/HB-3/ADM-3 byte-identical (改一处 = 改六处单测锁)
- **DL-1 #609 4 interface** — handler 走 Repository interface, baseline N=108 不增, factory 单源
- **anchor #360 owner-only ACL 锁链 22+ PRs** + REG-INV-002 fail-closed + ADM-0 §1.3 红线 + 字典分立第 7 处

## PM 拆死决策 (3 段)
- **4 actor_kind enum vs 5+ 漂拆死** — `human/agent/admin/mixed` 4 enum (本 PR 选), 反第 5 enum 漂入 (跟字典分立锁链承袭)
- **混合来源 mixed 单源 helper vs 散布判定拆死** — 单 helper SSOT (本 PR), 反多处散布 + 反 combined/multi_source/hybrid 同义词漂
- **复用 audit_events 表 vs 加 actor_kind 字段拆死** — 复用 ADM-3 #586 既有 (本 PR, 蓝图 §1.4 actor 字段隐式), 反 ALTER 加显式 actor_kind 列漂

## 用户主权红线 (5 项)
- ✅ 0 行为改既有 endpoint / 0 schema 字段加 (复用 audit_events 表)
- ✅ 既有 ACL gate 字面 + 行为 byte-identical (anchor #360 + REG-INV-002 守)
- ✅ audit-forward-only 立场延伸 (反 DELETE/UPDATE, 跟 ADM-3 #586 红线扩展段承袭)
- ✅ 0 user-facing change (server-only audit, 0 client UI 改)
- ✅ admin god-mode 不挂 audit_events 写路径 (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. 反向 grep `combined|multi_source|hybrid_kind|mixed_actor|migrations/adm_3_2_` count==0
2. 4 actor_kind enum byte-identical (`human|agent|admin|mixed` 反第 5 漂入)
3. 5-field audit schema byte-identical 跨 HB-1/HB-2/BPP-4/HB-4/HB-3/ADM-3 六源对锁
4. 0 schema 字段加 + 0 endpoint shape 改 (`git diff` 反向断言)
5. cov ≥85% (#613 gate) + audit-forward-only 守 + admin grep 0 hit
