package builtin

import (
	"testing"

	"wx_channel/internal/download/registry"
)

func TestDownloadAdaptersAreRegistered(t *testing.T) {
	for _, platformID := range []string{"69shuba", "bilibili", "douyin", "wxchannels", "wxmp"} {
		if registry.Get(platformID) == nil {
			t.Fatalf("adapter %q is not registered", platformID)
		}
	}
}

func TestRuntimeCapabilitiesAreRegistered(t *testing.T) {
	for _, platformID := range []string{"wxchannels", "wxmp"} {
		handler := registry.Get(platformID)
		if _, ok := handler.(registry.RuntimeAdapter); !ok {
			t.Errorf("adapter %q does not expose runtime registration", platformID)
		}
		if _, ok := handler.(registry.Postprocessor); !ok {
			t.Errorf("adapter %q does not expose postprocessing", platformID)
		}
	}
}
