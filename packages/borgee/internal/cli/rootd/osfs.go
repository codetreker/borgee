//go:build linux || darwin

package rootd

import (
	"errors"
	"os"
)

// osRemove + isNotExist are extracted so realFS does not pull os into
// the per-handler test surface — handlers_test injects a fakeFS that
// does not touch the real filesystem.
func osRemove(path string) error {
	return os.Remove(path)
}

func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
