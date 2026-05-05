# CV-10 spec brief — artifact comment 草稿持久化 + restore (CV-5..CV-9 续, client only)

> 战马E · Phase 5+ · ≤80 行 · 蓝图 [`canvas-vision.md`](../../blueprint/current/canvas-vision.md) L24 字面 "Linear issue + comment" + CV-5..CV-9 单源延伸 + CV-9 #539 同模式 (client-only, 0 server code). CV-10 让 artifact comment 输入框 unsaved-state 跨 reload 不丢 — 纯 localStorage, **0 server production code + 0 schema 改 + 0 新 endpoint**.

## 0. 关键约束 (4 项立场, 蓝图字面 + 跨链)

1. **草稿走 localStorage 单源, 0 server side state** (CV-9 #539 client-only 同模式延伸; 反约束 跨 user / 跨 device 不同步 — 草稿是本地, 提交才上 server): localStorage key 命名 `borgee.cv10.comment-draft:<artifactId>` (跟 DM-4 既有 `borgee.dm.draft:` 同模式承袭, 反向 grep 该字面 0 hit 但 spec 锁第二处). **反约束**: 不开 `comment_drafts` 表 / 不开 `/api/v1/artifacts/:id/draft` endpoint / 不写 `sessionStorage` (用 localStorage 跨 reload). 反向 grep `comment_drafts.*PRIMARY|/comments/.*/draft|/artifacts/.*/draft` 在 internal/ count==0.

2. **submit 后清 + page leave 警告** (UX 不变量): 提交成功后 localStorage key 删除 (反向 vitest 断 `localStorage.getItem` returns null after submit); 用户离开 page (beforeunload) 且 draft 非空时浏览器原生 warning (反约束: 不挂自定义 modal — 复用 `event.preventDefault() + returnValue=''` 浏览器内置). **反向 grep**: 不另写 confirm modal `cv10.*confirm.*leave` count==0.

3. **owner-only ACL byte-identical 跟 CV-5..CV-9 同源** (草稿仅本地, 用户切换不漏): localStorage 是 per-browser-profile, 无跨 user 隐私问题; 但 logout 必清 (反约束: 跨 session 漏写). **反向 grep**: logout handler 复用既有 cleanup 路径 (反向断: `cv10.*localStorage.*remove\|cv10.*draft.*clear` 在 logout path ≥1 hit, 不挂在 user.id 但全清 cv10 keys).

4. **client UI: textarea data-attr + restore toast 文案 byte-identical** (content-lock): textarea 渲染 `data-cv10-draft-textarea="<artifactId>"` (反向 grep ≥1); restore 时 toast 字面 "已恢复未保存的草稿" byte-identical (跟 DM-4 既有 unsaved 提示文案承袭若有, 否则新锁); cleared toast "草稿已清除" 文案 byte-identical. **反向 grep**: `data-cv10-draft-textarea\|data-cv10-restore-toast` ≥2 hit; `已恢复未保存的草稿` ≥1 hit.

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| CV-10.1 client | `packages/client/src/hooks/useArtifactCommentDraft.ts` (新) + `packages/client/src/components/ArtifactCommentDraftInput.tsx` (新) + content-lock | hook: load on mount + save on change (debounced 500ms) + clear method; component: textarea + restore toast 文案 byte-identical + onSubmit clears draft + beforeunload 警告 (draft 非空时); 复用既有 toast/notification UI 不另起. **0 server code** (CV-9 同模式) |
| CV-10.2 vitest + e2e | `packages/client/src/__tests__/useArtifactCommentDraft.test.ts` (4 case) + `packages/e2e/tests/cv-10-comment-draft.spec.ts` (3 case) | vitest: load empty / save → load 来回 / clear after submit / debounce 防抖 4 case; e2e: type → reload → restore / submit clears draft / leave with draft fires beforeunload (3 case) |
| CV-10.3 closure | REG-CV10-001..005 + acceptance + PROGRESS [x] | 0 schema 改 + 0 server code 反向 grep + DOM/文案 ≥1 hit |

## 2. 错误码 (0 新 — client-only, 无 server response)

CV-10 是纯 client-side feature, 0 server response, 0 错误码. 任何 server submit 失败走 CV-5..CV-9 既有 errcode (复用 byte-identical).

## 3. 反向 grep 锚 (CV-10 实施 PR 必跑)

```
git grep -nE 'comment_drafts.*PRIMARY|/comments/.*/draft|/artifacts/.*/draft' packages/server-go/internal/  # 0 hit (server-side draft 0)
git grep -nE 'cv10.*confirm.*leave|cv10.*custom.*modal' packages/client/src/  # 0 hit (复用浏览器原生 beforeunload)
git grep -nE 'cv10.*sessionStorage' packages/client/src/  # 0 hit (用 localStorage 跨 reload)
git grep -nE 'data-cv10-draft-textarea|data-cv10-restore-toast' packages/client/src/  # ≥ 2 hit (DOM 锚)
git grep -nE '已恢复未保存的草稿|草稿已清除' packages/client/src/  # ≥ 2 hit (文案 byte-identical)
git grep -nE 'borgee\.cv10\.comment-draft:' packages/client/src/  # ≥ 1 hit (key namespace 锁)
```

## 4. 不在本轮范围 (deferred)

- ❌ server-side draft 表 (跨 device sync, Phase 7+ Pro feature)
- ❌ draft expiry / GC (浏览器 localStorage 默认无限期, 不另起 TTL)
- ❌ admin god-mode 看 draft (隐私 §13 + ADM-0 §1.3 红线 — 草稿是本地, server 不持有)
- ❌ collaborative draft (CV-1 立场 ② 单文档锁 30s 不开 CRDT, draft 同精神)
- ❌ schema migration (0 schema 改, localStorage only)
