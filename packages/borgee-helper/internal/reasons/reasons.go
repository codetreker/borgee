// Package reasons — HB-2 host-bridge 8-dict (跟 HB-1 7-dict + AL-1a 6-dict
// 字典分立, 不混; 改一处 = 改 spec hb-2-spec.md §3.3 单一来源).
package reasons

// Reason 是 HB-2 IPC response 8-dict 字面 (含 "ok" 成功态).
type Reason string

// 8-dict (hb-2-spec.md §3.3 byte-identical).
const (
	OK                          Reason = "ok"
	PathOutsideGrants           Reason = "path_outside_grants"
	GrantExpired                Reason = "grant_expired"
	GrantNotFound               Reason = "grant_not_found"
	HostExceedsMaxBytes         Reason = "host_exceeds_max_bytes"
	EgressDomainNotWhitelisted  Reason = "egress_domain_not_whitelisted"
	CrossAgentReject            Reason = "cross_agent_reject"
	IOFailed                    Reason = "io_failed"
)

// All 反向枚举出处 — 单测断言字典不脱节.
func All() []Reason {
	return []Reason{
		OK, PathOutsideGrants, GrantExpired, GrantNotFound,
		HostExceedsMaxBytes, EgressDomainNotWhitelisted,
		CrossAgentReject, IOFailed,
	}
}
