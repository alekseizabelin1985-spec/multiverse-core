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
