# RT-3 ⭐ client — useRT3Presence + RT3PresenceDot (≤40 行)

> 实现位置: feat/rt-3 RT-3.2 client (`hooks/useRT3Presence.ts` + `components/RT3PresenceDot.tsx` + 13 vitest)
> 关联: server `docs/current/server/rt-3.md` PresenceState 4 态 enum; client state values must match it.

## 1. hook — `hooks/useRT3Presence.ts`

```ts
export type RT3PresenceState = 'online' | 'away' | 'offline' | 'thinking';
export const RT3_AWAY_THRESHOLD_MS = 5 * 60 * 1000;
export function markRT3Presence(userID, state, subject): void;
export function getRT3Presence(userID): RT3PresenceEntry | undefined;
export function useRT3Presence(userID): RT3PresenceEntry | undefined; // 派生 online ≥ 5min → away
```

**Drop rule**: thinking 态 + 空 subject → drop (防止显示未确认的 loading 状态, matching server `ValidateTaskStarted`).

## 2. component — `components/RT3PresenceDot.tsx`

DOM data attrs must match content-lock §2:
- `data-rt3-presence-dot` ∈ {online, offline, recently-active}
- `data-rt3-last-seen` = unix-ms
- `data-rt3-cursor-user` = user-id

UI literals locked by content-lock §1:
- `在线` / `离线` / `刚刚活跃` / `最近活跃 ${N} 分钟前`

## 3. tests

- `__tests__/RT3PresenceDot.test.tsx` 9 case (4 态 + last-seen + thinking subject drop rule + multi-device + RT3_AWAY_THRESHOLD_MS const)
- `__tests__/rt3-content-lock-reverse-grep.test.ts` 4 case (typing 9 同义词 0 hit + thought-process 5-pattern 0 hit + 4 态 enum + DOM attr lock)

## 4. 反向约束

- ❌ typing-indicator 启用 (永久不挂)
- ❌ AL-3 既有 `usePresence.ts` 不复用 (那是 agent presence cache, RT-3 是 human multi-device presence — 两个维度不混用)
- ❌ thought-process 5-pattern (processing/responding/analyzing/planning/"AI is thinking") 0 hit
