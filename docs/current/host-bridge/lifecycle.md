# Helper Lifecycle — reboot, crash, reconnect without user login

Scope: this doc explains *why* the host-bridge helper daemon survives both
a clean reboot and a process crash without anyone logging into the host.
For the asset contents themselves see
[`packages/borgee-helper/install/borgee-helper.service`](../../../packages/borgee-helper/install/borgee-helper.service)
(Linux), [`packages/borgee-helper/install/cloud.borgee.host-bridge.plist`](../../../packages/borgee-helper/install/cloud.borgee.host-bridge.plist)
(macOS), and [`helper-daemon.md`](./helper-daemon.md) for the steady-state
daemon contract.

## Why this matters

#968 says "the machine must remain controllable from web across reboot AND
crash, with no local user re-login". Parent #659 only covered the boot-time
autostart half. This doc walks the full chain so reviewers can verify each
mechanism without spinning up a real host.

## Linux reboot chain

1. PID 1 systemd reaches `multi-user.target` (the standard non-graphical
   boot target — graphical `default.target` is *not* a prerequisite).
2. `borgee-helper.service` declares `WantedBy=multi-user.target` and was
   linked into that target's `.wants` set when the installer ran
   `systemctl enable borgee-helper.service`. systemd therefore starts the
   unit as part of the target.
3. Ordering directives `After=network-online.target` +
   `Wants=network-online.target` keep the start from racing the network
   layer, so the helper does not crash-loop just because routes are not
   ready yet.
4. The unit runs `Type=simple` — systemd considers the service started
   as soon as `ExecStart=` exec's, with no daemonisation handshake.
5. The daemon then opens `/var/lib/borgee/server.db` (read-only, via
   `ReadOnlyPaths=/var/lib/borgee`) for `host_grants`. That DB is owned
   by system root/`borgee-helper`; no user session is involved.
6. `cmd/borgee-helper/main.go::run` continues with:
   `outbound.ValidateAndPrepare(...)` (validates `--outbound-server-origin`
   against the `--outbound-allowed-origins` allowlist and sets up TLS /
   dialer state for a *future* outbound poller — see negative scope
   below), then `sandbox.Apply(...)`, then opens the UDS socket and
   enters `ln.Accept()` for IPC. At this point the daemon is alive and
   accepts local IPC, but it does **not** by itself dial the API server.

### Reconnect chain — what's actually wired vs deliberately deferred

What the daemon does on every (re)start today (v0(D)):

- Reads CLI flags, runs `outbound.ValidateAndPrepare` (origin / allowlist
  / state-dir validation only — does **not** open a connection).
- Opens grants DB read-only and applies the sandbox profile.
- Opens the UDS socket and enters the `Accept()` loop.

What the daemon does **not** do today: outbound heartbeat. There is no
production caller of `outbound.Client.Poll` and no daemon-side
`POST /api/v1/helper/enrollments/{id}/status`. The server endpoint
(`packages/server-go/internal/api/helper_enrollments.go::handleStatus`)
exists and accepts heartbeats; the server's `serializeWithConfigure`
derives `connected` / `offline` from `LastSeenAt` against a 5-minute
freshness window. Today only an external orchestrator (the OpenClaw
configure flow / web-side enrollment manager) writes `LastSeenAt`; the
helper daemon itself does not.

This is intentional for #968's scope: the issue covers the boot/crash
*autostart* half (systemd unit + launchd plist + install plan) and is
tested at byte level by G1/G2/G3
(`outbound_prereq_assets_test.go::TestLinux/MacOSServiceBootCrashRestartIsBounded`,
`deploy_test.go::TestHB1B_*Plan_*`). What G1/G2/G3 prove: a rebooted or
crashed machine brings the daemon back up under the system service
manager, with no user login, and the UDS endpoint becomes available.
What they do **not** prove: that the daemon then re-establishes an
outbound session with the API server — because the daemon itself does
not do that yet.

G5 (`helper_enrollments_lifecycle_test.go::TestHelperEnrollmentStatus_*`)
locks the server-side derivation as a **forward contract**: the moment
a daemon revision starts writing `LastSeenAt` (whether by directly
calling `outbound.Client.Poll` or by some future heartbeat goroutine),
the server's `status` field flips to `connected` automatically with no
additional server-side change. The boundary, fresh-after-stale, and
revoked/uninstalled-precedence cases are all locked. Until that daemon
work lands, the displayed status will reflect whichever component last
wrote `LastSeenAt` — typically only the configure flow at claim time.

TODO / follow-up: wire a daemon-side heartbeat loop that calls
`outbound.Client.Poll` (or a thinner `POST /status`) at the freshness
cadence. That work is out of scope for #968 (boot/crash survival) and
should be filed as a separate issue so it does not silently expand this
PR.

## Linux crash chain

1. Daemon exits non-zero (panic, OOM kill, transient I/O, etc.).
2. systemd sees `Restart=on-failure` and waits `RestartSec=10s` before
   the next start. This pacing avoids a tight respawn loop while still
   recovering quickly on transient failures.
3. `StartLimitIntervalSec=5min` + `StartLimitBurst=5` cap the autorestart
   to 5 attempts per 5 minutes. Past the burst, systemd marks the unit
   inactive and stops trying; an operator (or external alerting on
   `systemctl is-failed`) must intervene. This is intentional — an
   un-bounded restart loop would mask a real bug.
4. Each restart re-enters the boot chain from step 5 above, so the
   post-restart wiring (grants DB reopen, sandbox apply, UDS Accept loop)
   is identical to the reboot path. Whether the daemon then needs to
   re-establish any outbound session depends on the outbound poller
   feature — see the "Reconnect chain" subsection above.

## macOS equivalents

The plist installs to `/Library/LaunchDaemons/` — a *LaunchDaemon*, not a
*LaunchAgent*. LaunchDaemons run under launchd's system domain and load
before any user logs in; LaunchAgents would only load inside a user
session and would defeat the "no user login" requirement (the installer
deploy plan test guards against an accidental switch).

- `RunAtLoad=true` — equivalent to `WantedBy=multi-user.target` +
  `systemctl start`; launchd starts the daemon as soon as the LaunchDaemon
  is loaded, which happens at boot.
- `KeepAlive.SuccessfulExit=false` — equivalent to `Restart=on-failure`;
  launchd only respawns on non-zero exit.
- `ThrottleInterval=10` — equivalent to `RestartSec=10s`; launchd waits
  at least 10s between respawns. macOS does not expose a direct burst cap
  the way systemd does, but the throttle prevents a spin loop.
- Run user: `UserName=_borgee-helper` (system-only `_` prefix), again
  ensuring no user session is required.

Install form: the installer's `DarwinPlan` invokes
`sudo launchctl load /Library/LaunchDaemons/cloud.borgee.host-bridge.plist`
to register the LaunchDaemon. `launchctl load` is the form currently
used (see `deploy.go::DarwinPlan` and `deploy_test.go::TestHB1B_DarwinPlan_HasSudoAndLaunchd`).
The modern alternative is `launchctl bootstrap system /Library/LaunchDaemons/cloud.borgee.host-bridge.plist`
(supported since 10.10, deprecated `load` in favor of the domain-aware
form). Switching to `bootstrap system` is a follow-up — it would tighten
error reporting on macOS 11+ but does not affect the reboot/crash
survival contract this doc is locking, so it is intentionally deferred.

## Persisted vs ephemeral state

Survives reboot and crash:

- Host grants DB (`/var/lib/borgee/server.db` Linux,
  `/Library/Application Support/Borgee/server.db` macOS) — read-only to
  the helper, owned by the system, populated by the installer / OpenClaw
  configure flow.
- Queue, status, and audit-handoff state directories
  (`/var/lib/borgee-helper/{queue,status,audit-handoff}` Linux,
  `/Library/Application Support/Borgee/Helper/{QueueState,StatusState,AuditHandoff}`
  macOS) — Helper-owned, append/replay safe.

Does *not* survive (intentionally ephemeral):

- Outbound WebSocket to the Borgee server — re-established on every start.
- In-memory caches and lease tokens — re-issued by the server on reconnect.

## Why no user login is required

- Linux: `User=borgee-helper` / `Group=borgee-helper` is a *system* user
  (created by the `.deb` postinst, UID < 1000). `loginctl enable-linger`
  is **not** needed because systemd PID 1 starts system units directly
  via `multi-user.target`; `enable-linger` is only relevant to user-mode
  systemd instances under `--user`.
- macOS: `_borgee-helper` is a system role account (`_` prefix); launchd
  starts the LaunchDaemon in the system domain at boot. No Aqua / Finder
  session, no console login, no SSH session is required.

## Windows

Out of scope for v1. `packages/borgee-installer/internal/deploy/deploy.go::PlanForCurrentOS`
fails fast with `Windows support planned for v2` so an operator cannot
silently get a half-installed Windows host. The user outcome ("remains
controllable across reboot/crash") therefore does not apply on Windows
in v1 — there is no install path to break.

## Test coverage map

| Mechanism                                  | Test file                                                                          | Test name                                       |
|--------------------------------------------|------------------------------------------------------------------------------------|-------------------------------------------------|
| systemd unit boot/crash directives present | `packages/borgee-helper/install/outbound_prereq_assets_test.go`                    | `TestLinuxServiceBootCrashRestartIsBounded`     |
| launchd plist boot/crash directives present| `packages/borgee-helper/install/outbound_prereq_assets_test.go`                    | `TestMacOSServiceBootCrashRestartIsBounded`     |
| Installer wires `systemctl enable` in order| `packages/borgee-installer/internal/deploy/deploy_test.go`                         | `TestHB1B_LinuxPlan_HasSudoAndSystemd`          |
| Installer wires `launchctl load` to system | `packages/borgee-installer/internal/deploy/deploy_test.go`                         | `TestHB1B_DarwinPlan_HasSudoAndLaunchd`         |
| Server flips `connected`/`offline` by freshness | `packages/server-go/internal/api/helper_enrollments_lifecycle_test.go`        | `TestHelperEnrollmentStatus_*`                  |

The byte-level asset assertions plus the installer-plan ordering plus the
server-side freshness derivation together stand in for a real reboot/crash
e2e (which a CI sandbox cannot perform).
