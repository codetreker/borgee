package api

import (
	"testing"
	"time"

	"borgee-server/internal/datalayer"
)

// helper_enrollments_lifecycle_test.go locks the server-side derivation of
// `status` + `fresh` from `LastSeenAt` freshness. This is the server-visible
// half of the #968 "machine comes back across reboot/crash without local
// user login" chain: as soon as the helper daemon reconnects post-restart
// and posts a fresh heartbeat, the enrollment must flip back to `connected`.
// A real reboot/crash e2e is not feasible in CI sandbox, so we lock the
// derivation rule mechanically instead.

const helperEnrollmentStaleThresholdMillis = int64(5 * time.Minute / time.Millisecond)

func ptrStr(s string) *string { return &s }
func ptrInt64(v int64) *int64 { return &v }

func fixedNowHandler(nowMs int64) *HelperEnrollmentHandler {
	return &HelperEnrollmentHandler{
		Now: func() time.Time { return time.UnixMilli(nowMs) },
	}
}

func TestHelperEnrollmentStatus_StaleAtBoundary(t *testing.T) {
	t.Parallel()
	const nowMs = int64(1_800_000_000_000) // arbitrary fixed clock
	claimed := nowMs - 24*60*60*1000
	h := fixedNowHandler(nowMs)

	cases := []struct {
		name        string
		lastSeenMs  int64
		wantStatus  string
		wantFresh   bool
	}{
		{
			name:       "exactly_at_threshold_is_still_connected",
			lastSeenMs: nowMs - helperEnrollmentStaleThresholdMillis,
			wantStatus: "connected",
			wantFresh:  true,
		},
		{
			name:       "one_ms_past_threshold_is_offline",
			lastSeenMs: nowMs - helperEnrollmentStaleThresholdMillis - 1,
			wantStatus: "offline",
			wantFresh:  false,
		},
		{
			name:       "one_ms_before_threshold_is_connected",
			lastSeenMs: nowMs - helperEnrollmentStaleThresholdMillis + 1,
			wantStatus: "connected",
			wantFresh:  true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			row := &datalayer.HelperEnrollment{
				ID:             "enr-boundary",
				HostLabel:      "Reboot Box",
				HelperDeviceID: ptrStr("dev-1"),
				Status:         "connected", // server-stored state pre-derivation
				LastSeenAt:     ptrInt64(tc.lastSeenMs),
				ClaimedAt:      ptrInt64(claimed),
				CreatedAt:      claimed,
			}
			out := h.serialize(row)
			if got := out["status"]; got != tc.wantStatus {
				t.Fatalf("status=%v, want %v", got, tc.wantStatus)
			}
			if got := out["fresh"]; got != tc.wantFresh {
				t.Fatalf("fresh=%v, want %v", got, tc.wantFresh)
			}
		})
	}
}

func TestHelperEnrollmentStatus_FreshAfterStale(t *testing.T) {
	t.Parallel()
	const nowMs = int64(1_800_000_000_000)
	claimed := nowMs - 24*60*60*1000
	h := fixedNowHandler(nowMs)

	// Simulate: helper went silent across a reboot/crash window.
	staleLastSeen := nowMs - 30*60*1000 // 30 minutes ago = well past the 5min threshold
	row := &datalayer.HelperEnrollment{
		ID:             "enr-reconnect",
		HostLabel:      "Reboot Box",
		HelperDeviceID: ptrStr("dev-2"),
		Status:         "connected",
		LastSeenAt:     ptrInt64(staleLastSeen),
		ClaimedAt:      ptrInt64(claimed),
		CreatedAt:      claimed,
	}
	staleOut := h.serialize(row)
	if staleOut["status"] != "offline" {
		t.Fatalf("stale enrollment status=%v, want offline", staleOut["status"])
	}
	if staleOut["fresh"] != false {
		t.Fatalf("stale enrollment fresh=%v, want false", staleOut["fresh"])
	}

	// Helper daemon comes back after reboot and posts a fresh heartbeat:
	// advance last_seen_at to "now" and re-serialize. Server view must
	// flip back to `connected` immediately without any other state change.
	row.LastSeenAt = ptrInt64(nowMs)
	freshOut := h.serialize(row)
	if freshOut["status"] != "connected" {
		t.Fatalf("reconnected enrollment status=%v, want connected", freshOut["status"])
	}
	if freshOut["fresh"] != true {
		t.Fatalf("reconnected enrollment fresh=%v, want true", freshOut["fresh"])
	}
}

func TestHelperEnrollmentStatus_RevokedTakesPrecedence(t *testing.T) {
	t.Parallel()
	const nowMs = int64(1_800_000_000_000)
	claimed := nowMs - 24*60*60*1000
	h := fixedNowHandler(nowMs)

	// Revoked enrollment with a fresh heartbeat must stay revoked — a
	// freshly-restarted helper that happens to still hold a valid credential
	// cannot promote a revoked row back to connected. The derivation rule
	// only rewrites Status when row.Status is "connected" or "offline".
	row := &datalayer.HelperEnrollment{
		ID:             "enr-revoked",
		HostLabel:      "Revoked Box",
		HelperDeviceID: ptrStr("dev-3"),
		Status:         "revoked",
		LastSeenAt:     ptrInt64(nowMs),
		ClaimedAt:      ptrInt64(claimed),
		RevokedAt:      ptrInt64(nowMs - 60*1000),
		CreatedAt:      claimed,
	}
	out := h.serialize(row)
	if out["status"] != "revoked" {
		t.Fatalf("revoked enrollment must stay revoked even with fresh last_seen, got %v", out["status"])
	}
	if out["fresh"] != false {
		t.Fatalf("revoked enrollment fresh=%v, want false (revoked rows never expose fresh=true)", out["fresh"])
	}
}

func TestHelperEnrollmentStatus_UninstalledTakesPrecedence(t *testing.T) {
	t.Parallel()
	const nowMs = int64(1_800_000_000_000)
	claimed := nowMs - 24*60*60*1000
	h := fixedNowHandler(nowMs)

	// Same precedence rule for `uninstalled`: a fresh heartbeat from a
	// half-restarted helper that has not yet noticed the uninstall cannot
	// resurrect the row.
	row := &datalayer.HelperEnrollment{
		ID:             "enr-uninstalled",
		HostLabel:      "Uninstalled Box",
		HelperDeviceID: ptrStr("dev-4"),
		Status:         "uninstalled",
		LastSeenAt:     ptrInt64(nowMs),
		ClaimedAt:      ptrInt64(claimed),
		UninstalledAt:  ptrInt64(nowMs - 60*1000),
		CreatedAt:      claimed,
	}
	out := h.serialize(row)
	if out["status"] != "uninstalled" {
		t.Fatalf("uninstalled enrollment must stay uninstalled even with fresh last_seen, got %v", out["status"])
	}
}
