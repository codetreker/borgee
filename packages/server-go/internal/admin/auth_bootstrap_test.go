package admin_test

import (
	"fmt"
	"sync"
	"testing"

	"borgee-server/internal/admin"
	"borgee-server/internal/migrations"
	tdb "borgee-server/internal/testutil/db"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var hashAtCache struct {
	sync.Mutex
	values map[string]string
}

// hashAt returns a bcrypt hash for plain at the given cost. Used to satisfy
// review checklist invariant "bcrypt cost ≥ 10" (the package's MinBcryptCost).
func hashAt(t *testing.T, plain string, cost int) string {
	t.Helper()
	key := fmt.Sprintf("%d:%s", cost, plain)
	hashAtCache.Lock()
	if hashAtCache.values != nil {
		if h, ok := hashAtCache.values[key]; ok {
			hashAtCache.Unlock()
			return h
		}
	} else {
		hashAtCache.values = make(map[string]string)
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	if err != nil {
		hashAtCache.Unlock()
		t.Fatalf("bcrypt: %v", err)
	}
	hashAtCache.values[key] = string(h)
	hashAtCache.Unlock()
	return string(h)
}

// openMigratedDB returns a fresh in-memory DB with the migrations engine
// applied through the full registry. The engine's earlier migrations (CM-1.1,
// AP-0-bis, ...) ALTER `users` / `channels` / `messages` / `workspace_files` /
// `remote_nodes`, and AP-0-bis (v=8) reads users.role / users.deleted_at and
// inserts into user_permissions, so we seed those Phase-0 tables and
// columns first — same pattern the migrations package's own tests use
// (see seedLegacyTables in cm_1_1_organizations_test.go).
func openMigratedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := tdb.Open(t)
	for _, name := range []string{"channels", "messages", "workspace_files", "remote_nodes"} {
		if err := db.Exec("CREATE TABLE " + name + " (id TEXT PRIMARY KEY)").Error; err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	// users needs role + deleted_at for AP-0-bis backfill predicate.
	if err := db.Exec(`CREATE TABLE users (
  id         TEXT PRIMARY KEY,
  role       TEXT,
  deleted_at INTEGER
)`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := db.Exec(`CREATE TABLE user_permissions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     TEXT NOT NULL,
  permission  TEXT NOT NULL,
  scope       TEXT NOT NULL,
  granted_at  INTEGER NOT NULL
)`).Error; err != nil {
		t.Fatalf("seed user_permissions: %v", err)
	}
	if err := migrations.Default(db).Run(0); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

// TestBootstrap_1A_PanicsOnMissingEnv covers review checklist invariant 1.A:
// "env 未设 → server 启动 fail-loud (panic with clear message)".
//
// We exercise BootstrapWith directly so we don't have to manipulate process
// env state in tests. cmd/collab/main.go calls Bootstrap which delegates to
// BootstrapWith using os.Getenv — equivalent fail-loud behavior.
func TestBootstrap_1A_PanicsOnMissingEnv(t *testing.T) {
	t.Parallel()
	db := openMigratedDB(t)

	t.Run("missing login", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on missing BORGEE_ADMIN_LOGIN")
			}
		}()
		_ = admin.BootstrapWith(db, "", hashAt(t, "pw", 10), "")
	})

	t.Run("missing password hash", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on missing BORGEE_ADMIN_PASSWORD_HASH")
			}
		}()
		_ = admin.BootstrapWith(db, "root", "", "")
	})

	t.Run("rejects non-bcrypt hash", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on non-bcrypt hash")
			}
		}()
		_ = admin.BootstrapWith(db, "root", "plain-text-not-bcrypt", "")
	})

	t.Run("rejects bcrypt cost < 10", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on bcrypt cost < 10")
			}
		}()
		_ = admin.BootstrapWith(db, "root", hashAt(t, "pw", 4), "")
	})
}

// TestBootstrap_1B_Idempotent covers review checklist invariant 1.B:
// "env 设了相同 login 重启 server → admins 表不重复插".
//
// Run BootstrapWith twice with the same login; admins row count must equal 1.
func TestBootstrap_1B_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMigratedDB(t)

	if err := admin.BootstrapWith(db, "root", hashAt(t, "secret", 10), ""); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	if err := admin.BootstrapWith(db, "root", hashAt(t, "secret", 10), ""); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}

	var n int64
	if err := db.Table("admins").Where("login = ?", "root").Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("admins WHERE login='root' count = %d, want 1 (idempotent)", n)
	}
}

// TestBootstrap_PlainEnv covers ADMIN-PASSWORD-PLAIN-ENV B 方案 — when only
// BORGEE_ADMIN_PASSWORD (plain) is set, server hashes it at startup using
// MinBcryptCost and inserts into admins table; verify path uses the stored
// hash byte-identical (反向断 PlainEnv 不污染 password 字面比较 path).
func TestBootstrap_PlainEnv(t *testing.T) {
	t.Parallel()
	db := openMigratedDB(t)

	if err := admin.BootstrapWith(db, "root", "", "Test@Plain2026"); err != nil {
		t.Fatalf("plain bootstrap: %v", err)
	}

	// Row inserted, password_hash must be a valid bcrypt with cost ≥ MinBcryptCost.
	var stored string
	if err := db.Raw(`SELECT password_hash FROM admins WHERE login=?`, "root").Row().Scan(&stored); err != nil {
		t.Fatalf("read stored hash: %v", err)
	}
	cost, err := bcrypt.Cost([]byte(stored))
	if err != nil {
		t.Fatalf("stored value is not bcrypt: %v (got %q)", err, stored)
	}
	if cost < admin.MinBcryptCost {
		t.Errorf("stored bcrypt cost %d < MinBcryptCost %d", cost, admin.MinBcryptCost)
	}
	// Verify the stored hash matches the original plain text.
	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte("Test@Plain2026")); err != nil {
		t.Errorf("CompareHashAndPassword failed: %v", err)
	}
}

// TestBootstrap_BothEnvSet_Panics covers spec §0.4 — both hash + plain set
// is mutually exclusive (反 surprise / 反 silent priority).
func TestBootstrap_BothEnvSet_Panics(t *testing.T) {
	t.Parallel()
	db := openMigratedDB(t)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when both hash + plain set")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value not string: %v", r)
		}
		// Must mention both env names (反 vague error).
		if !contains(msg, admin.EnvAdminPasswordHash) || !contains(msg, admin.EnvAdminPassword) {
			t.Errorf("panic message %q must mention both %s and %s",
				msg, admin.EnvAdminPasswordHash, admin.EnvAdminPassword)
		}
	}()
	_ = admin.BootstrapWith(db, "root", hashAt(t, "pw", 10), "Test@Plain2026")
}

// TestBootstrap_HashPriority_BackwardCompat covers spec §0.3 invariant —
// existing BORGEE_ADMIN_PASSWORD_HASH path unchanged when plain is empty.
func TestBootstrap_HashPriority_BackwardCompat(t *testing.T) {
	t.Parallel()
	db := openMigratedDB(t)
	storedHash := hashAt(t, "secret", 10)
	if err := admin.BootstrapWith(db, "root", storedHash, ""); err != nil {
		t.Fatalf("legacy bootstrap: %v", err)
	}
	var got string
	if err := db.Raw(`SELECT password_hash FROM admins WHERE login=?`, "root").Row().Scan(&got); err != nil {
		t.Fatalf("read stored: %v", err)
	}
	if got != storedHash {
		t.Errorf("stored hash mismatch — expected byte-identical legacy hash; got %q want %q", got, storedHash)
	}
}

// TestBootstrap_NeitherEnv_Panics covers spec §0.4 — at least one of hash /
// plain must be set; both empty → panic with helpful message.
func TestBootstrap_NeitherEnv_Panics(t *testing.T) {
	t.Parallel()
	db := openMigratedDB(t)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on neither env set")
		}
		msg, _ := r.(string)
		if !contains(msg, admin.EnvAdminPasswordHash) || !contains(msg, admin.EnvAdminPassword) {
			t.Errorf("panic message must mention both env names; got %q", msg)
		}
	}()
	_ = admin.BootstrapWith(db, "root", "", "")
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()))
}
