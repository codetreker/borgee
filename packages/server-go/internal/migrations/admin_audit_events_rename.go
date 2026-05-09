package migrations

import "gorm.io/gorm"

// auditEventsRename is migration v=43 — Phase 4 / ADM-3.
//
// Blueprint锚: `admin-model.md` §3 audit-forward-only 单表 + ADM-2.1
// #484 admin_actions 表起源 + BPP-8 #532 5 plugin lifecycle 事件 →
// admin_actions (名实不符 后续). Spec: docs/implementation/modules/
// adm-3-spec.md §0 设计 ①+② + §1 拆段.
//
// What this migration does (RENAME 元数据 + alias view, **0 数据迁移**):
//
//   1. ALTER TABLE admin_actions RENAME TO audit_events
//      — SQLite RENAME 是元数据操作, 0 数据拷贝, 索引 + 触发器自动跟随.
//      新表名语义统一 "全 audit-forward-only 单表跨任意 actor type"
//      (跟 ADM-2.1 admin actor + BPP-8 plugin_system actor + sweeper
//      system actor 全装).
//   2. CREATE VIEW admin_actions AS SELECT * FROM audit_events
//      — backward compat alias. 既有 gorm `(AdminAction).TableName() ==
//      "admin_actions"` SELECT/INSERT/UPDATE 不破 (SQLite view 跟 table
//      读写透明 — INSERT 通过 INSTEAD OF 触发器路由, 见 step 3).
//   3. CREATE TRIGGER admin_actions_insert INSTEAD OF INSERT ON
//      admin_actions BEGIN INSERT INTO audit_events ... — 透明写路由.
//      UPDATE 通过 archive sweeper 走 audit_events 直接, 不 trigger.
//
// 设计沿用 (跟 spec §0):
//   - 设计 ① 表名改 audit_events — RENAME 元数据 0 数据迁移
//   - 设计 ② alias view backward compat — 战马原代码 0 改
//   - 设计 ③ ADM-0 §1.3 红线扩展 — 实施 PR 顺手改蓝图
//
// 反约束 (spec §2):
//   - 不裂表 — RENAME 后 audit_events 是单源, view 是 alias
//   - 不真删 forward-only — grep 检查 `DELETE FROM audit_events` 0 hit
//   - alias view INSERT 通过 trigger (SQLite views 不可直接写)
//
// v=43 sequencing: 跟 team-lead 占号 v=43 (留位 ADM-3 #570 spec). 顺
// 位 next free post chn-14 #584 v=36 / cv-15 v=38 / 等.
//
// v0 stance: forward-only, no Down() (跟 ADM-2.1 + AL-7.1 + AP-2.1 +
// BPP-8.1 + AP-1.1 + AP-3.1 跨七 milestone audit 同模式).
var auditEventsRename = Migration{
	Version: 43,
	Name:    "adm_3_1_audit_events_rename",
	Up: func(tx *gorm.DB) error {
		// Idempotent — if audit_events already exists (re-run), skip.
		if exists, err := hasTable(tx, "audit_events"); err != nil {
			return err
		} else if exists {
			return nil
		}
		// admin_actions table must exist — created by ADM-2.1 #484 v=22.
		if exists, err := hasTable(tx, "admin_actions"); err != nil {
			return err
		} else if !exists {
			return nil
		}

		// Step 1 — RENAME admin_actions → audit_events (元数据, 0 数据).
		if err := tx.Exec(`ALTER TABLE admin_actions RENAME TO audit_events`).Error; err != nil {
			return err
		}

		// Step 2 — alias view admin_actions → audit_events (backward compat
		// 设计 ②). SELECT 透明.
		if err := tx.Exec(`CREATE VIEW IF NOT EXISTS admin_actions AS SELECT * FROM audit_events`).Error; err != nil {
			return err
		}

		// Step 3 — INSTEAD OF INSERT trigger 路由 view → table (SQLite views
		// 不可直接写). 字段顺序跟 ADM-2.1 schema 字面 byte-identical (id +
		// actor_id + target_user_id + action + metadata + created_at +
		// archived_at AL-7.1).
		if err := tx.Exec(`CREATE TRIGGER IF NOT EXISTS admin_actions_insert
			INSTEAD OF INSERT ON admin_actions
			BEGIN
				INSERT INTO audit_events (id, actor_id, target_user_id, action, metadata, created_at, archived_at)
				VALUES (NEW.id, NEW.actor_id, NEW.target_user_id, NEW.action, NEW.metadata, NEW.created_at, NEW.archived_at);
			END`).Error; err != nil {
			return err
		}

		// Step 4 — INSTEAD OF UPDATE trigger 路由 view → table (sweeper
		// archive 走 audit_events 直接, view UPDATE 也透明).
		if err := tx.Exec(`CREATE TRIGGER IF NOT EXISTS admin_actions_update
			INSTEAD OF UPDATE ON admin_actions
			BEGIN
				UPDATE audit_events SET archived_at = NEW.archived_at WHERE id = NEW.id;
			END`).Error; err != nil {
			return err
		}

		return nil
	},
}
