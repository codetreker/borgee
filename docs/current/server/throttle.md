# server-go — internal/throttle (G2.3 节流不变量)

> G2.3 (#221) · PR #229 + #236 · 蓝图 `concept-model.md §4.1` (B.1)

## 1. 适用范围

`internal/throttle` package 给 offline-mention system message 提供 **per-(channel_id, agent_id)** 节流: 5 分钟窗口内同 key 只发一条系统提示. 不进 ws hub — G2.6 BPP schema lock = 节流是策略不是传输契约.

## 2. API

| Symbol | 行为 |
|--------|------|
| `ThrottleWindow = 5 * time.Minute` | 蓝图常量 (concept §4.1 B.1). REG-CHECK grep 反向锁定, **不准 inline literal** |
| `New(clock.Clock) *Throttle` | prod 传 `clock.NewReal()`, 测试传 `clock.NewFake()` (G2.3 拒收红线 #4: 真 sleep > 100ms = CI 慢闸) |
| `(*Throttle).Allow(channelID, agentID) bool` | true = 允许发送 + 记 last; false = 落入窗口被压 |

线程安全: `sync.Mutex` 守 `map[key]time.Time`.

## 3. 验证证据 (G2.3 ✅)

`internal/throttle/notification_throttle_test.go` T1-T5 全过:

- T1 同 key 5min 内第二次 Allow → false
- T2 5min 边界 `>=` 通过 (clock.Advance 5min 后第二次 → true)
- T3 不同 channel_id 各自独立 (二维 key 不串)
- T4 不同 agent_id 各自独立
- T5 并发 100 goroutine 同 key → 只一个 Allow=true

## 4. 不在范围

- 不持久化. 重启清空; v1 单实例 OK, 多实例需迁 Redis (data-layer.md row 75 已留位).
- 不 throttle 普通消息流, 仅 offline-mention system 提示.
