package hermes

import (
	"context"
	"net"
	"time"
)

const (
	http_dial_timeout    = 5 * time.Second
	http_dial_keep_alive = 30 * time.Second
)

// HTTPDialContext builds the dialer shared by the built-in HTTP drivers. An
// empty family dials whatever address the resolver returns; "tcp4" restricts
// dialing to IPv4 so a request can bypass an IPv6 route that serves the wrong
// certificate or is otherwise unusable.
func HTTPDialContext(family string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: http_dial_timeout, KeepAlive: http_dial_keep_alive}
	if family == "" {
		return dialer.DialContext
	}
	return func(ctx context.Context, _, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, family, addr)
	}
}
