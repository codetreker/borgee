# install-butler — HB-1 signed-manifest binary installer

One-shot CLI. Downloads a Borgee plugin runtime binary (e.g. `openclaw`),
verifies the manifest's ed25519 signature + the binary's SHA256, atomically
writes it to an approved path, exits. Exit 0 on success, 1 on any of seven
documented failure modes.

Blueprint锚: `docs/blueprint/current/host-bridge.md` §1.2 + §4.5. Operational
contract: [`docs/current/host-bridge/manifest-signing.md`](../../../../docs/current/host-bridge/manifest-signing.md).

## What it does (9 steps)

1. HTTPS `GET --manifest-url`.
2. Parse as `pluginManifestPayload` (fields byte-identical to server-go
   `PluginManifestEntry`).
3. Locate the entry whose `id == --plugin-id`.
4. ed25519 verify that entry's `Signature` using `--pubkey-base64`. Canonical
   bytes: `ID + "|" + Version + "|" + BinaryURL + "|" + SHA256` — same form
   as the server's `EntryCanonicalBytes`.
5. HTTPS `GET entry.BinaryURL`, stream to a tempfile on the same filesystem
   as `--target` (so the eventual rename is atomic).
6. Compute SHA256 on the stream. Reject if it differs from `entry.SHA256`.
   The tempfile is removed; `--target` keeps its prior content.
7. `--dry-run` exits 0 here with "would write to …, verified plan" on stdout.
8. `os.Rename(tempfile, --target)`, `chmod 0755`.
9. If `geteuid==0`, `chown borgee-helper:borgee-helper`. Then exit 0.
   **No daemon. No lingering sudo.**

## Flags

| flag | required | default | meaning |
|---|---|---|---|
| `--manifest-url` | yes | — | HTTPS plugin-manifest URL |
| `--pubkey-base64` | yes | — | base64 ed25519 public key (32B after decode) |
| `--plugin-id` | yes | — | e.g. `openclaw` |
| `--target` | yes | — | absolute path to write verified binary to |
| `--dry-run` | no | false | verify only; do not write target |
| `--http-timeout` | no | 60s | per-request HTTP timeout |
| `--helper-user` / `--helper-group` | no | `borgee-helper` | chown when root |
| `--allow-insecure-{manifest,binary}-url` | no | false | allow http:// (test only) |

## Example

```
sudo install-butler \
  --manifest-url https://app.borgee.io/api/v1/plugin-manifest \
  --pubkey-base64 BASE64_ED25519_PUBKEY \
  --plugin-id openclaw \
  --target /usr/local/lib/borgee/openclaw
```

## Failure modes (stderr `install-butler: <reason>: <detail>`, exit 1)

`manifest_fetch_failed` (network / non-2xx) ·
`manifest_parse_failed` (JSON) ·
`plugin_not_found` (id absent) ·
`signature_invalid` (ed25519 false, or empty signature) ·
`binary_fetch_failed` (network / non-2xx / EOF mid-stream) ·
`sha256_mismatch` (stream hash ≠ entry SHA256) ·
`write_failed` (mkdir / tempfile / rename / chmod failed).

## Security model

One-shot process — not a daemon. After rename + chmod + chown it exits;
kernel reaps. The drop-privilege story is "exit", not `Setresuid`. Operator
invokes via `sudo` once. Process never re-acquires privilege.

`--target` is never replaced on failure — rejected paths leave the pre-
existing file intact. Tempfile is always cleaned up.

For background update polling, wrap in a systemd timer or launchd
`StartCalendarInterval`. Do NOT turn install-butler itself into a daemon —
that would defeat the install-butler / host-bridge daemon split.

## Production invocation

- Initial install: `.deb` / `.pkg` postinstall (`packages/borgee-helper/
  install/postinstall.sh`, part of #1003 release pipeline).
- Update flow: #999 follow-up re-runs install-butler with the same flags;
  atomic rename keeps `--target` healthy mid-update.

## Reference

- Server signer + canonical bytes: `packages/server-go/internal/api/manifest_signing.go::EntryCanonicalBytes`
- Manifest endpoint: `packages/server-go/internal/api/host_manifest.go`
- Ops contract: [`manifest-signing.md`](../../../../docs/current/host-bridge/manifest-signing.md)
