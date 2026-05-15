package datalayer

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
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

func (r *sqliteHelperJobRepo) ConfigureOpenClawForEnrollments(_ context.Context, ownerUserID, orgID string, enrollmentIDs []string) (map[string]HelperConfigureOpenClawStatus, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	orgID = strings.TrimSpace(orgID)
	ids := compactStrings(enrollmentIDs)
	if ownerUserID == "" || orgID == "" || len(ids) == 0 {
		return map[string]HelperConfigureOpenClawStatus{}, nil
	}
	var rows []store.HelperJob
	if err := r.s.DB().
		Where("owner_user_id = ? AND org_id = ? AND enrollment_id IN ? AND job_type IN ?", ownerUserID, orgID, ids, configureOpenClawJobTypes()).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	byEnrollment := map[string][]*HelperJob{}
	for i := range rows {
		job := helperJobFromStore(&rows[i])
		if job == nil {
			continue
		}
		byEnrollment[job.EnrollmentID] = append(byEnrollment[job.EnrollmentID], job)
	}
	out := make(map[string]HelperConfigureOpenClawStatus, len(byEnrollment))
	for enrollmentID, jobs := range byEnrollment {
		out[enrollmentID] = buildConfigureOpenClawStatus(jobs)
	}
	return out, nil
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

func compactStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func configureOpenClawJobTypes() []string {
	return []string{"openclaw.install_from_manifest", "openclaw.configure_agent", "borgee_plugin.configure_connection", "service.lifecycle"}
}

func buildConfigureOpenClawStatus(jobs []*HelperJob) HelperConfigureOpenClawStatus {
	latest := latestConfigureOpenClawJobs(jobs)
	steps := make([]HelperConfigureOpenClawStep, 0, len(latest))
	for _, jobType := range configureOpenClawJobTypes() {
		if job := latest[jobType]; job != nil {
			steps = append(steps, configureOpenClawStepFromJob(job))
		}
	}
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].CreatedAt == steps[j].CreatedAt {
			return steps[i].JobType < steps[j].JobType
		}
		return steps[i].CreatedAt < steps[j].CreatedAt
	})

	state := "manual_debug"
	var reason *HelperConfigureOpenClawStep
	if step := firstStepMatching(steps, func(step HelperConfigureOpenClawStep) bool {
		return step.Status == "failed" && configureOpenClawFailureIsDenial(step.FailureCode)
	}); step != nil {
		state = "denied"
		reason = step
	} else if step := firstStepMatching(steps, func(step HelperConfigureOpenClawStep) bool { return step.Status == "failed" }); step != nil {
		state = "failed"
		reason = step
	} else if step := firstStepMatching(steps, func(step HelperConfigureOpenClawStep) bool { return step.Status == "queued" }); step != nil {
		state = "queued"
		reason = step
	} else if step := firstStepMatching(steps, func(step HelperConfigureOpenClawStep) bool {
		return step.Status == "leased" || step.Status == "running"
	}); step != nil {
		state = "running"
		reason = step
	} else if allConfigureOpenClawRequiredSucceeded(latest) {
		state = "succeeded"
	} else if step := firstStepMatching(steps, func(step HelperConfigureOpenClawStep) bool {
		return step.Status == "cancelled" || step.Status == "expired"
	}); step != nil {
		state = "manual_debug"
		reason = step
	} else if len(steps) > 0 {
		reason = &steps[len(steps)-1]
	}

	out := HelperConfigureOpenClawStatus{State: state, Label: configureOpenClawLabel(state), Steps: steps}
	if reason != nil {
		out.FailureCode = reason.FailureCode
		out.FailureMessage = reason.FailureMessage
		out.AuditRefs = reason.AuditRefs
		out.LogRefs = reason.LogRefs
	}
	return out
}

func latestConfigureOpenClawJobs(jobs []*HelperJob) map[string]*HelperJob {
	latest := map[string]*HelperJob{}
	for _, job := range jobs {
		if job == nil || !configureOpenClawJobType(job.JobType) {
			continue
		}
		prev := latest[job.JobType]
		if prev == nil || job.CreatedAt > prev.CreatedAt || (job.CreatedAt == prev.CreatedAt && job.ID > prev.ID) {
			latest[job.JobType] = job
		}
	}
	return latest
}

func configureOpenClawJobType(jobType string) bool {
	for _, known := range configureOpenClawJobTypes() {
		if jobType == known {
			return true
		}
	}
	return false
}

func configureOpenClawStepFromJob(job *HelperJob) HelperConfigureOpenClawStep {
	auditRefs, logRefs := configureOpenClawRefs(job.ResultSummary)
	step := HelperConfigureOpenClawStep{
		JobType:        job.JobType,
		Status:         job.Status,
		CreatedAt:      job.CreatedAt,
		CompletedAt:    job.CompletedAt,
		FailureCode:    stringFromPtr(job.FailureCode),
		FailureMessage: stringFromPtr(job.FailureMessage),
		AuditRefs:      auditRefs,
		LogRefs:        logRefs,
	}
	if step.Status == "leased" {
		step.Status = "running"
	}
	return step
}

func configureOpenClawRefs(raw *string) ([]string, []string) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	var summary struct {
		AuditRefs []string `json:"audit_refs"`
		LogRefs   []string `json:"log_refs"`
	}
	if err := json.Unmarshal([]byte(*raw), &summary); err != nil {
		return nil, nil
	}
	return compactBoundedRefs(summary.AuditRefs), compactBoundedRefs(summary.LogRefs)
}

func compactBoundedRefs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 128 || strings.ContainsAny(value, "/\\\x00\n\r") {
			continue
		}
		out = append(out, value)
		if len(out) == 16 {
			break
		}
	}
	return out
}

func stringFromPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func firstStepMatching(steps []HelperConfigureOpenClawStep, match func(HelperConfigureOpenClawStep) bool) *HelperConfigureOpenClawStep {
	for i := range steps {
		if match(steps[i]) {
			return &steps[i]
		}
	}
	return nil
}

func configureOpenClawFailureIsDenial(code string) bool {
	switch code {
	case "policy_denied", "path_denied", "domain_denied", "service_denied", "wrong_owner", "wrong_org":
		return true
	default:
		return false
	}
}

func allConfigureOpenClawRequiredSucceeded(latest map[string]*HelperJob) bool {
	for _, jobType := range configureOpenClawJobTypes() {
		job := latest[jobType]
		if job == nil || job.Status != "succeeded" {
			return false
		}
	}
	return true
}

func configureOpenClawLabel(state string) string {
	switch state {
	case "queued":
		return "Configure OpenClaw queued"
	case "running":
		return "Configure OpenClaw running"
	case "succeeded":
		return "Configure OpenClaw complete"
	case "failed":
		return "Configure OpenClaw failed"
	case "denied":
		return "Configure OpenClaw denied"
	case "revoked":
		return "Configure OpenClaw revoked"
	default:
		return "Manual debug required"
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
