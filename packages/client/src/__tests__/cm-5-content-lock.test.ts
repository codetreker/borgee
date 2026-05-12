// cm-5-content-lock.test.ts — CM-5.3 client SPA copy + DOM attribute lock.
//
// Spec: docs/implementation/modules/cm-5-spec.md §1.3 + §3 client UI section.
// Acceptance: docs/qa/acceptance-templates/cm-5.md §3.1-§3.4.
// Blueprint: concept-model.md §1.3 §185 (transparent collaboration: agent↔agent
// collaboration remains owner-visible and is not hidden behind ai_only).
//
// Cross-referenced sources (byte-identical across all sources; change all of them together):
//   - X2 conflict toast literal `正在被 agent {name} 处理` (locks lib/cm5-toast.ts
//     formatCM5X2ConflictToast + acceptance §3.2 + spec §1.3)
//   - DOM hover anchor `data-cm5-collab-link` locks the ChannelMembersModal agent
//     row (same source as mention render @{display_name} DM-2.3 #388; hover shows
//     "正在协作")
//   - Constraint: ai_only / agent_only / visibility_scope DOM attributes must have
//     0 hits in channel/agent UI (blueprint §185 transparent collaboration keeps
//     the full chain visible to the owner and rejects owner_visibility_scope variants)
//   - Constraint: no push-frame subscription — `agent_config_update` single-quoted
//     code literal has 0 hits (BPP frame remains for AL-2b + BPP-3; CM-5 design ①
//     uses the human collaboration path and adds no new frame)
//   - X2 toast error-code literals stay historically consistent: CM-5-specific
//     X2 error-code synonyms must have 0 hits
//     (cm5.x2_conflict / agent_collision / artifact.x2_conflict / x2_lock_held)
//     and must reuse the existing CV-4 #380 ⑦ path (server-side grep guard:
//     cm5stance.TestCM51_X2ConflictLiteralReuse)

import { describe, it, expect } from 'vitest';
// @ts-expect-error — node:module has no @types/node here; Vitest node context can resolve it.
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
    // Lib source must contain the prefix literal for mismatch detection.
    expect(toastLib).toContain('正在被 agent ');
    expect(toastLib).toContain(' 处理');
  });

  it('② DOM hover anchor data-cm5-collab-link 锁 ChannelMembersModal agent 行', () => {
    expect(CM5_COLLAB_LINK_DOM_ATTR).toBe('data-cm5-collab-link');
    // ChannelMembersModal must render this attr on agent member-name span.
    expect(membersModal).toContain("'data-cm5-collab-link': ''");
  });

  it('③ 约束 ai_only / agent_only DOM attr 不渲染 (channel/agent UI)', () => {
    // Blueprint §185 transparent collaboration rejects owner_visibility scope variants.
    // membersModal is the actual channel/agent UI render source, so it must have 0 hits.
    // toastLib only defines these strings in the constraint array; that is intentional
    // and does not count as a leak.
    for (const forbidden of CM5_FORBIDDEN_VISIBILITY_DOM_ATTRS) {
      expect(membersModal).not.toContain(forbidden);
    }
  });

  it('④ 约束 不订阅 push frame (BPP frame 留 AL-2b + BPP-3)', () => {
    // CM-5 design ① uses the human collaboration path and adds no new frame.
    // The single-quoted code literal form must have 0 hits.
    const FRAME = 'agent_config' + '_update'; // concatenate to avoid tripping this lint check.
    expect(membersModal).not.toContain(`'${FRAME}'`);
    expect(membersModal).not.toContain(`"${FRAME}"`);
    expect(toastLib).not.toContain(`'${FRAME}'`);
    expect(toastLib).not.toContain(`"${FRAME}"`);
    // Reverse check: ws subscription / hub.subscribe calls have 0 hits in CM-5 lib.
    expect(toastLib).not.toContain('subscribeWS');
    expect(toastLib).not.toContain('hub.subscribe');
  });

  it('⑤ 约束 X2 错码同义词 0 hit (强制复用 CV-4 #380 ⑦ 既有路径)', () => {
    // CM-5 design ③: X2 conflicts reuse the existing CV-4 error code
    // `artifact.locked_by_another_iteration` byte-identical. Reject CM-5-specific
    // synonyms and keep this aligned with the server-side constraint
    // cm5stance.TestCM51_X2ConflictLiteralReuse.
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
    // Byte-identical synonym rejection: changing the toast requires changing the
    // spec, acceptance text, and server const together.
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
