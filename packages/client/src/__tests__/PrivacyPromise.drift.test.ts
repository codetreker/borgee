// PrivacyPromise.drift.test.ts — ADM-1 doc-as-truth drift 锁.
//
// 锚: docs/qa/adm-1-implementation-spec.md §4 反向断言 5 (drift test).
//   "doc §4.1 = PRIVACY_PROMISES 常量"
//   跟 CM-onboarding TestWelcomeConstantsMirrorMigrations 同模式.
//
// 任何 admin-model.md §4.1 字面修改必须同步 PrivacyPromise.tsx 常量,
// 反之亦然 — CI 拦避免漂移 (跟 #211 §1 + #228 spec 双声明锁同精神).
import { describe, it, expect } from 'vitest';
import { PRIVACY_PROMISES } from '../components/Settings/PrivacyPromise';
// vite ?raw import — 跟 ArtifactPanel-kind-switch.test.tsx 同模式
// (avoids node fs deps in client package).
// @ts-ignore vite ?raw import lacks built-in TS module decl
import adminModelSrc from '../../../../docs/blueprint/current/admin-model.md?raw';

describe('PrivacyPromise drift — doc §4.1 ↔ 组件常量 byte-identical', () => {
  it('admin-model.md §4.1 三条承诺字面 1:1 跟 PRIVACY_PROMISES 同源', () => {
    const src = adminModelSrc as string;

    // §4.1 段落定位 — heading 锚 byte-identical.
    expect(src).toContain('### 4.1 用户侧隐私承诺页文案 (ADM-1 acceptance 硬标尺)');

    // 三条承诺字面 1:1 (含 markdown bold 标记 + 顺序号).
    // 任何字面修改必须同步两边, 否则此 test 红 — CI 拦.
    for (let i = 0; i < PRIVACY_PROMISES.length; i++) {
      const promise = PRIVACY_PROMISES[i]!;
      const numbered = `${i + 1}. ${promise}`;
      expect(
        src,
        `承诺 ${i + 1} 字面跟 admin-model §4.1 不一致 — 改一边必须改两边`,
      ).toContain(numbered);
    }
  });
});
