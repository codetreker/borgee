package migrations

import "gorm.io/gorm"

// userPermissionsOrg is migration v=29 — Phase 5 / AP-3.1.
//
// Blueprint锚: `auth-permissions.md` §1.2 (Scope 层级 v1 三层) + §5 与现状的差距
// ("cross-org 强制 — AP-3 后续 milestone"). Spec brief:
// docs/implementation/modules/ap-3-spec.md (v0, d69b617) §0 设计 ② +
// §1 拆段 AP-3.1.
//
// What this migration does:
//  1. ALTER TABLE user_permissions ADD COLUMN org_id TEXT NULL
//     (跟 ap_1_1 #493 expires_at ALTER ADD COLUMN NULL 同模式).
//     NULL = legacy 行 (AP-1 现网行为零变, 任一 NULL 走 legacy 路径).
//     显式 org_id 行 = AP-3 cross-org owner-only enforce 的载体.
//  2. CREATE INDEX idx_user_permissions_org_id ON user_permissions(org_id)
//     WHERE org_id IS NOT NULL — sparse index 仅扫显式 org_id 行
//     (跟 ap_1_1 expires_at sparse index 同模式, 现网零开销).
//
// 反约束 (auth-permissions.md §5 + ap-3-spec.md §0 设计 ②):
//   - 不添加 NOT NULL — 现网行 org_id 全 NULL = legacy, 跟 AP-1 ABAC
//     行为零变.
//   - 不添加 default 值 — NULL 是合法终态 (跟 AP-1.1 expires_at 同精神,
//     0 / "" 不是合法 org_id 值).
//   - 不添加 FK org_id REFERENCES organizations(id) — 跟 user.org_id /
//     channels.org_id / messages.org_id 同精神 (CM-3 #208), 业务校验在
//     server 层做 (蓝图 §5 字面 "暂不业务化"); grep 检查 `user_permissions
//     .*FOREIGN KEY.*organizations` count==0 (sparse FK schema 留账).
//   - INDEX WHERE org_id IS NOT NULL — partial index, 现网零开销
//     (主键 + idx_user_permissions_lookup + idx_user_permissions_expires
//     不动).
//
// v=29 sequencing: AP-1.1 v=24 / AL-1.4 v=25 / DL-4.1 v=26 / HB-3.1 v=27 /
// CV-2 v2 v=28 (in flight #517) / **AP-3.1 v=29** (本 migration). registry.go
// 字面锁; CV-2 v2 / AP-3 同期 sequencing — merge order determines the version, later work moves forward (跟
// CV-2 v1 spec §2 v=14 三方撞号 sequencing 协议同).
//
// v0 stance: forward-only, no Down(). ALTER ADD COLUMN 在 SQLite
// idempotent-unsafe (重跑会报 duplicate column), engine 通过
// schema_migrations 版本号守 idempotency — 跟所有 ALTER 类 migration
// 同模式 (chn_3_1 / cm_3 / ap_1_1 等).
var userPermissionsOrg = Migration{
	Version: 29,
	Name:    "ap_3_1_user_permissions_org",
	Up: func(tx *gorm.DB) error {
		// Trimmed-schema gate (跟 ap_1_1 / cv_3_1 同模式 — 部分 migration
		// test 单独 register 此 migration 不带上游 user_permissions 表).
		exists, err := hasTable(tx, "user_permissions")
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		// ALTER ADD COLUMN — SQLite supports this without table rebuild
		// when no constraint is added. NULL default + nullable = 零行为变.
		if err := tx.Exec(`ALTER TABLE user_permissions ADD COLUMN org_id TEXT`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_permissions_org_id
			ON user_permissions(org_id) WHERE org_id IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
