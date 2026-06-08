// Package daemon — borgee daemon subcommand.
// Gutted in t3a (binary strip); the real reverse-WS daemon is rebuilt in T3b.
package daemon

import (
	"errors"
	"io"
)

// Run is a placeholder until T3b rebuilds the daemon. It fails loud so a
// build cut between t3a and T3b never silently no-ops a `borgee daemon`.
func Run(_ []string, _ io.Writer, _ io.Writer) error {
	return errors.New("borgee daemon: not implemented in this build (rebuilt by T3b)")
}
