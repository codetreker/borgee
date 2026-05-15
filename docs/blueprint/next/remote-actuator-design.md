# Bounded Remote Actuator Design

> 状态: v1.1 planning contract for Helper / OpenClaw onboarding Phase. `HB-RA-1A` locks product guardrails; `HB-RA-1B` locks the execution-contract shape at Phase/Milestone planning granularity. Task-level Dev design still owns exact implementation choices.
> 目的: 把 PR #916 stance 与 PM / Architect / QA / Dev findings 收敛成下一版 host bridge / Helper remote actuator 设计。

## §1 Locked HB-RA-1A product guardrails

### §1.1 HB-RA-1A reference boundary

`HB-RA-1A` locks only the product guardrails in §1.2 for Phase planning. It does not lock the execution contract, queue protocol, credential shape, sandbox profile, revoke race mechanics, service permission matrix, or implementation slices below. Those execution-contract areas stay in `HB-RA-1B` and are tracked in §2.1 for milestone breakdown and task-level Dev design.

Task PRs that cite `HB-RA-1A` must cite §1.1-§1.2 or the matching README ledger row. They must not cite this whole document as if all draft execution design were locked.

### §1.2 Locked guardrails

- After explicit local enrollment, Web-side Configure OpenClaw may enqueue bounded, pre-authorized typed jobs without asking the user to SSH again.
- Enrollment-time delegation is not blanket preauthorization; it covers only a closed v1 taxonomy for OpenClaw / Helper lifecycle and config.
- The helper uses outbound poll / long-poll. The server never dials the host.
- Server enqueue authorization and helper local policy both validate owner, org, enrollment, delegation, job type, manifest/artifact, paths/domains, service IDs, and revocation state.
- Web sends schema-bound typed jobs, not arbitrary shell commands, argv, executable paths, scripts, or service unit names.
- Long-lived Helper / OpenClaw services stay non-sudo. `install-butler` remains short-lived, visible, and never caches sudo.
- Revoke / uninstall prevents future jobs, deterministically settles queued or leased jobs, invalidates helper auth, disables in-scope services, and is visible in UI.
- Status and logs are bounded and redacted; failed jobs cannot look successful or spin indefinitely.
- Helper UI placement may move, but Remote Agent credentials, grants, and enforcement rails remain separate from Helper actuator credentials, grants, and enforcement rails.

## §2 HB-RA-1B execution-contract planning scope

### §2.1 Contract areas carried into Phase 1

- Manifest signing and artifact binding: signing authority, digest scope, cache invalidation, replay handling, and artifact-to-job binding.
- Helper credential model: token shape, rotation cadence, stale-device semantics, local storage rules, and invalidation behavior.
- Sandbox and Linux outbound poll: exact macOS/Linux write paths, allowed network domains, outbound polling permission, and resolution of the current Linux AF_UNIX-only long-lived service restriction.
- Revoke race mechanics: safe action boundaries, lease cancellation behavior, terminal status precedence, and running-helper behavior when revocation wins.
- Service permissions: allowed service manager operations, long-lived service privilege level, restart/crash-recovery boundaries, and install-time privilege handoff.
- Exact queue/lease/result contract: job states, lease duration and renewal, idempotency keys, result schema, retry rules, terminal failure shape, and server/helper clock authority.

`HB-RA-1B` is locked for Phase/Milestone planning as the execution-contract shape for these areas. Task-level Dev design must still choose exact schemas, migrations, endpoints, permission profiles, and test contracts before code. References to this section must not be treated as part of the locked `HB-RA-1A` product guardrail scope.

## §3 Product stance

Borgee Helper is a remote actuator for bounded, pre-authorized host-management jobs after enrollment。它不是 Borgee command channel, 也不是 runtime owner。Remote Agent / Helper 如果在安装后仍要求用户再 SSH 才能由 web 触发 Configure OpenClaw, 该能力对 onboarding 没有产品价值。

Enrollment-time delegation 不是 blanket preauthorization。它只允许 closed v1 typed jobs 覆盖 OpenClaw / helper lifecycle 与 config; install / config paths 之外的 file / network / resource access 仍走 owner-controlled allowlists / revocation, 保留“装时轻、用时问、问时有理由”。

## §4 Lifecycle sequence

1. 用户在 host 上执行一次 explicit local enrollment / install。该步骤可包含必要 privileged setup, 但必须可见。
2. Web 显示 Helper connected、last seen、owner / org、allowed job categories 与 revoke / uninstall controls。
3. 用户点击 Configure OpenClaw。
4. Server 通过 owner / org / enrollment / delegation gate 创建 typed job, 写入 queue。
5. Enrolled helper 用 helper credential outbound poll / long-poll queue; server never dials host。
6. Helper 获取 job lease, ack 后按 fixed schema validate payload。
7. Helper local policy 再验证 owner / org、job_type、manifest_digest、artifact、approved paths / domains、declared service IDs、revocation state。
8. Helper 执行允许的 non-sudo action, 写 status / bounded logs / local audit。
9. Web 显示 queued / running / succeeded / failed、failure reason 与 redacted bounded logs。
10. 成功时显示 OpenClaw connected; 失败时显示 policy denial / retry / manual debug, 不能假成功或无限 spin。
11. 用户 revoke / disable delegation 或 uninstall 后, future jobs blocked, queued jobs denied / cancelled, helper auth invalidated, UI 显示 revoked / uninstalled。

Configure OpenClaw v1 closure 必须具体到: install plugin, create / update OpenClaw agent config, configure Borgee plugin connection / channel binding。

## §5 Identity / enrollment

Enrollment 产生独立于 Remote Agent file-proxy token 的 helper / device identity。Remote Agent file-proxy tokens 只服务文件代理 / agent runtime rail, 不能复用为 host-management delegation, 也不能扩展成 helper command credential。

v1 identity model:

| 字段 | 语义 |
|---|---|
| `enrollment_id` | server-side enrollment record, 绑定 owner / org / host label / allowed job taxonomy |
| `helper_device_id` | host 上 helper instance identity, 用于 stale-device rejection 与 last seen |
| `owner_user_id` / `org_id` | enqueue gate 与 helper local policy 双重验证 |
| one-time enrollment secret | 本地 enrollment 时短期使用, 交换 persistent helper credential 后失效 |
| persistent helper credential | helper outbound poll / long-poll 使用; 支持 rotation / revoke / stale-device rejection |

Credential rotation 必须让旧 credential 进入 stale-credential / stale-device state。Server enqueue gate 拒绝 revoked / stale enrollment; helper poll 也必须能看到 revoked / stale 状态并停止执行 queued 或 leased jobs。

## §6 Job queue contract

Job 是 server-authorized、helper-revalidated 的 typed record, 不是 shell command。

Required envelope:

| 字段 | 要求 |
|---|---|
| `job_id` | 全局唯一, 用于 lease / ack / result / audit |
| `enrollment_id` | 指向已授权 helper enrollment |
| `job_type` | 必须属于 closed v1 taxonomy |
| `schema_version` | 每个 job_type 固定 schema version |
| `payload` | schema-bound, reject unknown / extra fields |
| `manifest_digest` | 绑定 signed manifest / artifact; required when job touches install / config / service IDs |
| `status` | queued / leased / running / succeeded / failed / cancelled / expired |
| `lease` | helper 领取后的 lease token + expiry, 防重复执行 |
| `ack` | helper 已收到并通过基本 envelope validation |
| `result` | terminal status、failure reason、bounded log refs、audit refs |
| `ttl` | job 过期后不能执行 |
| `retry_backoff` | bounded retry / backoff, 不做无限重试 |
| `cancellation` | revoke / uninstall / user cancel 可取消 queued 和 leased-before-action jobs |
| `idempotency_key` | Configure OpenClaw 重试必须可幂等收敛 |

Terminal failure reasons 至少包括: `policy_denied`, `schema_invalid`, `unknown_job_type`, `manifest_invalid`, `artifact_invalid`, `path_denied`, `domain_denied`, `service_denied`, `revoked`, `stale_credential`, `wrong_owner`, `wrong_org`, `ttl_expired`, `lease_lost`, `cancelled`, `execution_failed`。

## §7 Closed v1 typed jobs

Before accepted work promotes this scope into current, the v1 job set must be recorded as a closed taxonomy. Draft 设计先限定为:

| job_type | 允许动作 | 关键约束 |
|---|---|---|
| `openclaw.install_from_manifest` | 从 signed manifest / artifact 安装或补齐 OpenClaw plugin | manifest_digest required; approved paths / domains only |
| `openclaw.configure_agent` | create / update OpenClaw agent config | fixed schema; approved config paths only |
| `borgee_plugin.configure_connection` | create / update Borgee plugin connection / channel binding | owner / org / channel authorization required |
| `service.lifecycle` | start / stop / restart enrolled helper / OpenClaw services | only declared service IDs from signed manifest / enrollment state |
| `state.write` | 写 approved helper / OpenClaw config / state paths | no arbitrary path, no private content dump |
| `status.collect` | collect helper / OpenClaw status and bounded logs | redacted, bounded, no tokens / secrets |
| `delegation.revoke` | disable delegation locally | must settle queued / leased jobs deterministically |
| `helper.uninstall` | uninstall / disable in-scope helper / plugin artifacts | disables autostart / service; no out-of-scope deletion |

Rejected by design: unknown job types, extra fields, arbitrary argv, arbitrary executable path, client-supplied script, client-supplied unit names, arbitrary local service restart, arbitrary shell, and paths / domains outside allowlists。

## §8 Helper policy / sandbox mechanics

Helper isolation remains defense in depth。Sandbox / limits must permit declared jobs, not arbitrary host control。

Allowed inside helper sandbox / policy:

- helper state and credential store needed for enrollment / rotation / revoke。
- approved OpenClaw / Borgee config paths from signed manifest / enrollment state。
- signed manifest / artifact cache with digest validation。
- bounded logs and local audit records。
- outbound HTTPS poll / long-poll to Borgee job queue and status endpoints。
- controlled service operations for enrolled service IDs declared by signed manifest / enrollment state。

Denied by default: inbound server dial to host, arbitrary network domains, arbitrary file writes, arbitrary executable paths, client-supplied scripts, client-supplied service unit names, and sudo cache。

Task-level implementation decision: Linux service currently has an AF_UNIX-only issue to resolve before outbound poll / long-poll can work from the long-lived helper service. Milestone breakdown and Dev design must resolve the exact sandbox profile write / network / service permissions before accepted work can move this design to current。

## §9 Privilege boundary

Normal Configure OpenClaw after enrollment is non-sudo typed jobs。Initial enrollment may do privileged setup when visible and locally approved。

`install-butler` remains short-lived: no autostart, no persistent daemon role, no supervised restart loop, no sudo cache, no silent escalation。Any later privileged setup must be a signed bounded install task with visible consent and a terminal audit / status result。Long-lived helper / agent services stay non-sudo。

## §10 Revoke / uninstall races

Revoke / uninstall are policy state changes, not best-effort UI hints。

- Future jobs: server enqueue gate blocks them immediately。
- Queued jobs: server marks denied / cancelled and helper must not execute them。
- Leased jobs before local action: helper revalidates revocation before action and settles as cancelled / revoked。
- Running jobs: helper revalidates before each bounded action boundary; if revoke is observed, it stops at the next safe boundary and records deterministic terminal status。
- Helper auth: persistent helper credential is invalidated; stale-credential poll returns revoked / stale state。
- Uninstall: disables autostart / service and removes or disables in-scope helper / plugin artifacts。
- UI: shows revoked / uninstalled, not offline-only ambiguity。

## §11 Status / logs / audit

UI / API must expose helper online / offline, last seen, allowed job categories, job queued / running / succeeded / failed, failure reason, bounded redacted logs, and revoke / uninstall state。

Logs must not expose tokens, secrets, private message content, private file content, or full environment dumps。Failed jobs must not look successful or spin indefinitely。Local audit records enforcement decisions: schema rejection, policy denial, manifest / artifact verification, path / domain denial, service denial, revoke / stale credential, lease / cancellation outcome。

## §12 Boot / crash / cron stance

Boot + crash restart are must-have for every long-lived process in the Configure OpenClaw value path。For remote-actuator v1, that path is owned by Host Bridge / Helper, not the existing `packages/remote-agent` file-proxy CLI。The current Remote Agent rail remains separate file browsing / reverse-WS infrastructure; it must not inherit helper enrollment credentials or host-management authority。

If a release still ships `packages/remote-agent` as a long-lived user-visible file-proxy process, it needs its own Mac / Linux boot + crash packaging before claiming that rail is reliable。That packaging is separate from Configure OpenClaw remote actuator scope and cannot merge credentials / grants / enforcement rails with Helper。

Boot + crash requirements do not apply to `install-butler`, which is short-lived only。

Fast / slow cron are Teamlead coordination / runtime timer / lease / heartbeat concepts, not product promise。Blueprint should model externally visible behavior as status, last seen, lease expiry, retry / backoff, and heartbeat freshness, not as user-facing cron guarantees。

## §13 Implementation slices

1. Enrollment / status foundation: enrollment record, helper_device_id, owner / org binding, one-time secret exchange, persistent helper credential, last seen, revoke / uninstall state。
2. Outbound pull skeleton: server queue, enqueue gate, long-poll endpoint, lease / ack / result, TTL, retry / backoff, idempotency key, cancellation。
3. Helper pull client: credential rotation, stale-device rejection, long-poll loop, lease handling, result upload, bounded logs。
4. Local policy engine: fixed schemas, manifest / artifact digest checks, allowlists, service ID checks, revoked / wrong-owner / wrong-org / stale-credential denial。
5. Scoped service lifecycle: only enrolled Borgee / OpenClaw service identifiers, boot + crash restart, bounded restart / backoff。
6. Configure OpenClaw closure: install plugin, create / update OpenClaw agent config, configure Borgee plugin connection / channel binding, connected / failed UI states。

## §14 Task-Level Decisions Before Current Promotion

The Phase/Milestone plan is locked at execution-contract granularity. These choices remain for milestone breakdown and task-level Dev design, not as blockers to the corrected Phase plan:

- Manifest signing / artifact binding: signing authority, digest scope, cache invalidation, replay handling。
- Helper credential model: token shape, rotation cadence, stale-device semantics, local storage rules。
- Sandbox profile: exact write paths, network domains, service permissions, and Linux AF_UNIX-only outbound poll fix。
- Revoke race rules: exact safe action boundaries, lease cancellation behavior, and terminal status precedence for running jobs。
- Service permissions: allowed service manager operations, long-lived service privilege level, restart / crash-recovery boundary, and install-time privilege handoff。
- Exact queue / lease / result contract: job states, lease duration and renewal, idempotency keys, result schema, retry rules, terminal failure shape, and server / helper clock authority。
- Remote-agent vs helper naming / boundary: user-facing IA may move, but credentials / grants / enforcement rails stay separate。
