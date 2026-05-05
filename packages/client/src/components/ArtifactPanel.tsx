// ArtifactPanel — CV-1.3 client SPA canvas UI (#342 server / #334 schema).
//
// Blueprint: docs/blueprint/current/canvas-vision.md §0 (channel 围 artifact 协作)
// + §1.1-§1.6 (D-lite + workspace per channel + Markdown ONLY v1).
// Spec: docs/implementation/modules/cv-1-spec.md §3 (CV-1.3 段).
// Acceptance: docs/qa/acceptance-templates/cv-1.md §3.1-§3.3.
// Stance: docs/qa/cv-1-stance-checklist.md (v0, 7 立场) +
// docs/qa/cv-1-stance-v1-supplement.md (②③⑤⑦ v1 字段).
//
// 立场反查:
//   - ① 归属 = channel — 列表只显示当前 channel 的 artifacts; 没有 author owner.
//   - ② 单文档锁 30s TTL — 编辑提交收 409 → toast 字面 "内容已更新, 请刷新查看".
//   - ③ 版本线性 — sidebar 列表升序 version, 不删中间版本; rollback 也是新增 row.
//   - ④ Markdown ONLY — 永远渲染 marked + DOMPurify, 不接受 type 切换 (v1).
//   - ⑤ Frame 仅信号 — WS artifact_updated 收到后必须 GET /api/v1/artifacts/:id
//     才能拿 body / committer (envelope 不带 body); client 不能用 updated_at 排序.
//   - ⑥ committer_kind 'agent'|'human' — version row 渲染人/agent 标签 (head from GET).
//   - ⑦ rollback owner-only — 仅 channel.created_by 看到 "回滚" 按钮; 非 owner DOM
//     不渲染该按钮.
//
// 反约束 (本组件强制 grep 锚):
//   - 不上 CRDT (no yjs / no automerge — pure REST + WS signal)
//   - 不自造 envelope (使用 useArtifactUpdated hook, 走 #342 frame)
//   - 不用 client timestamp 排序 (列表按 version asc, RT-1 ① 反约束)
//   - rollback 不是 PATCH body 字段 (调 rollbackArtifact action endpoint)

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useAppContext } from '../context/AppContext';
import { useToast } from './Toast';
import { useArtifactUpdated, useAnchorCommentAdded } from '../hooks/useWsHubFrames';
import { renderMarkdown } from '../lib/markdown';
import CodeRenderer from './CodeRenderer';
import ImageLinkRenderer from './ImageLinkRenderer';
import MediaPreview from './MediaPreview';
import {
  ApiError,
  type Artifact,
  type ArtifactKind,
  type ArtifactVersion,
  type AnchorThread,
  commitArtifact,
  createArtifact,
  createAnchor,
  getArtifact,
  listAnchors,
  listArtifactVersions,
  rollbackArtifact,
} from '../lib/api';
import AnchorThreadPanel from './AnchorThreadPanel';
import IteratePanel from './IteratePanel';
import DiffView, { parseDiffParam, formatDiffParam } from './DiffView';

interface Props {
  channelId: string;
}

// Conflict toast 文案锁 (acceptance §3.3 byte-identical) — 任何 commit
// 路径 409 都走这条; 其它 409 (e.g. 锁持有=别人) 也复用同文案保持一致.
const CONFLICT_TOAST = '内容已更新, 请刷新查看';

// CV-2.3 anchor entry tooltip — byte-identical 跟 docs/qa/cv-2-content-lock.md
// 字面表 ① ("评论此段"). 不准 "Comment" / "添加评论" / "回复" / "讨论".
const ANCHOR_ENTRY_TOOLTIP = '评论此段';

export default function ArtifactPanel({ channelId }: Props) {
  const { state } = useAppContext();
  const { showToast } = useToast();
  const currentUser = state.currentUser;
  const channel = state.channels.find((c) => c.id === channelId);
  // 立场 ⑦ rollback owner = channel.created_by (channel-model §1.4).
  // 立场 ① CV-2 anchor entry: 仅 human (role !== 'agent') 看到 💬 入口.
  // (反约束: agent 视角 DOM 不渲染 ① hover 入口, byte-identical 跟
  // CV-1 立场 ⑦ rollback owner-only DOM omit 同模式 — 服务端 403
  // anchor.create_owner_only 兜底.)
  const isOwner = !!currentUser && channel?.created_by === currentUser.id;
  const isHuman = !!currentUser && currentUser.role !== 'agent';

  const [artifact, setArtifact] = useState<Artifact | null>(null);
  const [versions, setVersions] = useState<ArtifactVersion[]>([]);
  const [editing, setEditing] = useState(false);
  const [editBody, setEditBody] = useState('');
  const [busy, setBusy] = useState(false);
  const [errMsg, setErrMsg] = useState<string | null>(null);

  // CV-2.3 anchor state — 选区 → 锚点 entry + side thread panel.
  const [anchors, setAnchors] = useState<AnchorThread[]>([]);
  const [activeAnchorId, setActiveAnchorId] = useState<string | null>(null);
  const [selection, setSelection] = useState<{ start: number; end: number } | null>(null);

  // CV-4.3 diff view state — "对比" tab + URL `?diff=v3..v2` deep-link
  // (content-lock §1 ⑤ + spec #365 §0 立场 ③).
  // diffPair 是当前活跃的 N..M 对比; null = 不在 diff 模式.
  // 立场 ③ — client jsdiff, 不裂 server diff.
  const [diffPair, setDiffPair] = useState<{ newV: number; oldV: number } | null>(() => {
    if (typeof window === 'undefined') return null;
    const raw = new URLSearchParams(window.location.search).get('diff');
    return parseDiffParam(raw);
  });

  // syncDiffURL — 把 diffPair 写回 URL (replaceState, 不污染 history).
  const syncDiffURL = useCallback((pair: { newV: number; oldV: number } | null) => {
    if (typeof window === 'undefined') return;
    const url = new URL(window.location.href);
    if (pair) {
      url.searchParams.set('diff', formatDiffParam(pair.newV, pair.oldV));
    } else {
      url.searchParams.delete('diff');
    }
    window.history.replaceState(null, '', url.toString());
  }, []);

  // 立场 ③ deep-link: 当用户进入 panel 时若 URL `?diff=` 已存在, 取其
  // pair 渲染 diff view. 切换 channel 时清掉 diffPair (相当于 reset).
  useEffect(() => {
    setDiffPair(null);
  }, [channelId]);

  // diffBodies — diff 模式下解出 (newBody, oldBody) 从 versions 列表.
  // versions 已按 version asc 排序 (CV-1 立场 ③), 找用户号 v=N 的 row.
  // hooks-rules — useMemo 必须永远调用 (即使 diffPair 为 null, 列表位置稳定).
  const diffBodies = useMemo(() => {
    if (!diffPair) return null;
    const newRow = versions.find((v) => v.version === diffPair.newV);
    const oldRow = versions.find((v) => v.version === diffPair.oldV);
    if (!newRow || !oldRow) return null;
    return { newBody: newRow.body, oldBody: oldRow.body };
  }, [diffPair, versions]);

  const handleEnterDiff = useCallback((newV: number, oldV: number) => {
    const pair = { newV, oldV };
    setDiffPair(pair);
    syncDiffURL(pair);
  }, [syncDiffURL]);

  const handleExitDiff = useCallback(() => {
    setDiffPair(null);
    syncDiffURL(null);
  }, [syncDiffURL]);

  // Reload artifact + version list. Triggered by initial mount,
  // channel switch, and WS artifact_updated push (立场 ⑤ pull-after-signal).
  const reload = useCallback(
    async (artifactId: string) => {
      try {
        const [head, list] = await Promise.all([
          getArtifact(artifactId),
          listArtifactVersions(artifactId),
        ]);
        setArtifact(head);
        setVersions(list.versions);
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) {
          setArtifact(null);
          setVersions([]);
        }
      }
    },
    [],
  );

  // CV-2.3 reload anchors after WS push or local create. List endpoint
  // is channel-member ACL'd (立场 ⑦); on 403 we silently empty (agent
  // view 反约束 DOM 不渲染入口, list 路径仍可读 thread).
  const reloadAnchors = useCallback(
    async (artifactId: string) => {
      try {
        const { anchors } = await listAnchors(artifactId);
        setAnchors(anchors);
      } catch {
        setAnchors([]);
      }
    },
    [],
  );

  // Reset on channel switch + try to find the channel's existing artifact.
  // CV-1.3 v1: one artifact per channel surface — listing API is out of
  // scope for this PR, so we lazy-create on first interaction. Until the
  // user creates one we render the "create" affordance.
  useEffect(() => {
    setArtifact(null);
    setVersions([]);
    setEditing(false);
    setEditBody('');
    setErrMsg(null);
    setAnchors([]);
    setActiveAnchorId(null);
    setSelection(null);
  }, [channelId]);

  // Reload anchors when artifact lands.
  useEffect(() => {
    if (artifact?.id) {
      void reloadAnchors(artifact.id);
    }
  }, [artifact?.id, reloadAnchors]);

  // 立场 ⑤ — WS push: re-fetch on signal frame for our artifact.
  // The handler closes over the latest artifact.id via useCallback +
  // a stable identity check inside.
  const onArtifactUpdated = useCallback(
    (frame: { artifact_id: string; channel_id: string }) => {
      if (frame.channel_id !== channelId) return;
      if (!artifact || frame.artifact_id !== artifact.id) return;
      void reload(artifact.id);
    },
    [channelId, artifact, reload],
  );
  useArtifactUpdated(onArtifactUpdated);

  // CV-2.3 立场 ③: anchor_comment_added envelope is signal-only; on
  // any landing comment for this artifact, refresh the anchor list so
  // resolved/added counts stay live across tabs.
  const onAnchorCommentAdded = useCallback(
    (frame: { artifact_id: string }) => {
      if (!artifact || frame.artifact_id !== artifact.id) return;
      void reloadAnchors(artifact.id);
    },
    [artifact, reloadAnchors],
  );
  useAnchorCommentAdded(onAnchorCommentAdded);

  // 选区 → 锚点 entry: capture text selection inside the rendered
  // markdown surface. We map DOM selection back to body offsets via
  // textContent of `.artifact-rendered` (the rendered DOM has identical
  // visible text to artifact.body absent inline images, which CV-1
  // markdown-only 立场 ④ guarantees).
  const handleSelection = useCallback(() => {
    if (!artifact || editing) return;
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || sel.rangeCount === 0) {
      setSelection(null);
      return;
    }
    const root = document.querySelector('.artifact-rendered');
    if (!root) return;
    const range = sel.getRangeAt(0);
    if (!root.contains(range.commonAncestorContainer)) return;
    const text = sel.toString();
    if (!text) return;
    // Locate the substring in artifact.body. Falls back to first occurrence;
    // 立场 ② anchor pin is by start/end + version, so first-occurrence in
    // current body is OK — the version_id pin freezes review context.
    const start = artifact.body.indexOf(text);
    if (start < 0) return;
    setSelection({ start, end: start + text.length });
  }, [artifact, editing]);

  const handleCreate = async () => {
    const title = window.prompt('Artifact 标题:', '未命名 artifact');
    if (!title || !title.trim()) return;
    setBusy(true);
    setErrMsg(null);
    try {
      const created = await createArtifact(channelId, { title: title.trim(), body: '' });
      setArtifact(created);
      const list = await listArtifactVersions(created.id);
      setVersions(list.versions);
    } catch (err) {
      setErrMsg(err instanceof Error ? err.message : '创建失败');
    } finally {
      setBusy(false);
    }
  };

  const handleStartEdit = () => {
    if (!artifact) return;
    setEditBody(artifact.body);
    setEditing(true);
    setErrMsg(null);
  };

  // CV-2.3 立场 ① human-only entry: server enforces 403 too. Click
  // commits the current selection as an anchor anchored to the head
  // version (立场 ② version pin = head at create time).
  const handleCreateAnchor = async () => {
    if (!artifact || !selection || !isHuman) return;
    setBusy(true);
    setErrMsg(null);
    try {
      const created = await createAnchor(artifact.id, {
        version: artifact.current_version,
        start_offset: selection.start,
        end_offset: selection.end,
      });
      setSelection(null);
      window.getSelection()?.removeAllRanges();
      await reloadAnchors(artifact.id);
      setActiveAnchorId(created.id);
    } catch (err) {
      setErrMsg(err instanceof Error ? err.message : '创建锚点失败');
    } finally {
      setBusy(false);
    }
  };

  const handleSubmit = async () => {
    if (!artifact) return;
    setBusy(true);
    setErrMsg(null);
    try {
      await commitArtifact(artifact.id, {
        expected_version: artifact.current_version,
        body: editBody,
      });
      // Re-fetch authoritative head + version list.
      await reload(artifact.id);
      setEditing(false);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        // 立场 ② lock conflict / version mismatch — toast 文案锁.
        showToast(CONFLICT_TOAST);
        // Re-fetch so the editor's expected_version moves forward.
        await reload(artifact.id);
      } else {
        setErrMsg(err instanceof Error ? err.message : '提交失败');
      }
    } finally {
      setBusy(false);
    }
  };

  const handleRollback = async (toVersion: number) => {
    if (!artifact) return;
    if (!isOwner) return; // defense in depth — server enforces too
    if (!window.confirm(`确认回滚到 v${toVersion}? 旧版本不会删除, 会新建一条 rollback 记录.`)) return;
    setBusy(true);
    setErrMsg(null);
    try {
      await rollbackArtifact(artifact.id, toVersion);
      await reload(artifact.id);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        showToast(CONFLICT_TOAST);
        await reload(artifact.id);
      } else {
        setErrMsg(err instanceof Error ? err.message : '回滚失败');
      }
    } finally {
      setBusy(false);
    }
  };

  if (!artifact) {
    return (
      <div className="artifact-panel">
        <div className="artifact-empty">
          <p>该频道还没有 artifact</p>
          <button className="btn btn-primary" disabled={busy} onClick={handleCreate}>
            {busy ? '创建中…' : '新建 Markdown artifact'}
          </button>
          {errMsg && <p className="artifact-err">{errMsg}</p>}
        </div>
      </div>
    );
  }

  return (
    <div className="artifact-panel" data-artifact-kind={normalizeKind(artifact.type)}>
      <div className="artifact-header">
        <div className="artifact-title-row">
          <h3 className="artifact-title">{artifact.title}</h3>
          <span className="artifact-version-tag">v{artifact.current_version}</span>
        </div>
        {!editing && (
          <>
            <button className="btn btn-sm" disabled={busy} onClick={handleStartEdit}>
              编辑
            </button>
            {/* CV-4.3 — "对比" tab byte-identical (content-lock §1 ⑤ 单字).
                versions ≥ 2 才显示 (无前一版本无可对比).
                文案锁: "对比" 单字, 反同义词漂移
                (acceptance §3.5 + #380 ⑤). */}
            {versions.length >= 2 && !diffPair && (
              <button
                className="btn btn-sm artifact-diff-btn"
                disabled={busy}
                onClick={() => {
                  // 默认 N..(N-1) 对比 (跟 CV-1 #347 line 254 rollback 相邻
                  // 模式同精神 — 最新两版).
                  const sorted = [...versions].sort((a, b) => b.version - a.version);
                  if (sorted.length >= 2) {
                    handleEnterDiff(sorted[0]!.version, sorted[1]!.version);
                  }
                }}
              >
                对比
              </button>
            )}
            {diffPair && (
              <button
                className="btn btn-sm artifact-diff-exit-btn"
                disabled={busy}
                onClick={handleExitDiff}
              >
                返回
              </button>
            )}
          </>
        )}
      </div>

      <div className="artifact-body-area">
        {editing ? (
          <div className="artifact-edit">
            <textarea
              className="artifact-textarea"
              value={editBody}
              onChange={(e) => setEditBody(e.target.value)}
              rows={20}
              spellCheck={false}
            />
            <div className="artifact-edit-actions">
              <button className="btn btn-primary" disabled={busy} onClick={handleSubmit}>
                {busy ? '提交中…' : '提交'}
              </button>
              <button className="btn" disabled={busy} onClick={() => setEditing(false)}>
                取消
              </button>
            </div>
            {errMsg && <p className="artifact-err">{errMsg}</p>}
          </div>
        ) : diffPair && diffBodies ? (
          // CV-4.3 立场 ③ — client jsdiff 行级 (反 server diff). v0 仅
          // markdown kind 走 diffLines; image_link 走前后缩略图 fallback;
          // code v0 也走 diffLines (CV-3 spec §0 ① 字面: code 是 markdown
          // kind 同源 textual body, jsdiff 适用).
          <DiffView
            newBody={diffBodies.newBody}
            newVersion={diffPair.newV}
            oldBody={diffBodies.oldBody}
            oldVersion={diffPair.oldV}
            kind="markdown"
          />
        ) : (
          // CV-3.3 kind switch + CV-2.3 选区监听共存: ArtifactBody 内
          // 三分支都在外层 div 上落 className "artifact-rendered" (anchor
          // selection 依赖该 selector 定位 markdown body), 选区 handler
          // 包在外层 wrapper 上 — 仅 markdown 分支会触发有意义选区
          // (立场 ④ markdown 才走 dangerouslySetInnerHTML, code/image
          // 是 React 节点, sel.toString() 仍可工作但 anchor 入口语义
          // 主要服务于 markdown 文档协作).
          <div onMouseUp={handleSelection} onKeyUp={handleSelection}>
            <ArtifactBody artifact={artifact} />
          </div>
        )}
        {/* CV-2.3 立场 ① 选区 → 锚点 entry: 仅 human 看到 💬 入口
            (DOM 反约束 — agent 视角 isHuman=false count==0). 文案锁
            byte-identical 跟 cv-2-content-lock.md ① 字面表 (icon 💬 +
            tooltip "评论此段"). */}
        {!editing && isHuman && selection && (
          <button
            className="anchor-comment-btn"
            data-anchor-id="entry"
            title={ANCHOR_ENTRY_TOOLTIP}
            disabled={busy}
            onClick={handleCreateAnchor}
          >
            💬
          </button>
        )}
        {errMsg && !editing && <p className="artifact-err">{errMsg}</p>}
      </div>

      {/* CV-2.3 anchor side panel — list active threads, click → open. */}
      {anchors.length > 0 && (
        <aside className="artifact-anchors">
          <h4>锚点 ({anchors.length})</h4>
          <ul className="artifact-anchor-list">
            {anchors.map((a) => {
              // anchor.artifact_version_id is FK PK; we map to user-facing
              // version int by scanning versions list (created on the same
              // artifact; PK strictly increases with version int).
              const av = versions.find(
                (v) => v.created_at <= a.created_at && v.version <= artifact.current_version,
              );
              const anchorVersionInt = av?.version ?? artifact.current_version;
              const isStale = anchorVersionInt < artifact.current_version;
              const isResolved = a.resolved_at != null;
              return (
                <li
                  key={a.id}
                  className={`artifact-anchor-row${isResolved ? ' resolved' : ''}`}
                  data-anchor-id={a.id}
                  {...(isStale ? { 'data-anchor-stale': 'true' } : {})}
                  onClick={() => setActiveAnchorId(a.id)}
                >
                  <span className="artifact-anchor-range">
                    [{a.start_offset}-{a.end_offset}]
                  </span>
                  {isStale && (
                    <span className="anchor-stale-label" data-anchor-stale="true">
                      锚点指向 v{anchorVersionInt}, 文档已更新到 v{artifact.current_version}
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
        </aside>
      )}

      {activeAnchorId &&
        (() => {
          const active = anchors.find((a) => a.id === activeAnchorId);
          if (!active) return null;
          const av = versions.find(
            (v) => v.created_at <= active.created_at && v.version <= artifact.current_version,
          );
          const anchorVersionInt = av?.version ?? artifact.current_version;
          // 立场 ⑦: resolve = anchor creator OR channel owner. Server
          // enforces; we just gate the UI button.
          const canResolve =
            !!currentUser &&
            (active.created_by === currentUser.id || isOwner);
          return (
            <AnchorThreadPanel
              anchor={active}
              anchorVersion={anchorVersionInt}
              headVersion={artifact.current_version}
              canResolve={canResolve}
              onClose={() => setActiveAnchorId(null)}
              onResolved={() => {
                void reloadAnchors(artifact.id);
              }}
            />
          );
        })()}

      <aside className="artifact-versions">
        <h4>版本</h4>
        <ul className="artifact-version-list">
          {versions.map((v) => {
            const isHead = v.version === artifact.current_version;
            const label =
              v.rolled_back_from_version != null
                ? `v${v.version} (rollback from v${v.rolled_back_from_version})`
                : `v${v.version}`;
            const kindBadge = v.committer_kind === 'agent' ? '🤖' : '👤';
            // 立场 ⑦ owner-only rollback button: 非 owner DOM 不渲染.
            // 当前 head 不需要回滚按钮 (回滚到自己).
            const showRollbackBtn = isOwner && !isHead && !editing;
            return (
              <li key={v.version} className={isHead ? 'artifact-version-row head' : 'artifact-version-row'}>
                <span className="artifact-version-label">{label}</span>
                <span className="artifact-version-kind" title={v.committer_kind}>
                  {kindBadge}
                </span>
                {showRollbackBtn && (
                  <button
                    className="btn btn-sm artifact-rollback-btn"
                    disabled={busy}
                    onClick={() => handleRollback(v.version)}
                  >
                    回滚到此版本
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      </aside>

      {/* CV-4.3 — iterate UI (#409 server / #405 schema).
          立场 ⑥ owner-only DOM omit (defense-in-depth, 跟 line ~441
          showRollbackBtn 同模式). non-markdown artifact 不渲染 — iterate
          UI 仅在 markdown kind 上 (CV-2 §4 反约束承袭, code/image_link
          iterate v0 走 spec brief #365 §2 协调待 CV-3 协同). */}
      {isOwner && artifact.type === 'markdown' && (
        <IteratePanel
          artifactId={artifact.id}
          channelId={channelId}
          isOwner={isOwner}
          onIterationCompleted={() => {
            // commit 走 CV-1 既有路径 — ArtifactUpdated frame 已触发 reload;
            // 此回调让 panel 跳到新版本视图 (current_version 已 reload 更新).
            void reload(artifact.id);
          }}
        />
      )}
    </div>
  );
}

/**
 * normalizeKind — 三 enum 收口 (markdown / code / image_link). 旧/未来
 * kind (v2+ 蓝图 §2 不做清单留账) 走 fallback path 在 ArtifactBody 里
 * 渲染 `<div class="artifact-kind-unsupported">` 兜底文案.
 *
 * 三 enum byte-identical 跟 cv-3-content-lock.md §1 ① +
 * cv_3_2_artifact_validation.go ArtifactKind* 同源.
 */
export function normalizeKind(raw: string | undefined): ArtifactKind | string {
  if (
    raw === 'markdown' ||
    raw === 'code' ||
    raw === 'image_link' ||
    raw === 'video_link' ||
    raw === 'pdf_link'
  ) {
    return raw;
  }
  return raw ?? 'markdown';
}

/**
 * ArtifactBody — kind switch 三分支 (CV-3.3 §2.1 acceptance).
 * Switch 顺序 markdown → code → image_link byte-identical 跟
 * content-lock §1 ① 同源.
 *
 * 反约束: 不渲染 raw HTML (XSS 红线 §2.8) — markdown 路径走
 * renderMarkdown() (marked + DOMPurify), 其它两 kind 走 React 节点.
 */
function ArtifactBody({ artifact }: { artifact: Artifact }) {
  const kind = normalizeKind(artifact.type);
  switch (kind) {
    case 'markdown':
      return (
        <div
          data-artifact-kind="markdown"
          className="artifact-rendered markdown-content"
          // 立场 ④ Markdown ONLY — renderMarkdown() 走 marked + DOMPurify,
          // 不接受 HTML 直插. 仅 markdown 分支保留 dangerouslySetInnerHTML.
          dangerouslySetInnerHTML={{ __html: renderMarkdown(artifact.body) }}
        />
      );
    case 'code':
      // language 在当前 PR 协议: server validation 已收 metadata.language
      // 但不持久化 (CV-3.2 留账); client 默认走 'text' fallback,
      // mention preview 路径有显式 language 时按值走.
      return (
        <div data-artifact-kind="code" className="artifact-rendered">
          <CodeRenderer body={artifact.body} />
        </div>
      );
    case 'image_link':
      // body = https URL (server ValidateImageLinkURL 已闸).
      // sub-kind 默认 image; v0 不暴露 link 切换 (留 metadata 持久化后).
      return (
        <div data-artifact-kind="image_link" className="artifact-rendered">
          <ImageLinkRenderer body={artifact.body} title={artifact.title} subKind="image" />
        </div>
      );
    case 'video_link':
      // CV-2 v2 立场 ② HTML5 native — preview_url 当 poster.
      return (
        <div data-artifact-kind="video_link" className="artifact-rendered">
          <MediaPreview
            kind="video_link"
            body={artifact.body}
            title={artifact.title}
            previewUrl={artifact.preview_url}
          />
        </div>
      );
    case 'pdf_link':
      // CV-2 v2 立场 ② <embed type="application/pdf"> 浏览器内嵌.
      return (
        <div data-artifact-kind="pdf_link" className="artifact-rendered">
          <MediaPreview kind="pdf_link" body={artifact.body} title={artifact.title} />
        </div>
      );
    default:
      // 立场 ⑦ — 兜底文案 (content-lock §1 ⑦ byte-identical).
      // 不 throw, 不 fallback markdown — 优雅降级展示原 kind 字串.
      return (
        <div className="artifact-kind-unsupported">
          此 artifact 类型 ({kind}) 暂不支持渲染
        </div>
      );
  }
}
