// Package fetcher implements a resilient HTTP client for loading forum
// HTML pages, with tracking of the final URL and redirect chain.
package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

var (
	// ErrTransport is a transport-level failure during a request.
	ErrTransport = errors.New("fetcher: transport error")
	// ErrReadBody is an error while reading the response body.
	ErrReadBody = errors.New("fetcher: failed to read the response body")
)

// Options are the Client initialization parameters.
type Options struct {
	// Timeout is the per-request timeout; 0 means no timeout.
	Timeout time.Duration
	// Logger is a structured logger; 0 means slog.Default().
	Logger *slog.Logger
}

// NewOptions returns the Options with default values.
func NewOptions() Options {
	return Options{
		Timeout: 30 * time.Second,
		Logger:  slog.Default(),
	}
}

// Response is the result of an HTTP request.
type Response struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// FinalURL is the final URL after resolving redirects.
	FinalURL string
	// Body is the response body.
	Body []byte
	// Redirects is the list of status codes in the redirect chain.
	Redirects []int
	// Redirected is whether any redirect occurred.
	Redirected bool
}

// IsHTTPError reports whether StatusCode is an HTTP error (>= 400).
func (r *Response) IsHTTPError() bool {
	return r.StatusCode >= 400
}

// Client is the HTTP client.
type Client struct {
	transport http.RoundTripper
	timeout   time.Duration
	logger    *slog.Logger
}

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

// New creates a Client from the given Options.
func New(opts Options) *Client {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		transport: &http.Transport{},
		timeout:   opts.Timeout,
		logger:    logger,
	}
}

// Fetch performs a GET request.
// err is non-nil only on a transport-level error; HTTP status codes
// are always returned in Response.StatusCode.
func (c *Client) Fetch(ctx context.Context, url string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetcher: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*")
	req.Header.Set("Accept-Language", "en")

	transport := &recordingRoundTripper{next: c.transport}
	client := &http.Client{
		Transport: transport,
		Timeout:   c.timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, fmt.Errorf("fetcher: %w: %w", ErrTransport, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetcher: %w: %w", ErrReadBody, err)
	}

	c.logger.Debug("fetch",
		"url", url,
		"status", resp.StatusCode,
		"final_url", transport.lastURL,
		"redirects", transport.redirects)

	return &Response{
		StatusCode: resp.StatusCode,
		FinalURL:   transport.lastURL,
		Body:       body,
		Redirects:  transport.redirects,
		Redirected: len(transport.redirects) > 0,
	}, nil
}