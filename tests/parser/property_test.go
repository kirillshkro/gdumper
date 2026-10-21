package parser

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/kirillshkro/gdumper/internal/parser"
)

// TestParsePageProperty проверяет инварианты парсинга на случайных
// входных данных: если парсинг успешен, структура результата корректна.
func TestParsePageProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		html := rapid.String().Draw(t, "html")
		pageURL := "http://x"
		page, err := parser.ParsePage([]byte(html), pageURL)

		if err != nil {
			// Ошибка допустима для нераспознанной разметки; главное —
			// отсутствие паники и корректное сообщение об ошибке.
			return
		}

		// Успешный парсинг гарантирует непустой результат.
		if page == nil {
			t.Fatalf("page должен быть непустым при успешном парсинге")
		}
		if page.Topic == nil {
			t.Fatalf("topic должен быть непустым")
		}
		if len(page.Messages) < 1 {
			t.Fatalf("должно быть не менее одного сообщения")
		}

		for _, msg := range page.Messages {
			if msg.ID == "" {
				t.Fatalf("id сообщения не может быть пустым: %v", msg)
			}
			if msg.Level < 0 {
				t.Fatalf("уровень не может быть отрицательным: %d", msg.Level)
			}
			if msg.PostNumber < 0 {
				t.Fatalf("post_number не может быть отрицательным: %d", msg.PostNumber)
			}
		}
	})
}

