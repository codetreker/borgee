# Phase 2 退出公告 — 条件性全过

> 烈马 · 2026-04-28 · 草稿 v1
> 受众: dev / QA / 内部 review (业主感知公告走野马 `../implementation/00-foundation/phase-2-exit-summary.md` 那条, 不重复)
> 配套: 飞马 readiness review **PR #267** (`phase-2-readiness-review.md`, 6 ✅ + 2 留账 ✅ READY 判定)
> 配套: 烈马 QA 联签 `signoffs/g2-exit-gate-liema-signoff.md` + 野马 PM 立场签 `signoffs/phase-2-yema-stance-signoff.md`
> 状态锚: `phase-2-gate-status.md` v3 (#251) 看板实时 + `phase-2-closing-checklist.md` (#257)

## 1. 总判定: ✅ **条件性全过** (飞马 PR #267 拍板)

5+1 严格闸 SIGNED + 3 条件性闸按 #248 condition signoff 接受 + 2 留账闸全部挂 Phase 4 PR 编号 → 4 角色联签后 team-lead 宣布 Phase 2 关闭。

## 2. SIGNED — 5+1 严格闸 ✅

| 闸 | 闭合 PR | 证据 / Reg | 签字 |
|---|---|---|---|
| **G2.0** ADM-0 cookie 串扰反向断言 | #197 + #201 + **#223** | REG-ADM0-001..010 + AUD-G2-ADM01/02/03 | 烈马 ✅ |
| **G2.3** 节流不变量 T1-T5 单测 | #221 + **#236** | AUD-G2-G23 (二维 key + clock 注入 + 边界 `>=`) | 烈马 ✅ |
| **G2.6** /ws ↔ BPP schema 注释锁 | **#237** godoc | REG-RT0-003 grep `BPP-1.*byte-identical` count≥1 | 飞马 ✅ |
| **G2.audit** Phase 2 codedebt audit v2 | #212 + #231 + **#244** + **#251** | 11 行 audit + AUD-G2-AL1A (#252) | 飞马/烈马 ✅ |
| **AL-1a** runtime 三态 + reason codes | **#249** + #250 速读卡 + #252 REG | REG-AL1A-001..005 | 烈马 ✅ |

## 3. 条件性闸 — 3 项 (按 #248 condition signoff)

| 闸 | 当前 | 闭合路径 | 留账引 |
|---|---|---|---|
| **G2.1** 邀请审批 E2E | 🟡 server ✅ / e2e `.skip` | **#237 + #239** 战马B 解 `.skip` (rate-limit bypass) | `acceptance-templates/cm-onboarding.md` REG-CMO-006/007 |
| **G2.2** 离线 fallback E2E | 🟡 partial | **#237 + #239** RT-0 client e2e + presence stub | `acceptance-templates/rt-0.md` REG-RT0-007 (60s fallback) |
| **G2.4** 用户感知签字 ⭐ | 🟡 5/6 (#275) | 野马补签 #2 (AL-1b 后) + #3/#4 (e2e 后) + #6 (ADM-1 后) | `acceptance-templates/al-1b.md` + `acceptance-templates/adm-1.md` + `g2.4-demo-signoff.md` |

## 4. 留账闸 — 2 项 (Phase 4 PR 编号锁, 规则 6)

| 闸 | 留账 PR 路径 | 锚 acceptance-templates |
|---|---|---|
| **G2.5** presence 接口契约 | `internal/presence/contract.go` AL-3 同期 PR **#277** (战马A AL-3 占号) — 路径锁 + 符号锁 (`PresenceTracker.IsOnline` + `Sessions`) | `acceptance-templates/al-2a.md` (config SSOT, AL-3 前置) + AL-3 留账行 |
| **G2.6** /ws ↔ BPP schema CI lint | ✅ **DONE** — **#304** merged 2026-04-28 13:35:02Z (commit `4724efa`); `bpp/frame_schemas.go` whitelist + `bpp/frame_schemas_test.go` 6 reflection subtest + `bpp/schema_equivalence_test.go` 2 dispatcher subtest + CI workflow lint job | `acceptance-templates/al-2a.md` §蓝图行为对照 BPP-1 envelope CI lint 真落行 + REG-BPP1-001..008 全 🟢 |

> ⚠️ Phase 3 RT-1 ArtifactUpdated frame 必须套 #237 envelope 直到 BPP-1 CI lint 落 (飞马 #267 §5 强约束)。
>
> 📌 **Phase 3 进展更新 (2026-04-28)**: **RT-1 三段全闭** ✅ — RT-1.1 server cursor (#290) + RT-1.2 client backfill (#292) + RT-1.3 agent BPP session.resume (#296) 都 merged。RT-1 提前完成 → Phase 4 留账压力降 (G2.5 仍需 AL-3 实施落地, G2.6 仍需 BPP-1 envelope CI lint 实施 PR; **不变**: 这两条都不靠 RT-1 闭环达成)。RT-1.3 session.resume frame 已落 byte-identical 锁 (`session_resume_test.go::TestSessionResumeFrameFieldOrder`), 跟 RT-1.1 ArtifactUpdated envelope 同模式 — 给 BPP-1 envelope CI lint 落地时多一份参考样本。

## 5. Phase 4 acceptance template 索引 (本周新增 + 链)

`docs/qa/acceptance-templates/`:
- **AL-2a** (config SSOT 表 + update API) — **PR #264** (本周新增, 7 验收)
- **ADM-2** (分层透明 audit) — **PR #266** (本周新增, 7 验收)
- **AL-1b** (busy/idle, BPP 同期) — 留 Phase 4
- **ADM-1** (隐私承诺页实施) — 留 Phase 4 (文案 3 条已锁)

## 6. 后续动作 (3 天路线)

1. 战马B 解 `cm-4-realtime.spec.ts` `.skip` (#239) → G2.1/G2.2 真闭
2. 战马A `internal/presence/contract.go` 路径锁 PR (AL-3 同期) → G2.5 留账行确认
3. 烈马 G2.audit v3 补 (presence / 节流 / CHECK enforcement) → DRAFT → SIGNED
4. 野马 G2.4 demo #2/#3/#4/#6 补签 → 5/6
5. 飞马 G2.6 CI lint PR (BPP-1 同 PR) → Phase 4 准备齐全

## 7. 4 角色联签位

> 全部 PARTIAL 项满足 #248 condition + 留账闸挂 Phase 4 PR 编号后, 4 角色逐一签:

- [x] 飞马 (architecture / 闸 1+2): ✅ 拍板 — 引 PR #267 readiness review §4 (5+1 严格闸 SIGNED + 条件性 3 闸 #248 condition + 留账 2 闸挂 Phase 4 PR 编号: AL-3 / BPP-1 / ADM-1) + §5 Phase 3 entry 唯一冲突点 (RT-1 envelope 套 #237, 已 PR #269 spec 守门) (date: 2026-04-28)
- [x] 战马A (实施 / 闸 3): **战马A** (date: 2026-04-28) — 实施侧 acceptance 全挂闸: ADM-0.1/0.2/0.3 + CM-onboarding + AP-0-bis + CM-3 + RT-0 client+server + AL-1a 三态 + INFRA-2 全 merged。留账: G2.5 = AL-3 占号 **PR #277** (`internal/presence/contract.go` 路径+符号锁); CHN-1.1 schema in-flight **#276** (Phase 3 first wave, 不阻 Phase 2 闭)。
- [x] **野马** (PM / 闸 4 G2.4): **野马 ✅** (date: 2026-04-28) — 立场 OK + 反约束守住, 锚:
  - PR #263 Phase 3 立场反查 (CHN-1 / CV-1 / RT-1) — Phase 3 入口立场齐全, 跟 Phase 2 退出无漂移
  - PR #211 ADM-0 立场反查表 — admin 隐藏 / impersonation 红色横幅 / 受影响者 system message / god-mode 不返回内容 4 项闸 4 红线无漂移
  - PR #228 ADM-1 隐私承诺 — 3 条承诺文案锁住, 配 ADM-0 §1.4 红线 ③ fanout 模式
  - CM-3 资源归属反查 (#208) — org_id 直查闭合, 跨 org 403 反断言全绿
  - G2.4 demo 用户感知 4/6 → 接受条件性 (#2/#3/#4/#6 留 Phase 4 acceptance template AL-1b/ADM-1 同期补签, 不阻塞 Phase 2 退出)
- [x] **烈马** (QA / 闸 4 acceptance): **烈马 ✅** (date: 2026-04-28)
  - acceptance 全可机器化: `docs/qa/acceptance-templates/` AL-2a (#264) + ADM-2 (#266) 各 7 验收, 加上历史模板 (al-1b / adm-1 / cm-onboarding / adm-0 / al-1a) 全套 — 每行 unit / E2E / CI grep / reverse-grep / 反射扫描 实施方式锁
  - regression-registry 全 active 53 行 (#270 翻牌: REG-RT0-008 ⚪→🟢 + 新增 REG-RT0-009 rate-limit bypass 5-row matrix; RT-0 8→9, total 64→65)
  - 留账闸 Phase 4 PR 编号锁占位齐全: G2.5 → `internal/presence/contract.go` AL-3 同期 PR (战马A) / G2.6 CI lint → BPP-1 PR (飞马) — 规则 6 红线满足
  - PARTIAL 项 #248 condition signoff 形式齐: G2.1/G2.2 (#237+#239) / G2.4 4/6 (野马补签路径已锚 al-1b.md + adm-1.md + g2.4-demo-signoff.md)
  - SIGNED 5+1 闸证据全部回链 PR + REG: G2.0 #223 / G2.3 #236 / G2.6 注释锁 #237 / G2.audit #244+#251 / AL-1a #249+#252

> team-lead (建军) 在 4 联签齐后宣布 Phase 2 关闭, 同步触发 `../implementation/00-foundation/phase-2-exit-summary.md` 业主公告发布。

## 8. 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v0 草稿 — 4 SIGNED + 3 PARTIAL + 2 DEFERRED |
| 2026-04-28 | 烈马 | v1 — 顶部加飞马 #267 readiness review 引用; title 锁 "条件性全过"; SIGNED 升 5+1 (加 AL-1a #249); 留账行链 al-2a.md / adm-2.md / al-1b.md / adm-1.md |
| 2026-04-28 | 烈马 | v2 — 锁 deferred PR #: G2.5→#277 (战马A AL-3 占号), G2.6→#274 (飞马 BPP-1 占号), G2.4 PARTIAL 升 5/6 引 #275 (野马 G2.4 #5) |
| 2026-04-28 | 飞马 | §7 飞马联签 ✅ — 引 #267 readiness §4/§5 + #269 RT-1 envelope 守门 |
| 2026-04-28 | 烈马 | v1.1 — §7 烈马 QA 联签 ✅ (acceptance 机器化 + #270 翻牌 + 留账 PR 编号锁) |
| 2026-04-28 | 野马 | v1.2 — PM 联签落 (§7 野马签字位), 引 #263 / #211 / #228 / CM-3 锚点, 立场无漂移 |
| 2026-04-28 | 战马A | v1.3 — §7 战马A (实施 / 闸 3) 联签位填上 (date 2026-04-28); G2.5 留账行 PR # = #277 锁定 |
| 2026-04-28 | 建军 | v2.0 — §9 closure 宣布 (Phase 2 正式关闭) |
| 2026-04-28 | 烈马 | v2.1 — §4 加 RT-1 三段全闭备注 (#290+#292+#296), Phase 4 留账压力降; G2.5/G2.6 不靠 RT-1 闭环达成 (明确区分 — RT-1 是 realtime 主线, AL-3 是 presence 接口契约, BPP-1 是 envelope CI lint, 三者互不替代) |
| 2026-04-28 | 烈马 | v2.2 — §4 G2.6 留账行 ⏸️ → ✅ **DONE** (引 #304 merged commit `4724efa`); BPP-1 envelope CI lint 真落 → CV-1 v1 transition 三条件 (RT-1 ✅ + AL-3 ✅ + BPP-1 lint ✅) 全部满足 |

## §9. 关闭宣布 (建军, 2026-04-28)

Phase 2 (协作闭环 ⭐) 4 角色联签齐 + 5+1 严格闸 SIGNED + 3 PARTIAL #248 condition signoff + 2 DEFERRED 挂 Phase 4 PR #277 (AL-3) / #274+#280 (BPP-1) — **条件性全过, 正式关闭**。

- 联签: 飞马 #271 / 烈马 #272 / 野马 #273 / 战马A #279
- 留账: G2.4 5/6 #275 (待 #6 ADM-1 demo 跑落补到 6/6) / G2.5 AL-3 stub #277 / G2.6 BPP-1 envelope #274+#280
- Phase 3 entry 解封: CHN-1 (#265 拆段 + #283 spec + #276 schema), CV-1 (#282 立场), RT-1 (#269 spec) 三主线 ready
- Phase 3 唯一冲突点 (飞马 #267 §5): RT-1 ArtifactUpdated frame 必须套 #237 envelope 直到 BPP-1 CI lint 真落 (走飞马 review 闸位人工 lint)

下一步: Phase 3 启动 (CHN-1.2 战马A in-flight)。
