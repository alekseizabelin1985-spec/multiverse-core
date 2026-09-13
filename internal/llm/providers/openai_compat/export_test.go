package openaicompat

import (
	"context"
	"net"
	"net/http"
)

// WithDialContext replaces the dialer of the transport, so a test can send a
// request for a name nobody resolves (a service of compose, an address of the
// LAN) to its httptest server, and see that the proxy is not used on the way.
func WithDialContext(dial func(ctx context.Context, network, addr string) (net.Conn, error)) Option {
	return func(p *Provider) { p.transport.DialContext = dial }
}

// TransportOf exposes the transport of p.
func TransportOf(p *Provider) *http.Transport { return p.transport }
