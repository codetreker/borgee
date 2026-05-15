package datalayer

import (
	"context"
	"errors"
	"time"

	"borgee-server/internal/store"
)

type sqliteHelperEnrollmentRepo struct{ s *store.Store }

func NewSQLiteHelperEnrollmentRepository(s *store.Store) HelperEnrollmentRepository {
	return &sqliteHelperEnrollmentRepo{s: s}
}

func (r *sqliteHelperEnrollmentRepo) Create(_ context.Context, ownerUserID, hostLabel string, allowedCategories []string, now time.Time) (*HelperEnrollment, string, error) {
	row, secret, err := r.s.CreateHelperEnrollment(ownerUserID, hostLabel, allowedCategories, now)
	return helperEnrollmentFromStore(row), secret, mapHelperEnrollmentErr(err)
}

func (r *sqliteHelperEnrollmentRepo) ListForUser(_ context.Context, ownerUserID, orgID string) ([]HelperEnrollment, error) {
	rows, err := r.s.ListHelperEnrollmentsForUser(ownerUserID, orgID)
	if err != nil {
		return nil, mapHelperEnrollmentErr(err)
	}
	out := make([]HelperEnrollment, 0, len(rows))
	for i := range rows {
		out = append(out, *helperEnrollmentFromStore(&rows[i]))
	}
	return out, nil
}

func (r *sqliteHelperEnrollmentRepo) GetForUser(_ context.Context, id, ownerUserID, orgID string) (*HelperEnrollment, error) {
	row, err := r.s.GetHelperEnrollmentForUser(id, ownerUserID, orgID)
	return helperEnrollmentFromStore(row), mapHelperEnrollmentErr(err)
}

func (r *sqliteHelperEnrollmentRepo) RevokeForUser(_ context.Context, id, ownerUserID, orgID string, now time.Time) (*HelperEnrollment, error) {
	row, err := r.s.RevokeHelperEnrollmentForUser(id, ownerUserID, orgID, now)
	return helperEnrollmentFromStore(row), mapHelperEnrollmentErr(err)
}

func (r *sqliteHelperEnrollmentRepo) Claim(_ context.Context, id, enrollmentSecret, helperDeviceID string, now time.Time) (*HelperEnrollment, string, error) {
	row, credential, err := r.s.ClaimHelperEnrollment(id, enrollmentSecret, helperDeviceID, now)
	return helperEnrollmentFromStore(row), credential, mapHelperEnrollmentErr(err)
}

func (r *sqliteHelperEnrollmentRepo) UpdateLastSeen(_ context.Context, id, credential, helperDeviceID string, now time.Time) (*HelperEnrollment, error) {
	row, err := r.s.UpdateHelperEnrollmentLastSeen(id, credential, helperDeviceID, now)
	return helperEnrollmentFromStore(row), mapHelperEnrollmentErr(err)
}

func (r *sqliteHelperEnrollmentRepo) MarkUninstalled(_ context.Context, id, credential, helperDeviceID string, now time.Time) (*HelperEnrollment, error) {
	row, err := r.s.MarkHelperEnrollmentUninstalled(id, credential, helperDeviceID, now)
	return helperEnrollmentFromStore(row), mapHelperEnrollmentErr(err)
}

func helperEnrollmentFromStore(row *store.HelperEnrollment) *HelperEnrollment {
	if row == nil {
		return nil
	}
	return &HelperEnrollment{
		ID:                        row.ID,
		HostLabel:                 row.HostLabel,
		HelperDeviceID:            row.HelperDeviceID,
		AllowedCategories:         row.AllowedCategoryList(),
		Status:                    row.Status,
		LastSeenAt:                row.LastSeenAt,
		CreatedAt:                 row.CreatedAt,
		ClaimedAt:                 row.ClaimedAt,
		RevokedAt:                 row.RevokedAt,
		UninstalledAt:             row.UninstalledAt,
		EnrollmentSecretExpiresAt: row.EnrollmentSecretExpiresAt,
	}
}

func mapHelperEnrollmentErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, store.ErrHelperEnrollmentInvalidCategory):
		return ErrHelperEnrollmentInvalidCategory
	case errors.Is(err, store.ErrHelperEnrollmentInvalidInput):
		return ErrHelperEnrollmentInvalidInput
	case errors.Is(err, store.ErrHelperEnrollmentInvalidOwner):
		return ErrHelperEnrollmentInvalidOwner
	case errors.Is(err, store.ErrHelperEnrollmentNotFound):
		return ErrHelperEnrollmentNotFound
	case errors.Is(err, store.ErrHelperEnrollmentForbidden):
		return ErrHelperEnrollmentForbidden
	case errors.Is(err, store.ErrHelperEnrollmentUnauthorized):
		return ErrHelperEnrollmentUnauthorized
	case errors.Is(err, store.ErrHelperEnrollmentAlreadyClaimed):
		return ErrHelperEnrollmentAlreadyClaimed
	case errors.Is(err, store.ErrHelperEnrollmentDeviceMismatch):
		return ErrHelperEnrollmentDeviceMismatch
	case errors.Is(err, store.ErrHelperEnrollmentInactive):
		return ErrHelperEnrollmentInactive
	default:
		return err
	}
}
