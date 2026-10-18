package config

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrUnknownCookieKind - ошибка неизвестного вида источника cookies
	ErrUnknownCookieKind = errors.New("unknown cookies source kind")

	// ErrUnknownBrowser - ошибка неизвестного браузера
	ErrUnknownBrowser = errors.New("unknown browser")

	// ErrEmptyCookieSource - ошибка пустого источника cookies
	ErrEmptyCookieSource = errors.New("empty cookies source")

	// ErrInvalidCookiePair - ошибка невалидной пары cookie
	ErrInvalidCookiePair = errors.New("invalid cookie pair")
)

// supportedBrowsers - поддерживаемые браузеры (ТЗ 2.1)
var supportedBrowsers = map[string]bool{
	"chrome":  true,
	"firefox": true,
	"edge":    true,
	"safari":  true,
}

// CookieKind - тип источника cookies
type CookieKind string

// Виды источников cookies (Description.md: разбор cookies)
const (
	// CookieKindBrowser - cookies из браузера
	CookieKindBrowser CookieKind = "browser"
	// CookieKindFile - файл cookies в формате Netscape
	CookieKindFile CookieKind = "file"
	// CookieKindRaw - raw-строка cookies
	CookieKindRaw CookieKind = "raw"
)

// CookieSource - разобранный источник cookies
type CookieSource struct {
	Kind    CookieKind
	Browser string
	Path    string
	Pairs   []string
}

// ParseCookies разбирает строку-источник cookies.
// Поддерживаемые форматы: browser:chrome|firefox|edge|safari,
// file:/path/to/cookies.txt, raw:"k=v; k2=v2".
// Для пустого источника возвращает nil без ошибки.
func ParseCookies(source string) (*CookieSource, error) {
	if source == "" {
		return nil, nil
	}
	kind, rest, ok := strings.Cut(source, ":")
	if !ok {
		return nil, fmt.Errorf("%q: %w", source, ErrUnknownCookieKind)
	}
	switch CookieKind(kind) {
	case CookieKindBrowser:
		return parseBrowserSource(rest)
	case CookieKindFile:
		return parseFileSource(rest)
	case CookieKindRaw:
		return parseRawSource(rest)
	default:
		return nil, fmt.Errorf("%q: %w", kind, ErrUnknownCookieKind)
	}
}

// parseBrowserSource разбирает источник "browser:<имя>"
func parseBrowserSource(browser string) (*CookieSource, error) {
	if !supportedBrowsers[browser] {
		return nil, fmt.Errorf("%q: %w", browser, ErrUnknownBrowser)
	}
	return &CookieSource{Kind: CookieKindBrowser, Browser: browser}, nil
}

// parseFileSource разбирает источник "file:/path/to/cookies.txt"
func parseFileSource(path string) (*CookieSource, error) {
	if path == "" {
		return nil, fmt.Errorf("file: %w", ErrEmptyCookieSource)
	}
	return &CookieSource{Kind: CookieKindFile, Path: path}, nil
}

// parseRawSource разбирает источник "raw:\"k=v; k2=v2\""
func parseRawSource(raw string) (*CookieSource, error) {
	raw = strings.TrimSpace(strings.Trim(raw, `"`))
	if raw == "" {
		return nil, fmt.Errorf("raw: %w", ErrEmptyCookieSource)
	}
	var pairs []string
	for _, pair := range strings.Split(raw, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		if !strings.Contains(pair, "=") {
			return nil, fmt.Errorf("%q: %w", pair, ErrInvalidCookiePair)
		}
		pairs = append(pairs, pair)
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("raw: %w", ErrEmptyCookieSource)
	}
	return &CookieSource{Kind: CookieKindRaw, Pairs: pairs}, nil
}

// CookieSource возвращает разобранный источник cookies
// (nil, если --cookies не задан)
func (c *Config) CookieSource() (*CookieSource, error) {
	return ParseCookies(c.Cookies)
}