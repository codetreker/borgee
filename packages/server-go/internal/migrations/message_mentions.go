package migrations

import (
	"gorm.io/gorm"
)

// messageMentions is migration v=15 — Phase 3 / DM-2.1.
//
// Blueprint锚: `concept-model.md` §4 (agent 代表自己 — mention 只 ping
// target, 不抄送 owner) + §4.1 (离线 fallback — owner 系统 DM + 节流
// 5 分钟/channel + ❌ 不转发原始内容) + §13 隐私默认.
// Spec brief: `docs/implementation/modules/dm-2-spec.md` (#312, 3 条原则
// + 3 拆段). Acceptance template: `docs/qa/acceptance-templates/dm-2.md`
// (#293) §1 schema 数据契约 5 行. Content lock: `docs/qa/dm-2-content-lock.md`
// (#314).
//
// What this migration does:
//  1. CREATE TABLE message_mentions:
//     - id              INTEGER PRIMARY KEY AUTOINCREMENT (audit 序;
//     同 CV-2.1
//     anchor_comments.id
//     同模式)
//     - message_id      TEXT    NOT NULL              (FK messages.id;
//     逻辑 FK, 跟 cv_1_1
//     / cv_2_1 / al_3_1
//     同模式 SQLite FK
//     默认禁用. mention
//     归属 = message,
//     软删随 message)
//     - target_user_id  TEXT    NOT NULL              (FK users.id;
//     设计 ⑥ user / agent
//     同表同语义 — agents
//     也是 users.role='agent',
//     一列搞定; 无独立
//     target_kind 字段)
//     - created_at      INTEGER NOT NULL              (Unix ms)
//     - UNIQUE(message_id, target_user_id)            (acceptance §1.0.b
//     dedup 同 target ——
//     重复 `@<id>` 同
//     message 只一行;
//     设计 ⑥ agent / human
//     同语义无歧义)
//  2. CREATE INDEX idx_message_mentions_target_user_id
//     ON message_mentions(target_user_id)             (mention routing hot path
//     — fanout 时按 target
//     查; acceptance §1.0.c)
//
// 反约束 (dm-2-spec.md §0 + §3 + acceptance §1.0.e):
//   - 表无 `cursor` 列 (跟 RT-1.1 #290 envelope cursor 保持分离, 同 al_3_1
//     / cv_1_1 / cv_2_1 模式 — RT-1 单调发号是 frame 路径, 不下沉到 mention
//     schema).
//   - 表无 `fanout_to_owner_id` / `cc_owner_id` 列 (设计 ③ 蓝图 §4 mention
//     must never copy owner; offline fallback 走独立 `type=system` message 自带
//     channel 字段, 不在 mention 行写 owner 路由信息).
//   - 不添加 `target_kind` enum 列 (设计 ⑥ user / agent 同语义 —
//     `users.role` 已区分, mention 路径不分叉; 反约束跟 anchor_comments
//     不复用 committer_kind 同思路, 避免 schema 把"语义同"硬拆"列两份").
//   - 不添加 `read_at` / `acknowledged_at` (mention 阅读态由 Phase 5+ 覆盖,
//     acceptance §6 反约束).
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守
// idempotency. 跟 cv_1_1 / cv_2_1 / al_3_1 / cm_4_0 同模式逻辑 FK.
//
// v=15 sequencing (#312 spec §1 / #356 spec v2 §2): DM-2.1 / CV-2.1 /
// CHN-2.1 all proposed v=14; CV-2.1 took v=14 (#359 merged), DM-2.1
// moved to v=15; CHN-2.1 has no schema change (soft constraint in server)
// and does not allocate a migration version. AL-4.1 moved to v=16.
var messageMentions = Migration{
	Version: 15,
	Name:    "dm_2_1_message_mentions",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS message_mentions (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  message_id     TEXT    NOT NULL,
  target_user_id TEXT    NOT NULL,
  created_at     INTEGER NOT NULL,
  UNIQUE(message_id, target_user_id)
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_message_mentions_target_user_id
			ON message_mentions(target_user_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}
