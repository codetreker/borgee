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

## Operator-side claim (one-time, post-install)

The host-bridge stack ships two short-lived CLIs that the operator (or the
`.deb` / `.pkg` postinstall script) invokes once. They both exit before the
long-lived `borgee-helper` daemon ever runs, so neither lingers as root.

- `install-butler` ([`packages/borgee-helper/cmd/install-butler`](../../../packages/borgee-helper/cmd/install-butler/README.md))
  — one-shot signed-manifest installer. Downloads a verified runtime
  binary (manifest fetch → ed25519 verify → SHA256 verify → atomic rename
  → chmod → optional chown → exit). Run via `sudo`; drops privilege by
  exiting. See its README for flags + failure modes.
- `borgee-helper-claim` — pairs the host with a Borgee server enrollment
  (chain below).

1. Operator generates an enrollment in the web UI. The server returns an
   `enrollment_id` plus a one-time `enrollment_secret` (15-minute TTL — see
   [`helper_enrollment_queries.go`](../../../packages/server-go/internal/store/helper_enrollment_queries.go)).
2. Operator runs the claim CLI on the target machine, typically as root via
   `sudo`:

   ```
   sudo borgee-helper-claim \
       --enrollment-id <id> \
       --enrollment-secret <secret> \
       --server-origin https://app.borgee.io
   ```

   The CLI derives a stable `helper_device_id` (Linux `/etc/machine-id`,
   macOS `IOPlatformUUID`, falling back to a persisted UUIDv4), POSTs
   `/api/v1/helper/enrollments/{id}/claim` with body
   `{"enrollment_secret":..., "helper_device_id":...}`, and persists the
   three files the daemon reads on next start:

   - `--credential-file` (default `/var/lib/borgee-helper/credential` Linux,
     `/Library/Application Support/Borgee/Helper/credential` macOS;
     mode 0600, owned by helper user)
   - `--enrollment-id-file` (default `enrollment-id` in the same dir)
   - `--device-id-file` (default `device-id` in the same dir)

3. Operator runs `sudo systemctl restart borgee-helper` (Linux) or
   `sudo launchctl kickstart -k system/cloud.borgee.host-bridge` (macOS)
   so the daemon picks up the new files immediately, or simply lets the
   next reboot pick them up.

The CLI is intentionally local-only: `enrollment_secret` is short-lived
and never leaves the operator's session. The .deb / .pkg installer does
not bundle a claim; the operator runs the CLI once after install.

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
   against the `--outbound-allowed-origins` allowlist and sets up state
   dirs), then `sandbox.Apply(...)`, then opens the UDS socket and reads
   `--enrollment-id-file`, `--helper-device-id-file`,
   `--helper-credential-file`. If all three are populated, a heartbeat
   goroutine is spawned. If any are missing or empty, the daemon logs
   `no enrollment configured, skipping heartbeat` and continues serving
   UDS — the boot-survival contract therefore does not depend on a claim
   having already happened.

### Reconnect chain — what is wired

Daemon v0(D+heartbeat+dispatch) on start:

1. Asset chain brings process up (systemd unit on Linux, launchd plist on
   macOS — see assets above).
2. `outbound.ValidateAndPrepare` validates `--outbound-server-origin`
   against the `--outbound-allowed-origins` allowlist and sets up state
   dirs.
3. `outbound.Heartbeater` is spawned in a background goroutine sharing the
   daemon's SIGTERM-aware context. It POSTs
   `/api/v1/helper/enrollments/{id}/status` immediately on startup
   (within ~100ms — no initial sleep), then every 60s
   (`outbound.HeartbeatInterval`). Heartbeat failures apply exponential
   backoff (5s base, doubling, 60s cap) and reset to base on success. The
   heartbeater never panics on network errors and never aborts the daemon
   on 401/403/410 — those just log and continue retrying, because an
   admin may have revoked the enrollment and a re-claim must still be
   possible without bouncing the process.
4. `dispatch.Dispatcher` is spawned alongside the heartbeater (#1001 +
   #1002). It long-polls
   `/api/v1/helper/enrollments/{id}/jobs/poll`; each leased job runs
   through `jobpolicy.Evaluate` (the helper-side half of the
   double-validate gate the blueprint locks in §1.2); allowed jobs are
   handed to a per-`job_type` executor when one is registered; rejected
   jobs are reported back via `/result` with the deterministic reason.
   The executor map is intentionally empty in #1001 — typed-job
   executors land in #998 + later PRs, so any leased job today is
   reported as terminal `failed`/`not_implemented` rather than silently
   dropped. While an executor runs the dispatcher Acks on a fixed
   cadence to extend the server-side lease, then tears the ack loop
   down deterministically before posting the final terminal Result.
   Poll transport failures fall back to the same 5s→60s backoff curve as
   the heartbeater. A pre-claim daemon skips dispatcher startup the
   same way it skips heartbeat (missing enrollment / device id /
   credential file collapses to "no enrollment configured, skipping job
   dispatcher").
5. The UDS Accept loop runs for local IPC.
6. The server records `LastSeenAt` on each successful heartbeat;
   `serializeWithConfigure`
   ([helper_enrollments.go](../../../packages/server-go/internal/api/helper_enrollments.go))
   flips `status` to `connected` when `LastSeenAt` is within the
   5-minute freshness window.

End-to-end reboot path now closes: machine reboots → systemd / launchd
starts the daemon as a system service with no user login → daemon
validates config + applies sandbox → reads enrollment/device-id/credential
files → heartbeater fires within ~100ms → server updates `LastSeenAt` →
web UI shows `connected` again.

## Uninstall chain (#998)

Blueprint promise: 装得上卸得掉. One server-enqueued job tears down the
local helper footprint AND flips the server-recorded enrollment status
to `uninstalled`. End-to-end:

1. Operator (owner-rail user) enqueues a `helper.uninstall` job:
   `POST /api/v1/helper/enrollments/{id}/jobs` with body
   `{ "job_type": "helper.uninstall", "schema_version": 1,
   "payload": { "scope": "helper" } }`. The enrollment must include
   `helper_lifecycle` in its `allowed_categories`.
2. Server taxonomy row (`helper_job_queries.go` `helper.uninstall`)
   accepts the request (`Enabled: true`, manifest binding declares the
   helper's own state-path / runtime-path / service-id IDs).
3. The helper-side dispatcher polls and leases the job (#1001 + #1002).
4. `jobpolicy.Evaluate` (helper-side double-validate gate) accepts the
   payload (`scope: "helper"`).
5. The `internal/executors/uninstall` executor runs the cleanup
   sequence — service disable, unit/plist removal, runtime + helper
   binary wipe, state-dir wipe (skipped when
   `payload.preserve_state == true`), OS user/group delete — and returns
   terminal `succeeded`. The executor never sends a stop signal to its
   own daemon process (would SIGTERM mid-cleanup before /result lands).
   See [`internal/executors/uninstall/README.md`](../../../packages/borgee-helper/internal/executors/uninstall/README.md)
   for the bucket-by-bucket contract and the privilege caveat.
6. The dispatcher posts `/result` with the terminal status. The server
   handler (`store.CompleteHelperJobForHelper`) sees
   `JobType=helper.uninstall && Status=succeeded` and, in the same
   transaction, flips the enrollment to
   `status='uninstalled', uninstalled_at=now`.
7. Subsequent enqueues / polls against the same enrollment return
   `uninstalled` (server already had this code path — `MarkHelperEnrollmentUninstalled`
   plus the `serializeEnrollment` precedence rule). The web UI shows
   `uninstalled` distinctly from `offline` (matches blueprint §1.2 last
   bullet).
8. The daemon process then exits naturally on its next poll iteration
   (or systemd shutdown reaps it). The server-recorded terminal status
   is the source of truth that uninstall completed.

A failed terminal (executor returns `failed`) leaves the enrollment
untouched so an operator can retry. The dedicated
`POST /api/v1/helper/enrollments/{id}/uninstall` endpoint
(helper-credential rail, predates #998) remains the manual escape hatch
for cases where the helper is offline or the executor cannot finish.

## Why config is file-based, not on the cmdline

Earlier drafts passed `enrollment_id` / `helper_device_id` as cmdline
flags. That leaked `enrollment_id` to anyone with `ps` access via
`/proc/PID/cmdline`. The current contract:

- The systemd unit and launchd plist pass only *file paths* on the
  cmdline (which are operationally safe to disclose).
- The actual values live in `StateDirectory=borgee-helper` (Linux) or
  the Helper Application Support dir (macOS), with 0644 perms for
  enrollment/device id and 0600 for the credential.
- The daemon reads each file at startup; an empty or missing file
  collapses to `(nil, false)` and skips the heartbeat without
  preventing the daemon from booting.

## End-to-end verification

[`packages/borgee-helper/e2e/claim_heartbeat_e2e_test.go`](../../../packages/borgee-helper/e2e/claim_heartbeat_e2e_test.go)
spawns the real `borgee-helper-claim` and `borgee-helper` binaries
against an `httptest.Server` that mirrors the production /claim and
/status routes. It asserts:

- Claim posts the two-field JSON body and writes credential (0600),
  enrollment-id, device-id to disk.
- Daemon reads the three files and produces a real heartbeat to
  `/api/v1/helper/enrollments/{id}/status` within ~5s of startup, with
  `Authorization: Bearer <credential>` and body
  `{"helper_device_id":..., "state":"connected"}`.
- The server-side freshness rule (replicated from `serializeWithConfigure`)
  flips status to `connected` on the recorded `LastSeenAt`.

This proves the full producer chain in CI, not just one side.

G5
([`helper_enrollments_lifecycle_test.go`](../../../packages/server-go/internal/api/helper_enrollments_lifecycle_test.go))
still locks the server-side derivation (boundary, fresh-after-stale,
revoked/uninstalled precedence). The end-to-end wire shape
(`POST .../status` with `state=connected`, Bearer credential) is
additionally locked by `TestHelperEnrollmentStatus_HeartbeatUpdatesLastSeen`
in
[`helper_enrollments_test.go`](../../../packages/server-go/internal/api/helper_enrollments_test.go).

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
   post-restart wiring (grants DB reopen, sandbox apply, heartbeater
   spawn, UDS Accept loop) is identical to the reboot path. The
   heartbeater fires within ~100ms of every restart and the server
   flips `status` back to `connected` as soon as the first POST lands.

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
| Claim CLI persists credential + ids        | `packages/borgee-helper/cmd/borgee-helper-claim/main_test.go`                      | `TestClaim_HappyPath`                           |
| Claim CLI rejects non-https origin         | `packages/borgee-helper/cmd/borgee-helper-claim/main_test.go`                      | `TestClaim_HTTPSRequired`                       |
| End-to-end claim → daemon → heartbeat      | `packages/borgee-helper/e2e/claim_heartbeat_e2e_test.go`                           | `TestClaimHeartbeatE2E` (`-tags=integration`)   |
| helper.uninstall executor cleanup buckets  | `packages/borgee-helper/internal/executors/uninstall/executor_test.go`             | `TestExecutor_*`                                |
| Server flips enrollment on uninstall success | `packages/server-go/internal/api/helper_jobs_test.go`                            | `TestHelperJobsHelperUninstallTerminalSucceededMarksEnrollmentUninstalled` |
| Server taxonomy accepts well-formed uninstall payload | `packages/server-go/internal/api/helper_jobs_test.go`                   | `TestHelperJobsEnqueueHelperUninstallAcceptsAndCarriesManifestBinding` |
| Server taxonomy rejects malformed uninstall payload | `packages/server-go/internal/api/helper_jobs_test.go`                     | `TestHelperJobsEnqueueHelperUninstallRejectsInvalidPayload` |

The byte-level asset assertions plus the installer-plan ordering plus the
server-side freshness derivation together stand in for a real reboot/crash
e2e (which a CI sandbox cannot perform).
