package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runCV2V2 chains v=13 (CV-1.1) → v=17 (CV-3.1) → v=28 (CV-2 v2). The
// outer chain is required because v=28 piggy-backs on the artifacts table
// (CV-1.1) and extends the kind enum installed by CV-3.1.
func runCV2V2(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(artifacts)
	e.Register(artifactKinds)
	e.Register(cv2v2MediaPreview)
	if err := e.Run(0); err != nil {
		t.Fatalf("run cv_2_v2: %v", err)
	}
}

// REG-CV2V2-001a (acceptance §1.1) — kind enum CHECK accepts the full
// 5-tuple after v=28: markdown / code / image_link (CV-3.1 既有) +
// video_link / pdf_link (CV-2 v2 新).
func TestCV_AcceptsAllFiveKinds(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV2V2(t, db)

	insert := func(id, kind string) error {
		return db.Exec(`INSERT INTO artifacts
			(id, channel_id, type, title, body, current_version, created_at)
			VALUES (?, 'ch-A', ?, 'T', '', 1, 1700000000000)`, id, kind).Error
	}
	for _, k := range []string{"markdown", "code", "image_link", "video_link", "pdf_link"} {
		if err := insert("art-"+k, k); err != nil {
			t.Errorf("kind=%q rejected — CHECK should accept post-v=28: %v", k, err)
		}
	}
}

// REG-CV2V2-001b (spec §0 设计 ③ + acceptance §1.5) — CHECK still rejects
// kanban / mindmap / doc / video / pdf (bare 'pdf' / 'video' 没有 _link
// 后缀, 跟蓝图 §1.4 命名 "video_link"/"pdf_link" byte-identical).
func TestCV_RejectsForbiddenKinds(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV2V2(t, db)

	insert := func(kind string) error {
		return db.Exec(`INSERT INTO artifacts
			(id, channel_id, type, title, body, current_version, created_at)
			VALUES (?, 'ch-A', ?, 'T', '', 1, 1700000000000)`,
			"art-"+kind, kind).Error
	}
	for _, bad := range []string{"kanban", "mindmap", "doc", "video", "pdf", ""} {
		if err := insert(bad); err == nil {
			t.Errorf("kind=%q accepted — CHECK whitelist drift", bad)
		}
	}
}

// REG-CV2V2-001c — preview_url column added, defaults NULL, accepts TEXT.
func TestCV_PreviewURLColumn(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV2V2(t, db)

	if err := db.Exec(`INSERT INTO artifacts
		(id, channel_id, type, title, body, current_version, created_at)
		VALUES ('art-1', 'ch-A', 'image_link', 'T', 'https://cdn.example/x.png', 1, 1700000000000)`).Error; err != nil {
		t.Fatalf("insert without preview_url: %v", err)
	}
	var url *string
	if err := db.Raw(`SELECT preview_url FROM artifacts WHERE id='art-1'`).Scan(&url).Error; err != nil {
		t.Fatalf("scan preview_url: %v", err)
	}
	if url != nil {
		t.Errorf("preview_url default: got %q, want NULL", *url)
	}

	// UPDATE accepts a value.
	if err := db.Exec(`UPDATE artifacts SET preview_url = 'https://cdn.example/x-thumb.jpg' WHERE id='art-1'`).Error; err != nil {
		t.Fatalf("update preview_url: %v", err)
	}
	if err := db.Raw(`SELECT preview_url FROM artifacts WHERE id='art-1'`).Scan(&url).Error; err != nil {
		t.Fatalf("scan after update: %v", err)
	}
	if url == nil || *url != "https://cdn.example/x-thumb.jpg" {
		t.Errorf("preview_url after update: got %v, want set", url)
	}
}

// REG-CV2V2-001d (spec §3 反约束) — old rows preserved verbatim across
// the table-recreate copy. preview_url=NULL on copy (no thumbnail backfill).
func TestCV_PreservesExistingRows(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	e := New(db)
	e.Register(artifacts)
	e.Register(artifactKinds)
	if err := e.Run(0); err != nil {
		t.Fatalf("run pre v=28: %v", err)
	}
	if err := db.Exec(`INSERT INTO artifacts
		(id, channel_id, type, title, body, current_version, created_at, archived_at,
		 lock_holder_user_id, lock_acquired_at)
		VALUES ('art-old', 'ch-A', 'image_link', 'Old', 'https://cdn/x.png', 5, 1700000000000,
		        NULL, NULL, NULL)`).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	e2 := New(db)
	e2.Register(cv2v2MediaPreview)
	if err := e2.Run(0); err != nil {
		t.Fatalf("run cv_2_v2: %v", err)
	}

	type row struct {
		ID             string  `gorm:"column:id"`
		ChannelID      string  `gorm:"column:channel_id"`
		Type           string  `gorm:"column:type"`
		Title          string  `gorm:"column:title"`
		Body           string  `gorm:"column:body"`
		CurrentVersion int     `gorm:"column:current_version"`
		PreviewURL     *string `gorm:"column:preview_url"`
	}
	var got row
	if err := db.Raw(`SELECT id, channel_id, type, title, body, current_version, preview_url
		FROM artifacts WHERE id='art-old'`).Scan(&got).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if got.ID != "art-old" || got.Type != "image_link" || got.Title != "Old" ||
		got.Body != "https://cdn/x.png" || got.CurrentVersion != 5 {
		t.Errorf("row drift across rebuild: %+v", got)
	}
	if got.PreviewURL != nil {
		t.Errorf("preview_url after copy: got %q, want NULL (no backfill)", *got.PreviewURL)
	}
}

// REG-CV2V2-001e — idx_artifacts_channel_id survives the v=28 rebuild.
func TestCanvasMediaPreview_PreservesChannelIDIndex(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV2V2(t, db)

	const idx = "idx_artifacts_channel_id"
	var name string
	err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&name).Error
	if err != nil || name != idx {
		t.Errorf("missing index %s after v=28 rebuild (got %q, err=%v)", idx, name, err)
	}
}

// REG-CV2V2-001f (spec §0 设计 ③ + 反约束 不裂表) — no per-kind tables.
func TestCanvasMediaPreview_NoSeparateKindTables(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV2V2(t, db)

	for _, forbidden := range []string{"artifact_video", "artifact_pdf", "artifact_video_links", "artifact_pdf_links"} {
		var n int64
		if err := db.Raw(`SELECT COUNT(*) FROM sqlite_master
			WHERE type='table' AND name=?`, forbidden).Scan(&n).Error; err != nil {
			t.Fatalf("scan %s: %v", forbidden, err)
		}
		if n != 0 {
			t.Errorf("table %q exists — 反约束 broken (设计 ③ enum 扩不裂表)", forbidden)
		}
	}
}

// TestCanvasMediaPreview_Idempotent — re-running v=28 on an already-applied DB is a no-op.
func TestCanvasMediaPreview_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV2V2(t, db)

	e := New(db)
	e.Register(artifacts)
	e.Register(artifactKinds)
	e.Register(cv2v2MediaPreview)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run cv_2_v2: %v", err)
	}
}
