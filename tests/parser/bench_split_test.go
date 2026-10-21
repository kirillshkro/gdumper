package parser

import (
	"bytes"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// BenchmarkDOMOnly измеряет только стоимость построения DOM
// (goquery.NewDocumentFromReader) без Find/Render — нижняя граница
// стоимости парсинга одной страницы из 1000 постов.
func BenchmarkDOMOnly(b *testing.B) {
	posts := makePosts(1000)
	html := []byte(largePageHTML(posts))
	for b.Loop() {
		_, _ = goquery.NewDocumentFromReader(bytes.NewReader(html))
	}
}
