// Package fetcher_test — SOCKS5 proxy tests for the fetcher client.
//
// New applies the proxy to the underlying transport; malformed proxies fail
// with a netx parse/scheme error, valid ones construct successfully and route
// requests through the SOCKS5 proxy.
package fetcher_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/fetcher"
	"github.com/kirillshkro/gdumper/tests/socks5"
)

// TestProxyMalformedProxy — New rejects a proxy URL that is not a valid
// socks5/socks5h address (wrong scheme or unparseable).
func (s *FetcherTestSuite) TestProxyMalformedProxy() {
	for _, proxy := range []string{
		"http://proxy.example.com:3128",
		"https://proxy.example.com:3128",
		"socks4://proxy.example.com:1080",
		"not-a-url",
		"socks5://host%zz", // invalid percent-encoding
	} {
		opts := config.FetchOptions{Proxy: proxy}
		_, err := fetcher.New(opts)
		s.Require().Error(err, "malformed proxy %q", proxy)
		s.Contains(err.Error(), "proxy",
			"proxy %q should mention the proxy in its error", proxy)
	}
}

// TestProxyValidConstruct — New builds a usable client for a valid socks5 proxy.
func (s *FetcherTestSuite) TestProxyValidConstruct() {
	proxy, err := socks5.New(0)
	s.Require().NoError(err, "start socks5 server")
	defer proxy.Close()

	client := s.newClient(config.FetchOptions{
		Proxy: "socks5://" + proxy.Addr(),
	})
	s.NotNil(client, "New must return a usable client")
}

// TestProxyRouted — a request through a proxy'd fetcher is actually tunneled
// by the SOCKS5 proxy (the proxy records the target it dialed).
func (s *FetcherTestSuite) TestProxyRouted() {
	var hits int64
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ms.Close()

	proxy, err := socks5.New(0)
	s.Require().NoError(err, "start socks5 server")
	defer proxy.Close()

	client := s.newClient(config.FetchOptions{
		Proxy: "socks5://" + proxy.Addr(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	resp, err := client.Fetch(ctx, ms.URL)
	s.Require().NoError(err, "fetch via socks5 proxy")
	s.Equal(http.StatusOK, resp.StatusCode, "response status through proxy")
	s.True(atomic.LoadInt64(&hits) >= 1, "target was never hit")

	targetURL, perr := url.Parse(ms.URL)
	s.Require().NoError(perr, "parse target url")
	s.Require().Contains(proxy.Tunnels(), targetURL.Host,
		"proxy never tunneled the target %q", targetURL.Host)
}