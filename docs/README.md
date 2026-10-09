# Документация проекта Gitea–Jenkins Webhook Bridge

## Обзор

Микросервис на Go, принимающий вебхуки от Gitea о Pull Request. Для правил из `repositories[].checks` публикует статус коммита (сейчас — чёрный список файлов с привязкой к репозиторию и целевой ветке). Для настроенных репозиториев при открытии или повторном открытии PR ожидает джобу Jenkins и оставляет комментарий.

Источники истины: [README.md](../README.md), [config.example.yaml](../config.example.yaml), [internal/config/config.go](../internal/config/config.go), [Makefile](../Makefile), [.github/workflows/ci.yml](../.github/workflows/ci.yml).

## Оглавление документации

| Документ | Описание |
|----------|----------|
| [Архитектура](architecture.md) | Компоненты, пакеты, поток данных |
| [Конфигурация](configuration.md) | Формат YAML, секции, значения по умолчанию, валидация |
| [Установка и запуск](setup.md) | Быстрый старт, локально, Docker, настройка Gitea и Jenkins |
| [Команды приложения](commands/README.md) | Команды `run` и `check`: синтаксис, флаги, примеры |
| [Требования](requirements.md) | Версия Go, ОС, зависимости |
| [Правила для разработчиков](developer-rules.md) | Makefile, CI, линт, тесты, релизы |
| [Тестирование](test/README.md) | Тест-планы, примеры запросов для интеграции, запуск тестов |

## Документация для агентов

Единые документы для LLM и агентов (без дублирования с основной документацией):

| Файл | Назначение |
|------|------------|
| [architecture.md](architecture.md) | Компоненты, поток данных, пакеты, карта файлов, краткий контекст для агентов |
| [glossary.md](glossary.md) | Глоссарий терминов (PR, job, webhook, правило репозитория и т.д.) |
| [api-surface.md](api-surface.md) | HTTP-эндпоинты, форматы запросов/ответов, экспортируемые типы |
| [common-tasks.md](common-tasks.md) | Типовые задачи: добавить репозиторий, опцию конфига, эндпоинт |
| [conventions.md](conventions.md) | Именование, ошибки, логирование, стиль кода |
| [commands/README.md](commands/README.md) | CLI: команды `run`, `check` — синтаксис, флаги, коды возврата |
| [test/README.md](test/README.md) | Документация по тестам: тест-планы, примеры запросов к Jenkins/Gitea |
