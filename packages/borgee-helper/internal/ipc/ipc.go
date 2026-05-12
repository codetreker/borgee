// Package ipc implements the HB-2 IPC server: JSON-line request/response with
// request_id multiplexing on a single connection. The cmd layer selects the
// platform transport (UDS or Named Pipe); this package provides protocol
// parsing and handler wiring that stays byte-identical across platforms.
//
// hb-2-spec.md §3.1 IPC contract + §5.5 sandbox build tags keep platforms distinct.
package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"

	"borgee-helper/internal/acl"
	"borgee-helper/internal/audit"
	"borgee-helper/internal/fileio"
	"borgee-helper/internal/reasons"
)

// Request is the plugin-to-host-bridge wire format (hb-2-spec.md §3.1).
type Request struct {
	RequestID string                 `json:"request_id"`
	Action    string                 `json:"action"`
	AgentID   string                 `json:"agent_id"`
	Params    map[string]interface{} `json:"params"`
}

// Response is the host-bridge-to-plugin wire format.
type Response struct {
	RequestID   string      `json:"request_id"`
	Status      string      `json:"status"` // "ok" | "rejected" | "failed"
	Reason      string      `json:"reason"`
	Data        interface{} `json:"data,omitempty"`
	AuditLogID  string      `json:"audit_log_id,omitempty"`
}

// Handler processes one connection: handshake with agent_id in the first
// message, then a multiplexed request loop.
type Handler struct {
	Gate    *acl.Gate
	Audit   *audit.Logger
}

// New constructs a Handler.
func New(g *acl.Gate, a *audit.Logger) *Handler {
	return &Handler{Gate: g, Audit: a}
}

// Serve owns one net.Conn and runs the JSON-line protocol until EOF or error.
// The first line is handshake {agent_id}; later lines are the Request stream.
func (h *Handler) Serve(ctx context.Context, conn net.Conn) error {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	defer w.Flush()

	handshakeAgentID, err := h.readHandshake(r)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line, err := r.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			h.writeResp(w, Response{Status: "failed", Reason: string(reasons.IOFailed)})
			continue
		}
		resp := h.handle(ctx, handshakeAgentID, req)
		if err := h.writeResp(w, resp); err != nil {
			return err
		}
	}
}

func (h *Handler) readHandshake(r *bufio.Reader) (string, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return "", err
	}
	var hs struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(line, &hs); err != nil {
		return "", err
	}
	if hs.AgentID == "" {
		return "", errors.New("handshake missing agent_id")
	}
	return hs.AgentID, nil
}

func (h *Handler) writeResp(w *bufio.Writer, resp Response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return w.Flush()
}

// handle applies the ACL gate and audit logging, including rejected requests,
// then returns a Response.
//
// v0(D) real IO: after ACL passes, read_file / list_files actions use the
// fileio package (os.ReadFile / os.ReadDir); Landlock already limits paths.
func (h *Handler) handle(ctx context.Context, handshakeAgentID string, req Request) Response {
	target := extractTarget(req)
	d := h.Gate.Decide(ctx, handshakeAgentID, req.AgentID, acl.Action(req.Action), target)
	resp := Response{RequestID: req.RequestID, Reason: string(d.Reason)}
	if d.Allow {
		// v0(D) real IO is enabled, replacing the v0(C) ACL-only decision path.
		switch acl.Action(req.Action) {
		case acl.ActionReadFile:
			maxBytes, _ := req.Params["max_bytes"].(float64)
			data, ioErr := fileio.ReadFile(target, int64(maxBytes))
			if ioErr != nil {
				resp.Status = "rejected"
				resp.Reason = string(reasons.IOFailed)
			} else {
				resp.Status = "ok"
				resp.Data = data
			}
		case acl.ActionListFiles:
			data, ioErr := fileio.ListFiles(target)
			if ioErr != nil {
				resp.Status = "rejected"
				resp.Reason = string(reasons.IOFailed)
			} else {
				resp.Status = "ok"
				resp.Data = data
			}
		case acl.ActionNetworkEgress:
			// v0(D) only applies the ACL gate; real outbound proxy remains v1.5+ (spec §3 out of scope).
			resp.Status = "ok"
		default:
			resp.Status = "ok"
		}
	} else {
		resp.Status = "rejected"
	}
	// Audit every request, including rejects; the 5-field schema has one source.
	if h.Audit != nil {
		_ = h.Audit.Write(audit.Event{
			Actor:  req.AgentID,
			Action: req.Action,
			Target: target,
			Scope:  d.Scope,
		})
	}
	return resp
}

func extractTarget(req Request) string {
	if req.Params == nil {
		return ""
	}
	if v, ok := req.Params["path"].(string); ok {
		return v
	}
	if v, ok := req.Params["url"].(string); ok {
		return v
	}
	return ""
}
