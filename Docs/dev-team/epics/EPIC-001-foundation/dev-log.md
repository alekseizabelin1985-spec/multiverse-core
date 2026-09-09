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
