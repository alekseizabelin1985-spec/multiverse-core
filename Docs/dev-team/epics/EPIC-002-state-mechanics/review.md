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
# Ревью EPIC-002

Формат записи: задача, номер итерации, дата, экземпляр ревьюера; границы ревью; вердикт;
проверенные пункты DoD; замечания с серьёзностью (Critical / Major / Minor / Nit) в виде
«файл:строка — суть — предлагаемая правка». Записи добавляются, старые не удаляются.
Язык — русский; код, конфиги и команды — английские.

---

## T-050 · ревью #1 · 2026-09-13 · code-reviewer#1 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-050`, ветка `task/T-050-entity-reconcile`, HEAD `1c2ee7e`. Коммитов в ветке
нет (`git log epic/EPIC-002-state-mechanics..HEAD` пуст). Ревьюировалась **рабочая копия**
(`git diff`): `shared/entity/{types,attrs,hash,ops}.go`, `README.md`, тесты `entity_test.go`,
`attrs_test.go`, `ops_test.go`, `hash_test.go`; новые файлы — карточка `tasks/T-050.md` и `dev-log.md`.
Вне `shared/entity/**` и документов эпика правок нет, карта владения соблюдена (`tasks.md` §1 п. 6).

База ветки `1c2ee7e` отстаёт от `epic/EPIC-002-state-mechanics` (`c171104`) на один
документальный коммит. В нём `tasks.md`, `design.md` и карточки T-052…T-071. Файлов `tasks/T-050.md`
и `dev-log.md` в `c171104` нет, так что конфликта add/add при слиянии не будет. DoD сверялся
по разделу T-050 в `tasks.md` ветки эпика (`c171104`, строки 44–52) и по карточке.

Основание: C-02 v1.2–v1.4 (`architecture/contracts.md`, строки 261–284); `analysis/data-model.md` §3
(строки 117, 126, 159); `architecture/components/state-and-mechanics.md` §3.2–§3.3 (строки 162–172);
`shared/testkit/state/apply.go` (`statusRefusal`, строки 343–358); `internal/mechanics/actor.go`,
`types.go`; `rules/dark-forest.yaml`; `shared/eventbus/membus/membus.go`.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 3 · Nit: 3.

Правка C-02 v1.4 минимальна и верна. Три исправления кода закрывают реальные дефекты и не меняют
поведение для данных с шины. Золотой литерал не менялся, прогоны и мутанты подтверждены
независимо. Minor-1 — это недоделка того же класса, что исправление п. 5. Дефект существовал
до T-050, достижим только вызовами из Go в обход шины, поэтому не блокирует. Но тест и README
сейчас обещают больше, чем код делает.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | `StatusTransitionAllowed(x, x)` по C-02 v1.4 | `types.go:66-71`: сначала проверяется `IsTerminalStatus(from)` (тот же список, что у `Entity.IsTerminal()`, `types.go:42-49`), затем `to ∈ terminal ∪ {alive}`. Если `from == to`, то `to` не терминальный (иначе терминальный и `from`), значит `to == alive`. Поменялся ответ **только** для `alive→alive`. Пути, где `dead→dead`, `abandoned→abandoned` или `ascended_final→ascended_final` дают `true`, нет. `sleeping→sleeping` и `""→""` дают `false`. `FakeState` отсекает терминальную сущность раньше матрицы (`apply.go:288`), а матрицу зовёт на `changed[]` (`apply.go:343-358`). Двойник не менялся и зелёный |
| 2 | Тест связки с `ApplyOps` | `entity_test.go:257-272`: `changed` пуст, после `Commit` версия осталась 1, матрица не отказывает. Не тавтология: мутант «прежнее правило» роняет и эту строку, и строку таблицы |
| 3 | `CanonicalJSON` структуры = wire-форма | `hash.go:137-149`: вывод `json.Marshal` декодируется и пишется общим правилом. `time.Time` и строки после декодирования пишутся побайтно так же, как раньше. Вложенные структуры, `omitempty`, nil-поля внутри структуры, `*int`, `**int`, `time.Time` с зоной +03:00 совпадают с wire-формой (зонд). Золотой литерал `TestCanonicalJSONAndStateHashGolden` не менялся: diff `hash_test.go` содержит только добавления. Данные с шины приходят как `map[string]any`/`[]any`/`float64`/`string`/`bool`/`nil` и в изменённую ветку `default` не попадают. Хэши снапшотов и replay прежние. `testdata/fixtures/snapshots/state/*.json` (`state_hash` `sha256:0046b8…`) пересчитывается `test/fixtures` и зелёный. Исключения — см. Minor-1 |
| 4 | Дедуп `append` и `remove` в wire-форме | `ops.go:306-355`. Порядок сохраняется: `append` пишет в конец, `remove` копирует оставшиеся по порядку. `remove 3` из `[3.0]` соответствует `state-and-mechanics.md` §3.2 («по равенству для скаляров»): у JSON одно числовое пространство. Дедуп по `item_id` для `entity.Item` сделан как в §3.2. inv-03 («трофей за NPC ≤ 1», `lootIndex`) — проверка State, пакет её не ослабляет, повтор трофея — no-op. Путь с шины (значения `map`) по стоимости не изменился, для структур из Go — см. Nit-1 |
| 5 | `Flee()` числом | `attrs.go:161-173`. `data-model.md` §3.3 (стр. 126) — «int / формула», правила пишут `flee: 2`: расширение законно. `"-1"` и `"+2"` читает `mechanics.parseInt` (`internal/mechanics/formula.go:402`). Поведение для дробных чисел — см. Minor-2 |
| 6 | Потребители | `go test -short -count=1 ./...` — все пакеты ok, в том числе `internal/mechanics` (`ActorFromEntity`), `shared/testkit/{state,mechanics,swarm,gateway}`, `test/fixtures`. `go test -tags e2e -count=1 ./test/e2e/... ./shared/testkit/...` — ok (`test/e2e` 14,4 с, порты `127.0.0.1:0`) |
| 7 | Прогоны | go1.26.8 windows/amd64: `go build ./...` и `go vet ./...` — 0; `gofmt -l shared cmd internal test` — пусто; `golangci-lint run ./shared/...` (2.13.2) — 0 issues; `make test` — exit 0 (без `-race`, cgo нет), покрытие `shared/entity` **93,0 %** (DoD ≥ 86,9 %), гейт `internal/mechanics` 94,9 % |
| 8 | Мутанты (независимо) | Копия дерева в scratch (`git ls-files -co --exclude-standard` без `services/`, `Docs/`), без `-overlay`. Базовый прогон зелёный. Замены точные (`count == 1`), после каждой файл восстановлен побайтно, в конце `cmp` четырёх файлов с рабочей папкой — равны. Копия удалена по точному пути. **M0 (контрольный)**: `func broken( {` в `types.go` — красный (`types.go:42:14: syntax error`). **R1**: вернуть `if from == to { return false }` — красный (`alive->alive`, `TestSameStatusIsATurnWithNothingChanged`); `testkit/state` зелёный, как заявлено. **R2**: `writeCanonicalEncoded` пишет вывод кодировщика как есть — красный (`TestStateHashOfAGoBuiltWorldMatchesItsWireForm`). **R3**: `Flee` без ветки числа — красный (3 подтеста). Заявленные M1, M3, M4 подтверждены |
| 9 | Документы | Карточка и `dev-log.md` заполнены. Сверка C-02 v1.3/v1.4 для T-055/T-056 и расхождения документов для system-analyst и system-architect перечислены. Абзац C-02 «Код расходится» после слияния устареет — правит system-architect (риск ниже) |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-1 | Minor | `shared/entity/hash.go:93-113`, `:95-96`; `shared/entity/hash_test.go:132`, `:141`; `shared/entity/README.md:114-117`; карточка, п. 5 раздела «Для T-055/T-056» | Совпадение in-memory и wire-формы неполное, хотя тест говорит «Every Go shape an attribute can hold is here». Зонд ревьюера (`New` → `json.Marshal` → `json.Unmarshal` → `CanonicalJSON`) показал расхождения. Типизированный nil-слайс `[]string(nil)` в памяти пишется `[]`, после провода — `null`. `map[string]int(nil)`: `{}` против `null`. `float32(0.1)`: `0.10000000149011612` против `0.1`; в тесте `float32(1.5)` представима точно и дефект прячет. `[]byte` и `json.RawMessage`: массив байтов против base64-строки и объекта. Последствие на `ApplyOps` воспроизведено: `set players_present []string(nil)` → `Commit` → `StateHash` до и после снапшота **разный**. Повтор того же `set` после восстановления даёт `changed` из 1 элемента, в памяти — 0: версия у восстановленного State растёт, у исходного нет (NFR-061). Достижимость — как у исправленного случая со структурой: только значения из Go в обход шины (`membus` сериализует событие, `membus.go:213`, `:455`) — bootstrap, собственные ops State, прямые вызовы. Дефект был до T-050 | Вариант А (дёшево, рекомендую сейчас). В `writeCanonicalOther` для `reflect.Slice` и `reflect.Map` при `rv.IsNil()` писать `null`. Для `reflect.Float32` писать `strconv.FormatFloat(rv.Float(), 'f', -1, 32)`. `[]byte` и типы с `json.Marshaler` (`json.RawMessage`) отправлять в `writeCanonicalEncoded`. Декодер никогда не даёт типизированных nil и `float32`, поэтому золотой литерал и хэши с шины не изменятся. Добавить эти случаи в `TestStateHashOfAGoBuiltWorldMatchesItsWireForm`, `float32` — с непредставимым значением. Вариант Б: вынести в бэклог T-055/T-057 и сузить утверждения теста, README и карточки до «структуры и `time.Time`». `int64` > 2^53 после провода теряет точность — это неустранимо для JSON, только в бэклог |
| Mi-2 | Minor | `shared/entity/attrs.go:161-173`; `shared/entity/attrs_test.go:449` | Ошибку данных `Flee` по-прежнему прячет в «не бежит»: `2.5`, `true`, `uint64` > MaxInt64 (переполнение в `asInt64`) дают `"", false`. `ActorFromEntity` (`internal/mechanics/actor.go:51`) молча принимает это за отсутствие бонуса. Строка `"abc"` при этом падает громко в `mechanics.parseInt`: одна и та же ошибка данных громкая в тексте и тихая в числе. Комментарий `ActorFromEntity` («defaulting the number would turn that bug into a fight that quietly plays by different rules») требует обратного | Для числа, которое не целое, возвращать его десятичную запись (`strconv.FormatFloat(f, 'f', -1, 64)`) и `true`: формула упадёт так же, как на `"abc"`. `nil` оставить `false`. Подтест `fraction` сменит ожидание. Если это решение механики (T-053), записать в бэклог и в комментарий `Flee`, что дробь намеренно означает «нет бегства» |
| Mi-3 | Minor | `Docs/dev-team/epics/EPIC-002-state-mechanics/tasks/T-050.md:12`; `shared/entity/README.md:119-121` | Метка `contract-change` называет только `StatusTransitionAllowed(x, x)` и новый геттер. Меняются ещё два наблюдаемых поведения публичного пакета. Первое — `CanonicalJSON` для значений из Go: структура теперь пишется с сортированными ключами. README того же пакета требует проводить изменение кодировщика «только через системного архитектора». Второе — `append`/`remove` для значений из Go: дедуп и удаление структуры по id, удаление `3` из `[3.0]`. По §3.3 это исправления, но архитектор, получив ревью по метке, должен увидеть все три | Дополнить строку «Метка» карточки: «`CanonicalJSON` значений из Go (структуры — ключи отсортированы; формат wire не изменился) и сравнение элементов `append`/`remove` в wire-форме». Оркестратору — передать system-architect вместе с C-02 v1.4 |
| N-1 | Nit | `shared/entity/ops.go:348-355`, `:306-308` | `asObject(value)` вызывается один раз в `hasSameIdentity` и ещё раз в `sameIdentity` на каждый элемент списка, то есть `json.Marshal` и `json.Unmarshal` значения повторяются N раз (в `applyRemove` — тоже, `ops.go:272-277`). Бенчмарк ревьюера на списке из 1000 wire-предметов без совпадения id: `append entity.Item` — 4,9 мс/оп и 48 088 аллокаций, `append` того же предмета как `map` — 1,07 мс и 6 060. На пути с шины регрессии нет | Один раз привести значение к объекту (`bm, bok := asObject(value)`) до цикла и передавать готовую форму во внутренний `sameIdentity` |
| N-2 | Nit | `shared/entity/ops.go:350-351`; `shared/entity/README.md:75` | Комментарий «Only objects with an id are deduplicated» и строка README про `append` неточны: объект без id тоже не добавляется, если в списке уже есть канонически равный (`sameCanonical`). Так было и до T-050 (`reflect.DeepEqual`), а теперь `{at: 2}` совпадает и с `{at: 2.0}`. §3.2 этот случай не описывает | Уточнить комментарий и README («объект без id — по каноническому равенству целиком») или спросить system-analyst, нужен ли дедуп объектов без id |
| N-3 | Nit | `Docs/dev-team/epics/EPIC-002-state-mechanics/dev-log.md` (запись T-050) | DoD (сверка 2026-09-13) требует в `dev-log.md` таблицу «пункт прежнего описания → место в дереве → тест». Таблица лежит в карточке (раздел «Итог по пунктам»), `dev-log` на неё ссылается. В строке 8 нет колонки «место в дереве» (например, `Ref` — `entity.go`, `HistoryLimit` — `entity.go`) | Перенести или продублировать таблицу в `dev-log.md` с колонкой файла, либо оркестратору принять ссылку на карточку как выполнение пункта |

### Риски и допущения

- **contract-change.** DoD требует ревью system-architect. Это ревью — не его замена. После слияния
  абзац C-02 v1.4 «**Код расходится:** … Правка кода — EPIC-002 T-056» (`contracts.md:278`) устареет.
  Правит system-architect, не разработчик.
- `-race` в прогонах недоступен (нет cgo). Новые функции чистые, без горутин, риск низкий. CI гоняет
  `-race` на Linux.
- Интеграционные тесты и Docker не запускались: для `shared/entity` они не нужны.
- База ветки отстаёт от ветки эпика на `c171104`. Индекс `tasks.md` в ветке задачи старый, и сверка
  шла по версии эпика. Перед слиянием `tasks.md` в ветке задачи трогать не надо: конфликта нет,
  пока его никто не правит.
- Зонды ревьюера (nil-слайс, `float32`, `[]byte`, бенчмарк) выполнялись во временном тестовом файле
  копии. Копия удалена, в рабочую папку ничего не добавлено.

### Предложения в бэклог

1. (Mi-1, вариант Б, если его выберут) Полное совпадение `CanonicalJSON` значений из Go с
   wire-формой: типизированные nil, `float32`, `[]byte`/`json.Marshaler` — T-055/T-057.
2. `int64`/`uint64` вне ±2^53 в атрибутах: после JSON теряется точность, и хэш в памяти расходится
   с хэшем снапшота. Запретить в `jsonCompatible`, `ReasonNotJSON`, или задокументировать
   (решение system-architect).
3. Уже предложено разработчиком и поддерживаю: `Dmg()` числом (T-053/T-056), переполнение
   `asInt64(uint64)`.

---

## T-053 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-053`, ветка `task/T-053-resolve-npctarget-changes`, HEAD `8768980`.
Коммитов вне родителя нет (`git log epic/EPIC-002-state-mechanics..HEAD` пуст). Ветка эпика
с тех пор ушла вперёд на слияние T-052 (`2196b58`). Там схемы `analytics.replay.completed`
и `snapshot.created`, фикстуры событий, `test/fixtures/state_schemas_test.go` и артефакты
эпика. С кодом T-053 файлы не пересекаются, но при слиянии будет конфликт в `review.md` и
`dev-log.md`, см. «Риски».

Изменения не закоммичены (`git status`):

| Действие | Путь |
|---|---|
| M | `internal/mechanics/{resolve,target,changes,types,actor,rules,rng}.go` |
| M | `internal/mechanics/{actor_test,rules_test,dice_event_test}.go` |
| A | `internal/mechanics/{resolve_test,target_test,changes_test,changes_encounter_test}.go` |
| M | `shared/testkit/mechanics/{fixed,fixed_test,consumer_test}.go` |
| M | `Docs/dev-team/epics/EPIC-002-state-mechanics/{tasks/T-053.md,dev-log.md}` |

Артефакты в `Docs/` внутри папки задачи оркестратор разрешил, поэтому это не замечание.
`shared/testkit/swarm`, схемы и фикстуры не менялись, подтверждено. Карта владения не
нарушена: `internal/mechanics/**` и `shared/testkit/mechanics/**` принадлежат EPIC-002.

Основание ревью:
- `tasks.md` T-053: описание, DoD и сверки 2026-09-13;
- C-03 v1.2, C-05 v1.4–v1.6 (п. 1в, 1г);
- `state-and-mechanics.md` §4.6, §5.1, §5.4, §5.5, §7.1, §17;
- ADR-012 п. 4, ADR-024;
- `data-model.md` §3.3, §3.4;
- `rules/dark-forest.yaml`;
- `shared/contracts/ownership.go`;
- `shared/testkit/swarm/fake_encounter.go`: `attacked`, `fled`, `pack`, `wound`, `asEntity`.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 2 · Nit: 5.

`Resolve`, `NPCTarget` и `ChangesFor` реализованы по §5.4. Код детерминирован, чист и
покрыт тестами на границах. Семь мутантов ревьюера, кроме контрольного, убиты. Тест сверки
с двойником правда краснеет, если расхождение исчезает или появляется новое, но только для
сущностей игрока и NPC. Сущность встречи тест пропускает целиком, и Р5 им не держится
(Minor-1, мутант D3 выжил).

Незакрытый пункт DoD про `died_at` — не дефект кода, возврата он не требует (оценка ниже).
При этом приёмка tech-lead должна зафиксировать два условия:
- вопрос 1 карточки (время и версия для `ChangesFor`) и `Actor.Kind` (вопрос 4) переданы
  system-architect до слияния в ветку эпика;
- расхождение «код ↔ C-03 v1.2 ↔ §5.1» (поле `Actor.Kind`) по C-03 «Гарантии» считается
  дефектом документа, пока не выпущена C-03 v1.3.

### Проверенные пункты

| # | Что проверено | Как | Результат |
|---|---|---|---|
| 1 | Детерминизм | `resolve.go`: нет `time.*`, глобального состояния и обхода map (из `actors` только чтение по ключу). `target.go`: `slices.SortFunc` с полным порядком, ключи правил, затем `cmp.Compare(ID)`; id уникальны (повтор — ошибка), поэтому нестабильность сортировки не влияет. `changes.go`: порядок ops фиксирован кодом. `TestNPCTargetOrder` перебирает все перестановки, `TestResolveIsPureAndDeterministic` повторяет решения на новом `Rules` (200×5) | выполнено |
| 1а | Индексы бросков ↔ C-05, §5.5 и `FakeEncounter` | Удар: `hit` на idx, `damage` на idx+1 только при попадании. Ответ NPC: 2/3 (`rollNPCCounter`). Бегство: 0, свободная атака — 1/2 (`rollFlee`, `rollFreeAttack`). Совпадает с §5.5 («свободная атака … от `player.flee_attempted` (1..2)»). `compareRolls` в `changes_encounter_test.go:405` сравнивает броски на шине с бросками `Resolve` по index/formula/seed/result/natural/purpose | выполнено |
| 2 | Таблица §5.4 | Код `resolve.go:76-141`: `Fumble` побеждает, `Critical = !Fumble ∧ crit`, `Hit = !Fumble ∧ (Critical ∨ check)`, урон — `Damage(roll, crit)` (кубы × множитель), `ClampHP`, `TargetDead = HPAfter == 0`, `Loot` только у NPC. `Excluded` — `ErrInvalidTarget` (`:93`). Порог бегства — правая часть `flee.check`: 11 и 12 при 1 и 2 врагах. `FreeAttack = !success ∧ on_fail = free_attack` (`:159`). Тесты: 16 строк `TestResolveAttackTable` с гранями 1/2, 8/9, 19/20 и def 40/1, `TestResolveFleeTable` с `on_fail: none`, `TestResolveKillingBlow`, `TestResolveRefuses` (dead/abandoned/ascended/idle/out_of_combat) | выполнено |
| 2а | Мутанты ревьюера | см. ниже | 7 из 7 убиты, D3 выжил (Minor-1) |
| 3 | Смена сигнатуры `NPCTarget` | `grep "NPCTarget("` по всему дереву вне `services/`: `internal/mechanics` и `shared/testkit/mechanics`, в `test/e2e`, `cmd` и `shared/testkit/swarm` вызовов нет. Файлы с тегами проверены `go vet -tags "e2e integration" ./test/... ./shared/testkit/... ./cmd/...`, exit 0. `ErrNotImplemented` в дереве не встречается | выполнено |
| 4 | `Actor.Kind` | Совместимое дополнение: все литералы `Actor{…}` в дереве с именами полей, `go build` и `go vet` зелёные. Заполняют `ActorFromEntity` (только NPC, `actor.go:55-58`) и `Rules.Stats` (только NPC, `rules.go:348-352`). У персонажа поле пустое, поэтому `test/fixtures` (сравнение `Stats` ↔ `ActorFromEntity`) зелёный без правок. Двойники: `FixedMechanics` теперь заполняет `Outcome.Loot`. `FakeEncounter` берёт трофей из `rules.Loot(kind)` сам и `Outcome.Loot` не читает, `testkit/gateway` тоже, поэтому поведение двойников на шине не меняется. Контракт: C-03 v1.2 поле не знает — нужен v1.3 (см. вердикт) | выполнено, с условием |
| 5 | Сверка `ChangesFor` ↔ `FakeEncounter` | Разобран `changes_encounter_test.go`. Пять форм обмена гоняются на membus с настоящим `*Rules`, шпион записывает решения. Разница выбрасывается по пяти константам (`:47-64`), остальное сравнивается через JSON. Каждая константа обязана встретиться (`:394-398`). Мутант D1 (двойник перестаёт писать `died_at`/`killed_by` персонажу) краснит тест на «gap … did not occur». D2 (двойник пишет в `killed_by` id решения) краснит на «ops of wolf-alpha». **Но** сущность встречи пропускается целиком (`:342-344`), и D3 (двойник добавляет op в пакет встречи) выживает — Minor-1 | частично |
| 5а | Точность Р1–Р5 | Р1 (`expected_version`), Р2 (`died_at` NPC, `acquired_at`), Р4 (`position`) — точны и держатся тестом. Р5 точен по существу: `FakeEncounter` держит `last_damager` только в представлении (`fake_encounter.go:838`, `:1981-1983` в `asEntity` для `ActorFromEntity`), `turnOps` (`:998-1019`) пишет лишь `round_seq`/`participants`/`state`/`resolution`/`closed_by_event_id`. Тестом Р5 не держится, хотя карточка (`T-053.md:35`) и `dev-log.md:34` пишут «каждое проверяется тестом» | Р5 — Minor-1 |
| 5б | Р3: дефект двойника или норма? | `shared/contracts/ownership.go:83-86`: у `task`/`player` пути `hp, status, position, inventory, encounter_id`, пути `died_at` и `killed_by` отсутствуют. У `task`/`npc` (`:89-92`) они есть. `data-model.md` §3.3 у персонажа этих атрибутов не знает, §3.4 даёт их только NPC, строка «Механика…» в §4 (`data-model.md:197`) называет у Character `hp/status/position/inventory`. `FakeState` владение не проверяет (`testkit/state/state.go:21`), поэтому двойник зелёный. **Р3 — дефект двойника EPIC-003**: настоящий State ответит `level_violation`, и по C-05 v1.5 п. 1е пакет смерти персонажа будет выброшен. `ChangesFor` здесь прав | дефект двойника |
| 6 | Пункт DoD `died_at` | Оценка ниже | принять с вопросом |
| 7 | `rest` кубами — ошибка | `rules/dark-forest.yaml` использует `restore: hp_max`, прогон зелёный. Существующие тесты не ломаются: `formula_test.go:358` проверяет `Rules.Restore` с `restore: d4`, `Load` такой файл по-прежнему принимает, отказ только в `Resolve(rest)` (`resolve.go:171-175`). Остаток — Nit-5 | не ломает |
| 8 | Прогоны | см. ниже | зелёные |

**Оценка пункта DoD `died_at` (п. 6).** Считаю приемлемым принять с вопросом к архитектору,
возврат не нужен. Причины:
- В C-03 v1.2 у `ChangesFor(a, o, attacker, target, factEventID)` нет ни времени, ни
  версии. Без нарушения гарантии «без часов» (C-03, §5.1) `died_at` взять неоткуда. Вывод
  времени из id события — неявный контракт на формат id, и в replay-режиме это хуже
  явного поля.
- Отклонение названо в карточке и в `dev-log.md`, варианты с рекомендацией даны. Вариант (а) —
  `Action.At`, `Actor.Version` — совместим и закрывает Р1/Р2 одним дополнением.
- Рабочий потребитель пакета сегодня — `FakeEncounter`, и он пишет `died_at` сам. Регресса
  на шине нет, а `ChangesFor` до EPIC-003 T-230 никто не вызывает.

Условие приёмки: tech-lead#1 помечает п. 5 DoD как «частично, блокировано C-03». Вопрос 1
уходит system-architect. Дополнение (`Action.At`/`Actor.Version` и выставление `died_at`,
`acquired_at`, `expected_version` в `ChangesFor`) заводится отдельной задачей EPIC-002 и
закрывается до того, как агент встречи EPIC-003 перейдёт на `ChangesFor`. Без такой задачи
пункт DoD потеряется. Тогда это Major уже для плана, а не для кода.

### Прогоны ревьюера (go1.26.8 windows/amd64, golangci-lint 2.13.2)

- `go build ./... && go vet ./...` — exit 0;
- `go vet -tags "e2e integration" ./test/... ./shared/testkit/... ./cmd/...` — exit 0;
- `go test -short -count=1 ./...` — exit 0, все пакеты `ok`, FAIL нет;
- `golangci-lint run ./...` — 0 issues. Первый запуск упал с «parallel golangci-lint is
  running» (чужой процесс), повтор чистый;
- `gofmt -l internal shared` — пусто;
- `make test` — exit 0, `coverage-gate: internal/mechanics 95.7% (643 of 672)`, совпадает с
  карточкой.

### Мутанты ревьюера

Копия дерева без `.git`, `services/`, `Docs/`, `bin/` лежала в scratch
(`…/scratchpad/t053-review-mutants`), без `-overlay`, базовый прогон копии зелёный. Скрипт
делал литеральную замену с проверкой `count == 1` и восстанавливал файл из копии после
каждого прогона. После серии `diff -r` `internal/mechanics` и `shared/testkit` копии с
рабочей папкой — идентичны. Копия удалена по сохранённому точному пути.

| # | Мутант | Результат |
|---|---|---|
| R0 | контрольный (`resolve.go`): `Hit = !Fumble ∧ (Critical ∨ check.OK ∨ Threshold > −1000)`. Первая форма `Hit = true` не собиралась (`check` не используется), поэтому заменена поведенческой | **красный**: `TestResolveAttackTable`, 4 строки промаха |
| R1 | `Damage(damage.Result, out.Critical)` → `Damage(damage.Result, false)` | **красный**: крит игрока, крит против def 40, крит NPC |
| R2 | `FreeAttack: !success ∧ on_fail = free_attack` → `FreeAttack: !success` | **красный**: `TestResolveFleeTable` (`on_fail: none`) |
| R3 | порог бегства читает `living_enemies = 1` вместо `a.LivingEnemies` | **красный**: 5 строк `TestResolveFleeTable` |
| R4 | `min_hp` в обратную сторону (`cmp.Compare(y.HP, x.HP)`) | **красный**: `TestNPCTargetOrder` ×4, `TestNPCTargetReadsTheOrderOfTheRules` |
| R5 | причина свободной атаки `flee` → `combat` в `ChangesFor` | **красный**: `TestChangesForTable` и сверка с двойником («a failed flight is punished») |
| R6 | `if r.Excluded(*target)` → `if !target.Alive()` (участие не исключает) | **красный**: `TestResolveRefuses` idle / out_of_combat |
| D1 | двойник (`fake_encounter.go`, `wound`): персонажу не пишутся `died_at`/`killed_by` — Р3 исчезает | **красный**: «the gap "the double writes an NPC death record on a character" did not occur» |
| D2 | двойник: `killed_by` NPC = id решения вместо id атакующего — новое расхождение | **красный**: «a killing blow hands out the trophy» |
| D3 | двойник: в пакет встречи добавлен op `last_damager_probe` — новое расхождение на сущности встречи | **зелёный** → Minor-1 |

Мутанты исполнителя (M00–M35, в том числе выживший и закрытый M16) выборочно сверены с
тестами, заявленное подтверждается. R0–R6 выбраны так, чтобы не повторять их.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Minor-1 | Minor | `internal/mechanics/changes_encounter_test.go:342-344`, `:394`; `tasks/T-053.md:35`; `dev-log.md:34` | Сущность встречи в пакете двойника пропускается целиком. Р5 («`encounter.npcs[].last_damager` не пишет никто») в списке обязательных расхождений отсутствует: там пять констант — Р1, Р2 ×2, Р3, Р4. Если двойник или будущий агент начнёт писать `npcs[]` либо что-то ещё на встрече, тест останется зелёным (D3). Вопрос 3 к архитектору так и не всплывёт. Карточка и dev-log утверждают, что тестом держится каждое расхождение, — для Р5 это неверно. | Для сущности встречи проверять, что пути двойника ⊆ {`round_seq`, `participants`, `state`, `resolution`, `closed_by_event_id`}. Добавить расхождение Р5 как «пути `npcs` в пакете нет» — проверка отсутствия, с той же обязательностью, что у остальных. Либо, минимум, исправить формулировку в карточке и dev-log: Р5 установлен чтением кода, не тестом. | открыто |
| Minor-2 | Minor | `internal/mechanics/actor.go:55-58` | `kind` у NPC прочитан как необязательный: NPC без вида молча падает без трофея, и inv-03 этого не увидит. `data-model.md` §3.4 помечает `kind` обязательным. Обоснование канала ошибки `ActorFromEntity` в C-03 v1.2 — «актор без `hp`/`def` не должен молча получить ноль» — применимо и здесь: дефект создателя сущности превращается в бой по другим правилам (без трофея). | Одно из двух, решение вместе с вопросом 4 (`Actor.Kind` в C-03 v1.3). (а) NPC без `kind` → `missingAttr`; вид без таблицы трофеев остаётся законным. Перед правкой проверить сущности NPC в тестах `shared/testkit/{swarm,gateway,state}` (фикстура `testdata/fixtures/npc.json` вид содержит). (б) Явно записать в C-03 v1.3, что `Kind == ""` у NPC законен, и поправить `data-model.md` §3.4. | открыто |
| Nit-1 | Nit | `internal/mechanics/changes.go:112`, `:174` | Формула inv-02 `min(max(hp, 0), hpMax)` продублирована рядом с `Rules.ClampHP` (`rules.go:749`). Причина понятна: `ChangesFor` — свободная функция без `*Rules`. Но арифметика инварианта теперь живёт в двух местах. | Пакетная `clampHP(hp, hpMax)`, которую зовут и `Rules.ClampHP`, и `ChangesFor`. | открыто |
| Nit-2 | Nit | `internal/mechanics/changes.go:118-121` | Попадание с нулевым уроном (достижимо при формуле с отрицательным модификатором, например `d4-3`: `Damage` отсекает до 0) даёт `set hp` в то же значение. `restChanges` (`:175`) такой пустой `set` сознательно не порождает (`changed[]` пуст). Два пути одной функции ведут себя по-разному. | Не выпускать набор для цели, если `hpAfter == target.HP` и цель не пала, как в `restChanges`. Строка в `TestChangesForTable`. | открыто |
| Nit-3 | Nit | `internal/mechanics/changes.go:29-35` | Для пакета с несколькими ударами по одной цели (групповой раунд I2) вызывающий обязан передавать цель с HP после предыдущих ударов. Иначе второй `set hp` в одном наборе посчитан от устаревшего HP (двойник это делает в `pack`, `hp` map). Комментарий говорит «from the target as given», но это требование не называет. | Одна фраза в документации `ChangesFor`. Проверка — в задаче I2-2 (T-066 или соседней). | открыто |
| Nit-4 | Nit | `shared/testkit/mechanics/fixed.go:7-9`; `:319-338` | (1) Абзац пакетного комментария переформатирован с разрывом: «The rules themselves are» / «real — …». (2) Канал ошибки двойника уже настоящего. Настоящий `NPCTarget` отвечает ошибкой ещё на мёртвого NPC, не-NPC в роли кусающего, кандидата-не-персонажа и повтор id. Двойник на это отвечает целью, и потребитель, отлаженный на двойнике, узнает о дефекте только на настоящей механике. | (1) Сшить абзац. (2) Либо повторить четыре проверки в `FixedMechanics.NPCTarget`, либо назвать различие в его комментарии. | открыто |
| Nit-5 | Nit | `internal/mechanics/resolve.go:171-175`; `rules.go:739-745` | `rest.restore: d4` проходит `Load`, `Rules.Restore(a, rng)` бросает по неадресованному генератору, а `Resolve(rest)` на том же файле отказывает. Отказ верный, но проявится посреди игры, а не при загрузке правил. | Совпадает с предложением 2 исполнителя: запрет при `Load` (или назначение броска в схеме `dice.rolled`) — бэклог. | в бэклог |

### Предложения в бэклог

1. **EPIC-003 (двойник, запрос через tech-lead#1).** Р3: `FakeEncounter.wound`
   (`fake_encounter.go:1176-1182`) не должен писать `died_at`/`killed_by` павшему персонажу —
   нарушение `ownership.go:83-86`, настоящий State отвергнет пакет. Там же устаревшие
   комментарии `:132-133` («once EPIC-002 implements Resolve (T-053)»).
2. **EPIC-002, после ответа архитектора на вопрос 1.** Совместимое дополнение C-03 v1.3
   (`Action.At`, `Actor.Version`, `Actor.Kind`) и выставление `died_at`, `acquired_at`,
   `expected_version` в `ChangesFor`, с удалением Р1/Р2 из теста сверки. Размер S.
3. Предложения исполнителя 1–3 (проверка `attack.rolls`/`flee.rolls` при `Load`, запрет
   `rest.restore` кубами при `Load`, тип `dmg` в `data-model.md` §3.3) поддерживаю.

### Риски и допущения

- Слияние в ветку эпика: `review.md` и `dev-log.md` уже изменены там слиянием T-052
  (`2196b58`). Оба файла дописываются в конец, конфликт механический, но разрешать его
  нужно с сохранением обеих записей.
- `-race` не прогонялся (нет cgo), как и у исполнителя. Код T-053 конкурентности не
  содержит, а тест сверки вызывает двойник синхронно.
- `testkit.Deterministic` в `changes_encounter_test.go:228` меняет глобальный источник id
  шины. Сейчас в пакете нет `t.Parallel`, и это безопасно. Добавление `t.Parallel` в
  `internal/mechanics` сделает тест недетерминированным.

---

## T-060 · ревью #1 · 2026-09-13 · code-reviewer#1 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-060`, ветка `task/T-060-replay-eventclock`, HEAD `2196b58`. Коммитов вне
родителя нет. Ветка эпика ушла вперёд на 2 коммита (T-050, `03c551d`): там `shared/entity`,
`dev-log.md`, `review.md`, и с `serve.go` и `internal/replay` пересечений нет. Изменения не
закоммичены:

| Действие | Путь |
|---|---|
| A | `internal/replay/{eventclock,timers,cursor,journal,recording,middleware}.go` |
| A | `internal/replay/{clock,cursor,recording,middleware}_test.go` |
| M | `cmd/multiverse/serve.go` (+71/−17: `timeOf`, `runTime`, `process.run`, лог старта) |
| A | `cmd/multiverse/replay_test.go` |
| M | `Docs/dev-team/epics/EPIC-002-state-mechanics/{dev-log.md,tasks/T-060.md}` |

Артефакты в `Docs/` внутри папки задачи — по указанию оркестратора, замечанием не считаются.
Файлы — в границах раздела T-060 индекса (`internal/replay/**`, `cmd/multiverse/serve.go`).
Тесты сборки лежат в `cmd/multiverse`, как требует сверка 2026-09-13.

Основание:
- раздел T-060 в `tasks.md` ветки эпика: DoD и замена по T-416;
- карточка с отклонениями 1–4;
- C-01 v1.4–v1.7: «Источники конструкторов», «Таймеры повторной доставки», `Delivery`;
- C-07: ключ записи и `meta.replay`;
- C-14: порядок догона;
- `state-and-mechanics.md` §6.1–§6.2 и §7.3;
- ADR-001 доп. п. 2, ADR-003 п. 4, 6;
- `shared/eventbus/{sources,delivery,types}.go`, `membus`;
- `.golangci.yml`: depguard `internal-replay`, `cmd-others`; forbidigo.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 3 · Nit: 4.

Все четыре пункта DoD закрыты и доказаны тестами. Решение C-01 v1.4 «шина освобождена»
выполнено и закреплено тестом. Мутант ревьюера это подтвердил: тест на зависание краснеет.
Minor — пробелы доказательности и один вводящий в заблуждение комментарий. Их можно
исправить в этой ветке до слияния или вынести в бэклог — решение за tech-lead#1.

**Для приёмки tech-lead#1 (мягкий режим, обязательно).** `cmd/multiverse/serve.go`
правлен в ветке задачи. Проверено:
- Правка минимальна. Live-путь по поведению не изменился. `timeOf` для live возвращает
  `clock.Real{}`/`clock.RealTimers{}` и контекстам, и шине, как прежний
  `deps.Clock, deps.Timers = clock.Real{}, clock.RealTimers{}` + `openBus(…, deps.Timers)`.
  Обёртки транспорта и `SetClock` в live нет.
- Флаги `--mode`/`--recording`/`--id-source` по-прежнему разбирает `parseServe`, разбор не
  менялся. `--recording` без replay отказывается там же (serve.go:162–165).
- При конфликте с T-055 (тот же `process.run`) слияние разрешает developer#1 (индекс). Точки
  встречи: serve.go:284–294, где ставятся источники, и `defer eventbus.SetClock(nil)`.

### Проверенные пункты

**1. C-01 v1.4, таймеры.**
- serve.go:300: шине идёт `times.bus` = `clock.RealTimers{}` в обоих режимах.
- serve.go:283: контексты в replay получают `*replay.EventClock` и `replay.NullTimers{}`.
- Тест `TestTheModeDecidesTheTimeOfTheContextsAndNotOfTheBus` (шпион `openBus`) проверяет
  оба режима.
- Мутант M1 ревьюера (шине — `times.timers`) даёт красный
  `TestReplayRedeliversAFailingHandlerWithoutHanging`: «no dead letter within 10s after
  1 calls». Процесс при этом не виснет на остановке: `Delivery.wait` слушает `ctx.Done()`.

**2. `EventClock`.**
- `Observe`: `now = max(now, t)` под мьютексом. Настенных часов не читает: forbidigo и
  AST-тест `TestThePackageReadsNoWallClock`, который ловит и обход через `clock.Real`.
- Событие с тем же временем часы не сдвигает, событие «в прошлом» игнорирует
  (`TestEventClockIsMonotonicAndMovesOnlyByEvents`, шаги 2–4).
- Нулевое время не откатывает часы.
- Одна оговорка — Mi-3 ниже.

**3. `Recording` и `Writer`.**
- Round-trip побайтовый для записи, сделанной `Writer` (`json.Marshal`, ключи map
  отсортированы).
- Декодирование совпадает с шиной (`json.Unmarshal` в `eventbus.Event`, как `kafka.go:392`
  и `membus.go:455`). Потерь точности нет: `seed` в `dice.rolled` — строка.
- Ключ `LLMOutputKey(cid, agent.id, phase, attempt)` совпадает с C-07
  `(meta.correlation_id, meta.agent.id, phase, attempt)`, с ADR-003 п. 4 и с
  `swarm-llm-laws.md` «`providers/recorded`». Кодирование с длиной перед частью — то же,
  что у `WithCauseID` (C-01 v1.5). «Побеждает первая запись» соответствует «записан до
  использования».
- Нечитаемая запись отказывает до открытия шины: serve.go:279–282 стоят раньше
  `SetClock` и `openBus`. Тест `TestReplayRefusesAnUnreadableRecording` проверяет, что
  шпион шины не вызывался. Путь «битый JSON» на уровне процесса отдельно не проверен, но
  идёт по той же ветке ошибки `OpenRecording`. На уровне пакета номер строки проверяется.
- Чтение целиком в память для DoD допустимо: DoD о потоковом чтении молчит, а §6.1
  описывает `Recording` как JSONL сценария. Строка ограничена 16 МиБ. Процесс сейчас
  читает запись только ради `Start()`/`Len()` и сразу отпускает её. Потоковое чтение — в
  бэклог, когда появятся многочасовые записи.

**4. Middleware.**
- `Observe` вызывается до обработчика, `Meta.Replay = true` ставится на копии события
  (middleware.go:27–31). Журнал и `dead_letters` получают событие как опубликованное:
  `Meta` — значение, есть проверка в `TestWithMiddlewareDrivesTheClockOnEveryReadPath`.
- Middleware стоит внутри `Delivery`: валидация при чтении идёт раньше, повторы проходят
  через него снова.
- Утечки `meta.replay` в live нет: обёртка ставится только при `times.events != nil`, а
  `Middleware(live)` — тождество (`TestMiddlewareInLiveModeChangesNothing`, мутант
  исполнителя M18).
- Наследование производными: `eventbus.Derive` копирует `Replay: parent.Meta.Replay`
  (`shared/eventbus/types.go:196`), что соответствует C-07. Но тестом свойство не
  закреплено нигде в модуле — Mi-2.

**5. `eventbus.SetClock` глобально.** Гонки сегодня нет:
- в `cmd/multiverse` нет `t.Parallel()`, а `onLoopback` вызывает `t.Setenv`, который
  параллельность запрещает;
- пакеты тестов идут отдельными бинарниками;
- live-путь `SetClock` не вызывает, поэтому `fightThroughTheProcess` с
  `testkit.Deterministic` (fake_contexts_test.go:380) не затрагивается.

Затирание возможно только в будущем тесте, который ставит `Deterministic` и затем гоняет
процесс в replay. После прогона такой тест потеряет ручные часы. Оценка — латентный риск,
не блокирует. Решение — в T-055 вместе с остальными источниками (N-4).

**6. Лог старта.** В лог попадают только `recording` (путь, заданный оператором) и
`recorded_events` (число). Содержимого записи в логе нет: ни `response_raw`, ни текста
игрока. Приватность соблюдена.

**7. Вопрос исполнителя «кто публикует записанные события в шину».** Рекомендация: никто,
и публикатора в процессе не заводить.
- По §6.2, C-07 и ADR-003 п. 4 запись — таблица ответов для `RecordedProvider`, а не поток
  входа.
- В replay «новых `llm.output` не издаётся». `tick.fired`/`round.closed` читаются из
  журнала шины.
- Входы сценария подаёт харнесс по HTTP API (`testing/strategy.md`, строка e2e).
- Восстановление по C-14 догоняет журнал шины (`Journal`), а не `Recording`.
- Публикация `Recording.Events()` в `membus` дала бы вторые `llm.output` и задвоила бы
  факты.

Вопрос исполнителя вскрывает настоящий пробел, но не в T-060: как запись доходит до
потребителя. Подробности — «Вопросы к system-architect».

**8. Прогоны ревьюера** (папка задачи):
- `go build ./... && go vet ./...` — 0;
- `go test -short -count=1 ./...` — все пакеты ok;
- `go test -tags e2e -count=1 -timeout 10m ./test/e2e/...` — ok (15,4 с);
- `golangci-lint run ./...` — 0 issues, со второй попытки после «parallel golangci-lint
  is running»;
- `make test` — exit 0; `internal/replay` 97,3 % (145/149), `internal/mechanics` 94,9 %;
- `gofmt -l internal/replay cmd/multiverse` — пусто; `git diff --check` — чисто.

### Мутанты ревьюера

Копия дерева в scratch (`tar` без `.git`, `Docs`, `services`), без `-overlay`. Перед
мутантами проверено: базовый прогон копии зелёный. Копии удалены по сохранённому точному
пути.

| Мутант | Итог |
|---|---|
| M0 (контрольный): `Observe` не двигает часы (`if false && …`) | красный: 3 теста пакета + `TestReplayMovesTheClock…` |
| M1: шина в replay получает `times.timers` (`NullTimers`) | красный: `TestReplayRedelivers…` висит 10 с, 1 вызов обработчика |
| M2: `Observe` после обработчика, а не до | красный: `TestWithMiddlewareDrivesTheClock…`, `TestReplayMovesTheClock…` |
| M3 (зонд): в `serve.go` обёрнут только `Deps.Journal`, `Deps.Bus` — сырой транспорт | **выжил** → Mi-1 |
| M4 (зонд): `Derive` не наследует `Replay` (`shared/eventbus/types.go:196` → `false`) | **выжил** во всём модуле, включая e2e → Mi-2 |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-1 | Minor | `cmd/multiverse/replay_test.go:143–174` | Сборка проверяет обёртку только на пути `Deps.Journal.ReadRange`. Путь `Deps.Bus.Subscribe`, по которому будут читать почти все контексты, на уровне процесса не проверен. Мутант M3 (`Deps.Bus` без middleware) выживает. Пакетный тест проверяет `WithMiddleware`, но не проводку в `serve.go:308–311`. | В `TestReplayMovesTheClockOfTheContextsByTheEventsTheyRead` добавить чтение через `deps.Bus.Subscribe` с той же проверкой часов и `meta.replay`. Минимум — утверждение, что `deps.Bus` и `deps.Journal` — один и тот же объект после обёртки. |
| Mi-2 | Minor | `internal/replay/middleware_test.go:56–154`; обещание в `middleware.go:14–15` | Комментарий middleware опирается на свойство «производные события наследуют метку через `eventbus.Derive`» (C-07), но ни один тест модуля его не держит. Мутант M4 выживает во всём модуле, включая e2e. | В пакетный тест middleware добавить обработчик, который делает `eventbus.Derive(ev, …)`, и проверить `derived.Meta.Replay == true`. Правка только в файлах T-060, `shared/eventbus` не трогается. |
| Mi-3 | Minor | `internal/replay/middleware.go:11–13` | Комментарий «whatever the handler asks the clock is the time of the event it is handling» верен только при одном читателе. Часы одни на процесс, `now = max`. При нескольких подписках (разные топики, воркеры State) обработчик старого события видит время более позднего, а какое именно — зависит от планировщика горутин. Реализация соответствует §6.1, но комментарий обещает больше, и автор контекста может положиться на `Clock.Now()` вместо `ev.Timestamp` в пути, который попадает в байты события. | Переписать комментарий: часы стоят на самом позднем событии, которое видел любой обработчик процесса. Обработчику, которому нужно время своего события, брать `ev.Timestamp` (или `Derive`, наследующий его). Вопрос о детерминизме — к system-architect, ниже. |
| N-1 | Nit | `internal/replay/recording.go:97–104`, `cmd/multiverse/serve.go:397` | `Start` берёт время первой строки, а не минимальное. В `llm_records` порядок строк — порядок публикации, а время наследуется от причины, так что первая строка не обязательно самая ранняя. Монотонные часы, стартовавшие позже, проигнорируют более ранние события. | Брать минимум `Timestamp` по записи или записать в doc-комментарии, что запись упорядочена по времени, и проверять это при чтении. |
| N-2 | Nit | `internal/replay/recording.go:176–178, 194–204` | Комментарий `Writer`: «must be on disk when it is used (C-07)». Без `f.Sync()` запись только в кэше ОС: падение процесса она переживёт, падение машины — нет. Гарантия C-07 касается события шины `llm.output`, а не файла. | Смягчить комментарий («handed to the OS before Append returns») или вызывать `Sync` в `Append`, если писатель станет частью пути «записать до использования». |
| N-3 | Nit | `internal/replay/recording.go:57–73` | Если чтение оборвалось посреди строки с ошибкой ввода-вывода (не `EOF`), сначала разбирается обрывок. Наружу уходит ошибка JSON, а причина — ошибка чтения — теряется. | При `err != nil && !errors.Is(err, io.EOF)` вернуть ошибку чтения до `json.Unmarshal`. |
| N-4 | Nit | `cmd/multiverse/serve.go:292–293` | `defer eventbus.SetClock(nil)` возвращает настенные часы, а не те, что стояли до прогона. Сегодня безвредно (п. 5), но T-055 поставит рядом `SetIDSource`/`SetRegistry` и получит тот же вопрос. | Решить в T-055 одним способом для всех трёх источников: процесс ставит их из `Deps` и не снимает, а тесты ставят свои после старта, либо у `eventbus` появляется чтение текущих источников (`contract-change`). Отмечено исполнителем в карточке, подтверждаю. |

### Вопросы к system-architect (через оркестратора; не блокируют T-060)

1. **Как запись попадает к `RecordedProvider`.** По §6.2 он работает «над
   `Recording.Index("llm.output", …)`». Но ADR-001 доп. п. 2 и depguard (`.golangci.yml`,
   `cmd-others`, правила `internal-*` с `deny internal`) разрешают импорт `internal/replay`
   только `cmd/multiverse`. `internal/llm/providers/recorded` не может импортировать ни
   `Recording`, ни `LLMOutputKey`. В `runtime.Deps` поля для записи нет (C-01 v1.3), и путь
   `--recording` до контекста `llm` сейчас не доходит.

   Поэтому предложение исполнителя «EPIC-003 берёт `replay.LLMOutputKey`»
   (`tasks/T-060.md:107`) по правилам импорта невыполнимо. Если оставить как есть, в
   EPIC-003 появится второй ключ — ровно тот риск, о котором предупреждает исполнитель.

   Варианты:
   - (а) вынести формат записи и ключ в `shared/` (например, `shared/recording`) —
     изменение карты владения;
   - (б) процесс строит провайдер и передаёт его контексту `llm` — изменение `Deps`, C-01;
   - (в) `providers/recorded` читает JSONL сам, а ключ фиксируется golden-тестом с обеих
     сторон.
2. **Детерминизм `Clock.Now()` в обработчиках replay при нескольких читателях** (Mi-3).
   Нужно ли правило «в обработчике время — только `ev.Timestamp`», закреплённое в C-01 или
   §6.2? Или часы должны быть свои на каждого читателя?
3. **Время корневых событий в e2e-replay.** Действия харнесса по HTTP в replay получают
   время `EventClock`, то есть время последнего прочитанного события, а не время живого
   прогона. INT-05 требует «последовательность доменных событий побайтово идентична».
   Нужно определить, откуда берётся `timestamp` корней в replay: из запроса харнесса, из
   записи или исключается из сравнения. Решение — EPIC-005/T-061 с архитектором, в T-060
   не решается.

### Предложения в бэклог

1. T-055: общий способ ставить и снимать `SetClock`/`SetIDSource`/`SetRegistry` (N-4).
   Тест процесса в replay после `testkit.Deterministic`.
2. `internal/replay`: потоковое чтение записи для многочасовых сессий. `Recording`
   отдаёт итератор, `Index` строится за один проход.
3. EPIC-001 (`shared/eventbus`): тест «`Derive` наследует `Meta.Replay`» рядом с тестами
   конструкторов. Mi-2 закрывает это со стороны T-060, но свойство принадлежит конверту.
   Текст C-01 в перечне копируемых `Derive` полей не называет `Replay`, хотя код его
   копирует. Это редакционная правка контракта.
4. Решить, допустим ли `--mode=replay` без `--recording`. Сейчас часы стоят на
   `0001-01-01`, и корни штампуются этим временем. Возможно, нужен отказ старта или
   явная запись в логе (tech-lead#1).

### Риски

- Текстовые конфликты при слиянии в ветку эпика. `dev-log.md` и `review.md` дописаны в
  конец и в этой ветке, и в T-050 (`03c551d`). Разрешается склейкой разделов.
- Конфликт `serve.go` с T-055 в `process.run` (serve.go:279–311) — ожидаемый, назван в
  индексе.
- `meta.replay` сериализуется без `omitempty`. Сравнение replay-вывода с живой записью
  (EPIC-005) должно учитывать, что флаг меняет байты. Исполнитель это отметил.

## T-054 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-054`, ветка `task/T-054-solo-invariants`, HEAD `4119170` (база задачи).
Коммитов вне родителя нет. Изменения не закоммичены. Ветка эпика с тех пор ушла вперёд на
`24f1baf` (T-060: `internal/replay`, `cmd/multiverse/serve.go`). С файлами T-054 это не
пересекается, кроме артефактов эпика.

| Действие | Путь |
|---|---|
| M | `internal/mechanics/{invariants,rules,types}.go` — проверки; в `rules.go`, `types.go` только комментарии |
| M | `internal/mechanics/rules_test.go` — `TestInvariantsRegister` перевёрнут |
| A | `internal/mechanics/invariants_test.go`, `internal/mechanics/testdata/laws-v1.invariants.yaml` |
| M | `shared/testkit/mechanics/fixed.go` — только комментарий |
| M | `Docs/dev-team/epics/EPIC-002-state-mechanics/{tasks/T-054.md,dev-log.md}` |

Артефакты в `Docs/` внутри папки задачи разрешены оркестратором, это не замечание. Карта
владения не нарушена: `internal/mechanics/**` и `shared/testkit/mechanics/**` принадлежат
EPIC-002. `shared/entity`, `shared/testkit/state`, схемы и фикстуры не менялись.

На что опирается ревью:
- `tasks.md` T-054 (описание, DoD, сверка 2026-09-13);
- `state-and-mechanics.md` §4.5 п. 5 и п. 8, §5.7;
- ADR-012 п. 5; ADR-001, дополнение 2026-09-13, п. 1 (1a);
- C-02 v1.2–v1.5, C-03 v1.2, C-05 v1.4–v1.5 (п. 1в, 1е, 2, 5, 6);
- `data-model.md` §3.2–§3.7, §10;
- `shared/entity/{attrs,types}.go`, `internal/mechanics/changes.go` (T-053);
- `shared/testkit/swarm/fake_encounter.go` (`finish`, `leaveAll`, `livingCount`).

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 3 · Nit: 3.

Пять проверок реализованы, подключены к реестру и детерминированы. На законных путях соло,
которые закреплены контрактами, отказов нет: смертельный удар закрывает встречу тем же
пакетом, `/forget` меняет только персонажа, трофей выдаётся по `ChangesFor`, бегство ведёт
в `outside:{world}`, мир из фикстур проходит. Тест реестра перечисляет все десять законов.
Набор методов `StateView` закреплён присваиванием в обе стороны. Десять мутантов ревьюера
проверены, в том числе контрольный. Восемь убиты, один эквивалентен, один выжил (Nit-2).

Замечания Minor-1 и Minor-2 про inv-01 не блокируют соло. Но проверка inv-01 шире, чем
строка inv-01 в таблице КД §5.7. Она накладывает на агента встречи EPIC-003 требования,
которых нет в C-05: вывести погибшего из участия и не держать мёртвого NPC в `npcs[]`
идущей встречи. Эти требования нужно подтвердить у system-architect до I2 (T-066) или
сузить проверку (см. «Вопросы к system-architect»).

### Проверенные пункты

| # | Что проверено | Как | Результат |
|---|---|---|---|
| 1 | Реестр и DoD п. 2 | `allInvariants`: `Check` есть у 01, 02, 03, 09, 10; нет у 04–06 (I2, T-066) и у 07–08. `TestInvariantsRegister` перечисляет десять id с ожиданием. `checkOf` берёт проверку из `Rules.Invariants()`, а не по имени функции | выполнено |
| 2 | Список `laws@v1` | `testdata/laws-v1.invariants.yaml`: десять записей `kind: invariant`, без текста законов. `TestInvariantIDsMatchTheLaws` сравнивает множества и проверяет `kind` | выполнено |
| 3 | inv-01 на трёх терминальных статусах | `TestInvDeadDoesNotActTreatsEveryEndAlike` перебирает `entity.TerminalStatuses` и сравнивает нарушения `reflect.DeepEqual`. Сообщение статус не называет | выполнено |
| 4 | Детерминизм | Время не читается: `time.*` в `invariants.go` нет, `forbidigo` чист. `sortViolations` даёт полный порядок `(InvariantID, EntityID, Message)` и `slices.Compact`. Обход map (`sourcesHeldBy`) и порядок `ByType` снимаются сортировкой: сообщения разных источников различны. `TestInvariantsAreDeterministic`: `ByType` в порядке map, три порядка `touched` с повторами, 50 прогонов на закон | выполнено |
| 5 | Совместимость `StateView` со стражем | В `invariants_test.go:55-65` `guardianView` присваивается в `StateView` и обратно. Набор методов совпадает с `guardian.InvariantView` из ADR-001 доп. 2026-09-13 (1a). `Violation` у стража своя, её переводит адаптер в `internal/swarm`, так и записано в ADR | выполнено |
| 6 | Совместимость с T-053 | `TestInvariantsAcceptWhatChangesForProposes`: пакет `ChangesFor` смертельного удара с трофеем применяется через `entity.ApplyOps` и не нарушает ни одного закона. Встреча, оставленная открытой, даёт ровно одно inv-01 | выполнено |
| 7 | Совместимость с T-448 | Папка `.worktrees/T-448` (не закоммичено): сигнатура `entity.ApplyOps(e, ops) (map[string]any, []Change, error)` не меняется. `Change` получает `HasOld/HasNew`, T-054 его не читает. `shared/testkit/mechanics` T-448 не трогает. Пересечение только в `dev-log.md`/`review.md` (дописываются в конец, драйвер `appendtail`) | выполнено |
| 8 | Ложные срабатывания соло | Прочитаны `FakeEncounter.finish/leaveAll/fled` и `ChangesFor`, по ним прогнаны зонды в копии дерева. Смерть NPC или игрока в соло сразу закрывает встречу (`players_out`/`npc_dead`, `leaveAll`). Бегство ставит `outside:{world_id}`. У погибшего персонажа нет `died_at/killed_by` (`changes.go:138-142`), поэтому inv-09 молчит. JSON-числа `float64` читаются как целые | выполнено, исключения — Minor-1…3 |
| 9 | Нечитаемые данные → нарушение | `hp` строкой, дробью или `null` — inv-02. `inventory` затронутого персонажа не декодируется — inv-03. `npcs`/`participants` затронутой встречи не декодируются — inv-01. Сломанный инвентарь незатронутого персонажа молчит. Решение п. 6 dev-log поддерживаю: закон, который нельзя прочитать, никто не держит | выполнено |
| 10 | Прогоны ревьюера (go1.26.8 windows/amd64) | `go build ./...` — 0. `go vet` (`internal/mechanics`, `shared/testkit/...`) — 0. `gofmt -l internal shared cmd` — пусто. `go test -short -count=1 ./...` — все пакеты ok. `golangci-lint run ./internal/mechanics/... ./shared/testkit/mechanics/...` — 0 issues. Покрытие `internal/mechanics` — 96,5 %. `-race` не прогонялся (нет cgo) | выполнено |

### Мутанты ревьюера

Копия дерева в scratch (`t054rev-…`: `go.mod`, `go.sum`, `internal`, `shared`, `rules`,
`testdata`, `schemas`), без `-overlay`. Драйвер делал точную замену (`count == 1`), прогонял
`go test -short -count=1 ./internal/mechanics/` и восстанавливал файл. Перед удалением
копии `invariants.go` сверен с рабочей папкой — совпадает. Копия удалена по точному пути.
Мутанты выбраны так, чтобы не повторять M00–M52 исполнителя.

| # | Мутация | Результат |
|---|---|---|
| C0 | контрольный: `hp > hpMax` → `hp > hpMax+100` | красный |
| R12 | `isTerminal` без проверки `e != nil` | зелёный, эквивалентный: вид с `(nil, true)` не строится ни одной реализацией |
| R15 | inv-09: терминальные только `dead`/`abandoned` (без `ascended_final`) | **зелёный** → Nit-2 |
| R27 | inv-03: со стороны NPC `loot_claimed_by` не сверяется | красный |
| R28 | inv-03: проверка «два решения» выключена | красный |
| R30 | inv-10: `outside:` без сверки мира | красный |
| R31 | inv-02: отрицательный `hp` разрешён | красный |
| R32 | inv-01: ошибка декодирования `npcs` глотается | красный |
| R33 | inv-03: сломанный инвентарь затронутого персонажа молчит | красный |
| R34 | `touchedEntities` пропускает первый id | красный |

Зонды поведения (тест в копии, в рабочую папку не попал):

| # | Мир и `touched` | Ответ | Где в замечаниях |
|---|---|---|---|
| P1 | Два волка, первый убит, бой идёт; `touched` = встреча и убитый волк | inv-01 на `wolf-alpha` | Minor-2 |
| P2 | Группа: `player-A` погиб в раунде, участие не переписано (`in_combat`); `touched` = встреча и игрок | inv-01 на `player-A` | Minor-1 |
| P3 | `/forget` применён раньше пакета «оба промахнулись» (в пакете только `round_seq` встречи); `touched` = встреча | inv-01 на `player-A` | Minor-1 |
| P5 | У персонажа нет `hp` (`remove hp`) | нарушений нет | Minor-3 |
| P8 | Предмет с источником-регионом у двух персонажей | inv-03 «trophies of dark-forest-01» | Nit-1 |
| P9 | `ascended_final` с `killed_by` | нарушений нет (верно, но не закреплено тестом) | Nit-2 |
| P10 | `position: ""` у персонажа | inv-10 «no such region» | верно |

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Minor-1 | Minor | `internal/mechanics/invariants.go:139-145`, `:151-158` | Проверка участников inv-01 отвергает пакеты на двух путях, которые контракты не запрещают. (1) **Соло, MVP-1:** пакет «оба промахнулись» трогает только встречу (`round_seq`, C-05 п. 2). Если `/forget` того же игрока применён раньше, пакет получает `law_violation inv-01` (зонд P3). По C-05 п. 1е это «дефект»: `Error`, откат попытки. Харнесс e2e считает отказ дефектом сценария. Без T-054 пакет применился бы, а встречу закрыл бы пакет `resolve` по чужому факту. Исход тот же, что у варианта гонки с попаданием (п. 1в/1е, там `version_conflict` → `dead_entity`), так что нового класса ошибки нет, меняется только причина. (2) **Группа, I2:** участник погиб в раунде, бой идёт, `participants[].state` не переписан. Отвергается весь atomic-пакет раунда (зонд P2). C-05 выводить погибшего из участия не требует. `data-model.md` §3.7 состояние `dead` участника знает, но не обязывает. КД §5.7 для inv-01 на стороне State называет только `dead_entity` и неизменность статуса | Не сужать код без решения архитектора: проверка участников — единственное место, где DoD «три статуса, одно нарушение» выражается кодом. До ответа: (а) добавить в `TestInvDeadDoesNotAct` строки P2 и P3 как **закреплённые нарушения** с комментарием «требование к агенту встречи, вопрос 3 карточки», чтобы смена решения была видна тестом; (б) дополнить открытый вопрос 3 карточки и dev-log гонкой `/forget` из соло (пункт 1 здесь) | открыто |
| Minor-2 | Minor | `internal/mechanics/invariants.go:132-137` | Сторона NPC inv-01 — «терминальный NPC в `npcs[]` незакрытой встречи» — верна для встречи с одним NPC. При двух и более NPC отвергается законное продолжение боя после первого убийства (зонд P1). Правила такой бой уже предусматривают: порог бегства `10 + living_enemies`, `Action.LivingEnemies`. `FakeEncounter.livingCount` (`fake_encounter.go:1940-1948`) держит мёртвых в `npcs[]` и считает живых. Удалять NPC из `npcs[]` не требует ни C-05, ни `data-model.md` §3.7. В MVP-1 путь недостижим: встречу открывают с одним NPC (UC-008 шаг 3, `fake_encounter.go:735`). Проверка молча вводит правило для I2+ | Предпочтительно сформулировать сторону NPC как «встреча не `resolved`, `npcs[]` не пуст и **все** NPC терминальны». Для одного NPC это то же самое, для нескольких — без ложных отказов, и требования чистить `npcs[]` не появляется. Нарушение — на встрече или на каждом NPC, как сейчас. Выбор мёртвой цели уже держат `NPCTarget`/`Resolve` (`ErrInvalidTarget`). Либо вынести в вопросы архитектору (вопрос 4 ниже) и закрепить P1 тестом. Строки «a corpse named twice», «both sides» при правке пересмотреть | открыто |
| Minor-3 | Minor | `internal/mechanics/invariants.go:189-191` против `:407-411` | inv-02 пропускает `player`/`npc` без `hp` (зонд P5), а inv-10 того же `player`/`npc` без `position` отвергает. `hp` у обоих обязателен (`data-model.md` §3.3, §3.4). Строка владения `task` (§4.6) разрешает путь `hp`, `ApplyOps` умеет `remove`, поэтому `remove hp` на живом персонаже не остановит ни один закон. `hp: null` при этом уже нарушение (`not a whole number`), и два почти одинаковых пакета получают разные ответы | Как в inv-10: для `TypePlayer`/`TypeNPC` без `hp` — нарушение «has no hp to hold in range». Прочим типам `hp` по-прежнему необязателен. Строки в `TestInvHPInRange`: персонаж без `hp` — нарушение, регион без `hp` — нет (уже есть) | открыто |
| Nit-1 | Nit | `internal/mechanics/invariants.go:274` | Трофеем считается любой предмет с `source.entity.id`, тип источника не проверяется (зонд P8: предметы с источником-регионом у двоих — inv-03). В MVP-1 предметы дают только NPC (`data-model.md` §3.5), так что это не баг. Но первый же сундук или торговец (EPIC-006+) получит ложный отказ с текстом «one NPC leaves one trophy» | `if item.Source.Entity.ID == "" \|\| item.Source.Entity.Type != entity.TypeNPC { continue }` и строка теста «предмет не от NPC — не трофей» | открыто |
| Nit-2 | Nit | `internal/mechanics/invariants_test.go:511-514` | В `TestInvRespawnTTL` из терминальных статусов с записью смерти проверены `dead` и `abandoned`. `ascended_final` не проверен, мутант R15 выжил. Риск мал (`entity.IsTerminalStatus` общий), но inv-01 гоняется по всем `TerminalStatuses`, а inv-09 нет | Перебрать `entity.TerminalStatuses` в строках «с записью — не нарушение», как в `TestInvDeadDoesNotActTreatsEveryEndAlike` | открыто |
| Nit-3 | Nit | `internal/mechanics/invariants.go:226` | `trophyHolders` декодирует инвентари всех персонажей мира (JSON-круг на предмет) при каждом вызове, даже когда в `touched` нет ни `player`, ни `npc`: тик региона, пакет только встречи, создание встречи. При ≤ 50 сущностей это копейки, но проверка стоит на пути каждого предложения (C-02: p95 ≤ 50 мс) | Сначала отобрать затронутых `player`/`npc` и строить `holders` только если такие есть | открыто |

### Вопросы к system-architect (перечень с оценкой, не решение)

1. **`touched` — id сущностей или пути.** КД §4.5 п. 8 («`touched = ids изменённых`»), ADR-012
   п. 5 и сигнатура `Check(v, touched []string)` говорят про id. «Список изменённых путей» есть
   только в формулировке задачи. *Оценка:* правильно id. По путям без id не выразить ни
   inv-01, ни inv-03, а id вычисляются из путей тривиально. Остаётся уточнить для T-056, какие
   id входят. Рекомендую все сущности применённой части `changes[]` плюс созданную на пути
   `create`, а не только те, у кого `changed[]` не пуст. Тогда пакет «set в то же значение» не
   выпадает из проверки. Текст задачи поправить в `tasks.md`.
2. **`players_present` и inv-10.** `shared/entity/attrs.go:353-355` («checked by inv-10») и
   `data-model.md` §3.2 («проверяется инвариантом») обещают проверку проекции. В таблице КД
   §5.7 её нет, а строка `gateway` в `OwnershipRules` меняет `position` без региона.
   *Оценка:* согласен с исполнителем, в `Check` проекцию не проверять — это отклонило бы
   каждый ход. Проекцию считать производной (пересчитывает State или только
   `mvctl report --audit`). Правка — комментарий `attrs.go` и строка `data-model.md` §3.2.
3. **Выход погибшего из участия (группа) и гонка `/forget` (соло)** — Minor-1. *Оценка:*
   нужен пункт C-05. Вариант А: агент встречи в пакете, где участник стал терминальным при
   продолжающемся бое, пишет `participants[].state = dead` (для `/forget` — `out_of_combat`
   в пакете `resolve` или в следующем пакете встречи). Отказ `law_violation inv-01` в гонке с
   `/forget` признаётся остаточной ценой, как в п. 1в. Вариант Б: проверку участников снять,
   ведь `expected[]` шлюза и `NPCTarget` уже исключают терминальных. Но тогда DoD «три
   статуса, одно нарушение» теряет выражение в `Check`. Склоняюсь к А: форма участия уже есть
   в модели.
4. **Мёртвый NPC в `npcs[]` идущей встречи (несколько NPC)** — Minor-2, нового вопроса в
   карточке нет. *Оценка:* не требовать чистки `npcs[]`, двойник держит мёртвых как историю.
   Сузить закон до «незакрытая встреча без живых NPC».
5. **Кто отвечает на `dead → alive`: `dead_entity` или `law_violation inv-09`.** КД §4.5 п. 5
   (`state-and-mechanics.md:303`) говорит «переход `status: dead → alive` запрещён всегда
   (`law_violation inv-09`)». C-02 v1.5 (`contracts.md:313`) — «переход терминальной сущности —
   `dead_entity`». Проверка T-054 этот переход не видит и видеть не может (мира «до» нет),
   поэтому ответ целиком за шагом 5 T-056. *Оценка:* нужна одна формулировка до T-056. Логичнее
   `law_violation {invariant_id: inv-09}`: у условия есть инвариант, это правило самого C-02
   v1.5 («условие на состояние мира — `law_violation`, если у условия есть инвариант»).
6. **TTL возрождения (вторая половина inv-09).** *Оценка:* согласен, в State без атрибута
   связи «новый NPC заменяет прежнего» не проверить. В MVP-1 закон держит GM региона (§5.7).
   Атрибут в модели данных — только если закон захотят держать в State (вне MVP-1).

**Что State обязан держать на шаге 5 (T-056)**, потому что `Check` этого не видит:
- любое изменение терминальной сущности, кроме путей `died_at`, `killed_by`,
  `loot_claimed_by`, `encounter_id`, отвергается `dead_entity`. Текст §4.5 п. 5 перечисляет
  `{dead, ascended_final}`, но по сведению 3 (C-02 v1.2) сюда входит и `abandoned`;
- переход терминальный → нетерминальный отвергается **до** применения, по сущности «до», в
  том числе когда тот же пакет стирает `died_at`/`killed_by`. Иначе запись смерти не
  останется и inv-09 в `Check` промолчит. Причина — вопрос 5;
- `Violation.EntityID` может называть сущность **вне** `changes[]`: inv-01 отвечает на NPC
  или участника, когда затронута только встреча. Для `rejected.entity {id, type}` State берёт
  тип из вида мира, а не из `changes[]`;
- порядок: первое нарушение — первое в порядке реестра, внутри закона — после
  `sortViolations`. Воспроизводимость отказа при replay держится на этом порядке.

### Предложения в бэклог

1. T-056 (после T-448): снять комментарии «every Check is nil until T-054» в
   `shared/testkit/state/state.go:119,166-176` и `apply_test.go:482-497`. `WithInvariants()`
   двойника начинает звать `Invariants()`, а тест `TestWithInvariantsIsANoOp` меняет ожидание:
   позиция `nowhere-at-all` — это inv-10. Поддерживаю предложение 1 исполнителя.
2. Предложения 2–3 исполнителя поддерживаю: `mvctl report --audit` по всем id,
   `mvctl laws check` вместо `testdata/laws-v1.invariants.yaml`.
3. EPIC-003 (агент встречи, I2): тест на пакет раунда, где погибает один из участников
   группы, против `mechanics.Invariants()`. Он ловит расхождение Minor-1 раньше e2e.

### Риски и допущения

- Слияние в ветку эпика: `review.md` и `dev-log.md` дописываются в конец и там же
  изменены T-448 и T-060. Конфликт механический, обе записи сохранить (драйвер `appendtail`
  проверить после клона).
- Зонды P1–P3 — модельные миры, а не прогон двойника `FakeEncounter` против
  `WithInvariants()`: у двойника State проверки ещё нет. Реальная частота гонки P3 на стенде
  не измерялась.
- `-race` не прогонялся (нет cgo). Проверки — чистые функции без общего состояния.
- `make test` целиком ревьюер не запускал. Прогнаны `go test -short -count=1 ./...` и
  `golangci-lint` по двум пакетам задачи. Порог покрытия исполнитель прогнал через `make test`.

## T-448 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-448`, ветка `task/T-448-changed-form-proposal-id`, база `03c551d`. Коммитов в ветке нет. Ревьюировалась **рабочая копия**: `git diff` (13 файлов) и неотслеживаемые `shared/entity/changed_test.go`, `tasks/T-448.md`. Метка `contract-change`. Это ревью кода; решение по форме контракта подтверждает system-architect#1 (параллельно), здесь его не заменяют.

Основание: карточка `tasks/T-448.md` (DoD, текст C-02 v1.6 для T-449); раздел T-448 в `tasks.md`; запись `<!-- dev-log T-448 -->`; C-02 v1.5 (`contracts.md`); КД `state-and-mechanics.md` §3.2, §4.8, §8; журнал, «Пакет решений system-architect#1», строка T-448; ревью T-052 и T-050 (бэклог п. 2 о ±2^53).

Правки в `Docs/dev-team` внутри ветки задачи (КД, `tasks.md`, карточка, `dev-log.md`) заданы составом задачи, замечанием не считаются.

### Вердикт

**Вернуть.** Critical: 0 · Major: 1 · Minor: 2 · Nit: 3.

Форма `changed[]`, `MarshalJSON`/`UnmarshalJSON`, элемент созданного предка, `proposal_id` у create и схемы сделаны верно. Тест-свойство содержательное, совместимость с веткой эпика подтверждена прогоном. Возврат — из-за одного пункта: границы ±2^53. Для 2^53+1, числа из текста v1.6, правило не срабатывает ни на одном пути через шину. Двойник принимает такое значение молча и пишет 2^53. Тест через двойник взял 2^60 и этого не видит. Во время ревью system-architect#1 внёс в карточку своё решение и пришёл к той же границе (раздел «Решение system-architect#1», п. 4; «Что сделать в итерации 2», п. 1), текст v1.6 п. 6 уже исправлен. Код, тесты, КД и README пока на прежней границе.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Решение «догон `a[n]` — простое добавление без дедупликации» | Верно. Факт несёт итог дедупликации, принятой при применении. Повторная дедупликация при догоне отбросила бы законный элемент: после `append {colour: red}` + `set inventory[1] = {item_id: item-1}` в списке два элемента с одним `item_id`. Разобрал порядок элементов. Индексы `a[n]` без `old` от `append` идут подряд с длины «до». Хвост, отсутствующий в конце, выпадает целиком. Элемент списка целиком несёт итог. Значит, при догоне `n > len(a)` не возникает |
| 2 | Решение «удаление отсутствующего пути — no-op» | Верно. Каждый элемент `changed[]` читается из итогового состояния (`ops.go:653-654`). Предыдущий элемент того же факта мог записать контейнер уже без пути (`inventory[0] = {}` перед `inventory[0].kind` без `new`). Отсутствие пути здесь — целевое состояние, а не ошибка |
| 3 | Решение «элемент созданного предка» (`ops.go:602-625`, `:643-650`) | Верно. Это исправление дефекта T-050: `inc fresh.deep` + `remove fresh.deep` давали `changed: []` при сдвинутом хеше. Предок — самый внешний префикс, которого до изменения не было или который был скаляром (`isContainer` совпадает с тем, во что проходит `setIn`, `path.go:146-154`). Элемент дописывается в конец `paths` с итоговым значением, поэтому переписывает более ранние элементы под ним согласованно. Для префикса по `[` и для скаляра, ставшего контейнером (`inventory[0] = "x"` → `{x: {}}`), логика та же. **Мутант** в копии: условие `created && false` → красный `TestCatchingUpOnChangedReproducesTheState` |
| 4 | Отклонение: update без `proposal_id` пропускается с `Warn` (`apply.go:99`, `:162`, `:413-420`) | Согласуется. `entity.update.rejected` требует `proposal_id` (C-02, «Выход State»), поэтому валидного отказа без него нет. Подстановки `event.id` в C-02 нет. Обе схемы предложений поле требуют. Шина проверяет схему при публикации, `membus` — и при чтении (по умолчанию), и такое событие не доходит до двойника. Издатели передают поле: `Harness.CreatePlayer` и update (`harness.go:797`), `FakeEncounter` (`fake_encounter.go:779`, `:1264`), bootstrap-тест `cmd/multiverse`. У EPIC-003/EPIC-004 (`epic/*` HEAD) других издателей предложений нет. Для T-055 — «Предложения в бэклог», п. 3 |
| 5 | `Change` на проводе | `MarshalJSON` пишет ключ по `OldPresent`/`NewPresent`, присутствующий `null` сохраняется (`*json.RawMessage` + `omitempty`). `UnmarshalJSON` читает `map[string]json.RawMessage`, поэтому «ключа нет» и `null` различаются, что проверено шестью формами туда-обратно. `cloneChanges` флаги копирует. `last_change` в `StateHash` не входит (`hash.go:22`), золотой литерал не затронут. Потребители `FakeNarrator` (`New` у `hp`/`status`) и `FakeEncounter` (`fake_encounter.go:1630-1635`) компилируются и ведут себя как раньше. Строка для EPIC-003 в карточке верна |
| 6 | Схемы и фикстуры | `entity.updated`: `required: [path]` + `anyOf` при `additionalProperties: false` — `{path}` и лишний `op` отвергаются. `entity.create.proposed`: `proposal_id` в `required`, `minLength: 1`. Невалидные фикстуры `entity.updated` (нет `changed`) и `entity.create.proposed` (нет `attributes`, `proposal_id` есть) дают по одной ошибке. Валидная `entity.updated` — `append` с одним `new`. `go run ./cmd/mvctl contracts check`: 65 types, 8 topics, 58 schema files |
| 7 | Совместимость с веткой эпика | Копия `epic/EPIC-002-state-mechanics` HEAD `4119170` (T-050, T-052, T-053, слияние develop с T-444/T-445) плюс патч T-448. `git apply` не применился только к `dev-log.md` — это дописывание записи, его ведёт драйвер appendtail. Код, схемы, README фикстур и КД легли, README фикстур со сдвигом 7 строк. На копии: `go build ./...`, `go vet ./shared/... ./internal/... ./cmd/...` — 0; `go test -short -count=1 ./...` — все пакеты ok, в том числе `internal/mechanics` (T-053, `changes_encounter_test.go` на двойнике State); `mvctl contracts check` — 0. T-054 (`.worktrees/T-054`, не закоммичено) правит только `internal/mechanics` и `shared/testkit/mechanics` и не использует `Change`/`proposal_id`, пересечения нет |
| 8 | Прогоны в рабочей папке | `go test -short -count=1 ./shared/entity/ ./shared/testkit/state/ ./shared/contracts/` — ok; `gofmt -l shared/entity shared/testkit/state` — пусто. Полный набор разработчика не повторялся, кроме прогона на объединённой копии (п. 7) |
| 9 | Мутанты (независимо, копия в scratch, без `-overlay`) | **M0 (контрольный)**: `func broken( {` перед `intInSafeRange` → красный (`ops.go:517:14: syntax error`). **M5'** (п. 3) → красный. После каждой мутации файл восстановлен и сверен `cmp` с исходником копии. Копия удалена по сохранённому точному пути |
| 10 | Зонды ±2^53 (временный тестовый файл в той же копии) | `ApplyOps` с `Op`, разобранным из JSON (`"value": 9007199254740993`, `-9007199254740993`), и `json.Number("9007199254740993.0")` → `err = nil`, в `changed` — `9007199254740992`. `FakeState`: `set seed = int64(1)<<53 + 1` → `entity.updated` с `new: 9007199254740992`, отказа нет. См. Ma-1 |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Ma-1 | Major | `shared/entity/ops.go:484`, `:514`, `:517-519`; `shared/testkit/state/apply_test.go:581-590`; `shared/entity/changed_test.go:494-501`; `tasks/T-448.md:142` (текст v1.6, п. 6 — до правки архитектора); `state-and-mechanics.md:162`, `:747`; `shared/entity/README.md:138-141` | **Граница включительная (`\|x\| ≤ 2^53`), а проверка стоит после декодирования.** Всякое событие проходит JSON: `membus.go:213`, `:455`, адаптер kafka; двойник ещё раз перекодирует payload (`apply.go:79-85`). `float64` не отличает 2^53+1 от 2^53, поэтому литерал 2^53+1 приходит в `ApplyOps` как ровно 2^53 и принимается. Вместе с ним принимаются все дробные литералы, которые округляются к 2^53, и `json.Number("9007199254740993.0")` (`ParseInt` падает с синтаксической ошибкой, `ParseFloat` округляет). Итог: исходный текст v1.6 обещал «число по модулю больше 2^53 State отвергает», а на любом пути через шину для 2^53+1 State молча записывает 2^53. Это та самая потеря значения, ради которой правило заведено. Отказ срабатывает только при прямом вызове из Go, так что одно и то же предложение получает разный ответ в unit-тесте и через шину. Тест через двойник взял `1<<60` — число, точно представимое во `float64`, — и расхождения не видит. Хеш State и read-model при этом не расходится: округление происходит до State | **Совпадает с решением system-architect#1** (карточка, п. 4 решения и п. 1 итерации 2): исключающая граница `\|x\| ≤ 2^53−1` (I-JSON, RFC 7493 §2.2; `Number.MAX_SAFE_INTEGER`): `< maxSafeInteger` в `intInSafeRange` и в ветке `ParseFloat`. Тогда любой литерал вне безопасного диапазона декодируется в `\|f\| ≥ 2^53` и отвергается одинаково при прямом вызове и через шину. Правки: в `accepted` — `2^53−1` и `-(2^53−1)`, `2^53` и `float64(2^53)` переносятся в `refused`; в двойник — случай `int64(1)<<53 + 1`, а лучше payload с литералом `9007199254740993`; текст п. 6 карточки, КД §3.2 и §8, README. Мутант «предел `<= 1<<53`» должен покраснеть на случаях 2^53 |
| Mi-1 | Minor | `shared/entity/ops_test.go:600` (`TestChangedListAndStateHashMoveTogether`), `shared/entity/changed_test.go:334-419` | Три решения по ходу (элемент предка, добавление без дедупликации, удаление отсутствующего) держатся только на генераторе с фиксированным зерном. Именованных случаев нет: `grep fresh` в `ops_test.go` пуст. Сменится набор путей, значений или зерно — покрытие пропадёт молча. Мутант M5 красный сейчас лишь потому, что генератор случайно дошёл до контрпримера. Эти же случаи — эталонные векторы для догона T-055 (§4.8) и read-model EPIC-003/004 | Добавить именованные строки. (1) `inc fresh.deep -3` + `remove fresh.deep` → `changed == [{path: fresh, new: {}}]` без `old`, хеш сдвинулся; вариант `fresh = "s"` → `old: "s"`. (2) `append inventory {colour: red}` + `set inventory[1] = {item_id: item-1}` при `inventory = [{item_id: item-1}]` → догон воспроизводит хеш. (3) `set inventory[0] "x"` + `set inventory[0].kind 3` + `remove inventory[0].kind` → догон воспроизводит хеш |
| Mi-2 | Minor | `shared/entity/ops.go:59`, `:62`, `:73-88` | У наличия два источника: `OldPresent() = HasOld \|\| Old != nil`. `Change`, собранный вручную для `null` без флагов (`Change{Path: "killed_by"}` — удаление ключа, где лежал `null`, или `set` в `null`), кодируется как `{"path":"killed_by"}` (зонд). Схема такое отвергает (`anyOf`), и ошибка всплывёт у `Publish` без указания на источник. Обратное тоже возможно: `HasOld: false` при непустом `Old` считается наличием вопреки флагу. Тип публичный, его будут собирать T-055 и EPIC-003/004 | Минимум: `MarshalJSON` возвращает ошибку `entity: change %q: neither old nor new`, если нет ни того ни другого, — дефект виден у издателя с путём. В doc-комментарии типа прямо сказать, что для `nil` решают только флаги. По желанию — конструктор `NewChange(path, old, hadOld, new, hasNew)` |
| N-1 | Nit | `shared/entity/ops.go:102-127` | `UnmarshalJSON([]byte("null"))` обнуляет `*c`, а соглашение `encoding/json` для `Unmarshaler` — no-op на `null`. Для элементов среза разницы нет | `if bytes.Equal(bytes.TrimSpace(data), []byte("null")) { return nil }` |
| N-2 | Nit | `shared/entity/ops.go:280-282` | Текущее значение за пределом получает `ReasonNotJSON` («value is not JSON-compatible»), хотя виновато не значение операции, а атрибут. `float64` текущего вне `int64` (например `1e300`) получает `ReasonNotNumber`. На проводе оба — `invalid_op`, но разработчика текст уводит не туда | Отдельная причина (например `ReasonCurrentPastRange`) или уточнённый текст; на провод не влияет |
| N-3 | Nit | `tasks/T-448.md:130`; `state-and-mechanics.md:394` | «для `n = 0` — и когда списка нет». `append` к пути со значением `null` тоже создаёт список (`asAnySlice(nil)`), и помощник теста так и догоняет (`ops_test.go:855-877`, `current == nil`). Текст об этом молчит | «…когда списка нет или по пути `null`» |

### Риски и допущения

- **contract-change.** Ревью system-architect#1 обязательно, это ревью его не заменяет. Его решение по T-448 появилось в карточке во время ревью. С Ma-1 оно совпадает (п. 4). Пп. 1–3 и 5 совпадают с оценкой кода в «Проверено», пп. 1–4. Остальные пункты «Что сделать в итерации 2» — `entity.created.proposal_id` в `required`, проверка `attributes`, КД §4.5/§9, `a[n]` за концом — проверяются в ревью #2.
- `-race` недоступен (нет cgo). Новые функции чистые, без горутин.
- Интеграционные тесты, e2e, `golangci-lint` и `make test` не запускались: в рабочей папке — три пакета, на объединённой копии — `go test -short ./...`. Результаты e2e и lint взяты из карточки. Два нестабильных теста, отмеченных разработчиком, T-448 не затрагивает.
- При слиянии в эпик `dev-log.md` и `review.md` дописываются драйвером appendtail. README фикстур и КД на копии легли без конфликта.
- EPIC-003/004 несут в `shared/contracts/validate_test.go` тот же пример `entity.create.proposed` без `proposal_id`. После слияния EPIC-002 в develop эту строку подтянет их синхронизация. Новые тесты этих эпиков с create без `proposal_id` упадут на схеме, и это ожидаемо.
- Зонды и мутанты — только в копии в scratch (`cr2-t448-merged`), удалена по точному пути; в рабочую папку, кроме этого раздела и строки карточки, ничего не добавлено.

### Предложения в бэклог

1. Правило догона — одной production-функцией `shared/entity` (например `ApplyChanged(attrs, []Change)`), а не помощником теста `replayChanged` (`ops_test.go:828`). Иначе тонкое правило реализуют трижды: догон State §4.8 (T-055), read-model Swarm и Gateway (C-14). Тест-свойство тогда проверяет саму функцию. Решение — system-architect, XS–S.
2. ~~`attributes` у `entity.create.proposed` без проверки ±2^53~~ — закрыто решением system-architect#1: итерация 2, п. 5.
3. T-055: предложение без `proposal_id` на нестрогой шине — поведение нормативно по решению system-architect#1, п. 5. Сверх него предлагаю счётчик или метрику: молчание издатель иначе не диагностирует.
4. Поддерживаю п. 5 разработчика (нестабильные `TestTheProbeGivesUpOnAProcessThatExited`, `TestTheProcessRunsTheFightsOfIAlpha/fight-00`).

## T-448 · ревью #2 · 2026-09-13 · code-reviewer#2 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-448`, ветка `task/T-448-changed-form-proposal-id`, база `03c551d`, коммитов нет, ревьюировалась рабочая копия (16 файлов в `git diff`, неотслеживаемые `shared/entity/changed_test.go` и `tasks/T-448.md`). Это повторная итерация. Проверены исправления по ревью #1 (Ma-1, Mi-1, Mi-2, N-1…N-3), пункты 1–6 из «Что сделать в итерации 2» (раздел «Решение system-architect#1» карточки, нормативно), правка R2-Mi-2 из ревью T-449 в тексте v1.6, а также регрессия от этих правок. Раздел решения архитектора не трогался.

Основание: мой раздел «T-448 · ревью #1»; в карточке — «Итерация 2», «Решение system-architect#1», «Текст C-02 v1.6 для T-449»; запись `<!-- dev-log T-448 -->` (итерация 2). Правки КД, `tasks.md`, карточки и `dev-log.md` внутри ветки задачи входят в состав задачи и замечанием не считаются.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 3 · Nit: 2.

Ma-1 закрыт во всех местах: граница, тесты, КД, README и комментарии. Mi-1, Mi-2 и N-1…N-3 закрыты. Пункты 1–6 решения архитектора выполнены. Регрессий нет: ветка задачи зелёная, EPIC-002 HEAD с патчем зелёный, синхронизации EPIC-003 и EPIC-004 (симуляция) тоже зелёные. Три Minor не блокируют. Два из них — пробел теста двойника и устаревшая ссылка в карточке — правятся за минуты, до слияния или отдельной правкой. Третий (Mi-5, правка в зоне EPIC-003) требует решения оркестратора до слияния, см. «Открытые вопросы».

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Ma-1: граница `\|x\| ≤ 2^53−1` | `maxSafeInteger = 1<<53 - 1` (`ops.go:506`). `intInSafeRange` сравнивает включительно с 2^53−1 (`:564`), ветка `ParseFloat` — `math.Abs(f) <= maxSafeInteger` (`:560`; 2^53−1 во `float64` точна). В `inc` проверяются приращение (`:276`), текущее (`:283`, `:295`) и результат после clamp (`:299`). Сумма двух чисел из диапазона не переполняет `int64`. Литерал `9007199254740993` через JSON → `float64(2^53)` → отказ: `changed_test.go:515-528`; через двойник — `apply_test.go:585` (`overTheBus`, сверка, что значение стало `float64(2^53)`, затем `invalid_op`, значение не записано). Обратная сторона: `-(2^53−1)` через шину применяется (`apply_test.go:607`) |
| 2 | N-2: `ReasonCurrentRange` | Не ломает причин. `float64` целое за `int64` (`1e300`) и `uint64` за `int64`: `asInt64` не читает, затем `numberPastSafeRange` → `ReasonCurrentRange`. Дробное `2.5` → `ReasonNotNumber`, как было. `json.Number` с мусором и `NaN`: `json.Marshal` падает → `ReasonNotNumber`. Строки причин нигде, кроме `ErrInvalidOp.Error`, не читаются (`grep` по дереву и по `epic/EPIC-003`, `epic/EPIC-004`). На проводе всё — `invalid_op` |
| 3 | Mi-1 | Три именованных теста есть и содержательны: `TestChangedReportsTheAncestorAProposalCreated` (3 подслучая), `TestCatchingUpAppendsWithoutDeduplication` (с контрольной проверкой, что догон через `op: append` дал бы другой хеш), `TestCatchingUpSkipsAPathAnEarlierEntryAlreadyRemoved` (с проверкой, что `remove` после первого элемента — `ReasonMissingPath`). Каждый через `wantCaughtUp` сначала убеждается, что хеш сдвинулся |
| 4 | Mi-2 и строгость `MarshalJSON` | `OldPresent`/`NewPresent` удалены, в коде ссылок нет. `MarshalJSON` отказывает при «ни одного флага» и при «значение без флага» (`ops.go:71-78`), ошибка содержит путь. **Последствия для других эпиков** (`git grep` по `epic/EPIC-003-swarm-llm-laws` `3141755`, `epic/EPIC-004-gateway-bot` `1630fe7`, `develop`): `entity.Change` вручную никто не собирает. `FakeEncounter` (`fake_encounter.go:1793`/`:1785`) и `FakeNarrator` (`fake_narrator.go:914`) только декодируют, флаги ставит `UnmarshalJSON`. `Change` из `ApplyOps` противоречивым не бывает: `hadOld == hasNew == false` отсекается проверкой no-op, `Old`/`New` при отсутствии пути — `nil`. Единственный ручной литерал без флагов — `shared/entity/entity_test.go:51` (зона EPIC-002), и он не кодируется. Строгость ловит дефект у издателя, а не в схеме у `Publish`. Для T-055 и потребителей EPIC-003/004 это плюс. Одна висящая ссылка — Mi-4 |
| 5 | N-1, N-3 | N-1: `null` (с пробелами) — no-op, тест `a_null_in_place_of_the_object`. N-3: «или по пути списка `null`» есть в карточке (текст v1.6, п. 4), в КД §4.8 и в README `shared/entity`; помощник `catchUpOp` (`ops_test.go`, `case nil`) так и догоняет |
| 6 | Решение, п. 4: `entity.created.proposal_id` обязателен | Схема: `required` + `minLength: 1` + описание. Фикстуры: валидная несёт поле, у невалидной одна ошибка (`version: 0`), `test/fixtures` ok. Пример в `shared/contracts/validate_test.go:191-195`. Двойник пишет поле всегда (`apply.go:131-136`). Тест `TestCreatedSchemaRequiresTheProposalID`: есть / нет / пусто. `mvctl contracts check` — 65 типов, 8 топиков, 58 файлов схем |
| 7 | Решение, п. 5: `entity.JSONCompatible` для `attributes` | `apply.go:108`, до `seen` и `duplicate_entity`, отказ `invalid_op` с `entity`. Тест `TestCreateWithANumberPastTheRangeIsInvalidOp`: сущности нет, `entity.created` нет. Порядок «до дедупликации» разумен: это ошибка формы (§4.5 п. 1). Повтор уже применённого создания не может нести недопустимые атрибуты, поэтому порядок на исход не влияет. Тестом он не закреплён (N-4) |
| 8 | Решение, пп. 1–2: правило догона | КД §4.8, README и текст v1.6 п. 4 согласованы. Промежуточный узел, которого нет или который не контейнер, заменяется объектом. `null` по пути списка — отсутствующий список. `a[n]` за концом — `state_divergence` (строка добавлена и в §9). `replayChanged` роняет тест на `errCorruptFact`; `TestCatchingUpRefusesAnElementPastTheEndOfItsList` — 7 случаев |
| 9 | Порядок элемента предка (R2-Mi-2 ревью T-449) | Правило вписано в текст v1.6 (п. 4), в КД §3.2 и в README. Тест `changed_test.go:685` сравнивает форму целиком (`reflect.DeepEqual`) и воспроизводит хеш. **Наблюдение, не замечание.** Мутант R10 (элемент предка первым) краснеет только на этом сравнении формы. 4000 случайных предложений дают тот же хеш и при обратном порядке: каждый элемент читается из итогового состояния, поэтому догон сходится при любом положении предка. Порядок — соглашение формы, закреплённое нормой архитектора, а не условие сходимости хеша. Норме это не противоречит. Фраза «догон применяет элементы по порядку, и элемент предка переписывает…» объясняет порядок сильнее, чем он того требует. Править не прошу |
| 10 | Решение, п. 3: без `proposal_id` — `Warn` и пропуск | КД §4.5 п. 1 (новый абзац) и строка §9; двойник `refuseMalformed` (итерация 1), тесты create/update |
| 11 | Правка вне зоны: `shared/testkit/swarm/fake_narrator_test.go:1206` | Строка корректна: `"proposal_id": "create-" + id` у помощника `created()`. Он публикуется через шину с проверкой в `TestTheNarratorWorksOffTheBusToo` (`fake_narrator_test.go:932`). Симуляция в клоне в scratch: синхронизация EPIC-003 = `epic/EPIC-003` + `epic/EPIC-002` (`24f1baf`) + патч T-448. **Без этой строки** падает ровно один тест, `shared/testkit/swarm` `TestTheNarratorWorksOffTheBusToo` (`missing property 'proposal_id'`), с ней зелёный весь `go test -short ./...`. Других тестов EPIC-003/004, публикующих `entity.created` без `proposal_id` через шину, нет: `createdFact` (`fake_encounter_test.go:2000` в EPIC-003, `:1929` в EPIC-004) отдаётся в `enc.Observe` напрямую, без схемы. `born` (`shared/testkit/gateway/reaction_test.go:536`) складывается в харнесс напрямую и схеме не соответствует и так (`cause`, `applied_at`). `Harness.CreatePlayer`, `FakeEncounter` и bootstrap-тест `cmd/multiverse` поле передают. Помощник `created()` одинаков в EPIC-003 и EPIC-004 и после `1c2ee7e` не менялся, слияние строки чистое. Про зону — Mi-5 |
| 12 | КД State: §3.2, §4.1, §4.5, §4.8, §8, §9, §14; README `shared/entity` и фикстур | С текстом v1.6 в карточке согласованы: граница `2^53−1`, три проверки `inc`, `attributes` создания, `proposal_id` в обоих предложениях и в `entity.created`, пропуск без `proposal_id`, форма `changed[]`, элемент предка и его порядок, правило догона с `null`-списком и `a[n]` за концом. `§3.2 «reflect.DeepEqual»` не тронут — его вносит T-449 |
| 13 | Ожидаемый конфликт с T-449 (КД State) | T-449 уже влит в `epic/EPIC-001-foundation` (`647c5d8`). `git merge-file` КД (база `1c2ee7e`, переводы строк нормализованы) даёт **один** конфликтный блок: §3.2, строки таблицы ops `set`/`inc`/`append`/`remove` и абзац «Пути — грамматика…» сразу под таблицей. T-448 правит `set` и `inc` и дописывает в абзац «Форму элемента». T-449 правит `append`, `remove` и в том же абзаце заменяет `reflect.DeepEqual` на каноническое сравнение. Разрешать объединением: строки `set`/`inc` — от T-448, `append`/`remove` — от T-449, абзац — правка T-449 плюс хвост «Форма элемента…» от T-448. §8 (строки C-02 вход/выход у T-448, C-03 v1.3 у T-449) и §3.3, §4.1, §4.10, §5 сливаются без конфликта |
| 14 | Прогоны в рабочей папке (go1.26.8 windows/amd64) | `go build ./... && go vet ./...` — 0; `go run ./cmd/mvctl contracts check` — 0; `go test -short -count=1 ./...` — 0, 27 пакетов ok; `golangci-lint run ./...` — 0 issues |
| 15 | Совместимость | (а) Копия `epic/EPIC-002-state-mechanics` `24f1baf` (T-053, T-060, слияние develop) + патч: build, vet, `contracts check`, `go test -short ./...` — 0. (б) Клон: EPIC-003 + EPIC-002 + патч — 0. (в) Клон: EPIC-004 + EPIC-002 + патч — 0. При этом `testdata/fixtures/events/README.md` конфликтует между EPIC-002 (T-052) и EPIC-004 (T-301) и **без T-448**. В симуляции конфликт разрешён объединением, а строка T-448 (`entity.update.proposed` в таблице дефектов) лежит внутри этого конфликтного блока |
| 16 | Мутанты (независимо; копия дерева в scratch `t448r2-…`, без `-overlay`; после каждой мутации файл восстановлен, SHA-256 сверен; `ops.go` копии сверен `cmp` с рабочей папкой) | **R0 (контрольный)** `func broken( {` → красный (`ops.go:563:14: syntax error`). R1 нижняя граница `-maxSafeInteger-1` → красный. R2 ветка `ParseFloat` без `math.Abs` → красный. R3 текущее за `int64` без `ReasonCurrentRange` → красный. R4 предок-скаляр не считается созданным → красный (генератор и именованный вектор). R7 схема `entity.created` без `proposal_id` в `required` → красный. R8 `inc` без проверки приращения → красный. R10 элемент предка первым → красный (только сравнение формы, п. 9). **Выжили:** R5 `changedPayload` пишет `old` по `c.Old != nil`, R6 — `new` по `c.New != nil` (Mi-3); R9 проверка `attributes` после `seen`/`duplicate_entity` (N-4) |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-3 | Minor | `shared/testkit/state/apply.go:450-462`; `shared/testkit/state/apply_test.go:489` | `changedPayload` — второй кодировщик формы рядом с `Change.MarshalJSON`, и присутствующий `null` в двойнике не проверен. Мутант R6 (`new` по `c.New != nil`) выживает: `set` существующего ключа в `null` вышел бы как `{path, old}`. Схема такое принимает, а догон потребителя удалил бы ключ вместо записи `null` — ровно дефект, ради которого сделана v1.6, и e2e на двойнике его бы не заметил. R5 (`old` по `c.Old != nil`) тоже выживает: `remove` ключа, где лежал `null`, дал бы `{path}` — это ловит схема, но только у `Publish`. Зонд в копии: сейчас двойник пишет верно — `[{"new":null,"path":"nul"}]`, `[{"new":null,"old":"fox","path":"mark"}]`, `[{"old":null,"path":"nul"}]` | В `TestChangedCarriesOldAndNewByPresence` (или отдельным тестом) добавить три строки с `null`: `set` отсутствовавшего пути в `null` → только `new`; `set` существующего в `null` → оба; `remove` ключа со значением `null` → только `old`. Альтернатива — строить элемент через `Change.MarshalJSON`/`UnmarshalJSON`, чтобы кодировщик был один |
| Mi-4 | Minor | `tasks/T-448.md:157` («Строки для других эпиков», EPIC-003) | Инструкция для EPIC-003 ссылается на удалённый в этой же итерации метод: «Условие: `!change.NewPresent()`». По ней код не соберётся | Заменить на `!change.HasNew`. Смысл строки не меняется, поэтому оговорка решения «строки выше не меняются» здесь не мешает |
| Mi-5 | Minor (процесс; по букве правила ревьюера это Major, см. «Почему») | `shared/testkit/swarm/fake_narrator_test.go:1206`; `tasks.md`, раздел T-448, «Не трогать: `shared/testkit/{swarm,gateway}`» | Правка в файле EPIC-003 (`ownership.md`, строка `shared/testkit/**`: `FakeNarrator` — EPIC-003). Решение архитектора (п. 4 итерации 2) её прямо не называет, и в «Не трогать» задачи этот каталог перечислен. *Почему не Major:* сама строка верная (п. 11), тестовая и одна. Её вынуждает сужение схемы по решению архитектора: без неё ветка красная, иначе исполнитель не мог. Это тот же род правки, что пример в `shared/contracts/validate_test.go` (зона EPIC-001), который архитектор принял. Слияние с EPIC-003/004 чистое | До слияния оркестратор фиксирует в карточке согласие владельца (tech-lead EPIC-003) или system-architect на эту строку. Без такой записи замечание становится Major и строку нужно передать в EPIC-003, но тогда при синхронизации `TestTheNarratorWorksOffTheBusToo` покраснеет, пока EPIC-003 её не внесёт |
| N-4 | Nit | `shared/testkit/state/apply.go:108-122` | Порядок «проверка `attributes` до `seen`/`duplicate_entity`» (отклонение 4 итерации 2, КД §4.5) тестом не закреплён: мутант R9 выживает. На исход для потребителя порядок не влияет (п. 7) | Если порядок нормативен для T-055 — тест: создание уже существующей сущности с числом 2^53 в `attributes` → `invalid_op`, а не `duplicate_entity`. Если нет — в КД §4.5 не называть порядок |
| N-5 | Nit | `schemas/events/entity.created.v1.json` | Смешанные переводы строк в рабочей копии: 15 CRLF и 4 LF у добавленных строк. При коммите `text=auto` нормализует, в индексе будет LF | Ничего, либо сохранить файл с единым окончанием |

### Открытые вопросы (оркестратору)

1. Mi-5: подтвердить строку в `shared/testkit/swarm/fake_narrator_test.go` записью в карточке (tech-lead EPIC-003 или system-architect) или передать её EPIC-003.
2. **Текст v1.6 в `contracts.md` отстаёт от итерации 2 T-448.** T-449 влит в `epic/EPIC-001-foundation` (`647c5d8`) с текстом из итерации 1. В `contracts.md` (C-02) нет трёх вещей, которые теперь есть в карточке T-448 и в КД State: (а) правила «элемент предка идёт после элементов путей, порядок — часть формы» (строка :336, R2-Mi-2 ревью T-449); (б) «или по пути списка `null`» в правиле догона (:338, N-3); (в) именованных векторов в «Гарантии» (:340). После слияния эпиков КД State и контракт разойдутся в норме (а). Нужна отдельная правка `contracts.md` в EPIC-001 — вне зоны T-448.

### Риски и допущения

- `contract-change`: решение архитектора в карточке нормативно, ревью кода его не заменяет и не пересматривает.
- Симуляции синхронизации EPIC-003/004 выполнены в клоне `git clone --shared` в scratch. Конфликт README фикстур между EPIC-002 и EPIC-004 разрешён там объединением, только для прогона. При настоящей синхронизации его разрешает владелец слияния; строка T-448 в том же блоке.
- `-race` недоступен (нет cgo). e2e, `make test` и интеграционные тесты не запускались, результаты e2e и `make test` взяты из карточки. Docker, стенд `:8888` и `.env` не трогались.
- В ходе ревью неудачный heredoc запустил интерактивный `python -` из моей команды. Процесс (PID 44052, потомок моей оболочки, опознан по дереву процессов и командной строке) остановлен. Рабочая папка T-448 и чужие процессы не затронуты, файл копии сверен `cmp`.
- Копия дерева, клон и зонды — только в `scratchpad/t448r2-…`; папка удалена по сохранённому точному пути. В рабочую папку, кроме этого раздела и строки карточки, ничего не добавлено.

### Предложения в бэклог

1. Остаётся п. 1 ревью #1: правило догона одной production-функцией `shared/entity` (например `ApplyChanged`), а не помощником теста `catchUp`. Иначе T-055, read-model Swarm и Gateway реализуют его трижды. Теперь это ещё и эталон для `errCorruptFact` → `state_divergence`. Решение — system-architect, XS–S.
2. Правка `contracts.md` C-02 v1.6 в EPIC-001 по «Открытым вопросам», п. 2 (XS, docs).
3. `changedPayload` двойника строить из `Change.MarshalJSON`, чтобы кодировщик формы был один (закрывает Mi-3 по существу; T-056 заменяет двойник — учесть там).

## T-055 · ревью #1 · 2026-09-13 · code-reviewer#3 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-055`, ветка `task/T-055-state-pipeline`, база `34bdcca`. Коммитов нет, ревьюировалась рабочая копия: `git diff 34bdcca` (10 файлов) и неотслеживаемые `internal/state/**`, `cmd/multiverse/{contexts_state,contexts_state_test,sources_test}.go`. Правки `cmd/multiverse/*` сделаны в мягком режиме, отметку ставит tech-lead#1. Это ревью её не заменяет.

Основание: раздел T-055 в `tasks.md` (сверка 2026-09-13: C-01 v1.5–v1.7; приёмка T-448; строки T-060 Н-1/Н-2/N-4), карточка `tasks/T-055.md`, запись `<!-- dev-log T-055 -->`, КД `state-and-mechanics.md` §4.1–§4.8, §8, §9, `contracts.md` C-01/C-02, двойник `shared/testkit/state`. Правки `Docs/dev-team` внутри ветки (карточка, `tasks.md`, `dev-log.md`) входят в состав задачи, замечанием не считаются.

### Вердикт

**Вернуть.** Critical: 0 · Major: 1 · Minor: 4 · Nit: 3.

Конвейер сделан аккуратно. Копии сущностей, единственный писатель на мир, `recover` на границе worker'а, посредник доставки, порядок остановки, `WithCauseID` и форма `changed[]` соответствуют DoD. Источники конструкторов в `serve.go` стоят одним блоком и снимаются после закрытия шины. Прогоны зелёные, рецепт синхронизации с T-446 проверен слиянием. Возврат — из-за одного пункта, Ma-1. Правило «при падении публикации ничего не сохраняется» верно, пока шина повторяет доставку. Когда шина отправляет предложение в `dead_letters`, уже вышедшие факты остаются на шине, хотя мир не изменился. Следующее предложение публикует **вторую, другую** версию той же сущности с тем же номером. Зонд это воспроизводит.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Атомарность и повтор при сбое публикации (`apply.go:149-155`, `:224-233`) | Порядок «весь ответ опубликован → копии в `memstore` → `proposal_id` в окне» выдержан. При ошибке публикации мир и окно не меняются, повтор той же доставки решается заново с теми же id (`TestAFailedPublicationIsRetriedWithTheSameIDs`). `WithCauseID(e.ID)` — у фактов, `WithCauseID(ref.ID или "")` — у отказов. Версии строго +1: `entity.New` даёт v1, `Commit` без изменений версию не двигает. Правило C-02 v1.3 стоит до окна `proposal_id` (§4.5 п. 1а), включение сверх плана обосновано: без него два набора по одной сущности дают два факта под одной версией. **Дыра — после `dead_letters`, см. Ma-1** |
| 2 | Паника → `ErrWorldStopped` шине | Перехват в `worker.run` (`worker.go:102-116`), лог `Error` со стеком и `handled=false`, `/health fail` по миру, следующие предложения мира получают `ErrWorldStopped` без вызова `Applier`. Решение «ошибка, а не повторная паника» записано в `dev-log.md` с доводом про `service_panics`. DoD оставляет выбор исполнителю. **Риск исполнителя подтверждён зондом**: с паузами по умолчанию (`DefaultBackoff`) два предложения остановленного мира задержали предложение второго мира на **5,2 с**, по 2,6 с на каждое. Тест этого не видит: у `newBus` паузы нулевые (`helpers_test.go:248`). **Оценка: для MVP-1 приемлемо, это не дефект T-055.** По умолчанию в процессе один мир (`MV_STATE_WORLDS=dark-forest-world`). Остановленный мир — это уже `/health fail`, и процесс перезапускают. Повторы при этом ничего не дают: мир внутри процесса не оживает. Для процесса с несколькими мирами это неприемлемо, у C-01 нет «окончательной» ошибки без повтора. Предложение — в бэклог, п. 1 |
| 3 | Остановка (`context.go:215-241`) | `Stop` сначала отменяет свой контекст подписки (`WithCancel(WithoutCancel(ctx))`), ждёт возврата обработчика и только потом останавливает worker'ы. Принятое предложение worker доводит на `context.WithoutCancel`. `Stop` ограничен своим `ctx`. Тест фиксирует отмену подписки до `Close`. **`Stop`, не дождавшийся подписки**, возвращается на `:228-230` и не доходит до цикла worker'ов: горутины worker'ов живут до выхода процесса. Для бинарника это безразлично, процесс после `StopAll` закрывает шину и выходит. Опасен повторный `Start` того же экземпляра, см. Mi-2 |
| 4 | Гонки (без `-race`, разбор доступов) | Поля `Context` — под `c.mu`. `workers`/`worlds` заменяются целиком, `Health` читает снимок. `Dedup` со своим мьютексом; `Has`/`Add` посредника вызываются одной горутиной подписки, по событию за раз: `Delivery` синхронна, у kafka-адаптера один reader на группу. `Applier` вызывает только горутина worker'а. `memstore` отдаёт и принимает копии под `RWMutex`. Сущности плана — клоны до `Put`, который клонирует ещё раз. `worker.failure` — под `w.mu`. Пока экземпляр запускается один раз, гонок не нашёл. Исключение — повторный `Start` после `Stop` по таймауту (Mi-2). Окна реального времени в тестах: ожидания — опрос с верхней границей 60 с; бюджет 50 мс в `TestStopIsBoundedByItsContext` — это бюджет, а не окно: `Stop` не может вернуться раньше, worker держится хуком. Для медленного раннера под `-race` запас достаточен |
| 5 | `serve.go`: источники, Н-1, Н-2 | `installSources(deps)` (`:301`) стоит до `openBus` и `StartAll`. `defer installSources(runtime.Deps{})` (`:302`) объявлен до `defer closeBus` (`:315`), поэтому по LIFO срабатывает после закрытия шины. Обе ветки режима, live и replay, берут часы из `deps.Clock`. Прежний блок только для replay снят. `SetRegistry(nil)` вместо nil-указателя в интерфейсе — верно. Н-1: `clock_start` = `times.start` (`:359`). Н-2: справка `--recording` говорит про начало часов. Тест источников — после `testkit.Deterministic`, в обоих режимах. Порядок «сброс после `closeBus`» тестом не закреплён (N-1) |
| 6 | `memoryOptions().idSource = uuid` | Касается только тестов пакета `cmd/multiverse`: `test/e2e` процесс не запускает и держит `testkit.Deterministic`. Флаг `--id-source` и его значение по умолчанию (`uuid`) не менялись, на replay продукта это не влияет. Бои I1-α зависели от планировщика и раньше (комментарий `fake_contexts_test.go:290-294`), детерминизма они не теряют |
| 7 | Стенд боёв I1-α: `state` процесса на `world-of-no-stand` (`fake_contexts_test.go:390`) | Дефекта T-055 это не маскирует: без строки два State отвечают на одно предложение (мутант M22 исполнителя). Но настоящий State в боях через процесс не участвует. Уровень процесса проверяет только `TestTheProcessRunsTheStateOfEPIC002` — одно создание. Издатели предложений стенда (`FakeEncounter`, `Harness`) строят события через `Derive`/`root` от событий мира, `world` в конверте у них есть. Риск остаётся до T-056 (бэклог исполнителя) |
| 8 | `MV_STATE_WORLDS` | `shared/env/vars.go` — объявление с описанием. `.env.example`: 268 строк CRLF, 0 LF-only. `go run ./cmd/mvctl env check` — 68 переменных, 0. `make compose-lint` — 0 (52 bad, 10 good). Переменная не `[required]`, у неё есть значение по умолчанию |
| 9 | Совместимость с кончиком эпика `cc9a63c` (T-446/T-449) | Пробное трёхстороннее слияние в scratch `t055rev-merge` (дерево `cc9a63c` + `git merge-file` по изменённым файлам T-055, база `34bdcca`). Конфликт в `contexts.go` ожидаемый, add/add в `contexts_state.go` — тоже. `fake_contexts_test.go`, `main_test.go`, `serve.go`, `serve_test.go`, `vars.go` слились без конфликта. Конфликт `.env.example` — артефакт пробы: рабочая копия в CRLF, `git show` в LF. Между `34bdcca` и `cc9a63c` файл не менялся, настоящее слияние будет чистым. Рецепт карточки, пп. 1–2, применён: `contexts.go` — от T-446; `contexts_state.go` — T-446, строка `newStateContext = func() runtime.Context { return state.New(state.Config{}) }`, константа `stateContext` оставлена. После этого `go build ./...`, `go vet` — 0. `go test -count=1 ./cmd/multiverse/ ./internal/state/... ./shared/runtime/` — ok, включая `shutdown_test.go` и `contexts_test.go` T-446 (`TestEveryFactoryBuildsTheContextOfItsName`). `go test -tags e2e ./test/e2e/...` с новым `TestMain` — ok. П. 3 рецепта (T-454 рядом с `fightThroughTheProcess`) в `cc9a63c` не нужен, строка легла сама. Комментарий-инструкция в `contexts_state.go:14-18` после слияния устареет (N-2) |
| 10 | Прогоны в рабочей папке (go1.26, windows/amd64) | `go build ./... && go vet ./...` — 0. `go test -short -count=1 ./internal/state/... ./cmd/multiverse/... ./shared/testkit/...` — 10 пакетов ok. `go test -short -count=20 -cpu 1,4 ./internal/state/...` — ok (8,5 с). `go test -tags e2e -count=1 ./test/e2e/...` — ok (12 с). `golangci-lint run ./...` — 0 issues. `mvctl contracts check` — 65 типов, 8 топиков, 58 схем |
| 11 | Мутанты (независимо; копия дерева в scratch `t055rev-tree`, без `-overlay`, контрольный первым; после каждой мутации исходник восстановлен) | **C0 (контрольный)** `version` факта `+1` → красный (4 теста). **X7** update без окна `proposal_id` → красный. **Выжили:** X1 — посредник без окна id событий (Mi-4); X2 — у всех отказов пакета одна `WithCauseID`-часть (Mi-4); X3 — `plan` без проверки пустых `ops` (схема не пропускает, путь только при прямом вызове — не замечание); X4 — create: `Put` до публикации (Mi-3); X6 — подписка на контексте `Start` (процесс отменяет его сигналом до `StopAll`, подписка всё равно отменяется до `Close` — не замечание); X8 — create без окна `proposal_id` (Mi-3); X9 — `Stop` не останавливает worker'ы (Mi-2); X10 — отвергнутый целиком неатомарный пакет запоминается (поведение не нормировано, не замечание); X11 — сброс источников до `closeBus` (N-1, полный `go test ./cmd/multiverse/`) |
| 12 | Зонды (в той же копии, временный тестовый файл) | **P1** — Ma-1. **P2** — задержка второго мира (п. 2) |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Ma-1 | Major | `internal/state/apply.go:224-233` (update), `:149-155` (create), `:214-223` (факты раньше отказов); `internal/state/context.go:203-205`; КД `state-and-mechanics.md` §9, строка «Ошибка `Publish` факта после PUT», и строка «Факт журнала с версией ≠ mem+1» | **Фантомный факт после `dead_letters`.** Ответ публикуется по событию. Если одно из событий пакета не выходит, `Applier` возвращает ошибку, и шина повторяет доставку ×3 с теми же id — это верно. После четвёртой попытки шина паркует предложение и **фиксирует офсет**, а вышедшие события остаются на шине. Мир при этом не изменился. Следующее предложение решается от старой версии и публикует другой факт под тем же номером версии. **Зонд P1** (`membus`, контекст целиком): пакет `player-A inc hp -2` + `wolf-alpha inc hp -3`, публикация факта волка падает всегда. На шине 4 копии `entity.updated player-A v2 hp 10→8` (id `12409868-…`), затем `dead_letters`. Следующее `prop-next inc hp -1` даёт `entity.updated player-A v2 hp 10→9` (id `d54b7ace-…`), State держит `player-A v2 hp 9`. Потребитель read-model (Gateway, Swarm), который применяет «версия больше известной», примет первый v2 и отбросит настоящий, и расхождение останется навсегда. Восстановление по журналу (§4.8, T-059) увидит два факта v2 подряд, то есть `state_divergence`. Это отступление от §9. Там после PUT мир **изменён**, факт помечается неподтверждённым и досылается позже, так что опубликованные факты всегда подмножество состояния. Здесь инвариант обратный, и на пути `dead_letters` он теряется. **Второй путь, детерминированный** (только при `MV_BUS_VALIDATE_ON_READ=false` или прямом вызове): неатомарный пакет, набор A применим, у набора B есть `entity.id`, но нет `entity.type`. Факт A выходит, отказ B со `entity.type: ""` не проходит схему при `Publish`, после четырёх попыток факт A остаётся фантомом | Решение принять с tech-lead, записать в `dev-log.md`, закрепить тестом по сценарию P1. **Вариант А (минимальный, в духе `persist_failed` §9).** `Applier` помнит незавершённый ответ: id события-предложения и то, что публикация начиналась. Приходит `Apply` с **другим** id события при незавершённом ответе — значит, шина ушла дальше. Тогда мир останавливается: `ErrWorldStopped` с причиной `publish_failed`, `/health fail`, `Error handled=false` с `proposal_id` и id вышедших событий. Консервативно — при любом незавершённом ответе: ошибка kafka-писателя не доказывает, что запись не легла. Тест: после `dead_letters` второго `entity.updated player-A v2` нет, мир `fail`. **Вариант Б.** Worker сам повторяет публикацию того же ответа до успеха или до отмены `Stop`, шине ошибку не отдаёт. Шина стоит, пока брокер недоступен, что при недоступном брокере так и так. Для детерминированного пути дополнительно: отказы публиковать до фактов или нормализовать `Ref` отказа по сущности мира |
| Mi-1 | Minor | `internal/state/apply.go:92-97`; `internal/state/context.go:195-201`; CLAUDE.md, «Конверт события» | Предложение **без `world` в конверте** пропускается на уровне `Debug`. Издатель, забывший `WithWorld`/`worldID` в `NewRoot`, не получает ни факта, ни отказа, а в журнале уровня Info нет ни строки. Шина такое событие пропускает: `ValidateEnvelope` `world` не требует (`registry.go:111-146`). Кроме того, мир читается через `GetWorldIDFromEvent` с legacy-фолбэком из payload, а правило проекта требует в новом коде читать `Event.World` напрямую | Читать `ev.World` напрямую. Пустой мир — `Warn` с `event_id`, `type`, `proposal_id` («proposal without world in the envelope: no worker is addressed»). Чужой мир остаётся на `Debug`: это законный случай нескольких процессов State. Поведение по существу (пропуск или применение) — вопрос system-architect, см. ниже |
| Mi-2 | Minor | `internal/state/context.go:222`, `:228-230`, `:132`, `:157-159`; `internal/state/worker.go:135-143` | `Stop` по таймауту выставляет `running=false` и возвращается, не закрыв `quit` worker'ов. Горутины worker'ов не останавливаются никогда: выживший мутант X9 показывает, что это не проверяется. Повторный `Start` того же экземпляра, который прямо разрешён (комментарий `:102-105`), создаёт новые worker'ы над **теми же** `Applier` из `c.appliers`. Старый worker ещё может быть внутри `Applier.Apply`, и тогда у мира два писателя: чередование `store.Get`/`Put` даёт два факта одной версии. Старая горутина подписки, вернувшись, перезапишет `c.subErr` нового запуска. В процессе экземпляр стартует один раз, поэтому Minor | При таймауте всё равно закрыть `quit` всех worker'ов (без ожидания). Держать признак «останавливается» до закрытия `subDone` и `done` worker'ов, и пока он стоит, `Start` отказывает. Писать `subErr` только если `done` — текущий. Тест: `Stop` с бюджетом 50 мс → `Start` возвращает ошибку. После освобождения хука `Start` проходит, горутина worker'а завершилась (`done` закрыт) |
| Mi-3 | Minor | `internal/state/apply.go:137-155`; `internal/state/apply_test.go` (create) | Путь **создания** не закреплён тестами по двум пунктам DoD и КД. (1) «При падении публикации ничего не сохраняется»: мутант X4 (`Put` до `publish`) выживает. С ним повтор упавшего создания отвечает **своему же** предложению `duplicate_entity`. Тест исполнителя на сбой публикации есть только для update. (2) §4.5, абзац о create: «с тем же `proposal_id` — дедуп». Мутант X8 (create без окна `proposal_id`) выживает: повтор создания под тем же `proposal_id` новым событием получил бы `duplicate_entity` вместо молчания | Два теста на `Applier`. (1) Первая публикация `entity.created` падает, сущности нет, повтор того же события → один `entity.created` с тем же id, отказов нет. (2) Новое событие `entity.create.proposed` под применённым `proposal_id` → ничего не публикуется |
| Mi-4 | Minor | `internal/state/context.go:191-192`, `:206`; `internal/state/facts.go:81-96`; `internal/state/apply_test.go:175` (`TestANonAtomicPackageRefusesOnlyWhatFailed`) | Две нормы без теста. (1) **Окно id событий посредника** (C-01 v1.6) ни в одном тесте ничего не гасит: мутант X1 без `Has` выживает. Проверен только порядок «запомнить после ответа» (M1 исполнителя). Повтор события, получившего **отказ**, окно `proposal_id` не гасит: отказы в него не попадают, и без окна событий отказ публикуется дважды. (2) DoD `WithCauseID`: «разные сущности одного пакета — разные id» проверено только для фактов. Мутант X2 (у всех отказов одна часть) выживает. С ним потребитель, гасящий дубли по id, отбросит второй отказ неатомарного пакета, и издатель не узнает, что вторая сущность отвергнута | (1) Тест контекста: событие с `version_conflict` доставлено дважды (`bus.Append` тех же байт) → один `entity.update.rejected`. (2) В `TestANonAtomicPackageRefusesOnlyWhatFailed` или отдельно: два отказа одного пакета имеют разные `id`, повтор того же события даёт те же id |
| N-1 | Nit | `cmd/multiverse/serve.go:301-302`, `:315` | Порядок «источники снимаются после `closeBus`» объяснён в комментарии, но не закреплён: мутант X11 (`defer installSources` после `defer closeBus`) проходит полный `go test ./cmd/multiverse/` | Если порядок нормативен (N-4 T-060), нужен тест: обёртка `openBus`, у которой `Close` строит корневое событие, и проверка, что его id — `seq-N` при `--id-source=sequence`. Иначе ослабить комментарий |
| N-2 | Nit | `cmd/multiverse/contexts_state.go:14-18`; карточка, «Рецепт синхронизации», п. 2 | После синхронизации с T-446 комментарий-инструкция «When T-446 reaches this branch…» станет ложным. Рецепт о нём молчит | В п. 2 рецепта добавить: комментарий снять, над `newStateContext` оставить одну строку про `MV_STATE_WORLDS` |
| N-3 | Nit | `internal/state/apply.go:215`, `:281-290` | `last_change.batch_size = len(p.Changes)`. КД §4.5 п. 9 говорит: «**применённый** размер остаётся в `last_change.batch_size`». Для неатомарного пакета с отказами это разные числа. Двойник делает так же (`shared/testkit/state/apply.go:243`) | Решить в T-056 вместе с заменой двойника: либо `len(plans)`, либо поправить формулировку КД |

### Вопрос контракта для system-architect (через оркестратора)

**Предложение без `world` в конверте.** `Applier` его пропускает (`apply.go:92-97`), двойник применяет к своему единственному миру (`shared/testkit/state/apply.go:41`). Факты:
- C-01 `world` в конверте не требует (`ValidateEnvelope`), схемы предложений его тоже не знают, поэтому такое событие доходит до State при любой настройке шины;
- все известные издатели (`Harness`, `FakeEncounter`, bootstrap `mvctl`) мир ставят, так что сегодня путь недостижим;
- при нескольких процессах State (`MV_STATE_WORLDS` у каждого свой) «применить к единственному миру» неоднозначно: одно событие применят все процессы, у которых по одному миру.

Оценка ревьюера: правильнее **пропуск с `Warn`** (Mi-1) плюс норма C-02 «предложение обязано нести `world` в конверте». Её можно закрепить политикой `Spec.Policy` типов `entity.*.proposed` (новое правило `WorldRequired` — `contract-change` в `shared/eventbus`), тогда издатель узнаёт о дефекте при `Publish`. Решение нужно до T-056: при замене двойника одно из двух поведений уйдёт, и тест двойника или `Applier` поменяет ожидание.

Сопутствующие вопросы:
1. Нужна ли в C-01 «окончательная» ошибка обработчика: паркуется сразу, без повтора, и не считается паникой. Без неё остановленный мир задерживает остальные миры процесса на 2,6 с на каждое своё предложение (п. 2 «Проверено»).
2. Ma-1: выбор между «мир останавливается на незавершённом ответе» и «worker повторяет публикацию сам» затрагивает §9 КД (строки ошибок `Publish`) до появления PUT в T-057. Если tech-lead выберет вариант, расходящийся с §9, нужна правка КД.

### Риски и допущения

- `-race` недоступен (нет cgo): разбор доступов — по коду (п. 4), стресс `-count=20 -cpu 1,4` зелёный.
- С реальным State по умолчанию (`--contexts=state,…`, мир `dark-forest-world`) процесс compose поднимает State с **пустым** миром. Пока нет bootstrap и восстановления (T-057/T-059), любое предложение получит `unknown_entity`. Раньше заглушка молчала. Настоящих издателей в compose пока нет, это не регрессия, но при ручной проверке стенда это видно.
- Пробное слияние выполнено на копии в scratch, не в git. Настоящее слияние `develop` → `epic/EPIC-002` может отличаться, если в эпик войдут T-454 и последующие правки `fake_contexts_test.go`.
- Интеграционные тесты, `make test`, Docker, стенд `:8888` и `.env` не трогались. `make compose-lint` вызывает только `docker compose config` (разбор файлов, без контейнеров).
- Копии `t055rev-tree`, `t055rev-merge`, `t055rev-base` и скрипт `t055rev-mut.py` удалены по точным путям. В рабочую папку, кроме этого раздела и строки карточки, ничего не добавлено.

### Предложения в бэклог

1. **EPIC-001 / system-architect:** «окончательная» ошибка обработчика в C-01 (например `eventbus.Permanent(err)`): `Delivery` паркует без пауз и не пишет как панику. State отдаёт её после остановки мира. До этого процесс с несколькими мирами — известное ограничение, записать в КД §9.
2. **T-056:** `plan` сверяет только `entity.id` (`apply.go:252`). Набор, в котором `entity.type` не совпадает с типом сущности мира, применяется к сущности мира, и факт несёт её тип. Владение (§4.6) должно брать тип из мира, а несовпадение — отвергать `invalid_op`. Решение и тест — там.
3. **T-056:** стенд `fightThroughTheProcess` перевести на настоящий State (поддерживаю п. 1 бэклога исполнителя). Тогда Ma-1 и Mi-1 проверяются и на пути боя.
4. **T-059:** при переходе на курсор снапшота + `Journal` поведение на фантомных фактах после Ma-1 должно давать `state_divergence`, а не молча применяться. Нужен тест с журналом, где два `entity.updated` одной сущности несут одну версию.

## T-055 · ревью #2 · 2026-09-13 · code-reviewer#3 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-055`, ветка `task/T-055-state-pipeline`, база `34bdcca`. Коммитов нет, проверялась рабочая копия. Проверены исправления итерации 2 по ревью #1 (Ma-1, Mi-1…Mi-4, N-1…N-3) и регрессия от них. Основание: раздел «T-055 · ревью #1» выше, «Итерация 2» карточки, запись `<!-- dev-log T-055 итерация 2 -->`, решение оркестратора по Ma-1 (Б+А), КД `state-and-mechanics.md` §4.8, §6.2, §9, `contracts.md` C-01 v1.4–v1.7. Код шины (`shared/eventbus/delivery.go`, `kafka.go`, `membus/membus.go`) прочитан, чтобы проверить путь отмены. Отметку по `cmd/multiverse/*` (мягкий режим) ставит tech-lead#1, это ревью её не заменяет.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 3 · Nit: 2.

Ma-1 закрыт. Worker повторяет публикацию того же события с теми же id и байтами. Вышедшие события повторно не публикуются. Шине ошибка не возвращается, поэтому `dead_letters` пуст, а следующее предложение решается от версии, объявленной фактами. `Stop` обрывает попытки: мир получает `publish_failed`, память не тронута, предложение остаётся незакоммиченным. На `membus` это подтверждено зондом, на kafka-адаптере — чтением кода. Mi-1…Mi-4 и N-1…N-3 исправлены, у каждого есть тест, мутанты красные. Новые замечания блокирующими не считаю:
- Mi-5 — пауза повтора идёт на таймерах домена и в replay не кончается никогда. Затронут только replay, а в штатном `make replay` (`--bus=memory`) ошибка `Publish` и так не проходит сама, поэтому исход тот же.
- Mi-6 — тест не закрепляет, что брошенное предложение остаётся незакоммиченным.
- Mi-7 — ещё один отказ, который схема не пропустит никогда. Путь есть только при `MV_BUS_VALIDATE_ON_READ=false`.

Mi-5 и Mi-7 правятся несколькими строками. Предлагаю сделать это до слияния, если tech-lead согласен. Иначе — отдельной строкой в T-056.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Ma-1: повтор того же ответа (`apply.go:357-377`, `:396-402`) | `publish` повторяет `a.pub.Publish(out, ev)` с тем же объектом события. Пауза `retryPause`: 100 мс, удвоение, потолок 5 с. Тест `TestThePauseBetweenAttemptsDoublesUpToTheCap`, байты всех попыток сравнивает `TestAFailedPublicationIsPublishedAgainAsItIs`. Цикл идёт по `events[i:]`, вышедшие события не повторяются (мутант R7 красный). Сама публикация идёт на `WithoutCancel`, пауза — на контексте мира (R5 красный). Каждая неудача — `Warn handled=true` с `proposal_id`, `event_id`, `attempt`, `pause`. Пока идут попытки, `/health` мира — `degraded` с `publish_attempts_failed` (`context.go:316-321`, R6 и R21 красные). Регрессионный тест по зонду P1 `TestNoTwoFactsOfOneVersionWhileAPublicationKeepsFailing`: 30 неудач, 31-я попытка проходит, `dead_letters` пуст, один факт на версию, память равна объявленным фактам. Мутант «ошибка шине после 3 попыток» (R1) и «применить без публикации» (R2) — красные |
| 2 | Ma-1: `Stop` обрывает попытки (`apply.go:370-371`, `:405-420`) | `wait` возвращает ошибку только при `ctx.Done()`, а отмена необратима. Значит, к возврату `ErrWorldStopped`+`ErrPublishFailed` в `dispatch` у `ctx` всегда `Err() != nil`. `Delivery.Deliver` (`delivery.go:119-128`) в этом случае возвращает `ctx.Err()` и не паркует событие. Норма закреплена `TestDeliverReturnsTheCancellationInsteadOfParking` в `shared/eventbus`. **membus**: `deliverNext` не двигает курсор, `Subscribe` возвращает `nil` (`membus.go:289-295`, `:330-333`). **kafka**: обработчик получает `ctx` подписки, то есть `subCtx` State. `stopped(loopCtx, err)` возвращает `nil` до `CommitMessages` (`kafka.go:225-230`), офсет не фиксируется. Порядок C-01 v1.7 не нарушен: процесс вызывает `StopAll` до `closeBus`, и отмена приходит от `Stop`, а не от `Close`. Зонд PR2: после `Stop` писем в `dead_letters` нет, на шине один факт `player-A`. После повторного `Start` `prop-round` доставлено снова и запарковано с `ErrWorldStopped` после 4 попыток, второго факта v2 нет. `memstore` не тронут: `Put` стоит после `publish` (R15 красный для create). `proposal_id` в окно не попадает, но наблюдать это нельзя: остановленный мир отвечает `ErrWorldStopped` раньше окна. Эквивалентный мутант, не замечание. Незакоммиченность тестом не закреплена — Mi-6 |
| 3 | Остановка мира в `Applier` (`apply.go:73-80`, `:434-448`; `context.go:141-156`) | `failure` живёт в `Applier`, а `c.appliers` переживает `Stop`/`Start`. Мутант R8 «новый `Applier` на каждый `Start`» — красный. Паника: `worker.run` вызывает `applier.halt` с `errPanicked`, `/health` показывает `reason: panic` (R9 красный). Отказ по набору без типа публикуется без `entity`, часть `WithCauseID` — id (`facts.go:84-104`, R10 красный). `keepFailed` недостижим, сегодня `Put` падает только на пустом id или мире. Мутант R24 выживает, это не замечание |
| 4 | Mi-1 (`apply.go:133-143`, `:467-482`; `context.go:229-237`) | `worldOf` читает `Event.World` напрямую. Предложение без мира — `Warn` с `event_id`, `type`, `proposal_id`. Чужой мир остаётся на `Debug`. Мутанты R18 (`Debug`) и R19 (`GetWorldIDFromEvent` в `Applier`) — красные |
| 5 | Mi-2 (`context.go:113-121`, `:182-198`, `:271-281`, `:172-176`) | `Start` отказывает, пока подписка или worker прошлого запуска не вернулись (R11 красный). `Stop` по таймауту закрывает `quit` всех worker'ов (R12 красный). `Stop` без остановки worker'ов (X9 = R22) — красный. `subErr` пишет только текущий запуск, но тестом это не закреплено (R20 выжил) — N-4. Гонок не нашёл. `previousRunEnded` читает `c.workers` прошлого запуска под `c.mu`, `close(done)` срабатывает после записи `subErr` (порядок `defer`) |
| 6 | Mi-3, Mi-4, N-1…N-3 | X4 (R15), X8 (R16), X1 (R13), X2 (R14), N-3 (R17) — красные. N-1: `TestTheSourcesOutliveTheBusOfTheRun`, X11 в копии — красный (id UUID вместо `seq-N`). Префикс `testkit.Deterministic` (`installed-by-the-test`) с `seq-` не совпадает, так что тест не тавтологичен. N-2: комментарий `contexts_state.go` — одна фраза про `MV_STATE_WORLDS`, п. 2 рецепта дополнен |
| 7 | Удалённый `TestTheRetryOfAFailedDeliveryReachesTheWorker` (C-01 v1.6) | Норма не потеряна. Worker теперь отдаёт ошибку только остановленного мира, других ошибок у него нет, и «повтор доходит до worker'а» проверяется на двух путях. (а) `TestAPanicStopsItsWorldAndOnlyItsWorld`: в письме `Attempts == 4` и `ErrWorldStopped`, то есть каждая повторная доставка дошла до worker'а. (б) `TestAStopThatEndsTheAttemptsStopsTheWorld`. Мутант M1 (R23, запомнить до ответа) — красный на обоих. Сценарий «повтор после временной ошибки даёт факт» исчез вместе с временной ошибкой worker'а, это следствие решения Б |
| 8 | Остаточные риски исполнителя (п. 4 задания) | **(а) Событие, которое шина не примет никогда.** При `MV_BUS_VALIDATE_ON_READ=true` (значение по умолчанию, `shared/env/vars.go:75`) предложение проходит схему на чтении. Ответ собран из прошедшего схему payload, поэтому вечная ошибка возможна только при дефекте конструктора State. При `false` путь есть, и он не один: зонд PR3 (Mi-7). Для MVP-1 приемлемо: мир один, `/health degraded`, `Warn` каждые 5 с, `Stop` обрывает попытки. **(б) Недоступный брокер** останавливает всю подписку `system_events`. Для MVP-1 приемлемо: мир один, и без брокера не публикует никто. Для процесса с несколькими мирами это ограничение: здоровые миры в `/health` показывают `ok`, хотя их предложения стоят за чужими попытками. Строка в бэклог EPIC-001/system-architect — п. 1 ниже |
| 9 | Прогоны в рабочей папке (go1.26.8 windows/amd64) | `go build ./... && go vet ./...` — 0. `gofmt -l internal cmd shared` — пусто. `git diff --check 34bdcca` — чисто. `go test -short -count=1 ./internal/state/... ./cmd/multiverse/...` — ok. `go test -short -count=20 -cpu 1,4 ./internal/state/...` — ok (18,4 с). `go test -tags e2e -count=1 ./test/e2e/...` — ok (13,4 с). `golangci-lint run ./...` — 0 issues |
| 10 | Мутанты (копия дерева в scratch `t055r2-tree`, без `.git`/`Docs`/`services`, без `-overlay`; контрольный первым; после каждого мутанта файл восстановлен, `diff -r internal/state` с рабочей папкой — идентично) | **C0** (контрольный, `version` факта +1) — красный, 4 теста. **Красные (22):** R1 ошибка шине после 3 попыток; R2 невышедшее событие пропускается, ответ сохраняется; R4 брошенный ответ не останавливает мир; R5 публикация на отменяемом контексте; R6 счётчик попыток не сбрасывается; R7 вышедшие события публикуются снова на каждой попытке; R8 новый `Applier` на каждый `Start`; R9 паника как `stopped`; R10 отказ с пустым `entity.type`; R11 `Start` не ждёт прошлого запуска; R12 `Stop` по таймауту не сигналит; R13 (X1); R14 (X2); R15 (X4); R16 (X8); R17 (N-3); R18, R19 (Mi-1); R21 попытки не видны в `/health`; R22 (X9); R23 (M1); X11 (`cmd/multiverse`). **Выжили (3):** R3 — `dispatch` возвращает `nil` на `ErrPublishFailed` при отменённом контексте, брошенное предложение коммитится (Mi-6); R20 — `subErr` пишет любой запуск (N-4); R24 — `keepFailed` без остановки мира (путь недостижим, не замечание) |
| 11 | Зонды (копия `t055r2-probe`, временный тестовый файл) | **PR1** — одна временная ошибка `Publish` в контексте: с `clock.RealTimers` ответ вышел со 2-й попытки за 107 мс, с `replay.NullTimers` за 10 с ответа нет, `attempts=1`, `/health degraded publish_attempts_failed=1` (Mi-5). **PR2** — незакоммиченность брошенного предложения на `membus` (п. 2). **PR3** — `Applier` с публикатором, который проверяет схему: `expected_version: -1` (путь только при выключенной проверке на чтении) даёт отказ `details.expected_version=-1`. Схема `entity.update.rejected` его не пропускает (`minimum: 0`), 25 попыток из 25 неудачны, до отмены контекста (Mi-7) |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-5 | Minor | `internal/state/context.go:136-139`, `:150`; `internal/state/apply.go:51-53`, `:385`; `cmd/multiverse/serve.go:425`; C-01 v1.4 «Таймеры повторной доставки»; КД §6.2 | **Пауза повтора публикации идёт на `Deps.Timers`, а в replay это `replay.NullTimers`, которые не срабатывают никогда.** Первая неудачная публикация в `--mode=replay` оставляет мир в `degraded`, `publish_attempts_failed=1` до `Stop`. Второй попытки нет, нет и повторных `Warn`, а `Stop` превращает это в `publish_failed` (зонд PR1). Это тот же класс дефекта, который C-01 v1.4 снял с шины: пауза повтора транспорта — не доменное время. По §6.2 State в replay берёт только `EventClock`, `Timers` у него нет. В штатном `make replay` (`--bus=memory`) ошибка `Publish` постоянна и исход тот же. На `--mode=replay --bus=kafka` временный сбой брокера вешает прогон навсегда. Строка «на `Deps.Timers`» есть в решении оркестратора, исполнитель её выполнил | Паузы повтора — на реальных таймерах в любом режиме, как у шины. Добавить `Config.Timers clock.Timers` (nil → `clock.RealTimers{}`) и передавать в `ApplierConfig.Timers` его, а не `deps.Timers`. Тесты задают `Config.Timers` ручными таймерами, как сейчас задают `Deps.Timers`. Тест: контекст с `Deps.Timers: replay.NullTimers{}` и одной временной ошибкой `Publish` получает ответ. Если оркестратор оставляет `Deps.Timers`, записать ограничение в КД §6.2 |
| Mi-6 | Minor | `internal/state/context_test.go:325-327`, `:339-357` (`TestAStopThatEndsTheAttemptsStopsTheWorld`) | Из пунктов решения «предложение остаётся незакоммиченным» закреплено только «не в `dead_letters`». Мутант R3 выживает: `dispatch` отдаёт `nil` на `ErrPublishFailed` при отменённом контексте, шина фиксирует офсет, писем нет. После рестарта предложение, половина ответа которого уже на шине, не придёт снова, и догон §4.8 (T-059) его не увидит. Сейчас код ведёт себя верно (зонд PR2), но ничто не мешает это сломать | После повторного `Start` дождаться письма в `dead_letters` для `prop-round` с `ErrWorldStopped`: оно доказывает повторную доставку. Проверку поставить до ожидания `prop-next` |
| Mi-7 | Minor | `internal/state/apply.go:304-311`; `internal/state/facts.go:94-99`; `schemas/events/entity.update.rejected.v1.json` (`details.*_version minimum: 0`) | **Второй детерминированный путь «отказ, который шина не примет никогда»**, в дополнение к закрытому `entity.type`. Итерация 2 утверждает, что других известных нет. При `MV_BUS_VALIDATE_ON_READ=false` набор с `expected_version: -1` даёт `version_conflict` с `details.expected_version=-1`. Схема отказа его не пропускает, и подписка `system_events` всех миров стоит в попытках до `Stop` (зонд PR3). Одно ядовитое предложение — и мир остановлен при остановке процесса | В `plan` отрицательный `ExpectedVersion` считать `invalid_op` без `details`. Либо в `rejectedFact` не писать отрицательные версии в `details`. Тест на `Applier` с публикатором, проверяющим схему, по образцу `TestARefusalOfAnEntityWithoutATypeCanBePublished`. Общее решение для остальных путей — sentinel ошибки схемы (бэклог, п. 1) |
| N-4 | Nit | `internal/state/context.go:172-176` | «`subErr` пишет только горутина текущего запуска» (Mi-2) тестом не закреплено: мутант R20 (`if true`) выживает. Путь узкий: старая подписка вернулась с ошибкой после `Stop` по таймауту и нового `Start` | Если дёшево — в `TestStopIsBoundedByItsContext` дать старой подписке вернуть ошибку после нового `Start` и проверить, что `Health()` нового запуска не `fail`. Иначе оставить как есть |
| N-5 | Nit | `internal/state/apply.go:118-124`; КД §9, строка «Ошибка `Publish` факта после PUT» | Комментарий `Apply` ссылается на §9, но §9 описывает другое: «повтор ×3; затем факт помечен неподтверждённым, worker продолжает» с `/health degraded {unpublished_facts: n}`. Код повторяет бесконечно и отдаёт `publish_attempts_failed`. До PUT (T-057) это законное решение оркестратора, но в КД строки «ошибка `Publish` до PUT» нет. Ревью #1 предупреждало, что КД нужна правка | Ссылку в комментарии заменить на решение по Ma-1 (`journal.md`/`dev-log.md`). Строку §9 «до PUT: повтор того же ответа до успеха или `Stop` → `publish_failed`» и согласование имени поля `/health` — system-architect, при T-057 |

### Открытые вопросы (оркестратору)

1. Mi-5: в решении по Ma-1 записано «пауза на `Deps.Timers`», а C-01 v1.4 и КД §6.2 отдают паузы транспорта реальным таймерам. Нужно подтвердить перевод пауз State на реальные таймеры (`Config.Timers`) или записать ограничение replay в КД.
2. Mi-5/Mi-7 — до слияния T-055 или в T-056? Правка — несколько строк и два теста.

### Риски и допущения

- `-race` недоступен (нет cgo). Разбор доступов — по коду, стресс `-count=20 -cpu 1,4` зелёный.
- Путь kafka-адаптера на отмене проверен чтением кода (`delivery.go:119-128`, `kafka.go:225-230`) и unit-тестом `shared/eventbus`. Интеграционные тесты с Redpanda не запускались, как требует задание.
- Решение Б+А при рестарте процесса: предложение с половиной ответа на шине придёт снова. До T-059 память пуста, и ответ будет `unknown_entity`. После T-059 догон восстановит вышедший факт, и повтор решится заново — например, `version_conflict` у сущности, чей факт уже вышел. Атомарность пакета через рестарт так не сохранить, это нужно учесть в T-059 (бэклог, п. 3).
- Проверки выполнены на двух копиях дерева в scratch: `t055r2-tree` для мутантов и `t055r2-probe` для зондов. Скрипт `t055r2-mut.py` и лог удалены по точным путям. В рабочую папку добавлены только этот раздел и строка карточки.

### Предложения в бэклог

1. **EPIC-001 / system-architect (`contract-change`):** sentinel ошибки схемы при `Publish`, например `eventbus.ErrInvalidPayload` из `contracts.Validate`. Тогда `Applier` отличает вечный отказ от недоступного брокера и останавливает мир сразу (`publish_rejected`), не держа подписку. Вместе с `eventbus.Permanent(err)` из бэклога ревью #1, п. 1.
2. **system-architect / КД §9:** для процесса с несколькими мирами — подписка или партиция на мир (ключ `world`). Пока этого нет, в `/health` нужен признак «подписка стоит за попытками мира X» для остальных миров.
3. **T-059:** рестарт после `publish_failed` — повтор предложения, часть ответа которого уже в журнале. Решить, как догон §4.8 восстанавливает атомарный пакет: дослать недостающие факты из `intent` (T-057) или объявить расхождение. Тест: журнал с фактом `player-A v2` без факта `wolf-alpha v2` одного `proposal_id`.
4. **T-057:** привести `/health` к §9 (`unpublished_facts`) или §9 к коду (`publish_attempts_failed`); строка «ошибка `Publish` до PUT» в §9 (N-5).

## T-458 · ревью #1 · 2026-09-13 · code-reviewer#3 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-458`, ветка `task/T-458-session-recording`, база `a037efb`. Коммитов нет. В индексе два `git mv`, остальное — рабочая копия. Смотрелись `git diff --cached -M a037efb` (оба переименования, 100 %), `git diff -M a037efb` (9 файлов) и 8 неотслеживаемых файлов. Основание:
- раздел «### T-458» в `tasks.md`: DoD А1–А9, Б1–Б2;
- карточка `tasks/T-458.md` («Выполнение», три отклонения) и запись `<!-- dev-log T-458 -->`;
- `contracts.md` C-01 v1.9 («Запись сессии», «Чтение журнала», «Источники записей LLM», «Время корневых событий в replay», «Порядок поставки»), v1.10 (кончик эпика) и §16 п. 8;
- `ownership.md` v0.7 §1 и §3 п. 4 (а);
- код T-060 до переноса (`git show a037efb:internal/replay/recording.go`).

Код шины (`membus.ReadRange`, `Kafka.ReadRange`, `internal/replay/{journal,middleware}.go`) и `api/gateway.openapi.yaml` (EPIC-004) прочитаны для сверки. Ревью system-architect (`contract-change`) и отметку tech-lead#1 это ревью не заменяет. Правки `Docs/dev-team` внутри ветки (карточка, `tasks.md`, `dev-log.md`) входят в состав задачи, замечанием не считаются.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 2 · Nit: 5.

Перенос чистый: API и тексты ошибок совпадают с DoD, поведение перенесённого кода не изменилось, кроме `LLMOutputKeyOf`. Мутанты T-060 на новом месте красные. `ReadJournal`, `Deps.Recording`, `Advance` и маршрут часов соответствуют C-01 v1.9, прогоны зелёные. Не блокируют:
- Mi-1 — ветка «маршрут есть в replay без записи» тестом не закреплена, мутант выжил;
- Mi-2 — у одного маршрута три формы тела ошибки, если считать схему `Error` в openapi. Решение нужно до того, как EPIC-004 внесёт операцию.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Перенос (А1–А3) | `git diff --cached -M --summary a037efb` — 2 × `rename … (100%)`. Против рабочей копии `recording.go` 74 %, тест 80 %, `git log --follow` после коммита дойдёт до T-060. Построчный дифф: смена пакета, `OpenRecording`/`ReadRecording` → `Open`/`Read`, префикс `replay:` → `recording:` в пяти местах, новый `wholeAttempt`. Остальное — байт в байт. `git grep` по шаблону А3 в коде пуст, совпадения только в `Docs`. `go list -deps ./shared/recording` — `shared/{clock,jsonpath,eventbus}` и сам пакет, без `internal/*`, `runtime` и `testkit`. В EPIC-001/003/004, T-460 и T-461 `internal/replay` не импортируется, только упоминается в комментариях. Переименование чужой код не ломает. Тесты T-060 из А3 зелёные без правки ожиданий |
| 2 | `LLMOutputKeyOf` на дробном `attempt` (А4) | Прежний `jsonpath.GetInt` (`shared/jsonpath/accessor.go:252`) делает `int(v)` для `float64`: `1.5` → `1`, и `0` и отрицательные тоже проходили. Запись `attempt: 1.5` попадала в индекс под ключом первой попытки, а `providers/recorded` (EPIC-003, `recorded.go:258`, `:342`) такую запись отвергает: `*int` из JSON не разбирается, `< 1` — отказ. Это расхождение двух читателей, которое C-01 v1.9 запрещает, значит, прежнее поведение было ошибкой. Новый ключ: `int`/`int32`/`int64`/`float64` без дробной части, диапазон 1…2^53. JSON-ключ и Go-ключ совпадают: `TestLLMOutputKeyOfIsTheSameFromJSONAndFromGo` проверяет, что после `json.Unmarshal` там `float64`, и сравнивает ключи. Голден `TestLLMOutputKeyGolden` закрепляет байты. Узость типов — N-5 |
| 3 | `ReadJournal` (А5) | `End` снимается один раз. При `End ≤ from` — пустая запись, не `nil`: мой R4 красный. На membus и kafka `ReadRange` при `to ≤ from` отвечает `from, nil`, так что выживший мутант исполнителя эквивалентен. Отрицательный `from` уходит в `ReadRange` и возвращается его ошибкой через `%w`. Отменённый `ctx` даёт ошибку и до чтения, и после последнего события, текст по C-01. На kafka `End` при отменённом `ctx` отвечает `…: end: context canceled`, а не `read stopped at`, но `errors.Is(err, context.Canceled)` истинно. **Утечек нет**: `ReadRange` синхронный, подписок и горутин не создаёт. У membus горутин нет вовсе, у kafka reader закрывается `defer k.closeReader`. `goleak` в пакете не используется (в модуле он есть только в `shared/testkit/swarm`), и для синхронного вызова он не нужен. Частичная запись при ошибке `ReadRange` тестом не закреплена — N-2 |
| 4 | `EventClock.Advance` (Б1) | Сравнение и сдвиг — под `c.mu`, `Observe` и `Now` берут ту же блокировку, данных без неё нет. Равное время в другой зоне часы не трогает: `Now()` сохраняет зону, и тест это закрепляет. Корни всё равно пишутся в UTC (`eventbus.nowUTC`), поэтому байты от зоны не зависят. **Мутант «`Now()` + `Observe`»** детерминированным тестом не убить: он **эквивалентен**, а не просто трудноуловим. Обе операции только поднимают время (max), поэтому любое чередование мутанта линеаризуемо. Точку линеаризации `Advance` можно взять сразу перед первой операцией, которая подняла часы выше `at` в окне между проверкой и `Observe`. До неё все читатели видели время ≤ `at`, после неё `max(часы, at)` равно часам. Ответ `204` в этой точке законен и у атомарной версии. Данных без блокировки у мутанта нет, так что и `-race` его не отличит. Объяснение в карточке и в комментариях — N-1. Блокировку в коде оставить: так рассуждение проще, а C-01 требует именно её |
| 5 | Маршрут часов (Б2) | Монтируется через `runtime.AdminOnly` (`serve.go:336`), до `StartAll`, только при `times.events != nil`, то есть в replay. 403 — без `X-Client-Id` и с чужим id. 405 с `Allow: POST`. 400 — 11 тел, включая хвост после JSON и тело больше 1 КиБ. 409 `clock_behind`, часы не меняются. Время корней не идёт назад — процессный тест на настоящем HTTP-сервере (at1 → 409 на at0 → at1 → at2). Мой R3 (маршрут двигает другие часы) красный. **404 в live** — текст `http.ServeMux`, не JSON. **Приемлемо, не замечание**: C-01 говорит «в live маршрута нет (404)», как у любого неподключённого пути. JSON-404 потребовал бы подключать обработчик и в live, а это противоречит контракту. Для операции EPIC-004 это стоит записать: ответ 404 без схемы. Ветка «replay без `--recording`» не проверена (Mi-1). Форма тел — Mi-2 |
| 6 | `serve.go` (А6, А7), совместимость с T-055 | Запись читается один раз, в `timeOf`, до `installSources` и `openBus`. Шпион считает чтения (1/0/0), в `Deps` тот же указатель, `Start()` совпадает с часами. Поле `openRecording` у `process` сделано так же, как `openBus`: `nil` → `recording.Open`, литералы `process{…}` в `serve_test.go` (EPIC-004) и `dispatch_test.go` компилируются. `--recording` без replay по-прежнему отвергается в `parseServe` (`:165`), нечитаемая запись — до `openBus`. Блок источников T-055 (`installSources`/`defer`) и `times.bus = clock.RealTimers{}` не тронуты. `contexts_state.go` и остальные `contexts_*.go` задача не меняет, `TestTheProcessInstallsTheSourcesOfTheConstructorsFromDeps` зелёный. Мелочи читаемости — N-3 |
| 7 | depguard (А9), отклонение 2 | Правило `shared` запрещает `multiverse-core.io/internal` целиком, по префиксу, и `**/shared/**` покрывает `shared/recording`. Зонд на `internal/mechanics` доказывает то же, что зонд на `internal/replay`. Импорт `internal/replay` из `shared/recording` даёт цикл `replay → runtime → recording`, это ошибка компиляции раньше линтера. Защита здесь сильнее depguard, **отклонение приемлемо**. `**/shared/recording/**` в `no-testkit-in-production` работает, `_test.go` исключены. `desc` правила пакет не называет — N-4 |
| 8 | Отклонение 1: «`llm.output` без ключа» на шине без проверки при чтении | **Достаточно для того, что проверяет DoD**: обработчик `ReadJournal` ничего не отвергает. Мутант «разбор и отказ в обработчике» красный, `dead_letters` пуст. На валидирующей шине запись без ключа паркует `Delivery` до обработчика, и тест проверял бы шину, а не `ReadJournal`. Остаётся факт, не проверенный ни одним тестом и не описанный в C-01. При `MV_BUS_VALIDATE_ON_READ=true` (значение по умолчанию) запись, не прошедшая схему, паркуется **самим чтением журнала**. Из записи она выпадает без ошибки: `next == End`, а `Len() < End − from`. Каждое повторное чтение паркует её снова. `ReadJournal` этого не видит, а сверка `Len` с `End − from` на kafka невозможна из-за пропусков офсетов. Это поведение транспорта, а не дефект задачи — «Риски», бэклог п. 2 |
| 9 | Мутанты (свои; копия дерева в scratch `t458r1-<имя>` без `.git`/`Docs`/`services`, по копии на мутант, без `-overlay`, контрольный первым) | **C0** (контрольный: обработчик `ReadJournal` не добавляет событие) — красный (`TestReadJournalLeavesOutWhatIsPublishedAfterTheEnd`, `TestReadJournalKeepsLLMOutputWithoutAKey` и другие `TestReadJournal*`). **R1** маршрут монтируется при `opts.recording != ""` вместо replay — **выжил**, `./cmd/multiverse` и `test/e2e` (Mi-1). **R2** при ошибке `ReadRange` возвращается частичная запись вместе с ошибкой — **выжил** (N-2). **R3** маршрут двигает новый `EventClock`, не часы процесса — красный (`TestTheReplayClockRouteSetsTheTimeOfRootEvents`). **R4** при `End ≤ from` — `nil, nil` — красный (`TestReadJournalAtOrPastTheEndIsEmpty`). **R5** `recorded_events` в логе старта = 0 — красный (`TestReplayLogsTheRecordingItRunsOn`). Мутант исполнителя «`Now()` + `Observe`» разобран в п. 4 как эквивалентный |
| 10 | Прогоны в рабочей папке (go1.26.8 windows/amd64) | `go build ./... && go vet ./...` — 0. `gofmt -l cmd internal shared` — пусто. `go test -short -count=1 -cover ./shared/recording/... ./internal/replay/... ./cmd/multiverse/... ./shared/runtime/... ./shared/eventbus/...` — ok. Покрытие: `shared/recording` 96,6 %, `internal/replay` 100 %, `cmd/multiverse` 92,8 %, `shared/runtime` 96,0 %. `go test -tags e2e -count=1 ./test/e2e/...` — ok (13,8 с). `golangci-lint run ./...` — 0 issues |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-1 | Minor | `cmd/multiverse/serve.go:331-337`; `cmd/multiverse/replay_clock_test.go:193-198`; C-01 v1.9 «Время корневых событий в replay» (E2E-12 — сценарий без записи) | Маршрут часов по контракту есть в **любом** `--mode=replay`, с записью или без. В сценарии без записи (E2E-12, харнесс T-061) `at` — время шага, и маршрут нужен именно там. Процессный тест гоняет только replay с `--recording`. Мутант R1 (монтировать при `opts.recording != ""`) проходит `./cmd/multiverse` и `test/e2e`: такую регрессию заметила бы только T-061 | Прогнать `TestTheReplayClockRouteSetsTheTimeOfRootEvents` таблицей по `replayOptions(writeSession(t))` и `replayOptions("")`. Во втором случае часы стартуют с нулевого времени, `at1 > 0`, ожидания те же. Достаточно и отдельного подтеста «replay без записи: 204 и корень с `at1`» |
| Mi-2 | Minor | `internal/replay/clockroute.go:85-89`; `shared/runtime/http.go:230-234`; EPIC-004 `api/gateway.openapi.yaml:679-690` (`Error`/`ErrorBody`); DoD Б2 «JSON с полем `error`, как у `forbid`»; карточка, отклонение 3 | У маршрута три формы тела ошибки. 403 от `AdminOnly`: `{"error": "<человеческий текст>"}`. 400/405/409: `{"error": "<код>", "message": "<текст>"}`. В файле, куда EPIC-004 внесёт операцию под тегом `process`, общая схема — `{"error": {"code", "message"}}`. Клиенту маршрута (харнесс T-061, `mvctl`) нужны два разбора, EPIC-004 придётся описывать третью схему. DoD буквально выполнен (поле `error` есть), но «как у `forbid`» — только наполовину: в `error` другой смысл | Решение system-architect до внесения операции в openapi (вопрос ниже). (а) Оставить плоскую форму и записать её в C-01 рядом с маршрутом, `AdminOnly` не трогать — код задачи не меняется. (б) Привести тела маршрута к `Error` из openapi — правка `writeError` и ожиданий в двух тестах. Мой выбор — (а): это маршрут процесса, а не шлюза, и контракт уже отделяет его от раздела `admin` |
| N-1 | Nit | `internal/replay/eventclock.go:79-80`; `internal/replay/clock_test.go:116-120`; карточка, «Мутанты» и абзац «Почему выжил мутант Б1»; `dev-log.md` («атомарность держат блокировка и `-race` в CI») | Объяснение неточное. Мутант «`Now()` + `Observe`» не «трудно поймать» — он эквивалентен: обе операции монотонны (max), любое его чередование линеаризуемо (п. 4 «Проверено»). Гонки данных у него нет, и `-race` его не отличит. Фраза «would race with the middleware» годится как довод за простоту, но не как описание наблюдаемой ошибки | В карточке и комментарии теста: «мутант эквивалентен: `Advance` и `Observe` монотонны, чередование линеаризуемо; блокировка — по C-01 и ради простоты рассуждения». `-race` из объяснения убрать. Текст C-01 — на усмотрение system-architect, правка не обязательна |
| N-2 | Nit | `shared/recording/journal.go:40`, `:43`; `shared/recording/journal_test.go:183-238` | Карточка утверждает: «вместе с ошибкой частичная запись не возвращается». Закреплено это только в ветке `next < End` (тест `Close`). Мутант R2 (`return rec, err` при ошибке `ReadRange`) выживает, ветка отменённого `ctx` тоже не проверена. Потребитель, который смотрит на `rec != nil` раньше `err`, взял бы обрезанную запись за целую | В `TestReadJournalWrapsTheErrorsOfTheJournal` и в обоих подтестах `TestReadJournalRefusesACancelledContext` проверять `rec == nil` |
| N-3 | Nit | `cmd/multiverse/serve.go:364`; `:105` | (1) `times.recording.Len()` опирается на инвариант, который проверяет другая функция: `opts.recording != ""` ⇒ replay (`parseServe`, `:165`) ⇒ запись прочитана. `process.run` с `serveOptions`, собранными в обход `parseServe` (так делают тесты пакета), в live с `recording` упал бы на nil-разыменовании уже после `StartAll`. (2) Локальная `recording := fs.String(...)` в `parseServe` затеняет импортированный пакет `recording` | (1) Условие `if times.recording != nil`. (2) Переименовать в `recordingPath` или `recordingFlag` |
| N-4 | Nit | `.golangci.yml:101`; `CLAUDE.md`, «Тестовые заглушки» | `desc` правила `no-testkit-in-production` перечисляет «internal/*, cmd/*, shared/eventbus/*». Нарушение в `shared/recording` выдаст сообщение без этого пакета, хотя строка `files` его покрывает. В описании проекта список production-кода тот же | Добавить `shared/recording/*` в `desc`. `CLAUDE.md` — строкой для tech-writer, не в этой задаче |
| N-5 | Nit | `shared/recording/recording.go:210-231` | `wholeAttempt` принимает `int`/`int32`/`int64`/`float64`. Прежний `GetInt` принимал ещё `int8`/`int16`/`uint*`/`float32`, и `providers/recorded` через JSON-круг принимает их тоже. Событие, собранное в Go с `attempt: uint(2)`, провайдер найдёт, а `Index` пропустит. Сегодня так никто не публикует: с шины и из файла приходит `float64` | Либо добавить остальные целые типы с той же проверкой диапазона, либо прямо написать в комментарии, что другие типы дают `""` намеренно |

### Открытые вопросы (оркестратору → system-architect)

1. **Mi-2:** форма тела ошибок маршрута часов — плоская `{"error": code, "message"}` с записью в C-01 или схема `Error` из openapi. Нужно до операции EPIC-004 и до разбора ответа в T-061.
2. **Зонд А5 и источники `providers/recorded` (C-01 v1.9).** Для `--mode=replay` без записи контракт велит строить провайдер над `recording.ReadJournal(ctx, Deps.Journal, llm_records, 0)`. Зонд исполнителя показывает, что `Deps.Journal` в replay обёрнут middleware: чтение **сдвигает `EventClock` процесса** на самое позднее время прочитанных `llm.output`. Если `llm` читает `llm_records` в своём `Start`, а журнал уже содержит записи сессии, часы окажутся в её конце раньше первого входа харнесса. Тогда первый `POST /v1/admin/replay/clock` со временем первого корня получит `409 clock_behind`. Решение нужно до T-212. Варианты: читать журнал без middleware (отдельное поле или флаг), читать до старта часов или записать, что replay без записи идёт только на пустом `llm_records`. В T-458 поведение по DoD не менялось, дефектом задачи это не считаю.

### Риски и допущения

- `-race` недоступен (нет cgo). Разбор доступов к `EventClock` сделан по коду: все поля под `c.mu`.
- Путь kafka-адаптера (`End` при отменённом `ctx`, закрытие reader) проверен чтением кода. Интеграционные тесты не запускались, как требует задание.
- Выпадение записи, не прошедшей схему, из `ReadJournal` на валидирующей шине (п. 8) не закреплено тестом и не описано в C-01. Потребители T-212/T-221/T-237 могут принять «ошибки нет» за «запись полная».
- Покрытие и мутанты — в рабочей копии и копиях scratch. Слияние с кончиком эпика `281348a` не повторялось: по коду между `a037efb` и `281348a` расхождений нет (`git diff --stat` — только документы).
- Копии `t458r1-control`, `t458r1-routecond`, `t458r1-partialrec`, `t458r1-otherclock`, `t458r1-nilempty`, `t458r1-logcount` и скрипт `t458r1_mut.py` удалены по точным путям. Индекс не трогался. В рабочую папку добавлены только этот раздел и строка карточки.

### Предложения в бэклог

1. **T-061:** харнесс разбирает `409 clock_behind` по полю `error` и печатает `message`; общий клиент маршрута с `mvctl` (поддерживаю п. 3 бэклога исполнителя).
2. **EPIC-001 / system-architect:** в C-01 «Чтение журнала» записать поведение на валидирующей шине: запись, не прошедшая схему, паркуется самим чтением и выпадает из `Recording` без ошибки, при каждом чтении снова. Тест-факт на membus без `SkipValidateOnRead` — в `shared/recording`. Другой вариант — `ReadJournal` возвращает число припаркованных офсетов, взятое у `Delivery`.
3. **EPIC-001:** `shared/runtime`, `shared/clock` и `shared/contracts` тоже попадают в бинарник, но в `no-testkit-in-production` их нет. Расширить `files` до `**/shared/**` с исключением `**/shared/testkit/**` вместо перечня пакетов.
4. **EPIC-004:** в операции `POST /v1/admin/replay/clock` описать 404 в live как ответ без тела схемы (п. 5 «Проверено»).
5. Переименование тестов перенесённого файла (`TestReadRecording…`, `TestOpenRecording…`) — поддерживаю п. 1 бэклога исполнителя, отдельной правкой после слияния.

## T-458 · ревью #2 · 2026-09-13 · system-architect#1

### Границы ревью

Повторный просмотр `contract-change` после итерации 2. По поручению оркестратора он заменяет и ревью #2 кода. Смотрелись `git diff -M a037efb` рабочей копии `.worktrees/T-458` и неотслеживаемые файлы, против раздела «Итерация 2 (developer)» карточки. Индекс (2 × `git mv`) не трогался. Подробности — карточка `tasks/T-458.md`, раздел «Ревью system-architect #2».

### Вердикт

**Одобрено.** Critical: 0 · Major: 0 · Minor: 0 · Nit: 2.

- **Условия system-architect.** У-1 (`ReadJournal` метит контекст, middleware replay пропускает помеченный обработчик), У-2 (шаблон `POST /v1/admin/replay/clock`), У-3 (форма `Error`) и У-4 (префиксы `Read`/`Open`) закрыты.
- **Замечания ревью #1 code-reviewer#3.** Mi-1, Mi-2 и N-1…N-5 закрыты.
- **Строгий разбор тела в тестах.** `DisallowUnknownFields` допустим: это тесты производителя, а `details` в `ErrorBody` необязателен. Клиенты маршрута (T-061, `mvctl`) разбирают тело терпимо.
- **Текст C-01 v1.11.** В карточке он сверен с кодом и поправлен в пяти пунктах: префиксы, типы `attempt`, `End` без метки, `405` до проверки доступа, `details`.

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| N2-1 | Nit | `shared/recording/recording.go`, `NewWriter`/`Close` | `recording: create recording: …` и `recording: sync recording: …` повторяют слово. Ошибка закрытия файла в `Close` уходит без префикса | Вместе с переименованием тестов (бэклог ревью #1 system-architect, п. 4) или в T-221 |
| N2-2 | Nit | карточка, «Выполнение» → «Проверка слияния» | Абзац про T-460 устарел: T-460 уже в кончике эпика `862c8c0` | Актуальный факт — п. 4 раздела «Ревью system-architect #2» |

### Прогоны

- `go build ./... && go vet ./...` — 0.
- `go test -short -count=1 ./shared/recording/... ./internal/replay/... ./cmd/multiverse/...` — ok. Покрытие 97,8 % / 100,0 % / 92,8 %.
- `golangci-lint run ./shared/... ./internal/replay/... ./cmd/multiverse/...` — 0 issues.
- Мутанты: контрольный `Len()`+1 и «middleware не смотрит метку» — оба красные. Копии удалены по точным путям.
- `.golangci.yml` сливается с кончиком эпика `862c8c0` без конфликтов.

## T-056 · ревью #1 · 2026-09-13 · code-reviewer#3 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-056`, ветка `task/T-056-state-invariants`, база `862c8c0`. Коммитов нет, поэтому смотрел рабочую копию: `git diff` (23 файла) и неотслеживаемые `internal/state/{dedup,dedup_test,export_test,invariants,matrix_test,ownership}.go`. В ревью входят итерации 1 и 1b из карточки.

Чужие файлы в задаче:
- `.golangci.yml` и `cmd/multiverse/fake_contexts_test.go` (EPIC-001) — отметка tech-lead#1;
- `shared/testkit/swarm/*` (EPIC-003) — разрешение оркестратора, просмотр tech-lead#2;
- `shared/contracts/registry_state.go` — просмотр system-architect.

Это ревью перечисленные отметки не заменяет. Правки `Docs/dev-team` в ветке (карточка, `dev-log.md`) входят в задачу, замечанием не считаются.

Основание:
- раздел T-056 в `tasks.md` со строками приёмок T-448, T-054, T-055;
- карточка `tasks/T-056.md`, «Выполнение» и «Итерация 1b»;
- `contracts.md` C-02 v1.4–v1.7 (включая v1.5b), C-01 v1.10, C-05 v1.8 п. 9;
- КД `state-and-mechanics.md` §4.5, §4.6, §8, §9;
- строки T-056 в карточке T-457 (EPIC-001).

Решения оркестратора (не оспариваются):
- `ErrWorldStopped` обычная, не `Permanent`;
- `died_at`/`killed_by` пишутся только NPC;
- исключение `shared-testkit-state` остаётся;
- rest: «не выше `hp_max`», во встрече — `law_violation` без `invariant_id`;
- `world.go` — в T-057/T-059.

### Вердикт

**Вернуть.** Critical: 0 · Major: 2 · Minor: 5 · Nit: 2.

Что сделано хорошо:
- порядок шагов §4.5 выдержан;
- матрица отказов полна, все семь причин проходят схему;
- дедуп по окну и `last_change` доказан тестами;
- inv-01 проверяет сторону участников только при `touched`, как в C-05 v1.8 п. 9 (б);
- `FakeState` стал `Applier` над `memstore` с прежними сигнатурами;
- стенд I1-α работает на одном настоящем State.

Возврат из-за двух дефектов, оба воспроизведены зондами:
- **Ma-1:** в неатомарном пакете набор изменений может остаться без ответа — ни факта, ни отказа;
- **Ma-2:** запись пути не в каноническом виде (`status.`, `.description`) обходит нормы поверх таблицы и матрицу переходов статуса.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Порядок §4.5 и матрица отказов | **Update** (`apply.go:270-346`, `plan` `:352-413`): пустой пакет → одна сущность дважды (п. 1а, до дедупа) → `malformedOp` (п. 1, на всё предложение) → дедуп → по набору: существование → тип набора против мира → версия → шаг 5 `dead_entity` по сущности «до» → шаг 6 владение → `ApplyOps` → `statusRefusal`/`restRefusal` по результату → шаг 8 по пакету. **Create** (`:209-253`): форма → дедуп → `duplicate_entity` → владение → законы. Проверка обязательных атрибутов по типу из §4.5 не сделана, исполнитель вынес её в бэклог. Мутант R8 (владение раньше `dead_entity`) красный. `TestTheMatrixOfRefusals` покрывает семь причин и проверяет, что встретилась каждая. Отказ проходит схему: `journal` вызывает `contracts.Validate` на каждом ответе. `details.invariant_id` пишется только при законе (`facts.go:99-101`) |
| 2 | Дедуп (`dedup.go:25-35`) | Окно `proposal_id` пополняется только после успешного `Put`, отказы в окно не попадают. Довод «любая целевая сущность вместо всех» (§4.5 п. 2) **принимаю**. При `atomic=true` все сущности несут одну запись, и разницы нет. При `atomic=false` частично применённый пакет оставляет запись только на применённых сущностях, и правило «все» применило бы эту часть второй раз, как только окно забудет id. Ложного срабатывания «любой» не даёт: запись `last_change.proposal_id == p.ID` появляется только при применении. Повтор после частичного пакета закреплён тестом `TestARepeatPastTheWindow…/a package applied in part`. Текст КД §4.5 п. 2 надо поправить (бэклог, п. 2). **Пробел:** мутант R1 (без окна, только `last_change`) выжил, см. Mi-2 |
| 3 | Владение (`ownership.go`) | Предлагающий определяется так: агент — только с уровнем из пяти, с `meta.agent` источник не решает; `gateway` и `testkit/gateway` — gateway; `mvctl` — author. Правила берутся из `contracts.OwnershipRules`, тип сущности — из мира (`plan` сверяет тип набора, `:374-376`). Префикс сравнивается по границе сегмента. Нормы C-02: `status` у gateway только с `forget`, `hp` только с `rest`. Переход в `abandoned` не от gateway с `forget` → `level_violation`. У gateway с включённой таблицей разрешён только переход `alive → abandoned`. `abandoned` над `abandoned` → `dead_entity`, `set status alive` над `alive` → пустой `changed[]`. Rest выше `hp_max` → `law_violation {inv-02}`, во встрече — без `invariant_id` (решение оркестратора). **Обходы:** Ma-2 (путь не в каноническом виде), Mi-1 (rest в том же наборе стирает `encounter_id`) |
| 4 | inv-01, сторона участников (`internal/mechanics/invariants.go:149`) | Условие `slices.Contains(touched, p.PlayerID)` совпадает с C-05 v1.8 п. 9 (б). В `TestInvDeadDoesNotAct` поправлены ровно две строки из T-457 Mi-4, добавлены P3 и P2. На State: P3 — `TestAForgetOvertakingAPackageOfTheEncounterIsNotRefused` (факт), P2 — `TestADeathThatLeavesTheCharacterInTheFightIsRefused` (`inv-01` на игроке). Мутант исполнителя M15 красный на P3. Комментарии пакета и `attrs.go` соответствуют T-457 |
| 5 | `FakeState` = `Applier` над `memstore` | Экспортируемые имена сохранены: `New`, `Config`, `WithInvariants`, `Seed`, `Get`, `All`, `StateHash`, `Start`, `Wait`, `Apply`, `AppliedProposals`, `Snapshot`, `SnapshotKey`, `LoadFixtures`, константы. Добавлены `ReasonLevelViolation` и `ReasonLawViolation`. Константы причин теперь типизированы как `string`, потребители сравнивают со `string`, сборка зелёная. Потребители в `.worktrees/EPIC-003` (`9691699`) и `.worktrees/EPIC-004` (`ce7b35a`) — тесты `testkit/{gateway,swarm}`, `cmd/multiverse`, `test/e2e`, `internal/mechanics/changes_encounter_test.go`; production-код `testkit/swarm/fake_context.go` зовёт только `LoadFixtures`. **Кончик EPIC-003 на новом двойнике красный в трёх тестах** (зонд, см. Mi-5). Двойник без таблицы владения (КД §8) — риск приемлем. Издатели боя на стенде (`FakeEncounter`, `Harness`) проходят через настоящий State с таблицей, и любой отказ, кроме гонки версий, валит стенд (`onlyRetriedRefusals`). `TestEveryProposalPassesTheOwnershipTable` EPIC-003 сверяет с той же таблицей. Не покрыто: нормы, которые читают мир, и законы в процессе (`contexts_state.go` строит State без `Invariants`, бэклог исполнителя) |
| 6 | `WorldRequired` (`registry_state.go`) | Политика стоит у обоих типов предложений. `TestAProposalWithoutAWorldIsReported` проверяет `ErrPolicyViolation` при `Publish` и `Warn` при чтении без проверки, двойник отвечает так же. Издатели `entity.*.proposed` в T-056, EPIC-003 и EPIC-004 передают мир: `Harness.root` — `NewRoot(…, h.worldID, …)`; `Harness` и `FakeEncounter` — `Derive` от событий мира; `laws.Bump` — `NewRoot(TypeChanged, …, world, …)` → `Derive`; `internal/gateway/actions/publish.go` — `Derive(action, …)`; bootstrap стенда — `NewRoot(…, world, …)`. `go test -tags e2e ./test/e2e/...` — ok |
| 7 | `FakeEncounter` | Правка `wound` (`fake_encounter.go:1173-1190`) минимальна и по смыслу совпадает с T-452 на кончике EPIC-003. `swingUntilTheWolfFalls` (`fake_encounter_test.go:1611-1624`) дефектов не маскирует. Мутант исполнителя B2 красный. Смерть персонажа проверяют `TestTheCharacterDies`, а на кончике EPIC-003 ещё `TestAFallenCharacterIsProposedOnlyWhatTheOwnershipTableAllows`. Индекс `2` — это `rollNPCCounter` (`fake_encounter.go:123`). Тот же литерал уже используется в этом файле в семи местах (`:135`, `:315`, `:383`, `:1007`, `:1076`, `:1147`, `:1212`), замечанием не считаю. Если константа изменится, помощник перестанет отводить укус, и тесты исхода упадут громко, а не пройдут ложно |
| 8 | Стенд I1-α на `state.Context` | `MV_STATE_WORLDS` оставлен по умолчанию (`dark-forest-world`). Хук `fake_contexts.go` и стенд `FakeState` не строят (grep). `bootstrap` только публикует, мир читается у контекста процесса. Один State на мир — **по коду**, тестом не закреплено: мутант R7 (второй `FakeState` на том же мире) прошёл, см. Mi-4. Флейк ≈ 5,2·10⁻⁵ — известный с T-255 (`TestTheProcessTellsTheDeathOfACharacter`, волк 8 раз промахнулся при UUID-id). От State он не зависит, T-056 его вероятность не меняет. Прогон 100 раз × (16 боёв + смерть + ожидание мира) — зелёный (51 с) |
| 9 | Мутанты (копия дерева в scratch `t056r1-base`, без `-overlay`, контрольный первым, исходник восстанавливался после каждого) | **M0 (контрольный)** — зелёный. **Красные:** R4 — rest `hp >= hp_max` отвергается (матрица и бои стенда); R6 — законы в обратном порядке (`TestTheFirstBrokenLawIsTheSameOnEveryRun`); R8 — владение раньше шага 5 (матрица). **Выжили:** R1 — дедуп без окна `proposal_id` (Mi-2); R2 — префикс без дочерних путей (`inventory[0]`, `position.x`) (Mi-3); R3 — неатомарный пакет: нарушение закона вне пакета пропускается (Ma-1, у ветки нет теста); R5 — агент с неизвестным уровнем получает права по источнику (Mi-3); R7 — второй State на мире стенда, на шине по 5 `entity.created` и 3 `entity.updated` от `core/state` и `testkit/state` (Mi-4) |
| 10 | Зонды (временный тестовый файл в той же копии) | **A** — Ma-1. **B** — Mi-1. **C/D** — Ma-2. **E3** — тесты `shared/testkit/swarm` с кончика EPIC-003 (`fake_encounter.go`, `fake_encounter_test.go`, `fake_narrator_test.go` из `9691699`) на двойнике T-056: красные `TestTheWolfDies`, `TestTheEncounterEntityRecordsTheFightItHeld`, `TestTheTrophyIsHandedOutOnce` (Mi-5) |
| 11 | Прогоны в рабочей папке (go1.26.8, windows/amd64) | `go build ./... && go vet ./...` — 0. `go test -short -count=1 ./internal/state/... ./internal/mechanics/... ./shared/testkit/... ./shared/entity/... ./shared/contracts/... ./cmd/multiverse/...` — 13 пакетов ok. `golangci-lint run ./...` — 0 issues. `gofmt -l cmd internal shared` — пусто. В копии: `-count=20` по `internal/state`, `shared/testkit/{state,swarm,gateway}` — ok; `go test -tags e2e ./test/e2e/...` — ok (20 с) |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Ma-1 | Major | `internal/state/invariants.go:137-140`; `internal/state/apply.go:308-314` | **Набор неатомарного пакета остаётся без ответа.** Если закон отвечает на сущность, которой нет среди оставшихся наборов, `checkLaws` отвергает остаток пакета одним отказом на эту сущность (`i < 0`). Когда её набор уже отвергнут раньше, на шаге 3–7, `applyUpdate` отбрасывает второй отказ по `Ref.ID`. В итоге оставшиеся наборы не получают ни факта, ни отказа. Это нарушает C-02: «`atomic=false` — по одному `rejected` на сущность», и издатель ждёт ответа, который не придёт. **Зонд A:** мир `forest`, `wolf-alpha` мёртв; пакет `task`, `combat`, `atomic=false`: `wolf-alpha inc hp -1` → `dead_entity`, `enc-1 set round_seq 2` → inv-01 на `wolf-alpha`. Опубликован только `[entity.update.rejected wolf-alpha dead_entity]`, у `enc-1` нет ни факта, ни отказа, версия прежняя. Ветку `i < 0` не проверяет ни один тест: мутант R3 («пропустить нарушение вне пакета и применить остаток») выжил | Ни один оставшийся набор не должен уходить без ответа. Предложение: при `i < 0` отвергать **каждый** оставшийся набор отдельным отказом с его сущностью, `law_violation` и `invariant_id` закона. Id отказов выводятся из разных сущностей и не схлопываются. Если правило для нарушения вне пакета выберет system-architect (открытый вопрос 2 карточки), ответ на каждый набор остаётся обязательным при любом выборе. Тесты: сценарий зонда A — ответ есть у каждого набора; нарушение вне пакета при `atomic=false` без предшествующего отказа. Мутант R3 должен краснеть |
| Ma-2 | Major | `internal/state/proposal.go:180-189` (`malformedOp`); `internal/state/ownership.go:139-154` (`normAllows` по `rootOf`), `:171-177` (`underPrefix`), `:203` (`c.Path != entity.AttrStatus`); грамматика `shared/entity/path.go:36-71` | **Путь не в каноническом виде обходит нормы и матрицу статуса.** `splitPath` отбрасывает ведущие точки и пустые сегменты, поэтому `status.`, `.status` и `a..b` пишут тот же атрибут. Проверки State сравнивают строку пути: `rootOf(".description") == ""`, `underPrefix("status.", "status")` — да, а `Change.Path == "status."` ≠ `"status"`, и `statusRefusal` набор пропускает. **Зонд C** (законы включены): агент `task`, `combat`, `set "status." abandoned` → `entity.updated`, персонаж `abandoned` (C-02 v1.2 требует `level_violation`). То же со значением `sleeping` (статуса нет в матрице, должен быть `invalid_op`) и gateway `forget`, `set "status." dead` — все применены. **Зонд D:** domain `set ".description"` у региона и gateway `set ".encounter_id"` у группы применены, хотя обе строки таблицы делают для этих путей исключения. В факт уходит `changed[].path = "status."`, и читатель по правилу догона C-02 v1.6 разбирает этот путь своей грамматикой | На шаге 1 (`malformedOp`) отвергать `invalid_op` на всё предложение путь, который не совпадает со своей канонической записью: сегменты через `.`, индексы в `[n]`, без ведущих и хвостовых точек, пустых сегментов и `[]`. Тогда все проверки шагов 5–6 видят один вид пути. По C-02 v1.5 это «форма пакета», причина `invalid_op`. Строки матрицы: `status.` от `task`, `.description` от `domain`, `.encounter_id` группы от gateway, `a..b` от author → `invalid_op`. Правило грамматики для всех читателей (`shared/entity`) — вопрос к system-architect, бэклог п. 1 |
| Mi-1 | Minor | `internal/state/ownership.go:228-234` (`restRefusal`), `apply.go:405-411` | «Во встрече» читается по сущности **после** операций. Gateway может записать `encounter_id` игрока с любой своей причиной, включая `rest` (строка таблицы — произведение путей и причин). Пакет `rest {set hp 10, set encounter_id ""}` у игрока внутри `enc-1` применяется (**зонд B**: `entity.updated`). КД §4.6: «`encounter_id == ""`» — у сущности, над которой отдыхают. C-02 v1.1: rest — это `set hp = hp_max` | Условие «во встрече» проверять по `current` (сущность до применения). Добавить норму: gateway с `cause=rest` пишет только `hp` (иначе `level_violation`, C-02 v1.5). Строка матрицы — сценарий зонда B. Сужение строки gateway по `encounter_id` — владелец строки (EPIC-004), бэклог |
| Mi-2 | Minor | `internal/state/dedup.go:26-28`; `internal/state/dedup_test.go` | Окно `proposal_id` не закреплено ни одним тестом: мутант R1 (только `last_change`) выжил. Без окна повтор P после применения Q к той же сущности решается заново. Update применится второй раз (без `expected_version`), create получит `duplicate_entity` вместо молчания (КД §4.5, абзац о create: «с тем же `proposal_id` — дедуп») | Два теста на `Applier` с окном по умолчанию: (1) update P → update Q той же сущности → P новым событием: ответов нет, версия +2; (2) create P → update Q созданной сущности → P новым событием: ответов нет. R1 должен краснеть |
| Mi-3 | Minor | `internal/state/matrix_test.go:178-182`, `:207-210`; `internal/state/ownership.go:51-54`, `:176` | Два правила владения без стражей. (1) Дочерний путь под префиксом строки (`position.x` у gateway, `inventory[0]` у `task`) не проверен: строка «an item of the inventory» пишет ровно `inventory` через `append`. Мутант R2 («только точное совпадение») выжил. (2) «Агент с уровнем не из пяти не получает прав» проверено источником `swarm`, у которого строки нет вообще. Мутант R5 (фолбэк на источник) выжил. С ним `gateway` + `meta.agent{level: gateway}` + `forget` переводил бы в `abandoned`, а C-02 v1.2 говорит: с `meta.agent` → `level_violation` | (1) Строка матрицы: `task`, `loot`, `remove inventory[0]` (или `set inventory[0].kind`) над игроком с предметом → применяется. (2) Строка: источник `gateway`, `meta.agent{level: gateway}`, `forget`, `set status abandoned` → `level_violation`. R2 и R5 должны краснеть |
| Mi-4 | Minor | `cmd/multiverse/fake_contexts_test.go:416-420`, `:441-443`, `:681-700`; DoD (приёмка T-055): «у каждой сущности один факт на версию» | «Один State на мир стенда» держится только кодом. Второй State на том же мире ни один тест стенда не заметит: мутант R7 (`FakeState` на мире фикстур в `bootstrap`) — все бои зелёные, на шине по 5 `entity.created` и 3 `entity.updated` от `core/state` и от `testkit/state`. `onlyRetriedRefusals` видит только отказы, а счёт нарративов и ожидание фактов дубликатов не различают | В `fightThroughTheProcess` после боя проверить, что каждый `entity.created`/`entity.updated` на `system_events` опубликован источником `core/state` и что пара (сущность, версия) встречается один раз. В `cmd/multiverse` есть образец `assertOneFactPerVersion` в `internal/state`. R7 должен краснеть |
| Mi-5 | Minor | `shared/testkit/swarm/fake_encounter.go:1173-1190`, `fake_encounter_test.go:406-409`, `:496-499`, `:529-533`, `:587-590`, `:620-648`; `fake_narrator_test.go:537`; кончик EPIC-003 `9691699` (T-452) | **Совместимость с кончиком EPIC-003.** T-452 правит тот же `wound` и `TestTheCharacterDies` другим текстом и добавляет `TestAFallenCharacterIsProposedOnlyWhatTheOwnershipTableAllows`. Тесты кончика EPIC-003 на двойнике T-056 **красные** (зонд E3): `TestTheWolfDies`, `TestTheEncounterEntityRecordsTheFightItHeld`, `TestTheTrophyIsHandedOutOnce` — там остался `swingUntilOver`. Когда EPIC-002 и EPIC-003 сойдутся в `develop`, будет текстовый конфликт, и при неудачном разрешении — три красных теста | (1) В T-056 взять `wound` дословно из T-452 (`9691699`), чтобы `fake_encounter.go` сливался без конфликта. (2) В карточку — рецепт слияния: `fake_encounter.go` — T-452; в `fake_encounter_test.go` — тесты T-452 плюс `swingUntilTheWolfFalls` T-056 в четырёх тестах исхода и в `fake_narrator_test.go:537`; проверка — `go test ./shared/testkit/swarm/ ./cmd/multiverse/` на слитом дереве. (3) Рецепт и правку `wound` согласовать с tech-lead#2 EPIC-003 |
| N-1 | Nit | `internal/state/dedup_test.go:113-204` | `TestAStoppedWorldAnswersAnOrdinaryError` и `TestAStopDuringPublishFailedDeliversTheFactsAfterARestart` лежат в файле шага 2 (дедуп), хотя проверяют остановку мира | Перенести в `context_test.go` рядом с `TestAStopThatEndsTheAttemptsStopsTheWorld` |
| N-2 | Nit | `internal/state/invariants.go:55-60`, `:129-131`; `internal/state/memstore/memstore.go:79-93` | `overlay` на каждое решение делает глубокую копию всего мира (`List` клонирует и сортирует). В неатомарном пакете копия повторяется на каждой итерации пересчёта, а `ByType` на каждый вызов закона обходит всю map. Для мира MVP-1 (единицы–десятки сущностей) это ≪ 1 мс, но цена растёт с числом сущностей и длиной `history` | Строить вид мира без копий оригиналов: читать `memstore` только на чтение и подменять копиями лишь затронутые сущности (законы вид не меняют). Либо записать ограничение в §4.5 и вернуться к нему с замером p95 (C-02: ≤ 50 мс) |

### Открытые вопросы (к system-architect, через оркестратора)

1. **Грамматика пути (Ma-2).** Отвергать запись не в каноническом виде на уровне `shared/entity` (`ApplyOps` и правило догона для всех читателей) или только на шаге 1 State? В первом случае это `contract-change` C-02/C-03 (T-050).
2. **Нарушение закона вне пакета при `atomic=false` (Ma-1, вопрос 2 карточки).** Какую сущность называет отказ: каждого оставшегося набора или ту, на которую ответил закон? Сейчас код в одном случае теряет ответ.
3. **`expected_version` для путей боя.** C-02 v1.1 требует его для `hp`, `status`, `inventory`, `position` игрока и NPC в бою. State отсутствие версии не проверяет, двойник тоже. Это обязанность издателя или норма State? Если норма, то с какой причиной?

### Риски и допущения

- `-race` недоступен (нет cgo). Порядок доступа к `Applier.laws` (`SetInvariants` под `mu`) и сериализация `FakeState.Apply` (`deciding`) проверены чтением кода.
- Совместимость с EPIC-004 проверена только чтением: на кончике `ce7b35a` настоящий gateway в `--contexts=all` (T-303). После слияния его предложения на стенде пойдут через настоящий State с таблицей, а до T-056 их принимал мир `world-of-no-stand`. Если издатель шлюза нарушает таблицу, стенд покажет это отказом.
- Флейк ≈ 5,2·10⁻⁵ стенда — известный (T-255), в 100 прогонах не проявился. Иной нестабильности не нашёл.
- Копия `t056r1-base`, скрипт `t056r1_mutants.py` и временный листинг удалены по точным путям. В рабочей папке, кроме этого раздела и строки карточки, ничего не менял. Docker, `.env` и стенд `:8888` не трогал.

### Предложения в бэклог

1. **system-architect / EPIC-002 (T-050):** каноническая грамматика пути в `shared/entity`: `splitPath` отвергает пустые сегменты и ведущие и хвостовые точки, правило догона C-02 v1.6 ссылается на неё (см. вопрос 1).
2. **system-architect:** КД §4.5 п. 2 — «у **любой** целевой сущности `last_change.proposal_id == proposal_id`», с доводом про частично применённый неатомарный пакет (п. 2 «Проверено»).
3. **EPIC-004 (владелец строки gateway):** сузить `encounter_id` игрока в строке gateway до причин, где шлюз его пишет (`leave`, `group`?), или записать норму поверх таблицы (Mi-1).
4. **EPIC-002 / T-057–T-059:** опция двойника `WithOwnership()` для потребителей, которые хотят проверять права без стенда процесса. Загрузка `rules/dark-forest.yaml` в `contexts_state.go` (поддерживаю бэклог исполнителя).

## T-056 · ревью #2 · 2026-09-13 · code-reviewer#3 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-056`, ветка `task/T-056-state-invariants`, база `862c8c0`, коммитов нет. Смотрел правки итерации 2 по карточке («Итерация 2») и регрессию от них: `internal/state/{invariants,apply,proposal,ownership}.go`, `matrix_test.go`, `dedup_test.go`, новый `stopped_world_test.go`, страж в `cmd/multiverse/fake_contexts_test.go`, `shared/testkit/swarm/*`. Остальное принято в ревью #1 и повторно не проверялось.

Решения оркестратора (сверял, не оспариваю):
- в неатомарном пакете каждый набор получает факт или отказ;
- неканонический путь — `invalid_op` на шаге 1 State;
- rest — «во встрече» по сущности до операций;
- `wound` берётся из T-452 побайтно, тесты EPIC-003 проходят.

### Вердикт

**Вернуть.** Critical: 0 · Major: 1 · Minor: 1 · Nit: 1.

Из ревью #1 закрыты все девять замечаний: Ma-1, Ma-2, Mi-1…Mi-5, N-1 исправлены, N-2 ушло в бэклог. Возврат из-за одного дефекта. Он того же класса, что Ma-2, но путь записан в каноническом виде, поэтому правка шага 1 его не ловит.

**Ma-3.** `status.x` и `hp.x` превращают скаляр в объект мимо матрицы статуса и нормы rest. Воспроизведено зондом, с законами и без. В ревью #1 я этот обход пропустил: рецепт Ma-2 («привести путь к одному виду») его не закрывал. Исполнитель рецепт выполнил, это не его промах.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | **Ma-1**: ветка `i < 0` (`invariants.go:141-151`, `apply.go:291-311`) | Наборы, отвергнутые на шагах 3–7, в `plans` не попадают. `checkLaws` отвечает только за `kept`. При `i >= 0` набор отвергается и удаляется из `kept`. При `i < 0` каждый оставшийся набор получает отказ под своей сущностью (`plan.entity.Ref()`, тип из мира) с `invariant_id` закона, и `kept = nil`. Факты строятся только из `kept` после `checkLaws`, публикация идёт одним вызовом после решения. Поэтому **двойного ответа нет, и отказа на уже применённый набор нет**: у каждого набора ровно один ответ. Id отказов различаются, потому что `WithCauseID(entity)`, а сущность в пакете одна на набор (п. 1а). Фильтр по `Ref.ID` снят обоснованно. **Порядок:** сначала факты по возрастанию id, затем отказы шагов 3–7 в порядке пакета (в том числе `dead_entity`), затем отказы шага 8. Порядок детерминирован. КД §4.5 п. 9 и C-02 порядок отказов не задают, `TestEverySetOfALoosePackageIsAnswered` закрепляет `[dead_entity, law_violation]`. **Зонды:** (a) 4 набора, `task`/`combat`/`atomic=false`: мёртвый `wolf-dead` → `dead_entity`; `player-A`, `enc-1`, `enc-2` → `law_violation {inv-01}` под своими сущностями, у каждого по одному ответу; (b) `author`: `player-A hp 50`, `wolf-alpha inc hp -1`, `enc-2` при мёртвом NPC → три отказа `inv-01`, по одному на набор. Мутанты R3 и R3b исполнителя, а также мои K2 и K3 красные |
| 2 | **Ma-2**: грамматика (`proposal.go:204-227`) | Регулярное выражение сегмента `^[^.\[\]]+(\[[0-9]+\])*$`. **Отвергаются** `invalid_op` на всё предложение (зонд, `author`, игрок): `mark[`, `mark[x]`, `inventory[-1]`, `[0]`, `mark..fox`, `inventory[0]b`. Все пять строк матрицы исполнителя на месте (`matrix_test.go:285-304`). **Проходят**, и это законно: ` mark` и `my mark` (пробел — часть ключа, `splitPath` пробелы не срезает, второго написания того же атрибута нет), `метка.лис` (UTF-8, `[^.\[\]]` — любой символ), `inventory[0]`, `position.x`. **Второе написание:** `inventory[01]` удаляет `inventory[1]` (N-3). **Законные пути издателей не задеты.** Grep по `.worktrees/EPIC-003` (`35650f8`) и `.worktrees/EPIC-004` (`745f6da`), только чтение. `internal/gateway/actions/publish.go` пишет `position` и `hp`. `internal/mechanics/changes.go` пишет `hp`, `status`, `died_at`, `killed_by`, `loot_claimed_by` и `inventory` (append). `internal/laws/laws.go:524` пишет `laws_version`. `testkit/gateway/harness.go` пишет `hp` и `position`. `testkit/swarm/fake_encounter.go` пишет `round_seq`, `state`, `resolution`, `closed_by_event_id`, `participants`, `position`, `hp`, `status`, `died_at`, `killed_by`, `loot_claimed_by`, `inventory`, а в `:1642` — `change.Path` из `ChangesFor`, то есть те же константы. У всех издателей корневые константы `entity.Attr*`, ни одна не содержит `.` или `[`. `readmodel/apply.go` строит операции из фактов для своего read-model, в State не публикует. Фикстуры создаются через `attributes`, путей в них нет. **Обход каноническим путём** — Ma-3 |
| 3 | **Mi-1…Mi-3** | На месте. Mi-1: `restRefusal` читает `before.EncounterID()`, «не выше `hp_max`» проверяет по `after`; строка матрицы `:324-331`. Mi-2: `TestTheWindowRecognisesARepeatTheCommitRecordNoLongerCarries` (update и create). Mi-3: строки `:227-238`. Мутанты (мои прогоны): **R1** красный (`an_update`: три факта вместо двух; `a_create`: `duplicate_entity`), **R2** красный (`inventory[0]` → `level_violation`), **R3** красный (оба теста Ma-1), **R5** красный (строка «gateway source with meta.agent of level gateway») |
| 4 | **Mi-4**: страж `oneStateOverTheWorld` | (1) **Исключение «пустой `changed[]`» второй State не маскирует.** Второй State публикует и непустые факты, а они в счёт идут. Мутант K6 (исключение снято) красный: `player-A v3 is announced twice` с разными id. Второе объявление — факт без изменений под той же версией, законное по C-02, так что исключение нужно. (2) **Ложных срабатываний нет.** `go test -count=20 -run 'TestTheProcessRunsTheFightsOfIAlpha\|TestTheProcessTellsTheDeathOfACharacter\|TestTheStandWaitsUntilTheFakeHasLearntTheWorld' -v ./cmd/multiverse/` в рабочей папке: 60/60 PASS, 7,3 с. (3) **Слабое место — исключение по id** (`seen && first != ev.ID`), см. Mi-6. Id фактов выводятся из предложения (`WithCauseID`), поэтому второй State той же реализации публикует те же id. Половина стража «(сущность, версия) один раз» тогда не срабатывает никогда, и держит его только проверка источника. R7 (второй `FakeState`, источник `testkit/state`) красный. K4 (R7 без проверки источника) и K5 (каждый ответ State публикуется дважды, как от второго `core/state`) **выжили**. Строгий вариант без исключения по id: K7 зелёный 10/10, K8 (строгий + K5) красный |
| 5 | **Mi-5**: `shared/testkit/swarm` | `fake_encounter.go` **побайтно совпадает** с `epic/EPIC-003-swarm-llm-laws:shared/testkit/swarm/fake_encounter.go` (`35650f8`), sha256 `640d8813…` у обоих. `fake_encounter_test.go` отличается от кончика EPIC-003 ровно правкой T-056: 4 замены `swingUntilOver` → `swingUntilTheWolfFalls` (`:409`, `:492`, `:532`, `:590`) и новый помощник после `swingUntilOver`. `fake_narrator_test.go` — одна строка `:537`. **Рецепт слияния проверен.** `git merge-file` трёх файлов (ours — T-056, base — `c39cd18` = merge-base `epic/EPIC-002` и `epic/EPIC-003`, theirs — кончик EPIC-003) дал 0 конфликтов, результат побайтно равен версии T-056. У `862c8c0`, `e33246b` (`epic/EPIC-002`) и `b0e520b` (`develop`) эти три файла одинаковы, так что база не влияет. **Слитое дерево** (архив кончика EPIC-003 + `git diff 862c8c0` T-056 без `Docs` и `swarm` + неотслеживаемые файлы `internal/state` + три слитых файла): `go build ./...` — 0; `go test -count=3 ./shared/testkit/{swarm,state,gateway}/ ./cmd/multiverse/ ./internal/state/` — ok; `go test -tags e2e ./test/e2e/...` — ok. Шаг 5 рецепта выполнен |
| 6 | **N-1** | Оба теста в `stopped_world_test.go`, в `dedup_test.go` их больше нет (grep) |
| 7 | Мои мутанты (копия `scratchpad/t056r2-base`, без `-overlay`, контрольный первым, исходник восстанавливался после каждого) | **C0** — зелёный (`internal/state` и три теста стенда). **K1** (`canonicalPath` срезает ведущую точку) — красный: строки `.description` и `.encounter_id`. **K2** (при `i < 0` отказ получает только первый набор) — красный: `TestALawOutsideALoosePackageRefusesEachRemainingSet`. **K3** (отказ `i < 0` без `invariant_id`) — красный: оба теста Ma-1. **K4** (R7 + страж без проверки источника) — **выжил**. **K5** (двойная публикация ответа State) — **выжил**. **K6** (страж без исключения пустого `changed[]`) — красный. **K7** (страж без исключения по id), 10 прогонов — зелёный. **K8** (K7 + K5) — красный |
| 8 | Зонды (временные тесты в той же копии) | **G** — грамматика, п. 2. **L** — неатомарные пакеты, п. 1. **S** — Ma-3: с законами (`forest`) и без (`newOwnedFixture`, как State в процессе) |
| 9 | Прогоны в рабочей папке (go1.26.8, windows/amd64) | `go build ./... && go vet ./...` — 0. `go test -short -count=1 ./internal/state/... ./internal/mechanics/... ./shared/testkit/... ./cmd/multiverse/...` — 11 пакетов ok. `golangci-lint run ./...` (2.13.2) — 0 issues |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Ma-3 | Major | `internal/state/ownership.go:201-217` (`statusRefusal`: `c.Path != entity.AttrStatus`), `:232-245` (`restRefusal`: `after.HP()`); `shared/entity/path.go:146-154` (скаляр по пути становится объектом) | **Дочерний путь скалярного атрибута обходит матрицу статуса и норму rest.** `set status.x` — канонический путь, и шаг 1 его пропускает. Строка `task` даёт `status` по префиксу, `normAllows` проверяет корень. `ApplyOps` заменяет строку `alive` объектом `{x: …}`, `changed[].path = "status.x"`, и `statusRefusal` этот набор не смотрит. **Зонд S** (одинаково с законами и без): `task`/`combat` `set status.x abandoned` → `entity.updated`, `status = {x: abandoned}` (C-02 v1.2 требует `level_violation`); `set status.x sleeping` → применено (статуса нет в матрице, нужен `invalid_op`); gateway/`forget` `set status.x dead` → применено. После этого `IsTerminal` и `Status()` статус не читают, персонаж не жив и не мёртв. Следующий `set status alive` пройдёт матрицу, потому что `from` не строка. Gateway/`rest` `set hp.x 999` **без законов** (так State работает в процессе, `contexts_state.go`) → `entity.updated`, `hp = {x: 999}`: `after.HP()` не число, и потолок `hp_max` не проверяется. С законами inv-02 отвергает. Для рои агентов EPIC-003, чьи предложения идут от LLM, State — единственная защита | Решать переход статуса и rest по сущностям, а не по строке пути. В `plan` после `ApplyOps`, если набор касается корня `status` (`touchesRoot`): `after.Status()` должен вернуть строку, иначе `invalid_op`; переход проверять как `StatusTransitionAllowed(current.Status(), after.Status())` вместе с нормами `abandoned`. Для `hp` при `rest` так же: `after.HP()` обязан быть числом, иначе `invalid_op`. Такой вариант ловит любое написание. Проще, но уже: на шаге 1 отвергать пути под `status`, `hp`, `hp_max` (скаляры модели данных §3). Строки матрицы: `task` `status.x abandoned`, gateway `forget` `status.x dead`, gateway `rest` `hp.x` без законов → `invalid_op`. Мутант «переход по `changed[].path`» должен краснеть |
| Mi-6 | Minor | `cmd/multiverse/fake_contexts_test.go:703-706` (`oneStateOverTheWorld`) | Исключение `first != ev.ID` делает проверку «(сущность, версия) один раз» бесполезной против второго State. Id фактов выводятся из предложения, поэтому любой второй State (двойник или второй `core/state`) публикует те же id. Страж держится только на проверке источника и не видит второй `internal/state` на том же мире: K4 и K5 выжили. Строгий вариант ложных срабатываний не даёт (K7: 10/10) и ловит двойную публикацию (K8) | Снять `&& first != ev.ID`. Для фактов с пустым `changed[]` проверять, что id не встречается дважды. K4 и K5 должны краснеть |
| N-3 | Nit | `internal/state/proposal.go:204` | `\[[0-9]+\]` пропускает ведущие нули: `inventory[01]` адресует `inventory[1]` (`sliceIndex` → `Atoi`), а `changed[].path` уносит `inventory[01]`. Нормы индексы не сравнивают, поэтому обхода нет, но это второе написание одного пути, против цели Ma-2. Сюда же `position[99999999999999999999]`: у объекта это ключ, `isIndex` → false | Индекс `\[(0\|[1-9][0-9]*)\]` и строка матрицы `inventory[01]` → `invalid_op`. Предельную длину индекса — по решению system-architect о грамматике (открытый вопрос 2 карточки) |

### Открытые вопросы (к system-architect, через оркестратора)

1. **Ma-3.** Скалярные атрибуты модели данных (`status`, `hp`, `hp_max`, …) защищать от записи дочерним путём в `shared/entity.ApplyOps` для всех читателей (правило «скаляр по пути становится объектом», КД §3.2) или только в State? В первом случае это `contract-change` C-03, вместе с вопросом 2 карточки о грамматике.
2. Вопросы 1–7 карточки после итерации 2 остаются открытыми, новых данных по ним нет.

### Риски и допущения

- `-race` недоступен (нет cgo), параллельный доступ повторно не проверялся: итерация 2 его не трогает.
- Слитое дерево собрано вручную (архив EPIC-003 + патч T-056), это не настоящий `git merge` веток. Конфликтов вне `shared/testkit/swarm` на слиянии веток всё равно ждать можно, в документах и файлах, которые правили обе стороны. Эта проверка их не покрывает.
- 20 прогонов стенда — 60 из 60. Известный флейк T-255 (≈ 5,2·10⁻⁵) не проявился.
- Копии `t056r2-base` и `t056r2-merge`, скрипт `t056r2_mutants.py`, выгрузки `t056r2-e3-*.go` и черновик раздела удалены по точным путям. В рабочей папке, кроме этого раздела и строк карточки, ничего не менял. Docker, `.env` и стенд `:8888` не трогал.

### Предложения в бэклог

1. **system-architect / EPIC-002 (T-050):** правило «скаляр по пути становится объектом» (`setIn`, default-ветка) для атрибутов с типом из модели данных: запрет или `invalid_op` в `ApplyOps` (вопрос 1).
2. **EPIC-002 / T-057–T-059:** страж стенда `oneStateOverTheWorld` перенести в общий помощник, чтобы им пользовались e2e и будущие стенды с настоящим State.

## T-056 · ревью #3 · 2026-09-13 · code-reviewer#3 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-056`, ветка `task/T-056-state-invariants`, база `862c8c0`, коммитов нет. Проверял только правки итерации 3 (карточка, «Итерация 3») и регрессию от них: `internal/state/{ownership,proposal,apply}.go`, `matrix_test.go`, новый `plan_test.go`, страж `oneStateOverTheWorld` в `cmd/multiverse/fake_contexts_test.go`. Всё остальное принято в ревью #1 и #2.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 2 · Nit: 1.

Ma-3, Mi-6 и N-3 из ревью #2 закрыты. Новых Critical и Major нет. Замечания Mi-7, Mi-8 и N-4 не блокируют и уходят в приёмку: неполный список скаляров во втором слое защиты Ma-3 и три непокрытые границы в тестах.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | **Ma-3, слой 1: решение по сущности** (`ownership.go:205-226`, `:245-262`; вызов `apply.go` в `plan` после `ApplyOps`) | `statusRefusal` срабатывает, когда набор касается корня `status` (`touchesRoot`). `from` берётся из сущности до операций, `to` — после. Если после операций статус не строка или отсутствует, ответ `invalid_op`. Статус вне матрицы — `invalid_op`. Нормы `abandoned` и строка gateway прежние. Статус в то же значение не считается изменением. `restRefusal`: «во встрече» читается по `before`. `hp` после операций проверяется так: текст или не целое число — `invalid_op`, `hp_max` не целый или отсутствует, либо `hp` вне `[0, hp_max]` — `law_violation {inv-02}`. У gateway нет права писать `hp_max` (`contracts/ownership.go`, строка gateway/player), поэтому поднять потолок тем же набором нельзя |
| 2 | **Ma-3: четыре зонда ревью #2** (временный тест в копии, через `Applier.Apply`; `forest` с законами и `newOwnedFixture` без законов, как State в процессе) | С законами и без ответ одинаков. Z1 `task`/`combat` `set status.x abandoned` → `invalid_op`. Z2 `status.x sleeping` → `invalid_op`. Z3 gateway/`forget` `status.x dead` → `invalid_op`. Z4 gateway/`rest` `hp.x 999` → `invalid_op`. Сверх зондов: `task` NPC `status.x dead` → `invalid_op`; rest `hp 11` при `hp_max 10` → `law_violation inv-02`; rest `hp 5.5` → `invalid_op`; rest персонажа без `hp_max` → `law_violation inv-02`; rest `hp 10 = hp_max` → применено. Слой 1 отдельно от шага 1 закреплён тестом `TestTheStatusAndTheHitPointsAreReadOnTheEntity`: он вызывает `plan` мимо грамматики |
| 3 | **Ma-3, слой 2: `scalarAttributes`** (`proposal.go:238-248`) против `data-model.md` §3 | **Лишних нет.** `hp`, `hp_max`, `atk`, `def`, `dmg`, `flee`, `status`, `position` (§3.3 и §3.4: string, не объект), `actor_kind`, `group_id`, `encounter_id`, `kind`, `died_at`, `killed_by` — скаляры модели. `loot_claimed_by` в таблицах §3 нет, но это ссылка-строка из inv-03 и таблицы владения. `scope` не включён, и это верно: он бывает объектом `{id, type}`. **Пропущены** скаляры других типов — Mi-7. Зонды X1–X5 (с законами и без) применяются и превращают скаляр в объект: `task`/`resolve` `state.x` у встречи, `domain`/`tick` `respawn_ttl.x` у региона и `region_id.x` у NPC, gateway/`group` `leader_id.x`, `author` `weather.x` |
| 4 | **Ma-3: издатели не задеты** (только чтение: `.worktrees/EPIC-003` `35650f8`, `.worktrees/EPIC-004` `d1d6331` — кончик сдвинулся с `745f6da` ревью #2) | Все операции издателей строятся из корневых констант `entity.Attr*`: `internal/gateway/actions/publish.go:78,84`, `internal/mechanics/changes.go:129-197`, `internal/laws/laws.go:524`, `shared/testkit/gateway/harness.go:929,1140`, `shared/testkit/swarm/fake_encounter.go` (включая `change.Path` из `ChangesFor`, это те же константы). Вложенных путей под скалярами нет. Попадания grep `"(hp\|status\|position\|…)[.\[]"` относятся к payload действий (`position.from`/`position.to`, `hp.defender_before`), к тестам `ApplyOps` (`dmg.formula`) и к тестам read-model gateway (`position.region`, `hp.deep`). В State они не публикуются. Удаления `status` в тестах EPIC-003 и EPIC-004 нет |
| 5 | **Ma-3: rest без `hp_max` — `law_violation`, фикстуры** | У всех персонажей `testdata/fixtures/players.json` (`player-A`, `player-B`, `player-C`) и снапшота `snapshots/state/20260101T000000Z-000000.json` есть `hp`/`hp_max` 10/10. В событийных фикстурах и в `rules/dark-forest.yaml` (`player: {hp_max: 10}`) расхождений нет. В кончиках EPIC-003 и EPIC-004 gateway пока не создаёт персонажей (`entity.create.proposed` публикует только bootstrap). Ни в одной фикстуре скаляр не хранится объектом |
| 6 | **Mi-6: строгий страж** (`fake_contexts_test.go:687-714`) | Исключение `first != ev.ID` снято. Добавлены проверка «событие опубликовано дважды» и прежняя проверка источника. Мои мутанты: **K5** (каждый `entity.created`/`entity.updated` State публикуется дважды) — красный, «published twice under …» и «announced twice». **K4** (второй `testkit/state.FakeState` на шине стенда и страж без проверки источника) — красный, 889 строк «published twice» и 0 строк «published by». В ревью #2 оба выжили. **20 прогонов** трёх тестов стенда в рабочей папке: `TestTheProcessRunsTheFightsOfIAlpha`, `TestTheProcessTellsTheDeathOfACharacter`, `TestTheStandWaitsUntilTheFakeHasLearntTheWorld` — 60/60 PASS, 33,8 с, ложных срабатываний нет |
| 7 | **N-3: грамматика индекса** (`proposal.go:207`) | Зонд через `Apply` (`author`, 11 предметов): `inventory[0]` и `inventory[10]` применены, `npcs[0].last_damager` применено, `inventory[01]` и `inventory[00]` → `invalid_op`. Строка матрицы `:305-311` на месте. Положительной строки для многозначного индекса нет — N-4 |
| 8 | Мутанты (копия `scratchpad/t056r3-base`, без `-overlay`, контрольный первым, исходник восстанавливался после каждого) | **C0** — зелёный (`internal/state` и три теста стенда). **K5** — красный (п. 6). **K4** — красный (п. 6). **M1** (`restRefusal` пропускает rest без `hp_max`: `bounded && (…)`) — **выжил** (Mi-8). **M2** (потолок `hp > hpMax+1`) — **выжил** (Mi-8). **M5** (индекс `(0\|[1-9])`, то есть `inventory[10]` отвергается) — **выжил**: `internal/state`, `shared/testkit/state` и `shared/testkit/swarm` зелёные (N-4). Мутанты S1–S5, N3, K5–K7 исполнителя не повторял: их покрывает то же, что проверено в п. 2 и 6 |
| 9 | Прогоны в рабочей папке (go1.26.8, windows/amd64) | `go build ./... && go vet ./...` — 0. `go test -short -count=1 ./internal/state/... ./shared/testkit/... ./cmd/multiverse/...` — 10 пакетов ok. `go test -tags e2e -short -count=1 ./test/e2e/...` — ok. `golangci-lint run ./...` (2.13.2) — 0 issues |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-7 | Minor | `internal/state/proposal.go:232-242` (`scalarAttributes`) | Комментарий обещает «атрибуты, которые `data-model.md` §3 типизирует одним значением», а в списке только скаляры персонажа и NPC. Пропущены: World §3.1 — `laws_version`, `weather`, `time_of_day`, `day`, `season`, `epoch`, `locale`, `blueprint_ref`; Region §3.2 — `description`, `respawn_ttl`, `perception_radius`, `last_background_event_at`, `blueprint_ref` (и `encounter_chance` из `entity.Attr*`); NPC §3.4 — `region_id`; Group §3.6 — `leader_id`, `state`; Encounter §3.7 — `region_id`, `state`, `resolution`, `round_seq`, `task_agent_id`, `opened_by_event_id`, `closed_by_event_id`. Зонды X1–X5 применяются, и скаляр становится объектом. Для рои агентов важен X1: агент встречи пишет `state.x resolved`. **Почему не Major.** Норм и законов State этот обход не минует: нормы сравнивают корень пути, inv-01 без `resolved` проверяет встречу строже. Тот же результат уже даёт запись в корень объектом: `set state {x: resolved}`, `set encounter_id {…}`, а без законов и `set hp {x: 999}` агентом `task` — всё применено (зонды V1–V3). Типы значений State не проверяет вообще, и итерация 3 это не меняла | Дополнить список всеми скалярами §3 (перечень выше) и добавить строку матрицы, например `task`/`resolve` `state.x` → `invalid_op`. Либо сузить комментарий до «атрибутов, на которых стоят нормы State», а полный список вынести к `entity.Attr*` (открытый вопрос 2 карточки). Проверку типа значения в корне — в бэклог (п. 1) |
| Mi-8 | Minor | `internal/state/ownership.go:257-258`; `matrix_test.go:276,362-365`, `:662-676`; `plan_test.go:35-37` | Два новых условия нормы rest итерации 3 тестами не закреплены. (1) «rest без `hp_max` → `law_violation {inv-02}`»: мутант M1 (`!bounded` снят) выжил. (2) Граница потолка: rest проверяется на `10 = hp_max` (применено) и на `50` (отказ), а `hp_max + 1` не проверяется. Мутант M2 (`hp > hpMax+1`) выжил | В `TestTheStatusAndTheHitPointsAreReadOnTheEntity` добавить строки: `rest hp_max+1` (сущность `hp 3`/`hp_max 10`, `set hp 11`) → `law_violation`; `rest of a character without hp_max` → `law_violation`, с проверкой `InvariantID == "inv-02"`. M1 и M2 должны краснеть |
| N-4 | Nit | `internal/state/matrix_test.go:227-238`, `:305-311` | Для индекса в матрице есть только `[0]` (проходит) и `[01]` (отказ). Регулярное выражение, которое пропускает лишь однозначный индекс, прошло бы все тесты (M5). Законный `inventory[10]` тогда получил бы `invalid_op` | Положительная строка `author` `remove inventory[10]` при 11 предметах, либо табличный тест `canonicalPath` на `[0]`, `[10]`, `[01]`, `[00]` |

### Открытые вопросы (к system-architect, через оркестратора)

1. Вопросы 1 и 2 карточки после итерации 3 остаются. Mi-7 и зонды V1–V3 их дополняют: защищать скаляры модели данных нужно не только от записи дочерним путём, но и от значения чужого типа в корне (`set state {…}`, `set hp {…}`). Место проверки — `shared/entity` (C-03) или шаг 1 State.

### Риски и допущения

- `-race` недоступен (нет cgo). Итерация 3 параллельного доступа не касается.
- Зонды и мутанты гонялись в копии рабочей папки (tracked и untracked файлы без `Docs/` и `services/`). Контрольный C0 зелёный, K4 и K5 красные, так что копия исполнялась.
- Слияние с EPIC-003 и EPIC-004 после итерации 3 заново не собиралось. Итерация 3 не трогает `shared/testkit/swarm`, у EPIC-003 кончик тот же (`35650f8`). EPIC-004 сдвинулся на `d1d6331` — проверены только пути издателей (п. 4).
- Копия `t056r3-base`, скрипты `t056r3_mut.py`, `t056r3_k4.py`, выгрузка `t056r3-status-after.txt` и черновик раздела `t056r3-section.md` удалены по точным путям. В рабочей папке, кроме этого раздела и строк карточки, ничего не менял. Docker, `.env` и стенд `:8888` не трогал.

### Предложения в бэклог

1. **system-architect / EPIC-002 (T-050, C-03):** проверять тип значения скалярных атрибутов модели данных при `set` в корень и под корень, одним правилом для всех читателей, со списком рядом с `entity.Attr*` (Mi-7, V1–V3).
2. **EPIC-002 / T-057–T-059:** перенести страж `oneStateOverTheWorld` в общий помощник (повтор предложения 2 ревью #2).

## T-471 · ревью #1 · 2026-09-14 · code-reviewer#1 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-471`, ветка `task/T-471-state-laws-in-process`, база `7fcd02b`, коммитов нет, изменения не закоммичены. Смотрел весь `git diff` против базы (15 файлов): `cmd/multiverse/{contexts_state,contexts_state_test,serve_test}.go`, `test/e2e/empty_world_test.go`, `build/Dockerfile`, `shared/env/vars.go`, `.env.example`, `shared/contracts/ownership{,_test}.go`, `internal/state/{ownership,matrix_test,plan_test}.go`, карточки T-056 и T-471, `dev-log.md`. Сверял с разделом «### T-471» в `tasks.md`; C-02 v1.8 п. 3 и п. 6, «Связка "путь ↔ причина"», «Причина отказа по связке» и «Издатель предложений bootstrap» в `contracts.md`; ответом 7 system-architect в `tasks/T-056.md`; блоком «Передать» T-470 (`develop`); `design.md` §4.3 и §7; КД State §4.5 и §4.6 (строка 362). Для п. 3 проверки смотрел `docker-compose.yml` и правило 9 `scripts/compose-lint.sh` в `.worktrees/T-469`, только чтение.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 2 · Nit: 0.

Норма rest совпадает с C-02 v1.8 п. 3 дословно: причины и порядок «путь → встреча → вид `hp` → значение». Строка `system` пуста, пять тестов ответа 7 на месте. Законы в процессе включены, без книги правил `Start` отказывает. Оба Minor касаются передачи и описания `MV_RULES_PATH` и `rules/` в композиции и тексте для оператора, а не кода State. Их можно закрыть приёмкой или отдельной строкой.

### Проверено

1. **Порядок проверок rest против C-02 п. 3.**
   - Путь проверяет `normAllows`, первая ветка: `gateway` + `player` + `cause=rest` → только корень `hp`. Проверка стоит на шаге 6, до `ApplyOps`.
   - Встреча читается по `before`, вид `hp` и значение — по `after` (`restRefusal`, `ownership.go:261-281`).
   - `hp < hp_max` → `level_violation`; `hp > hp_max` или нет целого `hp_max` → `law_violation {inv-02}`; `hp` не целое (текст, дробь, `remove`) → `invalid_op`; во встрече → `law_violation` без `invariant_id`.
   - Другие предлагающие с `cause=rest` отвергаются самой таблицей. Причины `rest` нет ни в строке `gateway`/`group`, ни у `task`, `domain`, `global`, `author`. Сужать норму только до персонажа шлюза контракт позволяет: ничего не теряется.
2. **Смена ожиданий в строках T-056 следует из контракта, а не из решения разработчика.**
   - «rest, стирающий `encounter_id`» → `level_violation`. `encounter_id` — путь не `hp`, а по п. 3 норма пути (шаг 6) идёт раньше встречи.
   - «rest ниже нуля» → `level_violation`. По п. 3 «`hp < hp_max` — `level_violation`», и строка прямо названа в «Передать» T-470 («строка итерации 3 T-056 "rest below zero → inv-02" меняет ожидание»).
   - Свойство Mi-1 ревью #1 T-056 (встреча читается до операций) через `Applier` больше не наблюдается. Его держит прямой тест `TestRestReadsTheEncounterBeforeTheOperations`, это честно записано в карточке.
3. **Строка `system` (п. 6, ответ 7).** `{Proposer: ProposerSystem}` пуста, константа осталась. Тесты (1)–(4) в `TestNobodyProposesUnderTheRowSystem`, тест (5) — в `TestOwnershipRulesCoverEveryProposer` вместе с `object` и `monitor`. В (2)–(4) `proposerOf` не выводит строку `system` вовсе. Пустоту самой строки держит только (5), как и требует ответ 7.
4. **Dockerfile.**
   - `WORKDIR /home/nonroot` совпадает с рабочей папкой образов distroless `:nonroot`: там домашняя папка пользователя 65532 и `WorkingDir`. Без Docker это не проверено, но и не важно: папка задана явно.
   - Относительных путей в образе, кроме нового, нет. `ENTRYPOINT ["/multiverse"]`, `/data` (`COPY --chown … /data`, том `gateway-data:/data`) и проба compose `["CMD", "/multiverse", "health", …]` абсолютные. `working_dir` в `docker-compose.yml` нет ни у одного сервиса платформы.
   - Из умолчаний манифеста относительное только `MV_RULES_PATH`, у `MV_GATEWAY_DATA_DIR` путь `/data`.
   - `COPY rules/ ./rules/` кладёт файлы `root` 0644, `nonroot` может их читать. `.dockerignore` каталог `rules/` не исключает.
5. **Манифест.** `MV_RULES_PATH` объявлена без `Secret`/`Tooling`/`Required`, и это верно: переменную читает процесс в контейнере, это не секрет, а умолчание рабочее. Стоит в группе State рядом с `MV_STATE_WORLDS`. Описание по форме соседей. Строка `.env.example` совпадает с умолчанием. `mvctl env check` — exit 0.
6. **Тесты процесса и e2e.**
   - `onLoopback` задаёт абсолютный путь к книге дерева (`withTheRuleBook`), `emptyWorldEnv` передаёт тот же путь дочернему процессу.
   - На диск пишут только `t.TempDir()`: битый YAML и папка без `rules/`. `sqlitedir.Temp` был и до задачи.
   - Тесты, которые ждут отказа другого контекста (`TestAMalformedFlagRefusesToStart`, `TestAFakeWithoutRulesNamesTheFlag`), проверяют префикс `start context swarm: `. Отказ `state` без книги сделал бы их красными, а не ложно-зелёными.
7. **Мутанты** (копия `scratchpad/t471r-mutants`: `go.mod`, `go.sum`, `cmd`, `internal`, `shared`, `rules`, `schemas`, `testdata`, `test`; без `-overlay`). `go list -m` указал на копию, базовый прогон зелёный.
   - **Контрольный** — `restRefusal` ничего не отвергает: **красный** (`TestTheMatrixOfRefusals`, `TestTheStatusAndTheHitPointsAreReadOnTheEntity`, `TestRestReadsTheEncounterBeforeTheOperations`, `TestTheNormsOfRestHoldWithoutTheLaws`).
   - **R-A** — rest пишет ещё и `encounter_id` (`root == hp || root == encounter_id`): **красный** (строка «rest inside an encounter that erases encounter_id in the same set»).
   - **R-B** — отказ `loadTheRules` теряет причину (`%w` снят): **красный** (`TestTheStateOfTheProcessDoesNotStartWithoutItsLaws`, случаи «not there» и «does not parse»).
   - После отката файл зелёный. Копия удалена по точному пути.
8. **Прогоны** (go1.26.8 windows/amd64):
   - `go build ./... && go vet ./...` — 0; `gofmt -l cmd internal shared test` — пусто;
   - `go test -short -count=1 ./internal/state/... ./cmd/multiverse/... ./shared/contracts/... ./shared/env/...` — ok (`cmd/multiverse` 32 с);
   - `go test -tags e2e -count=1 ./test/e2e/...` — ok;
   - `golangci-lint run ./...` — 0 issues;
   - `go run ./cmd/mvctl env check` — 0 (75 переменных).

### Замечания

| # | Уровень | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-1 | Minor | `docker-compose.yml:289-317` (`core.environment`); `.env.example:163`; `shared/env/vars.go:158` | `MV_RULES_PATH` объявлена и стоит в `.env.example`, но сервису `core` (`--contexts=state,…`) не передаётся. Оператор, который задал путь в `.env`, молча получит умолчание образа, а `.env.example` не говорит, что композиция значение не пробрасывает. Это тот класс «забыли передать в compose», который T-469 закрывает правилом 9 (T-055, T-310, T-305). Сегодня умолчание рабочее, и на MVP-1 с одной книгой поведение не ломается — поэтому Minor | Добавить в `core.environment` строку `MV_RULES_PATH: ${MV_RULES_PATH:-rules/dark-forest.yaml}` (форма передачи T-469, просмотр tech-lead#1: `docker-compose.yml` — файл EPIC-001, в «Файлах» T-471 его нет). Другой вариант — явно отдать эту строку T-469 или слиянию эпиков, см. «Риски» п. 1 |
| Mi-2 | Minor | `.env.example:171-173`; `cmd/multiverse/fake_contexts.go:52`, `:64`; `cmd/multiverse/fake_contexts_test.go:124-126` | Правка образа сделала неверными три текста об `rules/`. `.env.example` над `MV_SWARM_FAKE`: «в образе его нет — в контейнере с флагом процесс не стартует». Текст отказа фейка: «and the image of the platform has none». Комментарий теста: «which is where the process stands inside the image». Теперь `rules/dark-forest.yaml` лежит в рабочей папке образа, фейк в контейнере стартует, а отказ при повреждённом файле направит оператора искать отсутствующий каталог. Разработчик это видел (открытый вопрос 4), но `.env.example` — файл, который задача и так правит | В этой задаче поправить комментарий `.env.example:171-173`, например: «Заглушка читает rules/dark-forest.yaml рабочего каталога процесса; в образе он есть (T-471), но compose флаг в контейнеры не передаёт». Текст отказа и комментарий теста хука T-255 (EPIC-003) — строкой бэклога T-256/T-447 или правкой через tech-lead#1, если tech-lead#2 EPIC-003 не возражает |

### Открытые вопросы (к оркестратору)

1. Mi-1: передавать `MV_RULES_PATH` в `core` в этой задаче (через tech-lead#1) или отдать строку T-469 / контрольному слиянию эпиков? От ответа зависит, кто добавит строку в `READERS` правила 9 (Риски п. 1).
2. Выбор `MV_RULES_PATH` вместо «`rules/` рабочей папки» требует отметки tech-lead#1 по строке индекса («выбор с tech-lead#1»; открытый вопрос 1 карточки). Ревью выбор поддерживает: имя и умолчание дословно из `design.md` §4.3 и §7.

### Риски и допущения

1. **Риск слияния с T-469 (правило 9 `compose-lint`).** В `.worktrees/T-469` правило 9 требует, чтобы каждая объявленная переменная, кроме `Tooling()`, стояла в `READERS` или `NOT_IN_CONTAINERS` (`compose-lint.sh`, около строк 1795-1800). `MV_RULES_PATH` там нет. Какая из задач придёт в `develop` второй, у той `make compose-lint` упадёт с «MV_RULES_PATH is declared … but rule 9 does not know who reads it». Если добавить строку `READERS`, правило потребует и `MV_RULES_PATH` в `core.environment` (это Mi-1). Строка по `design.md` §4.3 — `("state", "swarm")`. Причину `NOT_IN_CONTAINERS` у `MV_SWARM_FAKE` («the image does not carry rules/») тоже придётся поправить. Это не блокер T-471. Нужна отметка для того, кто разрешает слияние.
2. **Риск для T-472.** `TestTheStateOfTheProcessHoldsTheLaws` ждёт `law_violation {inv-02}` на `set hp {x: 999}` от `task` (DoD T-471). После T-472 `ApplyOps` отвергает значение чужого вида в корне раньше законов, и ожидание станет `invalid_op`. Правка этой строки входит в T-472, в её DoD («`cmd/multiverse` … зелёные»).
3. **Допущение о порядке.** Между нормой пути (шаг 6) и встречей стоит `ApplyOps` (шаг 7, `apply.go` не менялся). Операция, которая не применяется к `hp` вовсе (например, `append hp` у персонажа во встрече), получит `invalid_op` раньше проверки встречи. Я читаю это как форму пакета («`invalid_op` остаётся за формой пакета», «Причина отказа по связке»), а не как «вид `hp`» п. 3. В издателях такого предложения нет. Если system-architect читает иначе, нужна одна строка матрицы.
4. `WORKDIR` distroless `:nonroot` и сборку образа без Docker не проверял, `make compose-lint` не запускал: он вызывает `docker compose config`. Итоговая проверка — `make image` и `make compose-lint` у владельца.
5. `-race` недоступен (нет cgo). Docker, `.env` и стенд `:8888` не трогал. В рабочей папке, кроме этой записи и строки ревью в карточке T-471, ничего не менял.

### Предложения в бэклог

1. **EPIC-003 (T-256 или T-447):** фейк и настоящий рой читают книгу из того же `MV_RULES_PATH` (`design.md` §4.3). Тексты об `rules/` в `fake_contexts.go` и `.env.example` исправляются заодно (Mi-2).
2. **EPIC-002:** `rules_version` в `/health` у `state`, сверка со `swarm` (`design.md` §4.3) — после подключения роя.
3. **system-architect:** строка `MV_RULES_PATH` в эталоне `infrastructure.md` §4.2, вместе с недостающей `MV_STATE_WORLDS` (открытый вопрос 2 карточки), и снятие пометок «Код расходится до T-471» в C-02 и КД §4.1, §4.6 после слияния.

## T-057 · ревью #1 · 2026-09-14 · code-reviewer#2 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-057`, ветка `task/T-057-state-objstore-snapshots`, база `3452816`. Коммитов нет, всё в рабочей копии. Код: `internal/state/{apply,context,dedup,worker}.go`, новые `store.go`, `intent.go`, `snapshot.go`, тесты `persist_test.go`, `resend_test.go`, `snapshot_test.go`, `store_test.go`, `objects_helpers_test.go`, `store_integration_test.go` (`//go:build integration`), правка `helpers_test.go`, комментарий `memstore.go`, общий `shared/contracts/registry_state.go`. Сверял с разделом `### T-057` в `tasks.md`, КД `state-and-mechanics.md` §4.3–§4.5, §4.8, §4.9, §9, §10, ADR-011, ADR-013 и C-14 в `contracts.md`. В ветке эпика `3452816` текста C-14 v1.3 нет, строку реестра сверял с DoD (приёмка T-056). Пять открытых вопросов разработчика (§9 и публикация после PUT, имя поля `/health`, §4.4, синхронный снапшот, мир без `latest.json`) замечаниями не ставлю: их решает system-architect#1. Дополнительные риски по ним — в разделе «Риски».

### Вердикт

**Принять.** Blocker (Critical): 0 · Major: 0 · Minor: 4 · Nit: 4.

Порядок «запись → публикация → память» выдержан на всех путях. Путей: создание, пакет, досылка. Повторы записи и остановка `persist_failed` работают, интент живёт от записи до удаления, как в ADR-013 п. 4. Снапшот пишет объект, затем указатель, затем ротацию и `snapshot.created` с миром в конверте. Байты снапшота детерминированы. Замечания не блокируют. Главные из них: снапшот при `Stop` не ограничен сроком `Stop` (Mi-1); неатомарный пакет без интента теряет согласованность при обрыве между PUT (Mi-2).

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Порядок записи и публикации (`apply.go:310-321`, `:401-417`; `dedup.go:74-111`) | Создание и пакет: `persist` → `publish` → `SetFactEventID` → `store.Put` → окно. Сущность уходит в объект с пустым `fact_event_id` (`entity.Commit` его сбрасывает), это соответствует КД §4.5 п. 12. Отказы пакета `atomic=false` публикуются после записи применённых наборов. Пакет из одних отказов ничего не пишет |
| 2 | Повторы и `persist_failed` (`intent.go:39-105`) | Четыре попытки с паузами 100/300/900 мс на `Config.Timers`. Запись идёт под `context.WithoutCancel`, отмена обрывает только паузу. После неудачи — `halt`, лог `Error handled=false`, факта нет, рабочий набор не тронут. `Stopped()` запрещает и следующие предложения, и снапшот. Тесты `TestAWriteThatFailsIsTriedAgain` и `TestAWriteRefusedForGoodStopsTheWorld` это закрепляют. Повторы наслаиваются на повторы клиента MinIO (N-3) |
| 3 | Жизненный цикл интента | Интент пишется при `atomic=true` и > 1 сущности. Затем PUT сущностей по `id`, затем `DeleteIntent`. Неудачное удаление даёт `Error handled=true`, ответ уходит (решение 6 карточки; roll-forward пропускает сущности на `to_version`). Ключ экранируется `url.PathEscape`. `ListEntities` пропускает `_intents/`. Тест читает интент во время записи последней сущности и проверяет `from_version` и `to_version` |
| 4 | `Stop` посреди записи (`TestStopDuringAWriteFinishesTheWriteAndItsFact`) | PUT удержан хуком. `Stop` не возвращается, факта нет, после отпускания — запись, факт, затем снапшот `shutdown`. Это рестарт **в процессе**: тот же `Context` и та же память. Повтор того же события гасит окно `delivered` у `dispatch`, до `Applier` он не доходит. Новое событие с тем же `proposal_id` гасит окно `proposal_id`. Для сценария C-01 v1.7 этого достаточно |
| 5 | Падение между шагами | **После PUT, до публикации** (публикация брошена при `Stop`): объект впереди памяти, мир `publish_failed`. Рестарт над объектами досылает факт. **Между объектом и указателем снапшота**: указатель остаётся на прежнем снапшоте (unit и integration на MinIO); `nextSeq` берёт номер от объектов. **Между PUT сущностей атомарного пакета**: интент остаётся, догон — T-059 (риск 2). **Между PUT неатомарного пакета** — Mi-2 |
| 6 | Моделируют ли тесты досылки рестарт | Да, насколько это возможно без T-059. `TestAFactLostAfterItsWriteIsSentAfterARestart` поднимает **новый** `Context` через `NewOver`: новый `Applier`, пустые окна `proposal_id` и `delivered`. Шина та же, неподтверждённое предложение доставляется снова. Рабочий набор собран из объектов помощником `loadedFrom` вместо ветки восстановления «объекты без снапшота» §4.8. Тесты `TestAFactSentAgain…` и `TestACreatedFactLost…` работают на уровне `Applier` над тем же набором из объектов. Производственного чтения объектов в T-057 нет, поэтому тесты надо повторить над настоящим восстановлением T-059 (бэклог 1) |
| 7 | Досылка: гонки и дубликаты | Досылка идёт на горутине мира, гонок нет. Второй досылки нет: после публикации `fact_event_id` дописан в память, предложение в окне (`TestACreatedFactLost…`, второй повтор). Версия не растёт, записи нет, id и `applied_at` берутся из `last_change` (`originalCause`). Факты пакета, которые **успели** уйти до падения, после рестарта уходят снова под тем же id: объекты всегда без `fact_event_id`. Тест это закрепляет, §9 разрешает («тот же `event.id` — потребители дедуплицируют»). Если событие новое, у факта другие байты — Mi-3 |
| 8 | Снапшот: окно, ротация, указатель | Окно — последние 1000 от старых к новым (`newest`), тест на 1005 предложений. Ротация оставляет пять объектов по `seq` и идёт только после записанного указателя, так что объект, на который указывает `latest.json`, она не удаляет. `size_bytes` есть только в указателе и совпадает с длиной объекта. Ключ выводится из `taken_at` и `seq`. Выживший мутант номера — Mi-4 |
| 9 | `snapshot.created` | Мир в конверте: `NewRoot(…, worldID, …)` или `Derive` от предложения мира. Поля — восемь полей закрытой схемы, без `rules_version`, `entities_count`, `reason`. По счёту событие строится через `WithCauseID("state", seq)` и стоит сразу за фактами предложения. Конверт разный у двух путей — N-1 |
| 10 | Детерминизм байтов | Объект сериализуется `json.MarshalIndent` структуры. Ключи `map` сортирует `encoding/json`, сущности идут по (type, id), окно — в порядке LRU. `taken_at` берётся из `Config.Clock` (`Deps.Clock`), `written_at` пишется только в указатель. Тест двух прогонов сравнивает байты. `time.Now` в пакете нет, lint 0 |
| 11 | Курсор | `Apply` двигает курсор на офсет + 1 только после ответа без ошибки. К воркеру попадают только предложения его мира, поэтому курсор отстаёт от конца топика в безопасную сторону. Новый `Applier` начинает с 0 (риск 3) |
| 12 | `worker.do` и `Context.Snapshot` | Канал `jobs` небуферизован. `do` выбирает между `jobs`, `done` и `ctx.Done()`: взаимоблокировки при остановке нет. Снапшот `shutdown` пишется после `<-done` всех воркеров, поэтому с воркером он не пересекается. `Health` читает `SnapshotHealth` под `mu` |
| 13 | `registry_state.go`, `WorldRequired` | Политика действует у издателя. `snapshot.created` в дереве публикует только State; `shared/testkit/state` его не публикует (`shared/testkit/state/snapshot.go:100`). `internal/swarm` в ветке нет. `internal/gateway/...` — тесты зелёные. `go test ./shared/contracts/... ./shared/testkit/... ./test/... ./cmd/...` — ok. `TestSnapshotCreatedRequiresAWorld`: без мира — `ErrPolicyViolation` |
| 14 | Пересечение с T-471 (только чтение `.worktrees/T-471`) | Общих файлов нет. T-471 правит `ownership.go`, `matrix_test.go`, `plan_test.go`, `cmd/multiverse/contexts_state.go`; T-057 — `apply.go`, `context.go`, `dedup.go`, `worker.go`, `helpers_test.go`. База T-471 — `7fcd02b`, `git diff 3452816 7fcd02b` по `internal/state`, `registry_state.go` и `contexts_state.go` пуст, текстового конфликта не жду. Смысловая связь одна: T-471 заменяет `newStateContext` на `newStateWithTheLaws` → `state.New(state.Config{Invariants: …})`. Подключать `Objects` и `RulesVersion` (поле `rules_version` загруженной книги) придётся в этой функции. `plan_test.go` T-471 строит `NewApplier` без `Objects` и `Clock`, правка `fixtureWith` его не задевает |
| 15 | Тесты | `time.Sleep` нет, ожидание — `waitFor` и `advancing(manual, …)` на ручных часах. Integration помечен `//go:build integration`. Тестовое хранилище отказывает отменённому контексту, как клиент сервера. Это ловит мутант «запись видит отмену» |
| 16 | Мутанты (копия `scratchpad/t057r-mut` без `Docs/` и `services/`, без `-overlay`, контрольный первым) | **C0** (копия без изменений, 21 тест снапшотов, досылки и `Stop`) — зелёный. **M-R1** (`nextSeq` = число объектов вместо наибольшего `seq` + 1) — **выжил**, весь `internal/state` зелёный (Mi-4). **Зонд P1** (временный тест): досылка под новым событием с тем же `proposal_id` и другим `cause` дала факт с тем же id и `cause` `loot` вместо `combat` (Mi-3). Копия удалена по точному пути |
| 17 | Прогоны в рабочей папке (go1.26, windows/amd64) | `go build ./... && go vet ./... && go vet -tags integration ./internal/state/...` — 0; `gofmt -l` — пусто. `go test -short -count=1 ./internal/state/... ./shared/contracts/... ./shared/testkit/...` — 11 пакетов ok. `./internal/gateway/... ./test/... ./cmd/...` — ok. `golangci-lint run ./...` — 0 issues. `-race` недоступен: нет cgo (gcc). Интеграционный прогон не делал |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-1 | Minor | `internal/state/snapshot.go:134`; `context.go:361` | Снапшот `shutdown` вызывается из `Stop` с его контекстом, но `snapshot` сразу снимает отмену (`context.WithoutCancel`). Срок `Stop` на запись не действует. У клиента MinIO нет общего срока запроса: `ResponseHeaderTimeout` 1 мин, три попытки на операцию, а операций пять и больше (List, два PUT, List, удаления). Зависший сервер держит `Stop` минутами при бюджете `runtime.StopTimeout` = 15 с на все контексты и при КД §4.9 «≤ 10 с». Шина не закрывается, процесс не выходит до SIGKILL. Причины снимать отмену здесь нет: оборванный снапшот оставляет указатель на прежнем, так и задумано | Снимать отмену только у записи ответа (`persist`), а у снапшота `shutdown` и `admin` передавать контекст вызывающего (или срок из него) в `PutSnapshot`, `rotate` и `ListSnapshots`. Ошибку — в ошибку `Stop` и в `snapshot_stale`. Тест: хук держит `PUT state/…`, `Stop` с коротким контекстом возвращается в срок с ошибкой, `latest.json` — прежний |
| Mi-2 | Minor | `internal/state/intent.go:44`; `invariants.go:128-154`; `dedup.go:34-44` | Неатомарный пакет из нескольких сущностей пишется без интента. Обрыв между PUT — kill процесса или отказ хранилища на второй сущности — ведёт к трём последствиям. (1) Законы шага 8 проверены на всём наборе `kept` сразу (`checkLaws`), а на диске и после рестарта остаётся его часть, которую никто не проверял. Межсущностный закон (inv-04, inv-06) может оказаться нарушенным без записи об этом. (2) У записанной сущности `last_change.batch_size` равен полному размеру пакета, хотя применена часть. (3) Недописанные наборы не получают ответа: ни `entity.updated`, ни `rejected`. Дедупликация признаёт пакет применённым по одной сущности (решение T-056), и `resendUnpublished` объявляет только записанную часть. Код следует КД §4.5 п. 11 и ADR-013 п. 4, но строка DoD в `tasks.md` требует интент «только для пакетов > 1 сущности», без условия `atomic`. Разработчик выбрал КД и отметил риск | Вопрос к system-architect (ниже, п. 1). Вариант А: интент для любого пакета > 1 сущности, как в DoD. Цена — один PUT и одно удаление на неатомарный пакет. Roll-forward T-059 допишет наборы, проверенные законами вместе. Вариант Б: принять частичную запись явно — строка в КД §9 и `batch_size` по записанному |
| Mi-3 | Minor | `internal/state/dedup.go:94`; `facts.go:70` | Досылка под **новым** событием с тем же `proposal_id` берёт из `last_change` только id причины и `applied_at`. Поле `cause` payload приходит из нового предложения (`proposal.Cause`), конверт тоже: `correlation_id`, `scope`, `actor_kind`, `locale`, `replay` — через `Derive`. Зонд P1: тот же id факта, `cause` `loot` вместо исходного `combat`. Если первый факт успел уйти (падение после публикации, до памяти), на шине окажутся два разных события под одним id. Потребитель оставит то, что пришло первым, а трасса разойдётся: `causation_id` указывает на событие одной цепочки, `correlation_id` — на другую | `cause` брать из `e.LastChange.Cause`. Конверт восстановить нельзя: корреляции в `last_change` нет. Предел описать в КД §4.5 п. 2 (или спросить system-architect, не досылать ли только при `p.Event.ID == last_change.proposal_event_id`). Тест: повтор с другим `cause` — в факте исходный |
| Mi-4 | Minor | `internal/state/snapshot_test.go:109-140`; `snapshot.go:196-205` | Мутант M-R1 (`nextSeq` = `len(refs)`) выжил. Тест ротации пишет ровно шесть снапшотов, а на шестом число объектов (5) совпадает с `seq` + 1. С седьмого номера повторяются: два объекта `state:{world}:000005` с разными ключами, повтор id снапшота, неоднозначный выбор «предыдущего по seq» при повреждении (§4.8) | В `TestTheRotationKeepsFiveSnapshots` писать семь снапшотов и проверить `seq` 6…2, уникальность `snapshot.id` и то, что седьмой получил `seq` 6 |
| N-1 | Nit | `internal/state/snapshot.go:240-246` | У снапшота по счёту `snapshot.created` берёт через `Derive` `actor_kind`, `scope` и `locale` предложения (например `human` игрока и scope встречи). Снапшот по `admin`/`shutdown` — `actor_kind=system` без scope. Один тип события получает разный конверт в зависимости от триггера | Для снапшота по счёту ставить `actor_kind=system` и не переносить scope (опция `Derive` или правка после), либо записать различие в C-14 |
| N-2 | Nit | `internal/state/resend_test.go:235` | `_ = update(t, "prop-other", …)` — результат выброшен, строка ничего не проверяет. Если она сдвигает генератор id, это нигде не сказано | Удалить строку или объяснить в комментарии, зачем она |
| N-3 | Nit | `internal/state/intent.go:76-93`; `shared/objstore/minio.go:51-54`, `:310-327` | Повтор ×4 в State наложен на повтор ×3 клиента MinIO. До `persist_failed` — до 12 запросов и около 3,7 с пауз, против КД §4.5 п. 11 «×3 (100/300/900)». Отказ без повторной ценности (`ErrNoBucket`: мир без `mvctl world init`) State тоже повторяет четыре раза | В КД или комментарии `persistPauses` назвать фактическое число запросов. `objstore.ErrNoBucket` и `ErrNotFound` не повторять |
| N-4 | Nit | `internal/state/apply.go:230`; `snapshot.go:145-153` | Если снапшот по счёту записан, а `snapshot.created` не ушёл, событие теряется без повтора. `/health` этого не показывает: `setSnapshotWritten` уже сбросил ошибку, а `Apply` результат выбрасывает. Остаётся только лог | Отражать неудачу публикации в секции мира (`snapshot_event_failed`) или повторять событие при следующем триггере |

### Открытые вопросы (к system-architect, через оркестратора)

1. **Неатомарный пакет без интента (Mi-2).** Принять частичную запись (строка КД §9, `batch_size` по записанному) или писать интент для любого пакета > 1 сущности, как в строке DoD T-057? Сейчас КД §4.5 п. 11 и DoD расходятся.
2. **Досылка атомарного пакета до roll-forward.** DoD велит согласовать способ с решением T-059. Сценарий: процесс упал между PUT атомарного пакета, интент остался, и предложение доставлено снова раньше roll-forward. Тогда `resendUnpublished` объявит половину атомарного пакета и запомнит `proposal_id`. Подтвердить, что T-059 делает roll-forward интентов до `Tail`, или требовать, чтобы досылка не шла при интенте этого `proposal_id` (`state_divergence`).
3. **Конверт досылки под новым событием (Mi-3).** Досылать только при том же событии или принять разный конверт под одним id?

### Риски и допущения

- **К вопросу 4 разработчика (синхронный снапшот).** Снапшот по счёту идёт на горутине мира под `WithoutCancel` и без срока запроса. Зависшее хранилище останавливает мир на минуты. `/health` при этом молчит: `Retrying` считает только публикации, `snapshot_stale` появляется только после ошибки.
- **К вопросу 1 разработчика (публикация после PUT).** Пока брокер недоступен, мир стоит на одном предложении, а объекты уже впереди памяти. Досылка есть только через рестарт. Сценарий «брокер вернулся без рестарта» решён повтором того же события, и это верно.
- **Подключение `Objects` в `cmd/multiverse` до восстановления T-059 опасно.** Память нового процесса пуста. `entity.create.proposed` для сущности, которая есть на диске, перезапишет объект версией 1. Снапшот (если в памяти есть сущность мира) переведёт `latest.json` на неполный мир. Подключать хранилище и восстановление надо одним изменением.
- **Курсор нового `Applier` равен 0.** Снапшот `shutdown` мира без предложений в этом запуске запишет `cursor.system_events: 0`. Это безопасно, догон просто пойдёт с начала журнала, но T-059 должна переносить курсор из восстановленного снапшота в `Applier`.
- В ветке эпика `3452816` нет текста C-14 v1.3. Строку реестра сверял с DoD из приёмки T-056. Кончик эпика ушёл на `0e1aa7e`: `internal/state` и `registry_state.go` там не менялись (сверено для базы T-471 `7fcd02b`).
- Мутант и зонд гонялись в копии рабочей папки без `Docs/` и `services/`. Контрольный C0 зелёный — копия исполнялась. Копия `scratchpad/t057r-mut` с тестом зонда удалена по точному пути. В рабочей папке, кроме этого раздела, ничего не менял. Карточку T-057 не трогал. Docker, `.env` и стенд `:8888` не трогал.

### Предложения в бэклог

1. **EPIC-002 / T-059:** перевести `TestAFactLostAfterItsWriteIsSentAfterARestart`, `TestACreatedFactLostAfterItsWriteIsSent` и `TestStopDuringAWrite…` с `loadedFrom` на настоящее восстановление (`ReadLatest`/`ListEntities` → `Applier`). Добавить случай «интент + часть сущностей записана + повтор предложения».
2. **EPIC-002 / T-059:** секция `/health` мира — возраст снапшота, `pending_intents`, признак «снапшот пишется дольше N с» (к риску 1).
3. **EPIC-001 / `shared/objstore`:** срок одного запроса у клиента MinIO (например `http.Client.Timeout` или срок контекста внутри `retry`). Иначе любой вызов без отмены может висеть минутами.

## T-472 · ревью #1 · 2026-09-14 · code-reviewer#3 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-472`, ветка `task/T-472-path-grammar`, база `0345177`. Коммитов нет, всё в рабочей копии. Код: `shared/entity/{path,ops,attrs}.go`, `README.md`, новый `grammar_test.go`, правки `ops_test.go` и `changed_test.go`. В `internal/state` правлены `proposal.go`, `apply.go`, `ownership.go` (только комментарии), тесты `helpers_test.go`, `matrix_test.go`, `plan_test.go`, удалён `grammar_test.go`. Тесты потребителей: `shared/testkit/state/apply_test.go`, `cmd/multiverse/contexts_state_test.go`, `internal/gateway/readmodel/apply_test.go`. Сверял с разделом `### T-472` в `tasks.md`, C-02 v1.8a п. 2 и п. 3 в `contracts.md`, `data-model.md` §3, ответом 2 просмотра system-architect в карточке T-056 и п. 1 бэклога в карточке T-471. Вопросы разработчика 1–4 замечаниями не ставлю (их решает system-architect#1), оценка риска — ниже. Карточку T-472 не трогал.

### Вердикт

**Принять.** Blocker (Critical): 0 · Major: 0 · Minor: 3 · Nit: 3.

Грамматика перенесена без потерь: регулярное выражение и проверка `strconv.Atoi` совпадают с удалённым `canonicalPath`, а 24 написания (23 и пустой путь) — тот же набор. Таблица видов совпадает с §3.1–§3.4, §3.6, §3.7 в обе стороны. Отказ одинаков у всех читателей: у State на шаге 1 и шаге 7, у read-model шлюза и у тестового догона. Вектор У-1 закрыт точечным тестом. Замечания касаются силы тестов, а не поведения.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | `CanonicalPath` (`path.go:46`) и `CheckOp` (`ops.go:271`) | Сегмент `^([^.\[\]]+)(?:\[(?:0\|[1-9][0-9]{0,8})\])*$`, ключ не принимает `Atoi`. Это дословно C-02 v1.8 п. 2 и то же выражение, что у удалённого `canonicalPath`: изменились только незахватывающие группы. `CheckOp` проверяет пустой путь, запись, зарезервированный корень, путь ниже скаляра типа (`hp[0]` тоже) и глагол. `ApplyOps` зовёт `CheckOp(e.Type, op)` до каждой операции, шаг 1 State — `CheckOp(set.Ref().Type, op)`. Несовпадение типа набора и мира отвергает шаг 3 (`apply.go:459`), тоже `invalid_op`. `splitPath` больше не срезает ведущие точки, но другого вызывающего, кроме `applyOp`, у него нет |
| 2 | `remove inventory[0]` и `StateHash` догона | `TestTheRemovalOfAnElementIsSpelledWithBrackets`: `inventory.0` → `ReasonBadPath`; `inventory[0]` → один элемент `changed[]` на пути списка с `old` и `new`, хэш догона равен живому. Свойство `TestCatchingUpOnChangedReproducesTheState` зелёное, но вектор У-1 в нём недостижим (Mi-2). `TestCatchingUpRefusesAPathNotInItsCanonicalForm` — шесть случаев, в том числе элемент без `new` |
| 3 | Таблица `attrs.go:511` против `data-model.md` §3 | Сверил построчно. World — 8 строк и `name`, Region — 9 и `name`, Character — 14 и `name` (`id` и `created_by_link` не атрибуты), NPC — 14 и `name`, Group — 6 и `name`, Encounter — 10 и `name`. Вид, обязательность и `null` совпадают. `flee` и `leader_id` — обязательные с `null`. У `opened_by_event_id` и `closed_by_event_id` графа «Обяз.» пуста — в коде они необязательные, это разумное чтение. Поля Item §3.5 и `died_at`/`killed_by` у `player` не типизированы, как требует п. 2. `scope` и контейнеры — `KindOpen`, путь ниже них разрешён. Обратное направление в тесте проверено только по списку известных имён (Mi-1) |
| 4 | Проверка вида после операций (`ops.go:250`) | Проверяются только корни, затронутые операциями, в порядке первого касания: ответ детерминирован. Отсутствующий атрибут не отвергается (`remove hp` проходит). Дыр не нашёл. `set status.x` и `inc hp.x` отвергает `CheckOp`. `append` в отсутствующий скаляр создаёт список, и его ловит проверка вида. `append` в число не применяет `applyOp`. `inc` отсутствующего `died_at` даёт число и ловится проверкой вида. `null` у обязательного без пометки — `ReasonWrongKind`. `time.Time` и `int` из Go сравниваются в проводной форме: `time.Parse(RFC3339)` принимает дробные секунды `RFC3339Nano`. `±Inf` недостижим: раньше срабатывает `ReasonNotJSON` |
| 5 | `CheckAttributes` при создании (`apply.go:295`) | Стоит после `JSONCompatible` и до `alreadyApplied`, отказ — `invalid_op`, в лог пишется причина. Издатели на кончиках эпиков: `characters.Attributes` (`internal/gateway/characters/pending.go:43`, EPIC-003 `78de2d9` и EPIC-004 `ec699de`) — полный набор §3.3. `Harness.CreatePlayer` берёт атрибуты фикстуры. `FakeEncounter` (`fake_encounter.go:765-783`) передаёт `region_id`, `scope`, `participants`, `npcs`, `state`, `round_seq`. `/forget` — это обновление, а не создание. `mvctl laws bump` пишет `laws_version` текстом. Остальные `"attributes"` в тестах эпиков — факты `entity.created`, `CheckAttributes` к ним не применяется. Фикстуры `testdata/fixtures/*.json` проходят (`TestCheckAttributesOfACreate`) |
| 6 | Read-model шлюза | `readmodel/apply.go:233-252` применяет `changed[]` по элементу через `ApplyOps` с типом из факта. Новый `ErrCorruptFact` возможен в трёх случаях: неканонический путь с `new`, путь ниже скаляра, чужой вид в затронутом корне. State после T-472 такого не публикует: шаги 1 и 7 используют ту же функцию с тем же типом. Элемент скаляра в `changed[]` всегда корневой и несёт итоговое значение. Неканонический путь без `new` `opFor` по-прежнему молча пропускает, если пути нет (зона EPIC-004, бэклог 1). Правка шага `set position.region` → `banner` сохраняет смысл: текст заменяется объектом |
| 7 | Кончик EPIC-004 после T-308 и T-309 | Кончик ушёл с `9651c19` (оценка разработчика) на `ec699de`: слиты T-308 и `gatewaytest`. Дифф T-472 без `Docs` накладывается на `ec699de` чисто. В копии `scratchpad/t472r-e004` зелёные `go build ./...`, `go vet` и `go test -short` для `shared/entity`, `internal/state`, `internal/gateway/...`, `internal/mechanics`, `shared/testkit/...`; копия удалена. T-309 (`.worktrees/T-309`, только чтение) `readmodel/apply.go` и `apply_test.go` не трогает. Её `snapshot_test.go:271-278` применяет `set encounter_id`, `inc hp`, `remove encounter_id` к фикстуре — с T-472 совместимо |
| 8 | Удалённые `canonicalPath`, `scalarAttributes`, `underScalar`, `reservedRoot`, `knownOp` | Грамматика, зарезервированные корни и глаголы перешли в `CheckOp` один в один. Все 38 имён `scalarAttributes` есть в таблице у своих типов. Потеряны только межтиповые совпадения: `died_at.x` у `player`, `flee.x` у `npc`, `state.x` у `player`, любой скаляр у типа вне таблицы. Это следствие решения 1 (вопрос 1), а не пропуск. 23 написания и пустой путь сверены по спискам старого и нового теста: наборы совпадают |
| 9 | Мутанты (копия `scratchpad/t472r-mut` без `.git`, `Docs`, `services`; без `-overlay`) | Базовый прогон копии зелёный. **C0** (контрольный: `CanonicalPath` принимает любой непустой путь) — красный: `TestTheCanonicalSpellingOfAPath`, `TestTheRemovalOfAnElementIsSpelledWithBrackets`, `TestCatchingUpRefusesAPathNotInItsCanonicalForm`, `TestTheMatrixOfRefusals`. Свойство `TestCatchingUpOnChangedReproducesTheState` при этом **зелёное** (Mi-2). **K4** (шаг 1 зовёт `CheckOp("", op)`) — красный: из `internal/state`, `shared/testkit/state`, `cmd/multiverse` падает только строка `gateway: weather.x of the world, the form before the rights`. Вывод разработчика подтверждён: без этой строки K4 выживает. Отказ всему неатомарному предложению при этом не закреплён (Mi-3). Каждый мутант откатывался копированием файла с проверкой `cmp`, копия удалена по точному пути |
| 10 | Пересечение с T-058 и T-059 (только чтение) | **T-059** правит `apply.go`. `git merge-file` трёх версий (база `0345177`, T-472, T-059) — 0 конфликтов. `proposal.go`, `helpers_test.go`, `grammar_test.go` и `contexts_state_test.go` T-059 не меняет. Её новые тесты создают сущности через `create(...)`, после слияния они получат `withRequired`. Смысловой риск: `CatchUpChanged` (`.worktrees/T-059/internal/state/catchup.go:40`) канонический путь не проверяет, риск 1. **T-058** общих файлов не имеет. `Bootstrap` публикует фикстуры, а они проходят `CheckAttributes` |
| 11 | Прогоны в рабочей папке (go1.26.8, windows/amd64) | `go build ./... && go vet ./...` — 0. `go test -short -count=1 ./shared/entity/... ./internal/state/... ./internal/gateway/... ./shared/testkit/... ./cmd/multiverse/...` — 26 пакетов ok. `golangci-lint run ./...` — 0 issues. Интеграционные прогоны и e2e не запускал. Docker, `.env` и `:8888` не трогал |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-1 | Minor | `shared/entity/grammar_test.go:194-227` | `TestTheTableHoldsNothingTheModelDoesNot` проверяет обратное направление, пробуя только 48 известных имён. Лишняя строка таблицы с другим именем проходит незамеченной: `banner` у `player`, `item_id`, опечатка рядом с верной строкой. Комментарий «so a row the code adds without the model is found too» обещает больше, а DoD требует полноты «в обе стороны» | Сравнивать множества ключей точно. Например, `export_test.go` в пакете `entity` отдаёт `AttributeNames(type)` (ключи `attributeTable`), и тест сверяет их с ключами `model` из `TestEveryAttributeOfTheDataModelHasItsKind`. Список `names` и счётчики тогда не нужны |
| Mi-2 | Minor | `shared/entity/changed_test.go:417-451` | Добавка к свойству не ловит У-1. (1) Страж `!entity.CanonicalPath(op.Path)` проверяет код той же функцией и слеп к её поломке. (2) В `base` у `tags` и `inventory` по одному элементу, поэтому `remove inventory.0` даёт тот же хэш при любом правиле догона: расхождение У-1 требует двух элементов и больше. Контрольный мутант C0 оставил свойство зелёным, его поймали только именованные векторы. Карточка («DoD → тесты», строка У-1) приписывает свойству больше | В `base` дать `inventory` и `tags` по два элемента. Страж сверять с независимым списком написаний, которые должны отвергаться (`inventory.0`, `tags.1`, `status.x`, `dmg.formula`), а не с `CanonicalPath`. Проверка: мутант C0 краснит свойство |
| Mi-3 | Minor | `internal/state/proposal.go:189`; `internal/state/matrix_test.go:466-471` | Шаг 1 теперь зависит от типа набора, но правило «путь ниже скаляра — `invalid_op` на всё предложение» (C-02 v1.8 п. 2) при `atomic=false` не закреплено. С мутантом K4 такое предложение из двух наборов (`status.x` у персонажа и законный второй) отвергло бы на шаге 7 только первый набор, а второй применило бы. Все тесты, кроме новой строки о порядке с правами, при этом зелёные | Добавить строку матрицы или тест `apply_test.go`: `atomic=false`, набор 1 `set status.x`, набор 2 законный. Ожидается один `entity.update.rejected invalid_op`, `entity.updated` нет, версия второй сущности не выросла. Под K4 строка должна краснеть |
| N-1 | Nit | `shared/entity/grammar_test.go:392-408` | `TestCheckAttributesReportsTheFirstNameInOrder` описан как выбор «of two problems» (`hp` и `status`). Но у NPC без остальных атрибутов проблем больше, и отчёт `atk` — третья из них, отсутствие обязательного. Детерминизм тест проверяет, но не тот случай, что описан | Взять полный набор NPC с `hp: "10"` и `status: nil` и ждать `hp`, или поправить комментарий |
| N-2 | Nit | `internal/gateway/readmodel/apply_test.go:69-70` | «position was that text until T-472» читается так, будто `position` перестал быть текстом. Он всегда был текстом: изменилось только то, что путь ниже него теперь отвергается | «position is a text of data-model.md §3.3, and since T-472 no path goes below it; banner is untyped» |
| N-3 | Nit | `internal/state/apply.go:297` | Ключ лога `"error"`. В том же файле, в `answer` (`apply.go:269`) и в повторах записи (`:556`), ключ `"err"` | Взять один ключ, `"err"`, как рядом |

### Оценка риска по вопросам разработчика (не замечания)

1. **Таблица по типам; тип из `Entity.Type`, на шаге 1 — из `changes[].entity.type`.** Риск низкий. Все читатели получают один ответ, потому что тип у них один: мир у State, факт у шлюза. Потеря межтиповых отказов (`died_at.x` у `player`) безвредна: владение не даёт эти пути ни `task`, ни `gateway`, а атрибут никто не читает. Если system-architect выберет список имён, правка ограничится `AttributeSpecOf`/`CheckOp` и тестом полноты.
2. **`KindModifier` и `KindOpen`.** Риск низкий. `KindModifier` — это строка «int / формула» из §3.3, `Flee` и `AttrInt` её так и читают. `KindOpen` — внутренняя пометка обязательного контейнера, значение не проверяется, как требует п. 2. Проводная форма и схемы не меняются.
3. **Ключ из ≥ 20 цифр.** Сейчас риск низкий: `CanonicalPath` и `splitPath` согласованы, ключ ищется по объекту, второго написания элемента списка нет (индекс — до 9 цифр). Но у правила «принимает `strconv.Atoi`» есть скрытое следствие: оно зависит от размера `int`. На 32-битной платформе ключи из 10–19 цифр `Atoi` не принимает, и они канонические, а на 64-битной отвергаются. Грамматика формы не должна зависеть от платформы. Предлагаю system-architect формулировку «ключ не из одних цифр с необязательным знаком» (`^[+-]?[0-9]+$`): она закрывает и ≥ 20 цифр (бэклог 3).
4. **Членство в перечислении и разрешение ссылок.** Риск низкий. Статус проверяет матрица переходов (`StatusTransitionAllowed`), позицию — inv-10. Опечатка в `weather` или `state` встречи пройдёт форму, но словарь `weather` принадлежит блупринту, а не `shared/entity`.

### Риски и допущения

1. **Догон State в T-059 не проверяет каноническую запись.** `CatchUpChanged` в `.worktrees/T-059/internal/state/catchup.go` разбирает путь своим `pathTokens`. Он принимает `inventory[01]` как индекс 1, а `tags.0` — как ключ `0`: на списке это заменит список объектом. После слияния с T-472 правило C-02 v1.8 п. 2 («путь не в канонической записи — повреждённый факт») в production-догоне State не выполняется. T-059 нужно звать `entity.CanonicalPath` до разбора и перенести вектор `TestCatchingUpRefusesAPathNotInItsCanonicalForm`. Текстовых конфликтов нет, но это смысловая зависимость, и её надо назвать тому, кто сливает вторым.
2. **Старые журналы.** До T-472 State применял `set hp "10"` вне отдыха и подобные значения чужого вида. Догон read-model по такому журналу после обновления получит `ErrCorruptFact`. Выпуска ещё не было, поэтому риск касается только миров разработчиков.
3. **`flee` при создании.** В §3.3 графа «Обяз.» — «да», а в ограничениях — «`null` или отсутствие — «не убегает»». `CheckAttributes` отвергает создание персонажа без `flee`. Издатели `flee` передают, поэтому поведение не меняется, но текст модели двусмыслен. Вопрос к system-architect через оркестратора.
4. **`mvctl world init` (T-058) с пользовательскими фикстурами.** Фикстура без обязательного атрибута теперь получит `invalid_op`. Стоит, чтобы T-058 показывала оператору причину (атрибут и тип), а не голый отказ.
5. Кончик EPIC-003 (`78de2d9`) целиком с патчем не прогонял. В `internal/{laws,llm,swarm}` вызовов `ApplyOps` и создания сущностей нет (`git grep`). Общие с EPIC-004 пакеты проверены на `ec699de`.

### Предложения в бэклог

1. **EPIC-004:** в `readmodel.opFor` проверять `entity.CanonicalPath` до `HasAttr`: неканонический путь без `new` сейчас молча пропускается (согласен с п. 1–2 разработчика).
2. **EPIC-002 / T-059:** `CatchUpChanged` — отказ неканоническому пути (риск 1). Затем одна production-функция догона в `shared/entity` вместо трёх реализаций (бэклог T-448).
3. **system-architect, C-02 п. 2:** правило ключа без зависимости от размера `int` (оценка вопроса 3).

## T-058 · ревью #1 · 2026-09-14 · code-reviewer#1 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-058`, ветка `task/T-058-world-init`, база `0345177`. Коммитов нет, всё в рабочей копии. Код:
- новые `internal/state/bootstrap.go`, `bootstrap_test.go`;
- новые `cmd/mvctl/internal/world/{world,init,status}.go`, `world_test.go`;
- правки `cmd/mvctl/commands_state.go`, `cmd/mvctl/main_test.go` (файл EPIC-001), `.golangci.yml` (правило `cmd-mvctl-world`).

Сверял с разделом `### T-058` в `tasks.md` (три строки просмотра system-architect T-057), КД `state-and-mechanics.md` v0.4 (§4.4, §4.8–§4.10, §18), C-02 v1.8a («Издатель предложений bootstrap»), `ownership.md` v0.7 §3 п. 4 и п. 7. Вопросы 1–4 разработчика замечаниями не ставлю, по ним — только оценка риска (ниже).

Пока шло ревью, system-architect#1 дописал в рабочую копию «Правку T-058» КД §4.10 (не закоммичена). В ней четыре правила: `--bus memory` только с `--store memory`, коды выхода, отказ от снапшота `shutdown` через закрытое хранилище, `--force` с очисткой объектов State. Замечания ниже я нашёл независимо. Где они совпадают с правкой, это отмечено.

### Вердикт

**Вернуть.** Blocker (Critical): 0 · Major: 1 · Minor: 4 · Nit: 2.

`Bootstrap` сделан верно:
- порядок world → region → npc → players;
- `proposal_id`, `source=mvctl`, `actor_kind=system`, `cause=init` по C-02 v1.5;
- `Tail` от `End`, взятого до чтения `[0, End)`: окна гонки нет;
- подписка и таймер освобождаются на всех путях.

`world init` над `--store memory` работает по DoD. Вернуть из-за Ma-1: команда пишет в MinIO снапшот, курсор которого — офсет журнала `membus`, а этот журнал исчезает вместе с командой.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Порядок и форма предложений (`bootstrap.go:40-45`, `:167-183`) | `FixtureFiles` задаёт порядок, `LoadFixtures` проверяет тип файла, мир, повторы id, `DisallowUnknownFields` и один документ в файле. `BootstrapProposal`: `NewRoot(…, SourceMvctl, worldID, nil, ActorSystem, …)`, `meta.agent` нет, `proposal_id = bootstrap:{world}:{type}/{id}`. Тест прогоняет `contracts.Validate` и `ParseProposal` (`author`, `init`). Сверено с C-02 v1.5/v1.8a |
| 2 | Ожидание и утечки (`bootstrap.go:212-236`, `:332-353`) | `End` берётся до `ReadRange [0, End)`, `Tail` идёт от того же `End`, слоты заведены до запуска `Tail`. Ответ, вышедший между `End` и чтением, `Tail` не теряет. `take` не блокирует (`select default`, буфер 1). Горутина `Tail` отменяется и дожидается в `defer` на любом возврате. Таймер — `clock.Timers`, `Stop` в `defer`. `tail.err` читается после `close(done)`, гонки нет |
| 3 | Идемпотентность и гонка «State ещё не ответил» | Прошлый прогон мог опубликовать P и не дождаться ответа. Тогда повтор не найдёт `entity.created` в `[0, End)` и опубликует P снова. Ответ на исходное P выйдет после `End` и попадёт в слот; дубль State погасит окном. Гонки нет. Отказ в окно дедупа не входит (`dedup.go:24-27`), так что повтор после отказа получает ответ. Но чтение от `End` тестом не закреплено: мутант MR-A выжил (Mi-2) |
| 4 | `world init`: бакеты, `exit 2`, снапшот, курсор (`init.go:106-148`, `:212-224`) | `EnsureWorldBuckets` вызывается до чтения указателя. Если `latest.json` есть, а `--force` нет, — `exit 2`, текст называет ключ, seq и `--force`. Снапшот пишется через `Context.Snapshot(…, SnapshotBootstrap)`. DoD-команда даёт `exit 0`, `entities_count: 6`, `state_hash: sha256:0046b8a7…` (совпадает с фикстурой seq 0), `cursor.system_events: 11`. Курсор = офсет последнего предложения + 1: тест сверяет его с журналом и проверяет, что в `[cursor, End)` нет предложений |
| 5 | Закрытие хранилища и ошибка `Stop` (`init.go:196-225`, `:234-279`; `context.go:321-370`) | `Stop` собирает `errors.Join(subErr, ошибки воркеров, «shutdown snapshot of X: %w»)`. `writeSnapshot` возвращает первую ошибку, поэтому `errSealed` бывает только в элементе снапшота `shutdown`. Отбрасывание узкое: по идентичности `errSealed`, а не любой ошибки (мутант MR-D красный). Настоящие ошибки не прячутся. Обратная сторона: ошибка после записанного снапшота показывается как провал bootstrap (Mi-1). Кроме того, `unexpected` пересобирает ошибки с несколькими `%w` (N-1) |
| 6 | `--force` (`world_test.go:190-211`, зонд P1) | Снапшот получает seq 1, seq 0 остаётся, `state/` не чистится. Зонд P1: объект `player-A` v7 переписан версией 1, чужой объект `player-Z` v3 остался, в снапшоте 6 сущностей. Над T-059 тест `--force` падает по таймауту (Mi-3, риск 1) |
| 7 | `--bus kafka` (`init.go:68-81`) | Проверка `--bus` идёт до книги правил, фикстур и хранилища. `exit 2`, текст называет маршрут, T-059 и «nothing was done». Тест проверяет, что хранилище не открывалось. Ручной прогон — exit 2 |
| 8 | `--store` по умолчанию | Пустое значение даёт `memory`. Тесты передают `--store memory` явно и открывают только память (`stand.command`). Но `--store minio` при `--bus memory` принимается (Ma-1) |
| 9 | depguard `cmd-mvctl-world` (`.golangci.yml:203-204`, `:423-440`) | Правило минимальное: `files` — один каталог, `allow` — ровно `internal/state$` и `internal/mechanics$` с `$`. Исключение в `internal-unlisted` — тот же один каталог. Форма совпадает с `cmd-telegram-bot`. Других правил `cmd-mvctl-*` в файле нет. `no-testkit-in-production` и `cmd-others` (запрет `internal/replay`) продолжают действовать. Другие правила не ослаблены. Нужны отметки system-architect (`contract-change`) и tech-lead#1 (п. 4 (а)) |
| 10 | `main_test.go`, `commands_state.go` | `commands_state.go`: зарезервированная строка заменена командой (п. 7). `main_test.go`: пример зарезервированной команды стал `blueprint validate`, `world` перенесён из зарезервированных в реализованные, добавлен `world status --store=memory` → `exit 1` (без сети). Правки минимальны и нужны. Файл EPIC-001, поэтому нужен просмотр tech-lead#1 |
| 11 | Тесты | `time.Sleep` нет: ожидание — `waitFor` на `clock.RealTimers`, таймаут — ручные таймеры. Файлы пишутся только в `t.TempDir()`, `/data` и корень диска не используются. Правка фикстур для закона — в копии. Хэш сверяется с фикстурой seq 0 |
| 12 | Мутанты (копия `scratchpad/t058r-mut` без `Docs/` и `services/`, без `-overlay`) | **K0** (копия без изменений) — зелёный. **MR-A** (`Tail` от 0 вместо `End`) — **выжил** (Mi-2). **MR-B** (`wait` не видит конец `Tail`) — **выжил** (N-2). **MR-C** (ошибка `Stop` после снапшота молча отбрасывается) — **выжил** (Mi-1). **MR-D** (`unexpected` отбрасывает любую ошибку, в цепочке которой есть `errSealed`) — красный (`TestUnexpectedKeepsWhatTheSealDidNotCause`). Копия удалена по точному пути |
| 13 | Прогоны в рабочей папке (go1.26.8 windows/amd64) | `go build ./... && go vet ./...` — 0, `gofmt -l` — пусто. `go test -short -count=1 ./internal/state/... ./cmd/mvctl/...` — ok, кроме `cmd/mvctl/internal/env` («Access is denied»). Обход `go test -c -o scratchpad/t058r_env.exe` — PASS, exe удалён. `golangci-lint run ./...` — 0 issues. DoD-команда — exit 0; повтор с `--json` — ok; `--bus kafka` — exit 2 |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Ma-1 | Major | `cmd/mvctl/internal/world/init.go:53-54`, `:82-88`; карточка, «Решения по ходу» п. 5 | `--store minio` при `--bus memory` принимается, и карточка предлагает этот путь для мира в MinIO. Снапшот `bootstrap` уходит в `snapshots-{world}` с `cursor.system_events: 11`. Это офсет журнала `membus` внутри команды: он исчезает вместе с командой, а факты `entity.created` в `system_events` деплоя не попадают. Read-model gateway и swarm по §4.4 и восстановление T-059 по §4.8 догоняют `system_events` Redpanda от этого офсета. На живом топике они применят к миру из фикстур чужие факты с офсета 11: `entity.updated` прежней жизни мира с версией ровно +1 применится молча. Если офсетов меньше, будет `log_gap`. Рядом с работающим `core` State команды — второй писатель мира. Черновик «Правки T-058» КД §4.10 (system-architect#1) запрещает эту комбинацию | `--bus memory` принимает только `--store memory`: иначе `exit 2` до открытия хранилища, в тексте — причина (курсор чужого журнала) и то, что мир в MinIO создаёт путь `--bus kafka`. Строку help `--store` поправить. Тест: `--bus memory --store minio` → `exit 2`, `opened == 0`. Решение 5 в карточке и `dev-log.md` переписать |
| Mi-1 | Minor | `init.go:127-135`, `:203-210`, `:216-223` | Провал после записанного `latest.json` показывается как провал bootstrap. (а) `Context.Snapshot` возвращает указатель **и** ошибку, если снапшот записан, а `snapshot.created` не вышел (`snapshot.go:175-178`). Команда отвечает `[snapshot]`, `exit 1`, хотя указатель записан, и следующий `init` откажет «initialized». Комментарий `:203` («leaves no snapshot behind») здесь неверен. (б) Любая ошибка `Stop`, кроме закрытого хранилища, даёт `[bootstrap] … stop the in-process state`, `exit 1` при записанном мире. Пример на MinIO: снапшот `shutdown` сначала читает `ListSnapshots` (`snapshot.go:244`), и сбой этого чтения не `errSealed`. Черновик кодов выхода КД §4.10: `1` — «снапшот не записан». Мутант MR-C (ошибка `Stop` отброшена) выжил: теста нет | Если `pointer != nil`, снапшот записан: `exit 0`, недошедший `snapshot.created` или ошибку `Stop` показать предупреждением (отдельная проверка, не `bootstrap`). Либо `exit 1` с явным «мир инициализирован, latest.json записан». Тест: хранилище-обёртка, у которой после снапшота `bootstrap` отказывает `List`, — вывод и код по решению; мутант MR-C краснеет |
| Mi-2 | Minor | `internal/state/bootstrap.go:231`; `bootstrap_test.go` | Главное свойство ожидания не закреплено: ответы берутся только после `End`. Мутант MR-A (`Tail` от 0) выжил. На общем журнале (харнесс e2e, будущий путь kafka) он брал бы старые ответы из `[0, End)`. Пример: прошлый прогон получил отказ `law_violation` для `wolf-alpha`, фикстуры исправлены, а повтор сразу получает старый отказ и заканчивается `ErrBootstrap`. Второго прогона `Bootstrap` на том же журнале после отказа или без ответа нет ни в одном тесте | Тест на одном стенде (State + `membus`): первый `Bootstrap` с фикстурами, которые нарушают закон, — `ErrBootstrap`, созданы world и region. Второй с исправленными фикстурами — `Skipped` world и region, `Created` остальных, ни одного второго `entity.created`. Мутант MR-A краснеет |
| Mi-3 | Minor | `init.go:112-126`; `world_test.go:190-211` | `--force` над хранилищем с объектами сущностей. Зонд P1: объекты фикстур переписаны версией 1 (`player-A` v7 → v1). Объекты не из фикстур и `_intents/` остаются (`player-Z` v3), снапшот говорит о 6 сущностях. По §4.8 такой объект (`version > mem + 1`) даёт `state_divergence`. Зонд P2 (копия с `internal/state` из рабочей папки T-059): `Start` восстанавливает мир (`log_gap: true`), предложения bootstrap State гасит без ответа как применённые. `TestInitWithForceOverAWorldWithSnapshots` падает через 10,02 с: `no answer to bootstrap:dark-forest-world:world/dark-forest-world`. Решение «`seq > 0`, `state/` не чистится» держится только на State без восстановления. Номер seq — вопрос 3 разработчика; здесь речь об объектах сущностей (бэклог 4 разработчика) | Исправлять одним изменением с решением по вопросу 3. Черновик КД §4.10: до запуска State удалить объекты State мира (`entities-{world}/**`, включая `_intents/`, и `snapshots-{world}/state/**`, указатель последним). Тест: объект не из фикстур и интент перед `--force` — после команды их нет, снапшот seq 0. Прогнать тесты `world` на слиянии с T-059 |
| Mi-4 | Minor | `init.go:137-146`; `status.go:37` | Текстовый отчёт `world init` не называет хранилище, а по умолчанию оно `memory`. Команда из DoD печатает «world dark-forest-world initialized», хотя объекты исчезают вместе с процессом. У `world status` по умолчанию `--store minio`, и сразу после такого `init` он отвечает «not initialized». Хранилище видно только в `--json` | Строка отчёта `store: memory (discarded when the command ends)` и то же в `Summary`. В help `world status` сказать, что `memory` видит только собственный процесс |
| N-1 | Nit | `init.go:261-279` | `unexpected` разбирает любую ошибку с несколькими `%w` (`Unwrap() []error`) и собирает `errors.Join` из листьев, даже если `errSealed` среди них нет. Текст формата теряется: из `fmt.Errorf("%w: %w: … did not go out after %d attempts: %w", …)` остаются только тексты сентинелов | Если ни один лист не отброшен, вернуть исходную ошибку. Отдельный случай в `TestUnexpectedKeepsWhatTheSealDidNotCause` |
| N-2 | Nit | `bootstrap.go:344-349` | Ветка «`Tail` завершился» не покрыта: мутант MR-B выжил. Если журнал отказал, `Bootstrap` ждёт 10 с и отвечает «no answer … is the context state serving», а не ошибкой журнала | Тест с `Journal`, у которого `Tail` сразу возвращает ошибку: `ErrBootstrap` с текстом этой ошибки без продвижения ручных таймеров |

### Оценка риска по вопросам 1–4 разработчика (не замечания)

1. **Текст КД §4.10 про `duplicate_entity`.** Риск низкий, дело только в документе. Код обрабатывает оба случая: применённое — по журналу, `duplicate_entity` — пропуск. Черновик system-architect уже описывает State так, как он работает.
2. **Снапшот `shutdown` через закрытое хранилище.** Риск средний. Сегодня отбрасывание узкое (п. 5 «Проверено»). Но оно опирается на внутреннее устройство `Stop`: снапшот `shutdown` — отдельный элемент `errors.Join`, а первая запись идёт после чтения `ListSnapshots`. T-059 переписывает `context.go` (≈250 строк). После слияния тесты M7 и M8 разработчика надо прогнать снова. Признак в `state.Config` снял бы эту связь и Mi-1 (б). Черновик КД принял закрытие хранилища.
3. **seq при `--force`.** Сам номер (`nextSeq`, ротация) риска не несёт. Серьёзнее соседнее: объекты сущностей и восстановление T-059 (Mi-3, зонд P2). После слияния с T-059 выбранное поведение не работает.
4. **Идемпотентность при `--bus kafka` и retention.** Риск высокий, механизм подтверждён зондом P2 в другой форме. State знает мир из объектов или снапшота, а журнал `entity.created` не хранит (retention или новый `membus`). Тогда повтор гаснет без ответа, и через 10 с — `ErrBootstrap`. Вдобавок `createdBefore` читает весь `[0, End)` `system_events` на каждом прогоне. Черновик КД для пути kafka решает «уже создано» по объектам `entities-{world}`, а не по журналу. С этим согласен.

### Пересечение с T-059 и T-472 (только чтение их рабочих папок)

- **T-059.** Текстовых общих файлов нет: T-059 правит `internal/state/{apply,context,dedup,intent,snapshot,store}.go`, `cmd/multiverse/contexts_state.go`, `shared/testkit/state`; T-058 — новые файлы, `commands_state.go`, `main_test.go`, `.golangci.yml`. Смысловые пересечения:
  - **сборка ломается гарантированно:** `const CauseInit = "init"` объявлен и в `internal/state/bootstrap.go:29` (T-058), и в `internal/state/world.go:11` (T-059). В копии слияния — `CauseInit redeclared`. Одно объявление надо оставить до слияния второй задачи. Логичнее в `bootstrap.go`: §4.10 — T-058;
  - `Start` T-059 восстанавливает мир из хранилища, и `--force` падает (Mi-3);
  - правило `uninitialized` T-059 (`initializes`: `KindCreate && cause=init`) с предложениями bootstrap совместимо;
  - в копии слияния `go test ./internal/state/` висел 10 минут в тестах T-059 (`recovery_world_test.go:134`, `:167`). T-059 не закончена, связь с T-058 не устанавливал.
- **T-472.** Общих файлов нет. В копии с изменениями рабочей папки T-472 (`proposal.go`, `apply.go`, `helpers_test.go`, `shared/entity/*`) `go vet` чистый, `cmd/mvctl/internal/world` — ok, тесты `Bootstrap|Fixture` в `internal/state` — ok. Риск низкий. Если T-472 переименует помощники `helpers_test.go` (`newBus`, `onTopic`, `waitFor`), `bootstrap_test.go` перестанет собираться.

### Риски и допущения

- Зонды P1 и P2 и мутанты гонялись в копиях `scratchpad/t058r-mut` и `t058r-mut472` (без `Docs/` и `services/`). P2 — поверх незаконченной рабочей копии T-059 с переименованной второй `CauseInit`. Итог P2 может измениться вместе с T-059, но механизм (восстановленный мир гасит bootstrap без ответа) следует из §4.8 и §4.5 п. 2. Копии и скрипт удалены по точным путям.
- `.golangci.yml` и `main_test.go` — общие файлы. Без отметок system-architect (`contract-change`) и tech-lead#1 задача не сливается. Отметок в карточке на момент ревью нет.
- Ветка от `0345177`, кончик эпика `d91ba59`. Между ними правки только compose-lint, `Makefile`, `.github/ci.env`, `.dev-team.json` — с файлами T-058 не пересекаются.
- Интеграционный прогон (MinIO) не делал: DoD его не требует, а после Ma-1 путь MinIO из команды уходит.
- В рабочей папке T-058 я менял только этот раздел `review.md`. Карточку T-058 не трогал. Docker, `.env` и стенд `:8888` не трогал.

### Предложения в бэклог

1. **EPIC-002, задача пути `--bus kafka` (после T-059):** «уже создано» решать по объектам `entities-{world}`, а не по журналу (черновик КД §4.10 (б)). Тест: повтор после retention не ждёт таймаута.
2. **EPIC-002 / T-062:** e2e-харнесс, который вызывает `Bootstrap` в процессе с восстановлением State, должен проверять повтор `Bootstrap` над восстановленным миром (зонд P2).
3. **EPIC-002:** `shared/testkit/state.LoadFixtures` вызывает `state.LoadFixtures` (бэклог 2 разработчика) — один читатель фикстур.

## T-058 · ревью #2 · 2026-09-14 · code-reviewer#1 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-058`, ветка `task/T-058-world-init`, база `0345177`. Коммитов нет, всё в рабочей копии. Итерация 2 — проверял только исправления и регрессию от них:
- `cmd/mvctl/internal/world/init.go`: Ma-1, `clearWorld`, `inProcessRun` с предупреждениями, `withoutSeal`;
- `status.go`: справка и подсказка `memory`;
- `world_test.go`: новые и переписанные тесты;
- `internal/state/bootstrap_test.go`: `TestASecondBootstrapIsNotAnsweredByTheRefusalOfTheFirst`, `TestBootstrapEndsWhenItsJournalStops`.

`bootstrap.go`, `.golangci.yml`, `commands_state.go` и `main_test.go` с ревью #1 не менялись.

Сверял с разделами карточки «Решения system-architect» и «Итерация 2 (developer)» и с КД `state-and-mechanics.md` §4.10 «Правила `world init`» («Правка T-058»). Для слияния читал рабочую папку `.worktrees/T-059`, ничего в ней не менял.

### Вердикт

**Принять.** Blocker (Critical): 0 · Major: 0 · Minor: 0 · Nit: 2.

Ma-1, Mi-1…Mi-4, N-1 и N-2 ревью #1 закрыты. Правило `--force` совпадает с КД §4.10 по порядку, границам и кодам выхода. На пробном слиянии с T-059 (после удаления второй `CauseInit`) тесты `cmd/mvctl/internal/world` и `internal/state` зелёные. Новые замечания — два Nit, возврата они не требуют.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | **Ma-1** (`init.go:87-100`) | Сочетание проверяется после разбора `--bus` и до `mechanics.Load`, `LoadFixtures` и `OpenStore`. Пустое `--store` значит `memory`. Любое другое значение (`minio`, `s3`) даёт `exit 2`: текст называет курсор чужого журнала, второго писателя, путь `--bus kafka` и «nothing was done». Справка `--store` сказана так же. Тест — строка `memory bus, minio` в `TestWorldRefusesAWrongCall` (`exit 2`, `opened == 0`). Ручной прогон `--bus memory --store minio` — `exit status 2`, текст верный. Мутант C1 красный. **Закрыто** |
| 2 | **`--force`: порядок** (`init.go:136-141`, `:195-223`) | Очистка идёт после `EnsureWorldBuckets` и `ReadLatest`, до `initInProcess`. Порядок: (1) `List(entities-{world}, "")` и удаление каждого объекта, `_intents/` тоже (MinIO `List` рекурсивен, `minio.go:160`); (2) `List(snapshots-{world}, "state/")` и удаление всего, кроме указателя; (3) `Delete(state/latest.json)` последним. `swarm/` и `gateway/` под префикс `state/` не попадают. `Delete` отсутствующего ключа не ошибка (memory и MinIO), поэтому `--force` над миром без указателя тоже проходит. Совпадает с КД §4.10 «`--force`» |
| 3 | **`--force`: seq 0 и сохранность чужого** (`TestInitWithForceOverAWorldWithSnapshots`) | Перед `--force` в хранилище лежат `player-Z`, интент, лишний снапшот seq 7 и `swarm/latest.json`. После: один объект снапшота seq 0 `bootstrap`, указатель на него, `state_hash` фикстуры, в бакете сущностей ровно 6 объектов без интентов и `player-Z`, `swarm/latest.json` цел, в выводе `seq 0`. Мой мутант F5 (очистка пропускает `_intents/`) красный |
| 4 | **Оборванная очистка** (`TestInitWithForceCutOffKeepsThePointer`) | Хранилище отказывает `Delete` объекта снапшота. Результат: `exit 1` `[store]`, указатель на месте, следующий `init` без `--force` даёт `exit 2`. State не стартует: ошибка `clearWorld` возвращается до `initInProcess`. Мутанты F2 (указатель первым) и F4 (ошибка очистки проглочена, State стартует) красные. Замечание вне кода: оборванная очистка оставляет указатель на, возможно, удалённые сущности. КД это принимает («следующий `init` отказывает, а не создаёт мир поверх остатков»), выход — повтор `--force` |
| 5 | **Mi-1** (`init.go:143-177`, `:284-320`) | `pointer != nil` у `Context.Snapshot` означает, что записаны объект **и** `latest.json`: `PutSnapshot` возвращает указатель только после PUT указателя (`store.go:231-254`). Значит, `exit 0` с предупреждением не выдаёт недописанный мир за инициализированный. Разбор по веткам ниже. `exit 1` `[bootstrap]`/`[snapshot]` остаётся за случаями «снапшот не записан». Предупреждение печатается в stderr для текста и в `details.warnings` для `--json`. Мой мутант W3 (предупреждение не печатается) красный. **Закрыто**, остаток — N-3 |
| 6 | **Маскирует ли `exit 0` неконсистентный мир** | Нет. (а) Ошибка `Stop` после снапшота: хранилище закрывается **до** `Stop` (`defer`, `objects.seal()` первым). `Stop` уже ничего не пишет и не удаляет, и хранилище остаётся ровно таким, каким его зафиксировал снапшот. Остаются только сбои чтения или остановки воркеров. (б) `snapshot.created` не вышел: объект, указатель и сущности согласованы, нет только события. Журнал `membus` исчезает с командой, а после Ma-1 и хранилище только `memory`, так что потребителя у события нет. (в) Сбой ротации после снапшота только логируется (`snapshot.go` `rotate`), а над свежим хранилищем ротировать нечего. Для будущего пути `--bus kafka` ветка (б) значима: потребители узнают мир из снапшота (КД §4.10 (б), C-14 v1.4), а предупреждение оператор видит |
| 7 | **Ветка «`snapshot.created` не вышел» без теста** | Мутант W2 (указатель с ошибкой → находка `[snapshot]`, `exit 1`) **выжил**. Критичность низкая. Внутри команды `membus` без правки кода отказать публикации не может, в production ветка практически недостижима, код — три строки с комментарием. Если ветку сломать, пострадает только код выхода редкого случая, мир от этого не портится. Nit N-3 |
| 8 | **Mi-2** (`bootstrap_test.go:471-506`) | Один стенд (State + `membus`). Первый `Bootstrap` с волком в `nowhere` даёт `ErrBootstrap` `law_violation`, созданы world и region. Второй с фикстурами дерева: `Skipped` = world, region, `Created` = остальные, `entity.created` ровно 6. Мутант MR-A (`Tail` от 0) воспроизвёл — красный. **Закрыто** |
| 9 | **Mi-4** (`init.go:167-168`, `:180-186`; `status.go:37-39`, `:70-77`) | В тексте `init` есть строка `store: memory (its objects are gone when the command ends)` и итог `initialized in the memory store`. Справка `status --store` и находка при `memory` объясняют, что `memory` видит только записанное этой командой. Коды `0`/`1` у `status` не менялись. Тест S1 разработчика и `TestStatusOfAWorldNobodyInitialized` проверяют текст. **Закрыто** |
| 10 | **N-1** (`init.go:355-389`) | `withoutSeal` разбирает только ошибки с `Unwrap() []error`. Если `errSealed` среди листьев нет, возвращается исходная ошибка с полным текстом. Случай `several wraps in one format` проверяет и `errors.Is`, и неизменность текста. Ошибка с одним форматом, где `errSealed` — один из нескольких `%w`, по-прежнему разбирается и теряет текст формата, но так и написано в комментарии, и в `Stop` такой формы нет. **Закрыто** |
| 11 | **N-2** (`bootstrap_test.go:508-538`) | Журнал, у которого `Tail` сразу отказывает, при ручных таймерах без продвижения. Результат — `ErrBootstrap` с текстом журнала: ветка `<-tail.done` в `wait` покрыта. Тест детерминирован, таймаут недостижим. **Закрыто** |
| 12 | Регрессия от итерации 2 | `initInProcess` сохранил остановку в `defer` на любом пути. Хранилище закрывается до `Stop` и после успеха, и после неудачи. На неудаче bootstrap `Stop` пишет только в закрытое хранилище, поэтому `latest.json` не появляется (`TestInitOfFixturesThatBreakALawLeavesNoSnapshot`). Ошибка `Stop` при уже установленном `run.err` теряется (N-4). `TestInitWarnsOfAFailureAfterTheSnapshotIsWritten` проверяет, что до отказа дошло (`refused`), поэтому тавтологией не является |
| 13 | Мутанты ревьюера (копия `scratchpad/t058r2-mut` без `Docs/`, `services/`, `.git`; без `-overlay`) | Контрольные K0 `world` и K0 `internal/state` (`-run Bootstrap|Fixture`) — зелёные. Красные: C1, F2, F4 (мой), F5 (мой), W3 (мой), MR-A. **Выжил** W2 (мой) — N-3. **L2** (depguard): в копии в production-файл `cmd/mvctl/internal/storage/storage.go` добавлен импорт `internal/state`. Результат — 1 нарушение, `import 'multiverse-core.io/internal/state' is not allowed from list 'internal-unlisted'`. Контроль L0 (`storage` и `world` без правки) — 0 issues. Результат разработчика воспроизведён. Мутанты ставились через точную замену текста с проверкой, что образец встречается ровно один раз, файл восстанавливался побайтно |
| 14 | Прогоны в рабочей папке (go1.26.8 windows/amd64, golangci-lint 2.13.2) | `gofmt -l cmd internal` — пусто. `go build ./... && go vet ./...` — 0. `go test -short -count=1 ./internal/state/... ./cmd/mvctl/...` — ok, кроме `cmd/mvctl/internal/env` («Access is denied»); обход `go test -c -o scratchpad/t058r2_env.exe` — PASS, exe удалён по точному пути. `golangci-lint run ./...` — 0 issues. DoD-команда `go run ./cmd/mvctl world init --world dark-forest-world --fixtures testdata/fixtures/ --bus memory` — `exit 0`, `entities_count: 6`, `state_hash: sha256:0046b8a7…`, `cursor.system_events: 11`, строка `store: memory (…)`. С `--force --json` — `status: ok`. С `--store minio` — `exit 2` |

### Пробное слияние с T-059 (копия `scratchpad/t058r2-merge`)

Способ такой. За основу взята копия дерева T-058, поверх неё положены все изменённые и новые кодовые файлы рабочей папки T-059 (без `Docs/`). База обеих задач — `0345177`. Общих кодовых файлов нет (`git status` обеих папок): у T-059 файлы берутся как есть, у T-058 — свои, `git merge-file` не нужен. Общий файл один — `Docs/.../dev-log.md`, его сводит драйвер `appendtail`. Этот файл не проверял.

| # | Проверка | Результат |
|---|---|---|
| S1 | Сборка как есть | `internal\state\world.go:11:7: CauseInit redeclared in this block` (`bootstrap.go:29:7`). Git текстового конфликта **не покажет**: объявления лежат в разных файлах. Слияние пройдёт молча, а упадёт сборка |
| S2 | Рецепт: из `world.go` (T-059) убраны комментарий и `const CauseInit`, в `bootstrap.go` объявление осталось | `go build ./...` — 0. `go vet ./internal/state/... ./cmd/...` — 0. `golangci-lint run --allow-parallel-runners ./internal/state/... ./cmd/... ./shared/testkit/state/...` — 0 issues |
| S3 | `go test -short -count=1 ./cmd/mvctl/internal/world/... ./internal/state/...` | ok (`world` 0,9 с, `internal/state` 2,0 с, `memstore` ok). Все 11 тестов `TestInit*`/`TestStatus*` — PASS на новом `Start`, который при `Objects` восстанавливает мир. Условие приёмки system-architect (вопрос 2) на пробном дереве выполнено. Дополнительно `./cmd/mvctl/` и `./cmd/multiverse/` — ok |
| S4 | Закрытие хранилища над `Stop` T-059: мутант «без `objects.seal()`» | Красный на слитом дереве: 5 тестов (`TestInitCreatesTheWorldAndItsSnapshot`, `…RefusesAWorldAlreadyInitialized`, `…WithForceOverAWorldWithSnapshots`, `…OfFixturesThatBreakALawLeavesNoSnapshot`, `TestStatusPrintsTheSnapshotOfTheWorld`). С новым `Stop` закрытие по-прежнему нужно, и тесты его держат |
| S5 | Зонд «повтор `init` без `--force` после неудачного bootstrap» (риск 1 разработчика) на общем хранилище | Первый прогон с волком в `nowhere`: `exit 1` `[bootstrap] … law_violation`, в `entities-{world}` остались `world/dark-forest-world.json` и `region/dark-forest-01.json`, снапшотов нет. Повтор без `--force` с фикстурами дерева: **`exit 0` за 6 мс**, `entities created 6, skipped 0`, seq 0, `state_hash` фикстуры. Таймаута нет. Восстановление T-059 без `latest.json` поднимает мир из объектов (`recoverWithoutSnapshot`, `no_snapshot`), а повтор предложения по висящей записи досылает факт (КД v0.4 §4.8). Повтор с `--force` — тоже `exit 0`. **Риск 1 разработчика на текущей рабочей копии T-059 не подтверждается.** В CLI он и так недостижим: хранилище `memory` у каждой команды своё. Зонд удалён вместе с копией |

**Порядок слияния.** Оба порядка работают, если в коммит слияния, который идёт вторым, входит удаление `const CauseInit` из `internal/state/world.go`, а перед коммитом прогнан `go build ./...`. По решению оркестратора объявление остаётся в `bootstrap.go`, так что при любом порядке правится `world.go`. Если T-058 сливается второй, это правка файла T-059 в коммите слияния — файл того же эпика (`ownership.md` §1), не нарушение. С точки зрения ревью удобнее **T-059 первой, T-058 второй**: условие приёмки system-architect («тесты `world` на новом `Start` после слияния T-059») тогда проверяется на настоящей ветке эпика одним прогоном `go test ./cmd/mvctl/internal/world/... ./internal/state/...` в коммите слияния T-058.

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| N-3 | Nit | `cmd/mvctl/internal/world/init.go:304-315` | Ветку «снапшот записан, `snapshot.created` не вышел → предупреждение, `exit 0`» не проверяет ни один тест: мутант W2 («такой случай — находка `[snapshot]`, `exit 1`») выжил. Тест `Stop` покрывает соседнюю ветку, а не эту. Сегодня ветка практически недостижима (`membus` внутри команды), но путь `--bus kafka` получит её в реальной форме | Без правки `membus`: вынести решение «`pointer`, `err` → `run`» в чистую функцию (например `snapshotOutcome(pointer, err) (meta, warning, err)`) и проверить её таблицей: указатель и ошибка, только указатель, только ошибка. Либо взять тест в задачу пути `--bus kafka`, где публикация идёт по настоящей шине |
| N-4 | Nit | `cmd/mvctl/internal/world/init.go:289-296` | Если bootstrap уже упал (`run.err != nil`), ошибка `Stop`, отличная от `errSealed` (например, воркер не остановился за `StopTimeout`), молча отбрасывается: `else if run.err == nil`. Оператор видит отказ State и не видит, что State к тому же не остановился | Во втором случае присоединить: `run.err = errors.Join(run.err, stopped)`. `errSnapshot` в цепочке сохранится, и выбор `CheckSnapshot`/`CheckBootstrap` не изменится |

### Открытые вопросы (оркестратору)

Нет. Замечания для слияния — в разделе «Пробное слияние» (порядок, `go build` перед коммитом слияния).

### Риски и допущения

- Слияние проверено на **незакоммиченной** рабочей копии T-059 на момент ревью. Если T-059 до слияния изменится (`context.go`, `recovery.go`, `Stop`), S3–S5 нужно повторить. Сами команды — в таблице выше.
- `--force` при `--bus memory` рабочий только в тестах с общим хранилищем. В CLI хранилище `memory` у каждой команды новое, а путь `--bus kafka` по КД §4.10 (в) `--force` отказывает. Значит, сегодня `clearWorld` в production не исполняется, а правило написано на будущее и для харнессов. Два следствия. Первое: повреждённый `latest.json` (`ReadLatest` с ошибкой разбора) даже при `--force` даёт `exit 1` `[store]`, очистки не будет. Над `memory` это недостижимо, но задаче пути kafka или будущему `--store` стоит это решить. Второе: подсказка `world status --store minio` «run mvctl world init» пока ведёт к команде, которая мир в MinIO не создаёт (до задачи пути kafka).
- Ветка от `0345177`, кончик эпика `d91ba59`. Файлы между ними с T-058 не пересекаются (ревью #1), повторно не сверял.
- `.golangci.yml` и `main_test.go` с ревью #1 не менялись. Ревью правила system-architect в карточке есть. Отметки tech-lead#1 и `contract-change` остаются условием слияния.
- Копии `scratchpad/t058r2-mut` (переименована в `t058r2-merge`), скрипт мутантов, списки файлов и exe удалены по точным путям. В рабочей папке T-058 я менял только этот раздел `review.md` и раздел «Ревью (code-reviewer)» карточки. В `.worktrees/T-059` ничего не менял. Docker, `.env` и стенд `:8888` не трогал. `golangci-lint` в копии слияния запускался с `--allow-parallel-runners`: в это время шёл чужой прогон.

### Предложения в бэклог

1. **EPIC-002, задача пути `--bus kafka`:** тест ветки «снапшот записан, `snapshot.created` не вышел» (N-3), решение про `--force`/повреждённый указатель и подсказку `world status` для `minio`.
2. **EPIC-002 / T-062:** e2e-харнесс с восстановлением State — повтор `Bootstrap` после неудачи над объектами без указателя (зонд S5) как регрессионный тест, а не только ручной зонд.
3. **Процесс слияний:** для пар задач, у которых новые файлы в одном пакете, в DoD шага слияния держать `go build ./...` до коммита слияния. Семантический конфликт (`CauseInit`) git не показывает.

## T-059 · ревью #1 · 2026-09-14 · code-reviewer#4 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-059`, ветка `task/T-059-state-recovery`, база `0345177`. Коммитов нет, всё в рабочей копии. Код: новые `internal/state/{recovery,catchup,health,admin,world}.go`, правки `apply.go`, `context.go`, `dedup.go`, `intent.go`, `snapshot.go`, `store.go`, `memstore/memstore.go`. Тесты: `recovery_test.go`, `recovery_world_test.go`, `recovery_helpers_test.go`, правки `resend_test.go`, `persist_test.go`, `objects_helpers_test.go`, `export_test.go`. Страж `shared/testkit/state/guard.go` с тестом. Процесс: `cmd/multiverse/contexts_state.go`, `contexts_state_recovery_test.go`, `fake_contexts_test.go`. Сверял с разделом `### T-059` в `tasks.md`, в том числе с 8 строками приёмки T-057. Сверял также с КД `state-and-mechanics.md` v0.4 (§4.8, §9, §10, §18), ADR-011 с дополнением 2026-09-14, C-01 (Journal, v1.11 — маршруты), C-02, C-14 и NFR-061. Вопросы разработчика 1–6 и риск compose замечаниями не ставлю: их решает system-architect#1. Оценка риска — в разделе «Риски». Правки `Docs/` в ветке задачи сделаны по принятому в эпике порядку (так было и в T-057), замечанием не считаю.

### Вердикт

**Вернуть.** Blocker (Critical): 0 · Major: 1 · Minor: 3 · Nit: 2.

Протокол §4.8 выполнен в правильном порядке: указатель, снапшот с проверкой, окно, догон, сверка, roll-forward, событие, затем `Tail` с `End`. Одна публикация факта через рестарт соблюдается. Мир `uninitialized`, `/health`, admin-маршрут и подключение хранилища в процессе сделаны по DoD. Причина возврата — Ma-1. Интент полностью записанного пакета остаётся после неудачного `DeleteIntent`, а КД §9 прямо разрешает миру работать дальше. Если после этого любая сущность пакета изменилась, рестарт останавливает мир `state_divergence`. Зонд это подтвердил.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Порядок §4.8 (`recovery.go:132-191`, `context.go:200-246`) | `End` берётся один раз, до восстановления всех миров процесса (КД: «`End` берётся у шины до чтения»). Дальше по шагам: `ReadLatest` → `intactSnapshot` (указатель, затем до K=5 из `ListSnapshots` по убыванию `seq`) → `Replace` → `Dedup.Restore` → `readFacts [cursor, End)` → `catchUp` (окно дополняется `proposal_id` каждого факта) → `reconcile` → `rollForward` → `announceRecovery` → `a.cursor = End` → worker'ы → `Tail(End)`. Факты мира recovery не публикует (§18). Без указателя: объекты есть → `no_snapshot`, объектов нет → `uninitialized`, в обоих случаях с roll-forward |
| 2 | Идемпотентность повторного рестарта | (a) второй рестарт без предложений: `identical`, ни одного факта, страж зелёный. Висящий объект после рестарта остаётся с пустым `fact_event_id` и в снапшоте `shutdown`. На следующем рестарте ветка `sameProposal` сверки переносит пустой id, так что досылка остаётся за повтором предложения. `snapshot_corrupted` и `state_divergence` снапшот при `Stop` не пишут (`context.go:407`), поэтому следующий рестарт видит то же |
| 3 | `snapshot_corrupted` | Не JSON (`ErrUndecodable`), `ErrNotFound`, проверка метаданных и `state_hash` → следующий ключ. Все битые → `stopRecovery`: мир остановлен, 0 сущностей, `replay.completed` нет, предложения идут в `dead_letters`, `latest.json` не тронут. Проверка «ключ выведен из `taken_at`/`seq`» в коде есть, но тестом не закреплена (Mi-2) |
| 4 | `state_divergence` | Пропуск версии, два факта одной версии (по истории ≤ 50), `a[n]` за концом, объект на две версии впереди — тесты есть. Ветка «объект той же версии с другим состоянием» (`recovery.go:533`) тестом не закреплена (Mi-3). Интент над сущностью, ушедшей дальше `to_version`, даёт ложную дивергенцию (Ma-1) |
| 5 | Собственный курсор | С хранилищем каждый `Start` — рестарт. `Tail` идёт с `End`, `ReadRange` берёт `[cursor, End)`, границы не пересекаются, двойной обработки на `End` нет. Мутант MF «`Tail` с 0 вместо `End`» — красный (3 теста). Курсор снапшота у каждого мира свой (`Applier.cursor`): офсет после последнего предложения этого мира. Он отстаёт в безопасную сторону, `Tail` у процесса один. Без хранилища «последний обработанный» живёт в `Context.next` (atomic, только в памяти). `dispatch` двигает его только после обработки без ошибки, поэтому `Start` того же объекта предложение, прерванное `Stop`, прочитает снова. При рестарте процесса значение теряется: чтение идёт с 0 (риск 3) |
| 6 | `log_gap` | Разрыв — это `cursor > End` или первый доставленный обработчику офсет ≠ `cursor`. На membus (журнал моложе снапшота) и на обёртке с «обрезанным» началом это работает. Если запись на офсете `cursor` уходит в `dead_letters` при чтении (не декодируется, неизвестный тип, схема), разрыв ложный: зонд P2 (Mi-1). На kafka: `ReadRange` клампит только `to`. kafka-go при `OffsetOutOfRange`, насколько я знаю библиотеку, переходит к первому офсету, тогда `first ≠ from` и `log_gap` распознаётся. В этом дереве это не проверено, нужна интеграция (риск 4). Сущность в объектах новее снапшота берётся из объекта с пустым `fact_event_id`. Сущность той же версии под тем же предложением сохраняет запись снапшота с id факта (тест g) |
| 7 | `catchup.go` / `CatchUpChanged` | Правило C-02 v1.6 соблюдено. `new` пишется, промежуточный не-контейнер заменяется объектом. `a[n]` при `n = len` дописывается, `nil`/отсутствие для `n = 0` — тоже. `n > len` и индекс в не-список дают `ErrCorruptFact` → `state_divergence`. Без `new` путь удаляется, отсутствующий путь — no-op. Глубокая копия на входе и на значении, общих ссылок с фактом нет. `catchUpUpdated` проверяет версию `+1` до и после `Commit` и пропускает дубль по `announcerOf`/`announced`. Факт с пустым `changed[]` проверяется по id. Сверка с живым применением через `StateHash` — пять видов операций и именованные векторы. Неканонический путь не проверяется — это известно (слияние с T-472), не замечание |
| 8 | Сверка с объектами (`reconcile`) | `mem+1` и новая сущность v1 — висящая запись: принимается, не публикуется, лог `Info`. Та же версия под тем же предложением — в память идёт запись коммита объекта с id факта из памяти (§18, «Догон атомарного пакета»). Сущность памяти без объекта — дивергенция. Тест §18 (`player-A v2` в журнале, `wolf-alpha v2` в объектах) — ровно один `wolf-alpha v2`, второго `player-A v2` нет |
| 9 | Roll-forward до приёма предложений | Выполняется до worker'ов и `Tail`. Пишет `batch_size = len(changes)`, `atomic=true`, `applied_at` интента. После удаления интента `pending_intents` = число неудалённых. Тесты (d) и (d′) зелёные. Дефект — Ma-1 |
| 10 | `analytics.replay.completed mode=recovery` | Корень `NewRoot` с миром и `actor_kind=system`. `run_id` детерминирован (`core/state:{world}:{End}`). Без снапшота `snapshot_id: null`, `state_hash_before` и `identical` не ставятся. Схема это допускает (`required` без них), тест (a) валидирует событие. Публикуется один раз без повторов, мир от события не зависит |
| 11 | admin `POST /v1/admin/state/{world}/snapshot` | Маршрут стоит за `runtime.AdminOnly`, шаблон с методом (C-01 v1.11). Ответы: 403 без `X-Client-Id` из списка и при неизвестном `X-Actor-Kind`; 404 `unknown_world`; 409 `no_object_store`; 503 `not_running`/`world_stopped`; 500 `snapshot_failed`; 405 на GET. Тело ошибок — `{"error":{"code","message"}}`. Без `X-Actor-Kind` ответ 200. Это поведение `AdminOnly` по C-01 v1.11 и C-06 («доступ — как у прочих `/v1/admin/*`»), код контракту соответствует. Строка DoD «отклоняет запрос без `X-Actor-Kind`» с контрактом расходится. Отступление уже принято: КД §19 п. 9 (правка system-architect#1, внесена параллельно с ревью) |
| 12 | `/health` мира | Всегда: `entities`, `state_hash`, `rules_version`, `cursor`, `pending_intents`. При снапшоте: `seq`, `snapshot{seq,taken_at,age_s}`. `fail` с `reason` из пяти значений, при битом снапшоте ещё `snapshot: corrupted`. `degraded`: `publish_attempts_failed`, `world: uninitialized`, `snapshot_stale` (секунды, `null` без снапшота, текст ошибки только в лог), `snapshot_event_failed`, `log_gap`/`no_snapshot` до первого снапшота. Время — по часам `Applier`. Тест на ручных часах есть, мутант M3 разработчика красный |
| 13 | Мир без init | До записи, после `Stopped()` и разбора, отклоняется всё, кроме `create cause=init`: `unknown_entity` с сущностью мира. Признак снимается созданием сущности мира. Снапшот `shutdown` неинициализированного мира не пишется. Тест: ход и создание персонажа шлюза → отказ, записей нет, мир не `fail`; бакеты + init → `ok` без рестарта |
| 14 | `cmd/multiverse/contexts_state.go` | `Objects` и `RulesVersion` передаются одним изменением с восстановлением. Пути «хранилище без восстановления» нет: `Recover` безусловно вызывается в `Start` при `cfg.Objects != nil`. Другой production-код `Objects` не передаёт, `testkit/state` строит `NewApplier` без хранилища. Один ключ MinIO без другого — отказ `lawless` с именами переменных. `OneStateOverTheWorld` импортируется только из `_test.go` (`internal/state`, `cmd/multiverse`); `guard.go` зависит только от `contracts` и `eventbus`; depguard 0 |
| 15 | N-3, N-4 приёмки T-057 | `withAttempts` не повторяет `ErrNoBucket`/`ErrNotFound`, `ListIntents` на досылке идёт с повторами, число 12 запросов и ≈ 3,7 с — в комментарии `persistPauses`. `snapshot_event_failed` держится до вышедшего `snapshot.created`. Тесты есть |
| 16 | Пересечение с T-058 (`.worktrees/T-058`, только чтение; пробное слияние в копии) | `const CauseInit = "init"` объявлена и в `internal/state/bootstrap.go:29` (T-058), и в `world.go:11` (T-059). Файлы разные, текстового конфликта не будет, но сборка после второго слияния падает: «CauseInit redeclared». Больше общих файлов нет. In-process `world init` T-058 строит `state.New` с `Objects` и зовёт `Start` — после слияния это пойдёт через восстановление. В копии (T-059 + файлы T-058, одна `CauseInit` удалена) `go build ./...`, `go vet` и `go test -short ./internal/state/ ./cmd/mvctl/ ./cmd/mvctl/internal/world/` зелёные. Сигнатура `Context.Snapshot` не менялась, `ErrNotRunning`/`ErrUnknownWorld` T-058 не проверяет |
| 17 | Пересечение с T-472 (`.worktrees/T-472`, только чтение; пробное слияние в копии) | Общий файл кода один — `internal/state/apply.go`. T-059 правит поля `Applier`, курсор и проверку `uninitialized`, T-472 — `CheckAttributes` в `applyCreate`. `git merge-file` с базой `0345177` — 0 конфликтов. `proposal.go` и `helpers_test.go` правит только T-472, T-059 их не трогает. В слитой копии `go build ./...` и `go vet` дают 0. `go test -short ./internal/state/... ./shared/entity/... ./shared/testkit/state/... ./cmd/multiverse/...` зелёный, в том числе тесты recovery с неполными атрибутами поверх изменённого `helpers_test.go`. Конфликты ожидаются только в `Docs/` (`tasks.md`, `dev-log.md`, `review.md`, КД) |
| 18 | Мутанты (копия `scratchpad/t059r-copy` без `Docs/`, `services/`, `.claude/`; без `-overlay`; контрольный первым) | **C0** — копия без изменений, `internal/state` зелёный. **MF** (`Tail` с 0 вместо `End`) — красный. **MB** (`checkSnapshot` без сверки ключа с `SnapshotKey(taken_at, seq)`) — **выжил** (Mi-2). **MC** (`reconcile` без сравнения состояния при одной версии) — **выжил** (Mi-3). Зонды: **P1** (интент остался после неудачного `DeleteIntent`, затем ход по сущности пакета, `Stop`, рестарт) — мир `fail {reason: state_divergence}`: «the intent of prop-round takes player-A from v1 to v2, the world holds v3» (Ma-1). **P2** (на офсете курсора снапшота запись, которая не декодируется) — `LogGap=true`, `/health degraded {log_gap}`, запись в `dead_letters` (Mi-1). Копии удалены по точному пути |
| 19 | Прогоны в рабочей папке (go1.26, windows/amd64) | `go build ./... && go vet ./... && go vet -tags integration ./internal/state/...` — 0; `gofmt -l` — пусто. `go test -short -count=1 ./internal/state/... ./cmd/multiverse/... ./shared/testkit/...` — 11 пакетов ok. `internal/state` ×3 и `TestTheProcessRecoversItsWorldFromTheStore` ×3 — ok. `golangci-lint run ./...` — 0 issues. Стенд I1-α `-count=3` (`TestTheProcessRunsTheFightsOfIAlpha`, `TestTheProcessTellsTheDeathOfACharacter`, `TestTheStandWaitsUntilTheFakeHasLearntTheWorld`) — 9/9 PASS. Интеграцию не запускал |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Ma-1 | Major | `internal/state/recovery.go:615-649` (`rollForwardIntent`, ветки `:622` и `:624`); `internal/state/intent.go:157-165` (`finishedIn`) | Roll-forward пропускает сущность только при `held.Version == ToVersion` **и** `last_change.proposal_id` интента. В остальных случаях `held.Version != FromVersion` объявляется `state_divergence`. КД §9 разрешает интенту пережить записанный пакет: «Ошибка `DeleteIntent` после всех PUT мир не останавливает… оставшийся интент roll-forward пропускает — сущности уже на `to_version`». Мир при этом работает дальше, и следующий ход меняет сущность пакета: версия уходит выше `to_version` или ход без изменений переписывает `last_change`. Рестарт останавливает мир, хотя расхождения нет. Зонд P1: круг `prop-round` при отказе удаления интента, затем `prop-later` по `player-A`, `Stop`, рестарт — `fail {reason: state_divergence}`. Отказ одного DELETE в MinIO, то есть обычный сбой, после рестарта превращается в остановленный мир. `finishedIn` устроен так же: у повтора старого предложения с недосланным фактом он найдёт «незавершённый пакет». Кроме того, `rollForwardIntent` пишет изменения по одному и проверяет следующее после PUT предыдущего: при настоящей дивергенции на второй сущности первая уже дописана | `rollForwardIntent`: сначала проверить все изменения интента, не записывая. `held.Version >= ToVersion` — пакет по сущности записан, пропуск. Сущность не могла уйти дальше, пока пакет недописан: мир стоит `persist_failed`, а roll-forward идёт до приёма предложений. `held.Version == FromVersion` — дописать. Иначе (`held.Version < FromVersion`, сущности нет) — `state_divergence`, и ничего не писать. Затем PUT, затем `DeleteIntent`. `finishedIn`: то же условие `e.Version >= ToVersion`. Тесты: сценарий P1 (мир `ok`, интент удалён, `pending_intents=0`) и вариант «ход без изменений после пакета» (та же версия, другой `proposal_id`); мутант «прежнее условие» краснеет |
| Mi-1 | Minor | `internal/state/recovery.go:323-338` (`readFacts`, `first != from`) | Разрыв определяется по первому офсету, **доставленному обработчику** (так закреплено и в КД §4.8 правкой T-059 — вопрос 1). `Delivery.Deliver` не зовёт обработчик для записи, которая не декодируется или отвергнута при чтении (неизвестный тип, политика, схема), и паркует её в `dead_letters` (`delivery.go:132-136`, `DeliverRaw`). Если такая запись стоит ровно на `cursor`, `first` сдвигается, и recovery объявляет ложный `log_gap`. Последствия: мир из объектов, `/health degraded`; сущности впереди снапшота теряют id факта, и повтор их предложений опубликует факт второй раз. Каждый рестарт паркует ту же запись заново. Зонд P2 это подтвердил. Своё событие State на офсете курсора стоит почти всегда (факт или `snapshot.created`). Но курсор снапшота по счёту или по `admin` — это офсет после предложения, и следующим может стоять событие любого издателя. Например, тип, который знает только более новый реестр при поэтапном обновлении | Не выводить разрыв из доставки. Лучший вариант — «первый офсет журнала» в C-01 (`Journal.Start` или необязательный интерфейс, проверяемый приведением типа: membus — 0, kafka — `ReadFirstOffset`), через system-architect (`contract-change`). До решения — хотя бы проверить `from < first` через конкретную реализацию, а не через обработчик. Тест: на офсете курсора запись, отвергнутая при чтении → `log_gap=false`, `identical` |
| Mi-2 | Minor | `internal/state/recovery.go:272`; `recovery_test.go` | Мутант MB (убрать `SnapshotKey(meta.TakenAt, meta.Seq) != key`) выжил: весь `internal/state` зелёный. КД §4.4 требует выводить ключ из `taken_at` и `seq`, а не брать литералом: это мутация ревьюера T-016, Minor-1. Проверка в коде есть, но ничем не закреплена | Подслучай в `TestACorruptedSnapshotGivesWayToTheOneBefore`: у объекта последнего снапшота сдвинуть `snapshot.taken_at` на секунду, ключ и `state_hash` оставить → откат к предыдущему, в логе отказ по ключу |
| Mi-3 | Minor | `internal/state/recovery.go:533`; `recovery_test.go:457-509` | Мутант MC (убрать сравнение `StateHash` объекта и памяти при одной версии) выжил. Без этой ветки объект с другим состоянием при той же версии молча побеждает (другое предложение, `accepted++`) или проигрывает памяти (то же предложение). Дивергенция при этом не объявляется. Докстрока `reconcile` называет этот случай (`a different state at the same version`), тест — нет | Подслучай в `TestADivergenceStopsTheWorld`: объект `player-A` той же версии с другим `hp` → `state_divergence`, сообщение `byTheObjects` |
| N-1 | Nit | `internal/state/context.go:106`, `:125`, `:205` | При хранилище `Start` объявлен рестартом, и память, окна `proposal_id` и остановка мира строятся заново. Окно id событий `c.delivered` при этом переживает `Stop`/`Start` того же объекта. В настоящем процессе его нет. Тест, который повторит после рестарта событие, успешно обработанное до `Stop`, получит отсев в `dispatch` и не дойдёт до пути досылки. Сейчас такие тесты повторяют только необработанные события | В ветке `objects != nil` создавать `c.delivered = eventbus.NewDedup(...)` заново или назвать исключение в комментарии `Start` |
| N-2 | Nit | `internal/state/recovery.go:226-258` (`intactSnapshot`) | Кандидаты отката — все ключи `ListSnapshots` по убыванию `seq`, кроме указателя. Среди них могут быть объекты **новее** указателя: объект записан, PUT `latest.json` не прошёл. Выбирать такой объект безопасно: он проверен хэшем, курсор согласован. Но лог «rebuilt from a snapshot older than latest.json names» тогда неверен, а КД §4.8 говорит о «предыдущем» | Фильтровать `ref.Seq < pointer.Snapshot.Seq` или поправить текст лога и докстроку, упомянув случай в КД через system-architect |

### Открытые вопросы (к оркестратору)

1. **Mi-1 и КД §4.8 (правка T-059).** КД теперь прямо закрепляет распознавание разрыва «по первому офсету, который доставило `ReadRange`». Mi-1 показывает ложный разрыв при записи, отвергнутой на чтении. Исправлять Mi-1 в итерации 2 или вынести в бэклог C-01 — решает system-architect#1.
2. **Порядок слияния T-058 и T-059.** Кто сливается вторым, удаляет свою `CauseInit`. Предлагаю оставить её в `world.go` (T-059): там её читает `initializes`.

### Риски и допущения

- **Вопросы разработчика 1–6 (оценка, не замечания).**
  1. *`log_gap` и повторный факт под первым id* — риск низкий: потребители дедуплицируют по id (C-01). Mi-1 делает ложный `log_gap` чаще, чем нужно.
  2. *Конверт досылки (Mi-3 T-057)* — согласен оставить. Риск низкий-средний: при повторе новым событием трасса `correlation_id` у досланного факта чужая.
  3. *`batch_size`* — низкий.
  4. *`snapshot_event_failed`* — согласен, низкий.
  5. *Число повторов в комментарии* — низкий.
  6. *Страж в `shared/testkit/state`* — низкий: пакет уже доступен тестам `cmd/multiverse` и `test/e2e`, у `guard.go` нет новых зависимостей, depguard 0.
- **Compose: мир `uninitialized` → `degraded` → healthcheck.** Риск средний, касается первого запуска, не данных. `multiverse health` выходит с 1 на `degraded`, поэтому `make up` (`--wait`) на свежем стеке не дождётся `core`. Ни один сервис compose не зависит от `core: service_healthy`, `restart: unless-stopped` на `unhealthy` не перезапускает. Стек работает, `mvctl world init --bus kafka` дойдёт до admin-маршрута, и после init `core` станет `healthy` на следующей пробе. Решение уже принято: КД §19 — проба `multiverse health` принимает `degraded`, правка `cmd/multiverse/health.go` в итерации 2 T-059, `Makefile` и `infrastructure.md` за devops-engineer. Без этой правки слияние в `develop` ломает `make up` на свежем стеке.
- **Без хранилища на kafka** (`--bus=kafka` без ключей MinIO — вне compose, где ключи `[required]`) State после рестарта процесса читает `system_events` с 0 и заново решает весь журнал. Ответы выходят под теми же id, поэтому для потребителей это дубли. Прежняя группа продолжала бы с закоммиченного офсета. Решение уже принято: КД §19 — State без хранилища стартует только над шиной в памяти. Код — итерация 2.
- **kafka и начало журнала за retention.** Распознавание `log_gap` на kafka держится на том, что reader kafka-go переходит к первому офсету. Если вместо этого он вернёт ошибку, `Start` упадёт, и процесс уйдёт в цикл рестартов. Проверить в интеграции Redpanda (бэклог 2).
- **Окно T-059 → T-474.** Предложения `[cursor, End)` без ответа не решаются, пока T-474 не слита (MF это подтвердил: курсор идёт с `End`). Висящая запись досылается только повтором издателя.
- Пробные слияния с T-058 и T-472 сделаны копированием файлов рабочих папок поверх копии T-059. Для `apply.go` — `git merge-file` с базой `0345177`. Lint слитых деревьев не запускал.
- Копии `scratchpad/t059r-copy`, `t059r-m472`, `t059r-m058` и служебные файлы `t059r-*` удалены по точному пути. В рабочей папке T-059 изменена только эта запись. Карточку T-059, `.env`, Docker и стенд `:8888` не трогал.

### Предложения в бэклог

1. **EPIC-001 / C-01 (`contract-change`, system-architect):** «первый офсет журнала» у `Journal` (`Start(ctx, topic)`), membus и kafka одним contract-тестом. Он нужен recovery State (Mi-1) и догону read-model шлюза и роя (C-14).
2. **EPIC-002, интеграция Redpanda:** `ReadRange` с `from` ниже начала журнала после retention — `first ≠ from` без ошибки.
3. **EPIC-002 / T-474:** в DoD добавить тест «рестарт после интента, пережившего пакет, и хода по его сущности» поверх исправления Ma-1, чтобы решение предложений из `[cursor, End)` не упёрлось в тот же roll-forward.

## T-059 · ревью #2 · 2026-09-14 · code-reviewer#4 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-059`, ветка `task/T-059-state-recovery`. В ветке: wip-коммит итерации 1 `cbd38c4`, затем два слияния кончика эпика — T-472 (`b478e98`) и T-058 (`4da16c5`). Изменения итерации 2 не закоммичены (`git diff`): `cmd/multiverse/{contexts_state.go, contexts_state_recovery_test.go, health.go, main_test.go, serve.go}`, `internal/state/{catchup.go, context.go, intent.go, recovery.go, world.go}`, тесты `recovery_test.go`, `recovery_world_test.go`, `resend_test.go`. Правки `Docs/` (карточка, `dev-log.md`) замечанием не считаю — порядок эпика. Входы: ревью #1, раздел карточки «Решения system-architect» (п. 1–4 и «Итерация 2 T-059»), раздел «Итерация 2 (developer)». Решения оркестратора: Mi-1 — в бэклог C-01; вариант А — механизм в `cmd/multiverse`, `runtime.Deps` не меняется. По правилу повторной итерации проверены исправления и регрессия от них.

### Вердикт

**Вернуть.** Blocker (Critical): 0 · Major: 1 · Minor: 1 · Nit: 3.

Ma-1 ревью #1 закрыт: `rollForwardIntent` сначала проверяет весь интент, зонд P1 и «дивергенция без записи» закреплены тестами. Слияние, `log_gap`, проба `health`, Mi-2, Mi-3, N-1, N-2 и решения 2–3 сделаны. Причина возврата — Ma-2. Шина процесса передаётся фабрике State через глобальную переменную. Её можно убрать правкой примерно в 15 строк, это проверено в копии. Кроме того, новое правило roll-forward (`>= to_version` — пропуск) пропускает изменение без правки (`from_version == to_version`) недописанного атомарного пакета, итерация 1 его дописывала (Mi-4, зонд P3).

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Ma-1: `rollForwardIntent` (`recovery.go:666-706`), `finishedIn` (`intent.go:159-167`) | Первый цикл только проверяет: сущности нет → дивергенция, `held.Version >= to_version` → пропуск, `!= from_version` → дивергенция, иначе в очередь. Запись идёт вторым циклом. Ошибка проверки возвращает `0`, записей нет. `finishedIn` — `e.Version >= ToVersion` у каждой сущности. Тесты: `TestAnIntentThatOutlivedItsPackageDoesNotStopTheWorld` — оба варианта зонда P1 (ход дальше `to_version`; ход без изменений под другим `proposal_id`: версия та же, `last_change` переписан — `Commit` не повышает версию при пустом `changed`); мир `ok`, интент удалён, `pending_intents=0`, записей сущностей нет, страж зелёный. `TestAnIntentThatDoesNotFitWritesNothing`: `player-A` подходит и проверяется первым, `wolf-alpha` 5→6 при v1 → `state_divergence`, объект `player-A` v1, интент сохранён. В `TestAWrittenPackageWhoseIntentStayedIsSentByItsRepeat` ход `prop-between` перед повтором проверяет `finishedIn` дальше `to_version`. Доводы докстроки (сущность не могла уйти дальше при недописанном пакете: мир стоит `persist_failed`, roll-forward идёт до приёма предложений) верны. Интенты бывают только у атомарных пакетов из ≥ 2 обновлений (`persist`), создания в них не попадают, удаления сущностей нет — ветка «сущности нет» законно дивергенция. Остаток: изменение без правки — Mi-4; проверка версии после `Commit` во втором цикле — N-5 |
| 2 | Слияние: `CauseInit`, `CanonicalPath`, T-472 | `CauseInit` осталась одна (`bootstrap.go:29`), из `world.go` удалена. `CatchUpChanged` до `pathTokens` зовёт `entity.CanonicalPath` (`catchup.go:49`), неканонический путь → `ErrCorruptFact` → `state_divergence`; векторы `tags.0`, `tags[01]`, `a.+1`. Проверка атрибутов T-472 не обходится. Создания в тестах T-059 идут через `Apply` → `applyCreate` → `entity.CheckAttributes` (`apply.go:314`). Хелпер `create` → `createdBy` дополняет обязательные атрибуты через `withRequired` (`helpers_test.go:251-259`, это T-472), поэтому тесты T-059 проходят без собственных правок. Прямая запись в обход `Apply` есть в `fixture.seed` и в `PutEntity` тестов дивергенции. Это подготовка памяти и хранилища, а не путь создания. Восстановление (догон, сверка, roll-forward) создаёт сущности только из фактов и объектов, которые State уже принял, поэтому повторная проверка атрибутов там не нужна |
| 3 | `log_gap`: отметка вышедших фактов (КД §19 п. 1) | `readFacts` при `first != from` отдаёт собранные факты (`recovery.go:343`). `rebuildFromObjects` их не применяет: объект, который не совпал со снапшотом, получает `SetFactEventID` (ставит `fact_event_id` и `last_event_id`), если ключ `announcement{entity, version, proposal}` совпал с фактом. Объект той же версии под тем же предложением по-прежнему берёт запись снапшота. Тесты: (а) факт в остатке журнала → отметка у `player-A`, повтор не публикует второй факт, страж по остатку журнала; (б) факт до первого сохранённого офсета → `fact_event_id` пуст, повтор публикует ровно один факт под id первого вывода; (в) факт той же версии под `prop-other` → отметки нет, дивергенции нет, мир не `fail`. Подслучай «все офсеты за retention» (`earliest = End`) переписан обоснованно: при `End−1` в остатке законно оставался факт |
| 4 | Проба `multiverse health` (`health.go:23-54`, EPIC-001) | `0` при HTTP 200 и `status ∈ {ok, degraded}`, иначе `1`. Строки `TestRunHealth`: `degraded` → 0, `fail` с 200 → 1, `up` → 1, прежние (`fail` 503, мусор, недоступен) на месте. Процесс и e2e не сломаны. `test/e2e/empty_world_test.go` требует `/health ok` отдельной проверкой тела и затем зовёт пробу: смягчение пробы эту проверку не ослабляет. Healthcheck compose (`/multiverse health`) не менялся и по решению (б) пропускает `degraded`. `make health` (`Makefile:408`) по-прежнему печатает литерал `ok` для `core` по коду 0 — при `degraded` оператор увидит `ok`. Строгий режим — T-476, это риск, а не замечание |
| 5 | Вариант А: глобальная шина сборки (`contexts_state.go:54`, `:66-91`; `serve.go:218`) | Утечки между сборками нет: `defer` сбрасывает значение и при панике фабрики, тест проверяет сброс. Два `serve` в одном процессе мьютекс упорядочивает. e2e запускает отдельные бинарники, так что два `serve` в одном процессе сейчас не встречаются. `go test -p 8` гоняет пакеты отдельными процессами, гонок между пакетами нет. `t.Parallel` в `cmd/multiverse` нет (комментарий `process.run` его запрещает). Гонка не проявлена, но заложена: `busOfTheBuild()` читает `buildBus` без мьютекса. Любой `runtime.New` с `state` вне `contextsOver` (`startBriefly` в `dispatch_test.go:24`, девять тестов с `runtime.New([]string{runtime.All})`) при одновременном `contextsOver` — гонка данных. К тому же такой тест может получить `lawless` вместо `*state.Context`. Правило скрыто зависит от того, откуда вызвана фабрика, и вне `serve` молча не действует (вопрос 2 исполнителя). Убирается малой правкой без C-01 и `runtime.Deps` — Ma-2 |
| 6 | Mi-2, Mi-3, N-1, N-2, решения 2–3 | Mi-2: подслучай `a taken_at its key does not name` (`shiftTakenAt`: `taken_at` +1 с, ключ и хэш прежние) → откат к предыдущему. Mi-3: подслучай `an object at the same version in another state` → `state_divergence`, сообщение сверки объектов. N-1: `Start` с хранилищем создаёт `c.delivered` заново (`context.go:209`), до запуска `Tail`. N-2: кандидаты отката — только `ref.Seq < pointer.Snapshot.Seq` (`recovery.go:237`). Решение 2: `TestAFactSentAgainKeepsItsFirstIDUnderANewEvent` проверяет `timestamp` и `causation_id` первого вывода, `correlation_id` — от повтора. Повтор строится на сдвинутых часах (`again.Timestamp != lost.Timestamp`), так что проверка не тавтологична. Добавленный блок дублирует две проверки строками выше (N-3). Решение 3: `TestAWrittenEntityOfAPackageCutInHalfCarriesTheAppliedBatch` — неатомарный пакет, запись `wolf-alpha` отказала → объект `player-A` v2, `batch_size=2`, `atomic=false`, интента нет. Комментарий `persistPauses` ссылается на КД §4.5 п. 11 |
| 7 | Файлы EPIC-001 (`health.go`, `main_test.go`, `serve.go`, `contexts_state*.go`) | Замечания по коду: `health.go`, `main_test.go` — нет. `serve.go:218` и `contexts_state.go:54-91` — Ma-2. Отметку владельца ставит tech-lead#1 |
| 8 | Зонды и мутанты (копия `scratchpad/t059r2-copy`: tracked и untracked без `Docs/`, `services/`, `.claude/`, `.qwen/`, `.env`; без `-overlay`; контрольный первым) | **C0** — копия без изменений: `internal/state` и `cmd/multiverse` зелёные. **P3** (зонд) — атомарный круг: `player-A` hp −2 и `wolf-alpha` `set hp 10` при hp 10 (`from = to = 1`). Запись `npc/wolf-alpha.json` отказала, `Stop`, рестарт, повтор → `rolled_forward=0`, объект `wolf-alpha` под `prop-w`, у `prop-round` один факт (`player-A`) из двух (Mi-4). То же с условием «`> to`, либо `== to` при `from != to` или том же `proposal_id`» — P3 и четыре теста Ma-1/интентов зелёные. **S1** (набросок Ma-2) — без глобальной переменной: `stateOverBus(bus, names)` в `serve` до `runtime.New`; `go vet` и весь `cmd/multiverse` зелёные, около 15 строк. Копия удалена по точному пути |
| 9 | Прогоны в рабочей папке (go1.26.8, windows/amd64) | `gofmt -l` пусто; `go build ./... && go vet ./... && go vet -tags integration ./internal/state/...` — 0. `go test -short -count=1 ./internal/state/... ./cmd/multiverse/... ./cmd/mvctl/... ./shared/testkit/...` — 18 пакетов ok, «Access is denied» не было, обход для `env` не понадобился. `go test -short -count=3 -race=false -p 8 ./cmd/multiverse/...` — ok (84,7 с). `go test -tags e2e ./test/e2e/...` — ok. `golangci-lint run ./...` — 0 issues. `-race` недоступен (нет cgo), гонка в п. 5 выведена из кода, а не из детектора |

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Ma-2 | Major | `cmd/multiverse/contexts_state.go:54`, `:66-91` (`buildMu`, `buildBus`, `contextsOver`, `busOfTheBuild`); `cmd/multiverse/serve.go:218` | Шина процесса передаётся фабрике State глобальной переменной, которую ставят на время `runtime.New`. (1) `busOfTheBuild()` читает `buildBus` без мьютекса. Правильность держится на том, что все фабрики зовутся синхронно внутри `contextsOver`. Любой `runtime.New` с `state` вне него (`startBriefly`, девять тестов процесса с `runtime.All`), идущий одновременно с `contextsOver`, — гонка данных, и такая сборка может получить `lawless`. Сейчас это не проявляется только потому, что тесты пакета не зовут `t.Parallel`. (2) Правило КД §19 зависит от того, кто вызвал фабрику, и вне `serve` молча не действует (вопрос 2 исполнителя). (3) Проверка конфигурации процесса размазана по двум файлам и глобальному состоянию. Нужна же одна проверка «`--bus` не `memory`, State в `--contexts`, ключей MinIO нет» — всё это `serve` знает до сборки контекстов. Убирается малой правкой без C-01 и `runtime.Deps`, проверено в копии (S1) | Удалить `buildMu`, `buildBus`, `contextsOver`, `busOfTheBuild` и ветку в `newStateWithTheLaws`. В `contexts_state.go` добавить чистую функцию `stateOverBus(bus string, names []string) error`: `nil`, если `bus == busMemory` или среди `names` нет ни `stateContext`, ни `runtime.All`. Если `stateObjects()` вернул клиент или ошибку, тоже `nil`: ошибку назовёт фабрика. Иначе вернуть текущий текст отказа с именами `MV_MINIO_ACCESS_KEY`/`MV_MINIO_SECRET_KEY` и шиной. В `serve` перед `runtime.New(opts.contexts)` написать `if err := stateOverBus(opts.bus, opts.contexts); err != nil { return err }`. Для оператора это то же: процесс не стартует, а с отказом `Start` он тоже выходит (`process.run` возвращает ошибку `StartAll`). Тест `TestStateWithoutAStoreRunsOnlyOverTheMemoryBus` превращается в таблицу: kafka без ключей → отказ с обеими переменными; `memory` → `nil`; kafka с ключами → `nil`; kafka без `state` в `--contexts` → `nil`; `all` → отказ. Мутант «проверка не вызвана в `serve`» ловится тестом `serve` над kafka без ключей: ошибка до открытия шины |
| Mi-4 | Minor | `internal/state/recovery.go:674`; `internal/state/intent.go:162` | Новое правило «`held.Version >= to_version` — пропуск» верно для изменения, повышающего версию. Изменение без правки (`from_version == to_version`) оно не различает: такая сущность «уже на `to_version`», записана она или нет. В итерации 1 такая сущность дописывалась, потому что `proposal_id` не совпадал. Атомарный пакет, прерванный до записи сущности без правки, после рестарта не дописывается. У неё остаётся прежний `last_change` без записи истории `prop-round`, а повтор публикует факты только записанных сущностей — атомарный пакет объявлен наполовину (§4.7). По `batch_size=2` при одной сущности с этим `proposal_id` аудит увидит частичную запись атомарного пакета. Зонд P3 это подтвердил. `finishedIn` устроен так же. Атрибуты и версии не страдают, случай требует совпадения ход без правки + обрыв ровно перед этой сущностью, поэтому Minor. Задача и так возвращается — исправить в итерации 3 | В первом цикле `rollForwardIntent` и в `finishedIn` для `change.FromVersion == change.ToVersion` считать сущность записанной, только если `proposalOf(held) == in.ProposalID` или в `held.History` есть запись с `ProposalID == in.ProposalID` (история ≤ 50, а 50 более поздних ходов по сущности возможны только после целиком записанного пакета). Иначе при `held.Version == FromVersion` — дописать. Для `from < to` правило оставить как есть. Тесты: сценарий P3 (`wolf-alpha` дописан под `prop-round`, у повтора два факта) и вариант P1 «пакет записан целиком, затем ход без изменений по сущности без правки под другим `proposal_id`» (не переписывается, мир `ok`). Мутант «`>= to` для всех изменений» краснеет на P3 |
| N-3 | Nit | `internal/state/resend_test.go:277-280` | Добавленный блок повторяет проверки строк 270 (`fact.Timestamp.Equal(lost.Timestamp)`) и 273 (`fact.Meta.CausationID != lost.ID`). Дважды упадёт одно и то же | Удалить блок или перенести ссылку на КД §4.5 п. 2 в сообщения существующих проверок |
| N-4 | Nit | `internal/state/dedup.go:137-140` (докстрока `refuseUnfinishedPackage`) | «An intent whose every entity is at its to_version under this proposal is not unfinished» — правило итерации 1. `finishedIn` теперь «на `to_version` или дальше», без `proposal_id` | Привести к правилу `finishedIn` (и к исправлению Mi-4) |
| N-5 | Nit | `internal/state/recovery.go:686-692` | Проверка `held.Version != change.ToVersion` после `Commit` стоит во втором (пишущем) цикле. Интент, чьи `changed` не дают `to_version` (например, пустой `changed` при `from < to`), обнаружится уже после записи предыдущих сущностей того же интента. Это против докстроки «no entity of the intent is written». Интент пишет сам State, поэтому только Nit | Делать `Commit` и сверку версии в первом цикле (`held` — уже копия из `store.Get`), во втором только `PutEntity` и `store.Put` |

### Открытые вопросы (к оркестратору)

1. **Ma-2 и вопрос 2 исполнителя.** С `stateOverBus` в `serve` вопрос о сборке вне `serve` снимается: правило становится проверкой конфигурации процесса, а фабрика от шины не зависит. Подтверждение tech-lead#1 нужно только для места вызова в `serve.go` (файл EPIC-001).

### Риски и допущения

- **`make health` до T-476.** `Makefile:408` печатает `core ok` по коду пробы 0, а теперь это и `degraded`. Сигнал «выполните `mvctl world init`» в таблице пропадает, пока devops-engineer не сделает строгий режим (T-476, раздел system-architect «Пункт для devops-engineer»). На данные не влияет.
- **Гонка Ma-2** выведена из кода. Детектор `-race` в этом окружении недоступен (нет cgo), параллельный прогон `-p 8 -count=3` зелёный, потому что внутри пакета тесты последовательны.
- **Mi-1 (ложный `log_gap`)** по решению оркестратора в бэклоге C-01. Отметка фактов при разрыве (п. 3) ложный разрыв не усугубляет: при ложном разрыве остаток журнала полный, и объекты впереди снапшота получат id своих фактов. Это даже уменьшает число вторых публикаций, отмеченное в Mi-1.
- Набросок S1 и зонд P3 делались в копии и в рабочую папку не переносились. Копия `scratchpad/t059r2-copy` удалена по точному пути. В `.worktrees/T-059` изменены только эта запись и строка ревью в карточке T-059. `.env`, Docker, стенд `:8888`, `.claude/*`, `.mcp.json`, `.qwen/*`, `Docs/user-stories/` не трогал.

### Предложения в бэклог

1. **EPIC-002 / T-474:** в DoD — рестарт после атомарного пакета с изменением без правки, прерванного перед этой сущностью, поверх исправления Mi-4. Решение предложений из `[cursor, End)` не должно упереться в тот же roll-forward.
2. **EPIC-001 (`shared/runtime`):** если фабрикам понадобятся параметры процесса (шина, режим) не только для State, завести `Registry.NewWith(names, params)`, а не глобальные переменные. Это через system-architect (`contract-change` C-01), для T-059 не нужно.
3. Из ревью #1 остаются в силе: «первый офсет журнала» у `Journal` (Mi-1) и интеграция Redpanda `ReadRange` ниже начала журнала.

## T-059 · ревью #3 · 2026-09-14 · code-reviewer#4 (TEAM-1)

### Границы ревью

Папка `.worktrees/T-059`, ветка `task/T-059-state-recovery`. HEAD `4da16c5`, как в ревью #2. Изменения итераций 2 и 3 не закоммичены (`git diff`), новых коммитов нет. Входы: ревью #2 («вернуть», 0/1/1/3) и раздел карточки «Итерация 3 (developer)». Это повторная итерация, поэтому проверены исправления Ma-2, Mi-4, N-3, N-4, N-5 и регрессия от них. Остальное принято в ревью #2 и заново не разбиралось. Правки `Docs/` (карточка, `dev-log.md`) замечанием не считаю — таков порядок эпика.

### Вердикт

**Принять.** Blocker (Critical): 0 · Major: 0 · Minor: 0 · Nit: 0.

- Ma-2 закрыт так, как предлагало ревью #2: глобальной шины сборки нет, проверка конфигурации стоит в `serve` до `runtime.New`, фабрика State от шины не зависит.
- Mi-4 закрыт общим предикатом `writtenBy`. Зонд P3 теперь дописывает сущность, а пакет, записанный целиком, после хода без изменений не переписывается.
- N-3…N-5 сделаны. Прогоны не хуже итерации 2, мутанты ревьюера красные.
- Ограничение истории 50 записями на `StateHash` и догон не влияет. Остаток — в «Рисках» и в бэклоге.

### Проверено

| # | Пункт | Результат |
|---|---|---|
| 1 | Ma-2: глобальная переменная удалена | `git grep -E "buildMu\|buildBus\|contextsOver\|busOfTheBuild"` по `*.go` (tracked и untracked) пуст. Совпадения остались только в `Docs/` — это история итерации 2. Ветки отказа в `newStateWithTheLaws` нет: фабрика строит `lawless` только при ошибке книги правил или клиента хранилища |
| 2 | Ma-2: `stateOverBus` (`contexts_state.go:72`) и вызов в `serve` (`serve.go:218`) | Чистая функция. Возвращает `nil`, если шина `memory`, если в `names` нет ни `state`, ни `all`, или если `stateObjects()` вернул клиент либо ошибку (ошибку назовёт фабрика, отказ `Start`). Иначе — отказ: шина, `MV_MINIO_ACCESS_KEY`, `MV_MINIO_SECRET_KEY`, значений нет. `objstore.New` к серверу не подключается (`minio.New` только строит клиент), поэтому второй вызов `stateObjects()` в фабрике не даёт побочных эффектов. В `serve` проверка стоит первой строкой, до `runtime.New` и до `process.run`. `runtime.New` не добавляет зависимости сам (`DependsOn` вне набора игнорируется), поэтому State не попадёт в процесс в обход `names`. `--bus=memory` парсер и так пропускает только с `--contexts=all` (`serve.go:159`) |
| 3 | Ma-2: обходные пути запуска в production | `runtime.New` в production-коде вызывается ровно в одном месте — `serve.go:221`. Подкоманды `cmd/multiverse`: `serve` и форма без подкоманды идут через `dispatch(…, serve)` (`main.go:31`). `health`, `db`, `version` контексты не строят. `startBriefly` (`dispatch_test.go:20`) — тестовый код, и прямой `runtime.New` там законен: правило теперь относится к конфигурации процесса, а не к фабрике. Других `process{…}` в production нет |
| 4 | Ma-2: тесты | `TestStateWithoutAStoreRunsOnlyOverTheMemoryBus` (`contexts_state_recovery_test.go:135`) — таблица из шести случаев: kafka без ключей, `all` над kafka и `state` среди других над kafka дают отказ, в тексте обе переменные, шина и «memory bus only». Память, kafka с ключами и kafka без `state` проходят. В конце проверено, что фабрика вне `serve` даёт `*state.Context`. `TestServeRefusesStateWithoutAStoreOverKafka` (`:188`) проверяет, что `serve` над kafka (`127.0.0.1:1`) с `state` без ключей возвращает отказ. Мутант A ревьюера совпадает с S1 исполнителя (проверка не вызывается) и краснеет: без проверки `serve` не завершается за 10 с. S2 («`all` — не state») ловит строка `all over kafka without keys`, это видно из таблицы |
| 5 | Mi-4: `writtenBy` (`recovery.go:715`), `rollForwardIntent` (`:663`), `finishedIn` (`intent.go:158`) | Если изменение повышает версию, пакет записан при `version >= to_version`, как в итерации 2. Если изменения без правки (`from = to`): записан, когда сущность ушла дальше `to_version`, когда `last_change.proposal_id` называет предложение интента или когда его называет запись истории (при `version >= to`). Иначе при `version == from` сущность дописывается, при `version < from` — дивергенция. Ложного «записано» нет. Факт пакета публикуется только после всех записей, поэтому `prop-round` попадает в историю сущности в памяти восстановленного мира (снапшот, догон, объекты, `log_gap`) только с записанного объекта. Одно предложение затрагивает сущность один раз (`repeatedEntity`, `apply.go:381`). Интенты бывают только у атомарных пакетов |
| 6 | Mi-4: тесты | `TestAChangeWithoutAChangeOfACutPackageIsRolledForward` (P3): `rolled_forward=1`, объект `wolf-alpha` v1 под `prop-round`, повтор даёт по одному факту на сущность, мир `ok`, страж. `TestAWrittenChangeWithoutAChangeIsNotRolledForwardAgain`: пакет записан целиком, интент остался, затем ход `prop-later` без изменений по `wolf-alpha`. Рестарт ничего не пишет, `prop-later` сохранён, интент удалён. `TestARepeatOfAPackageCutBeforeItsChangeWithoutAChangeSendsNoFact`: повтор без восстановления над объектами — фактов нет, `persist_failed`. Мутант B ревьюера («история — любая запись, а не предложение интента», `h.ProposalID != ""`) краснеет на P3: `rolled_forward=0`, объект под `prop-w`. Этот мутант не совпадает с W1–W3 исполнителя |
| 7 | Mi-4: риск ограничения истории 50 записями | Когда риск срабатывает: `DeleteIntent` отказал после всех повторов (мир идёт, `pending_intents=1`), затем по сущности с изменением без правки прошло больше 50 ходов без изменения версии, затем рестарт. Тогда `writtenBy` даёт `false` при `version == from`, и сущность дописывается из интента. Чем это грозит: **`StateHash` не меняется.** `CanonicalJSON` берёт только `id`, `type`, `version`, `attributes` (`hash.go:28`). Атрибуты не могли измениться без роста версии, поэтому `attributes_after` интента равен текущим атрибутам. **Сверка и догон не ломаются.** На следующем рестарте без нового снапшота `reconcile` видит ту же версию, тот же хэш и другое предложение и берёт объект (`recovery.go:581`), дивергенции нет. Факты без изменений догон проверяет по id (`announced`), а `announcerOf` берёт первую запись версии, которую дописанная в хвост запись не вытесняет. Кроме переписанного `last_change` есть ещё три следа. (1) `updated_at` откатывается к `applied_at` интента. (2) В хвост истории встаёт запись с более ранним `at`. (3) `last_change.fact_event_id` пуст, и повтор `prop-round`, если он придёт, дошлёт факт `wolf-alpha` под первым id, а потребители погасят его по id. Для данных и восстановления риск низкий, замечанием не считаю. Точное условие без истории — в бэклоге |
| 8 | N-3 (`resend_test.go:275`) | Повторный блок итерации 2 удалён. Ссылка на КД §4.5 п. 2 теперь в сообщении существующей проверки `cause`/`causation_id`. Проверки `timestamp`, `causation_id`, `correlation_id` остались по одной |
| 9 | N-4 (`dedup.go:137`) | Докстрока `refuseUnfinishedPackage` описывает правило `finishedIn`/`writtenBy`: `to_version` или дальше, для изменения без правки — предложение в записи коммита или истории. Мир мог изменить сущности после пакета |
| 10 | N-5 (`recovery.go:663-700`) | `Commit` и сверка `to_version` перенесены в первый цикл, `held` — копия (`memstore.Get` возвращает `entity.Clone`, `memstore.go:46`). До записи память не меняется. Второй цикл только пишет (`PutEntity`, `store.Put`) в том же порядке и теми же значениями, `rolled` на ошибке записи тот же. Для корректного интента поведение не изменилось. Некорректный интент (`changed` не даёт `to_version`) теперь отказывает до первой записи, как и обещает докстрока. Докстрока `rollForwardIntent` добавлена |
| 11 | Мутанты ревьюера (копия `scratchpad/t059r3-copy`: tracked и untracked без `Docs/`, `services/`, `.claude/`, `.qwen/`, `.env`; без `-overlay`) | **C0** первым: копия без изменений, 10 тестов Ma-2/Mi-4/интентов в `cmd/multiverse` и `internal/state` зелёные. **A** — удалён вызов `stateOverBus` в `serve`: `TestServeRefusesStateWithoutAStoreOverKafka` красный. **B** — `writtenBy` принимает любую запись истории: `TestAChangeWithoutAChangeOfACutPackageIsRolledForward` красный. Копия и служебные скрипты `t059r3-*` удалены по точному пути |
| 12 | Прогоны в рабочей папке (go1.26.8, windows/amd64), сравнение с итерацией 2 | `gofmt -l cmd internal shared` пусто. `go build ./... && go vet ./... && go vet -tags integration ./internal/state/...` — 0 (как в итерации 2). `go test -short -count=1 ./internal/state/... ./cmd/multiverse/... ./cmd/mvctl/... ./shared/testkit/...` — 18 пакетов ok. В этот раз `cmd/mvctl/internal/env` упал с «Access is denied» на `env.test.exe`, это класс `updates`, а не регрессия. Обход `go test -c -o scratchpad/t059r3_envcheck.exe` — PASS, exe удалён по точному пути. `go test -short -count=3 -p 8 ./cmd/multiverse/...` — ok, 99,4 с (в итерации 2 было 84,7 с). Рост — два новых теста, в том числе `serve` над kafka, и загрузка машины. `golangci-lint run ./...` — 0 issues (как в итерации 2). e2e не запускал: вне списка прогонов ревью #3, итерация 3 e2e-пути не трогает |

### Замечания

Нет.

### Открытые вопросы (к оркестратору)

Нет. Отметка tech-lead#1 для `cmd/multiverse/{serve.go, contexts_state.go, contexts_state_recovery_test.go, health.go, main_test.go}` по-прежнему нужна на приёмке. Это вопрос владения, а не ревью.

### Риски и допущения

- **История 50 записей (п. 7).** Интент переживает пакет, а по сущности с изменением без правки проходит больше 50 ходов без роста версии — тогда roll-forward переписывает `last_change`, откатывает `updated_at`, дописывает в хвост истории запись с более ранним `at`, `fact_event_id` остаётся пуст. `StateHash`, сверка и догон не страдают. Повтор `prop-round` может дослать факт под первым id.
- **`make health` до T-476** — как в ревью #2: `core ok` печатается и при `degraded`.
- Мутант A краснеет по таймауту 10 с, а не по тексту ошибки. Без проверки `serve` над недоступным брокером не завершается. Порядок «до открытия шины» тест не проверяет. Для правила это не важно: отказ `serve` при любом порядке тот же.
- `-race` в окружении недоступен (нет cgo). После Ma-2 глобального состояния сборки нет, так что гонка ревью #2 снята по построению.
- В `.worktrees/T-059` изменены только эта запись и строка ревью в карточке T-059. `.env`, Docker, стенд `:8888`, `.claude/*`, `.mcp.json`, `.qwen/*`, `Docs/user-stories/` не трогал.

### Предложения в бэклог

1. **EPIC-002 (`internal/state`, после T-059):** точное условие «пакет записан» для изменения без правки, не зависящее от длины истории. Например, при полной истории (`len == HistoryLimit`) без записи предложения считать пакет записанным, если `at` самой старой записи позже `applied_at` интента. Или хранить `proposal_id` интента в объекте вне истории. Приоритет низкий (п. 7).
2. Из ревью #2 в силе: T-474 — рестарт после атомарного пакета с изменением без правки поверх Mi-4. EPIC-001 / C-01 — «первый офсет журнала» (Mi-1) и интеграция Redpanda `ReadRange` ниже начала журнала.
