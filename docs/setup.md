# Установка и запуск

## Быстрый старт

1. Скопировать пример конфигурации и заполнить реальные значения:
   ```bash
   cp config.example.yaml config.yaml
   ```
2. Запуск локально:
   ```bash
   go run ./cmd/webhook-service run -config config.yaml
   ```
   или через docker-compose:
   ```bash
   docker compose up --build
   ```

Подробности конфигурации — в [configuration.md](configuration.md).

## Локальный запуск

- Сборка: `make build` → бинарник в `bin/webhook-service`.
- Запуск сервиса: `./bin/webhook-service run -config config.yaml` или `make run-build` (если конфиг в `./config.yaml`).
- Отладка: `./bin/webhook-service run -config config.yaml --debug`.
- Проверка конфига и доступности сервисов: `./bin/webhook-service check -config config.yaml` (см. [doc/TZ_CHECK_COMMAND.md](../doc/TZ_CHECK_COMMAND.md)).

Адрес прослушивания задаётся в конфиге (`server.listen_addr`, по умолчанию `:8080`).

## Docker / docker-compose

- **Dockerfile**: мультистейдж (сборка на `golang:1.25`, образ на `ubuntu:noble`). Конфиг по умолчанию копируется как `/etc/webhook/config.yaml`; переменная `CONFIG_FILE` не используется в коде — путь передаётся в ENTRYPOINT. Порт 8080.
- Сборка образа: `make docker-build` или `docker build -t gitea-jenkins-webhook .`.
- Запуск контейнера с локальным конфигом: монтировать `config.yaml` в `/etc/webhook/config.yaml` и при необходимости задать `listen_addr` в конфиге так, чтобы он совпадал с пробросом порта (в [docker-compose.yml](../docker-compose.yml) порт 8081 снаружи — убедитесь, что в конфиге указан `:8081` при использовании compose).

**docker-compose**: сервис `webhook-service`, сборка из текущего каталога, монтирование `./config.yaml` в `/etc/webhook/config.yaml`, порты `8081:8081`. Для работы контейнера в конфиге должно быть `server.listen_addr: ":8081"`.

## Внешние системы (Gitea, Jenkins)

### Gitea

1. **Webhook**: в настройках репозитория (или организации) создать webhook для события «Pull Request». URL — адрес сервиса, например `http(s)://<host>:8080/webhook`. Секрет — тот же, что в `server.webhook_secret` (подпись проверяется по заголовку `X-Gitea-Signature`, HMAC-SHA256).
2. **Токен**: персональный access token с правом записи: комментарии к PR, чтение файлов pull request и публикация статусов коммита. Указать в `gitea.token`. API должен быть доступен по `gitea.base_url` (например `https://gitea.example.com/api/v1`).

### Jenkins

1. **Доступ**: сервис обращается к `GET <jenkins.base_url>/api/json` и к путям вида `/job/<job_root>/api/json?tree=jobs[name,url,fullName]`. Учётные данные — `jenkins.username` и `jenkins.api_token` (Basic Auth).
2. **Имена джоб**: имена джоб должны соответствовать `job_pattern` для соответствующего репозитория. Паттерн может содержать Go-шаблон (например `{{ .Number }}` для номера PR). Корневая «папка» джоб задаётся в `job_root` (пустая строка — корень Jenkins).

Проверка конфигурации и доступности перед запуском: `webhook-service check -config config.yaml`.
