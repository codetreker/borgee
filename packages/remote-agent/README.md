# @codetreker/borgee-remote-agent

The `@codetreker/borgee-remote-agent` package ships two CLIs:

1. **`borgee`** — the Borgee host-bridge daemon (Go binary, delivered through
   per-platform `optionalDependencies` subpackages). Provides the `daemon`,
   `setup`, `claim`, and `install` subcommands used to install the helper as
   a systemd unit (Linux) or launchd plist (macOS) and to claim an
   enrollment.
2. **`borgee-remote-agent`** — the Node-based remote file-system bridge
   (TypeScript CLI that connects local directories to a Borgee channel via
   WebSocket). Unchanged from the prior 0.1.1 release.

## Install (helper daemon path — `borgee`)

The recommended install path on a fresh host (replaces the prior
`.deb` / `.pkg` distribution that briefly shipped under
`release-helper.yml`; see chore/npm-bundle-rework, #993 #994 #995):

```bash
sudo npm i -g @codetreker/borgee-remote-agent
sudo borgee setup                                       # systemd unit + state dirs + system user
sudo borgee claim --enrollment-id=<id> \
                  --enrollment-secret=<secret> \
                  --server-origin=https://app.borgee.io
sudo systemctl enable --now borgee.service              # macOS: sudo launchctl load -w /Library/LaunchDaemons/cloud.borgee.host-bridge.plist
```

npm picks one platform `optionalDependency`
(`@codetreker/borgee-remote-agent-linux-x64`, `-linux-arm64`,
`-darwin-x64`, or `-darwin-arm64`) and skips the other three; a Node shim
(`bin/borgee.js`) resolves the chosen subpackage and exec's its
`bin/borgee`. Windows is intentionally out of scope; track issue #659.

Subcommands:

```
borgee daemon ...           # long-lived host-bridge daemon (started by systemd / launchd)
borgee setup                # install systemd unit / launchd plist + system user + state dirs
borgee claim ...            # one-time enrollment claim (writes credential + enrollment-id + device-id)
borgee install ...          # signed-manifest binary installer (HB-1; for runtime plugins like openclaw)
borgee uninstall            # pointer to the helper.uninstall job (run via Borgee web UI)
borgee --version
```

`borgee setup` is idempotent and writes:

- Linux: `/etc/systemd/system/borgee.service`, system user `borgee`,
  state dirs under `/var/lib/borgee/{queue,status,audit-handoff,credential}`,
  log dir `/var/log/borgee`, run dir `/run/borgee`.
- macOS: `/Library/LaunchDaemons/cloud.borgee.host-bridge.plist`, sandbox
  profile at `/Library/Application Support/Borgee/borgee-helper.sb`, system
  user `_borgee`, state dirs under
  `/Library/Application Support/Borgee/Helper/...`.

`borgee setup` does NOT auto-start the service — the operator must run
`borgee claim` first so the daemon has a credential.

## Use (Node remote-agent path — `borgee-remote-agent`)

This is the original Node WebSocket CLI; bin name unchanged.

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
