# REFACTOR-REASONS — AL-1a 6 错误原因码 SSOT (一 PR)

> 类型: refactor (无 schema/无 endpoint/无字典语义增减) — 8 源 dedupe 到单包.
> Owner: 战马D 实施 / 烈马 自签 (refactor 不进野马 G4 流, 跟 CM-5 / ADM-1 deferred 同模式)
> Blueprint锚: `agent-lifecycle.md` §2.3 (故障可解释 6 reason 字面)
> 飞马 #492 review flag 3 follow-up.

## 立场

- **① SSOT** — 6 const + `IsValid()` + `All()` 单包暴露, 字面只改这一处;
- **② 不增字典语义** — 仅 dedupe, 8 源行为 byte-identical (`go test ./...` 全 PASS);
- **③ 锚点** — `ALL` 切片字面顺序 byte-identical 跟 AL-1a #249 原序锁;
- **④ 反约束** — `validReasons.*=.*map[string]bool` / `validTaskReasons` / `validAL1aReasons` 在 internal/ 0 hit (production 路径); 后续新 milestone 凡需 6 dict reject gate 必 import 此包;
- **⑤ 不动 test 字面** — test 文件中的裸字面是 byte-identical lock pin 本身 (`agent/state_test.go::TestClassifyProxyError` / `task_lifecycle_test.go` golden JSON / `migrations/{al_4_1,cv_4_1}_test.go` 6-字典断言), 删了等于自我打脸; 单测锁链值在于多源独立断言, 不在 dedupe.

## What this PR does

1. 新建 `packages/server-go/internal/agent/reasons/reasons.go`:
   - 6 const `APIKeyInvalid / QuotaExceeded / NetworkUnreachable / RuntimeCrashed / RuntimeTimeout / Unknown`
   - `var ALL []string` (顺序 byte-identical 跟 AL-1a #249)
   - `IsValid(s string) bool` + `All() []string` (defensive copy)
2. 新建 `packages/server-go/internal/agent/reasons/reasons_test.go` 5 单测:
   - `TestALL_ByteIdentical_AL1a` — 6 字面顺序锁
   - `TestIsValid_AcceptsAL1a6` — 全 accept
   - `TestIsValid_RejectsOutOfDict` — 9 字典外 reject (含大小写漂移 / trim 漂移 / state 名混入 / CV-4 stub `runtime_not_registered`)
   - `TestAll_ReturnsCopy` — 防御性 copy
   - `TestConstants_ExportedNames` — 6 const 字面值锁
3. 4 production 源 import 替换:
   - `internal/agent/state.go` — 6 const → re-export `reasons.*` (既有 import-site 不破)
   - `internal/bpp/task_lifecycle.go` — `validTaskReasons` map → `validTaskReason()` func 调 `reasons.IsValid`
   - `internal/bpp/agent_config_ack_dispatcher.go` — `validAL1aReasons` map → `validAL1aReason()` func 调 `reasons.IsValid`
4. test 字面**不动** (立场 ⑤) — 跨 milestone byte-identical 锁链是多源独立断言, dedupe 后才能反向校验 SSOT 漂移.

## 反约束

- `grep -rn 'map\[string\]bool{.*api_key_invalid' packages/server-go/internal/` count==0 (test 文件外 production 路径)
- 8 处单测锁链 PASS 不变: AL-1a #249 / AL-3 #305 / DM-2 #321 / CV-4 #380 / AL-2a #454 / AL-1b #458 / AL-2b #481 / AL-1 #492
- `go test ./...` 全 PASS — 无行为级 regression
- 新代码请直接 `import "borgee-server/internal/agent/reasons"` (不走 `internal/agent.Reason*` re-export, 后续 deprecation 留余地)

## REG-RR-001..005 (acceptance template)

| ID | 锚点 | Evidence |
|---|---|---|
| REG-RR-001 | reasons SSOT 6 字面 byte-identical | `reasons_test.go::TestALL_ByteIdentical_AL1a` (顺序锁 + 字面锁) |
| REG-RR-002 | IsValid 字典外严 reject | `_RejectsOutOfDict` 9 case (含 state 名混入 / CV-4 stub 反向) |
| REG-RR-003 | const 字面 byte-identical | `_ExportedNames` 6 case + 反 trim |
| REG-RR-004 | bpp/task_lifecycle.go + agent_config_ack_dispatcher.go 走 SSOT | `go test ./internal/bpp/` 全 PASS (既有锁链不破) |
| REG-RR-005 | 反向 grep production map[string]bool 6 字典 0 hit | CI grep |

## Follow-up 留账 (非阻 PR merge)

- AL-1 #492 merge 后, `internal/store/agent_state_log.go::validReasons` 同样改走 SSOT (本 PR baseline 是 main, AL-1 在 feat/al-1 stacked, merge 顺序为 #492 → 本 PR 的 follow-up commit)
- client SPA `lib/agent-reasons.ts` (TODO) — 客户端文案 SSOT, 跟 server SSOT 跨层 byte-identical lock (留 future refactor)
- deprecation: `internal/agent.Reason*` re-export 标 // Deprecated, 后续 milestone 全切到 `reasons.*` 直接 import

## 退出条件

- `go test ./...` 全绿
- 反向 grep production 0 hit
- 烈马自签 (refactor 不进野马 G4 流)
- REG-RR-001..005 5 🟢
