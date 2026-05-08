# Phase 2 Readiness Review — 飞马

> 飞马 · 2026-04-28 · Phase 2 退出公告前架构师 review (单一汇总)
>
> 源整合: `phase-2-gate-status.md` v3 (#251) · `phase-2-exit-gate-decision.md` (#227) ·
> `phase-2-closing-checklist.md` (#257) · `acceptance-templates/` · `regression-registry.md`
>
> 作用: 给建军 + 野马联签前提供一份独立 ✅/⚠️ 判定; 不替代签字, 替代"我得自己翻 8 个文档"。

## 1. 5+1 严格闸 SIGNED 状态汇总

| 闸 | 状态 | 性质 | 闭合 PR | 锚点 commit | 备注 |
|---|---|---|---|---|---|
| **G2.0** ADM-0 cookie 串扰反向 | ✅ SIGNED | 严格 | #197 + #201 + #223 | 440c46e (#223) | v=10 后 `users WHERE role='admin'` count=0 (4 轴齐全) |
| **G2.3** 节流不变量单测 | ✅ SIGNED | 严格 | #221 + #229 + #236 | 21ec16f (#236) | T1-T5 全过 / 二维 key + clock 注入 + 边界 `>=` |
| **G2.audit** v2 (12 行) | ✅ SIGNED | 严格 | #212 + #231 + #244 + #251 | d1c58c5 (#244) → b058b10 (#251) | presence / 节流 / CHECK enforcement / AL-1a 5 行 / RT-0 翻牌全部入册 |
| **G2.6** /ws ↔ BPP schema lint | ✅ 注释锁 | 留账 (Phase 4 BPP-1 CI lint) | #237 client/server schema 已 lock | 45f7f27 (#237) | CI lint 落 BPP-1 (PR `bpp/frame_schemas.go`); 锚验收模板 `acceptance-templates/rt-0.md` |

(以上 4 闸构成 #248 烈马 condition signoff 的 "刚性 ✅ 必备" 集)

## 2. 条件性 / 留账闸状态

| 闸 | 状态 | 性质 | 闭合 PR | 留账接力 | 备注 |
|---|---|---|---|---|---|
| **G2.1** 邀请审批 E2E | 🟢 (#239 后) | 条件性 | #195 + #198 + #237 + #239 | — | RT-0 latency ≤ 3s 真过 + 截屏 (c91e866 / #239) — `.skip` 已解 |
| **G2.2** 离线 fallback E2E | 🟢 server / 留账 e2e | 条件性 | #237 server | REG-RT0-007 60s fallback Phase 4 | server 落, presence stub e2e 走 fallback 路径已覆盖 |
| **G2.4** 用户感知签字 ⭐ | 🟡 4/6 → 5/6 (待野马) | 条件性 (≥4/5 接受, #248) | #213 + #230 + #232 + #233 + #239 截屏 | #6 ADM-1 后 6/6 (Phase 4) | #1/#5/#3/#4 已落; #2 等 AL-1b 截屏 (Phase 4); 野马 #233 partial 已签, #239 后野马补一行 |
| **G2.5** presence 接口契约 | 🟢 (注释锁) | 留账 (AL-3) | #196 + #198 + #226 | `internal/presence/contract.go` ↔ AL-3 (`acceptance-templates/` 6 项 #3) | 路径锁 + AL-3 验收模板 #3 项 (6) 显式接 — 留账有去处 |

## 3. PR 锚点速查 (`git log --oneline -50` 锁)

- ADM-0 三段: 440c46e (#223 ADM-0.3 v=10) ← b32b88b (#233 野马 4/5 partial) ← 5238a41 (#231 翻牌 + audit 草稿)
- RT-0 双段: 24b160c (#218 client) → 45f7f27 (#237 server) → c91e866 (#239 latency 真过)
- G2.3 节流: 9e4061f (#229 review prep) → 21ec16f (#236 T1-T5)
- AL-1a 三态: 5d6f772 (#249 三态) ← 2bb7cdc (#250 速读卡) ← 51e31f7 (#252 REG-AL1A + AUD-G2-AL1A)
- 闸看板演进: ef83078 (#238 v2) → b058b10 (#251 v3 性质标 + 时间线)
- closing checklist: 039e13a (#257) · UAT walk: 54e1c13 (#240) · 烈马 condition signoff: 5812fe5 (#248)

## 4. 是否 ready 出 Phase 2 退出公告

**✅ READY (条件性全过 — 6 ✅ + 2 留账)** — 飞马拍板支持建军 + 野马联签发布。

理由: ① 严格 4 闸 (G2.0 / G2.3 / G2.6 注释锁 / G2.audit v2) 全 ✅; ② 条件性 3 闸 (G2.1 / G2.2 / G2.5) 在 #237 + #239 后真闭, e2e `.skip` 已解 + latency ≤ 3s 截屏在册; ③ G2.4 4/5 (#239 后野马补到 5/6) 满足 #248 condition signoff 的 ≥4/5 红线; ④ 留账闸 (G2.5/2.6 → AL-3 / BPP-1, G2.4 #6 → ADM-1) **全部有 Phase 4 去处编号**, 不构成"等以后"。

⚠️ **Pre-flip 必检 (建军 PR 前)**:
1. 野马 G2.4 #239 后从 4/6 → 5/6 一行签字 (`docs/qa/signoffs/`)
2. closing-checklist (#257) §1 三签链 + §3 registry 64 行 active 51 / pending 13 全勾
3. announcement title 锁 "条件性全过" (closing §6) — 不允许"严格全过"避口径漂移

## 5. Phase 3 entry 前置依赖 (CHN-1 / CV-1 / RT-1) vs Phase 2 留账冲突点

**Phase 3 第一波** (`PROGRESS.md` Phase 3 锁定顺序):
1. **CHN-1** workspace ↔ channel 关联 (schema + API)
2. **CV-1** ⭐ artifact 表 + 版本机制 (野马签字 milestone)
3. **RT-1** ArtifactUpdated frame (server 转发)

**Phase 2 留账与 Phase 3 入口的依赖矩阵**:

| Phase 2 留账 | Phase 3 第一波碰撞? | 处置 |
|---|---|---|
| G2.5 presence/contract.go (留 AL-3) | ❌ CHN-1/CV-1 不依赖 presence; RT-1 frame 复用 ws hub 不碰 contract | 不阻塞 |
| G2.6 /ws ↔ BPP schema CI lint (留 BPP-1) | ⚠️ **RT-1 ArtifactUpdated frame schema 必须遵守 RT-0 client/server lock 模板** — 否则 BPP-1 CI lint 上线时整片 RT-* frame 翻车 | RT-1 PR 强制套用 #237 frame envelope 模板 + reviewer (飞马) 闸位人工 lint, 直到 BPP-1 CI lint 落 |
| G2.4 #6 ADM-1 (留 Phase 4) | ❌ 与 Phase 3 第一波解耦 | 不阻塞 |
| AL-1a 三态 (#249 已 Phase 2 起步) | ⚠️ CV-1 artifact 列表若用 agent presence 着色, 走 AL-1a 6 reason codes | 文案锁 `phase-3-stance-table-v0.md` (#246) 已就位 |

**Phase 3 启动 trigger** (`phase-3-trigger-conditions.md` #241): 4 项检查保留, 本 review 同意触发条件已满足前 3 项, 第 4 项 "Phase 2 退出公告 PR merged" 待联签后 ✅。

## 6. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 飞马 | v0 — Phase 2 readiness review (5 节, ready ✅ 条件性全过) |
