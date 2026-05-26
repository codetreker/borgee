# Dev-stack sentinel artifacts

This directory carries committed-to-repo placeholder artifacts that the
dev-stack server-go container serves at `/dev-artifacts/<plugin>/<platform>`.
They exist solely so a local `docker compose up -d` dev-stack can complete
an end-to-end `openclaw.install_from_manifest` job (PR #1078, blocker #6/#8)
without depending on the unprovisioned production CDN.

## Layout

```
openclaw-plugin/linux-x64       — Linux helper-vm install target
openclaw-plugin/darwin-arm64    — macOS host install target (informational)
```

## Why committed (not generated)

The bytes are tiny (66 B each) shell scripts with a fixed sha256
(`2cab696f9b434cdf6b7be13e34fa6b241153e67372c8f06c383cab9906714c8b`). Pinning
the bytes + sha in the repo lets `.env` carry the matching
`BORGEE_DEV_MANIFEST_SHA256_OVERRIDE` JSON literal without a runtime
compute step that would otherwise need to land before `helpermanifest`'s
`LinuxDigest` package-level init runs.

## NOT for production

These bytes are not the real openclaw binary — they print a sentinel line
and exit 0. Production runs leave `BORGEE_DEV_MANIFEST_ORIGIN_BASE` /
`BORGEE_DEV_ARTIFACTS_DIR` unset and the canonical manifest declares
`https://cdn.borgee.io` (the placeholder release-helper.yml will eventually
populate).
