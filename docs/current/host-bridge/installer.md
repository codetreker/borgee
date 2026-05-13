# Installer

The Host Bridge installer is the deployment path for the helper daemon. It is intentionally separate from the helper enforcement path: the server manifest endpoint and installer verifier exist, but the end-to-end installer trust boundary is still partial wiring. The installer deploys a local artifact path, while the helper later enforces grants and IPC decisions.

## Overview

**Role**
The installer turns a locally supplied helper package into a platform service after running the current manifest verifier path. It fetches release metadata, asks the local operator to confirm host capabilities, and invokes platform package/service commands against the artifact path passed on the command line.

**Boundary**
The installer boundary is partial wiring plus local operator consent. It does not yet establish an end-to-end trust boundary because the server/client envelope shape, signing-key injection, and local artifact binding are not aligned. It also does not decide whether a future agent request is authorized; that remains a helper/grant decision after installation.

**Collaborators**
The installer collaborates with the manifest endpoint, a verification key path, local package artifacts, the operator prompt, and platform service managers. It does not collaborate with Remote Agent and does not create admin routes.

**Internal Architecture**

- Platform command: Linux and macOS have separate entrypoints and artifact flags.
- Manifest client: fetches a bounded JSON envelope and runs the current verifier path before deployment.
- Consent prompt: presents the host capability class before installation proceeds.
- Deployment plan: returns inspectable platform command steps and executes them outside dry-run mode.

**Key Flows**

```text
operator runs installer with local artifact path -> fetch manifest -> run verifier path
-> confirm host capability prompt -> build platform deploy plan for that local artifact
-> dry-run prints steps OR sudo commands install and start helper service
```

**Invariants**

- The manifest endpoint and installer verifier are present, but deployment trust remains partial because artifact integrity is not tied to a manifest entry.
- The installer uses local platform package/service tools rather than a server-side admin deploy endpoint.
- The installer installs the helper; it does not grant future host requests by itself.
- Platform deployment is explicit and inspectable through dry-run.

## Current Trust Boundary

The installer has a verifier path for manifest data fetched from the server manifest endpoint. That path is not yet a dependable end-to-end trust boundary: the client and server envelope shapes are not aligned, production signing-key injection is not clearly wired, and the local `.deb` or `.pkg` path supplied to the installer is not verified against a manifest entry before package-manager execution.

## Deployment Model

Linux deployment is modeled as package installation plus systemd unit enable/start. macOS deployment is modeled as package installation plus launchd load. The installer surfaces these as ordered steps so tests and dry-run can inspect the plan without invoking privileged commands.

## Out Of Scope

The installer does not enforce runtime grants, mediate helper IPC, install Remote Agent, or expose admin management APIs.

## Known Gaps

- Client and server manifest envelope shapes are not aligned.
- Artifact-to-manifest binding remains the installer trust gap described above.
- Installer prompt vocabulary and server host-grant vocabulary are not aligned.
- Production signing-key injection for the server manifest path is not clearly represented in the current wiring.

## Implementation Anchors

- `packages/borgee-installer/cmd/borgee-installer-linux/main.go`
- `packages/borgee-installer/cmd/borgee-installer-darwin/main.go`
- `packages/borgee-installer/internal/manifest` (`Envelope`, `Fetch`, `Verify`)
- `packages/borgee-installer/internal/deploy` (`Plan`, `LinuxPlan`, `DarwinPlan`)
- `packages/borgee-installer/internal/dialog` (`Confirm`, `GrantTypes`)
- `packages/server-go/internal/api/host_manifest.go` (`PluginManifestHandler`)
