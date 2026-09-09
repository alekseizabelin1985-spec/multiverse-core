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
