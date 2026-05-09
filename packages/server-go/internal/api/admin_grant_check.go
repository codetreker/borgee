// Package api — admin_grant_check.go: ADM-2-FOLLOWUP REG-010 grant 校验
// 守门 helper. admin 写动作前 must hold active ImpersonationGrant; 否则
// 返回 403 + reason="impersonate.no_grant" (跟 ADM-2 既有 5 模板字面承袭).
//
// 设计 (adm-2-followup-stance §1):
//   - 5/5 admin 写动作 (force_delete_channel / patch disabled / patch password
//     / patch role / start_impersonation) 全 wire grant 校验.
//   - 失败字面 byte-identical `impersonate.no_grant` 跟 ADM-2 既有承袭.
//   - admin god-mode 独立路径 (ADM-0 §1.3 红线), 不挂 user-rail.
//
// grep 检查 `RequireImpersonationGrant` 在 5/5 admin write handler 全挂.
package api

import (
	"net/http"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"
)

// RequireImpersonationGrant 校验 admin context + active impersonation grant
// 对 targetUserID. 返回 (true, nil) 表 grant 有效, 调用方继续; (false, _)
// 表 grant 缺失/过期, 已写 403 response, 调用方 return.
//
// REG-ADM2-010 wire — 5/5 admin write handler 全挂此 gate.
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
