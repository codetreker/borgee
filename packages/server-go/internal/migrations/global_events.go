package migrations

import "gorm.io/gorm"

// globalEvents is migration v=47 — DL-2 §3 events 双流 global-scoped 表.
//
// Spec: docs/implementation/modules/dl-2-spec.md §1 DL2.1.
// Blueprint: data-layer.md §4.A.4 (蓝图 §3.4 必落 4 类: perm grant/revoke /
// impersonate / agent state / admin force action — all global-scoped, not bound to
// channel).
//
// Schema (v=47, global-scoped events):
//   - lex_id      TEXT NOT NULL PRIMARY KEY  (ULID, monotonic)
//   - kind        TEXT NOT NULL              (e.g. "perm.grant", "impersonate.start")
/*   - payload     TEXT NOT NULL DEFAULT ''  (JSON-serialized event body) */
//   - created_at  INTEGER NOT NULL           (UnixMilli)
//   - retention_days INTEGER                 (NULL = use sweeper default)
//
// Indexes:
//   - idx_global_events_kind_lex (kind, lex_id DESC) — admin/ops query 路径
//   - idx_global_events_created  (created_at) — sweeper scan
//
// 反约束 (dl-2-spec.md §0):
//   - 跟 channel_events 各自单表分立 (channel-scoped vs global-scoped, so
//     privacy contracts for the 4 required event classes stay enforceable).
//   - retention_days NULL → mustPersistKinds 4 类永久 + 默认 90 天 + 其他
//     per-kind 覆盖 (SSOT enum 在 must_persist_kinds.go).
//
// v0 stance: forward-only, no Down().
var globalEvents = Migration{
	Version: 47,
	Name:    "global_events",
	Up: func(tx *gorm.DB) error {
		stmts := []string{
			`CREATE TABLE IF NOT EXISTS global_events (
  lex_id          TEXT    NOT NULL PRIMARY KEY,
  kind            TEXT    NOT NULL,
  payload         TEXT    NOT NULL DEFAULT '',
  created_at      INTEGER NOT NULL,
  retention_days  INTEGER
)`,
			`CREATE INDEX IF NOT EXISTS idx_global_events_kind_lex
				ON global_events(kind, lex_id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_global_events_created
				ON global_events(created_at)`,
		}
		for _, s := range stmts {
			if err := tx.Exec(s).Error; err != nil {
				return err
			}
		}
		return nil
	},
}
