// Package cmd — JSON-схемы артефактов и хелперы их загрузки
// для сценарных E2E-тестов.
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// readFile — читает файл out/name и возвращает его содержимое.
func (s *e2eTestSuite) readFile(out, name string) []byte {
	data, err := os.ReadFile(filepath.Join(out, name))
	s.Require().NoError(err, "file %q must be in the output directory", name)
	return data
}

// manifestJSON — schema of out/manifest.json (spec section 5).
type manifestJSON struct {
	DumpedAt     string   `json:"dumped_at"`
	PagesFetched int      `json:"pages_fetched"`
	Errors       []string `json:"errors"`
	Incomplete   bool     `json:"incomplete"`
	ToolVersion  string   `json:"tool_version"`
}

// loadManifest — reads and parses the manifest, asserting existence.
func (s *e2eTestSuite) loadManifest(out string) manifestJSON {
	var m manifestJSON
	s.Require().NoError(json.Unmarshal(s.readFile(out, "manifest.json"), &m))
	return m
}

// topicJSON — element of the topic.json array (spec section 5).
type topicJSON struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Author     struct {
		Name       string `json:"name"`
		ProfileURL string `json:"profile_url"`
	} `json:"author"`
	Created    string `json:"created_at"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"pages_total"`
	Content    string `json:"content"`
	Messages   []messageJSON `json:"messages"`
}

// messageJSON — message from topic.json (fields needed for assertions).
type messageJSON struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	Author   struct {
		Name       string `json:"name"`
		ProfileURL string `json:"profile_url"`
	} `json:"author"`
	BodyText string `json:"body_text"`
	Level    int    `json:"level"`
}

// loadTopic — reads topic.json and returns the single element.
func (s *e2eTestSuite) loadTopic(out string) topicJSON {
	var topics []topicJSON
	s.Require().NoError(json.Unmarshal(s.readFile(out, "topic.json"), &topics))
	s.Require().Len(topics, 1, "topic.json must contain exactly one topic")
	return topics[0]
}