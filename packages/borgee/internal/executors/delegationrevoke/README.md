# delegationrevoke executor

Dispatcher executor for `delegation.revoke`. Runs inside `borgee
daemon` (User=borgee, no root) — delegates the actual systemctl
disable + credential-file removal to `borgee rootd`'s
`delegation_revoke` whitelist entry over local UDS IPC.

## Flow

1. Parse leased job payload (`{target_category}`) — must be one of
   the allowed delegation categories (`openclaw_config /
   openclaw_lifecycle / status_collect / helper_lifecycle`).
2. Best-effort cooperatively drain the dispatcher's in-flight jobs.
   Today this is a no-op (the dispatcher has no central in-flight
   registry; the delegation.revoke job itself is the in-flight unit),
   but the API is in place so a richer drain can land without an
   executor change.
3. Call `rootdclient.DelegationRevoke` with `(enrollment_id,
   service_name, service_manager, credential_paths)`. rootd:
     - disables `borgee.service` (Linux) or
       `cloud.borgee.host-bridge` (macOS) so the init system does NOT
       respawn us after we exit.
     - removes the well-known credential trio
       (`/var/lib/borgee/credential/{credential,enrollment-id,
       device-id}` on Linux; the `Library/Application Support`
       equivalents on darwin). Missing files are idempotent successes.
4. Return `dispatch.StatusSucceeded` so the dispatcher's WS Result
   frame fires BEFORE the daemon process dies. The "no self-stop
   signal" pattern mirrors `helper.uninstall`.

## Self-shutdown safety

The executor does NOT call `os.Exit` or `signal.Kill`. The daemon
exits naturally on the next reconnect attempt: rootd has wiped the
credential, so the WS dial returns a 401 / unauthorized, the
dispatcher logs + returns, the daemon's outer loop tears down. The
disable side-effect ensures systemd does not respawn.

## delegation.revoke vs helper.uninstall

| | `delegation.revoke` | `helper.uninstall` |
|---|---|---|
| credential wipe | yes | yes |
| state-dir wipe | NO (forensic preserve) | yes (unless preserve_state) |
| binary wipe | NO | yes |
| OS user delete | NO | yes |
| service disable | yes | yes |
| can re-enroll on same machine without reinstall | yes | NO (binaries gone) |

Revoke is "stop accepting jobs + drop credential" — a fast path for a
short-lived suspension or for an operator rotating credentials.
Uninstall is the full footprint teardown.
