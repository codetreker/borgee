// Package api_test — CV-4 v2 server tests: limit query clamp + 反向断言
// (no schema change / admin god-mode not mounted / no history-event table).
//
// 设计反查 (跟 cv-4-v2-stance-checklist.md §1+§4):
//   ① iteration history 复用 v1 endpoint, 仅加 ?limit query (default 50,
//      max 200, 0/negative → 50)
//   ④ 0 schema 改 — grep 检查 `ALTER TABLE artifact_iterations` 等 0 hit
//   ⑦ admin god-mode 不挂 — grep 检查 admin*.go 反向断言

package api_test

import (
	"testing"

	"borgee-server/internal/api"
)

// TestCV_ListIterations_LimitClamp — acceptance §1.1 设计 ①
// limit query default/clamp matrix: 0 / -1 / 999 / 100 / "" → 50/50/200/100/50.
func TestCV_ListIterations_LimitClamp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want int
		desc string
	}{
		{"", 50, "empty → default 50"},
		{"0", 50, "zero → default 50"},
		{"-1", 50, "negative → default 50"},
		{"abc", 50, "non-numeric → default 50"},
		{"100", 100, "in-range pass-through"},
		{"200", 200, "max boundary"},
		{"999", 200, "above max → clamp 200"},
		{"1", 1, "minimum positive"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := api.ClampCV4V2LimitForTest(tc.raw)
			if got != tc.want {
				t.Errorf("limit %q: got %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}
