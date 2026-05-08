# AL-3 presence dot UI 文案锁 (野马 G2.5 demo 预备)

> **状态**: v0 (野马, 2026-04-28)
> **目的**: AL-3.3 client UI 实施前锁 presence dot 4 状态文案 + DOM 字面 — 跟 G2.4 demo screen #5 (#275) 同精神 (用户感知签字 + 文案 byte-identical), 防 AL-3 实施时文案漂移; 配 #303 立场反查 ⑤⑥ + #302 acceptance §3。
> **关联**: `agent-lifecycle.md` §2.3 三态决议 + §11 文案守 ("已离线" 不模糊灰); `agent/state.go` Reason* 6 codes (#249); DM-2 #293 §2.2 fallback 文案锁同模式。

---

## 1. presence dot 4 状态 — 文案 + DOM 字面锁

| 状态 | dot | tooltip 文案 (字面锁, byte-identical) | DOM `data-presence` | 触发 |
|------|-----|-----|-----|------|
| **online** | 🟢 | `"在线"` | `online` | `presence.IsOnline(agent_id) == true` (WS / plugin / poll 任一活) |
| **offline** | ⚫ | `"已离线"` | `offline` | 60s 无心跳 (跟 #303 立场 ⑤ 时序锁), 跟 DM-2 #293 §2.2 fallback 文案同根 |
| **error** | 🔴 | `"故障 ({reason_label})"` (跟 `lib/agent-state.ts` `describeAgentState` byte-identical, `{reason_label}` = REASON_LABELS[reason] 跟 AL-1a #249 REG-AL1A-005 + `agent/state.go` 6 reason codes 同源 — 改 = 改三处) | `error` | `agent.status == 'error'`, reason ∈ {`api_key_invalid` / `quota_exceeded` / `network_unreachable` / `runtime_crashed` / `runtime_timeout` / `unknown`} |
| ~~busy / idle~~ | ❌ | **不在 v0 范围** (留 BPP-1 同期) — 反向 grep 防 leak | ❌ | — |

**反约束 (跟 #303 立场 ⑥ 一致)**:
- ❌ 不显 IP / endpoint / connection_count
- ❌ 不显 `last_heartbeat_at` 时间戳 ("最近活跃 X 秒前" — §11 沉默胜于假 loading 反约束)
- ❌ 不显多端 (一个 agent 多 runtime 仍单 online dot)
- ❌ 不显 "Online" / "活跃" / "断线" / "下线" (英文 + 同义词 都不准漂)

---

## 2. 反向 grep — AL-3.3 PR merge 后跑, 全部预期 0 命中

```bash
# 文案漂移防御: tooltip 不准用同义词
grep -rnE "tooltip.*['\"](Online|Offline|断线|下线|活跃|不在线)['\"]" packages/client/src/ | grep -v _test
# busy/idle leak 防御: v0 不该出现
grep -rnE "data-presence=['\"](busy|idle)['\"]" packages/client/src/ | grep -v _test
# 心跳/多端/IP leak 防御
grep -rnE "last_heartbeat|connection_count|endpoint_ip|多端" packages/client/src/components/.*[Pp]resence.*\\.(tsx|ts) | grep -v _test
# error 文案模板锁: "故障 ({reason_label})" 必有 + reason_label byte-identical 跟 AL-1a #249 REASON_LABELS
grep -rnE "故障 \\(\\$\\{.*reason.*\\}\\)|故障 \\(\\{.*\\}\\)" packages/client/src/ | grep -v _test
# 反向: 早期文案锁草稿 "出错:" 不应再 leak 到实施 (孤儿 drift fix #后续)
grep -rnE "['\"]出错: " packages/client/src/ | grep -v _test
```

---

## 3. 验收挂钩 (AL-3.3 PR 必带)

- DOM 字面: `data-presence` ∈ `{online, offline, error}` (e2e DOM assert) — 跟 #302 acceptance §3 一致
- tooltip 文案 byte-identical: 4 行字面锁 + 6 reason codes byte-identical 跟 `agent/state.go` Reason* 同 (改字面 = 改两边 + 锁两端单测, 跟 AL-1a #249 同模式)
- 反向 grep §2 全 0 命中 / 全注释
- 野马 G2.5 demo screen 预备 (跟 G2.4 #5 同模式): 截屏归档 `docs/qa/screenshots/g2.5-presence-dot-{online,offline,error}.png` 三张 (CI Playwright 主动 `page.screenshot()`)

---

## 4. 不在范围

- ❌ busy / idle UI (留 BPP-1 同期); ❌ presence 历史时间线; ❌ 人的 presence dot (#303 立场 ① 永久不开)
- ❌ admin SPA presence UI (admin 不入 channel; god-mode 字段白名单见 #303 立场 ⑦)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 4 状态文案锁 (online/offline/error + busy/idle 反向 grep 防 leak) + 反约束 4 项 + 反向 grep 4 行 + 验收挂钩 (G2.5 demo 截屏 3 张预备) |
| 2026-04-29 | 野马 | v0.1 patch — 修孤儿 drift: error 文案 `"出错: {reason}"` → `"故障 ({reason_label})"` 跟 AL-1a #249 实施 (`lib/agent-state.ts` `describeAgentState` + REG-AL1A-005) 字面对齐, 反向 grep 加 "出错:" 防 drift 复发. 二轮反查抓出 (PR #324 后跨 doc/impl 不一致) |
