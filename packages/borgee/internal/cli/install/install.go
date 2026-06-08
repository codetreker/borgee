// Package install implements `borgee install` — enroll this machine by
// writing a systemd --user service that runs `borgee daemon`. A --user unit
// runs as the invoking user, so the daemon is non-root by construction (no
// User= field, no sudo, nothing written under /etc/). Linux-only; non-Linux
// fails loud.
package install

import (
	"errors"
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

// installConfig is the pure input to renderUnit + activate.
type installConfig struct {
	Server    string
	Token     string
	Dirs      []string
	TokenFile string
	ExecPath  string // absolute path to the borgee binary for ExecStart
}

// Run parses flags, renders the systemd --user unit, and (unless --dry-run)
// persists the token + writes + enables the unit.
func Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		server    = fs.String("server", "", "Borgee server WebSocket URL (e.g. ws://localhost:4900)")
		token     = fs.String("token", "", "One-shot connection token from the Borgee UI (persisted on first handshake)")
		dirs      = fs.String("dirs", "", "Comma-separated list of directories to expose (read-only)")
		tokenFile = fs.String("token-file", tokenstore.DefaultTokenPath(), "Path to the persisted token file (mode 0600, owner-only)")
		dryRun    = fs.Bool("dry-run", false, "Print the unit + activation commands without touching the system")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if runtime.GOOS != "linux" {
		return fmt.Errorf("borgee install is only supported on Linux (systemd --user); this is %s", runtime.GOOS)
	}
	if err := requireNonRoot(os.Getuid); err != nil {
		return err
	}
	if *server == "" {
		return errors.New("--server is required")
	}
	if strings.TrimSpace(*token) == "" {
		return errors.New("--token is required at install (the first enrollment token from the Borgee UI)")
	}
	allowed := splitDirs(*dirs)
	if len(allowed) == 0 {
		return errors.New("--dirs is required (at least one directory)")
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot resolve the borgee executable path: %w", err)
	}

	cfg := installConfig{
		Server:    *server,
		Token:     strings.TrimSpace(*token),
		Dirs:      allowed,
		TokenFile: *tokenFile,
		ExecPath:  execPath,
	}
	unit := renderUnit(cfg)

	if *dryRun {
		fmt.Fprintln(stdout, "# --dry-run: nothing was written. The systemd --user unit would be:")
		fmt.Fprintln(stdout, unit)
		fmt.Fprintln(stdout, "# Activation commands:")
		fmt.Fprintf(stdout, "#   write token to %s (mode 0600)\n", cfg.TokenFile)
		fmt.Fprintf(stdout, "#   write unit to %s\n", userUnitPath())
		fmt.Fprintln(stdout, "#   systemctl --user daemon-reload")
		fmt.Fprintln(stdout, "#   systemctl --user enable --now borgee.service")
		return nil
	}

	return activate(cfg, unit, stdout, stderr)
}

// renderUnit returns the systemd --user unit text. There is NO User= field: a
// --user unit runs as the invoking user by construction, which is the only
// mechanism consistent with the non-root + no-sudo + no-/etc-write boundary.
// The token is NOT embedded — the daemon reads it from the token file.
func renderUnit(cfg installConfig) string {
	execStart := fmt.Sprintf("%s daemon --server %s --dirs %s",
		cfg.ExecPath, cfg.Server, strings.Join(cfg.Dirs, ","))
	return strings.Join([]string{
		"[Unit]",
		"Description=Borgee remote node daemon",
		"After=network-online.target",
		"Wants=network-online.target",
		"",
		"[Service]",
		"Type=simple",
		"ExecStart=" + execStart,
		"Restart=on-failure",
		"RestartSec=5",
		"",
		"[Install]",
		"WantedBy=default.target",
		"",
	}, "\n")
}

// activate persists the token, writes the unit, and enables+starts it via
// systemctl --user. If systemctl is absent (e.g. a CI container with no init),
// the unit is written and a note is printed but the install does not fail.
func activate(cfg installConfig, unit string, stdout, stderr io.Writer) error {
	if err := tokenstore.WriteToken(cfg.TokenFile, cfg.Token); err != nil {
		return fmt.Errorf("failed to persist token to %s: %w", cfg.TokenFile, err)
	}
	fmt.Fprintf(stdout, "borgee install: persisted token to %s\n", cfg.TokenFile)

	unitPath := userUnitPath()
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("failed to create the systemd user unit dir: %w", err)
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("failed to write the unit to %s: %w", unitPath, err)
	}
	fmt.Fprintf(stdout, "borgee install: wrote unit to %s\n", unitPath)

	if _, err := exec.LookPath("systemctl"); err != nil {
		fmt.Fprintln(stdout, "borgee install: systemctl not found; the unit was written but not started. "+
			"Run `systemctl --user daemon-reload && systemctl --user enable --now borgee.service` on a systemd host.")
		return nil
	}

	for _, cmd := range [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", "borgee.service"},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Stdout = stdout
		c.Stderr = stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("`%s` failed: %w", strings.Join(cmd, " "), err)
		}
	}
	fmt.Fprintln(stdout, "borgee install: service enabled and started (systemctl --user).")
	return nil
}

// userUnitPath is ~/.config/systemd/user/borgee.service.
func userUnitPath() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "systemd", "user", "borgee.service")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", "borgee.service")
}

// requireNonRoot refuses to install as root. getuid is injected for testing.
func requireNonRoot(getuid func() int) error {
	if getuid() == 0 {
		return errors.New("refusing to install as root: borgee runs as a systemd --user service (non-root). Re-run as the target user.")
	}
	return nil
}

// splitDirs splits a comma-separated list, trims, and drops empties.
func splitDirs(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
