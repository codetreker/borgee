// DL-1 — EventBus interface (蓝图 §4 B 第 3 条).
//
// 设计 ① (DL-1 spec §0): Publish / Subscribe byte-identical 跟蓝图.
// v1 实现 InProcessEventBus 走 in-process map + buffered chan (跟 ws hub
// 同精神 in-process pub-sub) byte-identical 不破.
//
// 切换路径 (留 v3+, DL-3 阈值哨触发):
//   - InProcessEventBus (v1)
//   - NATSEventBus     → NATS jetstream (留 DL-3 阈值哨触发)
//   - RedisEventBus    → Redis pub-sub (alt path)
package datalayer

import "context"

// Event is the canonical pub-sub envelope.
// Topic 跟 ws frame type 同源 (e.g. "artifact_committed", "channel_created").
type Event struct {
	Topic   string
	Payload []byte
}

// EventBus is the SSOT interface for in-process pub-sub.
type EventBus interface {
	// Publish a single event under topic. Buffered (best-effort, RT-1.3 cursor
	// replay 兜底, 跟 BPP-4 dead_letter 设计沿用).
	Publish(ctx context.Context, topic string, payload []byte) error

	// Subscribe returns a buffered channel for events on topic. ctx cancel
	// closes the chan and unsubscribes. Multiple subscribers fan-out per
	// in-process map.
	Subscribe(ctx context.Context, topic string) (<-chan Event, error)
}
