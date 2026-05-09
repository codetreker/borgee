package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runCV11 runs migration v=13 (CV-1.1) on a memory DB. v=13 is a clean
// CREATE — no upstream tables required (channel_id is a logical FK, not
// enforced by SQLite default), so we don't seed anything.
func runCV11(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(artifacts)
	if err := e.Run(0); err != nil {
		t.Fatalf("run cv_1_1: %v", err)
	}
}

// TestCV_CreatesArtifactsTable pins acceptance §1 (cv-1.md §1.1):
// artifacts has the contract columns with the right NOT NULL / nullable
// shape. Drift here breaks workspace tab list correctness or schema
// equivalence with the CV-1.2 server API.
func TestCV_CreatesArtifactsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV11(t, db)

	cols := pragmaColumns(t, db, "artifacts")
	if len(cols) == 0 {
		t.Fatal("artifacts table not created")
	}

	idCol, ok := cols["id"]
	if !ok {
		t.Fatalf("artifacts missing id (have %v)", keys(cols))
	}
	if !idCol.pk {
		t.Error("artifacts.id must be PRIMARY KEY")
	}

	for _, name := range []string{"channel_id", "type", "title", "body", "current_version", "created_at"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("artifacts missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("artifacts.%s must be NOT NULL", name)
		}
	}

	archivedAt, ok := cols["archived_at"]
	if !ok {
		t.Fatalf("artifacts missing archived_at (have %v)", keys(cols))
	}
	if archivedAt.notNull {
		t.Error("artifacts.archived_at must be nullable (软删, 蓝图 §2)")
	}

	// 设计 ② 单文档锁 30s TTL — lock_holder_user_id + lock_acquired_at 必须
	// 存在且 nullable (NULL = 无人持锁可写; CV-1.2 PATCH conflict 409 路径).
	for _, name := range []string{"lock_holder_user_id", "lock_acquired_at"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("artifacts missing %q (设计 ② 单文档锁) — have %v", name, keys(cols))
		}
		if c.notNull {
			t.Errorf("artifacts.%s must be nullable (NULL = 无人持锁)", name)
		}
	}

	// 反约束: owner_id MUST NOT exist (设计 ① 归属=channel, 非 author).
	if _, has := cols["owner_id"]; has {
		t.Error("artifacts.owner_id exists — 反约束 broken (设计 ① channel-scoped, no author owner)")
	}
	// 反约束: cursor MUST NOT exist (跟 RT-1 cursor 序列拆死,
	// ArtifactUpdated frame 走 RT-1.1 cursor 不在 schema 层).
	if _, has := cols["cursor"]; has {
		t.Error("artifacts.cursor exists — 反约束 broken (RT-1 envelope cursor, not schema column)")
	}
}

// TestCV_RejectsNonMarkdownType pins 设计 ④: type CHECK = 'markdown'
// is the v1 gate — 代码/图片/PDF/看板 留 v2+. Insert with any other type
// must reject so the v0/v1 split stays enforced at schema layer.
func TestCV_RejectsNonMarkdownType(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV11(t, db)

	insert := func(typ string) error {
		return db.Exec(`INSERT INTO artifacts
			(id, channel_id, type, title, body, current_version, created_at)
			VALUES (?, 'chan-A', ?, 'T', '', 1, 1700000000000)`,
			"art-"+typ, typ).Error
	}

	// markdown allowed
	if err := insert("markdown"); err != nil {
		t.Fatalf("markdown insert rejected: %v", err)
	}
	// non-markdown rejected
	for _, bad := range []string{"code", "image", "pdf", "kanban"} {
		if err := insert(bad); err == nil {
			t.Errorf("type=%q accepted — CHECK ('markdown') missing", bad)
		}
	}
}

// TestCV_CreatesArtifactVersionsTable pins acceptance §1.1 second-table
// contract: artifact_versions captures committer_kind ('agent','human')
// for 设计 ⑥ system message 路径 + UNIQUE(artifact_id, version) for 设计 ③
// 版本线性.
func TestCV_CreatesArtifactVersionsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV11(t, db)

	cols := pragmaColumns(t, db, "artifact_versions")
	if len(cols) == 0 {
		t.Fatal("artifact_versions table not created")
	}
	for _, name := range []string{"artifact_id", "version", "body", "committer_kind", "committer_id", "created_at"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("artifact_versions missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("artifact_versions.%s must be NOT NULL", name)
		}
	}

	// 设计 ⑦ rollback 路径 — rolled_back_from_version 必须存在且 nullable
	// (NULL = 普通 commit; 非 NULL = rollback 触发的新 commit 记录原 version).
	rb, ok := cols["rolled_back_from_version"]
	if !ok {
		t.Fatalf("artifact_versions missing rolled_back_from_version (设计 ⑦) — have %v", keys(cols))
	}
	if rb.notNull {
		t.Error("artifact_versions.rolled_back_from_version must be nullable (NULL = 普通 commit)")
	}
}

// TestCV_VersionsTablePKMonotonic pins artifact_versions.id PK
// AUTOINCREMENT — global strictly increasing across all artifacts. This
// is a *different* invariant from UNIQUE(artifact_id, version):
//   - PK id: cross-artifact 全局 audit 序 (每行一次性)
//   - UNIQUE(artifact_id, version): per-artifact 线性版本号 (业务序)
// CV-1.2 commit 路径需要全局 PK 单调供 audit log + cursor stub 复用 (虽然
// CV-1.2 ArtifactUpdated frame 走 RT-1.1 cursor, 不是 PK; 但 PK 单调是
// SQLite AUTOINCREMENT 显式契约, drift 会破坏 backfill 排序假设).
func TestCV_VersionsTablePKMonotonic(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV11(t, db)

	insert := func(art string, version int) error {
		return db.Exec(`INSERT INTO artifact_versions
			(artifact_id, version, body, committer_kind, committer_id, created_at)
			VALUES (?, ?, '', 'human', 'user-1', 1700000000000)`,
			art, version).Error
	}
	// Interleave artifacts to prove PK is global, not per-artifact.
	if err := insert("art-A", 1); err != nil {
		t.Fatalf("art-A v1: %v", err)
	}
	if err := insert("art-B", 1); err != nil {
		t.Fatalf("art-B v1: %v", err)
	}
	if err := insert("art-A", 2); err != nil {
		t.Fatalf("art-A v2: %v", err)
	}
	if err := insert("art-B", 2); err != nil {
		t.Fatalf("art-B v2: %v", err)
	}

	type row struct {
		ID         int64  `gorm:"column:id"`
		ArtifactID string `gorm:"column:artifact_id"`
		Version    int    `gorm:"column:version"`
	}
	var rows []row
	if err := db.Raw(`SELECT id, artifact_id, version FROM artifact_versions ORDER BY id ASC`).Scan(&rows).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].ID <= rows[i-1].ID {
			t.Errorf("PK not strictly increasing at row %d: %d after %d (artifact_id %q v%d)",
				i, rows[i].ID, rows[i-1].ID, rows[i].ArtifactID, rows[i].Version)
		}
	}
	// Insertion order: art-A v1, art-B v1, art-A v2, art-B v2.
	// PK must reflect insertion order (interleaved across artifacts).
	wantOrder := []struct {
		art string
		ver int
	}{{"art-A", 1}, {"art-B", 1}, {"art-A", 2}, {"art-B", 2}}
	for i, w := range wantOrder {
		if rows[i].ArtifactID != w.art || rows[i].Version != w.ver {
			t.Errorf("row %d: got (%s, v%d), want (%s, v%d)",
				i, rows[i].ArtifactID, rows[i].Version, w.art, w.ver)
		}
	}
}

// TestCV_RejectsInvalidCommitterKind pins 设计 ⑥: committer_kind
// CHECK in ('agent','human'). Drift here breaks the agent-commit fanout
// system message routing.
func TestCV_RejectsInvalidCommitterKind(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV11(t, db)

	insert := func(kind string, version int) error {
		return db.Exec(`INSERT INTO artifact_versions
			(artifact_id, version, body, committer_kind, committer_id, created_at)
			VALUES ('art-A', ?, '', ?, 'committer-1', 1700000000000)`,
			version, kind).Error
	}
	if err := insert("agent", 1); err != nil {
		t.Fatalf("agent insert rejected: %v", err)
	}
	if err := insert("human", 2); err != nil {
		t.Fatalf("human insert rejected: %v", err)
	}
	for _, bad := range []string{"admin", "system", "bot", ""} {
		if err := insert(bad, 99); err == nil {
			t.Errorf("committer_kind=%q accepted — CHECK ('agent','human') missing", bad)
		}
	}
}

// TestCV_RejectsDuplicateArtifactVersion pins 设计 ③ 版本线性:
// UNIQUE(artifact_id, version) enforces strictly increasing version per
// artifact. CV-1.2 commit 路径必须 transactional bump current_version
// + insert new artifact_versions row; dup → reject.
func TestCV_RejectsDuplicateArtifactVersion(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV11(t, db)

	insert := func(version int) error {
		return db.Exec(`INSERT INTO artifact_versions
			(artifact_id, version, body, committer_kind, committer_id, created_at)
			VALUES ('art-A', ?, '', 'human', 'user-1', 1700000000000)`,
			version).Error
	}
	if err := insert(1); err != nil {
		t.Fatalf("first version: %v", err)
	}
	if err := insert(2); err != nil {
		t.Fatalf("second version: %v", err)
	}
	if err := insert(2); err == nil {
		t.Fatal("duplicate (artifact_id, version) accepted — UNIQUE constraint missing")
	}
	// Different artifact, same version is legal.
	other := db.Exec(`INSERT INTO artifact_versions
		(artifact_id, version, body, committer_kind, committer_id, created_at)
		VALUES ('art-B', 1, '', 'human', 'user-1', 1700000000000)`).Error
	if other != nil {
		t.Fatalf("cross-artifact same version rejected: %v", other)
	}
}

// TestCanvasArtifacts_HasIndexes pins acceptance §1.1 — channel-scoped list +
// version sidebar lookup require both indexes.
func TestCanvasArtifacts_HasIndexes(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV11(t, db)

	for _, idx := range []string{
		"idx_artifacts_channel_id",
		"idx_artifact_versions_artifact_id",
	} {
		var name string
		err := db.Raw(`SELECT name FROM sqlite_master
			WHERE type='index' AND name=?`, idx).Scan(&name).Error
		if err != nil || name != idx {
			t.Errorf("missing index %s (got %q, err=%v)", idx, name, err)
		}
	}
}

// TestCanvasArtifacts_Idempotent pins forward-only safety: re-running v=13 is no-op.
func TestCanvasArtifacts_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV11(t, db)
	e := New(db)
	e.Register(artifacts)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run cv_1_1: %v", err)
	}
}
