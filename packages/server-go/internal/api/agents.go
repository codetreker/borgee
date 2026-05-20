package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	agentpkg "borgee-server/internal/agent"
	"borgee-server/internal/datalayer"
	"borgee-server/internal/store"
)

// flexPermissions accepts both string array ["perm"] and object array [{permission:"perm",scope:"s"}].
type permEntry struct {
	Permission string `json:"permission"`
	Scope      string `json:"scope"`
}

type flexPermissions []permEntry

func (fp *flexPermissions) UnmarshalJSON(data []byte) error {
	// Try object array first
	var objs []permEntry
	if err := json.Unmarshal(data, &objs); err == nil {
		*fp = objs
		return nil
	}
	// Try string array
	var strs []string
	if err := json.Unmarshal(data, &strs); err != nil {
		return err
	}
	result := make([]permEntry, len(strs))
	for i, s := range strs {
		result[i] = permEntry{Permission: s}
	}
	*fp = result
	return nil
}

type AgentHandler struct {
	Store *store.Store
	// DataLayer — DL-1.2 SSOT 4-interface bundle (nil-safe; see UserHandler).
	DataLayer *datalayer.DataLayer
	Logger    *slog.Logger
	Hub       AgentFileProxy
	// State — AL-1a (#R3 Phase 2): runtime 三态查询. 提供 online/offline +
	// error 旁路 (蓝图 agent-lifecycle §2.3). nil 时 GET 返回退化 offline.
	State AgentRuntimeProvider
}

type AgentFileProxy interface {
	ProxyPluginRequest(agentID string, method string, path string, body []byte) (int, []byte, error)
}

// AgentRuntimeProvider — server.go 注入的薄壳, 把 hub plugin presence +
// agent.Tracker 错误 map 合并成单次查询. api 包不直接 import internal/ws
// 仅为了 GetPlugin (依赖反转, 测试也好 fake).
type AgentRuntimeProvider interface {
	ResolveAgentState(agentID string) agentpkg.Snapshot
}

// AgentRuntimeSetter — runtime 故障旁路. 实现挂在同一个 server.go adapter
// 上, 但 api 层只在 plugin proxy 出错时 best-effort cast + 调用.
type AgentRuntimeSetter interface {
	SetAgentError(agentID, reason string)
}

func (h *AgentHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }

	mux.Handle("POST /api/v1/agents", wrap(h.handleCreateAgent))
	mux.Handle("GET /api/v1/agents", wrap(h.handleListAgents))
	mux.Handle("GET /api/v1/agents/{id}", wrap(h.handleGetAgent))
	mux.Handle("PATCH /api/v1/agents/{id}", wrap(h.handlePatchAgent))
	mux.Handle("DELETE /api/v1/agents/{id}", wrap(h.handleDeleteAgent))
	mux.Handle("POST /api/v1/agents/{id}/rotate-api-key", wrap(h.handleRotateAPIKey))
	mux.Handle("GET /api/v1/agents/{id}/permissions", wrap(h.handleGetPermissions))
	mux.Handle("PUT /api/v1/agents/{id}/permissions", wrap(h.handleSetPermissions))
	mux.Handle("GET /api/v1/agents/{id}/files", wrap(h.handleGetAgentFiles))
	// AL-1b.2 (#R3 Phase 4) — agent status endpoint (5-state 合并 GET +
	// PATCH 405 reject 设计 ② BPP 单源).
	mux.Handle("GET /api/v1/agents/{id}/status", wrap(h.handleGetAgentStatus))
	mux.Handle("PATCH /api/v1/agents/{id}/status", wrap(h.handleRejectStatusPatch))
}

func sanitizeAgent(u *store.User) map[string]any {
	m := map[string]any{
		"id":              u.ID,
		"display_name":    u.DisplayName,
		"role":            u.Role,
		"avatar_url":      u.AvatarURL,
		"require_mention": u.RequireMention,
		"created_at":      u.CreatedAt,
		"disabled":        u.Disabled,
	}
	if u.OwnerID != nil {
		m["owner_id"] = *u.OwnerID
	}
	if u.APIKey != nil {
		m["api_key"] = *u.APIKey
	}
	return m
}

// withState — AL-1a (#R3): fold state + reason into the JSON dict the
// client expects. Disabled agents always render as offline (蓝图 §2.4
// 禁用 = 停接消息), 不查 runtime presence.
func (h *AgentHandler) withState(m map[string]any, agentID string, disabled bool) map[string]any {
	if disabled {
		m["state"] = string(agentpkg.StateOffline)
		return m
	}
	if h.State == nil {
		m["state"] = string(agentpkg.StateOffline)
		return m
	}
	snap := h.State.ResolveAgentState(agentID)
	m["state"] = string(snap.State)
	if snap.Reason != "" {
		m["reason"] = snap.Reason
	}
	if snap.UpdatedAt != 0 {
		m["state_updated_at"] = snap.UpdatedAt
	}
	return m
}

// classifyAgentProxyError — convenience wrapper over agent.ClassifyProxyError
// scoped to the api package. 单独命名是为了避免和 handler 里 `agent` 局部
// 变量冲突 (handleGetAgentFiles 等).
func classifyAgentProxyError(status int, err error) string {
	return agentpkg.ClassifyProxyError(status, err)
}

func (h *AgentHandler) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var body struct {
		ID          string          `json:"id"`
		DisplayName string          `json:"display_name"`
		AvatarURL   string          `json:"avatar_url"`
		Permissions flexPermissions `json:"permissions"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if body.DisplayName == "" {
		writeJSONError(w, http.StatusBadRequest, "display_name is required")
		return
	}

	apiKey, err := store.GenerateAPIKey()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to generate API key")
		return
	}

	agent := &store.User{
		ID:          body.ID,
		DisplayName: body.DisplayName,
		Role:        "agent",
		AvatarURL:   body.AvatarURL,
		OwnerID:     &user.ID,
		APIKey:      &apiKey,
		// CM-1.2: agent inherits owner's org. Blueprint §1.1: agents are
		// resources that belong to a person's org, not separate orgs.
		OrgID: user.OrgID,
	}

	// DL-1.2: prefer UserRepo.Create (interface seam) when DataLayer wired;
	// fall back to legacy store.CreateUser (nil-safe, byte-identical).
	var createErr error
	if h.DataLayer != nil {
		createErr = h.DataLayer.UserRepo.Create(r.Context(), agent)
	} else {
		createErr = h.Store.CreateUser(agent)
	}
	if createErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create agent")
		return
	}

	h.Store.GrantDefaultPermissions(agent.ID, "agent")

	for _, p := range body.Permissions {
		scope := p.Scope
		if scope == "" {
			scope = "*"
		}
		h.Store.GrantPermission(&store.UserPermission{
			UserID:     agent.ID,
			Permission: p.Permission,
			Scope:      scope,
			GrantedBy:  &user.ID,
		})
	}

	h.Store.AddUserToPublicChannels(agent.ID)

	writeJSONResponse(w, http.StatusCreated, map[string]any{"agent": sanitizeAgent(agent)})
}

func (h *AgentHandler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	// ADM-0.3: user-rail lists own agents only. Cross-owner enumeration is
	// admin-rail (/admin-api/v1/users/:id/agents).
	agents, err := h.Store.ListAgentsByOwner(user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list agents")
		return
	}

	result := make([]map[string]any, len(agents))
	for i, a := range agents {
		result[i] = h.withState(sanitizeAgent(&a), a.ID, a.Disabled)
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"agents": result})
}

func (h *AgentHandler) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	agent, _, ok := loadAgentByPath(w, r, h.Store)
	if !ok {
		return
	}

	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"agent": h.withState(sanitizeAgent(agent), agent.ID, agent.Disabled)})
}

// handlePatchAgent — owner-rail PATCH /api/v1/agents/{id}.
//
// Scope (anti scope-creep): currently only `require_mention` is mutable.
// Other agent fields go through dedicated endpoints (display_name/avatar/
// prompt/model/capabilities/enabled/memory_ref via agent_config PATCH;
// permissions via PUT /permissions). 不顺手开 disabled/role 口子.
//
// Authz (anti-IDOR — pm_stance_must_include_anti_x):
//   - Login required (auth middleware via mustUser).
//   - target.OwnerID == current_user.ID — else 404 (not 403) so we do not
//     leak existence of agents owned by other users. loadAgentByPath already
//     returns 404 for missing rows, so the two paths fold into the same
//     response shape.
//   - body strict-decode (DisallowUnknownFields) — extra fields → 400 so a
//     client can never silently widen the surface area.
//   - require_mention must be bool; non-bool → 400 via JSON type error.
//   - Empty body (no recognized fields) → 400 to avoid no-op writes that
//     look like success.
func (h *AgentHandler) handlePatchAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	agent, id, ok := loadAgentByPath(w, r, h.Store)
	if !ok {
		return
	}

	// Anti-IDOR: collapse "not yours" into the same 404 response as
	// "doesn't exist" — do not leak that the id is real but you cannot
	// touch it.
	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return
	}

	// Strict decode: explicit DisallowUnknownFields rejects extra keys so
	// a future field cannot be smuggled in via this PATCH without an
	// intentional API change. readJSON() is intentionally NOT used here
	// because the shared helper is permissive (no DisallowUnknownFields).
	const maxBytes = 1 << 16
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	body := struct {
		RequireMention *bool `json:"require_mention"`
	}{}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	// Disallow trailing tokens (a JSON stream with extra content).
	if dec.More() {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: unexpected trailing content")
		return
	}

	updates := map[string]any{}
	if body.RequireMention != nil {
		updates["require_mention"] = *body.RequireMention
	}
	if len(updates) == 0 {
		writeJSONError(w, http.StatusBadRequest, "no mutable fields supplied")
		return
	}

	if err := h.Store.UpdateUser(id, updates); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update agent")
		return
	}

	updated, err := h.Store.GetAgent(id)
	if err != nil || updated == nil {
		updated = agent
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"agent": h.withState(sanitizeAgent(updated), updated.ID, updated.Disabled)})
}

func (h *AgentHandler) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	agent, id, ok := loadAgentByPath(w, r, h.Store)
	if !ok {
		return
	}

	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	if err := h.Store.SoftDeleteUser(id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete agent")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AgentHandler) handleRotateAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	agent, id, ok := loadAgentByPath(w, r, h.Store)
	if !ok {
		return
	}

	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	apiKey, err := store.GenerateAPIKey()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to generate API key")
		return
	}

	if err := h.Store.SetAPIKey(id, apiKey); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to set API key")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"api_key": apiKey})
}

func (h *AgentHandler) handleGetPermissions(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	agent, id, ok := loadAgentByPath(w, r, h.Store)
	if !ok {
		return
	}

	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	perms, err := h.Store.ListUserPermissions(id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list permissions")
		return
	}

	permStrs := make([]string, len(perms))
	details := make([]map[string]any, len(perms))
	for i, p := range perms {
		permStrs[i] = p.Permission
		details[i] = map[string]any{
			"permission": p.Permission,
			"scope":      p.Scope,
			"granted_at": p.GrantedAt,
		}
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"agent_id":    agent.ID,
		"permissions": permStrs,
		"details":     details,
	})
}

func (h *AgentHandler) handleSetPermissions(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	agent, id, ok := loadAgentByPath(w, r, h.Store)
	if !ok {
		return
	}

	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var body struct {
		Permissions flexPermissions `json:"permissions"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.Store.DeletePermissionsByUserID(id)

	for _, p := range body.Permissions {
		scope := p.Scope
		if scope == "" {
			scope = "*"
		}
		h.Store.GrantPermission(&store.UserPermission{
			UserID:     id,
			Permission: p.Permission,
			Scope:      scope,
			GrantedBy:  &user.ID,
		})
	}

	perms, _ := h.Store.ListUserPermissions(id)
	permStrs := make([]string, len(perms))
	details := make([]map[string]any, len(perms))
	for i, p := range perms {
		permStrs[i] = p.Permission
		details[i] = map[string]any{
			"permission": p.Permission,
			"scope":      p.Scope,
			"granted_at": p.GrantedAt,
		}
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"agent_id":    agent.ID,
		"permissions": permStrs,
		"details":     details,
	})
}

func (h *AgentHandler) handleGetAgentFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	agent, id, ok := loadAgentByPath(w, r, h.Store)
	if !ok {
		return
	}

	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	if h.Hub == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Agent not connected")
		return
	}

	path := r.URL.Query().Get("path")
	status, body, err := h.Hub.ProxyPluginRequest(id, "read_file", path, nil)
	// AL-1a (#R3): runtime 故障旁路. 任意 plugin 调用失败 / 5xx 会让该
	// agent 进入 error 态, owner 端 Sidebar 立即看到原因码 + 修复入口.
	if reason := classifyAgentProxyError(status, err); reason != "" && h.State != nil {
		if setter, ok := h.State.(AgentRuntimeSetter); ok {
			setter.SetAgentError(id, reason)
		}
	}
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Agent not connected")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}
