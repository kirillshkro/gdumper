package fetcher

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const (
	// maxBackoff is the upper bound of the exponential backoff delay.
	maxBackoff = 10 * time.Second
	// minBackoff is the lower bound of the backoff delay.
	minBackoff = 100 * time.Millisecond
)

// recordingRoundTripper records the redirect chain in local fields
// and forwards requests to the shared transport (connection pooling).
type recordingRoundTripper struct {
	next      http.RoundTripper
	lastURL   string
	redirects []int
}

// RoundTrip records the request URL and 3xx status codes.
func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.lastURL = req.URL.String()
	resp, err := r.next.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		r.redirects = append(r.redirects, resp.StatusCode)
	}
	return resp, nil
}

// statusIsRetryable reports whether the response status is transient
// (HTTP 429 and 5xx).
func statusIsRetryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

// backoffDelay returns the backoff delay for the given attempt number.
// attempt is 1-based: the first retry waits base, the second 2*base, etc.
func backoffDelay(attempt int, base time.Duration) time.Duration {
	d := base
	for i := 1; i < attempt; i++ {
		d *= 2
		if d > maxBackoff {
			d = maxBackoff
		}
	}
	if d < minBackoff {
		d = minBackoff
	}
	return d
}

// jitter adds a random component (up to 1/8 of base) to avoid thundering herd.
func jitter(base time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(base))) / 8
}

// retryAfter parses the Retry-After header: an integer number of seconds
// or an HTTP date. It returns (0, false) when the header is absent or
// unparseable.
func retryAfter(headers http.Header) (time.Duration, bool) {
	v := headers.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
		return time.Duration(n) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d, true
		}
	}
	return 0, false
}

// retryDelay computes the pause before the next attempt: the greater of
// the backoff and the Retry-After header, plus jitter.
func (c *Client) retryDelay(attempt int, resp *Response) time.Duration {
	d := backoffDelay(attempt, c.retryBase)
	if resp != nil && statusIsRetryable(resp.StatusCode) {
		if resp.RetryAfter > d {
			d = resp.RetryAfter
		}
	}
	d += jitter(c.retryBase)
	if d < minBackoff {
		d = minBackoff
	}
	return d
}

// throttle enforces the minimum interval between consecutive requests
// (RPS limit controlled by the Delay option).
func (c *Client) throttle(ctx context.Context) error {
	for {
		c.mu.Lock()
		need := c.delay - time.Since(c.lastRequest)
		if need <= 0 {
			c.lastRequest = time.Now()
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		select {
		case <-time.After(need):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
