package parser

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kirillshkro/gdumper/internal/model"
)

// Маркеры, по которым распознаётся разметка phpBB.
var phpBBMarkers = [][]byte{
	[]byte(`id="phpbb"`),
	[]byte(`class="postdate"`),
}

// phpBBTopicIDRe — ID темы в URL вида thread-67890.php.
var phpBBTopicIDRe = regexp.MustCompile(`thread-(\d+)\.php`)

// phpBBPageSize — постов на странице (для расчёта ?start=N).
const phpBBPageSize = 20

// PHPBBParser — парсер тем движка phpBB.
type PHPBBParser struct{}

// NewPHPBB возвращает парсер движка phpBB.
func NewPHPBB() *PHPBBParser { return &PHPBBParser{} }

// Match возвращает true, если разметка страницы похожа на phpBB.
func (p *PHPBBParser) Match(html []byte, _ string) bool {
	for _, marker := range phpBBMarkers {
		if bytes.Contains(html, marker) {
			return true
		}
	}
	return false
}

// ParsePage извлекает данные темы со страницы phpBB.
func (p *PHPBBParser) ParsePage(html []byte, pageURL string) (*Page, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parser: не удалось разобрать HTML: %w", err)
	}

	topic := &model.Topic{URL: pageURL}
	topic.Title = strings.TrimSpace(doc.Find("h1.topic-title").First().Text())
	if topic.Title == "" {
		topic.Title = strings.TrimSpace(doc.Find("title").First().Text())
	}
	topic.ID = phpBBTopicID(pageURL)
	topic.TotalPages = p.lastPageFromDoc(doc)

	messages := p.messages(doc, pageURL)
	if len(messages) == 0 {
		return nil, ErrEmptyPage
	}
	topic.Author = messages[0].Author
	topic.Created = messages[0].Date
	assignLevels(messages)

	return &Page{Topic: topic, Messages: messages}, nil
}

// LastPage возвращает номер последней страницы темы.
func (p *PHPBBParser) LastPage(html []byte) (int, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return 0, fmt.Errorf("parser: не удалось разобрать HTML: %w", err)
	}
	return p.lastPageFromDoc(doc), nil
}

// PageURL возвращает URL n-й страницы темы phpBB.
func (p *PHPBBParser) PageURL(base string, n int) string {
	if n <= 1 {
		return base
	}
	return fmt.Sprintf("%s?start=%d", strings.SplitN(base, "?", 2)[0], (n-1)*phpBBPageSize)
}

// lastPageFromDoc — максимум номера страницы в навигации (без навигации — 1).
func (p *PHPBBParser) lastPageFromDoc(doc *goquery.Document) int {
	last := 1
	doc.Find("span.page-number, a.page-number").Each(func(_ int, sel *goquery.Selection) {
		number, err := strconv.Atoi(strings.TrimSpace(sel.Text()))
		if err == nil && number > last {
			last = number
		}
	})
	return last
}

// messages извлекает список сообщений со страницы.
func (p *PHPBBParser) messages(doc *goquery.Document, pageURL string) []*model.Message {
	var messages []*model.Message
	doc.Find("div.post").Each(func(_ int, sel *goquery.Selection) {
		id, _ := sel.Attr("data-content")
		topicID, _ := sel.Attr("data-thread")
		parentID, _ := sel.Attr("data-parent")
		datetime, _ := sel.Find("span.postdate").First().Attr("datetime")
		body := sel.Find("div.content").First()
		link := sel.Find("a.post-link").First()
		href, _ := link.Attr("href")

		bodyHTML := BBCodeToHTML(innerHTML(body))
		msg := &model.Message{
			ID:         id,
			TopicID:    topicID,
			Author:     authorFromLink(sel.Find("span.username a"), pageURL),
			Date:       datetime,
			BodyHTML:   bodyHTML,
			BodyText:   plainText(bodyHTML),
			Link:       absURL(href, pageURL),
			PostNumber: postNumberFromText(link.Text()),
			ParentID:   parentID,
			Deleted:    body.HasClass("deleted"),
		}
		msg.Attachments = parseAttachments(sel, "div.post-attachments a.attachment", pageURL)
		for _, att := range msg.Attachments {
			att.MessageID = msg.ID
			att.TopicID = msg.TopicID
		}
		messages = append(messages, msg)
	})
	return messages
}

// phpBBTopicID — ID темы, извлечённый из URL.
func phpBBTopicID(pageURL string) string {
	match := phpBBTopicIDRe.FindStringSubmatch(pageURL)
	if match == nil {
		return ""
	}
	return match[1]
}
