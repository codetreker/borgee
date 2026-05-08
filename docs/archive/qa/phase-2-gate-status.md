# Phase 2 闸进度看板 v3 — 实时翻牌 + 性质标 + 时间线

> 飞马 · 2026-04-28 · 整合 #221 + #227 + #231/#244 (G2.audit) + #236 (G2.3 ✅) + #237 (RT-0) + #243 (立场矩阵) + #248 (烈马 condition signoff)
> 用法: 每条 G2.X PR merge / 闸闭合后**当天**翻牌; 全 ✅ 时建军 + 野马联签宣布 Phase 2 退出
> v3 加: ① 性质标 (严格 / 条件性 / 留账) ② last-flip (PR # + 日期)

## 1. 6+1 闸看板

| 闸 | 状态 | 性质 | last-flip | 闭合 PR | owner | 备注 |
|---|---|---|---|---|---|---|
| G2.0 ADM-0 cookie 串扰反向 | ✅ | 严格 | #223 · 2026-04-26 | #197+#201+#223 | 飞马/战马A | v=10 后 `users WHERE role='admin'` count=0 |
| G2.1 邀请审批 E2E | 🟡 partial | 条件性 (#248) | #237 · 2026-04-28 | #195+#198+#237 | 战马A/烈马 | e2e cm-4-realtime.spec.ts 解 `.skip` 后真闭 |
| G2.2 离线 fallback E2E | 🟡 partial | 条件性 (#248) | #237 · 2026-04-28 | #237 | 战马A | RT-0 server 落; presence stub + e2e 后真闭 |
| G2.3 节流不变量单测 (B.1) | ✅ | 严格 | #236 · 2026-04-27 | #236 | 烈马 | T1-T5 全过, 二维 key + clock 注入 + 边界 `>=` |
| G2.4 用户感知签字 ⭐ | 🟡 2/5 | 条件性 (野马 ≥4/5) | #233 · 2026-04-27 | #213+#230+#232+#233 | 野马 | #2 等 AL-1b, #3/#4 等 e2e |
| G2.5 presence 接口契约 | 🟡 partial | 留账 (Phase 4) | #237 · 2026-04-28 | #237 | 飞马/战马A | `internal/presence/contract.go` 路径锁 PR |
| G2.6 /ws ↔ BPP schema lint | 🟡 partial | 留账 (Phase 4 CI) | #237 · 2026-04-28 | #237 | 飞马 | `bpp/frame_schemas.go` CI lint Phase 4 PR |
| G2.audit | 🟡 草稿 | 严格 | #244 · 2026-04-28 | #212+#231+#244 | 烈马 | 补 presence/节流/CHECK enforcement |

## 2. 通过判据 (#221 + 烈马 #248 condition signoff)

Phase 2 全过 ⇔ **严格闸** (G2.0/2.3/2.audit) ✅ + **条件性闸** (G2.1/2.2/2.4) 满足 #248 condition (G2.4 ≥4/5 野马签) + **留账闸** (G2.5/2.6) Phase 4 PR 编号 → 建军 + 野马联签

## 3. 剩余动作 (3 天路线, 锁 #227 + #248)

1. e2e `cm-4-realtime.spec.ts` 解 `.skip` (战马B 1-line) → G2.1/G2.2 真闭
2. `internal/presence/contract.go` 路径锁 PR (飞马+战马) → G2.5 留账行确认
3. G2.audit 补 presence/节流/CHECK enforcement (烈马, #244 后续)
4. 野马 G2.4 demo #3/#4 截屏 (AL-1b 后置) → 4/5
5. G2.6 CI lint PR (飞马, Phase 4 准备同 PR)

## 4. 翻牌红线

❌ 严格闸 partial 跳 ✅ 不补证据 · ❌ 条件性闸跳过 #248 condition 单独 ✅ · ❌ 留账闸 ✅ 而 Phase 4 PR 没编号 · ❌ G2.6 CI lint 口头"等 Phase 4" · ❌ G2.audit 留账行 "等以后", 规则 6 锁

## 5. 锚点

立场↔实施: `docs/blueprint/_meta/phase-2-stance-vs-impl.md` (#243). R3 决议: `docs/blueprint/_meta/r3-decisions.md`. 烈马 condition signoff: `docs/qa/signoffs/phase-2-uat-walkthrough.md` (#248).
