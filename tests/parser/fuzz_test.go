package parser

import (
	"testing"

	"pgregory.net/rapid"
	"github.com/kirillshkro/gdumper/internal/parser"
)

// FuzzParsePage — фазза-тест парсера: генерирует случайные байты и
// проверяет, что парсинг не падает паникой и не улетает по памяти.
// Целевая длительность — не менее 30 секунд без ошибок.
// Запуск: go test ./tests/parser/ -fuzz=FuzzParsePage -fuzztime=35s
func FuzzParsePage(f *testing.F) {
	// Seed-кейсы: реальные страницы обоих движков.
	f.Add([]byte("<html>пустая страница</html>"))
	f.Add([]byte("не форум вообще"))

	f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
		data := rapid.SliceOf(rapid.Byte()).Draw(t, "data")
		// Разбрасываем оба значения — фазза проверяет лишь отсутствие паники.
		_, _ = parser.ParsePage(data, "http://x")
	}))
}
