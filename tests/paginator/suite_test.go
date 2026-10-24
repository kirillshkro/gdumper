// Package paginator_test — unit and integration tests for internal/paginator (TC-03).
package paginator_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kirillshkro/gdumper/internal/fetcher"
	"github.com/kirillshkro/gdumper/internal/paginator"
	"github.com/kirillshkro/gdumper/internal/parser"

	"github.com/stretchr/testify/suite"
)

type PaginatorTestSuite struct {
	suite.Suite
	cap    *captureHandler
	server *httptest.Server
}

// TestPaginatorSuite — runs the test suite.
func TestPaginatorSuite(t *testing.T) {
	suite.Run(t, new(PaginatorTestSuite))
}

// SetupTest — creates a fresh capture handler before each test.
func (s *PaginatorTestSuite) SetupTest() {
	s.cap = &captureHandler{}
}

// TearDownTest — closes the mock server after each test.
func (s *PaginatorTestSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
	}
}

// newClient — creates a fetcher client.
func (s *PaginatorTestSuite) newClient() *fetcher.Client {
	return fetcher.New(fetcher.Options{Logger: s.newLogger()})
}

// newLogger — creates a logger that captures messages into s.cap.
func (s *PaginatorTestSuite) newLogger() *slog.Logger {
	return slog.New(s.cap)
}

// newPaginator — creates a Paginator for a mock server topic.
func (s *PaginatorTestSuite) newPaginator(ms *mockServer) *paginator.Paginator {
	return s.makePaginator(ms.URL + "/threads/123/")
}

// makePaginator — creates a Paginator for an arbitrary base URL.
func (s *PaginatorTestSuite) makePaginator(base string) *paginator.Paginator {
	p := paginator.New(s.newClient(), parser.NewXenForo(), base)
	p.SetLogger(s.newLogger())
	return p
}

type requestRecorder struct {
	mu   sync.Mutex
	path []string
}

func (r *requestRecorder) record(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.path = append(r.path, path)
}

func (r *requestRecorder) ordered() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.path))
	copy(out, r.path)
	return out
}

func (r *requestRecorder) countAll() int {
	return len(r.ordered())
}

type mockServer struct {
	*httptest.Server
	rec *requestRecorder
}

// newMockServer — creates a mock server for a XenForo topic; closed in TearDownTest.
func (s *PaginatorTestSuite) newMockServer(lastPage int, statuses map[string]int) *mockServer {
	ms := &mockServer{rec: &requestRecorder{}}
	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.rec.record(r.URL.Path)
		if status, ok := statuses[r.URL.Path]; ok {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(xenForoHTML(lastPage)))
	}))
	return ms
}

// xenForoHTML — XenForo topic HTML with nav links for `lastPage` pages.
func xenForoHTML(lastPage int) string {
	var nav strings.Builder
	for i := 1; i <= lastPage; i++ {
		if i == 1 {
			nav.WriteString(`<a class="p-nav-link">1</a>`)
		} else {
			fmt.Fprintf(&nav, `<a class="p-nav-link" href="/threads/123/page-%d">%d</a>`, i, i)
		}
	}
	return `<html lang="en"><head><meta charset="utf-8"><title>Test topic</title></head><body><div class="wrap" data-app="XenForo"><h1 class="p-title-title">Test topic</h1><nav class="p-nav"><ul>` + nav.String() + `</ul></nav><article class="message" data-content="post-123" data-thread="123"><h3 class="message-userName"><a href="/members/42/user">user</a></h3><time class="date" datetime="2024-01-01T00:00:00Z">Jan 1</time><div class="message-body"><p>Post content</p></div><a class="message-link" href="/threads/123#post-123">#post-123</a></article></body></html>`
}

// captureHandler — captures slog messages for assertions.
type captureHandler struct {
	mu   sync.Mutex
	msgs []string
}

// Enabled — reports whether the handler handles records at the given level.
func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

// Handle — stores the record message.
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgs = append(h.msgs, r.Message)
	return nil
}

// WithAttrs — returns the same handler (attrs are not needed).
func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

// WithGroup — returns the same handler (groups are not needed).
func (h *captureHandler) WithGroup(_ string) slog.Handler { return h }

func (h *captureHandler) contains(substr string) bool {
	for _, m := range h.msgs {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}
