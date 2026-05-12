// Package server — bpp_5_reconnect_adapter_test.go: BPP-5 server-boot
// channelScopeAdapter (bpp.ChannelScopeResolver ↔ store.GetUserChannelIDs)
// bridge coverage. Adapter is the only store→bpp boundary at boot;
// without this test it stays 0% covered because cross-package bridge code is a
// typical cold path, matching the hubLivenessAdapter and pluginFrameRouterAdapter
// coverage pattern.
package server

import (
	"testing"

	"borgee-server/internal/store"
)

func TestBpp5reconnectAdapter_EmptyForUnknownUser(t *testing.T) {
	t.Parallel()
	s := store.MigratedStoreFromTemplate(t)

	adapter := &channelScopeAdapter{store: s}
	ids, err := adapter.ChannelIDsForOwner("nonexistent-user")
	if err != nil {
		t.Errorf("expected nil err for unknown user, got %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty channel ids for unknown user, got %v", ids)
	}
}
