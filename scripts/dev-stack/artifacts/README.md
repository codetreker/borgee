# openclaw-plugin/ — real plugin tarball staging dir

This directory stores the **real** `@codetreker/borgee-openclaw-plugin`
tarball that the dev-stack `borgee-server` container serves under
`/dev-artifacts/openclaw-plugin/<platform>`. Bytes are produced by:

```bash
scripts/dev-stack/build-plugin-artifact.sh
```

That script:

1. Runs `pnpm build` in `packages/plugins/openclaw/` (tsc → `dist/`).
2. Runs `npm pack` to produce `codetreker-borgee-openclaw-plugin-<version>.tgz`.
3. Copies the tarball to both `linux-x64` and `darwin-arm64` here (same
   bytes — the plugin is platform-independent JavaScript).
4. Prints the tarball's sha256 plus the matching
   `BORGEE_DEV_MANIFEST_SHA256_OVERRIDE` line for `scripts/dev-stack/.env`.

## Why the bytes are not committed

`.gitignore` in this dir excludes `linux-x64` / `darwin-arm64` (and any
`*.tgz`). The bytes are derived; pinning them to the repo would
invalidate on every plugin edit. Only the dir + the README + the build
script are committed.

## End-to-end chain (no fakes, no sentinels)

```
build-plugin-artifact.sh
  → tarball at artifacts/openclaw-plugin/<platform>
  → server-go reads via BORGEE_DEV_ARTIFACTS_DIR mount
    → serves at GET /dev-artifacts/openclaw-plugin/<platform> (real bytes)
    → signs a plugin-manifest at GET /dev-artifacts/manifests/openclaw-plugin/<platform>.json
      with the real sha256
  → helper-vm `borgee daemon` leases install_from_manifest job
    → rootd install-butler fetches the bytes, verifies sha
    → writes /usr/local/lib/borgee/openclaw/openclaw-plugin (real .tgz bytes)
  → systemd path-watcher openclaw-plugin-install.path in borgee-vm
    fires openclaw-plugin-install.service which:
       cp /usr/local/lib/borgee/openclaw/openclaw-plugin /tmp/borgee-plugin.tgz
       openclaw plugins install --force /tmp/borgee-plugin.tgz
  → docker exec borgee-vm openclaw plugins list shows
    @codetreker/borgee-openclaw-plugin (id=borgee) status=enabled
```

## NOT for production

This whole tree is dev-stack only. Production `borgee-server` deployments
leave `BORGEE_DEV_ARTIFACTS_DIR` unset; the `/dev-artifacts/*` handler
is not registered. The production install-from-manifest flow targets
`https://cdn.borgee.io` once release-helper.yml publishes signed
plugin manifests there.
