# ADM-2 followup stance checklist — REG-ADM2-010 grant 校验 wire + REG-ADM2-011 admin SPA + 双截屏

> 7 立场 byte-identical 跟 adm-2-followup-spec.md (飞马待 commit). **真兑现 ADM-2 #484 deferred 2 行 (REG-ADM2-010 + REG-ADM2-011) + G4.x #4 双截屏 PM 必修 #2 真兑现**. 真有 prod code (admin handler 写动作前 grant 校验 wire + admin SPA `/admin/audit-log` 页 + e2e + 2 截屏归档) + client UI 字面 (content-lock 真锁). 跟 ADM-2 #484 既有 schema + ADM-3 #619 4 actor_kind enum 同源承袭. content-lock 必备 (admin SPA UI 字面真锁).

## 1. REG-ADM2-010 grant 校验 wire (admin 写动作前 403 守)
- [ ] **admin handler 写动作前 grant 校验 wire** — `store.ActiveImpersonationGrant` helper 既有 + 测试 PASS (ADM-2 #484 真留账完成), 本 PR 真 wire admin 写动作前 403 `impersonate.no_grant` 校验
- [ ] 反向 grep 反 admin handler 漏校验 (5/5 写动作: force_delete_channel / patch disabled / patch password / patch role / start_impersonation 全 wire)
- [ ] 失败字面 byte-identical `impersonate.no_grant` 跟 ADM-2 既有 5 模板字面承袭

## 2. REG-ADM2-011 admin SPA `/admin/audit-log` 页 + e2e + 双截屏
- [ ] admin SPA `/admin/audit-log` 页真启 (跟 ADM-2 既有 BannerImpersonate / AdminActionsList / ImpersonateGrantSection 同模式)
- [ ] e2e Playwright 真验 (audit-list 渲染 + red-banner impersonate 状态)
- [ ] 双截屏归档 → `docs/qa/signoffs/g4-screenshots/g4-2-adm2-audit-list.png` + `g4-2-adm2-red-banner.png` (PM 必修 #2 真兑现, G4.audit P0.1 第 2 项真补)

## 3. 5-field audit JSON-line schema byte-identical 跨六源不破
- [ ] `actor / action / target / when / scope` byte-identical 跨 HB-1+HB-2+BPP-4+HB-4+HB-3+ADM-3 (5-field reflect lint 升 5→6 源协同)
- [ ] audit-forward-only 立场延伸 (反 DELETE/UPDATE) byte-identical 不破

## 4. 0 schema 字段加 (复用 ADM-2 #484 既有)
- [ ] 复用 ADM-2 admin_actions schema v=22 + impersonation_grants schema v=23 既有 + ADM-3 audit_events alias view backward compat
- [ ] 反向 grep `migrations/adm_2_followup_` 0 hit + `currentSchemaVersion` 不动

## 5. owner-only ACL 锁链承袭 + admin god-mode 不挂 (ADM-0 §1.3 红线)
- [ ] grant 校验 wire owner-only (anchor #360 立场延伸 22+ PRs)
- [ ] 反向 grep `admin.*audit_events.*write|admin.*impersonation.*write` 0 hit
- [ ] admin SPA 仅 read-only audit-list (反 admin DELETE / UPDATE 漂, audit-forward-only 守)

## 6. 既有 unit + e2e 全 PASS byte-identical
- [ ] ADM-2 #484 既有 71 unit + ADM-3 既有 byte-identical 不破
- [ ] cov ≥85% (#613 gate, user memory `no_lower_test_coverage` 铁律)
- [ ] 0 race-flake (跟 #608 + #612/#613 deterministic 协议承袭)

## 7. agent ↔ human 同源 + sender_kind 反向漂
- [ ] admin SPA audit-list 渲染 actor_kind 4-enum (`human/agent/admin/mixed`) byte-identical 跟 ADM-3 #619
- [ ] 反向 grep `sender_specific_audit|admin_only_audit` 0 hit (agent ↔ human 同源 PR #568 §4 立场延伸)

## 反约束 — 真不在范围
- ❌ 加新 schema 字段 / migration v 号
- ❌ admin god-mode 加挂 audit_events 写 / impersonation 写 (永久不挂)
- ❌ audit DELETE / UPDATE (永久反, audit-forward-only 红线)
- ❌ 加新 CI step (跟 ADM-* + REFACTOR-* 同精神)

## 跨 milestone byte-identical 锁链 (5 链)
- **ADM-2 #484** REG-ADM2-001..009 + 既有 schema/handler/SPA 路径承袭
- **ADM-3 #586 + #619** actor_kind 4-enum + audit_events alias view backward compat
- **5-field audit schema 跨六源 reflect lint 升级** (跟 ADM-3 v1 e2e milestone 协同 G4.audit P1.4)
- **anchor #360 owner-only ACL 锁链 22+ PRs** + audit-forward-only + ADM-0 §1.3 红线
- **PM 必修 #2 G4.x 5 截屏归档路径** (G3.4 #590 同模式 docs/qa/signoffs/g4-screenshots/)

## PM 拆死决策 (3 段)
- **真 wire grant 校验 vs deferred 留账拆死** — 本 PR 真 wire (兑现 ADM-2 deferred), 反"再 follow-up"
- **admin read-only audit-list vs DELETE/UPDATE 拆死** — read-only (本 PR), audit-forward-only 永久反 DELETE/UPDATE
- **真截屏归档 vs 文字 signoff 拆死** — 真 PNG 归档 docs/qa/signoffs/g4-screenshots/ (本 PR, PM 必修 #2 真兑现)

## 用户主权红线 (5 项)
- ✅ 真兑现 ADM-2 deferred 2 行 (用户视角"未做"补真做)
- ✅ 既有 ACL gate 字面 + 行为 byte-identical
- ✅ audit-forward-only 立场延伸 (反 DELETE/UPDATE)
- ✅ 0 schema 字段加 / 0 既有 endpoint 改
- ✅ admin god-mode 不挂 audit_events 写 (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. admin handler 5/5 写动作前 grant 校验 wire (反向 grep 0 漏点)
2. admin SPA `/admin/audit-log` 页 + e2e Playwright PASS
3. 双截屏归档 docs/qa/signoffs/g4-screenshots/g4-2-adm2-{audit-list,red-banner}.png
4. 0 schema / 0 既有 endpoint shape 改 (`git diff` 反向断言)
5. cov ≥85% (#613 gate) + audit-forward-only + admin write 反向 grep 0 hit
