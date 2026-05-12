// DiffView — CV-4.3 client jsdiff 行级蓝绿配色 diff view.
//
// Spec: docs/implementation/modules/cv-4-spec.md §0 设计 ③
//   (client jsdiff 不裂 server diff). 文案锁:
//   docs/qa/cv-4-content-lock.md §1 ⑤. Acceptance: cv-4.md §3.5.
// Checklist: docs/qa/cv-4-stance-checklist.md §1 ③.
//
// 设计反查:
//   - ③ 走 client jsdiff 行级 (jsdiff diffLines), 不裂 schema 不裂 endpoint.
//     Constraint: server 不算 diff (CRDT risk, blueprint §2 forbids it); 不存
//     diff 缓存 (查时即算 ≤500ms 实测够 markdown 数 KB).
//
// DOM 字面锁 (content-lock §1 ⑤):
//   - tab required text is a single character; synonyms must not appear.
//   - 标题 "v{N} ↔ v{M}" (双向箭头 ↔ 锁).
//   - 行级 DOM data-diff-line="add" / data-diff-line="del" /
//     data-diff-line="context" 三 enum 字面 (a11y ARIA 替代仅颜色辨识 —
//     视觉障碍漏防御).
//   - deep-link `?diff=vN..vM` exact format.
//
// Rules (component grep targets):
//   - 不接 tab 文案的全名词扩展形式 (单字锁)
//   - 不接 server diff endpoint 调用 (跟 spec §0 设计 ③ 同源)
//   - image_link kind 走 fallback 前后缩略图并排 (jsdiff 不适用)
import { useMemo } from 'react';
import { diffLines } from 'diff';

interface Props {
  /** 较新版本 (左). */
  newBody: string;
  newVersion: number;
  /** 较旧版本 (右). */
  oldBody: string;
  oldVersion: number;
  /** kind — 'image_link' 走前后缩略图 fallback (jsdiff 不适用). */
  kind?: 'markdown' | 'code' | 'image_link';
}

export type DiffLineKind = 'add' | 'del' | 'context';

export interface DiffRow {
  kind: DiffLineKind;
  text: string;
}

/**
 * computeDiffRows — 纯函数, 给 vitest 直接锁. jsdiff diffLines 返回
 * `{value, added, removed}` part 数组, 这里展开成行级 row 数组保留
 * 顺序 (跟 unified diff 视觉同序).
 */
export function computeDiffRows(oldBody: string, newBody: string): DiffRow[] {
  const parts = diffLines(oldBody, newBody);
  const rows: DiffRow[] = [];
  for (const part of parts) {
    const kind: DiffLineKind = part.added ? 'add' : part.removed ? 'del' : 'context';
    // diffLines value 含 trailing newline; split 保留 visible row.
    const lines = part.value.split('\n');
    if (lines.length > 0 && lines[lines.length - 1] === '') {
      lines.pop();
    }
    for (const line of lines) {
      rows.push({ kind, text: line });
    }
  }
  return rows;
}

/**
 * deepLinkParam — 解析 / 序列化 `?diff=vN..vM` query (content-lock §1 ⑤).
 */
export function parseDiffParam(raw: string | null): { newV: number; oldV: number } | null {
  if (!raw) return null;
  const m = raw.match(/^v(\d+)\.\.v(\d+)$/);
  if (!m) return null;
  const newV = parseInt(m[1]!, 10);
  const oldV = parseInt(m[2]!, 10);
  if (isNaN(newV) || isNaN(oldV)) return null;
  return { newV, oldV };
}

export function formatDiffParam(newV: number, oldV: number): string {
  return `v${newV}..v${oldV}`;
}

export default function DiffView({ newBody, newVersion, oldBody, oldVersion, kind = 'markdown' }: Props) {
  // React hooks rules: 所有 hook 必须在 early return 之前调用 (kind 切换不
  // 能改变 hook 调用顺序). 即使 image_link 路径不用 rows, useMemo 也仍
  // 在 hooks 列表中 — 用 `useMemo(() => kind==='image_link' ? [] : ...)` 把
  // 计算条件化, 列表位置稳定.
  const rows = useMemo(
    () => (kind === 'image_link' ? [] : computeDiffRows(oldBody, newBody)),
    [oldBody, newBody, kind],
  );

  // image_link kind 走 fallback — jsdiff 不适用 binary URL.
  if (kind === 'image_link') {
    return (
      <div className="diff-view diff-view-fallback" data-diff-kind="image_link">
        <h4 className="diff-title">{`v${newVersion} ↔ v${oldVersion}`}</h4>
        <div className="diff-image-pair">
          <figure className="diff-image-side">
            <figcaption>{`v${oldVersion}`}</figcaption>
            {oldBody && (
              <img src={oldBody} alt={`v${oldVersion} preview`} loading="lazy" className="artifact-image" />
            )}
          </figure>
          <figure className="diff-image-side">
            <figcaption>{`v${newVersion}`}</figcaption>
            {newBody && (
              <img src={newBody} alt={`v${newVersion} preview`} loading="lazy" className="artifact-image" />
            )}
          </figure>
        </div>
      </div>
    );
  }

  return (
    <div className="diff-view" data-diff-kind={kind}>
      <h4 className="diff-title">{`v${newVersion} ↔ v${oldVersion}`}</h4>
      <pre className="diff-pre">
        {rows.map((row, i) => {
          // 设计 ③ a11y — three enum literals required by content-lock §1 ⑤.
          // grep 检查项: data-diff-line="add" / data-diff-line="del" /
          // data-diff-line="context" 三 enum 各 ≥1.
          if (row.kind === 'add') {
            return (
              <div
                key={i}
                className="diff-line diff-line-add"
                data-diff-line="add"
                aria-label="增行"
              >
                <span className="diff-marker" aria-hidden="true">+</span>
                <span className="diff-text">{row.text}</span>
              </div>
            );
          }
          if (row.kind === 'del') {
            return (
              <div
                key={i}
                className="diff-line diff-line-del"
                data-diff-line="del"
                aria-label="删行"
              >
                <span className="diff-marker" aria-hidden="true">-</span>
                <span className="diff-text">{row.text}</span>
              </div>
            );
          }
          return (
            <div
              key={i}
              className="diff-line diff-line-context"
              data-diff-line="context"
              aria-label="上下文"
            >
              <span className="diff-marker" aria-hidden="true"> </span>
              <span className="diff-text">{row.text}</span>
            </div>
          );
        })}
      </pre>
    </div>
  );
}
