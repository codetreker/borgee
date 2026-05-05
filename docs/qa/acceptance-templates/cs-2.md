# Acceptance Template — CS-2: 故障三态 + 四层 UX 呈现

> Spec: `docs/implementation/modules/cs-2-spec.md` (飞马 + 战马D v0)
> 蓝图: `docs/blueprint/current/client-shape.md` §1.3 (故障 UX 分层呈现 + 三态枚举 + plain language + inline 修复)
> Stance: `docs/qa/cs-2-stance-checklist.md` (野马 / 飞马 v0)
> 前置: AL-1b #462 PresenceDot ✅ + AL-4 #417 runtime status reason ✅ + reasons.IsValid #496 SSOT 包 ✅ + RT-4 #562 ChannelPresenceList ✅
> Owner: 战马D (主战) + 飞马 (spec) + 烈马 (acceptance) + 野马 (4 层 UX 文案)

## 验收清单

### 立场 ① — 故障三态 byte-identical (online/failed/offline 拆死)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `FAILURE_TRI_STATE = ['online', 'error', 'offline'] as const` 落 `lib/cs2-failure-state.ts` byte-identical 跟蓝图 §1.3 表 | unit | 战马D | `cs2-failure-state.test.ts::TestCS21_TriStateByteIdentical` |
| 1.2 `IsFailureState(s)` helper 单源 (跟 reasons.IsValid #496 同模式), 3 true + 5 false truth table | unit | 战马D | `TestCS21_IsFailureState_TruthTable` |
| 1.3 反向断 busy/idle/standby 不漂入 (反向 grep `'busy'\|'idle'\|'standby'` 在 cs-2-* 0 hit) | unit | 战马D | `TestCS21_NoBusyIdleStandbyDrift` (filepath.Walk + regex) |

### 立场 ② — 4 层 UX 呈现 byte-identical (角标/浮层/banner/中心 + inline 修复)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `<PresenceDot variant="failure">` 红点角标 (复用 AL-1b PresenceDot ≤5 行加 prop) | vitest | 战马D | `PresenceDot.test.tsx::TestCS22_FailureVariantRed` |
| 2.2 `FailurePopover.tsx` 浮层 — 触发: hover/click PresenceDot; 内容: reason text + 3 inline button (`重连` / `重填 API key` / `查日志`) byte-identical 跟蓝图字面 | vitest | 战马D | `FailurePopover.test.tsx` 3 case (PopoverOpensOnClick + 3ButtonsLiteralByteIdentical + ReasonTextRendered) |
| 2.3 `FailureBanner.tsx` 全屏宽 — 阈值触发 ("全部故障" or "核心 agent > 5min" const `CORE_AGENT_FAILURE_THRESHOLD_MS = 5*60*1000`) + 关闭 button | vitest | 战马D | `FailureBanner.test.tsx` 3 case (AllAgentsFailureTriggers + CoreAgent5MinTriggers + DismissButton) |
| 2.4 `FailureCenter.tsx` 团队栏按钮 — ≥2 故障 agent 展开列表 + 单 agent 时 0 渲染 | vitest | 战马D | `FailureCenter.test.tsx` 3 case (ButtonRendered + ExpandsOnClick + ZeroOnSingleFailure) |
| 2.5 `lib/use_failure_repair.ts` repair action handler stub — 3 action 占位 (reconnect / refillKey / viewLogs), hook 真路径留 plugin SDK | unit | 战马D | `use_failure_repair.test.ts` 3 case (3 action handler stub return + 反向不跳 settings) |
| 2.6 反向断 5 层不漂 (`toast.*failure\|FailureModal\|FailureInlineError` count==0) + inline 修复不跳设置页 (`navigate.*\/settings` 在 Failure*.tsx count==0) | unit | 战马D | `TestCS22_NoFifthLayer` + `TestCS22_NoNavigateToSettings` |

### 立场 ③ — plain language 6-dict + 0-server-prod + 0 schema

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 `FAILURE_REASON_LABELS` 6-dict byte-identical 跟 reasons.IsValid #496 + AL-4 #417 (api_key_invalid / quota_exceeded / network_unreachable / runtime_crashed / runtime_timeout / unknown) | unit | 战马D | `cs2-failure-labels.test.ts::TestCS21_FailureLabels_6DictByteIdentical` |
| 3.2 `formatFailureLabel(reason, agentName)` 占位符替换 — `"{agent_name} 跟 OpenClaw 失联"` byte-identical 跟蓝图字面 | unit | 战马D | `TestCS21_formatFailureLabel_AgentNamePlaceholder` |
| 3.3 反向断同义词漂 (`故障了` / `挂了` / `不可用` / `服务异常` 在 cs2-failure-labels.ts count==0) + plain language 测 (raw error code byte-identical 不暴) | unit | 战马D | `TestCS21_NoSynonymDrift` |
| 3.4 0 server 改 (`git diff origin/main -- packages/server-go/` count==0 production lines) | unit | 战马D | `cs2_no_server_diff_test.ts` (filepath.Walk server-go/) |
| 3.5 0 schema 改 (反向 grep `migrations/cs_2\|cs2.*api\|cs2.*server` 在 server-go/internal/ count==0) | unit | 战马D | `TestCS21_NoSchemaChange` |
| 3.6 admin god-mode 不挂 (ADM-0 §1.3 红线 — 反向 grep `admin.*failure-ux\|admin.*FailureCenter` count==0) | unit | 战马D | `TestCS22_NoAdminFailureUX` |

### 既有 AL-1b PresenceDot / AL-4 / reasons.IsValid 不破

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 PresenceDot 既有 5-state 渲染 byte-identical 不破 (variant prop 仅扩, 不替) | vitest | 战马D | 既有 PresenceDot.test.tsx 全 PASS + 新加 variant=failure 不影响其他 variant |
| 4.2 reasons.IsValid #496 SSOT 6-dict 字面 byte-identical 不动 (本 PR 仅 client labels 引用) | unit | 战马D | git diff `packages/server-go/internal/agent/reasons/` 0 行 |

### e2e (cs-2-failure-ux.spec.ts 4 case)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 5.1 角标渲染 — 单 agent failed → PresenceDot variant=failure 红点 DOM | playwright | 战马D / 烈马 | `cs-2-failure-ux.spec.ts::TestCS2E2E_PresenceDotFailureBadge` |
| 5.2 点头像 → 浮层 — 3 inline button 文案 byte-identical 跟蓝图 (`重连` / `重填 API key` / `查日志`) | playwright | 战马D / 烈马 | `TestCS2E2E_PopoverWithThreeButtons` |
| 5.3 banner 阈值触发 — 全 agent 故障 OR 核心 agent > 5min → FailureBanner DOM | playwright | 战马D / 烈马 | `TestCS2E2E_BannerThresholdTriggered` |
| 5.4 故障中心 — ≥2 故障 agent → 团队栏按钮可点击 + 展开列表 | playwright | 战马D / 烈马 | `TestCS2E2E_FailureCenterExpands` |

## 不在本轮范围 (spec §3 字面承袭)

- ❌ 第 4 态 busy/idle (留 AL-1b §2.3 BPP progress frame)
- ❌ inline 修复真路径 (留 plugin SDK + AL-2a / HB-3)
- ❌ IndexedDB 乐观缓存 (留 CS-4)
- ❌ Tauri / PWA install / Web Push (留 HB-2 / CS-3)
- ❌ admin god-mode 故障 UX (永久不挂 ADM-0 §1.3)
- ❌ 桌面通知 / 故障声音 (留 DL-4)

## 退出条件

- 立场 ① 1.1-1.3 (三态 byte-identical + helper + 反向断 busy/idle) ✅
- 立场 ② 2.1-2.6 (4 层 UX + repair hook + 反向断 5 层 + inline 不跳设置) ✅
- 立场 ③ 3.1-3.6 (plain language 6-dict + 0 server / 0 schema + admin god-mode 不挂) ✅
- 既有 4.1-4.2 (PresenceDot 不破 + reasons.IsValid 字面不动) ✅
- e2e 5.1-5.4 全 PASS ✅
- REG-CS2-001..006 = **6 行 🟢**

## 更新日志

- 2026-04-30 — 战马D / 飞马 / 烈马 / 野马 v0: CS-2 4 件套 acceptance template, 跟 spec 3 立场 + stance §2 黑名单 grep + 跨 milestone byte-identical (AL-1b PresenceDot variant + reasons.IsValid #496 6-dict + AL-4 #417 + ADM-0 §1.3 红线) 三段对齐. 0 server prod + 0 schema 改 — wrapper milestone 选项 C 同 CS-1 / CV-9..14 / DM-5..6 / DM-9 模式承袭.
