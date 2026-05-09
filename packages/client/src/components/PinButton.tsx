// PinButton.tsx — CHN-6.2 channel pin/unpin toggle.
//
// 反约束 (chn-6-content-lock.md §1):
//   - 文案 byte-identical: `置顶` 1 字 (未 pin) / `取消置顶` 3 字 (已 pin)
//   - data-action="pin" / "unpin" 二态
//   - 同义词反向 reject: 收藏/标星/star/favorite/top/顶置/钉住 0 hit
//   - 调用 lib/api.ts::pinChannel 单源; grep 检查 components/ inline
//     fetch 0 hit
import { useState } from 'react';
import { pinChannel } from '../lib/api';

interface PinButtonProps {
  channelId: string;
  pinned: boolean;
  onChange?: (pinned: boolean) => void;
}

export function PinButton({ channelId, pinned, onChange }: PinButtonProps) {
  const [busy, setBusy] = useState(false);

  const handleClick = async () => {
    if (busy) return;
    setBusy(true);
    try {
      await pinChannel(channelId, !pinned);
      onChange?.(!pinned);
    } catch {
      // toast handled upstream — keep button responsive
    } finally {
      setBusy(false);
    }
  };

  return (
    <button
      className="btn btn-sm btn-pin"
      data-action={pinned ? 'unpin' : 'pin'}
      disabled={busy}
      onClick={handleClick}
    >
      {pinned ? '取消置顶' : '置顶'}
    </button>
  );
}

export default PinButton;
