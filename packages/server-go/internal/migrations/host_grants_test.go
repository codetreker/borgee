package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runHB31 runs migration v=27 (HB-3.1 host_grants) on a memory DB.
// 跟 al_2a_1_agent_configs_test runAL2A1 同模式; SQLite FK off, 不 seed
// users 上游表 (logical FK).
func runHB31(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(hostGrants)
	if err := e.Run(0); err != nil {
		t.Fatalf("run hb_3_1: %v", err)
	}
}

// TestHB_CreatesHostGrantsTable pins acceptance §1.1 — 表 9 列 byte-
// identical 跟 hb-3-spec.md §1 BPP-3.1 + stance §1 设计 ① schema SSOT.
func TestHB_CreatesHostGrantsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runHB31(t, db)

	rows, err := db.Raw(`PRAGMA table_info(host_grants)`).Rows()
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer rows.Close()
	type col struct {
		name    string
		ctype   string
		notnull int
		pk      int
	}
	var cols []col
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    *string
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols = append(cols, col{name, ctype, notnull, pk})
	}
	want := map[string]struct {
		typ     string
		notnull int
		pk      int
	}{
		"id":         {"TEXT", 0, 1}, // SQLite: PK columns notnull=0 in PRAGMA, NOT NULL implied by PK
		"user_id":    {"TEXT", 1, 0},
		"agent_id":   {"TEXT", 0, 0},
		"grant_type": {"TEXT", 1, 0},
		"scope":      {"TEXT", 1, 0},
		"ttl_kind":   {"TEXT", 1, 0},
		"granted_at": {"INTEGER", 1, 0},
		"expires_at": {"INTEGER", 0, 0},
		"revoked_at": {"INTEGER", 0, 0},
	}
	if len(cols) != len(want) {
		t.Fatalf("column count drift: got %d, want %d (HB-3.1 schema 9 列)",
			len(cols), len(want))
	}
	for _, c := range cols {
		w, ok := want[c.name]
		if !ok {
			t.Errorf("unexpected column %q", c.name)
			continue
		}
		if c.ctype != w.typ {
			t.Errorf("column %q type=%q, want %q", c.name, c.ctype, w.typ)
		}
		if c.notnull != w.notnull {
			t.Errorf("column %q notnull=%d, want %d", c.name, c.notnull, w.notnull)
		}
		if c.pk != w.pk {
			t.Errorf("column %q pk=%d, want %d", c.name, c.pk, w.pk)
		}
	}
}

// TestHB_GrantTypeEnumReject pins acceptance §1.2 — CHECK constraint
// rejects 4-enum 外值 (跟蓝图 §1.3 字面 byte-identical: install/exec/
// filesystem/network).
func TestHB_GrantTypeEnumReject(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runHB31(t, db)
	for _, bad := range []string{"admin", "sudo", "root", "system", ""} {
		err := db.Exec(`INSERT INTO host_grants
			(id, user_id, grant_type, scope, ttl_kind, granted_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			"id-"+bad, "u1", bad, "{}", "always", 1).Error
		if err == nil {
			t.Errorf("CHECK should reject grant_type=%q (4-enum: install/exec/filesystem/network)", bad)
		}
	}
	// Sanity: 4 valid enum 全过.
	for _, good := range []string{"install", "exec", "filesystem", "network"} {
		err := db.Exec(`INSERT INTO host_grants
			(id, user_id, grant_type, scope, ttl_kind, granted_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			"id-"+good, "u1", good, "{}", "always", 1).Error
		if err != nil {
			t.Errorf("valid grant_type=%q rejected: %v", good, err)
		}
	}
}

// TestHB_TtlKindEnumReject pins acceptance §1.2 + content-lock §1.② —
// ttl_kind 2-enum CHECK (one_shot/always 跟弹窗 UX 字面 byte-identical).
func TestHB_TtlKindEnumReject(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runHB31(t, db)
	for _, bad := range []string{"once", "forever", "permanent", "transient", ""} {
		err := db.Exec(`INSERT INTO host_grants
			(id, user_id, grant_type, scope, ttl_kind, granted_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			"id-"+bad, "u1", "filesystem", "{}", bad, 1).Error
		if err == nil {
			t.Errorf("CHECK should reject ttl_kind=%q (2-enum: one_shot/always)", bad)
		}
	}
}

// TestHB_NoDomainBleed pins stance §2 设计 ② 字典分立 (host vs runtime),
// 反向断言 schema 不挂 user_permissions / runtime / cursor / org_id 等
// 跨域字段 (跟 al_2a_1_agent_configs_test::TestAL2A1_NoDomainBleed 同模式).
func TestHB_NoDomainBleed(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runHB31(t, db)
	// 反向断言 forbidden columns (字典分立守门).
	forbidden := []string{
		"permission",         // user_permissions schema 复用反断
		"is_admin",           // admin god-mode 反断 (ADM-0 §1.3)
		"cursor",             // RT-1 envelope cursor 拆死
		"org_id",             // org 隔离走 users.org_id 单源
		"source",             // BPP 单源跟 AL-1b 同模式
		"set_by",             // 反人工伪造
		"runtime_id",         // host vs runtime 字典分立
	}
	type col struct{ Name string }
	var cols []col
	rows, err := db.Raw(`PRAGMA table_info(host_grants)`).Rows()
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    *string
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols = append(cols, col{Name: name})
	}
	have := map[string]bool{}
	for _, c := range cols {
		have[c.Name] = true
	}
	for _, f := range forbidden {
		if have[f] {
			t.Errorf("forbidden column %q present (字典分立反约束 — host vs runtime)", f)
		}
	}
}

// TestHB_HasIndexes pins acceptance §1.1 idx_user_id + idx_agent_id 守
// (cross-user 403 ACL + daemon SELECT 热路径).
func TestHB_HasIndexes(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runHB31(t, db)
	type idx struct{ Name string }
	var idxs []idx
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='host_grants'`).Scan(&idxs).Error; err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	want := map[string]bool{"idx_host_grants_user_id": false, "idx_host_grants_agent_id": false}
	for _, i := range idxs {
		if _, ok := want[i.Name]; ok {
			want[i.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing index %q", name)
		}
	}
}

// TestHostGrants_Idempotent pins forward-only stance — re-running the migration
// must not error (IF NOT EXISTS guards). 跟 al_2a_1_agent_configs_test
// idempotency 同模式.
func TestHostGrants_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runHB31(t, db)
	// Run again via fresh engine; should be no-op.
	e := New(db)
	e.Register(hostGrants)
	if err := e.Run(0); err != nil {
		t.Errorf("re-run should be idempotent (forward-only stance): %v", err)
	}
}

// TestHB_VersionIs27 pins registry sequencing (HB-3.1 = v=27, after
// AL-1.4 v=25 + DL-4.1 v=26).
func TestHB_VersionIs27(t *testing.T) {
	t.Parallel()
	if hostGrants.Version != 27 {
		t.Errorf("hostGrants.Version=%d, want 27 (sequencing 跟 spec brief §1 byte-identical)", hostGrants.Version)
	}
}
