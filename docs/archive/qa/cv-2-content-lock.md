# CV-2 锚点对话文案锁 (野马 G3.x demo 预备)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CV-2.x client UI 实施前锁锚点对话 (anchor comment) 文案 + DOM 字面 — 跟 AL-3 #305 / DM-2 #314 / AL-4 #321 / CHN-2 #354 同模式 (用户感知签字 + 文案 byte-identical), 防 CV-2 锚点退化成普通 channel message 同义词。**4 件套并行**, 跟飞马 spec brief / 烈马 acceptance template / 战马A 实施同步起。
> **关联**: `canvas-vision.md` §1.4 (artifact 自带版本) + §1.5 (agent 默认可回锚点评论) + **§1.6 锚点对话钉死为人审产物, 不是 agent 间通信** + §2 v1 不做 (本锁是 v2 入口); CV-1 #346 ArtifactPanel.tsx (锚点 UI 挂在此组件); concept-model §1.3 (协作可以, 扩权不行)。
> **配套**: 飞马 CV-2 spec brief (拆段) + 烈马 CV-2 acceptance template — 一起 review 一起 merge → CV-2 实施基线。
> **#338 cross-grep 反模式遵守**: 反查既有实施 — `ArtifactPanel.tsx` 当前**无锚点对话相关字面** (CV-2 是新功能), `index.css:1137 /* @mention highlight */` 是 mention 不是锚点 — 字面池干净, 本锁直接落定 v0 字面, 后续战马A 实施跟此锁 byte-identical。

---

## 1. 7 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **段落锚点入口** (鼠标 hover artifact 段落右侧出现) | DOM: `<button class="anchor-comment-btn" data-anchor-id="{paragraph_id}" title="评论此段">💬</button>` (icon 锁 💬 + tooltip `"评论此段"` byte-identical) | ❌ 不准 "Comment" / "添加评论" / "回复" / "讨论" 同义词漂移; ❌ 锚点入口仅段落粒度 (蓝图 §1.4 段落锚点), 不是行级 / 字符级 |
| ② | **锚点对话气泡定位** (点 ① 后弹出) | DOM: `<div class="anchor-thread" data-anchor-id="{id}">` 锚定到段落右侧, header 字面 `"段落讨论"` byte-identical (蓝图 §1.6 钉死语义 = 人审产物) | ❌ 不准 "评论区" / "讨论组" / "Comments" / "Thread"; ❌ 气泡 z-index 不准盖住 chat (双支柱 channel-model §1.1); ❌ 不准锚点跟 chat 消息流混排 (是独立 thread, 不污染 chat) |
| ③ | **锚点输入框 placeholder** (owner 写 review) | textarea placeholder: `"针对此段写下你的 review…"` byte-identical (蓝图 §1.6 锚点 = owner review agent 产物) | ❌ 不准 "输入评论" / "Write a comment" / "回复 agent" (前者太泛, 后者把 agent 当对象会诱导 agent-to-agent 锚点 §1.6 反约束) |
| ④ | **agent 锚点回复角标** (agent 回 review 时 — §1.5 默认允许) | DOM: `<span class="anchor-reply-author" data-kind="agent">🤖 {agent_name}</span>` (跟 CV-1 ArtifactPanel.tsx kindBadge 立场 ⑥ 二元 🤖↔👤 同源 byte-identical) | ❌ 不准 "Bot" / "AI" / "Assistant" / 不加角标 (跟 CV-1 #347 line 251 byte-identical 锁); ❌ agent 不能主动起新锚点 thread, **只能回 owner 起的** thread (蓝图 §1.6 钉死) — DOM 反向断言: agent 没有 ① 入口权限 |
| ⑤ | **反约束: agent 不能开锚点** (§1.6 钉死) | agent runtime 调 `POST /api/v1/artifacts/:id/anchors` 路径 → server 端 403 + 错误码 `anchor.create_owner_only` (跟 CV-1 rollback owner-only #347 反向断言模式同根) | ❌ 不准 client UI 给 agent 开 ① hover 入口 (DOM 层就拒); ❌ 不准 server 端放过 agent kind = 'agent' 的 POST anchor 请求; ❌ 不准放过 cross-anchor (一个 agent 回另一个 agent 锚点 — §1.6 钉死人审场景) |
| ⑥ | **锚点关闭文案** (owner 标记 resolved) | 按钮: `"标为已解决"` byte-identical; 已解决 thread DOM `data-resolved="true"` + 视觉降权 (灰底); 反向: `"重新打开"` (resolved → open) byte-identical | ❌ 不准 "Resolve" / "Close" / "完成" / "Done"; ❌ 已解决 thread 不准从 DOM 移除 (蓝图 §1.4 版本历史保留同精神 — 锚点也是 review 历史, agent 默认无删历史权 跟 CV-1 #347 立场 ③ 同模式) |
| ⑦ | **stale 标签文案** (anchor.version < artifact 当前 version, 立场承袭 CV-2 spec #356 立场 ②) | DOM: `<span class="anchor-stale-label" data-anchor-stale="true">锚点指向 v{N}, 文档已更新到 v{M}</span>` byte-identical (跟 #358 acceptance §3.4 同源, `{N}` = anchor.version, `{M}` = artifact 当前 version 占位) | ❌ 不准 "stale" / "outdated" / "过期" / "已失效" 同义词漂移 (中文字面"已更新到"是中性 + 信息性, 不诋毁 anchor 价值, §1.6 锚点是 review 历史不是 bug); ❌ stale anchor 不准从 DOM 移除 (跟 ⑥ 已解决同精神, review 历史保留); ❌ 不准跟 anchor.version 漂移强相关地自动迁移段落 — 蓝图无 v1 OT/CRDT (#356 立场 ② 字面禁) |

---

## 2. 反向 grep — CV-2.x PR merge 后跑, 全部预期 0 命中

```bash
# ① 锚点入口 tooltip 同义词漂移
grep -rnE "['\"](Comment|添加评论|回复|讨论|Comments)['\"]" packages/client/src/components/ArtifactPanel.tsx packages/client/src/components/Anchor*.tsx 2>/dev/null | grep -v _test
# ② 锚点 thread header 同义词漂移
grep -rnE "['\"](评论区|讨论组|Comments|Thread|Discussion)['\"]" packages/client/src/components/Anchor*.tsx 2>/dev/null | grep -v _test
# ③ 输入框 placeholder 同义词漂移
grep -rnE "placeholder=['\"](输入评论|Write a comment|回复 agent|添加评论)['\"]" packages/client/src/ | grep -v _test
# ④ agent 角标同义词漂移 (跟 CV-1 #347 byte-identical 同源)
grep -rnE "['\"](Bot|AI|Assistant)['\"]" packages/client/src/components/Anchor*.tsx 2>/dev/null | grep -v _test
# ⑤ agent 起新锚点 path leak (蓝图 §1.6 钉死人审, 反约束)
grep -rnE "createAnchor.*kind.*=.*['\"]agent['\"]|agent.*POST.*anchors" packages/server-go/internal/api/anchors*.go 2>/dev/null | grep -v _test
# ⑥ 关闭文案同义词漂移 + 已解决 DOM 移除 leak
grep -rnE "['\"](Resolve|Close|完成|Done|删除评论|delete.*anchor)['\"]" packages/client/src/components/Anchor*.tsx 2>/dev/null | grep -v _test
# ⑦ stale 文案同义词漂移 (跟 #358 §3.4 同源)
grep -rnE "['\"](stale|outdated|过期|已失效|已过期)['\"]" packages/client/src/components/Anchor*.tsx 2>/dev/null | grep -v _test
# ⑦ stale anchor DOM 移除 leak (反约束, anchor 是 review 历史保留)
grep -rnE "data-anchor-stale.*remove|removeChild.*anchor.*stale|filter.*anchor.*\\.version.*<" packages/client/src/ 2>/dev/null | grep -v _test
# ⑦ anchor 自动迁移 leak (反约束, 蓝图 v1 无 OT/CRDT, #356 立场 ② 字面禁)
grep -rnE "anchor.*remap|migrateAnchor|anchor.*offset.*recompute|anchor.*shift" packages/server-go/internal/ packages/client/src/ 2>/dev/null | grep -v _test
```

---

## 3. 验收挂钩 (CV-2.x PR 必带)

- ① hover artifact 段落 → DOM 出现 `data-anchor-id` button + tooltip `"评论此段"` byte-identical e2e
- ② 点 ① → DOM 弹出 `<div class="anchor-thread">` + header 字面 `"段落讨论"` byte-identical
- ③ 输入框 placeholder e2e 字面锁
- ④ agent 回锚点 fanout → DOM `data-kind="agent"` + `🤖` 角标 byte-identical (跟 CV-1 #347 line 251 同源单测)
- ⑤ **反向断言三连**: (a) client DOM agent 视角无 ① hover 入口; (b) server agent role POST `/api/v1/artifacts/:id/anchors` → 403 + `anchor.create_owner_only`; (c) cross-anchor (agent 回 agent) 同 403
- ⑥ "标为已解决" / "重新打开" 字面锁 + 已解决 thread `data-resolved="true"` 不从 DOM 移除 + agent 默认无删 anchor history (跟 CV-1 立场 ③ 同根)
- ⑦ stale 标签 e2e: anchor.version < artifact 当前 version → DOM `data-anchor-stale="true"` + 文案 `"锚点指向 v{N}, 文档已更新到 v{M}"` byte-identical 跟 #358 §3.4 同源 + stale anchor 不从 DOM 移除 + 反向断言无自动 offset 迁移 (#356 立场 ② 字面禁)
- G3.x demo 截屏 4 张预备 (跟 G2.4#5 / G2.5 / G2.6 同模式): `docs/qa/screenshots/g3.x-cv2-{anchor-entry,thread-bubble,agent-reply,resolved}.png` (CI Playwright `page.screenshot()`)

---

## 4. 不在范围

- ❌ agent 主动起锚点 thread (蓝图 §1.6 永久钉死人审产物, 不开)
- ❌ 跨 artifact 锚点 (锚点强绑当前 artifact 段落, 不跨)
- ❌ 锚点 mention 第三人 (走 DM-2 #314 mention 路径, 锚点是独立 thread 不复用 mention)
- ❌ 锚点 GC / 历史聚合 (留 v3+, 跟 CV-1 立场 ③ 版本无 GC 同模式)
- ❌ 多 artifact 关联视图 / 无限画布 (蓝图 §2 显式不做, Phase 5+)
- ❌ admin SPA 锚点 god-mode (admin 不入 channel/artifact, ADM-0 §1.3 红线; god-mode 字段白名单不含 anchor body)
- ❌ realtime CRDT 锚点协同编辑 (蓝图 §2 不做, 跟 CV-1 锁 ② 单文档锁同精神 — 锚点串行编辑 last-writer-wins)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 6 处文案锁 (锚点入口 💬 + tooltip "评论此段" + thread header "段落讨论" + placeholder + agent 🤖 角标 byte-identical 跟 CV-1 #347 同源 + agent 起锚反约束 + "标为已解决"/"重新打开") + 6 行反向 grep + G3.x demo 截屏 4 张预备. #338 cross-grep 反模式遵守: 既有实施字面池干净 (CV-2 是新功能), 本锁直接落定 v0, 后续实施跟此锁 byte-identical |
| 2026-04-29 | 野马 | v0.1 patch — 跟 #358 acceptance §3.4 + #356 spec 立场 ② 同步: 加 ⑦ stale 标签字面锁 `"锚点指向 v{N}, 文档已更新到 v{M}"` byte-identical (DOM `data-anchor-stale="true"`, stale anchor 不从 DOM 移除); §2 反向 grep 加 3 行 (stale 同义词漂移 / DOM 移除 leak / 自动 offset 迁移 leak — 锁蓝图 §2 v1 无 OT/CRDT). 立场承袭 #356 立场 ② anchor 挂 artifact_version_id immutable, 不跨版本迁移. 跟 #358 §3.4 e2e DOM grep 同源 |
