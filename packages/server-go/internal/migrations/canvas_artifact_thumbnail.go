package migrations

import "gorm.io/gorm"

// cv3v2ArtifactThumbnail is migration v=31 — Phase 5+ / CV-3 v2.
//
// Blueprint锚: `canvas-vision.md` §1.4 ("artifact 集合: 多类型, 首屏快读
// 不是浏览器内全量解码"). Spec brief:
// docs/implementation/modules/cv-3-v2-spec.md (战马C v0, 484ec08) §0
// 设计 ③ + §1 拆段 CV-3 v2.1.
//
// What this migration does:
//   1. ALTER TABLE artifacts ADD COLUMN thumbnail_url TEXT NULL
//      (跟 cv_2_v2_media_preview v=28 preview_url + ap_1_1 expires_at +
//      ap_3_1 org_id + ap_2_1 revoked_at 五连 ALTER ADD COLUMN NULL 模式).
//      NULL = 未生成 thumbnail (跟 preview_url NULL = 未生成 thumbnail
//      same精神, 跟 AP-1 现网 ABAC 行为零变 same模式).
//
// 反约束 (cv-3-v2-spec.md §0 设计 ③ + §3):
//   - 不挂 NOT NULL — NULL = 未生成 (跟 preview_url 同精神, ALTER ADD
//     COLUMN 五连同模式).
//   - 不挂 default — NULL 是合法终态 (跟 expires_at / org_id /
//     revoked_at / preview_url 五连同精神).
//   - 不裂表 (CV-3.1 设计 ① "enum 扩不裂表" 字面承袭) — 表名仍
//     `artifacts`, grep 检查 `CREATE TABLE.*artifact_thumbnails|
//     artifact_previews` count==0.
//   - thumbnail_url MUST be https only — 校验在 server validation 层
//     (handler POST /artifacts/:id/thumbnail), schema 层仅允 NULL or
//     any TEXT (跟 preview_url + image_link metadata.url 同精神,
//     schema CHECK 不能装 URL parser).
//
// v=31 sequencing: cv_2_v2 v=28 (CV-2 v2 #517 merged) → ap_3_1 v=29
// (in flight #521 AP-3) → ap_2_1 v=30 (AP-2 #525 merged) → cv_3_v2 **v=31**
// (本 migration). registry.go 字面锁; 五连 ALTER ADD COLUMN NULL 系列收尾.
//
// v0 stance: forward-only, no Down(). ALTER ADD COLUMN 在 SQLite
// idempotent-unsafe (重跑会报 duplicate column), engine 通过
// schema_migrations 版本号守 idempotency — 跟所有 ALTER 类 migration
// 同模式 (cv_2_v2 / ap_1_1 / ap_3_1 / ap_2_1).
var cv3v2ArtifactThumbnail = Migration{
	Version: 32,
	Name:    "cv_3_v2_artifact_thumbnail",
	Up: func(tx *gorm.DB) error {
		// Trimmed-schema gate (跟 cv_2_v2 / ap_2_1 同模式 — 部分 migration
		// test 单独 register 此 migration 不带上游 artifacts 表).
		exists, err := hasTable(tx, "artifacts")
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		// ALTER ADD COLUMN — SQLite supports this without table rebuild
		// when no constraint is added. NULL default + nullable = 零行为变.
		if err := tx.Exec(`ALTER TABLE artifacts ADD COLUMN thumbnail_url TEXT`).Error; err != nil {
			return err
		}
		return nil
	},
}
