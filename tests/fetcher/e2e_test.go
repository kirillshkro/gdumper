// E2E tests for the fetcher client: full runs on a mock server
// (TC-07.4 partial dump, TC-07.5 SIGINT/graceful shutdown, TC-07.6 delay).
package fetcher_test

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/fetcher"
)

// page — builds the URL of page n (1-based).
func page(n int) string {
	return "/page-" + strconv.Itoa(n)
}

// urls — builds the URL slice of pages a..b (both inclusive).
func urls(a, b int) []string {
	out := make([]string, 0, b-a+1)
	for i := a; i <= b; i++ {
		out = append(out, page(i))
	}
	return out
}

// crawl — fetches pages in sequence on the given context (with the base
// URL prepended to each page path); stops at the first transport-level
// error or non-OK status. Returns the URLs fetched successfully (i.e. the
// partial/full dump).
func (s *FetcherTestSuite) crawl(ctx context.Context, c *fetcher.Client, base string, urls []string) []string {
	var got []string
	for _, u := range urls {
		resp, err := c.Fetch(ctx, base+u)
		if err != nil {
			break
		}
		if resp.IsHTTPError() {
			break
		}
		got = append(got, u)
	}
	return got
}

// TestE2ESigInt — TC-07.5: SIGINT (context cancellation) during crawl:
// graceful shutdown, no panic, partial results preserved.
func (s *FetcherTestSuite) TestE2ESigInt() {
	routes := map[string]route{
		page(1): route{body: []byte("page")},
		page(2): route{body: []byte("page")},
		page(3): route{body: []byte("page")},
		// page-4 is slow, so it is in flight when SIGINT arrives.
		page(4): route{body: []byte("page"), delay: 2 * time.Second},
	}
	ms := newServer(routes)
	defer ms.Close()

	c := s.newClient(config.FetchOptions{
		Retry:   2,
		Timeout: 10 * time.Second,
		Delay:   10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	var got []string
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		got = s.crawl(ctx, c, ms.URL, urls(1, 4))
	}()

	// Wait until page-3 is done and page-4 is in flight, then simulate SIGINT.
	time.Sleep(200 * time.Millisecond)
	cancel()
	wg.Wait()
	elapsed := time.Since(start)

	// Graceful shutdown: the crawl returned quickly after cancellation.
	s.Less(int(elapsed/time.Millisecond), 500)
	// Partial dump: pages 1..3 were fetched, page-4 was interrupted.
	s.Equal([]string{page(1), page(2), page(3)}, got)
	s.GreaterOrEqual(ms.hitCount(page(3)), 1)
}

// TestE2EPartialDump — TC-07.4: connection drop on the 5th page: the first
// four pages are kept (partial dump), no panic.
func (s *FetcherTestSuite) TestE2EPartialDump() {
	routes := make(map[string]route)
	for i := 1; i <= 5; i++ {
		routes[page(i)] = route{body: []byte("page")}
	}
	// page-5 drops the connection mid-body (aborted body).
	routes[page(5)] = route{abortBody: true, body: []byte("partial")}

	ms := newServer(routes)
	defer ms.Close()

	c := s.newClient(config.FetchOptions{
		Retry:   0,
		Timeout: 5 * time.Second,
		Delay:   0,
	})

	got := s.crawl(context.Background(), c, ms.URL, urls(1, 5))

	// Partial dump: the first four pages are kept, the dropped one is not.
	s.Equal([]string{page(1), page(2), page(3), page(4)}, got)
	s.Equal(4, len(got))
}

// TestE2EDelay — TC-07.6: --delay=1s equivalent: the interval between
// consecutive requests is >= 1 second.
func (s *FetcherTestSuite) TestE2EDelay() {
	ms := newServer(map[string]route{
		page(1): {body: []byte("page")},
		page(2): {body: []byte("page")},
		page(3): {body: []byte("page")},
	})
	defer ms.Close()

	c := s.newClient(config.FetchOptions{
		Retry:   0,
		Timeout: 30 * time.Second,
		Delay:   time.Second, // --delay=1s
	})

	start := time.Now()
	got := s.crawl(context.Background(), c, ms.URL, urls(1, 3))
	elapsed := time.Since(start)

	s.Equal([]string{page(1), page(2), page(3)}, got)
	// 3 pages => 2 intervals between them, each >= 1s => elapsed >= ~2s.
	s.GreaterOrEqual(elapsed, 1900*time.Millisecond)
	s.Less(elapsed, 5*time.Second)
}

// TestE2EMixedScenarios — full run on a mock server with mixed statuses:
// successful pages are captured, the run stops at the first 4xx, no panic.
func (s *FetcherTestSuite) TestE2EMixedScenarios() {
	routes := map[string]route{
		page(1): route{body: []byte("ok")},
		page(2): route{body: []byte("ok")},
		page(3): route{status: http.StatusNotFound},
		page(4): route{body: []byte("unreachable")},
	}
	ms := newServer(routes)
	defer ms.Close()

	c := s.newClient(config.FetchOptions{
		Retry:   0,
		Timeout: 5 * time.Second,
		Delay:   0,
	})

	got := s.crawl(context.Background(), c, ms.URL, urls(1, 4))

	// The run stops at the 404 (page-3); page-4 is never requested.
	s.Equal([]string{page(1), page(2)}, got)
	s.Equal(1, ms.hitCount(page(3)))
	s.Equal(0, ms.hitCount(page(4)))
}

// TestE2ERetryThenContinue — a transient 500 is retried, then the crawl
// continues to the next page.
func (s *FetcherTestSuite) TestE2ERetryThenContinue() {
	routes := map[string]route{
		page(1): {failStatus: http.StatusInternalServerError, failures: 2, body: []byte("ok")},
		page(2): {body: []byte("ok")},
	}
	ms := newServer(routes)
	defer ms.Close()

	c := s.newClient(config.FetchOptions{
		Retry:     3,
		Timeout:   5 * time.Second,
		RetryBase: 10 * time.Millisecond,
		Delay:     0,
	})

	got := s.crawl(context.Background(), c, ms.URL, urls(1, 2))

	// page-1 was retried (3 hits), then page-2 followed.
	s.Equal([]string{page(1), page(2)}, got)
	s.Equal(3, ms.hitCount(page(1)))
	s.Equal(1, ms.hitCount(page(2)))
}
