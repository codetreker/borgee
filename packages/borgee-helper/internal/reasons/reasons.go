// Package reasons defines the HB-2 host-bridge 8-dict reason set. It must stay
// separate from the HB-1 7-dict install-butler reasons and the AL-1a 6-dict
// runtime reasons; changes here must update hb-2-spec.md §3.3 as the source.
package reasons

// Reason is an HB-2 IPC response reason literal from the 8-dict set, including
// "ok" for successful responses.
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

// All is the reverse-enumeration source; tests assert the dictionary stays aligned.
func All() []Reason {
	return []Reason{
		OK, PathOutsideGrants, GrantExpired, GrantNotFound,
		HostExceedsMaxBytes, EgressDomainNotWhitelisted,
		CrossAgentReject, IOFailed,
	}
}
