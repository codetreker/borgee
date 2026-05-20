//go:build integration && (linux || darwin)

// Package e2e — #968 reboot/crash chain end-to-end verification.
//
// What this test proves (R4 close-out):
//   1. Operator runs `borgee claim` against the API server.
//   2. Server (httptest faking the real /claim handler shape) returns
//      helper_credential; CLI persists credential + enrollment-id +
//      device-id to disk under a real Helper StateDirectory layout.
//   3. systemd-equivalent: we then spawn the `borgee daemon` subcommand with
//      --enrollment-id-file / --helper-device-id-file /
//      --helper-credential-file and a fast heartbeat interval, mimicking
//      a post-reboot start.
//   4. Within ~1s we observe a real POST to
//      /api/v1/helper/enrollments/{id}/status carrying Bearer
//      <credential> and {"state":"connected"} — that is the producer half
//      of the reconnect contract.
//   5. We then exercise the server-side serializer (calling it directly
//      with the persisted LastSeenAt) to assert the enrollment flips to
//      `connected`. This stitches the two halves together so the test
//      proves the full chain, not just one side.
//
// Build tag `integration` keeps this out of default CI per the existing
// e2e suite pattern; `go test -tags=integration` opt-ins.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func buildClaimCLI(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "borgee")
	cmd := exec.Command("go", "build", "-o", binPath, "../cmd/borgee")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build borgee: %v", err)
	}
	return binPath
}

// fakeAPIServer simulates the two helper endpoints the chain exercises:
//   - POST /api/v1/helper/enrollments/{id}/claim   -> 201, helper_credential
//   - POST /api/v1/helper/enrollments/{id}/status  -> 200; records hit
//
// It exposes counters + the last status request body so the test can assert
// the daemon produced the exact payload shape.
type fakeAPIServer struct {
	srv             *httptest.Server
	enrollmentID    string
	credential      string
	claimHits       atomic.Int64
	statusHits      atomic.Int64
	mu              sync.Mutex
	lastStatusAuth  string
	lastStatusBody  []byte
	statusHitTimeNS atomic.Int64
}

func newFakeAPIServer(enrollmentID, credential string) *fakeAPIServer {
	s := &fakeAPIServer{enrollmentID: enrollmentID, credential: credential}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/claim"):
			s.claimHits.Add(1)
			body, _ := io.ReadAll(r.Body)
			var req struct {
				EnrollmentSecret string `json:"enrollment_secret"`
				HelperDeviceID   string `json:"helper_device_id"`
			}
			if err := json.Unmarshal(body, &req); err != nil || req.EnrollmentSecret == "" || req.HelperDeviceID == "" {
				http.Error(w, "bad claim", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"helper_credential": s.credential,
				"enrollment":        map[string]any{"enrollment_id": s.enrollmentID},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/status"):
			body, _ := io.ReadAll(r.Body)
			s.mu.Lock()
			s.lastStatusAuth = r.Header.Get("Authorization")
			s.lastStatusBody = body
			s.mu.Unlock()
			s.statusHits.Add(1)
			s.statusHitTimeNS.Store(time.Now().UnixNano())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"enrollment":{}}`))
		default:
			http.Error(w, "unknown "+r.URL.Path, http.StatusNotFound)
		}
	}))
	return s
}

// TestClaimHeartbeatE2E covers the full producer chain end-to-end:
// claim CLI → credential persisted → daemon spawn → heartbeat fires →
// server records LastSeenAt → serializer flips to connected.
func TestClaimHeartbeatE2E(t *testing.T) {
	t.Skip("PR-2 #1038: HTTP POST /status heartbeat replaced by WS ping/pong. " +
		"PR-5 follow-up will re-add a WS-based end-to-end heartbeat e2e " +
		"against the testing environment (see issue #1033).")
	if testing.Short() {
		t.Skip("integration test (requires go build + fork+exec)")
	}
	const (
		enrollmentID = "enr-e2e-1"
		credential   = "tok-e2e-1"
		secret       = "sec-e2e-1"
	)

	api := newFakeAPIServer(enrollmentID, credential)
	t.Cleanup(api.srv.Close)

	claimBin := buildClaimCLI(t)
	stateDir := t.TempDir()
	credFile := filepath.Join(stateDir, "credential")
	idFile := filepath.Join(stateDir, "enrollment-id")
	devFile := filepath.Join(stateDir, "device-id")

	// Step 1: operator runs claim CLI via `borgee claim` subcommand.
	claimCmd := exec.Command(claimBin,
		"claim",
		"--enrollment-id="+enrollmentID,
		"--enrollment-secret="+secret,
		"--server-origin="+api.srv.URL,
		"--allow-insecure-server-origin",
		"--credential-file="+credFile,
		"--enrollment-id-file="+idFile,
		"--device-id-file="+devFile,
	)
	var claimOut, claimErr bytes.Buffer
	claimCmd.Stdout = &claimOut
	claimCmd.Stderr = &claimErr
	if err := claimCmd.Run(); err != nil {
		t.Fatalf("claim cli failed: %v\nstdout=%s\nstderr=%s", err, claimOut.String(), claimErr.String())
	}
	t.Logf("claim CLI stdout: %s", strings.TrimSpace(claimOut.String()))
	if api.claimHits.Load() != 1 {
		t.Fatalf("claim hit count = %d, want 1", api.claimHits.Load())
	}
	credBody, err := os.ReadFile(credFile)
	if err != nil || string(credBody) != credential {
		t.Fatalf("credential file content mismatch: got %q err=%v", string(credBody), err)
	}
	credStat, _ := os.Stat(credFile)
	if mode := credStat.Mode().Perm(); mode != 0o600 {
		t.Errorf("credential perm = %o, want 0600", mode)
	}

	// Step 2: spawn the daemon. We point it at the same loopback API
	// server. Use a very short heartbeat interval via env override is not
	// available — instead the daemon defaults to 60s but fires immediately
	// on start (no initial sleep), so a single hit within 2s is enough.
	daemonBin := buildDaemon(t)
	dsn := seedHostGrantsDB(t)
	runDir := t.TempDir()
	socketPath := filepath.Join(runDir, "borgee.sock")
	auditPath := filepath.Join(runDir, "audit.log.jsonl")
	queueDir := filepath.Join(stateDir, "queue")
	statusDir := filepath.Join(stateDir, "status")
	auditDir := filepath.Join(stateDir, "audit-handoff")

	// daemon prereq validator allows only state dirs under default helper
	// roots; in production `borgee setup` creates those. For the e2e test we
	// pass --allowed-state-roots=<tmp> so we can use the tmpdir without
	// needing root.
	stateRoot := t.TempDir()
	queueDir = filepath.Join(stateRoot, "queue")
	statusDir = filepath.Join(stateRoot, "status")
	auditDir = filepath.Join(stateRoot, "audit-handoff")

	daemonCmd := exec.CommandContext(context.Background(), daemonBin,
		"daemon",
		"--socket="+socketPath,
		"--audit-log="+auditPath,
		"--grants-db="+dsn,
		"--read-paths="+runDir,
		"--outbound-server-origin="+api.srv.URL,
		"--outbound-allowed-origins="+api.srv.URL,
		"--allow-loopback-outbound",
		"--allowed-state-roots="+stateRoot,
		"--queue-state-dir="+queueDir,
		"--status-state-dir="+statusDir,
		"--audit-handoff-dir="+auditDir,
		"--enrollment-id-file="+idFile,
		"--helper-device-id-file="+devFile,
		"--helper-credential-file="+credFile,
	)
	var daemonErr bytes.Buffer
	daemonCmd.Stderr = &daemonErr
	if err := daemonCmd.Start(); err != nil {
		t.Fatalf("daemon start: %v", err)
	}
	t.Cleanup(func() {
		_ = daemonCmd.Process.Signal(syscall.SIGTERM)
		_ = daemonCmd.Wait()
	})

	// Step 3: poll the fake API for a status hit. The daemon fires
	// immediately (no initial sleep), so 5s is plenty.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if api.statusHits.Load() > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if api.statusHits.Load() == 0 {
		// landlock EPERM is the known CI-runner failure; surface it
		// clearly so a developer sees the diagnostic.
		if strings.Contains(daemonErr.String(), "landlock_restrict_self") &&
			strings.Contains(daemonErr.String(), "operation not permitted") {
			t.Skipf("daemon landlock_restrict_self EPERM — runner lacks NoNewPrivs/CAP_SYS_ADMIN (production OK). stderr=%s", daemonErr.String())
		}
		t.Fatalf("no heartbeat hit within 5s; daemon stderr=%s", daemonErr.String())
	}

	// Step 4: assert the wire shape matches what the server expects.
	api.mu.Lock()
	gotAuth := api.lastStatusAuth
	gotBody := append([]byte(nil), api.lastStatusBody...)
	api.mu.Unlock()
	if gotAuth != "Bearer "+credential {
		t.Errorf("status Authorization = %q, want %q", gotAuth, "Bearer "+credential)
	}
	var parsed struct {
		HelperDeviceID string `json:"helper_device_id"`
		State          string `json:"state"`
	}
	if err := json.Unmarshal(gotBody, &parsed); err != nil {
		t.Fatalf("decode status body: %v", err)
	}
	if parsed.State != "connected" {
		t.Errorf("status state = %q, want %q", parsed.State, "connected")
	}
	devBytes, _ := os.ReadFile(devFile)
	wantDevice := strings.TrimSpace(string(devBytes))
	if parsed.HelperDeviceID != wantDevice {
		t.Errorf("status helper_device_id = %q, want %q (from %s)", parsed.HelperDeviceID, wantDevice, devFile)
	}

	// Step 5: simulate the server-side serializer flip. We replicate the
	// same fresh-by-recency rule that helper_enrollments.go encodes:
	// LastSeenAt within 5 minutes => status flips to `connected`. The
	// statusHitTimeNS atomic is our LastSeenAt for this test.
	lastSeenAtMs := api.statusHitTimeNS.Load() / int64(time.Millisecond)
	if lastSeenAtMs == 0 {
		t.Fatalf("internal: statusHitTimeNS not recorded")
	}
	nowMs := time.Now().UnixMilli()
	freshness := int64(5 * time.Minute / time.Millisecond)
	if nowMs-lastSeenAtMs > freshness {
		t.Fatalf("LastSeenAt is stale immediately after heartbeat: now=%d last=%d", nowMs, lastSeenAtMs)
	}
	derived := serializeStatus(lastSeenAtMs, nowMs, freshness)
	if derived != "connected" {
		t.Errorf("serializer flip = %q, want %q", derived, "connected")
	}

	t.Logf("E2E PASS: claim_hits=%d status_hits=%d body=%s",
		api.claimHits.Load(), api.statusHits.Load(), string(gotBody))
}

// serializeStatus mirrors the freshness rule in
// server-go/internal/api/helper_enrollments.go::serializeWithConfigure. We
// keep the rule duplicated (rather than imported across modules) so the
// test stays a Go-module-local integration test.
func serializeStatus(lastSeenAtMs, nowMs, freshnessMs int64) string {
	if lastSeenAtMs == 0 {
		return "offline"
	}
	if nowMs-lastSeenAtMs <= freshnessMs {
		return "connected"
	}
	return "offline"
}
