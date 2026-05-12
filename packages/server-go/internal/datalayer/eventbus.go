// DL-1 — EventBus interface (blueprint §4 B item 3).
//
// Principle ① (DL-1 spec §0): Publish / Subscribe preserve the exact
// blueprint. The v1 InProcessEventBus uses an in-process map + buffered channel,
// matching the ws hub in-process pub-sub pattern without changing behavior.
//
// Implementation swap path (v3+, triggered by DL-3 threshold monitor):
//   - InProcessEventBus (v1)
//   - NATSEventBus     → NATS jetstream (DL-3 threshold trigger)
//   - RedisEventBus    → Redis pub-sub (alt path)
package datalayer

import "context"

// Event is the canonical pub-sub envelope.
// Topic shares naming with ws frame types (e.g. "artifact_committed", "channel_created").
type Event struct {
	Topic   string
	Payload []byte
}

// EventBus is the canonical interface for in-process pub-sub.
type EventBus interface {
	// Publish a single event under topic. Buffered best-effort delivery uses
	// RT-1.3 cursor replay as fallback and follows the BPP-4 dead_letter policy.
	Publish(ctx context.Context, topic string, payload []byte) error

	// Subscribe returns a buffered channel for events on topic. ctx cancel
	// closes the chan and unsubscribes. Multiple subscribers fan-out per
	// in-process map.
	Subscribe(ctx context.Context, topic string) (<-chan Event, error)
}
