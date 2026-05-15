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

	uninstalled, err := repo.MarkUninstalled(ctx, pending.ID, credential, "device-1", now.Add(3*time.Minute))
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
