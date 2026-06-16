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
//     <div class="search-result-snippet">{walk-rendered snippet}</div>
//   </li>
//
// XSS (#1030): server snippet() (search.go:148) 把 RAW 用户 artifact body
// 用 *字面* '<mark>'/'</mark>' 包起来 — FTS5 只 tokenize/highlight, 从不
// HTML-escape. 所以 attacker body 里的 `<img onerror>` / `<script>` 会原样
// 流进 snippet. 这里用 walk-render 在 server 的字面 <mark> 分隔符上切段,
// 高亮段渲染成真 <mark> 元素 (inner 作 React text child, 自动转义), 其余段
// 作纯字符串 (自动转义) — 全程不用 innerHTML, 任何 attacker markup 都变成
// 惰性转义文本, 不执行. server snippet contract 保持 byte-identical 不动.
import type React from 'react';
import type { SearchResult } from '../lib/api';

interface Props {
  results: SearchResult[];
  onSelect?: (artifactId: string) => void;
}

// renderSnippet — server snippet() 用字面 '<mark>'/'</mark>' (search.go:148)
// 标高亮. 在这对固定字面分隔符上切段后, 高亮段渲染成真 <mark>{text}</mark>
// (text 作 child, React 自动转义), 其余作纯字符串 (React 自动转义). 全程
// 不经 innerHTML — 即便 attacker 的 STORED 内容里自己带字面 '<mark>'/
// '</mark>' 字串, 最坏只是多一处误高亮, 绝无 HTML 执行.
function renderSnippet(snippet: string): React.ReactNode[] {
  return snippet.split(/(<mark>[\s\S]*?<\/mark>)/).map((seg, i) => {
    const m = /^<mark>([\s\S]*)<\/mark>$/.exec(seg);
    if (m) {
      return <mark key={i}>{m[1]}</mark>;
    }
    return seg;
  });
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
          {/* server snippet 的字面 <mark>...</mark> 高亮走 walk-render —
              不经 innerHTML, attacker markup 全转义为惰性文本 (#1030). */}
          <div className="search-result-snippet">{renderSnippet(r.snippet)}</div>
        </li>
      ))}
    </ul>
  );
}
