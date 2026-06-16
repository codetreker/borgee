// Package api — admin_retention_helper.go: REFACTOR-1 helper-3 SSOT
// admin retention override skeleton shared between AL-7 / HB-5.
//
// 设计 ① + ② (refactor-1-spec.md §0):
//   - 行为不变量 byte-identical pre/post refactor: status / error / clamp
//     boundary / SystemActorID 默认 / Logger 行为 / InsertAdminAction 调
//     用 byte-identical 跟 al_7 + hb_5 既有 45 行 × 2 skeleton.
//   - metadata.target 字面区分仍归 caller 决定 — al_7 (target unset) /
//     hb_5 (auth.HeartbeatTargetLabel) — 通过 extraMeta map 传入.
//
// Caller list 锁:
//   - al_7_audit_retention_override.go (action=ActionAuditRetentionOverride,
//     extraMeta=nil — al-7-spec.md §0 设计 ②③)
//   - hb_5_heartbeat_retention_override.go (复用 ActionAuditRetentionOverride,
//     extraMeta={"target": auth.HeartbeatTargetLabel} — hb-5-spec.md §0 设计 ②)
//
// Reverse-grep 锚 (refactor-1-spec.md §2 反约束 #6):
//   - func writeRetentionOverride ==1 hit (此文件)
//   - writeRetentionOverride( ≥2 hit (al_7 + hb_5 各 1 调)

package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"borgee-server/internal/admin"
	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// retentionOverrideRequest — al_7 / hb_5 同型 (RetentionDays + 可选
// TargetUserID). Single-source 跟 al7OverrideRequest / hb5OverrideRequest
// byte-identical (REFACTOR-1 R1.2 SSOT, 反 caller 各自定义 type).
type retentionOverrideRequest struct {
	RetentionDays int    `json:"retention_days"`
	TargetUserID  string `json:"target_user_id"` // optional — defaults to "system"
}

// writeRetentionOverride runs the 5-step admin retention skeleton
// (admin nil 401 → JSON decode → clamp → InsertAdminAction → response)
// and writes the response. Returns false if any path failed (caller
// MUST early-return without writing).
//
// 设计 ⑥ clamp 1..365 (auth.RetentionMin/MaxDays 单源, 跟 AL-7 #533 + HB-5
// 同源). target 默认 auth.SystemActorID. extraMeta 字段 merge 入 metadata
// JSON (hb_5 用之注入 target='heartbeat' 字面区分).
//
// action / responseExtra 留 caller 决定 — al_7 + hb_5 共享 const
// auth.ActionAuditRetentionOverride (HB-5 设计 ② 复用 不挂第 13 enum).
func writeRetentionOverride(
	w http.ResponseWriter,
	r *http.Request,
	store *store.Store,
	logger *slog.Logger,
	logTag string,
	action string,
	extraMeta map[string]any,
	responseExtra map[string]any,
) bool {
	a := admin.AdminFromContext(r.Context())
	if a == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return false
	}
	var req retentionOverrideRequest
	capJSONBody(w, r)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if isBodyTooLarge(err) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			return false
		}
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	// 设计 ⑥ clamp 1..365 — 反 0 / 负 / 非数 / >365 reject. 0 = ZeroValue
	// → reject (Go decoder defaults missing field to 0 — admin 必显式填).
	if req.RetentionDays < auth.RetentionMinDays || req.RetentionDays > auth.RetentionMaxDays {
		writeJSONError(w, http.StatusBadRequest, "retention_days must be in [1, 365]")
		return false
	}
	target := req.TargetUserID
	if target == "" {
		target = auth.SystemActorID
	}
	metaMap := map[string]any{
		"retention_days": req.RetentionDays,
		"override_by":    a.ID,
	}
	for k, v := range extraMeta {
		metaMap[k] = v
	}
	meta, err := json.Marshal(metaMap)
	if err != nil {
		if logger != nil {
			logger.Error(logTag+" marshal", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return false
	}
	if _, err := store.InsertAdminAction(a.ID, target, action, string(meta)); err != nil {
		if logger != nil {
			logger.Error(logTag+" audit insert", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to write audit")
		return false
	}
	resp := map[string]any{
		"retention_days": req.RetentionDays,
		"recorded":       true,
	}
	for k, v := range responseExtra {
		resp[k] = v
	}
	writeJSONResponse(w, http.StatusOK, resp)
	return true
}
