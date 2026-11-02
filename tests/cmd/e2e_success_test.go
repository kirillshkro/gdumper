// Package cmd — E2E-сценарий: полный успешный дамп темы XenForo.
package cmd

import (
	"fmt"
	"path/filepath"
	"time"
)

// TestFullDumpSuccess — main scenario (spec 2.2): full crawl of the
// 3-page topic with 2 messages per page, no failures. Exit 0,
// artifacts on disk (topic.json, manifest.json), all messages
// extracted with the correct nesting levels.
func (s *e2eTestSuite) TestFullDumpSuccess() {
	srv, rec := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	output, code := s.runBinary(s.baseArgs(s.topicURL(srv), out)...)

	s.Require().Equal(0, code, "successful crawl must exit with code 0\n%s", output)

	// Every page (1..3) must have been requested. The base (page 1) is also
	// fetched for parser detection, so it may appear more than once in the raw
	// request log.
	seen := make(map[string]struct{})
	for _, u := range rec.urls {
		seen[u] = struct{}{}
	}
	for _, want := range []string{"/threads/" + mockTopicID + "/", "/threads/"+mockTopicID+"/page-2", "/threads/"+mockTopicID+"/page-3"} {
		_, ok := seen[want]
		s.Require().True(ok, "page %q must be requested", want)
	}
	_, ok4 := seen["/threads/"+mockTopicID+"/page-4"]
	s.Require().False(ok4, "out-of-range page must not be requested")

	topic := s.loadTopic(out)
	s.Equal("12345", topic.ID)
	s.Equal("Mock topic title", topic.Title)
	s.Equal("2023-01-01T12:00:00Z", topic.Created)
	s.Equal(3, topic.TotalPages)
	s.Contains(topic.Author.ProfileURL, "/users/7/")

	m := s.loadManifest(out)
	s.Equal("1.0.0", m.ToolVersion)
	s.Equal(3, m.PagesFetched)
	s.False(m.Incomplete)
	s.Empty(m.Errors)
	_, err := time.Parse(time.RFC3339, m.DumpedAt)
	s.Require().NoError(err, "manifest dumped_at must be RFC3339")

	msgs := topic.Messages
	s.Require().Len(msgs, 6)
	for i := 0; i < 6; i++ {
		s.Equal(fmt.Sprintf("10%d", i), msgs[i].ID, "message id %d", i)
	}
	s.Equal(0, msgs[0].Level)
	s.Equal("100", msgs[1].ParentID)
	s.Equal(1, msgs[1].Level)
	s.Equal("Alice", msgs[0].Author.Name)
	s.Contains(msgs[0].BodyText, "page 1")
	s.Contains(msgs[5].BodyText, "page 3")
}