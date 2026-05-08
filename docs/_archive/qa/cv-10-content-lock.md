# CV-10 Content-Lock — DOM 锚 + 文案 byte-identical

> 战马E · 2026-04-29 · spec `cv-10-spec.md` 立场 ④ — UI 必锁 DOM + 文案 byte-identical.

## 1. DOM 锚 (反向 grep ≥1 hit)

| # | 锚 | 字面 | 用途 | 反向 grep |
|---|---|---|---|---|
| ① | textarea data-attr | `data-cv10-draft-textarea="<artifactId>"` | textarea 选择器 + e2e 验证 | `git grep -n 'data-cv10-draft-textarea' packages/client/src/` count≥1 |
| ② | restore toast data-attr | `data-cv10-restore-toast` | restore notification 选择器 | `git grep -n 'data-cv10-restore-toast' packages/client/src/` count≥1 |

## 2. 文案 byte-identical (反向 grep ≥1 hit)

| # | 文案 | 触发 | 反向 grep |
|---|---|---|---|
| ① | "已恢复未保存的草稿" | mount 时若 localStorage 有 draft 显 toast | `git grep -n '已恢复未保存的草稿' packages/client/src/` count≥1 |
| ② | "草稿已清除" | submit 成功后清 draft 显 toast (可选) | `git grep -n '草稿已清除' packages/client/src/` count≥1 |

## 3. localStorage key namespace (反向 grep ≥1)

| # | key | 锁 |
|---|---|---|
| ① | `borgee.cv10.comment-draft:<artifactId>` | namespace byte-identical, 跟 DM-4 既有 `borgee.dm.draft:` 同模式 |

反向 grep `borgee\.cv10\.comment-draft:` count≥1.

## 4. 反约束 (CI grep 0 hit)

| # | 反约束 | 反向 grep |
|---|---|---|
| ① | 不另起 confirm modal — 复用浏览器原生 beforeunload | `git grep -nE 'cv10.*confirm.*leave\|cv10.*custom.*modal' packages/client/src/` count==0 |
| ② | 不用 sessionStorage — 用 localStorage 跨 reload | `git grep -nE 'cv10.*sessionStorage' packages/client/src/` count==0 |
| ③ | 0 server-side draft 路径 | `git grep -nE 'comment_drafts.*PRIMARY\|/comments/.*/draft\|/artifacts/.*/draft' packages/server-go/internal/` count==0 |
| ④ | admin god-mode UI 不挂 | `git grep -nE 'admin.*ArtifactCommentDraftInput' packages/client/src/` count==0 |
