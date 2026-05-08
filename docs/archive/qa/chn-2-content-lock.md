# CHN-2 DM 文案锁 (野马 G3.x demo 预备)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CHN-2.x client UI 实施前锁 DM 文案 + DOM 字面 — 跟 AL-3 #305 / DM-2 #314 / AL-4 #321 同模式 (用户感知签字 + 文案 byte-identical), 防 CHN-2 实施时把 DM 退化成 "频道" 同义词。**4 件套并行**, 跟飞马 spec brief / 烈马 acceptance template / 战马A 实施同步起。
> **关联**: `channel-model.md` §1.2 (DM 概念独立) + §3.2 (UI 视觉与交互与 channel 明确不同 + workspace 入口需显式禁用) + §1.1 双支柱; DM-2 #314 mention/fallback 文案锁 (`私信` 字面同源) + concept-model §4.1。
> **配套**: 飞马 CHN-2 spec brief (拆段) + 烈马 CHN-2 acceptance template — 一起 review 一起 merge → CHN-2 实施基线。
> **#338 cross-grep 反模式遵守**: 反查既有实施 (Sidebar.tsx:396/411, commands/builtins.ts:63) 字面 `"私信"` 已稳定, 本锁字面跟既有实施 byte-identical, 不臆想新词。

---

## 1. 5 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **侧边栏分组标题** (DM 列表区) | `<div class="online-header" data-kind="dm">私信</div>` (跟 `Sidebar.tsx:396` 既有字面 byte-identical, **不改** "DM" / "Direct Message" / "对话" 同义词; `data-kind="dm"` attr 跟 #353 acceptance §0 ⑤ + §3.1 同源, channel 列表对侧 `data-kind="channel"` 拆死视觉混淆) | ❌ 同分组下不准混入 channel 行 (filter 严格按 `type === 'dm'`); ❌ 不准用 "聊天" / "私聊" / "对话" / "Chats"; ❌ `data-kind` attr 不准漏 (#353 §3.1 e2e DOM grep 命中 ≥1) |
| ② | **DM 行 hover tooltip** | ``title={`私信 ${user.display_name}`}`` (跟 `Sidebar.tsx:411` byte-identical, `${display_name}` 占位; raw user_id 仅 `data-user-id` attr, **不进文本**) | ❌ tooltip 文本节点 grep raw UUID count==0 (跟 #211 ADM-0 §1.1 同根); ❌ 不准 `"@${display_name}"` (那是 mention 候选 #314 ②, 不是 DM tooltip) |
| ③ | **slash command 描述** | `description: '打开与用户的私信'` (跟 `commands/builtins.ts:63` byte-identical) | ❌ 不准 `'发起对话'` / `'新建 DM'` / `'创建私聊'` (CHN-2 不开 DM 创建命令面板入口, DM 由侧边栏在线用户点开生成) |
| ④ | **DM 视图反约束 — 无 workspace tab** (跟 channel 视觉拆死) | DM 视图 DOM **不渲染** workspace tab (`<button data-tab="workspace">` 不出现), 也不渲染 channel topic banner / member list / 邀请按钮 (`channel-model.md` §3.2 显式禁用) | ❌ DM 视图 DOM 出现 `data-tab="workspace"` 即视为 leak; ❌ DM 视图出现 "添加成员" / "邀请" / "话题" / "Topic" 按钮即漂移 (跟 channel 拆死) |
| ⑤ | **DM "升级"提示反约束** (尝试 mention 第 3 人时) | DM 永远 2 人 — UI 禁止在 DM 内输入 `@<3rd_user>` 触发 mention 候选 (候选列表为空 + placeholder `"私信仅限两人, 想加人请新建频道"` byte-identical) | ❌ 不准 "升级为频道" / "Convert to channel" / "Upgrade DM" (蓝图 §1.2: "想加人就**新建** channel 把双方拉进去" — 是新建, 不是 DM 转换) |

---

## 2. 反向 grep — CHN-2.x PR merge 后跑, 全部预期 0 命中

```bash
# ① 分组标题同义词漂移 (私信 是唯一字面)
grep -rnE "['\"](DM|Direct Message|私聊|对话框|Chats|聊天)['\"]" packages/client/src/components/Sidebar.tsx | grep -v _test
# ① data-kind attr 必有 (反向预期 ≥1 — 跟 #353 §3.1 同源, 漏 = 视觉混淆)
grep -nE 'data-kind=["'"'"']dm["'"'"']' packages/client/src/components/Sidebar.tsx | grep -v _test  # 预期 ≥1, 漏视为漂
# ② DM tooltip 不准把 raw UUID 进文本
grep -rnE "title=\\{?\\`?私信 \\$\\{[a-z_]*\\.id" packages/client/src/ | grep -v _test
# ③ DM 创建命令同义词漂移
grep -rnE "['\"](发起对话|新建 DM|创建私聊|新对话|Start chat)['\"]" packages/client/src/commands/ | grep -v _test
# ④ DM 视图 workspace leak (蓝图 §3.2 显式禁用)
grep -rnE "data-tab=['\"]workspace['\"]" packages/client/src/components/DmView*.tsx | grep -v _test
# ④ DM 视图 channel-only 控件 leak
grep -rnE "['\"](添加成员|邀请|话题|Topic|invite)['\"]" packages/client/src/components/DmView*.tsx | grep -v _test
# ⑤ DM 升级 / 转换 同义词漂移 (蓝图 §1.2 是新建不是升级)
grep -rnE "['\"](升级为频道|Convert to channel|Upgrade DM|转为频道)['\"]" packages/client/src/ | grep -v _test
```

---

## 3. 验收挂钩 (CHN-2.x PR 必带)

- ① `Sidebar.tsx:396` `"私信"` 字面保持不动 + `data-kind="dm"` attr 必有 (改 = 改两边: 此锁 + Sidebar — byte-identical 单测锁; e2e DOM `[data-kind="dm"]` count≥1 + `[data-kind="channel"]` 在 channel 列表 count≥1 拆视觉混淆, 跟 #353 §3.1 同源)
- ② DM tooltip e2e: hover DM row → tooltip 文本 `"私信 ${display_name}"` + DOM `data-user-id` attr 存在 + 文本节点无 raw UUID
- ③ `/dm` slash command 描述 grep 命中 1 + ② 同义词反向 grep 0
- ④ DM 视图 e2e: open DM → DOM 反向断言无 `data-tab="workspace"` / "添加成员" 按钮 / "话题" banner (跟 §3.2 显式禁用呼应)
- ⑤ DM `@` 候选 e2e: input `@<3rd_user>` → 候选列表空 + placeholder 字面 byte-identical
- G3.x demo 截屏 3 张预备 (跟 G2.4#5 / G2.5 / G2.6 同模式): `docs/qa/screenshots/g3.x-dm-{sidebar-section,view-no-workspace,mention-third-blocked}.png` (CI Playwright `page.screenshot()`)

---

## 4. 不在范围

- ❌ DM 群聊化 / 升级为 channel (蓝图 §1.2 永久不开, "想加人就**新建** channel"); ❌ DM workspace 入口 (蓝图 §3.2 显式禁用)
- ❌ DM 离线 fallback 文案 (走 DM-2 #314 ③, 不在本锁); ❌ mention 候选 / 渲染 (走 DM-2 #314 ①②, 不在本锁)
- ❌ admin SPA DM god-mode (admin 不入 channel/DM, ADM-0 §1.3 红线)
- ❌ DM 历史搜索 / 归档 (Phase 5+)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 5 处文案锁 (侧边栏 `"私信"` + tooltip + slash command + workspace 反约束 + 升级反约束) + 6 行反向 grep + G3.x demo 截屏 3 张预备. 跟既有实施 cross-grep (#338 反模式遵守): `Sidebar.tsx:396/411` + `commands/builtins.ts:63` 字面 byte-identical |
| 2026-04-29 | 野马 | v0.1 patch — 跟 #353 acceptance §0 ⑤ + §3.1 同步: ① 加 `data-kind="dm"` DOM attr (channel 列表对侧 `data-kind="channel"`, 拆视觉混淆); §2 反向 grep 加预期 ≥1 行锚 `data-kind="dm"` attr 必有, 漏 = 视为漂 (实施 PR 写时跟 acceptance §3.1 e2e DOM grep byte-identical 同源). 字面跟 #353 §3.1 `'data-kind="dm"' count≥1` byte-identical 锁 |
