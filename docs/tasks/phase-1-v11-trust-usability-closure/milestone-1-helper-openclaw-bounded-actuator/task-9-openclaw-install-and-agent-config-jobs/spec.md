# Spec Brief: OpenClaw Install And Agent Config Jobs

## 0. Constraints

Task contract: make Configure OpenClaw enqueue closed typed jobs for OpenClaw plugin install/config and OpenClaw agent config after the Task6 transport/result rail, Task7 local policy, and Task8 terminal settlement exist. The job records must be server-owned, manifest/path/artifact/domain-bound, and safe for Helper local policy to revalidate before any future action.

Blueprint anchors:

- `HB-RA-1A` (`remote-actuator-design.md` section 1.2): Web-side Configure OpenClaw may enqueue bounded typed jobs after enrollment; server enqueue and Helper local policy both validate authority; Web must not send shell, argv, executable path, scripts, service unit names, arbitrary paths, or arbitrary domains.
- `HB-RA-1B` (`remote-actuator-design.md` sections 7 and 9): closed v1 typed jobs include `openclaw.install_from_manifest` and `openclaw.configure_agent`; normal Configure OpenClaw remains non-sudo, while `install-butler` remains short-lived and visible if later privileged setup is needed.

Dependencies: Task6 PR #943 supplies poll/lease/ack/result. Task7 PR #942 supplies local manifest/artifact/path/domain/service policy. Task8 PR #954 supplies bounded terminal settlement and redacted references.

## 1. Segmentation

Segment A: Server-owned install job.
`openclaw.install_from_manifest` is enabled only for Helpers with `openclaw_lifecycle` delegation. The browser can express OpenClaw install intent, but the server derives the effective install payload, manifest digest, artifact ID, approved path IDs, and approved artifact origin. Client-supplied manifest, artifact, URL, path, command, service, credential, TTL, or config-hash authority is rejected.

Segment B: Server-owned agent config job.
`openclaw.configure_agent` remains enabled for `openclaw_config` delegation, but its stored effective payload is server-derived from the target agent config row and now includes server-owned manifest/path binding for approved OpenClaw config paths. Optional channel access remains an authorization check only; Borgee plugin channel binding execution is not in this task.

Segment C: Helper lease projection.
Poll/lease may expose only safe effective payload, manifest digest, server-owned manifest binding, lease token, and lease metadata. User enqueue responses continue to hide payload bodies, manifest digests, owner/org internals, Helper credentials, and logs.

Segment D: Helper local policy alignment.
Local policy treats `openclaw.configure_agent` as manifest-required and requires signed manifest plus approved config path binding before allow. `openclaw.install_from_manifest` continues to require signed manifest, artifact digest, approved paths, approved artifact origin, and sandbox/profile affordances.

Segment E: Current-doc and evidence sync.
`docs/current` must describe the current job boundary without claiming OpenClaw execution, plugin channel binding, service lifecycle, raw log upload, sudo, or Configure OpenClaw success.

## 2. Out Of Scope

- No Borgee plugin channel binding job; that is Task10.
- No service lifecycle, boot/crash restart, sudo cache, privileged long-lived service, or install-butler execution; that is Task11 or later action work.
- No Configure OpenClaw terminal UI closure; that is Task12.
- No Remote Agent rail reuse, Remote Agent credentials/grants/status fallback, or reverse-WS transport changes.
- No raw/bulk log upload, private file/message content upload, arbitrary command channel, shell, argv, executable path, script, arbitrary service unit, arbitrary path, or arbitrary domain authority.

## 3. Reverse Checks

- If a client can supply manifest/artifact/path/domain/service/config-hash/TTL authority for OpenClaw install or config, the task fails the closed typed-job boundary.
- If `openclaw.install_from_manifest` can enqueue without `openclaw_lifecycle` delegation or `openclaw.configure_agent` without `openclaw_config`, server authority is too broad.
- If Helper local policy can allow OpenClaw config without signed manifest plus approved config path binding, enqueue approval has bypassed local policy.
- If docs describe this as OpenClaw execution success, plugin channel binding, service lifecycle, sudo behavior, terminal UI closure, or Remote Agent rail reuse, the task exceeds scope.
