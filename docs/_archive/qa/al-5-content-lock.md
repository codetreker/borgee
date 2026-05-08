# AL-5 文案锁 / DOM 字面锁 (战马C + 野马 v0)

> 战马C · 2026-04-29 · ≤40 行 byte-identical 锁 (4 件套第三件; 跟 BPP-3.2 #498 + AL-1b #458 + AP-1 #493 同模式)
> **蓝图锚**: [`agent-lifecycle.md`](../blueprint/current/agent-lifecycle.md) §1.6 失联与故障 + [`auth-permissions.md`](../blueprint/current/auth-permissions.md) §1.3 主入口字面承袭
> **关联**: spec `docs/implementation/modules/al-5-spec.md` (战马C v0, 1dded5e) + stance `docs/qa/al-5-stance-checklist.md` + acceptance `docs/qa/acceptance-templates/al-5.md`. 复用 BPP-3.2.1 #498 SendSystemDM + REFACTOR-REASONS #496 reasons SSOT 6-dict.

## §1 DM body 字面锁 (蓝图 §1.6 + 野马签字)

字面 (改 = 改三处: 蓝图 §1.6 + spec §0 立场 ① + content-lock + 实施代码 `internal/agent/recover.go::dmBodyTemplate`):

```
{agent_name} 状态变更: error ({reason_label}). 点击重连
```

字段插值:
- `{agent_name}` = `agents.display_name`
- `{reason_label}` = AL-1a 6-dict 字面 byte-identical 跟 reasons SSOT (api_key_invalid / quota_exceeded / network_unreachable / runtime_crashed / runtime_timeout / unknown — 跨 milestone 第 10 处单测锁链)

**反向 grep** (count==0): `agent.*报错.*重启\|agent.*已失联.*重连` (近义词漂禁, 仅 "状态变更: error (...) 点击重连"字面)

## §2 quick_action JSON shape 字面锁 (4-enum 扩 `recover`)

`messages.quick_action` JSON payload byte-identical 跟 BPP-3.2 既有 4-enum 模式扩:

```json
{"action":"recover","agent_id":"<uuid>","reason":"<6-dict>","request_id":"<uuid>"}
```

**反约束**: `action` 4 枚举锁 (跟 BPP-3.2.2 既有 grant/reject/snooze 共序, AL-5 加 1 = 4 enum). 反向 grep `"action":"reconnect"\|"action":"restart"\|"action":"reboot"` count==0 (近义词漂禁).

## §3 SystemMessageBubble 单按钮 DOM 字面锁

单按钮 byte-identical (改 = 改两处: 此 content-lock + `packages/client/src/components/SystemMessageBubble.tsx::renderRecoverAction`):

| label | data-action | data-bpp32-button | 视觉精神 |
|---|---|---|---|
| `重连` | `recover` | `"primary"` | 主按钮 (绿色) — 一键 owner 触发 agent error→online recovery |

**DOM attr** byte-identical 锁 (e2e 用, 反向 grep `data-action="reconnect\|reboot\|restart\|reset"` count==0)

**反向 grep 同义词禁词** (count==0, 跟 BPP-3.2 12 词 + AL-1b 4 词同模式守 future drift):

- `重启|重启动|reboot|restart` (字面仅 "重连")
- `重置|reset|reset_state` (字面仅 "重连")
- `恢复|recover_now|fix` (字面仅 "重连")
- `修复|repair|heal` (字面仅 "重连")

锚 implementation: `packages/client/src/__tests__/SystemMessageBubble.al5.test.tsx::TestAL5_RecoverButton_LiteralLock` + `internal/agent/recover_test.go::TestAL5_NotifyOwnerOnError_DMBodyLiteral`

## §4 错码字面锁

byte-identical 跟 BPP-2.2 `bpp.task_subject_empty` + BPP-3.1 `bpp.permission_denied` + BPP-3.2 `bpp.grant_*` 命名同模式 (改 = 改两处: 此锁 + 实施代码 const):

- `bpp.recover_not_owner` — non-owner 调 POST /agents/:id/recover → 403
- `bpp.recover_state_invalid` — agent 非 error 态 → 409 (online/idle/busy/offline 4 态全 reject)
- `bpp.recover_reason_unknown` — reason 字典外 (REFACTOR-REASONS 6-dict guard) → 400
- `bpp.recover_agent_not_found` — agent_id 不存在 → 404

## §5 跨 PR drift 守 (双向 grep CI lint)

改 `recover` action / `重连` label / 4 错码 = 改五处 (双向 grep 等价单测覆盖):
1. `docs/blueprint/current/agent-lifecycle.md` §1.6 + `auth-permissions.md` §1.3
2. `internal/agent/recover.go` (DM body template + reason validation)
3. `internal/api/agent_recover.go` (POST /agents/:id/recover handler + 4 错码)
4. `packages/client/src/components/SystemMessageBubble.tsx` (单按钮 DOM 锁 + isAL5RecoverPayload type guard)
5. 此 content-lock §1+§2+§3+§4

## 更新日志

- 2026-04-29 — 战马C + 野马 v0 (4 件套第三件, ≤40 行 +): DM body + quick_action JSON 4-enum + 单按钮 DOM + 4 错码 + 跨 PR drift 5 处全锁; 同义词反向 grep 12 词禁; BPP-3.2 / AL-1 / REFACTOR-REASONS 跨 PR byte-identical 守.
