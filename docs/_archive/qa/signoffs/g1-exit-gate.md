# Phase 1 退出 Gate — 全签 signoff

> 签字: 烈马 (QA) · 2026-04-28
> Trigger: PR #208 CM-3 admin merged 07:01:59Z → Phase 1 退出 gate 收尾派活 (team-lead)
> 文件目的: 五道闸 + audit row 单一签字单, 后续 milestone / R4 议程引用即可不重抄。

---

## 1. 闸状态总览

| 闸 | 主旨 | 证据 PR | Reg ID | Status |
|---|---|---|---|---|
| **G1.1** | CM-1 organizations + users.org_id schema | #184 | (CM-1 schema migration v=2) | ✅ |
| **G1.2** | idx_*_org_id 5 个索引存在 | #184 | (TestCM11_CreatesOrgIDIndexes) | ✅ |
| **G1.3** | schema_migrations v=2 行落地 + 幂等 | #184 | (TestCM11_IsIdempotentOnRerun) | ✅ |
| **G1.4** | 读路径 EXPLAIN idx_*_org_id + JOIN owner_id 黑名单 + 跨 org 反向 403 | #208 + audit 2026-04-28 | REG-CM3-001..004 | ✅ |
| **G1.5** | AP-0 默认权限注册回填 `[message.send, message.read]` | #184 | (AUD-G1-AP0 + R3 Decision #1) | ✅ |
| **G1.audit** | Phase 1 跨 milestone codedebt audit row | g1-audit.md §3 | AUD-G1-CM1/AP0/CM4/CM3 (6 行) | ✅ |

**全 6 闸 ✅ — Phase 1 退出 gate 全签。**

---

## 2. G1.4 闭合细节

PR #184 merge 时 G1.4 标 ⏸️ deferred — 因 owner_id→org_id 替换发生在 CM-3 (#208), G1.4 audit 必须在 CM-3 落地之后再跑才有意义。

**2026-04-28 audit (本次)**:
- §2 黑名单: `JOIN.*owner_id` grep count==0 (排除 queries_cm3.go 自我文档化 regex)
- §3 反向 403: 4 sub-test 全 PASS (PUT/DELETE message + GET channel + GET workspace_files)
- §3 EXPLAIN: 6 主查询全部 `SEARCH ... USING INDEX idx_*_org_id`, 无 SCAN
- 全量 `go test ./...`: 16 packages 绿

证据 + EXPLAIN 输出全文 见 `docs/implementation/00-foundation/g1-audit.md` §2。

---

## 3. G1.audit row 落地

6 条 audit row 登记在 g1-audit.md §3:
- 4 条 `✅ closed` / `✅ stable` (即时闭合)
- 2 条 `📝 logged` (Phase 2 输入: organizations 软删 + owner_id retire)

无任何 `🔶 audit-warning`, 无任何 `⛔ broken`。

---

## 4. 关联 PR / 文件

- **PR #184** — CM-1 organizations + AP-0 默认权限 (G1.1/G1.2/G1.3/G1.5)
- **PR #185** — CM-4.1 agent_invitations API handler
- **PR #206** — AP-0-bis message.read 默认 + backfill (v=8)
- **PR #208** — CM-3 资源归属 org_id 直查 (v=9, G1.4 trigger)
- **PR #209** — registry flip AP-0-bis 6 🟢 + REG-INV-003
- (本 PR) — Phase 1 退出 gate 全签 + g1-audit.md + signoff

文件:
- `docs/implementation/00-foundation/g1-audit.md` (audit 报告)
- `docs/qa/regression-registry.md` (CM-3 + G1.4 4 🟢, Phase 1 引用区已更)
- `docs/qa/signoffs/g1-exit-gate.md` (本文)

---

## 5. 签字

| Role | 名字 | 签字 | 日期 |
|---|---|---|---|
| QA | 烈马 | ✅ Phase 1 退出 gate 全签, 6 闸全 ✅, audit row 落地无 warning | 2026-04-28 |

> Phase 1 全过, Phase 2 milestone (CM-4.x / INFRA-2 / RT-0 / ADM-0 / CM-onboarding) 可全员推进。

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v1 — Phase 1 退出 gate 全签 (G1.1–G1.5 + G1.audit) |
