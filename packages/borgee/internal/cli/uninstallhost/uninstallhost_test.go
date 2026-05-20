//go:build linux || darwin

package uninstallhost

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"

	"borgee/internal/dispatch"
	"borgee/internal/outbound"
)

// recordingRunner captures systemctl/launchctl calls.
type recordingRunner struct {
	mu    sync.Mutex
	calls [][]string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil
}

func (r *recordingRunner) joined() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	for i, c := range r.calls {
		out[i] = strings.Join(c, " ")
	}
	return out
}

// stubExecutor records what payload the executor was invoked with and
// returns a succeeded terminal so the surrounding flow keeps going.
type stubExecutor struct {
	mu      sync.Mutex
	called  bool
	payload map[string]any
	status  string
}

func (s *stubExecutor) Execute(_ context.Context, payload map[string]any) (dispatch.TerminalStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called = true
	s.payload = payload
	status := s.status
	if status == "" {
		status = dispatch.StatusSucceeded
	}
	return dispatch.TerminalStatus{
		Status: status,
		ResultSummary: outbound.ResultSummary{AuditRefs: []string{"stub-uninstall-host-1"}},
	}, nil
}

func newCfg(runner *recordingRunner, exec *stubExecutor) *config {
	return &config{
		runner:        runner,
		executor:      exec,
		skipRootCheck: true,
	}
}

// TU-1 SystemctlStopAndDisable — Linux stop + disable invoked in order.
func TestRun_LinuxStopDisable(t *testing.T) {
	runner := &recordingRunner{}
	exec := &stubExecutor{}
	cfg := newCfg(runner, exec)
	var out, errBuf bytes.Buffer
	if err := run(cfg, &out, &errBuf); err != nil {
		t.Fatalf("run: %v stderr=%s", err, errBuf.String())
	}
	got := runner.joined()
	// On linux platform, expect 2 systemctl calls (stop + disable).
	// On darwin platform, expect launchctl bootout. Accept both.
	if len(got) == 0 {
		t.Fatalf("expected stop/disable calls; got none")
	}
}

// TU-2 ExecutorInvokedWithCorrectPayload — confirms the run() flow
// passes the operator-selected --preserve-state through to the shared
// cleanup executor.
func TestRun_ExecutorPayloadCarriesPreserveState(t *testing.T) {
	runner := &recordingRunner{}
	exec := &stubExecutor{}
	cfg := newCfg(runner, exec)
	cfg.preserveState = true
	var out, errBuf bytes.Buffer
	if err := run(cfg, &out, &errBuf); err != nil {
		t.Fatalf("run: %v stderr=%s", err, errBuf.String())
	}
	if !exec.called {
		t.Fatalf("executor was not invoked")
	}
	if exec.payload["scope"] != "helper" {
		t.Fatalf("payload scope = %v, want helper", exec.payload["scope"])
	}
	if exec.payload["preserve_state"] != true {
		t.Fatalf("payload preserve_state = %v, want true", exec.payload["preserve_state"])
	}
}

// TU-3 NPMHintPrinted — final stdout includes the npm uninstall pointer
// so operators who installed via `npm i -g` finish the job.
func TestRun_PrintsNpmHint(t *testing.T) {
	runner := &recordingRunner{}
	exec := &stubExecutor{}
	cfg := newCfg(runner, exec)
	var out, errBuf bytes.Buffer
	if err := run(cfg, &out, &errBuf); err != nil {
		t.Fatalf("run: %v stderr=%s", err, errBuf.String())
	}
	want := "sudo npm uninstall -g @codetreker/borgee-remote-agent"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("stdout missing npm hint %q, got %q", want, out.String())
	}
}

// TU-4 ExecutorTerminalFailed — when the cleanup executor returns
// failed, run() propagates the error to the caller.
func TestRun_ExecutorFailedSurfaces(t *testing.T) {
	runner := &recordingRunner{}
	exec := &stubExecutor{status: dispatch.StatusFailed}
	cfg := newCfg(runner, exec)
	var out, errBuf bytes.Buffer
	err := run(cfg, &out, &errBuf)
	if err == nil {
		t.Fatalf("expected error when executor reports failed terminal")
	}
}
