// Package install — borgee install subcommand.
// Gutted in t3a (binary strip); the real install flow is rebuilt in T3b.
package install

import (
	"errors"
	"io"
)

// Run is a placeholder until T3b rebuilds install. Fails loud.
func Run(_ []string, _ io.Writer, _ io.Writer) error {
	return errors.New("borgee install: not implemented in this build (rebuilt by T3b)")
}
