# R4 Review Trigger — Phase 2 退出 + Phase 3 启动锁立场

> 飞马 · 2026-04-28 · Phase 2 closing → Phase 3 BPP-1 启动前的强制 review 闸. 沿用 R3 (#188+#189) 24h 节奏.

## 1. 触发条件 (任一满足即拉群, 命中即冻 BPP-1 merge)

- **A**: Phase 2 退出 gate ≥ 4/6 闭 (锚 `docs/qa/phase-2-gate-status.md` v3) — 严格闸 G2.0/2.3/2.audit 全 ✅ + 条件性闸 ≥ 1 ✅
- **B**: Phase 3 第一个 BPP-1 PR (BPP frame schema lock, 跟 G2.6 留账行同 PR) 进 review queue
- **C**: 兜底 — Phase 2 进入 closing 满 7 天仍未全过, 强拉 R4 防漂移

## 2. 四人轮替 (沿用 R3 班底)

| 人 | 主审视角 |
|---|---|
| 飞马 | 立场冲突 + byte-identical 锁 + 蓝图 vs 实施漂移 |
| 烈马 | 闸条件性/严格性 + REG-CHECK 红线 |
| 野马 | 文案锁 + 故障可解释 + 隐私承诺 |
| 建军 | 节奏 + 派活 + 终签 |

## 3. 输出锁 (24h 内交付, 类似 R3 #188+#189)

- **R4-1** `docs/blueprint/r4-decisions.md`: 立场冲突 + 4 人决议 + 锁注 (R3 #188 schema)
- **R4-2** `docs/implementation/PROGRESS.md` 重排 (R3 #189): Phase 3 解封顺序 + 工期 + 后置区
- **R4-3** 受影响蓝图 follow-up PR ≤ 4 个 (R3 落 concept-model/agent-lifecycle/canvas-vision/realtime)
- **R4-4** Phase 4+ milestone 调整 (BPP cutover / Hermes 多 runtime / Windows)

## 4. R3 经验 (锚)

#188 6 条立场冲突 → 4 蓝图文件 24h merge; #189 Phase 2 解封顺序 ADM-0 + AP-0-bis + INFRA-2 + RT-0 + CM-onboarding → CM-4.3b/4.4 → 闸 4, 工期净增 +8-10 天.
**红线**: R4 触发 → 24h 内 4 人 LGTM → 4 件输出全 merge 才解冻 BPP-1.

## 5. 不在范围

- R4 决议具体内容 (触发后 4 人讨论才写) · R5 trigger (R4 跑完后再定)
