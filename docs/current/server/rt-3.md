# RT-3 ⭐ multi-device fanout + 活物感 4 态 + thinking subject 反约束 (≤80 行)

> 落地: PR feat/rt-3 · RT-3.1 server (PresenceState enum + ThinkingErrCodeSubjectRequired const) + RT-3.2 client (useRT3Presence hook + RT3PresenceDot component) + closure (REG-RT3-007/008)
> 蓝图锚: [`realtime.md`](../../blueprint/current/realtime.md) §0 + §1.1 (thinking subject ⭐) + §1.4 (活物感 4 态)
> 设计沿用: [`rt-3-spec.md`](../../implementation/modules/rt-3-spec.md) §0 ① DL-1+RT-1 byte-identical + ② 4 态 enum SSOT + thinking subject 必带 + ③ 0 schema/endpoint 改

## 1. PresenceState 4 态 enum SSOT (`internal/datalayer/presence.go`)

| 态 | const | 语义 | UI 派生 |
|---|---|---|---|
| online | `PresenceStateOnline = "online"` | 至少 1 live session (跟 IsOnline 同源) | `data-rt3-presence-dot=online` + `在线` tooltip |
| away | `PresenceStateAway = "away"` | online ≥ 5min 无活动派生 / server 推 | `data-rt3-presence-dot=recently-active` + `刚刚活跃` 或 `最近活跃 N 分钟前` |
| offline | `PresenceStateOffline = "offline"` | 0 live session | `data-rt3-presence-dot=offline` + `离线` |
| thinking | `PresenceStateThinking = "thinking"` | agent 执行任务 (走 bpp.task_started + Subject 必带非空) | `data-rt3-presence-dot=recently-active` (subject 由 caller UI 显示) |

**反约束**: 4 态封闭枚举 (反 PresenceStateTyping/Composing/Idle/Pending/Loading 漂 — grep 检查 0 hit).

## 2. thinking subject 反约束 (蓝图 §1.1 ⭐ 关键纪律)

`internal/bpp/task_lifecycle.go` 加 `ThinkingErrCodeSubjectRequired = "thinking.subject_required"` wire-level reason code SSOT. server 走 `ValidateTaskStarted` SSOT (errSubjectEmpty sentinel) reject 空 subject; client 防御层 `markRT3Presence` 在 thinking 态 + 空 subject 时 drop, 反"假 loading" 漂.

跟 chn-3 content-lock 5 源 byte-identical 守门同模式 (改 = 改三处: 此 const + acceptance §2.3 + content-lock §3).

## 3. client UI (`packages/client/src/`)

| 文件 | 范围 |
|---|---|
| `hooks/useRT3Presence.ts` (97 行) | 4 态 enum + markRT3Presence + getRT3Presence + useRT3Presence hook + RT3_AWAY_THRESHOLD_MS=5min const + thinking subject 防御层 |
| `components/RT3PresenceDot.tsx` (54 行) | 4 态 UI + DOM data-attr SSOT + tooltip 字面 byte-identical |
| `__tests__/RT3PresenceDot.test.tsx` (9 case PASS) | 4 态 + last-seen + thinking 反约束 + multi-device 后写覆盖前 |
| `__tests__/rt3-content-lock-reverse-grep.test.ts` (4 case PASS) | typing 9 同义词 0 hit + 5-pattern 0 hit + 4 态 enum + DOM attr SSOT |

## 4. 跨 milestone byte-identical 锁链

- **DL-1 #609** EventBus + PresenceStore interface signature 不破 (RT-3 仅扩 PresenceState enum, 不改 method 签名)
- **RT-1 #290** cursor 协议 ULID `kind+ulid` byte-identical 沿用 (RT-3 multi-device fanout 走 hub.cursors 单源)
- **reasons.IsValid #496** / **AP-4-enum #591** / **NAMING-1 #614** enum SSOT 同模式 (PresenceState 4 态单源)
- **chn-3 content-lock §1** 字面锁 (thinking.subject_required 5 源 byte-identical)
- **thought-process 5-pattern 锁链 RT-3 = 第 N+1 处延伸** (跟 BPP-3 + CV-* + DM-* 既有锁链一致)
- **admin god-mode 不挂红线** (ADM-0 §1.3, RT-3 域不挂)

## 5. 字面检查项 (PR body 用)

| 检查项 | 期望 | 当前 |
|---|---|---|
| `PresenceStateOnline\|Away\|Offline\|Thinking` const | 4 hit (单源) | ✅ 4 |
| `ThinkingErrCodeSubjectRequired = "thinking.subject_required"` | 1 hit | ✅ 1 |
| `data-rt3-presence-dot/last-seen/cursor-user` | 3 hit | ✅ 3 |
| typing 9 同义词 (英 5 + 中 4) in RT-3 path | 0 hit | ✅ 0 |
| thought-process 5-pattern in RT-3 path | 0 hit | ✅ 0 |
| `git diff origin/main -- internal/migrations/` | 0 行 | ✅ |
| `git diff origin/main -- internal/server/server.go` HandleFunc | 0 hit | ✅ |

## 6. 反约束 / 不在范围

- ❌ events 接 RT-3 fanout 上游 hook (留 DL-2 cold-stream wire-up — 真架构边界, 飞马 audit 接受)
- ❌ typing-indicator 真启 (永久不挂, 跟 thought-process 5-pattern 锁链一致)
- ❌ last-seen UI 跨设备 sync (留 RT-3.2 后续)
- ❌ agent presence (蓝图 §1.4 仅人类 4 态; agent 走 BPP heartbeat HB-1..6)
- ❌ session_resume_hint (蓝图 §1.3 留 DL-5+)
- ❌ per-channel presence 视图 (留 v3+)

## 7. Tests + verify

- `go test -tags sqlite_fts5 -timeout=300s ./...` 全 26 packages PASS ✅
- `pnpm test (client)` 98 files / 648 passed / 1 skipped ✅
- `pnpm typecheck (client)` clean ✅
- `pnpm test rt-3-presence (e2e)` Playwright 5 case PASS (真跑 server-go 4901 + vite 5174) ✅
- 5 截屏 demo PNG 入 git: `docs/qa/screenshots/rt-3-{multi-device,subject,busy-idle,reject,offline-fallback}.png` ✅
- post-#614 haystack gate Func=50/Pkg=70/Total=85 (CI 自然 trigger)
