// Package downloader_test — SOCKS5 proxy tests for the attachment downloader.
//
// New applies the proxy to the internal client when none is supplied; malformed
// proxies fail with a netx parse/scheme error, valid ones construct and route
// download requests through the SOCKS5 proxy.
package downloader_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"time"

	"github.com/kirillshkro/gdumper/internal/downloader"
	"github.com/kirillshkro/gdumper/internal/model"
	"github.com/kirillshkro/gdumper/tests/socks5"
)

// TestProxyMalformedProxy — New rejects a proxy URL that is not a valid
// socks5/socks5h address (wrong scheme or unparseable).
func (s *DownloaderTestSuite) TestProxyMalformedProxy() {
	for _, proxy := range []string{
		"http://proxy.example.com:3128",
		"https://proxy.example.com:3128",
		"socks4://proxy.example.com:1080",
		"not-a-url",
		"socks5://host%zz", // invalid percent-encoding
	} {
		_, err := downloader.New(downloader.Config{
			Strategy: downloader.StrategyDownload,
			Proxy:    proxy,
		})
		s.Require().Error(err, "malformed proxy %q", proxy)
		s.Contains(err.Error(), "proxy", "proxy %q should mention the proxy in its error", proxy)
	}
}

// TestProxyValidConstruct — New builds a usable downloader for a valid socks5 proxy.
func (s *DownloaderTestSuite) TestProxyValidConstruct() {
	proxy, err := socks5.New(0)
	s.Require().NoError(err, "start socks5 server")
	defer proxy.Close()

	dl := s.newDownloader(downloader.Config{
		Strategy: downloader.StrategyDownload,
		Proxy:    "socks5://" + proxy.Addr(),
	})
	s.NotNil(dl, "New must return a usable downloader")
}

// TestProxyRouted — a download through a proxy'd downloader is actually
// tunneled by the SOCKS5 proxy and saved to disk (the proxy records the target).
func (s *DownloaderTestSuite) TestProxyRouted() {
	body := []byte("file-content")
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer ms.Close()

	proxy, err := socks5.New(0)
	s.Require().NoError(err, "start socks5 server")
	defer proxy.Close()

	dl := s.newDownloader(downloader.Config{
		Strategy: downloader.StrategyDownload,
		Proxy:    "socks5://" + proxy.Addr(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	atts, err := dl.Download(ctx, []*model.Attachment{
		newAttachment("msg-1", "img.png", ms.URL+"/img.png"),
	})
	s.Require().NoError(err, "download via socks5 proxy")
	s.Require().Len(atts, 1)
	s.Require().NotEqual("", atts[0].LocalPath, "file was not downloaded")
	s.Require().NotEqual("", atts[0].SHA256, "sha256 was not computed")

	f, err := os.Open(atts[0].LocalPath)
	s.Require().NoError(err, "open downloaded file")
	defer f.Close()
	data, err := io.ReadAll(f)
	s.Require().NoError(err, "read downloaded file")
	s.Equal(body, data, "file content mismatch")

	targetURL, perr := url.Parse(ms.URL)
	s.Require().NoError(perr, "parse target url")
	s.Require().Contains(proxy.Tunnels(), targetURL.Host, "proxy never tunneled the target %q", targetURL.Host)
}
