# @codetreker/borgee-remote-agent

The `@codetreker/borgee-remote-agent` package ships two CLIs:

1. **`borgee`** — the Borgee host-bridge daemon (Go binary, delivered inside
   this same npm tarball under `bin/platforms/<plat>-<arch>/borgee`; the
   Node shim picks the right one at runtime). One-shot operator bootstrap
   (`install`), local cleanup (`uninstall-host`), the long-lived `daemon`
   + root companion `rootd`, and the signed-manifest plugin installer
   (`install-plugin`). (The pre-#1055 standalone `setup` and `claim`
   subcommands have been folded into `install` and are no longer
   operator-facing — they remain as internal helpers invoked by
   `borgee install`.)
2. **`borgee-remote-agent`** — the Node-based remote file-system bridge
   (TypeScript CLI that connects local directories to a Borgee channel via
   WebSocket). Unchanged from the prior 0.1.x release.

## Install on a host

Get a server URL + one-shot token from the Borgee web UI, then run a
single command on the target host:

```bash
sudo npx @codetreker/borgee-remote-agent install \
  --server wss://borgee.codetrek.cn \
  --token <token-from-web-ui>
```

That's it. The daemon is installed, claimed, started, and survives
reboot via systemd (Linux) / launchd (macOS). The internal sequence —
copy binary to a persistent path, write the systemd unit / launchd
plist + system user + state dirs, POST `/claim` with the enrollment
secret, `systemctl enable --now` / `launchctl bootstrap`, wait for the
first heartbeat — happens behind one operator-visible command.

The `--token` value is `<enrollment_id>.<enrollment_secret>` (a single
opaque string the web UI concatenates for paste convenience). The CLI
splits on the FIRST `.` so a dotted secret roundtrips intact.

`wss://` and `https://` are both accepted as `--server`; the CLI
derives the matching `https://` origin for API calls automatically.
Plaintext `http://` / `ws://` are rejected unless
`--allow-insecure-server` is passed (test environments only).

## Uninstall

```bash
sudo npx @codetreker/borgee-remote-agent uninstall-host
```

Stops + disables the service, wipes state / runtime / unit-file / OS
user, prints a pointer to `npm uninstall -g` if you installed globally.
For server-driven uninstall (operator triggers via web UI), the
`helper.uninstall` job runs the same cleanup buckets from inside the
daemon (`internal/executors/uninstall`).

## Advanced (re-run install, replace credential)

Subcommands available under `borgee`:

```
borgee install          # one-shot operator bootstrap (the recommended path above)
borgee uninstall-host   # operator-driven local cleanup
borgee daemon ...       # long-lived host-bridge daemon (started by systemd / launchd)
borgee rootd ...        # root companion daemon — narrow IPC whitelist (started by systemd)
borgee install-plugin   # signed-manifest plugin binary installer (HB-1; was: borgee install)
borgee --version
```

To re-claim with a new token or refresh the systemd unit / launchd plist,
re-run the one-shot bootstrap with a fresh token from the web UI:

```bash
sudo npx @codetreker/borgee-remote-agent install \
  --server wss://borgee.codetrek.cn \
  --token <new-token-from-web-ui>
```

`install` is idempotent: it overwrites the systemd unit / launchd plist,
preserves state dirs, and re-issues the enrollment claim with the new
token. The prior standalone `borgee setup` / `borgee claim` commands were
dropped from the public CLI (issue #1055) because bare `setup` produced a
non-functional install — the helpers live on as internal helpers under
`packages/borgee/internal/cli/setup/` and `packages/borgee/internal/cli/claim/`,
invoked by `borgee install`.

## What gets installed

Linux:

| Path | Purpose |
|---|---|
| `/usr/local/lib/borgee/bin/borgee` | Persistent helper binary (`install` copies it from npx cache) |
| `/etc/systemd/system/borgee.service` | systemd unit (ExecStart points at above) |
| `/var/lib/borgee/{queue,status,audit-handoff,credential}` | Helper-owned state dirs (mode 0750) |
| `/var/log/borgee` | Audit log dir |
| `/run/borgee` | UDS socket dir |
| user `borgee`, group `borgee` | System service account (UID < 1000) |

macOS:

| Path | Purpose |
|---|---|
| `/usr/local/libexec/borgee/borgee` | Persistent helper binary |
| `/Library/LaunchDaemons/cloud.borgee.host-bridge.plist` | launchd plist |
| `/Library/Application Support/Borgee/borgee-helper.sb` | sandbox-exec profile |
| `/Library/Application Support/Borgee/Helper/...` | Helper-owned state dirs |
| user `_borgee`, group `_borgee` | System service account |

## Use (Node remote-agent path — `borgee-remote-agent`)

The original Node WebSocket CLI; bin name unchanged.

```bash
npx @codetreker/borgee-remote-agent --server wss://borgee.codetrek.cn --token <connection_token> --dirs /path/to/dir
```

### First run

Pass `--token <one-shot token from Borgee UI>`. On the first successful handshake
the agent persists the token to a state file (mode `0600`, owner-only) so the
control rail survives host reboots (#1004).

```bash
borgee-remote-agent \
  --server wss://borgee.codetrek.cn \
  --token <one-shot token> \
  --dirs /path/to/dir
```

### Subsequent runs (including after reboot)

Omit `--token`. The agent reads the persisted token from `--token-file`
(default below).

```bash
borgee-remote-agent --server wss://borgee.codetrek.cn --dirs /path/to/dir
```

### Token file location

`--token-file <path>` overrides the default. Defaults:

| OS | Path |
|---|---|
| Linux (root) | `/var/lib/borgee-remote-agent/token` |
| Linux (non-root) | `$XDG_STATE_HOME/borgee-remote-agent/token` (or `~/.local/state/borgee-remote-agent/token`) |
| macOS | `/Library/Application Support/Borgee/RemoteAgent/token` |

File mode is `0600` (owner read/write only); parent directory is created with
mode `0700` if missing.

### Revoked token

If the server rejects the persisted token (close code 4001 / 4003 / 1008, or
a close reason mentioning "unauthorized" / "invalid token" / "token revoked"),
the agent logs the rejection and exits with status `2` instead of looping
forever on a bad credential. Re-run once with `--token <new token>` to
re-enroll.

The agent connects via WebSocket and exposes local directories for browsing
and file access within Borgee channels.

## Platform binaries

This package ships ONE tarball carrying all 4 platform Go binaries:

- `bin/platforms/linux-x64/borgee`
- `bin/platforms/linux-arm64/borgee`
- `bin/platforms/darwin-x64/borgee`
- `bin/platforms/darwin-arm64/borgee`

`bin/borgee.js` is a tiny Node shim that picks the right binary for the
current `process.platform` + `process.arch` and `spawn`s it with all argv
passed through. Tarball size is ~15-20 MB gzipped (same ballpark as
`typescript`); Borgee is a one-shot install per host, so the trade is to
keep the install path single-package rather than split across four
`optionalDependencies` subpackages.

Windows is intentionally out of scope (track issue #659); the shim exits 2
with a structured error if invoked on an unsupported `platform-arch`.

The release workflow (`.github/workflows/publish-remote-agent.yml`) builds
all 4 binaries from native runners and publishes a single
`@codetreker/borgee-remote-agent` tarball on every `borgee-v*` tag.
