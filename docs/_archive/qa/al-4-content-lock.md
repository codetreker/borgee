# AL-4 文案锁 — runtime start/stop/error system DM (野马 G2.7 demo 预备)

> **状态**: v0 (野马, 2026-04-28)
> **目的**: AL-4.2 server 触发 + AL-4.3 client UI 实施前锁 runtime 状态变化 3 处 system DM 文案 + DOM 字面 — 跟 AL-3 #305 / DM-2 #314 同模式 (用户感知签字 + 文案 byte-identical), 防 AL-4 实施时文案漂移; 配 #319 AL-4 立场 ⑤ + #313 spec §1 AL-4.2.
> **关联**: `agent-lifecycle.md` §2.3 三态决议 + §11 文案守 (沉默胜于假 loading); `agent/state.go` Reason* 6 codes (#249); ADM-0 §1.4 红线 ③ (system DM 模式); AL-3 #305 presence dot 文案锁; DM-2 #314 fallback DM 文案锁.

---

## 1. runtime 状态变化 — 3 处 system DM 文案锁 (字面 byte-identical, 仅 owner 收)

| 触发 | system DM body (字面锁, byte-identical) | 受众 | 反约束 |
|------|-----|------|------|
| **start** (status: registered/stopped/error → running) | `"{agent_name} 已启动"` (`{agent_name}` 占位, 其余字面锁) | agent owner DM only (不抄送 channel members, runtime 是 owner 的事) | ❌ payload 不含 raw `runtime_id` / `pid` / `endpoint_url` (隐私 + 立场 ① reverse: 进程内部细节不外暴) |
| **stop** (status: running → stopped) | `"{agent_name} 已停止"` (跟 AL-3 #305 offline `"已离线"` 区分: stop 是 process 级 explicit 停, offline 是 WS 断 implicit) | agent owner DM only | ❌ 不准跟 "已离线" / "已下线" / "已退出" 同义词漂; ❌ 不发 toast (沉默胜于假 loading §11 + DM-2 #314 ④ 反约束同精神) |
| **error** (status: running → error) | `"{agent_name} 出错: {reason}"` (`{reason}` byte-identical 跟 `agent/state.go` 6 reason codes 一致, 跟 AL-3 #305 ③ error 文案同根; server const `RuntimeStatusDMTemplateError = "%s 出错: %s"` byte-identical 同源 + client `lib/agent-state.ts::describeAgentState` `故障 (${reasonText})` 模板精神承袭) | agent owner DM only | ❌ `{reason}` 必须 ∈ {`api_key_invalid`, `quota_exceeded`, `network_unreachable`, `runtime_crashed`, `runtime_timeout`, `unknown`} (**六处单测锁** byte-identical: AL-1a `agent/state.go` Reason* + AL-3 #305 client REASON_LABELS + CV-4 #380 ③ + AL-2a #454 ④ + AL-1b #458 ⑤ + 本锁; 改 reason = 改六处); ❌ 不附 stack trace / log tail (隐私 + ADM-0 ⑦ admin 元数据 only 同精神) |

**通用反约束 (跟 #319 立场 ⑤ + ADM-0 §1.4 ③ + DM-2 #314 同模式)**:
- 所有 3 条: `kind='system'` + `sender_id='system'` (跟 DM-2 #314 ③ + soft-delete fanout 同模式)
- ❌ 不抄送 owner 之外 (channel members / admin 都不收)
- ❌ 不发 toast / inline banner / 浏览器通知 (走 DM 不走 UI 旁路, §11 沉默胜于假 loading)
- ❌ 不在 channel message stream fanout (只进 owner DM, runtime 不污染 channel)
- ❌ 不展开 raw `runtime_id` / `pid` / `endpoint_url` / `last_heartbeat_at` 时间戳 (跟 #319 立场 ① + ADM-0 ⑦ 同精神)

---

## 2. runtime UI 卡片 — DOM 字面锁 (AL-4.3 client SPA)

| 项 | 字面锁 |
|---|------|
| **status 渲染** | `<div data-runtime-status="{running|stopped|error}">...</div>` (3 态枚举严格闭合) |
| **start/stop button gate** | owner-only: 非 owner DOM **不渲染** (跟 CV-1 ⑦ rollback owner-only DOM gate 同模式, 不仅是 disabled — 直接 omit) |
| **error reason badge** | `<span data-error-reason="{reason}">{REASON_LABELS[reason]}</span>` (字面跟 `lib/agent-state.ts` REASON_LABELS 同源, 跟 AL-3 #305 / AL-1a #249 字面对齐) |

**反约束**:
- ❌ `data-runtime-status` 不准出现 `"starting"` / `"stopping"` / `"restarting"` 中间态 (v0 不做 in-flight 态 — start/stop API 同步 UPDATE status, 无异步 pending 期; v1 + 异步管控才考虑加, 跟 AL-3 #305 ① busy/idle 反约束同精神)
- ❌ button 非 owner 时 disabled 不算 — 必须 DOM omit (反向 grep `disabled.*owner` 不允许 leak owner 信息)
- ❌ 不显示 `endpoint_url` / `last_heartbeat_at` 原始时间戳 (跟 AL-3 #305 反约束同精神, 沉默胜于假精确)

---

## 3. 反向 grep — AL-4.2 / AL-4.3 PR merge 后跑, 全部预期 0 命中

```bash
# ① system DM 文案 byte-identical (server 端字面锁, 不准漂)
grep -rnE "已启动|已停止|出错:" packages/server-go/internal/api/runtimes.go packages/server-go/internal/api/messages.go | grep -v _test
# 同义词漂移防御
grep -rnE "['\"](已下线|已退出|崩溃|挂了|启动中|停止中|重启中)['\"]" packages/server-go/ packages/client/src/ | grep -v _test
# ② DOM 中间态 leak 防御
grep -rnE "data-runtime-status=['\"](starting|stopping|restarting)['\"]" packages/client/src/ | grep -v _test
# ③ raw runtime 内部字段 leak 防御
grep -rnE "runtime_id|endpoint_url.*innerText|pid.*display" packages/client/src/components/.*[Rr]untime.*\.(tsx|ts) | grep -v _test
# ④ owner button gate (非 owner DOM 不渲染, 不是 disabled)
grep -rnE "Button.*disabled.*!isOwner|disabled.*owner_id" packages/client/src/components/.*[Rr]untime.*\.(tsx|ts) | grep -v _test
# ⑤ 不准 toast 旁路 (跟立场 ⑤ system DM 唯一通道一致)
grep -rnE "toast.*runtime|RuntimeToast|Notification.*runtime" packages/client/src/ | grep -v _test
```

---

## 4. 验收挂钩 (AL-4.2 server + AL-4.3 client PR 必带)

- AL-4.2 server: ① 3 处 system DM body byte-identical (server 端 grep ≥1) + payload `kind='system'` + `sender_id='system'` + recipient = agent.owner_id only (反向断言 channel fanout count==0); error reason ∈ 6 枚举 (CHECK 反向断言)
- AL-4.3 client: ② `data-runtime-status` ∈ {running, stopped, error} (e2e DOM assert) + 中间态反向 grep §3 全 0; ④ owner-only button DOM 反向断言 (非 owner sniff DOM 无 button); error reason badge 跟 `lib/agent-state.ts` REASON_LABELS 字面对齐 (跟 AL-3 #305 / AL-1a #249 三处一致)
- 反向 grep §3 全 0 命中 / 全注释 (跟 AL-3 #305 / DM-2 #314 同模式)
- 野马 G2.7 demo screen 预备 (跟 G2.5 / G2.6 同模式): 截屏归档 `docs/qa/screenshots/g2.7-runtime-{start,stop,error}.png` 三张 (CI Playwright 主动 `page.screenshot()`)

---

## 5. 不在范围

- ❌ start/stop in-flight 中间态 (`starting` / `stopping`) — v0 同步 API, 留 v1 + 异步; ❌ restart 文案 (走 stop + start 两条 DM, v0 不合并)
- ❌ runtime 自动重启 / failover 提醒 (留第 6 轮 remote-agent 安全模型 + #319 立场 ⑦)
- ❌ admin SPA runtime UI (admin 不入写动作, ADM-0 ⑦ 红线 + #319 立场 ②)
- ❌ owner 静音偏好 (per-agent runtime 通知开关, 留 v1 — 跟 #319 §5 v0/v1 切换条件 ⑤ 一致)
- ❌ Hermes / Windows runtime 文案 (v1 only OpenClaw / Mac+Linux, 蓝图 §2.2 边界)

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 3 处 system DM 文案锁 (start/stop/error) + DOM 字面锁 (data-runtime-status 3 态严闭) + owner-only button DOM gate + 5 行反向 grep + G2.7 demo 截屏 3 张预备, 跟 #319 立场 ⑤ + #313 spec §1 AL-4.2 + AL-3 #305 + DM-2 #314 字面对齐 |
| 2026-04-29 | 野马 | v0.1 patch — error reason 单测锁 count drift fix: "改 = 改三处" → **"改 = 改六处"** byte-identical (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + 本锁); 加 server const `RuntimeStatusDMTemplateError = "%s 出错: %s"` 同源锚 + client `describeAgentState` 模板精神承袭. 跟 #387 AL-4 stance v0.1 patch 同步, 跟 #339/#393 follow-up patch 同模式 (历史干净) |
