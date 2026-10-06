# Спецификация тестов для утилиты `forumdump`

**Версия:** 1.1
**Дата:** 2026-10-05
**Связанный документ:** `forumdump.md`

---

## 1. Уровни тестирования

| Уровень | Покрытие | Инструменты |
|---|---|---|
| Unit | парсеры, авторизация, пагинация, storage | `testing`, `testify` |
| Integration | fetcher + parser + storage на моках HTTP | `httptest`, `gock` |
| E2E | полный прогон на локальном mock-сервере | `httptest`, `httpexpect` |
| Golden | сравнение JSON/Markdown с эталоном | `goldie/v2` |
| Property-based | генерация входных данных | `rapid` |
| Fuzz | парсер HTML | `go test -fuzz` + `rapid.MakeFuzz` |
| Bench | парсинг 1000 постов | `testing.B` |
| CLI | флаги, exit-коды | `os/exec` |

---

## 2. Матрица тест-кейсов

### TC-01. CLI / аргументы

| ID | Сценарий | Ожидание |
|---|---|---|
| 01.1 | Запуск без `--url` | exit code 2, сообщение об ошибке |
| 01.2 | Невалидный URL | exit code 2 |
| 01.3 | `--format=unknown` | exit code 2 |
| 01.4 | `--pages=5-2` | exit code 2 |
| 01.5 | `--help` | exit 0, справка со всеми флагами |
| 01.6 | Флаг из ENV (`FORUMDUMP_URL`) | переопределяется CLI |

### TC-02. Авторизация

| ID | Сценарий | Ожидание |
|---|---|---|
| 02.1 | Cookies из Chrome (mock profile) | заголовок `Cookie` установлен |
| 02.2 | Cookies из Netscape-файла | корректный парсинг |
| 02.3 | Cookies raw-строкой | разбор `k=v; k2=v2` |
| 02.4 | Просроченные cookies (401) | понятная ошибка |
| 02.5 | Отсутствие браузера | fallback-ошибка, не паникует |
| 02.6 | Cookies не попадают в `--verbose` лог | grep по логу не находит значений |

### TC-03. Пагинация

| ID | Сценарий | Ожидание |
|---|---|---|
| 03.1 | Одностраничная тема | lastPage=1, один запрос |
| 03.2 | Многостраничная (10 стр.) | 10 запросов, порядок 1→10 |
| 03.3 | `--pages=2-4` | только стр. 2,3,4 |
| 03.4 | Битые ссылки пагинации | graceful degradation, warning |
| 03.5 | Последняя страница без next | корректное завершение |
| 03.6 | Редирект на канонический URL | следование редиректу |

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

### TC-05. Вложения

| ID | Сценарий | Ожидание |
|---|---|---|
| 05.1 | Один PDF | скачан, SHA-256 совпадает |
| 05.2 | Несколько вложений | все в `attachments/<post_id>/` |
| 05.3 | `--attachments=links` | только URL в JSON |
| 05.4 | 404 на вложение | warning, пост сохраняется |
| 05.5 | Большой файл (100MB) | стриминг, память < 50MB |
| 05.6 | Повторный запуск | файл не скачивается |
| 05.7 | Изображение-превью + оригинал | оба сохранены |

### TC-06. Хранилище

| ID | Сценарий | Ожидание |
|---|---|---|
| 06.1 | `--format=json` | валидный JSON, схема соответствует |
| 06.2 | `--format=jsonl` | одна строка = один пост |
| 06.3 | `--format=markdown` | заголовки, цитаты, ссылки |
| 06.4 | Кириллица в именах файлов | корректный UTF-8 |
| 06.5 | Каталог существует и непуст | ошибка или `--force` |
| 06.6 | Нет прав на запись | exit code 3 |

### TC-07. Сеть / устойчивость

| ID | Сценарий | Ожидание |
|---|---|---|
| 07.1 | 500, затем 200 | retry успешен |
| 07.2 | 429 с `Retry-After` | пауза согласно заголовку |
| 07.3 | Таймаут | retry, затем ошибка |
| 07.4 | Обрыв соединения на 5-й странице | частичный дамп |
| 07.5 | Ctrl+C (SIGINT) | graceful shutdown |
| 07.6 | `--delay=1s` | интервал ≥1s |

### TC-08. Конфигурация и расширяемость

| ID | Сценарий | Ожидание |
|---|---|---|
| 08.1 | YAML-конфиг с селекторами | применяется |
| 08.2 | Неизвестный движок | auto-detect или ошибка |
| 08.3 | Плагин-парсер (тестовый) | регистрируется через `Register()` |

---

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

---

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

---

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

---

## 6. Бенчмарки

| Бенч | Метрика | Цель |
|---|---|---|
| `BenchmarkParsePage` | ns/op | < 5ms на страницу |
| `BenchmarkMarshalJSONL` | allocs/op | < 10 |
| `BenchmarkDownloadAttachment` | MB/s | ≥ 20 |

---

## 7. Критерии приёмки

- Покрытие unit-тестами ≥ 80% (`go test -cover`).
- Все golden-тесты зелёные.
- Fuzz 30s без падений.
- E2E на mock-сервере (3 страницы, 20 постов, 5 вложений) — успех.
- `golangci-lint` без ошибок.
- Бинарник < 20MB, стартует < 100ms.

---

## 8. Тестовая среда

- Go 1.21+, `testify`, `goldie/v2`, `rapid`, `httptest`, `go-cmp`, `golangci-lint`.
- CI: GitHub Actions matrix (linux/macos/windows, Go 1.21/1.22).
- Мок-сервер: `httptest.NewServer` с роутингом по страницам и вложениям.

---

## Приложение А: Сводная таблица тестовых зависимостей

| Компонент | Библиотека | Версия (рекоменд.) |
|---|---|---|
| Assertions | `stretchr/testify` | v1.9+ |
| Golden files | `sebdah/goldie/v2` | v2.5+ |
| Property-based | `pgregory.net/rapid` | v1.1+ |
| Test framework | `tkrop/go-testing` | v0.1+ |
| E2E HTTP | `gavv/httpexpect` | v2.16+ |
| HTTP mock | `httptest` (stdlib) | Go 1.21+ |
| Diff | `google/go-cmp` | v0.6+ |
| Lint | `golangci-lint` | v1.55+ |