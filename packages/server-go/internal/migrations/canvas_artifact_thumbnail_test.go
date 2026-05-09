package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runCV3V2 chains v=13 (CV-1.1) → v=17 (CV-3.1) → v=28 (CV-2 v2) → v=31
// (CV-3 v2) on a memory DB.
func runCV3V2(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(artifacts)
	e.Register(artifactKinds)
	e.Register(cv2v2MediaPreview)
	e.Register(cv3v2ArtifactThumbnail)
	if err := e.Run(0); err != nil {
		t.Fatalf("run cv_3_v2: %v", err)
	}
}

// REG-CV3V2-001 (acceptance §1.1) — schema adds nullable thumbnail_url
// column (NULL = 未生成, 跟 preview_url + AP-1.1/AP-3/AP-2 五连同模式).
func TestCV_AddsThumbnailURLColumn(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV3V2(t, db)

	cols := pragmaColumns(t, db, "artifacts")
	c, ok := cols["thumbnail_url"]
	if !ok {
		t.Fatalf("artifacts missing thumbnail_url column (have %v)", keys(cols))
	}
	if c.notNull {
		t.Errorf("artifacts.thumbnail_url must be nullable (NULL = 未生成, 设计 ③ 五连模式)")
	}
}

// REG-CV3V2-001b — legacy markdown rows preserve NULL thumbnail_url
// (跟 preview_url 同精神, ALTER ADD COLUMN NULL 现网行为零变).
func TestCV_LegacyRowsNullPreserved(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	// Run pre-CV-3 v2 chain, seed a row, then run v=31.
	e := New(db)
	e.Register(artifacts)
	e.Register(artifactKinds)
	e.Register(cv2v2MediaPreview)
	if err := e.Run(0); err != nil {
		t.Fatalf("run pre v=31: %v", err)
	}
	if err := db.Exec(`INSERT INTO artifacts
		(id, channel_id, type, title, body, current_version, created_at)
		VALUES ('art-legacy', 'ch-A', 'markdown', 'Legacy', 'body', 1, 1700000000000)`).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	e2 := New(db)
	e2.Register(cv3v2ArtifactThumbnail)
	if err := e2.Run(0); err != nil {
		t.Fatalf("run cv_3_v2: %v", err)
	}

	var url *string
	if err := db.Raw(`SELECT thumbnail_url FROM artifacts WHERE id='art-legacy'`).Scan(&url).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if url != nil {
		t.Errorf("legacy row thumbnail_url: got %q, want NULL (现网行为零变)", *url)
	}
}

// REG-CV3V2-001c — schema accepts explicit thumbnail_url assignment.
func TestCV_AcceptsExplicitThumbnailURL(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV3V2(t, db)

	if err := db.Exec(`INSERT INTO artifacts
		(id, channel_id, type, title, body, current_version, created_at, thumbnail_url)
		VALUES ('art-1', 'ch-A', 'markdown', 'T', '# h', 1, 1700000000000, 'https://cdn.example/thumb.png')`).Error; err != nil {
		t.Fatalf("insert with thumbnail_url: %v", err)
	}
	var got *string
	if err := db.Raw(`SELECT thumbnail_url FROM artifacts WHERE id='art-1'`).Scan(&got).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if got == nil || *got != "https://cdn.example/thumb.png" {
		t.Errorf("thumbnail_url after insert: got %v, want set", got)
	}
}

// REG-CV3V2-001d (spec §3 反约束 + 设计 ⑧) — does NOT create separate
// thumbnail tables (CV-3.1 设计 ① "enum 扩不裂表" 同精神).
func TestCV_NoSeparateThumbnailTables(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV3V2(t, db)

	for _, forbidden := range []string{"artifact_thumbnails", "artifact_previews"} {
		var n int64
		if err := db.Raw(`SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name=?`, forbidden).Scan(&n).Error; err != nil {
			t.Fatalf("scan %s: %v", forbidden, err)
		}
		if n != 0 {
			t.Errorf("table %q exists — 反约束 broken (设计 ⑧ 不裂表)", forbidden)
		}
	}
}

// REG-CV3V2-001e — registry.go 字面锁 v=31.
func TestCV_RegistryHasV32(t *testing.T) {
	t.Parallel()
	for _, m := range All {
		if m.Version == 32 {
			if m.Name != "cv_3_v2_artifact_thumbnail" {
				t.Errorf("v=31 name drift: got %q, want %q", m.Name, "cv_3_v2_artifact_thumbnail")
			}
			return
		}
	}
	t.Fatal("v=31 (CV-3 v2.1) not registered in migrations.All")
}

// TestCanvasArtifactThumbnail_Idempotent — re-running v=31 on already-applied DB is no-op.
func TestCanvasArtifactThumbnail_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV3V2(t, db)

	e := New(db)
	e.Register(artifacts)
	e.Register(artifactKinds)
	e.Register(cv2v2MediaPreview)
	e.Register(cv3v2ArtifactThumbnail)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run cv_3_v2: %v", err)
	}
}
