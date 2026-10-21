package parser

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
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
