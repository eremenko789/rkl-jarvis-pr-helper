# Правила для разработчиков

## Команды Makefile

Основные цели (см. [Makefile](../Makefile)):

| Команда | Описание |
|---------|----------|
| `make build` | Сборка бинарника в `bin/webhook-service`. |
| `make test` | Юнит-тесты с `-race` и покрытием, интеграционные тесты, вывод `go tool cover -func`, генерация `coverage.out` и `coverage.html`. |
| `make test-unit` | Только юнит-тесты с `-race` по `./internal/...` и `./pkg/...` (без покрытия). |
| `make test-integration` | Только интеграционные тесты (build tag `integration`); требуются переменные окружения, см. [docs/test/integration-requests.md](test/integration-requests.md). |
| `make lint` | `go vet ./...`. |
| `make fmt` | `gofmt -w` по всем Go-файлам. |
| `make tidy` | `go mod tidy`. |
| `make ci` | Последовательно: tidy, lint, test, build. |
| `make clean` | Удаление `bin/`, `coverage.out` и `coverage.html`. |
| `make docker-build` | Сборка Docker-образа. |
| `make docker-run` | Сборка и запуск контейнера (порт 8081, монтирование config.example.yaml). |
| `make docker-compose` | `docker compose up -d --build`. |
| `make run-server` | `go run ./cmd/webhook-service run -config ./config.yaml --debug`. |
| `make run-check` | `go run ./cmd/webhook-service check -config ./config.yaml`. |

Перед коммитом рекомендуется выполнять `make ci`.

## CI pipeline

Workflow: [.github/workflows/ci.yml](../.github/workflows/ci.yml).

- **Триггеры**: pull_request; push в ветку `main`; push тегов `v*.*.*`.
- **Job test**: checkout, Go 1.22, `make tidy`, `make lint`, `make test` (тесты + покрытие + HTML), загрузка артефактов `coverage.out` и `coverage.html`.
- **Job build**: после test, матрица сборки (linux/386, amd64, arm, arm64) → артефакты бинарников.
- **Job release**: только при push тега; скачивание артефактов сборки, создание GitHub Release с приложением всех бинарников (через softprops/action-gh-release).

## Линт и форматирование

- Линт: `go vet ./...` (цель `make lint`).
- Форматирование: `gofmt -w` по списку Go-файлов (цель `make fmt`). Стиль кода — стандартный gofmt.

## Тесты и покрытие

- **Тесты:** `make test` — юнит-тесты с покрытием, интеграционные тесты, генерация `coverage.out` и `coverage.html`. В CI артефактами публикуются оба файла. Только юнит без покрытия: `make test-unit`.
- **Интеграционные тесты:** тесты с build tag `integration` в `internal/jenkins/integration_test.go` и `internal/gitea/integration_test.go`. Запуск: `make test-integration`. Без переменных окружения (JENKINS_BASE_URL, GITEA_BASE_URL, GITEA_TOKEN) тесты пропускаются (skip). Примеры запросов и условия запуска: [docs/test/integration-requests.md](test/integration-requests.md).
- **Документация по тестам:** вся в поддиректории [docs/test/](test/): оглавление — [test/README.md](test/README.md); тест-планы по функциям — [test/test-plans.md](test/test-plans.md).

Файлы тестов: `internal/config/config_test.go`, `internal/server/server_test.go`, `internal/processor/processor_test.go`, `internal/jenkins/client_test.go`, `internal/gitea/client_test.go`, `pkg/webhook/types_test.go`.

## Релизы и артефакты

- Релиз создаётся при пуше тега вида `v*.*.*` (например `v1.0.0`).
- В Release попадают бинарники из job build: `webhook-service-linux-386`, `webhook-service-linux-amd64`, `webhook-service-linux-arm`, `webhook-service-linux-arm64`.
- Имя и описание релиза по умолчанию совпадают с именем тега; при необходимости их можно настроить в workflow или в интерфейсе GitHub.
