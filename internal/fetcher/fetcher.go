package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/kirillshkro/gdumper/internal/config"
)

var (
	// ErrTransport is a transport-level failure during a request.
	ErrTransport = errors.New("fetcher: transport error")
	// ErrReadBody is an error while reading the response body.
	ErrReadBody = errors.New("fetcher: failed to read the response body")
)

// Client is the HTTP client with retries and rate limiting.
type Client struct {
	transport   http.RoundTripper
	timeout     time.Duration
	logger      *slog.Logger
	delay       time.Duration
	retry       int
	retryBase   time.Duration
	userAgent   string
	headers     map[string]string
	jar         http.CookieJar
	mu          sync.Mutex
	lastRequest time.Time
}

// New creates a Client from the given FetchOptions.
func New(opts config.FetchOptions) *Client {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	base := opts.RetryBase
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	retries := opts.Retry
	if retries < 0 {
		retries = 0
	}
	c := &Client{
		transport: &http.Transport{},
		timeout:   opts.Timeout,
		logger:    logger,
		delay:     opts.Delay,
		retry:     retries,
		retryBase: base,
		userAgent: opts.UserAgent,
		headers:   opts.Headers,
		jar:       opts.Jar,
	}
	// Cookie values are never logged; only the attachment itself is
	// reported.
	if c.jar != nil {
		c.logger.Debug("cookie jar attached to fetch client")
	}
	return c
}

// Fetch performs a GET request with retries on transient failures.
// err is non-nil only on a transport-level failure after all attempts;
// HTTP status codes are always returned in Response.StatusCode.
func (c *Client) Fetch(ctx context.Context, url string) (*Response, error) {
	maxAttempt := c.retry + 1
	for attempt := 1; ; attempt++ {
		resp, err := c.fetchOnce(ctx, url)
		// context.Canceled is a graceful shutdown (SIGINT): no retries.
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		// read error: a partial body is available, return it as-is.
		if resp != nil && err != nil {
			return resp, err
		}
		// success or non-retriable status: return the Response as is.
		if err == nil && !statusIsRetryable(resp.StatusCode) {
			return resp, nil
		}
		// all attempts exhausted.
		if attempt >= maxAttempt {
			if err != nil {
				return nil, fmt.Errorf("fetcher: %w (after %d attempts): %w",
					ErrTransport, attempt, err)
			}
			// retriable status (429/5xx): keep the last status code.
			return resp, nil
		}
		// pause before the next attempt: backoff + Retry-After + jitter.
		select {
		case <-time.After(c.retryDelay(attempt, resp)):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// fetchOnce performs a single GET request without retries.
func (c *Client) fetchOnce(ctx context.Context, url string) (*Response, error) {
	if err := c.throttle(ctx); err != nil {
		return nil, err
	}
	// Apply the per-request timeout to the request context.
	reqCtx := ctx
	var cancelTimeout context.CancelFunc
	if c.timeout > 0 {
		reqCtx, cancelTimeout = context.WithTimeout(ctx, c.timeout)
	}
	defer func() {
		if cancelTimeout != nil {
			cancelTimeout()
		}
	}()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetcher: %w", err)
	}
	for name, value := range c.headers {
		req.Header.Set(name, value)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*")
	req.Header.Set("Accept-Language", "en")

	transport := &recordingRoundTripper{next: c.transport}
	// Jar is shared by all requests of this Client: cookies loaded by the
	// auth module are attached automatically to each request (http.Client
	// calls jar.Cookies before the RoundTrip), and Set-Cookie responses are
	// stored for subsequent requests (jar.SetCookies after each response).
	client := &http.Client{Transport: transport, Jar: c.jar}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, fmt.Errorf("fetcher: %w: %w", ErrTransport, err)
	}
	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	out := &Response{
		StatusCode: resp.StatusCode,
		FinalURL:   transport.lastURL,
		Body:       body,
		Redirects:  transport.redirects,
		Redirected: len(transport.redirects) > 0,
	}
	if ra, ok := retryAfter(resp.Header); ok {
		out.RetryAfter = ra
	}
	if readErr != nil {
		// Read error is the primary failure; the partial body is
		// preserved for the caller (partial dump).
		return out, fmt.Errorf("fetcher: %w: %w", ErrReadBody, readErr)
	}
	if closeErr != nil {
		// A close error after a full read means the server closed the
		// connection improperly.
		return out, fmt.Errorf("fetcher: %w: %w", ErrReadBody, closeErr)
	}

	c.logger.Debug("fetch",
		"url", url,
		"status", resp.StatusCode,
		"final_url", transport.lastURL,
		"redirects", transport.redirects)

	return out, nil
}
