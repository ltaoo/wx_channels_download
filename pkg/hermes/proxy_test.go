package hermes

import "testing"

func TestProxyServerURL(t *testing.T) {
	tests := []struct {
		name    string
		server  ProxyServer
		wantURL string
		wantErr bool
	}{
		{name: "empty"},
		{
			name:    "host and port default to HTTP",
			server:  ProxyServer{Address: "127.0.0.1:8080"},
			wantURL: "http://127.0.0.1:8080",
		},
		{
			name: "separate credentials",
			server: ProxyServer{
				Address:  "socks5://127.0.0.1:1080",
				Username: "download user",
				Password: "p@ss/word",
			},
			wantURL: "socks5://download%20user:p%40ss%2Fword@127.0.0.1:1080",
		},
		{
			name:    "explicit credentials override address",
			server:  ProxyServer{Address: "http://old:secret@127.0.0.1:8080", Username: "new", Password: "password"},
			wantURL: "http://new:password@127.0.0.1:8080",
		},
		{name: "invalid address", server: ProxyServer{Address: "http://"}, wantErr: true},
		{name: "unsupported scheme", server: ProxyServer{Address: "ftp://127.0.0.1:21"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.server.URL()
			if (err != nil) != tt.wantErr {
				t.Fatalf("URL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.wantURL {
				t.Fatalf("URL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}

func TestApplyTaskProxy(t *testing.T) {
	tests := []struct {
		name       string
		task       *TaskJob
		wantServer ProxyServer
	}{
		{
			name: "explicit task proxy",
			task: &TaskJob{
				ProxyServer: ProxyServer{Address: "  http://127.0.0.1:8080  ", Username: "user", Password: "password"},
				Config: map[string]any{
					TaskProxyServerConfigKey: map[string]any{"address": "http://ignored.example:8080"},
				},
			},
			wantServer: ProxyServer{Address: "http://127.0.0.1:8080", Username: "user", Password: "password"},
		},
		{
			name: "proxy from structured task config",
			task: &TaskJob{
				Config: map[string]any{
					TaskProxyServerConfigKey: map[string]any{
						"address":  "socks5://127.0.0.1:1080",
						"username": "user",
						"password": "password",
					},
				},
			},
			wantServer: ProxyServer{Address: "socks5://127.0.0.1:1080", Username: "user", Password: "password"},
		},
		{
			name: "legacy string config",
			task: &TaskJob{Config: map[string]any{
				legacyTaskProxyConfigKey: "http://127.0.0.1:8888",
			}},
			wantServer: ProxyServer{Address: "http://127.0.0.1:8888"},
		},
		{name: "no proxy", task: &TaskJob{Config: map[string]any{TaskProxyServerConfigKey: true}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.task.Resources = []ResourceJob{
				{Endpoints: []Endpoint{{ProxyServer: ProxyServer{Address: "http://old-proxy.example:8080"}}, {}}},
				{Endpoints: []Endpoint{{}}},
			}

			applyTaskProxy(tt.task)

			if tt.task.ProxyServer != tt.wantServer {
				t.Fatalf("task proxy = %+v, want %+v", tt.task.ProxyServer, tt.wantServer)
			}
			for resourceIndex, resource := range tt.task.Resources {
				for endpointIndex, endpoint := range resource.Endpoints {
					if endpoint.ProxyServer != tt.wantServer {
						t.Fatalf("resource %d endpoint %d proxy = %+v, want %+v", resourceIndex, endpointIndex, endpoint.ProxyServer, tt.wantServer)
					}
				}
			}
		})
	}
}

func TestTaskConfigForLogRedactsProxyCredentials(t *testing.T) {
	config := map[string]any{
		"filename": "video",
		TaskProxyServerConfigKey: map[string]any{
			"address":  "http://proxy.example:8080",
			"username": "user",
			"password": "secret",
		},
	}

	got := taskConfigForLog(config)
	if got["filename"] != "video" {
		t.Fatalf("non-secret config changed: %+v", got)
	}
	if got[TaskProxyServerConfigKey] != "<redacted>" {
		t.Fatalf("proxy config was not redacted: %+v", got)
	}
	if _, ok := config[TaskProxyServerConfigKey].(map[string]any); !ok {
		t.Fatalf("input config was mutated: %+v", config)
	}
}
