package store

import (
	"testing"

	"github.com/google/uuid"
)

// backfill_noop_guard_test.go — AC-3 / AC-4 guard tests for the six Go
// data-backfills removed in the migration baseline squash. Each test drives the
// ACTUAL creation/write path the backfill used to repair, WITHOUT any backfill
// running (the new Migrate() no longer has them), and asserts the resulting
// rows already carry the correct state — i.e. there is nothing left to repair.
//
// No-op arguments (one line each), proven by the tests below:
//   - defaultPermissions: every user/agent is granted its defaults at creation
//     via GrantDefaultPermissions; the backfill was a one-time repair, now dead.
//   - creatorChannelPermissions: channel creation grants the creator the
//     channel:<id> perms via GrantCreatorPermissions.
//   - agentOwnerID: agents are created with owner_id set (backfill was already a
//     no-op return nil); no owner-less agents exist to repair.
//   - positions: channels/DMs are created with a real fractional rank
//     (GenerateInitialRank), never the sentinel '0|aaaaaa' / '' the backfill
//     rebalanced.
//   - duplicateDMs: CreateDmChannel builds the canonical sorted name and
//     get-or-creates, so a repeat open of the same pair returns the same row —
//     no duplicate to clean up.
//   - dmExtraMembers: CreateDmChannel inserts exactly the two sorted members;
//     no path adds a third, so there are no extra members to remove.

// AC-3: default permissions are granted at creation, nothing left to repair.
func TestNoOpGuard_DefaultPermissions_GrantedAtCreation(t *testing.T) {
	t.Parallel()
	s := MigratedStoreFromTemplate(t)

	member := &User{ID: uuid.NewString(), DisplayName: "m", Role: "member"}
	if err := s.CreateUser(member); err != nil {
		t.Fatalf("create member: %v", err)
	}
	// Drive the live grant path (the handler layer calls this on every create).
	if err := s.GrantDefaultPermissions(member.ID, "member"); err != nil {
		t.Fatalf("grant member defaults: %v", err)
	}

	agent := &User{ID: uuid.NewString(), DisplayName: "a", Role: "agent"}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := s.GrantDefaultPermissions(agent.ID, "agent"); err != nil {
		t.Fatalf("grant agent defaults: %v", err)
	}

	assertPerms := func(uid string, want map[string]bool) {
		perms, err := s.ListUserPermissions(uid)
		if err != nil {
			t.Fatalf("list perms: %v", err)
		}
		got := map[string]bool{}
		for _, p := range perms {
			if p.Scope == "*" {
				got[p.Permission] = true
			}
		}
		for w := range want {
			if !got[w] {
				t.Fatalf("user %s missing default perm %q (got %v)", uid, w, got)
			}
		}
		if len(got) != len(want) {
			t.Fatalf("user %s perm set %v != expected %v", uid, got, want)
		}
	}
	// member default: (*, *).
	assertPerms(member.ID, map[string]bool{"*": true})
	// agent default: (message.send, *) + (message.read, *).
	assertPerms(agent.ID, map[string]bool{"message.send": true, "message.read": true})
}

// AC-3: channel creation grants the creator channel:<id> perms.
func TestNoOpGuard_CreatorChannelPermissions_GrantedAtCreation(t *testing.T) {
	t.Parallel()
	s := MigratedStoreFromTemplate(t)

	creator := &User{ID: uuid.NewString(), DisplayName: "c", Role: "member"}
	if err := s.CreateUser(creator); err != nil {
		t.Fatalf("create user: %v", err)
	}
	ch := &Channel{Name: "guard-ch", Visibility: "public", CreatedBy: creator.ID, Type: "channel", Position: GenerateInitialRank()}
	if err := s.CreateChannel(ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	// Live grant path (handler calls this right after CreateChannel).
	if err := s.GrantCreatorPermissions(creator.ID, "member", ch.ID, nil); err != nil {
		t.Fatalf("grant creator perms: %v", err)
	}

	perms, err := s.ListUserPermissions(creator.ID)
	if err != nil {
		t.Fatalf("list perms: %v", err)
	}
	scope := "channel:" + ch.ID
	want := map[string]bool{"channel.delete": false, "channel.manage_members": false, "channel.manage_visibility": false}
	for _, p := range perms {
		if p.Scope == scope {
			if _, ok := want[p.Permission]; ok {
				want[p.Permission] = true
			}
		}
	}
	for perm, ok := range want {
		if !ok {
			t.Fatalf("creator missing channel perm %q at scope %q", perm, scope)
		}
	}
}

// AC-3: agents are created with owner_id set — no owner-less agents to repair.
func TestNoOpGuard_AgentOwner_SetAtCreation(t *testing.T) {
	t.Parallel()
	s := MigratedStoreFromTemplate(t)

	owner := &User{ID: uuid.NewString(), DisplayName: "owner", Role: "member"}
	if err := s.CreateUser(owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	agent := &User{ID: uuid.NewString(), DisplayName: "agent", Role: "agent", OwnerID: &owner.ID}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	got, err := s.GetUserByID(agent.ID)
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if got.OwnerID == nil || *got.OwnerID != owner.ID {
		t.Fatalf("agent owner_id not set at creation: got %v", got.OwnerID)
	}
	// And no agent in the DB is left owner-less (the backfill's repair target).
	var orphan int64
	if err := s.db.Raw(`SELECT COUNT(*) FROM users WHERE role='agent' AND owner_id IS NULL AND deleted_at IS NULL`).Row().Scan(&orphan); err != nil {
		t.Fatalf("count orphan agents: %v", err)
	}
	if orphan != 0 {
		t.Fatalf("expected 0 owner-less agents, got %d", orphan)
	}
}

// AC-4: channels are created with a real fractional position, never the
// sentinel the positions backfill rebalanced.
func TestNoOpGuard_Positions_RealRankAtCreation(t *testing.T) {
	t.Parallel()
	s := MigratedStoreFromTemplate(t)

	u := &User{ID: uuid.NewString(), DisplayName: "u", Role: "member"}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	for i := 0; i < 3; i++ {
		ch := &Channel{Name: "pos-ch-" + uuid.NewString(), Visibility: "public", CreatedBy: u.ID, Type: "channel", Position: GenerateInitialRank()}
		if err := s.CreateChannel(ch); err != nil {
			t.Fatalf("create channel: %v", err)
		}
	}
	var sentinel int64
	if err := s.db.Raw(`SELECT COUNT(*) FROM channels WHERE deleted_at IS NULL AND (position = '0|aaaaaa' OR position = '')`).Row().Scan(&sentinel); err != nil {
		t.Fatalf("count sentinel positions: %v", err)
	}
	if sentinel != 0 {
		t.Fatalf("expected 0 channels at sentinel position, got %d (positions backfill would have rebalanced these)", sentinel)
	}
}

// AC-4: CreateDmChannel is get-or-create on a canonical sorted name — no
// duplicate DM is ever created.
func TestNoOpGuard_DuplicateDMs_NoneCreated(t *testing.T) {
	t.Parallel()
	s := MigratedStoreFromTemplate(t)

	u1 := &User{ID: uuid.NewString(), DisplayName: "u1", Role: "member"}
	u2 := &User{ID: uuid.NewString(), DisplayName: "u2", Role: "member"}
	for _, u := range []*User{u1, u2} {
		if err := s.CreateUser(u); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	a, err := s.CreateDmChannel(u1.ID, u2.ID)
	if err != nil {
		t.Fatalf("create dm a: %v", err)
	}
	// Open the same pair in the opposite order — must return the same channel.
	b, err := s.CreateDmChannel(u2.ID, u1.ID)
	if err != nil {
		t.Fatalf("create dm b: %v", err)
	}
	if a.ID != b.ID {
		t.Fatalf("expected same DM channel for the same pair, got %s vs %s", a.ID, b.ID)
	}
	var dmCount int64
	if err := s.db.Raw(`SELECT COUNT(*) FROM channels WHERE type='dm' AND deleted_at IS NULL AND name = ?`, a.Name).Row().Scan(&dmCount); err != nil {
		t.Fatalf("count dms: %v", err)
	}
	if dmCount != 1 {
		t.Fatalf("expected exactly 1 DM for the pair, got %d", dmCount)
	}
}

// AC-4: a created DM has exactly its two participants — no extra members.
func TestNoOpGuard_DMExtraMembers_ExactlyTwo(t *testing.T) {
	t.Parallel()
	s := MigratedStoreFromTemplate(t)

	u1 := &User{ID: uuid.NewString(), DisplayName: "u1", Role: "member"}
	u2 := &User{ID: uuid.NewString(), DisplayName: "u2", Role: "member"}
	for _, u := range []*User{u1, u2} {
		if err := s.CreateUser(u); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	ch, err := s.CreateDmChannel(u1.ID, u2.ID)
	if err != nil {
		t.Fatalf("create dm: %v", err)
	}
	var memberCount int64
	if err := s.db.Raw(`SELECT COUNT(*) FROM channel_members WHERE channel_id = ?`, ch.ID).Row().Scan(&memberCount); err != nil {
		t.Fatalf("count members: %v", err)
	}
	if memberCount != 2 {
		t.Fatalf("expected exactly 2 DM members, got %d", memberCount)
	}
	// Both members are the two participants, no third.
	var extra int64
	if err := s.db.Raw(`SELECT COUNT(*) FROM channel_members WHERE channel_id = ? AND user_id NOT IN (?, ?)`, ch.ID, u1.ID, u2.ID).Row().Scan(&extra); err != nil {
		t.Fatalf("count extra members: %v", err)
	}
	if extra != 0 {
		t.Fatalf("expected 0 extra DM members, got %d", extra)
	}
}
