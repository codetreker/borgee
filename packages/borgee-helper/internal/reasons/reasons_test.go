// Package reasons — 8-dict byte-identical 反断 (跟 hb-2-spec.md §3.3).
package reasons

import "testing"

func TestHB2_Reason8DictByteIdentical(t *testing.T) {
	t.Parallel()
	want := []Reason{
		"ok",
		"path_outside_grants",
		"grant_expired",
		"grant_not_found",
		"host_exceeds_max_bytes",
		"egress_domain_not_whitelisted",
		"cross_agent_reject",
		"io_failed",
	}
	got := All()
	if len(got) != len(want) {
		t.Fatalf("8-dict len 脱节: got=%d want=%d", len(got), len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("dict[%d] 脱节: got=%q want=%q", i, got[i], w)
		}
	}
}

// TestHB2_NoSeventhDictBleed 反 HB-1 7-dict / AL-1a 6-dict 字典污染 — 三字典分立.
func TestHB2_NoSeventhDictBleed(t *testing.T) {
	t.Parallel()
	forbidden := []Reason{
		// HB-1 install-butler 7-dict 字面 (HB-2 不能复用)
		"manifest_signature_invalid",
		"manifest_not_found",
		"runtime_signature_invalid",
		// AL-1a runtime 6-dict 字面 (HB-2 不能复用)
		"network_unreachable",
		"unknown",
		"rate_limited",
	}
	have := map[Reason]bool{}
	for _, r := range All() {
		have[r] = true
	}
	for _, f := range forbidden {
		if have[f] {
			t.Errorf("HB-2 8-dict 污染: 含禁字面 %q (跟 HB-1/AL-1a 字典分立反向约束冲突)", f)
		}
	}
}
