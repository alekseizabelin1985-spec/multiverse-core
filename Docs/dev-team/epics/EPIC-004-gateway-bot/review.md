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

## T-310 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-3)

### Границы ревью

Ветка `task/T-310-bot-core`, папка `.worktrees/T-310`, база `83b9351`. Изменения не закоммичены (`git status`):
- новое: `cmd/telegram-bot/internal/{privacy,config,updates,sender,commands,access}` — 20 файлов, 3329 строк (код ~1600, тесты ~1700);
- правки: `go.mod` (+1), `go.sum` (+2), `shared/env/vars.go` (+9), `.env.example` (+4, CRLF сохранён), `tasks.md` (статус), `tasks/T-310.md`, `dev-log.md`.

Коммитов вне ветки нет. `.golangci.yml`, `internal/**` и чужие файлы не тронуты. Файлы EPIC-001 правились в мягком режиме, минимально.

Основания: раздел T-310 в `tasks.md` (DoD и сверка 2026-09-13); `design.md` §3.1 п. 7; КД `gateway-and-bot.md` §10.1–10.4, §11.1, §11.3; ADR-018 с дополнениями; `threat-model.md` (T-01…T-05, SEC-06/07/08/10/11); US-008, FR-131, NFR-044, NFR-049; `.golangci.yml`. Прочитан исходный код `github.com/go-telegram/bot@v1.25.0` из кэша модулей: `bot.go`, `get_updates.go`, `wait_updates.go`, `raw_request.go`, `options.go`, `errors.go`.

### Вердикт

**ВЕРНУТЬ** — Critical 0 · Major 2 · Minor 3 · Nit 6.

Работа сильная. Шлюз стоит до любого вызова gateway, редакция токена и id проверена поверх «голого» обработчика, `Sender` без разметки закреплён сигнатурой, отклонения обоснованы, 52 мутанта исполнителя красные. Вернуть задачу нужно из-за двух дефектов на границе с библиотекой и внешним миром:
- M-1: подтверждённые Telegram, но не обработанные обновления теряются при любой остановке;
- M-2: любой посторонний аккаунт или групповой чат может остановить обработку команд всех игроков через паузы `429` в отказах.

Оба исправления локальны и небольшие.

### Прогоны (go1.26.8 windows/amd64, рабочая папка T-310)

| Команда | Результат |
|---|---|
| `go build ./... && go vet ./...` | 0 |
| `go test -short -count=1 ./cmd/telegram-bot/...` | ok ×6 пакетов |
| `golangci-lint run ./...` | 0 issues |
| `go run ./cmd/mvctl env check` | 0, 69 переменных |
| `go run ./cmd/mvctl privacy scan testdata/` | чисто, 147 файлов |
| `go mod graph`, фильтр `go-telegram` | одна строка `multiverse-core.io github.com/go-telegram/bot@v1.25.0`: транзитивных зависимостей нет |
| `go mod verify` | all modules verified |
| копия дерева: тесты `TestTelegram*`, `TestTheLibrary*` пакета `updates` ×40 | ok, без зависаний |

`-race` недоступен (cgo выключен).

### Мутанты и пробы ревьюера

Копия дерева `t310rev-tree` в scratch (`go.mod`, `go.sum`, `shared`, `internal`, `cmd/telegram-bot`), без `-overlay`, один мутант на прогон пакета. Копия удалена по точному пути.

| # | Мутант / проба | Итог |
|---|---|---|
| M0 (контрольный, первым) | `limiter.go:55` `<` → `<=` | красный |
| P1 (проба M-1) | пачка из 3 обновлений, обработчик первого ждёт второй `getUpdates` | **воспроизведено**: второй `getUpdates` ушёл с `offset=4`, пока обрабатывалось обновление 1. После `cancel()` в обработчике 1 библиотека всё равно передала обновления 2 и 3 с отменённым `ctx` (`handled [1 2 3]`) |
| P1-fix | то же с `bot.WithUpdatesChannelCap(0)` | `handled [1], polls 1`: обновления 2 и 3 не подтверждены; пакет `updates` ×5 зелёный |
| R1 | окно хранит отметку ровно 60,0 с (`!hit.Before(cutoff)`) | зелёный → N-1 |
| R2 | allowlist принимает id `0` | зелёный → N-1 |
| R3 | соль ровно 16 байт отклоняется | зелёный → N-1 |
| R4 | `retry_after` ровно 60 с не ждётся | зелёный → N-1 |
| R5 | `From.IsBot` → игнор | зелёный; поведение нейтрально, не замечание |
| R6 | `401` при отправке → `ErrUnavailable` | красный: тест закрепляет `ErrRejected` (см. Mi-1) |
| R7 | `Redactor` без URL-экранированной формы | красный |
| R8 | ответ «пишите в личные» только участнику allowlist | зелёный → Mi-2 |

### Разбор по пунктам поручения

**1. Безопасность.**
- Токен. `privacy.Redact`, `botTokenInURL` (`privacy.go:38`) и `Redactor` (токен, `QueryEscape`, половина после `:`) закрывают URL библиотеки, `url.Error`, текст `409`, ошибку старта (`updates/telegram.go:122-128`), ошибки `Sender` (`sender/telegram.go:152-156`) и `ErrorLog`. `WithDebug` не используется: это закреплено поведенческим тестом и разбором AST, а `WithDebugHandler` заглушён. `defaultErrorsHandler` (`log.Printf`) библиотеки заменён в обоих `bot.New`.
- Соль. `Secret()` в манифесте, в `.env.example:258` без значения. Явная соль короче 16 символов — ошибка (`config.go:101-106`), значение в текст не попадает. Производная `SHA-256(token)` в `Secrets()` не входит: токен там уже есть. `LogValue`/`String`/`GoString` без секретов.
- Внешние id. `updates.*` и `sender.Message` печатаются как `redacted` и в `slog`, и в `%v`. Ошибка allowlist называет только номер записи. `fail` вырезает `chat_id` из текста. Остаток: тело ответа в ошибке декодирования библиотеки (Mi-3).
- SEC-06: «ноль вызовов» проверен `httptest`-моком со счётчиком (`gate_test.go:125-150`). Вызов идёт сырым HTTP, а не `internal/gateway/client`, из-за depguard (п. 4). Доказательная сила та же: мок считает любой запрос. После правила depguard T-311 переводит тест на `client.New`.

**2. Отклонения.**
- **Скользящее окно вместо token bucket** — принимаю. Расчёт исполнителя верен: ведро на 20 с пополнением 20/мин пропускает 21-ю команду через ~3 с, а NFR-049 и DoD требуют отказа до 61-й секунды. Память ограничена числом участников allowlist и 20 отметками на каждого. Один ответ на серию отказов тоже принимаю. Текст КД §10.2 п. 4 и ADR-018 доп. п. 2 («token bucket») нужно поправить (architect#3, бэклог).
- **Типизированные ошибки `Sender`, потолок `retry_after` 60 с × 5** — принимаю: без потолка флуд-бан на час заблокировал бы `Send`. Худший случай — 5 минут в синхронном пути. Для отказов шлюза это дефект M-2. Для ответов `flow` в T-311 нужна короткая политика, для `deliver` в T-313 — политика по умолчанию в своей горутине.
- **`Handler` с `ctx`** — принимаю, нужна правка ADR-018 п. 2.
- **Служебные команды только со «/»** — принимаю: КД §10.3 перечисляет `/start`, `/help`, `/forget` только со слэшем, и «forget» во фразе не должен запускать удаление.
- **Ответ в групповом чате — вопрос к tech-lead#3 и BA.** Источники расходятся:
  - за ответ: DoD T-310 (SEC-07 «одно сообщение»), ADR-018 п. 3 и доп. п. 1, КД §10.2 п. 2 и §11.1, ADR-006 доп. п. 2;
  - против ответа: US-008 (негативный критерий: «бот не отвечает и не создаёт действий»), FR-131 («игнорируются (без ответа и без записи в лог с id)»), NFR-044 («100 % сообщений из групповых чатов проигнорированы»);
  - `threat-model.md` SEC-07 допускает оба варианта («игнор/отказ»).

  Требования стоят выше дизайна. Молчание в группе к тому же сужает M-2: бот не выдаёт себя в чужих группах. Рекомендую молчать, но решает BA. Сделано по DoD, для исполнителя это не замечание.
- **Объём** (~1600 + ~1700 строк при ориентире M ~600) обоснован: шесть пакетов в одной задаче. Тесты содержательные, не раздутые. Это сигнал планированию.

**3. Зависимость `github.com/go-telegram/bot v1.25.0`.**
- Версия совпадает с ADR-018 п. 1.
- В `go.mod` модуля библиотеки нет `require`, `go mod graph` даёт одно ребро: транзитивных зависимостей нет.
- Лицензия — **MIT** (`LICENSE`: «MIT License, Copyright (c) 2022 negasus»).
- В `go.sum` две строки (`h1:` и `/go.mod`), `go mod verify` зелёный.
- Более новые версии офлайн не проверял; исполнитель называет v1.27.0, обновление — отдельной задачей по ADR-018.
- Настенное время внутри библиотеки (вне `forbidigo`):
  - `get_updates.go`: пауза `time.After` между ошибками `getUpdates` — от 100 мс с удвоением до 5 с, а при `429` на `getUpdates` — `retry_after` без потолка;
  - `bot.New`: `getMe` с таймаутом 5 с от `context.Background()`, а не от `ctx` вызывающего.

  Тесты от этого не зависят, кроме одной паузы 100 мс после `502` (`TestTelegramRedactsTheErrorsOfTheLibrary`). Повтору (replay) это не мешает: бот в replay не участвует. Для e2e T-315 источник — `updates.Fake`. Риск принят.

**4. depguard.**
- T-310 текущих правил не нарушает, `golangci-lint` — 0.
- Вывод исполнителя подтверждаю чтением `.golangci.yml`: glob `**/internal/**` правила `internal-unlisted` совпадает с `cmd/telegram-bot/internal/**`, а список исключений (`!**/internal/<ctx>/**`) бота не содержит. Любой `import multiverse-core.io/internal/gateway/{client,api}` из бота, тесты включительно, будет отклонён. Это блокер T-311.
- Предложенное правило `cmd-telegram-bot` поддерживаю с тремя уточнениями для system-architect/tech-lead#1:
  1. `files: **/cmd/telegram-bot/**` покрывает и будущий `main.go` T-312. Сейчас у `cmd/telegram-bot/*.go` вне `internal/` нет запрета на `internal/*`, только `cmd-others` (replay) и `no-testkit-in-production`, так что правило закрывает и эту дыру. В `internal-unlisted` добавить `!**/cmd/telegram-bot/**`.
  2. Allow записями с `$` (`…/internal/gateway/client$`, `…/internal/gateway/api$`), как принято после T-445. Они не затеняют друг друга. Deny `multiverse-core.io/internal`, плюс deny `multiverse-core.io/shared/eventbus`: бот говорит с платформой только по HTTP (КД §10.1).
  3. Отдельное правило для `_test.go` нужно, только если тест бота импортирует `internal/gateway` напрямую. `shared/testkit/gateway` — не `internal/*`, и depguard смотрит только прямые импорты.

  Тест `go list -deps` (`config/imports_test.go`) остаётся второй линией: он ловит транзитивные зависимости, которых depguard не видит.

**5. `updates`.**
- `409` → `ErrConflict` → `ExitCode` 3 верно: `bot.ErrorConflict` оборачивается библиотекой через `%w`, отмена по `CompareAndSwap`, лог `handled=true`, тест с двумя `409` проверяет остановку на первом.
- Потеря обновлений — дефект M-1. Утверждение из решения 6 карточки («необработанные обновления получит оставшийся экземпляр») неверно для канала на 1024.
- Фейки: `updates/fake.go` и `sender/fake.go` лежат в production-пакетах, но `testkit` не импортируют (depguard чист). Без ссылки из `main` линкер выбрасывает их из бинарника. Имена и назначение совпадают с КД §15 и DoD. Допустимо, см. N-4.

**6. Мягкий режим EPIC-001.**
- `vars.go:268-277`: два объявления с комментарием о расхождении имён с §11.3.
- `.env.example:257-260`: две переменные с комментариями, соль пустая.
- `mvctl env check` зелёный.
- Переменные `MV_TELEGRAM_CLIENT_ID`, TTL диалога и ожидания доставок не объявлены: объявление идёт вместе с чтением, в T-311/T-312. Верно.
- Отметка tech-lead#1 о правке файлов EPIC-001 нужна.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| M-1 | Major | `cmd/telegram-bot/internal/updates/telegram.go:98-118` (опции без `WithUpdatesChannelCap`), `:132-139` (`adapt`); `updates/telegram_test.go:184-187`; `commands/actionkey.go:16-18`; карточка, решение 6 | **Обновления подтверждаются Telegram до обработки и теряются при остановке.** Как это устроено в v1.25.0: `getUpdates` для каждого обновления пишет `lastUpdateID` и кладёт его в буферизованный канал `b.updates` ёмкостью 1024 (`bot.go` `defaultUpdatesChanCap`), затем сразу шлёт следующий `getUpdates` с `offset = last+1`. Telegram считает этим вызовом подтверждёнными все обновления пачки. Единственный обработчик (`WithNotAsyncHandlers`, 1 воркер) в это время может разбирать первое обновление: онбординг ждёт `MV_GATEWAY_CHARACTER_WAIT`, `Send` ждёт `429`. Всё, что лежит в канале, при `SIGTERM` (деплой, T-312), при `409` и при падении пропадает навсегда: второй экземпляр его не получит. Проба P1 это воспроизводит: `offset=4` во время обработки обновления 1. Кроме того, после отмены `waitUpdates` случайно выбирает между `ctx.Done()` и очередью, и обработчик получает оставшиеся обновления с уже отменённым `ctx` (`handled [1 2 3]`). Это противоречит посылке `action_key` (ADR-018: «Telegram не переотдаёт подтверждённые обновления», повтор — «after a crash before Telegram saw the offset move»). Тест `409` построен так, что эффект не виден: конфликт отвечается только после обработки. | (а) Добавить `bot.WithUpdatesChannelCap(0)`. Поллер отдаёт обновление только готовому воркеру, и следующий `getUpdates` уходит, когда последнее обновление пачки уже взято в обработку. Потеря при остановке сжимается до одного обновления в работе, остаток пачки Telegram отдаст повторно (проба P1-fix, пакет зелёный). (б) В `adapt` не вызывать `h` при `ctx.Err() != nil`. (в) Тест: пачка из 3 обновлений, обработчик первого ждёт до 1 с второго `getUpdates` → второго запроса нет; `cancel()` в обработчике 1 → обработано только 1. (г) Исправить комментарий `telegram_test.go:184-187` и решение 6 в карточке. Остаточное окно в одно обновление — в риски T-312. | открыто |
| M-2 | Major | `access/gate.go:131-143` (`Notify: true` на каждое `NotPrivate`/`NotAllowed`), `:164`, `:182`; `sender/telegram.go:35`, `:120-131` | **Посторонний может остановить обработку команд всех игроков.** Отказ шлюза отправляется синхронно в том же единственном обработчике через `Sender` с политикой по умолчанию: до 5 ожиданий `retry_after` по ≤ 60 с. Аккаунт вне allowlist, флудящий в личку, или любой групповой чат, куда добавили бота, получает ответ на каждое сообщение. Лимит Telegram на чат (~1 сообщение/с) быстро даёт `429`, `Send` засыпает, и команды игроков из allowlist стоят в очереди за паузами. Это точка входа EP-01 без аутентификации (T-01/T-05), нарушение NFR-049 («лимит одного игрока не задерживает других», NFR-001) и NFR-044. Вдобавок каждый отказ пишет строку `Info` (`gate.go:164`): при ротации `json-file 10m×3` флуд вытесняет полезные логи. Лимит SEC-11 сейчас защищает gateway, но не сам бот. | (а) Отказы — best effort: без ожидания `429` и без повторов. Например, отдельная политика `Policy{Attempts: 1, RateLimitWaits: 0, MaxRetryAfter: 0}` для отправителя шлюза или короткий дедлайн `ctx` на отказ. (б) Не больше одного ответа на отказ на чат за окно: ограниченная карта `chat → последний ответ` (например, ≤ 1024 записей с вытеснением старых, id не логируются) или общий бюджет отказов в минуту. Лог — одна строка на серию или счётчики без строки. (в) Тесты: 30 сообщений постороннего за 1 с → ≤ 1 `Send`; `Sender` отвечает `429 retry_after=30` → `Wrap` возвращается без паузы (таймер не взведён), следующая команда участника доходит до мока gateway. Если BA решит молчать в группах (вопрос ниже), ветка `NotPrivate` уходит из этого пути сама. | открыто |
| Mi-1 | Minor | `sender/telegram.go:144-145`; `sender/telegram_test.go:316`; `updates/telegram.go:72-82` | **Отозванный на ходу токен.** `401` при `sendMessage` классифицируется как `ErrRejected` («повтор не поможет»), и `deliver` T-313, скорее всего, подтвердит доставку: все сообщения игрокам молча потеряются. `401` на `getUpdates` библиотека повторяет бесконечно (пауза до 5 с), бот остаётся «живым», в логе поток ошибок. | Отдельная `ErrUnauthorized` (или `ErrUnavailable`: не подтверждать). В `onError` источника останавливать опрос на `bot.ErrorUnauthorized`, как на `409`, и возвращать `ErrUnauthorized` (выход 1). Тесты на оба случая. Можно передать строкой DoD в T-312/T-313. | открыто |
| Mi-2 | Minor | `access/gate.go:131-134`; `access/gate_test.go:168-195` | Порядок шагов закреплён только для участника allowlist в группе. Посторонний в группе получает «Пишите боту в личные сообщения.», и мутант R8 (отвечать в группе только участнику allowlist) зелёный. Бот, добавленный в любую группу, отвечает любому её участнику. От ответа BA зависит, какое поведение верно. | После решения BA закрепить тестом поведение для постороннего в группе: молчание или один ответ без утечки id, но уже с учётом M-2. | открыто (ждёт BA) |
| Mi-3 | Minor | `privacy/privacy.go:245-251`; `sender/telegram.go:152-156` | Библиотека при ошибке декодирования ответа вставляет в текст ошибки **всё тело**: `raw_request.go`, `"error decode response body for method %s, %s, %w"`. Для `getUpdates` это id, `username`, `first_name` и тексты игроков, для `sendMessage` — `chat` и текст сообщения. `Redactor` удаляет только токен, так что при кривом JSON от прокси или Telegram ПДн попадут в лог (`ErrorLog`) и в текст ошибки `Send` (SEC-01/02, NFR-041). Вероятность мала, последствие — ПДн в логах. | В `privacy` усечь текст после маркера `error decode response body for method <m>,` до `[body redacted]` (и в `ErrorLog`, и в `Redactor.Redact` для ошибок). Тест с телом, содержащим id и текст. | открыто |
| N-1 | Nit | `access/limiter.go:50`; `config/config.go:102`, `:177`; `sender/telegram.go:126` | Границы не закреплены: мутанты R1–R4 зелёные (ровно 60,0 с, соль ровно 16, id `0`, `retry_after` ровно 60 с). Длина соли меряется в байтах, а текст ошибки и `.env.example` говорят «символов». | Четыре граничных случая в существующих табличных тестах; «байт» в тексте или `utf8.RuneCountInString`. | открыто |
| N-2 | Nit | `config/config.go:80` | Ошибка `MV_TELEGRAM_GATEWAY_URL` цитирует значение целиком; URL с `user:pass@` попадёт в лог старта. | Цитировать без `userinfo` (`u.Redacted()`) или не цитировать. | открыто |
| N-3 | Nit | `config/imports_test.go` | Тест границы всего бота лежит в пакете `config`. | Перенести в пакет `cmd/telegram-bot` (появится с `main.go` T-312) или оставить с комментарием. | открыто |
| N-4 | Nit | `updates/fake.go`, `sender/fake.go` | Фейки в production-пакетах расширяют их API (в бинарник не попадают). | Допустимо по КД §15. Если `testkit`-правило когда-нибудь распространят на `cmd/**`, вынести в `updatestest`/`sendertest`. | открыто → бэклог |
| N-5 | Nit | `updates/updates.go:32`; `updates/telegram.go:75` | `409` бывает и при активном webhook («can't use getUpdates method while webhook is active»), а сообщение говорит только о втором экземпляре. | «another instance is polling this token, or a webhook is set». | открыто |
| N-6 | Nit | `privacy/privacy.go:109-120` | Список ключей закрывает имена ADR-018, но не форму группы: `slog.Group("from", slog.Int64("id", …))` или `slog.Int64("player_chat", …)` пройдут. | Добавить в заглушаемые группы `from`, `chat`, `message`, `update`; в T-311 логировать только через заранее заданные ключи. | открыто |

### Вопросы к tech-lead#3 и BA (через оркестратора)

1. **Групповой чат: молчать или отвечать один раз?** Расхождение описано в п. 2 разбора. Рекомендация ревьюера — молчать (FR-131, NFR-044, US-008), с правкой DoD T-310, ADR-018 п. 3 и КД §10.2 п. 2.
2. **Решение depguard** (system-architect через tech-lead#3/#1) — до старта T-311, с уточнениями п. 4.

### Предложения в бэклог

1. **system-architect / tech-lead#1:** правило `cmd-telegram-bot` (п. 4, уточнения 1–3) и исключение `!**/cmd/telegram-bot/**` в `internal-unlisted`.
2. **architect#3:** КД §10.2 п. 4 и ADR-018 доп. п. 2 — «скользящее окно 60 с» вместо «token bucket»; ADR-018 п. 2 — `Handler(ctx, Update)`; §11.3 — имена `MV_TELEGRAM_*`; в §10.2 записать `WithUpdatesChannelCap(0)` и окно потери в одно обновление.
3. **T-311:** тест SEC-06 перевести на `client.New` против того же мока; короткая политика `Sender` для ответов `flow`; `flow` не должен звать gateway с отменённым `ctx`.
4. **T-312:** `os.Exit(updates.ExitCode(err))`; выход при `ErrUnauthorized` (Mi-1); в риски — одно обновление в работе при `SIGTERM`.
5. **T-313:** `ErrRejected`/`ErrUnauthorized` — какие ошибки подтверждают доставку (Mi-1).
6. **devops (эксплуатация):** в @BotFather выключить добавление бота в группы (`/setjoingroups` → Disable) — дешёвая внешняя мера к M-2 и Mi-2; упомянуть в runbook.
7. Поддерживаю бэклог исполнителя: п. 2 (лист-пакет редакции `shared/logging`, EPIC-001), п. 3 — поглощается M-2, п. 5, п. 6.

### Риски и допущения

- Наличие более новой версии `go-telegram/bot` и её changelog не проверял (сеть запрещена). Лицензия и отсутствие зависимостей проверены по кэшу модулей.
- M-2 оценён чтением: реальный Telegram не вызывался, лимиты на чат (~1 сообщение/с, `retry_after` при флуде) взяты из известного поведения Bot API.
- В пробе P1 `httptest.Server.Close` зависал на ответе `getUpdates`, который висит в ожидании: сервер не заметил закрытия соединения после полностью прочитанного multipart-тела. Это артефакт тестового сервера. Существующие тесты ×40 не зависают.
- Прогонов под `-race` не было, конкурентность `Gate`/`limiter` (мьютекс, атомарные счётчики) разобрана чтением.

## T-310 · ревью #2 · 2026-09-13 · code-reviewer#2 (TEAM-3)

### Границы ревью

Итерация 2 по ревью #1 (вернуть, 0/2/3/6). Ветка `task/T-310-bot-core`, папка `.worktrees/T-310`, база `83b9351`, изменения не закоммичены. Проверены только исправления и регрессия от них:
- код: `cmd/telegram-bot/internal/{access,updates,sender,privacy,config}`, включая новый `access/refusals.go`;
- документы исполнителя: раздел «Итерация 2» карточки, запись итерации 2 в `dev-log.md`, строки DoD T-311/T-312/T-315 в `tasks.md`.

Коммитов вне ветки нет. `.golangci.yml`, `internal/**`, `shared/**` итерацией 2 не тронуты.

Решение оркестратора принято как вход: в групповом чате бот молчит (US-008, FR-131, NFR-044 выше DoD T-310 SEC-07 и ADR-018 п. 3). Расхождение со строкой DoD дефектом не считается; правка строки DoD — tech-lead#3, ADR-018 и КД — architect#3.

Исходник `github.com/go-telegram/bot@v1.25.0` перечитан в кэше модулей (только чтение): `bot.go` (`Start`, `defaultWorkers = 1`), `get_updates.go`, `wait_updates.go`, `process_update.go`, `options.go`, `raw_request.go`.

### Вердикт

**ПРИНЯТЬ** — Critical 0 · Major 0 · Minor 1 · Nit 2.

Оба Major и все Minor ревью #1 закрыты. Исправления подтверждены чтением библиотеки и мутантами. Новое Minor-замечание Mi-4 — остаточный риск M-2 в синхронной отправке отказа. Итерацию 3 оно не требует: закрывается строкой DoD T-312 и небольшой правкой, которую можно сделать там же. Два Nit — неточный комментарий о сроке хранения id в памяти и три пробела в тестах.

### Прогоны (go1.26.8 windows/amd64, рабочая папка T-310)

| Команда | Результат |
|---|---|
| `go build ./... && go vet ./...` | 0 |
| `go test -short -count=5 ./cmd/telegram-bot/...` | ok ×6 пакетов |
| `golangci-lint run ./...` | 0 issues (depguard, forbidigo чисты) |
| `go run ./cmd/mvctl privacy scan testdata/` | чисто, 147 файлов |
| `go run ./cmd/mvctl env check` | 0, 69 переменных |

`-race` недоступен (cgo выключен).

### Мутанты ревьюера

Копия дерева `t310r2-tree` в scratch (`go.mod`, `go.sum`, `shared`, `internal`, `cmd/telegram-bot`), без `-overlay`. Один мутант на прогон пакета, контрольный — первым. После прогона копия сверена с рабочей папкой (`diff -r` пуст) и удалена по точному пути.

| # | Мутант | Итог |
|---|---|---|
| K0 (контрольный) | текст «Доступ по приглашению.» изменён | красный |
| M1a | без `bot.WithUpdatesChannelCap(0)` | красный, `TestTelegramConfirmsNoUpdateBeforeItIsInHand` за 0,00 с |
| M1b | `adapt` без проверки `ctx.Err()` | красный |
| M1c | «some updates lost» после остановки снова пишется как ошибка (`-count=5`) | красный 5/5 |
| M2g | `BestEffortPolicy` ждёт `429` (5 × 60 с) | красный |
| X15 | вычистка таблицы серий на `> Window` вместо `>=` | красный |
| X16 | серия длится до `<= Window` | красный |
| Mi1a | `401` при отправке → `ErrRejected` | красный |
| Mi1b | опрос не останавливается на `401` | красный (на утверждении, 5 с) |
| Mi3b | регэксп тела без `(?s)` | красный |
| R1 | окно лимита держит отметку ровно 60 с | красный |
| R2 | allowlist принимает id `0` | красный |
| R3a | соль ровно 16 символов отклоняется | красный |
| R3b | длина соли в байтах | красный |
| R4 | `retry_after` ровно 60 с не ждётся | красный |
| R8a (Mi-2) | постороннему в группе отвечать на каждое сообщение | красный |
| N6 | ключ/группа `from` не глушится | красный |
| N2 | URL gateway печатается с `userinfo` | красный |
| **R8b** | постороннему в группе отвечать один раз на серию, если серию открыл он | **зелёный** → N-8 |
| **X11** | `TooOften` пишет строку лога на каждый отказ, а не на серию | **зелёный** → N-8 |
| **X12** | `WithPolicy` меняет политику самого получателя, а не копии | **зелёный** → N-8 |

### Проверка закрытия

**M-1 — закрыто.** Разбор v1.25.0 при `WithUpdatesChannelCap(0)`, `WithNotAsyncHandlers`, `defaultWorkers = 1`:
- `getUpdates` для каждого обновления пачки пишет `lastUpdateID` и блокируется на небуферизованном `b.updates <- upd`, пока единственный воркер не выйдет из синхронного `ProcessUpdate` и не примет обновление;
- следующий `getUpdates` с `offset = last+1` (только он подтверждает пачку для Telegram) уходит после того, как воркер взял последнее обновление пачки;
- пока воркер занят обновлением k < N, подтверждения нет: при остановке Telegram отдаст повторно всю пачку, включая уже обработанные обновления, и повтор погасит `action_key`.

Окно потери действительно одно обновление: последнее в пачке, если оно в работе, а подтверждающий запрос уже дошёл до Telegram. Оно записано в рисках карточки и строкой DoD T-312.

После отмены `waitUpdates` и поллер выбирают ветку `select` случайно. Если передача всё же состоялась, `adapt` (`updates/telegram.go:165`) обновление не отдаёт. После этого поллер на верхней проверке `ctx.Done()` выходит без нового запроса, иначе пишет «some updates lost, ctx done». Эта строка теперь `Info` (`telegram.go:87-92`), условие — `runCtx.Err() != nil` и точный текст `b.error`.

Тест `TestTelegramConfirmsNoUpdateBeforeItIsInHand` проверяет два независимых свойства:
- ни один `getUpdates` с `offset > 2` не уходит, пока идёт обработка;
- `polled == 1` после отмены.

Второе ловит мутант без `cap 0` даже при неудачном порядке горутин в `onPoll`: он красный без зависимости от гонки. Ожидание 300 мс реального времени оправдано (доказательство отсутствия запроса), в рисках записано.

**M-2 — закрыто.**
- `access/refusals.go`: ответ и строка лога — один раз на чат за `Window`, граница `<` / `>=` закреплена (X15, X16). Таблица ≤ `MaxRefusedChats` = 1024, в полной таблице без просроченных записей новый чат молчит.
- `Gate.Check` (`gate.go:145-163`): `NotAllowed` — `Notify` и `Report` только на первом в серии; `TooOften` — через `limiter.warned`; `NotPrivate` — только `Report`.
- `bot_denied_total` считает каждое обновление.
- `sender.BestEffortPolicy = Policy{Attempts: 1}` при разборе `Send` (`sender/telegram.go:137-162`): `429` → `waits(1) > RateLimitWaits(0)` → `ErrUnavailable` без таймера; сеть и `5xx` → `tries(1) >= 1` → без паузы. `WithPolicy` копирует структуру и делит `*bot.Bot` и HTTP-клиент.
- `TestARateLimitedRefusalHoldsNobodyUp` на настоящем `sender.Telegram` против `429 retry_after=30`: 0 таймеров, 1 `sendMessage` на 30 сообщений, команда игрока доходит до мока gateway.

Остаточный риск синхронной отправки — Mi-4 ниже.

**Mi-1 — закрыто.**
- `sender/telegram.go:149-150`: `401` → `ErrUnauthorized`, проверка стоит до `403`/`400`.
- `updates/telegram.go:84-86`: `bot.ErrorUnauthorized` из `getUpdates` (библиотека оборачивает через `%w`) → `stop` и `ErrUnauthorized`, код выхода 1 (`ExitCode`), одна строка `handled=true`, токен вырезан.
- Ack по типам ошибок и `os.Exit` — строки DoD T-312. Уточнение исполнителя верно: `deliver` — это T-312, а не T-313, как ошибочно сказано в ревью #1.

**Mi-2 — закрыто** молчанием в группе: `TestSEC07AGroupChatGetsSilence` — group/supergroup/channel × участник и посторонний, ноль `Send`, ноль вызовов gateway, одна строка без id. Прямой аналог R8 (R8a) красный. Пробел порядка в тесте — N-8.

**Mi-3 — закрыто.**
- `privacy.go:45`, `:62-64`: `(?s)(error decode response body for method \w+,).*` → `[body redacted]`. Работает в `Redact`, в `Redactor.Redact` (через `Redact`), в `ErrorLog` и в строковых атрибутах `Handler`.
- Проверены все форматы ошибок `raw_request.go`: тело целиком цитирует только эта строка. `error response from telegram … %d %s` несёт лишь `description`.
- Тесты — на тексте с переводом строки и на настоящем `sendMessage` с битым JSON.

**N-1…N-6.**

| # | Итог |
|---|---|
| N-1 | закрыто: R1–R4 красные; соль в символах (`utf8.RuneCountInString`), R3b красный |
| N-2 | закрыто: `printableURL` без `userinfo`/query/fragment, неразбираемое не цитируется |
| N-3 | принято: комментарий в `imports_test.go` и строка DoD T-312 |
| N-4 | принято без изменений (КД §15) |
| N-5 | закрыто: `ErrConflict` и строка лога называют webhook |
| N-6 | закрыто: `from`/`chat`/`message`/`update` — ключи и группы глушатся, мутант красный |

**Регрессия и границы.**
- depguard и forbidigo чисты. Новых импортов, кроме `sync`/`time` в `refusals.go`, нет.
- Во всех новых строках лога только `reason`, `answered` и редактированная ошибка `Sender`: `fail` вырезает `chat_id`, `privacy.Handler` и `ErrorLog` — токен и тело. Внешних id в логах нет: проверено тестами и чтением `gate.go:179-191`, `telegram.go:75`, `:92`.
- `updates.Fake` соблюдает тот же контракт «после отмены обновление не отдаётся» (`fake.go:74`).

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Mi-4 | Minor | `sender/telegram.go:37-44` (`BestEffortPolicy`, `DefaultHTTPTimeout`), `:108-117` (`WithPolicy` делит клиент); `access/gate.go:33-36`, `:93-96`, `:189` | **Отказ без ожиданий всё ещё синхронен и ограничен только таймаутом HTTP 30 с.** Паузы `429` ушли. Остались: (1) медленный Telegram держит единственный обработчик до 30 с на каждый новый чат, потому что `WithPolicy` наследует клиент с `DefaultHTTPTimeout`; (2) бюджет ответов на отказ — до 1024 разных чатов в минуту. При обычном времени ответа 0,1–0,3 с около 300 посторонних аккаунтов по одному сообщению занимают обработчик на всю минуту, и команды игроков стоят в очереди (NFR-049/NFR-001). Нужны многие аккаунты, так что вероятность мала, но это та же точка входа EP-01 без аутентификации. Асинхронная отправка не нужна: она не снимает глобальный лимит Telegram на бота и добавляет очередь. Комментарий `BestEffortPolicy` («waits for nothing») обещает больше, чем даёт. | (а) **T-312, строка DoD:** шлюзу — отдельный `sender.NewTelegram` с `HTTPClient: &http.Client{Timeout: ≤ 5 s}` и `BestEffortPolicy`, а не `WithPolicy` от общего отправителя. Опция уже есть, `http.Client.Timeout` не попадает под `forbidigo`. Unit: мок Bot API не отвечает → `Wrap` возвращается не позже таймаута, следующая команда игрока доходит до gateway. (б) Общий бюджет ответов на отказ на весь бот (например, ≤ 30 в минуту, сверх него — молча, счётчик растёт) в `refusals` — одна проверка рядом с `opens`, можно в T-312 или отдельной задачей security-engineer. (в) В комментарии `BestEffortPolicy`/`Options.Sender` написать, что время одной попытки ограничено таймаутом клиента. | открыто → DoD T-312 |
| N-7 | Nit | `access/refusals.go:8-11`; карточка, решение 8 и риск 3 итерации 2 | Комментарий и карточка говорят, что id чата держится в памяти «не дольше окна» после серии. На деле запись вычищается только при вставке нового чата в полную таблицу (`refusals.go:35-40`). Id постороннего или группы лежит в памяти до рестарта, пока таблица не заполнится. В лог это не попадает, FR-131 не нарушен, но утверждение о приватности неверно. | Вычищать просроченные записи при каждой вставке нового чата (≤ 1024 итераций) или исправить комментарий и карточку: «до вытеснения, не дольше жизни процесса». | открыто |
| N-8 | Nit | `access/gate_test.go:171-198`; `access/gate_test.go:216-260`; `sender/telegram_test.go:438-447` | Три выживших мутанта — пробелы в тестах заявленного поведения: **R8b** — в тесте группы первым в каждом типе чата пишет участник allowlist, и серию одного `groupChat` открывает он, так что «ответить постороннему в группе один раз на серию» не ловится; **X11** — «`TooOften` — одна строка лога на серию» (dev-log итерации 2) ничем не закреплено; **X12** — не проверено, что `WithPolicy` не меняет политику исходного отправителя. При сборке в T-312 (`tg` для `flow`/`deliver`, `tg.WithPolicy(...)` для шлюза) такая регрессия незаметно сделала бы доставки best effort. | R8b: в тесте группы начинать с постороннего или дать каждому отправителю свою группу. X11: `strings.Count(logs, "update refused") == 1` после серии `TooOften`. X12: после `WithPolicy(BestEffortPolicy)` исходный отправитель на `5xx` делает `DefaultPolicy.Attempts` запросов. | открыто |

### Замечания к документам (не дефекты кода)

- В `tasks.md` исполнитель внёс строки DoD T-311, T-312 и T-315 по бэклогу ревью #1. Содержание верное. Владелец индекса — tech-lead#3, строки нужно подтвердить при приёмке; сюда же — строка SEC-07 самого T-310 (молчание).
- «Статус: review» в заголовке T-310 в `tasks.md` поставил исполнитель — оркестратору сверить с `state.js`.

### Предложения в бэклог

1. **T-312 (tech-lead#3):** строка DoD по Mi-4 (а) — отдельный отправитель шлюза с таймаутом ≤ 5 с. Тест X12 можно положить туда же, если не закрыт в T-310.
2. **security-engineer:** общий бюджет ответов на отказ на весь бот (Mi-4 б) и `/setjoingroups → Disable` в runbook (п. 6 ревью #1 остаётся в силе).
3. **architect#3:** пункты бэклога исполнителя итерации 2, п. 1, поддерживаю без изменений.
4. Остальные пункты бэклога ревью #1 (depguard `cmd-telegram-bot`, правки КД/ADR) — в силе.

### Риски и допущения

- Разбор M-1 опирается на семантику Bot API: `getUpdates` с `offset` подтверждает всё, что ниже, в момент получения запроса Telegram. Пограничный случай — запрос отправлен и отменён клиентом до ответа. Он входит в то же окно одного обновления.
- Оценка Mi-4 — чтением и расчётом. Реальный Telegram не вызывался, время ответа 0,1–0,3 с — типичное, не измеренное.
- `json.UnmarshalTypeError` в `error decode response result for method …` (без тела) может процитировать число, не влезшее в поле. Для id Telegram (int64) это неприменимо, риск не записан замечанием.
- Прогонов под `-race` не было, конкурентность `refusals` (один мьютекс, без вложенных блокировок) разобрана чтением.

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

## T-311 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-3)

### Границы ревью

Ветка `task/T-311-bot-flow-render`, папка `.worktrees/T-311`, база `ce7b35a`. Изменения не закоммичены (`git status`):
- новое: `cmd/telegram-bot/internal/flow` (5 файлов кода, 4 теста) и `cmd/telegram-bot/internal/render` (6 файлов кода, 3 теста), 3 300 строк;
- правки: `internal/gateway/client/client.go` (+46/−6), `client_test.go` (+45), `cmd/telegram-bot/internal/sender/telegram.go` (+9, `ReplyPolicy`), `access/gate_test.go` (SEC-06 на `client.New`), `tasks/T-311.md`, `dev-log.md`.

Коммитов вне ветки нет. `.golangci.yml`, `go.mod`, `shared/*`, контракты и КД не тронуты. Кончик эпика после базы ушёл на `745f6da` (T-305): тот коммит меняет только `internal/gateway/api/{middleware,router,openapi_test}.go`, пересечений с T-311 нет.

Основания:
- раздел T-311 в `tasks.md` со строками приёмок T-303, T-310, T-318, T-456;
- карточки `tasks/T-311.md` и `tasks/T-318.md` (§1 — тексты, §3 — подстроки, решения тимлида);
- КД `gateway-and-bot.md` §10.2–§10.5, §11.1;
- C-08 v1.5 («Дополнения v1.5», `contracts.md`);
- US-008, US-009; FR-009; `api-contracts.md` §1.2–§1.3.

Решения оркестратора приняты как данность:
- свободный текст у игрока с персонажем → список команд;
- копия правила имени в боте до `api.ValidCharacterName` (T-306);
- клиент не повторяет `forget_incomplete`;
- Р-2 A, Р-3 A;
- `/help` без `links/consent`.

### Вердикт

**ПРИНЯТЬ** — Critical 0 · Major 0 · Minor 7 · Nit 6.

Работа сделана тщательно:
- константы уведомления побайтно совпадают с T-318;
- инварианты согласия (кнопка только в `awaiting_consent`, `/help` без `consent`, `/forget confirm` без `resolve`) закреплены тестами и мутантами исполнителя;
- граница depguard и отказ от повтора `forget_incomplete` соблюдены.

Замечания — пробелы в тестах трёх объявленных свойств (Mi-1…Mi-3), текст ответа не по контракту (Mi-4), пустая часть при нарезке (Mi-5), редакция ошибок (Mi-6) и неверное утверждение о потокобезопасности (Mi-7). Все локальны. Их можно закрыть до слияния без повторного ревью: достаточно проверки тимлидом при приёмке.

### Прогоны (go1.26.8 windows/amd64, рабочая папка T-311)

| Команда | Результат |
|---|---|
| `go build ./... && go vet ./...` | 0 |
| `go test -short -count=1 ./cmd/telegram-bot/... ./internal/gateway/client/...` | ok ×9 пакетов; `updates` запустился напрямую, без `Access is denied` |
| `go test -short -count=1 ./internal/gateway/...` (регрессия клиента) | ok ×9 пакетов, в том числе `internal/gateway` (`compaction_test`, `context_test`) |
| `golangci-lint run ./cmd/telegram-bot/...` | 0 issues |
| `golangci-lint run ./...` | 0 issues (golangci-lint 2.13.2) |

`-race` недоступен (cgo выключен, gcc нет). Конкурентность разобрана чтением (Mi-7).

### Мутанты и пробы ревьюера

Копия `t311r1-mut` в scratch: `go.mod`, `go.sum`, `cmd/telegram-bot`, `internal/gateway/{api,client}`, `shared/{clock,env,eventbus,jsonpath,logging}` по `go list -deps -test`. Без `-overlay`, один мутант на прогон, исходник восстанавливался после каждого. Копия и скрипт удалены по точным путям.

| # | Мутант / проба | Итог |
|---|---|---|
| M0 (контрольный, первым) | без изменений | зелёный |
| M1 | `showNotice` ставит `awaiting_consent`, даже если `Send` уведомления упал | **выжил** → Mi-1 |
| M2 | `503 forget_incomplete` без `Reset(chat)` (только отметка) | **выжил** → Mi-2 |
| M3 | `notice_due` у согласившегося игнорируется (`start`, `idle`) | **выжил** → Mi-3 |
| M4 | `Handle` без `sweepLocked` | выжил → N-2 |
| M5 | `pack`: `<=` → `<` на границе 4096 | выжил → N-2 |
| M6 | клиент не повторяет любой контрактный `503` (а не только `forget_incomplete`) | красный: 6 тестов клиента, в том числе `TestForgetIncompleteIsNotRepeatedAndCarriesRetryAfter` |
| P1 | ответ `502` не по контракту (HTML прокси) на `/look` | игрок получает `"Bad Gateway"` → Mi-4 |
| P2 | `Split` строки длиннее 4096 с `\n` или `\n\n` в конце; `\n\n` перед длинным абзацем | 3 части, одна пустая; пустая часть — последняя и несёт клавиатуру → Mi-5 |
| P3 | `ConsentButton` у игрока с персонажем, затем `/look` | шаг `awaiting_consent`, `/look` отвечен уведомлением, действий 0 → N-1 |

Проверка текста уведомления программой (Python, блоки §1 карточки T-318 после приведения CRLF → LF):
- `NoticeText` (2 341), `ConsentButton` (21), `DeclineButton` (10), `DeclineReply` (161) **равны** константам `render/notice.go`;
- SHA-256 `NoticeText` совпадает с зашитым в `TestTheNoticeIsTheAcceptedText`;
- список 24 подстрок теста совпадает с таблицей §3 карточки. Каждая встречается ровно 1 раз, `30 дней` — 3 раза;
- нетипичные символы — только U+00AB, U+00BB, U+2014, U+2022;
- в кнопках нет U+00A0/200B/200C/200D/2060/FEFF, `TrimSpace` их не меняет;
- `notice.go` без CR и BOM. Go к тому же отбрасывает `\r` из raw-строк, так что `text=auto` на Windows текст не меняет.

### Разбор по пунктам поручения

**1. Константы `render/notice.go`** — выполнено, см. проверку выше.
- Нормализация — один `strings.NewReplacer("ё","е"," "," "," "," ","–","—")` для текста и подстрок (`notice_test.go:50`). `TestTypographyDoesNotFailTheCheck` применяет её к `ё`→`е`, U+00A0 перед «дней» и `—`→`–`. Кнопки сравниваются без нормализации.
- Четыре мутанта текста из DoD — постоянный тест `TestEditsOfMeaningFailTheCheck`.
- Константа кнопки одна: `grep` по не-тестовым `.go` находит «Мне есть 18, принимаю» и «Отказаться» только в `notice.go`. `flow` сравнивает `Message.Text == render.ConsentButton`/`DeclineButton` (`onboarding.go:74`, `:129-132`), клавиатура строится из тех же констант (`keyboards.go:14`).

**2. FSM.**
- Переходы соответствуют КД §10.3 с отклонениями Р-2 A, Р-3 A, Mi-5 и US-008.
- TTL 15 мин и кэш 1 ч считаются от `Clock.Now()`. Ожидание `202` идёт на `clock.Timers`, `time.Now`/`time.After` нет, `forbidigo` чист. Граница `>=` закреплена тестами `DialogTTL-1`/`DialogTTL` и `PlayerTTL`.
- `Reset` после `200` и после `forget_incomplete` (`forget.go:31`, `:41`). Для ветки `forget_incomplete` теста нет — Mi-2.
- Согласие ставится только после успешной отправки уведомления (`flow.go:214-219`). Код верен, теста нет — Mi-1.
- Путь к `links/consent` без нажатия кнопки не найден. `consent` вызывается ровно в одном месте (`onboarding.go:142`), из `awaitingConsent` по точному совпадению. `idle`, `/help`, `consent_required` ведут в `showNotice`.
- `/forget confirm` не вызывает `Resolve`: `forget` вызывается до разбора диалога (`flow.go:164-166`) и сам `resolve` не зовёт. Закреплено тестами `…WithoutResolve` и `…EndsTheSameForget`.
- Отменённый `ctx`: `Handle` возвращается сразу (`flow.go:153`), `fail` и `reply` молчат. Внутри хода между последовательными вызовами (`consent`→`start`→`resolve`, `create`→`Player`) отдельной проверки нет. Настоящий `http.Client` запрос с отменённым `ctx` не отправляет, поэтому это N-6.

**3. Приватность.**
- Ключи лога — константы `step, command, code, status, error` (`flow.go:130-136`), `msg` — литералы. Профиль (`Username`, `FirstName`) читается только в `matchesProfile` (`onboarding.go:191-199`), не хранится и не логируется. `TestTheLogCarriesNoIdentityNameOrText` проверяет ключи поверх `privacy.Handler`.
- Текст ошибки пишется как `err.Error()` без `privacy.Redact` (`flow.go:203`, `:229`). Сегодня в нём нет ни внешнего id, ни токена: клиент называет метод и путь, `Sender` редактирует сам. Но защита держится только на том, что T-312 подаст логгер поверх `privacy.Handler`, а в `Options.Log` это не записано — Mi-6.
- В текст игроку попадает только `message` из таблицы ошибок шлюза (`api.NewError` берёт сообщение из таблицы, `errors.go:141-148`). Внутренних деталей шлюза там нет. Исключение — ответ не по контракту: игрок видит английский `http.StatusText` (Mi-4).

**4. Клиент шлюза.**
- Условие `repeat(...) && !forgetIncomplete(status, data)` (`client.go:301`) снимает повтор только для `503` с кодом `forget_incomplete`. `bus_unavailable` у `/forget` по-прежнему повторяется (тест, 4 запроса), мутант M6 красный.
- `ForgetResult.Repeated` сохраняет смысл для сетевых ошибок.
- `APIError.RetryAfter` — целые секунды, дата и мусор дают 0 (`client.go:433-439`).
- Другие потребители. `grep` по `.worktrees/EPIC-00{1,2,3,4}` и `T-306` находит `client.Forget` только в `internal/gateway/{compaction,context}_test.go`. Там клиент с `Backoff = client.NoRetry` (`context_test.go:133`), и эти тесты ждут `forget_incomplete` ошибкой — поведение для них не меняется, пакет зелёный. `shared/testkit/gateway` `Forget` не вызывает.
- `forget_incomplete` и `RetryAfter` в других эпиках читают только сам шлюз и `api`.
- Строка DoD `tasks.md:297` («клиент повторяет сам (`DefaultBackoff`)») расходится с C-08 v1.5. По решению оркестратора её правят при приёмке — N-5.

**5. depguard.** `flow` импортирует из `internal/*` только `internal/gateway/api` и `internal/gateway/client`, `render` — только `api`. `shared/eventbus` не импортируется. Тесты `flow` дополнительно берут `shared/{clock,logging}`, что правило `cmd-telegram-bot` (lax) разрешает. `golangci-lint run ./cmd/telegram-bot/...` и `./...` — 0 issues.

**6. Риски.**
- **Блокирующий опрос `202 creating` до 10 с — бэклог, не Major.**
  - Опрос раз в секунду до 10 раз предписан КД §7.1 и UC-002 E4.
  - Последовательный обработчик принят КД §10.2 («≤ 6 игроков — достаточно»).
  - `202` возникает только при задержке `entity.created`, в норме это миллисекунды.
  - NFR-003 — Should.
  - Больше весит другое: каждый вызов шлюза в обработчике ограничен только `client.DefaultHTTPTimeout` 35 с × 4 попытки `DefaultBackoff`, и при зависшем шлюзе один ход держит всех дольше опроса. Это решает сборка T-312: клиенту `flow` нужен короткий `HTTP.Timeout`. См. бэклог п. 1–2.
- **Нарезка по 4096** считается в единицах UTF-16, как в Telegram, и проверена на эмодзи (`TestSplitCountsWhatTelegramCounts`). Режет по абзацам, затем по строкам и словам. Клавиатура уходит с последней частью. Дефект — пустые части (Mi-5).

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Mi-1 | Minor | `flow/flow.go:214-219`; `flow/onboarding_test.go` (нет теста) | **Инвариант «согласие — только после успешной отправки уведомления» не закреплён тестом.** Решение 2 карточки и пункт поручения. Мутант M1 (шаг `awaiting_consent` ставится и при ошибке `Send`) зелёный: после упавшего уведомления нажатие кнопки дало бы `links/consent` с `shown_at` текста, который игрок не получил (FR-009, NFR-045). Код верен. | Unit: `f.sent.FailNext(sender.ErrUnavailable)` → `/start` → `StepOf == Idle`; затем `ConsentButton` → 0 вызовов `consent`, ответ — `NoticeText` с клавиатурой. | открыто |
| Mi-2 | Minor | `flow/forget.go:31-32`; `flow/forget_test.go:100-130` | **`Reset` после `503 forget_incomplete` не закреплён.** Мутант M2 (без `Reset`, только отметка) зелёный. Тест после 503 шлёт только `/forget confirm`, а этот путь кэш не читает. С мутантом кэш `chat → player_id` удалённой связки живёт до часа, и `/look` уходит действием от забытого `player_id` (US-009, «`Reset` после forget»). | В `TestForgetIncompleteAsksToRepeat…` после первого ответа: `StepOf == Idle`; `/look` → вызов `resolve`, `actions` 0. | открыто |
| Mi-3 | Minor | `flow/onboarding.go:102-114` (`route`), `:36`, `:91`; `flow/mock_test.go:158` | **`notice_due` у согласившегося учитывается не везде и не тестируется.** (а) `route` (связка `consented`, персонажа нет или он `dead`) игнорирует `notice_due`. Вернувшийся через 30+ дней игрок с погибшим персонажем получает «прежний погиб» без повтора уведомления (FR-009 «после 30 дней неактивности», US-008 последний критерий, UC-001 A1; Should). (б) Фикстура `resolved` ставит `NoticeDue` только для несогласившихся, поэтому ветки `start`/`idle` с `NoticeDue` у согласившегося не выполняются ни одним тестом. Мутант M3 зелёный. | В `route` при `consented && NoticeDue` отправлять `NoticeText` (без клавиатуры согласия) перед подсказкой имени. Unit: `resolved("consented","alive"/"dead", …)` с `NoticeDue: true` → `NoticeText` первым сообщением; `/look` при промахе кэша → уведомление, затем «Принято.». | открыто |
| Mi-4 | Minor | `flow/flow.go:227-234`; `render/errors.go:45-55`; `internal/gateway/client/client.go:426` | **Ответ не по контракту показывается игроку английским статусом.** `apiError` для тела без `error.code` возвращает `APIError{Code: "", Message: http.StatusText}`. `fail` его принимает, `ErrorText` отдаёт `message` как есть: проба P1, `502` прокси → игроку «Bad Gateway». Так же будет с `503`/`504` без JSON после повторов. КД §10.5 требует для сети «сервис недоступен, повторите позже». Утверждение карточки «ответ не по контракту → «Сервис недоступен…»» верно только для `2xx` с неверным телом. | В `fail`: `apiErr.Code == ""` (или `Status >= 500` без кода) → `render.Unavailable`. Unit на `502` с HTML-телом. | открыто |
| Mi-5 | Minor | `render/delivery.go:109-136` (`pack`), `:60` | **`Split` отдаёт пустые части.** Пустой фрагмент рядом с частью длиннее 4096 уходит отдельным сообщением: длинная строка с `\n` в конце, длинный абзац с `\n\n` в конце, `\n\n` перед длинным абзацем (проба P2). В первых двух случаях пустая часть последняя и несёт клавиатуру. Telegram отклоняет пустой текст (`400`, `ErrRejected`), клавиатура теряется, а `deliver` T-312 получает ошибку посреди доставки. | В `pack` пропускать пустые (после `TrimSpace`) фрагменты при `flush` или отфильтровать части в `Split`. Unit на три формы из P2: частей без текста нет, клавиатура у последней непустой. | открыто |
| Mi-6 | Minor | `flow/flow.go:203`, `:229`; `:80-82` (`Options.Log`) | **Ошибка пишется в лог без редакции.** `slog.String(keyError, err.Error())` полагается на то, что логгер собран поверх `privacy.Handler`. `Options.Log` этого не требует, а T-312 может подать `logging.New(...)` напрямую. Соседние пакеты бота редактируют текст сами (`sender/telegram.go:212`, `updates/telegram.go:149`). Сегодня в тексте нет внешнего id: клиент называет метод и путь, `Sender` уже отредактирован. Это защита в глубину по пункту SEC-01/02. | `privacy.Redact(err.Error())` в обоих местах. В doc `Options.Log`: «должен быть построен на `privacy.NewHandler`» — строка DoD T-312. | открыто |
| Mi-7 | Minor | `flow/state.go:97-102` (`d.touched` до `Lock`), `:164-167` (`StepOf` читает `d.step` после `Unlock`); `flow/onboarding.go:177`, `:180`, `:207`, `:232`, `:285` | **Утверждение «`Reset`/`StepOf` из другой горутины безопасны» (карточка, риски) неверно для `StepOf`.** `dialogOf` возвращает указатель, и `StepOf` читает `d.step` вне мьютекса. `Handle` меняет `d.step`, `d.name` и `d.touched` без мьютекса. `StepOf` из `/health` T-312 параллельно с `Handle` — гонка данных. `-race` на Windows её не покажет. `Reset` безопасен: он только удаляет ключ карты. | Копировать шаг под мьютексом (`dialogOf` возвращает значение `dialog`, а `setDialog` пишет копию и `touched` под `Lock`) или записать в doc `StepOf`: «только из горутины `Handle`» и поправить риск карточки. Тест `StepOf` параллельно с `Handle` для CI с `-race`. | открыто |
| N-1 | Nit | `flow/onboarding.go:74-78` | Случайное нажатие `ConsentButton` игроком с персонажем переводит чат в `awaiting_consent` на 15 мин (проба P3): каждая игровая команда отвечается уведомлением, пока игрок снова не нажмёт кнопку (повторный `consent`) или не отправит `/start`. DoD этого требует («нажатие вне `awaiting_consent` → `NoticeText` и клавиатура»), но о тупике для играющего не говорит. | Оставить. В `HelpText` или в тексте рядом упомянуть `/start`; либо для чата с кэшем игрока показывать уведомление без смены шага. Решение tech-lead#3. | открыто |
| N-2 | Nit | `flow/flow.go:158`; `render/delivery.go:127` | Мутанты M4 (нет вычистки протухших чатов) и M5 (граница `<=` в `pack`) зелёные. Вычистка подкрепляет утверждение «chat id в памяти не дольше TTL и следующего обновления» (карточка, риски). | Вычистка — тест через `export_test.go` со счётчиком записей: два чата, один протух, после `Handle` другого чата запись удалена. Граница — две части ровно по 4096. | открыто |
| N-3 | Nit | `internal/gateway/client/client.go:433-439` | `time.Duration(secs) * time.Second` переполняется при `Retry-After` больше ~9,2·10⁹. Значение приходит от своего шлюза. | Ограничить сверху (например, `min(secs, 86400)`). | открыто |
| N-4 | Nit | `render/delivery.go:88-90` | Нарратив без текста превращается в сообщение из одной пометки «— текст создан ИИ». | Пустой нарратив — без сообщения, как остальные виды (`TrimSpace(d.Text) == ""` до `withMark`). | открыто |
| N-5 | Nit | `internal/gateway/client/client.go:1`; `tasks.md:297` | Doc пакета ссылается на «C-08 v1.3», поведение уже по v1.5. Строка DoD приёмки T-303 требует повтора `forget_incomplete` клиентом, что расходится с C-08 v1.5 (решение оркестратора — правка при приёмке). | «C-08 v1.5» в doc; строку DoD привести к контракту. | открыто |
| N-6 | Nit | `flow/onboarding.go:155-158`, `:265-270`, `:39` | Между последовательными вызовами шлюза внутри одного хода `ctx` отдельно не проверяется. С настоящим `http.Client` запрос не уходит; шлюз, не смотрящий на `ctx`, получил бы `resolve` после `consent` и `Player` после `create`. Тест `ctxBlindGateway` проверяет только вход в `Handle`. | `if t.ctx.Err() != nil { return }` перед `t.start()` в `consent` и перед опросом, либо оставить как есть с комментарием. | открыто |

### Вопросы к tech-lead#3 (через оркестратора)

1. **N-1:** принять тупик «кнопка согласия у играющего → 15 мин уведомлений» как есть или показывать уведомление без смены шага.
2. **Mi-3 (а):** повтор уведомления по `notice_due` в `route` (согласившийся без живого персонажа) — в T-311 или бэклогом вместе с T-320. Требование Should.

### Предложения в бэклог

1. **T-312 (строка DoD):** клиент шлюза для `flow` — короткий `HTTP.Timeout` (≤ 5–10 с) и отдельный от `deliver`. Сейчас один ход при зависшем шлюзе держит всех игроков до 35 с × 4 попытки. Логгер `flow` строится на `privacy.NewHandler` (Mi-6).
2. **EPIC-004, бэклог (поддерживаю п. 3 исполнителя):** на `202 creating` отвечать «создаётся» сразу и доводить через `/status` или доставку `system`, а не опросом в обработчике.
3. **T-306:** заменить `flow.ValidName` на `api.ValidCharacterName` (решение оркестратора, бэклог п. 1 исполнителя).
4. **system-architect:** КД §10.3 — `/help` без `links/consent`, Р-2 A, Р-3 A, текст кнопки, свободный текст → список команд (US-008); КД §10.5 — поведение при ответе не по контракту (Mi-4).
5. Поддерживаю п. 2, 4, 5 бэклога исполнителя.

### Риски и допущения

- Гонка Mi-7 выведена чтением: `-race` без cgo недоступен.
- Отказ Telegram на пустой текст (Mi-5) — известное поведение Bot API («message text is empty»). Реальный Telegram не вызывался.
- Регрессию клиента в других эпиках проверял `grep` по кончикам `.worktrees/EPIC-00{1,2,3,4}` и `T-306` (только чтение) и прогоном `./internal/gateway/...` в T-311. Ветки других эпиков не собирались.
- Доставку `deliver` (T-312) и e2e (T-314/T-315) не проверял — их ещё нет.

## T-317 · ревью #1 · 2026-09-13 · code-reviewer#3 (TEAM-3)

### Границы ревью
- Ветка `task/T-317-gateway-bot-docs` (`.worktrees/T-317`), база `745f6da` — совпадает с кончиком `epic/EPIC-004-gateway-bot`. Изменения не закоммичены:
  - новые: `internal/gateway/README.md`, `cmd/telegram-bot/README.md`;
  - правка: `Docs/ops/runbook.md`, только §6 (остальные разделы diff не затрагивает);
  - документы: карточка T-317 и запись dev-log.
- Сверено с:
  - раздел «### T-317» и DoD-common в `tasks.md`; строки приёмок T-310 и T-318 в DoD T-317; DoD T-311, T-312;
  - карточка T-318, «Условия, при которых текст правдив» и «Условия правдивости У-1…У-9 — носители», `NoticeText` §1;
  - карточка `EPIC-001-foundation/tasks/T-463.md` (статус `todo`, части (а)–(в), п. 6 «строка для T-317»);
  - КД `components/gateway-and-bot.md` §2 (таблица блоков), §11.4;
  - код: `internal/gateway/{context.go, api/router.go, api/middleware.go, handlers/links.go, links/forget.go, links/store.go, store/{open,compact,retention}.go}`, `cmd/telegram-bot/internal/{config,privacy,updates,access}`, `cmd/multiverse/{serve.go,main_test.go}`, `shared/env/vars.go`, `.env.example`, `.golangci.yml`, `docker-compose.yml`, `docker-compose.bot.yml`, `docker-compose.legacy.yml`, `Makefile`, `scripts/compose-lint.sh`, `shared/testkit/gateway/harness.go`.
- Карта владения не нарушена: код, `.env.example`, `CLAUDE.md` не менялись.

### Вердикт
**Вернуть.** Critical 0 · Major 2 · Minor 11 · Nit 8.

Сделано хорошо:
- списки переменных совпадают с манифестом и с тем, что код читает на самом деле;
- статус «готово / не готово» по T-306…T-309, T-311, T-312 и T-463 выделен отдельно, отсутствие `main.go` бота названо прямо;
- проверены и верны: маршруты, порядок middleware, поведение `replay`, коды выхода, `409`, лимиты отказов;
- условия правдивости У-1, У-2, У-4, У-5 названы условиями, а не готовыми фактами.

Возврат из-за двух Major:
- Ma-1: `/health` описан с проверкой БД, которой в коде нет;
- Ma-2: процедура ротации предлагает вписать токен в URL, и он попадает в историю оболочки. Сама проверка при этом делается отозванным токеном и ничего не показывает.

### Проверено ревьюером
- `go run ./cmd/mvctl env check` — `74 variables declared, compared with .env.example`, код 0.
- Относительные ссылки и пути — скрипт Python в scratch (`t317r1-links.py`), удалён по точному пути. Markdown-ссылок `[..](..)` в новых README нет, все пути даны в обратных кавычках. Каждый путь, названный существующим, есть в дереве. `api/gateway.openapi.yaml` находится только от корня (N-3). Пути, помеченные как будущие (`internal/flow`, `render/notice.go`, `outbox/`, `snapshot/`), отсутствуют, как и сказано. Таблицы корректны: число столбцов в строках совпадает.
- Секреты: grep по трём файлам на `\d{5,}:[A-Za-z0-9_-]{10,}`, `AAA…`, `minioadmin`, `password` — пусто. Токены только в виде плейсхолдеров `<токен>`, `<цифры>:<секрет>`.
- Цели `make up`, `make logs`, `make health`, переменные `PROFILES`/`COMPOSE_ENV_FILES` и правила 2 и 4 `compose-lint` есть в `Makefile` и `scripts/compose-lint.sh`. Команды Docker, стенда и сети не запускались.

Верно по дереву:
- переменные `MV_GATEWAY_*`, `MV_TELEGRAM_*`, `MV_WORLD_ID`, `MV_MINIO_*`, `MV_GM_PATH` и их смысл;
- `--bus=memory` требует `--contexts=all` (`serve.go:160`, `main_test.go:80`), `MV_CORE_ADDR` по умолчанию `127.0.0.1:8090`;
- 4 маршрута (`router.go:67-70`), порядок `Chain` (`middleware.go:100-103`), 64 КиБ, 5 с, `noLogOperations`;
- `replay` без лимитера и уборки (`context.go:215-231`);
- поля `/health`: `projection`, `mode`, `links_compaction`, `projection_error`, `bus` (кроме Ma-1);
- `ExitCode` 0/1/3 (`updates.go:43-51`), `409` из-за второго экземпляра или webhook (`telegram.go:83`), `WithUpdatesChannelCap(0)`;
- `config.Load`: форма токена, `ErrTokenMissing`, соль и её длина; `Redact` `bot<цифры>:…`;
- отказ «Доступ по приглашению.»: не больше 1 на чат и 10 на весь бот (`gate.go:49`), `bot_denied_total`;
- порты `127.0.0.1:8088` и `127.0.0.1:8089`, токен только через `environment:` (`docker-compose.bot.yml`), соль и лимит команд в compose не переданы (T-463 (в));
- `/setjoingroups` → Disable: и в README, и в runbook, с верной причиной (SEC-07).

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Ma-1 | Major | `internal/gateway/README.md:117-118` | «`links_store`, `gateway_store` — `ok`/`fail` (реальная проверка БД, кэш результата не чаще раза в 10 с)». В коде обе детали — константа `runtime.StatusOK`, пока контекст запущен (`context.go:460-461`). Ни проверки БД, ни кэша нет. КД §11.4 прямо говорит: «В коде T-303 `links_store` и `gateway_store` — `ok`, пока контекст запущен; настоящая проверка БД … — T-309». Нереализованное описано как готовое, а оператор будет доверять `ok` при недоступной базе. | «`links_store`, `gateway_store` — сейчас всегда `ok`, пока контекст запущен; настоящая проверка БД — T-309». Заодно: `bus` появляется только со значением `fail`, `bus: ok` в ответе нет. | открыто |
| Ma-2 | Major | `cmd/telegram-bot/README.md:119-121`; `Docs/ops/runbook.md:512-516` | «(`https://api.telegram.org/bot<токен>/getWebhookInfo` — `deleteWebhook`, если он есть)». (1) Совет исполняется вставкой токена в командную строку или адресную строку браузера. Токен уходит в историю оболочки (`~/.bash_history`, история PSReadLine), в аргументы процесса и в историю и синхронизацию браузера. Процедура по SEC-08 сама создаёт утечку. (2) Шаг 5 runbook: «проверить, что webhook для него [старого токена] не установлен». После `/revoke` из шага 1 старый токен получает `401`, и проверка ничего не показывает. Проверять надо новым токеном. | Шаг «проверить webhook» — новым токеном и без токена в команде. Например, Git Bash: `IFS= read -rs TG_TOKEN` (ввод не отображается и не пишется в историю) → `printf 'url = "https://api.telegram.org/bot%s/getWebhookInfo"\n' "$TG_TOKEN" \| curl -sS -K -` → при непустом `"url"` тот же приём с `deleteWebhook` → `unset TG_TOKEN`. `printf` встроен в оболочку, а `curl -K -` читает URL из stdin, поэтому токена нет ни в истории, ни в argv. Добавить явный запрет: не открывать URL с токеном в браузере и не вставлять в чаты вывод `docker compose config` и `docker inspect telegram-bot`, где токен виден открыто. | открыто |
| Mi-1 | Minor | `internal/gateway/README.md:164-168` | «`/forget` … планирует сжатие … если сжатие не завершилось сразу, … повторный `/forget` отвечает `503 forget_incomplete`, пока фоновая уборка (раз в час, `sweep`) не закончит его». По коду всё иначе. Сжатие выполняется в том же вызове, а не планируется (`forget.go:117-120`). Первый же `/forget` отвечает `503` (`handlers/links.go:148-151`). Повтор сам пробует сжатие и при успехе отвечает `200 {deleted:false}` (`forget.go:88-93`). `Sweep` добивает отметку раз в минуту (`retention.go:13`, `links/store.go:270`), раз в час идёт отдельный `Compact`, ещё раз — при старте. | «`/forget` удаляет строку и сразу сжимает `links.db`. Если сжатие не прошло (например, базу держит читатель), ответ — `503 forget_incomplete` с `Retry-After: 5`, `/health` — `degraded` с `links_compaction: pending`. Отметку снимает первый успешный повтор `/forget`, уборка раз в минуту, часовое сжатие или рестарт». | открыто |
| Mi-2 | Minor | `internal/gateway/README.md:27, 32, 53` | Таблица «эти пакеты в дереве отсутствуют» включает `shared/testkit/gateway`, но пакет есть (`harness.go`, `Harness` v0, T-018). Нет только `FakeGateway` с HTTP-обвязкой. Строка 53: `client/` «используется ботом, `shared/testkit/gateway`». Сейчас его импортируют только тесты `internal/gateway` и `cmd/multiverse`, бот начнёт с T-311. | Строка: «`FakeGateway` и HTTP-обвязка рядом с `Harness` v0 (`shared/testkit/gateway` уже есть) — T-308». Для `client/`: «клиент C-08; потребители — бот (с T-311) и `FakeGateway` (T-308)». | открыто |
| Mi-3 | Minor | `internal/gateway/README.md:61-73` | Команды «Как поднять локально» не задают `MV_GATEWAY_DATA_DIR`. По умолчанию это `/data` (`vars.go:98`). На Linux и macOS без root старт падает, на Windows создаётся `\data` в корне диска (runbook §2, строки 189-194). Строка 101 таблицы это упоминает, но команды в таком виде не работают. | Перед обеими командами: `export MV_GATEWAY_DATA_DIR="$HOME/.multiverse/gateway"` (вне репозитория) со ссылкой на runbook §2. | открыто |
| Mi-4 | Minor | `cmd/telegram-bot/README.md:186-189` | «правило для `cmd/telegram-bot` пока не принято отдельно; граница проверяется тестом `go list -deps`». На этом дереве правило есть: `cmd-telegram-bot` (`.golangci.yml:385-396`) и исключение в `internal-unlisted` (`:178-179`). | «Граница — правило depguard `cmd-telegram-bot` (только `internal/gateway/client` и `internal/gateway/api`, без `shared/eventbus`); вторая линия — тест `go list -deps` в `internal/config/imports_test.go` (переедет в пакет бота с T-312)». | открыто |
| Mi-5 | Minor | `cmd/telegram-bot/README.md:5-7` | «Собственного хранилища у бота нет: состояние диалога и связка `chat_id → player_id` — на стороне шлюза (`links.db`)». По КД §2 (таблица блоков) у бота «ничего персистентного; в памяти — только состояние диалога на чат (TTL), кэш `chat_id → player_id` (TTL)». FSM `flow` с TTL 15 мин тоже живёт в боте (T-311). | «Персистентного хранилища у бота нет: связка аккаунта с игроком — в `links.db` шлюза; состояние диалога и кэш `chat_id → player_id` — в памяти бота с TTL (T-311)». | открыто |
| Mi-6 | Minor | `cmd/telegram-bot/README.md:72` | Пример `make up PROFILES=memory,bot` стоит в том же README, где правило У-2 (строки 143-147) запрещает `memory`, пока в allow-list есть кто-то кроме владельца. Пример подталкивает к нарушению условия правдивости FR-009. | Пример оставить, но рядом написать: «`memory`/`legacy` — только пока allow-list состоит из владельца (У-2, «Данные игроков»)». Для внешних игроков пример — `make up PROFILES=bot`. | открыто |
| Mi-7 | Minor | `cmd/telegram-bot/README.md:78, 131-133` | Прямые `docker compose -f docker-compose.yml -f docker-compose.bot.yml up -d telegram-bot` и `… logs` даны без `COMPOSE_ENV_FILES`. Без переменной compose падает на первом `*_IMAGE` (врезка runbook, строки 28-36; шапка `docker-compose.bot.yml`). В runbook это закрывает общая врезка, в README предупреждения нет. | Перед командами: `export COMPOSE_ENV_FILES=.env,build/versions.env` (PowerShell: `$env:COMPOSE_ENV_FILES = ".env,build/versions.env"`) со ссылкой на врезку runbook. | открыто |
| Mi-8 | Minor | `Docs/ops/runbook.md:523-524, 535-538` | (1) Проверки через голый `docker compose ps` зависят от набора `-f`. Без `-f docker-compose.bot.yml` или `-f docker-compose.legacy.yml` сервисов `telegram-bot`, `chromadb` и `semantic-memory` нет в модели, и `ps` может их не показать. Список У-2 неполон: нет `memory` (`docker-compose.yml:392-394`) и `narrative-orchestrator` (`docker-compose.legacy.yml:113-114`). (2) `/telegram-bot health --url …` на хосте не существует: бинарник есть только в образе. | (1) Проверка, не зависящая от файлов: `docker ps --filter label=com.docker.compose.project=<проект> --format '{{.Names}}'`, где нет `qdrant`, `neo4j`, `memory`, `chromadb`, `semantic-memory`, `narrative-orchestrator`. (2) Здоровье бота — `make health PROFILES=<набор>,bot` (проба `127.0.0.1:8089`, `Makefile:386-388`) или `docker compose -f docker-compose.yml -f docker-compose.bot.yml exec telegram-bot /telegram-bot health` после T-312. | открыто |
| Mi-9 | Minor | `Docs/ops/runbook.md:565-581` | Прежний хвост §6 («**`PROFILES=` замещает активный набор…**» и «Проверка: … `logs --tail=50`») остался после новых подразделов. Теперь он стоит под заголовком «Другие правила эксплуатации бота», к которому не относится, и повторяет шаги 3-4 ротации (строки 499-511). | Предупреждение про `PROFILES=` перенести в шаг 3 (одна фраза и ссылка на раздел 2), повтор «Проверка: …» удалить. | открыто |
| Mi-10 | Minor | `Docs/ops/runbook.md:495-498, 520` | Чек-лист: «`MV_TELEGRAM_BOT_TOKEN` в `.env` обновлён, нигде больше не хранится». Шага для копий `.env` вне рабочей папки нет. Раздел 4 (строка 453) восстанавливает `.env` «из менеджера паролей владельца». Без обновления записи там восстановление на чистой машине вернёт отозванный токен, и бот выйдет с кодом 1. | Шаг 2: «обновить запись `.env` в менеджере паролей (раздел 4); резервные копии `.env` рядом с репозиторием обновить или удалить». Пункт чек-листа — так же. | открыто |
| Mi-11 | Minor | карточка T-317, «Выполнение»; `tasks.md:410, 411, 414` | Два пункта DoD без следа выполнения. (1) «Раздел runbook … проверен „сухим прогоном“ (шаги выполнимы без доступа к прод-токену)». (2) Согласование правки runbook с devops (поле «Метка» карточки; строки приёмок T-318 и T-310). В карточке и dev-log нет ни результата, ни явного переноса. | (1) Записать результат сухого прогона: какие шаги пройдены без токена; шаги, требующие бинарника, — перенос на T-390 со ссылкой. (2) Согласование devops — через tech-lead#3; отметить в карточке. | открыто |
| N-1 | Nit | `cmd/telegram-bot/README.md:131-133`; `Docs/ops/runbook.md:508-511, 522` | Проверка «токена нет» — глазами по `logs --tail=50`. Утечка при старте может уйти выше 50 строк, а глазами токен узнает только тот, кто его помнит. | `… logs telegram-bot \| grep -cE 'bot[0-9]+:[A-Za-z0-9_-]{20,}'` → `0`: шаблон без самого токена, по всему логу контейнера. | открыто |
| N-2 | Nit | `Docs/ops/runbook.md:491-494, 519` | «`/token` для получения нового без явного отзыва старого». Фраза читается так, будто старый токен останется рабочим. Ревьюер это не проверял (сеть вне поручения), но у бота один действующий токен. | Нейтрально: «`/revoke` (или «API Token → Revoke» в меню бота) — выпустить новый токен; старый перестаёт действовать». Сверить с текущим интерфейсом @BotFather при сухом прогоне (Mi-11). | открыто |
| N-3 | Nit | `internal/gateway/README.md:134-135` | Путь `api/gateway.openapi.yaml` в README каталога `internal/gateway` читается как `internal/gateway/api/…`, а файл лежит в корне. «Остальные операции появляются вместе с T-306/T-307» — операции I2 появятся с T-352, T-354 и T-356. | «`api/gateway.openapi.yaml` в корне репозитория»; «с T-306/T-307 (I1) и T-352/T-354/T-356 (I2)». | открыто |
| N-4 | Nit | `internal/gateway/README.md:161-162, 165` | «более широкие права — отказ старта»: на Windows проверка прав пропускается (`store/open.go:213-215`). «`checkpoint` + `incremental_vacuum`»: в коде порядок обратный (`store/compact.go:28-32`). | «(кроме Windows, где POSIX-прав нет)»; «`incremental_vacuum` + `wal_checkpoint(TRUNCATE)`». | открыто |
| N-5 | Nit | `internal/gateway/README.md:37-38, 129-130, 148-150`; `cmd/telegram-bot/README.md:17-19, 103-104` | Язык. (1) «Не описывай эти возможности как готовые…» — указание авторам на «ты» в документе оператора. (2) «— трекер задачи» — непонятно. (3) «не логируют тело и код причины ошибки за пределами `request_id`/`code`» — по коду в лог идут ровно `request_id` и `code` (`middleware.go:180-191`), а фраза читается как «код не логируется». (4) «см. «Открытые вопросы» отчёта задачи» — отчёта в репозитории нет. (5) Комментарий compose дан в кавычках как цитата, хотя в файле он по-английски. | (1) Убрать или заменить на «Статус обновляется вместе с T-306…T-309». (2) «полный состав — T-309». (3) «Для них в журнал запроса пишутся только `request_id` и `code`». (4) Ссылка на карточку T-463, часть (в). (5) Пересказ без кавычек. | открыто |
| N-6 | Nit | `cmd/telegram-bot/README.md:167-168`; `Docs/ops/runbook.md:531-533` | DoD (`tasks.md:411`) требует ссылку на карточку T-318, а в тексте только путь в кавычках. Кликабельных ссылок в новых README нет вообще. | `[tasks/T-318.md](../../Docs/dev-team/epics/EPIC-004-gateway-bot/tasks/T-318.md)`; так же для runbook (`../dev-team/epics/…`) и КД. | открыто |
| N-7 | Nit | карточка T-317:10; `tasks.md:401` | «Ветка» — `task/T-317-docs-gateway-bot`, фактическая ветка — `task/T-317-gateway-bot-docs`. | Оркестратору: привести к фактическому имени. | открыто |
| N-8 | Nit | `cmd/telegram-bot/README.md:137-139`; `Docs/ops/runbook.md:528-529` | Вводная пересказывает обещание `/start` («записи удаляются автоматически», «копия связки ≤ 30 дней») без фразы, на которой стоит У-1: «В резервных копиях игры эти записи могут храниться ещё до 30 дней сверх указанных сроков». Сами правила верны. | Добавить эту фразу в вводную, чтобы правило про архивы `make backup` читалось как следствие. | открыто |

### Вопросы tech-writer — рекомендация
1. **Нет `main.go` бота.** README и §6 runbook не откладывать до T-312: статус «бинарника нет» записан честно, этого достаточно. Предложение tech-lead#3 — строка DoD T-312: «`cmd/telegram-bot/README.md` (статус, `/health`, коды выхода из `main`) и `Docs/ops/runbook.md` §6 (снять пометку „выполнимо после T-312“, сухой прогон ротации) обновлены». И строка DoD T-311: «таблица статуса `cmd/telegram-bot/README.md` — `flow`/`render` готовы».
2. **Строка для `CLAUDE.md`.** В этой задаче не править: DoD (`tasks.md:412`), владелец — tech-writer (EPIC-001 F-9). `CLAUDE.md` живёт в `develop`, а `internal/gateway` пока только в ветке эпика. Поэтому правка идёт одним заходом при контрольном слиянии I1-α или I1 EPIC-004 в `develop`. Там же исправить устаревшее «`gateway` — заглушка» в «Статус кода». Предлагаемые строки карты каталогов: `cmd/telegram-bot/  # Telegram-бот игрока (EPIC-004); main.go — T-312, до неё бинарник не собирается (cmd/telegram-bot/README.md)` и `internal/gateway/  # контекст gateway (C-08): links, actions, readmodel, consumer; статус — internal/gateway/README.md`.

### Предложения в бэклог (вне границ T-317)
1. Runbook §8, карточка `gateway` (строка 606): «том `gateway-data` зарезервирован, `internal/gateway` ещё не реализован» — неверно с T-303. Раздел 4 (строки 457-461) про `multiverse db backup` для `links.db` сверить с деревом. Владелец — tech-writer и devops.
2. T-463 п. 6: когда появится `multiverse db backup`, в README бота и runbook записать формат имён `links-<date>.db.age` и `gateway-<date>.db` и порядок «открытая копия удаляется сразу после `docker cp`».
3. `docker-compose.bot.yml`: порт `127.0.0.1:8089:8089` и URL healthcheck зашиты литералами, при другом `MV_TELEGRAM_HEALTH_ADDR` проба промахнётся. Для `gateway` это уже решено через `MV_CORE_ADDR` (T-408). Решает devops.

### Риски и допущения
- Поведение Telegram (`401` на отозванный токен, `409` при активном webhook, работа `/token` у @BotFather) проверено по коду и документации проекта, без сети.
- Утверждение Mi-8 «`docker compose ps` может не показать сервисы вне модели» зависит от версии compose; предложенная проверка через `docker ps` от версии не зависит.
- Docker, стенд LLM, `.env` и `— копия.env` не трогал. В рабочей папке задачи ревьюер добавил только этот раздел и строку в карточке.

## T-317 · ревью #2 · 2026-09-13 · code-reviewer#3 (TEAM-3)

### Границы ревью
- Ветка `task/T-317-gateway-bot-docs` (`.worktrees/T-317`), база `745f6da`. Изменения не закоммичены: новые `internal/gateway/README.md` и `cmd/telegram-bot/README.md`; правка `Docs/ops/runbook.md` (по diff затронут только §6); карточка T-317 и dev-log.
- Повторная итерация: проверены исправления Ma-1, Ma-2, Mi-1…Mi-11, N-1…N-8 и регрессия в переписанных местах. Каждое утверждение исправлений сверено с кодом дерева, а не с текстом ревью #1.
- Сверено с кодом и конфигурацией: `internal/gateway/{context.go, handlers/links.go, links/forget.go, links/store.go, store/{compact,open,retention}.go, api/middleware.go}`, `cmd/telegram-bot/internal/{access/gate.go, privacy/privacy.go, updates/updates.go, config/imports_test.go}`, `shared/env/vars.go`, `.golangci.yml` (`internal-gateway`, `cmd-telegram-bot`), `docker-compose.yml`, `docker-compose.bot.yml`, `docker-compose.legacy.yml`, `Makefile` (`ACTIVE_PROFILES`, `health`, `logs`), `shared/testkit/gateway`, карточка T-318 («Условия правдивости»).
- Карта владения не нарушена: код, `.env.example`, `CLAUDE.md`, `tasks.md` не менялись.

### Вердикт
**Принять.** Critical 0 · Major 0 · Minor 2 · Nit 2 (новые; открытые из ревью #1 — только Mi-11, часть 2, и N-7 для `tasks.md` — адресованы оркестратору).

Оба Major закрыты. Новые Minor касаются только удобства процедуры проверки webhook и не создают утечку: их можно исправить в T-312, когда снимается пометка «выполнимо после T-312» (строка DoD T-312 из ревью #1), или отдельной правкой до приёмки — на усмотрение tech-lead#3.

### Проверено ревьюером
- `go run ./cmd/mvctl env check` — `74 variables declared, compared with .env.example`, код 0.
- Ссылки: скрипт Python в scratch (`t317r2-links.py`), удалён по точному пути. Относительных Markdown-ссылок: `cmd/telegram-bot/README.md` — 2, runbook — 4, карточка T-317 — 3; битых — 0. Пути в обратных кавычках, которых нет в дереве, — только заявленные будущими (`internal/flow`, `internal/render`, `internal/deliver`, `render/notice.go`, `outbox/`, `snapshot/`, `bin/telegram-bot`) и имена символов или библиотек. `api/gateway.openapi.yaml` есть в корне.
- Синтаксис процедуры webhook проверен по документации, без выполнения и без сети:
  - `help read` (bash в Git Bash): `-r` не даёт обратной косой черте экранировать символы, `-s` не выводит ввод с терминала. Без `-e` ввод не идёт через Readline и в историю не попадает. `IFS=` сохраняет пробелы по краям;
  - `printf` — встроенная команда оболочки (`type printf`), поэтому токен не попадает в argv внешнего процесса. В истории остаётся литерал `"$TG_TOKEN"`. Переменная не экспортирована и не передаётся `curl` через окружение;
  - `curl --manual` (8.21.0), `-K, --config`: имя файла `-` — чтение конфигурации из stdin. URL задаётся строкой `url = "…"`, в кавычках значимы только `\\ \" \t \n \r \v`. В токене (`[0-9]+:[A-Za-z0-9_-]+`) таких символов нет. Продолжение строки `\` с `| curl` на следующей строке в runbook записано верно;
  - запрет выкладывать `docker compose config` и `docker inspect telegram-bot` есть в обоих местах. Интерполяция `${MV_TELEGRAM_BOT_TOKEN:?…}` в `docker-compose.bot.yml:63` подтверждает, что в выводе токен открыт.
- Команды Docker, стенда, сети и `curl` к Telegram не запускались.

### Статус замечаний ревью #1

| # | Статус | Проверка по дереву |
|---|---|---|
| Ma-1 | закрыто | `internal/gateway/README.md:124-126` — «сейчас всегда `ok`, пока контекст запущен; настоящая проверка — T-309» — совпадает с `context.go:460-461`. Ключ `bus` только со значением `fail` (`context.go:477-480`) — верно. `fail` до `Start` и после `Stop` (`context.go:406-409, 455-456`), значения `projection` `ok/missing/stale` (`readmodel/model.go:43-49`) — верно. |
| Ma-2 | закрыто | Оба места: проверка **новым** токеном, `IFS= read -rs` → `printf … \| curl -sS -K -` → `unset`. Токена нет в URL команды, argv и истории; запрет на URL в браузере и на вывод `config`/`inspect` есть. Шаг «проверить старым токеном» удалён. Остались недочёты PowerShell-варианта и порядка шагов — новые Mi-12, Mi-13, утечки нет. |
| Mi-1 | закрыто | Сжатие в том же вызове (`forget.go:117-120`); `503 forget_incomplete` + `Retry-After: 5` (`handlers/links.go:31, 148-151`); отметку снимают повтор (`forget.go:88-93`), `Sweep` раз в минуту (`links/store.go:270-271`, `retention.go:13`), часовой `Compact` (`context.go:389`), `Stop`/рестарт (`context.go:164, 424-428`). |
| Mi-2 | закрыто | `shared/testkit/gateway` есть (`harness.go`), строка переписана; `client/` сейчас импортируют только тесты — «бот (с T-311) и `FakeGateway` (T-308)» верно. |
| Mi-3 | закрыто | `export MV_GATEWAY_DATA_DIR=…` перед обеими командами; умолчание `/data` — runbook §2. |
| Mi-4 | закрыто | Правило `cmd-telegram-bot` (`.golangci.yml:385-396`): allow `internal/gateway/client$`, `internal/gateway/api$`, deny `internal`, `shared/eventbus`; тест `go list -deps` (`config/imports_test.go:37`). |
| Mi-5 | закрыто | Вводная соответствует КД §2. |
| Mi-6 | закрыто | Условие У-2 и `PROFILES=bot` рядом с примером. |
| Mi-7 | закрыто | `COMPOSE_ENV_FILES` (bash и PowerShell) перед прямой командой; соответствует шапке `docker-compose.bot.yml:20-22`. |
| Mi-8 | закрыто | `docker ps --filter label=com.docker.compose.project=…`; список дополнен `memory` (`docker-compose.yml:392`) и `narrative-orchestrator` (`docker-compose.legacy.yml:113`). `make health` пробует `127.0.0.1:8089` (`Makefile:386-388`). |
| Mi-9 | закрыто | Хвоста §6 нет; предупреждение `PROFILES=` — в шаге 3 со ссылкой на раздел 2. |
| Mi-10 | закрыто | Шаг 2 и чек-лист: менеджер паролей (раздел 4, строка 453) и копии `.env`. |
| Mi-11 | частично | (1) Сухой прогон записан в карточке по шагам; перенос шагов 3–4 на T-390 обоснован (нет `main.go`). (2) Согласование правки runbook с devops не выполнено, передано оркестратору и tech-lead#3. Ревью это не блокирует, но это пункт DoD: закрыть до приёмки. |
| N-1 | закрыто | `grep -cE` по всему логу; шаблон совпадает с формой, которую вырезает `privacy.go:38`. Уточнение — N-9. |
| N-2 | закрыто | Формулировка нейтральна, сверка с @BotFather — открытый пункт живого прогона. |
| N-3 | закрыто | «в корне репозитория»; T-352/T-354/T-356. |
| N-4 | закрыто | `store/open.go:213-215` (Windows); порядок `incremental_vacuum` → `wal_checkpoint(TRUNCATE)` (`store/compact.go:28-32`). |
| N-5 | закрыто | Фраз «Не описывай», «трекер», «отчёт задачи» нет; «только `request_id` и `code`» — верно (`middleware.go:36, 180-191`); комментарий compose пересказан без кавычек и по смыслу верно (`docker-compose.bot.yml:53-55`). |
| N-6 | закрыто | Ссылки на T-318 (оба README, runbook), T-463, КД — все резолвятся. |
| N-7 | частично | Карточка исправлена; `tasks.md:401` по-прежнему `task/T-317-docs-gateway-bot` — у оркестратора. |
| N-8 | закрыто | Фраза У-1 в обоих местах дословно совпадает с `NoticeText` (T-318, строка 66). |

### Новые замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Mi-12 | Minor | `Docs/ops/runbook.md:540-541`; `cmd/telegram-bot/README.md:139-140` | PowerShell-вариант дан словами («аналогично, токен через `Read-Host -AsSecureString`, конфигурация `curl` через stdin»), готовой команды нет. Утечки текст не создаёт, но выполнить его буквально нельзя. (1) В Windows PowerShell 5.1 `curl` — псевдоним `Invoke-WebRequest`, и `curl -sS -K -` падает на разборе параметров. (2) Из `SecureString` ещё нужно получить строку, и способ не назван. Оператор будет импровизировать, и самый короткий путь — вставить токен в URL, то есть тот сценарий, который закрывал Ma-2. | Дать команду: `$s = Read-Host -AsSecureString` → `$t = [System.Net.NetworkCredential]::new('', $s).Password` → ``"url = `"https://api.telegram.org/bot$t/getWebhookInfo`"" \| curl.exe -sS -K -`` → `Remove-Variable s, t`. Явно `curl.exe`, не `curl`. В истории PSReadLine остаётся литерал `$t`; ввод `Read-Host` в историю не пишется. | открыто |
| Mi-13 | Minor | `Docs/ops/runbook.md:511-543` | Порядок шагов противоречит тексту. Шаг 3 перезапускает бота, шаг 5 проверяет webhook и заканчивается фразой «снять его перед перезапуском». Если оператор идёт по порядку и webhook у бота есть, после шага 3 процесс выходит с кодом 3 (`updates.go:47-48`) и уходит в цикл рестартов. README (`:142-143`) сам предупреждает об этом цикле. | Переставить проверку webhook перед перезапуском (шаг 3 ↔ шаг 5, чек-лист — в том же порядке). Второй вариант — в шаге 5 написать: «если после шага 3 бот вышел с кодом 3 — снять webhook и перезапустить ещё раз». | открыто |
| N-9 | Nit | `Docs/ops/runbook.md:526-527`; `cmd/telegram-bot/README.md:157` | Шаблон `bot[0-9]+:[A-Za-z0-9_-]{20,}` ловит только форму с префиксом `bot`, то есть только то, что вырезает `Redact` (`privacy.go:38`). Голый `<цифры>:<секрет>`, например из будущей ошибки конфигурации `main.go`, проверка «токена нет» пропустит. | `grep -cE '[0-9]{5,}:[A-Za-z0-9_-]{30,}'` покрывает обе формы, `bot<redacted>` не совпадает. | открыто |
| N-10 | Nit | `Docs/ops/runbook.md:490, 521, 543`; `internal/gateway/README.md:127` | (1) В runbook пути `internal/updates.ErrUnauthorized`, `internal/privacy`, `internal/updates.ErrConflict` читаются от корня репозитория, где `internal/` — контексты платформы, а этих пакетов нет. Полный путь назван только во врезке «Статус». (2) «нового мира ещё нет снапшота» — пропущено «у». | (1) `cmd/telegram-bot/internal/…` при первом упоминании в шаге или один раз после врезки «Статус». (2) «у нового мира ещё нет снапшота». | открыто |

### Предложения в бэклог
- Без изменений к ревью #1: runbook §8, карточка `gateway`; формат имён копий по T-463 п. 6; литералы порта в `docker-compose.bot.yml`.
- Строки DoD T-311, T-312 и строки `CLAUDE.md` в карточке (раздел «Передать») записаны верно, дословно из ревью #1. Внести их — оркестратору и tech-lead#3.

### Риски и допущения
- Поведение Telegram не проверялось по сети: `409` на `getUpdates` при активном webhook, судьба webhook после `/revoke`, путь меню @BotFather. Процедура это оговаривает («сверить при живом прогоне», T-390/T-391).
- Утверждение Mi-12 о псевдониме `curl` относится к Windows PowerShell 5.1. В PowerShell 7 псевдонима нет, но `curl.exe` работает в обеих версиях.
- Docker, стенд LLM, `.env` и «— копия.env» не трогал. В рабочей папке задачи добавил только этот раздел и строку в карточке.

## T-306 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-3)

### Границы ревью
- Ветка `task/T-306-characters-sessions` (`.worktrees/T-306`), база — кончик эпика `745f6da`. Изменения не закоммичены: 14 изменённых файлов и 23 новых.
- Код:
  - пакеты `internal/gateway/{characters,session,turns}`;
  - `api/names.go`, `api/router.go`, `handlers/characters.go`, `context.go`;
  - `links/{store,forget}.go`, `readmodel/model.go`, `actions/{service,turns}.go`;
  - `shared/env/vars.go`, `.env.example`, `api/gateway.openapi.yaml`;
  - фикстура `internal/gateway/testdata/analytics/solo-30.jsonl` и тесты.
- Сверено с:
  - разделом «T-306» в `tasks.md`, включая строки DoD приёмки T-303 и T-305;
  - карточкой `tasks/T-306.md`;
  - КД шлюза §4.2, §5.2, §6, §7.1, §7.6, §11.3;
  - C-08 (коды `api/errors.go`) и C-10 (`contracts.md` §10, схемы `schemas/events/analytics.*.v1.json`);
  - ADR-009 (дополнение, п. 2) и ADR-019;
  - кодом T-302…T-305: `store/open.go`, `consumer`, `actions/service.go`, `actions/publish.go`, `shared/eventbus/kafka.go`.
- Решения оркестратора 1–7 по вопросам исполнителя учтены, эти пункты замечаниями не считаются. Исключение — объём решения 1 (Mi-5).
- Карточку и `dev-log.md` в ветке задачи ведёт исполнитель (правило раздела T-302). Это не замечание.
- Миграции: новых файлов нет, `migrations/gateway/0001_init.sql` и `migrations/links/0001_init.sql` не менялись. Таблицы `sessions`, `turns`, `pending_characters` созданы в T-302. ADR-019 соблюдён.

### Вердикт
**Принять.** Critical 0 · Major 0 · Minor 6 · Nit 6.

DoD закрыт тестами, прогоны зелёные, мутанты исполнителя сверены с кодом. Из моих семи мутантов шесть зелёные. За каждым стоит либо узкая гонка, либо непокрытая граница. Основные пути работают верно, поэтому Major нет.

Рекомендации:
- Mi-1, Mi-3 и Mi-4 — короткие правки с тестом, лучше сделать до finish.
- Mi-2 и Mi-6 — риски доступности и парности, нужно решение system-architect до T-307.
- Mi-5 — вопрос оркестратору об объёме решения 1.

### Проверено ревьюером (go1.26 windows/amd64, cgo выключен — без `-race`)
- `go build ./... && go vet ./...` — 0.
- `go test -short -count=1 ./internal/gateway/...` — 13 пакетов `ok`.
- `golangci-lint run ./internal/gateway/...` — 0 issues.
- `go run ./cmd/mvctl env check` — 78 переменных, `.env.example` сверен.
- `go run ./cmd/mvctl privacy scan internal/gateway/testdata/` — внешних ID нет (1 файл).
- Мутанты. На каждый — своя копия дерева `scratchpad/t306r1-<имя>`: tar без `.git`, `services`, `Docs`, `.claude`, `.qwen`, `.env`, `.worktrees`. Без `-overlay`, контрольный первым. Замена — единственное вхождение, скрипт проверяет число вхождений. Прогон: `go test -short -count=1 -timeout 300s ./internal/gateway/...`. Каждая копия удалена по точному пути сразу после прогона.

| # | Мутант | Результат |
|---|---|---|
| K | контрольный, без изменений | зелёный |
| M1 | `Tracker.Sweep` не ищет просроченные ходы в статусе `narrated` (`turns/tracker.go:336`) | **зелёный** → N-1 |
| M2 | `forgetEnded` стирает резерв номеров и у активных сессий (`turns/tracker.go:382`) | **зелёный** → Mi-1 |
| M3 | `existing` переиздаёт предложение после дедлайна вместо нового `player_id` (`characters/service.go:230`) | **зелёный** → N-6 |
| M4 | `Touch` ровно через `MV_GATEWAY_SESSION_IDLE` продолжает сессию, `<` → `<=` (`session/manager.go:131`) | **зелёный** → N-2 |
| M5 | результат фильтра имени не обрезается (`characters/service.go:303`) | **зелёный** → Mi-4 |
| M6 | sweeper снимает со связки и персонажа, чей факт пришёл (`characters/service.go:372`) | **зелёный** → Mi-3 |
| M7 | удаление связки не двигает счётчик поколений (`links/forget.go:142`) | **красный**: `TestASweepKeepsTheMarkOfAForgetThatDeletedAfterItsCheckpoint` |

### Разбор по пунктам поручения

**1. Корректность и DoD.**
- **Ожидание факта.** `propose` ставит ожидание до публикации, а после ожидания решает проекция: персонаж в ней — `201`, иначе `202 {player_id, status: creating}`. Ответ сохраняется под ключом на контексте без отмены. Проверяют `TestALateFactAnswersCreating` и мутант C9 исполнителя.
- **Снятие со связки.**
  - `Sweep` берёт строки с `deadline_at <= now`, `OnRejected` переносит дедлайн на «сейчас», синхронный отказ вызывает `clear` сразу.
  - `DetachPlayer` срабатывает, только если связка ещё указывает на этого игрока. Связку, уже переназначенную новому персонажу, он не трогает (`TestDetachPlayerUnbindsOnlyTheCharacterItNames`).
  - Ветка «факт пришёл, строка просрочена» не покрыта и оставляет гонку — Mi-3.
- **Идемпотентность `(link_id, action_key)`.** Блокировка на `link_id`, ключ читается под ней, связка перечитывается. Сохраняются только `200/201/202`. В `gateway.db` нет ни внешнего ID, ни `link_id`: это проверяет тест дампа с контролем.
- **Тот же `proposal_id` при повторе после `503`** — `TestARepeatAfterABusFailureProposesTheSameCharacter`, мутант C6.
- **Парность `started/ended`.** Строка пишется до публикации, неопубликованное событие откатывает строку (S4). Неоднозначный отказ брокера пару рвёт — Mi-6.
- **`seq`.** Резервируется в `Begin` (T5, `TestTheNumberOfATurnIsReservedAtItsBeginning`), после рестарта счёт продолжается от `MAX(seq)`. При гонке с уборкой резерв теряется — Mi-1.
- **Исходы хода.**
  - `ok` и `degraded` — по ack последнего адресата.
  - `timeout` — по дедлайну, один раз, `turns_failed + 1`.
  - `rejected` — сразу, только с `received_at`/`acked_at`.
  - Id завершения выводится из действия (`WithCauseID`), поэтому повтор после сбоя шины гасится.
- **Тесты T-305.** Из тестов `actions` изменён только `helpers_test.go`: двойник `memoryTurns` заменил удалённый по поручению `actions.MemoryTurns`. Тестовые функции не менялись, пакеты `actions` и `gateway` зелёные.

**2. Транзакции и конкурентность.**
- **Публикация внутри транзакции `gateway.db`** — Mi-2.
  - У `gateway.db` одно соединение (`store/open.go:116`). `Tracker.Sweep` держит его, пока `Kafka.Publish` ждёт подтверждения `RequireAll`.
  - Контекст sweeper'а без срока. У писателя kafka-go заданы только `RequiredAcks`, `BatchSize` и `BatchTimeout`, остальные таймауты — по умолчанию.
  - На исправном одноузловом Redpanda это миллисекунды. При недоступном брокере соединение занято до исчерпания попыток писателя — на каждом просроченном ходе подряд.
  - T-307 повторит тот же рисунок в транзакции потребителя и ack (`OnDelivered`).
- **Гонки sweeper'ов с обработчиками.**
  - `session.Sweep` → `endIdle` перепроверяет простой под `m.mu` — корректно.
  - `turns.Sweep` перечитывает ход в транзакции и проверяет статус. Шаги `On*` и `Sweep` идут по очереди через единственное соединение — корректно.
  - `forgetEnded` читает список активных сессий без `t.mu` — Mi-1.
  - `characters.Sweep` проверяет проекцию до `DetachPlayer` без блокировки связки — Mi-3.
- **Счётчик поколений окна сжатия** верен. Разобраны все порядки: `DELETE` до и после чтения `began`, до и после checkpoint. Отметка снимается, только если за время сжатия не было удалений. Ошибиться можно лишь в безопасную сторону: `DELETE` между `began` и checkpoint оставляет отметку до следующего сжатия, а sweeper повторяет его раз в минуту. M7 красный.
- **Id сессии из секунд старта** — N-3. В T-306 конфликт недостижим при `IDLE ≥ 1 с`, достижимым он станет с `End(leave|forget)` (T-352, T-355).

**3. Приватность.**
- **События C-10.**
  - `scope` нет ни в конверте, ни в payload. Копии `world` в payload нет, имён нет. `world` в конверте — ключ сообщения.
  - Обязательные идентификаторы (`session.id`, `participants[].entity.id`, `entity.entity.id`) пишутся по решению оркестратора 1.
  - Сверх обязательных `turn.completed` пишет необязательные поля: измерения, ссылки на события и `turn.target.entity.id` — Mi-5.
- **Логи.** `characters` пишет `request_id`, `error`, `handled`, а `clear` — ещё `player_id` и `proposal_id`. Имени и внешних ID нет. Ошибки SQL и шины значений строк не содержат.
- **Фикстура.** `mvctl privacy scan` чист. Вручную: 33 строки, `actor_kind=ci`, нет ни имени, ни текста игрока.
- **`gateway.db`.** `pending_characters.name` и `turns.player_name` есть в схеме T-302, а ADR-009 п. 1 разрешает имя персонажа.

**4. `api.ValidCharacterName`.**
- Табличный тест покрывает: 2 и 32 символа, 1 и 33, кириллицу, CJK, цифры, дефис, таб, перевод строки, двойной пробел, NFD против NFC, невалидный UTF-8.
- Функция объявлена общим правилом шлюза и бота, но пропускает пробелы по краям (`" Вася "`) — Mi-4. Обрезка результата фильтра тестом не закреплена (M5 зелёный).
- Имена только из дефисов или цифр (`"--"`, `"12"`) проходят. FR-060 их не запрещает — вопрос architect#3 вместе с NFC.

**5. OpenAPI.**
- `listWorlds` (`GET /v1/worlds`), `createCharacter` (`POST /v1/characters`) и `getPlayer` (`GET /v1/players/{player_id}`) смонтированы и сняты из `notYetMounted`.
- Все коды обработчиков объявлены (`TestOpenAPIOperationsOfCharactersDeclareTheirCodes`):
  - `400` — `name_required`, `name_invalid`, `invalid_request`;
  - `403 consent_required`;
  - `404` — `world_not_found`, `player_not_found`;
  - `422 filter_error`, `500 internal`, `503 bus_unavailable`.
- Описание `createCharacter` совпадает с порядком проверок в коде и с КД §5.2. `x-nolog` и `x-idempotency` на месте.

**6. Миграции** — см. «Границы ревью»: не менялись, новых нет.

**7. Переменные.**
- Четыре новые `MV_GATEWAY_*` объявлены с `IsDuration` и есть в `.env.example` (CRLF сохранён).
- Значение `≤ 0` — ошибка старта с именем переменной (`TestStartRefusesDurationsOfSessionsAndCharactersItCannotUse`).
- `shared/env/vars.go` принадлежит EPIC-001, нужна отметка tech-lead#1, как в T-305.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Mi-1 | Minor | `internal/gateway/turns/tracker.go:370-387`, `:355-367` | `forgetEnded` читает `Sessions.Active` без `t.mu`, затем под `t.mu` удаляет резервы сессий, которых нет в прочитанном списке. Сессия, открытая в `Begin` между чтением и захватом блокировки, теряет резерв. Сценарий: `Begin` новой сессии резервирует `seq=1`, пакет не публикуется (`503`). Уборка стирает резерв. Другое действие scope читает `MAX(seq)=0`, строки хода ещё нет, и получает `seq=1`. Повтор ключа публикует пакет со старым `seq=1`. `Accepted` падает на `ux_turns_session_seq`, ошибка уходит только в журнал, `turn.completed` для этого хода не будет. Нарушается гарантия строки DoD приёмки T-305. Окно — время одного `SELECT`, поэтому Minor. Уборка резервов тестами не закреплена совсем: M2 зелёный. | Держать `t.mu` на время `Sessions.Active`. Порядок блокировок «`t.mu` → БД» уже есть в `reserve`, а `Active` не берёт `m.mu`. Другой вариант — удалять только id, которые БД подтвердила как `ended`. Тест: сбой пакета → `Tracker.Sweep` → другое действие scope → повтор ключа → `seq` 1 и 2, обе строки в `turns`. M2 должен покраснеть. | открыто |
| Mi-2 | Minor (риск) | `turns/tracker.go:297-303, 317-331`; `context.go:466`; `characters/service.go:323` | `Tracker.Sweep` публикует `turn.completed` внутри транзакции `gateway.db` на контексте sweeper'а без срока. Соединение одно, поэтому ожидание подтверждения брокера останавливает все обращения к файлу: потребитель, `Keys.Lookup` действий (до `500` по бюджету решения), `GET /v1/players`, `resolve`. `characters.Status` читает на `context.Background()` и ждёт без срока. При исправном брокере это миллисекунды. При недоступном — до исчерпания попыток писателя kafka-go на каждом просроченном ходе подряд. А ходы массово истекают как раз тогда, когда шина лежит. Откат и повтор с тем же id (`WithCauseID`) корректны: целостность не страдает, страдает доступность. T-307 повторит рисунок в `OnDelivered`. | Сейчас: срок на публикацию внутри транзакции (`context.WithTimeout` порядка `api.RequestTimeout`) и срок чтения в `Status`. До T-307 — решение system-architect. Варианты: коммитить статус с отметкой «не опубликовано» и публиковать после коммита с повтором по отметке (id уже детерминирован) или записать принятый риск в КД §7.6. | открыто |
| Mi-3 | Minor | `characters/service.go:371-381, 392-404` | `Sweep` проверяет `Model.Character` и только потом, без блокировки связки, вызывает `clear` → `DetachPlayer`. Если `entity.created` применится между проверкой и `DetachPlayer` (факт ровно на границе дедлайна), живой персонаж снимется со связки. Следующий `/start` создаст второго персонажа — нарушение A-7. Строку `OnCreated` уже удалил, повторное удаление в `clear` ошибки не даёт. Ветка «факт пришёл, строка просрочена → удалить только строку» не покрыта: M6 зелёный. | После `DetachPlayer` перепроверить проекцию и, если персонаж появился, вернуть связку (`AttachPlayer`). Либо брать блокировку связки из `Create`: нужен `ByPlayer` → `link_id`. Тест ветки «факт есть, строка просрочена»: строка удалена, связка на месте. M6 должен покраснеть. | открыто |
| Mi-4 | Minor | `api/names.go:36-39`; `api/names_test.go`; `characters/service.go:303` | `ValidCharacterName` задокументирован как единое правило шлюза и бота: T-311 предлагается звать его напрямую. Но он корректен только для уже обрезанной строки: `" Вася "` и `"Вася "` проходят, потому что пробел входит в `NamePattern`. Бот, вызвавший функцию на сыром вводе, пропустит имя, которое шлюз сохранит иначе. В шлюзе обе обрезки на месте, но обрезка результата фильтра тестом не закреплена: M5 зелёный. | В функции отвергать `name != strings.TrimSpace(name)`. В таблицу добавить `" Вася"`, `"Вася "`, `"--"`, `"12"` с решёнными значениями. В `TestTheNameGoesThroughTheInputFilter` добавить фильтр, возвращающий `" Петя "`: в предложении должно быть `"Петя"`. | открыто |
| Mi-5 | Minor (вопрос) | `turns/store.go:147-213` (`:155` — `turn.target`) | Решение оркестратора 1: «в событиях C-10 ничего сверх обязательных полей схемы». `Payload` пишет и необязательные поля: идентификатор сущности `turn.target.entity.id` (регион, NPC); ссылки на события (`delivery.result_event_id`, `delivery.narrative_event_id`, `absence.surfaced_event_ids`); измерения, ради которых событие и существует (`timings.mechanics_at`, `narrative_at`, `*_ms`, `narrative.agent_*`, `fallback_reason`, `turn.phase1_mode`, `lod`). Без измерений метрики `metrics.md` §2.1 не посчитать, так что буквальное прочтение вряд ли имелось в виду. Но `target` в I2 может указать на игрока — цель группового действия. | Уточнить у оркестратора, относится решение 1 к идентификаторам или ко всем необязательным полям. Минимум сейчас — писать `turn.target` только для `npc`/`region`, с тестом. Если решение буквальное — убрать необязательные поля из `Payload` и перегенерировать фикстуру. | открыто |
| Mi-6 | Minor | `session/manager.go:286-295, 305-316`; `session/analytics.go:55-57` | Откат строки при ошибке `Publish` верен для явного отказа брокера, но не для неоднозначного: подтверждение потеряно, а событие записано. `Started`/`Ended` — корневые события со случайным id. Неоднозначный сбой в `open` оставляет в топике `session.started`, чья строка удалена. Следующее действие откроет сессию с другим `session.id`, и первому `started` пары не будет. Неоднозначный сбой в `end` вернёт строку в `active`, и следующий sweep опубликует второй `session.ended` с новым id — дубль, который по id не гасится. На деградировавшем брокере это рвёт NFR-036. | Id событий сессии выводить из `session.id` и типа, как `WithCauseID` у хода: тогда повтор `ended` гасится. Для `started` два варианта: не удалять строку при ошибке `Publish`, а помечать «старт не подтверждён» и повторять публикацию с тем же id; или записать остаточный риск для EPIC-005 (`mvctl report` сводит пары по `session.id`). | открыто |
| N-1 | Nit | `turns/tracker.go:335-336`; `turns/tracker_test.go:193` | Просроченный ход в статусе `narrated` (подтвердили не все адресаты) закрывается `timeout`. Это верно и шире КД §7.6, но тестом не закреплено: M1 зелёный. В T-306 статус `narrated` в проде недостижим, до T-307. | В `TestATurnPastItsDeadlineTimesOut` добавить ход после `OnNarrative` с одним ack из двух. | открыто |
| N-2 | Nit | `session/manager.go:131`; `session/manager_test.go:178-184` | Граница `Touch` не закреплена: при усилении теста под S5 действие сдвинули на `+5 мин`, и M4 (`<=`) зелёный. У `Sweep` граница проверена (`−1 нс` и ровно `Idle`). Если границы `Touch` и `Sweep` разойдутся, момент конца сессии будет зависеть от тика sweeper'а. | Тест: действие ровно в `last + Idle` открывает новую сессию. | открыто |
| N-3 | Nit | `session/manager.go:331-333` | `session.id = {scope}:{unix-секунды}`. Вторая сессия scope в ту же секунду нарушит первичный ключ, и `Begin` ответит `500`. В T-306 это недостижимо: `IDLE ≥ 1 с`, а `End(leave|forget)` не вызывается. | В бэклог T-352/T-355 и КД §4.2 (предложение исполнителя): суффикс или миллисекунды. | открыто |
| N-4 | Nit | `session/manager.go:125-146, 291, 311` | `m.mu` один на все scope и удерживается на время `Publish`. Старт или конец любой сессии задерживает `Begin` всех игроков на время подтверждения брокера. На масштабе MVP-1 незаметно. | Блокировка на scope, как `playerLock`, — когда появится нагрузка. | открыто |
| N-5 | Nit | `context.go:333-345`; `characters/service.go:132-134` | Не проверяется, что `MV_GATEWAY_CHARACTER_WAIT < MV_GATEWAY_CHARACTER_DEADLINE`. При обратном соотношении sweeper может снять связку, пока `propose` ещё ждёт факт. | Ошибка старта с именами обеих переменных. | открыто |
| N-6 | Nit | `characters/service.go:217-236` | Граница дедлайна в `existing` не закреплена: M3 зелёный. Там же: если связка указывает на игрока, которого нет ни в проекции, ни в `pending_characters`, молча создаётся новый персонаж. Для мёртвого или снятого это верно, но то же случится и при неполной проекции (например, снапшот старше срока хранения журнала). | Тест: строка с истёкшим дедлайном до прохода sweeper'а → новый `player_id`, связка переназначена. Неполную проекцию — в вопросы architect#3 рядом с `projection=missing`. | открыто |

### Вопросы
- **Оркестратору:** объём решения 1 (Mi-5).
- **system-architect, до T-307:**
  - публикация аналитики внутри транзакции единственного соединения `gateway.db` (Mi-2);
  - детерминированные id событий сессии (Mi-6).
- **architect#3:** допустимы ли имена только из дефисов или цифр (Mi-4) — вместе с вопросом NFC; поведение `existing` при неполной проекции (N-6).

### Предложения в бэклог (вне границ T-306)
1. **T-307:**
   - `OnDelivered` вызывать ровно один раз на доставку: повторный ack увеличит `delivered_count` и завершит ход раньше срока;
   - нарратив с `recipients=0` сейчас закончится `timeout`, а не `ok` — решить при подключении.
2. **T-309:** восстановление при старте по КД §7.6 (предложение исполнителя) — поддерживаю.
3. **T-311:** проверять имя через `api.ValidCharacterName` после исправления Mi-4.
4. **T-352/T-355:** N-3.
5. **К Mi-2:** outbox аналитики в `gateway.db` — вместе с outbox пакета действий из бэклога T-305.

### Риски и допущения
- `-race` не запускался (cgo выключен). Гонки Mi-1 и Mi-3 найдены чтением и не воспроизведены: точки внедрения нет, а зонд потребовал бы правки кода.
- Поведение kafka-go при недоступном брокере (Mi-2) оценено по конфигурации писателя в `shared/eventbus/kafka.go`, без интеграционного прогона.
- `shared/env/vars.go` и `.env.example` — файлы EPIC-001, нужна отметка tech-lead#1.
- В рабочей папке задачи ревьюер добавил только этот раздел и строку в карточке. Мутанты выполнялись в копиях в scratchpad, копии удалены.

## T-312 · ревью #1 · 2026-09-13 · code-reviewer#2 (TEAM-3)

### Границы ревью

Ветка `task/T-312-bot-deliver-main`, папка `.worktrees/T-312`, база — кончик эпика `aa83086`. Изменения не закоммичены (`git status`):
- новое: `cmd/telegram-bot/internal/deliver/{loop.go, loop_test.go, fakes_test.go}`; `cmd/telegram-bot/{main.go, serve.go, health.go, main_test.go, mocks_test.go, imports_test.go}`;
- удалено: `cmd/telegram-bot/internal/config/imports_test.go` (перенесён в пакет бинарника, текст теста не изменился, только пакет и комментарий);
- правки: `cmd/telegram-bot/README.md`, `Docs/ops/runbook.md` §6, карточка `tasks/T-312.md`, `dev-log.md`.

Коммитов вне ветки нет. `shared/*`, `internal/gateway/*`, `go.mod`/`go.sum`, `.env.example`, `.golangci.yml`, `docker-compose.bot.yml`, контракты и КД не тронуты. Правки в `artifactsDir` — только карточка и `dev-log.md`, как положено исполнителю.

Основания:
- раздел T-312 в `tasks.md` со строками приёмок T-310 (Mi-1, M-1, M-2, N-3), T-311 (риск 6, Mi-6, `ValidName`, README/runbook) и T-317 (README/runbook после `main.go`);
- карточка `tasks/T-312.md`;
- КД `gateway-and-bot.md` §8.2–§8.4, §10.2, §10.4, §11.1;
- ADR-006 п. 3, ADR-018 п. 5–6 и дополнение п. 3;
- C-08 v1.1–v1.5 (лимиты long-poll, «повтор без ack через 30 с», одна доставка в лизинге на игрока).

Решения оркестратора приняты как данность:
- доставка без маршрута Telegram подтверждается без отправки, с предупреждением в логе (до ответа system-architect);
- замена `flow.ValidName` на `api.ValidCharacterName` уходит в DoD T-315 (T-306 не слита);
- устаревшие имена `MV_BOT_*` в КД §11.3 — вопрос к architect#3.

### Вердикт

**ПРИНЯТЬ** — Critical 0 · Major 0 · Minor 4 · Nit 4.

Семантика ack соответствует C-08 и КД §10.4, потери доставки и бесконечного повтора в достижимых сценариях нет. Коды выхода, разделение клиентов, `/health` и подкоманда `health` соответствуют DoD, ADR-018 и `docker-compose.bot.yml`; утверждения README и runbook сверены с кодом. Замечания Minor — три пробела в тестах объявленных свойств цикла доставки (мутанты M2–M4 выжили) и 30-секундный клиент Telegram в обработчике обновлений (Mi-4). Все локальны; их можно закрыть при приёмке без повторного ревью.

### Прогоны (go1.26.8 windows/amd64, рабочая папка T-312)

| Команда | Результат |
|---|---|
| `go build ./... && go vet ./...` | 0 |
| `go test -short -count=1 ./cmd/telegram-bot/... ./internal/gateway/client/...` | ok ×11 пакетов; `updates` запустился напрямую, без `Access is denied` |
| `golangci-lint run ./...` | 0 issues (в копии дерева `./cmd/telegram-bot/...` — тоже 0) |

`-race` недоступен (cgo выключен). Конкурентность разобрана чтением: `cursor` и `pending` трогает только горутина `Run`; `sender.Telegram` общий у `flow` и цикла, но состояния между вызовами не держит. Бинарник не запускался, Docker, стенд LLM и api.telegram.org не трогались.

### Мутанты ревьюера

Копия дерева `t312r1-tree` в scratch (без `.git`, `Docs`, `services`, `.env`), без `-overlay`, один мутант на прогон, файл восстанавливался из рабочей папки со сверкой `cmp`. Копия и скрипт удалены по точным путям.

| # | Мутант | Итог |
|---|---|---|
| M0 (контрольный, первым) | без изменений: `go test ./cmd/telegram-bot/ ./…/deliver/`, `golangci-lint run ./cmd/telegram-bot/...` | зелёный, 0 issues |
| M1 | `updates.NewTelegram` получает `base` вместо логгера поверх `privacy` (`serve.go:180`) | **выжил** → N-1 |
| M2 | в `Run` снят сброс `failures = 0` после успешного опроса (`loop.go:151`) | **выжил** → Mi-1 |
| M3 | отправка, прерванная остановкой, подтверждается: `return true, true` (`loop.go:209`) | **выжил** → Mi-2 |
| M4 | в `Once` снят `flush` перед опросом (`loop.go:167`): ack с прошлого раза уходит только после следующего long-poll | **выжил** → Mi-3 |
| M5 | `ErrUnauthorized` не останавливает остаток ответа (`loop.go:233`, `stop=false`) | красный: `TestARevokedTokenAcknowledgesNothingItDidNotSend` |
| M6 | `keep` без проверки дублей | **выжил** → N-2 |
| M7 | `deliver/loop.go` импортирует `shared/eventbus` | сборка зелёная, `golangci-lint`: `depguard` правило `cmd-telegram-bot` — 1 issue |

### Разбор по пунктам поручения

**1. Семантика ack** (`loop.go:166-297`).
- Подтверждаются: отправлено, `ErrBlocked`, `ErrChatNotFound`, `ErrRejected`, доставка без маршрута Telegram (решение оркестратора). Не подтверждаются: `ErrUnavailable`, неизвестная ошибка, отмена `ctx` посреди отправки, `ErrUnauthorized`. Совпадает с DoD (Mi-1 ревью T-310) и КД §10.4. Решение — только по типу ошибки `Sender` через `errors.Is`.
- `401`: `stop=true`, остаток ответа не отправляется, отправленное до него подтверждается (`TestARevokedToken…`, M5 красный). Цикл после этого опрашивает снова, но при одной доставке в лизинге на игрока следующий ответ приходит не раньше истечения лизинга или с доставками других игроков. Горячего цикла нет; процесс останавливает источник обновлений тем же `401`.
- Неудачный ack: id остаются в `pending` (без дублей, ≤ 1000), `flush` в начале `Once` повторяет их до long-poll. Код верен, порядок тестом не закреплён — Mi-3.
- Остановка: `defer ackOnStop` под `context.WithoutCancel` + 5 с. Прерванный на середине `flush` оставляет `pending`, и `ackOnStop` его повторяет. Тест `TestAStoppingLoopAcknowledgesWhatItSent` красный на снятом `ackOnStop` (мутант исполнителя). Прерванная отправка не подтверждается — код верен, теста нет (Mi-2).
- Backoff опроса: 1, 2, 4 … 30 с на `clock.Timers`, строка лога на каждую ошибку и строка восстановления; сброс после успеха тестом не закреплён — Mi-1.
- Потери: доставка теряется только при ack без отправки. Такие ветки — «игрок недоступен», `ErrRejected` и «нет маршрута» — предписаны DoD или решением оркестратора. Бесконечного повтора в достижимых сценариях нет: неподтверждённое живёт у шлюза до TTL 24 ч, частота — раз в лизинг. Недостижимый сегодня случай «шлюз навсегда отвергает ack кодом 4xx» — N-4.
- Длинная пачка (например, `429 retry_after` 35 с на первой доставке) подтверждается позже 30 с лизинга. Дубля не будет, только если `Ack` шлюза принимает id с истёкшим, но никем не перехваченным лизингом (§8.4: `WHERE leased_by = :client`), а sweeper не стирает `leased_by`. Это вопрос реализации T-307 — в рисках.

**2. Коды выхода** (`main.go`, `serve.go:189-233`, `updates.ExitCode`).
- 0 — сигнал: `Start` возвращает `nil`, тест `TestEachSenderRepeatsByItsOwnPolicy` (`p.cancel()` → 0).
- 1 — пустой или кривой токен, переменная, `MV_LOG_LEVEL`, занятый порт `/health`, `401 getMe` (`getUpdates` не вызывался), `401 getUpdates` (ровно один вызов) — тесты на каждый случай.
- 2 — неизвестная подкоманда, лишний аргумент или неизвестный флаг `health`.
- 3 — `409 getUpdates`.
- Соответствует ADR-018 п. 6, DoD Mi-1 ревью T-310 и таблице README. Токен, его секретная половина и id в stderr и в логе не найдены (`assertNoSecret`).

**3. Клиенты, таймауты, логгеры.**
- Отказ постороннему — `NewBestEffortTelegram` со своим клиентом 5 с. Цикл — `DefaultPolicy`. `flow` — `WithPolicy(ReplyPolicy)` того же отправителя (клиент 30 с, Mi-4).
- Шлюз: у `flow` свой клиент 6 с и `Backoff{1, 200 мс}`, у цикла — 35 с и `DefaultBackoff`.
- DoD M-2 ревью T-310 и риск 6 ревью T-311 выполнены, тесты `TestARefusalTelegramDoesNotAnswer…`, `TestAGatewayThatDoesNotAnswer…`, `TestTheWiring…`, `TestTheProductionLimits`.
- Один логгер `slog.New(privacy.NewHandler(base, redactor))` подан в `access`, `flow`, `deliver`, `updates` и `ErrorLog` отправителей. Тестом проверены первые три (N-1).
- Лог цикла: ключи `kind`, `reason`, `count`, `error` (через `privacy.Redact`, а `sender` ещё и вырезает chat id), `pause`. `config` в логе — через `LogValue` без токена, соли и id.

**4. `/health` и подкоманда `health`.**
- `GET /health` → `{"status":"ok","details":{"bot_denied_total","bot_not_private_total","bot_too_often_total"}}`. Только счётчики, без id и токена; `ReadHeaderTimeout` 5 с.
- `telegram-bot health [--url]`: умолчание из `MV_TELEGRAM_HEALTH_ADDR`, пустой хост, `0.0.0.0` и `::` → `127.0.0.1` (таблица из шести адресов и умолчание манифеста). Код 0 только при 200 и `status: ok`, проба без прокси, таймаут 3 с — внутри `timeout: 5s` compose.
- Healthcheck `docker-compose.bot.yml` (`/telegram-bot health --url http://127.0.0.1:8089/health`, `entrypoint /telegram-bot`) совпадает с кодом. `build/Dockerfile` собирает `./cmd/...` в `/`. `make health` пробует `:8089` по коду 200 — совпадает.
- `status` всегда `ok` — записано в README, runbook и рисках карточки; предложение исполнителя в бэклог поддерживаю.

**5. Гонка при старте** (`serve.go:211-216`). Цикл доставки стартует до `getMe` внутри `Source.Start`.
- С отозванным токеном цикл успевает взять доставки в лизинг и получить `401`: они не подтверждаются и вернутся через 30 с. Отправить их этим токеном всё равно нельзя, потери нет.
- С `restart: unless-stopped` такой цикл повторяется на каждом рестарте. Цена — сдвиг выдачи на 30 с, счётчик `attempts` и лишние строки `Error` в логе.
- При `409` второй экземпляр с тем же токеном может отправить и подтвердить часть доставок: токен рабочий, `ackOnStop` подтверждает отправленное. Отправка, прерванная остановкой, может прийти игроку дважды.
- Риск низкий — N-3.

**6. depguard и тест импортов.**
- Правило `cmd-telegram-bot` покрывает новые файлы: M7 даёт `depguard` 1 issue. `golangci-lint run ./...` — 0.
- `imports_test.go` перенесён в `package main` без изменения логики: `go list -deps ./cmd/telegram-bot/...`, проверка `sawBot`, запрет `internal/*` кроме `client`/`api`, запрет `modernc.org/sqlite` и `goose`. Старый файл удалён, тест зелёный.

**7. README и runbook.**
- README: статус (бинарник собирается, `make build` → `bin/`), таблица частей и «Как устроен процесс» совпадают с кодом. Сверено: таймауты, `ReplyPolicy` (2 попытки, `429` ≤ 3 с), `DefaultPolicy` (3 попытки через 1 с, `429` до 60 с × 5), повтор ack клиентом на сеть и `503`, long-poll без повтора. То же для таблицы решений по ответам Telegram, «только в памяти», `/health`, подкоманды и кодов выхода.
- Runbook §6: врезка «выполнима только после T-312» и оговорка «до T-312» сняты. Шаги 4–5 явно оставлены за T-390 со ссылкой. Строки лога `deliveries not polled` и `answer not delivered` существуют (`loop.go:156`, `flow/flow.go:206`).
- Пометок «после T-312» в `.md` вне `Docs/dev-team` и в коде не осталось. Устаревший комментарий «The binary lands in EPIC-004» в `docker-compose.bot.yml` — файл devops, бэклог п. 4.

### Замечания

| # | Серьёзность | Файл:строка | Замечание | Предложение | Статус |
|---|---|---|---|---|---|
| Mi-1 | Minor | `cmd/telegram-bot/internal/deliver/loop.go:151`; `loop_test.go:379-413` | **Сброс паузы после восстановления опроса не закреплён тестом.** Мутант M2 (без `failures = 0`) зелёный. С ним после одного сбоя шлюза любая следующая одиночная ошибка ждёт уже 30 с, а строка «deliveries polled again» пишется на каждый успешный опрос. Тест подаёт восемь ошибок подряд, затем успех, и новых ошибок после успеха не даёт. | Продолжить сценарий: ошибка, ошибка, успех, ошибка → паузы `[1s 2s 1s]`; одна строка «polled again». | открыто |
| Mi-2 | Minor | `loop.go:208-210`; `loop_test.go:335-375` | **«Отправка, прерванная остановкой, не подтверждается» не закреплено тестом** — ветка «без потери» пункта 1 поручения. Мутант M3 (`return true, true`) зелёный. `cancellingSender` отменяет `ctx` после успешной отправки, и следующая доставка до `Send` не доходит (проверка `ctx.Err()` в `Once`). С мутантом доставка, чей `sendMessage` оборван `SIGTERM`, подтверждается и может не дойти до игрока. | Отправитель, который ждёт `ctx.Done()` и возвращает `ctx.Err()` на второй доставке; после `Run`: `unacked() == [b1]`, ни один вызов `Ack` не содержит `b1`. | открыто |
| Mi-3 | Minor | `loop.go:167`; `loop_test.go:316-333` | **«Неудачный ack повторяется перед следующим опросом» — порядок тестом не проверяется.** Мутант M4 (без `flush` в начале `Once`) зелёный: `ackCalls` одинаковы, потому что конечный `flush` второго `Once` шлёт те же id. С мутантом ack ждёт следующий long-poll (до 25 с) и всю его пачку. Лизинг 30 с истекает, и после отзыва лизинга шлюзом сообщение приходит игроку повторно. Имя теста обещает именно порядок. | Писать опросы и ack в один журнал фейка (`journal` уже есть) и сравнивать `ack a1` → `poll` → … Либо во втором `Once` отдать пустой ответ с ошибкой опроса и проверить, что ack `[a1]` всё равно ушёл. | открыто |
| Mi-4 | Minor | `cmd/telegram-bot/serve.go:131`, `:166`; `serve.go:23-34` (комментарий `limits`) | **Ответы `flow` идут через HTTP-клиент Telegram с таймаутом 30 с.** Зависший `sendMessage` держит единственный обработчик обновлений до 30 с × 2 попытки `ReplyPolicy` + 1 с, около 61 с. Всё это время стоят команды всех игроков. Комментарий `limits` обосновывает отдельные короткие клиенты тем же доводом («must hold the commands of the other players for seconds»), но к Telegram его не применяет. DoD прямо требует короткую политику повторов, а не таймаут, и README честно пишет 30 с, поэтому это не Major. | Отдельный `sender.NewTelegram` для `flow` со своим клиентом (например, `FlowTelegramTimeout` 10 с в `limits`) и `ReplyPolicy`; unit по образцу `TestARefusalTelegramDoesNotAnswer…` для ответа игроку; строка README. Либо решение tech-lead#3 «принять» с записью в риски runbook. | открыто |
| N-1 | Nit | `serve.go:179-181`; `main_test.go:230-248` | Логгер источника обновлений не проверяется тестом сборки. M1 (`Log: base`) зелёный. Сегодня это почти эквивалентно: `updates` пишет постоянные строки и текст через `privacy.ErrorLog`. Но пункт «логгеры всех пакетов через `privacy.NewHandler`» закреплён только для трёх. | Хранить `updates.TelegramOptions` в `bot` (как `flowOpts`) и добавить `"updates"` в таблицу проверки `*privacy.Handler`. | открыто |
| N-2 | Nit | `loop.go:255-265` | Отсутствие дублей в `pending` не закреплено (M6 зелёный). Дубль появляется, если id подтверждён, ack не дошёл, а доставка после лизинга выдана и отправлена снова. Вреда мало: шлюз вернёт лишний id в `unknown`. | Unit: ack падает, лизинг истекает, та же доставка отправлена снова → следующий ack `[a1]`, а не `[a1 a1]`. | открыто |
| N-3 | Nit | `serve.go:210-216` | Цикл доставки стартует до `getMe` (пункт 5 разбора). Потери нет. С отозванным токеном и `restart: unless-stopped` каждый рестарт берёт доставки в лизинг, пишет `Error` на каждую и сдвигает выдачу на 30 с. | Запускать цикл после успешного `getMe`: колбэк `OnReady` в `updates.TelegramOptions` или `bot.New` отдельно от `Start`. Можно бэклогом. | открыто |
| N-4 | Nit | `loop.go:271-286` | Ack, который шлюз отвергает навсегда (`4xx`, не `503`), повторяется перед каждым опросом без конца. Доставки при этом каждые 30 с отправляются игрокам повторно до TTL 24 ч. Сегодня недостижимо: `403 client_*` валит и опрос, `413` при ≤ 1000 id не наступает, `400` на id от самого шлюза не ожидается. | На `*client.APIError` со статусом `4xx` — строка `Error` и сброс `pending`, либо оставить с комментарием. | открыто |

### Вопросы к tech-lead#3 (через оркестратора)

1. **Mi-4:** отдельный короткий клиент Telegram для ответов `flow` в T-312 или принять 30 с × 2 с записью в риски.
2. **T-307 (риск ниже):** `Ack` шлюза должен принимать id с истёкшим, но никем не перехваченным лизингом (sweeper не стирает `leased_by`), иначе пачка дольше 30 с даёт дубли. Нужна строка DoD T-307 или уточнение КД §8.4.

### Предложения в бэклог

1. Поддерживаю п. 1 исполнителя: `/health` бота — `degraded` при N неудачных опросах подряд или `ErrUnauthorized` отправки.
2. Поддерживаю п. 2–3 исполнителя (`.gitignore` для бинарника в корне, строка `cmd/telegram-bot` в `CLAUDE.md`).
3. Цикл доставки после `getMe` (N-3) — вместе с п. 1, если не закрывается при приёмке.
4. devops: комментарий `docker-compose.bot.yml` «The binary lands in EPIC-004 … starts working the moment cmd/telegram-bot exists» и «moves out of compose entirely in EPIC-004» — привести к факту после слияния T-312.
5. Защитная пауза цикла, если шлюз отдал пустой ответ заметно раньше `wait` (например, в режиме `replay` или при остановке шлюза): сейчас такой ответ означал бы опрос без паузы. Проверить вместе с реализацией long-poll в T-307/T-309.

### Риски и допущения

- Outbox шлюза (T-307) ещё не реализован. Поведение ack после истёкшего лизинга, отзыв лизинга sweeper'ом и досрочный пустой ответ long-poll проверены только по КД §8.2–§8.4 и фейку `outbox` в тестах.
- Гонок чтением не найдено; `-race` без cgo недоступен.
- Живой Telegram, Docker-образ и healthcheck compose не запускались. Соответствие сверено по `docker-compose.bot.yml`, `build/Dockerfile` и `Makefile`.
