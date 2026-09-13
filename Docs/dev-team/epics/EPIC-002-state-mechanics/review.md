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
