# Installer (`npx @codetreker/borgee-remote-agent install` + npm bundle)

The Host Bridge installer path is the deployment route for the helper daemon. After chore/npm-bundle-rework (#993 #994 #995) the installer collapsed from a separate Go binary tree (the prior `packages/borgee-installer/` and `.deb` / `.pkg` artifact chain) into the `borgee` Go binary delivered through the `@codetreker/borgee-remote-agent` npm package. After chore/install-onecmd + issue #1055 the operator-facing surface is the package default command `npx @codetreker/borgee-remote-agent install`; the prior standalone `setup` and `claim` commands were folded into `install` and dropped from the public CLI because bare `setup` produced a non-functional install.

## Overview

**Role**
The installer turns a fresh host into a running helper service. The operator runs one command without a leading sudo: `npx @codetreker/borgee-remote-agent install --server <wss://host> --token <enrollment_id>.<secret>`. The npm tarball carries all 4 platform `borgee` binaries under `bin/platforms/<plat>-<arch>/borgee`; the default CLI resolves the current platform internally and dispatches to the embedded binary. `install` is the one-shot operator bootstrap: it sudo-installs the running binary once to the install prefix (`/usr/local/borgee/bin/borgee` by default), writes the user service, creates user-owned state directories, POSTs `/claim` to mint a long-term credential, prompts for sudo only to install/start the rootd companion and enable Linux linger, starts the user daemon, and waits for the first heartbeat before returning. The main user service and rootd both execute the same shared binary.

**Boundary**
`install` orchestrates platform service install + enrollment claim + service start + heartbeat wait. It does not decide whether a future agent request is authorized (a helper/grant decision at runtime), it does not fetch the helper binary (the npm package machinery already did that), and it does not embed an enrollment secret (the operator supplies `--token` from the web UI's one-shot reveal). Internal helpers `internal/cli/setup/` and `internal/cli/claim/` render platform service assets and post the claim respectively — they are not operator-facing public commands.

**Collaborators**
`install` collaborates with the platform service manager (`systemctl --user`, `loginctl enable-linger`, sudo-managed rootd systemd / launchd), the file system (user state dirs + unit / plist files), the server enrollment API (`POST /api/v1/helper/enrollments/{id}/claim`), and the heartbeat endpoint (poll until `last_seen_at` populates). It does not create a `borgee` / `_borgee` OS user, does not collaborate with Remote Agent, and does not create admin routes.

**Internal Architecture**

- Package default CLI (`packages/remote-agent/src/cli.ts`) dispatches host-bridge subcommands to the embedded platform binary resolved by `packages/remote-agent/src/platform-binary.ts`.
- Embedded binary dispatcher (`packages/borgee/cmd/borgee/main.go`) routes `install`, `uninstall-host`, `daemon`, `rootd`, `install-plugin`. `setup` and `claim` are NOT public commands (issue #1055); their packages are linked transitively because `internal/cli/install` imports them.
- `install` (`packages/borgee/internal/cli/install/install.go`) chains:
  1. platform and install-user pre-flight,
  2. derives the https origin from the wss:// `--server` (or accepts https:// directly),
  3. splits `--token` on the first `.` into `<enrollment_id>.<enrollment_secret>`,
  4. sudo-installs the running binary to `<install-prefix>/bin/borgee` (`/usr/local/borgee/bin/borgee` by default),
  5. calls `setup.Run` (internal helper) with `--server-origin = <derived wss>` plus install user uid/gid/home,
  6. calls `claim.Run` (internal helper) with the parsed enrollment id + secret,
  7. enables Linux linger, installs the rootd system service with sudo, then starts the user daemon,
  8. polls the server until heartbeat lands or `--heartbeat-timeout` elapses.
- `--dry-run` (where supported by the internal helpers) prints what would be done without touching the system.
- The setup package embeds the systemd unit and launchd plist templates (`renderLinuxUnit` / `renderDarwinPlist`); regression test `setup_test.go` locks the rendered content against silent drift even though setup is no longer operator-facing.

**Key Flows**

```text
operator opens the Borgee web UI Helper panel -> clicks "Add host"
  -> fills host label + picks allowed categories -> clicks Create
  -> the modal reveals a single
       `npx @codetreker/borgee-remote-agent install --server <wss://host> --token <enrollment_id>.<secret>`
     command (shown ONCE)
operator runs `sudo npm i -g @codetreker/borgee-remote-agent`
  -> tarball includes `bin/platforms/<plat>-<arch>/borgee` for all 4 platforms
  -> default package CLI resolves the current platform's binary inside the tarball
operator pastes the one-line install command on the host VM
  -> installer copies that binary once to `<install-prefix>/bin/borgee`
  -> `install` runs setup → claim → start → wait-heartbeat as one shot
  -> the daemon is installed, claimed, started, survives reboot
```

The web UI's "Add host" button (`HelperStatusPanel.tsx` → `POST /api/v1/helper/enrollments`) is the standard operator entry point — it eliminates the curl-era footgun of hand-building the `<enrollment_id>.<enrollment_secret>` token. The token + install command are revealed exactly once; the server only persists the secret's digest, so a lost token requires revoking the enrollment and minting a new one.

## Owner-driven OpenClaw install (#1050)

Once the helper is connected, owners can install OpenClaw without SSHing into the host. From the same `HelperStatusPanel` page the operator selects a helper that is fresh (`status=connected`) and whose `configure_openclaw` aggregate does not yet show a succeeded `openclaw.install_from_manifest` step, then clicks **Install OpenClaw**. A confirmation modal surfaces the read-only target path (`/usr/local/lib/borgee/openclaw`) and plugin id (`openclaw`); Confirm POSTs the existing helper-jobs enqueue endpoint (`POST /api/v1/helper/enrollments/{id}/jobs`, `job_type=openclaw.install_from_manifest`, deterministic `idempotency_key=install-openclaw-<enrollment_id>`). The server constructs the canonical payload + signed manifest binding (see [`manifest-signing.md`](./manifest-signing.md)); the daemon's `install_plugin` executor writes the binary to the manifest-declared target path. The button is replaced by a live progress badge driven by the existing helper-job state subscription, and flips to "OpenClaw installed" once the install step reaches `succeeded`.

**Invariants**

- npm bundle delivery means the operator NEVER hand-copies a `.deb` / `.pkg`; the tarball carries all 4 platform binaries at `bin/platforms/<plat>-<arch>/borgee` and the default package CLI picks one at runtime.
- `install` is idempotent on re-runs (user state dirs preserved, user unit/rootd unit overwritten, claim re-issued with the new token).
- `setup` and `claim` are internal-only helpers (issue #1055). They are not exposed as public commands; operators must use `npx @codetreker/borgee-remote-agent install`. A re-run of `install` with a fresh token is the supported re-claim path.
- Root execution is supported only as an explicit operator choice: running as root prints a warning and requires `--allow-root-user`, then stores main-daemon state under `/root`.
- `--install-prefix <path>` overrides the shared root-owned binary prefix for tests, packages, or non-standard deployments; production defaults to `/usr/local/borgee`.

## `install_command` Origin Selection (#1052)

The server stamps a `scheme://host` into the printed `install_command` so the operator pastes a ready-to-run line; that origin must be reachable from the helper host. Selection priority in `handleCreate` → `buildHelperInstallCommand`:

1. **`BORGEE_PUBLIC_HELPER_ORIGIN` env (optional override).** When set, used verbatim as the `--server` value. Must start with `ws://` or `wss://`, no trailing path (validated at boot in `config.Validate`). Use this when the inbound `r.Host` reaching `server-go` is NOT the address the helper VM should dial:
   - **Docker dev-stack** (`scripts/dev-stack/.env.example` ships `BORGEE_PUBLIC_HELPER_ORIGIN=ws://borgee-server:4900`): the server is bound on the host's `127.0.0.1:4900` for the operator browser, but the helper container reaches it via the shared docker network DNS name `borgee-server:4900`.
   - **Reverse proxy / multi-host deploys** where the proxy does NOT set `X-Forwarded-Host` (or where the public hostname is intentionally different from any header the operator browser sends): pin the public WS origin explicitly (e.g. `wss://borgee.codetrek.cn`).
2. **`X-Forwarded-Proto` + `X-Forwarded-Host`** (when the env override is unset). Standard TLS-terminating reverse proxy path — nginx in front of `server-go` setting both headers.
3. **`r.TLS != nil` → `wss://r.Host`, else `ws://r.Host`.** The single-host on-prem default: the host the operator browser hit IS the host the helper must connect back to. Leaving `BORGEE_PUBLIC_HELPER_ORIGIN` unset preserves this behavior — the env knob is additive, not a breaking change.

## Current Trust Boundary

`install-plugin` (folded from install-butler in #996) is the signed-manifest path for *runtime plugin* binaries (openclaw etc.), still backed by the server-side ed25519 signing chain documented in [`manifest-signing.md`](./manifest-signing.md). The helper binary itself is delivered through npm (registry trust + the main package's own provenance), which is a separate trust boundary from the manifest-signing path.

## Out Of Scope

`install` does not enforce runtime grants, mediate helper IPC, install Remote Agent, expose admin management APIs, or chain in plugin installs.

## Implementation Anchors

- `packages/borgee/cmd/borgee/main.go` — subcommand dispatcher (single binary entry); public surface is `install`, `uninstall-host`, `daemon`, `rootd`, `install-plugin` (issue #1055 dropped `setup` / `claim` from the public dispatch).
- `packages/borgee/internal/cli/install/install.go` — `install` (one-shot bootstrap: setup → claim → start → wait-heartbeat).
- `packages/borgee/internal/cli/setup/setup.go` — internal helper called by `install` (renders systemd unit + launchd plist + user-owned state dirs).
- `packages/borgee/internal/cli/claim/claim.go` — internal helper called by `install` (enrollment claim).
- `packages/borgee/internal/cli/installbutler/installbutler.go` — `install-plugin` (signed-manifest installer).
- `packages/client/src/components/HelperStatusPanel.tsx` — operator UI "Add host" button + create-form modal + token-reveal view (single-display).
- `packages/server-go/internal/api/helper_enrollments.go::handleCreate` — server endpoint that returns `enrollment_token` + `install_command` (one-line `sudo npx ...`) the modal hands the operator.
- `packages/remote-agent/src/cli.ts` — public package CLI and host-bridge subcommand dispatcher.
- `packages/remote-agent/src/platform-binary.ts` — platform binary resolver for embedded `bin/platforms/*/borgee` assets.
- `packages/remote-agent/bin/platforms/{linux-x64,linux-arm64,darwin-x64,darwin-arm64}/borgee` — 4 platform binaries (populated at publish time by the release workflow; not checked into git).
- `.github/workflows/publish-remote-agent.yml` — release pipeline (tag `borgee-v*` → matrix build 4 platforms → stage into `bin/platforms/` → single `npm publish`).
- `packages/server-go/internal/api/host_manifest.go` (`PluginManifestHandler`) — server side of the signed manifest endpoint.
