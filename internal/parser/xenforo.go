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

// Маркеры, по которым распознаётся разметка XenForo.
var xenForoMarkers = [][]byte{
	[]byte(`data-app="XenForo"`),
	[]byte(`class="message-body"`),
}

// xenForoTopicIDRe — ID темы в URL вида /threads/12345/.
var xenForoTopicIDRe = regexp.MustCompile(`/threads/(\d+)/`)

// XenForoParser — парсер тем движка XenForo.
type XenForoParser struct{}

// NewXenForo возвращает парсер движка XenForo.
func NewXenForo() *XenForoParser { return &XenForoParser{} }

// Match возвращает true, если разметка страницы похожа на XenForo.
func (p *XenForoParser) Match(html []byte, _ string) bool {
	for _, marker := range xenForoMarkers {
		if bytes.Contains(html, marker) {
			return true
		}
	}
	return false
}

// ParsePage извлекает данные темы со страницы XenForo.
func (p *XenForoParser) ParsePage(html []byte, pageURL string) (*Page, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parser: не удалось разобрать HTML: %w", err)
	}

	topic := &model.Topic{URL: pageURL}
	topic.Title = strings.TrimSpace(doc.Find("h1.p-title-title").First().Text())
	if topic.Title == "" {
		topic.Title = strings.TrimSpace(doc.Find("title").First().Text())
	}
	topic.ID = xenForoTopicID(pageURL)
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
func (p *XenForoParser) LastPage(html []byte) (int, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return 0, fmt.Errorf("parser: не удалось разобрать HTML: %w", err)
	}
	return p.lastPageFromDoc(doc), nil
}

// PageURL возвращает URL n-й страницы темы XenForo.
func (p *XenForoParser) PageURL(base string, n int) string {
	if n <= 1 {
		return base
	}
	return strings.TrimRight(base, "/") + fmt.Sprintf("/page-%d", n)
}

// lastPageFromDoc — максимум номера страницы в навигации (без навигации — 1).
func (p *XenForoParser) lastPageFromDoc(doc *goquery.Document) int {
	last := 1
	doc.Find("a.p-nav-link").Each(func(_ int, sel *goquery.Selection) {
		number, err := strconv.Atoi(strings.TrimSpace(sel.Text()))
		if err == nil && number > last {
			last = number
		}
	})
	return last
}

// messages извлекает список сообщений со страницы.
func (p *XenForoParser) messages(doc *goquery.Document, pageURL string) []*model.Message {
	var messages []*model.Message
	doc.Find("article.message").Each(func(_ int, sel *goquery.Selection) {
		id, _ := sel.Attr("data-content")
		topicID, _ := sel.Attr("data-thread")
		parentID, _ := sel.Attr("data-parent")
		datetime, _ := sel.Find("time.date").First().Attr("datetime")
		body := sel.Find("div.message-body").First()
		link := sel.Find("a.message-link").First()
		href, _ := link.Attr("href")

		msg := &model.Message{
			ID:         id,
			TopicID:    topicID,
			Author:     strings.TrimSpace(sel.Find("h3.message-userName a").First().Text()),
			Date:       datetime,
			Content:    BBCodeToHTML(innerHTML(body)),
			Link:       absURL(href, pageURL),
			PostNumber: postNumberFromText(link.Text()),
			ParentID:   parentID,
			Deleted:    body.HasClass("deleted"),
		}
		msg.Attachments = parseAttachments(sel, "div.message-attachments a.attachment", pageURL)
		for _, att := range msg.Attachments {
			att.MessageID = msg.ID
			att.TopicID = msg.TopicID
		}
		messages = append(messages, msg)
	})
	return messages
}

// xenForoTopicID — ID темы, извлечённый из URL.
func xenForoTopicID(pageURL string) string {
	match := xenForoTopicIDRe.FindStringSubmatch(pageURL)
	if match == nil {
		return ""
	}
	return match[1]
}
