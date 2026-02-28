# Section Templates for docs/

Use these as suggested heading structures for each file in `docs/`. Only headings; fill content from the project. **Все заголовки и содержимое в сгенерированных документах — на русском языке.**

## docs/README.md (или index.md)

- Обзор
- Оглавление документации (ссылки на все документы)
- Документация для агентов (ссылки на context-for-agents, glossary, file-map, api-surface, common-tasks, conventions)

## docs/architecture.md

- Компоненты
- Поток данных
- Пакеты / модули (cmd, internal, pkg)
- Диаграммы (mermaid при необходимости)

## docs/configuration.md

- Формат и расположение конфига
- Секции и ключи
- Значения по умолчанию и валидация
- Ссылка на пример конфига

## docs/setup.md

- Быстрый старт
- Локальный запуск
- Docker / docker-compose
- Внешние системы (Gitea, Jenkins и т.д.)

## docs/requirements.md

- Язык и версия
- ОС / окружение
- Зависимости и переменные окружения (при необходимости)

## docs/developer-rules.md

- Команды Makefile (или сборки)
- CI pipeline
- Линт и форматирование
- Тесты и покрытие
- Релизы и артефакты

## docs/context-for-agents.md

- Стек и точки входа
- Ключевые пути
- Основные концепции и сущности

## docs/glossary.md

- Термины (заголовок на термин или группа): PR, job, webhook, правило репозитория, шаблон комментария, poll_interval и т.д.

## docs/file-map.md

- Корень и каталоги верхнего уровня
- cmd / точка входа
- internal (по пакетам или каталогам)
- pkg (если есть)
- Конфиги и скрипты

## docs/api-surface.md

- HTTP-эндпоинты
- Форматы запросов и ответов
- Экспортируемые пакеты и типы
- Точки расширения и контракт vs внутренняя реализация

## docs/common-tasks.md

- Добавить репозиторий (или аналог)
- Добавить опцию конфига
- Добавить эндпоинт (или обработчик)
- (Другие типовые задачи проекта)

## docs/conventions.md

- Именование (файлы, пакеты, символы)
- Обработка ошибок
- Логирование
- Стиль кода и форматирование
