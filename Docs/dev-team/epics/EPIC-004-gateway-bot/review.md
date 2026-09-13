# Ревью EPIC-004

Формат записи: задача, номер итерации, дата, экземпляр ревьюера; границы ревью; вердикт;
проверенные пункты DoD; замечания с серьёзностью (Critical / Major / Minor / Nit) в виде
«файл:строка — суть — предлагаемая правка». Записи добавляются, старые не удаляются.
Язык — русский; код, конфиги и команды — английские.

---

<!-- review T-302 #1 -->
## T-302 · ревью #1 · 2026-09-13 · code-reviewer#1 (TEAM-3)

### Границы ревью

Рабочая папка `.worktrees/T-302`, ветка `task/T-302-store-migrations`, HEAD `1c2ee7e`. Коммита задачи нет:
ревьюировались **незакоммиченные изменения** (`git status --porcelain --untracked-files=all`, 14 путей):
`M go.mod`, `M go.sum`; новые `internal/gateway/migrations/{embed.go,embed_test.go,links/0001_init.sql,gateway/0001_init.sql}`,
`internal/gateway/store/{open.go,migrate.go,compact.go,retention.go,helpers_test.go,open_test.go,migrate_test.go,compact_test.go}`,
`Docs/dev-team/epics/EPIC-004-gateway-bot/{tasks/T-302.md,dev-log.md}`. Посторонних изменений в папке нет.

Карточка и `dev-log.md` в ветке задачи — по правилу раздела T-302 в `tasks.md` эпика
(«Карточку `tasks/T-302.md` и `dev-log.md` ведёт исполнитель в ветке задачи»), замечанием не считаются.

Ветка задачи отстаёт от `epic/EPIC-004-gateway-bot` на один коммит документов (`53f1ea2`: `design.md`, `tasks.md`,
карточки T-303…T-393). Пересечений путей с изменениями T-302 нет, конфликта при слиянии не будет. Раздел T-302 в
`tasks.md` («сверка 2026-09-13») сверялся по ветке эпика (`git show epic/EPIC-004-gateway-bot:…/tasks.md`).

Основание: `tasks.md` §T-302 (сверка 2026-09-13), карточка T-302, `components/gateway-and-bot.md` §3, §4.1, §4.2
и «Дополнение после сведения 3» (З-1), ADR-019 с дополнением п. 1–4, `contracts.md` C-10 v1.1 (C-14 задачу не касается),
`analysis/data-model.md` §5, `threat-model.md` (SEC-03/04/05, §4.8).

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 1 · Nit: 6.

Схемы обеих БД совпадают с физической моделью component §4.1/§4.2 колонка в колонку: те же типы, CHECK, индексы,
`WITHOUT ROWID`, каскад `character_requests`. CHECK `end_reason` содержит `forget` (З-1, C-10 v1.1). PRAGMA применяются
и затем читаются обратно. `CompactLinks` доказуемо стирает ID из файла и WAL. goose работает без глобального
состояния. `go.mod` чистый: `tidy -diff` пуст, `verify` проходит, лишних драйверов в графе сборки нет, лицензии допустимы.
Единственное Minor-замечание относится к непроверяемости логики прав на Windows. Сама логика верна (разбор ниже),
и её проверит Linux-джоб CI.

### Проверено ревьюером (только чтение; прогоны на go1.26.8 windows/amd64)

| Проверка | Как | Результат |
|---|---|---|
| сборка и vet | `go build ./... && go vet ./...`; `GOOS=linux go vet ./internal/gateway/...` | 0 / 0 — тесты прав компилируются под Linux |
| unit-тесты gateway | `go test -short -count=1 -v ./internal/gateway/...` | `migrations` ok; `store` ok: 11 PASS, 2 SKIP (права, Windows) |
| линтер | `golangci-lint run ./internal/gateway/...` и `./...` (2.13.2) | 0 issues |
| `make test` | весь модуль, без `-race` (нет cgo) | exit 0, 29 пакетов ok; `store` 81,1 %, `migrations` 83,3 % |
| `make vuln` | govulncheck | 0 вызываемых уязвимостей; 3 — в требуемых модулях, не вызываются (`x/crypto v0.55.0`, до задачи) |
| `go mod verify` / `go mod tidy -diff` | как в CI (`go.yml` job `unit`) | all modules verified / пусто |
| граф сборки платформы | `go list -deps ./cmd/multiverse ./cmd/mvctl` по `goose|sqlite|modernc|pgx|pq|mysql|clickhouse|mssql|vertica|ydb|libsql|turso|duckdb` | **пусто**: `store` в бинарники пока не подключён, размер `multiverse` не меняется |
| граф `internal/gateway/store` | `go list -deps` (не std) | только `goose/v3` (core, `database`, `lock`, `internal/*`), `interpolate`, `multierr`, `x/sync/errgroup`, `go-retry`, `modernc.org/{sqlite,libc,mathutil,memory}`, `bigfft`, `go-strftime`, `go-isatty`, `go-humanize`. **Драйверов других СУБД нет** |
| обязательность подъёмов | `go.mod` goose v3.28.0 из кэша модулей | требует `go 1.26.0`, `lz4/v4 v4.1.29`, `otel* v1.46.0`, `otelhttp v0.71.0`, `go-connections v0.8.1`, `moby/client v0.5.1`, `testify v1.12.1`, `logr v1.4.4`, `multierr v1.11.0` — подъёмы по MVS вынуждены, исполнитель прав |
| лицензии | LICENSE в кэше модулей | goose MIT; `modernc.org/{sqlite,libc,mathutil,memory}` BSD-3; `bigfft`, `x/sync` BSD-3; `interpolate`, `go-strftime`, `go-isatty`, `multierr`, `go-humanize` MIT; `go-retry` Apache-2.0 — все допустимы |
| `go 1.26` → `go 1.26.0` | `build/versions.env` (`GO_VERSION=1.26.8`), `build/Dockerfile` (`golang:${GO_VERSION}`), `.github/workflows/go.yml` (`setup-go go-version-file: go.mod`, сверка `toolchain` ↔ `GO_VERSION`) | согласовано: `toolchain go1.26.8` не менялся, CI сверяет именно его; скриптов и тестов, разбирающих строку `go 1.26`, в дереве нет |
| lz4 в бинарнике | `go list -deps ./cmd/multiverse` | `pierrec/lz4/v4` входит через `kafka-go/compress/lz4` (4.1.15 → 4.1.29, патч-релизы v4); `otel`/`moby`/`go-connections` — только тестовый граф testcontainers |
| DSN-ключи драйвера | `modernc.org/sqlite@v1.58.0/sqlite.go` `applyQueryParams` | `_busy_timeout`, `_auto_vacuum`, `_foreign_keys`, `_journal_mode`, `_synchronous` поддерживаются. Значения проверяются до применения, опечатка не пройдёт. Порядок применения: `busy_timeout` → `auto_vacuum` → `_pragma` → `foreign_keys` → `journal_mode` → `synchronous`. Утверждение исполнителя верно |
| логирование goose | `provider.go`, `provider_run.go` v3.28.0 | `logf` пишет только при `WithVerbose(true)`, по умолчанию молчит: stdlib `log` в процесс не попадает |
| глобальное состояние goose | `migrate.go:35` | `goose.NewProvider(…, WithDisableGlobalRegistry(true))`, без `SetBaseFS`/`SetDialect`; стор и конфиг у каждого `Provider` свои, две БД в одном процессе не мешают друг другу |
| forbidigo | чтение кода + линтер | `os.Getenv`/`time.Now`/таймеров/`log.*` в production-коде нет; `MV_GATEWAY_DATA_DIR` читается через `env.GatewayDataDir.StringFrom`; `retention.go` использует только `time.Duration`/`time.Time` как типы; `time.Sleep` в `helpers_test.go:30` — тест, не запрещён |
| колонка `cursors.offset` (ключевое слово SQLite) | зонд в копии модуля в scratch: `CREATE`, `INSERT … ON CONFLICT DO UPDATE SET offset = …`, `UPDATE … SET offset = offset + 1`, `SELECT offset … LIMIT 1 OFFSET 0` | всё работает (OFFSET — fallback-идентификатор); будущему consumer'у переименование не нужно. Копия удалена по сохранённому точному пути |

### Разбор по пунктам поручения

**1. Схемы БД.** `links/0001_init.sql` и `gateway/0001_init.sql` сверены построчно с component §4.1/§4.2:
таблицы, типы, `NOT NULL`, CHECK-списки, `PRIMARY KEY`, `WITHOUT ROWID`, частичные уникальные индексы
(`ux_links_player`, `ux_sessions_active`, `ux_rounds_open`, `ix_deliveries_lease`), `ON DELETE CASCADE`
у `character_requests` и `REFERENCES sessions(id)` у `turns` без каскада — совпадают. `sessions.end_reason` —
`IN ('leave','idle','death','error','forget')` (`gateway/0001_init.sql:29`) по З-1/C-10 v1.1. Основной текст §4.2
ещё без `forget`, исполнитель это отметил. `abandoned` в gateway не хранится, об этом комментарий в шапке (`:10-11`).
`group_participation` — `scope_id` + `participation`, как в component §4.2. В `data-model.md` §5.4 — `group_id` и без
`participation`; физическая схема главнее, документ отстаёт (бэклог). ПДн сверх allowlist SEC-03 нет: в `gateway.db` нет
`external*`, `link_id`, `chat_id`, `username`. `turns.player_name`, `pending_characters.name` — игровые имена персонажей,
по threat-model §4.2 («Утечка ПДн: карта») это не внешние ID. Allowlist-тест `migrate_test.go:93-116` проверяет точный
порядок колонок и отдельно, независимо от списка, подстроку `external` и `link_id`.

**2. Открытие.** PRAGMA в DSN применяются к каждому новому соединению и после открытия читаются обратно
(`open.go:148-172`): `journal_mode`, `synchronous`, `foreign_keys`, `busy_timeout`, а для `links.db` ещё
`auto_vacuum=2` и `secure_delete=1`. Тест открывает файл повторно и проверяет PRAGMA снова. Файл без
`auto_vacuum` отвергается (`TestOpenLinksRefusesAFileWithoutIncrementalVacuum`). Одно соединение:
`SetMaxOpenConns(1)`, `SetMaxIdleConns(1)`, lifetime и idle-time 0 — соединение не пересоздаётся, per-connection
PRAGMA не теряются. Последствия для конкурентного доступа совпадают с ADR-019 («нет параллельных чтений»). Будущим
пакетам важно: внутри `BeginTx` нельзя обращаться к `*sql.DB` мимо `tx` — это дедлок. `busy_timeout` действует только
против внешних процессов, например `sqlite3 .backup` из ADR-019 п. 8. Эти риски относятся к T-303 и далее, в T-302
не проявляются. Права: логику `checkMode` (`open.go:209-220`) разобрал вручную. `got.Perm() &^ limit != 0` отвергает
`0755` у каталога и `0644` у файла; `0700`/`0600` и более узкие пропускает. `MkdirAll(0700)` при umask 022 даёт `0700`.
`OpenFile(…, 0600)` создаёт файл до SQLite. SQLite (unix VFS, `findCreateFileMode`, `unixOpenSharedMemory`) создаёт
`-wal`/`-shm` с правами самого файла БД, поэтому ожидание `0600` у `-wal`/`-shm` в `open_test.go:164` верно.
Тесты `TestOpenCreatesThePrivateDirectoryAndFiles` и `TestOpenRefusesWiderModes` на Linux проверят то, что заявлено.
Пропуск на Windows дефект логики не маскирует: production-код на Windows проверку не выполняет, и тест проверял бы
пустое место. Но до первого прогона в CI логика прав не исполнялась ни разу (Mi-1).

**3. `CompactLinks`: порядок.** Обоснование исполнителя верно. В WAL-режиме `DELETE` при `secure_delete=ON` пишет
обнулённую страницу в WAL, а в самом файле остаётся старая. `incremental_vacuum` тоже пишет в WAL: перенос страниц
и новый размер БД в commit-кадре. Если `wal_checkpoint(TRUNCATE)` идёт последним, одним проходом переносятся все кадры,
файл усекается до нового размера, а WAL — до 0 байт. При обратном порядке (ADR-019 доп. п. 1) ID тоже стирается, так
как стирание обеспечивает `secure_delete`, но в WAL остаются кадры vacuum. Мутант O1 исполнителя: 4152 байта WAL.
Требование «WAL = 0» тогда не выполняется. `busy != 0` → ошибка (`compact.go:37-39`) закрывает случай, когда
checkpoint молча блокирует читатель. Доказательность теста: реальные файлы во временном каталоге ОС, реальные страницы
B-дерева (ID делит лист с остающимися строками, поэтому вычищает его именно `secure_delete`, а не освобождение
страницы). Первый контроль: ID есть в самом `links.db` после вставки и checkpoint. Затем свободные страницы есть,
после — `freelist_count = 0`, WAL 0 байт, каскад сработал. Мутанты исполнителя V1/C1/B1/O1/S1/K1 красные.
Слабый второй контроль — Nit N-3.

**4. Миграции.** goose `Provider` на `fs.Sub` одного `embed.FS`, у каждой БД своя `goose_db_version`. Повторный `Up`
возвращает 0, в том числе после переоткрытия файла; версия 1 (`migrate_test.go:18-62`). `Provider.Close` не вызывается,
иначе он закрыл бы чужой `*sql.DB`. Глобальный реестр отключён, `SetBaseFS`/`SetDialect`/`SetLogger` не используются.

**5. `go.mod`.** См. таблицу выше. Две прямые зависимости — пины ADR-019 п. 1–2. Все косвенные подъёмы вынуждены
`go.mod` goose; драйверы Postgres/MySQL/ClickHouse и прочие остаются в `go.mod` goose, но из-за module graph pruning
не попадают ни в граф пакетов, ни в `go.sum`. `go 1.26.0` согласован с `GO_VERSION=1.26.8`, Dockerfile и CI.
Неточность в отчёте — Nit N-5.

**6. forbidigo.** Нарушений нет.

**7. Прогоны.** См. таблицу. Все зелёные.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Mi-1 | Minor | `internal/gateway/store/open.go:209-220`; `internal/gateway/store/open_test.go:156-212` | Проверка прав каталога и файла — требование ADR-019 п. 1 («ошибка старта, если права шире»). На платформе разработки она не исполнялась: оба теста пропускаются, а `checkMode` на Windows сразу возвращает `nil`. Мутанты по правам не гонялись. Первый запуск этой логики будет в Linux-джобе CI (`-race`), вместе с первым прогоном `modernc.org/sqlite` под `-race`/`checkptr`. По чтению логика верна, но регрессия (`&^` → `&`, `!= 0` → `== 0`) локально не ловится. | Вынести сравнение в чистую функцию без `runtime.GOOS`, например `wider(got, limit fs.FileMode) bool`. `checkMode` вызывает её на не-Windows. Табличный тест (`0700/0700` ок, `0750/0700`, `0755/0700`, `0644/0600` отказ, `0400/0600` ок) идёт на всех ОС. Тесты с реальными файлами оставить со skip. Приёмку T-302 фиксировать после зелёного job `unit` в CI. | открыто |
| N-1 | Nit | `internal/gateway/migrations/links/0001_init.sql:3-5` | Комментарий «/forget checkpoints the WAL and vacuums» описывает порядок, обратный коду (`compact.go:15-20`: сначала vacuum, потом checkpoint). T-303 будет читать и SQL, и component §4.1 с тем же обратным порядком. | Переписать: «/forget vacuums and then checkpoints the WAL (store.CompactLinks)». Правка component §4.1 и ADR-019 доп. п. 1 — у architect#3 (бэклог 2). | открыто |
| N-2 | Nit | `internal/gateway/store/migrate_test.go:124` | `m.name NOT LIKE 'sqlite_%'`: `_` в LIKE — любой символ, поэтому таблица с именем вроде `sqliteX…` выпадет из allowlist-проверки SEC-03 незамеченной. Сейчас таких нет, но это дыра в сторожевом тесте. | `substr(m.name, 1, 7) <> 'sqlite_'` или `NOT LIKE 'sqlite\_%' ESCAPE '\'`. | открыто |
| N-3 | Nit | `internal/gateway/store/compact_test.go:70-72` | Контроль «the DELETE alone left no trace» следует из первого контроля (`:47-49`): ID уже лежит в самом `links.db`, а до checkpoint файл не меняется. Новой информации контроль не даёт. | Проверять конкретно файл: `occurrences(t, path) > 0` после DELETE, то есть одного `secure_delete` без checkpoint мало. Или убрать контроль, оставив `freelist_count`. | открыто |
| N-4 | Nit | `Docs/dev-team/epics/EPIC-004-gateway-bot/tasks/T-302.md:33, 112-119`; `internal/gateway/store/open.go:127-142` | ADR-019 п. 1 задаёт DSN в виде `_pragma=journal_mode(WAL)&…`. Реализация берёт короткие ключи драйвера (`_journal_mode`, `_auto_vacuum`, …). Решение хорошее: драйвер проверяет значения до применения. Но отклонение описано только в dev-log («Решения по ходу»), а в разделе «Отклонения от дизайна и задачи» его нет. | Добавить строку в «Отклонения» карточки и в пункт бэклога 2 (правка ADR-019 п. 1). | открыто |
| N-5 | Nit | `Docs/dev-team/epics/EPIC-004-gateway-bot/tasks/T-302.md:87`; `Docs/dev-team/epics/EPIC-004-gateway-bot/dev-log.md:40` | «`go.uber.org/multierr` 1.10.0→1.11.0»: до задачи `multierr` не было ни в `go.mod`, ни в `go.sum` (в diff только добавление), это новая косвенная зависимость. Также не упомянуты записи, добавленные только в `go.sum` (`google/pprof`, `hashicorp/golang-lru/v2`, `x/mod`, `x/tools`, `modernc.org/{cc,ccgo,gc,goabi0,fileutil,opt,sortutil,strutil,token}`). Это хэши `go.mod` из графа `modernc.org/sqlite`/`libc`, в сборку они не входят. | Исправить формулировку и одной строкой перечислить записи, добавленные только в `go.sum`. | открыто |
| N-6 | Nit | `internal/gateway/store/retention.go:8-9, 22-24` | Шапка блока ссылается на «ADR-019 p. 7», а `LinksCompactInterval` (страховка раз в час) — это ADR-019 доп. п. 1 / п. 5, а не п. 7 (retention). | Дописать ссылку у `LinksCompactInterval`: «ADR-019 addendum p. 1». | открыто |

### Предложения в бэклог (вне границ T-302)

1. **system-analyst — `data-model.md` §5.** Преамбула §5 и §5.1 говорят, что `link_id` хранится и в `gateway.db`, а ADR-019 доп. п. 2, component §4.2 и SEC-03 это запрещают, и тест T-302 тоже. В §5.4 `GroupParticipation.group_id` без `participation`, в физической схеме — `scope_id` + `participation`. В §5.1a у `CharacterRequest` есть `correlation_id`/`created_at`, в физической схеме — `status_code`/`response_json`/`expires_at`. В §5.6 `Delivery.state` содержит `leased`, в физической схеме лизинг выражен `leased_by`/`leased_until` при `state IN ('pending','delivered','dropped')`. Сверить с component §4.
2. **architect#3 — component §3/§4.1/§6 и ADR-019 п. 1, доп. п. 1.** Сигнатуры `OpenLinks(ctx, path)`/`OpenGateway(ctx, path)`, `store.CompactLinks` как реализация `links.Store.Compact` и шага физического удаления в `Forget`, порядок vacuum → checkpoint(TRUNCATE), DSN на коротких ключах драйвера, `CHECK end_reason` с `forget` в основном тексте §4.2. Совпадает с предложением 2 исполнителя.
3. **devops (T-l, том `MV_GATEWAY_DATA_DIR`).** В `build/Dockerfile` каталога `/data` нет, образ работает от `nonroot`. Именованный том `gateway-data:/data` будет принадлежать root с правами `0755`: процесс не сможет писать, а `store` откажется стартовать. Нужен `/data` в образе, владелец `nonroot`, права `0700` (Docker копирует права каталога образа в пустой том). Уточняет предложение 4 исполнителя.
4. **devops/EPIC-001 — `make test` и coverage-gate.** Добавить `internal/gateway/*` в порог покрытия, когда появятся пакеты с логикой (предложение 1 исполнителя).
5. **CLAUDE.md, карта каталогов:** `go.mod # … go 1.26` → `go 1.26.0`. Косметика, владелец документа — оркестратор.

### Риски и допущения

- Код T-302 ни разу не исполнялся на Linux и под `-race`. Тесты прав и связка `modernc.org/sqlite` + race detector + `checkptr` впервые пойдут в CI (job `unit`). По устройству `modernc.org/libc` (память вне кучи Go) срабатываний `checkptr` не ожидаю, но это допущение, а не проверка.
- `pierrec/lz4/v4` 4.1.15 → 4.1.29 входит в бинарник платформы через `kafka-go` (декодирование lz4-батчей). Unit-тесты шины зелёные. Contract-тест на живой Redpanda ревьюер не запускал: Docker в этом поручении запрещён.
- Физическое удаление доказано на уровне файлов SQLite. Остатки на уровне файловой системы или SSD (усечённые блоки WAL) вне модели SEC-04 и не проверялись.
- Два процесса gateway на одном томе (например, при перекрывающемся рестарте) не исключены блокировкой: `locking_mode` обычный, у goose нет session locker. Единственность писателя обеспечивает деплой (ADR-004/ADR-019).
# Ревью EPIC-004

Формат записи: задача, номер ревью, дата, экземпляр ревьюера; границы ревью; вердикт; что проверено; замечания по серьёзности (Critical / Major / Minor / Nit) в виде «файл:строка — суть — предлагаемая правка». Записи добавляются, старые не удаляются.

## T-301 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-3)

### Границы ревью

Ветка `task/T-301-openapi-dto-client`, папка `.worktrees/T-301`. Изменения не закоммичены, все файлы новые (`git status`):
- `api/gateway.openapi.yaml`;
- `internal/gateway/api/{dto.go,errors.go,router.go,openapi_test.go,errors_test.go,router_test.go}`;
- `internal/gateway/client/{client.go,longpoll.go,client_test.go,longpoll_test.go}`;
- `tasks/T-301.md`, `dev-log.md`.

Задача проверялась в сокращённом составе по сверке 2026-09-13 (раздел T-301 в `tasks.md` ветки эпика 53f1ea2): схемы C-04/C-10 и реестр сделаны в EPIC-001 и не менялись. Чужие файлы, `go.mod`, `.golangci.yml`, схемы и реестр не тронуты.

Основания:
- `contracts.md` v0.10: C-04 v1.3, C-06, C-08 v1.1–v1.3, C-10 v1.1;
- `analysis/api-contracts.md` §1;
- `components/gateway-and-bot.md` §5, §6, §10.4–10.5;
- решения ревизии 4 system-architect (журнал `develop` 2026-09-13; черновик T-444 — `contracts.md` v0.11, ADR-001 доп. п. 5; черновик T-445 — `.golangci.yml`).

### Вердикт

**ПРИНЯТЬ** — Critical 0 · Major 0 · Minor 7 · Nit 6.

Спецификация, DTO, таблица кодов и клиент соответствуют C-08 v1.3 и КД. Отклонения от §5.3, §5.6 и §6 обоснованы в карточке и потребителей плана не ломают. Тест сверки доказателен в заявленных пределах: мутанты автора A1–A23 не перепроверялись поштучно, свои мутанты ниже. Замечания касаются дыр в тестах и ловушек для будущих обработчиков и потребителей.

Mi-1, Mi-2 и Mi-5 дешевле закрыть до поставки в `develop`: DTO и спецификацию берут другие команды. Остальное можно внести в DoD T-303, T-308 и T-310/T-311. Решение за tech-lead#3.

### Прогоны (go1.26.8 windows/amd64, golangci-lint 2.13.2, рабочая папка T-301)

| Команда | Результат |
|---|---|
| `go build ./... && go vet ./...` | 0 |
| `gofmt -l internal/gateway` | пусто |
| `go test -short -count=1 ./internal/gateway/...` | `api` ok, `client` ok |
| `golangci-lint run ./internal/gateway/...` | 0 issues |
| то же с `--config` черновика T-445 (правило `internal-gateway-client`, решение 1e) | 0 issues |
| `go list -deps ./internal/gateway/client` (модуль) | только `internal/gateway/api`, `shared/clock` — клиент листовой |
| `go run ./cmd/mvctl contracts check` | 65 types, 8 topics, 58 schema files, 0 |

### Мутанты ревьюера

Копия дерева (`git ls-files -co --exclude-standard` без `services/`, `Docs/`, `.claude/`) лежала в scratch в каталоге с точным именем, без `-overlay`. Базовый прогон копии был зелёным. Каждая мутация — точная замена (`count == 1`), после неё файл восстанавливался со сверкой sha256. В конце шесть изменяемых файлов копии сверены с рабочей папкой — равны. Копия удалена по сохранённому точному пути.

| # | Мутант | Результат |
|---|---|---|
| R0 | контрольный: синтаксическая ошибка в `router.go` | **красный** (сборка) |
| R1 | спецификация: `GroupView.leader_id` без `null` (сторона YAML; у автора — сторона Go) | **красный**: `TestDTOsMatchOpenAPISchemas` |
| R2 | спецификация: `cloud_enabled` → `cloudEnabled` | **красный**: `TestDTOsMatchOpenAPISchemas` |
| R3 | спецификация: `ActionPending.status` enum `[accepted]` | **зелёный** → Mi-5 |
| R4 | `group_full` удалён согласованно из `errors.go` и из `x-error-codes`/описания `Conflict` | **зелёный** → Mi-5 |
| R5 | роутер монтирует `resolveLink` как `GET`, операция вычеркнута из `notYetMounted` | **красный**: `TestOpenAPIRoutesMatchRouter` |
| R5p | то же с верным `POST` (позитивный) | `TestOpenAPIRoutesMatchRouter` зелёный; красные только `router_test`, которые рассчитывают на пустой `NewRouter()`, — ожидаемо |
| R6 | второй конструктор маршрутов (как в T-303) с неверным путём; тест остаётся на `api.NewRouter()` | **зелёный** → Mi-6 |
| R7 | клиент повторяет `429` | **зелёный** → Mi-4 |
| R8 | транспортная ошибка клиента содержит тело запроса (`external_id`) | **зелёный** → Mi-4 |
| R9 | повтор уходит с другим `action_key` | **красный**: `TestUnavailableIsRepeatedThreeTimesWithTheSameActionKey` |
| R10 | `CloseRound` без повторов | зелёный — поведение не закреплено, см. Mi-7 |

### Проверено по пунктам задания

**1. OpenAPI против C-04/C-08.**
- Операции: 17 `operationId` §5.6 плюс `adminLlmUsage` (в §5.6 без имени) и `health`. Пути, методы, `x-idempotency` у `createCharacter`/`postAction`, `x-actor-kind: [ci]` у служебных — верно.
- Коды: 36 кодов таблицы = 33 кода §1.6 и C-08 v1.1 + `consent_incomplete` (§1.2), `internal` (§5.1), `character_alive_exists` (§1.3). Статусы совпадают с §1.6.
- `202 pending`: `postAction` отдаёт `202 oneOf [ActionAccepted, ActionPending]`, `status` в двух ветках не пересекается; `200` — `GroupView` (C-08 v1.1). Верно.
- `409 no_leader`: есть в `Conflict`, в описании — «checked before not_leader»; в `postAction` — то же. Верно.
- `leader_id`: `[string, 'null']` и обязательное; `CharacterState.group` ссылается на тот же `GroupView` (C-08 v1.2). Верно.
- `llm.cloud_enabled`: `WorldSummary.llm: WorldLLM{cloud_enabled}` обязательное; провайдера, URL и ключей нет (SEC-21). Верно.
- `external_platform`: `enum: [telegram]` в `ResolveRequest`/`ConsentRequest`/`ForgetRequest`/`CreateCharacterRequest`. В `DeliveryRoute` (ответ) — строка без enum. Для MVP-1 так правильно. Расширение enum запроса для второго бота совместимо (открытый вопрос автора 2 — согласен, снимать enum не нужно).
- `admin`: три операции C-06 с `x-reserved: EPIC-003 T-239`, тег с пометкой Reserved, ответ `default`. Совладение по C-06 соблюдено, тест не даёт снять пометку незаметно. Неточность описания тега — N-1.
- `character_dead` — «also for abandoned»; `CharacterState.status` содержит `abandoned`; `ResolveResponse.character_status` без `abandoned` (после `/forget` — `none`). Верно.
- Расхождение: `ForgetResponse` — Mi-2.

**2. Доказательность теста сверки.**
- Сверка DTO со схемами (свойства, `required`, nullable, вид типа, `$ref` на свой тип) ловит расхождение с обеих сторон (R1, R2; A4, A5, A17, A22 автора).
- Коды ↔ спецификация — в обе стороны со статусами.
- Роутер ↔ спецификация: неверный метод при монтировании краснеет (R5), смонтированная операция не может остаться в списке.
- Пределы: `notYetMounted` не вынуждает вычеркнуть операцию, пока тест смотрит на пустой `api.NewRouter()` (R6, Mi-6). Согласованное выпадение кода §1.6 не ловится (R4), константы статусов действия со спецификацией не связаны (R3) — Mi-5.

**3. Клиент.**
- Повторы — только сетевые ошибки и `503` (`client.go:279-284`), тело сериализуется один раз (`client.go:243-249`), поэтому `action_key` тот же (R9). `4xx` и прочие `5xx` не повторяются (A10, A15 автора). Дыра в тестах по `429` — Mi-4.
- Паузы через `clock.Timers.After`, отмена контекста прерывает паузу. В тестах фейковые таймеры, реального ожидания нет: тесты пакета идут около 10 мс каждый.
- Long-poll не повторяется (`longpoll.go:41`), `409 poll_in_progress` отдаётся как `APIError`. Ограничения `limit ≤ 100`, `wait_ms ≤ 25000` зажимаются. `DefaultHTTPTimeout` 35 с больше серверного предела long-poll `wait_ms + 5 с ≤ 30 с` (§5.1) и согласован с решением 6 ревизии 4 (`runtime.SetDeadlines`/`ShuttingDown`, T-446). Отмена контекстом во время висящего опроса тестом не закреплена — Mi-4.
- Горутин клиент не заводит. Тело ответа закрывается `defer` и читается с пределом 4 МиБ. Таймер паузы при отмене останавливается. Утечек нет.
- `X-Client-Id` ставится на каждый запрос и совпадает с `{client_id}` пути по построению (`longpoll.go:57-59`).
- Логов в клиенте нет. `APIError` несёт только `code`/`message`/`details`/`X-Request-Id` из ответа сервера. Тексты ошибок содержат метод и путь, где есть `player_id`/`group_id`/`scope_id`/`client_id`, но нет внешних ID и тела. Сейчас верно, но не закреплено тестом — Mi-4.

**4. Отклонения от КД §5.3, §5.6, §6.** Все обоснованы.
- `ActionResult`: `202 pending` не помещается в `(ActionAccepted, *GroupView)`.
- `Client.Group`: нужен для опроса после `pending`.
- `Deliveries`/`Ack` без `clientID`: SEC-12, один источник. Вызов бота §10.4 `Deliveries(ctx, "telegram-bot", …)` превращается в `Client.ClientID = "telegram-bot"`.
- `Ack` → `AckResponse`: нужен `unknown[]`.
- `WorldSummary`: имя `WorldView` занято.
- `Health = runtime.Status`: `/health` принадлежит процессу.

Потребители плана ещё не написаны (T-308, T-310, T-311). Сигнатуры им подходят: Harness (T-308) обернёт `Action` в `Act`, бот (T-311) разберёт `ActionResult` по одному полю. Ловушка нулевого `Client{}` для тех, кто пишет по §6, — Mi-3.

**5. forbidigo и depguard.** `time.Now`, `time.After` и `os.Getenv` в новом коде нет. `internal-gateway` чист. Правило `internal-gateway-client` из черновика T-445 на этом дереве чистое: `client` импортирует из модуля только `api` и `shared/clock`. `api` в production-коде зависит только от stdlib, в тестах — от `schemas`, `shared/contracts`, `shared/runtime` и `yaml.v3`, который уже прямая зависимость `go.mod`.

**6. Объём.** 3543 строки:
- спецификация — 949;
- тесты — 1576 (из них `openapi_test.go` — 783);
- код — 1018.

Это больше нормы M (≤ ~600 строк), но вес дают сама спецификация и табличные сверки. Генерация DTO (`oapi-codegen`) отвергнута КД §5.3, и вместо неё стоит рефлексивная сверка. Дублирование «YAML ↔ Go» здесь намеренное и охраняется тестом. Лишнего кода, который стоило бы убрать, не нашёл. Для планирования: пункты «OpenAPI обновлён» в T-303…T-306 и T-352 станут малыми правками. Не замечание.

### Замечания

#### Critical
Нет.

#### Major
Нет.

#### Minor

**Mi-1. `internal/gateway/api/dto.go:111, 118, 192, 210, 312, 325, 342` — nil-срез уходит в JSON как `null` в обязательных массивах.**
- Суть. `worlds`, `regions`, `members`, `npcs`, `deliveries`, `unknown` и `sessions` в спецификации объявлены как `type: array` без `null` и входят в `required`. `encoding/json` кодирует nil-срез как `null`. Комментарий `dto.go:309-310` обещает «empty array, not null», но структура этого не обеспечивает.
- Где сработает. `AckResponse{Acked: n}` без неизвестных id — самый частый ответ `ack` — уйдёт с `"unknown": null`. Сверка `TestDTOsMatchOpenAPISchemas` смотрит форму, а не кодирование, поэтому не поймает. Клиенту Go это безразлично, но харнесс и внешний валидатор увидят нарушение контракта.
- Правка. Добавить рефлексивный тест: кодировать нулевое значение каждого DTO из `dtoOfSchema` и проверять, что обязательные массивы не `null`. Чинить одним из двух способов: `MarshalJSON` у ответов нормализует nil в `[]`, или конструкторы ответов в T-303. Тест лучше завести сейчас: он охранит и будущие поля.

**Mi-2. `api/gateway.openapi.yaml:117-118` против `:673-678` — `ForgetResponse` противоречит сам себе и `api-contracts.md` §1.2.**
- Суть. Описание операции и §1.2 говорят «без связки — `200 {deleted: false}`», а схема требует `player_id_detached`. DTO всегда кодирует поле как `null` — это совместимо со схемой, но не с текстом.
- Правка. В описании `forgetLink` написать `{deleted: false, player_id_detached: null}`. Правку примера §1.2 передать system-analyst/architect#3 (см. «Для architect#3», п. 6).

**Mi-3. `internal/gateway/client/client.go:65-79` (поля `Timers`, `Backoff`) — нулевой `Client` молча отключает повторы.**
- Суть. КД §6 описывает клиента литералом `Client{BaseURL, ClientID, ActorKind, HTTP}`. Разработчик бота, который пишет по §6, получит клиента без повторов: нулевой `Backoff` = 0 повторов. Обещание §10.5 «клиент повторил с тем же `action_key` до 3 раз» не выполнится, и ни один тест бота этого не заметит.
- Правка (одно из двух).
  - (а) Нулевой `Backoff` трактовать как `DefaultBackoff`, а «без повторов» задавать явно (`Retries: -1` или переменная `NoRetry`). `TestZeroBackoffRepeatsNothing` переписать на явную форму.
  - (б) Оставить как есть, но в doc пакета и в §6 (через architect#3) записать: создавать только через `New`. В DoD T-310 и T-308 добавить тест «клиент создан через `New`».

**Mi-4. `internal/gateway/client/client_test.go:223-245, 387-398`, `longpoll_test.go` — три поведения клиента не закреплены тестами (R7, R8 зелёные).**
- (а) Невповтор `4xx` проверен только на `409` и `502`. Повтор `429` (R7) проходит, хотя бьёт по rate limit SEC-11. Правка: табличный тест по статусам 400, 403, 404, 409, 413, 422, 429, 500, 501, 502 — ровно один запрос и ни одной паузы.
- (б) Нет теста, что ошибка `Resolve`/`Consent`/`Forget`/`CreateCharacter` не содержит `external_id`. Нужны оба случая: исчерпанные сетевые повторы и `APIError`. Бот будет логировать эти ошибки (SEC-01/02, DoD-common «тест, где указано»). Утечка тела в текст ошибки (R8) проходит. Правка: тест с `external_id = "tg-4242"` проверяет `err.Error()` и `%+v` у `APIError`.
- (в) Отмена контекстом во время висящего long-poll не проверена. Правка: сервер держит ответ, `cancel()`, проверить `errors.Is(err, context.Canceled)`, один запрос, возврат без ожидания.

**Mi-5. `internal/gateway/api/openapi_test.go:369-390, 685-731` — два расхождения, которые сверка не ловит (R3, R4 зелёные).**
- (а) `api.ActionStatusAccepted`/`ActionStatusPending` не сверены с enum `ActionAccepted.status`/`ActionPending.status`. Клиент различает `202` именно по этой константе (`client.go:205`). Правка: две строки `equalSets` в `TestOpenAPIEnumsMatchGoAndEventSchemas`.
- (б) Код §1.6 можно согласованно убрать из `errors.go` и спецификации: `TestErrorTableHasTheCodesOfC08` закрепляет поимённо только дополнения C-08, а DoD требует наличия всех кодов. Правка: закрепить в тесте полный перечень §1.6 с C-08 v1.1–v1.3 (36 кодов со статусами), чтобы потеря кода была видна в диффе теста.

**Mi-6. `internal/gateway/api/openapi_test.go:237-242, 255` — `notYetMounted` не мешает «забыть» операцию.**
- Суть. Тест сверяет спецификацию с `api.NewRouter()`, который по построению пуст. Настоящий конструктор маршрутов появится в T-303 и может монтировать что угодно по неверному пути — тест останется зелёным (R6), пока кто-то не поменяет строку 255. Список ни к чему не привязан: операция может остаться в нём до конца эпика.
- Правка.
  - Сделать `notYetMounted` картой `operationId → задача` (`"resolveLink": "T-303"`, `"postAction": "T-305"`, `"pollDeliveries": "T-307"`, … — раскладка в «Предложениях в бэклог», п. 1), чтобы пропуск был виден по номеру.
  - Предложить tech-lead#3 строку DoD в T-303: «тест сверяет спецификацию с конструктором маршрутов шлюза, а не с `api.NewRouter()`».
  - Во все задачи с пунктом «OpenAPI обновлён»: «свои операции вычеркнуты из `notYetMounted`». Автор предложил то же в бэклог (п. 1) — поддерживаю, это закрывает замечание.

**Mi-7. `internal/gateway/client/client.go:135` (`Forget`), `:222-229` (`CloseRound`) — повтор после потерянного ответа меняет исход.**
- `Forget` при сетевой ошибке повторяется. Если первая попытка удалила связку, повтор вернёт `{deleted: false}`, а `player_id_detached` потеряется. Бот скажет «нечего удалять» после настоящего удаления (UC-031, SEC-26).
- `CloseRound` после успешного закрытия вернёт `409 no_open_round`, и харнесс увидит провал. В комментарии это признано, но вызывающему не видно.
- Правка.
  - `CloseRound` — `retry: false`: служебный вызов `ci`, харнесс сам решает, повторять ли.
  - `Forget` — в doc метода записать, что `deleted: false` после транспортной ошибки означает «исход неизвестен». Либо вернуть признак повтора (например, `ForgetResult{…, Repeated bool}`), чтобы T-311 проверил итог через `Resolve`.
  - Строку о трактовке добавить в DoD T-311.

#### Nit

- **N-1. `api/gateway.openapi.yaml:45-49`.** Тег `admin` описан как «Proxy of /v1/admin/*», но `/v1/admin/sessions` (`:377`) и `/v1/admin/links/{player_id}` (`:138`) — собственные маршруты шлюза и в прокси не уходят. Правка: «Proxy of /v1/admin/agents* and /v1/admin/llm/usage». Та же неточность есть в C-06 — см. «Для architect#3», п. 7.
- **N-2. `internal/gateway/client/client.go:205-210`.** Любой `status`, кроме `pending`, декодируется как `Accepted`: неизвестное значение пройдёт молча. Правка: `accepted` → `Accepted`, иначе `ErrUnexpectedStatus`.
- **N-3. `internal/gateway/client/client.go:295-298`.** Ошибка `http.NewRequestWithContext` (кривой `BaseURL`) повторяется трижды с паузами как сетевая. Правка: вернуть её сразу, без повтора.
- **N-4. `internal/gateway/client/client.go:85, 287`.** Хвостовой `/` у `BaseURL` срезается только в `New`; литерал с `http://host/` даст `//v1/...`. Правка: срезать в `once`.
- **N-5. `api/gateway.openapi.yaml:73-74`.** Заголовок `X-Request-Id` объявлен только у `200` в `resolveLink`, хотя middleware §5.1 ставит его на каждый ответ. Правка: убрать отовсюду до T-303 или описать единообразно.
- **N-6. `internal/gateway/api/errors.go:128`.** Схема `Error` соответствует Go-типу `ErrorResponse`, а `api.Error` — значение ошибки обработчика. Имена легко спутать. Правка: одна строка в doc `Error`/`ErrorResponse` о соответствии.

### Для architect#3 (записать в `components/gateway-and-bot.md` через develop)

1. §6, клиент:
   - `Action` → `ActionResult{Status, Accepted|Pending|Group}`;
   - добавлен `Group(ctx, groupID)`;
   - `Deliveries(ctx, after, limit, wait)` и `Ack(ctx, ids) (AckResponse, error)` без `clientID` — берётся из `Client.ClientID` (SEC-12);
   - поля `Timers clock.Timers`, `Backoff`, конструктор `New`, `APIError.RequestID`, `ErrUnexpectedStatus`;
   - трактовка повторов «3 повтора, паузы 200/400/800 мс, потолок 1,6 с»;
   - long-poll внутри клиента не повторяется;
   - по итогу Mi-3 — правило создания клиента;
   - §10.4 — вызов `Deliveries` без `"telegram-bot"`.
2. §5.3, DTO:
   - `ResolveResponse.CharacterStatus string` (enum с `none`, а не `*string`);
   - `CreateCharacterResponse.Created *bool`, `Status` с `omitempty`;
   - `GroupView.LeaderID *string`, `Position *PositionRef`, `State`;
   - `Delivery.Route *DeliveryRoute`, `NarrativeEventID`;
   - новые типы `WorldsResponse/WorldSummary/RegionSummary/WorldLLM`, `ActionPending`, `AckResponse`, `RoundCloseResponse/ClosedRound`, `AdminSessions/AdminSession`;
   - правило тегов «без `omitempty` — обязательное, указатель без `omitempty` — nullable».
3. §5.6:
   - 18 операций (17 `operationId` + `health`), имя `adminLlmUsage`;
   - схемы `ErrorBody`, `ActionPending`, `TurnRef`, `ClosedRound`, `AdminSession`, `WorldSummary`/`RegionSummary`/`WorldLLM`;
   - ответы `PayloadTooLarge`, `UnprocessableEntity`, `InternalError`, `NotImplemented` с расширением `x-error-codes`;
   - тест сверяет роутер через `notYetMounted`, а не равенством множеств на пустом роутере.
4. §11.4 и `api-contracts.md` §1.8: схема `/health` в спецификации = `runtime.Status{status, details}`. Расширенная форма (`deps`, `agents_by_level`, `snapshot`) ложится в `details` (T-309).
5. `/v1/admin/sessions` и `DELETE /v1/admin/links/{player_id}` — тег `service`, не раздел `admin`.
6. `api-contracts.md` §1.2: пример «без связки» — `{deleted: false, player_id_detached: null}` (Mi-2; владелец — system-analyst).
7. C-06: «gateway проксирует `/v1/admin/*`» — уточнить до `/v1/admin/agents*` и `/v1/admin/llm/usage`. Собственные admin-маршруты шлюза проксироваться не должны. В процессе `--contexts=all` прокси и маршруты `swarm`/`llm` делят один mux, повторная регистрация шаблона уронит процесс паникой — это решать в задаче прокси (N-1, риск).
8. Открытый вопрос автора 1 (заголовок C-08) закрыт решением 8(б) ревизии 4 (T-444, C-08 v1.4). Ссылки T-301 на C-08 v1.3 после слияния T-444 не устаревают: API `/v1` в v1.4 не менялся.

### Связь с ревизией 4 system-architect

- **1e.** Клиент листовой — подтверждено линтером с черновиком T-445 и `go list -deps`.
- **6.** Таймауты long-poll (`SetDeadlines`, `ShuttingDown`, T-446) — на клиента не влияют, `DefaultHTTPTimeout` согласован.
- **5.** `gm.created` — T-305, T-301 не касается.
- **8(в).** В черновике T-444 (`contracts.md` v0.11, C-04 v1.4, строка 390 и таблица решений п. 8) записано: «`group_move` … Схемы сужает владелец в T-301». В дереве `group_move` ещё есть в `schemas/events/group.{created,joined,left,disbanded}.v1.json`. Карточка T-301 честно говорит «схемы не менялись», и по действующей v0.10 это верно, поэтому замечанием к этой итерации не считаю. Вопрос оркестратору и tech-lead#3 — в отчёте.

### Предложения в бэклог

1. DoD T-303 и задач с пунктом «OpenAPI обновлён»: тест сверяется с конструктором маршрутов шлюза, свои операции вычеркнуты из `notYetMounted` (Mi-6; совпадает с п. 1 автора). Раскладка операций по задачам `tasks.md`:
   - T-303 — `resolveLink`, `consentLink`, `forgetLink`;
   - T-305 — `postAction`;
   - T-306 — `listWorlds`, `createCharacter`, `getPlayer`;
   - T-307 — `pollDeliveries`, `ackDeliveries`, `streamDeliveries`;
   - T-352 — `getGroup`;
   - T-354 — `closeRound`;
   - T-356 — `adminForgetLink`, `adminSessions` и прокси `adminTick`/`adminAgents`/`adminLlmUsage` (текст — EPIC-003 T-239).
1а. **DoD T-303, строка «`openapi_test.go` зелёный: маршруты `links`, `/health` из `router.go` есть в `api/gateway.openapi.yaml`»** противоречит решению T-301 (отклонение 7 карточки). `/health` регистрирует `shared/runtime.NewHTTP`, и тест теперь краснеет, если роутер шлюза его смонтирует (`servedByProcess`). Строку нужно поправить: «`/health` — маршрут процесса, в `router.go` не монтируется». Правка tech-lead#3.
2. DoD T-310 и T-311: клиент создаётся через `New` (Mi-3); `deleted: false` после транспортной ошибки у `/forget` проверяется через `Resolve` (Mi-7).
3. DoD T-303: ответы шлюза не отдают `null` в обязательных массивах (Mi-1), если тест не заведён в T-301.
4. Makefile `test` из PowerShell (п. 2 автора) — поддерживаю, владелец EPIC-001.

<!-- review T-303 #1 -->
## T-303 · ревью #1 · 2026-09-13 · code-reviewer#1 (TEAM-3)

### Границы ревью

Рабочая папка `.worktrees/T-303`, ветка `task/T-303-links-http-layer`, база `fdc7e05` (`epic/EPIC-004-gateway-bot`). Коммита задачи нет, поэтому ревьюировались **незакоммиченные изменения** (`git status --untracked-files=all`). Изменены: `api/gateway.openapi.yaml`, `cmd/multiverse/{contexts.go,main_test.go,serve_test.go}`, `internal/gateway/api/{errors.go,openapi_test.go,router.go}`, `test/e2e/empty_world_test.go`, карточка и `dev-log.md`. Новые: `cmd/multiverse/gateway_context_test.go`, `internal/gateway/{context.go,context_test.go}`, `internal/gateway/api/{middleware.go,middleware_test.go}`, `internal/gateway/handlers/{links.go,links_test.go}`, `internal/gateway/links/{link.go,pseudonym.go,store.go,forget.go,*_test.go}`. Посторонних путей нет. `go.mod`, `shared/env/vars.go` и `.env.example` не менялись.

Основание: `tasks.md` §T-303 (сверка 2026-09-13), карточка T-303 («Выполнение»), запись `dev-log T-303`, `components/gateway-and-bot.md` §3, §4.1, §5.1, §5.2, §6, §7.5, §11.1, §11.4, ADR-019 с доп. п. 1, `analysis/api-contracts.md` §1.1, §1.2, §1.6, `api/gateway.openapi.yaml`. Решение по C-08 (`503 forget_incomplete`) принимает system-architect#1 параллельно; здесь оценены реализация и риски.

### Вердикт

**Вернуть.** Critical: 0 · Major: 1 · Minor: 6 · Nit: 7.

Ядро задачи сделано хорошо. `/forget` идёт в порядке «хуки → DELETE → сжатие» до ответа. Стирание доказано сканом `links.db`, `-wal` и `-shm` с контролями. Отказ сжатия не выдаётся за успех. Middleware закрывает SEC-11/12, nolog-политика проверена тестом и мутантами. Отклонения от дизайна обоснованы. Возвращаю из-за M-1: с этой задачей контекст `gateway` в compose впервые открывает SQLite на томе `/data`, а образ к этому не готов, и `make up` перестаёт подниматься. Minor: гонка `/forget` с привязкой персонажа; потеря отметки «сжатие отложено» при рестарте; `/health degraded` и сжатие в `Stop` не проверены тестами; неполное согласие создаёт связку; умолчание `/data` на хосте; `Link` внутри структур не редактируется в JSON-логе.

### Проверено ревьюером (только чтение; go1.26.8 windows/amd64, cgo выключен — без `-race`)

| Проверка | Как | Результат |
|---|---|---|
| сборка и vet | `go build ./...`; `go vet ./internal/gateway/... ./cmd/multiverse/`; `go vet -tags e2e ./test/e2e/` | 0 / 0 / 0 |
| unit | `go test -short -count=1 ./internal/gateway/... ./cmd/multiverse/` | 8 пакетов ok |
| линтер | `golangci-lint run ./internal/gateway/... ./cmd/multiverse/... ./test/...` | 0 issues |
| порядок старта | `cmd/multiverse/serve.go:297-303`, `shared/runtime/lifecycle.go:64-67` | `Routes` → `Start` → `srv.Start()`: сервер начинает обслуживать после `StartAll`, и поля `api.Config`/`handlers.Links` к этому моменту заполнены. Гонки в процессе нет |
| `/health` при `degraded` | `shared/runtime/http.go:59-63`, `lifecycle.go:137-156` | `degraded` → HTTP 200, `fail` → 503: отложенное сжатие не валит healthcheck compose |
| редакция `Link` | зонд вне модуля в scratch (копия типа с теми же `LogValue/String/GoString`) | верхний атрибут slog, `%v/%+v/%#v` и обёрнутая ошибка дают `redacted`. **Вложенный** `Link` (`Resolution`, `[]Link`) в `slog.NewJSONHandler` печатает `LinkID` и `ExternalID` (Mi-6). Зонд удалён по точному пути |
| образ и том | `build/Dockerfile:28-35`, `docker-compose.yml:243-273`, `internal/gateway/store/open.go:177-220` | каталога `/data` в образе нет, `USER nonroot`; том `gateway-data:/data` — root `0755` → gateway отказывается стартовать (M-1). Docker не запускался, вывод по чтению |
| мутанты | копия дерева в scratch (tar без `.git`, `services/`, `Docs/`), без `-overlay`; замена только единственного вхождения, побайтный откат, `diff -r` копии с папкой задачи; копия удалена по точному пути | таблица ниже |

| # | Мутант | Результат |
|---|---|---|
| M0 | контрольный: `BodyLimit = 1 << 30` | **красный**: `TestBodyLimitAndContentType/65_KiB`, `65_KiB_chunked` |
| K1 | `Health` не смотрит `CompactionPending` | **зелёный**: `degraded` не проверяется (Mi-3) |
| K2 | `Stop` не пытается сжать | **зелёный**: не проверяется (Mi-3; исполнитель это отметил) |
| K3 | `limitBody` раньше `admitClient` в `Chain` | **зелёный**: порядок шагов 3/4 не закреплён (N-1) |
| K4 | `ContentLength >= BodyLimit` | **зелёный**: граница ровно 64 КиБ не проверяется (N-1) |
| K6 | значение паники пишется и на nolog-операции | **красный**: `TestNoLogOperationsLogOnlyTheRequestIDAndTheCode` (первая форма не собралась и не засчитана) |
| K7 | `Sweep`: `expires_at < ?` вместо `<=` | **красный**: `TestCharacterRequestsKeepTheFirstAnswerForTheirTTL` |
| K9 | `client_mismatch` только для путей на `/deliveries` | зелёный: других смонтированных маршрутов с `{client_id}` пока нет, для T-303 не дефект |
| K11 | открытие БД без `context.WithoutCancel` | **красный**: `TestServeSubcommandAndTheBareFormAreOneCommand` (4 случая) |

### Разбор по пунктам поручения

**1. Приватность `/forget`.** Порядок верный (`links/forget.go:78-92`). Хуки вызываются только при наличии персонажа; если хук упал, связка остаётся. `DELETE` каскадом удаляет `character_requests`. `Compact` выполняется до ответа; его ошибка превращается в `ErrCompactionPending` и `503`. Тест `TestForgetWipesTheExternalIDFromTheFileAndTheWAL` доказательный. Сначала ID переносится в сам файл (контроль > 0). После `Forget` в `links.db`, `-wal` и `-shm` его 0, а ID, который должен остаться, скан видит. `CompactLinks` (T-302) считает `busy != 0` ошибкой, и ветка 503 опирается именно на это. `Link` редактирует себя на верхнем уровне slog и во всех формах fmt. Вложенный в структуру или срез `Link` в JSON-логе платформы не редактируется (Mi-6): так сейчас никто не логирует, но обещанной component §11.1 гарантии «по построению» нет. Nolog: в строке access-лога только `request_id` и `code` (у паники ещё `handled=false`). Значение паники и стек на nolog-маршрутах не пишутся (мутант K6 красный). `403` шага 3 тоже логируется без маршрута и тела. Цена этого — причина `500` на этих маршрутах теряется; исполнитель вынес вопрос в бэклог (п. 8), согласен.

**2. `503 forget_incomplete`.** Реализация согласована в четырёх местах: `errors.go:106-108`, `Unavailable.x-error-codes`, ответ 503 у `forgetLink`, эталонный список `openapi_test.go`. Отметка «сжатие отложено» ставится при любой неудаче `Compact` и снимается при успехе. Сжатие доделывают: `/forget` без связки (`forget.go:70-76`), `Sweep` раз в минуту, полное сжатие раз в час и попытка в `Stop`. Пока отметка стоит, `/health` отвечает `degraded` (HTTP 200, compose контейнер не перезапускает). Отвергнутые варианты (200 с доделкой sweeper'ом, 500) отвергнуты правильно.

Риски:
- (а) отметка живёт только в памяти (Mi-2);
- (б) повтор после 503 отвечает `{deleted:false, player_id_detached:null}`, и клиент теряет `player_id_detached`. По документации клиента T-301 (`client/client.go:141-146`) неоднозначность разрешается через `Resolve`. Но `Resolve` по дизайну **создаёт** связку и снова пишет внешний ID в `links.db` сразу после `/forget` (бэклог 2, system-architect/T-311);
- (в) пока сжатие отложено, `/forget` любого аккаунта без связки тоже получает 503 (N-2);
- (г) узкое окно: успешный `Compact` sweeper'а может снять отметку, поставленную параллельным неуспешным `/forget`, если `DELETE` этого `/forget` попал между checkpoint sweeper'а и `pending.Store(false)` (`store.go:247-254`). Вероятность пренебрежимо мала; счётчик поколений закрыл бы и это. Оставлено в рисках.

**3. Middleware.** Фактический порядок: request_id + access-лог → recover → client → body_limit → ratelimit → pollguard → timeout (`api/middleware.go:99-102`). Перестановка request_id/recover обоснована: 500 от паники получает `X-Request-Id` и попадает в лог. Паника внутри самого access-лога не перехватывается, но там вызываются только `Clock.Now` и логгер, заполненные в `Start`.

Коды ошибок:
- `client_unknown` — при пустом и неизвестном `X-Client-Id`;
- `actor_kind_forbidden` — `ci|sim` от клиента вне списка;
- `400 invalid_request` — прочие значения `X-Actor-Kind`;
- `client_mismatch` — для любого пути с `{client_id}`.

Лимит тела: при `Content-Length > 64 КиБ` сразу 413; chunked-тело ограничивают `MaxBytesReader` и `DecodeJSON`. Порядок шагов 3/4 и граница ровно 64 КиБ тестами не закреплены (N-1). Дедлайн 5 с ставится через `context.WithTimeout` на контексте запроса, а не через `Timers`: это не доменное время, для replay допустимо. Long-poll из дедлайна исключён.

**4. Гонки.** Зависимости заполняются в `Start` без синхронизации, и при порядке `serve.go` это безопасно (см. таблицу). Будущему `shared/testkit/gateway.Harness` нужен тот же порядок. Запрос до `Start` упадёт на `nil` `Clock` в access-логе, вне recover, и `net/http` запишет панику в stderr мимо slog (бэклог 5). `/forget` против параллельной привязки персонажа — Mi-1.

**5. Отклонения от дизайна.** Все обоснованы:
- пакет `handlers`: `api` импортирует бот, а через `links` → `store` в бинарник бота попал бы драйвер SQLite;
- сигнатуры стора: `Resolution.PreviousSeenAt` нужен для `notice_due`; `ConsentForm` держит инвариант неполного согласия в сторе; `now` передаётся аргументом ради детерминизма;
- id берутся из `Deps.IDs` — по сверке задачи;
- `X-Request-Id` генерирует `uuid.NewString`, чтобы запросы не сдвигали `sequence`; это закреплено тестом и мутантом C6.

Отклонения перечислены в карточке, правки component §3/§5.1/§6 и C-08 — в бэклоге исполнителя (п. 3).

**6. Правки EPIC-001.** Правки минимальны:
- `contexts.go` — одна ветка `case gateway.Name` в `factoryOf`, фабрика в одну строку и комментарий; `gateway` остаётся на своём месте в `platformContexts`;
- `main_test.go` — пропуск одного имени;
- `serve_test.go` и `empty_world_test.go` — временный `MV_GATEWAY_DATA_DIR` с повторным удалением.

Перенос в `contexts_gateway.go` (T-446) правка не усложняет: `newGateway` и `case` переезжают целиком. Если `platformContexts` останется в `contexts.go`, импорт `internal/gateway` ради константы `gateway.Name` можно заменить литералом `"gateway"` — на усмотрение T-446. Хук `swarm` (T-255) не тронут.

**7. Умолчание `MV_GATEWAY_DATA_DIR=/data` на хосте.** См. Mi-5 и M-1.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| M-1 | Major | `internal/gateway/context.go:93-104`; `internal/gateway/store/open.go:177-188, 209-220`; `build/Dockerfile:28-35`; `docker-compose.yml:256, 273` | С T-303 сервис `gateway` (`--contexts=gateway`) при старте открывает `links.db` в `/data`. В образе каталога `/data` нет, процесс работает от `nonroot`, именованный том `gateway-data:/data` монтируется как root `0755`. `prepareDir` → `checkMode` отвергает `0755` при пределе `0700`; при подходящих правах `nonroot` всё равно не смог бы создать файл. Контекст не стартует, `make up` (`docker compose up --wait`) красный. Это регрессия стека по умолчанию: до задачи gateway отвечал `/health: ok`. CI её не ловит: compose там проверяется только `config -q`. Проблема известна с ревью T-302 (бэклог 3, devops), T-303 делает её действующей, но в карточке она не упомянута ни в рисках, ни в отклонениях. Docker ревьюер не запускал, вывод сделан по чтению Dockerfile, compose и `open.go`. | Выбор за оркестратором: (а) с согласия tech-lead#1 (так же, как правка `contexts.go`) добавить `/data` в образ с владельцем `nonroot` (65532) и правами `0700`, например `COPY --from=builder --chown=65532:65532 --chmod=0700 <пустой каталог> /data`; (б) отдельная задача devops, слитая в эпик **до** T-303. На стенде проверить `make minio-image && make up && make health` — gateway `ok`. Том `gateway-data`, уже созданный на машине владельца прежними `make up` (root `0755`), права каталога образа не унаследует: в runbook нужна строка о пересоздании пустого тома. В карточку T-303 добавить строку в «Риски» или «Отклонения» со ссылкой на выбранный путь. | открыто |
| Mi-1 | Minor | `internal/gateway/links/forget.go:54, 78-88` | Связка читается вне транзакции, затем идут хуки, затем `DELETE … WHERE link_id = ?` без условия на `player_id` и без проверки `RowsAffected`. Если между чтением и `DELETE` успеет `AttachPlayer` (T-306), новый персонаж останется без каскада: outbox не сброшен, `abandoned` не предложен, сессия не закрыта (связка при этом удалена, внешний ID стёрт). Два параллельных `/forget` одного аккаунта оба прогоняют хуки и оба отвечают `deleted:true`. Контракт стора задаётся здесь, а в T-306 эта ветка станет достижимой. | `DELETE FROM links WHERE link_id = ? AND player_id IS ?` со значением, прочитанным до хуков. При `RowsAffected = 0` перечитать связку: исчезла — `{deleted:false}`; сменился персонаж — повторить каскад, ограничив число попыток. Тест: хук вызывает `AttachPlayer` с другим `player_id`, и второй персонаж тоже проходит хуки. | открыто |
| Mi-2 | Minor | `internal/gateway/links/store.go:60-62, 245-257`; `internal/gateway/context.go:126-130, 158` | Отметка «сжатие отложено» хранится только в `atomic.Bool`. После рестарта или аварии между `DELETE` и checkpoint её нет: `/health` отвечает `ok`, `Sweep` не сжимает. Первое полное сжатие наступит только через `LinksCompactInterval` = 1 ч аптайма (`Timers.Every` сразу не тикает). Если процесс перезапускается чаще, сжатие не наступит вовсе, и байты внешнего ID остаются в `-wal` или в файле (SEC-04; ADR-019 доп. п. 1 называет этот случай «страховкой после аварийного завершения»). | В `Start` (live) один раз вызвать `store.Compact`: неудача ставит отметку (`degraded`) и пишется в лог, старт не валится. Тест: в WAL остались кадры удалённой строки (или отметка стояла до `Start`) → после `Start` скан = 0. | открыто |
| Mi-3 | Minor | `internal/gateway/context.go:197-201, 224-227`; `internal/gateway/context_test.go` | Новое поведение решения 6 не покрыто тестом контекста: `/health` `degraded` с `links_compaction: pending` и попытка сжатия в `Stop` (мутанты K1, K2 зелёные). Это наблюдаемая часть нового кода 503: по ней оператор видит, что `/forget` не довёл стирание. | Тест контекста с внешним читателем (как `forget_test.go:184-200`; `busy_timeout = 0` через отдельное соединение или короткий дедлайн запроса): `/forget` → `503 forget_incomplete` → `Health` `degraded`/`pending` → читатель уходит → `Stop` → скан `links.db` и `-wal` = 0. Второй вариант проверки: тик `SweepInterval` → `Health` `ok`. | открыто |
| Mi-4 | Minor | `internal/gateway/links/store.go:132-140, 159-161` | Неполное согласие для аккаунта без связки вставляет строку с внешним ID (`pending_consent`) и отвечает `400 consent_incomplete`. `api-contracts.md` §1.2 требует: «`consent_incomplete` (…запись `pending_consent`, ничего не создаётся)». Если человек отказался от согласия и `resolve` до этого не вызывался, ПДн хранятся без необходимости (минимизация, SEC-03). Решение 8 карточки говорит о создании связки только при полном согласии. | Если связки нет и форма неполная — вернуть `ErrConsentIncomplete` без `INSERT`. Тест: неполное согласие без `resolve` → `SELECT COUNT(*) FROM links` = 0. | открыто |
| Mi-5 | Minor | `shared/env/vars.go:98`; `internal/gateway/context.go:93`; `CLAUDE.md:136`; `Makefile:496` | Умолчание `/data` вне compose. Документированный запуск без Docker `go run ./cmd/multiverse serve --contexts=all --bus=memory` и `make replay` теперь открывают SQLite в `/data`. На Linux/macOS без root старт падает («mkdir /data: permission denied»). На Windows создаётся `C:\data` в корне текущего диска; проверка прав там пропускается (`open.go:213`), и файл с ПДн наследует ACL корня диска. Replay и ручной live-запуск делят один каталог. Для тестов это закрыто, для документации и `make replay` — нет. Исполнитель вынес пункт в бэклог (п. 7), но в «Риски» карточки не записал. | Манифест не менять: на него опирается compose. Оркестратору и tech-lead#1: в `make replay` задать `MV_GATEWAY_DATA_DIR` явно (временный каталог или `./.data/replay`); в `CLAUDE.md`/README рядом с `go run … --contexts=all` указать эту переменную. В карточку T-303 добавить строку в «Риски». | открыто |
| Mi-6 | Minor | `internal/gateway/links/link.go:62-70` | `LogValue` срабатывает только для значения атрибута верхнего уровня. `slog.Any("res", links.Resolution{…})` или `slog.Any("links", []links.Link{…})` в `slog.NewJSONHandler` (формат логов платформы, `shared/logging/logging.go:115`) печатает `LinkID` и `ExternalID` через `encoding/json` — проверено зондом. Таких вызовов сейчас нет, но component §11.1 обещает редакцию «по построению», а экспортируемый `Resolution` легко окажется в логе. | Добавить `MarshalJSON` у `Link`, возвращающий JSON-строку `"redacted"`: JSON-сериализации `Link` в коде нет, ломать нечего. В `TestALinkNeverPrintsItsIdentifiers` добавить случаи `Resolution` и `[]Link` в JSON-хендлере. | открыто |
| N-1 | Nit | `internal/gateway/api/middleware.go:99-102, 256`; `internal/gateway/api/middleware_test.go:192-232` | Порядок «client → body_limit» (§5.1 п. 3→4) и граница ровно 64 КиБ тестами не закреплены: мутанты K3 (перестановка) и K4 (`>=`) зелёные. Случай «under 64 KiB» — это 64 КиБ минус 1 байт. | Добавить: неизвестный клиент с телом 65 КиБ → `403 client_unknown` (а не 413); тело ровно 64 КиБ → 200. | открыто |
| N-2 | Nit | `internal/gateway/links/forget.go:70-76`; `api/gateway.openapi.yaml:153-156` | Пока сжатие отложено, `/forget` любого аккаунта без связки отвечает `503 forget_incomplete` («Удаление не завершено»), хотя у этого аккаунта ничего не удалялось. Описание 503 называет причиной только внешнего читателя, а 503 дают и истёкший дедлайн запроса, и отмена. | Поведение безопасное, его можно оставить, но уточнить описание: пока links.db не сжат после любого /forget, 503 получают и повторы, и другие аккаунты; причины — внешний читатель и дедлайн запроса. | открыто |
| N-3 | Nit | `internal/gateway/context.go:219-222` | `links_store` и `gateway_store` в `Health` — константы `ok`, а component §11.4 понимает под ними доступность БД (`fail`, если БД недоступна). Сейчас эти поля ничего не проверяют. | Убрать поля до появления реальной проверки или назвать их по смыслу (`opened`). Проверку, которая не блокирует единственное соединение, — в бэклог. | открыто |
| N-4 | Nit | `internal/gateway/context.go:187-195` | Если sweeper не остановился до дедлайна `Stop`, метод возвращается, уже выставив `started=false`: БД не закрыты, и повторный `Stop` их не закроет (на Windows файлы останутся заблокированными). | Закрывать БД и при таймауте ожидания (`database/sql` дождётся освобождения соединения) или не снимать `started` до закрытия. | открыто |
| N-5 | Nit | `internal/gateway/handlers/links.go:112-117` | `*link.ConsentAt`, `*link.AgeConfirmedAt` и `*link.NoticeShownAt` разыменовываются без проверки. Строка `consented` с NULL-отметкой (фикстура, ручная правка) вызовет панику, и вместо внятной ошибки клиент получит 500 от recover. | Проверить `nil` и явно ответить `500 internal` или возвращать ошибку инварианта из стора. | открыто |
| N-6 | Nit | `internal/gateway/store/helpers_test.go:18`; `internal/gateway/links/helpers_test.go:41`; `internal/gateway/context_test.go:36`; `cmd/multiverse/serve_test.go:150`; `test/e2e/empty_world_test.go:143` | Помощник «временный каталог SQLite с повторным удалением» скопирован в пять мест. | Бэклог: один помощник в `shared/testkit` (например, `testkit.SQLiteDir(t)`); тестам `cmd/**` и `test/e2e` импортировать testkit можно. | открыто |
| N-7 | Nit | `internal/gateway/links/forget_test.go:257-270` | `TestSweepFinishesAPendingCompaction` проверяет, что после `Sweep` скан = 0, но не проверяет, что ID был в файлах до `Sweep` (соседний тест такой контроль делает, `:212-214`). | После заблокированного `Forget` добавить контроль `occurrences(-wal)+occurrences(file) > 0`. | открыто |

### Предложения в бэклог (вне границ T-303)

1. **devops / tech-lead#1 (EPIC-001):**
   - `/data` в `build/Dockerfile` с владельцем `nonroot` и правами `0700`;
   - строка в runbook о пересоздании тома `gateway-data`;
   - `MV_GATEWAY_DATA_DIR` в `make replay` и в описании запуска `go run … --contexts=all` (`CLAUDE.md`, README).

   Это путь (б) из M-1, если выбран он.
2. **system-architect#1 и T-311:** проверка неоднозначного `/forget` через `Resolve` (`internal/gateway/client/client.go:141-146`, Mi-7 ревью T-301) противоречит SEC-04. `Resolve` создаёт связку и снова пишет внешний ID в `links.db` сразу после забвения. Нужен способ проверки без записи: например, `/forget` отвечает `deleted:true`, если этот вызов доделал отложенное сжатие, или появляется отдельный read-only статус. Решать вместе с C-08 v1.4 (`forget_incomplete`).
3. **system-architect#1 (C-08):** задать `maxLength` у `external_id` в запросах §1.2/§1.3. Сейчас в `links.db` можно записать строку до ~64 КиБ, а Telegram user id — не больше 20 цифр.
4. **T-356 / T-354:** поддерживаю п. 5 исполнителя (`adminForgetLink` на `ForgetByPlayer`, проверка `ci` у служебных маршрутов). В ту же строку — nolog-политика для `adminForgetLink`, если architect отнесёт `player_id` в запросе оператора к чувствительным данным.
5. **T-304 (`shared/testkit/gateway.Harness`):** соблюдать порядок `Routes → Start → serve`, иначе запрос до `Start` падает вне recover.

### Риски и допущения

- M-1 и часть Mi-5 выведены из чтения Dockerfile, compose и `open.go`, без запуска Docker (поручение его запрещает). Вывод о правах тома опирается на поведение Docker: пустой том получает права каталога образа, а если каталога в образе нет — root `0755`.
- Прогонов под `-race` не было (cgo выключен). Конкурентные места (`pending`, `polls`, заполнение `Config` в `Start`) разобраны чтением.
- Узкое окно, в котором параллельный успешный `Compact` снимает отметку `pending` (разбор п. 2, риск «г»), оставлено риском, а не замечанием.
- Решение system-architect#1 по `503 forget_incomplete` может изменить код или статус ответа. Тогда затрагиваются `errors.go:106-108`, `handlers/links.go:131-133`, `openapi_test.go:493`, описания `Unavailable` и `forgetLink`.

## T-303 · ревью #2 · 2026-09-13 · code-reviewer#1 (TEAM-3)

### Границы ревью

Итерация 2 developer#1 по ревью #1 (0/1/6/7), по решению system-architect#1 о `503 forget_incomplete` и по решению оркестратора о M-1. Рабочая папка `.worktrees/T-303`, база `fdc7e05`. Коммита нет, ревьюировались незакоммиченные изменения. По правилу повторной итерации проверены только исправления и регрессия от них.

Новое или изменённое в итерации:
- `build/Dockerfile`, `Docs/ops/runbook.md` (раздел 2), `README.md`, `.env.example` (только комментарий);
- `api/gateway.openapi.yaml`;
- `internal/gateway/{context.go,export_test.go,compaction_test.go,context_test.go}`;
- `internal/gateway/links/{forget.go,store.go,link.go,*_test.go}`;
- `internal/gateway/handlers/{links.go,links_test.go}`;
- `internal/gateway/api/middleware_test.go`;
- `internal/gateway/store/*_test.go`;
- новый пакет `shared/testkit/gateway/sqlitedir`;
- тесты EPIC-001 `cmd/multiverse/serve_test.go`, `test/e2e/empty_world_test.go`.

`shared/env/vars.go`, `Makefile`, `CLAUDE.md`, `go.mod` и `contexts.go` в итерации не менялись. Посторонних путей нет.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 1 · Nit: 3 (новые). N-3 из ревью #1 остаётся открытым как Nit и уходит в бэклог.

M-1 закрыт по чтению, сборку образа и `make up` на стенде ещё нужно подтвердить (риски). Условия system-architect#1 по 503 выполнены все. Mi-1…Mi-6, N-1, N-2, N-4…N-7 закрыты. Мутанты K1–K4 ревью #1 теперь красные, ревьюер перепроверил их сам. Новый Minor касается только тестов: ветка повторного чтения связки по `link_id` из исправления Mi-1 не закреплена тестом (мутант R1 зелёный), код при этом верен.

### Проверено ревьюером (go1.26.8 windows/amd64, cgo выключен — без `-race`)

| Проверка | Как | Результат |
|---|---|---|
| сборка и vet | `go build ./... && go vet ./...`; `go vet -tags e2e ./test/e2e/` | 0 / 0 |
| unit | `go test -short -count=1 ./internal/gateway/... ./shared/testkit/gateway/... ./cmd/multiverse/...` | 10 пакетов ok |
| e2e | `go test -tags e2e -count=1 ./test/e2e/...` | ok (19 с) |
| линтер | `golangci-lint run ./...` (первый запуск упал на «parallel golangci-lint is running»: подождал и повторил) | 0 issues |
| манифест | `go run ./cmd/mvctl env check` | 67 переменных, exit 0 |
| hadolint | не установлен | не запускался |
| Docker | запрещён поручением | образ, том и `make up` не проверялись |
| мутанты | копия дерева в scratch (`t303r2-*`, tar без `.git`, `services/`, `Docs/`, `bin/`), без `-overlay`, контрольный первым. Мутация — точная замена единственного вхождения, откат побайтно с проверкой. В конце `diff -rq` каталогов `internal cmd test api shared build` копии и папки задачи — совпадают. Копии удалены по точному сохранённому пути | таблица ниже |

| # | Мутант | Результат |
|---|---|---|
| R0 | контрольный: `/forget` при `ErrCompactionPending` отвечает `bus_unavailable` | **красный**: `TestLinkHandlersMapStoreErrors/forget_not_yet_wiped`, `TestAPendingCompactionIsVisibleUntilItIsFinished` |
| K1 | `Health` не смотрит `CompactionPending` | **красный**: `TestAPendingCompactionIsVisibleUntilItIsFinished`, `TestABlockedCompactionAtTheStartIsFinishedByTheSweeper` |
| K2 (R12) | `Stop` не доделывает отложенное сжатие | **красный**: `TestAPendingCompactionIsVisibleUntilItIsFinished`, `TestStopFinishesAPendingCompaction` |
| K3 | `limitBody` раньше `admitClient` | **красный**: `TestBodyLimitAndContentType/65_KiB_from_a_stranger` |
| K4 | `ContentLength >= BodyLimit` | **красный**: `TestBodyLimitAndContentType/exactly_64_KiB` |
| R10 | `player_id = ?` вместо `IS ?` в `DELETE` (связка без персонажа) | **красный**: `TestForgetOfAnAccountWithoutACharacter`, `TestTheContextServesTheLinksRoutes` и др. |
| R13 | неудача сжатия при старте валит `Start` | **красный**: `TestABlockedCompactionAtTheStartIsFinishedByTheSweeper` |
| R1 | повтор каскада ищет связку прежним `lookup`, а не по `link_id` | **зелёный** → R2-Mi-1 |
| R2 | часовое полное сжатие sweeper'а не выполняется | **зелёный** → R2-N-1 |
| R3 | сжатие при старте на контексте `Start`, без `WithoutCancel` | зелёный: не наблюдаемо (неудача ставит только отметку), не дефект |
| R4 | `Stop` сжимает, даже если sweeper не остановился | зелёный: не наблюдаемо в тестах, исполнитель это оговорил; не дефект |

### Разбор по пунктам поручения

**M-1 (Dockerfile и runbook).** Закрыт по чтению.
- В builder (`golang:*-bookworm`, root) выполняется `RUN mkdir -m 0700 /gateway-data` (`build/Dockerfile:24`).
- В runtime до `USER nonroot` стоит `COPY --from=builder --chown=65532:65532 --chmod=0700 /gateway-data /data` (`:45-46`). Числовой UID не требует `/etc/passwd`. `--chmod` требует BuildKit, а заголовок `# syntax=docker/dockerfile:1.7` уже есть. В distroless `/data` нет, поэтому BuildKit создаёт каталог и применяет к нему `--chown` и `--chmod`.
- Пустой именованный том `gateway-data:/data` без `nocopy` (`docker-compose.yml:273, 483`) при первом монтировании получает владельца и права каталога образа. Это открывает `prepareDir`/`checkMode` при пределе `0700`.
- Runbook, раздел 2 (`Docs/ops/runbook.md:143-175`). Названы признаки в логе и команда пересоздания; указано, что она для оператора и агенты её не выполняют. Имя тома берётся из `docker volume ls`, стоит запрет удалять том, в котором уже есть `links.db`. Утверждение «в томе `root 0755` данных быть не может» верно: store отказывает до создания файла. Там же описан запуск вне compose.
- Правка `build/Dockerfile` — файл EPIC-001; отметку даёт tech-lead#1 (решение оркестратора).

**Условия system-architect#1 по 503.** Выполнены все.
- 503 означает «связка удалена, стирание не подтверждено» и отличается от `bus_unavailable`. Это записано в `handlers/links.go:133-139`, в описании `forgetLink` и `Unavailable`, в сообщении кода (`errors.go`).
- Пока сжатие отложено, 503 получает любой `/forget`. Без связки это ветка `forget.go:87-93`. Со связкой `Compact` после `DELETE` снова падает (`:117-120`). Вызов, который доделал сжатие, отвечает как обычно. Проверено `TestAPendingCompactionIsVisibleUntilItIsFinished`: аккаунт без связки получает 503.
- `200 {deleted:false}` после 503 означает конец того же забвения: так в OpenAPI и в doc-комментарии `Forget`.
- `Retry-After: 5` выставляется до `WriteError` (`handlers/links.go:148-151`), закреплено `TestForgetIncompleteCarriesRetryAfter` и тестом контекста.
- В `Start` одно безусловное `linkStore.Compact(context.WithoutCancel(ctx))` (`context.go:121-123`). Оно идёт после миграций, но до заполнения `api.Config`/`handlers.Links` и до sweeper'а, то есть до обслуживания (сервер процесса стартует после `StartAll`). Неудача пишется в лог и ставит отметку, старт не падает (мутант R13 красный). `TestTheStartWipesWhatAStoppedProcessLeft` доказателен: контроль «ID в файле», replay без sweeper'а, после старта скан = 0, остающийся ID виден.

**Mi-1.**
- Реализовано по предложению: `DELETE … WHERE link_id = ? AND player_id IS ?` с `RowsAffected` (`forget.go:128-142`). `IS` верно сравнивает NULL (мутант R10 красный).
- При 0 строк связка перечитывается по `link_id`: исчезла → `{deleted:false}`, сменился персонаж → каскад заново. Всего не больше `forgetAttempts = 3`, дальше ошибка → 500 (`:82-123`).
- Тесты на смену персонажа, исчерпание попыток и параллельный `/forget` доказательны.
- Пробел: ветка «перечитать именно по `link_id`» не закреплена (R2-Mi-1).

**Mi-2.** Закрыт сжатием при старте (см. выше). Отметка по-прежнему живёт в памяти, и это теперь безопасно: любой рестарт начинается с полного сжатия.

**Mi-3.** Закрыт. `TestAPendingCompactionIsVisibleUntilItIsFinished` проверяет цепочку:
1. 503 и `Retry-After`;
2. контроль «след есть»;
3. `degraded` и `links_compaction: pending`;
4. 503 для чужого аккаунта;
5. уход читателя → `{deleted:false}`, скан 0, `ok`;
6. `Stop` под читателем сообщает «uncompacted».

`TestStopFinishesAPendingCompaction` работает в replay: sweeper'а нет, стирание может доделать только `Stop`, и контроль стоит до `Stop`. Мутанты K1/K2 красные.

**Mi-4.** Закрыт: `store.go:133-139` при неполной форме без связки ничего не вставляет. Обработчик получает нулевой `Link` и `ErrConsentIncomplete` и отвечает 400 до разыменования отметок. Тест `TestIncompleteConsentWithoutResolveCreatesNothing` проверяет `COUNT(*)` = 0.

**Mi-5.** Закрыт в границах задачи.
- Ошибка старта называет каталог и `MV_GATEWAY_DATA_DIR` и подсказывает, что делать (`context.go:113-114`), тест есть.
- README (`:195-202`) и комментарий в `.env.example` (`:126-130`) описывают запуск на хосте; `mvctl env check` — 0.
- `make replay` и строка `CLAUDE.md` остаются за EPIC-001 (бэклог исполнителя, п. 1).

**Mi-6.** Закрыт: `Link.MarshalJSON` → `"redacted"` (`link.go:78`), метод на значении, поэтому покрыт и `*Link`. Тест проверяет `Resolution` и `[]Link` в `slog.NewJSONHandler`, а также `json.Marshal` с `[]*Link`.

**Nit ревью #1.**
- N-1 закрыт: тесты «ровно 64 КиБ» (с `Content-Length` и chunked) и «65 КиБ от чужого → 403».
- N-2 закрыт текстом OpenAPI (замечание к формулировке общего ответа — R2-N-2).
- N-4 закрыт: `Stop` закрывает обе БД и при истёкшем дедлайне, `database/sql` дождётся занятого соединения.
- N-5 закрыт: `consented` без отметки → 500 без паники.
- N-6 закрыт: общий помощник вместо пяти копий.
- N-7 закрыт: контроль до `Sweep`.
- N-3 не менялся (вне поручения итерации), остаётся в бэклоге.

**`shared/testkit/gateway/sqlitedir`.**
- depguard. Правила с именем `shared-testkit-gateway` в `.golangci.yml` нет. К пакету применяется правило `shared` (запрет `internal/*`), а пакет импортирует только stdlib. Импортируют его только `_test.go`: поиск по не-тестовым файлам находит лишь сам пакет. На такие импорты действует исключение `_test\.go$` для `shared/testkit` из `no-testkit-in-production`. `golangci-lint run ./...` — 0.
- Владение. По `plan/ownership.md` в `shared/testkit/**` фейки лежат в подпакете поставщика, а `…/gateway` принадлежит EPIC-004. Отдельный подпакет на stdlib не тянет харнесс и не создаёт цикла, когда `testkit/gateway` начнёт импортировать `internal/gateway` (T-304).
- `os.MkdirTemp` создаёт каталог `0700`, это совместимо с `checkMode`.

**Правки тестов EPIC-001.** Минимальны. В `serve_test.go` одна строка `t.Setenv(env.GatewayDataDir…, sqlitedir.Temp(t))`, импорт и комментарий. В `empty_world_test.go` то же в `emptyWorldEnv`. Итерация 2 лишь заменила локальный помощник на `sqlitedir.Temp`.

**Остаточные риски исполнителя.**
- *Окно снятия отметки sweeper'ом.* Отметку ошибочно снимет только такой порядок: успешный checkpoint sweeper'а завершился, затем `DELETE` и неудачное сжатие параллельного `/forget` (которое ставит `true`), и лишь после этого sweeper выполняет `pending.Store(false)`. Все операторы идут через одно соединение `links.db`. Неудача под читателем занимает `busy_timeout` 5 с, так что на практике окно открыто только для неудачи по уже истёкшему дедлайну запроса, и то в пределах одного вытеснения горутины. Последствие не ноль: повтор клиента получит `200 {deleted:false}`, хотя кадры ещё в `-wal`, до часового сжатия или рестарта. Вероятность пренебрежимо мала. Согласен оставить риском с мьютексом «DELETE + сжатие» в T-306/T-314 (предложение 3 исполнителя).
- *`Retry-After` 5 с против паузы клиента ≈1,4 с.* Риск меньше, чем выглядит. Под удерживающим читателем каждая попытка сама ждёт на сервере `busy_timeout` (5 с, в пределах дедлайна запроса 5 с) и только потом отвечает 503. Четыре попытки `DefaultBackoff` растягиваются примерно до 20 с, а `DefaultHTTPTimeout` 35 с это покрывает. Быстрый 503 бывает только при исчерпанном хуками дедлайне. Бот после исчерпания повторов не говорит «удалено» (строка T-311). Решение, читать ли `Retry-After`, — T-456.
- Добавлю наблюдение. Пока читатель держит файл, каждый `/forget` и каждый тик sweeper'а занимают единственное соединение `links.db` до 5 с, и `resolve`/`consent` в это время ждут соединение в пределах своего дедлайна. При коротком бэкапе небольшого `links.db` это приемлемо; в эксплуатационные заметки T-456.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| M-1 | Major (ревью #1) | `build/Dockerfile:24, 45-46`; `Docs/ops/runbook.md:143-175` | Исправлено путём (а). | Подтвердить на стенде: `make image && make up && make health` → gateway `ok`. Отметка tech-lead#1 о правке файла EPIC-001. | закрыто (по чтению) |
| Mi-1…Mi-6 | Minor (ревью #1) | см. разбор | Исправлены. | — | закрыто |
| N-1, N-2, N-4…N-7 | Nit (ревью #1) | см. разбор | Исправлены. | — | закрыто |
| N-3 | Nit (ревью #1) | `internal/gateway/context.go:242-246` | `links_store`/`gateway_store` — константы `ok`. | Бэклог (п. 4 исполнителя). | открыто → бэклог |
| R2-Mi-1 | Minor | `internal/gateway/links/forget.go:106-115`; `internal/gateway/links/forget_test.go:142-227` | Повтор каскада перечитывает связку по `link_id`, и именно это делает исправление Mi-1 верным в двух случаях, которые тесты не проверяют (мутант R1: повтор тем же `lookup` — зелёный). (1) `ForgetByPlayer`: если хук вызвал `AttachPlayer` с другим персонажем, поиск по старому `player_id` не найдёт связку. Ответ будет `{deleted:false}`, а связка с внешним ID останется. (2) `Forget`: параллельный `/forget` удалил связку, затем `Resolve` создал новую для того же аккаунта. Поиск по внешнему ID удалил бы связку нового игрока. Код сейчас верен, но это ветка приватности, и `adminForgetLink` (T-356) смонтирует (1). | Два теста: (1) `ForgetByPlayer("player-A")`, хук делает `AttachPlayer(linkID, "player-B")` → `deleted:true`, `player_id_detached = player-B`, `COUNT(*) FROM links` = 0; (2) хук внешнего `Forget` вызывает вложенный `Forget` и затем `Resolve` того же аккаунта → внешний отвечает `{deleted:false}`, новая связка на месте. Можно в T-303 или строкой DoD в T-356. | открыто (не блокирует) |
| R2-N-1 | Nit | `internal/gateway/context.go:183-186` | Часовое полное сжатие sweeper'а (`LinksCompactInterval`, «страховка» ADR-019 доп. п. 1) не закреплено тестом: мутант R2 зелёный. После сжатия при старте последствие меньше, но при долгом аптайме и потерянной отметке (риск окна выше) это единственная страховка. | Тест live: оставить кадры удалённой строки в `-wal` без отметки (удаление через отдельное соединение, как в `TestTheStartWipesWhatAStoppedProcessLeft`), продвинуть `Manual` на `LinksCompactInterval` → скан 0. | открыто |
| R2-N-2 | Nit | `api/gateway.openapi.yaml:662-672` | Общий ответ `Unavailable` начинается словами «the action is not accepted and action_key is not stored», а для `forget_incomplete` верно обратное (связка удалена). Так как ответ общий (`:261`, `:325`), `forget_incomplete` и `Retry-After` формально становятся возможными и у операций, где их не бывает. | Для T-456 (system-architect#1): отдельный ответ `ForgetIncomplete` только у `forgetLink` со своими `x-error-codes` и `Retry-After`, либо первая фраза «для bus_unavailable/state_unavailable». Эталонный тест кодов это допускает. | открыто |
| R2-N-3 | Nit | `internal/gateway/context.go:197-232, 236-240` | `Stop` держит `c.mu` на всё ожидание sweeper'а и сжатие (до дедлайна `Stop` плюс `busy_timeout`), и `/health` процесса всё это время ждёт мьютекс. Это не регрессия итерации (ожидание было и раньше), но сжатие в `Stop` удлинило окно. | Снимать `started` и копировать ссылки под мьютексом, а ждать и сжимать вне него; либо принять и отметить в комментарии. | открыто |

### Предложения в бэклог

1. **T-356:** строкой DoD — тест R2-Mi-1 (1) для `adminForgetLink` на `ForgetByPlayer`, если в T-303 его не добавят.
2. **T-456 (system-architect#1):** R2-N-2 (отдельный ответ `ForgetIncomplete`). Там же зафиксировать, что под удерживающим читателем каждая попытка `/forget` ждёт до `busy_timeout` и занимает единственное соединение `links.db`.
3. **tech-lead#1 (EPIC-001):** отметка о правке `build/Dockerfile`; `MV_GATEWAY_DATA_DIR` в `make replay` и в `CLAUDE.md` (п. 1 исполнителя); строка `CLAUDE.md` «gateway — заглушка» устарела.
4. **T-306/T-314:** мьютекс «DELETE + сжатие» или счётчик поколений отметки (п. 3 исполнителя) — поддерживаю.

### Риски и допущения

- M-1 закрыт чтением Dockerfile, compose и поведения BuildKit/Docker: `COPY --chown/--chmod` применяется к создаваемому каталогу назначения, пустой именованный том копирует владельца и права каталога образа. Образ не собирался, hadolint не установлен, `make up` не запускался (запрет поручения). До пересоздания старого тома `gateway-data` по runbook gateway в compose на машине владельца не стартует.
- Прогонов под `-race` не было (cgo выключен). Конкурентность `pending`, `Stop` и sweeper'а разобрана чтением.
- Мутанты R3 и R4 зелёные, но замечаниями не считаются: их поведение тестами не наблюдаемо и дефекта не образует.

## T-304 · ревью #1 · 2026-09-13 · code-reviewer#1 (TEAM-3)

### Границы ревью
- Ветка `task/T-304-readmodel-consumer` (`.worktrees/T-304`), база `83b9351` (`epic/EPIC-004-gateway-bot`), изменения не закоммичены: `internal/gateway/readmodel/**`, `internal/gateway/consumer/**`, `internal/gateway/context.go`, тесты `internal/gateway/{bus,projection,context,compaction,export}_test.go`; карточка, строка индекса и запись dev-log T-304.
- Сверено с: раздел «### T-304» в `tasks.md` (DoD), карточка «Выполнение», КД `gateway-and-bot.md` §4.2, §6, §8.1, §11.2, §11.4; `contracts.md` C-01 v1.5–v1.7, C-02 v1.5/v1.5a/v1.6 (форма `changed[]` и правило догона), C-14 (снапшот, курсор — следующий офсет); `state-and-mechanics.md` §4.4; код `shared/entity` (`ApplyOps`, `changeTracker.changes`, `CanonicalJSON`), `shared/eventbus` (`Delivery`, kafka `ReadRange`/`End`/`Subscribe`, membus), `shared/objstore/minio.go`, kafka-go v0.4.51 `reader.initialize`.
- Файлы вне `internal/gateway/**` и документов EPIC-004 не менялись; карта владения не нарушена.

### Вердикт
**Принять.** Critical 0 · Major 0 · Minor 7 · Nit 4.

Транзакция consumer, два курсора, дедуп, догон и остановка сделаны верно. Тесты содержательные, мутанты исполнителя подтверждаются. Замечания:
- ложное утверждение о форме v1.5, и тест его прячет;
- неполная оговорка о захвате перехода встречи после рестарта;
- четыре непокрытые ветки (мои мутанты R1–R4 зелёные);
- молчаливый `missing` при неверной настройке хранилища;
- старт без срока.

Minor рекомендую закрыть до слияния или перенести явным решением оркестратора.

### Проверено ревьюером (go1.26.8 windows/amd64, cgo выключен — без `-race`)
- `go build ./... && go vet ./...` — 0.
- `go test -short -count=1 ./internal/gateway/... ./cmd/multiverse/...` — все `ok` (`cmd/multiverse` 17,5 с).
- `go test -tags e2e -count=1 ./test/e2e/...` — `ok` (12,5 с). `Start` с обязательными `Deps.Bus`/`Journal` не ломает `serve` и e2e: процесс всегда передаёт шину (`cmd/multiverse/serve.go`).
- `golangci-lint run ./...` — 0 issues.
- Мутанты и зонды — в копии дерева `scratchpad/t304rev-tree` (tar без `.git`, `services/`, `Docs/`, `bin/`), без `-overlay`. Каждый — точная замена единственного вхождения с побайтовым восстановлением. Копия удалена по точному пути.

| # | Мутант / зонд | Результат |
|---|---|---|
| K0 | контрольный: `resolves` всегда `false` | **красный** (`TestBothOrdersOfTheEndCloseTheSameEncounter`, `TestARetryOfTheClaimingEventReportsTheEndAgain`) |
| R1 | `Health` не смотрит `consumer.Err()` (`context.go:349`) | **зелёный** → Mi-3 |
| R2 | `Sweep` без удаления по возрасту (`dispatcher.go:276`) | **зелёный** → Mi-5 |
| R3 | sweeper контекста не вызывает `consumer.Sweep` (`context.go:260`) | **зелёный** → Mi-5 |
| R4 | v1.6: элемент без `new` для отсутствующего пути — не no-op (`apply.go:253`) | **зелёный** → Mi-4 |
| Z1 | зонд: `entity.ApplyOps(remove encounter_id)` → факт в форме v1.5 (`old`/`new` как пишет `entity.Change`) → хеш проекции против хеша State | **расходится**: `sha256:14b0…` против `sha256:23a0…` → Mi-1 |
| Z2 | зонд: сущность встречи `state=resolved` уже в проекции (как после снапшота), затем `encounter.ended` | `EncounterEnded = "encounter-1"`: конец захвачен второй раз → Mi-2 |

### Разбор по пунктам поручения

**1. Две формы `changed[]`.**

Для v1.6 правило «`new` есть — записать, нет — удалить» реализовано точно по C-02 v1.6:
- `presence` различает отсутствующий ключ и `null`;
- `a[n]` при `n = len` — добавление без дедупликации;
- `n > len` — `ErrCorruptFact`;
- удаление отсутствующего пути — no-op, но без теста (Mi-4).

Для v1.5 неоднозначность есть, и по самому факту её не устранить:
- `entity.Change` пишет `old` и `new` всегда. `changeTracker.changes` (`shared/entity/ops.go:408-418`) для удалённого ключа даёт `new: null` — так же, как для `set` в `null`.
- Проекция оставляет `null`, State ключ удалил, а `CanonicalJSON` различает `"k":null` и отсутствие ключа. Хеши расходятся (зонд Z1).
- Утверждение «State v1.5 хеширует так же» (`apply.go:87-88`, решение 1 карточки, dev-log) неверно.
- Тест `TestChangedOfBothFormsReproducesTheStateOfState` пропускает для v1.5 именно этот шаг (`apply_test.go:76-79`).
- `TestARemovedKeyInEitherForm` сверяется не с `ApplyOps`, а с ожиданием, которое сам построил (`apply_test.go:134-138`, `want.Attributes["encounter_id"] = nil`).

На чтение через геттеры это не влияет: `null` читается как отсутствие. Но до слияния T-448 `Hash()` расходится с State после любого удаления ключа. А T-309 положит этот хеш в снапшот gateway как `projection_hash`.

**2. Порядок версий, `stale`, идемпотентность, догон → подписка.** Сделано верно.
- Факт с версией ≤ известной ничего не меняет. Разрыв версий или неизвестная сущность — `stale` с `Warn`.
- Догон читает `[курсор снапшота, End)`. Подписка группы начинает с зафиксированного офсета группы: у kafka новая группа — с `FirstOffset`, у membus — с 0. Поэтому события, пришедшие во время догона, не теряются.
- Повтор уже применённого события — no-op в проекции, для эффектов его отсекает `behindCursor`.
- Курсор снапшота — «следующий офсет» (`state-and-mechanics.md` §4.4), `Advance` пишет `pos+1`: семантика одна.
- Курсор снапшота старше retention (30 дней) не вешает догон: kafka-go сам поднимает офсет до начала лога (`reader.initialize`, `offset < first → first`). Проекция при этом станет `stale`.
- `stale` не снимается до рестарта (N-1).

**3. Конец встречи.**

В одном процессе сделано верно:
- первое событие пары захватывает переход по своему id, второе ничего не сообщает;
- повтор захватившего события сообщает переход снова;
- `encounter.started` после конца встречу не открывает;
- оба порядка покрыты тестами.

Строка риска для T-351 остаточный риск рестарта **не исчерпывает** (Mi-2):
- Захват живёт в памяти. После рестарта второе событие пары снова даёт `EncounterEnded` (зонд Z2).
- С открытием то же. Сущность встречи из снапшота открыта, `encounter.started` из `world_events` приходит после рестарта и снова даёт `EncounterOpened`. Эффект T-307 «доставка `encounter.started` по `Result.EncounterOpened`» (предложение 3 исполнителя) даст вторую доставку.
- Документ `Result` (`apply.go:25-31`) обещает то, что держится только в пределах процесса.

**4. Транзакция consumer и sweeper.** Сделано верно.
- Проверка `processed_events` → эффекты → отметка → курсор — одна транзакция на единственном соединении, отметка пишется после эффектов.
- Ошибка эффекта возвращается из `Handle`, своего `recover` нет.
- Событие с офсетом ≤ курсора эффектов идёт только в проекцию.
- При отмене подписки `BeginTx` получает отменённый контекст. `Delivery` возвращает `ctx.Err()` без dead letter, и событие будет выдано снова.

Sweeper не удалит строки, нужные для дедупа при долгом догоне:
- `processed_at` — время обработки (`Clock.Now`), а не время события, поэтому строки догона свежие.
- Повтор с тем же офсетом закрывает курсор эффектов, независимо от окна.
- Окно защищает только от повтора с новым офсетом (повторная публикация), а такой повтор приходит в пределах секунд.

Остаются два пункта:
- В replay `processed_at` штампуется ручными часами (нулевое время), и первый sweep в `live` после перехода сотрёт всё окно (в бэклог T-309).
- Ветка «по возрасту» и вызов из sweeper'а тестами не закреплены (Mi-5).

**5. Waiters.** Своих горутин нет, таймер — `clock.Timers`. Любой выход снимает регистрацию и останавливает таймер. Отказ сопоставляется только по `proposal_id`. `PendingWaiters` = 0 проверяется на каждом выходе. Две оговорки (N-2):
- утечка возможна только у вызывающего, который сделал `Expect`, но не вызвал ни `Wait`, ни `Cancel` (например, при ошибке публикации): такое ожидание висит до факта той же корреляции;
- повторный `Wait` блокируется навсегда на остановленном таймере.

**6. `/health`.** Состояния `ok|missing|stale` и `projection_error` работают. `bus: fail` при упавшей подписке тоже работает, но без теста на уровне контекста (Mi-3).

Отклонение от §11.4 (`missing` без снапшота не деградирует) приемлемо для процесса без хранилища: `--bus=memory`, e2e EPIC-001. Для процесса с настроенным MinIO довод слабее. После `mvctl world init` снапшот seq 0 есть всегда (§4.10), поэтому `missing` там означает неинициализированный мир или неверный `MV_WORLD_ID` — то самое «пустое состояние молча» из US-011. Сейчас туда же молча попадают полузаданные ключи и неверный `MV_MINIO_USE_SSL` (Mi-6). Вопрос для architect#3 — ниже.

**7. `WithoutCancel` и секреты.** Секретов в логах и `/health` нет: `objstore.New` называет только имена переменных, ошибки minio-go — бакет, ключ и адрес.

Старт идёт без срока (Mi-7):
- `context.WithoutCancel` снимает и отмену, и дедлайн.
- `Journal.End` (kafka) берёт дедлайн соединения только из `ctx`, поэтому `ReadLastOffset` и `FetchMessage` догона могут висеть на брокере, который принял TCP и молчит.
- SIGTERM во время старта его не прерывает. HTTP-сервер процесса ещё не слушает (`serve.go`: `StartAll` до `srv.Start`), так что и `/health` недоступен.
- При заданных `MV_MINIO_*` и недоступном MinIO старт растягивается на повторы minio-go и `objstore`.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Mi-1 | Minor | `internal/gateway/readmodel/apply.go:83-92`; `readmodel/apply_test.go:76-79, 116-140`; карточка, решение 1; dev-log T-304 | Утверждение «под v1.5 удалённый ключ остаётся `null`, и State v1.5 хеширует так же» неверно (зонд Z1). Тест сверки хеша пропускает для v1.5 единственный расходящийся шаг, а соседний тест сверяется с ожиданием, которое построил сам. | Код не менять: v1.5 неоднозначна по построению, а выбор «оставить `null`» разумен. Исправить комментарий, решение 1 и dev-log: «под v1.5 удаление ключа и `set null` неразличимы; проекция хранит `null`; до слияния T-448 `Hash()` расходится с State после удаления ключа». Тест v1.5 переписать честно: шаг удаления не пропускать, а утверждать известное расхождение (или сверять хеш только для v1.6, а для v1.5 отдельно проверять геттеры). Строка в бэклог T-309: сверка `projection_hash` валидна только на фактах v1.6. | открыто |
| Mi-2 | Minor | `internal/gateway/readmodel/apply.go:25-31, 347-372`; карточка, предложения 3 и 5 | Захват перехода встречи живёт в памяти. После рестарта второе событие пары снова даёт `EncounterEnded`, а `encounter.started` после сущности из снапшота снова даёт `EncounterOpened` (зонд Z2). Пункт DoD «второе событие пары повторного эффекта не даёт (доставки)» держится только в пределах процесса. Строка риска упоминает только T-351. | Минимум: в документе `Result` написать, что переход уникален только в пределах процесса и эффект обязан быть идемпотентным по `encounter_id` в `gateway.db`; строку риска распространить на T-307 (доставка `encounter.started`, доставки конца). Лучше — фильтр в `consumer.commit`: `INSERT OR IGNORE` в таблицу переходов `(encounter_id, kind)` той же транзакцией, а при чужом `event_id` обнулять поле `Result` до вызова эффектов. Тогда T-307 и T-351 не будут повторять это каждый у себя; миграция — владение EPIC-004. Выбор — за tech-lead#3. | открыто |
| Mi-3 | Minor | `internal/gateway/context.go:349-352` | Упавшая подписка (`bus: fail`, `degraded`) — единственный сигнал, что gateway больше не слышит топик, но на уровне контекста он не проверен. Мутант R1 зелёный: `TestStopEndsTheSubscriptionsAndAFailedOneIsReported` проверяет только `Dispatcher.Err`. | Тест в `projection_test.go`: шина-обёртка, у которой `Subscribe` одного топика возвращает ошибку → `Health` = `degraded`, `details.bus = fail`. | открыто |
| Mi-4 | Minor | `internal/gateway/readmodel/apply.go:252-256` | Правило догона v1.6 «если пути уже нет, ничего не делать» не закреплено тестом: мутант R4 зелёный. Без этой ветки факт с элементом удалённого пути под созданным предком (C-02 v1.6) уйдёт в `ErrCorruptFact` → dead letter → проекция `stale`. | Тест: факт v1.6 с элементом `{path: "fresh.deep", old: …}` без `new` для отсутствующего пути и элементом `{path: "fresh", new: {}}` применяется без ошибки, а хеш совпадает с `ApplyOps` для `inc fresh.deep` + `remove fresh.deep`. | открыто |
| Mi-5 | Minor | `internal/gateway/consumer/dispatcher.go:276`; `internal/gateway/context.go:260` | `TestSweepForgetsOldMarksAndKeepsTheWindowBounded` не отличает удаление по возрасту от ограничения по числу: `old` и `young` уходят обоими `DELETE` (мутант R2 зелёный). Вызов `consumer.Sweep` из sweeper'а контекста тоже не проверен (R3 зелёный), а без него `processed_events` растёт на каждое событие четырёх топиков. | Тест возраста с числом строк ниже `ProcessedEventsMax`: одна старая и одна молодая отметка → остаётся молодая. Тест контекста в `live`: вставить старую отметку, продвинуть `Manual` на `SweepInterval` → отметки нет. | открыто |
| Mi-6 | Minor | `internal/gateway/context.go:181-184, 203-220`; `readmodel/bootstrap.go:77-79` | Ошибка настройки хранилища неотличима от нового мира. Неверный `MV_MINIO_USE_SSL`, пустой `MV_MINIO_ENDPOINT` при заданных ключах или один ключ без другого дают `store = nil` → `ErrNoSnapshot` → `projection: missing` и `/health ok`. Остаётся лишь строка `Error` в логе, что нарушает US-011 «не поднимается с пустым состоянием молча». | Ошибку `objectStore()` и полузаданные ключи считать несостоявшейся загрузкой: `projection_error` и `degraded`. `missing` без деградации оставить только для случаев «ключей нет совсем» и «указателя нет». Тест на `MV_MINIO_USE_SSL=maybe` через `env.MapSource`. | открыто |
| Mi-7 | Minor | `internal/gateway/context.go:138, 143` | Загрузка снапшота и догон идут под `context.WithoutCancel` без срока. SIGTERM во время старта их не прерывает, дедлайна нет ни у `End`/`ReadRange` (kafka), ни у `Get` (MinIO), а `/health` процесса не слушает до конца `StartAll`. В карточке риск назван, но не ограничен. | `context.WithTimeout(context.WithoutCancel(ctx), budget)` на загрузку и на догон, например 60 с (константа рядом с `runtime.StopTimeout`). Это совместимо с `dispatch_test` (там на входе отменённый контекст) и превращает зависание в ошибку старта с понятным текстом. Ещё лучше — отдельные бюджеты: превышение на загрузке даёт `missing` с `projection_error`, на догоне — ошибку старта. | открыто |
| N-1 | Nit | `internal/gateway/readmodel/apply.go:205-214`; `model.go:375-386` | `stale` необратим до рестарта: одного разрыва (dead letter повреждённого факта, факт чужого мира) хватает, чтобы gateway остался `degraded` навсегда. §11.4 определяет `stale` иначе. | Вопрос для architect#3 (ниже). Как вариант — снимать признак сущности, когда пришёл её факт, следующий за версией из свежего снапшота State, или после догона до `End` без разрывов. | открыто → architect#3 |
| N-2 | Nit | `internal/gateway/readmodel/waiters.go:41-46, 72-106` | `Expect` без `Wait`/`Cancel` оставляет регистрацию до факта той же корреляции. Повторный `Wait` блокируется навсегда: таймер остановлен, `done` пуст. | В документ `Expect`: «при ошибке публикации — `Cancel`» (пример `defer w.Cancel()`). `Wait` после завершения — сразу `ErrWaitUsed` или прежний результат. | открыто |
| N-3 | Nit | `internal/gateway/readmodel/model.go:304-326` | `EncounterOf` при каждом вызове собирает и сортирует id всех встреч, а встречи и сущности из проекции не удаляются. На предусловиях каждого действия T-305 это O(n log n) по всем встречам за жизнь процесса. | В бэклог T-305/T-351: индекс «игрок → открытая встреча» или вытеснение закрытых встреч из `encounters`. | открыто → бэклог |
| N-4 | Nit | `internal/gateway/context.go:342-344` | `projection_error` отдаёт в `/health` сырой текст ошибки: бакет, ключ, адрес MinIO, текст minio-go. | Если `/health` доступен вне сети compose, отдавать короткий код (`snapshot_unreadable`, `hash_mismatch`, `world_mismatch`), а полный текст писать только в лог. | открыто |

### Для architect#3
1. **§11.4, `missing`.** Предлагаю правило:
   - без клиента хранилища (нет ключей MinIO) `missing` не деградирует — это процесс на `--bus=memory` и e2e;
   - при настроенном хранилище отсутствие указателя — `degraded` с причиной `no_snapshot`: после `mvctl world init` seq 0 есть всегда (`state-and-mechanics.md` §4.10).

   Если healthcheck compose считает `degraded` провалом, оставить как у исполнителя, но зафиксировать это в §11.4.
2. **§11.4, `stale`.** Определение исполнителя (разрыв версий) полезнее, чем «> 60 с без событий при активных сессиях», но ему нужно правило снятия (N-1). Предлагаю также учитывать разрыв журнала (`курсор < начала лога`, аналог `log_gap` State §4.4).
3. **§6.** `Apply` → `(Result, error)` и `Expect`/`Wait`. Переходы `Result.EncounterOpened/Ended` уникальны только в пределах процесса (Mi-2).

### Предложения в бэклог (вне границ T-304)
1. **T-307:**
   - доставки по `Result.EncounterOpened/Ended` сделать идемпотентными по `encounter_id` в `gateway.db` (или через фильтр consumer из Mi-2);
   - первый старт с пустым `gateway.db` против непустого журнала: новая группа kafka читает все четыре топика с `FirstOffset`, курсора эффектов нет, и эффекты сработают для всей истории (старые доставки). Нужен начальный курсор эффектов, например `End` при первом старте.
2. **T-309:**
   - в replay `processed_at` штампуется ручными часами (нулевое время), и первый sweep после перехода в `live` сотрёт всё окно. Нужен штамп по времени события или sweep только отметок `live`;
   - сверка `projection_hash` — только на фактах v1.6 (Mi-1);
   - догон `game_events`/`world_events`/`narrative_output` при старте: сейчас догоняется только `system_events`, и `Round` встречи после рестарта теряется.
3. **T-316:** в integration-тест consumer на Redpanda добавить:
   - сценарий «брокер принял соединение и молчит» на старте (Mi-7);
   - повторную подписку после ошибки `Subscribe`: сейчас упавший топик до рестарта не переподписывается.
4. **T-305:** индекс открытой встречи игрока (N-3).

### Риски и допущения
- `-race` не запускался (cgo выключен). Конкурентность `Model`, `Dispatcher` и ожиданий разобрана чтением: обработчики четырёх подписок сериализуются единственным соединением `gateway.db`, `Model` защищён мьютексом.
- Поведение kafka-адаптера при молчащем брокере (Mi-7) выведено из кода kafka-go v0.4.51 и `kafka.go`. На Redpanda не проверялось: интеграционные тесты запрещены поручением.
- e2e и `serve` читают окружение процесса. Если в оболочке заданы `MV_MINIO_*`, а MinIO не запущен, gateway на `--bus=memory` стартует медленнее (повторы) и отвечает `degraded`. Сейчас это не воспроизводится: Makefile не экспортирует `.env`.
- Зонды Z1/Z2 и мутанты R1–R4 выполнялись только в копии дерева. В рабочей папке задачи ревьюер добавил только этот раздел и строку в карточке.

## T-305 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-3)

### Границы ревью
- Ветка `task/T-305-actions` (`.worktrees/T-305`), база `abfc04b` (`epic/EPIC-004-gateway-bot`, слияние T-304). Изменения не закоммичены:
  - новые: пакет `internal/gateway/actions/**`, `internal/gateway/handlers/actions{,_test}.go`, `internal/gateway/actions_test.go`;
  - правки: `internal/gateway/{context,context_test}.go`, `api/{middleware,router,openapi_test}.go`, `readmodel/{model,apply,encounter_test}.go`, `api/gateway.openapi.yaml`;
  - файлы EPIC-001: `shared/env/vars.go`, `.env.example`;
  - документы: карточка, строка индекса и запись dev-log T-305.
- Сверено с:
  - раздел «### T-305» и DoD-common в `tasks.md`; карточка, раздел «Выполнение»;
  - КД `gateway-and-bot.md` §1, §4.2, §5.1, §5.2, §5.4, §5.5; `analysis/api-contracts.md` §1.1, §1.4, §1.6, §2.3.1;
  - `contracts.md` v0.14 из `.worktrees/EPIC-004`: C-01 v1.9 (время корней, `replayClock`), C-04 v1.2–v1.4, C-08 v1.1–v1.5; строка DoD T-305 в `EPIC-001-foundation/tasks/T-456.md`;
  - ADR-018 (форма `action_key`), ADR-019 (схема `gateway.db`; новой миграции нет);
  - код T-303/T-304 (`api`, `handlers`, `links`, `readmodel`), клиент `internal/gateway/client` (политика повторов), `shared/eventbus` (`kafka.Publish`, `membus.Publish`);
  - as-is `services/narrative-orchestrator` (`CreateGM`) и `services/game-service` (форма `gm.created`).
- Карта владения не нарушена. Вне `internal/gateway/**`, `api/**` и документов EPIC-004 изменены только объявления в `shared/env/vars.go` и `.env.example` — мягкий режим по DoD-common.

### Вердикт
**Вернуть.** Critical 0 · Major 1 · Minor 5 · Nit 5.

Сделано аккуратно:
- валидация и порядок проверок, идемпотентность `4xx`, лимит, фильтр;
- мост `gm.created`, предложения без `scope`, OpenAPI;
- текст игрока и ключ не попадают в лог, это проверено тестом;
- всё проверяется до публикации;
- мутанты исполнителя подтверждаются.

Возврат из-за одного Major (Ma-1). Когда действие уже ушло в шину, повтор того же `action_key` публикует второе, другое действие. Так бывает в двух случаях: не прошла следующая публикация или отменён контекст запроса. Эталонный клиент повторяет `503` и сетевые ошибки сам. Зонды P1–P3 это воспроизводят.

Остальное — пять пробелов в тестах (мои мутанты зелёные) и мелочи.

### Проверено ревьюером (go1.26.8 windows/amd64, cgo выключен — без `-race`)
- `go build ./... && go vet ./...` — 0.
- `go test -short -count=1 ./internal/gateway/... ./shared/env/...` — все `ok`.
- `golangci-lint run ./internal/gateway/...` — 0 issues.
- `go run ./cmd/mvctl env check` — 71 переменная, `.env.example` сверен. `contracts check` — 65 типов, 8 топиков, 58 схем.
- Мутанты и зонды — в копии дерева `scratchpad/t305r1-mut` (tar без `.git`, `services/`, `.claude/`, `.qwen/`), без `-overlay`, контрольный первым. Каждый мутант — точная замена единственного вхождения с побайтовым восстановлением. Копия удалена по точному пути.
- Проверка слияния — в `scratchpad/t305r1-merge`: `git merge-file` по базе `abfc04b`, рабочей копии T-305 и кончику эпика `c92eb22` с нормализацией CRLF; для `dev-log.md` — дополнительно локальный драйвер `appendtail`. Копия удалена по точному пути.

| # | Мутант / зонд | Результат |
|---|---|---|
| M0 | контрольный, без изменений (`actions`, `gateway`, `handlers`, `readmodel`, `api`) | зелёный |
| R1 | под ключом сохраняется и `5xx` (`service.go:189`) | **красный** (`TestNotImplementedIsNotKept`) |
| R2 | `actor_kind` события всегда `human` (`publish.go:70`) | **зелёный** → Mi-1 |
| R3 | текст фильтра не перепроверяется `CheckText` (`service.go:180`) | **зелёный** → Mi-5 |
| R4 | граница минуты `!After` → `Before` (`ratelimit.go:130`) | зелёный, эквивалентный: при `wait = 0` действие и так принимается |
| R5 | grace включительно, `>` → `>=` (`validate.go:257`) | **красный** (`…/encounter_without_agent_within_the_grace`) |
| R6 | sweeper контекста не вызывает `Limiter.Sweep` (`context.go:377`) | **зелёный** → Mi-5 |
| R7 | `character_dead` только для `dead`, не для `abandoned` (`validate.go:161`) | **зелёный** → Mi-2 |
| R8 | `Retry-After` округляется вниз (`ratelimit.go:93`) | **зелёный** → Mi-4 |
| R9 | повтор сохранённого `4xx` теряет `details` (`service.go:249`) | **зелёный** → Mi-3 |
| P1 | зонд: предложение `enter` падает один раз, затем повтор того же ключа | `503`, затем `202`; в шине два `player.entered_region` на один ключ, id `ev-1` и `ev-2` → Ma-1 |
| P2 | зонд: контекст запроса отменён сразу после публикации `player.*` | `503 bus_unavailable`, повтор → `202`, два `player.entered_region` → Ma-1 |
| P3 | зонд: контекст отменён после последней публикации | `202`, но ключ **не сохранён** (`Keys.Save` на отменённом контексте); повтор опубликует действие заново → Ma-1 |

### Оценка отступлений исполнителя

1. **Лимит — ведро с burst 5 плюс журнал 30 принятий за минуту. Принять.**
   - Чистое ведро 30/мин с ёмкостью 5 пропускает 35 действий за первую минуту. DoD требует `429` на 31-е.
   - Реализация строже текста и выполняет оба требования: GCRA без дробных жетонов, журнал ограничен `perMinute`, всё под одним мьютексом, гонок нет.
   - Текст «token bucket» в КД §5.1 п. 6, C-08 v1.1 и `api-contracts.md` §1.1 нужно привести к коду — вопрос для architect#3 ниже.
   - Непокрытое округление `Retry-After` — Mi-4.
2. **`group.*` → `501` до T-352, ответ не сохраняется. Принять.**
   - Сохраняются только `4xx` (R1 красный).
   - `501` добавлен в OpenAPI у `postAction`. В §1.6 этот код есть только у `/stream`; снять его — строка DoD T-352.
   - `group.join` с несуществующей целью получает `404 unknown_target`, и ответ сохраняется. Это шаг 1 КД §5.4.
3. **`Turns` и `MemoryTurns` как точка вставки T-306. Принять.**
   - `Begin` ничего не хранит, поэтому неудачная публикация хода не занимает (есть тест).
   - Гонка номеров в `Begin`/`Accepted` в I1 невозможна: `scope` соло — это сам игрок, а его запросы идут строго по одному под блокировкой игрока.
   - Сессии в памяти не заканчиваются; заменяет T-306.
4. **`readmodel.Encounter.TaskAgentID`, `CreatedAt`. Принять.**
   - Поля добавочные, заполняются в `mergeEntity` из сущности.
   - Поведение T-304 не меняется: тесты `readmodel` зелёные, новый тест проверяет оба поля. Владение EPIC-004.
   - Правка КД §6 — вопрос architect#3, исполнитель его уже задал.
   - Крайний случай нулевого `CreatedAt` — N-2.
5. **Повтор принятого ключа расходует жетон и может получить `429`. Принять как риск, в бэклог.**
   - Порядок middleware задан КД §5.1: `ratelimit` стоит до обработчика.
   - Клиент `429` не повторяет (`client.go:317-323`). Следующий повтор после `Retry-After` получит сохранённый `202`.
   - Цена: сетевая ошибка внутри серии из 5 действий может показать «слишком часто» на уже принятое действие.
6. **Частичная публикация — не принятый риск, а Major (Ma-1).**
   - Шлюз **знает**, что `player.*` уже в шине, и отвечает «не принято». А C-08 гарантирует: `503 bus_unavailable` — действие не принято, второго хода нет (`api-contracts.md` §1.1).
   - Повтор не зависит от пользователя: `internal/gateway/client` сам повторяет каждый `503` и каждую сетевую ошибку тем же телом (`DefaultBackoff`, 3 раза).
   - Окно шире, чем «между двумя `Publish`». Публикация идёт на контексте запроса, а `net/http` отменяет его при разрыве соединения клиентом — ровно тогда, когда клиент будет повторять. Ещё его режет дедлайн 5 с middleware `timeout`, то есть медленный брокер — именно при деградации.
   - Последствие не косметическое. Второй `player.entered_region` с новым id — второй повод к спавну встречи (`entity.create.proposed encounter` у агента встречи на это событие; C-05, форма `FakeEncounter`) и второй нарратив `entry`.
   - Исправление укладывается в задачу и контракт не меняет (см. Ma-1). Слова «устранимо только транзакционной шиной» в карточке («Риски») неверны.
7. **`actor_kind` из `X-Actor-Kind`. Принять до T-306.**
   - Значение доверенное: `admitClient` пропускает `human` по умолчанию, `ci`/`sim` — только клиентам из `MV_GATEWAY_ACTOR_KIND_CLIENTS`, прочее — `400`.
   - C-04 требует `actor_kind` сессии; это делает T-306, пункт уже есть в бэклоге исполнителя.
   - Путь не покрыт тестом — Mi-1.
8. **Переменные `MV_GATEWAY_*` в файлах EPIC-001. Принять; конфликтов при finish не будет.**
   - С базы `abfc04b` обе стороны меняли `.env.example`, `shared/env/vars.go`, `tasks.md` и `dev-log.md`. Кончик эпика `c92eb22` — это T-310 и слияние develop с T-456.
   - `vars.go` и `.env.example` сливаются трёхсторонне без конфликтов: T-305 добавляет строки в блок gateway, T-310 — `MV_TELEGRAM_*` в блок бота. `tasks.md` — тоже без конфликтов.
   - `dev-log.md` при простом `merge-file` конфликтует: обе стороны дописали хвост. Драйвер `appendtail` (`.gitattributes`, `.git/merge-drivers/append_tail.py`) разрешает это чисто — секция T-305, затем T-310. Условие — драйвер настроен в клоне, где делается finish.
   - Рабочая копия для проверки нормализована из CRLF. Без этого `merge-file` даёт ложный конфликт на весь файл; git при слиянии нормализует сам.

### Разбор по пунктам поручения

**Корректность и безопасность.**
- Порядок `Validate` совпадает с КД §5.4; шаги «группа» и «раунд» — I2.
- `unknown_target` на шаге 1 — «ни одной сущности с таким id», на шаге 4 — регион другого мира или NPC вне встречи. `target_dead` раньше `unknown_target` для NPC вне встречи — буква порядка §5.4 (встреча раньше региона), тест есть.
- Текст `say`: `TrimSpace`, UTF-8, 1–500 рун, без управляющих символов; затем фильтр, и в шину уходит результат фильтра.
- Лог:
  - запись фильтра содержит `text_len` без текста;
  - ошибка фильтра не пишется;
  - тип действия вне словаря пишется как `unknown`;
  - ключа в логе нет;
  - access log не пишет тело (`requestLog`);
  - `err.Error()` из `Publish` текста игрока не несёт: kafka/membus возвращают маршрут, `marshal` или транспорт, а схема `player.said` к этому моменту уже проверена `CheckText`.
- Всё проверяется до публикации, `Turns.Begin` ничего не хранит.
- `4xx` сохраняется. `5xx`, включая `501`, — нет (R1 красный). `429` и `413` отвечают до обработчика и не сохраняются.
- `gm.created` публикуется первым. Payload совпадает с формой as-is `game-service`, кроме `scope_type`: `solo` вместо `player`. `CreateGM` при неизвестном типе берёт профиль по умолчанию, поэтому это работает; дубликат по `scope_id` отсекается.

**Гонки и конкурентность.**
- Блокировка на игрока со счётчиком пользователей корректна: запись удаляется вместе с последним пользователем. Параллельные повторы дают одну публикацию (тест, M19 исполнителя).
- `Limiter` целиком под одним мьютексом.
- Sweeper читает `c.keys`/`c.limiter`, записанные до запуска горутины. В replay sweeper не стартует, поэтому `c.limiter == nil` безопасен. `Stop` останавливает цикл до закрытия `gateway.db`.
- Проекция читается несколькими отдельными вызовами (`Character`, `EncounterOf`, `NPC`, `Region`), каждый под своим захватом. Для предусловий по проекции это допустимо: окончательно решает State по `expected_version`.
- Ожидание блокировки игрока не ограничено контекстом — N-5.

**Покрытие DoD.**
- Все строки DoD T-305 закрыты тестами; оговорки — Mi-1…Mi-5.
- Строка DoD T-456 закрыта: `ForgetIncomplete` у `forgetLink` и `adminForgetLink`, `Unavailable` без `Retry-After`, тексты; есть тест `TestOpenAPIForgetIncompleteIsAResponseOfItsOwn`.
- `replayClock` под тегом `process` — по C-01 v1.9, тесты есть.
- Табличный тест `Validate` не содержит строк `group.join`/`group.leave` кроме `501` — это I2.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Ma-1 | Major | `internal/gateway/actions/service.go:137-150`; `internal/gateway/handlers/actions.go:42`; карточка, «Риски», п. 1 | Повтор `action_key` публикует второе действие, хотя первое уже в шине. (а) `player.*` опубликован, а предложение (или следующее событие пакета) — нет: ответ `503 bus_unavailable`, ключ не сохранён, повтор строит новый `player.*` с новым id (зонд P1). (б) Публикация, `Turns.Accepted` и `keep` идут на контексте запроса. `net/http` отменяет его при разрыве соединения клиентом, middleware `timeout` — через 5 с. Отмена после `player.*` даёт `503` и дубликат (P2). Отмена после последней публикации даёт `202` без сохранённого ключа (P3). Клиент `internal/gateway/client` повторяет `503` и сетевые ошибки сам. Нарушены C-08 («`503` — действие не принято») и `api-contracts.md` §1.1 («второго хода нет»). Второй `player.entered_region` — второй повод к спавну встречи (C-05) и второй нарратив. | 1) Пакет событий строится один раз; публикация и запись результата (`Turns.Accepted`, `keep`) — на `context.WithoutCancel(ctx)` со своим сроком (например, `api.RequestTimeout`). Начатый пакет не обрывается уходом клиента, `202` не остаётся без ключа. 2) Пакет, решённый не до конца (ошибка любой публикации), хранится в памяти сервиса по `(player_id, action_key)` под той же блокировкой игрока, с TTL (например, `store.KeyTTL`). Повтор с тем же ключом публикует **те же** события — те же id и `timestamp`, — затем пишет ключ и отвечает `202` с тем же `correlation_id`. Повтор события с тем же id потребители отсекают (C-01, ADR-027; `processed_events` шлюза) — эта гарантия и так нужна при потерянном подтверждении брокера. Контракт не меняется: `503` по-прежнему значит «ключ не записан, повторите тем же ключом». 3) Тесты по зондам P1–P3: один id `player.*` на ключ после сбоя предложения и после отмены контекста; ключ сохранён после `202` на отменённом контексте. Остаточный риск (пакет в памяти теряется при рестарте) — в карточку. Если tech-lead#3 решит не исправлять в задаче — только явным исключением в C-08 через system-architect, а не строкой «Риски». | открыто |
| Mi-1 | Minor | `internal/gateway/actions/publish.go:70, 114`; `internal/gateway/handlers/actions.go:37` | `meta.actor_kind` событий из `X-Actor-Kind` не проверен ни одним тестом: мутант R2 (всегда `human`) зелёный. Для CI-харнесса и аналитики C-10 признак `ci`/`sim` — единственное, что отделяет его ходы от живых. | Тест контекста: клиент из `MV_GATEWAY_ACTOR_KIND_CLIENTS` с `X-Actor-Kind: ci` → `player.*` в `player_events` с `meta.actor_kind=ci`. Unit в `actions`: `Command.ActorKind=sim` → `sim` у `player.*` и `gm.created`. | открыто |
| Mi-2 | Minor | `internal/gateway/actions/validate.go:161`; `validate_test.go:59` | `409 character_dead` для `status=abandoned` не закреплён тестом, хотя его требует C-08 v1.2 и карточка («включая `abandoned`»): мутант R7 (`== dead`) зелёный. После `/forget` это единственная защита от действий `ci`/`sim` по `player_id`. | В фикстуру `newWorld` — персонаж со `status=abandoned`, в таблицу — строка `character_dead abandoned`. | открыто |
| Mi-3 | Minor | `internal/gateway/actions/service.go:245-249`; `service_test.go:224-227` | «Повтор — тот же ответ» для `4xx` проверяется только по коду и `message`: мутант R9 (без `details`) зелёный. А `details` различают, например, `text_invalid {filter: blocked}` и `text_invalid`, `invalid_request {field}`. | Сравнить тело повтора с телом первого ответа целиком (JSON `api.ErrorResponse`) на случае с `details`: `text_invalid` от блокирующего фильтра или `invalid_request {field: target}`. | открыто |
| Mi-4 | Minor | `internal/gateway/actions/ratelimit.go:93`; `ratelimit_test.go`, `actions_test.go:131-155` | Округление `Retry-After` вверх не закреплено: во всех тестах ожидание — целые секунды, мутант R8 (вниз) зелёный. При округлении вниз клиент придёт на полсекунды раньше и снова получит `429`. | Тест: после серии из 5 сдвинуть часы на 500 мс → ожидание 1,5 с → `Retry-After: 2`; ожидание 31-го действия — тоже с дробной частью. | открыто |
| Mi-5 | Minor | `internal/gateway/actions/service.go:180-183`; `internal/gateway/context.go:376-377` | Две ветки без тестов. (1) Фильтр заменил текст на недопустимый (пустой, > 500 рун, управляющие символы) → должно быть `400 text_invalid` без публикации. Мутант R3 зелёный; без этой ветки такой текст упрётся в схему шины и получит `503`. (2) Вызов `Limiter.Sweep` из sweeper'а контекста: мутант R6 зелёный, а без вызова таблица лимитера растёт на каждый `player_id`. | (1) Фильтр-заглушка с `Status: replaced, Text: ""` и с 501 руной → `text_invalid`, `published = 0`. (2) В `TestTheSweeperRunsInLiveModeOnly` или отдельно: действие → `Manual.Advance(LimiterSweepInterval)` → `Players() == 0`. Для этого нужен доступ к лимитеру контекста (экспорт для тестов, как `ReadModel`). | открыто |
| N-1 | Nit | `internal/gateway/actions/service.go:162-185` | `FilterText(KindName, …)` проверяет имя правилами `say` (1–500 рун) и отвечает `text_invalid`. Для имени по §1.6 нужны `name_invalid`/`name_required` и 2–32 символа. Сейчас метод не вызывается, но T-306 может взять его как есть. | Строка в карточку T-306 или в doc `FilterText`: для `KindName` вызывающий перепроверяет результат правилами имени и сам сопоставляет код. | открыто → T-306 |
| N-2 | Nit | `internal/gateway/actions/validate.go:257`; `internal/gateway/context.go:259`; `shared/env/vars.go` (`GatewayEncounterGrace`) | Нулевой `CreatedAt` (сущность встречи без `created_at`, например из старого снапшота) сразу даёт `encounter_unavailable`: `now.Sub(zero) > grace`. Отрицательный `MV_GATEWAY_ENCOUNTER_GRACE` тоже принимается. | Условие `!enc.CreatedAt.IsZero()`; `grace < 0` при старте — ошибка. | открыто |
| N-3 | Nit | `internal/gateway/context.go:249-256` | `MV_GATEWAY_RATE_ACTIONS_PER_MIN=many` даёт две ошибки: разбора и «must be at least 1» (значение осталось 0). | Проверять `< 1` только для разобранных значений. | открыто |
| N-4 | Nit | `internal/gateway/actions/service.go:98-108, 194` | `404 player_not_found` сохраняется под любым `player_id` из пути на 24 ч. Лимит считается по тому же произвольному `player_id`, поэтому объём `idempotency_keys` им не ограничен. Контур доверенный (loopback, allowlist клиентов), и по букве C-08 так и должно быть. | В бэклог: не сохранять `player_not_found` или до поиска ключа ограничивать длину и алфавит `player_id` в пути (`400 invalid_request`). | открыто → бэклог |
| N-5 | Nit | `internal/gateway/actions/service.go:94-100, 264-273` | Параллельный повтор ждёт блокировку игрока, не глядя на контекст. Если первый запрос держит её до дедлайна, `Lookup` второго идёт на истёкшем контексте → `500 internal`. Клиент такой ответ не повторяет, хотя действие принято. | Вместе с Ma-1: ожидание блокировки с выходом по `ctx.Done()` в повторяемый `503 bus_unavailable` либо `Lookup` на `WithoutCancel`. | открыто |

### Для architect#3
1. **КД §5.1 п. 6, C-08 v1.1, `api-contracts.md` §1.1.** Заменить «token bucket 30/мин, burst 5» на «не больше `PER_MIN` за любую минуту и не больше `BURST` подряд» (ведро плюс минутное окно). Это текст к коду: код строже, а иначе не выполняется DoD «31-е за минуту — `429`». Бот в T-310 отступил так же (скользящее окно).
2. **КД §5.5.** Сейчас описана одна публикация, а пакет действия — до трёх событий (`gm.created`, `player.*`, предложение). Туда же стоит записать правило повтора после частичной публикации (Ma-1): «повтор ключа публикует тот же пакет с теми же id».
3. **КД §6.** Поля `readmodel.Encounter.TaskAgentID` и `CreatedAt` (отступление 4).

### Предложения в бэклог (вне границ T-305)
1. **T-306:** `actor_kind` сессии вместо заголовка; для `FilterText(KindName)` — перепроверка правилами имени (N-1); `Turns.Accepted`/`Rejected` на контексте без отмены (как в Ma-1).
2. **Лимитер после идемпотентности (отступление 5).** Если повтор принятого ключа не должен тратить жетон, лимит нужно проверять после `Lookup`, но до валидации. Это меняет порядок middleware КД §5.1; решает architect#3.
3. **T-307:** доставка `entity.update.rejected` по предложениям движения и отдыха (предложение исполнителя). При отставании проекции игрок получает `202` без результата.
4. **N-3 ревью T-304** (индекс открытой встречи игрока) T-305 не закрыла: `EncounterOf` по-прежнему сортирует все встречи на каждое `attack`/`flee`/`defend`.
5. **N-4:** ограничить `player_id` пути или не хранить `player_not_found`.

### Риски и допущения
- `-race` не запускался (cgo выключен). Конкурентность блокировок игрока, лимитера и sweeper'а проверена чтением.
- Отмена контекста запроса при разрыве соединения — документированное поведение `net/http` (`Request.Context`). На kafka-адаптере не проверялась: интеграционные прогоны вне поручения. Зонды P2/P3 моделируют отмену обёрткой шины над `membus`.
- Последствие дубликата `player.entered_region` для спавна встречи выведено из C-05 (форма `FakeEncounter`) и КД роя. Идемпотентность настоящего агента встречи по игроку не проверялась.
- Слияние проверено на кончике эпика `c92eb22`. Если до finish в эпик войдут другие правки `vars.go`/`.env.example`, проверку надо повторить.
- Мутанты и зонды выполнялись только в копии дерева. В рабочей папке задачи ревьюер добавил только этот раздел и строку в карточке.

## T-305 · ревью #2 · 2026-09-13 · code-reviewer#2 (TEAM-3)

### Границы ревью
- Ветка `task/T-305-actions` (`.worktrees/T-305`), база `abfc04b`. Изменения не закоммичены. Проверены только исправления итерации 2 и регрессия от них:
  - `internal/gateway/actions/{service,pending,validate}.go`;
  - `internal/gateway/context.go`, `export_test.go`;
  - тесты `actions/{pending,service,helpers,validate,ratelimit}_test.go`, `gateway/actions_test.go`.
- Сверено с:
  - своим ревью #1 T-305;
  - разделом «Итерация 2» карточки и записью dev-log итерации 2;
  - `contracts.md` v0.14 из `.worktrees/EPIC-004`: C-08 v1.5 («`503 bus_unavailable` — не принято, ключ не записан, повторите с тем же ключом»);
  - `shared/runtime/http.go` (`ShutdownTimeout`, `ShuttingDown`), `api/middleware.go` (`timeout`), `shared/eventbus` (`Kafka.Publish` событие не меняет).
- Решение оркестратора по Ma-1: исправить в задаче, контракт не менять. Исполнитель ему следует: C-08 и OpenAPI в итерации 2 не менялись.

### Вердикт
**Принять.** Critical 0 · Major 0 · Minor 1 · Nit 2.

Ma-1 закрыт:
- P1–P3 закреплены тестами;
- пакет строится один раз, упавшее событие уходит повторно с тем же id и байтами;
- `Turns.Begin` и `Turns.Accepted` вызываются по одному разу на пакет, второго `player.*` нет;
- уход клиента и таймаут middleware пакет не рвут.

Остался узкий случай того же класса, что P3: срок решения истекает между последней успешной публикацией и записью ключа. Это Mi-6. Для дубля нужно совпадение двух событий: срок истёк именно в этом зазоре (миллисекунды на пятой секунде медленной публикации), и клиент потерял уже полученный `202`. Поэтому Minor, а не Major. Исправление — две строки, рекомендую сделать до finish.

Mi-1…Mi-5 закрыты тестами: мутанты R2, R3, R6–R9 ревью #1 красные. N-1, N-2, N-3, N-5 закрыты, N-4 ушёл в бэклог.

### Проверено ревьюером (go1.26.8 windows/amd64, cgo выключен — без `-race`)
- `go build ./... && go vet ./...` — 0.
- `go test -short -count=1 ./internal/gateway/...` — все 10 пакетов `ok`.
- `golangci-lint run ./internal/gateway/...` — 0 issues.
- Мутанты и зонды — в копии дерева `scratchpad/t305r2-mut` (tar без `.git`, `services/`, `.claude/`, `.qwen/`, `.env`), без `-overlay`, контрольный первым. Каждый мутант — точная замена единственного вхождения. После прогона файл восстановлен и сверен побайтово. Прогон: `go test -short -count=1 -timeout 120s -skip TestProbe ./internal/gateway/...`. Копия и скрипт удалены по точным путям.

| # | Мутант / зонд | Результат |
|---|---|---|
| M0 | контрольный, без изменений | зелёный |
| R2 | `player.*` всегда `human` (`publish.go:70`) | **красный**: `TestTheActorKindHeaderReachesTheBus`, `TestTheActorKindOfTheRequestIsTheActorKindOfItsEvents` |
| R3 | результат фильтра не перепроверяется (`out, ok := res.Text, true`) | **красный**: `TestAReplacementThatIsNoTextIsRefused` (3 случая) |
| R6 | sweeper не вызывает `Limiter.Sweep` (`context.go`) | **красный**: `TestTheSweeperForgetsIdlePlayersAndExpiredBatches` |
| R7 | `character_dead` только для `dead` (`validate.go:161`) | **красный**: `TestValidateEveryRowAndCode/character_dead_abandoned` |
| R8 | `Retry-After` вниз (`ratelimit.go:93`) | **красный**: `TestRetryAfterRoundsTheWaitUp` |
| R9 | повтор `4xx` без `details` (`service.go:342`) | **красный**: `TestARepeatOfARefusalIsTheWholeFirstBody` (2 случая) |
| N1 | `take` отдаёт пакет после срока ключа (`pending.go:61`) | **красный**: `TestABatchPastItsExpiryIsNotPublished` |
| N2 | на пределе вытесняется самый новый пакет, а не ближайший к сроку (`pending.go:75`) | **красный**: `TestHalfPublishedActionsAreBoundedAndExpire` |
| N3 | отказ от ожидания блокировки не вызывает `leave()` (`service.go:382`) | **зелёный** → N-6 |
| N4 | срок решения не отменяет контекст (`service.go:231`) | **красный**: `TestABudgetThatRunsOutHoldsTheBatch` (таймаут пакета) |
| N5 | уборка пакетов без границы срока, `!After` → `Before` (`pending.go:92`) | **красный**: `TestHalfPublishedActionsAreBoundedAndExpire` |
| P4 | зонд: хук после успешной публикации предложения `rest` сдвигает часы на `api.RequestTimeout` (срок решения истекает), затем повтор ключа | первый ответ `202`, **ключ не записан**, пакета нет; повтор → `202` и второй `player.rested` (id `ev-1`, `ev-2`) → Mi-6 |
| P6 | зонд: 40 действий на уже отменённом контексте при свободной блокировке | 40 из 40 `202`: свободная блокировка берётся, как обещает комментарий |

### Разбор по пунктам поручения

**1. Ma-1.**
- **P1** — `TestTheRepeatOfAHalfPublishedActionFinishesItsBatch`, три случая: сбой `player.*`, сбой предложения, `legacy` после `gm.created`. Тест проверяет:
  - каждый тип опубликован один раз;
  - упавшее событие ушло с id и байтами первой попытки;
  - `correlation_id` — id единственного `player.*`;
  - `Turns.Accepted` вызван один раз;
  - ключ записан, пакета нет;
  - третий запрос ничего не публикует.

  Мутанты исполнителя A3–A5 и мои N1, N2, N5 красные.
- **P2**: `TestAHangUpDoesNotCutTheBatch` (отмена после `player.*` → `202`, ключ записан) и `TestABudgetThatRunsOutHoldsTheBatch` (срок → `503`, повтор публикует предложение с тем же id). A1 и N4 красные.
- **P3** — `TestAHangUpAfterTheLastPublicationKeepsTheKey`: отмена контекста **запроса** после последней публикации → ключ и ход записаны, повтор отдаёт тот же `acked_at`. A2 красный. Отмена по **сроку** в том же месте не закрыта — Mi-6 (зонд P4).
- **Те же байты.** `Kafka.Publish` делает `Route` и `json.Marshal` от копии события и его не меняет. `Timestamp` корня ставится при построении, поэтому повтор побайтово тот же.
- **Двойной ход.** `Begin` вызывается при построении пакета, `ref` хранится в пакете, `Accepted` — после полной публикации. Второго хода нет. Совпадение `seq` у двух действий scope при повторе после сбоя исполнитель честно отнёс к T-306.
- **Повтор без перепроверки.** Пакет повторяется без повторной валидации, даже если тело запроса другое. Это то же правило, что у сохранённого ключа (`TestARepeatOfTheKeyAnswersTheFirstAnswer`, случай «другое тело»). Принять.

**2. Новые дыры.**
- **Гонки.**
  - `take`/`hold` одного ключа идут под блокировкой его игрока: пакет одного ключа не публикуют два запроса сразу.
  - `take` удаляет пакет до публикации. Sweeper во время публикации его не видит.
  - `hold` при повторном сбое кладёт пакет обратно и никого не вытесняет: после `take` его место свободно.
  - `pendingBatches` целиком под своим мьютексом, sweeper берёт только его.
  - Блокировка игрока — канал на одно место плюс счётчик под `s.mu`. Запись удаляется вместе с последним пользователем, это корректно.
- **Предел 4096 и вытеснение.**
  - Вытесняется пакет, ближайший к сроку, то есть самый старый: срок у всех — время построения плюс 24 ч.
  - Эталонный клиент повторяет `503` три раза за секунды (`DefaultBackoff`). К моменту вытеснения у самого старого пакета повторов почти наверняка уже не будет.
  - Пакет, который клиент ещё повторит, теряется только при 4096 разных сбойных действиях за окно повторов. На масштабе MVP-1 с лимитом 30/мин на игрока это нереально.
  - Решение 4 (вытеснять, а не отказывать) — принять. N2 красный.
  - Оценка памяти в комментарии занижена — N-7.
- **Уборка по TTL.** `SweepPending` вызывается в тике `store.SweepInterval` вместе с `Keys.Sweep`. Граница срока одна у `take`, `sweep` и `Lookup` ключа: истёк при `expires <= now`. R6, A6, N5 красные.
- **`WithoutCancel` не держит запрос дольше срока.**
  - Всё решение идёт на контексте с отменой по `Timers.After(Budget)`. `Lookup`, фильтр, `Publish` и `Save` уважают контекст; `Turns.Begin` у `MemoryTurns` работает в памяти.
  - Верхняя граница запроса — ожидание блокировки (контекст запроса, до 5 с) плюс `Budget` (5 с). Клиенту это не мешает: `DefaultHTTPTimeout` 35 с.
  - Горутина срока завершается в `release`.
  - Остаётся остановка процесса — см. «Риски».
- **Replay.** Срок считается по `deps.Timers`, то есть по таймерам replay. Ожидание блокировки ограничено контекстом middleware (настенные 5 с). Допущение исполнителя «харнесс не сдвигает часы, пока идёт публикация» разумно: харнесс шлёт действия последовательно и ждёт `202`. Принять.
- **N-5.**
  - Ожидание блокировки прерывается контекстом запроса → `503 bus_unavailable`; ничего не решено и не хранится.
  - Тест `TestARequestThatGivesUpWaitingForItsPlayerDecidesNothing`; мутант N5 исполнителя красный.
  - Свободная блокировка на завершённом контексте берётся (зонд P6).
  - Код `bus_unavailable` для занятого игрока верен: по C-08 он значит «не принято, повторите тем же ключом».
  - Утечка записи при отказе от ожидания тестом не закреплена — N-6.

**3. Mi-1…Mi-5 и Nit.**
- Mi-1…Mi-5 закрыты: R2, R3, R6–R9 красные (таблица выше).
- N-1 — doc `FilterText`.
- N-2 — условие `!CreatedAt.IsZero()` с тестом; `grace < 0` — ошибка старта (случай `negative grace`).
- N-3 — на `burst=many` одна ошибка; тест проверяет, что «at least 1» нет.
- N-5 — выше. N-4 — в бэклоге по решению оркестратора.

**Оценка решений итерации 2.** Приняты решения 1–5 карточки:
- пакет хранится и при сбое первого события;
- повтор без перепроверки;
- срок покрывает всё решение;
- на пределе — вытеснение;
- `Timers`/`KeyTTL` обязательны.

Остаточный риск перезапуска описан и закреплён тестом `TestARestartBetweenPublicationsBuildsTheActionAgain`. По решению оркестратора он передаётся architect#3 в КД §5.5.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Mi-6 | Minor | `internal/gateway/actions/service.go:137-138, 204-211` | После полной публикации `Turns.Accepted` и `keep` идут на том же контексте, который отменяет срок решения. Если `Budget` истекает между последней успешной публикацией и `Keys.Save`, `ExecContext` получает отменённый контекст. Ключ не пишется, ошибка уходит только в журнал, клиенту уходит `202`. Пакет к этому моменту уже снят (`take`) или не держался. Повтор ключа строит действие заново: второй `player.*` с новым id. Зонд P4 это воспроизводит: `202`, ключа нет, два `player.rested`. Это тот же класс, что P3 ревью #1, и это противоречит фразе карточки «`202` без ключа не остаётся». Для дубля нужны три условия: медленный брокер (публикация около 5 с), срок, попавший в зазор в миллисекунды, и потеря полученного `202`. Отсюда Minor. | Записывать результат после полной публикации на `context.WithoutCancel(ctx)`: SQLite ограничен `busy_timeout`, `MemoryTurns` работает в памяти. Альтернатива — при ошибке `keep` оставлять пакет с `sent == len(events)` в `pending`, чтобы повтор ответил `202` без публикации. Тест по зонду P4: хук после успешной публикации последнего события сдвигает `Manual` на `Budget` и ждёт отмены; после повтора ожидаются записанный ключ и один `player.*`. То же правило — в карточку T-306 для `Turns.Accepted` на БД. | открыто |
| N-6 | Nit | `internal/gateway/actions/service.go:381-383`; `pending_test.go:300` | Ветка «запрос бросил ожидание блокировки» освобождает запись игрока (`leave()`), но тест этого не проверяет: мутант N3 без `leave()` зелёный. Без вызова счётчик не доходит до 0, и запись в `players` не удаляется никогда. Это утечка на игрока, взаимной блокировки нет. | В `TestARequestThatGivesUpWaitingForItsPlayerDecidesNothing` после завершения первого запроса проверить, что записей блокировок не осталось. Для этого нужен счётчик для тестов, как `Pending()`. | открыто |
| N-7 | Nit | `internal/gateway/actions/pending.go:11-15`; карточка, «Итерация 2», Ma-1 («единицы МБ») | Оценка памяти неверна: 4096 пакетов × до 3 событий × «несколько КБ» — десятки МБ, а не единицы. Событие в памяти Go (payload `map[string]any`, `say` до 500 рун) занимает порядка 1–3 КБ. Сам предел разумный, неверна только оценка. | Исправить комментарий и карточку («до нескольких десятков МБ») или снизить `DefaultPendingLimit`. | открыто |

**Замечания ревью #1:**
- Ma-1 закрыт, остаток — Mi-6.
- Mi-1…Mi-5 закрыты.
- N-1, N-2, N-3, N-5 закрыты.
- N-4 — в бэклоге (architect#3).

### Для architect#3 (КД §5.5, дополнение к ревью #1, п. 2)
1. К правилу «повтор ключа публикует тот же пакет с теми же id» добавить: запись хода и ключа после полной публикации не зависит ни от отмены запроса, ни от срока решения (Mi-6).
2. В остаточный риск перезапуска добавить остановку процесса (см. «Риски»): решение, не уложившееся в `ShutdownTimeout`, застаёт `gateway.db` и шину закрытыми.

### Предложения в бэклог (вне границ T-305)
1. **T-306:**
   - `Turns.Accepted` на БД — на контексте без отмены и без срока решения (Mi-6);
   - выдача `seq` с резервированием при `Begin` (предложение исполнителя) — поддерживаю.
2. **Outbox пакета в `gateway.db`** (предложение исполнителя) — поддерживаю. Он же закрывает вытеснение на пределе и остановку процесса.
3. **Остановка шлюза.** `Context.Stop` закрывает `gateway.db`, не дожидаясь незавершённых `Submit`. Можно считать активные решения и ждать их в пределах контекста `Stop`. Вопрос соседний с outbox.

### Риски и допущения
- `-race` не запускался (cgo выключен). Конкурентность `pendingBatches`, блокировки игрока и sweeper'а проверена чтением и мутантами N1–N5.
- **Остановка процесса.**
  - `runtime.HTTP.Stop` ждёт обработчики не дольше `ShutdownTimeout` (5 с) и контексты запросов не отменяет.
  - Решение длится до «ожидание блокировки + `Budget` (5 с)». Если оно началось к концу `Shutdown`, в `Context.Stop` оно может застать `gateway.db` и шину закрытыми.
  - Исход тот же, что у принятого риска перезапуска: пакет в памяти теряется. Нового класса дыр нет, но окно шире, чем «рестарт между сбоем и повтором».
- Зонд P4 моделирует срок сдвигом `clock.Manual` в хуке после публикации и паузой 100 мс. На kafka-адаптере не проверялся: интеграционные прогоны вне поручения. На реальном брокере зазор — время `Turns.Accepted` плюс `Keys.Save`.
- Вывод «к моменту вытеснения клиент уже не повторит» опирается на политику эталонного `internal/gateway/client` (3 повтора). Сторонний клиент с долгими повторами при более чем 4096 сбойных действиях за окно может получить дубль — это тот же остаточный риск.
- Мутанты и зонды выполнялись только в копии дерева. В рабочей папке задачи ревьюер добавил только этот раздел и строку в карточке.
