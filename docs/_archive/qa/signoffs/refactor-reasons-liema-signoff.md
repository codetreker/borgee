# REFACTOR-REASONS AL-1a 6 错误原因码 SSOT — 烈马 (QA acceptance) signoff

> **状态**: ✅ **SIGNED** (烈马 acceptance 代签, 2026-04-29, post-#496 9e7880f / f439fcc)
> **范围**: REFACTOR-REASONS milestone — `internal/agent/reasons` SSOT 包 (6 const + ALL 切片顺序锁 + IsValid + All 防御性 copy) + 4 production 源 import 替换 + 5 单测; 飞马 #492 review flag 3 follow-up — 8 处单测锁链 (#249/#305/#321/#380/#454/#458/#481/#492) duplicate literal map 路径 dedupe 到 SSOT import; type=refactor (无 schema/无 endpoint/无字典语义增减), 用户感知 0 变化 (字典字面 byte-identical)
> **关联**: REFACTOR-REASONS #496 (zhanma-d 9e7880f / f439fcc) 整 PR 一闭; 前置: AL-1a #249 + AL-3 #305 + AL-4 #321 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-2b #481 + AL-1 #492 八源 byte-identical 锁链全 merged; REG-RR-001..005 5🟢 + REG-RR-006..008 3 ⏸️ follow-up; 全套 `go test ./...` PASS 无行为级 regression
> **方法**: 跟 #403 G3.3 + #449 G3.1+G3.2+G3.4 + #459 G4.1 + G4.2 + G4.3 + G4.4 + G4.5 + AL-1 + AP-1 + BPP-3.1 烈马代签机制承袭 — 真单测实施证据 + 立场反查 + acceptance template 闭锁 + 跨 milestone byte-identical 链不破 + 烈马代签 (refactor 工程内部 cleanup, 不进野马 G4 流, 跟 CM-5 / ADM-1 / AL-1 / AP-1 / BPP-3.1 deferred 同模式)

---

## 1. 验收清单 (烈马 acceptance 视角 5 项, 跟 acceptance refactor-reasons.md 6 项 ✅ byte-identical)

| # | 验收项 | 立场锚 | 结果 | 实施证据 (PR/SHA + 测试名 byte-identical) |
|---|--------|--------|------|------|
| ① | **数据契约 — `internal/agent/reasons/reasons.go` SSOT 包** — 6 const (APIKeyInvalid / QuotaExceeded / NetworkUnreachable / RuntimeCrashed / RuntimeTimeout / Unknown) + `ALL` 切片顺序锁 byte-identical 跟 AL-1a #249 + IsValid() + All() 4 export | acceptance §数据契约 + spec §立场 ① + 蓝图 §2.3 reason 字面 | ✅ pass | `internal/agent/reasons/reasons_test.go::TestALL_ByteIdentical_AL1a` (顺序 + 字面锁) + `TestConstants_ExportedNames` (6 const 字面值锁 + 反 trim) PASS — REG-RR-001 + acceptance §数据契约 (1 项) |
| ② | **行为不变量 — IsValid() 严 byte-identical match** — 字典外 / 大小写漂移 / trim 漂移 / state 名混入 (online) / CV-4 stub `runtime_not_registered` 反向 全 reject; 6 reason 全 accept; All() 防御性 copy (调用方改返值不影响 ALL slice — a[0]="MUTATED" 后 ALL[0] 仍 "api_key_invalid") | acceptance §行为不变量 + spec §立场 ② IsValid 严 + §立场 ③ defensive copy | ✅ pass | `reasons_test.go::TestIsValid_AcceptsAL1a6` (6 全 accept) + `TestIsValid_RejectsOutOfDict` (9 case reject 含大小写漂移 + trim + state 名混入 online + CV-4 stub `runtime_not_registered` 反向 + typo) + `TestAll_ReturnsCopy` (defensive a[0]="MUTATED" 验) PASS — REG-RR-002 + REG-RR-003 + acceptance §行为不变量 (2 项) |
| ③ | **4 production 源 import 替换走 SSOT** — `internal/agent/state.go` 6 const re-export `reasons.*` + `internal/bpp/task_lifecycle.go::validTaskReason()` + `internal/bpp/agent_config_ack_dispatcher.go::validAL1aReason()` 全调 `reasons.IsValid`; `go test ./...` 全 PASS 无行为级 regression, 8 处单测锁链 byte-identical 全锁住 (#249/#305/#321/#380/#454/#458/#481/#492 不破) | acceptance §行为不变量 4 production 源 + 跨 milestone 八处单测锁链承袭 | ✅ pass | `go test ./internal/{agent,bpp,api,store,migrations,ws}/...` 全 PASS — REG-RR-004 + acceptance §行为不变量 4 production 源 (1 项) |
| ④ | **反约束 grep — production inline 6-字典 map 0 hit** — 反向 grep `validReasons.*=.*map[string]bool\|validTaskReasons.*=.*map\|validAL1aReasons.*=.*map` 在 internal/ (除 reasons SSOT 包 + test) count==0; production 路径已无 inline 6-字典 map (`bpp/task_lifecycle.go` + `bpp/agent_config_ack_dispatcher.go` 均改 `validTaskReason()` / `validAL1aReason()` func wrapper); `agent/state.go` 6 const re-export 不算 inline map | acceptance §反约束 立场 ④ + spec §反约束 dedupe 校验 | ✅ pass | CI grep production 路径 inline 6-字典 map 0 hit (除 reasons SSOT 包自身) — REG-RR-005 + acceptance §反约束 立场 ④ (1 项) |
| ⑤ | **test 字面立场 ⑤ 不动** — `agent/state_test.go::TestClassifyProxyError` + `bpp/task_lifecycle_test.go` golden JSON + `migrations/{al_4_1,cv_4_1}_test.go` 6-字典断言 5 处裸字面保留 (跨 milestone byte-identical lock pin 是多源独立断言, dedupe production 路径后才能反向校验 SSOT 漂移) | acceptance §反约束 立场 ⑤ + spec §立场 ⑤ test 字面立场守 | ✅ pass | test 文件中 5 处裸字面保留 (验证多源独立断言不退化为 SSOT self-check), 立场反查 PASS — acceptance §反约束 立场 ⑤ (1 项) |

**总体**: 5/5 通过 (覆盖 acceptance 6 项 ✅ 含数据契约 + 行为不变量 + 反约束 全节) → ✅ **SIGNED**, REFACTOR-REASONS 一 PR 整闭通过.

---

## 2. 反向断言 (核心立场守门 byte-identical)

REFACTOR-REASONS 三处反向断言全 PASS:

- **production inline map count==0** (CI grep 守): `validReasons.*=.*map[string]bool\|validTaskReasons.*=.*map\|validAL1aReasons.*=.*map` 在 internal/ production 路径 (除 reasons SSOT 包 + test) count==0; 跟 BPP-2 ActionHandler / cm5stance NoBypassEndpoint AST walk / AP-1 HasCapability hardcode 0 hit CI lint 同模式立场守
- **defensive copy 真守 (mutation 不污染 ALL)**: `TestAll_ReturnsCopy` a[0]="MUTATED" 后 ALL[0] 仍 "api_key_invalid" 验证; 反向防御 caller 改返值污染 SSOT slice (Go slice header 共享 backing array 隐患守); 跟 BPP-2 ConfigRevTracker per-agent 独立 tracker state + AL-2a SSOT blob 整体替换 same 立场 (单源不被外部 mutation 污染)
- **state 名混入 reject test 真覆盖**: `TestIsValid_RejectsOutOfDict` 9 case 含 state 名 (online — AL-1a 状态名不是 reason) + CV-4 stub `runtime_not_registered` 反向 + 大小写漂移 + trim 漂移 + typo — 反向防 reason/state 命名空间漂移 (AL-1 wrapper state machine 跟 reason 字典拆死, validator 单门各管各)
- **test 字面立场 ⑤ 不动 (跨 milestone 多源独立断言)**: 5 处裸字面保留 (`agent/state_test.go::TestClassifyProxyError` + `bpp/task_lifecycle_test.go` golden JSON + `migrations/{al_4_1,cv_4_1}_test.go` 6-字典断言), 反向防 dedupe 退化为 SSOT self-check (锁链值在多源独立断言, 任一处漂 = 多处单测同步红)

---

## 3. 跨 milestone byte-identical 链验 (REFACTOR-REASONS 是 reason 八处单测锁链 dedupe 后续守)

REFACTOR-REASONS 兑现/承袭多源 byte-identical:

- **8 处单测锁链 byte-identical 不破** (改字面 = 改 `reasons.ALL` 一处即 8 处单测同步挂): AL-1a #249 (源头 6 reason) → AL-3 #305 → AL-4 #321 → CV-4 #380 → AL-2a #454 → AL-1b #458 → AL-2b/BPP-2 #481/#485 → AL-1 #492 → 此 SSOT (REFACTOR-REASONS dedupe production 路径); `go test ./...` 全 PASS 八处锁链未破
- **SSOT 包跟 ActionHandler / Pusher seam / AppendAgentStateTransition / HasCapability / PermissionDeniedPusher 同精神**: 单 entry + interface seam + 依赖反转 — `internal/agent/reasons` 是 reason 字典 SSOT 单 entry, 跟 BPP-2 ActionHandler (op 派生 SSOT) / AL-2a AgentConfigPusher (config 推送 SSOT) / AL-1 AppendAgentStateTransition (state set SSOT) / AP-1 HasCapability (capability check SSOT) / BPP-3.1 PermissionDeniedPusher (permission denied frame SSOT) 同模式 dedupe duplicate 路径
- **defensive copy 跟 AL-2a SSOT blob 整体替换 + BPP-2 ConfigRevTracker per-agent 独立 tracker state 同立场**: 单源不被外部 mutation 污染
- **forward-only 跟 AL-1 agent_state_log + ADM-2.1 admin_actions + ADM-2.2 impersonation_grants 同精神**: refactor 不删历史字面, 不退化 (deprecation 留 follow-up REG-RR-008 渐进迁移)
- **test 字面跨 milestone 多源独立断言 (锁链 pin 立场 ⑤)**: 跟 reason 八处单测锁链立场承袭 — 多源独立断言才能反向校验 SSOT 漂移, dedupe production 路径不动 test 字面
- **跟 G4 batch / AL-1 / AP-1 / BPP-3.1 烈马代签机制同模式**: refactor 工程内部 cleanup (用户感知 0 变化 — 字典字面 byte-identical), 不进野马 G4 流, 烈马代签

---

## 4. 留账 (REFACTOR-REASONS 闭闸不阻, follow-up — 跟 spec §follow-up 字面承袭)

- ⏸️ **REG-RR-006 AL-1 #492 `internal/store/agent_state_log.go::validReasons` 同改 SSOT** — 本 PR baseline 是 main, AL-1 在 feat/al-1 stacked; merge 顺序 #492 → 本 PR follow-up commit (跟 BPP-3.1 AP-1 wiring 1-line follow-up 同模式 — interface seam ready, 依赖 PR merge 顺序)
- ⏸️ **REG-RR-007 client SPA `lib/agent-reasons.ts` 客户端文案 SSOT** — 跨层 byte-identical lock (server SSOT ↔ client SSOT), 跟 AL-2a allowedConfigKeys ↔ ALLOWED_CONFIG_KEYS + ADM-2 system DM 5 模板 + BPP-3.1 abac.go 403 body ↔ PermissionDeniedFrame 跨层 byte-identical 锁同模式 (future refactor milestone)
- ⏸️ **REG-RR-008 deprecation: `internal/agent.Reason*` re-export 标 // Deprecated** — 后续 milestone 全切到 `reasons.*` 直接 import; 跟 forward-only 立场承袭 (refactor 不删历史字面, 渐进迁移)

---

## 5. 解封路径 + Registry 数学验

**REFACTOR-REASONS 闸通过** (一 PR 整闭, refactor 不进 G4 流):
- ✅ **G4.1 ADM-1**: 野马 ✅ #459
- ✅ **G4.2 ADM-2**: 烈马 ✅ #484 6cf5240
- ✅ **G4.3 BPP-2**: 烈马 ✅ G4 batch
- ✅ **G4.4 CM-5**: 烈马 ✅ G4 batch
- ✅ **G4.5 AL-2a + AL-2b + AL-4 联签**: 烈马 ✅ G4 batch
- ✅ **AL-1 状态四态 wrapper**: 烈马 ✅ #492
- ✅ **BPP-3 plugin 上行 dispatcher**: ✅ #489
- ✅ **AP-1 ABAC SSOT + 严格 403**: 烈马 ✅ #493 d6625b2
- ✅ **BPP-3.1 permission_denied frame**: 烈马 ✅ #494 9c356b4
- ✅ **REFACTOR-REASONS SSOT**: 烈马 acceptance ✅ 本 signoff (5/5 验收 + REG-RR-001..005 5🟢 + 8 处单测锁链不破 + `go test ./...` 全 PASS)

**Registry 数学验 (post-#496 9e7880f / f439fcc)**:
- 总计 238 → **243** (+5 行 REFACTOR-REASONS 全 🟢)
- active 213 → **218** (+5 净)
- pending **25** → **25** (REFACTOR-REASONS 5 行全 🟢 不增 pending)
- 跟 #475 spec brief / G4.audit 飞马 row + AL-1 5🟢 + BPP-2 17🟢 + CM-5 5🟢 + AL-2a 7🟢 + AL-1b 6🟢 + BPP-3.1 6🟢 baseline 累加链

后续:
- ⏸️ **REG-RR-006 AL-1 #492 store 路径 stack follow-up commit** — #492 merge 后接
- ⏸️ **REG-RR-007 client SPA SSOT + REG-RR-008 deprecation** — future refactor milestone
- ⏸️ **G4.audit** Phase 4 代码债 audit (软 gate 飞马职责) — 含 REFACTOR-REASONS 3 follow-up + BPP-3.1 + AP-1 8 项 + AL-1 4 项 + AL-4.2/4.3 5⚪
- ⏸️ **Phase 4 closure announcement** (Phase 4 entry 8/8 全签 ✅ + BPP-3.1 + REFACTOR-REASONS 闭环 + G4.audit 飞马软 gate 链入)

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 烈马 | v0 — REFACTOR-REASONS AL-1a 6 错误原因码 SSOT ✅ SIGNED post-#496 9e7880f / f439fcc (zhanma-d, 飞马 #492 review flag 3 follow-up). 5/5 验收通过 covers acceptance refactor-reasons.md 6 项 ✅: 数据契约 SSOT 包 6 const + ALL 顺序锁 + IsValid + All 4 export / 行为不变量 IsValid 严 byte-identical 9 reject + 6 accept + defensive copy mutation 不污染 / 4 production 源 import 替换 (state.go re-export + bpp/task_lifecycle + agent_config_ack_dispatcher) `go test ./...` 全 PASS / 反约束 production inline 6-字典 map 0 hit CI grep / test 字面立场 ⑤ 不动 (跨 milestone 多源独立断言守 SSOT 漂移校验). 跟 #403 G3.3 / G4.1-G4.5 / AL-1 / AP-1 / BPP-3.1 烈马代签机制同模式 — refactor 工程内部 cleanup (用户感知 0 变化 字典字面 byte-identical) 不进野马 G4 流. 反向断言三处全过 (production inline map count==0 CI grep 守 + defensive copy 真守 mutation 不污染 ALL + state 名混入 reject test 真覆盖 + test 字面立场 ⑤ 不动多源独立断言). 跨 milestone 链全锚 (8 处单测锁链 #249/#305/#321/#380/#454/#458/#481/#492 byte-identical 不破 — 改字面 = 改 reasons.ALL 一处即 8 处单测同步挂 + SSOT 包跟 ActionHandler/Pusher/AppendAgentStateTransition/HasCapability/PermissionDeniedPusher 同精神依赖反转 + defensive copy 跟 AL-2a SSOT blob + BPP-2 ConfigRevTracker per-agent 独立 tracker 同立场 + forward-only 跟 AL-1 agent_state_log + ADM-2 admin_actions + impersonation_grants 同精神 + test 字面跨 milestone 多源独立断言). 留账 3 项 ⏸️ deferred (REG-RR-006 AL-1 #492 store 路径 stack follow-up commit interface seam ready 1-line + REG-RR-007 client SPA SSOT 跨层 byte-identical lock + REG-RR-008 agent.Reason* deprecation 渐进迁移). registry 数学: 238 → 243 (+5 全 🟢), active 213 → 218 (+5 净), pending 25 → 25. 跟新协议 "一 milestone 一 worktree 一 PR" 模式同 (worktree `.worktrees/refactor-reasons`, branch `refactor/agent-reasons-ssot`). |
