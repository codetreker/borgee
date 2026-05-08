# CV-4 iterate UI 文案锁 (野马 G3.4 demo 预备)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CV-4.x client UI 实施前锁 iterate 完整流文案 + DOM 字面 + state 4 态文案 + jsdiff 蓝绿配色 — 跟 AL-3 #305 / DM-2 #314 / AL-4 #321 / CHN-2 #354 / CV-2 #355 / CV-3 #370 同模式 (用户感知签字 + 文案 byte-identical), 防 CV-4 实施时 iterate 退化成普通 commit / state 模糊 / diff 视觉混淆。**4 件套并行**, 跟飞马 #365 spec / 烈马 acceptance template (待派) / 战马A 实施同步起。
> **关联**: `canvas-vision.md` §1.4 (artifact 自带版本历史: agent 每次修改产生一个版本, 人可以回滚) + §1.5 (agent 写内容默认允许); 飞马 CV-4 spec brief #365 §0 ① iteration 独立 entity / §0 ② CV-1 commit 单源 / §0 ③ client jsdiff 不裂 server diff; CV-1 #347 line 251 kindBadge 二元 🤖↔👤 byte-identical; CV-2 #355 立场 ⑤ 反约束三连同精神; AL-1a #249 6 reason codes byte-identical 跟 IterationFailed reason 同源。
> **#338 cross-grep 反模式遵守**: 既有实施字面池干净 (CV-4 是新功能), 既有 ArtifactPanel.tsx (#346) + lib/agent-state.ts REASON_LABELS (#249) 字面已稳定, 本锁字面跟既有 byte-identical 不臆想新词。

---

## 1. 7 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **iterate 触发按钮** (artifact panel 行尾, owner-only) | DOM: `<button class="iterate-btn" data-iteration-target-agent-id="" title="请求 agent 迭代">🔄</button>` (icon 锁 🔄 + tooltip `"请求 agent 迭代"` byte-identical, owner-only `if !isOwner DOM omit` 跟 CV-1 #347 line 254 showRollbackBtn 同模式 defense-in-depth) | ❌ 不准 "Iterate" / "迭代" / "重新生成" / "regenerate" / "再来一次" 同义词漂移; ❌ 非 owner DOM 不能 disable 渲染 — 必须 omit (跟 #347 防御深度同精神); ❌ iterate 按钮在 DM 视图 / non-markdown artifact 上不渲染 (CV-2 §4 反约束承袭, 跟 #355 ⑤ + #363 立场 ① 同源) |
| ② | **iterate 输入框** (intent textarea + agent picker) | textarea placeholder: `"告诉 agent 你希望它做什么…"` byte-identical (蓝图 §1.5 字面 "agent 是同事, 能贡献内容" 同精神 — 输入是协作语境, 非命令式); agent picker label: `"选择 agent"` byte-identical, 候选列表 channel member.kind='agent' 行加 🤖 跟 #347 line 251 byte-identical | ❌ 不准 "Prompt" / "命令" / "请求内容" / "任务描述" 同义词 (协作语境锁); ❌ agent picker 不准列人 (蓝图 §1.5 iterate 是 agent 写内容路径, 人写走普通 commit); ❌ 不准 admin 出现在候选 (admin 不入 channel ADM-0 §1.3 红线) |
| ③ | **iterate state 4 态文案 byte-identical** (DOM `data-iteration-state` + 用户感知文案) | DOM: `<div class="iteration-state" data-iteration-state="{pending|running|completed|failed}">{LABEL}</div>` 4 态字面锁:<br>• `pending` → `"等待 agent 开始…"` + spinner<br>• `running` → `"agent 正在迭代…"` + 进度条 (无具体百分比, agent 不报)<br>• `completed` → `"已生成 v{N}"` + 自动跳新版本视图<br>• `failed` → `"失败: {reason_label}"` + reason_label 走 AL-1a #249 REASON_LABELS byte-identical (改 = 改两边, 跟 AL-3 #305 ③ error 文案模式同源) | ❌ 不准 "Pending/Running/Completed/Failed" 英文; ❌ 不准 "处理中/进行中/成功/出错" 同义词漂移 (跟 #305 "出错: {reason}" → "故障 ({reason_label})" 漂移修复同精神); ❌ failed reason **必须**走 REASON_LABELS (改 reason 字面 = 改 #249 + AL-3 #305 + 此锁三处, byte-identical 单测锁); ❌ state 不准用 emoji 替代文案 |
| ④ | **iterate 完成自动跳新版本 + ArtifactUpdated kindBadge** | iteration completed → 自动 navigate 到新 artifact_version_id 视图; kindBadge 必为 `🤖 {agent_name}` (跟 CV-1 #347 line 251 byte-identical: `kindBadge = committer_kind === 'agent' ? '🤖' : '👤'`); commit message system 通知走 CV-1 既有 fanout 路径 `{agent_name} 更新 {artifact_name} v{n}` byte-identical (跟 artifacts.go:591 byte-identical) | ❌ 不准跳完不渲染 kindBadge (二元 🤖↔👤 永久锁); ❌ 不准 iterate 完成单独发新 fanout 文案 (走 CV-1 既有 commit 路径同源, 跟 #365 立场 ② "CV-1 commit 单源" 字面); ❌ 不准 navigate 到 v(N) 但 dropdown 不更新 (UI 一致性) |
| ⑤ | **diff view tab 文案 + jsdiff 蓝绿配色锁** | tab 文案: `"对比"` byte-identical (单字, 跟 chat/workspace 双 tab 文案精神 — 简洁不啰嗦); diff 视图字面: `"v{N} ↔ v{M}"` 标题 (双向箭头 ↔ 锁); jsdiff 行级配色 byte-identical: 增行 `bg: var(--green-add); color: var(--green-text)` / 删行 `bg: var(--red-del); color: var(--red-text)` / 上下文行无 bg; deep-link `?diff=vN..vM` byte-identical (跟 #365 spec §0 ③ + §1 CV-4.3 字面同源) | ❌ 不准 "对比版本"/"Compare"/"Diff"/"差异" 同义词漂移; ❌ 不准红绿配色不带 ARIA label (a11y — 仅靠颜色辨识 增/删 视觉障碍漏); ❌ 不准 image_link kind 走 jsdiff (走前后缩略图并排, jsdiff 不适用, 跟 #365 §2 字面同源); ❌ 不准 server 端算 diff (走 client jsdiff, 跟 #365 立场 ③ 反约束同源, grep `serverDiff|/api/v1/diff` count==0) |
| ⑥ | **iterate 历史 inline (artifact panel 内, 不进 messages 流)** | artifact panel 加 "迭代历史" 折叠区 (`data-section="iteration-history"`), 列 active + 最近 5 条 iteration (state + intent_text 头 40 字 + completed_at); 反约束: messages 流不渲染 iteration state 进度 | ❌ 不准 "Iteration History"/"修改历史"/"版本日志" 同义词 (历史 vs 版本不混); ❌ messages 流出现 iteration state 进度 = leak (跟 #365 立场 ① "iteration 独立 entity 不污染 messages" + #374 立场 ② 反约束 `messages.*iterate_progress` count==0 同源); ❌ 不准在历史区显示 raw `intent_text` 完整 (头 40 字截断, 跟 #314 fallback DM body_preview 80 字同精神 — 防 UI 噪声 + 隐私) |
| ⑦ | **失败重试反约束** (failed state) | failed iteration UI 仅显示 `"失败: {reason_label}"` + 不显示"重试"按钮 (跟 #365 反约束 ② "iterate 取消/pause/resume 留 v3+, 失败重试 = 重新触发新 iteration" 字面同源); owner 重新触发新 iteration 走 ① iterate 按钮路径 (新 iteration_id, 不复用 failed) | ❌ 不准 "重试"/"Retry"/"重新尝试"/"再试一次" 按钮 (失败状态机锁死, 跟 #365 立场 ① state 转移图反断 `completed→running reject`+`failed→pending reject` 同源); ❌ 不准在 failed 状态自动 retry (隐式触发新 iteration = bypass owner 决策 = 立场 ② 单源 owner 触发字面违反) |

---

## 2. 反向 grep — CV-4.x PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# ① iterate 按钮文案 + owner-only DOM omit (跟 #347 line 254 同模式)
grep -rnE "['\"](Iterate|迭代|重新生成|regenerate|再来一次)['\"]" packages/client/src/components/Artifact*.tsx packages/client/src/components/Iterate*.tsx 2>/dev/null | grep -v _test
grep -rnE 'data-iteration-target-agent-id' packages/client/src/components/Artifact*.tsx packages/client/src/components/Iterate*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
# ② intent textarea placeholder + agent picker label
grep -rnE "['\"](Prompt|命令|请求内容|任务描述)['\"]" packages/client/src/components/Iterate*.tsx 2>/dev/null | grep -v _test
grep -rnE "placeholder=['\"](告诉 agent 你希望它做什么…|选择 agent)['\"]" packages/client/src/components/Iterate*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
# ③ state 4 态英文 + 同义词漂移防御
grep -rnE "['\"](Pending|Running|Completed|Failed)['\"]" packages/client/src/components/Iterate*.tsx 2>/dev/null | grep -v _test
grep -rnE "['\"](处理中|进行中|出错: |成功)['\"]" packages/client/src/components/Iterate*.tsx 2>/dev/null | grep -v _test
# ③ failed reason 必走 REASON_LABELS (跟 AL-1a #249 + AL-3 #305 byte-identical)
grep -rnE 'REASON_LABELS\[' packages/client/src/components/Iterate*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
# ④ kindBadge 二元锁 (跟 CV-1 #347 line 251 byte-identical 同源)
grep -rnE "committer_kind.*===.*['\"]agent['\"]\\s*\\?\\s*['\"]🤖['\"]" packages/client/src/ 2>/dev/null | grep -v _test  # 预期 ≥1
# ⑤ diff view 文案 + 同义词漂移
grep -rnE "['\"](对比版本|Compare|Diff|差异)['\"]" packages/client/src/components/Diff*.tsx packages/client/src/components/Artifact*.tsx 2>/dev/null | grep -v _test
grep -rnE 'data-diff-line=["'"'"'](add|del|context)["'"'"']' packages/client/src/components/Diff*.tsx 2>/dev/null | grep -v _test  # 预期 ≥1 (a11y ARIA 替代仅颜色辨识)
# ⑤ server diff leak (跟 #365 立场 ③ 反约束同源)
grep -rnE 'serverDiff|computeDiff.*server|/api/v1/diff' packages/server-go/internal/api/ 2>/dev/null | grep -v _test
# ⑥ messages 流 iterate state leak (跟 #365 立场 ① + #374/#378 立场 ②/⑤ 同源)
grep -rnE 'messages.*iterate_progress|messages.*iteration_state|MessageList.*iteration' packages/client/src/ 2>/dev/null | grep -v _test
# ⑦ 重试按钮反约束 (失败状态机锁死)
grep -rnE "['\"](重试|Retry|重新尝试|再试一次)['\"]" packages/client/src/components/Iterate*.tsx 2>/dev/null | grep -v _test
# ⑦ 自动重试 leak (隐式 bypass owner 决策)
grep -rnE 'autoRetry.*iteration|setTimeout.*POST.*iterate.*failed' packages/client/src/ 2>/dev/null | grep -v _test
```

---

## 3. 验收挂钩 (CV-4.x PR 必带)

- ① iterate 按钮 e2e: owner 视角 DOM 出现 + 非 owner DOM omit (count==0) + DM 视图 / non-markdown artifact 不渲染反向断言
- ② intent textarea + agent picker e2e: placeholder 字面锁 + 候选列表 agent-only (人/admin 反向断言不在)
- ③ state 4 态 vitest table-driven: `data-iteration-state` 4 enum + 文案 byte-identical + failed reason 走 REASON_LABELS (改 reason = 改六处单测锁: AL-1a #249 + AL-3 #305 + CV-4 本 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461)
- ④ iteration completed e2e: 自动 navigate 到新 version + kindBadge 🤖 byte-identical (跟 CV-1 #347 line 251 同源单测) + 走 CV-1 既有 fanout 不另发 (#365 立场 ② 单源)
- ⑤ diff view e2e: tab 文案 "对比" byte-identical + jsdiff 行级蓝绿 + ARIA label 反向断言 + deep-link `?diff=vN..vM` 进对比模式 + image_link kind fallback 缩略图并排
- ⑥ iteration history inline e2e: 折叠区 `data-section="iteration-history"` + intent_text 头 40 字截断 + messages 流反向断言无 iteration state 进度
- ⑦ failed state e2e: UI 仅 "失败: {reason_label}" + 反向断言无 "重试" 按钮 + owner 重新触发走 ① 路径生成新 iteration_id (反向断言不复用 failed iteration_id)
- G3.4 demo 截屏 4 张归档 (跟 G2.4#5 / G2.5 / G2.6 / G3.x 同模式): `docs/qa/screenshots/g3.4-cv4-{iterate-trigger,running-state,completed-newversion,diff-view}.png` (CI Playwright `page.screenshot()`)

---

## 4. 不在范围

- ❌ iterate 取消 / pause / resume (蓝图无, #365 反约束 ②)
- ❌ iterate 历史聚合视图 ("我的 iteration 列表") — Phase 5+
- ❌ multi-agent 协作 iterate (一 iteration = 一 target_agent_id, #365 反约束 ④)
- ❌ iterate intent 模板 / 预设 prompt — Phase 5+
- ❌ server 端 diff 算法 (#365 立场 ③ + #370 §3 反约束 grep 同源 — CRDT 巨坑同精神)
- ❌ admin SPA iteration god-mode — admin 不入 channel + intent_text 含 user 输入 (ADM-0 §1.3 红线 + #365 反约束 ⑦)
- ❌ batch iterate 跨 artifact (一 iteration 锁单 artifact_id, #365 反约束 ⑧)
- ❌ image_link / code kind 走 jsdiff — 走前后缩略图并排 / 代码仍走 jsdiff 但带 prism 染色 (跟 CV-3 立场 ② 同精神)
- ❌ iteration state push frame 改字段 (BPP-1 #304 envelope CI lint 自动闸, IterationStateChangedFrame 9 字段 byte-identical 锁)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 7 处文案锁 (iterate 按钮 🔄 "请求 agent 迭代" owner-only DOM omit + intent textarea "告诉 agent 你希望它做什么…" + state 4 态字面 byte-identical 跟 AL-1a #249 REASON_LABELS 同源 + completed 自动跳新版 + kindBadge 🤖 跟 CV-1 #347 line 251 byte-identical + diff view "对比" 蓝绿配色 a11y ARIA label + iteration history inline `data-section` + failed state 无重试按钮反约束) + 14 行反向 grep (含 4 预期 ≥1 + 10 反约束) + G3.4 demo 4 张截屏预备. #338 cross-grep 反模式遵守: 既有 ArtifactPanel/REASON_LABELS 字面已稳定, 本锁跟既有 byte-identical |
| 2026-04-29 | 野马 | v0.x patch — cross-milestone reason count audit (跟 #461 同模式 follow-up): "五处单测锁"/"四处单测锁"/"三处单测锁" → "六处单测锁" (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461); CHN-4 stance ④ "6 源" → "7 源" 跟 body 列举 7 源对齐. 跟 #339/#393/#387/#461 follow-up patch 同模式, 历史干净 |
