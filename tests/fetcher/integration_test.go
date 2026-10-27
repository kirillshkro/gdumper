// Integration tests for the fetcher client (testing + testify, TC-07).
package fetcher_test

import (
	"context"
	"net/http"
	"time"

	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/fetcher"
)

// TestIntegrationRetry500 — TC-07.1: 500→500→500→200 with retries.
func (s *FetcherTestSuite) TestIntegrationRetry500() {
	ms := newServer(map[string]route{
		"/page-1": {failStatus: http.StatusInternalServerError, failures: 3},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{
		Retry:     5,
		Timeout:   5 * time.Second,
		RetryBase: 10 * time.Millisecond,
	})
	resp, err := c.Fetch(context.Background(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Equal(4, ms.hitCount("/page-1"))
}

// TestIntegrationRetryExhausted — all attempts fail: 500 is returned, no panic.
func (s *FetcherTestSuite) TestIntegrationRetryExhausted() {
	ms := newServer(map[string]route{
		"/page-1": {failStatus: http.StatusInternalServerError, failures: 10},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{
		Retry:     3,
		Timeout:   5 * time.Second,
		RetryBase: 10 * time.Millisecond,
	})
	resp, err := c.Fetch(context.Background(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusInternalServerError, resp.StatusCode)
	s.True(resp.IsHTTPError())
	s.Equal(4, ms.hitCount("/page-1"))
}

// TestIntegration429RetryAfter — TC-07.2: 429 + Retry-After header honored.
func (s *FetcherTestSuite) TestIntegration429RetryAfter() {
	ms := newServer(map[string]route{
		"/page-1": {failStatus: http.StatusTooManyRequests, failures: 2, retryAfter: "2", body: []byte("ok")},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{
		Retry:     5,
		Timeout:   30 * time.Second,
		RetryBase: 1 * time.Millisecond,
	})
	start := time.Now()
	resp, err := c.Fetch(context.Background(), ms.URL+"/page-1")
	elapsed := time.Since(start)
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Equal([]byte("ok"), resp.Body)
	// The first retry must wait at least 2 seconds (Retry-After).
	s.GreaterOrEqual(elapsed, 2*time.Second)
}

// TestIntegration429Backoff — 429 without header: exponential backoff.
func (s *FetcherTestSuite) TestIntegration429Backoff() {
	ms := newServer(map[string]route{
		"/page-1": {failStatus: http.StatusTooManyRequests, failures: 3, body: []byte("ok")},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{
		Retry:     5,
		Timeout:   30 * time.Second,
		RetryBase: 50 * time.Millisecond,
	})
	start := time.Now()
	resp, err := c.Fetch(context.Background(), ms.URL+"/page-1")
	elapsed := time.Since(start)
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Equal(4, ms.hitCount("/page-1"))
	// Backoff 50ms + 100ms + 150ms + jitter (<= 6ms): >= ~260ms, <= 400ms.
	s.GreaterOrEqual(elapsed, 260*time.Millisecond)
	s.Less(elapsed, 500*time.Millisecond)
}

// TestIntegrationTimeout — TC-07.3: server delay > timeout: error after all
// attempts, status is not masked.
func (s *FetcherTestSuite) TestIntegrationTimeout() {
	ms := newServer(map[string]route{
		"/page-1": {delay: 2 * time.Second, status: http.StatusOK, body: []byte("ok")},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{
		Timeout:   500 * time.Millisecond,
		Retry:     2,
		RetryBase: 10 * time.Millisecond,
	})
	resp, err := c.Fetch(context.Background(), ms.URL+"/page-1")
	s.Nil(resp)
	s.Error(err)
	s.ErrorIs(err, context.DeadlineExceeded)
	s.Contains(err.Error(), "after 3 attempts")
}

// TestIntegrationReadError — connection reset mid-body: ErrReadBody is reported.
func (s *FetcherTestSuite) TestIntegrationReadError() {
	ms := newServer(map[string]route{
		"/page-1": {abortBody: true, body: []byte("short-body")},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{Retry: 0, Timeout: 5 * time.Second})
	resp, err := c.Fetch(context.Background(), ms.URL+"/page-1")
	s.NotNil(resp)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Error(err)
	s.ErrorIs(err, fetcher.ErrReadBody)
}

// TestIntegrationStatuses — 200/404/500 handling: status code is kept.
func (s *FetcherTestSuite) TestIntegrationStatuses() {
	for _, status := range []int{http.StatusOK, http.StatusNotFound, http.StatusInternalServerError} {
		ms := newServer(map[string]route{
			"/p": {status: status, failStatus: status, failures: 10},
		})
		defer ms.Close()
		c := s.newClient(config.FetchOptions{
			Retry:     2,
			Timeout:   5 * time.Second,
			RetryBase: 10 * time.Millisecond,
			Delay:     0,
		})
		resp, _ := c.Fetch(context.Background(), ms.URL+"/p")
		s.NotNil(resp)
		s.Equal(status, resp.StatusCode)
	}
}

// TestIntegrationHeaders — headers are set on every attempt.
func (s *FetcherTestSuite) TestIntegrationHeaders() {
	ms := newServer(map[string]route{
		"/page-1": {failStatus: http.StatusInternalServerError, failures: 2},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{
		Retry:     3,
		Timeout:   5 * time.Second,
		RetryBase: 10 * time.Millisecond,
		Headers:   map[string]string{"X-Api-Key": "secret"},
		UserAgent: "gdumper/1.0",
	})
	resp, err := c.Fetch(context.Background(), ms.URL+"/page-1")
	s.NotNil(resp)
	s.NoError(err)
	reqs := ms.rec.requests()
	s.GreaterOrEqual(len(reqs), 3)
	for _, r := range reqs {
		s.Equal("/page-1", r.path)
		s.Equal("secret", r.headers.Get("X-Api-Key"))
		s.Equal("gdumper/1.0", r.headers.Get("User-Agent"))
		s.Contains(r.headers.Get("Accept"), "text/html")
		s.Equal("en", r.headers.Get("Accept-Language"))
	}
}
