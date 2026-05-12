package migrations

import (
	"borgee-server/internal/idgen"
	"gorm.io/gorm"
)

// cmOnboardingWelcome is migration v7 — Phase 2 / CM-onboarding schema landing.
//
// Blueprint: concept-model.md §10 + onboarding-journey.md (v1, R2 merged).
//
// What this migration does:
//  1. Adds `messages.quick_action` (TEXT, JSON-encoded, NULL by default) so a
//     system message can carry a single quick action button. Schema is
//     `{"kind":"button","label":string,"action":string}` for v0.
//  2. Seeds the `system` user row (id='system') so messages with
//     sender_id='system' satisfy the FK constraint. The row is disabled.
//     Idempotent via INSERT OR IGNORE.
//  3. Backfills #welcome for existing users without one (per
//     onboarding-journey.md §4 invariant 4.1: every user must land on a
//     non-empty channel). The handler-level auto-create lives in
//     api/auth.go (handleRegister) and api/admin.go (handleCreateUser); see
//     store.CreateWelcomeChannelForUser.
//
// Field invariants:
//   - quick_action is NULL for non-onboarding messages (CM-onboarding scope).
//   - sender_id='system' messages render specially in the client (existing
//     MessageItem.tsx behavior, no client change needed for that branch).
//
// CM-onboarding scope (strict):
//   - Schema column + system user seed + per-user backfill only.
//   - No history rewrite for messages (handled by v1 work).
var cmOnboardingWelcome = Migration{
	Version: 7,
	Name:    "cm_onboarding_welcome",
	Up: func(tx *gorm.DB) error {
		// 1. Column add (SQLite-friendly: TEXT, default NULL).
		if err := tx.Exec(`ALTER TABLE messages ADD COLUMN quick_action TEXT`).Error; err != nil {
			return err
		}
		// Step 2 + 3 require the full users / channels / messages schema. On
		// minimal scaffolds (e.g. migration unit tests where seedLegacyTables
		// only creates `(id TEXT PRIMARY KEY)` placeholders), the column-add
		// alone is the meaningful effect — the per-user seed/backfill is a
		// no-op. Probe required columns and gracefully skip the rest.
		if !hasColumns(tx, "users", "display_name", "role", "disabled", "require_mention", "org_id", "deleted_at") {
			return nil
		}
		if !hasColumns(tx, "channels", "name", "topic", "visibility", "created_by", "type", "position", "deleted_at") {
			return nil
		}
		if !hasColumns(tx, "channel_members", "channel_id", "user_id", "joined_at") {
			return nil
		}
		if !hasColumns(tx, "messages", "channel_id", "sender_id", "content", "content_type", "quick_action") {
			return nil
		}
		// 2. System user seed (FK target for sender_id='system').
		if err := tx.Exec(`
			INSERT OR IGNORE INTO users (id, display_name, role, created_at, disabled, require_mention, org_id)
			VALUES ('system', '系统', 'system', 0, 1, 0, '')
		`).Error; err != nil {
			return err
		}
		// 3. Backfill #welcome for users that have none. Detect "no welcome"
		// by checking type='system' channels owned by (created_by=) the user.
		var users []struct {
			ID          string `gorm:"column:id"`
			DisplayName string `gorm:"column:display_name"`
		}
		if err := tx.Raw(`
			SELECT u.id, u.display_name FROM users u
			WHERE u.id != 'system'
			  AND u.deleted_at IS NULL
			  AND NOT EXISTS (
			    SELECT 1 FROM channels c
			    WHERE c.created_by = u.id AND c.type = 'system' AND c.deleted_at IS NULL
			  )
		`).Scan(&users).Error; err != nil {
			return err
		}
		now := nowMillis(tx)
		for _, u := range users {
			// Channel name must be globally UNIQUE (channels.name); embed the
			// owner's UUID short-prefix so backfills don't collide. The full
			// channel id is a fresh UUID.
			chID := idgen.NewID()
			chName := "welcome-" + shortPrefix(u.ID)
			if err := tx.Exec(`
				INSERT OR IGNORE INTO channels
				  (id, name, topic, visibility, created_at, created_by, type, position)
				VALUES (?, ?, '', 'private', ?, ?, 'system', '0|aaaaaa')
			`, chID, chName, now, u.ID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`
				INSERT OR IGNORE INTO channel_members (channel_id, user_id, joined_at)
				VALUES (?, ?, ?)
			`, chID, u.ID, now).Error; err != nil {
				return err
			}
			// Best-effort welcome system message; failure does NOT abort the
			// backfill (per onboarding-journey.md §3 step 2 ❌ branch — the
			// channel is the hard contract; the message is graceful).
			_ = tx.Exec(`
				INSERT OR IGNORE INTO messages
				  (id, channel_id, sender_id, content, content_type, created_at, quick_action)
				VALUES (?, ?, 'system', ?, 'text', ?, ?)
			`, idgen.NewID(), chID, WelcomeMessageBody, now, WelcomeQuickActionJSON).Error
		}
		return nil
	},
}

// WelcomeMessageBody is the locked welcome copy from onboarding-journey.md
// §3 step 2 success state. Any change requires a content-lock update (see CODEOWNERS).
//
// Plain-text format (rendered with markdown by MessageItem.tsx):
const WelcomeMessageBody = "**欢迎来到 Borgee 👋**\n\n" +
	"这里是你的工作区。Borgee 不是一个 AI 工具, 而是让你和 AI 同事一起协作的地方。\n\n" +
	"第一步: 创建你的第一个 agent 同事 →"

// WelcomeQuickActionJSON is the JSON-encoded quick_action button payload for
// the welcome system message. Kept as a literal string to keep the migration
// init free of encoding/json. Schema:
//   - kind:  "button" (only kind in v0)
//   - label: button text shown to user
//   - action: client action key; "open_agent_manager" → setShowAgents(true).
const WelcomeQuickActionJSON = `{"kind":"button","label":"创建 agent","action":"open_agent_manager"}`

// nowMillis returns SQLite's current epoch millis. Avoids importing time at
// the migration package level so test seams stay deterministic via the tx.
func nowMillis(tx *gorm.DB) int64 {
	var n int64
	if err := tx.Raw("SELECT CAST(strftime('%s','now') AS INTEGER) * 1000").Scan(&n).Error; err != nil {
		return 0
	}
	return n
}

// shortPrefix returns up to the last 8 chars of the given id for use as a
// human-readable channel name discriminator.
//
// ULID-MIGRATION caveat: ULID first 10 chars encode timestamp millis →
// users registered in the same millisecond window share identical prefix
// and collide on channels (org_id, name) UNIQUE. UUID-36 first 8 chars
// are random hex (collision-resistant). To preserve uniqueness regardless
// of ID format, we use the LAST 8 chars (entropy tail in ULID, last hex
// segment in UUID), giving ≥40 bits of randomness either way.
func shortPrefix(id string) string {
	if len(id) >= 8 {
		return id[len(id)-8:]
	}
	return id
}

// hasColumns reports whether `table` exists and contains every name in `cols`.
// Used to guard step 2/3 of the onboarding migration on minimal test scaffolds
// where seed tables are only `(id TEXT PRIMARY KEY)` placeholders.
func hasColumns(tx *gorm.DB, table string, cols ...string) bool {
	rows, err := tx.Raw("PRAGMA table_info(" + table + ")").Rows()
	if err != nil {
		return false
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    *string
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		have[name] = true
	}
	for _, c := range cols {
		if !have[c] {
			return false
		}
	}
	return true
}
