// Package netx provides network helpers shared by the fetcher and the
// downloader.
//
// It knows only how to build an http.Transport, optionally routing every
// connection through a SOCKS5 proxy specified by a --proxy URL of the form
// socks5://<host>:<port> (or socks5h://).
package netx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/proxy"
)

// supportedProxies — the URL schemes that map to a SOCKS5 proxy.
//
// Both socks5 and socks5h are handled by proxy.FromURL via its SOCKS5 dialer;
// the only difference between the two is DNS resolution (host vs. proxy),
// which the SOCKS5 protocol resolves on the proxy side regardless.
var supportedProxies = map[string]bool{
	"socks5":  true,
	"socks5h": true,
}

// NewTransport builds an http.Transport. When proxyURL is empty the returned
// transport connects directly (the zero transport). When proxyURL is non-empty
// it is parsed and used as a SOCKS5 proxy: every connection made through the
// transport is tunneled through it via its DialContext.
//
// A non-nil error means the proxy URL is malformed; the caller must not use
// the returned transport in that case.
func NewTransport(proxyURL string) (*http.Transport, error) {
	if proxyURL == "" {
		return &http.Transport{}, nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("netx: parse proxy url %q: %w", proxyURL, err)
	}
	if !supportedProxies[u.Scheme] {
		return nil, fmt.Errorf(
			"netx: unsupported proxy scheme %q in %q (want socks5:// or socks5h://)",
			u.Scheme, proxyURL)
	}
	dialer, err := proxy.FromURL(u, nil)
	if err != nil {
		return nil, fmt.Errorf("netx: build socks5 proxy %q: %w", proxyURL, err)
	}
	// Adapt the proxy dialer to the function signature expected by
	// http.Transport.DialContext. The SOCKS5 dialer returned by FromURL
	// implements proxy.ContextDialer, which honors ctx (cancellation and
	// deadline); prefer it so a hung SOCKS5 handshake can be cancelled.
	// For any other dialer run Dial in a goroutine and select on ctx.Done,
	// mirroring net/http's own context handling.
	dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			return cd.DialContext(ctx, network, address)
		}
		var (
			conn net.Conn
			done = make(chan struct{}, 1)
			err  error
		)
		go func() {
			conn, err = dialer.Dial(network, address)
			close(done)
			if conn != nil && ctx.Err() != nil {
				conn.Close()
			}
		}()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-done:
		}
		return conn, err
	}
	return &http.Transport{DialContext: dialContext}, nil
}