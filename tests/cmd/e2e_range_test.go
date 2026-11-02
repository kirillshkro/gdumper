// Package cmd — E2E-сценарии: диапазоны страниц и формат JSONL.
package cmd

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
)

// TestPagesRangeOnlyRequested — --pages 2-3: only pages 2 and 3 are
// crawled; the base (page 1) is fetched for parser detection and no
// out-of-range page is requested. Exit 0.
func (s *e2eTestSuite) TestPagesRangeOnlyRequested() {
	srv, rec := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := append(s.baseArgs(s.topicURL(srv), out), "--pages", "2-3")
	output, code := s.runBinary(args...)

	s.Require().Equal(0, code, "\n%s", output)

	// Only the in-range pages are requested as content; the base page
	// (page 1) is fetched for parser detection, and no out-of-range page
	// is requested. Crawled messages must come from pages 2 and 3.
	m := s.loadManifest(out)
	s.Require().Equal(2, m.PagesFetched)
	s.Require().Contains(rec.urls, "/threads/"+mockTopicID+"/page-2")
	s.Require().Contains(rec.urls, "/threads/"+mockTopicID+"/page-3")
	s.Require().False(slices.Contains(rec.urls, "/threads/"+mockTopicID+"/page-4"), "out-of-range page-4 must not be requested")

	topic := s.loadTopic(out)
	s.Require().Len(topic.Messages, 4)
	s.Equal("102", topic.Messages[0].ID)
	s.Equal("105", topic.Messages[3].ID)
}

// TestSinglePageRange — --pages 3: single-page crawl; only page 3 is
// crawled and no out-of-range page is requested. Exit 0.
func (s *e2eTestSuite) TestSinglePageRange() {
	srv, rec := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := append(s.baseArgs(s.topicURL(srv), out), "--pages", "3")
	output, code := s.runBinary(args...)

	s.Require().Equal(0, code, "\n%s", output)

	m := s.loadManifest(out)
	s.Require().Equal(1, m.PagesFetched)
	s.Require().Contains(rec.urls, "/threads/"+mockTopicID+"/page-3")
	s.Require().False(slices.Contains(rec.urls, "/threads/"+mockTopicID+"/page-2"), "page-2 must not be requested")
	s.Require().False(slices.Contains(rec.urls, "/threads/"+mockTopicID+"/page-4"), "out-of-range page-4 must not be requested")

	topic := s.loadTopic(out)
	s.Require().Len(topic.Messages, 2)
	s.Equal("104", topic.Messages[0].ID)
}

// TestJSONLFormat — --format jsonl: topic.jsonl contains one line per
// message, each line is valid JSON.
func (s *e2eTestSuite) TestJSONLFormat() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := append(s.baseArgs(s.topicURL(srv), out), "--format", "jsonl")
	output, code := s.runBinary(args...)

	s.Require().Equal(0, code, "\n%s", output)
	data := s.readFile(out, "topic.jsonl")
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	s.Require().Len(lines, 6)

	ids := make(map[string]bool)
	for _, line := range lines {
		var m messageJSON
		s.Require().NoError(json.Unmarshal([]byte(line), &m))
		ids[m.ID] = true
	}
	for _, id := range []string{"100", "101", "102", "103", "104", "105"} {
		s.Require().True(ids[id], "missing message %q in JSONL", id)
	}
}
