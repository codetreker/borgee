package fileio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHB2D_ReadFile_HappyPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	res, err := ReadFile(p, 0)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(res.Bytes) != "hello world" {
		t.Errorf("content 脱节: %q", res.Bytes)
	}
	if res.Truncated {
		t.Errorf("expected non-truncated")
	}
	if res.Size != 11 {
		t.Errorf("size: %d", res.Size)
	}
}

func TestHB2D_ReadFile_MaxBytesTruncate(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "big.txt")
	body := strings.Repeat("a", 1000)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	res, err := ReadFile(p, 100)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(res.Bytes) != 100 {
		t.Errorf("expect 100 bytes, got %d", len(res.Bytes))
	}
	if !res.Truncated {
		t.Errorf("expect truncated")
	}
	if res.Size != 1000 {
		t.Errorf("expect size=1000, got %d", res.Size)
	}
}

func TestHB2D_ReadFile_NotExist(t *testing.T) {
	t.Parallel()
	_, err := ReadFile("/this/never/exists", 0)
	if err == nil {
		t.Errorf("expected error for missing file")
	}
}

func TestHB2D_ReadFile_DirectoryRejected(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	_, err := ReadFile(tmp, 0)
	if err == nil {
		t.Errorf("expected error reading a directory")
	}
}

func TestHB2D_ListFiles_HappyPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		_ = os.WriteFile(filepath.Join(tmp, name), []byte("x"), 0o644)
	}
	if err := os.Mkdir(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	res, err := ListFiles(tmp)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(res.Entries) != 4 {
		t.Errorf("expect 4 entries, got %d", len(res.Entries))
	}
	hasDir := false
	for _, e := range res.Entries {
		if e.Name == "sub" && e.IsDir {
			hasDir = true
		}
	}
	if !hasDir {
		t.Errorf("missing sub-dir entry")
	}
}

func TestHB2D_ListFiles_NotExist(t *testing.T) {
	t.Parallel()
	_, err := ListFiles("/this/never/exists")
	if err == nil {
		t.Errorf("expected error")
	}
}
