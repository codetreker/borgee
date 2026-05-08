# Phase 2 退出 gate 闭合判定 — 飞马

> 飞马 · 2026-04-28 · 给建军的判定建议 (非签字, 由建军/野马最终宣布)
> 源: `docs/implementation/00-foundation/phase-2-exit-gate.md` (#221)

## 1. 当前 6+1 闸状态

| 闸 | 当前 | 证据 | 闸位 owner | 备注 |
|---|---|---|---|---|
| G2.0 ADM-0 cookie 串扰反向断言 | ✅ | 4.1.a/b/c #201 + 4.1.d #223 (count==0) | 烈马 | ADM-0.3 落 → 4 轴齐全 |
| G2.1 邀请审批 E2E | 🟡 partial | 代码 #195 + #198 就位; e2e spec #218 skip 形 | 战马 / 烈马 | 等 RT-0 server (#226 写) 解 skip |
| G2.2 离线 fallback E2E | ⏳ | RT-0 server + presence stub 待落 | 战马 / 烈马 | 后置 |
| G2.3 节流不变量 单测 | ⏳ | 代码/单测均未挂 | 烈马 | 可独立挂, 不挡 G2.4 partial |
| G2.4 用户感知签字 ⭐ | 🟡 partial 2/5 | 野马 #213 partial 签; 截屏 #1/#5 已落, #2/#3/#4 留账 | 野马 (闸 4) | 留账明文: AL-1b + RT-0 e2e |
| G2.5 presence 接口契约 | ⏳ | `internal/presence/contract.go` 路径未建 | 飞马 / 战马 | RT-0 server PR 一并落 |
| G2.6 /ws ↔ BPP schema lint | ⏳ | client 端 schema 已 lock #218; server CI lint 待 RT-0 server | 飞马 | RT-0 server PR 一并加 |
| G2.audit | ✅ partial | #212 + 7 行 audit; presence/节流/CHECK 待 RT-0 后补 | 飞马 | 不挡宣布 |

## 2. 判定建议: ❌ **现在不宣布 "条件性全过"**, 等 RT-0 server merge 再判

### 理由

- **G2.0 / G2.audit 部分** 已 ✅, 这是过去几天的硬成果, 值得宣布"前半场 ✅"
- 但 **G2.1 / G2.5 / G2.6 三闸都被 RT-0 server (#226 战马A 在写) 一个 PR 同时解锁** — 该 PR ≤ 500 LOC, 1-2 天内可落; **强行此刻宣布"条件性全过"会让闸位失去信号意义**: G2.1 e2e 锁形未跑, G2.5 接口契约文件还不存在, G2.6 lint 未挂 — "条件" 太多, 后续口径不一致风险大
- **G2.4 partial 2/5 + 野马留账** 是**唯一可接受的"条件性"项** (用户感知主观签字本就允许 partial — R3 立场, 野马 #213 已签 partial), 但单这一项不能驱动整 Phase "条件性全过"
- **G2.3 节流单测**也未落, 但烈马可独立挂 (≤ 100 LOC), 不挡

### 建议路线 (按时间线)

1. **现在 (今/明天)**: RT-0 server PR 落地 (战马A) — 同时关闭 G2.1 / G2.5 / G2.6 三闸 (e2e 解 skip + presence/contract.go + CI lint)
2. **同期/并行**: 烈马挂 G2.3 节流单测 (`internal/notify/throttle_test.go`)
3. **G2.audit 补**: presence / 节流 / CHECK enforcement (ADM-0.3 deferred) 三行追入 audit registry
4. **闸 4 流程**: 野马跑 G2.4 demo 跑剩余 3/5 截屏 (AL-1b 完成后), 在签字流上 **G2.4 升 ✅**
5. **宣布 Phase 2 全过**: 当 G2.0/2.1/2.2/2.3/2.5/2.6/audit ✅ + G2.4 至少 4/5 ✅ (野马签条件性接受) → 建军 + 野马联签宣布

### 红线

- ❌ 不要绕过 G2.6 CI lint 直接宣布 (R3 schema lock 立场会被弱化)
- ❌ G2.4 不要降到 1/5 接受 (野马立场 #213 是 2/5 partial 已让步, 再降无信号)
- ❌ G2.audit 留账行不能"等以后", 必须本周内挂 (规则 6)

## 3. 一句话给建军

**现在宣布"前半场 ✅" (G2.0/audit) 没问题; 整 Phase 2 退出宣布请等 RT-0 server merge + G2.3 单测 + G2.4 至少 4/5 ✅, 预估 2-3 天内可达。**
