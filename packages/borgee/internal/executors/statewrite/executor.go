//go:build linux || darwin

// Package statewrite implements the `state.write` dispatcher executor
// (PR-3 #1041). The job runs INSIDE the existing `borgee daemon`
// (User=borgee, no root) — it writes to the manifest-resolved root for
// the `borgee_state_config` PathID carried in the leased job's binding.
//
// Path resolution: manifestpath.Resolve(manifest_json, manifest_binding_json,
// borgee_state_config) → absolute root → safe-join with the payload's
// state_key. NO daemon-startup flag controls this root; the server is the
// authority for where state is written.
//
// Server-side gap: as of #1041 the server's helper_job_queries.go does
// NOT yet bind `borgee_state_config` for HelperJobTypeStateWrite (the
// state.write job type itself is not enumerated server-side). Until that
// gap closes (separate issue), this executor is plumbing-only and will
// fail-loud `manifest_missing_path_id` on any leased state.write job.
package statewrite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"borgee/internal/dispatch"
	"borgee/internal/executors/manifestpath"
	"borgee/internal/outbound"
)

// PathID — manifest path id this executor writes under. Mirror of the
// server-side helperJobBorgeeStateConfigPathID constant once that lands.
const PathID = "borgee_state_config"

// Payload mirrors the jobpolicy.validatePayload `state.write` shape.
type Payload struct {
	StateKey    string `json:"state_key"`
	ValueSHA256 string `json:"value_sha256,omitempty"`
}

// Executor implements dispatch.Executor for job_type=state.write.
type Executor struct {
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

// Execute resolves the state root from the manifest binding, safe-joins
// with the payload state_key, and atomically writes the metadata file.
func (e *Executor) Execute(_ context.Context, job *outbound.LeasedJob) (dispatch.TerminalStatus, error) {
	if job == nil {
		return failed("schema_invalid", "nil leased job"), errors.New("statewrite: nil job")
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return failed("schema_invalid", "payload decode: "+err.Error()), err
	}
	if strings.TrimSpace(payload.StateKey) == "" {
		return failed("schema_invalid", "empty state_key"), errors.New("statewrite: empty state_key")
	}

	resolved, err := manifestpath.Resolve(job.ManifestJSON, job.ManifestBindingJSON, PathID)
	if err != nil {
		return failed(mapResolveError(err), err.Error()), err
	}
	dest, err := manifestpath.JoinUnderResolved(resolved, payload.StateKey+".json")
	if err != nil {
		return failed("path_escape", err.Error()), err
	}

	content := map[string]any{
		"state_key":    payload.StateKey,
		"value_sha256": payload.ValueSHA256,
		"written_at":   e.now().UTC().Format(time.RFC3339),
	}
	raw, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return failed("encode_failed", err.Error()), err
	}

	if err := atomicWrite(dest, raw); err != nil {
		e.logf("borgee: state.write write %s failed: %v", dest, err)
		return failed("write_failed", err.Error()), err
	}

	return dispatch.TerminalStatus{
		Status: dispatch.StatusSucceeded,
		ResultSummary: outbound.ResultSummary{
			AuditRefs: []string{"state-write-ok"},
			LogRefs:   []string{filepath.Base(dest)},
		},
	}, nil
}

func mapResolveError(err error) string {
	switch {
	case errors.Is(err, manifestpath.ErrPathIDNotInBinding), errors.Is(err, manifestpath.ErrPathIDNotInManifest):
		return "manifest_missing_path_id"
	case errors.Is(err, manifestpath.ErrManifestParse):
		return "manifest_invalid"
	case errors.Is(err, manifestpath.ErrBindingParse):
		return "binding_invalid"
	case errors.Is(err, manifestpath.ErrPathNotAbsolute):
		return "manifest_invalid"
	case errors.Is(err, manifestpath.ErrPathEscape):
		return "path_escape"
	default:
		return "manifest_invalid"
	}
}

// atomicWrite writes `data` to `dest` via tempfile + rename in the same dir.
func atomicWrite(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(dest)+".tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o640); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func failed(code, msg string) dispatch.TerminalStatus {
	return dispatch.TerminalStatus{
		Status:         dispatch.StatusFailed,
		FailureCode:    code,
		FailureMessage: msg,
	}
}

// silence unused-import linters when manifestpath errors are matched only via errors.Is.

var _ dispatch.Executor = (*Executor)(nil)
