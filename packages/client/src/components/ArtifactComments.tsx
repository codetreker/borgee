// ArtifactComments — CV-5.2 client SPA: artifact-level comment list + composer.
//
// Blueprint: docs/blueprint/current/canvas-vision.md §0 L24 字面 "Linear issue +
// comment, 不是 Miro 白板". Spec: docs/implementation/modules/cv-5-spec.md
// §1 CV-5.2 (client). Stance: docs/qa/cv-5-stance-checklist.md.
//
// 设计反查:
//   - ① comment 走 messages 表单源 — 不写 artifact_comments 类型, 调 postArtifactComment +
//     listArtifactComments (服务端落 messages 表 + virtual `artifact:<id>` channel).
//   - ② frame 信号 + 增量 append — useArtifactCommentAdded 监听 WS frame, 命中
//     当前 artifact 时调 listArtifactComments 拉最新 (跟 AnchorThreadPanel 同模式),
//     反约束: 不用 frame.body_preview 渲染 comment text (server 80-rune cap, 隐私 §13).
//   - ③ agent thinking subject — 服务端 reject; client 仅显错码 (人审 + agent 都可看, 不裂渲染).
//
// 反约束:
//   - 不挂 admin-only 视图 (ADM-0 §1.3 constraint)
//   - hover anchor `data-cv5-author-link` (跟 CM-5.3 透明协作 hover 同源 — UI 元素锚)

import { useCallback, useEffect, useState } from 'react';
import {
  ApiError,
  postArtifactComment,
  listArtifactComments,
  type ArtifactComment,
} from '../lib/api';
import { useArtifactCommentAdded } from '../hooks/useWsHubFrames';

interface ArtifactCommentsProps {
  artifactId: string;
}

export default function ArtifactComments({ artifactId }: ArtifactCommentsProps) {
  const [comments, setComments] = useState<ArtifactComment[]>([]);
  const [body, setBody] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const refetch = useCallback(async () => {
    try {
      const out = await listArtifactComments(artifactId);
      setComments(out.comments ?? []);
    } catch {
      // silent — list path is best-effort; WS push will retry on next frame.
    }
  }, [artifactId]);

  useEffect(() => {
    void refetch();
  }, [refetch]);

  // 设计 ② WS frame signal — refetch when frame matches current artifact.
  // 反约束: 不用 frame.body_preview 作渲染源 — 服务端 80-rune cap.
  useArtifactCommentAdded(
    useCallback(
      (frame) => {
        if (frame.artifact_id === artifactId) {
          void refetch();
        }
      },
      [artifactId, refetch],
    ),
  );

  const submit = useCallback(async () => {
    const trimmed = body.trim();
    if (!trimmed) return;
    setSubmitting(true);
    setErrorMessage(null);
    try {
      await postArtifactComment(artifactId, trimmed);
      setBody('');
      await refetch();
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorMessage(err.message || 'failed');
      } else if (err instanceof Error) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage('failed');
      }
    } finally {
      setSubmitting(false);
    }
  }, [artifactId, body, refetch]);

  return (
    <div className="cv5-artifact-comments" data-testid="cv5-artifact-comments">
      <div className="cv5-artifact-comments-list">
        {comments.length === 0 ? (
          <div className="cv5-artifact-comments-empty" data-testid="cv5-empty">
            No comments yet.
          </div>
        ) : (
          comments.map((c) => (
            <div
              key={c.id}
              className="cv5-artifact-comment-row"
              data-cv5-comment-id={c.id}
            >
              <span
                className="cv5-artifact-comment-author"
                data-cv5-author-link
                data-cv5-author-role={c.sender_role}
              >
                {c.sender_role === 'agent' ? '🤖' : '👤'} {c.sender_id}
              </span>
              <span className="cv5-artifact-comment-body">{c.body}</span>
              <span className="cv5-artifact-comment-time">
                {new Date(c.created_at).toLocaleString()}
              </span>
            </div>
          ))
        )}
      </div>
      <div className="cv5-artifact-comment-composer">
        <textarea
          aria-label="Add a comment"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={2}
          disabled={submitting}
          data-testid="cv5-composer-input"
        />
        <button
          type="button"
          onClick={() => void submit()}
          disabled={submitting || body.trim() === ''}
          data-testid="cv5-composer-submit"
        >
          {submitting ? 'Posting...' : 'Comment'}
        </button>
        {errorMessage && (
          <div className="cv5-artifact-comment-error" data-testid="cv5-error">
            {errorMessage}
          </div>
        )}
      </div>
    </div>
  );
}
