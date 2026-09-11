# C4 уровень 2 — контейнеры

**Вопрос читателя: из каких процессов и хранилищ собрана платформа, кто из них с кем говорит и что запускается вне композиции?**

Дата: 2026-09-11 · system-architect#1
Иллюстрирует: [`../overview.md`](../overview.md) §13, ADR-001 (число процессов — параметр деплоя), ADR-021 (MinIO из исходников), C-01/C-06/C-08/C-09.
Проверено по дереву: `docker-compose.yml`, `docker-compose.bot.yml`, `docker-compose.legacy.yml`, `cmd/multiverse/{serve.go,contexts.go}`, `shared/eventbus/topics.go`, `shared/objstore/buckets.go`, `build/redpanda-init.sh`, `build/versions.env`.

```mermaid
C4Container
    title multiverse-core — контейнеры, MVP-1; один образ, разный набор контекстов

    Person(player, "Игрок")
    Person(operator, "Оператор")

    Container(bot, "telegram-bot, БУДУЩЕЕ", "Go, cmd/telegram-bot", "long polling; allowlist Telegram user id; только личные чаты; единственное место, где chat_id живёт в памяти. Бинарника в дереве нет, сервис профиля bot его ждёт")
    Container(gw, "gateway", "Go, cmd/multiverse --contexts=gateway", "HTTP API v1, псевдонимизация, сессии, раунды, идемпотентность, outbox доставок, read-model. Сегодня контекст пустой: процесс поднимается и отвечает /health")
    Container(core, "core", "Go, cmd/multiverse --contexts=state,mechanics,laws,llm,swarm", "состояние, механика, законы, шлюз модели, рой агентов. Из пяти контекстов написан один - mechanics; остальные четыре зарегистрированы пустыми")
    Container(mem, "memory", "Go, cmd/multiverse --contexts=memory", "проекция журнала: векторный индекс и граф, контекст агентов, сводка отсутствия. Контекст пустой")
    Container(cli, "mvctl", "Go, cmd/mvctl", "env check, contracts check, storage init, privacy scan. Команды world init, replay, session-report, laws bump - будущее")

    ContainerQueue(bus, "Redpanda", "Kafka API, 1 брокер, 1 партиция на топик", "восемь топиков: player_events, game_events, world_events, system_events, narrative_output, llm_records, analytics_events, dead_letters; retention 30/90/180 дней, segment.ms=1d")
    ContainerDb(minio, "MinIO", "S3, сборка из исходников тега RELEASE.2025-10-15", "entities-{world}, snapshots-{world}/{state,swarm,gateway}, prompts-{world} по флагу, ops-artifacts; versioning и ILM ставит EnsureBucket")
    ContainerDb(sqlite, "SQLite, БУДУЩЕЕ", "modernc.org/sqlite в томе gateway", "links.db - связки персональных данных; gateway.db - сессии, раунды, ключи идемпотентности, outbox. Файлов и миграций в дереве нет")
    ContainerDb(qdrant, "Qdrant", "gRPC 6334, профиль memory", "эмбеддинги событий и сущностей; клиента в дереве нет")
    ContainerDb(neo4j, "Neo4j 5.26 LTS", "Bolt 7687, профили memory и legacy", "граф Event/Entity/World; клиента в целевом коде нет, читает as-is semantic-memory профиля legacy")

    Container_Ext(llama, "llama-server", "llama.cpp, НАТИВНО ВНЕ COMPOSE, 127.0.0.1:1234", "Qwen3.8-27B UD-Q3_K_XL; /v1/chat/completions с json_schema, /v1/embeddings, /v1/models, /health. Запуск make llm-up; контейнеры ходят через host.docker.internal")
    Container_Ext(ollamac, "Ollama", "контейнер, профиль gpu, 11434", "второй рантайм модели; нужен только при MV_LLM_PROVIDER=ollama или для эмбеддингов")
    Container_Ext(console, "Redpanda Console", "профиль dev, 8092", "просмотр топиков оператором")
    Container_Ext(legacy, "narrative-orchestrator + semantic-memory + chromadb", "профиль legacy, отдельный файл compose", "as-is путь GM для сравнения нарративов до вехи S5; собирается из коммита LEGACY_SRC_REF, не из рабочего дерева")

    Rel(player, bot, "Telegram")
    Rel(bot, gw, "POST /v1/.../actions; GET /v1/clients/{id}/deliveries long-poll; links/*")
    Rel(operator, cli, "командная строка на хосте, MV_KAFKA_BROKERS=127.0.0.1:19092")

    Rel(gw, bus, "публикует player_events, game_events, system_events, analytics_events; читает system_events, game_events, world_events, narrative_output")
    Rel(gw, sqlite, "SQL, два отдельных файла")
    Rel(gw, minio, "читает snapshots-{world}/state/latest.json, пишет snapshots-{world}/gateway/")
    Rel(gw, core, "обратный прокси /v1/admin/* на MV_CORE_URL; только клиенты ci и operator")

    Rel(core, bus, "все доменные топики и llm_records")
    Rel(core, minio, "сущности, снапшоты state и swarm, промпты по флагу")
    Rel(core, llama, "HTTP, провайдер openai_compat по умолчанию")
    Rel(core, ollamac, "HTTP, провайдер ollama")
    Rel(core, mem, "HTTP /v1/context/*, таймаут 500 мс, деградация в окно журнала")

    Rel(mem, bus, "читает все доменные топики как проекцию")
    Rel(mem, qdrant, "upsert и поиск")
    Rel(mem, neo4j, "MERGE")

    Rel(cli, bus, "чтение analytics_events, llm_records, game_events")
    Rel(cli, minio, "артефакты прогонов, ops-artifacts")
    Rel(console, bus, "чтение топиков")
    Rel(legacy, bus, "as-is типы: gm.*, narrative.generate, player.moved - помечены deprecated в реестре")
```

## Что здесь будущее

| Узел | Чего нет в дереве | Кто делает |
|---|---|---|
| `telegram-bot` | каталога `cmd/telegram-bot` нет; сервис профиля `bot` уже описан и стартует, как только появится бинарник | EPIC-004 |
| `gateway` (содержимое) | каталога `internal/gateway` нет; `cmd/multiverse --contexts=gateway` поднимает пустой контекст с `/health` | EPIC-004 |
| `core` (содержимое) | из `internal/{state,swarm,llm,laws,replay}` в дереве нет ни одного; есть только `internal/mechanics` | EPIC-002, EPIC-003 |
| `memory` (содержимое) | каталога `internal/memory` нет; Qdrant и Neo4j поднимаются, но клиента нет | EPIC-005 |
| `SQLite` | ни файлов, ни миграций `internal/gateway/migrations/**` | EPIC-004 |
| прокси `/v1/admin/*` | ни маршрутов, ни прокси; `shared/runtime` даёт только каркас `Routes(mux)` | EPIC-003 + EPIC-004 |

## Расхождения с деревом

1. **Число процессов совпадает, содержимое — нет.** `overview.md` §13 описывает `gateway`, `core`, `memory` через их ответственность, как будто она реализована. В дереве это один и тот же бинарник с разным набором **пустых** контекстов (`cmd/multiverse/contexts.go`, `stubContexts`). Диаграмма показывает границу процессов честно, содержимое — с пометками.
2. **Порядок контекстов в `core`.** `overview.md` §13 пишет `--contexts=state,mechanics,swarm,llm,laws`, `docker-compose.yml` запускает `--contexts=state,mechanics,laws,llm,swarm`. Порядок в списке ничего не решает (старт упорядочивает `runtime.Registry` по `DependsOn`), но два разных списка в двух источниках истины — повод для будущей ошибки; правится в `overview.md`.
3. **Адрес HTTP-сервера процесса.** Все три сервиса читают адрес из `MV_CORE_ADDR`, а не из `MV_GATEWAY_ADDR`/`MV_MEMORY_ADDR`: `cmd/multiverse/serve.go` знает одну переменную, а compose дублирует в неё нужное значение (комментарий D-7 в `docker-compose.yml`). `overview.md` §13 и `infrastructure.md` §4.2 называют три отдельные переменные так, будто их читает код.
4. **Профили `bot` и `legacy` живут в отдельных файлах compose** (`docker-compose.bot.yml`, `docker-compose.legacy.yml`, задача T-397), потому что `docker compose` интерполирует файл целиком до фильтрации по профилю. `overview.md` §13 перечисляет все профили так, будто они в одном файле.
5. **Профиль `legacy` сегодня не поднимается**: `CHROMA_IMAGE` в `build/versions.env` намеренно пуст до решения о теге. Это не дефект, а громкий отказ вместо тихой ошибки, но читатель `overview.md` §13 ждёт работающий профиль.
6. **`MV_LLM_URL` не имеет значения по умолчанию** (T-404): compose требует его явно. `overview.md` §13 пишет `MV_LLM_URL=http://host.docker.internal:1234` как данность.
