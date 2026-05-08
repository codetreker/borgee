# CV-4 立场反查表 (artifact iterate 完整流, agent 多版本协作 orchestration)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CV-4.x 实施 PR 直接吃此表为 acceptance; 飞马 spec #365 (锚) / 烈马 acceptance template (待派) / 战马A 拆段实施 review 拿此表反查立场漂移. 一句话立场 + §X.Y 锚 + 反约束 (X 是, Y 不是) + v0/v1.
> **关联**: `canvas-vision.md` §1.4 ("artifact 自带版本历史: agent 每次修改产生一个版本, 人可以回滚") + §1.5 ("agent 写内容默认允许 / 创建新 artifact 默认允许") + §2 v1 做清单 ("agent 可 iterate, 再次写入触发新版本"); 飞马 #365 spec brief §0 ①②③ (iteration 独立 entity / CV-1 commit 单源 / client jsdiff 不裂 server diff); 野马 #380 文案锁 7 处字面 + state 4 态 byte-identical; 烈马 #293 acceptance + AL-1a #249 REASON_LABELS 八处单测锁.
> **依赖**: CV-1 ✅ (#334+#342+#346+#348) commit/rollback 端点已落; RT-1 ✅ (#290+#292+#296) cursor 单调发号; AL-4 spec ✅ #379 v2 stub 接口前置就位.
> **#338 cross-grep 反模式遵守**: CV-4 是新 milestone (artifact_iterations 表新建), 既有 ArtifactPanel #346 + REASON_LABELS #249 字面已稳定, 立场跟既有 byte-identical 引用不臆想新词.

---

## 1. CV-4 立场反查表 (iterate 完整流)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | spec #365 §0 ① + 蓝图 §1.4 | **iteration 是独立 entity, 不污染 messages / artifact_versions** | **是** `artifact_iterations` 表锁 request lifecycle (`{id, artifact_id, requested_by, intent_text, target_agent_id, state, created_artifact_version_id NULL, error_reason NULL, created_at, completed_at NULL}`) + state ENUM 4 态 `('pending','running','completed','failed')`; **不是** messages 表加 `iteration_id` 列 (mention 路径走 DM-2 已锁); **不是** artifact_versions 加 iteration_id 反指 (artifact_versions 是 v0 immutable append, 跟 CV-1 #347 立场 ③ 同源不动 schema) | v0: 独立表; v1 同永久拆死 |
| ② | spec #365 §0 ② + CV-1 #347 立场 ⑦ | **owner 触发 iterate, agent 完成时 commit 走 CV-1 既有端点 (server-side 单源)** | **是** `POST /artifacts/:id/iterate` body `{intent_text, target_agent_id}` 创 iteration_id state='pending'; agent runtime commit artifact 时**走 CV-1 既有 `POST /artifacts/:id/commits` 端点带 query `?iteration_id=<uuid>`** server 反查回填 `created_artifact_version_id` + state='completed' 一原子事务; **不是** `/iterations/:id/commit` 旁路 endpoint (CV-1 commit 路径单源, 跟 CV-3 #363 立场 ① "enum 扩不裂表" 同精神); **不是** rollback 挂 iteration_id (rollback 走 CV-1 owner-only 跟 iterate 路径互不干涉, #347 立场 ⑦ 同源) | v0: 单源 commit; v1 同 |
| ③ | spec #365 §0 ③ + 蓝图 §2 反约束 | **diff view = client jsdiff 行级, 不裂 schema 不裂 endpoint** (CRDT 巨坑同源) | **是** `<ArtifactPanel>` 加 "对比" tab 走 client jsdiff (跟 #380 ⑤ 文案锁 byte-identical); 加 `?diff=v3..v2` deep-link query; **不是** server 端算 diff (CRDT 巨坑同源, 蓝图 §2 字面禁); **不是** 存 diff 缓存 (查时即算 ≤500ms 实测够 markdown 数 KB); **不是** image_link kind 走 jsdiff (走前后缩略图并排, 跟 CV-3 #363 §2 + #380 ⑤ 同源 fallback) | v0: client jsdiff + 缩略图 fallback; v1 同 |
| ④ | spec #365 §1 + AL-1a #249 6 reason | **state 4 态机锁死 + reason 走 REASON_LABELS 八处单测锁** (改 reason = 改八处) | **是** state 转移图: `pending→running` (agent 接管) / `pending→failed` (timeout / runtime fail-closed) / `running→completed` (commit ok) / `running→failed` (runtime error); state CHECK reject 'unknown'; failed reason ∈ AL-1a 6 reason byte-identical (`api_key_invalid|quota_exceeded|network_unreachable|runtime_crashed|runtime_timeout|unknown`) **八处单测锁** (AL-1a #249 + AL-3 #305 + 本立场 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461); **不是** state 回退 (`completed→running reject` / `failed→pending reject`); **不是** failed 自动 retry (隐式 bypass owner 决策违反立场 ②); **不是** intent_text 模板 (留 v3+) | v0: 4 态机 + 6 reason; v1 同, runtime 真接管走 AL-4 落地后 |
| ⑤ | spec #365 §0 + 文案锁 #380 ⑥ + #374/#378 立场 ②/⑤ | **iterate 进度仅 artifact panel inline, 不污染 messages 流** (域隔离永久锁) | **是** iteration history inline `<div data-section="iteration-history">` 在 artifact panel 内, 列 active + 最近 5 条 (state + intent_text 头 40 字截断 + completed_at); messages 流不渲染 iteration state 进度; **不是** messages 流 inline iterate progress (跟 #374/#378 立场 ②/⑤ + #380 ⑥ 反约束 grep `messages.*iterate_progress|iteration_state` count==0 同源); **不是** intent_text 完整渲染 (头 40 字截断, 跟 #314 fallback DM body_preview 80 字同精神 — 防 UI 噪声 + 隐私) | v0/v1 同 — 域隔离永久 |
| ⑥ | spec #365 §0 ② + CV-1 #347 立场 ⑦ + 文案锁 #380 ① | **iterate 触发 owner-only DOM omit (defense-in-depth)** | **是** iterate 按钮 🔄 跟 CV-1 #347 line 254 showRollbackBtn 同模式 — 非 owner DOM omit 不 disable (defense-in-depth); 跟 DM 视图 / non-markdown artifact 反约束承袭 (CV-2 §4 留 v3+, 跟 #355 ⑤ + #363 立场 ① 同源 server 端 `anchor.unsupported_artifact_kind` 防御); **不是** disable 渲染 (omit 才安全, 跟 #347 立场 ⑦ 同源); **不是** workspace tab 外其他视图渲染 iterate 入口 (跟 #374/#378 立场 ② 双 tab 不交叉同源) | v0/v1 同 |
| ⑦ | spec #365 §2 + ADM-0 §1.3 红线 | **admin SPA 不入 iteration god-mode (intent_text 含 user 输入是隐私字段)** | **是** admin god-mode endpoint 字段白名单**不含** `intent_text` (跟 ADM-0 §1.3 红线 + AL-3 #303 ⑦ 字段白名单 + AL-4 #379 v2 §2 + ADM-0 ⑦ 同模式); admin 仅看 iteration 元数据 (state / created_at / completed_at), 不看 intent_text raw 文本; **不是** admin 写 iterate 路径 (跟 CV-1 rollback / CV-2 anchor / CHN-3 layout 同源 admin 不入业务路径); **不是** intent_text 进 push frame (走 GET /iterations/:id 拉, push frame 仅 state 信号) | v0/v1 永久锁 |

---

## 2. 黑名单 grep — CV-4.x 实施 PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# 立场 ① iteration 不污染 messages / artifact_versions
grep -rnE 'ALTER TABLE messages.*ADD.*iteration_id|ALTER TABLE artifact_versions.*ADD.*iteration_id' packages/server-go/internal/migrations/ | grep -v _test.go
# 立场 ② CV-1 commit 单源 (不开旁路 endpoint)
grep -rnE 'POST.*\\/iterations\\/.*\\/commit|iteration_commit_endpoint|/api/v1/iterations/.*/commit' packages/server-go/internal/api/ | grep -v _test.go
# 立场 ② commit?iteration_id query 锁 (预期 ≥1 — 跟 #365 §1 CV-4.2 字面)
grep -rnE 'iteration_id.*[?&]|query.*iteration_id|r\\.URL\\.Query\\(\\)\\.Get\\("iteration_id"' packages/server-go/internal/api/artifacts.go 2>/dev/null | grep -v _test.go  # 预期 ≥1
# 立场 ③ server 不算 diff (CRDT 巨坑同源)
grep -rnE 'serverDiff|computeDiff.*server|/api/v1/diff|POST.*\\/diff' packages/server-go/internal/api/ | grep -v _test.go
# 立场 ③ jsdiff 走 client (预期 ≥1)
grep -rnE 'jsdiff|diffLines' packages/client/ 2>/dev/null | grep -v _test  # 预期 ≥1
# 立场 ④ state 回退 reject + reason 走 REASON_LABELS
grep -rnE 'completed.*->.*running|state.*backward|UPDATE.*iterations.*state.*pending.*WHERE.*completed' packages/server-go/internal/api/ | grep -v _test.go
grep -rnE 'autoRetry.*iteration|setTimeout.*POST.*iterate.*failed' packages/client/src/ 2>/dev/null | grep -v _test
# 立场 ⑤ messages 流不渲染 iterate 进度 (4+ 源同根: #365/#374/#378/#380)
grep -rnE 'messages.*iterate_progress|messages.*iteration_state|MessageList.*iteration' packages/client/src/ 2>/dev/null | grep -v _test
# 立场 ⑥ owner-only DOM omit (跟 #347 line 254 同模式, 预期 ≥1)
grep -rnE 'showIterateBtn.*=.*isOwner|iterate-btn.*data-iteration-target-agent-id' packages/client/src/components/ 2>/dev/null | grep -v _test  # 预期 ≥1
# 立场 ⑦ admin god-mode 不返 intent_text (字段白名单反断)
grep -rnE 'intent_text.*admin|admin.*intent_text|GodModeFields.*intent_text' packages/server-go/internal/api/admin*.go 2>/dev/null | grep -v _test.go
```

---

## 3. 不在 CV-4 范围 (避免 PR 膨胀)

- ❌ CRDT 多人实时编辑 (蓝图 §2 字面 "CRDT 巨坑"; iterate 仍是顺序 append, 一人一锁)
- ❌ iterate 取消 / pause / resume (留 v3+, 失败重试 = 重新触发新 iteration, #380 ⑦ 同源)
- ❌ iterate 历史聚合视图 ("我的 iteration 列表") — Phase 5+
- ❌ multi-agent 协作 iterate (一 iteration = 一 target_agent_id; 多 agent 协作留 v3+)
- ❌ iterate intent 模板 / 预设 prompt — Phase 5+
- ❌ server 端 diff 算法 (立场 ③ + 蓝图 §2 不做)
- ❌ admin SPA iteration god-mode (intent_text 含 user 输入, ADM-0 §1.3 红线)
- ❌ batch iterate 跨 artifact (一 iteration 锁单 artifact_id)

---

## 4. 验收挂钩

- CV-4.1 schema PR (v=18): 立场 ①④ — `artifact_iterations` 表 + 4 态 CHECK + 反向断言 messages/artifact_versions 不加 iteration_id 列
- CV-4.2 server PR: 立场 ②④⑦ — `POST /iterate` owner-only + `?iteration_id` query 命中 atomic UPDATE + state 转移图反断 + admin god-mode 字段白名单不含 intent_text + IterationStateChangedFrame 9 字段 byte-identical (跟 RT-1=7/AnchorComment=10/MentionPushed=8 共序)
- CV-4.3 client PR: 立场 ③⑤⑥ — iterate 按钮 owner-only DOM omit 跟 #347 line 254 同模式 + intent textarea + state 4 态 inline (跟 #380 ③ byte-identical) + diff tab "对比" + jsdiff 蓝绿配色 + ARIA label + deep-link `?diff=vN..vM`
- CV-4 entry 闸: 立场 ①-⑦ 全锚 + §2 黑名单 grep 全 0 (除标 ≥1) + reason 八处单测锁 (AL-1a #249 + AL-3 #305 + 本 stance + AL-2a #454 + AL-1b #458 + AL-4 #387/#461) + IterationStateChangedFrame 9 字段 byte-identical 跟 RT-1 共序

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 7 立场 (iteration 独立 entity 不污染 messages/versions / CV-1 commit 单源不开旁路 / client jsdiff 不裂 server diff / state 4 态机 + reason 走 REASON_LABELS 八处单测锁 / iterate 进度仅 artifact panel inline 域隔离 / iterate 按钮 owner-only DOM omit 跟 #347 line 254 同模式 / admin god-mode 不返 intent_text 字段白名单反断) 承袭 #365 spec 3 立场 + #380 文案锁拆细 + 跨 milestone byte-identical 锁; 9 行反向 grep (含 7 反约束 + 2 预期 ≥1) + 8 项不在范围 + 验收挂钩三段对齐 + reason 八处单测锁 (#249 + AL-3 #305 + 本 stance + AL-4 #379 v2 = 四处). #338 cross-grep 反模式遵守 (CV-4 新 milestone, 既有 ArtifactPanel + REASON_LABELS 字面引用 byte-identical) |
| 2026-04-29 | 野马 | v0.x patch — cross-milestone reason count audit (跟 #461 同模式 follow-up): "五处单测锁"/"四处单测锁"/"八处单测锁" → "八处单测锁" (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461); CHN-4 stance ④ "6 源" → "7 源" 跟 body 列举 7 源对齐. 跟 #339/#393/#387/#461 follow-up patch 同模式, 历史干净 |
