# Документация по тестированию

В этом разделе собрана вся документация по тестам проекта.

## Содержание

- **[test-plans.md](test-plans.md)** — тест-планы по функциям: основные и граничные тест-кейсы, привязка к тестам в коде.
- **[integration-requests.md](integration-requests.md)** — примеры HTTP-запросов и ответов для Jenkins, Gitea и вебхука; условия запуска интеграционных тестов.

## Запуск тестов

- **Юнит-тесты:** `make test-unit` (или `go test -race ./internal/... ./pkg/...`)
- **Все тесты (юнит + интеграция):** `make test`
- **Покрытие:** `make cover`; HTML-отчёт: `make cover-html` → `coverage.html`
- **Интеграционные тесты:** `make test-integration` (требуют переменные окружения, см. [integration-requests.md](integration-requests.md))

Тесты размещены рядом с кодом: `*_test.go` в соответствующих пакетах.
