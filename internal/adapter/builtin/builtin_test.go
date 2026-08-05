package builtin

import (
	"testing"

	"wx_channel/internal/adapter"
)

func TestDownloadAdaptersAreRegistered(t *testing.T) {
	for _, platformID := range []string{"69shuba", "bilibili", "douyin", "wxchannels", "wxmp"} {
		if adapter.Get(platformID) == nil {
			t.Fatalf("adapter %q is not registered", platformID)
		}
	}
}

func TestRuntimeCapabilitiesAreRegistered(t *testing.T) {
	for _, platformID := range []string{"wxchannels", "wxmp"} {
		handler := adapter.Get(platformID)
		if _, ok := handler.(adapter.RuntimeAdapter); !ok {
			t.Errorf("adapter %q does not expose runtime registration", platformID)
		}
		if _, ok := handler.(adapter.Postprocessor); !ok {
			t.Errorf("adapter %q does not expose postprocessing", platformID)
		}
	}
}
