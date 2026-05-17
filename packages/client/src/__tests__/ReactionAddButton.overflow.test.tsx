// ReactionAddButton.overflow.test.tsx — gh#bug-reaction-overflow 视口翻转覆盖
//
// 9 cases (OF-1..OF-9) 锁住 ReactionAddButton.tsx useLayoutEffect 翻转逻辑:
//   - OF-1 mobile sheet detection (innerWidth <= 768)
//   - OF-2 desktop left-anchor (btn 在视口左半)
//   - OF-3 desktop right-anchor (btn 靠右导致 popover 沿 left:0 会溢出)
//   - OF-4 cross-boundary resize 1440 → 400 (带 pre/post 两段断言, 防"残留
//          inline left:0 / right:auto 胜过 mobile media query"回归)
//   - OF-5 zero-rect guard (rect.width === 0 && rect.height === 0 → 保留 null,
//          走 visibility:hidden, 不写 inline)
//   - OF-6 边界 768 (用 <= 跟 CSS @media (max-width:768px) 同语义, 不留 1px 缝)
//   - OF-7 close → resize listener 按 fn 身份 (identity) 被卸 (C3 hardening)
//   - OF-8 StrictMode 幂等 (useLayoutEffect double-mount 不应让 align 在
//          deterministic input 下抖动)
//   - OF-9 Escape → picker 卸载 + 重开 align 重新 null→measured (无 stale)
//
// 遵守 skeptic-owner 4 项 hardening:
//   - C1 per-test rect mock mandatory: beforeEach 装非零默认 rect, 防 jsdom
//        {0,0,0,0} 默认让 zero-rect guard (ReactionAddButton.tsx:111) 静默
//        吞掉 flip → 测试假绿. 每个桌面用例前先 assertFlipActuallyRan(popover)
//        确认 align !== 'measuring' / align !== null.
//   - C2 OF-4 必须做 pre-resize 断言: 先证 style.left === '0px' &&
//        style.right === 'auto', 再证 resize 后变成 ''.  少了 pre 就证不出
//        bug 真复现过.
//   - C3 OF-7 用 fn-identity: 捕 addEventListener('resize', fn) 传入的 fn,
//        断言 removeEventListener 收到同一引用 (不靠 name-only filter, 防
//        StrictMode double-mount 假绿).
//   - C4 OUT-1 / OUT-2 真几何 (popover.right <= innerWidth - 4) 需要 Playwright;
//        vitest+jsdom 不能算真 CSS layout.  本文件作为 proxy: 断言当
//        rect.left + 352 > innerWidth - 8 时, 组件写 right:0 left:auto inline,
//        机械上保证 CSS 中 popover.right === button.right === innerWidth - 16
//        (无溢出).  真几何留 Playwright e2e (本环境无浏览器执行能力).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

// mock @emoji-mart/react: 避免拉重依赖 + picker DOM 不影响 align 测量
vi.mock('@emoji-mart/react', () => ({
  default: ({ onEmojiSelect }: { onEmojiSelect: (e: { native: string }) => void }) =>
    React.createElement('div', {
      'data-test': 'emoji-mart',
      onClick: () => onEmojiSelect({ native: '👍' }),
    }, 'mock-picker'),
}));
vi.mock('@emoji-mart/data', () => ({ default: {} }));

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    addReaction: vi.fn().mockResolvedValue(undefined),
    removeReaction: vi.fn().mockResolvedValue(undefined),
  };
});

const mockDispatch = vi.fn();
const mockShowToast = vi.fn();
vi.mock('../context/AppContext', () => ({
  useAppContext: () => ({ dispatch: mockDispatch }),
}));
vi.mock('../components/Toast', () => ({
  useToast: () => ({ showToast: mockShowToast }),
}));

import ReactionAddButton from '../components/ReactionAddButton';

let container: HTMLDivElement | null = null;
let root: Root | null = null;
let originalInnerWidth: number;

// 非零默认 rect — C1 强制: 每 test 起码有这个垫底, 不让 jsdom {0,0,0,0}
// 路过 zero-rect guard 让 desktop flip 测试假绿.
const DEFAULT_RECT: DOMRect = {
  left: 100,
  top: 200,
  width: 24,
  height: 24,
  right: 124,
  bottom: 224,
  x: 100,
  y: 200,
  toJSON: () => ({}),
};

function setInnerWidth(value: number) {
  Object.defineProperty(window, 'innerWidth', {
    value,
    configurable: true,
    writable: true,
  });
}

function mockRect(rect: Partial<DOMRect> & { left: number }) {
  // 用 vi.spyOn 装 prototype 级别 mock, afterEach restore 全清.
  const full: DOMRect = {
    ...DEFAULT_RECT,
    ...rect,
    right: (rect.left ?? DEFAULT_RECT.left) + (rect.width ?? DEFAULT_RECT.width),
    bottom: (rect.top ?? DEFAULT_RECT.top) + (rect.height ?? DEFAULT_RECT.height),
    x: rect.left ?? DEFAULT_RECT.left,
    y: rect.top ?? DEFAULT_RECT.top,
    toJSON: () => ({}),
  } as DOMRect;
  vi.spyOn(HTMLButtonElement.prototype, 'getBoundingClientRect').mockReturnValue(full);
}

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  mockDispatch.mockClear();
  mockShowToast.mockClear();
  originalInnerWidth = window.innerWidth;
  // C1 默认非零 rect; 每个用例可以再 mockRect(...) 覆盖
  vi.spyOn(HTMLButtonElement.prototype, 'getBoundingClientRect').mockReturnValue(DEFAULT_RECT);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  root = null;
  setInnerWidth(originalInnerWidth);
  vi.restoreAllMocks();
});

function render(node: React.ReactElement) {
  root = createRoot(container!);
  act(() => { root!.render(node); });
}

function openPicker(): HTMLElement {
  const btn = container!.querySelector('button[data-reaction-add-variant]') as HTMLButtonElement;
  act(() => { btn.click(); });
  const popover = container!.querySelector('.reaction-picker-popover') as HTMLElement;
  expect(popover).not.toBeNull();
  return popover;
}

// C1 helper: 测真 flip 跑过, 不是被 zero-rect guard 静默吞掉
function assertFlipActuallyRan(popover: HTMLElement) {
  const align = popover.getAttribute('data-reaction-picker-align');
  expect(align).not.toBe('measuring');
  expect(align).not.toBeNull();
}

const baseProps = {
  channelId: 'ch-1',
  messageId: 'm-1',
  currentUserId: 'u-current',
};

describe('ReactionAddButton — gh#bug-reaction-overflow 翻转 9 cases', () => {
  it('OF-1 mobile sheet detection (innerWidth=400 → align=mobile, no inline left/right)', () => {
    setInnerWidth(400);
    // mobile 路径走 innerWidth <= 768 早返回, rect 用不到, 但 C1 hygiene 留默认
    render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
    const popover = openPicker();
    expect(popover.getAttribute('data-reaction-picker-align')).toBe('mobile');
    expect((popover as HTMLElement).style.left).toBe('');
    expect((popover as HTMLElement).style.right).toBe('');
  });

  it('OF-2 desktop left-anchor (innerWidth=1440, rect.left=100 → align=left, left:0 right:auto)', () => {
    setInnerWidth(1440);
    mockRect({ left: 100, top: 200, width: 24, height: 24 });
    render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
    const popover = openPicker();
    assertFlipActuallyRan(popover);
    expect(popover.getAttribute('data-reaction-picker-align')).toBe('left');
    expect((popover as HTMLElement).style.left).toBe('0px');
    expect((popover as HTMLElement).style.right).toBe('auto');
  });

  it('OF-3 desktop right-anchor (innerWidth=1440, rect.left=1300 → align=right, left:auto right:0)', () => {
    setInnerWidth(1440);
    mockRect({ left: 1300, top: 200, width: 24, height: 24 });
    render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
    const popover = openPicker();
    assertFlipActuallyRan(popover);
    expect(popover.getAttribute('data-reaction-picker-align')).toBe('right');
    expect((popover as HTMLElement).style.left).toBe('auto');
    expect((popover as HTMLElement).style.right).toBe('0px');
  });

  it('OF-4 cross-boundary resize 1440→400 (C2: pre-resize inline values asserted first)', () => {
    setInnerWidth(1440);
    mockRect({ left: 100, top: 200, width: 24, height: 24 });
    render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
    const popover = openPicker();
    assertFlipActuallyRan(popover);

    // C2 pre-resize: 必须先证 desktop 路径真写了 inline left:0 / right:auto,
    // 否则后面 "resize 后变 ''" 是空证 (没真复现过 bug).
    expect(popover.getAttribute('data-reaction-picker-align')).toBe('left');
    expect((popover as HTMLElement).style.left).toBe('0px');
    expect((popover as HTMLElement).style.right).toBe('auto');

    // 跨边界 resize 到移动
    setInnerWidth(400);
    act(() => {
      window.dispatchEvent(new Event('resize'));
    });

    // 重新查 popover (可能被 React rerender 换节点)
    const popover2 = container!.querySelector('.reaction-picker-popover') as HTMLElement;
    expect(popover2).not.toBeNull();
    expect(popover2.getAttribute('data-reaction-picker-align')).toBe('mobile');
    expect(popover2.style.left).toBe('');
    expect(popover2.style.right).toBe('');
  });

  it('OF-5 zero-rect guard ({0,0,0,0} → align 维持 null / data-attr=measuring)', () => {
    setInnerWidth(1440);
    // overwrite mock with zero-rect — D1 silent-fallback path
    vi.spyOn(HTMLButtonElement.prototype, 'getBoundingClientRect').mockReturnValue({
      left: 0, top: 0, width: 0, height: 0, right: 0, bottom: 0, x: 0, y: 0,
      toJSON: () => ({}),
    } as DOMRect);
    render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
    const popover = openPicker();
    // align 应停在 null → data attr 'measuring', visibility:hidden 避免闪烁
    expect(popover.getAttribute('data-reaction-picker-align')).toBe('measuring');
    expect(popover.style.visibility).toBe('hidden');
  });

  it('OF-6 boundary 768 (innerWidth=768 → align=mobile, 不留 1px 缝)', () => {
    setInnerWidth(768);
    render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
    const popover = openPicker();
    // 用 <= 跟 CSS @media (max-width:768px) 同语义
    expect(popover.getAttribute('data-reaction-picker-align')).toBe('mobile');
  });

  it('OF-7 resize listener cleanup with fn-identity (C3: same fn ref added & removed)', () => {
    setInnerWidth(1440);
    mockRect({ left: 100, top: 200, width: 24, height: 24 });

    // C3: 捕获 addEventListener('resize', fn) 的 fn 引用 + 断言
    // removeEventListener('resize', fn) 拿到的是同一引用. 不靠 name-only
    // filter — 那会在 StrictMode double-mount 下假绿 (移走的是旧 closure,
    // 新 closure 还泄漏).
    const addSpy = vi.spyOn(window, 'addEventListener');
    const removeSpy = vi.spyOn(window, 'removeEventListener');

    render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
    const btn = container!.querySelector('button[data-reaction-add-variant]') as HTMLButtonElement;
    act(() => { btn.click(); }); // open

    // 收集所有 resize add — 期望恰好 1 (非 StrictMode)
    const resizeAddCalls = addSpy.mock.calls.filter(c => c[0] === 'resize');
    expect(resizeAddCalls.length).toBeGreaterThanOrEqual(1);
    const addedFn = resizeAddCalls[resizeAddCalls.length - 1]![1];

    // 关 picker → 触发 useLayoutEffect cleanup → 必须 removeEventListener
    // 同一 fn ref
    act(() => { btn.click(); }); // close

    const resizeRemoveCalls = removeSpy.mock.calls.filter(c => c[0] === 'resize');
    expect(resizeRemoveCalls.length).toBeGreaterThanOrEqual(1);
    // C3 identity check: 同一 fn 被 add + 被 remove
    const removedFns = resizeRemoveCalls.map(c => c[1]);
    expect(removedFns).toContain(addedFn);
  });

  it('OF-8 StrictMode idempotence (double-mount 下 align 不抖动)', () => {
    setInnerWidth(1440);
    mockRect({ left: 100, top: 200, width: 24, height: 24 });
    render(
      <React.StrictMode>
        <ReactionAddButton {...baseProps} variant="toolbar-btn" />
      </React.StrictMode>
    );
    const popover = openPicker();
    assertFlipActuallyRan(popover);
    // 同一 rect + 同一 innerWidth 下, 不管 double-mount 触发几次 measure,
    // 最终 align 应 deterministic = 'left'
    expect(popover.getAttribute('data-reaction-picker-align')).toBe('left');
    expect((popover as HTMLElement).style.left).toBe('0px');
    expect((popover as HTMLElement).style.right).toBe('auto');
  });

  it('OF-9 Escape resets align (close + 重开 → 重新 measure, 无 stale align)', () => {
    setInnerWidth(1440);
    mockRect({ left: 100, top: 200, width: 24, height: 24 });
    render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
    const popover = openPicker();
    assertFlipActuallyRan(popover);
    expect(popover.getAttribute('data-reaction-picker-align')).toBe('left');

    // Escape → useEffect 监 keydown → setOpen(false) → popover 卸载
    act(() => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    });
    expect(container!.querySelector('.reaction-picker-popover')).toBeNull();

    // 重开: 把 mock 换成靠右, 期望 align 真重新 measure (不 stale 留 'left')
    mockRect({ left: 1300, top: 200, width: 24, height: 24 });
    const popover2 = openPicker();
    assertFlipActuallyRan(popover2);
    expect(popover2.getAttribute('data-reaction-picker-align')).toBe('right');
    expect(popover2.style.left).toBe('auto');
    expect(popover2.style.right).toBe('0px');
  });
});
