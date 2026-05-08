# CV-15 Content Lock — artifact comment edit history 文案 + DOM byte-identical 锁 (野马 v0)

> 战马C · 2026-04-30 · CV-15 artifact comment edit history audit 文案 + DOM 字面锁
> **关联**: spec `cv-15-spec.md` v0 + acceptance `cv-15.md` + stance `cv-15-stance-checklist.md`. 跟 DM-7 #558 EditHistoryModal 同源 (artifact comment scoped variant).

## §1 ArtifactCommentEditHistoryModal 文案锁 (3 字面 byte-identical 跟 DM-7 同源)

字面 (改 = 改三处: client component + 此 content-lock + 测试文件):

```
comment_edit_history.title       → "编辑历史"
comment_edit_history.empty       → "暂无编辑记录"
comment_edit_history.count       → "共 N 次编辑" (N 替换为实际数字)
```

**字面同源锚**: `编辑历史` + `共 N 次编辑` 文案跟 `packages/client/src/components/EditHistoryModal.tsx` (DM-7 #558) byte-identical (改 DM-7 → 同步改 CV-15, byte-identical 跨两组件防漂); `暂无编辑记录` 跟 DM-7 同精神 (DM-7 用类似 `加载编辑历史失败` reject 文案, CV-15 用 empty state 文案).

**反向 grep** (count==0): `changes|revisions|revs|版本|修订|变更|回退` 在 `packages/client/src/components/ArtifactCommentEditHistoryModal.tsx` user-visible 文案 0 hit (跟 DM-7 同精神).

## §2 ArtifactCommentEditHistoryModal DOM 字面锁

容器 + entry byte-identical (改 = 改两处: 此 content-lock +
`packages/client/src/components/ArtifactCommentEditHistoryModal.tsx`):

```html
<div
  class="comment-edit-history-modal"
  data-testid="comment-edit-history-modal"
  role="dialog"
  aria-label="编辑历史"
>
  <h2 class="comment-edit-history-title">编辑历史</h2>
  <p class="comment-edit-history-count">共 N 次编辑</p>

  {history.length === 0 ? (
    <p class="comment-edit-history-empty">暂无编辑记录</p>
  ) : (
    <ul class="comment-edit-history-list">
      <li
        data-testid="comment-edit-history-entry"
        data-ts="<RFC3339>"
        class="comment-edit-history-entry"
      >
        <time datetime="<RFC3339>">{formatted_ts}</time>
        <pre class="comment-edit-history-old-content">{old_content}</pre>
      </li>
    </ul>
  )}
</div>
```

`data-ts` 是 RFC3339 ts byte-identical 跟 server-side `time.Format("2006-01-02 15:04")` 同精神跟 DM-7 同源.

## §3 错码字面单源 (server const ↔ client toast 双向锁)

`internal/api/cv_15_comment_edit_history.go::CommentEditHistoryErrCode*` const +
`packages/client/src/lib/api.ts::COMMENT_EDIT_HISTORY_ERR_TOAST` map 双向锁
(跟 DM-7 / DM-8 / CHN-15 / AL-9 / CV-6 同模式):

```ts
export const COMMENT_EDIT_HISTORY_ERR_TOAST: Record<string, string> = {
  'comment.not_artifact_comment':  '该消息不是 artifact 评论',
  'comment.not_owner':             '仅评论作者可查看历史',
  'comment.message_not_found':     '消息不存在',
};
```

**改 = 改三处** (server const + client map + 此 content-lock).

## §4 跨 PR drift 守

改 3 文案 / 3 错码 / DOM data-* attrs = 改五处:
1. `internal/api/cv_15_comment_edit_history.go::CommentEditHistoryErrCode*` const (3 字面)
2. `packages/client/src/lib/api.ts::COMMENT_EDIT_HISTORY_ERR_TOAST` map (3 字面)
3. `packages/client/src/lib/comment_edit_history.ts::COMMENT_EDIT_HISTORY_LABEL` 3 文案 const
4. `packages/client/src/components/ArtifactCommentEditHistoryModal.tsx` (DOM data-* + 文案使用)
5. 此 content-lock §1+§2+§3

## §5 admin god-mode 红线 (ADM-0 §1.3 同源)

CV-15 admin-rail GET only — admin god-mode 看 history 但**不能改**.
反向 grep `mux.Handle("(POST|DELETE|PATCH|PUT).*admin-api/v[0-9]+/.*comment-edit-history` 在 internal/api+server/ count==0. 跟 DM-7 admin readonly + DM-8 admin 不挂 + CHN-15 admin 不挂同精神 (admin god-mode read-only + 不入业务路径).

## 更新日志

- 2026-04-30 — 战马C v0 (4 件套第四件 ≤30 行): ArtifactCommentEditHistoryModal 3 文案 byte-identical 跟 DM-7 EditHistoryModal §1 同源 + DOM data-* + 3 错码 toast 双向锁; 反同义词 7 字面禁; 跟 DM-7 / DM-8 / CHN-15 / AL-9 / CV-6 同 4 件套模式.
