package migrations

import (
	"gorm.io/gorm"
)

// cv2v2MediaPreview is migration v=28 — Phase 5 / CV-2 v2 (artifact preview
// thumbnail + media player; 跟 CV-2 v1 锚点对话已闭解耦).
//
// Blueprint锚: `canvas-vision.md` §1.4 (artifact 集合: Markdown / 代码片段
// 带语言标注 / 设计稿图片或链接 / 看板 v2+) — preview 是首屏快读, 不是 inline
// 全量. Spec brief: `docs/implementation/modules/cv-2-v2-media-preview-spec.md`
// (战马D v0) §0 设计 ① server CDN thumbnail + ③ kind enum 扩 CV-3 同源.
//
// What this migration does (SQLite CHECK rewrite via the standard 12-step
// "create _new + copy + swap" pattern, since SQLite cannot ALTER an existing
// CHECK constraint, 跟 CV-3.1 #396 同模式):
//   1. Skip entirely if `artifacts` table is absent (trimmed migration test
//      schemas — 跟 cm_3 / adm_0_3 / cv_3_1 同模式).
//   2. CREATE TABLE artifacts_cv2v2_new with the expanded CHECK:
//        type IN ('markdown','code','image_link','video_link','pdf_link')
//      (CV-3.1 #396 v=17 had 3 项; CV-2 v2 扩 'video_link' / 'pdf_link');
//      and a new NULLABLE preview_url TEXT column.
//   3. INSERT INTO artifacts_cv2v2_new SELECT ... FROM artifacts — copies
//      every existing row verbatim with preview_url=NULL (no thumbnail
//      backfill — server-side endpoint generates lazily on first POST
//      /preview, 设计 ①).
//   4. DROP TABLE artifacts; ALTER TABLE artifacts_cv2v2_new RENAME TO
//      artifacts. SQLite swap is atomic within the migration tx.
//   5. CREATE INDEX IF NOT EXISTS idx_artifacts_channel_id (CV-1.1 +
//      CV-3.1 idempotent — DROP TABLE 顺带丢索引).
//
// 反约束 (cv-2-v2-media-preview-spec.md §0 + spec §3):
//   - 不裂表 (CV-3.1 设计 ① enum 扩不裂表字面承袭): 表名仍 `artifacts`,
//     grep 检查 `CREATE TABLE.*artifact_video|CREATE TABLE.*artifact_pdf`
//     count==0.
//   - CHECK 严格 reject 'kanban' / 'mindmap' / 'doc' (蓝图 §2 v1 不做字面禁
//     依然守; 'pdf' 是字面禁 — CV-2 v2 用 'pdf_link' 承袭蓝图 §1.4 "PDF 链接"
//     非 inline pdf, 设计 ② HTML5 native <embed>).
//   - preview_url MUST be https only — 校验在 server validation 层 (handler
//     POST /artifacts/:id/preview), schema 层仅允 NULL or any TEXT (跟
//     cv_3_2 metadata.url 同精神, schema CHECK 不能装 URL parser).
//
// v0 stance: forward-only, no Down(). Idempotent re-run guard via outer
// migration framework's schema_migrations gate.
//
// v=28 sequencing — 跟 dl_4_1 v=26 + hb_3_1 v=27 后顺位拿. registry.go 字面锁.
var cv2v2MediaPreview = Migration{
	Version: 28,
	Name:    "cv_2_v2_media_preview",
	Up: func(tx *gorm.DB) error {
		exists, err := hasTable(tx, "artifacts")
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		// Step 1 — create the _new shadow table with the expanded CHECK +
		// new preview_url column.
		if err := tx.Exec(`CREATE TABLE artifacts_cv2v2_new (
  id                  TEXT    PRIMARY KEY,
  channel_id          TEXT    NOT NULL,
  type                TEXT    NOT NULL CHECK (type IN ('markdown','code','image_link','video_link','pdf_link')),
  title               TEXT    NOT NULL,
  body                TEXT    NOT NULL DEFAULT '',
  current_version     INTEGER NOT NULL DEFAULT 1,
  created_at          INTEGER NOT NULL,
  archived_at         INTEGER,
  lock_holder_user_id TEXT,
  lock_acquired_at    INTEGER,
  preview_url         TEXT
)`).Error; err != nil {
			return err
		}

		// Step 2 — copy data verbatim (preview_url defaults NULL).
		if err := tx.Exec(`INSERT INTO artifacts_cv2v2_new
			(id, channel_id, type, title, body, current_version,
			 created_at, archived_at, lock_holder_user_id, lock_acquired_at, preview_url)
			SELECT
			 id, channel_id, type, title, body, current_version,
			 created_at, archived_at, lock_holder_user_id, lock_acquired_at, NULL
			FROM artifacts`).Error; err != nil {
			return err
		}

		// Step 3 — drop old, rename new in.
		if err := tx.Exec(`DROP TABLE artifacts`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`ALTER TABLE artifacts_cv2v2_new RENAME TO artifacts`).Error; err != nil {
			return err
		}

		// Step 4 — recreate the channel-scoped index (CV-1.1 #311 line 83 +
		// CV-3.1 #396 同模式).
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_artifacts_channel_id
			ON artifacts(channel_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}
