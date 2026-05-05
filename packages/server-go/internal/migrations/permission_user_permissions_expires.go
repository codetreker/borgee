package migrations

import "gorm.io/gorm"

// userPermissionsExpires is migration v=24 — Phase 4 / AP-1.1.
//
// Blueprint锚: `auth-permissions.md` §1.2 (Scope 层级 v1 三层 — `*` /
// `channel:<id>` / `artifact:<id>` 全 ✅, expires_at 列 "schema 保留, UI
// 不做") + §5 与现状的差距 ("expires_at 列 — 加列 (schema 不破), 暂不
// 业务化"). Spec stance: docs/blueprint/current/auth-permissions.md §1.2 字面承袭.
//
// What this migration does:
//   1. ALTER TABLE user_permissions ADD COLUMN expires_at INTEGER NULL
//      (Unix ms; NULL = 永久, 跟现状 ABAC 行为零变).
//   2. CREATE INDEX idx_user_permissions_expires ON user_permissions(
//      expires_at) WHERE expires_at IS NOT NULL — sparse index 仅扫
//      有期限行, sweeper 路径热查 (v2+ 业务化时挂 cron, v1 不消费).
//
// 反约束 (auth-permissions.md §1.2 + 立场 "v1 schema 保留, UI 不做"):
//   - 不挂 NOT NULL — 现网行 expires_at 全 NULL = 永久, 跟蓝图 §1.1
//     "ABAC source of truth" 行为不变.
//   - 不挂 default 值 — NULL 是合法终态, 不 default 0 (0 会被 sweeper
//     当过期清; 反约束: 防 v2+ sweeper 误删现网永久行).
//   - 不挂 CHECK (expires_at > granted_at) — schema 留账, 业务校验 v2+
//     server 路径做 (蓝图 §5 字面 "暂不业务化").
//   - INDEX WHERE expires_at IS NOT NULL — partial index, 现网零开销
//     (主键 + 现有 idx_user_permissions_lookup 不动).
//
// v=24 sequencing: AL-1b.1 v=21 / ADM-2.1 v=22 / ADM-2.2 v=23 / **AP-1.1
// v=24** (本 migration). registry.go 字面锁.
//
// v0 stance: forward-only, no Down(). ALTER ADD COLUMN 在 SQLite 是
// idempotent-unsafe (重跑会报 duplicate column), engine 通过
// schema_migrations 版本号守 idempotency — 跟所有 ALTER 类 migration
// 同模式 (chn_3_1 / cm_3 等).
var userPermissionsExpires = Migration{
	Version: 24,
	Name:    "ap_1_1_user_permissions_expires",
	Up: func(tx *gorm.DB) error {
		// ALTER ADD COLUMN — SQLite supports this without table rebuild
		// when no constraint is added. NULL default + nullable = 零行为变.
		if err := tx.Exec(`ALTER TABLE user_permissions ADD COLUMN expires_at INTEGER`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_permissions_expires
			ON user_permissions(expires_at) WHERE expires_at IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
