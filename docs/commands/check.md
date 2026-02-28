# Команда `check`

Выполняет проверку конфигурации и доступности компонентов системы (файл конфигурации, сервер, Jenkins, Gitea, репозитории и джобы) перед запуском сервиса.

## Синтаксис

```bash
webhook-service check -config <путь_к_конфигу> [-debug]
```

## Флаги

| Флаг | Обязательный | Описание |
|------|--------------|----------|
| `-config` | Да | Путь к файлу конфигурации (YAML). |
| `-debug` | Нет | Включить детальное логирование. |

## Коды возврата

- `0` — все проверки пройдены успешно или есть только предупреждения.
- `1` — не указан `-config`, обнаружены критические ошибки или ошибки в репозиториях/джобах.

## Последовательность проверок

### 1. Файл конфигурации

- Проверяется существование файла по указанному пути.
- Если файл не найден: сообщение `ERROR: Configuration file not found: <путь>`, выход с кодом 1.

### 2. Загрузка и валидация конфигурации

- Загрузка через `config.Load()` и валидация.
- При ошибке: вывод в stderr, выход с кодом 1.
- При успехе: `✓ Configuration file loaded and validated`.

### 3. Настройки сервера

- Проверяются секция `server`: `listen_addr`, `webhook_secret`, `worker_pool_size`, `queue_size` (не пустые/положительные).
- При ошибке: вывод в stderr, выход с кодом 1.
- При успехе: `✓ Server configuration is valid`.

### 4. Доступность Jenkins

- Запрос к Jenkins API (Basic Auth). При ошибке — вывод в stderr и выход с кодом 1.
- При успехе: `✓ Jenkins is accessible at <base_url>`.

### 5. Доступность Gitea

- Запрос к Gitea API (токен). При ошибке — вывод в stderr и выход с кодом 1.
- При успехе: `✓ Gitea is accessible at <base_url>`.

### 6. Доступ к репозиторию в Gitea (опционально)

- Проверка доступа к первому репозиторию из конфигурации. Если репозиториев нет — предупреждение и пропуск.
- При успехе: `✓ Gitea repository access verified`.
- При ошибке/предупреждении: `⚠ Warning: ...` (не критично, выполнение продолжается).

### 7. Проверка репозиториев

Для каждого репозитория из `repositories`:

- **7.1. Наличие в Gitea** — запрос `GET /repos/{owner}/{repo}`. При успехе: `✓ Repository {name} exists in Gitea`. При ошибке — вывод ошибки, учёт в сводке, переход к следующему репозиторию.
- **7.2. Job root в Jenkins** (если указан `job_root`) — проверка существования пути. При успехе: `✓ Job root "{job_root}" exists in Jenkins`. При ошибке — вывод ошибки и переход к следующему репозиторию.
- **7.3. Наличие джоб в root** — получение списка джоб. Если джоб нет: предупреждение `⚠ No jobs found in root "…"`. Иначе: `✓ Found N job(s) in root "…"`.
- **7.4. Соответствие паттерну** — в `job_pattern` плейсхолдер `{{ .Number }}` заменяется на `\d+`, проверяется соответствие имён/полных имён джоб. При успехе: `✓ Job pattern matches at least one job`. При отсутствии совпадений: `✗ No jobs match pattern "…"`. Если джоб не было — предупреждение о невозможности проверить паттерн.

## Формат вывода

- `✓` — успешная проверка.
- `✗` — ошибка.
- `⚠` — предупреждение.

В конце выводится сводка:
```text
Summary: <passed> checks passed, <errors> errors, <warnings> warnings
```

Критические ошибки (файл конфигурации, загрузка/валидация, сервер, недоступность Jenkins или Gitea) прерывают выполнение. Ошибки по репозиториям/джобам не прерывают проверку остальных репозиториев, но приводят к коду выхода 1 при наличии хотя бы одной ошибки.

## Примеры

Успешная проверка:
```bash
$ webhook-service check -config config.yaml
Checking configuration...

✓ Configuration file loaded and validated
✓ Server configuration is valid
✓ Jenkins is accessible at https://jenkins.example.com
✓ Gitea is accessible at https://gitea.example.com/api/v1
✓ Gitea repository access verified

Checking repositories:
  Repository: org/repo-one
  ✓ Repository org/repo-one exists in Gitea
  ✓ Job root "org_name/repo_name" exists in Jenkins
  ✓ Found 5 job(s) in root "org_name/repo_name"
  ✓ Job pattern matches at least one job

Summary: 7 checks passed, 0 errors, 0 warnings
```

Файл конфигурации не найден:
```bash
$ webhook-service check -config missing.yaml
ERROR: Configuration file not found: missing.yaml
```

Ошибка доступности Jenkins:
```bash
$ webhook-service check -config config.yaml
Checking configuration...

✓ Configuration file loaded and validated
✓ Server configuration is valid
✗ Jenkins is not accessible at https://jenkins.example.com: connection timeout

Summary: 2 checks passed, 1 errors, 0 warnings
```

Справка по команде:
```bash
webhook-service check -h
```
