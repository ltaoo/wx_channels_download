package interceptor

import (
	"testing"

	"wx_channel/pkg/certificate"
	"wx_channel/pkg/configapi"
)

func publish_interceptor_config(t *testing.T, store *configapi.Store, hostname string, port int) {
	t.Helper()
	if err := store.Publish(map[string]any{
		"debug": map[string]any{"error": true, "echolog": false},
		"proxy": map[string]any{
			"hostname": hostname,
			"port":     port,
			"system":   true,
			"tcpRelay": map[string]any{
				"enabled":  true,
				"hostname": "127.0.0.1",
				"port":     9901,
			},
		},
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

func TestNewInterceptorSettingsUsesDeclaredSnapshots(t *testing.T) {
	store := configapi.NewStore()
	publish_interceptor_config(t, store, "127.0.0.2", 3031)

	settings, err := NewInterceptorSettings(store, configapi.Runtime{Version: "test-version"})
	if err != nil {
		t.Fatalf("new interceptor settings: %v", err)
	}
	if settings.ProxyServerHostname != "127.0.0.2" || settings.ProxyServerPort != 3031 {
		t.Fatalf("proxy endpoint = %s:%d", settings.ProxyServerHostname, settings.ProxyServerPort)
	}
	if !settings.ProxyTCPRelayEnabled || settings.ProxyTCPRelayPort != 9901 {
		t.Fatalf("TCP relay settings = enabled:%v port:%d", settings.ProxyTCPRelayEnabled, settings.ProxyTCPRelayPort)
	}
}

func TestInterceptorServerAppliesConfigWhileStoppedAndUnsubscribesOnClose(t *testing.T) {
	store := configapi.NewStore()
	publish_interceptor_config(t, store, "127.0.0.1", 3032)
	server, err := NewInterceptorServer(ServerDeps{
		ConfigProvider: store,
		Runtime:        configapi.Runtime{Version: "test-version"},
		CertificateLoader: func() *certificate.CertFileAndKeyFile {
			return certificate.DefaultCertFiles
		},
	})
	if err != nil {
		t.Fatalf("new interceptor server: %v", err)
	}

	publish_interceptor_config(t, store, "127.0.0.1", 3033)
	if server.Addr() != "127.0.0.1:3033" {
		t.Fatalf("address after publication = %q", server.Addr())
	}
	if err := server.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	publish_interceptor_config(t, store, "127.0.0.1", 3034)
	if server.Addr() != "127.0.0.1:3033" {
		t.Fatalf("address changed after close = %q", server.Addr())
	}
}
