# Installer (now `borgee setup` + npm bundle)

The Host Bridge installer path is the deployment route for the helper daemon. After chore/npm-bundle-rework (#993 #994 #995) the installer collapsed from a separate Go binary tree (the prior `packages/borgee-installer/` and `.deb` / `.pkg` artifact chain) into the `borgee setup` subcommand of the single `borgee` Go binary, which itself is delivered through the `@codetreker/borgee-remote-agent` npm package.

## Overview

**Role**
The installer turns a fresh host into a running helper service. The operator runs `sudo npm i -g @codetreker/borgee-remote-agent` (which carries all 4 platform `borgee` binaries inside its tarball under `bin/platforms/<plat>-<arch>/borgee`; the Node shim picks the right one at runtime), then `sudo borgee setup` to write the systemd unit (Linux) or launchd plist (macOS), create the system user, and create the helper-owned state directories. `borgee setup` does NOT auto-start the service; the operator must run `sudo borgee claim ...` first so the daemon has a credential, then `sudo systemctl enable --now borgee.service` (or `sudo launchctl load -w /Library/LaunchDaemons/cloud.borgee.host-bridge.plist`).

**Boundary**
`borgee setup` writes platform service assets and creates the system user; it does not decide whether a future agent request is authorized (a helper/grant decision after installation), it does not fetch the helper binary (the npm package machinery already did that), and it does not embed an enrollment secret (delegated to `borgee claim`).

**Collaborators**
`borgee setup` collaborates with `useradd` / `dscl` (system user creation), the platform service manager (`systemctl daemon-reload` / `launchctl`), and the file system (state dirs + unit / plist files). It does not collaborate with Remote Agent and does not create admin routes.

**Internal Architecture**

- Subcommand dispatcher (`packages/borgee/cmd/borgee/main.go`) routes `borgee setup` to `internal/cli/setup/`.
- The setup package embeds the systemd unit and launchd plist templates (`renderLinuxUnit` / `renderDarwinPlist`); regression test `setup_test.go` locks the rendered content against silent drift.
- `--dry-run` prints what `borgee setup` would do without touching the system.

**Key Flows**

```text
operator opens the Borgee web UI Helper panel -> clicks "Add host"
  -> fills host label + picks allowed categories -> clicks Create
  -> the modal reveals a single `sudo npx @codetreker/borgee-remote-agent install
     --server <wss://host> --token <enrollment_id>.<secret>` command (shown ONCE)
operator pastes that command on the host VM
  -> `borgee install` runs setup → claim → systemctl enable --now in one shot
operator runs `sudo npm i -g @codetreker/borgee-remote-agent`
  -> tarball includes `bin/platforms/<plat>-<arch>/borgee` for all 4 platforms
  -> Node shim `bin/borgee.js` resolves the current platform's binary inside the tarball
operator runs `sudo borgee setup`
  -> create system user (idempotent skip if present)
  -> create /var/lib/borgee/{queue,status,audit-handoff,credential} (+ /var/log/borgee, /run/borgee)
  -> write /etc/systemd/system/borgee.service (or /Library/LaunchDaemons/...plist)
  -> systemctl daemon-reload (Linux) — DO NOT auto-start; wait for claim
operator runs `sudo borgee claim --enrollment-id=X --enrollment-secret=Y --server-origin=Z`
  -> POST /api/v1/helper/enrollments/{id}/claim
  -> persist credential (0600) + enrollment-id + device-id
operator runs `sudo systemctl enable --now borgee.service`
```

The web UI's "Add host" button (`HelperStatusPanel.tsx` → `POST /api/v1/helper/enrollments`) is the standard operator entry point — it eliminates the curl-era footgun of hand-building the `<enrollment_id>.<enrollment_secret>` token. The token + install command are revealed exactly once; the server only persists the secret's digest, so a lost token requires revoking the enrollment and minting a new one.

**Invariants**

- npm bundle delivery means the operator NEVER hand-copies a `.deb` / `.pkg`; the tarball carries all 4 platform binaries at `bin/platforms/<plat>-<arch>/borgee` and the Node shim picks one at runtime.
- `borgee setup` is idempotent on re-runs (state dirs preserved, user creation skipped if present, unit file overwritten).
- `borgee setup` never reads or writes enrollment credentials — that path lives in `borgee claim`.

## Current Trust Boundary

`borgee install-plugin` (folded from install-butler in #996) is the signed-manifest path for *runtime plugin* binaries (openclaw etc.), still backed by the server-side ed25519 signing chain documented in [`manifest-signing.md`](./manifest-signing.md). The helper binary itself is delivered through npm (registry trust + the main package's own provenance), which is a separate trust boundary from the manifest-signing path.

## Out Of Scope

`borgee setup` does not enforce runtime grants, mediate helper IPC, install Remote Agent, expose admin management APIs, or auto-claim enrollments.

## Implementation Anchors

- `packages/borgee/cmd/borgee/main.go` — subcommand dispatcher (single binary entry).
- `packages/borgee/internal/cli/setup/setup.go` — `borgee setup` (renders systemd unit + launchd plist + creates user + state dirs).
- `packages/borgee/internal/cli/installbutler/installbutler.go` — `borgee install-plugin` (signed-manifest installer).
- `packages/borgee/internal/cli/claim/claim.go` — `borgee claim` (enrollment claim).
- `packages/client/src/components/HelperStatusPanel.tsx` — operator UI "Add host" button + create-form modal + token-reveal view (single-display).
- `packages/server-go/internal/api/helper_enrollments.go::handleCreate` — server endpoint that returns `enrollment_token` + `install_command` (one-line `sudo npx ...`) the modal hands the operator.
- `packages/remote-agent/bin/borgee.js` — Node shim resolving the platform binary embedded in the same tarball.
- `packages/remote-agent/bin/platforms/{linux-x64,linux-arm64,darwin-x64,darwin-arm64}/borgee` — 4 platform binaries (populated at publish time by the release workflow; not checked into git).
- `.github/workflows/publish-remote-agent.yml` — release pipeline (tag `borgee-v*` → matrix build 4 platforms → stage into `bin/platforms/` → single `npm publish`).
- `packages/server-go/internal/api/host_manifest.go` (`PluginManifestHandler`) — server side of the signed manifest endpoint.
