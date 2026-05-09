// cm5-toast.ts — CM-5 X2 冲突 + 协作可见性 toast 文案锁.
//
// Spec: docs/implementation/modules/cm-5-spec.md §1.3 + §3 + 设计 ③
//   X2 冲突复用 CV-1.2 single-doc lock + CV-4 #380 ⑦ 错码字面 + ⑤
//   透明 owner-first 可见性.
// Acceptance: docs/qa/acceptance-templates/cm-5.md §3 client UI.
// Blueprint: concept-model.md §1.3 §185 (透明协作 — agent↔agent 协作
// 用户感知不被 ai_only 隐藏).
//
// Sources cross-referenced (byte-identical 多源 同根, 改一处必改全部):
//   - X2 toast 字面 "正在被 agent {name} 处理" 跟 server `artifact.
//     locked_by_another_iteration` 错码同源 (cm5stance.TestCM51_X2Conflict
//     LiteralReuse 反约束守 — 强制复用既有 lock conflict path).
//   - DOM hover anchor `data-cm5-collab-link` (锁 ChannelMembersModal
//     agent 行 hover 显示 "正在协作" 提示).
//   - 反约束: 不订阅 push frame (BPP frame 留 AL-2b + BPP-3, CM-5
//     走轮询 + 既有 path), 不引 ai_only visibility scope.

/**
 * X2 conflict toast 文案锁 — agent collision 时 toast.
 * 字面 "正在被 agent {name} 处理" byte-identical 跟 cm-5-spec.md §1.3
 * + acceptance §3.2 同源.
 *
 * 改此字面 = 改 cm-5-content-lock.test.ts case ① + acceptance §3.2 三处
 * 同步.
 */
export function formatCM5X2ConflictToast(agentName: string): string {
  return `正在被 agent ${agentName} 处理`;
}

/**
 * X2 conflict toast 字面前缀 (无 agent name 占位形式) — 用于 vitest
 * content-lock 检测字面是否变.
 */
export const CM5_X2_CONFLICT_TOAST_PREFIX = '正在被 agent ';
export const CM5_X2_CONFLICT_TOAST_SUFFIX = ' 处理';

/**
 * Hover anchor DOM attr — agent 行 hover 显示 "正在协作" 提示.
 * 锁 ChannelMembersModal 的 agent member-name span (data-cm5-collab-link
 * 空字符串 attr, 锁 DOM 字面).
 */
export const CM5_COLLAB_LINK_DOM_ATTR = 'data-cm5-collab-link';

/**
 * 反约束 (蓝图 §185 透明协作设计): 不渲染 ai_only / agent_only
 * visibility scope 字段. 此 array 在 vitest grep 检查 守 — 任何这些
 * DOM attr 出现在 channel/agent 视图都 reject.
 */
export const CM5_FORBIDDEN_VISIBILITY_DOM_ATTRS = [
  'data-ai-only',
  'data-agent-only',
  'data-visibility-scope',
  'data-agent-visible-only',
] as const;
