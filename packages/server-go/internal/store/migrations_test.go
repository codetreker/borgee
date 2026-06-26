package store

import "testing"

func TestMigrateIdempotent(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	// Running twice should be idempotent
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateWithoutAdminSeedIsIdempotent(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	// Run migrate again - should not duplicate
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	users, err := s.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if user.Role == "admin" {
			t.Fatal("admin should not be seeded into users table")
		}
	}
}

func TestMigrateWithExistingData(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	// Create some data
	u := createUser(t, s, "migdata", "admin")
	ch := &Channel{Name: "mig-ch", Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: ""}
	s.CreateChannel(ch)

	// Add member
	s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: u.ID})

	// Create message
	s.CreateMessageFull(ch.ID, u.ID, "test", "text", nil, nil)

	// Run migration again - should handle backfills
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateWithDMChannel(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	u1 := createUser(t, s, "dmm1", "member")
	u2 := createUser(t, s, "dmm2", "member")

	dmCh, _ := s.CreateDmChannel(u1.ID, u2.ID)
	_ = dmCh

	// Re-migrate - should handle DM cleanup
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
}

// TestMigrateDefaultPermissions / TestMigrateCreatorPermissions were removed in
// the migration baseline squash: they asserted that re-running Migrate()
// backfilled permissions for store-level-created rows. Those data-backfills are
// deleted (they were a one-time repair, now dead because every user-facing
// creation path grants on creation — GrantDefaultPermissions in
// api/auth.go|admin.go|agents.go, GrantCreatorPermissions in api/channels.go).
// The replacement no-op guards live in backfill_noop_guard_test.go, which drives
// the live creation path and asserts nothing is left to repair.
