# PROGRESS — 实施进度打勾

> **单一进度真相**。任何 milestone / PR / gate 状态变化都更新此文件 (概览) 或对应 `progress/phase-*.md` 子文件 (detail).
>
> 形式: ✅ DONE / 🔄 IN PROGRESS / ⏳ PENDING (依赖未就绪) / ⏸️ BLOCKED (有阻塞需处理) / TODO (未开工)。
>
> 更新规则:
> - PR 合并 → 在对应 `progress/phase-*.md` 子文件行打 ✅, 提交注明 PR 号; 概览表同步翻 (主文件本表)
> - Phase gate 通过 → 在子文件 gate 行打 ✅, 注明证据 (PR / 截屏路径 / SQL 输出)
> - 标志性 milestone (⭐) 关闭 → 野马签字一行 (姓名缩写 + 日期) + 关键截屏 3-5 张存 `docs/evidence/<milestone>/`
> - 每周一由飞马 review 一遍, 落后项标 ⚠️ 并加备注
>
> **签字回滚条款 (野马 P3 弱采纳)**: ⭐ milestone 关闭后 1 周内, 野马在 dogfood 发现立场稀释可作废重做; 仅 reopen 该 milestone, 不阻塞下一 Phase (飞马 R2). 产品立场底线, 工程节奏不被反复打断.

---

## Phase 概览

| Phase | 状态 | 退出条件 | 备注 |
|-------|------|---------|------|
| Phase 0 基建闭环 | ✅ DONE | G0.1+G0.2+G0.3+G0.audit 全过 (G0.4/G0.5 软 gate, 不卡退出) | 起步; 含 INFRA-1a/1b 拆分; 实际 5 PR (#169-#173) 一日完成. detail → [`progress/phase-0.md`](progress/phase-0.md) |
| Phase 1 身份闭环 | ✅ DONE | G1.1~G1.5 + G1.audit 全过 | CM-1 + AP-0 + CM-3 全 merged; G1 全签 #210, G1.4 closed by #208 + #210. detail → [`progress/phase-1.md`](progress/phase-1.md) |
| Phase 2 协作闭环 ⭐ | ✅ DONE | 4 角色联签 + 5+1 闸 SIGNED | closure #284; 锚 `phase-2-exit-announcement.md`. → [`phase-2.md`](progress/phase-2.md) |
| Phase 3 第二维度产品 | ✅ DONE | 11 milestone + G3.1-G3.4 + G3.audit | RT-1/CHN-1/AL-3/CV-1/CV-2/3/4/CHN-2/3/4/DM-2/AL-4 全 ✅; G3 evidence #442. → [`phase-3.md`](progress/phase-3.md) |
| Phase 4+ 剩余模块 | ✅ DONE | G4.1-G4.5 + G4.audit closure | **46/46 milestone (41 G4 + 5 wave), REG 846**, cov 85%+. → [`progress/phase-4.md`](progress/phase-4.md) |

---

## In-flight 当前状态 (≤10 行)

- (history — Phase 4+ ✅ closure, post-closure follow-up wave merged; detail → phase-4.md)

---

## 子文件跳转

- Phase 0 detail → [`progress/phase-0.md`](progress/phase-0.md) — INFRA-1a/1b + G0.* 闸
- Phase 1 detail → [`progress/phase-1.md`](progress/phase-1.md) — CM-1 / AP-0 / CM-3 + G1.* 闸
- Phase 2 detail → [`progress/phase-2.md`](progress/phase-2.md) — 解封前置 + CM-4 + G2.* 闸 + closure #284
- Phase 3 detail → [`progress/phase-3.md`](progress/phase-3.md) — 11 milestone + G3.* 闸 + 野马 CV-1 签字
- Phase 4+ detail → [`progress/phase-4.md`](progress/phase-4.md) — AL/BPP/HB/RT/AP/CM/ADM/DL/CS 9 模块组 + 历史 changelog 归档

---

## v0 → v1 切换

参见 [`README.md`](README.md) 切换 checklist。完成日期: ___

---

