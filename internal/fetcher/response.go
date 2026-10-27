// Package fetcher implements a resilient HTTP client for loading forum
// HTML pages, with tracking of the final URL and redirect chain.
//
// The client retries transient failures (HTTP 429/5xx and transport
// errors) with exponential backoff and jitter, honours the Retry-After
// response header and enforces a minimum interval between consecutive
// requests (RPS limit).
package fetcher

import "time"

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
	// RetryAfter is the parsed Retry-After response header value,
	// zero when the header is absent or unparseable.
	RetryAfter time.Duration
}

// IsHTTPError reports whether StatusCode is an HTTP error (>= 400).
func (r *Response) IsHTTPError() bool {
	return r.StatusCode >= 400
}
