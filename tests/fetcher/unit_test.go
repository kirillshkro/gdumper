// Tests for the fetcher client, unit level (testing + testify, TC-07).
package fetcher_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/fetcher"
)

// TestUnitSuccess — 200: body, final URL, no redirects, fetch logged.
func (s *FetcherTestSuite) TestUnitSuccess() {
	ms := newServer(map[string]route{
		"/page-1": {body: []byte("<h1>Hi</h1>")},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{Retry: 0, Timeout: 5 * time.Second})
	resp, err := c.Fetch(s.T().Context(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.Equal([]byte("<h1>Hi</h1>"), resp.Body)
	s.Equal(ms.URL+"/page-1", resp.FinalURL)
	s.False(resp.Redirected)
	s.Len(resp.Redirects, 0)
	s.True(s.cap.contains("fetch"))
}

// TestUnitRedirect — 302: redirect chain is tracked.
func (s *FetcherTestSuite) TestUnitRedirect() {
	ms := newServer(map[string]route{
		"/a": {redirect: "/b"},
		"/b": {body: []byte("final")},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{Retry: 0, Timeout: 5 * time.Second})
	resp, err := c.Fetch(s.T().Context(), ms.URL+"/a")
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	s.True(resp.Redirected)
	s.Equal([]int{302}, resp.Redirects)
	s.Equal(ms.URL+"/b", resp.FinalURL)
	s.Equal([]byte("final"), resp.Body)
}

// TestUnitNotFoundNotRetried — 4xx is not retried: exactly one attempt.
func (s *FetcherTestSuite) TestUnitNotFoundNotRetried() {
	ms := newServer(map[string]route{
		"/page-1": {status: http.StatusNotFound},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{Retry: 5, Timeout: 5 * time.Second})
	resp, err := c.Fetch(s.T().Context(), ms.URL+"/page-1")
	s.NoError(err)
	s.Equal(http.StatusNotFound, resp.StatusCode)
	s.True(resp.IsHTTPError())
	s.Equal(1, ms.hitCount("/page-1"))
}

// TestUnitCanceled — already-canceled context: graceful shutdown, no retries.
func (s *FetcherTestSuite) TestUnitCanceled() {
	ms := newServer(map[string]route{
		"/page-1": {body: []byte("ok")},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{Retry: 3, Timeout: 5 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	resp, err := c.Fetch(ctx, ms.URL+"/page-1")
	elapsed := time.Since(start)
	s.Nil(resp)
	s.ErrorIs(err, context.Canceled)
	s.Less(int(elapsed/time.Millisecond), 50)
	s.LessOrEqual(ms.rec.count(), 1)
}

// TestUnitCanceledInFlight — canceled while the server is not ready.
func (s *FetcherTestSuite) TestUnitCanceledInFlight() {
	ms := newServer(map[string]route{
		"/page-1": {block: true},
	})
	defer ms.Close()
	c := s.newClient(config.FetchOptions{Retry: 3, Timeout: 10 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	// cancel() must fire while the in-flight request is still pending.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	resp, err := c.Fetch(ctx, ms.URL+"/page-1")
	elapsed := time.Since(start)
	s.Nil(resp)
	s.ErrorIs(err, context.Canceled)
	s.Less(int(elapsed/time.Millisecond), 500)
	s.Equal(1, ms.hitCount("/page-1"))
}

// TestUnitIsHTTPError — IsHTTPError: status code table.
func (s *FetcherTestSuite) TestUnitIsHTTPError() {
	t := s.T()
	for _, tt := range []struct {
		status int
		want   bool
	}{
		{200, false}, {204, false}, {302, false}, {399, false},
		{400, true}, {404, true}, {500, true}, {599, true},
	} {
		t.Run(fmt.Sprintf("status=%d", tt.status), func(t *testing.T) {
			r := &fetcher.Response{StatusCode: tt.status}
			s.True(r.IsHTTPError() == tt.want)
		})
	}
}

// TestUnitOptionsDefaults — NewOptions default values.
func (s *FetcherTestSuite) TestUnitOptionsDefaults() {
	opts := config.NewFetchOptions()
	s.Equal(30 * time.Second, opts.Timeout)
	s.Equal(500 * time.Millisecond, opts.Delay)
	s.Equal(3, opts.Retry)
	s.Equal(250 * time.Millisecond, opts.RetryBase)
	s.NotNil(opts.Logger)
}
