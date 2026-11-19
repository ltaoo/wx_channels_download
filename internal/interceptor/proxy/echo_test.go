//go:build !sunnynet
// +build !sunnynet

package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"wx_channel/pkg/certificate"
)

func TestRemoteServerRelaySupportsHTTPSAndWSS(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			_, _ = io.WriteString(w, r.URL.RequestURI())
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		message_type, message, err := conn.ReadMessage()
		if err == nil {
			_ = conn.WriteMessage(message_type, append([]byte(r.URL.RequestURI()+":"), message...))
		}
	}))
	defer backend.Close()
	cert, key, err := certificate.GenerateRootCA("relay-test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	relay, err := NewProxy(cert, key, "", false, "127.0.0.1", 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	backend_addr := backend.Listener.Addr().(*net.TCPAddr)
	relay.AddPlugin(&Plugin{
		Match:  "weixin110.qq.com",
		Target: &TargetConfig{Protocol: "http", Host: "127.0.0.1", Port: backend_addr.Port},
	})
	server := httptest.NewServer(http.HandlerFunc(relay.ServeHTTP))
	defer server.Close()
	proxy_url, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(cert) {
		t.Fatal("invalid relay test certificate")
	}
	tls_config := &tls.Config{RootCAs: roots}
	transport := &http.Transport{Proxy: http.ProxyURL(proxy_url), TLSClientConfig: tls_config}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	response, err := client.Get("https://weixin110.qq.com/api/v1/download_task/list?probe=remote")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || string(body) != "/api/v1/download_task/list?probe=remote" {
		t.Fatalf("HTTPS relay: status=%d body=%q error=%v", response.StatusCode, body, err)
	}
	dialer := websocket.Dialer{
		Proxy: http.ProxyURL(proxy_url), TLSClientConfig: tls_config, HandshakeTimeout: 3 * time.Second,
	}
	conn, _, err := dialer.Dial("wss://weixin110.qq.com/ws/v1/download_task?probe=remote", http.Header{
		"Origin": {"https://channels.weixin.qq.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil || string(message) != "/ws/v1/download_task?probe=remote:ping" {
		t.Fatalf("WSS relay: message=%q error=%v", message, err)
	}
}
