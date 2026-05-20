//go:build !linux && !darwin

// Package daemon is the unsupported-platform fallback for non-linux/darwin builds.
// Current helper runtime is linux/darwin only; Windows support remains deferred.
package daemon

import (
	"errors"
	"io"
)

// Run is the unsupported-platform stub.
func Run(_ []string, _ io.Writer, _ io.Writer) error {
	return errors.New("borgee daemon: this platform is not supported; current helper runtime supports linux/darwin only")
}
