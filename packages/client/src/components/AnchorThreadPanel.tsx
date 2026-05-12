// AnchorThreadPanel — CV-2.3 client SPA anchor side panel (#360 server).
//
// Blueprint: docs/blueprint/current/canvas-vision.md §1.4 (artifact 集合) +
// §1.6 (锚点对话 = owner review agent 产物的工具). Spec brief:
// docs/implementation/modules/cv-2-spec.md §0 (3 条原则) + §1 段 CV-2.3.
// Acceptance: docs/qa/acceptance-templates/cv-2.md §3. Content-lock:
// docs/qa/cv-2-content-lock.md (7 文案锁, byte-identical).
//
// 设计反查 (cv-2-spec.md §0):
//   - ① 锚点 = 人审 agent 产物 — UI 入口仅 owner / human (DOM 反约束:
//     agent 视角 createAnchor 入口不渲染, 服务端兜底 403
//     `anchor.create_owner_only`).
//   - ② anchor pin 创建时 artifact_version_id, 不跨版本迁移 — stale
//     标签 (anchor 指向版本号 vs 当前 head) 字面锁 §1.6 review 历史.
//   - ③ AnchorCommentAdded envelope 仅信号 (10 字段 byte-identical 锁
//     anchor_comment_frame.go) — 收到后必须 GET /comments 拉 body.
//   - ⑦ channel 权限继承 — 不自起 anchor-level 权限层 (server 端
//     canAccessChannel 双轴对齐 CHN-1 #286 同源).
//
// 反约束 (本组件强制 grep 锚, byte-identical 跟 cv-2-content-lock.md):
//   - ① "评论此段" tooltip — 不准 "Comment" / "添加评论" / "回复" / "讨论"
//   - ② "段落讨论" header — 不准 "评论区" / "Comments" / "Thread"
//   - ③ "针对此段写下你的 review…" placeholder — 不准 "输入评论" / "Write a comment"
//   - ④ 🤖 角标 byte-identical 跟 CV-1 #347 line 251 同源
//   - ⑥ '标为已解决' / '重新打开' — 不准英文同义词 / 完成 / Done (anchor-content-lock.test.ts ⑥ enforces forbidden terms)
//   - ⑦ stale 标签 "锚点指向 v{N}, 文档已更新到 v{M}" byte-identical
//     (#358 acceptance §3.4 同源)

import { useCallback, useEffect, useState } from 'react';
import {
  ApiError,
  type AnchorThread,
  type AnchorComment,
  addAnchorComment,
  listAnchorComments,
  resolveAnchor,
} from '../lib/api';
import { useAnchorCommentAdded } from '../hooks/useWsHubFrames';

interface Props {
  anchor: AnchorThread;
  /**
   * Mapping from artifact_version_id (PK) → user-facing version int.
   * Owner side ArtifactPanel passes the head version + lookup so the
   * stale label can render `锚点指向 v{N}, 文档已更新到 v{M}` byte-identical.
   */
  anchorVersion: number;
  headVersion: number;
  /** 设计 ⑦ resolve = creator OR channel owner; non-eligible DOM omits btn. */
  canResolve: boolean;
  /** Caller controls open/close — the entry trigger lives in ArtifactPanel. */
  onClose: () => void;
  /** After resolve, parent reloads anchor list. */
  onResolved: () => void;
}

// Content-lock constants — byte-identical 跟 docs/qa/cv-2-content-lock.md
// §1 字面表 + acceptance §3.4. Drift on these strings → reverse grep
// fail (cv-2-content-lock.md §2).
const HEADER_LITERAL = '段落讨论';
const PLACEHOLDER_LITERAL = '针对此段写下你的 review…';
const RESOLVE_LITERAL = '标为已解决';
const REOPEN_LITERAL = '重新打开';

function staleLabel(anchorVersion: number, headVersion: number): string {
  // §3.4 byte-identical 跟 #358 acceptance + cv-2-content-lock.md ⑦.
  return `锚点指向 v${anchorVersion}, 文档已更新到 v${headVersion}`;
}

export default function AnchorThreadPanel({
  anchor,
  anchorVersion,
  headVersion,
  canResolve,
  onClose,
  onResolved,
}: Props) {
  const [comments, setComments] = useState<AnchorComment[]>([]);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const reload = useCallback(
    async (anchorId: string) => {
      try {
        const { comments } = await listAnchorComments(anchorId);
        setComments(comments);
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) {
          setComments([]);
        }
      }
    },
    [],
  );

  useEffect(() => {
    void reload(anchor.id);
  }, [anchor.id, reload]);

  // 设计 ③: WS frame is signal-only — body comes from REST GET above.
  const onFrame = useCallback(
    (frame: { anchor_id: string }) => {
      if (frame.anchor_id !== anchor.id) return;
      void reload(anchor.id);
    },
    [anchor.id, reload],
  );
  useAnchorCommentAdded(onFrame);

  const handleSubmit = async () => {
    const body = draft.trim();
    if (!body) return;
    setBusy(true);
    setErr(null);
    try {
      const c = await addAnchorComment(anchor.id, body);
      // Optimistic append; WS frame will idempotent-rehydrate.
      setComments((prev) => (prev.some((p) => p.id === c.id) ? prev : [...prev, c]));
      setDraft('');
    } catch (e) {
      setErr(e instanceof Error ? e.message : '提交失败');
    } finally {
      setBusy(false);
    }
  };

  const handleResolve = async () => {
    setBusy(true);
    setErr(null);
    try {
      await resolveAnchor(anchor.id);
      onResolved();
    } catch (e) {
      setErr(e instanceof Error ? e.message : '操作失败');
    } finally {
      setBusy(false);
    }
  };

  const isResolved = anchor.resolved_at != null;
  const isStale = anchorVersion < headVersion;

  return (
    <div
      className={`anchor-thread${isResolved ? ' anchor-thread-resolved' : ''}`}
      data-anchor-id={anchor.id}
      {...(isResolved ? { 'data-resolved': 'true' } : {})}
      {...(isStale ? { 'data-anchor-stale': 'true' } : {})}
    >
      <div className="anchor-thread-header">
        <h4 className="anchor-thread-title">{HEADER_LITERAL}</h4>
        <button className="anchor-thread-close" onClick={onClose} title="关闭">
          ×
        </button>
      </div>

      {isStale && (
        <div className="anchor-stale-row">
          <span className="anchor-stale-label" data-anchor-stale="true">
            {staleLabel(anchorVersion, headVersion)}
          </span>
        </div>
      )}

      <ul className="anchor-comment-list">
        {comments.map((c) => {
          const isAgent = c.author_kind === 'agent';
          // ④ byte-identical 跟 CV-1 ArtifactPanel kindBadge 同源.
          const kindBadge = isAgent ? '🤖' : '👤';
          return (
            <li key={c.id} className="anchor-comment-row">
              <span
                className="anchor-reply-author"
                data-kind={c.author_kind}
                title={c.author_kind}
              >
                {kindBadge} {c.author_id}
              </span>
              <span className="anchor-comment-body">{c.body}</span>
            </li>
          );
        })}
        {comments.length === 0 && (
          <li className="anchor-comment-empty">暂无评论</li>
        )}
      </ul>

      {!isResolved && (
        <div className="anchor-thread-input">
          <textarea
            className="anchor-thread-textarea"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder={PLACEHOLDER_LITERAL}
            rows={3}
            disabled={busy}
          />
          <div className="anchor-thread-actions">
            <button
              className="btn btn-primary btn-sm"
              disabled={busy || !draft.trim()}
              onClick={handleSubmit}
            >
              {busy ? '提交中…' : '提交'}
            </button>
            {canResolve && (
              <button
                className="btn btn-sm anchor-resolve-btn"
                disabled={busy}
                onClick={handleResolve}
              >
                {RESOLVE_LITERAL}
              </button>
            )}
          </div>
          {err && <p className="anchor-err">{err}</p>}
        </div>
      )}

      {isResolved && canResolve && (
        <div className="anchor-thread-actions">
          {/* Negative constraint: resolved → reopen 文案锁; v1 服务端不暴露 reopen 路径,
              UI 仅占位 byte-identical 锁内容防止文案漂移; 服务端 v2 加 reopen
              endpoint 时此按钮启用. */}
          <button className="btn btn-sm anchor-reopen-btn" disabled title="v1 暂不支持重开">
            {REOPEN_LITERAL}
          </button>
        </div>
      )}
    </div>
  );
}
