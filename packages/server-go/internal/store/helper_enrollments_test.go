package store

import (
	"errors"
	"testing"
	"time"
)

func helperOwner(t *testing.T, s *Store, name string) *User {
	t.Helper()
	u := createUser(t, s, name, "member")
	if _, err := s.CreateOrgForUser(u, name+" Org"); err != nil {
		t.Fatalf("create org: %v", err)
	}
	got, err := s.GetUserByID(u.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return got
}

func TestHelperEnrollmentCreateStampsOwnerOrgAndValidatesCategories(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-owner")
	now := time.UnixMilli(1778840000000)

	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Mac Studio", []string{"openclaw_config", "status_collect"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	if secret == "" {
		t.Fatal("one-time secret should be returned once")
	}
	if enrollment.OwnerUserID != owner.ID || enrollment.OrgID != owner.OrgID {
		t.Fatalf("owner/org not stamped: got owner=%q org=%q want owner=%q org=%q", enrollment.OwnerUserID, enrollment.OrgID, owner.ID, owner.OrgID)
	}
	if enrollment.Status != "pending" {
		t.Fatalf("status=%q, want pending", enrollment.Status)
	}
	if enrollment.EnrollmentSecretDigest == nil || *enrollment.EnrollmentSecretDigest == "" || *enrollment.EnrollmentSecretDigest == secret {
		t.Fatalf("one-time secret digest not stored safely: digest=%v secret=%q", enrollment.EnrollmentSecretDigest, secret)
	}
	if enrollment.PersistentCredentialDigest != nil {
		t.Fatalf("persistent credential digest should be nil before claim")
	}

	if _, _, err := s.CreateHelperEnrollment(owner.ID, "Mac Studio", []string{"shell"}, now); !errors.Is(err, ErrHelperEnrollmentInvalidCategory) {
		t.Fatalf("invalid category error=%v, want ErrHelperEnrollmentInvalidCategory", err)
	}
	if _, _, err := s.CreateHelperEnrollment(owner.ID, "Mac Studio", []string{}, now); !errors.Is(err, ErrHelperEnrollmentInvalidCategory) {
		t.Fatalf("empty categories error=%v, want ErrHelperEnrollmentInvalidCategory", err)
	}
}

func TestHelperEnrollmentListGetAndAllowedCategoryList(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-list")
	other := helperOwner(t, s, "helper-list-other")
	now := time.UnixMilli(1778840000000)

	first, _, err := s.CreateHelperEnrollment(owner.ID, "First", []string{"openclaw_config"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment first: %v", err)
	}
	second, _, err := s.CreateHelperEnrollment(owner.ID, "Second", []string{"status_collect"}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("CreateHelperEnrollment second: %v", err)
	}
	if _, _, err := s.CreateHelperEnrollment(other.ID, "Other", []string{"helper_lifecycle"}, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("CreateHelperEnrollment other: %v", err)
	}

	rows, err := s.ListHelperEnrollmentsForUser(owner.ID, owner.OrgID)
	if err != nil {
		t.Fatalf("ListHelperEnrollmentsForUser: %v", err)
	}
	if len(rows) != 2 || rows[0].ID != second.ID || rows[1].ID != first.ID {
		t.Fatalf("list order/scope = %+v, want second then first", rows)
	}
	if cats := rows[0].AllowedCategoryList(); len(cats) != 1 || cats[0] != "status_collect" {
		t.Fatalf("AllowedCategoryList = %v", cats)
	}
	if cats := (&HelperEnrollment{}).AllowedCategoryList(); len(cats) != 0 {
		t.Fatalf("empty AllowedCategoryList = %v, want empty", cats)
	}
	if cats := (*HelperEnrollment)(nil).AllowedCategoryList(); len(cats) != 0 {
		t.Fatalf("nil AllowedCategoryList = %v, want empty", cats)
	}
	if cats := (&HelperEnrollment{AllowedCategories: "not-json"}).AllowedCategoryList(); len(cats) != 0 {
		t.Fatalf("invalid JSON AllowedCategoryList = %v, want empty", cats)
	}

	if _, err := s.GetHelperEnrollment("missing"); !errors.Is(err, ErrHelperEnrollmentNotFound) {
		t.Fatalf("GetHelperEnrollment missing error=%v, want ErrHelperEnrollmentNotFound", err)
	}
	if _, _, err := s.ClaimHelperEnrollment("missing", "secret", "device-1", now); !errors.Is(err, ErrHelperEnrollmentNotFound) {
		t.Fatalf("ClaimHelperEnrollment missing error=%v, want ErrHelperEnrollmentNotFound", err)
	}
}

func TestHelperEnrollmentClaimIsSingleUseAndReturnsCredentialOnce(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-claim")
	createdAt := time.UnixMilli(1778840000000)
	claimedAt := createdAt.Add(time.Minute)

	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Linux Workstation", []string{"helper_lifecycle"}, createdAt)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, credential, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-1", claimedAt)
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	if credential == "" {
		t.Fatal("persistent Helper credential should be returned once on claim")
	}
	if claimed.HelperDeviceID == nil || *claimed.HelperDeviceID != "device-1" {
		t.Fatalf("helper_device_id=%v, want device-1", claimed.HelperDeviceID)
	}
	if claimed.PersistentCredentialDigest == nil || *claimed.PersistentCredentialDigest == "" || *claimed.PersistentCredentialDigest == credential {
		t.Fatalf("persistent credential digest not stored safely: digest=%v credential=%q", claimed.PersistentCredentialDigest, credential)
	}
	if claimed.EnrollmentSecretDigest != nil {
		t.Fatalf("one-time enrollment secret digest should be cleared after claim")
	}

	lastSeen := *claimed.LastSeenAt
	if _, _, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-2", claimedAt.Add(time.Minute)); !errors.Is(err, ErrHelperEnrollmentAlreadyClaimed) {
		t.Fatalf("second claim error=%v, want ErrHelperEnrollmentAlreadyClaimed", err)
	}
	after, err := s.GetHelperEnrollment(enrollment.ID)
	if err != nil {
		t.Fatalf("GetHelperEnrollment: %v", err)
	}
	if after.HelperDeviceID == nil || *after.HelperDeviceID != "device-1" {
		t.Fatalf("second claim mutated helper_device_id: %v", after.HelperDeviceID)
	}
	if after.LastSeenAt == nil || *after.LastSeenAt != lastSeen {
		t.Fatalf("second claim mutated last_seen_at: got %v want %d", after.LastSeenAt, lastSeen)
	}
}

func TestHelperEnrollmentHeartbeatStaleDevicePredicatesAndFreshnessRecovery(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-heartbeat")
	createdAt := time.UnixMilli(1778840000000)
	staleSeen := createdAt.Add(-30 * time.Minute).UnixMilli()

	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "MacBook", []string{"status_collect"}, createdAt)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, credential, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-a", createdAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	if err := s.DB().Model(&HelperEnrollment{}).Where("id = ?", claimed.ID).Update("last_seen_at", staleSeen).Error; err != nil {
		t.Fatalf("seed stale last_seen_at: %v", err)
	}

	if _, err := s.UpdateHelperEnrollmentLastSeen(claimed.ID, "wrong-credential", "device-a", createdAt.Add(2*time.Minute)); !errors.Is(err, ErrHelperEnrollmentUnauthorized) {
		t.Fatalf("wrong credential error=%v, want ErrHelperEnrollmentUnauthorized", err)
	}
	afterWrongCredential, _ := s.GetHelperEnrollment(claimed.ID)
	if afterWrongCredential.LastSeenAt == nil || *afterWrongCredential.LastSeenAt != staleSeen {
		t.Fatalf("wrong credential mutated last_seen_at: got %v want %d", afterWrongCredential.LastSeenAt, staleSeen)
	}

	if _, err := s.UpdateHelperEnrollmentLastSeen(claimed.ID, credential, "device-b", createdAt.Add(3*time.Minute)); !errors.Is(err, ErrHelperEnrollmentDeviceMismatch) {
		t.Fatalf("wrong device error=%v, want ErrHelperEnrollmentDeviceMismatch", err)
	}
	afterWrongDevice, _ := s.GetHelperEnrollment(claimed.ID)
	if afterWrongDevice.LastSeenAt == nil || *afterWrongDevice.LastSeenAt != staleSeen {
		t.Fatalf("wrong device mutated last_seen_at: got %v want %d", afterWrongDevice.LastSeenAt, staleSeen)
	}

	recoveredAt := createdAt.Add(4 * time.Minute)
	recovered, err := s.UpdateHelperEnrollmentLastSeen(claimed.ID, credential, "device-a", recoveredAt)
	if err != nil {
		t.Fatalf("valid stale-freshness heartbeat should recover: %v", err)
	}
	if recovered.LastSeenAt == nil || *recovered.LastSeenAt != recoveredAt.UnixMilli() {
		t.Fatalf("valid heartbeat last_seen_at=%v, want %d", recovered.LastSeenAt, recoveredAt.UnixMilli())
	}
}

func TestHelperEnrollmentTerminalStatesBlockLastSeen(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-terminal")
	now := time.UnixMilli(1778840000000)

	revoked, secret, err := s.CreateHelperEnrollment(owner.ID, "Revoked Host", []string{"status_collect"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment revoked: %v", err)
	}
	revokedClaim, revokedCredential, err := s.ClaimHelperEnrollment(revoked.ID, secret, "device-r", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Claim revoked fixture: %v", err)
	}
	lastSeen := *revokedClaim.LastSeenAt
	if _, err := s.RevokeHelperEnrollmentForUser(revoked.ID, owner.ID, owner.OrgID, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("RevokeHelperEnrollmentForUser: %v", err)
	}
	if _, err := s.UpdateHelperEnrollmentLastSeen(revoked.ID, revokedCredential, "device-r", now.Add(3*time.Minute)); !errors.Is(err, ErrHelperEnrollmentInactive) {
		t.Fatalf("revoked heartbeat error=%v, want ErrHelperEnrollmentInactive", err)
	}
	afterRevoke, _ := s.GetHelperEnrollment(revoked.ID)
	if afterRevoke.LastSeenAt == nil || *afterRevoke.LastSeenAt != lastSeen {
		t.Fatalf("revoked heartbeat mutated last_seen_at: got %v want %d", afterRevoke.LastSeenAt, lastSeen)
	}

	uninstalled, uninstallSecret, err := s.CreateHelperEnrollment(owner.ID, "Uninstall Host", []string{"helper_lifecycle"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment uninstall: %v", err)
	}
	_, uninstallCredential, err := s.ClaimHelperEnrollment(uninstalled.ID, uninstallSecret, "device-u", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Claim uninstall fixture: %v", err)
	}
	if _, err := s.MarkHelperEnrollmentUninstalled(uninstalled.ID, uninstallCredential, "device-u", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("MarkHelperEnrollmentUninstalled: %v", err)
	}
	if _, err := s.UpdateHelperEnrollmentLastSeen(uninstalled.ID, uninstallCredential, "device-u", now.Add(3*time.Minute)); !errors.Is(err, ErrHelperEnrollmentInactive) {
		t.Fatalf("uninstalled heartbeat error=%v, want ErrHelperEnrollmentInactive", err)
	}
}
