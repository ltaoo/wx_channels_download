package interceptor

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestProxyStopClosesConnections(t *testing.T) {
	for _, connection_state := range []string{"new", "active", "hijacked"} {
		t.Run(connection_state, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			tracked_listener := new_proxy_listener(listener)
			defer tracked_listener.Close()
			accepted := make(chan struct{})
			started := make(chan error, 1)
			handler_done := make(chan struct{})
			server := &http.Server{
				ConnState: func(_ net.Conn, state http.ConnState) {
					if state == http.StateNew {
						close(accepted)
					}
				},
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer close(handler_done)
					if connection_state == "hijacked" {
						_, _, err := w.(http.Hijacker).Hijack()
						started <- err
						return
					}
					started <- nil
					<-r.Context().Done()
				}),
			}
			defer server.Close()
			serve_done := make(chan error, 1)
			go func() { serve_done <- server.Serve(tracked_listener) }()
			proxy_server := &InterceptorServer{
				Interceptor: NewInterceptor(&InterceptorConfig{}, nil, nil),
				server:      server,
				running:     true,
			}
			conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			select {
			case <-accepted:
			case <-time.After(time.Second):
				t.Fatal("proxy did not accept connection")
			}
			if connection_state != "new" {
				_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
				if _, err := fmt.Fprint(conn, "GET / HTTP/1.1\r\nHost: example.test\r\n\r\n"); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-started:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(time.Second):
					t.Fatal("proxy did not handle request")
				}
			}
			stop_done := make(chan error, 1)
			go func() { stop_done <- proxy_server.Stop() }()
			select {
			case err := <-stop_done:
				if err != nil {
					t.Fatalf("Stop: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("Stop waited for a long-lived proxy connection")
			}
			if proxy_server.running || proxy_server.server != nil {
				t.Fatal("proxy still marked as running")
			}
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			var buf [1]byte
			if _, err := conn.Read(buf[:]); err == nil {
				t.Fatal("connection is still open")
			} else if net_err, ok := err.(net.Error); ok && net_err.Timeout() {
				t.Fatal("connection was not closed by Stop")
			}
			if connection_state != "new" {
				select {
				case <-handler_done:
				case <-time.After(time.Second):
					t.Fatal("request handler did not exit")
				}
			}
			if err := <-serve_done; err != http.ErrServerClosed {
				t.Fatalf("Serve: %v", err)
			}
			if err := proxy_server.Stop(); err != nil {
				t.Fatalf("repeated Stop: %v", err)
			}
		})
	}
}

func TestProxyConnectionCloseRemovesTracking(t *testing.T) {
	listener := new_proxy_listener(nil)
	server_conn, client_conn := net.Pipe()
	defer client_conn.Close()
	conn := &proxy_conn{Conn: server_conn, listener: listener}
	listener.connections[conn] = struct{}{}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if len(listener.connections) != 0 {
		t.Fatal("closed connection retained by listener")
	}
}
