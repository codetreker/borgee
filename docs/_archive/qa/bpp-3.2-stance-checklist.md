# BPP-3.2 立场反查清单 (战马C v0)

> 战马C · 2026-04-29 · 立场 review checklist (跟 BPP-2 #460 + AP-1 #493 同模式)
> **目的**: BPP-3.2 三段实施 (3.2.1 server DM dispatch / 3.2.2 owner UI / 3.2.3 plugin retry) PR review 时, 飞马 / 野马 / 烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/bpp-3.2-spec.md` (战马C v0, c8e37a4) + acceptance `docs/qa/acceptance-templates/bpp-3.2.md` (战马C v0) + 文案锁 `docs/qa/bpp-3.2-content-lock.md` (战马C v0) + 蓝图 `auth-permissions.md` §1.3 主入口 + §2 不变量 + §4 BPP frame; 复用 BPP-3.1 #494 PermissionDeniedFrame + AP-1 #493 abac.go::Capabilities + DM-2 #372 message_mentions

## §0 立场总表 (3 立场 + 4 边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | plugin → owner DM 走 DM-2 既有路径 | auth-permissions.md §1.3 主入口 + plugin-protocol.md §2.1 | DM 走 `messages` + `message_mentions` + `quick_action` JSON, 不开新 channel 类型 / 不写新 system_message_kind enum |
| ② | owner 一键 grant 走 AP-1 const 白名单 | auth-permissions.md §1 ABAC + AP-1 #493 capabilities.go | `Store.GrantPermission` 既有 API, `Permission` 必走 `auth.Capabilities` 14 项 const, 反向 grep hardcode 字面 count==0 |
| ③ | plugin 自动重试 ≤3 次 30s 固定退避 | plugin-protocol.md §1.6 + BPP-4 watchdog 拆三路径 | plugin SDK in-memory `RequestRetryCache` (TTL 5min), 30s 固定退避不指数, ≤3 次, `bpp.retry_exhausted` 错码 |
| ④ (边界) | BPP-2.1 7→8 op 加 `request_capability_grant` | plugin-protocol.md §1.3 + BPP-2 #485 ValidSemanticOps | `internal/bpp/dispatcher.go::ValidSemanticOps` map 加 1 行 byte-identical 跟蓝图 + 4 件套同步 (改 = 改五处+) |
| ⑤ (边界) | bundle 字面不入 server | auth-permissions.md §1.1 (UI 糖) | server 端只看 capability list, 反向 grep `"bundle":\|bundle_id` 在 internal/ count==0 |
| ⑥ (边界) | admin god-mode 不入此路径 | ADM-0.1 §1.3 红线 | admin grant 走 /admin-api 单独 mw, 不走 owner DM; 反向 grep `admin.*permission_denied\|admin.*request_capability_grant` count==0 |
| ⑦ (边界) | scope 不漂出 v1 三层 | auth-permissions.md §1.2 三层 scope | grant 入参 `Scope` ∈ `{*, channel:<id>, artifact:<id>}`, 反向 grep `Scope.*"workspace:\|Scope.*"org:` count==0 |
| ⑧ (边界) | 不另起 `capability_granted` BPP frame | auth-permissions.md §4.2 (v1 不做) | grant 后走 BPP-2.3 `agent_config_update` (AL-2b #481), 反向 grep `FrameTypeBPPCapabilityGranted\|"capability_granted"` count==0 |

## §1 立场 ① plugin → owner DM 走 DM-2 既有路径 (BPP-3.2.1 守)

**蓝图字面源**: `auth-permissions.md` §1.3 "动态请求" → "server 给 owner 写一条 system message 到内置 DM" + 字面文案 `"agent 想 create_artifact 但缺权限 workspace.create"` + 三 quick_action 按钮

**反约束清单**:

- [ ] BPP-2.1 `ValidSemanticOps` 加 `request_capability_grant` (7→8) byte-identical 跟蓝图 §1.3 字面承袭, 不另起命名 (反向 grep `request_grant\|capability_request_op` count==0)
- [ ] handler 调 `Store.SendSystemDM(ownerID, body, quickActionJSON)` 既有 helper (复用 CM-onboarding #203 system DM 路径), 不写新表 / 不开新 endpoint
- [ ] `quick_action` JSON shape `{action: 'grant'|'reject'|'snooze', agent_id, capability, scope, request_id}` byte-identical 跟 CM-onboarding 既有 schema (反向 grep 新 system_message_kind enum count==0)
- [ ] 文案 byte-identical 跟蓝图 §1.3 + 野马签字 (见 content-lock §1)

## §2 立场 ② owner 一键 grant 走 AP-1 const 白名单 (BPP-3.2.2 守)

**蓝图字面源**: `auth-permissions.md` §1 ABAC + AP-1 #493 实施 `internal/auth/capabilities.go` 14 项 const + §1.4 跨 org 只能减权 owner-only

**反约束清单**:

- [ ] 新 endpoint `POST /api/v1/me/grants {agent_id, capability, scope}` body 校验: `capability` 必 ∈ `auth.Capabilities` (枚举外 reject + log warn `bpp.grant_capability_disallowed`); 反向 grep `GrantPermission.*Permission:.*"[a-z_]+"` 在 internal/api/ count==0
- [ ] `scope` byte-identical 跟 BPP-3.1 frame `current_scope` 字段, ∈ `{*, channel:<id>, artifact:<id>}` (反约束 ⑦)
- [ ] grant 后调 `Hub.PushAgentConfigUpdate` (AL-2b #481) 触发 plugin 重读权限, **不**另起 `capability_granted` 新 BPP frame (反约束 ⑧)
- [ ] owner-only ACL: 仅 grant 给自己 owned 的 agent (`agent.OwnerID == user.ID`); 反向 grep `admin.*\/me\/grants` count==0 (反约束 ⑥)
- [ ] reject / snooze 路径仅 dismiss DM (不持久化 deny list, v1 不做 — spec §2 留账)

## §3 立场 ③ plugin 自动重试 ≤3 次 30s 固定退避 (BPP-3.2.3 守)

**蓝图字面源**: `plugin-protocol.md` §1.6 失联与故障状态 + BPP-4 watchdog spec (拆三路径)

**反约束清单**:

- [ ] plugin SDK `RequestRetryCache` 类型: `map[requestID]*SemanticActionFrame` + TTL 5min (反约束: 不持久化, 不复用 BPP-4 server-side watchdog 队列)
- [ ] 退避策略: 30s **固定**, 反向 grep `expBackoff\|exponential.*retry` 在 packages/plugin-sdk/ count==0 (蓝图 §1.6 字面 server-side timing 单源, plugin 端不增添新 timing 信号)
- [ ] 上限: ≤3 次重试 (const `MaxPermissionRetries = 3`, 反向 grep `MaxPermissionRetries.*[4-9]` count==0)
- [ ] 超限 abort: log warn `bpp.retry_exhausted` 错码 (跟 BPP-2.2 `bpp.task_subject_empty` + BPP-2.3 `bpp.config_field_disallowed` 命名同模式)
- [ ] retry trigger: 仅 `agent_config_update` frame 触发 cache 扫 (跟立场 ②⑧ 复用既有 frame, 不另起 `capability_granted`)

## §4 联签清单 (实施 PR 时填)

- [ ] 飞马 (spec ↔ 立场对齐): _(签)_
- [ ] 野马 (文案锁 ↔ DM body + quick_action labels byte-identical): _(签)_
- [ ] 烈马 (反向 grep + 单测覆盖率 ≥85% + 8 反约束全 count==0): _(签)_
- [ ] 战马C (实施代码 ↔ 立场反查 8 项全过): _(签)_
