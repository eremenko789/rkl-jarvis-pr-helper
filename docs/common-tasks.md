# Типовые задачи

Пошаговые сценарии для частых изменений в проекте (в т.ч. при работе агента).

## Добавить правило репозитория (новый репозиторий)

1. Открыть [config.example.yaml](../config.example.yaml) и при необходимости [internal/config/config.go](../internal/config/config.go) (структура `RepositoryRule`).
2. В секцию `repositories` добавить новый элемент с обязательными полями:
   - `name` — полное имя репозитория `owner/repo`.
   - `job_pattern` — строка regex с опциональным Go-шаблоном (например `{{ .Number }}`).
3. При необходимости задать: `job_root`, `poll_interval`, `timeout`, `success_comment_template`, `failure_comment_template`. Если не заданы, применяются значения по умолчанию из `Validate()` (глобальные из `jenkins` и шаблоны по умолчанию).
4. Индекс репозиториев строится автоматически при `Load()` через `buildIndex()`; отдельно менять код не нужно. Документацию по полям обновить в [docs/configuration.md](configuration.md).

## Добавить опцию конфига

1. **Структура**: в [internal/config/config.go](../internal/config/config.go) добавить поле в нужную структуру (`ServerConfig`, `JenkinsConfig`, `GiteaConfig`, `RepositoryRule`) с тегом `yaml:"snake_case_name"`.
2. **Валидация и умолчания**: в методе `Config.Validate()` обработать новое поле — установить значение по умолчанию при отсутствии или недопустимом значении, при необходимости вернуть ошибку.
3. **Пример**: добавить ключ и комментарий в [config.example.yaml](../config.example.yaml).
4. **Использование**: в коде использовать новое поле из `cfg.Server`, `cfg.Jenkins`, `cfg.Gitea` или из `rule` в процессоре/check. При использовании в шаблонах комментариев или паттерне — описать в [docs/configuration.md](configuration.md) и при необходимости расширить `data` в [internal/processor/processor.go](../internal/processor/processor.go) (для шаблонов).
5. **Тесты**: при необходимости добавить кейс в [internal/config/config_test.go](../internal/config/config_test.go).

## Добавить эндпоинт (обработчик)

1. В [internal/server/server.go](../internal/server/server.go) в функции `New()` зарегистрировать маршрут: `mux.HandleFunc("METHOD /path", s.handleXxx)`.
2. Реализовать метод `handleXxx(w http.ResponseWriter, r *http.Request)`: чтение запроса, при необходимости — конфиг из `s.cfg`, вызов логики (при необходимости — процессор или другие пакеты), запись ответа и статуса.
3. При необходимости добавить настройки в конфиг (например путь или флаг включения) — см. «Добавить опцию конфига».
4. Обновить [docs/api-surface.md](api-surface.md) и при необходимости [docs/README.md](README.md) или [setup.md](setup.md).

## Добавить правило проверки file_blacklist

1. В правиле нужного репозитория в [config.example.yaml](../config.example.yaml) (и в рабочем конфиге) добавить элемент в `checks`. Если поля `checks` нет, его можно добавить: без него проверки для репозитория не выполняются.
2. Заполнить общие поля: `name`, `type: file_blacklist`, `target_branches` (регулярные выражения целевой ветки). При необходимости — `context`, `success_description`, `failure_description`.
3. В `file_blacklist.patterns` перечислить glob-шаблоны файлов. Точный путь, `*`/`?` внутри одного сегмента и `**` через каталоги описаны в [configuration.md](configuration.md).
4. Отдельно менять код не нужно: `Validate()` компилирует выражения веток, процессор публикует статус при создании (`opened`), обновлении (`synchronized`) и повторном открытии (`reopened`). Поиск джобы Jenkins при этом не запускается, кроме действия `opened`.

## Добавить тип проверки

Структура `repositories[].checks` общая для всех типов. Новый тип не меняет привязку к репозиторию и ветке и публикацию статуса.

1. В [internal/config/config.go](../internal/config/config.go) добавить константу `CheckType*` и структуру настроек.
2. Добавить в `CheckRule` указатель на эту структуру с тегом `yaml:"<type>"`.
3. В `validateChecks` принять новый `type`: проверить, что заполнен свой объект и не заполнены чужие (`rejectForeignSpecs`), проставить описания статуса по умолчанию.
4. В [internal/checks/checks.go](../internal/checks/checks.go) добавить ветку `Evaluate`. Результат — `Outcome` со `State` `success` или `failure` и описанием. Процессор сам публикует статус коммита.
5. Добавить пример в [config.example.yaml](../config.example.yaml) и описать ключи в [configuration.md](configuration.md).

## Добавить поле в шаблон комментария или job_pattern

1. В [internal/processor/processor.go](../internal/processor/processor.go) в `processEvent()` расширить карту `data`, передаваемую в `executeTemplate()`: добавить новые ключи (например из `evt`, `rule` или из результата Jenkins/Gitea).
2. Обновить [docs/configuration.md](configuration.md) и [docs/glossary.md](glossary.md): перечислить новые поля в описании шаблонов и/или `job_pattern`.

## Изменить поведение команды check

1. Логика проверки: [cmd/webhook-service/check.go](../cmd/webhook-service/check.go) — добавить или изменить этап проверки (например новый вызов клиента Jenkins/Gitea).
2. Клиенты: при необходимости добавить метод в [internal/jenkins/client.go](../internal/jenkins/client.go) или [internal/gitea/client.go](../internal/gitea/client.go) и вызывать его из `checkCommand` или вспомогательных функций в check.go.
3. Формат вывода и коды выхода — по ТЗ в [doc/TZ_CHECK_COMMAND.md](../doc/TZ_CHECK_COMMAND.md). Документацию в [docs/setup.md](setup.md) обновить при изменении формата вывода или флагов.
