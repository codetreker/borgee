# Spec Brief: Job Envelope And Enqueue Authority

## 0. Constraints

Task contract: start the first Typed Job Policy Loop task by defining the boundary for server-authorized Helper job creation. This task owns the job envelope and enqueue authority only. It must not implement Helper polling, local execution, Linux service lifecycle, local policy manifest/sandbox work, or Configure OpenClaw closure UI.

Blueprint anchors:

- `HB-RA-1A` (`remote-actuator-design.md` section 1.2): Web may enqueue bounded, pre-authorized typed jobs only after explicit local Helper enrollment. Server enqueue authorization and later Helper local policy both validate authority; Web sends schema-bound jobs, not shell commands or client-provided execution authority.
- `HB-RA-1B` (`remote-actuator-design.md` sections 6 and 7): the queue record is a typed job envelope with closed taxonomy, idempotency, TTL, status, failure shape, and rejection of unknown job types and extra fields. Exact schemas and implementation choices remain Dev design work.
- `PS-1` (`migration-analysis.md` section 6.1): preserve data minimization and admin/user/agent rail separation; do not add user-facing privacy/compliance product scope.

Dependency base:

- Canonical tasks 1-3 are accepted through PR #934, PR #936, and PR #937. Helper enrollment identity, credential lifecycle/revoke authority, and user-visible Helper status are available as accepted inputs for task 4 planning.

## 1. Segmentation

Segment A: Typed job envelope boundary.
The accepted task records and later implements a server-owned job envelope shape for Helper jobs. The envelope should carry enough authority and lifecycle data for enqueue, idempotency, TTL, and terminal failure reporting without accepting arbitrary client execution fields.

Segment B: Server enqueue authority.
The accepted task gates job creation on owner, org, Helper enrollment, delegation/category, job type, and revocation state. A browser request may ask for a supported typed job, but the server remains the authority that decides whether a queue record exists.

Segment C: Closed v1 job taxonomy at enqueue.
The accepted task starts from the closed v1 taxonomy in the blueprint and rejects unknown job types. It may narrow which job types are initially enqueueable if Dev design finds that safer, but it must not broaden into arbitrary command, path, domain, service, or script authority.

Segment D: Schema and negative-field handling.
The accepted task rejects unknown payload fields and client-supplied execution authority. Payload validation belongs at enqueue for server-side trust, and later Helper local policy revalidation remains a separate task.

Segment E: Queue lifecycle seeds.
The accepted task defines the server-side state needed for queued jobs, including idempotency, TTL, initial status, and terminal failure shape for enqueue-time denials. Lease, ack, result upload, retry execution, cancellation settlement, and bounded logs are later tasks unless a minimal schema placeholder is needed to keep the enqueue record coherent.

## 2. Carry-Over

Carry into later Dev design, but do not solve in this task-start package:

- Exact database migrations, endpoint names, schema versions, enum names, and test contracts.
- Exact manifest/artifact signing authority and local policy allowlists beyond enqueue-time references needed to deny client-supplied authority.
- Helper-side lease/result protocol, local policy enforcement, sandbox permissions, outbound poll mechanics, and service lifecycle semantics.
- OpenClaw closure UX and job progress/log UI.

## 3. Reverse Checks

- If a client can provide shell, argv, executable path, script body, arbitrary service unit, arbitrary local path, or arbitrary network domain, the task violates `HB-RA-1A`.
- If enqueue authorization skips owner, org, enrollment, delegation/category, job type, or revocation checks, the task cannot be accepted.
- If queue creation depends on Remote Agent credentials, host grants, user permissions fallback, or file-proxy status, rail separation is broken.
- If failed enqueue or invalid job input can look queued/successful, terminal status truthfulness is incomplete.
- If docs describe this task as Helper polling, lease execution, local sandbox policy, Linux service lifecycle, or Configure OpenClaw UI closure, the scope is too broad.

## 4. Out Of Scope

- Helper polling, long-poll transport, lease acquisition, ack/result upload, retry/backoff execution, and leased-job cancellation.
- Linux service lifecycle, boot/crash restart, AF_UNIX/outbound network permission repair, sudo handoff, or service-manager operations.
- Local policy manifest/sandbox profile, artifact cache validation on host, path/domain/service allowlists enforced by Helper, and Helper-side execution.
- OpenClaw closure UI, job progress UI, bounded log UI, terminal job result UI, or claims that OpenClaw is installed/configured.
- Remote Agent rail changes, shared Helper/Remote Agent credentials, shared grants, or new user-facing privacy/compliance product surfaces.
