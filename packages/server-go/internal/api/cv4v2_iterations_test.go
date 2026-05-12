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
	"os"
	"path/filepath"
	"strings"
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

// TestCv4v2Iterations_NoSchemaChange — acceptance §1.2 设计 ④ 0 schema 改.
// Scan production migrations + bpp/api packages for forbidden
// CV-4 v2 history table / event sequence literals.
func TestCv4v2Iterations_NoSchemaChange(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"ALTER TABLE artifact_iterations",
		"CREATE TABLE iteration_history",
		"CREATE TABLE artifact_iteration_history",
		"iteration_history_event",
		"artifact_iteration_log",
		"iteration_history_table",
	}
	dirs := []string{
		"../migrations",
		"../api",
		"../bpp",
	}
	hits := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := string(b)
			for _, bad := range forbidden {
				if strings.Contains(content, bad) {
					hits = append(hits, path+":"+bad)
				}
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("CV-4 v2 stance §0.1+§0.4 broken — forbidden history-event/schema literals in production: %v", hits)
	}
}

// TestCV_AdminGodModeNotMounted — acceptance §1.3 设计 ③+§4 ADM-0
// red-line. admin*.go must not reference iteration list endpoint.
func TestCV_AdminGodModeNotMounted(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"admin.*iterations",
		"admin.*CV4",
		"admin.*ListIterations",
	}
	// Use literal substring (not regex) for substring scan.
	literals := []string{
		"admin/iterations",
		"AdminListIterations",
		"AdminCV4",
		"adminListIterations",
		"/admin-api/iterations",
	}
	dir := "../api"
	hits := []string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "admin") {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(b)
		for _, bad := range literals {
			if strings.Contains(content, bad) {
				hits = append(hits, path+":"+bad)
			}
		}
	}
	if len(hits) > 0 {
		t.Errorf("CV-4 v2 stance §3 broken — admin god-mode reference iteration endpoint (ADM-0 §1.3 red-line): %v", hits)
	}
	_ = forbidden // documented pattern set, literal scan above is the gate
}
