package migrations

import (
	"gorm.io/gorm"
)

// artifactKinds is migration v=17 — Phase 3 / CV-3.1.
//
// Blueprint出处: `canvas-vision.md` §1.2 (D-lite, 不是 Miro) + §1.4
// (artifact 集合: Markdown / 代码片段带语言标注 / 设计稿图片或链接 /
// 看板 v2+) + §2 v1 不做清单 (❌ 无限画布 / ❌ 多 artifact 关联视图 /
// ❌ CRDT / ❌ PDF / ❌ 看板). Spec brief: `docs/implementation/modules/cv-3-spec.md`
// (飞马 #363, merged 74700e0) §0 设计 ① + §1 拆段 CV-3.1.
// 原则: `docs/qa/cv-3-stance-checklist.md` (野马 #385). Acceptance:
// `docs/qa/acceptance-templates/cv-3.md` (#376) §1.1-§1.5. Content lock:
// `docs/qa/cv-3-content-lock.md` (野马 #370) §1 ①②⑦ + §2 grep 检查.
//
// What this migration does (SQLite CHECK rewrite via the standard
// 12-step "create _new + copy + swap" pattern, since SQLite cannot
// ALTER an existing CHECK constraint):
//   1. Skip entirely if `artifacts` table is absent (trimmed migration
//      test schemas — same gate as cm_3 / adm_0_3).
//   2. CREATE TABLE artifacts_cv31_new with the expanded CHECK:
//        type IN ('markdown','code','image_link')
//      (CV-1.1 #311 v=13 had `type = 'markdown'`; CV-3.1 extends).
//   3. INSERT INTO artifacts_cv31_new SELECT * FROM artifacts — copies
//      every row verbatim. Existing rows are all kind='markdown' (CV-1
//      v1 was Markdown ONLY by设计 ④), they pass the new CHECK.
//   4. DROP TABLE artifacts; ALTER TABLE artifacts_cv31_new RENAME TO
//      artifacts. SQLite's swap is atomic within the migration tx;
//      recovery comes from the migration framework's tx rollback.
//   5. CREATE INDEX IF NOT EXISTS idx_artifacts_channel_id (CV-1.1
//      idempotent — DROP TABLE 顺带丢索引, 必须重建 — channel-list 热路径).
//
// Naming note (semantic vs schema):
//   - 蓝图 + spec + content-lock + acceptance 全部用 "kind" 概念命名
//     (DOM `data-artifact-kind`, anchor_comments.author_kind, 设计 ④
//     "kind=='code'必含 metadata.language"). 现有 schema column 仍叫
//     `type` (CV-1.1 #311 落地命名). 此 migration **不重命名 column** —
//     重命名要扫所有 server / store / api / test, 远超 CV-3.1 schema 范围
//     (≤500 行预算). 语义层用 "kind", 物理层 column 仍 `type`. 反查 grep
//     `artifacts.kind` 在文档层命中, `artifacts.type` 在 schema 层命中.
//
// 反向约束 (cv-3-spec.md §0 + §3 + acceptance §1.5):
//   - 不拆表: 表名仍 `artifacts`, 不开 `artifact_code` / `artifact_images`
//     (设计 ① enum 扩不拆表; grep 检查 `CREATE TABLE.*artifact_code|
//     CREATE TABLE.*artifact_images` count==0).
//   - CHECK 严格 reject 'pdf' / 'kanban' / 'mindmap' (蓝图 §2 v1 不做字面禁;
//     acceptance §1.2 + content-lock §1 ① 反向约束 byte-identical).
//   - body / metadata / language enum 校验在 server validation 层 (CV-3.1
//     /artifacts POST handler 在 acceptance §1.3 / §1.4, 此 migration 仅
//     锁定 enum CHECK, 校验留 server 层 — schema CHECK 不能装 11 项白名单).
//
// v0 原则: forward-only, no Down(). Rows pass new CHECK because
// existing data is exclusively 'markdown' (CV-1 v1 设计 ④ Markdown ONLY).
// Idempotent re-run guard: hasTable("artifacts_cv31_new") + COPIED
// already would skip rerun via outer migration framework's `applied`
// table — this body assumes fresh tx (one-shot CHECK migration).
//
// v=17 sequencing (#363 spec §2 + 飞马 #379 v2 patch): CV-2.1 v=14 ✅
// (#359 merged) / DM-2.1 v=15 ✅ (#361 merged) / AL-4.1 v=16 待落 (战马待派) /
// CV-3.1 **v=17** (本 migration). registry.go 字面锁定.
var artifactKinds = Migration{
	Version: 17,
	Name:    "cv_3_1_artifact_kinds",
	Up: func(tx *gorm.DB) error {
		// Trimmed-schema gate (跟 adm_0_3 / cm_3 同模式 — 部分 migration
		// test 单独 register 此 migration 不带上游 artifacts 表).
		exists, err := hasTable(tx, "artifacts")
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		// Step 1 — create the _new shadow table with the expanded CHECK.
		// 字段集合跟 CV-1.1 #311 字面对齐 (id / channel_id / type / title /
		// body / current_version / created_at / archived_at /
		// lock_holder_user_id / lock_acquired_at). 新 CHECK 扩 enum 三项.
		// 反向约束: CHECK 字面排除 'pdf' / 'kanban' / 'mindmap' (走 IN whitelist
		// 而非显式 NOT IN — 白名单收窄, 等价但更字面).
		if err := tx.Exec(`CREATE TABLE artifacts_cv31_new (
  id                  TEXT    PRIMARY KEY,
  channel_id          TEXT    NOT NULL,
  type                TEXT    NOT NULL CHECK (type IN ('markdown','code','image_link')),
  title               TEXT    NOT NULL,
  body                TEXT    NOT NULL DEFAULT '',
  current_version     INTEGER NOT NULL DEFAULT 1,
  created_at          INTEGER NOT NULL,
  archived_at         INTEGER,
  lock_holder_user_id TEXT,
  lock_acquired_at    INTEGER
)`).Error; err != nil {
			return err
		}

		// Step 2 — copy data verbatim. Existing rows are all 'markdown'
		// (CV-1 设计 ④); they pass the new CHECK. We list columns
		// explicitly so a future ALTER TABLE artifacts ADD COLUMN doesn't
		// silently 脱节 between v=17 and the running schema.
		if err := tx.Exec(`INSERT INTO artifacts_cv31_new
			(id, channel_id, type, title, body, current_version,
			 created_at, archived_at, lock_holder_user_id, lock_acquired_at)
			SELECT
			 id, channel_id, type, title, body, current_version,
			 created_at, archived_at, lock_holder_user_id, lock_acquired_at
			FROM artifacts`).Error; err != nil {
			return err
		}

		// Step 3 — drop old, rename new in. SQLite drops indexes on
		// drop-table, so the index recreate at step 4 is required.
		if err := tx.Exec(`DROP TABLE artifacts`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`ALTER TABLE artifacts_cv31_new RENAME TO artifacts`).Error; err != nil {
			return err
		}

		// Step 4 — recreate the channel-scoped index (CV-1.1 #311 line 83).
		// IF NOT EXISTS guard for safety though after DROP TABLE the index
		// is fresh-gone.
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_artifacts_channel_id
			ON artifacts(channel_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}
