# Архитектура

## Компоненты

Сервис состоит из точки входа (CLI с командами `run` и `check`), HTTP-сервера, процессора событий с очередью и воркерами, клиентов Jenkins и Gitea.

- **Точка входа**: `cmd/webhook-service` — разбор команд (`run`, `check`), загрузка конфига, запуск сервера или проверки.
- **HTTP-сервер**: приём вебхуков, проверка подписи, декодирование payload, постановка событий в очередь.
- **Процессор**: очередь событий, пул воркеров, проверки PR по правилам `checks`, опрос Jenkins, генерация комментария по шаблону, публикация в Gitea.
- **Проверки**: пакет `internal/checks` сопоставляет целевую ветку с регулярными выражениями и выполняет проверку по `type`. Сейчас реализован `file_blacklist`.
- **Jenkins**: REST API — получение списка джоб по `job_root`, ожидание появления джобы по regex.
- **Gitea**: REST API — список файлов PR, статус коммита, публикация комментария в issue/PR.

## Поток данных

```mermaid
sequenceDiagram
    participant Gitea
    participant Server
    participant Processor
    participant Jenkins
    participant GiteaAPI

    Gitea->>Server: POST /webhook (pull_request)
    Server->>Server: Проверка X-Gitea-Event, подпись, JSON
    Server->>Processor: Enqueue(evt)
    Server-->>Gitea: 202 Accepted

    loop Воркер
        Processor->>Processor: Правило репозитория, checks этого репозитория
        Processor->>GiteaAPI: GET .../pulls/{pr}/files
        GiteaAPI-->>Processor: files[]
        Processor->>Processor: Оценка по type (file_blacklist)
        Processor->>GiteaAPI: POST .../statuses/{head.sha}
        Processor->>Processor: Правило репозитория, action opened/reopened
        Processor->>Processor: Шаблон job_pattern → regex
        loop Опрос до timeout
            Processor->>Jenkins: GET .../api/json?tree=jobs[name,url,fullName]
            Jenkins-->>Processor: jobs[]
            Processor->>Processor: Поиск по regex
        end
        Processor->>Processor: Шаблон комментария (success/failure)
        Processor->>GiteaAPI: POST .../issues/{pr}/comments
        GiteaAPI-->>Processor: 201
    end
```

**Вход**: HTTP POST на `/webhook` с телом — JSON события Gitea `pull_request`.  
**Обработка**: событие в очередь → воркер ищет правило по `repository.full_name` → выполняет проверки из `repositories[].checks`, если целевая ветка `pull_request.base.ref` совпала и действие `opened`, `reopened` или `synchronized`, и публикует статус коммита на `pull_request.head.sha` → для действий `opened` и `reopened` шаблон `job_pattern` → опрос Jenkins → комментарий в Gitea. Репозиторий без поля `checks` проверяется только через Jenkins.  
**Выход**: статус коммита в Gitea (`success`, если файлы из чёрного списка не изменены, `failure`, если изменён хотя бы один) и, для настроенных репозиториев, комментарий в PR.

## Пакеты / модули

| Путь | Назначение |
|------|------------|
| `cmd/webhook-service` | Точка входа: команды `run`, `check`; флаги `-config`, `-debug`. |
| `internal/config` | Загрузка YAML, структуры конфига, `Validate()`, `GetRepositoryRule()`, `NewHTTPClient()`. |
| `internal/server` | HTTP-сервер: `GET /health`, `POST /webhook`; проверка подписи HMAC-SHA256; запуск/остановка процессора. |
| `internal/processor` | Очередь, пул воркеров, `Enqueue`, проверки PR, обработка Jenkins-правила и комментария. |
| `internal/checks` | Выбор правил по целевой ветке, glob-сопоставление путей, оценка `file_blacklist`. |
| `internal/jenkins` | Клиент: `WaitForJob`, `GetJobs`, `CheckAccessibility`, `CheckJobRootExists`; API `tree=jobs[name,url,fullName]`. |
| `internal/gitea` | Клиент: `PostComment`, `ListPullRequestFiles`, `CreateCommitStatus`, `CheckAccessibility`, `GetRepository`. |
| `pkg/webhook` | Типы событий Gitea: `PullRequestEvent`, `PullRequest`, `PRBranch`, `Repository`, `Sender`. |

## Диаграмма компонентов

```mermaid
flowchart LR
    subgraph cmd
        main[main.go]
        run[run.go]
        check[check.go]
    end
    subgraph internal
        config[config]
        server[server]
        processor[processor]
        checks[checks]
        jenkins[jenkins]
        gitea[gitea]
    end
    subgraph pkg
        webhook[webhook]
    end
    main --> run
    main --> check
    run --> config
    run --> server
    run --> processor
    run --> jenkins
    run --> gitea
    check --> config
    check --> jenkins
    check --> gitea
    server --> config
    server --> processor
    server --> webhook
    processor --> config
    processor --> checks
    processor --> jenkins
    processor --> gitea
    processor --> webhook
    checks --> config
```

Команда `run`: загрузка конфига → создание HTTP-клиентов (Jenkins, Gitea) → создание процессора и сервера → обработка SIGINT/SIGTERM → `srv.Run(ctx)`. Команда `check`: загрузка конфига → проверки (файл, валидация, сервер, Jenkins, Gitea, репозитории и джобы).

## Карта файлов и каталогов

Назначение основных каталогов и файлов для навигации по проекту.

**Корень:** [go.mod](../go.mod), [go.sum](../go.sum) — модуль и зависимости; [Makefile](../Makefile) — сборка, тесты, линт, Docker; [config.example.yaml](../config.example.yaml) — пример конфига; [Dockerfile](../Dockerfile), [docker-compose.yml](../docker-compose.yml); [README.md](../README.md); [doc/](../doc/) — доп. документация (напр. [TZ_CHECK_COMMAND.md](../doc/TZ_CHECK_COMMAND.md)).

**cmd/webhook-service:** [main.go](../cmd/webhook-service/main.go) — разбор команд `run`/`check`, `setupLogger`; [run.go](../cmd/webhook-service/run.go) — команда `run`; [check.go](../cmd/webhook-service/check.go) — команда `check`.

**internal/config:** [config.go](../internal/config/config.go) — структуры конфига, `Load`, `Validate`, `GetRepositoryRule`, `NewHTTPClient`; [config_test.go](../internal/config/config_test.go) — тесты.

**internal/server:** [server.go](../internal/server/server.go) — HTTP `GET /health`, `POST /webhook`, проверка подписи, запуск/остановка процессора; [server_test.go](../internal/server/server_test.go) — тесты.

**internal/processor:** [processor.go](../internal/processor/processor.go) — очередь, воркеры, проверки и обработка Jenkins; [processor_test.go](../internal/processor/processor_test.go), [checks_test.go](../internal/processor/checks_test.go) — тесты.

**internal/checks:** [checks.go](../internal/checks/checks.go) — `Applicable`, `Evaluate`; [glob.go](../internal/checks/glob.go) — `MatchFile`; тесты в том же каталоге.

**internal/jenkins:** [client.go](../internal/jenkins/client.go) — клиент Jenkins API; [client_test.go](../internal/jenkins/client_test.go) — тесты; [integration_test.go](../internal/jenkins/integration_test.go) — интеграционные тесты (build tag `integration`).

**internal/gitea:** [client.go](../internal/gitea/client.go) — клиент Gitea API (`PostComment`, `ListPullRequestFiles`, `CreateCommitStatus`); [client_test.go](../internal/gitea/client_test.go) — тесты; [integration_test.go](../internal/gitea/integration_test.go) — интеграционные тесты (build tag `integration`).

**pkg/webhook:** [types.go](../pkg/webhook/types.go) — типы событий Gitea; [types_test.go](../pkg/webhook/types_test.go) — тесты.

**Тестирование:** вся документация по тестам — в [docs/test/](test/README.md): тест-планы по функциям ([test-plans.md](test/test-plans.md)), примеры запросов к Jenkins/Gitea ([integration-requests.md](test/integration-requests.md)). Интеграционные тесты в `internal/jenkins` и `internal/gitea` компилируются с тегом `integration` и пропускаются без переменных окружения.

**Конфиг и CI:** конфиг приложения — YAML по пути из `-config`. CI: [.github/workflows/ci.yml](../.github/workflows/ci.yml).

## Контекст для агентов

- **Стек:** Go 1.22. Точка входа: [cmd/webhook-service](../cmd/webhook-service) (команды `run`, `check`; флаги `-config`, `-debug`). Конфиг: один YAML, загрузка в [internal/config](../internal/config/config.go) — `Load(path)`, `Validate()`. Проверки PR: поле `repositories[].checks`, исполнение в [internal/checks](../internal/checks/checks.go) и [internal/processor](../internal/processor/processor.go).
- **Где что искать:** компоненты и поток — разделы выше; пакеты — таблица «Пакеты / модули»; файлы — «Карта файлов и каталогов». Термины — [glossary.md](glossary.md). Контракт API — [api-surface.md](api-surface.md). Типовые изменения — [common-tasks.md](common-tasks.md). Стиль кода — [conventions.md](conventions.md).
