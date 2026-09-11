# C4 уровень 1 — контекст

**Вопрос читателя: кто пользуется системой, с чем она разговаривает снаружи и где проходит её внешняя граница?**

Дата: 2026-09-11 · system-architect#1
Иллюстрирует: [`../overview.md`](../overview.md) §12 (контекст to-be), ADR-005 дополнение 2 (нативный llama-server), ADR-009 (граница персональных данных).
Проверено по дереву: `docker-compose.yml`, `docker-compose.bot.yml`, `docker-compose.legacy.yml`, `build/versions.env`, `cmd/`, `scripts/llm-server.ps1`.

```mermaid
C4Context
    title multiverse-core — контекст, MVP-1

    Person(player, "Игрок", "соло или группа 2-6; общается только через Telegram, словарь команд FR-002")
    Person(operator, "Оператор", "он же автор блупринтов и исследователь: make up, mvctl, файлы в Git, .env")
    Person(ci, "Тест-харнесс CI", "actor_kind=ci; ходит по тому же HTTP API и по служебным маршрутам")

    System(mc, "multiverse-core", "платформа живых миров: процессы gateway, core, memory, клиент telegram-bot, CLI mvctl")

    System_Ext(tg, "Telegram Bot API", "long polling getUpdates/sendMessage; единственный источник внешних идентификаторов")
    System_Ext(llama, "llama-server, llama.cpp", "нативный процесс на машине оператора, вне compose, 127.0.0.1:1234; OpenAI-совместимый API с грамматикой по JSON-схеме")
    System_Ext(ollama, "Ollama", "необязательный второй рантайм модели, профиль gpu или нативно")
    System_Ext(cloud, "Облачная модель, БУДУЩЕЕ", "тот же адаптер openai_compat с внешним URL; закрыто гейтом MV_LLM_CLOUD_ENABLED, в MVP-1 выключено")

    System_Ext(bus, "Redpanda", "журнал событий и шина, Kafka API; отдельный контейнер, данными владеет платформа")
    System_Ext(objs, "MinIO", "объектное хранилище состояния и снапшотов; собирается из исходников, ADR-021")
    System_Ext(vec, "Qdrant", "векторный индекс памяти, профиль memory")
    System_Ext(graph, "Neo4j", "граф событий и сущностей, профиль memory")

    Rel(player, tg, "команды и нарратив в личном чате")
    Rel(tg, mc, "getUpdates / sendMessage через telegram-bot; allowlist user id до любого вызова")
    Rel(operator, mc, "make up, mvctl, блупринты и законы в Git, .env")
    Rel(ci, mc, "HTTP v1 + /v1/admin/*; записанные выводы модели вместо живой")

    Rel(mc, llama, "POST /v1/chat/completions с response_format json_schema; thinking выключен на запрос")
    Rel(mc, ollama, "POST /api/chat, /api/embed; только при MV_LLM_PROVIDER=ollama")
    Rel(mc, cloud, "HTTPS; наружу уходит только player_id и текст, никогда внешний идентификатор")

    Rel(mc, bus, "публикация и чтение восьми топиков")
    Rel(mc, objs, "сущности, снапшоты, промпты по флагу")
    Rel(mc, vec, "upsert и поиск эмбеддингов")
    Rel(mc, graph, "MERGE узлов и связей")
```

## Что здесь будущее

- **Игрок и Telegram.** Бинарника `cmd/telegram-bot` в дереве нет; `docker-compose.bot.yml` уже описывает сервис с `entrypoint: ["/telegram-bot"]` и ждёт, когда файл появится (EPIC-004). Сегодня по этой стрелке не проходит ничего.
- **Облачная модель.** Провайдера `anthropic` нет, `openai_compat` с внешним адресом закрыт гейтом; строка в реестре есть, кода нет (E-H).
- **Qdrant и Neo4j.** Контейнеры поднимаются профилем `memory` и здоровы, но контекст `memory` — пустая заглушка: клиента ни к одному из них в дереве нет.

## Расхождения с деревом

1. `overview.md` §12 показывает Ollama как основной рантайм модели рядом с `System_Ext(ollama, "Ollama (Qwen3)")`. По U-8 и ADR-005 дополнение 2 основной рантайм — **нативный llama-server вне compose**, а Ollama второй; `docker-compose.yml` это уже отражает (сервиса `llama-server` нет вовсе, есть `extra_hosts: host.docker.internal`), а §12 — ещё нет.
2. `overview.md` §12 называет актором «Тест-харнесс CI» с доступом к «HTTP v1 + служебные эндпоинты». Служебные маршруты `/v1/admin/*` в дереве отсутствуют: `shared/runtime` умеет монтировать `Routes(mux)`, но ни один контекст их не монтирует, потому что контекстов нет.
3. Внешних систем, кроме перечисленных, у платформы нет — в частности, нет GitHub Actions как внешней системы контекста: workflow'ы это часть репозитория, а не собеседник рантайма. В `overview.md` §2 (as-is) GitHub Actions нарисован как внешняя система; в целевом контексте его быть не должно.
