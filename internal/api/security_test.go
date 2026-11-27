package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"wx_channel/internal/config"
	"wx_channel/internal/database/model"
)

func TestAPIRequestTrustAndRemoteAuthentication(t *testing.T) {
	for _, test_case := range []struct {
		name, origin, host, remote, username, password, fetch_site, fetch_mode string
		config_username, config_password                                       string
		configured                                                             bool
		status                                                                 int
	}{
		{name: "local client", status: 200},
		{name: "same origin", origin: "http://127.0.0.1:2022", status: 200},
		{name: "injected channels client", origin: "https://channels.weixin.qq.com", status: 200},
		{name: "injected channels liteapp client", origin: "https://liteapp.weixin.qq.com", status: 200},
		{name: "injected zhihu client", origin: "https://www.zhihu.com", status: 200},
		{name: "injected zhihu article client", origin: "https://zhuanlan.zhihu.com", status: 200},
		{name: "any origin allowed (hostile)", origin: "https://audit.invalid", status: 200},
		{name: "any origin allowed (null)", origin: "null", status: 200},
		{name: "cross-site without origin", fetch_site: "cross-site", status: 200},
		{name: "injected xweb cors without origin", fetch_site: "cross-site", fetch_mode: "cors", status: 200},
		{name: "cross-site script load without origin", fetch_site: "cross-site", fetch_mode: "no-cors", status: 200},
		{name: "cross-site navigation without origin", fetch_site: "cross-site", fetch_mode: "navigate", status: 200},
		{name: "DNS rebinding", host: "audit.invalid:2022", origin: "http://audit.invalid:2022", configured: true, status: 401},
		{name: "remote without configured credentials", remote: "192.0.2.10:40000", status: 200},
		{name: "remote same origin without configured credentials", remote: "192.0.2.10:40000", host: "downloads.example.com", origin: "http://downloads.example.com", status: 200},
		{name: "only username configured", remote: "192.0.2.10:40000", config_username: "owner", status: 200},
		{name: "only password configured", remote: "192.0.2.10:40000", config_password: "secret", status: 200},
		{name: "authentication disabled ignores supplied credentials", remote: "192.0.2.10:40000", username: "wrong", password: "wrong", status: 200},
		{name: "remote without auth", remote: "192.0.2.10:40000", configured: true, status: 401},
		{name: "remote wrong password", remote: "192.0.2.10:40000", username: "owner", password: "wrong", configured: true, status: 401},
		{name: "remote authenticated", remote: "192.0.2.10:40000", username: "owner", password: "secret", configured: true, status: 200},
		{name: "loopback proxy cannot bypass configured auth", configured: true, status: 401},
		{name: "local authenticated", username: "owner", password: "secret", configured: true, status: 200},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			cfg := &APIConfig{Hostname: "0.0.0.0", Username: test_case.config_username, Password: test_case.config_password}
			if test_case.configured {
				cfg.Username, cfg.Password = "owner", "secret"
			}
			handler := with_cors(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }), cfg)
			request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:2022/api/file", nil)
			request.RemoteAddr = "127.0.0.1:40000"
			if test_case.remote != "" {
				request.RemoteAddr = test_case.remote
			}
			if test_case.host != "" {
				request.Host = test_case.host
			}
			request.Header.Set("Origin", test_case.origin)
			request.Header.Set("Sec-Fetch-Site", test_case.fetch_site)
			request.Header.Set("Sec-Fetch-Mode", test_case.fetch_mode)
			if test_case.username != "" {
				request.SetBasicAuth(test_case.username, test_case.password)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test_case.status {
				t.Fatalf("status=%d, want %d: %s", response.Code, test_case.status, response.Body.String())
			}
			if response.Code == 403 && response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("blocked origin has CORS permission")
			}
		})
	}
}

func TestFileBrowserServesSandboxedDocument(t *testing.T) {
	secret_path := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(secret_path, []byte("private bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	client := &APIClient{cfg: &APIConfig{Hostname: "127.0.0.1", DownloadDir: t.TempDir()}}
	router := gin.New()
	router.GET("/api/file", client.handle_fetch_file)
	handler := with_cors(router, client.cfg)
	// Origin restrictions are disabled by design. The response is still served
	// as a sandboxed document so an HTML file cannot script the API.
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:2022/api/file?path="+url.QueryEscape(secret_path), nil)
	request.RemoteAddr = "127.0.0.1:40000"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 || response.Header().Get("Content-Security-Policy") != "sandbox" {
		t.Fatalf("file browser cannot read a sandboxed file: %d", response.Code)
	}
}

func TestHTTPSProxyAuthentication(t *testing.T) {
	cfg := &APIConfig{Hostname: "127.0.0.1", Username: "owner", Password: "secret"}
	handler := with_cors(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }), cfg)
	for _, test_case := range []struct {
		origin        string
		authenticated bool
		status        int
	}{
		{"https://downloads.example.com", true, 200},
		{"https://downloads.example.com", false, 401},
		{"https://audit.invalid", true, 200},
	} {
		request := httptest.NewRequest(http.MethodPost, "http://downloads.example.com/api/config", nil)
		request.RemoteAddr = "127.0.0.1:40000"
		request.Header.Set("X-Forwarded-Proto", "https")
		request.Header.Set("Origin", test_case.origin)
		if test_case.authenticated {
			request.SetBasicAuth("owner", "secret")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test_case.status {
			t.Fatalf("origin=%s status=%d want=%d", test_case.origin, response.Code, test_case.status)
		}
	}
}

func TestImageProxyRejectsActiveContent(t *testing.T) {
	for _, test_case := range []struct {
		name, content_type, body string
		status                   int
	}{
		{"html", "text/html", "<script>fetch('/api/file')</script>", 502},
		{"mislabeled html", "image/png", "<script>alert(1)</script>", 502},
		{"svg", "image/svg+xml", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`, 502},
		{"png", "text/html", "\x89PNG\r\n\x1a\nimage data", 200},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", test_case.content_type)
				_, _ = w.Write([]byte(test_case.body))
			}))
			defer upstream.Close()
			logger := zerolog.Nop()
			client := &APIClient{logger: &logger}
			router := gin.New()
			router.GET("/imgproxy", client.handle_img_proxy)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/imgproxy?url="+url.QueryEscape(upstream.URL), nil))
			if response.Code != test_case.status {
				t.Fatalf("status=%d: %s", response.Code, response.Body.String())
			}
			if test_case.status == 200 && (response.Header().Get("Content-Type") != "image/png" || response.Header().Get("X-Content-Type-Options") != "nosniff") {
				t.Fatal("unsafe image response headers")
			}
		})
	}
}

func TestSyncUsesServerSideAuthenticationWithoutExposingCredentials(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "owner" || password != "s:e@cret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = io.WriteString(w, `{"code":0,"data":{"version":"v2"}}`)
		case "/api/v1/sync/info":
			_, _ = io.WriteString(w, `{"code":0,"data":{"version":3,"receive_enabled":true}}`)
		case "/api/v1/sync/import":
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = io.WriteString(w, `{"code":0,"data":{"tasks":1,"target_task_ids":[42]}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	target.User = url.UserPassword("owner", "s:e@cret")
	previous := viper.Get("sync.servers")
	t.Cleanup(func() { viper.Set("sync.servers", previous) })
	viper.Set("sync.servers", []map[string]string{{"name": "authenticated target", "url": target.String()}})
	client := new_sync_test_client(t, t.TempDir())
	if err := client.db.Create(&model.DownloadTask{Id: 1, Name: "authenticated sync", Timestamps: model.Timestamps{CreatedAt: 1}}).Error; err != nil {
		t.Fatal(err)
	}
	client.cfg.Original = &config.Config{}
	router := gin.New()
	router.GET("/servers", client.handle_sync_servers)
	router.GET("/servers/:server_id/status", client.handle_sync_server_status)
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/servers", nil))
	if bytes.Contains(list.Body.Bytes(), []byte("owner")) || bytes.Contains(list.Body.Bytes(), []byte("cret")) {
		t.Fatalf("server list exposes credentials: %s", list.Body.String())
	}
	status := httptest.NewRecorder()
	router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/servers/0/status", nil))
	if !bytes.Contains(status.Body.Bytes(), []byte(`"version":"v2"`)) {
		t.Fatalf("authenticated status lookup failed: %s", status.Body.String())
	}
	server := sync_server_config{Name: "authenticated target", URL: target.String()}
	if err := check_sync_receiver(context.Background(), server); err != nil {
		t.Fatal(err)
	}
	if _, err := client.send_sync_bundle(httptest.NewRequest(http.MethodPost, "/", nil), server, nil); err != nil {
		t.Fatal(err)
	}
}
