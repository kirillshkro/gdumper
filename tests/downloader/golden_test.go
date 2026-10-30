// Golden tests for the downloader (specs-tests.md §4).
package downloader_test

import (
	"os"
	"path/filepath"

	"github.com/kirillshkro/gdumper/internal/downloader"
	"github.com/kirillshkro/gdumper/internal/model"

	"github.com/sebdah/goldie/v2"
)

// TestGoldenDownloadedAttachment — downloads the seeded report.pdf fixture and
// compares its bytes and SHA-256 with the golden fixtures via goldie/v2.
func (s *DownloaderTestSuite) TestGoldenDownloadedAttachment() {
	t := s.T()
	fixture, err := os.ReadFile(filepath.Join("testdata", "report.pdf"))
	s.Require().NoError(err)
	ms := newServer(map[string]route{"/report.pdf": {body: fixture}})
	defer ms.Close()
	d := s.newDownloader(downloader.Config{Strategy: downloader.StrategyDownload, Retries: 0})
	att := newAttachment("golden", "report.pdf", ms.URL+"/report.pdf")
	_, err = d.Download(t.Context(), []*model.Attachment{att})
	s.Require().NoError(err)
	s.Require().NotEmpty(att.LocalPath)

	downloaded, err := os.ReadFile(att.LocalPath)
	s.Require().NoError(err)
	g := goldie.New(t, goldie.WithNameSuffix(".pdf"))
	g.Assert(t, "report", downloaded)

	g2 := goldie.New(t, goldie.WithNameSuffix(".sha256"))
	g2.Assert(t, "report", []byte(att.SHA256))
}