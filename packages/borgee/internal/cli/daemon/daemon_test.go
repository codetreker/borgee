package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"borgee/internal/tokenstore"
)

func TestRun_RequiresServer(t *testing.T) {
	var out, errb bytes.Buffer
	err := Run([]string{"--dirs", "/tmp"}, &out, &errb)
	if err == nil || !strings.Contains(err.Error(), "--server is required") {
		t.Fatalf("err = %v; want --server is required", err)
	}
}

func TestRun_RequiresDirs(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: requireNonRoot fires before the --dirs check")
	}
	var out, errb bytes.Buffer
	err := Run([]string{"--server", "ws://x", "--token", "tok"}, &out, &errb)
	if err == nil || !strings.Contains(err.Error(), "--dirs is required") {
		t.Fatalf("err = %v; want --dirs is required", err)
	}
}

func TestRun_NoTokenAndNoPersistedFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: requireNonRoot fires before token resolution")
	}
	missing := filepath.Join(t.TempDir(), "no-token")
	var out, errb bytes.Buffer
	err := Run([]string{"--server", "ws://x", "--dirs", "/tmp", "--token-file", missing}, &out, &errb)
	if err == nil {
		t.Fatal("err = nil; want a no-token error")
	}
	if !strings.Contains(err.Error(), "no token provided via --token") {
		t.Errorf("err = %q; want the no-token guidance", err.Error())
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("err = %q; want it to name the token-file path %q", err.Error(), missing)
	}
}

func TestRequireNonRoot(t *testing.T) {
	if err := requireNonRoot(func() int { return 0 }); err == nil {
		t.Error("requireNonRoot(uid=0) = nil; want refuse error")
	}
	if err := requireNonRoot(func() int { return 1000 }); err != nil {
		t.Errorf("requireNonRoot(uid=1000) = %v; want nil", err)
	}
}

func TestSplitDirs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"   ", []string{}},
		{"/a", []string{"/a"}},
		{"/a,/b", []string{"/a", "/b"}},
		{" /a , , /b ,", []string{"/a", "/b"}},
	}
	for _, c := range cases {
		got := splitDirs(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitDirs(%q) = %v; want %v", c.in, got, c.want)
		}
	}
}

// TestRun_PersistedTokenReadByTokenstore covers the persisted-token fallback
// contract daemon.Run relies on: a token written by tokenstore is read back so
// Run gets past token resolution without --token. (Run itself then blocks on a
// real connection, so the resolve branch is exercised here against tokenstore
// directly rather than driving Run into a network dial.)
func TestRun_PersistedTokenReadByTokenstore(t *testing.T) {
	tf := filepath.Join(t.TempDir(), "token")
	if err := tokenstore.WriteToken(tf, "persisted-tok"); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	got, ok := tokenstore.ReadToken(tf)
	if !ok || got != "persisted-tok" {
		t.Fatalf("ReadToken = (%q,%v); want (persisted-tok,true)", got, ok)
	}
}
