// Package uninstall implements `borgee uninstall` — stop and remove the
// systemd --user service. The persisted token is left in place by default so a
// reinstall on the same machine keeps the node identity; --purge wipes it.
// Idempotent: a missing unit / not-loaded service is not an error. Never
// touches /etc/, never needs root.
package uninstall

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"borgee/internal/tokenstore"
)

// runnerFunc runs a command (injected for tests). The default shells out via
// os/exec; a non-zero exit is returned as an error the caller may tolerate.
type runnerFunc func(name string, args ...string) error

func execRunner(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

// Run parses flags and removes the service. The systemctl runner is injectable
// via the unexported run() so tests assert the command set without systemd.
func Run(args []string, stdout, stderr io.Writer) error {
	return run(args, stdout, stderr, execRunner, hasSystemctl)
}

func run(args []string, stdout, stderr io.Writer, runner runnerFunc, systemctlPresent func() bool) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		purge     = fs.Bool("purge", false, "Also delete the persisted token (forget this machine entirely)")
		tokenFile = fs.String("token-file", tokenstore.DefaultTokenPath(), "Path to the persisted token file")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if runtime.GOOS != "linux" {
		return fmt.Errorf("borgee uninstall is only supported on Linux (systemd --user); this is %s", runtime.GOOS)
	}

	// Stop + disable the service (best-effort: tolerate a not-loaded unit).
	if systemctlPresent() {
		_ = runner("systemctl", "--user", "disable", "--now", "borgee.service")
	} else {
		fmt.Fprintln(stdout, "borgee uninstall: systemctl not found; skipping service stop.")
	}

	// Remove the unit file (a missing file is not an error).
	unitPath := userUnitPath()
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit %s: %w", unitPath, err)
	}
	fmt.Fprintf(stdout, "borgee uninstall: removed unit %s\n", unitPath)

	if systemctlPresent() {
		_ = runner("systemctl", "--user", "daemon-reload")
	}

	// Token: leave it unless --purge.
	if *purge {
		if err := os.Remove(*tokenFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove token %s: %w", *tokenFile, err)
		}
		fmt.Fprintf(stdout, "borgee uninstall: purged token %s\n", *tokenFile)
	} else {
		fmt.Fprintf(stdout, "borgee uninstall: kept token %s (use --purge to remove it)\n", *tokenFile)
	}
	return nil
}

func hasSystemctl() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

// userUnitPath is ~/.config/systemd/user/borgee.service (mirrors install).
func userUnitPath() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "systemd", "user", "borgee.service")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", "borgee.service")
}
