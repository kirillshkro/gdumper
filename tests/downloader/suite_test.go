// Package downloader_test — tests for the internal/downloader package per
// specs-tests.md (TC-05).
package downloader_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kirillshkro/gdumper/internal/downloader"
	"github.com/kirillshkro/gdumper/internal/model"

	"github.com/stretchr/testify/suite"
)

// DownloaderTestSuite — the attachment-downloader test suite.
type DownloaderTestSuite struct {
	suite.Suite
	cap *captureHandler
	dir string
}

// TestDownloaderSuite — runs the suite.
func TestDownloaderSuite(t *testing.T) { suite.Run(t, new(DownloaderTestSuite)) }

// SetupTest — fresh logger capture and temp dir before each test.
func (s *DownloaderTestSuite) SetupTest() {
	s.cap = &captureHandler{}
	s.dir = s.T().TempDir()
}

// newAttachment — builds an attachment fixture.
func newAttachment(mid, name, url string) *model.Attachment {
	return &model.Attachment{MessageID: mid, Name: name, URL: url}
}

// newDownloader — downloader wired to the suite logger and temp dir.
func (s *DownloaderTestSuite) newDownloader(cfg downloader.Config) *downloader.Downloader {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(s.cap)
	}
	if cfg.RootDir == "" {
		cfg.RootDir = s.dir
	}
	d, err := downloader.New(cfg)
	if err != nil {
		s.T().Fatal("downloader.New: ", err)
	}
	return d
}

// route — mock behaviour for a single path.
type route struct {
	status     int // 0 → 200
	body       []byte
	failStatus int // status to return while failures remain
	failures   int
	delay      time.Duration
	setCookie  string // non-empty → Set-Cookie header
}

// mockServer — httptest server with per-path routing and request counts.
type mockServer struct {
	*httptest.Server
	routes      map[string]route
	hits        map[string]int
	uaPerPath   map[string]string
	seenCookies map[string][]string
	mu          sync.Mutex
}

// newServer — creates a mock server from per-path routes.
func newServer(routes map[string]route) *mockServer {
	ms := &mockServer{
		routes:      routes,
		hits:        make(map[string]int),
		uaPerPath:   make(map[string]string),
		seenCookies: make(map[string][]string),
	}
	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := ms.record(r)
		rt := ms.routes[r.URL.Path]
		if rt.setCookie != "" {
			w.Header().Set("Set-Cookie", rt.setCookie)
		}
		if rt.failStatus != 0 && n <= rt.failures {
			http.Error(w, "fail", rt.failStatus)
			return
		}
		if rt.delay > 0 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(rt.delay):
			}
		}
		status := rt.status
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

// record — increments and returns the request counter for r.URL.Path,
// recording the User-Agent and the received cookies at p.
func (ms *mockServer) record(r *http.Request) int {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	p := r.URL.Path
	ms.hits[p]++
	ms.uaPerPath[p] = r.Header.Get("User-Agent")
	for _, kv := range strings.Split(r.Header.Get("Cookie"), ";") {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || name == "" {
			continue
		}
		ms.seenCookies[p] = append(ms.seenCookies[p], name+"="+strings.TrimSpace(value))
	}
	return ms.hits[p]
}

// hitCount — number of requests served for p.
func (ms *mockServer) hitCount(p string) int {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.hits[p]
}

// uaSeen — the User-Agent header received at p.
func (ms *mockServer) uaSeen(p string) string {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.uaPerPath[p]
}

// cookiesAt — the cookies received at p (name=value).
func (ms *mockServer) cookiesAt(p string) []string {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.seenCookies[p]
}

// captureHandler — captures slog messages for assertions.
type captureHandler struct {
	mu   sync.Mutex
	msgs []string
}

// Enabled — always handle records.
func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

// WithAttrs — attrs are not needed.
func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

// WithGroup — groups are not needed.
func (h *captureHandler) WithGroup(_ string) slog.Handler { return h }

// Handle — stores the record message.
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgs = append(h.msgs, r.Message)
	return nil
}

// contains — reports whether a stored message contains a substring.
func (h *captureHandler) contains(substr string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range h.msgs {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}