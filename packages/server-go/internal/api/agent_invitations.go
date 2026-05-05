// Package api — agent_invitations.go: CM-4.1 HTTP handler for agent
// invitations (cross-org: requester invites someone else's agent into a
// channel; agent's owner approves/rejects).
//
// Blueprint: concept-model.md §4.2 流程 B (default).
//
// Boundaries (per team-lead 04/28):
//   - HTTP only. NO BPP frame (CM-4.3), NO client UI (CM-4.2),
//     NO offline detection / system message (CM-4.3b).
//   - State machine reused from CM-4.0 (store.AgentInvitation.Transition).
//   - Sanitizer hand-built. We never marshal *store.AgentInvitation
//     directly (admin-sanitizer pattern).
//   - Create handler MUST set inv.State = AgentInvitationPending explicitly
//     (no GORM default reliance — 飞马 review flag #2).
//   - (state, expires_at) composite index intentionally NOT added here —
//     deferred for the future sweep handler (飞马 review flag #3).
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"borgee-server/internal/store"
	"borgee-server/internal/ws"


	"borgee-server/internal/idgen"
	"gorm.io/gorm"
)

type AgentInvitationHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	// Hub is the RT-0 (#40) push surface. Wired with *ws.Hub in
	// production (see internal/server/server.go); tests inject a
	// fake to assert frame shape + recipient. nil-safe: a handler
	// without Hub silently skips the push (the persisted row is the
	// source of truth and the client falls back to bell-poll).
	Hub AgentInvitationPusher
	// Now is injected so tests can stamp deterministic decided_at /
	// created_at values (Phase 1 testutil/clock convention).
	Now func() time.Time
}

// AgentInvitationPusher is the RT-0 (#40) seam between the handler
// and *ws.Hub. Defined here so the api package does not import the
// ws package directly (mirrors the existing `hubPluginAdapter`
// pattern in internal/server/server.go).
//
// The two methods are typed —编译期 schema 锁 per
// docs/qa/rt-0-server-review-prep.md §S2 + 拒收红线 (no `interface{}`
// payload — a typo must fail `go build`, not run silently).
// The frames are *ws.AgentInvitationPendingFrame /
// *ws.AgentInvitationDecidedFrame (internal/ws/event_schemas.go);
// marshalled as-is so wire layout matches the BPP frame schema
// byte-for-byte (CI lint enforces parity with PR #218 client TS).
type AgentInvitationPusher interface {
	PushAgentInvitationPending(userID string, frame *ws.AgentInvitationPendingFrame)
	PushAgentInvitationDecided(userID string, frame *ws.AgentInvitationDecidedFrame)
}

func (h *AgentInvitationHandler) pushPending(userID string, frame *ws.AgentInvitationPendingFrame) {
	if h.Hub == nil || userID == "" || frame == nil {
		return
	}
	h.Hub.PushAgentInvitationPending(userID, frame)
}

func (h *AgentInvitationHandler) pushDecided(userID string, frame *ws.AgentInvitationDecidedFrame) {
	if h.Hub == nil || userID == "" || frame == nil {
		return
	}
	h.Hub.PushAgentInvitationDecided(userID, frame)
}

func (h *AgentInvitationHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *AgentInvitationHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }

	mux.Handle("POST /api/v1/agent_invitations", wrap(h.handleCreate))
	mux.Handle("GET /api/v1/agent_invitations", wrap(h.handleList))
	mux.Handle("GET /api/v1/agent_invitations/{id}", wrap(h.handleGet))
	mux.Handle("PATCH /api/v1/agent_invitations/{id}", wrap(h.handlePatch))
}

// sanitizeAgentInvitation hand-builds the response payload — never marshal
// *store.AgentInvitation directly (admin sanitizer pattern, 飞马 review flag #1).
//
// Bug-029 P0 (CM-4 闸 4 野马 not-signed): UI used to render raw UUID for
// agent_id / channel_id / requested_by; blueprint concept-model §1.1 + §1.2
// hardline says owners see names. We now resolve and ship `agent_name`,
// `channel_name`, `requester_name` alongside the IDs so the client can render
// names with the IDs kept only as a `title` hover for a11y / debugging.
//
// Lookups are best-effort: if a referenced row is gone (deleted user / channel),
// we ship the field as the empty string and let the client fall back to the
// ID. Errors during lookup are logged-and-swallowed to avoid bringing down
// the list endpoint over a single missing FK.
func sanitizeAgentInvitation(s *store.Store, inv *store.AgentInvitation) map[string]any {
	m := map[string]any{
		"id":           inv.ID,
		"channel_id":   inv.ChannelID,
		"agent_id":     inv.AgentID,
		"requested_by": inv.RequestedBy,
		"state":        string(inv.State),
		"created_at":   inv.CreatedAt,
	}
	if inv.DecidedAt != nil {
		m["decided_at"] = *inv.DecidedAt
	}
	if inv.ExpiresAt != nil {
		m["expires_at"] = *inv.ExpiresAt
	}

	// Resolve display names. Best-effort; on lookup failure we leave the
	// field as "" — the client must accept the empty case and fall back
	// to the raw ID. Tests assert both branches.
	if s != nil {
		if u, err := s.GetUserByID(inv.AgentID); err == nil && u != nil {
			m["agent_name"] = u.DisplayName
		} else {
			m["agent_name"] = ""
		}
		if c, err := s.GetChannelByID(inv.ChannelID); err == nil && c != nil {
			m["channel_name"] = c.Name
		} else {
			m["channel_name"] = ""
		}
		if u, err := s.GetUserByID(inv.RequestedBy); err == nil && u != nil {
			m["requester_name"] = u.DisplayName
		} else {
			m["requester_name"] = ""
		}
	} else {
		// Defensive — should never hit in the wired-up handler, but keep the
		// fields present so the client schema is stable.
		m["agent_name"] = ""
		m["channel_name"] = ""
		m["requester_name"] = ""
	}
	return m
}

// handleCreate — POST /api/v1/agent_invitations
//
// Body: { channel_id, agent_id, expires_at? (epoch ms) }
//
// Caller MUST be a member of the target channel (only members can invite
// others into the channel they sit in). Agent must exist; we don't check
// here that the agent is in another org (cross-org constraint), since v0
// the org column is mostly empty — CM-3 will tighten this. Already-pending
// invitation for the same (channel, agent) returns 409.
func (h *AgentInvitationHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var body struct {
		ChannelID string `json:"channel_id"`
		AgentID   string `json:"agent_id"`
		ExpiresAt *int64 `json:"expires_at,omitempty"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.ChannelID == "" {
		writeJSONError(w, http.StatusBadRequest, "channel_id is required")
		return
	}
	if body.AgentID == "" {
		writeJSONError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	if _, err := h.Store.GetChannelByID(body.ChannelID); err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	if !h.Store.IsChannelMember(body.ChannelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Not a channel member")
		return
	}

	agent, err := h.Store.GetAgent(body.AgentID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return
	}
	if agent.OwnerID == nil {
		writeJSONError(w, http.StatusBadRequest, "Agent has no owner")
		return
	}
	// Agent already in channel — no point inviting.
	if h.Store.IsChannelMember(body.ChannelID, agent.ID) {
		writeJSONError(w, http.StatusConflict, "Agent already in channel")
		return
	}

	inv := &store.AgentInvitation{
		ID:          idgen.NewID(),
		ChannelID:   body.ChannelID,
		AgentID:     agent.ID,
		RequestedBy: user.ID,
		// Explicit pending init — do not rely on GORM column default.
		State:     store.AgentInvitationPending,
		CreatedAt: h.now().UnixMilli(),
		ExpiresAt: body.ExpiresAt,
	}
	if err := h.Store.CreateAgentInvitation(inv); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create invitation")
		return
	}

	// RT-0 (#40): push agent_invitation_pending to the agent's owner so
	// the bell badge updates ≤ 3s without polling. Field order locked
	// to docs/blueprint/current/realtime.md §2.3 + PR #218 client TS interface.
	expiresMs := int64(0)
	if inv.ExpiresAt != nil {
		expiresMs = *inv.ExpiresAt
	}
	h.pushPending(*agent.OwnerID, &ws.AgentInvitationPendingFrame{
		Type:            ws.FrameTypeAgentInvitationPending,
		InvitationID:    inv.ID,
		RequesterUserID: inv.RequestedBy,
		AgentID:         inv.AgentID,
		ChannelID:       inv.ChannelID,
		CreatedAt:       inv.CreatedAt,
		ExpiresAt:       expiresMs,
	})

	writeJSONResponse(w, http.StatusCreated, map[string]any{
		"invitation": sanitizeAgentInvitation(h.Store, inv),
	})
}

// handleGet — GET /api/v1/agent_invitations/{id}
//
// Visible to: the requester, or the agent's owner.
func (h *AgentInvitationHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	inv, err := h.Store.GetAgentInvitation(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Invitation not found")
		return
	}

	if !h.canSee(user, inv) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"invitation": sanitizeAgentInvitation(h.Store, inv),
	})
}

// handleList — GET /api/v1/agent_invitations[?role=owner|requester]
//
// role=owner   → invitations addressed to agents the caller owns (default).
// role=requester → invitations the caller created.
// Admins listing as owner also see all-agent invitations.
func (h *AgentInvitationHandler) handleList(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	role := r.URL.Query().Get("role")
	if role == "" {
		role = "owner"
	}

	var invs []store.AgentInvitation
	var err error
	switch role {
	case "owner":
		// ADM-0.3: user-rail lists owned agents only.
		var agents []store.User
		agents, err = h.Store.ListAgentsByOwner(user.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to list agents")
			return
		}
		ids := make([]string, len(agents))
		for i, a := range agents {
			ids[i] = a.ID
		}
		invs, err = h.Store.ListAgentInvitationsForAgents(ids)
	case "requester":
		invs, err = h.Store.ListAgentInvitationsByRequester(user.ID)
	default:
		writeJSONError(w, http.StatusBadRequest, "role must be 'owner' or 'requester'")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list invitations")
		return
	}

	out := make([]map[string]any, len(invs))
	for i := range invs {
		out[i] = sanitizeAgentInvitation(h.Store, &invs[i])
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"invitations": out})
}

// handlePatch — PATCH /api/v1/agent_invitations/{id}
//
// Body: { state: "approved" | "rejected" }
//
// Only the agent's owner may transition. Uses the CM-4.0 state machine
// (inv.Transition) — illegal transitions return 409. On approval the
// agent is added to the channel as a side-effect (the whole point of the
// invite). DecidedAt is stamped by Transition using the injected clock.
func (h *AgentInvitationHandler) handlePatch(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	inv, err := h.Store.GetAgentInvitation(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Invitation not found")
		return
	}

	agent, err := h.Store.GetAgent(inv.AgentID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return
	}
	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Only the agent owner may decide")
		return
	}

	var body struct {
		State string `json:"state"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	target := store.AgentInvitationState(body.State)
	switch target {
	case store.AgentInvitationApproved, store.AgentInvitationRejected:
		// allowed targets via owner action; expired is sweep-only
	default:
		writeJSONError(w, http.StatusBadRequest, "state must be 'approved' or 'rejected'")
		return
	}

	if err := inv.Transition(target, h.now().UnixMilli()); err != nil {
		if errors.Is(err, store.ErrInvalidTransition) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.Store.UpdateAgentInvitationState(inv); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSONError(w, http.StatusNotFound, "Invitation not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to update invitation")
		return
	}

	if inv.State == store.AgentInvitationApproved {
		// Side-effect: agent joins the channel. Idempotent (FirstOrCreate).
		if err := h.Store.AddChannelMember(&store.ChannelMember{
			ChannelID: inv.ChannelID,
			UserID:    inv.AgentID,
			JoinedAt:  h.now().UnixMilli(),
		}); err != nil {
			h.Logger.Error("approved invitation: add channel member failed",
				"invitation_id", inv.ID, "err", err)
			// Don't unwind state — the persisted decision is the source of
			// truth. A retry / sweep can reconcile membership.
		}
	}

	// RT-0 (#40): push agent_invitation_decided to BOTH parties (the
	// requester and the agent's owner) so every open tab updates
	// without polling. Multi-device parity per realtime.md §1.4.
	decidedAt := int64(0)
	if inv.DecidedAt != nil {
		decidedAt = *inv.DecidedAt
	}
	frame := &ws.AgentInvitationDecidedFrame{
		Type:         ws.FrameTypeAgentInvitationDecided,
		InvitationID: inv.ID,
		State:        string(inv.State),
		DecidedAt:    decidedAt,
	}
	h.pushDecided(inv.RequestedBy, frame)
	if agent.OwnerID != nil {
		h.pushDecided(*agent.OwnerID, frame)
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"invitation": sanitizeAgentInvitation(h.Store, inv),
	})
}

// canSee reports whether `user` may read `inv`. Requester or agent owner
// only on the user-rail (ADM-0.3 removed the role='admin' shortcut; admin
// observation lives on /admin-api/v1/agent-invitations).
func (h *AgentInvitationHandler) canSee(user *store.User, inv *store.AgentInvitation) bool {
	if inv.RequestedBy == user.ID {
		return true
	}
	agent, err := h.Store.GetAgent(inv.AgentID)
	if err != nil {
		return false
	}
	return agent.OwnerID != nil && *agent.OwnerID == user.ID
}
