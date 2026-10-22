package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kirillshkro/gdumper/internal/parser"
	"github.com/stretchr/testify/suite"
)

// xenForoTestURL — URL тестовой темы XenForo.
const xenForoTestURL = "https://forum.example.com/threads/1/"

// phpbbTestURL — URL тестовой темы phpBB.
const phpbbTestURL = "https://forum.example.com/thread-67890.php"

// postSpec — описание поста для сборки тестовой страницы.
type postSpec struct {
	id          string
	parent      string
	author      string
	date        string
	body        string
	number      int
	deleted     bool
	attachments []attachmentSpec
}

// attachmentSpec — вложение для сборки тестовой страницы.
type attachmentSpec struct {
	name   string
	url    string
	mime   string
	size   string
}

// ParserSuite — тестовый комплект для проверки пакетного API парсеров (TC-04).
type ParserSuite struct {
	suite.Suite
	xenForo *parser.XenForoParser
	phpBB   *parser.PHPBBParser
}

// SetupTest готовит парсеры движков перед каждым тестом.
func (s *ParserSuite) SetupTest() {
	s.xenForo = parser.NewXenForo()
	s.phpBB = parser.NewPHPBB()
}

// TestParserSuite запускает все тесты парсеров.
func TestParserSuite(t *testing.T) {
	suite.Run(t, new(ParserSuite))
}

// samplePosts — пара тестовых постов: ответ на первый.
func samplePosts() []postSpec {
	return []postSpec{
		{id: "1", author: "alice", date: "2023-01-01T12:00:00Z", body: "Текст.", number: 1},
		{id: "2", author: "bob", date: "2023-01-01T13:00:00Z", body: "Ответ.", number: 2, parent: "1"},
	}
}

// attachmentAttrs — атрибуты data-mime/data-size для сборки тестовой страницы.
func attachmentAttrs(a attachmentSpec) string {
	var parts []string
	if a.mime != "" {
		parts = append(parts, fmt.Sprintf(`data-mime="%s"`, a.mime))
	}
	if a.size != "" {
		parts = append(parts, fmt.Sprintf(`data-size="%s"`, a.size))
	}
	return strings.Join(parts, " ")
}

// xenForoPageHTML собирает HTML-страницу XenForo из описанных постов.
func xenForoPageHTML(posts []postSpec) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html lang="ru" data-app="XenForo">`)
	sb.WriteString(`<head><title>Тестовая тема</title></head><body id="app">`)
	sb.WriteString(`<h1 class="p-title-title">Тестовая тема</h1>`)
	for _, p := range posts {
		parent := ""
		if p.parent != "" {
			parent = fmt.Sprintf(` data-parent="%s"`, p.parent)
		}
		body, class := p.body, "message-body"
		if p.deleted {
			body, class = "", "message-body deleted"
		}
		fmt.Fprintf(&sb, `<article class="message" data-thread="1" data-content="%s"%s>`, p.id, parent)
		fmt.Fprintf(&sb, `<h3 class="message-userName"><a href="/members/1/%s">%s</a></h3>`, p.author, p.author)
		fmt.Fprintf(&sb, `<time class="date" datetime="%s">01.01.2023</time>`, p.date)
		fmt.Fprintf(&sb, `<div class="%s">%s</div>`, class, body)
		fmt.Fprintf(&sb, `<a class="message-link" href="/threads/1/#post-%s">#%d</a>`, p.id, p.number)
		for _, a := range p.attachments {
			fmt.Fprintf(&sb, `<div class="message-attachments"><a class="attachment" href="%s"%s>%s</a></div>`, a.url, attachmentAttrs(a), a.name)
		}
		sb.WriteString(`</article>`)
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}

// phpbbPageHTML собирает HTML-страницу phpBB из описанных постов.
func phpbbPageHTML(posts []postSpec) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html lang="ru" id="phpbb">`)
	sb.WriteString(`<head><title>Тестовая тема</title></head><body class="section-topic">`)
	sb.WriteString(`<h1 class="topic-title">Тестовая тема</h1>`)
	for _, p := range posts {
		parent := ""
		if p.parent != "" {
			parent = fmt.Sprintf(` data-parent="%s"`, p.parent)
		}
		body, class := p.body, "content"
		if p.deleted {
			body, class = "", "content deleted"
		}
		fmt.Fprintf(&sb, `<div class="post" data-thread="67890" data-content="%s"%s>`, p.id, parent)
		fmt.Fprintf(&sb, `<span class="username"><a href="member.php?u=1">%s</a></span>`, p.author)
		fmt.Fprintf(&sb, `<span class="postdate" datetime="%s">01.01.2023</span>`, p.date)
		fmt.Fprintf(&sb, `<div class="%s">%s</div>`, class, body)
		fmt.Fprintf(&sb, `<a class="post-link" href="thread-67890.php#p%s">#%d</a>`, p.id, p.number)
		for _, a := range p.attachments {
			fmt.Fprintf(&sb, `<div class="post-attachments"><a class="attachment" href="%s"%s>%s</a></div>`, a.url, attachmentAttrs(a), a.name)
		}
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}

// xenForoNavHTML — минимальная страница XenForo с навигацией на n страниц.
func xenForoNavHTML(pages int) string {
	var sb strings.Builder
	sb.WriteString(`<html lang="ru" data-app="XenForo"><body id="app">`)
	for p := 1; p <= pages; p++ {
		fmt.Fprintf(&sb, `<a class="p-nav-link" href="/threads/1/page-%d">%d</a>`, p, p)
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}

// phpbbNavHTML — минимальная страница phpBB с навигацией на n страниц.
func phpbbNavHTML(pages int) string {
	var sb strings.Builder
	sb.WriteString(`<html lang="ru" id="phpbb"><body class="section-topic">`)
	for p := 1; p <= pages; p++ {
		fmt.Fprintf(&sb, `<span class="page-number">%d</span>`, p)
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}
