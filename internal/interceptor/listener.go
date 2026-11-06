package interceptor

import (
	"net"
	"sync"
)

// proxy_listener owns every accepted connection, including connections hijacked
// for CONNECT or WebSocket tunnels and no longer tracked by net/http.
type proxy_listener struct {
	net.Listener
	mu          sync.Mutex
	connections map[*proxy_conn]struct{}
	closed      bool
}

type proxy_conn struct {
	net.Conn
	listener *proxy_listener
}

func new_proxy_listener(listener net.Listener) *proxy_listener {
	return &proxy_listener{
		Listener:    listener,
		connections: make(map[*proxy_conn]struct{}),
	}
}

func (l *proxy_listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	tracked_conn := &proxy_conn{Conn: conn, listener: l}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		_ = conn.Close()
		return nil, net.ErrClosed
	}
	l.connections[tracked_conn] = struct{}{}
	return tracked_conn, nil
}

func (l *proxy_listener) Close() error {
	err := l.Listener.Close()
	l.mu.Lock()
	l.closed = true
	connections := l.connections
	l.connections = nil
	l.mu.Unlock()
	for conn := range connections {
		_ = conn.Close()
	}
	return err
}

func (c *proxy_conn) Close() error {
	err := c.Conn.Close()
	c.listener.mu.Lock()
	delete(c.listener.connections, c)
	c.listener.mu.Unlock()
	return err
}
