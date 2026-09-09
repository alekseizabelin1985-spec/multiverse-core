# Стратегия тестирования проекта

Версия 0.1 · 2026-09-09 · qa-engineer#1 (TEAM-1) · статус: к утверждению на G3 (объединённый).
Основание: ADR-010 (с дополнением 2026-09-09), `architecture/contracts.md` v0.2, `plan/epics.md` v0.2, `plan/teams.md` v0.1, `epics/*/design.md` v0.1, `architecture/components/*.md` v0.2, `requirements/user-stories.md` v0.4, `requirements/nfr.md` v0.4, `project/vision.md` v1.1 (S1–S14), `project/metrics.md` v0.2 §7, `architecture/threat-model.md` §6–§7, `architecture/infrastructure.md` §3.
Область: MVP-1 «Тёмный лес» (EPIC-001…005). Целевые эпики EPIC-006…013 — вне области, стратегия расширяется при их старте.

---

## 1. Цели, принципы, уровни

### 1.1. Цели

1. Подтвердить критерии успеха MVP-1 **S1, S2, S3, S4, S5, S6, S8, S9, S10, S14** воспроизводимыми прогонами (S7 — наблюдение после G4, S11–S13 — целевые).
2. Ловить регрессию контрактов `C-01…C-15` на PR, а не на интеграции: три команды работают параллельно на заглушках, и расхождение «заглушка ↔ реализация» — главный класс дефектов этого проекта.
3. Сделать проверку **дешёвой**: полный обязательный набор в CI ≤ 10 мин, без GPU и (для e2e) без Docker — иначе при одном человеке-ревьюере тесты перестанут запускать.
4. Отделить то, что можно утверждать сейчас (детерминизм, контракты, приватность), от того, что можно утверждать **только после замера** на стенде (все пороги латентности и качества LLM).

### 1.2. Принципы

- **Одно поведение — один уровень.** Юнит-набор разработчика (таблицы правил, парсеры, валидаторы) в планах QA не дублируется; QA отвечает за поведение системы: контракты, сквозные сценарии, деградации, приватность, восстановление.
- **Приоритет по риску, не по коду.** Сначала то, что ломает основную ценность (ход игрока не доходит до нарратива) или данные (расхождение состояния, утечка внешнего ID), затем удобства.
- **Детерминизм — требование, а не пожелание.** Любой автотест, который может «иногда» пройти, считается дефектом теста: часы — `clock.Manual`/`EventClock`, ID — `--id-source=sequence`, LLM — `providers/recorded` (промах ключа = ошибка, не тихий шаблон), шина — `membus`.
- **Заглушка и реализация проходят один и тот же contract-тест.** `membus` ↔ kafka-адаптер, `FakeState` ↔ `internal/state`, `FakeNarrator` ↔ рой, `FixedMechanics` ↔ `Rules.Resolve`, `FakeGateway` ↔ `internal/gateway`. Это единственная защита от «на заглушке работало».
- **Пороги «после замера» не блокируют разработку.** До фиксации в `ops/metrics/baseline.md` они — ориентир проектирования; критерием приёмки становятся только после протокола B1–B8 (§6.3).
- **Безопасность не дублируется.** Чек-лист `threat-model.md` §6 выполняется security-engineer в `epics/*/security-review.md`; стратегия на него ссылается и лишь встраивает в критерии выхода (§6.2).
- **Тестовые данные не содержат ПДн по построению.** Фикстуры `player-A/B/C`, `actor_kind=ci`; `mvctl record` отклоняет `actor_kind=human`; job `privacy-scan` сканирует `testdata/`.

### 1.3. Уровни и их доля

| Уровень | Тег/запуск | Где живёт | Кто пишет | Доля (ориентир) | Что доказывает |
|---|---|---|---|---|---|
| **unit** | `go test -short ./...` | рядом с кодом | developer#N (DoD задачи) | ~60 % кейсов | правила, инварианты, парсеры, валидаторы, лимиты; без сети и Docker (NFR-063) |
| **contract (CT)** | `-short` на `membus`/фейках + `-tags integration` на testcontainers | `shared/contracts`, `shared/testkit`, пакет-владелец | developer владельца контракта, ревью qa-engineer команды | ~10 % | что заглушка и реализация одного контракта `C-NN` неотличимы для потребителя |
| **integration (IT)** | `-tags integration` (Docker) | пакет-адаптер | developer#N | ~10 % | адаптеры к Redpanda/MinIO/Qdrant/Neo4j/SQLite: офсеты, versioning, миграции, физическое удаление |
| **e2e детерминированный (E2E)** | `-tags e2e`, один процесс `cmd/multiverse --contexts=all --mode=replay --bus=memory` | `internal/*/e2e_test.go` + сценарии EPIC-005 | developer#N (каркас), qa-engineer#N (сценарий) | ~12 % | сквозной цикл, восстановление, деградации, приватность — за секунды, без GPU |
| **e2e бота** | `-tags e2e` | `cmd/telegram-bot` | developer TEAM-3 | входит в 12 % | тексты, клавиатуры, ack, отказ чужому id/групповому чату — на `FakeUpdateSource`/`FakeSender`/`FakeGateway` |
| **golden LLM** | `mvctl golden check` в job `e2e` | `testdata/golden`, `internal/llm` | EPIC-003 (записи) + 005-ops (эталоны) | ~3 % | регрессия качества: схема, язык, соответствие механике, 0 ложных срабатываний фильтра (a) |
| **межэпиковый (INT)** | ручной запуск на `integration/mvp-1` (те же теги + стенд) | `testing/integration/` (сценарии), выполнение — tester#N | qa-engineer#1 (план), tester#N (выполнение) | ~5 % | контракты между эпиками на реальной сборке (§5) |
| **стендовый (ST)** | вручную на машине владельца | чек-листы в тест-планах, отчёты в `ops/metrics/` | человек + tester#N/architect#N | ~15 % кейсов, 0 % в CI | живой Ollama, живой Telegram, GPU, сеть, замер B1–B8 |
| **security-review** | чек-лист | `epics/*/security-review.md` | security-engineer | — | `threat-model.md` §6 (не дублируется здесь) |
| **privacy-scan** | job `security` + e2e-кейс + стенд | `testdata/`, e2e, стенд | EPIC-004 (e2e), devops (job), tester#3 (стенд) | — | NFR-041/042, SEC-01/03/23 |

### 1.4. Что автоматизируем обязательно

- Все контракты `C-01…C-15` (CT) — блокируют мерж.
- Сквозные сценарии MVP-1: `solo-30`, `group-3x30`, `recovery`, `background-6h`, `death`, `flee-fail`, `degraded`, `forget`, `injections-10`, `privacy-scan`, `bot-flow`, `chaos-duplicate`, `determinism`, `golden`, `second-region`.
- Детерминизм (NFR-061) и идемпотентность при дублях (NFR-013).
- Отсутствие внешних ID в шине, снапшотах, логах, `testdata/` (NFR-041).
- Восстановление всех трёх stateful-компонентов (C-14).

### 1.5. Что проверяем только вручную (стенд)

Живой Ollama и GPU (все пороги латентности и качества LLM), живой Telegram (allowlist, личные чаты, реальные тексты и клавиатуры), `make up`/`make down` и тома, сетевая экспозиция портов, отзыв секретов и история git, бэкап/восстановление `links.db`, качество нарратива по чек-листу (NFR-023/024), профиль `legacy` (S5), инструкция «второй регион» (S6, ручной проход автора), наблюдение S7.

### 1.6. Что **не** тестируем в MVP-1 (осознанно)

| Не тестируем | Почему | Когда вернуть |
|---|---|---|
| Аутентификация клиентов, токены | вне объёма MVP-1 (FR-008); защита — allowlist | EPIC-013 |
| Нагрузка > 6 игроков / > 50 сущностей в scope | NFR-080 — целевой масштаб MVP-1 | EPIC-012 |
| WebSocket `/stream` | зарезервирован, в MVP-1 `501` — проверяется только код ответа | EPIC-013 |
| Пробой законов `world.law_breach.*` | типы зарегистрированы без издателя (C-12); проверяется только валидность схем и `laws_version` | EPIC-007 |
| Entity-Actor, эволюция правил (C-13) | резерв контракта; проверяется только то, что блупринт `level: object` валидируется и спавн выключен флагом | EPIC-006 |
| Полный фильтр 18+, категории (b)–(g) | MVP-1 — только категория (a), fail-closed | EPIC-013 |
| Prometheus/Grafana/алерты | метрики MVP-1 — CSV + CLI | EPIC-012 |
| Совместимость с Windows-сборкой в CI | целевая платформа — Linux-контейнер; локальная сборка проверяется разработчиком | при появлении второй платформы |
| Замороженные сервисы и `services/_archive/**` | вне `go build ./...`; профиль `legacy` проверяется только сценарием S5 | при возврате кода |

---

## 2. Инструменты, запуск, отчёты

### 2.1. Команды

| Что | Команда | Примечание |
|---|---|---|
| unit | `go test -short -race ./...` / `make test` | без сети и Docker; порог покрытия `internal/{state,mechanics,swarm,llm,replay}` ≥ 60 % (`scripts/coverage-gate.sh`) |
| integration | `go test -tags integration -count=1 -timeout 15m ./...` / `make test-integration` | testcontainers-go; версии образов — `testkit.Versions()` из `build/versions.env`; MinIO — образ `build/minio.Dockerfile` |
| e2e | `go test -tags e2e -count=1 -timeout 10m ./...` / `make test-e2e` | `cmd/multiverse --contexts=all --mode=replay --bus=memory --recording=testdata/recordings/<scenario>.jsonl --id-source=sequence`; харнесс ходит по HTTP API с `X-Client-Id: ci-harness`, `X-Actor-Kind: ci` |
| хаос | `--chaos=duplicate` (5 % дублей), `--chaos=reorder-topics` | NFR-013 и проверка ожидания по `correlation_id` |
| контракты | `mvctl contracts check`, `mvctl blueprint validate blueprints/`, `mvctl env check` | job `contracts` |
| golden | `mvctl golden check solo-30` | job `e2e`; обновление — `mvctl golden update` отдельной задачей с ревью |
| запись LLM | `mvctl record --scenario <name>` на стенде → `testdata/recordings/*.jsonl` | только `actor_kind=ci` и фикстурные игроки; `.gitattributes: merge=binary`, текстовый diff сохраняется |
| отчёт сессии | `mvctl session-report <session_id> [--audit] [--json] [--weekly] [--from-recording]` | CSV `ops/metrics/sessions.csv`, JSON `ops/metrics/sessions/<id>.json` |
| трасса | `mvctl trace <correlation_id>` (журнал, Must) / `GET /v1/trace/{id}` (память, Should) | NFR-034 |
| замер | `make bench` на стенде → `ops/metrics/baseline.md` | протокол B1–B8 |

### 2.2. Тестовый инструментарий (что должно существовать)

`shared/testkit`: `membus` (шина + `Journal` + `--chaos`), `Dedup`, `Versions()`, `containers`; `testkit/state.FakeState`, `testkit/mechanics.FixedMechanics`; `testkit/gateway.Harness`, `FakeGateway`; `testkit/swarm.FakeNarrator` (+`WithEncounterStub`, `FakeEncounter`), `RecordingWriter`, `template/ru.go`; `shared/clock.Manual`/`ManualTimers`; `internal/replay.EventClock`/`NullTimers`; `providers/{fake,recorded}`; фикстуры `testdata/fixtures/{world,region,npc,players}.json`, `testdata/analytics/*.jsonl`, `testdata/recordings/*.jsonl`, `testdata/golden/`.

### 2.3. CI (`.github/workflows/go.yml`, F-7)

Триггеры `push`/`pull_request` в `main`, `develop`, `epic/**`; блокируют мерж job'ы `unit`, `integration`, `e2e`, `contracts`, `security`, `compose-lint`; суммарно ≤ 10 мин. Артефакты: `coverage.out`, `ops/metrics/sessions/*.json` из e2e — прикладываются к отчёту о прогоне.
Вне CI: `make bench` на стенде (GPU-раннера нет; self-hosted — решение владельца, открытый вопрос devops).

### 2.4. Отчётность

| Артефакт | Кто ведёт | Когда |
|---|---|---|
| Прогон CI (ссылка + номер) | автоматически | каждый PR |
| `ops/metrics/sessions.csv` + `sessions/<id>.json` | tester#N через `session-report` | каждый прогон S1/S2 и каждая живая сессия |
| `ops/metrics/incidents.csv` | оператор/tester | при ручной правке состояния (`manual_fix=true`) |
| `ops/metrics/baseline.md` | architect#1 + человек | после каждого замера B1–B8 |
| Отчёт инкремента (что прогнали, дефекты, отклонения) | qa-engineer#1 | на выходе из интеграции I1-α / I1 / I2 |
| `testing/regression.md` | qa-engineer#1 | пополняется после каждого дефекта (§7.4) |

---

## 3. Матрица покрытия: критерии успеха, NFR, US → тесты

Обозначения уровня: **CT** контракт, **IT** интеграционный, **E2E** детерминированный e2e, **INT** межэпиковый (§5), **ST** стендовый, **UT** юнит (в DoD разработчика). А = автомат (CI), С = стенд/ручной.

### 3.1. Критерии успеха S1–S14

| S | Формулировка (кратко) | Тесты | А/С | Эпик-владелец | Инкремент |
|---|---|---|---|---|---|
| S1 | соло 30 ходов без поломки мира | E2E-01, INT-01, INT-02, INT-03, ST-02, ST-03 | А + С | 003 (ведущий), 002, 004 | I1 |
| S2 | 30 раундов группы из 3, одна версия событий | E2E-02, INT-07, ST-06 | А + С | 004 (раунды), 003 (нарратив), 002 (atomic) | I2 |
| S3 | восстановление, состояние идентично, LLM = 0 | E2E-03, INT-06, IT-04, IT-05 | А | 002 (формат), 003, 004 | I1 (соло) / I2 (раунды) |
| S4 | латентность на целевом железе | ST-04 (B1, B7), INT-09 | С | 003 + architect#1 | I1 замер, I2 фиксация |
| S5 | одна архитектура GM, `legacy_gm_path_share = 0` | INT-14, ST-07 | А + С | 003 | I2 |
| S6 | новый регион блупринтом без Go | E2E-18, ST-08, CT-11 | А + С | 003 | I1b/I2 |
| S7 | 8 сессий / 3 групповых за 4 недели без поломок | ST-15 (наблюдение), `session-report --weekly` | С | 005-ops (инструмент) | после G4 |
| S8 | ноль внешних затрат, включая фон | E2E-04 (счётчик провайдеров), ST-16 | А + С | 003 | I1 |
| S9 | приватность и уведомление | E2E-08, E2E-10, E2E-11, ST-03, ST-05 | А + С | 004 | I1 |
| S10 | секреты вне репозитория, токены отозваны | job `security`, ST-11 | А + С | 001 | волна 0 |
| S14 | фоновая жизнь между сессиями | E2E-04, INT-04, ST-04 (B8) | А + С | 003 | I1 |
| S11–S13 | целевые | вне области MVP-1 | — | 006/007/008/010 | — |

### 3.2. NFR

| NFR | Тест | Уровень | А/С | Владелец | Примечание |
|---|---|---|---|---|---|
| NFR-001 механика p95 | ST-04 (B1), INT-09 | ST/INT | С | 002/003 | порог после замера |
| NFR-002 нарратив p95 | ST-04 (B1) | ST | С | 003 | порог после замера |
| NFR-003 ack ≤ 300 мс | ST-03, UT (таймер бота) | ST | С | 004 | |
| NFR-004 холодный старт | ST-04 (B2) | ST | С | 003 | |
| NFR-005 группа не деградирует (6 игроков) | **ST-14** | ST | С | 004 + 003 | **нет владельца в дизайнах — см. §8 пробел G-1** |
| NFR-006 бюджет вызовов на ход/раунд | E2E-01/02 (счёт `llm.output` по `phase`), ST-04 (B7) | E2E | А | 003 | |
| NFR-007 бюджет фона ≤ B/час | E2E-04, INT-04 | E2E/INT | А | 003 | |
| NFR-010 RTO, состояние после рестарта | E2E-03, INT-06, ST-04 (B6) | E2E/ST | А + С | 002 | RTO — после замера |
| NFR-011 RPO = 0, retention | E2E-03, IT-03 | E2E/IT | А | 002/001 | |
| NFR-012 мир не ломается | E2E-01, E2E-02 (сверка снапшот ↔ журнал, инварианты, паники) | E2E | А | 002 | |
| NFR-013 идемпотентность | E2E-13, IT-07, CT-01 | E2E/IT/CT | А | 001/002/004 | |
| NFR-014 replay без недетерминизма | E2E-03, E2E-12 | E2E | А | 002/003 | |
| NFR-015 деградация без LLM | E2E-09 | E2E | А | 003 | |
| NFR-016 отказ зависимости виден ≤ 30 с | **ST-09** | ST | С | 001/devops | **автотеста нет — пробел G-2** |
| NFR-020 инварианты 1–10 | UT (`state`), E2E-01/02 (проверка после каждого хода) | UT/E2E | А | 002 | |
| NFR-021 LLM в своём уровне | E2E-07, CT-06, UT (страж 9×2) | E2E/CT | А | 003 | |
| NFR-022 валидность JSON | ST-04 (B3), E2E-15 (golden) | ST | С | 003 | порог только на стенде |
| NFR-023/024 расхождение нарратива и канона | **ST-12** (чек-лист 100 ходов) | ST | С | qa-engineer#2 + человек | измерение без порога; **исполнитель не назначен — пробел G-3** |
| NFR-026 контракт пробоя, `laws_version` | INT-11, CT-12 | INT/CT | А | 003 | |
| NFR-030 `/health` всех сервисов | ST-01, E2E-14 | ST/E2E | А + С | 001 | |
| NFR-031 сквозной `correlation_id` | INT-01 (проверка 100 % цепочки), E2E-16 (`trace`) | INT | А | 001 (конверт) | |
| NFR-032 метрики LLM и цикла | E2E-16, INT-09 | E2E/INT | А | 005-ops | |
| NFR-033 структурированные логи, `handled` | UT + E2E-08 (grep) | UT/E2E | А | 001 | |
| NFR-034 трасса решения | E2E-16, CT-07 | E2E | А | 005-ops / 003 | |
| NFR-036 аналитика собираема и приватна; не влияет на replay | INT-09, E2E-08, CT-10 | INT/CT | А | 004 | |
| NFR-040 секреты | job `security`, ST-11 | CI/ST | А + С | 001 | |
| NFR-041 псевдонимизация | E2E-08, IT-06, ST-03, job `privacy-scan` | E2E/IT/ST | А + С | 004 | |
| NFR-042 удаление связки | E2E-10, IT-06, ST-05 | E2E/IT/ST | А + С | 004 | |
| NFR-043 инъекции | E2E-07 (10 прямых + 3 второго порядка) | E2E | А | 003 (+005 память) | |
| NFR-044 allowlist, личные чаты | E2E-11, UT (бот) | E2E | А | 004 | |
| NFR-045 уведомление, согласие, возраст | E2E-11, IT-06 | E2E/IT | А | 004 | |
| NFR-046 юрисдикция и облако | **ST-16** | ST | С | 003/004 | **негативные случаи US-014 не в дизайнах — пробел G-4** |
| NFR-048 контент (a) | E2E-15 (golden, 0 FP), UT (фильтр), security-review | E2E | А | 003 | |
| NFR-049 лимиты частоты и размера | E2E-19, UT (бот и gateway), ST-14 | E2E/UT | А + С | 004 | «флудящий не мешает другим» — только ST-14 |
| NFR-050 ноль внешних вызовов | E2E-04 (счётчик провайдера), ST-16 | E2E | А | 003 | |
| NFR-051 лимит облака | **ST-16** | ST | С | 003 | облако — Could в MVP-1, кейс негативный |
| NFR-052 учёт токенов ≤ 5 % | **ST-13** | ST | С | 005-ops + 003 | **сверка с отчётом Ollama не назначена — пробел G-5** |
| NFR-053 мир без игроков | E2E-04, INT-04 | E2E/INT | А | 003 | |
| NFR-060 детерминированный RNG | UT (1000 × 4, χ²) | UT | А | 002 | |
| NFR-061 record-replay | E2E-12, E2E-20, INT-05 | E2E/INT | А | 002/003 | |
| NFR-062 e2e в CI ≤ 10 мин | job `e2e` (timeout 10m) | CI | А | 001/devops | |
| NFR-063 разделение уровней | job `unit` (без Docker) | CI | А | 001 | |
| NFR-064 покрытие ≥ 60 % | `coverage-gate.sh` в job `unit` | CI | А | все | |
| NFR-065 золотой набор | E2E-15 | E2E | А | 005-ops (эталоны) + 003 (записи) | |
| NFR-070 `make up` ≤ 3 мин | ST-01, ST-04 (B5) | ST | С | devops | |
| NFR-071 пины версий | job `compose-lint` | CI | А | devops | |
| NFR-072 работа без GPU | job `e2e`, ST-01 (вариант «без GPU») | CI/ST | А + С | 001 | |
| NFR-073 ресурсный след | ST-04 (B5) | ST | С | devops | |
| NFR-074 конфигурация | CT-15 (`mvctl env check`) | CT | А | 001 | |
| NFR-075 число процессов — параметр | E2E-14 (`--contexts` комбинации) | E2E | А | 001 | Could |
| NFR-076 две модели одновременно | ST-04 (B1, `ollama ps`) | ST | С | architect#1 | |
| NFR-080 масштаб | ST-14 | ST | С | 004 | |
| NFR-083 размер роя ≤ 20 агентов | E2E-02/04 (`GET /v1/admin/agents`), UT (Lifecycle) | E2E | А | 003 | |
| NFR-090 русский язык | E2E-15, ST-04 (B4) | E2E/ST | А + С | 003 | порог латиницы после замера |
| NFR-091 `locale` | CT-06 | CT | А | 003 | |
| NFR-092 совместимость клиентов | CT-08 (один Go-клиент у бота и харнесса) | CT | А | 004 | |
| NFR-093 совместимость среды | job `unit` (Linux) + ST-01 | CI/ST | А + С | devops | |
| NFR-095 сопровождаемость | ревью документации (F-9), не тест | — | С | tech-writer | |

### 3.3. Пользовательские истории MVP-1

| US | Ключевые критерии → тесты | А/С | Эпик |
|---|---|---|---|
| US-001 создание персонажа | UT (таблица предусловий), E2E-11, CT-05 | А | 004 |
| US-002 соло 30 ходов | E2E-01, E2E-05, E2E-06, INT-01…INT-03, ST-02 | А + С | 002/003/004 |
| US-003 предсказуемая механика | UT (`Resolve`, `Seed`), E2E-01, E2E-16 (`trace`) | А | 002 |
| US-004 нарратив асинхронно и деградация | E2E-09, E2E-01 (порядок «механика → нарратив») | А | 003 |
| US-005 мир помнит последствия | E2E-01 (волк не возрождается), E2E-04 (сводка из журнала), **INT-08** (приоритет `authored` над `generated`) | А | 003 (+005-memory) |
| US-006 групповая сессия | E2E-02, INT-07 | А | 004 |
| US-007 бой по раундам | E2E-02, E2E-19 (`409 already_acted`), UT (`rounds` с `FakeClock`) | А | 004 |
| US-008 Telegram-бот | E2E-11, ST-02, ST-03 | А + С | 004 |
| US-009 псевдонимизация и `/forget` | E2E-08, E2E-10, IT-06, ST-03, ST-05 | А + С | 004 |
| US-010 запуск стека | ST-01, E2E-14 | А + С | 001/devops |
| US-011 восстановление | E2E-03, INT-06, IT-04, IT-05 | А | 002/003/004 |
| US-012 контроль LLM | E2E-04, E2E-16 (`llm usage`), ST-16 | А + С | 003/005-ops |
| US-013 гигиена секретов | job `security`, ST-11 | А + С | 001 |
| US-014 переключение провайдера | CT-14, **ST-16** (негативные: без ключа, внешние игроки) | А + С | 003 |
| US-015 регион блупринтом | CT-11, E2E-18, ST-08 | А + С | 003 |
| US-016 фильтр и абсолютные запреты | UT (фильтр, `fail-closed`), E2E-15, security-review | А | 003/004 |
| US-017 воспроизводимый прогон | E2E-12, E2E-20, INT-05 | А | 002/003 |
| US-018 LLM-выводы как события | CT-07, CT-12, E2E-16 | А | 003 |
| US-019 фича-флаг миграции | UT (оба значения флага), INT-14, ST-07 | А + С | 004/003 |
| US-036 мир живёт между сессиями | E2E-04, INT-04 | А | 003 |
| US-037 рой GM — иерархия | E2E-04 (цепочка `global → domain → task`), E2E-07 (уровни), E2E-02 (`admin/agents`) | А | 003 |
| US-038 отчёт по сессии | E2E-16, INT-09, CT-10 | А | 005-ops/004 |

### 3.4. Каталог автотестов

**Contract-тесты (CT), 16 — блокируют мерж** *(CT-02a добавлен сведением 3)*

| ID | Контракт | Что проверяет | P | Владелец |
|---|---|---|---|---|
| CT-01 | C-01 | порядок в топике, at-least-once, retry ×3 → `dead_letters`, `ReadRange` по возрастанию, `End` монотонен, `Tail`, `PositionFromContext`, невалидное при чтении → DLQ; **один набор для `membus` и kafka** | P1 | 001 |
| CT-02 | C-01 | реестр: каждый тип — схема, издатель, ≥ 1 потребитель (исключения `world.law_breach.*`, `rules.change.*`); примеры `api-contracts.md` валидны; политики топиков (`player_events` без `meta.agent`) | P1 | 001 |
| CT-02a | §0, §16 п. 7 | **`source ∈ Spec.Publishers`** для каждого события фикстур и e2e (тип — владелец схемы, издатели — список реестра); `testkit/*` как источник допустим только на `membus`/`MV_SWARM_FAKE=true` *(сведение 3, tech-lead#1 по запросу QA)* | P1 | 001 |
| CT-03 | C-02 | предложение → факт/отказ по всем `reason`; `atomic` all-or-nothing и по-сущностный отказ; дедуп `proposal_id`; `applied_at = timestamp` предложения; `changed[]` для `append`; **`FakeState` и `internal/state` — один набор** | P1 | 002 |
| CT-04 | C-03 | сигнатуры и детерминизм `Resolve`/`Seed`/`NPCTarget`/`Roll`; **`FixedMechanics` ↔ `Rules` эквивалентны на наборе seed** | P1 | 002 |
| CT-05 | C-04 | схемы `player.*`/`group.*`/`round.*`; `round.opened` до `202`; `say` не открывает раунд и не входит в `acted[]`; в `solo` `round.*` отсутствуют | P1 | 004 |
| CT-06 | C-05 | схема `narrative.output`, один на ход/раунд, `recipients` = scope (включая `idle`), `filter.status`, `locale`; **`FakeNarrator` ↔ рой — один набор** | P1 | 003 |
| CT-07 | C-06/C-07 | `tick.lod_allowed` обязателен, `tick_seq` монотонен; `llm.output` до использования ответа; уникальность ключа записи `(correlation_id, agent.id, phase, attempt)`; admin-маршруты ↔ раздел `admin` OpenAPI | P1 | 003 |
| CT-08 | C-08 | OpenAPI ↔ маршруты и коды (`403 client_*`, `409 no_open_round`/`already_acted`/`poll_in_progress`, `413`, `429`, `501`); идемпотентность `action_key`; **`FakeGateway` ↔ `internal/gateway` — один набор** | P1 | 004 |
| CT-09 | C-09 | `memory.openapi.yaml` ↔ маршруты; `MemoryClient` и `journalContext` — один интерфейс, деградация без ошибки | P2 | 005 |
| CT-10 | C-10 | схемы аналитики, парность `session.started/ended`, `turn.completed` на каждый принятый и отклонённый ход, отсутствие текста и внешних ID | P1 | 004 |
| CT-11 | C-11 | `mvctl blueprint validate` на 5 блупринтах; 15 правил валидатора; glob по реестру; модель против `Provider.Models()`; блупринт не выбирает провайдера | P1 | 003 |
| CT-12 | C-12 | `laws_version` в 100 % `llm.output`/`narrative.output`/`snapshot.created`; типы пробоя валидны и исключены из проверки «есть издатель» | P2 | 003 |
| CT-13 | C-14 | схема `snapshot.created`, `component` enum, `latest.json` — указатель, `cursor` по всем читаемым топикам; одна схема `analytics.replay.completed` для `mode=recovery|test` | P1 | 002 (+005) |
| CT-14 | C-15 | `Provider{Generate, Embed, Health, Models}` — один набор для `ollama`/`recorded`/`fake`; облако только по `MV_LLM_CLOUD_ENABLED` | P2 | 003 |
| CT-15 | env | `mvctl env check`: реестр `shared/env` ↔ `.env.example`, префикс `MV_`, секреты пусты | P2 | 001 |

**Интеграционные (IT), 10**

| ID | Что | P | Владелец |
|---|---|---|---|
| IT-01 | `objstore` над собранным MinIO: `Put/Get/Stat/List/Delete`, `EnsureBucket` (versioning + ILM), `latest.json` после падения между PUT | P1 | 001/002 |
| IT-02 | kafka-адаптер: набор CT-01 на testcontainers Redpanda | P1 | 001 |
| IT-03 | `Journal` под записью: `ReadRange`/`Tail`/`End`, догон до `End` | P1 | 001/002 |
| IT-04 | State: снапшот + восстановление над MinIO + Redpanda; сценарии (a)–(g) recovery | P1 | 002 |
| IT-05 | Swarm: снапшот роя + `latest.json`, догон с курсора | P1 | 003 |
| IT-06 | Gateway: миграции `links.db`/`gateway.db`, `/forget` физически (файл + WAL, `strings`) | P1 | 004 |
| IT-07 | Gateway consumer: дедуп, курсоры, DLQ, `Journal.End` | P1 | 004 |
| IT-08 | `providers/ollama` против `httptest`: таймауты, отмена по `ctx`, ошибки, отсутствие утечки горутин | P2 | 003 |
| IT-09 | `mvctl --audit`: снапшот ↔ объекты, подложенное расхождение → `state_divergence` | P2 | 005-ops |
| IT-10 | Qdrant/Neo4j: upsert/search по scope, constraints, `Trace`, индексация ≤ 2 с | P3 | 005-memory (отрезаемо) |

**E2E детерминированные, 20**

| ID | Сценарий | Проверяет | P | Инкремент |
|---|---|---|---|---|
| E2E-01 | `solo-30` | S1, NFR-012, NFR-020, NFR-006 | P1 | I1-α (на заглушках) → I1 |
| E2E-02 | `group-3x30` | S2, US-006/007, NFR-083 | P1 | I2 |
| E2E-03 | `recovery` | S3, US-011, NFR-010/011/014 | P1 | I1 |
| E2E-04 | `background-6h` | S14, US-036/037, NFR-007/053, S8 | P1 | I1 |
| E2E-05 | `death` | US-002 (гибель), инварианты 1, 8 | P1 | I1 |
| E2E-06 | `flee-fail` | US-002 (провал бегства), свободная атака | P2 | I1 |
| E2E-07 | `injections-10` (+3 второго порядка) | NFR-043, NFR-021, SEC-17/18 | P1 | I1 (память — I2) |
| E2E-08 | `privacy-scan` | NFR-041, NFR-033, SEC-01/03 | P1 | I1 |
| E2E-09 | `degraded` | NFR-015, US-004 | P1 | I1 |
| E2E-10 | `forget` (соло и в группе): дополнительно проверяет `entity.updated status=abandoned cause=forget`, `group.left {cause: forget}` и `analytics.session.ended {end_reason: forget}` в обязательном порядке каскада C-04 v1.1 *(сведение 3, tech-lead#1 по запросу QA)* | NFR-042, US-009, FR-061 | P1 | I1 / I2 |
| E2E-11 | `bot-flow` | US-008, NFR-044/045 | P1 | I1 |
| E2E-12 | `determinism` (два прогона `solo-30`) | NFR-061 | P1 | I1 |
| E2E-13 | `chaos-duplicate` / `reorder-topics` | NFR-013 | P1 | I1 |
| E2E-14 | `stubs`/`empty-world` | готовность заглушек, `/health`, NFR-075 | P2 | волна 0 |
| E2E-15 | `golden check` (20 ходов + 3 тика) | NFR-065/048/090 | P1 | I1b |
| E2E-16 | `session-report --from-recording`, `--json`, `trace` | US-038, US-003, NFR-032/034 | P2 | I1 |
| E2E-17 | `memory-absence` | US-005/US-036 через память | P3 | I2 (отрезаемо) |
| E2E-18 | `second-region` (`domain-swamp`, `git diff --stat -- '*.go'` = 0) | S6, US-015 | P1 | I1b |
| E2E-19 | `limits` (`429`, `413`, `409 already_acted`, `409 poll_in_progress`) | NFR-049 | P2 | I1 / I2 |
| E2E-20 | `incomplete-record` (промах ключа записи) | US-017, ADR-010 п. 2 | P2 | I1b |

**Стендовые (ST), 16 — вручную на машине владельца, слот разработчика не занимают**

| ID | Что | Проверяет | P | Кто | Когда |
|---|---|---|---|---|---|
| ST-01 | `make up` → `make health` → `ollama ps` → `make down` → данные в томах целы | US-010, NFR-070/073/076/030 | P1 | devops + человек | волна 0, повтор перед I1/G4 |
| ST-02 | I1-α: живая игра через Telegram на шаблонах (создание персонажа, вход, бой, `/forget`) | US-001/002/008, замечания к 002/004 | P1 | человек + tester#3 | I1-α |
| ST-03 | S9: живой сквозной ход + privacy-scan по всем хранилищам и логам с тестовым Telegram ID | S9, NFR-041, NFR-003, SEC-01/03 | P1 | tester#3 + security-engineer | I1 |
| ST-04 | Матрица замера B1–B8 → `baseline.md` | NFR-001/002/004/006/007/010/022/052/070/073/076/090, S4, S14 | P1 | architect#1 + человек | конец I1a (F-8), I1, I2 |
| ST-05 | Учения `/forget`: `strings` по `links.db`+WAL чист; бэкап зашифрован, восстановление выполнено | NFR-042, SEC-04/05 | P1 | tester#3 + devops | I1, повтор перед G4 |
| ST-06 | S2 на трёх живых Telegram-аккаунтах: раунды, таймаут, выход участника | S2, US-006/007 | P1 | человек + tester#3 | I2 |
| ST-07 | S5: профиль `legacy` vs `agent`, сравнение нарративов, снятие флага | S5, US-019 | P2 | tester#2 | I2 |
| ST-08 | S6: второй регион по инструкции автора, без правки Go | S6, US-015 | P1 | человек (автор) | I1b/I2 |
| ST-09 | Остановка Ollama / MinIO / Redpanda по одной → `/health` ≠ ok ≤ 30 с, игра в деградации | NFR-015/016 | P1 | tester#1 + devops | I1 |
| ST-10 | `docker compose config` без публикаций на `0.0.0.0`; `netstat`/`nmap` с другого устройства LAN | SEC-13, NFR-071 | P1 | devops + security-engineer | I1, повтор перед G4 |
| ST-11 | `gitleaks` по истории PR, отзыв токенов владельцем, статус ключа в `shared/oracle/README.md` | S10, US-013, NFR-040 | P1 | security-engineer + человек | волна 0, повтор перед G4 |
| ST-12 | Чек-лист качества: выборка 100 ходов (урон, смерть, инвентарь, позиция, погода, канон) | NFR-023/024 | P2 | qa-engineer#2 + человек | после I1 |
| ST-13 | Сверка суммы `tokens` из `llm.output` с отчётом Ollama на 100 вызовах | NFR-052 | P2 | tester#2 | вместе с B7 |
| ST-14 | Нагрузка: 6 `sim`-игроков в одном scope 5 мин + один «флудящий»; латентность остальных | NFR-005/049/080 | P2 | tester#3 | I2 |
| ST-15 | Наблюдение S7: 4 недели, `session-report --weekly`, `incidents.csv` | S7 | P2 | человек + qa-engineer#1 | после G4 |
| ST-16 | Провайдер и облако: ключ не задан; облако при внешних игроках; денежный лимит; `config.cloud_enabled` без ключей | US-014, NFR-046/050/051 | P2 | tester#2 + security-engineer | I2 |

**Итого кейсов уровня стратегии: 75 (auto 59 — CT 15, IT 10, E2E 20, INT 14; manual 16 — ST); P1 — 51.** Юнит-кейсы разработчиков в счёт не входят (их перечень — в `components/*.md` и `epics/*/design.md`).

---

## 4. Пробелы покрытия (сводка)

| # | Пробел | Риск | Предложение |
|---|---|---|---|
| G-1 | NFR-005/NFR-049: нагрузочный прогон «6 `sim`-игроков в одном scope» и «флудящий не мешает остальным» не назван ни в одном дизайне | S | ST-14 как стендовая задача EPIC-004 I2; генератор `sim`-клиентов — расширение `testkit/gateway.Harness` (≤ S) |
| G-2 | NFR-016 «отказ зависимости виден ≤ 30 с» проверяется только руками | S | добавить IT-кейс: остановка контейнера в testcontainers → `/health` ≠ ok (EPIC-001, ≤ S) |
| G-3 | NFR-023/024 (расхождение нарратива с механикой и каноном) — метрика без исполнителя | Н | чек-лист 100 ходов ведёт qa-engineer#2 + человек после I1 (ST-12); порога в MVP-1 нет |
| G-4 | US-014/NFR-046/051: негативные случаи облака (ключ не задан, внешние игроки, лимит) не в тестах эпиков | С | ST-16 (EPIC-003, стенд, ≤ S) |
| G-5 | NFR-052 (сверка токенов с отчётом Ollama, ≤ 5 %) не назначена | С | ST-13 в рамках замера B7 (005-ops + 003) |
| G-6 | US-010 «`make down` → данные сохраняются в томах» и восстановление из бэкапа `links.db` (SEC-05) — только в чек-листе оператора | С | ST-01/ST-05 с явным шагом восстановления (devops) |
| G-7 | US-011 «рестарт между Phase 1 и Phase 2»: у EPIC-002 покрыта висящая запись, у EPIC-003 — свой `recovery`; сквозной случай ничей | В | INT-06 (обязателен на интеграции I1) |
| G-8 | US-005 «`authored` важнее `generated`» проверяется в 005-memory юнитом, но не со стороны роя | С | INT-08; при отрезании 005-memory — на факте из окна журнала |
| G-9 | NFR-031 «100 % событий цепочки несут `correlation_id`» — нет автоматической проверки полноты цепочки | С | утверждение в INT-01 и в `session-report --audit` |
| G-10 | S7 не может быть критерием выхода MVP-1 (наблюдение 4 недели после G4) | Н | зафиксировать в §6: S7 — критерий продукта, не тестирования |

---

## 5. Интеграционное тестирование между эпиками

### 5.1. Принципы

- Интеграция выполняется **на ветке `integration/mvp-1`** после слияния по порядку `plan/teams.md` §3.3: I1 — 002 → 004 → 003; I2 — 002 → 003 → 004 → 005.
- Каждый межэпиковый кейс привязан к контракту `C-NN` и проверяет **замену заглушки реализацией**: тот же CT-набор, но обе стороны настоящие.
- Дефект интеграции оформляется задачей `T-NNN` в эпик-владелец по карте владения (`plan/ownership.md`), **не** «быстрой правкой» в интеграционной ветке (`epics.md` §5 п. 6).
- План — `qa-engineer#1`; выполнение — `tester#1` (TEAM-1, координация), `tester#2` (сценарии роя и фона), `tester#3` (стендовые прогоны через Telegram); стыки gateway ↔ core — `security-engineer`; сборка — `devops-engineer`.

### 5.2. Волны интеграции

| Волна | Что влито | Обязательные кейсы | Кто | Выход |
|---|---|---|---|---|
| **I1-α** «соло на шаблонах» | 002 I1 + 004 I1 + `FakeNarrator` | INT-01 (на `FakeNarrator`), INT-09, INT-12, E2E-01, E2E-03, E2E-11, ST-02 | tester#1, tester#3, человек | тег `mvp-1/i1-alpha`; замечания человека → задачи 002/004 |
| **I1** «соло + фон» | + 003 I1a+I1b (замена `FakeNarrator`/`FixedMechanics`) | INT-01…INT-06, INT-09…INT-13, E2E-01/03/04/05/07/08/09/12/13/15/18, ST-03, ST-04 (B1, B6, B8), ST-09 | tester#1, tester#2, tester#3, security-engineer, devops | тег `mvp-1/i1`; S1, S3, S8, S9, S10, S14 |
| **I2** «группа + память/операции» | + 002 I2, 003 I2, 004 I2, 005 | INT-06 (раунды), INT-07, INT-08, INT-14, E2E-02/10/17/19, ST-05, ST-06, ST-07, ST-12…ST-16 | те же + release-manager | тег `mvp-1/i2`; S2, S4, S5, S6 → G4 |

### 5.3. Межэпиковые сценарии (INT)

| ID | Контракты | Сценарий (шаги → ожидаемое) | P | Волна |
|---|---|---|---|---|
| INT-01 | C-04 → C-02 → C-03 → C-05 → C-08 | Харнесс: `POST /v1/players/{id}/actions {attack wolf-alpha}` → `202` ≤ 300 мс → `player.attacked` в `player_events` → агент встречи: `dice.rolled` ×N → `combat.decided` → `entity.update.proposed` → `entity.updated` (version +1) → `narrative.output` → `Delivery` в long-poll → `ack`. **Ожидаемое:** все события одной цепочки несут `meta.correlation_id` действия; порядок «механика раньше нарратива»; `changed[].new.hp` = `combat.decided.hp_after`; ровно один `narrative.output` | P1 | I1-α, I1 |
| INT-02 | C-05 | Соло: 30 ходов → ровно 30 `narrative.output`, `recipients=[player-A]`. Группа: раунд из 3 действий → один `narrative.output` с общим `narrative_event_id`, `recipients` = 3 игрока включая `idle` | P1 | I1 / I2 |
| INT-03 | C-06 + C-04 | `enter dark-forest-01` → `POST /v1/admin/agents/{region-gm}/tick` через прокси gateway → `encounter.started` не позднее одного интервала; `look` до тика описывает регион без боя | P1 | I1 |
| INT-04 | C-06/C-07 | Фон 6 ч ускоренными тиками без игроков → `tick.fired` глобального и регионального GM, ≥ 1 `world.*`/`region.*` событие, `entity.updated`; `llm.output` фоновых агентов ≤ B; от `task`/`personal` = 0; вход игрока → сводка «пока тебя не было» отражает событие; HP игрока не изменён | P1 | I1 |
| INT-05 | C-07 | `mvctl record --scenario solo-30` на стенде (живой Ollama) → коммит записи → прогон `--mode=replay` в CI → последовательность доменных событий побайтово идентична; `llm.output` новых = 0; удаление одной записи → «запись неполная», а не тихий вызов модели | P1 | I1 |
| INT-06 | C-14 | 10 ходов → остановка процесса → старт: `state` догоняет журнал от `state/latest.json` до `End` → `analytics.replay.completed mode=recovery identical=true, llm_calls=0, dice_rolled_new=0` → `swarm` и `gateway` строят проекции от своих `latest.json` и догоняют `entity.updated`; ход 11 принимается; **I2:** открытый раунд восстановлен, `round.closed` не продублирован | P1 | I1 / I2 |
| INT-07 | C-02 (atomic) + C-04 (раунд) + C-05 | Группа из 3: одновременные удары → `round.closed acted[]` по времени приёма → пакет `atomic=true` → три `entity.updated` по версиям; конфликт версии → `version_conflict` + повтор; `group.entered_region` меняет позицию всех `alive` одним пакетом; смерть лидера → `group.leader_changed cause=death`; молчащий → `defend`, после двух пропусков → `idle` и не выбирается целью | P1 | I2 |
| INT-08 | C-09 | Память остановлена → `journalContext`, нарратив без ошибки игроку, `memory_fallback` в логе, задержка в пределах NFR-002. Память поднята → `/v1/context/scope` возвращает факт `authored` из блупринта выше противоречащего `generated`; сводка `absence` ⊇ события журнала | P2 | I2 |
| INT-09 | C-10 | Прогон S1/S2 → парные `analytics.session.started/ended`; `turn.completed` на каждый ход, включая `status=rejected`; `mvctl session-report --json` даёт латентности и вызовы LLM; `--audit` без `state_divergence`; в `analytics_events` 0 текста и внешних ID; повторный replay игнорирует `analytics.*` | P1 | I1 / I2 |
| INT-10 | C-01 | На реальной Redpanda: публикация невалидного payload → `dead_letters` без вызова handler; после зелёного прогона `dead_letters` пуст; `player_events` не принимает событие с `meta.agent` | P2 | I1 |
| INT-11 | C-11/C-12 | `mvctl laws bump` → `world.laws.changed` → ответ агента со старой `laws_version` отклонён (`law_violation`), один повтор с новой версией, второй провал → шаблон; `laws_version` в 100 % `llm.output`/`narrative.output`/`snapshot.created` | P2 | I1 |
| INT-12 | C-05/C-08 | Доставки строго по порядку на `player_id`, одна в лизинге; без `ack` — повтор через 30 с; дубль события не даёт второго сообщения; бот офлайн → доставка после восстановления, действие не потеряно | P2 | I1-α, I1 |
| INT-13 | C-03 + фикстуры | Каждая запись `npc_table` блупринта региона имеет `stats_ref` в `rules/dark-forest.yaml` и тип сущности из `testdata/fixtures`; после TTL убитого волка — `npc.spawned` с новым `entity_id`; до TTL респауна нет | P2 | I1 |
| INT-14 | S5 / US-019 | `MV_GM_PATH=agent`: `gm_instances_active = 0`, `gm_path=agent` в 100 % `llm.output` и `analytics.turn.completed`; переключение флага без пересборки; после удаления флага (последняя задача 003 I2) — `legacy_gm_path_share = 0` и профиль `legacy` в `services/_archive/` | P1 | I2 |

### 5.4. Проверка заглушек против реальных поставщиков

Обязательный шаг перед закрытием интеграции волны: для каждой пары из `contracts.md` §17 прогнать **один и тот же CT-набор** на заглушке и на реализации и приложить дифф результатов (должен быть пуст):

| Заглушка | Реальный поставщик | CT | Когда |
|---|---|---|---|
| `testkit/membus` | kafka-адаптер (testcontainers) | CT-01 | до старта волны 1 (F-5t) |
| `testkit/state.FakeState` | `internal/state` | CT-03 | I1 (после слияния 002) |
| `testkit/mechanics.FixedMechanics` | `mechanics.Rules` | CT-04 | I1 |
| `testkit/gateway.Harness` v0 | `internal/gateway` + бот | CT-05, CT-08 | I1-α |
| `testkit/gateway.FakeGateway` | `internal/gateway` | CT-08 | I1-α |
| `testkit/swarm.FakeNarrator` (+`WithEncounterStub`, `FakeEncounter`) | рой `internal/swarm` | CT-06 | I1 (и удаление `WithEncounterStub` — критерий I1) |
| `journalContext` | `internal/memory` | CT-09 | I2 |
| `providers/fake`, `providers/recorded` | `providers/ollama` | CT-14 + ST-04 | I1 |

### 5.5. Критерии выхода из интеграции волны

1. Все кейсы волны из §5.2 выполнены; P1 — 100 % пройдены, P2 — ≥ 90 % и ни один открытый P2 не связан с потерей/искажением данных.
2. Нет открытых дефектов Blocker/Critical/Major по кейсам волны (§7).
3. Все пары «заглушка ↔ реализация» из §5.4, актуальные для волны, дали пустой дифф; неактуальные заглушки удалены (`WithEncounterStub` — критерий I1).
4. `make ci` зелёный на `integration/mvp-1`; `dead_letters` пуст; `invariant_violations = 0`; `world_break_count = 0`.
5. `security-review.md` эпиков волны без открытых Critical/Major (`threat-model.md` §6).
6. `session-report` по прогону приложен (CSV + JSON); для I1 и I2 — свежий `baseline.md` с замерами волны.
7. Регрессионный набор (§7.4) прогнан целиком и зелёный.

---

## 6. Критерии выхода из тестирования (этап 6 flow)

### 6.1. Общие (для инициативы и для MVP-1)

1. Все кейсы тест-плана выполнены (выполнен = пройден, провален или обоснованно отменён с записью причины); **P1 — 100 % пройдены**.
2. **0 открытых дефектов Blocker/Critical/Major**; Minor — с заведённым техдолгом и решением владельца эпика.
3. Автоматические кейсы включены в CI и зелёные на ветке ≥ 2 прогонов подряд (защита от флаки).
4. Покрытие ядра `internal/{state,mechanics,swarm,llm,replay}` ≥ 60 % (NFR-064).
5. `security-review.md` инициативы заполнен, Critical/Major закрыты.
6. NFR-пороги «после замера» либо зафиксированы (§6.3), либо явно перенесены с записью в `journal.md` и решением tech-lead#1.

### 6.2. Дополнительно для G4 (MVP-1)

- S1, S2, S3, S5, S6, S8, S9, S10, S14 подтверждены прогонами; S4 — измерен и пороги зафиксированы; **S7 — не критерий выхода тестирования** (наблюдение 4 недели после G4, ведётся `session-report --weekly` + `incidents.csv`).
- Выполнены пункты 1–9 «Проверки перед релизом MVP-1» (`threat-model.md` §7) — ответственный security-engineer.
- `privacy-scan` зелёный **на живом стенде**, не только в e2e.
- Записи `testdata/recordings/*.jsonl` и golden соответствуют текущим промптам (последнее обновление — осознанной задачей с ревью диффа).

### 6.3. Как фиксируются пороги «после замера»

1. Замер выполняется на стенде по протоколу `metrics.md` §7 (**B1…B8**), ≥ 3 прогона каждый, при обеих загруженных моделях и включённых фоновых тиках.
2. Результат — `ops/metrics/baseline.md` (дата, драйвер, модели, режимы, сырые числа). Владелец файла — architect#1, исполнитель — человек на стенде.
3. Architect#1 предлагает пороги; business-analyst вносит их в `nfr.md` (версия +1), заменяя пометку «после замера» на число и ссылку на прогон в `baseline.md`.
4. Только после шага 3 соответствующий NFR становится **критерием приёмки**; до этого провал по порогу — наблюдение, не дефект (регистрируется как Minor/наблюдение).
5. Соответствие B → NFR: B1 → NFR-001/002 и S4; B2 → NFR-004; B3 → NFR-022; B4 → NFR-090; B5 → NFR-070/073; B6 → NFR-010 и S3; B7 → NFR-006/052; B8 → NFR-007/053 и параметры S14.
6. Повторный замер обязателен, если сменилась модель, `num_ctx`, версия Ollama или драйвера — с новой записью в `baseline.md`.

---

## 7. Дефекты

### 7.1. Формат

```
BUG-<NNN> · <краткая суть в одну строку>
Серьёзность: Blocker | Critical | Major | Minor · Приоритет: P1|P2|P3
Найден: <ID кейса> · Волна/инкремент: <wave-0|i1-alpha|i1|i2> · Ветка/прогон: <ссылка на CI или тег>
Эпик-владелец: EPIC-00N (по plan/ownership.md) · Команда: TEAM-N
Окружение: CI (unit|integration|e2e) | стенд (модели, GPU) | integration/mvp-1
Предусловие: <фикстуры, seed, запись>
Шаги: 1) … 2) … 3) …
Ожидаемое: <со ссылкой на критерий US/NFR/контракт C-NN>
Фактическое: <что произошло; фрагмент события/лога без ПДн>
Артефакты: correlation_id, session-report JSON, ссылка на прогон
Почему не поймали раньше: <уровень, на котором должен был отсечься>
Регрессионный кейс: <ID нового или дополненного кейса>
```

### 7.2. Серьёзность

| Уровень | Определение (для этого проекта) | Пример |
|---|---|---|
| **Blocker** | нельзя продолжать тестирование волны; ветка не собирается или сценарий не запускается | `make ci` красный, e2e не стартует, contract-тест шины падает |
| **Critical** | потеря или искажение данных, поломка мира, утечка ПДн, нарушение инварианта, недетерминизм replay | расхождение снапшота и журнала, внешний ID в шине, `identical=false`, двойное применение урона |
| **Major** | ключевой критерий приёмки US Must не выполняется, но обходной путь есть; нарушение контракта `C-NN` без потери данных | нарратив не доходит до части группы, `409` вместо `202`, `laws_version` отсутствует в части событий |
| **Minor** | косметика, неудобство, неточный текст, флаки-тест без влияния на данные | формулировка ошибки бота, лишний лог, нестабильный тайминг в тесте |

Отдельно: **наблюдение** — превышение порога, ещё не зафиксированного замером (§6.3); в дефекты не попадает, идёт в отчёт волны.

### 7.3. Поток

1. **tester#N** (или CI) находит → заводит `BUG-NNN` по §7.1 → определяет эпик-владельца по карте владения → передаёт `tech-lead` этой команды.
2. **tech-lead#N** заводит задачу `T-NNN` в своём эпике (дефект интеграции — тоже задачей в эпик, не правкой в интеграционной ветке) и назначает **developer#K**.
3. **developer#K** чинит **вместе с тестом**, воспроизводящим дефект (обязательное условие приёмки задачи), на уровне, который должен был отсечь дефект.
4. **code-reviewer#N** (не ревьюит задачу «своего» разработчика) проверяет исправление и наличие теста.
5. **tester#N** перепроверяет исходный кейс + новый регрессионный кейс; закрывает `BUG-NNN`.
6. Конфликт в `shared/*`, `schemas/` или контракте — только через **system-architect** (`ownership.md` §2); обе команды получают задачи.
7. **Minor** без исправления в волне → строка техдолга в `epics/EPIC-00N/tasks.md` с решением tech-lead#N; не блокирует выход, но перечисляется в отчёте волны.

### 7.4. Регрессионный набор и его пополнение

- Файл `testing/regression.md` — список ID кейсов обязательного прогона: **все CT**, **все P1 E2E**, **все P1 INT текущей и предыдущих волн**, стендовый минимум (ST-01, ST-03, ST-09, ST-11).
- После каждого дефекта Critical/Major: (а) кейс, воспроизводящий дефект, добавляется в набор с пометкой `from BUG-NNN`; (б) qa-engineer#1 отвечает на вопрос «почему не поймали» и указывает уровень, на который добавлен тест; (в) если пробел системный (класс дефектов, а не один случай) — правка §1.3/§3 этой стратегии с новой версией и записью в `journal.md`.
- Прогон набора: полностью — перед каждым тегом (`mvp-1/i1-alpha`, `mvp-1/i1`, `mvp-1/i2`) и перед G4; частично (CT + P1 E2E) — на каждом PR в `integration/mvp-1` (это и есть обязательный CI).
- Флаки-кейс (нестабильный без изменения кода) считается дефектом теста уровня Minor и чинится владельцем теста; при двух подряд флаках — временно `t.Skip` с задачей и записью в отчёт волны, но не более одного инкремента.

---

## 8. Тест-планы инициатив

### 8.1. Какие нужны

| Файл | Кто пишет | Когда | Что покрывает |
|---|---|---|---|
| `epics/EPIC-001-foundation/test-plan.md` | qa-engineer#1 | волна 0 | CT-01, CT-02, CT-15, IT-01…IT-03, E2E-14, job-проверки CI, ST-01, ST-11 |
| `epics/EPIC-002-state-mechanics/test-plan.md` | qa-engineer#1 | I1 и I2 (два раздела) | CT-03, CT-04, CT-13, IT-04, E2E-01/03/05/13, вклад в INT-06/07 |
| `epics/EPIC-003-swarm-llm-laws/test-plan.md` | qa-engineer#2 | I1a, I1b, I2 (три раздела) | CT-06, CT-07, CT-11, CT-12, CT-14, IT-05, IT-08, E2E-04/07/09/12/15/18/20, ST-04, ST-07, ST-12, ST-16 |
| `epics/EPIC-004-gateway-bot/test-plan.md` | qa-engineer#3 | I1 и I2 | CT-05, CT-08, CT-10, IT-06, IT-07, E2E-02/08/10/11/19, ST-02, ST-03, ST-05, ST-06, ST-14 |
| `epics/EPIC-005-memory-ops/test-plan.md` | qa-engineer#1 | 005-ops и 005-memory (два раздела) | CT-09, IT-09, IT-10, E2E-16/17, ST-13; golden-эталоны |
| `testing/integration/test-plan-integration-i1-alpha.md` | qa-engineer#1 | перед слиянием 004 I1 | INT-01, INT-09, INT-12 + §5.4 (Harness/FakeGateway) |
| `testing/integration/test-plan-integration-i1.md` | qa-engineer#1 | перед слиянием 003 I1b | INT-01…INT-06, INT-09…INT-13 + §5.4 (FakeNarrator, FakeState, FixedMechanics) |
| `testing/integration/test-plan-integration-i2.md` | qa-engineer#1 | перед волной 2 | INT-06 (раунды), INT-07, INT-08, INT-14 + §5.4 (journalContext) |
| `testing/regression.md` | qa-engineer#1 | с I1-α, далее пополняется | §7.4 |
| `epics/*/security-review.md` | security-engineer | этап тестирования эпика | `threat-model.md` §6 — здесь не дублируется |

**Отклонение от шаблона `team-process`:** вместо `epics/*/test-plan-integration.md` (по одному на эпик) заводится **один интеграционный план на волну** в `testing/integration/` — интеграция в этом проекте идёт волнами по трём эпикам сразу, план на эпик дублировал бы одни и те же сквозные сценарии трижды. Решение — за tech-lead#1; при возражении планы разносятся по эпикам-владельцам контрактов волны.

### 8.2. Шаблон тест-плана инициативы

```markdown
# Тест-план: <EPIC-00N / инкремент>

Версия · дата · автор (qa-engineer#N) · статус
Основание: epics/<EPIC>/design.md, contracts.md §<C-NN>, user-stories.md <US-xxx>, nfr.md <NFR-xxx>

## 1. Область и вне области
Что проверяется в этом инкременте; что явно не проверяется и почему (ссылка на strategy.md §1.6).

## 2. Предусловия и тестовые данные
Фикстуры, записи, seed, окружение (CI / стенд), флаги env, версии образов.

## 3. Кейсы

| ID | Критерий приёмки (US/NFR/C-NN) | Тип (позитив/негатив/край) | Уровень | Шаги | Данные | Ожидаемый результат | auto/manual | P |
|---|---|---|---|---|---|---|---|---|

Обязательные классы краевых кейсов для каждого критерия:
границы (0, max, max+1), пустые значения, дубли (at-least-once),
конкурентность (одновременные действия в одном scope),
отказ зависимости (LLM / MinIO / Redpanda / память / сеть клиента),
рестарт посреди операции, повтор с тем же ключом идемпотентности.

## 4. Регрессия затронутых областей
Какие существующие кейсы прогоняем и почему (ссылки на testing/regression.md).

## 5. NFR-проверки
Ссылки на §3.2 стратегии; для порогов «после замера» — какой пункт B1–B8 их закроет.

## 6. Безопасность
Ссылка на epics/<EPIC>/security-review.md (не дублировать чек-лист).

## 7. Критерии выхода
Ссылка на strategy.md §6 + специфика инкремента.

## 8. Риски тест-плана
Что может помешать выполнить план (нет записей, нет стенда, зависимость от другого эпика).
```

---

## 9. Роли и распределение

| Роль | Ответственность в тестировании |
|---|---|
| qa-engineer#1 (TEAM-1) | эта стратегия, интеграционные планы волн, `testing/regression.md`, тест-планы EPIC-001/002/005, отчёты инкрементов |
| qa-engineer#2 (TEAM-2) | тест-план EPIC-003 (I1a/I1b/I2), чек-лист качества нарратива (ST-12) |
| qa-engineer#3 (TEAM-3) | тест-план EPIC-004 (I1/I2), приватность и лимиты |
| tester#1/#2/#3 | выполнение кейсов, ведение дефектов, `session-report` по прогонам |
| developer#N | unit- и contract-тесты своего кода, каркасы e2e своих сценариев, тест к каждому исправлению дефекта |
| code-reviewer#N | проверка наличия теста к исправлению и уровня, на котором он добавлен |
| security-engineer | `security-review.md`, §7 threat-model перед G4 |
| devops-engineer | CI-job'ы, testcontainers на раннере, `compose-lint`, стендовые проверки сети и томов |
| architect#1 + человек | замер B1–B8, `baseline.md`, решение по порогам |
| tech-lead#1 | приёмка выхода из интеграции волны, эскалация дефектов между командами |

## 10. Риски стратегии

| Риск | Реакция |
|---|---|
| Записи LLM устаревают при правке промптов → e2e падают массово | ключ записи не зависит от текста промпта; расхождение `prompt_hash` — предупреждение, не ошибка; перезапись — отдельная задача с ревью диффа нарративов |
| `membus` расходится с Redpanda → «в CI зелено, на стенде нет» | CT-01 обязателен на обеих реализациях до старта волны 1 (F-5t); IT-02/IT-03 в каждой волне |
| Один человек-ревьюер: тесты станут узким местом | обязательный набор ≤ 10 мин; стендовые задачи не занимают слот разработчика; P2/P3 кейсы разрешено прогонять пакетом на выходе волны |
| Пороги не зафиксированы к G4 (замер сдвинулся) | §6.3 п. 4: до фиксации — наблюдение, не дефект; перенос порога фиксируется в `journal.md` решением tech-lead#1 |
| 005-memory отрезана на G3/G4 | все Must-кейсы построены без памяти (`MV_MEMORY_ENABLED=false`); CT-09, IT-10, E2E-17, INT-08 помечены P3 и отрезаются вместе с эпиком (кроме деградационной части INT-08, которая остаётся на `journalContext`) |
| Профиль `legacy` нежизнеспособен → S5 не проверить сравнением | запасной критерий S5: «флаг удалён, `gm_path=agent` в 100 % `llm.output`» (INT-14 без ST-07) |
| Стенд один и он же машина разработки | стендовые прогоны планируются пакетами по волнам (ST-04 вместе с ST-01/ST-09), а не по одному |
