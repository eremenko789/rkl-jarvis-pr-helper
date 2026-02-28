# Конфигурация

## Формат и расположение конфига

Конфигурация задаётся одним YAML-файлом. Путь передаётся флагом `-config` (например `config.yaml`). Пример файла в репозитории: [config.example.yaml](../config.example.yaml).

Загрузка и валидация: [internal/config/config.go](../internal/config/config.go) — функция `Load(path string) (*Config, error)` и метод `Validate()`.

## Секции и ключи

### server

| Ключ | Тип | Обязательный | Описание |
|------|-----|--------------|----------|
| `listen_addr` | string | нет | Адрес прослушивания HTTP (по умолчанию `:8080`). |
| `webhook_secret` | string | да* | Секрет для проверки подписи вебхука (HMAC-SHA256). |
| `worker_pool_size` | int | нет | Размер пула воркеров (по умолчанию 4). |
| `queue_size` | int | нет | Размер очереди событий (по умолчанию 100). |

\* Если не задан, проверка подписи не выполняется (не рекомендуется для продакшена).

### jenkins

| Ключ | Тип | Обязательный | Описание |
|------|-----|--------------|----------|
| `base_url` | string | да | Базовый URL Jenkins (например `https://jenkins.example.com`). |
| `username` | string | нет | Имя пользователя для Basic Auth. |
| `api_token` | string | нет | API-токен (пароль для Basic Auth). |
| `poll_interval` | time.Duration | нет | Интервал опроса джоб (по умолчанию 15s). |
| `timeout` | time.Duration | нет | Таймаут ожидания джобы (по умолчанию 5m). |
| `insecure_skip_verify` | bool | нет | Игнорировать ошибки SSL-сертификатов (по умолчанию false). |

Длительности задаются как строки с суффиксом: `15s`, `5m` и т.д.

### gitea

| Ключ | Тип | Обязательный | Описание |
|------|-----|--------------|----------|
| `base_url` | string | да | Базовый URL API Gitea (например `https://gitea.example.com/api/v1`). |
| `token` | string | да | Персональный access token (заголовок `Authorization: token <token>`). |
| `insecure_skip_verify` | bool | нет | Игнорировать ошибки SSL (по умолчанию false). |

### repositories

Массив правил по репозиториям. Каждый элемент:

| Ключ | Тип | Обязательный | Описание |
|------|-----|--------------|----------|
| `name` | string | да | Полное имя репозитория в формате `owner/repo`. |
| `job_root` | string | нет | Корневая «папка» джоб в Jenkins (путь вида `folder/subfolder`; пустая строка — корень). |
| `job_pattern` | string | да | Регулярное выражение для имени джобы; поддерживает Go-шаблон (например `{{ .Number }}`). |
| `poll_interval` | time.Duration | нет | Интервал опроса для этого репозитория (иначе из `jenkins.poll_interval`). |
| `timeout` | time.Duration | нет | Таймаут для этого репозитория (иначе из `jenkins.timeout`). |
| `success_comment_template` | string | нет | Шаблон комментария при найденной джобе (Go template). |
| `failure_comment_template` | string | нет | Шаблон комментария при отсутствии джобы (Go template). |

В шаблонах (включая `job_pattern`) доступны поля: `Number`, `Title`, `Repo`, `Sender`, `Timeout`; в шаблоне успеха дополнительно `JobName`, `JobURL`.

## Значения по умолчанию и валидация

- **server**: `listen_addr` = `:8080`, `worker_pool_size` = 4, `queue_size` = 100, если не заданы или ≤ 0.
- **jenkins**: `poll_interval` = 15s, `timeout` = 5m при не заданных или ≤ 0. `base_url` обязателен.
- **gitea**: `base_url` и `token` обязательны.
- **repositories**: для каждого правила обязательны `name` и `job_pattern`. Если не заданы `poll_interval`/`timeout`, берутся из секции `jenkins`. Если не заданы шаблоны комментариев, подставляются строки по умолчанию (см. [internal/config/config.go](../internal/config/config.go), метод `Validate()`).

Валидация выполняется в `Config.Validate()` после разбора YAML; при ошибке `Load` возвращает ошибку.

## Ссылка на пример конфига

Полный пример со всеми секциями и комментариями: [config.example.yaml](../config.example.yaml).
