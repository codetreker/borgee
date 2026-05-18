# Dev Design: Local Policy / Manifest / Sandbox Profile

## 1. Boundary And Approach

This task adds the Helper-side policy boundary that must pass after a server-authorized job is delivered and before any host-management action starts. The design is deliberately split from task 6: task 6 owns polling, leasing, result upload, retry/backoff, idempotency, cancellation, and result settlement; task 7 owns a local policy evaluator and the manifest/artifact/sandbox authority model that task 6 can call.

The implementation should create a focused Helper package, tentatively `packages/borgee-helper/internal/jobpolicy`, with pure validation functions and no HTTP client, no poll loop, no OpenClaw execution, and no service-manager calls. Server-side changes, if needed, should be limited to durable manifest-binding metadata and schema definitions for jobs that require local policy; do not mount new pull/lease/result routes from this task.

The policy evaluator accepts a candidate job envelope from the transport layer and returns a deterministic allow/deny decision with a reason code. It does not execute the job. The expected shape is:

```go
type Decision struct {
    Allow  bool
    Reason Reason
}
```

Reason literals should cover the local policy failures already named by the blueprint and accepted task docs: `ok`, `schema_invalid`, `unknown_job_type`, `manifest_invalid`, `artifact_invalid`, `path_denied`, `domain_denied`, `service_denied`, `revoked`, `stale_credential`, `wrong_owner`, `wrong_org`, and `policy_denied`. Task 8 can later decide how those reasons settle terminal job rows and logs.

## 2. Dev Scouting Inputs

Relevant code and docs inspected:

- Helper daemon entrypoint: `packages/borgee-helper/cmd/borgee-helper/main.go` validates outbound prereqs, opens the read-only grants DB, applies sandbox, and serves UDS IPC. It does not poll jobs or run local policy today.
- Helper outbound prereq: `packages/borgee-helper/internal/outbound/prereq.go` validates exact public HTTPS origins and Helper-owned queue/status/audit-handoff state roots. It rejects localhost/private/link-local/metadata literal origins by default but does not resolve DNS/CNAME chains.
- Service assets: `packages/borgee-helper/install/borgee-helper.service`, `cloud.borgee.host-bridge.plist`, and `borgee-helper.sb` add outbound networking and Helper-owned state roots while preserving local UDS as the only inbound path.
- Existing sandbox/ACL: `internal/sandbox`, `internal/acl`, `internal/grants`, and `internal/fileio` are for current host-grant-backed IPC reads. They are useful patterns but should not be reused as authority for Helper jobs.
- Server job enqueue: `packages/server-go/internal/store/helper_job_queries.go`, `internal/api/helper_jobs.go`, and migration v51 create enqueue-only `helper_jobs` rows. Current enabled job type is only `openclaw.configure_agent`; manifest-required and service/lifecycle types are recognized but rejected.
- Manifest verifier: `packages/borgee-installer/internal/manifest` verifies an installer `Envelope` with `entries` and `signed_at`. Server `host_manifest.go` currently serves a different `PluginManifestPayload` shape with `plugins`, `manifest_version`, `issued_at`, and `expires_at`. Current docs call this installer trust boundary partial.
- Current docs: `docs/current/host-bridge/*`, `docs/current/security/README.md`, and `docs/current/server/*` record that Helper polling and local policy are not current behavior.

File ownership recommendation for implementation:

- Create: `packages/borgee-helper/internal/jobpolicy/policy.go`
- Create: `packages/borgee-helper/internal/jobpolicy/policy_test.go`
- Create or extend only if implementation needs reusable digest helpers: `packages/borgee-helper/internal/jobpolicy/manifest.go` and `manifest_test.go`
- Modify only if policy must be configurable at daemon startup: `packages/borgee-helper/cmd/borgee-helper/main.go`, `packages/borgee-helper/internal/sandbox/*`, and `packages/borgee-helper/install/*`
- Modify server files only for manifest-binding metadata needed by policy fixtures; do not implement task 6 transport endpoints here.

## 3. Security Scouting Inputs

Security review should focus on fail-closed boundaries rather than execution behavior:

- The existing installer verifier is not sufficient as the local Helper job policy trust boundary because envelope shapes differ and local artifact binding is documented as incomplete. Task 7 must either add a runtime policy manifest verifier with a clear signed byte shape or explicitly adapt and align the existing verifier before claiming manifest/artifact trust.
- Policy must derive authority from Helper enrollment state, server-owned job metadata, signed manifest/artifact binding, and task 5 outbound/sandbox config. It must not derive authority from Remote Agent tokens, host grants, plugin API keys, user permissions, local plugin file-access allowlists, or job payload fields.
- Current task 5 origin validation does not resolve DNS/CNAME chains. Do not claim that domain policy prevents rebinding unless implementation adds runtime DNS/network-policy enforcement; for this task, exact origin and literal-host denial remain the documented boundary.
- Do not let manifest fields grant shell, argv, executable path, client-supplied service units, arbitrary deletion paths, arbitrary network domains, full environment dumps, private content dumps, or sudo behavior.
- Artifact verification must bind the artifact bytes or local cache entry to a digest named by the signed manifest and job binding. A verified manifest without artifact digest verification is not enough.
- Sandbox alignment must fail closed: if policy allows a path/domain/service category but platform sandbox or outbound prereq cannot support it, the policy decision should deny or report a policy/sandbox mismatch before action.

## 4. Policy Schema

Define Helper-side policy input structs separately from server enqueue request structs. The local policy input is a post-enqueue, server-owned job view, not a browser request:

```go
type Job struct {
    JobID              string
    OwnerUserID        string
    OrgID              string
    EnrollmentID       string
    HelperDeviceID     string
    CredentialGeneration int
    JobType            string
    Category           string
    SchemaVersion      int
    PayloadJSON        []byte
    PayloadHash        string
    ManifestDigest     string
    ManifestBindingJSON []byte
    ExpiresAt          int64
}

type EnrollmentState struct {
    OwnerUserID          string
    OrgID                string
    EnrollmentID         string
    HelperDeviceID       string
    CredentialGeneration int
    Status               string
    Revoked              bool
    Uninstalled          bool
    StaleCredential      bool
    AllowedCategories    []string
}
```

The exact field names can differ, but the semantics should be present. Local policy must re-check that job owner/org/enrollment/device/generation match current local/server-supplied Helper state before action. `pending`, `revoked`, `uninstalled`, wrong owner, wrong org, wrong device, missing enrollment, missing category delegation, stale credential/device, or expired job input denies.

For schema validation, use strict decoding at two layers:

- Envelope-level fields are fixed and reject unknown or missing required fields.
- Payload-level structs are selected by `job_type` and `schema_version`; each decoder rejects unknown fields and known forbidden names.

Initial local policy should support the closed v1 taxonomy as recognized types, even if some return deny because their action implementation is not present:

| Job type | Local policy stance |
|---|---|
| `openclaw.configure_agent` | Schema-check server-derived config binding. It may allow policy if no manifest/path/service authority is needed and owner/org/device/state match. It still does not execute OpenClaw config in this task. |
| `openclaw.install_from_manifest` | Requires signed manifest, artifact digest verification, allowed install/config paths, allowed domains, and declared service IDs before it can allow policy. |
| `borgee_plugin.configure_connection` | Requires server-derived connection/channel binding and allowed config path. It must not accept client-supplied base URLs or credentials as policy authority. |
| `service.lifecycle` | Requires declared service ID from signed manifest/enrollment state and fixed operation enum. It must not accept arbitrary unit names. |
| `state.write` | Requires manifest-declared state key/path under Helper/OpenClaw config roots. It must not accept arbitrary paths or raw private content. |
| `status.collect` | Requires closed status scope and bounded output plan. It must not accept arbitrary log paths or selectors. |
| `delegation.revoke` | Requires current enrollment/category authority and local state check. Settlement remains task 8. |
| `helper.uninstall` | Requires helper lifecycle category and in-scope artifacts/services only. Actual uninstall action remains later work. |

Unknown `job_type`, unsupported `schema_version`, malformed JSON, extra fields, and forbidden execution fields deny locally with no action.

## 5. Manifest And Artifact Binding

Task 7 should not treat the current installer manifest path as already sufficient for runtime Helper jobs. The design should establish a local runtime policy manifest contract with these properties:

- One canonical signed byte shape. Prefer a new Helper policy manifest type instead of reusing the existing mismatched installer/server shapes without alignment.
- Ed25519 verification over manifest metadata before any artifact/path/domain/service authority is trusted.
- Manifest freshness fields, such as `issued_at` and `expires_at`, checked against the local policy clock with a bounded skew rule.
- A manifest digest computed over canonical signed bytes and compared to `job.ManifestDigest`.
- Artifact entries containing ID, platform, version, digest, optional size, and source origin. Artifact bytes or cache entries must hash to the signed digest before policy can allow install/config work.
- Path/domain/service declarations live inside the signed manifest or server-owned `manifest_binding_json`; job payload fields cannot add them.

A minimal policy manifest shape can be:

```json
{
  "manifest_version": 1,
  "issued_at": 1760000000000,
  "expires_at": 1760086400000,
  "artifacts": [
    {"id":"openclaw-plugin", "platform":"linux-x64", "sha256":"...", "origin":"https://cdn.borgee.io"}
  ],
  "paths": [
    {"id":"openclaw_config", "root":"/var/lib/openclaw", "mode":"write_config"}
  ],
  "domains": ["https://app.borgee.io", "https://cdn.borgee.io"],
  "services": [
    {"id":"openclaw-user", "platform":"linux", "manager":"systemd", "unit":"openclaw.service"}
  ],
  "signature":"base64-ed25519"
}
```

This JSON is illustrative; implementation can choose a different exact struct after review. The important constraints are canonical signing, digest comparison, artifact digest verification, and signed authority declarations.

`manifest_binding_json` in `helper_jobs` should remain server-owned and narrow. It can bind a job to manifest digest, artifact IDs, path IDs, domain IDs, and service IDs selected by the server. It must not carry raw shell, argv, executable paths, private secrets, arbitrary URLs, raw files, or full manifest blobs unless those blobs are signed and verified by the local policy package.

## 6. Signature And Digest Verification Boundary

Verification order should be explicit and fail closed:

1. Decode the job envelope with strict schema checks.
2. Check current Helper enrollment state and job owner/org/enrollment/device/category binding.
3. For manifest-required jobs, decode the signed manifest envelope and verify Ed25519 signature with configured trust roots.
4. Compute canonical manifest digest and compare to the job `manifest_digest`.
5. Decode `manifest_binding_json` and ensure every referenced artifact/path/domain/service ID exists in the verified manifest.
6. Verify artifact cache bytes against the signed SHA-256 digest before any action can use them.
7. Check path/domain/service declarations against the platform and sandbox profile.
8. Return allow only if all applicable checks pass.

Trust roots should be explicit startup/config input or compiled test seams. Production must not silently accept nil signing keys, placeholder signatures, or unsigned manifests. Tests should include bad signature, wrong key, changed signed bytes, missing signature, digest mismatch, artifact digest mismatch, expired manifest, unknown binding ID, and replay/wrong-owner/wrong-org cases.

## 7. Allowlisted Paths, Domains, And Service IDs

Path policy:

- Normalize paths with `filepath.Clean`, require absolute paths, reject NUL bytes, reject `..` segments before and after cleaning, and reject symlink escape unless implementation has an explicit realpath strategy.
- Allow only signed manifest path IDs and Helper-owned state roots from task 5. Do not accept raw job payload paths.
- Separate read/write categories. Task 7 may define policy categories for later actions, but it should not perform file writes.

Domain policy:

- Reuse the origin-normalization stance from `internal/outbound` for exact public HTTPS origins.
- Allow domains/origins only when present in the signed manifest and compatible with the task 5 allowed origins. Job payloads cannot add domains.
- Preserve the documented DNS limitation unless runtime DNS/network-policy enforcement is added: exact origin validation rejects unsafe literal origins, but does not prove DNS answers or CNAME chains cannot resolve privately.

Service ID policy:

- Treat service identifiers as logical IDs selected from signed manifest/enrollment state, not as client-supplied systemd/launchd unit names.
- A binding may map `service_id` to a platform-specific unit/plist name only after manifest verification.
- Allowed operations must be a fixed enum such as `start`, `stop`, `restart`, or `disable`, but task 7 should only validate the enum and service ID; it must not call the service manager.

## 8. Sandbox Profile Alignment

Policy and sandbox need a shared vocabulary. The design should add a small mapping layer from policy declarations to platform sandbox affordances:

- Linux: systemd `ReadWritePaths` and Helper-owned state roots already cover queue/status/audit-handoff. Landlock currently supports read-only `ReadPaths`; write-capable job paths must not be claimed as available unless the service/sandbox design explicitly grants them in a later action task.
- macOS: static `borgee-helper.sb` permits current Helper state roots and remote TCP. If future manifest-declared paths are needed, the generated/static profile must name them before policy can allow the job.
- Both platforms: local UDS remains the only inbound listener; task 7 must not add inbound TCP or broad host-control listeners.

Implementation should represent sandbox capability as input to policy, for example `SandboxProfile{WriteRoots, ReadRoots, AllowedOrigins, ServiceIDs}`. Policy returns `policy_denied` or a more specific reason if manifest authority exists but sandbox/profile support is absent. Do not widen sandbox assets in task 7 unless the specific path/domain/service declaration is locked by tests and remains non-executing.

## 9. Auth And State Rejection

The local policy must duplicate critical server authority checks because enqueue approval is not sufficient once local state changes:

- `owner_user_id`, `org_id`, `enrollment_id`, `helper_device_id`, and `credential_generation` must match current Helper state.
- `status` must not be pending, revoked, uninstalled, or otherwise inactive.
- Stale credentials or stale device identity deny before action.
- Allowed category must contain the job category.
- Job expiry denies before action. Task 6 owns how expiry is fetched and settled; task 7 only returns the denial reason.
- Wrong owner/org/device/enrollment deny even if the server queue row previously existed.

No Remote Agent credential, remote node token, host grant, plugin WebSocket identity, file-proxy status, user permission row, or admin identity can satisfy local Helper policy.

## 10. Test Plan

Focused tests should be written before implementation and should not require a live server, root, systemd, launchd, or a poll endpoint.

Helper policy unit tests:

- Allow a minimal `openclaw.configure_agent` candidate only when owner/org/enrollment/device/category/schema/payload match.
- Reject unknown job type, unsupported schema version, extra envelope fields, and forbidden payload fields such as `shell`, `argv`, `script`, `service_unit`, `path`, `domain`, `url`, `credential`, and `env`.
- Reject wrong owner, wrong org, wrong enrollment, wrong device, stale credential, revoked state, uninstalled state, missing category, and expired job.
- Reject manifest-required jobs with missing manifest, bad signature, wrong key, expired manifest, digest mismatch, missing artifact, artifact digest mismatch, unknown path ID, unknown domain ID, unknown service ID, and platform mismatch.
- Reject path traversal, relative paths, NUL paths, root-only paths when a child path is required, local/private/link-local/metadata literal origins, client-added domains, and undeclared service IDs.

Sandbox/profile tests:

- Policy denies a manifest-declared write path when sandbox capability input lacks that write root.
- Policy denies a manifest-declared domain when task 5 allowed origins do not include it.
- Policy denies a manifest-declared service when sandbox/service capability input lacks the declared service ID.
- Asset/static tests continue to prove no `--poll-loop`, `--lease`, `--result`, `--restart-service`, `sudo`, `--reverse-ws`, or Remote Agent flags appear in task 7 changes.

Server-side tests, only if server binding changes are made:

- `manifest_binding_json` is server-owned, digest-bound, and not serialized publicly.
- Recognized manifest-required job types remain not executable unless the required server binding exists.
- No pull/lease/result/local-policy HTTP routes are mounted by task 7.

Verification commands expected after implementation:

```bash
GOTMPDIR=$PWD/.gotmp go test ./internal/jobpolicy ./internal/outbound ./install
GOTMPDIR=$PWD/.gotmp go test ./cmd/borgee-helper ./internal/...
git diff --check
```

If server files change, also run focused server tests for helper jobs/enrollment and migration/schema coverage.

## 11. Docs/Current Sync

After implementation, update current docs only for behavior that actually lands:

- `docs/current/host-bridge/helper-daemon.md`: add the local policy evaluator role, reason shape, manifest/artifact verification boundary, and unchanged no-poll/no-execution limits if still true.
- `docs/current/host-bridge/README.md`: update Host Bridge architecture and key flows for local policy as a pre-action gate.
- `docs/current/security/README.md`: add Helper local policy as a separate rail boundary and record manifest/artifact/path/domain/service rejection rules.
- `docs/current/server/data-model-and-migrations.md` and `docs/current/server/api-auth-admin-rails.md`: update only if server-side manifest binding or job taxonomy behavior changes.
- `docs/current/known-gaps.md`: remove local policy from known gaps only if implemented; keep pull/lease/result/action execution/service lifecycle gaps until their tasks land.

Docs must not claim OpenClaw install/config succeeded, services restarted, jobs were polled, results were uploaded, bounded logs were uploaded, or revoke settlement was completed by this task.

## 12. Non-Goals

- No Helper poll loop, long-poll loop, lease acquisition, ack, result upload, retry/backoff, idempotency, cancellation, or bounded log upload.
- No OpenClaw install, agent config write, plugin connection write, channel binding action, service lifecycle operation, helper uninstall action, or status/log collection action.
- No service-manager calls, boot/crash restart, arbitrary unit execution, sudo cache, persistent privileged service, or silent escalation.
- No arbitrary shell, argv, executable path, script, command channel, arbitrary local path, arbitrary network domain, or client-supplied service unit.
- No Remote Agent rail reuse: no Remote Agent credentials, reverse WebSocket transport, host grants, file-proxy status, remote node rows, or user permission fallback.
- No Configure OpenClaw terminal UI, job progress UI, bounded logs UI, or OpenClaw connected/failed closure.

## 13. Review Checklist

- Architect: confirm `internal/jobpolicy` is a clean pre-action evaluator and does not entangle with task 6 transport or task 9/11 action execution.
- PM: confirm manifest/path/domain/service policy is bounded enough for Configure OpenClaw later without introducing arbitrary host command capability.
- QA: confirm the test plan proves negative cases and can run without task 6 endpoints.
- Security: confirm existing installer manifest gaps are not overstated as solved, artifact verification binds actual bytes/cache entries, and Remote Agent/host-grant/user-permission rails cannot authorize Helper policy.

Task status after this design: ready for design review. Product implementation remains blocked until role review accepts the design.
