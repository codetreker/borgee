# CS-2 spec brief — 故障三态 + 四层 UX 呈现 (≤80 行)

> 飞马 + 战马D · 2026-04-30 · Phase 4+ Client Shape 第二段 (蓝图 client-shape.md §1.3)
> **蓝图锚**: [`client-shape.md`](../../blueprint/current/client-shape.md) §1.3 (故障 UX 分层呈现 + 三态枚举 + plain language + inline 修复) + §0 (Web SPA 协作主战场)
> **关联**: AL-1b #462 5-state 头像状态色环 (busy/idle 跟故障三态拆分立场承袭) + AL-3 #305 PresenceDot + AL-4 #417 runtime status (`error` reason 6-dict byte-identical) + RT-4 #562 ChannelPresenceList (顶部团队栏数据源) + reasons.IsValid #496 SSOT 包 (跟 AL-1a 6-reason byte-identical 锁链)
> **命名**: CS-2 = Client Shape 第二段 — 故障 UX 分层呈现 (CS-3 留: PWA install + Web Push, CS-4 留: IndexedDB 乐观缓存 — 跟 CS-1 spec §3 留账 byte-identical)

> ⚠️ Wrapper milestone — 复用 AL-1b PresenceDot + AL-4 reason 字典 + reasons.IsValid SSOT, 仅落
> client-only **故障三态枚举 + 四层 UX 呈现 (角标/浮层/banner/故障中心) + plain language 文案映射 + inline 修复 hook 占位**.
> **0 server prod + 0 schema 改 + 0 新 endpoint** — 真符合三选项决策树选项 C (跟 CV-9..14 / DM-5..6 / CS-1 同精神).

## 0. 关键约束 (3 条立场)

1. **故障三态枚举 byte-identical 跟蓝图 §1.3 表格** (野马 push back 三态收敛锁): `online` (runtime 已连接) / `failed` (API key 失效 / 超限 / 进程崩溃 / 网络断) / `offline` (disable / 用户主动关); 反约束: 不允许第 4 态 `busy` / `idle` 漂入此枚举 (那是 AL-1b §2.3 BPP progress frame 的活, CS-2 三态拆死锁); `lib/cs2-failure-state.ts::FAILURE_TRI_STATE = ['online', 'error', 'offline'] as const` SSOT 单源 + `IsFailureState(s)` helper (跟 reasons.IsValid #496 同模式); 反向 grep `'busy'\|'idle'\|'standby'` 在 `cs-2-*` 0 hit (跟 AL-1b 立场承袭, 拆死锁).

2. **四层 UX 呈现 byte-identical 跟蓝图 §1.3 表** (4 层不漂): `头像角标` (`<PresenceDot variant="failure">` 红点 — 复用 AL-1b PresenceDot, 仅加 variant prop) / `点头像 → 浮层` (`FailurePopover.tsx` 显示 reason + 3 inline 修复按钮 `重连` / `重填 API key` / `查日志`) / `顶部 banner` (`FailureBanner.tsx` 全屏宽 — 触发 "全部故障" or "核心 agent 故障 > 5min", 阈值 const `CORE_AGENT_FAILURE_THRESHOLD_MS = 5 * 60 * 1000`) / `故障中心` (`FailureCenter.tsx` 团队栏按钮聚合 — 多 agent 故障 ≥2 时展开). 反约束: 4 层渲染 byte-identical 跟蓝图 ASCII (不另起 toast / modal / inline-error 5 层); 反向 grep `toast.*failure\|FailureModal\|FailureInlineError` count==0; 不允许浮层直接跳设置页 (蓝图 "inline 修复, 不跳设置页" 字面承袭).

3. **plain language 错误文案 byte-identical + 0 server / 0 schema** (跟 host-bridge §1.3 同源映射): `lib/cs2-failure-labels.ts::FAILURE_REASON_LABELS` 字面映射 (跟 AL-4 reason 6-dict + reasons.IsValid #496 SSOT byte-identical, 改 = 改两处 + content-lock 第三处) — `api_key_invalid` → "API key 已失效, 需要重新填写" / `quota_exceeded` → "配额已用完" / `network_unreachable` → "{agent_name} 跟 OpenClaw 失联" / `runtime_crashed` → "{agent_name} 进程崩溃, 请重启" / `runtime_timeout` → "{agent_name} 响应超时" / `unknown` → "{agent_name} 出错, 请查日志"; 反约束: server diff 0 行 (`git diff origin/main -- packages/server-go/` count==0); 不引入 cs_2 命名 server file (反向 grep `migrations/cs_2\|cs2.*api\|cs2.*server` count==0); 文案禁同义词漂 (`故障了` / `挂了` / `不可用` / `服务异常` 0 hit user-visible).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **CS-2.1** 三态 SSOT + plain language 映射 | `packages/client/src/lib/cs2-failure-state.ts` (新, ≤40 行) — `FAILURE_TRI_STATE` const + `IsFailureState(s)` helper + `FailureState` type; `lib/cs2-failure-labels.ts` (新, ≤60 行) — `FAILURE_REASON_LABELS` 6-dict (byte-identical 跟 reasons.IsValid #496) + `formatFailureLabel(reason, agentName)` helper; 8 vitest (TestCS21_TriStateByteIdentical + IsFailureState_TruthTable + FailureLabels_6DictByteIdentical + formatFailureLabel_AgentNamePlaceholder + 4 反向 grep busy/idle/同义词) | 战马D |
| **CS-2.2** 4 层 UX 组件 + inline 修复 hook | `components/FailurePopover.tsx` (新, ≤80 行) — popover hover/click PresenceDot 触发 + 3 inline button (`重连` / `重填 API key` / `查日志`) data-action; `components/FailureBanner.tsx` (新, ≤60 行) — 阈值触发 banner 全屏宽 + 关闭 button; `components/FailureCenter.tsx` (新, ≤80 行) — 团队栏按钮 + ≥2 故障 agent 展开列表; `lib/use_failure_repair.ts` (新, ≤40 行) — repair action handler stub (3 action 占位, hook 真实施留 plugin SDK); `PresenceDot.tsx` (改 ≤5 行) — 加 `variant="failure"` prop 红点; 12 vitest (4 组件各 3 case + repair hook 3 case) | 战马D |
| **CS-2.3** closure | REG-CS2-001..006 + acceptance + content-lock + PROGRESS [x] CS-2 + 4 件套 + docs/current sync (`docs/current/client/failure-ux.md` ≤80 行 — 三态字面 + 4 层 byte-identical + reason labels + inline 修复 3 按钮) + e2e (`packages/e2e/tests/cs-2-failure-ux.spec.ts` 4 case: 角标渲染 / 点头像 → 浮层 + 3 button / banner 阈值触发 / center ≥2 agent 展开) | 战马D / 烈马 |

## 2. 反向 grep 锚 (5 反约束, count==0)

```bash
# 1) 0 server 改 (Wrapper milestone 立场 ③)
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 production lines
# 2) 三态拆死 — busy/idle 不漂入 (跟 AL-1b §2.3 拆死)
git grep -nE "'busy'|'idle'|'standby'" packages/client/src/lib/cs2-failure-*  # 0 hit
# 3) 4 层不漂 — 不另起 5 层 (蓝图 §1.3 byte-identical)
git grep -nE 'toast.*failure|FailureModal|FailureInlineError' packages/client/src/  # 0 hit
# 4) plain language 文案 — 同义词反向
git grep -nE '故障了|挂了|不可用|服务异常' packages/client/src/lib/cs2-failure-labels.ts  # 0 hit
# 5) 不跳设置页 — inline 修复 (蓝图 §1.3 字面)
git grep -nE 'navigate.*\/settings|history\.push.*settings' packages/client/src/components/Failure*.tsx  # 0 hit
```

## 3. 不在范围 (留账)

- ❌ 第 4 态 `busy` / `idle` (留 AL-1b §2.3 BPP progress frame, CS-2 三态拆死)
- ❌ inline 修复真路径 (重连 RPC / 重填 key form / 查日志 page) — 留 plugin SDK 真接入 + AL-2a config update / HB-3 host_grants 真路径
- ❌ IndexedDB 乐观缓存 (蓝图 §1.4) — 留 **CS-4** (跟 CS-1 spec §3 留账 byte-identical)
- ❌ Tauri 壳 + host-bridge daemon — 留 HB-2
- ❌ PWA install + Web Push — 留 CS-3
- ❌ admin god-mode 故障 UX (永久不挂, ADM-0 §1.3 红线 — admin 看 audit 不直接修)
- ❌ 故障声音 / 桌面通知 (留 DL-4 push gateway 接 Web Notifications API)

## 4. 跨 milestone byte-identical 锁

- 复用 AL-1b PresenceDot byte-identical (CS-2 仅加 `variant="failure"` prop ≤5 行)
- reason labels byte-identical 跟 reasons.IsValid #496 SSOT 6-dict (改 = 改两处 + content-lock 第三处)
- 跟 AL-4 #417 runtime status `error` reason 字面 byte-identical
- 跟 host-bridge §1.3 plain language 同根 (错误说人话, 不暴 wire 错码)
- ADM-0 §1.3 admin god-mode 不挂 (CS-2 仅 client 用户视角)
- 0-server-prod 系列模式承袭 (CV-9..14 / DM-5..6 / DM-9 / CHN-11..12 / CS-1 第 14 处)

## 5. 验收挂钩

- REG-CS2-001..006 (5 反向 grep + 三态 SSOT 单测 + 4 层 UX vitest + e2e 4 case)
- 既有 PresenceDot / AL-1b / AL-4 unit tests 全 PASS (Wrapper variant 不破)
- vitest cs2-failure-state.test.ts + cs2-failure-labels.test.ts + 4 组件 .test.tsx + use_failure_repair.test.ts (≥20 case 全闭)
