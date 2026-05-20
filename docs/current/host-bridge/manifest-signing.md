# Manifest signing — operational contract

`GET /api/v1/plugin-manifest` (HB-1) ships an ed25519-signed plugin manifest. This doc is the operator-facing contract: where the signing key lives, what gets signed, how to rotate, how `borgee install-plugin` verifies, what happens when config is missing.

Single source of truth for the canonical signing bytes is `packages/server-go/internal/api/manifest_signing.go::EntryCanonicalBytes`. Changing the format here without changing client (`borgee install-plugin`, folded from install-butler in #996) means breaking verification. Do both together.

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
    fmt.Println("# public (publish for `borgee install-plugin`): " + base64.StdEncoding.EncodeToString(pub))
}
GO
```

## Per-entry canonical form

Each entry is signed independently. Canonical bytes:

```
ID + "|" + Version + "|" + BinaryURL + "|" + SHA256
```

Separator is single ASCII `|` (0x7C). The four fields above are concatenated byte-for-byte, no JSON encoding, no whitespace, no trailing newline. The `Platforms` field is intentionally excluded — platforms is client-side metadata not security-relevant; tampering with platforms cannot trick the client into installing a different binary because BinaryURL + SHA256 still verify.

The base64-encoded signature is stored in `Signature` on each entry. `borgee install-plugin` verifies:

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

The `borgee install-plugin` client code itself is unchanged: it still does the same fetch + ed25519 verify + sha256 verify + atomic write loop. Only the operational `BORGEE_MANIFEST_ENTRIES_JSON` content shifts to point at npm registry URLs instead of GitHub Release download URLs. Per-target rows are duplicated for runtime plugins (`openclaw` etc.) that continue to be served from their own per-plugin download channels.

## Failure modes

- **Key env unset** — `LoadSigningKey` returns nil + logs `manifest_signing.key_unset` warn. Server keeps running. Per-entry `Signature=""`. `borgee install-plugin` in production must reject empty signatures. Dev environments can ignore (warn is the operator's signal).
- **Key env malformed** — logs `hb1.signing_key_invalid` error at startup. Same effect as unset.
- **Entry env malformed** — falls back to built-in default + logs `manifest_signing.entries_*_invalid`.

## Rotation

Signing key:

1. generate new key (see above) and store both private + public
2. publish new public key for `borgee install-plugin` (mechanism out of scope — likely versioned pubkey list shipped with helper)
3. update `BORGEE_MANIFEST_SIGNING_KEY` env in deploy
4. restart server — handler reads key at handler-construction time

Entry list (URLs, SHA256, versions):

1. update `BORGEE_MANIFEST_ENTRIES_JSON` env or `BORGEE_MANIFEST_ENTRIES_FILE` content
2. no restart needed — handler loads entries per request

## Client verification

`borgee install-plugin` (Go, [`packages/borgee/internal/cli/installbutler/`](../../../packages/borgee/internal/cli/installbutler/README.md), #996) implements the same canonical form byte-for-byte. The Go reference is `EntryCanonicalBytes` (5 lines), mirrored in the client as `entryCanonicalBytes` with a "MUST stay byte-identical" comment. Mismatch on either side = silent verify failure.

## SHA256 real values

This PR plumbs the signing chain but leaves `SHA256` zeros in the built-in default. Real values come from the first published `borgee-v*` tag — `publish-remote-agent.yml` builds the 4 platform binaries from native runners, stages them inside the single `@codetreker/borgee-remote-agent` tarball, and the operator records the registry .tgz URL + sha256 sum in `BORGEE_MANIFEST_ENTRIES_JSON` after the publish lands.
