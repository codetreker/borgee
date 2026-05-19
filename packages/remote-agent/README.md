# @codetreker/borgee-remote-agent

CLI tool for connecting remote file systems to [Borgee](https://borgee.codetrek.cn) channels.

## Install

```bash
npx @codetreker/borgee-remote-agent --server wss://borgee.codetrek.cn --token <connection_token> --dirs /path/to/dir
```

## Usage

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
