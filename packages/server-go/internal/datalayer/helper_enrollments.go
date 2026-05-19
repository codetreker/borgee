package datalayer

import (
	"context"
	"errors"
	"time"
)

var (
	ErrHelperEnrollmentInvalidCategory = errors.New("datalayer: helper enrollment invalid category")
	ErrHelperEnrollmentInvalidInput    = errors.New("datalayer: helper enrollment invalid input")
	ErrHelperEnrollmentInvalidOwner    = errors.New("datalayer: helper enrollment invalid owner")
	ErrHelperEnrollmentNotFound        = errors.New("datalayer: helper enrollment not found")
	ErrHelperEnrollmentForbidden       = errors.New("datalayer: helper enrollment forbidden")
	ErrHelperEnrollmentUnauthorized    = errors.New("datalayer: helper enrollment unauthorized")
	ErrHelperEnrollmentAlreadyClaimed  = errors.New("datalayer: helper enrollment already claimed")
	ErrHelperEnrollmentDeviceMismatch  = errors.New("datalayer: helper enrollment device mismatch")
	ErrHelperEnrollmentInactive        = errors.New("datalayer: helper enrollment inactive")
)

// HelperEnrollment is the API-facing Helper enrollment projection. It excludes
// owner/org internals and credential material; the v1 SQLite adapter maps from
// store rows without exposing store types to internal/api.
type HelperEnrollment struct {
	ID                        string
	HostLabel                 string
	HelperDeviceID            *string
	AllowedCategories         []string
	Status                    string
	LastSeenAt                *int64
	CreatedAt                 int64
	ClaimedAt                 *int64
	RevokedAt                 *int64
	UninstalledAt             *int64
	EnrollmentSecretExpiresAt *int64
	CredentialCreatedAt       *int64
	CredentialRotatedAt       *int64
	CredentialGeneration      int
	// #999 update-detection projection. UpdatesAvailable is the parsed
	// snapshot the helper reported last (latest-wins, idempotent). Nil =
	// no check has been reported yet (pre-#999 enrollments, or helpers
	// that haven't completed a tick since rollout).
	UpdatesAvailable    []HelperEnrollmentUpdateAvailable
	LastUpdateCheckAt   *int64
}

// HelperEnrollmentUpdateAvailable is one drift entry. Class is normalized
// server-side to "security" or "feature" — see api.NormalizeUpdateClass.
type HelperEnrollmentUpdateAvailable struct {
	PluginID        string `json:"plugin_id"`
	CurrentVersion  string `json:"current_version"`
	ManifestVersion string `json:"manifest_version"`
	Class           string `json:"class"`
}

type HelperEnrollmentRepository interface {
	Create(ctx context.Context, ownerUserID, hostLabel string, allowedCategories []string, now time.Time) (*HelperEnrollment, string, error)
	ListForUser(ctx context.Context, ownerUserID, orgID string) ([]HelperEnrollment, error)
	GetForUser(ctx context.Context, id, ownerUserID, orgID string) (*HelperEnrollment, error)
	RevokeForUser(ctx context.Context, id, ownerUserID, orgID string, now time.Time) (*HelperEnrollment, error)
	Claim(ctx context.Context, id, enrollmentSecret, helperDeviceID string, now time.Time) (*HelperEnrollment, string, error)
	RotateCredential(ctx context.Context, id, credential, helperDeviceID string, now time.Time) (*HelperEnrollment, string, error)
	UpdateLastSeen(ctx context.Context, id, credential, helperDeviceID string, now time.Time) (*HelperEnrollment, error)
	MarkUninstalled(ctx context.Context, id, credential, helperDeviceID string, now time.Time) (*HelperEnrollment, error)
	// RecordUpdatesAvailable persists the latest drift snapshot the helper
	// reports (#999). Empty `updates` is a valid "no drift" snapshot;
	// callers must still pass a non-nil slice (use []HelperEnrollmentUpdateAvailable{}).
	RecordUpdatesAvailable(ctx context.Context, id, credential, helperDeviceID string, updates []HelperEnrollmentUpdateAvailable, now time.Time) (*HelperEnrollment, error)
}
