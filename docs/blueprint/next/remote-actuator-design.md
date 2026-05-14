# Bounded Remote Actuator Design

> 状态: 草拟 / 讨论稿, 不是 frozen blueprint。
> 目的: 把 PR #916 stance 与 PM / Architect / QA / Dev findings 收敛成下一版 host bridge / Helper remote actuator 设计。

## 0. Product stance

Borgee Helper is a remote actuator for bounded, pre-authorized host-management jobs after enrollment。它不是 Borgee command channel, 也不是 runtime owner。Remote Agent / Helper 如果在安装后仍要求用户再 SSH 才能由 web 触发 Configure OpenClaw, 该能力对 onboarding 没有产品价值。

Enrollment-time delegation 不是 blanket preauthorization。它只允许 closed v1 typed jobs 覆盖 OpenClaw / helper lifecycle 与 config; install / config paths 之外的 file / network / resource access 仍走 owner-controlled allowlists / revocation, 保留“装时轻、用时问、问时有理由”。

## 1. Lifecycle sequence

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

## 2. Identity / enrollment

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

## 3. Job queue contract

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

## 4. Closed v1 typed jobs

Freeze 前必须把 v1 job set 作为 closed taxonomy 写入 current blueprint。Draft 设计先限定为:

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

## 5. Helper policy / sandbox mechanics

Helper isolation remains defense in depth。Sandbox / limits must permit declared jobs, not arbitrary host control。

Allowed inside helper sandbox / policy:

- helper state and credential store needed for enrollment / rotation / revoke。
- approved OpenClaw / Borgee config paths from signed manifest / enrollment state。
- signed manifest / artifact cache with digest validation。
- bounded logs and local audit records。
- outbound HTTPS poll / long-poll to Borgee job queue and status endpoints。
- controlled service operations for enrolled service IDs declared by signed manifest / enrollment state。

Denied by default: inbound server dial to host, arbitrary network domains, arbitrary file writes, arbitrary executable paths, client-supplied scripts, client-supplied service unit names, and sudo cache。

Open implementation blocker: Linux service currently has AF_UNIX-only issue to resolve before outbound poll / long-poll can work from the long-lived helper service. Freeze must decide the exact sandbox profile write / network / service permissions before moving this design to current。

## 6. Privilege boundary

Normal Configure OpenClaw after enrollment is non-sudo typed jobs。Initial enrollment may do privileged setup when visible and locally approved。

`install-butler` remains short-lived: no autostart, no persistent daemon role, no supervised restart loop, no sudo cache, no silent escalation。Any later privileged setup must be a signed bounded install task with visible consent and a terminal audit / status result。Long-lived helper / agent services stay non-sudo。

## 7. Revoke / uninstall races

Revoke / uninstall are policy state changes, not best-effort UI hints。

- Future jobs: server enqueue gate blocks them immediately。
- Queued jobs: server marks denied / cancelled and helper must not execute them。
- Leased jobs before local action: helper revalidates revocation before action and settles as cancelled / revoked。
- Running jobs: helper revalidates before each bounded action boundary; if revoke is observed, it stops at the next safe boundary and records deterministic terminal status。
- Helper auth: persistent helper credential is invalidated; stale-credential poll returns revoked / stale state。
- Uninstall: disables autostart / service and removes or disables in-scope helper / plugin artifacts。
- UI: shows revoked / uninstalled, not offline-only ambiguity。

## 8. Status / logs / audit

UI / API must expose helper online / offline, last seen, allowed job categories, job queued / running / succeeded / failed, failure reason, bounded redacted logs, and revoke / uninstall state。

Logs must not expose tokens, secrets, private message content, private file content, or full environment dumps。Failed jobs must not look successful or spin indefinitely。Local audit records enforcement decisions: schema rejection, policy denial, manifest / artifact verification, path / domain denial, service denial, revoke / stale credential, lease / cancellation outcome。

## 9. Boot / crash / cron stance

Boot + crash restart are must-have for every long-lived process in the Configure OpenClaw value path。For remote-actuator v1, that path is owned by Host Bridge / Helper, not the existing `packages/remote-agent` file-proxy CLI。The current Remote Agent rail remains separate file browsing / reverse-WS infrastructure; it must not inherit helper enrollment credentials or host-management authority。

If a release still ships `packages/remote-agent` as a long-lived user-visible file-proxy process, it needs its own Mac / Linux boot + crash packaging before claiming that rail is reliable。That packaging is separate from Configure OpenClaw remote actuator scope and cannot merge credentials / grants / enforcement rails with Helper。

Boot + crash requirements do not apply to `install-butler`, which is short-lived only。

Fast / slow cron are Teamlead coordination / runtime timer / lease / heartbeat concepts, not product promise。Blueprint should model externally visible behavior as status, last seen, lease expiry, retry / backoff, and heartbeat freshness, not as user-facing cron guarantees。

## 10. Implementation slices

1. Enrollment / status foundation: enrollment record, helper_device_id, owner / org binding, one-time secret exchange, persistent helper credential, last seen, revoke / uninstall state。
2. Outbound pull skeleton: server queue, enqueue gate, long-poll endpoint, lease / ack / result, TTL, retry / backoff, idempotency key, cancellation。
3. Helper pull client: credential rotation, stale-device rejection, long-poll loop, lease handling, result upload, bounded logs。
4. Local policy engine: fixed schemas, manifest / artifact digest checks, allowlists, service ID checks, revoked / wrong-owner / wrong-org / stale-credential denial。
5. Scoped service lifecycle: only enrolled Borgee / OpenClaw service identifiers, boot + crash restart, bounded restart / backoff。
6. Configure OpenClaw closure: install plugin, create / update OpenClaw agent config, configure Borgee plugin connection / channel binding, connected / failed UI states。

## 11. Open decisions / blockers before freeze

- Manifest signing / artifact binding: signing authority, digest scope, cache invalidation, replay handling。
- Helper credential model: token shape, rotation cadence, stale-device semantics, local storage rules。
- Sandbox profile: exact write paths, network domains, service permissions, and Linux AF_UNIX-only outbound poll fix。
- Revoke race rules: exact safe action boundaries and terminal status precedence for running jobs。
- Remote-agent vs helper naming / boundary: user-facing IA may move, but credentials / grants / enforcement rails stay separate。
