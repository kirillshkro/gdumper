package parser

import (
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	cyberhtml "github.com/cybergodev/html"
	"golang.org/x/net/html"

	"github.com/kirillshkro/gdumper/internal/model"
)

// postNumberRe — номер поста в тексте ссылки в виде "#12".
var postNumberRe = regexp.MustCompile(`#?(\d+)`)

// innerHTML возвращает HTML-разметку содержимого элементов выборки.
func innerHTML(sel *goquery.Selection) string {
	var sb strings.Builder
	sel.Contents().Each(func(_ int, node *goquery.Selection) {
		_ = html.Render(&sb, node.Nodes[0])
	})
	return strings.TrimSpace(sb.String())
}

// absURL приводит относительную ссылку ref к абсолютной относительно base.
func absURL(ref, base string) string {
	baseU, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refU, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return baseU.ResolveReference(refU).String()
}

// postNumberFromText извлекает номер поста из текста ссылки вида "#12".
func postNumberFromText(text string) int {
	match := postNumberRe.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return 0
	}
	number, _ := strconv.Atoi(match[1])
	return number
}

// assignLevels вычисляет уровень вложенности сообщений по цепочкам ParentID:
// сообщение без родителя — уровень 0, ответ — уровень родителя плюс один.
func assignLevels(messages []*model.Message) {
	parents := make(map[string]string, len(messages))
	byID := make(map[string]*model.Message, len(messages))
	for _, msg := range messages {
		parents[msg.ID] = msg.ParentID
		byID[msg.ID] = msg
	}

	done := make(map[string]int, len(messages))
	var level func(id string, visiting map[string]bool) int
	level = func(id string, visiting map[string]bool) int {
		if saved, ok := done[id]; ok {
			return saved
		}
		if visiting[id] { // защита от циклов в data-parent
			return 0
		}
		parent := parents[id]
		visiting[id] = true
		depth := 0
		if parent != "" {
			if _, known := byID[parent]; known {
				depth = level(parent, visiting) + 1
			}
		}
		visiting[id] = false
		done[id] = depth
		return depth
	}

	for id := range parents {
		byID[id].Level = level(id, map[string]bool{})
	}
}

// parseAttachments извлекает вложения из элемента сообщения.
//
// containerSelector — CSS-селектор ссылок-вложений внутри поста
// (например, "div.message-attachments a.attachment").
// Возвращает nil, если вложений не найдено.
func parseAttachments(sel *goquery.Selection, containerSelector, url string) []*model.Attachment {
	var atts []*model.Attachment
	sel.Find(containerSelector).Each(func(_ int, a *goquery.Selection) {
		href, ok := a.Attr("href")
		if !ok {
			return
		}
		name, _ := a.Attr("data-file-name")
		if name == "" {
			name = a.Text()
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = path.Base(strings.TrimSpace(href))
		}
		if name == "" {
			return
		}
		mime, _ := a.Attr("data-mime")
		if mime == "" {
			mime = guessMIME(name)
		}
		att := &model.Attachment{
			Name: name,
			URL:      absURL(href, url),
			MIME:     strings.TrimSpace(mime),
		}
		if size, ok := a.Attr("data-size"); ok && size != "" {
			if v, err := strconv.ParseInt(size, 10, 64); err == nil && v > 0 {
				att.Size = v
			}
		}
		atts = append(atts, att)
	})
	return atts
}

// mimeByExt — тип MIME для распространённых расширений файлов.
var mimeByExt = map[string]string{
	"pdf":  "application/pdf",
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"svg":  "image/svg+xml",
	"webp": "image/webp",
	"mp3":  "audio/mpeg",
	"mp4":  "video/mp4",
	"txt":  "text/plain",
	"zip":  "application/zip",
	"doc":  "application/msword",
}

// guessMIME определяет тип MIME по расширению имени файла;
// при неизвестном расширении возвращает пустую строку.
func guessMIME(name string) string {
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 {
		return ""
	}
	ext := strings.ToLower(name[dot+1:])
	return mimeByExt[ext]
}

// authorFromLink возвращает имя автора и абсолютный URL профиля
// из элемента ссылки; без ссылки возвращает пустой Author.
func authorFromLink(sel *goquery.Selection, baseURL string) model.Author {
	first := sel.First()
	if first.Length() == 0 {
		return model.Author{}
	}
	href, ok := first.Attr("href")
	profileURL := ""
	if ok && href != "" {
		profileURL = absURL(href, baseURL)
	}
	return model.Author{Name: strings.TrimSpace(first.Text()), ProfileURL: profileURL}
}

// plainText извлекает plain-текст из HTML-строки через cybergodev/html.
// Пустой ввод или ошибка парсинга дают пустой результат без паники.
func plainText(htmlStr string) string {
	if strings.TrimSpace(htmlStr) == "" {
		return ""
	}
	out, err := cyberhtml.ExtractText([]byte(htmlStr))
	if err != nil {
		return ""
	}
	return out
}
