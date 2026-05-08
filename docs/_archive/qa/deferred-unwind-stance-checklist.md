# DEFERRED-UNWIND stance checklist — Phase 4 deferred 5 项收尾 (server-only)

> 7 立场 byte-identical 跟 deferred-unwind-spec.md (飞马待 commit). **真兑现 G4.audit P0.1 + 用户铁律 progress_must_be_accurate** — 5 个 Phase 4 deferred 项收口 (HB-4 4.2 / AL-2 wrapper 4.2 / AL-1 dispatcher wire / AL-2b plugin read loop / AL-1 Phase 4 follow-up validReasons). 真有 prod code (5 处 wire-up 真启) 但 0 schema / 0 endpoint shape 改. content-lock 不需 (server-only). 跟 user memory `progress_must_be_accurate` + `strict_one_milestone_one_pr` 铁律承袭.

## 1. HB-4 4.2 真兑现 (deferred 3 截屏 PM 必修 #2)
- [ ] 五支柱状态页截屏 → `docs/qa/signoffs/g4-screenshots/g4-4-hb4-pillars.png` (跟 HB-2 v0(D) e2e milestone 协同, 不重复, 本 PR 仅 wire 已建实施)
- [ ] HB-4 release-gate.yml audit-schema-cross-milestone reflect lint 升 5→6 源 (加 ADM-3, 跟 G4.audit P1.4 协同)

## 2. AL-2 wrapper 4.2 真兑现 (deferred 3 截屏 PM 必修 #2)
- [ ] 5-state UI / error→online 反向链 / busy/idle BPP frame 触发 3 张截屏归档
- [ ] 跟 AL-2 wrapper #482 既有实施 byte-identical 不破 (仅截屏归档 + REG flip)

## 3. AL-1 dispatcher wire 真兑现 (BPP-2.2 frame → audit 自动写)
- [ ] BPP-2.2 task lifecycle frame → AL-1 audit 自动写 wire-up (server.go boot 注入)
- [ ] 跟 AL-1 #492 single-gate AppendAgentStateTransition byte-identical 不破

## 4. AL-2b plugin read loop ack ingress 真兑现 (deferred from #481, BPP-3 后)
- [ ] plugin.go read loop ack ingress wire (BPP-3 #489 PluginFrameDispatcher 真接 AckFrameAdapter)
- [ ] 跟 AL-2b #481 frame schema byte-identical 不破

## 5. REFACTOR-REASONS AL-1 follow-up validReasons (#496 后续)
- [ ] `internal/store/agent_state_log.go::validReasons` 改 reasons.IsValid SSOT 调用 (跟 #496 byte-identical 模式)
- [ ] 反向 grep `validReasons.*map\\[string\\]bool` 在 internal/store/ 0 hit (SSOT 真兑现)

## 6. PROGRESS phase-4.md 概览 [ ]→[x] 9 项翻牌 (G4.audit P0.2 真补)
- [ ] HB-2 / RT-3 / AP-2 / ADM-3 / DL-2 / DL-3 / CS-1 / CS-2 / CS-3 概览 [ ]→[x] (跟实际已合 PR 真状态对齐)
- [ ] 跟 user memory `progress_must_be_accurate` 铁律承袭 (做完即翻牌, stale = 误派活根因)
- [ ] 反向断言: PROGRESS 概览 [ ] count 真降 9

## 7. 0 schema / 0 endpoint shape / 0 client UI 改 + admin god-mode 不挂
- [ ] 反向 grep `migrations/deferred_unwind_` 0 hit + `currentSchemaVersion` 不动
- [ ] 0 endpoint 加 / 0 既有 endpoint shape 改
- [ ] admin god-mode 反向 grep 0 hit (ADM-0 §1.3 红线)

## 反约束 — 真不在范围
- ❌ HB-4 v2 / AL-2 v2 / AL-1 v2 真改实施 (反 0 行为改, 仅 wire-up + 截屏 + REG flip)
- ❌ 加 schema / endpoint / client UI / 加新 CI step
- ❌ admin god-mode 加挂 (永久不挂, ADM-0 §1.3 红线)
- ❌ scope 滑出 5 deferred 项 (反 user memory `strict_one_milestone_one_pr` 铁律)

## 跨 milestone byte-identical 锁链 (5 链)
- HB-4 #509 release gate + HB-2 v0(D) e2e milestone 协同 (5 截屏 #4 真兑现)
- AL-2 wrapper #482 + AL-1 #492 + AL-2b #481 既有实施 byte-identical 不破
- BPP-3 #489 PluginFrameDispatcher AckFrameAdapter 真接 (deferred from #481)
- REFACTOR-REASONS #496 SSOT 模式承袭 (validReasons follow-up)
- user memory `progress_must_be_accurate` 铁律 (PROGRESS 翻牌 9 项)

## PM 拆死决策 (3 段)
- **deferred 收口 vs 改实施拆死** — 仅 wire-up + 截屏 + REG flip + PROGRESS 翻牌 (本 PR), 反 v2 实施改
- **scope 全清 5 项 vs 留尾拆死** — 一次 5 项全闭 (用户铁律, 反 REFACTOR-1 留尾教训)
- **真兑现 vs 文字声明拆死** — 真 wire-up + 真 PNG 截屏归档 (本 PR), 反 文字 deferred 永久拖

## 用户主权红线 (5 项)
- ✅ 真兑现 deferred 5 项 (用户视角"未做"真补)
- ✅ 既有 ACL gate + interface byte-identical 不破
- ✅ PROGRESS 概览准确 (反 stale 误派活根因)
- ✅ 0 schema / 0 endpoint shape / 0 client UI 改
- ✅ admin god-mode 不挂 (ADM-0 §1.3 红线)

## PR 出来 5 核对疑点
1. 5 deferred 真兑现 — wire-up 真启 + 截屏归档 + REG flip
2. PROGRESS phase-4.md [ ]→[x] 9 项真翻 (反向 grep [ ] count 降 9)
3. 0 schema / 0 endpoint shape 改 (`git diff` 反向断言)
4. 反向 grep `validReasons.*map\\[string\\]bool` 0 hit (SSOT 真兑现)
5. cov ≥85% (#613 gate) + admin grep 0 hit
