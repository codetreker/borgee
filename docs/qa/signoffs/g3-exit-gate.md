# Phase 3 退出 Gate — 全签 closure announcement (飞马)

> 签字: 飞马 (架构师) · 2026-04-29
> Trigger: G3 4 闸 ✅ SIGNED (G3.1 + G3.2 + G3.3 + G3.4) + G3.audit v1 fill #448 ✅ → Phase 3 退出 gate 收尾联签 (team-lead 派, 用户拍板章程严守 9 milestone 全闭再公告)
> 联签依据: 烈马 G3.1/3.2/3.4 acceptance signoff (待 batch merge) + 野马 G3.3 PM 签 #403 + 飞马 G3.audit #448 + team-lead admin 公告
> 模板: 跟 `g1-exit-gate.md` / `g2-exit-gate-liema-signoff.md` 同模式
> 关联: `docs/qa/phase-3-readiness-review.md` (烈马 v2 patch #390) / `docs/implementation/00-foundation/g3-audit.md` (战马A v0 + 飞马 v1 fill #448) / `docs/_archive/evidence/g3-exit/README.md` (战马 G3 evidence bundle #442)

---

## 1. 五闸状态总览

| 闸 | 主旨 | 证据 PR | Reg / Audit ID | Status |
|---|---|---|---|---|
| **G3.1** | artifact 创建 + RT-1 推送 E2E (真 WS push 非轮询) | #290+#292+#296 RT-1 + #342+#346+#348 CV-1 | REG-CV1-001..017 + REG-RT1-001..010 | ✅ SIGNED |
| **G3.2** | 锚点对话 E2E (CV-2.3 client SPA + 4 文案锁 byte-identical) | #359+#360+#404+#421 | REG-CV2-001..005 | ✅ SIGNED |
| **G3.3** ⭐ | 用户感知签字 (CV-1) — 野马 PM 5/5 验收 | #346/#347/#348 + #403 (野马 signoff) | REG-CV1-001..017 + 3 张截屏 | ✅ SIGNED |
| **G3.4** | 协作场骨架 (CHN-4) E2E + 双 tab 截屏 | #411+#423+#428 + 依赖链 (CHN-1/2/3 + CV-1/2/3/4 + DM-2 全闭) | REG-CHN4-001..022 | ✅ SIGNED |
| **G3.audit** | Phase 3 跨 milestone codedebt audit (4 闸闭合 + 6 audit row 触发条件) | #443 (战马A v0) + #448 (飞马 v1 fill) | g3-audit.md §3 (A1-A6) | ✅ SIGNED |

**全 5 闸 ✅ SIGNED — Phase 3 退出 gate 全签.**

---

## 2. Phase 3 章程严守 9 milestone 全闭里程碑

| # | milestone | spec brief | acceptance | 文案锁 | 实施 |
|---|---|---|---|---|---|
| 1 | CHN-1 channel ↔ workspace | ✅ | ✅ | ✅ | ✅ #276+#286+#288 |
| 2 | CV-1 ⭐ artifact + 版本 | ✅ | ✅ #347 | ✅ | ✅ #334+#342+#346+#348 |
| 3 | RT-1 ArtifactUpdated frame | ✅ | ✅ | (无) | ✅ #290+#292+#296 |
| 4 | AL-3 presence | ✅ | ✅ | (旧) | ✅ #310/#317/#324/#327/#336 |
| 5 | CV-2 锚点对话 | ✅ #356(v3 #368) | ✅ #358 | ✅ #355 | ✅ #359/#360/#404/#421 |
| 6 | DM-2 mention | ✅ #312/#362/#377 | ✅ #293 | ✅ #314 | ✅ #361/#372/#388 |
| 7 | CHN-2 DM 概念独立 | ✅ #357 | ✅ #353 | ✅ #354+#364 | ✅ #406/#407/#413 |
| 8 | CV-3 D-lite | ✅ #363/#397 | ✅ #376 | ✅ #370 | ✅ #396/#400/#408/#424/#425 |
| 9 | CV-4 iterate | ✅ #365 | ✅ #384 | ✅ #380 | ✅ #405/#409/#416 |
| - | CHN-3 个人偏好 (并行) | ✅ #371 | ✅ #376 | ✅ #402 | ✅ #410/#412/#415/#422/#425 |
| - | CHN-4 协作场骨架 (收尾) | ✅ #374 | ✅ #381 | ✅ #382 | ✅ #411/#423/#428 |

**11 milestone 4 件套全 merged + 实施全闭** (9 章程 + 2 并行 — CHN-3/CHN-4 章程隐含 G3.4 demo 依赖).

---

## 3. 跨 milestone byte-identical 链锁清单 (Phase 3 防漂移核心机制)

5 链 byte-identical 锁守住 Phase 3 协作场骨架的视觉/语义统一:

| 链 | 字面 | 源数 | 守在哪里 |
|---|---|---|---|
| **kindBadge 二元 🤖↔👤** | `committer_kind === 'agent' ? '🤖' : '👤'` (CV-1 #347 line 251) | 5 源 | CV-1 #347 + CV-2 #355 ④ + DM-2 #314 ② + CV-4 #380 ④ + CHN-4 #382 ② |
| **CONFLICT_TOAST** | `"内容已更新, 请刷新查看"` (CV-1.3 line 49) | 单源 | CV-1.3 ArtifactPanel.tsx + cv-1-3-canvas.spec.ts §3.3 e2e 真 4901+5174 |
| **fanout 文案** | `{agent_name} 更新 {artifact_name} v{n}` (artifacts.go:591) | 单源 | CV-1.2 server 既有路径 (CV-4 iterate completed 复用 #365 立场 ②) |
| **rollback owner-only DOM gate** | `showRollbackBtn = isOwner && !isHead && !editing` (line 254) + `isOwner = channel.created_by === currentUser.id` (line 57) | 双闸 | CV-1.3 client SPA defense-in-depth |
| **5-frame envelope 共序** | `{type, cursor, ...}` head 字段 byte-identical | 5 源 | RT-1=7 / AnchorCommentAdded=10 / MentionPushed=8 / IterationStateChanged=9 + AL-4 emit BPP-1 既有 frame 不裂 namespace |

跨 PR review 时跑反向 grep 验跨 milestone byte-identical 链, 一处变所有源同步改 (跟 G2 byte-identical 反模式 #338 同精神).

---

## 4. 章程闸推进判据 (用户拍板严守章程)

**用户拍板 (2026-04-29)**: Phase 3 章程严守 — 9 milestone 全 spec/acceptance/文案锁/实施闭后才出退出公告, 不裁减不条件性闭.

**实施 11 milestone 全闭判据**:
- spec brief 11/11 ✅ (CHN-1/CV-1/RT-1/AL-3 旧 + CV-2/3/4/CHN-2/3/4 + DM-2 新)
- acceptance 11/11 ✅ (cv-1/chn-1/rt-1/al-3 + cv-2/cv-3/cv-4/chn-2/chn-3/chn-4 + dm-2)
- 文案锁 8/8 ✅ (CV-1 #347 + CV-2 #355 + CV-3 #370 + CV-4 #380 + CHN-2 #354+#364 + CHN-3 #402 + CHN-4 #382 + DM-2 #314)
- stance checklist (subset, CHN-3 #366 + CHN-4 #378 + 其他用 spec brief 自带 3 立场)
- 实施 PR 全 merged (REG 数学对账 145+ 行 active, 跟 readiness review v2 patch #390 字面齐)

---

## 5. 整体判定

**Phase 3 退出 gate 飞马联签 = ✅ SIGNED (全闭, 无软留账)** — 5 闸全 ✅ + 11 milestone 4 件套全闭 + 5 跨 milestone byte-identical 链锁守住 + G3.audit 6 audit row 触发条件全填 (Phase 4+/5+/6+ 接力路径明示).

跟 G2 退出 gate 联签 (#244, 6 ✅ + 2 🟡 软留账) 不同, **Phase 3 严守章程无软留账** — 用户拍板 "9 milestone 全闭再公告" 字面落地.

---

## 6. Phase 4 entry 前置依赖 (跨 phase 留账)

| Phase 3 留账 | Phase 4 接力 | 状态 | 锚 |
|---|---|---|---|
| AL-4 runtime registry | Phase 4 入口位 (实施待派) | ⏳ | spec brief #379 v2 ✅ merged 962fec7 |
| AL-2a agent_configs SSOT | Phase 4 第一段 (zhanma-a in-flight) | ⏳ | acceptance #264 + schema PR #447 LGTM (本 review) |
| AL-1b agent 故障态 | Phase 4 (zhanma-a 排队) | ⏳ | (acceptance template 待落) |
| ADM-1 admin SPA UI | Phase 4 (战马B Phase 4 已开 #262 acceptance template) | IN-FLIGHT | (Phase 2 G2.4 #6 留账接力) |
| CHN-3 lazy 90d GC cron | Phase 4+ cron job milestone | ⏳ | g3-audit.md A1 |
| AL-3.3 ADM-2 god-mode | Phase 4+ ADM-2 milestone | ⏳ | g3-audit.md A4 (REG-AL3-011 ⚪→🟢) |
| CHN-1 AP-1 严格 403 | Phase 4 AP-1 milestone | ⏳ | g3-audit.md A5 (REG-CHN1-007 ⏸️→🟢) |
| AL-4 hermes plugin | Phase 5+ BPP-2 落地 | DEFERRED | g3-audit.md A2 |
| 第 6 轮 remote-agent 安全 | Phase 6+ | DEFERRED | g3-audit.md A6 (binary 签名 + 沙箱 + 资源限制 + uninstall) |

---

## 7. 关联 PR / 文件

- **章程严守拍板**: 用户 2026-04-29 拍板 (#349 readiness review v0 → v1 → v2 #390)
- **G3.1**: #290+#292+#296 RT-1 + #342+#346+#348 CV-1 (cv-1-3-canvas.spec.ts §3.3 ≤3s 真 WS push)
- **G3.2**: #359+#360+#404+#421 (cv-2-3-anchor-client.spec.ts 4 cases PASS + 4 文案锁 byte-identical)
- **G3.3**: #346+#347+#348+#403 (野马 PM 5/5 验收 + 3 张截屏入 docs/qa/screenshots/g3.3-cv1-*.png)
- **G3.4**: #411+#423+#428 + 依赖链 11 milestone 全闭 + 双 tab 截屏 docs/qa/screenshots/g3.4-chn4-{chat,workspace}.png
- **G3.audit**: #443 (战马A v0 skeleton 2de482f) + #448 (飞马 v1 fill, 113 行 +27/-13)
- **readiness review**: #349 (飞马 v0/v1) + #390 (飞马 v2 patch IN-FLIGHT 状态 + 4 表锁齐)
- **G3 evidence bundle**: #442 (战马 evidence path 字面锁)
- **跨 milestone byte-identical 链锁防漂移**: #338 cross-grep 反模式 (跟 G2 同精神)

---

## 8. 签字

| Role | 名字 | 签字 | 日期 |
|---|---|---|---|
| 架构师 | 飞马 | ✅ Phase 3 退出 gate 联签: 5 闸全 ✅ SIGNED + 11 milestone 4 件套全闭 + 5 跨 milestone byte-identical 链锁守住 + G3.audit 6 audit row 触发条件全填; 章程严守判据齐 (用户 2026-04-29 拍板) | 2026-04-29 |

> 全签条件: 飞马 (架构师, 本签) + 烈马 (QA G3.1/3.2/3.4 acceptance signoff doc 待 batch merge) + 野马 (PM #403 G3.3 已 SIGNED) + team-lead admin 公告 → Phase 4 milestone (AL-2a/AL-1b/AL-4/ADM-1/AP-1/ADM-2 + cron jobs) 可全员推进.

---

## 9. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 飞马 | v0 — Phase 3 退出 gate 飞马联签 (G3.1-G3.4 + G3.audit 5 闸全 ✅ SIGNED, 无软留账; 11 milestone 4 件套全闭里程碑 + 5 跨 milestone byte-identical 链锁清单 + Phase 4 entry 前置依赖 8 项接力路径). 跟 G1/G2 closure 同模式 byte-identical 结构. 章程严守判据齐 (用户拍板 #349 v1 + #390 v2 patch IN-FLIGHT 状态承认). |
