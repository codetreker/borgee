package uninstall

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeRunner records the systemctl invocations without touching a real init.
type fakeRunner struct {
	calls [][]string
}

func (f *fakeRunner) run(name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	return nil
}

func TestUninstall_MissingUnitIsIdempotent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uninstall is Linux-only")
	}
	// Point the unit dir at an empty temp config home so no unit file exists.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tokenFile := filepath.Join(t.TempDir(), "token")
	var out, errb bytes.Buffer
	fr := &fakeRunner{}
	if err := run([]string{"--token-file", tokenFile}, &out, &errb, fr.run, func() bool { return true }); err != nil {
		t.Fatalf("run with missing unit err = %v; want nil (idempotent)", err)
	}
}

func TestUninstall_LeavesTokenByDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uninstall is Linux-only")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("tok"), 0o600); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	var out, errb bytes.Buffer
	fr := &fakeRunner{}
	if err := run([]string{"--token-file", tokenFile}, &out, &errb, fr.run, func() bool { return true }); err != nil {
		t.Fatalf("run err = %v", err)
	}
	if _, err := os.Stat(tokenFile); err != nil {
		t.Errorf("token removed without --purge (err=%v); want it kept", err)
	}
}

func TestUninstall_PurgeRemovesToken(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uninstall is Linux-only")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("tok"), 0o600); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	var out, errb bytes.Buffer
	fr := &fakeRunner{}
	if err := run([]string{"--purge", "--token-file", tokenFile}, &out, &errb, fr.run, func() bool { return true }); err != nil {
		t.Fatalf("run err = %v", err)
	}
	if _, err := os.Stat(tokenFile); !os.IsNotExist(err) {
		t.Errorf("--purge did not remove the token (err=%v)", err)
	}
}

// TestUninstall_DisablesAndReloads asserts the systemctl --user command set.
func TestUninstall_DisablesAndReloads(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uninstall is Linux-only")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tokenFile := filepath.Join(t.TempDir(), "token")
	var out, errb bytes.Buffer
	fr := &fakeRunner{}
	if err := run([]string{"--token-file", tokenFile}, &out, &errb, fr.run, func() bool { return true }); err != nil {
		t.Fatalf("run err = %v", err)
	}
	wantContains := [][]string{
		{"systemctl", "--user", "disable", "--now", "borgee.service"},
		{"systemctl", "--user", "daemon-reload"},
	}
	for _, want := range wantContains {
		found := false
		for _, got := range fr.calls {
			if equalSlice(got, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing systemctl call %v; got %v", want, fr.calls)
		}
	}
}

// TestUninstall_NoSystemctlSkipsRunner — when systemctl is absent, no runner
// call is made and the uninstall still succeeds.
func TestUninstall_NoSystemctlSkipsRunner(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uninstall is Linux-only")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tokenFile := filepath.Join(t.TempDir(), "token")
	var out, errb bytes.Buffer
	fr := &fakeRunner{}
	if err := run([]string{"--token-file", tokenFile}, &out, &errb, fr.run, func() bool { return false }); err != nil {
		t.Fatalf("run err = %v", err)
	}
	if len(fr.calls) != 0 {
		t.Errorf("runner called %v with no systemctl present; want none", fr.calls)
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
