// ArtifactComments — CV-5.2 client SPA: artifact-level comment list + composer.
//
// Blueprint: docs/blueprint/current/canvas-vision.md §0 L24 wording:
// "Linear issue + comment, not a Miro whiteboard". Spec: docs/implementation/modules/cv-5-spec.md
// §1 CV-5.2 (client). Stance: docs/qa/cv-5-stance-checklist.md.
//
// Design checks:
//   - ① comments use messages table as the single source. Do not introduce an
//     artifact_comments type; call postArtifactComment + listArtifactComments
//     (server writes messages table + virtual `artifact:<id>` channel).
//   - ② frame signal + incremental append. useArtifactCommentAdded listens to
//     the WS frame; when it matches the current artifact, listArtifactComments
//     fetches the latest state, matching AnchorThreadPanel. Do not render
//     comment text from frame.body_preview because the server caps it at 80 runes
//     for privacy §13.
//   - ③ agent thinking subject is rejected by the server. The client only shows
//     the error code, visible to both human reviewers and agents, without split rendering.
//
// Required constraints:
//   - Do not mount an admin-only view (ADM-0 §1.3 constraint).
//   - Keep hover anchor `data-cv5-author-link`; it is the UI-element anchor shared
//     with CM-5.3 transparent-collaboration hover.

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

  // Design ② WS frame signal: refetch when frame matches current artifact.
  // Do not render from frame.body_preview because the server caps it at 80 runes.
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
