# Спецификация тестов для пакета `internal/fetcher`

## 1. Уровни тестирования

| Уровень | Покрытие | Инструменты |
|---|---|---|
| Unit | HTTP-запросы | `testing`, `testify` |
| Integration | моки HTTP | `httptest`, `gock` |
| E2E | полный прогон на локальном mock-сервере | `httptest`, `httpexpect` |
| CLI | флаги, exit-коды | `os/exec` |

## 2. Тест-кейсы

### TC-07. Сеть / устойчивость

| ID | Сценарий | Ожидание |
|---|---|---|
| 07.1 | 500, затем 200 | retry успешен |
| 07.2 | 429 с `Retry-After` | пауза согласно заголовку |
| 07.3 | Таймаут | retry, затем ошибка |
| 07.4 | Обрыв соединения на 5-й странице | частичный дамп |
| 07.5 | Ctrl+C (SIGINT) | graceful shutdown |
| 07.6 | `--delay=1s` | интервал ≥1s |

### TC-08. Cookies (интеграция модуля auth)

| ID | Сценарий | Ожидание | Тест |
|---|---|---|---|
| 08.1 | raw cookies из jar | отправляются в заголовке `Cookie` запроса | `TestUnitRawCookiesSent` |
| 08.2 | просроченные cookies из jar | не отправляются; валидные — отправляются | `TestUnitExpiredCookiesNotSent` |
| 08.3 | `Jar == nil` | поведение без изменений, заголовка `Cookie` нет (обратная совместимость) | `TestUnitNilJarNoCookies` |
| 08.4 | cookies с domain-scope | отправляются только на совпадающие хосты; чужие домены — нет | `TestIntegrationDomainScopedMatching` |
| 08.5 | `Set-Cookie` в ответе | сохраняется в jar и отправляется в следующем запросе (персистентность сессии) | `TestIntegrationSessionCookiesPersisted` |
| 08.6 | полный пайплайн: `config.ParseCookies` → `auth.Load` → fetch | raw-источник прикрепляется; значения cookies (`t0k3n`) не появляются в логах | `TestIntegrationAuthModuleRawSource` |

- Тесты собраны в `cookies_test.go` (suite `cookiesTestSuite`, пакет `fetcher_test`).
- Лог-безопасность: в структурированных логгах появляется только debug-строка «cookie jar attached to fetch client» без значений cookies.

## 3. Integration тесты

- Тестирование с моками HTTP запросов с разными ответами (200, 404, 500, 429)
- Проверка работы механизма retry
- Проверка обработки таймаутов
- Проверка работы с заголовками

## 4. E2E тесты

- Полный прогон на mock-сервере с разными сценариями
- Проверка корректной обработки всех типов ошибок сети