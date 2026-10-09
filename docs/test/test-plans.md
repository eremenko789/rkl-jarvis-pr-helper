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

### checks

**Назначение:** провалидировать правила проверок, скомпилировать `target_branches` и подставить описания статуса.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Пример конфига загружается, проверка `file_blacklist` матчит `main` | `TestLoad_ExampleConfig` |
| Основной | YAML с checks — дефолтные context и описания, ветки совпадают | `TestLoad_Checks` |
| Негативный | Нет name / дубликат name / неизвестный type / нет target_branches / битый regex / дубликат context | `TestValidate_ChecksMissingName`, `TestValidate_ChecksDuplicateName`, `TestValidate_ChecksUnknownType`, `TestValidate_ChecksMissingTargetBranches`, `TestValidate_ChecksInvalidTargetBranch`, `TestValidate_ChecksDuplicateContext` |
| Негативный | Нет блока `file_blacklist`, пустые patterns, абсолютный путь, чужой блок настроек | `TestValidate_ChecksFileBlacklistMissing`, `TestValidate_ChecksFileBlacklistEmptyPatterns`, `TestValidate_ChecksFileBlacklistInvalidPattern`, `TestValidate_ChecksForeignSpec` |
| Граничный | `MatchesTargetBranch` до `Validate` — false | `TestCheckRule_MatchesTargetBranchWithoutValidate` |
| Граничный | У репозитория нет поля `checks` | `TestValidate_RepositoryWithoutChecks` |
| Граничный | Одинаковые name и context в разных репозиториях допустимы | `TestValidate_ChecksSameNameDifferentRepos` |

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
| Негативный | Ошибка чтения тела запроса → 400 | `TestHandleWebhook_ReadBodyError` |
| Негативный | Enqueue возвращает ошибку → 503 | `TestHandleWebhook_EnqueueFails` |
| Граничный | Пустое тело при event pull_request → 400 (или 202 с пустым payload — по реализации) | `TestHandleWebhook_EmptyBody` |

### Run

**Назначение:** запустить процессор и HTTP-сервер; при ошибке ListenAndServe вернуть ошибку.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Негативный | Порт занят (ListenAndServe ошибка) → Run возвращает ошибку | `TestRun_ListenAndServeError` |

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

## internal/checks

### MatchFile

**Назначение:** сопоставить путь файла с glob-шаблоном (`*`, `?`, `**`).

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Точный путь, `*`, `**`, `?` | `TestMatchFile` |
| Граничный | `*` не переходит через `/`; регистр учитывается | `TestMatchFile` |

### Applicable и Evaluate

**Назначение:** отобрать правила по целевой ветке и оценить `file_blacklist`.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Ветка совпадает только с подходящими правилами | `TestApplicable` |
| Основной | Нет совпадений — success, в том числе при пустом diff | `TestEvaluateFileBlacklist_Success`, `TestEvaluateFileBlacklist_EmptyDiff` |
| Основной | Совпавшие пути в описании failure | `TestEvaluateFileBlacklist_Failure` |
| Основной | Учитывается `previous_filename` | `TestEvaluateFileBlacklist_PreviousFilename` |
| Граничный | Длинное описание обрезается до 255 байт без разрыва UTF-8 | `TestEvaluateFileBlacklist_TruncatesDescription`, `TestEvaluateFileBlacklist_TruncatesOnRuneBoundary` |
| Негативный | Неизвестный тип и пустые настройки | `TestEvaluate_UnknownType`, `TestEvaluate_MissingSpec` |

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

**Назначение:** по правилу репо обработать только opened; шаблон job_pattern → regex; WaitForJob; шаблон комментария; PostComment.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Репо настроен, action opened, джоба найдена → success comment | `TestProcessor_PostsSuccessComment` |
| Основной | Репо настроен, джоба не найдена (timeout) → failure comment | `TestProcessor_PostsFailureCommentWhenNoJobFound` |
| Граничный | Репозиторий без правила → Gitea не вызывается | `TestProcessor_ProcessEvent_RepoNotConfigured` |
| Граничный | action synchronized/reopened/closed → комментарий Jenkins не публикуется | `TestProcessor_ProcessEvent_IgnoredAction` |
| Граничный | repository.full_name пустой → выход без паники | `TestProcessor_ProcessEvent_EmptyRepoName` |
| Граничный | Невалидный job_pattern (некомпилируемый regex после шаблона) → выход без паники, без комментария | `TestProcessor_ProcessEvent_InvalidJobPattern` |
| Граничный | Ошибка шаблона комментария (невалидный синтаксис) → без комментария | `TestProcessor_InvalidCommentTemplate` |
| Граничный | PostComment возвращает ошибку → логирование, без паники | `TestProcessor_PostCommentFails` |
| Граничный | WaitForJob возвращает (job, err) — логирование "error waiting for jenkins job", затем success comment | `TestProcessor_WaitForJobReturnsError` |

### runChecks

**Назначение:** для `opened`/`synchronized`/`reopened` выбрать правила по `base.ref`, получить файлы PR и опубликовать статус коммита. Поиск Jenkins выполняется только для `opened`.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Нет файлов из чёрного списка → status success | `TestProcessor_FileBlacklistSuccess` |
| Основной | Изменён файл из списка → status failure с путём | `TestProcessor_FileBlacklistFailure` |
| Основной | Переименование с `previous_filename` из списка → failure | `TestProcessor_FileBlacklistPreviousFilename` |
| Основной | Два правила на одну ветку публикуют два статуса | `TestProcessor_FileBlacklistTwoMatchingRules` |
| Основной | Проверка и комментарий Jenkins на одном событии | `TestProcessor_FileBlacklistWithJenkinsComment` |
| Граничный | Ветка не совпала — файлы не запрашиваются, Jenkins-комментарий остаётся | `TestProcessor_FileBlacklistSkipsUnmatchedBranch` |
| Граничный | `synchronized` выполняет проверку и не вызывает Jenkins | `TestProcessor_FileBlacklistSynchronizedWithoutJenkins` |
| Граничный | `closed` не запускает проверку | `TestProcessor_FileBlacklistIgnoresClosed` |
| Негативный | Ошибка списка файлов → status error | `TestProcessor_FileBlacklistListError` |
| Граничный | Пустой head SHA — статус не публикуется | `TestProcessor_FileBlacklistMissingSHA` |
| Негативный | Ошибка публикации статуса — без паники | `TestProcessor_FileBlacklistStatusPostError` |
| Граничный | Проверки другого репозитория не выполняются | `TestProcessor_FileBlacklistSkipsOtherRepository` |

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
| Негативный | 401/400 → ошибка | `TestGetJobs_Unauthorized`, `TestGetJobs_StatusBadRequest` |
| Негативный | Ошибка Do (сеть) → ошибка | `TestGetJobs_DoFails` |
| Негативный | Невалидный JSON в ответе → ошибка | `TestGetJobs_InvalidJSON` |

### NewClient (jenkins)

**Назначение:** создать клиент; при nil httpClient/logger — дефолты.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Граничный | httpClient == nil → клиент с таймаутом 10s | `TestNewClient_NilHTTPClient` |

### CheckAccessibility

**Назначение:** GET /api/json — доступность и аутентификация.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | 200 → nil | `TestCheckAccessibility_OK` |
| Негативный | 401/403/404/5xx → ошибка | `TestCheckAccessibility_Unauthorized`, `TestCheckAccessibility_Forbidden`, `TestCheckAccessibility_NotFound`, `TestCheckAccessibility_ServerError` |

### CheckJobRootExists

**Назначение:** проверить существование пути job_root (GET .../api/json).

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | jobRoot пустой → nil | `TestCheckJobRootExists_Empty` |
| Основной | Путь существует, 200 → nil | `TestCheckJobRootExists_OK` |
| Негативный | 404/403/5xx → ошибка | `TestCheckJobRootExists_NotFound`, `TestCheckJobRootExists_Forbidden`, `TestCheckJobRootExists_ServerError` |

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
| Граничный | Джобы есть, но ни одна не совпадает с паттерном → таймаут | `TestWaitForJob_NoMatch` |
| Основной | job_root непустой — правильный путь | `TestWaitForJobWithJobRoot` |

---

## internal/gitea

### NewClient

**Назначение:** создать клиент; baseURL без завершающего слэша; при nil httpClient/logger — дефолты.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Граничный | baseURL с завершающим слэшем → обрезка (проверка через путь в PostComment) | `TestNewClient_TrimTrailingSlash` |
| Граничный | logger == nil → slog.Default() | `TestNewClient_NilLogger` |
| Граничный | httpClient == nil → клиент с таймаутом 10s | `TestNewClient_NilHTTPClient` |

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
| Негативный | 500 → ошибка | `TestPostComment_ServerError` |
| Негативный | Ошибка Do (сеть) → ошибка | `TestPostComment_DoFails` |
| Негативный | Неверный repoFullName → ошибка от split | `TestPostComment_InvalidRepoName` |

### ListPullRequestFiles

**Назначение:** GET `/repos/{owner}/{repo}/pulls/{index}/files` постранично по заголовкам `X-HasMore`, `X-Page`, `X-PageCount`.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Две страницы склеиваются, читается `previous_filename` | `TestListPullRequestFiles_Pagination` |
| Граничный | Полная страница и `X-HasMore: false` — следующий запрос не делается | `TestListPullRequestFiles_FullPageWithoutMore` |
| Граничный | Короткая страница и `X-HasMore: true` — запрашивается следующая | `TestListPullRequestFiles_ShortPageWithMore` |
| Граничный | Пустой список, `X-PageCount: 0` | `TestListPullRequestFiles_Empty` |
| Негативный | Нет заголовков пагинации / `X-Page` не совпал с запросом | `TestListPullRequestFiles_MissingPaginationHeaders`, `TestListPullRequestFiles_PageMismatch` |
| Негативный | 404 / невалидный JSON / сеть / неверное имя репозитория / больше 200 страниц | `TestListPullRequestFiles_ServerError`, `TestListPullRequestFiles_InvalidJSON`, `TestListPullRequestFiles_DoFails`, `TestListPullRequestFiles_InvalidRepoName`, `TestListPullRequestFiles_TooManyPages` |

### CreateCommitStatus

**Назначение:** POST `/repos/{owner}/{repo}/statuses/{sha}`.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | 201, путь и тело со state/context | `TestCreateCommitStatus_OK` |
| Негативный | Пустой sha, пустой state, неизвестный state, пустой context, неверное имя репозитория | `TestCreateCommitStatus_Validation` |
| Негативный | 500 и ошибка сети | `TestCreateCommitStatus_ServerError`, `TestCreateCommitStatus_DoFails` |

### CheckAccessibility

**Назначение:** GET /user с токеном.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | 200 → nil | `TestCheckAccessibility_OK` |
| Негативный | 401/403/404/5xx → ошибка | `TestCheckAccessibility_Unauthorized`, `TestCheckAccessibility_Forbidden`, `TestCheckAccessibility_NotFound`, `TestCheckAccessibility_ServerError` |

### GetRepository

**Назначение:** GET /repos/{owner}/{repo}.

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | 200 → nil | `TestGetRepository_OK` |
| Негативный | 404/403/401/5xx → ошибка | `TestGetRepository_NotFound`, `TestGetRepository_Forbidden`, `TestGetRepository_Unauthorized`, `TestGetRepository_ServerError` |

---

## pkg/webhook

### PullRequest.DisplayName

**Назначение:** вернуть Title, если не пустой; иначе "PR".

| Тип | Тест-кейс | Тест |
|-----|-----------|------|
| Основной | Title не пустой → Title | `TestDisplayName_WithTitle` |
| Граничный | Title пустой → "PR" | `TestDisplayName_EmptyTitle` |
| Основной | JSON `base`/`head` разбирается в `ref` и `sha` | `TestPullRequest_UnmarshalBaseHead` |

---

## Сводка по файлам тестов

| Пакет | Файл тестов |
|-------|-------------|
| internal/config | config_test.go |
| internal/checks | checks_test.go, glob_test.go |
| internal/server | server_test.go |
| internal/processor | processor_test.go, checks_test.go |
| internal/jenkins | client_test.go |
| internal/gitea | client_test.go |
| pkg/webhook | types_test.go |

При добавлении новой функции: дополнить этот тест-план и добавить соответствующие тесты в указанные файлы.
