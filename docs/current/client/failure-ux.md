# CS-2 layered failure UX (client)

> Source: `docs/blueprint/current/client-shape.md` §1.3 + `docs/implementation/modules/cs-2-spec.md` v0
> Scope: only covers CS-2 client layered failure UX; does not modify server production code or schema.

## Failure Three-State Enum (lib/cs2-failure-state.ts)

```ts
export const FAILURE_TRI_STATE = ['online', 'error', 'offline'] as const;
```

This enum stays aligned with the existing `<PresenceDot data-presence>` enum and AL-3 guard chain. AL-1b
busy/idle stays separate from the CS-2 failure three-state model; v2 only adds a fourth state when the BPP progress frame is actually implemented.

`IsFailureState(s)` helper uses the same centralized validation pattern as reasons.IsValid #496.

## 6-Class Failure Copy Dictionary (lib/cs2-failure-labels.ts)

| reason key | label template |
|---|---|
| `api_key_invalid` | `API key 已失效, 需要重新填写` |
| `quota_exceeded` | `{agent_name} 的配额已用完` |
| `network_unreachable` | `{agent_name} 跟 OpenClaw 失联` |
| `runtime_crashed` | `{agent_name} 进程崩溃, 请重启` |
| `runtime_timeout` | `{agent_name} 响应超时` |
| `unknown` | `{agent_name} 出错, 请查日志` |

`formatFailureLabel(reason, agentName)` replaces the `{agent_name}` placeholder. Label literals must stay aligned with reasons.IsValid #496 + AL-4 #321.

| Change point | Must synchronize |
|---|---|
| failure reason / label literal changes | server reasons.go + client cs2-failure-labels.ts + content-lock §1 |

## 4-Layer UX Presentation (aligned with blueprint §1.3 table)

| Layer | Component | DOM source | Trigger |
|---|---|---|---|
| avatar badge | `PresenceDot` (adds `data-failure-badge="true"`) | `data-presence="error"` + `data-failure-badge="true"` | Automatic when `state==='error'` |
| popover | `FailurePopover.tsx` | `data-cs2-failure-popover="open"` + `role="dialog"` | hover/click PresenceDot (caller controls `open` prop) |
| banner | `FailureBanner.tsx` | `data-cs2-failure-banner="visible"` + `role="alert"` | ≥2 agents all failed OR core agent > 5min (`CORE_AGENT_FAILURE_THRESHOLD_MS = 5 * 60 * 1000`) |
| failure center | `FailureCenter.tsx` | `data-cs2-failure-center-toggle` + `data-cs2-failure-center-list` | ≥2 failed agents (single agent uses the popover) |

## In-Page Repair Placeholder Implementation (lib/use_failure_repair.ts)

```ts
export type FailureRepairAction = 'reconnect' | 'refill_api_key' | 'view_logs';
```

The three actions are currently placeholders. The v0 placeholder implementation returns `status: 'pending'` plus a placeholder message; v1 connects them to the real paths.

| action | v1 integration path |
|---|---|
| `reconnect` | BPP-3 force-reconnect frame |
| `refill_api_key` | AL-2a config update PATCH |
| `view_logs` | plugin SDK log stream |

The blueprint literal requirement is "inline 修复, 不跳设置页". Grep check `navigate.*\/settings` in
`components/Failure*.tsx` count==0.

## Prohibited Behavior / QA Checks

| Constraint | Check |
|---|---|
| Three-state model stays separate from busy/idle/standby states | `'busy'|'idle'|'standby'` has no matches in `cs-2-*` |
| Do not add a fifth failure UI layer | `toast.*failure|FailureModal|FailureInlineError` has no matches |
| Do not introduce unlocked failure copy | `故障了|挂了|不可用|服务异常|崩了|掉线` has no matches |
| Do not expose raw error codes | `401 Unauthorized|connection refused|invalid_token|openclaw://` has no matches |
| Do not provide an admin failure UX entry point (ADM-0 §1.3) | `admin.*failure-ux|admin.*FailureCenter` has no matches |
| Do not modify server production code | `git diff origin/main -- packages/server-go/` has 0 lines |
| Do not modify schema | `migrations/cs_2|cs2.*api|cs2.*server` has no matches |

## Cross-Module Consistency Requirements

| Source | Lock point |
|---|---|
| AL-3 PresenceDot data-presence enum | CS-2 three-state model stays aligned |
| AL-1b 5-state split | CS-2 three-state model stays separate from AL-1b 5-state model; only merge in v2 when BPP progress is actually implemented |
| reasons.IsValid #496 centralized 6-class reason dictionary | Update all three places when reason or label literals change |
| AL-4 #321 system DM copy lock | Reason text stays aligned |
| blueprint client-shape.md §1.3 | Failure copy reconciliation |
| ADM-0 §1.3 | Do not provide an admin failure UX entry point |

## Out of Scope

- fourth state busy/idle; covered by AL-1b §2.3 BPP progress frame
- real in-page repair paths; covered by plugin SDK + AL-2a / HB-3
- IndexedDB optimistic cache; covered by CS-4
- Tauri shell / PWA install / Web Push; covered by HB-2 / CS-3
- admin failure UX; admin / privileged admin routes must not expose or mount this UX, see ADM-0 §1.3
- desktop notifications / failure sound; covered by DL-4
