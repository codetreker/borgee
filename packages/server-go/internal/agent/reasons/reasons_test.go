package reasons

import (
	"strings"
	"testing"
)

// TestALL_ByteIdentical_AL1a — 锁 6 字面顺序 byte-identical 跟 AL-1a #249.
// 改顺序 / 改字面 = 改 8 处单测同时挂 (#249/#305/#321/#380/#454/#458/#481/#492 + 此).
func TestALL_ByteIdentical_AL1a(t *testing.T) {
	want := []string{
		"api_key_invalid",
		"quota_exceeded",
		"network_unreachable",
		"runtime_crashed",
		"runtime_timeout",
		"unknown",
	}
	if len(ALL) != len(want) {
		t.Fatalf("ALL len=%d want=%d", len(ALL), len(want))
	}
	for i := range want {
		if ALL[i] != want[i] {
			t.Errorf("ALL[%d]=%q want=%q (字面/顺序漂移 — AL-1a #249 锁链断)", i, ALL[i], want[i])
		}
	}
}

// TestIsValid_AcceptsAL1a6 — 6 字面全 accept.
func TestIsValid_AcceptsAL1a6(t *testing.T) {
	for _, r := range ALL {
		if !IsValid(r) {
			t.Errorf("IsValid(%q) = false, want true", r)
		}
	}
}

// TestIsValid_RejectsOutOfDict — 字典外 / 大小写漂移 / trim 漂移 全 reject.
func TestIsValid_RejectsOutOfDict(t *testing.T) {
	bads := []string{
		"",
		"API_KEY_INVALID",   // 大写
		" api_key_invalid",  // leading space
		"api_key_invalid ",  // trailing space
		"api-key-invalid",   // dash
		"apikey_invalid",    // typo
		"online",            // state 名 (不是 reason)
		"runtime_not_registered", // CV-4 stub (不在 6 dict)
		"unknown_reason",    // typo
	}
	for _, b := range bads {
		if IsValid(b) {
			t.Errorf("IsValid(%q) = true, want false (字典外应 reject)", b)
		}
	}
}

// TestAll_ReturnsCopy — All() 返回防御性 copy, 改不影响 ALL.
func TestAll_ReturnsCopy(t *testing.T) {
	a := All()
	if len(a) != len(ALL) {
		t.Fatalf("All() len=%d want=%d", len(a), len(ALL))
	}
	a[0] = "MUTATED"
	if ALL[0] != "api_key_invalid" {
		t.Errorf("All() did not return a copy — ALL[0] mutated to %q", ALL[0])
	}
}

// TestConstants_ExportedNames — const 字面 byte-identical (grep 检查
// 反字面漂移 在 import-site 上反向断言).
func TestConstants_ExportedNames(t *testing.T) {
	cases := []struct{ name, val, want string }{
		{"APIKeyInvalid", APIKeyInvalid, "api_key_invalid"},
		{"QuotaExceeded", QuotaExceeded, "quota_exceeded"},
		{"NetworkUnreachable", NetworkUnreachable, "network_unreachable"},
		{"RuntimeCrashed", RuntimeCrashed, "runtime_crashed"},
		{"RuntimeTimeout", RuntimeTimeout, "runtime_timeout"},
		{"Unknown", Unknown, "unknown"},
	}
	for _, c := range cases {
		if c.val != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.val, c.want)
		}
		if strings.TrimSpace(c.val) != c.val {
			t.Errorf("%s has whitespace: %q", c.name, c.val)
		}
	}
}
