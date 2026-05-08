# HB-3 文案锁 — 弹窗三按钮 DOM 字面 + 反向同义词禁词 (战马A v0)

> 战马A · 2026-04-29 · ≤40 行 byte-identical 文案锁 (跟 BPP-2 #485 / AL-2b #481 / CHN-2 #354 content-lock 同模式)
> **蓝图锚**: [`host-bridge.md`](../blueprint/current/host-bridge.md) §1.3 弹窗 UX 字面: `[✗ 拒绝]    [✓ 仅这一次]    [✓ 始终允许]`
> **关联**: spec `docs/implementation/modules/hb-3-spec.md` §1 HB-3.3 + acceptance `docs/qa/acceptance-templates/hb-3.md` §3.1 + stance `docs/qa/hb-3-stance-checklist.md` §0 立场 ⑤

## §1 字面锁 (改 = 改三处: 此文档 + spec §1 HB-3.3 + 实施代码 HostGrantsPanel.tsx)

### ① 弹窗三按钮 DOM byte-identical (蓝图 §1.3 字面)

```tsx
<button data-action="deny"            data-hb3-button="danger">  拒绝       </button>
<button data-action="grant_one_shot"  data-hb3-button="primary"> 仅这一次   </button>
<button data-action="grant_always"    data-hb3-button="primary"> 始终允许   </button>
```

**改 = 改三处**: 此文档 + `spec brief §1 HB-3.3` + 实施代码
`packages/client/src/permissions/HostGrantsPanel.tsx`. data-action 字面映射
ttl_kind enum 字面 byte-identical (one_shot ↔ grant_one_shot, always ↔ grant_always); deny 不写 grants 表.

### ② 弹窗 title + body 字面 byte-identical (蓝图 §1.3 弹窗 UX 模板)

```
title: "{agentName} 想{actionLabel}你的{scopeLabel}"
body:  "原因: {agentName} 配置中的「{capabilityLabel}」能力\n      需要{actionLabel}{scopeLabel}"
```

**actionLabel** byte-identical (跟 grant_type 4-enum 字面映射):
- `install` → "安装"
- `exec` → "执行"
- `filesystem` → "读取" (read) / "写入" (write, v2+)
- `network` → "访问"

**改 = 改两处**: 此文档 + 实施代码 HostGrantsPanel 文案表 (跟蓝图 §1.3 同源).

## §2 反约束 — 同义词禁词反向 grep ≥10 (CI lint 守门, 改 = 改单测)

```
git grep -nE '"否决"|"拒绝授权"|"不允许"|"deny\(\)"|"reject"' packages/client/src/permissions/   # 0 hit (拒绝 字面单源)
git grep -nE '"一次"|"单次"|"临时"|"once"|"transient"' packages/client/src/permissions/HostGrantsPanel\.tsx   # 0 hit ("仅这一次" 字面单源)
git grep -nE '"永久"|"长期"|"forever"|"permanent"|"persistent"' packages/client/src/permissions/HostGrantsPanel\.tsx   # 0 hit ("始终允许" 字面单源)
git grep -nE 'data-action="(allow|approve|reject|cancel|once|forever|permanent)"' packages/client/src/   # 0 hit (data-action 仅 deny/grant_one_shot/grant_always 三值)
git grep -nE 'data-hb3-button=(?!"danger"|"primary")' packages/client/src/   # 0 hit (二值锁)
```

**改 = 改两处**: 此文档反约束清单 + `packages/client/src/__tests__/hb-3-content-lock.test.ts` 单测.

## §3 退出条件 (跟 stance + acceptance + spec 4 件套联签)

- §1 三按钮 DOM data-action + label byte-identical (改 = 改三处)
- §2 同义词反向 grep ≥10 全 0 hit
- 跟蓝图 §1.3 弹窗 UX 模板 byte-identical (跟 onboarding-journey 弹窗模式同源)
- DOM attr 锁 `data-hb3-button` ∈ {"danger", "primary"} 二值 (反向断言枚举外值)
