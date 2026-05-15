package datalayer

import (
	"context"
	"errors"
	"time"

	"borgee-server/internal/store"
)

type sqliteHelperJobRepo struct{ s *store.Store }

func NewSQLiteHelperJobRepository(s *store.Store) HelperJobRepository {
	return &sqliteHelperJobRepo{s: s}
}

func (r *sqliteHelperJobRepo) EnqueueForUser(_ context.Context, input EnqueueHelperJobInput, now time.Time) (*HelperJob, bool, error) {
	row, created, err := r.s.EnqueueHelperJobForUser(store.EnqueueHelperJobInput{
		OwnerUserID:    input.OwnerUserID,
		OrgID:          input.OrgID,
		EnrollmentID:   input.EnrollmentID,
		JobType:        input.JobType,
		SchemaVersion:  input.SchemaVersion,
		PayloadJSON:    input.PayloadJSON,
		IdempotencyKey: input.IdempotencyKey,
	}, now)
	return helperJobFromStore(row), created, mapHelperJobErr(err)
}

func helperJobFromStore(row *store.HelperJob) *HelperJob {
	if row == nil {
		return nil
	}
	return &HelperJob{
		ID:             row.ID,
		EnrollmentID:   row.EnrollmentID,
		JobType:        row.JobType,
		Category:       row.Category,
		SchemaVersion:  row.SchemaVersion,
		Status:         row.Status,
		PayloadHash:    row.PayloadHash,
		ManifestDigest: row.ManifestDigest,
		IdempotencyKey: row.IdempotencyKey,
		CreatedAt:      row.CreatedAt,
		ExpiresAt:      row.ExpiresAt,
		FailureCode:    row.FailureCode,
		CompletedAt:    row.CompletedAt,
	}
}

func mapHelperJobErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, store.ErrHelperJobInvalidInput):
		return ErrHelperJobInvalidInput
	case errors.Is(err, store.ErrHelperJobUnknownType):
		return ErrHelperJobUnknownType
	case errors.Is(err, store.ErrHelperJobTypeNotEnabled):
		return ErrHelperJobTypeNotEnabled
	case errors.Is(err, store.ErrHelperJobSchemaInvalid):
		return ErrHelperJobSchemaInvalid
	case errors.Is(err, store.ErrHelperJobForbiddenField):
		return ErrHelperJobForbiddenField
	case errors.Is(err, store.ErrHelperJobEnrollmentNotFound):
		return ErrHelperJobEnrollmentNotFound
	case errors.Is(err, store.ErrHelperJobWrongOwner):
		return ErrHelperJobWrongOwner
	case errors.Is(err, store.ErrHelperJobWrongOrg):
		return ErrHelperJobWrongOrg
	case errors.Is(err, store.ErrHelperJobForbidden):
		return ErrHelperJobForbidden
	case errors.Is(err, store.ErrHelperJobEnrollmentUnclaimed):
		return ErrHelperJobEnrollmentUnclaimed
	case errors.Is(err, store.ErrHelperJobEnrollmentRevoked):
		return ErrHelperJobEnrollmentRevoked
	case errors.Is(err, store.ErrHelperJobEnrollmentUninstalled):
		return ErrHelperJobEnrollmentUninstalled
	case errors.Is(err, store.ErrHelperJobStaleEnrollment):
		return ErrHelperJobStaleEnrollment
	case errors.Is(err, store.ErrHelperJobEnrollmentInactive):
		return ErrHelperJobEnrollmentInactive
	case errors.Is(err, store.ErrHelperJobDelegationDenied):
		return ErrHelperJobDelegationDenied
	case errors.Is(err, store.ErrHelperJobManifestRequired):
		return ErrHelperJobManifestRequired
	case errors.Is(err, store.ErrHelperJobIdempotencyConflict):
		return ErrHelperJobIdempotencyConflict
	case errors.Is(err, store.ErrHelperJobExpired):
		return ErrHelperJobExpired
	default:
		return err
	}
}
