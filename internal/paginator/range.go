package paginator

import (
	"fmt"
	"strconv"
	"strings"
)

// PageRange — the range of pages to crawl.
// start is from 1; end is 0 — "no explicit upper bound"
// (clamped to the last page).
type PageRange struct {
	start int
	end   int
}

// Resolve — returns a slice of page numbers in the range,
// clamped to lastPage. If start exceeds lastPage,
// returns an empty slice (not an error).
func (r PageRange) Resolve(lastPage int) ([]int, error) {
	if lastPage < 1 {
		return nil, ErrInvalidRange
	}
	start := r.start
	if start < 1 {
		start = 1
	}
	end := r.end
	if end <= 0 {
		end = lastPage
	}
	if start > lastPage || start > end {
		return []int{}, nil
	}
	if end > lastPage {
		end = lastPage
	}
	out := make([]int, 0, end-start+1)
	for n := start; n <= end; n++ {
		out = append(out, n)
	}
	return out, nil
}

// ParseRange — parses the pages flag value: "all", "N", "N-M", "N-".
func ParseRange(raw string) (PageRange, error) {
	raw = strings.TrimSpace(raw)
	if raw == "all" {
		return PageRange{start: 1, end: 0}, nil
	}
	if n, err := strconv.Atoi(raw); err == nil && n >= 1 {
		return PageRange{start: n, end: 0}, nil
	}
	dash := strings.IndexByte(raw, '-')
	if dash > 0 {
		start, err := strconv.Atoi(raw[:dash])
		if err != nil || start < 1 {
			return PageRange{}, fmt.Errorf("%w: %q", ErrInvalidRange, raw)
		}
		endStr := raw[dash+1:]
		if endStr == "" {
			return PageRange{start: start, end: 0}, nil
		}
		end, err := strconv.Atoi(endStr)
		if err != nil || end < 1 {
			return PageRange{}, fmt.Errorf("%w: %q", ErrInvalidRange, raw)
		}
		if start > end {
			return PageRange{}, fmt.Errorf("%w: %q", ErrInvalidRange, raw)
		}
		return PageRange{start: start, end: end}, nil
	}
	return PageRange{}, fmt.Errorf("%w: %q", ErrInvalidRange, raw)
}