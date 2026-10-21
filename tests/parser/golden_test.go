package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kirillshkro/gdumper/internal/parser"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

// goldenFixturePath возвращает путь к фикстуру testdata/{engine}/page_001.html.
func goldenFixturePath(t *testing.T, engine string) string {
	t.Helper()
	return filepath.Join("testdata", engine, "page_001.html")
}

// goldenTestURL возвращает URL для фикстура движка.
func goldenTestURL(engine string) string {
	if engine == "xenforo" {
		return "https://forum.example.com/threads/12345/"
	}
	return "https://forum.example.com/thread-67890.php"
}

// TestParserGoldenTests сравнивает результат парсинга фикстура с эталоном
// через goldie/v2. Эталонные файлы обновляются командой -update.
func TestParserGoldenTests(t *testing.T) {
	t.Run("XenForo", func(t *testing.T) {
		page := goldenPage(t, "xenforo")
		goldenAssert(t, "xenforo/expected", page)
	})
	t.Run("PHPBB", func(t *testing.T) {
		page := goldenPage(t, "phpbb")
		goldenAssert(t, "phpbb/expected", page)
	})
}

// goldenPage парсит фикстуру движка и возвращает страницу.
func goldenPage(t *testing.T, engine string) *parser.Page {
	t.Helper()

	path := goldenFixturePath(t, engine)
	html, err := os.ReadFile(path)
	require.NoError(t, err, "не удалось прочитать фикстуру %s", path)

	page, err := parser.ParsePage(html, goldenTestURL(engine))
	require.NoError(t, err, "не удалось распарсить фикстуру %s", engine)

	return page
}

// goldenAssert сравнивает JSON-сериализацию страницы с эталоном через goldie.
func goldenAssert(t *testing.T, name string, page *parser.Page) {
	data, err := json.Marshal(page)
	require.NoError(t, err)

	g := goldie.New(t, goldie.WithNameSuffix(".json"))
	g.Assert(t, name, data)
}
