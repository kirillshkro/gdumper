// Package auth loads and manages the cookies used to authorize requests to
// forum pages.
//
// It supports the three sources parsed by internal/config as CookieSource:
//
//   - browser:<name> — cookies of a locally installed browser, read via
//     codeberg.org/sorl/browsercookie (chrome, firefox, edge, safari).
//   - file:<path>    — a Netscape-compatible cookies.txt file.
//   - raw:"name=value; ..." — an explicit cookie string.
//
// Load returns an http.CookieJar that the fetcher can attach to its
// http.Client so the loaded cookies are sent automatically on every request.
// Cookie values are never written to logs; only names and counts are
// reported.
package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/sorl/browsercookie"
	"github.com/kirillshkro/gdumper/internal/config"
)

var (
	// ErrUnknownCookieKind — unsupported cookies source kind.
	ErrUnknownCookieKind = errors.New("unsupported cookies source kind")
	// ErrEmptyBrowserSource — the browser cookie source is empty.
	ErrEmptyBrowserSource = errors.New("empty browser cookie source")
	// ErrUnknownBrowser — the browser name is not supported by the loader.
	ErrUnknownBrowser = errors.New("unsupported browser")
	// ErrBrowserNotFound — the browser or its cookie store is not available.
	ErrBrowserNotFound = errors.New("browser cookie store not found")
	// ErrBrowserLoadFailed — cookies could not be loaded from the browser.
	ErrBrowserLoadFailed = errors.New("failed to load browser cookies")
	// ErrFileNotFound — the cookies file does not exist.
	ErrFileNotFound = errors.New("cookies file not found")
	// ErrFileReadFailed — the cookies file could not be read.
	ErrFileReadFailed = errors.New("failed to read cookies file")
	// ErrEmptyRawSource — the raw cookies source is empty.
	ErrEmptyRawSource = errors.New("empty raw cookies source")
	// ErrInvalidCookiePair — a raw cookie pair is malformed.
	ErrInvalidCookiePair = errors.New("invalid cookie pair")
)

// Load builds a cookie jar for the given source.
//
// A nil source returns (nil, nil). The returned jar implements
// http.CookieJar and may be assigned to http.Client.Jar so the loaded
// cookies are sent automatically on every request.
//
// log is used only for safe, values-free diagnostics.
func Load(source *config.CookieSource, log *slog.Logger) (http.CookieJar, error) {
	if source == nil {
		return nil, nil
	}
	switch source.Kind {
	case config.CookieKindBrowser:
		return loadBrowser(source, log)
	case config.CookieKindFile:
		return loadNetscape(source, log)
	case config.CookieKindRaw:
		return loadRaw(source, log)
	default:
		return nil, fmt.Errorf("cookies: %w (kind=%q)", ErrUnknownCookieKind, source.Kind)
	}
}

// loadBrowser reads the cookies of the named browser via browsercookie.
func loadBrowser(source *config.CookieSource, log *slog.Logger) (http.CookieJar, error) {
	if source == nil || source.Browser == "" {
		return nil, ErrEmptyBrowserSource
	}
	bs, ok := browserFromName(source.Browser)
	if !ok {
		return nil, fmt.Errorf("cookies: %w (browser=%q)", ErrUnknownBrowser, source.Browser)
	}
	jar, err := browsercookie.ReadCookies(browsercookie.WithBrowser(bs))
	if err != nil {
		return nil, wrapBrowserErr(source.Browser, err)
	}
	if log != nil {
		log.Info("browser cookies loaded", "browser", source.Browser, "count", len(jar.All()))
	}
	return jar, nil
}

// wrapBrowserErr adds context to a browsercookie error.
func wrapBrowserErr(browser string, err error) error {
	switch {
	case errors.Is(err, browsercookie.ErrNotFound):
		return fmt.Errorf("%s: browser cookie store not found: %w", browser, browsercookie.ErrNotFound)
	case errors.Is(err, browsercookie.ErrDetection):
		return fmt.Errorf("%s: could not detect browser profile: %w", browser, browsercookie.ErrDetection)
	case errors.Is(err, browsercookie.ErrUnsupportedBrowser):
		return fmt.Errorf("%s: %w", browser, browsercookie.ErrUnsupportedBrowser)
	default:
		return fmt.Errorf("%s: failed to load cookies: %w", browser, err)
	}
}

// browserFromName maps the CLI-friendly name to a browsercookie.Browser.
func browserFromName(name string) (browsercookie.Browser, bool) {
	switch strings.ToLower(name) {
	case "chrome":
		return browsercookie.Chrome, true
	case "chromium":
		return browsercookie.Chromium, true
	case "firefox":
		return browsercookie.Firefox, true
	case "edge":
		return browsercookie.Edge, true
	case "safari":
		return browsercookie.Safari, true
	default:
		return browsercookie.Edge, false
	}
}

// loadNetscape reads a Netscape cookies.txt file into a cookie jar.
func loadNetscape(source *config.CookieSource, log *slog.Logger) (http.CookieJar, error) {
	if source == nil || source.Path == "" {
		return nil, fmt.Errorf("cookies: %w (empty file source)", ErrEmptyBrowserSource)
	}
	data, err := os.ReadFile(source.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("cookies: %w (path=%q)", ErrFileNotFound, source.Path)
		}
		return nil, fmt.Errorf("cookies: %w (path=%q): %v", ErrFileReadFailed, source.Path, err)
	}
	cookies, err := parseNetscape(data)
	if err != nil {
		return nil, fmt.Errorf("cookies: parse %q: %w", source.Path, err)
	}
	if log != nil {
		log.Info("file cookies loaded", "path", source.Path, "count", len(cookies))
	}
	return NewMemoryJar(cookies), nil
}

// loadRaw builds a cookie jar from explicit name=value pairs.
//
// Raw cookies carry no domain, so they are sent to every host.
func loadRaw(source *config.CookieSource, log *slog.Logger) (http.CookieJar, error) {
	if source == nil || len(source.Pairs) == 0 {
		return nil, ErrEmptyRawSource
	}
	cookies := make([]*http.Cookie, 0, len(source.Pairs))
	for _, pair := range source.Pairs {
		name, value, ok := splitCookiePair(pair)
		if !ok {
			return nil, fmt.Errorf("cookies: %w (pair=%q)", ErrInvalidCookiePair, pair)
		}
		cookies = append(cookies, &http.Cookie{Name: name, Value: value})
	}
	if log != nil {
		log.Info("raw cookies loaded", "count", len(cookies))
	}
	return NewMemoryJar(cookies), nil
}

// splitCookiePair splits "name=value" into its parts. The value may
// contain '=' characters.
func splitCookiePair(pair string) (name, value string, ok bool) {
	idx := strings.IndexByte(pair, '=')
	if idx <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(pair[:idx]), strings.TrimSpace(pair[idx+1:]), true
}

// parseNetscape parses Netscape cookies.txt lines into cookies.
//
// Format: domain<TAB>name<TAB>value[TAB]secure<TAB>path<TAB>expires[TAB]httpOnly
func parseNetscape(data []byte) ([]*http.Cookie, error) {
	lines := strings.Split(string(data), "\n")
	out := make([]*http.Cookie, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || line[0] == '#' {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		domain := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		if domain == "" || name == "" {
			continue
		}
		cookie := &http.Cookie{
			Domain: domain,
			Name:   name,
			Value:  trimQuotes(strings.TrimSpace(parts[2])),
			Path:   strings.TrimSpace(parts[4]),
		}
		if len(parts) > 3 {
			cookie.Secure = strings.EqualFold(strings.TrimSpace(parts[3]), "true")
		}
		if len(parts) > 5 {
			if n, err := strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64); err == nil {
				cookie.Expires = time.Unix(n, 0)
			}
		}
		if len(parts) > 6 {
			cookie.HttpOnly = strings.EqualFold(strings.TrimSpace(parts[6]), "true")
		}
		out = append(out, cookie)
	}
	return out, nil
}

// trimQuotes removes surrounding quotes from a value.
func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// memoryJar is a minimal in-memory cookie jar.
//
// Cookies with a Domain attribute are sent only to matching hosts (RFC 6265
// suffix + path-prefix matching); cookies without a domain are sent to every
// host. This mirrors the two source types: Netscape file cookies (with domain)
// vs raw cookies (without domain).
type memoryJar struct {
	mu      sync.RWMutex
	cookies []*http.Cookie
}

// NewMemoryJar returns a jar seeded with the given cookies.
func NewMemoryJar(cookies []*http.Cookie) *memoryJar {
	c := make([]*http.Cookie, 0, len(cookies))
	c = append(c, cookies...)
	return &memoryJar{cookies: c}
}

// Cookies returns the cookies applicable to the request URL.
func (m *memoryJar) Cookies(u *url.URL) []*http.Cookie {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	out := make([]*http.Cookie, 0, len(m.cookies))
	for _, c := range m.cookies {
		if expired(c, now) {
			continue
		}
		if !cookieMatches(c, u) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// SetCookies stores received cookies. It is invoked by http.Client after each
// response so dynamically-set cookies are kept for the session.
func (m *memoryJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cookies = append(m.cookies, cookies...)
}

// cookieMatches reports whether c applies to the request URL u.
func cookieMatches(c *http.Cookie, u *url.URL) bool {
	if c.Domain != "" && !domainMatch(hostWithoutPort(u), c.Domain) {
		return false
	}
	return pathMatch(u, c.Path)
}

// domainMatch reports whether reqHost is inside cookieDomain (RFC 6265 suffix
// match; a leading dot in cookieDomain is allowed).
func domainMatch(reqHost, cookieDomain string) bool {
	d := strings.ToLower(strings.TrimPrefix(cookieDomain, "."))
	if d == "" {
		return true
	}
	if reqHost == d {
		return true
	}
	return strings.HasSuffix(reqHost, "."+d)
}

// pathMatch reports whether u.Path starts with p.
func pathMatch(u *url.URL, p string) bool {
	if p == "" || p == "/" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(u.Path), strings.ToLower(p))
}

// hostWithoutPort returns the lowercased host without its port.
func hostWithoutPort(u *url.URL) string {
	host := u.Host
	if colon := strings.Index(host, ":"); colon >= 0 {
		host = host[:colon]
	}
	return strings.ToLower(host)
}

// expired reports whether c is past its Expiry. A zero time means session-cookie
// semantics and is never treated as expired.
func expired(c *http.Cookie, now time.Time) bool {
	return !c.Expires.IsZero() && now.After(c.Expires)
}
