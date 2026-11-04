// Package fetcher_test — cookie-auth tests for the fetcher client
// (auth module integration: jar loading, cookie sending, persistence,
// domain/expiry matching, log safety).
package fetcher_test

import (
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/kirillshkro/gdumper/internal/auth"
	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/fetcher"

	"github.com/stretchr/testify/suite"
)

// TestCookiesSuite — runs the cookie test suite.
func TestCookiesSuite(t *testing.T) { suite.Run(t, new(cookiesTestSuite)) }

// cookiesTestSuite — the cookie test suite; reuses the shared mock-server
// helpers (newServer, route, captureHandler, requestRecorder) declared in
// suite_test.go of the same package.
type cookiesTestSuite struct {
	suite.Suite
	cap *captureHandler
}

// SetupTest — creates a fresh logger capture before each test.
func (s *cookiesTestSuite) SetupTest() {
	s.cap = &captureHandler{}
}

// newClient — creates a fetcher client with the given options.
func (s *cookiesTestSuite) newClient(opts config.FetchOptions) *fetcher.Client {
	if opts.Logger == nil {
		opts.Logger = slog.New(s.cap)
	}
	c, err := fetcher.New(opts)
	if err != nil {
		s.T().Fatal("fetcher.New: ", err)
	}
	return c
}

// TestUnitRawCookiesSent — unit: raw cookies from the jar are sent in the
// request Cookie header (TC-08.1).
func (s *cookiesTestSuite) TestUnitRawCookiesSent() {
	ms := newServer(map[string]route{
		"/page-1": {body: []byte("ok")},
	})
	defer ms.Close()

	jar := auth.NewMemoryJar([]*http.Cookie{
		{Name: "session", Value: "abc123", Path: "/"},
		{Name: "theme", Value: "dark", Path: "/"},
	})
	c := s.newClient(config.FetchOptions{Jar: jar, Retry: 0, Timeout: 5 * time.Second, Delay: 0})

	resp, err := c.Fetch(s.T().Context(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	reqs := ms.rec.requests()
	s.Require().Len(reqs, 1)
	s.Contains(reqs[0].headers.Get("Cookie"), "session=abc123")
	s.Contains(reqs[0].headers.Get("Cookie"), "theme=dark")
}

// TestUnitExpiredCookiesNotSent — unit: expired cookies from the jar are
// not sent, valid ones still are (TC-08.2).
func (s *cookiesTestSuite) TestUnitExpiredCookiesNotSent() {
	ms := newServer(map[string]route{
		"/page-1": {body: []byte("ok")},
	})
	defer ms.Close()

	jar := auth.NewMemoryJar([]*http.Cookie{
		{Name: "expired", Value: "old", Path: "/", Expires: time.Now().Add(-time.Hour)},
		{Name: "live", Value: "ok", Path: "/"},
	})
	c := s.newClient(config.FetchOptions{Jar: jar, Retry: 0, Timeout: 5 * time.Second, Delay: 0})

	resp, err := c.Fetch(s.T().Context(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	reqs := ms.rec.requests()
	s.Require().Len(reqs, 1)
	s.Contains(reqs[0].headers.Get("Cookie"), "live=ok")
	s.NotContains(reqs[0].headers.Get("Cookie"), "expired=old")
}

// TestUnitNilJarNoCookies — unit: without a jar the behaviour is unchanged
// (no Cookie header), preserving backwards compatibility (TC-08.3).
func (s *cookiesTestSuite) TestUnitNilJarNoCookies() {
	ms := newServer(map[string]route{
		"/page-1": {body: []byte("ok")},
	})
	defer ms.Close()

	c := s.newClient(config.FetchOptions{Retry: 0, Timeout: 5 * time.Second, Delay: 0})
	resp, err := c.Fetch(s.T().Context(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	reqs := ms.rec.requests()
	s.Require().Len(reqs, 1)
	s.Empty(reqs[0].headers.Get("Cookie"))
}

// TestIntegrationDomainScopedMatching — integration: domain-scoped cookies
// are sent only to matching hosts, non-matching domains are omitted
// (TC-08.4).
func (s *cookiesTestSuite) TestIntegrationDomainScopedMatching() {
	ms := newServer(map[string]route{
		"/page-1": {body: []byte("ok")},
	})
	defer ms.Close()

	jar := auth.NewMemoryJar([]*http.Cookie{
		{Name: "match", Value: "1", Path: "/", Domain: "127.0.0.1"},
		{Name: "nomatch", Value: "2", Path: "/", Domain: "example.com"},
	})
	c := s.newClient(config.FetchOptions{Jar: jar, Retry: 0, Timeout: 5 * time.Second, Delay: 0})

	resp, err := c.Fetch(s.T().Context(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	reqs := ms.rec.requests()
	s.Require().Len(reqs, 1)
	s.Contains(reqs[0].headers.Get("Cookie"), "match=1")
	s.NotContains(reqs[0].headers.Get("Cookie"), "nomatch=2")

	u, err := url.Parse(ms.URL)
	s.NoError(err)
	s.Equal("127.0.0.1", u.Hostname())
}

// TestIntegrationSessionCookiesPersisted — integration: Set-Cookie from a
// response is persisted in the jar and sent with the next request (TC-08.5).
func (s *cookiesTestSuite) TestIntegrationSessionCookiesPersisted() {
	ms := newServer(map[string]route{
		"/page-1": {body: []byte("first"), headers: map[string]string{"Set-Cookie": "sess=v1; Path=/"}},
		"/page-2": {body: []byte("second")},
	})
	defer ms.Close()

	c := s.newClient(config.FetchOptions{
		Jar:     auth.NewMemoryJar(nil),
		Retry:   0,
		Timeout: 5 * time.Second,
		Delay:   0,
	})

	resp1, err := c.Fetch(s.T().Context(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusOK, resp1.StatusCode)

	resp2, err := c.Fetch(s.T().Context(), ms.URL+"/page-2")
	s.NoError(err)
	s.Equal(http.StatusOK, resp2.StatusCode)

	reqs := ms.rec.requests()
	s.Require().Len(reqs, 2)
	// The first request must not carry the session cookie (not set yet);
	// the second must carry it (persisted from the page-1 response).
	s.Empty(reqs[0].headers.Get("Cookie"))
	s.Contains(reqs[1].headers.Get("Cookie"), "sess=v1")
}

// TestIntegrationAuthModuleRawSource — integration: the full pipeline of
// config.ParseCookies -> auth.Load -> fetcher request: raw cookies source is
// attached and cookie values are never written to logs (TC-08.6).
func (s *cookiesTestSuite) TestIntegrationAuthModuleRawSource() {
	ms := newServer(map[string]route{
		"/page-1": {body: []byte("ok")},
	})
	defer ms.Close()

	source, err := config.ParseCookies(`raw:"auth_token=t0k3n; lang=en"`)
	s.Require().NoError(err, "ParseCookies")

	jar, err := auth.Load(source, slog.New(s.cap))
	s.Require().NoError(err, "auth.Load")

	c := s.newClient(config.FetchOptions{Jar: jar, Retry: 0, Timeout: 5 * time.Second, Delay: 0})
	resp, err := c.Fetch(s.T().Context(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	reqs := ms.rec.requests()
	s.Require().Len(reqs, 1)
	s.Contains(reqs[0].headers.Get("Cookie"), "auth_token=t0k3n")
	s.Contains(reqs[0].headers.Get("Cookie"), "lang=en")

	// Security: cookie values must not appear in the structured logs.
	s.Require().False(s.cap.contains("t0k3n"), "cookie values must not appear in logs")
}
