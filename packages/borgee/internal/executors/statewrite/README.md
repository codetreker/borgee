# state.write executor

Implements `JobTypeStateWrite` for the borgee daemon dispatcher (PR-3 #1041).
Runs inside the `borgee daemon` process as `User=borgee`; no root, no rootd
companion.

## Payload

```json
{ "state_key": "<relative/path>", "value_sha256": "sha256:..." }
```

`state_key` is a slash-separated relative path appended (safely) to the
manifest-declared root. The executor rejects any key containing `..`, NUL
bytes, or resolving outside that root.

`value_sha256` is optional; recorded into the written file for downstream
verification. The actual value bytes are not in the policy-checked payload.

## Path resolution

The write root is NOT a daemon-startup flag. The executor parses the signed
manifest + binding carried in the leased job and calls
`internal/executors/manifestpath.Resolve(manifest_json,
manifest_binding_json, "borgee_state_config")` to look up the absolute root
declared by the manifest for the `borgee_state_config` PathID. The
dispatcher's `jobpolicy.Evaluate` already verified the manifest signature
and the binding's PathIDs ⊆ manifest paths before the executor runs.

## Where it writes

`<resolved>/<state_key>.json` where `<resolved>` is the manifest-declared
absolute root. The systemd unit's `ReadWritePaths` must include that root
(or one of its ancestors); otherwise the atomic write fails loud.

## Server-side gap

As of PR-3 #1041 the server's `helper_job_queries.go` does not yet bind
`borgee_state_config` for `HelperJobTypeStateWrite` — the state.write job
type is itself not enumerated server-side. Until that gap closes (separate
issue), this executor is plumbing-only and will fail-loud
`manifest_missing_path_id` on any leased state.write job. No fallback path.

## Failure codes

| code | meaning |
|---|---|
| `schema_invalid` | empty/missing state_key, malformed payload, nil job |
| `manifest_missing_path_id` | binding does not list `borgee_state_config`, or manifest does not declare it |
| `manifest_invalid` | manifest JSON malformed or declares a non-absolute / `..`-containing root |
| `binding_invalid` | binding JSON malformed |
| `path_escape` | state_key resolves outside the manifest-declared root |
| `encode_failed` | JSON marshal of the record failed |
| `write_failed` | filesystem error writing the record file |
