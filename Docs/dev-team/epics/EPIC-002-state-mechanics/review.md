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
