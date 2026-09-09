# Ревью кода EPIC-001 «Фундамент»

Формат записи: задача, номер итерации, дата, экземпляр ревьюера; границы ревью; вердикт;
проверенные пункты DoD; замечания с серьёзностью (Critical / Major / Minor / Nit) в виде
«файл:строка — суть — предлагаемая правка». Записи добавляются, старые не удаляются.
Язык — русский; код, конфиги и команды — английские.

---

## T-001 · ревью #1 · 2026-09-09 · code-reviewer#1 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `be33e93`. Ревьюировались **изменения в индексе**
(`git diff --cached`, 16 записей) — коммита нет (`git.commits: ask`). Незакоммиченные
не-staged правки других ролей (`.claude/agents/**`, `services/*/go.mod`, `.qwen/**`, `.mcp.json`,
`Docs/dev-team/{dashboard.html,journal.md,state.js}`, `services/world-generator/**`) к T-001
не относятся и не рассматривались.

Состав изменений (проверено `git diff --cached --name-status`):

| Действие | Путь |
|---|---|
| A | `.gitleaks.toml`, `.gitleaksignore`, `.pre-commit-config.yaml`, `.mcp.env.example`, `Docs/dev-team/epics/EPIC-001-foundation/dev-log.md` |
| M | `.gitignore`, `.gitattributes`, `.env.example`, `shared/oracle/README.md` |
| D | `.mcp.env`, `.claude/settings.local.json`, `examples.exe`, `semantic-memory.exe`, `services/narrative-orchestrator/cmd.exe`, `mcp_kafka.log`, `mcp_audit.log` |

Основание: `tasks.md` §1 (общий DoD) и §2 (T-001), `dev-log.md` (запись developer#1),
`architecture/infrastructure.md` v0.3 §4.5, §3.4, §4.2, §2.4, `architecture/threat-model.md`
T-34/SEC-24 (§4.5), `plan/ownership.md` §1.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 5 · Nit: 6.

Задача выполнена по §4.5 п. 1–11 дословно, порядок соблюдён, секрет из индекса выведен,
контроль (`gitleaks`) перепроверен ревьюером независимо. Два критерия DoD не выполнены
(`pre-commit run --all-files` зелёный; `gitleaks dir --redact .` → 0), но **оба недостижимы
в границах T-001** — они требуют правок в файлах других владельцев (T-002/T-003/T-004/T-008)
и решения пользователя по каталогам вне git. Разработчик это выявил, задокументировал и
эскалировал (ОВ-1, ОВ-2, ОВ-5). Возврат задачи результата не даст; вместо него — решения
tech-lead#1 / security-engineer / devops-engineer по M-2…M-5 и правка M-1 (комментарий).

### Проверенные пункты DoD

Прогоны ревьюера (только чтение; рабочее дерево и индекс не изменялись):

| Проверка | Как проверено | Результат |
|---|---|---|
| gitleaks по диапазону ветки | `gitleaks git --redact --log-opts="integration/mvp-1..HEAD" .` | **0 находок**, 1 коммит — подтверждено |
| хук на staged-изменениях T-001 | `gitleaks git --pre-commit --redact --staged .` | **0 находок** — коммит T-001 хук пройдёт |
| скан **содержимого индекса** (аналог `gitleaks dir` по `git ls-files`) | `git checkout-index -a --prefix=<tmp>/` + `gitleaks dir --config .gitleaks.toml <tmp>` | **1 находка** — ровно та, что гасится `.gitleaksignore` (`generic-api-key`, `swarm-llm-laws.md:504`); в остальных отслеживаемых файлах — ничего |
| действенность `.gitleaksignore` | тот же скан с `--gitleaks-ignore-path` на пустой каталог и без него | fingerprint без коммита **работает** (`dir`-режим); fingerprint с коммитом `be33e93` корректен (`git blame` строки 504 = `be33e93`) |
| `gitleaks dir --redact .` (критерий §4.5 п. 12) | `gitleaks dir --redact .` | **5 находок, все вне git** — см. M-3 |
| бинарники/логи/секреты в индексе | `git ls-files` + фильтр по `.exe`/`.log`/`.mcp.env`/`settings.local.json` | **пусто** |
| ключ в `shared/oracle/README.md` | `grep -c "sk-4659"` по staged-блобу; `git diff --cached | grep -c "^+.*sk-4659"` | **0 и 0**; в отслеживаемых файлах остались только усечённые упоминания длиной 7–9 символов (`Docs/**`) — не секрет |
| плейсхолдеры поставлены | staged `shared/oracle/README.md`, строки 14, 29, 68 | `sk-xxxxxxxxxxxxxxxxxxxxxxxx` (14, 29) и `export ORACLE_API_KEY="<your-key>"` (68) — по §4.5 п. 3 |
| `.env.example` ↔ `infrastructure.md` §4.2 | скрипт-сверка имён в обе стороны + сверка «закомментировано/активно» | **расхождений нет**; 56 имён `MV_*`; все секреты пусты; `OLLAMA_*` и `MV_OLLAMA_URL` закомментированы; `MV_OPENAI_API_KEY`/`MV_DEEPSEEK_API_KEY` отсутствуют (v0.3 п. 1) |
| `.gitignore` — что игнорируется | `git check-ignore -v` по 9 путям | `.env`, `.mcp.env`, `Docs/dev-team/.dashboard.port`, `.claude/worktrees/` — игнорируются; `.env.example` и `.mcp.env.example` — **не** игнорируются (негации работают); `build/versions.env`, `.github/ci.env`, `testdata/**` — не игнорируются (нужны T-004/T-008/F-10) |
| `.gitattributes` | сверка с §4.5 п. 10 | дословно; `merge=binary` **без** `-diff` (D-14) — верно |
| `.pre-commit-config.yaml` | сверка с §3.4 и пинами §2.4 | дословно; `gitleaks v8.30.1`, `golangci-lint v2.13.2`, `pre-commit-hooks v5.0.0` совпадают с §2.4; bash-зависимостей нет (`language: golang`/`python`); `.git/hooks/pre-commit` установлен |
| поведение `.gitleaks.toml` | синтетические фикстуры во временном каталоге вне репозитория | allowlist по путям срабатывает только на `^\.env\.example$`/`^\.mcp\.env\.example$` (вложенные `sub/*.env` ловятся); токены формата Telegram, отличные от плейсхолдера, **ловятся** правилом `telegram-bot-api-token`; реальные ключи allowlist не глушит |
| окончания строк новых файлов | побайтовая проверка блобов индекса и рабочих копий | все 7 добавленных/изменённых файлов — **LF**, несмотря на `core.autocrlf=true` |
| полнота удалений | `git diff --cached --diff-filter=D --name-only` | ровно 7 путей из §4.5 п. 4–5, лишнего нет; `.idea/`, `.vscode/`, `.kilo*`, `.roo*`, `.qwen/`, `memory/`, `plans/`, `reports/`, `event-model.pdf` из индекса **не выведены** — верно по U-11 |
| файлы на диске | `ls` | `.mcp.env` и `.claude/settings.local.json` остались (верно); три `.exe` удалены; `mcp_kafka.log`/`mcp_audit.log` остались (заняты процессом — отклонение 3 dev-log, приемлемо); `-p/`, `Multiverse/`, `hook-test.env` отсутствуют |
| карта владения (`ownership.md` §1) | сверка списка файлов | соблюдена: `.gitleaks.toml`, `.gitignore`, `.gitattributes`, `.env.example`, `.mcp.env.example` — EPIC-001; `shared/oracle/README.md` — предмет F-1/F-3 по §4.5 п. 3 |
| общий DoD §1 п. 1–3 (`make lint`, `go build/vet/test`, тесты) | — | **n/a** обоснованно: Go-код не менялся; предмет задачи — конфигурация инструментов, контроль — прогоны gitleaks и pre-commit |
| `pre-commit run --all-files` зелёный | dev-log §5 + разбор ревьюера | **не выполнен** — M-2 |
| `gitleaks dir --redact .` → 0 | прогон ревьюера | **не выполнен** — M-3 |
| проверка хука на Windows | dev-log §3 | выполнена в изменённом виде (токен, отличный от плейсхолдера); причина изменения подтверждена ревьюером независимо — M-4 |
| `dev-log.md` заполнен (§1 п. 5) | чтение | да: экземпляр, задача, команды, отклонения (5), открытые вопросы (5), риски; одна цифра устарела — N-6 |

### Замечания

#### Critical

Нет.

#### Major

Нет.

#### Minor

**M-1. `.gitleaksignore:5-7` — обоснование гашения не соответствует факту.**
В комментарии сказано, что `generic-api-key` цепляет «имя фича-флага в тексте таблицы
(`LLM_CLOUD_ENABLED=true`)». Фактический матч в `swarm-llm-laws.md:504` — строка таблицы
`providers/ollama` в §9.1, срабатывание даёт связка `max_tokens, num_ctx=LLM_NUM_CTX (8192)` <!-- gitleaks:allow: цитата ложного срабатывания -->
(ключевое слово `token` внутри `max_tokens` плюс присваивание). Обоснование в allowlist-артефакте
должно быть проверяемым: сейчас сопоставить комментарий с находкой нельзя.
*Правка*: заменить текст обоснования на фактический матч (строка таблицы `providers/ollama`,
`num_ctx=LLM_NUM_CTX`) и отметить, что fingerprint гасит **любую** находку `generic-api-key`
в строке 504 этого файла.

**M-2. DoD «`pre-commit run --all-files` зелёный» не выполнен (`.pre-commit-config.yaml`).**
Красны три автофиксера: `golangci-lint-fmt`, `end-of-file-fixer` (81 файл), `mixed-line-ending`
(8 файлов). Проверено ревьюером: **часть красноты — артефакт рабочего дерева, а не содержимого
git**. Блобы `docker-compose.yml`, `Makefile`, `configs/gm_*.yaml` в git уже **LF** (CRLF = 0),
CRLF есть только в рабочих копиях, выданных `core.autocrlf=true` до появления `.gitattributes`
с `eol=lf`.
*Правка*: (а) обновить рабочие копии этих 8 файлов после появления `.gitattributes` (удалить и
`git checkout --` по путям) — содержимое в git не меняется, чужое владение не затрагивается,
`mixed-line-ending` становится зелёным; (б) остальное (`end-of-file-fixer`, `golangci-lint-fmt`)
в границах T-001 недостижимо — нужно решение tech-lead#1: временный `exclude:` в
`.pre-commit-config.yaml` до T-003 либо формальная отсрочка критерия §4.5 п. 12 с записью
в `tasks.md`.

**M-3. DoD «`gitleaks dir --redact .` → 0» не выполнен; скомпрометированный ключ остаётся на диске.**
Прогон ревьюера — 5 находок, все вне git: `.env` (2) и `.claude/worktrees/frosty-bell/**` (3, в том
числе копия старого `shared/oracle/README.md` с тремя вхождениями ключа и копия `.env`). Каталог
покрыт `.gitignore` (`git check-ignore` подтверждает), в индекс не попадёт — поэтому не Major.
*Правка*: (а) решение пользователя об удалении `.claude/worktrees/frosty-bell/` (U-1 — агент сам
не удаляет); (б) переформулировать критерий §4.5 п. 12 на скан **содержимого `git ls-files`**
(предложение ОВ-2 п. б/в) — ревьюер воспроизвёл такой скан через `git checkout-index`, он
детерминирован и годится для `make secrets-scan` (T-008).

**M-4. Конфликт критерия §3.4/§4.5 и allowlist подтверждён независимо (ОВ-1).**
Синтетическая проверка: файл вне `*.example` со строкой `MV_TELEGRAM_BOT_TOKEN=123456789:AA`
+ 33 символа `x` **проходит** gitleaks (гасится `regexes = ['''123456789:AA[x]{33}''']`), а токен
того же формата с иным значением — ловится (`telegram-bot-api-token`). То есть буквальный
негативный тест из DoD негативным тестом быть не может.
*Правка* (security-engineer): либо убрать `123456789:AA[x]{33}` из `regexes` — плейсхолдеры в
`.env.example`/`.mcp.env.example` уже покрыты allowlist по путям, а вне примеров глушить шаблон
Telegram-токена незачем; либо переформулировать критерий в §3.4, §4.5 п. 12 и `tasks.md` T-001
(«токен формата Telegram, отличный от плейсхолдера»). До правки allowlist остаётся узким и
реальные токены не глушит — это проверено.

**M-5. Автофиксеры pre-commit изменят содержимое переносимых файлов в T-002/T-003.**
`golangci-lint-fmt` (`types: [go]`) и `end-of-file-fixer` правят staged-файлы на коммите. В T-002
`git mv` переносит большой объём as-is кода при требовании U-1 «ничего не менять», а `git log
--follow` входит в DoD T-002. Побочный эффект уже виден в этой задаче: в `shared/oracle/README.md`
хук дописал перевод строки в конце файла (N-3).
*Правка* (devops-engineer + tech-lead#1, до старта T-002): добавить в `.pre-commit-config.yaml`
`exclude: '^services/_archive/'` (когда каталог появится) либо зафиксировать в `tasks.md`/dev-log
допустимость `git commit --no-verify` для коммита-переноса с последующим ручным прогоном gitleaks.

#### Nit

**N-1. `.env.example` — не-ASCII в комментариях заменены и два комментария укорочены.**
`≥ 16 символов` → `>= 16 символов`, `MV_LLM_NUM_CTX × число слотов` → `x число слотов`; из
комментариев `MV_LLM_THREADS`/`MV_LLM_BATCH_SIZE` убраны «(i9-13900)» и «значение из командной
строки владельца». На поведение не влияет; имена переменных сверены с §4.2 — расхождений нет.

**N-2. `.env.example` — `MV_LLM_BIN`, `MV_LLM_MODEL_FILE`, `MV_LLM_MODELS_DIR`,
`MV_LLM_SLOT_SAVE_PATH` оставлены пустыми** (отклонение 2 dev-log). Решение разумно (файл общий
для всех окружений, в §4.2 показаны пути одной машины), но `scripts/llm-server.ps1` (T-008) без
образца значений менее очевиден. Подтвердить devops-engineer при T-008.

**N-3. `shared/oracle/README.md` — четвёртое изменение сверх §4.5 п. 3**: добавлен перевод строки
в конце файла (исчез `\ No newline at end of file`) — результат `end-of-file-fixer`. Безвредно.

**N-4. `.gitignore:6` `!.mcp.env.example` — фактически no-op**: шаблон `.mcp.env` (строка 5)
не покрывает `.mcp.env.example`, негации нечего отменять. Оставлено дословно по §4.5 п. 8,
вреда нет; при следующей правке §4.5 — убрать либо заменить на `.mcp.env*` + негацию.

**N-5. `.gitattributes:9` `testdata/recordings/*.jsonl merge=binary` привязан к корню** — шаблон
содержит слэш, поэтому вложенные `**/testdata/recordings/*.jsonl` не покрываются. Соответствует
§4.5 п. 10 дословно и карте владения (`testdata/recordings/**` — корневой каталог, EPIC-005);
при появлении вложенных записей шаблон расширить.

**N-6. `dev-log.md` §4 и ОВ-2 — цифры устарели**: на момент ревью `gitleaks dir --redact .` даёт
**5** находок (не 22), на диске остался **один** каталог `.claude/worktrees/frosty-bell` (не 13).
Освежить при следующей итерации, чтобы ОВ-2 не выглядел крупнее, чем есть.

### Предложения в бэклог

1. `make secrets-scan` (T-008): `gitleaks git` по диапазону + `gitleaks dir` по содержимому
   `git ls-files` (через `git checkout-index --prefix`), а не по рабочему каталогу — критерий
   станет детерминированным и перестанет зависеть от `.env` и остатков worktree (закрывает M-3).
2. `ownership.md` §1: в строку корневых конфигов EPIC-001 добавить `.pre-commit-config.yaml`
   и `.gitleaksignore` — сейчас они не поименованы, хотя создаются в F-1.
3. Runbook F-9 (T-019): зафиксировать, что `go install` gitleaks идёт по модулю
   `github.com/zricethezav/gitleaks/v8` (путь `gitleaks/gitleaks/v8` даёт
   `version constraints conflict`), и требование добавить каталог скриптов Python в `PATH`.
4. Устойчивое гашение ложного срабатывания `swarm-llm-laws.md:504` — inline `# gitleaks:allow`
   правкой владельца (architect#2) вместо fingerprint по номеру строки (ОВ-3): при правке файла
   выше строки 504 fingerprint молча перестаёт действовать.
5. F-7 (T-012): в job `security` использовать `GITLEAKS_VERSION` из `build/versions.env` и тот же
   `.gitleaks.toml`; проверить, что `.gitleaksignore` учитывается в CI (gitleaks ищет его
   в корне репозитория).

### Риски и допущения

- Ревью выполнено по **индексу**, не по коммиту. Если перед коммитом сработают автофиксеры
  pre-commit, содержимое может измениться; проверено, что на текущем составе staged-файлов
  хук `gitleaks --pre-commit --staged` даёт 0 находок, а прочие хуки на этих файлах зелёные
  (dev-log §4).
- `.gitattributes` вводит `eol=lf`: при первом обновлении рабочих копий
  `*.yml`/`*.yaml`/`*.toml`/`*.sh`/`Makefile` окончания строк в рабочем дереве сменятся на LF.
  Содержимое в git не изменится (блобы уже LF) — проверено побайтово для `docker-compose.yml`,
  `Makefile`, `configs/gm_*.yaml`.
- Ключ `ORACLE_API_KEY` остаётся в истории git и в копии worktree на диске — считается
  скомпрометированным до отзыва пользователем (ОВ-4). `git filter-repo` в T-001 обоснованно
  не выполнялся (§4.5 п. 13).
- Проверка «хук отклоняет коммит» ревьюером не повторялась (потребовала бы записи в индекс);
  принят результат dev-log §3 плюс независимо воспроизведённое поведение allowlist (M-4).
- `pre-commit` и `gitleaks` установлены только у текущего пользователя Windows; для CI версии
  придут из `build/versions.env` (T-004). Сгенерированный `.git/hooks/pre-commit` содержит
  абсолютный путь к Python с кириллицей в имени профиля — файл машинно-локальный, в git не входит,
  на другой машине пересоздаётся `pre-commit install`.

---

## T-002 · ревью #1 · 2026-09-09 · code-reviewer#1 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `04a3a15`. Ревьюировались **изменения в индексе**
(`git diff --cached -M`, 89 записей) — коммита нет (`git.commits: ask`). Незастейдженные правки
пользователя (`services/*/go.mod`, `.claude/**`, `.qwen/**`, `.mcp.json`, `mcp_kafka.log`,
`Docs/dev-team/{dashboard.html,journal.md,state.js}`) к T-002 не относятся и не рассматривались.

Состав изменений (`git diff --cached -M --name-status`, сводка по буквам): **60 R** (все `R100`),
**26 A**, **3 M**, **0 D**.

| Действие | Путь |
|---|---|
| R (60) | `services/_archive/**` (56 файлов), `build/Dockerfile → build/legacy.Dockerfile`, `Dockerfile → services/_archive/build/Dockerfile.root`, 3 сервисных `Dockerfile`, 6 `configs/gm_*.yaml` |
| A (26) | `services/_archive/{README.md,go.mod}`, 15 × `ARCHIVED.md`, 8 × `services/<frozen>/FROZEN.md`, `Docs/archive/README.md` |
| M (3) | `.pre-commit-config.yaml`, `go.work`, `dev-log.md` |

Основание: `tasks.md` §1 (общий DoD) и §2 (T-002), `dev-log.md` (запись developer#1 · T-002,
отклонения 1–5, ОВ-1…ОВ-5), `architecture/infrastructure.md` v0.3 §4.6 и §10 (строка F-3),
`architecture/components/foundation.md` v0.2 §11, `plan/ownership.md` §1,
`Docs/dev-team/journal.md` (запись «Решения оркестратора по ОВ T-002» — принята как данность).

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 4 · Nit: 6.

Перенос выполнен точно по `foundation.md` §11 и `infrastructure.md` §4.6: ни одного `delete`
в индексе, все 60 переименований распознаны как `R100` (содержимое байт в байт), состав
перенесённого совпадает со списком §11 «в обе стороны» — лишнего нет, забытого нет. Корневая
сборка зелёная и архив ей невидим. Сопроводительные документы (`README.md`, 15 `ARCHIVED.md`,
8 `FROZEN.md`) заполнены по шаблону §4.6 полностью; выборочная сверка полей «последний рабочий
коммит» с `git log -1 -- <путь>` дала совпадение по всем 16 проверенным путям архива и по всем 8
замороженным сервисам. Секретов нет (`gitleaks` перепроверен ревьюером). Замечания касаются
качества сопроводительных текстов и одного вопроса конфигурации `pre-commit`; ни одно не
блокирует T-003.

Отдельно: решение оркестратора по ОВ-1 (`shared/agent/filter.go` → архив) на момент ревью
**не исполнено и не привязано к задаче**, а его формулировка неполна — см. Minor M-4. Это не
дефект сделанного в T-002 (файл в объём задачи не входил), но требует назначения до старта T-003.

### Проверенные пункты DoD

Прогоны ревьюера (только чтение; рабочее дерево и индекс не изменялись):

| Проверка | Как проверено | Результат |
|---|---|---|
| ничего не удалено (U-1) | `git diff --cached --diff-filter=D --name-only` | **пусто**; в `--name-status` только `A`/`M`/`R` |
| перенос, а не удаление+добавление | `git diff --cached -M --summary` + подсчёт `rename` / `rename ... (100%)` | **60 и 60**; записей `rename` не-100 % нет |
| содержимое перенесённого не изменено | статус `R100` по всем 60 записям (git считает similarity по блобам); содержательных строк в патче нет | подтверждено |
| состав переноса = `foundation.md` §11 | построчная сверка §11 ↔ `--summary`; плюс `ls shared/`, `ls services/`, `ls -d configs`, `ls Dockerfile` | совпадает: в `shared/` остались `agent`, `entity`, `eventbus`, `jsonpath`; в `shared/agent/tools` — только `registry.go` + `go.mod`; лишнего не перенесено |
| архив невидим корневому модулю | `go list ./...` | 3 пакета (`shared/entity`, `shared/jsonpath`, `shared/jsonpath/examples`); вхождений `_archive` — **0** |
| `go build ./... && go vet ./...` в корне | прогон ревьюера | **зелёные** (замечание `test_minio.go:44` ушло вместе с файлом) |
| `go test -short -count=1 ./...` | прогон ревьюера | **зелёный** (`shared/jsonpath` ok, у прочих нет тестов); `-race` — n/a (нет cgo/gcc), принято решением оркестратора по ОВ-5 |
| `go.work` валиден и не ссылается на архив | `go work edit -json`; `git diff --cached -- go.work` | валиден; убраны ровно 6 записей перенесённых модулей, `./services/_archive` не добавлен |
| изоляция архива | `services/_archive/go.mod` = `module multiverse-core.io/archive`, без `require` | соответствует §4.6 |
| `ARCHIVED.md` в каждом каталоге | подсчёт `find services/_archive -name ARCHIVED.md` + автосверка 6 полей шаблона §4.6 в каждом файле | **15 файлов, во всех 15 все поля заполнены** |
| `FROZEN.md` = 8 | `ls services/*/FROZEN.md`; сверка состава со списком §11 | **8**, состав совпадает; `rule-engine`/`entity-manager`/`game-service`/`narrative-orchestrator`/`semantic-memory` `FROZEN.md` не получили — верно |
| достоверность полей «последний рабочий коммит» | `git log -1 --format=%h -- <исходный путь>` по 16 путям архива и по 8 замороженным сервисам | **совпало везде**; все упомянутые хэши существуют в истории |
| индекс `services/_archive/README.md` полон | сверка 27 строк таблицы с 60 файлами переноса | все перенесённые пути перечислены с причиной, коммитом и эпиком возврата (кроме `build/legacy.Dockerfile` — N-1) |
| секретов нет | `gitleaks git --staged --redact .` | **0 находок** (`no leaks found`, ~68 КБ) |
| плейсхолдер ключа доехал до архива (U-5) | `grep -n "sk-" services/_archive/shared/oracle/README.md` | строки 14/29 — `sk-xxxxxxxxxxxxxxxxxxxxxxxx`, строка 68 — `<your-key>`; реального ключа нет |
| `pre-commit` на файлах задачи | `pre-commit run --files <8 файлов задачи>` | зелёный, ни один файл не изменён (`git status` до и после совпадает) |
| действенность `exclude` + `always_run` | `pre-commit run --files services/_archive/configs/gm_defaults.yaml services/_archive/shared/rules/engine.go` | `Detect hardcoded secrets` — **Passed** (хук не пропущен), все остальные — **Skipped**: заявленное поведение подтверждено |
| карта владения (`ownership.md` §1) | сверка путей изменений со строками §1 | нарушений нет: `services/_archive/**` и `FROZEN.md` — EPIC-001 F-3, `build/**` и корневые конфиги — EPIC-001, `Docs/archive/**` — F-9/tech-writer (создание индекса прямо предписано формулировкой T-002) |
| общий DoD §1 п. 1, 3 | — | **n/a** и зафиксировано в dev-log: `.golangci.yml` появляется в T-003, Go-код только перемещался, нового поведения нет |

Отклонения из dev-log §7 проверены отдельно:

1. **`build/Dockerfile → build/legacy.Dockerfile`** — обосновано: `infrastructure.md` §10 (строка F-3)
   и раздел про образы (строка 206) прямо это предписывают, `build/**` — владение EPIC-001.
   `R100`, история цела.
2. **`always_run: true` у `gitleaks`** — техническая необходимость подтверждена экспериментом
   (см. строку таблицы выше); о более аккуратном варианте — M-1.
3. **`go.mod` в подкаталогах архива не созданы** — цель формулировки T-002 («чтобы корневой `./...`
   их не видел») достигнута одним `services/_archive/go.mod`; побочный эффект — M-2.
4. **Пустой `configs/` удалён** — U-1 не затрагивает (файлов в каталоге не осталось, git пустые
   каталоги не хранит); проверено: `git diff --cached --diff-filter=D` пуст.
5. **Профиль `legacy` не сломан сверх ожидаемого**: `services/narrative-orchestrator` импортирует
   `shared/{config,minio,oracle,spatial}` — у всех четырёх собственные `go.mod` уехали вместе
   с кодом и путь модуля сохранён (`multiverse-core.io/shared/...`), поэтому механизм `replace`
   из решения ОВ-2 (T-008) работоспособен. `rule-engine` (`shared/minio`) — то же самое.

### Замечания

#### Critical

Нет.

#### Major

Нет.

#### Minor

**M-1. `.pre-commit-config.yaml:8` — верхнеуровневый `exclude` шире, чем нужно, и потребовал
костыля.** `exclude: '^services/_archive/'` объявлен на уровне файла, поэтому выключает для архива
не только три автофиксера (ради которых вводился), но и `check-added-large-files`, `check-yaml`,
`check-merge-conflict`; а `gitleaks` пришлось спасать `always_run: true`. Проверено ревьюером:
на прогоне только по архивным файлам все хуки, кроме gitleaks, — `Skipped`. Практический риск:
(а) бинарник > 1 МБ, добавленный в `services/_archive/**`, хуком уже не ловится (архив «только
чтение», но правило перестало действовать молча); (б) каждый новый хук в будущих задачах
автоматически перестаёт видеть архив, о чём легко забыть. **Правка**: убрать верхнеуровневый
`exclude` и поставить `exclude: '^services/_archive/'` пофайлово у `golangci-lint-fmt`,
`golangci-lint`, `end-of-file-fixer`, `mixed-line-ending`; тогда `always_run: true` у `gitleaks`
не нужен, а `check-added-large-files` продолжает работать. Владелец решения — devops-engineer;
`infrastructure.md` §3.4 (эталон конфига) при этом надо дополнить — сейчас он про `exclude`
не знает, и следующая сверка файла с §3.4 тихо вернёт прежний вид.

**M-2. `services/_archive/shared/{schema,redis,rules,intent,tinyml}` — без собственных `go.mod`
(отклонение 3 dev-log)**, вопреки прямой формулировке T-002 («…и собственные `go.mod` подкаталогов»)
и схеме §4.6. Заявленная в той же формулировке цель («чтобы корневой `./...` их не видел»)
достигнута, корневая сборка чиста — поэтому Minor, а не Major. Побочный эффект: эти пакеты стали
частью модуля `multiverse-core.io/archive`, то есть их импортный путь сменился
(`multiverse-core.io/shared/rules` → `.../archive/shared/rules`), и механизм `replace` из решения
ОВ-2 для них неприменим: `replace multiverse-core.io/shared/rules => ../../services/_archive/shared/rules`
не сработает, так как в каталоге нет `go.mod` с этим путём модуля. Для `narrative-orchestrator`
это не блокер (все четыре его архивные зависимости `go.mod` сохранили), но для `entity-actor` и
`evolution-watcher` (импортируют `shared/{intent,rules,tinyml,redis}`) восстановление сборки в их
эпике потребует ручного возврата файлов. **Правка (решение — architect#1)**: либо добавить пять
файлов `go.mod` (`module multiverse-core.io/shared/<имя>` + `go 1.24`, без `require`) — по две
строки, содержимое перенесённого кода не трогается; либо зафиксировать в
`services/_archive/README.md`, что возврат этих пакетов делается только `git mv` обратно.

**M-3. `services/_archive/README.md`, раздел «Правила», п. 1 — утверждение не соответствует
текущему состоянию репозитория.** Написано как факт: архив «исключён из `.golangci.yml`,
`.dockerignore`, `Makefile`, `docker-compose.yml` и `CODEOWNERS`». Проверено: `.golangci.yml`,
`.dockerignore` и `.github/CODEOWNERS` в репозитории **не существуют** (создаются в T-003/T-004/T-012),
а `Makefile` (строки 10, 13, 31) и `docker-compose.yml` (строки 101…283, 302, 316, 330) наоборот
**ссылаются на перенесённые пути**. Индекс архива — документ длительного хранения, его будут читать
как описание фактического положения дел. **Правка**: переформулировать в будущее время с указанием
задач: «исключается из … — T-003 (`.golangci.yml`), T-004 (`.dockerignore`, `docker-compose.yml`),
T-008 (`Makefile`), T-012 (`CODEOWNERS`)».

**M-4. Решение оркестратора по ОВ-1 не исполнено и неполно по составу файлов.** Журнал
(`journal.md`, запись «Решения оркестратора по ОВ T-002») постановил: `shared/agent/filter.go`
уходит в архив вместе с `shared/rules`. На момент ревью файл на месте, и модуль `shared/agent`
не собирается — проверено: `cd shared/agent && go build ./...` →
`filter.go:11:2: no required module provides package multiverse-core.io/shared/rules`. Кроме того,
на `shared/rules` завязан **ещё один** файл, в ОВ-1 не упомянутый —
`shared/agent/e2e_dark_forest_test.go:13`; без него `go vet`/`go test` по объединённому модулю
после T-003 останутся красными, даже если перенести только `filter.go`. Формально это вне объёма
T-002 (обоих файлов нет ни в §11, ни в списке файлов задачи), поэтому не Major, — но T-003 обязан
влить `shared/agent` в единый модуль и упрётся в это на первом же `go build`. **Правка**:
назначить исполнителя и задачу — либо доитерация T-002 (`git mv` обоих файлов в
`services/_archive/shared/agent/` + строка в `README.md` и новый `ARCHIVED.md`), либо явный пункт
в описании T-003 с перечислением **обоих** файлов.

#### Nit

**N-1. `services/_archive/README.md` — нет строки про `build/Dockerfile → build/legacy.Dockerfile`.**
Переименование сделано в этой же задаче (обоснованно, см. отклонение 1) и зафиксировано только
в `dev-log.md`. DoD требует, чтобы индекс перечислял «все перенесённые пути»; формально файл
не в архиве, но читателю индекса эта строка нужна. Добавить её в таблицу «Что в архив **не**
уходит» с пометкой «переименован для профиля `legacy`, T-008».

**N-2. `ARCHIVED.md` не в каждом каталоге буквально.** Нет файлов в
`services/_archive/build/services/{entity-actor,evolution-watcher,rule-engine}/` и рядом с
`services/_archive/test_minio.go` (корень архива). Фактически они покрыты
`services/_archive/build/ARCHIVED.md` (в нём перечислены все четыре Dockerfile) и `README.md`.
Трактовка §4.6 разумная; при желании довести до буквы — три однострочные ссылки на родительский
`ARCHIVED.md`.

**N-3. `dev-log.md` §8, последний абзац — занижен масштаб ОВ-3.** Указано, что
`docker-compose.yml` ссылается на перенесённые пути в строках 302/316/330. Фактически он ещё
в **девяти** местах (101, 121, 139, 163, 185, 196, 208, 220, 283) собирает из `./build/Dockerfile`,
которого больше нет (переименован в `legacy.Dockerfile`). На принятое решение ОВ-3 («дефект
не заводить») это не влияет, но T-004 должен знать, что чинить надо не три строки, а двенадцать.

**N-4. `.github/workflows/validate-blueprints.yml:9,15,48` фильтрует по `configs/gm_*.yaml`** —
путь больше не существует, триггер мёртв. Workflow удаляется в T-012 (F-7); действий не требуется,
отмечено для полноты картины «что сломал перенос».

**N-5. Корневой `go.mod` сохранил зависимости уехавшего в архив кода** — `xeipuuv/gojsonschema`
(был нужен `shared/schema`), `yalue/onnxruntime_go` (`shared/tinyml`), `minio-go/v7`, `kafka-go`:
в оставшихся пакетах корневого модуля (`shared/entity`, `shared/jsonpath`) они не импортируются
(проверено). `go mod tidy` в T-002 намеренно не запускался — файл переписывается в T-003;
отмечено как вход для T-003.

**N-6. Исполнитель.** По `tasks.md` §2 T-002 — developer#2 (пара — tech-writer), в `dev-log.md`
запись сделана от имени developer#1. На результат не влияет; учесть в учёте слотов подволны 0.1.

### Предложения в бэклог

1. `infrastructure.md` §3.4: внести в эталон конфига решение по `exclude`/архиву (M-1), иначе
   следующая сверка «файл ↔ §3.4» откатит правку T-002.
2. `infrastructure.md` §4.6: уточнить формулировку про `go.mod` подкаталогов — обязателен он
   только там, где был в исходном коде, или требуется всегда (M-2). Сейчас текст §4.6 и текст
   T-002 читаются по-разному.
3. T-003: добавить в описание явный пункт «архивировать `shared/agent/{filter.go,
   e2e_dark_forest_test.go}` перед слиянием `shared/agent` в единый модуль» (M-4), если решение
   ОВ-1 не исполняется доитерацией T-002.
4. `journal.md`, решение ОВ-2: формулировка «`rule-engine` заморожен (`FROZEN.md`)» не соответствует
   артефактам — по §11 и `ownership.md` §1 `rule-engine` не заморожен, а является источником для
   переписывания в EPIC-002 (T-064), и `FROZEN.md` не получал (проверено: 8 файлов `FROZEN.md`,
   `rule-engine` среди них нет). Вывод решения («действий не требуется») верен — поправить стоит
   только обоснование.
5. T-019 (F-9): `Docs/archive/README.md` уже содержит готовый список кандидатов — использовать
   как вход, а не составлять заново.

### Риски и допущения

- Ревью выполнено по **индексу**, не по коммиту. `git log --follow` по новым путям станет
  проверяемым только после коммита; сейчас перенос подтверждается статусом `R100` в индексе.
  Требование dev-log §10 «коммитить всё staged одним коммитом» существенно: выборочный коммит
  частями ухудшит распознавание переименований и обесценит U-1.
- `.gitattributes` из T-001 вводит `eol=lf`. Перенесённые `configs/gm_*.yaml` сейчас с CRLF;
  при первой нормализации они дадут содержательный diff внутри архива. `exclude` в `pre-commit`
  от этого не защищает (нормализацию делает git, а не хук).
- Поле «Коммит архивации» во всех 24 документах содержит ссылку на T-002 и базу `04a3a15`,
  но не хэш коммита переноса — принято решением оркестратора по ОВ-4.
- `-race` локально не проверялся (нет cgo/gcc) — принято решением по ОВ-5, проверка уходит в CI (T-012).
- Модули рабочего пространства, переставшие собираться (`entity-actor`, `evolution-watcher`,
  `universe-genesis-oracle`, `world-generator`, `narrative-orchestrator`, `rule-engine`,
  `shared/agent`), перепроверены ревьюером; для шести из семи это ожидаемое поведение по §11
  и решениям ОВ-2/ОВ-3, для `shared/agent` — M-4.

---

## T-003 · ревью #1 · 2026-09-09 · code-reviewer#1 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `5a20bb8`. Ревьюировались **изменения в индексе**
(`git diff --cached`, 45 записей) — коммита нет (`git.commits: ask`). Незастейдженные правки
пользователя (`.claude/**`, `.qwen/**`, `.mcp.json`, `mcp_kafka.log`, `services/*/go.mod`) к T-003
не относятся и не рассматривались.

Состав изменений (`git diff --cached --name-status -M`):

| Действие | Путь |
|---|---|
| A | `.golangci.yml`, `build/Dockerfile`, `cmd/multiverse/{main,serve,contexts,health,db,main_test}.go`, `shared/clock/{clock,manual,clock_test}.go`, `shared/runtime/{runtime,http,lifecycle,runtime_test,http_test}.go`, `services/_archive/shared/agent/ARCHIVED.md` |
| M | `go.mod`, `go.sum`, `.gitattributes`, `services/_archive/README.md`, `shared/agent/**` (формат), `shared/eventbus/**` (формат), `shared/jsonpath/accessor.go`, `Docs/dev-team/epics/EPIC-001-foundation/dev-log.md` |
| D | `go.work`, `go.work.sum`, `shared/eventbus/go.mod`, `shared/agent/go.mod`, `shared/agent/tools/go.mod` |
| R100 | `shared/agent/{filter.go,e2e_dark_forest_test.go}` → `services/_archive/shared/agent/` |

Основание: `tasks.md` §1 (общий DoD) и §3 (T-003), `dev-log.md` (запись developer#1, §6 отклонения
1–8, §8 ОВ-6…ОВ-10), `journal.md` (решения оркестратора по ОВ T-003 — считаются принятыми),
`architecture/components/foundation.md` v0.2 §1–§4, §11, §12, §14, `architecture/infrastructure.md`
v0.3 §2.1–§2.4, §4.2, `architecture/contracts.md` §0, C-06, `epics/EPIC-001-foundation/design.md`
§4.1, §6, ADR-001 + доп. п. 1–3, 5, 7, 8, ADR-009 п. 9, `plan/ownership.md` §1.

Прогоны выполнены ревьюером с `GOFLAGS=-buildvcs=false`, `GOTOOLCHAIN=auto` (`go version` в модуле —
`go1.26.8 windows/amd64` при локальном Go 1.25.3). `-race` не проверялся (нет gcc — принято по ОВ-5);
`docker build` не повторялся (по указанию оркестратора) — `build/Dockerfile` оценён статически.

### Вердикт

**Вернуть** — 1 Major (`runtime.AdminOnly` расходится с C-06 / ADR-009 п. 9 и с enum `actor_kind`).
Critical — 0. Все прогоны DoD зелёные, каркас работоспособен и воспроизведён ревьюером вживую;
правка по M-1 точечная (множество разрешённых значений плюс одна строка ОВ архитектору),
доитерация, а не переделка. Остальные 8 Minor и 11 Nit на вердикт не влияют.

### Проверенные пункты DoD

**Общий DoD §1**

| # | Критерий | Результат |
|---|---|---|
| 1 | `make lint` чист | ✔ `golangci-lint run ./...` (v2.13.2) → `0 issues` |
| 1 | `gofmt`/`goimports` без диффа | ✔ по модулю: все 168 отслеживаемых `.go` в LF, `golangci-lint fmt` чист. `gofmt -l .` по репозиторию даёт 33 файла — **все** в `services/_archive/**` и замороженных/legacy сервисах (вне модуля, исключены в `.golangci.yml`), это соответствует §11/§2.1 |
| 2 | `go build ./... && go vet ./...` | ✔ зелёные |
| 2 | `go test -short -count=1 ./...` | ✔ 6 пакетов `ok`, 4 `[no test files]` |
| 2 | `-race` | не проверялся (нет cgo/gcc) — принято по ОВ-5 → T-012. Точки внимания для CI: `clock.realTicker.Stop` (см. Mi-8), `runtime.HTTP.errored`, `clock.Manual` |
| 3 | Тесты в той же задаче | ✔ `shared/clock` (8 тестов, включая детерминизм и негативные), `shared/runtime` (13, включая цикл, неизвестное имя, откат `StartAll`), `cmd/multiverse` (10). Не покрыт `serve()` — Mi-7; `shared/entity` без тестов — принято по ОВ-10 → T-011 |
| 4 | `make secrets-scan` | ✔ `gitleaks git --staged --redact .` → `no leaks found`. `gitleaks dir` по рабочей копии — 5 находок вне индекса, принято по ОВ-8 (цель переопределяется в T-008) |
| 5 | `dev-log.md` заполнен | ✔ экземпляр, задача, что сделано, §6 отклонения 1–8, §7 правки as-is, §8 ОВ-6…ОВ-10, §10 риски |
| 6 | Поведение по критериям приёмки | ✔ воспроизведено ревьюером (таблица ниже) |
| 7 | Владение и контракты | ✔ по `ownership.md` §1 все пути в границах EPIC-001; правки в `shared/{eventbus,jsonpath}` — только форматирование и `reflect.Ptr` → `reflect.Pointer`, семантики не меняют (проверено `git diff --cached -w`). **Исключение — M-1**: `AdminOnly` (C-06, общий код EPIC-001 → system-architect) вводит значение `actor_kind`, которого нет в контракте, без запроса архитектору |
| 8 | CI зелёный | n/a до T-012 (F-7) |

**DoD задачи T-003**

| Критерий | Результат |
|---|---|
| `go mod tidy` не даёт диффа | ✔ (прогнан ревьюером, `git diff` по `go.mod`/`go.sum` пуст) |
| `go mod verify` | ✔ `all modules verified` |
| `go.mod`: `module multiverse-core.io`, `go 1.26`, `toolchain go1.26.8` | ✔ |
| Лишних зависимостей нет | ✔ N-5 ревью T-002 закрыт: `gojsonschema`, `onnxruntime_go`, `minio-go/v7`, `testify` и хвост ушли (`go.sum` −219 строк). Остались `google/uuid`, `segmentio/kafka-go v0.4.51` (пин §2.4), `gopkg.in/yaml.v3` — все импортируются |
| `go.work`/`go.work.sum` и три `go.mod` удалены | ✔ статус `D` |
| История сохранена | ✔ `filter.go` и `e2e_dark_forest_test.go` — `R100` (`--find-renames=100%`) |
| `go list ./...` без `_archive` и замороженных | ✔ 10 пакетов; `grep -c _archive` = `0` |
| `--contexts=all --bus=memory` → `/health` | ✔ `{"status":"ok","details":{"contexts":{gateway,laws,llm,mechanics,memory,state,swarm}}}` (порт 8097) |
| `--contexts=state,gateway` поднимает только их | ✔ ровно два имени в `details.contexts` (порт 8098); порядок старта `state,gateway` — регистрационный, совпадает с foundation §2 |
| `health --url` для distroless | ✔ печатает `ok`, exit 0; тест покрывает 200/503/битый JSON/недоступный порт |
| `runtime.New` соблюдает `DependsOn` | ✔ `TestNewOrdersByDependsOn` + независимость от порядка аргументов + игнорирование зависимостей вне выборки |
| `clock.Manual`/`ManualTimers` детерминированы | ✔ `TestManualIsDeterministic` (два прогона сравниваются поэлементно) |
| `AdminOnly` отклоняет без `X-Actor-Kind` | ✔ отклоняет, но множество разрешённых значений неверно — **M-1** |
| `docker build` собирает три бинарника | частично, обосновано: существует только `cmd/multiverse`; `mvctl` — T-010, `telegram-bot` — EPIC-004. `COPY --from=builder /out/ /` универсален и не потребует правки (отклонение §6 п. 4). Образ ревьюером не пересобирался |
| Go 1.26 локально зафиксирован в `dev-log` | ✔ 1.25.3 + тулчейн `go1.26.8` (`GOTOOLCHAIN=auto`), подтверждено ревьюером |
| `.gitattributes` `*.go text eol=lf` (ОВ-6 принято) | ✔ содержательного диффа по коду не даёт: 168/168 отслеживаемых `.go` в LF, `git diff --cached -w` по as-is пакетам пуст, кроме трёх безобидных правок (ниже) |
| `pre-commit run --files <файлы задачи>` | ✔ 8 хуков `Passed` (gitleaks, golangci-lint, golangci-lint-fmt, mixed line ending, …) |

Содержательные (не пробельные) правки в as-is коде — три, все безопасны:
`shared/agent/md_parser.go` (порядок импортов, `(string)` → `string`),
`shared/eventbus/types.go` (пустая строка в группе импортов),
`shared/jsonpath/accessor.go:163` (`reflect.Ptr` → `reflect.Pointer` — алиас той же константы).

### Замечания

#### Major

**M-1. `shared/runtime/http.go:16-21,90-103` — `AdminOnly` расходится с C-06 и с enum `actor_kind`;
штатный путь оператора получит `403`.**
Реализация допускает `X-Actor-Kind` из множества `{"ci", "operator"}`. Обе половины неверны:

1. `operator` **не является** значением `actor_kind`. Enum конверта — `human|ci|sim|system`
   (`contracts.md` C-01, ADR-007 п. 1). `operator` — это `client_id`: `gateway-and-bot.md` §868
   задаёт `MV_GATEWAY_CLIENTS="telegram-bot:telegram:human;ci-harness:ci:ci,sim;operator:*:human"`
   в формате `client_id:platform:allowed_actor_kinds`.
2. Как следствие, у клиента `operator` разрешённый `actor_kind` — **`human`**, и именно `human`
   придёт на `core`, когда gateway проксирует `/v1/admin/*` («заголовки клиента пробрасываются»,
   `gateway-and-bot.md` §387). Текущая проверка вернёт такому запросу `403`, то есть штатный доступ
   оператора к `/v1/admin/agents*` и `/v1/admin/llm/usage` (C-06, `contracts.md` §6) закрыт.

Формулировка `foundation.md` §3 «только `X-Actor-Kind: ci`/оператор» действительно двусмысленна,
но ADR-009 п. 9 читается однозначно: «`X-Client-Id` из списка, `X-Actor-Kind: ci|sim` **только для
разрешённых клиентов**» — решение принимается по **паре** заголовков, а не по одному.
Вес: `shared/runtime` — общий код (`ownership.md` §1: «EPIC-001 → system-architect»), а `AdminOnly`
поставляется как часть контракта C-06 трём эпикам (`design.md` §6). Ошибка уйдёт в EPIC-003
(маршруты) и EPIC-004 (прокси) вместе с зелёным unit-тестом, который её фиксирует
(`shared/runtime/http_test.go:70-71`).

**Правка**: (а) проверять пару — `X-Client-Id` по списку допущенных к admin клиентов **и**
`X-Actor-Kind` из enum (минимально в T-003: разрешить `ci` и `sim` вместо выдуманного `operator`,
либо принимать `human` только при `X-Client-Id: operator`); (б) убрать константу `ActorKindOp`
или переименовать в `ClientIDOperator`; (в) поскольку это решение по C-06 — завести ОВ на
system-architect и зафиксировать выбранный вариант в `dev-log.md` §8 (сейчас вопрос не поднят
вовсе). Тест `http_test.go` перевести на новые значения, добавив кейс «`human` + `X-Client-Id: operator`».

#### Minor

**Mi-1. `.golangci.yml:222-225` — исключение `forbidigo` покрывает весь `shared/`, а не только
владельцев времени и окружения.** Правило `path: shared/` + `text: (os\.(Getenv|LookupEnv)|time\.Now)`
снимает запрет со всех пакетов `shared/*`. По `foundation.md` §4 запрет на `time.Now` действует
«вне `shared/clock`», по §8/NFR-074 чтение env — через `shared/env`. Риск конкретен:
`shared/eventbus` по `foundation.md` §5.2 берёт часы через `eventbus.SetClock`, и `time.Now` там —
прямая потеря воспроизводимости (NFR-061), но линтер о ней не скажет; то же будет у будущих
`shared/logging`, `shared/testkit`.
**Правка**: сузить путь до `shared/(clock|runtime|env)/` (позже `logging`), оставив общий запрет для
остальных `shared/*`. Отдельное временное исключение `shared/eventbus/` (строки 233-236) уже есть и
снимается в T-005 — этого достаточно на волну 0.

**Mi-2. `.golangci.yml:203-210` — `forbidigo` закрывает только `time.Now`, оставляя открытыми
остальные входы во время.** `shared/clock.Timers` (`After`, `Every`) существует ровно затем, чтобы
заменить `time.After`, `time.Tick`, `time.NewTimer`, `time.NewTicker`; они, как и `time.Since` /
`time.Until`, ломают replay так же, как `time.Now`, и сейчас разрешены в `internal/*`. Аналогично
`os.Environ` / `os.ExpandEnv` обходят манифест `shared/env`. Формально DoD выполнен (в `tasks.md` и
`infrastructure.md` §2.2 назван только `time.Now`), но дыра реальная и дешёвая в закрытии.
**Правка**: `^time\.(Now|Since|Until|After|Tick|NewTimer|NewTicker)$` и
`^os\.(Getenv|LookupEnv|Environ|ExpandEnv)$`; по текущему коду нарушителей нет (проверено).

**Mi-3. `.golangci.yml:32-201` — depguard закодирован deny-списками и не «падает закрытым».**
Правила перечисляют запрещённых соседей для каждого известного `internal/<ctx>`. Контекст,
добавленный позже, не получит **ни одного** правила и сможет импортировать что угодно; при этом
смысл ADR-001 п. 3 — «запрещено всё, кроме перечисленного».
**Правка** (можно в бэклог, когда появится первый `internal/*` — T-005/T-011): перевести правила на
`allow` (`$gostd`, `multiverse-core.io/shared/...` плюс явно разрешённые соседи
`swarm → mechanics|laws|llm`, `state → mechanics`, `memory → llm`, `cmd/multiverse → replay`).
Сейчас 170 строк deny дают гарантию только для семи перечисленных имён.

**Mi-4. `build/Dockerfile:24` — runtime-образ запинен подвижным тегом, а не digest.**
`FROM gcr.io/distroless/static-debian12:nonroot`; `infrastructure.md` §2.4 требует именно digest
(«Runtime image … **по digest** в `build/Dockerfile`»). Тег `nonroot` перезаписывается апстримом,
воспроизводимость сборки теряется, Dependabot (экосистема `docker`, каталог `build/`) без digest
обновлять нечего. В `dev-log.md` §6 отклонение не зафиксировано.
**Правка**: `FROM gcr.io/distroless/static-debian12:nonroot@sha256:<digest>  # nonroot, <дата>`
либо явная запись отклонения в `dev-log.md` с переносом в DoD T-004 (владелец `build/` — devops).

**Mi-5. `build/Dockerfile:31-33` — `HEALTHCHECK` в образе жёстко указывает порт `core` (8090).**
Образ один на все роли (ADR-001 доп. п. 4: `gateway` :8088, `core` :8090, `memory` :8082), поэтому
контейнеры `gateway` и `memory` будут отмечены `unhealthy`, если каждый сервис compose не
переопределит `healthcheck`; при `depends_on: condition: service_healthy` (T-004) это заблокирует
старт стека. В эталоне `infrastructure.md` §2.3 `HEALTHCHECK` в Dockerfile отсутствует — проверка
живёт в compose. Отклонение в `dev-log.md` §6 не зафиксировано.
**Правка**: убрать `HEALTHCHECK` из образа (владелец проверки — compose, T-004) либо
параметризовать адрес (`ARG HEALTH_URL` / env) и написать это в комментарии, чтобы T-004 не
наступил на грабли.

**Mi-6. Отсутствие `.dockerignore` тянет в контекст сборки реальные секреты, а не только «лишние
каталоги».** `dev-log.md` §6 п. 5 оценивает последствие как «контекст лишний». Фактически при
`COPY . .` в builder-слой попадают неотслеживаемые `.env` владельца и
`.claude/worktrees/frosty-bell/{.env,shared/oracle/README.md}` — те самые 5 находок `gitleaks dir`
из §5, — а также `.git` целиком. Финальный distroless чист (копируется только `/out/`), в реестр
ничего не публикуется, поэтому это не Critical; но builder-слой и локальный build-кэш секреты
держат, а `docker build` по §5 уже выполнялся.
**Правка**: `.dockerignore` закреплён за T-004 — либо добавить минимальную версию здесь (6 строк),
либо зафиксировать в `dev-log.md` §10 как риск и внести в DoD T-004 явное требование перечислить
`.env*` (кроме `.env.example`), `.claude/`, `.git`, `backups/`, `bin/`, `Docs/`, `services/`,
`*.exe`, `*.log`; до этого образы не публиковать.

**Mi-7. `cmd/multiverse/serve.go:108-174` — `serve()` не покрыт автотестами.**
Самая большая функция команды (сборка `Deps`, выбор часов по `--mode`, старт HTTP, обработка
сигналов, порядок и таймауты остановки) проверена только руками (`dev-log.md` §5). Общий DoD §1 п. 3
требует тесты на новый код; покрытие пакета 61,6 % — остаток именно здесь. Ревьюер прогон
воспроизвёл (обе конфигурации `--contexts`), поведение верное, поэтому Minor, а не Major.
**Правка**: сделать функцию тестируемой — вынести сборку зависимостей отдельно либо передавать
готовый `*runtime.HTTP` / сообщать связанный адрес наружу, чтобы тест мог поднять процесс на
`127.0.0.1:0` и опросить `/health`; либо явно записать в `dev-log.md`, что проверка уходит на
уровень e2e F-5t (T-014), и внести это в DoD T-014.

**Mi-8. `shared/clock/clock.go:48-62` — `realTicker.Stop()` не потокобезопасен.**
Поле `stopped bool` читается и пишется без синхронизации, тогда как `time.Ticker.Stop` безопасен, а
у `manualTimer` то же поле защищено мьютексом владельца. Контекст, который держит тикер в рабочей
горутине и останавливает его из `Stop(ctx)`, даст гонку — её увидит `-race` в CI (T-012), а не
локально (гонка не покрыта тестами: `TestManualTimersEveryFiresEachPeriod` работает с `Manual`).
**Правка**: `sync.Once` либо `atomic.Bool` с `CompareAndSwap` для возврата «был ли взведён».

#### Nit

**N-1. `shared/clock/manual.go:32` — `Set(t)` при `t == now` считается движением назад.**
`backwards := !t.After(m.now)`, поэтому `Set(m.Now())` не срабатывает, а `Advance(0)` в тот же момент
— срабатывает (таймер `After(0)` через `Set` не выстрелит никогда). Либо `t.Before(m.now)`, либо
строка в комментарии.

**N-2. `shared/clock/manual.go:84-91` — `fire` выполняется после снятия `m.mu`.** Таймер,
остановленный между `collectDue` и `fire`, всё равно получит тик, хотя `Stop()` вернул `true`.
Та же оговорка есть у `time.Timer`; достаточно строки в doc-комментарии `Stop`.

**N-3. `shared/clock/manual.go:44-51` — `Advance` не проверяет отрицательное `d`**, в отличие от
`Set`. Документировать «`d` ≥ 0» либо переиспользовать защиту `Set`.

**N-4. `shared/runtime/lifecycle.go:17` — `StartAll` молча пропускает `Routes`, если `deps.Mux == nil`.**
Контекст, которому нужны admin-маршруты, стартует без них и без единого сообщения. Логировать или
возвращать ошибку.

**N-5. Два экспортированных бюджета остановки с близкими именами** — `runtime.StopTimeout`
(`lifecycle.go:10`, 15 с) и `runtime.ShutdownTimeout` (`http.go:14`, 5 с). `HTTPShutdownTimeout`
читалось бы однозначно; сейчас в `serve.go:165` легко перепутать.

**N-6. `shared/clock/manual.go:103-104` — `NewManualTimers` не используется** ни кодом, ни тестами
(везде `Manual.Timers()`); экспортированные функции `unused` не ловит. Покрыть тестом или убрать.

**N-7. `cmd/multiverse/serve.go:102-104` — `--recording` проверяется только на сочетание с
`--mode=replay`**, существование файла не проверяется, тогда как `dev-log.md` §6 п. 2 утверждает
«путь проверяется». Поправить формулировку в журнале либо добавить проверку, когда появится
`internal/replay`.

**N-8. `.env.example:51` (файл T-001, не T-003) — `MV_CORE_ADDR=:8090` расходится с умолчанием кода
`127.0.0.1:8090`.** `runtime.EnvAddr` берёт значение как есть, поэтому разработчик, скопировавший
`.env.example` в `.env`, поднимет admin-порт на всех интерфейсах вопреки ADR-009 доп. п. 3
(в compose `0.0.0.0` внутри сети — осознанно, локально — нет). В бэклог T-004/T-008.

**N-9. `.golangci.yml:244-249` — у исключения `shared/[a-z]+/examples/` нет задачи-владельца.**
Решение оркестратора по ОВ-7 назвало владельцев снятия только для `shared/eventbus` (T-005) и
`shared/agent` (EPIC-003). Третье исключение привязано к F-9/T-019 лишь комментарием в файле —
внести строку в DoD T-019, иначе останется навсегда.

**N-10. depguard-правила для `shared/testkit` нет.** ADR-001 доп. п. 8 требует, чтобы единственным
импортёром `shared/testkit/*` в production-бинарнике был `cmd/multiverse/fake_contexts.go`
(«`depguard`-исключение по файлу»). Пакета ещё нет, и `dev-log.md` §6 п. 7 переносит в T-018 только
сам хук, про правило линтера не пишет. Добавить правило в DoD T-018.

**N-11. `.golangci.yml:217-218, 261-262` — `services/` в `exclusions.paths` — регулярное выражение
без якоря.** Под него попадёт любой путь, содержащий `services/` (например, гипотетический
`internal/microservices/...`). Написать `^services/` и `^services/_archive`.

### Предложения в бэклог

1. **T-005 (F-4a)**: вместе со снятием исключения `shared/eventbus` расширить `forbidigo` на
   `time.After|Tick|NewTimer|NewTicker|Since|Until` и `os.Environ|ExpandEnv` (Mi-2) и сузить
   исключение `path: shared/` до владельцев времени/окружения (Mi-1).
2. **T-004 (F-6a)**: в DoD `.dockerignore` перечислить `.env*`, `.claude/`, `.git`, `backups/`,
   `bin/`, `Docs/`, `services/`, `*.exe`, `*.log` (Mi-6); решить судьбу `HEALTHCHECK` в образе
   (Mi-5) и digest-пина distroless (Mi-4); там же — `MV_CORE_ADDR` в `.env.example` (N-8).
3. **T-012 (F-7)**: сверка `toolchain` в `go.mod` ↔ `GO_VERSION` в `build/versions.env` — принято по
   ОВ-9, продублировать явным пунктом DoD job `unit`; там же первый прогон `-race` (Mi-8).
4. **T-018 (F-10)**: depguard-правило `shared/testkit` плюс исключение по файлу
   `cmd/multiverse/fake_contexts.go` (N-10).
5. **T-019 (F-9)**: снятие исключения `shared/*/examples/` из `.golangci.yml` (N-9) — вместе с
   судьбой демо-программ.
6. **Отдельная задача после появления первых `internal/*`**: перевести depguard с deny- на
   allow-списки, чтобы новый контекст закрывался по умолчанию (Mi-3).

### Риски и допущения

- Ревью выполнено **по индексу**, не по коммиту. `R100` подтверждается статусом индекса; `git log
  --follow` станет проверяемым после коммита. Коммитить нужно всё staged одним коммитом — выборочный
  коммит частями ухудшит распознавание переименований (то же замечание, что и в ревью T-002).
- `-race` не проверялся локально (принято по ОВ-5). Помимо Mi-8, под наблюдением в CI:
  `runtime.HTTP` (буферизованный `errored` плюс `close` в горутине `Serve`) и `clock.Manual`
  (`fire` вне мьютекса). Логика прочитана построчно, доказательных гонок, кроме Mi-8, не найдено.
- `docker build` ревьюером не повторялся (по указанию оркестратора); Mi-4/Mi-5/Mi-6 получены
  статическим разбором `build/Dockerfile` и `infrastructure.md` §2.3/§2.4, а не прогоном.
- Корректность depguard-глобов (`**/shared/**`, `**/internal/<ctx>/**`) на живых пакетах ревьюером
  не воспроизводилась: временные пакеты `internal/*` из `dev-log.md` §4 в индекс не попали, а
  создавать файлы в рабочей копии ревьюер не вправе. Принято по журналу разработчика; первая
  фактическая проверка — T-005, когда появится настоящий `internal/*`.
- `gofmt -l .` по всему репозиторию не пуст (33 файла) и таким останется, пока архив и
  legacy-сервисы лежат в рабочей копии. Общий DoD §1 п. 1 трактуется как «по единому модулю» —
  иначе он недостижим при живом `services/_archive/**` (U-1). Стоит закрепить эту трактовку в
  `tasks.md` §1, чтобы следующая сверка не открывала вопрос заново.
- `--bus`, `--recording`, `--mode=replay` в волне 0 только разбираются и валидируются (отклонения
  §6 п. 2–3): шины и журнала в `Deps` нет. Значит, `--bus=kafka` (умолчание) сегодня поднимает
  процесс без единого соединения — это ожидаемо, но `make up` в T-008 не должен трактовать
  «`/health` = ok» как «шина жива» до T-005.
