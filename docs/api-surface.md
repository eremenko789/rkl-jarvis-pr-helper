# Публичная поверхность API

HTTP-эндпоинты, форматы запросов и ответов, экспортируемые типы и точки расширения.

## HTTP-эндпоинты

Реализация: [internal/server/server.go](../internal/server/server.go). Маршруты регистрируются в `Server.New()`.

### GET /health

- **Назначение**: проверка работоспособности сервиса (health check).
- **Метод**: GET.
- **Параметры**: нет.
- **Ответ**: статус 200, тело `ok` (plain text).
- **Заголовки**: без особых требований.

### POST /webhook

- **Назначение**: приём вебхуков Gitea о событии Pull Request.
- **Метод**: POST.
- **Заголовки**:
  - `X-Gitea-Event`: тип события (обрабатывается только `pull_request`).
  - `X-Gitea-Signature`: подпись тела запроса (HMAC-SHA256, hex). Если в конфиге задан `server.webhook_secret`, подпись проверяется; при несовпадении возвращается 401.
- **Тело**: JSON — payload события Gitea. Структура соответствует [pkg/webhook/types.go](../pkg/webhook/types.go): `PullRequestEvent` с полями `action`, `number`, `pull_request`, `repository`, `sender` и т.д.
- **Ответы**:
  - **202 Accepted** — событие принято и поставлено в очередь.
  - **400 Bad Request** — неверный тип события, не удалось прочитать тело или распарсить JSON.
  - **401 Unauthorized** — неверная или отсутствующая подпись (при настроенном секрете).
  - **503 Service Unavailable** — очередь процессора переполнена.

Других HTTP-эндпоинтов нет.

## Форматы запросов и ответов

- **Вебхук**: входящий JSON — структура Gitea Pull Request event. Ключевые поля для сервиса: `repository.full_name`, `pull_request.number`, `action`, `sender.login`, `pull_request.title`, `pull_request.base.ref` (целевая ветка), `pull_request.head.sha` (коммит для статуса). См. `webhook.PullRequestEvent` и `webhook.PRBranch`.
- **Ответы**: тело только у `/health` (строка `ok`). На `/webhook` при успехе тело не задаёно (202); при ошибках — стандартные сообщения `http.Error` (текст в теле).

## Экспортируемые пакеты и типы

- **internal** — не предназначен для импорта извне модуля; все символы доступны только внутри проекта.
- **pkg/webhook** — публичный пакет:
  - `PullRequestEvent`, `PullRequest`, `PRBranch`, `Repository`, `Sender` — структуры для декодирования вебхука. `PRBranch` содержит `ref` и `sha` веток `base` и `head`.
  - `PullRequest.DisplayName()` — метод.

Типы конфига (например `config.Config`, `config.RepositoryRule`) экспортируются в рамках модуля и используются в `cmd` и `internal`; для внешних потребителей контрактом является YAML и описание в [configuration.md](configuration.md).

## Точки расширения и контракт

- **Добавление эндпоинта**: в `server.New()` зарегистрировать новый обработчик через `mux.HandleFunc`; при необходимости добавить флаг или конфиг. Обработчики принимают `(http.ResponseWriter, *http.Request)`.
- **Изменение формата вебхука**: изменить структуры в `pkg/webhook/types.go` и парсинг в `server.handleWebhook`; при добавлении новых полей в шаблоны комментариев — расширить `data` в `processor.processEvent` и описать в [configuration.md](configuration.md).
- **Новые поля конфига**: добавить поля в структуры в [internal/config/config.go](../internal/config/config.go), обработать в `Validate()` (значения по умолчанию и проверки), обновить [config.example.yaml](../config.example.yaml) и документацию.
- **Новый тип проверки**: общее правило — `config.CheckRule` (секция `checks`). Тип задаётся полем `type`, настройки — вложенным объектом с тем же именем. Реализация оценки — ветка `checks.Evaluate`. Подробные шаги — в [common-tasks.md](common-tasks.md).
- **Контракт Jenkins**: используется дерево API `jobs[name,url,fullName]` (захардкожено в [internal/jenkins/client.go](../internal/jenkins/client.go)). Изменение набора полей потребует правки структур и запросов.
- **Контракт Gitea**:
  - комментарий — `POST /repos/{owner}/{repo}/issues/{index}/comments` с телом `{"body": "..."}`;
  - файлы PR — `GET /repos/{owner}/{repo}/pulls/{index}/files` (пагинация `page` и `limit`);
  - статус коммита — `POST /repos/{owner}/{repo}/statuses/{sha}` с телом `state`, `context`, `description`.
  - Заголовок аутентификации: `Authorization: token <token>`. Состояния статуса: `success`, `failure`, `error`.
