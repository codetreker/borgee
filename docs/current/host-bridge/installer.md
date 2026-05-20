# Installer (now `borgee setup` + npm bundle)

The Host Bridge installer path is the deployment route for the helper daemon. After chore/npm-bundle-rework (#993 #994 #995) the installer collapsed from a separate Go binary tree (the prior `packages/borgee-installer/` and `.deb` / `.pkg` artifact chain) into the `borgee setup` subcommand of the single `borgee` Go binary, which itself is delivered through the `@codetreker/borgee-remote-agent` npm package.

## Overview

**Role**
The installer turns a fresh host into a running helper service. The operator runs `sudo npm i -g @codetreker/borgee-remote-agent` (which carries the platform `borgee` binary as an `optionalDependencies` subpackage), then `sudo borgee setup` to write the systemd unit (Linux) or launchd plist (macOS), create the system user, and create the helper-owned state directories. `borgee setup` does NOT auto-start the service; the operator must run `sudo borgee claim ...` first so the daemon has a credential, then `sudo systemctl enable --now borgee.service` (or `sudo launchctl load -w /Library/LaunchDaemons/cloud.borgee.host-bridge.plist`).

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
operator runs `sudo npm i -g @codetreker/borgee-remote-agent`
  -> npm picks the right `@codetreker/borgee-remote-agent-<plat>-<arch>` optionalDependency
  -> Node shim `bin/borgee.js` resolves the platform subpackage's `bin/borgee`
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

**Invariants**

- npm bundle delivery means the operator NEVER hand-copies a `.deb` / `.pkg`; the platform subpackage carries exactly one file at `bin/borgee`.
- `borgee setup` is idempotent on re-runs (state dirs preserved, user creation skipped if present, unit file overwritten).
- `borgee setup` never reads or writes enrollment credentials — that path lives in `borgee claim`.

## Current Trust Boundary

`borgee install` (folded from install-butler in #996) is the signed-manifest path for *runtime plugin* binaries (openclaw etc.), still backed by the server-side ed25519 signing chain documented in [`manifest-signing.md`](./manifest-signing.md). The helper binary itself is delivered through npm (registry trust + the platform subpackage's own provenance), which is a separate trust boundary from the manifest-signing path.

## Out Of Scope

`borgee setup` does not enforce runtime grants, mediate helper IPC, install Remote Agent, expose admin management APIs, or auto-claim enrollments.

## Implementation Anchors

- `packages/borgee/cmd/borgee/main.go` — subcommand dispatcher (single binary entry).
- `packages/borgee/internal/cli/setup/setup.go` — `borgee setup` (renders systemd unit + launchd plist + creates user + state dirs).
- `packages/borgee/internal/cli/installbutler/installbutler.go` — `borgee install` (signed-manifest installer).
- `packages/borgee/internal/cli/claim/claim.go` — `borgee claim` (enrollment claim).
- `packages/remote-agent/bin/borgee.js` — Node shim resolving the platform subpackage.
- `packages/remote-agent/platforms/{linux-x64,linux-arm64,darwin-x64,darwin-arm64}/` — 4 platform subpackages (one binary each).
- `.github/workflows/release-borgee.yml` — release pipeline (tag `borgee-v*` → build 4 platforms → publish 4 platform npm subpackages → publish main package).
- `packages/server-go/internal/api/host_manifest.go` (`PluginManifestHandler`) — server side of the signed manifest endpoint.
