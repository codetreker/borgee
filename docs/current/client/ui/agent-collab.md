# Agent collab (CM-5.3 client SPA)

> 蓝图: `concept-model.md §1.3` (§185 "未来你会看到 agent 互相协作") + `agent-lifecycle.md §1` (Borgee 是协作平台)
> Spec: `docs/implementation/modules/cm-5-spec.md` (5 设计原则 + 3 拆段)
> Server 出处: `docs/current/server/cm-5.md` (CM-5.1 反向约束 + CM-5.2 server 路径验证)
> Client lib: `packages/client/src/lib/cm5-toast.ts` (X2 conflict toast + DOM attr 锁定)
> Tests: `packages/client/src/__tests__/cm-5-content-lock.test.ts` (6 cases) + `packages/e2e/tests/chat-two-user-collab.spec.ts`

## 1. 入口与场景

owner 在 channel 视图中把 agent 作为协作者看到: channel members modal 中 agent 行带 hover anchor `data-cm5-collab-link`, hover 显示 "{agentName} 正在协作" tooltip. agent↔agent 协作产物 (mention / message / artifact iterate) 走与人类协作者相同的协作路径. UI 不添加 `ai_only` / `agent_only` visibility scopes, owner 因此看到完整协作链.

```
+──────────────────────────────────────────────+
│  #channel-name                  [⚙ 成员]    │
├──────────────────────────────────────────────┤
│  Members:                                    │
│   👤 Alice (owner)                          │
│   🤖 AgentA [Bot]   <-- hover: "正在协作"   │ ← data-cm5-collab-link
│   🤖 AgentB [Bot]   <-- hover: "正在协作"   │ ← data-cm5-collab-link
│   👤 Bob                                     │
└──────────────────────────────────────────────+
```

## 2. Text consistency (byte-identical across layers)

| 文案                       | Client source                                       | Matching source                                                |
| -------------------------- | --------------------------------------------------- | -------------------------------------------------------------- |
| `正在被 agent {name} 处理` | `formatCM5X2ConflictToast(name)` (lib/cm5-toast.ts) | acceptance §3.2 + spec §1.3 + cm-5-content-lock.test.ts case ① |
| `正在协作`                 | `title=` attr in ChannelMembersModal agent rows     | acceptance §3.1 + cm-5-content-lock.test.ts case ②             |

## 3. DOM attribute consistency

- `data-cm5-collab-link=""` — agent 行 hover anchor (锁定 ChannelMembersModal `member-name` span on agent rows)
- Negative constraint: `data-ai-only` / `data-agent-only` / `data-visibility-scope` / `data-agent-visible-only` 0 hit (蓝图 §185 透明协作设计)

## 4. Negative constraints (蓝图 §185 transparent collaboration + §1.4 source of truth)

UI 层设计:

- 不引 owner_visibility scope 字段 — agent↔agent 协作产物对 owner 透明可见 (跟人协作产物可见同模式)
- 不订阅 BPP frame `agent_config_update` 单引号字面 0 hit — 走轮询 + 既有 path (BPP frame 留 AL-2b + BPP-3 同合)
- X2 error-code wording is reused: CM-5 must not introduce `cm5.x2_conflict` / `agent_collision` / `artifact.x2_conflict` / `x2_lock_held` synonyms. It reuses the CV-4 #380 ⑦ existing code `artifact.locked_by_another_iteration` byte-identical (cm5stance.TestCM51_X2ConflictLiteralReuse server-side negative constraint).

## 5. ADM-0 constraint (no admin bypass path)

CM-5 不开新 endpoint, 走既有 channel members + artifacts API. Admin bypass APIs must not add any CM-5-specific path (跟 ADM-0 §1.3 + AL-3 #303 ⑦ 同模式).

## 6. 跟 server 字段映射 (byte-identical 锁定)

| client `lib/cm5-toast.ts`            | server (无新 endpoint)                                                         | spec §1.3              |
| ------------------------------------ | ------------------------------------------------------------------------------ | ---------------------- |
| `formatCM5X2ConflictToast(name)`     | CV-1.2 既有 409 错码 `Artifact is locked by another editor` (artifacts.go:434) | 设计 ③ X2 复用既有路径 |
| `CM5_COLLAB_LINK_DOM_ATTR`           | (UI-only)                                                                      | 设计 ⑤ 透明可见        |
| `CM5_FORBIDDEN_VISIBILITY_DOM_ATTRS` | 反向约束 (server cm5stance.TestCM51_NoBypassTable)                             | 设计 ⑤ 反 ai_only      |

## 7. 测试

`packages/client/src/__tests__/cm-5-content-lock.test.ts` 6 cases:

- ① X2 conflict toast 字面 byte-identical
- ② DOM hover anchor `data-cm5-collab-link` 锁定
- ③ 反向约束 ai_only / agent_only DOM attr 不渲染
- ④ 反向约束 不订阅 push frame
- ⑤ 反向约束 X2 错码同义词 0 hit
- reverse assertion for toast synonym drift

`packages/e2e/tests/chat-two-user-collab.spec.ts` 1 case 综合: 双 agent 同 channel + members modal hover anchor 锁定 + X2 stale commit 409 + screenshot `docs/qa/screenshots/cm-5-x2-conflict.png`.
