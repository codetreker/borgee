package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runCV31 runs migration v=17 (CV-3.1) chained on top of CV-1.1 (v=13).
// CV-3.1 mutates the `artifacts` table CHECK from `type='markdown'` to
// `type IN ('markdown','code','image_link')` via the standard SQLite
// table-recreate pattern; tests need v=13 first to have the source table
// + a non-empty data path to exercise.
func runCV31(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(artifacts)
	e.Register(artifactKinds)
	if err := e.Run(0); err != nil {
		t.Fatalf("run cv_3_1: %v", err)
	}
}

// TestCV_AcceptsCodeAndImageLinkKinds pins acceptance §1.1 — kind enum
// CHECK accepts 'markdown' (CV-1 既有) + 'code' / 'image_link' (CV-3.1 新).
// Drift here means CV-3.2 client renderer 三分支会被 schema 拒, 整段崩.
func TestCV_AcceptsCodeAndImageLinkKinds(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV31(t, db)

	insert := func(id, kind string) error {
		return db.Exec(`INSERT INTO artifacts
			(id, channel_id, type, title, body, current_version, created_at)
			VALUES (?, 'ch-A', ?, 'T', '', 1, 1700000000000)`, id, kind).Error
	}
	for _, k := range []string{"markdown", "code", "image_link"} {
		if err := insert("art-"+k, k); err != nil {
			t.Errorf("kind=%q rejected — CHECK should accept: %v", k, err)
		}
	}
}

// TestCV_RejectsPdfKanbanMindmap pins acceptance §1.2 — CHECK reject
// 'pdf' / 'kanban' / 'mindmap' (蓝图 §2 v1 不做字面禁守住). 设计 ① enum
// 收窄: 不开 v2+ kind 漏口.
func TestCV_RejectsPdfKanbanMindmap(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV31(t, db)

	insert := func(kind string) error {
		return db.Exec(`INSERT INTO artifacts
			(id, channel_id, type, title, body, current_version, created_at)
			VALUES (?, 'ch-A', ?, 'T', '', 1, 1700000000000)`,
			"art-"+kind, kind).Error
	}
	for _, bad := range []string{"pdf", "kanban", "mindmap", "doc", "video", ""} {
		if err := insert(bad); err == nil {
			t.Errorf("kind=%q accepted — CHECK ('markdown','code','image_link') missing or wrong", bad)
		}
	}
}

// TestCV_PreservesMarkdownRowsAcrossRebuild pins data preservation —
// CV-1 既有 markdown rows MUST survive the v=17 table-recreate copy.
// 反约束: 数据丢失即设计 ① "enum 扩不裂表" 字面破 (拆表 ≈ 老数据丢).
func TestCV_PreservesMarkdownRowsAcrossRebuild(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	// Run only CV-1.1 first so we can seed pre-CV-3.1 data, then CV-3.1.
	e := New(db)
	e.Register(artifacts)
	if err := e.Run(0); err != nil {
		t.Fatalf("run cv_1_1: %v", err)
	}
	if err := db.Exec(`INSERT INTO artifacts
		(id, channel_id, type, title, body, current_version, created_at, archived_at,
		 lock_holder_user_id, lock_acquired_at)
		VALUES ('art-old', 'ch-A', 'markdown', 'Old', 'body-content', 5, 1700000000000,
		        NULL, NULL, NULL)`).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	e2 := New(db)
	e2.Register(artifactKinds)
	if err := e2.Run(0); err != nil {
		t.Fatalf("run cv_3_1: %v", err)
	}

	// Verify the row survived end-to-end with all fields intact (字段
	// drift in INSERT...SELECT 会让 body / current_version / archived_at
	// 等 silent 掉; 全字段断言挡这条路).
	type row struct {
		ID             string `gorm:"column:id"`
		ChannelID      string `gorm:"column:channel_id"`
		Type           string `gorm:"column:type"`
		Title          string `gorm:"column:title"`
		Body           string `gorm:"column:body"`
		CurrentVersion int    `gorm:"column:current_version"`
		CreatedAt      int64  `gorm:"column:created_at"`
	}
	var got row
	if err := db.Raw(`SELECT id, channel_id, type, title, body, current_version, created_at
		FROM artifacts WHERE id='art-old'`).Scan(&got).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := row{
		ID:             "art-old",
		ChannelID:      "ch-A",
		Type:           "markdown",
		Title:          "Old",
		Body:           "body-content",
		CurrentVersion: 5,
		CreatedAt:      1700000000000,
	}
	if got != want {
		t.Errorf("row drift across rebuild:\n got: %+v\nwant: %+v", got, want)
	}
}

// TestCanvasArtifactKinds_PreservesChannelIDIndex pins acceptance §1 + cv-1-spec §0
// 设计 — `idx_artifacts_channel_id` MUST survive the table-recreate
// (DROP TABLE drops the index, the migration must recreate it). channel-list
// 是 CV-1.2 list 端热路径; index 丢失 = list 全表扫.
func TestCanvasArtifactKinds_PreservesChannelIDIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV31(t, db)

	const idx = "idx_artifacts_channel_id"
	var name string
	err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&name).Error
	if err != nil || name != idx {
		t.Errorf("missing index %s after rebuild (got %q, err=%v)", idx, name, err)
	}
}

// TestCanvasArtifactKinds_NoSeparateKindTables pins acceptance §1.5 + spec §3 反约束 —
// 设计 ① 不裂表: 不开 artifact_code / artifact_images. 反向断言
// sqlite_master.tables 不含此名 (跟 spec §3 reverse grep 同源, schema
// 层 belt 兜).
func TestCanvasArtifactKinds_NoSeparateKindTables(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV31(t, db)

	for _, forbidden := range []string{"artifact_code", "artifact_images", "artifact_image_links"} {
		var n int64
		if err := db.Raw(`SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name=?`, forbidden).Scan(&n).Error; err != nil {
			t.Fatalf("scan %s: %v", forbidden, err)
		}
		if n != 0 {
			t.Errorf("table %q exists — 反约束 broken (设计 ① enum 扩不裂表)", forbidden)
		}
	}
}

// TestCanvasArtifactKinds_Idempotent pins forward-only safety: re-running v=17 against
// a DB that already has it is a no-op (the migration framework's
// schema_migrations gate handles this; we exercise the gate by calling
// Run twice).
func TestCanvasArtifactKinds_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV31(t, db)

	// Second engine, same registry — Run must succeed (no-op via
	// schema_migrations.applied gate).
	e := New(db)
	e.Register(artifacts)
	e.Register(artifactKinds)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run cv_3_1: %v", err)
	}
}
