package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/model"
	"github.com/kirillshkro/gdumper/internal/storage"
	"github.com/stretchr/testify/suite"
)

// StorageSuite представляет тестовый набор для функциональности storage
type StorageSuite struct {
	suite.Suite
	tempDir string
	stor    *storage.Storage
}

// SetupTest готовит тестовые данные перед каждым тестом
func (s *StorageSuite) SetupTest() {
	var err error
	s.tempDir, err = os.MkdirTemp("", "storage_test")
	s.NoError(err)
	options := config.Options{
		OutputDir: s.tempDir,
	}
	s.stor = storage.NewStorage(options)
	s.NotNil(s.stor)
}

// TearDownTest очищает тестовые данные после каждого теста
func (s *StorageSuite) TearDownTest() {
	if s.tempDir != "" {
		s.stor.Close()
		os.RemoveAll(s.tempDir)
	}
}

// TestStorageSuite запускает все тесты для storage
func TestStorageSuite(t *testing.T) {
	suite.Run(t, new(StorageSuite))
}

// TestStorageJSONFormat проверяет сохранение в формате JSON
func (s *StorageSuite) TestStorageJSONFormat() {
	// TODO: реализовать тест для формата JSON
	// Сценарий: --format=json
	// Ожидание: валидный JSON, схема соответствует
	s.stor.SetFormatJSON()
	ftopic := &model.Topic{
		ID:    uuid.NewString(),
		Title: "test title",
	}
	err := s.stor.Save(ftopic, []*model.Message{}, []*model.Attachment{})
	s.NoError(err)
	content, err := s.stor.Content()
	s.NoError(err)
	var converted []model.Topic
	err = json.Unmarshal(content, &converted)
	s.NoError(err)
}

// TestStorageJSONLFormat проверяет сохранение в формате JSONL (TC 06.2)
func (s *StorageSuite) TestStorageJSONLFormat() {
	s.stor.SetFormatJSONL()

	topic := &model.Topic{ID: "t1", Title: "Тема"}
	messages := []*model.Message{
		{ID: "m1", BodyText: "Сообщение 1"},
		{ID: "m2", BodyText: "Сообщение 2"},
	}
	err := s.stor.Save(topic, messages, nil)
	s.NoError(err)

	content, err := s.stor.Content()
	s.NoError(err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	s.Len(lines, len(messages)) // одна строка = один пост

	for i, line := range lines {
		var msg model.Message
		s.NoError(json.Unmarshal([]byte(line), &msg))
		s.Equal(messages[i].ID, msg.ID)
	}
}

// TestStorageMarkdownFormat проверяет сохранение в формате Markdown (TC 06.3)
func (s *StorageSuite) TestStorageMarkdownFormat() {
	s.stor.SetFormatMarkdown()

	topic := &model.Topic{
		ID:      "t1",
		Title:   "Тестовая тема",
		Author:  model.Author{Name: "Автор"},
		Created: "2026-01-01T00:00:00Z",
		URL:     "https://example.com/topic-1",
	}
	messages := []*model.Message{
		{ID: "m1", Author: model.Author{Name: "Пользователь"}, BodyText: "Текст цитаты", Link: "https://example.com/topic-1#m1"},
	}
	err := s.stor.Save(topic, messages, nil)
	s.NoError(err)

	content, err := s.stor.Content()
	s.NoError(err)
	text := string(content)

	s.Contains(text, "# Тестовая тема")                              // заголовок
	s.Contains(text, "> Текст цитаты")                               // цитата
	s.Contains(text, "[**Ссылка**](https://example.com/topic-1#m1)") // ссылка
}

// TestStorageUTF8Filenames проверяет корректный UTF-8 в кириллических именах файлов (TC 06.4)
func (s *StorageSuite) TestStorageUTF8Filenames() {
	s.stor.SetFormatJSON()

	topic := &model.Topic{ID: "t1", Title: "Тема"}
	err := s.stor.Save(topic, nil, nil)
	s.NoError(err)

	name := "тест_архив_темы.json"
	path := filepath.Join(s.tempDir, name)
	err = s.stor.WriteToFile(path)
	s.NoError(err)

	info, err := os.Stat(path)
	s.NoError(err)
	s.False(info.IsDir())
	s.Equal(name, info.Name()) // корректный UTF-8
}

// TestStorageDirectoryExists проверяет поведение при непустом каталоге (TC 06.5)
func (s *StorageSuite) TestStorageDirectoryExists() {
	s.stor.SetFormatJSON()

	topic := &model.Topic{ID: "t1", Title: "Тема"}
	err := s.stor.Save(topic, nil, nil)
	s.NoError(err)

	err = os.WriteFile(filepath.Join(s.tempDir, "existing.txt"), []byte("data"), 0o644)
	s.NoError(err)

	// Без --force: ошибка
	err = s.stor.WriteToFile(filepath.Join(s.tempDir, "out.json"))
	s.ErrorIs(err, storage.ErrDirectoryExists)

	// С --force: запись успешна
	forced := storage.NewStorage(config.Options{OutputDir: s.tempDir, Force: true})
	forced.SetFormatJSON()
	s.NoError(forced.Save(topic, nil, nil))
	s.NoError(forced.WriteToFile(filepath.Join(s.tempDir, "out.json")))
}

// TestStoragePermissionDenied проверяет поведение при отсутствии прав на запись (TC 06.6)
func (s *StorageSuite) TestStoragePermissionDenied() {
	if os.Getuid() == 0 {
		s.Suite.T().Skip("тест не выполняется под root: ограничения прав игнорируются")
	}

	s.stor.SetFormatJSON()

	topic := &model.Topic{ID: "t1", Title: "Тема"}
	err := s.stor.Save(topic, nil, nil)
	s.NoError(err)

	err = os.Chmod(s.tempDir, 0o500)
	s.NoError(err)
	defer func() { _ = os.Chmod(s.tempDir, 0o755) }()

	err = s.stor.WriteToFile(filepath.Join(s.tempDir, "out.json"))
	s.ErrorIs(err, storage.ErrPermissionDenied)
}
