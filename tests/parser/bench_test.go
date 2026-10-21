package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kirillshkro/gdumper/internal/parser"
)

// makePosts создаёт список из n постов-описаний.
func makePosts(n int) []postSpec {
	posts := make([]postSpec, 0, n)
	for i := 0; i < n; i++ {
		spec := postSpec{
			id:     fmt.Sprintf("%d", i),
			author: fmt.Sprintf("Author %d", i),
			date:   "2023-01-01T12:00:00Z",
			body:   fmt.Sprintf("Body text for post %d.", i),
			number: i + 1,
		}
		// Каждый четвёртый пост — ответ на предыдущий (уровень 1).
		if i > 0 && i%4 == 0 {
			spec.parent = fmt.Sprintf("%d", i-1)
		}
		posts = append(posts, spec)
	}
	return posts
}

// largePageHTML собирает HTML-страницу XenForo с n постами для бенчмарка.
func largePageHTML(posts []postSpec) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html lang="ru" data-app="XenForo">`)
	sb.WriteString(`<head><title>Бench</title></head><body id="app">`)
	sb.WriteString(`<h1 class="p-title-title">Бench</h1>`)
	for _, p := range posts {
		parent := ""
		if p.parent != "" {
			parent = fmt.Sprintf(` data-parent="%s"`, p.parent)
		}
		sb.WriteString(fmt.Sprintf(
			`<article class="message" data-thread="1" data-content="%s"%s>`, p.id, parent))
		sb.WriteString(fmt.Sprintf(
			`<h3 class="message-userName"><a href="/members/1/%s">%s</a></h3>`, p.author, p.author))
		sb.WriteString(`<time class="date" datetime="2023-01-01T12:00:00Z">01.01.2023</time>`)
		sb.WriteString(fmt.Sprintf(`<div class="message-body">%s</div>`, p.body))
		sb.WriteString(fmt.Sprintf(`<a class="message-link" href="/threads/1/#post-%s">#%d</a>`, p.id, p.number))
		sb.WriteString(`</article>`)
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}

// BenchmarkParsePage измеряет время парсинга страницы из 1000 постов.
// Цель: < 5ms на страницу.
func BenchmarkParsePage(b *testing.B) {
	posts := makePosts(1000)
	html := largePageHTML(posts)

	b.ReportAllocs()
	for b.Loop() {
		_, err := parser.ParsePage([]byte(html), "https://forum.example.com/threads/1/")
		if err != nil {
			b.Fatalf("не удалось распарсить страницу: %v", err)
		}
	}
}
