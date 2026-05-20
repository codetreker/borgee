package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/datalayer"
)

type fakeHelperJobRepo struct {
	pollLease *datalayer.HelperJobLease
	ackJob    *datalayer.HelperJob
	resultJob *datalayer.HelperJob

	// PR-2 #1038 — captured inputs for the WS-rail Process* shared
	// mutations so tests can assert the threaded args.
	lastAckInput    datalayer.HelperJobAckInput
	lastResultInput datalayer.HelperJobResultInput
}

func (r *fakeHelperJobRepo) EnqueueForUser(context.Context, datalayer.EnqueueHelperJobInput, time.Time) (*datalayer.HelperJob, bool, error) {
	return nil, false, errors.New("unused")
}

func (r *fakeHelperJobRepo) PollAndLeaseForHelper(_ context.Context, input datalayer.HelperJobPollInput, _ time.Time) (*datalayer.HelperJobLease, error) {
	if input.EnrollmentID == "" || input.HelperCredential == "" || input.HelperDeviceID == "" {
		return nil, datalayer.ErrHelperJobInvalidInput
	}
	return r.pollLease, nil
}

func (r *fakeHelperJobRepo) AckForHelper(_ context.Context, input datalayer.HelperJobAckInput, _ time.Time) (*datalayer.HelperJob, error) {
	r.lastAckInput = input
	if input.EnrollmentID == "" || input.JobID == "" || input.HelperCredential == "" || input.HelperDeviceID == "" || input.LeaseToken == "" || input.AckStatus != "received" {
		return nil, datalayer.ErrHelperJobInvalidInput
	}
	return r.ackJob, nil
}

func (r *fakeHelperJobRepo) CompleteForHelper(_ context.Context, input datalayer.HelperJobResultInput, _ time.Time) (*datalayer.HelperJob, error) {
	r.lastResultInput = input
	if input.EnrollmentID == "" || input.JobID == "" || input.HelperCredential == "" || input.HelperDeviceID == "" || input.LeaseToken == "" || input.Status == "" {
		return nil, datalayer.ErrHelperJobInvalidInput
	}
	return r.resultJob, nil
}

func (r *fakeHelperJobRepo) ConfigureOpenClawForEnrollments(context.Context, string, string, []string) (map[string]datalayer.HelperConfigureOpenClawStatus, error) {
	return map[string]datalayer.HelperConfigureOpenClawStatus{}, nil
}

func TestHelperJobsWriteHelperRailRepoErrorMapping(t *testing.T) {
	t.Parallel()
	h := &HelperJobsHandler{}
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"schema invalid", datalayer.ErrHelperJobSchemaInvalid, http.StatusBadRequest, "schema_invalid"},
		{"forbidden field", datalayer.ErrHelperJobForbiddenField, http.StatusBadRequest, "forbidden_field"},
		{"unauthorized", datalayer.ErrHelperJobUnauthorized, http.StatusUnauthorized, "unauthorized"},
		{"stale credential", datalayer.ErrHelperJobStaleCredential, http.StatusForbidden, "stale_credential"},
		{"device mismatch", datalayer.ErrHelperJobDeviceMismatch, http.StatusForbidden, "device_mismatch"},
		{"revoked", datalayer.ErrHelperJobEnrollmentRevoked, http.StatusForbidden, "revoked"},
		{"uninstalled", datalayer.ErrHelperJobEnrollmentUninstalled, http.StatusForbidden, "uninstalled"},
		{"inactive forbidden", datalayer.ErrHelperJobEnrollmentInactive, http.StatusForbidden, "forbidden"},
		{"not found", datalayer.ErrHelperJobNotFound, http.StatusNotFound, "not_found"},
		{"lease lost", datalayer.ErrHelperJobLeaseLost, http.StatusConflict, "lease_lost"},
		{"ttl expired", datalayer.ErrHelperJobExpired, http.StatusConflict, "ttl_expired"},
		{"terminal conflict", datalayer.ErrHelperJobTerminalConflict, http.StatusConflict, "terminal_conflict"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "helper_job_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.writeHelperRailRepoError(rr, tc.err)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%s, want %d", rr.Code, rr.Body.String(), tc.wantStatus)
			}
			var body map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("response JSON: %v body=%s", err, rr.Body.String())
			}
			if body["code"] != tc.wantCode {
				t.Fatalf("code=%v body=%v, want %s", body["code"], body, tc.wantCode)
			}
		})
	}
}

func TestHelperJobsWriteUserRailRepoErrorMapping(t *testing.T) {
	t.Parallel()
	h := &HelperJobsHandler{}
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"unknown type", datalayer.ErrHelperJobUnknownType, http.StatusBadRequest, "unknown_job_type"},
		{"type not enabled", datalayer.ErrHelperJobTypeNotEnabled, http.StatusBadRequest, "job_type_not_enabled"},
		{"manifest required", datalayer.ErrHelperJobManifestRequired, http.StatusBadRequest, "manifest_required"},
		{"schema invalid", datalayer.ErrHelperJobSchemaInvalid, http.StatusBadRequest, "schema_invalid"},
		{"invalid input", datalayer.ErrHelperJobInvalidInput, http.StatusBadRequest, "schema_invalid"},
		{"forbidden field", datalayer.ErrHelperJobForbiddenField, http.StatusBadRequest, "forbidden_field"},
		{"not found", datalayer.ErrHelperJobEnrollmentNotFound, http.StatusNotFound, "not_found"},
		{"wrong owner", datalayer.ErrHelperJobWrongOwner, http.StatusForbidden, "wrong_owner"},
		{"wrong org", datalayer.ErrHelperJobWrongOrg, http.StatusForbidden, "wrong_org"},
		{"unclaimed", datalayer.ErrHelperJobEnrollmentUnclaimed, http.StatusForbidden, "pending_or_unclaimed"},
		{"revoked", datalayer.ErrHelperJobEnrollmentRevoked, http.StatusForbidden, "revoked"},
		{"uninstalled", datalayer.ErrHelperJobEnrollmentUninstalled, http.StatusForbidden, "uninstalled"},
		{"stale", datalayer.ErrHelperJobStaleEnrollment, http.StatusForbidden, "stale_enrollment"},
		{"delegation", datalayer.ErrHelperJobDelegationDenied, http.StatusForbidden, "delegation_denied"},
		{"idempotency", datalayer.ErrHelperJobIdempotencyConflict, http.StatusConflict, "idempotency_conflict"},
		{"inactive", datalayer.ErrHelperJobEnrollmentInactive, http.StatusForbidden, "forbidden"},
		{"forbidden", datalayer.ErrHelperJobForbidden, http.StatusForbidden, "forbidden"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "helper_job_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.writeRepoError(rr, tc.err)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%s, want %d", rr.Code, rr.Body.String(), tc.wantStatus)
			}
			var body map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("response JSON: %v body=%s", err, rr.Body.String())
			}
			if body["code"] != tc.wantCode {
				t.Fatalf("code=%v body=%v, want %s", body["code"], body, tc.wantCode)
			}
		})
	}
}

func TestHelperJobsHelperRailHandlersWithRepoSuccessAndNoWork(t *testing.T) {
	t.Parallel()
	now := time.UnixMilli(1778840000000)
	job := &datalayer.HelperJob{
		ID:             "job-1",
		EnrollmentID:   "enroll-1",
		JobType:        "openclaw.configure_agent",
		Category:       "openclaw_config",
		SchemaVersion:  1,
		Status:         "leased",
		PayloadJSON:    `{"agent_id":"agent-1"}`,
		ManifestDigest: "sha256:manifest",
		CreatedAt:      now.UnixMilli(),
		ExpiresAt:      now.Add(time.Hour).UnixMilli(),
	}
	repo := &fakeHelperJobRepo{pollLease: &datalayer.HelperJobLease{Status: "leased", Job: job, LeaseToken: "lease-1", LeaseExpiresAt: now.Add(time.Minute).UnixMilli(), Attempt: 1}}
	h := &HelperJobsHandler{Repo: repo, Now: func() time.Time { return now }}

	req := helperRailRequest(t, `{"helper_device_id":"device-1","wait_ms":10}`)
	req.SetPathValue("enrollmentId", "enroll-1")
	rr := httptest.NewRecorder()
	h.handlePoll(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("poll status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body["status"] != "leased" || body["job"] == nil {
		t.Fatalf("poll body=%v err=%v", body, err)
	}

	repo.pollLease = nil
	req = helperRailRequest(t, `{"helper_device_id":"device-1"}`)
	req.SetPathValue("enrollmentId", "enroll-1")
	rr = httptest.NewRecorder()
	h.handlePoll(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("no-work poll status=%d body=%s", rr.Code, rr.Body.String())
	}
	body = map[string]any{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body["status"] != "no_work" || body["retry_after_ms"] != float64(5000) {
		t.Fatalf("no-work body=%v err=%v", body, err)
	}

	running := *job
	running.Status = "running"
	repo.ackJob = &running
	req = helperRailRequest(t, `{"helper_device_id":"device-1","lease_token":"lease-1","ack_status":"received"}`)
	req.SetPathValue("enrollmentId", "enroll-1")
	req.SetPathValue("jobId", "job-1")
	rr = httptest.NewRecorder()
	h.handleAck(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"status":"running"`) {
		t.Fatalf("ack status=%d body=%s", rr.Code, rr.Body.String())
	}

	failed := *job
	failed.Status = "failed"
	failureCode := "policy_denied"
	failed.FailureCode = &failureCode
	repo.resultJob = &failed
	req = helperRailRequest(t, `{"helper_device_id":"device-1","lease_token":"lease-1","status":"failed","failure_code":"policy_denied","failure_message":"denied"}`)
	req.SetPathValue("enrollmentId", "enroll-1")
	req.SetPathValue("jobId", "job-1")
	rr = httptest.NewRecorder()
	h.handleResult(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"failure_code":"policy_denied"`) {
		t.Fatalf("result status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func helperRailRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer helper-credential")
	return req
}

func TestHelperJobsDecodeHelperRailRequests(t *testing.T) {
	t.Parallel()
	poll, code, err := decodeHelperJobPollRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"helper_device_id":" device-1 ","wait_ms":250}`)))
	if err != nil || code != "" || poll.HelperDeviceID != "device-1" || poll.WaitMS != 250 {
		t.Fatalf("valid poll decoded as %+v code=%q err=%v", poll, code, err)
	}
	if _, code, err := decodeHelperJobPollRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"helper_device_id":"device-1","wait_ms":30001}`))); err == nil || code != "schema_invalid" {
		t.Fatalf("oversized wait code=%q err=%v, want schema_invalid", code, err)
	}
	if _, code, err := decodeHelperJobPollRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"helper_device_id":"device-1","url":"https://example.com"}`))); err == nil || code != "forbidden_field" {
		t.Fatalf("poll URL override code=%q err=%v, want forbidden_field", code, err)
	}

	ack, code, err := decodeHelperJobAckRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"helper_device_id":" device-1 ","lease_token":" lease ","ack_status":"received"}`)))
	if err != nil || code != "" || ack.HelperDeviceID != "device-1" || ack.LeaseToken != "lease" || ack.AckStatus != "received" {
		t.Fatalf("valid ack decoded as %+v code=%q err=%v", ack, code, err)
	}
	if _, code, err := decodeHelperJobAckRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"helper_device_id":"device-1","lease_token":"lease","ack_status":"done"}`))); err == nil || code != "schema_invalid" {
		t.Fatalf("bad ack status code=%q err=%v, want schema_invalid", code, err)
	}

	result, code, err := decodeHelperJobResultRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"helper_device_id":" device-1 ","lease_token":" lease ","status":" failed ","failure_code":" policy_denied ","failure_message":" denied ","result_summary":{"audit_refs":["a1"],"log_refs":[]}}`)))
	if err != nil || code != "" || result.HelperDeviceID != "device-1" || result.LeaseToken != "lease" || result.Status != "failed" || result.FailureCode != "policy_denied" || result.FailureMessage != "denied" || result.ResultSummary == "" {
		t.Fatalf("valid result decoded as %+v code=%q err=%v", result, code, err)
	}
	nullSummary, code, err := decodeHelperJobResultRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"helper_device_id":"device-1","lease_token":"lease","status":"cancelled","result_summary":null}`)))
	if err != nil || code != "" || nullSummary.ResultSummary != "" {
		t.Fatalf("null summary decoded as %+v code=%q err=%v", nullSummary, code, err)
	}
	if _, code, err := decodeHelperJobResultRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"helper_device_id":"device-1","lease_token":"lease","status":"failed","path":"/tmp/x"}`))); err == nil || code != "forbidden_field" {
		t.Fatalf("result path override code=%q err=%v, want forbidden_field", code, err)
	}
	if _, code, err := decodeHelperJobResultRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"helper_device_id":"device-1","lease_token":"","status":"failed"}`))); err == nil || code != "schema_invalid" {
		t.Fatalf("missing lease code=%q err=%v, want schema_invalid", code, err)
	}
	if _, code, err := decodeHelperJobResultRequest(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not-json`))); err == nil || code != "schema_invalid" {
		t.Fatalf("bad json code=%q err=%v, want schema_invalid", code, err)
	}
}

func TestHelperJobsSerializeLeaseAndJobOptionalFields(t *testing.T) {
	t.Parallel()
	idempotency := "retry-1"
	failureCode := "policy_denied"
	now := time.UnixMilli(1778840000000)
	completed := now.Add(time.Second).UnixMilli()
	job := &datalayer.HelperJob{
		ID:             "job-1",
		EnrollmentID:   "enroll-1",
		JobType:        "openclaw.configure_agent",
		SchemaVersion:  1,
		Status:         "failed",
		Category:       "openclaw_config",
		CreatedAt:      now.UnixMilli(),
		ExpiresAt:      now.Add(time.Hour).UnixMilli(),
		IdempotencyKey: &idempotency,
		FailureCode:    &failureCode,
		CompletedAt:    &completed,
	}
	serialized := serializeHelperJob(job)
	if serialized["idempotency_key"] != idempotency || serialized["failure_code"] != failureCode || serialized["completed_at"] != completed {
		t.Fatalf("optional job fields missing: %v", serialized)
	}

	job.PayloadJSON = `{"agent_id":"agent-1"}`
	job.ManifestDigest = "sha256:manifest"
	lease := &datalayer.HelperJobLease{Status: "leased", Job: job, LeaseToken: "lease-1", LeaseExpiresAt: now.Add(5 * time.Minute).UnixMilli(), Attempt: 2}
	serializedLease := serializeHelperJobLease(lease)
	if serializedLease["lease_token"] != "lease-1" || serializedLease["attempt"] != 2 || serializedLease["manifest_digest"] != "sha256:manifest" {
		t.Fatalf("lease fields missing: %v", serializedLease)
	}
	if payload := serializedLease["payload"].(map[string]any); payload["agent_id"] != "agent-1" {
		t.Fatalf("lease payload not decoded: %v", payload)
	}
}

// PR-2 #1038 — ProcessHelperAck / ProcessHelperResult are the shared
// mutations the WS read loop (internal/ws/helper.go) calls in place of
// the REST handlers. Pinning their direct call paths here so cov
// doesn't see them as 0%.
func TestProcessHelperAck_Direct(t *testing.T) {
	now := time.UnixMilli(1778840000000)
	running := &datalayer.HelperJob{ID: "job-1", Status: "running"}
	repo := &fakeHelperJobRepo{ackJob: running}
	h := &HelperJobsHandler{Repo: repo, Now: func() time.Time { return now }}
	if err := h.ProcessHelperAck(context.Background(), "enroll-1", "job-1", "lease-1", "tok", "device-1"); err != nil {
		t.Fatalf("ProcessHelperAck: %v", err)
	}
	// Repository receives the call with the threaded args.
	if repo.lastAckInput.JobID != "job-1" || repo.lastAckInput.LeaseToken != "lease-1" || repo.lastAckInput.AckStatus != "received" {
		t.Fatalf("repo input=%+v", repo.lastAckInput)
	}
}

func TestProcessHelperResult_Direct(t *testing.T) {
	now := time.UnixMilli(1778840000000)
	completed := &datalayer.HelperJob{ID: "job-1", Status: "failed"}
	repo := &fakeHelperJobRepo{resultJob: completed}
	h := &HelperJobsHandler{Repo: repo, Now: func() time.Time { return now }}
	summary := json.RawMessage(`{"audit_refs":["a-1"]}`)
	if err := h.ProcessHelperResult(context.Background(), "enroll-1", "job-1", "lease-1", "tok", "device-1", "failed", "policy_denied", "denied", summary); err != nil {
		t.Fatalf("ProcessHelperResult: %v", err)
	}
	if repo.lastResultInput.Status != "failed" || repo.lastResultInput.FailureCode != "policy_denied" || repo.lastResultInput.ResultSummary == "" {
		t.Fatalf("repo input=%+v", repo.lastResultInput)
	}
	// Null summary should not flow through to the repo as the string "null".
	if err := h.ProcessHelperResult(context.Background(), "enroll-1", "job-2", "lease-2", "tok", "device-1", "succeeded", "", "", json.RawMessage("null")); err != nil {
		t.Fatalf("ProcessHelperResult null: %v", err)
	}
	if repo.lastResultInput.ResultSummary != "" {
		t.Fatalf("null summary leaked: %q", repo.lastResultInput.ResultSummary)
	}
}
