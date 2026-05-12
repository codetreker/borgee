// Package regression_suite hosts the single dispatcher test that
// `make regression` runs to execute every entry registered via
// `internal/testutil/regression`.
//
// Tests register themselves at init() (or TestMain) of their own test
// package; this package merely imports them via blank import so the init()s
// run.
//
// As regression entries land, add the corresponding test package to the
// blank import list below. The Makefile target invokes:
//
//	go test ./internal/testutil/regression_suite -run TestRegressionSuite
//
// which causes Go to load this package and, through the imports, every
// regression-contributing package before dispatching the registered tests.
package regression_suite

import (
	"testing"

	"borgee-server/internal/testutil/regression"
	// Blank-import test packages that register regression entries.
	// Append entries here as milestones close their 4.1 acceptance.
	//
	// (None yet — Phase 0 milestones land at the same time as this file.)
)

// TestRegressionSuite is the single entry point invoked by `make regression`.
// It calls regression.RunAll, which runs every registered Func.
func TestRegressionSuite(t *testing.T) {
	regression.RunAll(t)
}
