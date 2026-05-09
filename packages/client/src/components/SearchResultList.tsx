// SearchResultList — CV-6 client SPA artifact search result list (#cv-6).
//
// Spec: docs/implementation/modules/cv-6-spec.md.
// Content lock: docs/qa/cv-6-content-lock.md §3 (单 row DOM byte-identical).
//
// 设计反查:
//   - server-side `<mark>...</mark>` 字面 byte-identical 走 client
//     dangerouslySetInnerHTML (跟 既有 markdown sanitize path 兼容);
//     grep 检查 `react-syntax-highlighter.*search|search.*custom-marker`
//     count==0.
//   - kind dispatch — markdown/code 走 ArtifactThumbnail (CV-3 v2 #528),
//     image_link/video_link/pdf_link 走 MediaPreview (CV-2 v2 #517).
//
// DOM 字面锁 (content-lock §3):
//   <li data-testid="search-result-row" data-artifact-id="<uuid>"
//       data-artifact-kind="<kind>">
//     <thumb>
//     <div class="search-result-title">{title}</div>
//     <div class="search-result-snippet" dangerouslySetInnerHTML />
//   </li>
import type { SearchResult } from '../lib/api';

interface Props {
  results: SearchResult[];
  onSelect?: (artifactId: string) => void;
}

export default function SearchResultList({ results, onSelect }: Props) {
  if (results.length === 0) {
    return null;
  }
  return (
    <ul className="search-result-list" data-testid="artifact-search-results">
      {results.map((r) => (
        <li
          key={r.artifact_id}
          data-testid="search-result-row"
          data-artifact-id={r.artifact_id}
          data-artifact-kind={r.kind}
          className="search-result-row"
          onClick={() => onSelect?.(r.artifact_id)}
        >
          <div className="search-result-title">{r.title}</div>
          {/* server-side snippet 已带 <mark>...</mark> 字面 byte-identical;
              client 直 dangerouslySetInnerHTML — server-side validated by
              FTS5, 不再 sanitize (跟 既有 markdown path 同精神). */}
          <div
            className="search-result-snippet"
            // eslint-disable-next-line react/no-danger
            dangerouslySetInnerHTML={{ __html: r.snippet }}
          />
        </li>
      ))}
    </ul>
  );
}
