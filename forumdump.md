# Техническое задание (ТЗ) на утилиту `forumdump`

**Версия:** 1.1 (с выбранными библиотеками)
**Дата:** 2026-10-05
**Язык реализации:** Go (≥ 1.21)

---

## 1. Общие сведения

**Название:** `forumdump` — CLI-утилита для архивации тем форумов.

**Назначение:** Обход темы форума по страницам (от первой до последней), извлечение комментариев, вложенных ответов и прикреплённых материалов, сохранение в структурированном виде на диск. Поддержка авторизации через cookies браузера.

**Область применения:** Архивация, офлайн-чтение, резервное копирование обсуждений.

---

## 2. Функциональные требования

### 2.1 Входные данные

| Параметр | Обязательность | Описание |
|---|---|---|
| `--url` | да | Ссылка на тему форума (абсолютный URL). |
| `--cookies` | нет | Источник cookies: `browser:chrome|firefox|edge|safari`, `file:/path/to/cookies.txt` (Netscape), `raw:"k=v; k2=v2"`. |
| `--out` | нет | Каталог вывода (по умолчанию `./dump_<timestamp>`). |
| `--format` | нет | `json` (по умолчанию), `jsonl`, `markdown`, `html`. |
| `--pages` | нет | Диапазон: `all` (по умолчанию), `1-5`, `3`, `3-`. |
| `--concurrency` | нет | Число параллельных запросов (по умолчанию 1–2). |
| `--delay` | нет | Задержка между запросами (по умолчанию 500ms). |
| `--attachments` | нет | `download` (по умолчанию), `links`, `skip`. |
| `--retries` | нет | Кол-во повторов при ошибках (по умолчанию 3). |
| `--user-agent` | нет | Кастомный UA. |
| `--config` | нет | Путь к YAML/JSON-конфигу с правилами парсинга. |
| `--verbose` | нет | Подробный лог. |

### 2.2 Основной сценарий

1. Валидация URL и загрузка конфигурации.
2. Инициализация HTTP-клиента с cookies и заголовками.
3. Загрузка первой страницы темы.
4. Определение последней страницы (пагинация).
5. Постраничный обход:
   - Загрузка HTML.
   - Парсинг: список сообщений (автор, дата, ID, тело), дерево ответов, вложения.
   - Скачивание вложений (если включено).
6. Сохранение результатов в выбранном формате.
7. Формирование `manifest.json` с метаданными обхода.

### 2.3 Извлекаемые данные

- **Тема:** заголовок, URL, ID темы, автор, дата создания, число страниц.
- **Сообщение:** ID, автор (имя + ссылка на профиль), дата/время (ISO 8601), текст (HTML + plaintext), ссылка на пост, номер поста, ID родителя (для ответов), уровень вложенности.
- **Вложение:** имя файла, URL, MIME, размер (если известен), локальный путь, SHA-256.

### 2.4 Выходные артефакты

```
out/
├── manifest.json          # метаданные обхода
├── topic.json             # все сообщения и вложения
├── pages/                 # сырой HTML (опционально)
│   ├── page_001.html
│   └── ...
└── attachments/
    ├── <post_id>/
    │   └── file.pdf
    └── ...
```

### 2.5 Нефункциональные требования

- **Надёжность:** повторные попытки с экспоненциальной задержкой при 5xx/429.
- **Идемпотентность:** при повторном запуске не скачивать уже сохранённые вложения.
- **Вежливость:** уважение `robots.txt` (опционально), лимит RPS.
- **Логирование:** структурированный лог (slog) в stderr, отчёт — в stdout.
- **Кроссплатформенность:** Linux, macOS, Windows.
- **Безопасность:** cookies не пишутся в логи.
- **Производительность:** обработка темы из 1000 страниц без утечек памяти.

---

## 3. Выбор библиотек и фреймворков

### 3.1 CLI Framework

| Библиотека | Обоснование |
|---|---|
| **`github.com/spf13/cobra`** | Де-факто стандарт для CLI на Go. Поддержка подкоманд, флагов, генерация справки, автодополнение. |

**Дополнительно:** `github.com/spf13/viper` — для загрузки конфигурации (YAML/JSON), поддержки ENV-переменных и каскадного переопределения флагов.

### 3.2 HTTP-клиент и устойчивость

| Библиотека | Обоснование |
|---|---|
| **`github.com/hashicorp/go-retryablehttp`** | Обёртка над `net/http` с настраиваемыми retry, exponential backoff, обработкой 429/5xx. |
| **`golang.org/x/time/rate`** | Rate limiting (RPS) на уровне HTTP-клиента. |

### 3.3 Авторизация (cookies)

| Библиотека | Обоснование |
|---|---|
| **`codeberg.org/sorl/browsercookie`** | Загрузка cookies из Chrome, Chromium, Brave, Edge, Firefox, Safari на macOS, Linux, Windows. |
| **`github.com/aisk/browsercookies`** | Fallback: загрузка cookies из FireFox и Chrome напрямую в `http.CookieJar`. |

### 3.4 Парсинг HTML

| Библиотека | Обоснование |
|---|---|
| **`github.com/PuerkitoBio/goquery`** | jQuery-подобный API для навигации по DOM. Стандарт для скрапинга на Go. |
| **`golang.org/x/net/html`** | Низкоуровневый парсер (используется goquery под капотом). |
| **`github.com/cybergodev/html`** | Извлечение текста, конвертация в Markdown, автоматическое определение кодировки. |

### 3.5 Скачивание вложений

| Библиотека | Обоснование |
|---|---|
| **`github.com/dracory/base/files`** | `DownloadURL(url, path)` — стриминговая загрузка файлов без загрузки в память. |

**Альтернатива:** собственная реализация на `io.Copy` с `net/http`.

### 3.6 Тестирование

| Библиотека | Назначение | Обоснование |
|---|---|---|
| **`github.com/stretchr/testify`** | Assertions, mocks, require | Стандарт для unit-тестов в Go. |
| **`github.com/sebdah/goldie/v2`** | Golden-файлы | Сравнение JSON/Markdown с эталонами, обновление через `-update`. |
| **`pgregory.net/rapid`** | Property-based testing | Генерация случайных входных данных, `MakeFuzz` для fuzz-таргетов. |
| **`github.com/tkrop/go-testing`** | Параметризованные тесты | Framework для table-driven тестов, mock-сеть, параллельный запуск. |
| **`github.com/gavv/httpexpect`** | E2E HTTP-тесты | Удобное тестирование HTTP-взаимодействия. |

### 3.7 Логирование

| Библиотека | Обоснование |
|---|---|
| **`log/slog`** (stdlib, Go 1.21+) | Структурированное логирование, JSON/текстовый формат, уровни. |

### 3.8 Хеширование

| Библиотека | Обоснование |
|---|---|
| **`crypto/sha256`** (stdlib) | SHA-256 для вложений. |

---

## 4. Архитектура (модули)

```
cmd/forumdump/main.go        # CLI (cobra + viper)
internal/config/             # парсинг флагов и конфига
internal/auth/               # загрузка cookies (browsercookie)
internal/fetcher/            # HTTP-клиент (retryablehttp + rate limiter)
internal/parser/             # интерфейс Parser + реализации (goquery)
internal/model/              # Topic, Post, Attachment
internal/storage/            # JSON/JSONL/Markdown writers
internal/downloader/         # скачивание вложений
internal/paginator/          # определение границ страниц
```

**Расширяемость:** интерфейс `Parser` позволяет добавлять новые движки форумов:

```go
type Parser interface {
    Match(html []byte, url string) bool
    ParsePage(html []byte, url string) (*Page, error)
    LastPage(html []byte) (int, error)
    PageURL(base string, n int) string
}
```

---

## 5. Схема данных (JSON)

```json
{
  "topic": {
    "id": "12345",
    "title": "Название темы",
    "url": "https://forum.example/threads/12345/",
    "author": {"name": "user", "profile_url": "..."},
    "created_at": "2023-01-01T12:00:00Z",
    "pages_total": 10
  },
  "posts": [
    {
      "id": "100",
      "parent_id": null,
      "level": 0,
      "author": {"name": "user", "profile_url": "..."},
      "created_at": "2023-01-01T12:01:00Z",
      "body_html": "<p>...</p>",
      "body_text": "...",
      "attachments": [
        {
          "name": "doc.pdf",
          "url": "https://forum.example/attachments/1",
          "mime": "application/pdf",
          "size": 12345,
          "local_path": "attachments/100/doc.pdf",
          "sha256": "..."
        }
      ]
    }
  ],
  "manifest": {
    "dumped_at": "2024-01-01T00:00:00Z",
    "pages_fetched": 10,
    "errors": [],
    "incomplete": false,
    "tool_version": "1.0.0"
  }
}
```

---

## 6. Обработка ошибок и exit-коды

| Код | Значение |
|---|---|
| 0 | Успех |
| 1 | Общая ошибка (сеть, парсинг) |
| 2 | Ошибка аргументов CLI |
| 3 | Ошибка записи на диск |
| 4 | Ошибка авторизации |
| 5 | Частичный успех (есть предупреждения) |

---

## 7. Ограничения и допущения

- Утилита не обходит CAPTCHA и anti-bot (Cloudflare).
- Не гарантируется работа при изменении HTML-разметки форума; поддержка через конфиг/плагины.
- Cookies пользователя используются «как есть»; утилита не выполняет вход по логину/паролю.
- Соблюдение авторских прав и ToS форума — ответственность пользователя.

---

## Приложение А: Сводная таблица зависимостей

| Компонент | Библиотека | Версия (рекоменд.) |
|---|---|---|
| CLI | `spf13/cobra` | v1.8+ |
| Конфиг | `spf13/viper` | v1.18+ |
| HTTP retry | `hashicorp/go-retryablehttp` | v0.7+ |
| Rate limit | `golang.org/x/time/rate` | latest |
| Cookies | `codeberg.org/sorl/browsercookie` | latest |
| HTML parsing | `PuerkitoBio/goquery` | v1.9+ |
| HTML→Markdown | `cybergodev/html` | v1.3+ |
| File download | `dracory/base/files` | v0.39+ |
| Assertions | `stretchr/testify` | v1.9+ |
| Golden files | `sebdah/goldie/v2` | v2.5+ |
| Property-based | `pgregory.net/rapid` | v1.1+ |
| Test framework | `tkrop/go-testing` | v0.1+ |
| E2E HTTP | `gavv/httpexpect` | v2.16+ |
| Logging | `log/slog` (stdlib) | Go 1.21+ |