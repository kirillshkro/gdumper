package storage

import (
	"fmt"
	"testing"

	"github.com/kirillshkro/gdumper/internal/config"
	"github.com/kirillshkro/gdumper/internal/model"
	"github.com/kirillshkro/gdumper/internal/storage"
)

// makePosts создаёт набор из n тестовых постов
func makePosts(n int) []*model.Message {
	posts := make([]*model.Message, 0, n)
	for i := 0; i < n; i++ {
		posts = append(posts, &model.Message{
			ID:         fmt.Sprintf("msg-%d", i),
			TopicID:    "topic-1",
			Author:     fmt.Sprintf("Автор %d", i),
			Date:       "2026-01-01T00:00:00Z",
			Content:    fmt.Sprintf("Текст сообщения %d", i),
			PostNumber: i + 1,
			Level:      i % 3,
		})
	}

	return posts
}

// BenchmarkMarshalJSONL измеряет сериализацию одного поста в JSONL (цель: allocs/op < 10)
func BenchmarkMarshalJSONL(b *testing.B) {
	posts := makePosts(1000)

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		p := i % len(posts)
		_, _ = storage.MarshalJSONL(posts[p : p+1])
		i++
	}
}

// BenchmarkStorageSave измеряет сохранение 1000 постов в хранилище (формат JSONL)
func BenchmarkStorageSave(b *testing.B) {
	posts := makePosts(1000)
	topic := &model.Topic{ID: "topic-1", Title: "Тема бенчмарка"}
	outDir := b.TempDir()

	b.ReportAllocs()
	for b.Loop() {
		st := storage.NewStorage(config.Options{
			OutputDir: outDir,
			Format:    string(storage.FormatJSONL),
		})
		if err := st.Save(topic, posts, nil); err != nil {
			b.Fatalf("не удалось сохранить посты: %v", err)
		}
	}
}
