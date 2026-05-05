# G3 Phase 3 退出闸 — Evidence Bundle (战马 prep, 野马/烈马/飞马 签前)

> **状态**: v0 (战马A, 2026-04-29) — implementation evidence 整合完毕, 等签字闸落
> **目的**: Phase 3 9 milestone 实施 100% 闭环 (CHN-1/RT-1/AL-3/BPP-1/CV-1/CV-2/CV-3/CV-4/DM-2/CHN-2/CHN-3/CHN-4 全 merged), 此 bundle 把 4 闸 + audit 的 evidence 路径锁死, 给三角色 (野马/烈马/飞马) 签字铺路, 防 evidence 散落漏查
> **同模式**: G2.4 #275 / G2.5 #277 / G2.6 #274+#280 / G3.3 ⭐ CV-1 #403 (野马签字) — 截屏路径预承认 + acceptance template 闭锁 + 跨 milestone byte-identical 链
> **关联**:
>  - `docs/qa/phase-3-readiness-review.md` (烈马 v2 patch #390, READY 翻牌依据)
>  - `docs/qa/g3-screenshot-audit.md` (战马C audit, 命名 drift 修补)
>  - `docs/qa/signoffs/g3.3-cv1-yema-signoff.md` (野马 G3.3 单 milestone signoff)
>  - `docs/qa/screenshots/README.md` §1 (路径锁全表)

---

## 0. 4 闸 + audit overview

| 闸 | 状态 | 严格度 | 实施 evidence | 签字角色 |
|----|------|--------|--------------|----------|
| **G3.1** artifact 创建 + 推送 E2E (RT-1 真 WS push 非轮询) | ✅ READY | 严格 | RT-1 #290+#292+#296 + CV-1.2 #342 + CV-1.3 #346/#348 真 4901+5174 e2e ≤3s | 烈马 (acceptance) |
| **G3.2** 锚点对话 E2E | ✅ READY | 严格 (章程) | CV-2.1 #359 + CV-2.2 #360 + CV-2.3 #404 + REG-CV2 #421 | 烈马 (acceptance) |
| **G3.3** ⭐ 用户感知签字 (CV-1) | ✅ **SIGNED** (#403) | 严格 (野马) | g3.3-cv1-yema-signoff.md 5/5 验收通过 | 野马 (PM) ✅ |
| **G3.4** 协作场骨架 (CHN-4) E2E + 双 tab 截屏 | ✅ READY | 严格 | CHN-4 #411 + #423 follow-up + CHN-2/3 + CV-1/2 全闭 | 战马 e2e + 烈马 acceptance + 野马 双 tab 截屏文案锁验 三签 |
| **G3.audit** Phase 3 代码债 audit 行已登记 | ⚪ pending | 弱 (软 gate) | 飞马职责 — 跟 G0.audit/G1.audit 同模式, 留账行入 README §audit | 飞马 |

**总览**: G3.3 已签 (#403); G3.1/G3.2/G3.4 evidence 全就位等签字; G3.audit 飞马轻量 prep.

---

## 1. G3.1 artifact 创建 + RT-1 推送 E2E (真 WS push 非轮询)

### 1.1 实施 evidence (全 merged)

| 段 | PR | SHA | acceptance 锚 | 测试名 |
|---|----|-----|------|------|
| RT-1.1 server cursor + envelope 7 字段 | #290 | d1538f5 | rt-1.md §1 + REG-RT1-001..005 | `cursor_test.go::TestArtifactUpdatedFrameFieldOrder` (golden JSON byte-identical) |
| RT-1.2 client backfill ≤3s + 离线 30s × 5 + 2-tab dedup | #292 | (merged) | rt-1.md §2 + REG-RT1-006..008 | `packages/e2e/tests/rt-1-2-backfill.spec.ts` 真路径 |
| RT-1.3 BPP session.resume 三 hint | #296 | (merged) | rt-1.md §3 + REG-RT1-009..010 | `internal/ws/session_resume_test.go` 三 hint 反约束 |
| CV-1.2 server commit/rollback owner-only | #342 | b2ed5c0f | cv-1.md §2.1-§2.5 + REG-CV1-005..011 | `cv_1_2_artifacts_test.go` 11 cases PASS |
| CV-1.3 client SPA + WS push refresh | #346 | 623c1bb | cv-1.md §3.1-§3.3 + REG-CV1-012..016 | `__tests__/ws-artifact-updated.test.ts` 5 vitest |
| CV-1.3 e2e 真 4901+5174 ≤3s | #348 | 0ef0cb1 | cv-1.md §3.* + REG-CV1-017 | `packages/e2e/tests/cv-1-3-canvas.spec.ts` 2 playwright PASS ~3.7s |

### 1.2 G3.1 签字依据 — 烈马 acceptance signoff path

**已 ✅** (实施 100% 闭): `docs/qa/phase-3-readiness-review.md:14` 已锁 G3.1 ✅ SIGNED + #348 e2e 真路径锚.

**关键 evidence (烈马查这些)**:
- `cv-1-3-canvas.spec.ts::§3.3 WS push refresh ≤3s + conflict toast 文案锁` PASS
- frame 7 字段 byte-identical 跟 RT-1.1 #290 envelope 同源 (REG-CV1-009)
- ArtifactUpdated frame `{type, cursor, artifact_id, version, channel_id, updated_at, kind}` byte-identical, BPP-1 #304 envelope CI lint reflect 自动覆盖

### 1.3 不在范围
- ❌ RT-1.2/1.3 e2e 已 ⚪→🟢 翻 #298 closure patch (REG-RT1 全 🟢)

---

## 2. G3.2 锚点对话 E2E

### 2.1 实施 evidence (全 merged)

| 段 | PR | SHA | acceptance 锚 | 测试名 |
|---|----|-----|------|------|
| CV-2.1 schema v=14 (artifact_anchors + anchor_comments 双表) | #359 | c5bf03d | cv-2.md §1 + REG-CV2-001 | `cv_2_1_anchor_comments_test.go` 8 PASS |
| CV-2.2 server REST + WS push (10 字段 byte-identical) | #360 | 84f9e5d | cv-2.md §2 + REG-CV2-002..003 | `cv_2_2_anchors_test.go` + `anchor_comment_frame_test.go` |
| CV-2.3 client SPA — 选区→锚点 entry + thread side panel + WS push 接入 | #404 | 693e70c | cv-2.md §3 + REG-CV2-004..005 | `cv-2-3-anchor-client.spec.ts` 4 playwright PASS |
| REG-CV2 add 5🟢 | #421 | (merged) | regression-registry §3 CV-2 段 | — |

### 2.2 G3.2 签字依据 — 烈马 acceptance signoff path

**关键 evidence (烈马查这些)**:
- `cv-2-3-anchor-client.spec.ts::§3.1 选区→锚点 entry button + tooltip "评论此段" byte-identical` (#355 文案锁 ① 同源)
- `§3.2 thread panel literals byte-identical` 4 字面 (`段落讨论` / `针对此段写下你的 review…` / `标为已解决` / `🤖 reply`)
- `§3.5 WS push 实时 ≤3s` 真 server-go(4901)+vite(5174) PASS
- `§3.6 反约束 agent 视角 DOM 无入口 (count==0)` defense-in-depth + server #360 anchor.create_owner_only 兜底
- AnchorCommentAddedFrame 10 字段 byte-identical (`{type, cursor, anchor_id, comment_id, artifact_id, artifact_version_id, channel_id, author_id, author_kind, created_at}`) — author_kind 命名拆 commit 之 committer_kind (anchor 是评论作者非提交者, spec v2 字面锁)

### 2.3 不在范围
- ❌ G3.2 demo 截屏 — 走 G3.4 demo bundle (跟 #391 §1 路径锁 byte-identical)

---

## 3. G3.3 ⭐ 用户感知签字 (CV-1) — 已 ✅ SIGNED

### 3.1 已签依据
- **PR**: #403 docs(g3.3): CV-1 用户感知签字闸 ✅ SIGNED + 截屏路径预备
- **signoff doc**: `docs/qa/signoffs/g3.3-cv1-yema-signoff.md` (野马 PM, 2026-04-29)
- **5/5 验收通过**:
  ① artifact 归属 channel — 无 owner_id 主权列 ✅
  ② 单文档锁 30s TTL — last-writer-wins + 409 conflict toast byte-identical ✅
  ③ 版本线性 + agent 默认无删历史权 + rollback owner-only DOM gate 三条件 ✅
  ④ committer_kind 二元 🤖↔👤 byte-identical (跨 milestone 五处单测锁源头) ✅
  ⑤ ArtifactUpdated WS frame 7 字段 byte-identical + 实时 ≤3s + cursor 单调 ✅

### 3.2 截屏路径承认 (CI Playwright 后置补归档, 跟 G2.4 #275 同模式)

| 截屏 | 路径 | 状态 | 验内容 |
|------|------|------|--------|
| markdown render baseline | `docs/qa/screenshots/g3.3-cv1-markdown-render.png` | ⏸️ 待补 (path 已锁 #391 §0) | markdown 渲染 + `data-artifact-id` + kindBadge 二元 |
| commit dropdown agent fanout | `docs/qa/screenshots/g3.3-cv1-commit-dropdown.png` | ⏸️ 待补 | version dropdown v1/v2 + agent commit fanout `{agent_name} 更新 {artifact_name} v{n}` |
| rollback flow gate | `docs/qa/screenshots/g3.3-cv1-rollback-flow.png` | ⏸️ 待补 | rollback DOM gate 三条件 + 409 conflict toast + version label "v3 (rollback from v1)" |

**补法**: `cv-1-3-canvas.spec.ts` 加 `page.screenshot({path: 'docs/qa/screenshots/g3.3-cv1-*.png'})` 入 git (Playwright 主动归档反 PS 修改, 跟 G2.4 #275 同模式), 留 follow-up PR.

---

## 4. G3.4 协作场骨架 (CHN-4) E2E + 双 tab 截屏

### 4.1 实施 evidence (全 merged)

| 段 | PR | SHA | acceptance 锚 | 测试名 |
|---|----|-----|------|------|
| CHN-4 协作场骨架 client + 双 tab + G3.4 双截屏 | #411 | c37dd5e | chn-4.md §1+§2+§3 | `chn-4-collab-skeleton.spec.ts` 双 tab + page.screenshot |
| CHN-4 follow-up e2e — 反约束兜底 + 跨 org 隔离 + 2 边界态截屏 | #423 | 3da88e7 | chn-4.md §4 反向 grep 兜底 | `chn-4-collab-skeleton.spec.ts` 反约束兜底 |
| CHN-4 closure (acceptance + REG + PROGRESS) | #428 | (merged) | chn-4.md 全 ✅ + REG-CHN4-001..005 🟢 | docs only |
| **依赖链**: CHN-1 ✅ + CHN-2 ✅ (#406/#407/#413) + CHN-3 ✅ (#410/#412/#415/#422/#425) + CV-1 ✅ + CV-2 ✅ (#359/#360/#404/#421) + CV-3 ✅ (#396/#400/#408/#424/#425) + CV-4 ✅ (#405/#409/#416) + DM-2 ✅ (#361/#372/#388) | — | — | — | — |

### 4.2 G3.4 签字依据 — 三签

**战马 e2e**: `packages/e2e/tests/chn-4-collab-skeleton.spec.ts` 真 4901+5174 双 tab + 反约束 + 跨 org 隔离 PASS ✅

**烈马 acceptance**: `docs/qa/acceptance-templates/chn-4.md` §1+§2+§3+§4 全 ✅ (#428 closure 翻牌)

**野马 双 tab 截屏文案锁验** (跟 #382 文案锁 ⑥ + #391 §1 byte-identical 同源):

| 截屏 | 路径 | 状态 | 验内容 |
|------|------|------|--------|
| chat tab active | `docs/qa/screenshots/g3.4-chn4-chat.png` | ⏸️ 待补 (path 已锁 #391 §1 line 38) | "聊天" tab active + agent 🤖 二元角标 + 私信不混排 |
| workspace tab active | `docs/qa/screenshots/g3.4-chn4-workspace.png` | ⏸️ 待补 (path 已锁) | "工作区" tab active + artifact list `data-artifact-kind` 三态 + iterate 按钮 🔄 owner 视角 |
| CV-2 anchor entry 💬 | `docs/qa/screenshots/g3.4-cv2-anchor-entry.png` | ⏸️ 待补 (#355 文案锁 ① 同源) | 段落锚点入口 💬 + tooltip "评论此段" |
| CV-2 thread bubble | `docs/qa/screenshots/g3.4-cv2-thread-bubble.png` | ⏸️ 待补 (#355 ②) | 锚点对话气泡 + header "段落讨论" |
| CV-2 agent reply | `docs/qa/screenshots/g3.4-cv2-agent-reply.png` | ⏸️ 待补 (#355 ④) | agent 锚点回复 🤖 + author_kind="agent" |
| CV-3 markdown baseline | `docs/qa/screenshots/g3.4-cv3-markdown.png` | ✅ landed #408 | markdown render baseline (CV-3 三 kind 起步) |
| CV-4 iterate pending | `docs/qa/screenshots/g3.4-cv4-iterate-pending.png` | ✅ landed #422 | iterate 4 态 pending 起手 + 反约束无重试 |
| CV-4 iterate error baseline | `docs/qa/screenshots/g3.4-cv4-iterate-error-baseline.png` | ✅ landed | iterate failed 态 + reason byte-identical 跟 AL-1a #249 三处单测锁同源 |

**补全机制**: 走 follow-up PR — `chn-4-collab-skeleton.spec.ts` + `cv-2-3-anchor-client.spec.ts` 加 `page.screenshot()` 入 git, 跟 #391 §1 路径锁 byte-identical (反 PS 修改).

### 4.3 跨 milestone byte-identical 链 (G3.4 锁字面源头链)

CV-1 是源头, G3.4 demo 路径全继承:
- **kindBadge 二元** 🤖↔👤 — `ArtifactPanel.tsx:251` 源 → CV-2 锚点回复 + DM-2 mention 渲染 + CV-4 iterate completed + CHN-4 chat tab agent 角标 (五处单测锁)
- **CONFLICT_TOAST** `"内容已更新, 请刷新查看"` — `ArtifactPanel.tsx:49` 源, REG-CV1-015
- **fanout 文案** `{agent_name} 更新 {artifact_name} v{n}` — `artifacts.go:591` 源, byte-identical 跟 #380 ④ + #382 ② + onboarding-journey #391 §7.x
- **rollback DOM gate** 三条件 — `ArtifactPanel.tsx:254` 源, defense-in-depth 模式承袭到 CV-4 iterate / AL-4 启停 / CV-2 anchor 创建按钮
- **5-frame envelope 共序** type/cursor 头位 — RT-1=7 / AnchorComment=10 / MentionPushed=8 / IterationStateChanged=9 / AL-4 复用 BPP-1 既有 (#304 CI lint 自动覆盖)

---

## 5. G3.audit 代码债 audit 留账 (飞马职责, 软 gate)

### 5.1 待飞马补的 audit 行 (跟 G0.audit/G1.audit 同模式)

挂 `docs/implementation/00-foundation/g1-audit.md` 同位置 (新建 `g3-audit.md` 或追加同文件). 候选 audit 行 (实施时发现的留账):

| 留账项 | 来源 PR | 触发条件 | 处理 Phase |
|--------|---------|---------|-----------|
| CHN-3 作者删 group 路径 lazy 90d GC cron | #410 + #412 (CHN-3.1+3.2) | layout 表无 ON DELETE CASCADE, 作者删 group 不阻塞但留 layout 行孤儿 | Phase 4+ cron job |
| AL-4 hermes plugin 占号 (v1 仅 openclaw) | #398 (AL-4.1) | process_kind enum 'hermes' 占号但 v1 不实施, v2+ 加 | Phase 5+ |
| CV-4 iterate retry 路径 (failed 不复用 iteration_id) | #405 + #409 (CV-4.1+4.2) | failed 态 owner 重新触发 = 新 iteration_id, 不复用 failed 行 (#380 ⑦) | 永不实施 (反约束) |
| AL-3.3 ADM-2 god-mode 元数据 (REG-AL3-011 ⚪) | #324/#327 | client dot DOM 已 ✅, admin god-mode 段元数据走 ADM-2 | Phase 4+ ADM-2 milestone |
| CHN-1 AP-1 严格 403 (REG-CHN1-007 ⏸️) | #286 | 当前 AP-0 grants member (*,*) 仅 guard 5xx, AP-1 落时 flip 改断 status==403 | Phase 4 AP-1 milestone |
| 第 6 轮 remote-agent 安全 (蓝图 §4) | AL-4 spec | 二进制下载/沙箱/资源限制/uninstall 留第 6 轮, 不挡 G3 闭合 | Phase 6+ |

### 5.2 audit 入册位置

挂 `docs/implementation/00-foundation/g1-audit.md` Phase 3 段, 跟 G0/G1 audit 同模式 (留账行 + Phase 4+ 接续指针, 不挡 G3 退出公告).

---

## 6. 闭锁清单 (签字前 final check)

战马 prep 完成 ✅, 三签前 final check (野马/烈马/飞马):

- [x] G3.1 artifact + RT-1 推送 E2E — phase-3-readiness-review.md:14 已 ✅ SIGNED (#348 e2e ≤3s 真 WS push)
- [x] G3.2 锚点对话 E2E — implementation 100% 闭 (#359/#360/#404/#421), 烈马 acceptance signoff 待补 (cv-2.md 已全 ✅)
- [x] G3.3 ⭐ CV-1 用户感知签字 — 已 ✅ SIGNED (#403 野马, 5/5 验收通过)
- [x] G3.4 协作场骨架 demo — implementation 100% 闭 (#411/#423/#428), 三签依据齐:
   - 战马 e2e ✅ (chn-4-collab-skeleton.spec.ts PASS)
   - 烈马 acceptance ✅ (chn-4.md §1-§4 全 ✅, #428 closure)
   - 野马 双 tab 截屏文案锁验 ⏸️ 待补 (3 张 g3.4-cv2-* + 2 张 g3.4-chn4-* 待 page.screenshot 入 git, 已 landed: g3.4-cv3-markdown / g3.4-cv4-iterate-pending / g3.4-cv4-iterate-error-baseline)
- [ ] G3.audit — 飞马轻量 prep 留账行 6 项 (上节 §5.1)

**结论**: 实施 100% 闭, 待 (a) 烈马补 G3.2 acceptance signoff doc + (b) 野马补 G3.4 双 tab 截屏 (5 张 ⏸️ 走 follow-up PR `page.screenshot()` 入 git) + (c) 飞马补 G3.audit 留账行 → Phase 3 退出公告就位.

---

## 7. 跟历史闸 same-pattern 引用

跟 G0.5 / G1-exit / G2-exit 同模式拼装:

- **G0.5 evidence**: `docs/evidence/g0.5/README.md` (烈马 QA + 战马 实施)
- **G1 exit gate**: `docs/qa/signoffs/g1-exit-gate.md` (Phase 1 全签)
- **G2 exit gate** (烈马 QA): `docs/qa/signoffs/g2-exit-gate-liema-signoff.md`
- **G2 demo signoffs**: G2.4 #275 / G2.5 #277 / G2.6 #274+#280 (截屏后置补 + acceptance + 立场反查 三联签)
- **G3.3 CV-1 signoff** (野马 PM): `docs/qa/signoffs/g3.3-cv1-yema-signoff.md` (#403)

---

## 8. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 战马A | v0 — G3 退出闸 evidence bundle 战马 prep 完毕. 4 闸 + audit 全锚 PR/SHA + acceptance + 测试名 byte-identical + 截屏路径锁 (#391 §1 同源). G3.1/G3.2/G3.4 implementation 100% 闭, G3.3 已 ✅ SIGNED #403 野马. 待签: 烈马 G3.2 acceptance signoff doc / 野马 G3.4 双 tab 5 张 ⏸️ screenshot follow-up PR / 飞马 G3.audit 留账 6 项. 跟 G0.5/G1-exit/G2-exit/G3.3 同模式拼装, 防 evidence 散落漏查. |
