// Package fileio - HB-2 v0(D) real IO actions (read_file / list_files).
// It replaces the v0(C) ACL-only placeholder. On Linux, Landlock LSM already
// limits access to the allowed path list, so out-of-scope reads fail at open()
// with EACCES. This layer only enforces max_bytes and JSON-friendly
// serialization.
//
// hb-2-v0d-spec.md §0.2: read_file uses real file reads with a max_bytes
// limit; list_files uses os.ReadDir with an entry-count limit.
//
// Read-only invariant: grep must find no `os\.WriteFile|os\.Create|os\.Remove` matches
// in this package. This is a read-only domain; write attempts are rejected by
// the ACL layer and Landlock.

package fileio

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ReadFileResult is the result returned by the read_file action.
type ReadFileResult struct {
	Bytes     []byte `json:"bytes"`     // raw file content, capped by max_bytes
	Truncated bool   `json:"truncated"` // true when max_bytes capped the content
	Size      int64  `json:"size"`      // real file size; caller decides whether to retry
}

// ListFilesResult is the result returned by the list_files action.
type ListFilesResult struct {
	Entries   []DirEntry `json:"entries"`
	Truncated bool       `json:"truncated"` // true when max_entries capped the list
}

// DirEntry is one list_files result entry.
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// MaxReadBytes is the per-call read_file limit, preventing excessive daemon memory use.
const MaxReadBytes = 16 * 1024 * 1024 // 16 MiB

// MaxListEntries is the per-call list_files entry limit.
const MaxListEntries = 1000

// ErrPathDenied means the sandbox or Landlock denied the path.
var ErrPathDenied = errors.New("path denied by sandbox")

// ReadFile reads an absolute path. The caller must already have passed the ACL
// check; this layer does not repeat it. max_bytes 0 uses MaxReadBytes, and any
// value above MaxReadBytes is capped to MaxReadBytes.
func ReadFile(path string, maxBytes int64) (*ReadFileResult, error) {
	if maxBytes == 0 || maxBytes > MaxReadBytes {
		maxBytes = MaxReadBytes
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: %v", ErrPathDenied, err)
		}
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("read_file: %q is a directory", path)
	}
	size := stat.Size()
	limit := maxBytes
	if size < limit {
		limit = size
	}
	buf := make([]byte, limit)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && err != io.EOF {
		return nil, err
	}
	return &ReadFileResult{
		Bytes:     buf[:n],
		Truncated: size > maxBytes,
		Size:      size,
	}, nil
}

// ListFiles reads a directory. The caller must already have passed the ACL check.
func ListFiles(path string) (*ListFilesResult, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: %v", ErrPathDenied, err)
		}
		return nil, err
	}
	limit := len(entries)
	truncated := false
	if limit > MaxListEntries {
		limit = MaxListEntries
		truncated = true
	}
	out := make([]DirEntry, 0, limit)
	for i := 0; i < limit; i++ {
		e := entries[i]
		info, err := e.Info()
		if err != nil {
			// Skip stat-failed entries (e.g. broken symlink); not fatal.
			continue
		}
		out = append(out, DirEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  info.Size(),
		})
	}
	return &ListFilesResult{Entries: out, Truncated: truncated}, nil
}
