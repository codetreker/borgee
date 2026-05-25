//go:build linux || darwin

// Package pluginconfigure implements the
// `borgee_plugin.configure_connection` dispatcher executor (PR-3 #1041).
// The job runs INSIDE the existing `borgee daemon` (User=borgee, no root)
// — it writes the per-connection record under the manifest-resolved root
// for the `borgee_plugin_config` PathID carried in the leased job's
// binding.
//
// Path resolution: manifestpath.Resolve(manifest_json, manifest_binding_json,
// borgee_plugin_config) → absolute root → safe-join with the connection_id
// suffix. NO daemon-startup flag controls this root; the server is the
// authority (server-go's helperJobBorgeePluginConfigPathID names the same
// PathID).
package pluginconfigure

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
// helperJobBorgeePluginConfigPathID.
const PathID = "borgee_plugin_config"

const connectionIDPrefix = "borgee-plugin:"

// Payload mirrors the server's borgeePluginEffectivePayload.
type Payload struct {
	ConnectionID string `json:"connection_id"`
	AgentID      string `json:"agent_id"`
	ChannelID    string `json:"channel_id"`
}

// Executor implements dispatch.Executor for
// job_type=borgee_plugin.configure_connection.
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
// writes the per-connection record atomically.
func (e *Executor) Execute(_ context.Context, job *outbound.LeasedJob) (dispatch.TerminalStatus, error) {
	if job == nil {
		return failed("schema_invalid", "nil leased job"), errors.New("pluginconfigure: nil job")
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return failed("schema_invalid", "payload decode: "+err.Error()), err
	}
	if !strings.HasPrefix(payload.ConnectionID, connectionIDPrefix) {
		return failed("schema_invalid", fmt.Sprintf("connection_id must start with %q", connectionIDPrefix)), errors.New("pluginconfigure: bad connection_id prefix")
	}
	suffix := strings.TrimPrefix(payload.ConnectionID, connectionIDPrefix)
	if !validSuffix(suffix) {
		return failed("schema_invalid", fmt.Sprintf("invalid connection_id suffix %q", suffix)), fmt.Errorf("invalid connection_id suffix %q", suffix)
	}
	if strings.TrimSpace(payload.AgentID) == "" {
		return failed("schema_invalid", "empty agent_id"), errors.New("pluginconfigure: empty agent_id")
	}
	if strings.TrimSpace(payload.ChannelID) == "" {
		return failed("schema_invalid", "empty channel_id"), errors.New("pluginconfigure: empty channel_id")
	}

	resolved, err := manifestpath.Resolve(job.ManifestJSON, job.ManifestBindingJSON, PathID)
	if err != nil {
		return failed(mapResolveError(err), err.Error()), err
	}
	dest, err := manifestpath.JoinUnderResolved(resolved, suffix+".json")
	if err != nil {
		return failed("path_escape", err.Error()), err
	}

	record := map[string]any{
		"connection_id": payload.ConnectionID,
		"agent_id":      payload.AgentID,
		"channel_id":    payload.ChannelID,
		"applied_at":    e.now().UTC().Format(time.RFC3339),
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return failed("encode_failed", err.Error()), err
	}

	if err := atomicWrite(dest, raw); err != nil {
		e.logf("borgee: borgee_plugin.configure_connection write %s failed: %v", dest, err)
		return failed("write_failed", err.Error()), err
	}

	return dispatch.TerminalStatus{
		Status: dispatch.StatusSucceeded,
		ResultSummary: outbound.ResultSummary{
			AuditRefs: []string{"borgee-plugin-configure-connection-ok"},
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

var _ dispatch.Executor = (*Executor)(nil)
