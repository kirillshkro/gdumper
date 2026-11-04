// Package netx_test — unit tests for netx.NewTransport, per the SOCKS5
// proxy work.
package netx_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kirillshkro/gdumper/internal/netx"
	"github.com/kirillshkro/gdumper/tests/socks5"

	"github.com/stretchr/testify/suite"
)

// NetxTestSuite — unit tests for the netx transport.
type NetxTestSuite struct {
	suite.Suite
}

// TestNetxSuite — runs the suite.
func TestNetxSuite(t *testing.T) {
	suite.Run(t, new(NetxTestSuite))
}

// TestTransportEmptyProxy — empty proxy → direct transport, no custom dialer.
func (s *NetxTestSuite) TestTransportEmptyProxy() {
	t, err := netx.NewTransport("")
	s.NoError(err, "empty proxy must not error")
	s.NotNil(t, "empty proxy must return a usable transport")
	s.Nil(t.DialContext, "empty proxy must not tunnel")
}

// TestTransportValidProxy — socks5/socks5h addresses → no error and
// the transport tunnels via a non-nil DialContext.
func (s *NetxTestSuite) TestTransportValidProxy() {
	for _, addr := range []string{
		"socks5://proxy.example.com:1080",
		"socks5://127.0.0.1:1080",
		"socks5h://proxy.example.com:1080",
		"socks5h://[::1]:1080",
	} {
		t, err := netx.NewTransport(addr)
		s.Require().NoError(err, "valid proxy %q", addr)
		s.Require().NotNil(t.DialContext, "proxy %q must tunnel via a ContextDialer", addr)
	}
}

// TestTransportInvalidURL — malformed addresses → error.
func (s *NetxTestSuite) TestTransportInvalidURL() {
	for _, addr := range []string{
		"socks5://host%zz", // invalid percent-encoding → parse error
		"socks5://%zz",     // invalid percent-encoding → parse error
	} {
		_, err := netx.NewTransport(addr)
		s.Require().Error(err, "malformed proxy %q", addr)
	}
}

// TestTransportUnsupportedScheme — non-socks5 schemes → error.
func (s *NetxTestSuite) TestTransportUnsupportedScheme() {
	for _, addr := range []string{
		"http://proxy.example.com:3128",
		"https://proxy.example.com:3128",
		"socks4://proxy.example.com:1080",
		"ftp://proxy.example.com:21",
		"not-a-url",
	} {
		_, err := netx.NewTransport(addr)
		s.Require().Error(err, "unsupported scheme %q", addr)
	}
}

// TestTransportRoutedThroughProxy — proves the SOCKS5 dialer is actually used:
// a GET through the netx transport is delivered to a local server only when
// the proxy tunnels it (the proxy must record the target).
func (s *NetxTestSuite) TestTransportRoutedThroughProxy() {
	// local target server
	var hits int64
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ms.Close()

	// in-process SOCKS5 proxy
	proxy, err := socks5.New(0)
	s.Require().NoError(err, "start socks5 server")
	defer proxy.Close()
	proxyURL := "socks5://" + proxy.Addr()

	t, err := netx.NewTransport(proxyURL)
	s.Require().NoError(err, "NewTransport with proxy")

	client := &http.Client{
		Transport: t,
		Timeout:   15 * time.Second,
	}

	targetURL, perr := url.Parse(ms.URL)
	s.Require().NoError(perr, "parse target url")
	req, err := http.NewRequest("GET", ms.URL, nil)
	s.Require().NoError(err, "new request")
	req.Header.Set("Connection", "close")
	resp, err := client.Do(req)
	s.Require().NoError(err, "GET via socks5 proxy")
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode, "response through proxy")
	s.True(atomic.LoadInt64(&hits) >= 1, "target was never hit")

	// The request must have gone through the proxy to reach the target.
	s.Require().Contains(proxy.Tunnels(), targetURL.Host, "proxy never tunneled the target %q", targetURL.Host)
}