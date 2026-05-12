package migrations

import (
	"gorm.io/gorm"
)

// impersonationGrants is migration v=23 — Phase 4 / ADM-2.2.
//
// Blueprint锚: `admin-model.md` §1.4 (L91 "被 impersonate 用户" 行 — 红色横幅 +
// 24h 倒计时) + §3 (impersonation_grants 数据模型片段, "由 user 创建, admin
// 仅消费这条记录") + §4.1 R3 (ADM-1 文案 "24h 时窗顶部红色横幅常驻可随时撤销"
// 兑现锚).
// Acceptance: `docs/qa/acceptance-templates/adm-2.md` §impersonate 红横幅
// 4.2.a + 4.2.b.
// Spec: `docs/implementation/modules/adm-2-spec.md` §2.5.
//
// What this migration does:
//  1. CREATE TABLE impersonation_grants:
//     - id          TEXT    NOT NULL PRIMARY KEY  (UUID)
//     - user_id     TEXT    NOT NULL              (FK users.id; 业主自己 grant)
//     - granted_at  INTEGER NOT NULL              (Unix ms)
//     - expires_at  INTEGER NOT NULL              (granted_at + 24h, server
//     固定不接受 client 传)
//     - revoked_at  INTEGER NULL                  (业主主动撤销时 stamp;
//     NULL 表示有效)
//  2. CREATE INDEX idx_impersonation_grants_user_id_expires
//     ON impersonation_grants(user_id, expires_at DESC) — ActiveGrant
//     high-frequency query path (server validates before admin write actions,
//     设计 ⑦).
//
// 反约束 (acceptance §4.2 + spec §2.5 + stance §1 设计 ⑦):
//   - 蓝图 §3 字面 "由 user 创建": 此表 INSERT 路径仅从 user-rail 进入
//     (POST /api/v1/me/impersonation-grants 走 user cookie), admin SPA
//     不开授予自己 impersonate 路径 (grep 检查 `force_impersonate\|
//     admin_impersonate_self` count==0)
//   - 期限固定 24h: server 端 expires_at = granted_at + 24h, schema 不添加
//     CHECK 但 server 校验; 反约束: client 传 expires_at 字段 server 忽略
//   - revoked_at 是唯一允许的 UPDATE 路径 (forward-only 设计 ⑤ 例外 — 撤销
//     是业主主动权, 不是 audit 改写). 跟 admin_actions 设计 ⑤ "audit 不可改写"
//     精神不冲突 (admin_actions 是历史记录, impersonation_grants 是状态记录)
//   - 反向列名: 不添加 `actor_id` (admin 不在此表, 蓝图 §3 字面 "admin 仅消费
//     这条记录"); 不添加 `cursor` (跟 al_3_1 / cv_4_1 / chn_3_1 / al_2a_1 /
//     adm_2_1 同模式 frame 路径不下沉 schema)
//
// v=23 sequencing: CV-2.1 v=14 / DM-2.1 v=15 / AL-4.1 v=16 / CV-3.1 v=17 /
// CV-4.1 v=18 / CHN-3.1 v=19 / AL-2a.1 v=20 (#447) / AL-1b.1 v=21 (#453) /
// ADM-2.1 v=22 (admin_actions) / **ADM-2.2 v=23** (this migration,
// reserved for concurrent sequencing work).
// registry.go 字面锁.
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守
// idempotency.
var impersonationGrants = Migration{
	Version: 23,
	Name:    "adm_2_2_impersonation_grants",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS impersonation_grants (
  id          TEXT    NOT NULL PRIMARY KEY,
  user_id     TEXT    NOT NULL,
  granted_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL,
  revoked_at  INTEGER NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_impersonation_grants_user_id_expires
			ON impersonation_grants(user_id, expires_at DESC)`).Error; err != nil {
			return err
		}
		return nil
	},
}
