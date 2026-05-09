// ArchivedChannelsPanel.tsx — CHN-5.3 archived channels 折叠面板.
//
// Blueprint: channel-model.md §2 不变量 #3 archive 留 history. Spec:
// docs/implementation/modules/chn-5-spec.md (战马D v0). Content-lock:
// docs/qa/chn-5-content-lock.md §2 (DOM byte-identical) + §3 (toast).
//
// 反约束 (chn-5-content-lock.md §2+§3+§4):
//   - DOM byte-identical: <details> + <summary>已归档频道</summary> +
//     data-testid="archived-channels-panel" + data-archived="true" +
//     button data-action="restore" + 文案 `恢复` 1 字 / `已归档` badge.
//   - 同义词反向 reject: grep 检查 `存档/封存/还原/解档/重启/restore` 0 hit.
//   - lib/api.ts::listArchivedChannels / archiveChannel 单源调用.
//
// CHN-1.3 #288 SortableChannelItem `已归档` badge 字面 byte-identical.
import { useEffect, useState } from 'react';
import type { Channel } from '../types';
import { listArchivedChannels, archiveChannel } from '../lib/api';

export function ArchivedChannelsPanel({ onRestore }: { onRestore?: (id: string) => void }) {
  const [channels, setChannels] = useState<Channel[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listArchivedChannels();
      setChannels(list);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const handleRestore = async (channelId: string) => {
    try {
      await archiveChannel(channelId, false);
      setChannels(prev => prev.filter(c => c.id !== channelId));
      onRestore?.(channelId);
    } catch (err) {
      setError(err instanceof Error ? err.message : '恢复失败');
    }
  };

  return (
    <details className="archived-panel" data-testid="archived-channels-panel">
      <summary className="archived-panel-summary">已归档频道</summary>
      {loading && <p className="archived-panel-loading">加载中...</p>}
      {error && <p className="archived-panel-error">{error}</p>}
      {!loading && channels.length === 0 && (
        <p className="archived-panel-empty">暂无已归档频道</p>
      )}
      <ul className="archived-channel-list">
        {channels.map(ch => (
          <li
            key={ch.id}
            className="archived-channel-item"
            data-archived="true"
          >
            <span className="channel-name">#{ch.name}</span>
            <span className="archived-badge" title="已归档">已归档</span>
            <button
              className="btn btn-sm btn-restore"
              data-action="restore"
              onClick={() => handleRestore(ch.id)}
            >
              恢复
            </button>
          </li>
        ))}
      </ul>
    </details>
  );
}

export default ArchivedChannelsPanel;
