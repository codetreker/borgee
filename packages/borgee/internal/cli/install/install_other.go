//go:build !linux && !darwin

// Package install — unsupported-platform fallback (Windows / others).
package install

import (
	"errors"
	"io"
)

// Run is the unsupported-platform stub.
func Run(_ []string, _ io.Writer, _ io.Writer) error {
	return errors.New("borgee install: this platform is not supported; helper runtime supports linux/darwin only")
}
