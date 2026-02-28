# Тест-планы по функциям

Для каждой функции или группы указаны назначение, основные тест-кейсы и граничные/негативные. Связь с кодом — имена тестов или файлов (`*_test.go`).

---

## internal/config

### Load

**Назначение:** загрузить YAML с диска, распарсить, провалидировать и построить индекс репозиториев.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Валидный файл — конфиг загружен, RepoIndex заполнен, дефолты применены | `TestLoad` |
| Граничный | Файл не существует — ошибка | `TestLoad_FileNotExist` |
| Граничный | Невалидный YAML — ошибка unmarshal | `TestLoad_InvalidYAML` |
| Граничный | Пустой/минимальный YAML — ошибка валидации (обязательные поля) | `TestLoad_EmptyFile` |

### Validate

**Назначение:** проверить конфиг и выставить дефолты для необязательных полей.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Валидный конфиг — без ошибок, дефолты проставлены | `TestValidate_Valid` |
| Основной | Пустой listen_addr → :8080; worker_pool_size/queue_size ≤ 0 → 4 и 100 | `TestValidate_Defaults` |
| Негативный | jenkins.base_url пустой — ошибка | `TestValidate_MissingJenkinsURL` |
| Негативный | gitea.base_url пустой — ошибка | `TestValidate_MissingGiteaURL` |
| Негативный | gitea.token пустой — ошибка | `TestValidate_MissingGiteaToken` |
| Негативный | Репо без name (индекс 0) — ошибка | `TestValidate_RepoMissingName` |
| Негативный | Репо без job_pattern — ошибка | `TestValidate_RepoMissingJobPattern` |
| Граничный | У репо пустые poll_interval/timeout — наследуются из jenkins | `TestValidate_RepoInheritsIntervals` |
| Граничный | Пустые шаблоны комментариев — дефолтные строки | `TestValidate_DefaultTemplates` |

### GetRepositoryRule

**Назначение:** по full_name вернуть правило репозитория и флаг наличия.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Репо есть в индексе — правило и true | `TestGetRepositoryRule_Found` |
| Основной | Репо нет — пустое правило и false | `TestGetRepositoryRule_NotFound` |
| Граничный | RepoIndex == nil — вызывается buildIndex, затем поиск | `TestGetRepositoryRule_NilIndex` |

### NewHTTPClient

**Назначение:** создать HTTP-клиент с TLS и таймаутом.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | insecureSkipVerify true — клиент создан | `TestNewHTTPClient_InsecureSkipVerify` |
| Основной | timeout > 0 — клиент с заданным таймаутом | `TestNewHTTPClient_Timeout` |
| Граничный | timeout ≤ 0 — таймаут 10s | `TestNewHTTPClient_ZeroTimeout` |

---

## internal/server

### handleHealth

**Назначение:** ответить 200 OK с телом "ok" на GET /health.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | GET /health → 200, тело "ok" | `TestHandleHealth_OK` |

### handleWebhook

**Назначение:** проверить event, подпись (если секрет задан), распарсить JSON и поставить событие в очередь.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | X-Gitea-Event: pull_request, валидный JSON, без секрета → 202 | `TestHandleWebhook_AcceptedNoSecret` |
| Основной | С секретом и верной подписью → 202 | `TestHandleWebhook_AcceptedWithValidSignature` |
| Негативный | Секрет задан, подпись неверная → 401 | `TestHandleWebhook_InvalidSignature` |
| Негативный | Секрет задан, подпись отсутствует → 401 | `TestHandleWebhook_MissingSignature` |
| Негативный | Event не pull_request → 400 | `TestHandleWebhook_UnsupportedEvent` |
| Негативный | Невалидный JSON → 400 | `TestHandleWebhook_InvalidJSON` |
| Негативный | Enqueue возвращает ошибку → 503 | `TestHandleWebhook_QueueFull` |
| Граничный | Пустое тело при event pull_request → 400 (или 202 с пустым payload — по реализации) | `TestHandleWebhook_EmptyBody` |

### verifySignature, computeSignature, normalizeSignature

**Назначение:** проверка и вычисление HMAC-SHA256 подписи; нормализация заголовка (sha256=, пробелы).

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | computeSignature: одинаковые payload и secret → один и тот же hex | `TestComputeSignature_Deterministic` |
| Основной | verifySignature: верная подпись (hex) → nil | `TestVerifySignature_Valid` |
| Основной | verifySignature: подпись с префиксом sha256= → nil | `TestVerifySignature_WithPrefix` |
| Негативный | verifySignature: неверная подпись → ошибка | `TestVerifySignature_Mismatch` |
| Негативный | verifySignature: пустая подпись → ошибка | `TestVerifySignature_Empty` |
| Граничный | normalizeSignature: с пробелами, с sha256= | через verifySignature или отдельный тест |

---

## internal/processor

### New, Start, Stop

**Назначение:** создать процессор с очередью заданного размера; запустить/остановить воркеры.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | New — очередь размера cfg.Server.QueueSize | `TestNew_QueueSize` |
| Основной | Start затем Stop — воркеры завершаются | уже в TestProcessor_* |
| Граничный | Start дважды — второй раз не запускает повторно | `TestProcessor_StartIdempotent` |
| Граничный | Stop без Start — не блокирует | `TestProcessor_StopWithoutStart` |

### Enqueue

**Назначение:** добавить событие в очередь; при переполнении или не запущенном процессоре — ошибка.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Негативный | Процессор не запущен → ошибка | `TestProcessor_EnqueueNotStarted` |
| Негативный | Очередь полная → ошибка | `TestProcessor_EnqueueQueueFull` |

### processEvent

**Назначение:** по правилу репо обработать только opened/reopened; шаблон job_pattern → regex; WaitForJob; шаблон комментария; PostComment.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Репо настроен, action opened, джоба найдена → success comment | `TestProcessor_PostsSuccessComment` |
| Основной | Репо настроен, джоба не найдена (timeout) → failure comment | `TestProcessor_PostsFailureCommentWhenNoJobFound` |
| Граничный | Репозиторий без правила → Gitea не вызывается | `TestProcessor_ProcessEvent_RepoNotConfigured` |
| Граничный | action synchronized/closed → Gitea не вызывается | `TestProcessor_ProcessEvent_IgnoredAction` |
| Граничный | repository.full_name пустой → выход без паники | `TestProcessor_ProcessEvent_EmptyRepoName` |
| Граничный | Невалидный job_pattern (некомпилируемый regex после шаблона) → выход без паники, без комментария | `TestProcessor_ProcessEvent_InvalidJobPattern` |
| Граничный | Ошибка шаблона комментария → без повторного поста | через шаблон с синтаксической ошибкой |

### executeTemplate

**Назначение:** выполнить Go-шаблон с данными (покрывается через processEvent с кастомными шаблонами).

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Валидный шаблон и данные → строка | через процессор (success/failure templates) |
| Негативный | Невалидный синтаксис шаблона → ошибка | через процессор с битым шаблоном |

---

## internal/jenkins

### GetJobs

**Назначение:** получить список джоб по job_root (tree=jobs[name,url,fullName]).

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | jobRoot пустой — запрос к /api/json, список джоб | уже частично в WaitForJob |
| Основной | jobRoot задан — путь /job/.../api/json, список джоб | `TestWaitForJobWithJobRoot` |
| Основной | Пустой список jobs — 200, jobs: [] | через httptest |
| Негативный | 401/404/500 → ошибка | `TestGetJobs_Unauthorized`, `TestGetJobs_NotFound`, `TestGetJobs_ServerError` |
| Негативный | Невалидный JSON в ответе → ошибка | `TestGetJobs_InvalidJSON` |

### CheckAccessibility

**Назначение:** GET /api/json — доступность и аутентификация.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | 200 → nil | `TestCheckAccessibility_OK` |
| Негативный | 401/403/404/5xx → ошибка | `TestCheckAccessibility_Unauthorized` и др. |

### CheckJobRootExists

**Назначение:** проверить существование пути job_root (GET .../api/json).

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | jobRoot пустой → nil | `TestCheckJobRootExists_Empty` |
| Основной | Путь существует, 200 → nil | `TestCheckJobRootExists_OK` |
| Негативный | 404/403/5xx → ошибка | `TestCheckJobRootExists_NotFound` и др. |

### findJob (косвенно через WaitForJob)

**Назначение:** среди джоб найти первую по совпадению Name или FullName с regex.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Совпадение по Name | уже в TestWaitForJob |
| Основной | Совпадение по FullName | `TestWaitForJob_MatchFullName` |
| Основной | Несколько джоб, матчится первая | при наличии нескольких в ответе |

### WaitForJob

**Назначение:** опрос GetJobs по интервалу до таймаута; вернуть найденную джобу или ошибку.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Джоба появляется в течение таймаута → job | `TestWaitForJob` |
| Основной | Таймаут — джоба не найдена → ошибка | `TestWaitForJobTimeout` |
| Основной | job_root непустой — правильный путь | `TestWaitForJobWithJobRoot` |

---

## internal/gitea

### NewClient

**Назначение:** создать клиент; baseURL без завершающего слэша.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Граничный | baseURL с завершающим слэшем → обрезка (проверка через путь в PostComment) | `TestNewClient_TrimTrailingSlash` или через PostComment |

### splitRepoFullName

**Назначение:** разбить "owner/repo" на owner и repo.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | "owner/repo" → owner, repo | `TestSplitRepoFullName_Valid` |
| Основной | "a/b" → a, b | табличный тест |
| Негативный | "single" или "" → ошибка | `TestSplitRepoFullName_Invalid` |

(Функция не экспортирована — тестировать через PostComment с разным repoFullName или вынести в отдельный файл и тестировать в том же пакете.)

### PostComment

**Назначение:** POST .../issues/{index}/comments с телом {"body": "..."}.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | 200/201 → nil | `TestPostComment_OK` |
| Негативный | 400/401/404/500 → ошибка | `TestPostComment_BadRequest` и др. |
| Негативный | Неверный repoFullName → ошибка от split | `TestPostComment_InvalidRepoName` |

### CheckAccessibility

**Назначение:** GET /user с токеном.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | 200 → nil | `TestCheckAccessibility_OK` |
| Негативный | 401/403/404/5xx → ошибка | `TestCheckAccessibility_Unauthorized` и др. |

### GetRepository

**Назначение:** GET /repos/{owner}/{repo}.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | 200 → nil | `TestGetRepository_OK` |
| Негативный | 404/403/401/5xx → ошибка | `TestGetRepository_NotFound` и др. |

---

## pkg/webhook

### PullRequest.DisplayName

**Назначение:** вернуть Title, если не пустой; иначе "PR".

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Title не пустой → Title | `TestDisplayName_WithTitle` |
| Граничный | Title пустой → "PR" | `TestDisplayName_EmptyTitle` |

---

## Сводка по файлам тестов

| Пакет | Файл тестов |
|-------|-------------|
| internal/config | config_test.go |
| internal/server | server_test.go |
| internal/processor | processor_test.go |
| internal/jenkins | client_test.go |
| internal/gitea | client_test.go |
| pkg/webhook | types_test.go |

При добавлении новой функции: дополнить этот тест-план и добавить соответствующие тесты в указанные файлы.
