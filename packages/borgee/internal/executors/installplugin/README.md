# installplugin executor

Dispatcher executor for `openclaw.install_from_manifest`. Runs inside
`borgee daemon` (User=borgee, no root) — delegates the actual fetch+
verify+place sequence to `borgee rootd`'s `install_plugin` whitelist
entry over local UDS IPC.

## Flow

1. Parse leased job payload (`{install_plan_id}`) + manifest binding.
2. Resolve the `openclaw_install` PathID and the bound origin from the
   signed manifest (no daemon flag, no hardcoded path).
3. Build a typed `rootdclient.InstallPluginRequest` and call rootd.
4. rootd invokes install-butler in-process (which is already a clean
   `Run(args, stdout, stderr)` entrypoint — no separate binary spawn).
5. Map rootd's reason:detail line onto a TerminalStatus failure_code so
   the operator sees the exact install-butler failure mode
   (`manifest_fetch_failed` / `signature_invalid` / `sha256_mismatch` /
   `write_failed` / `binary_fetch_failed` / `plugin_not_found` /
   `manifest_parse_failed`).

## Known limitations (server-side gap follow-up)

The server's leased-job emission today carries `manifest_digest +
manifest_binding_json` but NOT the manifest JSON body itself. Until
the follow-up wires manifest emission + trust root distribution, the
helper-side `jobpolicy.Evaluate` rejects every manifest-required job
at the policy gate; this executor therefore reaches `Execute` only in
tests. The contract is correct now — when manifest emission lands, no
executor change is needed.

The trust root pubkey is read from `BORGEE_MANIFEST_SIGNING_PUBKEY`
env at daemon startup; the same key install-butler verifies plugin
manifest signatures against (see `manifest-signing.md`).
