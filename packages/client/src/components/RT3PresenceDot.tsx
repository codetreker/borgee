// RT-3 ⭐ presence dot — 4 态 UI 组件 (蓝图 §1.4 活物感).
//
// 设计沿用 (rt-3-spec.md §0 + content-lock §1+§2):
//   - Four required labels: `在线` / `离线` / `刚刚活跃` / `最近活跃 ${N} 分钟前`
//   - DOM data-attr SSOT: data-rt3-presence-dot ∈ {online, offline, recently-active}
//     + data-rt3-last-seen=unix-ms + data-rt3-cursor-user=user-id
//   - Synthetic progress indicators must not drift in (content-lock §3) — 仅活物感 + 时间戳,
//     不显语义中间态
//   - thought-process 5-pattern wording must not drift in (content-lock §4) — 5 字面 0 hit
//     (跟 BPP-3 + CV-* + DM-* 既有锁链承袭, RT-3 = 锁链第 N+1 处延伸)
//
// Tests: __tests__/RT3PresenceDot.test.tsx (4 态 + last-seen + DOM data-attr).
import { useRT3Presence, type RT3PresenceState } from '../hooks/useRT3Presence';

interface RT3PresenceDotProps {
  userID: string;
  /** 注入式 now() — 测试用 controllable clock; 生产默认 Date.now. */
  now?: () => number;
}

/** 把 4 态映射到 DOM data-attr 字面 (content-lock §2). */
function dotAttr(state: RT3PresenceState | undefined, lastSeenAt: number | undefined, now: number): string {
  if (state === 'online') return 'online';
  if (state === 'offline' || state === undefined) return 'offline';
  // away / thinking 共享 "recently-active" UI hint (内容由 last-seen 字面区分).
  return 'recently-active';
}

/** 把 last-seen unix-ms 渲染为 `最近活跃 ${N} 分钟前` (≥1min) 或 `刚刚活跃` (<1min). */
function lastSeenLabel(lastSeenAt: number, now: number): string {
  const diffMs = now - lastSeenAt;
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return '刚刚活跃';
  return `最近活跃 ${diffMin} 分钟前`;
}

/**
 * RT3PresenceDot — 用户 multi-device presence 4 态渲染.
 * online → `在线` 绿点 / offline → `离线` 灰点 / away or thinking → "刚刚活跃"
 * 或 "最近活跃 N 分钟前" 黄点.
 */
export function RT3PresenceDot({ userID, now }: RT3PresenceDotProps) {
  const entry = useRT3Presence(userID);
  const nowFn = now ?? Date.now;
  const t = nowFn();
  const attr = dotAttr(entry?.state, entry?.lastSeenAt, t);
  const lastSeenAt = entry?.lastSeenAt ?? 0;
  const tooltip =
    attr === 'online'
      ? '在线'
      : attr === 'offline'
        ? '离线'
        : lastSeenLabel(lastSeenAt, t);
  // 颜色 + 尺寸全部走 CSS 类 (index.css `.rt3-presence-dot*`) — dot 必须是有真
  // bounding box 的可见圆点 (online 绿 / offline 灰 / recently-active 黄), 所有
  // 态都可见, 不许 offline display:none / zero-size, 否则 #971 "活物感不可见"复发.
  return (
    <span
      className={`rt3-presence-dot rt3-presence-dot-${attr}`}
      data-rt3-presence-dot={attr}
      data-rt3-last-seen={lastSeenAt}
      data-rt3-cursor-user={userID}
      title={tooltip}
      role="status"
      aria-label={tooltip}
    />
  );
}
