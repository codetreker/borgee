//go:build linux || darwin

package install

import (
	"os"
	"runtime"
)

// isRoot tells the test helpers whether the process can exercise the
// sudo-only branches.
func isRoot() bool { return os.Geteuid() == 0 }

// runtimeGOOS exposes runtime.GOOS to test files in this package without
// each adding a `runtime` import.
func runtimeGOOS() string { return runtime.GOOS }
