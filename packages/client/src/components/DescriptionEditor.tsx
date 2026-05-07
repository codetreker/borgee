// DescriptionEditor — CHN-10.3 channel description editor modal.
// 文案 byte-identical 跟 docs/qa/chn-10-content-lock.md §1.
// CHN-14.3 加历史按钮 (跟 chn-14-content-lock.md §5 byte-identical).
// gh#703 加 useUnsavedChangesGuard — sidepane 切换前如果用户改了 description
// 但没保存, 弹 confirm 让用户决定要不要丢. isDirty 按 value !== initial 推算.
import React, { useState } from 'react';
import { setChannelDescription, DESCRIPTION_MAX_LENGTH } from '../lib/api';
import { DescriptionHistoryModal } from './DescriptionHistoryModal';
import { useUnsavedChangesGuard } from '../hooks/useUnsavedChangesGuard';

interface Props {
  channelID: string;
  initial: string;
  onSaved: (description: string) => void;
  onCancel: () => void;
}

export function DescriptionEditor({ channelID, initial, onSaved, onCancel }: Props) {
  const [value, setValue] = useState<string>(initial);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<boolean>(false);
  // CHN-14.3 — 编辑历史 modal trigger.
  const [showHistory, setShowHistory] = useState<boolean>(false);

  // gh#703 — sidepane 切换守卫. busy 中 (await save) 不算 dirty (用户点了保存
  // 在等结果, 不应该再弹一次确认). 跟 initial prop 比, 不跟服务器最新比 (服务器
  // 端有人改了不归 client 守卫管, 那是另一条 race 路径).
  useUnsavedChangesGuard(
    () => !busy && value !== initial,
    '频道说明有未保存改动, 离开会丢失. 确认离开吗?',
  );

  async function handleSave() {
    if (value.length > DESCRIPTION_MAX_LENGTH) {
      setError(`频道说明不能超过 ${DESCRIPTION_MAX_LENGTH} 字符`);
      return;
    }
    setBusy(true);
    try {
      await setChannelDescription(channelID, value);
      onSaved(value);
    } catch {
      setError('保存频道说明失败');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      className="description-editor"
      data-testid="description-editor"
      role="dialog"
    >
      <header className="description-editor-header">
        <h3>频道说明</h3>
      </header>
      <textarea
        className="description-editor-textarea"
        data-testid="description-editor-input"
        value={value}
        onChange={(e) => {
          setValue(e.target.value);
          setError(null);
        }}
        maxLength={DESCRIPTION_MAX_LENGTH}
      />
      <span className="description-editor-counter">
        {value.length}/{DESCRIPTION_MAX_LENGTH}
      </span>
      {error && (
        <div
          className="description-editor-error"
          data-testid="description-editor-error"
          role="alert"
        >
          {error}
        </div>
      )}
      <div className="description-editor-actions">
        <button
          type="button"
          data-testid="description-save"
          onClick={handleSave}
          disabled={busy}
        >
          保存
        </button>
        <button
          type="button"
          data-testid="description-cancel"
          onClick={onCancel}
          disabled={busy}
        >
          取消
        </button>
        <button
          type="button"
          data-testid="description-history-trigger"
          onClick={() => setShowHistory(true)}
          disabled={busy}
        >
          查看编辑历史
        </button>
      </div>
      {showHistory && (
        <DescriptionHistoryModal
          channelID={channelID}
          onClose={() => setShowHistory(false)}
        />
      )}
    </div>
  );
}
