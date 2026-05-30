# helper.uninstall executor (#998)

One-key uninstall: a server-enqueued `helper.uninstall` job tears down the
helper's local footprint and posts terminal `succeeded`, which the server
treats as the signal to flip enrollment status to `uninstalled`.

## What gets removed

1. **Service unit / plist disabled** — `systemctl --user disable
   borgee.service` plus the sudo-managed rootd unit (Linux), or launchd peers
   (macOS). Intentionally **no** `stop` — see self-uninstall safety below.
2. **Service unit / plist file** — user service file plus the rootd system
   service/plist file.
3. **Runtime binaries** — user-owned Borgee runtime tree.
4. **Helper binaries** — only when a custom layout explicitly lists extra
   binary paths.
5. **State directories** — user-owned Borgee state dirs. Skipped when
   `preserve_state: true`.
6. **OS user/group** — skipped for current user-owned installs. Legacy/custom
   layouts that explicitly provide a user/group still exercise the old
   best-effort deletion bucket.

Each bucket records `removed` / `absent` / `failed` / `disabled` / `skipped`
into the daemon log. The dispatcher posts a short audit ref to the server
shaped `helper-uninstall-<platform>-buckets-<N>-ok-<X>-fail-<Y>`.

## Payload contract

```json
{ "scope": "helper", "preserve_state": false }
```

`scope` MUST equal `"helper"`; future scopes (e.g. `agent`-only teardown)
extend the field without changing the wire shape. `preserve_state` is
optional; when `true` the state-dir bucket is skipped so a forensic /
post-mortem session can keep credentials + audit handoff intact.

## Self-uninstall safety

The executor runs **inside the long-lived borgee daemon process**.
The cleanup order intentionally never sends a stop signal to the daemon:

- `systemctl disable` / `launchctl disable` only flips the auto-start bit;
  the running process is untouched.
- Removing the daemon's own binary while it executes is safe on POSIX —
  the kernel keeps the live inode resident.
- The executor returns terminal `succeeded`; the dispatcher posts /result;
  the daemon then exits naturally on its next poll iteration (or the
  systemd shutdown path picks it up). Either way the **server-recorded
  terminal status is the source of truth** that uninstall completed.

If the executor instead called `systemctl stop borgee`, systemd
would SIGTERM us mid-cleanup and the dispatcher would never POST the
final Result. The executor would still succeed locally, but the server
would never learn — defeating the whole point.

## Privilege caveat

Disabling/removing the rootd system service still requires root (or
CAP_DAC_OVERRIDE / CAP_SYS_ADMIN). The production helper daemon runs as the
installing user, which has neither by default.

The executor still runs every bucket. Buckets that can't be done as the
helper user (rootd system service disable/removal, root-owned cleanup) are
recorded as `failed` and **the executor still returns succeeded** — the
per-bucket result is structured so an operator can see what was actually
done. The state-dir bucket (paths owned by the helper user) succeeds even
on a non-root helper.

(The executor does **not** invoke sudo today — that's a follow-up if the
sudoers path becomes the standard ship-form.)

## Test coverage

See `executor_test.go`:

- **TU-1 SuccessfulUninstall** — every bucket against a temp tree.
- **TU-2 PreserveState** — state dirs untouched, rest wiped.
- **TU-3 PartialFailureTolerant** — pre-deleted files register `absent`;
  recorded systemctl failure does not abort.
- **TU-4 RejectInvalidScope** — payload `scope != "helper"` returns
  terminal failed/schema_invalid without touching the filesystem.
- **TU-5 NoSelfStopSignal** — guard against future regression — recorded
  command list MUST NOT include `systemctl stop borgee`.
- **TU-6 DarwinUsesLaunchctlAndDscl** — macOS branches use `launchctl` +
  `dscl`, never `systemctl` / `userdel`.
- **TU-7 NilJob** — defensive nil-job rejection.
