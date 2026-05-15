package api

import (
	"bytes"
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

func (h *HelperJobsHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *HelperJobsHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/helper/enrollments/{enrollmentId}/jobs", authMw(http.HandlerFunc(h.handleEnqueue)))
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

type helperJobEnqueueRequest struct {
	JobType        string          `json:"job_type"`
	SchemaVersion  int             `json:"schema_version"`
	Payload        json.RawMessage `json:"payload"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
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
		case "shell", "argv", "command", "raw_command", "executable_path", "script", "service_unit", "path", "domain", "url", "credential", "credentials", "token", "env", "environment", "owner_user_id", "org_id", "device_id", "helper_device_id", "category", "agent_config_id", "config_hash", "config_version", "schema_hash":
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
	if job.CompletedAt != nil {
		out["completed_at"] = *job.CompletedAt
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

func (h *HelperJobsHandler) writeErrorCode(w http.ResponseWriter, status int, code, msg string) {
	writeJSONErrorCode(w, status, code, msg)
}
