package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"borgee-server/internal/datalayer"

	"github.com/coder/websocket"
)

// helper.go — PR-2 host-bridge WS transport (issue #1038).
//
// Server-side companion to the daemon outbound WS client. Mounts at
// /ws/helper/<enrollmentId>; authenticates the upgrade via the same
// Bearer credential + X-Helper-Device-Id pair the REST helper rail
// validates (`HelperEnrollmentRepository.UpdateLastSeen` checks both
// the credential digest and the device id under one DB call, so we
// reuse it for both auth and the connection-time last_seen_at update
// that replaces the prior POST /status heartbeat).
//
// Origin policy: the helper daemon is a server-to-server WS client, it
// does NOT send an Origin header. We therefore accept upgrades with no
// Origin and reject any non-empty Origin (defense against confused-
// deputy via a browser somehow targeting this endpoint). This is the
// reverse of plugin.go's InsecureSkipVerify=true, which was tolerable
// there because the plugin path runs same-origin from packaged web UI.
//
// One session per enrollment: a newer connect displaces older with
// close code 4001 "displaced". The hub's helperSessions map (added in
// hub.go) keys by enrollment_id and the displacement is enforced under
// the same lock as the registration so the second connect never sees
// a half-old session.

// HelperWSSubprotocol is the negotiated subprotocol token. Pinned so
// future revisions of the frame schema can bump the token + reject old
// clients without ambiguity.
const HelperWSSubprotocol = "borgee.helper.v1"

// HelperWSCloseDisplaced is the close code surfaced when a newer
// session for the same enrollment displaces the older one. Inside the
// 4000-4999 private range (same range plugin BPP-4 watchdog uses for
// its displaced-by-newer signal). Daemon's outbound client interprets
// this code as "stop trying to reconnect for ~back-off then resume" —
// the displacement is a normal operational signal, not a fatal stop.
const HelperWSCloseDisplaced websocket.StatusCode = 4001

// HelperEnrollmentAuthenticator is the narrow seam between this WS
// endpoint and the datalayer.HelperEnrollmentRepository. Declared here
// (not imported as the full repo) so unit tests can mock just the auth
// call without standing up the SQLite layer. The production wire
// happens in server.go via an inline adapter.
type HelperEnrollmentAuthenticator interface {
	UpdateLastSeen(ctx context.Context, id, credential, helperDeviceID string, now time.Time) (*datalayer.HelperEnrollment, error)
}

// HelperJobProcessor is the WS-side mirror of the REST helper job ack
// + result handlers. Declared here as an interface so the read loop
// stays decoupled from the package that owns the SQLite-backed
// mutation (internal/api wires the production adapter via a setter on
// the hub).
//
// ProcessAck: daemon sends {"type":"ack","job_id":...,"lease_token":...}
// → server marks delivered. Mirrors REST POST /jobs/<id>/ack.
//
// ProcessResult: daemon sends terminal {"type":"result",...} → server
// marks terminal status. Mirrors REST POST /jobs/<id>/result.
type HelperJobProcessor interface {
	ProcessHelperAck(ctx context.Context, enrollmentID, jobID, leaseToken, helperCredential, helperDeviceID string) error
	ProcessHelperResult(ctx context.Context, enrollmentID, jobID, leaseToken, helperCredential, helperDeviceID, status, failureCode, failureMessage string, summary json.RawMessage) error
}

// HelperSession is the per-enrollment WS session bookkeeping. One
// session per enrollment; older is displaced on new connect.
type HelperSession struct {
	hub          *Hub
	conn         *websocket.Conn
	enrollmentID string
	credential   string
	deviceID     string

	send chan []byte
	done chan struct{}

	lastSeenMu sync.Mutex
	lastSeenAt time.Time

	logger *slog.Logger
}

// Send queues a serialized frame on the writer pump. Drops on a full
// buffer rather than blocking — the daemon is expected to either be
// reading promptly or be torn down by the displacement / liveness
// path.
func (h *HelperSession) Send(data []byte) {
	select {
	case h.send <- data:
	default:
	}
}

// SendJSON serializes and queues a JSON frame.
func (h *HelperSession) SendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.Send(data)
}

// LastSeen returns the most recent inbound-frame timestamp.
func (h *HelperSession) LastSeen() time.Time {
	h.lastSeenMu.Lock()
	defer h.lastSeenMu.Unlock()
	return h.lastSeenAt
}

func (h *HelperSession) touchLastSeen() {
	h.lastSeenMu.Lock()
	h.lastSeenAt = time.Now()
	h.lastSeenMu.Unlock()
}

// EnrollmentID lets callers (hub, tests) read the bound enrollment.
func (h *HelperSession) EnrollmentID() string {
	return h.enrollmentID
}

// Close terminates the session, idempotent under the done channel.
func (h *HelperSession) Close(code websocket.StatusCode, reason string) {
	if h.conn != nil {
		_ = h.conn.Close(code, reason)
	}
	select {
	case <-h.done:
	default:
		close(h.done)
	}
}

// HandleHelper returns the /ws/helper/<enrollmentId> upgrade handler.
//
// authenticator validates the Bearer credential + device id pair and
// returns the enrollment row on success (errors map to 401/403). It is
// passed in (not imported as the concrete sqlite repo) so unit tests
// can substitute a fake without standing up a full server.
//
// processor handles ack/result frames from the daemon. Nil-safe; if
// processor is nil the read loop drops ack/result frames (pre-claim
// boot or unit tests not exercising the mutation path).
func HandleHelper(hub *Hub, authenticator HelperEnrollmentAuthenticator, processor HelperJobProcessor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Origin: helper daemon does not send Origin (it's not a browser).
		// Any non-empty Origin → 403. This is stricter than coder/websocket
		// defaults; the goal is to make it impossible for a same-host
		// browser script to land on this endpoint via a confused-deputy
		// attack regardless of CORS posture.
		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		enrollmentID := strings.TrimSpace(r.PathValue("enrollmentId"))
		if enrollmentID == "" {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		credential := ""
		if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
			credential = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}
		deviceID := strings.TrimSpace(r.Header.Get("X-Helper-Device-Id"))
		if credential == "" || deviceID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if authenticator == nil {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		enroll, err := authenticator.UpdateLastSeen(r.Context(), enrollmentID, credential, deviceID, time.Now())
		if err != nil {
			status := mapHelperAuthErrorToStatus(err)
			http.Error(w, http.StatusText(status), status)
			return
		}
		if enroll == nil || enroll.ID != enrollmentID {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Accept without InsecureSkipVerify. Origin is already "" by the
		// time we reach here (rejected above if not), so the package's
		// default authenticateOrigin path passes trivially.
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: []string{HelperWSSubprotocol},
		})
		if err != nil {
			hub.logger.Error("helper ws accept failed", "error", err, "enrollment_id", enrollmentID)
			return
		}

		sess := &HelperSession{
			hub:          hub,
			conn:         conn,
			enrollmentID: enrollmentID,
			credential:   credential,
			deviceID:     deviceID,
			send:         make(chan []byte, sendBufSize),
			done:         make(chan struct{}),
			lastSeenAt:   time.Now(),
			logger:       hub.logger,
		}

		// Displace any existing session for this enrollment under the
		// hub's lock. The hub returns the displaced session (if any) so
		// we close it outside the lock to avoid holding hub.mu while
		// waiting on conn.Close.
		if old := hub.RegisterHelper(enrollmentID, sess); old != nil {
			go old.Close(HelperWSCloseDisplaced, "displaced by newer session")
		}

		ctx := r.Context()
		go sess.writePump(ctx)

		// On disconnect: unregister this session ONLY if it is still the
		// current one (a displacement path already swapped us out).
		defer func() {
			hub.UnregisterHelperIfCurrent(enrollmentID, sess)
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		// Trigger any queued jobs on connect — push semantics: jobs
		// that arrived between previous disconnect and now should fire
		// immediately, not wait for a poll. The hub-level callback (set
		// by server.go boot) is called best-effort; nil means no push
		// integration wired.
		hub.invokeHelperConnectHook(enrollmentID)

		sess.readLoop(ctx, processor)
	}
}

// readLoop reads frames from the daemon. Each frame is a JSON object
// with a `type` discriminator. Unknown types are silently dropped
// (forward-compat for future daemon revisions). Ack/result frames are
// dispatched to processor.
func (h *HelperSession) readLoop(ctx context.Context, processor HelperJobProcessor) {
	for {
		_, data, err := h.conn.Read(ctx)
		if err != nil {
			return
		}
		h.touchLastSeen()

		var envelope struct {
			Type           string          `json:"type"`
			JobID          string          `json:"job_id,omitempty"`
			LeaseToken     string          `json:"lease_token,omitempty"`
			Status         string          `json:"status,omitempty"`
			FailureCode    string          `json:"failure_code,omitempty"`
			FailureMessage string          `json:"failure_message,omitempty"`
			Summary        json.RawMessage `json:"summary,omitempty"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			continue
		}

		switch envelope.Type {
		case "ack":
			if processor == nil || envelope.JobID == "" || envelope.LeaseToken == "" {
				continue
			}
			if err := processor.ProcessHelperAck(ctx, h.enrollmentID, envelope.JobID, envelope.LeaseToken, h.credential, h.deviceID); err != nil {
				h.logger.Warn("helper ws ack process failed", "enrollment_id", h.enrollmentID, "job_id", envelope.JobID, "err", err)
			}
		case "result":
			if processor == nil || envelope.JobID == "" || envelope.LeaseToken == "" {
				continue
			}
			if err := processor.ProcessHelperResult(ctx, h.enrollmentID, envelope.JobID, envelope.LeaseToken, h.credential, h.deviceID, envelope.Status, envelope.FailureCode, envelope.FailureMessage, envelope.Summary); err != nil {
				h.logger.Warn("helper ws result process failed", "enrollment_id", h.enrollmentID, "job_id", envelope.JobID, "err", err)
			}
		default:
			// Unknown frame type — soft-drop (forward-compat).
		}
	}
}

func (h *HelperSession) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.done:
			return
		case msg, ok := <-h.send:
			if !ok {
				return
			}
			if h.conn == nil {
				continue
			}
			_ = h.conn.Write(ctx, websocket.MessageText, msg)
		}
	}
}

// mapHelperAuthErrorToStatus translates datalayer auth errors to HTTP
// status codes for the upgrade response. Matches the existing
// helper_jobs.go writeHelperRailRepoError mapping so operators see a
// consistent surface across the REST + WS rails.
func mapHelperAuthErrorToStatus(err error) int {
	switch {
	case errors.Is(err, datalayer.ErrHelperEnrollmentUnauthorized),
		errors.Is(err, datalayer.ErrHelperEnrollmentInvalidInput):
		return http.StatusUnauthorized
	case errors.Is(err, datalayer.ErrHelperEnrollmentDeviceMismatch),
		errors.Is(err, datalayer.ErrHelperEnrollmentInactive),
		errors.Is(err, datalayer.ErrHelperEnrollmentForbidden):
		return http.StatusForbidden
	case errors.Is(err, datalayer.ErrHelperEnrollmentNotFound):
		return http.StatusNotFound
	default:
		return http.StatusUnauthorized
	}
}
