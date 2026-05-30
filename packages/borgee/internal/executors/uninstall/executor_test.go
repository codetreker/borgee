//go:build linux || darwin

package uninstall

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"borgee/internal/dispatch"
	"borgee/internal/outbound"
)

// recordingCmd captures the (name, args) tuple of every Run call without
// actually invoking external binaries. Production tests must NEVER call
// real systemctl / userdel — we'd damage the dev box.
type recordingCmd struct {
	mu      sync.Mutex
	calls   [][]string
	failFor map[string]error
}

func (r *recordingCmd) Run(_ context.Context, name string, args ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.failFor != nil {
		if err, ok := r.failFor[name]; ok {
			return err
		}
	}
	return nil
}

func (r *recordingCmd) callNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	for i, c := range r.calls {
		out[i] = strings.Join(c, " ")
	}
	return out
}

// fixtureLayout builds a Layout rooted in a temp dir so the test can use
// real os.RemoveAll + os.Stat without touching any production path.
func fixtureLayout(t *testing.T) (Layout, string) {
	t.Helper()
	root := t.TempDir()
	state := filepath.Join(root, "state")
	runtimeDir := filepath.Join(root, "runtime")
	binDir := filepath.Join(root, "bin")
	unitDir := filepath.Join(root, "unit")
	for _, dir := range []string{state, runtimeDir, binDir, unitDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	// Populate fixture content.
	for _, leaf := range []string{"queue", "status", "audit-handoff", "credential", "enrollment-id"} {
		path := filepath.Join(state, leaf)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("write state %s: %v", path, err)
		}
	}
	runtimeBin := filepath.Join(runtimeDir, "openclaw")
	if err := os.WriteFile(runtimeBin, []byte("rt"), 0o755); err != nil {
		t.Fatalf("write runtime: %v", err)
	}
	binaries := []string{filepath.Join(binDir, "borgee-helper"), filepath.Join(binDir, "borgee-helper-claim"), filepath.Join(binDir, "install-butler")}
	for _, b := range binaries {
		if err := os.WriteFile(b, []byte("bin"), 0o755); err != nil {
			t.Fatalf("write bin %s: %v", b, err)
		}
	}
	unit := filepath.Join(unitDir, "borgee-helper.service")
	if err := os.WriteFile(unit, []byte("[Unit]"), 0o644); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	stateDirs := []string{
		filepath.Join(state, "queue"),
		filepath.Join(state, "status"),
		filepath.Join(state, "audit-handoff"),
		filepath.Join(state, "credential"),
		filepath.Join(state, "enrollment-id"),
	}
	return Layout{
		StateDirs:       stateDirs,
		RuntimeDir:      runtimeDir,
		HelperBinaries:  binaries,
		ServiceUnitPath: unit,
		ServiceName:     "borgee-helper.service",
		UserName:        "borgee-helper",
		GroupName:       "borgee-helper",
	}, root
}

func newJob(t *testing.T, payload map[string]any) *outbound.LeasedJob {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return &outbound.LeasedJob{
		JobID:         "job-uninstall-1",
		EnrollmentID:  "enr-uninstall-1",
		JobType:       "helper.uninstall",
		SchemaVersion: 1,
		Payload:       raw,
		LeaseToken:    "v1:deadbeef",
	}
}

func assertAbsent(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected absent: %s (err=%v)", p, err)
		}
	}
}

func assertPresent(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected present: %s (err=%v)", p, err)
		}
	}
}

// TU-1 SuccessfulUninstall — full cleanup against a temp tree. Verifies
// state dirs, runtime, binaries, and unit file are gone; OS-user removal
// is attempted via the recording SystemCommand; terminal status is
// succeeded with a structured audit ref.
func TestExecutor_SuccessfulUninstall(t *testing.T) {
	t.Parallel()
	layout, _ := fixtureLayout(t)
	cmd := &recordingCmd{}
	exec := &Executor{Layout: layout, Cmd: cmd, GOOS: "linux"}

	terminal, err := exec.Execute(context.Background(), newJob(t, map[string]any{"scope": "helper"}))
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if terminal.Status != dispatch.StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", terminal.Status)
	}

	// File-system buckets executed.
	assertAbsent(t, layout.StateDirs...)
	assertAbsent(t, layout.RuntimeDir)
	assertAbsent(t, layout.HelperBinaries...)
	assertAbsent(t, layout.ServiceUnitPath)

	// systemd + userdel + groupdel commands were attempted.
	names := cmd.callNames()
	wantSubstrs := []string{"systemctl disable borgee-helper.service", "userdel borgee-helper", "groupdel borgee-helper"}
	for _, want := range wantSubstrs {
		found := false
		for _, got := range names {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing recorded cmd %q (calls=%v)", want, names)
		}
	}

	// Audit ref encodes platform + bucket counts.
	if len(terminal.ResultSummary.AuditRefs) != 1 || !strings.HasPrefix(terminal.ResultSummary.AuditRefs[0], "helper-uninstall-linux-buckets-") {
		t.Fatalf("audit refs = %v, want helper-uninstall-linux-buckets-...", terminal.ResultSummary.AuditRefs)
	}
}

// TU-2 PreserveState — preserve_state=true skips the state dir bucket but
// proceeds with runtime / binaries / unit / OS principal.
func TestExecutor_PreserveState(t *testing.T) {
	t.Parallel()
	layout, _ := fixtureLayout(t)
	cmd := &recordingCmd{}
	exec := &Executor{Layout: layout, Cmd: cmd, GOOS: "linux"}

	terminal, err := exec.Execute(context.Background(), newJob(t, map[string]any{"scope": "helper", "preserve_state": true}))
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if terminal.Status != dispatch.StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", terminal.Status)
	}

	// State dirs untouched.
	assertPresent(t, layout.StateDirs...)
	// Everything else wiped.
	assertAbsent(t, layout.RuntimeDir)
	assertAbsent(t, layout.HelperBinaries...)
	assertAbsent(t, layout.ServiceUnitPath)
}

// TU-3 PartialFailureTolerant — files already absent (prior partial run)
// register as `absent` instead of `failed`; executor still returns
// succeeded. systemctl-disable-failed becomes a `failed` bucket but does
// NOT abort the rest of the cleanup.
func TestExecutor_PartialFailureTolerant(t *testing.T) {
	t.Parallel()
	layout, _ := fixtureLayout(t)
	// Pre-delete one binary + one state dir so they're already absent.
	if err := os.Remove(layout.HelperBinaries[0]); err != nil {
		t.Fatalf("pre-delete bin: %v", err)
	}
	if err := os.Remove(layout.StateDirs[0]); err != nil {
		t.Fatalf("pre-delete state: %v", err)
	}
	// Force systemctl to fail to simulate a non-root daemon.
	cmd := &recordingCmd{failFor: map[string]error{
		"systemctl": errors.New("Permission denied"),
		"userdel":   errors.New("not root"),
	}}
	exec := &Executor{Layout: layout, Cmd: cmd, GOOS: "linux"}

	terminal, err := exec.Execute(context.Background(), newJob(t, map[string]any{"scope": "helper"}))
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if terminal.Status != dispatch.StatusSucceeded {
		t.Fatalf("partial-failure path should still report succeeded (executor reports per-bucket), got %s", terminal.Status)
	}

	// Remaining files still got wiped.
	for _, p := range layout.HelperBinaries[1:] {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected absent after partial-failure path: %s", p)
		}
	}
	assertAbsent(t, layout.RuntimeDir, layout.ServiceUnitPath)
}

// TU-4 RejectInvalidScope — payload scope != "helper" fails terminal with
// schema_invalid; NO filesystem touched.
func TestExecutor_RejectInvalidScope(t *testing.T) {
	t.Parallel()
	layout, _ := fixtureLayout(t)
	cmd := &recordingCmd{}
	exec := &Executor{Layout: layout, Cmd: cmd, GOOS: "linux"}

	terminal, err := exec.Execute(context.Background(), newJob(t, map[string]any{"scope": "agent"}))
	if err == nil {
		t.Fatalf("expected error for invalid scope")
	}
	if terminal.Status != dispatch.StatusFailed || terminal.FailureCode != "schema_invalid" {
		t.Fatalf("terminal = %+v, want failed/schema_invalid", terminal)
	}
	// Nothing touched.
	assertPresent(t, layout.StateDirs...)
	assertPresent(t, layout.RuntimeDir)
	assertPresent(t, layout.HelperBinaries...)
	assertPresent(t, layout.ServiceUnitPath)
	if calls := cmd.callNames(); len(calls) != 0 {
		t.Fatalf("expected no system commands invoked, got %v", calls)
	}
}

// TU-5 NoSelfStopSignal — guard against future regressions: the executor
// must NEVER call `systemctl stop borgee-helper` from inside the running
// daemon (would SIGTERM us before /result lands). Verifies the recorded
// command list contains `systemctl disable ...` but no `systemctl stop`.
func TestExecutor_NoSelfStopSignal(t *testing.T) {
	t.Parallel()
	layout, _ := fixtureLayout(t)
	cmd := &recordingCmd{}
	exec := &Executor{Layout: layout, Cmd: cmd, GOOS: "linux"}

	if _, err := exec.Execute(context.Background(), newJob(t, map[string]any{"scope": "helper"})); err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	for _, c := range cmd.callNames() {
		if strings.HasPrefix(c, "systemctl stop ") || strings.Contains(c, " stop borgee-helper") {
			t.Fatalf("executor must not stop itself: invoked %q", c)
		}
	}
}

// TU-6 DarwinPlatform — macOS branches use launchctl + dscl, no systemctl
// or userdel anywhere in the command record.
func TestExecutor_DarwinUsesLaunchctlAndDscl(t *testing.T) {
	t.Parallel()
	layout, _ := fixtureLayout(t)
	// Reuse the linux-shaped fixture but adjust principal names so the
	// recorded commands match the macOS conventions.
	layout.UserName = "_borgee-helper"
	layout.GroupName = "_borgee-helper"
	layout.ServiceName = "cloud.borgee.host-bridge"
	cmd := &recordingCmd{}
	exec := &Executor{Layout: layout, Cmd: cmd, GOOS: "darwin"}

	if _, err := exec.Execute(context.Background(), newJob(t, map[string]any{"scope": "helper"})); err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	calls := cmd.callNames()
	wantSubstrs := []string{
		"launchctl disable system/cloud.borgee.host-bridge",
		"dscl . -delete /Users/_borgee-helper",
		"dscl . -delete /Groups/_borgee-helper",
	}
	for _, want := range wantSubstrs {
		found := false
		for _, got := range calls {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing recorded cmd %q (calls=%v)", want, calls)
		}
	}
	for _, c := range calls {
		if strings.HasPrefix(c, "systemctl") || strings.HasPrefix(c, "userdel") {
			t.Fatalf("darwin path must not use Linux commands, got %q", c)
		}
	}
}

// TU-7 NilJobSafety — defensive: nil job is rejected without panic.
func TestExecutor_NilJob(t *testing.T) {
	t.Parallel()
	exec := &Executor{}
	terminal, err := exec.Execute(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error for nil job")
	}
	if terminal.Status != dispatch.StatusFailed || terminal.FailureCode != "schema_invalid" {
		t.Fatalf("terminal = %+v, want failed/schema_invalid", terminal)
	}
}

// TU-8 DefaultLayoutUserOwned — the default uninstall layout follows the
// installing user's home/XDG paths and does not try to delete an OS user.
func TestDefaultLayout_LinuxPostRename(t *testing.T) {
	t.Parallel()
	l := DefaultLayout("linux")
	if l.UserName != "" || l.GroupName != "" {
		t.Fatalf("linux user/group = %s/%s, want empty (do not delete OS user)", l.UserName, l.GroupName)
	}
	if l.ServiceName != "borgee.service" {
		t.Fatalf("linux service name = %q, want borgee.service", l.ServiceName)
	}
	if !strings.HasSuffix(l.ServiceUnitPath, "/.config/systemd/user/borgee.service") {
		t.Fatalf("linux unit path = %q", l.ServiceUnitPath)
	}
	// rootd-skeleton: DefaultLayout must include the rootd companion
	// unit + service name so uninstall takes both down.
	if !strings.HasPrefix(l.RootdServiceName, "borgee-rootd-") || !strings.HasSuffix(l.RootdServiceName, ".service") {
		t.Fatalf("linux rootd service name = %q, want per-uid borgee-rootd-<uid>.service", l.RootdServiceName)
	}
	if !strings.HasPrefix(l.RootdServiceUnitPath, "/etc/systemd/system/borgee-rootd-") {
		t.Fatalf("linux rootd unit path = %q", l.RootdServiceUnitPath)
	}
	// rootd UDS socket file is listed as AuxFiles so a stale socket
	// from a prior boot does not trip the new rootd's bind.
	foundRootdSock := false
	for _, a := range l.AuxFiles {
		if strings.Contains(a, "/run/borgee/") && strings.HasSuffix(a, "/borgee-rootd.sock") {
			foundRootdSock = true
		}
	}
	if !foundRootdSock {
		t.Fatalf("linux DefaultLayout missing rootd socket in AuxFiles, got %v", l.AuxFiles)
	}
	wantSuffixes := map[string]bool{
		"/.local/state/borgee/queue":         false,
		"/.local/state/borgee/status":        false,
		"/.local/state/borgee/audit-handoff": false,
		"/.local/state/borgee/credential":    false,
	}
	for _, d := range l.StateDirs {
		matched := false
		for suffix := range wantSuffixes {
			if strings.HasSuffix(d, suffix) {
				wantSuffixes[suffix] = true
				matched = true
			}
		}
		if !matched {
			t.Fatalf("unexpected state dir %q in DefaultLayout(linux)", d)
		}
	}
	for suffix, seen := range wantSuffixes {
		if !seen {
			t.Fatalf("missing state dir suffix %q in %v", suffix, l.StateDirs)
		}
	}
	if !strings.HasSuffix(l.RuntimeDir, "/.local/share/borgee") {
		t.Fatalf("runtime dir = %q", l.RuntimeDir)
	}
	// `HelperBinaries` must be empty — `/usr/local/bin/borgee` is an
	// npm-owned symlink not for the executor to remove.
	if len(l.HelperBinaries) != 0 {
		t.Fatalf("HelperBinaries should be empty post-#1017, got %v", l.HelperBinaries)
	}
	// Bug 2 reverse-grep: every Layout string must NOT contain the
	// pre-rename "borgee-helper" prefix.
	for _, s := range append(append(l.StateDirs, l.ServiceUnitPath, l.ServiceName, l.RuntimeDir, l.UserName, l.GroupName), l.HelperBinaries...) {
		if strings.Contains(s, "borgee-helper") {
			t.Fatalf("post-#1017 DefaultLayout(linux) still contains pre-rename string %q", s)
		}
	}
}

func TestDefaultLayout_DarwinPostRename(t *testing.T) {
	t.Parallel()
	l := DefaultLayout("darwin")
	if l.UserName != "" || l.GroupName != "" {
		t.Fatalf("darwin user/group = %s/%s, want empty (do not delete OS user)", l.UserName, l.GroupName)
	}
	if l.ServiceName != "cloud.borgee.host-bridge" {
		t.Fatalf("darwin service name = %q", l.ServiceName)
	}
	if !strings.Contains(l.RuntimeDir, "/Library/Application Support/Borgee") {
		t.Fatalf("darwin runtime dir = %q", l.RuntimeDir)
	}
	// rootd-skeleton: DefaultLayout must include the rootd companion
	// plist + label so uninstall takes both down.
	if !strings.HasPrefix(l.RootdServiceName, "cloud.borgee.host-bridge.rootd.") {
		t.Fatalf("darwin rootd service name = %q, want per-uid label", l.RootdServiceName)
	}
	if !strings.HasPrefix(l.RootdServiceUnitPath, "/Library/LaunchDaemons/cloud.borgee.host-bridge.rootd.") {
		t.Fatalf("darwin rootd plist path = %q", l.RootdServiceUnitPath)
	}
	// AuxFiles must include the sandbox profile path so an uninstall
	// removes it (anomaly #5 from the prior audit), AND the rootd UDS
	// socket file (rootd-skeleton).
	foundSandbox := false
	foundRootdSock := false
	for _, a := range l.AuxFiles {
		if a == "/Library/Application Support/Borgee/borgee-helper.sb" {
			foundSandbox = true
		}
		if strings.Contains(a, "/Users/Shared/Borgee/") && strings.HasSuffix(a, "/borgee-rootd.sock") {
			foundRootdSock = true
		}
	}
	if !foundSandbox {
		t.Fatalf("darwin DefaultLayout AuxFiles missing sandbox profile, got %v", l.AuxFiles)
	}
	if !foundRootdSock {
		t.Fatalf("darwin DefaultLayout AuxFiles missing rootd socket, got %v", l.AuxFiles)
	}
	// Reverse-grep: state dirs must not contain pre-rename helper user
	// suffix. Sandbox profile filename is intentionally kept as
	// borgee-helper.sb (matches the file setup.go writes today; safe
	// to keep across the rename).
	for _, s := range append(l.StateDirs, l.RuntimeDir) {
		if strings.Contains(s, "_borgee-helper") {
			t.Fatalf("post-#1017 DefaultLayout(darwin) still contains pre-rename %q", s)
		}
	}
}

// TU-9 AuxFilesRemoved — when Layout.AuxFiles is non-empty, those paths
// are removed (e.g. macOS sandbox profile).
func TestExecutor_AuxFilesRemoved(t *testing.T) {
	t.Parallel()
	layout, _ := fixtureLayout(t)
	// Add an aux file outside the runtime / state trees.
	auxRoot := t.TempDir()
	aux := auxRoot + "/borgee-helper.sb"
	if err := os.WriteFile(aux, []byte("(version 1)"), 0o644); err != nil {
		t.Fatalf("write aux: %v", err)
	}
	layout.AuxFiles = []string{aux}
	cmd := &recordingCmd{}
	exec := &Executor{Layout: layout, Cmd: cmd, GOOS: "linux"}
	if _, err := exec.Execute(context.Background(), newJob(t, map[string]any{"scope": "helper"})); err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if _, err := os.Stat(aux); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("aux file %s should be removed (err=%v)", aux, err)
	}
}

// TU-10 RootdCompanionDisabledAndRemoved — when Layout.RootdServiceName +
// RootdServiceUnitPath are populated (the post-rootd-skeleton DefaultLayout
// shape), the executor must disable + remove both the main service and
// the rootd companion. Guards against a regression that forgot to take
// rootd down, which would leave a stale UDS-bound process after uninstall.
func TestExecutor_RootdCompanionDisabledAndRemoved(t *testing.T) {
	t.Parallel()
	layout, root := fixtureLayout(t)
	// Add the rootd companion unit + service name to the fixture layout.
	rootdUnit := filepath.Join(root, "unit", "borgee-rootd.service")
	if err := os.WriteFile(rootdUnit, []byte("[Unit]"), 0o644); err != nil {
		t.Fatalf("write rootd unit: %v", err)
	}
	layout.RootdServiceName = "borgee-rootd.service"
	layout.RootdServiceUnitPath = rootdUnit
	cmd := &recordingCmd{}
	exec := &Executor{Layout: layout, Cmd: cmd, GOOS: "linux"}

	if _, err := exec.Execute(context.Background(), newJob(t, map[string]any{"scope": "helper"})); err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	// rootd unit file must be gone.
	if _, err := os.Stat(rootdUnit); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rootd unit file should be removed (err=%v)", err)
	}
	// Both `systemctl disable borgee-helper.service` AND
	// `systemctl disable borgee-rootd.service` must have been attempted.
	calls := cmd.callNames()
	wantSubstrs := []string{
		"systemctl disable borgee-helper.service",
		"systemctl disable borgee-rootd.service",
	}
	for _, want := range wantSubstrs {
		found := false
		for _, got := range calls {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing recorded cmd %q (calls=%v)", want, calls)
		}
	}
}
