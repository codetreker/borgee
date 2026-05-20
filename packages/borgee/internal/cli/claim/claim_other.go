//go:build !linux && !darwin

// Package claim — unsupported-platform fallback for non-linux/darwin.
package claim

import (
	"errors"
	"io"
)

// Run is the unsupported-platform stub.
func Run(_ []string, _ io.Writer, _ io.Writer) error {
	return errors.New("borgee claim: this platform is not supported; helper runtime supports linux/darwin only")
}
