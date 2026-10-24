// Tests for the paginator pagination scenarios (TC-03.1..03.6).
package paginator_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/kirillshkro/gdumper/internal/paginator"
	"github.com/kirillshkro/gdumper/internal/parser"
)

// TestSinglePage — TC-03.1: single-page topic: lastPage=1, exactly one request.
func (s *PaginatorTestSuite) TestSinglePage() {
	ms := s.newMockServer(1, nil)
	p := s.newPaginator(ms)
	last, err := p.LastPage(s.T().Context())
	s.NoError(err)
	s.Equal(1, last)
	summary, pages := p.Walk(s.T().Context())
	s.Equal(1, summary.Total)
	s.Equal(1, summary.Planned)
	s.Equal(1, summary.Fetched)
	s.Equal(0, summary.Failed)
	s.Len(pages, 1)
	s.Equal(1, pages[0].Number)
	s.NotNil(pages[0].Data)
	s.Equal(1, ms.rec.countAll())
}

// TestMultiPage — TC-03.2: multi-page: N requests in order 1→N.
func (s *PaginatorTestSuite) TestMultiPage() {
	ms := s.newMockServer(10, nil)
	p := s.newPaginator(ms)
	last, err := p.LastPage(s.T().Context())
	s.NoError(err)
	s.Equal(10, last)
	summary, pages := p.Walk(s.T().Context())
	s.Equal(10, summary.Total)
	s.Equal(10, summary.Planned)
	s.Equal(10, summary.Fetched)
	s.Equal(0, summary.Failed)
	s.Len(pages, 10)
	for i, page := range pages {
		s.Equal(i+1, page.Number, "page %d out of order", i)
	}
	s.Equal(10, ms.rec.countAll())
	expected := []string{"/threads/123/"}
	for i := 2; i <= 10; i++ {
		expected = append(expected, fmt.Sprintf("/threads/123/page-%d", i))
	}
	s.Equal(expected, ms.rec.ordered(), "requests out of order")
}

// TestPagesRange — TC-03.3: range --pages=2-4: only the specified pages.
func (s *PaginatorTestSuite) TestPagesRange() {
	ms := s.newMockServer(10, nil)
	p := s.newPaginator(ms)
	s.NoError(p.SetRange("2-4"))
	last, err := p.LastPage(s.T().Context())
	s.NoError(err)
	s.Equal(10, last)
	summary, pages := p.Walk(s.T().Context())
	s.Equal(10, summary.Total)
	s.Equal(3, summary.Planned)
	s.Equal(3, summary.Fetched)
	s.Equal(0, summary.Failed)
	s.Len(pages, 3)
	s.Equal([]int{2, 3, 4},
		[]int{pages[0].Number, pages[1].Number, pages[2].Number})
	// Only pages 2, 3, 4 were requested; 1 and 5 were not touched.
	s.Equal(3, ms.rec.countAll())
	ordered := ms.rec.ordered()
	s.Contains(ordered, "/threads/123/page-2")
	s.Contains(ordered, "/threads/123/page-3")
	s.Contains(ordered, "/threads/123/page-4")
	s.NotContains(ordered, "/threads/123/")
	s.NotContains(ordered, "/threads/123/page-5")
}

// TestBrokenLinks — TC-03.4: broken links: graceful degradation + warning.
func (s *PaginatorTestSuite) TestBrokenLinks() {
	ms := s.newMockServer(5, map[string]int{
		"/threads/123/page-3": 404,
	})
	p := s.newPaginator(ms)
	summary, pages := p.Walk(s.T().Context())
	s.Equal(5, summary.Total)
	s.Equal(5, summary.Planned)
	s.Equal(4, summary.Fetched)
	s.Equal(1, summary.Failed)
	s.Len(summary.Errors, 1)
	s.True(errors.Is(summary.Errors[0], paginator.ErrNotFound))
	s.Len(pages, 5)
	// Page 3: error, warning, no data.
	page3 := pages[2]
	s.Equal(3, page3.Number)
	s.NotNil(page3.Error)
	s.NotEmpty(page3.Warning)
	s.Nil(page3.Data)
	// A warning was logged.
	s.True(s.cap.contains("page failed"))
	// Other pages are parsed normally.
	s.Nil(pages[0].Error)
	s.NotNil(pages[0].Data)
	s.NotNil(pages[1].Data)
	s.NotNil(pages[3].Data)
}

// TestLastPageTermination — TC-03.5: last page without a "next" link.
func (s *PaginatorTestSuite) TestLastPageTermination() {
	// The nav ends at page 3; page 4 does not exist (404).
	ms := s.newMockServer(3, map[string]int{
		"/threads/123/page-4": 404,
	})
	p := s.newPaginator(ms)
	last, err := p.LastPage(s.T().Context())
	s.NoError(err)
	s.Equal(3, last)
	summary, pages := p.Walk(s.T().Context())
	s.Equal(3, summary.Total)
	s.Equal(3, summary.Planned)
	s.Equal(3, summary.Fetched)
	s.Equal(0, summary.Failed)
	s.Len(pages, 3)
	// Page 4 was not fetched at all.
	s.NotContains(ms.rec.ordered(), "/threads/123/page-4")
	s.Equal(3, ms.rec.countAll())
}

// TestRedirect — TC-03.6: base URL redirect to a canonical URL.
func (s *PaginatorTestSuite) TestRedirect() {
	var rec []string
	var mu sync.Mutex
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		rec = append(rec, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/threads/123/":
			http.Redirect(w, r, "/threads/123/canonical", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(xenForoHTML(1)))
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	base := server.URL + "/threads/123/"
	c := s.newClient()
	p := paginator.New(c, parser.NewXenForo(), base)
	p.SetLogger(s.newLogger())
	last, err := p.LastPage(s.T().Context())
	s.NoError(err)
	s.Equal(1, last)
	summary, pages := p.Walk(s.T().Context())
	s.Equal(1, summary.Total)
	s.Equal(1, summary.Fetched)
	s.Equal(0, summary.Failed)
	// Final URL is the canonical one, not the initial base.
	s.Contains(pages[0].URL, "/threads/123/canonical")
	s.NotNil(pages[0].Data)
	// The redirect chain: initial base -> canonical.
	s.Contains(rec, "/threads/123/")
	s.Contains(rec, "/threads/123/canonical")
}
