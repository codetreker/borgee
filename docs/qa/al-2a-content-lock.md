# AL-2a SPA agent 配置文案锁 (野马 G4.x demo 预备)

> **状态**: v1 (gh#701 drift 修, zhanma-c 2026-05-08) — 跟 v0 (野马, 2026-04-29) 对比: 容器元素 form→section + 标题"设置"→"配置" + 文件名 AgentSettings*→AgentConfigPanel, 跟代码 + 测试事实真值对齐 (PR #454 之后代码 + test 用了 6 天稳定, drift 真因是 md 早写代码晚写漂的, 真值 = code+test)
> **目的**: AL-2a.x client UI 实施前锁 SPA agent 配置面板 + button + toast + disabled state 文案 byte-identical — 跟 AL-3 #305 / DM-2 #314 / AL-4 #321 / CHN-2 #354 / CV-2 #355 / CV-3 #370 / CV-4 #380 / CHN-4 #382 / CHN-3 #402 同模式 (用户感知签字 + 文案 byte-identical), 防 AL-2a 实施时把 SSOT 字段漂入 runtime-only 字段 / 失败 toast 漂同义词 / disabled state 视觉模糊。
> **关联**: 蓝图 `plugin-protocol.md` §1.4 (Borgee SSOT 字段划界 — name/avatar/prompt/model/能力开关/启用状态/memory_ref) + §1.5 (热更新分级); 蓝图 `agent-lifecycle.md` §2.1 (用户完全自主决定 agent 的 name/prompt/能力/model); 飞马 spec brief `docs/implementation/modules/al-2-spec.md`; 烈马 acceptance template `al-2a.md` 7 项 (REG-AL2A-001..007); AL-1a #249 6 reason codes byte-identical (失败 reason 同源).
> **#338 cross-grep 反模式遵守**: 既有 `lib/agent-state.ts` REASON_LABELS (#249) + AL-3 #305 toast 模板 + CV-1 #347 kindBadge 字面已稳定, 本锁字面跟既有 byte-identical 引用 (改 reason = 改源头), 不臆想新词。

---

## 1. 7 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **agent 配置面板入口** (agent 详情页 owner-only) | DOM: `<section data-agent-config="root" data-schema-version="{n}">` byte-identical (`data-agent-config` attr 锁); 标题 `"Agent 配置"` byte-identical 中文 (反 "Agent Settings" / "Agent Config" / "设置" / "Configuration"); owner-only DOM omit (跟 CV-1 #347 line 254 + CV-4 #380 ① + AL-4 #321 同模式 defense-in-depth) | ❌ 不准 "Settings" / "Config" / "Configuration" / "设置" 同义词漂移 (中文 "配置" byte-identical 锁); ❌ 非 owner DOM omit (反 disable 渲染); ❌ 不准 admin SPA 渲染此面板 (admin god-mode 字段白名单不含 agent_configs.blob, 跟 ADM-0 §1.3 红线 + AL-4 #379 v2 同模式); ❌ 不准用 `<form data-form="agent-config">` 容器 (跟代码 AgentConfigPanel.tsx L110 事实真值 byte-identical 锁; gh#701 drift 修 — md 早期写过 form 但代码用 section, 真值 = section)
| ② | **SSOT form 字段锁** (蓝图 §1.4 字段划界 byte-identical) | form 字段 byte-identical 跟蓝图 §1.4 "归 Borgee 管" 列同源: `name` / `avatar` / `prompt` / `model` / `capabilities` (能力开关) / `enabled` (启用状态) / `memory_ref` 七字段; **不准** runtime-only 字段进 form (跟 §1.4 反约束 byte-identical) | ❌ 不准 form 出现 `temperature` / `token_limit` / `api_key` / `retry_policy` / `rate_limit` / `memory_content` 字段 (蓝图 §1.4 "归 Runtime 管" 列字面禁); ❌ 不准 model 字段写死下拉 (走 `runtime_schema_advertise` 通用渲染, 蓝图 §1.4 "不写死 OpenClaw/Hermes 具体模型列表"); ❌ 不准 memory_ref 直接编辑 memory 内容 (蓝图 §1.4 "memory 内容在 runtime") |
| ③ | **保存按钮 + 成功 toast** | 保存按钮文案: `"保存"` byte-identical (反 "Save" / "更新" / "提交" / "Apply" 同义词); 成功 toast: `"已保存"` byte-identical 1.5s (跟 CV-3 #370 ③ "已复制" toast 1.5s 同精神 + 中文 byte-identical 锁); PATCH `/api/v1/agents/:id/config` body 含整 blob (SSOT 整体替换, 跟 spec §AL-2a 同源) | ❌ 不准 "Save" / "Update" / "Apply" / "更新" 同义词; ❌ 不准 toast 持续 >3s (UX 噪声); ❌ 不准 PATCH body 仅含变更字段 — 必须整 blob 替换 (反 multi-row config_key 漂移漂入); ❌ 不准成功 toast 显示 `schema_version` 数字 (隐藏内部状态 — UX 简洁) |
| ④ | **失败 toast** (PATCH /config 失败 — 网络 / 409 / 403) | 失败 toast 字面 byte-identical: `"保存失败 ({reason_label})"` (跟 CV-4 #380 ③ "失败: {reason_label}" + AL-3 #305 ③ "故障 ({reason_label})" 同括号格式精神, byte-identical reason_label 走 AL-1a #249 REASON_LABELS); 4xx / 5xx 走映射: 409 → reason='conflict' / 403 → reason='unauthorized' / 5xx → reason='unknown' (跟 #249 6 reason 不直接对应, 可在 AL-2a SPA 侧加 2 项 SPA-only reason `conflict` + `unauthorized`, 但**仍跟 AL-1a 模板字面同源**) | ❌ 不准 raw error.message 显示 (隐私 + UX, 跟 AL-3 #305 + CV-4 #380 ③ 同精神); ❌ 不准 "Save failed" / "Failed to save" / "请稍后重试" 同义词; ❌ 不准 reason_label 漂出枚举 (改 reason = 改 AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-4 #387/#461 + AL-1b #458 + 本锁 **六处单测锁** 模式承袭) |
| ⑤ | **disabled state** (PATCH 进行中 / form invalid) | 保存按钮 disabled 态: `disabled` attr + `aria-disabled="true"` byte-identical + 视觉降灰 (CSS `opacity: 0.5`); 提交中按钮文案 `"保存中…"` byte-identical (跟 CV-4 #380 ③ "agent 正在迭代…" 进行态字面精神); form invalid 态 — 必填字段空 → 按钮 disabled + 字段下显错 `"此字段必填"` byte-identical | ❌ 不准 "Saving..." / "Submitting..." / "处理中" 同义词; ❌ 不准漏 `aria-disabled` (a11y 永久锁); ❌ 不准 disabled 仍触发 PATCH 请求 (双层防御); ❌ 不准 invalid state 显示 server 端 error (那是 ④ failed toast 的事, 字段下错显示 client 端校验) |
| ⑥ | **schema_version 隐式锁** (UI 不显示, 但拉取时附带) | UI form **不渲染** schema_version (隐藏内部状态); GET `/api/v1/agents/:id/config` 返 body 含 `schema_version` 数字, PATCH 时 client 端读后**不必**回传 (server 端单调递增不依赖 client 数字, 跟 spec §AL-2a "schema_version 严格递增" + 烈马 acceptance §4.1.a 同源) | ❌ 不准 form 字段渲染 schema_version (UI 噪声); ❌ 不准 client 端 lock-on schema_version (那是 CV-1 #347 锁 ② 单文档锁 30s TTL 路径 — AL-2a 不裂; AL-2a 走 last-writer-wins 跟 spec §AL-2a 同源); ❌ 不准 PATCH body 含 client 端算的 schema_version (server 单调递增锁 — 反 first-write-wins 错算) |
| ⑦ | **agent silent (无入场白)** (蓝图 #11 沉默胜于假活物感) | agent settings form 修改后 — 不发自我介绍 / 不发 "已更新" 系统消息 (跟 AL-3 #305 ③ "agent join silent default" 字面同精神); 仅 owner 在 SPA 看到 ③ "已保存" toast, 不污染 channel chat 流 / 不发 system DM (跟 #11 silent default + #382 立场 ⑤ "messages 流不污染 iterate state" 同精神) | ❌ 不准发 agent system message "{agent_name} 已更新设置" (跟 #11 + AL-3 silent default 同源); ❌ 不准 fanout 给 channel members (这是 owner 自己的事); ❌ 不准触发 BPP frame `agent_config_update` (留给 AL-2b, 跟 spec + acceptance §1.5 字面对齐) |

---

## 2. 反向 grep — AL-2a.x PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# ① 容器 data-attr + 标题 byte-identical (预期 ≥1)
grep -rnE 'data-agent-config=["'"'"']root["'"'"']' packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"]Agent 配置['\"]" packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"](Settings|Config|Configuration|设置|Agent Settings)['\"]" packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test
# ① gh#701 drift 防御反向锁 — md / packages 都不准再出现 form data-form="agent-config" (代码事实是 section + data-agent-config)
grep -rnE 'data-form=["'"'"']agent-config["'"'"']' packages/ docs/qa/ 2>/dev/null  # 预期 0
# ② SSOT form 不准含 runtime-only 字段
grep -rnE 'name=["'"'"']?(temperature|token_limit|api_key|retry_policy|rate_limit|memory_content)["'"'"']?' packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test
# ② model 字段不写死下拉 (走 runtime_schema_advertise 通用渲染)
grep -rnE "['\"](gpt-4|gpt-3\\.5|claude-3|gemini)['\"]" packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test
# ③ 保存按钮文案 + 成功 toast byte-identical (预期 ≥1 + 预期 ≥1)
grep -rnE "['\"]保存['\"]" packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"]已保存['\"]" packages/client/src/ 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"](Save|Update|Apply|提交|更新)['\"]" packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test
# ④ 失败 toast 字面 + reason_label 走 REASON_LABELS (预期 ≥1)
grep -rnE "['\"]保存失败 \\(\\$\\{.*reason.*\\}\\)['\"]|['\"]保存失败 \\(\\{.*\\}\\)['\"]" packages/client/src/ 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE 'REASON_LABELS\[' packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"](Save failed|Failed to save|请稍后重试|网络错误)['\"]" packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test
# ⑤ disabled state aria-disabled + "保存中…" byte-identical (预期 ≥1)
grep -rnE 'aria-disabled=["'"'"']true["'"'"']' packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"]保存中…['\"]" packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"](Saving\\.\\.\\.|Submitting\\.\\.\\.|处理中)['\"]" packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test
# ⑥ schema_version 不进 UI form (UI 噪声防御)
grep -rnE "label=['\"].*schema.*version|aria-label=['\"].*schema.*version" packages/client/src/components/AgentConfigPanel*.tsx 2>/dev/null | grep -v _test
# ⑦ agent silent default — 不发 system message
grep -rnE "['\"]\\{agent_name\\} 已更新设置['\"]|agent.*config.*system.*message" packages/server-go/internal/api/ 2>/dev/null | grep -v _test
# ⑦ AL-2b 留账 — agent_config_update BPP frame 不在 AL-2a (spec + acceptance §1.5)
grep -rnE 'agent_config_update' packages/server-go/internal/ws/ packages/server-go/internal/bpp/ 2>/dev/null | grep -v _test.go
```

---

## 3. 验收挂钩 (AL-2a.x PR 必带)

- ① 容器 DOM e2e: owner 视角 `data-agent-config="root"` 渲染 + 非 owner DOM omit (count==0) + admin god-mode 反向断言不渲染 + gh#701 drift 反向锁 `data-form="agent-config"` 在 packages/ + docs/qa/ count==0
- ② SSOT 字段 e2e + vitest: 7 字段 byte-identical 跟蓝图 §1.4 同源 + 反向断言无 runtime-only 字段 + model 走 runtime_schema_advertise 通用渲染
- ③ 保存 + 成功 toast e2e: PATCH 成功 → toast `"已保存"` byte-identical 1.5s + PATCH body 整 blob 替换反向断言
- ④ 失败 toast e2e: 模拟 409/403/5xx → toast `"保存失败 ({reason_label})"` byte-identical + reason_label 走 REASON_LABELS (改 reason = 改六处单测锁: AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-4 #387/#461 + AL-1b #458 + 本锁)
- ⑤ disabled state e2e: PATCH 进行中按钮 `disabled` + `aria-disabled="true"` + 文案 "保存中…" + 反向断言无 PATCH 重复触发
- ⑥ schema_version e2e: GET 返 body 含 schema_version 但 form 不渲染 + PATCH body 不含 client 算的 schema_version (server 单调递增反断)
- ⑦ silent default e2e: PATCH 后反向断言无 system message + 无 fanout + 无 BPP frame `agent_config_update` (留 AL-2b)
- G4.x demo 截屏 3 张归档 (跟 #391 §1 截屏路径锁同源): `docs/qa/screenshots/g4.x-al2a-{form-empty,form-saving,toast-error}.png` 撑 Phase 4 退出闸 demo

---

## 4. 不在范围

- ❌ runtime-only 字段配置 (蓝图 §1.4 字面禁, AL-2a 永久锁不准漂入)
- ❌ multi-row config_key 模式 (走整 blob 替换, 反 SSOT 漂)
- ❌ BPP frame `agent_config_update` (留 AL-2b, 跟 spec + acceptance §1.5 字面)
- ❌ admin SPA 改 agent_configs (admin 不入业务路径, ADM-0 §1.3 红线)
- ❌ memory 内容编辑 (蓝图 §1.4 字面 "memory 内容在 runtime", memory_ref 仅指针)
- ❌ schema_version client 端锁 (那是 CV-1 #347 锁 ② 30s TTL 路径; AL-2a 走 last-writer-wins)
- ❌ agent self-update (跟 #11 silent default + agent 不能自 grant 同源, runtime 配置由 owner 决定)
- ❌ system message broadcast 设置变化 (跟 #11 silent default 永久锁)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 7 处文案锁 (form data-form attr + 标题 "Agent 设置" + SSOT 7 字段锁跟蓝图 §1.4 同源 + 保存按钮"保存" + 成功 toast "已保存" 1.5s + 失败 toast "保存失败 ({reason_label})" byte-identical 跟 AL-3 #305/CV-4 #380/AL-4 #387/#249 五处单测锁 + disabled state aria-disabled + "保存中…" + schema_version UI 隐藏 + agent silent default 跟 #11 同精神) + 17 行反向 grep (含 7 预期 ≥1 + 10 反约束) + G4.x demo 截屏 3 张归档. #338 cross-grep 反模式遵守: 既有 REASON_LABELS (#249) + AL-3/CV-4 toast 模板 字面已稳定, 本锁跟既有 byte-identical 引用不臆想新词 |
| 2026-04-29 | 野马 | v0.x patch — cross-milestone reason count audit (跟 #461 同模式 follow-up): "五处单测锁"/"四处单测锁"/"三处单测锁" → "六处单测锁" (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461); CHN-4 stance ④ "6 源" → "7 源" 跟 body 列举 7 源对齐. 跟 #339/#393/#387/#461 follow-up patch 同模式, 历史干净 |
