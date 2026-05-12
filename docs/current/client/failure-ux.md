# CS-2 故障 UX 分层呈现 (client)

> 出处: `docs/blueprint/current/client-shape.md` §1.3 + `docs/implementation/modules/cs-2-spec.md` v0
> 范围: 仅覆盖 CS-2 客户端故障 UX 分层呈现；不修改 server production code 或 schema。

## 故障三态枚举 (lib/cs2-failure-state.ts)

```ts
export const FAILURE_TRI_STATE = ['online', 'error', 'offline'] as const;
```

该枚举与既有 `<PresenceDot data-presence>` enum + AL-3 守护链保持一致。AL-1b
busy/idle 与 CS-2 故障三态明确区分；只有 BPP progress frame 真正实施时，v2 才增加第 4 态。

`IsFailureState(s)` helper 使用与 reasons.IsValid #496 相同的集中校验模式。

## 6 类故障文案字典 (lib/cs2-failure-labels.ts)

| reason key | label 模板 |
|---|---|
| `api_key_invalid` | `API key 已失效, 需要重新填写` |
| `quota_exceeded` | `{agent_name} 的配额已用完` |
| `network_unreachable` | `{agent_name} 跟 OpenClaw 失联` |
| `runtime_crashed` | `{agent_name} 进程崩溃, 请重启` |
| `runtime_timeout` | `{agent_name} 响应超时` |
| `unknown` | `{agent_name} 出错, 请查日志` |

`formatFailureLabel(reason, agentName)` 替换 `{agent_name}` 占位符。label 字面必须与 reasons.IsValid #496 + AL-4 #321 保持一致。

| 改动点 | 必须同步 |
|---|---|
| failure reason / label 字面变化 | server reasons.go + client cs2-failure-labels.ts + content-lock §1 |

## 4 层 UX 呈现 (与蓝图 §1.3 表保持一致)

| 层 | 组件 | DOM 出处 | 触发 |
|---|---|---|---|
| 头像角标 | `PresenceDot` (扩 `data-failure-badge="true"`) | `data-presence="error"` + `data-failure-badge="true"` | `state==='error'` 自动 |
| 浮层 | `FailurePopover.tsx` | `data-cs2-failure-popover="open"` + `role="dialog"` | hover/click PresenceDot (caller 控制 `open` prop) |
| banner | `FailureBanner.tsx` | `data-cs2-failure-banner="visible"` + `role="alert"` | ≥2 agents 全 failed OR 核心 agent > 5min (`CORE_AGENT_FAILURE_THRESHOLD_MS = 5 * 60 * 1000`) |
| 故障中心 | `FailureCenter.tsx` | `data-cs2-failure-center-toggle` + `data-cs2-failure-center-list` | ≥2 故障 agent (单 agent 走浮层) |

## 页面内修复占位实现 (lib/use_failure_repair.ts)

```ts
export type FailureRepairAction = 'reconnect' | 'refill_api_key' | 'view_logs';
```

3 个 action 当前为占位实现。v0 stub 返回 `status: 'pending'` + 占位 message；v1 接入真实路径。

| action | v1 接入路径 |
|---|---|
| `reconnect` | BPP-3 force-reconnect frame |
| `refill_api_key` | AL-2a config update PATCH |
| `view_logs` | plugin SDK log stream |

蓝图字面要求为 "inline 修复, 不跳设置页"。grep 检查 `navigate.*\/settings` 在
`components/Failure*.tsx` count==0。

## 禁止行为 / QA 检查

| 约束 | 检查 |
|---|---|
| 三态与 busy/idle/standby 状态保持独立 | `'busy'|'idle'|'standby'` 在 `cs-2-*` 无匹配 |
| 不新增第 5 层故障 UI | `toast.*failure|FailureModal|FailureInlineError` 无匹配 |
| 禁止引入未锁定的故障文案 | `故障了|挂了|不可用|服务异常|崩了|掉线` 无匹配 |
| 不暴露原始错误码 | `401 Unauthorized|connection refused|invalid_token|openclaw://` 无匹配 |
| 不提供管理端故障 UX 入口 (ADM-0 §1.3 红线) | `admin.*failure-ux|admin.*FailureCenter` 无匹配 |
| 不修改 server production code | `git diff origin/main -- packages/server-go/` 0 行 |
| 不修改 schema | `migrations/cs_2|cs2.*api|cs2.*server` 无匹配 |

## 跨模块一致性要求

| 来源 | 锁定点 |
|---|---|
| AL-3 PresenceDot data-presence enum | CS-2 三态保持一致 |
| AL-1b 5-state 拆分 | CS-2 三态与 AL-1b 5-state 保持独立；BPP progress 真正实施时 v2 再合并 |
| reasons.IsValid #496 集中 6 类 reason 字典 | reason 或 label 字面变化时同步更新三处 |
| AL-4 #321 system DM 文案锁定 | reason text 保持一致 |
| 蓝图 client-shape.md §1.3 | 故障文案字面对账 |
| ADM-0 §1.3 | 不提供管理端故障 UX 入口 |

## 不在范围

- 第 4 态 busy/idle；由 AL-1b §2.3 BPP progress frame 覆盖
- 页面内修复真实路径；由 plugin SDK + AL-2a / HB-3 覆盖
- IndexedDB 乐观缓存；由 CS-4 覆盖
- Tauri 壳 / PWA install / Web Push；由 HB-2 / CS-3 覆盖
- 管理端故障 UX；管理端 / 管理特权路由不得暴露或挂载该 UX，见 ADM-0 §1.3
- 桌面通知 / 故障声音；由 DL-4 覆盖
