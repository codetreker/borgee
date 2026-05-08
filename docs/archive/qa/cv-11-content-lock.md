# CV-11 Content-Lock — DOM 锚 + sanitize 反向断

> spec `cv-11-spec.md` 立场 ④ — UI DOM 必锚 + sanitize 反向断.

## 1. DOM 锚 (反向 grep ≥1 hit)

| # | 锚 | 字面 | 用途 | 反向 grep |
|---|---|---|---|---|
| ① | comment body root | `data-cv11-comment-body` | rendered markdown 容器选择器 | `git grep -n 'data-cv11-comment-body' packages/client/src/` count≥1 |

## 2. sanitize 反向断 (vitest 必跑)

| # | 输入 | 期望 |
|---|---|---|
| ① | `<script>alert(1)</script>hello` | `<script>` 元素 0; "hello" 文本保留 |
| ② | `<iframe src="//evil"></iframe>` | `<iframe>` 元素 0 |
| ③ | `<img src=x onerror="alert(1)">` | `onerror` 属性删除 (DOMPurify 默认配置) |

## 3. 反约束 (CI grep 0 hit)

| # | 反约束 | 反向 grep |
|---|---|---|
| ① | 不引入新 markdown lib | `git grep -nE "from 'react-markdown'\|from 'remark\|from 'rehype\|from 'markdown-it'" packages/client/src/` count==0 |
| ② | 不直接注入 raw HTML | `git grep -nE 'cv11.*innerHTML.*body\|cv11.*raw.*html' packages/client/src/` count==0 |
| ③ | admin god-mode 不挂 | `git grep -nE 'admin.*ArtifactCommentBody\|admin.*comment.*markdown' packages/client/src/` count==0 |
| ④ | server 不解 markdown | server 不调 marked/DOMPurify (反向 grep `marked\|DOMPurify` 在 packages/server-go/internal/ count==0) |

## 4. 5-pattern thinking subject 错误码 byte-identical (跟 CV-5/7/8/9 同字符)

- `comment.thinking_subject_required` — **第 8 处链** (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 + CV-9 + CV-11)
