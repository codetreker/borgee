// CodeRenderer — CV-3.3 client (kind='code') renderer.
//
// Spec: docs/implementation/modules/cv-3-spec.md §0 设计 ① 三 enum
//   + §1 CV-3.2 client; 文案锁: docs/qa/cv-3-content-lock.md §1 ②③
//   + acceptance: docs/qa/acceptance-templates/cv-3.md §2.2 §2.3.
// Schema 锚: cv_3_1_artifact_kinds.go (#396, kind='code') +
//   cv_3_2_artifact_validation.go ValidCodeLanguages (#400, 11+1).
//
// 设计反查:
//   - ① 12 项语言短码白名单 byte-identical 跟 server ValidCodeLanguages
//     同源 (CODE_LANGUAGES from lib/code-languages.ts).
//   - ② 复制按钮文案锁 — title/aria 中文双绑, icon 锁 📋, toast 锁 1.5s.
//   - ③ 复制按钮只在 code kind 渲染 (kind switch 上层就分到此组件).
//
// 反约束 (CodeRenderer.tsx 路径grep 检查 干净, content-lock §2 一致):
//   - 短码唯一 (drift 全名同义词被 lib/code-languages.ts + prism-lang-map.ts
//     收口, 本文件不出现全名 — CodeRenderer.tsx grep 检查 0 hit)
//   - 复制文案中文锁 (grep 检查 0 hit)
//   - 不用 dangerouslyset html (XSS; prism 走 React 节点)
import { useCallback, useState } from 'react';
import { Highlight, themes } from 'prism-react-renderer';
import { useToast } from './Toast';
import {
  CODE_LANGUAGES,
  LANG_LABEL,
  normalizeLang,
  type CodeLanguage,
} from '../lib/code-languages';
import { PRISM_LANG_MAP } from '../lib/prism-lang-map';

// Re-export so callers (MentionArtifactPreview, vitest) can import from
// the renderer without leaking the internal lib path.
export { CODE_LANGUAGES, LANG_LABEL, normalizeLang };
export type { CodeLanguage };

interface Props {
  body: string;
  /** language short-code; if omitted/invalid, falls back to 'text'. */
  language?: string;
}

export default function CodeRenderer({ body, language }: Props) {
  const lang = normalizeLang(language);
  const { showToast } = useToast();
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(body);
      } else {
        const ta = document.createElement('textarea');
        ta.value = body;
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
      }
      // Toast 文案 byte-identical (content-lock §1 ③, 1.5s 自动消由
      // ToastProvider 4s 默认覆盖 — short-form `已复制` 跟 spec 同).
      showToast('已复制');
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      showToast('复制失败');
    }
  }, [body, showToast]);

  return (
    <div className="code-renderer">
      <div className="code-renderer-header">
        <span
          className="code-lang-badge"
          data-lang={lang}
        >
          {LANG_LABEL[lang]}
        </span>
        <button
          type="button"
          className="code-copy-btn"
          title="复制代码"
          aria-label="复制代码"
          onClick={handleCopy}
          data-copied={copied ? '1' : '0'}
        >
          📋
        </button>
      </div>
      <Highlight
        theme={themes.vsLight}
        code={body}
        language={PRISM_LANG_MAP[lang]}
      >
        {({ className, style, tokens, getLineProps, getTokenProps }) => (
          <pre className={`code-renderer-pre ${className}`} style={style}>
            {tokens.map((line, i) => {
              const lineProps = getLineProps({ line });
              return (
                <div key={i} {...lineProps}>
                  {line.map((token, key) => {
                    const tokenProps = getTokenProps({ token });
                    return <span key={key} {...tokenProps} />;
                  })}
                </div>
              );
            })}
          </pre>
        )}
      </Highlight>
    </div>
  );
}
