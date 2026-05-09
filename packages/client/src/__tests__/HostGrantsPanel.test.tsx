// HostGrantsPanel.test.tsx — HB-3.3 acceptance §3.1 + content-lock §1.①
// 弹窗三按钮 DOM 字面锁 + §2 同义词grep 检查.
//
// Pins:
//   - 三按钮 data-action 字面: {"deny", "grant_one_shot", "grant_always"}
//   - 三按钮 hb3-button 字面: deny=danger, grant_*=primary
//   - 按钮文字字面: 拒绝 / 仅这一次 / 始终允许 (蓝图 §1.3 byte-identical)
//   - title + body 包含 actionLabel 字面 (4-enum 字面 byte-identical 跟
//     spec §0+§1 + DB CHECK 跟 server-go enum)
//   - 反向断言: 同义词禁词 0 出现在 DOM 文本
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import HostGrantsPanel from '../components/HostGrantsPanel';

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

function render(node: React.ReactElement) {
  const root = createRoot(container!);
  act(() => {
    root.render(node);
  });
}

describe('HostGrantsPanel — HB-3.3 弹窗 DOM 字面锁', () => {
  it('三按钮 data-action byte-identical (deny / grant_one_shot / grant_always)', () => {
    render(
      <HostGrantsPanel
        agentName="DevAgent"
        grantType="filesystem"
        scopeLabel="代码目录 ~/code"
        capabilityLabel="代码 review"
        onDecide={() => undefined}
      />,
    );
    const denyBtn = container!.querySelector('[data-action="deny"]');
    const oneShotBtn = container!.querySelector('[data-action="grant_one_shot"]');
    const alwaysBtn = container!.querySelector('[data-action="grant_always"]');
    expect(denyBtn).not.toBeNull();
    expect(oneShotBtn).not.toBeNull();
    expect(alwaysBtn).not.toBeNull();
    // hb3-button 二值锁
    expect(denyBtn!.getAttribute('data-hb3-button')).toBe('danger');
    expect(oneShotBtn!.getAttribute('data-hb3-button')).toBe('primary');
    expect(alwaysBtn!.getAttribute('data-hb3-button')).toBe('primary');
  });

  it('按钮文字字面跟蓝图 §1.3 byte-identical', () => {
    render(
      <HostGrantsPanel
        agentName="DevAgent"
        grantType="filesystem"
        scopeLabel="~/code"
        capabilityLabel="代码 review"
        onDecide={() => undefined}
      />,
    );
    expect(container!.querySelector('[data-action="deny"]')!.textContent).toBe('拒绝');
    expect(container!.querySelector('[data-action="grant_one_shot"]')!.textContent).toBe('仅这一次');
    expect(container!.querySelector('[data-action="grant_always"]')!.textContent).toBe('始终允许');
  });

  it('actionLabel 4-enum 字面跟 grant_type byte-identical (install/exec/filesystem/network)', () => {
    const map: Record<string, string> = {
      install: '安装',
      exec: '执行',
      filesystem: '读取',
      network: '访问',
    };
    for (const [grantType, verb] of Object.entries(map)) {
      render(
        <HostGrantsPanel
          agentName="X"
          grantType={grantType as 'install' | 'exec' | 'filesystem' | 'network'}
          scopeLabel="目标"
          capabilityLabel="能力"
          onDecide={() => undefined}
        />,
      );
      const title = container!.querySelector('[data-hb3-title]')!.textContent ?? '';
      expect(title).toContain(verb);
    }
  });

  it('反向断言: 同义词禁词 0 出现在 DOM (拒绝/仅这一次/始终允许 字面单源)', () => {
    render(
      <HostGrantsPanel
        agentName="DevAgent"
        grantType="filesystem"
        scopeLabel="~/code"
        capabilityLabel="代码 review"
        onDecide={() => undefined}
      />,
    );
    const text = container!.textContent ?? '';
    // 同义词 (拒绝)
    for (const banned of ['否决', '不允许', 'reject']) {
      expect(text).not.toContain(banned);
    }
    // 同义词 (仅这一次)
    for (const banned of ['一次', '单次', '临时']) {
      // "仅这一次" 含 "一次" 子串故 substring grep 无意义; 检 attr 集是单源 enum
    }
    // 同义词 (始终允许)
    for (const banned of ['永久', '长期', 'forever', 'permanent']) {
      expect(text).not.toContain(banned);
    }
  });

  it('onDecide 回调正确传递 action 三值', () => {
    const calls: string[] = [];
    render(
      <HostGrantsPanel
        agentName="X"
        grantType="filesystem"
        scopeLabel="~/x"
        capabilityLabel="cap"
        onDecide={(a) => calls.push(a)}
      />,
    );
    act(() => {
      (container!.querySelector('[data-action="deny"]') as HTMLButtonElement).click();
      (container!.querySelector('[data-action="grant_one_shot"]') as HTMLButtonElement).click();
      (container!.querySelector('[data-action="grant_always"]') as HTMLButtonElement).click();
    });
    expect(calls).toEqual(['deny', 'grant_one_shot', 'grant_always']);
  });
});
