package wxchannelsadapter

import (
	"testing"

	adapterpkg "wx_channel/internal/adapter"
)

func TestRegisteredPlatformUsesChannelsAdapter(t *testing.T) {
	registered := adapterpkg.Get(PlatformID)
	if _, ok := registered.(*ChannelsAdapter); !ok {
		t.Fatalf("registered adapter type = %T, want *ChannelsAdapter", registered)
	}
}

func TestChannelsAdapterRuntimeLifecycle(t *testing.T) {
	channels_adapter := NewChannelsAdapter()
	handle, err := channels_adapter.RegisterRuntime(adapterpkg.RuntimeDeps{})
	if err != nil {
		t.Fatalf("register runtime: %v", err)
	}
	if handle != channels_adapter {
		t.Fatalf("runtime handle = %T, want the ChannelsAdapter instance", handle)
	}
	if _, err := channels_adapter.RegisterRuntime(adapterpkg.RuntimeDeps{}); err == nil {
		t.Fatal("second runtime registration succeeded")
	}

	channels_adapter.Stop()
	if _, err := channels_adapter.RegisterRuntime(adapterpkg.RuntimeDeps{}); err != nil {
		t.Fatalf("register runtime after Stop: %v", err)
	}
	channels_adapter.Stop()
}
