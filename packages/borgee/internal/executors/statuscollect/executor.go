//go:build linux || darwin

// Package statuscollect implements the `status.collect` dispatcher executor
// (PR-3 #1041). The job runs INSIDE the existing `borgee daemon`
// (User=borgee, no root) — it gathers read-only system info and returns
// the snapshot via outbound.ResultSummary. NO filesystem write happens
// here: status.collect is a "read + report" job, not a "write a cache"
// job. The server is the authority for status persistence.
//
// Payload schema (matches jobpolicy.validatePayload for JobTypeStatusCollect):
//
//	{ "scope": "helper" | "openclaw" | "service" }
//
// Behavior:
//   - "helper":   collect borgee daemon process facts (PID, GOOS,
//                 GOARCH, runtime version, executable path).
//   - "openclaw": collect any installed-versions snapshot present at
//                 InstalledVersionsPath (best-effort, missing file is
//                 reported as "absent" not failed).
//   - "service":  read uptime / boot-time (Linux: /proc/uptime; darwin:
//                 placeholder — best-effort, not load-bearing).
package statuscollect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"borgee/internal/dispatch"
	"borgee/internal/outbound"
)

// Payload mirrors the jobpolicy.validatePayload `status.collect` shape.
type Payload struct {
	Scope string `json:"scope"`
}

// Executor implements dispatch.Executor for job_type=status.collect.
type Executor struct {
	// InstalledVersionsPath is the install-butler-maintained snapshot
	// (#999). Empty = scope=openclaw reports `unconfigured`.
	InstalledVersionsPath string

	// Now overrides time.Now for tests.
	Now func() time.Time

	// Logger lets tests capture log lines.
	Logger func(format string, v ...any)
}

func (e *Executor) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func (e *Executor) logf(format string, v ...any) {
	if e.Logger != nil {
		e.Logger(format, v...)
	}
}

// Execute gathers the requested scope and returns the snapshot in the
// terminal result summary. No filesystem write.
func (e *Executor) Execute(_ context.Context, job *outbound.LeasedJob) (dispatch.TerminalStatus, error) {
	if job == nil {
		return failed("schema_invalid", "nil leased job"), errors.New("statuscollect: nil job")
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return failed("schema_invalid", "payload decode: "+err.Error()), err
	}
	switch payload.Scope {
	case "helper", "openclaw", "service":
	default:
		return failed("schema_invalid", fmt.Sprintf("invalid scope %q", payload.Scope)), fmt.Errorf("invalid scope %q", payload.Scope)
	}

	snapshot := e.collect(payload.Scope)
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return failed("encode_failed", err.Error()), err
	}
	e.logf("borgee: status.collect scope=%s bytes=%d", payload.Scope, len(encoded))

	return dispatch.TerminalStatus{
		Status: dispatch.StatusSucceeded,
		ResultSummary: outbound.ResultSummary{
			AuditRefs: []string{fmt.Sprintf("status-collect-%s-ok", payload.Scope)},
			LogRefs:   []string{string(encoded)},
		},
	}, nil
}

// collect gathers the requested scope. All errors are folded into the
// returned snapshot so the executor never aborts the whole job over a
// single missing source.
func (e *Executor) collect(scope string) map[string]any {
	out := map[string]any{
		"scope":        scope,
		"collected_at": e.now().UTC().Format(time.RFC3339),
		"platform":     runtime.GOOS,
	}
	switch scope {
	case "helper":
		exe, _ := os.Executable()
		out["pid"] = os.Getpid()
		out["goos"] = runtime.GOOS
		out["goarch"] = runtime.GOARCH
		out["go_version"] = runtime.Version()
		out["executable"] = exe
	case "openclaw":
		if strings.TrimSpace(e.InstalledVersionsPath) == "" {
			out["installed_versions"] = nil
			out["installed_versions_status"] = "unconfigured"
		} else if raw, err := os.ReadFile(e.InstalledVersionsPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				out["installed_versions"] = nil
				out["installed_versions_status"] = "absent"
			} else {
				out["installed_versions"] = nil
				out["installed_versions_status"] = "read_failed"
				out["installed_versions_error"] = err.Error()
			}
		} else {
			var parsed any
			if err := json.Unmarshal(raw, &parsed); err == nil {
				out["installed_versions"] = parsed
				out["installed_versions_status"] = "ok"
			} else {
				out["installed_versions_status"] = "decode_failed"
				out["installed_versions_error"] = err.Error()
			}
		}
	case "service":
		if runtime.GOOS == "linux" {
			if raw, err := os.ReadFile("/proc/uptime"); err == nil {
				fields := strings.Fields(string(raw))
				if len(fields) > 0 {
					out["uptime_seconds"] = fields[0]
				}
			}
		}
	}
	return out
}

func failed(code, msg string) dispatch.TerminalStatus {
	return dispatch.TerminalStatus{
		Status:         dispatch.StatusFailed,
		FailureCode:    code,
		FailureMessage: msg,
	}
}

var _ dispatch.Executor = (*Executor)(nil)
