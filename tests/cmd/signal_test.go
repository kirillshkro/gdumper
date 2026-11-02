// Package cmd — E2E-scenario: graceful stop by SIGINT (spec section 7).
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// TestSigintGracefulPartialDump — SIGINT during crawl: the process
// exits cleanly within timeout, pages already fetched are saved,
// manifest.incomplete=true, exit code 5.
func (s *e2eTestSuite) TestSigintGracefulPartialDump() {
	srv, rec := s.newMockServer(mockOpts{Delay: 500 * time.Millisecond})
	defer srv.Close()

	out := filepath.Join(s.T().TempDir(), "out")
	cmd := exec.Command(s.binPath, s.baseArgs(s.topicURL(srv), out)...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	s.Require().NoError(cmd.Start(), "process start")

	page1Path := "/threads/" + mockTopicID + "/"
	page2Path := "/threads/" + mockTopicID + "/page-2"
	s.Require().Nil(rec.waitForURL(page1Path, 10*time.Second), "page 1 request")
	s.Require().Nil(rec.waitForURL(page2Path, 10*time.Second), "page 2 request")
	time.Sleep(100 * time.Millisecond) // page 2 response is still in flight
	s.Require().NoError(cmd.Process.Signal(os.Interrupt), "SIGINT should be deliverable")

	code, werr := s.waitWithTimeout(cmd, 5*time.Second)
	s.Require().Nil(werr, "process must exit after SIGINT")

	output := buf.String()
	s.Require().Equal(5, code,
		"partial crawl interrupted by SIGINT must exit with code 5\n%s", output)
	s.Require().Contains(output, "interrupt signal received")

	m := s.loadManifest(out)
	s.Require().True(m.Incomplete)
	s.Require().Equal(1, m.PagesFetched)

	topic := s.loadTopic(out)
	s.Require().Len(topic.Messages, 2)
	s.Equal("100", topic.Messages[0].ID)
	s.Equal("101", topic.Messages[1].ID)
}

// waitWithTimeout — waits for process termination with a timeout and returns
// its exit code; on timeout it kills the process and returns an error.
func (s *e2eTestSuite) waitWithTimeout(cmd *exec.Cmd, timeout time.Duration) (int, error) {
	done := make(chan struct{})
	code := make(chan int, 1)
	go func() {
		if err := cmd.Wait(); err == nil {
			code <- 0
		} else if ex, ok := err.(*exec.ExitError); ok {
			code <- ex.ExitCode()
		} else {
			code <- -1
		}
		close(done)
	}()
	select {
	case <-done:
		return <-code, nil
	case <-time.After(timeout):
		if e := cmd.Process.Kill(); e != nil {
			s.T().Errorf("failed to kill process on timeout: %v", e)
		}
		return -1, fmt.Errorf("process did not exit within %v", timeout)
	}
}
