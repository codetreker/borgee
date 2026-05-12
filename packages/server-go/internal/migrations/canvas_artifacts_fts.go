package migrations

import "gorm.io/gorm"

// artifactsFTS is migration v=36 — Phase 5+ / CV-6.1.
//
// Blueprint出处: `canvas-vision.md` §1.4 (artifact 集合, "首屏快读") + 整体
// 技术栈 SQLite 单一来源 (不另起 elasticsearch / opensearch /
// typesense / meilisearch / sonic / bleve search service). Spec brief:
// docs/implementation/modules/cv-6-spec.md (v0, d2fe1f0) §0
// 设计 ① + §1 拆段 CV-6.1.
//
// What this migration does:
//  1. CREATE VIRTUAL TABLE artifacts_fts USING fts5(title, body,
//     content=artifacts, content_rowid=id, tokenize='unicode61
//     remove_diacritics 2') — contentless 模式跟 artifacts 单一来源,
//     不拆表 (设计 ③ grep 检查 `CREATE TABLE.*search_index|
//     artifact_search_results|fts_documents` 0 hit).
//  2. 三 AFTER trigger byte-identical 命名 artifacts_ai (INSERT) /
//     artifacts_au (UPDATE) / artifacts_ad (DELETE) — 自动同步
//     artifacts CRUD 路径 (CV-1 / CV-3 v2 既有 endpoint 不改, 走 CRUD
//     自动入 index).
//  3. Initial backfill — `INSERT INTO artifacts_fts(rowid, title, body)
//     SELECT id, title, body FROM artifacts WHERE archived_at IS NULL`
//     — 历史行入 index, 设计 ⑥ archived_at IS NOT NULL 反向断言
//     不出现.
//
// 反向约束 (cv-6-spec.md §0 设计 ①③ + §3 反向约束 grep):
//   - 不另起 search 表 (FTS5 contentless 跟 artifacts 单一来源, 反向
//     grep 0 hit).
//   - 不引入 cron 框架 reindex (FTS5 trigger 自动同步).
//   - 不引入 elasticsearch / opensearch / typesense / meilisearch /
//     sonic / bleve / blevesearch (蓝图 SQLite 单一来源).
//
// v0 原则: forward-only, no Down(). FTS5 virtual table 在 SQLite
// 是原生支持 (contrib module), engine 通过 schema_migrations 版本号
// 守 idempotency.
//
// v=36 sequencing: cv_2_v2 v=28 (CV-2 v2 #517) → ap_3_1 v=29 (AP-3 #521
// in flight) → ap_2_1 v=30 (AP-2 #525 merged) → cv_3_v2 v=31 (CV-3 v2
// #528 in flight) → cv_6_1 **v=36** (本 migration). registry.go 字面锁定;
// merge order determines the version sequence.
var artifactsFTS = Migration{
	Version: 36,
	Name:    "cv_6_1_artifacts_fts",
	Up: func(tx *gorm.DB) error {
		// Trimmed-schema gate (跟 cv_2_v2 / cv_3_v2 / ap_2_1 同模式).
		exists, err := hasTable(tx, "artifacts")
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		// Step 1 — FTS5 contentless virtual table (跟 artifacts 单一来源,
		// content_rowid='id' 出处 artifact PK).
		if err := tx.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS artifacts_fts USING fts5(
			title, body,
			content='artifacts',
			content_rowid='rowid',
			tokenize='unicode61 remove_diacritics 2'
		)`).Error; err != nil {
			return err
		}

		// Step 2 — 三 AFTER trigger byte-identical 命名同步.
		// SQLite FTS5 contentless 模式: INSERT/UPDATE/DELETE 必显式同步.
		// 用 'delete' 命令清旧行, 'insert' 写新行 (FTS5 不支持 ROW UPDATE).
		if err := tx.Exec(`CREATE TRIGGER IF NOT EXISTS artifacts_ai
			AFTER INSERT ON artifacts BEGIN
			INSERT INTO artifacts_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
		END`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE TRIGGER IF NOT EXISTS artifacts_ad
			AFTER DELETE ON artifacts BEGIN
			INSERT INTO artifacts_fts(artifacts_fts, rowid, title, body)
			VALUES('delete', old.rowid, old.title, old.body);
		END`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE TRIGGER IF NOT EXISTS artifacts_au
			AFTER UPDATE ON artifacts BEGIN
			INSERT INTO artifacts_fts(artifacts_fts, rowid, title, body)
			VALUES('delete', old.rowid, old.title, old.body);
			INSERT INTO artifacts_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
		END`).Error; err != nil {
			return err
		}

		// Step 3 — initial backfill, 历史行入 index (设计 ⑥
		// archived_at IS NULL 过滤 — archived 不出现).
		if err := tx.Exec(`INSERT INTO artifacts_fts(rowid, title, body)
			SELECT rowid, title, body FROM artifacts WHERE archived_at IS NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
