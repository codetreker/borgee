package store

import (
	"strings"
	"testing"
)

func TestAddUserToPublicChannelsIsOrgScoped(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)

	owner := createUser(t, s, "public-owner", "member")
	if _, err := s.CreateOrgForUser(owner, "owner org"); err != nil {
		t.Fatal(err)
	}
	other := createUser(t, s, "public-other", "member")
	if _, err := s.CreateOrgForUser(other, "other org"); err != nil {
		t.Fatal(err)
	}

	ch := &Channel{
		Name:       "org-public",
		Visibility: "public",
		CreatedBy:  owner.ID,
		Type:       "channel",
		Position:   GenerateInitialRank(),
		OrgID:      owner.OrgID,
	}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatal(err)
	}
	if err := s.AddChannelMember(&ChannelMember{ChannelID: ch.ID, UserID: owner.ID}); err != nil {
		t.Fatal(err)
	}

	if err := s.AddUserToPublicChannels(other.ID); err != nil {
		t.Fatal(err)
	}

	members, err := s.ListChannelMembers(ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].UserID != owner.ID {
		t.Fatalf("cross-org public channel auto-join leaked members: %+v", members)
	}
}

func TestSQLiteDSNWithPragmas(t *testing.T) {
	t.Parallel()
	if got := sqliteDSNWithPragmas(":memory:"); got != ":memory:" {
		t.Fatalf(":memory: DSN changed: %q", got)
	}

	got := sqliteDSNWithPragmas("data/collab.db")
	for _, want := range []string{"_busy_timeout=5000", "_foreign_keys=on", "_journal_mode=WAL"} {
		if !strings.Contains(got, want) {
			t.Fatalf("DSN %q missing %q", got, want)
		}
	}
	if !strings.Contains(sqliteDSNWithPragmas("file:test.db?cache=shared"), "&") {
		t.Fatal("existing query DSN should append pragmas with &")
	}
}
