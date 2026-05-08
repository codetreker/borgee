# CV-11 立场反查清单 (战马E v0)

> 战马E · 2026-04-29 · 跟 CV-9/CV-10 client-only + DM-2.3 markdown 同模式承袭 + thinking 5-pattern 第 8 处链.
> **关联**: spec `cv-11-spec.md` (8f834c9) + acceptance + content-lock.

## §0 立场总表 (4 立场 + 3 边界)

| # | 立场 | 反约束 (代码层守门) |
|---|---|---|
| ① | markdown 渲染走 `lib/markdown::renderMarkdown` 单源 (marked + DOMPurify), **0 server code + 0 新 lib** | 反向 grep `from 'react-markdown'\|from 'remark\|from 'rehype\|from 'markdown-it'` 在 client/src/ count==0 |
| ② | owner-only ACL byte-identical 13+ 处, admin god-mode 不挂 | 反向 grep `admin.*ArtifactCommentBody\|admin.*comment.*markdown` count==0 |
| ③ | thinking 5-pattern 第 8 处链 byte-identical (server gate, client 不预判) | 反向 grep 5-pattern 字面在 internal/api/ 排除 _test.go count==0; 改 = 改 8 处 |
| ④ | DOM `data-cv11-comment-body` 锚 + sanitize 反向断 (XSS 0 hit) | 反向 grep `data-cv11-comment-body` ≥1; vitest 验 `<script>` 输入后 DOM 0 script element |

## §1 立场 ① renderMarkdown 单源

- [ ] ArtifactCommentBody.tsx 仅 `import { renderMarkdown } from '../lib/markdown'` (反向断不引入新 lib)
- [ ] 0 server code — git diff packages/server-go/ 0 production 行
- [ ] 反向 grep 4 lib literal 0 hit

## §2 立场 ② owner-only / admin 不挂

- [ ] server-side ACL 既有 messages.go 不动
- [ ] 反向 grep `admin.*ArtifactCommentBody` 0 hit

## §3 立场 ③ thinking 5-pattern 第 8 处链

- [ ] CV-7/CV-8 既有 hook 覆盖 — markdown body 含 `**thinking**` 等 syntax 不豁免
- [ ] client 不预判 thinking (反向断)
- [ ] 5-pattern 改 = 改 8 处 byte-identical

## §4 立场 ④ DOM 锚 + sanitize

- [ ] ArtifactCommentBody root 渲染 `data-cv11-comment-body` (反向 grep ≥1)
- [ ] dangerouslySetInnerHTML 仅在 DOMPurify 后注入 (合法路径, vitest 验 sanitize 真跑)
- [ ] vitest 反向断 `<script>alert(1)</script>` 输入后 container.querySelector('script') == null
- [ ] 反向 grep `cv11.*innerHTML.*body\|cv11.*raw.*html` 0 hit

## §5 边界 ⑤⑥⑦ — fail-closed / forward-only / 不裂表

- [ ] cross-channel reject 跟 CV-5..CV-9 同源
- [ ] forward-only — markdown 渲染纯 read-side
- [ ] 不裂表 — 0 schema 改

## §6 退出条件

- §1+§2+§3+§4 全 ✅ + 反向 grep 5 锚: 4 处 0 hit + DOM ≥1 hit
- vitest 4 case + e2e 3 case + 1 server unit 全 PASS
- 0 schema 改 + 0 server production code
