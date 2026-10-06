# Каталог `cmd`

## Назначение

Каталог `cmd` содержит точку входа в приложение `forumdump`. Здесь находится основной файл `main.go`, который отвечает за инициализацию CLI-интерфейса с использованием библиотеки `cobra` и обработку аргументов командной строки.

## Реализация

- **Основной файл:** `main.go` - точка входа в приложение
- **CLI Framework:** Использует `github.com/spf13/cobra` для создания командной строки
- **Конфигурация:** Поддержка загрузки конфигурации через `github.com/spf13/viper`
- **Флаги:** Обработка всех флагов, указанных в ТЗ (url, cookies, out, format, pages, concurrency, delay, attachments, retries, user-agent, config, verbose)
- **Аргументы:** Валидация URL и других входных параметров
- **Инициализация:** Инициализация всех модулей приложения (config, auth, fetcher, parser, storage, downloader, paginator)

## Связанные модули

- `internal/config` - парсинг флагов и конфига
- `internal/auth` - загрузка cookies
- `internal/fetcher` - HTTP-клиент
- `internal/parser` - парсинг HTML
- `internal/model` - модель данных
- `internal/storage` - сохранение данных
- `internal/downloader` - скачивание вложений
- `internal/paginator` - определение страниц