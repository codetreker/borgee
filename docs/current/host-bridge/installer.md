# Installer

The Host Bridge installer is the deployment path for the helper daemon. It is intentionally separate from the helper enforcement path: the installer verifies the manifest gate and installs a local artifact path, while the helper later enforces grants and IPC decisions. Current artifact integrity is not yet bound end to end to the manifest entry.

## Overview

**Role**
The installer turns a locally supplied helper package into a platform service after a manifest-signature gate. It fetches release metadata, verifies the manifest signature, asks the local operator to confirm host capabilities, and invokes platform package/service commands against the artifact path passed on the command line.

**Boundary**
The installer boundary is current manifest authenticity plus local operator consent. It does not yet prove that the local artifact path corresponds to a specific manifest entry, and it does not decide whether a future agent request is authorized; that remains a helper/grant decision after installation.

**Collaborators**
The installer collaborates with the manifest endpoint, an ed25519 public key, local package artifacts, the operator prompt, and platform service managers. It does not collaborate with Remote Agent and does not create admin routes.

**Internal Architecture**

- Platform command: Linux and macOS have separate entrypoints and artifact flags.
- Manifest client: fetches a bounded JSON envelope and verifies a detached signature as a gate before deployment.
- Consent prompt: presents the host capability class before installation proceeds.
- Deployment plan: returns inspectable platform command steps and executes them outside dry-run mode.

**Key Flows**

```text
operator runs installer with local artifact path -> fetch manifest -> verify manifest signature
-> confirm host capability prompt -> build platform deploy plan for that local artifact
-> dry-run prints steps OR sudo commands install and start helper service
```

**Invariants**

- Manifest verification is mandatory before deployment proceeds, but current local artifact integrity is not tied to the verified manifest entry.
- The installer uses local platform package/service tools rather than a server-side admin deploy endpoint.
- The installer installs the helper; it does not grant future host requests by itself.
- Platform deployment is explicit and inspectable through dry-run.

## Current Trust Boundary

The installer expects a signed envelope and verifies the signature over a canonical payload. This is a manifest authenticity gate: a network fetch alone is not enough to proceed. Release trust is still partial because the local `.deb` or `.pkg` path supplied to the installer is not currently verified against the manifest entry before package-manager execution.

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
