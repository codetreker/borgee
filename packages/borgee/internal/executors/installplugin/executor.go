//go:build linux || darwin

// Package installplugin implements the `openclaw.install_from_manifest`
// dispatcher executor (PR-4 #1033). The job runs INSIDE the
// `borgee daemon` (User=borgee, no root) but writes happen under root-
// owned directories (/usr/local/lib/borgee/, etc), so the executor
// delegates the actual fetch+verify+place sequence to the privileged
// `borgee rootd` companion via the rootdclient IPC.
//
// Lifecycle:
//
//	1. Parse the leased job's payload + manifest_binding.
//	2. Resolve `openclaw_install` PathID from the binding → manifest
//	   PathDeclaration (real filesystem root + write mode).
//	3. Read the `openclaw-plugin` artifact + the bound origin from
//	   the same manifest to get (binary_url, sha256).
//	4. Call rootdclient.InstallPlugin with the manifest URL + pubkey +
//	   plugin_id + resolved target path. rootd invokes install-butler
//	   in-process and reports back.
//	5. Map rootd's reason:detail line onto a TerminalStatus failure_code.
//
// SERVER-SIDE GAP (#1033 follow-up): as of PR-4 the server's leased-job
// emission does NOT include the manifest JSON body itself — only
// manifest_digest + manifest_binding_json. The helper-side jobpolicy.
// Evaluate runs at the dispatcher gate BEFORE the executor; today it
// rejects manifest-required jobs because the trust root + manifest JSON
// are absent. This executor therefore reaches Execute only in tests
// with hand-rolled inputs; in production it fails-loud at the policy
// gate. A follow-up PR will wire manifest emission + trust root
// distribution. The executor's contract is correct now so when that
// wires up, no executor change is needed.
package installplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"borgee/internal/dispatch"
	"borgee/internal/jobpolicy"
	"borgee/internal/outbound"
	"borgee/internal/rootdclient"
)

// RequiredArtifactID + RequiredPathID — mirror the server-side binding
// constants (openClawPluginArtifactID + openClawInstallPathID). Kept
// here so the executor's contract is readable without crossing the
// helper/server package boundary.
const (
	RequiredArtifactID = "openclaw-plugin"
	RequiredPathID     = "openclaw_install"
)

// RootdInstaller is the seam tests use to swap in a fake. Production
// callers wire to *rootdclient.Client.
type RootdInstaller interface {
	InstallPlugin(ctx context.Context, opts rootdclient.InstallPluginRequest) (*rootdclient.InstallPluginResponse, error)
}

// Executor implements dispatch.Executor for
// JobTypeOpenClawInstallFromManifest.
type Executor struct {
	Rootd RootdInstaller
	// PubKeyBase64 is the ed25519 trust root for plugin manifest
	// signature verification. Today this is wired from the daemon's
	// startup env (BORGEE_MANIFEST_SIGNING_PUBKEY); follow-up wires it
	// from the leased-job manifest body once that emission lands.
	PubKeyBase64 string
	// Logger lets tests capture log lines.
	Logger func(format string, v ...any)
}

func (e *Executor) logf(format string, v ...any) {
	if e.Logger != nil {
		e.Logger(format, v...)
	}
}

// Payload — the leased job's effective payload shape. Matches the
// server-side openClawInstallEffectivePayload.
type Payload struct {
	InstallPlanID string `json:"install_plan_id"`
}

func (e *Executor) Execute(ctx context.Context, job *outbound.LeasedJob) (dispatch.TerminalStatus, error) {
	if job == nil {
		return failed("schema_invalid", "nil leased job"), errors.New("installplugin: nil job")
	}
	var payload Payload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return failed("schema_invalid", "payload decode: "+err.Error()), err
	}
	if strings.TrimSpace(payload.InstallPlanID) == "" {
		return failed("schema_invalid", "empty install_plan_id"), errors.New("installplugin: empty install_plan_id")
	}
	if e.Rootd == nil {
		return failed("executor_error", "rootd client not configured"), errors.New("installplugin: nil rootd")
	}
	if strings.TrimSpace(e.PubKeyBase64) == "" {
		return failed("policy_denied", "manifest trust root not configured"), errors.New("installplugin: empty PubKeyBase64")
	}

	target, manifestURL, err := resolveTargetAndManifestURL(job.ManifestJSON, job.ManifestBindingJSON)
	if err != nil {
		return failed(mapResolveErr(err), err.Error()), err
	}
	plugin := pluginIDFromInstallPlan(payload.InstallPlanID)
	dest := filepath.Join(target, plugin)
	req := rootdclient.InstallPluginRequest{
		ManifestURL:  manifestURL,
		PubKeyBase64: e.PubKeyBase64,
		PluginID:     plugin,
		TargetPath:   dest,
	}
	resp, err := e.Rootd.InstallPlugin(ctx, req)
	if err != nil {
		code := mapRootdErr(err)
		e.logf("borgee: install_plugin failed plugin=%s target=%s err=%v", plugin, dest, err)
		return failed(code, err.Error()), err
	}
	if resp == nil {
		return failed("executor_error", "rootd returned nil response"), errors.New("installplugin: nil response")
	}
	if !resp.Installed {
		return failed("execution_failed", "rootd reported not installed: "+resp.StderrSummary), errors.New("installplugin: not installed")
	}
	return dispatch.TerminalStatus{
		Status: dispatch.StatusSucceeded,
		ResultSummary: outbound.ResultSummary{
			AuditRefs: []string{"install-plugin-" + plugin + "-ok"},
			LogRefs:   []string{filepath.Base(dest)},
		},
	}, nil
}

// pluginIDFromInstallPlan maps the server-emitted install_plan_id
// ("openclaw-plugin-v1") back to the plugin_id rootd's whitelist regex
// expects ("openclaw-plugin"). Today only one plan exists; a future
// multi-plan world adds entries here.
func pluginIDFromInstallPlan(planID string) string {
	switch planID {
	case "openclaw-plugin-v1":
		return "openclaw-plugin"
	default:
		// Fail loud at rootd's regex gate rather than silently routing
		// a typo through. The returned string preserves the caller's
		// intent for the error message.
		return planID
	}
}

// resolveTargetAndManifestURL parses the signed manifest + binding to
// produce (target_install_dir, manifest_url). The manifest URL is the
// first bound domain — manifest-signing.md guarantees it carries the
// JSON manifest endpoint, not an artifact mirror.
func resolveTargetAndManifestURL(manifestJSON, bindingJSON json.RawMessage) (string, string, error) {
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
	if !contains(binding.PathIDs, RequiredPathID) {
		return "", "", fmt.Errorf("manifest_missing_path_id: %s", RequiredPathID)
	}
	if !contains(binding.ArtifactIDs, RequiredArtifactID) {
		return "", "", fmt.Errorf("manifest_missing_artifact_id: %s", RequiredArtifactID)
	}
	if len(binding.Domains) == 0 {
		return "", "", errors.New("manifest_missing_domain")
	}
	var manifest jobpolicy.PolicyManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return "", "", fmt.Errorf("manifest_invalid: %w", err)
	}
	var target string
	for _, p := range manifest.Paths {
		if p.ID == RequiredPathID {
			target = p.Root
			break
		}
	}
	if target == "" {
		return "", "", fmt.Errorf("manifest_missing_path_id: %s not declared", RequiredPathID)
	}
	if !filepath.IsAbs(target) {
		return "", "", errors.New("manifest_invalid: target path not absolute")
	}
	return target, binding.Domains[0], nil
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func mapResolveErr(err error) string {
	switch {
	case strings.HasPrefix(err.Error(), "binding_invalid"):
		return "binding_invalid"
	case strings.HasPrefix(err.Error(), "manifest_invalid"):
		return "manifest_invalid"
	case strings.HasPrefix(err.Error(), "manifest_missing_path_id"):
		return "manifest_missing_path_id"
	case strings.HasPrefix(err.Error(), "manifest_missing_artifact_id"):
		return "manifest_missing_artifact_id"
	case strings.HasPrefix(err.Error(), "manifest_missing_domain"):
		return "manifest_missing_domain"
	default:
		return "manifest_invalid"
	}
}

// mapRootdErr maps rootd's reason:detail prefix to a server-known
// failure code. Anything install-butler can emit gets passed through.
func mapRootdErr(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "manifest_fetch_failed"):
		return "manifest_fetch_failed"
	case strings.Contains(msg, "manifest_parse_failed"):
		return "manifest_parse_failed"
	case strings.Contains(msg, "plugin_not_found"):
		return "plugin_not_found"
	case strings.Contains(msg, "signature_invalid"):
		return "signature_invalid"
	case strings.Contains(msg, "binary_fetch_failed"):
		return "binary_fetch_failed"
	case strings.Contains(msg, "sha256_mismatch"):
		return "sha256_mismatch"
	case strings.Contains(msg, "write_failed"):
		return "write_failed"
	case strings.Contains(msg, "schema_invalid"):
		return "schema_invalid"
	case strings.Contains(msg, "target_path_denied"):
		return "policy_denied"
	default:
		return "execution_failed"
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
