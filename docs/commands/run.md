# Команда `run`

Запускает вебхук-сервис для приёма вебхуков от Gitea и обработки событий Pull Request: проверки из `repositories[].checks` (статус коммита) и ожидание джоб в Jenkins с комментарием в PR.

## Синтаксис

```bash
webhook-service run -config <путь_к_конфигу> [-debug]
```

## Флаги

| Флаг | Обязательный | Описание |
|------|--------------|----------|
| `-config` | Да | Путь к файлу конфигурации (YAML). |
| `-debug` | Нет | Включить детальное логирование. |

**Примечание:** в коде для `run` используется значение по умолчанию `config.yaml` для `-config`, если флаг не указан; для надёжности рекомендуется всегда указывать `-config` явно.

## Поведение

1. Загружается конфигурация из указанного файла.
2. Создаются HTTP-клиенты и клиенты для Jenkins и Gitea.
3. Инициализируются процессор событий и HTTP-сервер.
4. Сервер запускается и обрабатывает сигналы `SIGINT`/`SIGTERM` для корректного завершения.

## Коды возврата

- `0` — сервис завершился штатно (после получения сигнала).
- `1` — ошибка загрузки конфигурации или аварийное завершение сервера.

## Примеры

Запуск с конфигом по умолчанию:
```bash
./bin/webhook-service run -config config.yaml
```

Запуск с отладочным логированием:
```bash
./bin/webhook-service run -config config.yaml -debug
```

Пример вывода при успешном старте:
```
starting webhook service config_path=config.yaml debug=false
configuration loaded successfully server_addr=:8080 worker_pool_size=4 queue_size=100 repositories_count=2
initializing processor and server
webhook service started successfully
```
