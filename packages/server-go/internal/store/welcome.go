package store

import (
	"errors"
	"time"


	"borgee-server/internal/idgen"
	"gorm.io/gorm"
)

// CM-onboarding (#42) — Welcome channel + system message helpers.
//
// Blueprint: docs/blueprint/current/concept-model.md §10 + onboarding-journey.md §3.
//
// Contract (handler-level invariant per acceptance-templates/cm-onboarding.md
// "数据契约 (步骤 1)"):
//   - org + user + #welcome (kind=system) + channel_member + system message
//     are written in a SINGLE transaction.
//   - host-bridge / push / external IO is NOT in this transaction (callers
//     must compose).
//
// The system message body and the quick_action button JSON are mirrored from
// migrations.WelcomeMessageBody / migrations.WelcomeQuickActionJSON. We
// duplicate them here as constants to avoid an import cycle (store →
// migrations is forbidden; migrations consumes store models). If they ever
// diverge, the unit test in this package will catch it.

// WelcomeMessageBody must equal migrations.WelcomeMessageBody. Lock per
// onboarding-journey.md §3 step 2 success state — change requires 野马 +1.
const WelcomeMessageBody = "**欢迎来到 Borgee 👋**\n\n" +
	"这里是你的工作区。Borgee 不是一个 AI 工具, 而是让你和 AI 同事一起协作的地方。\n\n" +
	"第一步: 创建你的第一个 agent 同事 →"

// WelcomeQuickActionJSON must equal migrations.WelcomeQuickActionJSON.
const WelcomeQuickActionJSON = `{"kind":"button","label":"创建 agent","action":"open_agent_manager"}`

// CreateWelcomeChannelForUser provisions the per-user #welcome channel + the
// initial system message in a single transaction. Returns the channel and a
// "system message attempted" flag — when the system message insert fails, the
// channel is still created but the caller should surface the §11 reduced
// state on the client (channel header retry pill).
//
// Idempotent: if a type='system' channel already exists for the user, the
// existing row is returned and no new message is inserted.
//
// Channel name is "welcome-<userid-prefix>" because channels.name is globally
// UNIQUE; the client renders the channel by id, so the cosmetic "welcome"
// label can collide-proof itself without changing copy.
//
// systemMessageOK is false only when the message insert errored — the channel
// row is committed regardless. This matches the onboarding-journey.md §3
// step 1 vs step 2 split: channel = hard contract, message = graceful.
func (s *Store) CreateWelcomeChannelForUser(userID, displayName string) (channel *Channel, systemMessageOK bool, err error) {
	if userID == "" {
		return nil, false, errors.New("user id required")
	}
	now := time.Now().UnixMilli()

	// First check (outside tx) for idempotency to avoid unique-constraint
	// noise on re-registration. This is best-effort; the inner tx still uses
	// FirstOrCreate semantics.
	var existing Channel
	if err := s.db.Where("created_by = ? AND type = ? AND deleted_at IS NULL", userID, "system").
		First(&existing).Error; err == nil {
		return &existing, true, nil
	}

	ch := &Channel{
		ID:         idgen.NewID(),
		Name:       "welcome-" + shortPrefix(userID),
		Topic:      "",
		Visibility: "private",
		CreatedAt:  now,
		CreatedBy:  userID,
		Type:       "system",
		Position:   "0|aaaaaa",
	}

	systemMessageOK = true
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ch).Error; err != nil {
			return err
		}
		member := &ChannelMember{
			ChannelID: ch.ID,
			UserID:    userID,
			JoinedAt:  now,
		}
		if err := tx.Where("channel_id = ? AND user_id = ?", ch.ID, userID).
			FirstOrCreate(member).Error; err != nil {
			return err
		}
		// Insert the welcome system message via raw SQL so the quick_action
		// column (added by migration v=7) is populated. The message FK
		// requires sender_id='system' to exist; that row is seeded by the
		// same migration.
		msgID := idgen.NewID()
		if err := tx.Exec(`
			INSERT INTO messages (id, channel_id, sender_id, content, content_type, created_at, quick_action)
			VALUES (?, ?, 'system', ?, 'text', ?, ?)
		`, msgID, ch.ID, WelcomeMessageBody, now, WelcomeQuickActionJSON).Error; err != nil {
			// Per onboarding-journey.md §3 step 2 error branch: do NOT abort
			// the channel. Mark systemMessageOK=false and let the caller
			// continue. We surface the failure by returning a nil error on
			// the outer tx (so the channel commits) but flipping the flag.
			systemMessageOK = false
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return ch, systemMessageOK, nil
}

// shortPrefix returns up to the first 8 chars of the given id.
//
// ULID-MIGRATION caveat: ULID first 10 chars encode timestamp millis; users
// registered in the same millisecond/second window share identical prefix
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
