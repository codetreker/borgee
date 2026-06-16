import React, { useMemo, useState, useCallback, useEffect } from 'react';
import { renderMarkdown } from '../lib/markdown';
import { parseFileLinks } from '../lib/file-links';
import ReactionBar from './ReactionBar';
import ReactionAddButton from './ReactionAddButton';
import EditEditor from './EditEditor';
import FileLink from './FileLink';
import * as api from '../lib/api';
import SystemMessageBubble, { isBPP32GrantPayload } from './SystemMessageBubble';
import { useAppContext } from '../context/AppContext';
import { useLongPress } from '../hooks/useLongPress';
import CommandResultCard from './CommandResultCard';
import { trackCommand, getCommandStatus } from '../hooks/useCommandTracking';
import type { Message } from '../types';
import type { ChannelMember } from '../lib/api';

interface Props {
  message: Message;
  userMap: Map<string, string>;
  members: ChannelMember[];
  memberMap: Map<string, ChannelMember>;
  currentUserId?: string;
  currentUserRole?: string;
  onRetry?: (message: Message) => void;
}

export default function MessageItem({ message, userMap, members, memberMap, currentUserId, currentUserRole, onRetry }: Props) {
  const { dispatch } = useAppContext();
  const isSystem = message.sender_id === 'system';
  const senderName = isSystem ? '系统' : (message.sender_name ?? userMap.get(message.sender_id) ?? 'Unknown');
  const isOwn = message.sender_id === currentUserId;
  const isAdmin = currentUserRole === 'admin';
  const isDeleted = !!message.deleted_at;
  const time = formatTime(message.created_at);

  const avatarLetter = senderName[0]?.toUpperCase() ?? '?';
  const avatarColor = stringToColor(message.sender_id);

  const [editing, setEditing] = useState(false);
  const [editSaving, setEditSaving] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [mobileActionsOpen, setMobileActionsOpen] = useState(false);

  const commandData = useMemo(() => {
    if (message.content_type !== 'command') return null;
    try { return JSON.parse(message.content) as { command: string; params?: string }; }
    catch { return null; }
  }, [message.content, message.content_type]);

  const [commandStatus, setCommandStatus] = useState(() => getCommandStatus(message.id));

  useEffect(() => {
    if (message.content_type !== 'command') return;
    trackCommand(message.id);
    const interval = setInterval(() => {
      setCommandStatus(getCommandStatus(message.id));
    }, 1000);
    return () => clearInterval(interval);
  }, [message.id, message.content_type]);

  const renderedContent = useMemo(() => {
    if (message.content_type === 'image' || message.content_type === 'command') return null;
    let html = renderMarkdown(message.content, message.mentions, userMap);
    html = html.replace(
      /\[workspace:([a-f0-9]+):([^\]]+)\]/g,
      (_match, fileId: string, fileName: string) =>
        `<span class="workspace-ref-card" data-file-id="${fileId}"><span class="workspace-ref-card-name">📎 ${fileName}</span></span>`,
    );
    return html;
  }, [message.content, message.content_type, message.mentions, userMap]);

  const senderUser = memberMap.get(message.sender_id);
  const agentId = senderUser?.role === 'agent' ? message.sender_id : null;

  const startEdit = useCallback(() => {
    setEditing(true);
  }, []);

  const cancelEdit = useCallback(() => {
    setEditing(false);
  }, []);

  const saveEdit = useCallback(async (newContent: string) => {
    const trimmed = newContent.trim();
    if (!trimmed || trimmed === message.content) {
      cancelEdit();
      return;
    }
    setEditSaving(true);
    try {
      const updated = await api.editMessage(message.id, trimmed);
      dispatch({
        type: 'EDIT_MESSAGE',
        channelId: message.channel_id,
        messageId: message.id,
        content: updated.content,
        editedAt: updated.edited_at!,
      });
      setEditing(false);
    } catch {
      // keep editing on failure
    } finally {
      setEditSaving(false);
    }
  }, [message, dispatch, cancelEdit]);

  const handleDelete = useCallback(async () => {
    try {
      await api.deleteMessage(message.id);
    } catch {
      // ignore
    }
    setShowDeleteConfirm(false);
  }, [message.id]);

  const canEdit = isOwn && !isDeleted && !message._pending && !message._failed;
  const canDelete = (isOwn || isAdmin) && !isDeleted && !message._pending && !message._failed;
  // gh#686 §4 #4 #5: 只有当消息没 reaction 时, 才把 + 加到 .message-actions
  // 工具栏 (跟 edit/delete 一组). 已经有 reaction 时, ReactionBar 自带 +
  // 在那一行不重复出现. 这样无 reaction 状态消息容器不撑高.
  const hasReactions = (message.reactions?.length ?? 0) > 0;
  const canAddReaction =
    !isDeleted &&
    !message._pending &&
    !message._failed &&
    !hasReactions &&
    !!currentUserId;

  const longPressHandlers = useLongPress(() => {
    if (canEdit || canDelete || canAddReaction) setMobileActionsOpen(true);
  });

  if (isSystem) {
    // CM-onboarding + BPP-3.2.2 system DM render path. Delegated to
    // SystemMessageBubble so it can be unit-tested without an AppContext
    // provider. Detection: parse quick_action JSON; if shape matches
    // BPP-3.2 GrantPayload (action ∈ grant/reject/snooze + 4 fields),
    // render three buttons; else fall back to CM-onboarding single-button.
    let parsed: unknown = null;
    if (message.quick_action) {
      try { parsed = JSON.parse(message.quick_action); } catch { parsed = null; }
    }
    const bpp32 = isBPP32GrantPayload(parsed) ? parsed : null;
    const fallback = (!bpp32 && parsed && typeof parsed === 'object')
      ? (parsed as { kind?: string; label?: string; action?: string })
      : null;
    return (
      <SystemMessageBubble
        bodyHTML={renderedContent!}
        bpp32={bpp32}
        fallback={fallback}
      />
    );
  }

  return (
    <div className={`message-item ${isOwn ? 'message-own' : ''}`} {...longPressHandlers}>
      <div className="message-avatar" style={{ backgroundColor: avatarColor }}>
        {avatarLetter}
      </div>
      <div className="message-body">
        <div className="message-header">
          <span className="message-sender">{senderName}</span>
          <span className="message-time">{time}</span>
          {message.edited_at && !isDeleted && (
            <span className="message-edited" title={`已编辑于 ${formatTime(message.edited_at)}`}>
              (已编辑)
            </span>
          )}
          {isOwn && (
            <span className="message-delivery-status">
              {message._pending && '⏳'}
              {message._failed && (
                <>
                  ❌
                  {onRetry && (
                    <button className="retry-btn" onClick={() => onRetry(message)}>重试</button>
                  )}
                </>
              )}
              {!message._pending && !message._failed && <span className="delivery-check">✓</span>}
            </span>
          )}
        </div>
        <div className="message-content">
          {isDeleted ? (
            <div className="message-deleted">此消息已删除</div>
          ) : editing ? (
            <EditEditor
              initialContent={message.content}
              onSave={saveEdit}
              onCancel={cancelEdit}
              disabled={editSaving}
              users={members}
            />
          ) : message.content_type === 'command' && commandData ? (
            <div className="message-command">
              <div className="message-command-label">⚡ {commandData.command}{commandData.params ? ` ${commandData.params}` : ''}</div>
              {commandStatus && <CommandResultCard status={commandStatus} />}
            </div>
          ) : message.content_type === 'image' ? (
            <ImageContent url={message.content} />
          ) : (
            <MessageTextWithFileLinks html={renderedContent!} rawContent={message.content} agentId={agentId} />
          )}
        </div>
        {!isDeleted && !editing && (
          <ReactionBar
            reactions={message.reactions ?? []}
            channelId={message.channel_id}
            messageId={message.id}
            currentUserId={currentUserId}
            userMap={userMap}
          />
        )}
      </div>
      {!isDeleted && !editing && (canEdit || canDelete || canAddReaction) && (
        <div className="message-actions">
          {canAddReaction && currentUserId && (
            <ReactionAddButton
              channelId={message.channel_id}
              messageId={message.id}
              currentUserId={currentUserId}
              variant="toolbar-btn"
            />
          )}
          {canEdit && (
            <button className="message-action-btn" onClick={startEdit} title="编辑">
              ✏️
            </button>
          )}
          {canDelete && (
            <button className="message-action-btn message-action-delete" onClick={() => setShowDeleteConfirm(true)} title="删除">
              🗑️
            </button>
          )}
        </div>
      )}
      {showDeleteConfirm && (
        <div className="message-delete-overlay" onClick={() => setShowDeleteConfirm(false)}>
          <div className="message-delete-dialog" onClick={e => e.stopPropagation()}>
            <p>确定删除这条消息？</p>
            <div className="message-delete-dialog-actions">
              <button className="btn-cancel" onClick={() => setShowDeleteConfirm(false)}>取消</button>
              <button className="btn-danger" onClick={handleDelete}>删除</button>
            </div>
          </div>
        </div>
      )}
      {mobileActionsOpen && (
        <div className="mobile-action-sheet-overlay" onClick={() => setMobileActionsOpen(false)}>
          <div className="mobile-action-sheet" onClick={e => e.stopPropagation()}>
            {canEdit && (
              <button className="mobile-action-sheet-btn" onClick={() => { setMobileActionsOpen(false); startEdit(); }}>
                ✏️ 编辑
              </button>
            )}
            {canDelete && (
              <button className="mobile-action-sheet-btn mobile-action-sheet-danger" onClick={() => { setMobileActionsOpen(false); setShowDeleteConfirm(true); }}>
                🗑️ 删除
              </button>
            )}
            <button className="mobile-action-sheet-btn mobile-action-sheet-cancel" onClick={() => setMobileActionsOpen(false)}>
              取消
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function MessageTextWithFileLinks({ html, rawContent, agentId }: { html: string; rawContent: string; agentId: string | null }) {
  const segments = parseFileLinks(rawContent);
  const hasPaths = segments.some(s => s.type === 'path');

  if (!hasPaths) {
    return <div className="message-text" dangerouslySetInnerHTML={{ __html: html }} />;
  }

  const paths = segments.filter(s => s.type === 'path').map(s => s.value);
  const escaped = paths.map(p => p.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
  const re = new RegExp(`(${escaped.join('|')})`);
  const htmlParts = html.split(re);
  const pathSetLower = new Set(paths);

  return (
    <div className="message-text">
      {htmlParts.map((part, i) =>
        pathSetLower.has(part)
          ? <FileLink key={i} path={part} agentId={agentId} />
          : <span key={i} dangerouslySetInnerHTML={{ __html: part }} />
      )}
    </div>
  );
}

function ImageContent({ url }: { url: string }) {
  const [error, setError] = React.useState(false);

  // borgee #1108 F5: only render an <img src>/<a href> for URLs that pass the
  // same allowlist the server enforces at write time — http(s):// (any case)
  // OR a same-origin relative path with a single leading '/' (NOT '//', which
  // is protocol-relative and unsafe). Anything else (javascript:, data:, etc.)
  // is shown as inert plain text so a stored row can't become a phishing /
  // script-anchor vector. Required regardless of the server guard because the
  // server can't fix rows persisted before the fix.
  const isSafe = /^https?:\/\//i.test(url) || /^\/(?!\/)/.test(url);
  if (!isSafe) {
    return <span className="message-image-unsafe">{url}</span>;
  }

  if (error) {
    return (
      <div className="image-error">
        <span>🖼️ 图片加载失败</span>
        <a href={url} target="_blank" rel="noopener noreferrer" className="image-fallback-link">
          {url}
        </a>
      </div>
    );
  }

  return (
    <a href={url} target="_blank" rel="noopener noreferrer">
      <img
        src={url}
        alt="uploaded image"
        className="message-image"
        onError={() => setError(true)}
        loading="lazy"
      />
    </a>
  );
}

function formatTime(ts: number): string {
  const date = new Date(ts);
  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();
  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);
  const isYesterday = date.toDateString() === yesterday.toDateString();

  const timeStr = date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });

  if (isToday) return timeStr;
  if (isYesterday) return `昨天 ${timeStr}`;
  return `${date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })} ${timeStr}`;
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
