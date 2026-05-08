# Phase 2 Closing Checklist — Pre-announcement Verify

> 烈马 (QA) · 2026-04-28 · team-lead announcement PR 前置 verify 单. 任一 ❌ → block 重判.

## 1. 签字 / Stacked PR merged

- [ ] #233 野马 PM 立场签 / [ ] #248 烈马 QA 联签 / [ ] #244 REG-RT0 + g2-audit v2
- [ ] #239 战马B `.skip` 解 + latency ≤ 3s + 截屏 / [ ] #252 REG-AL1A + AUD-G2-AL1A / [ ] #254 Phase 4 index

## 2. G2 8 闸最终态

- [ ] G2.0 ✅ / [ ] G2.1 server ✅ + e2e ✅ (#239 后) / [ ] G2.2 ✅ / [ ] G2.3 ✅ (#236)
- [ ] G2.4 🟡 4/6 → 5/6 (#239 截屏 → 野马补) — #6 ADM-1 后 6/6
- [ ] G2.5 ✅ (#196/#198/#226; presence/contract.go AL-3 留账 #248 §7)
- [ ] G2.6 ✅ 注释锁 / CI lint Phase 4 BPP-1 留账 / [ ] G2.audit ✅ v2 12 行

## 3. Registry 最终行数

- [ ] 总计 64 行 (CM-4.x 8 + INFRA-2 7 + ADM-0 10 + AP-0-bis 6 + RT-0 8 + CM-onboarding 13 + 不变量 3 + CM-3 4 + AL-1a 5)
- [x] active 51 / pending 13 / [x] REG-RT0-008 ✅ #270 (post-merged 已翻 🟢, 历史 evidence)

## 4. 留账明文 (Phase 4 接, announcement 须列)

- [ ] G2.4 5/6+6/6 / [ ] G2.6 CI lint Phase 4 BPP-1 / [ ] AL-3 presence 表 + contract.go
- [ ] AL-1b busy/idle / [ ] ADM-1 #228 + ADM-2 audit / [ ] REG-RT0-007 60s fallback

## 5. Block 条件 (任一 = 重判)

- [ ] #239 e2e merge 失败 / latency > 3s → G2.1 e2e 降级, title 改 "条件性 server only"
- [ ] 任一 active 行变 🔴 (CI break) → 先修
- [ ] 6 刚性闸 (G2.0/G2.2/G2.3/G2.5/G2.6 注释锁/G2.audit) 任一跌 → block

## 6. Announcement PR 检查

- [ ] title 锁 "Phase 2 退出 gate 全过 (条件性) — 6 ✅ + 2 留账"
- [ ] body 引三签链: `g1-exit-gate.md` + `phase-2-yema-stance-signoff.md` + `g2-exit-gate-liema-signoff.md`
- [ ] Phase 4 派活清单引 `phase-4-templates-index.md` (#254)

## 7. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v0 — 6 段 verify + block 条件 + announcement 检查 |
