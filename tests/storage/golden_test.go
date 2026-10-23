package storage

import (
	"encoding/json"
	"testing"

	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/model"
	"github.com/kirillshkro/gdumper/internal/storage"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

// goldenFixture возвращает детерминированные данные для golden-тестов
func goldenFixture() (*model.Topic, []*model.Message) {
	topic := &model.Topic{
		ID:         "topic-1",
		Title:      "Тестовая тема",
		URL:        "https://example.com/forum/topic-1",
		Author:     model.Author{Name: "Автор темы"},
		Created:    "2026-01-01T00:00:00Z",
		PageSize:   20,
		TotalPages: 1,
	}

	messages := []*model.Message{
		{
			ID:         "msg-1",
			TopicID:    "topic-1",
			Author:     model.Author{Name: "Пользователь 1"},
			Date:       "2026-01-01T01:00:00Z",
			BodyText:   "Первое сообщение",
			Link:       "https://example.com/forum/topic-1#msg-1",
			PostNumber: 1,
			Level:      0,
		},
		{
			ID:         "msg-2",
			TopicID:    "topic-1",
			Author:     model.Author{Name: "Пользователь 2"},
			Date:       "2026-01-01T02:00:00Z",
			BodyText:   "Второе сообщение",
			PostNumber: 2,
			ParentID:   "msg-1",
			Level:      1,
		},
	}

	return topic, messages
}

// goldenStorage создаёт хранилище в заданном формате для golden-тестов
func goldenStorage(t *testing.T, format storage.Format) *storage.Storage {
	t.Helper()

	return storage.NewStorage(config.Options{
		OutputDir: t.TempDir(),
		Format:    string(format),
	})
}

// TestStorageGoldenTests сравнивает реальный вывод хранилища с эталонными файлами (goldie/v2)
func TestStorageGoldenTests(t *testing.T) {
	topic, messages := goldenFixture()

	t.Run("JSON Format", func(t *testing.T) {
		st := goldenStorage(t, storage.FormatJSON)
		require.NoError(t, st.Save(topic, messages, nil))

		data, err := st.Content()
		require.NoError(t, err)

		g := goldie.New(t, goldie.WithNameSuffix(".json"))
		g.Assert(t, "json_output", data)
	})

	t.Run("JSONL Format", func(t *testing.T) {
		st := goldenStorage(t, storage.FormatJSONL)
		require.NoError(t, st.Save(topic, messages, nil))

		data, err := st.Content()
		require.NoError(t, err)

		g := goldie.New(t, goldie.WithNameSuffix(".jsonl"))
		g.Assert(t, "jsonl_output", data)
	})

	t.Run("Markdown Format", func(t *testing.T) {
		st := goldenStorage(t, storage.FormatMarkdown)
		require.NoError(t, st.Save(topic, messages, nil))

		data, err := st.Content()
		require.NoError(t, err)

		g := goldie.New(t, goldie.WithNameSuffix(".md"))
		g.Assert(t, "markdown_output", data)
	})
}

// TestStorageGoldenJSON проверяет корректность JSON-выхода: схема массива тем
func (s *StorageSuite) TestStorageGoldenJSON() {
	s.stor.SetFormatJSON()

	topic, messages := goldenFixture()
	err := s.stor.Save(topic, messages, nil)
	s.NoError(err)

	content, err := s.stor.Content()
	s.NoError(err)

	var topics []model.Topic
	err = json.Unmarshal(content, &topics)
	s.NoError(err)

	s.Require().Len(topics, 1)
	s.Equal(topic.ID, topics[0].ID)
	s.Equal(topic.Title, topics[0].Title)
	s.Len(topics[0].Messages, len(messages))
}

// TestStorageGoldenMarkdown проверяет корректность Markdown-выхода: заголовки, цитаты, ссылки
func (s *StorageSuite) TestStorageGoldenMarkdown() {
	s.stor.SetFormatMarkdown()

	topic, messages := goldenFixture()
	err := s.stor.Save(topic, messages, nil)
	s.NoError(err)

	content, err := s.stor.Content()
	s.NoError(err)
	text := string(content)

	s.Contains(text, "# "+topic.Title)
	s.Contains(text, "[**Ссылка**]("+topic.URL+")")
	for _, msg := range messages {
		s.Contains(text, "> "+msg.BodyText)
	}
}
