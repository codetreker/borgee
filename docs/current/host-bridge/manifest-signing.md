# Manifest signing — operational contract

`GET /api/v1/plugin-manifest` (HB-1) ships an ed25519-signed plugin manifest. This doc is the operator-facing contract: where the signing key lives, what gets signed, how to rotate, how `npx @codetreker/borgee-remote-agent install-plugin` verifies, what happens when config is missing.

Single source of truth for the canonical signing bytes is `packages/server-go/internal/api/manifest_signing.go::EntryCanonicalBytes`. Changing the format here without changing client (`install-plugin`, folded from install-butler in #996) means breaking verification. Do both together.

## Env vars

| name | type | required | meaning |
|---|---|---|---|
| `BORGEE_MANIFEST_SIGNING_KEY` | base64 ed25519 seed (32 bytes after decode) | prod yes; dev optional | server signs each entry + top-level payload |
| `BORGEE_MANIFEST_ENTRIES_JSON` | JSON array (inline) | optional | overrides entry list; takes precedence over file |
| `BORGEE_MANIFEST_ENTRIES_FILE` | filesystem path to JSON file | optional | overrides entry list if env JSON unset |

Generate a fresh key (one-time, store in secret manager):

```bash
go run - <<'GO'
package main
import ("crypto/ed25519"; "crypto/rand"; "encoding/base64"; "fmt")
func main() {
    pub, priv, _ := ed25519.GenerateKey(rand.Reader)
    fmt.Println("BORGEE_MANIFEST_SIGNING_KEY=" + base64.StdEncoding.EncodeToString(priv.Seed()))
    fmt.Println("# public (publish for `install-plugin`): " + base64.StdEncoding.EncodeToString(pub))
}
GO
```

## Per-entry canonical form

Each entry is signed independently. Canonical bytes:

```
ID + "|" + Version + "|" + BinaryURL + "|" + SHA256
```

Separator is single ASCII `|` (0x7C). The four fields above are concatenated byte-for-byte, no JSON encoding, no whitespace, no trailing newline. The `Platforms` field is intentionally excluded — platforms is client-side metadata not security-relevant; tampering with platforms cannot trick the client into installing a different binary because BinaryURL + SHA256 still verify.

The base64-encoded signature is stored in `Signature` on each entry. `install-plugin` verifies:

1. recompute canonical bytes from the entry it just received
2. base64-decode `Signature`
3. `ed25519.verify(pubkey, canonical_bytes, sig)`
4. if false → reject with `manifest_signature_invalid`

Per-entry signing means rotating one entry (e.g. bumping openclaw version) does not invalidate other entries' signatures. Top-level payload signature still covers the whole payload as a defense in depth.

## Entry list source (three-tier fallback)

1. `BORGEE_MANIFEST_ENTRIES_JSON` (env, full JSON array inline)
2. `BORGEE_MANIFEST_ENTRIES_FILE` (env, path to file)
3. built-in `PluginManifestEntries` default slice (dev / test only — `SHA256` is placeholder zeros)

Malformed env / unreadable file falls back to the default with a logged error. Endpoint never returns 500 due to entry config typo.

### BinaryURL pattern after the npm bundle rework

After chore/npm-bundle-rework (#993 #994 #995) the helper binary itself ships through npm rather than `.deb` / `.pkg`. After chore/collapse-npm (2026-05-20) the 4 platform binaries collapsed into the single `@codetreker/borgee-remote-agent` tarball. Concrete entry shape after first release:

```json
{
  "id": "borgee-helper",
  "version": "0.1.0",
  "binary_url": "https://registry.npmjs.org/@codetreker/borgee-remote-agent/-/borgee-remote-agent-0.1.0.tgz",
  "sha256": "<sha256 of the .tgz>",
  "signature": "<base64 ed25519>",
  "platforms": ["linux-x64", "linux-arm64", "darwin-x64", "darwin-arm64"]
}
```

The `install-plugin` client code itself is unchanged: it still does the same fetch + ed25519 verify + sha256 verify + atomic write loop. Only the operational `BORGEE_MANIFEST_ENTRIES_JSON` content shifts to point at npm registry URLs instead of GitHub Release download URLs. Per-target rows are duplicated for runtime plugins (`openclaw` etc.) that continue to be served from their own per-plugin download channels.

## Failure modes

- **Key env unset** — `LoadSigningKey` returns nil + logs `manifest_signing.key_unset` warn. Server keeps running. Per-entry `Signature=""`. `install-plugin` in production must reject empty signatures. Dev environments can ignore (warn is the operator's signal).
- **Key env malformed** — logs `hb1.signing_key_invalid` error at startup. Same effect as unset.
- **Entry env malformed** — falls back to built-in default + logs `manifest_signing.entries_*_invalid`.

## Rotation

Signing key:

1. generate new key (see above) and store both private + public
2. publish new public key for `install-plugin` (mechanism out of scope — likely versioned pubkey list shipped with helper)
3. update `BORGEE_MANIFEST_SIGNING_KEY` env in deploy
4. restart server — handler reads key at handler-construction time

Entry list (URLs, SHA256, versions):

1. update `BORGEE_MANIFEST_ENTRIES_JSON` env or `BORGEE_MANIFEST_ENTRIES_FILE` content
2. no restart needed — handler loads entries per request

## Client verification

`install-plugin` (Go, [`packages/borgee/internal/cli/installbutler/`](../../../packages/borgee/internal/cli/installbutler/README.md), #996) implements the same canonical form byte-for-byte. The Go reference is `EntryCanonicalBytes` (5 lines), mirrored in the client as `entryCanonicalBytes` with a "MUST stay byte-identical" comment. Mismatch on either side = silent verify failure.

## SHA256 real values

This PR plumbs the signing chain but leaves `SHA256` zeros in the built-in default. Real values come from the first published `borgee-v*` tag — `publish-remote-agent.yml` builds the 4 platform binaries from native runners, stages them inside the single `@codetreker/borgee-remote-agent` tarball, and the operator records the registry .tgz URL + sha256 sum in `BORGEE_MANIFEST_ENTRIES_JSON` after the publish lands.

## Helper-policy manifest (PR-4 amend, #1033)

`BORGEE_MANIFEST_SIGNING_KEY` doubles as the signing key for the **helper-policy manifest** — the second signed manifest body that scopes what helper jobs may touch on the helper host. Same private key on the server signs both; same `BORGEE_MANIFEST_SIGNING_PUBKEY` (base64 ed25519 public key) on the daemon trusts both.

Where the canonical body lives: `packages/server-go/internal/helpermanifest/manifest.go`. PR-4 final amend split it into two builders — `BuildLinux` + `BuildDarwin` — and added `CanonicalManifest(platform)` / `CanonicalDigest(platform)` entry points. Both declare the same Path / Service / Artifact ID symbols (`PathIDHelperRuntime`, `ServiceIDOpenClawUser`, …) but with platform-specific filesystem roots + service Manager / Unit. Server-side enqueue stamps each helper_jobs row with the platform's canonical-bytes digest; the leased-job payload emitted by `serializeHelperJobLease(lease, platform)` carries the signed body in a `manifest_json` field next to `manifest_binding_json`.

Daemon-side: `cli/daemon/daemon.go::loadHelperManifestTrustRoots` decodes `BORGEE_MANIFEST_SIGNING_PUBKEY` (comma-separated entries supported for rotation grace windows) and populates `jobpolicy.EvaluationInput.TrustRoots`. `jobpolicy.verifyManifestAuthority` then:

1. recompute canonical bytes from the manifest_json it just received (signature stripped)
2. base64-decode `Signature` from the body
3. `ed25519.Verify(pubkey, canonical_bytes, sig)`
4. if false → reject with `ReasonManifestInvalid`

Without a configured trust root, every manifest-required job (state.write, openclaw.*, borgee_plugin.configure_connection, service.lifecycle) falls into `ReasonManifestInvalid`. That is the safe production default; operators must inject `BORGEE_MANIFEST_SIGNING_PUBKEY` to lift it.

### Per-platform paths + services (PR-4 final amend)

| PathID                       | Linux root                          | Darwin root                                          |
|------------------------------|-------------------------------------|------------------------------------------------------|
| `openclaw_install`           | `/usr/local/lib/borgee/openclaw`    | `/usr/local/libexec/borgee/openclaw`                 |
| `openclaw_agent_config`      | `/var/lib/borgee/openclaw`          | `/Library/Application Support/Borgee/openclaw`       |
| `borgee_plugin_config`       | `/var/lib/borgee/plugins`           | `/Library/Application Support/Borgee/plugins`        |
| `borgee_state_config`        | `/var/lib/borgee/state`             | `/Library/Application Support/Borgee/state`          |
| `helper_state`               | `/var/lib/borgee`                   | `/Library/Application Support/Borgee`                |
| `helper_runtime`             | `/usr/local/lib/borgee`             | `/usr/local/libexec/borgee`                          |

| ServiceID                    | Linux (systemd unit)                | Darwin (launchd label)                               |
|------------------------------|-------------------------------------|------------------------------------------------------|
| `openclaw-user`              | `openclaw.service`                  | `cloud.borgee.openclaw`                              |
| `borgee-helper-service`      | `borgee.service`                    | `cloud.borgee.host-bridge`                           |

Darwin path roots match `packages/borgee/internal/cli/setup/setup.go` Darwin constants. `helper_state` declares the parent of `darwinStateRoot = /Library/Application Support/Borgee/Helper`, so writes under the Helper subdir are descendants of an allowed root — same containment pattern `helper_runtime` uses for `openclaw_install`.

### Platform selection

The daemon's WS upgrade carries `X-Helper-Platform: <runtime.GOOS>` (`linux` or `darwin`). Server's WS upgrade handler (`internal/ws/helper.go`) gates on the header (HTTP 400 before `websocket.Accept` if missing or unknown), stores the parsed platform on the session, and the push path threads it into `serializeHelperJobLease`. The deprecated REST poll body carries the same selector as `helper_platform`.

### Determinism

`IssuedAt` is pinned to Unix epoch and `ExpiresAt` to 2099-01-01 so the canonical digest is byte-stable across server reboots — helper_jobs rows persisted before a restart stay dischargeable after. Rotation = new manifest version (bump both `BuildLinux` + `BuildDarwin` in lockstep), not new timestamps. Linux + Darwin digests differ (different paths + service Manager), so a daemon that somehow received the wrong platform's manifest gets `ReasonManifestInvalid` silently.

### Follow-ups

- **Trust-root distribution**: env-var only in v1. Production `-ldflags` injection or signed-pubkey-list endpoint is a follow-up. Rotation flow (key v2 with grace window via the comma-CSV env var) is implemented but undocumented at the deploy layer.
- **Windows / freebsd**: not v1. The `ParsePlatform` enum + per-platform builders make adding a third token a single PR; no schema migration needed.
