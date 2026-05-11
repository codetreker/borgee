package migrations

import (
	"gorm.io/gorm"
)

// hostGrants is migration v=27 for the HB-3.1 host_grants schema.
//
// Blueprint出处: `host-bridge.md` §1.3 (情境化授权 4 类: install / exec /
// filesystem / network) + §1.5 release gate 第 5 行 (撤销 grant → daemon
// < 100ms 拒绝) + §2 信任五支柱第 3 条 (可审计日志).
// Spec brief: `docs/implementation/modules/hb-3-spec.md` §0+§1 HB-3.1.
// Acceptance: `docs/qa/acceptance-templates/hb-3.md` §1.
// 原则: `docs/qa/hb-3-stance-checklist.md` §1+§2+§3.
//
// What this migration does:
//  1. CREATE TABLE host_grants:
//     - id          TEXT    PRIMARY KEY        (UUID)
//     - user_id     TEXT    NOT NULL           (逻辑 FK users.id;
//     跟 al_3_1 / al_4_1 / cv_2_1 /
//     chn_3_1 / al_2a_1 同模式)
//     - agent_id    TEXT    NULL               (逻辑 FK users.id agent
//     行; install/exec 类 grant
//     是 user-level — agent_id NULL;
//     filesystem/network 类是
//     per-agent — agent_id NOT NULL;
//     蓝图 §1.3 "per-agent subset")
//     - grant_type  TEXT    NOT NULL CHECK     (4-enum from blueprint §1.3:
//     install/exec/filesystem/network)
//     - scope       TEXT    NOT NULL           (JSON; filesystem 是 path 串,
//     network 是 domain, install/
//     exec 是 runtime id; this field does not reuse
//     user_permissions.scope 字面)
//     - ttl_kind    TEXT    NOT NULL CHECK     (2-enum used by permission UX:
//     one_shot/always)
//     - granted_at  INTEGER NOT NULL           (Unix ms)
//     - expires_at  INTEGER NULL               (Unix ms; ttl_kind=one_shot 时
//     填 now+1h, always 时 NULL)
//     - revoked_at  INTEGER NULL               (Unix ms; DELETE 路径 stamp,
//     forward-only 不真删行 — 留作
//     audit; daemon 每次 SELECT 守
//     `revoked_at IS NULL` 实现 < 100ms
//     撤销, HB-4 §1.5 release gate
//     第 5 行)
//  2. CREATE INDEX idx_host_grants_user_id ON host_grants(user_id) — owner
//     lookup 热路径 (cross-user 403 ACL).
//  3. CREATE INDEX idx_host_grants_agent_id ON host_grants(agent_id) — daemon
//     SELECT (agent_id, scope) 校验热路径; partial index `WHERE agent_id IS NOT
//     NULL` 防 install/exec 类 row 占索引页.
//
// Constraints (hb-3-spec.md §0 + §3 + 原则 §1+§2+§3):
//   - 设计 ① schema 单一来源 — HB-3 owns the schema; HB-2 daemon (Go module
//     `packages/borgee-helper/`) is a current read-only consumer.
//   - 设计 ② 字典分立 — 不复用 AP-1 user_permissions schema (host vs runtime
//     两层独立); grant_type 4-enum 跟 user_permissions.permission 4 域 字面
//     集不交; handler code must not join `host_grants` to `user_permissions`.
//   - 设计 ③ audit log 5 字段 aligned with BPP-4 #499 DeadLetterAuditEntry
//     (此 schema 不挂 audit 列 — audit 走 BPP-4 LogFrameDroppedPluginOffline
//     通道延伸 + four related test areas: HB-1 install + HB-2 host-IPC + BPP-4
//     dead-letter + HB-3 grants).
//   - 不挂 ON DELETE CASCADE (forward-only revoke, 蓝图 §1.3 + §2 信任 — grants
//     行删后审计断链).
//   - 不挂 cursor 列 (跟 RT-1 envelope cursor 真分清, 跟 al_3_1 / al_4_1 / cv_*_1 /
//     dm_2_1 / al_2a_1 / al_1b_1 同模式).
//   - 不挂 admin-wide grant 列 (admin 不撤销用户 grant — 用户主权, ADM-0 §1.3
//     红线).
//
// Migration style: forward-only, no Down(). IF NOT EXISTS preserves idempotency.
// Logical FKs follow the al_3_1 / al_4_1 / cv_2_1 / dm_2_1 / al_2a_1 / al_1b_1 pattern.
//
// v=27 sequencing: AL-1.4 v=25 ✅ (#492) / DL-4.1 v=26 ✅ (#502 web_push) /
// **HB-3.1 v=27** (this migration).
var hostGrants = Migration{
	Version: 27,
	Name:    "hb_3_1_host_grants",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS host_grants (
  id          TEXT    PRIMARY KEY,
  user_id     TEXT    NOT NULL,
  agent_id    TEXT,
  grant_type  TEXT    NOT NULL CHECK (grant_type IN ('install','exec','filesystem','network')),
  scope       TEXT    NOT NULL,
  ttl_kind    TEXT    NOT NULL CHECK (ttl_kind IN ('one_shot','always')),
  granted_at  INTEGER NOT NULL,
  expires_at  INTEGER,
  revoked_at  INTEGER
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_host_grants_user_id
			ON host_grants(user_id)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_host_grants_agent_id
			ON host_grants(agent_id) WHERE agent_id IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
