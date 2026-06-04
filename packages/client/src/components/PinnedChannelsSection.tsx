// PinnedChannelsSection.tsx — CHN-6.2 顶部 section for pinned channels.
//
// 反约束 (chn-6-content-lock.md §2):
//   - <section> + <header>已置顶频道</header> + data-testid="pinned-channels-section"
//   - 行 data-pinned="true"
//   - filter `channel.position < POSITION_PIN_THRESHOLD` byte-identical
//     跟 server PinThreshold=0 双向锁
//   - empty state — 无 pin 时整个 section **不渲染** (return null)
//   - 同义词反向: 收藏/标星/star/favorite/top/顶置/钉住 0 hit
import type { Channel } from '../types';
import { POSITION_PIN_THRESHOLD } from '../lib/pin';
import { channelDisplayName } from '../lib/channelDisplay';

interface PinnedChannelsSectionProps {
  channels: Array<Channel & { position?: number }>;
  onSelect?: (channelId: string) => void;
}

export function PinnedChannelsSection({ channels, onSelect }: PinnedChannelsSectionProps) {
  // filter 字面 byte-identical 跟 content-lock §2 — 双向锁跟 server.
  const pinned = channels.filter(
    ch => typeof ch.position === 'number' && ch.position < POSITION_PIN_THRESHOLD,
  );

  // empty state: 不渲染 section (return null).
  if (pinned.length === 0) {
    return null;
  }

  // ASC sort within pinned (server stamps -(nowMs) so smaller position
  // = more recent pin → top).
  const sorted = [...pinned].sort((a, b) => (a.position ?? 0) - (b.position ?? 0));

  return (
    <section className="pinned-channels-section" data-testid="pinned-channels-section">
      <header className="pinned-channels-header">已置顶频道</header>
      <ul className="pinned-channels-list">
        {sorted.map(ch => (
          <li
            key={ch.id}
            className="pinned-channel-item"
            data-pinned="true"
            onClick={() => onSelect?.(ch.id)}
          >
            <span className="channel-name">#{channelDisplayName(ch)}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

export default PinnedChannelsSection;
