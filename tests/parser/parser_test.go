package parser

import (
	"encoding/json"

	"github.com/kirillshkro/gdumper/internal/parser"
)

// TestParsePageSimple — TC-04.1: из простого поста извлечены author, date, body.
func (s *ParserSuite) TestParsePageSimple() {
	page, err := s.xenForo.ParsePage([]byte(xenForoPageHTML(samplePosts())), xenForoTestURL)
	s.Require().NoError(err)
	s.Require().Len(page.Messages, 2)
	s.Assert().Equal("alice", page.Messages[0].Author)
	s.Assert().Equal("2023-01-01T12:00:00Z", page.Messages[0].Date)
	s.Assert().Equal("Текст.", page.Messages[0].Content)
}

// TestParsePagePHPBB — TC-04.1: движок phpBB извлекает те же поля.
func (s *ParserSuite) TestParsePagePHPBB() {
	page, err := s.phpBB.ParsePage([]byte(phpbbPageHTML(samplePosts())), phpbbTestURL)
	s.Require().NoError(err)
	s.Require().Len(page.Messages, 2)
	s.Assert().Equal("bob", page.Messages[1].Author)
	s.Assert().Equal("2023-01-01T13:00:00Z", page.Messages[1].Date)
	s.Assert().Equal("Ответ.", page.Messages[1].Content)
}

// TestParsePageQuote — TC-04.2: у ответа заполнено parent_id.
func (s *ParserSuite) TestParsePageQuote() {
	page, err := s.xenForo.ParsePage([]byte(xenForoPageHTML(samplePosts())), xenForoTestURL)
	s.Require().NoError(err)
	s.Assert().Equal("1", page.Messages[1].ParentID)
}

// TestParsePageTreeLevels — TC-04.3: уровни вложенности 0, 1, 2.
func (s *ParserSuite) TestParsePageTreeLevels() {
	posts := append(samplePosts(), postSpec{
		id: "3", author: "carol", date: "2023-01-02T00:00:00Z",
		body: "Второй уровень.", number: 3, parent: "2",
	})
	page, err := s.xenForo.ParsePage([]byte(xenForoPageHTML(posts)), xenForoTestURL)
	s.Require().NoError(err)
	s.Require().Len(page.Messages, 3)
	s.Assert().Equal(0, page.Messages[0].Level)
	s.Assert().Equal(1, page.Messages[1].Level)
	s.Assert().Equal(2, page.Messages[2].Level)
}

// TestParsePageBBCode — TC-04.4: [quote] и [url] конвертируются в HTML.
func (s *ParserSuite) TestParsePageBBCode() {
	posts := []postSpec{{
		id: "1", author: "alice", date: "2023-01-01T12:00:00Z", number: 1,
		body: "[quote=Bob]привет[/quote] [url=https://example.com]ссылка[/url]",
	}}
	page, err := s.xenForo.ParsePage([]byte(xenForoPageHTML(posts)), xenForoTestURL)
	s.Require().NoError(err)
	content := page.Messages[0].Content
	s.Assert().Contains(content, `<blockquote><cite>Bob:</cite>привет</blockquote>`)
	s.Assert().Contains(content, `<a href="https://example.com">ссылка</a>`)
}

// TestParsePageUnicode — TC-04.5: юникод и эмодзи сохраняются без искажений.
func (s *ParserSuite) TestParsePageUnicode() {
	emoji := "Привет 🌍🔥"
	posts := []postSpec{{id: "1", author: "a", date: "2023-01-01T12:00:00Z", body: emoji, number: 1}}
	page, err := s.xenForo.ParsePage([]byte(xenForoPageHTML(posts)), xenForoTestURL)
	s.Require().NoError(err)
	s.Assert().Equal(emoji, page.Messages[0].Content)
}

// TestParsePageDeleted — TC-04.6: удалённый пост помечен, парсинг не падает.
func (s *ParserSuite) TestParsePageDeleted() {
	posts := []postSpec{{id: "1", author: "alice", date: "2023-01-01T12:00:00Z", number: 1, deleted: true}}
	page, err := s.xenForo.ParsePage([]byte(xenForoPageHTML(posts)), xenForoTestURL)
	s.Require().NoError(err)
	s.Require().Len(page.Messages, 1)
	s.Assert().True(page.Messages[0].Deleted)
}

// TestParsePageXSS — TC-04.7: XSS-инъекция экранируется в JSON.
func (s *ParserSuite) TestParsePageXSS() {
	posts := []postSpec{{id: "1", author: "alice", date: "2023-01-01T12:00:00Z", number: 1, body: `<script>alert(1)</script>`}}
	page, err := s.xenForo.ParsePage([]byte(xenForoPageHTML(posts)), xenForoTestURL)
	s.Require().NoError(err)
	encoded, err := json.Marshal(page.Messages[0])
	s.Require().NoError(err)
	s.Assert().NotContains(string(encoded), "<script>")
	s.Assert().Contains(string(encoded), `\u003cscript\u003e`)
}

// TestParsePageV2Markup — TC-04.8: v2-разметка не распознаётся (Match=false, ошибка).
func (s *ParserSuite) TestParsePageV2Markup() {
	v2 := `<html data-app="XenForoV2"><body><article class="message-v2"></article></body></html>`
	s.Assert().False(parser.Match([]byte(v2), xenForoTestURL))
	_, err := parser.ParsePage([]byte(v2), xenForoTestURL)
	s.Require().Error(err)
}

// TestMatchAndErrors — Match распознаёт движки; пустая/нераспознанная -> ошибки.
func (s *ParserSuite) TestMatchAndErrors() {
	s.Assert().True(parser.Match([]byte(xenForoPageHTML(samplePosts())), xenForoTestURL))
	s.Assert().True(parser.Match([]byte(phpbbPageHTML(samplePosts())), phpbbTestURL))
	s.Assert().False(parser.Match([]byte("<html>пусто</html>"), "http://x"))

	_, err := s.xenForo.ParsePage([]byte(xenForoPageHTML(nil)), xenForoTestURL)
	s.Require().Error(err)
	s.Assert().ErrorIs(err, parser.ErrEmptyPage)

	_, err = parser.ParsePage([]byte("<html>не форум</html>"), xenForoTestURL)
	s.Require().Error(err)
	s.Assert().ErrorIs(err, parser.ErrNoParser)
}

// TestLastPageAndPageURL — LastPage и PageURL для обоих движков.
func (s *ParserSuite) TestLastPageAndPageURL() {
	last, err := s.xenForo.LastPage([]byte(xenForoNavHTML(5)))
	s.Require().NoError(err)
	s.Assert().Equal(5, last)
	s.Assert().Equal("https://forum.example.com/threads/1/page-3", s.xenForo.PageURL(xenForoTestURL, 3))
	s.Assert().Equal(xenForoTestURL, s.xenForo.PageURL(xenForoTestURL, 1))

	last, err = s.phpBB.LastPage([]byte(phpbbNavHTML(7)))
	s.Require().NoError(err)
	s.Assert().Equal(7, last)
	s.Assert().Equal("https://forum.example.com/thread-67890.php?start=40", s.phpBB.PageURL(phpbbTestURL, 3))
}
