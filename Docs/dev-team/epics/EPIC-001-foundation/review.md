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
