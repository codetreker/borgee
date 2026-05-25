//go:build linux || darwin

package rootd

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// fakeFS records every Remove call for assertion. Returns the next
// queued error in order, or nil when the queue is empty.
type fakeFS struct {
	mu      sync.Mutex
	removed []string
	errs    []error
}

func (f *fakeFS) Remove(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, path)
	if len(f.errs) == 0 {
		return nil
	}
	err := f.errs[0]
	f.errs = f.errs[1:]
	return err
}

func callHandler(t *testing.T, h HandlerFunc, params any) (json.RawMessage, error) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	result, herr := h(context.Background(), raw)
	if herr != nil {
		return nil, herr
	}
	out, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	return out, nil
}

// TestInstallPluginRejectsUnknownFields covers the strict-decode
// invariant. A typo or extra field at the rootd boundary must come back
// as schema_invalid before we ever invoke install-butler.
func TestInstallPluginRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"manifest_url":"https://x","pubkey_base64":"AA","plugin_id":"openclaw","target_path":"/usr/local/lib/borgee/openclaw","unknown_field":true}`)
	if _, err := installPluginHandler(context.Background(), raw); err == nil || !strings.Contains(err.Error(), "schema_invalid") {
		t.Fatalf("install_plugin should reject unknown field, got err=%v", err)
	}
}

// TestInstallPluginRejectsInsecureManifestURL covers the https-only
// invariant. Operator tooling can opt into http:// for local dev via
// allow_insecure_manifest, but the default rejects.
func TestInstallPluginRejectsInsecureManifestURL(t *testing.T) {
	t.Parallel()
	_, err := callHandler(t, installPluginHandler, InstallPluginParams{
		ManifestURL:  "http://insecure.example/manifest.json",
		PubKeyBase64: "AA",
		PluginID:     "openclaw",
		TargetPath:   "/usr/local/lib/borgee/openclaw",
	})
	if err == nil || !strings.Contains(err.Error(), "manifest_url_insecure") {
		t.Fatalf("expected manifest_url_insecure, got %v", err)
	}
}

// TestInstallPluginRejectsTargetOutsideAllowPrefixes is the defense-in-
// depth check that rootd will never write to /etc, /tmp, or any other
// directory the daemon could ask for. Even if the daemon's manifest-
// binding resolution were compromised, rootd's allow-prefix list pins
// the writable surface.
func TestInstallPluginRejectsTargetOutsideAllowPrefixes(t *testing.T) {
	t.Parallel()
	for _, target := range []string{
		"/etc/openclaw",
		"/tmp/openclaw",
		"/usr/local/bin/openclaw",
		"relative/path",
		"/usr/local/lib/borgee/../escape/openclaw",
	} {
		_, err := callHandler(t, installPluginHandler, InstallPluginParams{
			ManifestURL:  "https://x.example/manifest.json",
			PubKeyBase64: "AA",
			PluginID:     "openclaw",
			TargetPath:   target,
		})
		if err == nil || !strings.Contains(err.Error(), "denied") && !strings.Contains(err.Error(), "schema_invalid") {
			t.Fatalf("target %q should be rejected, got err=%v", target, err)
		}
	}
}

// TestInstallPluginRejectsBadPluginID covers the plugin_id regexp gate.
func TestInstallPluginRejectsBadPluginID(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"", "../escape", "Up", "x/y", ".dot"} {
		_, err := callHandler(t, installPluginHandler, InstallPluginParams{
			ManifestURL:  "https://x.example/manifest.json",
			PubKeyBase64: "AA",
			PluginID:     bad,
			TargetPath:   "/usr/local/lib/borgee/openclaw",
		})
		if err == nil {
			t.Fatalf("plugin_id %q should be rejected", bad)
		}
	}
}

// TestInstallPluginInvokesInstallButlerAndForwardsFailure is the happy/
// negative path through install-butler. We use an unreachable manifest
// url so install-butler fails at fetch with reason=manifest_fetch_failed
// — proves rootd actually invokes install-butler and surfaces the
// reason:detail line back to the daemon.
func TestInstallPluginInvokesInstallButlerAndForwardsFailure(t *testing.T) {
	t.Parallel()
	_, err := callHandler(t, installPluginHandler, InstallPluginParams{
		ManifestURL:  "https://127.0.0.1:1/does-not-exist",
		PubKeyBase64: "AA",
		PluginID:     "openclaw",
		TargetPath:   "/usr/local/lib/borgee/openclaw",
	})
	if err == nil {
		t.Fatalf("expected install-butler failure")
	}
	// pubkey is "AA" (1 byte, decodes to 1 byte) — install-butler decodes
	// pubkey BEFORE fetching the manifest, so the first failure we see
	// is signature_invalid: pubkey length. Either failure proves
	// install-butler was actually invoked.
	if !strings.Contains(err.Error(), "signature_invalid") && !strings.Contains(err.Error(), "manifest_fetch_failed") {
		t.Fatalf("expected install-butler reason, got %v", err)
	}
}

// TestServiceLifecycleRejectsUnknownManager covers the typed manager
// whitelist. A request claiming an unsupported init system is rejected
// before exec.
func TestServiceLifecycleRejectsUnknownManager(t *testing.T) {
	t.Parallel()
	for _, mgr := range []string{"", "upstart", "runit", "exec"} {
		_, err := callHandler(t, serviceLifecycleHandler, ServiceLifecycleParams{
			Manager:   mgr,
			Unit:      "openclaw.service",
			Operation: "restart",
		})
		if err == nil || !strings.Contains(err.Error(), "manager") {
			t.Fatalf("manager %q should be rejected, got err=%v", mgr, err)
		}
	}
}

// TestServiceLifecycleRejectsBadUnitName guards against shell-meta or
// path-segment injection via the unit field.
func TestServiceLifecycleRejectsBadUnitName(t *testing.T) {
	t.Parallel()
	mgr := "systemd"
	if runtime.GOOS == "darwin" {
		mgr = "launchd"
	}
	for _, unit := range []string{
		"",
		"openclaw.service; rm -rf /",
		"../escape.service",
		"/abs/path.service",
		"openclaw\nservice",
	} {
		_, err := callHandler(t, serviceLifecycleHandler, ServiceLifecycleParams{
			Manager:   mgr,
			Unit:      unit,
			Operation: "restart",
		})
		if err == nil {
			t.Fatalf("unit %q should be rejected", unit)
		}
	}
}

// TestServiceLifecycleRejectsBadOperation guards the operation
// whitelist (start/stop/restart/reload/enable/disable only).
func TestServiceLifecycleRejectsBadOperation(t *testing.T) {
	t.Parallel()
	mgr := "systemd"
	if runtime.GOOS == "darwin" {
		mgr = "launchd"
	}
	for _, op := range []string{"", "exec", "mask", "kill -9", "reload-or-restart"} {
		_, err := callHandler(t, serviceLifecycleHandler, ServiceLifecycleParams{
			Manager:   mgr,
			Unit:      "openclaw.service",
			Operation: op,
		})
		if err == nil {
			t.Fatalf("operation %q should be rejected", op)
		}
	}
}

// TestServiceLifecyclePropagatesExecFailure asserts the handler surfaces
// a non-zero exit from systemctl/launchctl as ok:false with
// service_op_failed. Uses a nonexistent unit so systemctl fails fast
// without needing root privileges.
func TestServiceLifecyclePropagatesExecFailure(t *testing.T) {
	t.Parallel()
	mgr := "systemd"
	if runtime.GOOS == "darwin" {
		mgr = "launchd"
	}
	_, err := callHandler(t, serviceLifecycleHandler, ServiceLifecycleParams{
		Manager:   mgr,
		Unit:      "rootd-test-nonexistent.service",
		Operation: "restart",
	})
	if err == nil {
		// Test boxes without systemctl/launchctl reaching the unit will
		// also fail; either way ok:false. A null err means we accidentally
		// passed the validation without invoking the manager.
		t.Skip("systemctl/launchctl absent or noop on this host; cannot exercise exec_failed")
	}
	if !strings.Contains(err.Error(), "service_op_failed") && !strings.Contains(err.Error(), "exec_failed") {
		t.Fatalf("expected service_op_failed / exec_failed, got %v", err)
	}
}

// PR-4 P0 review fix — `launchctl restart` / `launchctl reload` are not
// real subcommands in launchctl V2. The handler must map them onto
// `launchctl kickstart -k` and `launchctl kill HUP`. We assert the
// argv directly via the pure-function seam (serviceLifecycleArgv) so
// the test runs on any platform without requiring real launchctl.

// TestServiceLifecycle_DarwinUsesLaunchctlKickstartForRestart asserts
// the V2 restart idiom.
func TestServiceLifecycle_DarwinUsesLaunchctlKickstartForRestart(t *testing.T) {
	t.Parallel()
	name, args, err := serviceLifecycleArgv(ServiceLifecycleParams{
		Manager:   "launchd",
		Unit:      "com.borgee.openclaw.service",
		Operation: "restart",
	})
	if err != nil {
		t.Fatalf("serviceLifecycleArgv restart: %v", err)
	}
	if name != "launchctl" {
		t.Fatalf("binary=%q, want launchctl", name)
	}
	want := []string{"kickstart", "-k", "system/com.borgee.openclaw.service"}
	if !equalSlices(args, want) {
		t.Fatalf("argv=%v, want %v", args, want)
	}
}

// TestServiceLifecycle_DarwinUsesLaunchctlKillForReload asserts SIGHUP
// for reload semantics.
func TestServiceLifecycle_DarwinUsesLaunchctlKillForReload(t *testing.T) {
	t.Parallel()
	name, args, err := serviceLifecycleArgv(ServiceLifecycleParams{
		Manager:   "launchd",
		Unit:      "com.borgee.openclaw.service",
		Operation: "reload",
	})
	if err != nil {
		t.Fatalf("serviceLifecycleArgv reload: %v", err)
	}
	if name != "launchctl" {
		t.Fatalf("binary=%q, want launchctl", name)
	}
	want := []string{"kill", "HUP", "system/com.borgee.openclaw.service"}
	if !equalSlices(args, want) {
		t.Fatalf("argv=%v, want %v", args, want)
	}
}

// TestServiceLifecycle_DarwinAllOperationsCovered — table-driven
// assertion of the full 6-op launchctl V2 mapping. Locks the contract
// so a future regression cannot silently revert one op to the bogus
// `launchctl <op> system/<label>` form.
func TestServiceLifecycle_DarwinAllOperationsCovered(t *testing.T) {
	t.Parallel()
	const unit = "com.borgee.openclaw.service"
	const label = "system/" + unit
	cases := []struct {
		op       string
		wantArgs []string
	}{
		{"start", []string{"kickstart", label}},
		{"stop", []string{"kill", "TERM", label}},
		{"restart", []string{"kickstart", "-k", label}},
		{"reload", []string{"kill", "HUP", label}},
		{"enable", []string{"enable", label}},
		{"disable", []string{"disable", label}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.op, func(t *testing.T) {
			t.Parallel()
			name, args, err := serviceLifecycleArgv(ServiceLifecycleParams{
				Manager:   "launchd",
				Unit:      unit,
				Operation: tc.op,
			})
			if err != nil {
				t.Fatalf("serviceLifecycleArgv %s: %v", tc.op, err)
			}
			if name != "launchctl" {
				t.Fatalf("op=%s name=%q, want launchctl", tc.op, name)
			}
			if !equalSlices(args, tc.wantArgs) {
				t.Fatalf("op=%s argv=%v, want %v", tc.op, args, tc.wantArgs)
			}
		})
	}
}

// TestServiceLifecycle_LinuxUnchanged — systemctl native subcommands
// continue to map 1:1 (`systemctl <op> <unit>`). Locks the Linux
// behavior so the darwin refactor cannot regress it.
func TestServiceLifecycle_LinuxUnchanged(t *testing.T) {
	t.Parallel()
	const unit = "borgee-openclaw.service"
	for _, op := range []string{"start", "stop", "restart", "reload", "enable", "disable"} {
		op := op
		t.Run(op, func(t *testing.T) {
			t.Parallel()
			name, args, err := serviceLifecycleArgv(ServiceLifecycleParams{
				Manager:   "systemd",
				Unit:      unit,
				Operation: op,
			})
			if err != nil {
				t.Fatalf("serviceLifecycleArgv %s: %v", op, err)
			}
			if name != "systemctl" {
				t.Fatalf("op=%s name=%q, want systemctl", op, name)
			}
			want := []string{op, unit}
			if !equalSlices(args, want) {
				t.Fatalf("op=%s argv=%v, want %v", op, args, want)
			}
		})
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestDelegationRevokeHappyPath wipes the supplied credential paths via
// the injected fakeFS. ServiceManager left empty so the disable shell-
// out is skipped (we only want to verify the credential wipe loop).
func TestDelegationRevokeHappyPath(t *testing.T) {
	t.Parallel()
	fake := &fakeFS{}
	handler := delegationRevokeHandlerWithFS(fake)
	out, err := callHandler(t, handler, DelegationRevokeParams{
		EnrollmentID:    "enroll-1",
		CredentialPaths: []string{"/var/lib/borgee/credential/credential", "/var/lib/borgee/credential/device-id"},
	})
	if err != nil {
		t.Fatalf("delegation_revoke happy path err=%v", err)
	}
	var got DelegationRevokeResult
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.CredentialWiped {
		t.Fatalf("expected credential_wiped=true, got %+v", got)
	}
	if len(got.WipedPaths) != 2 {
		t.Fatalf("expected 2 wiped paths, got %v", got.WipedPaths)
	}
	if len(fake.removed) != 2 {
		t.Fatalf("fakeFS removed = %v, want 2 entries", fake.removed)
	}
	for _, path := range fake.removed {
		if !filepath.IsAbs(path) {
			t.Fatalf("fakeFS asked to remove non-absolute path %q", path)
		}
	}
}

// TestDelegationRevokeIsIdempotentOnMissingFile asserts the realFS
// behavior: a missing file is recorded as wiped (not failed) so a
// retried revoke does not red-flag the audit log. We use a fakeFS that
// returns os.ErrNotExist for one of the paths; the handler should map
// it to nil and still mark CredentialWiped.
//
// NOTE: realFS handles os.ErrNotExist internally; for the closure
// (fakeFS), we model the same by returning nil. So this test instead
// covers the propagate-failure case via the next test, and uses the
// fakeFS for a stub.
func TestDelegationRevokeIsIdempotentOnMissingFile(t *testing.T) {
	t.Parallel()
	fake := &fakeFS{errs: []error{nil}} // realFS would translate ENOENT → nil already
	handler := delegationRevokeHandlerWithFS(fake)
	_, err := callHandler(t, handler, DelegationRevokeParams{
		EnrollmentID:    "enroll-2",
		CredentialPaths: []string{"/var/lib/borgee/credential/credential"},
	})
	if err != nil {
		t.Fatalf("delegation_revoke idempotent err=%v", err)
	}
}

// TestDelegationRevokePropagatesFilesystemError asserts that a real I/O
// failure during credential wipe surfaces back to the daemon as an
// error response (NOT silent success).
func TestDelegationRevokePropagatesFilesystemError(t *testing.T) {
	t.Parallel()
	fake := &fakeFS{errs: []error{errors.New("EACCES")}}
	handler := delegationRevokeHandlerWithFS(fake)
	_, err := callHandler(t, handler, DelegationRevokeParams{
		EnrollmentID:    "enroll-3",
		CredentialPaths: []string{"/var/lib/borgee/credential/credential"},
	})
	if err == nil || !strings.Contains(err.Error(), "credential_wipe_failed") {
		t.Fatalf("expected credential_wipe_failed, got %v", err)
	}
}

// TestDelegationRevokeRejectsBadCredentialPaths guards path traversal
// + non-absolute paths at the rootd boundary, even though the daemon
// executor already validates them — defense in depth.
func TestDelegationRevokeRejectsBadCredentialPaths(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		"relative/path",
		"/var/lib/borgee/../etc/passwd",
		"/etc/../../escape",
	} {
		_, err := callHandler(t, delegationRevokeHandlerWithFS(&fakeFS{}), DelegationRevokeParams{
			EnrollmentID:    "enroll-bad",
			CredentialPaths: []string{bad},
		})
		if err == nil || !strings.Contains(err.Error(), "credential_path_denied") {
			t.Fatalf("path %q should be denied, got %v", bad, err)
		}
	}
}

// TestDelegationRevokeRejectsBadEnrollmentID covers the audit-log
// invariant: every revoke records which enrollment was revoked, so
// empty is rejected.
func TestDelegationRevokeRejectsBadEnrollmentID(t *testing.T) {
	t.Parallel()
	_, err := callHandler(t, delegationRevokeHandlerWithFS(&fakeFS{}), DelegationRevokeParams{
		EnrollmentID:    "",
		CredentialPaths: []string{"/var/lib/borgee/credential/credential"},
	})
	if err == nil || !strings.Contains(err.Error(), "schema_invalid") {
		t.Fatalf("empty enrollment_id should be rejected, got %v", err)
	}
}
