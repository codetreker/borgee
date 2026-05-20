package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"borgee-server/internal/datalayer"
)

type HelperJobsHandler struct {
	Repo datalayer.HelperJobRepository
	Now  func() time.Time
}

// ProcessHelperAck applies the helper job ack mutation. Extracted from
// handleAck so the WS read loop (internal/ws.helper.go) can reuse the
// SAME store mutation without duplicating it. PR-2 #1038.
//
// The shared signature matches ws.HelperJobProcessor — internal/server
// instantiates one HelperJobsHandler and exposes the two Process*
// methods to the hub via an adapter.
func (h *HelperJobsHandler) ProcessHelperAck(ctx context.Context, enrollmentID, jobID, leaseToken, helperCredential, helperDeviceID string) error {
	_, err := h.Repo.AckForHelper(ctx, datalayer.HelperJobAckInput{
		EnrollmentID:     enrollmentID,
		JobID:            jobID,
		HelperCredential: helperCredential,
		HelperDeviceID:   helperDeviceID,
		LeaseToken:       leaseToken,
		AckStatus:        "received",
	}, h.now())
	return err
}

// ProcessHelperResult applies the helper job terminal-result mutation.
// Extracted from handleResult for ws.helper.go reuse.
func (h *HelperJobsHandler) ProcessHelperResult(ctx context.Context, enrollmentID, jobID, leaseToken, helperCredential, helperDeviceID, status, failureCode, failureMessage string, summary json.RawMessage) error {
	var summaryJSON string
	if len(summary) > 0 && string(summary) != "null" {
		summaryJSON = string(summary)
	}
	_, err := h.Repo.CompleteForHelper(ctx, datalayer.HelperJobResultInput{
		EnrollmentID:     enrollmentID,
		JobID:            jobID,
		HelperCredential: helperCredential,
		HelperDeviceID:   helperDeviceID,
		LeaseToken:       leaseToken,
		Status:           status,
		FailureCode:      failureCode,
		FailureMessage:   failureMessage,
		ResultSummary:    summaryJSON,
	}, h.now())
	return err
}

func (h *HelperJobsHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *HelperJobsHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/helper/enrollments/{enrollmentId}/jobs", authMw(http.HandlerFunc(h.handleEnqueue)))
	// PR-2 #1038: poll/ack/result REST endpoints remain mounted for
	// backward compatibility. Deprecated: new daemons use the WS
	// transport at /ws/helper/<enrollmentId> and exchange ack/result
	// frames inline. The store mutations (ProcessHelperAck /
	// ProcessHelperResult) are shared between both paths so behavior
	// stays identical across the cutover.
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/jobs/poll", h.handlePoll)
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/jobs/{jobId}/ack", h.handleAck)
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/jobs/{jobId}/result", h.handleResult)
}

func (h *HelperJobsHandler) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	if !isHelperHumanOwner(user) {
		h.writeErrorCode(w, http.StatusForbidden, "forbidden", "helper job enqueue forbidden")
		return
	}
	req, code, err := decodeHelperJobEnqueueRequest(r)
	if err != nil {
		h.writeErrorCode(w, http.StatusBadRequest, code, err.Error())
		return
	}
	job, created, err := h.Repo.EnqueueForUser(r.Context(), datalayer.EnqueueHelperJobInput{
		OwnerUserID:    user.ID,
		OrgID:          user.OrgID,
		EnrollmentID:   r.PathValue("enrollmentId"),
		JobType:        req.JobType,
		SchemaVersion:  req.SchemaVersion,
		PayloadJSON:    string(req.Payload),
		IdempotencyKey: req.IdempotencyKey,
	}, h.now())
	if err != nil {
		h.writeRepoError(w, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	writeJSONResponse(w, status, map[string]any{"job": serializeHelperJob(job)})
}

func (h *HelperJobsHandler) handlePoll(w http.ResponseWriter, r *http.Request) {
	credential, ok := helperCredentialFromRequest(r)
	if !ok {
		h.writeErrorCode(w, http.StatusUnauthorized, "unauthorized", "helper credential required")
		return
	}
	req, code, err := decodeHelperJobPollRequest(r)
	if err != nil {
		h.writeErrorCode(w, http.StatusBadRequest, code, err.Error())
		return
	}
	lease, err := h.Repo.PollAndLeaseForHelper(r.Context(), datalayer.HelperJobPollInput{
		EnrollmentID:     r.PathValue("enrollmentId"),
		HelperCredential: credential,
		HelperDeviceID:   req.HelperDeviceID,
		WaitMS:           req.WaitMS,
	}, h.now())
	if err != nil {
		h.writeHelperRailRepoError(w, err)
		return
	}
	if lease == nil || lease.Status == "no_work" || lease.Job == nil {
		retryAfter := 5000
		if lease != nil && lease.RetryAfterMS > 0 {
			retryAfter = lease.RetryAfterMS
		}
		writeJSONResponse(w, http.StatusOK, map[string]any{"status": "no_work", "retry_after_ms": retryAfter})
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"status": "leased", "job": serializeHelperJobLease(lease)})
}

func (h *HelperJobsHandler) handleAck(w http.ResponseWriter, r *http.Request) {
	credential, ok := helperCredentialFromRequest(r)
	if !ok {
		h.writeErrorCode(w, http.StatusUnauthorized, "unauthorized", "helper credential required")
		return
	}
	req, code, err := decodeHelperJobAckRequest(r)
	if err != nil {
		h.writeErrorCode(w, http.StatusBadRequest, code, err.Error())
		return
	}
	job, err := h.Repo.AckForHelper(r.Context(), datalayer.HelperJobAckInput{
		EnrollmentID:     r.PathValue("enrollmentId"),
		JobID:            r.PathValue("jobId"),
		HelperCredential: credential,
		HelperDeviceID:   req.HelperDeviceID,
		LeaseToken:       req.LeaseToken,
		AckStatus:        req.AckStatus,
	}, h.now())
	if err != nil {
		h.writeHelperRailRepoError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"job": serializeHelperJob(job)})
}

func (h *HelperJobsHandler) handleResult(w http.ResponseWriter, r *http.Request) {
	credential, ok := helperCredentialFromRequest(r)
	if !ok {
		h.writeErrorCode(w, http.StatusUnauthorized, "unauthorized", "helper credential required")
		return
	}
	req, code, err := decodeHelperJobResultRequest(r)
	if err != nil {
		h.writeErrorCode(w, http.StatusBadRequest, code, err.Error())
		return
	}
	job, err := h.Repo.CompleteForHelper(r.Context(), datalayer.HelperJobResultInput{
		EnrollmentID:     r.PathValue("enrollmentId"),
		JobID:            r.PathValue("jobId"),
		HelperCredential: credential,
		HelperDeviceID:   req.HelperDeviceID,
		LeaseToken:       req.LeaseToken,
		Status:           req.Status,
		FailureCode:      req.FailureCode,
		FailureMessage:   req.FailureMessage,
		ResultSummary:    req.ResultSummary,
	}, h.now())
	if err != nil {
		h.writeHelperRailRepoError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"job": serializeHelperJob(job)})
}

type helperJobEnqueueRequest struct {
	JobType        string          `json:"job_type"`
	SchemaVersion  int             `json:"schema_version"`
	Payload        json.RawMessage `json:"payload"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
}

type helperJobPollRequest struct {
	HelperDeviceID string `json:"helper_device_id"`
	WaitMS         int    `json:"wait_ms,omitempty"`
}

type helperJobAckRequest struct {
	HelperDeviceID string `json:"helper_device_id"`
	LeaseToken     string `json:"lease_token"`
	AckStatus      string `json:"ack_status"`
}

type helperJobResultRequest struct {
	HelperDeviceID string          `json:"helper_device_id"`
	LeaseToken     string          `json:"lease_token"`
	Status         string          `json:"status"`
	FailureCode    string          `json:"failure_code,omitempty"`
	FailureMessage string          `json:"failure_message,omitempty"`
	ResultSummary  string          `json:"-"`
	RawSummary     json.RawMessage `json:"result_summary,omitempty"`
}

func decodeHelperJobPollRequest(r *http.Request) (helperJobPollRequest, string, error) {
	var req helperJobPollRequest
	if code, err := decodeStrictHelperJobRequest(r, &req, map[string]bool{"helper_device_id": true, "wait_ms": true}); err != nil {
		return req, code, err
	}
	req.HelperDeviceID = strings.TrimSpace(req.HelperDeviceID)
	if req.HelperDeviceID == "" || len(req.HelperDeviceID) > 255 || req.WaitMS < 0 || req.WaitMS > 30000 {
		return req, "schema_invalid", errors.New("invalid helper poll request")
	}
	return req, "", nil
}

func decodeHelperJobAckRequest(r *http.Request) (helperJobAckRequest, string, error) {
	var req helperJobAckRequest
	if code, err := decodeStrictHelperJobRequest(r, &req, map[string]bool{"helper_device_id": true, "lease_token": true, "ack_status": true}); err != nil {
		return req, code, err
	}
	req.HelperDeviceID = strings.TrimSpace(req.HelperDeviceID)
	req.LeaseToken = strings.TrimSpace(req.LeaseToken)
	req.AckStatus = strings.TrimSpace(req.AckStatus)
	if req.HelperDeviceID == "" || req.LeaseToken == "" || req.AckStatus != "received" {
		return req, "schema_invalid", errors.New("invalid helper ack request")
	}
	return req, "", nil
}

func decodeHelperJobResultRequest(r *http.Request) (helperJobResultRequest, string, error) {
	var req helperJobResultRequest
	if code, err := decodeStrictHelperJobRequest(r, &req, map[string]bool{"helper_device_id": true, "lease_token": true, "status": true, "failure_code": true, "failure_message": true, "result_summary": true}); err != nil {
		return req, code, err
	}
	req.HelperDeviceID = strings.TrimSpace(req.HelperDeviceID)
	req.LeaseToken = strings.TrimSpace(req.LeaseToken)
	req.Status = strings.TrimSpace(req.Status)
	req.FailureCode = strings.TrimSpace(req.FailureCode)
	req.FailureMessage = strings.TrimSpace(req.FailureMessage)
	if len(req.RawSummary) > 0 && string(req.RawSummary) != "null" {
		req.ResultSummary = string(req.RawSummary)
	}
	if req.HelperDeviceID == "" || req.LeaseToken == "" || req.Status == "" {
		return req, "schema_invalid", errors.New("invalid helper result request")
	}
	return req, "", nil
}

func decodeStrictHelperJobRequest(r *http.Request, out any, allowed map[string]bool) (string, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err != nil {
		return "schema_invalid", errors.New("invalid helper job request")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return "schema_invalid", errors.New("invalid helper job request")
	}
	for key := range top {
		if !allowed[key] {
			return "forbidden_field", errors.New("unknown helper job request field")
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return "schema_invalid", errors.New("invalid helper job request")
	}
	return "", nil
}

func decodeHelperJobEnqueueRequest(r *http.Request) (helperJobEnqueueRequest, string, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err != nil {
		return helperJobEnqueueRequest{}, "schema_invalid", errors.New("invalid helper job envelope")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return helperJobEnqueueRequest{}, "schema_invalid", errors.New("invalid helper job envelope")
	}
	for key := range top {
		switch key {
		case "job_type", "schema_version", "payload", "idempotency_key":
			continue
		case "ttl", "expires_at", "deadline", "lease_expires_at":
			return helperJobEnqueueRequest{}, "ttl_invalid", errors.New("client ttl fields are not allowed")
		default:
			return helperJobEnqueueRequest{}, "extra_field", errors.New("unknown helper job envelope field")
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var req helperJobEnqueueRequest
	if err := dec.Decode(&req); err != nil {
		return helperJobEnqueueRequest{}, "schema_invalid", errors.New("invalid helper job envelope")
	}
	req.JobType = strings.TrimSpace(req.JobType)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if req.JobType == "" || req.SchemaVersion == 0 || len(req.Payload) == 0 || string(req.Payload) == "null" {
		return helperJobEnqueueRequest{}, "schema_invalid", errors.New("invalid helper job envelope")
	}
	if len(req.IdempotencyKey) > 128 {
		return helperJobEnqueueRequest{}, "schema_invalid", errors.New("invalid idempotency key")
	}
	if code := rejectHelperJobPayloadPreflight(req.Payload); code != "" {
		if code == "ttl_invalid" {
			return helperJobEnqueueRequest{}, code, errors.New("client ttl fields are not allowed")
		}
		return helperJobEnqueueRequest{}, code, errors.New("forbidden helper job payload field")
	}
	return req, "", nil
}

func rejectHelperJobPayloadPreflight(raw json.RawMessage) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return "schema_invalid"
	}
	for key := range payload {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "ttl", "expires_at", "deadline", "lease_expires_at":
			return "ttl_invalid"
		case "shell", "argv", "command", "raw_command", "executable_path", "script", "service_unit", "path", "paths", "path_id", "path_ids", "domain", "domains", "domain_id", "domain_ids", "url", "base_url", "credential", "credentials", "token", "api_key", "bot_user_id", "account_id", "env", "environment", "owner_user_id", "org_id", "device_id", "helper_device_id", "category", "agent_config_id", "config_hash", "config_version", "schema_hash", "connection_id", "manifest_id", "manifest_digest", "manifest_binding", "manifest_binding_json", "artifact", "artifact_id", "artifact_ids", "service_id", "service_ids", "install_plan_id":
			return "forbidden_field"
		}
	}
	return ""
}

func serializeHelperJob(job *datalayer.HelperJob) map[string]any {
	out := map[string]any{
		"job_id":         job.ID,
		"enrollment_id":  job.EnrollmentID,
		"job_type":       job.JobType,
		"schema_version": job.SchemaVersion,
		"status":         job.Status,
		"category":       job.Category,
		"created_at":     job.CreatedAt,
		"expires_at":     job.ExpiresAt,
	}
	if job.IdempotencyKey != nil {
		out["idempotency_key"] = *job.IdempotencyKey
	}
	if job.FailureCode != nil {
		out["failure_code"] = *job.FailureCode
	}
	if job.FailureMessage != nil {
		out["failure_message"] = *job.FailureMessage
	}
	if summary := decodeHelperJobResultSummary(job.ResultSummary); summary != nil {
		out["result_summary"] = summary
	}
	if job.CompletedAt != nil {
		out["completed_at"] = *job.CompletedAt
	}
	return out
}

func decodeHelperJobResultSummary(raw *string) map[string]any {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	var summary struct {
		AuditRefs []string `json:"audit_refs"`
		LogRefs   []string `json:"log_refs"`
	}
	if err := json.Unmarshal([]byte(*raw), &summary); err != nil {
		return nil
	}
	return map[string]any{"audit_refs": summary.AuditRefs, "log_refs": summary.LogRefs}
}

func serializeHelperJobLease(lease *datalayer.HelperJobLease) map[string]any {
	job := lease.Job
	payload := map[string]any{}
	if strings.TrimSpace(job.PayloadJSON) != "" {
		_ = json.Unmarshal([]byte(job.PayloadJSON), &payload)
	}
	out := map[string]any{
		"job_id":           job.ID,
		"enrollment_id":    job.EnrollmentID,
		"job_type":         job.JobType,
		"schema_version":   job.SchemaVersion,
		"status":           job.Status,
		"category":         job.Category,
		"payload":          payload,
		"manifest_digest":  job.ManifestDigest,
		"lease_token":      lease.LeaseToken,
		"lease_expires_at": lease.LeaseExpiresAt,
		"attempt":          lease.Attempt,
	}
	if binding := decodeHelperJobManifestBinding(job.ManifestBindingJSON); binding != nil {
		out["manifest_binding"] = binding
	}
	return out
}

func decodeHelperJobManifestBinding(raw *string) map[string]any {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	var binding struct {
		ManifestDigest string   `json:"manifest_digest"`
		ArtifactIDs    []string `json:"artifact_ids"`
		PathIDs        []string `json:"path_ids"`
		Domains        []string `json:"domains"`
		ServiceIDs     []string `json:"service_ids"`
	}
	if err := json.Unmarshal([]byte(*raw), &binding); err != nil || strings.TrimSpace(binding.ManifestDigest) == "" {
		return nil
	}
	out := map[string]any{"manifest_digest": binding.ManifestDigest}
	if len(binding.ArtifactIDs) > 0 {
		out["artifact_ids"] = binding.ArtifactIDs
	}
	if len(binding.PathIDs) > 0 {
		out["path_ids"] = binding.PathIDs
	}
	if len(binding.Domains) > 0 {
		out["domains"] = binding.Domains
	}
	if len(binding.ServiceIDs) > 0 {
		out["service_ids"] = binding.ServiceIDs
	}
	return out
}

func (h *HelperJobsHandler) writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, datalayer.ErrHelperJobUnknownType):
		h.writeErrorCode(w, http.StatusBadRequest, "unknown_job_type", "unknown helper job type")
	case errors.Is(err, datalayer.ErrHelperJobTypeNotEnabled):
		h.writeErrorCode(w, http.StatusBadRequest, "job_type_not_enabled", "helper job type is not enabled")
	case errors.Is(err, datalayer.ErrHelperJobManifestRequired):
		h.writeErrorCode(w, http.StatusBadRequest, "manifest_required", "server manifest binding is required")
	case errors.Is(err, datalayer.ErrHelperJobSchemaInvalid), errors.Is(err, datalayer.ErrHelperJobInvalidInput):
		h.writeErrorCode(w, http.StatusBadRequest, "schema_invalid", "invalid helper job schema")
	case errors.Is(err, datalayer.ErrHelperJobForbiddenField):
		h.writeErrorCode(w, http.StatusBadRequest, "forbidden_field", "forbidden helper job payload field")
	case errors.Is(err, datalayer.ErrHelperJobEnrollmentNotFound):
		h.writeErrorCode(w, http.StatusNotFound, "not_found", "helper enrollment not found")
	case errors.Is(err, datalayer.ErrHelperJobWrongOwner):
		h.writeErrorCode(w, http.StatusForbidden, "wrong_owner", "helper enrollment belongs to a different owner")
	case errors.Is(err, datalayer.ErrHelperJobWrongOrg):
		h.writeErrorCode(w, http.StatusForbidden, "wrong_org", "helper enrollment belongs to a different org")
	case errors.Is(err, datalayer.ErrHelperJobEnrollmentUnclaimed):
		h.writeErrorCode(w, http.StatusForbidden, "pending_or_unclaimed", "helper enrollment is not claimed")
	case errors.Is(err, datalayer.ErrHelperJobEnrollmentRevoked):
		h.writeErrorCode(w, http.StatusForbidden, "revoked", "helper enrollment is revoked")
	case errors.Is(err, datalayer.ErrHelperJobEnrollmentUninstalled):
		h.writeErrorCode(w, http.StatusForbidden, "uninstalled", "helper enrollment is uninstalled")
	case errors.Is(err, datalayer.ErrHelperJobStaleEnrollment):
		h.writeErrorCode(w, http.StatusForbidden, "stale_enrollment", "helper enrollment is stale")
	case errors.Is(err, datalayer.ErrHelperJobDelegationDenied):
		h.writeErrorCode(w, http.StatusForbidden, "delegation_denied", "helper enrollment category delegation denied")
	case errors.Is(err, datalayer.ErrHelperJobIdempotencyConflict):
		h.writeErrorCode(w, http.StatusConflict, "idempotency_conflict", "idempotency key conflicts with an active job")
	case errors.Is(err, datalayer.ErrHelperJobForbidden), errors.Is(err, datalayer.ErrHelperJobEnrollmentInactive):
		h.writeErrorCode(w, http.StatusForbidden, "forbidden", "helper job enqueue forbidden")
	default:
		h.writeErrorCode(w, http.StatusInternalServerError, "helper_job_error", "helper job enqueue failed")
	}
}

func (h *HelperJobsHandler) writeHelperRailRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, datalayer.ErrHelperJobInvalidInput), errors.Is(err, datalayer.ErrHelperJobSchemaInvalid):
		h.writeErrorCode(w, http.StatusBadRequest, "schema_invalid", "invalid helper job request")
	case errors.Is(err, datalayer.ErrHelperJobForbiddenField):
		h.writeErrorCode(w, http.StatusBadRequest, "forbidden_field", "forbidden helper job field")
	case errors.Is(err, datalayer.ErrHelperJobUnauthorized):
		h.writeErrorCode(w, http.StatusUnauthorized, "unauthorized", "helper credential unauthorized")
	case errors.Is(err, datalayer.ErrHelperJobStaleCredential):
		h.writeErrorCode(w, http.StatusForbidden, "stale_credential", "helper credential is stale")
	case errors.Is(err, datalayer.ErrHelperJobDeviceMismatch):
		h.writeErrorCode(w, http.StatusForbidden, "device_mismatch", "helper device mismatch")
	case errors.Is(err, datalayer.ErrHelperJobEnrollmentRevoked):
		h.writeErrorCode(w, http.StatusForbidden, "revoked", "helper enrollment is revoked")
	case errors.Is(err, datalayer.ErrHelperJobEnrollmentUninstalled):
		h.writeErrorCode(w, http.StatusForbidden, "uninstalled", "helper enrollment is uninstalled")
	case errors.Is(err, datalayer.ErrHelperJobEnrollmentUnclaimed), errors.Is(err, datalayer.ErrHelperJobEnrollmentInactive), errors.Is(err, datalayer.ErrHelperJobForbidden):
		h.writeErrorCode(w, http.StatusForbidden, "forbidden", "helper job forbidden")
	case errors.Is(err, datalayer.ErrHelperJobEnrollmentNotFound), errors.Is(err, datalayer.ErrHelperJobNotFound):
		h.writeErrorCode(w, http.StatusNotFound, "not_found", "helper job not found")
	case errors.Is(err, datalayer.ErrHelperJobLeaseLost):
		h.writeErrorCode(w, http.StatusConflict, "lease_lost", "helper job lease lost")
	case errors.Is(err, datalayer.ErrHelperJobExpired):
		h.writeErrorCode(w, http.StatusConflict, "ttl_expired", "helper job expired")
	case errors.Is(err, datalayer.ErrHelperJobTerminalConflict):
		h.writeErrorCode(w, http.StatusConflict, "terminal_conflict", "helper job terminal state conflicts")
	default:
		h.writeErrorCode(w, http.StatusInternalServerError, "helper_job_error", "helper job request failed")
	}
}

func (h *HelperJobsHandler) writeErrorCode(w http.ResponseWriter, status int, code, msg string) {
	writeJSONErrorCode(w, status, code, msg)
}
