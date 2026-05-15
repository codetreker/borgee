package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"borgee-server/internal/store"
)

type HelperEnrollmentHandler struct {
	Store *store.Store
	Now   func() time.Time
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
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/status", h.handleStatus)
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/uninstall", h.handleUninstall)
}

func (h *HelperEnrollmentHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
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
	enrollment, secret, err := h.Store.CreateHelperEnrollment(user.ID, req.HostLabel, req.AllowedCategories, h.now())
	if err != nil {
		if errors.Is(err, store.ErrHelperEnrollmentInvalidCategory) || errors.Is(err, store.ErrHelperEnrollmentInvalidInput) || errors.Is(err, store.ErrHelperEnrollmentInvalidOwner) {
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
	rows, err := h.Store.ListHelperEnrollmentsForUser(user.ID, user.OrgID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list helper enrollments")
		return
	}
	out := make([]any, 0, len(rows))
	for i := range rows {
		out = append(out, h.serialize(&rows[i]))
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"enrollments": out})
}

func (h *HelperEnrollmentHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	row, err := h.Store.GetHelperEnrollmentForUser(r.PathValue("enrollmentId"), user.ID, user.OrgID)
	if err != nil {
		h.writeUserLookupError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"enrollment": h.serialize(row)})
}

func (h *HelperEnrollmentHandler) handleRevoke(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	row, err := h.Store.RevokeHelperEnrollmentForUser(r.PathValue("enrollmentId"), user.ID, user.OrgID, h.now())
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
	row, credential, err := h.Store.ClaimHelperEnrollment(r.PathValue("enrollmentId"), req.EnrollmentSecret, req.HelperDeviceID, h.now())
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
	row, err := h.Store.UpdateHelperEnrollmentLastSeen(r.PathValue("enrollmentId"), credential, req.HelperDeviceID, h.now())
	if err != nil {
		h.writeHelperError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"enrollment": h.serialize(row)})
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
	row, err := h.Store.MarkHelperEnrollmentUninstalled(r.PathValue("enrollmentId"), credential, req.HelperDeviceID, h.now())
	if err != nil {
		h.writeHelperError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"enrollment": h.serialize(row)})
}

func (h *HelperEnrollmentHandler) serialize(row *store.HelperEnrollment) map[string]any {
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
		"enrollment_id":      row.ID,
		"host_label":         row.HostLabel,
		"allowed_categories": row.AllowedCategoryList(),
		"status":             status,
		"fresh":              fresh,
		"created_at":         row.CreatedAt,
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
	return out
}

func (h *HelperEnrollmentHandler) writeUserLookupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrHelperEnrollmentNotFound):
		writeJSONError(w, http.StatusNotFound, "Helper enrollment not found")
	case errors.Is(err, store.ErrHelperEnrollmentForbidden):
		writeJSONError(w, http.StatusForbidden, "Forbidden")
	default:
		writeJSONError(w, http.StatusInternalServerError, "Helper enrollment error")
	}
}

func (h *HelperEnrollmentHandler) writeHelperError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrHelperEnrollmentInvalidInput):
		writeJSONError(w, http.StatusBadRequest, "Invalid helper enrollment")
	case errors.Is(err, store.ErrHelperEnrollmentUnauthorized):
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
	case errors.Is(err, store.ErrHelperEnrollmentDeviceMismatch), errors.Is(err, store.ErrHelperEnrollmentInactive):
		writeJSONError(w, http.StatusForbidden, "Forbidden")
	case errors.Is(err, store.ErrHelperEnrollmentNotFound):
		writeJSONError(w, http.StatusNotFound, "Helper enrollment not found")
	case errors.Is(err, store.ErrHelperEnrollmentAlreadyClaimed):
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
