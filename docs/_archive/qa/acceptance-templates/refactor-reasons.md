# Acceptance Template — REFACTOR-REASONS: AL-1a 6 错误原因码 SSOT

> 类型: refactor (无 schema/无 endpoint/无字典语义增减)
> 蓝图: `agent-lifecycle.md` §2.3 (故障可解释 6 reason 字面)
> 飞马 #492 review flag 3 follow-up: 8 处单测锁链 (#249/#305/#321/#380/#454/#458/#481/#492) 现是 duplicate literal map 而非 SSOT import, 第 9/10 处 fork 风险高
> Owner: 战马D 实施 / 烈马 自签 (refactor 不进野马 G4 流, 跟 CM-5 / ADM-1 deferred 同模式)

## 拆 PR 顺序 (新协议: 一 milestone 一 PR)

- **REFACTOR-REASONS 一 PR** — SSOT 包 + 4 production 源 import 替换 + 5 单测 + spec brief + 留账 follow-up.

---

## 验收清单

### 数据契约 (蓝图 §2.3 reason 字面)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `internal/agent/reasons/reasons.go` SSOT 包 6 const + `ALL` 切片顺序锁 + `IsValid()` + `All()` | unit | 战马D / 烈马 | ✅ — `reasons_test.go::TestALL_ByteIdentical_AL1a` (顺序 + 字面锁) + `_ExportedNames` (6 const 字面值锁 + 反 trim) |

### 行为不变量 (REFACTOR — dedupe 不增语义)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `IsValid()` 严 byte-identical match — 字典外 / 大小写漂移 / trim 漂移 / state 名混入 / CV-4 stub `runtime_not_registered` 反向 全 reject | unit | 战马D / 烈马 | ✅ — `_RejectsOutOfDict` 9 case PASS |
| `All()` 防御性 copy — 调用方改返值不影响 ALL | unit | 战马D / 烈马 | ✅ — `_ReturnsCopy` PASS (a[0]="MUTATED" 后 ALL[0] 仍 "api_key_invalid") |
| 4 production 源走 SSOT — `internal/agent/state.go` re-export / `internal/bpp/task_lifecycle.go::validTaskReason()` / `internal/bpp/agent_config_ack_dispatcher.go::validAL1aReason()` 全调 `reasons.IsValid` | unit + build | 战马D / 烈马 | ✅ — `go test ./...` 全 PASS (无行为级 regression, 8 处单测锁链 byte-identical 全锁住) |

### 反约束 (立场 ④)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 反向 grep `validReasons.*=.*map[string]bool\|validTaskReasons.*=.*map\|validAL1aReasons.*=.*map` 在 internal/ (除 reasons SSOT 包 + test) count==0 | CI grep | 战马D / 烈马 | ✅ — production 路径已无 inline 6-字典 map (`bpp/task_lifecycle.go` + `bpp/agent_config_ack_dispatcher.go` 均改 `validTaskReason()` / `validAL1aReason()` func wrapper); `agent/state.go` 6 const re-export 不算 inline map |
| test 字面**不动** — `agent/state_test.go::TestClassifyProxyError` / `bpp/task_lifecycle_test.go` golden JSON / `migrations/{al_4_1,cv_4_1}_test.go` 6-字典断言 是跨 milestone byte-identical lock pin 本身 | 立场反查 | 战马D / 烈马 | ✅ — test 文件中 5 处裸字面保留 (单测锁链值在多源独立断言, dedupe production 路径后才能反向校验 SSOT 漂移) |

### 退出条件

- 上表 6 项: **6 ✅** (全绿)
- `go test ./...` 全 PASS (无行为级 regression)
- 反向 grep production 0 hit (除 reasons SSOT 包自身)
- 烈马自签 (refactor 不进野马 G4 流)
- 登记 `docs/qa/regression-registry.md` REG-RR-001..005 (5 🟢 active)
- ⚠️ REFACTOR-REASONS 是工程内部 cleanup — 用户感知 0 变化 (字典字面 byte-identical), 不进 G4 签字流, 烈马代签

### Follow-up 留账 (非阻 PR merge)

- AL-1 #492 `internal/store/agent_state_log.go::validReasons` 同样改走 SSOT (本 PR baseline 是 main, AL-1 在 feat/al-1 stacked, merge 顺序 #492 → 本 PR follow-up commit)
- client SPA `lib/agent-reasons.ts` (TODO) — 客户端文案 SSOT, 跟 server SSOT 跨层 byte-identical lock (留 future refactor)
- deprecation: `internal/agent.Reason*` re-export 标 // Deprecated, 后续 milestone 全切到 `reasons.*` 直接 import

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — REFACTOR-REASONS 一 PR 整闭: SSOT 包 + 5 单测 + 4 production 源 import 替换 + spec brief 62 行 + REG-RR-001..005 5🟢; 飞马 #492 review flag 3 follow-up; AL-1 #492 store 路径留 follow-up commit 待 #492 merge 后 stack |
