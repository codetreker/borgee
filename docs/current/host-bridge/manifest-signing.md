# Manifest signing — operational contract

`GET /api/v1/plugin-manifest` (HB-1) ships an ed25519-signed plugin manifest. This doc is the operator-facing contract: where the signing key lives, what gets signed, how to rotate, how install-butler verifies, what happens when config is missing.

Single source of truth for the canonical signing bytes is `packages/server-go/internal/api/manifest_signing.go::EntryCanonicalBytes`. Changing the format here without changing client (`install-butler`, #996) means breaking verification. Do both together.

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
    fmt.Println("# public (publish for install-butler): " + base64.StdEncoding.EncodeToString(pub))
}
GO
```

## Per-entry canonical form

Each entry is signed independently. Canonical bytes:

```
ID + "|" + Version + "|" + BinaryURL + "|" + SHA256
```

Separator is single ASCII `|` (0x7C). The four fields above are concatenated byte-for-byte, no JSON encoding, no whitespace, no trailing newline. The `Platforms` field is intentionally excluded — platforms is client-side metadata not security-relevant; tampering with platforms cannot trick the client into installing a different binary because BinaryURL + SHA256 still verify.

The base64-encoded signature is stored in `Signature` on each entry. install-butler verifies:

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

## Failure modes

- **Key env unset** — `LoadSigningKey` returns nil + logs `manifest_signing.key_unset` warn. Server keeps running. Per-entry `Signature=""`. install-butler in production must reject empty signatures. Dev environments can ignore (warn is the operator's signal).
- **Key env malformed** — logs `hb1.signing_key_invalid` error at startup. Same effect as unset.
- **Entry env malformed** — falls back to built-in default + logs `manifest_signing.entries_*_invalid`.

## Rotation

Signing key:

1. generate new key (see above) and store both private + public
2. publish new public key for install-butler (mechanism out of scope — likely versioned pubkey list shipped with helper)
3. update `BORGEE_MANIFEST_SIGNING_KEY` env in deploy
4. restart server — handler reads key at handler-construction time

Entry list (URLs, SHA256, versions):

1. update `BORGEE_MANIFEST_ENTRIES_JSON` env or `BORGEE_MANIFEST_ENTRIES_FILE` content
2. no restart needed — handler loads entries per request

## Client verification

install-butler (Rust, `packages/borgee-helper/install-butler`, #996) must implement the same canonical form byte-for-byte. The Go reference is `EntryCanonicalBytes` (5 lines). Mismatch on either side = silent verify failure.

## SHA256 real values

This PR plumbs the signing chain but leaves `SHA256` zeros in the built-in default. Real values come from `#1003`'s `release-helper.yml` `SHA256SUMS` artifact once the first `borgee-helper-v*` tag ships. Deploy step then writes the right env JSON pointing at the GitHub Release download URLs with matching SHAs.
