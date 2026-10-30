// stream.go содержит потоковое копирование HTTP-ответа в файл с вычислением
// SHA-256 и хеширование существующего локального файла.
package downloader

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
)

// streamToDisk копирует resp.Body в localPath кусками по d.chunkSize,
// вычисляя SHA-256. Память O(chunkSize), независимо от размера файла.
func (d *Downloader) streamToDisk(localPath string, resp *http.Response) (sha string, err error) {
	f, e := os.Create(localPath)
	if e != nil {
		return "", e
	}
	defer func() {
		if ce := f.Close(); ce != nil {
			d.logger.Debug("attachment: close file error", "path", localPath, "error", ce)
		}
		if err != nil {
			if re := os.Remove(localPath); re != nil {
				d.logger.Debug("attachment: remove file error", "path", localPath, "error", re)
			}
		}
	}()

	h := sha256.New()
	buf := make([]byte, d.chunkSize)
	for {
		n, e := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return "", werr
			}
			if _, herr := h.Write(buf[:n]); herr != nil {
				return "", herr
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
	}
	sha = hex.EncodeToString(h.Sum(nil))
	d.logger.Debug("attachment downloaded", "path", localPath, "sha256", sha)
	return sha, nil
}

// hashFile вычисляет SHA-256 существующего локального файла потоком.
func (d *Downloader) hashFile(path string) (sha string, err error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer func() {
		if ce := f.Close(); ce != nil {
			d.logger.Debug("attachment: close file error", "path", path, "error", ce)
		}
	}()

	h := sha256.New()
	buf := make([]byte, d.chunkSize)
	for {
		n, e := f.Read(buf)
		if n > 0 {
			if _, herr := h.Write(buf[:n]); herr != nil {
				return "", herr
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
	}
	sha = hex.EncodeToString(h.Sum(nil))
	return sha, nil
}
