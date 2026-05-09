import React from 'react';
import type { User } from '../types';

interface Props {
  users: User[];
  query: string;
  onSelect: (user: User) => void;
  onDismiss: () => void;
  visible: boolean;
  selectedIndex: number;
}

const isMobile = () => window.innerWidth <= 768;

// DM-2.3 (#377) §0 设计 ① — placeholder 字面 byte-identical (跟 #314 文案锁
// + #377 §3 grep 锚 "输入 @ 提到 channel 成员"). 显在空状态 (filtered.length===0)
// 时, 提示用户继续打字. 不漂同义词 (Mention/提及/@提到/@他 全 0 hit).
export const MENTION_PICKER_PLACEHOLDER = '输入 @ 提到 channel 成员…';

export default function MentionPicker({ users, query, onSelect, onDismiss, visible, selectedIndex }: Props) {
  if (!visible) return null;

  const q = query.toLowerCase();
  // DM-2.3 (#377) §0 反约束 — admin 不入候选 (ADM-0 §1.3 红线). channel
  // members API (CHN-1 #286) 已不返 admin, 且 User.role 类型 = 'member' |
  // 'agent' 编译期就拒 'admin' (此 filter 不再需要 — TypeScript types
  // 即 belt; 保留注释作为 grep 检查项 + 设计说明).
  // 蓝图 §1.5 二元 🤖↔👤 字面: role 仅 'agent' / 'member' 入候选.
  const filtered = users
    .filter(u =>
      u.display_name.toLowerCase().includes(q) ||
      u.id.toLowerCase().includes(q),
    )
    .slice(0, 10);

  if (filtered.length === 0) {
    // 空状态显字面锁 placeholder; grep 检查 `MENTION_PICKER_PLACEHOLDER`
    // count==1 (此处定义) + DOM 渲染 1 hit (#377 §3 acceptance).
    return (
      <div className="mention-picker mention-picker-empty">
        <span className="mention-empty-hint">{MENTION_PICKER_PLACEHOLDER}</span>
      </div>
    );
  }

  const picker = (
    <div className="mention-picker">
      {filtered.map((user, idx) => (
        <button
          key={user.id}
          className={`mention-option ${idx === selectedIndex ? 'mention-option-active' : ''}`}
          data-kind={user.role === 'agent' ? 'agent' : 'user'}
          data-user-id={user.id}
          onMouseDown={(e) => {
            e.preventDefault();
            // DM-2.3 (#377) §0 设计 ① — 选中回填 user_id token 由
            // MessageInput 端处理 (走 createMentionExtension); 这里仅
            // 把 user 对象交给上层 callback. 反约束: 不在此处把 user.id
            // 拼成 display_name 串 (设计 ① 字面禁; 反查 grep
            // template-literal display_name 反约束行 0 hit).
            onSelect(user);
          }}
        >
          <span className="mention-avatar" style={{ backgroundColor: stringToColor(user.id) }}>
            {user.display_name[0]?.toUpperCase()}
          </span>
          <span className="mention-status-icon">
            {user.role === 'agent' ? '🤖' : '👤'}
          </span>
          <span className="mention-name">{user.display_name}</span>
          <span className="mention-id">({user.id})</span>
          {user.role === 'agent' && <span className="user-badge">Bot</span>}
        </button>
      ))}
    </div>
  );

  if (isMobile()) {
    return (
      <>
        <div className="mention-backdrop" onMouseDown={onDismiss} onTouchStart={onDismiss} />
        {picker}
      </>
    );
  }

  return picker;
}

function stringToColor(str: string): string {
  const colors = [
    '#e74c3c', '#e67e22', '#f1c40f', '#2ecc71', '#1abc9c',
    '#3498db', '#9b59b6', '#e91e63', '#00bcd4', '#ff5722',
  ];
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }
  return colors[Math.abs(hash) % colors.length]!;
}
