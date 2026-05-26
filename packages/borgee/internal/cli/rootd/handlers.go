//go:build linux || darwin

package rootd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"borgee/internal/cli/installbutler"
)

// DefaultHandlers returns the production whitelist. PR-4 extends the
// PR-1 skeleton (which carried only `ping`) with three privileged
// commands:
//
//   - install_plugin      — invokes installbutler in-process to fetch +
//                           verify + place a signed runtime plugin
//                           binary. Root is needed because the target
//                           directories live under /usr/local/lib
//                           (Linux) or /Library/Application Support
//                           (macOS).
//   - service_lifecycle   — start/stop/restart/reload/enable/disable a
//                           declared systemd unit (Linux) or launchd
//                           service (macOS). Unit names come from the
//                           signed manifest carried in the leased job,
//                           NOT from client-supplied free-form strings.
//   - delegation_revoke   — disable borgee.service so systemd does not
//                           respawn the daemon after revoke, then wipe
//                           the helper credential / enrollment-id /
//                           device-id files. Used by the
//                           delegation.revoke executor on the daemon
//                           side; the daemon process exits AFTER the
//                           WS Result frame is sent.
//
// Every command:
//
//  1. Type-checks its params with a fixed schema (no map[string]any pass-through).
//  2. Rejects unknown / extra fields.
//  3. Is safe to log the cmd name + ok status (no secrets in audit line).
//  4. Documents the threat model addition in the package doc comment.
func DefaultHandlers() map[string]HandlerFunc {
	return DefaultHandlersWithFS(realFS{})
}

// DefaultHandlersWithFS is the test-injectable variant. Production
// callers use DefaultHandlers (which passes realFS so the
// delegation_revoke handler deletes real credential files). Tests inject
// a fakeFS to assert the right files were targeted without touching the
// host's /var/lib/borgee.
func DefaultHandlersWithFS(fs CredentialFS) map[string]HandlerFunc {
	return map[string]HandlerFunc{
		"ping":              pingHandler,
		"install_plugin":    installPluginHandler,
		"service_lifecycle": serviceLifecycleHandler,
		"delegation_revoke": delegationRevokeHandlerWithFS(fs),
	}
}

// pingHandler is the smoke command. Echoes a small pong envelope so the
// main daemon can prove IPC connectivity + audit-trail wiring.
func pingHandler(_ context.Context, _ json.RawMessage) (any, error) {
	return map[string]any{
		"pong": true,
		"time": time.Now().UnixMilli(),
	}, nil
}

// InstallPluginParams — the typed contract for the `install_plugin`
// rootd command. The DAEMON executor (`internal/executors/installplugin`)
// resolves these values from the leased job's payload + manifest binding;
// rootd performs defense-in-depth validation a second time so a buggy or
// malicious daemon cannot smuggle arbitrary paths past rootd.
type InstallPluginParams struct {
	ManifestURL   string `json:"manifest_url"`
	PubKeyBase64  string `json:"pubkey_base64"`
	PluginID      string `json:"plugin_id"`
	TargetPath    string `json:"target_path"`
	HelperUser    string `json:"helper_user,omitempty"`
	HelperGroup   string `json:"helper_group,omitempty"`
	DryRun        bool   `json:"dry_run,omitempty"`
	AllowInsecure bool   `json:"allow_insecure_manifest,omitempty"`
}

// InstallPluginResult — the typed response for install_plugin. The
// `installed` flag distinguishes a real placement from a dry-run plan;
// `target_path` echoes back so callers can audit which path rootd wrote
// to. stderr_summary captures install-butler's reason+detail line on
// either path for operator visibility.
type InstallPluginResult struct {
	Installed     bool   `json:"installed"`
	TargetPath    string `json:"target_path"`
	StdoutSummary string `json:"stdout_summary,omitempty"`
	StderrSummary string `json:"stderr_summary,omitempty"`
}

// installPluginAllowPrefixes — the only target-path prefixes rootd will
// write under. Mirrors the production install layout (Linux:
// /usr/local/lib/borgee/; macOS: /Library/Application Support/Borgee/
// and /usr/local/libexec/borgee/). Any target_path outside these is
// rejected with target_path_denied before install-butler is invoked.
var installPluginAllowPrefixes = []string{
	"/usr/local/lib/borgee/",
	"/usr/local/libexec/borgee/",
	"/Library/Application Support/Borgee/",
}

var pluginIDRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]+$`)

func installPluginHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	params, err := parseInstallPluginParams(raw)
	if err != nil {
		return nil, err
	}
	args := []string{
		"--manifest-url=" + params.ManifestURL,
		"--pubkey-base64=" + params.PubKeyBase64,
		"--plugin-id=" + params.PluginID,
		"--target=" + params.TargetPath,
	}
	if params.HelperUser != "" {
		args = append(args, "--helper-user="+params.HelperUser)
	}
	if params.HelperGroup != "" {
		args = append(args, "--helper-group="+params.HelperGroup)
	}
	if params.DryRun {
		args = append(args, "--dry-run")
	}
	if params.AllowInsecure {
		args = append(args, "--allow-insecure-manifest-url")
		// #1050 blocker #3: dev-stack pairs an http:// manifest URL
		// with an http:// binary URL (both served by the dev server
		// container). install-butler validates them independently —
		// allow both when the dev opt-in is set. Production rootd
		// rejects both at parseInstallPluginParams before this line.
		args = append(args, "--allow-insecure-binary-url")
	}
	// Respect a ctx deadline by tightening installbutler's HTTP timeout.
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			args = append(args, "--http-timeout="+remaining.String())
		}
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	runErr := installbutler.Run(args, &stdoutBuf, &stderrBuf)
	result := InstallPluginResult{
		Installed:     runErr == nil && !params.DryRun,
		TargetPath:    params.TargetPath,
		StdoutSummary: strings.TrimSpace(stdoutBuf.String()),
		StderrSummary: strings.TrimSpace(stderrBuf.String()),
	}
	if runErr != nil {
		// Surface install-butler's reason:detail string as the error
		// payload so the daemon executor can map the exact failure
		// (manifest_fetch_failed / signature_invalid / sha256_mismatch /
		// write_failed / ...) onto a TerminalStatus failure_code.
		msg := strings.TrimSpace(stderrBuf.String())
		if msg == "" {
			msg = runErr.Error()
		}
		return result, errors.New(msg)
	}
	return result, nil
}

func parseInstallPluginParams(raw json.RawMessage) (InstallPluginParams, error) {
	var p InstallPluginParams
	if err := strictDecode(raw, &p); err != nil {
		return InstallPluginParams{}, fmt.Errorf("schema_invalid: %w", err)
	}
	p.ManifestURL = strings.TrimSpace(p.ManifestURL)
	p.PubKeyBase64 = strings.TrimSpace(p.PubKeyBase64)
	p.PluginID = strings.TrimSpace(p.PluginID)
	p.TargetPath = strings.TrimSpace(p.TargetPath)
	if p.ManifestURL == "" || p.PubKeyBase64 == "" || p.PluginID == "" || p.TargetPath == "" {
		return InstallPluginParams{}, errors.New("schema_invalid: required field missing")
	}
	lowerURL := strings.ToLower(p.ManifestURL)
	if !strings.HasPrefix(lowerURL, "https://") {
		if !p.AllowInsecure || !strings.HasPrefix(lowerURL, "http://") {
			return InstallPluginParams{}, errors.New("manifest_url_insecure: must be https://")
		}
	}
	if !pluginIDRegexp.MatchString(p.PluginID) {
		return InstallPluginParams{}, errors.New("plugin_id_invalid: must match ^[a-z0-9][a-z0-9._-]+$")
	}
	if !filepath.IsAbs(p.TargetPath) || strings.Contains(p.TargetPath, "..") {
		return InstallPluginParams{}, errors.New("target_path_denied: must be absolute clean path")
	}
	if !hasAllowedPrefix(p.TargetPath, installPluginAllowPrefixes) {
		return InstallPluginParams{}, fmt.Errorf("target_path_denied: %q not under an allowed prefix", p.TargetPath)
	}
	return p, nil
}

func hasAllowedPrefix(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// ServiceLifecycleParams — the typed contract for the
// `service_lifecycle` rootd command. manager + unit are resolved by the
// daemon executor from the leased job's manifest ServiceDeclaration
// (NOT from client free-form strings), then re-validated by rootd as
// defense-in-depth.
type ServiceLifecycleParams struct {
	Manager   string `json:"manager"`
	Unit      string `json:"unit"`
	Operation string `json:"operation"`
}

// ServiceLifecycleResult — the typed response. ExitCode==0 + Stdout/Stderr
// captured so the daemon executor can record the systemctl/launchctl
// output in the terminal result summary's log_refs.
type ServiceLifecycleResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

var serviceUnitRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]+$`)

func serviceLifecycleHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	params, err := parseServiceLifecycleParams(raw)
	if err != nil {
		return nil, err
	}
	name, args, err := serviceLifecycleArgv(params)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	result := ServiceLifecycleResult{
		ExitCode: exitCode,
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
	}
	if runErr != nil && exitCode == 0 {
		// Command never executed (binary missing, ctx error, etc).
		// Surface as a non-nil error so rootd's response carries
		// ok:false. Operators reading the audit log can distinguish
		// this from a non-zero exit by the empty exit_code+stderr.
		return result, fmt.Errorf("exec_failed: %v", runErr)
	}
	if exitCode != 0 {
		return result, fmt.Errorf("service_op_failed: exit_code=%d stderr=%s", exitCode, truncate(result.Stderr, 200))
	}
	return result, nil
}

// serviceLifecycleArgv maps the typed (manager, operation, unit) tuple
// to the argv we actually exec. Extracted from serviceLifecycleHandler
// so unit tests can assert the launchctl V2 invocations without a real
// macOS host (the parse-time `runtime.GOOS` check enforces real platform
// at the boundary; argv selection is pure).
//
// Linux (systemd): `systemctl <op> <unit>` — every op in the whitelist
// is a native systemctl subcommand, so the mapping is a direct passthrough.
//
// Darwin (launchd / launchctl V2): operations DO NOT map 1:1.
// `launchctl restart` / `launchctl reload` are NOT subcommands in V2.
// The mapping used here picks the closest semantic equivalent to the
// systemd op so the helper-job contract behaves identically on both
// platforms:
//
//   - `start`   → `launchctl kickstart system/<label>`
//                 Spawns the service. (`launchctl start <label>` is the
//                 legacy V1 form; kickstart is the V2 equivalent.)
//   - `stop`    → `launchctl kill TERM system/<label>`
//                 Sends SIGTERM (graceful stop), matching systemd's
//                 `stop` semantics. If the plist has KeepAlive=true the
//                 launchd supervisor will respawn — that mirrors
//                 systemd's `Restart=on-failure` behavior. Operators
//                 wanting a full unload should issue `delegation.revoke`
//                 (which bootouts via `launchctl disable`) instead.
//   - `restart` → `launchctl kickstart -k system/<label>`
//                 -k means "kill then start", the V2 restart idiom.
//   - `reload`  → `launchctl kill HUP system/<label>`
//                 SIGHUP, the convention macOS daemons use for
//                 config-reload-without-restart (mirrors systemd
//                 `reload`'s ExecReload= contract).
//   - `enable`  → `launchctl enable system/<label>`
//   - `disable` → `launchctl disable system/<label>`
func serviceLifecycleArgv(params ServiceLifecycleParams) (string, []string, error) {
	switch params.Manager {
	case "systemd":
		// systemctl subcommands match the whitelist 1:1.
		return "systemctl", []string{params.Operation, params.Unit}, nil
	case "launchd":
		label := "system/" + params.Unit
		switch params.Operation {
		case "start":
			return "launchctl", []string{"kickstart", label}, nil
		case "stop":
			return "launchctl", []string{"kill", "TERM", label}, nil
		case "restart":
			return "launchctl", []string{"kickstart", "-k", label}, nil
		case "reload":
			return "launchctl", []string{"kill", "HUP", label}, nil
		case "enable":
			return "launchctl", []string{"enable", label}, nil
		case "disable":
			return "launchctl", []string{"disable", label}, nil
		default:
			return "", nil, fmt.Errorf("operation_invalid: %q (launchd)", params.Operation)
		}
	default:
		// Already filtered by parseServiceLifecycleParams; defensive.
		return "", nil, fmt.Errorf("manager_invalid: %q", params.Manager)
	}
}

func parseServiceLifecycleParams(raw json.RawMessage) (ServiceLifecycleParams, error) {
	var p ServiceLifecycleParams
	if err := strictDecode(raw, &p); err != nil {
		return ServiceLifecycleParams{}, fmt.Errorf("schema_invalid: %w", err)
	}
	p.Manager = strings.TrimSpace(p.Manager)
	p.Unit = strings.TrimSpace(p.Unit)
	p.Operation = strings.TrimSpace(p.Operation)
	switch p.Manager {
	case "systemd", "launchd":
	default:
		return ServiceLifecycleParams{}, fmt.Errorf("manager_invalid: %q (allowed: systemd, launchd)", p.Manager)
	}
	if !serviceUnitRegexp.MatchString(p.Unit) {
		return ServiceLifecycleParams{}, fmt.Errorf("unit_invalid: %q (must match %s)", p.Unit, serviceUnitRegexp.String())
	}
	switch p.Operation {
	case "start", "stop", "restart", "reload", "enable", "disable":
	default:
		return ServiceLifecycleParams{}, fmt.Errorf("operation_invalid: %q (allowed: start, stop, restart, reload, enable, disable)", p.Operation)
	}
	// Manager + platform sanity: a hosts running launchctl on Linux or
	// systemctl on darwin would be misconfigured. Reject at rootd so we
	// fail before invoking the missing binary.
	switch runtime.GOOS {
	case "linux":
		if p.Manager != "systemd" {
			return ServiceLifecycleParams{}, fmt.Errorf("manager_mismatch: linux host requires systemd, got %q", p.Manager)
		}
	case "darwin":
		if p.Manager != "launchd" {
			return ServiceLifecycleParams{}, fmt.Errorf("manager_mismatch: darwin host requires launchd, got %q", p.Manager)
		}
	}
	return p, nil
}

// DelegationRevokeParams — typed contract for `delegation_revoke`. The
// rootd handler disables borgee.service so systemd does NOT respawn the
// daemon after the daemon process exits, then wipes the credential /
// enrollment-id / device-id files at well-known daemon paths. The
// `enrollment_id` field is recorded for the audit log so the operator can
// correlate which enrollment was revoked.
type DelegationRevokeParams struct {
	EnrollmentID         string `json:"enrollment_id"`
	DrainTimeoutSeconds  int    `json:"drain_timeout_seconds,omitempty"`
	ServiceName          string `json:"service_name,omitempty"`
	ServiceManager       string `json:"service_manager,omitempty"`
	CredentialPaths      []string `json:"credential_paths,omitempty"`
}

// DelegationRevokeResult — typed response. Disabled+CredentialWiped are
// the two bookkeeping signals the daemon executor maps onto the terminal
// result summary's audit_refs.
type DelegationRevokeResult struct {
	Disabled        bool     `json:"disabled"`
	CredentialWiped bool     `json:"credential_wiped"`
	WipedPaths      []string `json:"wiped_paths,omitempty"`
}

// CredentialFS is the seam tests use to verify delegation_revoke
// targets the right paths without actually touching /var/lib/borgee.
type CredentialFS interface {
	Remove(path string) error
}

// realFS is the production implementation (uses os.Remove). Treat
// missing files as success so a retried revoke does not red-flag the
// audit log.
type realFS struct{}

func (realFS) Remove(path string) error {
	if err := osRemove(path); err != nil && !isNotExist(err) {
		return err
	}
	return nil
}

// delegationRevokeHandlerWithFS returns a handler closure bound to the
// injected filesystem. Production callers go through DefaultHandlers
// which uses realFS; tests inject a fakeFS.
func delegationRevokeHandlerWithFS(fs CredentialFS) HandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		params, err := parseDelegationRevokeParams(raw)
		if err != nil {
			return nil, err
		}
		// 1. Disable the service so systemd does not respawn us after
		//    exit. Failure here is best-effort: rootd may not have been
		//    started by systemd on this host (e.g. dev), so a disable
		//    failure does NOT abort the credential wipe.
		var sysErr error
		switch params.ServiceManager {
		case "systemd":
			cmd := exec.CommandContext(ctx, "systemctl", "disable", params.ServiceName)
			sysErr = cmd.Run()
		case "launchd":
			cmd := exec.CommandContext(ctx, "launchctl", "disable", "system/"+params.ServiceName)
			sysErr = cmd.Run()
		}
		_ = sysErr

		// 2. Wipe credential files. Missing files are treated as
		//    already-wiped (revoke is idempotent).
		wiped := make([]string, 0, len(params.CredentialPaths))
		for _, p := range params.CredentialPaths {
			if err := fs.Remove(p); err != nil {
				return DelegationRevokeResult{
					Disabled:        sysErr == nil,
					CredentialWiped: false,
					WipedPaths:      wiped,
				}, fmt.Errorf("credential_wipe_failed: %s: %v", p, err)
			}
			wiped = append(wiped, p)
		}
		return DelegationRevokeResult{
			Disabled:        sysErr == nil,
			CredentialWiped: true,
			WipedPaths:      wiped,
		}, nil
	}
}

func parseDelegationRevokeParams(raw json.RawMessage) (DelegationRevokeParams, error) {
	var p DelegationRevokeParams
	if err := strictDecode(raw, &p); err != nil {
		return DelegationRevokeParams{}, fmt.Errorf("schema_invalid: %w", err)
	}
	p.EnrollmentID = strings.TrimSpace(p.EnrollmentID)
	p.ServiceName = strings.TrimSpace(p.ServiceName)
	p.ServiceManager = strings.TrimSpace(p.ServiceManager)
	if p.EnrollmentID == "" {
		return DelegationRevokeParams{}, errors.New("schema_invalid: enrollment_id is required")
	}
	if p.ServiceManager != "" && p.ServiceManager != "systemd" && p.ServiceManager != "launchd" {
		return DelegationRevokeParams{}, fmt.Errorf("manager_invalid: %q", p.ServiceManager)
	}
	if p.ServiceName != "" && !serviceUnitRegexp.MatchString(p.ServiceName) {
		return DelegationRevokeParams{}, fmt.Errorf("service_invalid: %q", p.ServiceName)
	}
	for _, path := range p.CredentialPaths {
		if !filepath.IsAbs(path) || strings.Contains(path, "..") {
			return DelegationRevokeParams{}, fmt.Errorf("credential_path_denied: %q", path)
		}
	}
	if p.DrainTimeoutSeconds < 0 || p.DrainTimeoutSeconds > 300 {
		return DelegationRevokeParams{}, fmt.Errorf("drain_timeout_invalid: %d (must be 0..300)", p.DrainTimeoutSeconds)
	}
	return p, nil
}

// strictDecode is a tiny helper that DisallowUnknownFields + decodes
// into the destination struct. Each handler uses it on its typed params
// so any extra/typo field rejects at the wire boundary.
func strictDecode(raw json.RawMessage, dst any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("empty params")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
