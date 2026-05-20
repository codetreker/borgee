//go:build !linux && !darwin

// Package uninstallhost — unsupported-platform fallback.
package uninstallhost

import (
	"errors"
	"io"
)

func Run(_ []string, _ io.Writer, _ io.Writer) error {
	return errors.New("borgee uninstall-host: this platform is not supported; helper runtime supports linux/darwin only")
}
