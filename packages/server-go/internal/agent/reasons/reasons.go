// Package reasons — AL-1a 6 错误原因码 单一来源 (REFACTOR-REASONS).
//
// 背景: AL-1a #249 立的 6 字面错误原因码 (api_key_invalid / quota_exceeded /
// network_unreachable / runtime_crashed / runtime_timeout / unknown) 在
// Phase 2~4 被 8 处单测守护链 byte-identical 复用 (#249 / #305 / #321 / #380 /
// #454 / #458 / #481 / #492). 每处都是 duplicate literal map (validReasons /
// validTaskReasons / validAL1aReasons / validReasons in agent_state_log) 而
// 非 单一来源 import — 第 9/10 处 fork 风险高 (字面脱节 = 8 处单测同时挂).
//
// 原则:
//   ① 单一来源 — 6 const + IsValid() + All() 单包暴露, 字面只改这一处;
//   ② 不增字典语义 — 仅 dedupe, 8 源行为 byte-identical (test 全 PASS);
//   ③ 出处 — `ALL` 切片字面顺序 byte-identical 跟 AL-1a #249 原序锁定;
//   ④ 反向约束 — `validReasons.*=.*map[string]bool` 在 internal/ 0 hit (test
//      enforces); 后续新 milestone 凡需 6 dict reject gate 必 import 此包.
//
// 不在此包:
//   - state 名 (online/busy/idle/error/offline) — 是 state 不是 reason,
//     已在 AL-1 #492 internal/store/agent_state_log.go::AgentState 锁定;
//   - iteration 专属 reason (runtime_not_registered) — CV-4 stub 字面不
//     在 AL-1a 6-dict 内, 不归此 单一来源 (acceptance §2.5 反向约束).
package reasons

// 6 错误原因码 — AL-1a #249 立, 8 处单测守护链 byte-identical.
//
// 改这里 = 改 8 处单测同时挂 (#249/#305/#321/#380/#454/#458/#481/#492 + 此).
const (
	APIKeyInvalid      = "api_key_invalid"
	QuotaExceeded      = "quota_exceeded"
	NetworkUnreachable = "network_unreachable"
	RuntimeCrashed     = "runtime_crashed"
	RuntimeTimeout     = "runtime_timeout"
	Unknown            = "unknown"
)

// ALL is the canonical 6-字面 ordered list (顺序 byte-identical 跟 AL-1a #249).
//
// 调用方需要枚举 (test table-driven / migration CHECK 反向断言) 用此切片,
// 不再写 inline literal slice.
var ALL = []string{
	APIKeyInvalid,
	QuotaExceeded,
	NetworkUnreachable,
	RuntimeCrashed,
	RuntimeTimeout,
	Unknown,
}

// validSet — 内部 lookup map, 一次构造避免每次 IsValid 调用分配.
var validSet = func() map[string]bool {
	m := make(map[string]bool, len(ALL))
	for _, r := range ALL {
		m[r] = true
	}
	return m
}()

// IsValid — true iff s ∈ AL-1a 6 字典. 替代 8 处 `validReasons[s]` /
// `validTaskReasons[s]` / `validAL1aReasons[s]` map lookup.
//
// 反向约束: 严格 byte-identical match — 大小写脱节 / trim 脱节 全 reject
// (acceptance §状态机 原则 ④ 跨 milestone 同源).
func IsValid(s string) bool {
	return validSet[s]
}

// All returns a copy of the canonical reason list (defensive copy — 调用方
// 不应改 ALL slice 内容; test enumeration 用此 helper 避免 ALL 被改).
func All() []string {
	out := make([]string, len(ALL))
	copy(out, ALL)
	return out
}
