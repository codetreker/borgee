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

// ListPluginConnections — #1049. One grouped query (no N+1) that pulls
// every configure / remove job for this enrollment+owner+org, then
// folds the stream into the current active connection set in memory.
// Active iff the latest succeeded configure for the connection_id is
// newer than the latest succeeded remove (or there is no remove).
//
// Known limitation (architect WARN-5 / acceptance-criteria.md "Out of
// Scope"): the projection is derived purely from the helper_jobs job
// stream. There is no reconciliation against (a) the daemon's
// per-connection JSON files on disk and (b) the plugin's actual wiring.
// If a job row is GC'd (TTL expiry / future prune) the projection
// silently drops that connection even though the daemon's file may
// still exist. A dedicated `plugin_connections` table will replace
// this projection before the connection count exceeds ~50 per agent;
// tracked in a follow-up issue.
func (r *sqliteHelperJobRepo) ListPluginConnections(_ context.Context, ownerUserID, orgID, enrollmentID string) ([]PluginConnectionRow, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	orgID = strings.TrimSpace(orgID)
	enrollmentID = strings.TrimSpace(enrollmentID)
	if ownerUserID == "" || orgID == "" || enrollmentID == "" {
		return nil, ErrHelperJobInvalidInput
	}
	// Owner+org scoping is done in the SQL filter, so a cross-owner
	// request silently returns an empty slice (handler upgrades that
	// to 404 by separately verifying enrollment existence via the
	// enrollment repo when needed; for #1049 the simpler 200+empty is
	// also acceptable since the empty-state UI handles both cases).
	// #1049 — cap the projection-source row scan. Acceptance §"Performance"
	// targets ≥100 connections per agent under 500ms p95; for now we hard-
	// cap at 5000 historical configure/remove rows per enrollment to keep
	// the in-memory fold bounded. If an enrollment ever exceeds this we
	// silently truncate to the oldest 5000 — the latest-wins fold then
	// reflects whatever recent activity fits the window. Pagination of
	// the projected output (per-connection rows) is intentionally not
	// implemented in this PR; documented as a known limitation in
	// acceptance-criteria.md ("Out of Scope") and a dedicated DB table
	// will replace the projection before the connection count exceeds
	// ~50 per agent.
	const helperJobsPluginConnectionsRowCap = 5000
	var rows []store.HelperJob
	if err := r.s.DB().
		Where("owner_user_id = ? AND org_id = ? AND enrollment_id = ? AND job_type IN ?",
			ownerUserID, orgID, enrollmentID,
			[]string{"borgee_plugin.configure_connection", "borgee_plugin.remove_connection"}).
		Order("created_at ASC, id ASC").
		Limit(helperJobsPluginConnectionsRowCap).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	type acc struct {
		latestConfigureAt int64
		latestRemoveAt    int64
		agentID           string
		channelID         string
	}
	byConn := map[string]*acc{}
	for i := range rows {
		row := &rows[i]
		if row.Status != "succeeded" {
			continue
		}
		var payload struct {
			ConnectionID string `json:"connection_id"`
			AgentID      string `json:"agent_id"`
			ChannelID    string `json:"channel_id"`
		}
		if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err != nil {
			continue
		}
		if !strings.HasPrefix(payload.ConnectionID, "borgee-plugin:") {
			continue
		}
		entry := byConn[payload.ConnectionID]
		if entry == nil {
			entry = &acc{}
			byConn[payload.ConnectionID] = entry
		}
		ts := row.CreatedAt
		if row.CompletedAt != nil {
			ts = *row.CompletedAt
		}
		if row.JobType == "borgee_plugin.configure_connection" {
			if ts >= entry.latestConfigureAt {
				entry.latestConfigureAt = ts
				entry.agentID = payload.AgentID
				if payload.ChannelID != "" {
					entry.channelID = payload.ChannelID
				}
			}
		} else if row.JobType == "borgee_plugin.remove_connection" {
			if ts > entry.latestRemoveAt {
				entry.latestRemoveAt = ts
			}
		}
	}
	out := make([]PluginConnectionRow, 0, len(byConn))
	for connID, entry := range byConn {
		if entry.latestConfigureAt == 0 {
			continue
		}
		if entry.latestRemoveAt >= entry.latestConfigureAt {
			continue
		}
		out = append(out, PluginConnectionRow{
			ConnectionID:     connID,
			AgentID:          entry.agentID,
			ChannelID:        entry.channelID,
			LastConfiguredAt: entry.latestConfigureAt,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LastConfiguredAt == out[j].LastConfiguredAt {
			return out[i].ConnectionID < out[j].ConnectionID
		}
		return out[i].LastConfiguredAt > out[j].LastConfiguredAt
	})
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
	// NOTE (#1049): borgee_plugin.remove_connection is intentionally NOT
	// in this set. The set drives the "Configure OpenClaw" multi-step
	// status projection (install → configure → bind → start), which is
	// a setup pipeline. Remove is a tear-down that should not regress
	// the configured-status to running/failed when it succeeds.
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
		// Lease projection carries the envelope fields the daemon-side
		// jobpolicy.validateJobSchema requires. The bare enqueue
		// projection above intentionally drops them (the REST enqueue
		// response is consumed by the operator, not the daemon, and
		// must not echo owner/org back to the API caller — see
		// TestHelperJobRepositoryEnqueueProjectionAndErrorMapping).
		job.OwnerUserID = row.OwnerUserID
		job.OrgID = row.OrgID
		job.HelperDeviceID = row.HelperDeviceID
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
