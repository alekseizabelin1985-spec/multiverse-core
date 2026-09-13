# Журнал разработки EPIC-002 «Состояние и механика»

<!-- dev-log T-051 -->
## developer#1 · T-051 · RNG и формулы: добор статистических тестов · 2026-09-13

Ветка `task/T-051-rng-statistics` (от `1c2ee7e`), TEAM-1, Opus. Подробности — карточка `tasks/T-051.md`, раздел «Выполнение».
- **Что сделано**: новый `internal/mechanics/rng_stats_test.go`, код пакета не менялся.
  - `TestRollReproducibleThousandCausesTwoOrders` — 1000 причин × 4 броска хода (`d20`, `2d6+1`, `d20`, `d4`), два прохода в разном порядке на разных наборах правил, 4000 из 4000 `Roll` совпадают.
  - `TestRollD20ChiSquare` — χ² d20 по каждому индексу (n = 2500) и общий (n = 10000), порог 43,82 (df = 19, p = 0,001). Факт: 22,45 / 15,14 / 29,28 / 20,10, общий 13,69.
  - `TestRollIndicesUncorrelated` — Пирсон на 1000 причинах, |r| < 0,1 для пары 0–1 (+0,0349) и остальных пяти пар.
  - `TestStatisticsHelpers` — формулы χ² и r на ручных примерах.
- **Решения по ходу.** Броски идут через `Rules.Roll`, а не через `NewRNG` напрямую: проверяется тот же путь, по которому бросает бой и фон. Порог χ² применён к каждому индексу отдельно — смещение на одном индексе в общей сумме размывается.
- **Отклонения**: векторы `Seed` — inline (T-015), не в `testdata`; разрешено оркестратором.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути): M0 контрольный — красный (сборка); D1 (крайние грани вдвое чаще, среднее 10,5), D2 (сдвиг вверх), D3 (грань 1 → 2) — χ² красный, D1 и D2 прежний `TestRollDistribution` не ловит; C1 (индекс 1 на seed индекса 0) — r = 1, C2 (повтор у каждой пятой причины) — r = 0,23, красный, C2 прежний `TestRollIndicesDiffer` не ловит; S1 (состояние в `NewRNG`) — воспроизводимость красная (2172 из 4000).
- **Прогоны**: `go build ./... && go vet ./...` — 0; `gofmt -l internal/mechanics` — пусто; `go test -short -count=1 ./...` — 27 пакетов ok; `golangci-lint run ./internal/mechanics/...` — 0 issues; `make test` — exit 0, покрытие `internal/mechanics` 94,9 %. `-race` недоступен (нет cgo).
- Docker, интеграционные тесты и стенд `:8888` не трогались, `.env` не открывался. Не коммитил.

<!-- dev-log T-052 -->
## developer#2 · T-052 · Схемы EPIC-002: ревизия, шаблон хеша, фикстуры valid/invalid · 2026-09-13

Ветка `task/T-052-schemas-fixtures` (от `8768980`), TEAM-1, Opus. Подробности — карточка `tasks/T-052.md`, раздел «Выполнение».
- **Ревизия восьми схем** (`OwnerState` в `registry.go`; `entity.delete.proposed` в реестре нет): enum `op`, опциональный `expected_version`, семь причин отказа C-02 v1.4 без `invariant`, `changed[]` для `append`, `cause` с `forget|resolve|init|author`, `component` C-14 v1.2, `mode ∈ recovery|test`, `events_hash_match` — на месте, правок не потребовали. Примеры `api-contracts.md` §2.3.4, §2.3.5, §2.3.12 (и §2.3.14) сверены — совпадают, пример §2.3.4 перенесён в валидную фикстуру.
- **Шаблон хеша** (решение system-architect#1, ревизия 4 п. 3): `snapshot.state_hash`, `replay.state_hash_before`, `replay.state_hash_after` получили `pattern ^sha256:[0-9a-f]{64}$` — форма `entity.StateHash`. Следствие — четыре тестовых литерала хеша в `shared/contracts/validate_test.go` (EPIC-001) заменены на форму `sha256:` (по указанию оркестратора; код и реестр не менялись).
- **Фикстуры**: 16 файлов `testdata/fixtures/events/<тип>.v1.{valid,invalid}.json`; у каждой невалидной одно нарушение (сверено выводом валидатора), описано в README. `events_test.go` не менялся.
- **Тест словарей и форм**: `test/fixtures/state_schemas_test.go` — семь причин, `mode`/`component`, поля совладения с EPIC-005, шаблон у трёх хешей и его совпадение с `entity.StateHash`.
- **Записано явно** (README и карточка): C-02 v1.3 (одна сущность дважды) и v1.4 (путь ↔ причина, статус в то же значение) проверяет State, а не схема.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути): M0 контрольный — красный; M1–M14 (снятие `required`, ослабление `const`, расширение/сужение enum, снятие `type` у `seed`, снятие и ослабление шаблона хеша, удаление `events_hash_match`) — все красные.
- **Прогоны**: `mvctl contracts check` — 65 типов, 58 файлов; `go test -short -count=1 ./shared/contracts/... ./test/fixtures/...` — ok; `golangci-lint run ./test/... ./shared/contracts/...` — 0 issues; `go build ./...`, `go vet`, `gofmt -l` — чисто; `mvctl privacy scan testdata/` — чисто; `make test` — exit 0 (27 пакетов ok, `internal/mechanics` 94,9 %, без `-race`).
- **Вопросы и риски**: `proposal_id` у `entity.create.proposed` оставлен опциональным (ОВ-21) — подтвердить; T-445 правит тот же `testdata/fixtures/events/README.md` — возможен текстовый конфликт при слиянии. Docker, интеграционные тесты и стенд `:8888` не трогались, `.env` не открывался. Не коммитил.

<!-- dev-log T-052 итерация 2 -->
## developer#2 · T-052 · итерация 2 по ревью #1 (2 Minor, 3 Nit) · 2026-09-13

Подробности и ответ по каждому пункту — карточка `tasks/T-052.md`, «Итерация 2». Не коммитил.
- **Mi-1**: пара `analytics.replay.completed` приведена к согласованному сочетанию §4.4 — штатная остановка: `events_replayed: 0`, `state_hash_before == state_hash_after` (хеш снапшота seq 0), `identical: true`. Невалидная отличается от валидной одной строкой `mode: "replay"`.
- **Mi-2**: README и комментарий `TestReplayKeepsTheFieldsOfModeTest` больше не приписывают EPIC-005 `identical` и `state_hash_before`. За EPIC-005 — `events_hash_match` и значение `mode=test` (`ownership.md` §1, C-14). Строка карточки «правок полей EPIC-005 нет» заменена.
- **Уведомление tech-lead#1 и EPIC-005 (совладение `schemas/events/analytics.replay.completed.v1.json`)**: в T-052 полю `replay.state_hash_before` добавлен `pattern ^sha256:[0-9a-f]{64}$` и описание (прежде — любая строка, включая пустую); то же — `state_hash_after`. Харнесс `mode=test` (EPIC-005) обязан писать хеши в форме `entity.StateHash`, пустая строка вместо отсутствующего поля теперь невалидна — поле без снапшота опускается. `events_hash_match` и `mode` не менялись.
- **Nit 3**: ссылки сведены к одной форме — нормы C-02 v1.3/v1.4 проверяет State в T-056; `StatusTransitionAllowed(x, x)` правит T-050.
- **Nit 5**: сценарий невалидной `entity.updated` — `set status alive` у живого персонажа в бою (`cause: combat`) вместо «повторного `/forget`»; в README добавлено, что у терминальной сущности тот же `set` — `dead_entity`.
- **Nit 4** не трогался (решение оркестратора) — в бэклог: `schemaEnum` через `schemaNode`.
- **Проверки**: ровно одна ошибка валидатора у каждой из 8 невалидных фикстур (диагностический тест в копии в scratch, копия удалена по точному пути); `mvctl contracts check` — 65 типов, 58 файлов; `go test -short -count=1 ./shared/contracts/... ./test/fixtures/...` — ok; `golangci-lint run ./test/... ./shared/contracts/...` — 0 issues; `mvctl privacy scan testdata/` — чисто; `gofmt -l test`, `git diff --check` — чисто.
- Вопросы `proposal_id` и форма `changed[]` переданы оркестратором system-architect и tech-lead#1, в этой итерации не трогались. Docker, стенд `:8888`, `.env` не трогались.

<!-- dev-log T-060 -->
## developer#2 · T-060 · `internal/replay` — EventClock, NullTimers, Recording, сборка replay в `serve.go` · 2026-09-13

Ветка `task/T-060-replay-eventclock` (от `2196b58`), TEAM-1, Opus. Подробности — карточка `tasks/T-060.md`, раздел «Выполнение».
- **Пакет `internal/replay`**:
  - `EventClock` — монотонен по времени событий, настенных часов не читает.
  - `NullTimers` — таймеры не срабатывают.
  - `Cursor` — `Advance`/`Merge`/`Min`/`Clone`, детерминированный JSON.
  - `ReadToEnd`/`CatchUp` — конец журнала берётся один раз.
  - `Recording` + `Writer` — JSONL, побайтовый round-trip; `Index` по `(correlation_id, agent.id, phase, attempt)`, у каждой части ключа — её длина.
  - `Middleware` + `WithMiddleware` — в replay `Observe` до обработчика и `meta.replay=true`.
- **`cmd/multiverse/serve.go`**:
  - В replay контексты получают `EventClock` и `NullTimers`. Часы стоят на первом событии `--recording`, без записи — на нулевом времени.
  - **Шина получает `clock.RealTimers` в любом режиме** (C-01 v1.4). Прежнее нарушение исправлено и закреплено тестом: повтор ×3 → `dead_letters` без зависания.
  - На время replay-прогона стоит `eventbus.SetClock(EventClock)`, транспорт обёрнут middleware.
  - Нечитаемая запись отказывает в старте; лог старта называет запись.
- **Решения по ходу**:
  - Обёртки `ReadRange`/`Tail` из §6.1 заменены на `ReadToEnd`/`CatchUp`: обёртка без поведения была бы вторым именем метода.
  - Middleware ставится обёрткой транспорта; API `shared/eventbus` не менялся.
  - `SetClock` ставится только в replay, чтобы в live не затирать ручные часы тестов процесса. Источники live, `SetIDSource` и `SetRegistry` — за T-055.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути): M0 контрольный — красный, M1–M19 — все красные. M11 и M17 покраснели после усиления тестов: шпион диапазонов и чтение «с ошибкой на полпути».
- **Прогоны**:
  - `go build ./... && go vet ./...` — 0;
  - `go test -short -count=1 ./...` — ok;
  - `go test -tags e2e ./test/e2e/...` — ok;
  - `golangci-lint run ./...` — 0 issues;
  - `make test` — exit 0, `internal/replay` 97,3 %, без `-race` (нет cgo).
- **Для T-055**:
  - `serve.go` правлен в `process.run`: `timeOf` → `SetClock` в replay → `openBus` с `times.bus`. `SetIDSource`/`SetRegistry` ставить рядом.
  - Тесты сборки режима — в новом `cmd/multiverse/replay_test.go`; `serve_test.go` не менялся.
- Docker, интеграционные тесты и стенд `:8888` не трогались, `.env` не открывался. Не коммитил.

<!-- dev-log T-060 итерация 2 -->
## developer#2 · T-060 · итерация 2 по ревью #1 (3 Minor, 4 Nit) · 2026-09-13

Подробности и ответ по каждому пункту — карточка `tasks/T-060.md`, «Итерация 2». Не коммитил.
- **Mi-1**: процессный тест replay читает и через `Deps.Bus.Subscribe`, и проверяет, что `Deps.Bus` и `Deps.Journal` — один объект. `WithMiddleware` возвращает указатель, иначе сравнение паниковало бы. Мутант ревьюера M3 — красный.
- **Mi-2**: пакетный тест — событие, построенное `eventbus.Derive` в обработчике replay, лежит в журнале с `meta.replay=true` и временем причины. Мутант ревьюера M4 — красный; `shared/eventbus` не трогался.
- **Mi-3**: комментарий `Middleware` исправлен. Правило «время события — `ev.Timestamp`, следствие — через `Derive`; `Clock.Now()` при нескольких читателях зависит от планировщика» записано в doc пакета.
- **N-1**: `Recording.Start` — самое раннее ненулевое время, а не первая строка. **N-2**: `Writer.Close` делает `Sync`, комментарий честный. **N-3**: ошибка чтения посреди строки не маскируется ошибкой JSON. **N-4** — строка бэклога для T-055, не исправлялось.
- **Принято**: записанные события в шину никто не публикует — запись служит таблицей ответов `RecordedProvider`. Три вопроса ревьюера к system-architect записаны в карточку.
- **Мутанты** (копия в scratch, без `-overlay`, удалена по точному пути): M0 контрольный — красный; прежние M1–M19, мутанты ревьюера M3r и M4r, новые M20–M22 — красные. M21 сначала выжил, после перестановки строк в тесте — красный.
- **Прогоны**: `go build ./... && go vet ./...` — 0; `go test -short -count=1 ./...` — ok; `go test -tags e2e ./test/e2e/...` — ok; `golangci-lint run ./...` — 0 issues; `make test` — exit 0, `internal/replay` 96,8 %.
- Docker, интеграционные тесты и стенд `:8888` не трогались, `.env` не открывался.
