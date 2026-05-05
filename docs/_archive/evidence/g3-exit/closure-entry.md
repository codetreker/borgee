# G3 退出闸 closure 收口 — 三角色 follow-up entry point

> 作者: 战马A · 2026-04-29 · team-lead 派 G3 closure 收口 entry
> 目的: G3 退出闸三签 pending 收尾 — **整合 entry point**, 不抢三角色活, 给野马 / 烈马 / 飞马 三 follow-up PR 各自留独立入口 + 完成判据.
> 形式: 此文件 = 收口入口表; 不含具体内容 (三角色各自 PR fill); G3.audit skeleton 走 #443 (已 merged 2de482f), G3 evidence bundle 走 #442 (已 merged f71e26f).
> 锚: `docs/evidence/g3-exit/README.md` (#442) + `docs/implementation/00-foundation/g3-audit.md` (#443) + `docs/qa/signoffs/g3.3-cv1-yema-signoff.md` (#403 ✅ SIGNED).

---

## G3 退出闸 status (cut: 2026-04-29)

| 闸 | Status | 责任角色 | follow-up 入口 |
|---|---|---|---|
| G3.1 artifact + RT-1 推送 E2E | ✅ READY | (战马 e2e PASS) | 无 (已闭) |
| G3.2 锚点对话 E2E | ✅ READY (acceptance signoff doc ⏸️) | **烈马** | §1 |
| G3.3 ⭐ 用户感知签字 (CV-1) | ✅ **SIGNED** (#403 野马 PM 5/5, 2026-04-29) | (野马 已签) | 无 (已闭) |
| G3.4 协作场骨架 (CHN-4) E2E + 双 tab 截屏 | ✅ READY (5 张 ⏸️ 截屏 follow-up) | **野马** | §2 |
| G3.audit 跨 milestone 留账 6 项 | 🟡 DRAFT (#443 skeleton, v1 fill ⏸️) | **飞马** | §3 |

通过判据 (G2 模式同根): G3.1 ✅ + G3.2 ✅ + signoff doc + G3.3 ✅ SIGNED + G3.4 ✅ + 5 截屏 + G3.audit v1 fill → 飞马 G3 closure announcement.

---

## §1 烈马 G3.2 acceptance signoff doc

- **入口文件**: `docs/qa/signoffs/g3.2-cv2-liema-signoff.md` (新, 跟 #403 yema-signoff 同模式 byte-identical 结构)
- **期望 PR 命名**: `docs(qa): G3.2 烈马 acceptance signoff — 锚点对话 E2E ✅`
- **PR 内容**:
  - acceptance template `cv-2.md` §1-§5 全 ✅ 状态承认 (基于 #358 spec + #404 client + #421 REG 5 行 🟢)
  - 锚 `cv-2-3-anchor-client.spec.ts` 4 cases PASS + 4 文案锁 byte-identical (`#355 ① 评论此段 / ② 段落讨论 / ③ 针对此段写下你的 review… / ④ 标为已解决`)
  - AnchorCommentAddedFrame 10 字段 byte-identical 跟 RT-1.1 (#290) / DM-2.2 (#372) / CV-4.2 (#409) 共 cursor sequence 锁承认
  - 反约束承认: agent 视角 0 入口 (`anchor.create_owner_only` 双层防御) + WS ≤3s 契约
- **行数**: ≤120 行 (跟 #403 yema-signoff 同 budget)
- **锚**: #442 evidence §2 + #443 audit §2.2 + #421 REG-CV2 + #404 client SPA
- **完成判据**: 烈马签字 + cv-2.md §1-§5 evidence 锚每项 ≥1 PR/SHA + acceptance template `cv-2.md` 状态行 "G3.2 ⏸️" → "G3.2 ✅ #N (烈马)"

## §2 野马 G3.4 5 张截屏 follow-up

- **入口文件**: `packages/e2e/tests/chn-4-screenshots-followup.spec.ts` (新, 跟 #422/#423 G3.x screenshot 同模式 `page.screenshot()` 入 git)
- **期望 PR 命名**: `test(chn-4): G3.4 5 张截屏 follow-up — 野马 双 tab + 边界态文案锁`
- **PR 内容**:
  - 5 张 ⏸️ 截屏入 `docs/qa/screenshots/g3.4-*` (跟 #439 §1 锁路径同源 byte-identical)
  - 跟 G2.4 #275 同模式 — 截屏 owner = 野马, page.screenshot() Playwright 真截 (反 PS 修改)
  - 双 tab 文案锁字面验承认: `聊天` / `工作区` byte-identical (#382 §1 ④ + #364 + #371 + #374 + #378 ④ 7 源同根)
- **行数**: ≤200 行 (e2e spec 含 5 case + screenshot path 锁)
- **锚**: #442 evidence §4 + #439 路径锁 + #428 closure follow-up + 已 landed 3 张 (g3.4-cv3-markdown / g3.4-cv4-iterate-pending / g3.4-cv4-iterate-error-baseline)
- **完成判据**: 5 张 PNG 入 git + acceptance template `chn-4.md` 状态行 "G3.4 双截屏 ⏸️" → "G3.4 双截屏 ✅ #N (野马)"

## §3 飞马 G3.audit v1 fill

- **入口文件**: `docs/implementation/00-foundation/g3-audit.md` (修, 在 #443 skeleton 上填 §2 闸闭合详情 v1 + §3 audit row 6 项 v1)
- **期望 PR 命名**: `docs(impl): G3.audit v1 fill — 飞马 闸闭合详情 + 6 项 audit row 触发条件`
- **PR 内容**:
  - §2.1-§2.4 飞马 v1 fill 闸闭合摘要 + 跨 milestone byte-identical 链承认 (TBD 标记 → v1 真值)
  - §3 audit row 6 项 (A1 CHN-3 GC / A2 AL-4 hermes / A3 CV-4 retry 永不实施 / A4 AL-3.3 ADM-2 / A5 CHN-1 AP-1 / A6 第 6 轮安全) 触发条件 SQL/migration 路径 + 验收锚
  - §1 闸概览状态承认 (G3.audit DRAFT → ✅) + §5 changelog 加 v1 fill 行
- **行数**: ≤150 行 (在 #443 skeleton 99 行基础 +50 v1 fill)
- **锚**: #443 G3.audit skeleton 2de482f + #442 evidence + 跨 milestone 实施 PR (CHN-3 #410-#415 / AL-4 #398-#417 / CV-4 #405-#416 / AL-3.3 #324-#327 / CHN-1 #286 / 蓝图 §4)
- **完成判据**: §2 全 4 闸 v1 fill 落 + §3 6 项触发条件全填 + §1 G3.audit ✅ + 飞马 G3 closure announcement 触发

---

## 完成全闭判据

G3 退出闸全过 = §1 烈马 signoff PR + §2 野马 5 截屏 PR + §3 飞马 v1 fill PR → 飞马 G3 closure announcement (`docs/qa/phase-3-exit-announcement.md`, 跟 G1/G2 closure 同模式) → Phase 4 启动.

---

## 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 战马A | v0 — G3 closure 收口 entry 创建. 三角色 follow-up 入口表 (烈马 G3.2 signoff / 野马 G3.4 5 截屏 / 飞马 G3.audit v1 fill), 各项含入口文件 path + 期望 PR + 行数 budget + 锚 + 完成判据. 跟 #442 evidence + #443 audit skeleton 双轨, 不抢三角色活仅留 entry point. G3.3 ✅ SIGNED #403 + G3.1/3.2/3.4 ✅ READY 状态承认. |
