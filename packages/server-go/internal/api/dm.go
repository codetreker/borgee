package api

import (
	"log/slog"
	"net/http"

	agentpkg "borgee-server/internal/agent"
	"borgee-server/internal/config"
	"borgee-server/internal/store"
)

type DmHandler struct {
	Store  *store.Store
	Config *config.Config
	Logger *slog.Logger
	State  AgentRuntimeProvider
}

func (h *DmHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }

	mux.Handle("POST /api/v1/dm/{userId}", wrap(h.handleCreateDm))
	mux.Handle("GET /api/v1/dm", wrap(h.handleListDms))
}

func (h *DmHandler) handleCreateDm(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	targetID := r.PathValue("userId")
	if targetID == user.ID {
		writeJSONError(w, http.StatusBadRequest, "Cannot create DM with yourself")
		return
	}

	target, err := h.Store.GetUserByID(targetID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	ch, err := h.Store.CreateDmChannel(user.ID, targetID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create DM channel")
		return
	}

	peer := h.withPeerState(store.DmPeer{
		ID:          target.ID,
		DisplayName: target.DisplayName,
		AvatarURL:   target.AvatarURL,
		Role:        target.Role,
		Disabled:    target.Disabled,
	})

	writeJSONResponse(w, http.StatusOK, map[string]any{"channel": ch, "peer": peer})
}

func (h *DmHandler) withPeerState(peer store.DmPeer) store.DmPeer {
	if peer.Role != "agent" {
		return peer
	}
	if peer.Disabled || h.State == nil {
		peer.State = string(agentpkg.StateOffline)
		peer.Reason = ""
		peer.StateUpdatedAt = 0
		return peer
	}
	snap := h.State.ResolveAgentState(peer.ID)
	peer.State = string(snap.State)
	peer.Reason = snap.Reason
	peer.StateUpdatedAt = snap.UpdatedAt
	if peer.State != string(agentpkg.StateError) {
		peer.Reason = ""
	}
	return peer
}

func (h *DmHandler) handleListDms(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channels, err := h.Store.ListDmChannelsForUser(user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list DM channels")
		return
	}
	if channels == nil {
		channels = []store.DmChannelInfo{}
	}
	for i := range channels {
		channels[i].Peer = h.withPeerState(channels[i].Peer)
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"channels": channels})
}
