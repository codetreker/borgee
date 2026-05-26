//go:build linux || darwin

// Package setup — `borgee setup` subcommand.
//
// Replaces the .deb / .pkg postinstall scripts that the prior nfpm-based
// distribution used. The operator-facing `install` command calls this helper
// after resolving the installing user:
//
//   - Linux:
//     1. mkdir user-owned Borgee state/config/data dirs under the install user's home
//     2. Write ~/.config/systemd/user/borgee.service from the embedded template
//     3. Prepare the per-uid rootd service template for privileged install
//     5. Print next-step (claim → start) — do NOT auto-start
//   - macOS:
//     1. mkdir user-owned Borgee state/config/data dirs under the install user's home
//     2. Write ~/Library/LaunchAgents/cloud.borgee.host-bridge.plist + sandbox profile
//     3. Prepare the per-uid rootd LaunchDaemon template for privileged install
//     4. Print next-step (claim → start) — do NOT auto-load
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
	// linuxBinaryPath is the shared root-owned binary used by both the
	// user-level main daemon and the rootd companion. The services do not
	// depend on npx's temporary cache or npm's global shim.
	linuxBinaryPath = "/usr/local/borgee/bin/borgee"
	linuxRuntimeDir = "/usr/local/borgee"
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

	darwinUser       = "_borgee"
	darwinStateRoot  = "/Library/Application Support/Borgee/Helper"
	darwinAppSupport = "/Library/Application Support/Borgee"
	darwinLogDir     = "/Library/Logs/Borgee"
	darwinPlistDst   = "/Library/LaunchDaemons/cloud.borgee.host-bridge.plist"
	darwinSandboxDst = "/Library/Application Support/Borgee/borgee-helper.sb"
	// macOS persistent binary path. Same shared-binary contract as Linux.
	darwinBinaryPath      = "/usr/local/borgee/bin/borgee"
	darwinRuntimeDir      = "/usr/local/borgee"
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

type UserLayout struct {
	Username        string
	UID             int
	GID             int
	HomeDir         string
	BinaryPath      string
	InstallPrefix   string
	StateRoot       string
	LogDir          string
	ConfigDir       string
	UserUnitPath    string
	UserSocket      string
	RootdSocket     string
	RootdBinaryPath string
	RootdService    string
	RootdServiceDst string
	ServerDSN       string
}

func LinuxUserLayout(username string, uid, gid int, homeDir string) UserLayout {
	return LinuxUserLayoutWithInstallPrefix(username, uid, gid, homeDir, linuxRuntimeDir)
}

func LinuxUserLayoutWithInstallPrefix(username string, uid, gid int, homeDir string, installPrefix string) UserLayout {
	stateRoot := filepath.Join(homeDir, ".local", "state", "borgee")
	binaryPath := filepath.Join(installPrefix, "bin", "borgee")
	return UserLayout{
		Username:        username,
		UID:             uid,
		GID:             gid,
		HomeDir:         homeDir,
		BinaryPath:      binaryPath,
		InstallPrefix:   installPrefix,
		StateRoot:       stateRoot,
		LogDir:          filepath.Join(stateRoot, "log"),
		ConfigDir:       filepath.Join(homeDir, ".config", "systemd", "user"),
		UserUnitPath:    filepath.Join(homeDir, ".config", "systemd", "user", "borgee.service"),
		UserSocket:      "%t/borgee/borgee.sock",
		RootdSocket:     filepath.Join("/run/borgee", fmt.Sprintf("%d", uid), "borgee-rootd.sock"),
		RootdBinaryPath: binaryPath,
		RootdService:    fmt.Sprintf("borgee-rootd-%d.service", uid),
		RootdServiceDst: filepath.Join("/etc/systemd/system", fmt.Sprintf("borgee-rootd-%d.service", uid)),
		ServerDSN:       "file:" + filepath.Join(stateRoot, "server.db") + "?mode=ro&_busy_timeout=5000",
	}
}

func DarwinUserLayout(username string, uid, gid int, homeDir string) UserLayout {
	return DarwinUserLayoutWithInstallPrefix(username, uid, gid, homeDir, darwinRuntimeDir)
}

func DarwinUserLayoutWithInstallPrefix(username string, uid, gid int, homeDir string, installPrefix string) UserLayout {
	stateRoot := filepath.Join(homeDir, "Library", "Application Support", "Borgee", "Helper")
	binaryPath := filepath.Join(installPrefix, "bin", "borgee")
	return UserLayout{
		Username:        username,
		UID:             uid,
		GID:             gid,
		HomeDir:         homeDir,
		BinaryPath:      binaryPath,
		InstallPrefix:   installPrefix,
		StateRoot:       stateRoot,
		LogDir:          filepath.Join(homeDir, "Library", "Logs", "Borgee"),
		ConfigDir:       filepath.Join(homeDir, "Library", "LaunchAgents"),
		UserUnitPath:    filepath.Join(homeDir, "Library", "LaunchAgents", "cloud.borgee.host-bridge.plist"),
		UserSocket:      filepath.Join(homeDir, "Library", "Application Support", "Borgee", "borgee.sock"),
		RootdSocket:     filepath.Join("/Users/Shared/Borgee", fmt.Sprintf("%d", uid), "borgee-rootd.sock"),
		RootdBinaryPath: binaryPath,
		RootdService:    "cloud.borgee.host-bridge.rootd." + fmt.Sprintf("%d", uid),
		RootdServiceDst: "/Library/LaunchDaemons/cloud.borgee.host-bridge.rootd." + fmt.Sprintf("%d", uid) + ".plist",
		ServerDSN:       "file:" + filepath.Join(stateRoot, "server.db") + "?mode=ro&_busy_timeout=5000",
	}
}

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
	LinuxRootdServiceDst  = linuxRootdServiceDst
	LinuxRootdServiceName = "borgee-rootd.service"
	LinuxRootdSocket      = linuxRootdSocket
	DarwinRootdPlistDst   = darwinRootdPlistDst
	DarwinRootdPlistLabel = darwinRootdPlistLabel
	DarwinRootdSocket     = darwinRootdSocket
)

// Run is the entry for `borgee setup`. Dispatcher in cmd/borgee passes the
// remaining argv + stdio.
func Run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("borgee setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "Print what would be done without touching the system")
	serverOrigin := fs.String("server-origin", "wss://app.borgee.io", "Borgee server WS origin to bake into the systemd/launchd unit (wss:// for the daemon's persistent transport, PR-2 #1038)")
	allowInsecureOrigin := fs.Bool("allow-insecure-server-origin", false, "Allow http:// / ws:// server-origin (test environments only)")
	installUsername := fs.String("install-username", "", "User that owns the main daemon service")
	installUID := fs.Int("install-uid", -1, "UID that owns the main daemon service")
	installGID := fs.Int("install-gid", -1, "Primary GID that owns the main daemon service")
	installHome := fs.String("install-home", "", "Home directory for user-owned Borgee state and service files")
	installPrefix := fs.String("install-prefix", "", "Shared root-owned Borgee install prefix (default /usr/local/borgee)")
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

	layout, err := layoutForCurrentPlatform(*installUsername, *installUID, *installGID, *installHome, *installPrefix)
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "linux":
		return runLinux(stdout, stderr, *serverOrigin, layout, *dryRun)
	case "darwin":
		return runDarwin(stdout, stderr, *serverOrigin, layout, *dryRun)
	default:
		return fmt.Errorf("borgee setup: unsupported platform %q", runtime.GOOS)
	}
}

func layoutForCurrentPlatform(username string, uid, gid int, homeDir string, installPrefix string) (UserLayout, error) {
	if username == "" || uid < 0 || gid < 0 || homeDir == "" {
		u, err := user.Current()
		if err != nil {
			return UserLayout{}, fmt.Errorf("current user: %w", err)
		}
		if username == "" {
			username = u.Username
		}
		if homeDir == "" {
			homeDir = u.HomeDir
		}
		if uid < 0 {
			var parsed int
			if _, err := fmt.Sscanf(u.Uid, "%d", &parsed); err != nil {
				return UserLayout{}, fmt.Errorf("parse uid %q: %w", u.Uid, err)
			}
			uid = parsed
		}
		if gid < 0 {
			var parsed int
			if _, err := fmt.Sscanf(u.Gid, "%d", &parsed); err != nil {
				return UserLayout{}, fmt.Errorf("parse gid %q: %w", u.Gid, err)
			}
			gid = parsed
		}
	}
	if runtime.GOOS == "darwin" {
		if installPrefix != "" {
			return DarwinUserLayoutWithInstallPrefix(username, uid, gid, homeDir, installPrefix), nil
		}
		return DarwinUserLayout(username, uid, gid, homeDir), nil
	}
	if installPrefix != "" {
		return LinuxUserLayoutWithInstallPrefix(username, uid, gid, homeDir, installPrefix), nil
	}
	return LinuxUserLayout(username, uid, gid, homeDir), nil
}

func runLinux(stdout, stderr io.Writer, serverOrigin string, layout UserLayout, dryRun bool) error {
	logStep := func(label string) {
		if dryRun {
			fmt.Fprintln(stdout, "[dry-run] "+label)
		} else {
			fmt.Fprintln(stdout, label)
		}
	}

	stateSubdirs := []string{"queue", "status", "audit-handoff", "credential", "openclaw", "plugins", "state"}
	for _, sub := range stateSubdirs {
		p := filepath.Join(layout.StateRoot, sub)
		logStep("mkdir " + p)
		if !dryRun {
			if err := os.MkdirAll(p, 0o750); err != nil {
				return fmt.Errorf("mkdir %s: %w", p, err)
			}
		}
	}
	for _, p := range []string{layout.LogDir, filepath.Dir(layout.UserUnitPath)} {
		logStep("mkdir " + p)
		if !dryRun {
			if err := os.MkdirAll(p, 0o750); err != nil {
				return fmt.Errorf("mkdir %s: %w", p, err)
			}
		}
	}

	unit := renderLinuxUserUnit(serverOrigin, layout)
	logStep("write " + layout.UserUnitPath)
	if !dryRun {
		if err := os.WriteFile(layout.UserUnitPath, []byte(unit), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", layout.UserUnitPath, err)
		}
	}

	logStep("seed " + layout.ServerDSN)
	if !dryRun {
		if err := seedHostGrantsDB(layout.ServerDSN, "", ""); err != nil {
			return err
		}
	}

	rootdUnit := renderLinuxRootdUnit(layout)
	logStep("write " + layout.RootdServiceDst)
	if !dryRun {
		if os.Geteuid() == 0 {
			if err := os.WriteFile(layout.RootdServiceDst, []byte(rootdUnit), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", layout.RootdServiceDst, err)
			}
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
	fmt.Fprintln(stdout, "  2. borgee claim --enrollment-id=<id> --enrollment-secret=<secret> \\")
	fmt.Fprintln(stdout, "         --server-origin="+serverOrigin)
	fmt.Fprintln(stdout, "  3. systemctl --user enable --now borgee.service")
	return nil
}

func runDarwin(stdout, stderr io.Writer, serverOrigin string, layout UserLayout, dryRun bool) error {
	logStep := func(label string) {
		if dryRun {
			fmt.Fprintln(stdout, "[dry-run] "+label)
		} else {
			fmt.Fprintln(stdout, label)
		}
	}

	for _, p := range []string{
		layout.StateRoot,
		filepath.Join(layout.StateRoot, "QueueState"),
		filepath.Join(layout.StateRoot, "StatusState"),
		filepath.Join(layout.StateRoot, "AuditHandoff"),
		filepath.Join(layout.StateRoot, "credential"),
		layout.LogDir,
		filepath.Dir(layout.UserSocket),
		filepath.Join(layout.HomeDir, "Library", "Application Support", "Borgee", "openclaw"),
		filepath.Join(layout.HomeDir, "Library", "Application Support", "Borgee", "plugins"),
		filepath.Join(layout.HomeDir, "Library", "Application Support", "Borgee", "state"),
	} {
		logStep("mkdir " + p)
		if !dryRun {
			if err := os.MkdirAll(p, 0o750); err != nil {
				return fmt.Errorf("mkdir %s: %w", p, err)
			}
		}
	}

	// 3. Write launchd plist + sandbox profile.
	plist := renderDarwinUserPlist(serverOrigin, layout)
	logStep("write " + layout.UserUnitPath)
	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(layout.UserUnitPath), 0o750); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(layout.UserUnitPath), err)
		}
		if err := os.WriteFile(layout.UserUnitPath, []byte(plist), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", layout.UserUnitPath, err)
		}
	}
	sandbox := embeddedSandboxProfile()
	logStep("write " + darwinSandboxDst)
	if !dryRun {
		if err := os.WriteFile(darwinSandboxDst, []byte(sandbox), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", darwinSandboxDst, err)
		}
	}

	// 3a. Seed the host_grants SQLite DB the daemon opens with mode=ro.
	// Mirrors the Linux step — see runLinux for rationale.
	logStep("seed " + layout.ServerDSN)
	if !dryRun {
		if err := seedHostGrantsDB(layout.ServerDSN, "", ""); err != nil {
			return err
		}
	}

	// 3b. Write the rootd companion plist. Same binary, `borgee rootd`
	//     subcommand, runs as root, listens on a local UDS, accepts only
	//     a hardcoded command whitelist. Independent of the main plist
	//     so each launchd unit can be loaded/unloaded separately.
	rootdPlist := renderDarwinRootdPlist(layout)
	logStep("write " + layout.RootdServiceDst)
	if !dryRun {
		if os.Geteuid() == 0 {
			if err := os.WriteFile(layout.RootdServiceDst, []byte(rootdPlist), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", layout.RootdServiceDst, err)
			}
		}
	}

	// 4. Next-step banner — DO NOT auto-load. Operator must claim first.
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "borgee setup: macOS scaffold ready. Next steps:")
	fmt.Fprintln(stdout, "  1. Generate an enrollment in the Borgee web UI")
	fmt.Fprintln(stdout, "  2. borgee claim --enrollment-id=<id> --enrollment-secret=<secret> \\")
	fmt.Fprintln(stdout, "         --server-origin="+serverOrigin)
	fmt.Fprintln(stdout, "  3. launchctl bootstrap gui/$(id -u) "+layout.UserUnitPath)
	return nil
}

// renderLinuxUnit builds the systemd unit content. Keeping the template here
// (rather than embedding from install/) lets `borgee setup` ship as a single
// statically-linked binary — the npm subpackage only carries one file.
func renderLinuxUserUnit(serverOrigin string, layout UserLayout) string {
	return `[Unit]
Description=Borgee host-bridge daemon
Documentation=https://github.com/codetreker/borgee
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=5min
StartLimitBurst=5

[Service]
Type=simple
ExecStart=` + layout.BinaryPath + ` daemon \
    --socket=` + layout.UserSocket + ` \
    --audit-log=` + layout.LogDir + `/audit.log.jsonl \
    --grants-db=` + layout.ServerDSN + ` \
    --rootd-socket=` + layout.RootdSocket + ` \
    --outbound-server-origin=` + serverOrigin + ` \
    --outbound-allowed-origins=` + serverOrigin + ` \
    --queue-state-dir=` + layout.StateRoot + `/queue \
    --status-state-dir=` + layout.StateRoot + `/status \
    --audit-handoff-dir=` + layout.StateRoot + `/audit-handoff \
    --enrollment-id-file=` + layout.StateRoot + `/credential/enrollment-id \
    --helper-device-id-file=` + layout.StateRoot + `/credential/device-id \
    --helper-credential-file=` + layout.StateRoot + `/credential/credential

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
# @sandbox covers landlock_create_ruleset / landlock_add_rule / landlock_restrict_self —
# the daemon's in-process landlock layer SIGSYS-dies without it. The two
# groups are additive per systemd-syscall-filter(7).
SystemCallFilter=@system-service @sandbox

# cgroups resource caps (蓝图 host-bridge.md:57).
MemoryMax=256M
MemoryHigh=192M
CPUQuota=50%
TasksMax=256
IOWeight=100

# RuntimeDirectory=borgee: systemd creates /run/borgee with helper
# ownership before ExecStart and removes it on stop. Without this
# directive /run is tmpfs and the daemon cannot bind its UDS after a
# reboot (root-only mkdir, helper user fails).
RuntimeDirectory=borgee
RuntimeDirectoryMode=0750
# ReadWritePaths must align with the signed canonical helper-policy
# manifest (server-go internal/helpermanifest.BuildLinux) Path
# declarations. Misalignment fails loud at write — the executor does not
# invent fallback paths. The rootd companion (separate unit) covers any
# path requiring root + a different ReadWritePaths set.
#
# Path roots (each maps to a manifest PathID):
#   queue/status/audit-handoff/credential under the user's Borgee state root
#   openclaw/plugins/state under the user's Borgee state root
#   log dir under the user's Borgee state root
#   %t/borgee                    — user runtime UDS dir
ReadWritePaths=` + layout.LogDir + ` ` + layout.StateRoot + `/queue ` + layout.StateRoot + `/status ` + layout.StateRoot + `/audit-handoff ` + layout.StateRoot + `/credential ` + layout.StateRoot + `/openclaw ` + layout.StateRoot + `/plugins ` + layout.StateRoot + `/state
ReadOnlyPaths=` + layout.StateRoot + `

Restart=on-failure
RestartSec=10s

[Install]
WantedBy=default.target
`
}

func renderLinuxUnit(serverOrigin string) string {
	return renderLinuxUserUnit(serverOrigin, LinuxUserLayout(linuxUser, 0, 0, "/var/lib/borgee"))
}

// renderLinuxRootdUnit builds the systemd unit for the rootd companion
// daemon. Same binary as borgee.service, different subcommand (`borgee
// rootd`), runs as root, locked down with a tight defense-in-depth
// hardening profile because rootd does not need network access at all.
//
// ReadWritePaths covers what PR-4 root commands will need to write to
// (install_plugin → /usr/local/borgee, service_lifecycle → systemd
// units, delegation_revoke → /var/lib/borgee). We set this now so PR-4
// can extend the whitelist without needing to change the unit; the
// systemd-level hardening is independent of which commands are exposed.
func renderLinuxRootdUnit(layout UserLayout) string {
	return `[Unit]
Description=Borgee root-privileged companion daemon
Documentation=https://github.com/codetreker/borgee
After=network.target

[Service]
Type=simple
User=root
ExecStart=` + layout.RootdBinaryPath + ` rootd \
    --socket=` + layout.RootdSocket + ` \
    --allowed-peer-uid=` + fmt.Sprintf("%d", layout.UID) + ` \
    --socket-owner-uid=` + fmt.Sprintf("%d", layout.UID) + ` \
    --socket-owner-gid=` + fmt.Sprintf("%d", layout.GID) + `

# Defense-in-depth: rootd's whitelisted commands include install_plugin,
# which invokes install-butler in-process to GET the signed plugin
# manifest + artifact bytes over HTTPS. AF_INET/AF_INET6 are therefore
# required outbound families. AF_UNIX remains for the inbound rootd
# control socket. No other socket families are permitted; the rest of
# the hardening (NoNewPrivileges / ProtectSystem / etc.) still binds.
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
RestrictNamespaces=yes
MemoryDenyWriteExecute=yes
# @sandbox kept aligned with borgee.service: even though the current
# rootd surface does not invoke landlock, the unit's SystemCallFilter
# is deliberately the same shape to avoid silently diverging hardening
# between the two daemons. Additive group syntax per systemd-syscall-
# filter(7).
SystemCallFilter=@system-service @sandbox
LockPersonality=yes

MemoryMax=64M
CPUQuota=10%
TasksMax=32

# RuntimeDirectory=borgee: rootd binds its UDS at /run/borgee/borgee-rootd.sock,
# so /run/borgee must exist before ExecStart. borgee.service also declares
# the same RuntimeDirectory — systemd handles two units sharing one
# RuntimeDirectory cleanly: it is created on first start and removed when
# the last user stops. Without this directive, rootd fails on first boot
# with "Failed to set up mount namespacing: /run/borgee: No such file or
# directory" until borgee.service has run and lazily created the dir
# (Restart=on-failure masked it, but polluted journals and delayed first
# job acceptance — issue #1053).
RuntimeDirectory=borgee/` + fmt.Sprintf("%d", layout.UID) + `
RuntimeDirectoryMode=0750

# PR-4 commands write to these paths. Setting them now so PR-4 does not
# need to ship a unit change alongside the executor code.
ReadWritePaths=` + filepath.Dir(layout.RootdSocket) + ` ` + filepath.Dir(filepath.Dir(layout.RootdBinaryPath)) + ` ` + layout.StateRoot + ` /etc/systemd/system

Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
`
}

func RenderLinuxRootdUnit(layout UserLayout) string {
	return renderLinuxRootdUnit(layout)
}

func renderDarwinPlist(serverOrigin string) string {
	return renderDarwinUserPlist(serverOrigin, DarwinUserLayout(darwinUser, 0, 0, "/var/empty"))
}

func renderDarwinUserPlist(serverOrigin string, layout UserLayout) string {
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
      <string>` + layout.BinaryPath + `</string>
      <string>daemon</string>
      <string>--socket=` + layout.UserSocket + `</string>
      <string>--audit-log=` + layout.LogDir + `/audit.log.jsonl</string>
      <string>--grants-db=` + layout.ServerDSN + `</string>
      <string>--rootd-socket=` + layout.RootdSocket + `</string>
      <string>--outbound-server-origin=` + serverOrigin + `</string>
      <string>--outbound-allowed-origins=` + serverOrigin + `</string>
      <string>--queue-state-dir=` + layout.StateRoot + `/QueueState</string>
      <string>--status-state-dir=` + layout.StateRoot + `/StatusState</string>
      <string>--audit-handoff-dir=` + layout.StateRoot + `/AuditHandoff</string>
      <string>--enrollment-id-file=` + layout.StateRoot + `/credential/enrollment-id</string>
      <string>--helper-device-id-file=` + layout.StateRoot + `/credential/device-id</string>
      <string>--helper-credential-file=` + layout.StateRoot + `/credential/credential</string>
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

    <key>StandardOutPath</key>
    <string>` + layout.LogDir + `/stdout.log</string>

    <key>StandardErrorPath</key>
    <string>` + layout.LogDir + `/stderr.log</string>
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
func renderDarwinRootdPlist(layouts ...UserLayout) string {
	layout := DarwinUserLayout(darwinUser, 0, 0, "/var/empty")
	if len(layouts) > 0 {
		layout = layouts[0]
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>` + layout.RootdService + `</string>

    <key>ProgramArguments</key>
    <array>
      <string>` + layout.RootdBinaryPath + `</string>
      <string>rootd</string>
      <string>--socket=` + layout.RootdSocket + `</string>
      <string>--allowed-peer-uid=` + fmt.Sprintf("%d", layout.UID) + `</string>
      <string>--socket-owner-uid=` + fmt.Sprintf("%d", layout.UID) + `</string>
      <string>--socket-owner-gid=` + fmt.Sprintf("%d", layout.GID) + `</string>
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

func RenderDarwinRootdPlist(layout UserLayout) string {
	return renderDarwinRootdPlist(layout)
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
