package migrations

import "gorm.io/gorm"

// channelEvents is migration v=46 — DL-2 §3 events 双流 channel-scoped 表.
//
// Spec: docs/implementation/modules/dl-2-spec.md §1 DL2.1.
// Blueprint: data-layer.md §4.A.4 (lex_id ULID PK + 蓝图 §3.4 必落 4 类).
//
// Schema (v=46, channel-scoped events):
//   - lex_id      TEXT NOT NULL PRIMARY KEY  (ULID, monotonic per producer)
//   - channel_id  TEXT NOT NULL              (logical FK to channels.id)
//   - kind        TEXT NOT NULL              (e.g. "perm.grant", "channel.archived")
//   - payload     TEXT NOT NULL DEFAULT ''   (JSON-serialized event body)
//   - created_at  INTEGER NOT NULL           (UnixMilli)
//   - retention_days INTEGER                 (NULL = use sweeper default; per-row override)
//
// Indexes:
//   - idx_channel_events_channel_lex (channel_id, lex_id DESC) — hot replay path
//   - idx_channel_events_kind_created (kind, created_at) — sweeper scan
//
// 反约束 (dl-2-spec.md §0):
//   - lex_id PK (ULID, 跟 RT-1.3 cursor replay byte-identical 设计).
//   - retention_days NULL = sweeper default (per-kind enum SSOT, 反 inline 字面漂).
//   - 不裂表 (channel_events 单表; agent_task / artifact 类按 kind 字段筛, 反 N 表漂).
//
// v0 stance: forward-only, no Down() (蓝图 §4.A.5 retention sweeper 真删行
// 是数据生命周期, 不是 schema 回退路径).
var channelEvents = Migration{
	Version: 46,
	Name:    "channel_events",
	Up: func(tx *gorm.DB) error {
		stmts := []string{
			`CREATE TABLE IF NOT EXISTS channel_events (
  lex_id          TEXT    NOT NULL PRIMARY KEY,
  channel_id      TEXT    NOT NULL,
  kind            TEXT    NOT NULL,
  payload         TEXT    NOT NULL DEFAULT '',
  created_at      INTEGER NOT NULL,
  retention_days  INTEGER
)`,
			`CREATE INDEX IF NOT EXISTS idx_channel_events_channel_lex
				ON channel_events(channel_id, lex_id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_channel_events_kind_created
				ON channel_events(kind, created_at)`,
		}
		for _, s := range stmts {
			if err := tx.Exec(s).Error; err != nil {
				return err
			}
		}
		return nil
	},
}
