// Package fileio — HB-2 v0(D) 真 IO actions (read_file / list_files).
// 替代 v0(C) 仅 ACL 决策 stub. landlock LSM 守门 (Linux) 已限路径白名单,
// 越界 read open() 真 EACCES; 此层仅做 max_bytes 限 + JSON-friendly 序列化.
//
// hb-2-v0d-spec.md §0.2: read_file 真走 os.ReadFile (max_bytes 限);
// list_files 真走 os.ReadDir (entry 数限).
//
// 反向约束: grep 检查 `os\.WriteFile|os\.Create|os\.Remove` 在本包 0 hit
// (read-only domain, 写类 100% reject 由 ACL 层 + landlock 双守).

package fileio

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ReadFileResult is read_file action 返回数据.
type ReadFileResult struct {
	Bytes     []byte `json:"bytes"`     // raw file content (max_bytes 截断)
	Truncated bool   `json:"truncated"` // 是否被 max_bytes 截断
	Size      int64  `json:"size"`      // 真文件大小 (caller 决定要不要重读)
}

// ListFilesResult is list_files action 返回数据.
type ListFilesResult struct {
	Entries   []DirEntry `json:"entries"`
	Truncated bool       `json:"truncated"` // 是否被 max_entries 截断
}

// DirEntry 是 list_files 单条记录.
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// MaxReadBytes 是 read_file 单 call 上限 (反 DoS, daemon 内存膨胀).
const MaxReadBytes = 16 * 1024 * 1024 // 16 MiB

// MaxListEntries 是 list_files 单 call 上限.
const MaxListEntries = 1000

// ErrPathDenied — 路径被 sandbox/landlock reject (caller 走 IO_FAILED reason).
var ErrPathDenied = errors.New("path denied by sandbox")

// ReadFile 真读 absolute path. caller 应保证 ACL gate 已 pass (此层不重做 ACL).
// max_bytes 0 表用 MaxReadBytes default; 超过 MaxReadBytes 的 max_bytes 也被
// 截到 MaxReadBytes (反 caller 绕过限).
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

// ListFiles 真读 directory. caller 应保证 ACL gate 已 pass.
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
