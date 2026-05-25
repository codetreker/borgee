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
	ErrHelperJobUnauthorized          = errors.New("datalayer: helper job unauthorized")
	ErrHelperJobStaleCredential       = errors.New("datalayer: helper job stale credential")
	ErrHelperJobDeviceMismatch        = errors.New("datalayer: helper job device mismatch")
	ErrHelperJobNoWork                = errors.New("datalayer: helper job no work")
	ErrHelperJobLeaseLost             = errors.New("datalayer: helper job lease lost")
	ErrHelperJobTerminalConflict      = errors.New("datalayer: helper job terminal conflict")
	ErrHelperJobNotFound              = errors.New("datalayer: helper job not found")
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
	ID                  string
	OwnerUserID         string
	OrgID               string
	EnrollmentID        string
	// HelperDeviceID is the device the enrollment was claimed under at
	// the time the job was leased. Nil on a pre-lease projection (the
	// queue row only acquires a binding once a daemon polls / receives
	// it). PR-4 amend (#1033): projected into the WS lease frame so the
	// daemon-side jobpolicy.Evaluate can match against its own enrollment
	// state (validateJobSchema requires this field present).
	HelperDeviceID      *string
	JobType             string
	Category            string
	SchemaVersion       int
	Status              string
	PayloadJSON         string
	PayloadHash         string
	ManifestDigest      string
	ManifestBindingJSON *string
	IdempotencyKey      *string
	CreatedAt           int64
	ExpiresAt           int64
	FailureCode         *string
	FailureMessage      *string
	LeasedAt            *int64
	LeaseExpiresAt      *int64
	CompletedAt         *int64
	ResultSummary       *string
}

type HelperConfigureOpenClawStatus struct {
	State          string
	Label          string
	FailureCode    string
	FailureMessage string
	AuditRefs      []string
	LogRefs        []string
	Steps          []HelperConfigureOpenClawStep
}

type HelperConfigureOpenClawStep struct {
	JobType        string
	Status         string
	CreatedAt      int64
	CompletedAt    *int64
	FailureCode    string
	FailureMessage string
	AuditRefs      []string
	LogRefs        []string
}

type HelperJobPollInput struct {
	EnrollmentID     string
	HelperCredential string
	HelperDeviceID   string
	WaitMS           int
}

type HelperJobAckInput struct {
	EnrollmentID     string
	JobID            string
	HelperCredential string
	HelperDeviceID   string
	LeaseToken       string
	AckStatus        string
}

type HelperJobResultInput struct {
	EnrollmentID     string
	JobID            string
	HelperCredential string
	HelperDeviceID   string
	LeaseToken       string
	Status           string
	FailureCode      string
	FailureMessage   string
	ResultSummary    string
}

type HelperJobLease struct {
	Status         string
	Job            *HelperJob
	LeaseToken     string
	LeaseExpiresAt int64
	Attempt        int
	RetryAfterMS   int
}

type HelperJobRepository interface {
	EnqueueForUser(ctx context.Context, input EnqueueHelperJobInput, now time.Time) (*HelperJob, bool, error)
	PollAndLeaseForHelper(ctx context.Context, input HelperJobPollInput, now time.Time) (*HelperJobLease, error)
	AckForHelper(ctx context.Context, input HelperJobAckInput, now time.Time) (*HelperJob, error)
	CompleteForHelper(ctx context.Context, input HelperJobResultInput, now time.Time) (*HelperJob, error)
	ConfigureOpenClawForEnrollments(ctx context.Context, ownerUserID, orgID string, enrollmentIDs []string) (map[string]HelperConfigureOpenClawStatus, error)
}
