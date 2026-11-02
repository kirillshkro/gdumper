// Package cmd — E2E-сценарии: cookies, user-agent, YAML-конфиг,
// unknown flag (exit 4 / exit 2).
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TestCookiesApplied — raw cookies are sent with each request
// (Cookie header of the mock server).
func (s *e2eTestSuite) TestCookiesApplied() {
	srv, rec := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := append(s.baseArgs(s.topicURL(srv), out),
		"--cookies", `raw:"session=abc; theme=dark"`)
	output, code := s.runBinary(args...)

	s.Require().Equal(0, code, "\n%s", output)
	cookie, ok := rec.cookies[s.topicURL(srv)]
	s.Require().True(ok, "the first request must have a Cookie header")
	s.Require().Contains(cookie, "session=abc")
}

// TestMissingCookiesFileExit4 — nonexistent cookies file: exit 4
// (authentication error).
func (s *e2eTestSuite) TestMissingCookiesFileExit4() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := append(s.baseArgs(s.topicURL(srv), out),
		"--cookies", "file:"+filepath.Join(s.T().TempDir(), "missing.txt"))
	output, code := s.runBinary(args...)

	s.Require().Equal(4, code, "auth error must exit with code 4\n%s", output)
	s.Require().Contains(output, "authentication failed")
}

// TestInvalidRawCookiesExit2 — invalid raw cookies (no "=" in the pair):
// configuration error at the Load stage, exit 2.
func (s *e2eTestSuite) TestInvalidRawCookiesExit2() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := append(s.baseArgs(s.topicURL(srv), out),
		"--cookies", `raw:"noequals"`)
	output, code := s.runBinary(args...)

	s.Require().Equal(2, code, "invalid cookies must exit with code 2\n%s", output)
	s.Require().Contains(output, "invalid cookie pair")
}

// TestUnknownBrowserExit2 — неизвестный браузер в источнике cookies:
// exit 2.
func (s *e2eTestSuite) TestUnknownBrowserExit2() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := append(s.baseArgs(s.topicURL(srv), out),
		"--cookies", "browser:opera")
	output, code := s.runBinary(args...)

	s.Require().Equal(2, code, "unknown browser must exit with code 2\n%s", output)
	s.Require().Contains(output, "unknown browser")
}

// TestCustomUserAgent — --user-agent is set in the request header.
func (s *e2eTestSuite) TestCustomUserAgent() {
	srv, rec := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := append(s.baseArgs(s.topicURL(srv), out),
		"--user-agent", "forumdump-test/1.0")
	output, code := s.runBinary(args...)

	s.Require().Equal(0, code, "\n%s", output)
	ua, ok := rec.ua[s.topicURL(srv)]
	s.Require().True(ok, "the first request must have a User-Agent header")
	s.Require().Equal("forumdump-test/1.0", ua)
}

// TestUnknownFlagExit2 — unknown flag: exit 2 (argument error).
func (s *e2eTestSuite) TestUnknownFlagExit2() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := append(s.baseArgs(s.topicURL(srv), out), "--bogus-flag")
	output, code := s.runBinary(args...)

	s.Require().Equal(2, code, "unknown flag must exit with code 2\n%s", output)
	s.Require().Contains(output, "unknown flag")
}

// TestConfigFileYAML — settings from a YAML file (spec section 8.1):
// URL without --url flag comes from the file; --format flag takes
// precedence over the file.
func (s *e2eTestSuite) TestConfigFileYAML() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	cfgFile := filepath.Join(s.T().TempDir(), "config.yml")
	cfg := fmt.Sprintf("url: %s\nout: %s\nformat: json\ndelay: 0\nretries: 0\nattachments: skip\n",
		s.topicURL(srv), out)
	s.Require().NoError(os.WriteFile(cfgFile, []byte(cfg), 0o600))

	// Without --url flag: URL is taken from the config file.
	output, code := s.runBinary("--config", cfgFile)
	s.Require().Equal(0, code, "\n%s", output)
	m := s.loadManifest(out)
	s.Require().Equal(3, m.PagesFetched)

	// The flag --format takes precedence over the file.
	out2 := filepath.Join(s.T().TempDir(), "out2")
	args2 := []string{"--config", cfgFile, "--format", "jsonl", "--out", out2}
	output, code = s.runBinary(args2...)
	s.Require().Equal(0, code, "\n%s", output)
	data := s.readFile(out2, "topic.jsonl")
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	s.Require().Len(lines, 6)
}