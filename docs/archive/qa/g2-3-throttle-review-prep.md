# G2.3 节流不变量单测 Review Prep — 5 分钟过审

> 飞马 · 2026-04-28 · 烈马 G2.3 节流单测 PR (≤100 LOC) 预备
> 引: blueprint `concept-model.md §4.1` (5 分钟节流) + `data-layer.md` 行 75 (`offline_mention_notifications`) + `phase-2-exit-gate.md` G2.3

## 1. 5 条盯点

| # | 盯点 | 看文件 | 通过条件 |
|---|------|--------|---------|
| T1 | 节流窗口 = 5 分钟 (per channel × per agent) | `internal/notify/throttle.go` (or 等价) | 常量 `ThrottleWindow = 5 * time.Minute` 字面; key = `(channel_id, agent_id)` 二元组 |
| T2 | 第 1 次 @ 发 system message | `internal/notify/throttle_test.go` | sub-test: 单次 @ → 1 条 system msg |
| T3 | 5 分钟内连续 @ → 仅 1 条 (B.1 不变量) | 同上 | sub-test: 同一 (channel, agent) 在窗口内 @5-6 次 → system msg count==1 |
| T4 | 跨窗口 @ → 重新发 | 同上 | sub-test: 第 1 次 @ → 推过 5 分钟 (mock clock) → 第 2 次 @ → system msg count==2 |
| T5 | 不同 channel / 不同 agent 互不影响 | 同上 | 二维 sub-test: (c1,a1) 触发后 (c2,a1) / (c1,a2) 仍发 (key 隔离) |

## 2. 接口契约

- 时钟用 `clock` 接口 (mockable), 不直接 `time.Now()` — 否则 T4 跨窗口测试用 sleep 5min 不可行
- 节流状态可 in-memory map (v0) 或 SQLite 表 `offline_mention_notifications` (data-layer.md 行 75); v0 推 in-memory + sync.Mutex, v1 表; **本 PR 任选**, 但选项需在注释固化
- 单测断言用直接 count, 不依赖 ws hub (节流逻辑独立于 push 通道)

## 3. 行为不变量 + LOC

- ≤ 100 LOC (烈马 budget) · 单元测试可不带 server 依赖 · 不动 schema (本 PR 无 migration); 若选 SQLite 路径则下个 PR 落 `v=11 offline_mention_notifications` migration · forward-only · 锁 G2.3

## 4. 拒收红线

❌ 用真 `time.Now()` 不 mock (T4 跑不了或 flaky 5 分钟) · ❌ 节流 key 单维 (只 channel 或只 agent) — 隔离 T5 失守 · ❌ 节流逻辑掺进 ws hub (耦合 G2.6 schema lock 风险) · ❌ 单测带 sleep > 100ms (CI 慢闸) · ❌ 5 分钟数字不写 const (魔法数 grep 不到, audit 失守)
