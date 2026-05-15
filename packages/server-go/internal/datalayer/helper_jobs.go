package datalayer

import (
	"context"
	"errors"
	"time"
)

var (
	ErrHelperJobInvalidInput          = errors.New("datalayer: helper job invalid input")
	ErrHelperJobUnknownType           = errors.New("datalayer: helper job unknown type")
	ErrHelperJobTypeNotEnabled        = errors.New("datalayer: helper job type not enabled")
	ErrHelperJobSchemaInvalid         = errors.New("datalayer: helper job schema invalid")
	ErrHelperJobForbiddenField        = errors.New("datalayer: helper job forbidden field")
	ErrHelperJobEnrollmentNotFound    = errors.New("datalayer: helper job enrollment not found")
	ErrHelperJobForbidden             = errors.New("datalayer: helper job forbidden")
	ErrHelperJobWrongOwner            = errors.New("datalayer: helper job wrong owner")
	ErrHelperJobWrongOrg              = errors.New("datalayer: helper job wrong org")
	ErrHelperJobEnrollmentInactive    = errors.New("datalayer: helper job enrollment inactive")
	ErrHelperJobEnrollmentUnclaimed   = errors.New("datalayer: helper job enrollment unclaimed")
	ErrHelperJobEnrollmentRevoked     = errors.New("datalayer: helper job enrollment revoked")
	ErrHelperJobEnrollmentUninstalled = errors.New("datalayer: helper job enrollment uninstalled")
	ErrHelperJobStaleEnrollment       = errors.New("datalayer: helper job stale enrollment")
	ErrHelperJobDelegationDenied      = errors.New("datalayer: helper job delegation denied")
	ErrHelperJobManifestRequired      = errors.New("datalayer: helper job manifest required")
	ErrHelperJobIdempotencyConflict   = errors.New("datalayer: helper job idempotency conflict")
	ErrHelperJobExpired               = errors.New("datalayer: helper job expired")
)

type EnqueueHelperJobInput struct {
	OwnerUserID    string
	OrgID          string
	EnrollmentID   string
	JobType        string
	SchemaVersion  int
	PayloadJSON    string
	IdempotencyKey string
}

type HelperJob struct {
	ID             string
	OwnerUserID    string
	OrgID          string
	EnrollmentID   string
	JobType        string
	Category       string
	SchemaVersion  int
	Status         string
	PayloadJSON    string
	PayloadHash    string
	ManifestDigest string
	IdempotencyKey *string
	CreatedAt      int64
	ExpiresAt      int64
	FailureCode    *string
	CompletedAt    *int64
}

type HelperJobRepository interface {
	EnqueueForUser(ctx context.Context, input EnqueueHelperJobInput, now time.Time) (*HelperJob, bool, error)
}
