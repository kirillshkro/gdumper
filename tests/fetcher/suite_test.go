// Package fetcher_test — tests for the internal/fetcher package per
// specs-tests.md (TC-07, integration, E2E).
package fetcher_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/fetcher"

	"github.com/stretchr/testify/suite"
)

// FetcherTestSuite — the fetcher client test suite.
type FetcherTestSuite struct {
	suite.Suite
	cap *captureHandler
}

// TestFetcherSuite — runs the test suite.
func TestFetcherSuite(t *testing.T) { suite.Run(t, new(FetcherTestSuite)) }

// SetupTest — creates a fresh logger capture before each test.
func (s *FetcherTestSuite) SetupTest() { s.cap = &captureHandler{} }

// newClient — creates a fetcher client with the given options.
func (s *FetcherTestSuite) newClient(opts config.FetchOptions) *fetcher.Client {
	if opts.Logger == nil { opts.Logger = slog.New(s.cap) }
	return fetcher.New(opts)
}

// route — mock server behavior for a single path.
type route struct {
	status     int
	redirect   string
	body       []byte
	headers    map[string]string
	retryAfter string
	failStatus int
	failures   int
	delay      time.Duration
	block      bool
	abortBody  bool
}

// mockServer — mock HTTP server with request recording.
type mockServer struct {
	*httptest.Server
	routes map[string]route
	rec    *requestRecorder
	hits   map[string]int
	hitsMu sync.Mutex
}

// newServer — creates a mock server with per-path routes.
func newServer(routes map[string]route) *mockServer {
	ms := &mockServer{routes: routes, rec: &requestRecorder{}, hits: make(map[string]int)}
	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.rec.record(r.URL.Path, r.Header)
		n := ms.hit(r.URL.Path)
		rt := ms.routes[r.URL.Path]
		if rt.redirect != "" {
			http.Redirect(w, r, rt.redirect, http.StatusFound)
			return
		}
		if rt.block {
			<-r.Context().Done()
			return
		}
		select {
		case <-time.After(rt.delay):
		case <-r.Context().Done():
			return
		}
		if rt.abortBody {
			w.Header().Set("Connection", "close")
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write(rt.body)
			return
		}
		if rt.failures > 0 && n <= rt.failures {
			if rt.retryAfter != "" {
				w.Header().Set("Retry-After", rt.retryAfter)
			}
			http.Error(w, "", rt.failStatus)
			return
		}
		status := rt.status
		for k, v := range rt.headers {
			w.Header().Set(k, v)
		}
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		if rt.body != nil {
			_, _ = w.Write(rt.body)
		}
	}))
	return ms
}

// hit — increments and returns the request counter for a path.
func (ms *mockServer) hit(p string) int {
	ms.hitsMu.Lock(); defer ms.hitsMu.Unlock()
	ms.hits[p]++
	return ms.hits[p]
}

// hitCount — number of recorded requests to the path p.
func (ms *mockServer) hitCount(p string) int {
	ms.hitsMu.Lock(); defer ms.hitsMu.Unlock()
	return ms.hits[p]
}

// record — stores a request with its headers.
func (r *requestRecorder) record(p string, h http.Header) {
	r.mu.Lock(); defer r.mu.Unlock()
	r.reqs = append(r.reqs, recorded{p, h})
}

// count — the total number of recorded requests.
func (r *requestRecorder) count() int {
	r.mu.Lock(); defer r.mu.Unlock()
	return len(r.reqs)
}

// requests — all recorded requests.
func (r *requestRecorder) requests() []recorded {
	r.mu.Lock(); defer r.mu.Unlock()
	return r.reqs
}

// requestRecorder — records all requests with their headers.
type requestRecorder struct {
	mu   sync.Mutex
	reqs []recorded
}

// recorded — a single recorded request.
type recorded struct {
	path    string
	headers http.Header
}

// captureHandler — captures slog messages for assertions.
type captureHandler struct {
	mu   sync.Mutex
	msgs []string
}

// Enabled — reports whether the handler handles records at the given level.
func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

// WithAttrs — returns the same handler (attrs are not needed).
func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

// WithGroup — returns the same handler (groups are not needed).
func (h *captureHandler) WithGroup(_ string) slog.Handler { return h }

// Handle — stores the record message.
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock(); defer h.mu.Unlock()
	h.msgs = append(h.msgs, r.Message)
	return nil
}

// contains — reports whether a stored message contains a substring.
func (h *captureHandler) contains(substr string) bool {
	h.mu.Lock(); defer h.mu.Unlock()
	for _, m := range h.msgs {
		if strings.Contains(m, substr) { return true }
	}
	return false
}
