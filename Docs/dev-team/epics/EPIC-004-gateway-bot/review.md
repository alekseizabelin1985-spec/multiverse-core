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
