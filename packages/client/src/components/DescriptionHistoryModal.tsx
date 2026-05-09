// DescriptionHistoryModal — CHN-14.3 channel description edit history viewer.
//
// 文案 byte-identical 跟 docs/qa/chn-14-content-lock.md §1.
// 跟 DM-7 EditHistoryModal 同模式 (owner-only fetched at parent level).
import React, { useEffect, useState } from 'react';
import { getChannelDescriptionHistory, type CHN14DescriptionHistoryEntry } from '../lib/api';

interface Props {
  channelID: string;
  onClose: () => void;
}

export function DescriptionHistoryModal({ channelID, onClose }: Props) {
  const [history, setHistory] = useState<CHN14DescriptionHistoryEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getChannelDescriptionHistory(channelID)
      .then((resp) => {
        if (!cancelled) setHistory(resp.history || []);
      })
      .catch(() => {
        if (!cancelled) setError('加载编辑历史失败');
      });
    return () => {
      cancelled = true;
    };
  }, [channelID]);

  if (error) {
    return (
      <div data-testid="description-history-modal-error" role="alert">
        {error}
      </div>
    );
  }
  // CHN-14 设计略别于 DM-7 — empty 也渲染 modal + 显式空态文案 `暂无编辑记录`
  // (DM-7 设计是空 → return null; CHN-14 owner-only 触发, 空也要给反馈).
  if (history === null) {
    return null;
  }

  return (
    <div
      className="description-history-modal"
      data-testid="description-history-modal"
      role="dialog"
    >
      <header className="description-history-header">
        <h3>编辑历史</h3>
        <button
          type="button"
          className="description-history-close"
          data-testid="description-history-close"
          onClick={onClose}
          aria-label="关闭"
        >
          ×
        </button>
      </header>
      {history.length === 0 ? (
        <div className="description-history-empty" data-testid="description-history-empty">
          暂无编辑记录
        </div>
      ) : (
        <ul className="description-history-list">
          {history.map((entry, i) => (
            <li
              key={i}
              className="description-history-entry"
              data-history-index={i}
            >
              <time
                dateTime={new Date(entry.ts).toISOString()}
                className="description-history-ts"
              >
                {new Date(entry.ts).toISOString()}
              </time>
              <span className="description-history-action">: 修改了说明</span>
              <pre className="description-history-old-content">{entry.old_content}</pre>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
