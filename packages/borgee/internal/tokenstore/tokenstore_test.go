package tokenstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	tok := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := WriteToken(path, tok); err != nil {
		t.Fatalf("WriteToken: %v", err)
	}
	got, ok := ReadToken(path)
	if !ok {
		t.Fatal("ReadToken ok = false; want true")
	}
	if got != tok {
		t.Errorf("ReadToken = %q; want %q", got, tok)
	}
}

func TestWriteToken_Mode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := WriteToken(path, "abc"); err != nil {
		t.Fatalf("WriteToken: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token mode = %o; want 0600", perm)
	}
}

func TestWriteToken_AtomicNoTmpLeft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := WriteToken(path, "abc"); err != nil {
		t.Fatalf("WriteToken: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("tmp file still present (err=%v); want it renamed away", err)
	}
}

func TestWriteToken_EmptyRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := WriteToken(path, ""); err == nil {
		t.Error("WriteToken(empty) err = nil; want non-nil")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("empty WriteToken created a file (err=%v); want none", err)
	}
}

func TestReadToken_MissingReturnsFalse(t *testing.T) {
	got, ok := ReadToken(filepath.Join(t.TempDir(), "nonexistent"))
	if ok || got != "" {
		t.Errorf("ReadToken(missing) = (%q,%v); want (\"\",false)", got, ok)
	}
}

func TestReadToken_EmptyFileReturnsFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("  \n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, ok := ReadToken(path)
	if ok || got != "" {
		t.Errorf("ReadToken(whitespace) = (%q,%v); want (\"\",false)", got, ok)
	}
}

func TestDefaultTokenPath_XDGOverride(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: DefaultTokenPath uses the system dir, not XDG")
	}
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)
	got := DefaultTokenPath()
	want := filepath.Join(xdg, "borgee-remote-agent", "token")
	if got != want {
		t.Errorf("DefaultTokenPath with XDG = %q; want %q", got, want)
	}
}

func TestDefaultTokenPath_NoXDGFallsBackToLocalState(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: DefaultTokenPath uses the system dir, not XDG")
	}
	t.Setenv("XDG_STATE_HOME", "")
	got := DefaultTokenPath()
	suffix := filepath.Join(".local", "state", "borgee-remote-agent", "token")
	if !strings.HasSuffix(got, suffix) {
		t.Errorf("DefaultTokenPath without XDG = %q; want suffix %q", got, suffix)
	}
}
