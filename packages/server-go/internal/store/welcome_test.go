package store

import (
	"strings"
	"testing"
)

// TestCreateWelcomeChannelForUser_Success exercises the happy-path of
// CM-onboarding step 1 + step 2: a fresh user gets a type=system channel,
// a channel_member row, and exactly one system message carrying the
// quick_action payload.
func TestCreateWelcomeChannelForUser_Success(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	email := "alice@example.com"
	u := &User{DisplayName: "Alice", Role: "member", Email: &email, PasswordHash: "h"}
	if err := s.CreateUser(u); err != nil {
		t.Fatal(err)
	}

	ch, sysOK, err := s.CreateWelcomeChannelForUser(u.ID, u.DisplayName)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !sysOK {
		t.Fatalf("systemMessageOK = false, want true")
	}
	if ch == nil || ch.Type != "system" || ch.CreatedBy != u.ID {
		t.Fatalf("unexpected channel: %+v", ch)
	}
	if !strings.HasPrefix(ch.Name, "welcome-") {
		t.Fatalf("channel name = %q, want welcome-* prefix", ch.Name)
	}

	// channel_member exists.
	var mcount int64
	s.db.Raw("SELECT COUNT(*) FROM channel_members WHERE channel_id = ? AND user_id = ?", ch.ID, u.ID).Row().Scan(&mcount)
	if mcount != 1 {
		t.Fatalf("channel_members count = %d, want 1", mcount)
	}

	// Welcome system message present with quick_action populated.
	var (
		body string
		qa   *string
	)
	row := s.db.Raw(`
		SELECT content, quick_action FROM messages
		WHERE channel_id = ? AND sender_id = 'system'
	`, ch.ID).Row()
	if err := row.Scan(&body, &qa); err != nil {
		t.Fatalf("scan welcome message: %v", err)
	}
	if body != WelcomeMessageBody {
		t.Fatalf("body mismatch:\n got=%q\nwant=%q", body, WelcomeMessageBody)
	}
	if qa == nil || *qa != WelcomeQuickActionJSON {
		t.Fatalf("quick_action = %v, want %q", qa, WelcomeQuickActionJSON)
	}
}

// TestCreateWelcomeChannelForUser_Idempotent verifies re-running the helper
// for the same user does not create duplicate channels — the existing
// type=system row is returned.
func TestCreateWelcomeChannelForUser_Idempotent(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	email := "bob@example.com"
	u := &User{DisplayName: "Bob", Role: "member", Email: &email, PasswordHash: "h"}
	if err := s.CreateUser(u); err != nil {
		t.Fatal(err)
	}

	ch1, _, err := s.CreateWelcomeChannelForUser(u.ID, u.DisplayName)
	if err != nil {
		t.Fatal(err)
	}
	ch2, _, err := s.CreateWelcomeChannelForUser(u.ID, u.DisplayName)
	if err != nil {
		t.Fatal(err)
	}
	if ch1.ID != ch2.ID {
		t.Fatalf("expected idempotent: ch1=%s ch2=%s", ch1.ID, ch2.ID)
	}

	var n int64
	s.db.Raw("SELECT COUNT(*) FROM channels WHERE created_by = ? AND type = 'system'", u.ID).Row().Scan(&n)
	if n != 1 {
		t.Fatalf("system channels per user = %d, want 1", n)
	}
}

// TestCreateWelcomeChannelForUser_GracefulMessageFailure simulates the
// message-insert failure branch (onboarding-journey.md §3 step 2 ❌). We force
// the failure by dropping the messages table just before the call. The
// channel + channel_member must still commit and the helper must return
// systemMessageOK=false without an error.
func TestCreateWelcomeChannelForUser_GracefulMessageFailure(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	email := "carol@example.com"
	u := &User{DisplayName: "Carol", Role: "member", Email: &email, PasswordHash: "h"}
	if err := s.CreateUser(u); err != nil {
		t.Fatal(err)
	}

	// Break the message insert path.
	if err := s.db.Exec("DROP TABLE messages").Error; err != nil {
		t.Fatalf("drop messages: %v", err)
	}

	ch, sysOK, err := s.CreateWelcomeChannelForUser(u.ID, u.DisplayName)
	if err != nil {
		t.Fatalf("expected nil error (graceful), got %v", err)
	}
	if sysOK {
		t.Fatalf("expected systemMessageOK=false")
	}
	if ch == nil || ch.Type != "system" {
		t.Fatalf("channel must still commit: %+v", ch)
	}
	// channel_member also committed.
	var n int64
	s.db.Raw("SELECT COUNT(*) FROM channel_members WHERE channel_id = ? AND user_id = ?", ch.ID, u.ID).Row().Scan(&n)
	if n != 1 {
		t.Fatalf("channel_members count = %d, want 1 (channel must persist)", n)
	}
}

// TestListChannelsWithUnread_IncludesSystemWelcome guards bug-030: the
// channel list endpoint used to filter `c.type = 'channel'`, which silently
// dropped the freshly-created #welcome (type='system') row. The SPA never
// saw the channel, so auto-select couldn't pick it and the welcome system
// message never rendered (e2e: cm-onboarding.spec.ts:92 fail).
//
// Contract: a member of a type='system' channel MUST see it in
// ListChannelsWithUnread / ListAllChannelsForAdmin output.
func TestListChannelsWithUnread_IncludesSystemWelcome(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	email := "dave@example.com"
	u := &User{DisplayName: "Dave", Role: "member", Email: &email, PasswordHash: "h"}
	if err := s.CreateUser(u); err != nil {
		t.Fatal(err)
	}
	ch, _, err := s.CreateWelcomeChannelForUser(u.ID, u.DisplayName)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.ListChannelsWithUnread(u.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, c := range got {
		if c.ID == ch.ID && c.Type == "system" && c.IsMember {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("welcome (type=system) channel missing from ListChannelsWithUnread output: %+v", got)
	}

	// A non-member must NOT see another user's type=system channel — the
	// welcome row is per-user-private. Create a second user and assert.
	email2 := "eve@example.com"
	u2 := &User{DisplayName: "Eve", Role: "member", Email: &email2, PasswordHash: "h"}
	if err := s.CreateUser(u2); err != nil {
		t.Fatal(err)
	}
	got2, err := s.ListChannelsWithUnread(u2.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got2 {
		if c.ID == ch.ID {
			t.Fatalf("user %s should NOT see other user's welcome channel %s", u2.ID, ch.ID)
		}
	}
}

// TestWelcomeConstantsMirrorMigrations was removed in the migration baseline
// squash: the migrations/cm_onboarding_welcome.go copy it cross-checked against
// no longer exists. store/welcome.go is now the single owner of the welcome
// literals.
