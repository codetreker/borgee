# CV-11 spec brief — artifact comment markdown 渲染 (CV-5..CV-10 续, client only)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) L24 字面 "Linear issue + comment" + CV-5..CV-10 单源延伸 + CV-9 #539 / CV-10 #541 client-only 同模式 + thinking 5-pattern 第 8 处链. CV-11 让 artifact comment body 渲染 markdown — 复用既有 `lib/markdown::renderMarkdown` (marked + DOMPurify), **0 server production code + 0 schema 改 + 0 新 endpoint + 0 新 lib**.

## 0. 关键约束 (4 项立场, 跨链承袭)

1. **markdown 渲染走 `lib/markdown::renderMarkdown` 单源, 0 新 lib + 0 server code** (跟 CV-9 / CV-10 client-only 同模式 + DM-2.3 mention 渲染 + RT-1.3 既有 ArtifactPanel.tsx markdown 渲染同源): comment body 通过 既有 `renderMarkdown(body, mentions, userMap)` 走 marked → DOMPurify (反向 dangerouslySetInnerHTML 直接 0 hit — 必经 DOMPurify 兜底). **反约束**: 不引入 react-markdown / remark / rehype 任何新 lib (反向 grep `from 'react-markdown'\|from 'remark\|from 'rehype` 0 hit on cv-11 component); 不另写 sanitize 路径 (复用既有 lib/markdown.ts).

2. **owner-only ACL byte-identical 13+ 处一致, admin god-mode 不挂** (CV-5..CV-10 同源): comment body 仅 channel-member 看 (server-side ACL 既有 messages.go 覆盖); admin god-mode 不渲染 comment body (跟 ADM-0 §1.3 同源). **反向 grep**: `admin.*ArtifactCommentBody\|admin.*comment.*markdown` 在 client/src/ count==0.

3. **thinking 5-pattern 第 8 处链 byte-identical** (CV-5/7/8/9 延伸): markdown 不豁免 thinking validate — 即使 body 含 markdown syntax (e.g. `**thinking**` bold), agent post 时仍走 server CV-7/CV-8 既有 hook 5-pattern reject. **client-side 不另判 thinking** — 是 server 层 gate (反向断 client 不预判). **反向 grep**: 5-pattern 字面在 internal/api/ 排除 _test.go count==0; 5-pattern 改 = 改 8 处 byte-identical (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7 + CV-8 + CV-9 + CV-11).

4. **DOM 锁 + sanitize 反向断** (content-lock): rendered output `data-cv11-comment-body` 锚 + 子节点限制白名单 (`p`, `pre`, `code`, `blockquote`, `h1-h6`, `ul/ol/li`, `strong`, `em`, `a`, `br` — DOMPurify 默认白名单复用); 反向断 `<script>` / `<iframe>` / `onerror=` 即使 body 含也被 sanitize 删. **反向 grep**: `data-cv11-comment-body` ≥1 hit; `dangerouslySetInnerHTML` 在 ArtifactCommentBody.tsx ≥1 hit (因为 marked 输出走 DOMPurify 后必须用 dSIH 注入, 这是合法路径), 但 cv-11 必须**只在** sanitize 后 dSIH (反向断 unsafe path 0 hit — vitest 验证 `<script>alert</script>` 输入后 DOM 0 script element).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-11.1 server | (无 server 实施) + `internal/api/cv_11_no_markdown_test.go` 1 unit 反向断 server 不解 markdown (server response body 是 raw markdown source, 不渲染 HTML — 跟 CV-1.2 既有 artifact body 同模式) | 1 unit PASS; 0 行 production code |
| CV-11.2 client | `packages/client/src/components/ArtifactCommentBody.tsx` (新, ≤30 行 — 调 `renderMarkdown` + dSIH + data-cv11-comment-body 锚) + content-lock | 复用 `lib/markdown::renderMarkdown` 既有; 4 vitest case (基本 markdown / sanitize XSS / mention 渲染 / 空 body fallback) |
| CV-11.3 e2e + closure | `packages/e2e/tests/cv-11-comment-markdown.spec.ts` (3 case, REST-driven + browser DOM 验证) + REG-CV11-001..005 + acceptance + PROGRESS [x] | client-only e2e — type markdown body → POST → reload page → DOM 渲染验证 |

## 2. 错误码 (0 新 — client-only, 沿用 CV-5..CV-10 既有)

CV-11 是纯 client-side 渲染, 0 server response 改, 0 错误码新增.

## 3. 反向 grep 锚 (CV-11 实施 PR 必跑)

```
git grep -nE "from 'react-markdown'|from 'remark|from 'rehype|from 'markdown-it" packages/client/src/  # 0 hit (单源 marked, 反约束新 lib)
git grep -nE 'admin.*ArtifactCommentBody|admin.*comment.*markdown' packages/client/src/  # 0 hit (ADM-0 §1.3)
git grep -nE 'data-cv11-comment-body' packages/client/src/  # ≥ 1 hit (DOM 锚)
git grep -nE 'dangerouslySetInnerHTML' packages/client/src/components/ArtifactCommentBody.tsx  # ≥ 1 hit (合法 DOMPurify 后注入路径, 反向断 unsafe 路径不存在)
git grep -nE 'cv11.*innerHTML.*body|cv11.*raw.*html' packages/client/src/  # 0 hit (不直接注入 raw HTML, 必经 sanitize)
```

## 4. 不在本轮范围 (deferred)

- ❌ markdown editor (CV-10 textarea 既有, plain text 输入)
- ❌ syntax highlighting (CV-3 #408 既有 code renderer 复用; CV-11 仅基本 markdown)
- ❌ 表格 / 数学 / 自定义 plugin (留 v2)
- ❌ admin god-mode 看 markdown 渲染 (ADM-0 §1.3 红线)
- ❌ schema migration (0 schema 改, body 仍 raw text 落 messages.content)
- ❌ server-side markdown 渲染 (CV-1.2 立场 body 是 raw, client 渲染)
