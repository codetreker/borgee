//go:build linux || darwin

// Package uninstall implements the `helper.uninstall` dispatcher executor
// (#998). Blueprint promise: 装得上卸得掉 — one server-enqueued job tears
// down the helper's local footprint (binaries, state, runtime, service files,
// systemd / launchd unit) and POSTs a terminal `succeeded` Result. The
// server-side complete handler (helper_job_queries.CompleteHelperJobForHelper)
// flips the enrollment to `uninstalled` in the same transaction so the
// server-recorded lifecycle state matches the helper's local teardown.
//
// Self-uninstall safety: the executor runs INSIDE the long-lived
// borgee daemon process. Removing the daemon's own binary while it
// runs is safe on POSIX (open inode keeps the live process resident), but
// `systemctl stop borgee` from inside the daemon would SIGTERM us
// mid-cleanup and the dispatcher would never POST the final Result. The
// executor therefore intentionally does NOT issue a stop signal to itself.
// Cleanup order:
//
//  1. systemctl disable / launchctl disable (does not kill us)
//  2. remove unit / plist file
//  3. wipe user-owned runtime binaries
//  4. wipe helper binaries when a custom layout explicitly lists them
//  5. wipe Helper-owned state dirs (queue / status / audit-handoff /
//     credential / enrollment-id / device-id) UNLESS preserve_state=true
//  6. delete OS user/group only when a legacy/custom layout explicitly lists one
//  7. return terminal `succeeded` with a typed summary of what was removed
//
// After the dispatcher posts the Result, the daemon exits naturally on the
// next poll loop iteration (or systemd reaps it on shutdown). Either path
// leaves the server with the source-of-truth terminal status.
//
// Privilege: most cleanup steps require root or CAP_DAC_OVERRIDE. The
// production helper daemon runs as the installing user, which does NOT have
// those caps by default. Therefore the executor uses a
// SystemCommand interface that defaults to `exec.Command` in production
// and that tests stub out. When the executor lacks the OS privilege to run
// `systemctl disable` / `userdel`, those individual steps log a warning and
// the executor continues — the per-file cleanup it CAN do (state dirs
// owned by the installing user) still happens, and the executor reports the
// per-bucket results in the terminal `result_summary`. Operators that need
// a fully-clean uninstall can wrap borgee with the documented
// sudoers entry (see README.md).
package uninstall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"borgee/internal/dispatch"
	"borgee/internal/outbound"
)

// Default install layout for Linux + macOS. Tests override every field via
// Executor.Layout so no real filesystem is touched. Production main.go
// constructs Executor with Layout=DefaultLayout(runtime.GOOS).
type Layout struct {
	// Helper-owned state directories — wiped unless preserve_state=true.
	StateDirs []string
	// Runtime binaries installed by install-butler. `borgee install`
	// (operator-driven) also drops its persistent binary copy under
	// RuntimeDir/bin/borgee, so wiping the tree takes that with it.
	RuntimeDir string
	// Legacy field: extra absolute binary paths to remove. Post-#1017 the
	// distribution is a single `borgee` binary; `/usr/local/bin/borgee` is
	// an npm shim symlink owned by npm so the executor leaves it alone
	// (operator removes it via `npm uninstall -g`). Kept here so tests can
	// still inject paths and so a future bundled distro could re-populate.
	HelperBinaries []string
	// AuxFiles — additional file paths (NOT directories) to remove. Used
	// for the macOS sandbox profile that lives outside the state and
	// runtime trees, and for the rootd UDS socket file + companion unit
	// file (rootd-skeleton).
	AuxFiles []string
	// Service unit / plist file path.
	ServiceUnitPath string
	// systemd service name (Linux) or launchd label (macOS).
	ServiceName string
	// Rootd companion service unit / plist file path + service name.
	// Disabled + removed alongside the main service so a fresh install
	// re-deploys both cleanly (rootd-skeleton). Empty means "no rootd
	// companion" — older Layouts without this field skip the bucket.
	RootdServiceUnitPath string
	RootdServiceName     string
	// OS user + group to delete at the end.
	UserName  string
	GroupName string
}

// DefaultLayout returns the production install layout for the given GOOS.
// `goos` must be "linux" or "darwin"; any other value returns a zero
// Layout (the executor will then no-op every bucket — safe but useless).
//
// #1017 bug 2 fix (chore/install-onecmd): the prior layout still referenced
// the pre-rename `borgee-helper` paths / user / service. Post-#1017 the
// distribution shipped as a single `borgee` binary + `borgee` system user
// + `borgee.service` unit; align the executor with the actual on-disk
// names. `HelperBinaries` is intentionally empty — `/usr/local/bin/borgee`
// (if present) is an npm shim symlink owned by npm, not the executor.
// `borgee uninstall-host` removes `/usr/local/lib/borgee/` whole, which is
// where `borgee install` deposits the persistent binary at
// `/usr/local/lib/borgee/bin/borgee` (see internal/cli/install). The
// RuntimeDir wipe in this executor therefore takes that path with it.
func DefaultLayout(goos string) Layout {
	u := currentUserLayout(goos)
	switch goos {
	case "linux":
		return Layout{
			StateDirs: []string{
				filepath.Join(u.StateRoot, "queue"),
				filepath.Join(u.StateRoot, "status"),
				filepath.Join(u.StateRoot, "audit-handoff"),
				filepath.Join(u.StateRoot, "credential"),
			},
			RuntimeDir:           filepath.Dir(filepath.Dir(u.BinaryPath)),
			HelperBinaries:       nil,
			ServiceUnitPath:      u.UserUnitPath,
			ServiceName:          "borgee.service",
			RootdServiceUnitPath: u.RootdServiceDst,
			RootdServiceName:     u.RootdService,
			AuxFiles: []string{
				u.RootdSocket,
			},
		}
	case "darwin":
		return Layout{
			StateDirs: []string{
				filepath.Join(u.StateRoot, "QueueState"),
				filepath.Join(u.StateRoot, "StatusState"),
				filepath.Join(u.StateRoot, "AuditHandoff"),
				filepath.Join(u.StateRoot, "credential"),
			},
			RuntimeDir:     filepath.Dir(filepath.Dir(u.BinaryPath)),
			HelperBinaries: nil,
			// Sandbox profile path (written by the internal setup helper
			// invoked from `borgee install`) lives outside
			// the runtime dir wipe; remove it explicitly so a fresh install
			// re-deploys a clean profile. rootd UDS socket file is also
			// listed so an uninstall-then-reinstall does not leave a stale
			// socket.
			AuxFiles: []string{
				"/Library/Application Support/Borgee/borgee-helper.sb",
				u.RootdSocket,
			},
			ServiceUnitPath:      u.UserUnitPath,
			ServiceName:          "cloud.borgee.host-bridge",
			RootdServiceUnitPath: u.RootdServiceDst,
			RootdServiceName:     u.RootdService,
		}
	default:
		return Layout{}
	}
}

type userLayout struct {
	UID             int
	GID             int
	HomeDir         string
	BinaryPath      string
	StateRoot       string
	UserUnitPath    string
	RootdSocket     string
	RootdService    string
	RootdServiceDst string
}

func currentUserLayout(goos string) userLayout {
	u, err := user.Current()
	home := "/root"
	uid := 0
	gid := 0
	if err == nil {
		home = u.HomeDir
		_, _ = fmt.Sscanf(u.Uid, "%d", &uid)
		_, _ = fmt.Sscanf(u.Gid, "%d", &gid)
	}
	switch goos {
	case "darwin":
		stateRoot := filepath.Join(home, "Library", "Application Support", "Borgee", "Helper")
		return userLayout{
			UID:             uid,
			GID:             gid,
			HomeDir:         home,
			BinaryPath:      filepath.Join(home, "Library", "Application Support", "Borgee", "bin", "borgee"),
			StateRoot:       stateRoot,
			UserUnitPath:    filepath.Join(home, "Library", "LaunchAgents", "cloud.borgee.host-bridge.plist"),
			RootdSocket:     filepath.Join("/Users/Shared/Borgee", fmt.Sprintf("%d", uid), "borgee-rootd.sock"),
			RootdService:    "cloud.borgee.host-bridge.rootd." + fmt.Sprintf("%d", uid),
			RootdServiceDst: "/Library/LaunchDaemons/cloud.borgee.host-bridge.rootd." + fmt.Sprintf("%d", uid) + ".plist",
		}
	default:
		stateRoot := filepath.Join(home, ".local", "state", "borgee")
		return userLayout{
			UID:             uid,
			GID:             gid,
			HomeDir:         home,
			BinaryPath:      filepath.Join(home, ".local", "share", "borgee", "bin", "borgee"),
			StateRoot:       stateRoot,
			UserUnitPath:    filepath.Join(home, ".config", "systemd", "user", "borgee.service"),
			RootdSocket:     filepath.Join("/run/borgee", fmt.Sprintf("%d", uid), "borgee-rootd.sock"),
			RootdService:    fmt.Sprintf("borgee-rootd-%d.service", uid),
			RootdServiceDst: filepath.Join("/etc/systemd/system", fmt.Sprintf("borgee-rootd-%d.service", uid)),
		}
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
	Buckets        []bucketResult `json:"buckets"`
	Platform       string         `json:"platform"`
	PreservedState bool           `json:"preserved_state"`
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

	// Bucket A2: disable the rootd companion service (if Layout has one).
	// rootd-skeleton: PR-1 ships borgee-rootd.service alongside the main
	// daemon; an uninstall must take both down or the next install would
	// trip on an already-bound UDS owned by the prior rootd process.
	if layout.RootdServiceName != "" {
		summary.Buckets = append(summary.Buckets, e.disableRootdService(ctx, layout))
	}

	// Bucket B: remove the service unit / plist file so a fresh install
	// re-deploys cleanly.
	summary.Buckets = append(summary.Buckets, e.removePath(layout.ServiceUnitPath, "service_unit"))

	// Bucket B2: remove the rootd companion unit / plist file.
	if layout.RootdServiceUnitPath != "" {
		summary.Buckets = append(summary.Buckets, e.removePath(layout.RootdServiceUnitPath, "rootd_service_unit"))
	}

	// Bucket C: remove the runtime binaries install-butler dropped under
	// /usr/local/lib/borgee/. Whole-tree wipe — operator opted into uninstall.
	if layout.RuntimeDir != "" {
		summary.Buckets = append(summary.Buckets, e.removeTree(layout.RuntimeDir, "runtime_dir"))
	}

	// Bucket D: remove the helper-shipped binaries. Post-#1017 this slice is
	// nil in DefaultLayout — `/usr/local/bin/borgee` is an npm shim symlink
	// not owned by the executor — but tests + future bundled distros may
	// still inject paths.
	for _, bin := range layout.HelperBinaries {
		summary.Buckets = append(summary.Buckets, e.removePath(bin, "helper_binary"))
	}

	// Bucket D2: additional auxiliary files (e.g. macOS sandbox profile)
	// that live outside the runtime / state trees and therefore need
	// explicit removal.
	for _, aux := range layout.AuxFiles {
		summary.Buckets = append(summary.Buckets, e.removePath(aux, "aux_file"))
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
	e.logf("borgee: uninstall summary: %s", string(summaryJSON))
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
	return e.disableServiceNamed(ctx, layout.ServiceName, "service_disable")
}

// disableRootdService is the same as disableService but for the rootd
// companion. Reports under a distinct bucket name ("rootd_service_disable")
// so an operator reading the summary can tell which service the per-step
// status belongs to.
func (e *Executor) disableRootdService(ctx context.Context, layout Layout) bucketResult {
	return e.disableServiceNamed(ctx, layout.RootdServiceName, "rootd_service_disable")
}

func (e *Executor) disableServiceNamed(ctx context.Context, serviceName, bucketName string) bucketResult {
	if serviceName == "" {
		return bucketResult{Name: bucketName, Status: "skipped", Detail: "no service name configured"}
	}
	var args []string
	var name string
	switch e.goos() {
	case "linux":
		name = "systemctl"
		args = []string{"disable", serviceName}
	case "darwin":
		name = "launchctl"
		args = []string{"disable", "system/" + serviceName}
	default:
		return bucketResult{Name: bucketName, Status: "skipped", Detail: "unsupported platform " + e.goos()}
	}
	if err := e.cmd().Run(ctx, name, args...); err != nil {
		e.logf("borgee: uninstall: %s %s failed: %v", name, strings.Join(args, " "), err)
		return bucketResult{Name: bucketName, Path: serviceName, Status: "failed", Detail: err.Error()}
	}
	return bucketResult{Name: bucketName, Path: serviceName, Status: "disabled"}
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
		e.logf("borgee: uninstall: remove %s failed: %v", path, err)
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
		e.logf("borgee: uninstall: removeAll %s failed: %v", path, err)
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
			e.logf("borgee: uninstall: userdel %s failed: %v", layout.UserName, err)
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
			e.logf("borgee: uninstall: dscl delete user %s failed: %v", layout.UserName, err)
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
