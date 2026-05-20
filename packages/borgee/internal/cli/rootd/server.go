//go:build linux || darwin

package rootd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

// Server listens on a Unix domain socket and dispatches incoming JSON
// requests to a hardcoded handler whitelist. The wire protocol is
// line-delimited JSON; one request per line, one response per line, then
// the connection closes.
//
// Request shape:
//
//	{"cmd": "ping", "request_id": "<uuid>", "params": {...}}
//
// Response shape:
//
//	{"request_id": "<echoed>", "ok": true|false,
//	 "result": {...}|null, "error": "msg"}
//
// Unknown `cmd` values are rejected with `ok:false, error:"unknown_command"`.
// The server NEVER executes the cmd field directly — it only ever calls
// Handlers[req.Cmd], so the whitelist is the security boundary.
type Server struct {
	// SocketPath is the UDS path to listen on. Required.
	SocketPath string
	// PeerGroup is the unix group whose members may connect. Connections
	// from peers whose primary gid is not in this group are closed
	// immediately. Empty disables the peer-cred check (tests only).
	PeerGroup string
	// Handlers is the command whitelist. cmd names not present here are
	// rejected with unknown_command before any work runs.
	Handlers map[string]HandlerFunc
	// Logger receives audit-line log records: accepted requests, rejected
	// requests, peer-cred failures. Defaults to no-op if nil.
	Logger func(format string, v ...any)

	// listener is the active UDS listener; held so Close can shut Serve down.
	mu       sync.Mutex
	listener net.Listener
}

// HandlerFunc executes a single whitelisted command. params is the raw
// `params` field of the request envelope (may be empty). The return value
// is marshaled into the response `result` field; a non-nil error becomes
// the response `error` field with ok=false.
type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, error)

// Request and Response are the wire envelopes. Exported so tests + the
// rootdclient can share types.
type Request struct {
	Cmd       string          `json:"cmd"`
	RequestID string          `json:"request_id"`
	Params    json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	RequestID string          `json:"request_id"`
	OK        bool            `json:"ok"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// Serve binds the UDS, applies mode/owner, then runs the Accept loop until
// ctx is canceled. Returns ctx.Err() on graceful shutdown, or the first
// fatal listen/setup error.
func (s *Server) Serve(ctx context.Context) error {
	if s.SocketPath == "" {
		return errors.New("rootd: SocketPath is required")
	}
	if len(s.Handlers) == 0 {
		return errors.New("rootd: no handlers registered (whitelist empty)")
	}

	// Ensure parent dir exists. The systemd unit covers /run/borgee via
	// RuntimeDirectory but the dev / test paths may not, so create
	// best-effort with mode 0755.
	if dir := filepath.Dir(s.SocketPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("rootd: mkdir parent %s: %w", dir, err)
		}
	}

	// Clean up stale socket from a prior unclean shutdown. We do this
	// even though systemd's RuntimeDirectory wipe usually handles it,
	// because dev/test invocations bypass systemd.
	if _, err := os.Stat(s.SocketPath); err == nil {
		if err := os.Remove(s.SocketPath); err != nil {
			return fmt.Errorf("rootd: remove stale socket %s: %w", s.SocketPath, err)
		}
	}

	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return fmt.Errorf("rootd: listen %s: %w", s.SocketPath, err)
	}
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()
	defer func() {
		_ = ln.Close()
		// Best-effort cleanup; ignore ENOENT.
		_ = os.Remove(s.SocketPath)
	}()

	// chmod 0660 + chown root:<PeerGroup> so only members of PeerGroup
	// can connect. The peer-cred check is defense-in-depth on top of
	// these filesystem perms.
	if err := os.Chmod(s.SocketPath, 0o660); err != nil {
		return fmt.Errorf("rootd: chmod %s: %w", s.SocketPath, err)
	}
	if s.PeerGroup != "" {
		if gid, err := lookupGID(s.PeerGroup); err == nil {
			if err := os.Chown(s.SocketPath, 0, gid); err != nil {
				// chown failure is logged but not fatal in dev (e.g.
				// running as non-root in tests); production runs as
				// root so this succeeds.
				s.logf("warn: chown %s root:%s (gid=%d): %v", s.SocketPath, s.PeerGroup, gid, err)
			}
		} else {
			s.logf("warn: lookup group %q: %v (skipping chown)", s.PeerGroup, err)
		}
	}

	// Close the listener when ctx is done so Accept unblocks.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// Distinguish graceful shutdown (ctx canceled → listener
			// closed) from a real accept error.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// net.ErrClosed indicates the listener was closed by us.
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.logf("accept error: %v", err)
			return err
		}
		go s.handleConn(ctx, conn)
	}
}

// Close shuts down the Accept loop. Safe to call concurrently with Serve.
func (s *Server) Close() error {
	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()
	if ln == nil {
		return nil
	}
	return ln.Close()
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	// Peer-cred check. We expect a *net.UnixConn — anything else
	// indicates a programming error (we Listen("unix", ...)).
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		s.logf("non-unix conn rejected (type %T)", conn)
		return
	}

	uid, gid, err := peerUIDGID(uc)
	if err != nil {
		s.logf("peer cred lookup failed: %v", err)
		return
	}

	if s.PeerGroup != "" {
		wantGID, gerr := lookupGID(s.PeerGroup)
		if gerr != nil {
			s.logf("warn: lookup group %q: %v (allowing connection by default)", s.PeerGroup, gerr)
		} else if !isPeerInGroup(uid, gid, wantGID) {
			s.logf("reject peer uid=%d gid=%d not in group %q (gid=%d)", uid, gid, s.PeerGroup, wantGID)
			return
		}
	}

	// Read a single request line, dispatch, write response, close.
	// One-shot connections keep the protocol simple; clients open a new
	// conn per request. This is fine for an RPC pattern with low rate.
	r := bufio.NewReader(conn)
	line, err := r.ReadBytes('\n')
	if err != nil {
		s.logf("read request from uid=%d failed: %v", uid, err)
		return
	}

	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		s.logf("audit: uid=%d cmd=<invalid> ok=false reason=parse_error", uid)
		s.writeResponse(conn, Response{OK: false, Error: "parse_error"})
		return
	}

	handler, found := s.Handlers[req.Cmd]
	if !found {
		s.logf("audit: uid=%d cmd=%q ok=false reason=unknown_command", uid, req.Cmd)
		s.writeResponse(conn, Response{RequestID: req.RequestID, OK: false, Error: "unknown_command"})
		return
	}

	result, herr := handler(ctx, req.Params)
	if herr != nil {
		s.logf("audit: uid=%d cmd=%q ok=false reason=%s", uid, req.Cmd, herr.Error())
		s.writeResponse(conn, Response{RequestID: req.RequestID, OK: false, Error: herr.Error()})
		return
	}

	resultJSON, mErr := json.Marshal(result)
	if mErr != nil {
		s.logf("audit: uid=%d cmd=%q ok=false reason=marshal_error", uid, req.Cmd)
		s.writeResponse(conn, Response{RequestID: req.RequestID, OK: false, Error: "marshal_error"})
		return
	}
	s.logf("audit: uid=%d cmd=%q ok=true", uid, req.Cmd)
	s.writeResponse(conn, Response{RequestID: req.RequestID, OK: true, Result: resultJSON})
}

func (s *Server) writeResponse(conn net.Conn, resp Response) {
	enc := json.NewEncoder(conn)
	if err := enc.Encode(resp); err != nil {
		s.logf("write response failed: %v", err)
	}
}

func (s *Server) logf(format string, v ...any) {
	if s.Logger != nil {
		s.Logger(format, v...)
	}
}

func (s *Server) handlerNames() []string {
	out := make([]string, 0, len(s.Handlers))
	for k := range s.Handlers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// lookupGID returns the numeric GID for a group name. Exposed so tests
// can stub.
func lookupGID(group string) (int, error) {
	g, err := user.LookupGroup(group)
	if err != nil {
		return 0, err
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf("parse gid %q: %w", g.Gid, err)
	}
	return gid, nil
}

// isPeerInGroup returns true when the peer is allowed. We accept the peer
// if either its primary gid matches wantGID OR its uid resolves to a user
// whose secondary group set contains wantGID. The simpler primary-gid
// match is sufficient for the production `borgee` user (whose primary
// group IS `borgee`), the secondary lookup is defense for hosts where an
// operator placed the helper account in a richer group set.
func isPeerInGroup(uid, gid uint32, wantGID int) bool {
	if int(gid) == wantGID {
		return true
	}
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return false
	}
	groups, err := u.GroupIds()
	if err != nil {
		return false
	}
	for _, g := range groups {
		if gid, err := strconv.Atoi(g); err == nil && gid == wantGID {
			return true
		}
	}
	return false
}
