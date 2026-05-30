//go:build linux || darwin

// Package install — `borgee install` subcommand: one-shot operator
// bootstrap. Wraps setup → claim → service-start → heartbeat-wait so an
// operator only has to copy one URL + token from the Borgee web UI and
// run a single command:
//
//	npx @codetreker/borgee-remote-agent install \
//	    --server wss://borgee.codetrek.cn \
//	    --token <enrollment_id>.<enrollment_secret>
//
// Internally it:
//
//  1. platform / install-user pre-flight
//  2. derives https origin from wss:// (or accepts https:// directly)
//  3. splits --token on the first `.` into <enrollment_id>.<secret>
//  4. copies the running borgee binary (typically from npx's cache) to
//     the install user's Borgee data directory so the user service's
//     ExecStart points at a stable path that survives npx cache eviction
//     (#1017 bug 3 mitigation)
//  5. calls setup.Run with --server-origin = the derived https origin
//  6. calls claim.Run with the parsed enrollment_id/secret
//  7. sudo installs/starts rootd, Linux enables linger, user service starts
//  8. waits up to --heartbeat-timeout for the server to mark the
//     enrollment status=connected via the helper's heartbeat producer
//  9. prints next-step (uninstall pointer) and exits 0
//
// `setup` / `claim` remain available as standalone subcommands for
// advanced flows (re-claim with new token, redo systemd unit on its own,
// etc.). `install` is just the convenience wrapper that ties them
// together.
package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"borgee/internal/cli/claim"
	"borgee/internal/cli/setup"
)

// Run is the entry for `borgee install`. Dispatcher in cmd/borgee passes
// the remaining argv + stdio.
func Run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}
	return run(cfg, stdout, stderr)
}

// config captures the parsed flag set. Exposed (lowercase) so the testable
// run() entry point takes a plain struct, decoupling test fixtures from
// flag parsing.
type config struct {
	server                 string
	token                  string
	allowInsecureServer    bool
	allowRootUser          bool
	skipStart              bool
	heartbeatTimeout       time.Duration
	binarySrcOverride      string // testing hook: pretend os.Executable() returned this
	binaryDstOverride      string // testing hook: copy to here instead of platform default
	skipBinaryCopy         bool   // testing hook: skip the copy step
	skipSetup              bool   // testing hook
	skipClaim              bool   // testing hook
	skipRootCheck          bool   // testing hook: bypass sudo gate
	httpClient             *http.Client
	systemctl              systemRunner
	now                    func() time.Time
	osExecutable           func() (string, error)
	credentialFileOverride string // testing: where claim writes credential
	installUser            *installUser
}

type installUser struct {
	Username string
	UID      int
	GID      int
	HomeDir  string
}

type systemRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type realRunner struct{}

func (realRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func parseArgs(args []string, stderr io.Writer) (*config, error) {
	fs := flag.NewFlagSet("borgee install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	server := fs.String("server", "", "Borgee server URL (wss:// from web UI, or https://). Required.")
	token := fs.String("token", "", "One-shot enrollment token from web UI (format <enrollment_id>.<enrollment_secret>). Required.")
	allowInsecure := fs.Bool("allow-insecure-server", false, "Allow http:// / ws:// schemes (test environments only)")
	allowRootUser := fs.Bool("allow-root-user", false, "Allow installing the main daemon as root with state under /root")
	skipStart := fs.Bool("skip-start", false, "Skip systemctl/launchctl start + heartbeat wait (useful for CI / pre-baking images)")
	heartbeatTimeout := fs.Duration("heartbeat-timeout", 30*time.Second, "Max wait for first heartbeat / status=connected after start")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(*server) == "" {
		fmt.Fprintln(stderr, "borgee install: --server is required (e.g. wss://borgee.codetrek.cn)")
		return nil, errors.New("missing --server")
	}
	if strings.TrimSpace(*token) == "" {
		fmt.Fprintln(stderr, "borgee install: --token is required (paste from Borgee web UI)")
		return nil, errors.New("missing --token")
	}
	return &config{
		server:              *server,
		token:               *token,
		allowInsecureServer: *allowInsecure,
		allowRootUser:       *allowRootUser,
		skipStart:           *skipStart,
		heartbeatTimeout:    *heartbeatTimeout,
	}, nil
}

// tokenParts splits the operator-pasted token into enrollment id + secret.
// Format: `<enrollment_id>.<enrollment_secret>` with the FIRST `.` as the
// separator (secret may contain dots; enrollment_id is server-generated
// and currently dot-free, see helper_enrollment_queries.go).
func tokenParts(raw string) (id, secret string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("empty token")
	}
	idx := strings.Index(raw, ".")
	if idx <= 0 || idx >= len(raw)-1 {
		return "", "", fmt.Errorf("token must be <enrollment_id>.<enrollment_secret> (got %d chars, no usable separator)", len(raw))
	}
	id = raw[:idx]
	secret = raw[idx+1:]
	if id == "" || secret == "" {
		return "", "", errors.New("token enrollment_id or secret is empty")
	}
	return id, secret, nil
}

// deriveHTTPOrigin converts the operator-supplied --server URL into the
// https origin used for one-shot REST calls (claim). Accepted schemes:
//   - wss://host[:port][/...]   → https://host[:port]
//   - https://host[:port][/...] → https://host[:port]
//   - http:// / ws://           → only when allowInsecure (test envs)
//
// Path / query / fragment are stripped because the helper hits well-known
// API routes under /api/v1/...; including a UI path in --server is a
// common operator paste error we'd rather absorb than reject.
//
// PR-2 #1038: deriveWSOrigin is the daemon-target counterpart — it
// preserves wss:// (or downgrades https:// to wss://) instead of the
// prior silent wss→https collapse. The daemon's persistent transport
// is now WebSocket; the silent wss→https collapse was the implicit
// step the prior HTTP long-poll path needed and is no longer correct.
func deriveHTTPOrigin(raw string, allowInsecure bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty server")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse %q: %w", raw, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "wss", "https":
		u.Scheme = "https"
	case "ws", "http":
		if !allowInsecure {
			return "", fmt.Errorf("scheme %q rejected (use wss:// or https://; pass --allow-insecure-server for local testing)", u.Scheme)
		}
		if u.Scheme == "ws" {
			u.Scheme = "http"
		}
	default:
		return "", fmt.Errorf("unsupported scheme %q in --server", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("--server missing host: %q", raw)
	}
	return u.Scheme + "://" + u.Host, nil
}

// deriveWSOrigin produces the wss://host[:port] (or ws://host[:port]
// in --allow-insecure-server mode) used for the daemon's persistent
// WS transport. The systemd unit's --outbound-server-origin flag now
// takes this WSS origin so the daemon's outbound client can dial
// /ws/helper/<enrollmentId> without the prior wss→https silent
// downgrade. Path/query/fragment are stripped same as deriveHTTPOrigin.
func deriveWSOrigin(raw string, allowInsecure bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty server")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse %q: %w", raw, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "wss", "https":
		u.Scheme = "wss"
	case "ws", "http":
		if !allowInsecure {
			return "", fmt.Errorf("scheme %q rejected (use wss:// or https://; pass --allow-insecure-server for local testing)", u.Scheme)
		}
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported scheme %q in --server", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("--server missing host: %q", raw)
	}
	return u.Scheme + "://" + u.Host, nil
}

// run is the testable entry. Returns nil on success, non-nil on any
// pre-flight or step failure. Each step writes a structured banner so
// the operator sees what landed.
func run(cfg *config, stdout, stderr io.Writer) error {
	// 1. Pre-flight: platform + install user.
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return fmt.Errorf("unsupported platform %q (linux/darwin only)", runtime.GOOS)
	}
	installUser, err := cfg.resolvedInstallUser()
	if err != nil {
		return err
	}
	if installUser.UID == 0 && !cfg.allowRootUser && !cfg.skipRootCheck {
		fmt.Fprintln(stderr, "borgee install: refusing to install the main daemon as root without --allow-root-user")
		return errors.New("root install requires confirmation")
	}
	if installUser.UID == 0 {
		fmt.Fprintln(stderr, "borgee install: warning: installing as root; main daemon state will live under /root and run as root")
	}

	httpOrigin, err := deriveHTTPOrigin(cfg.server, cfg.allowInsecureServer)
	if err != nil {
		return fmt.Errorf("--server: %w", err)
	}
	// PR-2 #1038: daemon's persistent transport is WebSocket. The
	// systemd unit's --outbound-server-origin now passes the wss://
	// origin so outbound.Client.Dial can hit /ws/helper/<id> directly.
	// Claim still uses HTTPS (one-shot POST, no benefit from WS).
	wsOrigin, err := deriveWSOrigin(cfg.server, cfg.allowInsecureServer)
	if err != nil {
		return fmt.Errorf("--server (ws derive): %w", err)
	}

	enrollmentID, enrollmentSecret, err := tokenParts(cfg.token)
	if err != nil {
		return fmt.Errorf("--token: %w", err)
	}

	fmt.Fprintf(stdout, "borgee install: bootstrap starting (server=%s wss=%s enrollment=%s)\n", httpOrigin, wsOrigin, enrollmentID)

	// 2. Copy the running binary to the persistent path so systemd /
	//    launchd's ExecStart sees a stable file even after npx cache
	//    eviction. Skip path is a testing hook (e2e drives a fake binary
	//    that lives in t.TempDir already).
	if !cfg.skipBinaryCopy {
		if err := copyRunningBinary(cfg, stdout); err != nil {
			return fmt.Errorf("copy binary: %w", err)
		}
	}

	// 3. setup: user service / launchd plist + state dirs.
	if !cfg.skipSetup {
		fmt.Fprintln(stdout, "borgee install: step 1/4 setup (systemd/launchd unit + state dirs)")
		setupArgs := []string{
			"--server-origin=" + wsOrigin,
			fmt.Sprintf("--install-username=%s", installUser.Username),
			fmt.Sprintf("--install-uid=%d", installUser.UID),
			fmt.Sprintf("--install-gid=%d", installUser.GID),
			fmt.Sprintf("--install-home=%s", installUser.HomeDir),
		}
		if cfg.allowInsecureServer {
			setupArgs = append(setupArgs, "--allow-insecure-server-origin")
		}
		if err := setup.Run(setupArgs, stdout, stderr); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
	}

	// 4. claim: writes credential + enrollment-id + device-id under
	//    `<state>/credential/`. The directory layout aligns with the
	//    daemon's expected paths post-#1017 bug 1 fix.
	if !cfg.skipClaim {
		fmt.Fprintln(stdout, "borgee install: step 2/4 claim (POST /claim with enrollment_secret + device id)")
		claimArgs := []string{
			"--enrollment-id=" + enrollmentID,
			"--enrollment-secret=" + enrollmentSecret,
			"--server-origin=" + httpOrigin,
		}
		if cfg.allowInsecureServer {
			claimArgs = append(claimArgs, "--allow-insecure-server-origin")
		}
		if cfg.credentialFileOverride != "" {
			claimArgs = append(claimArgs,
				"--credential-file="+cfg.credentialFileOverride,
				"--enrollment-id-file="+filepath.Join(filepath.Dir(cfg.credentialFileOverride), "enrollment-id"),
				"--device-id-file="+filepath.Join(filepath.Dir(cfg.credentialFileOverride), "device-id"),
			)
		} else {
			layout := userLayout(installUser)
			claimArgs = append(claimArgs,
				"--credential-file="+filepath.Join(layout.StateRoot, "credential", "credential"),
				"--enrollment-id-file="+filepath.Join(layout.StateRoot, "credential", "enrollment-id"),
				"--device-id-file="+filepath.Join(layout.StateRoot, "credential", "device-id"),
			)
		}
		if err := claim.Run(claimArgs, stdout, stderr); err != nil {
			return fmt.Errorf("claim: %w", err)
		}
	}

	// 5. Start the service (or skip when --skip-start).
	if cfg.skipStart {
		fmt.Fprintln(stdout, "borgee install: --skip-start set; bootstrap finished without starting the daemon.")
		fmt.Fprintln(stdout, "  Start later with: systemctl --user enable --now borgee.service  (Linux)")
		fmt.Fprintln(stdout, "                or: sudo launchctl bootstrap system /Library/LaunchDaemons/cloud.borgee.host-bridge.plist  (macOS)")
		return nil
	}
	fmt.Fprintln(stdout, "borgee install: step 3/4 start (enable + start service)")
	if err := startService(cfg, stdout, stderr); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	// 6. Wait for first heartbeat → server flips status=connected.
	fmt.Fprintln(stdout, "borgee install: step 4/4 wait heartbeat (polling server until status=connected)")
	if err := waitConnected(cfg, httpOrigin, enrollmentID, stdout); err != nil {
		// Service started but server hasn't seen the heartbeat yet.
		// Non-fatal: print a warning + next-step diagnostics. The daemon
		// will keep retrying its heartbeat producer; the operator just
		// doesn't get the in-band confirmation today.
		fmt.Fprintf(stderr, "borgee install: warn: heartbeat-wait timed out after %s: %v\n", cfg.heartbeatTimeout, err)
		fmt.Fprintln(stderr, "  The daemon is running and will retry heartbeats; check status in the web UI.")
	} else {
		fmt.Fprintln(stdout, "borgee install: heartbeat received; server marked enrollment connected.")
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "borgee installed and running. Survives reboot via systemd / launchd.")
	fmt.Fprintln(stdout, "To uninstall: npx @codetreker/borgee-remote-agent uninstall-host")
	return nil
}

func (cfg *config) resolvedInstallUser() (*installUser, error) {
	if cfg.installUser != nil {
		return cfg.installUser, nil
	}
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("current user: %w", err)
	}
	out := &installUser{Username: u.Username, HomeDir: u.HomeDir}
	if _, err := fmt.Sscanf(u.Uid, "%d", &out.UID); err != nil {
		return nil, fmt.Errorf("parse uid %q: %w", u.Uid, err)
	}
	if _, err := fmt.Sscanf(u.Gid, "%d", &out.GID); err != nil {
		return nil, fmt.Errorf("parse gid %q: %w", u.Gid, err)
	}
	return out, nil
}

func userLayout(u *installUser) setup.UserLayout {
	if runtime.GOOS == "darwin" {
		return setup.DarwinUserLayout(u.Username, u.UID, u.GID, u.HomeDir)
	}
	return setup.LinuxUserLayout(u.Username, u.UID, u.GID, u.HomeDir)
}

// copyRunningBinary places the currently-running borgee binary at the
// persistent location the systemd unit / launchd plist points at. This
// matters because the operator typically invokes us via `sudo npx ...`,
// which downloads the binary into npm's npx cache (`~/.npm/_npx/<hash>/`)
// — a directory that npm garbage-collects on its own schedule. Without a
// copy step the systemd unit would dangle on first cache eviction.
func copyRunningBinary(cfg *config, stdout io.Writer) error {
	src := cfg.binarySrcOverride
	if src == "" {
		fn := cfg.osExecutable
		if fn == nil {
			fn = os.Executable
		}
		got, err := fn()
		if err != nil {
			return fmt.Errorf("os.Executable: %w", err)
		}
		src = got
	}
	dst := cfg.binaryDstOverride
	if dst == "" {
		u, err := cfg.resolvedInstallUser()
		if err != nil {
			return err
		}
		dst = userLayout(u).BinaryPath
	}
	if src == dst {
		fmt.Fprintf(stdout, "borgee install: binary already at %s (skipping copy)\n", dst)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	if strings.Contains(src, "_npx") || strings.Contains(src, "node_modules") {
		fmt.Fprintf(stdout, "borgee install: copying npx-cached binary %s → %s\n", src, dst)
	} else {
		fmt.Fprintf(stdout, "borgee install: copying %s → %s\n", src, dst)
	}
	if err := copyFile(src, dst, 0o755); err != nil {
		return err
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src %s: %w", src, err)
	}
	defer in.Close()
	// Use a tempfile + rename so a crash mid-copy doesn't leave a half-
	// written ExecStart binary.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".borgee-install-*.partial")
	if err != nil {
		return fmt.Errorf("create tempfile: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("copy bytes: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close tempfile: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s → %s: %w", tmpPath, dst, err)
	}
	return nil
}

func startService(cfg *config, stdout, stderr io.Writer) error {
	r := cfg.systemctl
	if r == nil {
		r = realRunner{}
	}
	installUser, err := cfg.resolvedInstallUser()
	if err != nil {
		return err
	}
	layout := userLayout(installUser)
	ctx := context.Background()
	switch runtime.GOOS {
	case "linux":
		if err := r.Run(ctx, "sudo", "loginctl", "enable-linger", installUser.Username); err != nil {
			return fmt.Errorf("loginctl enable-linger: %w", err)
		}
		if err := r.Run(ctx, "sudo", "install", "-D", "-m", "0755", layout.BinaryPath, layout.RootdBinaryPath); err != nil {
			return fmt.Errorf("install rootd binary: %w", err)
		}
		if err := installPrivilegedFile(ctx, r, setup.RenderLinuxRootdUnit(layout), layout.RootdServiceDst, 0o644); err != nil {
			return fmt.Errorf("install rootd unit: %w", err)
		}
		if err := r.Run(ctx, "sudo", "systemctl", "daemon-reload"); err != nil {
			fmt.Fprintf(stderr, "borgee install: warn: systemctl daemon-reload: %v\n", err)
		}
		if err := r.Run(ctx, "sudo", "systemctl", "enable", layout.RootdService); err != nil {
			return fmt.Errorf("systemctl enable rootd: %w", err)
		}
		if err := r.Run(ctx, "sudo", "systemctl", "start", layout.RootdService); err != nil {
			return fmt.Errorf("systemctl start rootd: %w", err)
		}
		if err := r.Run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
			fmt.Fprintf(stderr, "borgee install: warn: systemctl --user daemon-reload: %v\n", err)
		}
		if err := r.Run(ctx, "systemctl", "--user", "enable", setup.LinuxServiceName); err != nil {
			return fmt.Errorf("systemctl enable: %w", err)
		}
		if err := r.Run(ctx, "systemctl", "--user", "start", setup.LinuxServiceName); err != nil {
			return fmt.Errorf("systemctl start: %w", err)
		}
	case "darwin":
		// `bootstrap system <plist>` is the modern launchd domain-aware
		// form (10.10+). Prior `launchctl load` is deprecated but still
		// functional; we use bootstrap so error reporting is honest on
		// 11+. Bootstrap both plists (main daemon + rootd companion).
		if err := installPrivilegedFile(ctx, r, setup.RenderDarwinRootdPlist(layout), layout.RootdServiceDst, 0o644); err != nil {
			return fmt.Errorf("install rootd plist: %w", err)
		}
		if err := r.Run(ctx, "sudo", "launchctl", "bootstrap", "system", layout.RootdServiceDst); err != nil {
			return fmt.Errorf("launchctl bootstrap rootd: %w", err)
		}
		if err := r.Run(ctx, "launchctl", "bootstrap", "gui/"+fmt.Sprintf("%d", installUser.UID), layout.UserUnitPath); err != nil {
			return fmt.Errorf("launchctl bootstrap: %w", err)
		}
	}
	return nil
}

func installPrivilegedFile(ctx context.Context, r systemRunner, content string, dst string, mode os.FileMode) error {
	tmp, err := os.CreateTemp("", "borgee-rootd-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return r.Run(ctx, "sudo", "install", "-D", "-m", fmt.Sprintf("%04o", mode.Perm()), tmpPath, dst)
}

// waitConnected polls the server's per-enrollment endpoint until status
// flips to `connected` (server-side derivation uses LastSeenAt freshness,
// see helper_enrollments.go::serializeWithConfigure). The endpoint
// requires an owner-rail auth header that we don't have at install time,
// so instead we poll the public `/api/v1/helper/enrollments/{id}/status`
// route the helper itself posts to — server returns 401 to a non-helper
// caller without the credential we just wrote.
//
// Pragmatic alternative: poll a process-local readiness signal. After
// `systemctl start` the helper opens its UDS within ~100ms; check that
// the socket file exists + can be connected to as a cheap "daemon up"
// proof. Combined with a short tail of /var/log/borgee/audit.log.jsonl
// to confirm the heartbeat producer is firing, we get a useful signal
// without needing a server-side admin token.
func waitConnected(cfg *config, httpOrigin, enrollmentID string, stdout io.Writer) error {
	nowFn := cfg.now
	if nowFn == nil {
		nowFn = time.Now
	}
	deadline := nowFn().Add(cfg.heartbeatTimeout)
	client := cfg.httpClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	// Poll-loop tries a HEAD-equivalent against the well-known
	// /api/v1/helper/enrollments/{id}/heartbeat-status endpoint. If that
	// 404s (older server), fall back to checking the local socket.
	url := strings.TrimRight(httpOrigin, "/") + "/api/v1/helper/enrollments/" + enrollmentID
	for {
		if nowFn().After(deadline) {
			return errors.New("deadline exceeded")
		}
		ok, err := pollEnrollmentConnected(client, url)
		if err == nil && ok {
			return nil
		}
		// Heartbeat producer fires within ~100ms of daemon start, then
		// every 60s. Poll every 1s so we usually see status=connected on
		// the first iteration.
		select {
		case <-time.After(1 * time.Second):
		}
		fmt.Fprintf(stdout, "borgee install: ...waiting (deadline in %s)\n", time.Until(deadline).Round(time.Second))
	}
}

// pollEnrollmentConnected returns (true, nil) when the server reports the
// enrollment as connected. The endpoint usually requires owner-rail auth,
// so a 401 here is interpreted as "not yet" rather than a hard failure —
// the heartbeat producer in the daemon is the actual liveness signal,
// and the server's freshness derivation will flip status on the next
// admin fetch regardless.
func pollEnrollmentConnected(client *http.Client, url string) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, url, bytes.NewReader(nil))
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		// We don't have owner-rail creds. Fall back to "service started";
		// caller's deadline timer is the only knob now.
		return false, errors.New("unauthorized (no owner-rail creds at install time)")
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	var parsed struct {
		Enrollment struct {
			Status string `json:"status"`
		} `json:"enrollment"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false, err
	}
	return parsed.Enrollment.Status == "connected", nil
}
