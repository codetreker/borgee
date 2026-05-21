# Helper Lifecycle — reboot, crash, reconnect without user login

Scope: this doc explains *why* the host-bridge helper daemon survives both
a clean reboot and a process crash without anyone logging into the host.
The systemd unit (Linux) and launchd plist (macOS) are written by
`borgee setup` from the templates inside
[`packages/borgee/internal/cli/setup/setup.go`](../../../packages/borgee/internal/cli/setup/setup.go);
the steady-state daemon contract is in
[`helper-daemon.md`](./helper-daemon.md).

## Why this matters

#968 says "the machine must remain controllable from web across reboot AND
crash, with no local user re-login". Parent #659 only covered the boot-time
autostart half. This doc walks the full chain so reviewers can verify each
mechanism without spinning up a real host.

## Distribution + setup (one-time, post-install)

The host-bridge stack ships as a single `borgee` Go binary distributed
through the `@codetreker/borgee-remote-agent` npm package (chore/npm-bundle-rework,
#993 #994 #995; one-shot install wrapper in chore/install-onecmd).
Operator one-line path on a fresh host:

```
sudo npx @codetreker/borgee-remote-agent install \
  --server wss://borgee.codetrek.cn \
  --token <enrollment_id>.<enrollment_secret>
```

That single command does: sudo + platform pre-flight → copy the running
`borgee` binary (from npx's cache) to `/usr/local/lib/borgee/bin/borgee`
(Linux) or `/usr/local/libexec/borgee/borgee` (macOS) → `borgee setup`
(systemd unit / launchd plist + system user + state dirs) → `borgee
claim` (POST `/api/v1/helper/enrollments/{id}/claim` with the parsed
enrollment_secret + a stable helper_device_id) → `systemctl enable
--now borgee.service` (Linux) / `launchctl bootstrap system <plist>`
(macOS) → wait up to `--heartbeat-timeout` (default 30s) for first
heartbeat.

`setup` / `claim` remain available as standalone subcommands (advanced
flows — e.g. re-claim with a new token after rotation, or rewrite the
systemd unit after a config change). `install` is the convenience
wrapper that ties them together for the one-line operator path.

The `borgee` binary's subcommands:

- `borgee install` — one-shot operator bootstrap (the wrapper above).
- `borgee uninstall-host` — operator-driven local cleanup mirror.
- `borgee daemon` — long-lived host-bridge daemon (started by systemd / launchd).
- `borgee rootd` — long-lived root-privileged companion daemon (started by
  systemd / launchd as a separate unit, `User=root`). Listens on a local
  UDS, accepts only a hardcoded command whitelist, executed via IPC by
  `borgee daemon`. Defense-in-depth: the WS-facing main daemon does not
  hold root; rootd's command set is narrow + audited. See
  [`docs/blueprint/current/host-bridge.md`](../../blueprint/current/host-bridge.md)
  §1.1 (two-process privilege separation) and
  [`helper-daemon.md`](helper-daemon.md) (Privilege Separation section)
  for the rationale + wire protocol.
- `borgee setup` — writes the systemd unit / launchd plist + sandbox profile,
  creates the system user (`borgee` Linux, `_borgee` macOS), creates the
  Helper-owned state directories (`/var/lib/borgee/{queue,status,audit-handoff,credential}`
  Linux, `/Library/Application Support/Borgee/Helper/...` macOS), and
  pre-creates the persistent binary dir (`/usr/local/lib/borgee/bin/`).
  Does NOT auto-start; `install` issues the start after claim.
- `borgee claim` — one-time enrollment claim. Derives a stable
  `helper_device_id` (Linux `/etc/machine-id`, macOS `IOPlatformUUID`,
  falling back to a persisted UUIDv4), POSTs
  `/api/v1/helper/enrollments/{id}/claim` with body
  `{"enrollment_secret":..., "helper_device_id":...}`, and persists the
  three files the daemon reads on next start (`credential` mode 0600,
  `enrollment-id`, `device-id`) under `/var/lib/borgee/credential/`
  (Linux) or `/Library/Application Support/Borgee/Helper/credential/`
  (macOS). Defaults updated in chore/install-onecmd (#1017 bug 1 fix)
  to match the daemon's expected directory layout.
- `borgee install-plugin` — signed-manifest binary installer (HB-1).
  One-shot CLI that fetches a manifest, ed25519-verifies an entry,
  fetches the referenced binary, sha256-verifies the bytes, atomically
  renames into place, and exits. Used to deliver runtime plugins (e.g.
  openclaw) separately from the helper itself; the helper itself ships
  as the npm bundle above. Renamed from `borgee install` in
  chore/install-onecmd. Source:
  [`packages/borgee/internal/cli/installbutler/`](../../../packages/borgee/internal/cli/installbutler/README.md).

After `borgee claim` the daemon either picks up the new files on next
start (Linux `sudo systemctl restart borgee` / macOS
`sudo launchctl kickstart -k system/cloud.borgee.host-bridge`) or at next
reboot. `enrollment_secret` is short-lived (15-minute TTL — see
[`helper_enrollment_queries.go`](../../../packages/server-go/internal/store/helper_enrollment_queries.go))
and never leaves the operator's session.

## Linux reboot chain

1. PID 1 systemd reaches `multi-user.target` (the standard non-graphical
   boot target — graphical `default.target` is *not* a prerequisite).
2. `borgee.service` declares `WantedBy=multi-user.target` and was
   linked into that target's `.wants` set when the installer ran
   `systemctl enable borgee.service`. systemd therefore starts the
   unit as part of the target.
3. Ordering directives `After=network-online.target` +
   `Wants=network-online.target` keep the start from racing the network
   layer, so the helper does not crash-loop just because routes are not
   ready yet.
4. The unit runs `Type=simple` — systemd considers the service started
   as soon as `ExecStart=` exec's, with no daemonisation handshake.
5. The daemon then opens `/var/lib/borgee/server.db` (read-only, via
   `ReadOnlyPaths=/var/lib/borgee`) for `host_grants`. That DB is owned
   by system root/`borgee`; no user session is involved.
6. `cmd/borgee/main.go::run` continues with:
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

Daemon v0(D+WS+dispatch) on start (PR-2 #1038 swapped HTTP long-poll +
POST `/status` heartbeat for a persistent WebSocket transport):

1. Asset chain brings process up (systemd unit on Linux, launchd plist on
   macOS — see assets above).
2. `outbound.ValidateAndPrepare` validates `--outbound-server-origin`
   against the `--outbound-allowed-origins` allowlist and sets up state
   dirs. The validator now accepts `wss://` (production) and `https://`
   (the WS client rewrites `https://` → `wss://` transparently for the
   actual dial); `ws://` / `http://` loopback stays gated behind
   `--allow-loopback-outbound` for e2e tests.
3. `dispatch.Dispatcher` constructs an `outbound.Client` and calls
   `RunWithReconnect`. The client dials
   `wss://<server>/ws/helper/<enrollmentId>` with `Authorization: Bearer
   <credential>` and `X-Helper-Device-Id: <id>` headers; the server's
   `HandleHelper` upgrade path calls
   `HelperEnrollmentRepository.UpdateLastSeen` (same call the legacy
   POST `/status` used) to validate the credential digest + device id
   and bump `last_seen_at` in one DB write.
4. Once connected, the dispatcher blocks on `client.Receive` for the
   next pushed `{"type":"job",...}` frame. Each leased job runs through
   `jobpolicy.Evaluate` (helper-side half of the double-validate gate)
   and then a per-`job_type` executor when one is registered. Ack and
   Result are now `{"type":"ack",...}` / `{"type":"result",...}` text
   frames over the same WS connection (no separate HTTP calls). The
   per-job ack ticker still runs to extend the server-side lease while
   an executor is in progress, and tears down deterministically before
   the final Result so the server never sees an Ack after the terminal
   state.
5. Heartbeat is now WS ping/pong. The `outbound.Client.pingLoop`
   goroutine sends a `Ping` frame every 30s; the server's pong handler
   updates `last_seen_at`. Three consecutive ping failures
   (`MissedPingsToReconnect`) tear the connection down and the
   `RunWithReconnect` outer loop re-dials with exponential backoff (1s
   base, 30s cap, ±20% jitter). Server's 5-minute freshness window
   stays the same — the producer is just the pong now, not a POST
   `/status`.
6. Server-pushed directive frames (`{"type":"directive","code":"revoked"|
   "stale_credential"|"uninstalled"|"unauthorized"}`) tell the daemon to
   stop processing and exit. `systemd Restart=on-failure` brings the
   process back; the credential is still bad so the next dial 401s and
   StartLimit eventually parks the unit — same end state as the prior
   REST 403 + dispatcher backoff path.
7. The UDS Accept loop runs for local IPC, independent of the WS
   transport state. A pre-claim daemon (missing enrollment / device-id
   / credential file) skips WS startup entirely and only serves UDS,
   the same way it used to skip heartbeat + dispatch.

End-to-end reboot path now closes: machine reboots → systemd / launchd
starts the daemon as a system service with no user login → daemon
validates config + applies sandbox → reads enrollment/device-id/credential
files → outbound.Client dials the persistent WS → server's
`UpdateLastSeen` bumps `last_seen_at` on upgrade → web UI shows
`connected` again. Sub-second push latency replaces the prior ~25s
long-poll budget for queued jobs.

The REST endpoints (`POST /api/v1/helper/enrollments/{id}/jobs/poll`,
`/jobs/{job_id}/ack`, `/jobs/{job_id}/result`, `/status`) remain
mounted for backward compatibility; new daemons use the WS path. They
are marked Deprecated and will be removed in a future PR.

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
   See [`internal/executors/uninstall/README.md`](../../../packages/borgee/internal/executors/uninstall/README.md)
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

## Revoke flow (PR-4 #1033)

`delegation.revoke` is the lightweight cousin of `helper.uninstall`.
The operator wants to stop the helper from accepting new jobs and drop
the credential, but does NOT want to remove binaries / state dirs / the
OS user — for example to rotate credentials, suspend the host without
re-installing, or quarantine a misbehaving enrollment.

End-to-end:

1. Operator (owner-rail user) enqueues a `delegation.revoke` job:
   `POST /api/v1/helper/enrollments/{id}/jobs` with body
   `{ "job_type": "delegation.revoke", "schema_version": 1,
   "payload": { "target_category": "<category-to-revoke>" } }`. The
   enrollment must include `helper_lifecycle` in its
   `allowed_categories`. The field is `target_category` (not
   `category`) because `category` is in the server-authority
   forbidden-payload set.
2. Server taxonomy row (`helper_job_queries.go` `delegation.revoke`)
   accepts the request (`Enabled: true`, no manifest binding — revoke
   removes authority rather than uses it, so it's not in
   `jobpolicy.requiresManifest`).
3. The helper-side dispatcher polls + leases the job.
4. `jobpolicy.Evaluate` accepts the payload (non-empty
   `target_category`).
5. The `internal/executors/delegationrevoke` executor runs:
   - cooperatively drains the dispatcher (no-op today; richer drain
     wires in a follow-up),
   - calls `rootdclient.DelegationRevoke` with `(enrollment_id,
     service_name, service_manager, credential_paths)`. rootd:
       - disables `borgee.service` (Linux) or
         `cloud.borgee.host-bridge` (macOS),
       - removes the credential trio at the well-known daemon paths
         (`/var/lib/borgee/credential/{credential,enrollment-id,
         device-id}` on Linux).
   - returns `dispatch.StatusSucceeded` so the dispatcher's WS Result
     frame fires BEFORE the daemon process dies.
6. The dispatcher posts `/result` with the terminal status. The daemon
   process then exits naturally on its next reconnect attempt: rootd
   has wiped the credential, so the WS dial returns 401, the
   dispatcher logs + returns, the daemon's outer loop tears down. The
   disable side-effect ensures systemd does not respawn.

A re-enrollment on the same machine can fast-path: binaries and state
dirs are still in place, so the operator only needs to run `borgee
claim` (or `npx ... install` with a fresh token) — no `borgee setup`
re-run, no system user re-creation.

| | `delegation.revoke` | `helper.uninstall` |
|---|---|---|
| credential wipe | yes | yes |
| state-dir wipe | NO (forensic preserve) | yes (unless `preserve_state`) |
| binary wipe | NO | yes |
| OS user delete | NO | yes |
| service disable | yes | yes |
| can re-enroll on same machine without reinstall | yes | NO |

## Update detection chain (#999)

A third daemon goroutine — `updatecheck.Checker` — runs alongside the
heartbeater + dispatcher. Every ~15 minutes it reads
`/var/lib/borgee/installed-versions.json` (written by
`borgee install-plugin`) and POSTs the snapshot to
`POST /api/v1/helper/enrollments/{id}/installed-versions`. The server
computes drift against the current signed manifest and returns a
classified list (`security` vs `feature` per blueprint §1.3). The helper
logs each drift entry tagged by class. Application is NOT triggered
automatically — auto-update is an explicit anti-pattern. Full details +
the (deferred) apply executor design live in
[`update-flow.md`](update-flow.md).

## Why config is file-based, not on the cmdline

Earlier drafts passed `enrollment_id` / `helper_device_id` as cmdline
flags. That leaked `enrollment_id` to anyone with `ps` access via
`/proc/PID/cmdline`. The current contract:

- The systemd unit and launchd plist pass only *file paths* on the
  cmdline (which are operationally safe to disclose).
- The actual values live in `StateDirectory=borgee` (Linux) or
  the Helper Application Support dir (macOS), with 0644 perms for
  enrollment/device id and 0600 for the credential.
- The daemon reads each file at startup; an empty or missing file
  collapses to `(nil, false)` and skips the heartbeat without
  preventing the daemon from booting.

## End-to-end verification

[`packages/borgee/e2e/claim_heartbeat_e2e_test.go`](../../../packages/borgee/e2e/claim_heartbeat_e2e_test.go)
spawns the real `borgee claim` and `borgee` binaries
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
- Run user: `UserName=_borgee` (system-only `_` prefix), again
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
  (`/var/lib/borgee/{queue,status,audit-handoff}` Linux,
  `/Library/Application Support/Borgee/Helper/{QueueState,StatusState,AuditHandoff}`
  macOS) — Helper-owned, append/replay safe.

Does *not* survive (intentionally ephemeral):

- Outbound WebSocket to the Borgee server — re-established on every start.
- In-memory caches and lease tokens — re-issued by the server on reconnect.

## Why no user login is required

- Linux: `User=borgee` / `Group=borgee` is a *system* user
  (created by the `.deb` postinst, UID < 1000). `loginctl enable-linger`
  is **not** needed because systemd PID 1 starts system units directly
  via `multi-user.target`; `enable-linger` is only relevant to user-mode
  systemd instances under `--user`.
- macOS: `_borgee` is a system role account (`_` prefix); launchd
  starts the LaunchDaemon in the system domain at boot. No Aqua / Finder
  session, no console login, no SSH session is required.

## Windows

Out of scope for v1. The npm tarball ships platform binaries only for
linux-x64, linux-arm64, darwin-x64, and darwin-arm64; an `npm i -g
@codetreker/borgee-remote-agent` on Windows leaves the `borgee` Go binary
unresolved (the shim exits 2 with a structured error pointing at issue #659).
The user outcome ("remains controllable across reboot/crash") therefore
does not apply on Windows in v1 — there is no install path to break.

## Test coverage map

| Mechanism                                  | Test file                                                                          | Test name                                       |
|--------------------------------------------|------------------------------------------------------------------------------------|-------------------------------------------------|
| systemd unit boot/crash directives present | `packages/borgee/internal/cli/setup/setup_test.go`                                 | `TestRenderLinuxUnit_Shape`                     |
| launchd plist boot/crash directives present| `packages/borgee/internal/cli/setup/setup_test.go`                                 | `TestRenderDarwinPlist_Shape`                   |
| Server flips `connected`/`offline` by freshness | `packages/server-go/internal/api/helper_enrollments_lifecycle_test.go`        | `TestHelperEnrollmentStatus_*`                  |
| Claim CLI persists credential + ids        | `packages/borgee/internal/cli/claim/claim_test.go`                                 | `TestClaim_HappyPath`                           |
| Claim CLI rejects non-https origin         | `packages/borgee/internal/cli/claim/claim_test.go`                                 | `TestClaim_HTTPSRequired`                       |
| End-to-end claim → daemon → heartbeat      | `packages/borgee/e2e/claim_heartbeat_e2e_test.go`                                  | `TestClaimHeartbeatE2E` (`-tags=integration`)   |
| helper.uninstall executor cleanup buckets  | `packages/borgee/internal/executors/uninstall/executor_test.go`                    | `TestExecutor_*`                                |
| Server flips enrollment on uninstall success | `packages/server-go/internal/api/helper_jobs_test.go`                            | `TestHelperJobsHelperUninstallTerminalSucceededMarksEnrollmentUninstalled` |
| Server taxonomy accepts well-formed uninstall payload | `packages/server-go/internal/api/helper_jobs_test.go`                   | `TestHelperJobsEnqueueHelperUninstallAcceptsAndCarriesManifestBinding` |
| Server taxonomy rejects malformed uninstall payload | `packages/server-go/internal/api/helper_jobs_test.go`                     | `TestHelperJobsEnqueueHelperUninstallRejectsInvalidPayload` |
| npm shim platform → binary path mapping     | `packages/remote-agent/src/__tests__/borgeeShim.test.ts`                           | `borgee shim platform matrix`                   |

The rendered systemd / launchd assertion plus the server-side freshness
derivation together stand in for a real reboot/crash e2e (which a CI
sandbox cannot perform).
