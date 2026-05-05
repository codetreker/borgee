# BPP-3.2 文案锁 / DOM 字面锁 (战马C + 野马 v0)

> 战马C · 2026-04-29 · ≤40 行 byte-identical 锁 (4 件套第三件; 跟 BPP-2 #485 + AL-1b #458 + AP-1 #493 同模式)
> **蓝图锚**: [`auth-permissions.md`](../blueprint/current/auth-permissions.md) §1.3 主入口字面 (DM body + 三按钮)
> **关联**: spec `docs/implementation/modules/bpp-3.2-spec.md` (战马C v0, c8e37a4) + stance `docs/qa/bpp-3.2-stance-checklist.md` (战马C v0) + acceptance `docs/qa/acceptance-templates/bpp-3.2.md` (战马C v0); 复用 BPP-3.1 #494 frame body + AP-1 #493 capabilities const

## §1 DM body 字面锁 (蓝图 §1.3 主入口字面承袭)

字面 (改 = 改三处: 蓝图 §1.3 + spec §0 立场 ① + content-lock + 实施代码 `internal/api/capability_grant.go::dmBodyTemplate`):

```
{agent_name} 想 {attempted_action} 但缺权限 {required_capability}
```

字段插值 byte-identical 跟 BPP-3.1 PermissionDeniedFrame 字段名 (`attempted_action` / `required_capability`, 跨 PR drift 守 — 改 = 改五处+: 蓝图 §4.1 + AP-1 abac.go + BPP-3.1 frame + DM body + content-lock).

**反向 grep** (count==0): `agent.*尝试.*权限\|agent.*请求.*授权` (近义词漂禁, 仅"想 ... 但缺权限"字面)

## §2 quick_action JSON shape 字面锁

`messages.quick_action` JSON payload byte-identical (跟 CM-onboarding #203 既有 schema 同模式 — 改 = 改两处: CM-onboarding `quick_action` schema + 此 content-lock):

```json
{"action":"grant"|"reject"|"snooze","agent_id":"<uuid>","capability":"<14-const>","scope":"<v1-three-layer>","request_id":"<uuid>"}
```

**反约束**: `action` 仅 3 枚举 (grant/reject/snooze), 反向 grep `"action":"defer"\|"action":"approve"\|"action":"deny"\|"action":"allow"` count==0 (近义词漂禁, 字面承袭蓝图 §1.3)

## §3 SystemMessageBubble DOM 字面锁 (BPP-3.2.2 client UI)

三按钮 byte-identical (改 = 改两处: 此 content-lock + `packages/client/src/components/SystemMessageBubble.tsx::renderQuickActions`):

| label | data-action | data-bpp32-button | 视觉精神 |
|---|---|---|---|
| `授权` | `grant` | `"primary"` | 主按钮 (绿色) — 一键 grant 限于此 channel |
| `拒绝` | `reject` | `"danger"` | 次按钮 (红色) — 仅 dismiss DM, 不持久化 deny |
| `稍后` | `snooze` | `"ghost"` | 弱按钮 (灰色) — dismiss DM 暂不处理 |

**DOM attr** byte-identical 锁 (e2e 用, 反向 grep `data-action="approve\|defer\|allow\|disallow\|maybe"` count==0)

**反向 grep 同义词禁词** (count==0, 跟 AL-1b "活跃/running/Standing by" 同模式守 future drift):

- `批准|授予|同意|许可` (字面仅 "授权")
- `驳回|拒接|否决|不允许` (字面仅 "拒绝")
- `稍候|延后|推迟|暂缓|过会儿` (字面仅 "稍后")

锚 implementation: `packages/client/src/__tests__/SystemMessageBubble.bpp32.test.tsx::TestThreeButtons_LiteralLock` + `packages/e2e/tests/bpp-3.2-grant-flow.spec.ts::§3 三按钮 DOM 锁`

## §4 错码字面锁

byte-identical 跟 BPP-2.2 `bpp.task_subject_empty` + BPP-2.3 `bpp.config_field_disallowed` + BPP-3.1 `bpp.permission_denied` 命名同模式 (改 = 改两处: 此锁 + 实施代码 const):

- `bpp.grant_capability_disallowed` — owner 端 POST /me/grants 入 capability 不在 14 项 const 时 reject + log warn
- `bpp.retry_exhausted` — plugin 端 retry > 3 次 abort + log warn

## §5 跨 PR drift 守 (双向 grep CI lint)

改 `required_capability` / `current_scope` / `request_id` = 改五处 (双向 grep 等价单测覆盖):
1. `docs/blueprint/current/auth-permissions.md` §4.1 row
2. `packages/server-go/internal/auth/abac.go` 403 body (AP-1 #493)
3. `packages/server-go/internal/bpp/envelope.go::PermissionDeniedFrame` 字段 (BPP-3.1 #494)
4. `packages/server-go/internal/api/capability_grant.go` DM body 模板 (BPP-3.2.1)
5. 此 content-lock §1+§2

## 更新日志

- 2026-04-29 — 战马C + 野马 v0 (4 件套第三件, ≤40 行 +): DM body + quick_action JSON + 三按钮 DOM + 错码 + 跨 PR drift 5 处全锁; 同义词反向 grep 12 词禁; BPP-3.1 / AP-1 / CM-onboarding 跨 PR byte-identical 守.
