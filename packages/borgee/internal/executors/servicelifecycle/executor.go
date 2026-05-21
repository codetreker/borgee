//go:build linux || darwin

// Package servicelifecycle implements the `service.lifecycle` dispatcher
// executor (PR-4 #1033). The job runs INSIDE the `borgee daemon`
// (User=borgee, no root) but the operations (systemctl / launchctl)
// require root, so the executor delegates to the privileged `borgee
// rootd` companion via the rootdclient IPC.
//
// Lifecycle:
//
//	1. Parse the leased job's effective payload (just `operation`).
//	2. Read the manifest_binding to find the bound ServiceIDs (today the
//	   server emits exactly one — openclaw-user).
//	3. Read the signed manifest to look up that ServiceDeclaration —
//	   yields (Manager: systemd|launchd, Unit: <unit-name>).
//	4. Call rootdclient.ServiceLifecycle with (manager, unit, operation).
//	5. Map rootd's exit_code / stderr onto a TerminalStatus.
//
// SERVER-SIDE GAP (same as installplugin): the server does not yet
// emit the manifest JSON body itself. Until that wires up the executor
// fails-loud at the manifest decode step with manifest_invalid. The
// dispatcher's policy gate also rejects the job earlier today.
package servicelifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"borgee/internal/dispatch"
	"borgee/internal/jobpolicy"
	"borgee/internal/outbound"
	"borgee/internal/rootdclient"
)

// RootdLifecycle is the seam tests use to swap in a fake.
type RootdLifecycle interface {
	ServiceLifecycle(ctx context.Context, opts rootdclient.ServiceLifecycleRequest) (*rootdclient.ServiceLifecycleResponse, error)
}

// Executor implements dispatch.Executor for JobTypeServiceLifecycle.
type Executor struct {
	Rootd  RootdLifecycle
	Logger func(format string, v ...any)
}

func (e *Executor) logf(format string, v ...any) {
	if e.Logger != nil {
		e.Logger(format, v...)
	}
}

// Payload — the leased job's effective payload shape. Matches the
// server-side serviceLifecycleEffectivePayload.
type Payload struct {
	Operation string `json:"operation"`
}

func (e *Executor) Execute(ctx context.Context, job *outbound.LeasedJob) (dispatch.TerminalStatus, error) {
	if job == nil {
		return failed("schema_invalid", "nil leased job"), errors.New("servicelifecycle: nil job")
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return failed("schema_invalid", "payload decode: "+err.Error()), err
	}
	if !allowedOperation(payload.Operation) {
		return failed("schema_invalid", fmt.Sprintf("invalid operation %q", payload.Operation)), fmt.Errorf("invalid operation %q", payload.Operation)
	}
	if e.Rootd == nil {
		return failed("executor_error", "rootd client not configured"), errors.New("servicelifecycle: nil rootd")
	}

	manager, unit, err := resolveServiceFromManifest(job.ManifestJSON, job.ManifestBindingJSON)
	if err != nil {
		return failed(mapResolveErr(err), err.Error()), err
	}

	resp, err := e.Rootd.ServiceLifecycle(ctx, rootdclient.ServiceLifecycleRequest{
		Manager:   manager,
		Unit:      unit,
		Operation: payload.Operation,
	})
	if err != nil {
		e.logf("borgee: service.lifecycle %s %s failed: %v", payload.Operation, unit, err)
		return failed("service_denied", err.Error()), err
	}
	if resp == nil {
		return failed("executor_error", "rootd returned nil response"), errors.New("servicelifecycle: nil response")
	}
	if resp.ExitCode != 0 {
		msg := fmt.Sprintf("exit_code=%d stderr=%s", resp.ExitCode, resp.Stderr)
		return failed("service_denied", msg), fmt.Errorf("servicelifecycle: %s", msg)
	}
	return dispatch.TerminalStatus{
		Status: dispatch.StatusSucceeded,
		ResultSummary: outbound.ResultSummary{
			AuditRefs: []string{"service-lifecycle-" + payload.Operation + "-ok"},
			LogRefs:   []string{unit},
		},
	}, nil
}

func allowedOperation(op string) bool {
	switch op {
	case "start", "stop", "restart", "reload", "enable", "disable":
		return true
	default:
		return false
	}
}

// resolveServiceFromManifest reads the binding's first ServiceID + the
// matching manifest ServiceDeclaration, returning (manager, unit). The
// binding is server-issued so a future multi-service manifest just adds
// another ID — this code reads whichever ID the server pinned for this
// job.
func resolveServiceFromManifest(manifestJSON, bindingJSON json.RawMessage) (string, string, error) {
	if len(manifestJSON) == 0 {
		return "", "", errors.New("manifest_invalid: empty manifest")
	}
	if len(bindingJSON) == 0 {
		return "", "", errors.New("manifest_invalid: empty binding")
	}
	var binding jobpolicy.ManifestBinding
	if err := json.Unmarshal(bindingJSON, &binding); err != nil {
		return "", "", fmt.Errorf("binding_invalid: %w", err)
	}
	if len(binding.ServiceIDs) == 0 {
		return "", "", errors.New("manifest_missing_service_id")
	}
	wantID := binding.ServiceIDs[0]
	var manifest jobpolicy.PolicyManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return "", "", fmt.Errorf("manifest_invalid: %w", err)
	}
	for _, s := range manifest.Services {
		if s.ID == wantID {
			if s.Manager == "" || s.Unit == "" {
				return "", "", fmt.Errorf("service_invalid: id=%s declaration missing manager/unit", wantID)
			}
			return s.Manager, s.Unit, nil
		}
	}
	return "", "", fmt.Errorf("manifest_missing_service_id: %s not declared", wantID)
}

func mapResolveErr(err error) string {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "binding_invalid"):
		return "binding_invalid"
	case strings.HasPrefix(msg, "manifest_invalid"):
		return "manifest_invalid"
	case strings.HasPrefix(msg, "manifest_missing_service_id"):
		return "manifest_missing_service_id"
	case strings.HasPrefix(msg, "service_invalid"):
		return "service_invalid"
	default:
		return "manifest_invalid"
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
