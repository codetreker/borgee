# RT-3 ⭐ multi-device fanout + four presence states + thinking subject constraints (≤80 行)

> Landed in PR feat/rt-3: RT-3.1 server (PresenceState enum + ThinkingErrCodeSubjectRequired const) + RT-3.2 client (useRT3Presence hook + RT3PresenceDot component) + closure (REG-RT3-007/008)
> Blueprint source: [`realtime.md`](../../blueprint/current/realtime.md) §0 + §1.1 (thinking subject ⭐) + §1.4 (four presence states)
> Design source: [`rt-3-spec.md`](../../implementation/modules/rt-3-spec.md) §0 ① DL-1+RT-1 byte-identical + ② four-state enum single source + thinking subject required + ③ no schema/endpoint changes

## 1. PresenceState Four-State Enum (`internal/datalayer/presence.go`)

| State | const | Meaning | UI Derived Value |
|---|---|---|---|
| online | `PresenceStateOnline = "online"` | at least one live session, matching IsOnline | `data-rt3-presence-dot=online` + `在线` tooltip |
| away | `PresenceStateAway = "away"` | online for ≥ 5min with no activity, derived or pushed by server | `data-rt3-presence-dot=recently-active` + `刚刚活跃` 或 `最近活跃 N 分钟前` |
| offline | `PresenceStateOffline = "offline"` | 0 live session | `data-rt3-presence-dot=offline` + `离线` |
| thinking | `PresenceStateThinking = "thinking"` | agent is executing a task via bpp.task_started, with a non-empty Subject | `data-rt3-presence-dot=recently-active` (subject is displayed by caller UI) |

**Constraint**: the four-state enum is closed; grep checks keep PresenceStateTyping/Composing/Idle/Pending/Loading at 0 hits.

## 2. thinking subject Constraint (蓝图 §1.1 ⭐)

`internal/bpp/task_lifecycle.go` adds `ThinkingErrCodeSubjectRequired = "thinking.subject_required"` as the single source for the wire-level reason code. The server rejects empty subjects through `ValidateTaskStarted` (errSubjectEmpty sentinel); the client-side `markRT3Presence` guard drops thinking updates with an empty subject to avoid fake loading states.

This follows the chn-3 content-lock pattern for five byte-identical sources: changing it requires updating this const, acceptance §2.3, and content-lock §3 together.

## 3. client UI (`packages/client/src/`)

| 文件 | 范围 |
|---|---|
| `hooks/useRT3Presence.ts` (97 行) | four-state enum + markRT3Presence + getRT3Presence + useRT3Presence hook + RT3_AWAY_THRESHOLD_MS=5min const + thinking subject guard |
| `components/RT3PresenceDot.tsx` (54 行) | four-state UI + DOM data-attr single source + tooltip text byte-identical |
| `__tests__/RT3PresenceDot.test.tsx` (9 case PASS) | four states + last-seen + thinking constraint + multi-device last-write-wins behavior |
| `__tests__/rt3-content-lock-reverse-grep.test.ts` (4 case PASS) | typing 9 同义词 0 hit + 5-pattern 0 hit + 4 态 enum + DOM attr 单一来源 |

## 4. 跨 milestone byte-identical 守护链

- **DL-1 #609** EventBus + PresenceStore interface signature 不破 (RT-3 仅扩 PresenceState enum, 不改 method 签名)
- **RT-1 #290** cursor 协议 ULID `kind+ulid` byte-identical 沿用 (RT-3 multi-device fanout 走 hub.cursors 单一来源)
- **reasons.IsValid #496** / **AP-4-enum #591** / **NAMING-1 #614** enum 单一来源 同模式 (PresenceState 4 态单一来源)
- **chn-3 content-lock §1** 字面锁定 (thinking.subject_required 5 源 byte-identical)
- **thought-process 5-pattern 守护链 RT-3 = 第 N+1 处延伸** (跟 BPP-3 + CV-* + DM-* 既有守护链一致)
- **admin path isolation** (ADM-0 §1.3; RT-3 is not exposed through admin paths)

## 5. Literal Checks (for PR body)

| 检查项 | 期望 | 当前 |
|---|---|---|
| `PresenceStateOnline\|Away\|Offline\|Thinking` const | 4 hit (单一来源) | ✅ 4 |
| `ThinkingErrCodeSubjectRequired = "thinking.subject_required"` | 1 hit | ✅ 1 |
| `data-rt3-presence-dot/last-seen/cursor-user` | 3 hit | ✅ 3 |
| typing 9 同义词 (英 5 + 中 4) in RT-3 path | 0 hit | ✅ 0 |
| thought-process 5-pattern in RT-3 path | 0 hit | ✅ 0 |
| `git diff origin/main -- internal/migrations/` | 0 行 | ✅ |
| `git diff origin/main -- internal/server/server.go` HandleFunc | 0 hit | ✅ |

## 6. Constraints / Out Of Scope

- ❌ events to RT-3 fanout upstream hook (left for DL-2 cold-stream wire-up; accepted architecture boundary)
- ❌ typing-indicator enablement must never be mounted or exposed; this permanent ban matches the thought-process 5-pattern guard
- ❌ last-seen UI cross-device sync (left for an RT-3.2 follow-up)
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
