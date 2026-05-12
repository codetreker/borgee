// Package api — admin_grant_check.go: ADM-2-FOLLOWUP REG-010 grant validation
// helper. Before an admin write action, the admin must hold an active
// ImpersonationGrant; otherwise this helper returns 403 with
// reason="impersonate.no_grant", matching the existing ADM-2 five-template
// wording.
//
// Design (adm-2-followup-stance §1):
//   - All 5 admin write actions (force_delete_channel / patch disabled /
//     patch password / patch role / start_impersonation) are wired through this
//     grant validation.
//   - Failure literal `impersonate.no_grant` stays byte-identical with ADM-2.
//   - Admin path remains separate from the user rail (ADM-0 §1.3).
//
// Grep checks require `RequireImpersonationGrant` in all 5 admin write handlers.
package api

import (
	"net/http"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"
)

// RequireImpersonationGrant validates the admin context and active
// impersonation grant for targetUserID. It returns (true, admin) when the grant
// is valid and the caller may continue. It returns (false, _) after writing the
// rejection response when the grant is missing or expired.
//
// REG-ADM2-010 wiring: all 5 admin write handlers call this helper.
func RequireImpersonationGrant(w http.ResponseWriter, r *http.Request, s *store.Store, targetUserID string) (bool, *admin.Admin) {
	a := admin.AdminFromContext(r.Context())
	if a == nil {
		writeJSONErrorCode(w, http.StatusUnauthorized, "impersonate.no_admin",
			"admin context required")
		return false, nil
	}
	if targetUserID == "" {
		writeJSONErrorCode(w, http.StatusBadRequest, "impersonate.no_target",
			"target user required for grant check")
		return false, nil
	}
	g, err := s.ActiveImpersonationGrant(targetUserID)
	if err != nil || g == nil {
		writeJSONErrorCode(w, http.StatusForbidden, "impersonate.no_grant",
			"target user has no active impersonation grant; admin write rejected")
		return false, nil
	}
	return true, a
}
