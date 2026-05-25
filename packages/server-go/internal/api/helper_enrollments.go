package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"borgee-server/internal/datalayer"
)

type HelperEnrollmentHandler struct {
	Repo    datalayer.HelperEnrollmentRepository
	JobRepo datalayer.HelperJobRepository
	Now     func() time.Time
	// Logger is used by handlers that emit structured logs (e.g. update-
	// detection manifest load fallback warnings). Nil safe — passed through
	// to LoadManifestEntries which itself nil-checks.
	Logger *slog.Logger
	// PublicHelperOrigin — optional override for the scheme+host that
	// handleCreate stamps into `install_command`. #1052: in docker dev-stack
	// or behind a reverse proxy the inbound r.Host is not the address the
	// helper VM needs to dial. When set (e.g. `ws://borgee-server:4900`
	// or `wss://borgee.codetrek.cn`) it wins over r.Host / X-Forwarded-*.
	// When empty handleCreate falls back to the prior r.Host derivation,
	// keeping existing prod / single-host deploys working with zero config.
	// Sourced from config.Config.PublicHelperOrigin (env
	// BORGEE_PUBLIC_HELPER_ORIGIN) at server wiring; validated once at
	// boot by config.Validate() so handleCreate can trust the format.
	PublicHelperOrigin string
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
	// #999 update detection. The helper POSTs a snapshot of locally-
	// installed plugin versions; the server compares vs the current signed
	// manifest, computes drift (with class normalization), persists the
	// drift snapshot, and returns it so the helper can log a per-class
	// prompt event (security = prominent, feature = settings-panel only).
	// Apply is a separate user-confirmed path (out of scope for this PR;
	// blueprint §1.3 explicitly bans auto-apply).
	mux.HandleFunc("POST /api/v1/helper/enrollments/{enrollmentId}/installed-versions", h.handleInstalledVersions)
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
	// The enrollment token is `<enrollment_id>.<enrollment_secret>` —
	// `borgee install --token` splits on the first `.` (see
	// packages/borgee/internal/cli/install/install.go::tokenParts). This is
	// the ONE-time string the operator pastes; the server never stores it
	// (only the digest of the secret) so we must return it in the create
	// response and nowhere else.
	enrollmentToken := enrollment.ID + "." + secret
	installCommand := buildHelperInstallCommand(r, enrollmentToken, h.PublicHelperOrigin)
	writeJSONResponse(w, http.StatusCreated, map[string]any{
		"enrollment":                   h.serialize(enrollment),
		"enrollment_secret":            secret,
		"enrollment_secret_expires_at": enrollment.EnrollmentSecretExpiresAt,
		"enrollment_token":             enrollmentToken,
		"install_command":              installCommand,
	})
}

// buildHelperInstallCommand returns the one-line `sudo npx ...` command an
// operator pastes on the host VM.
//
// Origin selection priority (#1052):
//  1. publicOrigin (from BORGEE_PUBLIC_HELPER_ORIGIN env at boot) when non-
//     empty. Required for docker dev-stack (server bound on 127.0.0.1:4900
//     on the host, helper container reaches it via docker network DNS
//     `borgee-server:4900`) and for deploys where the public URL doesn't
//     match the Host header reaching server-go (e.g. behind a reverse
//     proxy that strips X-Forwarded-Host). Format validated at boot by
//     config.Validate (`ws://...` or `wss://...`, no path) so we use it
//     verbatim here.
//  2. Otherwise derive from the inbound request: X-Forwarded-Proto /
//     X-Forwarded-Host (production behind a TLS terminating proxy that
//     does set the headers) take precedence, else r.TLS != nil → wss,
//     and r.Host. This was the original (pre-#1052) behavior; it is the
//     correct default when the server-facing host IS the operator-facing
//     host (single-host on-prem / staging) and is preserved as the
//     fallback so adding the env knob does not regress existing deploys.
func buildHelperInstallCommand(r *http.Request, enrollmentToken string, publicOrigin string) string {
	if origin := strings.TrimSpace(publicOrigin); origin != "" {
		return fmt.Sprintf("sudo npx @codetreker/borgee-remote-agent install --server %s --token %s", origin, enrollmentToken)
	}
	scheme := "wss"
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		if proto == "http" || proto == "ws" {
			scheme = "ws"
		}
	} else if r.TLS == nil {
		scheme = "ws"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return fmt.Sprintf("sudo npx @codetreker/borgee-remote-agent install --server %s://%s --token %s", scheme, host, enrollmentToken)
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

// installedVersionsRequest is the helper-posted snapshot of locally installed
// plugin versions. Helper sends every plugin it has on disk (per
// install-butler's installed-versions.json) regardless of drift state — the
// server computes drift authoritatively against the signed manifest.
type installedVersionsRequest struct {
	HelperDeviceID string                     `json:"helper_device_id"`
	Installed      []installedVersionsEntry   `json:"installed"`
}

type installedVersionsEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// handleInstalledVersions — #999. Helper POSTs installed snapshot; server:
//   1. authenticates via helper Bearer credential + device id match
//   2. loads the live manifest (LoadManifestEntries, three-tier ops fallback)
//   3. computes drift: for each manifest entry, if its Version != installed
//      version, record a drift entry with normalized class
//   4. persists snapshot via RecordUpdatesAvailable
//   5. returns the drift list so helper can log per-class prompt events
//
// Idempotent (latest-wins). Empty installed list is valid (fresh helper that
// hasn't installed anything yet) — drift list is then "every manifest entry
// reported as new install opportunity" with current_version="" + class as
// configured. The helper-side logger uses class to decide prompt visibility.
func (h *HelperEnrollmentHandler) handleInstalledVersions(w http.ResponseWriter, r *http.Request) {
	credential, ok := helperCredentialFromRequest(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req installedVersionsRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.HelperDeviceID) == "" {
		writeJSONError(w, http.StatusBadRequest, "helper_device_id is required")
		return
	}

	installedByID := make(map[string]string, len(req.Installed))
	for _, e := range req.Installed {
		id := strings.TrimSpace(e.ID)
		if id == "" {
			continue
		}
		installedByID[id] = strings.TrimSpace(e.Version)
	}

	manifestEntries := LoadManifestEntries(h.Logger)
	drift := make([]datalayer.HelperEnrollmentUpdateAvailable, 0, len(manifestEntries))
	for _, m := range manifestEntries {
		installed, present := installedByID[m.ID]
		// Drift conditions: not installed yet (treat as available update
		// opportunity) OR installed version differs from manifest version.
		// Equal versions are NOT drift (and not reported back).
		if present && installed == m.Version {
			continue
		}
		drift = append(drift, datalayer.HelperEnrollmentUpdateAvailable{
			PluginID:        m.ID,
			CurrentVersion:  installed, // empty when not installed
			ManifestVersion: m.Version,
			Class:           NormalizeUpdateClass(m.Class),
		})
	}

	row, err := h.Repo.RecordUpdatesAvailable(r.Context(), r.PathValue("enrollmentId"), credential, req.HelperDeviceID, drift, h.now())
	if err != nil {
		h.writeHelperError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"enrollment":           h.serialize(row),
		"updates_available":    serializeUpdatesAvailable(drift),
		"last_update_check_at": h.now().UnixMilli(),
	})
}

func serializeUpdatesAvailable(items []datalayer.HelperEnrollmentUpdateAvailable) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"plugin_id":        it.PluginID,
			"current_version":  it.CurrentVersion,
			"manifest_version": it.ManifestVersion,
			"class":            it.Class,
		})
	}
	return out
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
	// #999 update-detection projection. Always present in the output (empty
	// slice when no drift / no check yet) so UI consumers don't need to
	// distinguish "missing key" from "no updates"; last_update_check_at is
	// nullable so UI can show "never checked" when helper hasn't reported.
	out["updates_available"] = serializeUpdatesAvailable(row.UpdatesAvailable)
	if row.LastUpdateCheckAt != nil {
		out["last_update_check_at"] = *row.LastUpdateCheckAt
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
