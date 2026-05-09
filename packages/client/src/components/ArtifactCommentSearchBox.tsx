// ArtifactCommentSearchBox — CV-12.2 client: search input + result list for
// artifact-comment search. Reuses existing message-search endpoint via
// searchArtifactComments wrapper (lib/api.ts CV-12 block).
//
// Spec: docs/implementation/modules/cv-12-spec.md §1 CV-12.2.
// Stance: docs/qa/cv-12-stance-checklist.md §4.
// Content-lock: docs/qa/cv-12-content-lock.md §1+§2.
//
// 设计反查:
//   - ① 复用既有 search endpoint 单源 (0 server code).
//   - ④ DOM `data-cv12-search-input` + `data-cv12-search-result-id` 锚 +
//     文案 "未找到匹配评论" byte-identical; 空 query 不调 API.

import { useCallback, useState } from 'react';
import {
  ApiError,
  searchArtifactComments,
  type ArtifactCommentSearchHit,
} from '../lib/api';

const NO_RESULT_TEXT = '未找到匹配评论';

interface ArtifactCommentSearchBoxProps {
  artifactId: string;
  /** UUID of the virtual `artifact:<artifactId>` namespace channel.
   *  Resolved upstream (e.g. by ArtifactComments parent component). */
  artifactChannelId: string;
}

export default function ArtifactCommentSearchBox({
  artifactId,
  artifactChannelId,
}: ArtifactCommentSearchBoxProps) {
  const [query, setQuery] = useState('');
  const [hits, setHits] = useState<ArtifactCommentSearchHit[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onSubmit = useCallback(async () => {
    const q = query.trim();
    // 设计 ④ 空 query 不调 API.
    if (q === '') {
      setHits(null);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const out = await searchArtifactComments(artifactChannelId, q);
      setHits(out.messages ?? []);
    } catch (err) {
      setHits([]);
      if (err instanceof ApiError) {
        setError(err.message || 'search failed');
      } else {
        setError('search failed');
      }
    } finally {
      setBusy(false);
    }
  }, [artifactChannelId, query]);

  return (
    <div className="cv12-comment-search-box">
      <input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') void onSubmit();
        }}
        placeholder="搜索评论..."
        disabled={busy}
        data-cv12-search-input={artifactId}
      />
      <button
        type="button"
        onClick={() => void onSubmit()}
        disabled={busy || query.trim() === ''}
        data-testid="cv12-search-submit"
      >
        搜索
      </button>
      {error && (
        <span className="cv12-search-error" role="alert" data-testid="cv12-search-error">
          {error}
        </span>
      )}
      {hits !== null && hits.length === 0 && !error && (
        <div className="cv12-search-no-result" data-testid="cv12-no-result">
          {NO_RESULT_TEXT}
        </div>
      )}
      {hits !== null && hits.length > 0 && (
        <ul className="cv12-search-results" data-testid="cv12-search-results">
          {hits.map((h) => (
            <li
              key={h.id}
              className="cv12-search-result-row"
              data-cv12-search-result-id={h.id}
            >
              <span className="cv12-result-sender">{h.sender_id}</span>
              <span className="cv12-result-body">{h.content}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
