# Agent Config Panel (AL-2a.3 client SPA)

> 蓝图: `agent-lifecycle.md §2.1` (用户完全自主决定 agent 的 name/prompt/能力/model) + `plugin-protocol.md §1.4` (Borgee owns the documented config fields) + §1.5 (热更新分级：server PATCH 后经 BPP frame `agent_config_update` 推送给 plugin 端重新加载，走 AL-2b #481 wire；client 端不订阅 WebSocket，走 PATCH/GET 同步)
> Server 出处: `docs/current/server/README.md §Agent config 单一来源 (AL-2a.2)` + `docs/current/server/data-model.md::agent_configs` (v=20)
> Component: `packages/client/src/components/AgentConfigPanel.tsx` form + mount in `AgentManager.tsx` expanded section (between RuntimeCard 和 Permissions, 标题 "Config (单一来源)")
> API: `packages/client/src/lib/api.ts::fetchAgentConfig` + `updateAgentConfig`
> Tests: `packages/client/src/__tests__/al-2a-content-lock.test.ts` (8 cases)

## 1. 入口与场景

owner 在 agent settings 下编辑本人 agent 的 7 个配置字段：name / avatar / prompt / model / capabilities / enabled / memory_ref。Save 提交 PATCH `/api/v1/agents/{id}/config`，server schema_version 严格递增（由 server 标记版本并执行单调 UPSERT）。

```
+──────────────────────────────────────────────────+
│  Agent 配置                              [v3]   │
├──────────────────────────────────────────────────┤
│  名称       [_________________________________]  │
│  头像 URL   [_________________________________]  │
│  Prompt     ┌───────────────────────────────┐   │
│             │                               │   │
│             └───────────────────────────────┘   │
│  模型       [_________________________________]  │
│  memory_ref [_________________________________]  │
│  启用       [✓]                                  │
│                                                  │
│                                       [ 保存 ]   │
+──────────────────────────────────────────────────+
```

## 2. 文案一致性（跨层逐字一致）

| 文案                                                                                | Client source                   | Matching source                                                               |
| ----------------------------------------------------------------------------------- | ------------------------------- | ----------------------------------------------------------------------------- |
| `agent 配置保存失败, 请重试`                                                        | `AGENT_CONFIG_SAVE_TOAST` const | server-go `agentConfigSaveErrorMsg` const + al-2a-content-lock.test.ts case ① |
| 加载中...                                                                           | render loading state            | DOM `data-agent-config="loading"` 出处                                        |
| Agent 配置 / 名称 / 头像 URL / Prompt / 模型 / memory_ref / 启用 / 保存 / 保存中... | form labels                     | AgentConfigPanel.tsx 中逐字锁定                                               |

## 3. DOM attribute consistency

- `data-agent-config="root"` — section 容器
- `data-agent-config="loading"` — 加载态
- `data-agent-config-version` — schema_version 显示元素
- `data-agent-config-field="{name|avatar|prompt|model|memory_ref|enabled}"` — 6 form input
- `data-agent-config-action="save"` — 保存按钮

## 4. 数据流

```
onMount → fetchAgentConfig(agentId) → GET /api/v1/agents/{id}/config
       → setConfig({schema_version, blob, updated_at})
       → setDraft(config.blob)

onSave → updateAgentConfig(agentId, draft) → PATCH /api/v1/agents/{id}/config
      → response: {schema_version: prev+1, blob, updated_at}
      → setConfig(updated) + setDraft(updated.blob) (re-fetch 防 cache 不刷)
      → 失败: showToast(AGENT_CONFIG_SAVE_TOAST)
```

`onMount + Save 后 re-fetch` 是 acceptance §4.1.d 的证据：agent reload path 使用 polling，client 使用 GET，不订阅 WebSocket push frames（蓝图 §1.5 BPP `agent_config_update` 留 AL-2b）。

## 5. Negative constraints (蓝图 §1.4 source of truth + §1.5 BPP frame constraints)

UI 层 + server 层双层拒绝未允许字段:

- `data-agent-config-field="{api_key|temperature|token_limit|retry_policy}"` count==0 — runtime-only 字段 UI **不渲染** form input（UI 层拒绝）；server `allowedConfigKeys` whitelist 以 400 拒绝，code 为 `agent_config.runtime_field_rejected`（server 层拒绝）
- 不订阅 WebSocket push — grep 检查: `subscribeWS` / `hub.subscribe` count==0 in AgentConfigPanel.tsx
- BPP frame `'agent_config_update'` 单引号字面 (代码使用形式) count==0 — 仅 doc comment 出现说明设计, 不在代码路径

## 6. ADM-0 constraint (no admin bypass path)

`/admin-api/v1/agents/{id}/config` 路径**不**挂 (跟 ADM-0 §1.3 + AL-3 #303 ⑦ 同模式)。client 的 `fetchAgentConfig` / `updateAgentConfig` 只调 `/api/v1/agents/{id}/config` (owner-only ACL, server 校验 owner.id == agent.OwnerID)。Cross-owner 调用 → 403。

## 7. 跟 server 字段映射（逐字锁定）

| client `ALLOWED_CONFIG_KEYS` | server `allowedConfigKeys` | 蓝图 §1.4                           |
| ---------------------------- | -------------------------- | ----------------------------------- |
| `name`                       | `name`                     | "归 Borgee 管"                      |
| `avatar`                     | `avatar`                   | "归 Borgee 管"                      |
| `prompt`                     | `prompt`                   | "归 Borgee 管"                      |
| `model`                      | `model`                    | identifier 字符串 (非 LLM 调用参数) |
| `capabilities`               | `capabilities`             | 能力开关                            |
| `enabled`                    | `enabled`                  | 启用状态                            |
| `memory_ref`                 | `memory_ref`               | 单一来源一致                        |

改动 list 时，需要同步改 server map、al-2a-content-lock.test.ts 字面锁定、acceptance §数据契约 row 2 三处。

## 8. 测试

`packages/client/src/__tests__/al-2a-content-lock.test.ts` 9 cases:

- ① toast 字面逐字一致
- ② allowedConfigKeys 7 字段
- ③ data-agent-config-field 二态锁定
- ④ DOM root + version + save action
- ⑤ API endpoint path + method 跟 server 同源
- 反向约束 runtime-only 4 字段不渲染
- 反向约束 不订阅 push frame
- 限制: toast synonym drift 0 hit
- grep 检查 (gh#701 drift fix): packages/ + docs/qa/ 全树 `data-form="agent-config"` 字面 0 hit（容器是 section，不是 form）

## 9. Layout checks (gh#698)

6 个 label 的 inline `style="display: block"` 用于避免移动端 viewport 宽度下 label/input 重叠。不要替换成 `.form-group` / `.form-field` classes，因为这些 classes 在本项目中不存在。checkbox label 例外使用 `style="display: flex; align-items: center; gap: 8px"`，让 ☐ 跟 "启用" 同行。

`<label htmlFor>` 隐式关联走 `<label> {input}` 嵌套 (不显式 htmlFor 配 input id) — 跟 borgee 既有 form 一致.

Inline style checks were verified at 1280/480/1024 with 0 overlap. 详见 design `docs/implementation/design/698-agent-config-form-overlap.md`.

## 10. Shape check (gh#684)

textarea (prompt 字段) 默认 `rows={8}` + `style={{ resize: 'vertical', width: '100%', boxSizing: 'border-box' }}`，避免默认 1 行过小。详见 [`agent-manager.md §5`](agent-manager.md#5-prompt-textarea-gh684-22)。

## 11. form 状态保护 (gh#703)

接入 `useUnsavedChangesGuard`（跟 #695 sidepane 切换 + #709 hook beforeunload 联动）：sidepane 切换、关 tab、刷新前，如有未保存改动则弹 confirmation。

isDirty 推算 (编辑 form 模式):

<!-- prettier-ignore -->
```ts
() => !loading && !saving && config !== null
   && JSON.stringify(draft) !== JSON.stringify(config.blob)
```

详见 [`../hooks/useUnsavedChangesGuard.md`](../hooks/useUnsavedChangesGuard.md).
