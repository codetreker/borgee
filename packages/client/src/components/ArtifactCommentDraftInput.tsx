// ArtifactCommentDraftInput — CV-10.1 client component: textarea with
// unsaved-draft persistence + restore toast + page-leave warning.
//
// Spec: docs/implementation/modules/cv-10-spec.md §1 CV-10.1.
// Stance: docs/qa/cv-10-stance-checklist.md §4.
// Content-lock: docs/qa/cv-10-content-lock.md §1 + §2.
//
// 设计反查:
//   - ④ DOM `data-cv10-draft-textarea="<artifactId>"` + `data-cv10-restore-toast`
//     必锚; 文案 "已恢复未保存的草稿" + "草稿已清除" byte-identical.
//   - ② page-leave warning 走浏览器原生 beforeunload (反约束 不挂自定义 modal).

import { useCallback, useEffect, useState } from 'react';
import { useArtifactCommentDraft } from '../hooks/useArtifactCommentDraft';

const RESTORE_TEXT = '已恢复未保存的草稿';
const CLEARED_TEXT = '草稿已清除';

interface ArtifactCommentDraftInputProps {
  artifactId: string;
  onSubmit: (body: string) => Promise<void> | void;
}

export default function ArtifactCommentDraftInput({
  artifactId,
  onSubmit,
}: ArtifactCommentDraftInputProps) {
  const { draft, setDraft, clear, restored } = useArtifactCommentDraft(artifactId);
  const [showRestoreToast, setShowRestoreToast] = useState(restored);
  const [showClearedToast, setShowClearedToast] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  // 设计 ② page-leave 警告 — 走浏览器原生 beforeunload, 不挂自定义 modal.
  useEffect(() => {
    if (draft.trim() === '') return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      // Modern browsers ignore the message; setting returnValue triggers
      // the native prompt. Spec lock: do NOT render a custom modal here
      // (反约束 grep `cv10.*confirm.*leave` 0 hit).
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [draft]);

  const handleSubmit = useCallback(async () => {
    const body = draft.trim();
    if (!body) return;
    setSubmitting(true);
    try {
      await onSubmit(body);
      clear();
      setShowRestoreToast(false);
      setShowClearedToast(true);
      // Auto-hide cleared toast after a short window.
      setTimeout(() => setShowClearedToast(false), 1500);
    } finally {
      setSubmitting(false);
    }
  }, [draft, onSubmit, clear]);

  return (
    <div className="cv10-comment-draft-input">
      {showRestoreToast && (
        <div
          className="cv10-restore-toast"
          data-cv10-restore-toast
          role="status"
          aria-live="polite"
        >
          {RESTORE_TEXT}
        </div>
      )}
      <textarea
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        disabled={submitting}
        rows={3}
        placeholder="写下你的评论..."
        data-cv10-draft-textarea={artifactId}
      />
      <button
        type="button"
        onClick={() => void handleSubmit()}
        disabled={submitting || draft.trim() === ''}
        data-testid="cv10-submit"
      >
        {submitting ? '提交中...' : '发送'}
      </button>
      {showClearedToast && (
        <span className="cv10-cleared-toast" role="status" aria-live="polite">
          {CLEARED_TEXT}
        </span>
      )}
    </div>
  );
}
