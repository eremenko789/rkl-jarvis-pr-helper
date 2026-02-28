# Примеры запросов для интеграционных тестов

Эти примеры можно использовать для ручных запросов (curl, Postman) и для написания интеграционных тестов. Интеграционные тесты запускаются только при наличии переменных окружения (см. раздел «Условия запуска»).

## Условия запуска интеграционных тестов

- Тесты с build tag `integration` компилируются и запускаются отдельно: `go test -tags=integration ./...`.
- Рекомендуется проверять переменную окружения (например `RUN_INTEGRATION=1` или `JENKINS_BASE_URL` / `GITEA_BASE_URL`) и пропускать тесты (`t.Skip`), если окружение не настроено, чтобы в CI по умолчанию не обращаться к внешним сервисам.

---

## Jenkins API

Базовый URL: например `https://jenkins.example.com`. В конфиге задаётся `jenkins.base_url`, при наличии `username` и `api_token` используется Basic Auth.

### Список джоб в корне

**Запрос:**

```http
GET /api/json?tree=jobs[name,url,fullName] HTTP/1.1
Host: jenkins.example.com
Authorization: Basic <base64(username:api_token)>
```

Полный URL: `https://jenkins.example.com/api/json?tree=jobs[name,url,fullName]`

**Пример ответа (200 OK):**

```json
{
  "jobs": [
    {
      "name": "build-42",
      "url": "https://jenkins.example.com/job/build-42/",
      "fullName": "build-42"
    },
    {
      "name": "deploy",
      "url": "https://jenkins.example.com/job/deploy/",
      "fullName": "deploy"
    }
  ]
}
```

Пустой список: `{"jobs": []}`.

### Список джоб в папке (job_root)

При `job_root: "folder/subfolder"` путь к API формируется как `/job/folder/job/subfolder/api/json`.

**Запрос:**

```http
GET /job/folder/job/subfolder/api/json?tree=jobs[name,url,fullName] HTTP/1.1
Host: jenkins.example.com
Authorization: Basic <base64(username:api_token)>
```

Полный URL: `https://jenkins.example.com/job/folder/job/subfolder/api/json?tree=jobs[name,url,fullName]`

**Пример ответа (200 OK):** тот же формат с полем `jobs` (массив объектов с `name`, `url`, `fullName`).

### Проверка доступности (CheckAccessibility)

Тот же эндпоинт корня: `GET {base_url}/api/json` без параметра `tree` или с ним. Код 200 — доступен; 401/403 — ошибка аутентификации; 404 — Jenkins не найден.

### Проверка существования job root (CheckJobRootExists)

Запрос: `GET {base_url}/job/folder/job/subfolder/api/json`. Коды: 200 — существует; 404 — не найден; 403 — доступ запрещён.

### Примеры для тестов

- **200 с непустым списком** — тело как выше с массивом `jobs`.
- **200 с пустым списком** — `{"jobs": []}`.
- **401 Unauthorized** — без заголовка Authorization или с неверными учётными данными.
- **403 Forbidden** — нет прав на просмотр.
- **404 Not Found** — неверный путь к job root.

---

## Gitea API

Базовый URL: например `https://gitea.example.com/api/v1` (из конфига `gitea.base_url`). Заголовок аутентификации: `Authorization: token <token>`.

### Проверка доступности (CheckAccessibility)

**Запрос:**

```http
GET /user HTTP/1.1
Host: gitea.example.com
Authorization: token your_access_token
```

Полный URL: `https://gitea.example.com/api/v1/user`

**Ответ:** 200 OK с телом с информацией о пользователе. 401/403 — ошибка аутентификации.

### Проверка репозитория (GetRepository)

**Запрос:**

```http
GET /repos/{owner}/{repo} HTTP/1.1
Host: gitea.example.com
Authorization: token your_access_token
```

Пример: `GET https://gitea.example.com/api/v1/repos/myorg/myrepo`

**Ответ:** 200 OK — репозиторий существует. 404 — не найден; 403 — доступ запрещён; 401 — не авторизован.

### Создание комментария в issue/PR (PostComment)

**Запрос:**

```http
POST /repos/{owner}/{repo}/issues/{index}/comments HTTP/1.1
Host: gitea.example.com
Content-Type: application/json
Authorization: token your_access_token

{"body": "Текст комментария"}
```

Пример: `POST https://gitea.example.com/api/v1/repos/myorg/myrepo/issues/5/comments`

**Пример тела запроса:**

```json
{"body": "✅ Jenkins job build-5 detected: https://jenkins.example.com/job/build-5/"}
```

**Пример ответа (201 Created):**

```json
{
  "id": 123,
  "body": "✅ Jenkins job build-5 detected: https://jenkins.example.com/job/build-5/",
  "user": { "login": "bot" },
  "created_at": "2025-01-15T10:00:00Z"
}
```

Для интеграционных тестов: подставить реальные `owner`, `repo`, `index` и токен; при необходимости проверить появление комментария через GET списка комментариев к issue.

---

## Вебхук (POST /webhook)

Эндпоинт сервиса: `POST http://localhost:8080/webhook` (или другой `server.listen_addr`).

### Заголовки

- `X-Gitea-Event: pull_request` — обязательно для обработки (другие события приводят к 400).
- `X-Gitea-Signature` — при настроенном `server.webhook_secret` подпись тела запроса в формате HMAC-SHA256, hex. Допускается префикс `sha256=`. Подпись вычисляется так: `HMAC-SHA256(secret, body)` и кодируется в hex.

### Пример минимального payload (opened/reopened)

Сервис обрабатывает только `action` = `opened` или `reopened`. Остальные действия игнорируются (событие принимается 202, но комментарий не постится).

**Тело запроса (JSON):**

```json
{
  "action": "opened",
  "number": 5,
  "pull_request": {
    "number": 5,
    "title": "Add feature X",
    "body": "Description",
    "url": "https://gitea.example.com/myorg/myrepo/pulls/5"
  },
  "repository": {
    "id": 1,
    "name": "myrepo",
    "full_name": "myorg/myrepo",
    "html_url": "https://gitea.example.com/myorg/myrepo"
  },
  "sender": {
    "id": 10,
    "login": "developer",
    "full_name": "Developer Name"
  }
}
```

Для `reopened` достаточно заменить `"action": "reopened"`.

### Пример запроса с подписью (curl)

Секрет в конфиге: `webhook_secret: "mysecret"`. Подпись в hex: например, для тела `{"action":"opened"}` и секрета `mysecret` значение можно вычислить в Go или через openssl.

```bash
# Пример (подпись нужно вычислить по реальному телу и секрету)
curl -X POST http://localhost:8080/webhook \
  -H "X-Gitea-Event: pull_request" \
  -H "X-Gitea-Signature: sha256=<hex_signature>" \
  -H "Content-Type: application/json" \
  -d '{"action":"opened","number":5,"pull_request":{"number":5,"title":"Test","body":"","url":""},"repository":{"id":1,"name":"myrepo","full_name":"myorg/myrepo","html_url":""},"sender":{"id":1,"login":"u","full_name":""}}'
```

**Ожидаемые ответы:**

- **202 Accepted** — событие принято и поставлено в очередь (при валидном event и при необходимости верной подписи).
- **400 Bad Request** — неверный тип события, невалидный JSON или не удалось прочитать тело.
- **401 Unauthorized** — неверная или отсутствующая подпись при настроенном секрете.
- **503 Service Unavailable** — очередь процессора переполнена.

Эти примеры можно подставлять в интеграционные тесты (URL, тело, заголовки) для консистентности с документацией.
