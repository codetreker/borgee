package datalayer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"
)

func TestHelperJobRepositoryEnqueueProjectionAndErrorMapping(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "helper-job-dl")
	now := time.UnixMilli(1778840000000)
	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Mac Studio", []string{"openclaw_config"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, _, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-1", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	agent := datalayerHelperJobAgent(t, s, owner, "dl-openclaw-agent")
	seedDatalayerAgentConfig(t, s, agent.ID, 2, map[string]any{"name": "DL Agent"}, now)

	job, created, err := dl.HelperJobRepo.EnqueueForUser(context.Background(), EnqueueHelperJobInput{
		OwnerUserID:    owner.ID,
		OrgID:          owner.OrgID,
		EnrollmentID:   claimed.ID,
		JobType:        "openclaw.configure_agent",
		SchemaVersion:  1,
		PayloadJSON:    `{"agent_id":"` + agent.ID + `"}`,
		IdempotencyKey: "dl-retry",
	}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("EnqueueForUser: %v", err)
	}
	if !created || job.ID == "" || job.Status != "queued" || job.Category != "openclaw_config" {
		t.Fatalf("bad helper job projection created=%v job=%+v", created, job)
	}
	if job.OwnerUserID != "" || job.OrgID != "" || job.PayloadJSON != "" {
		t.Fatalf("datalayer projection leaked store-only owner/org/raw payload fields: %+v", job)
	}
	if !strings.HasPrefix(job.PayloadHash, "sha256:") || !strings.HasPrefix(job.ManifestDigest, "sha256:") {
		t.Fatalf("safe digest projection missing: %+v", job)
	}

	_, _, err = dl.HelperJobRepo.EnqueueForUser(context.Background(), EnqueueHelperJobInput{
		OwnerUserID:   owner.ID,
		OrgID:         owner.OrgID,
		EnrollmentID:  claimed.ID,
		JobType:       "command.run",
		SchemaVersion: 1,
		PayloadJSON:   `{}`,
	}, now.Add(2*time.Minute))
	if !errors.Is(err, ErrHelperJobUnknownType) {
		t.Fatalf("unknown type error=%v, want ErrHelperJobUnknownType", err)
	}
	if err := mapHelperJobErr(store.ErrHelperJobForbiddenField); !errors.Is(err, ErrHelperJobForbiddenField) {
		t.Fatalf("mapHelperJobErr forbidden field = %v", err)
	}
}

func TestHelperJobRepositoryPollAckCompleteProjection(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "helper-job-dl-transport")
	now := time.UnixMilli(1778840000000)
	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Mac Studio", []string{"openclaw_config"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, credential, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-1", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	agent := datalayerHelperJobAgent(t, s, owner, "dl-transport-agent")
	seedDatalayerAgentConfig(t, s, agent.ID, 3, map[string]any{"name": "DL Transport"}, now)

	job, created, err := dl.HelperJobRepo.EnqueueForUser(context.Background(), EnqueueHelperJobInput{
		OwnerUserID:   owner.ID,
		OrgID:         owner.OrgID,
		EnrollmentID:  claimed.ID,
		JobType:       "openclaw.configure_agent",
		SchemaVersion: 1,
		PayloadJSON:   `{"agent_id":"` + agent.ID + `"}`,
	}, now.Add(2*time.Minute))
	if err != nil || !created {
		t.Fatalf("EnqueueForUser created=%v err=%v", created, err)
	}

	lease, err := dl.HelperJobRepo.PollAndLeaseForHelper(context.Background(), HelperJobPollInput{
		EnrollmentID:     claimed.ID,
		HelperCredential: credential,
		HelperDeviceID:   "device-1",
	}, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("PollAndLeaseForHelper: %v", err)
	}
	if lease == nil || lease.Status != store.HelperJobPollLeased || lease.Job == nil || lease.Job.ID != job.ID || lease.Job.PayloadJSON == "" || lease.LeaseToken == "" {
		t.Fatalf("bad lease projection: %+v", lease)
	}

	acked, err := dl.HelperJobRepo.AckForHelper(context.Background(), HelperJobAckInput{
		EnrollmentID:     claimed.ID,
		JobID:            job.ID,
		HelperCredential: credential,
		HelperDeviceID:   "device-1",
		LeaseToken:       lease.LeaseToken,
		AckStatus:        "received",
	}, now.Add(3*time.Minute+time.Second))
	if err != nil || acked == nil || acked.Status != store.HelperJobStatusRunning {
		t.Fatalf("AckForHelper job=%+v err=%v", acked, err)
	}

	completed, err := dl.HelperJobRepo.CompleteForHelper(context.Background(), HelperJobResultInput{
		EnrollmentID:     claimed.ID,
		JobID:            job.ID,
		HelperCredential: credential,
		HelperDeviceID:   "device-1",
		LeaseToken:       lease.LeaseToken,
		Status:           store.HelperJobStatusFailed,
		FailureCode:      "policy_denied",
		FailureMessage:   "policy denied",
		ResultSummary:    `{"audit_refs":["audit-1"],"log_refs":[]}`,
	}, now.Add(3*time.Minute+2*time.Second))
	if err != nil || completed == nil || completed.Status != store.HelperJobStatusFailed || completed.FailureCode == nil || *completed.FailureCode != "policy_denied" || completed.CompletedAt == nil {
		t.Fatalf("CompleteForHelper job=%+v err=%v", completed, err)
	}

	if _, err := dl.HelperJobRepo.PollAndLeaseForHelper(context.Background(), HelperJobPollInput{EnrollmentID: claimed.ID, HelperCredential: "wrong", HelperDeviceID: "device-1"}, now.Add(4*time.Minute)); !errors.Is(err, ErrHelperJobUnauthorized) {
		t.Fatalf("wrong credential error=%v, want ErrHelperJobUnauthorized", err)
	}
}

func TestHelperJobConfigureOpenClawProjectionDerivation(t *testing.T) {
	t.Parallel()
	now := time.UnixMilli(1778840000000)
	msg := "policy handoff denied"
	denied := buildConfigureOpenClawStatus([]*HelperJob{
		configureOpenClawTestJob("old-config", "openclaw.configure_agent", store.HelperJobStatusSucceeded, now, nil, nil, nil),
		configureOpenClawTestJob("install", "openclaw.install_from_manifest", store.HelperJobStatusSucceeded, now.Add(time.Second), nil, nil, nil),
		configureOpenClawTestJob("config", "openclaw.configure_agent", store.HelperJobStatusFailed, now.Add(2*time.Second), strPtr("policy_denied"), &msg, strPtr(`{"audit_refs":["audit-1","../audit-secret","`+strings.Repeat("a", 129)+`"],"log_refs":["log-1","log/path","`+strings.Repeat("l", 129)+`"]}`)),
		configureOpenClawTestJob("ignored", "state.write", store.HelperJobStatusSucceeded, now.Add(3*time.Second), nil, nil, nil),
	})
	if denied.State != "denied" || denied.Label != "Configure OpenClaw denied" || denied.FailureCode != "policy_denied" || denied.FailureMessage != msg {
		t.Fatalf("denied projection = %+v", denied)
	}
	if got, want := denied.AuditRefs, []string{"audit-1"}; !equalStrings(got, want) {
		t.Fatalf("audit refs=%v, want %v", got, want)
	}
	if got, want := denied.LogRefs, []string{"log-1"}; !equalStrings(got, want) {
		t.Fatalf("log refs=%v, want %v", got, want)
	}
	if len(denied.Steps) != 2 || denied.Steps[1].CompletedAt == nil {
		t.Fatalf("steps should include safe latest known configure jobs: %+v", denied.Steps)
	}

	queued := buildConfigureOpenClawStatus([]*HelperJob{configureOpenClawTestJob("queued", "openclaw.install_from_manifest", store.HelperJobStatusQueued, now, nil, nil, nil)})
	if queued.State != "queued" || queued.Label != "Configure OpenClaw queued" {
		t.Fatalf("queued projection = %+v", queued)
	}
	running := buildConfigureOpenClawStatus([]*HelperJob{configureOpenClawTestJob("leased", "service.lifecycle", store.HelperJobStatusLeased, now, nil, nil, nil)})
	if running.State != "running" || running.Steps[0].Status != "running" || running.Label != "Configure OpenClaw running" {
		t.Fatalf("running projection = %+v", running)
	}
	failed := buildConfigureOpenClawStatus([]*HelperJob{configureOpenClawTestJob("failed", "openclaw.configure_agent", store.HelperJobStatusFailed, now, strPtr("execution_failed"), nil, nil)})
	if failed.State != "failed" || failed.Label != "Configure OpenClaw failed" {
		t.Fatalf("failed projection = %+v", failed)
	}
	manual := buildConfigureOpenClawStatus([]*HelperJob{configureOpenClawTestJob("expired", "service.lifecycle", store.HelperJobStatusExpired, now, strPtr("ttl_expired"), nil, nil)})
	if manual.State != "manual_debug" || manual.Label != "Manual debug required" || manual.FailureCode != "ttl_expired" {
		t.Fatalf("manual projection = %+v", manual)
	}
	succeeded := buildConfigureOpenClawStatus([]*HelperJob{
		configureOpenClawTestJob("install-ok", "openclaw.install_from_manifest", store.HelperJobStatusSucceeded, now, nil, nil, nil),
		configureOpenClawTestJob("config-ok", "openclaw.configure_agent", store.HelperJobStatusSucceeded, now.Add(time.Second), nil, nil, nil),
		configureOpenClawTestJob("plugin-ok", "borgee_plugin.configure_connection", store.HelperJobStatusSucceeded, now.Add(2*time.Second), nil, nil, nil),
		configureOpenClawTestJob("service-ok", "service.lifecycle", store.HelperJobStatusSucceeded, now.Add(3*time.Second), nil, nil, nil),
	})
	if succeeded.State != "succeeded" || succeeded.Label != "Configure OpenClaw complete" {
		t.Fatalf("succeeded projection = %+v", succeeded)
	}
	if configureOpenClawLabel("revoked") != "Configure OpenClaw revoked" {
		t.Fatal("revoked label mismatch")
	}
}

func TestHelperJobConfigureOpenClawForEnrollmentsScopesRows(t *testing.T) {
	t.Parallel()
	dl, s, owner := newHelperJobRepoFixture(t, "helper-job-dl-configure")
	now := time.UnixMilli(1778841000000)
	enrollment, secret, err := s.CreateHelperEnrollment(owner.ID, "Linux Host", []string{"openclaw_config", "openclaw_lifecycle"}, now)
	if err != nil {
		t.Fatalf("CreateHelperEnrollment: %v", err)
	}
	claimed, _, err := s.ClaimHelperEnrollment(enrollment.ID, secret, "device-configure", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimHelperEnrollment: %v", err)
	}
	seedDatalayerConfigureOpenClawJob(t, s, owner, claimed.ID, "repo-install", "openclaw.install_from_manifest", store.HelperJobStatusSucceeded, now)
	seedDatalayerConfigureOpenClawJob(t, s, owner, claimed.ID, "repo-config", "openclaw.configure_agent", store.HelperJobStatusSucceeded, now.Add(time.Second))
	seedDatalayerConfigureOpenClawJob(t, s, owner, claimed.ID, "repo-plugin", "borgee_plugin.configure_connection", store.HelperJobStatusSucceeded, now.Add(2*time.Second))
	seedDatalayerConfigureOpenClawJob(t, s, owner, claimed.ID, "repo-service", "service.lifecycle", store.HelperJobStatusSucceeded, now.Add(3*time.Second))

	byEnrollment, err := dl.HelperJobRepo.ConfigureOpenClawForEnrollments(context.Background(), " "+owner.ID+" ", " "+owner.OrgID+" ", []string{"", claimed.ID, claimed.ID})
	if err != nil {
		t.Fatalf("ConfigureOpenClawForEnrollments: %v", err)
	}
	projection := byEnrollment[claimed.ID]
	if projection.State != "succeeded" || len(projection.Steps) != 4 {
		t.Fatalf("projection = %+v", projection)
	}
	empty, err := dl.HelperJobRepo.ConfigureOpenClawForEnrollments(context.Background(), "", owner.OrgID, []string{claimed.ID})
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty owner projection=%v err=%v", empty, err)
	}
}

func TestHelperJobLeaseProjectionRetryBounds(t *testing.T) {
	t.Parallel()
	if helperJobLeaseFromStore(nil) != nil {
		t.Fatal("helperJobLeaseFromStore(nil) should return nil")
	}
	row := &store.HelperJobLease{
		Status:     store.HelperJobPollNoWork,
		RetryAfter: 60 * 24 * time.Hour,
	}
	lease := helperJobLeaseFromStore(row)
	if lease == nil || lease.RetryAfterMS != 2147483647 {
		t.Fatalf("retry_after_ms should cap at MaxInt32, got %+v", lease)
	}
}

func TestHelperJobErrorMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"invalid input", store.ErrHelperJobInvalidInput, ErrHelperJobInvalidInput},
		{"unknown type", store.ErrHelperJobUnknownType, ErrHelperJobUnknownType},
		{"type not enabled", store.ErrHelperJobTypeNotEnabled, ErrHelperJobTypeNotEnabled},
		{"schema invalid", store.ErrHelperJobSchemaInvalid, ErrHelperJobSchemaInvalid},
		{"forbidden field", store.ErrHelperJobForbiddenField, ErrHelperJobForbiddenField},
		{"enrollment not found", store.ErrHelperJobEnrollmentNotFound, ErrHelperJobEnrollmentNotFound},
		{"wrong owner", store.ErrHelperJobWrongOwner, ErrHelperJobWrongOwner},
		{"wrong org", store.ErrHelperJobWrongOrg, ErrHelperJobWrongOrg},
		{"forbidden", store.ErrHelperJobForbidden, ErrHelperJobForbidden},
		{"enrollment unclaimed", store.ErrHelperJobEnrollmentUnclaimed, ErrHelperJobEnrollmentUnclaimed},
		{"enrollment revoked", store.ErrHelperJobEnrollmentRevoked, ErrHelperJobEnrollmentRevoked},
		{"enrollment uninstalled", store.ErrHelperJobEnrollmentUninstalled, ErrHelperJobEnrollmentUninstalled},
		{"stale enrollment", store.ErrHelperJobStaleEnrollment, ErrHelperJobStaleEnrollment},
		{"enrollment inactive", store.ErrHelperJobEnrollmentInactive, ErrHelperJobEnrollmentInactive},
		{"delegation denied", store.ErrHelperJobDelegationDenied, ErrHelperJobDelegationDenied},
		{"manifest required", store.ErrHelperJobManifestRequired, ErrHelperJobManifestRequired},
		{"idempotency conflict", store.ErrHelperJobIdempotencyConflict, ErrHelperJobIdempotencyConflict},
		{"expired", store.ErrHelperJobExpired, ErrHelperJobExpired},
		{"unauthorized", store.ErrHelperJobUnauthorized, ErrHelperJobUnauthorized},
		{"stale credential", store.ErrHelperJobStaleCredential, ErrHelperJobStaleCredential},
		{"device mismatch", store.ErrHelperJobDeviceMismatch, ErrHelperJobDeviceMismatch},
		{"no work", store.ErrHelperJobNoWork, ErrHelperJobNoWork},
		{"lease lost", store.ErrHelperJobLeaseLost, ErrHelperJobLeaseLost},
		{"terminal conflict", store.ErrHelperJobTerminalConflict, ErrHelperJobTerminalConflict},
		{"not found", store.ErrHelperJobNotFound, ErrHelperJobNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := mapHelperJobErr(tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("mapHelperJobErr(%v)=%v, want %v", tc.in, err, tc.want)
			}
		})
	}
	if err := mapHelperJobErr(nil); err != nil {
		t.Fatalf("mapHelperJobErr(nil)=%v", err)
	}
	unknown := errors.New("other")
	if err := mapHelperJobErr(unknown); !errors.Is(err, unknown) {
		t.Fatalf("mapHelperJobErr(other)=%v, want original", err)
	}
	if helperJobFromStore(nil) != nil {
		t.Fatal("helperJobFromStore(nil) should return nil")
	}
}

func newHelperJobRepoFixture(t *testing.T, name string) (*DataLayer, *store.Store, *store.User) {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { _ = s.Close() })
	pt, err := presence.NewSessionsTracker(s.DB())
	if err != nil {
		t.Fatalf("presence.NewSessionsTracker: %v", err)
	}
	dl := NewDataLayer(s, pt, slog.New(slog.NewTextHandler(io.Discard, nil)))
	email := name + "@example.com"
	owner := &store.User{DisplayName: name, Role: "member", Email: &email, PasswordHash: "hash"}
	if err := dl.UserRepo.Create(context.Background(), owner); err != nil {
		t.Fatalf("UserRepo.Create: %v", err)
	}
	if _, err := s.CreateOrgForUser(owner, name+" Org"); err != nil {
		t.Fatalf("CreateOrgForUser: %v", err)
	}
	reloaded, err := s.GetUserByID(owner.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	return dl, s, reloaded
}

func datalayerHelperJobAgent(t *testing.T, s *store.Store, owner *store.User, name string) *store.User {
	t.Helper()
	apiKey := name + "-key"
	agent := &store.User{DisplayName: name, Role: "agent", OwnerID: &owner.ID, APIKey: &apiKey, OrgID: owner.OrgID, PasswordHash: "hash"}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("CreateUser agent: %v", err)
	}
	return agent
}

func configureOpenClawTestJob(id, jobType, status string, createdAt time.Time, failureCode, failureMessage, summary *string) *HelperJob {
	completed := createdAt.Add(time.Second).UnixMilli()
	job := &HelperJob{
		ID:             id,
		EnrollmentID:   "enr-1",
		JobType:        jobType,
		Status:         status,
		CreatedAt:      createdAt.UnixMilli(),
		CompletedAt:    &completed,
		FailureCode:    failureCode,
		FailureMessage: failureMessage,
		ResultSummary:  summary,
	}
	if status == store.HelperJobStatusQueued || status == store.HelperJobStatusLeased || status == store.HelperJobStatusRunning {
		job.CompletedAt = nil
	}
	return job
}

func seedDatalayerConfigureOpenClawJob(t *testing.T, s *store.Store, owner *store.User, enrollmentID, id, jobType, status string, createdAt time.Time) {
	t.Helper()
	createdMS := createdAt.UnixMilli()
	completedMS := createdAt.Add(time.Second).UnixMilli()
	job := &store.HelperJob{
		ID:               id,
		OwnerUserID:      owner.ID,
		OrgID:            owner.OrgID,
		EnrollmentID:     enrollmentID,
		JobType:          jobType,
		Category:         "openclaw_config",
		SchemaVersion:    1,
		PayloadJSON:      `{}`,
		PayloadHash:      "sha256:" + id,
		IdempotencyScope: id + "-scope",
		Status:           status,
		CreatedAt:        createdMS,
		UpdatedAt:        createdMS,
		CompletedAt:      &completedMS,
		ExpiresAt:        createdAt.Add(5 * time.Minute).UnixMilli(),
	}
	if err := s.DB().Create(job).Error; err != nil {
		t.Fatalf("seed helper job %s: %v", id, err)
	}
}

func strPtr(value string) *string {
	return &value
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func seedDatalayerAgentConfig(t *testing.T, s *store.Store, agentID string, version int64, blob map[string]any, now time.Time) {
	t.Helper()
	b, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("marshal config blob: %v", err)
	}
	if err := s.DB().Exec(`INSERT INTO agent_configs (agent_id, schema_version, blob, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, agentID, version, string(b), now.UnixMilli(), now.UnixMilli()).Error; err != nil {
		t.Fatalf("seed agent config: %v", err)
	}
}
