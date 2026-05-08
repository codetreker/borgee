# CV-12 Content-Lock — DOM 锚 + 文案 byte-identical

> spec `cv-12-spec.md` 立场 ④.

## 1. DOM 锚 (反向 grep ≥1 hit)

| # | 锚 | 字面 | 反向 grep |
|---|---|---|---|
| ① | search input | `data-cv12-search-input="<artifactId>"` | `git grep -n 'data-cv12-search-input' packages/client/src/` count≥1 |
| ② | result row | `data-cv12-search-result-id="<msgId>"` | `git grep -n 'data-cv12-search-result-id' packages/client/src/` count≥1 |

## 2. 文案 byte-identical (反向 grep ≥1)

| # | 文案 | 触发 |
|---|---|---|
| ① | "未找到匹配评论" | search 0 result 状态 |
| ② | "搜索评论..." | input placeholder (可选) |

## 3. 反约束 (CI grep 0 hit)

| # | 反约束 | 反向 grep |
|---|---|---|
| ① | 不另起 search endpoint | `git grep -nE '/comments/search\|comment_fts\|artifact_search.*PRIMARY' packages/server-go/internal/` count==0 |
| ② | 空 query 不调 API | vitest 反向断 (mock fetch 调用计数==0) |
| ③ | admin god-mode UI 不挂 | `git grep -nE 'admin.*ArtifactCommentSearchBox' packages/client/src/` count==0 |
| ④ | client 不预判 thinking | `git grep -nE 'cv12.*thinking\|cv12.*subject' packages/client/src/` count==0 |
