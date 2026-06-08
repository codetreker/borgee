package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"borgee-server/internal/datalayer"
	"borgee-server/internal/store"
)

type RemoteProxy interface {
	IsNodeOnline(nodeID string) bool
	ProxyRequest(nodeID string, action string, path string) (json.RawMessage, error)
}

type RemoteHandler struct {
	Store *store.Store
	// DataLayer — DL-1.2 SSOT 4-interface bundle (nil-safe; see UserHandler).
	DataLayer *datalayer.DataLayer
	Logger    *slog.Logger
	Hub       RemoteProxy
}

func (h *RemoteHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }

	mux.Handle("GET /api/v1/remote/nodes", wrap(h.handleListNodes))
	mux.Handle("POST /api/v1/remote/nodes", wrap(h.handleCreateNode))
	mux.Handle("DELETE /api/v1/remote/nodes/{id}", wrap(h.handleDeleteNode))
	mux.Handle("GET /api/v1/remote/nodes/{nodeId}/bindings", wrap(h.handleListBindings))
	mux.Handle("POST /api/v1/remote/nodes/{nodeId}/bindings", wrap(h.handleCreateBinding))
	mux.Handle("DELETE /api/v1/remote/nodes/{nodeId}/bindings/{id}", wrap(h.handleDeleteBinding))
	mux.Handle("GET /api/v1/channels/{channelId}/remote-bindings", wrap(h.handleListChannelBindings))
	mux.Handle("GET /api/v1/remote/nodes/{nodeId}/status", wrap(h.handleNodeStatus))
	mux.Handle("GET /api/v1/remote/nodes/{nodeId}/ls", wrap(h.handleNodeLs))
	mux.Handle("GET /api/v1/remote/nodes/{nodeId}/read", wrap(h.handleNodeRead))
	mux.Handle("GET /api/v1/remote/nodes/{nodeId}/stat", wrap(h.handleNodeStat))
}

func (h *RemoteHandler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	nodes, err := h.Store.ListRemoteNodes(user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list nodes")
		return
	}
	if nodes == nil {
		nodes = []store.RemoteNode{}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"nodes": nodes})
}

func (h *RemoteHandler) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var body struct {
		MachineName string `json:"machine_name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.MachineName == "" {
		writeJSONError(w, http.StatusBadRequest, "machine_name is required")
		return
	}

	node, err := h.Store.CreateRemoteNode(user.ID, body.MachineName)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create node")
		return
	}

	writeJSONResponse(w, http.StatusCreated, map[string]any{"node": node})
}

func (h *RemoteHandler) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	node, err := h.Store.GetRemoteNode(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Node not found")
		return
	}
	if node.UserID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	if err := h.Store.DeleteRemoteNode(id); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete node")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *RemoteHandler) handleListBindings(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	nodeID := r.PathValue("nodeId")
	node, err := h.Store.GetRemoteNode(nodeID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Node not found")
		return
	}
	if node.UserID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	bindings, err := h.Store.ListRemoteBindings(nodeID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list bindings")
		return
	}
	if bindings == nil {
		bindings = []store.RemoteBinding{}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"bindings": bindings})
}

func (h *RemoteHandler) handleCreateBinding(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	nodeID := r.PathValue("nodeId")
	node, err := h.Store.GetRemoteNode(nodeID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Node not found")
		return
	}
	if node.UserID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var body struct {
		ChannelID string `json:"channel_id"`
		Path      string `json:"path"`
		Label     string `json:"label"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.ChannelID == "" || body.Path == "" {
		writeJSONError(w, http.StatusBadRequest, "channel_id and path are required")
		return
	}

	binding, err := h.Store.CreateRemoteBinding(nodeID, body.ChannelID, body.Path, body.Label)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create binding")
		return
	}

	writeJSONResponse(w, http.StatusCreated, map[string]any{"binding": binding})
}

func (h *RemoteHandler) handleDeleteBinding(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	nodeID := r.PathValue("nodeId")
	node, err := h.Store.GetRemoteNode(nodeID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Node not found")
		return
	}
	if node.UserID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	bindingID := r.PathValue("id")
	if err := h.Store.DeleteRemoteBinding(bindingID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete binding")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *RemoteHandler) handleListChannelBindings(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	bindings, err := h.Store.ListChannelRemoteBindings(channelID, user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list bindings")
		return
	}
	if bindings == nil {
		bindings = []store.RemoteBinding{}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"bindings": bindings})
}

func (h *RemoteHandler) handleNodeStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	nodeID := r.PathValue("nodeId")
	node, err := h.Store.GetRemoteNode(nodeID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Node not found")
		return
	}
	if node.UserID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	online := h.Hub != nil && h.Hub.IsNodeOnline(nodeID)
	writeJSONResponse(w, http.StatusOK, map[string]any{"online": online})
}

func (h *RemoteHandler) handleNodeLs(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	nodeID := r.PathValue("nodeId")
	node, err := h.Store.GetRemoteNode(nodeID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Node not found")
		return
	}
	if node.UserID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	if h.Hub == nil || !h.Hub.IsNodeOnline(nodeID) {
		writeJSONError(w, http.StatusServiceUnavailable, "node_offline")
		return
	}

	path := r.URL.Query().Get("path")
	resp, err := h.Hub.ProxyRequest(nodeID, "ls", path)
	if err != nil {
		if err == context.DeadlineExceeded {
			writeJSONError(w, http.StatusGatewayTimeout, "timeout")
			return
		}
		writeJSONError(w, http.StatusBadGateway, "Remote request failed")
		return
	}

	writeRemoteResponse(w, resp)
}

func (h *RemoteHandler) handleNodeRead(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	nodeID := r.PathValue("nodeId")
	node, err := h.Store.GetRemoteNode(nodeID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Node not found")
		return
	}
	if node.UserID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	if h.Hub == nil || !h.Hub.IsNodeOnline(nodeID) {
		writeJSONError(w, http.StatusServiceUnavailable, "node_offline")
		return
	}

	path := r.URL.Query().Get("path")
	resp, err := h.Hub.ProxyRequest(nodeID, "read", path)
	if err != nil {
		if err == context.DeadlineExceeded {
			writeJSONError(w, http.StatusGatewayTimeout, "timeout")
			return
		}
		writeJSONError(w, http.StatusBadGateway, "Remote request failed")
		return
	}

	writeRemoteResponse(w, resp)
}

func (h *RemoteHandler) handleNodeStat(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	nodeID := r.PathValue("nodeId")
	node, err := h.Store.GetRemoteNode(nodeID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Node not found")
		return
	}
	if node.UserID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	if h.Hub == nil || !h.Hub.IsNodeOnline(nodeID) {
		writeJSONError(w, http.StatusServiceUnavailable, "node_offline")
		return
	}

	path := r.URL.Query().Get("path")
	resp, err := h.Hub.ProxyRequest(nodeID, "stat", path)
	if err != nil {
		if err == context.DeadlineExceeded {
			writeJSONError(w, http.StatusGatewayTimeout, "timeout")
			return
		}
		writeJSONError(w, http.StatusBadGateway, "Remote request failed")
		return
	}

	writeRemoteResponse(w, resp)
}

func writeRemoteResponse(w http.ResponseWriter, resp json.RawMessage) {
	var parsed struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp, &parsed); err == nil && parsed.Error != "" {
		switch parsed.Error {
		case "path_not_allowed":
			writeJSONError(w, http.StatusForbidden, parsed.Error)
		case "file_not_found":
			writeJSONError(w, http.StatusNotFound, parsed.Error)
		case "file_too_large":
			writeJSONError(w, http.StatusRequestEntityTooLarge, parsed.Error)
		case "timeout":
			writeJSONError(w, http.StatusGatewayTimeout, parsed.Error)
		default:
			writeJSONError(w, http.StatusBadGateway, parsed.Error)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}
