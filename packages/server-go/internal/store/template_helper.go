package store

// TEST-FIX-3-COV: shared migrated DB template helper.
//
// 真因优化: 每 test 调 store.Open(":memory:") + s.Migrate() = ~26ms / 12k
// allocs. internal/api 750+ tests 跑下来 ~19.5s 重复 migrate, 占 36s 总时间
// >50%. 本 helper 把 Migrate 一次性跑到 file-backed template.db, 之后每
// test 只 io.Copy template.db → fresh.db (1.24ms / 461 allocs, ~20x 加速).
//
// 设计:
//   - 0 production code 改 (本文件 _test.go scoped, 仅 test path 用)
//   - byte-identical schema (template 跑同 Migrate, 0 schema drift)
//   - 每 test 独立 DB (clone file, sqlite 单进程 file backed, 互不干扰)
//   - sync.Once 跨 test 跨 package 共享 template (init 摊 1 次 ~26ms)
//
// 使用 (跟既有 testStore + Migrate 同精神, 减重复):
//
//	s := store.MigratedStoreFromTemplate(t)  // 1.24ms 替代 26ms
//
// 跟 :memory: 路径行为 byte-identical (PRAGMA / FK / busy_timeout 全同源,
// 走 store.Open(file://...) 同 ctor).

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var (
	templateDBOnce sync.Once
	templateDBPath string
	templateDBDir  string
	templateDBErr  error
)

// ensureTemplateDB does the one-time Migrate into a file-backed template
// stored under os.TempDir(). Subsequent test sessions reuse the in-process
// path (template lives until the test binary exits, t.Cleanup not used —
// process-scoped sync.Once).
func ensureTemplateDB() (string, error) {
	templateDBOnce.Do(func() {
		dir, err := os.MkdirTemp("", "borgee-store-template-*")
		if err != nil {
			templateDBErr = err
			return
		}
		templateDBDir = dir
		path := filepath.Join(dir, "template.db")
		s, err := Open(path)
		if err != nil {
			templateDBErr = err
			return
		}
		if err := s.Migrate(); err != nil {
			s.Close()
			templateDBErr = err
			return
		}
		// Close template so file is consistent on disk for io.Copy.
		if err := s.Close(); err != nil {
			templateDBErr = err
			return
		}
		templateDBPath = path
	})
	return templateDBPath, templateDBErr
}

// MigratedStoreFromTemplate returns a freshly migrated *Store backed by a
// per-test sqlite file cloned from the shared template DB. The file is
// auto-cleaned via t.Cleanup. Use this in lieu of `Open(":memory:") +
// Migrate()` to skip the ~26ms migrate per test.
//
// Behavior matches `Open(file_path) + Migrate()` byte-identically — same
// schema, same PRAGMAs (FK on, busy_timeout 5000, WAL for file-backed).
//
// The template is initialized exactly once per test process (sync.Once) by
// running the real Migrate(), so test schema stays in lockstep with
// production migrations — no schema drift between this helper and the
// legacy `testStore + Migrate` path.
func MigratedStoreFromTemplate(t testing.TB) *Store {
	t.Helper()
	srcPath, err := ensureTemplateDB()
	if err != nil {
		t.Fatalf("template db init: %v", err)
	}

	dstDir, err := os.MkdirTemp("", "borgee-store-clone-*")
	if err != nil {
		t.Fatalf("clone dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dstDir) })
	dstPath := filepath.Join(dstDir, "store.db")

	in, err := os.Open(srcPath)
	if err != nil {
		t.Fatalf("template open: %v", err)
	}
	defer in.Close()
	out, err := os.Create(dstPath)
	if err != nil {
		t.Fatalf("clone create: %v", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatalf("clone copy: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("clone close: %v", err)
	}

	s, err := Open(dstPath)
	if err != nil {
		t.Fatalf("open cloned: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
