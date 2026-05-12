# Installer

The Host Bridge installer is the deployment path for the helper daemon. It is intentionally separate from the helper enforcement path: the installer verifies and installs artifacts, while the helper later enforces grants and IPC decisions.

## Overview

**Role**
The installer turns a signed helper release into a platform service. It fetches release metadata, verifies the manifest signature, asks the local operator to confirm host capabilities, and invokes platform package/service commands.

**Boundary**
The installer boundary is release trust and local operator consent. It does not decide whether a future agent request is authorized; that remains a helper/grant decision after installation.

**Collaborators**
The installer collaborates with the manifest endpoint, an ed25519 public key, local package artifacts, the operator prompt, and platform service managers. It does not collaborate with Remote Agent and does not create admin routes.

**Internal Architecture**

- Platform command: Linux and macOS have separate entrypoints and artifact flags.
- Manifest client: fetches a bounded JSON envelope and verifies a detached signature.
- Consent prompt: presents the host capability class before installation proceeds.
- Deployment plan: returns inspectable platform command steps and executes them outside dry-run mode.

**Key Flows**

```text
operator runs installer -> fetch manifest -> verify signature
-> confirm host capability prompt -> build platform deploy plan
-> dry-run prints steps OR sudo commands install and start helper service
```

**Invariants**

- Manifest verification is mandatory before deployment proceeds.
- The installer uses local platform package/service tools rather than a server-side admin deploy endpoint.
- The installer installs the helper; it does not grant future host requests by itself.
- Platform deployment is explicit and inspectable through dry-run.

## Manifest Trust Model

The installer expects a signed envelope and verifies the signature over a canonical payload. This makes the manifest a release integrity boundary: a network fetch alone is not enough to install. Bearer authentication may protect the fetch, but signature verification is the release trust check.

## Deployment Model

Linux deployment is modeled as package installation plus systemd unit enable/start. macOS deployment is modeled as package installation plus launchd load. The installer surfaces these as ordered steps so tests and dry-run can inspect the plan without invoking privileged commands.

## Out Of Scope

The installer does not enforce runtime grants, mediate helper IPC, install Remote Agent, or expose admin management APIs.

## Known Gaps

- The installer manifest envelope expected by the client and the current server manifest shape are not aligned.
- The installer prompt's capability vocabulary and the server host-grant vocabulary are not aligned.
- Production signing-key injection for the server manifest path is not clearly represented in the current wiring.

## Implementation Anchors

- `packages/borgee-installer/cmd/borgee-installer-linux/main.go`
- `packages/borgee-installer/cmd/borgee-installer-darwin/main.go`
- `packages/borgee-installer/internal/manifest` (`Envelope`, `Fetch`, `Verify`)
- `packages/borgee-installer/internal/deploy` (`Plan`, `LinuxPlan`, `DarwinPlan`)
- `packages/borgee-installer/internal/dialog` (`Confirm`, `GrantTypes`)
- `packages/server-go/internal/api/host_manifest.go` (`PluginManifestHandler`)
