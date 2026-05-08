# G3 Audit — Phase 3 退出闸 (audit 集成) — v1 fill

> 作者: 战马A v0 skeleton (#443) + 飞马 v1 fill · 2026-04-30 · Phase 3 退出 gate 收口
> 目的: G1 / G2 audit 同款单源 — Phase 3 全 11 milestone 落地后, 4 闸 + audit 一次集成.
> 状态: 🟢 **v1 fill** — 7 段齐 (PR # 锚 / 立场反查 / acceptance 闭 / 4 gate 状态 / evidence / 飞马 signoff 占位 / 留账)
> 关联: `docs/qa/regression-registry.md` (REG-* 翻 🟢) · `docs/_archive/evidence/g3-exit/README.md` (#442) · `docs/qa/signoffs/g3-exit-gate.md` (待落 — 建军真签).

---

## 1. Phase 3 实施 — 11 milestone PR # 锚

| Milestone | 三段 PR # | acceptance template | REG 翻 🟢 count |
|-----------|-----------|---------------------|----------------|
| **RT-1** server cursor + client backfill + BPP session.resume | #290 / #292 / #296 | `rt-1.md` | REG-RT1-001..010 = **11 🟢** (含 -010b) |
| **CHN-1** channel 模型 + workspace 自动建 | #276 / #286 / #288 | `chn-1.md` | REG-CHN1-001..010 = **13 🟢** (含 -001b/c/d) |
| **AL-3** presence 完整版 (5-state) | #310 / #317 / #324 + #327 | `al-3.md` | REG-AL3-001..010e = **6 🟢** + 4 ⚪ (留 §7-A4) |
| **CV-1** ⭐ artifact 创建 + 推送 | #334+#340 / #342 / #346+#348 | `cv-1.md` | REG-CV1-001..017 = **20 🟢** (含 -001b/c/d) |
| **BPP-1** envelope CI lint reflect | #304 | (覆盖 RT-1/CV-1/CV-2 frame) | REG-BPP1-001..009 = **9 🟢** |
| **CV-2** 锚点对话 | #359 / #360 / #404+#421 | `cv-2.md` (#358) | REG-CV2-001..005 = **6 🟢** |
| **CV-3** D-lite 画布渲染 | #396 / #400 / #408+#424+#425 | `cv-3.md` (#376) | REG-CV3-001..005 = **6 🟢** |
| **CV-4** artifact iterate 完整流 | #405 / #409 / #416 | `cv-4.md` (#384) | REG-CV4-001..010 = **8 🟢** + 2 永不实施 (§7-A3) |
| **CHN-2** DM 概念独立 | #407 / #406 / #413 | `chn-2.md` (#353) | REG-CHN2-001..009 = **9 🟢** |
| **CHN-3** 个人分组 reorder + pin | #410 / #412 / #415+#422+#425 | `chn-3.md` (#376) | REG-CHN3-001..012 = **12 🟢** |
| **CHN-4** 协作场骨架 demo | #411+#423 / #428 closure | `chn-4.md` (#381) | REG-CHN4-001..022 = **16 🟢** |
| **DM-2** mention (Phase 4 提前) | #361 / #372 / #388 | `dm-2.md` (#293) | REG-DM2-001..015 = **14 🟢** |
| **AL-4** runtime registry (Phase 4 提前) | #398 / #414 / #417+#427 | `al-4.md` | REG-AL4-001..010 = **6 🟢** + 4 ⚪ (留 BPP-2) |

合计: Phase 3 主线 13 milestone (含提前 DM-2/AL-4) **136 🟢 REG active**, 跨 milestone 全 merged.

---

## 2. 立场反查 — 跨 milestone byte-identical 不漂证据 (5 链)

REG 链承袭 — 一处真源, 五处单测 grep count==1 锁:

1. **kindBadge 二元** (`'agent' / 'human'`): cv-1 #347 ArtifactPanel.tsx:251 (源) ↔ #355 文案锁 ④ ↔ #314 DM-2 文案锁 ② ↔ #380 CV-4 文案锁 ④ ↔ #382 CHN-4 文案锁 ② — 5 处 byte-identical, REG-CV1-016 + REG-CV2-005 + REG-CHN4-007 反查.
2. **CONFLICT_TOAST** `内容已更新, 请刷新查看`: ArtifactPanel.tsx:49 (源) — REG-CV1-015 + e2e #348 §3.3 字面断言, 不漂.
3. **agent commit fanout** `{agent_name} 更新 {artifact_name} v{n}`: artifacts.go:591 (源) — REG-CV1-008 + CHN-4 acceptance #428 引同字面.
4. **rollback owner-only DOM gate** `showRollbackBtn = isOwner && !isHead && !editing`: ArtifactPanel.tsx:254 (源) — REG-CV1-014 + server #342 双层防御 (REG-CV1-007 admin 401 + non-owner 403 + lock-conflict 409).
5. **5-frame envelope 共序 (type/cursor 头位)**: ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 / IterationStateChanged 9 / RT-1 backfill 7 — BPP-1 #304 reflect lint 自动覆盖, REG-RT1-005 + REG-CV1-009 + REG-CV2-001 + REG-DM2-005 + REG-CV4-005 五源 golden JSON byte-equality 单测.

跨 milestone drift count==0 — 真 grep 验证 (regression-registry §3 全 🟢).

---

## 3. Acceptance template 全闭

`docs/qa/acceptance-templates/` 下 13 份 Phase 3 模板全 ✅:
- `rt-1.md` ✅ / `chn-1.md` ✅ / `al-3.md` ✅ (4 项 ⚪ 留 §7-A4) / `cv-1.md` ✅ #340+#347 / `cv-2.md` ✅ #358 / `cv-3.md` ✅ #376 / `cv-4.md` ✅ #384 / `chn-2.md` ✅ #353 / `chn-3.md` ✅ #376 / `chn-4.md` ✅ #381+#428 / `dm-2.md` ✅ #293 / `al-4.md` ✅ + #427.

13/13 acceptance template 真路径 evidence 全锚 (PR/SHA + 测试名 byte-identical), 跟 G2 模式同源.

---

## 4. G3 退出闸 4 道 + audit row 状态 (cut: 2026-04-30)

| 闸 | 状态 | signoff doc | 备注 |
|---|------|-------------|------|
| **G3.1** artifact 创建 + RT-1 推送 E2E ≤3s | ✅ **SIGNED** | `g3.1-rt1-cv1-liema-signoff.md` (烈马 2026-04-29) | 5/5 验收, 真 4901+5174 e2e ≤3s |
| **G3.2** 锚点对话 E2E | ✅ **SIGNED** | `g3.2-cv2-liema-signoff.md` (烈马 2026-04-29) | 5/5 验收, 反约束三连永久锁 + AnchorCommentAdded 10 字段 |
| **G3.3** ⭐ 用户感知签字 (CV-1) | ✅ **SIGNED** | `g3.3-cv1-yema-signoff.md` (野马 PM 2026-04-29 #403) | 5/5 验收, 3 张截屏路径承认 |
| **G3.4** 协作场骨架 (CHN-4) E2E + 双 tab | ✅ **SIGNED** (acceptance) | `g3.4-chn4-liema-signoff.md` (烈马 2026-04-29) | 5/5 验收 #428; 野马 5 张截屏 ⏸️ follow-up (§7-B) |
| **G3.audit** Phase 3 跨 milestone audit row | 🟢 **v1 fill** (本文) | (本文 §1-§7) | skeleton #443 + 飞马 v1 fill (本 PR) |

**通过判据**: G3.1 ✅ + G3.2 ✅ + G3.3 ✅ + G3.4 ✅ + G3.audit 🟢 → 飞马 G3 closure announcement (`g3-exit-gate.md` 待落, 建军真签).

---

## 5. Evidence bundle 锚

- **G3 evidence bundle**: `docs/_archive/evidence/g3-exit/README.md` (战马 #442 merged f71e26f) — 4 闸 evidence path + acceptance 闭锁 + 跨 milestone byte-identical 链全锚.
- **本 audit**: 本文 — Phase 3 代码债清单 + 闸状态总览 (跟 G1/G2 audit 同模式).
- **双轨**: evidence (#442) 给签字依据, audit (本文) 给代码债清单, 二者拆死不重合 — 跟 G1/G2 同款.

---

## 6. 飞马 signoff 占位 (建军真签)

> ⏳ **待签** — 本 v1 fill 落 PR 后, 飞马 (建军本人) 在 `docs/qa/signoffs/g3-exit-gate.md` 落 G3 closure announcement, 引本文 §4 4 闸全 ✅ + §1 11 milestone PR # 锚 + §2 5 链 byte-identical + §7 留账, 同模式跟 G1/G2 closure announcement.
> 占位字段: `签字角色: 飞马 (建军)` / `日期: TBD` / `引: 本文 §1-§5 + #442 evidence + #403 G3.3 野马 + 烈马 g3.1/g3.2/g3.4 三 signoff doc`.

---

## 7. 留账 (不阻塞 G3 闸通过, 入 Phase 4 follow-up)

| ID | 内容 | 触发 / 处理 Phase | 状态 |
|----|------|------------------|------|
| **A1** CHN-3 `user_channel_layout` lazy 90d GC cron (作者删 group 后孤儿行) | Phase 4+ cron job milestone (每日 0:00 UTC, 反向断言孤儿 count==0) | ⏸️ deferred |
| **A2** AL-4 hermes plugin 占号 (v1 仅 OpenClaw) | BPP-2 plugin 协议 v2 落地后启用 (蓝图 §2.2 字面 v1 only OpenClaw) | ⏸️ deferred (4 ⚪ REG-AL4) |
| **A3** CV-4 iterate retry 路径 — failed 不复用 iteration_id | **永不实施** (反约束 #380 ⑦ + #365 ② state machine + autoRetry grep count==0) | 🔒 锁死 |
| **A4** AL-3.3 ADM-2 god-mode 元数据 (REG-AL3-011 ⚪) | ADM-2 milestone 落地后 flip 🟢 (已 ✅ #484 — Phase 4 收尾时 flip) | 🔄 in-progress |
| **A5** CHN-1 AP-1 严格 403 (REG-CHN1-007 ⏸️→🟢 已翻) | AP-1 实施后 (已落) — Phase 4 audit 复核 | 🟢 已 flip |
| **A6** 第 6 轮 remote-agent 安全 (binary 签名 / 沙箱 / 资源限制 / uninstall) | Phase 6+ milestone (蓝图 §4 字面留第 6 轮) | ⏸️ deferred |
| **B1** G3.4 野马双 tab 5 张截屏 follow-up | CHN-4 closure 后 follow-up `page.screenshot()` 入 git (3 张已 landed, 5 张 ⏸️) | ⏸️ deferred |
| **B2** G3.2 烈马 acceptance signoff doc (已落) | `g3.2-cv2-liema-signoff.md` (烈马 2026-04-29) | ✅ 已落 |

**入册位置**: 此 audit 行**不**进 `docs/qa/regression-registry.md` (那个是回归 active 红线; audit 是 v0 代码债清单, 二者拆死). 后续 milestone 引本文即可.

---

## 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 战马A | v0 — G3 audit skeleton 4 闸概览 + 6 项留账候选 (#443) |
| 2026-04-30 | 飞马 | v1 fill — 7 段齐: §1 11 milestone PR # + REG 🟢 count 真查 (136 🟢) / §2 5 链 byte-identical 跨 milestone drift count==0 / §3 13 acceptance template 全闭 / §4 4 闸全 ✅ SIGNED + audit 🟢 / §5 evidence bundle #442 + 双轨 / §6 飞马 closure 占位 (建军真签) / §7 留账 8 项 (A1-A6 + B1-B2, A5 已 flip / B2 已落 / A3 永不实施 / 其余 follow-up Phase 4+ 不阻 G3 闸). 跟 PROGRESS.md G3.audit 🔄→✅ flip 对齐. |
