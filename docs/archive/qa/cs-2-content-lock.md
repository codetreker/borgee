# CS-2 文案锁 — 故障三态 + 4 层 UX + reason 6-dict 字面

> **状态**: v0 (野马, 2026-04-30)
> **目的**: CS-2 实施 PR 落 client 故障 UX 前锁字面 — 跟 reasons.IsValid #496 / AL-4 #321 同模式, 防 reason 文案 / inline button / 三态 enum 漂.
> **关联**: spec `cs-2-spec.md` §0 + AL-4 #321 文案锁 + reasons.IsValid #496 SSOT 6-dict + 蓝图 client-shape.md §1.3.

---

## 1. 故障三态 enum — byte-identical 锁 (3 项 SSOT)

| 顺序 | 字面 | 含义 | 反约束 |
|------|------|------|--------|
| 1 | `online` | runtime 已连接 | 不准漂 `connected` / `active` |
| 2 | `failed` | API key 失效 / 超限 / 进程崩溃 / 网络断 | 不准漂 `error` (server-side reason 字典用 error, client UI 用 failed; 拆死) / `broken` / `down` |
| 3 | `offline` | disable / 用户主动关 | 不准漂 `disabled` / `paused` |

**通用反约束**:
- ❌ 第 4 态 `busy` / `idle` / `standby` 漂入 (AL-1b §2.3 BPP progress frame 真实施时 v2 才加)
- ❌ enum 顺序漂 (跟蓝图 §1.3 表 byte-identical)
- ❌ 大小写漂 (`Online` / `OFFLINE` 全禁, lowercase byte-identical)

---

## 2. 4 层 UX 呈现 — DOM + 文案 byte-identical (跟蓝图 §1.3 表)

| 层 | DOM 锚 | 文案 byte-identical | 反约束 |
|---|------|------|------|
| **头像角标** | `<PresenceDot variant="failure" data-failure-badge="true">` | (无文案, 红点视觉) | 不准 toast 替代 / 不漂 status="error" |
| **点头像 → 浮层** | `<div data-cs2-failure-popover="open">` | 浮层标题 = `formatFailureLabel(reason, agentName)` byte-identical 跟 §3 6-dict; 3 inline button data-action: `reconnect` (label `重连` 2 字) / `refill_api_key` (label `重填 API key` 6 字) / `view_logs` (label `查日志` 3 字) | 不准跳设置页 (`navigate.*\/settings` 0 hit); 不准 raw error code 暴 (必走 plain language 映射) |
| **顶部 banner** | `<div data-cs2-failure-banner="visible" role="alert">` | banner body byte-identical: 全部故障 → `"全部 agent 故障, 请检查"` (10 字); 核心 agent > 5min → `"{agent_name} 已故障 5 分钟以上"` 模板 | 不准 `<dialog>` 替 banner; 不准声音通知 (留 DL-4) |
| **故障中心** | `<button data-cs2-failure-center-toggle> + <ul data-cs2-failure-center-list>` | 按钮文案 `故障中心 ({N})` 模板 byte-identical, N=故障 agent 数; 单 agent 时不渲染 (return null) | 不准跳 admin SPA / 不准 admin god-mode 看 |

**通用反约束 (跟 spec §0 立场 ② + AL-4 #321 同模式)**:
- ❌ 不另起第 5 层 (toast / modal / inline-error 反向 grep `toast.*failure\|FailureModal\|FailureInlineError` count==0)
- ❌ inline 修复跳设置页 (蓝图字面 "inline 修复, 不跳设置页"; `navigate.*\/settings` 在 Failure*.tsx count==0)
- ❌ 浮层显示 raw error code (`401 Unauthorized` / `connection refused` 在 user-visible text 0 hit)
- ❌ 浮层 3 button 数量 / 顺序 / 文案漂 (vitest assert literal byte-identical)
- ❌ admin god-mode 看故障中心 (ADM-0 §1.3 红线 — admin 看 audit 不看实时故障 UX)

---

## 3. plain language reason 6-dict — byte-identical 锁 (跟 reasons.IsValid #496 + AL-4 #321 同源)

| reason key | label 模板 byte-identical | 跟蓝图字面对比 |
|---|------|------|
| `api_key_invalid` | `"API key 已失效, 需要重新填写"` | 蓝图 §1.3 字面 ("API key 已失效, 需要重新填写") |
| `quota_exceeded` | `"{agent_name} 的配额已用完"` | 蓝图未明示, 跟 AL-1a 6-dict + plain language 同精神 |
| `network_unreachable` | `"{agent_name} 跟 OpenClaw 失联"` | 蓝图 §1.3 字面 ("DevAgent 跟 OpenClaw 失联") |
| `runtime_crashed` | `"{agent_name} 进程崩溃, 请重启"` | 跟 AL-4 reason 字面承袭 + plain language |
| `runtime_timeout` | `"{agent_name} 响应超时"` | 同上 |
| `unknown` | `"{agent_name} 出错, 请查日志"` | 同上 + 引导 `查日志` button |

**反约束**:
- ❌ 同义词漂 (`故障了` / `挂了` / `不可用` / `服务异常` / `崩了` / `掉线` 在 cs2-failure-labels.ts 0 hit)
- ❌ 6-dict 字面跟 reasons.IsValid #496 / AL-4 不一致 (改 = 改三处: server reasons.go + client cs2-failure-labels.ts + 本锁)
- ❌ raw error code 暴露 (`401 Unauthorized: invalid_token` / `connection refused: openclaw://localhost:9100` 在 user-visible 0 hit)

---

## 4. 反向 grep 锚 (跟 stance §2 + spec §2 同源)

```bash
# ① 三态拆死 — busy/idle 不漂
git grep -nE "'busy'|'idle'|'standby'" packages/client/src/lib/cs2-failure-*  # 0 hit
# ② 4 层不漂 — 不另起 5 层
git grep -nE 'toast.*failure|FailureModal|FailureInlineError' packages/client/src/  # 0 hit
# ③ inline 修复不跳设置页
git grep -nE 'navigate.*\/settings|history\.push.*settings' packages/client/src/components/Failure*.tsx  # 0 hit
# ④ plain language 同义词反向
git grep -nE '故障了|挂了|不可用|服务异常|崩了|掉线' packages/client/src/lib/cs2-failure-labels.ts  # 0 hit
# ⑤ raw error code 不暴
git grep -nE 'connection refused|invalid_token|401 Unauthorized' packages/client/src/components/Failure*.tsx  # 0 hit
# ⑥ admin god-mode 不挂 (ADM-0 §1.3 红线)
git grep -nE 'admin.*failure-ux|admin.*FailureCenter' packages/client/src/  # 0 hit
# ⑦ 0 server 改
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 production lines
```

---

## 5. 验收挂钩

- CS-2.1 PR: §1 三态 byte-identical + §3 6-dict byte-identical + 单测 `TestCS21_FailureLabels_6DictByteIdentical`
- CS-2.2 PR: §2 4 层 DOM + 3 inline button 文案 byte-identical + vitest literal assert
- CS-2.3 entry 闸: §1+§2+§3 全锚 + §4 反向 grep 7 行全 0 + 跨 milestone byte-identical (reasons.IsValid #496 + AL-4 #321 + ADM-0 §1.3 红线)

---

## 6. 不在范围

- ❌ 第 4 态 busy/idle (留 AL-1b §2.3)
- ❌ repair 真路径 (留 plugin SDK)
- ❌ 桌面通知 / 故障声音 (留 DL-4)
- ❌ admin god-mode 故障 UX (永久不挂)
- ❌ 同义词容忍 (cf. content-lock §3 反约束严锁)

---

## 7. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 | v0 — CS-2 文案锁 (3 段: 三态 enum 字面 + 4 层 UX DOM + 文案 byte-identical + 6-dict reason labels). 7 行反向 grep + 验收三段对齐. 跟 reasons.IsValid #496 / AL-4 #321 / 蓝图 client-shape.md §1.3 同源 byte-identical. plain language 同义词反向严锁 (6 词 0 hit) + raw error code 不暴严锁 (3 patterns 0 hit). |
