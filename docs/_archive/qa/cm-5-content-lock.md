# CM-5 文案锁 (野马 client UI 字面 byte-identical 锚)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CM-5.x client UI 实施前锁 X2 conflict toast + agent 协作链路 hover anchor 文案 — 跟跨 milestone byte-identical 链同源 (CV-4 #380 ⑦ + AL-2a #454 ④ + 跨 milestone reason 六处单测锁 post-#467), 防 CM-5.3 实施时 X2 toast 漂同义词 / hover anchor 漂离立场 ⑤ 透明协作精神.
> **关联**: 战马 spec brief `docs/implementation/modules/cm-5-spec.md` 立场 ③ X2 + 立场 ⑤ 透明协作; #463 spec v0 + #473 CM-5.1 反约束 grep + #476 CM-5.2 5 立场端到端 PASS; CV-4 #380 ⑦ "失败 reason" toast 模板 + AL-3 #305 + AL-1b #458 dot UI 视觉锁同精神.
> **#338 cross-grep 反模式遵守**: 既有 CV-1 #347 + CV-4 #380 ⑦ "失败: {reason_label}" toast 模板字面已稳定, 本锁字面跟既有 byte-identical 引用不臆想新词。

---

## 1. 5 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **X2 conflict toast** (artifact 被 agent_B 抢锁, agent_A 收 409) | toast 字面: `"正在被 agent {name} 处理"` byte-identical 跟 **spec §1 立场 ③ + CV-4 #380 ⑦ failed toast 模板精神同源** ({name} 占位 = `ownerName` 跟 spec line 19 同源); 跟 CV-1 CONFLICT_TOAST `"内容已更新, 请刷新查看"` (#347 line 49) 拆死 — CM-5 是 agent↔agent X2, CV-1 是人 vs agent X1, 两 toast 字面不混 | ❌ 不准 "Conflict" / "冲突" / "其他 agent 正在处理" / "请稍后重试" 同义词漂移 (中文 byte-identical 锁); ❌ 不准 raw `iteration_id` UUID 进 toast 文本 (跟 ADM-0 #211 §1.1 raw UUID 隐私同源); ❌ toast 持续 >3s (UX 噪声); ❌ 不准 X2 toast 出现在 X1 路径 (agent vs 人) — agent_id 区分锁死 |
| ② | **hover agent 协作链路 anchor** (AgentManager hover) | DOM: `<span class="agent-collab-link" data-agent-collab-id="{agent_id}" title="正在协作: {agentName}">{agentName}</span>` byte-identical 跟 spec line 53 立场 ⑤ 字面同源 (`"正在协作: {agentName}"` 中文 + 冒号 + 单空格 byte-identical) | ❌ 不准 "Working with" / "Collaborating" / "正在合作" / "联合工作" 同义词; ❌ raw agent_id 不进文本节点 (跟 #211 同源); ❌ 不准 hover anchor 出现在非 collaborator (反约束: 仅同 channel + 同 artifact iterate 链路上的 agent 显 hover); ❌ 不准 admin SPA 显示 hover anchor (admin 不入业务路径, ADM-0 §1.3 红线) |
| ③ | **retry 入口** (X2 conflict 后) | toast 旁右边按钮: `"重新提交"` byte-identical (跟 CV-4 #380 ⑦ "failed state 无重试按钮" 反约束**精神不冲突** — CV-4 iterate 失败 = state 转移锁死, CM-5 X2 conflict = lock 持有者切换的临时态, 走 retry 是合理 UX); 点击 → 重新触发 commit attempt (跟 CV-1.2 既有 commit endpoint 同源, 不开新路径) | ❌ 不准 "Retry" / "再试一次" / "刷新页面" 同义词漂移; ❌ 不准 retry 按钮自动触发 (隐式 bypass, 跟 #11 silent default 反约束精神同源); ❌ 不准 retry 按钮出现在 owner 触发 iterate 路径 (CV-4 立场 ②: owner 触发 + agent 完成 走 CV-1 既有 commit, retry 仅 X2 路径); ❌ 不准 retry 5 次自动触发 (跟 polling spam 反约束同源, 跟 AL-1b #458 ⑦ "polling debounce 5s" 同精神) |
| ④ | **agent silent default** (X2 触发后状态变化) | 反约束: agent_A 收 409 后 — 不发 system message / 不 fanout / 不污染 channel chat 流 (跟 #11 沉默胜于假活物感 + AL-3 #305 + AL-2a #454 ⑦ + AL-1b #458 ⑦ + ADM-1 #483 同精神); 仅 agent_A 自身 SPA UI toast + retry 入口 (UI 单点单源, 跟 dot UI 同精神); X2 路径走 inline UI 不进 messages 流 (跟 CV-4 #380 ⑥ + #382 立场 ⑤ "messages 流不污染 iterate state" 同源) | ❌ 不准 system message broadcast `"agent_A 与 agent_B 冲突"` (跟 #11 silent default 永久锁); ❌ 不准 fanout 给 channel members (这是 agent 内部协作事); ❌ 不准 toast 显示 raw error.message (隐私 + UX, 跟 AL-3 #305 同精神); ❌ 不准 X2 状态进 BPP frame push (CM-5 走 REST 409 + client toast, 不裂 frame namespace, 跟 RT-1 4 frame + BPP-1 9 frame 已锁同源) |
| ⑤ | **owner-first 透明可见** (CM-5.2 立场 ⑤) | DOM: artifact 详情页 + GET /iterations response **不含** `ai_only`/`visibility_scope`/`agent_visible_only` 字面 (跟 #476 CM-5.2 反约束 inline 同源); owner 优先 GET /iterations 拿全 iteration 链路 (含 agent↔agent 协作), agent 不能反查 owner-only iteration | ❌ 不准 server response 含 `ai_only`/`visibility_scope`/`agent_visible_only` 字段 (跟 #476 立场 ⑤ inline 反约束 grep count==0 同源); ❌ 不准 client SPA hide owner-only iteration (反人为隐藏, 跟蓝图 §4.1 隐私承诺精神反向 — owner 看完整, 不是 admin god-mode); ❌ 不准走 `?visibility=` query 参数 (反裂 endpoint, 跟 CV-4 立场 ② "CV-1 commit 单源" 同精神) |

---

## 2. 反向 grep — CM-5.x PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# ① X2 toast 字面 byte-identical (预期 ≥1)
grep -rnE "['\"]正在被 agent \\$\\{.*name.*\\} 处理['\"]|正在被 agent \\{.*\\} 处理" packages/client/src/ 2>/dev/null | grep -v _test  # 预期 ≥1
# ① X2 toast 同义词漂移防御
grep -rnE "['\"](Conflict|冲突|其他 agent 正在处理|请稍后重试)['\"]" packages/client/src/components/Artifact*.tsx 2>/dev/null | grep -v _test
# ② hover 协作链路字面 byte-identical (预期 ≥1)
grep -rnE "title=['\"]正在协作: \\$\\{.*agentName.*\\}['\"]" packages/client/src/components/AgentManager*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
# ② hover anchor 同义词漂移防御
grep -rnE "['\"](Working with|Collaborating|正在合作|联合工作)['\"]" packages/client/src/components/AgentManager*.tsx 2>/dev/null | grep -v _test
# ② DOM data-agent-collab-id 必有 + 不进文本节点 raw UUID (预期 ≥1)
grep -rnE 'data-agent-collab-id' packages/client/src/components/AgentManager*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
# ③ retry 按钮文案 + 同义词漂移 + 自动触发反约束
grep -rnE "['\"]重新提交['\"]" packages/client/src/components/Artifact*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"](Retry|再试一次|刷新页面)['\"]" packages/client/src/components/Artifact*.tsx 2>/dev/null | grep -v _test
grep -rnE 'autoRetry.*X2|setTimeout.*POST.*commit.*409' packages/client/src/ 2>/dev/null | grep -v _test
# ④ X2 状态不进 messages 流 (域隔离, 跟 #382 立场 ⑤ 同源)
grep -rnE 'messages.*x2_conflict|messages.*lock_holder_id' packages/server-go/internal/migrations/ packages/client/src/ 2>/dev/null | grep -v _test
# ④ X2 不裂 frame namespace (RT-1 4 + BPP-1 9 已锁)
grep -rnE 'X2ConflictFrame|LockChangedFrame' packages/server-go/internal/ws/ 2>/dev/null | grep -v _test.go
# ⑤ owner-first 透明 — 反人为隐藏 (跟 #476 立场 ⑤ 反约束同源)
grep -rnE "['\"](ai_only|visibility_scope|agent_visible_only)['\"]" packages/server-go/internal/api/ packages/client/src/ 2>/dev/null | grep -v _test
# ⑤ ?visibility= query 反约束 (反裂 endpoint, 跟 CV-1 commit 单源同源)
grep -rnE "URL\\.Query\\(\\)\\.Get\\(['\"]visibility['\"]\\)|/iterations\\?visibility=" packages/server-go/internal/api/ 2>/dev/null | grep -v _test.go
```

---

## 3. 验收挂钩 (CM-5.3 client PR 必带)

- ① X2 toast e2e: 模拟 agent_A 抢锁 + agent_B 触发 commit → 409 → toast `"正在被 agent {name} 处理"` byte-identical 字面 1.5s 显示 (跟 CV-3 #370 ③ "已复制" toast 1.5s 同精神) + 反向断言无 raw iteration_id UUID 进文本节点
- ② hover agent 协作链路 e2e: AgentManager hover → tooltip `"正在协作: {agentName}"` byte-identical + DOM `data-agent-collab-id="{agent_id}"` attr + 反向断言 admin SPA 不渲染 (admin 不入业务路径)
- ③ retry 入口 e2e: X2 toast 显示后 → 按钮 `"重新提交"` byte-identical + 点击触发新 commit attempt + 反向断言无 5 次自动 retry / 隐式触发
- ④ silent default e2e: X2 触发后反向断言无 system message / 无 fanout / 无 BPP frame push (跟 CM-5.2 #476 立场 ⑤ + 立场 ④ 同源)
- ⑤ owner-first 透明 e2e: owner GET /iterations 返完整链 + 反向断言 response 不含 `ai_only/visibility_scope/agent_visible_only` 字段 (跟 #476 反约束 inline 同源)
- G4.x demo 截屏 3 张归档 (跟 #391 §1 截屏路径锁同源): `docs/qa/screenshots/g4.x-cm5-{x2-conflict-toast,agent-collab-hover,retry-button}.png` 撑 Phase 4 退出闸 demo

---

## 4. 不在范围

- ❌ X2 conflict 走 BPP frame push (RT-1 4 frame + BPP-1 9 frame 已锁不裂, 跟 AL-4 #379 v2 ⑥ + #382 立场 ① + AL-1b #458 ⑥ 同精神)
- ❌ retry 自动触发 (反 #11 silent default + AL-1b polling debounce 同精神)
- ❌ X2 进 messages 流 (域隔离, 跟 #382 立场 ⑤ + CV-4 #380 ⑥ 同源)
- ❌ owner-only iteration 隐藏 client UI (反透明协作精神, 立场 ⑤)
- ❌ admin SPA 显 X2 toast / hover anchor (admin 不入业务路径, ADM-0 §1.3 红线 + ADM-1 #483 强权但不窥视 同源)
- ❌ raw iteration_id UUID 进 toast 文本 (隐私, 跟 #211 同源)
- ❌ X2 conflict count 历史聚合 ("我跟 agent_X 冲突 N 次") — Phase 5+
- ❌ retry 走 PATCH /iterations/:id/state (跟 CV-4 立场 ② "CV-1 commit 单源" 同源, retry 走 CV-1.2 既有 commit endpoint, 不开旁路)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 5 处文案锁 (X2 toast `"正在被 agent {name} 处理"` byte-identical 跟 spec §1 立场 ③ 同源 + CV-4 #380 ⑦ failed toast 模板精神 / hover 协作链路 `"正在协作: {agentName}"` byte-identical 跟 spec line 53 立场 ⑤ 同源 / retry 入口 `"重新提交"` byte-identical / agent silent default 跟 #11 + AL-3/AL-2a/AL-1b/ADM-1 多源同精神 / owner-first 透明可见 跟 #476 立场 ⑤ 反约束 inline 同源) + 11 行反向 grep (含 4 预期 ≥1 + 7 反约束) + G4.x demo 截屏 3 张归档. #338 cross-grep 反模式遵守: 既有 CV-1 #347 + CV-4 #380 toast 模板字面已稳定, 本锁跟既有 byte-identical 引用不臆想新词 |
