package store

import "testing"

// TestCM_OrgIDQueries covers MessageOrgID / WorkspaceFileOrgID /
// RemoteNodeOrgID — CM-3 cross-org backfill query helpers (uncovered
// 0% → tested smoke; coverage 后续 to cross 85% threshold).
func TestCM_OrgIDQueries(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	// MessageOrgID — bogus id returns ErrRecordNotFound (covers query path).
	if _, err := s.MessageOrgID("bogus-message-id"); err == nil {
		t.Error("MessageOrgID(bogus) should return error")
	}

	// WorkspaceFileOrgID — bogus id returns ErrRecordNotFound.
	if _, err := s.WorkspaceFileOrgID("bogus-file-id"); err == nil {
		t.Error("WorkspaceFileOrgID(bogus) should return error")
	}

	// RemoteNodeOrgID — bogus id returns ErrRecordNotFound.
	if _, err := s.RemoteNodeOrgID("bogus-node-id"); err == nil {
		t.Error("RemoteNodeOrgID(bogus) should return error")
	}

	// ChannelOrgID — bogus id returns error (already partially covered).
	if _, err := s.ChannelOrgID("bogus-channel-id"); err == nil {
		t.Error("ChannelOrgID(bogus) should return error")
	}
}

// TestCreateOrgForUserGuards covers CreateOrgForUser early-return guards
// (nil user / empty id / already has org) — uncovered 55.6% → fuller cover.
func TestCreateOrgForUserGuards(t *testing.T) {
	t.Parallel()
	s := testStore(t)
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	// nil user → error.
	if _, err := s.CreateOrgForUser(nil, "TestOrg"); err == nil {
		t.Error("CreateOrgForUser(nil) should return error")
	}

	// User with empty ID → error.
	if _, err := s.CreateOrgForUser(&User{}, "TestOrg"); err == nil {
		t.Error("CreateOrgForUser(empty-id) should return error")
	}

	// User with existing org_id → idempotent no-op (returns nil, nil if not found).
	if _, err := s.CreateOrgForUser(&User{ID: "u1", OrgID: "bogus-org"}, "TestOrg"); err != nil {
		t.Errorf("CreateOrgForUser(already-has-org) idempotent path: %v", err)
	}
}
