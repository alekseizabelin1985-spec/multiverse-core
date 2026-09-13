# Ревью EPIC-002

Формат записи: задача, номер итерации, дата, экземпляр ревьюера; границы ревью; вердикт;
проверенные пункты DoD; замечания с серьёзностью (Critical / Major / Minor / Nit) в виде
«файл:строка — суть — предлагаемая правка». Записи добавляются, старые не удаляются.
Язык — русский; код, конфиги и команды — английские.

---

## T-051 · ревью #1 · 2026-09-13 · code-reviewer#1 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-051`, ветка `task/T-051-rng-statistics`, HEAD `1c2ee7e` = HEAD
`epic/EPIC-002-state-mechanics` (коммитов вне родителя нет в обе стороны). Изменения не
закоммичены; `git diff HEAD` по отслеживаемым файлам пуст, новые файлы (untracked):

| Действие | Путь |
|---|---|
| A | `internal/mechanics/rng_stats_test.go` |
| A | `Docs/dev-team/epics/EPIC-002-state-mechanics/tasks/T-051.md` |
| A | `Docs/dev-team/epics/EPIC-002-state-mechanics/dev-log.md` |

Код пакета (`rng.go`, `formula.go`) не менялся — подтверждено. Артефакты в `Docs/` внутри
папки задачи — по прямому указанию оркестратора, замечанием не считаются.

Основание: `tasks.md` §2 «T-051» и общий DoD §1; DoD, уточнённый сверкой tech-lead#1
(карточка, «Описание и DoD», п. 1–4); `internal/mechanics/{rng.go,rng_test.go,formula.go}`;
ADR-003 п. 5, ADR-012, NFR-060, C-03.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 1 · Nit: 1.

Все три пункта уточнённого DoD выполнены с запасом (χ² по каждому индексу и общий, Пирсон
по шести парам), помощники статистики закреплены ручными примерами, тесты детерминированы и
быстры. Единственное содержательное замечание — комментарий теста корреляции обещает
независимость, а Пирсон ловит только линейную зависимость; это доказано мутантом ревьюера
(P11), который не краснит ни один тест пакета. DoD tech-lead#1 требует именно Пирсона, так
что это не возврат, а правка формулировки и предложение в бэклог.

### Проверенные пункты DoD

| # | Пункт | Как проверено | Результат |
|---|---|---|---|
| 1 | 1000 причин × 4 броска, два прохода в разном порядке, 100 % | чтение `rng_stats_test.go:39-76`: проход 1 — причины 0→999, индексы 0→3 на `forward`; проход 2 — индексы 3→0, причины 999→0 на отдельном `backward`; сравнивается весь `Roll` (seed, result, natural, formula, purpose); `total == 4000 && matched == total` | выполнено |
| 2 | χ² d20, n ≥ 2000, df = 19, порог 43,82 | `rng_stats_test.go:81-118`: 20 корзин (1..20), ожидаемая частота n/20, df = 19; χ²₀.₉₉₉(19) = 43,82 — верно; n = 2500 на индекс, 10000 общий; границы грани проверяются. Прогон: 22,45 / 15,14 / 29,28 / 20,10, общий 13,69 | выполнено |
| 3 | Пирсон индексов 0 и 1, 1000 причин, \|r\| < 0,1 | `rng_stats_test.go:125-155`, формула `pearson` (`:205-221`) — выборочный r, NaN при нулевой дисперсии трактуется как провал. Прогон: r(0,1) = +0,0349, прочие пары от −0,0127 до +0,0182. Порог 0,1 ≈ 3,2 SE (1/√1000) — обоснование в комментарии верно | выполнено |
| 4 | Векторы inline | `TestSeedVectors`, `TestGoldenRolls` (T-015) без изменений | выполнено (разрешение оркестратора) |
| — | Формулы помощников | `TestStatisticsHelpers`: χ² {20,0,0,0} = 45 + 15 = 60; r(1..5; 2,1,4,3,5) = 8/√(10·10) = 0,8 — пересчитано вручную, верно | верно |
| — | Детерминизм | причины — фиксированные строки `statCause`; `time.*`, глобального состояния, `t.Parallel`, итерации по map нет; SHA-256 + PCG без состояния между бросками | детерминировано |
| — | Время `-short` | `go test -short -count=1 -v -run '<4 теста>'`: 0,00 / 0,01 / 0,00 / 0,00 s; пакет целиком ~1 s | не утяжеляет |
| — | Карта владения | только `internal/mechanics/**` (EPIC-002) и артефакты эпика | не нарушена |

Прогоны ревьюера (go1.26.8 windows/amd64, golangci-lint 2.13.2):

- `go vet ./internal/mechanics/...` — exit 0;
- `go test -short -count=1 ./internal/mechanics/...` — ok;
- `go test -short -count=1 -cover ./internal/mechanics/` — 94,9 % (совпадает с карточкой);
- `gofmt -l internal/mechanics` — пусто;
- `golangci-lint run ./internal/mechanics/...` — 0 issues.

### Мутанты ревьюера

Копия нужной части дерева (`go.mod`, `go.sum`, `internal/mechanics`, зависимости
`shared/{clock,jsonpath,eventbus,entity,contracts}`, `schemas`, `rules`) в scratch, без
`-overlay`; базовый прогон копии зелёный. Замена скриптом с `assert count == 1`, байтовое
восстановление с `assert`, после серии `cmp rng.go` с рабочей папкой — равны. Копия удалена
по сохранённому точному пути.

| # | Мутант (`rng.go`) | Результат |
|---|---|---|
| M0 | контрольный, поведенческий: `rand.NewPCG(seed, 0)` → `rand.NewPCG(seed, 1)` | **красный** (`TestGoldenRolls`) — копия тестируется, изменения поведения видны; новые статистические тесты зелёные, как и должны при другом, но честном потоке |
| P11 | в `Roll`: для нечётного индекса и формулы `d20` результат = `(prev·11) mod 20 + 1`, где `prev` — d20 предыдущего индекса той же причины. Индекс 1 — функция индекса 0 (перестановка граней, маргиналы остаются равномерными) | **зелёный весь пакет**: χ² проходит, r(0,1) = −0,0772, r(2,3) = −0,0372 — под порогом 0,1; `TestRollIndicesDiffer` не срабатывает |

Мутанты исполнителя (D1–D3, C1, C2, S1) убедительны: каждый бьёт в свой тест и показывает,
чего не ловили прежние (`TestRollDistribution`, `TestRollIndicesDiffer`). P11 дополняет их
границей метода, а не дефектом реализации.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| 1 | Minor | `internal/mechanics/rng_stats_test.go:120-124`; `:9-11` | Комментарий утверждает, что «the d20 at one index says nothing about the d20 at another» и что броски разных индексов «do not follow each other». Пирсон проверяет только линейную зависимость: мутант P11 (индекс 1 — детерминированная перестановка индекса 0) проходит весь пакет с r(0,1) = −0,077. Тест соответствует DoD, но обещает больше, чем проверяет, — следующий читатель сочтёт независимость доказанной. | Сузить формулировку до «no linear dependence (Pearson)» и сослаться на ограничение; полноценную проверку независимости — в бэклог (см. предложение 1 ниже). | открыто |
| 2 | Nit | `internal/mechanics/rng_stats_test.go:13-16`, `:78-80` | «The thresholds … are the point past which the generator is called broken» верно только для текущего набора причин. При смене префикса/числа причин (допустимая правка теста) честный генератор даёт ложный красный с вероятностью ≈ 1–1,5 % (5 проверок χ² при p = 0,001 и 6 проверок \|r\| ≥ 0,1 при p ≈ 0,0015 каждая); тот, кто получит красный после переименования `statCause`, пойдёт «чинить» генератор. | Одна фраза: при смене выборки красный с небольшой вероятностью возможен и на исправном генераторе — сначала проверить соседнюю выборку, а не генератор. | открыто |

Карточка `tasks/T-051.md` и запись в `dev-log.md` точны: фактические χ² и r, покрытие,
отсутствие `-race` (Makefile, ОВ-5), время тестов и состав файлов подтверждены прогонами
ревьюера; расхождений не найдено.

### Предложения в бэклог

1. Проверка независимости индексов, не ограниченная линейной: χ² по таблице сопряжённости
   d20×d20 для пары 0–1 (400 клеток, df = 361, n ≥ 2000 даёт ожидаемую частоту ≥ 5) или
   более дешёвый вариант — χ² по разности `(r1 − r0) mod 20` (df = 19, тот же порог 43,82;
   ловит P11 и любые сдвиги/перестановки вида «r1 = f(r0)»). Размер XS, `internal/mechanics`.

---

## T-052 · ревью #1 · 2026-09-13 · code-reviewer#1 (TEAM-3, помощь TEAM-1)

### Границы ревью

Папка `.worktrees/T-052`, ветка `task/T-052-schemas-fixtures`, HEAD `8768980` = HEAD
`epic/EPIC-002-state-mechanics` (коммитов вне родителя нет). Изменения не закоммичены (по
указанию оркестратора):

| Действие | Путь |
|---|---|
| M | `schemas/events/snapshot.created.v1.json`, `schemas/events/analytics.replay.completed.v1.json` — шаблон хеша и описание |
| M | `shared/contracts/validate_test.go` (EPIC-001) — четыре литерала хеша |
| M | `testdata/fixtures/events/README.md` |
| A | `test/fixtures/state_schemas_test.go` |
| A | `testdata/fixtures/events/<8 типов>.v1.{valid,invalid}.json` — 16 файлов |
| M | `Docs/dev-team/epics/EPIC-002-state-mechanics/{dev-log.md,tasks/T-052.md}` — артефакты по указанию оркестратора, замечанием не считаются |

Основание: `tasks.md` §T-052 (с уточнениями сверки 2026-09-13); `contracts.md` C-02 v1.4,
C-03 v1.2, C-14 v1.2; `api-contracts.md` §2.3.4, §2.3.5, §2.3.12, §2.3.14; `ownership.md`
строка `analytics.replay.completed`; `state-and-mechanics.md` §4.4; решение
system-architect#1 (ревизия 4, п. 3; `journal.md` develop 2026-09-13 и C-07 v1.3 в T-444:
«для `state_hash` в схемах EPIC-002 шаблон добавляет владелец»); `shared/entity/hash.go`;
образец T-214/T-215 — `test/fixtures/events_test.go`.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 2 · Nit: 3.

DoD выполнен по всем пунктам. Каждая невалидная фикстура ломает ровно одно правило. Ревьюер
проверил это на всех восьми, а не на выборке: одна ошибка валидатора, и после починки
названного нарушения документ валиден. Шаблон хеша совпадает с выводом `entity.StateHash`.
Производителей хеша в старой форме в дереве нет. Новый тест не дублирует `events_test.go` и
доказателен: контрольный мутант и оба мутанта ревьюера красные. Два замечания Minor
касаются смысла, а не формы. Валидная фикстура `analytics.replay.completed` показывает
сочетание полей, которое §4.4 называет ненормальным. README и комментарий теста
приписывают EPIC-005 два поля, которых карта владения ему не отдаёт. Возврата это не
требует: исправляется в той же задаче до слияния.

### Проверенные пункты DoD

| # | Пункт | Как проверено | Результат |
|---|---|---|---|
| 1 | Фикстуры для всех типов EPIC-002 из реестра | `registry.go:53-79` — ровно восемь `OwnerState`; в каталоге 70 файлов и 35 типов, как в README:11; `TestEventFixturesComeInPairs`/`…BelongToRegisteredTypes` зелёные | выполнено |
| 2 | Валидная доходит до обработчика, невалидная паркуется и проходит при `Lenient()` | существующий `events_test.go` без правок, прогон `go test ./test/fixtures/...` — ok | выполнено |
| 3 | Ровно одно нарушение у каждой невалидной, названо в README | зонд ревьюера в копии дерева (все 8 типов): вывод `contracts.Validate` содержит одну причину (`missing property 'attributes'` / `'proposal_id'` / `'changed'`, `/version: value must be 1`, `/reason: value must be one of …`, `/roll/seed: got number, want string`, `/snapshot/state_hash: does not match pattern`, `/mode: value must be one of 'recovery', 'test'`). Починка названного нарушения (добавить поле, `version: 1`, `reason: law_violation`, seed строкой, префикс `sha256:`, `mode: recovery`) делает документ валидным. Строки README:130-137 совпадают | выполнено |
| 4 | Примеры `api-contracts.md` | §2.3.4 `entity.update.proposed` перенесён в валидную фикстуру по форме, с конкретными id; §2.3.4 created/updated/rejected/create.proposed, §2.3.5 (`seed` строкой), §2.3.12 (`component`, поля `snapshot{}`), §2.3.14 — сверены с фикстурами и схемами | выполнено |
| 5 | `reason` — семь значений C-02 v1.4, без `invariant`; FR-034 | `entity.update.rejected.v1.json:12`, `TestStateRejectionReasonsAreTheSevenOfC02` | выполнено |
| 6 | Поля `mode=test` и `events_hash_match` не удалены и не переименованы | `analytics.replay.completed.v1.json` — `mode` enum и `events_hash_match` не тронуты; `TestReplayKeepsTheFieldsOfModeTest` | выполнено (см. Minor 2 о формулировке) |
| 7 | Запись «C-02 v1.3/v1.4 проверяет State» | README:71-91, комментарий `state_schemas_test.go:16-21`, карточка | выполнено |
| 8 | Шаблон хеша = `entity.StateHash` | `hash.go:63` — `"sha256:" + hex.EncodeToString` (нижний регистр, 64 символа) ⇔ `^sha256:[0-9a-f]{64}$`; вторая половина `TestStateHashesCarryTheOneForm` проверяет это на `StateHash(nil)` и на мире из одной сущности | выполнено |
| 9 | Сужение не ломает производителей | grep `state_hash`/`StateHash`/`hash_before`/`hash_after` по `shared/ cmd/ internal/ test/ testdata/` (и по всему дереву вне `Docs/`, `services/`). Хеш пишут только `testkit/state` (`snapshot.go:122`, `state.go:306`), и оба раза через `entity.StateHash`. `testdata/fixtures/snapshots/state/{latest.json,20260101T000000Z-000000.json}` уже несут `sha256:0046…c29e`. Старая форма была только в четырёх литералах `validate_test.go`, их исправили. Прогон `shared/testkit/state`, `shared/entity` — ok. `snapshot.created` в production никто не публикует, `SourceMvctl` (EPIC-005) — ещё не издатель | не ломает |
| 10 | `mvctl privacy scan testdata/` чист | прогон | выполнено |

Прогоны ревьюера (в `.worktrees/T-052`):

- `go run ./cmd/mvctl contracts check` — `65 types, 8 topics, 58 schema files checked`, exit 0;
- `go test -short -count=1 ./shared/contracts/... ./test/fixtures/...` — ok, ok;
- `go test -short -count=1 ./shared/testkit/state/... ./shared/entity/...` — ok, ok;
- `golangci-lint run ./test/... ./shared/contracts/...` — `0 issues`;
- `go run ./cmd/mvctl privacy scan testdata/` — `no external identifiers in testdata/ (145 files read)`;
- `go vet ./test/... ./shared/contracts/...`, `gofmt -l test shared/contracts`, `git diff --check` — чисто.

### `state_schemas_test.go`: дублирование, источник словарей, доказательность

- **Не дублирует `events_test.go`.** Пары фикстур и путь через шину остаются в
  `events_test.go`. Новый файл проверяет только то, что одна фикстура удержать не может:
  перечень целиком, наличие общих полей, шаблон в трёх местах и его связь с кодом. Проверки
  словарей T-215 касаются других файлов (`llm.output*`). Помощник `schemaEnum` взят из
  `events_test.go`, а не скопирован; `schemaNode` повторяет его обход (Nit 4).
- **Словари читаются из схем.** Фактические значения берутся из файлов `schemas/events`
  (`schemaEnum`/`schemaNode`). Ожидания записаны литералами как решение контракта: сравнивать
  схему саму с собой бессмысленно. Это тот же приём, что в T-215. Второй копии тех же
  словарей в Go-коде нет: `internal/state` ещё не существует.
- **Мутанты ревьюера.** Копия дерева в scratch без `-overlay` (523 файла без `Docs/` и
  `services/`). Базовый прогон зелёный. Каждый мутант откатывался копированием файла из
  рабочей папки, `cmp` показал совпадение. Копию удалили по сохранённому точному пути.

| # | Мутант | Результат |
|---|---|---|
| M0 | контрольный: `hash.go:63` `"sha256:"` → `"sha256-"` | **красный**: `TestStateHashesCarryTheOneForm`, `TestLatestPointerDescribesSnapshotZero`, `TestSnapshotObjectHoldsTheSameWorld` — копия собирается из своих исходников, связь шаблона с кодом держится |
| R1 | `entity.update.rejected.reason`: `duplicate_entity` заменён на `invariant` (число значений то же, семь) | **красный**: `TestStateRejectionReasonsAreTheSevenOfC02` и `TestInvalidFixturesAreParkedOnRead/entity.update.rejected…` |
| R2 | шаблон только `state_hash_after` допускает заглавные hex (`[0-9a-fA-F]`) | **красный**: `TestStateHashesCarryTheOneForm`. Фикстуры этот мутант не ловят, и так задумано: невалидная пара `analytics.replay.completed` ломает `mode` |

Мутанты исполнителя M0–M14 в карточке с этим согласуются.

### Правка в чужой зоне: `shared/contracts/validate_test.go` (EPIC-001)

Правка минимальна и оправдана. Изменены только значения четырёх литералов хеша: два
в `TestPayloadExamples` (иначе тест красный) и два в `TestPayloadRejects`. В последних
двух случай снова отвергается ровно по своей причине (`component: core`, отсутствие
`incomplete_record`), а не заодно по шаблону. Код и реестр `shared/contracts` не менялись. В
`unknown snapshot component` поле `state_hash` перенесено в конец объекта — это косметика
ради длины строки. У T-445 (EPIC-001) правок в `validate_test.go` нет, конфликта нет. Правка
сделана по указанию оркестратора и записана в карточке.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| 1 | Minor | `testdata/fixtures/events/analytics.replay.completed.v1.valid.json:6,10-12` (и зеркально `…invalid.json`) | Валидная фикстура показывает `mode: recovery`, `events_replayed: 42`, `state_hash_before == state_hash_after`, `identical: true`. По `state-and-mechanics.md` §4.4 `identical = state_hash_after == snapshot.state_hash` истинно после штатной остановки при `events_replayed = 0`, а после догона фактов `identical=false` — это норма. Новое описание `state_hash_before` в схеме («state_hash of the snapshot the catch-up started from») это подтверждает. 42 переигранных факта, после которых хеш не сдвинулся, возможны только если у всех пустой `changed[]`. README:3-4 объявляет фикстуру формой, которую потребитель вправе ожидать, и потребитель получает образец неверного сочетания | Выбрать один из двух согласованных вариантов: (а) штатная остановка — `events_replayed: 0`, хеши равны, `identical: true`; (б) догон — `events_replayed: 42`, `state_hash_after` другой, `identical: false`. Невалидную пару привести к тем же значениям, чтобы она по-прежнему отличалась только `mode` | открыто |
| 2 | Minor | `testdata/fixtures/events/README.md:65-67`; `test/fixtures/state_schemas_test.go:65-68,75` | README называет `identical` и `state_hash_before` «полями `mode=test`», которые «делит EPIC-005». Комментарий теста говорит «the fields of mode=test belong to EPIC-005». Но `ownership.md:14` и C-14 отдают EPIC-005 только значение `mode=test` и `events_hash_match`, а §4.4 (диаграмма, строка 386) и UC-015 шаг 4 показывают, что `state_hash_before` и `identical` публикует State в `mode=recovery`. Вдобавок `state_hash_before` в этой же задаче получил шаблон и описание, а карточка (строка 37) пишет «правок полей EPIC-005 нет». Либо README ошибается во владении, либо правка поля EPIC-005 прошла без уведомления tech-lead#1, которого требует DoD | Переформулировать README и комментарий теста. `events_hash_match` (и значение `mode=test`) — EPIC-005. `identical` и `state_hash_before` читают оба режима, их публикует и State в `recovery`, поэтому тест охраняет все три. Если исполнитель считает эти поля совладением, записать уведомление tech-lead#1 о шаблоне у `state_hash_before` в dev-log | открыто |
| 3 | Nit | `test/fixtures/state_schemas_test.go:20` | Задачи State названы «T-055, T-056», а в README:77, :82 — «T-056», в README:86 — «T-050, T-056». Для одних и тех же правил C-02 v1.3/v1.4 это два разных набора ссылок | Свести к одному набору (по `tasks.md`: пакет `changes[]` и связка путь ↔ причина — T-056, статус в то же значение — T-050/T-056) | открыто |
| 4 | Nit | `test/fixtures/state_schemas_test.go:109-126` | `schemaNode` повторяет цикл обхода `schemaEnum` (`events_test.go:435-452`) строка в строку. Сейчас это оправдано: DoD запрещает править `events_test.go` | В бэклог: после слияния выразить `schemaEnum` через `schemaNode` (один обход на пакет) | открыто |
| 5 | Nit | `testdata/fixtures/events/README.md:135` | Сценарий невалидной `entity.updated` — «повторный `/forget`» с пустым фактом. По тексту той же C-02 v1.4 терминальная сущность (`abandoned`) получает `dead_entity`, а второй `/forget` после применения первого застаёт `abandoned`. Пустой факт при повторном `/forget` в C-02 упомянут только в обосновании «почему так», и там он противоречит правилу | Взять однозначный сценарий хода без изменений (например, `set status alive` у живой сущности в бою, `cause: combat`) или оставить как есть и передать противоречие C-02 system-architect (см. бэклог) | открыто |

### Оценка открытых вопросов исполнителя (не решение)

1. **`proposal_id` у `entity.create.proposed` опционален.** Оставить опциональным в рамках
   T-052 — верно: сужение несовместимо, и решение принадлежит владельцу контракта. Но вопрос
   серьёзнее, чем «не назван в §2.3.4». `entity.update.rejected` требует `proposal_id`, а
   C-02 гарантирует гашение повторов по нему. Предложение создать сущность без `proposal_id`
   нельзя ни отвергнуть валидно, ни погасить повторно. `testkit/state` обходит это
   подстановкой `event.id` (`shared/testkit/state/apply.go:400-411`). Этот обход нигде не
   записан в контракте, и T-055 может его не повторить. Рекомендация system-architect: до
   первого настоящего издателя (gateway `POST /v1/characters`, `mvctl world init`) либо
   сделать поле обязательным (издателей в production нет, сужение сейчас дешевле всего),
   либо записать в C-02 правило «нет `proposal_id` → `event.id`».
2. **Форма `changed[]` для `remove` (`new: null`).** Вопрос обоснован, и это пробел, который
   T-055 встретит в первую очередь. `entity.Change` (`shared/entity/ops.go:37-41`, `:416`)
   сериализует `old` и `new` без `omitempty`. Для `remove` получается
   `{path, old: X, new: null}`, на проводе это неотличимо от `set path null`. Для `append`
   получается `old: null`, а C-02 и описание схемы говорят «`old` отсутствует». Схема
   принимает обе формы. §4.4 восстанавливает состояние правилом «записать `changed[].new`
   по путям». Если применить его буквально, `remove` превратится в запись `null`. Ключ со
   значением `null` и отсутствующий ключ дают разный `CanonicalJSON`. Итог — после догона
   `state_hash_after` расходится с хешем живого мира, а `inventory[n]` оставляет null-элемент
   вместо сдвига. Рекомендация system-architect до T-055: зафиксировать форму на проводе
   (например, `new` отсутствует у `remove` зеркально `old` у `append`, или поле `op` в
   элементе `changed[]`). Затем привести к ней `entity.Change`, схему (`required`) и правило
   переигрывания §4.4.

### Предложения в бэклог

1. C-02 v1.4: противоречие между «статус в то же значение — пустой факт, пример — второй
   `/forget`» и «терминальная сущность → `dead_entity`». Уточнить пример в обосновании
   (system-architect).
2. `test/fixtures`: `schemaEnum` через `schemaNode` после слияния T-052 (XS, владелец —
   EPIC-003/T-214 или общий файл).
3. Вопрос 2 выше как задача EPIC-002 перед T-055 (метка `contract-change`): форма
   `changed[]` для `append`/`remove` и правило переигрывания.
4. Предложения исполнителя поддерживаю: `snapshot_at` в `metrics.md`/FR-087, ссылка C-14 v1.1
   в описании `snapshot.created.v1.json`, `$defs.StateHash` в `_common.json` после T-445.

### Риски

- Текстовый конфликт `testdata/fixtures/events/README.md` с T-445 (EPIC-001). Обе ветки
  вставляют абзац сразу после строки 57 («нет (T-217)…») и абзац в разделе «Числа». Слияние
  в `develop` через разные эпики даст конфликт в двух местах; разрешается склейкой абзацев,
  смысловых противоречий нет (T-445 правит хеши записей LLM, T-052 — хеши состояния).
