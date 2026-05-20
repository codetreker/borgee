//go:build linux || darwin

// Package setup — `borgee setup` subcommand.
//
// Replaces the .deb / .pkg postinstall scripts that the prior nfpm-based
// distribution used. After `npm i -g @codetreker/borgee-remote-agent` placed
// the `borgee` binary on PATH, the operator runs `sudo borgee setup` once to:
//
//   - Linux:
//       1. useradd --system --no-create-home --shell /usr/sbin/nologin borgee (if missing)
//       2. mkdir -p /var/lib/borgee/{queue,status,audit-handoff,credential}
//          /var/log/borgee /run/borgee, chown borgee:borgee, perm 0750
//       3. Write /etc/systemd/system/borgee.service from the embedded template
//       4. systemctl daemon-reload
//       5. Print next-step (claim → start) — do NOT auto-start
//   - macOS:
//       1. dscl create _borgee group + user if missing
//       2. mkdir -p the equivalent /Library/Application Support/Borgee/Helper subdirs
//       3. Write /Library/LaunchDaemons/cloud.borgee.host-bridge.plist + sandbox profile
//       4. Print next-step (claim → launchctl load) — do NOT auto-load
//
// Intentionally NOT idempotent in the "wipe and reinstall" sense: each step
// checks for the prior state and skips if already correct, so re-running
// `borgee setup` on a previously claimed host preserves the credential.
package setup

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// linuxBinaryPath — the persistent on-disk path the systemd unit's
	// ExecStart refers to. `borgee install` copies the running borgee
	// binary (typically from npx's cache) to this location so the daemon
	// survives npx cache eviction. `/usr/local/bin/borgee` (if present
	// from `npm i -g`) is an npm-owned symlink that the daemon does NOT
	// depend on — keeping the persistent binary under `/usr/local/lib/`
	// also sidesteps the #1017 bug 3 symlink-vs-real-binary confusion.
	linuxBinaryPath = "/usr/local/lib/borgee/bin/borgee"
	linuxRuntimeDir = "/usr/local/lib/borgee"
	linuxStateRoot  = "/var/lib/borgee"
	linuxLogDir     = "/var/log/borgee"
	linuxRunDir     = "/run/borgee"
	linuxUser       = "borgee"
	linuxGroup      = "borgee"
	linuxServiceDst = "/etc/systemd/system/borgee.service"
	linuxServerDSN  = "file:/var/lib/borgee/server.db?mode=ro&_busy_timeout=5000"

	// rootd companion daemon (User=root). Same binary, different
	// subcommand; the unit file is separate so each service can be
	// enabled/disabled/restarted independently and so the systemd-level
	// hardening differs (rootd locks down AF_UNIX-only, etc.).
	linuxRootdServiceDst = "/etc/systemd/system/borgee-rootd.service"
	linuxRootdSocket     = "/run/borgee/borgee-rootd.sock"

	darwinUser            = "_borgee"
	darwinStateRoot       = "/Library/Application Support/Borgee/Helper"
	darwinAppSupport      = "/Library/Application Support/Borgee"
	darwinLogDir          = "/Library/Logs/Borgee"
	darwinPlistDst        = "/Library/LaunchDaemons/cloud.borgee.host-bridge.plist"
	darwinSandboxDst      = "/Library/Application Support/Borgee/borgee-helper.sb"
	// macOS persistent binary path. Mirror of linuxBinaryPath. Apple
	// convention uses `libexec` for per-product helper binaries.
	darwinBinaryPath      = "/usr/local/libexec/borgee/borgee"
	darwinRuntimeDir      = "/usr/local/libexec/borgee"
	darwinPlistLabel      = "cloud.borgee.host-bridge"
	darwinUDS             = "/Users/Shared/Borgee/borgee.sock"
	darwinAuditLog        = "/Library/Logs/Borgee/audit.log.jsonl"
	darwinServerDSN       = "file:/Library/Application Support/Borgee/server.db?mode=ro&_busy_timeout=5000"
	darwinQueueStateDir   = "/Library/Application Support/Borgee/Helper/QueueState"
	darwinStatusStateDir  = "/Library/Application Support/Borgee/Helper/StatusState"
	darwinAuditHandoffDir = "/Library/Application Support/Borgee/Helper/AuditHandoff"

	// rootd companion launchd plist + UDS on macOS.
	darwinRootdPlistDst   = "/Library/LaunchDaemons/cloud.borgee.host-bridge.rootd.plist"
	darwinRootdPlistLabel = "cloud.borgee.host-bridge.rootd"
	darwinRootdSocket     = "/Users/Shared/Borgee/borgee-rootd.sock"
)

// LinuxBinaryPath / DarwinBinaryPath / LinuxRuntimeDir / DarwinRuntimeDir
// are exported so the `install` and `uninstall-host` subcommands can
// reference the same persistent paths without duplicating the constants.
const (
	LinuxBinaryPath  = linuxBinaryPath
	DarwinBinaryPath = darwinBinaryPath
	LinuxRuntimeDir  = linuxRuntimeDir
	DarwinRuntimeDir = darwinRuntimeDir
	LinuxServiceDst  = linuxServiceDst
	DarwinPlistDst   = darwinPlistDst
	LinuxServiceName = "borgee.service"
	DarwinPlistLabel = darwinPlistLabel
	LinuxUser        = linuxUser
	DarwinUser       = darwinUser

	// rootd companion daemon (PR-1 skeleton). Exported so install /
	// uninstall flows can manage the second unit alongside borgee.service.
	LinuxRootdServiceDst   = linuxRootdServiceDst
	LinuxRootdServiceName  = "borgee-rootd.service"
	LinuxRootdSocket       = linuxRootdSocket
	DarwinRootdPlistDst    = darwinRootdPlistDst
	DarwinRootdPlistLabel  = darwinRootdPlistLabel
	DarwinRootdSocket      = darwinRootdSocket
)

// Run is the entry for `borgee setup`. Dispatcher in cmd/borgee passes the
// remaining argv + stdio.
func Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("borgee setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "Print what would be done without touching the system")
	serverOrigin := fs.String("server-origin", "wss://app.borgee.io", "Borgee server WS origin to bake into the systemd/launchd unit (wss:// for the daemon's persistent transport, PR-2 #1038)")
	allowInsecureOrigin := fs.Bool("allow-insecure-server-origin", false, "Allow http:// / ws:// server-origin (test environments only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	originLower := strings.ToLower(*serverOrigin)
	// PR-2 #1038: the daemon's persistent transport is WebSocket. Accept
	// wss:// (production) + https:// (backward compat with deployments
	// still on the prior HTTP long-poll path; outbound.Client.Dial
	// transparently rewrites https:// → wss:// for the actual WS dial).
	if !*allowInsecureOrigin && !(strings.HasPrefix(originLower, "wss://") || strings.HasPrefix(originLower, "https://")) {
		fmt.Fprintln(stderr, "borgee setup: --server-origin must be wss:// or https:// (use --allow-insecure-server-origin only for local testing)")
		return errors.New("insecure server origin")
	}

	if os.Geteuid() != 0 && !*dryRun {
		fmt.Fprintln(stderr, "borgee setup: must be run as root (use sudo); pass --dry-run to preview without writing")
		return errors.New("not root")
	}

	switch runtime.GOOS {
	case "linux":
		return runLinux(stdout, stderr, *serverOrigin, *dryRun)
	case "darwin":
		return runDarwin(stdout, stderr, *serverOrigin, *dryRun)
	default:
		return fmt.Errorf("borgee setup: unsupported platform %q", runtime.GOOS)
	}
}

func runLinux(stdout, stderr io.Writer, serverOrigin string, dryRun bool) error {
	logStep := func(label string) {
		if dryRun {
			fmt.Fprintln(stdout, "[dry-run] "+label)
		} else {
			fmt.Fprintln(stdout, label)
		}
	}

	// 1. Ensure system user exists.
	if _, err := user.Lookup(linuxUser); err != nil {
		logStep("create system user " + linuxUser)
		if !dryRun {
			if err := runCmd("useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", linuxUser); err != nil {
				return fmt.Errorf("useradd %s: %w", linuxUser, err)
			}
		}
	} else {
		logStep("system user " + linuxUser + " already exists; skip")
	}

	// 2. Create state dirs.
	stateSubdirs := []string{"queue", "status", "audit-handoff", "credential"}
	for _, sub := range stateSubdirs {
		p := filepath.Join(linuxStateRoot, sub)
		logStep("mkdir " + p)
		if !dryRun {
			if err := os.MkdirAll(p, 0o750); err != nil {
				return fmt.Errorf("mkdir %s: %w", p, err)
			}
			if err := chown(p, linuxUser, linuxGroup); err != nil {
				return fmt.Errorf("chown %s: %w", p, err)
			}
		}
	}
	for _, p := range []string{linuxLogDir, linuxRunDir} {
		logStep("mkdir " + p)
		if !dryRun {
			if err := os.MkdirAll(p, 0o750); err != nil {
				return fmt.Errorf("mkdir %s: %w", p, err)
			}
			if err := chown(p, linuxUser, linuxGroup); err != nil {
				return fmt.Errorf("chown %s: %w", p, err)
			}
		}
	}

	// Runtime dir: holds the persistent borgee binary copy that the
	// `install` subcommand drops (`/usr/local/lib/borgee/bin/borgee`).
	// Owned by root with mode 0755 so the helper user can exec it but
	// only root can replace it.
	runtimeBinDir := filepath.Join(linuxRuntimeDir, "bin")
	logStep("mkdir " + runtimeBinDir)
	if !dryRun {
		if err := os.MkdirAll(runtimeBinDir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", runtimeBinDir, err)
		}
	}

	// 3. Write systemd unit.
	unit := renderLinuxUnit(serverOrigin)
	logStep("write " + linuxServiceDst)
	if !dryRun {
		if err := os.WriteFile(linuxServiceDst, []byte(unit), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", linuxServiceDst, err)
		}
	}

	// 3b. Write the rootd companion unit. Same binary, different
	//     subcommand (`borgee rootd`), runs as User=root, listens on a
	//     local UDS, accepts only a hardcoded command whitelist. The
	//     unit is written even when PR-1's whitelist is just `ping`
	//     because the systemd-level hardening (AF_UNIX-only, no network,
	//     tight memory/cpu caps) is independent of which commands the
	//     whitelist contains.
	rootdUnit := renderLinuxRootdUnit()
	logStep("write " + linuxRootdServiceDst)
	if !dryRun {
		if err := os.WriteFile(linuxRootdServiceDst, []byte(rootdUnit), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", linuxRootdServiceDst, err)
		}
	}

	// 4. systemctl daemon-reload (best-effort; absent on minimal containers).
	if _, err := exec.LookPath("systemctl"); err == nil {
		logStep("systemctl daemon-reload")
		if !dryRun {
			if err := runCmd("systemctl", "daemon-reload"); err != nil {
				fmt.Fprintf(stderr, "borgee setup: warn: systemctl daemon-reload failed: %v\n", err)
			}
		}
	}

	// 5. Next-step banner.
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "borgee setup: Linux scaffold ready. Next steps:")
	fmt.Fprintln(stdout, "  1. Generate an enrollment in the Borgee web UI")
	fmt.Fprintln(stdout, "  2. sudo borgee claim --enrollment-id=<id> --enrollment-secret=<secret> \\")
	fmt.Fprintln(stdout, "         --server-origin="+serverOrigin)
	fmt.Fprintln(stdout, "  3. sudo systemctl enable --now borgee.service")
	return nil
}

func runDarwin(stdout, stderr io.Writer, serverOrigin string, dryRun bool) error {
	logStep := func(label string) {
		if dryRun {
			fmt.Fprintln(stdout, "[dry-run] "+label)
		} else {
			fmt.Fprintln(stdout, label)
		}
	}

	// 1. Ensure macOS user/group via dscl.
	if _, err := user.Lookup(darwinUser); err != nil {
		logStep("create macOS user " + darwinUser + " (via dscl)")
		if !dryRun {
			if err := ensureMacUser(darwinUser); err != nil {
				return fmt.Errorf("create macOS user %s: %w", darwinUser, err)
			}
		}
	} else {
		logStep("macOS user " + darwinUser + " already exists; skip")
	}

	// 2. Create state dirs + log dir.
	for _, p := range []string{
		darwinStateRoot,
		darwinQueueStateDir,
		darwinStatusStateDir,
		darwinAuditHandoffDir,
		filepath.Join(darwinStateRoot, "credential"),
		darwinLogDir,
		filepath.Dir(darwinUDS),
		darwinAppSupport,
		filepath.Join(darwinRuntimeDir, "bin"),
	} {
		logStep("mkdir " + p)
		if !dryRun {
			if err := os.MkdirAll(p, 0o750); err != nil {
				return fmt.Errorf("mkdir %s: %w", p, err)
			}
			if err := chown(p, darwinUser, darwinUser); err != nil {
				// chown best-effort on macOS — directory ownership matters
				// only for the writable subset; sandbox-exec walls the rest.
				fmt.Fprintf(stderr, "borgee setup: warn: chown %s: %v\n", p, err)
			}
		}
	}

	// 3. Write launchd plist + sandbox profile.
	plist := renderDarwinPlist(serverOrigin)
	logStep("write " + darwinPlistDst)
	if !dryRun {
		if err := os.WriteFile(darwinPlistDst, []byte(plist), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", darwinPlistDst, err)
		}
	}
	sandbox := embeddedSandboxProfile()
	logStep("write " + darwinSandboxDst)
	if !dryRun {
		if err := os.WriteFile(darwinSandboxDst, []byte(sandbox), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", darwinSandboxDst, err)
		}
	}

	// 3b. Write the rootd companion plist. Same binary, `borgee rootd`
	//     subcommand, runs as root, listens on a local UDS, accepts only
	//     a hardcoded command whitelist. Independent of the main plist
	//     so each launchd unit can be loaded/unloaded separately.
	rootdPlist := renderDarwinRootdPlist()
	logStep("write " + darwinRootdPlistDst)
	if !dryRun {
		if err := os.WriteFile(darwinRootdPlistDst, []byte(rootdPlist), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", darwinRootdPlistDst, err)
		}
	}

	// 4. Next-step banner — DO NOT auto-load. Operator must claim first.
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "borgee setup: macOS scaffold ready. Next steps:")
	fmt.Fprintln(stdout, "  1. Generate an enrollment in the Borgee web UI")
	fmt.Fprintln(stdout, "  2. sudo borgee claim --enrollment-id=<id> --enrollment-secret=<secret> \\")
	fmt.Fprintln(stdout, "         --server-origin="+serverOrigin)
	fmt.Fprintln(stdout, "  3. sudo launchctl load -w "+darwinPlistDst)
	return nil
}

// renderLinuxUnit builds the systemd unit content. Keeping the template here
// (rather than embedding from install/) lets `borgee setup` ship as a single
// statically-linked binary — the npm subpackage only carries one file.
func renderLinuxUnit(serverOrigin string) string {
	return `[Unit]
Description=Borgee host-bridge daemon
Documentation=https://github.com/codetreker/borgee
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=5min
StartLimitBurst=5

[Service]
Type=simple
User=` + linuxUser + `
Group=` + linuxGroup + `
ExecStart=` + linuxBinaryPath + ` daemon \
    --socket=` + linuxRunDir + `/borgee.sock \
    --audit-log=` + linuxLogDir + `/audit.log.jsonl \
    --grants-db=` + linuxServerDSN + ` \
    --outbound-server-origin=` + serverOrigin + ` \
    --outbound-allowed-origins=` + serverOrigin + ` \
    --queue-state-dir=` + linuxStateRoot + `/queue \
    --status-state-dir=` + linuxStateRoot + `/status \
    --audit-handoff-dir=` + linuxStateRoot + `/audit-handoff \
    --enrollment-id-file=` + linuxStateRoot + `/credential/enrollment-id \
    --helper-device-id-file=` + linuxStateRoot + `/credential/device-id \
    --helper-credential-file=` + linuxStateRoot + `/credential/credential

# Defense-in-depth sandbox layers (landlock 在 daemon 内 + systemd OS-level).
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
NoNewPrivileges=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
RestrictNamespaces=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
RestrictRealtime=yes
SystemCallArchitectures=native
SystemCallFilter=@system-service

# cgroups resource caps (蓝图 host-bridge.md:57).
MemoryMax=256M
MemoryHigh=192M
CPUQuota=50%
TasksMax=256
IOWeight=100

StateDirectory=borgee
ReadWritePaths=` + linuxLogDir + ` ` + linuxRunDir + ` ` + linuxStateRoot + `/queue ` + linuxStateRoot + `/status ` + linuxStateRoot + `/audit-handoff ` + linuxStateRoot + `/credential
ReadOnlyPaths=` + linuxStateRoot + `

Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
`
}

// renderLinuxRootdUnit builds the systemd unit for the rootd companion
// daemon. Same binary as borgee.service, different subcommand (`borgee
// rootd`), runs as root, locked down with a tight defense-in-depth
// hardening profile because rootd does not need network access at all.
//
// ReadWritePaths covers what PR-4 root commands will need to write to
// (install_plugin → /usr/local/lib/borgee, service_lifecycle → systemd
// units, delegation_revoke → /var/lib/borgee). We set this now so PR-4
// can extend the whitelist without needing to change the unit; the
// systemd-level hardening is independent of which commands are exposed.
func renderLinuxRootdUnit() string {
	return `[Unit]
Description=Borgee root-privileged companion daemon
Documentation=https://github.com/codetreker/borgee
After=network.target

[Service]
Type=simple
User=root
ExecStart=` + linuxBinaryPath + ` rootd \
    --socket=` + linuxRootdSocket + `

# Defense-in-depth: rootd has no network access at all. The only inbound
# path is the local UDS at --socket; the only outbound is to whatever
# system tooling the whitelisted commands invoke (systemctl, etc.).
RestrictAddressFamilies=AF_UNIX
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
RestrictNamespaces=yes
MemoryDenyWriteExecute=yes
SystemCallFilter=@system-service
LockPersonality=yes

MemoryMax=64M
CPUQuota=10%
TasksMax=32

# PR-4 commands write to these paths. Setting them now so PR-4 does not
# need to ship a unit change alongside the executor code.
ReadWritePaths=` + linuxRunDir + ` ` + linuxRuntimeDir + ` ` + linuxStateRoot + ` /etc/systemd/system

Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
`
}

func renderDarwinPlist(serverOrigin string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>` + darwinPlistLabel + `</string>

    <key>ProgramArguments</key>
    <array>
      <string>/usr/bin/sandbox-exec</string>
      <string>-f</string>
      <string>` + darwinSandboxDst + `</string>
      <string>` + darwinBinaryPath + `</string>
      <string>daemon</string>
      <string>--socket=` + darwinUDS + `</string>
      <string>--audit-log=` + darwinAuditLog + `</string>
      <string>--grants-db=` + darwinServerDSN + `</string>
      <string>--outbound-server-origin=` + serverOrigin + `</string>
      <string>--outbound-allowed-origins=` + serverOrigin + `</string>
      <string>--queue-state-dir=` + darwinQueueStateDir + `</string>
      <string>--status-state-dir=` + darwinStatusStateDir + `</string>
      <string>--audit-handoff-dir=` + darwinAuditHandoffDir + `</string>
      <string>--enrollment-id-file=` + darwinStateRoot + `/credential/enrollment-id</string>
      <string>--helper-device-id-file=` + darwinStateRoot + `/credential/device-id</string>
      <string>--helper-credential-file=` + darwinStateRoot + `/credential/credential</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
      <key>SuccessfulExit</key>
      <false/>
    </dict>

    <key>ThrottleInterval</key>
    <integer>10</integer>

    <key>UserName</key>
    <string>` + darwinUser + `</string>

    <key>GroupName</key>
    <string>` + darwinUser + `</string>

    <key>StandardOutPath</key>
    <string>` + darwinLogDir + `/stdout.log</string>

    <key>StandardErrorPath</key>
    <string>` + darwinLogDir + `/stderr.log</string>
  </dict>
</plist>
`
}

// renderDarwinRootdPlist builds the launchd plist for the rootd companion.
// Same binary, `borgee rootd` subcommand, runs as root with GroupName
// `wheel`, no sandbox-exec wrapper (rootd is intentionally root, so the
// helper-daemon sandbox profile would be inappropriate). The plist
// path is kept distinct from the main plist so `launchctl bootstrap` /
// `launchctl bootout` can manage each unit independently.
func renderDarwinRootdPlist() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>` + darwinRootdPlistLabel + `</string>

    <key>ProgramArguments</key>
    <array>
      <string>` + darwinBinaryPath + `</string>
      <string>rootd</string>
      <string>--socket=` + darwinRootdSocket + `</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
      <key>SuccessfulExit</key>
      <false/>
    </dict>

    <key>ThrottleInterval</key>
    <integer>10</integer>

    <key>UserName</key>
    <string>root</string>

    <key>GroupName</key>
    <string>wheel</string>

    <key>StandardOutPath</key>
    <string>` + darwinLogDir + `/rootd-stdout.log</string>

    <key>StandardErrorPath</key>
    <string>` + darwinLogDir + `/rootd-stderr.log</string>
  </dict>
</plist>
`
}

// embeddedSandboxProfile returns the macOS sandbox-exec profile contents. We
// deliberately keep this short — the helper daemon itself applies a finer
// runtime sandbox (internal/sandbox/sandbox_darwin.go); sandbox-exec is just
// the outer OS-level guard rail.
func embeddedSandboxProfile() string {
	return `(version 1)
(deny default)
(allow process-exec)
(allow process-fork)
(allow file-read*)
(allow file-write*
  (subpath "` + darwinStateRoot + `")
  (subpath "` + darwinLogDir + `")
  (literal "` + darwinUDS + `"))
(allow network*)
(allow signal (target self))
(allow mach-lookup)
(allow sysctl-read)
(allow iokit-open)
`
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func chown(path, username, groupname string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}
	var uid, gid int
	if _, err := fmt.Sscanf(u.Uid, "%d", &uid); err != nil {
		return err
	}
	if _, err := fmt.Sscanf(u.Gid, "%d", &gid); err != nil {
		return err
	}
	if g, err := user.LookupGroup(groupname); err == nil {
		if _, err := fmt.Sscanf(g.Gid, "%d", &gid); err != nil {
			return err
		}
	}
	return os.Chown(path, uid, gid)
}

// ensureMacUser creates a hidden _borgee user/group via dscl. dscl errors are
// non-trivial to parse so we treat "create" as best-effort and let the
// follow-up file ops fail loudly if the user really wasn't created.
func ensureMacUser(username string) error {
	// Find first free uid in the [200,400) range that's free for service
	// accounts on macOS (system reserved < 500 but Apple uses < 200 for its
	// own daemons).
	uid := pickFreeMacUID()
	if uid < 0 {
		return errors.New("no free macOS service uid in 200..400")
	}
	idStr := fmt.Sprintf("%d", uid)
	steps := [][]string{
		{"dscl", ".", "-create", "/Groups/" + username},
		{"dscl", ".", "-create", "/Groups/" + username, "PrimaryGroupID", idStr},
		{"dscl", ".", "-create", "/Users/" + username},
		{"dscl", ".", "-create", "/Users/" + username, "UserShell", "/usr/bin/false"},
		{"dscl", ".", "-create", "/Users/" + username, "RealName", "Borgee Helper"},
		{"dscl", ".", "-create", "/Users/" + username, "UniqueID", idStr},
		{"dscl", ".", "-create", "/Users/" + username, "PrimaryGroupID", idStr},
		{"dscl", ".", "-create", "/Users/" + username, "NFSHomeDirectory", "/var/empty"},
		{"dscl", ".", "-create", "/Users/" + username, "IsHidden", "1"},
	}
	for _, s := range steps {
		if err := runCmd(s[0], s[1:]...); err != nil {
			return fmt.Errorf("%v: %w", s, err)
		}
	}
	return nil
}

func pickFreeMacUID() int {
	for uid := 200; uid < 400; uid++ {
		if _, err := user.LookupId(fmt.Sprintf("%d", uid)); err != nil {
			return uid
		}
	}
	return -1
}
