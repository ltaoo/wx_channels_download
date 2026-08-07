package wxmpadapter

import (
	"testing"

	adapterpkg "wx_channel/internal/adapter"
)

func TestRegisteredPlatformUsesOfficialAccountAdapter(t *testing.T) {
	registered := adapterpkg.Get(PlatformID)
	if _, ok := registered.(*OfficialAccountAdapter); !ok {
		t.Fatalf("registered adapter type = %T, want *OfficialAccountAdapter", registered)
	}
}

func TestOfficialAccountAdapterRuntimeLifecycle(t *testing.T) {
	official_account_adapter := NewOfficialAccountAdapter()
	handle, err := official_account_adapter.RegisterRuntime(adapterpkg.RuntimeDeps{})
	if err != nil {
		t.Fatalf("register runtime: %v", err)
	}
	if handle != official_account_adapter {
		t.Fatalf("runtime handle = %T, want the OfficialAccountAdapter instance", handle)
	}
	if _, err := official_account_adapter.RegisterRuntime(adapterpkg.RuntimeDeps{}); err == nil {
		t.Fatal("second runtime registration succeeded")
	}

	official_account_adapter.Stop()
	if _, err := official_account_adapter.RegisterRuntime(adapterpkg.RuntimeDeps{}); err != nil {
		t.Fatalf("register runtime after Stop: %v", err)
	}
	official_account_adapter.Stop()
}
