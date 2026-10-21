// Package parser реализует извлечение структурированных данных
// (тема, сообщения, уровни вложенности) из HTML-страниц форумов.
//
// Поддерживаемые движки: XenForo (NewXenForo) и phpBB (NewPHPBB).
// Пакетный API (ParsePage, Match, LastPage) выбирает первый
// распознавший страницу парсер из списка Parsers.
package parser

import (
	"errors"

	"github.com/kirillshkro/gdumper/internal/model"
)

// Ошибки пакета.
var (
	// ErrNoParser — ни один известный парсер не распознал разметку страницы.
	ErrNoParser = errors.New("parser: ни один парсер не распознал разметку страницы")
	// ErrEmptyPage — разметка распознана, но сообщений на странице нет.
	ErrEmptyPage = errors.New("parser: на странице не найдено ни одного сообщения")
)

// Page — данные, извлечённые из одной страницы темы.
type Page struct {
	// Topic — метаданные темы.
	Topic *model.Topic `json:"topic"`
	// Messages — сообщения, найденные на странице.
	Messages []*model.Message `json:"messages"`
}

// Parser — контракт парсера движка форума.
type Parser interface {
	// Match распознаёт разметку страницы.
	Match(html []byte, url string) bool
	// ParsePage извлекает данные со страницы.
	ParsePage(html []byte, url string) (*Page, error)
	// LastPage возвращает номер последней страницы темы.
	LastPage(html []byte) (int, error)
	// PageURL строит URL n-й страницы по URL первой.
	PageURL(base string, n int) string
}

// Parsers — поддерживаемые парсеры; первый распознанный побеждает.
var Parsers = []Parser{
	NewXenForo(),
	NewPHPBB(),
}

// Match возвращает true, если страницу распознаёт хотя бы один известный парсер.
func Match(html []byte, url string) bool {
	for _, p := range Parsers {
		if p.Match(html, url) {
			return true
		}
	}
	return false
}

// ParsePage передаёт страницу первому распознавшему её парсеру.
func ParsePage(html []byte, url string) (*Page, error) {
	for _, p := range Parsers {
		if p.Match(html, url) {
			return p.ParsePage(html, url)
		}
	}
	return nil, ErrNoParser
}

// LastPage возвращает номер последней страницы распознанной темы.
func LastPage(html []byte, url string) (int, error) {
	for _, p := range Parsers {
		if p.Match(html, url) {
			return p.LastPage(html)
		}
	}
	return 0, ErrNoParser
}
