# @codetreker/borgee-remote-agent

The `@codetreker/borgee-remote-agent` package ships two CLIs:

1. **`borgee`** — the Borgee host-bridge daemon (Go binary, delivered through
   per-platform `optionalDependencies` subpackages). One-shot operator
   bootstrap (`install`), local cleanup (`uninstall-host`), and the
   long-lived `daemon` + advanced `setup` / `claim` / `install-plugin`
   subcommands.
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

## Advanced (re-claim, redo setup)

Subcommands available under `borgee`:

```
borgee install          # one-shot operator bootstrap (the recommended path above)
borgee uninstall-host   # operator-driven local cleanup
borgee setup            # systemd unit / launchd plist + system user + state dirs (called by install)
borgee claim ...        # one-time enrollment claim (called by install; re-runnable to re-claim)
borgee daemon ...       # long-lived host-bridge daemon (started by systemd / launchd)
borgee install-plugin   # signed-manifest plugin binary installer (HB-1; was: borgee install)
borgee --version
```

Re-claim with a new token:

```bash
sudo borgee claim --enrollment-id=<id> --enrollment-secret=<secret> \
                  --server-origin=https://borgee.codetrek.cn
```

Redo systemd unit only (e.g. after a config bump):

```bash
sudo borgee setup --server-origin=https://borgee.codetrek.cn
sudo systemctl daemon-reload && sudo systemctl restart borgee
```

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

## Platform subpackages

The 4 `optionalDependencies` packages
(`@codetreker/borgee-remote-agent-linux-x64`, `-linux-arm64`,
`-darwin-x64`, `-darwin-arm64`) each ship ONE file at `bin/borgee` — the
platform-specific Go build of the host-bridge daemon. They have no Node
code and never run anything at install time. Installation is gated by the
standard npm `os` / `cpu` machinery, so only one is materialized into
`node_modules/` per host. The release workflow
(`.github/workflows/release-borgee.yml`) builds and publishes all four on
each `borgee-v*` tag.
