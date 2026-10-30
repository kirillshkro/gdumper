// job.go содержит скачивание одного вложения: обработка HTTP-ответа,
// повторные попытки, идемпотентность и выполнение запроса через rate-limit.
package downloader

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
)

// closeBody закрывает тело HTTP-ответа, логгируя ошибки закрытия
// (они не фатальны для дальнейшей обработки вложения).
func (d *Downloader) closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	if e := resp.Body.Close(); e != nil {
		d.logger.Debug("attachment: close body error", "error", e)
	}
}

// runJob скачивает одно вложение в j.dir под именем j.fileName.
//
// Возвращает абсолютный локальный путь и SHA-256 при успехе.
// Ошибки самого вложения (HTTP 4xx/5xx после повторов) логируются warning'ом
// и возвращаются ("", "", nil), чтобы обработать остальные. Не nil ошибка —
// фатальная для поста (отмена контекста/дедлайн, ошибка клиента).
func (d *Downloader) runJob(ctx context.Context, j *job) (localPath string, sha string, err error) {
	if j.url == "" {
		return "", "", nil
	}
	if j.fileName == "" {
		j.fileName = "attachment"
	}
	localPath = filepath.Join(j.dir, j.fileName)

	// Идемпотентность: существующий (не-каталог) файл пересчитывается локально
	// (SHA-256), сеть не вызывается при повторном запуске.
	if info, e := os.Stat(localPath); e == nil && !info.IsDir() {
		sha, e = d.hashFile(localPath)
		if e != nil {
			d.logger.Warn("attachment: failed to hash existing file", "path", localPath, "error", e)
			return "", "", nil
		}
		return localPath, sha, nil
	}

	if e := os.MkdirAll(j.dir, 0o755); e != nil {
		d.logger.Warn("attachment: failed to create dir", "dir", j.dir, "error", e)
		return "", "", nil
	}

	for attempt := 1; ; attempt++ {
		resp, e := d.doOnce(ctx, j.url)
		if e != nil {
			if errors.Is(e, context.Canceled) {
				return "", "", e
			}
			if attempt > d.retries {
				d.logger.Warn("attachment: transport error", "url", j.url, "error", e, "attempts", attempt)
				return "", "", nil
			}
			d.sleeper(attempt)
			continue
		}

		status := resp.StatusCode
		switch {
		case status == http.StatusNotFound:
			d.closeBody(resp)
			d.logger.Warn("attachment: not found", "url", j.url, "status", http.StatusNotFound)
			return "", "", nil
		case status == http.StatusTooManyRequests || status >= 500:
			d.closeBody(resp)
			if attempt > d.retries {
				d.logger.Warn("attachment: server error", "url", j.url, "status", status)
				return "", "", nil
			}
			d.sleeper(attempt)
			continue
		case status < 200 || status >= 300:
			d.closeBody(resp)
			d.logger.Warn("attachment: unexpected status", "url", j.url, "status", status)
			return "", "", nil
		}

		sha, e = d.streamToDisk(localPath, resp)
		d.closeBody(resp)
		if e != nil {
			if errors.Is(e, context.Canceled) {
				return "", "", e
			}
			d.logger.Warn("attachment: failed to write file", "path", localPath, "error", e)
			return "", "", nil
		}
		return localPath, sha, nil
	}
}

// doOnce выполняет один GET, применяя rate-limit через throttle
// и заголовок User-Agent из конфигурации.
func (d *Downloader) doOnce(ctx context.Context, url string) (*http.Response, error) {
	if e := d.throttle(ctx); e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if e != nil {
		return nil, e
	}
	if d.userAgent != "" {
		req.Header.Set("User-Agent", d.userAgent)
	}
	return d.client.Do(req)
}
