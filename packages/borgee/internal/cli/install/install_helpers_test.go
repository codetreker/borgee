//go:build linux || darwin

package install

import "os"

// isRoot tells the test helpers whether the process can exercise the
// sudo-only branches.
func isRoot() bool { return os.Geteuid() == 0 }
