# DM-2 文案锁 (野马 G2.6 demo 预备)

> **状态**: v0 (野马, 2026-04-28)
> **目的**: DM-2.3 client UI 实施前锁 mention 渲染 + 候选列表 + 离线 fallback DM 文案 + 发送方 UI 反约束 — 跟 G2.4 demo #5 (#275) + AL-3 #305 同模式 (用户感知签字 + 文案 byte-identical), 防 DM-2 实施时文案漂移。
> **关联**: 飞马 DM-2 spec #312 §0 立场 ①②③; 烈马 #293 acceptance §2.2/§3.1/§3.3; 野马 #211 ADM-0 §1.1 永不暴露 UUID; concept-model §4.1 (4-27 决策选 B 实用主义)。

---

## 1. 4 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **mention 渲染** (消息流) | DOM: `<span data-mention-id="{uuid}" class="mention">@{display_name}</span>` (raw UUID 仅 `data-mention-id` attr, **不进文本节点**) | ❌ 文本节点 grep raw UUID (`/[0-9a-f]{8}-[0-9a-f]{4}/` regex) count==0 (跟 #211 ADM-0 §1.1 同根) |
| ② | **mention 候选列表** (textarea `@` 触发) | tooltip / placeholder: `"输入 @ 提到 channel 成员…"` byte-identical (跟 #377 spec §0 ① + #388 实施 + #293 §3.2 同源, 改 = 改四处单测锁); 候选行 = `@{display_name}` + agent 加 🤖 badge (人不加, 立场 ⑥ agent=同事) | ❌ 不准 "Mention" / "提及" / "@提到" / "@他" / "@后输入 ID 或 name" 同义词漂移 (`"@后输入 ID 或 name"` 是 v0 草稿字面已废, post-#388 实施跟 #377 spec 字面对齐); ❌ 候选回填 textarea 时是 `@<user_id>` token, **不是** display_name (防同名歧义, #312 立场 ① + #293 §3.2) |
| ③ | **离线 fallback system DM** (target agent offline 时 owner 收到) | message body byte-identical: `"{agent_name} 当前离线，#{channel} 中有人 @ 了它，你可能需要处理"` (`{agent_name}` / `{channel}` 占位, 其余字面锁; 跟 #293 §2.2 + #211 + concept-model §4.1 例句三处一致) | ❌ payload 不含 raw message body 字符串 (隐私 §13, 反向断言 grep); ❌ 不准 "{agent_name} 不在线" / "已离线" (那是 AL-3 #305 presence dot 文案, 此处是 fallback DM, 文案不复用); kind=`system` + sender_id=`'system'` |
| ④ | **发送方 UI 反约束** (owner 发 mention, target 离线时) | 发送方界面 **无任何离线提示** (无 toast / inline / banner) — fallback 是 owner 后台事, 不污染 mention 发送方流 (#293 §3.3 反向断言) | ❌ DOM 不准出现 `"{agent_name} 离线"` / `"已发送但 agent 不在线"` / `"Pending"` 等; 防 owner 焦虑 — 发了就发了, AL-3 presence dot 显 ⚫ 已经够 |

---

## 2. 反向 grep — DM-2.3 PR merge 后跑, 全部预期 0 命中

```bash
# ① raw UUID 不进 DOM 文本节点 (仅 data-mention-id attr)
grep -rnE "textContent.*[0-9a-f]{8}-[0-9a-f]{4}|innerText.*[0-9a-f]{8}-" packages/client/src/components/Message*.tsx | grep -v _test
# ② 候选列表 tooltip 同义词漂移防御
grep -rnE "['\"](Mention|提及|@提到|@他)['\"]" packages/client/src/ | grep -v _test
# ② 候选回填不准是 display_name (必须 @<user_id>)
grep -rnE "insertMention.*display_name|@\\$\\{.*\\.display_name\\}" packages/client/src/ | grep -v _test
# ③ fallback DM 文案 byte-identical (server 端字面锁, 不准漂)
grep -rnE "当前离线，#.*中有人 @ 了它，你可能需要处理" packages/server-go/internal/api/messages.go | grep -v _test
# ④ 发送方 UI 不准出现离线提示 (反约束)
grep -rnE "['\"](.*离线.*已发送|Pending.*offline|Sent.*offline|未送达)['\"]" packages/client/src/components/Mention*.tsx packages/client/src/components/Message*.tsx | grep -v _test
```

---

## 3. 验收挂钩 (DM-2.3 PR 必带)

- ① DOM `data-mention-id` attr 必有 + raw UUID 文本节点 grep 0 (跟 #293 §3.1 同模式)
- ② 候选列表 tooltip 字面 + agent 🤖 badge + 候选回填 token 测试 (跟 #293 §3.2 同模式)
- ③ fallback DM 字面锁 server 端 grep ≥1 + payload 不含 raw body 反向断言 (跟 #293 §2.2 同模式)
- ④ 发送方 UI 反向断言 e2e (sniff DOM 无 toast/inline 离线提示, 跟 #293 §3.3 同模式)
- G2.6 demo 截屏 5 张预备 (跟 G2.4 / G2.5 同模式): `docs/qa/screenshots/g2.6-mention-{render,candidate,offline-fallback-dm,sender-no-hint,online-ping}.png` (CI Playwright 主动 `page.screenshot()`)

---

## 4. 不在范围

- ❌ `@channel` mention (留 DM-3); ❌ mention 搜索 / 历史聚合 (Phase 5+); ❌ batch mention; ❌ mention 撤回
- ❌ admin SPA mention god-mode (admin 不入 channel, ADM-0 §1.3 红线)
- ❌ 跨 org 邀请审批 UI (走 §4.2 `agent_invitations`, ADM-1/CHN-2 落地)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 4 处文案锁 (mention 渲染 + 候选 tooltip + fallback DM byte-identical + 发送方 UI 反约束) + 5 行反向 grep + G2.6 demo 截屏 5 张预备 |
| 2026-04-29 | 野马 | v0.1 patch — 修 ② placeholder drift: `"@后输入 ID 或 name"` (v0 草稿字面) → `"输入 @ 提到 channel 成员…"` byte-identical 跟 #377 DM-2.3 spec §0 ① + #388 实施 + #293 §3.2 同源 (四处单测锁); 反向 grep 加 `"@后输入 ID 或 name"` 防 v0 字面 leak. 跟 #336/#338 cross-grep drift fix 同模式 follow-up patch (历史干净, 不在原 v0 PR 加 commit). acceptance review subagent (post-#388) 抓出 |
