// Integration tests for paginator + fetcher + parser on HTTP mocks
// (200/404/500, redirects, network errors, fetcher status/redirect checks).
package paginator_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/kirillshkro/gdumper/internal/paginator"
)

// TestIntegrationHTTPStatuses — 200/404/500: mixed statuses.
func (s *PaginatorTestSuite) TestIntegrationHTTPStatuses() {
	ms := s.newMockServer(5, map[string]int{
		"/threads/123/page-2": 404,
		"/threads/123/page-3": 500,
	})
	p := s.newPaginator(ms)
	summary, pages := p.Walk(s.T().Context())
	s.Equal(5, summary.Total)
	s.Equal(5, summary.Planned)
	s.Equal(3, summary.Fetched) // 1, 4, 5
	s.Equal(2, summary.Failed)  // 2 (404), 3 (500)
	s.Len(summary.Errors, 2)
	s.True(errors.Is(pages[1].Error, paginator.ErrNotFound)) // page 2
	s.Contains(pages[2].Error.Error(), "500")                // page 3
	s.Nil(pages[0].Error)
	s.NotNil(pages[0].Data)
	s.Nil(pages[3].Error)
	s.NotNil(pages[3].Data)
	s.Nil(pages[4].Error)
	s.NotNil(pages[4].Data)
}

// TestIntegrationNetworkError — transport-level failure propagates as error.
func (s *PaginatorTestSuite) TestIntegrationNetworkError() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	base := srv.URL + "/threads/123/"
	srv.Close()
	time.Sleep(50 * time.Millisecond) // ensure the listener is closed
	p := s.makePaginator(base)
	last, err := p.LastPage(s.T().Context())
	_ = last
	s.Error(err)
	s.Contains(err.Error(), "paginator")
}

// TestIntegrationFetcherRedirect — fetcher follows a redirect, tracks it.
func (s *PaginatorTestSuite) TestIntegrationFetcherRedirect() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/a" {
			http.Redirect(w, r, "/b", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		n, err := w.Write([]byte("<p>ok</p>"))
		if n <= 0 && err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	c := s.newClient()
	resp, err := c.Fetch(s.T().Context(), srv.URL+"/a")
	s.NoError(err)
	s.Equal(200, resp.StatusCode)
	s.True(resp.Redirected)
	s.Len(resp.Redirects, 1)
	s.Contains(resp.FinalURL, "/b")
	s.Equal([]byte("<p>ok</p>"), resp.Body)
}

// TestIntegrationFetcherStatuses — 200/404/500/429 via the fetcher.
func (s *PaginatorTestSuite) TestIntegrationFetcherStatuses() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/404":
			http.Error(w, "", http.StatusNotFound)
		case "/500":
			http.Error(w, "", http.StatusInternalServerError)
		case "/429":
			http.Error(w, "", http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusOK)
			n, err := w.Write([]byte("ok"))
			if n <= 0 && err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}))
	defer srv.Close()
	c := s.newClient()

	resp, err := c.Fetch(s.T().Context(), srv.URL+"/404")
	s.NoError(err)
	s.Equal(404, resp.StatusCode)
	s.True(resp.IsHTTPError())

	resp, err = c.Fetch(s.T().Context(), srv.URL+"/500")
	s.NoError(err)
	s.Equal(500, resp.StatusCode)
	s.True(resp.IsHTTPError())

	resp, err = c.Fetch(s.T().Context(), srv.URL+"/429")
	s.NoError(err)
	s.Equal(429, resp.StatusCode)
	s.True(resp.IsHTTPError())

	resp, err = c.Fetch(s.T().Context(), srv.URL+"/ok")
	s.NoError(err)
	s.Equal(200, resp.StatusCode)
	s.False(resp.IsHTTPError())
}
