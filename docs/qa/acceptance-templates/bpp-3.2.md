# Acceptance Template — BPP-3.2: `permission_denied` plugin UX 流 (owner DM + 一键 grant + plugin retry)

> Spec: `docs/implementation/modules/bpp-3.2-spec.md` (战马C v0, c8e37a4)
> 蓝图: `docs/blueprint/current/auth-permissions.md` §1.3 主入口 (动态请求 → owner DM → 一键 grant) + §2 不变量 + §4 BPP frame · `plugin-protocol.md` §1.5 (`agent_config_update` 触发 plugin reload)
> 前置: BPP-3.1 #494 PermissionDeniedFrame ✅ + AP-1 #493 abac.go + capabilities.go 14 const ✅ + DM-2 #361/#372/#388 message_mentions ✅ + BPP-2 #485 ValidSemanticOps ✅
> Owner: 战马C (主战) + 野马 (文案) + 烈马 (验收)

## 验收清单

### BPP-3.2.1 server DM dispatch (新 `request_capability_grant` semantic op)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `internal/bpp/dispatcher.go::ValidSemanticOps` 加 `request_capability_grant` (7→8) | unit | 战马C / 烈马 | `dispatcher_test.go::TestValidSemanticOps_8Ops` count + 字面锁 |
| 1.2 handler 调 `Store.SendSystemDM(ownerID, body, quickActionJSON)` 既有路径 (复用 CM-onboarding #203) | unit | 战马C / 烈马 | `internal/api/capability_grant_test.go::TestRequestGrant_WritesSystemDM` (DM body + mention 行) |
| 1.3 `quick_action` JSON shape `{action, agent_id, capability, scope, request_id}` byte-identical | unit | 战马C / 野马 | `TestRequestGrant_QuickActionShape` round-trip + content-lock §2 字面锁 |
| 1.4 文案 byte-identical 跟 content-lock §1 (DM body + 三按钮 label) | unit + 反向 grep | 战马C / 野马 | `TestRequestGrant_DMBodyLiteral` (3 fragment toContain) |
| 1.5 反约束 grep: 不开新 channel 类型 / system_message_kind enum / capability_granted 新 frame | reverse grep | 烈马 | `bpp_3_2_grep_test.go::TestNoNewChannelType` + `TestNoSystemMessageKindDrift` + `TestNoCapabilityGrantedFrame` |

### BPP-3.2.2 owner DM UI 一键 grant (SystemMessageBubble 三按钮)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 SystemMessageBubble 渲染 quick_action 三按钮 (跟 CM-onboarding QuickActionButton 同模式) byte-identical 跟 content-lock §3 (label + data-action attr) | vitest | 战马C / 野马 | `packages/client/src/__tests__/SystemMessageBubble.bpp32.test.tsx` (3 button + data-action='grant'\|'reject'\|'snooze') |
| 2.2 新 endpoint `POST /api/v1/me/grants {agent_id, capability, scope}` body 走 `auth.Capabilities` 14 项 const 校验 (枚举外 reject + log warn `bpp.grant_capability_disallowed`) | unit + 反向 grep | 战马C / 烈马 | `internal/api/me_grants_test.go::TestPostGrant_Whitelist` (14 valid + 5 reject) + `TestNoHardcodedGrantPermission` 反向 grep `GrantPermission.*Permission:.*"[a-z_]+"` count==0 |
| 2.3 owner-only ACL: 仅 grant 给自己 owned 的 agent (`agent.OwnerID == user.ID`); admin path 不挂 (反约束 ⑥) | unit | 战马C / 烈马 | `TestPostGrant_NonOwner403` + `TestPostGrant_AdminAPINotMounted` |
| 2.4 grant 后调 `Hub.PushAgentConfigUpdate` (AL-2b #481), 不另起 `capability_granted` BPP frame | unit | 战马C / 烈马 | `TestPostGrant_TriggersAgentConfigUpdate` (interface mock 验证调 1 次, payload 含新 capability) |
| 2.5 reject / snooze 仅 dismiss DM (不持久化反向 grant); v1 不做 deny list | unit | 战马C / 烈马 | `TestPostReject_NoSideEffect` + `TestPostSnooze_NoSideEffect` |

### BPP-3.2.3 plugin 自动重试 + e2e + closure

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 plugin SDK `RequestRetryCache` (`map[requestID]*SemanticActionFrame` + TTL 5min, in-memory; 反约束: 不持久化, 不复用 BPP-4 server-side watchdog) | unit | 战马C / 烈马 | `packages/plugin-sdk/internal/retry_cache_test.go::TestCache_TTL5Min` + `TestCache_NoBPP4Reuse` reverse grep `BPP-?4.*retry` count==0 |
| 3.2 `MaxPermissionRetries = 3` const + 30s 固定退避 (反 expBackoff); 超限 abort + log `bpp.retry_exhausted` | unit | 战马C / 烈马 | `TestCache_3RetryThenExhaust` (1st/2nd/3rd retry pass + 4th abort + log line 字面锁) |
| 3.3 retry trigger: 仅 `agent_config_update` frame 触发 cache 扫 (跟 BPP-2.3 同源) | unit | 战马C / 烈马 | `TestCache_OnlyAgentConfigUpdateTriggers` (5 frame type 路径, 仅 agent_config_update 触发 retry, 其它 4 类 no-op) |
| 3.4 e2e 真路径: agent commit_artifact 无权 → owner DM 一键 grant → plugin 自动 retry 成功 | playwright | 烈马 / 野马 | `packages/e2e/tests/bpp-3.2-grant-flow.spec.ts` (full flow ≤ 5s) + screenshot `g4.x-bpp32-grant-flow.png` |
| 3.5 closure — REG-BPP32-001..N 入 registry §3 + §5 总计 sync + PROGRESS [x] BPP-3.2 + docs/current/server/bpp.md + ws/event-schemas.md sync | docs | 战马C / 烈马 | registry §3 + §5 + PROGRESS + 4 件套全闭 |

## 不在本轮范围 (spec §4)

- ABAC v2 condition (time-based / IP-based) — v1 仅 (user_id, permission, scope) 三元组
- multi-owner grant (v3+)
- grant 历史 audit UI (admin SPA, 走 ADM-2 既有 audit)
- reject 持久化 deny list (v2+)
- 跨语言 plugin SDK retry cache 实现 (本 milestone 仅 reference plugin)

## 退出条件

- BPP-3.2.1 1.1-1.5 (server DM dispatch + 5 反约束) ✅
- BPP-3.2.2 2.1-2.5 (owner UI + AP-1 const 校验 + interface seam) ✅
- BPP-3.2.3 3.1-3.5 (plugin retry + e2e + closure) ✅
- 现网回归不破: 全套 server + client + e2e 测试套全 PASS
- REG-BPP32-001..N 落 registry + 8 反约束 grep 全 count==0
- 4 件套全闭 (spec ✅ + stance ✅ + acceptance ✅ + content-lock ✅)

## 更新日志

- 2026-04-29 — 战马C v0 acceptance template (4 件套第二件): 3 段实施 (1.1-1.5 / 2.1-2.5 / 3.1-3.5) + 5 不在范围 + 退出条件 6 项. 联签 BPP-3.2.1/.2/.3 三 PR 同 branch 叠 commit, AP-1 + BPP-3.1 同模式.
