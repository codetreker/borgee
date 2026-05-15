package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"borgee-server/internal/datalayer"
)

type HelperEnrollmentHandler struct {
	Repo    datalayer.HelperEnrollmentRepository
	JobRepo datalayer.HelperJobRepository
	Now     func() time.Time
}

func (h *HelperEnrollmentHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *HelperEnrollmentHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("POST /api/v1/helper/enrollments", wrap(h.handleCreate))
	mux.Handle("GET /api/v1/helper/enrollments", wrap(h.handleList))
	mux.Handle("GET /api/v1/helper/enrollments/{enrollmentId}", wrap(h.handleGet))
	mux.Handle("DELETE /api/v1/helper/enrollments/{enrollmentId}", wrap(h.handleRevoke))
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/claim", h.handleClaim)
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/rotate-credential", h.handleRotateCredential)
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/status", h.handleStatus)
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/uninstall", h.handleUninstall)
}

func (h *HelperEnrollmentHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	if !isHelperHumanOwner(user) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	var req struct {
		HostLabel         string   `json:"host_label"`
		AllowedCategories []string `json:"allowed_categories"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	enrollment, secret, err := h.Repo.Create(r.Context(), user.ID, req.HostLabel, req.AllowedCategories, h.now())
	if err != nil {
		if errors.Is(err, datalayer.ErrHelperEnrollmentInvalidCategory) || errors.Is(err, datalayer.ErrHelperEnrollmentInvalidInput) || errors.Is(err, datalayer.ErrHelperEnrollmentInvalidOwner) {
			writeJSONError(w, http.StatusBadRequest, "Invalid helper enrollment")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to create helper enrollment")
		return
	}
	writeJSONResponse(w, http.StatusCreated, map[string]any{
		"enrollment":                   h.serialize(enrollment),
		"enrollment_secret":            secret,
		"enrollment_secret_expires_at": enrollment.EnrollmentSecretExpiresAt,
	})
}

func (h *HelperEnrollmentHandler) handleList(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	if !isHelperHumanOwner(user) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	rows, err := h.Repo.ListForUser(r.Context(), user.ID, user.OrgID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list helper enrollments")
		return
	}
	out := make([]any, 0, len(rows))
	configureByEnrollment := h.configureOpenClawForRows(r, user.ID, user.OrgID, rows)
	for i := range rows {
		out = append(out, h.serializeWithConfigure(&rows[i], configureByEnrollment[rows[i].ID]))
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"enrollments": out})
}

func (h *HelperEnrollmentHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	if !isHelperHumanOwner(user) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	row, err := h.Repo.GetForUser(r.Context(), r.PathValue("enrollmentId"), user.ID, user.OrgID)
	if err != nil {
		h.writeUserLookupError(w, err)
		return
	}
	configureByEnrollment := h.configureOpenClawForRows(r, user.ID, user.OrgID, []datalayer.HelperEnrollment{*row})
	writeJSONResponse(w, http.StatusOK, map[string]any{"enrollment": h.serializeWithConfigure(row, configureByEnrollment[row.ID])})
}

func (h *HelperEnrollmentHandler) handleRevoke(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	if !isHelperHumanOwner(user) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	row, err := h.Repo.RevokeForUser(r.Context(), r.PathValue("enrollmentId"), user.ID, user.OrgID, h.now())
	if err != nil {
		h.writeUserLookupError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"enrollment_id": row.ID,
		"status":        row.Status,
		"revoked_at":    row.RevokedAt,
	})
}

func (h *HelperEnrollmentHandler) handleClaim(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnrollmentSecret string `json:"enrollment_secret"`
		HelperDeviceID   string `json:"helper_device_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	row, credential, err := h.Repo.Claim(r.Context(), r.PathValue("enrollmentId"), req.EnrollmentSecret, req.HelperDeviceID, h.now())
	if err != nil {
		h.writeHelperError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, map[string]any{
		"enrollment":        h.serialize(row),
		"helper_credential": credential,
	})
}

func (h *HelperEnrollmentHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	credential, ok := helperCredentialFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req struct {
		HelperDeviceID string `json:"helper_device_id"`
		State          string `json:"state"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.State != "connected" {
		writeJSONError(w, http.StatusBadRequest, "Invalid helper status")
		return
	}
	row, err := h.Repo.UpdateLastSeen(r.Context(), r.PathValue("enrollmentId"), credential, req.HelperDeviceID, h.now())
	if err != nil {
		h.writeHelperError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"enrollment": h.serialize(row)})
}

func (h *HelperEnrollmentHandler) handleRotateCredential(w http.ResponseWriter, r *http.Request) {
	credential, ok := helperCredentialFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req struct {
		HelperDeviceID string `json:"helper_device_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	row, newCredential, err := h.Repo.RotateCredential(r.Context(), r.PathValue("enrollmentId"), credential, req.HelperDeviceID, h.now())
	if err != nil {
		h.writeHelperError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"enrollment":        h.serialize(row),
		"helper_credential": newCredential,
	})
}

func (h *HelperEnrollmentHandler) handleUninstall(w http.ResponseWriter, r *http.Request) {
	credential, ok := helperCredentialFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req struct {
		HelperDeviceID string `json:"helper_device_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := h.Repo.MarkUninstalled(r.Context(), r.PathValue("enrollmentId"), credential, req.HelperDeviceID, h.now())
	if err != nil {
		h.writeHelperError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"enrollment": h.serialize(row)})
}

func (h *HelperEnrollmentHandler) serialize(row *datalayer.HelperEnrollment) map[string]any {
	return h.serializeWithConfigure(row, nil)
}

func (h *HelperEnrollmentHandler) serializeWithConfigure(row *datalayer.HelperEnrollment, configure *datalayer.HelperConfigureOpenClawStatus) map[string]any {
	status := row.Status
	fresh := false
	if row.Status == "connected" || row.Status == "offline" {
		if row.LastSeenAt != nil && h.now().UnixMilli()-*row.LastSeenAt <= int64(5*time.Minute/time.Millisecond) {
			status = "connected"
			fresh = true
		} else if row.ClaimedAt != nil {
			status = "offline"
		}
	}
	out := map[string]any{
		"enrollment_id":         row.ID,
		"host_label":            row.HostLabel,
		"allowed_categories":    row.AllowedCategories,
		"status":                status,
		"fresh":                 fresh,
		"credential_generation": row.CredentialGeneration,
		"created_at":            row.CreatedAt,
	}
	if row.HelperDeviceID != nil {
		out["helper_device_id"] = *row.HelperDeviceID
	}
	if row.LastSeenAt != nil {
		out["last_seen_at"] = *row.LastSeenAt
	}
	if row.ClaimedAt != nil {
		out["claimed_at"] = *row.ClaimedAt
	}
	if row.RevokedAt != nil {
		out["revoked_at"] = *row.RevokedAt
	}
	if row.UninstalledAt != nil {
		out["uninstalled_at"] = *row.UninstalledAt
	}
	if row.CredentialRotatedAt != nil {
		out["credential_rotated_at"] = *row.CredentialRotatedAt
	}
	if configure != nil {
		out["configure_openclaw"] = serializeConfigureOpenClaw(configure)
	}
	return out
}

func (h *HelperEnrollmentHandler) configureOpenClawForRows(r *http.Request, ownerUserID, orgID string, rows []datalayer.HelperEnrollment) map[string]*datalayer.HelperConfigureOpenClawStatus {
	out := map[string]*datalayer.HelperConfigureOpenClawStatus{}
	ids := make([]string, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	if h.JobRepo != nil && len(ids) > 0 {
		configured, err := h.JobRepo.ConfigureOpenClawForEnrollments(r.Context(), ownerUserID, orgID, ids)
		if err == nil {
			for id, status := range configured {
				copy := status
				out[id] = &copy
			}
		}
	}
	for i := range rows {
		if rows[i].Status == "revoked" || rows[i].RevokedAt != nil {
			out[rows[i].ID] = revokedConfigureOpenClawStatus("revoked")
		} else if rows[i].Status == "uninstalled" || rows[i].UninstalledAt != nil {
			out[rows[i].ID] = revokedConfigureOpenClawStatus("uninstalled")
		}
	}
	return out
}

func revokedConfigureOpenClawStatus(code string) *datalayer.HelperConfigureOpenClawStatus {
	return &datalayer.HelperConfigureOpenClawStatus{
		State:       "revoked",
		Label:       "Configure OpenClaw revoked",
		FailureCode: code,
	}
}

func serializeConfigureOpenClaw(status *datalayer.HelperConfigureOpenClawStatus) map[string]any {
	out := map[string]any{
		"state": status.State,
		"label": status.Label,
	}
	if status.FailureCode != "" {
		out["failure_code"] = status.FailureCode
	}
	if status.FailureMessage != "" {
		out["failure_message"] = status.FailureMessage
	}
	if len(status.AuditRefs) > 0 {
		out["audit_refs"] = status.AuditRefs
	}
	if len(status.LogRefs) > 0 {
		out["log_refs"] = status.LogRefs
	}
	if len(status.Steps) > 0 {
		steps := make([]any, 0, len(status.Steps))
		for i := range status.Steps {
			steps = append(steps, serializeConfigureOpenClawStep(status.Steps[i]))
		}
		out["steps"] = steps
	}
	return out
}

func serializeConfigureOpenClawStep(step datalayer.HelperConfigureOpenClawStep) map[string]any {
	out := map[string]any{
		"job_type":   step.JobType,
		"status":     step.Status,
		"created_at": step.CreatedAt,
	}
	if step.CompletedAt != nil {
		out["completed_at"] = *step.CompletedAt
	}
	if step.FailureCode != "" {
		out["failure_code"] = step.FailureCode
	}
	if step.FailureMessage != "" {
		out["failure_message"] = step.FailureMessage
	}
	if len(step.AuditRefs) > 0 {
		out["audit_refs"] = step.AuditRefs
	}
	if len(step.LogRefs) > 0 {
		out["log_refs"] = step.LogRefs
	}
	return out
}

func (h *HelperEnrollmentHandler) writeUserLookupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, datalayer.ErrHelperEnrollmentNotFound):
		writeJSONError(w, http.StatusNotFound, "Helper enrollment not found")
	case errors.Is(err, datalayer.ErrHelperEnrollmentForbidden):
		writeJSONError(w, http.StatusForbidden, "Forbidden")
	default:
		writeJSONError(w, http.StatusInternalServerError, "Helper enrollment error")
	}
}

func (h *HelperEnrollmentHandler) writeHelperError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, datalayer.ErrHelperEnrollmentInvalidInput):
		writeJSONError(w, http.StatusBadRequest, "Invalid helper enrollment")
	case errors.Is(err, datalayer.ErrHelperEnrollmentUnauthorized):
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
	case errors.Is(err, datalayer.ErrHelperEnrollmentDeviceMismatch), errors.Is(err, datalayer.ErrHelperEnrollmentInactive):
		writeJSONError(w, http.StatusForbidden, "Forbidden")
	case errors.Is(err, datalayer.ErrHelperEnrollmentNotFound):
		writeJSONError(w, http.StatusNotFound, "Helper enrollment not found")
	case errors.Is(err, datalayer.ErrHelperEnrollmentAlreadyClaimed):
		writeJSONError(w, http.StatusConflict, "Helper enrollment already claimed")
	default:
		writeJSONError(w, http.StatusInternalServerError, "Helper enrollment error")
	}
}

func helperCredentialFromRequest(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}
	credential := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	return credential, credential != ""
}
