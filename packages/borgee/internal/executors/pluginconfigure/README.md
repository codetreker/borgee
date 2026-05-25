# borgee_plugin.configure_connection executor

Implements `JobTypePluginConfigureConnection` for the borgee daemon
dispatcher (PR-3 #1041). Runs inside the `borgee daemon` process as
`User=borgee`; no root, no rootd companion.

## Payload

Mirrors the server's `borgeePluginEffectivePayload` and the
`jobpolicy.validatePayload` shape:

```json
{
  "connection_id": "borgee-plugin:<sha-digest>",
  "agent_id":      "<id>",
  "channel_id":    "<id>"
}
```

The server computes `connection_id` from `(org_id, agent_id, channel_id)`
via a sha256 digest so the helper does not need the raw server URL / API
key in this payload — those are already on the dispatcher's authenticated
WS transport.

## Path resolution

The write root is NOT a daemon-startup flag. The executor parses the signed
manifest + binding carried in the leased job and calls
`internal/executors/manifestpath.Resolve(manifest_json,
manifest_binding_json, "borgee_plugin_config")` to look up the absolute
root declared by the manifest for the `borgee_plugin_config` PathID
(server-go's `helperJobBorgeePluginConfigPathID` names the same ID). The
dispatcher's `jobpolicy.Evaluate` already verified the manifest signature
and the binding's PathIDs ⊆ manifest paths before the executor runs.

## Where it writes

`<resolved>/<connection-suffix>.json` where `<resolved>` is the
manifest-declared absolute root. The systemd unit's `ReadWritePaths` must
include that root (or one of its ancestors); otherwise the atomic write
fails loud.

## Failure codes

| code | meaning |
|---|---|
| `schema_invalid` | bad payload, bad prefix, bad suffix, empty agent/channel, nil job |
| `manifest_missing_path_id` | binding does not list `borgee_plugin_config`, or manifest does not declare it |
| `manifest_invalid` | manifest JSON malformed or declares a non-absolute / `..`-containing root |
| `binding_invalid` | binding JSON malformed |
| `path_escape` | connection-suffix resolves outside the manifest-declared root |
| `encode_failed` | JSON marshal failed |
| `write_failed` | filesystem error writing the record |
