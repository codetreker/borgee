// CS-2 — Client Shape 故障三态 SSOT (蓝图 client-shape.md §1.3).
//
// 三态枚举 byte-identical 跟蓝图 §1.3 表 (野马 push back 收敛锁):
//   - online  : runtime 已连接
//   - error   : API key 失效 / 超限 / 进程崩溃 / 网络断 (跟既有
//               PresenceDot data-presence="error" 字面 byte-identical, AL-3 锁)
//   - offline : disable / 用户主动关
//
// 反约束 (cs-2-stance-checklist 设计 ①):
//   - 不允许第 4 态 'busy' / 'idle' 漂入 (那是 AL-1b §2.3 BPP progress
//     frame 真实施时 v2 才加; CS-2 三态拆死锁).
//   - grep 检查 `'busy'|'idle'|'standby'` 在 cs-2-* 0 hit (拆死锁).

export const FAILURE_TRI_STATE = ['online', 'error', 'offline'] as const;

export type FailureState = (typeof FAILURE_TRI_STATE)[number];

/** IsFailureState — 单源 helper (跟 reasons.IsValid #496 SSOT 包同模式). */
export function IsFailureState(s: string): s is FailureState {
  return (FAILURE_TRI_STATE as readonly string[]).includes(s);
}
