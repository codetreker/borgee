// Package regression provides a tiny in-process registry of named acceptance
// tests, keyed by the milestone ID that introduced them.
//
// INFRA-1b.3 — see docs/current/server/testing.md (testutil/regression).
//
// Why: README.md gate 5 requires every merged milestone's "4.1 acceptance
// invariant" to be included in the regression suite from that point on.
// Without a registry, maintainers must remember which tests count as regression
// coverage. With a registry, test files declare themselves at init() and
// `make regression` runs the combined set.
//
// The mechanism is deliberately minimal:
//
//   - At package init time, a *_test.go file calls Register(milestone, name, fn).
//   - `go test ./...` ignores the registry — those tests run normally as part
//     of unit/integration sweeps.
//   - The Makefile target `make regression` runs `go test -run RegressionSuite`
//     which is a single dispatcher (RunAll) that invokes every registered test
//     under a sub-test named "<milestone>/<name>".
//
// Registration is panic-on-duplicate to surface mistakes early.
package regression

import (
	"fmt"
	"sort"
	"sync"
	"testing"
)

// Func is the signature a regression test must satisfy. It mirrors the body
// of a *testing.T test but is a free function so it can be registered.
type Func func(t *testing.T)

// Entry is one registered regression test.
type Entry struct {
	Milestone string // e.g. "INFRA-1a", "CM-1"
	Name      string // human-readable acceptance label
	Fn        Func
}

var (
	mu       sync.Mutex
	registry []Entry
	seen     = map[string]struct{}{}
)

// Register adds (milestone, name, fn) to the regression registry. Call this
// from a test file's init() (or TestMain). Duplicate (milestone, name) panics
// because two tests claiming the same identity defeats the audit trail.
func Register(milestone, name string, fn Func) {
	if milestone == "" {
		panic("regression.Register: empty milestone")
	}
	if name == "" {
		panic("regression.Register: empty name")
	}
	if fn == nil {
		panic("regression.Register: nil fn")
	}
	key := milestone + "/" + name
	mu.Lock()
	defer mu.Unlock()
	if _, dup := seen[key]; dup {
		panic(fmt.Sprintf("regression.Register: duplicate %q", key))
	}
	seen[key] = struct{}{}
	registry = append(registry, Entry{Milestone: milestone, Name: name, Fn: fn})
}

// Entries returns a copy of the registry sorted by (milestone, name) for
// stable output.
func Entries() []Entry {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Entry, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Milestone != out[j].Milestone {
			return out[i].Milestone < out[j].Milestone
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// RunAll dispatches every registered entry as a sub-test of t. Use it from a
// dispatcher test (typically named TestRegressionSuite) that the Makefile
// target invokes via `-run RegressionSuite`.
func RunAll(t *testing.T) {
	t.Helper()
	entries := Entries()
	if len(entries) == 0 {
		t.Skip("no regression entries registered")
		return
	}
	for _, e := range entries {
		e := e
		t.Run(e.Milestone+"/"+e.Name, func(t *testing.T) {
			e.Fn(t)
		})
	}
}

// Reset clears the registry. Test-only; do not use from production code (this
// package has no production consumers anyway).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	registry = nil
	seen = map[string]struct{}{}
}
