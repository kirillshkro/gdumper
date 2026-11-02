// Package cmd — end-to-end scenarios for the `forumdump` CLI
// (subprocess execution of the prebuilt binary on a mock
// XenForo-like forum, httptest).
//
// The scenarios cover the main crawl path (spec section 2.2),
// page ranges, output formats and the exit code matrix
// (spec section 6): exit 0 — success, exit 5 — partial success.
package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// e2eTestSuite — test suite for the full crawl scenarios.
type e2eTestSuite struct {
	suite.Suite
	binPath string
}

// TestE2ESuite — launches the suite.
func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(e2eTestSuite))
}

// mockTopicID — fixed ID of the topic in the mock forum.
const mockTopicID = "12345"

// mockOpts — parameters of the mock forum server.
type mockOpts struct {
	// Delay — delay per response (for graceful stop tests).
	Delay time.Duration
	// Status — per-page status code; pages not in the map are 200.
	Status map[int]int
}

// newMockServer — httptest server that serves a XenForo-like forum:
// /threads/<id>/ and /threads/<id>/page-N — pages, /plain/ — markup
// that no parser recognizes, all other paths — HTTP 404.
//
// Pages are routed in a single /threads/ handler: http.ServeMux treats
// patterns that do not end with "/" as exact matches, so a separate
// /page- prefix would never catch real URLs like /page-2. The handler
// records each request once and dispatches by path.
// `Status` injects a non-200 status for a specific page
// (e.g. page 3 -> 404 for the partial-dump scenario).
func (s *e2eTestSuite) newMockServer(opts mockOpts) (*httptest.Server, *recorder) {
	rec := newRecorder()
	t := s.T()

	// servePage replies with Status[n] when set, otherwise 200 + HTML.
	// The handler records the request; servePage does not.
	servePage := func(n int) func(w http.ResponseWriter, r *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			if opts.Delay > 0 {
				time.Sleep(opts.Delay)
			}
			status := http.StatusOK
			if st, ok := opts.Status[n]; ok {
				status = st
			}
			if status != http.StatusOK {
				w.WriteHeader(status)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if _, e := w.Write([]byte(mockPage(n, 3))); e != nil {
				t.Errorf("mock server: write page %d: %v", n, e)
			}
		}
	}

	base := "/threads/" + mockTopicID + "/"
	mux := http.NewServeMux()
	mux.HandleFunc("/threads/", func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		switch {
		case r.URL.Path == base:
			servePage(1)(w, r)
		case strings.HasPrefix(r.URL.Path, base+"page-"):
			seg := filepath.Base(r.URL.Path)
			n, err := strconv.Atoi(strings.TrimPrefix(seg, "page-"))
			if err != nil || n < 1 {
				http.Error(w, "invalid page", http.StatusBadRequest)
				return
			}
			servePage(n)(w, r)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	mux.HandleFunc("/plain/", func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, e := w.Write([]byte(`<html><body><p>Not a forum</p></body></html>`)); e != nil {
			t.Errorf("mock server: write plain page: %v", e)
		}
	})

	return httptest.NewServer(mux), rec
}

// mockPage — returns the HTML of a XenForo-like page:
// marker data-app="XenForo", title h1.p-title-title, page nav
// a.p-nav-link, messages article.message (2 per page, second — reply).
func mockPage(page, totalPages int) string {
	nav := makePageNav(totalPages)
	var posts strings.Builder
	for i, text := range []string{
		fmt.Sprintf("First post on page %d", page),
		fmt.Sprintf("Reply to the first post on page %d", page),
	} {
		id := 100 + (page-1)*2 + i
		parent := ""
		if i > 0 {
			parent = fmt.Sprintf(` data-parent="%d"`, 100+(page-1)*2+i-1)
		}
		fmt.Fprintf(&posts, `<article class="message" data-content="%d" data-thread="%s"%s>`,
			id, mockTopicID, parent)
		posts.WriteString(`<h3 class="message-userName"><a href="/users/7/">Alice</a></h3>`)
		posts.WriteString(`<time class="date" datetime="2023-01-01T12:00:00Z">Jan 1, 2023</time>`)
		fmt.Fprintf(&posts, `<div class="message-body"><p>%s</p></div>`, text)
		fmt.Fprintf(&posts, `<a class="message-link" href="/threads/%s/page-%d/#%d">Post</a>`,
			mockTopicID, page, id)
		posts.WriteString(`</article>`)
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Mock Forum — Topic %s</title>
<div data-app="XenForo"></div>
</head>
<body>
<nav class="p-nav">%s</nav>
<h1 class="p-title-title">Mock topic title</h1>%s
</body>
</html>`, mockTopicID, nav, posts.String())
}

// makePageNav — returns the page navigation fragment.
func makePageNav(totalPages int) string {
	var sb strings.Builder
	for i := 1; i <= totalPages; i++ {
		fmt.Fprintf(&sb, `<a class="p-nav-link">%d</a>`, i)
	}
	return sb.String()
}

// recorder — log of requests received by the mock server.
type recorder struct {
	mu      sync.Mutex
	urls    []string
	ua      map[string]string
	cookies map[string]string
}

// newRecorder — returns a new recorder.
func newRecorder() *recorder {
	return &recorder{
		ua:      map[string]string{},
		cookies: map[string]string{},
	}
}

// record — adds the request to the recorder. `urls` holds request paths,
// while `ua` and `cookies` are keyed by the full URL (scheme://host/path),
// so tests can look them up with s.topicURL(srv). Server-side requests do
// not populate req.URL.Scheme/req.URL.Host; the authority is req.Host.
func (r *recorder) record(req *http.Request) {
	scheme := req.URL.Scheme
	if scheme == "" {
		scheme = "http"
	}
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}
	key := scheme + "://" + host + req.URL.Path
	if req.URL.RawQuery != "" {
		key += "?" + req.URL.RawQuery
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if req.URL != nil {
		r.urls = append(r.urls, req.URL.Path)
	}
	if _, ok := r.ua[key]; !ok && req.Header.Get("User-Agent") != "" {
		r.ua[key] = req.Header.Get("User-Agent")
	}
	if _, ok := r.cookies[key]; !ok && req.Header.Get("Cookie") != "" {
		r.cookies[key] = req.Header.Get("Cookie")
	}
}

// waitForURL — waits for a request with the given path prefix
// to appear in the request log (timeout).
func (r *recorder) waitForURL(prefix string, timeout time.Duration) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			for _, u := range r.urls {
				if strings.HasPrefix(u, prefix) {
					r.mu.Unlock()
					return nil
				}
			}
			r.mu.Unlock()
		case <-time.After(timeout):
			return fmt.Errorf("server did not receive a request %q within %v", prefix, timeout)
		}
	}
}


// SetupSuite — builds the CLI binary once before all tests.
//
// Subprocess execution is used: os.Exit() inside RunE terminates
// the process, so exit codes can only be observed via a separate
// process (not via go run or in-process Execute).
func (s *e2eTestSuite) SetupSuite() {
	s.binPath = filepath.Join(s.T().TempDir(), "forumdump")
	build := exec.Command("go", "build", "-o", s.binPath, cliPackage)
	output, err := build.CombinedOutput()
	s.Require().NoError(err, "failed to build CLI: %s", output)
}

// topicURL — base URL of the topic in the mock forum.
func (s *e2eTestSuite) topicURL(srv *httptest.Server) string {
	return srv.URL + "/threads/" + mockTopicID + "/"
}

// baseArgs — base CLI arguments: url, out dir, no delay and no
// retries, attachments not downloaded.
func (s *e2eTestSuite) baseArgs(url, out string) []string {
	return []string{
		"--url", url, "--out", out,
		"--delay", "0", "--retries", "0",
		"--attachments", "skip",
	}
}

// runBinary — runs the CLI with the given args and returns
// (combined output, exit code).
func (s *e2eTestSuite) runBinary(args ...string) (string, int) {
	cmd := exec.Command(s.binPath, args...)
	output, err := cmd.CombinedOutput()
	code := 0
	if ex, ok := err.(*exec.ExitError); ok {
		code = ex.ExitCode()
	} else if err != nil {
		code = -1
	}
	return string(output), code
}
