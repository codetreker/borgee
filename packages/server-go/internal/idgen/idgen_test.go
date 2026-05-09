// Package idgen — idgen_test.go: ULID-MIGRATION SSOT helper tests.
//
// Spec: docs/implementation/modules/ulid-migration-spec.md §0 设计 ① + 必修-3
// (ULID monotonic 真测).

package idgen

import (
	"sort"
	"sync"
	"testing"
)

// TestNewID_LengthIs26 pins ULID canonical 26-char output.
func TestNewID_LengthIs26(t *testing.T) {
	t.Parallel()
	for i := 0; i < 50; i++ {
		id := NewID()
		if len(id) != 26 {
			t.Fatalf("NewID len = %d, want 26 (ULID canonical), got %q", len(id), id)
		}
	}
}

// TestNewID_Unique reports collision over a small batch (deterministic).
func TestNewID_Unique(t *testing.T) {
	t.Parallel()
	const N = 1000
	seen := make(map[string]struct{}, N)
	for i := 0; i < N; i++ {
		id := NewID()
		if _, dup := seen[id]; dup {
			t.Fatalf("collision at i=%d id=%q", i, id)
		}
		seen[id] = struct{}{}
	}
}

// TestNewID_Monotonic_SerialCalls pins lex-sortable monotonic order over
// serial calls (蓝图 §4.A.1 字面 ULID lock-in monotonic 设计).
func TestNewID_Monotonic_SerialCalls(t *testing.T) {
	t.Parallel()
	const N = 200
	ids := make([]string, N)
	for i := 0; i < N; i++ {
		ids[i] = NewID()
	}
	if !sort.StringsAreSorted(ids) {
		// Find first violation for diagnostic.
		for i := 1; i < N; i++ {
			if ids[i-1] >= ids[i] {
				t.Fatalf("monotonic violation at i=%d: %q >= %q", i, ids[i-1], ids[i])
			}
		}
	}
}

// TestNewID_GoroutineSafe pins concurrent-safe (反 monotonic violation
// across goroutines, mu serialize entropy).
func TestNewID_GoroutineSafe(t *testing.T) {
	t.Parallel()
	const G = 16
	const N = 200
	all := make([]string, 0, G*N)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for g := 0; g < G; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]string, N)
			for i := 0; i < N; i++ {
				local[i] = NewID()
			}
			mu.Lock()
			all = append(all, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	seen := make(map[string]struct{}, len(all))
	for _, id := range all {
		if _, dup := seen[id]; dup {
			t.Fatalf("collision across goroutines: %q", id)
		}
		seen[id] = struct{}{}
	}
}
