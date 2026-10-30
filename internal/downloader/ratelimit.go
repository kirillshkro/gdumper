// ratelimit.go содержит rate-limit (минимальный интервал между запросами)
// и экспоненциальный backoff перед повторными попытками.
package downloader

import (
	"context"
	"time"
)

// throttle обеспечивает не менее d.delay между соседними запросами.
func (d *Downloader) throttle(ctx context.Context) error {
	if d.delay <= 0 {
		return nil
	}
	d.mu.Lock()
	now := time.Now()
	wait := d.delay - now.Sub(d.lastRequest)
	if wait < 0 {
		wait = 0
	}
	d.lastRequest = now
	d.mu.Unlock()
	select {
	case <-time.After(wait):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// sleeper — экспоненциальный backoff перед следующим attempt.
func (d *Downloader) sleeper(attempt int) {
	delay := d.retryBase
	for i := 1; i < attempt; i++ {
		delay *= 2
	}
	time.Sleep(delay)
}
