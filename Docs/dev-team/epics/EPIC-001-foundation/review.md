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

---

## T-004 · ревью #1 · 2026-09-09 · code-reviewer#1 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `b7df900`. Ревьюировались **изменения в индексе**
(`git diff --cached`) **только по файлам T-004**. Параллельно developer#1 ведёт T-005
(`shared/eventbus/**`, `.golangci.yml`, в т.ч. удаления `shared/eventbus/eventbus*.go`) —
эти пути в границы не входят и не рассматривались; незастейдженные правки пользователя
(`.claude/**`, `.qwen/**`, `.mcp.json`, `services/*/go.mod`) — тоже.

| Действие | Путь |
|---|---|
| A | `build/versions.env`, `build/minio.Dockerfile`, `.dockerignore`, `.github/ci.env`, `services/_archive/build/docker-compose.as-is.yml` |
| M | `docker-compose.yml` (переписано ядро), `Makefile` (+27 строк), `services/_archive/build/ARCHIVED.md`, `Docs/dev-team/epics/EPIC-001-foundation/dev-log.md` |
| вне git | `backups/minio-src-RELEASE.2025-10-15T17-29-55Z.tar.gz` (24 257 476 Б, проверен `tar -xzO`) |

Основание: `tasks.md` §1 (общий DoD) и §3 (T-004, включая «Дополнение (сведение 3)» и
«Предпосылку»), §4 (T-008 — границы отложенного); `dev-log.md` запись T-004 (отклонения 1–6,
ОВ-13…ОВ-16); `architecture/infrastructure.md` **v0.3** §1.1–§1.4, §2.2–§2.5, §3.1, §3.1.1, §3.2,
§4.2, §5.1, §5.2, §5.5, §10 (F-6); `threat-model.md` SEC-13, SEC-14, SEC-15, SEC-25, SEC-33,
T-14, T-28…T-32, чек-лист §10; ADR-021, ADR-005 доп. 2 п. 7, ADR-004 доп. п. 4, ADR-001 доп. п. 4/6;
`journal.md` (форк MinIO — U-10; llama-server вне compose — U-8; retention 30/90/180 — OQ-A-19,
относится к T-008).

Прогоны ревьюера (Docker Desktop есть, `make` нет — цели проверены чтением; пересборка образа
MinIO не повторялась по указанию оркестратора, `build/minio.Dockerfile` оценён статически +
сверен с исходником `buildscripts/gen-ldflags.go` из тарбола):

- `docker compose --env-file build/versions.env --env-file .github/ci.env config -q` → ✔ без вывода;
  то же с `--profile gpu` → ✔;
- `docker compose … --profile gpu config --format json` → 6 публикаций портов, **все** с
  `host_ip=127.0.0.1`; образы `docker.redpanda.com/redpandadata/redpanda:v26.1.17`,
  `multiverse-core/minio:RELEASE.2025-10-15T17-29-55Z`, `ollama/ollama:0.33.3`,
  `multiverse-core:ci` — все с явным тегом, `latest` нет; `core.extra_hosts` =
  `host.docker.internal=host-gateway`; сервиса `llama-server` нет; `name=multiverse`;
- `docker compose --env-file .github/ci.env config` (без `versions.env`) → падает на
  `required variable REDPANDA_IMAGE is missing a value` — fail-closed подтверждён;
- `gitleaks git --staged --redact .` → `no leaks found`;
- `pre-commit run --files <9 файлов T-004>` → все хуки `Passed` (`golangci-lint*` — Skipped, нет
  Go-файлов);
- `git grep -icE "minioadmin|password|secret"` по `build/`, `docker-compose.yml`, `.github/ci.env`
  → только ожидаемые имена переменных и комментарии, литералов учётных данных нет; `minioadmin`
  в файлах T-004 отсутствует;
- `git ls-files --eol` по шести файлам → `i/lf w/lf` (CRLF нет), хвостовых пробелов нет;
- `git hash-object services/_archive/build/docker-compose.as-is.yml` = `git show
  b7df900:docker-compose.yml | git hash-object --stdin` = `df991b8a…` — архивная копия
  побайтно идентична оригиналу;
- `go build ./... && go vet ./...` с `GOFLAGS=-buildvcs=false` → зелёные (T-004 Go-кода не добавляет,
  проверка «ничего не сломано»);
- отдельный микро-эксперимент (`docker build` тестового контекста в scratchpad) — проверка
  якорения глобов `.dockerignore`, см. Mi-1.

### Вердикт

**Принять** — Critical 0, Major 0, Minor 5, Nit 8. Задача сделана по дизайну, отклонения 1–6
из `dev-log.md` разумны и обоснованы, самое содержательное из них (`ENV MINIO_RELEASE` +
явный аргумент версии) проверено по исходнику `gen-ldflags.go` и **улучшает** воспроизводимость,
а не ухудшает её (см. «Проверено отдельно»). Все команды DoD воспроизведены ревьюером и зелёные.
Пять Minor — доработки, ни одна не блокирует T-008 и волну 0; три из них (Mi-2, Mi-3, Mi-4)
по существу адресованы T-008 и архитектору, а не переделке T-004.

### Проверено отдельно: отклонение 1 (`ENV MINIO_RELEASE=RELEASE`)

Замечаний нет, отклонение обосновано и корректно. Из `buildscripts/gen-ldflags.go` тега
(извлечён из тарбола в `backups/`):

- `releaseTag(version)`: `relPrefix := "DEVELOPMENT"`, переопределяется `os.Getenv("MINIO_RELEASE")`
  → без `ENV MINIO_RELEASE=RELEASE` штамп был бы `DEVELOPMENT.…` и критерий DoD «`minio --version`
  печатает `RELEASE.2025-10-15T17-29-55Z`» недостижим — вывод `dev-log.md` подтверждается кодом;
- версия-аргумент `"${MINIO_TAG#RELEASE.}"` = `2025-10-15T17-29-55Z` парсится как
  `time.Parse("2006-01-02T15-04-05Z", …)`, из неё же берётся `CopyrightYear`. Без аргумента версия
  и год выводятся из **времени коммита клона** — то есть предложенный §2.5 вариант отдаёт штамп на
  откуп метаданным `git clone`, а вариант разработчика делает его детерминированным. При «не том»
  формате тега `time.Parse` паникует, то есть ошибка громкая, а не тихая.

ОВ-15 (внести правку в `infrastructure.md` §2.5) — поддерживаю, это дефект документа, не кода.

### Проверенные пункты DoD

**DoD задачи (`tasks.md` §3, T-004)**

| # | Критерий | Результат |
|---|---|---|
| 1 | `make minio-image` собирается; `minio --version` = `RELEASE.2025-10-15T17-29-55Z` | ✔ по `dev-log.md` §3/§7 (образ 164 МБ, `Runtime: go1.26.8`); ревьюером не пересобиралось (указание оркестратора). Цель `Makefile:26-28` прочитана: `--build-arg` передаются все три (`MINIO_TAG`, `MINIO_REPO`, `MINIO_BUILDER_IMAGE`), тег `-t $(MINIO_IMAGE)` |
| 1 | откат на `golang:1.24.x` при провале, выбор зафиксирован | ✔ не понадобился; факт и запасное значение записаны в `versions.env:50-53` и `dev-log.md` §3 |
| 2 | `docker compose --env-file … config -q` | ✔ воспроизведено, вывод пуст |
| 3 | `config --format json`: ни одной публикации без `127.0.0.1` | ✔ 6/6 портов с `host_ip=127.0.0.1` (19092, 9644, 9000, 8088, 8090, 11434) |
| 3 | ни одного `image` без явного тега / с `latest` | ✔ 4 уникальных образа, все с тегом |
| 3 | `extra_hosts` у `core` | ✔ `host.docker.internal=host-gateway` (`docker-compose.yml:170-171`) |
| 3 | сервиса `llama-server` нет | ✔ и профиля для него нет |
| 4 | `mc admin info local` против собранного образа | ✔ по `dev-log.md` §3 (`1 drive online`), ревьюером не повторялось |
| 5 | `docker build --build-arg MINIO_REPO=<форк>` | ✔ на upstream; **форка нет** — предпосылка задачи не выполнена владельцем, задача корректно пошла по разрешённому запасному пути (ОВ-14, эскалация tech-lead#1) |
| 5 | `git ls-remote --tags <форк>` не пуст | частично: на upstream — ✔ (`9e49d5e7a648…`); на форке — n/a, форка нет |
| 5 | `backups/minio-src-<tag>.tar.gz` существует и распаковывается | ✔ 24 257 476 Б, проверено ревьюером (`tar -xzO … buildscripts/gen-ldflags.go`); `/backups/` в `.gitignore:16` |
| 5 | `versions.env` содержит `MINIO_REPO`, `LLAMACPP_BUILD`, `LLM_MODEL_DEFAULT` | ✔ строки 44, 74, 78 |
| 6 | путь тарбола записан в `dev-log.md` | ✔ §1 |
| — | состав файлов = «Файлы» задачи, ничего лишнего | ✔ ровно 5 файлов задачи + архив as-is compose + `ARCHIVED.md` + `dev-log.md` |
| — | «Не входит» (профили `memory/dev/legacy/bot`, init-скрипты, Makefile целиком, `compose-lint`) не сделано | ✔ границы соблюдены, ссылки на T-008 расставлены комментариями по месту |

**Общий DoD §1**

| # | Критерий | Результат |
|---|---|---|
| 1 | `make lint` / `gofmt` | n/a (Go-кода нет); `make` на машине отсутствует, цель `lint` появляется в T-008 |
| 2 | `go build ./... && go vet ./...` | ✔ зелёные (проверено ревьюером) |
| 2 | `go test -short -race` | Go-кода задача не добавляет; `-race` — только CI (принято по ОВ-5) |
| 3 | Тесты на новое поведение | n/a по составу задачи; поведение конфигурации закрывается `scripts/compose-lint.sh` в T-008 — это **зафиксировано** в DoD T-008, отдельного риска нет |
| 4 | `make secrets-scan` (gitleaks) | ✔ `gitleaks git --staged --redact .` → 0 находок |
| 5 | `dev-log.md` заполнен | ✔ образцово: таблица файлов, происхождение каждого значения, 6 отклонений с обоснованием, отложенное в T-008, таблица результатов DoD, 4 ОВ, 6 рисков |
| 6 | Соответствие критериям приёмки/дизайну | ✔ §1.3 (состав ядра), §1.4 (порты), §2.4 (пины), §2.5 (Dockerfile), §3.1 (`ci.env`), §5.5 (тома не апгрейдятся) — расхождения перечислены ниже |
| 7 | Изменений вне карты владения нет | ✔ `build/**`, `docker-compose.yml`, `Makefile`, `.dockerignore`, `.github/ci.env` — владение devops; `services/_archive/**` — перенос по U-1; `shared/eventbus/**` и `.golangci.yml` (T-005) не тронуты |
| 8 | CI зелёный | n/a до T-012 |

**Безопасность (SEC)**

| Пункт | Результат |
|---|---|
| SEC-13 (порты только loopback) | ✔ 6/6; консоли (MinIO 9001, Redpanda Console, Neo4j 7474) не публикуются. Оговорка — Mi-4 |
| SEC-14 (нет паролей по умолчанию) | ✔ `MINIO_ROOT_USER/PASSWORD`, `MV_MINIO_ACCESS_KEY/SECRET_KEY` — только `${VAR:?подсказка}`; `env_file: .env` не используется ни у одного сервиса (правильное решение: иначе правило 4 `compose-lint` становится непроверяемым) |
| SEC-15 (Ollama) | ✔ версия с пином, порт на loopback, литерала `OLLAMA_HOST=0.0.0.0` / `OLLAMA_ORIGINS=*` в файле нет |
| SEC-25 (цепочка поставок) | частично — Mi-5 (клон по мутабельному тегу, без сверки коммита) |
| SEC-33 (консоли только `dev`) | ✔ |
| D-10 (токен бота) | ✔ `MV_TELEGRAM_BOT_TOKEN` в `docker-compose.yml` отсутствует; в `.github/ci.env:92` — заведомо нереалистичная заглушка, обоснование в комментарии |
| Секреты в файлах T-004 | ✔ `gitleaks` чист; `.github/ci.env` корректно исключает `MV_LLM_API_KEY`/`MV_ANTHROPIC_API_KEY` (отклонение 5) — согласен: пустая строка `KEY=` действительно провоцирует `generic-api-key` на захват следующей строки |

**Полнота `build/versions.env` против §2.4**: все 16 требуемых ключей на месте, лишних нет;
`GO_VERSION=1.26.8` = `toolchain go1.26.8` в `go.mod` (сверено), `MINIO_BUILDER_IMAGE` тот же патч,
`GOLANGCI_LINT_VERSION=v2.13.2` и `GITLEAKS_VERSION=v8.30.1` совпадают с `rev:` в
`.pre-commit-config.yaml:10,14` (дрейфа нет). Формат: ни одной строки, которая не является
комментарием или `NAME=value` (проверено регуляркой) → `include` в `make` безопасен; `=` во всех
строках стоит **до** первого `:`, поэтому GNU make разбирает их как присваивания, а не как правила;
инлайновых комментариев после значений нет — ровно то, о чём предупреждает шапка файла. Файл
успешно прочитан `docker compose --env-file` в прогонах выше, то есть совместимость с обоими
потребителями подтверждена практически.

**`.github/ci.env` против `.env.example`**: множество имён — строгое подмножество (лишних
переменных нет ни одной); отсутствуют только `COMPOSE_ENV_FILES` (осознанно, файлы передаются
ключами), два ключевых `*_API_KEY` (отклонение 5) и 11 машинно-зависимых `MV_LLM_*`, которые
читает только `scripts/llm-server.*` и которые compose не интерполирует (§2.4, §4.2 п. 5).
Дрейфа значений с `.env.example` не обнаружено.

**Архивация as-is compose**: перенос корректен, содержимое побайтно идентично `b7df900`, секции
`narrative-orchestrator` (стр. 118), `semantic-memory` (136) и `chromadb` (63) — источник профиля
`legacy` — на месте вместе с as-is переменными; `ARCHIVED.md` заполнен по всем шести полям
(причина, коммит архивации, последний рабочий коммит, эпик возврата, что использовало) и честно
фиксирует, что `--follow` истории не даст, с двумя способами прочитать оригинал. U-1 соблюдён.

### Замечания

#### Critical

Нет.

#### Major

Нет.

#### Minor

**Mi-1. `.dockerignore:23, 53-60` — глобы без `**` якорятся на корень контекста, вложенные файлы
в образ попадают.** Заявленная в шапке файла первая цель — безопасность («`.env`, local IDE state
and the archive must not be baked into an image»), и в `dev-log.md` §7 стоит «✔ `.dockerignore`
исключает … `*.exe`, `*.log`». Фактически исключается только корневой уровень. Проверено
экспериментом (контекст с `root.exe` и `sub/nested.exe`, `.dockerignore` = `*.exe`): в образ попал
`sub/nested.exe`, `root.exe` — нет. Касается `*.exe`, `*.test`, `*.out`, `*.log`, `*.db`,
`*.db-wal`, `*.db-shm`, `coverage.*`, `.env.*`. Сегодня прикрыто тем, что `services/`, `bin/`,
`backups/` исключены целиком, а T-001 вывел бинарники из индекса, — но правило рассчитано на
будущее. **Как исправить:** заменить на рекурсивную форму с двумя звёздочками и слешем
(`**/*.exe`, `**/*.log`, `**/*.test`, `**/*.out`, `**/*.db`, `**/*.db-wal`, `**/*.db-shm`,
`**/coverage.*`, `**/.env.*`); каталоги `Docs/`, `bin/`, `.claude/` якорить на корень —
правильно, их не трогать.

**Mi-2. `docker-compose.yml:250-253` — имена томов `minio_data` и `ollama_data` совпадают с as-is
(`services/_archive/build/docker-compose.as-is.yml:349,353`) при одинаковом имени проекта.**
As-is стек запускался из того же каталога, то есть его тома — `multiverse_minio_data` и
`multiverse_ollama_data`; `COMPOSE_PROJECT_NAME=multiverse` (`.env.example`, `.github/ci.env:25`)
даёт ровно те же имена. Первый же `docker compose up` на машине владельца молча примонтирует
каталог данных as-is MinIO (созданный под `minioadmin/minioadmin`) в новый сервер — а
`infrastructure.md` §5.5 / D-8 прямо требуют **не** апгрейдить тома на месте, а пересоздать их
после `make archive-legacy`. Комментарий в секции `volumes` это описывает, но ничего не
предотвращает; `redpanda_data` коллизии не имеет (у as-is тома вообще не было).
**Как исправить:** развести имена (`minio_data_v2` / `ollama_models`) **или** внести в DoD T-008
проверку «`docker volume ls` не содержит `multiverse_minio_data` до первого `make up`» и
жёсткий порядок `make archive-legacy` → `docker volume rm` → `make up`. Решение — за T-008,
но зафиксировать нужно сейчас.

**Mi-3. `build/versions.env:68` — `CHROMA_IMAGE=` пуста, пин §2.4 отсутствует.** Логика
«пустое падает громко, выдуманный тег — тихо» верная, и ОВ-16 честно передаёт решение в T-008,
но формально DoD «создать `build/versions.env` со **всеми** пинами `infrastructure.md` §2.4»
на конец T-004 не выполнен. **Как исправить:** не менять код — попросить оркестратора закрыть
это решением (как ОВ-16) и перенести пункт «`CHROMA_IMAGE` = конкретный тег» в DoD T-008 явной
строкой, иначе критерий останется висеть между двумя задачами. Дополнительно: пустая переменная
в `--env-file` не даёт ошибки интерполяции сама по себе — сервис `chromadb` в T-008 должен
использовать `${CHROMA_IMAGE:?}`, иначе «громко» не получится.

**Mi-4. `docker-compose.yml:201-202` — публикация admin/health-порта `core` противоречит
`threat-model.md` (T-14, чек-лист §10), но соответствует `infrastructure.md` v0.3 §1.4.**
T-14 в графе «Митигация» требует «admin-порт core **не публикуется**», чек-лист §10 — «admin-порт
core не опубликован»; §1.4 таблицы портов требует ровно `127.0.0.1:8090`. Разработчик выбрал
более новый и прямо названный в задаче документ — это правильный выбор, но противоречие живёт в
двух утверждённых документах, и `scripts/compose-lint.sh` (T-008) придётся писать под одно из них.
Практический риск низкий (loopback + `AdminOnly` по `X-Client-Id` из `MV_CORE_ADMIN_CLIENTS`).
**Как исправить:** вопрос system-architect/security-engineer — привести `threat-model.md` T-14 и
чек-лист §10 в соответствие с §1.4 (или наоборот); в T-004 менять нечего.

**Mi-5. `build/minio.Dockerfile:21` — клон по мутабельному тегу без сверки коммита (SEC-25).**
`git clone --depth 1 --branch "${MINIO_TAG}"` даёт воспроизводимость ровно настолько, насколько
неизменяем тег в `${MINIO_REPO}`. Именно этот репозиторий планируется заменить на **форк**, где
владелец (или кто-то с доступом к аккаунту) может переставить тег, — то есть страховка U-10
одновременно открывает возможность подмены содержимого при неизменных значениях в `versions.env`.
Коммит уже известен и записан в `dev-log.md` §3 (`9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a`).
**Как исправить:** добавить `MINIO_COMMIT=9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a` в
`build/versions.env`, прокинуть `ARG MINIO_COMMIT` и после `git clone` — fail-closed проверку
вида `test "$(git rev-parse HEAD)" = "${MINIO_COMMIT}"` (одна строка, ноль стоимости сборки).
Тот же аргумент закрывает и переход на форк без повторной ручной сверки.

#### Nit

**N-1. `docker-compose.yml:250-253` — разнобой в именовании томов:** `redpanda_data`, `minio_data`,
`ollama_data` через подчёркивание, `gateway-data` через дефис. Привести к одному стилю (в T-008,
где файл всё равно дополняется).

**N-2. `docker-compose.yml:90-92` и `Makefile:28` — контекст сборки MinIO — весь репозиторий,
хотя из него не используется ничего:** Dockerfile клонирует исходники внутрь. `context: build`
(или отдельный пустой каталог) сократит передачу контекста до килобайтов и уберёт зависимость
сборки образа хранилища от `.dockerignore` платформы.

**N-3. `docker-compose.yml:47, 94-96` — build-аргументы без `:?`/дефолта.**
`GO_VERSION: ${GO_VERSION}` при отсутствующем `versions.env` дал бы `golang:-bookworm`.
На практике недостижимо (проверено: интерполяция падает раньше, на `REDPANDA_IMAGE:?`), поэтому
только для единообразия — `${GO_VERSION:?}`.

**N-4. `docker-compose.yml:42` — `image: multiverse-core:${MV_IMAGE_TAG:-dev}` берёт тег не из
`versions.env`.** Правило 1 `compose-lint` (§3.1.1) сформулировано как «совпадает со значением из
`build/versions.env`» — для образа платформы нужно явное исключение в `scripts/compose-lint.sh`
(T-008), иначе правило либо ложно сработает, либо будет ослаблено целиком.

**N-5. `docker-compose.yml:77, 238` — healthcheck'и `redpanda` и `ollama` не проверены в рантайме.**
`docker compose up` в T-004 запрещён, и это правильно, но `rpk cluster health | grep -q
'Healthy:.*true'` завязан на формат текстового вывода `rpk` v26.1, а `depends_on: service_healthy`
у `gateway`/`core` делает его блокирующим: при несовпадении формата стек не поднимется вовсе.
Внести в DoD T-008 («стенд») отдельной строкой; запасной вариант — `rpk cluster health
--exit-when-healthy` (код возврата вместо grep по тексту).

**N-6. `build/minio.Dockerfile:37-38` — runtime `alpine` без `ca-certificates`.** Для MVP-1
(loopback, `MV_MINIO_USE_SSL=false`, без tiering и notification targets) не нужен, но первая же
исходящая TLS-операция даст невнятную ошибку `x509: certificate signed by unknown authority`.
Либо `RUN apk add --no-cache ca-certificates` (≈ 0,5 МБ), либо строка в `dev-log`/ADR-021
«исходящий TLS вне области MVP-1».

**N-7. `Makefile:9` — `GIT_SHA := $(shell git rev-parse --short HEAD)` вычисляется при каждом
запуске make (включая `make help`) и пуст вне git-checkout** → `make image` соберёт
`-t multiverse-core:` и упадёт с `invalid reference format`. Заменить на
`GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)`.

**N-8. `Makefile:54-56` — as-is цель `build` теперь собирает новый `build/Dockerfile` и тегирует
его `multiverse-core` (то есть неявный `:latest`)**, что прямо противоречит правилу «никаких
`latest`» из шапки `docker-compose.yml` и NFR-071. Цель снимается T-008 (файл переписывается
целиком) — но до тех пор она вводит в заблуждение; внести удаление as-is целей отдельной строкой
в DoD T-008.

### Предложения в бэклог

1. **T-008 (F-6b), обязательное к учёту:** (а) Mi-2 — коллизия имён томов с as-is; (б) Mi-3 —
   `CHROMA_IMAGE` и `${CHROMA_IMAGE:?}` у сервиса `chromadb`; (в) N-4 — исключение для образа
   платформы в правиле 1 `compose-lint`; (г) N-5 — проверка healthcheck'ов на стенде;
   (д) N-8 — снятие as-is целей `Makefile`; (е) перевод `depends_on` на
   `redpanda-init`/`minio-init` (`service_completed_successfully`) — комментарии уже расставлены.
2. **T-008, правило 3 `compose-lint` требует правки формулировки.** «`git grep -i minioadmin`
   пуст вне `Docs/` и `services/_archive/`» сегодня невыполнимо и от T-004 не зависит: совпадения
   дают замороженные сервисы в `services/**` (`entity-manager`, `entity-actor`,
   `evolution-watcher`, `ontological-archivist`, `rule-engine`, `semantic-memory`) и комментарий
   «не minioadmin» в `.env.example:18`. Либо расширить исключения на `services/**` и
   `.env.example`, либо переформулировать правило как «нет литерала в `docker-compose.yml` и
   `build/**`».
3. **T-008:** N-8 из ревью T-003 (`.env.example:51` `MV_CORE_ADDR=:8090` против умолчания кода
   `127.0.0.1:8090`) был направлен «в бэклог T-004/T-008»; `.env.example` в состав файлов T-004 не
   входит, поэтому пункт целиком переходит в T-008.
4. **system-architect / security-engineer:** Mi-4 — согласовать `threat-model.md` T-14 и чек-лист
   §10 с `infrastructure.md` v0.3 §1.4 по публикации admin-порта `core`.
5. **devops-engineer:** ОВ-15 — внести `ENV MINIO_RELEASE=RELEASE` и аргумент версии
   `gen-ldflags.go` в `infrastructure.md` §2.5 (подтверждено чтением исходника, см. выше);
   заодно Mi-5 (`MINIO_COMMIT`).
6. **Владелец / tech-lead#1:** ОВ-14 — форк `minio/minio` в `alekseizabelin1985-spec` не создан,
   первая линия страховки U-10 отсутствует. Задача корректно пошла по разрешённому запасному
   пути, но эскалацию нужно довести до владельца: после создания форка меняется одна строка
   `build/versions.env:44`.
7. **T-012 (F-7):** ОВ-13 — Dependabot (`docker`, каталог `build/`) должен покрывать оба
   Dockerfile (digest distroless и `alpine:3.22` / `MINIO_BUILDER_IMAGE`); пины `versions.env`
   Dependabot не видит — ежемесячная ручная сверка (§2.4). Плюс сверка `toolchain` в `go.mod` ↔
   `GO_VERSION` (ОВ-9, уже принято).

### Риски и допущения

- Ревью выполнено **по индексу**, не по коммиту; параллельная работа T-005 в том же индексе
  сознательно исключена из границ. Итоговый коммит T-004 должен захватить ровно 9 путей из
  таблицы «Границы ревью» — иначе граница задач размоется.
- **Сборка образа MinIO и `mc admin info local` ревьюером не воспроизводились** (указание
  оркестратора): приняты по `dev-log.md` §3/§7. Статически проверено всё, что можно проверить без
  сборки, включая семантику `gen-ldflags.go` по исходнику из тарбола.
- **`docker compose up` не запускался** ни разработчиком (запрет задачи), ни ревьюером. Значит,
  рантайм-поведение ядра (healthcheck'и, порядок `depends_on`, два listener'а Redpanda,
  доступность `host.docker.internal` из `core`) подтверждено только статически. Первый фактический
  подъём — DoD «стенд» T-008; N-5 и Mi-2 — самые вероятные места отказа.
- Образ `multiverse-core/minio:<tag>` существует только локально: на чистой машине и в CI до
  `make minio-image` / шага сборки в job `integration` любой `docker compose up` и любой
  integration-тест T-007 упадут с «image not found». В `dev-log.md` это записано; критично, чтобы
  порядок команд попал в `make up` (T-008) и в README (F-9).
- Тарбол `backups/minio-src-<tag>.tar.gz` — вне git и вне бэкапов проекта: при переезде на другую
  машину вторая линия страховки U-10 не переезжает автоматически. Пока форка нет (ОВ-14), это
  единственная страховка.
- Значения `${VAR:-…}` в `docker-compose.yml` дублируют дефолты `.env.example`; `mvctl env check`
  (T-010) сверяет только `.env.example` с реестром `shared/env`, но не с compose. Расхождение
  дефолтов будет тихим до тех пор, пока в `scripts/compose-lint.sh` (T-008) не появится
  соответствующая проверка. Риск отмечен разработчиком — подтверждаю его как реальный.
- `make` на машине ревьюера отсутствует: `Makefile` (включая корректность `include
  build/versions.env`) проверен чтением плюс формальным разбором файла на «make-враждебные»
  конструкции; фактического `make -n` не было.

---

## T-005 · ревью #1 · 2026-09-09 · code-reviewer#2 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `b7df900`, изменения **в индексе** (коммита нет,
`git.commits: ask`). Ревьюировались только файлы T-005:

| Действие | Путь |
|---|---|
| A | `shared/eventbus/{bus,registry,delivery,dedup,kafka,sources}.go`, `shared/eventbus/{types,registry,delivery,dedup,kafka,fixtures}_test.go` |
| M | `shared/eventbus/{types,topics,relations,relation_types,payload_types}.go`, `README.md`, `examples/universal_paths_example.go`, `relations_test.go`, `shared/runtime/runtime.go`, `.golangci.yml`, `dev-log.md` (запись T-005) |
| D | `shared/eventbus/{eventbus.go,eventbus_test.go}` |

Файлы T-004 (`build/**`, `docker-compose.yml`, `Makefile`, `.dockerignore`, `.github/ci.env`,
`services/_archive/build/**`) — параллельная задача другого разработчика, не ревьюировались.
Основание: `contracts.md` v0.4 §0 и C-01 v1.1; `components/foundation.md` v0.2 §5–§6;
`analysis/api-contracts.md` v0.2.1 §2.0–§2.2; ADR-007, ADR-010; `tasks.md` §1 и T-005;
`dev-log.md` (запись developer#1 · T-005); `journal.md` (решения оркестратора по ОВ T-005).

### Вердикт

**Вернуть.** Critical: 0 · Major: 3 · Minor: 7 · Nit: 4.

Задача сделана по контракту: имена типов, полей и методов C-01 воспроизведены, `Route`/`Delivery`
действительно транспортно-независимы (membus T-014 переиспользует их без копирования правил),
тесты содержательные и покрывают негативные пути, исключение линтера снято без единого `//nolint`.
Возврат — из-за трёх дефектов, каждый чинится в несколько строк: настройка writer'а kafka-go даёт
≈ 1 с задержки на каждую публикацию (M-1); значение по умолчанию у флага валидации при чтении
«выключено», тогда как SEC-16/C-01 требуют «включено» (M-2); конверт `NewRoot` не заполняет
обязательный по §2.1 `meta.gm_path`, что рассинхронит библиотеку с `_envelope.json` из T-006 (M-3).

### Проверенные пункты DoD

| Проверка | Результат ревьюера |
|---|---|
| `go build ./...` | ✔ |
| `go vet ./...` | ✔ |
| `go test -short -count=1 ./...` | ✔ (10 пакетов `ok`) |
| `go test -cover ./shared/eventbus/` | ✔ **69,7 %** (порог DoD — 60 %) |
| `golangci-lint run ./...` (v2.13.2) | ✔ `0 issues`; исключение `shared/eventbus/` из `.golangci.yml` снято (ОВ-7 закрыт), `//nolint` в `shared/**`, `cmd/**` — 0 |
| `gofmt -l shared cmd` | ✔ пусто |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| `pre-commit run --files <файлы T-005>` | ✔ 8 хуков `Passed` |
| DoD T-005: unit `NewRoot`/`Derive` (наследование `CorrelationID`/`Timestamp`, новый `event_id`) | ✔ `types_test.go:9,70,113,128,158` |
| DoD T-005: `Dedup` (повтор не проходит дважды) | ✔ `dedup_test.go` — повтор, вытеснение LRU, снапшот, 8 горутин |
| DoD T-005: политики топиков на фикстурах | ✔ `registry_test.go:239,260` (`player_events` отклоняет `actor_kind=system` и `meta.agent`; топики роя требуют `meta.agent`) |
| DoD T-005: DLQ после 3 повторов | ✔ `delivery_test.go:162` (`Attempts=4` — трактовка принята оркестратором) |
| DoD T-005: `End` монотонен, `ReadRange` по возрастанию офсета | ⚠ по DoD «полностью проверяется в T-014»; здесь только пустой интервал и отрицательный офсет — принято |
| Общий DoD §1 п. 2 `-race` | не запускался (нет gcc, ОВ-5); построчный разбор гонок — см. «Риски» |
| Общий DoD §1 п. 7 (карта владения) | ⚠ см. Mi-7: `shared/runtime/runtime.go` вне карты файлов T-005 |

### Соответствие C-01 (пословная сверка)

Совпадает буквально: `Event{ID,Type,Timestamp,Source,World,Scope,Meta,Payload,Relations}`;
`Meta{SchemaVersion,CorrelationID,CausationID,CausationType,ActorKind,Agent,Replay,Locale,GMPath}`;
`NewRoot(type,source,worldID,scope,actorKind,payload)`; `Derive(parent,type,source,payload,opts…)`
с наследованием `CorrelationID`/`ActorKind`/`Locale`/`GMPath` и `CausationID = parent.ID`;
`WithAgent(AgentRef)`; `Bus{Publish,Subscribe,Close}`; `Handler func(ctx,Event) error`;
`Journal{ReadRange(ctx,topic,from,to,h)(next,err), Tail, End}`; `Position{Topic,Offset}`;
`PositionFromContext(ctx)(Position,bool)`; `Dedup` LRU по `event.id`, ёмкость 10 000, `Seen(id) bool`,
сериализация в снапшот; политики `player_events` и топиков роя; валидация при чтении;
`Event.Path()`, `GetWorldIDFromEvent`, `GetScopeFromEvent` — без изменений;
`runtime.Deps{Bus, Journal, …}`. Отклонения — M-3 (`meta.gm_path`) и Mi-2 (`DeriveOption`).

### Замечания

**M-1 (Major). `shared/eventbus/kafka.go:299-308` — writer без `BatchSize`/`BatchTimeout`: каждая
публикация ждёт ≈ 1 с.** У `kafka-go` v0.4.51 `Writer.batchTimeout()` по умолчанию — 1 с, а
`batchSize()` — 100 (`writer.go:825,1067`); синхронный `WriteMessages` (`Async=false`) ждёт
`<-batch.done`, а батч закрывается либо по заполнению 100 сообщениями, либо по таймеру. При
одном событии в батче каждый `Publish` блокируется на ~1 с. Для MVP-1 (низкий трафик, 1 партиция)
это режим по умолчанию, а не исключение: рушится NFR-001 (ack gateway ≤ 300 мс, `overview.md`
строка `gateway`) и цепочка «действие → механика → нарратив». Тестами без брокера не ловится —
именно поэтому 24 % покрытия `kafka.go` здесь недостаточно как аргумент. **Правка**: в
`k.writer()` добавить `BatchSize: 1` (публикация у нас всегда одиночная) либо
`BatchTimeout: 10 * time.Millisecond`; значение вынести в константу рядом с `kafkaMaxWait` и
упомянуть в `README.md` в разделе «Шина и журнал» вместе с reader-параметрами. Проверку задержки
публикации добавить в contract-тест T-014 (testcontainers), иначе регрессия вернётся незаметно.

**M-2 (Major). `shared/eventbus/kafka.go:37`, `delivery.go:58` — нулевое значение
`ValidateOnRead` = «валидация выключена», тогда как C-01/SEC-16 требуют «по умолчанию
включено».** Решение вынести чтение `MV_BUS_VALIDATE_ON_READ` в процесс правильное (`forbidigo`,
`shared/env` появится в T-007) и задокументировано в `dev-log.md` §2 п. 2 — но защитная мера
получила fail-open умолчание в общей библиотеке. Любой, кто соберёт `KafkaConfig{Brokers,
Registry}` (membus T-014, харнесс T-018, `mvctl` T-010, будущие контексты), молча отключит
проверку схемы и политик при чтении; `cmd/multiverse` шину пока не строит вообще, то есть владельца
у умолчания сейчас нет ни в коде, ни в DoD. **Правка** (достаточно одной из двух): (а) инвертировать
поле — `SkipValidateOnRead bool` (нулевое значение = валидация включена), поправив `kafka.go:85`,
`delivery.go:72` и `README.md:86`; или (б) оставить как есть, но внести явным пунктом DoD T-007
(`shared/env`) и T-010 «умолчание `MV_BUS_VALIDATE_ON_READ` при отсутствии переменной — `true`» и
сослаться на это в комментарии к полю. Вариант (а) предпочтительнее: он не зависит от чужой задачи.

**M-3 (Major). `shared/eventbus/types.go:70,144-165` — `meta.gm_path` обязателен по
`api-contracts.md` §2.1, но `NewRoot` его не заполняет, а тег — `omitempty`.** Каждое корневое
событие уходит на шину без `gm_path`; `Derive` его наследует, значит вся цепочка тоже. Как только
T-006 напишет `_envelope.json` с `required: [… gm_path]` (таблица §2.1 помечает поле «обяз. да»),
валидация при чтении отправит **все** события в `dead_letters`, а поймается это только на
contract-тесте T-014 или позже. `ValidateEnvelope` пустой `gm_path` тоже пропускает
(`registry.go:133`). **Правка**: либо ставить умолчание в `NewRoot` (`GMPath: GMPathAgent`, а
legacy-издатель переопределяет `WithGMPath(GMPathLegacy)`) и проверять непустоту в
`ValidateEnvelope` для не-legacy конверта, либо (если поле задумано опциональным) поднять ОВ
системному архитектору и зафиксировать `gm_path` как необязательный в `_envelope.json` **до**
старта T-006. Решение принять нельзя молча: оно жёстко связывает T-005, T-006 и всех издателей.

**Mi-1 (Minor). `shared/eventbus/kafka.go:142-147` и `delivery.go:91-96` — при штатной остановке
`Subscribe` возвращает `context.Canceled`, а не `nil`.** `readerError` (`kafka.go:356`) специально
превращает отмену в `nil` («shutdown, not a failure»), но два других выхода из цикла этого не
делают: отмена во время backoff'а (`Deliver` → `ctx.Err()`) и `reader.CommitMessages(ctx, msg)` с
уже отменённым контекстом. Контекст рантайма получит ошибку на нормальном `Stop`, а contract-тест
T-014 будет сравнивать kafka и membus по разным веткам. **Правка**: пропускать возврат `deliver`
и `CommitMessages` через тот же `k.readerError(ctx, topic, err)` (он уже умеет отличать отмену), а
в `bus.go` у `Bus.Subscribe` дописать в комментарий, что штатное завершение по `ctx` — это `nil`.

**Mi-2 (Minor). `shared/eventbus/types.go:90` — тип опции назван `Option`, C-01 называет
`DeriveOption`.** Расхождение имён из контракта, который реализуют три эпика; в `dev-log.md`
отклонение не отмечено. **Правка**: добавить `type DeriveOption = Option` (алиас, ноль изменений в
коде вызывающих) либо зафиксировать переименование в `dev-log.md` и запросом на изменение C-01.

**Mi-3 (Minor). `shared/eventbus/delivery.go:104-109,144-151` — размер dead letter не ограничен:
«ядовитое» большое сообщение навсегда останавливает топик.** `DeliverRaw` кладёт в `Raw` тело
целиком (reader принимает до `kafkaMaxBytes` = 10 МиБ), а `deadLetter` — весь `Original`. Запись
в `dead_letters` больше 1 МиБ (`batchBytes` kafka-go и `max.message.bytes` брокера по умолчанию)
не пройдёт, `Deliver` вернёт ошибку, офсет не закоммитится — и то же сообщение будет читаться
бесконечно, блокируя топик, ради чего DLQ и заводился. **Правка**: обрезать `Raw` до константы
(например, 64 КиБ) с пометкой в `Error` об усечении и делать то же для `Original.Payload`, если
сериализованный dead letter превысил порог.

**Mi-4 (Minor). `shared/eventbus/kafka.go:210-226` — `End` не реагирует на отмену контекста и
знает только `brokers[0]`.** `conn.SetDeadline` ставится, лишь если у `ctx` есть дедлайн; при
`context.WithCancel` без дедлайна `ReadLastOffset` на неотвечающем брокере повиснет до TCP-таймаута
ОС, а признак «конца журнала» — это как раз путь выхода из `replay` в `live` (C-01). Плюс `End` не
проверяет `k.closed` и всегда идёт в первый брокер списка. **Правка**: ставить дедлайн всегда
(`deadline, ok := ctx.Deadline()`; иначе `nowUTC().Add(kafkaEndTimeout)` — константа рядом с
`kafkaMaxWait`), проверять `closed` и перебирать `k.brokers`, пока `DialLeader` не удастся.

**Mi-5 (Minor). `shared/eventbus/registry.go:108-137` — `ValidateEnvelope` не проверяет
`meta.schema_version` и `meta.locale`, обязательные по §2.1.** Конверт с `schema_version = 0` или
пустой `locale` (собранный не конструктором — например, при ручной правке в фикстурах или после
неполного JSON) пройдёт и публикацию, и чтение. Проверка `correlation_id`/`actor_kind`/`gm_path`
рядом уже есть, добавление симметрично. **Правка**: для не-legacy конверта требовать
`SchemaVersion >= 1` и `Locale != ""`.

**Mi-6 (Minor). `.golangci.yml` — предложение №1 из ревью T-003, адресованное T-005, не выполнено
и не отклонено.** Просилось вместе со снятием исключения `shared/eventbus` расширить `forbidigo`
на `time.After|Tick|NewTimer|NewTicker|Since|Until` и `os.Environ|ExpandEnv` и сузить
`path: shared/` до владельцев времени и окружения (второе, кстати, уже сделано в T-003 —
`shared/(clock|runtime|env)/`). Проверено: расширение шаблонов даёт `0 issues` (единственные
нарушители — `shared/agent`, у него исключение уже есть). **Правка**: либо добавить шаблоны, либо
записать отказ в `dev-log.md` и перенести пункт в бэклог T-012 явной строкой.

**Mi-7 (Minor). `shared/runtime/runtime.go:19,56-64` — правка вне карты файлов T-005, решения по
ней нет.** Изменение по существу верное и предписано C-01 (`runtime.Deps{Bus, Journal, …}`), но
общий DoD §1 п. 7 требует отсутствия изменений вне карты владения. Разработчик поднял это как
ОВ-17 в `dev-log.md` §6, однако в `journal.md` под номером ОВ-17 записано другое решение
(`contracts.Spec` встраивает `eventbus.TypeSpec`) — нумерация сдвинулась, и вопрос остался без
ответа. **Правка**: tech-lead#1 подтверждает правку одной строкой в `journal.md`; в `tasks.md`
раздел T-005 «Файлы» дополнить `shared/runtime/runtime.go`.

**N-1 (Nit). `shared/eventbus/delivery.go:16` — `DefaultBackoff` экспортирован изменяемым срезом.**
Любой потребитель (или тест) может переписать элемент и изменить поведение всех подписок процесса.
Заменить на функцию `DefaultBackoff() []time.Duration`, возвращающую копию, либо оставить
неэкспортируемым, а наружу дать только поле `Backoff`.

**N-2 (Nit). `shared/eventbus/kafka.go:166,196` — имя потребителя журнала `"journal." + topic`
собирается литералом в двух местах.** membus в T-014 обязан повторить ту же строку, иначе
contract-тест сравнит разные `DeadLetter.Consumer`. Вынести в экспортируемый хелпер
(`JournalConsumer(topic string) string`) рядом с `Position`.

**N-3 (Nit). `shared/eventbus/delivery.go:154` — запись dead letter идёт с
`context.WithoutCancel(ctx)` без собственного таймаута.** Решение (пережить отмену при остановке)
правильное, но верхняя граница держится только на дефолтах kafka-go (`WriteTimeout` 10 с,
`MaxAttempts` 10). Обернуть в `context.WithTimeout(context.WithoutCancel(ctx), …)` — таймаут
взять из `Delivery` (поле или константа).

**N-4 (Nit). Legacy-профиль ссылается на удалённые символы.**
`services/{semantic-memory,narrative-orchestrator}` (остаются до S5 по T-002) используют
`eventbus.EventBus`, `NewEventBus`, `TopicScopeManagement` — удалены вместе с `eventbus.go` и
`topics.go`. Регрессии сегодня нет: эти модули после разделения workspace (T-003) не имеют
`require` на общий модуль и не собираются и без T-005. Отразить факт в решении по профилю `legacy`
(`tasks.md` §9, подволна 0.3), чтобы он не всплыл при сборке профиля в T-008.

### Предложения в бэклог

1. **T-014 (F-5t)**: contract-тест обязан сравнивать kafka и membus не только по порядку и
   офсетам, но и по (а) `nil` при штатной отмене `Subscribe`/`Tail` (Mi-1), (б) `DeadLetter.Consumer`
   для журнала (N-2), (в) задержке публикации одиночного события (M-1).
2. **T-014**: `testkit.Dedup` — псевдоним `eventbus.Dedup` (C-01), в T-005 сознательно не делался.
3. **T-006**: закрыть M-3 в `_envelope.json` до фиксации схем; `Spec` встраивает
   `eventbus.TypeSpec` (решение оркестратора по ОВ-17 в `journal.md`).
4. **T-010/T-014**: кеш соединения для `Journal.End` (риск из `dev-log.md` §7) — измерить накладные
   расходы, прежде чем усложнять.
5. **T-019 (F-9)**: `shared/eventbus/{MIGRATION.md,docs/,examples/}` описывают предыдущую итерацию
   модели событий (`NewStructuredEvent`, `PublishEntityCreated`) — расходятся с новым README
   (ОВ-19 оркестратора).

### Риски и допущения

- Ревью по индексу, не по коммиту; `-race` не запускался (нет gcc). Построчный разбор гонок:
  `Dedup` — всё состояние под `sync.Mutex`, `evict` вызывается только под ним, `IDs`/`Restore`
  согласованы по порядку (oldest → newest) — гонок не видно; `sources.go` — `sync.RWMutex`, значение
  копируется под `RLock` и вызывается вне блокировки (`nextID`, `nowUTC`) — реентерабельность
  безопасна; `Kafka` — `writers`/`readers`/`closed` под `k.mu`, `registry`/`brokers` неизменяемы
  после конструктора, двойной `Reader.Close()` (из `Close` и `closeReader`) безопасен, kafka-go
  защищён своим флагом; горутин пакет не создаёт (`go func` — только в тесте `Dedup`).
  Единственное разделяемое изменяемое состояние — пакетные `SetIDSource`/`SetClock`/`SetRegistry`
  и экспортированный `DefaultBackoff` (N-1); тесты источники не гоняют параллельно (`t.Parallel`
  в пакете нет), но и запрета на переустановку «на живом процессе» в коде нет — допущение принято
  как в `dev-log.md` §7.
- Транспортная независимость (`Route` + `Delivery`) проверена по коду: правила публикации,
  валидации при чтении, ретраев и DLQ не зависят ни от одного типа kafka-go; membus в T-014 обязан
  вызывать те же `Route`/`Delivery.Deliver` и предоставить свой `DeadLetterSink`. Единственные
  места, которые придётся повторить руками, — имя потребителя журнала (N-2) и значение
  `ValidateOnRead` (M-2); оба замечания об этом и написаны.
- `kafka.go` покрыт на 24 % — принято: без брокера дальше конфигурации и отказов до сети не уйти.
  Но M-1 показывает цену этого допущения: дефекты настройки клиента ловятся только в T-014,
  поэтому предложение №1 в бэклог считаю обязательным к внесению в DoD T-014.
- `ReadRange` с `to` больше high watermark блокируется на `FetchMessage` до отмены контекста, а
  курсор впереди конца журнала (`from > End`) отдан на откуп поведению kafka-go при
  `OffsetOutOfRange`. Для MVP-1 (`from`/`to` берутся из снапшота и `End()`) допустимо; проверить
  на брокере в T-014.
- Проверка `source ∈ Spec.Publishers` в `Route` отсутствует — соответствует решению оркестратора
  (ОВ-18: проверка статическая, job `contracts`/T-010).

---

## T-006 · ревью #1 · 2026-09-09 · code-reviewer#1 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `dfc0498` (T-005), изменения **в индексе**
(`git diff --cached`), коммита нет. Ревьюировались только файлы T-006:

| Действие | Путь |
|---|---|
| A | `shared/contracts/{spec.go,registry.go,contracts.go,sources.go,ownership.go}` + `{registry,validate,ownership}_test.go` |
| A | `schemas/embed.go`, `schemas/events/{_common.json,_envelope.json}` + 25 схем типов блоков «а» и «б» |
| M | `go.mod`/`go.sum` — в части `github.com/santhosh-tekuri/jsonschema/v6 v6.0.3` |
| M | `dev-log.md` — запись «developer#1 · T-006» |

Файлы T-007 (`shared/{env,objstore,logging}`, `shared/runtime/http*.go`,
`cmd/multiverse/{serve,health}.go`, `.golangci.yml`) и не-staged правки других ролей
не рассматривались — их смотрит второй ревьюер.

Основание: `tasks.md` §1 (общий DoD) и §4 (T-006, T-009); `architecture/contracts.md` v0.4 §0,
C-01 v1.1, C-02 v1.2, C-13, C-14, §16 п. 6–7; `architecture/components/foundation.md` v0.2 §6;
`architecture/components/state-and-mechanics.md` §4.4, §4.6; `analysis/api-contracts.md` v0.2.1
§2.1–§2.3; ADR-007, ADR-013; `journal.md` — решения оркестратора по ОВ-17/ОВ-18 и по ОВ T-006
(ОВ-20…ОВ-24).

### Вердикт

**Вернуть.** Critical: 0 · Major: 1 · Minor: 10 · Nit: 6.

Работа сделана добротно: реестр и схемы соответствуют C-01, foundation §6 и §2.2/§2.3
`api-contracts.md`, состав типов полон (8 `player.*` + 7 `group.*` + 2 `round.*` + 8 блока «б» =
25 схем при 25 не-deprecated `Spec`), «нет фантомов» проверяется в обе стороны, негативных фикстур
достаточно, все прогоны зелёные, покрытие 79,6 % (порог 60 %). Решения оркестратора по ОВ-17
(`Spec` встраивает `eventbus.TypeSpec`), ОВ-18 (`source ∈ Publishers` — статическая проверка),
ОВ-22 (`scope` в `round.opened` не добавлять) и требование T-005 (`gm_path` в `required` с enum
`agent|legacy`) выполнены дословно.

Возврат — из-за одного Major: три из восьми схем `player.*` потеряли общее поле `action`
(с `key_hash`), которое §2.3.1 даёт всем типам семейства; при `additionalProperties: false` это
жёсткий отказ штатному издателю. Правка — три схемы, объём минут. Остальное — Minor/Nit; часть
Minor закрывается не кодом, а открытыми вопросами к system-architect (Mi-3, Mi-5, Mi-8) и
назначением владельца (Mi-7).

### Проверенные пункты DoD

| Критерий T-006 | Результат |
|---|---|
| `go test ./shared/contracts/... -run TestSchemasValid` | ✔ перепроверено ревьюером, PASS |
| каждый пример §2.3 блоков «а»/«б» валиден против своей схемы | ✔ `TestPayloadExamples`, 25 примеров; тест сам сверяет «примеров = типов со схемой» |
| «нет фантомов» в обе стороны | ✔ `TestNoPhantoms` (тип без файла и файл без типа), deprecated исключены явно |
| `OwnershipRules` — уровни + пустые `object`/`monitor` + строка gateway | ✔ `TestOwnershipRulesCoverEveryProposer`, `TestGatewayRow`, `TestOwnershipRulesAreACopy`; см. Mi-9 |
| у каждого `Spec` непустой `Publishers`; таблица типов с несколькими издателями | ✔ `TestEveryTypeHasPublisherAndConsumer`, `TestPublishersOfSharedTypes` (5 типов) |
| `cause` содержит `forget` в `entity.*.proposed`/`entity.updated` | ✔ `TestCauseForget`; см. Mi-4 про остальные значения |
| покрытие ≥ 60 % | ✔ **79,6 %** (перепроверено) |

| Общий DoD §1 | Результат (перепроверено ревьюером) |
|---|---|
| `golangci-lint run ./...` | ✔ `0 issues` |
| `gofmt -l shared/contracts schemas` | ✔ пусто |
| `go build ./... && go vet ./...` | ✔ |
| `go test -short -count=1 ./...` | ✔ 11 пакетов `ok`, ни одного `FAIL` |
| `go test -race` | n/a локально (ОВ-5), в CI T-012 |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| dev-log заполнен | ✔ 8 разделов, отклонения и ОВ-20…ОВ-24 записаны; см. N-4 |
| изменений вне карты владения нет | ✔ `shared/contracts/**`, `schemas/**` — EPIC-001 |
| пин `jsonschema/v6 v6.0.3` | ✔ в блоке прямых зависимостей, `go list -m` подтверждает; `xeipuuv/gojsonschema` в корневом `go.mod` отсутствует (foundation §6) |

### Замечания

**M-1 (Major). `schemas/events/player.entered_region.v1.json:7`,
`player.left_region.v1.json:7`, `player.said.v1.json:7` — потеряно общее поле `action`.**
`api-contracts.md` §2.3.1 задаёт **одну** форму payload на всё семейство `player.*`; пометка
«только `player.said`» стоит единственный раз — у `text`, у `action{type, key_hash}` её нет.
В поставке `action` есть в `player.{looked,rested,attacked,flee_attempted,defended}`, но
отсутствует в `entered_region`, `left_region`, `said`. При `additionalProperties: false` это не
«необязательное поле», а отказ: все три события рождаются из `POST /v1/players/{id}/actions`
(§1.4: `enter`, `leave`, `say` — все с `action_key`), и `key_hash` — след идемпотентности,
который gateway логично кладёт в payload наравне с `look`/`rest`. Правило разное для типов
одного семейства, в dev-log §3/§4 как решение не зафиксировано.
*Правка*: добавить в три схемы блок `action` в том же виде, что в `player.looked.v1.json`
(`{type: string minLength 1, key_hash: string}`, `required: [type]`), либо — если решено, что
`key_hash` в payload не едет вовсе — убрать `action` из всех восьми и записать это отклонение
от §2.3.1 в dev-log с ОВ к system-analyst.

**Mi-1 (Minor). `shared/contracts/contracts.go:133` — `Validate` для deprecated-типа возвращает
`nil`, не проверив конверт.** `foundation.md` §6 про legacy-типы говорит «валидируется только
конверт», а здесь не валидируется ничего. Сегодня дыры нет: `eventbus.Route`
(`shared/eventbus/registry.go:150`) и `Delivery.deliver` (`shared/eventbus/delivery.go:131`) зовут
`ev.ValidateEnvelope()` **до** `Registry.Validate`. Но пакетная `contracts.Validate` экспортирована
и предназначена в том числе для `mvctl contracts check` (T-010) и фикстур: прямой вызов признает
валидным legacy-событие с пустыми `id`/`source` и нулевым `timestamp`.
`TestLegacyTypesCarryNoSchema` это поведение закрепляет.
*Правка*: `if spec.Deprecated { return ev.ValidateEnvelope() }` и негативный подтест (legacy без `id`).

**Mi-2 (Minor). `shared/contracts/contracts.go:115` (`All`) и `:106` (`Spec`) отдают `Spec` с
общими backing-массивами `Publishers`/`Consumers`.** `slices.Clone(r.ordered)` копирует только
внешний срез; `spec.Publishers[0] = …` у любого потребителя правит пакетную `definitions`
глобально и во всех горутинах — реестр читается из нескольких (шина при `Publish` и при чтении).
Сам `Registry` после `New` иммутабелен, и это единственная лазейка; контраст очевиден с
`OwnershipRules()` (`shared/contracts/ownership.go:146`), где копия сделана глубокой намеренно и
покрыта тестом `TestOwnershipRulesAreACopy`.
*Правка*: клонировать `Publishers`/`Consumers` в `All`/`Spec` тем же приёмом, что в
`OwnershipRules()`, либо явно объявить срезы неизменяемыми в godoc `Spec` и добавить тест-«часовой».

**Mi-3 (Minor). `schemas/events/entity.create.proposed.v1.json:20` — поле `proposer` не
представимо, а `OwnershipRules` его различает.** §2.3.4 прямо называет `proposer=system` при
загрузке фикстур (`mvctl world init --fixtures`, UC-036); в схеме поля нет, а
`additionalProperties: false` запрещает его прислать. При этом `shared/contracts/ownership.go:126–140`
содержит две разные строки — `author` (`mvctl`) и `system` (bootstrap), — которые по конверту не
различаются (в обоих случаях `source = mvctl`, `meta.agent = nil`). Значит либо строки дублируют
друг друга и одна лишняя, либо в payload нужен `proposer`. В dev-log вопрос не поднят
(ОВ-21 закрывает соседний `proposal_id`, но не этот).
*Правка*: завести ОВ к system-architect/EPIC-002 (T-052): чем State отличает `author` от `system`;
до ответа — либо добавить `proposer: {enum: [gateway, author, system]}` (опц.), либо свести две
строки таблицы в одну.

**Mi-4 (Minor). Внутренняя несогласованность поставки: `shared/contracts/ownership.go:68,84,96,131,138`
допускает `cause ∈ {leave, flee, death, resolve, init, author}`, схемы `entity.*` их отвергают.**
enum в `entity.create.proposed`/`entity.update.proposed`/`entity.updated` —
`combat|rest|move|loot|spawn|tick|group|create|bootstrap|forget` (§2.3.4), таблица владения — по
§4.6. Итого в одном пакете правило владения разрешает причину, которую схема того же пакета
отклонит. Решение оркестратора по **ОВ-20** (журнал, 2026-09-09) — расширить enum до объединения
§2.3.4 и §4.6, правку вносит T-009 или EPIC-002 T-052; в T-006 она не внесена, и это соответствует
решению. *Дефектом T-006 не считаю*, фиксирую как факт и как риск: если правка не попадёт в
DoD T-009, State не сможет опубликовать штатные причины, а тест «копия ↔ `levels.go`» (T-202) её
не поймает — он сверяет таблицу с таблицей, а не таблицу со схемой.
*Правка*: внести в DoD T-009 пункт «enum `cause` = объединение §2.3.4 и §4.6» и тест
«каждое значение `Causes` из `OwnershipRules()` присутствует в enum `cause` схем `entity.*`».

**Mi-5 (Minor). `schemas/events/_common.json:27` — `WorldRef` не фиксирует `entity.type`.**
`api-contracts.md` §2.1 задаёт `world{entity{id, type: world}}`; в схеме `type` — любая непустая
строка, то есть `world.entity.type = "region"` пройдёт. То же у `ScopeRef` (`_common.json:34`):
в MVP-1 это `solo`/`group`, ограничения нет. Смысл `additionalProperties: false` — ловить
опечатки издателя; здесь опечатка в типе мира/скоупа проходит, хотя `AgentRef.level`
(`_common.json:47`) пять значений enum'ом честно перечисляет.
*Правка*: `WorldRef.entity.type` → `{"const": "world"}`; для `ScopeRef.type` — enum
`["solo", "group"]` либо явная строка в dev-log, что тип скоупа сознательно оставлен открытым.

**Mi-6 (Minor). `schemas/events/_envelope.json:48` и `:43` — `additionalProperties: false` на
конверте и на `meta` противоречит обещанию совместимости C-01/ADR-007 п. 3.**
`foundation.md` §6 требует запрета лишних полей **на верхнем уровне payload**; godoc
`eventbus.Event` (`shared/eventbus/types.go:77`) и ADR-007 п. 3 обещают «новые поля добавляются
без слома существующих потребителей». С текущим конвертом добавление любого поля в `Event` или
в `Meta` делает невалидными **все** события до правки схемы, то есть совместимое по ADR изменение
становится несовместимым. Решение осознанное (dev-log §1), но противоречие не оговорено.
*Правка*: либо `additionalProperties: true` на конверте (оставив `false` на `meta` и на payload),
либо абзац в dev-log/запрос к system-architect: «расширение конверта = синхронная правка
`_envelope.json`, помечается `contract-change`».

**Mi-7 (Minor). `runtime.Deps.Contracts` не добавлен, отклонение не записано.**
`shared/runtime/runtime.go:56` (комментарий T-003) прямо говорит: «Store и Env в F-5 (T-007),
**Contracts в F-4b (T-006)**»; C-01 и `foundation.md` §3 перечисляют `Contracts` в составе `Deps`;
dev-log T-003 §6 п. 1 зафиксировал это как временное отклонение с указанием задачи-владельца.
В T-006 поле не появилось и в §4 «Отклонения» не упомянуто; ни один контекст сегодня реестр не
получает (`grep contracts.Default` вне пакета — пусто). Функционально не блокирует —
`contracts.Default()` синглтон, — но лишает контексты возможности подменить реестр фикстурой
в тестах, ради чего поле и заводилось.
*Правка*: не в T-006 (файл вне списка задачи), а решением оркестратора: назначить поле
`Deps.Contracts` конкретной задаче (T-010 — там уже есть `contracts check`, либо отдельная правка
через tech-lead#1) и снять пометку с комментария `runtime.go:56`.

**Mi-8 (Minor). `schemas/events/group.{created,joined,left,disbanded}.v1.json` не называют
затронутого игрока.** Схемы дословно повторяют §2.3.2 (`group`, `leader`, `members[]`, `cause`),
и это правильно как буква; но потребитель `group.joined`/`group.left` вынужден вычислять
«кто именно вошёл/вышел» диффом `members[]` против собственной проекции, а §2.1 объявляет
`entity{entity{id,type},name}` общим доменным полем «основная сущность». При
`additionalProperties: false` gateway (EPIC-004) не сможет добавить `entity` без правки схемы.
*Правка*: ОВ к system-analyst/EPIC-004 (T-301) по образцу ОВ-22 — добавить `entity` в перечень
полей §2.3.2 и в схемы, либо зафиксировать, что дифф `members[]` — намеренный контракт.

**Mi-9 (Minor). `shared/contracts/ownership.go:52–63` — в перечне «условий §4.6, не укладывающихся
в форму правила» пропущено одно.** Комментарий называет три (hp gateway только при `cause=rest`;
`status` gateway только `alive → abandoned`; исключения строк `*`), а §4.6 содержит и четвёртое:
строка `task`/`player` — «`position` (только `flee`)». В копии `Paths` содержит `position`, а
`Causes` — `combat, loot, flee, death`, то есть агент встречи по таблице может двигать игрока с
`cause=combat`. По C-02 State читает **только** эту копию, поэтому потерянное условие State
неоткуда взять.
*Правка*: дописать условие в комментарий `ownership.go` (и, при ревизии, в `levels.go` T-202).

**Mi-10 (Minor). `shared/contracts/contracts.go:166` — `Validate` маршалит событие в JSON на
каждый вызов, замера нет.** `json.Marshal` + `jsonschema.UnmarshalJSON` выполняются и при
публикации, и при чтении (`MV_BUS_VALIDATE_ON_READ=true` по умолчанию), то есть дважды на событие
end-to-end; на чтении исходный JSON уже есть в руках адаптера и повторно кодируется из структуры.
Компиляция схем закеширована корректно (`sync.OnceValues`, `contracts.go:184`) — претензий нет.
Оценка `foundation.md` §6 (20–50 мкс) не подтверждена; dev-log §8 честно это признаёт и уносит
в T-014. Ставлю Minor, а не Nit, потому что цена платится на каждом событии в обе стороны.
*Правка*: `go test -bench` на `Validate` в этой же задаче (десяток строк) либо явный пункт в
DoD T-014; при неудовлетворительном замере — вариант `ValidateRaw([]byte)` для пути чтения.

**N-1 (Nit). `schemas/events/group.{joined,left,disbanded}.v1.json` отформатированы иначе, чем
остальные 22 схемы** (развёрнутый JSON, 66 строк против ~30). Правила форматирования JSON в
проекте нет (в `.pre-commit-config.yaml` нет ни `pretty-format-json`, ни `check-json`), поэтому
Nit; но каталог схем — предмет глазного ревью контрактов тремя другими эпиками.
Привести к компактному стилю остальных файлов.

**N-2 (Nit). `shared/contracts/sources.go:52` — `retention180` выбивается из ряда**
`retention30d`/`retention90d`. Переименовать в `retention180d`.

**N-3 (Nit). `shared/contracts/contracts.go:145` — в сообщении об ошибке payload нет `ev.ID`.**
Путь внутри документа jsonschema/v6 даёт (`at "/changes/0/ops/0/op": value must be one of …`),
тип есть, идентификатора события нет — а при разборе `dead_letters` ищут именно его.
`fmt.Errorf("%w %s (%s): %w", ErrInvalidPayload, ev.Type, ev.ID, err)`.

**N-4 (Nit). dev-log §5 — «17 тестов, 107 подтестов».** Фактически `go test -v ./shared/contracts/`
даёт 17 тестов и 45 подтестов (62 строки `PASS:`). Поправить цифру, чтобы она не расходилась с
последующими прогонами.

**N-5 (Nit). `schemas/events/group.created.v1.json:26` и `group.disbanded.v1.json` допускают весь
enum `cause` из семи значений** там, где семантически возможны `create` и `disband`. Сужение
enum по типу события даёт ту же защиту от опечаток издателя, что и `additionalProperties: false`.

**N-6 (Nit). `shared/contracts/ownership.go:22` — константа `AnyCause` объявлена, но ни одна
строка таблицы её не использует** (строки `author`/`system` перечисляют `init, author` явно).
Либо применить в этих строках, либо удалить, чтобы не подсказывать несуществующее правило.

### Предложения в бэклог

1. **T-009**: тест «каждое значение `Causes` из `OwnershipRules()` есть в enum `cause` схем
   `entity.*`» — закрывает Mi-4 навсегда, а не однократной правкой enum.
2. **T-010 (`mvctl contracts check`)**: проверка `source ∈ Spec.Publishers` на фикстурах; а также
   «у каждого `TopicSpec` есть хотя бы один `Spec`» (сейчас `world_events`, `narrative_output`,
   `llm_records`, `dead_letters` пусты — законно до T-009, после должно стать ошибкой).
3. **T-014**: бенчмарк `contracts.Validate` (Mi-10) и сверка семантики `Registry.ValidateEnvelope`
   (JSON-схема) с `eventbus.Event.ValidateEnvelope` (Go) — сейчас первая отвергает legacy-конверт
   без `meta`, вторая принимает; это законно, но в godoc не оговорено.
4. **EPIC-002 (T-052) / EPIC-004 (T-301)**: при ревизии учесть `entity.created.version: const 1`
   (dev-log §4) и опциональный `entity.created.proposal_id` — последнего в §2.3.4 нет, поле
   добавлено сверх контракта (совместимо, но в отклонениях не записано).
5. **system-architect**: `eventbus.Entity` (`shared/eventbus/payload_types.go:30`) несёт `version`,
   `created_at`, `updated_at`, `world`, `metadata`, которых нет в
   `_common.json#/$defs/EntityWithName`; при `additionalProperties: false` payload, собранный
   старым билдером `EventPayload.WithEntity`, будет отклонён, как только заполнят любое из этих
   полей. Либо привести билдер к `EntityRef` + `name`, либо расширить `EntityWithName`
   (пометка `contract-change`).

### Риски и допущения

- Ревью по индексу, не по коммиту; `-race` не запускался (ОВ-5, нет gcc). Разбор
  потокобезопасности по коду: `Registry` после `New` не мутируется (карта и срез только читаются),
  `defaultRegistry` — `sync.OnceValues`, компилированные `*jsonschema.Schema` при `Validate`
  состояние не меняют (валидатор v6 держит контекст в стеке вызова). Единственное разделяемое
  изменяемое состояние — backing-массивы `Publishers`/`Consumers`, отдаваемые наружу без копии
  (Mi-2); гонки сегодня нет только потому, что потребителей у пакета ещё нет.
- Полнота «нет фантомов» проверена ревьюером независимо: 25 файлов `*.v1.json` ↔ 25 не-deprecated
  `Spec`; 9 legacy-типов (`player.moved`, `player.used_skill`, `gm.{created,deleted,merged,split}`,
  `narrative.generate`, `violation.detected`, `time.syncTime`) внесены как `Deprecated` без схемы —
  ровно перечень `foundation.md` §6; обоснование (профиль `legacy` иначе не опубликует ничего)
  принимаю, решение ОВ-23 «в T-009 не дублировать» зафиксировано в журнале.
- Топики и retention сверены с §2.2 построчно: 8 топиков, 30/90/180 дн., `analytics_events` и
  `dead_letters` не читаются в replay. Маршрутизация сверена полностью для блоков «а»/«б»:
  `group.entered_region`/`left_region` → `player_events` (а не `game_events`), `dice.rolled` →
  `game_events`, `snapshot.created` → `system_events`, `analytics.replay.completed` →
  `analytics_events` — всё по §2.2.
- `go.mod`/`go.sum`: пин `jsonschema/v6 v6.0.3` внесён корректно и в блок **прямых** зависимостей.
  В том же staged-дифе `minio-go/v7 v7.3.0` и `testcontainers-go v0.44.0` стоят в блоке
  `// indirect`, хотя импортируются напрямую (`shared/objstore`, T-007), и `klauspost/compress`
  поднят `1.15.9 → 1.19.2` — следствие невыполненного `go mod tidy` (ОВ-24/ОВ-29, отложен
  оркестратором до приёмки подволны 0.3). К T-006 не относится, фиксирую как факт: после `tidy`
  блоки перестроятся — повторно проверить, что `jsonschema/v6` остался прямым.
- `additionalProperties: false` ложных срабатываний в текущем наборе не даёт: конверт, собранный
  `eventbus.NewRoot`/`Derive`, проходит `_envelope.json` полностью (поля `Event` и `Meta`
  совпадают со схемой один в один, `omitempty` расставлены согласованно, `ensurePayload` не даёт
  `payload: null`). Единственный найденный источник ложного отказа — M-1 (и, на перспективу,
  п. 5 бэклога).
- Тест равенства `shared/agent/levels.go ↔ OwnershipRules` появится только в EPIC-003 T-202;
  до него копия проверяется «изнутри», а расхождение с §4.6 ловится глазами — Mi-9 как раз такой
  случай, найденный ручной сверкой таблицы с §4.6.
- Ничего не правил и не коммитил: изменена только эта запись в `review.md`.

---

## T-007 · ревью #1 · 2026-09-09 · code-reviewer#2 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `dfc0498`, изменения в индексе (коммита нет).
В границах — только файлы T-007 (F-5):

- `shared/env/**` (`env.go`, `vars.go`, `infra.go`, `example.go` + тесты);
- `shared/objstore/**` (`objstore.go`, `buckets.go`, `memory.go`, `minio.go` + unit- и
  integration-тесты);
- `shared/logging/**` (`logging.go` + тесты);
- правки `shared/runtime/http.go` и `http_test.go` (удаление `EnvAddr`/`envOr`, чтение манифеста);
- `cmd/multiverse/serve.go`, `cmd/multiverse/health.go`;
- `.golangci.yml` (сужение исключения `forbidigo`);
- запись «developer#2 · T-007» в `dev-log.md`; добавленные зависимости `go.mod`.

**Вне границ** (смотрит code-reviewer#1): `shared/contracts/**`, `schemas/**` (T-006). Находки в
этих файлах в отчёт T-007 не включались.

Основания: `tasks.md` §1 (общий DoD) и T-007 с дополнением сведения 3;
`architecture/components/foundation.md` v0.2 §7, §8; `architecture/infrastructure.md` v0.3 §4.1,
§4.2, §5.2, §7.1; ADR-021 п. 1–4; ADR-009 п. 4, п. 9; ADR-005 доп. 2 п. 1–3, п. 5;
`architecture/threat-model.md` (T-02, T-06, T-08, §4.2, SEC-02/06/14/22/28);
`plan/ownership.md` v0.4 §1 (строки 9, 11, 12, 35); `journal.md` от 2026-09-10 (решения
оркестратора по ОВ-25…ОВ-29).

### Вердикт

**Вернуть.** Critical: 0 · Major: 1 · Minor: 10 · Nit: 6.

Задача сделана добротно: манифест полон и сверен в обе стороны, секреты не выводятся ни на одном
пути, редакция логов работает по ключу и по шаблону, integration-тест `objstore` действительно
поднимает собранный образ MinIO и проверяет versioning/ILM, `os.Getenv` вне `shared/env` устранён
полностью. Возврат — из-за одного Major: две реализации `objstore.Client` расходятся в поведении
`Delete` несуществующего объекта, причём расходятся вопреки тому, что записано в doc-комментарии
самого интерфейса. Правка на две строки плюс кейс в тесте; остальное — Minor/Nit, часть из них
уместно закрыть в этой же итерации, часть — в T-008/T-010/T-012.

### Что проверено прогоном (машина владельца, `GOFLAGS=-buildvcs=false`)

| Проверка | Результат |
|---|---|
| `go build ./...` | ✔ 0 |
| `go vet ./...` | ✔ 0 |
| `go test -short -count=1 ./...` | ✔ 15 пакетов `ok` |
| `go test -cover ./shared/env/ ./shared/objstore/ ./shared/logging/` | ✔ 94,6 % · 65,0 % · 98,4 % (порог 60 %) |
| `go test -tags integration ./shared/objstore/` | ✔ `ok 5,483s` (testcontainers, `multiverse-core/minio:RELEASE.2025-10-15T17-29-55Z`) |
| `golangci-lint run ./...` | ✔ `0 issues` |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` |
| `pre-commit run --files <22 файла T-007>` | ✔ 8 хуков `Passed` |
| `-race` | не прогонялся (нет cgo/gcc — по решению оркестратора уходит в CI, T-012) |

Заявленные в `dev-log.md` §5 цифры подтверждены полностью; расхождений с фактом нет.

### Проверенные пункты DoD

**DoD задачи T-007**

1. `objstore` по ADR-021 п. 2 — интерфейс ровно `Put/Get/Stat/List/Delete/EnsureBucket/Capabilities`,
   без multipart, presigned, object lock и уведомлений. ✔
2. Две реализации сразу (`Memory` + `minio-go/v7 v7.3.0`). ✔ (с оговоркой M-1)
3. `EnsureBucket` сам включает versioning и ILM по `BucketOptionsFor` — проверено на живом сервере:
   `entities-integration` → `Versioning=Enabled` + `NoncurrentDays=30`; `prompts-integration` →
   versioning выключен + `Expiration.Days=30` (SEC-22); `ops-artifacts` → `GetBucketLifecycle`
   возвращает ошибку, то есть правил нет; повторный вызов идемпотентен. ✔ (частично — Mi-10)
4. `buckets.go` с `entities-*`, `snapshots-*`, `prompts-*`, `ops-artifacts` и единой таблицей. ✔
5. `env.Declare` с обязательным префиксом `MV_`, `DeclareExternal`, сверка с `.env.example`. ✔
6. `logging` — slog с обязательными полями, `BusMiddleware`, редакция внешних ID и токенов. ✔
   (обязательное поле времени называется `time`, а не `ts` — Mi-9)
7. **Сведение 3, LLM-блок**: `MV_LLM_PROVIDER` с enum `openai_compat|ollama|anthropic|recorded|fake`
   и дефолтом `openai_compat`, `MV_LLM_URL`, `MV_LLM_API_KEY` (`Secret`), `MV_LLM_MODELS_DIR`
   (`Tooling`), `MV_LLM_CLOUD_ENABLED` — все на месте. ✔
8. **Сведение 3, `OLLAMA_*`**: условная обязательность реализована (`RequiredWhen`), тест
   `TestValidateOllamaBlockIsRequiredOnlyForTheOllamaProvider` закрывает оба направления — при
   `openai_compat` отсутствие блока не ошибка, при `ollama` ошибка называет и `MV_OLLAMA_URL`, и
   `OLLAMA_MAX_LOADED_MODELS`, и причину «`MV_LLM_PROVIDER` is ollama». ✔
9. **Сведение 3, устаревшие**: `MV_OPENAI_API_KEY`/`MV_DEEPSEEK_API_KEY` через `DeclareDeprecated`,
   в `.env.example` их нет, `Deprecated()` сообщает имя и замену **без значения** (проверено
   тестом). ✔
10. unit: `Memory` целиком включая `Capabilities()`; `Declare` отклоняет имя без `MV_`; «внешний ID
    и токен не попадают в лог». ✔
11. integration с `-tags integration`, образ из `build/versions.env`, `Capabilities()={true,true}`. ✔
12. `.env.example` дополнен всеми объявленными переменными. ✔ — **сверено независимо от кода
    задачи**: манифест 68 переменных (57 `MV_*` + 11 внешних), в `.env.example` 68 присваиваний,
    множества совпадают точно; `.env.example` совпадает с `infrastructure.md` §4.2 в обе стороны с
    единственным добавлением `MV_CORE_ADMIN_CLIENTS` (внесено в T-003).
13. Покрытие ≥ 60 % по всем трём пакетам. ✔

**Общий DoD §1**

1. `golangci-lint run` чист, `gofmt`/`goimports` без диффа (хук `golangci-lint-fmt` зелёный). ✔
2. `go build`/`go vet`/`go test -short -count=1` зелёные; `-race` — n/a локально, в CI (T-012). ✔ (с оговоркой)
3. Тесты написаны в той же задаче, 30 тест-функций на три пакета, негативные случаи есть,
   недетерминированности нет (паузы ретраев — через `clock.Timers`, `Memory` — через `clock.Clock`). ✔
4. `gitleaks` — 0 находок. ✔
5. `dev-log.md` заполнен: экземпляр, задача, что сделано, отклонения, ОВ-25…ОВ-29. ✔
6. Соответствие критериям историй — в границах F-5 проверяется NFR-074 (сверка манифеста, ✔) и
   NFR-041 (редакция, ✔ с оговоркой Mi-3).
7. Карта владения — правки в `shared/runtime`, `cmd/multiverse`, `.golangci.yml`, `go.mod` вне
   списка файлов T-007 и не отмечены как отклонение (Mi-7); добавление зависимостей в `go.mod`
   отмечено в отчёте, как требует `ownership.md` §1 строка 9. ✔ с оговоркой
8. CI — n/a до T-012.

### Сверка с решениями оркестратора (`journal.md`, 2026-09-10)

| Решение | Состояние |
|---|---|
| ОВ-25: имя `MV_MINIO_USE_SSL` | ✔ взято; `foundation.md` §7 и `design.md` §8 остаются с `MV_MINIO_SECURE` — правка документов за architect/devops |
| ОВ-26: `MV_CONTEXTS`/`MV_ID_SOURCE` — флаги, не env | ✔ в манифест не внесены |
| ОВ-27: `env.Validate` вызывает `cmd/multiverse`, но только для включённых контекстов | ✘ не реализовано (решение появилось после сдачи задачи). Важно для T-010: **текущий API этого не выражает** — у `Var` есть только `scope=process\|tooling`, признака «нужна контексту X» нет, а `Validate` проверяет манифест целиком. Реализация ОВ-27 потребует расширения `shared/env` (общий код) — учесть в объёме T-010/EPIC-002 |
| ОВ-28: `MV_CORE_ADMIN_CLIENTS` по умолчанию `operator` | ✔ дефолт не менялся, fail-closed сохранён |
| ОВ-29/ОВ-24: `go mod tidy` после приёмки 0.3 | ✘ ещё не сделан — см. Mi-8 |
| Централизованный `vars.go` вместо `foundation.md` §8 | ✔ принят; следствие — `ownership.md` §1 строка 35 («владелец процесса/контекста объявляет через `shared/env.Declare`») и `foundation.md` §8 теперь противоречат коду, их надо править (бэклог, п. 1) |
| T-005: «передавать `SkipValidateOnRead = !MV_BUS_VALIDATE_ON_READ`» | переменная объявлена, doc-комментарий фиксирует правило; провод пока некому — потребителей шины в волне 0 нет, ожидается в T-010/T-018 |

### Замечания

#### Major

**M-1. `objstore`: `Delete` несуществующего объекта — `Memory` и MinIO ведут себя по-разному.**
`shared/objstore/objstore.go:76-78` (doc `Client.Delete`), `shared/objstore/memory.go:129-137`,
`shared/objstore/minio.go:165-174`.

Контракт интерфейса записан прямо в коде: «Delete removes the object; removing one that is not
there is ErrNotFound, so a caller that means to be idempotent says so». `Memory.Delete` его
соблюдает, `minioClient.Delete` — нет: S3 `DeleteObject` отвечает `204` и на отсутствующий ключ,
поэтому `RemoveObject` возвращает `nil`.

Проверено прогоном против собранного образа MinIO (временный тест, в индекс не добавлялся):

```
minio  Delete(missing) = <nil>                                          ErrNotFound=false
memory Delete(missing) = objstore: object not found: entities-parity/…  ErrNotFound=true
```

Почему это Major, а не Minor:

- это ровно тот класс расхождений, который `memory.go:17-19` обещает не допускать («fails the same
  way the server does, so a unit test written against it does not pass while the integration test
  against MinIO fails»), причём расхождение здесь в обратную, более опасную сторону: unit-тест на
  `Memory` **зелёный** там, где продакшен молча делает другое;
- потребитель — EPIC-002 (State: удаление сущности, ротация снапшотов K=5), который по плану пишет
  unit-тесты на `Memory`. Код вида `if errors.Is(err, ErrNotFound) { /* уже удалено */ }` будет
  протестирован и не сработает ни разу на живом хранилище;
- ни unit-, ни integration-тест этот кейс не покрывают, то есть расхождение не зафиксировано.

Как исправить (предпочтительно первый вариант — он дешевле и соответствует семантике S3):

а) принять идемпотентность: убрать проверку существования из `Memory.Delete` (оставив
   `ErrNoBucket`), переписать doc `Client.Delete` на «удаление отсутствующего объекта — не ошибка»,
   поправить `TestMemoryDelete`/`TestMemoryReportsMissingObjectAndBucket` и **добавить кейс в
   `integration_test.go`**, чтобы контракт был закреплён с обеих сторон;
б) либо сохранить `ErrNotFound` и добавить в `minioClient.Delete` предварительный `StatObject` —
   но это лишний round-trip на каждом удалении и гонка между Stat и Remove; не рекомендую.

#### Minor

**Mi-1. `cmd/multiverse health`: дефолт `--url` собирается конкатенацией listen-адреса.**
`cmd/multiverse/health.go:26`.

Было: `fs.String("url", "http://"+defaultCoreAddr+"/health", …)`, где `defaultCoreAddr` —
константа `127.0.0.1:8090`. Стало: `"http://"+env.CoreAddr.String()+"/health"`. Но `MV_CORE_ADDR` —
адрес **прослушивания**, а не адрес клиента, и его штатное значение по `infrastructure.md` §4.2,
`.env.example` и `.github/ci.env:59` — `:8090`. Тогда дефолт превращается в `http://:8090/health` —
URL с пустым хостом.

Проверено экспериментом: `http.Client.Get("http://:PORT/health")` даёт `200` только когда прокси не
настроен; на этой машине заданы `HTTP_PROXY=http://127.0.0.1:10808` и
`NO_PROXY=localhost,127.0.0.1,::1,.local` — пустой хост под `NO_PROXY` не подпадает, запрос уходит
в прокси и возвращает **503 Service Unavailable** вместо опроса локального сервера. При
`MV_CORE_ADDR=0.0.0.0:8090` получится `http://0.0.0.0:8090/health`, что на Windows не соединяется.

Контейнеры не задеты: `docker-compose.yml:152` и `:207` передают `--url` явно
(`http://127.0.0.1:8088/health` и `http://127.0.0.1:8090/health`). Страдает ручной запуск
`multiverse health` на хосте — то есть ровно тот сценарий, ради которого подкоманда и делалась.

Как исправить: разобрать адрес и подставить хост для клиента —
`host, port, err := net.SplitHostPort(env.CoreAddr.String())`; при `host == ""`, `"0.0.0.0"` или
`"::"` использовать `127.0.0.1`. Одна функция на 6 строк плюс табличный тест.

**Mi-2. `shared/logging`: цена одной строки лога выросла вчетверо, аллокаций — в десять раз.**
`shared/logging/logging.go:196-206` (`Redact`), `:210-222` (`redactAttr`).

`Redact` прогоняет два `regexp.ReplaceAllString` по **каждой** строковой паре, включая `msg` и
атрибуты `service`/`version`. `ReplaceAllString` копирует строку в новый буфер даже когда
совпадений нет (`replaceAll` всегда делает `append(buf, src[lastMatchEnd:]...)` и `string(...)`),
поэтому платятся не только два прохода regexp, но и две лишние аллокации на строку.

Замер (13900K, `-benchtime=200000x`, `Output: io.Discard`, временный бенчмарк, в индекс не
добавлялся):

```
BenchmarkRedactNoMatch        2634 ns/op   290 B/op    6 allocs/op   // строка 60 символов, совпадений нет
BenchmarkLogLine              2162 ns/op   403 B/op   21 allocs/op   // logging.New + 2 строковых атрибута
BenchmarkLogLinePlainSlog      548 ns/op    96 B/op    2 allocs/op   // тот же JSONHandler без ReplaceAttr
```

Для `info`-уровня на горячем пути обработки событий это заметно. Замечание не о микрооптимизации:
редакция обязана остаться, но её надо сделать бесплатной в типичном случае «совпадений нет».

Как исправить: в `Redact` — быстрый выход до аллокации, `if !re.MatchString(s) { continue }`
(`MatchString` не аллоцирует), либо ещё дешевле — предфильтр
`if strings.IndexByte(s, ':') < 0 && !strings.Contains(s, "sk-") { return s }` перед циклом по
шаблонам. Существующий `TestRedact` изменений не потребует.

**Mi-3. `shared/logging`: список ключей редакции не покрывает ПДн, названные в threat-model.**
`shared/logging/logging.go:35-46`.

Покрыто `infrastructure.md` §7.1 (`external_id`, `token`, `authorization`, `api_key`, `text`) плюс
разумные соседи. Не покрыто то, что threat-model перечисляет прямо: `chat_id`, `first_name`
(T-06 «библиотека бота логирует обновления: username, chat_id, текст», T-08 «`username`,
`first_name`, `chat_id`, `update_id` в чистом виде», §4.2 «Telegram user id, `chat_id`» — оба
пункта оценены **Major**). `username` есть, `chat_id`/`first_name`/`last_name` — нет.

Как исправить: добавить в `sensitiveKeys` ключи `chat_id`, `first_name`, `last_name`, `update_id`.
Правка в четыре строки; в `dev-log.md` §7 автор сам записал этот класс риска («ключ, названный
иначе, пройдёт») — здесь речь не о гипотетическом ключе, а о четырёх поимённо названных в
threat-model.

**Mi-4. Обязательность инфраструктурных переменных потеряна в манифесте.**
`shared/env/infra.go:24-33`.

`infrastructure.md` §4.2 помечает `MINIO_ROOT_USER` и `MINIO_ROOT_PASSWORD` как «обязательна»,
`NEO4J_PASSWORD` — «обязательна при профиле memory». В `infra.go` у всех трёх нет ни `Required()`,
ни `RequiredWhen()` — только `Tooling()` и `Secret()`. Флаг ничего бы не сломал: `validate`
(`env.go:410-412`) снимает безусловную обязательность именно у `scope=tooling`, то есть
`Required()+Tooling()` — это «в манифесте записано, процесс не валится». Зато без флага
`mvctl env check` (T-010) не сможет выполнить проверку «(г) обязательные переменные без дефолта»
из §4.2 по этим трём именам, и единственной страховкой остаётся `${VAR:?}` в compose.

Как исправить: добавить `Required()` к `MINIO_ROOT_USER`/`MINIO_ROOT_PASSWORD`; для
`NEO4J_PASSWORD` условие «профиль memory» через `RequiredWhen` не выражается (`COMPOSE_PROFILES` —
список, а `conditionMet` сравнивает значение целиком) — либо оставить как есть с комментарием,
либо завести отдельный вид условия. Второе — кандидат в бэклог, не в эту задачу.

**Mi-5. `Validate` не проверяет тип значения — ошибка вылезает в рантайме, а не в `env check`.**
`shared/env/env.go:400-420`.

Валидация смотрит только на «пусто/не пусто» и на enum. Значения `MV_SNAPSHOT_EVERY_FACTS=abc`,
`MV_TELEGRAM_POLL_TIMEOUT_S=25s`, `MV_LLM_CLOUD_ENABLED=yes` проходят `Validate` без единой жалобы
и падают только при первом `Int()`/`Bool()`/`Duration()` — то есть уже внутри процесса, а не в
`mvctl env check`, ради которого весь манифест и заведён. Для `MV_LLM_CLOUD_ENABLED` это заметно:
по ADR-005 доп. 2 п. 3 переменная — гейт облачного эндпоинта, и «не распарсилось» не должно
решаться в момент первого вызова LLM.

Как исправить: добавить в `Var` объявленный вид (`string|int|bool|duration`), задаваемый опцией и
проверяемый в `validate`. Дешёвый паллиатив на сейчас — `OneOf("true","false")` на булевых
переменных манифеста.

**Mi-6. Пустое значение не может перебить непустой дефолт — для allow-list это ловушка.**
`shared/env/env.go:277-288` (`rawFrom`), `:262` (`StringFrom`), `:296-310` (`ListFrom`).

`rawFrom` трактует set-but-empty как unset, поэтому `MV_CORE_ADMIN_CLIENTS=` возвращает дефолт
`operator`, а не пустой список. Оператор, который хочет закрыть `/v1/admin/*`, обнулив список,
получит обратное: доступ клиенту `operator` останется. То же с `MV_GATEWAY_CLIENT_IDS=`.
Поведение унаследовано от `runtime.envOr` из T-003 (там было так же), так что регрессии нет, но в
`shared/env` оно стало правилом для всех 68 переменных и нигде не описано.

Как исправить: минимум — записать это в doc-комментарии `String`/`List` вместе с рецептом («чтобы
закрыть список, задайте несуществующий идентификатор»); лучше — для списков различать unset и
empty (`ListFrom` возвращает `nil` при заданном пустом значении, а дефолт применяет только при
unset).

**Mi-7. Правки вне списка файлов T-007 не отмечены как отклонение.**
`shared/runtime/http.go`, `shared/runtime/http_test.go`, `cmd/multiverse/serve.go`,
`cmd/multiverse/health.go`, `.golangci.yml`, `go.mod`.

`tasks.md` T-007 «Файлы» перечисляет только `shared/{objstore,env,logging}/**`. По `ownership.md`
v0.4 §1 строка 11 `shared/runtime/**` — «общий код (часть C-01 v1.1) … правки — запрос»; изменён
публичный API пакета: удалены экспортированные `EnvAddr` и `DefaultAdminClients`,
`EnvAdminClients` из `const` стал `var`. По существу возражений нет — удаление `EnvAddr` было
заранее согласовано в T-003 (в комментарии к хелперу прямо написано «shared/env (F-5, T-007)
replaces this helper»), а без этого пункт «`os.Getenv` вне `shared/env` устранён» невыполним. Но:

- в `dev-log.md` §3 «Отклонения от дизайна» расширение списка файлов не зафиксировано (в §1 оно
  описано как факт, без пометки «вне списка»);
- по прецеденту T-005 коммит с изменением публичного API общего кода помечается `contract-change`
  (`contracts.md` §16 п. 3) — здесь пометки нет;
- `tasks.md` T-007 «Файлы» стоит расширить, иначе история правок общего кода не читается.

Как исправить: внести отклонение в `dev-log.md` §3 и пометить коммит; решение о правке `tasks.md` —
за tech-lead#1 (аналог ОВ-6 из T-003).

**Mi-8. `go.mod`: прямые зависимости помечены `// indirect`.** `go.mod:16-79`.

`github.com/minio/minio-go/v7 v7.3.0` — прямая зависимость `shared/objstore/minio.go`,
`testcontainers-go` и `modules/minio` — прямые зависимости `integration_test.go`, но все три стоят
во втором блоке `require` с `// indirect`. Причина известна и объявлена (ОВ-29: `go mod tidy` не
запускался, потому что в дереве параллельно шла T-006), решение оркестратора — выполнить после
приёмки подволны 0.3. Фиксирую как незакрытое: пока `tidy` не прогнан, `go mod tidy -diff` (или
`git diff --exit-code go.mod go.sum`) в job `unit`/`contracts` T-012 будет красным, и это
единственная причина.

**Mi-9. Обязательные поля строки лога расходятся с `infrastructure.md` §7.1.**
`shared/logging/logging.go:69-88`, `shared/logging/logging_test.go:78-80`.

§7.1 перечисляет `ts, level, service, context, msg, correlation_id, event_id, event_type,
agent.level?, handled, version`. `slog.JSONHandler` пишет поле времени как **`time`**, а не `ts`;
переименования через `ReplaceAttr` нет, и тест `TestLoggerCarriesTheRequiredFields` закрепляет
именно `time`. Отдельно: `version` объявлено обязательным, но `Init(service, level)` его не ставит
(`Options.Version` пуст → атрибут не добавляется), а линкер пока ничего не штампует.

Как исправить: либо добавить в `redactAttr` переименование `slog.TimeKey → "ts"` (одна ветка) и
завести `-ldflags -X` для версии в `build/Dockerfile` (T-004), либо поправить §7.1 на `time` и
пометить `version` как «появляется с F-7». Решение — за tech-lead#1/devops; сейчас документ и код
расходятся молча, и `jq`-рецепты из §7.1 писались под другой ключ.

**Mi-10. `EnsureBucket` не приводит уже существующий бакет к объявленным правилам.**
`shared/objstore/minio.go:180-217`.

Метод сходится только в одну сторону: versioning **включается** при `opts.Versioned=true`, но
никогда не снимается при `false` (нет `SuspendVersioning`), а lifecycle не удаляется, когда правил
нет (`lifecycleOf` возвращает `nil` → вызова просто не происходит). Между тем бакеты создают двое:
`EnsureBucket` и `build/minio-init.sh` (T-008), плюс возможен `mc` вручную. Если `prompts-{world}`
окажется создан с versioning, платформа этого не заметит и не исправит, а SEC-22 требует обратного
(«для `prompts-*` versioning выключен»). Integration-тест проверяет только сценарий с чистым
бакетом, поэтому расхождение не поймается.

Как исправить: при `opts.Versioned == false` проверять `GetBucketVersioning` и вызывать
`SuspendVersioning`, если включён (либо явно записать в doc-комментарии, что приведение к правилам
делается только при создании, и вынести сходимость в `mvctl storage check`); добавить кейс «бакет
создан с versioning → `EnsureBucket` его снял» в `integration_test.go`. Согласовать с T-008, чтобы
`minio-init.sh` брал правила из той же таблицы `BucketOptionsFor`, как обещает её doc-комментарий.

#### Nit

**N-1. `TestMinIOReportsAMissingBucket` — слабая и небезопасная проверка.**
`shared/objstore/integration_test.go:208-217`. При `err == nil` тест не падает с внятным
сообщением, а паникует на `err.Error()`. Второй дизъюнкт
`strings.Contains(err.Error(), "NoSuchBucket")` делает проверку `translate` необязательной: тест
останется зелёным, даже если маппинг в `ErrNoBucket` сломается и наружу пойдёт сырая ошибка
сервера. Оставить только `errors.Is(err, objstore.ErrNoBucket)`.

**N-2. `Memory` игнорирует `context.Context` во всех методах.** `shared/objstore/memory.go:57`,
`:82`, `:101`, `:113`, `:124`, `:130`. У `minioClient` отменённый контекст даёт ошибку, у `Memory` —
нет. Тот же класс расхождения, что M-1, но безобидный: строка
`if err := ctx.Err(); err != nil { return err }` в начале каждого метода выровняла бы поведение и
позволила тестам EPIC-002 проверять отмену на фейке.

**N-3. `runtime.EnvAdminClients` — экспортированная изменяемая переменная вместо константы.**
`shared/runtime/http.go:24-28`. `var EnvAdminClients = env.CoreAdminClients.Name()` — на месте
бывшей константы теперь пакетный `var`, который любой импортёр может переприсвоить; используется
он только в тексте ответа `403`. Проще звать `env.CoreAdminClients.Name()` по месту в `forbid(...)`
и не заводить экспортированное имя вовсе.

**N-4. Исключение `forbidigo` шире необходимого.** `.golangci.yml:188-192`. Правило
`path: shared/(clock|env)/` с текстом `(os\.(…)|time\.(…))` разрешает пакету `shared/env` ещё и
`time.Now`/`time.After`, которые ему не нужны (проверено: в `shared/env` их нет). Само сужение
дыры не открыло — grep по `shared/{runtime,logging,objstore,contracts}` не находит ни одного
запрещённого вызова вне тестов, и `golangci-lint run ./...` даёт `0 issues`. Можно разнести на два
правила (`shared/clock/` → time, `shared/env/` → os), чтобы исключение было ровно по обязанности.

**N-5. Пустое значение чувствительного ключа превращается в `[redacted]`.**
`shared/logging/logging.go:211-213`. `redactAttr` подменяет значение, не глядя на него, поэтому
`slog.String("token", "")` даст `"token":"[redacted]"` — в логе появляется намёк на токен там, где
его не было. Проверять `a.Value.String() != ""` перед подменой.

**N-6. Расхождение в значении `MV_LLM_URL` между документами.** `tasks.md` T-007 (сведение 3)
называет `http://host.docker.internal:1234/v1`, `infrastructure.md` v0.3 §4.2 и `.env.example` —
`http://host.docker.internal:1234` без `/v1`. Реализация взяла второе (источник истины, тот же
принцип, что в ОВ-25), это правильно. Но расхождение стоит закрыть до EPIC-003: провайдеру
`openai_compat` важно, добавляет он `/v1` сам или ждёт его в переменной.

### Отдельно проверено и признано неверным подозрением

- **Утечка горутины в `minioClient.List`.** `List` выходит из `range` по каналу при первой ошибке,
  не отменяя контекст, а продюсер `minio-go` пишет в канал через `select` с `ctx.Done()` — то есть
  ранний выход мог бы подвесить горутину до отмены контекста вызывающего. Проверено по исходникам
  `minio-go v7.3.0` (`api-list.go:774-807`; `listObjectsV2` — `yield(ObjectInfo{Err: err}); return`):
  ошибка в итераторе **терминальна**, после неё продюсер сам завершается и закрывает канал. Утечки
  нет, замечание не выставляю. На будущее: если в `List` появится ранний выход по лимиту или
  `break` не по ошибке, понадобится `lctx, cancel := context.WithCancel(ctx); defer cancel()`.
- **Секреты на путях логирования и ошибок.** Прочитаны все пути, где значение переменной может
  попасть в текст: `Var.errf` (`env.go:386-392`) кладёт `Value` только при `!v.secret`;
  `Error.Error()` при `Secret` печатает «value withheld»; `Validate` наследует то же;
  `Deprecated()` сообщает имя и причину, значения не читает; `objstore.New` (`minio.go:66-72`)
  называет только имена переменных; `ConfigFromEnv` значения не логирует. Тесты
  `TestSecretValueNeverReachesTheError`, `TestValidateRealManifestNeedsTheObjectStoreCredentials`,
  `TestDeprecatedNamesAreReportedWhenStillPresent` это закрепляют. Замечаний нет, ADR-009 п. 4
  соблюдён.
- **Паника на этапе объявления** (`Declare`/`DeclareExternal`/`DeclareDeprecated`) — оправдана: все
  пять условий (нет префикса, невалидное имя, пустое описание, дубль, `Required` с дефолтом) —
  ошибки программиста, видимые при загрузке пакета, до того как процесс что-либо обслужит; каждая
  покрыта тестом через `isolate`+`mustPanic`. Потокобезопасность реестра: все операции под
  `sync.RWMutex`, `Manifest`/`Lookup`/`DeprecatedNames` возвращают копии, вложенных захватов нет
  (`Validate` вызывает `Manifest` до цикла, `conditionMet`→`Lookup` берёт `RLock` уже после
  возврата) — дедлока нет.
- **`AdminOnly` не сломан.** Логика допуска не изменилась: список читается один раз при построении
  middleware, `X-Client-Id` сверяется со списком из `MV_CORE_ADMIN_CLIENTS` (дефолт `operator`,
  fail-closed), `X-Actor-Kind` необязателен и только валидируется по enum `human|ci|sim|system`.
  Единственная разница — `env.Var.List()` дополнительно обрезает пробелы у значения целиком (было —
  только у элементов); поведение эквивалентно. Все кейсы `TestAdminOnly` и
  `TestAdminOnlyReportsReason` из T-003 сохранены и зелёные; удалён только `TestEnvAddr` вместе с
  удалённым хелпером. Поведение `/health` не менялось: `runtime.Routes`/`Aggregate` не тронуты,
  `cmd/multiverse` изменил лишь источник адреса (следствие — Mi-1).
- **Ретраи `objstore` — только транзиентные.** `retryable` (`minio.go:281-296`) исключает
  `ErrNotFound`, `ErrNoBucket` и `context.Canceled`, повторяет только `code==0` (сетевая), `429` и
  `5xx`; `List` не ретраится осознанно; `wait` проверяет `ctx.Err()` до постановки таймера (в
  комментарии объяснено, почему `select` по двум готовым каналам не годится) и работает через
  `clock.Timers`, поэтому тесты ретраев детерминированы и не спят. `context.DeadlineExceeded`
  формально попадает в «транзиентные» (`code==0`), но следующий же `wait` возвращает `ctx.Err()`,
  так что цикл ограничен — отдельного замечания не выставляю.

### Предложения в бэклог

1. **Привести `foundation.md` §8 и `ownership.md` v0.4 §1 (строка 35) в соответствие с принятым
   централизованным `vars.go`.** Оба документа предписывают «владелец процесса/контекста объявляет
   через `shared/env.Declare`» рядом со своим кодом; код делает иначе, и решение оркестратора это
   утвердило. Пока документы не поправлены, разработчик EPIC-002/003/004 по инструкции добавит
   `Declare` у себя. Отчасти это поймается (`CheckExample` сообщит «present in .env.example but not
   declared», если пакет не импортирован `mvctl`), но диагностика будет непонятной. Владелец:
   architect#1 при ревизии §8, tech-lead#1 — по `ownership.md`.
2. **Расширить `shared/env` признаком «нужна такому-то контексту»** — без него решение ОВ-27
   («`env.Validate` при старте проверяет только включённые контексты») не реализуемо. Кандидат:
   опция `ForContexts("state","memory")` и `env.ValidateFor(src, contexts…)`. Владелец: T-010
   совместно с EPIC-002; `shared/env` — общий код.
3. **Согласовать `build/minio-init.sh` (T-008) с `BucketOptionsFor`.** Doc-комментарий таблицы
   обещает, что init-контейнер и платформа читают правила из одного места; сейчас это обещание
   ничем не подкреплено, а расхождение versioning на `prompts-*` — прямое нарушение SEC-22 (см.
   Mi-10). Кандидат в DoD T-008: `minio-init.sh` вызывает `mvctl storage init` либо правила в нём
   сверяются тестом.
4. **`privacy-scan` (NFR-041, EPIC-005) обязателен независимо от редакции.** Разделяю оценку автора
   в `dev-log.md` §7: редакция работает по фиксированному списку ключей и двум шаблонам, поэтому
   ключ с другим именем и токен другого формата (JWT `eyJ…`, `Bearer` без `sk-`, секрет MinIO,
   пароль Neo4j) пройдут. Mi-3 закрывает поимённо названные в threat-model случаи, но не класс.
5. **Запретить `MV_LOG_LEVEL=debug` при `MV_ENV=prod`** (`infrastructure.md` §7.1 «`debug` не
   включается в `prod`»). Сейчас это только текст в описании переменной. Кандидат: правило в
   `mvctl env check` (T-010) — кросс-проверка двух переменных, та же механика, что `RequiredWhen`.
6. **Замерить паузы ретраев `objstore` (100 мс / 500 мс).** Взяты из `foundation.md` §7 («3
   попытки»), значения не обосновывались (`dev-log.md` §7 это признаёт). Кандидат в замер
   T-014/EPIC-005 вместе с NFR по задержкам.

### Риски и допущения

- `-race` локально не прогонялся (нет cgo/gcc) — по решению оркестратора уходит в CI (T-012).
  Гонки в границах T-007 вероятнее всего в реестре `shared/env` (пакетное состояние, а `isolate` в
  тестах подменяет его целиком) и в `objstore.Memory` (`sync.RWMutex` + копирование тел). Оба места
  написаны так, чтобы `-race` их покрывал; `isolate` дополнительно требует, чтобы тесты
  `shared/env` никогда не звали `t.Parallel()` — сейчас не зовут, но это ничем не защищено.
- Integration-тест `objstore` поднимает **три** контейнера MinIO (по одному на тест-функцию);
  локально это 5,5 с, в CI на холодном образе будет заметно дороже. Если job `integration` T-012
  упрётся в 10 минут (§1 п. 8) — кандидат на общий контейнер через `TestMain`.
- Покрытие `shared/objstore` 65 % — самое низкое из трёх, и почти весь непокрытый код это
  `minioClient.Put/Get/Stat/List/Delete/EnsureBucket`, проверяемые только под тегом `integration`.
  Это осознанный размен (unit-тесты покрывают `translate`, `retryable`, `retry`, `lifecycleOf`,
  `New`, `ConfigFromEnv`), но он же объясняет, почему M-1 не был замечен: расхождение живёт ровно в
  той части, которую видит только integration-тест. Отсюда рекомендация закреплять контракт
  `Delete` **обеими** сторонами.
- Сверку манифеста с `.env.example` я проверил дважды: тестом задачи
  (`TestRepositoryExampleMatchesTheManifest`) и независимо — выгрузкой `env.Manifest()` во временный
  пакет и сравнением множеств имён с `.env.example` и с `infrastructure.md` §4.2. Совпадение точное
  (68 = 68). Временные файлы (`shared/env/tmpdump/`, `shared/objstore/zz_tmp_*`,
  `shared/logging/zz_bench_tmp_test.go`) удалены, в индекс не добавлялись; рабочее дерево после
  ревью — в том же состоянии, `git status` не изменился.
- Файлы T-006 (`shared/contracts/**`, `schemas/**`) не смотрел; они присутствуют в индексе и
  участвуют в прогонах `go build`/`go test`/`golangci-lint`, но все прогоны зелёные, так что
  разделение областей на результат не повлияло.
- Ничего не правил и не коммитил: изменена только эта запись в `review.md`.

---

## T-009 · ревью #1 · 2026-09-10 · code-reviewer#1 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `0c92f2a` (T-006 + T-007), изменения **в индексе**
(`git diff --cached`), коммита нет. Ревьюировались только файлы T-009:

| Действие | Путь |
|---|---|
| A | `schemas/events/*.v1.json` — 31 схема блока «в» |
| M | `shared/contracts/registry.go` (+137 строк: 31 запись, `worldEvent`, `swarmEvent`, `reserved`) |
| M | `shared/contracts/spec.go` (+8: поле `Spec.Reserved`) |
| A | `shared/contracts/blockv_test.go` (631 строка) |
| M | `shared/contracts/{registry_test.go,validate_test.go}` — приведение тестов T-006 |
| M | `dev-log.md` — запись «developer#1 · T-009» |

Файлы T-008 (`docker-compose.yml`, `Makefile`, `scripts/**`, `build/**`, `.github/ci.env`) и
незастейдженные правки других ролей не рассматривались — по указанию оркестратора их пишет
второй разработчик прямо сейчас.

Основание: `tasks.md` §1 (общий DoD) и §5 (T-009); `architecture/contracts.md` v0.4 §0, C-01,
C-05, C-06, C-07, C-10, C-12, C-14, §16 п. 3, п. 4, п. 7; `architecture/components/foundation.md`
v0.2 §6; `analysis/api-contracts.md` v0.2.1 §2.0–§2.3; `analysis/data-model.md` §7.2, §7.4;
`project/metrics.md` §4.2; `epics/EPIC-005-memory-ops/design.md` §4.1, §7, §12 п. 2; ADR-005 доп. 3,
ADR-007, ADR-014, ADR-016, ADR-017 доп. 1; `journal.md` — решения оркестратора по ОВ-33…ОВ-38
(2026-09-10).

### Вердикт

**Вернуть.** Critical: 0 · Major: 1 · Minor: 7 · Nit: 4.

Работа сильная. Состав блока «в» полон и точен: 31 тип — это ровно карта §2.2 минус то, что уже
внесено блоками «а»/«б», плюс `combat.decided` и `config.cloud_enabled`, которых нет в буквальном
перечне описания задачи, но которые обязаны появиться здесь, иначе «нет фантомов» в T-010 упадёт.
Лишних типов нет; `rules.change.*` не зарегистрированы — ровно решение ОВ-36. Схемы аккуратно
следуют §2.3 и `data-model.md` §7.2/§7.4; enum-ы `validation_status` (6 значений), `reason`
(10 значений), `end_reason` (+`forget`), `parse.strategy` (5 стратегий ADR-016 п. 1) совпадают с
контрактами пословно и закреплены «часовыми»-тестами. Реестр полон, `TestEveryTopicCarriesAType`
закрывает пункт 2 бэклога ревью T-006 раньше T-010. Прогоны зелёные, покрытие 81,5 % при пороге 81 %.

Возврат — из-за одного Major: решение оркестратора по ОВ-33 (`response_raw` запрещён **и при**
`filter_error`) в схему не внесено. Дата решения — 2026-09-10, дата записи в dev-log — та же;
похоже, решение пришло после сдачи. Правка — одна строка в `llm.output.v1.json`, один кейс в
тесте и два абзаца в dev-log (§2 п. 5, §5 ОВ-33). Остальное — Minor/Nit, из них три пункта
(Mi-1, Mi-3, Mi-4) закрываются не кодом, а согласованием с system-architect.

### Проверенные пункты DoD

| Критерий T-009 | Результат |
|---|---|
| все схемы блока «в» компилируются | ✔ `TestSchemasValid` + `New()` компилирует каждую при сборке реестра |
| схемы покрывают примеры §2.3 | ✔ `TestBlockVPayloadExamples` — 31 пример; полнота таблицы жёстко связана с реестром в `TestPayloadExamples` (`len(cases)+len(blockVExamples) == countTypesWithSchema()`) |
| «нет фантомов» в обе стороны | ✔ `TestNoPhantoms`: 56 файлов `*.v1.json` ↔ 56 не-deprecated `Spec` (25 от T-006 + 31 от T-009), 9 legacy без схем |
| реестр полон по §2.2 (`Publishers`/`Consumers`/`Policy`) | ✔ сверено построчно по таблице §2.2; единственный тип §2.2 без записи — `dead_letters`, и это обёртка шины, а не тип (по составу `Publishers` одного типа — см. Mi-1) |
| у каждого `TopicSpec` есть `Spec` (кроме `dead_letters`) | ✔ `TestEveryTopicCarriesAType`, исключение для `dead_letters` проверяется в обе стороны (типов там быть не должно) |
| `analytics.consistency.violated`: `code ⊇ {state_divergence, log_gap, invariant}`, `severity ∈ {break, warn}`, `detected_by` | ✔ все три условия; `code` — объединение девяти значений `metrics.md` §4.2 и двух из C-10 v0.3, `detected_by` в `required` |
| retention 30/90/180 | ✔ `TestTopics`; `dead_letters` = 30 дн., `ReplayRead=false` (добавлено в T-009) |
| `player_events` отклоняет `actor_kind=system` | ✔ `TestPlayerEventsRejectSystem` — по всем не-deprecated типам топика, плюс положительная ветка `sim` |
| `SwarmPolicy` там, где положено | ⚠ прямой список §2.0 покрыт полностью; расширение на 8 типов сверх списка — см. Mi-3 |
| покрытие ≥ 81 % | ✔ **81,5 %** (перепроверено ревьюером) |

| Общий DoD §1 | Результат (перепроверено ревьюером) |
|---|---|
| `go build ./... && go vet ./...` | ✔ обе пусты |
| `go test -short -count=1 ./...` | ✔ 10 пакетов `ok`, ни одного `FAIL` |
| `go test -short -count=1 -cover ./shared/contracts/` | ✔ `81.5% of statements` |
| `golangci-lint run ./...` | ✔ `0 issues` |
| `gofmt -l shared schemas` | ✔ пусто |
| `gitleaks git --staged --redact .` | ✔ `no leaks found` (~100 КБ просканировано) |
| `go test -race` | n/a локально (нет cgo/gcc), уходит в CI T-012 |
| dev-log заполнен | ✔ 6 разделов: сделано, решения (10), отклонения, DoD, ОВ-33…ОВ-38, риски; см. M-1 (§2 п. 5 и §5 ОВ-33 устарели после решения оркестратора) и N-1 (имя экземпляра) |
| изменений вне карты владения нет | ✔ `schemas/events/**` и `shared/contracts/**` — EPIC-001 по `contracts.md` §0; создание схем для типов чужих эпиков в этой задаче предписано прямо (C-10 v0.3, `EPIC-005/design.md` §12 п. 2) |
| контракты не меняются без system-architect | ⚠ `Spec.Reserved` и расширение `SwarmPolicy` — см. Mi-3, Mi-4 |

### Сверка с решениями оркестратора (`journal.md`, 2026-09-10)

| ОВ | Решение | Внесено |
|---|---|---|
| ОВ-33 | `response_raw` запрещён и при `filter_error` | ✗ **нет** — M-1 |
| ОВ-34 | имя поля — `external_players_ack` | ✔ `config.cloud_enabled.v1.json:12` |
| ОВ-35 | `world`/`scope` в аналитике — опциональны | ✔ все четыре `analytics.*` схемы блока «в» |
| ОВ-36 | `rules.change.*` не регистрируем; механизм `Reserved` готов | ✔ в реестре их нет; `TestReservedTypes` фиксирует ровно пять `world.law_breach.*` |
| ОВ-37 | `max.message.bytes` — в T-010 | ✔ в `TopicSpec` не добавлялось, ОВ передан в T-010 |
| ОВ-38 | `tick.aborted.reason`, `narrative.output.fallback_reason` — строки | ✔ обе строки, в описаниях схем указано почему |

### Замечания

**M-1 (Major). `schemas/events/llm.output.v1.json:100–107` — решение оркестратора по ОВ-33 не
внесено: при `validation_status=filter_error` схема по-прежнему разрешает `response_raw`.**
Конструкция `if/then` завязана на `const: "quarantined"`, поэтому запись с
`validation_status=filter_error` и полным сырым текстом ответа проходит валидацию и уходит в
`llm_records` с retention 90 дней. Журнал (2026-09-10) закрывает расхождение C-07 v1.2 ↔
ADR-016 п. 3 в пользу C-07: «`response_raw` НЕ сохраняется и при `filter_error` — приватность
важнее отладки»; ADR-016 правит system-architect, схема должна следовать C-07 уже сейчас.
Цена ошибки — не отказ издателю, а обратное: текст, который fail-closed не пустил игроку, остаётся
на диске.
*Правка*: заменить условие на перечисление —
`"if": { "properties": { "validation_status": { "enum": ["quarantined", "filter_error"] } }, "required": ["validation_status"] }`,
`$comment` в `then` дополнить ссылкой на решение ОВ-33; в `blockv_test.go:370`
`TestQuarantinedRecordKeepsNoText` добавить кейс `filter_error` с `response_raw` (ожидание —
`ErrInvalidPayload`) и переименовать тест так, чтобы имя не обещало только карантин; в
`dev-log.md` §2 п. 5 и §5 ОВ-33 записать решение вместо описания неопределённости.

**Mi-1 (Minor). `shared/contracts/registry.go:181–182` — у `analytics.consistency.violated`
лишний издатель `testkit/state`.** `EPIC-005/design.md` §7 говорит прямо: «единственная запись в
шину — `analytics.consistency.violated` и `replay.completed mode=test` с `meta.actor_kind=ci`,
**`source=mvctl`**»; §4.1 и строка 80 того же файла называют издателями `mvctl --audit` и golden —
и то и другое исполняется как `mvctl`. `contracts.md` §0 («Исключение для заглушек») разрешает
фейку публиковать типы **подменяемого** контракта: `testkit/state` подменяет State (C-02, C-14),
поэтому `analytics.replay.completed` у него законен, а C-10 — нет. Практический эффект: проверка
`source ∈ Publishers` (§16 п. 7) в T-010 станет на один источник слабее для типа, владелец
которого этого не просил.
*Правка*: `[]string{SourceMvctl}` в `Publishers`; если нужен фейк для тестов EPIC-005 — заводить
его как `testkit/ops` отдельным запросом владельцу типа (§16 п. 1).

**Mi-2. `schemas/events/llm.output.v1.json:32, 48` — условная обязательность `response_raw` и
`filter` не выражена.** `data-model.md` §7.2 помечает `response_raw` как «условно» (обязателен,
кроме `quarantined`) и `filter` как «при тексте»; §2.3.10 перечисляет `response_raw` в списке
обязательных полей с оговоркой в скобках. В схеме оба поля просто опциональны, поэтому запись
`validation_status=valid` без единого следа ответа (`response_raw` нет, `filter` нет) валидна —
а именно такая запись ломает `providers/recorded` в replay, ради которого топик и существует.
Структура для этого уже есть: `if/then` из M-1 достаточно превратить в `if/then/else`.
*Правка*: добавить ветку «`validation_status ∈ valid|partially_rejected|invalid` →
`required: [response_raw]`» (при `error` ответа нет, при `quarantined`/`filter_error` он запрещён —
три ветки покрывают все шесть статусов); по `filter` — либо `required` при
`validation_status ∈ quarantined|filter_error`, либо оставить как есть и записать это решением в
dev-log, чтобы владелец (EPIC-003) увидел его в T-214/T-215.

**Mi-3. `shared/contracts/registry.go:86–144, 239–243` — `SwarmPolicy` расширена за пределы списка
C-01, и документ об этом не знает.** Прямой перечень §2.0 и C-01 («Валидация при чтении») —
`llm_records`, `system_events(tick.*, agent.*)`, `narrative_output`, `combat.decided`. В поставке
политика стоит ещё на восьми типах: `encounter.started/ended`, `world.weather_changed`,
`world.time_advanced`, `world.event_occurred`, `region.event_occurred`, `npc.moved`, `npc.spawned`,
`content.incident.recorded`. Обоснование в dev-log §2 п. 1 я принимаю по существу — §2.3.8 говорит
«Все — с `meta.agent{level global|domain}`», §2.3.15 «`agent`, `scope` — в конверте», C-05
подтверждает `meta.agent` у фейков — но это ужесточение общего контракта: шина будет отклонять при
публикации и при чтении то, что текст C-01 разрешает. Пока политика и документ расходятся, любой
будущий издатель этих типов без агента (например, `mvctl` при загрузке фикстур или E-B) получит
`ErrPolicyViolation` в рантайме, а не на ревью.
*Правка*: не менять код, а провести через system-architect правку §2.0/C-01 (перечень «типы,
требующие `meta.agent`» → «`llm_records`, `narrative_output`, `combat.decided`, `encounter.*`,
`tick.*`, `agent.*`, `world.*` кроме `laws`, `region.*`, `npc.*`, `content.incident.recorded`») и
пометить коммит `contract-change` (§16 п. 3) — как это уже сделано в T-005 и T-006.

**Mi-4. `shared/contracts/spec.go:49–56` — `Spec.Reserved` добавлен в публичный API общего пакета
без отражения в контракте.** Поле обосновано (машинная форма исключения §16 п. 4 и C-12),
аддитивно, существующих потребителей не ломает — `Spec` собирается по именам полей, а
`cloneSpec`/`TestSpecsAreCopies` не затронуты; выбор «обёртка `reserved(...)` вместо флага в
конструкторе» читается на месте вызова и мне нравится. Но C-01 перечисляет `Spec{Topic,
SchemaVersion, Schema, Policy}`, а `foundation.md` §6 (строка 198) даёт полную структуру без
`Owner`, `Policy` (это уже расхождение от T-006) и теперь без `Reserved`. Изменения `shared/*` —
только через system-architect с пометкой `contract-change` (§16 п. 3).
*Правка*: код оставить; в отчёте оркестратору попросить system-architect обновить C-01 и
`foundation.md` §6 одной правкой на оба поля, коммит пометить `contract-change`.

**Mi-5. `schemas/events/encounter.started.v1.json` — `scope` выброшен из payload, хотя в
`tick.fired`/`agent.spawned`/`agent.spawn_rejected` он оставлен.** dev-log §2 п. 4 формулирует
правило: «документ перечисляет поле в payload → оставляем опциональную копию» (§2.3.9,
`data-model.md` §7.4). §2.3.7 перечисляет `scope` в payload `encounter.*` ровно так же, но в схеме
его нет, а `additionalProperties: false` превращает это в отказ издателю, который положил копию.
Любое из двух правил приемлемо; неприемлемо, что внутри одного блока они разные — потребитель не
может вывести, где копия допустима.
*Правка*: либо добавить опциональный `scope` в `encounter.started`/`encounter.ended` (тогда правило
= «как в документе»), либо убрать `scope` из `tick.*`/`agent.*` (тогда правило = «конверт —
единственный источник», как в блоках «а»/«б», где `scope` не оставлен нигде, включая `round.opened`
по решению ОВ-22) и записать выбор в dev-log §2.

**Mi-6. `shared/contracts/blockv_test.go:243–350` — у `combat.decided` нет ни одной негативной
фикстуры.** 25 негативных случаев распределены разумно (три на `tick.fired`, пять на `llm.*`,
три на `analytics.consistency.violated`), но самый нагруженный тип блока — результат механики,
который читают gateway, State и нарратор, — проверяется только положительным примером. Между тем
именно в нём больше всего мест, где опечатка издателя дорога: `action` вне enum, `outcome` без
`natural`, `hp` с одним из трёх полей, `rolls[].index` строкой.
*Правка*: добавить 2–3 кейса на `combat.decided` (минимум — `action` вне enum и `hp` без
`defender_max`); заодно стоит по одному кейсу на `npc.spawned` (`spawned_by` без `agent`) и
`content.incident.recorded` (`category` вне `["a"]`) — это те схемы, где вложенный `required`
написан вручную и никем не проверен.

**Mi-7. Форма `{event: {id, type?}}` продублирована 8 раз, из них 3 раза в одном файле.**
`narrative.output.v1.json` (`llm_output`, `based_on[]`, `background_refs[]`),
`combat.decided.v1.json` (`rolls[]`), `llm.output.rejected.v1.json`,
`content.incident.recorded.v1.json`, `npc.spawned.v1.json` (`spawned_by.tick`) — плюс
`round.closed.v1.json` от T-006. Решение не трогать `_common.json` (dev-log §2 п. 9) правильное —
файл меняется только через system-architect. Но это не мешает объявить локальный `$defs` внутри
каждого файла: три копии в одном файле разъедутся при первой же правке.
*Правка*: в `narrative.output.v1.json` (и по желанию в остальных пяти) добавить
`"$defs": {"EventRef": {…}}` и три `$ref: "#/$defs/EventRef"`; предложение про общий `EventRef`
в `_common.json` оставить в бэклоге, как и предложил разработчик.

**N-1 (Nit). `dev-log.md:1975` — запись подписана `developer#1`, тогда как `tasks.md` §5 назначает
T-009 на developer#2** (а T-008 — на devops). Общий DoD §1 п. 5 требует именно экземпляр; при двух
параллельных задачах несовпадение подписи и плана мешает соотнести запись с исполнителем.
*Правка*: привести подпись к факту распределения или отметить перестановку в шапке записи.

**N-2. `shared/contracts/blockv_test.go:453–510` — списки типов в
`TestSwarmTopicsDemandAnAgent` (20) и `TestTopicsWithoutAnAgentRule` (11) заданы руками.**
Сегодня 20 + 11 = 31 = весь блок «в», но ничто не проверяет, что объединение покрывает реестр:
32-й тип, добавленный завтра, не попадёт ни в один список и его политика останется без теста.
*Правка*: в одном из тестов дополнительно сверить, что множество `Spec` с
`Policy.Agent == AgentRequired` равно первому списку (тогда второй становится следствием).

**N-3. `schemas/events/analytics.turn.completed.v1.json:54` — `narrative.agent_level` опционален,
а `metrics.md` §4.2 перечисляет его среди обязательных подполей `narrative{}`.** Для
`generated_by=none` (`status=rejected`) уровня действительно нет, так что решение защитимо, но в
описании схемы оно не объяснено, а `metrics.md` его не знает.
*Правка*: одна фраза в `description` поля `narrative` («`agent_level` отсутствует, когда нарратива
не было») либо вопрос владельцу (EPIC-004) вместе с ОВ-35.

**N-4. `schemas/events/combat.decided.v1.json` — `round` в `required`.** В `solo` раунды не
публикуются (C-04: «в `solo` раунд = ход и `round.*` не публикуются»), и §2.3.6 не помечает `round`
опциональным только потому, что пример дан для группы. Стоит подтвердить у EPIC-003, что агент
встречи ведёт `seq` и в соло (в `encounter.ended.rounds` он его ведёт, так что скорее да).

### Отдельно проверено и признано верным

- **Состав блока «в».** Пересчитал независимо: 31 файл `A` в индексе ↔ 31 запись между маркерами
  блока «в» в `registry.go` ↔ 31 ключ в `blockVExamples`. Сверил с картой §2.2 построчно: все типы
  таблицы присутствуют, кроме `dead_letters` (обёртка `{original, error, consumer}`, у неё нет
  типа события — и `TestEveryTopicCarriesAType` требует, чтобы в этот топик не был смаршрутизирован
  ни один `Spec`). `combat.decided` и `config.cloud_enabled` в тексте задачи не названы, но названы
  в §2.2; без них «нет фантомов» в T-010 не прошло бы. Включение — верное решение.
- **Маршрутизация.** `combat.decided` → `game_events` (не `world_events`), `encounter.*` →
  `world_events`, `content.incident.recorded` и `config.cloud_enabled` → `system_events`,
  `llm.*` → `llm_records`, `narrative.output` → `narrative_output`, `analytics.*` →
  `analytics_events`, `world.law_breach.*` → `world_events`. Совпадает с §2.2 целиком.
- **`Reserved` не ослабляет проверку.** `TestEveryTypeHasPublisherAndConsumer` по-прежнему требует
  непустые `Publishers`/`Consumers` у **всех** типов, включая зарезервированные, а у них
  заполнено `core/laws` — то есть флаг сегодня не служит лазейкой, а только помечает семейство для
  T-010. `TestReservedTypes` жёстко фиксирует «ровно эти пять» в обе стороны.
- **Приватность.** `llm.output` не несёт промпта (только `prompt_hash`), `content.incident.recorded`
  не несёт текста, `config.cloud_enabled` не имеет места для ключа или URL (проверено негативной
  фикстурой «cloud flag carrying a key»), `analytics.*` — без внешних ID и текста.
  `world.event_occurred.summary`/`region.event_occurred.summary` ограничены 300 символами, как в
  §2.3.8. Единственное место, где сырой текст модели остаётся на диске штатно, — `response_raw`,
  и именно про него M-1.
- **Не является находкой.** Асимметрия «в `llm.output` сквозных полей нет, в `tick.fired` копия
  `agent` есть» — это асимметрия документов (§2.3.10 против §2.3.9 и `data-model.md` §7.4), а не
  разработчика; dev-log §2 п. 2 и п. 4 фиксирует это честно. Вложенность
  `combat.decided.outcome.living_enemies` — допущение, оговорённое в dev-log §6; §2.3.6 читается и
  так и так, правка при расхождении — одна строка.

### Предложения в бэклог

1. **system-architect**: свести C-01 «политики топиков» / `api-contracts.md` §2.0 с фактическим
   перечнем `SwarmPolicy` (Mi-3) и добавить в C-01 поля `Spec{Owner, Policy, Reserved}` (Mi-4) —
   одной правкой, пометка `contract-change`.
2. **system-architect**: `EventRef` (`{event:{id,type?}}`) в `_common.json` — форма встречается в
   семи схемах; предложение разработчика (dev-log §2 п. 9) поддерживаю, но после MVP-1: правка
   общего файла сейчас задевает T-010/T-011.
3. **T-010 (`mvctl contracts check`)**: (а) читать `Spec.Reserved` для исключения §16 п. 4;
   (б) внести `max.message.bytes=4 МиБ` для `llm_records` в `TopicSpec` (ОВ-37) — сейчас значение
   живёт только в скрипте T-008, и два источника разъедутся; (в) проверка «каждый источник из
   `Publishers` — существующий процесс или `testkit/*`» поймала бы Mi-1 машинно.
4. **EPIC-003 (T-214/T-215)**: ревизия `llm.output` (условная обязательность `response_raw`/`filter`
   — Mi-2), словари `tick.aborted.reason` и `narrative.output.fallback_reason` (ОВ-38, сужение
   строки до enum — совместимое), подтверждение вложенности `outcome.living_enemies` и
   обязательности `combat.decided.round` в соло (N-4).
5. **EPIC-004 (T-301) / BA**: `world`/`scope` в payload `analytics.*` остались опциональными
   (ОВ-35) — `metrics.md` §4.2 всё ещё держит их в обязательных, расхождение стоит закрыть текстом,
   а не схемой; там же `narrative.agent_level` (N-3) и отсутствие `correlation_id`/`trigger{event}`
   в `analytics.turn.completed` (dev-log §2 п. 2).

### Риски и допущения

- Ревью по индексу, не по коммиту; `-race` не запускался (нет cgo/gcc, решение оркестратора —
  CI T-012). Нового разделяемого изменяемого состояния T-009 не вносит: `definitions` — пакетная
  таблица, читаемая один раз в `New`; `reserved()`/`swarmEvent()` — чистые функции над копией
  `Spec`; `cloneSpec` уже отвязывает `Publishers`/`Consumers`/`Policy.ActorKinds`, и поле
  `Reserved` (bool) ничего к этому не добавляет.
- Проверку «нет фантомов» и полноту примеров я не принимал на веру: прогнал
  `go test -short -count=1 ./shared/contracts/` и отдельно сверил 31 файл индекса с 31 записью
  реестра и 31 ключом `blockVExamples` вручную. Совпадение точное.
- Списки `Consumers` сверял с колонкой «Потребители» §2.2 как перевод ролей в значения `source`;
  сам разработчик отмечает (dev-log §6), что «оператор», «страж», «replay» отображены на
  `mvctl`/`core/swarm` приблизительно. Единственное расхождение, которое я считаю ошибкой, а не
  переводом, — Mi-1 (`Publishers`, не `Consumers`). Мелочи вроде отсутствия `core/state` среди
  потребителей `combat.decided` (§2.2 пишет «State (через `entity.update.proposed`)», то есть
  напрямую State его не читает) я находкой не считаю.
- `llm.output.response_len` внесён в `required` сверх списка §2.3.10 — это не отсебятина:
  `data-model.md` §7.2 помечает `response_hash, response_len` как обязательные. Проверил отдельно,
  чтобы не выписать ложное замечание.
- Схемы блока «в» — первая редакция по документам, владельцы (EPIC-002/003/004/005) ревизуют их в
  своих задачах. Замечания Mi-2, Mi-5, N-3, N-4 намеренно сформулированы так, чтобы их можно было
  закрыть либо правкой сейчас, либо записанным решением с передачей владельцу — навязывать
  рефакторинг за пределами T-009 я не считаю правильным.
- Ничего не правил и не коммитил: изменена только эта запись в `review.md`. Временных файлов в
  рабочем дереве не оставил (`git status` после ревью совпадает с состоянием до него).

---

## T-008 · ревью #1 · 2026-09-09 · code-reviewer#2 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `0c92f2a`, изменения в индексе (коммита нет).
В границах — только файлы T-008 (F-6b):

- `docker-compose.yml` (профили, init-контейнеры, якоря);
- `Makefile` (переписан целиком);
- `build/{redpanda-init.sh, minio-init.sh, legacy.Dockerfile}`, дополнение `build/versions.env`
  (`LEGACY_GO_IMAGE`, `LEGACY_SRC_REF`, `ALPINE_IMAGE`);
- `scripts/{compose-lint.sh, llm-server.ps1, llm-server.sh}`;
- `.github/ci.env` (секция `legacy`), `.env.example` (секция `legacy`, `OLLAMA_ORIGINS`),
  `.gitignore` (одна строка);
- запись «developer#2 · T-008» в `dev-log.md`.

**Вне границ** (смотрит code-reviewer#1): `schemas/events/**` и `shared/contracts/**` (T-009).
Находки в этих файлах в отчёт не включались. `shared/env/infra.go` в границы не входит, но
упомянут в M-1 как место правки — сама находка в `.env.example`.

Основания: `tasks.md` §1 (общий DoD) и T-008 с дополнением сведения 3;
`architecture/infrastructure.md` v0.3 §1.1, §1.3, §1.4, §2.1, §2.2, §3.1, §3.1.1, §4.1, §4.2,
§4.4, §5.1–§5.3, §5.5, §5.6, §6.1–§6.3, §9; ADR-004 доп. 1–3/6/8, ADR-005 доп. 2 п. 1/3/4/7,
ADR-007 п. 4–6 и доп. 1–2, ADR-010 доп. 4, ADR-021; `threat-model.md` (T-14, T-17, T-23,
T-27…T-30, T-32; SEC-13, SEC-14, SEC-15, SEC-22, SEC-31, SEC-32, SEC-33, чек-лист §7);
`plan/ownership.md` v0.4 §1 (строки 9, 22, 27, 35); D-3, D-8, D-9, D-10, U-1, U-3, U-8;
`journal.md` 2026-09-10 (решения оркестратора по ОВ-40…ОВ-45).

### Вердикт

**Вернуть.** Critical: 0 · Major: 5 · Minor: 7 · Nit: 8.

Задача сделана основательно и с редкой для инфраструктуры аккуратностью: пять профилей и два
init-контейнера собраны по §1.3/§1.4 без единого исключения из правила «порт только на
`127.0.0.1`», admin-порт `core` не публикуется (T-14), пароли живут исключительно в `${VAR:?}`,
токен бота — только у `telegram-bot`, `NEO4J_PLUGINS` отсутствует, `OLLAMA_ORIGINS` не `*`,
сервиса `llama-server` в compose нет. `redpanda-init` идемпотентен по-настоящему (`alter-config`
безусловен), а сходимость `minio-init` с `objstore.EnsureBucket` не задекларирована, а
обеспечена: тот же единственный id правила `multiverse-retention-<bucket>`, та же семантика
«import/Set заменяет конфигурацию целиком» — сверено по `shared/objstore/minio.go:228-248` и
`buckets.go`, конфликта нет ни при каком порядке запусков. Решение по профилю `legacy` (сборка
из архивного коммита) — правильный выход из ситуации, где починить нельзя, а удалить запрещено
U-1.

Возврат — из-за пяти Major, четыре из которых воспроизводятся за минуту и чинятся несколькими
строками:

1. **M-1** — красный `go test -short ./shared/env/` из-за одной добавленной строки в
   `.env.example` (ломает общий DoD §1 п. 2 и будущий job `contracts`);
2. **M-2** — healthcheck `qdrant` не может работать: `CMD-SHELL` — это `/bin/sh`, а в образе это
   `dash`, который `/dev/tcp` не поддерживает; профиль `memory` (в `prod` включён по §1.1) не
   поднимется;
3. **M-3** — `make up` возвращает ≠ 0, когда LLM не поднят, вопреки прямому требованию §6.3
   («код возврата `make up` при этом нулевой», деградация FR-080/NFR-072); сегодня на машине
   владельца (нет `pwsh`, ОВ-43) это означает, что `make up` падает **всегда**;
4. **M-4** — правило 3 линтера (SEC-14) имеет два подтверждённых ложных пропуска: литеральный
   пароль в `environment` списком и литеральный пароль в любом якоре, кроме `x-platform-env`;
5. **M-5** — ни `Makefile`, ни `scripts/llm-server.*` не читают `.env`, хотя §6.2 говорит
   «значения берутся из `.env`»; `make llm-up` в чистой оболочке падает на
   `MV_LLM_BIN is not set`, а `make models`/`make warm` всегда уходят в ветку «не ollama».

Остальное — Minor/Nit; часть уместно закрыть в этой же итерации, часть — записанным решением.
Отклонения ОВ-40…ОВ-45 сверены с решениями оркестратора (`journal.md` 2026-09-10) и повторно как
замечания не выписываются.

### Что проверено прогоном (машина владельца; `make` и `pwsh` не установлены)

| Проверка | Команда | Результат |
|---|---|---|
| Профили compose | `docker compose --env-file build/versions.env --env-file .github/ci.env [--profile P] config -q` — ядро, `gpu`, `memory`, `dev`, `legacy`, `bot` | 6/6 ok |
| Публикации портов и теги | `config --format json` по всем профилям | 15 сервисов; все публикации с `127.0.0.1`; ни одного `latest`; у `core` публикаций нет |
| compose-lint | `bash scripts/compose-lint.sh` | ok, 15 сервисов, 6 правил, exit 0 |
| compose-lint, подложенное нарушение A | копия compose, `environment` у `minio` **списком** с литералами `MINIO_ROOT_USER=mvadmin`, `MINIO_ROOT_PASSWORD=SuperSecret123456` | **не пойман**, exit 0 → M-4 |
| compose-lint, подложенное нарушение B | копия compose, второй якорь `x-extra-env` с `NEO4J_PASSWORD: hunter2…`, `MV_MINIO_SECRET_KEY: plaintextsecret123`, влит в `core` через `<<: [*platform-env, *extra-env]` | **не пойман**, exit 0 → M-4 |
| compose-lint, подложенное нарушение C | копия compose, `image: qdrant/qdrant:v1.19.1` мимо `versions.env` | пойман, `[rule 1]`, exit 1 |
| healthcheck `qdrant` | `docker run --rm --entrypoint /bin/sh qdrant/qdrant:v1.19.1 -c 'ls -l /bin/sh; exec 3<>/dev/tcp/127.0.0.1/6333'` | `/bin/sh -> dash`; `cannot create /dev/tcp/…: Directory nonexistent`; `bash` в образе есть (`/usr/bin/bash`) → M-2 |
| healthcheck `neo4j` | `docker run --rm --entrypoint sh neo4j:5 -c 'command -v wget'` | `/usr/bin/wget` — ok |
| инструменты `redpanda-init` | `docker run … redpanda:v26.1.17 -c 'command -v bash seq date'` | все есть — ok |
| инструменты `minio-init` | `docker run … minio/mc:RELEASE.2025-08-13T08-35-41Z`: `bash`, `seq`, `date`, `mc ilm rule import --help`, `mc admin policy attach --help` | все есть — ok |
| сходимость ILM | чтение `shared/objstore/minio.go:228-248` (`lifecycleOf`) и `buckets.go` (`BucketOptionsFor`) против `import_ilm` в `build/minio-init.sh` | один и тот же id, одно правило, 30 дн., `Filter.Prefix=""` — совпадает |
| разбор Makefile | GNU make 4.4 в контейнере: `make -n help`, `make -n up`, `make -n reset` | разбирается, `include build/versions.env` работает |
| `health_code()` в `llm-server.sh` | `curl -s -o /dev/null -w '%{http_code}' --max-time 3 http://127.0.0.1:1/health \|\| echo 0` | возвращает `0000`, а не `000` → Mi-1 |
| манифест env | `go test -short ./shared/env/` | **FAIL**: `OLLAMA_ORIGINS:27: present in .env.example but not declared through env.Declare` → M-1 |
| секреты | `gitleaks git --staged --redact .` | 0 находок |
| хуки | `pre-commit run --files <11 файлов T-008>` | 8 хуков passed / 2 skipped (нет Go-файлов) |
| режимы и переводы строк | `git ls-files -s`, поиск CR | все `*.sh` — `100755`, LF; `Makefile`, `docker-compose.yml` — LF |
| имена топиков | `build/redpanda-init.sh` vs `shared/eventbus/topics.go` | 8/8 совпадают; retention 30/90/180 дн. и `segment.ms=86400000` совпадают с §5.1 и ADR-007 |
| владение | `plan/ownership.md` v0.4 §1 | все изменённые пути — EPIC-001/devops; `schemas/events/**` и `shared/contracts/**` (T-009) не тронуты |

Долгие операции (полный `up`, сборка legacy-образа, живые `redpanda-init`/`minio-init`) не
повторял — они выполнены и задокументированы разработчиком в `dev-log.md` §4; проверял
статически и лёгкими командами. Временных файлов не оставил: три копии
`docker-compose.lint-*.yml` удалены, `git status` после ревью совпадает с состоянием до него.

### Проверенные пункты DoD

| Пункт DoD T-008 | Статус |
|---|---|
| `make compose-lint` зелёный (5 проверок §3.1.1 + правило `llama-server`/`MV_LLM_URL`) | **частично**: все шесть правил реализованы и скрипт зелёный, но правило 3 имеет ложные пропуски (M-4), правило 1 слабее формулировки §3.1.1 (Mi-4) |
| `git grep -i minioadmin` пуст вне `Docs/` и `services/_archive/` | **закрыто решением оркестратора** (ОВ-42): область переформулирована, линтер проверяет `docker-compose.yml build scripts cmd shared .github .env.example` — прогон чистый |
| `docker compose … config -q` для каждого профиля | **выполнено** (6/6, проверено мной) |
| `make llm-up` / повторный no-op / `make llm-health` / `make llm-down` / `make up` не поднимает LLM | **не проверено на стенде** (нет `pwsh`, ОВ-43 — решение оркестратора: нужна установка пользователем). По чтению: `.ps1` логику соблюдает, `.sh` — нет (Mi-1); `make up` LLM действительно не поднимает, но возвращает ≠ 0 (M-3); в чистой оболочке `llm-up` не запустится вовсе (M-5) |
| стенд: `make up` → `make health` = ok, 8 топиков с retention, `make up PROFILES=legacy` | **выполнено разработчиком** (`dev-log.md` §4), профиль `legacy` — с ограничением ОВ-41 (принято оркестратором) |
| `make down` сохраняет тома | **выполнено**: `down` без `-v`; `-v` только в `reset` с подтверждением и проверкой бэкапа |
| Общий DoD §1 п. 1–3 (`lint`, `build/vet`, `go test -short`) | **не выполнен**: `go test -short ./shared/env/` красный из-за `.env.example` (M-1). Отметка «n/a, изменений в Go-коде нет» в `dev-log.md` §5 неверна — проверка тут данными, а не кодом |
| Общий DoD §1 п. 4 (`secrets-scan`) | **выполнено**: 0 находок |
| Общий DoD §1 п. 5 (`dev-log.md`) | **выполнено**, запись подробная и честная (в том числе про невыполненный критерий) |
| Общий DoD §1 п. 7 (границы владения) | **выполнено** |
| Общий DoD §1 п. 8 (CI) | n/a до T-012 |

### Замечания

#### Major

**M-1. `.env.example`: `OLLAMA_ORIGINS` добавлен в пример, но не объявлен в манифесте `shared/env` — тест красный.**
`.env.example:27`; манифест — `shared/env/infra.go:42-56`.

```
--- FAIL: TestRepositoryExampleMatchesTheManifest (0.00s)
    example_test.go:133: OLLAMA_ORIGINS:27: present in .env.example but not declared through env.Declare
```

Проверка `CheckExample` разбирает и закомментированные строки (иначе условно обязательный блок
`OLLAMA_*` не проверялся бы), поэтому `#OLLAMA_ORIGINS=…` — полноценная запись примера.
Последствия шире одного теста: этой же проверкой живут `make contracts`, `mvctl env check`
(T-010) и job `contracts` (T-012) — то есть T-008 в текущем виде оставляет волне 0 красный гейт.
Общий DoD §1 п. 2 не выполнен.

Как исправить — три строки рядом с остальными `OLLAMA_*`:

```go
OllamaOrigins = DeclareExternal("OLLAMA_ORIGINS", "",
    "allowed CORS origins of Ollama; never `*` (SEC-15)",
    Tooling(), RequiredWhen("MV_LLM_PROVIDER", "ollama"))
```

Переменная действительно нужна: `docker-compose.yml:363` читает `${OLLAMA_ORIGINS:-…}`, то есть
это вход compose, а не украшение примера. Альтернатива — убрать строку из `.env.example` — хуже:
compose-переменная останется недокументированной. `shared/env/infra.go` — тот же EPIC-001, но
файл вне перечня T-008: правку согласовать с автором T-007 / tech-lead#1 и прогнать
`go test -short ./shared/env/` до сдачи.

**M-2. Healthcheck `qdrant` не может стать зелёным: `/dev/tcp` под `CMD-SHELL` — это `dash`.**
`docker-compose.yml:284`.

```yaml
test: ["CMD-SHELL", "exec 3<>/dev/tcp/127.0.0.1/6333"]
```

`CMD-SHELL` — это `/bin/sh -c`, а в образе `qdrant/qdrant:v1.19.1` `/bin/sh` — символическая
ссылка на `dash`, у которого конструкции `/dev/tcp` нет. Проверено на **закреплённом** теге:

```
lrwxrwxrwx 1 root root 4 Feb  4  2025 /bin/sh -> dash
/usr/bin/bash
/bin/sh: 1: cannot create /dev/tcp/127.0.0.1/6333: Directory nonexistent
```

Последствие: `qdrant` навсегда `unhealthy` → `memory` объявлен с
`depends_on: qdrant: condition: service_healthy` и не стартует → `make up PROFILES=memory`
(а это, по §1.1, штатный набор профилей в `prod`) падает на `--wait`. Не поймано потому, что
профиль `memory` проверялся только `config -q`.

`infrastructure.md` §5.3 предписывает ровно то, что нужно, и даже помечает «проверить при
реализации»: `bash -c 'exec 3<>/dev/tcp/127.0.0.1/6333'`. Исправление:

```yaml
test: ["CMD", "bash", "-c", "exec 3<>/dev/tcp/127.0.0.1/6333"]
```

`bash` в образе есть (`/usr/bin/bash`). После правки — поднять профиль `memory` на стенде и
убедиться, что `qdrant` доходит до `healthy`, а `memory` стартует.

**M-3. `make up` возвращает ненулевой код, когда `llama-server` не поднят.**
`Makefile:144-146` (`up`), `Makefile:183-214` (`health`), `Makefile:207`
(`llm-health || rc=1`), `Makefile:214` (`exit $$rc`).

`up` заканчивается `$(MAKE) --no-print-directory health`; `health` завершает себя `exit $$rc`, а
`rc=1` ставится в том числе при неуспешном `llm-health`. `infrastructure.md` §6.3 говорит прямо
противоположное:

> `make up` **только проверяет** `GET $MV_LLM_URL/health` и при не-`200` печатает
> `warning: LLM недоступен, нарратив деградирует (FR-080); подними процесс: make llm-up` — код
> возврата `make up` при этом нулевой.

То же в §2.2. Это не буквоедство: отсутствие LLM — задокументированная деградация
(FR-080/NFR-072), а не отказ стека, и `make up` в цепочке (`deploy` — `Makefile:326`,
`rollback` — `Makefile:335`) начинает падать после успешного развёртывания. Сегодня на машине
владельца эффект абсолютный: `pwsh` нет (ОВ-43), `$(call llm,health)` уходит в ветку
`ON_WINDOWS` → `exit 1` → `make up` падает **всегда**, даже когда весь стек здоров.

Как исправить (любой из вариантов, первый — минимальный):

а) в `up` не звать `health` целиком, а собрать: контейнерные пробы строго +
   `$(MAKE) llm-health || echo "warning: LLM недоступен, нарратив деградирует (FR-080);
   подними процесс: make llm-up"`;
б) ввести в `health` переменную строгости — `LLM_STRICT ?= 1`, и звать из `up`
   `$(MAKE) health LLM_STRICT=0`; `make health` сам по себе остаётся строгим по §2.2
   («код возврата ≠ 0, если что-то не ok»).

**M-4. `compose-lint` правило 3 (SEC-14) пропускает литеральные пароли в двух распространённых раскладках.**
`scripts/compose-lint.sh:229-265` (разбор `raw_env` и якоря).

Правило 3 читает исходный YAML построчными регулярками: `^  <service>:$` → `^    environment:$`
→ `^      KEY: value`, плюс отдельно якорь **с жёстко зашитым именем** `x-platform-env`. Обе
посылки ломаются на легальном YAML. Проверено на временных копиях (удалены):

- **список вместо отображения.** `environment:` у `minio` переписан в
  `- MINIO_ROOT_USER=mvadmin` / `- MINIO_ROOT_PASSWORD=SuperSecret123456` — линтер:
  `compose-lint: ok — 15 services, 6 rules`, exit 0. Раскладка не экзотическая: именно так
  написан as-is `docker-compose.yml`, из которого перенесены секции `legacy`;
- **другой якорь.** Добавлен `x-extra-env: &extra-env` с `NEO4J_PASSWORD: hunter2hunter2hunter2`
  и `MV_MINIO_SECRET_KEY: plaintextsecret123`, влит в `core` через
  `<<: [*platform-env, *extra-env]` — линтер снова `ok`, exit 0.

Это единственная автоматическая защита SEC-14/T-29 в CI, и она пропускает ровно тот класс
дефекта, ради которого написана. Правило 1 (подложенный `image: qdrant/qdrant:v1.19.1`) при этом
сработало, так что проблема локальна.

Как исправить, не переписывая линтер: перестать привязываться к раскладке и сканировать текст
файла целиком одним проходом по обеим формам записи, а разбиение по сервисам оставить только для
сообщений:

```python
for m in re.finditer(r"^\s*-?\s*([A-Za-z_][A-Za-z0-9_]*)\s*[:=]\s*(\S.*?)\s*$", raw, re.M):
    key, value = m.group(1), m.group(2)
    if not SECRET_KEY.match(key):
        continue
    # дальше — та же проверка ${VAR} / ${VAR:?}
```

Список-форму `- KEY=value` покрывает та же регулярка. Заодно стоит добавить фикстуры
(`testdata/compose-lint/bad-*.yml`) и цель, которая гоняет по ним скрипт: тогда ложный пропуск
ловится прогоном, а не ревью.

**M-5. Ни `Makefile`, ни `scripts/llm-server.*` не читают `.env` — цели `llm-*`, `models`, `warm` не работают как описано.**
`Makefile:56-66` (макрос `llm`), `Makefile:227-247` (`models`, `warm`);
`scripts/llm-server.sh:139`; `scripts/llm-server.ps1:167`.

`infrastructure.md` §6.2: «Значения берутся из `.env` (`MV_LLM_*`, §4.2), в скрипте не зашиты»,
и §4.2 действительно держит `MV_LLM_BIN`, `MV_LLM_MODEL_FILE`, `MV_LLM_*` в `.env`. Но `.env`
никто не загружает: compose читает его сам, а `make` — нет, и в скрипты переменные не попадают.
Практически:

- `make llm-up` в чистой оболочке падает на `MV_LLM_BIN is not set (see .env.example, §4.2)` —
  то есть ровно на том, что в `.env` записано;
- `make models` и `make warm` читают `$${MV_LLM_PROVIDER:-openai_compat}` из среды, а не из
  `.env`, поэтому всегда уходят в ветку «не ollama» и печатают подсказку — даже если оператор
  выбрал `ollama`.

Образец правильного поведения есть в самом дизайне — §4.3 для `make run-local`:
`set -a; source .env; set +a`. Достаточно того же в макросе `llm` и в `models`/`warm`:

```make
define llm
set -a; [ -f .env ] && . ./.env; set +a
if command -v pwsh >/dev/null 2>&1; then
...
```

(или загружать `.env` в самих скриптах — тогда поведение одинаково и при прямом запуске
`bash scripts/llm-server.sh`). Если решение сознательное — «`MV_LLM_*` оператор держит в
переменных среды Windows, а не в `.env`» — оно противоречит §4.2/§6.2 и должно быть записано как
отклонение с правкой `.env.example`.

#### Minor

**Mi-1. `llm-server.sh`: `health_code()` возвращает `0000`, из-за чего ветка «сервер не отвечает» мертва, а `up` при живом PID и молчащем порте не перезапускает сервер.**
`scripts/llm-server.sh:56-58`, `:98`, `:130`.

```bash
curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$base_url/health" 2>/dev/null || echo 0
```

`curl` при отказе соединения **и** печатает `000` через `-w`, **и** возвращает 7 — поэтому
срабатывает ещё и `|| echo 0`, а на выходе получается `0000`. Воспроизведено:

```
$ code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 http://127.0.0.1:1/health || echo 0)
health_code returns: [0000]
FELL THROUGH to default branch
```

Следствия:

- `health` печатает `llm: …/health = 0000` вместо подготовленного и куда более полезного
  «unreachable — the process is not running (make llm-up)» (`:98`), то есть самый частый случай
  диагностируется хуже всего. Код возврата при этом верный (`exit 1`);
- `up` (`:130`): условие `[ "$(health_code)" != "000" ]` теперь истинно всегда, поэтому при
  «PID записан и жив, а порт молчит» скрипт печатает «already running … nothing to do» и выходит
  с 0 — вместо перезапуска, который описан в `dev-log.md` §1 и реализован в `.ps1` (там
  `Invoke-Health` в `catch` честно возвращает `0`).

Исправление — одна функция:

```bash
health_code() {
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$base_url/health" 2>/dev/null) || true
  printf '%s' "${code:-000}"
}
```

**Mi-2. `minio-init.sh`: любая ошибка `mc admin user add` трактуется как «пользователь уже есть».**
`build/minio-init.sh:73-85`.

```bash
if mc admin user add "$ALIAS" "$MV_MINIO_ACCESS_KEY" "$MV_MINIO_SECRET_KEY" >/dev/null 2>&1; then
  log "service user added"
else
  log "service user exists"
fi
```

Ветка `else` — не только «уже есть»: MinIO отвергает access key короче 3 и secret короче 8
символов, отвечает ошибкой при проблемах admin API и т. п. В этих случаях контейнер напишет
«service user exists», затем «policy readwrite already attached» (та же схема на `:80-84`) и
завершится с 0 — а платформа стартует с учётными данными, которых не существует, и упрётся в
403 при первом обращении к MinIO. Для init-контейнера, от которого `gateway`/`core` зависят
через `service_completed_successfully`, это ровно та ошибка, которую он обязан ловить.

Как исправить: в ветке отказа подтвердить факт, а не предположить его —

```bash
else
  mc admin user info "$ALIAS" "$MV_MINIO_ACCESS_KEY" >/dev/null 2>&1 || {
    log "cannot create service user ${MV_MINIO_ACCESS_KEY} (access key >= 3, secret >= 8 chars?)"
    exit 1
  }
  log "service user exists"
fi
```

и аналогично для `policy attach`.

**Mi-3. Секреты MinIO передаются аргументами командной строки.**
`build/minio-init.sh:47` (`mc alias set … "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"`), `:73`
(`mc admin user add … "$MV_MINIO_SECRET_KEY"`).

Аргументы видны в `/proc/<pid>/cmdline` внутри контейнера и в `docker top`; это тот самый пункт,
который проверяется в задаче отдельно. Риск невелик (контейнер одноразовый и однопроцессный,
переменные всё равно видны в `docker inspect`), поэтому Minor, а не Major, но половина проблемы
убирается бесплатно: `mc` берёт алиас из переменной окружения, и тогда пароль в argv не попадает
вовсе —

```bash
export MC_HOST_local="http://${MINIO_ROOT_USER}:${MINIO_ROOT_PASSWORD}@minio:9000"
```

(URL-энкодинг пароля обязателен). Для `admin user add` замены нет — оставить как есть, отметив
это комментарием, чтобы следующий читатель не считал упущением.

**Mi-4. Правило 1 линтера слабее, чем §3.1.1: проверяется имя переменной, а не значение.**
`scripts/compose-lint.sh:165-176`.

§3.1.1 п. 1: «каждый `image:` имеет явный тег ≠ `latest` **и совпадает со значением из
`build/versions.env`**». Реализация проверяет только, что в исходной строке есть подстановка
какой-нибудь переменной, объявленной в `versions.env` (`if not used & set(versions)`). Значение
при этом может прийти откуда угодно — из `.env`, из `--env-file`, из среды: `REDPANDA_IMAGE`,
переопределённый на другой тег, правило 1 не заметит (сработает только общий запрет `latest`).

Отклонение объяснимо (`CHROMA_IMAGE` в `versions.env` намеренно пуст, а в `ci.env` —
плейсхолдер), но в `dev-log.md` §3 не записано. Исправление: сравнивать значение, когда
переменная в `versions.env` непуста и строка — ровно одна подстановка:

```python
if len(used) == 1 and versions.get(next(iter(used))):
    expected = versions[next(iter(used))]
    if image != expected:
        fail(1, f"{name}: image {image!r} != {expected!r} from build/versions.env")
```

— пустой `CHROMA_IMAGE` при этом естественно выпадает из сравнения. Либо, если сравнение
отброшено сознательно, записать это отклонением от §3.1.1.

**Mi-5. `make up` подтягивает `legacy-src` только при `PROFILES=`, но не при `COMPOSE_PROFILES` из `.env`.**
`Makefile:144`.

```make
up: $(if $(findstring legacy,$(PROFILES)),legacy-src)
```

По §1.3 штатный способ выбрать профили — `COMPOSE_PROFILES` в `.env` («`make up PROFILES=...`
переопределяет»), и в этом случае предпосылка не сработает: `build/.legacy-src` не существует
(каталог в `.gitignore`), а `semantic-memory`/`narrative-orchestrator` объявлены с
`context: build/.legacy-src` — `docker compose up` упадёт на несуществующем контексте сборки с
сообщением, по которому причину не угадать.

Как исправить: считать профили из обоих источников (например,
`ALL_PROFILES := $(PROFILES)$(COMPOSE_PROFILES)` вместе с загрузкой `.env` из M-5) и строить
предпосылку по `findstring legacy,$(ALL_PROFILES)`; либо сделать `legacy-src` дешёвым и
идемпотентным и вызывать его, когда `docker compose config --profiles` содержит `legacy`.

**Mi-6. `build/legacy.Dockerfile`: плавающий тег базового образа рантайма и загрузка ONNX Runtime без контрольной суммы.**
`build/legacy.Dockerfile:103` (`FROM debian:bookworm-slim`), `:53-56` (`wget -O /tmp/onnxruntime.tgz …`).

Стадия сборки честно закреплена через `ARG LEGACY_GO_IMAGE` из `versions.env`, а стадия рантайма
— нет: `debian:bookworm-slim` перекатывается при каждом обновлении ветки. Это противоречит
правилу из шапки самого `versions.env` («no `latest` and no floating tags anywhere») и NFR-071.
Тарбол ONNX Runtime тянется по HTTPS с GitHub без `sha256sum -c` — единственная внешняя загрузка
во всей сборке.

Часть кода унаследована из as-is корневого `Dockerfile`, и модернизировать замороженный сервис
задача не ставит, но эти две строки — про сборку, а не про поведение сервиса: добавить
`LEGACY_RUNTIME_IMAGE=debian:bookworm-slim@sha256:…` (или хотя бы патч-тег) в
`build/versions.env` и `echo "<sha256>  /tmp/onnxruntime.tgz" | sha256sum -c -` после `wget`.

**Mi-7. `make reset` проверяет наличие бэкапа, но не его свежесть.**
`Makefile:167-176`.

§2.2: «`make reset` — `down -v` с подтверждением **и проверкой свежего бэкапа**». Реализация
берёт `ls -t … | head -n 1`, печатает имя и на этом останавливается: годовалый архив пройдёт
проверку так же, как вчерашний. Подтверждение вводом `yes` на месте, поэтому Minor, но
«свежесть» из требования выпала.

Исправление: рядом уже есть готовый расчёт возраста в `health` (`Makefile:208-212`) —

```bash
age=$(( ( $(date +%s) - $(date -r "$latest" +%s) ) / 86400 ))
[ "$age" -le 7 ] || { echo "reset: newest backup is $age days old; run 'make backup'" >&2; exit 1; }
```

#### Nit

**N-1. `.github/ci.env` нарушает собственное правило про пины.** `.github/ci.env:18-19` объявляет
«image pins are not repeated», а строка `CHROMA_IMAGE=chromadb/chroma:0.0.0-ci-placeholder` его
нарушает. Причина объяснена в комментарии двумя строками выше и она уважительная (D-3), но
правило и исключение стоят рядом. Аккуратнее — завести в `versions.env` `CHROMA_IMAGE_CI` (или
отметить исключение в §3.2), чтобы файл оставался «без пинов».

**N-2. `ORACLE_MODEL: ${LLM_MODEL_DEFAULT}` без модификатора.** `docker-compose.yml:534`. Весь
файл выдержан в стиле `${VAR:-default}` / `${VAR:?message}`; здесь — «голая» подстановка, и при
запуске без `build/versions.env` в `COMPOSE_ENV_FILES` значение молча станет пустым. Уместно
`${LLM_MODEL_DEFAULT:?set in build/versions.env}`.

**N-3. `legacy-src`: переносы строк держатся на `.ONESHELL`.** `Makefile:157-159` — вместо
`\`-продолжений в строке `git archive` остались табуляции, а склейка работает благодаря
`.ONESHELL` и висящему `|`. Сейчас корректно, но при снятии `.ONESHELL` рецепт сломается тихо.

**N-4. Фильтр `minioadmin` может съесть настоящую находку.** `scripts/compose-lint.sh:96-97`:
`grep -v -i -E '(never|not|не) minioadmin'` отбросит любую строку, где рядом со словом окажется
`not`, — включая строку с настоящим паролем. Точнее — сопоставлять с явным списком разрешённых
файлов/строк (`.env.example`, `shared/env/infra.go`) или требовать, чтобы строка была
комментарием.

**N-5. `MV_LLM_HOST` не проверяется на loopback.** `scripts/llm-server.sh:52`,
`scripts/llm-server.ps1:57` — комментарий «loopback ONLY (SEC-15)» стоит, а проверки нет:
`MV_LLM_HOST=0.0.0.0` в `.env` выставит сервер наружу. §6.3 прямо предупреждает, что `0.0.0.0`
допустим только вместе с правилом файрвола. Строка вида
`case "$llm_host" in 127.*|localhost|::1) ;; *) echo "llm: WARNING …" >&2 ;; esac` сделала бы
комментарий правдой.

**N-6. `make health` по-разному относится к незапущенным сервисам.** `Makefile:186-205`: для
`gateway`/`memory`/`telegram-bot` отсутствие сервиса в `docker compose ps --services` печатается
как `-` и не влияет на код возврата, а `core` при остановленном стеке всегда `FAIL` (`exec`
падает). На выключенном стеке `make health` возвращает 1 «из-за `core`», хотя причина другая.

**N-7. `redpanda-init` берёт адрес брокера из `MV_KAFKA_BROKERS`.** `docker-compose.yml:133`.
Контейнер всегда внутри compose-сети, а `MV_KAFKA_BROKERS` — переменная, для которой §4.2/§4.3
документируют хостовое значение `127.0.0.1:19092` (для `go run` и `mvctl`). Оператор, поправивший
`.env` под `go run`, сломает init-контейнер. Надёжнее прошить `RPK_BROKERS: redpanda:9092`.

**N-8. Рантайм-образ `legacy` работает от `root` и ставит `curl`, которым никто не пользуется.**
`build/legacy.Dockerfile:105-115` — `USER` не задан, healthcheck у legacy-сервисов нет
(`depends_on: service_started`). Профиль временный и наследует as-is, поэтому только Nit.

### Отдельно проверено и признано верным

- **Сходимость `minio-init` ↔ `objstore.EnsureBucket`.** Подозревал конфликт двух правил ILM на
  одном бакете. Не подтвердилось: `lifecycleOf` (`shared/objstore/minio.go:228-248`) формирует
  ровно одно правило `multiverse-retention-<bucket>` с `Filter.Prefix=""` и вызывает
  `SetBucketLifecycle` (замена целиком), а `import_ilm` пишет тот же id, ту же форму и тот же
  срок через `mc ilm rule import` (тоже замена целиком). Порядок запусков значения не имеет.
  Дублирование константы `30` (`RETENTION_DAYS` в скрипте против `retentionDays` в
  `buckets.go`) разработчик честно назвал единственной точкой расхождения; integration-тест
  T-007 её стережёт — отдельным замечанием не выписываю.
- **`redpanda-init`: `alter-config` выполняется безусловно.** Выглядит как лишняя работа при
  каждом `make up`, но это именно то, что требует §5.1 (изменённый retention должен доехать до
  существующего кластера). Побочный эффект даже полезен: если `create` упал не из-за «уже есть»,
  а по другой причине, `alter-config` на несуществующем топике завершится ошибкой и уронит
  контейнер — глушения ошибки здесь нет.
- **`OLLAMA_HOST` намеренно не задан.** Сначала счёл пропуском правила 5. Разработчик прав:
  образ и так слушает интерфейс контейнера, порт опубликован на `127.0.0.1`, а запись
  `OLLAMA_HOST=0.0.0.0` была бы ровно тем, что запрещает SEC-15. Правило 5 линтера при этом
  проверяет переменную, если она появится, — и текстом по файлу тоже.
- **Тома и `make down`.** `down` без `-v`; `-v` только в `reset` (подтверждение + бэкап);
  целевые тома переименованы (`redpanda-data`, `minio-data`) так, что as-is данные не
  примонтируются молча, а `chroma_data` сохранил as-is имя намеренно (§5.5, D-3). Всё сходится.
- **Профиль `legacy` не тянет уязвимую конфигурацию в дефолтный запуск.** `chromadb`,
  `semantic-memory`, `narrative-orchestrator` объявлены с `profiles: ["legacy"]`, публикаций у
  двух из трёх нет, у третьего — `127.0.0.1:8083`; `neo4j` разделён между `memory` и `legacy`
  осознанно. В дефолтном `up` (без профилей) поднимаются ровно шесть сервисов §1.3.
- **`MV_MINIO_ACCESS_KEY`/`MV_MINIO_SECRET_KEY` и root-ключ.** Платформа получает сервисные
  ключи, root-учётка используется только внутри `minio-init` (§5.2). Ветка «ключ совпал с root»
  честно пропускает создание пользователя, а не создаёт его втихую с root-именем.

### Предложения в бэклог

1. **Фикстуры для `compose-lint`.** `testdata/compose-lint/bad-*.yml` (по одному файлу на
   нарушение) + цель, которая гоняет по ним скрипт и требует ненулевого кода. Тогда ложные
   пропуски вроде M-4 ловятся прогоном, а не ревью. Кандидат — T-012 (F-7), вместе с job
   `compose-lint`.
2. **Разбор compose YAML-парсером.** `PyYAML` в CI (установка — секунды) убирает весь класс
   зависимостей от раскладки файла; сам разработчик назвал это в рисках. Область — тот же T-012.
3. **Таблица топиков в одном месте.** `build/redpanda-init.sh` дублирует
   `shared/eventbus/topics.go` и политики реестра; §2.2 уже предусматривает
   `mvctl contracts topics --format=rpk` (T-010). Либо генерировать список init-скрипту из
   `mvctl`, либо добавить тест «имена и retention совпадают».
4. **Права сервисного пользователя MinIO.** Встроенная политика `readwrite` — это `s3:*` на все
   бакеты, включая чужие; минимальными были бы правила на `entities-*`, `snapshots-*`,
   `prompts-*`, `ops-artifacts`. Сейчас `readwrite` предписан §5.2, поэтому это не находка, а
   вопрос к архитектуре — уместно поднять вместе с EPIC-005.
5. **`make backup` без окна простоя или с `trap`.** Сейчас `core` и `redpanda` останавливаются,
   и при ошибке `tar` (`set -e`) остаются остановленными.
   `trap '$(COMPOSE) start redpanda core' EXIT` закрывает это одной строкой; вариант
   `mc mirror` из §5.6 не автоматизирован.
6. **Оверлей `build/compose.dev.yml`** для консолей MinIO/Neo4j — вариант (б) из ОВ-40, если
   оператору понадобится UI; сейчас решением оркестратора не заводится.
7. **Стендовая проверка ОВ-41** (`semantic-memory` с CGO + ONNX, тег `CHROMA_IMAGE`, живая
   связка orchestrator → semantic-memory → chroma) — до решения по S5, с уведомлением
   tech-lead#2.

### Риски и допущения

- `make` и PowerShell 7 на машине отсутствуют, поэтому `Makefile` проверялся разбором GNU make
  4.4 в контейнере (`make -n` по целям) и чтением, а `scripts/llm-server.ps1` — только чтением
  (ОВ-43). Замечания M-3 и M-5 выведены из текста рецептов, а не из прогона; M-5 дополнительно
  подтверждается тем, что `.env` не упоминается ни в одном рецепте, кроме `deploy`
  (`Makefile:322`, и там — `grep`, не `source`).
- M-2 проверен на закреплённом теге `qdrant/qdrant:v1.19.1` (образ подтянут специально), но не
  на живом контейнере с healthcheck: вывод о том, что `memory` не стартует, сделан из
  `depends_on: condition: service_healthy`. Проверка на стенде — одна команда
  `make up PROFILES=memory`.
- Профиль `legacy` целиком (сборка `semantic-memory`, старт связки) не проверялся — по указанию
  не повторять долгие операции; принято по `dev-log.md` §2 и решению оркестратора (ОВ-41).
- Ложные пропуски M-4 демонстрировались на временных копиях `docker-compose.yml`; копии удалены,
  рабочее дерево не изменено. Кроме этой записи в `review.md` не правил ничего и не коммитил.
- Отклонения ОВ-40…ОВ-45 приняты оркестратором (`journal.md` 2026-09-10) и повторно как
  замечания не выписаны: непубликация консолей MinIO/Neo4j, сборка `legacy` из архивного
  коммита, переформулировка критерия `minioadmin`, отсутствие `pwsh`, заглушки `telegram-bot` и
  `make bench`.
