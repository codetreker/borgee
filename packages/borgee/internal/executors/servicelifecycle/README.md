# servicelifecycle executor

Dispatcher executor for `service.lifecycle`. Runs inside `borgee
daemon` (User=borgee, no root) — delegates the actual systemctl /
launchctl invocation to `borgee rootd`'s `service_lifecycle` whitelist
entry over local UDS IPC.

## Flow

1. Parse leased job payload (`{operation}`) — operation must be one of
   `start / stop / restart / reload / enable / disable`.
2. Read the leased job's manifest binding to find the bound
   `ServiceIDs` (today the server emits exactly one — `openclaw-user`).
3. Read the signed manifest to look up that `ServiceDeclaration` —
   yields `(Manager: systemd|launchd, Unit: <unit-name>)`. No daemon
   flag, no hardcoded unit name.
4. Build a typed `rootdclient.ServiceLifecycleRequest` and call rootd.
5. rootd re-validates manager + unit name + operation at the IPC
   boundary then exec's systemctl / launchctl. Stdout/stderr/exit_code
   round-trip back; a non-zero exit maps to `service_denied` with the
   stderr captured in the terminal failure_message.

## Allowed operations

`start / stop / restart / reload / enable / disable` — server-side
`decodeServiceLifecyclePayload` enforces the same set, and rootd's
handler validates a third time. Any other operation is rejected at all
three layers before exec.

## Known limitations

Same server-side manifest emission gap as installplugin — the policy
gate today rejects every leased service.lifecycle job. Executor
contract is forward-compatible.
