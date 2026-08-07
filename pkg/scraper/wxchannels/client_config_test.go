package wxchannels

import (
	"fmt"
	"sync"
	"testing"

	"wx_channel/pkg/configapi"
)

func TestChannelsClientAppliesRuntimeConfig(t *testing.T) {
	store := configapi.NewStore()
	if err := store.Publish(channels_test_config(5, "cookie-1")); err != nil {
		t.Fatalf("publish initial config: %v", err)
	}
	client := NewChannelsClient(store)
	defer client.Stop()

	initial := client.current_config()
	if initial.RefreshInterval != 5 || initial.YuanbaoCookie != "cookie-1" {
		t.Fatalf("initial config = %+v", initial)
	}

	_, changed := client.runtime_config_state()
	if err := store.Publish(channels_test_config(10, "cookie-2")); err != nil {
		t.Fatalf("publish updated config: %v", err)
	}
	select {
	case <-changed:
	default:
		t.Fatal("runtime config change was not broadcast")
	}

	updated := client.current_config()
	if updated.RefreshInterval != 10 || updated.YuanbaoCookie != "cookie-2" {
		t.Fatalf("updated config = %+v", updated)
	}
}

func TestChannelsClientKeepsLastValidRuntimeConfig(t *testing.T) {
	store := configapi.NewStore()
	if err := store.Publish(channels_test_config(5, "cookie-1")); err != nil {
		t.Fatalf("publish initial config: %v", err)
	}
	client := NewChannelsClient(store)
	defer client.Stop()

	if err := store.Publish(channels_test_config(-1, "cookie-2")); err != nil {
		t.Fatalf("publish invalid config: %v", err)
	}
	current := client.current_config()
	if current.RefreshInterval != 5 || current.YuanbaoCookie != "cookie-1" {
		t.Fatalf("config changed after invalid update: %+v", current)
	}
}

func TestChannelsClientRuntimeConfigConcurrentRead(t *testing.T) {
	store := configapi.NewStore()
	if err := store.Publish(channels_test_config(1, "cookie-0")); err != nil {
		t.Fatalf("publish initial config: %v", err)
	}
	client := NewChannelsClient(store)
	defer client.Stop()

	var wait_group sync.WaitGroup
	for reader := 0; reader < 4; reader++ {
		wait_group.Add(1)
		go func() {
			defer wait_group.Done()
			for i := 0; i < 100; i++ {
				_ = client.current_config()
			}
		}()
	}
	for i := 1; i <= 20; i++ {
		if err := store.Publish(channels_test_config(i, fmt.Sprintf("cookie-%d", i))); err != nil {
			t.Fatalf("publish config %d: %v", i, err)
		}
	}
	wait_group.Wait()

	current := client.current_config()
	if current.RefreshInterval != 20 || current.YuanbaoCookie != "cookie-20" {
		t.Fatalf("final config = %+v", current)
	}
}

func channels_test_config(refresh_interval int, cookie string) map[string]any {
	return map[string]any{
		"channels": map[string]any{
			"refreshInterval": refresh_interval,
		},
		"cloudflare": map[string]any{
			"sphCookie": cookie,
		},
	}
}
