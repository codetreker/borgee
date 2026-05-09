// ArtifactPanel-kind-switch.test.tsx — CV-3.3 acceptance §2.1 §2.7 vitest 锁.
//
// 锚: docs/qa/cv-3-content-lock.md §1 ① ⑦ + acceptance §2.1 §2.7 + spec §0 设计 ①.
//
// 反约束: data-artifact-kind 三 enum byte-identical (markdown / code /
// image_link), 反 camelCase imageLink + 同义词 pdf/kanban/mindmap.
//
// 这层锁验 normalizeKind 逻辑 + DOM 字面 (data-artifact-kind 三 enum) 通过
// fetch panel source via Vite ?raw import — fs 路径在 jsdom env 下不可用.
import { describe, it, expect } from 'vitest';
import { normalizeKind } from '../components/ArtifactPanel';
// Vite raw import — 跟 #346 markdown-content-lock.test 同模式 (loaded
// via vitest-vite, 字面 grep DOM 锁不依赖 node fs).
// @ts-ignore vite ?raw import lacks built-in TS module decl
import panelSrc from '../components/ArtifactPanel.tsx?raw';

describe('normalizeKind — 三 enum 收口 (设计 ①)', () => {
  it('passes through markdown / code / image_link', () => {
    expect(normalizeKind('markdown')).toBe('markdown');
    expect(normalizeKind('code')).toBe('code');
    expect(normalizeKind('image_link')).toBe('image_link');
  });

  it('undefined defaults to markdown (CV-1 兼容)', () => {
    expect(normalizeKind(undefined)).toBe('markdown');
  });

  it('unknown kind passes through verbatim (兜底文案展示原值)', () => {
    expect(normalizeKind('future_kanban')).toBe('future_kanban');
  });
});

describe('ArtifactPanel — DOM 字面 byte-identical (acceptance §2.1)', () => {
  const src: string = panelSrc;

  it('emits data-artifact-kind="markdown" literal', () => {
    expect(src).toMatch(/data-artifact-kind="markdown"/);
  });

  it('emits data-artifact-kind="code" literal', () => {
    expect(src).toMatch(/data-artifact-kind="code"/);
  });

  it('emits data-artifact-kind="image_link" literal (snake_case lock)', () => {
    expect(src).toMatch(/data-artifact-kind="image_link"/);
  });

  it.each(['imageLink', 'code_image', "'pdf'", "'kanban'", "'mindmap'"])(
    'rejects 同义词 / camelCase drift: %s',
    (bad) => {
      expect(src).not.toContain(bad);
    },
  );

  it('renders 兜底文案 byte-identical (content-lock §1 ⑦)', () => {
    expect(src).toContain('暂不支持渲染');
  });

  it.each(['Unsupported', '未支持', '类型错误', 'Unknown kind'])(
    'rejects 兜底文案漂移: %s',
    (bad) => {
      expect(src).not.toContain(bad);
    },
  );
});
