package fsops

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeFile creates a file with the given bytes under dir and returns its path.
func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// --- error code: path_not_allowed (ls + read + stat) ---

func TestFsops_PathNotAllowed(t *testing.T) {
	allowedRoot := t.TempDir()
	otherRoot := t.TempDir() // a sibling tmp dir, outside allowedRoot
	outside := writeFile(t, otherRoot, "secret.txt", []byte("nope"))
	allowed := []string{allowedRoot}

	if _, code, _ := Ls(allowed, otherRoot); code != ErrPathNotAllowed {
		t.Errorf("Ls outside root: code = %q, want %q", code, ErrPathNotAllowed)
	}
	if _, code, _ := Read(allowed, outside); code != ErrPathNotAllowed {
		t.Errorf("Read outside root: code = %q, want %q", code, ErrPathNotAllowed)
	}
	if _, code, _ := Stat(allowed, outside); code != ErrPathNotAllowed {
		t.Errorf("Stat outside root: code = %q, want %q", code, ErrPathNotAllowed)
	}
}

// --- error code: path_not_found (ls + stat ENOENT) ---

func TestFsops_PathNotFound(t *testing.T) {
	root := t.TempDir()
	allowed := []string{root}
	missing := filepath.Join(root, "does-not-exist")

	if _, code, _ := Ls(allowed, missing); code != ErrPathNotFound {
		t.Errorf("Ls missing dir: code = %q, want %q", code, ErrPathNotFound)
	}
	if _, code, _ := Stat(allowed, missing); code != ErrPathNotFound {
		t.Errorf("Stat missing path: code = %q, want %q", code, ErrPathNotFound)
	}
}

// --- error code: file_not_found (read ENOENT — the 5th, distinct code) ---

func TestFsops_FileNotFound(t *testing.T) {
	root := t.TempDir()
	allowed := []string{root}
	missing := filepath.Join(root, "ghost.txt")

	if _, code, _ := Read(allowed, missing); code != ErrFileNotFound {
		t.Errorf("Read missing file: code = %q, want %q (must differ from %q)", code, ErrFileNotFound, ErrPathNotFound)
	}
}

// --- error code: file_too_large (read > 2 MiB) + boundary (== 2 MiB allowed) ---

func TestFsops_FileTooLarge(t *testing.T) {
	root := t.TempDir()
	allowed := []string{root}

	over := writeFile(t, root, "big.txt", make([]byte, MaxFileSize+1))
	if _, code, _ := Read(allowed, over); code != ErrFileTooLarge {
		t.Errorf("Read MaxFileSize+1: code = %q, want %q", code, ErrFileTooLarge)
	}

	// Exactly MaxFileSize is allowed (asserts the `>` check, not `>=`).
	exact := writeFile(t, root, "exact.txt", make([]byte, MaxFileSize))
	res, code, _ := Read(allowed, exact)
	if code != "" {
		t.Errorf("Read exactly MaxFileSize: code = %q, want empty (equal is allowed)", code)
	}
	if res.Size != MaxFileSize {
		t.Errorf("Read exactly MaxFileSize: size = %d, want %d", res.Size, MaxFileSize)
	}
}

// --- error code: is_directory (read target is a dir) ---

func TestFsops_IsDirectory(t *testing.T) {
	root := t.TempDir()
	allowed := []string{root}
	sub := filepath.Join(root, "adir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, code, _ := Read(allowed, sub); code != ErrIsDirectory {
		t.Errorf("Read on dir: code = %q, want %q", code, ErrIsDirectory)
	}
}

// --- happy path: ls (mixed file + subdir entries) ---

func TestFsops_LsHappy(t *testing.T) {
	root := t.TempDir()
	allowed := []string{root}
	writeFile(t, root, "a.txt", []byte("hello"))
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	res, code, _ := Ls(allowed, root)
	if code != "" {
		t.Fatalf("Ls happy: code = %q, want empty", code)
	}
	byName := map[string]DirEntry{}
	for _, e := range res.Entries {
		byName[e.Name] = e
	}
	if len(byName) != 2 {
		t.Fatalf("Ls returned %d entries, want 2: %+v", len(byName), res.Entries)
	}
	if f, ok := byName["a.txt"]; !ok {
		t.Error("Ls missing a.txt")
	} else {
		if f.IsDirectory {
			t.Error("a.txt marked as directory")
		}
		if f.Size != int64(len("hello")) {
			t.Errorf("a.txt size = %d, want %d", f.Size, len("hello"))
		}
		if f.Mtime == "" {
			t.Error("a.txt mtime empty, want populated")
		}
	}
	if d, ok := byName["sub"]; !ok {
		t.Error("Ls missing sub")
	} else if !d.IsDirectory {
		t.Error("sub not marked as directory")
	}
}

// --- happy path: read (small UTF-8 file) ---

func TestFsops_ReadHappy(t *testing.T) {
	root := t.TempDir()
	allowed := []string{root}
	body := "package main\n"
	p := writeFile(t, root, "main.go", []byte(body))

	res, code, _ := Read(allowed, p)
	if code != "" {
		t.Fatalf("Read happy: code = %q, want empty", code)
	}
	if res.Content != body {
		t.Errorf("Read content = %q, want %q", res.Content, body)
	}
	if res.MimeType != "text/x-go" {
		t.Errorf("Read mime = %q, want text/x-go", res.MimeType)
	}
	if res.Size != int64(len(body)) {
		t.Errorf("Read size = %d, want %d", res.Size, len(body))
	}
}

// --- happy path: stat (file and dir) ---

func TestFsops_StatHappy(t *testing.T) {
	root := t.TempDir()
	allowed := []string{root}
	p := writeFile(t, root, "f.txt", []byte("xyz"))

	fres, code, _ := Stat(allowed, p)
	if code != "" {
		t.Fatalf("Stat file: code = %q, want empty", code)
	}
	if fres.IsDirectory {
		t.Error("Stat file: IsDirectory = true, want false")
	}
	if fres.Size != 3 {
		t.Errorf("Stat file: size = %d, want 3", fres.Size)
	}
	if fres.Mtime == "" {
		t.Error("Stat file: mtime empty")
	}

	dres, code, _ := Stat(allowed, root)
	if code != "" {
		t.Fatalf("Stat dir: code = %q, want empty", code)
	}
	if !dres.IsDirectory {
		t.Error("Stat dir: IsDirectory = false, want true")
	}
}

// --- MIME: sample rows + lowercase + fallback ---

func TestFsops_MimeType(t *testing.T) {
	cases := []struct{ path, want string }{
		{"x.go", "text/x-go"},
		{"x.png", "image/png"},
		{"X.GO", "text/x-go"},                        // uppercase ext → lowercase lookup
		{"x.unknownext", "application/octet-stream"}, // fallback
		{"noext", "application/octet-stream"},
	}
	for _, c := range cases {
		if got := getMimeType(c.path); got != c.want {
			t.Errorf("getMimeType(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// --- mtime layout round-trip (millis + Z) ---

func TestFsops_MtimeLayout(t *testing.T) {
	root := t.TempDir()
	allowed := []string{root}
	p := writeFile(t, root, "t.txt", []byte("z"))

	fixed := time.Date(2026, 6, 8, 17, 50, 0, 0, time.UTC)
	if err := os.Chtimes(p, fixed, fixed); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	res, code, _ := Stat(allowed, p)
	if code != "" {
		t.Fatalf("Stat: code = %q", code)
	}
	const want = "2026-06-08T17:50:00.000Z"
	if res.Mtime != want {
		t.Errorf("Stat mtime = %q, want %q", res.Mtime, want)
	}
}

// --- containment boundary: /home/userX must not match allowed /home/user ---

func TestFsops_ContainmentBoundary(t *testing.T) {
	tmp := t.TempDir()
	allowedDir := filepath.Join(tmp, "home", "user")
	siblingDir := filepath.Join(tmp, "home", "userX")
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatalf("mkdir allowed: %v", err)
	}
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatalf("mkdir sibling: %v", err)
	}
	siblingFile := writeFile(t, siblingDir, "f", []byte("x"))
	allowed := []string{allowedDir}

	// /home/userX/f must NOT be allowed under allowed root /home/user.
	if _, code, _ := Read(allowed, siblingFile); code != ErrPathNotAllowed {
		t.Errorf("Read sibling /home/userX: code = %q, want %q", code, ErrPathNotAllowed)
	}

	// /home/user/sub/f IS allowed.
	subDir := filepath.Join(allowedDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	subFile := writeFile(t, subDir, "f", []byte("ok"))
	if _, code, _ := Read(allowed, subFile); code != "" {
		t.Errorf("Read /home/user/sub/f: code = %q, want empty (allowed)", code)
	}
}

// --- invalid UTF-8 read does NOT error and round-trips the bytes ---

func TestFsops_ReadInvalidUTF8(t *testing.T) {
	root := t.TempDir()
	allowed := []string{root}
	// 0xff 0xfe is not valid UTF-8. JS fs.readFileSync(_, 'utf-8') decodes
	// lossily without throwing; the Go mirror must NOT raise an error code
	// and must round-trip the raw bytes via string(b).
	raw := []byte{0xff, 0xfe}
	p := writeFile(t, root, "bin.dat", raw)

	res, code, err := Read(allowed, p)
	if err != nil {
		t.Fatalf("Read invalid-UTF8: unexpected Go error %v", err)
	}
	if code != "" {
		t.Errorf("Read invalid-UTF8: code = %q, want empty (no error on invalid UTF-8)", code)
	}
	if res.Content != string(raw) {
		t.Errorf("Read invalid-UTF8: content = %q, want round-trip of raw bytes", res.Content)
	}
	if len(res.Content) == 0 {
		t.Error("Read invalid-UTF8: content empty, want non-empty lossy string")
	}
}

// --- ls swallows per-entry stat error (broken symlink → size 0, mtime "") ---

func TestFsops_LsSwallowsPerEntryStatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	root := t.TempDir()
	allowed := []string{root}
	// A symlink pointing at a missing target. os.Stat (which Ls uses, to
	// mirror statSync's symlink-follow) returns ENOENT → the per-entry
	// stat error is swallowed → that entry has Size 0 / Mtime "".
	link := filepath.Join(root, "dangling")
	if err := os.Symlink(filepath.Join(root, "no-such-target"), link); err != nil {
		t.Skipf("symlink unsupported here: %v", err)
	}

	res, code, _ := Ls(allowed, root)
	if code != "" {
		t.Fatalf("Ls: code = %q, want empty", code)
	}
	var found bool
	for _, e := range res.Entries {
		if e.Name == "dangling" {
			found = true
			if e.Size != 0 {
				t.Errorf("dangling symlink: size = %d, want 0 (swallowed stat)", e.Size)
			}
			if e.Mtime != "" {
				t.Errorf("dangling symlink: mtime = %q, want \"\" (swallowed stat)", e.Mtime)
			}
		}
	}
	if !found {
		t.Fatalf("Ls did not list the dangling symlink entry; got %+v", res.Entries)
	}
}

func TestFsops_MaxFileSizeConstant(t *testing.T) {
	if MaxFileSize != 2*1024*1024 {
		t.Errorf("MaxFileSize = %d, want %d (2 MiB)", MaxFileSize, 2*1024*1024)
	}
}

// ensure the strings import stays used by an assertion on error-code values.
func TestFsops_ErrCodeStrings(t *testing.T) {
	codes := []ErrCode{ErrPathNotAllowed, ErrPathNotFound, ErrFileTooLarge, ErrIsDirectory, ErrFileNotFound}
	want := []string{"path_not_allowed", "path_not_found", "file_too_large", "is_directory", "file_not_found"}
	for i, c := range codes {
		if string(c) != want[i] {
			t.Errorf("code[%d] = %q, want %q", i, c, want[i])
		}
		if strings.TrimSpace(string(c)) != string(c) {
			t.Errorf("code %q has surrounding whitespace", c)
		}
	}
}
