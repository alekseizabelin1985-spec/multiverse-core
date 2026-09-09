# Журнал разработки EPIC-001 «Фундамент»

Формат записи: экземпляр разработчика, задача, что сделано (команды), отклонения от дизайна,
результаты проверок DoD, открытые вопросы. Язык — русский; код и конфиги — английские.

---

## developer#1 · T-001 · F-1 «Гигиена индекса, gitleaks и pre-commit» · 2026-09-09

Ветка `epic/EPIC-001-foundation` (от `integration/mvp-1`, база `744fb10`; на момент работы
HEAD = `be33e93`). Основание: `architecture/infrastructure.md` v0.3 §4.5 п. 1–13, §3.4, §4.2;
`epics/EPIC-001-foundation/design.md` §2 (US-013), §9; `plan/ownership.md` §1.
Коммит не выполнялся (`git.commits: ask`) — изменения подготовлены в индексе для оркестратора.

### 1. Установка инструментов (их не было на машине)

| Инструмент | Версия | Как установлен | Где лежит |
|---|---|---|---|
| `pre-commit` | 4.6.2 (пин §2.4) | `py -3 -m pip install --user pre-commit` | `%APPDATA%\Roaming\Python\Python314\Scripts\pre-commit.exe` |
| `gitleaks` | v8.30.1 (пин §2.4) | `go install github.com/zricethezav/gitleaks/v8@v8.30.1` | `%USERPROFILE%\go\bin\gitleaks.exe` (каталог уже в `PATH`) |

Замечание по модулю gitleaks: команда из §2.4/§3.4 подразумевает путь `github.com/gitleaks/gitleaks/v8`,
но модуль по-прежнему объявлен как `github.com/zricethezav/gitleaks/v8` — `go install` по «новому» пути
падает с `version constraints conflict`. Рабочая команда — с `zricethezav`. Альтернатива без Go:
`winget install Gitleaks.Gitleaks`. Записать в runbook (F-9) и учесть в CI (F-7).

**Требуется действие пользователя**: каталог `%APPDATA%\Roaming\Python\Python314\Scripts`
не входит в `PATH`, поэтому `pre-commit` из произвольной консоли не запускается. Сам git-хук
работает: `pre-commit install` записывает в `.git/hooks/pre-commit` абсолютный путь к Python.
Чтобы команда `pre-commit run …` была доступна вручную, добавить каталог в пользовательский `PATH`
(агент системные и пользовательские переменные окружения не меняет).

Установлены окружения хуков: `pre-commit install-hooks` (gitleaks и golangci-lint собираются
`language: golang` локальным Go 1.25.3, хуки `pre-commit-hooks` — `language: python`).

### 2. Что сделано (порядок §4.5)

1. `.gitleaks.toml` — создан первым, дословно по §4.5 п. 1 (`[extend] useDefault = true`;
   allowlist путей `^\.env\.example$`, `^\.mcp\.env\.example$`; allowlist очевидных плейсхолдеров).
2. `.pre-commit-config.yaml` — по §3.4 без изменений (gitleaks `rev: v8.30.1`,
   golangci-lint `rev: v2.13.2`, `pre-commit-hooks rev: v5.0.0`). Затем `pre-commit install`
   → `.git/hooks/pre-commit`.
3. `shared/oracle/README.md` — значение ключа заменено на плейсхолдеры:
   строки 14 и 29 → `sk-xxxxxxxxxxxxxxxxxxxxxxxx`, строка 68 → `export ORACLE_API_KEY="<your-key>"`.
   Значение ключа нигде не выводилось и не логировалось. Файл уходит в архив в T-002 — плейсхолдер
   поставлен до `git mv` (U-5).
4. `git rm --cached .mcp.env .claude/settings.local.json` — файлы остались на диске.
5. `git rm --cached examples.exe semantic-memory.exe services/narrative-orchestrator/cmd.exe`
   `mcp_kafka.log mcp_audit.log`; с диска удалены три `.exe`.
   **Отклонение**: `mcp_kafka.log` и `mcp_audit.log` с диска удалить не удалось —
   `rm: Device or resource busy` (файлы держит запущенный MCP-сервер `mcp-kafka`). Из индекса они
   выведены, правилом `*.log` в `.gitignore` закрыты; удалить с диска — после остановки MCP-сервера.
6. Пустые каталоги: проверено `ls -ld -- -p Multiverse` (оба пусты), удалены `rm -rf -- -p Multiverse`.
7. Перенос кода (`fake_deps`, `test_minio.go`, дубли `Dockerfile`) не выполнялся — это T-002 (F-3).
8. `.gitignore` — заменён целевым по §4.5 п. 8. По решению U-11 `git rm --cached` по
   `.idea/`, `.vscode/`, `.kilo*`, `.roo*`, `.qwen/`, `memory/`, `plans/`, `reports/`,
   `shared/eventbus/docs/event-model.pdf` **не выполнялся** — решение за tech-writer в T-019 (F-9).
9. `.mcp.env.example` — создан (`GITHUB_TOKEN=`, `DB_MCP_TOKEN=`, пустые, с комментарием где взять).
10. `.gitattributes` — заменён целевым: `eol=lf` для `sh/yml/yaml/toml/Makefile`,
    `*.exe` и `*.pdf` — `binary`, `testdata/recordings/*.jsonl merge=binary` (без `-diff`, D-14).
11. `.env.example` — переписан по §4.2 полностью. Сверка имён переменных с §4.2 автоматическая:
    расхождений нет ни в одну сторону; 55 переменных `MV_*`; `OLLAMA_*` и `MV_OLLAMA_URL`
    закомментированы; `MV_OPENAI_API_KEY` и `MV_DEEPSEEK_API_KEY` отсутствуют (v0.3 п. 1).
12. `.gitleaksignore` — **добавлен сверх списка файлов задачи** (механизм разрешён §4.5 п. 1:
    «fingerprint в `.gitleaksignore`, не расширение allowlist»). Гасит одно ложное срабатывание
    `generic-api-key` на `Docs/dev-team/architecture/components/swarm-llm-laws.md:504` — правило
    цепляет имя фича-флага в тексте таблицы провайдеров, секрета там нет; файл — владение architect#2,
    поэтому inline `# gitleaks:allow` в чужом файле не ставился.
13. `git filter-repo` — не выполнялся (только по явной команде владельца, §4.5 п. 13).

### 3. Проверка хука на Windows

Тестовый файл `hook-test.env` с фиктивным токеном, `git add`, `git commit`:

- **Вариант из формулировки задачи** — `MV_TELEGRAM_BOT_TOKEN=123456789:AA` + 33 символа `x`:
  хук **проходит** (`Detect hardcoded secrets … Passed`). Причина: это ровно тот шаблон, который
  внесён в allowlist «obvious placeholders» (`123456789:AA[x]{33}`, §4.5 п. 1). То есть буквальный
  пример из §3.4 и DoD негативным тестом хука быть не может — см. открытый вопрос ОВ-1.
- **Вариант с токеном формата Telegram, не совпадающим с плейсхолдером**
  (`123456789:A` + 34 символа `[A-Za-z0-9]`, значение вымышленное): хук **отклоняет**,
  `RuleID: telegram-bot-api-token`, значение в выводе редактировано (`REDACTED`).
  Полный `git commit` завершился с кодом 1, `HEAD` не изменился (`be33e93`).
  Файл затем убран из индекса (`git reset HEAD -- hook-test.env`) и удалён с диска.

### 4. Результаты DoD задачи

| Проверка | Результат |
|---|---|
| `gitleaks git --redact --log-opts="integration/mvp-1..HEAD" .` | **0 находок** (`no leaks found`), 1 коммит просканирован |
| `gitleaks dir --redact .` | **22 находки, все вне git**: `.env` (2 — реальный локальный файл владельца, в `.gitignore`, по назначению содержит секреты) и `.claude/worktrees/**` (20 — копии удалённых worktree, оставшиеся на диске после `git worktree prune` в F-0; среди них 13 копий старого `shared/oracle/README.md` с невыведенным ключом и 6 копий `.env`). В отслеживаемых git файлах — 0: контрольный прогон по содержимому `git ls-files` даёт только гасимое `.gitleaksignore` ложное срабатывание. См. ОВ-2 |
| `git ls-files` + фильтр по `.exe`/`.log`/`.mcp.env`/`settings.local.json` | пусто |
| `grep -n "sk-4659" shared/oracle/README.md` | пусто |
| `pre-commit run --all-files` | **не зелёный** — см. раздел 5 |
| `pre-commit run --files <файлы задачи>` | зелёный: gitleaks, large-files, end-of-file-fixer, check-yaml, check-merge-conflict, mixed-line-ending — Passed; golangci-lint и golangci-lint-fmt — Skipped (нет `.go` среди файлов задачи) |
| `git status` | работает, 44 записи (свои — в индексе, остальное — незакоммиченные правки других ролей) |
| `.env.example` содержит все переменные §4.2 с префиксом `MV_` | да, автосверка имён без расхождений |
| Общий DoD §1 п. 1–2 (`make lint`, `go build/vet/test`) | **n/a**: Go-код в задаче не менялся (изменены только корневые конфиги и один `README.md`) |
| Общий DoD §1 п. 3 (тесты) | **n/a**: предмет задачи — конфигурация инструментов; проверяется прогонами gitleaks и pre-commit выше |
| Общий DoD §1 п. 7 (карта владения) | соблюдена: правки только в корневых конфигах EPIC-001 и `shared/oracle/README.md` (файл уходит в архив T-002) |

### 5. `pre-commit run --all-files` — причина «красного» и что с этим делать

Прогон по всему индексу (419 файлов) даёт:

```text
Detect hardcoded secrets ......... Passed
golangci-lint-fmt ................ Failed   (files were modified by this hook)
golangci-lint .................... Passed
check for added large files ...... Passed
fix end of files ................. Failed   (81 файл дописан переводом строки)
check yaml ....................... Passed
check for merge conflicts ........ Passed
mixed line ending ................ Failed   (8 файлов: Makefile, docker-compose.yml, configs/gm_*.yaml)
```

Все три «красных» хука — **автофиксеры**, и падают они исключительно на as-is коде и документах,
которые в T-001 трогать нельзя (владение других ролей и кандидаты в архив T-002 и переформат T-003):
`services/**`, `shared/{agent,config,minio,intent,rules,schema,spatial,tinyml,eventbus,entity,jsonpath}/**`,
`fake_deps/**`, `docs/**`, `.idea/**`, `.kilocode/**`, `.roo/**`, `configs/gm_*.yaml`, `Makefile`,
`docker-compose.yml`. Чужие файлы не чинились; все изменения, внесённые хуками в ходе прогона,
откачены побайтово (рабочее дерево сверено с копией, снятой до прогона, — расхождений нет).

Ожидаемое закрытие: `mixed-line-ending` и часть `end-of-file-fixer` уходят после T-002 (F-3, архив)
и T-004/T-008 (`Makefile`, `docker-compose.yml` пишутся заново), `golangci-lint-fmt` — после T-003
(F-2: единый модуль, `.golangci.yml`, `gofmt`/`goimports` по всему дереву). До этого зелёным
является прогон по изменяемым файлам, а хук на коммите (`git commit` обрабатывает только staged-файлы)
работает штатно — что и проверено в разделе 3.

### 6. Отклонения от дизайна

1. **`.gitleaksignore` добавлен** сверх списка файлов задачи (механизм предусмотрен §4.5 п. 1).
2. **`.env.example`: машинно-зависимые пути LLM оставлены пустыми.** §4.2 показывает их со значениями
   владельца (`MV_LLM_BIN=D:\Models\…`, `MV_LLM_MODEL_FILE`, `MV_LLM_MODELS_DIR`, `MV_LLM_SLOT_SAVE_PATH`).
   В файле-примере, который лежит в Git и общий для всех окружений, это пути одной машины, поэтому
   значения оставлены пустыми (формулировка задачи в `tasks.md`: «полный, значения пустые»).
   Остальные значения §4.2 сохранены дословно. Если `mvctl env check` (T-010) должен видеть их
   непустыми — вернуть; решение за devops-engineer.
3. **`mcp_kafka.log` и `mcp_audit.log` не удалены с диска** — файлы заняты процессом (см. п. 2.5).
4. **`go install` gitleaks — по пути `zricethezav`,** а не `gitleaks/gitleaks` (см. раздел 1).
5. **`gitleaks dir --redact .` не даёт 0** по причинам вне репозитория (см. ОВ-2).

### 7. Открытые вопросы

- **ОВ-1 (security-engineer и devops-engineer).** Критерий F-1 «commit файла с
  `MV_TELEGRAM_BOT_TOKEN=123456789:AA…` отклонён» противоречит allowlist-правилу
  `123456789:AA[x]{33}` из того же §4.5 п. 1: ровно этот шаблон хук пропускает. Нужно либо
  переформулировать критерий («токен формата Telegram, отличный от плейсхолдера»), либо убрать
  шаблон из allowlist. Сейчас проверка выполнена во втором варианте (раздел 3).
- **ОВ-2 (пользователь и security-engineer).** `gitleaks dir --redact .` сканирует и файлы вне git
  (`.gitignore` он не учитывает — у команды `dir` такого флага нет), поэтому на машине владельца
  всегда будет не меньше двух находок в `.env`. Дополнительно на диске остались **13 каталогов
  `.claude/worktrees/*`** — остатки удалённых worktree после F-0; они содержат копии старого
  `shared/oracle/README.md` с невыведенным ключом и копии `.env`. Предложение: (а) удалить каталоги
  `.claude/worktrees/*` (агент не удаляет по U-1 — нужно решение пользователя); (б) в T-008 сделать
  цель `make secrets-scan` как `gitleaks git` по диапазону плюс `gitleaks dir` по содержимому
  `git ls-files`, а не по всему рабочему каталогу; (в) критерий F-1 переформулировать под (б).
- **ОВ-3 (architect#2 и security-engineer).** Ложное срабатывание `generic-api-key` на
  `Docs/dev-team/architecture/components/swarm-llm-laws.md:504` погашено fingerprint'ом в
  `.gitleaksignore`; при следующей правке этого файла номер строки сместится и fingerprint перестанет
  действовать. Устойчивее — inline `# gitleaks:allow` в самом файле владельцем.
- **ОВ-4 (пользователь).** Ключ `ORACLE_API_KEY`, лежавший в `shared/oracle/README.md`, считается
  скомпрометированным (он есть в истории git и в копиях worktree). Отзыв ключа и решение по
  `git filter-repo` (§4.5 п. 13) — за пользователем.
- **ОВ-5 (devops-engineer).** `pre-commit` при прогоне прячет незакоммиченные правки
  (патч по образцу `git stash`) и восстанавливает их; на Windows при этом файлы возвращаются с CRLF.
  Содержимое для git не меняется (нормализация `text=auto`), но рабочие копии других ролей
  переписываются. Учесть при одновременной работе нескольких экземпляров в одном дереве.

### 8. Риски и допущения

- `pre-commit` и `gitleaks` установлены только у текущего пользователя Windows; в CI (T-012)
  версии берутся из `build/versions.env` (`GITLEAKS_VERSION`, `GOLANGCI_LINT_VERSION`, T-004).
- Хуки `language: golang` собираются локальным Go 1.25.3; после перехода на Go 1.26 (T-003)
  кэш pre-commit (`%USERPROFILE%\.cache\pre-commit`) может потребовать `pre-commit clean`.
- `.gitattributes` вводит `eol=lf`; при следующем `git checkout` или нормализации существующие
  `Makefile`, `configs/gm_*.yaml`, `docker-compose.yml` изменят окончания строк — ожидаемо,
  эти файлы переписываются в T-004 и T-008.

---

## developer#1 · T-002 · F-3 «Архив заменяемого кода `services/_archive/`» · 2026-09-09

Ветка `epic/EPIC-001-foundation`, HEAD на старте = `04a3a15` (T-001). Основание:
`architecture/infrastructure.md` v0.3 §4.6 и §10 (строка F-3); `architecture/components/foundation.md`
v0.2 §11; `plan/ownership.md` §1 (строка `services/_archive/**`); `epics/EPIC-001-foundation/review.md`
(замечание M-5); U-1 / OQ-A-17, ADR-001 доп. п. 5.
Коммит не выполнялся (`git.commits: ask`) — всё подготовлено в индексе. Код **не удалялся**:
60 путей перенесены `git mv`, все переименования распознаны как `R100` (содержимое байт в байт).

### 1. Шаг 0 — замечание ревью M-5 (до первого `git mv`)

В `.pre-commit-config.yaml` добавлено верхнеуровневое `exclude: '^services/_archive/'` — чтобы
автофиксеры (`golangci-lint-fmt`, `end-of-file-fixer`, `mixed-line-ending`) не правили содержимое
переносимых файлов и `git log --follow` оставался чистым.

**Дополнение сверх формулировки M-5**: хуку `gitleaks` добавлен `always_run: true`. Причина: хук
объявлен с `pass_filenames: false`, поэтому верхнеуровневый `exclude` не сужает область сканирования,
но **отключает хук целиком**, если все staged-файлы попали под `exclude` — то есть ровно на коммите
переноса. По §4.6 архив из gitleaks исключаться не должен; `always_run` это сохраняет.
Проверено: `pre-commit run --files services/_archive/configs/gm_defaults.yaml
services/_archive/shared/rules/engine.go` -> все автофиксеры `Skipped`, `Detect hardcoded secrets`
`Passed`; `git hash-object` файла до и после прогона совпадает.

### 2. Что перенесено (`git mv`, исходный путь -> архив)

| Исходный путь | Путь в архиве | Файлов |
|---|---|---|
| `services/ban-of-world` | `services/_archive/services/ban-of-world` | 6 |
| `services/reality-monitor` | `services/_archive/services/reality-monitor` | 5 |
| `shared/schema` | `services/_archive/shared/schema` | 2 |
| `shared/redis` | `services/_archive/shared/redis` | 1 |
| `shared/config` | `services/_archive/shared/config` | 2 |
| `shared/minio` | `services/_archive/shared/minio` | 8 |
| `shared/oracle` | `services/_archive/shared/oracle` | 3 |
| `shared/rules` | `services/_archive/shared/rules` | 2 |
| `shared/intent` | `services/_archive/shared/intent` | 4 |
| `shared/tinyml` | `services/_archive/shared/tinyml` | 2 |
| `shared/spatial` | `services/_archive/shared/spatial` | 6 |
| `shared/agent/tools/{adapter,entity_tool,narrative_tool,world_tool}.go` | `services/_archive/shared/agent/tools/` | 4 |
| `fake_deps/` | `services/_archive/fake_deps/` | 4 |
| `test_minio.go` | `services/_archive/test_minio.go` | 1 |
| `Dockerfile` (корневой) | `services/_archive/build/Dockerfile.root` | 1 |
| `services/{entity-actor,evolution-watcher,rule-engine}/Dockerfile` | `services/_archive/build/services/<svc>/Dockerfile` | 3 |
| `configs/gm_*.yaml` | `services/_archive/configs/` | 6 |
| `build/Dockerfile` (as-is) | `build/legacy.Dockerfile` | 1 |

Итого 60 переименований, все `R100`. Проверка:
`git diff --cached -M --summary | grep -c 'rename .*(100%)'` = 60.

**В `shared/agent/tools` остался только реестр инструментов** (`registry.go` + `go.mod`) — по
`foundation.md` §11 («`tools/*` кроме реестра»). Модуль собирается.

Каталог `configs/` после переноса остался пустым и удалён (`rmdir`; файлов в нём не было —
`find configs -type f` = 0). Git пустые каталоги не отслеживает, содержимое не потеряно.

### 3. Что создано

- `services/_archive/go.mod` — `module multiverse-core.io/archive`, `go 1.24`, **без `require`**:
  делает архив невидимым для `./...` корневого модуля (§4.6). Собственные `go.mod` у
  `ban-of-world`, `reality-monitor`, `shared/{config,minio,oracle,spatial}`, `fake_deps`
  переехали вместе с каталогами; у `shared/{schema,redis,rules,intent,tinyml}` своего `go.mod`
  не было (были пакетами корневого модуля) — их закрывает `services/_archive/go.mod`.
- `services/_archive/README.md` — индекс: 27 строк «исходный путь -> путь в архиве -> причина ->
  последний рабочий коммит -> эпик возврата», правила работы с архивом и таблица «что в архив
  в волне 0 **не** уходит».
- `ARCHIVED.md` — 15 файлов, по одному в каждом каталоге архива, по шаблону §4.6
  (исходный путь, причина со ссылкой на ADR/`overview` §16, коммит архивации, последний рабочий
  коммит, эпик возврата, что использовало).
- `FROZEN.md` — 8 файлов в замороженных сервисах (`world-generator`, `universe-genesis-oracle`,
  `ontological-archivist`, `cultivation-module`, `plan-manager`, `city-governor`, `entity-actor`,
  `evolution-watcher`). `ls services/*/FROZEN.md | wc -l` = 8.
- `Docs/archive/README.md` — каталог создан с индексом (пока пустым) и списком кандидатов;
  отбор и перенос документов — T-019 (F-9), владелец tech-writer.

`ARCHIVED.md`/`FROZEN.md` в поле «Коммит архивации» содержат ссылку на задачу T-002 и базу `04a3a15`;
**хэш самого коммита переноса не проставлен** — коммит выполняет оркестратор (`commits: ask`).
Проставить хэш после коммита — см. ОВ-4.

### 4. `go.work`

Из `use (...)` убраны 6 записей перенесённых модулей: `./services/ban-of-world`,
`./services/reality-monitor`, `./shared/config`, `./shared/minio`, `./shared/oracle`,
`./shared/spatial`. Без этого `go build ./...` в корне падает («directory does not exist»).
`./shared/agent/tools` оставлен — модуль остался на месте вместе с `registry.go`.
`go.work.sum` не трогался (файл целиком удаляется в T-003).
Записи `./services/_archive/**` в `go.work` **не добавлялись** — архив должен оставаться вне
рабочего пространства (§4.6).

### 5. Незакоммиченные правки пользователя, попавшие в перенос

`git mv` переносит рабочую копию файла. Два перенесённых файла имели незакоммиченные правки
пользователя (в `git status` они отмечены `RM`):

| Файл | Правка (не моя, не staged) |
|---|---|
| `services/_archive/services/ban-of-world/go.mod` | `go 1.24` -> `go 1.24.0`, удалён `require github.com/segmentio/kafka-go v0.4.49` |
| `services/_archive/services/reality-monitor/go.mod` | `go 1.24` -> `go 1.24.0`, удалён блок `require` целиком |

В индекс попал **только перенос** (версия из HEAD, `R100`); сами правки остались незастейдженными
и в коммит задачи не войдут. Остальные незакоммиченные правки пользователя
(`.claude/agents/*`, `.mcp.json`, `.qwen/*`, `Docs/dev-team/{dashboard.html,journal.md,state.js}`,
`go.mod` прочих сервисов, `mcp_kafka.log`) не трогались.

### 6. Результаты DoD

| Проверка | Результат |
|---|---|
| `services/_archive/README.md` перечисляет все перенесённые пути с причиной и коммитом | да, 27 строк = 60 перенесённых файлов; плюс таблица «что не уходит» |
| `ARCHIVED.md` в каждом каталоге | 15 файлов (все каталоги архива) |
| `ls services/*/FROZEN.md \| wc -l` = 8 | **8** |
| перенос, а не удаление+добавление | `git diff --cached -M --summary` — **60 из 60** `rename ... (100%)`. `git log --follow` по новому пути будет читаться **после коммита** (до коммита нового пути ещё нет ни в одном коммите); история старого пути читается сейчас: `git log --follow -- shared/oracle/client.go` -> 5+ коммитов |
| `go build ./...` в корне не видит `services/_archive/**` | `go list ./...` -> 3 пакета (`shared/entity`, `shared/jsonpath`, `shared/jsonpath/examples`); `go list ./... \| grep -c _archive` = **0** |
| `go build ./...` в корне | зелёный (до задачи — тоже) |
| `go vet ./...` в корне | зелёный. **До задачи** `go vet` выдавал `test_minio.go:44:3: result of fmt.Errorf call not used` — файл ушёл в архив, замечание снялось само |
| `go test -short -count=1 ./...` | зелёный (`shared/jsonpath` ok, у остальных нет тестов) |
| `go test -short -race` | **n/a на этой машине**: `-race` требует cgo, `gcc` в `PATH` нет. Ограничение окружения, не задачи; в CI (Linux, T-012) выполнится |
| `make lint` / `gofmt` | **n/a**: цели `lint` в as-is `Makefile` нет, `.golangci.yml` создаётся в T-003 (F-2). Go-файлы в задаче не менялись — только перемещались |
| Тесты на новое поведение (общий DoD §1 п. 3) | **n/a**: задача — перенос файлов и сопроводительные Markdown-документы, нового поведения нет. Проверки — прогоны выше плюс проверка действенности `exclude` в разделе 1 |
| `gitleaks git --staged --redact .` | **0 находок** (`no leaks found`, просканировано ~47,5 КБ) |
| `pre-commit run --files <файлы задачи>` | зелёный: gitleaks, large-files, end-of-file-fixer, check-yaml, check-merge-conflict, mixed-line-ending — Passed; golangci-lint(-fmt) — Skipped (нет `.go`) |
| Карта владения (`ownership.md` §1) | соблюдена: `services/_archive/**` (создаёт EPIC-001 F-3), `services/<8>/FROZEN.md`, `Docs/archive/**`, `build/`, `go.work`, `.pre-commit-config.yaml` — всё EPIC-001 |

### 7. Отклонения от дизайна и задачи

1. **`build/Dockerfile` -> `build/legacy.Dockerfile`** выполнено, хотя в списке файлов T-002 этой
   строки нет. Основание — `infrastructure.md` §10, строка F-3 (входит в «Ссылки» задачи):
   «as-is `build/Dockerfile` -> `build/legacy.Dockerfile`». Без переименования T-003 (F-2) пишет
   новый `build/Dockerfile` поверх as-is, а T-008 нужен `build/legacy.Dockerfile` для профиля
   `legacy`. `build/**` — владение EPIC-001. Переименование `R100`, история сохранена.
2. **`always_run: true` у хука `gitleaks`** — обоснование в разделе 1.
3. **Собственные `go.mod` в подкаталогах архива не создавались** там, где их не было
   (`shared/{schema,redis,rules,intent,tinyml}`, `test_minio.go`): достаточно одного
   `services/_archive/go.mod` — корневой `./...` архив уже не видит (проверено). Создание
   лишних модулей только добавило бы файлов, которых в исходном коде не было.
4. **Пустой каталог `configs/` удалён** (после переноса всех 6 `gm_*.yaml`). Это не код (U-1
   не затрагивается), git пустые каталоги не отслеживает.
5. **Каталогов из списка задачи, которых нет на диске, не обнаружено** — перенесено всё
   перечисленное, ничего не создавалось «под список».

### 8. Побочный эффект: модули, переставшие собираться

Замер `go build ./...` по каждому модулю рабочего пространства **до** и **после** переноса:

| Модуль | До | После | Ожидаемо? |
|---|---|---|---|
| корневой `multiverse-core.io` | ok | **ok** | да |
| `services/entity-actor` | ok | ломается (`shared/{intent,minio,redis,rules,tinyml}`) | **да** — заморожен, `foundation.md` §11: «их `go.mod` остаются автономными (могут не собираться)» |
| `services/evolution-watcher` | ok | ломается (`shared/{intent,minio,redis}`) | **да** — заморожен |
| `services/universe-genesis-oracle` | ok | ломается (`shared/oracle`) | **да** — заморожен |
| `services/world-generator` | ok | ломается (`shared/oracle`) | **да** — заморожен |
| **`shared/agent`** | ok | **ломается** (`filter.go` -> `shared/rules`) | **нет** — см. ОВ-1 |
| **`services/narrative-orchestrator`** | ok | **ломается** (`shared/{config,minio,oracle,spatial}` + транзитивно `shared/agent`) | **нет** — см. ОВ-2 |
| **`services/rule-engine`** | ok | **ломается** (`shared/minio`) | **нет** — см. ОВ-2 |
| `city-governor`, `cultivation-module`, `entity-manager`, `game-service`, `ontological-archivist`, `plan-manager`, `semantic-memory`, `shared/agent/tools`, `shared/eventbus` | ok | **ok** | да |

Общий DoD §1 п. 2 сформулирован как `go build ./... && go vet ./...` **в корне** — он зелёный.
Отдельные модули рабочего пространства в него не входят, а в T-003 (F-2) `go.work` удаляется целиком.

Дополнительно: as-is `docker-compose.yml` (строки 302/316/330) ссылается на перенесённые
`./services/<svc>/Dockerfile`, as-is `Makefile` (строки 10, 13) — на `ban-of-world`
и `reality-monitor`, а строка 31 — на `build/Dockerfile`. Оба файла переписываются в T-004 (F-6a)
и T-008 (F-6b); чинить их в T-002 не входит в задачу и в владение (см. ОВ-3).

### 9. Открытые вопросы

- **ОВ-1 (architect#1 / tech-lead#1, до T-003).** `shared/agent/filter.go` импортирует
  `multiverse-core.io/shared/rules`, который по `foundation.md` §11 уходит в архив. После переноса
  модуль `shared/agent` не собирается, а T-003 (F-2) должен влить `shared/agent` в единый модуль.
  Варианты: **(а)** архивировать `filter.go` вместе с рантаймом агента (в модуль по §11 идут только
  «типы/парсер/валидатор», фильтр к ним не относится) — тогда решение принимает архитектор и перенос
  делается тем же способом; **(б)** переписать `filter.go` под `internal/mechanics` — это EPIC-003,
  не волна 0; **(в)** вернуть `shared/rules` из архива — противоречит §11 и ADR-012.
  Сам решать не стал: `shared/agent/**` — общий код, `filter.go` в списке T-002 не значится.
- **ОВ-2 (tech-lead#1 + владельцы EPIC-002/EPIC-003).** `services/narrative-orchestrator` (нужен
  профилю `legacy` до S5) и `services/rule-engine` (живёт до EPIC-002 T-064) зависят от
  заархивированных `shared/{config,minio,oracle,spatial}`. Их `go.mod` — **пустые**
  (`module` + `go` без `require`/`replace`), то есть зависимости резолвились только через `go.work`;
  после T-003 (`go.work` удаляется) они сломались бы и без архива. Чтобы профиль `legacy` в T-008
  собрался, кому-то нужно прописать в `services/narrative-orchestrator/go.mod` `require`/`replace`
  на `../../services/_archive/shared/...`. Это не владение EPIC-001 и не входит в T-002 — решение и
  исполнителя определяет tech-lead#1 (кандидат: T-008 вместе с `build/legacy.Dockerfile`).
- **ОВ-3 (devops-engineer, T-004/T-008).** As-is `Makefile` и `docker-compose.yml` ссылаются на
  перенесённые пути (см. раздел 8). До T-004/T-008 as-is-сборка образов не работает. Подтвердить,
  что это ожидаемо, и не заводить дефект.
- **ОВ-4 (оркестратор).** Поле «Коммит архивации» в 15 `ARCHIVED.md`, 8 `FROZEN.md` и в
  `services/_archive/README.md` содержит ссылку на задачу и базу `04a3a15`, но не хэш коммита
  переноса (агент не коммитит). Нужно ли отдельным коммитом проставить хэш после коммита T-002,
  или ссылки на задачу достаточно (DoD говорит «с причиной и коммитом»).
- **ОВ-5 (devops-engineer).** `-race` на машине владельца не работает: `-race` требует cgo,
  `gcc` в `PATH` нет. Общий DoD §1 п. 2 требует `go test -short -race`. Либо поставить
  toolchain с gcc (MinGW/TDM-GCC), либо в T-012 зафиксировать, что `-race` — только CI (Linux),
  а локально критерий читается без `-race`.

### 10. Риски и допущения

- Индекс содержит переносы как `R100`, но `git log --follow` по новым путям станет доступен только
  после коммита. Если оркестратор закоммитит выборочно (частями, разными коммитами),
  распознавание переименований может ухудшиться — коммитить **всё staged одним коммитом**.
- `.gitattributes` из T-001 вводит `eol=lf`; при следующей нормализации перенесённые
  `configs/gm_*.yaml` (сейчас с CRLF) изменят окончания строк. Внутри архива это не помешает
  `--follow`, но даст «содержательный» diff. `exclude` в `pre-commit` от нормализации git не защищает.
- `services/_archive/go.mod` объявляет `go 1.24`. После T-003 (переход на Go 1.26) архив всё равно
  не собирается по назначению — версия оставлена «эпохи as-is» намеренно.
- `Docs/archive/` создан пустым (только `README.md`). Если по факту F-9 документы решат не
  переносить, каталог останется с одним индексом — это ожидаемо.

---

## developer#1 · T-003 · F-2 «Единый модуль Go 1.26, `cmd/multiverse`, `shared/runtime`, `shared/clock`» · 2026-09-09

Ветка `epic/EPIC-001-foundation`, база `5a20bb8` (T-002). Основание: `architecture/components/foundation.md`
v0.2 §1–§4, §11, §12 (F-2), §14 п. 2/3/8; `architecture/infrastructure.md` v0.3 §2.1, §2.3, §2.4, §10;
`epics/EPIC-001-foundation/design.md` §4.1; ADR-001 + дополнения п. 1–3, 5, 7, 8; `plan/ownership.md` §1.
Дополнительно — решение оркестратора по ревью T-002 (M-4 / ОВ-1, вариант «а»).
Коммит не выполнялся (`git.commits: ask`) — изменения подготовлены в индексе.

### 1. Версия Go

`go.dev/dl` доступен, последний патч линейки — **`go1.26.8`** (в наличии 1.26.0…1.26.8; актуальный
релиз Go — 1.27.1, что совпадает с §2.4). Зафиксировано:

- `go.mod`: `go 1.26` + `toolchain go1.26.8`;
- `build/Dockerfile`: `ARG GO_VERSION=1.26.8`.

Локально установлен Go **1.25.3**; тулчейн 1.26.8 подтянулся автоматически (`GOTOOLCHAIN=auto`,
`go: downloading go1.26.8 (windows/amd64)`), `go version` в модуле печатает `go1.26.8 windows/amd64`.
Отдельная установка дистрибутива не требовалась.

**Вход для T-004 (F-6a)**: в `build/versions.env` — `GO_VERSION=1.26.8` и
`MINIO_BUILDER_IMAGE=golang:1.26.8-bookworm` (первая попытка по ADR-021). Файл `build/versions.env`
в T-003 не создавался: он в списке файлов T-004, значение продублировано в `go.mod`/`Dockerfile`
как значение по умолчанию, `Dockerfile` принимает `--build-arg GO_VERSION`.

### 2. Единый модуль

- `git rm` `go.work`, `go.work.sum` (конфигурация workspace, не код — U-1 не касается, §11).
- `git rm` `shared/eventbus/go.mod`, `shared/agent/go.mod`, `shared/agent/tools/go.mod` — пакеты
  вошли в корневой модуль. У `shared/{entity,jsonpath}` своих `go.mod` не было.
- Корневой `go.mod`: `module multiverse-core.io`, `go 1.26`, `toolchain go1.26.8`, зависимости после
  `go mod tidy`: `google/uuid v1.6.0`, `segmentio/kafka-go v0.4.51` (пин §2.4, было v0.4.49),
  `gopkg.in/yaml.v3 v3.0.1`; indirect — `klauspost/compress`, `pierrec/lz4/v4`.
  Ушли зависимости заархивированного кода (ревью T-002, N-5): `xeipuuv/gojsonschema`,
  `yalue/onnxruntime_go`, `minio-go/v7`, `stretchr/testify` и весь их транзитивный хвост
  (`go.sum` −219 строк). Остальные библиотеки целевого набора §11 (`minio-go v7.3.0`,
  `santhosh-tekuri/jsonschema/v6`, `oklog/ulid/v2`, `testcontainers-go`, `goose`, `modernc.org/sqlite`,
  `go-telegram/bot`, `qdrant/go-client`, `neo4j-go-driver/v5`, `testify`) добавляют задачи, которые
  первыми их импортируют (F-4/F-5/F-5t и далее): `go mod tidy` не держит зависимость без импорта.
- Состав корневого модуля (`go list ./...`, 10 пакетов): `cmd/multiverse`,
  `shared/{agent,agent/tools,clock,entity,eventbus,eventbus/examples,jsonpath,jsonpath/examples,runtime}`.
- `services/_archive/**` и 8 замороженных сервисов в `./...` не попадают: у замороженных свои `go.mod`,
  а имя `_archive` начинается с `_`, поэтому go-инструмент такой каталог не обходит в принципе.

### 3. M-4 / ОВ-1 — архивация двух файлов `shared/agent`

По решению оркестратора (вариант «а» ревью T-002) через `git mv` перенесены в
`services/_archive/shared/agent/`:

| Исходный путь | Причина |
|---|---|
| `shared/agent/filter.go` | импортирует заархивированный `shared/rules`; в модуль по §11 идут только типы, парсер и валидатор блупринтов — фильтр переписывает EPIC-003 (`internal/llm/filter`) |
| `shared/agent/e2e_dark_forest_test.go` | тот же импорт + `shared/agent/tools/*` из архива; сценарий «Тёмный лес» пересобирается в F-10/EPIC-003 |

Создан `services/_archive/shared/agent/ARCHIVED.md`; в `services/_archive/README.md` добавлены две
строки индекса, уточнена оговорка про коммит архивации (эти два файла — T-003, база `5a20bb8`,
остальные — T-002) и строка «что в архив не уходит». Внешних импортёров у `filter.go` нет
(`grep` по `FilterResult|DeterministicOutput|NewFilter` вне архива — пусто). После переноса
`shared/agent` собирается, `go vet` и тесты пакета зелёные.

### 4. Что создано

**`shared/clock`** (`clock.go`, `manual.go`): `Clock`, `Timer`, `Timers`, `Real`/`RealTimers`,
`Manual` (`NewManual`, `Now`, `Set`, `Advance`, `Timers()`), `ManualTimers` (`After`, `Every`).
`Manual` срабатывает по `Advance`/`Set` строго в порядке дедлайнов, канал таймера буферизован на 1
(пропущенные периоды схлопываются в один тик — семантика `time.Ticker`), движение времени назад
ничего не запускает. `EventClock`/`NullTimers` для replay — `internal/replay` (EPIC-002), как в §4.

**`shared/runtime`** (`runtime.go`, `http.go`, `lifecycle.go`): `Mode`, `Status`, `Deps`, `Context`,
`Routes`, `Registry` (`Register`/`Names`/`New` + пакетные обёртки над реестром по умолчанию),
`StartAll`/`StopAll`/`Aggregate`, `HTTP` (`NewHTTP` с `GET /health`, `Start`/`Stop` с graceful 5 с),
`AdminOnly`, `EnvAddr`. `New` упорядочивает контексты топологически по `DependsOn`, ничьи разрешает
порядком регистрации (результат не зависит от порядка аргументов), ловит цикл и неизвестное имя
(`ErrUnknownContext`), понимает `all`. Зависимость на контекст, которого нет в выбранном наборе,
не считается ошибкой: такой контекст работает в другом процессе и доступен через шину.

**`cmd/multiverse`** (`main.go`, `serve.go`, `contexts.go`, `health.go`, `db.go`):

- флаги `--contexts` (обязателен), `--mode=live|replay`, `--bus=kafka|memory` (`memory` только с
  `--contexts=all`), `--recording` (только с `--mode=replay`), `--id-source=uuid|sequence`;
- подкоманды `health --url` (healthcheck distroless-образа: 0 при `200` + `status=ok`, иначе 1),
  `db backup|check`, `version`;
- адрес HTTP — env `MV_CORE_ADDR` (по умолчанию `127.0.0.1:8090`), флага нет (D-7);
- регистрация 7 пустых контекстов `state, laws, mechanics, llm, swarm, gateway, memory`
  (`Health()=ok`, `Start/Stop` — no-op) в порядке §2; владелец заменяет заглушку своим
  `runtime.Register` в своей ветке.

**`.golangci.yml`** (v2, пин `v2.13.2`):

- линтеры: `depguard`, `errcheck`, `forbidigo`, `govet`, `ineffassign`, `staticcheck`, `unused`;
  форматтеры `gofmt` + `goimports` (`local-prefixes: multiverse-core.io`);
- `depguard` — 10 правил по границам ADR-001 п. 3 и доп. п. 2/7: `shared/*` не импортирует
  `internal/*`; для каждого `internal/<ctx>` перечислены запрещённые соседи, разрешены только
  `swarm → mechanics|laws|llm`, `state → mechanics`, `memory → llm` (при запрете
  `llm/{guardian,prompt,filter,parser}`); `internal/replay` запрещён всем, кроме `cmd/multiverse`
  (отдельное правило запрещает его и `cmd/mvctl`, `cmd/telegram-bot`);
- `forbidigo`: `os.Getenv|os.LookupEnv`, `time.Now`, `log.Print*|Fatal*|Panic*`; исключение по
  `os.*`/`time.Now` — только `shared/**` (там живут `clock`, `env`, `runtime`);
- `issues.max-issues-per-linter: 0`, `max-same-issues: 0` — иначе golangci-lint по умолчанию
  показывает не больше трёх однотипных находок и часть проблем не видна в ревью;
- исключены `services/_archive` и `services/` (замороженные и legacy — вне модуля).

Правила проверены на живом примере: во временных пакетах `internal/{state,memory,laws}` и
`shared/probe` (созданы и удалены в ходе задачи, в индекс не попали) depguard отклонил
`state → swarm`, `state → replay`, `memory → llm/guardian` и `shared → internal`, пропустил
`memory → llm`; forbidigo отработал на `os.Getenv`/`time.Now`/`log.Printf` вне `shared/*`.

**`build/Dockerfile`** по §2.3: multi-stage `golang:${GO_VERSION}-bookworm` → `distroless/static-debian12:nonroot`,
`CGO_ENABLED=0`, `GOFLAGS=-trimpath`, кэш модулей и сборки через `--mount=type=cache`,
`-ldflags "-s -w -X main.version=${VERSION}"`, `HEALTHCHECK` вызывает `/multiverse health --url …`.

### 5. Результаты DoD

| Критерий | Результат |
|---|---|
| `go mod tidy && go build ./... && go vet ./...` | зелёные |
| `go mod verify` | `all modules verified` |
| `make lint` (`golangci-lint run ./...`) | `0 issues` |
| `gofmt`/`goimports` без диффа | `gofmt -l` пусто; `golangci-lint fmt` без изменений |
| `go test -short -count=1 ./...` | 6 пакетов ok, 4 без тестов |
| Покрытие нового кода | `shared/runtime` 93,2 %, `shared/clock` 87,1 %, `cmd/multiverse` 61,6 % |
| `go run ./cmd/multiverse --contexts=all --bus=memory` → `curl 127.0.0.1:8090/health` | `{"status":"ok","details":{"contexts":{"gateway":…,"laws":…,"llm":…,"mechanics":…,"memory":…,"state":…,"swarm":…}}}` |
| `--contexts=state,gateway` поднимает только их | да, в `details.contexts` ровно два имени (проверено на `MV_CORE_ADDR=127.0.0.1:8099`) |
| `go list ./... \| grep -c _archive` | `0`; замороженных сервисов в списке тоже 0 |
| `docker build -f build/Dockerfile .` | образ собран; `docker run … --contexts=all --bus=memory` → `/health` = ok, `docker exec … /multiverse health --url …` → `ok`, exit 0, `docker inspect .State.Health.Status` = `healthy` |
| `pre-commit run --files <файлы задачи>` | 8 хуков Passed (включая `gitleaks`, `golangci-lint`, `golangci-lint-fmt`) |
| `gitleaks git --staged --redact .` | `no leaks found` |
| `gitleaks git --redact --log-opts="integration/mvp-1..HEAD" .` | `no leaks found` |
| `go test -short -race` | **не выполнялся** — на машине нет gcc/cgo (решение по ОВ-5: `-race` только в CI, T-012) |

`gitleaks dir --redact .` по рабочей копии даёт 5 находок, все — в неотслеживаемых файлах вне
индекса: `.env` (реальный env владельца, в `.gitignore`) и `.claude/worktrees/frosty-bell/{.env,shared/oracle/README.md}`
(остаток снятого в F-0 worktree). Задачей не внесены; см. ОВ-8.

### 6. Отклонения от дизайна

1. **`runtime.Deps` неполный.** По `foundation.md` §3 в `Deps` есть `Bus`, `Journal`, `Store`, `Env`,
   `Contracts`. Пакетов `shared/{objstore,env,contracts}` и интерфейсов `eventbus.Bus`/`Journal`
   ещё нет (F-4a — T-005, F-4b — T-006, F-5 — T-007), объявить поля не на чем. В T-003 `Deps`
   содержит `Clock`, `Timers`, `Mode`, `IDs`, `Log`, `Mux`; в комментарии к типу перечислено, какая
   задача добавляет каждое оставшееся поле. Добавление поля в `Deps` — совместимое изменение.
2. **`--bus` только валидируется.** Реализации шины (`kafka`, `membus`) появляются в F-4a/F-5t;
   в волне 0 флаг разбирается и проверяется (`memory` только с `--contexts=all`), но объект шины
   в `Deps` не кладётся. То же с `--recording`: путь проверяется, читает его `internal/replay` (EPIC-002).
3. **`--mode=replay` без `EventClock`.** До появления `internal/replay` replay получает
   `clock.Manual`, который никто не двигает, — для пустых контекстов этого достаточно.
4. **`build/Dockerfile` копирует каталог `/out/`, а не три файла поимённо.** В §2.3 указано
   `COPY --from=builder /out/multiverse /out/telegram-bot /out/mvctl /`; `cmd/mvctl` создаёт T-010,
   `cmd/telegram-bot` — EPIC-004, и до тех пор такой `COPY` падает. `-o /out/ ./cmd/...` + `COPY /out/ /`
   даёт тот же результат и не требует править файл при появлении новых бинарников. По той же причине
   опущены `COPY` каталогов `blueprints/`, `rules/`, `laws/`, `config/` — их создают F-10 и EPIC-003;
   в файле стоит комментарий с задачами.
5. **`.dockerignore` не создан** — он в списке файлов T-004 (F-6a). Без него в контекст сборки уходят
   `Docs/`, `services/`, `.git`; сборка проходит (проверено), но контекст лишний.
6. **Чтение env — `runtime.EnvAddr`.** `shared/env` появляется в T-007, а `forbidigo` запрещает
   `os.Getenv` вне `shared/*`, поэтому `MV_CORE_ADDR` читает хелпер в `shared/runtime`; в комментарии
   указано, что его заменяет манифест `shared/env`.
7. **Хук `MV_SWARM_FAKE` (`cmd/multiverse/fake_contexts.go`, ADR-001 доп. п. 8) не делался** —
   по `ownership.md` §1 он относится к F-2/F-10 и требует `shared/testkit/swarm.FakeContext`,
   которого ещё нет (F-10, T-018). Ожидаемое место — T-018.
8. **`db backup|check` — распознаваемые подкоманды без реализации.** Базы данных платформы
   (`links.db`, `gateway.db`) появляются в EPIC-004 (ADR-019); сейчас обе печатают причину и
   возвращают 1, неизвестная подкоманда — 2. Тест это фиксирует.

### 7. Правки в as-is коде, потребовавшиеся для «зелёного» линтера

Задача переводит `shared/{eventbus,agent,entity,jsonpath}` в единый модуль, после чего они впервые
попадают под `golangci-lint`. Сделано минимально:

- `.gitattributes`: добавлена строка **`*.go text eol=lf`**. При `* text=auto` и `core.autocrlf=true`
  все `.go` выкладываются на диск с CRLF, и `gofmt`/`golangci-lint` считают неотформатированным
  **каждый** файл — DoD §1 п. 1 локально не выполним в принципе. Содержимое в индексе уже LF, поэтому
  строка не даёт диффа по коду; рабочая копия приведена к LF (131 файл, `git diff` по ним пуст).
  Файл в границах владения EPIC-001 (`ownership.md` §1), но в списке файлов T-003 его не было —
  фиксирую как добавление, см. ОВ-6.
- `gofmt`/`goimports` применены к 14 файлам `shared/agent`, `shared/agent/tools`, `shared/eventbus`
  (только пробелы и группировка импортов, `+324/−324`, семантики не меняет).
- `shared/jsonpath/accessor.go`: `reflect.Ptr` → `reflect.Pointer` (это алиас той же константы;
  `go vet` Go 1.26 добавил проверку `inline` и ругается на устаревшее имя). Единственная правка,
  затрагивающая код, и она не меняет поведение.
- Историческая семантика as-is кода **не исправлялась**: в `.golangci.yml` заведены точечные
  исключения с указанием задачи, которая их снимет — `shared/eventbus/` (`errcheck`, `forbidigo`;
  снимается в F-4a/T-005), `shared/agent/` (`errcheck`, `staticcheck`, `unused`; снимается при
  переписывании рантайма агента в EPIC-003), `shared/*/examples/` (`forbidigo`, `unused`; судьбу
  демо-программ решает F-9/T-019). Исключения — только по перечисленным линтерам, форматирование
  проверяется везде.

### 8. Открытые вопросы

- **ОВ-6 (tech-lead#1).** Строка `*.go text eol=lf` в `.gitattributes` добавлена вне списка файлов
  T-003 (обоснование — раздел 7). Подтвердить или перенести правку в отдельную задачу; без неё
  `make lint` на Windows красный на всём репозитории.
- **ОВ-7 (tech-lead#1 / system-architect).** Исключения as-is пакетов в `.golangci.yml` (раздел 7) —
  временные. Нужно ли завести на них задачи явно (снятие исключения `shared/eventbus` — критерий
  DoD T-005; `shared/agent` — критерий EPIC-003), чтобы они не остались навсегда.
- **ОВ-8 (devops-engineer, T-008/T-012).** `make secrets-scan` по `infrastructure.md` §2.2 включает
  `gitleaks dir --redact .` по рабочей копии. На машине владельца это всегда 5 находок:
  реальный `.env` (в `.gitignore`, секреты там по назначению) и остаток каталога
  `.claude/worktrees/frosty-bell/` (снятый в F-0 worktree, физически на диске). Решить: сузить цель
  до отслеживаемых файлов, добавить пути в `.gitleaks.toml` (`allowlist.paths`) или удалить
  каталог worktree с диска (последнее — решение пользователя, там лежит его `.env`).
- **ОВ-9 (tech-lead#1 / T-004).** `build/versions.env` создаёт T-004; значения из T-003 —
  `GO_VERSION=1.26.8`, `GOLANGCI_LINT_VERSION=v2.13.2`. Проверить, что `go.mod` (`toolchain go1.26.8`)
  и `versions.env` не разъедутся: сверка «`toolchain` = `GO_VERSION`» — кандидат в `mvctl env check`
  (T-010) или в job `unit` (T-012).
- **ОВ-10 (tech-lead#1 / EPIC-002).** `shared/entity` вошёл в модуль без тестов (`[no test files]`).
  Общий DoD §1 п. 3 требует тесты на новый код; здесь кода не писалось — это перенос. Полная модель
  v2 с тестами — F-10/T-011.

### 9. Установка инструментов

`golangci-lint` на машине не было. Установлен пином §2.4:
`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2` →
`%USERPROFILE%\go\bin\golangci-lint.exe` (каталог уже в `PATH`, там же `gitleaks`).
`golangci-lint --version` → `2.13.2 built with go1.26.8`. Хуки `pre-commit` из T-001 подхватывают
его автоматически. `make` на машине по-прежнему нет (`infrastructure.md` §2.1:
`winget install ezwinports.make`), поэтому цели `make lint`/`make test` прогонялись их содержимым
напрямую; сам `Makefile` — задача T-004/T-008.

### 10. Риски и допущения

- `-race` локально не проверялся (нет cgo/gcc) — по решению оркестратора проверка уходит в CI (T-012).
  Гонки вероятнее всего в `shared/clock.Manual` (мьютекс + буферизованные каналы) и
  `shared/runtime.Registry`/`HTTP`; тесты этих пакетов написаны так, чтобы `-race` их покрывал.
- Тулчейн 1.26.8 скачивается автоматически по `GOTOOLCHAIN=auto`. На машине без сети и без
  установленного Go 1.26 сборка не пойдёт — в CI `setup-go` с `go-version-file: go.mod` (T-012)
  этот случай закрывает.
- `GOFLAGS=-buildvcs=false` требуется локально (путь с кириллицей, `infrastructure.md` §2.1);
  все команды задачи прогонялись с ним. В `Makefile` (T-004/T-008) он должен экспортироваться
  для локальных целей.
- `cmd/multiverse/main.go` намеренно тонкий: диспетчер подкоманд и `run()`. Владельцы контекстов
  добавляют только строку импорта в `contexts.go` — это уменьшает число PR, проходящих через
  tech-lead#1.
- Каталог `shared/*/examples/` остался в сборке (`go build ./...` их компилирует). Если F-9 решит,
  что демо-программы не нужны, их место — `services/_archive/` или `Docs/`.

---

## developer#1 · T-003 · итерация 2 (исправления по ревью #1) · 2026-09-09

Основание: `review.md`, запись «T-003 · ревью #1 · 2026-09-09» (вердикт «вернуть»), и решение
оркестратора по M-1 из `journal.md` (админ-доступ решается по клиенту, а не по виду актора).
В границах итерации — M-1, Mi-1, Mi-3, Mi-4, Mi-5, Mi-8. Mi-2, Mi-6, Mi-7 и Nit не трогались:
они назначены в T-004/T-012 и бэклог T-019. Коммит не выполнялся, изменения в индексе.

### 1. M-1 — `AdminOnly` решает по клиенту (`shared/runtime/http.go`)

Было: допуск по `X-Actor-Kind ∈ {ci, operator}`; `operator` — не значение `actor_kind`, а `client_id`,
поэтому штатный оператор (`human` через прокси gateway) получал `403`.

Стало (ADR-009 п. 9, C-01, C-06):

- `X-Client-Id` сверяется со списком из `MV_CORE_ADMIN_CLIENTS` (значение по умолчанию — `operator`,
  список через запятую, пробелы обрезаются, пустые элементы отбрасываются). Отсутствующий или пустой
  заголовок не совпадает ни с чем → `403`.
- `X-Actor-Kind` необязателен; если передан — проверяется на принадлежность enum
  `human|ci|sim|system` и ничего не разрешает сам по себе. Неизвестное значение → `403`.
- Константы: `ActorKindHeader`, добавлены `ClientIDHeader`, `EnvAdminClients`, `DefaultAdminClients`;
  `ActorKindCI` и `ActorKindOp` удалены (правка «б» ревью). Enum — неэкспортируемая карта
  `actorKinds`.
- Список читается один раз, при построении middleware (`AdminOnly(next)`), а не на каждый запрос;
  это записано в doc-комментарии.
- `EnvAddr` сохранён как есть (используется `serve.go`) и делегирует новому `envOr`, чтобы чтение
  окружения в пакете осталось в одном месте до появления `shared/env` (F-5, T-007).

Тесты `shared/runtime/http_test.go`: `TestAdminOnly` переписан на пары заголовков — оператор
(`operator` + `human`) → 200; оператор без `X-Actor-Kind` → 200; `ci`-клиент из списка
(`MV_CORE_ADMIN_CLIENTS=operator,ci-harness`, `ci-harness` + `ci`) → 200; чужой клиент
(`telegram-bot`) → 403; без `X-Client-Id` → 403; вовсе без заголовков → 403; пустое значение → 403;
неизвестный `actor_kind` (`operator` в роли вида актора) → 403; `Human` в другом регистре → 403;
клиент, выпавший из переопределённого списка → 403. Старый кейс `http_test.go:70-71`, закреплявший
прежнее поведение, удалён вместе с константами. Добавлен `TestAdminOnlyReportsReason` — в теле `403`
названа причина и заголовок `X-Client-Id`.

`MV_CORE_ADMIN_CLIENTS` внесена в `.env.example` строкой ниже `MV_CORE_ADDR` (файл T-001; правка в
одну строку, содержательно относится к T-003).

### 2. Mi-1 — `forbidigo` только для владельцев времени и окружения (`.golangci.yml`)

Исключение `path: shared/` заменено на `path: shared/(clock|runtime|env)/` (`env` объявлен заранее,
появится в T-007). Прогон после сужения дал 38 issues на as-is коде:
`shared/agent` + `shared/agent/tools` (27) и `shared/entity` (11). `shared/eventbus` снова под
правилом `time.Now` лишь формально: его временное исключение (снятие — T-005) перечисляет
`forbidigo` целиком.
По образцу ОВ-7 добавлено: `forbidigo` в существующее исключение `shared/agent/` (снятие — EPIC-003,
там же переписывается рантайм агента) и новое исключение `shared/entity/` только на `forbidigo`
(снятие — F-10/T-011, модель сущности v2, где отметки времени уходят в `shared/clock`).
Итог: `golangci-lint run ./...` → `0 issues`.

### 3. Mi-3 — depguard стал fail-closed (`.golangci.yml`)

Правила контекстов переведены с deny-перечислений соседей на `list-mode: lax` + deny всего префикса
`multiverse-core.io/internal` + `allow` с явным перечислением разрешённых соседей. В режиме `lax`
запрет действует на всё, что не названо в `allow`, а стандартная библиотека, `shared/*` и внешние
модули остаются открытыми — то есть fail-closed ровно по границам ADR-001 п. 3, без превращения
списка зависимостей `go.mod` в часть конфига линтера.

Разрешения: `state → mechanics`; `mechanics` — ничего; `swarm → mechanics, laws, llm`; `llm`, `laws`,
`gateway`, `replay` — ничего; `memory → llm` (точное совпадение корневого пакета, запись
`multiverse-core.io/internal/llm$`) и `llm/providers`, подпакеты `llm/guardian|prompt|filter|parser`
остаются закрытыми (ADR-001 доп. п. 7).

Добавлено правило-ловушка `internal-unlisted`: `files: **/internal/**` с отрицаниями
(`!**/internal/state/**` и так далее для восьми известных имён) и deny всего `internal`. Новый
`internal/<ctx>` без собственного правила не получает молчаливого разрешения — линтер требует сначала
объявить границы. Это записано в комментарии правила.

Проверено на временных пакетах (созданы, прогнаны, удалены, в индекс не добавлялись):
`internal/{state,mechanics,swarm,llm,llm/guardian,llm/providers,laws,memory,foo}`. Ожидаемые 4 отказа
получены, других нет: `state → memory`, `swarm → state`, `memory → llm/guardian`,
`foo → mechanics` (ловушка). Разрешённое прошло: `state → mechanics`, `state → shared/clock`,
`state → github.com/google/uuid`, `memory → llm` и `memory → llm/providers`, `swarm → laws|llm`.
Это закрывает и замечание ревьюера о непроверенных глобах `**/internal/<ctx>/**`.

### 4. Mi-4 — distroless по digest (`build/Dockerfile`)

`FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872…f7ab` (индекс-манифест, multi-arch;
получен 2026-09-09). В комментарии над строкой — обе команды обновления digest:
`docker buildx imagetools inspect` и вариант через `curl` к `gcr.io/v2/.../manifests/nonroot`
(заголовок `Docker-Content-Digest`), чтобы T-004 и Dependabot не искали способ.

### 5. Mi-5 — `HEALTHCHECK` убран из образа (`build/Dockerfile`)

Инструкция удалена: образ один на все роли, порты разные (gateway :8088, core :8090, memory :8082),
и вшитый URL пометил бы две роли из трёх `unhealthy`. На её месте комментарий с причиной и готовой
строкой `healthcheck` для compose (`infrastructure.md` §2.3, T-004) — проверка по-прежнему вызывает
сам бинарник (`/multiverse health --url …`), в образе нет ни shell, ни curl. `docker build` в этой
итерации не перезапускался: правки статические (удаление инструкции и пин базового образа).

### 6. Mi-8 — гонка в `realTicker.Stop()` (`shared/clock/clock.go`)

`stopped bool` заменён на `atomic.Bool` с `CompareAndSwap(false, true)`: первый `Stop()` возвращает
`true` и останавливает `time.Ticker`, все последующие — `false`. Причина выбора (владелец держит
тикер в рабочей горутине, а останавливает из `Stop(ctx)`) записана комментарием к типу.
Тест `TestRealTickerStopIsIdempotentAndConcurrent`: повторный `Stop()` возвращает `false`; восемь
горутин наперегонки останавливают один тикер — ровно одна видит `true`. Под `-race` (CI, T-012) этот
тест покрывает исходное замечание; локально `-race` не запускался (нет gcc, ОВ-5).

### 7. Результаты DoD итерации

| Проверка | Результат |
|---|---|
| `go build ./...` | ✔ |
| `go vet ./...` | ✔ |
| `go test -short -count=1 ./...` | ✔ 6 пакетов `ok`, 4 `[no test files]` |
| `golangci-lint run ./...` (v2.13.2) | ✔ `0 issues` |
| `golangci-lint fmt --diff ./...`, `gofmt -l cmd shared` | ✔ пусто |
| `pre-commit run --files <изменённые>` | ✔ все хуки Passed |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| `-race` | локально не запускался (ОВ-5); точка внимания T-012 по `realTicker` снята по существу (Mi-8) |

### 8. Открытые вопросы

- **ОВ-11 (system-architect).** Решение по M-1 принято оркестратором и реализовано: админ-допуск —
  по `X-Client-Id` из `MV_CORE_ADMIN_CLIENTS`, `X-Actor-Kind` только валидируется по enum
  `human|ci|sim|system`. Требуется уточнить формулировку C-06 и ADR-009 п. 9 («`X-Actor-Kind: ci|sim`
  только для разрешённых клиентов» читается как допуск по паре заголовков) и решить, нужен ли на
  `core` разбор формата `client_id:platform:allowed_actor_kinds` из `MV_GATEWAY_CLIENTS`
  (`gateway-and-bot.md` §868) — сейчас `core` знает только список идентификаторов клиентов, а
  сопоставление «клиент ↔ допустимые виды актора» остаётся за gateway (EPIC-004).
- **ОВ-12 (tech-lead#1 / T-011).** Временное исключение `forbidigo` для `shared/entity` (раздел 2) —
  внести снятие явным критерием DoD T-011, как это сделано для `shared/eventbus` (T-005) и
  `shared/agent` (EPIC-003) по решению ОВ-7.
- **ОВ-13 (devops-engineer / T-004).** Digest distroless в `build/Dockerfile` теперь пин: при
  подключении Dependabot (экосистема `docker`, каталог `build/`) убедиться, что правило обновляет
  строку `FROM … @sha256:`; при ручном обновлении пользоваться командами из комментария.

### 9. Риски и допущения

- `MV_CORE_ADMIN_CLIENTS` читается один раз при построении middleware. Пока `AdminOnly` вызывается на
  старте процесса, это эквивалентно чтению на каждый запрос; при появлении горячей перезагрузки
  конфигурации (в MVP-1 не планируется) поведение надо пересмотреть.
- Значение по умолчанию `operator` означает, что без переменной окружения `/v1/admin/*` доступен
  только клиенту `operator`; `ci-harness` в CI (T-012) и `mvctl` (T-010) должны быть добавлены в
  список явно — иначе получат `403`. Это ожидаемо (fail-closed), но требует строки в их DoD.
- depguard в режиме `lax` оставляет внешние модули открытыми: границы третьих сторон по-прежнему
  контролирует `go.mod`, а не линтер. Альтернатива (полные allow-списки с перечислением
  `github.com/...`) сделала бы правку `.golangci.yml` обязательной при добавлении любой зависимости —
  отклонено как избыточное для ADR-001 п. 3.
- Digest distroless зафиксирован на 2026-09-09. Обновление базового образа теперь осознанное
  действие (в этом и смысл пина); без Dependabot он будет стареть — отслеживается в T-004.

---

## developer#2 · T-004 · F-6a «`build/versions.env`, образ MinIO из исходников, ядро compose» · 2026-09-09

Ветка `epic/EPIC-001-foundation`, база `b7df900` (после T-003). Основание:
`architecture/infrastructure.md` v0.3 §1.1–§1.4, §2.2–§2.5, §3.1, §3.1.1, §3.2, §4.2, §5.1, §5.2,
§5.5, §10 (F-6); ADR-021 (вариант B), ADR-005 доп. 2 п. 7, ADR-004 доп. п. 4/6, ADR-001 доп. п. 4/6;
`tasks.md` §3 T-004. Коммит не выполнялся (`git.commits: ask`) — изменения подготовлены в индексе.
Параллельно developer#1 вёл T-005 в `shared/eventbus/**` и `.golangci.yml`; эти пути не трогались.

### 1. Что создано и изменено

| Файл | Что |
|---|---|
| `build/versions.env` | новый; пины §2.4, комментарии только отдельными строками |
| `build/minio.Dockerfile` | новый; сборка MinIO из исходников тега, alpine-runtime, non-root, healthcheck |
| `docker-compose.yml` | переписано ядро: `redpanda`, `minio`, `gateway`, `core`, `ollama` (профиль `gpu`) |
| `services/_archive/build/docker-compose.as-is.yml` | as-is compose перенесён `git mv` (U-1), не удалён. Путь `docker-compose.yml` занят новым ядром, поэтому git фиксирует не переименование, а копию: `--follow` истории не даст, as-is содержимое читается по `git show b7df900:docker-compose.yml` (записано и в `ARCHIVED.md`) |
| `services/_archive/build/ARCHIVED.md` | добавлен раздел про перенесённый compose (причина, коммиты, источник профиля `legacy`) |
| `.dockerignore` | новый; §2.3 + `.env*`, `.claude/`, `Docs/`, `services/**`, `*.exe`, `*.log` |
| `.github/ci.env` | новый; `.env.example` с фиктивными значениями для `${VAR:?}` |
| `Makefile` | добавлены `include build/versions.env`, цели `image` и `minio-image` (только они — остальной набор §2.2 в T-008) |
| `backups/minio-src-RELEASE.2025-10-15T17-29-55Z.tar.gz` | тарбол исходников MinIO, 24 257 476 байт, **вне git** (`/backups/` в `.gitignore`) |

### 2. Версии: что откуда взято

Все значения — из `infrastructure.md` v0.3 §2.4 (проверено 2026-09-09) без изменений, кроме трёх:

- `GO_VERSION=1.26.8` — конкретный патч взят из `go.mod` (`toolchain go1.26.8`, зафиксирован в
  T-003), а не «1.26.x»; `MINIO_BUILDER_IMAGE=golang:1.26.8-bookworm` — тот же патч. Обе метки
  проверены `docker manifest inspect` (существуют; запасной `golang:1.24.10-bookworm` тоже есть).
- `LLAMACPP_BUILD=b10441` — по установленной у владельца сборке:
  `D:\Models\llama\llama\llama-server.exe --version` даёт
  `version: 0.1.0-dev (build 10441, commit 0177dcc73)`. Задача предписывала внести фактическое
  значение — оно снято с машины, а не выдумано.
- `MINIO_REPO=https://github.com/minio/minio.git` — **upstream, а не форк владельца**: форка нет,
  см. ОВ-14.

`CHROMA_IMAGE` объявлена, но оставлена **пустой** — см. отклонение 3 и ОВ-16.

### 3. MinIO из исходников (ADR-021 вариант B)

Собрано с первой попытки на `golang:1.26.8-bookworm` — откат на `golang:1.24.x` не потребовался
(`go.mod` тега объявляет `go 1.24.0`, новый toolchain собрал его без правок). Проверено:

- `docker build -f build/minio.Dockerfile --build-arg MINIO_REPO=https://github.com/minio/minio.git`
  с остальными аргументами из `versions.env` — успешно; образ
  `multiverse-core/minio:RELEASE.2025-10-15T17-29-55Z`, 164 МБ на диске;
- `docker run --rm $MINIO_IMAGE --version` печатает
  `minio version RELEASE.2025-10-15T17-29-55Z (commit-id=9e49d5e7...)`, `Runtime: go1.26.8 linux/amd64`;
- контейнер поднимается и переходит в `healthy` по healthcheck самого образа;
- `mc admin info local` из `minio/mc:RELEASE.2025-08-13T08-35-41Z` против собранного образа:
  `Version: 2025-10-15T17-29-55Z`, `1 drive online, 0 drives offline`;
- `git ls-remote --tags` upstream по тегу `RELEASE.2025-10-15T17-29-55Z` даёт
  `9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a` (тег на месте, репозиторий архивирован, но доступен);
- тарбол: `git archive --format=tar.gz --prefix=minio-<tag>/ <tag>` в
  `backups/minio-src-RELEASE.2025-10-15T17-29-55Z.tar.gz`; распаковка проверена, внутри
  `minio-RELEASE.2025-10-15T17-29-55Z/go.mod` = `module github.com/minio/minio`, `go 1.24.0`.

Проверки выполнялись `docker build`/`docker run`; `docker compose up` не запускался (запрет задачи).

### 4. Ядро compose: принятые решения

- Состав без профилей: `redpanda`, `minio`, `gateway`, `core` (§1.3); `ollama` — единственный
  сервис с профилем (`gpu`), нужен только при `MV_LLM_PROVIDER=ollama`. Сервиса `llama-server` нет
  и профиля для него не заводится (ADR-005 доп. 2 п. 7); у `core` —
  `extra_hosts: ["host.docker.internal:host-gateway"]`.
- Все публикации портов — `127.0.0.1`: `19092`/`9644` (Redpanda), `9000` (MinIO API), `8088`
  (gateway), `8090` (core), `11434` (Ollama). Консоли (MinIO `9001`, Redpanda Console, Neo4j `7474`)
  не публикуются — они в профиле `dev` (T-008, SEC-33).
- Пароли и ключи — только `${VAR:?подсказка}`: `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`,
  `MV_MINIO_ACCESS_KEY`, `MV_MINIO_SECRET_KEY`. Остальные переменные — `${VAR:-дефолт из §4.2}`,
  чтобы файл читался как документация состава окружения.
- `env_file: .env` не используется ни у одного сервиса: иначе токен бота и все секреты попадут во
  все контейнеры и правило 4 `compose-lint` станет непроверяемым. Вместо этого явные `environment`.
- Ротация логов `50m x 5` — через якорь `x-logging`; общая часть платформенных сервисов (образ,
  `build`, `restart`, логи) — через якорь `x-platform` с `<<:`.
- Healthcheck платформы вызывает сам бинарник (`/multiverse health --url ...`): в distroless нет ни
  shell, ни curl (это же зафиксировано комментарием в `build/Dockerfile`, Mi-5 из T-003).
- `depends_on` пока `redpanda: service_healthy` + `minio: service_healthy`; на
  `redpanda-init`/`minio-init` (`service_completed_successfully`, §5.1) их переводит T-008 —
  в файле стоит комментарий с этим указанием.
- Тома: `redpanda_data`, `minio_data`, `gateway-data`, `ollama_data`. As-is тома Redpanda 24.2 и
  MinIO не апгрейдятся на месте: §5.5 требует `make archive-legacy` и пересоздание — это
  операторский шаг, в compose он не автоматизируется (комментарий в секции `volumes`).
- `version: '3.8'` из as-is файла убран (Compose v2+ его игнорирует и предупреждает).

### 5. Отклонения от дизайна

1. **`ENV MINIO_RELEASE=RELEASE` и явный аргумент версии в `build/minio.Dockerfile`.** Фрагмент §2.5
   даёт `go run buildscripts/gen-ldflags.go` без аргументов и без переменной. Собранный так образ
   печатает `minio version DEVELOPMENT.2025-10-15T17-29-55Z`, то есть критерий DoD «печатает
   `RELEASE.2025-10-15T17-29-55Z`» не выполняется: `gen-ldflags.go` берёт префикс из `MINIO_RELEASE`
   (по умолчанию `DEVELOPMENT`), а саму версию — из времени коммита. Добавлены
   `ENV MINIO_RELEASE=RELEASE` и аргумент `"${MINIO_TAG#RELEASE.}"` — так штамп не зависит от
   метаданных клона. Остальное (`-tags kqueue`, `alpine:3.22`, non-root, `HEALTHCHECK`,
   `VOLUME /data`) — как в §2.5. Требуется правка §2.5 (ОВ-15).
2. **`MINIO_REPO` = upstream вместо форка** — предпосылка задачи не выполнена (ОВ-14).
3. **`CHROMA_IMAGE` оставлена пустой.** §2.4 требует «конкретный тег, который работает с as-is
   `semantic-memory`». As-is compose собирал semantic-memory с `GO_BUILD_TAGS=chroma_v2_enabled`,
   то есть клиентом `chroma-go/pkg/api/v2` (Chroma API v2), а без этого тега тот же сервис ходит в
   `/api/v1`. Подходящий тег зависит от того, как T-008 будет собирать legacy-образ, и проверяется
   только подъёмом профиля. Пустое значение падает громко, выдуманный тег — тихо; выбран первый
   вариант, решение за T-008 (ОВ-16).
4. **`MV_CORE_ADDR` у `gateway`.** Каркас T-003 читает адрес HTTP-сервера процесса из
   `MV_CORE_ADDR` независимо от роли (`cmd/multiverse/serve.go`), а дефолт — `127.0.0.1:8090`,
   который внутри контейнера недоступен снаружи. У `gateway` задано
   `MV_CORE_ADDR: ${MV_GATEWAY_ADDR:-:8088}` (и сама `MV_GATEWAY_ADDR`), с комментарием, что строка
   уходит вместе с переходом gateway на `MV_GATEWAY_ADDR` в EPIC-004.
5. **`.github/ci.env` без `MV_LLM_API_KEY` и `MV_ANTHROPIC_API_KEY`.** §3.2 описывает файл как
   «`.env.example` + фиктивные значения». Пустая строка `MV_LLM_API_KEY=` даёт находку gitleaks
   (правило `generic-api-key` захватывает следующую строку как значение), а непустая — это ключ в
   Git. Переменные исключены: в compose у них дефолт `${VAR:-}`, для интерполяции они не нужны.
   Причина записана комментарием в самом файле.
6. **`Makefile`.** Добавлены только `include build/versions.env`, `GIT_SHA`, цели `image` и
   `minio-image` и две строки в `help` — по составу «Файлы» T-004. As-is цели не трогались: их
   снимает T-008, который переписывает файл целиком. Рабочая копия `Makefile` была в CRLF при
   `eol=lf` в `.gitattributes` — файл записан в LF, поэтому в индексе изменений сверх моих строк
   нет (`git diff --stat` = `1 file changed, 27 insertions(+)`).

### 6. Что отложено в T-008 (F-6b) — границы соблюдены

Профили `memory`, `dev`, `bot`, `legacy`; `redpanda-init` (8 топиков, retention 30/90/180,
`segment.ms=1d`) и `minio-init` (бакеты, versioning/ILM, сервисный пользователь); env Ollama
(`OLLAMA_KEEP_ALIVE`, `OLLAMA_MAX_LOADED_MODELS`, `OLLAMA_NUM_PARALLEL`, `OLLAMA_ORIGINS`);
Neo4j и Qdrant; `build/legacy.Dockerfile` (он копирует удалённый в T-003 `go.work` — T-008
придётся его чинить); полный `Makefile` §2.2; `scripts/compose-lint.sh`; `scripts/llm-server.*` и
цели `llm-up`/`llm-down`/`llm-health`; тег Chroma и решение по жизнеспособности профиля `legacy`.
Ссылки на T-008 расставлены комментариями по месту в `docker-compose.yml`, `build/versions.env`,
`.dockerignore` и `Makefile`.

### 7. Результаты DoD

| Проверка | Результат |
|---|---|
| `make minio-image` (эквивалент `docker build -f build/minio.Dockerfile` с теми же `--build-arg`) | ✔ собирается на `golang:1.26.8-bookworm`, откат на 1.24 не понадобился |
| `docker run --rm $MINIO_IMAGE --version` | ✔ `RELEASE.2025-10-15T17-29-55Z` (после отклонения 1) |
| `mc admin info local` против собранного образа | ✔ `1 drive online, 0 drives offline` |
| `git ls-remote --tags $MINIO_REPO` по тегу | ✔ непусто (upstream; форка нет — ОВ-14) |
| `backups/minio-src-<tag>.tar.gz` существует и распаковывается | ✔ 24 257 476 байт, распаковка проверена |
| `docker compose --env-file build/versions.env --env-file .github/ci.env config -q` | ✔ без ошибок и предупреждений |
| `docker compose config --format json`: порты | ✔ все пять публикаций с `host_ip=127.0.0.1` |
| то же: образы | ✔ у всех явный тег, `latest` нет |
| то же: `extra_hosts` у `core` | ✔ `host.docker.internal=host-gateway` |
| то же: сервис `llama-server` | ✔ отсутствует |
| `build/versions.env` содержит `MINIO_REPO`, `LLAMACPP_BUILD`, `LLM_MODEL_DEFAULT` | ✔ |
| `.dockerignore` исключает `.env`, `.claude/`, `Docs/`, `services/_archive/`, `*.exe`, `*.log` | ✔ (`services/` целиком, архив внутри) |
| сборка образа платформы с новым `.dockerignore` | ✔ `docker build -f build/Dockerfile` проходит, `multiverse version` печатает штамп |
| `go build ./...`, `go vet ./...` | ✔ (Go-кода задача не добавляет; проверка «ничего не сломано») |
| `go test -short -count=1 ./...` | ✔ 6 пакетов `ok`, 4 `[no test files]` |
| `pre-commit run --files <мои файлы>` | ✔ все хуки Passed |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| `git grep -i minioadmin` по файлам задачи | ✔ пусто (упоминание убрано даже из комментария compose) |
| Go-тесты на новое поведение | n/a — Go-кода в задаче нет; поведение проверяется командами выше и `scripts/compose-lint.sh` в T-008 |
| `-race`, `make lint`, `make ci` | не запускались: `make` на машине нет (цели проверены чтением), `-race` без gcc недоступен (ОВ-5 из T-003) |

### 8. Открытые вопросы

- **ОВ-14 (владелец / tech-lead#1).** Форка `minio/minio` в аккаунте `alekseizabelin1985-spec`
  **нет**: `git ls-remote https://github.com/alekseizabelin1985-spec/minio.git` отвечает
  `Repository not found`. Предпосылка задачи (форк делает владелец до сборки образа) не выполнена,
  поэтому `MINIO_REPO` = upstream `https://github.com/minio/minio.git`: первая линия страховки U-10
  сейчас отсутствует, работает только вторая (тарбол в `backups/`). После создания форка меняется
  одна строка в `build/versions.env`; причина уже стоит там комментарием.
- **ОВ-15 (devops-engineer).** Внести в `infrastructure.md` §2.5 `ENV MINIO_RELEASE=RELEASE` и
  аргумент версии у `gen-ldflags.go` — без них критерий «`minio --version` печатает `RELEASE....`»
  недостижим (отклонение 1).
- **ОВ-16 (T-008 / devops-engineer).** Тег `CHROMA_IMAGE` и вместе с ним способ сборки as-is
  `semantic-memory` (с `chroma_v2_enabled` — Chroma API v2, без него — `/api/v1`). Решение
  принимается при подъёме профиля `legacy`; до этого переменная пуста намеренно.
- **ОВ-13 (продолжение, из T-003).** Dependabot экосистемы `docker` в каталоге `build/`: теперь там
  два Dockerfile с пинами — digest distroless (`build/Dockerfile`) и
  `MINIO_BUILDER_IMAGE`/`alpine:3.22` (`build/minio.Dockerfile`), а пины из `build/versions.env`
  Dependabot не видит вовсе. При F-7 (T-012) проверить, что правило покрывает оба файла, а сверка
  `versions.env` с реестрами остаётся ручной (§2.4).

### 9. Риски и допущения

- Сборка MinIO зависит от доступности архивированного `minio/minio` на GitHub. Проверено сегодня;
  страховка — тарбол в `backups/` (вне git: при переносе на другую машину его надо копировать
  отдельно) и, после ОВ-14, форк владельца.
- Образ MinIO собирается локально и нигде не публикуется: на чистой машине `make up` до
  `make minio-image` упадёт с «image not found». В compose у сервиса есть секция `build`, поэтому
  `docker compose build minio` тоже сработает; порядок команд закрепляют `make up` в T-008 и
  README в F-9.
- `ollama` оставлен без переменных окружения: образ сам слушает свой интерфейс контейнера, а
  литерала `OLLAMA_HOST=0.0.0.0` в файле нет (правило 5 `compose-lint`). Когда T-008 добавит
  `OLLAMA_*`, важно этот литерал не внести.
- `.dockerignore` исключает `services/` целиком. Для профиля `legacy` (T-008) это значит, что его
  образы придётся собирать со своим контекстом либо с точечными исключениями `!` — предупреждение
  оставлено комментарием в `.dockerignore`.
- Значения `${VAR:-...}` в compose дублируют дефолты `.env.example`. Расхождение поймает
  `mvctl env check` только по `.env.example`, но не по compose; при изменении дефолта в §4.2 надо
  править оба места, пока T-008 не добавит соответствующую проверку в `scripts/compose-lint.sh`.
- Локальный `.env` на машине владельца остался as-is (`KAFKA_BROKERS`, `MINIO_ACCESS_KEY` без
  префикса) и целевых переменных не содержит, поэтому
  `COMPOSE_ENV_FILES=.env,build/versions.env docker compose config -q` сейчас честно падает на
  `required variable MINIO_ROOT_USER is missing a value` — это ожидаемое fail-closed поведение, а не
  дефект compose (проверка DoD идёт через `.github/ci.env`). Перед первым `make up` владельцу нужно
  пересобрать `.env` из `.env.example` (§9.1, README в F-9); шаг стоит внести в DoD T-008 «стенд».

---

## developer#1 · T-005 · F-4a «`shared/eventbus`: конверт `meta`, `Bus`, `Journal`, `Dedup`, DLQ» · 2026-09-09

Ветка `epic/EPIC-001-foundation`, база `b7df900` (T-003). Основание: `contracts.md` v0.4 C-01 v1.1 и
§0; `components/foundation.md` v0.2 §5.1–§5.3, §6; ADR-007 (п. 1–6 и дополнение п. 2–5);
ADR-010; `analysis/api-contracts.md` v0.2.1 §2.0–§2.2; `tasks.md` §1 и раздел T-005.
Параллельно developer#2 выполнял T-004 (`build/**`, `docker-compose.yml`, `Makefile`,
`.dockerignore`, `.github/ci.env`) — эти файлы не трогались.
Коммит не выполнялся (`git.commits: ask`); изменения подготовлены в индексе.

### 1. Что реализовано

**Конверт и конструкторы** (`types.go`):

- `Meta{SchemaVersion, CorrelationID, CausationID, CausationType, ActorKind, Agent *AgentRef,
  Replay, Locale, GMPath}`, `AgentRef{ID, Level, Blueprint, BlueprintVersion}`, поле `Meta` в `Event`.
- Константы `ActorHuman|ActorCI|ActorSim|ActorSystem`, `GMPathAgent|GMPathLegacy`,
  `DefaultLocale = "ru"`, `GlobalKey = "global"`.
- `NewRoot(typ, source, worldID, scope, actorKind, payload, opts…)` — `correlation_id = id`,
  причины нет, `timestamp` из часов, `schema_version` из реестра (по умолчанию 1).
- `Derive(parent, typ, source, payload, opts…)` — наследует `World`, `Scope` (копией, не ссылкой),
  `CorrelationID`, `ActorKind`, `Replay`, `Locale`, `GMPath`; ставит `CausationID = parent.ID`,
  `CausationType = parent.Type`; **`Timestamp` наследуется всегда** (NFR-061).
- Опции: `WithAgent`, `WithScope`, `WithWorld`, `WithRelations`, `WithGMPath`, `WithReplay`,
  `WithTimestamp` (`Deprecated`).
- `Event.CorrelationID()`, `Event.Key()` (ключ сообщения `world.entity.id | global`),
  `Event.Path()`, `GetWorldIDFromEvent`, `GetScopeFromEvent`, `GetEntityIDWithFallback`,
  `GetTargetEntityID` — сохранены; методы `Event` приведены к value-receiver.
- `NewEvent`/`NewEventWithDescription`/`NewStructuredEvent` помечены `Deprecated` и переведены на
  общие источники id и часов; `Publish*`-хелперы (`PublishEntityCreated`, `PublishEntityUpdated`,
  `PublishActionEvent`) удалены (`foundation.md` §5.1).

**Источники** (`sources.go`): `SetIDSource`, `SetClock`, `SetRegistry`, `PackageRegistry`,
`SequenceIDs(prefix)` — пакетные переменные под `sync.RWMutex`, ставит процесс (`cmd/multiverse`).

**Реестр и политики** (`registry.go`): интерфейс `Registry{Lookup(type) (TypeSpec, bool);
Validate(Event) error}`, `TypeSpec{Topic, SchemaVersion, Policy, Deprecated}`,
`Policy{ActorKinds, Agent AgentRule}` с `AgentOptional|AgentRequired|AgentForbidden`,
готовые `PlayerEventsPolicy()` и `SwarmPolicy()`, `Policy.Check`, `Event.ValidateEnvelope`,
`Route(reg, ev) (topic, error)`; ошибки `ErrUnknownType`, `ErrInvalidEnvelope`,
`ErrPolicyViolation`, `ErrNoRegistry`, `ErrClosed`.

**Интерфейсы** (`bus.go`): `Handler`, `Middleware`, `Chain`, `Bus{Publish, Subscribe, Close}`,
`Journal{ReadRange, Tail, End}`, `Position{Topic, Offset}`, `ContextWithPosition`,
`PositionFromContext` — имена ровно по C-01.

**Доставка** (`delivery.go`): `Delivery{Consumer, Registry, DLQ, ValidateOnRead, Backoff, Timers,
Log}` с `Deliver` и `DeliverRaw`; `DeadLetter{Original, Error, Consumer, Attempts, FailedAt, Raw}`,
`DeadLetterSink`, `DeadLetterFunc`, `DefaultBackoff = {100 мс, 500 мс, 2 с}`.

**Дедуп** (`dedup.go`): `Dedup` — LRU по `event.id`, `NewDedup(capacity)`,
`DefaultDedupCapacity = 10 000`, `Seen`, `Len`, `Capacity`, `IDs`, `Restore` (сериализация в снапшот
потребителя, C-14).

**Kafka** (`kafka.go`): `NewKafka(KafkaConfig{Brokers, Registry, ValidateOnRead, Backoff, Timers,
Log})`, реализует `Bus` и `Journal`. Writer на топик лениво, `RequiredAcks=all`,
`Balancer=Hash`, `AllowAutoTopicCreation=false`; reader подписки — `MinBytes=1`, `MaxBytes=10 МиБ`,
`MaxWait=100 мс`, `CommitInterval=0`, `StartOffset=FirstOffset`, коммит после обработки;
журнал — reader без `GroupID` на партиции 0 с `SetOffset`, `End` — `Conn.ReadLastOffset()`
(high watermark); DLQ пишется в `dead_letters` напрямую (обёртка — не зарегистрированный тип).

**Топики** (`topics.go`): восемь констант MVP-1; `scope_management` удалён, добавлены
`llm_records`, `analytics_events`, `dead_letters`; префиксные константы `Type*` удалены.

**Прочее**: `shared/runtime.Deps` дополнен полями `Bus eventbus.Bus` и `Journal eventbus.Journal`
(это было заложено комментарием в T-003); `README.md` пакета переписан по факту; пример
`examples/universal_paths_example.go` переведён на `NewRoot`.

### 2. Принятые решения

1. **`Registry` — интерфейс в `eventbus`, а не импорт `shared/contracts`.** `contracts.Validate`
   принимает `eventbus.Event`, значит реестр зависит от конверта; обратный импорт дал бы цикл.
   Шина зависит от узкого интерфейса (`Lookup` + `Validate`), реестр T-006 его реализует. Побочный
   плюс: T-005 не блокируется T-006, а тесты работают на фикстуре реестра.
2. **`MV_BUS_VALIDATE_ON_READ` читает процесс, а не пакет.** `forbidigo` запрещает `os.Getenv` вне
   `shared/{clock,runtime,env}`, а `shared/env` появится только в T-007. Флаг вынесен в поле
   `KafkaConfig.ValidateOnRead`/`Delivery.ValidateOnRead`; переменную читает `cmd/multiverse` через
   `shared/env`. Поведение по контракту не меняется, значение по умолчанию (`true`) задаёт процесс.
3. **Общий слой чтения `Delivery` вместо дублирования в каждой реализации.** Валидация при чтении,
   политика топика, ретраи и DLQ живут в одном типе; `Route` — то же для публикации. `membus`
   (T-014) обязан использовать их же — тогда contract-тест F-5t сравнивает транспорт, а не две
   разные реализации правил. Это же сделало DoD-пункты («DLQ после 3 повторов», «политики на
   фикстурах») проверяемыми unit-тестом без брокера.
4. **«Повтор ×3» = 4 вызова обработчика.** Три значения backoff (100/500/2000 мс) из ADR-007 п. 6
   трактованы как три повтора после первой попытки; `DeadLetter.Attempts = 4`. Зафиксировано
   тестом и в README, чтобы contract-тест T-014 ожидал то же число.
5. **`Deprecated`-тип проверяется только по конверту — без схемы и без политики топика.**
   `foundation.md` §6 говорит «валидируется только конверт»; legacy-издатель не заполняет `meta`
   вовсе, поэтому политика `player_events` (`actor_kind ∈ human|ci|sim`) отклоняла бы каждое
   `player.moved`. Правило одно и предсказуемое: `Deprecated ⇒ только конверт`.
6. **`Deliver` возвращает `nil` после записи в DLQ и ошибку — если DLQ недоступен.** Возврат `nil`
   разрешает коммит офсета (событие учтено), ошибка запрещает: иначе потерянное событие выглядело
   бы как обработанное. Отмена контекста во время ретраев возвращает `ctx.Err()` — событие не
   паркуется, а будет доставлено повторно; сама запись в DLQ идёт с `context.WithoutCancel`.
7. **`WithRelations` как опция конструктора вместо as-is обёртки.** Старая
   `WithRelations(ev, rels) EventWithRelations` конфликтовала по имени с опцией из
   `foundation.md` §5.2 и дублировала поле `Event.Relations`; `EventWithRelations` и `AddRelation`
   удалены вместе с их тестами (в модуле не использовались).
8. **Паника в обработчике не перехватывается** (NFR-012) — записано комментарием к `Handler`.

### 3. Снятие временного исключения линтера (ОВ-7)

Из `.golangci.yml` удалён блок исключения `shared/eventbus/` (`errcheck`, `forbidigo`). Источники
запрещённых вызовов убраны, а не подавлены: `time.Now` заменён на `shared/clock` через
`eventbus.SetClock` (`nowUTC()`), `os.Getenv` (`KAFKA_POLL_FREQUENCY_MS`) и `log.Printf` ушли вместе
с переписанным `eventbus.go`, непроверенные ошибки закрытия — через `errors.Join` и
`defer func() { _ = … }()`. `golangci-lint run ./...` — `0 issues`.

### 4. Что осталось заглушкой и что делают следующие задачи

- `kafka.go` покрыт unit-тестами только на путях, не доходящих до сети (конфигурация, отказ до
  отправки, `Close`): без брокера остальное не проверить. Полное покрытие — contract-тест T-014
  (`membus` + testcontainers) и `-tags integration`.
- Реестра типов ещё нет: `Route` и `Delivery` работают с любым `Registry`, в тестах — фикстура
  `fakeRegistry` на четырёх представительных типах. Боевой реестр и схемы — T-006/T-009.
- `membus` — T-014; он обязан переиспользовать `Route` и `Delivery`.
- `shared/jsonpath` перенесён в модуль ещё в T-003, правок не требовал (пункт «перенос как есть»
  раздела «Файлы» T-005 закрыт).
- `MIGRATION.md`, `docs/` и `examples/` пакета оставлены на месте: перенос в `Docs/archive/`
  выполняет tech-writer в T-019 (владелец каталога по `Docs/archive/README.md`).

### 5. Результаты DoD

| Проверка | Результат |
|---|---|
| `go build ./...` | ✔ |
| `go vet ./...` | ✔ |
| `go test -short -count=1 ./...` | ✔ все пакеты `ok` |
| `golangci-lint run ./...` (v2.13.2) | ✔ `0 issues` (после снятия исключения `shared/eventbus`) |
| `gofmt -l shared cmd` | ✔ пусто |
| `go mod tidy` | ✔ `go.mod`/`go.sum` без диффа; `go mod verify` — `all modules verified` |
| Покрытие `shared/eventbus` | ✔ **69,7 %** (порог 60 %); по файлам: `registry.go` 100 %, `bus.go` 100 %, `sources.go` 98 %, `dedup.go` 95 %, `delivery.go` 73 %, `types.go` 67 %, `kafka.go` 24 % (нужен брокер) |
| `pre-commit run --files <изменённые>` | ✔ |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| `-race` | локально не запускался (нет gcc, ОВ-5); тесты написаны под `-race`: общее состояние только в `Dedup` (мьютекс) и в источниках пакета (`sync.RWMutex`), тесты источников не параллельные, `TestDedupIsSafeForConcurrentConsumers` гоняет 8 горутин |

Unit-тесты по DoD T-005: `NewRoot`/`Derive` (наследование `CorrelationID`/`Timestamp`, новый
`event_id`, копии `World`/`Scope`, детерминизм байтов), `Dedup` (повтор, вытеснение, снапшот,
конкурентность), политики топиков на фикстурах (`player_events` отклоняет `actor_kind=system` и
`meta.agent`; топики роя требуют `meta.agent`), DLQ после 3 повторов (`Attempts=4`), валидация при
чтении (невалидное → DLQ без вызова handler), `PositionFromContext`, `ReadRange` на пустом
интервале. Монотонность `End` и строгий порядок `ReadRange` полностью проверяются в T-014.

### 6. Открытые вопросы

- **ОВ-14 (system-architect / T-006).** Имя метода реестра: C-01 и `foundation.md` §6 описывают
  `contracts.Lookup(type) (Spec, bool)` со «своим» `Spec`, а шина требует
  `Lookup(type) (eventbus.TypeSpec, bool)`. Либо `contracts.Registry` реализует обе сигнатуры под
  разными именами (например, `Lookup` для `Spec` и отдельный адаптер для шины), либо `Spec`
  встраивает `eventbus.TypeSpec`. Предпочтителен второй вариант — тогда `Spec.Policy` и
  `Spec.Topic` не дублируются. Решение нужно до старта T-006.
- **ОВ-15 (system-architect).** `Spec.Publishers` (C-01/§0 v0.4) проверяется job'ом `contracts`,
  а не библиотекой: `Route` сейчас `source ∈ Publishers` не проверяет. Подтвердить, что проверка
  остаётся статической (T-010 `mvctl contracts check`), или добавить её в `Route` в T-006.
- **ОВ-16 (tech-lead#1 / T-019).** `shared/eventbus/{MIGRATION.md,docs/,examples/}` — кандидаты в
  `Docs/archive/`; `docs/event-model.pdf` уже в списке отложенных решений T-001 (U-11).
- **ОВ-17 (tech-lead#1 / ревью).** `runtime.Deps` дополнен `Bus`/`Journal` из T-005 — файл общий
  (`shared/runtime`), правка минимальная и предусмотрена комментарием, оставленным в T-003.
  Отметить, что она сделана вне карты файлов T-005.

### 7. Риски и допущения

- `End` открывает соединение к лидеру партиции 0 (`kafka.DialLeader`) на каждый вызов: в MVP-1
  партиция одна и вызовов немного (выход из `replay` в `live`), но при частом опросе это заметно.
  Если T-014 покажет накладные расходы — кешировать соединение.
- `Journal` жёстко читает партицию 0. Это соответствует «одна партиция на топик» (ADR-007 п. 5);
  при росте числа партиций интерфейс `Journal` придётся расширять — это изменение C-01.
- Пакетные `SetIDSource`/`SetClock`/`SetRegistry` — общее состояние процесса. Ставятся один раз при
  старте; переустановка на живом процессе не предусмотрена и не тестируется.
- `Delivery` без реестра проверяет только конверт. Боевой процесс всегда строит шину с реестром
  (`NewKafka` падает без него), но фикстуры такой режим используют — при переносе кода в
  production-путь это допущение надо снимать.
- `DeadLetter.Raw` (тело недекодируемого сообщения) не входит в формулировку C-01
  (`{original, error, consumer}`) — добавлено как расширение, иначе битое сообщение теряется
  целиком. Поле `omitempty`, обратной несовместимости нет.

---

## developer#1 · T-005 · итерация 2 (исправления по ревью #1) · 2026-09-09

Вход: `review.md` «T-005 · ревью #1 · code-reviewer#2», решения оркестратора в `journal.md`
(запись «Решения оркестратора по ревью T-005»). Исправлены M-1, M-2, M-3, Mi-1, Mi-2, Mi-3, Mi-6.
Mi-4, Mi-5, N-1…N-4 по решению оркестратора уходят в T-014/T-010/T-019 и здесь не трогались.
Файлы T-004 (`build/**`, `docker-compose.yml`, `Makefile`, `.dockerignore`, `.github/ci.env`,
`services/_archive/build/**`) не изменялись.

### 1. M-1 — writer больше не ждёт батч (`kafka.go`)

Рядом с `kafkaMaxWait` заведены `kafkaBatchSize = 1` и `kafkaBatchTimeout = 10 * time.Millisecond`,
оба выставляются в `k.writer()`. Обоснование в комментарии к константам: kafka-go закрывает батч
по заполнению (`BatchSize`, умолчание 100) или по таймеру (`BatchTimeout`, умолчание 1 с), а
синхронный `WriteMessages` ждёт закрытия батча; MVP-1 публикует по одному событию в топик из одной
партиции, батч не заполняется никогда — значит каждая публикация платила бы секунду при бюджете
подтверждения NFR-001 ≤ 300 мс. `BatchTimeout` оставлен как вторая страховка на случай, когда
kafka-go всё же сгруппирует записи.

Тест — на конфигурации, не на брокере: `TestWriterSendsEachEventWithoutWaitingForABatch` проверяет
`BatchSize == 1`, `BatchTimeout == kafkaBatchTimeout` (и что он не больше 100 мс), `Async == false`
и переиспользование writer'а по топику. **Фактическую задержку публикации одиночного события нужно
внести в DoD T-014** (contract-тест на testcontainers) — без брокера конфигурация проверяема, а
время нет; это же значится предложением №1 в бэклог из ревью.

### 2. M-2 — валидация при чтении стала fail-closed (`kafka.go`, `delivery.go`)

`ValidateOnRead bool` инвертирован в `SkipValidateOnRead bool` в обеих структурах
(`KafkaConfig`, `Delivery`), внутреннее поле `Kafka.validateOnRead` → `skipValidateOnRead`,
условие в `Delivery.Deliver` — `if !d.SkipValidateOnRead`. Нулевое значение теперь означает
«валидировать»: тот, кто соберёт конфигурацию, не зная про флаг (membus T-014, харнесс T-018,
`mvctl` T-010), получит защиту SEC-16, а не её отсутствие. Умолчание больше не зависит от чужой
задачи — это вариант (а) из замечания.

Процесс по-прежнему читает `MV_BUS_VALIDATE_ON_READ` сам (запрет `os.Getenv` в библиотеке) и
передаёт **инверсию**: `SkipValidateOnRead = !MV_BUS_VALIDATE_ON_READ`. Это записано в
`README.md` (пример `NewKafka` и отдельный пункт в «Гарантиях и правилах») и в комментариях к
обоим полям.

Тесты: `testDelivery` больше не выставляет флаг вовсе — все существующие проверки валидации при
чтении теперь идут через умолчание; добавлен `TestZeroDeliveryValidatesOnRead` (нулевая `Delivery`
паркует незарегистрированный тип и не зовёт обработчик); `TestDeliverSkipsValidationWhenItIsOff`
переведён на `SkipValidateOnRead = true`.

### 3. M-3 — `meta.gm_path` обязателен (`types.go`, `registry.go`)

- `NewRoot` ставит `GMPath: GMPathAgent`. Переопределение — существующей опцией
  `WithGMPath(GMPathLegacy)`, новых механизмов не заводилось.
- Тег поля `gm_path` потерял `omitempty`: поле обязательное по §2.1, и конверт, собранный мимо
  конструкторов, должен падать на валидации, а не уходить на шину без required-поля схемы T-006.
- `ValidateEnvelope` требует `gm_path` из перечисления `{agent, legacy}` для не-legacy конверта
  (проверка переведена из «если не пусто — из перечисления» в `switch` без пустого случая). Конверт
  вовсе без `meta` (`Meta{}`, профиль `legacy`) по-прежнему выходит раньше и до этой проверки не
  доходит — поведение legacy не менялось.

Тесты: `TestNewRootDefaultsToTheAgentGMPath` (умолчание, переопределение опцией, наследование
`Derive`), `TestGMPathIsAlwaysOnTheWire` (поле есть в JSON), новый случай `"no gm path"` в
`TestValidateEnvelopeRejectsHalfFilledMeta`.

**Решение по `Derive`**: умолчание в `Derive` НЕ добавлялось. `Derive` наследует `gm_path` причины,
как требует C-01; если причина собрана мимо конструкторов и поля не имеет, вся цепочка отвергается
валидацией — это fail-closed и соответствует духу M-2. (У `Locale` в `Derive` умолчание есть
исторически; трогать его вне задачи не стал.)

### 4. Mi-1 — штатная остановка возвращает `nil` (`kafka.go`, `bus.go`)

Условие «это остановка, а не отказ» вынесено из `readerError` в функцию `stopped(ctx, err)`
(отмена контекста, `io.EOF`, `io.ErrClosedPipe`), через неё теперь проходят все три выхода из
циклов: `FetchMessage` (как и раньше, внутри `readerError`), возврат `deliver` и
`reader.CommitMessages`. Единообразно поправлены `Subscribe`, `Tail` и `ReadRange` (последняя при
отмене отдаёт достигнутый `next` и `nil`). Комментарии `Bus.Subscribe`, `Kafka.Subscribe` и `Tail`
дополнены фразой, что штатное завершение по `ctx` — это `nil`, с указанием на единую семантику с
membus.

Тесты: `TestSubscribeReturnsNilWhenTheCallerStopsIt` (отменённый контекст → `nil` у `Subscribe` и
`Tail`, брокер не нужен) и табличный `TestStoppedTellsAShutdownFromAFailure`.

### 5. Mi-2 — `Option` → `DeriveOption` (`types.go`)

Тип переименован по C-01 без алиаса: пакет вне себя ещё нигде не используется (проверено grep по
`eventbus.Option`), поэтому оставлять два имени незачем. Сигнатуры `NewRoot`, `Derive`, всех
`With*` и `applyOptions` обновлены, `README.md` — тоже.

### 6. Mi-3 — размер dead letter ограничен (`delivery.go`)

- Константа `MaxDeadLetterRaw = 512 << 10` (512 КиБ) с обоснованием: reader принимает до 10 МиБ,
  но запись в `dead_letters` больше 1 МиБ (`max.message.bytes`) не пройдёт, `Deliver` вернёт
  ошибку, офсет не закоммитится — и топик встанет навсегда на том самом сообщении, ради которого
  DLQ и заводился.
- `Delivery.deadLetter` пропускает тело через `truncateRaw`; в конверт добавлено поле
  `raw_truncated bool` (`omitempty`) — пометка об усечении для читателя `dead_letters`.
- **`DeadLetter.Raw` сменил тип `json.RawMessage` на `[]byte`.** Это не косметика: усечённое (да и
  любое недекодируемое) тело — невалидный JSON, а `json.Marshal` от `json.RawMessage` с невалидным
  содержимым возвращает ошибку. То есть до этой правки `kafkaDeadLetters.WriteDeadLetter` не смог
  бы сериализовать ни одну запись из `DeliverRaw`. С `[]byte` тело уезжает base64: 512 КиБ даёт
  около 683 КиБ, с запасом под 1 МиБ вместе с конвертом. Поле по-прежнему `omitempty`, обратной
  несовместимости wire-формата нет (валидного JSON там и не бывало).

Тесты: `TestDeliverRawTruncatesAnOversizedBody` (длина ровно `MaxDeadLetterRaw`, флаг выставлен,
запись сериализуется), `TestDeliverRawKeepsASmallBodyWhole` (флаг не выставлен на коротком теле).

### 7. Mi-6 — `forbidigo` расширен (`.golangci.yml`)

Добавлен шаблон на `time.(After|Tick|NewTimer|NewTicker|Since|Until)`; `os.(Environ|ExpandEnv)`
дописаны в существующий шаблон `os.(Getenv|LookupEnv)`. Текст исключения владельцев времени и
окружения `shared/(clock|runtime|env)/` расширен теми же именами, иначе `shared/clock` сам себя
нарушает. Правило действует на `internal/**`, `shared/**` и `cmd/**`; временные исключения
`shared/agent` и `shared/entity` (снимаются в EPIC-003 и T-011) уже покрывают своих нарушителей.

В `shared/eventbus` правило ничего не нашло — таймеры пакета идут через `clock.Timers`, времени и
окружения он не читает. Действенность правил проверена контрольным файлом-зондом (`time.Since`,
`time.NewTicker`, `os.Environ`, `os.ExpandEnv` в `shared/eventbus` дали 4 находки forbidigo), зонд
удалён.

### 8. Результаты DoD итерации

| Проверка | Результат |
|---|---|
| `go build ./...` | ✔ |
| `go vet ./...` | ✔ |
| `go test -short -count=1 ./...` | ✔ 10 пакетов `ok` |
| `go test -cover ./shared/eventbus/` | ✔ **76,1 %** (было 69,7 %) |
| `golangci-lint run ./...` (v2.13.2) | ✔ `0 issues` |
| `gofmt -l shared cmd` | ✔ пусто |
| `pre-commit run --files <изменённые>` | ✔ |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| `-race` | локально недоступен (нет gcc, ОВ-5) — в CI, T-012 |

### 9. Открытые вопросы

- **ОВ (tech-lead#1 / T-014).** В DoD T-014 внести проверку задержки публикации одиночного события
  (M-1) — конфигурацию writer'а тест без брокера проверяет, время нет.
- **ОВ (tech-lead#1 / T-007, T-010, T-018).** Потребители `MV_BUS_VALIDATE_ON_READ` должны
  передавать в конфигурацию **инверсию** значения; про поле `SkipValidateOnRead` упомянуть в DoD
  задач, которые строят шину.
- **ОВ (system-architect / T-006).** `_envelope.json` должен объявить `gm_path` в `required` с
  перечислением `agent|legacy` — библиотека теперь это требует, схема обязана совпасть.

### 10. Риски и допущения

- `BatchSize: 1` отключает группировку записей: при заметном росте трафика (за пределами MVP-1)
  это станет стоить пропускной способности. Ориентир для пересмотра — измерения T-014.
- `MaxDeadLetterRaw` подобран под умолчание брокера `max.message.bytes = 1 МиБ`. Если в T-008
  умолчание топика `dead_letters` изменится, константу надо пересмотреть вместе с ним.
- `stopped` считает `io.EOF` штатной остановкой и для журнала: у kafka-go это признак закрытого
  reader'а, а не конца топика (конец журнала определяется через `End`), но допущение стоит
  перепроверить на брокере в T-014.

---

## developer#1 · T-006 · F-4b-1 «`shared/contracts` + схемы блоков «а» и «б»» · 2026-09-09

Ветка `epic/EPIC-001-foundation`, база `dfc0498` (T-005). Основание: `epics/EPIC-001-foundation/tasks.md`
§1 и T-006; `architecture/contracts.md` v0.4 §0, C-01 v1.1, C-02 v1.2, C-13, C-14, §16 п. 6–7;
`architecture/components/foundation.md` v0.2 §6; `architecture/components/state-and-mechanics.md` §4.4,
§4.6, §4.8; `analysis/api-contracts.md` v0.2.1 §2.1–§2.3; ADR-007, ADR-013; решения оркестратора
ОВ-17, ОВ-18 и требование T-005 (`gm_path` в `required`).

### 1. Что сделано

- **`schemas/embed.go`** — пакет `schemas` с `//go:embed all:events` (`schemas.FS`). Префикс `all:`
  обязателен: обычный шаблон каталога пропустил бы как раз `_common.json` и `_envelope.json`
  (имена с подчёркивания).
- **`schemas/events/_common.json`** — `$defs` по `foundation.md` §6: `EntityRef`, `EntityWithName`,
  `WorldRef`, `ScopeRef`, `AgentRef`, `Timestamp`, `Money`.
- **`schemas/events/_envelope.json`** — конверт C-01 v1.1: `id`, `type`, `timestamp`, `source`,
  `world?`, `scope?`, `meta`, `payload`, `relations?`; `meta.required` =
  `schema_version, correlation_id, actor_kind, replay, locale, gm_path`, `actor_kind` —
  `human|ci|sim|system`, **`gm_path` — `agent|legacy` и в `required`** (требование T-005),
  `additionalProperties: false` на конверте и на `meta`.
- **25 схем типов** блоков «а» и «б» (список и владельцы — §2), `additionalProperties: false` на
  верхнем уровне payload, `$ref` в `_common.json`.
- **`shared/contracts`**: `Spec` (встраивает `eventbus.TypeSpec`, плюс `Type`, `Owner`,
  `Publishers`, `Consumers`, `Since`, `Schema`), `TopicSpec`, реестр-таблица `registry.go`,
  `New(fs.FS)` / `Default()` / `Lookup` / `Spec` / `All` / `Topics` / `Validate` /
  `ValidateEnvelope`, константы источников (`SourceGateway`, `SourceState`, `SourceSwarm`,
  `SourceMvctl`, `SourceTestkit*`, `SourceLegacy`) и владельцев (`OwnerFoundation…OwnerOps`),
  статичная копия `OwnershipRules()`.
- Валидация — `github.com/santhosh-tekuri/jsonschema/v6` **v6.0.3** (пин `foundation.md` §6):
  схемы компилируются один раз (`sync.OnceValues`), событие валидируется в том виде, в котором
  уходит в шину (маршалинг → `jsonschema.UnmarshalJSON` → конверт, затем payload).

### 2. Созданные схемы и их владельцы

| Файл | Владелец схемы (`contracts.md` §0) | Ревизия |
|---|---|---|
| `_common.json`, `_envelope.json` | EPIC-001 (только через system-architect) | — |
| `player.{entered_region,left_region,looked,attacked,flee_attempted,rested,said,defended}.v1.json` | EPIC-004 | T-301 |
| `group.{created,joined,left,leader_changed,disbanded,entered_region,left_region}.v1.json` | EPIC-004 | T-301 |
| `round.{opened,closed}.v1.json` | EPIC-004 | T-301 |
| `entity.{create.proposed,update.proposed,created,updated,update.rejected}.v1.json` | EPIC-002 | T-052 |
| `snapshot.created.v1.json`, `dice.rolled.v1.json` | EPIC-002 | T-052 |
| `analytics.replay.completed.v1.json` | EPIC-002 (совладение с EPIC-005: `mode=test`, `events_hash_match`) | T-052, EPIC-005 |

Схемы блока «в» (`agent.*`, `tick.*`, `llm.*`, `narrative.output`, `world.*`, `region.*`, `npc.*`,
`encounter.*`, `content.incident.recorded`, `analytics.session.*`, `analytics.turn.completed`,
`analytics.consistency.violated`) **не создавались** — это T-009; реестр дополняется той же
таблицей `definitions` в `registry.go`.

### 3. Решения по ходу

1. **`Spec` встраивает `eventbus.TypeSpec`** (ОВ-17). Следствие: метод `(*Registry).Lookup`
   возвращает `eventbus.TypeSpec` (этого требует интерфейс `eventbus.Registry`), а весь контракт
   отдают `(*Registry).Spec(type)` и пакетная функция `contracts.Lookup(type) (Spec, bool)` — как в
   `foundation.md` §6. Цикла импортов нет: `contracts → eventbus`, обратной стрелки нет.
2. **`Publishers` — значения `source` конверта**, а не имена команд: `core/<контекст>` для
   контекстов `core`, `mvctl` для CLI, `testkit/<владелец>` для фейков (`api-contracts.md` §2.1
   даёт `core/swarm`, tasks.md T-017 — `testkit/state`). Проверка `source ∈ Publishers` остаётся
   статической (ОВ-18): в `Publish` её нет.
3. **Legacy-типы** (`player.moved`, `player.used_skill`, `gm.*`, `narrative.generate`,
   `violation.detected`, `time.syncTime`) внесены в реестр как `Deprecated: true, Schema: nil` по
   `foundation.md` §6 — иначе профиль `legacy` (T-008) не сможет опубликовать ни одного события.
   Проверка «нет фантомов» их исключает (у deprecated по определению нет файла схемы). Ни T-006, ни
   T-009 их явно не называют — в T-009 их **повторно вносить не нужно**.
4. **`AssertFormat`** включён у компилятора: `format: date-time` проверяется, иначе `applied_at`,
   `deadline_at`, `taken_at` принимали бы любую строку.
5. **`Default()` паникует**, если встроенные схемы не компилируются: это дефект сборки, а не
   рантайма; `TestSchemasValid` ловит его до выпуска бинарника.
6. **`OwnershipRules()` возвращает копию** (тест «копию нельзя переписать через возвращённый
   срез»): State читает таблицу на каждом предложении.
7. **`scope` не дублируется в payload** `round.opened` (в `api-contracts.md` §2.3.3 он указан в
   перечне полей, но §2.1 прямо запрещает дублировать `world`/`scope`); при
   `additionalProperties: false` присланный в payload `scope` будет отклонён — см. ОВ-22.

### 4. Отклонения от дизайна

- **`entity.create.proposed.proposal_id`** сделан *опциональным* полем (в §2.3.4 его нет в списке
  полей, но `entity.update.rejected reason=duplicate_entity` ссылается на `proposal_id`). Делать
  его обязательным — изменение контракта, поэтому оставлено на владельца (EPIC-002, ОВ-21).
- **`entity.created.version`** объявлен `const: 1` — буквально по C-02 (`entity.created {…,
  version: 1, …}`). Если State публикует `created` с другой версией, схема отклонит; проверить при
  реализации T-052.
- Таблица `OwnershipRules` перенесена из §4.6 **буквально**, но два условия в форму
  `OwnershipRule` не укладываются и остаются за State: `hp` у gateway только при `cause=rest`,
  `status` у gateway только `alive → abandoned` при `cause=forget`. Исключения строк `*`
  (`region` кроме `description`, `npc` кроме понижения hp) — того же рода, описаны комментарием.
  В строке `domain`/`region` поле `Create` = false: столбец «Create: npc, encounter» §4.6 покрыт
  строками `domain`/`npc` и `domain`/`encounter`, чьи `EntityTypes` эти типы и называют.

### 5. Как тестировалось

`shared/contracts`: 17 тестов, 107 подтестов. Компиляция всех схем (`TestSchemasValid`); «нет
фантомов» в обе стороны (`TestNoPhantoms`); непустые `Publishers`/`Consumers`/`Owner` у каждого
`Spec`; таблица типов с несколькими издателями (`dice.rolled`, `entity.*.proposed`,
`analytics.replay.completed`, `snapshot.created`); карта топиков (8 топиков, retention 30/90/180 дн.,
`analytics_events` не читается в replay); `Lookup` как `eventbus.Registry` + `eventbus.Route`
(политика `player_events` отклоняет событие с `meta.agent`); позитивные фикстуры — по одному
примеру `api-contracts.md` §2.3 на каждый тип с проверкой «примеров столько же, сколько типов со
схемой»; негативные фикстуры конверта — пустой и чужой `gm_path`, чужой `actor_kind`, отсутствующий
`correlation_id`, `schema_version = 0`, пустые `locale`/`source`, `agent` без `level`; негативные
фикстуры payload — опечатка в имени поля, отсутствие обязательного поля, значения вне enum
(`op`, `reason`, `purpose`, `component`, `cause`), пустой `changes`, текст > 500, `applied_at` не
дата, `entity.created` с `version = 2`; `cause = forget` в `entity.update.proposed` и
`entity.updated` (C-02 v1.2).

### 6. Результаты DoD

| Проверка | Результат |
|---|---|
| `go build ./...` | ✔ |
| `go vet ./...` | ✔ |
| `go test -short -count=1 ./...` | ✔ все пакеты `ok` |
| `go test -cover ./shared/contracts/` | ✔ **79,6 %** (порог 60 %) |
| `golangci-lint run ./...` (v2.13.2) | ✔ `0 issues` |
| `gofmt -l shared/contracts schemas` | ✔ пусто |
| `pre-commit run --files <свои>` | ✔ |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| `-race` | локально недоступен (ОВ-5) — в CI, T-012 |

### 7. Открытые вопросы

- **ОВ-20 (system-architect / T-052).** Перечень `cause` в схемах взят из `api-contracts.md` §2.3.4
  (`combat|rest|move|loot|spawn|tick|group|create|bootstrap|forget`), но столбец «Причины»
  `state-and-mechanics.md` §4.6 содержит ещё `leave`, `flee`, `death`, `resolve`, `init`, `author`.
  Публикация с ними сейчас будет отклонена схемой. Требуется решение: расширить enum C-02 или
  поправить §4.6.
- **ОВ-21 (EPIC-002 / T-052).** `proposal_id` в `entity.create.proposed` — сделать обязательным?
  Без него `entity.update.rejected reason=duplicate_entity` не на что сослаться.
- **ОВ-22 (system-analyst / T-301).** `round.opened`: убрать `scope` из перечня полей payload в
  `api-contracts.md` §2.3.3 (он в конверте) либо разрешить его в схеме.
- **ОВ-23 (tech-lead#1 / T-009).** Legacy-типы уже в реестре (решение 3) — не дублировать в T-009.
- **ОВ-24 (tech-lead#1).** `go.mod`: `jsonschema/v6` поднят `v6.0.2 → v6.0.3` (пин
  `foundation.md` §6) и переведён в блок прямых зависимостей вручную. Общий `go mod tidy` не
  запускался: в рабочем дереве одновременно шла T-007 (`minio-go`, `testcontainers`), tidy мог бы
  снести ещё не импортированные зависимости соседней задачи. Запустить после приёмки подволны 0.3.

### 8. Риски и допущения

- Тест равенства `shared/agent/levels.go ↔ contracts.OwnershipRules` (§16 п. 6) появится в EPIC-003
  T-202; до него копия проверяется только «изнутри» (`ownership_test.go`: покрытие уровней, строка
  gateway, пустые `object`/`monitor`). Расхождение до T-202 обнаружить нечем.
- `additionalProperties: false` на верхнем уровне payload ловит опечатки, но делает **любое** новое
  поле несовместимым изменением до правки схемы. Это осознанно (`foundation.md` §6), однако
  владельцам типов (EPIC-002/003/004) стоит помнить об этом при ревизии.
- `Validate` маршалит событие в JSON на каждый вызов. Замер `foundation.md` §6 (20–50 мкс) не
  проверялся — бенчмарк уместен в T-014 вместе с contract-тестом шины.

---

## developer#2 · T-007 · F-5 «`shared/env`, `shared/objstore`, `shared/logging`» · 2026-09-09

### 1. Что сделано

Три пакета из описания T-007. Задача в `tasks.md` §4 включает и `shared/logging` — он сделан
здесь же, хотя в постановке оркестратора была названа только область `shared/env` + `shared/objstore`
(отмечено в отчёте).

**`shared/env`** — единственный читатель окружения платформы.

- `env.go`: тип `Var` (имя, дефолт, описание, `secret`, `required`, `scope`, `external`, enum,
  условная обязательность), опции `Secret()`, `Required()`, `RequiredWhen(name, values…)`,
  `Tooling()`, `OneOf(…)`; `Declare` (паникует на имени без `MV_`, на пустом описании, на дубле и
  на «required с дефолтом»), `DeclareExternal` (паникует на имени с `MV_`), `DeclareDeprecated`;
  `Manifest()`, `Lookup()`, `DeprecatedNames()`; чтение `String/List/Int/Bool/Duration/Set` и их
  `…From(Source)`-варианты; `Validate(Source)` и `Deprecated(Source)`; тип ошибки `*env.Error`.
- `vars.go` — манифест платформенных `MV_*` по `infrastructure.md` v0.3 §4.2 (55 переменных):
  compose (`MV_IMAGE_TAG`), общие (`MV_ENV/LOG_LEVEL/LOG_FORMAT/MODE/BUS/BUS_VALIDATE_ON_READ/
  KAFKA_BROKERS/MINIO_*/WORLD_ID/BACKUP_AGE_RECIPIENT`), gateway (5), core (6, включая
  `MV_CORE_ADMIN_CLIENTS` с умолчанием `operator`), llm (9 + 11 со `scope=tooling` для
  `scripts/llm-server.*`), `MV_OLLAMA_URL`, memory (6), telegram-bot (5).
  `MV_OPENAI_API_KEY` и `MV_DEEPSEEK_API_KEY` объявлены через `DeclareDeprecated`.
- `infra.go` — сторонние переменные без префикса (`COMPOSE_*`, `MINIO_ROOT_*`, `NEO4J_PASSWORD`,
  `OLLAMA_*`), все со `scope=tooling`; блок `OLLAMA_*` и `MV_OLLAMA_URL` —
  `RequiredWhen(MV_LLM_PROVIDER, ollama)`.
- `example.go` — разбор dotenv (`ParseExample`) и сверка манифеста с `.env.example` в обе стороны
  (`CheckExample`, `CheckExampleFile`): (а) всё объявленное есть в файле, (б) всё из файла
  объявлено, (в) секреты пусты, (г) обязательные без дефолта пусты, (д) устаревших нет,
  (е) значение внутри enum, плюс дубль присваивания. Секция `legacy` исключена (сравнение по
  первому слову заголовка `# ===== legacy (…) =====`).

**`shared/objstore`** по ADR-021 п. 2.

- `objstore.go` — интерфейс `Client{Put,Get,Stat,List,Delete,EnsureBucket,Capabilities}`, типы
  `ObjectInfo`, `PutOptions{ContentType}`, `BucketOptions{Versioned,NoncurrentExpireDays,ExpireDays}`,
  `Capabilities{Versioning,Lifecycle}`, `ErrNotFound`.
- `buckets.go` — `EntitiesBucket/SnapshotsBucket/PromptsBucket`, `OpsArtifacts`, единая таблица
  `BucketOptionsFor` (entities/snapshots — versioning + noncurrent 30 дн.; prompts — без
  versioning, expire 30 дн.; остальное — без правил).
- `memory.go` — `Memory` (`NewMemory`, `NewMemoryWithClock`) с копированием тела на запись и чтение,
  `ErrNoBucket` (как у сервера), `BucketOptionsOf` для проверок в тестах; `Capabilities{}` пустые.
- `minio.go` — реализация на `minio-go/v7 v7.3.0`; `Config`/`ConfigFromEnv`; `EnsureBucket`
  создаёт бакет и **сам** включает versioning и ILM (`lifecycle.Configuration`), идемпотентно;
  `translate` переводит ответы сервера в `ErrNotFound`/`ErrNoBucket`; `retry` — 3 попытки с паузами
  100 мс / 500 мс только на транзиентных ошибках (сеть, 429, 5xx), паузы — через `clock.Timers`.

**`shared/logging`** по `infrastructure.md` §7.1.

- `New(Options)`/`Init(service, level)` — slog JSON (или text) с полями `service`, `version`;
  `LevelFromEnv`/`FormatFromEnv` из манифеста.
- `Fields` в контексте (`ContextWith`, `WithContextName`, `FieldsFrom`, `EventFields`) и
  `With(ctx, l)` — поля `context`, `correlation_id`, `event_id`, `event_type`, `agent.level`;
  пустые не логируются.
- `BusMiddleware(l)` кладёт поля события в контекст и пишет отказ обработчика с `handled=false`;
  `LogDeadLetter` — `handled=true` после DLQ.
- Редакция: значения ключей `external_id`, `token`, `authorization`, `api_key`, `text`, `username`,
  `password`, `secret`, `bot_token` не выводятся вообще; каждая строка, включая `msg` и текст
  ошибки, проходит `Redact` (шаблоны токена Telegram и ключа `sk-…`).

**Подключение к манифесту.** `shared/runtime/http.go` больше не читает окружение сам: `AdminOnly`
берёт `env.CoreAdminClients.List()`, `EnvAddr`/`envOr` удалены (их назначение «до F-5» было записано
в комментарии T-003). `cmd/multiverse` использует `env.CoreAddr.String()` в `serve` и `health`.
В `.golangci.yml` исключение `forbidigo` сужено до `shared/(clock|env)/`.

### 2. Решения по ходу

1. **Манифест централизован в `shared/env/vars.go`**, а не объявляется по месту в пакетах-читателях
   (как в `foundation.md` §8). Причина: `mvctl env check` (T-010) и job `contracts` (T-012) должны
   видеть весь состав независимо от того, что импортирует конкретный бинарник; объявление внутри
   `internal/gateway` для `mvctl` невидимо, и сверка прошла бы «зелёной», не увидев половины
   переменных. Читатель обращается к своей переменной по имени из `vars.go`, `os.Getenv` по-прежнему
   запрещён линтером.
2. **Три уровня обязательности вместо двух.** `Required()` — безусловная, проверяется только у
   `scope=process`; `RequiredWhen(...)` — условная, действует независимо от scope (это правило
   развёртывания, а не процесса); `scope=tooling` снимает только безусловную. Иначе либо
   `MINIO_ROOT_PASSWORD` валила бы старт контейнера core, либо блок `OLLAMA_*` не проверялся бы при
   `MV_LLM_PROVIDER=ollama` (требование DoD).
3. **`MV_MINIO_ACCESS_KEY`/`MV_MINIO_SECRET_KEY` — единственные безусловно обязательные** в
   манифесте (SEC-14: процесс без ключей до хранилища не доберётся). `env.Validate` при старте
   процесса **не вызывается** — это сломало бы `go run ./cmd/multiverse` и e2e «пустой мир» (T-012);
   вызов принадлежит владельцу контекста, открывающего хранилище (EPIC-002), и `mvctl env check`
   (ОВ-27).
4. **`Memory` возвращает `ErrNoBucket`** на операции в несозданном бакете — как MinIO. Unit-тест на
   `Memory` не должен проходить там, где integration против сервера падает.
5. **Ретраи `objstore`** не применяются к `List`: это поток, и повтор на середине выдал бы уже
   отданную часть заново. Отдельно: `wait` проверяет `ctx.Err()` до постановки таймера — иначе
   `select` по двум готовым каналам выбирает случайно и отмена контекста не останавливает ретраи.
6. **Образ для integration-теста** читается из `build/versions.env` (`MINIO_IMAGE`) тем же
   `env.ParseExample`, а не из окружения: файл — источник истины (NFR-071), а `os.Getenv` в
   `shared/**` запрещён.
7. **`.env.example` не менялся** — сверка манифеста с текущим файлом (T-001 + T-003) проходит без
   правок.

### 3. Отклонения от дизайна

- `foundation.md` §8 описывает `env.Load() Env` и `Env.String(name, def)`. Реализовано иначе:
  геттеры — методы `Var` (`brokers.String()`), дефолт живёт в объявлении, а не в точке чтения (иначе
  один и тот же ключ мог бы читаться с разными дефолтами и сверка с `.env.example` теряет смысл).
  Роль `Load()` играет `env.Source` — `OS()` или `MapSource(...)`, что позволяет `mvctl env check`
  валидировать разобранный файл, не подменяя окружение процесса.
- `foundation.md` §7 и `design.md` §8 называют переменную SSL хранилища `MV_MINIO_SECURE`; в
  `infrastructure.md` v0.3 §4.2 и в `.env.example` — `MV_MINIO_USE_SSL`. Взято имя из
  `infrastructure.md` (источник истины по указанию оркестратора) — ОВ-25.
- `design.md` §8 упоминает `MV_CONTEXTS` и `MV_ID_SOURCE`; в §4.2 их нет, а в `cmd/multiverse` это
  флаги `--contexts`/`--id-source`. В манифест не внесены — ОВ-26.
- `shared/logging` реализован в этой задаче (он есть в `tasks.md` T-007), хотя в постановке
  оркестратора область была ограничена `shared/env` + `shared/objstore`.

### 4. Как тестировалось

- `shared/env` — 94,6 %: панические проверки `Declare`/`DeclareExternal`/`DeclareDeprecated`,
  сортировка манифеста, дефолты и обрезка пробелов, типизированные геттеры и их ошибки, «значение
  секрета не попадает в ошибку», `Validate` (безусловная и условная обязательность, enum, tooling),
  устаревшие имена; разбор всех форм dotenv (инлайн-комментарий, кавычки, закомментированная
  строка, заголовок секции), сверка в обе стороны на синтетическом манифесте, **сверка реального
  `.env.example` с реальным манифестом**, наличие блока `MV_LLM_*` и отсутствие
  `MV_OPENAI_API_KEY`/`MV_DEEPSEEK_API_KEY`.
- `shared/objstore` — 65,0 % (unit): `Memory` целиком (все методы, `Capabilities`, идемпотентность
  `EnsureBucket`, изоляция копий, `ErrNotFound`/`ErrNoBucket`, сортировка `List`), таблица
  `BucketOptionsFor`, `lifecycleOf`, `translate`, `retryable`, поведение `retry` (3 попытки, паузы,
  остановка на финальном ответе, отмена контекста), валидация `New`/`ConfigFromEnv`.
- `shared/objstore` integration (`-tags integration`, testcontainers, образ `MINIO_IMAGE` из
  `build/versions.env`) — **прогнан локально**, образ
  `multiverse-core/minio:RELEASE.2025-10-15T17-29-55Z` уже собран в T-004: `Put/Get/Stat/List/Delete`,
  `EnsureBucket` включает versioning + ILM 30 дн. на `entities-*`, `prompts-*` остаётся без
  versioning с `Expiration.Days=30` (SEC-22), `ops-artifacts` без ILM, повторный `EnsureBucket`,
  `Capabilities()={true,true}`. Результат: `ok 9,4 c`.
- `shared/logging` — 98,4 %: обязательные поля строки, пропуск пустых полей, «внешний ID, username,
  текст игрока и токен не попадают в лог» (ни под ключом, ни в `msg`, ни внутри ошибки), `Redact`
  на таблице, `BusMiddleware` (поля события в контексте, `handled=false`), `LogDeadLetter`
  (`handled=true`), уровень и формат из манифеста, text-формат.

### 5. Результаты DoD

| Проверка | Результат |
|---|---|
| `go build ./...` | ✔ |
| `go vet ./...` | ✔ |
| `go test -short -count=1 ./...` | ✔ 15 пакетов ok |
| `go test -tags integration ./shared/objstore/` | ✔ ok 9,4 c (MinIO в testcontainers) |
| `golangci-lint run ./...` | ✔ `0 issues` |
| Покрытие `shared/env` | 94,6 % (≥ 60 %) |
| Покрытие `shared/objstore` | 65,0 % (≥ 60 %) |
| Покрытие `shared/logging` | 98,4 % (≥ 60 %) |
| Сверка манифеста с `.env.example` | ✔ 0 расхождений, файл не менялся |
| `gofmt`/`goimports` | ✔ без диффа |
| `pre-commit run --files …` | ✔ |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| `-race` | локально недоступен — в CI, T-012 |

### 6. Открытые вопросы

- **ОВ-25 (system-architect).** Имя переменной SSL хранилища: `MV_MINIO_USE_SSL`
  (`infrastructure.md` v0.3 §4.2, `.env.example`) против `MV_MINIO_SECURE` (`foundation.md` §7,
  `design.md` §8). Взято первое; поправить один из документов.
- **ОВ-26 (tech-lead#1).** `MV_CONTEXTS` и `MV_ID_SOURCE` из `design.md` §8 в манифест не внесены
  (в `infrastructure.md` §4.2 их нет, в `cmd/multiverse` это флаги). Если они нужны как env —
  внести в манифест и в `.env.example` одной правкой.
- **ОВ-27 (T-010 / EPIC-002).** Кто вызывает `env.Validate` при старте процесса. Сейчас никто:
  безусловно обязательны только ключи MinIO, и вызов в `cmd/multiverse` сломал бы `go run` без
  хранилища. Предложение: `mvctl env check` плюс контекст, открывающий хранилище.
- **ОВ-28 (T-010, T-012).** `MV_CORE_ADMIN_CLIENTS` по умолчанию `operator` (fail-closed): `mvctl`
  и `ci-harness` добавляют себя в список в своих задачах — умолчание в манифесте не менялось.
- **ОВ-29 (tech-lead#1).** `go.mod`: добавлены `minio-go/v7 v7.3.0` и
  `testcontainers-go` (+ `modules/minio`) `v0.44.0`; последний тянет заметное дерево (moby client,
  otel, gopsutil). Он же понадобится T-014. `go mod tidy` не запускался — в дереве параллельно шла
  T-006. Запустить после приёмки подволны 0.3 (совпадает с ОВ-24).

### 7. Риски и допущения

- Централизованный `vars.go` — компромисс: он гарантирует полноту сверки, но переменная контекста
  объявляется не рядом с кодом, который её читает. При росте числа контекстов стоит вернуться к
  объявлению по месту вместе с явным реестром импортов в `mvctl` (кандидат в EPIC-012).
- Редакция логов работает по ключу и по шаблону значения. Ключ, названный иначе (например
  `player_external_ref`), и токен нового формата пройдут. Тест NFR-041 (`privacy-scan`, EPIC-005)
  остаётся обязательным — редакция его не заменяет.
- `objstore.Memory` не применяет правила бакета (`Capabilities{}` пустые): тест, проверяющий ILM,
  обязан быть integration. `BucketOptionsOf` даёт проверить только намерение, не эффект.
- Ретраи и паузы `objstore` (3 попытки, 100 мс / 500 мс) выбраны по `foundation.md` §7 «3 попытки»;
  значения пауз не замерялись.

---

## developer#1 · T-006 · итерация 2 (исправления по ревью #1) · 2026-09-10

Вход: `review.md` «T-006 · ревью #1 · 2026-09-09 · code-reviewer#1» и решения оркестратора в
`journal.md` («Решения оркестратора по ревью T-006»). Исправлены M-1, Mi-1, Mi-2, Mi-7 и ОВ-20
(Mi-4). Mi-3, Mi-5, Mi-6, Mi-8, Mi-9, Mi-10 и весь список Nit по решению оркестратора уходят в
T-009/T-014 и в бэклог — здесь не трогались. Файлы T-007 (`shared/{env,objstore,logging}`,
`cmd/multiverse/{serve,health}.go`, `.golangci.yml`) не изменялись; из чужой зоны затронут только
`shared/runtime/runtime.go` — одним полем, назначенным этой задаче ещё в T-003.

### 1. M-1 — общее поле `action` вернулось в три схемы `player.*`

`api-contracts.md` §2.3.1 задаёт одну форму payload на всё семейство `player.*`; пометка
«только `player.said`» стоит там единственный раз — у `text`. Блок

```json
"action": { "type": "object",
  "properties": { "type": {"type": "string", "minLength": 1}, "key_hash": {"type": "string"} },
  "required": ["type"], "additionalProperties": false }
```

добавлен в `player.entered_region.v1.json`, `player.left_region.v1.json`, `player.said.v1.json`
дословно в том виде, в каком он уже был в пяти остальных схемах семейства, и на той же позиции —
сразу за `entity`. В `required` верхнего уровня `action` **не** внесён: в поставке он обязателен
только у `player.attacked`, и менять это ревью не просило. `text` остался только у `player.said`.

Проверка — позитивным примером §2.3 для каждой из трёх схем: в `TestPayloadExamples` фикстуры
`player.entered_region`, `player.left_region`, `player.said` теперь несут
`action{type, key_hash}` (`enter`, `leave`, `say` — три ключа из §1.4, от которых эти события и
рождаются). Плюс тест-«часовой» `TestPlayerFamilySharesTheActionField`: он читает восемь файлов
`player.*.v1.json` из `schemas.FS` и требует у каждого `properties.action.properties.{type,key_hash}`
— то есть ловит потерю поля в любой из схем семейства, а не только в этих трёх. Перепроверено
обратным прогоном: с убранным блоком тест падает.

### 2. ОВ-20 / Mi-4 — enum `cause` расширен до объединения §2.3.4 и §4.6

В `entity.create.proposed.v1.json`, `entity.update.proposed.v1.json`, `entity.updated.v1.json`
enum стал объединением `api-contracts.md` §2.3.4 и столбца «Причины» `state-and-mechanics.md` §4.6:
к `combat, rest, move, loot, spawn, tick, group, create, bootstrap, forget` добавлены
`leave, flee, death, resolve, init, author`. В `entity.created` и `entity.update.rejected` поля
`cause` нет (§2.3.4), правка их не касается. Рядом с enum — `description` с обоими источниками,
чтобы следующая правка не сузила его обратно по одному документу.

Тест-«часовой» `TestCauseEnumCoversOwnershipRules`: для каждого из трёх типов читает enum из
самой схемы (`schemas.FS`, не из списка в тесте) и требует, чтобы каждое значение `Causes` каждой
строки `OwnershipRules()` в нём присутствовало; `AnyCause` пропускается. Проверка односторонняя
намеренно — enum есть объединение, и `bootstrap` не назван ни одной строкой таблицы. Обратный
прогон: с удалённым `leave` тест падает с указанием строки-предъявителя. Тем самым замечание
закрыто структурно, а не однократной правкой enum (предложение №1 ревьюера в бэклог T-009
выполнено здесь же — дублировать его в T-009 не нужно).

### 3. Mi-1 — `Validate` для deprecated-типа проверяет конверт

`Registry.Validate` вместо `return nil` для `spec.Deprecated` возвращает `ev.ValidateEnvelope()` —
метод `eventbus.Event`, а не одноимённый метод реестра: по `foundation.md` §6 у legacy-конверта
`meta` нет вовсе, и JSON-схема `_envelope.json` отвергла бы его целиком, тогда как Go-проверка
именно этот случай и допускает (пустая `Meta` — ок, полузаполненная — дефект). Прежний путь через
шину не изменился (`Route` и `Delivery.deliver` звали `ValidateEnvelope` и раньше); закрыта дыра
в прямом вызове пакетной `contracts.Validate` из `mvctl contracts check` (T-010) и из фикстур.

Тест `TestLegacyEnvelopeIsStillChecked` — четыре негативных подтеста на legacy-типе
`time.syncTime`: без `id`, без `source`, с нулевым `timestamp` и с полузаполненной `meta`; все
обязаны дать `eventbus.ErrInvalidEnvelope`. `TestLegacyTypesCarryNoSchema` (позитивный случай —
legacy без `meta` проходит) оставлен без изменений и по-прежнему зелёный.

### 4. Mi-2 — `All()` и `Spec()` отдают глубокие копии

Заведён `cloneSpec`, которым пользуются оба метода: клонируются `Publishers`, `Consumers` и —
на одно поле глубже — `Policy.ActorKinds`, приезжающий во встроенном `eventbus.TypeSpec`.
Приём тот же, что в `OwnershipRules()`. `Schema` (`*jsonschema.Schema`) сознательно остаётся
общей: скомпилированная схема неизменяема, а перекомпиляция на вызов стоила бы дороже самой
валидации — это записано комментарием у `cloneSpec`. `Lookup` (метод интерфейса `eventbus.Registry`)
не трогал: он отдаёт `TypeSpec` по значению на горячем пути шины, и ревью его не называло.

Тест `TestSpecsAreCopies`: переписывает `Publishers[0]`, `Consumers[0]` и `Policy.ActorKinds[0]`
у того, что вернули `Spec()` и `All()`, и требует, чтобы следующий вызов отдал исходные значения.
Обратный прогон подтверждает, что без `cloneSpec` тест падает на всех трёх срезах.

### 5. Mi-7 — поле `Contracts` в `runtime.Deps`

`shared/runtime/runtime.go`: в `Deps` добавлено `Contracts *contracts.Registry` — тип по
`foundation.md` §3 (там `Deps` объявлен именно так) и C-01. Узкий интерфейс не понадобился:
цикла импортов нет — `shared/contracts` тянет `schemas`, `shared/eventbus`, `shared/jsonpath`,
`shared/clock` и не тянет `shared/runtime` (проверено `go list -deps`). Комментарий к `Deps`
теперь называет незакрытым только `Store`/`Env` из T-007; пометка «Contracts в F-4b (T-006)»
снята. Поле пока никем не заполняется — заполнит его задача, поднимающая контексты (EPIC-002),
и `mvctl` (T-010); смысл правки в том, что контекст теперь **может** получить реестр-фикстуру
(`contracts.New` над деревом-фикстурой) вместо синглтона `contracts.Default()`.

### 6. Результаты DoD итерации

| Критерий | Результат |
|---|---|
| `go build ./...` | ✔ |
| `go vet ./...` | ✔ |
| `go test -short -count=1 ./...` | ✔ 11 пакетов `ok`, `FAIL` нет |
| `golangci-lint run ./...` | ✔ `0 issues` |
| покрытие `shared/contracts` ≥ 79 % | ✔ **81,4 %** (было 79,6 %) |
| `gofmt -l shared/contracts schemas shared/runtime` | ✔ пусто |
| `pre-commit run --files <изменённые>` | ✔ |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| `-race` | n/a локально (ОВ-5), в CI T-012 |

Новых тестов четыре, все «часовые»: `TestPlayerFamilySharesTheActionField`,
`TestCauseEnumCoversOwnershipRules`, `TestLegacyEnvelopeIsStillChecked`, `TestSpecsAreCopies`.
Каждый проверен обратным прогоном — с откаченной правкой падает именно он.

### 7. Открытые вопросы

- **ОВ-30.** `Deps.Contracts` объявлен, но ни один вызывающий его не заполняет: `cmd/multiverse`
  собирает `Deps` без реестра, и до появления первого контекста (EPIC-002) поле нулевое. Кто
  обязан класть туда `contracts.Default()` — `cmd/multiverse` при старте или каждая задача
  контекста? Предложение: `cmd/multiverse`, одной строкой, в задаче подключения контекстов;
  до тех пор контекст, которому реестр нужен, обязан падать в `Start` (по godoc `Deps`).
- **ОВ-31.** Ревизия §2.3.1 у system-analyst: считать ли `action` обязательным полем для всех
  восьми `player.*` (сейчас в `required` только у `player.attacked`). Сегодняшняя форма —
  «поле разрешено везде, обязательно там, где так было в поставке»; это совместимо в обе
  стороны, но правило не записано ни в одном документе.

### 8. Риски и допущения

- Расширение enum `cause` — совместимое изменение (принимается строго больше), потребителей
  не ломает; но схема теперь не отличит опечатку издателя в пределах шести новых значений от
  штатной причины: семантику причины по-прежнему проверяет State по `OwnershipRules`, не схема.
- «Часовой» по `cause` сверяет таблицу владения со схемой, а не с `state-and-mechanics.md` §4.6:
  если строку таблицы изменят вместе со схемой, тест смолчит. Сверку копии с истиной
  (`shared/agent/levels.go`) по-прежнему даёт только T-202.
- `cloneSpec` копирует срезы на каждый вызов `All()`; `All()` вызывается в тестах и в
  `mvctl contracts *`, на горячем пути шины его нет (там `Lookup`). Замера не делал.
- `Contracts *contracts.Registry` вводит зависимость `shared/runtime` → `shared/contracts`
  (и транзитивно → `schemas`, `jsonschema/v6`). Цикла нет, но `shared/runtime` перестал быть
  пакетом без тяжёлых зависимостей — если это нежелательно, замена на узкий интерфейс в
  `runtime` возможна одной правкой и без изменения вызывающих.

---

## developer#2 · T-007 · итерация 2 (исправления по ревью #1) · 2026-09-10

Вход: `review.md` «T-007 · ревью #1 · 2026-09-09 · code-reviewer#2» и решения оркестратора в
`journal.md` («Решения оркестратора по ревью T-007»). Исправлены M-1, Mi-1, Mi-2, Mi-3, Mi-4,
Mi-5, Mi-6, Mi-9. Mi-7 (пометка `contract-change` — за оркестратором при коммите), Mi-8
(`go mod tidy` — за оркестратором после приёмки подволны 0.3), Mi-10 (сходимость `EnsureBucket`
и `minio-init` — T-008) и N-1…N-6 по решению оркестратора здесь не трогались.

Файлы T-006 (`shared/contracts/**`, `schemas/**`, `shared/runtime/runtime.go`) не изменялись:
параллельно в них работает developer#1. Из общего кода затронут только
`shared/runtime/http_test.go` — одним тестом на следствие Mi-6, файл уже был в объёме T-007.

### 1. M-1 — `Delete` идемпотентен с обеих сторон

Принят вариант (а) из ревью: семантика S3. `DeleteObject` отвечает `204` и на отсутствующий ключ,
и требовать от `Memory` другого поведения значило бы держать в unit-тестах контракт, которого на
живом сервере нет.

- `objstore.go`: doc `Client.Delete` переписан — удаление отсутствующего ключа не ошибка, и
  вызывающий, которому нужно знать, был ли объект, делает `Stat` и принимает гонку.
- `memory.go`: проверка существования убрана, `ErrNoBucket` сохранён — бакета, которого никто не
  создавал, на сервере тоже нет.
- `minio.go`: doc-комментарий `Delete` теперь называет идемпотентность явно, кода не менял.

Расхождение было невидимо потому, что кейс «удалить то, чего нет» не покрывал ни один тест. Теперь
он закреплён с обеих сторон, и оба теста ссылаются друг на друга по имени:

- `memory_test.go` — `TestMemoryDeleteOfAMissingObjectIsNotAnError` (удаление отсутствующего
  ключа, повторное удаление уже удалённого);
- `integration_test.go` — `TestMinIODeleteIsIdempotent`, те же три шага на живом MinIO плюс
  `Stat` после удаления (`ErrNotFound`), то есть идемпотентность `Delete` не размывает `Stat`.

`TestMemoryDelete` больше не ждёт `ErrNotFound` от повторного `Delete`: факт удаления проверяется
через `Get`.

### 2. Mi-1 — дефолт `--url` у `multiverse health` (`cmd/multiverse/health.go`)

Добавлена `defaultHealthURL()`: `net.SplitHostPort` разбирает `MV_CORE_ADDR`, и хост `""`,
`0.0.0.0` или `::` заменяется на `127.0.0.1`; сборка обратно через `net.JoinHostPort`, чтобы
IPv6 остался в скобках. Адрес без порта (`core`) возвращается как есть — `SplitHostPort` даёт
ошибку, и подстановка не нужна.

Тест `TestDefaultHealthURLSubstitutesLoopbackForAWildcardHost` — табличный, семь случаев:
`:8090`, `0.0.0.0:8090`, `[::]:8090` → `http://127.0.0.1:8090/health`; `127.0.0.1:8090`,
`core:8090`, `[::1]:8090`, `core` остаются собой.

### 3. Mi-3 — ПДн из threat-model в списке редакции (`shared/logging/logging.go`)

В `sensitiveKeys` добавлены `chat_id`, `first_name`, `last_name`, `update_id` — все четыре
названы поимённо в T-06 и T-08. В комментарии к списку записано, почему они там: библиотека бота,
логирующая обновление целиком, пишет ровно эти поля.

Тест `TestTelegramUpdateFieldsNeverReachTheLog`: строка с `chat_id`, `update_id`, `first_name`,
`last_name`, `username` — ни одно значение не встречается в выводе, у каждого ключа стоит
`[redacted]`. `chat_id` и `update_id` передаются как `slog.Int64`, чтобы проверить, что редакция
по ключу срабатывает независимо от типа значения.

### 4. Mi-4 — обязательность переменных образов (`shared/env/infra.go`)

`MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`, `NEO4J_PASSWORD` помечены `Required()`. Флаг ничего не
ломает: `validate` снимает безусловную обязательность у `scope=tooling`, поэтому core в
контейнере по-прежнему стартует, не видя `MINIO_ROOT_PASSWORD`. Зачем нужен: без него
`mvctl env check` (T-010) не может выполнить проверку «(г) обязательные переменные без дефолта».
Условие «профиль memory» для `NEO4J_PASSWORD` через `RequiredWhen` невыразимо (`COMPOSE_PROFILES`
— список, а условие сравнивает значение целиком); записано безусловно, причина — в комментарии,
compose продолжает страховать через `${VAR:?}`.

Тест `TestImageCredentialsAreRequiredInTheManifestButNotForAProcess` проверяет обе стороны:
флаг стоит, scope остался `tooling`, и `Validate` без этих трёх переменных возвращает `nil`.
Сверка с `.env.example` осталась зелёной — все три там уже пустые, правило (г) выполняется.

### 5. Mi-5 — `Validate` проверяет тип значения (`shared/env/env.go`)

У `Var` появился объявленный вид значения: `ValueKind` (`string|int|bool|duration`), опция
`Kind(...)` и три короткие формы — `IsInt()`, `IsBool()`, `IsDuration()`. Названы с приставкой
`Is`, потому что `Int`, `Bool` и `Duration` уже заняты методами `Var`, и одноимённая функция
пакета читалась бы как их вариант.

- `validate` после проверки enum вызывает `checkKind`, который разбирает значение тем же
  парсером, что и типовой геттер, и возвращает ту же ошибку (`expected an integer` и т. д.).
  Пустое значение до `checkKind` не доходит: незаданная переменная — вопрос обязательности, а не
  типа.
- `declare` панически падает, если дефолт не разбирается по объявленному виду. Это ошибка
  программиста, видимая при загрузке пакета, а не при первом чтении переменной.
- `CheckExample` получил правило (ж): значение, которое `.env.example` действительно задаёт,
  должно разбираться по объявленному виду. Правка на четыре строки в `valueProblems`; логически
  это то же требование, только для файла, который процесс не загружает.

Помечены 14 переменных манифеста (`MV_BUS_VALIDATE_ON_READ`, `MV_MINIO_USE_SSL`,
`MV_SNAPSHOT_EVERY_FACTS`, `MV_LAWS_BREACH_PHASE`, `MV_LLM_NUM_CTX`, `MV_LLM_STORE_PROMPTS`,
`MV_LLM_CLOUD_ENABLED`, `MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS`, `MV_LLM_PORT`, `MV_LLM_SLOTS`,
`MV_LLM_NGL`, `MV_LLM_THREADS`, `MV_LLM_BATCH_SIZE`, `MV_TELEGRAM_POLL_TIMEOUT_S`) и две внешние
(`OLLAMA_MAX_LOADED_MODELS`, `OLLAMA_NUM_PARALLEL`). `MV_LLM_CLOUD_BUDGET_USD_PER_DAY` оставлена
строкой сознательно: это сумма денег, она может быть дробной, а вида «число с плавающей точкой» в
манифесте нет; причина записана в шапке `vars.go`. `OLLAMA_KEEP_ALIVE` тоже строка — там
допустимы и `-1`, и `5m`.

Тесты: `TestValidateChecksTheDeclaredKind` (корректные значения проходят, `abc`/`yes`/`25` дают
три ошибки с именем и значением, пустое значение не считается ошибкой типа),
`TestValidateReportsATypedSecretWithoutItsValue` (значение секрета в ошибку не попадает, стоит
пометка «withheld»), `TestDeclareRejectsADefaultOfTheWrongKind`.

### 6. Mi-2 — быстрый выход в `Redact` (`shared/logging/logging.go`)

`credentialPatterns` стал срезом структур `{must, re}`: рядом с каждым шаблоном записан литерал,
который обязан присутствовать в любой подходящей строке (`":"` для токена бота, `"sk-"` для ключа
`sk-`). `Redact` сначала делает `strings.Contains(s, must)`, затем `re.MatchString` и только
потом `ReplaceAllString`, который копирует строку даже когда совпадений нет.

Инвариант «литерал следует из выражения» записан в комментарии к типу: добавляющий шаблон обязан
назвать литерал, иначе быстрый путь пропустит секрет. Проверять инвариант автоматически нечем —
это ограничение зафиксировано в §9.

Замер (13900K, `-benchtime=200000x`, `Output: io.Discard`); бенчмарки добавлены в
`logging_test.go`:

| | было (из ревью) | стало |
|---|---|---|
| `Redact`, совпадений нет | 2634 ns, 6 alloc | **18,5 ns, 0 alloc** |
| строка лога, 2 атрибута | 2162 ns, 21 alloc | **1580 ns, 3 alloc** |

До 548 ns голого `slog` не дошло: остаток — сам `ReplaceAttr` на каждом атрибуте и лишний
атрибут `version`, который по §7.1 обязателен.

### 7. Mi-6 — пустое значение перебивает дефолт (`shared/env/env.go`)

`rawFrom` больше не выдаёт «задано пустым» за «не задано»: возвращается пара
«обрезанное значение, есть ли переменная в источнике вообще». Следствия:

- `String`/`List` при `MV_CORE_ADMIN_CLIENTS=` возвращают пусто, а не `operator` — попытка
  закрыть `/v1/admin/*` теперь их закрывает;
- `Set()` сохранил прежний смысл («несёт непустое значение») — добавлена проверка `raw != ""`;
- `validate` считает явно пустую обязательную переменную отсутствующей, а не подставляет дефолт.

Поведение описано в doc-комментариях `String` и `List` — не только в дневнике: это то место, куда
смотрит вызывающий.

Тесты: `TestEmptyValueOverridesTheDefault` (пусто и пробелы для `String`, пустой allow-list для
`List` и `ListFrom`, незаданная переменная по-прежнему даёт дефолт),
`TestValidateRejectsAnExplicitlyEmptyRequiredVariable`, и главное — следствие для маршрутов:
`shared/runtime/http_test.go` → `TestAdminOnlyAdmitsNobodyWhenTheAllowListIsEmptied` (при
`MV_CORE_ADMIN_CLIENTS=` отвергаются `operator`, `ci-harness` и запрос без заголовка).
`TestStringFallsBackToDefault` потерял кейс «пусто → дефолт»: он утверждал ровно то, что было
ошибкой.

### 8. Mi-9 — `ts` и `version` (`shared/logging/logging.go`)

- Введена константа `TimeKey = "ts"`; `redactAttr` переименовывает `slog.TimeKey` в неё, но
  только на верхнем уровне (`len(groups) == 0`) и только для значения типа time — группа вправе
  нести собственное поле `time`.
- Добавлена переменная пакета `Version` (по умолчанию `dev`), которую штампует линкер:
  `-ldflags "-X multiverse-core.io/shared/logging.Version=<sha>"`. `Init` теперь передаёт её в
  `Options.Version`, и поле `version` появляется на каждой строке, как требует §7.1. `New` с
  явным `Options.Version` работает как раньше.

Флаг линкера в `build/Dockerfile` (там сейчас `-X main.version=${VERSION}`) не добавлял: файл
принадлежит T-004/T-012. Незаштампованная сборка пишет `version=dev`, что для локального запуска
и есть правда. Пункт вынесен в §10 как открытый вопрос.

Тест `TestLoggerCarriesTheRequiredFields` теперь требует поле `ts` и отсутствие `time`;
в `TestInitAndTextFormat` добавлена проверка, что `logging.Version` непуста (иначе `Init` молча
не поставил бы поле).

### 9. Результаты DoD итерации

| Проверка | Команда | Результат |
|---|---|---|
| Сборка | `go build ./...` | ok |
| Vet | `go vet ./...` | ok |
| Тесты | `go test -short -count=1 ./...` | 10 пакетов ok |
| Integration | `go test -tags integration -count=1 ./shared/objstore/` | ok, 6,4 с (с новым кейсом) |
| Линтер | `golangci-lint run ./...` | 0 issues |
| Формат | `gofmt -l shared cmd` | пусто |
| Покрытие | `go test -short -cover` | env **94,9 %** (было 94,6), objstore **65,1 %** (было 65,0), logging **98,5 %** (было 98,4), cmd/multiverse 63,4 % |

`-race` в этом окружении недоступен — как и в итерации 1.

### 10. Открытые вопросы

- **ОВ-30.** `version` в строке лога сейчас берётся из `logging.Version`, а `build/Dockerfile`
  штампует `main.version`. Чтобы поле было не `dev`, в T-004/T-012 нужно добавить второй `-X`:
  `-X multiverse-core.io/shared/logging.Version=${VERSION}` — либо свести обе переменные к одной.
  Решение за devops/tech-lead#1.
- **ОВ-31.** `infrastructure.md` §7.1 требует `version` на каждой строке, но `cmd/multiverse`
  строит логгер напрямую через `slog.NewJSONHandler` (`serve.go`), а не через `logging.Init`, и
  редакции с полем `ts` на нём нет. Перевод `serve.go` на `logging.Init` — правка за границей
  замечаний этой итерации; предлагаю внести в T-010 или в задачу подключения контекстов.
- **ОВ-32.** Mi-6 меняет наблюдаемое поведение всех 68 переменных: `VAR=` больше не даёт дефолт.
  В `.github/ci.env` под это подпадает только `COMPOSE_PROFILES=` (читается docker compose, не
  Go, и пустое значение там и задумано). Если у кого-то в `.env` есть `MV_*=` в расчёте на
  дефолт — поведение изменится; стоит упомянуть в migration-guide (tech-writer).

### 11. Риски и допущения

- Быстрый путь `Redact` корректен, пока литерал `must` действительно следует из шаблона.
  Автоматической проверки этого нет — только комментарий у типа `credentialPattern`. Кандидат в
  бэклог: property-тест, который генерирует строки по шаблону и требует, чтобы `must` в них
  входил.
- `Kind` покрывает `int/bool/duration`. Значения-адреса (`MV_KAFKA_BROKERS`, `MV_MINIO_ENDPOINT`),
  URL и суммы денег остаются строками — вида для них нет, и вводить его ревью не просило.
- Идемпотентный `Delete` снимает у вызывающего способ узнать «объект был» без второго запроса.
  Для ротации снапшотов K=5 (EPIC-002) это безразлично — там удаляют по списку `List`; для
  сценария «удалить и посчитать, сколько удалили» понадобится `Stat` и принятая гонка.
- `Required()` на трёх внешних переменных ничего не проверяет прямо сейчас: проверку выполняет
  `mvctl env check` (T-010), которого ещё нет. До него страховка — `${VAR:?}` в compose.

---

## developer#1 · T-009 · F-4b-2 «Схемы блока «в» и политики топиков» · 2026-09-10

Ветка `epic/EPIC-001-foundation`, база `0c92f2a` (T-006 + T-007). Параллельно developer#2 ведёт
T-008 (`docker-compose.yml`, `Makefile`, `scripts/**`, `build/**`, `.github/ci.env`) — файлово
не пересекаемся. Основание: `tasks.md` §1 и T-009; `architecture/contracts.md` v0.4 §0, C-05 v1.1,
C-06 v1.1, C-07 v1.2, C-10 v1.1, C-12, C-14, §16 п. 4 и п. 7; `analysis/api-contracts.md` v0.2.1
§0, §2.0, §2.2, §2.3.6–§2.3.16; `analysis/data-model.md` §7.2, §7.4; `project/metrics.md` §4.2;
`epics/EPIC-005-memory-ops/design.md` §4.1, §12 п. 2; ADR-005 доп. 3, ADR-007, ADR-014, ADR-016,
ADR-017.

### 1. Что сделано

**31 схема блока «в»** в `schemas/events/` (стиль T-006: `$schema` 2020-12, `$id` в пространстве
`https://multiverse-core.io/schemas/events/`, `$ref` в `_common.json`, `additionalProperties: false`
на верхнем уровне и во вложенных объектах). Владельцы — по `contracts.md` §0:

| Схемы | Владелец |
|---|---|
| `combat.decided`, `encounter.started`, `encounter.ended` | EPIC-003 |
| `world.weather_changed`, `world.time_advanced`, `world.event_occurred`, `region.event_occurred`, `npc.moved`, `npc.spawned` | EPIC-003 |
| `tick.fired`, `tick.aborted`, `agent.spawned`, `agent.child_resolved`, `agent.stopped`, `agent.spawn_rejected`, `agent.blueprint_reloaded` | EPIC-003 |
| `llm.output`, `llm.output.rejected`, `content.incident.recorded`, `narrative.output`, `config.cloud_enabled` | EPIC-003 |
| `world.laws.changed`, `world.law_breach.{proposed,rejected,applied,review_decided,rolled_back}` | EPIC-003 (издатель — E-B) |
| `analytics.session.started`, `analytics.session.ended`, `analytics.turn.completed` | EPIC-004 |
| `analytics.consistency.violated` | EPIC-005 (по запросу `EPIC-005/design.md` §12 п. 2) |

**Реестр `shared/contracts/registry.go`** — 31 запись с `Owner`/`Publishers`/`Consumers`/`Policy`
по карте §2.2. Новые конструкторы: `worldEvent` (топик `world_events`), `swarmEvent` (любой топик +
`eventbus.SwarmPolicy()`), обёртка `reserved`.

**`shared/contracts/spec.go`** — поле `Spec.Reserved`: машинная форма исключения `contracts.md`
§16 п. 4 и C-12 «издатель + ≥ 1 потребитель» для семейства `world.law_breach.*`. Это то поле,
которое `mvctl contracts check` (T-010) читает, чтобы отличить зарезервированный тип от мёртвого;
`Publishers`/`Consumers` у них заполнены будущим издателем (`core/laws`), а не пусты — чтобы
проверка `source ∈ Publishers` заработала без правки реестра, когда придёт E-B.

**Тесты** (`shared/contracts/blockv_test.go`):
- `TestBlockVPayloadExamples` — по одному валидному примеру §2.3 на каждый из 31 типа; полнота
  таблицы проверяется в `TestPayloadExamples` (сумма примеров обоих блоков = число типов со схемой);
- `TestBlockVPayloadRejects` — 25 негативных фикстур: лишнее поле (`backgroud_refs`, `api_key` в
  `config.cloud_enabled`), отсутствие обязательного (`tick.lod_allowed`, `prompt_hash`, `timings`,
  `detected_by`), значение вне enum (`mode=idle`, `lod=maximum`, `end_reason=bored`,
  `severity=fatal`, `created_by=operator`, `reason=too_long`, `strategy=guess`);
- `TestQuarantinedRecordKeepsNoText` — `if/then` схемы `llm.output`: при
  `validation_status=quarantined` поле `response_raw` запрещено, при `valid` — допустимо;
- `TestValidationStatusEnum`, `TestRejectionReasonEnum` — «часовые» по ADR-017: шесть значений
  статуса и десять значений `reason`, плюс равенство `llm.output.reasons[]` тому же enum;
- `TestEndReasonIncludesForget` — `forget` в `analytics.session.ended` (C-10 v1.1);
- `TestSwarmTopicsDemandAnAgent` — 20 типов роя: с `meta.agent` политика пропускает, без него
  возвращает `ErrPolicyViolation`; `TestTopicsWithoutAnAgentRule` — 11 типов, которым агент не
  нужен; `TestPlayerEventsRejectSystem` — `player_events` отклоняет `actor_kind=system` и
  принимает `sim`;
- `TestEveryTopicCarriesAType` — у каждого `TopicSpec` есть хотя бы один `Spec` (исключение —
  `dead_letters`: обёртку `DeadLetter` пишет сама шина, типа события у неё нет);
- `TestReservedTypes` — `Reserved` стоит ровно у пяти `world.law_breach.*`;
- `TestBlockVOwners` — владелец каждого семейства.

В `registry_test.go`: `encounter.started` внесён в таблицу типов с несколькими издателями (снята
заглушка «регистрируется в T-009»); в `TestTopics` добавлена проверка `dead_letters` (30 дн., не
читается в replay).

### 2. Решения по ходу

1. **Политики топиков — по списку C-01 §2.0 плюс §0.** `SwarmPolicy` (обязателен `meta.agent`)
   поставлена на `llm.*`, `tick.*`, `agent.*`, `narrative.output`, `combat.decided` (прямой список
   §2.0) и на `encounter.*`, `world.weather_changed`, `world.time_advanced`, `world.event_occurred`,
   `region.event_occurred`, `npc.*`, `content.incident.recorded` — их §0 называет событиями,
   изданными рантаймом роя, а других издателей у них нет (`FakeEncounter`/`FakeNarrator` тоже
   пишут `meta.agent` с префиксом `fake-`, C-05). Политика **не** поставлена на
   `world.laws.changed` (издаёт автор из CLI), `world.law_breach.*` (E-B), `config.cloud_enabled`
   (публикует контекст `llm` при старте, агента нет) и `analytics.*`.
2. **Сквозные поля не дублируются в payload.** В `llm.output` нет `correlation_id`, `agent`,
   `actor_kind`, `replay`, `gm_path` — ADR-007 п. 1 и §2.3.10 прямо говорят «из payload убраны»;
   ключ записи для replay `(meta.correlation_id, meta.agent.id, phase, attempt)` собирается из
   конверта. В `analytics.turn.completed` по той же причине нет `correlation_id` и
   `trigger{event}` (§0: «`payload.trigger{event}` не используется», причина — `meta.causation_*`),
   хотя `metrics.md` §4.2 их ещё перечисляет.
3. **`world`/`scope` в аналитике — опциональные.** `metrics.md` §4.2 держит их в обязательных
   полях payload, §2.1 говорит «дублировать не требуется». Схема принимает оба варианта: издатель
   (gateway) волен положить копию для CSV-конвейера, а истина — конверт. Жёсткий выбор здесь
   означал бы либо отклонять законного издателя, либо ломать `metrics.md`; это вопрос владельцу
   (ОВ-35).
4. **`tick.fired.agent`/`scope` и `agent.spawned.scope` — опциональные копии.** §2.3.9 и
   `data-model.md` §7.4 перечисляют их в payload, конверт содержит те же данные. Асимметрия с
   `llm.output` (п. 2) — из документов, а не из моего решения.
5. **`response_raw` запрещён только при `quarantined`.** C-07 v1.2 пишет «`quarantined`/
   `filter_error` — без `response_raw`», а ADR-016 п. 3 — «`error` (фильтр упал) → `filter_error`,
   `response_raw` **сохраняется**». Взято пересечение: запрет только для `quarantined`
   (`data-model.md` §7.2 говорит ровно это). Расхождение — ОВ-33.
6. **`config.cloud_enabled.external_players_ack`** — имя по `api-contracts.md` §2.3.16 (эталон
   payload по задаче), в C-06 v1.1 то же поле названо `allow_external_players`. Расхождение — ОВ-34.
7. **`violation{code, layer, severity, detected_by}`** у `analytics.consistency.violated` — вложены
   в объект `violation` по `metrics.md` §4.2 (DoD T-009 называет поля, но не уровень вложенности).
   `code` — объединение девяти значений `metrics.md` и `log_gap`/`invariant` из C-10 v0.3; три
   значения из DoD (`state_divergence`, `log_gap`, `invariant`) присутствуют.
8. **`combat.decided`: обязательны** `encounter, round, action, attacker, outcome, rolls,
   rules_version, phase1_mode`; `defender` и `hp` — опциональны, потому что `action=flee`
   разрешается против порога, а не против цели. `living_enemies` положен внутрь `outcome` рядом с
   `success`/`threshold` (§2.3.6 перечисляет их одной фразой; чтение — допущение, см. §6).
9. **`EventRef` не заведён в `_common.json`.** Ссылка `{event:{id,type?}}` повторяется в шести
   схемах, но `_common.json` меняется только через system-architect (`tasks.md` T-006); формы
   описаны на месте, как уже сделано в `round.closed`. Предложение — в бэклог.
10. **`rules.change.*` не зарегистрированы.** T-009 называет их в списке исключений проверки, но
    их нет ни в карте §2.2, ни в списке схем задачи; C-13 относит их к резерву EPIC-006 (E-A).
    Механизм для них готов (`Reserved`) — ОВ-36.

### 3. Отклонения от дизайна

- Добавлено поле `Spec.Reserved` (`spec.go`) — задача перечисляет только
  `schemas/events/*.v1.json` и `registry.go`. Без него исключение §16 п. 4 нельзя выразить иначе,
  чем пустыми `Publishers`/`Consumers`, что противоречит DoD T-006 («у каждого `Spec` непустой
  `Publishers`»). Изменение аддитивное, публичный API `shared/contracts` не ломает.
- Правки в `registry_test.go` (`encounter.started`, `dead_letters`) и в `validate_test.go`
  (счётчик примеров) — тесты T-006, их нужно было привести в соответствие.

### 4. Результаты DoD

- `go build ./... && go vet ./...` — зелёные.
- `go test -short -count=1 ./...` — все пакеты `ok` (`-race` в этом окружении недоступен).
- `go test -short -count=1 -cover ./shared/contracts/...` — **81,5 %** (порог 81 %).
- `golangci-lint run ./...` — **0 issues**.
- `gofmt -l shared schemas` — пусто.
- Все схемы компилируются как JSON Schema 2020-12 (`TestSchemasValid`); «нет фантомов» в обе
  стороны (`TestNoPhantoms`); каждый пример §2.3 валиден (`TestPayloadExamples` +
  `TestBlockVPayloadExamples`); у каждого `Spec` непустые `Publishers`/`Consumers`
  (`TestEveryTypeHasPublisherAndConsumer`); у каждого `TopicSpec` есть тип
  (`TestEveryTopicCarriesAType`).

### 5. Открытые вопросы

- **ОВ-33.** `response_raw` при `filter_error`: C-07 v1.2 запрещает, ADR-016 п. 3 требует
  сохранять (fail-closed, текст не доставляется, но запись нужна для разбора). Схема сейчас
  разрешает. Решение за system-architect; если победит C-07, в `if/then` добавляется второе
  значение.
- **ОВ-34.** Имя третьего поля `config.cloud_enabled`: `external_players_ack` (§2.3.16) против
  `allow_external_players` (C-06 v1.1). Взято первое. Нужна правка одного из документов.
- **ОВ-35.** `world`/`scope` в payload аналитики — обязательные (`metrics.md` §4.2) или только в
  конверте (§2.1)? Сейчас опциональны. Вопрос владельцу типов (EPIC-004) и BA.
- **ОВ-36.** `rules.change.proposed`/`rules.changed` (C-13) — регистрировать в MVP-1 как
  `Reserved` или отложить до EPIC-006? В карте §2.2 их нет, схем задача не заказывала.
- **ОВ-37.** `TopicSpec` не выражает `max.message.bytes=4 МиБ` для `llm_records` (§2.2), хотя
  задуман как единственный источник для `redpanda-init` и `mvctl contracts topics`. Пока значение
  живёт только в скрипте T-008. Кандидат в T-010 или в бэклог.
- **ОВ-38.** `tick.aborted.reason` и `narrative.output.fallback_reason` описаны в документах без
  enum; сделаны строками. Когда EPIC-003 зафиксирует словарь, это совместимое сужение.

### 6. Риски и допущения

- Схемы блока «в» — первая редакция по документам; их владельцы (EPIC-002/003/004/005) ревизуют
  и дополняют в своих задачах (T-052, T-214/T-215, T-301). Там, где документ давал список полей
  без пометки «обязательно», в `required` вошёл минимум, при котором событие остаётся осмысленным
  для потребителя: слишком широкая схема ловит меньше дефектов, слишком узкая отклоняет законного
  издателя, и в пользу второго риска выбор сделан только там, где документ говорит «обязателен»
  прямо (`tick.lod_allowed`, поля `llm.output` по `data-model.md` §7.2).
- `combat.decided.outcome.living_enemies` — допущение о вложенности (§2.3.6 перечисляет
  `outcome.success`, `outcome.threshold`, `living_enemies` одной фразой). Если EPIC-003 читает
  его как поле верхнего уровня, правка — одна строка в схеме и в примере.
- Списки `Consumers` — из колонки «Потребители» §2.2, переведённой в значения `source`. Слова
  «оператор», «страж», «replay», «`session-report`» отображены на `mvctl`/`core/swarm`; если
  оператор окажется отдельным процессом, списки придётся расширить.

---

## developer#2 · T-008 · F-6b «Профили compose, `redpanda-init`/`minio-init`, Makefile, `compose-lint`» · 2026-09-10

Ветка `epic/EPIC-001-foundation` (база T-008 — `0c92f2a`). Основание: `architecture/infrastructure.md`
v0.3 §1.3, §1.4, §2.2, §3.1.1, §5.1, §5.2, §5.3, §6.1–§6.3, §9; `epics/EPIC-001-foundation/tasks.md`
T-008 и §1; ADR-005 доп. 2 п. 1/3/7, ADR-007, ADR-021; threat-model SEC-13/14/15/22/32/33, T-14,
T-17, T-32; D-3, D-8, D-9, D-10, U-3, U-8. Параллельно developer#1 вёл T-009 в `schemas/events/**`
и `shared/contracts/**` — эти каталоги не трогал.

### 1. Что сделано

**`docker-compose.yml`** — к ядру T-004 добавлены пять профилей и два init-контейнера.

| Профиль | Сервисы | Порты на хосте |
|---|---|---|
| (без профиля) | `redpanda`, **`redpanda-init`**, `minio`, **`minio-init`**, `gateway`, `core` | 19092, 9644, 9000, 8088 |
| `memory` | `qdrant`, `neo4j`, `memory` | 6333, 6334, 7687, 8082 |
| `gpu` | `ollama` | 11434 |
| `dev` | `redpanda-console` | 8092 |
| `bot` | `telegram-bot` | 8089 |
| `legacy` | `chromadb`, `semantic-memory`, `narrative-orchestrator` (+ `neo4j`) | 8083 |

Все публикации — `127.0.0.1:` (SEC-13); admin-порт `core` не публикуется, только `expose: 8090`
(T-14). `neo4j` объявлен с двумя профилями (`memory`, `legacy`) — as-is `semantic-memory` пишет
свой граф туда же, и держать два сервера незачем. `NEO4J_PLUGINS` не задан (T-17). У `ollama`
проставлены `OLLAMA_KEEP_ALIVE=-1`, `OLLAMA_MAX_LOADED_MODELS=2`, `OLLAMA_NUM_PARALLEL=1`,
`OLLAMA_FLASH_ATTENTION`, `OLLAMA_KV_CACHE_TYPE`, `OLLAMA_ORIGINS` со списком loopback-адресов
(никогда `*`); `OLLAMA_HOST` не переопределяется намеренно — образ и так слушает интерфейс
контейнера, а запись `0.0.0.0` здесь была бы ровно тем, что запрещает SEC-15. Токен бота — только
у `telegram-bot`, `env_file` нет ни у одного сервиса (D-10). Логи бота — `10m × 3` через отдельный
якорь, платформа остаётся на `50m × 5`. Введены якоря `x-platform-env` (общие `MV_*`) и
`x-depends-on-init`: `gateway` и `core` теперь ждут не `service_healthy` шины и хранилища, а
`service_completed_successfully` обоих init-контейнеров (§5.1).

**`build/redpanda-init.sh`** — 8 топиков таблицы §5.1 одной таблицей `TOPICS` в шапке скрипта:
retention 30 дн. (`player_events`, `game_events`, `world_events`, `system_events`,
`narrative_output`, `dead_letters`), 90 дн. (`llm_records` + `max.message.bytes=4194304`), 180 дн.
(`analytics_events`); всем — `-p 1 -r 1 -c cleanup.policy=delete -c segment.ms=86400000` (D-9).
`create` идёт с «уже есть» в ветке отказа, а `alter-config` выполняется **всегда** — иначе
изменённый retention никогда не доехал бы до существующего кластера. `scope_management` и прочие
фантомные топики не создаются, Schema Registry не поднимается.

**`build/minio-init.sh`** — `mc alias set` → `mc mb --ignore-existing ops-artifacts` →
сервисный пользователь + политика `readwrite` (пропускается, если `MV_MINIO_ACCESS_KEY` совпадает
с root — обычный dev-случай) → по существующим бакетам: `entities-*`/`snapshots-*` — versioning +
ILM на 30 дн. для неактуальных версий, `prompts-*` — ILM `Expiration 30 дн.` **без** versioning
(SEC-22), прочие — ничего.

**Сходимость с `objstore.EnsureBucket` (Mi-10 из ревью T-007).** `EnsureBucket` вызывает
`SetBucketLifecycle`, который *заменяет* всю конфигурацию одним правилом с id
`multiverse-retention-<bucket>` (`shared/objstore/minio.go`, `lifecycleOf`). Скрипт пишет **то же
самое единственное правило с тем же id** через `mc ilm rule import`, который тоже заменяет
конфигурацию целиком. Поэтому порядок запусков не важен и второго, конфликтующего правила не
появляется ни при каком чередовании. Значения (30 дн.) продублированы в скрипте константой
`RETENTION_DAYS` со ссылкой на `shared/objstore/buckets.go` — единственное место, где они могут
разъехаться, и его стережёт integration-тест T-007.

**`scripts/compose-lint.sh`** — шесть правил §3.1.1, один и тот же скрипт для CI и Git Bash.
Разбор модели — `docker compose config --format json`, дальше встроенная программа на `python3`:
`jq` на машине владельца нет, а тянуть контейнер ради разбора документа — странная зависимость для
линтера. Правила 1 и 3 смотрят **исходный текст** файла, а не интерполированную модель: после
подстановки литеральный пароль и правильный `${VAR:?}` выглядят одинаково.

**`Makefile`** — переписан целиком под §2.2: `SHELL := bash`, `.SHELLFLAGS := -euo pipefail -c`,
`.ONESHELL`, `include build/versions.env`, `export GOFLAGS=-buildvcs=false`. Цели as-is
(`SERVICES`, `build-service`, `build-all`, `test-service`, `sync`, `run`) удалены вместе с
workspace. Добавлены `build lint test test-integration test-e2e contracts secrets-scan vuln
compose-lint ci ci-full image minio-image up down reset logs health llm-up llm-down llm-health
models warm bench backup restore archive-legacy legacy-src replay deploy rollback clean help`.
`make up` LLM не поднимает — только `make health` зовёт `llm-health`. `make clean` больше не делает
`docker system prune` (он удалял чужие образы). `PROFILES=` переопределяет `COMPOSE_PROFILES` на
один вызов.

**`scripts/llm-server.ps1` / `.sh`** — старт/стоп/health нативного `llama-server`. Аргументы — по
таблице §6.2, значения из `.env`; семплинга (`--temp`, `--top-p`, `--top-k`, `--min-p`) нет
намеренно — его задаёт шлюз на каждый запрос из блупринта фазы (ADR-005 доп. 2 п. 1). `--no-webui`
по умолчанию, `--tools all`/`--ui-mcp-proxy` — только по явному `-WithUi`/`--with-ui`. `up` ждёт
`/health = 200` до 180 с (503 = loading — норма), затем делает один короткий
`/v1/chat/completions` (`max_tokens: 1`, `enable_thinking: false`) и печатает его латентность.
Повторный `up` при живом процессе — no-op; при «PID есть, порт молчит» — рестарт. `health`
печатает код `/health`, имена из `/v1/models`, факт билда против пина `LLAMACPP_BUILD`
(расхождение — предупреждение, не ошибка) и `nvidia-smi`.

**`build/versions.env`** (дополнение): `LEGACY_GO_IMAGE`, `LEGACY_SRC_REF`, `ALPINE_IMAGE`
(тар-контейнер для `backup`/`restore`/`archive-legacy`; в рецептах Makefile тегов нет).
**`.github/ci.env`** (дополнение): секция `legacy` — `CHROMA_IMAGE` с заведомо непритягиваемым
плейсхолдером и три `LEGACY_*`. **`.env.example`**: заполнена секция `legacy`, в блок Ollama
добавлен `OLLAMA_ORIGINS`. **`.gitignore`**: `/build/.legacy-src/`.

### 2. Решение по жизнеспособности профиля `legacy`

Профиль **жив, но собирается не из рабочего дерева, а из архивного коммита**. Запасной вариант S5
не потребовался.

Что выяснилось. `build/legacy.Dockerfile` (копия as-is корневого Dockerfile) копировал `go.work`,
которого больше нет. Простая починка невозможна: замороженные сервисы не компилируются против
текущего `shared/`. Проверено воспроизведением раскладки вручную (`shared/` из корня + as-is
`shared/{config,minio,oracle,spatial}` из архива, `go.work` над ними):

```
narrative_agent.go:38:16: undefined: eventbus.EventBus
narrative_agent.go:77:19: undefined: tools.WorldTool
narrative_agent.go:80:20: undefined: tools.EntityTool
narrative_agent.go:83:23: undefined: tools.NarrativeTool
```

То есть F-4a (T-005) заменил `shared/eventbus`, F-3 (T-002) увёл `shared/{config,minio,oracle,
spatial}` в другой модуль и убрал инструменты с побочными эффектами (SEC-18, ADR-001 доп. п. 5), а
F-2 (T-003) удалил `go.work`. Править замороженный код нельзя по определению заморозки.

Решение: контекст сборки профиля — **последний коммит, на котором этот код собирался**,
`LEGACY_SRC_REF=e6103e4` в `build/versions.env`. `make legacy-src` делает
`git archive $(LEGACY_SRC_REF) go.work go.mod go.sum shared services/narrative-orchestrator
services/semantic-memory | tar -x -C build/.legacy-src`; `make up PROFILES=legacy` вызывает его сам.
Экспортируются только нужные пути — `.mcp.env` того коммита намеренно остаётся за бортом. Ничего не
удалено и не разморожено, U-1 соблюдён.

Вторая находка: as-is корневой Dockerfile генерировал `go.work` как `use (. ./services/X)`, а
`shared/{agent,agent/tools,config,eventbus,minio,oracle,spatial}` в as-is раскладке — **отдельные
модули**, и без них сборка падает с «no required module provides package
multiverse-core.io/shared/oracle». `build/legacy.Dockerfile` теперь генерирует `go.work` из
фактически найденных `shared/**/go.mod`.

Проверено: `docker build -f build/legacy.Dockerfile --build-arg SERVICE=narrative-orchestrator
build/.legacy-src` — образ собран (exit 0). `semantic-memory` (CGO + ONNX Runtime) не собирался —
только `docker compose --profile legacy config -q`; это стендовая проверка (ОВ-41).

### 3. Отклонения от дизайна

1. **Консоли MinIO (9001) и Neo4j (7474) не публикуются вообще.** §1.4 просит «только в профиле
   `dev`», но публикация порта в compose не может зависеть от профиля, а вынести её в отдельный
   сервис нельзя: Docker запрещает `ports` вместе с `network_mode: service:`. Принято более строгое
   прочтение SEC-33 — не публиковать; доступ оператора — `docker compose port` или локальный,
   git-ignored `docker-compose.override.yml`. `redpanda-console` — единственная консоль в файле, и
   она в профиле `dev` (ОВ-40).
2. **Пути init-скриптов** — `build/redpanda-init.sh` и `build/minio-init.sh` (как в `tasks.md` и в
   комментарии `shared/objstore/buckets.go`), а не `scripts/`. `scripts/` остаётся за скриптами,
   которые запускает оператор и CI.
3. **`entrypoint` `minio-init` — `/bin/bash`, не `/bin/sh`.** Образ `minio/mc` несёт bash и
   coreutils и **не несёт** `sed`, `grep`, `awk`: первая версия скрипта на POSIX sh молча
   отработала вхолостую (`sed: command not found` внутри подстановки, цикл по нулю бакетов). Разбор
   `mc ls` переписан на builtin'ы bash, а листинг сначала кладётся в переменную — чтобы ошибка
   листинга роняла контейнер, а не приводила к тихому «ничего не сделано».
4. **`rpk` вызывается как `-X brokers=...`, а не `--brokers`.** У rpk v26 `rpk cluster health`
   ходит в admin API на 9644 и флага `--brokers` не имеет вовсе (`Error: unknown flag`). Готовность
   проверяется через `rpk cluster info -X brokers=...` — тот же Kafka-путь, которым потом идут
   команды создания топиков. Первая версия скрипта из-за этого падала с «broker not reachable».
5. **`make llm-*` на Windows без PowerShell 7 падает с понятным сообщением**, а не откатывается на
   `.sh`: супервизия нативного Windows-процесса MSYS-сигналами хуже, чем честный отказ (ОВ-43).
6. **`CHROMA_IMAGE` продублирован в `.github/ci.env`** — единственный пин, повторённый вне
   `versions.env`, и только потому, что там он намеренно пуст (D-3). Иначе `docker compose
   --profile legacy config -q` невозможен в CI. Значение — заведомо несуществующий тег.
7. **Изменения за пределами перечня файлов T-008**: `build/versions.env` (+3 переменные),
   `.gitignore` (+1 строка, каталог артефакта `make legacy-src`), `.env.example` (секции `legacy`
   и `OLLAMA_ORIGINS`). Все — минимальные и следуют из состава задачи.

### 4. Как проверял

| Проверка | Команда | Результат |
|---|---|---|
| Профили compose | `docker compose --env-file build/versions.env --env-file .github/ci.env [--profile P] config -q` для «ядра», `gpu`, `memory`, `dev`, `legacy`, `bot` | 6/6 ok |
| compose-lint | `bash scripts/compose-lint.sh` | ok, 15 сервисов, 6 правил |
| compose-lint ловит нарушения | 10 временных копий compose с подложенными нарушениями (latest; порт без `127.0.0.1`; 7474 вне `dev`; литерал `minioadmin`; `${VAR:-minioadmin}`; токен бота у `core`; `NEO4J_PLUGINS`; `OLLAMA_ORIGINS=*` + `OLLAMA_HOST=0.0.0.0`; сервис `llama-server`; `MV_LLM_URL=https://api.openai.com/...`) | 10/10 пойманы, копии удалены |
| Синтаксис скриптов | `bash -n` (redpanda-init, minio-init, compose-lint, llm-server.sh) | ok |
| Makefile | разбор GNU make 4.4.1 в контейнере (`make -n` по 23 целям) | все цели разбираются |
| **redpanda-init на живом брокере** | `COMPOSE_PROJECT_NAME=mvt008check docker compose up -d --wait redpanda redpanda-init`, затем `rpk topic list` / `rpk topic describe -c` | 8 топиков; `retention.ms` 2592000000 / 7776000000 / 15552000000, `segment.ms=86400000`, `cleanup.policy=delete`, `llm_records.max.message.bytes=4194304` |
| Идемпотентность redpanda-init | повторный запуск контейнера | «exists» + «config is up to date», exit 0 |
| **minio-init на живом MinIO** | образ собран `docker build -f build/minio.Dockerfile` (164 МБ); бакеты `entities-probeworld`, `snapshots-probeworld`, `prompts-probeworld` созданы вручную, затем init | `entities/snapshots` — `versioning is enabled` + правило `multiverse-retention-<bucket>` с `NoncurrentVersionExpiration.NoncurrentDays=30`; `prompts` — `is un-versioned` + `Expiration.Days=30`; `ops-artifacts` — правил нет |
| Идемпотентность minio-init | повторный запуск | ровно одно правило на бакет, без дублей |
| Сервисный пользователь MinIO | запуск с `MV_MINIO_ACCESS_KEY=svc-probe-user` | «service user added» + «policy readwrite attached», повтор — «exists» |
| **Полный подъём ядра** | `docker compose up -d --wait` (проект `mvt008check`, образ платформы собран) | `redpanda healthy → redpanda-init exited 0 → minio healthy → minio-init exited 0 → core healthy, gateway healthy`; exit 0 |
| Health | `curl 127.0.0.1:8088/health` = 200; `docker exec core /multiverse health --url http://127.0.0.1:8090/health` = ok | ok |
| Публикации портов | `docker compose ps` | только `127.0.0.1:` (8088, 9000, 9644, 19092); у `core` — `8090/tcp` без публикации, `docker port core` пуст |
| Сборка профиля legacy | `docker build -f build/legacy.Dockerfile --build-arg SERVICE=narrative-orchestrator build/.legacy-src` | образ собран (exit 0) |
| Уборка стенда | `docker compose -p mvt008check down -v` | контейнеры, сеть и тома проекта удалены; as-is тома владельца (`multiverse_*`) не тронуты |

Go-тесты — n/a (задача без Go-кода). `make lint`/`go test` по общему DoD §1 не запускал: Go-файлы
не менялись.

### 5. Результаты общего DoD §1

- п. 1–3 (`make lint`, `go build/vet/test`, тесты) — **n/a**: изменений в Go-коде нет.
- п. 4 `make secrets-scan` — `gitleaks git --staged --redact .` и `gitleaks dir` по рабочей копии:
  0 находок (см. отчёт оркестратору).
- п. 5 — эта запись.
- п. 7 — изменения в границах владения (`docker-compose.yml`, `Makefile`, `scripts/**`, `build/**`
  кроме `Dockerfile`/`minio.Dockerfile`, `.github/ci.env`, `.env.example`, `.gitignore`);
  `schemas/events/**` и `shared/contracts/**` (T-009, developer#1) не тронуты.
- п. 8 — CI появится в T-012; локальный эквивалент запускался частями (см. §4).

**Не выполнен один критерий T-008**: «`git grep -i minioadmin` пуст вне `Docs/` и
`services/_archive/`» — см. ОВ-42.

### 6. Открытые вопросы

- **ОВ-40.** Консоли MinIO (9001) и Neo4j Browser (7474) в профиле `dev` не опубликованы —
  compose не умеет условную публикацию порта (§3 п. 1). Варианты: (а) оставить как есть, доступ
  через `docker compose port`; (б) завести `build/compose.dev.yml` как второй `-f`, который
  добавляет публикации, и научить `compose-lint` линтовать обе комбинации; (в) поправить §1.4.
  Решение — devops-engineer / tech-lead#1.
- **ОВ-41.** Профиль `legacy` собирается из `LEGACY_SRC_REF=e6103e4`, а не из рабочего дерева
  (§2). На стенде остаётся проверить: сборку `semantic-memory` (CGO + ONNX Runtime, тяжёлая),
  тег `CHROMA_IMAGE` (D-3, сейчас пуст) и фактический старт связки orchestrator → semantic-memory
  → chroma. Пока это не сделано, `MV_GM_PATH=legacy` считать непроверенным; уведомить tech-lead#2.
  Побочный эффект решения: при исправлении бага в замороженном сервисе править придётся коммит,
  а не файл — то есть заводить новый ref. Для профиля, который удаляется в EPIC-003 I2, приемлемо.
- **ОВ-42.** `git grep -i minioadmin` вне `Docs/` и `services/_archive/` **не пуст**: 20 вхождений
  — `QWEN.md` (5), `.claude/plugins/multiverse-core-plugins/README.md` (2) и **замороженные as-is
  сервисы** (`services/{entity-manager,entity-actor,evolution-watcher,ontological-archivist,
  rule-engine}/cmd/main.go`, `services/entity-manager/entitymanager/manager.go`,
  `services/semantic-memory/semanticmemory/indexer.go` — дефолт `getEnv("MINIO_ACCESS_KEY",
  "minioadmin")`). Ни один из этих файлов не в моём владении и не в целевой сборке. `compose-lint`
  проверяет правило 3 по области целевой платформы (`docker-compose.yml`, `build/`, `scripts/`,
  `cmd/`, `shared/`, `.github/`, `.env.example`), исключая упоминания вида «не minioadmin» —
  `.env.example:18` и `shared/env/infra.go:34` именно такие. Значимая часть находки: у
  `services/semantic-memory` этот дефолт **попадает в рантайм профиля `legacy`**, если
  `MINIO_ACCESS_KEY` не задан; в compose он задан явно из `MV_MINIO_ACCESS_KEY`, так что дыры нет,
  но формулировку критерия и судьбу `QWEN.md` (T-019) нужно закрыть решением tech-lead#1.
- **ОВ-43.** PowerShell 7 на машине **не установлен** (`pwsh` нет ни в `PATH`, ни в
  `C:\Program Files\PowerShell\`), хотя `infrastructure.md` §1.1 числит его в составе окружения.
  `scripts/llm-server.ps1` требует `#Requires -Version 7` (`??`, `Set-StrictMode Latest`),
  проверить его парсером 5.1 нельзя. До установки (`winget install Microsoft.PowerShell`)
  `make llm-up`, `make llm-down`, `make llm-health` и `make health` на Windows не работают.
  Проверить скрипт на стенде обязательно до F-8.
- **ОВ-44.** Сервис `telegram-bot` профиля `bot` объявлен с `entrypoint: ["/telegram-bot"]`, но
  бинарника ещё нет — `cmd/telegram-bot` появится в EPIC-004. `docker compose --profile bot config
  -q` проходит, `up` — нет. Это ожидаемо; если tech-lead#1 предпочтёт, чтобы профиль до EPIC-004
  вообще отсутствовал, сервис легко изъять.
- **ОВ-45.** `make bench` зовёт `scripts/llm-bench.ps1`/`.sh`, которых ещё нет — они относятся к
  F-8 (T-013). Цель оставлена по §2.2, чтобы имя не менялось потом.

### 7. Риски и допущения

- `compose-lint` требует `python3` (есть и на машине владельца, и на `ubuntu-latest`). Это
  сознательная замена `jq`, которого на машине нет. Если CI когда-нибудь останется без Python,
  правило разбора придётся переписать.
- Правила 1 и 3 линтера читают исходный YAML построчными регулярками (`^  <service>:`,
  `^    image:`, `^      KEY: value`). Это работает, пока файл отформатирован так, как сейчас;
  переход на другой стиль отступов правила ослабит — не сломает громко, а перестанет что-то
  видеть. Кандидат в бэклог: разбирать исходник YAML-парсером в CI.
- `mc ilm rule import` заменяет конфигурацию целиком. Если EPIC-005 когда-нибудь захочет второе
  правило на том же бакете, сходимость с `EnsureBucket` придётся пересматривать вместе — сейчас
  обе стороны исходят из «ровно одно правило на бакет».
- `make health` считает сервис отсутствующим, если его нет в `docker compose ps --services`; при
  неверном `PROFILES` это выглядит как «-», а не как ошибка. Осознанно: выключенный профиль
  `memory` — это не сбой.
- `make backup` останавливает `core` и `redpanda` на время `tar`. При большом томе окно простоя
  растёт линейно; альтернатива без остановки (`mc mirror`) описана в §5.6 и не автоматизирована.
- Дефолт `LEGACY_SRC_REF=e6103e4` взят из `services/_archive/build/ARCHIVED.md` («последний рабочий
  коммит»). Если история ветки будет переписана (`git filter-repo`, §4.5), ref нужно обновить.

---

## developer#2 · T-008 · итерация 2 (исправления по ревью #1) · 2026-09-09

Вход: `review.md` «T-008 · ревью #1 · 2026-09-09 · code-reviewer#2» и задание оркестратора.
Исправлены M-1…M-5, Mi-1, Mi-2, Mi-3, Mi-6. Mi-4 (правило 1 линтера сверяет имя переменной, а не
значение), Mi-5 (`legacy-src` не подхватывается из `COMPOSE_PROFILES`), Mi-7 (свежесть бэкапа в
`make reset`) и Nit по решению оркестратора не трогались — они уходят в бэклог (T-012/T-019).

### 1. M-1 — `OLLAMA_ORIGINS` объявлен в манифесте

`shared/env/infra.go` (файл T-007, правка согласована оркестратором) — три строки рядом с
остальным блоком `OLLAMA_*`, в том же стиле: `DeclareExternal` + `Tooling()` +
`RequiredWhen("MV_LLM_PROVIDER", "ollama")`. Переменная — вход compose (`docker-compose.yml`
читает `${OLLAMA_ORIGINS:-…}`), поэтому убирать строку из `.env.example` было бы хуже.
`go test -short ./shared/env/` — зелёный.

### 2. M-2 — healthcheck `qdrant` на `bash -c`

`CMD-SHELL` — это `/bin/sh -c`, а `/bin/sh` в `qdrant/qdrant:v1.19.1` — `dash`, у которого нет
`/dev/tcp`. Заменено на `["CMD", "bash", "-c", "exec 3<>/dev/tcp/127.0.0.1/6333"]` (§5.3).
Проверено на стенде отдельным проектом (`-p mvqdranttest`), чтобы не трогать тома владельца:
`docker compose --profile memory up -d qdrant --wait` → `Container … Healthy`, код 0; затем
`down -v` этого проекта.

### 3. M-3 — `make up` не падает без LLM

Введена переменная строгости `LLM_STRICT ?= 1` (вариант «б» из ревью). `make health` сам по себе
остаётся строгим по §2.2; `up`, `deploy` и `rollback` зовут его как `health LLM_STRICT=0`, и тогда
неуспешный `llm-health` печатает предупреждение из §6.3 дословно
(`warning: LLM недоступен, нарратив деградирует (FR-080); подними процесс: make llm-up`) и не
влияет на код возврата. Случай «`pwsh` не установлен» (ОВ-43) закрывается тем же путём: макрос
`llm` по-прежнему завершается с 1 и объясняет, что делать, но `make up` это больше не роняет.

**Отклонение от языка файла.** Текст предупреждения — русский, единственная такая строка в
`Makefile`. Он задан §6.3 дословно; менять формулировку значило бы отклоняться от дизайна ради
единообразия вывода.

### 4. M-4 — правило 3 линтера видит обе раскладки и любой якорь

`scripts/compose-lint.sh`: разбор `environment` по позиции (`^  <service>:` → `^    environment:`
→ `^      KEY: value`) и отдельная ветка для якоря с зашитым именем `x-platform-env` заменены
одним проходом по всему тексту файла регуляркой вида
`^\s*-?\s*([A-Za-z_][A-Za-z0-9_]*)\s*[:=]\s*(\S.*?)\s*$`.
Одна регулярка покрывает и отображение (`KEY: value`), и список (`- KEY=value`), и любой якорь.
Строки-комментарии пропускаются; ближайший объемлющий ключ по-прежнему отслеживается, но только
чтобы назвать место в сообщении (теперь ещё и с номером строки).

Фикстуры: `testdata/compose-lint/bad-*.yml` — по одной на класс пропуска, плюс `README.md` с
таблицей. Цель `make compose-lint` после прогона по настоящему `docker-compose.yml` гоняет скрипт
по каждой фикстуре и падает, если хоть одна прошла. Проверено вручную (make на машине нет):
настоящий файл — 0; `bad-env-list.yml`, `bad-anchor.yml`, `bad-port.yml`, `bad-latest.yml` — 1,
каждая на своём правиле (3, 3, 2, 1).

### 5. M-5 — цели LLM читают `.env`

Добавлен макрос `load_dotenv` (`set -a; . ./.env; set +a` по образцу §4.3) — он вызывается из
макроса `llm` (то есть из `llm-up`/`llm-down`/`llm-health`) и из `models`/`warm`. При отсутствии
`.env` цель печатает, что делать (`copy .env.example to .env and fill the MV_LLM_* block`), и
выходит с 1, а не работает с пустыми переменными. В связке с M-3 отсутствие `.env` больше не
роняет `make up`.

Побочный эффект `source` в bash: незакавыченный путь Windows теряет обратные слэши. В
`.env.example` над блоком `MV_LLM_*` добавлены две строки с требованием одинарных кавычек —
compose-парсер их тоже снимает. Прогон `set -a; . ./.env.example; set +a` под
`bash -euo pipefail` проходит.

### 6. Mi-1 — `health_code()` больше не возвращает `0000`

`curl` при отказе соединения и печатает `000` через `-w`, и выходит с 7, поэтому `|| echo 0`
дописывал второе значение. Функция переписана: код захватывается отдельной строкой, `|| true`
гасит статус, `${code:-000}` закрывает пустой ответ. Проверено на закрытом порту — `[000]`.
Следствия: ветка `up` «PID жив, порт молчит» снова перезапускает сервер, а `health` печатает
подготовленное «unreachable — the process is not running». Мёртвая альтернатива `| 0` в `case`
убрана.

### 7. Mi-2 — `minio-init` различает «уже есть» и отказ

`mc admin user add` падает и когда пользователь уже создан, и когда MinIO отверг ключ (access key
короче 3 символов, secret короче 8). Теперь в ветке отказа факт подтверждается через
`mc admin user info`: пусто → лог с подсказкой и `exit 1`, непусто → «service user exists». Так же
для `mc admin policy attach`: политика ищется в выводе `user info` через `case` (в образе `mc` нет
`grep`), и невозможность применить политику останавливает контейнер.

Проверено на стенде (отдельный проект `-p mvinit`, свой том, снесён после):

| Прогон | Результат |
|---|---|
| чистый MinIO | `service user added`, `policy readwrite attached`, код 0 |
| повтор | то же, код 0 (MinIO перезаписывает существующего пользователя) |
| секрет короче 8 символов | `cannot create service user … (access key >= 3 and secret >= 8 characters?)`, код 1 |

До правки третий случай давал «service user exists» и код 0.

### 8. Mi-3 — секреты MinIO вне argv

Ключи передаются `mc` через stdin, а не аргументами: `mc alias set ALIAS URL` и
`mc admin user add ALIAS` обе читают пару «access-key, secret-key» построчно из пайпа
(документировано в `mc … --help`, пример «piped keys»).

**Отклонение от предложенного в ревью решения.** Вариант с `MC_HOST_local` (URL вида
`http://user:pass@minio:9000`) пробовался первым и отвергнут по результату проверки на
закреплённом образе `minio/mc:RELEASE.2025-08-13T08-35-41Z`: `mc` не декодирует percent-encoding
в userinfo, поэтому percent-encoded пароль даёт `signature does not match`, а сырой пароль с
`/`, `@` или `#` ломает разбор URL (`security token invalid`). То есть `MC_HOST_*` обменял бы
видимый секрет на стек, который не стартует при сильном пароле. Пайп решает ту же задачу и
работает с любым паролем — проверено на пароле со спецсимволами.

### 9. Mi-6 — пины `legacy.Dockerfile`

- `FROM debian:bookworm-slim` → `FROM ${LEGACY_RUNTIME_IMAGE}`; в `build/versions.env` добавлен
  `LEGACY_RUNTIME_IMAGE` с дайджестом `sha256:8820…4171` (получен `docker pull` 2026-09-09).
  Правило шапки `versions.env` «no floating tags anywhere» больше не нарушается.
- Тарбол ONNX Runtime: версия и контрольная сумма вынесены в `versions.env`
  (`LEGACY_ONNXRUNTIME_VERSION=1.18.0`, `LEGACY_ONNXRUNTIME_SHA256=fa4d…04f0`), URL собирается из
  версии, после `wget` идёт `sha256sum -c -`. Сумма посчитана с релизного ассета GitHub.
- `libonnxruntime.so.1.18.0` в стадии рантайма заменён на `…so.${LEGACY_ONNXRUNTIME_VERSION}`,
  чтобы версия жила в одном месте.
- Новые аргументы прокинуты в `docker-compose.yml`: `semantic-memory` получает все три,
  `narrative-orchestrator` — только `LEGACY_RUNTIME_IMAGE` (ONNX ему не нужен, `BUILD_CGO=0`).

Сборка профиля `legacy` не перезапускалась (долгая, по указанию оркестратора): проверено
`docker compose --profile legacy config -q` и синтаксис Dockerfile.

### 10. Как проверено

| Проверка | Результат |
|---|---|
| `docker compose --env-file build/versions.env --env-file .github/ci.env config -q` (ядро + `memory`, `gpu`, `bot`, `dev`, `legacy`) | 6/6 OK |
| `bash scripts/compose-lint.sh` | `ok — 15 services, 6 rules`, код 0 |
| `bash scripts/compose-lint.sh -f testdata/compose-lint/bad-*.yml` | 4/4 код 1 |
| `go test -short -count=1 ./...` | все пакеты `ok`, включая `shared/env` |
| `golangci-lint run ./...` | `0 issues` |
| `pre-commit run --files <изменённые>` | 8 хуков Passed |
| `gitleaks git --staged --redact .` | `no leaks found` |
| `qdrant` healthy на стенде | да (отдельный проект, снесён `down -v`) |
| `minio-init` на стенде: чисто / повтор / плохой ключ | 0 / 0 / 1 |

### 11. Открытые вопросы и допущения итерации

- **Допущение.** Sha256 тарбола ONNX Runtime посчитан с самого ассета — независимой публикации
  контрольной суммы у релиза `v1.18.0` нет. Значение фиксирует «то, что скачалось 2026-09-09»;
  подмена ассета после этой даты сборку остановит, подмена до неё осталась бы незамеченной.
- **Допущение.** Дайджест `debian:bookworm-slim` придётся обновлять вручную вместе с остальными
  пинами (§9.5) — Dependabot `versions.env` не видит.
- Предупреждение M-3 печатается по-русски по §6.3 — см. отклонение в п. 3.
- Прогон `make` по-прежнему не выполнялся: `make` на машине владельца не установлен (ОВ-42).
  Рецепты проверены построчно в bash, включая новый самотест `compose-lint`.
