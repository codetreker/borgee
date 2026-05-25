//go:build linux || darwin

// Package openclawconfigure implements the `openclaw.configure_agent`
// dispatcher executor (PR-3 #1041). The job runs INSIDE the existing
// `borgee daemon` (User=borgee, no root) — it writes the per-agent
// effective config record under the manifest-resolved root for the
// `openclaw_agent_config` PathID carried in the leased job's binding.
//
// Path resolution: manifestpath.Resolve(manifest_json, manifest_binding_json,
// openclaw_agent_config) → absolute root → safe-join with `<agent_id>.json`.
// NO daemon-startup flag controls this root; the server is the authority
// (server-go's helperJobOpenClawAgentConfigPathID names the same PathID).
package openclawconfigure

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
// helperJobOpenClawAgentConfigPathID.
const PathID = "openclaw_agent_config"

// Payload mirrors the server's openClawEffectivePayload + the
// jobpolicy.validatePayload shape for openclaw.configure_agent.
type Payload struct {
	AgentID             string `json:"agent_id"`
	ChannelID           string `json:"channel_id,omitempty"`
	ConfigSchemaVersion int64  `json:"config_schema_version"`
	ConfigHash          string `json:"config_hash"`
}

// Executor implements dispatch.Executor for
// job_type=openclaw.configure_agent.
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
// atomically writes the per-agent record.
func (e *Executor) Execute(_ context.Context, job *outbound.LeasedJob) (dispatch.TerminalStatus, error) {
	if job == nil {
		return failed("schema_invalid", "nil leased job"), errors.New("openclawconfigure: nil job")
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return failed("schema_invalid", "payload decode: "+err.Error()), err
	}
	if strings.TrimSpace(payload.AgentID) == "" {
		return failed("schema_invalid", "empty agent_id"), errors.New("openclawconfigure: empty agent_id")
	}
	if !validAgentID(payload.AgentID) {
		return failed("schema_invalid", fmt.Sprintf("invalid agent_id %q", payload.AgentID)), fmt.Errorf("invalid agent_id %q", payload.AgentID)
	}
	if payload.ConfigSchemaVersion <= 0 {
		return failed("schema_invalid", "config_schema_version must be > 0"), errors.New("openclawconfigure: bad schema version")
	}
	if !strings.HasPrefix(payload.ConfigHash, "sha256:") {
		return failed("schema_invalid", "config_hash must be sha256:-prefixed"), errors.New("openclawconfigure: bad config_hash")
	}

	resolved, err := manifestpath.Resolve(job.ManifestJSON, job.ManifestBindingJSON, PathID)
	if err != nil {
		return failed(mapResolveError(err), err.Error()), err
	}
	dest, err := manifestpath.JoinUnderResolved(resolved, payload.AgentID+".json")
	if err != nil {
		return failed("path_escape", err.Error()), err
	}

	record := map[string]any{
		"agent_id":              payload.AgentID,
		"channel_id":            payload.ChannelID,
		"config_schema_version": payload.ConfigSchemaVersion,
		"config_hash":           payload.ConfigHash,
		"applied_at":            e.now().UTC().Format(time.RFC3339),
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return failed("encode_failed", err.Error()), err
	}

	if err := atomicWrite(dest, raw); err != nil {
		e.logf("borgee: openclaw.configure_agent write %s failed: %v", dest, err)
		return failed("write_failed", err.Error()), err
	}

	return dispatch.TerminalStatus{
		Status: dispatch.StatusSucceeded,
		ResultSummary: outbound.ResultSummary{
			AuditRefs: []string{"openclaw-configure-agent-ok"},
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

// validAgentID rejects path-escape on agent_id (used as filename segment).
func validAgentID(id string) bool {
	if id == "" || id == "." || id == ".." || strings.Contains(id, "..") {
		return false
	}
	if strings.ContainsAny(id, "/\\\x00") {
		return false
	}
	for _, r := range id {
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
