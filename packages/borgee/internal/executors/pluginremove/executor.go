//go:build linux || darwin

// Package pluginremove implements the
// `borgee_plugin.remove_connection` dispatcher executor (#1049).
// The job runs INSIDE the existing `borgee daemon` (User=borgee, no root)
// — it removes the per-connection record under the manifest-resolved root
// for the `borgee_plugin_config` PathID carried in the leased job's
// binding. Removing a non-existent connection_id is idempotent (success).
package pluginremove

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"borgee/internal/dispatch"
	"borgee/internal/executors/manifestpath"
	"borgee/internal/outbound"
)

// PathID — manifest path id this executor writes under. Mirrors server-side
// helperJobBorgeePluginConfigPathID and pluginconfigure.PathID.
const PathID = "borgee_plugin_config"

const connectionIDPrefix = "borgee-plugin:"

// Payload mirrors the server's borgeePluginRemoveEffectivePayload.
type Payload struct {
	ConnectionID string `json:"connection_id"`
	AgentID      string `json:"agent_id"`
}

// Executor implements dispatch.Executor for
// job_type=borgee_plugin.remove_connection.
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

// Execute resolves the manifest-declared root, validates payload, and
// removes the per-connection record. Missing file = idempotent success.
func (e *Executor) Execute(_ context.Context, job *outbound.LeasedJob) (dispatch.TerminalStatus, error) {
	if job == nil {
		return failed("schema_invalid", "nil leased job"), errors.New("pluginremove: nil job")
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return failed("schema_invalid", "payload decode: "+err.Error()), err
	}
	if !strings.HasPrefix(payload.ConnectionID, connectionIDPrefix) {
		return failed("schema_invalid", fmt.Sprintf("connection_id must start with %q", connectionIDPrefix)), errors.New("pluginremove: bad connection_id prefix")
	}
	suffix := strings.TrimPrefix(payload.ConnectionID, connectionIDPrefix)
	if !validSuffix(suffix) {
		return failed("schema_invalid", fmt.Sprintf("invalid connection_id suffix %q", suffix)), fmt.Errorf("invalid connection_id suffix %q", suffix)
	}
	if strings.TrimSpace(payload.AgentID) == "" {
		return failed("schema_invalid", "empty agent_id"), errors.New("pluginremove: empty agent_id")
	}

	resolved, err := manifestpath.Resolve(job.ManifestJSON, job.ManifestBindingJSON, PathID)
	if err != nil {
		return failed(mapResolveError(err), err.Error()), err
	}
	dest, err := manifestpath.JoinUnderResolved(resolved, suffix+".json")
	if err != nil {
		return failed("path_escape", err.Error()), err
	}

	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		e.logf("borgee: borgee_plugin.remove_connection remove %s failed: %v", dest, err)
		return failed("remove_failed", err.Error()), err
	}

	return dispatch.TerminalStatus{
		Status: dispatch.StatusSucceeded,
		ResultSummary: outbound.ResultSummary{
			AuditRefs: []string{"borgee-plugin-remove-connection-ok"},
			LogRefs:   []string{filepath.Base(dest)},
		},
	}, nil
}

func mapResolveError(err error) string {
	switch {
	case errors.Is(err, manifestpath.ErrPathIDNotInBinding), errors.Is(err, manifestpath.ErrPathIDNotInManifest):
		return "manifest_missing_path_id"
	case errors.Is(err, manifestpath.ErrManifestParse), errors.Is(err, manifestpath.ErrPathNotAbsolute):
		return "manifest_invalid"
	case errors.Is(err, manifestpath.ErrBindingParse):
		return "binding_invalid"
	case errors.Is(err, manifestpath.ErrPathEscape):
		return "path_escape"
	default:
		return "manifest_invalid"
	}
}

func validSuffix(s string) bool {
	if s == "" || s == "." || s == ".." || strings.Contains(s, "..") {
		return false
	}
	if strings.ContainsAny(s, "/\\\x00") {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_' || r == '.':
			continue
		default:
			return false
		}
	}
	return true
}

func failed(code, msg string) dispatch.TerminalStatus {
	return dispatch.TerminalStatus{
		Status:         dispatch.StatusFailed,
		FailureCode:    code,
		FailureMessage: msg,
	}
}

// silence unused-import warning when Now() is not invoked in some paths.
var _ = time.Now

var _ dispatch.Executor = (*Executor)(nil)
