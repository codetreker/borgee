package store

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"borgee-server/internal/idgen"
	"gorm.io/gorm"
)

var (
	ErrHelperEnrollmentInvalidCategory = errors.New("helper enrollment: invalid category")
	ErrHelperEnrollmentInvalidInput    = errors.New("helper enrollment: invalid input")
	ErrHelperEnrollmentInvalidOwner    = errors.New("helper enrollment: invalid owner")
	ErrHelperEnrollmentNotFound        = errors.New("helper enrollment: not found")
	ErrHelperEnrollmentForbidden       = errors.New("helper enrollment: forbidden")
	ErrHelperEnrollmentUnauthorized    = errors.New("helper enrollment: unauthorized")
	ErrHelperEnrollmentAlreadyClaimed  = errors.New("helper enrollment: already claimed")
	ErrHelperEnrollmentDeviceMismatch  = errors.New("helper enrollment: device mismatch")
	ErrHelperEnrollmentInactive        = errors.New("helper enrollment: inactive")
)

var helperAllowedCategories = map[string]bool{
	"openclaw_lifecycle": true,
	"openclaw_config":    true,
	"helper_lifecycle":   true,
	"status_collect":     true,
}

const HelperEnrollmentSecretTTL = 15 * time.Minute

func (s *Store) CreateHelperEnrollment(ownerUserID, hostLabel string, allowedCategories []string, now time.Time) (*HelperEnrollment, string, error) {
	owner, err := s.GetUserByID(ownerUserID)
	if err != nil {
		return nil, "", err
	}
	if owner.OrgID == "" {
		return nil, "", ErrHelperEnrollmentInvalidOwner
	}
	hostLabel = strings.TrimSpace(hostLabel)
	if hostLabel == "" || len(hostLabel) > 255 {
		return nil, "", ErrHelperEnrollmentInvalidInput
	}
	categories, err := normalizeHelperCategories(allowedCategories)
	if err != nil {
		return nil, "", err
	}
	categoriesJSON, err := json.Marshal(categories)
	if err != nil {
		return nil, "", err
	}
	secret, err := newHelperSecret()
	if err != nil {
		return nil, "", err
	}
	digest := helperSecretDigest(secret)
	expires := now.Add(HelperEnrollmentSecretTTL).UnixMilli()
	ts := now.UnixMilli()
	row := &HelperEnrollment{
		ID:                        idgen.NewID(),
		OwnerUserID:               owner.ID,
		OrgID:                     owner.OrgID,
		HostLabel:                 hostLabel,
		AllowedCategories:         string(categoriesJSON),
		Status:                    "pending",
		CreatedAt:                 ts,
		UpdatedAt:                 ts,
		EnrollmentSecretDigest:    &digest,
		EnrollmentSecretExpiresAt: &expires,
		CredentialGeneration:      1,
	}
	if err := s.db.Create(row).Error; err != nil {
		return nil, "", err
	}
	return row, secret, nil
}

func (s *Store) GetHelperEnrollment(id string) (*HelperEnrollment, error) {
	var row HelperEnrollment
	if err := s.db.Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHelperEnrollmentNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *Store) GetHelperEnrollmentForUser(id, ownerUserID, orgID string) (*HelperEnrollment, error) {
	row, err := s.GetHelperEnrollment(id)
	if err != nil {
		return nil, err
	}
	if row.OwnerUserID != ownerUserID || row.OrgID != orgID {
		return nil, ErrHelperEnrollmentForbidden
	}
	return row, nil
}

func (s *Store) ListHelperEnrollmentsForUser(ownerUserID, orgID string) ([]HelperEnrollment, error) {
	var rows []HelperEnrollment
	err := s.db.Where("owner_user_id = ? AND org_id = ?", ownerUserID, orgID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (s *Store) ClaimHelperEnrollment(id, enrollmentSecret, helperDeviceID string, now time.Time) (*HelperEnrollment, string, error) {
	helperDeviceID = strings.TrimSpace(helperDeviceID)
	if enrollmentSecret == "" || helperDeviceID == "" || len(helperDeviceID) > 255 {
		return nil, "", ErrHelperEnrollmentInvalidInput
	}
	var out HelperEnrollment
	var credential string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var row HelperEnrollment
		if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrHelperEnrollmentNotFound
			}
			return err
		}
		if row.Status != "pending" || row.ClaimedAt != nil || row.HelperDeviceID != nil || row.PersistentCredentialDigest != nil {
			return ErrHelperEnrollmentAlreadyClaimed
		}
		if row.RevokedAt != nil || row.UninstalledAt != nil || row.OwnerUserID == "" || row.OrgID == "" {
			return ErrHelperEnrollmentInactive
		}
		if row.EnrollmentSecretDigest == nil || row.EnrollmentSecretExpiresAt == nil || now.UnixMilli() > *row.EnrollmentSecretExpiresAt {
			return ErrHelperEnrollmentUnauthorized
		}
		if !constantTimeDigestMatch(*row.EnrollmentSecretDigest, enrollmentSecret) {
			return ErrHelperEnrollmentUnauthorized
		}
		var err error
		credential, err = newHelperSecret()
		if err != nil {
			return err
		}
		credentialDigest := helperSecretDigest(credential)
		ts := now.UnixMilli()
		updates := map[string]any{
			"helper_device_id":             helperDeviceID,
			"status":                       "connected",
			"last_seen_at":                 ts,
			"claimed_at":                   ts,
			"updated_at":                   ts,
			"enrollment_secret_digest":     nil,
			"enrollment_secret_expires_at": nil,
			"persistent_credential_digest": credentialDigest,
			"credential_created_at":        ts,
		}
		res := tx.Model(&HelperEnrollment{}).
			Where("id = ? AND status = 'pending' AND claimed_at IS NULL", id).
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrHelperEnrollmentAlreadyClaimed
		}
		if err := tx.Where("id = ?", id).First(&out).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return &out, credential, nil
}

func (s *Store) UpdateHelperEnrollmentLastSeen(id, credential, helperDeviceID string, now time.Time) (*HelperEnrollment, error) {
	row, err := s.loadActiveHelperEnrollmentForCredential(id, credential, helperDeviceID)
	if err != nil {
		return nil, err
	}
	ts := now.UnixMilli()
	if err := s.db.Model(&HelperEnrollment{}).
		Where("id = ? AND revoked_at IS NULL AND uninstalled_at IS NULL AND status IN ?", id, []string{"connected", "offline"}).
		Updates(map[string]any{"last_seen_at": ts, "updated_at": ts, "status": "connected"}).Error; err != nil {
		return nil, err
	}
	row, err = s.GetHelperEnrollment(id)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Store) MarkHelperEnrollmentUninstalled(id, credential, helperDeviceID string, now time.Time) (*HelperEnrollment, error) {
	row, err := s.loadActiveHelperEnrollmentForCredential(id, credential, helperDeviceID)
	if err != nil {
		return nil, err
	}
	if row.Status == "uninstalled" || row.UninstalledAt != nil {
		return row, nil
	}
	ts := now.UnixMilli()
	if err := s.db.Model(&HelperEnrollment{}).
		Where("id = ? AND revoked_at IS NULL AND uninstalled_at IS NULL", id).
		Updates(map[string]any{"status": "uninstalled", "uninstalled_at": ts, "updated_at": ts}).Error; err != nil {
		return nil, err
	}
	return s.GetHelperEnrollment(id)
}

func (s *Store) RotateHelperEnrollmentCredential(id, credential, helperDeviceID string, now time.Time) (*HelperEnrollment, string, error) {
	helperDeviceID = strings.TrimSpace(helperDeviceID)
	if credential == "" || helperDeviceID == "" || len(helperDeviceID) > 255 {
		return nil, "", ErrHelperEnrollmentInvalidInput
	}
	var out HelperEnrollment
	var newCredential string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var row HelperEnrollment
		if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrHelperEnrollmentNotFound
			}
			return err
		}
		if row.OwnerUserID == "" || row.OrgID == "" || row.ClaimedAt == nil || row.PersistentCredentialDigest == nil || row.RevokedAt != nil || row.UninstalledAt != nil || row.Status == "revoked" || row.Status == "uninstalled" || row.Status == "pending" {
			return ErrHelperEnrollmentInactive
		}
		if !constantTimeDigestMatch(*row.PersistentCredentialDigest, credential) {
			return ErrHelperEnrollmentUnauthorized
		}
		if row.HelperDeviceID == nil || *row.HelperDeviceID != helperDeviceID {
			return ErrHelperEnrollmentDeviceMismatch
		}

		var err error
		newCredential, err = newHelperSecret()
		if err != nil {
			return err
		}
		newDigest := helperSecretDigest(newCredential)
		ts := now.UnixMilli()
		generation := row.CredentialGeneration
		if generation < 1 {
			generation = 1
		}
		res := tx.Model(&HelperEnrollment{}).
			Where("id = ? AND revoked_at IS NULL AND uninstalled_at IS NULL AND persistent_credential_digest = ?", id, *row.PersistentCredentialDigest).
			Updates(map[string]any{
				"persistent_credential_digest": newDigest,
				"credential_rotated_at":        ts,
				"credential_generation":        generation + 1,
				"updated_at":                   ts,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrHelperEnrollmentUnauthorized
		}
		if err := tx.Where("id = ?", id).First(&out).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return &out, newCredential, nil
}

func (s *Store) RevokeHelperEnrollmentForUser(id, ownerUserID, orgID string, now time.Time) (*HelperEnrollment, error) {
	row, err := s.GetHelperEnrollmentForUser(id, ownerUserID, orgID)
	if err != nil {
		return nil, err
	}
	if row.UninstalledAt != nil || row.Status == "uninstalled" {
		return row, nil
	}
	if row.RevokedAt != nil || row.Status == "revoked" {
		return row, nil
	}
	ts := now.UnixMilli()
	if err := s.db.Model(&HelperEnrollment{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": "revoked", "revoked_at": ts, "updated_at": ts}).Error; err != nil {
		return nil, err
	}
	return s.GetHelperEnrollment(id)
}

func (e *HelperEnrollment) AllowedCategoryList() []string {
	var out []string
	if e == nil || e.AllowedCategories == "" {
		return []string{}
	}
	if err := json.Unmarshal([]byte(e.AllowedCategories), &out); err != nil {
		return []string{}
	}
	return out
}

func (s *Store) loadActiveHelperEnrollmentForCredential(id, credential, helperDeviceID string) (*HelperEnrollment, error) {
	row, err := s.GetHelperEnrollment(id)
	if err != nil {
		return nil, err
	}
	if row.OwnerUserID == "" || row.OrgID == "" || row.RevokedAt != nil || row.UninstalledAt != nil || row.Status == "revoked" || row.Status == "uninstalled" {
		return nil, ErrHelperEnrollmentInactive
	}
	if row.PersistentCredentialDigest == nil || credential == "" || !constantTimeDigestMatch(*row.PersistentCredentialDigest, credential) {
		return nil, ErrHelperEnrollmentUnauthorized
	}
	if row.HelperDeviceID == nil || *row.HelperDeviceID != helperDeviceID {
		return nil, ErrHelperEnrollmentDeviceMismatch
	}
	return row, nil
}

func normalizeHelperCategories(in []string) ([]string, error) {
	if len(in) == 0 || len(in) > len(helperAllowedCategories) {
		return nil, ErrHelperEnrollmentInvalidCategory
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		category := strings.TrimSpace(raw)
		if !helperAllowedCategories[category] || seen[category] {
			return nil, ErrHelperEnrollmentInvalidCategory
		}
		seen[category] = true
		out = append(out, category)
	}
	return out, nil
}

func newHelperSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func helperSecretDigest(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func constantTimeDigestMatch(digest, presented string) bool {
	presentedDigest := helperSecretDigest(presented)
	return subtle.ConstantTimeCompare([]byte(digest), []byte(presentedDigest)) == 1
}
