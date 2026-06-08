// Package tokenstore persists the borgee remote daemon's enrollment token to
// a known state directory so the daemon can restart (including after reboot)
// without the operator re-passing --token.
//
// Ported from packages/remote-agent/src/tokenStore.ts: platform-aware default
// path, atomic 0600 owner-only write, never-throwing read. Stdlib only.
package tokenstore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// linuxStateDirRoot is the system token dir used when running as root,
	// mirroring DEFAULT_LINUX_STATE_DIR_ROOT in tokenStore.ts.
	linuxStateDirRoot = "/var/lib/borgee-remote-agent"
	// macStateDir mirrors DEFAULT_MAC_STATE_DIR in tokenStore.ts. Retained
	// for read-compat with a host that ran the Node agent on macOS; the
	// install flow itself is Linux-only in this build.
	macStateDir = "/Library/Application Support/Borgee/RemoteAgent"
	// stateDirName is the per-user subdirectory under XDG_STATE_HOME. It is
	// deliberately the literal "borgee-remote-agent" (not "borgee") so a
	// token persisted by the Node agent stays readable after the binary
	// cutover.
	stateDirName = "borgee-remote-agent"
)

// DefaultTokenPath returns the platform-aware default path for the persisted
// token file. Mirrors defaultTokenPath() in tokenStore.ts.
//
//	macOS:            /Library/Application Support/Borgee/RemoteAgent/token
//	Linux root:       /var/lib/borgee-remote-agent/token
//	Linux non-root:   $XDG_STATE_HOME/borgee-remote-agent/token
//	                  (or ~/.local/state/borgee-remote-agent/token)
func DefaultTokenPath() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(macStateDir, "token")
	}
	if os.Getuid() == 0 {
		return filepath.Join(linuxStateDirRoot, "token")
	}
	stateRoot := strings.TrimSpace(os.Getenv("XDG_STATE_HOME"))
	if stateRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fall back to a relative path rather than panicking; the
			// daemon will surface a clear error when it cannot read/write.
			home = ""
		}
		stateRoot = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateRoot, stateDirName, "token")
}

// ReadToken reads a token from disk. It returns ("", false) if the file is
// missing, empty (after trim), or unreadable — never an error. Callers treat
// ok==false as "no persisted token, fall back to --token". Mirrors
// readToken() in tokenStore.ts.
func ReadToken(path string) (token string, ok bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

// WriteToken writes a token atomically with file mode 0600 (owner-only).
// Mirrors writeToken() in tokenStore.ts:
//  1. reject an empty token,
//  2. mkdir -p the parent at 0700 (tolerate already-exists),
//  3. write <path>.tmp at 0600, fsync (best-effort), close,
//  4. preserve the destination's prior mode if it exists (else 0600),
//  5. rename .tmp -> path (atomic on POSIX).
func WriteToken(path, token string) error {
	if token == "" {
		return errors.New("tokenstore: token must be non-empty")
	}

	parent := filepath.Dir(path)
	// mkdir -p 0700 owner-only. If the dir already exists with a different
	// mode (e.g. a preset /var/lib/...), leave it alone — MkdirAll is a
	// no-op on an existing dir and does not chmod it.
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}

	// Preserve an existing destination's mode; otherwise default to 0600.
	targetMode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		if m := info.Mode().Perm(); m != 0 {
			targetMode = m
		}
	}

	tmp := path + ".tmp"
	// Open the tmp file with explicit 0600 so even if the rename races, no
	// intermediate world-readable file ever exists.
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(token); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	// fsync can fail on some filesystems (e.g. tmpfs in CI); the rename is
	// still atomic, so a missing fsync is non-fatal.
	_ = f.Sync()
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	// Apply the target mode before renaming so the final inode lands with
	// the correct perms; ignore a chmod failure (fallback is the 0600 from
	// OpenFile).
	_ = os.Chmod(tmp, targetMode)

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
