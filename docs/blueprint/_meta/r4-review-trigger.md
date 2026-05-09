# R4 Review Trigger — Phase 2 退出 + Phase 3 启动锁定原则

> ⚠️ **历史回溯说明** (2026-05-06 审计): 此触发条件在 Phase 2 退出 (#284, 2026-04-28) 时即满足, 但 R4 决议从未启动. Phase 4+ 已全部收口 (PR #621), 此触发条件的锁定对象失效. 留作 R5+ 模板参考.

> 飞马 · 2026-04-28 · Phase 2 收尾 → Phase 3 BPP-1 启动前的强制评审环节. 沿用 R3 (#188+#189) 24h 节奏.

## 1. 触发条件 (任一满足即拉评审组, 触发后立即冻结 BPP-1 merge)

- **A**: Phase 2 退出关卡 ≥ 4/6 通过 (见 `docs/qa/phase-2-gate-status.md` v3) — 强制关卡 G2.0/2.3/2.audit 全 ✅ + 条件关卡 ≥ 1 ✅
- **B**: Phase 3 第一个 BPP-1 PR (BPP frame schema 锁定, 跟 G2.6 遗留行同 PR) 进入评审队列
- **C**: 保底条款 — Phase 2 进入收尾满 7 天仍未全过, 强制启动 R4 防止脱节

## 2. 四人轮替 (沿用 R3 评审组人选)

| 人 | 主审视角 |
|---|---|
| 飞马 | 原则冲突 + byte-identical 锁定 + 蓝图 vs 实施偏离 |
| 烈马 | 关卡条件性/强制性 + REG-CHECK 红线 |
| 野马 | 文案锁定 + 故障可解释 + 隐私承诺 |
| 建军 | 节奏 + 任务分配 + 最终签字 |

## 3. 应输出 (24h 内交付, 类似 R3 #188+#189)

- **R4-1** `docs/blueprint/r4-decisions.md`: 原则冲突 + 4 人决议 + 锁定说明 (R3 #188 schema)
- **R4-2** `docs/implementation/PROGRESS.md` 重排 (R3 #189): Phase 3 可启动顺序 + 工期 + 推迟项区
- **R4-3** 受影响蓝图 后续 PR ≤ 4 个 (R3 处理了 concept-model/agent-lifecycle/canvas-vision/realtime)
- **R4-4** Phase 4+ milestone 调整 (BPP 切换 / Hermes 多 runtime / Windows)

## 4. R3 经验 (参考)

#188 6 条原则冲突 → 4 蓝图文件 24h merge; #189 Phase 2 可启动顺序 ADM-0 + AP-0-bis + INFRA-2 + RT-0 + CM-onboarding → CM-4.3b/4.4 → 关卡 4, 工期净增 +8-10 天.
**红线**: R4 触发 → 24h 内 4 人 LGTM → 4 件输出全 merge 才解除 BPP-1 merge 限制.

## 5. 不在范围

- R4 决议具体内容 (触发后 4 人讨论才写) · R5 触发条件 (R4 跑完后再定)
