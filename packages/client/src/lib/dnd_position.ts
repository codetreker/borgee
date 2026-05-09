// dnd_position.ts — CHN-12.3 channel reorder 单调小数算法.
//
// CHN-12 设计 ②: 0 server prod — 算法跑 client 端, server PUT /me/layout
// 既有 batch upsert (CHN-3.2 #357 + CHN-3.3 #415 单调小数 acceptance §2.4).
//
// 数学锁 (跟 CHN-3.3 #415 acceptance §2.4 同源 byte-identical):
// - 中点策略保 REAL 单调性 — 永不撞 unique constraint (server 也无 unique
//   on position; REAL 浮点足够分辨力 v0).
// - prev=null 边界 → next-1.0 (= 最小已用 position 减 1.0).
// - next=null 边界 → prev+1.0 (= 最大已用 position 加 1.0).
// - 都 null (列表唯一行 fallback) → 1.0.
//
// 反约束 grep 锚: 此函数仅 channel reorder 用, 不被其它 milestone 复用 (
// hide / mute / pin 各走自己的 path).

/**
 * computeReorderPosition — 给两邻 position 算中点 (单调小数).
 *
 * @param prev 拖拽目标位置之前一行的 position (null 表 newIdx==0).
 * @param next 拖拽目标位置之后一行的 position (null 表 newIdx==len-1).
 * @returns 新行的 position REAL.
 */
export function computeReorderPosition(
  prev: number | null,
  next: number | null,
): number {
  if (prev === null && next === null) return 1.0;
  if (prev !== null && next === null) return prev + 1.0;
  if (prev === null && next !== null) return next - 1.0;
  return (prev! + next!) / 2.0;
}
