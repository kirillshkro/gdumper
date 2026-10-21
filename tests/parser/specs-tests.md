# Спецификация тестов для пакета `internal/parser`

## 1. Уровни тестирования

| Уровень | Покрытие | Инструменты |
|---|---|---|
| Unit | парсеры | `testing`, `testify` |
| Golden | сравнение с эталоном | `goldie/v2` |
| Property-based | генерация HTML | `rapid` |
| Fuzz | парсер HTML | `go test -fuzz` + `rapid.MakeFuzz` |
| Bench | парсинг 1000 постов | `testing.B` |

## 2. Тест-кейсы

### TC-04. Парсинг

| ID | Сценарий | Ожидание |
|---|---|---|
| 04.1 | Простой пост | author, date, body извлечены |
| 04.2 | Пост с цитированием | поле `parent_id` заполнено |
| 04.3 | Древовидные ответы (2 уровня) | `level` = 0,1,2 |
| 04.4 | BB-code теги `[quote]`, `[url]` | конвертация в HTML/Markdown |
| 04.5 | Юникод/эмодзи | сохранены без искажений |
| 04.6 | Пустой пост (удалён) | `deleted: true`, не падает |
| 04.7 | XSS-инъекция в теле | экранирование в JSON |
| 04.8 | Изменённая разметка (v2 движка) | parser.Match() = false → ошибка |
| 04.9 | Пост с вложением (XenForo) | имя, URL (абсл.), MIME, размер, MessageID извлечены |
| 04.10 | Пост с вложением (phpBB) | имя, URL (абсл.), MIME, размер, MessageID извлечены |
| 04.11 | Вложение без alt/text | имя из базового имени URL, mime по расширению |
| 04.12 | Пост без вложений | Attachments пустое |

## 3. Golden-тесты

Для каждого движка — фикстуры:

```
testdata/
├── xenforo/page_001.html
├── xenforo/expected.json
├── phpbb/page_001.html
├── phpbb/expected.json
└── ...
```

Запуск: `go test ./... -update` для обновления эталонов через `goldie`.

## 4. Property-based тесты

```go
func TestParsePageProperty(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        html := rapid.String().Draw(t, "html")
        page, err := parser.ParsePage([]byte(html), "http://x")
        if err == nil {
            assert.NotNil(t, page)
        }
    })
}
```

## 5. Fuzz-тесты

```go
func FuzzParsePage(f *testing.F) {
    f.Add([]byte("<html>...</html>"))
    f.Fuzz(rapid.MakeFuzz(func(t *rapid.T) {
        data := rapid.SliceOf(rapid.Byte()).Draw(t, "data")
        _ = parser.ParsePage(data, "http://x")
    }))
}
```

**Цель:** ≥ 30 секунд без паник и OOM.

## 6. Бенчмарки

| Бенч | Метрика | Цель |
|---|---|---|
| `BenchmarkParsePage` | ns/op | < 50ms на страницу |