// VisibilityBadge.test.tsx — CHN-9.2 three-state DOM byte-identical checks,
// locked Chinese labels, synonym rejection, and Visibility constant alignment.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { VisibilityBadge } from '../components/VisibilityBadge';
import {
  VISIBILITY_CREATOR_ONLY,
  VISIBILITY_MEMBERS,
  VISIBILITY_ORG_PUBLIC,
  VISIBILITY_LABELS,
  isValidVisibility,
} from '../lib/visibility';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

describe('VisibilityBadge — CHN-9.2 three-state DOM and locked labels', () => {
  it('creator_only label=`🔒 仅创建者` remains byte-identical', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<VisibilityBadge visibility="creator_only" />);
    });
    const badge = container!.querySelector('[data-visibility="creator_only"]') as HTMLElement;
    expect(badge).not.toBeNull();
    expect(badge.textContent).toBe('🔒 仅创建者');
    expect(badge.getAttribute('title')).toBe('可见性: 仅创建者');
  });

  it('private label=`👥 成员可见` remains byte-identical', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<VisibilityBadge visibility="private" />);
    });
    const badge = container!.querySelector('[data-visibility="private"]') as HTMLElement;
    expect(badge).not.toBeNull();
    expect(badge.textContent).toBe('👥 成员可见');
  });

  it('public label=`🌐 组织内可见` remains byte-identical', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<VisibilityBadge visibility="public" />);
    });
    const badge = container!.querySelector('[data-visibility="public"]') as HTMLElement;
    expect(badge).not.toBeNull();
    expect(badge.textContent).toBe('🌐 组织内可见');
  });

  it('Visibility constants stay byte-identical and isValidVisibility accepts only known values', () => {
    expect(VISIBILITY_CREATOR_ONLY).toBe('creator_only');
    expect(VISIBILITY_MEMBERS).toBe('private');
    expect(VISIBILITY_ORG_PUBLIC).toBe('public');
    expect(isValidVisibility('creator_only')).toBe(true);
    expect(isValidVisibility('private')).toBe(true);
    expect(isValidVisibility('public')).toBe(true);
    for (const bad of ['secret', 'team', 'Public', '', 'Private']) {
      expect(isValidVisibility(bad)).toBe(false);
    }
    expect(VISIBILITY_LABELS.creator_only.emoji).toBe('🔒');
    expect(VISIBILITY_LABELS.creator_only.text).toBe('仅创建者');
    expect(VISIBILITY_LABELS.private.text).toBe('成员可见');
    expect(VISIBILITY_LABELS.public.text).toBe('组织内可见');
  });

  it('synonym rejection: forbidden visibility labels do not appear in user-visible text', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<VisibilityBadge visibility="creator_only" />);
    });
    const badge = container!.querySelector('[data-visibility="creator_only"]') as HTMLElement;
    const text = badge.textContent || '';
    const forbidden = ['secret', 'exclusive', 'team-only', '外部', '绝密', '公共'];
    for (const f of forbidden) {
      expect(text).not.toContain(f);
    }
  });
});
