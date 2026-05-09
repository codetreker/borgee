package migrations

import (
	"testing"

	"gorm.io/gorm"
)

// runCV21 runs migrations through v=14 (CV-2.1) on a memory DB. We chain on
// top of CV-1.1 (v=13) because logical FKs (artifact_id / artifact_version_id)
// only make sense once the upstream tables exist; tests that exercise FK
// behavior seed real artifacts + artifact_versions rows.
func runCV21(t *testing.T, db *gorm.DB) {
	t.Helper()
	e := New(db)
	e.Register(artifacts)
	e.Register(anchorComments)
	if err := e.Run(0); err != nil {
		t.Fatalf("run cv_2_1: %v", err)
	}
}

// TestCV_CreatesArtifactAnchorsTable pins acceptance §1 (cv-2.md §1.1):
// artifact_anchors has the contract columns with the right NOT NULL / nullable
// shape + the end_offset CHECK. Drift here breaks 设计 ② (锚钉死 version) or
// the range invariant.
func TestCV_CreatesArtifactAnchorsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV21(t, db)

	cols := pragmaColumns(t, db, "artifact_anchors")
	if len(cols) == 0 {
		t.Fatal("artifact_anchors table not created")
	}

	idCol, ok := cols["id"]
	if !ok {
		t.Fatalf("artifact_anchors missing id (have %v)", keys(cols))
	}
	if !idCol.pk {
		t.Error("artifact_anchors.id must be PRIMARY KEY")
	}

	for _, name := range []string{
		"artifact_id",
		"artifact_version_id",
		"start_offset",
		"end_offset",
		"created_by",
		"created_at",
	} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("artifact_anchors missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("artifact_anchors.%s must be NOT NULL", name)
		}
	}

	// resolved_at — NULL = active, 非 NULL = 已审 (owner / creator 翻).
	resolvedAt, ok := cols["resolved_at"]
	if !ok {
		t.Fatalf("artifact_anchors missing resolved_at (have %v)", keys(cols))
	}
	if resolvedAt.notNull {
		t.Error("artifact_anchors.resolved_at must be nullable (NULL = active)")
	}

	// 反约束: anchor_kind / author_kind MUST NOT exist on artifact_anchors —
	// 锚是定位 + 元数据, kind 在 anchor_comments (评论作者) 才有意义; 防止
	// 蓝图 §1.6 锚点 = 人审工具的语义被 schema 拆错列 (设计 ① owner / 成员
	// 创锚, agent 不创锚 — kind 列若挂 anchors 会把 review tool 错位成
	// "agent 也能创锚" 的口子).
	for _, forbidden := range []string{"anchor_kind", "author_kind", "kind"} {
		if _, has := cols[forbidden]; has {
			t.Errorf("artifact_anchors.%s exists — 反约束 broken (kind 仅在 anchor_comments, 设计 ①)", forbidden)
		}
	}
	// 反约束: cursor 不挂 schema (跟 RT-1 envelope cursor 拆死, 同 cv_1_1).
	if _, has := cols["cursor"]; has {
		t.Error("artifact_anchors.cursor exists — 反约束 broken (RT-1 envelope cursor, not schema)")
	}
}

// TestCV_RejectsInvalidEndOffsetRange pins range CHECK — end_offset >=
// start_offset. start>end → reject. spec §0 设计 ② anchor_range 字符索引,
// invalid 范围让 review 跑偏.
func TestCV_RejectsInvalidEndOffsetRange(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV21(t, db)

	insert := func(start, end int) error {
		return db.Exec(`INSERT INTO artifact_anchors
			(id, artifact_id, artifact_version_id, start_offset, end_offset, created_by, created_at)
			VALUES (?, 'art-A', 1, ?, ?, 'user-1', 1700000000000)`,
			"anchor-test", start, end).Error
	}

	// end == start (single-char anchor) allowed.
	if err := insert(5, 5); err != nil {
		t.Fatalf("end==start anchor rejected: %v", err)
	}
	_ = db.Exec(`DELETE FROM artifact_anchors`).Error

	// end > start (normal range) allowed.
	if err := insert(0, 10); err != nil {
		t.Fatalf("end>start anchor rejected: %v", err)
	}
	_ = db.Exec(`DELETE FROM artifact_anchors`).Error

	// end < start rejected.
	if err := insert(10, 5); err == nil {
		t.Error("end<start accepted — CHECK (end_offset >= start_offset) missing")
	}
}

// TestCV_CreatesAnchorCommentsTable pins acceptance §1.2 second-table
// contract: anchor_comments captures author_kind ('agent','human') + PK
// AUTOINCREMENT (audit 序). 注意命名: anchor 是评论作者用 author_kind, 不
// 复用 CV-1.1 artifact_versions.committer_kind (commit 提交者) — spec v2
// 字面锁.
func TestCV_CreatesAnchorCommentsTable(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV21(t, db)

	cols := pragmaColumns(t, db, "anchor_comments")
	if len(cols) == 0 {
		t.Fatal("anchor_comments table not created")
	}

	idCol, ok := cols["id"]
	if !ok {
		t.Fatalf("anchor_comments missing id (have %v)", keys(cols))
	}
	if !idCol.pk {
		t.Error("anchor_comments.id must be PRIMARY KEY")
	}

	for _, name := range []string{"anchor_id", "body", "author_kind", "author_id", "created_at"} {
		c, ok := cols[name]
		if !ok {
			t.Fatalf("anchor_comments missing %q (have %v)", name, keys(cols))
		}
		if !c.notNull {
			t.Errorf("anchor_comments.%s must be NOT NULL", name)
		}
	}

	// 反约束: committer_kind MUST NOT exist — anchor 评论作者用 author_kind,
	// commit 提交者用 committer_kind (CV-1.1), spec v2 字面拆。复用列名会让
	// 反查 grep `author_kind=='agent'.*author_kind=='agent'` 跟 CV-1 fanout
	// 路径混淆。
	if _, has := cols["committer_kind"]; has {
		t.Error("anchor_comments.committer_kind exists — 反约束 broken (anchor 用 author_kind, spec v2)")
	}
}

// TestCV_RejectsInvalidAuthorKind pins anchor_comments.author_kind CHECK
// in ('agent','human'). 设计 ① 反 agent→agent thread 由 server 校验 (至少
// 一 'human'), 但 schema 层先把 kind enum 锁死, drift 上去 server 也兜不住.
func TestCV_RejectsInvalidAuthorKind(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV21(t, db)

	insert := func(kind string) error {
		return db.Exec(`INSERT INTO anchor_comments
			(anchor_id, body, author_kind, author_id, created_at)
			VALUES ('anchor-A', '', ?, 'author-1', 1700000000000)`,
			kind).Error
	}
	if err := insert("agent"); err != nil {
		t.Fatalf("agent insert rejected: %v", err)
	}
	if err := insert("human"); err != nil {
		t.Fatalf("human insert rejected: %v", err)
	}
	for _, bad := range []string{"admin", "system", "bot", "owner", ""} {
		if err := insert(bad); err == nil {
			t.Errorf("author_kind=%q accepted — CHECK ('agent','human') missing", bad)
		}
	}
}

// TestCV_CommentsTablePKMonotonic pins anchor_comments.id PK
// AUTOINCREMENT — global strictly increasing across all anchors. Same shape
// as TestCV11_VersionsTablePKMonotonic for artifact_versions, gives audit
// log + cursor stub a stable backfill order assumption. UNIQUE constraints
// don't substitute (PK 是 cross-anchor 全局序; UNIQUE 这里没有, 同 anchor
// 多 comment 合法).
func TestCV_CommentsTablePKMonotonic(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV21(t, db)

	insert := func(anchor string) error {
		return db.Exec(`INSERT INTO anchor_comments
			(anchor_id, body, author_kind, author_id, created_at)
			VALUES (?, '', 'human', 'user-1', 1700000000000)`,
			anchor).Error
	}
	// Interleave anchors to prove PK is global, not per-anchor.
	for _, a := range []string{"anchor-A", "anchor-B", "anchor-A", "anchor-B"} {
		if err := insert(a); err != nil {
			t.Fatalf("%s: %v", a, err)
		}
	}

	type row struct {
		ID       int64  `gorm:"column:id"`
		AnchorID string `gorm:"column:anchor_id"`
	}
	var rows []row
	if err := db.Raw(`SELECT id, anchor_id FROM anchor_comments ORDER BY id ASC`).Scan(&rows).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].ID <= rows[i-1].ID {
			t.Errorf("PK not strictly increasing at row %d: %d after %d (anchor_id %q)",
				i, rows[i].ID, rows[i-1].ID, rows[i].AnchorID)
		}
	}
	// Insertion order interleaved across anchors.
	wantOrder := []string{"anchor-A", "anchor-B", "anchor-A", "anchor-B"}
	for i, w := range wantOrder {
		if rows[i].AnchorID != w {
			t.Errorf("row %d: got %q, want %q", i, rows[i].AnchorID, w)
		}
	}
}

// TestCanvasAnchorComments_HasIndexes pins acceptance §1.x — per-version anchor list +
// thread comment lookup require both indexes.
func TestCanvasAnchorComments_HasIndexes(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV21(t, db)

	for _, idx := range []string{
		"idx_anchors_artifact_version",
		"idx_anchor_comments_anchor",
	} {
		var name string
		err := db.Raw(`SELECT name FROM sqlite_master
			WHERE type='index' AND name=?`, idx).Scan(&name).Error
		if err != nil || name != idx {
			t.Errorf("missing index %s (got %q, err=%v)", idx, name, err)
		}
	}
}

// TestCV_AnchorsAcrossVersionsCoexist pins 设计 ② 锚钉死 version (immutable
// across artifact 滚动). 同 artifact 不同 version 各自挂锚 row, 互不干扰
// (artifact_version_id FK 严格不同). 反向断言: 单 anchor 不会被 version 滚动
// "携带" — schema 层完全分行, 没有任何 update_to_next_version 字段.
func TestCV_AnchorsAcrossVersionsCoexist(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV21(t, db)

	insert := func(anchorID string, versionID int) error {
		return db.Exec(`INSERT INTO artifact_anchors
			(id, artifact_id, artifact_version_id, start_offset, end_offset, created_by, created_at)
			VALUES (?, 'art-A', ?, 0, 5, 'user-1', 1700000000000)`,
			anchorID, versionID).Error
	}
	if err := insert("anchor-v1-1", 1); err != nil {
		t.Fatalf("anchor on v=1: %v", err)
	}
	if err := insert("anchor-v2-1", 2); err != nil {
		t.Fatalf("anchor on v=2: %v", err)
	}

	type row struct {
		ID                string `gorm:"column:id"`
		ArtifactVersionID int    `gorm:"column:artifact_version_id"`
	}
	var rows []row
	if err := db.Raw(`SELECT id, artifact_version_id FROM artifact_anchors
		WHERE artifact_id='art-A' ORDER BY artifact_version_id ASC`).Scan(&rows).Error; err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (anchors on different versions coexist)", len(rows))
	}
	if rows[0].ArtifactVersionID == rows[1].ArtifactVersionID {
		t.Error("anchors on same artifact_version_id — version pinning broken")
	}
}

// TestCanvasAnchorComments_Idempotent pins forward-only safety: re-running v=14 is no-op.
func TestCanvasAnchorComments_Idempotent(t *testing.T) {
	t.Parallel()
	db := openMem(t)
	runCV21(t, db)
	e := New(db)
	e.Register(artifacts)
	e.Register(anchorComments)
	if err := e.Run(0); err != nil {
		t.Fatalf("re-run cv_2_1: %v", err)
	}
}
