package parser

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/kirillshkro/gdumper/internal/model"
)

// Маркеры, по которым распознаётся разметка SMF (Simple Machines Forum).
var smfMarkers = [][]byte{
	[]byte(`class="posttext"`),
}

// smfPageSize — постов на странице SMF по умолчанию.
const smfPageSize = 10

// smfTidRe — ID темы: query-параметр `tid=NNN` либо путь `/threads/NNN/`.
var smfTidRe = regexp.MustCompile(`tid=(\d+)|/threads/(\d+)/`)

// SMFParser — парсер тем движка SMF.
type SMFParser struct{}

// NewSMF возвращает парсер движка SMF.
func NewSMF() *SMFParser { return &SMFParser{} }

// Match возвращает true, если разметка страницы похожа на SMF.
func (p *SMFParser) Match(html []byte, _ string) bool {
	for _, marker := range smfMarkers {
		if bytes.Contains(html, marker) {
			return true
		}
	}
	return false
}

// ParsePage извлекает данные темы со страницы SMF.
func (p *SMFParser) ParsePage(html []byte, pageURL string) (*Page, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parser: не удалось разобрать HTML: %w", err)
	}

	topic := &model.Topic{URL: pageURL}
	topic.Title = strings.TrimSpace(doc.Find("div.topic").First().Text())
	if topic.Title == "" {
		topic.Title = strings.TrimSpace(doc.Find("title").First().Text())
	}
	topic.ID = smfTopicID(pageURL)
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

// LastPage возвращает номер последней страницы темы SMF.
func (p *SMFParser) LastPage(html []byte) (int, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return 0, fmt.Errorf("parser: не удалось разобрать HTML: %w", err)
	}
	return p.lastPageFromDoc(doc), nil
}

// PageURL возвращает URL n-й страницы темы SMF; query-параметр `tid`
// сохраняется, `start` подставляется как смещение от начала темы.
func (p *SMFParser) PageURL(base string, n int) string {
	if n <= 1 {
		return base
	}
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := u.Query()
	query.Set("start", strconv.Itoa((n-1)*smfPageSize))
	return u.String()
}

// lastPageFromDoc — максимум номера страницы в навигации (без навигации — 1).
func (p *SMFParser) lastPageFromDoc(doc *goquery.Document) int {
	last := 1
	doc.Find("a.smf-page").Each(func(_ int, sel *goquery.Selection) {
		number, err := strconv.Atoi(strings.TrimSpace(sel.Text()))
		if err == nil && number > last {
			last = number
		}
	})
	return last
}

// messages извлекает список сообщений со страницы SMF.
func (p *SMFParser) messages(doc *goquery.Document, pageURL string) []*model.Message {
	var messages []*model.Message
	doc.Find("div.post").Each(func(_ int, sel *goquery.Selection) {
		id, _ := sel.Attr("data-content")
		topicID, _ := sel.Attr("data-thread")
		parentID, _ := sel.Attr("data-parent")
		datetime, _ := sel.Find("span.smf-date").First().Attr("datetime")
		body := sel.Find("div.posttext").First()
		link := sel.Find("a.post-link").First()
		href, _ := link.Attr("href")

		bodyHTML := BBCodeToHTML(innerHTML(body))
		msg := &model.Message{
			ID:         id,
			TopicID:    topicID,
			Author:     authorFromLink(sel.Find("a.post-author"), pageURL),
			Date:       datetime,
			BodyHTML:   bodyHTML,
			BodyText:   plainText(bodyHTML),
			Link:       absURL(href, pageURL),
			PostNumber: postNumberFromText(link.Text()),
			ParentID:   parentID,
			Deleted:    body.HasClass("deleted"),
		}
		msg.Attachments = parseAttachments(sel, "div.postattachments a.attachment", pageURL)
		for _, att := range msg.Attachments {
			att.MessageID = msg.ID
			att.TopicID = msg.TopicID
		}
		messages = append(messages, msg)
	})
	return messages
}

// smfTopicID — ID темы, извлечённый из URL; пустая строка, если не найден.
func smfTopicID(pageURL string) string {
	match := smfTidRe.FindStringSubmatch(pageURL)
	if match == nil {
		return ""
	}
	if id := match[1]; id != "" {
		return id
	}
	return match[2]
}
