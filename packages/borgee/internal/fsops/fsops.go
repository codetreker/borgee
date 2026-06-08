// Package fsops provides read-only filesystem operations (ls / read / stat)
// for the borgee remote agent, mirroring the Node implementation at
// packages/remote-agent/src/fs-ops.ts line-for-line.
//
// Behaviour pinned to that reference (canonical until T5 deletes it):
//   - five wire error codes (path_not_allowed / path_not_found /
//     file_not_found / file_too_large / is_directory),
//   - 2 MiB read cap (strictly greater-than rejects),
//   - the 33-row MIME map with octet-stream fallback,
//   - lexical directory containment (no symlink resolution — Boundary #10),
//   - whole-file UTF-8 read with lossy decode of invalid bytes,
//   - millisecond-precision ISO-8601 UTC mtime.
//
// T3b/T3c serialize the ErrCode strings onto the WS frame, so the wire
// values are a cross-task contract — do not rename.
package fsops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrCode is the stable wire error string mirrored from fs-ops.ts. Empty
// means success.
type ErrCode string

const (
	// ErrPathNotAllowed — target outside every allowed root (containment
	// fail; fs-ops.ts:67,94,116).
	ErrPathNotAllowed ErrCode = "path_not_allowed"
	// ErrPathNotFound — ls / stat target does not exist (ENOENT;
	// fs-ops.ts:86,123).
	ErrPathNotFound ErrCode = "path_not_found"
	// ErrFileTooLarge — read target exceeds MaxFileSize (fs-ops.ts:102).
	ErrFileTooLarge ErrCode = "file_too_large"
	// ErrIsDirectory — read target is a directory (fs-ops.ts:99).
	ErrIsDirectory ErrCode = "is_directory"
	// ErrFileNotFound — read target does not exist (ENOENT; fs-ops.ts:108).
	// Distinct from ErrPathNotFound: read-ENOENT maps here, ls/stat-ENOENT
	// maps to ErrPathNotFound. Both ends must serialize these two distinct
	// strings.
	ErrFileNotFound ErrCode = "file_not_found"
)

// MaxFileSize is the read cap, mirroring MAX_FILE_SIZE (fs-ops.ts:50).
// Read rejects iff stat.Size() > MaxFileSize (strictly greater-than;
// equal is allowed).
const MaxFileSize int64 = 2 * 1024 * 1024 // 2 MiB

// mimeMap mirrors MIME_MAP (fs-ops.ts:23-48), 33 rows. The image rows are
// label-only — Read still does a UTF-8 text read regardless (Boundary #12).
var mimeMap = map[string]string{
	".ts":   "text/typescript",
	".tsx":  "text/typescript",
	".js":   "text/javascript",
	".jsx":  "text/javascript",
	".json": "application/json",
	".md":   "text/markdown",
	".html": "text/html",
	".htm":  "text/html",
	".css":  "text/css",
	".py":   "text/x-python",
	".rb":   "text/x-ruby",
	".go":   "text/x-go",
	".rs":   "text/x-rust",
	".java": "text/x-java",
	".c":    "text/x-c",
	".h":    "text/x-c",
	".cpp":  "text/x-c++",
	".hpp":  "text/x-c++",
	".sh":   "text/x-shellscript",
	".yml":  "text/yaml",
	".yaml": "text/yaml",
	".xml":  "application/xml",
	".svg":  "image/svg+xml",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".txt":  "text/plain",
	".log":  "text/plain",
	".env":  "text/plain",
	".toml": "text/toml",
	".sql":  "text/x-sql",
}

// DirEntry is one ls entry. JSON tags are camelCase to match the TS
// DirEntry field names the existing Node agent emits.
type DirEntry struct {
	Name        string `json:"name"`
	IsDirectory bool   `json:"isDirectory"`
	Size        int64  `json:"size"`
	Mtime       string `json:"mtime"`
}

// LsResult is the ls success shape.
type LsResult struct {
	Entries []DirEntry `json:"entries"`
}

// ReadResult is the read success shape (mirrors TS FileContent).
type ReadResult struct {
	Content  string `json:"content"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
}

// StatResult is the stat success shape.
type StatResult struct {
	Size        int64  `json:"size"`
	Mtime       string `json:"mtime"`
	IsDirectory bool   `json:"isDirectory"`
}

// pathAllowed mirrors isPathAllowed (fs-ops.ts:52-58): lexical containment
// only, NO filepath.EvalSymlinks (Boundary #10). The dir+separator suffix on
// the prefix check stops "/home/userX" matching allowed "/home/user".
func pathAllowed(target string, allowed []string) bool {
	resolved, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	resolved = filepath.Clean(resolved)
	for _, dir := range allowed {
		d, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		d = filepath.Clean(d)
		if resolved == d || strings.HasPrefix(resolved, d+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// getMimeType mirrors getMimeType (fs-ops.ts:60-63): lowercase extension
// lookup with octet-stream fallback.
func getMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if t, ok := mimeMap[ext]; ok {
		return t
	}
	return "application/octet-stream"
}

// formatMtime mirrors stat.mtime.toISOString() (fs-ops.ts): millisecond
// precision, UTC "Z". The ".000" forces exactly 3 fractional digits (fixed
// width, like JS millis); "Z07:00" renders "Z" for UTC. Not RFC3339 (no
// millis) and not RFC3339Nano (variable-width nanos).
func formatMtime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// Ls mirrors ls (fs-ops.ts:65-90). Returns (result, "", nil) on success;
// (zero, ErrCode, nil) on a known wire error; (zero, ErrCode(err.Error()),
// nil) for an unexpected non-ENOENT error. The Go error return is reserved
// and currently always nil.
func Ls(allowed []string, target string) (LsResult, ErrCode, error) {
	if !pathAllowed(target, allowed) {
		return LsResult{}, ErrPathNotAllowed, nil
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LsResult{}, ErrPathNotFound, nil
		}
		return LsResult{}, ErrCode(err.Error()), nil
	}
	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		entry := DirEntry{Name: name, IsDirectory: e.IsDir()}
		// Best-effort per-entry stat via os.Stat(filepath.Join(...)) to
		// mirror fs-ops.ts's statSync(fullPath) (follows symlinks). A
		// per-entry stat error (e.g. a broken symlink) is swallowed →
		// Size 0 / Mtime "" (fs-ops.ts:76-80 try{}catch{}).
		if info, statErr := os.Stat(filepath.Join(target, name)); statErr == nil {
			entry.Size = info.Size()
			entry.Mtime = formatMtime(info.ModTime())
		}
		out = append(out, entry)
	}
	return LsResult{Entries: out}, "", nil
}

// Read mirrors readFile (fs-ops.ts:92-112). Whole-file UTF-8 read; invalid
// UTF-8 bytes are carried lossily via string(b) (no error, no utf8.Valid
// gate). Order: containment → stat → IsDir → Size > Max → ReadFile.
func Read(allowed []string, target string) (ReadResult, ErrCode, error) {
	if !pathAllowed(target, allowed) {
		return ReadResult{}, ErrPathNotAllowed, nil
	}
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ReadResult{}, ErrFileNotFound, nil
		}
		return ReadResult{}, ErrCode(err.Error()), nil
	}
	if info.IsDir() {
		return ReadResult{}, ErrIsDirectory, nil
	}
	if info.Size() > MaxFileSize {
		return ReadResult{}, ErrFileTooLarge, nil
	}
	b, err := os.ReadFile(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ReadResult{}, ErrFileNotFound, nil
		}
		return ReadResult{}, ErrCode(err.Error()), nil
	}
	return ReadResult{Content: string(b), MimeType: getMimeType(target), Size: info.Size()}, "", nil
}

// Stat mirrors stat (fs-ops.ts:114-127).
func Stat(allowed []string, target string) (StatResult, ErrCode, error) {
	if !pathAllowed(target, allowed) {
		return StatResult{}, ErrPathNotAllowed, nil
	}
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StatResult{}, ErrPathNotFound, nil
		}
		return StatResult{}, ErrCode(err.Error()), nil
	}
	return StatResult{Size: info.Size(), Mtime: formatMtime(info.ModTime()), IsDirectory: info.IsDir()}, "", nil
}
