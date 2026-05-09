# Agent Manager — Detail page (gh#683 + gh#684) — implementation note

> gh#683 (PR #694) — Agents 页面默认 width:100%, 反 flex cross-axis 缩 334px 闪跳 800px.
> gh#684 (PR #710) — Agent 详情 6 卡 section 重组 + Credentials 卡 mask + 复制 + auto-clear 60s.
> 蓝图: `client-shape.md` § agent-manage / agent-detail.

## 1. 设计

agent 详情页 ("Manage" 展开区) 重组成 6 卡 section, 视觉分层"读"vs"写"清晰 + Credentials 卡 by-construction 反完整 plaintext API key 进 DOM.

反约束:
- ① agents page 容器 width:100% (反 flex cross-axis 缩造成默认 334px / Manage 撑大闪跳 800px) — gh#683 PR #694
- ② Credentials 卡完整 plaintext key 永不进 React state / ref / DOM (走 closure GC, 仅末 4 位进 last4 state) — gh#684 by-construction
- ③ 不引第三方 clipboard 库 (走 navigator.clipboard 原生 + execCommand fallback)
- ④ auto-clear 60s 走 readText 比对 (反误清用户 60s 内主动改的剪贴板内容)
- ⑤ 不动 server-side endpoint (#710 client-only mask, server-side mask + reveal endpoint + audit log 留 followup cluster)

## 2. agents page width (gh#683 / PR #694)

`packages/client/src/components/AgentManager.tsx`:
- 顶层容器 `width: 100%` — 反 flex cross-axis 默认 `width: auto` 缩到 334px (内容最小宽度), Manage 展开后撑大闪跳到 800px (visual jank)

跟 #694 PR body 一致, 1 行 CSS 改 (`.agent-page { width: 100%; }`) 反 jank.

## 3. 6 卡 section 重组 (gh#684 / PR #710)

按 yema brief v3 (`docs/implementation/design/agent-manager-detail.md`) §2.1, expanded 区 6 卡顺序:

```
+──────────────────────────────────────────────────+
│  [Identity 卡]                                    │
│  📷  AgentX  • 在线                              │
│      ID: ag-12345... | Created: 2026-04-29       │
│                              [Collapse] [Delete] │
├──────────────────────────────────────────────────┤
│  [Credentials 卡]                                 │
│  API Key                                          │
│  bgr_...abc1                          [📋]       │
│  [Rotate API Key]                                 │
├──────────────────────────────────────────────────┤
│  [Runtime 卡]  RuntimeCard (owner-only)          │
├──────────────────────────────────────────────────┤
│  [Config 卡]  AgentConfigPanel (#447 + #698)     │
├──────────────────────────────────────────────────┤
│  [Permissions 卡]  KNOWN_PERMISSIONS 复选框       │
├──────────────────────────────────────────────────┤
│  [Channels 卡]  Add to Channel                    │
+──────────────────────────────────────────────────+
```

DOM 锚:
- `<section className="agent-detail-card agent-detail-card-identity">` (Header)
- `<section className="agent-detail-card agent-detail-card-credentials">`
- `<section className="agent-detail-card agent-detail-card-runtime">`
- `<section className="agent-detail-card agent-detail-card-config">`
- `<section className="agent-detail-card agent-detail-card-permissions">`
- `<section className="agent-detail-card agent-detail-card-channels">`

state hoist 全在 `AgentCard` 顶层 (反 prop drilling): `last4` / `loadingKey` / `copying` / `autoClearTimerRef` / `permissions` / `runtime` / `joinChannelId`. 子卡片只 React `<section>` wrapper + class CSS, 0 跨段共享 state 漂.

## 4. Credentials 卡 mask + 复制 + auto-clear (gh#684)

### 4.1 Mask 模式

`bgr_...{last4}` (前 4 char `bgr_` + `...` + 末 4 char). 例: `bgr_...abc1`.

前缀 `bgr_` 跟 server `GenerateAPIKey()` (`packages/server-go/internal/store/queries_phase2b.go:440`) 真值 byte-identical 锁 — 反 OpenAI `sk-` 误抄 (yema brief v3 §2.3 + §3 文案锁).

末 4 位露够认人不够暴破解, 思路跟 GitHub PAT / Stripe / OpenAI 行业一致.

### 4.2 复制 + auto-clear

```
点 📋 复制
  → fetchAgent(agentId) 拉完整 plaintext
  → navigator.clipboard.writeText(key)
  → setLast4(key.slice(-4))     ← 仅末 4 位进 state, 完整 key 走出 closure 后 GC
  → showToast('API Key 已复制, 60 秒后自动清空')
  → setTimeout(60_000) 启动:
      readText() 比对当前剪贴板 === copiedKey
        ? writeText('') + showToast('剪贴板已清空 (安全保护)')
        : 不动 (用户主动改了剪贴板)
  → catch readText 拒 (Firefox 默认拒): 降级不清, 让用户掌控
```

unmount cleanup `clearTimeout(autoClearTimerRef.current)` 反 dirty timer 卸载后还触发. cleanup 不主动 writeText('') — 用户 unmount 之后剪贴板状态由用户掌控.

### 4.3 execCommand fallback

浏览器不支持 navigator.clipboard (非 https / 旧浏览器):

```ts
const ok = document.execCommand('copy');
```

`execCommand` 已 deprecated 但仍是 fallback 唯一选项 (反第三方 clipboard 库, heima Sec 设计 4).

### 4.4 grep 检查 7 锚 (heima Sec 必扫清单)

| 锚 | 预期 | 真量 |
|---|---|---|
| `'Show'` / `reveal.*key` 在 AgentManager.tsx | 0 | 0 |
| innerText/innerHTML.*api_key | 0 | 0 |
| 第三方 `clipboard` / `copy-to-clipboard` / `clipboard.js` | 0 | 0 |
| `writeText` 在 AgentManager.tsx (#684 范围) | ≥2 | 5 (load/copy/rotate) |
| `'sk-` 字面 | 0 | 0 |
| `bgr_` 真前缀 | ≥1 | 3 (helper L37 + 2 注释) |
| `Rotate API Key` 字面 | 1 | 1 |

## 5. Prompt textarea (gh#684 §2.2)

`AgentConfigPanel.tsx` 内部 prompt textarea:
- `rows={8}` (默认 8 行高, 反默认 1 行太小)
- `style={{ resize: 'vertical', width: '100%', boxSizing: 'border-box' }}` (用户拖拽扩到合适高度)
- 反约束: 不强制 monospace (prompt 是自然语言指令, 不是代码)

## 6. 文案锁 (byte-identical)

| 位置 | 文案 byte-identical |
|---|---|
| Mask 模式 | `bgr_...{last4}` |
| 复制按钮 aria-label | `复制 API Key` |
| 复制按钮 title | `复制完整 API Key 到剪贴板` |
| 复制成功 toast | `API Key 已复制, 60 秒后自动清空` |
| auto-clear 后 toast | `剪贴板已清空 (安全保护)` |
| 复制失败 toast | `复制失败, 请手动选择 mask 后的 key 复制片段` |
| Rotate API Key 按钮 | `Rotate API Key` |
| 加载中占位 | `加载中...` |

## 7. 留账 followup (cluster, design §6 + §7 已留账)

不在 #684 范围, 等独立 cluster brainstorm:
- **server-side mask + reveal endpoint**: `sanitizeAgent` 默认返 mask, 加 `POST /agents/:id/reveal_api_key` 显式 endpoint
- **audit log 拉 key 事件**: 加 server audit log 记 GET api_key + reveal 动作 (user + ts + IP)
- **Create Agent 模态 createdKey 仍进 DOM**: brief §7 不在 #684 范围, 跟 server-side mask 同 cluster 一起讨论
- **mask helper 抽 lib/mask.ts**: 如果 admin 那边也要 mask secret 时再抽 (当前 1 处用)

## 8. 测试

- `packages/client/src/__tests__/AgentManager-detail.test.tsx` 8 case (mask 字面 / aria-label / Show 反向断言 / toast 文案 / sk- 反向 / closure GC / writeText / readText 比对)
- `packages/e2e/tests/gh-684-agent-detail-credentials.spec.ts` 6 case (1280 / login / Manage 展开 / mask 显 / 复制 toast + clipboard / 480 mobile / 反向 DOM)

## 9. 锚

- 蓝图: `client-shape.md` § agent-manage / agent-detail
- design: `docs/implementation/design/agent-manager-detail.md` (211 行 SSOT, gh#684 PR #710 一锅入)
- PR: #694 (gh#683 width:100% + pnpm@10) + #710 (gh#684 6 卡 + Credentials)
