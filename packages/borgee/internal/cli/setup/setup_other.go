//go:build !linux && !darwin

// Package setup — unsupported-platform stub.
package setup

import (
	"errors"
	"io"
)

// Run is the unsupported-platform stub.
func Run(_ []string, _ io.Writer, _ io.Writer) error {
	return errors.New("borgee setup: only linux/darwin are supported")
}
