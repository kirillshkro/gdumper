// Package paginator determines the boundaries of the topic and generates
// URLs for page-by-page crawling.
//
// Paginator uses a fetcher to load HTML pages, and a parser
// (internal/parser) to recognize the structure, determine the last page
// and extract data. Per-page errors are not fatal (graceful degradation).
package paginator

import (
	"errors"
	"log/slog"

	"github.com/kirillshkro/gdumper/internal/fetcher"
	"github.com/kirillshkro/gdumper/internal/parser"
)

var (
	// ErrNoParser — the page is not recognized by the parser.
	ErrNoParser = errors.New("paginator: page not recognized by the parser")
	// ErrNotFound — the page does not exist (HTTP 404).
	ErrNotFound = errors.New("paginator: page not found (HTTP 404)")
	// ErrInvalidRange — the pages range is invalid.
	ErrInvalidRange = errors.New("paginator: invalid pages range")
)

// Page — one page of the topic.
type Page struct {
	// Number — page number (from 1).
	Number int
	// URL — URL of the page.
	URL string
}

// PageData — data parsed from one page.
// Error and Warning describe a non-fatal error of the page.
type PageData struct {
	// Number — page number.
	Number int
	// URL — the final URL of the page (canonical).
	URL string
	// Data — parsed page, nil on error.
	Data *parser.Page
	// Error — not nil on a fetch/parse error.
	Error error
	// Warning — a short message about a non-fatal error.
	Warning string
}

// Summary — summary of the crawl.
type Summary struct {
	// Total — total number of pages in the topic.
	Total int
	// Planned — number of pages planned for fetch (after applying the range).
	Planned int
	// Fetched — number of pages successfully parsed.
	Fetched int
	// Failed — number of failed pages (graceful degradation).
	Failed int
	// Errors — list of non-fatal errors.
	Errors []error
}

// cachedPage — cached data for the first page of the range
// (to avoid re-fetching the base page in Walk).
type cachedPage struct {
	number int
	url    string
	data   *parser.Page
	ok     bool
}

// Paginator — the page-by-page crawler of the forum topic.
type Paginator struct {
	fetcher   *fetcher.Client
	parser    parser.Parser
	base      string
	logger    *slog.Logger
	pageRange PageRange
	lastPage  int
	cached    cachedPage
}

// New creates a Paginator for a topic with the given base URL.
// The default pages range is "all" (from the first to the last page).
func New(fetcherClient *fetcher.Client, parserInstance parser.Parser, base string) *Paginator {
	return &Paginator{
		fetcher:   fetcherClient,
		parser:    parserInstance,
		base:      base,
		logger:    slog.Default(),
		pageRange: PageRange{start: 1, end: 0},
	}
}

// SetLogger — sets a structured logger.
func (p *Paginator) SetLogger(logger *slog.Logger) {
	if logger != nil {
		p.logger = logger
	}
}

// SetRange — parses and sets the pages range: "all", "N", "N-M", "N-".
func (p *Paginator) SetRange(raw string) error {
	rng, err := ParseRange(raw)
	if err != nil {
		return err
	}
	p.pageRange = rng
	return nil
}
