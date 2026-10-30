// Integration tests for the downloader (TC-05.2, TC-05.4, TC-05.5, TC-05.7).
package downloader_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kirillshkro/gdumper/internal/downloader"
	"github.com/kirillshkro/gdumper/internal/model"
)

// TestIntegrationMultipleAttachments — TC-05.2: several attachments all land in
// attachments/<post_id>/ under distinct names.
func (s *DownloaderTestSuite) TestIntegrationMultipleAttachments() {
	ctx := s.T().Context()
	ms := newServer(map[string]route{
		"/one.pdf":  {body: []byte("one")},
		"/two.txt":  {body: []byte("two")},
		"/three.bin": {body: []byte("three")},
	})
	defer ms.Close()
	d := s.newDownloader(downloader.Config{Strategy: downloader.StrategyDownload, Retries: 0})
	atts := []*model.Attachment{
		newAttachment("post-a", "one.pdf", ms.URL+"/one.pdf"),
		newAttachment("post-a", "two.txt", ms.URL+"/two.txt"),
		newAttachment("post-a", "three.bin", ms.URL+"/three.bin"),
	}
	out, err := d.Download(ctx, atts)
	s.NoError(err)
	s.Len(out, 3)
	postDir := filepath.Join(s.dir, "attachments", "post-a")
	want := map[string][]byte{"one.pdf": []byte("one"), "two.txt": []byte("two"), "three.bin": []byte("three")}
	for _, a := range out {
		s.NotEmpty(a.LocalPath)
		s.Contains(a.LocalPath, postDir)
		content, err := os.ReadFile(a.LocalPath)
		s.NoError(err)
		s.Equal(want[a.Name], content)
	}
	s.Equal(1, ms.hitCount("/one.pdf"))
	s.Equal(1, ms.hitCount("/two.txt"))
	s.Equal(1, ms.hitCount("/three.bin"))
}

// TestIntegrationNotFound — TC-05.4: a 404 attachment is skipped with a
// warning while valid ones still download; the overall post keeps saving.
func (s *DownloaderTestSuite) TestIntegrationNotFound() {
	ctx := s.T().Context()
	ms := newServer(map[string]route{
		"/missing.pdf": {failStatus: http.StatusNotFound, failures: 10},
		"/ok.png":      {body: []byte("ok")},
	})
	defer ms.Close()
	d := s.newDownloader(downloader.Config{Strategy: downloader.StrategyDownload, Retries: 0})
	atts := []*model.Attachment{
		newAttachment("post-404", "missing.pdf", ms.URL+"/missing.pdf"),
		newAttachment("post-404", "ok.png", ms.URL+"/ok.png"),
	}
	out, err := d.Download(ctx, atts)
	s.NoError(err)
	s.Empty(out[0].LocalPath)
	s.Empty(out[0].SHA256)
	s.NotEmpty(out[1].LocalPath)
	okSHA := sha256.Sum256([]byte("ok"))
	s.Equal(hex.EncodeToString(okSHA[:]), out[1].SHA256)
	s.Equal(1, ms.hitCount("/missing.pdf")) // 4xx is not retried
	s.True(s.cap.contains("not found"))
}

// TestIntegrationLargeFile — TC-05.5: a 100 MiB body is streamed (256 KiB
// chunks, bounded memory) and fully hashed.
func (s *DownloaderTestSuite) TestIntegrationLargeFile() {
	ctx := s.T().Context()
	const size = 100 * 1024 * 1024
	body := make([]byte, size)
	for i := range body {
		body[i] = byte(i % 0xFF)
	}
	bigSHA := sha256.Sum256(body)
	expectedSHA := hex.EncodeToString(bigSHA[:])
	ms := newServer(map[string]route{"/big.bin": {body: body}})
	defer ms.Close()
	d := s.newDownloader(downloader.Config{Strategy: downloader.StrategyDownload, Retries: 0, ChunkSize: 256 * 1024})
	att := newAttachment("post-big", "big.bin", ms.URL+"/big.bin")
	_, err := d.Download(ctx, []*model.Attachment{att})
	s.NoError(err)
	s.NotEmpty(att.LocalPath)
	info, err := os.Stat(att.LocalPath)
	s.NoError(err)
	s.Equal(int64(size), info.Size())
	s.Equal(expectedSHA, att.SHA256)
}

// TestIntegrationPreviewAndOriginal — TC-05.7: an image preview and its
// original are both saved, each under its own filename.
func (s *DownloaderTestSuite) TestIntegrationPreviewAndOriginal() {
	ctx := s.T().Context()
	ms := newServer(map[string]route{
		"/thumb.png": {body: []byte("thumb")},
		"/photo.png": {body: []byte("photo")},
	})
	defer ms.Close()
	d := s.newDownloader(downloader.Config{Strategy: downloader.StrategyDownload, Retries: 0})
	atts := []*model.Attachment{
		newAttachment("post-7", "thumb.png", ms.URL+"/thumb.png"),
		newAttachment("post-7", "photo.png", ms.URL+"/photo.png"),
	}
	out, err := d.Download(ctx, atts)
	s.NoError(err)
	postDir := filepath.Join(s.dir, "attachments", "post-7")
	want := map[string][]byte{"thumb.png": []byte("thumb"), "photo.png": []byte("photo")}
	for _, a := range out {
		s.NotEmpty(a.LocalPath)
		s.Contains(a.LocalPath, postDir)
		content, err := os.ReadFile(a.LocalPath)
		s.NoError(err)
		s.Equal(want[a.Name], content)
	}
}