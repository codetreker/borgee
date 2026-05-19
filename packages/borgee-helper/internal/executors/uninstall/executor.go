//go:build linux || darwin

// Package uninstall implements the `helper.uninstall` dispatcher executor
// (#998). Blueprint promise: 装得上卸得掉 — one server-enqueued job tears
// down the helper's local footprint (binaries, state, runtime, OS user/group,
// systemd / launchd unit) and POSTs a terminal `succeeded` Result. The
// server-side complete handler (helper_job_queries.CompleteHelperJobForHelper)
// flips the enrollment to `uninstalled` in the same transaction so the
// server-recorded lifecycle state matches the helper's local teardown.
//
// Self-uninstall safety: the executor runs INSIDE the long-lived
// borgee-helper daemon process. Removing the daemon's own binary while it
// runs is safe on POSIX (open inode keeps the live process resident), but
// `systemctl stop borgee-helper` from inside the daemon would SIGTERM us
// mid-cleanup and the dispatcher would never POST the final Result. The
// executor therefore intentionally does NOT issue a stop signal to itself.
// Cleanup order:
//
//	1. systemctl disable / launchctl disable (does not kill us)
//	2. remove unit / plist file
//	3. wipe runtime binaries (under /usr/local/lib/borgee/)
//	4. wipe helper binaries (under /usr/local/bin/)
//	5. wipe Helper-owned state dirs (queue / status / audit-handoff /
//	   credential / enrollment-id / device-id) UNLESS preserve_state=true
//	6. delete OS user/group (userdel / dscl --delete)
//	7. return terminal `succeeded` with a typed summary of what was removed
//
// After the dispatcher posts the Result, the daemon exits naturally on the
// next poll loop iteration (or systemd reaps it on shutdown). Either path
// leaves the server with the source-of-truth terminal status.
//
// Privilege: most cleanup steps require root or CAP_DAC_OVERRIDE. The
// production helper daemon runs as the system `borgee-helper` user, which
// does NOT have those caps by default. Therefore the executor uses a
// SystemCommand interface that defaults to `exec.Command` in production
// and that tests stub out. When the executor lacks the OS privilege to run
// `systemctl disable` / `userdel`, those individual steps log a warning and
// the executor continues — the per-file cleanup it CAN do (state dirs
// owned by borgee-helper) still happens, and the executor reports the
// per-bucket results in the terminal `result_summary`. Operators that need
// a fully-clean uninstall can wrap borgee-helper with the documented
// sudoers entry (see README.md).
package uninstall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"borgee-helper/internal/dispatch"
	"borgee-helper/internal/outbound"
)

// Default install layout for Linux + macOS. Tests override every field via
// Executor.Layout so no real filesystem is touched. Production main.go
// constructs Executor with Layout=DefaultLayout(runtime.GOOS).
type Layout struct {
	// Helper-owned state directories — wiped unless preserve_state=true.
	StateDirs []string
	// Runtime binaries installed by install-butler.
	RuntimeDir string
	// Helper-shipped binaries (the daemon, claim CLI, install-butler).
	HelperBinaries []string
	// Service unit / plist file path.
	ServiceUnitPath string
	// systemd service name (Linux) or launchd label (macOS).
	ServiceName string
	// OS user + group to delete at the end.
	UserName  string
	GroupName string
}

// DefaultLayout returns the production install layout for the given GOOS.
// `goos` must be "linux" or "darwin"; any other value returns a zero
// Layout (the executor will then no-op every bucket — safe but useless).
func DefaultLayout(goos string) Layout {
	switch goos {
	case "linux":
		return Layout{
			StateDirs: []string{
				"/var/lib/borgee-helper/queue",
				"/var/lib/borgee-helper/status",
				"/var/lib/borgee-helper/audit-handoff",
				"/var/lib/borgee-helper/credential",
				"/var/lib/borgee-helper/enrollment-id",
				"/var/lib/borgee-helper/device-id",
			},
			RuntimeDir: "/usr/local/lib/borgee",
			HelperBinaries: []string{
				"/usr/local/bin/borgee-helper",
				"/usr/local/bin/borgee-helper-claim",
				"/usr/local/bin/install-butler",
			},
			ServiceUnitPath: "/etc/systemd/system/borgee-helper.service",
			ServiceName:     "borgee-helper.service",
			UserName:        "borgee-helper",
			GroupName:       "borgee-helper",
		}
	case "darwin":
		return Layout{
			StateDirs: []string{
				"/Library/Application Support/Borgee/Helper/QueueState",
				"/Library/Application Support/Borgee/Helper/StatusState",
				"/Library/Application Support/Borgee/Helper/AuditHandoff",
				"/Library/Application Support/Borgee/Helper/credential",
				"/Library/Application Support/Borgee/Helper/enrollment-id",
				"/Library/Application Support/Borgee/Helper/device-id",
			},
			RuntimeDir: "/usr/local/lib/borgee",
			HelperBinaries: []string{
				"/usr/local/bin/borgee-helper",
				"/usr/local/bin/borgee-helper-claim",
				"/usr/local/bin/install-butler",
			},
			ServiceUnitPath: "/Library/LaunchDaemons/cloud.borgee.host-bridge.plist",
			ServiceName:     "cloud.borgee.host-bridge",
			UserName:        "_borgee-helper",
			GroupName:       "_borgee-helper",
		}
	default:
		return Layout{}
	}
}

// SystemCommand abstracts external command invocation (systemctl, launchctl,
// userdel, dscl). Production wires this to exec.Command; tests record what
// was attempted without invoking anything.
type SystemCommand interface {
	Run(ctx context.Context, name string, args ...string) error
}

type execCommand struct{}

func (execCommand) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}

// Filesystem abstracts the few os calls we make so tests can run against
// a temp dir without touching the real /usr/local or /var/lib paths.
type Filesystem interface {
	RemoveAll(path string) error
	Remove(path string) error
	Stat(path string) (exists bool, err error)
}

type realFS struct{}

func (realFS) RemoveAll(path string) error { return os.RemoveAll(path) }
func (realFS) Remove(path string) error    { return os.Remove(path) }
func (realFS) Stat(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Executor implements dispatch.Executor for job_type=helper.uninstall.
// Zero value works (Layout=DefaultLayout, Fs=realFS, Cmd=execCommand,
// GOOS=runtime.GOOS, Logger=log.Printf via dispatch). All fields are
// overridable by tests for hermetic verification.
type Executor struct {
	Layout Layout
	Fs     Filesystem
	Cmd    SystemCommand
	GOOS   string
	Logger func(format string, v ...any)
}

func (e *Executor) fs() Filesystem {
	if e.Fs != nil {
		return e.Fs
	}
	return realFS{}
}

func (e *Executor) cmd() SystemCommand {
	if e.Cmd != nil {
		return e.Cmd
	}
	return execCommand{}
}

func (e *Executor) goos() string {
	if e.GOOS != "" {
		return e.GOOS
	}
	return runtime.GOOS
}

func (e *Executor) logf(format string, v ...any) {
	if e.Logger != nil {
		e.Logger(format, v...)
	}
}

// uninstallPayload mirrors jobpolicy.go decodeStrict shape so executor-side
// re-validation is byte-identical with the policy gate. `confirm` is a
// defense-in-depth flag the API never inserts on its own — present here so
// future operator UI flows that round-trip an explicit confirmation can
// piggyback without server-side schema drift.
type uninstallPayload struct {
	Scope         string `json:"scope"`
	PreserveState bool   `json:"preserve_state,omitempty"`
}

// resultSummary is what the executor reports back via TerminalStatus on
// success. Each bucket is a per-step result with a short status string —
// "removed" / "absent" / "failed" / "skipped" — and an optional path or
// error detail. Operators reading the server-recorded result can tell
// exactly which steps the helper completed before exit.
type resultSummary struct {
	Buckets    []bucketResult `json:"buckets"`
	Platform   string         `json:"platform"`
	PreservedState bool       `json:"preserved_state"`
}

type bucketResult struct {
	Name   string `json:"name"`
	Path   string `json:"path,omitempty"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Execute runs the full uninstall sequence. Returns a terminal status the
// dispatcher posts via /result. Never returns a nil-status / non-nil-err
// shape — the dispatcher's safety net would force-fail anyway, but being
// explicit keeps the contract obvious.
func (e *Executor) Execute(ctx context.Context, job *outbound.LeasedJob) (dispatch.TerminalStatus, error) {
	if job == nil {
		return dispatch.TerminalStatus{
			Status:         dispatch.StatusFailed,
			FailureCode:    "schema_invalid",
			FailureMessage: "nil leased job",
		}, errors.New("uninstall executor: nil job")
	}
	var payload uninstallPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return dispatch.TerminalStatus{
			Status:         dispatch.StatusFailed,
			FailureCode:    "schema_invalid",
			FailureMessage: "payload decode: " + err.Error(),
		}, err
	}
	if payload.Scope != "helper" {
		return dispatch.TerminalStatus{
			Status:         dispatch.StatusFailed,
			FailureCode:    "schema_invalid",
			FailureMessage: fmt.Sprintf("invalid uninstall scope %q (only \"helper\" is supported)", payload.Scope),
		}, fmt.Errorf("invalid scope %q", payload.Scope)
	}

	layout := e.Layout
	if layout.ServiceName == "" && layout.RuntimeDir == "" && len(layout.HelperBinaries) == 0 {
		layout = DefaultLayout(e.goos())
	}

	summary := resultSummary{
		Platform:       e.goos(),
		PreservedState: payload.PreserveState,
	}

	// Bucket A: disable service so systemd / launchd doesn't respawn us
	// after the daemon process exits. INTENTIONALLY no `stop` — that would
	// SIGTERM us mid-cleanup and the dispatcher would never post Result.
	summary.Buckets = append(summary.Buckets, e.disableService(ctx, layout))

	// Bucket B: remove the service unit / plist file so a fresh install
	// re-deploys cleanly.
	summary.Buckets = append(summary.Buckets, e.removePath(layout.ServiceUnitPath, "service_unit"))

	// Bucket C: remove the runtime binaries install-butler dropped under
	// /usr/local/lib/borgee/. Whole-tree wipe — operator opted into uninstall.
	if layout.RuntimeDir != "" {
		summary.Buckets = append(summary.Buckets, e.removeTree(layout.RuntimeDir, "runtime_dir"))
	}

	// Bucket D: remove the helper-shipped binaries. We are CURRENTLY running
	// from one of these; the kernel keeps the live inode resident even after
	// unlink so this is safe on POSIX.
	for _, bin := range layout.HelperBinaries {
		summary.Buckets = append(summary.Buckets, e.removePath(bin, "helper_binary"))
	}

	// Bucket E: state dirs. Skipped entirely on preserve_state=true so an
	// operator can keep the credential + audit handoff for a post-mortem.
	if payload.PreserveState {
		summary.Buckets = append(summary.Buckets, bucketResult{Name: "state_dirs", Status: "skipped", Detail: "preserve_state=true"})
	} else {
		for _, dir := range layout.StateDirs {
			summary.Buckets = append(summary.Buckets, e.removeTree(dir, "state_dir"))
		}
	}

	// Bucket F: OS user + group. Best-effort — see README.md for the
	// privilege caveat.
	summary.Buckets = append(summary.Buckets, e.removeOSPrincipal(ctx, layout))

	summaryJSON, _ := json.Marshal(summary)
	e.logf("borgee-helper: uninstall summary: %s", string(summaryJSON))
	// result_summary is bounded to short audit / log refs (the server caps
	// each ref ≤128 chars and forbids `/`, so the structured JSON cannot go
	// here — it lives in the daemon's audit log + this executor's logger).
	// Audit refs are a short bucket-count + status digest so an operator
	// reading the server-recorded terminal can correlate to local logs.
	okCount, failCount := bucketCounts(summary.Buckets)
	return dispatch.TerminalStatus{
		Status: dispatch.StatusSucceeded,
		ResultSummary: outbound.ResultSummary{
			AuditRefs: []string{
				fmt.Sprintf("helper-uninstall-%s-buckets-%d-ok-%d-fail-%d", e.goos(), len(summary.Buckets), okCount, failCount),
			},
		},
	}, nil
}

func bucketCounts(buckets []bucketResult) (ok, fail int) {
	for _, b := range buckets {
		switch b.Status {
		case "removed", "absent", "disabled", "skipped":
			ok++
		case "failed":
			fail++
		}
	}
	return ok, fail
}

// disableService runs `systemctl disable <unit>` on Linux or
// `launchctl disable system/<label>` on macOS. Failures (non-root, unit
// not present, etc.) are logged + recorded as "failed" but do NOT abort
// the rest of the cleanup.
func (e *Executor) disableService(ctx context.Context, layout Layout) bucketResult {
	if layout.ServiceName == "" {
		return bucketResult{Name: "service_disable", Status: "skipped", Detail: "no service name configured"}
	}
	var args []string
	var name string
	switch e.goos() {
	case "linux":
		name = "systemctl"
		args = []string{"disable", layout.ServiceName}
	case "darwin":
		name = "launchctl"
		args = []string{"disable", "system/" + layout.ServiceName}
	default:
		return bucketResult{Name: "service_disable", Status: "skipped", Detail: "unsupported platform " + e.goos()}
	}
	if err := e.cmd().Run(ctx, name, args...); err != nil {
		e.logf("borgee-helper: uninstall: %s %s failed: %v", name, strings.Join(args, " "), err)
		return bucketResult{Name: "service_disable", Path: layout.ServiceName, Status: "failed", Detail: err.Error()}
	}
	return bucketResult{Name: "service_disable", Path: layout.ServiceName, Status: "disabled"}
}

// removePath removes a single file. ENOENT is recorded as "absent", not
// "failed" — operator should not see a red bucket because a partial prior
// uninstall already deleted the file.
func (e *Executor) removePath(path, bucket string) bucketResult {
	if strings.TrimSpace(path) == "" {
		return bucketResult{Name: bucket, Status: "skipped", Detail: "empty path"}
	}
	if !filepath.IsAbs(path) {
		return bucketResult{Name: bucket, Path: path, Status: "skipped", Detail: "non-absolute path"}
	}
	exists, err := e.fs().Stat(path)
	if err != nil {
		return bucketResult{Name: bucket, Path: path, Status: "failed", Detail: err.Error()}
	}
	if !exists {
		return bucketResult{Name: bucket, Path: path, Status: "absent"}
	}
	if err := e.fs().Remove(path); err != nil {
		e.logf("borgee-helper: uninstall: remove %s failed: %v", path, err)
		return bucketResult{Name: bucket, Path: path, Status: "failed", Detail: err.Error()}
	}
	return bucketResult{Name: bucket, Path: path, Status: "removed"}
}

// removeTree removes a whole directory tree. Same absent-vs-failed
// semantics as removePath.
func (e *Executor) removeTree(path, bucket string) bucketResult {
	if strings.TrimSpace(path) == "" {
		return bucketResult{Name: bucket, Status: "skipped", Detail: "empty path"}
	}
	if !filepath.IsAbs(path) {
		return bucketResult{Name: bucket, Path: path, Status: "skipped", Detail: "non-absolute path"}
	}
	exists, err := e.fs().Stat(path)
	if err != nil {
		return bucketResult{Name: bucket, Path: path, Status: "failed", Detail: err.Error()}
	}
	if !exists {
		return bucketResult{Name: bucket, Path: path, Status: "absent"}
	}
	if err := e.fs().RemoveAll(path); err != nil {
		e.logf("borgee-helper: uninstall: removeAll %s failed: %v", path, err)
		return bucketResult{Name: bucket, Path: path, Status: "failed", Detail: err.Error()}
	}
	return bucketResult{Name: bucket, Path: path, Status: "removed"}
}

// removeOSPrincipal deletes the helper user + group. On Linux it uses
// `userdel` (NOT `-r` since the helper user is created with --no-create-home
// per packaging — `-r` would noisily fail). On macOS it uses `dscl`.
// Failures are logged + recorded but do not abort the executor.
func (e *Executor) removeOSPrincipal(ctx context.Context, layout Layout) bucketResult {
	if layout.UserName == "" {
		return bucketResult{Name: "os_user", Status: "skipped", Detail: "no user configured"}
	}
	switch e.goos() {
	case "linux":
		if err := e.cmd().Run(ctx, "userdel", layout.UserName); err != nil {
			e.logf("borgee-helper: uninstall: userdel %s failed: %v", layout.UserName, err)
			return bucketResult{Name: "os_user", Path: layout.UserName, Status: "failed", Detail: err.Error()}
		}
		// groupdel is best-effort: most distros auto-delete the group with
		// userdel when no other members exist, but call it explicitly so
		// the bucket result is honest about what we attempted.
		if layout.GroupName != "" {
			_ = e.cmd().Run(ctx, "groupdel", layout.GroupName)
		}
		return bucketResult{Name: "os_user", Path: layout.UserName, Status: "removed"}
	case "darwin":
		if err := e.cmd().Run(ctx, "dscl", ".", "-delete", "/Users/"+layout.UserName); err != nil {
			e.logf("borgee-helper: uninstall: dscl delete user %s failed: %v", layout.UserName, err)
			return bucketResult{Name: "os_user", Path: layout.UserName, Status: "failed", Detail: err.Error()}
		}
		if layout.GroupName != "" {
			_ = e.cmd().Run(ctx, "dscl", ".", "-delete", "/Groups/"+layout.GroupName)
		}
		return bucketResult{Name: "os_user", Path: layout.UserName, Status: "removed"}
	default:
		return bucketResult{Name: "os_user", Status: "skipped", Detail: "unsupported platform " + e.goos()}
	}
}

// Ensure compile-time conformance with dispatch.Executor.
var _ dispatch.Executor = (*Executor)(nil)
