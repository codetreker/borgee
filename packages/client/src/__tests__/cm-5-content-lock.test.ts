// cm-5-content-lock.test.ts — CM-5.3 client SPA 文案 + DOM attr lock.
//
// Spec: docs/implementation/modules/cm-5-spec.md §1.3 + §3 client UI 段.
// Acceptance: docs/qa/acceptance-templates/cm-5.md §3.1-§3.4.
// Blueprint: concept-model.md §1.3 §185 (透明协作 — agent↔agent 协作
// 用户感知 owner-first 不被 ai_only 隐藏).
//
// Sources cross-referenced (byte-identical 多源 同根, 改一处必改全部):
//   - X2 conflict toast 字面 `正在被 agent {name} 处理` (锁 lib/cm5-toast.ts
//     formatCM5X2ConflictToast + acceptance §3.2 + spec §1.3 三源同源)
//   - DOM hover anchor `data-cm5-collab-link` 锁 ChannelMembersModal agent
//     行 (跟 mention render @{display_name} DM-2.3 #388 同源, hover 显示
//     "正在协作")
//   - 约束: 反向断言 ai_only / agent_only / visibility_scope DOM attr 在
//     channel/agent UI 0 hit (蓝图 §185 透明协作设计 — owner-first 视角
//     看到完整链, 反 owner_visibility_scope 多源)
//   - 约束: 不订阅 push frame — `agent_config_update` 单引号代码字面 0
//     hit (BPP frame 留 AL-2b + BPP-3, CM-5 设计 ① 走人 path 不开新 frame)
//   - X2 toast 错码字面跟历史一致 — 反向断言 CM-5 自起 X2 错码同义词 0 hit
//     (cm5.x2_conflict / agent_collision / artifact.x2_conflict / x2_lock_held)
//     强制复用 CV-4 #380 ⑦ 既有路径 (server-side 约束 grep 守见
//     cm5stance.TestCM51_X2ConflictLiteralReuse)

import { describe, it, expect } from 'vitest';
// @ts-expect-error — node:module 没 @types/node, vitest node 上下文可达.
import { createRequire } from 'module';
import {
  formatCM5X2ConflictToast,
  CM5_X2_CONFLICT_TOAST_PREFIX,
  CM5_X2_CONFLICT_TOAST_SUFFIX,
  CM5_COLLAB_LINK_DOM_ATTR,
  CM5_FORBIDDEN_VISIBILITY_DOM_ATTRS,
} from '../lib/cm5-toast';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const url: any = nodeRequire('url');

const HERE = nodePath.dirname(url.fileURLToPath(import.meta.url));
const TOAST_LIB = nodePath.join(HERE, '..', 'lib', 'cm5-toast.ts');
const MEMBERS_MODAL = nodePath.join(HERE, '..', 'components', 'ChannelMembersModal.tsx');

function read(file: string): string {
  return fs.readFileSync(file, 'utf-8');
}

describe('CM-5.3 content-lock literals + DOM attrs', () => {
  const toastLib = read(TOAST_LIB);
  const membersModal = read(MEMBERS_MODAL);

  it('① X2 conflict toast 字面 byte-identical: "正在被 agent {name} 处理"', () => {
    // formatCM5X2ConflictToast(name) returns "正在被 agent {name} 处理".
    const got = formatCM5X2ConflictToast('Helper');
    expect(got).toBe('正在被 agent Helper 处理');
    // Suffix + prefix const literals (used by reverse-grep).
    expect(CM5_X2_CONFLICT_TOAST_PREFIX).toBe('正在被 agent ');
    expect(CM5_X2_CONFLICT_TOAST_SUFFIX).toBe(' 处理');
    // Lib source must contain the prefix literal (锁定 — mismatch detection).
    expect(toastLib).toContain('正在被 agent ');
    expect(toastLib).toContain(' 处理');
  });

  it('② DOM hover anchor data-cm5-collab-link 锁 ChannelMembersModal agent 行', () => {
    expect(CM5_COLLAB_LINK_DOM_ATTR).toBe('data-cm5-collab-link');
    // ChannelMembersModal must render this attr on agent member-name span.
    expect(membersModal).toContain("'data-cm5-collab-link': ''");
  });

  it('③ 约束 ai_only / agent_only DOM attr 不渲染 (channel/agent UI)', () => {
    // 蓝图 §185 透明协作设计 — 反 owner_visibility scope 多源.
    // membersModal 是 channel/agent UI 真渲染 source — 反向断言 0 hit.
    // (toastLib 只是 lib 定义这些为约束 const, 出现在反向断言 array 内
    // 是 intentional, 不算 leak.)
    for (const forbidden of CM5_FORBIDDEN_VISIBILITY_DOM_ATTRS) {
      expect(membersModal).not.toContain(forbidden);
    }
  });

  it('④ 约束 不订阅 push frame (BPP frame 留 AL-2b + BPP-3)', () => {
    // CM-5 设计 ① 走人 path 不开新 frame. 单引号字面 (代码使用形式) 0 hit.
    const FRAME = 'agent_config' + '_update'; // 拼接防 lint 自 trip.
    expect(membersModal).not.toContain(`'${FRAME}'`);
    expect(membersModal).not.toContain(`"${FRAME}"`);
    expect(toastLib).not.toContain(`'${FRAME}'`);
    expect(toastLib).not.toContain(`"${FRAME}"`);
    // 反向 ws subscription / hub.subscribe 调用 0 hit in CM-5 lib.
    expect(toastLib).not.toContain('subscribeWS');
    expect(toastLib).not.toContain('hub.subscribe');
  });

  it('⑤ 约束 X2 错码同义词 0 hit (强制复用 CV-4 #380 ⑦ 既有路径)', () => {
    // CM-5 设计 ③ 字面: X2 冲突复用 CV-4 既有错码 `artifact.locked_by_
    // another_iteration` byte-identical. 反向 reject CM-5 自起同义词
    // (跟 cm5stance.TestCM51_X2ConflictLiteralReuse server-side 约束
    // 守同源).
    for (const drift of [
      'cm5.x2_conflict',
      'agent_collision',
      'artifact.x2_conflict',
      'x2_lock_held',
    ]) {
      expect(toastLib).not.toContain(`'${drift}'`);
      expect(toastLib).not.toContain(`"${drift}"`);
      expect(membersModal).not.toContain(`'${drift}'`);
      expect(membersModal).not.toContain(`"${drift}"`);
    }
  });

  it('约束: X2 toast 同义词漂移 0 hit (字面唯一根)', () => {
    // 反向同义词 byte-identical reject — 改 toast 必须改 spec/acceptance/
    // server const 三处同源.
    for (const drift of [
      '正在被 agent 占用',
      '正在被 agent 锁定',
      '冲突: agent',
      'agent X2 conflict',
      '已被 agent',
    ]) {
      expect(toastLib).not.toContain(drift);
    }
  });
});
