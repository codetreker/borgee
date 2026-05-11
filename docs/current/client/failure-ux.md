# CS-2 故障 UX 分层呈现 (client)

> 出处: `docs/blueprint/current/client-shape.md` §1.3 + `docs/implementation/modules/cs-2-spec.md` v0
> 落点: 战马D + 飞马 + 烈马 + 野马 (一个 milestone 一个 PR, 0 server prod + 0 schema)

## 故障三态枚举 (lib/cs2-failure-state.ts)

```ts
export const FAILURE_TRI_STATE = ['online', 'error', 'offline'] as const;
```

该枚举与既有 `<PresenceDot data-presence>` enum + AL-3 守护链 byte-identical。AL-1b
busy/idle 与 CS-2 故障三态真分清；只有 BPP progress frame 真实施时，v2 才增加第 4 态。

`IsFailureState(s)` helper 使用与 reasons.IsValid #496 相同的单一来源模式。

## plain language 6-dict (lib/cs2-failure-labels.ts)

| reason key | label 模板 byte-identical |
|---|---|
| `api_key_invalid` | `API key 已失效, 需要重新填写` |
| `quota_exceeded` | `{agent_name} 的配额已用完` |
| `network_unreachable` | `{agent_name} 跟 OpenClaw 失联` |
| `runtime_crashed` | `{agent_name} 进程崩溃, 请重启` |
| `runtime_timeout` | `{agent_name} 响应超时` |
| `unknown` | `{agent_name} 出错, 请查日志` |

`formatFailureLabel(reason, agentName)` 替换 `{agent_name}` 占位符。label 字面与 reasons.IsValid #496 + AL-4 #321 byte-identical。

| 改动点 | 必须同步 |
|---|---|
| failure reason / label 字面变化 | server reasons.go + client cs2-failure-labels.ts + content-lock §1 |

## 4 层 UX 呈现 (蓝图 §1.3 表 byte-identical)

| 层 | 组件 | DOM 出处 | 触发 |
|---|---|---|---|
| 头像角标 | `PresenceDot` (扩 `data-failure-badge="true"`) | `data-presence="error"` + `data-failure-badge="true"` | `state==='error'` 自动 |
| 浮层 | `FailurePopover.tsx` | `data-cs2-failure-popover="open"` + `role="dialog"` | hover/click PresenceDot (caller 控制 `open` prop) |
| banner | `FailureBanner.tsx` | `data-cs2-failure-banner="visible"` + `role="alert"` | ≥2 agents 全 failed OR 核心 agent > 5min (`CORE_AGENT_FAILURE_THRESHOLD_MS = 5 * 60 * 1000`) |
| 故障中心 | `FailureCenter.tsx` | `data-cs2-failure-center-toggle` + `data-cs2-failure-center-list` | ≥2 故障 agent (单 agent 走浮层) |

## inline 修复 stub (lib/use_failure_repair.ts)

```ts
export type FailureRepairAction = 'reconnect' | 'refill_api_key' | 'view_logs';
```

3 个 action 当前为占位。v0 stub 返回 `status: 'pending'` + 占位 message；v1 接入真实路径。

| action | v1 接入路径 |
|---|---|
| `reconnect` | BPP-3 force-reconnect frame |
| `refill_api_key` | AL-2a config update PATCH |
| `view_logs` | plugin SDK log stream |

蓝图字面要求为 "inline 修复, 不跳设置页"。grep 检查 `navigate.*\/settings` 在
`components/Failure*.tsx` count==0。

## 反向约束 / QA 守门

| 约束 | 检查 |
|---|---|
| 三态真分清 | `'busy'|'idle'|'standby'` 在 `cs-2-*` 0 hit |
| 4 层不脱节第 5 层 | `toast.*failure|FailureModal|FailureInlineError` 0 hit |
| 同义词脱节禁 | `故障了|挂了|不可用|服务异常|崩了|掉线` 0 hit |
| raw error code 不暴露 | `401 Unauthorized|connection refused|invalid_token|openclaw://` 0 hit |
| admin god-mode 不挂 (ADM-0 §1.3 红线) | `admin.*failure-ux|admin.*FailureCenter` 0 hit |
| 0 server prod | `git diff origin/main -- packages/server-go/` 0 行 |
| 0 schema 改 | `migrations/cs_2|cs2.*api|cs2.*server` 0 hit |

## 跨 milestone byte-identical 锁定

| 来源 | 锁定点 |
|---|---|
| AL-3 PresenceDot data-presence enum | CS-2 三态 byte-identical |
| AL-1b 5-state 拆分 | CS-2 三态真分清 vs AL-1b 5-state；BPP progress 真实施时 v2 合 |
| reasons.IsValid #496 单一来源 6-dict | 改 = 改三处 |
| AL-4 #321 system DM 文案锁定 | reason text byte-identical |
| 蓝图 client-shape.md §1.3 | plain language 字面对账 |
| ADM-0 §1.3 | admin god-mode 不挂 |

## 不在范围

- 第 4 态 busy/idle；留 AL-1b §2.3 BPP progress frame
- inline 修复真实路径；留 plugin SDK + AL-2a / HB-3
- IndexedDB 乐观缓存；留 CS-4
- Tauri 壳 / PWA install / Web Push；留 HB-2 / CS-3
- admin god-mode 故障 UX；永久不挂 ADM-0 §1.3
- 桌面通知 / 故障声音；留 DL-4
