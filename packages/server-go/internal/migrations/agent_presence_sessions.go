package migrations

import (
	"gorm.io/gorm"
)

// presenceSessions is migration v=12 — Phase 4 / AL-3.1.
//
// Blueprint锚: `agent-lifecycle.md` §2.2 (默认 remote-agent — /ws hub
// 心跳决定 reach) + §2.3 (四态机含 offline 态). #277 contract stub
// (`internal/presence/contract.go`) 已锁 PresenceTracker 接口的 read
// 端 (`IsOnline` + `Sessions`); AL-3.1 落数据契约, AL-3.2 接 hub
// lifecycle hook (写端 `TrackOnline` / `TrackOffline`).
//
// What this migration does:
//   1. CREATE TABLE presence_sessions:
//        - session_id        TEXT NOT NULL UNIQUE  (一 session 一行;
//                                                  UNIQUE 不是 PK ——
//                                                  跟 #302 sync patch
//                                                  一致, id 列做 PK 留
//                                                  AUTOINCREMENT 行号)
//        - user_id           TEXT NOT NULL         (人 + agent 共用)
//        - agent_id          TEXT                  (NULL = 人 session;
//                                                  agent session 时落
//                                                  agent 的 user_id 副本
//                                                  方便 IsOnline(agentID)
//                                                  快查; #301 spec §0
//                                                  设计 ③ 多 session per
//                                                  user 合法)
//        - connected_at      INTEGER NOT NULL      (Unix ms)
//        - last_heartbeat_at INTEGER NOT NULL      (Unix ms; #302 §1.1
//                                                  字面 last_heartbeat_at
//                                                  非 last_seen_at)
//   2. CREATE INDEX idx_presence_sessions_user_id (IsOnline O(1) lookup
//      必需 — #301 spec §1 AL-3.1 验收行).
//   3. CREATE INDEX idx_presence_sessions_agent_id WHERE agent_id IS
//      NOT NULL (mention 路由热路径; agent fallback 查 IsOnline 用,
//      跟 DM-2 §2.2 contract 对齐).
//
// 反约束 (PR-spec #301 §0 + §4 + acceptance §5.3):
//   - presence_sessions 不挂 cursor 列 (跟 RT-1 cursor 序列拆死;
//     瞬时态 vs 不可回退序列, 数据特性硬拆).
//   - 不挂 last_seen_at / busy / idle 字段 (busy/idle 留 BPP-1 #280
//     同期; last_seen_at 设计争议留 Phase 5+).
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, 无 trimmed-schema
// 兼容路径需要; 直接 IF NOT EXISTS 守住 idempotency.
var presenceSessions = Migration{
	Version: 12,
	Name:    "al_3_1_presence_sessions",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS presence_sessions (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id        TEXT    NOT NULL UNIQUE,
  user_id           TEXT    NOT NULL,
  agent_id          TEXT,
  connected_at      INTEGER NOT NULL,
  last_heartbeat_at INTEGER NOT NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_presence_sessions_user_id
			ON presence_sessions(user_id)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_presence_sessions_agent_id
			ON presence_sessions(agent_id) WHERE agent_id IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
