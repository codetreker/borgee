# Phase 2 退出 Gate — QA 联签 (烈马)

> 签字: 烈马 (QA) · 2026-04-28
> Trigger: PR #244 stacked merged → Phase 2 退出 gate 联签 (team-lead 派)
> 联签: 野马 PM #233 (`phase-2-yema-stance-signoff.md`) + team-lead admin 公告
> 模板: 跟 `g1-exit-gate.md` 同模式

---

## 1. 八闸状态总览

| 闸 | 主旨 | 证据 PR | Reg / Audit ID | Status |
|---|---|---|---|---|
| **G2.0** | CM-4 schema + state machine + handler | #185 + #198 | REG-CM4-001..006 | ✅ |
| **G2.1** | RT-0 server typed Push + no-op + schema lock | #237 | REG-RT0-001..005 | ✅ server / 🟡 e2e (战马B #239) |
| **G2.2** | INFRA-2 Playwright scaffold | #195 | REG-INFRA2-001..003 | ✅ |
| **G2.3** | dev-only 节流 T1-T5 5 测 | #236 | AUD-G2-G23 | ✅ |
| **G2.4** | 闸 4 demo 截屏 (业主感知) | #213 + 野马 partial | g2.4-demo-signoff.md | 🟡 4/6 (#3+#4 等 e2e, #6 等 ADM-1) |
| **G2.5** | bug-029 / bug-030 修 + sanitizer | #196 + #198 + #226 | REG-INV-002 + REG-INV-003 | ✅ |
| **G2.6** | BPP byte-identical 注释锁 | #237 godoc | REG-RT0-003 grep | 🟡 注释锁 ✅ / CI lint 等 BPP-1 |
| **G2.audit** | Phase 2 codedebt audit DRAFT v2 (11 行) | #244 | AUD-G2-ADM01/02/03-a/b + RT0-a/b/c + CM-onboard + AP0-bis + CM3 + G23 + CHECK | ✅ |

**6 ✅ + 2 🟡 留账 = 退出 gate 候选 4/6 已就位 (G2.0/G2.3/G2.6 注释锁/G2.audit) + G2.1/G2.5 server 已 ✅ + G2.4 软留账.**

---

## 2. Acceptance 验签 (烈马 QA)

- **G2.1 server** ✅ — `go test ./internal/ws/... ./internal/api/...` 12 测 PASS; typed Push 无 `interface{}`; nil-Hub / userID="" / frame=nil / offline 4 路 silent no-op 单测全绿; schema 与 #218 client TS 字段名 byte-identical (人工 grep 比对).
- **G2.1 e2e** 🟡 — `cm-4-realtime.spec.ts` 仍 `.skip`, 战马B #239 解锁 in-flight; 不阻退出 gate 联签.
- **G2.3** ✅ — `internal/throttle` T1-T5 (<budget / =budget / >budget / window reset / per-user 隔离) PASS, AUD-G2-G23 落地.
- **G2.5** ✅ — REG-INV-003 evidence 列加 `cm-onboarding-bug-030-regression.spec.ts` (#226 in #244).
- **G2.6** 🟡 — `event_schemas.go` godoc "byte-identical" 字面锁 + Phase 4 BPP-1 文档化; CI lint 等 BPP-1 PR; 注释锁是 grep 兜底.
- **G2.audit** ✅ — DRAFT v2 §3 11 行, AUD-G2-CHECK (SQLite ALTER ADD CHECK 不支持) single-source 落账.

---

## 3. 整体判定

**Phase 2 退出 gate QA 联签 = ✅ SIGNED (有限)** — 6 闸 ✅ (G2.0/G2.1 server/G2.2/G2.3/G2.5/G2.audit) + 2 软留账 (G2.4 demo 5/6+6/6, G2.6 CI lint). 与野马 #233 5 milestone 立场判定一致, 业主感知 ①②③④ ✅, ⑤ 邀请 ≤ 3s 代码就位 / e2e evidence 战马B #239.

---

## 4. 留账 (不阻 Phase 2 退出 gate)

- 🟡 **G2.4 5/6** 等战马B #239 (cm-4-realtime.spec.ts .skip 解锁 + 截屏) → 我补 REG-RT0-008 evidence + 野马补签.
- 🟡 **G2.4 6/6** 等 ADM-1 (Phase 4) AdminBanner demo merged → 野马补签.
- 🟡 **G2.6 CI lint** 等 Phase 4 BPP-1 cutover PR (`bpp/frame_schemas.go ↔ ws/event_schemas.go`).
- 🟡 **REG-RT0-007** 60s polling fallback e2e — Phase 2 收尾后置.

---

## 5. 关联 PR

#235 vitest CI / #237 RT-0 server / #218 RT-0 client / #226 bug-030 e2e / #236 G2.3 throttle / #244 stacked REG 翻牌 + g2-audit.md v2 / #213 G2.4 partial 4/6 / #233 野马 PM 立场.

---

## 6. 签字

| Role | 名字 | 签字 | 日期 |
|---|---|---|---|
| QA | 烈马 | ✅ Phase 2 退出 gate QA 联签 (有限): 6 刚性闸 ✅ + 2 软闸留账明文; 业主感知 ④ ✅ ⑤ 等战马B #239 | 2026-04-28 |

> 全签条件: 本签 (QA) + 野马 #233 (PM 立场) + team-lead admin 公告 + 战马B #239 merge → Phase 4 milestone (ADM-1 / AL-1b / BPP-1) 可全员推进.

---

## 7. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v1 — Phase 2 退出 gate QA 联签 (G2.0–G2.audit 8 闸, 6 ✅ + 2 🟡) |
