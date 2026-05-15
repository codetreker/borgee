package datalayer

import (
	"context"
	"errors"
	"math"
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

func (r *sqliteHelperJobRepo) PollAndLeaseForHelper(_ context.Context, input HelperJobPollInput, now time.Time) (*HelperJobLease, error) {
	lease, err := r.s.PollAndLeaseHelperJobForHelper(store.PollHelperJobInput{
		EnrollmentID:     input.EnrollmentID,
		HelperCredential: input.HelperCredential,
		HelperDeviceID:   input.HelperDeviceID,
	}, now)
	if err != nil {
		return nil, mapHelperJobErr(err)
	}
	return helperJobLeaseFromStore(lease), nil
}

func (r *sqliteHelperJobRepo) AckForHelper(_ context.Context, input HelperJobAckInput, now time.Time) (*HelperJob, error) {
	row, err := r.s.AckHelperJobForHelper(store.AckHelperJobInput{
		EnrollmentID:     input.EnrollmentID,
		JobID:            input.JobID,
		HelperCredential: input.HelperCredential,
		HelperDeviceID:   input.HelperDeviceID,
		LeaseToken:       input.LeaseToken,
		AckStatus:        input.AckStatus,
	}, now)
	return helperJobFromStore(row), mapHelperJobErr(err)
}

func (r *sqliteHelperJobRepo) CompleteForHelper(_ context.Context, input HelperJobResultInput, now time.Time) (*HelperJob, error) {
	row, err := r.s.CompleteHelperJobForHelper(store.CompleteHelperJobInput{
		EnrollmentID:      input.EnrollmentID,
		JobID:             input.JobID,
		HelperCredential:  input.HelperCredential,
		HelperDeviceID:    input.HelperDeviceID,
		LeaseToken:        input.LeaseToken,
		Status:            input.Status,
		FailureCode:       input.FailureCode,
		FailureMessage:    input.FailureMessage,
		ResultSummaryJSON: input.ResultSummary,
	}, now)
	return helperJobFromStore(row), mapHelperJobErr(err)
}

func helperJobFromStore(row *store.HelperJob) *HelperJob {
	if row == nil {
		return nil
	}
	return &HelperJob{
		ID:                  row.ID,
		EnrollmentID:        row.EnrollmentID,
		JobType:             row.JobType,
		Category:            row.Category,
		SchemaVersion:       row.SchemaVersion,
		Status:              row.Status,
		PayloadHash:         row.PayloadHash,
		ManifestDigest:      row.ManifestDigest,
		ManifestBindingJSON: row.ManifestBindingJSON,
		IdempotencyKey:      row.IdempotencyKey,
		CreatedAt:           row.CreatedAt,
		ExpiresAt:           row.ExpiresAt,
		FailureCode:         row.FailureCode,
		FailureMessage:      row.FailureMessage,
		LeasedAt:            row.LeasedAt,
		LeaseExpiresAt:      row.LeaseExpiresAt,
		CompletedAt:         row.CompletedAt,
		ResultSummary:       row.ResultSummaryJSON,
	}
}

func helperJobLeaseFromStore(row *store.HelperJobLease) *HelperJobLease {
	if row == nil {
		return nil
	}
	retryAfterMS := 0
	if row.RetryAfter > 0 {
		ms := row.RetryAfter.Milliseconds()
		if ms > math.MaxInt32 {
			ms = math.MaxInt32
		}
		retryAfterMS = int(ms)
	}
	return &HelperJobLease{
		Status:         row.Status,
		Job:            helperJobLeaseJobFromStore(row.Job),
		LeaseToken:     row.LeaseToken,
		LeaseExpiresAt: row.LeaseExpiresAt,
		Attempt:        row.Attempt,
		RetryAfterMS:   retryAfterMS,
	}
}

func helperJobLeaseJobFromStore(row *store.HelperJob) *HelperJob {
	job := helperJobFromStore(row)
	if job != nil && row != nil {
		job.PayloadJSON = row.PayloadJSON
	}
	return job
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
	case errors.Is(err, store.ErrHelperJobUnauthorized):
		return ErrHelperJobUnauthorized
	case errors.Is(err, store.ErrHelperJobStaleCredential):
		return ErrHelperJobStaleCredential
	case errors.Is(err, store.ErrHelperJobDeviceMismatch):
		return ErrHelperJobDeviceMismatch
	case errors.Is(err, store.ErrHelperJobNoWork):
		return ErrHelperJobNoWork
	case errors.Is(err, store.ErrHelperJobLeaseLost):
		return ErrHelperJobLeaseLost
	case errors.Is(err, store.ErrHelperJobTerminalConflict):
		return ErrHelperJobTerminalConflict
	case errors.Is(err, store.ErrHelperJobNotFound):
		return ErrHelperJobNotFound
	default:
		return err
	}
}
