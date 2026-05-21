//go:build linux || darwin

// Package delegationrevoke implements the `delegation.revoke` dispatcher
// executor (PR-4 #1033). The job runs INSIDE the `borgee daemon`
// (User=borgee, no root). The executor:
//
//	1. Validates the leased payload (target_category is a non-empty
//	   allowed category string).
//	2. Calls rootdclient.DelegationRevoke to disable borgee.service
//	   (so systemd does not respawn after the daemon exits) and to wipe
//	   the credential / enrollment-id / device-id files.
//	3. Returns dispatch.StatusSucceeded so the dispatcher emits a
//	   terminal Result frame over WS BEFORE any process teardown.
//
// Self-shutdown safety: the executor does NOT call os.Exit or
// signal.Kill. The "no self-stop signal" pattern mirrors
// executors/uninstall — the dispatcher MUST flush its terminal Result
// over WS before the daemon process dies. The actual daemon exit
// happens via the systemd disable side-effect: once borgee.service is
// disabled and the operator (or the next reboot) stops the unit,
// systemd will not respawn. For an immediate shutdown the executor
// signals the daemon's outer loop via the DaemonShutdown callback (if
// wired). Today that callback is nil in production; a follow-up
// observability PR will land it.
package delegationrevoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"borgee/internal/dispatch"
	"borgee/internal/outbound"
	"borgee/internal/rootdclient"
)

// DefaultLayout returns the production credential paths + service name
// for the running platform. Mirrors executors/uninstall.DefaultLayout
// for the same files; revoke is "uninstall light" — it removes the
// credential trio + disables the unit, but keeps binaries and state
// directories so a re-enrollment can fast-path.
type Layout struct {
	ServiceName     string
	ServiceManager  string
	CredentialPaths []string
}

func DefaultLayout(goos string) Layout {
	switch goos {
	case "linux":
		return Layout{
			ServiceName:    "borgee.service",
			ServiceManager: "systemd",
			CredentialPaths: []string{
				"/var/lib/borgee/credential/credential",
				"/var/lib/borgee/credential/enrollment-id",
				"/var/lib/borgee/credential/device-id",
			},
		}
	case "darwin":
		return Layout{
			ServiceName:    "cloud.borgee.host-bridge",
			ServiceManager: "launchd",
			CredentialPaths: []string{
				"/Library/Application Support/Borgee/Helper/credential/credential",
				"/Library/Application Support/Borgee/Helper/credential/enrollment-id",
				"/Library/Application Support/Borgee/Helper/credential/device-id",
			},
		}
	default:
		return Layout{}
	}
}

// RootdRevoker is the seam tests use to swap in a fake.
type RootdRevoker interface {
	DelegationRevoke(ctx context.Context, opts rootdclient.DelegationRevokeRequest) (*rootdclient.DelegationRevokeResponse, error)
}

// Dispatcher is the seam for cooperative drain. The production
// dispatcher implements Drain(ctx, timeout) — tests can pass nil to
// skip the drain step.
type Dispatcher interface {
	Drain(ctx context.Context, timeout time.Duration) error
}

// Executor implements dispatch.Executor for JobTypeDelegationRevoke.
type Executor struct {
	Rootd        RootdRevoker
	Dispatcher   Dispatcher
	Layout       Layout
	GOOS         string
	DrainTimeout time.Duration
	Logger       func(format string, v ...any)
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

// Payload — the leased job's effective payload. Matches the server-side
// delegationRevokeEffectivePayload.
type Payload struct {
	TargetCategory string `json:"target_category"`
}

func (e *Executor) Execute(ctx context.Context, job *outbound.LeasedJob) (dispatch.TerminalStatus, error) {
	if job == nil {
		return failed("schema_invalid", "nil leased job"), errors.New("delegationrevoke: nil job")
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return failed("schema_invalid", "payload decode: "+err.Error()), err
	}
	if !allowedCategory(payload.TargetCategory) {
		return failed("schema_invalid", fmt.Sprintf("invalid target_category %q", payload.TargetCategory)), fmt.Errorf("invalid category %q", payload.TargetCategory)
	}
	if e.Rootd == nil {
		return failed("executor_error", "rootd client not configured"), errors.New("delegationrevoke: nil rootd")
	}

	layout := e.Layout
	if layout.ServiceName == "" && len(layout.CredentialPaths) == 0 {
		layout = DefaultLayout(e.goos())
	}
	for _, p := range layout.CredentialPaths {
		if !filepath.IsAbs(p) {
			return failed("policy_denied", fmt.Sprintf("non-absolute credential path %q", p)), fmt.Errorf("non-absolute path %q", p)
		}
	}

	// Drain in-flight jobs cooperatively before asking rootd to wipe
	// the credential. If the drain stalls past DrainTimeout we still
	// proceed — the goal is to make a best-effort attempt to flush
	// in-flight Results before the credential disappears.
	if e.Dispatcher != nil {
		drainDeadline := e.drainTimeout()
		if err := e.Dispatcher.Drain(ctx, drainDeadline); err != nil {
			e.logf("borgee: delegation.revoke drain timed out: %v", err)
		}
	}

	resp, err := e.Rootd.DelegationRevoke(ctx, rootdclient.DelegationRevokeRequest{
		EnrollmentID:        job.EnrollmentID,
		DrainTimeoutSeconds: int(e.drainTimeout().Seconds()),
		ServiceName:         layout.ServiceName,
		ServiceManager:      layout.ServiceManager,
		CredentialPaths:     layout.CredentialPaths,
	})
	if err != nil {
		e.logf("borgee: delegation.revoke rootd err: %v", err)
		return failed("execution_failed", err.Error()), err
	}
	if resp == nil || !resp.CredentialWiped {
		return failed("execution_failed", "rootd did not wipe credential"), errors.New("delegationrevoke: credential not wiped")
	}
	e.logf("borgee: delegation.revoke wiped %d paths disabled=%v", len(resp.WipedPaths), resp.Disabled)
	return dispatch.TerminalStatus{
		Status: dispatch.StatusSucceeded,
		ResultSummary: outbound.ResultSummary{
			AuditRefs: []string{
				fmt.Sprintf("delegation-revoke-%s-wiped-%d-disabled-%t", payload.TargetCategory, len(resp.WipedPaths), resp.Disabled),
			},
		},
	}, nil
}

func (e *Executor) drainTimeout() time.Duration {
	if e.DrainTimeout > 0 {
		return e.DrainTimeout
	}
	return 5 * time.Second
}

func allowedCategory(category string) bool {
	switch strings.TrimSpace(category) {
	case "openclaw_config", "openclaw_lifecycle", "status_collect", "helper_lifecycle":
		return true
	default:
		return false
	}
}

func failed(code, msg string) dispatch.TerminalStatus {
	return dispatch.TerminalStatus{
		Status:         dispatch.StatusFailed,
		FailureCode:    code,
		FailureMessage: msg,
	}
}

var _ dispatch.Executor = (*Executor)(nil)
