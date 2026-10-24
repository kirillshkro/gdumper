package paginator

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kirillshkro/gdumper/internal/parser"
)

// LastPage — loads the first page of the range and returns
// the number of the topic's last page.
// Subsequent calls use the cache and don't make network requests.
func (p *Paginator) LastPage(ctx context.Context) (int, error) {
	if p.cached.ok {
		return p.lastPage, nil
	}
	start := p.pageRange.start
	if start < 1 {
		start = 1
	}
	pageURL := p.parser.PageURL(p.base, start)
	html, finalURL, status, err := p.fetchPage(ctx, pageURL)
	if err != nil {
		return 0, fmt.Errorf("paginator: page %d failed to load: %w", start, err)
	}
	if status >= http.StatusBadRequest {
		return 0, fmt.Errorf("paginator: page %d unavailable (HTTP %d)", start, status)
	}
	data, err := p.parsePage(html, finalURL)
	if err != nil {
		return 0, fmt.Errorf("paginator: page %d: %w", start, err)
	}
	last, err := p.parser.LastPage(html)
	if err != nil {
		return 0, fmt.Errorf("paginator: failed to determine last page: %w", err)
	}
	if last < 1 {
		last = 1
	}
	p.cached = cachedPage{number: start, url: finalURL, data: data, ok: true}
	p.lastPage = last
	p.logger.Info("pagination resolved",
		"start_page", start,
		"last_page", last,
		"base_url", finalURL)
	return last, nil
}

// Walk — resolves the pages range, fetches and parses each page.
// Per-page errors do not stop the traversal (graceful degradation, TC-03.4).
// Returns a Summary and a list of PageData (one per planned page).
func (p *Paginator) Walk(ctx context.Context) (Summary, []PageData) {
	last, err := p.LastPage(ctx)
	if err != nil {
		return Summary{Errors: []error{err}}, nil
	}
	pageNumbers, err := p.pageRange.Resolve(last)
	if err != nil {
		return Summary{Errors: []error{err}}, nil
	}
	summary := Summary{Total: last, Planned: len(pageNumbers)}
	results := make([]PageData, 0, len(pageNumbers))
	for _, n := range pageNumbers {
		var result PageData
		var pageErr error
		if n == p.cached.number && p.cached.ok {
			result = PageData{
				Number: n,
				URL:    p.cached.url,
				Data:   p.cached.data,
			}
		} else {
			result, pageErr = p.walkPage(ctx, n)
			if pageErr != nil {
				summary.Failed++
				summary.Errors = append(summary.Errors, pageErr)
				p.logger.Warn("page failed, skipping",
					"page", n,
					"url", result.URL,
					"error", pageErr)
			}
		}
		if pageErr == nil {
			summary.Fetched++
		}
		results = append(results, result)
	}
	return summary, results
}

// walkPage — fetches and parses a single page.
// On page error, returns PageData with Error set.
func (p *Paginator) walkPage(ctx context.Context, n int) (PageData, error) {
	pageURL := p.parser.PageURL(p.base, n)
	html, finalURL, status, err := p.fetchPage(ctx, pageURL)
	result := PageData{Number: n, URL: finalURL}
	var pageErr error
	switch {
	case err != nil:
		pageErr = fmt.Errorf("page %d: network error: %w", n, err)
		result.Error = pageErr
		result.URL = pageURL
	case status >= http.StatusBadRequest:
		if status == http.StatusNotFound {
			pageErr = ErrNotFound
		} else {
			pageErr = fmt.Errorf("page %d: HTTP %d", n, status)
		}
		result.Error = pageErr
		result.Warning = pageErr.Error()
	default:
		data, parseErr := p.parsePage(html, finalURL)
		if parseErr != nil {
			pageErr = fmt.Errorf("page %d: %w", n, parseErr)
			result.Error = pageErr
			result.Warning = pageErr.Error()
		} else {
			result.Data = data
		}
	}
	return result, pageErr
}

// parsePage — parses a page, checking whether the parser recognizes it.
func (p *Paginator) parsePage(html []byte, finalURL string) (*parser.Page, error) {
	if !p.parser.Match(html, finalURL) {
		return nil, ErrNoParser
	}
	return p.parser.ParsePage(html, finalURL)
}

// fetchPage — loads a page.
// err is non-nil only on a transport-level error;
// the HTTP status code is in status.
func (p *Paginator) fetchPage(ctx context.Context, pageURL string) ([]byte, string, int, error) {
	resp, err := p.fetcher.Fetch(ctx, pageURL)
	if err != nil {
		return nil, pageURL, 0, err
	}
	return resp.Body, resp.FinalURL, resp.StatusCode, nil
}