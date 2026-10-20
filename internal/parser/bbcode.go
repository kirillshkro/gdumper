package parser

import (
	"html"
	"regexp"
	"strings"
)

// Регулярные выражения BB-code тегов; (?s) позволяет телу переносить строки.
var (
	bbQuoteAuthorRe = regexp.MustCompile(`(?s)\[quote=([^\]]*)\](.*?)\[/quote\]`)
	bbQuoteRe       = regexp.MustCompile(`(?s)\[quote\](.*?)\[/quote\]`)
	bbURLRe         = regexp.MustCompile(`(?s)\[url=([^\]]*)\](.*?)\[/url\]`)
)

// BBCodeToHTML конвертирует BB-code теги [quote] и [url] в HTML.
//
// Вложенные теги [quote] внутри [quote] не поддерживаются:
// преобразование останавливается на первом закрывающем теге.
func BBCodeToHTML(s string) string {
	s = bbQuoteAuthorRe.ReplaceAllStringFunc(s, func(match string) string {
		groups := bbQuoteAuthorRe.FindStringSubmatch(match)
		if len(groups) != 3 {
			return match
		}
		return quoteHTML(groups[1], groups[2])
	})
	s = bbQuoteRe.ReplaceAllStringFunc(s, func(match string) string {
		groups := bbQuoteRe.FindStringSubmatch(match)
		if len(groups) != 2 {
			return match
		}
		return quoteHTML("", groups[1])
	})
	s = bbURLRe.ReplaceAllStringFunc(s, func(match string) string {
		groups := bbURLRe.FindStringSubmatch(match)
		if len(groups) != 3 {
			return match
		}
		return urlHTML(groups[1], groups[2])
	})
	return s
}

// quoteHTML — HTML-представление цитаты.
func quoteHTML(author, inner string) string {
	if author == "" {
		return "<blockquote>" + inner + "</blockquote>"
	}
	return "<blockquote><cite>" + html.EscapeString(author) + ":</cite>" + inner + "</blockquote>"
}

// urlHTML — HTML-представление ссылки.
func urlHTML(href, inner string) string {
	if isUnsafeURL(href) {
		return inner
	}
	return `<a href="` + html.EscapeString(href) + `">` + inner + `</a>`
}

// isUnsafeURL — схемы ссылок, которые нельзя выводить как якорь.
func isUnsafeURL(href string) bool {
	normalized := strings.ToLower(strings.TrimSpace(href))
	return strings.HasPrefix(normalized, "javascript:") || strings.HasPrefix(normalized, "vbscript:")
}
