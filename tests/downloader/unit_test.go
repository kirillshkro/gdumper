// Unit tests for the downloader (TC-05.1, TC-05.3, TC-05.6).
package downloader_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/kirillshkro/gdumper/internal/downloader"
	"github.com/kirillshkro/gdumper/internal/model"
)

// TestUnitSinglePDF — TC-05.1: one PDF is downloaded and its SHA-256 matches.
func (s *DownloaderTestSuite) TestUnitSinglePDF() {
	body := []byte("%PDF-1.4\nminimal golden fixture\n")
	ms := newServer(map[string]route{
		"/report.pdf": {body: body, status: http.StatusOK},
	})
	defer ms.Close()
	d := s.newDownloader(downloader.Config{
		Strategy: downloader.StrategyDownload,
		Retries:  0,
	})
	att := newAttachment("msg-1", "report.pdf", ms.URL+"/report.pdf")
	atts, err := d.Download(s.T().Context(), []*model.Attachment{att})
	s.NoError(err)
	s.Len(atts, 1)
	s.NotEmpty(att.LocalPath)
	bodySHA := sha256.Sum256(body)
	s.Equal(hex.EncodeToString(bodySHA[:]), att.SHA256)
	f, err := os.Open(att.LocalPath)
	s.Require().NoError(err)
	defer func() {
		if ce := f.Close(); ce != nil {
			s.Require().NoError(ce)
		}
	}()
	content, err := io.ReadAll(f)
	s.NoError(err)
	s.Equal(body, content)
	s.Contains(att.LocalPath, filepath.Join(s.dir, "attachments", "msg-1"))
}

// TestUnitLinksSkip — TC-05.3: links/skip strategies return attachments
// unchanged and never contact the server.
func (s *DownloaderTestSuite) TestUnitLinksSkip() {
	ms := newServer(map[string]route{})
	defer ms.Close()
	for _, strategy := range []downloader.Strategy{downloader.StrategyLinks, downloader.StrategySkip} {
		s.T().Run(string(strategy), func(t *testing.T) {
			att := newAttachment("msg-2", "a.pdf", ms.URL+"/a.pdf")
			d := s.newDownloader(downloader.Config{Strategy: strategy})
			out, err := d.Download(t.Context(), []*model.Attachment{att})
			s.NoError(err)
			s.Equal(att.URL, out[0].URL)
			s.Empty(att.LocalPath)
			s.Empty(att.SHA256)
			s.Equal(0, ms.hitCount("/a.pdf"))
		})
	}
}

// TestUnitIdempotentRerun — TC-05.6: a second Download reuses the local file
// and makes no additional network request.
func (s *DownloaderTestSuite) TestUnitIdempotentRerun() {
	body := []byte("hello attachment")
	ms := newServer(map[string]route{"/a.pdf": {body: body}})
	defer ms.Close()
	d := s.newDownloader(downloader.Config{Strategy: downloader.StrategyDownload, Retries: 0})
	att := newAttachment("msg-3", "a.pdf", ms.URL+"/a.pdf")
	ctx := s.T().Context()
	_, err := d.Download(ctx, []*model.Attachment{att})
	s.NoError(err)
	s.NotEmpty(att.LocalPath)
	s.Equal(1, ms.hitCount("/a.pdf"))
	// second run must not re-download
	_, err = d.Download(ctx, []*model.Attachment{att})
	s.NoError(err)
	s.Equal(1, ms.hitCount("/a.pdf"))
}