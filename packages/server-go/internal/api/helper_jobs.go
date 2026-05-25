package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"borgee-server/internal/datalayer"
	"borgee-server/internal/helpermanifest"
)

type HelperJobsHandler struct {
	Repo datalayer.HelperJobRepository
	Now  func() time.Time
	// ManifestProvider supplies the signed canonical helper-policy
	// manifest body injected into every leased-job payload (PR-4 amend
	// #1033). Nil → manifest_json field is omitted; the daemon then
	// rejects manifest-required jobs with ReasonManifestInvalid, which
	// is the safe pre-wiring default and matches the dev-no-key path.
	ManifestProvider *HelperManifestProvider

	// PushAdapter is the WS push integration. Nil-safe: when nil, the
	// enqueue handler skips the immediate push (job stays queued for
	// the next poll / connect-hook drain). PR-4 final amend wires this
	// in server.go via SetPushAdapter so internal/api stays free of
	// internal/ws import (interface seam pattern).
	PushAdapter HelperJobsPushAdapter

	// Logger surfaces best-effort push warnings. Nil-safe; falls back
	// to slog default.
	Logger *slog.Logger
}

// HelperJobsPushAdapter is the narrow seam between the helper-jobs
// REST/WS handler and internal/ws.Hub. Implementations:
//
//   - GetHelperSessionPlatform looks up the connected daemon's session
//     and returns (platform, deviceID, credential, true) if connected,
//     else "" / false. Returning the platform here avoids dragging the
//     ws.HelperSession type into the api package.
//
//   - SendJobFrameToHelper queues a `{"type":"job","job":<json>}` frame
//     to the helper session's write pump. Returns true iff a session
//     was connected and the buffer accepted the queue.
//
// Production wire (server.go): a thin adapter that closes over *ws.Hub.
// Unit tests substitute a fake to assert the push contract without a
// real WS stack.
type HelperJobsPushAdapter interface {
	GetHelperSessionPlatform(enrollmentID string) (platform, deviceID, credential string, ok bool)
	SendJobFrameToHelper(enrollmentID string, jobJSON json.RawMessage) bool
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
	enrollmentID := r.PathValue("enrollmentId")
	job, created, err := h.Repo.EnqueueForUser(r.Context(), datalayer.EnqueueHelperJobInput{
		OwnerUserID:    user.ID,
		OrgID:          user.OrgID,
		EnrollmentID:   enrollmentID,
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
	// PR-4 final amend: best-effort immediate WS push. If the daemon
	// is connected we lease + push the next queued job now so the
	// helper sees sub-second latency. If push fails (no session,
	// transient lease conflict, send-buffer full) the job stays
	// queued and the daemon picks it up via REST poll fallback OR the
	// next connect-hook drain. No error path to the API caller — the
	// enqueue contract is "job persisted", push is an optimization.
	h.tryPushAfterEnqueue(r.Context(), enrollmentID)
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
	platform, _ := helpermanifest.ParsePlatform(req.HelperPlatform)
	writeJSONResponse(w, http.StatusOK, map[string]any{"status": "leased", "job": h.serializeHelperJobLease(lease, platform)})
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
	// HelperPlatform: PR-4 final amend — REST poll backward-compat
	// platform selector. The WS upgrade path reads runtime.GOOS from
	// X-Helper-Platform; REST poll daemons (the deprecated rail) MUST
	// send the same token in the body. Missing/invalid → 400
	// helper_platform_required. v1 enum: {linux, darwin}.
	HelperPlatform string `json:"helper_platform"`
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
	if code, err := decodeStrictHelperJobRequest(r, &req, map[string]bool{"helper_device_id": true, "helper_platform": true, "wait_ms": true}); err != nil {
		return req, code, err
	}
	req.HelperDeviceID = strings.TrimSpace(req.HelperDeviceID)
	req.HelperPlatform = strings.TrimSpace(req.HelperPlatform)
	if req.HelperDeviceID == "" || len(req.HelperDeviceID) > 255 || req.WaitMS < 0 || req.WaitMS > 30000 {
		return req, "schema_invalid", errors.New("invalid helper poll request")
	}
	if req.HelperPlatform == "" {
		return req, "helper_platform_required", errors.New("helper_platform required")
	}
	if _, ok := helpermanifest.ParsePlatform(req.HelperPlatform); !ok {
		return req, "helper_platform_required", errors.New("helper_platform unknown")
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

func (h *HelperJobsHandler) serializeHelperJobLease(lease *datalayer.HelperJobLease, platform helpermanifest.Platform) map[string]any {
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
		// Amend gap #1: daemon-side jobpolicy.validateJobSchema requires
		// owner_user_id / org_id / helper_device_id / category /
		// payload_hash / expires_at to be non-empty. Prior to this PR the
		// lease frame omitted them, so every pushed job got rejected at
		// the schema gate (ReasonSchemaInvalid) before any executor ran.
		// All values come from the persisted helper_jobs row + the
		// credential subject the lease was issued to. Category is already
		// emitted above; the rest are added here.
		"owner_user_id": job.OwnerUserID,
		"org_id":        job.OrgID,
		"payload_hash":  job.PayloadHash,
		"expires_at":    job.ExpiresAt,
	}
	if job.HelperDeviceID != nil {
		out["helper_device_id"] = *job.HelperDeviceID
	}
	if binding := decodeHelperJobManifestBinding(job.ManifestBindingJSON); binding != nil {
		out["manifest_binding"] = binding
	}
	// PR-3 #1041: emit the binding as a raw JSON string too so the daemon's
	// no-root executors can pass byte-stable bytes into manifestpath.Resolve
	// + jobpolicy.Evaluate without re-marshalling. The structured
	// `manifest_binding` field above is kept for human-readable HTTP debug;
	// `manifest_binding_json` is the authoritative copy for executors.
	if job.ManifestBindingJSON != nil && strings.TrimSpace(*job.ManifestBindingJSON) != "" {
		out["manifest_binding_json"] = json.RawMessage(*job.ManifestBindingJSON)
	}
	// PR-4 amend (#1033): inject the signed canonical helper-policy
	// manifest body so daemon-side jobpolicy.verifyManifestAuthority can
	// recompute canonical bytes, verify the signature against its trust
	// root, and resolve binding PathIDs/ServiceIDs against the manifest's
	// authoritative declarations. Without this field present, 5/8 job
	// types (every manifest-required type) get rejected with
	// ReasonManifestInvalid.
	//
	// PR-4 final amend: platform-aware. The daemon's WS upgrade header
	// (X-Helper-Platform) flows down through the WS session into
	// this call; REST poll callers pass req.HelperPlatform. Empty
	// platform → linux (compat with the pre-amend default; production
	// callers always supply a platform).
	if h != nil && h.ManifestProvider != nil && strings.TrimSpace(job.ManifestDigest) != "" {
		platformToken := string(platform)
		if signed, _, err := h.ManifestProvider.SignedManifestForPlatform(platformToken); err == nil && len(signed) > 0 {
			out["manifest_json"] = json.RawMessage(signed)
		}
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

// SetPushAdapter wires the WS push integration after construction.
// Safe to call once at server boot; nil-safe at call time. Server.go
// constructs the helper-jobs handler before the hub is fully ready,
// then injects the adapter so the import direction stays
// api → (no ws) while the hub closes over both.
func (h *HelperJobsHandler) SetPushAdapter(a HelperJobsPushAdapter) {
	h.PushAdapter = a
}

// tryPushAfterEnqueue is the best-effort push triggered from
// handleEnqueue. Resolves the connected daemon's platform + helper
// device + credential via the adapter, leases the next queued job
// using the same store mutation REST poll uses, serializes the lease
// (platform-aware manifest), and queues the WS frame.
//
// All failure modes (no session, lease conflict, send-buffer full,
// provider error) are soft: the job remains queued and will be
// delivered either by the next REST poll or the connect-hook drain.
// Caller (enqueue handler) is unaware.
func (h *HelperJobsHandler) tryPushAfterEnqueue(ctx context.Context, enrollmentID string) {
	if h == nil || h.PushAdapter == nil || enrollmentID == "" {
		return
	}
	platformToken, deviceID, credential, ok := h.PushAdapter.GetHelperSessionPlatform(enrollmentID)
	if !ok {
		return
	}
	h.pushOneLeasedFrame(ctx, enrollmentID, platformToken, deviceID, credential)
}

// PushQueuedToHelper is the connect-hook drain: when a daemon WS
// session registers (RegisterHelper), the hub's helperConnectHook
// invokes this so any jobs that queued while the helper was
// disconnected ship immediately. The store's lease semantics are
// one-at-a-time per enrollment, so the first leased job is pushed;
// subsequent jobs follow on each ack/result the daemon sends back.
//
// Returns the number of frames pushed. Soft on every error so a hub
// connect callback never blocks the WS upgrade path.
func (h *HelperJobsHandler) PushQueuedToHelper(ctx context.Context, enrollmentID string) int {
	if h == nil || h.PushAdapter == nil || enrollmentID == "" {
		return 0
	}
	platformToken, deviceID, credential, ok := h.PushAdapter.GetHelperSessionPlatform(enrollmentID)
	if !ok {
		return 0
	}
	if h.pushOneLeasedFrame(ctx, enrollmentID, platformToken, deviceID, credential) {
		return 1
	}
	return 0
}

// pushOneLeasedFrame leases the next queued job for this enrollment
// + pushes one `{"type":"job","job":...}` frame. Returns true iff a
// frame was queued onto the session's send buffer.
//
// Lease idempotency: store.PollAndLeaseForHelper marks the row leased
// + sets lease_token under one transaction; a concurrent REST poll
// for the same enrollment will see "no work" until the lease expires
// or the daemon completes. This is why double-push to the same
// daemon does not duplicate execution.
func (h *HelperJobsHandler) pushOneLeasedFrame(ctx context.Context, enrollmentID, platformToken, deviceID, credential string) bool {
	if h.Repo == nil {
		return false
	}
	platform, ok := helpermanifest.ParsePlatform(platformToken)
	if !ok {
		// WS upgrade gates on ParsePlatform too; if we hit this branch
		// the session was somehow stored with a bad platform — log +
		// drop the push (REST poll fallback still works).
		h.logger().Warn("helper push skipped: unknown session platform",
			"enrollment_id", enrollmentID, "platform", platformToken)
		return false
	}
	lease, err := h.Repo.PollAndLeaseForHelper(ctx, datalayer.HelperJobPollInput{
		EnrollmentID:     enrollmentID,
		HelperCredential: credential,
		HelperDeviceID:   deviceID,
		WaitMS:           0,
	}, h.now())
	if err != nil {
		// Non-fatal: REST poll will retry. Log at debug to avoid log
		// spam when push-on-enqueue races a concurrent poll.
		h.logger().Debug("helper push lease skipped",
			"enrollment_id", enrollmentID, "err", err)
		return false
	}
	if lease == nil || lease.Status == "no_work" || lease.Job == nil {
		return false
	}
	frame, err := json.Marshal(h.serializeHelperJobLease(lease, platform))
	if err != nil {
		h.logger().Warn("helper push marshal failed",
			"enrollment_id", enrollmentID, "err", err)
		return false
	}
	pushed := h.PushAdapter.SendJobFrameToHelper(enrollmentID, json.RawMessage(frame))
	if !pushed {
		// PR-4 P0 review fix: the WS frame was dropped (no connected
		// session OR session's send buffer full / writer wedged) AFTER
		// store.PollAndLeaseForHelper already marked the row leased.
		// We log a warning so operators can correlate stuck rows with
		// helper-side slowness; we do NOT release the lease here. The
		// row recovers via the lease-expiry timer (REST poll + next
		// reconnect-hook drain still pick it up after expiry). A first-
		// class store-side "release lease" would touch the repo + a
		// fresh integration-test surface; for v1 we accept the lease-
		// expiry recovery window. Follow-up issue tracks adding an
		// explicit release path so the recovery is sub-second instead
		// of bounded by the lease TTL.
		h.logger().Warn("helper push dropped after lease",
			"enrollment_id", enrollmentID,
			"job_id", lease.Job.ID,
			"lease_token", lease.LeaseToken,
		)
	}
	return pushed
}

func (h *HelperJobsHandler) logger() *slog.Logger {
	if h != nil && h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}
