// Package bpp — heartbeat_decay.go: HB-3 v2.1 helper for deriving the three
// heartbeat decay buckets.
//
// Blueprint reference: docs/blueprint/current/plugin-protocol.md §1.6
// (disconnected is not a binary state).
// Spec brief: docs/implementation/modules/hb-3-v2-spec.md §0.1 + §1
// HB-3 v2.1.
//
// Stance (byte-identical with stance §1+§4):
//
//   - **0 schema changes** — DecayState is derived from last_heartbeat_at;
//     do not split a table or add another sequence. Reverse grep ensures
//     production code does not introduce a new table.
//   - **threshold is byte-identical with BPP-4** — StaleThreshold = 30 *
//     time.Second, matching srvbpp/BPP-7 SDK HeartbeatInterval.
//   - **enum literals have one source of truth** — DecayState const literals
//     (`fresh / stale / dead`) are locked; reverse grep hardcode outside
//     hb_3_v2*.go must return 0 hits.
//
// Negative constraints:
//   - nil-safe: DeriveDecayState(now, 0) returns dead (never live); negative
//     lastHeartbeatAt is treated as 0.
//   - No IO and no store dependency; this is a pure function.

package bpp

import "time"

// DecayState is the single source of truth for the three literals; values must
// remain byte-identical with spec §1 and acceptance §1 enum literals.
type DecayState string

const (
	// DecayStateFresh — last heartbeat ≤ StaleThreshold (plugin healthy).
	DecayStateFresh DecayState = "fresh"
	// DecayStateStale — StaleThreshold < last heartbeat ≤ DeadThreshold.
	DecayStateStale DecayState = "stale"
	// DecayStateDead — last heartbeat > DeadThreshold (plugin gone).
	DecayStateDead DecayState = "dead"
)

// StaleThreshold — same wall-clock value as BPP-4 #499 watchdog stale
// threshold (30s) and BPP-7 SDK HeartbeatInterval (30s). Any change must update
// three constants together: BPP-4 watchdog const, BPP-7 SDK const, and this
// const.
const StaleThreshold = 30 * time.Second

// DeadThreshold — fully-failed plugin. > DeadThreshold means the
// next bucket transition fires the BPP-8 RecordHeartbeatTimeout audit.
const DeadThreshold = 60 * time.Second

// DeriveDecayState — pure function. now and lastHeartbeatAt are Unix
// milliseconds. Negative or zero lastHeartbeatAt counts as "no heartbeat
// ever" → dead.
//
// Stance ①: derive in reverse from timestamps without table reads, store
// dependencies, or IO. Reverse-monotonic safe: now < lastHeartbeatAt returns
// fresh because the future-dated heartbeat is treated as healthy by being less
// than StaleThreshold old.
func DeriveDecayState(now, lastHeartbeatAt int64) DecayState {
	if lastHeartbeatAt <= 0 {
		return DecayStateDead
	}
	delta := now - lastHeartbeatAt
	if delta < 0 {
		// future-dated heartbeat — clamp to 0 for fresh.
		delta = 0
	}
	d := time.Duration(delta) * time.Millisecond
	switch {
	case d <= StaleThreshold:
		return DecayStateFresh
	case d <= DeadThreshold:
		return DecayStateStale
	default:
		return DecayStateDead
	}
}

// IsCrossBucketTransition returns true iff the two states are in
// different decay buckets — used by the watchdog wire (HB-3 v2.2) to
// decide whether to fire BPP-8 RecordHeartbeatTimeout audit. Same-bucket
// transitions are silently no-op (stance ⑦: do not repeatedly fire within the
// same bucket, which keeps high-frequency noise out of the audit log).
func IsCrossBucketTransition(from, to DecayState) bool {
	return from != to
}
