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

func TestHelperEnrollmentCredentialRotationMakesOldCredentialStaleAndNewCredentialAuthoritative(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-rotate")
	now := time.UnixMilli(1778840000000)

	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Rotate Host", []string{"helper_lifecycle", "status_collect"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, oldCredential, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-r", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	createdAt := *claimed.CredentialCreatedAt
	oldDigest := *claimed.PersistentCredentialDigest

	rotatedAt := now.Add(2 * time.Minute)
	rotated, newCredential, err := s.RotateHelperEnrollmentCredential(enrollment.ID, oldCredential, "device-r", rotatedAt)
	if err != nil {
		t.Fatalf("RotateHelperEnrollmentCredential: %v", err)
	}
	if newCredential == "" || newCredential == oldCredential {
		t.Fatalf("new credential=%q old=%q", newCredential, oldCredential)
	}
	if rotated.CredentialCreatedAt == nil || *rotated.CredentialCreatedAt != createdAt {
		t.Fatalf("credential_created_at=%v, want preserved %d", rotated.CredentialCreatedAt, createdAt)
	}
	if rotated.CredentialRotatedAt == nil || *rotated.CredentialRotatedAt != rotatedAt.UnixMilli() {
		t.Fatalf("credential_rotated_at=%v, want %d", rotated.CredentialRotatedAt, rotatedAt.UnixMilli())
	}
	if rotated.CredentialGeneration != 2 {
		t.Fatalf("credential_generation=%d, want 2", rotated.CredentialGeneration)
	}
	if rotated.PersistentCredentialDigest == nil || *rotated.PersistentCredentialDigest == "" || *rotated.PersistentCredentialDigest == oldDigest || *rotated.PersistentCredentialDigest == newCredential {
		t.Fatalf("rotated credential digest not stored safely: oldDigest=%q newDigest=%v newCredential=%q", oldDigest, rotated.PersistentCredentialDigest, newCredential)
	}

	if _, err := s.UpdateHelperEnrollmentLastSeen(enrollment.ID, oldCredential, "device-r", now.Add(3*time.Minute)); !errors.Is(err, ErrHelperEnrollmentUnauthorized) {
		t.Fatalf("old credential heartbeat error=%v, want ErrHelperEnrollmentUnauthorized", err)
	}
	if _, _, err := s.RotateHelperEnrollmentCredential(enrollment.ID, oldCredential, "device-r", now.Add(4*time.Minute)); !errors.Is(err, ErrHelperEnrollmentUnauthorized) {
		t.Fatalf("old credential rotate error=%v, want ErrHelperEnrollmentUnauthorized", err)
	}
	if _, err := s.MarkHelperEnrollmentUninstalled(enrollment.ID, oldCredential, "device-r", now.Add(5*time.Minute)); !errors.Is(err, ErrHelperEnrollmentUnauthorized) {
		t.Fatalf("old credential uninstall error=%v, want ErrHelperEnrollmentUnauthorized", err)
	}

	heartbeatAt := now.Add(6 * time.Minute)
	seen, err := s.UpdateHelperEnrollmentLastSeen(enrollment.ID, newCredential, "device-r", heartbeatAt)
	if err != nil {
		t.Fatalf("new credential heartbeat: %v", err)
	}
	if seen.LastSeenAt == nil || *seen.LastSeenAt != heartbeatAt.UnixMilli() {
		t.Fatalf("new credential last_seen_at=%v, want %d", seen.LastSeenAt, heartbeatAt.UnixMilli())
	}

	uninstalled, err := s.MarkHelperEnrollmentUninstalled(enrollment.ID, newCredential, "device-r", now.Add(7*time.Minute))
	if err != nil {
		t.Fatalf("new credential uninstall: %v", err)
	}
	if uninstalled.Status != "uninstalled" || uninstalled.UninstalledAt == nil {
		t.Fatalf("new credential uninstall row=%+v", uninstalled)
	}
}

func TestHelperEnrollmentCredentialRotationRejectsInvalidAndTerminalAuthorityWithoutMutation(t *testing.T) {
	t.Parallel()
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-rotate-invalid")
	now := time.UnixMilli(1778840000000)

	pending, _, err := s.CreateHelperEnrollment(owner.ID, "Pending", []string{"helper_lifecycle"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment pending: %v", err)
	}
	if _, _, err := s.RotateHelperEnrollmentCredential(pending.ID, "credential", "device-p", now.Add(time.Minute)); !errors.Is(err, ErrHelperEnrollmentInactive) {
		t.Fatalf("pending rotate error=%v, want ErrHelperEnrollmentInactive", err)
	}
	afterPending, _ := s.GetHelperEnrollment(pending.ID)
	if afterPending.PersistentCredentialDigest != nil || afterPending.HelperDeviceID != nil || afterPending.Status != "pending" {
		t.Fatalf("pending rotate mutated row: %+v", afterPending)
	}

	claimed, secret, err := s.CreateHelperEnrollment(owner.ID, "Claimed", []string{"helper_lifecycle"}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("CreateHelperEnrollment claimed: %v", err)
	}
	row, credential, err := s.ClaimHelperEnrollment(claimed.ID, secret, "device-a", now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	before := *row.PersistentCredentialDigest
	lastSeen := *row.LastSeenAt
	if _, _, err := s.RotateHelperEnrollmentCredential(claimed.ID, "wrong-credential", "device-a", now.Add(4*time.Minute)); !errors.Is(err, ErrHelperEnrollmentUnauthorized) {
		t.Fatalf("wrong credential rotate error=%v, want ErrHelperEnrollmentUnauthorized", err)
	}
	afterWrongCredential, _ := s.GetHelperEnrollment(claimed.ID)
	if *afterWrongCredential.PersistentCredentialDigest != before || *afterWrongCredential.LastSeenAt != lastSeen || afterWrongCredential.CredentialRotatedAt != nil || afterWrongCredential.CredentialGeneration != 1 {
		t.Fatalf("wrong credential rotate mutated row: %+v", afterWrongCredential)
	}
	if _, _, err := s.RotateHelperEnrollmentCredential(claimed.ID, credential, "device-b", now.Add(5*time.Minute)); !errors.Is(err, ErrHelperEnrollmentDeviceMismatch) {
		t.Fatalf("wrong device rotate error=%v, want ErrHelperEnrollmentDeviceMismatch", err)
	}
	afterWrongDevice, _ := s.GetHelperEnrollment(claimed.ID)
	if *afterWrongDevice.PersistentCredentialDigest != before || *afterWrongDevice.LastSeenAt != lastSeen || *afterWrongDevice.HelperDeviceID != "device-a" || afterWrongDevice.CredentialRotatedAt != nil || afterWrongDevice.CredentialGeneration != 1 {
		t.Fatalf("wrong device rotate mutated row: %+v", afterWrongDevice)
	}

	revoked, revokeSecret, err := s.CreateHelperEnrollment(owner.ID, "Revoked", []string{"helper_lifecycle"}, now.Add(6*time.Minute))
	if err != nil {
		t.Fatalf("CreateHelperEnrollment revoked: %v", err)
	}
	_, revokedCredential, err := s.ClaimHelperEnrollment(revoked.ID, revokeSecret, "device-r", now.Add(7*time.Minute))
	if err != nil {
		t.Fatalf("Claim revoked: %v", err)
	}
	revokedRow, err := s.RevokeHelperEnrollmentForUser(revoked.ID, owner.ID, owner.OrgID, now.Add(8*time.Minute))
	if err != nil {
		t.Fatalf("RevokeHelperEnrollmentForUser: %v", err)
	}
	revokedAt := *revokedRow.RevokedAt
	if _, _, err := s.RotateHelperEnrollmentCredential(revoked.ID, revokedCredential, "device-r", now.Add(9*time.Minute)); !errors.Is(err, ErrHelperEnrollmentInactive) {
		t.Fatalf("revoked rotate error=%v, want ErrHelperEnrollmentInactive", err)
	}
	afterRevoke, _ := s.GetHelperEnrollment(revoked.ID)
	if afterRevoke.Status != "revoked" || afterRevoke.RevokedAt == nil || *afterRevoke.RevokedAt != revokedAt || afterRevoke.CredentialRotatedAt != nil {
		t.Fatalf("revoked rotate mutated terminal row: %+v", afterRevoke)
	}

	uninstalled, uninstallSecret, err := s.CreateHelperEnrollment(owner.ID, "Uninstalled", []string{"helper_lifecycle"}, now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateHelperEnrollment uninstalled: %v", err)
	}
	_, uninstallCredential, err := s.ClaimHelperEnrollment(uninstalled.ID, uninstallSecret, "device-u", now.Add(11*time.Minute))
	if err != nil {
		t.Fatalf("Claim uninstalled: %v", err)
	}
	uninstalledRow, err := s.MarkHelperEnrollmentUninstalled(uninstalled.ID, uninstallCredential, "device-u", now.Add(12*time.Minute))
	if err != nil {
		t.Fatalf("MarkHelperEnrollmentUninstalled: %v", err)
	}
	uninstalledAt := *uninstalledRow.UninstalledAt
	if _, _, err := s.RotateHelperEnrollmentCredential(uninstalled.ID, uninstallCredential, "device-u", now.Add(13*time.Minute)); !errors.Is(err, ErrHelperEnrollmentInactive) {
		t.Fatalf("uninstalled rotate error=%v, want ErrHelperEnrollmentInactive", err)
	}
	afterUninstall, _ := s.GetHelperEnrollment(uninstalled.ID)
	if afterUninstall.Status != "uninstalled" || afterUninstall.UninstalledAt == nil || *afterUninstall.UninstalledAt != uninstalledAt || afterUninstall.CredentialRotatedAt != nil {
		t.Fatalf("uninstalled rotate mutated terminal row: %+v", afterUninstall)
	}
}

func TestHelperEnrollmentCredentialRotationRejectsStaleCredentialAfterHeartbeatValidation(t *testing.T) {
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-rotate-heartbeat-race")
	now := time.UnixMilli(1778840000000)

	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Heartbeat Race", []string{"status_collect"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, credential, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-race", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	originalSeen := *claimed.LastSeenAt
	tracedAt := now.Add(2 * time.Minute)

	helperEnrollmentCredentialRaceHook = func(s *Store, row *HelperEnrollment) error {
		newDigest := helperSecretDigest("newer-credential")
		return s.DB().Model(&HelperEnrollment{}).
			Where("id = ?", row.ID).
			Updates(map[string]any{"persistent_credential_digest": newDigest, "credential_rotated_at": tracedAt.UnixMilli(), "credential_generation": row.CredentialGeneration + 1}).Error
	}
	t.Cleanup(func() { helperEnrollmentCredentialRaceHook = nil })

	if _, err := s.UpdateHelperEnrollmentLastSeen(enrollment.ID, credential, "device-race", now.Add(3*time.Minute)); !errors.Is(err, ErrHelperEnrollmentUnauthorized) {
		t.Fatalf("heartbeat with credential stale after validation error=%v, want ErrHelperEnrollmentUnauthorized", err)
	}
	after, err := s.GetHelperEnrollment(enrollment.ID)
	if err != nil {
		t.Fatalf("GetHelperEnrollment: %v", err)
	}
	if after.LastSeenAt == nil || *after.LastSeenAt != originalSeen {
		t.Fatalf("stale-after-validation heartbeat mutated last_seen_at: got %v want %d", after.LastSeenAt, originalSeen)
	}
	if after.Status != "connected" || after.UninstalledAt != nil {
		t.Fatalf("stale-after-validation heartbeat mutated terminal/status fields: %+v", after)
	}
}

func TestHelperEnrollmentCredentialRotationRejectsStaleCredentialAfterUninstallValidation(t *testing.T) {
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-rotate-uninstall-race")
	now := time.UnixMilli(1778840000000)

	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Uninstall Race", []string{"helper_lifecycle"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, credential, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-race", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	originalSeen := *claimed.LastSeenAt
	tracedAt := now.Add(2 * time.Minute)

	helperEnrollmentCredentialRaceHook = func(s *Store, row *HelperEnrollment) error {
		newDigest := helperSecretDigest("newer-credential")
		return s.DB().Model(&HelperEnrollment{}).
			Where("id = ?", row.ID).
			Updates(map[string]any{"persistent_credential_digest": newDigest, "credential_rotated_at": tracedAt.UnixMilli(), "credential_generation": row.CredentialGeneration + 1}).Error
	}
	t.Cleanup(func() { helperEnrollmentCredentialRaceHook = nil })

	if _, err := s.MarkHelperEnrollmentUninstalled(enrollment.ID, credential, "device-race", now.Add(3*time.Minute)); !errors.Is(err, ErrHelperEnrollmentUnauthorized) {
		t.Fatalf("uninstall with credential stale after validation error=%v, want ErrHelperEnrollmentUnauthorized", err)
	}
	after, err := s.GetHelperEnrollment(enrollment.ID)
	if err != nil {
		t.Fatalf("GetHelperEnrollment: %v", err)
	}
	if after.LastSeenAt == nil || *after.LastSeenAt != originalSeen {
		t.Fatalf("stale-after-validation uninstall mutated last_seen_at: got %v want %d", after.LastSeenAt, originalSeen)
	}
	if after.Status != "connected" || after.UninstalledAt != nil {
		t.Fatalf("stale-after-validation uninstall mutated terminal/status fields: %+v", after)
	}
}

func TestHelperEnrollmentTerminalRaceBlocksHeartbeatAndUninstallAfterValidation(t *testing.T) {
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-terminal-race")
	now := time.UnixMilli(1778840000000)

	heartbeat, heartbeatSecret, err := s.CreateHelperEnrollment(owner.ID, "Heartbeat Terminal Race", []string{"status_collect"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment heartbeat: %v", err)
	}
	heartbeatClaim, heartbeatCredential, err := s.ClaimHelperEnrollment(heartbeat.ID, heartbeatSecret, "device-heartbeat", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Claim heartbeat: %v", err)
	}
	heartbeatSeen := *heartbeatClaim.LastSeenAt
	uninstalledAt := now.Add(2 * time.Minute).UnixMilli()
	helperEnrollmentCredentialRaceHook = func(s *Store, row *HelperEnrollment) error {
		return s.DB().Model(&HelperEnrollment{}).
			Where("id = ?", row.ID).
			Updates(map[string]any{"status": "uninstalled", "uninstalled_at": uninstalledAt}).Error
	}
	if _, err := s.UpdateHelperEnrollmentLastSeen(heartbeat.ID, heartbeatCredential, "device-heartbeat", now.Add(3*time.Minute)); !errors.Is(err, ErrHelperEnrollmentInactive) {
		t.Fatalf("heartbeat after terminal race error=%v, want ErrHelperEnrollmentInactive", err)
	}
	helperEnrollmentCredentialRaceHook = nil
	afterHeartbeat, err := s.GetHelperEnrollment(heartbeat.ID)
	if err != nil {
		t.Fatalf("GetHelperEnrollment heartbeat: %v", err)
	}
	if afterHeartbeat.Status != "uninstalled" || afterHeartbeat.UninstalledAt == nil || *afterHeartbeat.UninstalledAt != uninstalledAt {
		t.Fatalf("heartbeat terminal race row=%+v, want uninstalled at %d", afterHeartbeat, uninstalledAt)
	}
	if afterHeartbeat.LastSeenAt == nil || *afterHeartbeat.LastSeenAt != heartbeatSeen {
		t.Fatalf("heartbeat terminal race mutated last_seen_at: got %v want %d", afterHeartbeat.LastSeenAt, heartbeatSeen)
	}

	uninstall, uninstallSecret, err := s.CreateHelperEnrollment(owner.ID, "Uninstall Terminal Race", []string{"helper_lifecycle"}, now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("CreateHelperEnrollment uninstall: %v", err)
	}
	_, uninstallCredential, err := s.ClaimHelperEnrollment(uninstall.ID, uninstallSecret, "device-uninstall", now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("Claim uninstall: %v", err)
	}
	revokedAt := now.Add(6 * time.Minute).UnixMilli()
	helperEnrollmentCredentialRaceHook = func(s *Store, row *HelperEnrollment) error {
		return s.DB().Model(&HelperEnrollment{}).
			Where("id = ?", row.ID).
			Updates(map[string]any{"status": "revoked", "revoked_at": revokedAt}).Error
	}
	t.Cleanup(func() { helperEnrollmentCredentialRaceHook = nil })
	if _, err := s.MarkHelperEnrollmentUninstalled(uninstall.ID, uninstallCredential, "device-uninstall", now.Add(7*time.Minute)); !errors.Is(err, ErrHelperEnrollmentInactive) {
		t.Fatalf("uninstall after terminal race error=%v, want ErrHelperEnrollmentInactive", err)
	}
	afterUninstall, err := s.GetHelperEnrollment(uninstall.ID)
	if err != nil {
		t.Fatalf("GetHelperEnrollment uninstall: %v", err)
	}
	if afterUninstall.Status != "revoked" || afterUninstall.RevokedAt == nil || *afterUninstall.RevokedAt != revokedAt || afterUninstall.UninstalledAt != nil {
		t.Fatalf("uninstall terminal race row=%+v, want revoked at %d without uninstall", afterUninstall, revokedAt)
	}
}

func TestHelperEnrollmentRevokeDoesNotOverwriteUninstallRace(t *testing.T) {
	s := migratedStore(t)
	owner := helperOwner(t, s, "helper-revoke-race")
	now := time.UnixMilli(1778840000000)

	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Revoke Race", []string{"helper_lifecycle"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	_, credential, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-race", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	uninstalledAt := now.Add(2 * time.Minute).UnixMilli()
	helperEnrollmentRevokeRaceHook = func(s *Store, row *HelperEnrollment) error {
		if row.Status != "connected" {
			t.Fatalf("revoke hook saw status=%q, want connected before race", row.Status)
		}
		if _, err := s.MarkHelperEnrollmentUninstalled(row.ID, credential, "device-race", time.UnixMilli(uninstalledAt)); err != nil {
			return err
		}
		return nil
	}
	t.Cleanup(func() { helperEnrollmentRevokeRaceHook = nil })

	row, err := s.RevokeHelperEnrollmentForUser(enrollment.ID, owner.ID, owner.OrgID, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("RevokeHelperEnrollmentForUser: %v", err)
	}
	if row.Status != "uninstalled" || row.UninstalledAt == nil || *row.UninstalledAt != uninstalledAt || row.RevokedAt != nil {
		t.Fatalf("revoke overwrote uninstall race: %+v", row)
	}
	reloaded, err := s.GetHelperEnrollment(enrollment.ID)
	if err != nil {
		t.Fatalf("GetHelperEnrollment: %v", err)
	}
	if reloaded.Status != "uninstalled" || reloaded.RevokedAt != nil {
		t.Fatalf("persisted revoke race row=%+v, want uninstalled without revoked_at", reloaded)
	}
}
