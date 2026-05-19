package datalayer

import (
	"context"
	"errors"
	"testing"
	"time"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"
)

func newHelperEnrollmentRepoFixture(t *testing.T, name string) (*DataLayer, *store.User) {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { _ = s.Close() })
	pt, err := presence.NewSessionsTracker(s.DB())
	if err != nil {
		t.Fatalf("presence.NewSessionsTracker: %v", err)
	}
	dl := NewDataLayer(s, pt, nil)
	email := name + "@example.com"
	owner := &store.User{DisplayName: name, Role: "member", Email: &email, PasswordHash: "hash"}
	if err := dl.UserRepo.Create(context.Background(), owner); err != nil {
		t.Fatalf("UserRepo.Create: %v", err)
	}
	if _, err := s.CreateOrgForUser(owner, name+" Org"); err != nil {
		t.Fatalf("CreateOrgForUser: %v", err)
	}
	reloaded, err := s.GetUserByID(owner.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	return dl, reloaded
}

func TestHelperEnrollmentRepositoryLifecycle(t *testing.T) {
	t.Parallel()
	dl, owner := newHelperEnrollmentRepoFixture(t, "helper-dl-lifecycle")
	repo := dl.HelperEnrollmentRepo
	if repo == nil {
		t.Fatal("HelperEnrollmentRepo nil")
	}
	ctx := context.Background()
	now := time.UnixMilli(1778840000000)

	pending, secret, err := repo.Create(ctx, owner.ID, "Mac Studio", []string{"openclaw_config", "status_collect"}, now)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if pending.ID == "" || secret == "" || pending.Status != "pending" || pending.EnrollmentSecretExpiresAt == nil {
		t.Fatalf("bad pending enrollment: %+v secret=%q", pending, secret)
	}
	if got := pending.AllowedCategories; len(got) != 2 || got[0] != "openclaw_config" || got[1] != "status_collect" {
		t.Fatalf("allowed categories = %v", got)
	}

	listed, err := repo.ListForUser(ctx, owner.ID, owner.OrgID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != pending.ID {
		t.Fatalf("ListForUser = %+v, want enrollment %s", listed, pending.ID)
	}

	got, err := repo.GetForUser(ctx, pending.ID, owner.ID, owner.OrgID)
	if err != nil {
		t.Fatalf("GetForUser: %v", err)
	}
	if got.ID != pending.ID || got.HostLabel != "Mac Studio" {
		t.Fatalf("GetForUser = %+v", got)
	}

	claimed, credential, err := repo.Claim(ctx, pending.ID, secret, "device-1", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if credential == "" || claimed.Status != "connected" || claimed.HelperDeviceID == nil || *claimed.HelperDeviceID != "device-1" {
		t.Fatalf("bad claimed enrollment: %+v credential=%q", claimed, credential)
	}

	heartbeatAt := now.Add(2 * time.Minute)
	seen, err := repo.UpdateLastSeen(ctx, pending.ID, credential, "device-1", heartbeatAt)
	if err != nil {
		t.Fatalf("UpdateLastSeen: %v", err)
	}
	if seen.LastSeenAt == nil || *seen.LastSeenAt != heartbeatAt.UnixMilli() {
		t.Fatalf("LastSeenAt = %v, want %d", seen.LastSeenAt, heartbeatAt.UnixMilli())
	}

	rotated, rotatedCredential, err := repo.RotateCredential(ctx, pending.ID, credential, "device-1", now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("RotateCredential: %v", err)
	}
	if rotatedCredential == "" || rotatedCredential == credential {
		t.Fatalf("rotated credential=%q old=%q", rotatedCredential, credential)
	}
	if rotated.CredentialRotatedAt == nil || rotated.CredentialGeneration != 2 {
		t.Fatalf("bad rotation metadata: %+v", rotated)
	}
	if _, err := repo.UpdateLastSeen(ctx, pending.ID, credential, "device-1", now.Add(4*time.Minute)); !errors.Is(err, ErrHelperEnrollmentUnauthorized) {
		t.Fatalf("old credential UpdateLastSeen error=%v, want ErrHelperEnrollmentUnauthorized", err)
	}
	rotatedSeenAt := now.Add(5 * time.Minute)
	rotatedSeen, err := repo.UpdateLastSeen(ctx, pending.ID, rotatedCredential, "device-1", rotatedSeenAt)
	if err != nil {
		t.Fatalf("rotated credential UpdateLastSeen: %v", err)
	}
	if rotatedSeen.LastSeenAt == nil || *rotatedSeen.LastSeenAt != rotatedSeenAt.UnixMilli() {
		t.Fatalf("rotated LastSeenAt = %v, want %d", rotatedSeen.LastSeenAt, rotatedSeenAt.UnixMilli())
	}

	uninstalled, err := repo.MarkUninstalled(ctx, pending.ID, rotatedCredential, "device-1", now.Add(6*time.Minute))
	if err != nil {
		t.Fatalf("MarkUninstalled: %v", err)
	}
	if uninstalled.Status != "uninstalled" || uninstalled.UninstalledAt == nil {
		t.Fatalf("bad uninstalled enrollment: %+v", uninstalled)
	}

	revokable, _, err := repo.Create(ctx, owner.ID, "Linux", []string{"helper_lifecycle"}, now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("Create revokable: %v", err)
	}
	revoked, err := repo.RevokeForUser(ctx, revokable.ID, owner.ID, owner.OrgID, now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("RevokeForUser: %v", err)
	}
	if revoked.Status != "revoked" || revoked.RevokedAt == nil {
		t.Fatalf("bad revoked enrollment: %+v", revoked)
	}
}

// TestHelperEnrollmentRepositoryRecordUpdatesAvailable covers the #999
// datalayer RecordUpdatesAvailable wrapper + the store-level
// RecordHelperEnrollmentUpdatesAvailable query end-to-end against a
// freshly migrated SQLite store. Lifecycle: create → claim → record
// snapshot (security + feature mixed) → reload via GetForUser and
// assert the JSON round-trips through the typed projection.
func TestHelperEnrollmentRepositoryRecordUpdatesAvailable(t *testing.T) {
	t.Parallel()
	dl, owner := newHelperEnrollmentRepoFixture(t, "helper-dl-updates")
	repo := dl.HelperEnrollmentRepo
	ctx := context.Background()
	now := time.UnixMilli(1778840000000)

	pending, secret, err := repo.Create(ctx, owner.ID, "Linux Studio", []string{"openclaw_config"}, now)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, credential, err := repo.Claim(ctx, pending.ID, secret, "device-upd", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}

	checkAt := now.Add(5 * time.Minute)
	updates := []HelperEnrollmentUpdateAvailable{
		{PluginID: "openclaw", CurrentVersion: "1.0.0", ManifestVersion: "1.1.0", Class: "security"},
		{PluginID: "plug-b", CurrentVersion: "", ManifestVersion: "0.1.0", Class: "feature"},
	}
	row, err := repo.RecordUpdatesAvailable(ctx, pending.ID, credential, "device-upd", updates, checkAt)
	if err != nil {
		t.Fatalf("RecordUpdatesAvailable: %v", err)
	}
	if row.LastUpdateCheckAt == nil || *row.LastUpdateCheckAt != checkAt.UnixMilli() {
		t.Fatalf("LastUpdateCheckAt = %v, want %d", row.LastUpdateCheckAt, checkAt.UnixMilli())
	}
	if len(row.UpdatesAvailable) != 2 {
		t.Fatalf("UpdatesAvailable len=%d, want 2: %+v", len(row.UpdatesAvailable), row.UpdatesAvailable)
	}
	if row.UpdatesAvailable[0].PluginID != "openclaw" || row.UpdatesAvailable[0].Class != "security" {
		t.Fatalf("first update entry shape wrong: %+v", row.UpdatesAvailable[0])
	}
	if row.UpdatesAvailable[1].CurrentVersion != "" || row.UpdatesAvailable[1].Class != "feature" {
		t.Fatalf("second update entry shape wrong: %+v", row.UpdatesAvailable[1])
	}

	// Reload to prove the snapshot persists across a fresh read (the
	// JSON column round-trips via helperEnrollmentFromStore).
	reloaded, err := repo.GetForUser(ctx, pending.ID, owner.ID, owner.OrgID)
	if err != nil {
		t.Fatalf("GetForUser after record: %v", err)
	}
	if len(reloaded.UpdatesAvailable) != 2 {
		t.Fatalf("reloaded UpdatesAvailable len=%d, want 2", len(reloaded.UpdatesAvailable))
	}
	if reloaded.LastUpdateCheckAt == nil || *reloaded.LastUpdateCheckAt != checkAt.UnixMilli() {
		t.Fatalf("reloaded LastUpdateCheckAt = %v, want %d", reloaded.LastUpdateCheckAt, checkAt.UnixMilli())
	}

	// Empty snapshot (drift cleared) is a valid latest-wins write —
	// proves the nil-vs-empty normalization in the wrapper.
	clearedAt := now.Add(10 * time.Minute)
	cleared, err := repo.RecordUpdatesAvailable(ctx, pending.ID, credential, "device-upd", nil, clearedAt)
	if err != nil {
		t.Fatalf("RecordUpdatesAvailable cleared: %v", err)
	}
	if len(cleared.UpdatesAvailable) != 0 {
		t.Fatalf("cleared UpdatesAvailable len=%d, want 0 (snapshot cleared)", len(cleared.UpdatesAvailable))
	}
	if cleared.LastUpdateCheckAt == nil || *cleared.LastUpdateCheckAt != clearedAt.UnixMilli() {
		t.Fatalf("cleared LastUpdateCheckAt = %v, want %d", cleared.LastUpdateCheckAt, clearedAt.UnixMilli())
	}

	// Wrong device id closes — credential rail guard still applies.
	if _, err := repo.RecordUpdatesAvailable(ctx, pending.ID, credential, "device-other", nil, now.Add(11*time.Minute)); !errors.Is(err, ErrHelperEnrollmentDeviceMismatch) {
		t.Fatalf("device mismatch error=%v, want ErrHelperEnrollmentDeviceMismatch", err)
	}
	// Bad credential closes too.
	if _, err := repo.RecordUpdatesAvailable(ctx, pending.ID, "bad-credential", "device-upd", nil, now.Add(12*time.Minute)); !errors.Is(err, ErrHelperEnrollmentUnauthorized) {
		t.Fatalf("bad credential error=%v, want ErrHelperEnrollmentUnauthorized", err)
	}
}

func TestHelperEnrollmentRepositoryErrors(t *testing.T) {
	t.Parallel()
	dl, owner := newHelperEnrollmentRepoFixture(t, "helper-dl-errors")
	ctx := context.Background()
	now := time.UnixMilli(1778840000000)

	if _, _, err := dl.HelperEnrollmentRepo.Create(ctx, owner.ID, "Mac Studio", []string{"shell"}, now); !errors.Is(err, ErrHelperEnrollmentInvalidCategory) {
		t.Fatalf("Create invalid category error=%v, want ErrHelperEnrollmentInvalidCategory", err)
	}
	if _, err := dl.HelperEnrollmentRepo.GetForUser(ctx, "missing", owner.ID, owner.OrgID); !errors.Is(err, ErrHelperEnrollmentNotFound) {
		t.Fatalf("GetForUser missing error=%v, want ErrHelperEnrollmentNotFound", err)
	}
	if _, _, err := dl.HelperEnrollmentRepo.Claim(ctx, "missing", "secret", "device-1", now); !errors.Is(err, ErrHelperEnrollmentNotFound) {
		t.Fatalf("Claim missing error=%v, want ErrHelperEnrollmentNotFound", err)
	}
	if _, err := dl.HelperEnrollmentRepo.UpdateLastSeen(ctx, "missing", "credential", "device-1", now); !errors.Is(err, ErrHelperEnrollmentNotFound) {
		t.Fatalf("UpdateLastSeen missing error=%v, want ErrHelperEnrollmentNotFound", err)
	}
	if _, _, err := dl.HelperEnrollmentRepo.RotateCredential(ctx, "missing", "credential", "device-1", now); !errors.Is(err, ErrHelperEnrollmentNotFound) {
		t.Fatalf("RotateCredential missing error=%v, want ErrHelperEnrollmentNotFound", err)
	}
	if _, err := dl.HelperEnrollmentRepo.MarkUninstalled(ctx, "missing", "credential", "device-1", now); !errors.Is(err, ErrHelperEnrollmentNotFound) {
		t.Fatalf("MarkUninstalled missing error=%v, want ErrHelperEnrollmentNotFound", err)
	}
}

func TestHelperEnrollmentErrorMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"invalid category", store.ErrHelperEnrollmentInvalidCategory, ErrHelperEnrollmentInvalidCategory},
		{"invalid input", store.ErrHelperEnrollmentInvalidInput, ErrHelperEnrollmentInvalidInput},
		{"invalid owner", store.ErrHelperEnrollmentInvalidOwner, ErrHelperEnrollmentInvalidOwner},
		{"not found", store.ErrHelperEnrollmentNotFound, ErrHelperEnrollmentNotFound},
		{"forbidden", store.ErrHelperEnrollmentForbidden, ErrHelperEnrollmentForbidden},
		{"unauthorized", store.ErrHelperEnrollmentUnauthorized, ErrHelperEnrollmentUnauthorized},
		{"already claimed", store.ErrHelperEnrollmentAlreadyClaimed, ErrHelperEnrollmentAlreadyClaimed},
		{"device mismatch", store.ErrHelperEnrollmentDeviceMismatch, ErrHelperEnrollmentDeviceMismatch},
		{"inactive", store.ErrHelperEnrollmentInactive, ErrHelperEnrollmentInactive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := mapHelperEnrollmentErr(tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("mapHelperEnrollmentErr(%v)=%v, want %v", tc.in, err, tc.want)
			}
		})
	}
	if err := mapHelperEnrollmentErr(nil); err != nil {
		t.Fatalf("mapHelperEnrollmentErr(nil)=%v", err)
	}
	unknown := errors.New("other")
	if err := mapHelperEnrollmentErr(unknown); !errors.Is(err, unknown) {
		t.Fatalf("mapHelperEnrollmentErr(other)=%v, want original", err)
	}
	if helperEnrollmentFromStore(nil) != nil {
		t.Fatal("helperEnrollmentFromStore(nil) should return nil")
	}
}
