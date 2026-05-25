# openclaw.configure_agent executor

Implements `JobTypeOpenClawConfigureAgent` for the borgee daemon dispatcher
(PR-3 #1041). Runs inside the `borgee daemon` process as `User=borgee`; no
root, no rootd companion.

## Payload

Mirrors the server's `openClawEffectivePayload` and the
`jobpolicy.validatePayload` shape:

```json
{
  "agent_id":              "<id>",
  "channel_id":            "<id>",
  "config_schema_version": 7,
  "config_hash":           "sha256:..."
}
```

The server resolves the canonical effective payload from its
`agent_configs` table (server is the source of truth for the blob's
content + hash). The executor here records the server-attested metadata.

## Path resolution

The write root is NOT a daemon-startup flag. The executor parses the signed
manifest + binding carried in the leased job and calls
`internal/executors/manifestpath.Resolve(manifest_json,
manifest_binding_json, "openclaw_agent_config")` to look up the absolute
root declared by the manifest for the `openclaw_agent_config` PathID
(server-go's `helperJobOpenClawAgentConfigPathID` names the same ID). The
dispatcher's `jobpolicy.Evaluate` already verified the manifest signature
and the binding's PathIDs ⊆ manifest paths before the executor runs.

## Where it writes

`<resolved>/<agent_id>.json` where `<resolved>` is the manifest-declared
absolute root. The systemd unit's `ReadWritePaths` must include that root
(or one of its ancestors); otherwise the atomic write fails loud.

## Failure codes

| code | meaning |
|---|---|
| `schema_invalid` | bad payload, invalid agent_id, bad config_hash, bad schema_version, nil job |
| `manifest_missing_path_id` | binding does not list `openclaw_agent_config`, or manifest does not declare it |
| `manifest_invalid` | manifest JSON malformed or declares a non-absolute / `..`-containing root |
| `binding_invalid` | binding JSON malformed |
| `path_escape` | agent_id resolves outside the manifest-declared root |
| `encode_failed` | JSON marshal failed |
| `write_failed` | filesystem error writing the record |
