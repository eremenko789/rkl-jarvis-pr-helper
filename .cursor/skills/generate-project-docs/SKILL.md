---
name: generate-project-docs
description: Analyzes the project codebase and generates or updates structured markdown documentation in the docs/ directory, in Russian. Creates separate files for architecture, configuration, setup, requirements, developer rules, and agent-oriented docs (context, glossary, file map, API surface, common tasks, conventions). Use when the user asks to create or update project documentation, to document architecture or setup, or to add docs/ with standard blocks for humans and LLM agents.
---

# Generate Project Documentation in docs/

## Purpose

When applied, the agent studies the current project and produces or updates a `docs/` directory at the project root. Each logical block lives in its own markdown file. Documentation targets both humans and LLM/agents (context, glossary, file map, API surface, common tasks, conventions).

## How to Study the Project

Before writing any doc, gather context:

1. **Root and layout** — List root and main directories; identify project type (Go, Node, Ansible, etc.).
2. **Entry points** — Locate `main`, `cmd/`, or equivalent; how config is loaded (YAML, env, flags).
3. **Config source** — Config struct and validation (e.g. `internal/config/config.go`); example config file.
4. **Build and run** — Makefile, Dockerfile, docker-compose, scripts.
5. **CI** — Workflow file (e.g. `.github/workflows/ci.yml`): test, lint, build, release.
6. **Existing docs** — README, CONTEXT.md, `doc/`, `docs/`; what to reuse or migrate.

Cite sources of truth explicitly (e.g. `config.example.yaml`, `internal/config/config.go`, `README.md`, `Makefile`, `ci.yml`).

## Language

**Вся генерируемая документация в `docs/` должна быть на русском языке.** Заголовки, текст, подписи к диаграммам, названия подразделов — только русский. Термины из кода (имена пакетов, полей конфига, команд) можно оставлять как в коде.

## Structure of docs/

Create or update these files. All content in Russian.

| File | Content |
| ---- | ------- |
| `docs/README.md` or `docs/index.md` | Оглавление: краткое описание проекта и ссылки на все документы. Обязательный подраздел «Документация для агентов» со ссылками на шесть файлов для агентов. |
| `docs/architecture.md` | Компоненты, пакеты/модули, поток данных (вход → обработка → выход). Для Go: cmd, internal, pkg; при необходимости — диаграммы mermaid. |
| `docs/configuration.md` | Формат конфига (YAML/env), все секции и ключи, значения по умолчанию, валидация. Ссылка на пример конфига в репозитории. |
| `docs/setup.md` | Установка, быстрый старт, запуск (локально, Docker, docker-compose). Отдельно — настройка внешних систем (Gitea webhook, Jenkins, токены). |
| `docs/requirements.md` | Версия языка (например Go 1.22), ОС, при необходимости зависимости и переменные окружения. |
| `docs/developer-rules.md` | Команды Makefile (build, test, lint, cover, tidy, ci), CI pipeline, линтеры, форматирование, тесты, релизы (теги, артефакты). |
| `docs/context-for-agents.md` | Структурированный контекст для LLM/агентов: стек, точки входа, ключевые пути, основные концепции; краткий «стартовый контекст» без чтения всего кода. |
| `docs/glossary.md` | Глоссарий: единые термины (PR, job, webhook, правило репозитория, шаблон комментария и т.д.) для согласованной терминологии. |
| `docs/file-map.md` | Карта файлов и каталогов: назначение `cmd/`, `internal/*`, `pkg/`, конфигов; где что искать (навигация для агента). |
| `docs/api-surface.md` | Публичная поверхность: HTTP-эндпоинты, экспортируемые пакеты/типы, точки расширения; контракт и внутренняя реализация. |
| `docs/common-tasks.md` | Типовые задачи: пошагово «добавить правило репозитория», «добавить опцию конфига», «добавить эндпоинт» — сценарии для изменений агентом. |
| `docs/conventions.md` | Конвенции: именование, обработка ошибок, логирование, стиль кода; чтобы сгенерированный код соответствовал проекту. |

## Workflow

1. Study the project using the steps in "How to Study the Project".
2. Create `docs/` if it does not exist.
3. Create or update each file listed above from code and existing README/doc. Do not copy long README blocks verbatim; give structured content and link to README where appropriate.
4. Create and update all six agent-oriented files (`context-for-agents`, `glossary`, `file-map`, `api-surface`, `common-tasks`, `conventions`) alongside the main docs.
5. Ensure cross-links work and repo file references use explicit paths from repo root (e.g. `config.example.yaml`, `internal/config/config.go`).
6. Write all doc content in Russian (see Language section).

## Documentation for LLM and Agents

Agent-oriented files reduce the need to traverse the whole repo and give structured context:

- **context-for-agents.md** — One place for stack, entry points, key paths, main entities (e.g. webhook → queue → worker → Jenkins/Gitea). Agent gets "where is what" quickly.
- **glossary.md** — Fixed terms (PR, job, webhook, repository rule, comment template, poll_interval). Reduces confusion in answers and refactors.
- **file-map.md** — Role of each directory and key files (cmd = CLI, internal/server = HTTP, internal/processor = queue/workers, internal/config = config load). Agent opens the right files.
- **api-surface.md** — Endpoints (e.g. POST /webhook, GET /healthz), request/response shapes, exported types in pkg, config structs. Clarifies what is stable contract and where to extend.
- **common-tasks.md** — Step-by-step: "add config field" (config.go + example + validation), "add repository" (repositories + templates), "add endpoint" (server + handler). Agent follows steps and does not miss touchpoints.
- **conventions.md** — File/package naming, error returns, logging (e.g. slog), formatting (gofmt). Generated code aligns with the project.

## Optional: Section Templates

For consistent section structure across docs, use [reference.md](reference.md) — it lists suggested headings for each doc (no long text, headings only).

## Go Service Example (Reference)

For a Go microservice similar to a Gitea–Jenkins webhook bridge:

- **Architecture:** cmd → server (HTTP) → processor (queue, workers) → jenkins (poll) + gitea (comments); pkg/webhook = event models.
- **Config:** Single YAML; sections `server`, `jenkins`, `gitea`, `repositories`; comment templates with Go templates; defaults in config package Validate().
- **Setup:** Copy `config.example.yaml` → `config.yaml`; run via `go run` or docker-compose; document Gitea webhook (URL, secret) and Jenkins (job name, API).
- **Requirements:** Go version from go.mod / CI (e.g. 1.22); "make tidy" for deps.
- **Developer rules:** make tidy, lint, test, cover, build, ci; CI on PR/push main/tags v*.*.*; build matrix (e.g. linux 386/amd64/arm/arm64); release via GitHub Release.

Adapt these to the actual project when generating docs.

## Verification

Before finishing:

- [ ] Description is third-person and includes when to use the skill.
- [ ] All listed docs are created or updated; agent-oriented files are included.
- [ ] docs/README (or index) has «Документация для агентов» with links to the six agent files.
- [ ] All generated documentation is in Russian.
- [ ] Links between docs and to repo files are valid; paths are from repo root.
