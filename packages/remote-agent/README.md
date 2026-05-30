# @codetreker/borgee-remote-agent

The `@codetreker/borgee-remote-agent` package exposes one public npm CLI:
`borgee-remote-agent`. Host setup subcommands such as `install` and
`uninstall-host` are dispatched by that default CLI to the embedded Go binary
stored under `bin/platforms/<plat>-<arch>/borgee`. The older direct
remote-filesystem bridge mode still exists for compatibility, but direct
startup via `--server ... --dirs ...` is deprecated.

## Install on a host

Get a server URL + one-shot token from the Borgee web UI, then run a
single command on the target host:

```bash
npx @codetreker/borgee-remote-agent install \
  --server wss://borgee.codetrek.cn \
  --token <token-from-web-ui>
```

That's it. The daemon is installed, claimed, started, and survives
reboot via systemd (Linux) / launchd (macOS). The internal sequence —
copy binary to a persistent user path, write the user service, POST `/claim`
with the enrollment secret, prompt for sudo only to install/start rootd and
enable Linux linger, start the user daemon, wait for the first heartbeat —
happens behind one operator-visible command.

The `--token` value is `<enrollment_id>.<enrollment_secret>` (a single
opaque string the web UI concatenates for paste convenience). The CLI
splits on the FIRST `.` so a dotted secret roundtrips intact.

`wss://` and `https://` are both accepted as `--server`; the CLI
derives the matching `https://` origin for API calls automatically.
Plaintext `http://` / `ws://` are rejected unless
`--allow-insecure-server` is passed (test environments only).

## Uninstall

```bash
npx @codetreker/borgee-remote-agent uninstall-host
```

Stops + disables the user service and rootd companion, wipes state / runtime /
unit files, and prints a pointer to `npm uninstall -g` if you installed
globally. It does not delete an OS user.
For server-driven uninstall (operator triggers via web UI), the
`helper.uninstall` job runs the same cleanup buckets from inside the
daemon (`internal/executors/uninstall`).

## Advanced (re-run install, replace credential)

Subcommands available through the package default CLI:

```
npx @codetreker/borgee-remote-agent install          # one-shot operator bootstrap
npx @codetreker/borgee-remote-agent uninstall-host   # operator-driven local cleanup
npx @codetreker/borgee-remote-agent install-plugin   # signed-manifest plugin installer
npx @codetreker/borgee-remote-agent --version
```

To re-claim with a new token or refresh the systemd unit / launchd plist,
re-run the one-shot bootstrap with a fresh token from the web UI:

```bash
npx @codetreker/borgee-remote-agent install \
  --server wss://borgee.codetrek.cn \
  --token <new-token-from-web-ui>
```

`install` is idempotent: it overwrites the systemd unit / launchd plist,
preserves state dirs, and re-issues the enrollment claim with the new
token. The prior standalone `setup` / `claim` commands were
dropped from the public CLI (issue #1055) because bare `setup` produced a
non-functional install — the helpers live on as internal helpers under
`packages/borgee/internal/cli/setup/` and `packages/borgee/internal/cli/claim/`,
invoked by `install`.

## What gets installed

Linux:

| Path | Purpose |
|---|---|
| `~/.local/share/borgee/bin/borgee` | Persistent user daemon binary |
| `~/.config/systemd/user/borgee.service` | user systemd unit |
| `~/.local/state/borgee/{queue,status,audit-handoff,credential,...}` | user-owned state dirs |
| `/usr/local/lib/borgee/rootd/<uid>/borgee` | root-owned companion binary |
| `/etc/systemd/system/borgee-rootd-<uid>.service` | rootd system unit |
| `/run/borgee/<uid>/borgee-rootd.sock` | rootd UDS, owned by the installing UID |

macOS:

| Path | Purpose |
|---|---|
| `~/Library/Application Support/Borgee/bin/borgee` | Persistent user daemon binary |
| `~/Library/LaunchAgents/cloud.borgee.host-bridge.plist` | user launchd plist |
| `/Library/Application Support/Borgee/borgee-helper.sb` | sandbox-exec profile |
| `~/Library/Application Support/Borgee/Helper/...` | user-owned state dirs |
| `/usr/local/libexec/borgee/rootd/<uid>/borgee` | root-owned companion binary |
| `/Library/LaunchDaemons/cloud.borgee.host-bridge.rootd.<uid>.plist` | rootd LaunchDaemon |

If you run `install` as root, the CLI warns that the main daemon will run as
root with state under `/root`; pass `--allow-root-user` to confirm that mode.

## Deprecated: direct Node remote-agent path

The original Node WebSocket CLI remains available temporarily:

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

The default CLI resolves the current `process.platform` + `process.arch`
internally and spawns the matching embedded binary for host-bridge
subcommands. Tarball size is ~15-20 MB gzipped (same ballpark as
`typescript`); Borgee is a one-shot install per host, so the trade is to keep
the install path single-package rather than split across four
`optionalDependencies` subpackages.

Windows is intentionally out of scope (track issue #659); the CLI exits 2 with
a structured error if invoked on an unsupported `platform-arch`.

The release workflow (`.github/workflows/publish-remote-agent.yml`) builds
all 4 binaries from native runners and publishes a single
`@codetreker/borgee-remote-agent` tarball on every `borgee-v*` tag.
