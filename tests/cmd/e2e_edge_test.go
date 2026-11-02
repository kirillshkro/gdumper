// Package cmd — E2E-сценарии: выход 3 (диск), 0 с incomplete-флагом,
// уровень логирования.
package cmd

import (
	"os"
	"path/filepath"
)

// TestVerboseLogs — --verbose включает DEBUG-уровень (fetch-лог)
// и сохраняет INFO; без флага DEBUG-строки отсутствуют.
func (s *e2eTestSuite) TestVerboseLogs() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")

	verbose, code := s.runBinary(append(s.baseArgs(s.topicURL(srv), out), "--verbose")...)
	s.Require().Equal(0, code, "\n%s", verbose)
	s.Require().Contains(verbose, "level=DEBUG", "verbose log should contain DEBUG lines")
	s.Require().Contains(verbose, "level=INFO")

	out2 := filepath.Join(s.T().TempDir(), "out2")
	output, code := s.runBinary(s.baseArgs(s.topicURL(srv), out2)...)
	s.Require().Equal(0, code, "\n%s", output)
	s.Require().NotContains(output, "level=DEBUG", "without --verbose, DEBUG lines must not be emitted")
	s.Require().Contains(output, "level=INFO")
	s.Require().Contains(output, "dump complete")
}

// TestOccupiedDirectoryExit3 — повторный запуск в непустую существующую
// каталог: exit 3 (ErrDirectoryExists).
func (s *e2eTestSuite) TestOccupiedDirectoryExit3() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	args := s.baseArgs(s.topicURL(srv), out)

	_, code := s.runBinary(args...)
	s.Require().Equal(0, code, "first run must succeed")

	output, code := s.runBinary(args...)
	s.Require().Equal(3, code, "second run must exit with code 3\n%s", output)
	s.Require().Contains(output, "already exists")
}

// TestOutputDirNotDirectoryExit3 — --out указывает на файл, а не
// каталог: exit 3.
func (s *e2eTestSuite) TestOutputDirNotDirectoryExit3() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	outFile := filepath.Join(s.T().TempDir(), "out.txt")
	s.Require().NoError(os.WriteFile(outFile, []byte("x"), 0o600))

	output, code := s.runBinary(s.baseArgs(s.topicURL(srv), outFile)...)
	s.Require().Equal(3, code, "code 3 expected for file, not directory\n%s", output)
	s.Require().Contains(output, "not a directory")
}

// TestFirstPageUnavailable — first page not available (404): the process
// exits cleanly with code 0 (behavior documented in exitCodeForCrawl),
// manifest.incomplete=true, non-empty errors.
func (s *e2eTestSuite) TestFirstPageUnavailable() {
	srv, rec := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	url := srv.URL + "/threads/9999/"
	output, code := s.runBinary(s.baseArgs(url, out)...)

	s.Require().Equal(0, code, "\n%s", output)
	s.Require().Equal(1, len(rec.urls), "only the single failed request must be logged")
	m := s.loadManifest(out)
	s.Require().True(m.Incomplete)
	s.Require().Len(m.Errors, 1)
	s.Require().Contains(m.Errors[0], "first page unavailable")

	topic := s.loadTopic(out)
	s.Require().Len(topic.Messages, 0)
}

// TestNoParserMatch — first page markup is not recognized by any parser:
// exit 0, manifest.incomplete=true, error with parser name.
func (s *e2eTestSuite) TestNoParserMatch() {
	srv, _ := s.newMockServer(mockOpts{})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	url := srv.URL + "/plain/"
	output, code := s.runBinary(s.baseArgs(url, out)...)
	s.Require().Equal(0, code, "\n%s", output)
	m := s.loadManifest(out)
	s.Require().True(m.Incomplete)
	s.Require().Len(m.Errors, 1)
	s.Require().Contains(m.Errors[0], "parser")

	topic := s.loadTopic(out)
	s.Require().Len(topic.Messages, 0)
}
