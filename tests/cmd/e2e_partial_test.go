// Package cmd — E2E-сценарии: частичный успех (exit 5) и доступность
// страниц (500, 404).
package cmd

import (
	"net/http"
	"path/filepath"
)

// TestPartialSuccessExit5 — one page unavailable (HTTP 500): graceful
// degradation, exit 5 (spec 6), manifest.incomplete=true, non-empty
// errors, saved pages remain.
func (s *e2eTestSuite) TestPartialSuccessExit5() {
	srv, _ := s.newMockServer(mockOpts{Status: map[int]int{3: http.StatusInternalServerError}})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	output, code := s.runBinary(s.baseArgs(s.topicURL(srv), out)...)

	s.Require().Equal(5, code, "crawl with errors must exit with code 5\n%s", output)

	m := s.loadManifest(out)
	s.Require().True(m.Incomplete)
	s.Require().Equal(2, m.PagesFetched)
	s.Require().Len(m.Errors, 1)
	s.Require().Contains(m.Errors[0], "page 3")

	topic := s.loadTopic(out)
	s.Require().Len(topic.Messages, 4)
	s.Equal("100", topic.Messages[0].ID)
	s.Equal("103", topic.Messages[3].ID)
}

// TestNotFoundPageExit5 — unavailable page (HTTP 404): graceful
// degradation, exit 5, manifest.incomplete=true.
func (s *e2eTestSuite) TestNotFoundPageExit5() {
	srv, _ := s.newMockServer(mockOpts{Status: map[int]int{3: http.StatusNotFound}})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	output, code := s.runBinary(s.baseArgs(s.topicURL(srv), out)...)

	s.Require().Equal(5, code, "\n%s", output)
	m := s.loadManifest(out)
	s.Require().True(m.Incomplete)
	s.Require().Equal(2, m.PagesFetched)
	s.Require().Contains(m.Errors[0], "404")
}