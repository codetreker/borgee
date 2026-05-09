package migrations

import (
	"gorm.io/gorm"
)

// adminActions is migration v=22 — Phase 4 / ADM-2.1.
//
// Blueprint锚: `admin-model.md` §1.4 (L82-105 "谁能看到什么" 四档分层) +
// §2 不变量 (L109-120 受影响者必感知 + Audit 100% 留痕 + 分层可见).
// Acceptance: `docs/qa/acceptance-templates/adm-2.md` §数据契约 (admin_actions
// schema 字段 + 索引 + action 类型枚举 DB CHECK).
// Implementation: `docs/architecture/admin-model.md` §ADM-2 (R2 取消
// ⭐ 标志性 — 内部 milestone, 不进野马签字流; 但兑现 §4.1 ADM-1 隐私承诺页
// "你能在设置看到 admin 影响记录" 文案).
// 依赖: ADM-1 (PR #455+#459+#464) ✅ 已落.
//
// What this migration does:
//   1. CREATE TABLE admin_actions:
//        - id              TEXT    NOT NULL              (PK; UUID)
//        - actor_id        TEXT    NOT NULL              (FK admins.id; 逻辑 FK
//                                                         跟 al_3_1 / cv_4_1 等
//                                                         同模式 — SQLite FK
//                                                         enforcement off)
//        - target_user_id  TEXT    NOT NULL              (FK users.id; 逻辑 FK;
//                                                         必填: 蓝图 §1.4 红线 1
//                                                         "受影响者必收 system
//                                                         message" — 必有受影响
//                                                         user)
//        - action          TEXT    NOT NULL              (枚举 CHECK; 见下方
//                                                         AcceptedActions)
//        - metadata        TEXT    NOT NULL DEFAULT ''   (JSON; 可选上下文,
//                                                         channel_id / 旧值 /
//                                                         新值, 不挂 schema
//                                                         CHECK, server 校验)
//        - created_at      INTEGER NOT NULL              (Unix ms, 跟 al_3_1 /
//                                                         cv_4_1 同模式)
//        - PRIMARY KEY (id)
//   2. CHECK constraint on action: 5 个枚举值 byte-identical
//      ('delete_channel' / 'suspend_user' / 'change_role' / 'reset_password' /
//      'start_impersonation'). 反向 reject 同义词 / 大小写漂移 / 字典外值.
//   3. CREATE INDEX idx_admin_actions_target_user_id_created_at
//      ON admin_actions(target_user_id, created_at DESC) — 受影响者
//      GET /api/v1/me/admin-actions 热路径 (蓝图 §1.4 "user 只见与己相关" 行).
//   4. CREATE INDEX idx_admin_actions_actor_id_created_at
//      ON admin_actions(actor_id, created_at DESC) — admin SPA
//      GET /admin-api/v1/audit-log 热路径 (蓝图 §1.4 "admin 之间互相可见" 行).
//
// 反约束 (acceptance + admin-model.md §1.4 红线 + §2 不变量):
//   - 蓝图 §1.4 红线 1 "受影响者必收 system message": target_user_id NOT NULL
//     强制 — 没有受影响者的 admin action (例如 admin 之间互相 promote / demote)
//     不进此表, 走单独的 admin internal log (ADM-2 v1 不开).
//   - 蓝图 §1.4 红线 3 "admin 之间互相留痕": admin SPA 任意写动作 INSERT 此表
//     是规则 (server 实施在 ADM-2.2 — 此 schema 仅锁数据契约).
//   - 蓝图 §2 不变量 "Audit 100% 留痕": forward-only schema, 不开 DELETE /
//     UPDATE 路径 (server 实施在 ADM-2.2 — 此 schema 不挂 ON DELETE CASCADE,
//     跟 al_3_1 / cv_4_1 同精神 lazy GC).
//   - ADM-0 红线 (admin ∉ users 表): actor_id FK admins.id (独立表),
//     target_user_id FK users.id; grep 检查 `actor_id.*users\b` 在 schema /
//     handler count==0.
//   - 反约束列名: 不挂 'updated_at' (audit 不可改写, forward-only) /
//     'org_id' (受影响者 org 通过 users.org_id 派生, 不冗余存) /
//     'session_id' (impersonate 走单独 impersonation_grants 表, 不混入此表).
//
// v=22 sequencing: CV-2.1 v=14 / DM-2.1 v=15 / AL-4.1 v=16 / CV-3.1 v=17 /
// CV-4.1 v=18 / CHN-3.1 v=19 / AL-2a.1 v=20 (#447) / AL-1b.1 v=21 (#453) /
// AL-2b.1 (BPP frame 无 migration, 不占号) / **ADM-2.1 v=22** (本 migration).
// registry.go 字面锁.
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守
// idempotency.
var adminActions = Migration{
	Version: 22,
	Name:    "adm_2_1_admin_actions",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS admin_actions (
  id              TEXT    NOT NULL PRIMARY KEY,
  actor_id        TEXT    NOT NULL,
  target_user_id  TEXT    NOT NULL,
  action          TEXT    NOT NULL CHECK (action IN ('delete_channel','suspend_user','change_role','reset_password','start_impersonation')),
  metadata        TEXT    NOT NULL DEFAULT '',
  created_at      INTEGER NOT NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_target_user_id_created_at
			ON admin_actions(target_user_id, created_at DESC)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_actor_id_created_at
			ON admin_actions(actor_id, created_at DESC)`).Error; err != nil {
			return err
		}
		return nil
	},
}
