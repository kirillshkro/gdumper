package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/model"
)

var (
	// ErrDirectoryExists - ошибка существования каталога
	ErrDirectoryExists = errors.New("output directory already exists and is not empty")

	// ErrPermissionDenied - ошибка отсутствия прав на запись
	ErrPermissionDenied = errors.New("permission denied for output directory")

	// ErrInvalidFormat - ошибка невалидного формата
	ErrInvalidFormat = errors.New("invalid output format")
)

// Format представляет формат вывода
type Format string

const (
	// FormatJSON формат JSON
	FormatJSON Format = "json"

	// FormatJSONL формат JSONL
	FormatJSONL Format = "jsonl"

	// FormatMarkdown формат Markdown
	FormatMarkdown Format = "markdown"

	// FormatHTML формат HTML
	FormatHTML Format = "html"
)

// Storage представляет хранилище данных
type Storage struct {
	ctx     context.Context
	options config.Options
	format  Format
	buffer  *bytes.Buffer
	topics  []*model.Topic
	closed  bool
	logger  *slog.Logger
}

// NewStorage создает новое хранилище
func NewStorage(options config.Options) *Storage {
	return &Storage{
		ctx:     context.Background(),
		options: options,
		format:  Format(options.Format),
		buffer:  &bytes.Buffer{},
		closed:  false,
		logger:  slog.Default(),
	}
}

// SetFormatJSON устанавливает формат JSON
func (s *Storage) SetFormatJSON() {
	s.format = FormatJSON
}

// SetFormatJSONL устанавливает формат JSONL
func (s *Storage) SetFormatJSONL() {
	s.format = FormatJSONL
}

// SetFormatMarkdown устанавливает формат Markdown
func (s *Storage) SetFormatMarkdown() {
	s.format = FormatMarkdown
}

// SetFormatHTML устанавливает формат HTML
func (s *Storage) SetFormatHTML() {
	s.format = FormatHTML
}

// Save сохраняет тему, сообщения и вложения
func (s *Storage) Save(topic *model.Topic, messages []*model.Message, attachments []*model.Attachment) error {
	if s.closed {
		return errors.New("storage is closed")
	}

	messages = mergeAttachments(messages, attachments)

	switch s.format {
	case FormatJSON:
		return s.saveJSON(topic, messages)
	case FormatJSONL:
		return s.saveJSONL(messages)
	case FormatMarkdown:
		return s.saveMarkdown(topic, messages)
	case FormatHTML:
		return s.saveHTML(topic, messages)
	default:
		return ErrInvalidFormat
	}
}

// mergeAttachments объединяет вложения с родительскими сообщениями по ID поста
func mergeAttachments(messages []*model.Message, attachments []*model.Attachment) []*model.Message {
	if len(attachments) == 0 {
		return messages
	}

	byPost := make(map[string][]*model.Attachment, len(attachments))
	for _, att := range attachments {
		byPost[att.MessageID] = append(byPost[att.MessageID], att)
	}

	result := make([]*model.Message, len(messages))
	for i, msg := range messages {
		result[i] = withAttachments(msg, byPost[msg.ID])
	}

	return result
}

// withAttachments возвращает копию сообщения с объединёнными вложениями
func withAttachments(msg *model.Message, extra []*model.Attachment) *model.Message {
	if len(extra) == 0 {
		return msg
	}

	copied := *msg
	merged := make([]*model.Attachment, 0, len(msg.Attachments)+len(extra))
	merged = append(merged, msg.Attachments...)
	merged = append(merged, extra...)
	copied.Attachments = merged

	return &copied
}

// saveJSON сохраняет данные в формате JSON: массив тем, каждая — со своими сообщениями
func (s *Storage) saveJSON(topic *model.Topic, messages []*model.Message) error {
	entry := *topic
	entry.Messages = messages

	s.topics = append(s.topics, &entry)

	data, err := json.MarshalIndent(s.topics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal topics: %w", err)
	}

	s.buffer.Reset()
	if _, err := s.buffer.Write(data); err != nil {
		return fmt.Errorf("failed to write JSON data: %w", err)
	}

	return nil
}

// saveJSONL сохраняет сообщения в формате JSONL: одна строка — один пост
func (s *Storage) saveJSONL(messages []*model.Message) error {
	data, err := MarshalJSONL(messages)
	if err != nil {
		return err
	}

	if _, err := s.buffer.Write(data); err != nil {
		return fmt.Errorf("failed to write JSONL data: %w", err)
	}

	return nil
}

// saveMarkdown сохраняет данные в формате Markdown
func (s *Storage) saveMarkdown(topic *model.Topic, messages []*model.Message) error {
	fmt.Fprintf(s.buffer, "# %s\n\n", topic.Title)
	fmt.Fprintf(s.buffer, "**Автор:** %s\n\n", topic.Author.Name)
	fmt.Fprintf(s.buffer, "**Создано:** %s\n\n", topic.Created)
	if topic.URL != "" {
		fmt.Fprintf(s.buffer, "[**Ссылка**](%s)\n\n", topic.URL)
	}

	s.buffer.WriteString("---\n\n")

	for _, msg := range messages {
		fmt.Fprintf(s.buffer, "## %s\n\n", msg.Author.Name)
		fmt.Fprintf(s.buffer, "> %s\n\n", msg.BodyText)
		if msg.Link != "" {
			fmt.Fprintf(s.buffer, "[**Ссылка**](%s)\n\n", msg.Link)
		}

		for _, att := range msg.Attachments {
			if att.URL != "" {
				fmt.Fprintf(s.buffer, "- [%s](%s)\n", att.Name, att.URL)
			}
		}
		s.buffer.WriteString("\n")
	}

	return nil
}

// saveHTML сохраняет данные в формате HTML
func (s *Storage) saveHTML(topic *model.Topic, messages []*model.Message) error {
	s.buffer.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"UTF-8\">\n")
	fmt.Fprintf(s.buffer, "<title>%s</title>\n", topic.Title)
	s.buffer.WriteString("</head>\n<body>\n")

	fmt.Fprintf(s.buffer, "<h1>%s</h1>\n", topic.Title)
	fmt.Fprintf(s.buffer, "<p><strong>Автор:</strong> %s</p>\n", topic.Author.Name)
	fmt.Fprintf(s.buffer, "<p><strong>Создано:</strong> %s</p>\n", topic.Created)
	if topic.URL != "" {
		fmt.Fprintf(s.buffer, "<p><a href=\"%s\">Ссылка</a></p>\n", topic.URL)
	}

	s.buffer.WriteString("<hr>\n")

	for _, msg := range messages {
		fmt.Fprintf(s.buffer, "<h2>%s</h2>\n", msg.Author.Name)
		fmt.Fprintf(s.buffer, "<blockquote>%s</blockquote>\n", msg.BodyHTML)
		if msg.Link != "" {
			fmt.Fprintf(s.buffer, "<p><a href=\"%s\">Ссылка</a></p>\n", msg.Link)
		}

		for _, att := range msg.Attachments {
			if att.URL != "" {
				fmt.Fprintf(s.buffer, "<p><a href=\"%s\">%s</a></p>\n", att.URL, att.Name)
			}
		}
	}

	s.buffer.WriteString("</body>\n</html>\n")

	return nil
}

// Content возвращает содержимое буфера
func (s *Storage) Content() ([]byte, error) {
	if s.closed {
		return nil, errors.New("storage is closed")
	}

	return s.buffer.Bytes(), nil
}

// WriteToFile записывает содержимое в файл
func (s *Storage) WriteToFile(path string) error {
	if s.closed {
		return errors.New("storage is closed")
	}

	content, err := s.Content()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := s.checkDirectory(dir); err != nil {
		return err
	}

	if err := s.checkWritePermission(dir); err != nil {
		return err
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	s.logger.Info("file written", "path", path)

	return nil
}

// checkDirectory проверяет существование каталога
func (s *Storage) checkDirectory(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return errors.New("path is not a directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	if len(entries) > 0 && !s.options.Force {
		return ErrDirectoryExists
	}

	return nil
}

// checkWritePermission проверяет права на запись
func (s *Storage) checkWritePermission(dir string) error {
	testFile := filepath.Join(dir, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return fmt.Errorf("failed to create test file: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// Close закрывает хранилище
func (s *Storage) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true
	s.buffer = nil

	return nil
}

// MarshalJSONL сериализует сообщения в формат JSONL
func MarshalJSONL(messages []*model.Message) ([]byte, error) {
	var buf bytes.Buffer

	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal message: %w", err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

// SanitizeFilename удаляет недопустимые символы из имени файла
func SanitizeFilename(filename string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		`"`, "-",
		"<", "-",
		">", "-",
		"|", "-",
	)

	sanitized := replacer.Replace(filename)
	sanitized = strings.TrimLeft(sanitized, " .")

	if len(sanitized) > 255 {
		sanitized = sanitized[:255]
	}

	return sanitized
}
