# ADR-010: Тестируемость — уровни тестов, testcontainers для интеграции, детерминированные e2e на записанных LLM-выводах в одном процессе, CI без GPU

Статус: предложено (к утверждению на G2) · Дата: 2026-09-09 · Автор: system-architect#1
Связи: OQ-A-10 (стратегия тестов), OQ-A-12; FR-100…FR-102, FR-086, FR-087; NFR-012, NFR-014, NFR-060…NFR-065, NFR-072; аудит §1 (тесты semantic-memory требуют живой Neo4j; CI на Go отсутствует).

## Контекст

As-is: 12 из 15 сервисов без тестов; интеграционные тесты semantic-memory не отделены и падают без Neo4j; CI только `qwen-*` AI-триаж и `validate-blueprints` по несуществующему пути; `go vet` root падает. Требования: unit без сети; e2e «Тёмный лес» (соло 30 ходов, группа из 3 — 30 раундов, фон с ускоренными тиками) в CI ≤ 10 мин без GPU на записанных LLM-выводах; replay побайтово идентичен; покрытие ядра ≥ 60 %.

## Рассмотренные варианты

| Вариант | Плюсы | Минусы |
|---|---|---|
| A. Build-tag `integration` + testcontainers-go в CI | изоляция, реальные Redpanda/MinIO/Qdrant, работает локально и в CI | Docker в CI; +30–60 с на job |
| B. Моки хранилищ, интеграция вручную | быстро | as-is «вручную» = никогда |
| C. Compose-профиль `test` в CI | близко к prod | медленнее, состояние между тестами, хуже параллелизм |
| E2E через Docker-стек | «как в проде» | нужен GPU или мок Ollama в сети; минуты на старт |
| **E2E в одном процессе `--contexts=all --mode=replay` с in-memory шиной** | секунды; без Docker; детерминизм | in-memory шина должна воспроизводить семантику Redpanda (порядок в топике, at-least-once — эмулируется дублями) |

## Решение

1. **Уровни**:
   - **unit** (`go test -short ./...`): без сети и Docker; интерфейсы `Store`, `Provider`, `Bus`, `Clock`, `MemoryClient` имеют in-memory/fake реализации в `shared/testkit`; обязательные unit-наборы: RNG (NFR-060: 1000 × 4), правила боя (таблица исходов, нат. 20/1, бегство, цель волка), инварианты State, страж (9 правил), парсер/валидатор блупринтов, реестр контрактов (все схемы валидны, все типы имеют издателя/потребителя), OpenAPI ↔ маршруты;
   - **integration** (`//go:build integration`, `go test -tags integration ./...`): testcontainers-go модули `redpanda`, `minio`, `qdrant`, `neo4j`; проверяют адаптеры (шина: порядок/дедуп/DLQ; objstore: versioning; memory: индекс/поиск; SQLite: миграции/forget);
   - **e2e** (`//go:build e2e`): один процесс `cmd/multiverse --contexts=all --mode=replay --bus=memory --recording=testdata/recordings/<scenario>.jsonl` + CI-харнесс через HTTP API (`X-Client-Id: ci-harness`, `X-Actor-Kind: ci`); сценарии: `solo-30` (S1), `group-3x30` (S2, с явным `rounds/close`), `background-6h` (S14, `POST /v1/admin/agents/{id}/tick`), `recovery` (S3: 10 ходов → рестарт контекстов в процессе → `analytics.replay.completed identical=true`), `death`, `flee-fail`, `injections-10` (NFR-043), `privacy-scan` (NFR-041);
   - **golden** (NFR-065): 20 записанных ходов с эталонами — сравнение схемы/языка/чисел/фильтра (a);
   - **nightly-gpu** (вне обязательного CI, на целевой машине): матрица замера §18.1, живой Ollama, `ops/metrics/baseline.md`.
2. **Записи LLM** (`testdata/recordings/*.jsonl`): последовательность `llm.output` с ключом `correlation_id+agent.id+phase+attempt`; создаются `mvctl record --scenario …` на машине с GPU и коммитятся; `RecordedProvider` при промахе ключа — ошибка теста (не тихий шаблон). Обновление записей — осознанная задача с ревью диффа нарративов.
3. **Детерминизм** (NFR-061): `EventClock`, seed по ADR-003, фиксированные `blueprint_version`, `ULID` из детерминированного источника в тестовом режиме (`--id-source=sequence`), сортировка map при сериализации; тест сравнивает хэш последовательности доменных событий двух прогонов.
4. **In-memory шина** (`shared/testkit/membus`): топики как упорядоченные очереди, ключ игнорируется (одна партиция), режим `--chaos=duplicate` переиздаёт 5 % событий (NFR-013), `--chaos=reorder-topics` тасует между топиками (проверка ожидания по `correlation_id`).
5. **CI** (`.github/workflows/go.yml`): job `unit` (build, `go vet`, `staticcheck`, `depguard` границ, `go test -short -cover`, порог покрытия ядра ≥ 60 % по пакетам `internal/{state,mechanics,swarm,llm,replay}`), job `integration` (Docker, testcontainers), job `e2e` (без Docker), job `contracts` (`mvctl contracts check`, `mvctl blueprint validate blueprints/`, `.env.example` ↔ `os.Getenv`), job `security` (`gitleaks`, `govulncheck`), job `compose-lint` (пины образов, NFR-071). Всё — на `push`/`pull_request` в `main`/`develop`/`epic/*`; ≤ 10 мин суммарно. Существующие `qwen-*` workflow — оставить (не мешают), `validate-blueprints.yml` — заменить job `contracts`.
6. **Тестовые роли и данные**: фикстуры `dark-forest-world`, `dark-forest-01`, `wolf-alpha`, `player-A/B/C` — из `blueprints/` и `testdata/`; `actor_kind=ci` исключает прогоны из S7 (BR-17).

## Последствия

- Позитивные: e2e за секунды без GPU и Docker; регрессии контрактов ловятся на PR; замер на GPU — отдельная, повторяемая процедура.
- Негативные: две реализации шины (Redpanda и in-memory) — расхождение семантики ловится интеграционными тестами адаптера; записи LLM — бинарные фикстуры в git (≈ 100 КБ/сценарий).
- Что придётся сделать: EPIC-001 — `shared/testkit`, CI-workflow, testcontainers; EPIC-002/003/004 — unit-наборы своих контекстов; EPIC-005 — `mvctl record`, `--audit`, golden; devops — Docker в CI-раннере, кэш модулей.

## Дополнение 2026-09-09 (сведение A3 шаг 4)

1. **Записи и golden только из CI-сессий (SEC-23, п. 2/6 уточнение)**: `mvctl record` принимает только события с `meta.actor_kind=ci` и фикстурных игроков `player-A/B/C`; при обнаружении `actor_kind=human` в сценарии — отказ; job `privacy-scan` (часть `security`) сканирует `testdata/` на числовые внешние ID/username теми же правилами, что тест NFR-041.
2. **`.gitattributes` для записей**: `testdata/recordings/*.jsonl merge=binary` (конфликты решаются перегенерацией, не ручным слиянием); текстовый diff **сохраняется** (одна запись = одна строка — ревью диффа нарративов в PR остаётся возможным); `mvctl record diff` — удобное представление, не замена.
3. **CI hardening (SEC-25, п. 5 уточнение)**: Actions пинятся по SHA (Dependabot `github-actions` обновляет), `permissions: contents: read` по умолчанию, `go mod verify` и `go mod tidy -diff` в job `unit`, Dependabot для `gomod`/`github-actions`/`docker`, `CODEOWNERS` на `blueprints/`, `laws/`, `config/`, `.github/`, `schemas/`, `shared/`.
4. **`compose-lint`** проверяет: пины образов (NFR-071), публикацию портов только на `127.0.0.1` (ADR-009 дополнение п. 3), отсутствие паролей по умолчанию, `env_file` с токеном бота только у сервиса `telegram-bot` (профиль `bot`), версии из `build/versions.env`.
5. **`govulncheck`** — в job `security`, блокирующий с переходом на Go 1.26 (ADR-001 дополнение п. 1).
6. **Integration** использует версии образов из `build/versions.env` (`testkit.Versions()`), чтобы совпадать с compose (ADR-004 дополнение п. 7); образ MinIO — собранный `build/minio.Dockerfile` (ADR-021).
