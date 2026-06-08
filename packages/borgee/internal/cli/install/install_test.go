package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testConfig() installConfig {
	return installConfig{
		Server:    "ws://localhost:4900",
		Token:     "secret-token-value",
		Dirs:      []string{"/srv/data", "/srv/logs"},
		TokenFile: "/home/u/.local/state/borgee-remote-agent/token",
		ExecPath:  "/usr/local/bin/borgee",
	}
}

// TestRenderUnit_NonRootByConstruction is the AC-4 machine-verifiable check:
// the rendered systemd --user unit has NO User= field (so it runs as the
// invoking user — non-root by construction) and targets default.target (the
// user-scope marker), and its ExecStart runs `borgee daemon` with the server +
// dirs.
func TestRenderUnit_NonRootByConstruction(t *testing.T) {
	unit := renderUnit(testConfig())

	if strings.Contains(unit, "User=root") {
		t.Errorf("unit contains User=root:\n%s", unit)
	}
	// No User= line at all — a --user unit cannot run as another user.
	for _, line := range strings.Split(unit, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "User=") {
			t.Errorf("unit has a User= line %q; a --user unit is non-root by construction", line)
		}
	}
	if !strings.Contains(unit, "WantedBy=default.target") {
		t.Errorf("unit missing WantedBy=default.target (user-scope marker):\n%s", unit)
	}
	if strings.Contains(unit, "WantedBy=multi-user.target") {
		t.Errorf("unit uses multi-user.target (system scope); want default.target:\n%s", unit)
	}
	if !strings.Contains(unit, "ExecStart=/usr/local/bin/borgee daemon ") {
		t.Errorf("ExecStart does not invoke `borgee daemon`:\n%s", unit)
	}
	if !strings.Contains(unit, "--server ws://localhost:4900") {
		t.Errorf("ExecStart missing --server:\n%s", unit)
	}
	if !strings.Contains(unit, "--dirs /srv/data,/srv/logs") {
		t.Errorf("ExecStart missing the comma-joined --dirs:\n%s", unit)
	}
}

// TestRenderUnit_TokenNotEmbedded — the secret must never land in the unit (it
// is read from the token file by the daemon).
func TestRenderUnit_TokenNotEmbedded(t *testing.T) {
	unit := renderUnit(testConfig())
	if strings.Contains(unit, "secret-token-value") {
		t.Errorf("unit embeds the token value:\n%s", unit)
	}
}

func TestRequireNonRoot_GuardsRoot(t *testing.T) {
	if err := requireNonRoot(func() int { return 0 }); err == nil {
		t.Error("requireNonRoot(uid=0) = nil; want refuse error")
	}
	if err := requireNonRoot(func() int { return 1000 }); err != nil {
		t.Errorf("requireNonRoot(uid=1000) = %v; want nil", err)
	}
}

// TestInstall_DryRunNoSideEffects — --dry-run prints the unit and touches
// nothing (no token file created).
func TestInstall_DryRunNoSideEffects(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: install refuses before reaching --dry-run")
	}
	tokenFile := filepath.Join(t.TempDir(), "token")
	var out, errb bytes.Buffer
	err := Run([]string{
		"--server", "ws://x",
		"--token", "tok",
		"--dirs", "/srv/data",
		"--token-file", tokenFile,
		"--dry-run",
	}, &out, &errb)
	if err != nil {
		t.Fatalf("dry-run Run err = %v (stderr=%q)", err, errb.String())
	}
	if _, statErr := os.Stat(tokenFile); !os.IsNotExist(statErr) {
		t.Errorf("--dry-run created the token file (err=%v); want no side effects", statErr)
	}
	if !strings.Contains(out.String(), "ExecStart=") {
		t.Errorf("--dry-run stdout missing the rendered unit; got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "tok") && strings.Contains(out.String(), "--token tok") {
		t.Errorf("--dry-run leaked the token into ExecStart:\n%s", out.String())
	}
}

func TestRun_RequiresTokenAtInstall(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: install refuses before the --token check")
	}
	var out, errb bytes.Buffer
	err := Run([]string{"--server", "ws://x", "--dirs", "/tmp"}, &out, &errb)
	if err == nil || !strings.Contains(err.Error(), "--token is required") {
		t.Fatalf("err = %v; want --token is required", err)
	}
}
