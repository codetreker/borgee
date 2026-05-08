# ADM-3 v1 e2e stance checklist — actor_kind 4-enum mixed 路径 e2e 真验 (server-only)

> 7 立场 byte-identical 跟 adm-3-v1-e2e-spec.md (飞马待 commit). **真兑现 G4.audit P2.3 (actor_kind mixed 路径无 e2e gap)** + 真兑现蓝图 §1.4 来源透明 mixed 来源场景. 0 schema / 0 endpoint shape 改 / 复用 ADM-3 #619 + ADM-3 #586 既有 audit_events 表. 跟 ADM-3 stance (commit f7305c1d) 7 立场 byte-identical 承袭. content-lock 不需 (server-only audit).

## 1. mixed 来源场景 e2e 真验 (蓝图 §1.4 真兑现)
- [ ] e2e #1: agent 代 user 执行 + admin 推翻 user 决策 → audit_events 写 actor_kind=`mixed` byte-identical
- [ ] e2e #2: 单一来源 (human / agent / admin 各) → actor_kind 各自非 mixed 字面 byte-identical
- [ ] e2e #3: mixed 来源 reason 透明真显 (audit log entry 真 reason 字面跟 5-field schema byte-identical)

## 2. 4 actor_kind enum byte-identical 跟 ADM-3 #619 真闭环
- [ ] `human` / `agent` / `admin` / `mixed` byte-identical (反 user/bot/system/hybrid 同义词漂)
- [ ] 反第 5 enum 漂入 (反 5/3 偷工减料, 跟 DL-1 4 interface count==4 + DL-2 retention 3-enum + AP-4-enum 14-cap 单源同精神)
- [ ] 字典分立锁链第 8 处真兑现 (跟 G4.audit signoff §2.1)

## 3. 5-field audit JSON-line schema byte-identical 跨六源
- [ ] `actor / action / target / when / scope` byte-identical 跨 HB-1+HB-2+BPP-4+HB-4+HB-3+ADM-3 (改一处 = 改六处单测锁)
- [ ] 跟 G4.audit P1.4 reflect lint 升 5→6 源 (加 ADM-3 audit_events) 协同
- [ ] audit-forward-only 立场延伸 (反 DELETE/UPDATE)

## 4. 0 schema 字段加 (复用 ADM-3 #586 audit_events 表)
- [ ] 反向 grep `migrations/adm_3_v1_` 0 hit + `currentSchemaVersion` 不动
- [ ] 复用 ADM-3 #586 audit_events 表 + alias view (RENAME backward compat)
- [ ] 反 ALTER 加 actor_kind 字段 (蓝图 §1.4 actor 字段隐式表达)

## 5. ADM-2 / AL-7 / AL-8 既有 audit 路径 byte-identical 不破
- [ ] ADM-2 #484 audit 写路径 byte-identical (alias view backward compat 守)
- [ ] AL-7 archived_at sweeper + AL-8 audit-forward-only 立场字面 + 行为 byte-identical
- [ ] 既有 unit + e2e 全 PASS byte-identical (反 race-flake)

## 6. 0 endpoint 行为改 (audit log 内部)
- [ ] 0 endpoint shape 改 (server.go register 不增, 反向 grep `\\+.*Method|\\+.*Register` 0 hit)
- [ ] 0 response body / 0 error code 字面改
- [ ] alias view backward compat 守 (反 admin_actions read 路径漂)

## 7. admin god-mode 不挂 audit_events 写 (ADM-0 §1.3 + audit-forward-only 红线)
- [ ] 反向 grep `admin.*audit_events|admin.*actor_kind` 在 packages/server-go/ 0 hit (admin 仅走 /admin-api/* 既有路径)
- [ ] anchor #360 owner-only ACL 锁链 22+ PRs 立场延伸 + REG-INV-002 fail-closed

## 反约束 — 真不在范围
- ❌ 加 actor_kind 字段 / migration / endpoint / client UI 改
- ❌ admin god-mode 加挂 audit_events 写 (永久不挂)
- ❌ 引入 actor_kind 第 5 enum (反 4 enum byte-identical 锁)
- ❌ audit DELETE/UPDATE (永久反, audit-forward-only 红线)

## 跨 milestone byte-identical 锁链 (5 链)
- ADM-3 #586 RENAME + alias view + ADM-3 #619 actor_kind 4-enum 真闭环
- ADM-2 #484 + AL-7 + AL-8 audit 路径 + audit-forward-only 立场延伸
- 5-field audit schema 锁链跨六源 (G4.audit P1.4 升 5→6 reflect lint 协同)
- DL-1 #609 4 interface + handler baseline N=108 + factory 单源
- anchor #360 owner-only ACL 22+ PRs + 字典分立第 8 处

## PM 拆死决策 (3 段)
- **mixed 路径 e2e vs unit-only 拆死** — Playwright e2e + 真 audit_events 写路径验 (本 PR), 反 unit mock 一刀切
- **复用 audit_events 表 vs 加 actor_kind 字段拆死** — 复用 (本 PR, 蓝图 §1.4 字段隐式)
- **4 actor_kind vs 5+ 漂拆死** — `human/agent/admin/mixed` byte-identical (本 PR), 反 user/bot/hybrid 同义词

## 用户主权红线 (5 项)
- ✅ mixed 来源透明真兑现 (蓝图 §1.4 字面承袭 e2e 真验)
- ✅ 既有 audit 路径 + ACL byte-identical 不破
- ✅ 0 schema 字段加 / 0 endpoint shape 改 (复用 audit_events)
- ✅ audit-forward-only 立场延伸 (反 DELETE/UPDATE)
- ✅ admin god-mode 不挂 audit_events 写 (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. e2e Playwright mixed 来源场景 PASS (agent + admin 双 source 真验)
2. 4 actor_kind byte-identical (`human|agent|admin|mixed`) 反第 5 漂入
3. 5-field audit schema 跨六源 byte-identical (reflect lint 升 5→6)
4. 0 schema 字段加 + 0 endpoint shape 改 + alias view backward compat 守
5. cov ≥85% (#613 gate) + audit-forward-only + admin grep 0 hit
