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

---

## T-010 · ревью #1 · 2026-09-10 · code-reviewer#1 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `eda1b3b`, изменения в индексе (коммита нет).
В границах — только файлы T-010 (F-4c):

- `cmd/mvctl/main.go`, `cmd/mvctl/main_test.go`;
- `cmd/mvctl/internal/cli/{cli.go, cli_test.go}`;
- `cmd/mvctl/internal/contracts/{contracts.go, check.go, topics.go, check_test.go, topics_test.go}`;
- `cmd/mvctl/internal/env/{env.go, env_test.go}`;
- `cmd/mvctl/internal/storage/{storage.go, storage_test.go, integration_test.go}`;
- точечные правки `shared/contracts/{spec.go, sources.go}` (`TopicSpec.MaxMessageBytes`,
  константы топиков, `KnownSources()`);
- `shared/env/vars.go` и `.env.example` (`MV_CORE_ADMIN_CLIENTS=operator,mvctl`);
- `Makefile` (одна строка в цели `contracts`);
- запись «developer#1 · T-010» в `dev-log.md`.

**Вне границ** (смотрит второй ревьюер): `shared/entity/**` и `.golangci.yml` (T-011). Находки
в этих файлах в отчёт не включались; `.golangci.yml` прочитан только чтобы убедиться, что
`cmd/mvctl/**` не исключён из линтера (не исключён — исключение только на импорт
`internal/replay`).

Основания: `tasks.md` §1 (общий DoD) и T-010; `epics/EPIC-001-foundation/design.md` §4.1
(реестр подкоманд, проверки (а)–(д)), §10; `architecture/components/foundation.md` v0.2 §6,
§10, §12; `architecture/contracts.md` v0.4 §0, C-01, §16 п. 4 и 7; `architecture/infrastructure.md`
v0.3 §4.2, §5.1, §5.2; `plan/ownership.md` v0.4 §1 (строки 20, 35); ADR-004 доп. п. 2, ADR-007
п. 4–6, ADR-009 п. 9, ADR-021 п. 1–2; D-6, D-9, U-3; `journal.md` 2026-09-10 (ОВ-27, ОВ-28,
ОВ-37 и решения по ОВ-46…ОВ-49).

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 8 · Nit: 5.

Задача сделана по описанию и по DoD; замечания — качество вывода и полнота двух проверок, ни
одно не блокирует приёмку и ни одно не требует переделки принятых решений. Отдельно отмечу
решение 1 из `dev-log.md` §2: переписать проверку (г) со сверки политики самой с собой на сверку
с политикой топика — это и есть тот случай, ради которого проверка существует, и первая редакция
его пропускала.

### Что прогнано

| Проверка | Результат |
|---|---|
| `go build ./...` (корень, весь модуль) | зелёный |
| `go vet ./...` | зелёный |
| `go test -short -count=1 ./...` | все пакеты `ok` (`-race` недоступен, общий DoD §1 п. 2 в этой части не проверен) |
| `go test -cover ./cmd/mvctl/...` | 75,0 % / cli 94,1 % / contracts 94,1 % / env 87,9 % / storage 93,2 % — цифры `dev-log.md` §4 воспроизвелись |
| `go test -tags integration ./cmd/mvctl/internal/storage/...` | `ok` (живой MinIO из `MINIO_IMAGE`, Docker) |
| `golangci-lint run ./...` | `0 issues` |
| `gofmt -l cmd/mvctl shared/contracts shared/env` | пусто |
| `gitleaks git --staged --redact .` | `no leaks found` |
| `go run ./cmd/mvctl contracts check` | `65 types, 8 topics, 58 schema files checked`, код 0 |
| `go run ./cmd/mvctl contracts topics` / `--format=rpk` | 8 строк таблицы / 16 команд, код 0, stderr пуст |
| `go run ./cmd/mvctl env check` | `69 variables declared`, код 0 |
| `mvctl storage init --store=memory` / `--store=s3` / `--store=minio` без ключей | 0 / 2 / 1 — коды разделены как заявлено |
| `mvctl world init` (зарезервированное имя) | код 2, сообщение с владельцем EPIC-002 |
| строки цели `contracts` построчно | все три зелёные (`make` на машине нет — ОВ-42) |

Синтетические случаи прогонял сам, временными `_test.go` в `cmd/mvctl/internal/contracts`;
файлы удалены, рабочее дерево не изменено (`git status` по `cmd/mvctl` — только файлы задачи).

### Minor

**Mi-1. `check.go:63-76` — фантомный файл схемы прошлой версии проверке невидим.**
`checkRegistry` кладёт файлы в `map[тип]schemaOwner`, то есть на один тип помещается один файл.
`fs.Glob` возвращает имена в алфавитном порядке, поэтому при паре `x.v1.json` + `x.v2.json`
в карте остаётся `v2`, спек зарегистрирован на `v2` — находки нет, а `x.v1.json` остаётся в
`schemas/events/` навсегда. Воспроизвёл на синтетическом входе: `SchemaFiles` в порядке глоба
(`llm.output.v1.json`, `llm.output.v2.json`) при `SchemaVersion: 2` → `findings == []`. В обратном
порядке файлов находка появляется, но с неверной причиной («the file on disk is llm.output.v1.json»,
хотя `v2` лежит рядом и корректен).
Это ровно та половина правила (б), которую `design.md` §4.1 называет «и наоборот»: `contracts.New`
проверяет только наличие нужного файла, лишние ему безразличны, так что второго заслона нет.
Сегодня влияния нет — все схемы `v1`, ложного «зелёного» не возникает; **дыра открывается на
первом же бампе версии схемы**, поэтому Minor, а не Major.
Как исправить: ключевать карту именем файла, а не типом (`files[name]`), и в цикле по спекам
удалять из неё ровно `sharedcontracts.SchemaFile(spec.Type, spec.SchemaVersion)`; остаток карты
и так уже печатается как «schema file of a type nobody registered» — сообщение для лишней версии
стоит дать отдельным текстом («более не зарегистрированная версия схемы типа X»). Тест —
четвёртый подслучай в `TestPhantomTypeAndPhantomSchema`.

**Mi-2. `topics_test.go:66-105` — сверка с `build/redpanda-init.sh` неполна.**
`TestRPKMatchesTheInitScript` сверяет только имена топиков и `retention.ms`. `segment.ms`,
`max.message.bytes`, `-p/-r` и `cleanup.policy` в скрипте — отдельные литералы
(`SEGMENT_MS=86400000`, `LLM_MAX_MESSAGE_BYTES=4194304`, `-p 1 -r 1`,
`-c cleanup.policy=delete`), и расхождение по любому из них тест не увидит. Решение ОВ-37
(«единый источник для redpanda-init и `mvctl contracts topics`») этим держится наполовину:
`MaxMessageBytes` внесено в `TopicSpec` именно чтобы не было двух чисел, а число в скрипте
ничем не привязано к `llmRecordsMaxMessageBytes`.
Как исправить: в том же тесте вытащить из скрипта `SEGMENT_MS=` и `LLM_MAX_MESSAGE_BYTES=`
(обычные строки «ИМЯ=значение», регулярка на две строки) и сверить с `TopicSegmentMS` и с
`MaxMessageBytes` строки `llm_records`; заодно проверить, что в скрипте есть `-p 1 -r 1`.

**Mi-3. `topics.go:80-84` — вывод `--format=rpk` не является рабочим скриптом, хотя комментарий
обещает обратное.** В комментарии к `RPK` сказано, что пара create+alter «is what makes the init
script idempotent», а `dev-log.md` §1 — что это «ровно те команды, что выполняет
`build/redpanda-init.sh`». Команды текстуально те же, но идемпотентность в скрипте даёт не сама
пара, а обёртка `if rpk topic create …; then … else log "exists"; fi`: на существующем топике
`rpk topic create` завершается ненулевым кодом. Сгенерированный файл такой обёртки не несёт,
поэтому `mvctl contracts topics --format=rpk | sh -e` (и `bash -e`, и `set -euo pipefail` в
любом враппере) обрывается на втором запуске первой же командой. Плюс в выводе нет адреса
брокера — в контейнере `redpanda-init` он `redpanda:9092`, а `rpk` без `-X brokers=` идёт в
`localhost:9092`; работает только если вызывающий выставил `RPK_BROKERS`.
Как исправить (любое из двух, правки на одну–две строки): либо печатать `rpk topic create … || true`
и упомянуть `RPK_BROKERS` в `-h` команды, либо снять обещание — переформулировать комментарий и
описание флага как «справочный список целевой конфигурации, а не готовый скрипт». Второе честнее,
если генерация `redpanda-init.sh` уходит в бэклог (ОВ-49).

**Mi-4. `storage.go:137-142` — примечания о деградации печатаются, когда пропускать нечего.**
Заметки «the store does not support versioning; the rule was skipped» и «…lifecycle…expiry was
skipped» зависят только от `Capabilities()`, а единственный бакет `PlatformBuckets` —
`ops-artifacts` — по `BucketOptionsFor` не просит ни versioning, ни ILM (`infrastructure.md` §5.2:
«`ops-artifacts` — без правил»). В результате `mvctl storage init --store=memory` печатает подряд
`bucket ops-artifacts ready (no rules)` и два «правило пропущено» — оператор читает это как
«у меня что-то отвалилось», хотя не отваливалось ничего. Это же попадёт в лог CI.
Как исправить: печатать заметку, только если хотя бы у одного бакета прогона
`opts.Versioned` (соответственно `opts.ExpireDays > 0 || opts.NoncurrentExpireDays > 0`) и
возможности сервера этого не покрывают. Три строки в `Init`, `PlatformBuckets` уже под рукой.
Тест `TestMissingCapabilitiesAreANoteNotAFailure` при этом придётся переписать на бакет с
правилами — что и покажет, что заметка проверяется по делу.

**Mi-5. `storage.go:144-147` — `--json` смешивает два стиля ключей.** `objstore.Capabilities`
объявлен без json-тегов, поэтому в отчёте рядом с `"expire_days"`, `"noncurrent_expire_days"`,
`"schema_files"` появляется `"capabilities": {"Versioning": …, "Lifecycle": …}`. `--json` заявлен
(решение 2 в `dev-log.md` §2) как форма для скрипта, то есть это контракт вывода, и потребителю
придётся помнить исключение.
Как исправить, не трогая общий `shared/objstore` (это уже `contract-change`): локальный тип в
`storage.go` с двумя полями и тегами `json:"versioning"` / `json:"lifecycle"`, заполняемый из
`caps`. Альтернатива (теги в самом `objstore.Capabilities`) лучше по существу, но требует
запроса system-architect и в объём T-010 не входит.

**Mi-6. `sources.go:51-63` — `knownSources` не привязан к константам `Source*`.**
Список ведётся руками рядом с константами, тестов на его полноту нет ни в `shared/contracts`,
ни в `cmd/mvctl`. Сам разработчик записал это допущением в `dev-log.md` §6, но описал только
последствие «ложная находка». Последствие точнее и неприятнее: тот, кто в EPIC-002/003 добавит
`SourceX` и пропишет его в `Publishers`, получит не ошибку компиляции и не падение теста своего
пакета, а падение `contracts check` в чужом job'е с сообщением «`"core/x"` is not an envelope
source of the platform» — то есть подсказку ровно наоборот: похоже, что неверен `Publishers`,
а неверен список.
Как исправить: тест в `shared/contracts`, который держит список и константы вместе. Дёшево и
без рефлексии — перечислить константы в тесте и потребовать `slices.Equal` с `KnownSources()`:
забывший строку в `knownSources` увидит красный тест своего пакета, а не чужого. Второй вариант —
объявить `Source*` через `var` в одном блоке с построением списка.

**Mi-7 (процесс). Правки `shared/contracts` без пометки `contract-change`.**
`tasks.md` §0 (строка про общие файлы) и общий DoD §1 п. 7 требуют менять
`shared/{eventbus,contracts,entity,clock,runtime}` только через system-architect с пометкой
`contract-change`; T-010 в `tasks.md` помечена лишь «⚠ только через tech-lead#1» — по
`cmd/mvctl/main.go`. Фактически изменены `spec.go` и `sources.go`. Из трёх правок явно
санкционирована одна: `TopicSpec.MaxMessageBytes` — решение ОВ-37 («внести в `TopicSpec` в
T-010»). Экспортируемые `TopicPartitions/TopicReplicas/TopicSegmentMS/TopicCleanupPolicy` и
новый публичный `KnownSources()` запросом не покрыты — они появились по ходу и зафиксированы
в `journal.md` задним числом (запись 2026-09-10 о выполнении T-010).
Замечание не к коду: обе правки аддитивны, ничего не ломают (`go test ./shared/...` зелёный),
и по существу правильны. Нужна явная отметка: либо tech-lead#1/system-architect подтверждает
их как принятые (одна строка в `journal.md` и пометка `contract-change` у задачи), либо они
переносятся в задачу владельца. Исправлений в коде не требуется.

**Mi-8 (безопасность, `shared/env/vars.go:82`, `.env.example:53`).**
`MV_CORE_ADMIN_CLIENTS=operator,mvctl` формально покрыт ОВ-28 («mvctl и ci-harness добавляют себя
в своих задачах»), и как решение я его не оспариваю. Проблема в том, что описание переменной,
изменённое тем же диффом, теперь противоречит своему же значению: там сказано «the default is
fail-closed, a client is added when it needs the routes», а ни одна подкоманда **этой** сборки
в `/v1/admin/*` не ходит — `world`, `laws`, `snapshot` принадлежат EPIC-002/003 и сейчас
возвращают 2. `runtime.AdminOnly` (`shared/runtime/http.go:112`) пускает по самозаявленному
`X-Client-Id` без аутентификации, то есть строка в списке — это и есть весь допуск; порт на
loopback, поэтому риск невелик, но принцип ADR-009 п. 9 сформулирован именно как «добавляем,
когда понадобилось».
Как исправить (на выбор оркестратора, обе правки — одна строка): перенести добавление `mvctl`
в задачу EPIC-002, где появляется `mvctl world init`; либо оставить как есть, но убрать из
описания оговорку «when it needs the routes», чтобы дефолт и его собственная документация не
расходились. Второе достаточно, если ОВ-28 читается как «заранее».

### Nit

**N-1. `cli.go:236-239`.** Если `json.Encoder.Encode` упал, `Report.Write` возвращает
`ExitFindings` (1) — «команда отработала и что-то нашла». Сбой записи в stdout не находка;
по собственной таблице кодов пакета это ближе к 2 либо к отдельному коду. Влияния почти нет
(упасть может только на закрытом stdout), но комментарий к константам обещает другое.

**N-2. `topics.go:65-77`.** В человекочитаемой таблице есть `partitions`, но нет `replicas` и
`cleanup.policy`, хотя команда заявлена как «целевая конфигурация кластера»; в `--json` и в
`rpk` они есть. Две колонки — если строка не станет слишком широкой.

**N-3. `env.go:60`.** Умолчание `--file` — `sharedenv.ExamplePath`, то есть относительный
`.env.example`: команда работает только из корня репозитория. Для `make contracts` и CI это так и
есть, тесты передают путь явно; одна фраза в описании флага («относительно текущего каталога»)
сняла бы вопрос у того, кто запустит `mvctl` из `cmd/`.

**N-4. `check.go:78-104`.** Два спека с одним `Type` во входе дают находку с неверной причиной
(«registered without the schema file …», потому что первый проход уже сделал `delete(files, …)`),
тогда как у топиков дубль ловится явно («listed twice in the topic map»). На реальном пути это
недостижимо — `contracts.New` (`contracts.go:72`) сам падает на `duplicate type`, — так что это
касается только тех, кто соберёт `Input` руками; симметричная проверка по спекам стоила бы трёх
строк.

**N-5 (вне области, в бэклог devops).** `build/Dockerfile:26`: комментарий «wave 0 builds
multiverse, mvctl follows» устарел — `go build … ./cmd/...` собирает `mvctl` с этой задачи, и
`-ldflags "-X main.version=…"` (строка 18) на него распространяется, `mvctl version` штампуется
корректно. Файл принадлежит T-004/T-008.

### Что проверено и признано верным

- **Коды возврата.** 0/1/2 разделены последовательно во всех четырёх командах и в реестре;
  ошибка использования (`--store=s3`, лишний позиционный аргумент, неизвестный формат,
  неизвестный флаг) даёт 2, недоступный сервер и расхождения — 1. Зарезервированное имя
  возвращает 2 — это **не** маскирует отсутствие команды в CI: код ненулевой, шаг падает,
  сообщение называет эпик-владельца. Проверено прогоном, не только чтением.
- **Разделение потоков.** Находки и итог по находкам — в stderr, полезная нагрузка — в stdout;
  `--format=rpk` дополнительно гасит `Summary`, и тест требует, чтобы каждая строка stdout
  начиналась с `rpk topic `. Перенаправление `> init.sh` действительно даёт файл без мусора.
- **Проверка (в) и `Spec.Reserved`.** Исключение `contracts.md` §16 п. 4 реализовано узко:
  зарезервированный тип не обязан иметь издателя и потребителя, но опечатка в его `Publishers`
  ловится — это отдельный подтест, и он проходит. Требование «у каждого TopicSpec есть тип»
  и обратное («тип идёт в топик карты») реализованы обоими направлениями, `dead_letters`
  исключён по делу (обёртка `DeadLetter` типа не несёт).
- **Проверка (г) действительно ловит.** Подкладывал синтетику сам: пустая `Policy` у типа на
  `player_events` → две находки (`accepts actor_kind=system`, `accepts actor_kind=human with
  meta.agent`); пустая `Policy` у типа на `llm_records` → находка про запись без агента;
  политика уже топика (`ActorKinds=[human]`) → `refuses actor_kind=ci`; выдуманный
  `actor_kind=operator` → отдельная находка. Восемь фикстур (4 `actor_kind` × агент есть/нет)
  покрывают всё пространство поведения `eventbus.Policy` — у неё ровно два поля, — так что
  сверка на фикстурах здесь не слабее структурного сравнения, но переживёт добавление поля.
- **Секреты в `env check`.** Проверил все ветки вывода, а не только ту, что закреплена тестом:
  `example.go:227` (ветка `v.secret`) стоит **до** веток enum и типа, поэтому секрет с
  неподходящим значением уходит по ветке без значения; `Var.errf` (`env.go:440`) не кладёт
  значение в `Error.Value` для секрета; `reasonOf` в `env.go:124` печатает `problem.Value`
  только когда `Secret == false`. Ветка «не `*env.Error`» печатает `err.Error()`, но `Validate`
  других ошибок не производит. `--json` идёт через те же `Finding`. Утечки нет ни в одной ветке.
- **Сверка `.env.example` в обе стороны** реально двусторонняя (`CheckExampleFile`:
  объявлено-но-нет-в-файле и есть-в-файле-но-не-объявлено), плюс дубли присвоений, устаревшие,
  enum и типы; `--env` добавляет полную проверку манифеста против окружения — вторая половина
  ОВ-27 закрыта.
- **`storage init` идемпотентен** и на `objstore.Memory`, и на живом MinIO: прогнал
  `-tags integration` — повторный запуск не тронул положенный между прогонами объект, бакет
  создан, `Capabilities()` на сервере `{true,true}`. Правила бакета берутся из
  `objstore.BucketOptionsFor`, а не переизобретаются в команде (тест это и стережёт);
  `ops-artifacts` без правил — совпадает с `infrastructure.md` §5.2. Таймаут 30 с ограничивает
  весь прогон, зависшего pipeline не будет.
- **Значения `--format=rpk` совпадают с `build/redpanda-init.sh`** по составу и порядку
  конфигов: `cleanup.policy=delete`, `retention.ms` (2592000000 / 7776000000 / 15552000000),
  `segment.ms=86400000`, `max.message.bytes=4194304` только у `llm_records`, `-p 1 -r 1`.
  Расхождений нет; замечания Mi-2/Mi-3 — о полноте теста и о применимости вывода, не о значениях.
- **Расширяемость реестра.** Таблица `name → Command`, дубль имени — паника при сборке реестра
  (тест есть), владелец зарезервированного имени виден в `mvctl help`, добавление команды —
  одна строка. Флаги единообразны (`--json` у всех четырёх, `-h` не ошибка, `flag` не зовёт
  `os.Exit`), лишний позиционный аргумент отвергается везде.
- **Аддитивность правок в общем коде.** `MaxMessageBytes` — новое поле с нулевым значением по
  умолчанию, константы и `KnownSources()` — новые имена; существующие тесты
  `shared/{contracts,env,eventbus,objstore,runtime}` и `cmd/multiverse` зелёные.
- **`Makefile`.** Удаление строки `mvctl blueprint validate blueprints/` обосновано (команда
  зарезервирована и возвращает 2, цель падала бы всегда), причина и момент возврата записаны
  комментарием над целью, решение подтверждено ОВ-48. Все три оставшиеся строки цели прогнаны
  вручную — зелёные.
- **Отклонения из `dev-log.md` §3** (имя `llm` вместо `llmusage`, строка в `Makefile`,
  нерасширение API `shared/env`) закрыты решениями оркестратора ОВ-46, ОВ-48, ОВ-49
  (`journal.md` 2026-09-10) и повторно как замечания не выписаны.

### DoD

| Пункт DoD T-010 | Статус |
|---|---|
| `contracts check` — 0 фантомных типов/топиков | выполнен (код 0 на реальном реестре); ограничение — Mi-1 |
| `env check` зелёный на `.env.example` | выполнен |
| `storage init` создаёт `ops-artifacts` на `objstore.Memory` и на MinIO (integration) | выполнен, прогнал оба |
| `make contracts` зелёный | выполнен по содержанию (три команды цели зелёные); `make` на машине нет — ОВ-42 |
| unit на синтетическом реестре с фантомом → код ≠ 0 | выполнен, плюс мои собственные синтетические прогоны |
| Общий DoD §1 п. 1 (lint, gofmt) | выполнен |
| Общий DoD §1 п. 2 (build, vet, test) | выполнен, кроме `-race` (недоступен на машине) |
| Общий DoD §1 п. 3 (тесты в той же задаче) | выполнен, покрытие 75–94 % |
| Общий DoD §1 п. 4 (gitleaks) | выполнен |
| Общий DoD §1 п. 5 (`dev-log.md`) | выполнен, запись подробная и честная (риски §6 названы сами) |
| Общий DoD §1 п. 6 (критерии историй, NFR-074) | выполнен |
| Общий DoD §1 п. 7 (карта владения, контракты) | `cmd/mvctl/**` — по карте (строка 20, каркас EPIC-001); по `shared/contracts` см. Mi-7 |
| Общий DoD §1 п. 8 (CI) | n/a до T-012 |

### Предложения в бэклог

1. **Генерация `build/redpanda-init.sh` из `contracts topics --format=rpk`** — уже отправлено
   оркестратором в бэклог devops (ОВ-49). Закрывает разом Mi-2 и Mi-3: скрипт перестаёт быть
   вторым источником, а обёртка идемпотентности пишется один раз в генераторе.
2. **Свести `topicPolicies` (`check.go:209`) с конструкторами `playerEvent`/`swarmEvent`
   в `shared/contracts`.** Сейчас правило уровня топика записано дважды; разработчик назвал это
   риском сам. Расхождение поймает эта же проверка, но карту придётся править руками при
   появлении топика с новым правилом. Область — владелец `shared/contracts`, не T-010.
3. **Заглушка `mvctl privacy scan`** — решено ОВ-47 (T-012); при её появлении имя `privacy`
   перестаёт быть зарезервированным, правка в `cmd/mvctl/main.go` идёт через tech-lead#1.
4. **Возврат `mvctl blueprint validate blueprints/` в цель `contracts`** — ОВ-48, внести в DoD
   задачи EPIC-003.
5. **`design.md` §4.1: `llmusage` → `llm`** — ОВ-46, при ближайшей ревизии документа.

### Риски и допущения

- `-race` на машине недоступен, поэтому общий DoD §1 п. 2 проверен без него. Для T-010 это почти
  не сужает проверку: в `cmd/mvctl` нет ни одной горутины, разделяемого состояния между командами
  нет, единственный конкурентный код на пути — `minio-go` внутри `objstore`, и он принадлежит
  T-007.
- `make` на машине нет (ОВ-42), цель `contracts` проверена построчным прогоном её рецептов.
  Вывод «`make contracts` зелёный» опирается на то, что рецепт — три команды без условий и
  переменных.
- Integration-тест `storage` прогнан на образе из `build/versions.env`
  (`multiverse-core/minio:RELEASE.2025-10-15T17-29-55Z`), один прогон. Флейкость на повторных
  запусках не проверял.
- Mi-1 воспроизведён на синтетическом `Input`, а не на реальном дереве схем: сегодня все схемы
  `v1`, и подложить вторую версию в `schemas/events/` значило бы менять чужие файлы. Вывод
  «на реальном пути тоже не поймает» опирается на то, что `Input.SchemaFiles` в
  `contracts.go:92` заполняется `fs.Glob("events/*.json")`, а глоб сортирован.
- Кроме этой записи в `review.md` ничего не правил и не коммитил. Временные `_test.go`,
  которыми проверялись Mi-1 и проверка (г), удалены; `git status` по `cmd/mvctl` содержит
  только файлы задачи.

---

## T-011 · ревью #1 · 2026-09-10 · code-reviewer#2 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `eda1b3b`, изменения в индексе (коммита нет).
В границах — только файлы T-011 (F-10a):

- `shared/entity/**` — `entity.go`, `ops.go`, `path.go`, `attrs.go`, `hash.go`, `types.go`,
  `README.md` и тесты `ops_test.go`, `entity_test.go`, `hash_test.go`, `attrs_test.go`,
  `schema_test.go`;
- `.golangci.yml` (снятие временного исключения `forbidigo` для `shared/entity`, ОВ-12);
- запись «developer#2 · T-011» в `dev-log.md`.

**Вне границ** (смотрит code-reviewer#1): `cmd/mvctl/**`, `shared/contracts/{spec,sources}.go`,
`shared/env/vars.go`, `Makefile`, `.env.example` — T-010. Находки там в отчёт не включались.

Основания: `tasks.md` §1 (общий DoD) и T-011; `architecture/components/state-and-mechanics.md`
v0.2 §3 (§3.1–§3.3), §4.5, §4.8, §4.10; `analysis/data-model.md` v0.2.1 §3.1–§3.7;
`architecture/contracts.md` v0.4 C-02 v1.2 и C-03; `schemas/events/entity.*.v1.json`,
`schemas/events/_common.json`; ADR-003 п. 6, ADR-011, ADR-013; `plan/ownership.md` §1 (строка
`shared/entity/**`); `journal.md` от 2026-09-10 — решения оркестратора по шести ОВ T-011.

### Вердикт

**Вернуть.** Critical: 0 · Major: 3 · Minor: 11 · Nit: 5.

Модель сделана на совесть и заметно выше уровня заглушки: сигнатуры §3 совпадают дословно,
часов в пакете нет ни одного, политика действительно осталась за `internal/state`, канонический
JSON детерминирован и внутри процесса, и между процессами (проверено двумя отдельными прогонами),
геттеры и согласование со схемами `entity.*` закрыты тестами, покрытие 83,9 % при пороге 60 %,
`golangci-lint` — 0 issues без единого `nolint`.

Возврат — из-за трёх Major, и все три бьют ровно в то, ради чего задача делалась: в
воспроизводимость состояния по журналу.

1. Есть три способа изменить атрибуты так, что `changed[]` останется пустым: `append` элемента
   `null`, `set` отсутствующего ключа в `null` и `remove` ключа, у которого значение `null`.
   Во всех трёх `state_hash` сдвигается, а версия — нет, и факт уходит с `changed: []`.
   Обещание dev-log «не изменилось = не сдвинуло `state_hash`» и гарантия C-02 «version строго
   +1 при непустом `changed[]`» перестают быть эквивалентны, а `mvctl session-report --audit`
   (пересчёт хэша из снапшота + `changed[]`) на такой сущности разойдётся с реальностью.
2. Индексный сегмент по отсутствующему или не-`[]any` контейнеру молча делает объект с ключом
   `"0"` вместо списка — то самое, чего пакет, по собственному комментарию в `path.go`, избегает
   отказом от `jsonpath.Accessor.Set`. Тест `TestPathGrammarMatchesJSONPath`, который должен был
   это прибить, поймать не может по устройству: читающая половина (`jsonpath.navigate`) на map
   разрешает `[0]` как обычный ключ `"0"` точно так же, поэтому обе половины ошибаются
   согласованно и тест зелёный.
3. `remove` элемента списка по индексу пишет `changed = {path: "inventory[0]", new: null}`, тогда
   как список сдвинулся. Решение 6 dev-log закрыло эту дыру для формы `remove` со значением
   (путь = путь списка), но индексная форма осталась.

Все три — правки в пределах десятка строк плюс тесты; ни одна не требует пересмотра дизайна.
Minor — качество и полнота, в основном по DoD «геттеры всех атрибутов §3» и по устойчивости
канонической формы как межреализационного контракта.

### Что проверено прогоном (машина владельца, `GOFLAGS=-buildvcs=false`, `-race` недоступен)

| Проверка | Результат |
|---|---|
| `go build ./...` | ok |
| `go vet ./...` | ok |
| `go test -short -count=1 ./...` | все пакеты `ok` |
| `go test -short -count=1 -cover ./shared/entity/` | **coverage: 83.9 %** (порог 60 %) |
| `golangci-lint run ./...` | **0 issues** |
| `gofmt -l shared/entity` | пусто |
| `gitleaks git --staged --redact .` | `no leaks found` |
| `grep -rn "nolint|time.Now|time.Since" shared/entity/*.go` | пусто (`time.Now` упоминается только в тексте `README.md`) |
| детерминизм хэша **между процессами** | два отдельных прогона зонда дали побайтово одинаковый вывод |

Отдельно: при первом прогоне `go test ./...` падал `cmd/mvctl/internal/contracts`
(`TestZZReviewStaleVersionGlobOrder`, файл `zzreview_test.go`). Это зонд параллельного ревьюера
в области T-010; к моменту повторного прогона файл был удалён и весь `./...` зелёный. К T-011
отношения не имеет, в замечания не выношу.

Проверка краевых случаев велась отдельным модулем-зондом в каталоге скретчпада (собственный
`go.mod` с `replace` на репозиторий), рабочее дерево репозитория не изменялось — `git status` по
`shared/entity` и `.golangci.yml` до и после ревью совпадает.

### Сверка с решениями оркестратора (`journal.md`, 2026-09-10)

| Решение | Реализовано | Комментарий |
|---|---|---|
| (1) Атрибуты региона: `data-model.md` §3.2 + `encounter_chance` | да | `NPCIDs()`, `PlayersPresent()`, `EncounterChance()`; `AttrNPCs` относится к встрече, не к региону — см. риски |
| (2) `scope` — объект `{id, type}` | да | `Scope()` читает обе формы; doc-комментарий отстал от решения (N-2) |
| (3) Сортировка ключей на всех уровнях | да | `writeCanonicalObject` применяется и к верхнему уровню, тест `hash_test.go:14` это фиксирует |
| (4) `name` не зарезервирован | да | `ReservedPaths` без `name`; проверено таблицей `ops_test.go:379` |
| (5) `HistoryEntry` по §3.1 | да | `{Version, EventID, ProposalID, At}` + json-теги |
| (6) No-op по канонической равности | да, но не полностью | `sameCanonical` сравнивает **значения**, но не факт существования пути — отсюда M-1 |

### Проверенные пункты DoD

| Пункт DoD T-011 | Итог |
|---|---|
| unit покрывает `ApplyOps` по всем операциям, ошибкам и no-op | **да** — четыре операции, 9 причин `Reason*`, no-op, «первый `old` / последний `new`», пустой список ops, зарезервированные пути |
| `StateHash` стабилен между запусками и не зависит от порядка ключей | **да** по факту (проверено вручную между процессами), но тестом пинится только внутрипроцессная пересборка — Mi-2 |
| `CanonicalJSON` детерминирован | да |
| покрытие `shared/entity` ≥ 60 % | да, 83,9 % |
| сигнатуры совпадают с §3 | **да, дословно**: `Entity` (12 полей и теги), `HistoryEntry`, `LastChange` (8 полей), `OpKind`/`OpSet`/`OpInc`/`OpAppend`/`OpRemove`, `Op`, `Change`, `ApplyOps(e *Entity, ops []Op) (map[string]any, []Change, error)`, `Clone(e *Entity) *Entity`, `CanonicalJSON(e *Entity) []byte`, `StateHash(es []*Entity) string`, `ErrInvalidOp{Op, Reason}`. Дополнения (`New`, `Commit`, `CheckVersion`, `ChangeSet`/`Propose`, `Ref`⇄`eventbus.EntityRef`, геттеры, `StatusTransitionAllowed`) — расширение, §3 не ломают |
| геттеры **всех** атрибутов `data-model.md` §3 | **частично** — Mi-8 |
| снятие исключения `forbidigo` (ОВ-12) | **да**, и без `nolint`-заплаток |
| общий DoD §1 п. 1, 2 (кроме `-race`), 3, 4, 5, 7 | да |

### Замечания

#### Major

**M-1. Состояние меняется, `changed[]` пуст, версия не растёт — три пути.**
`shared/entity/ops.go:377-389` (`changeTracker.changes`), `ops.go:219-241` (`applyAppend`),
`ops.go:243-254` (`applyRemove`).

`changes()` отбрасывает запись по `sameCanonical(Old, New)`. И «пути не было», и «по пути лежит
`null`» дают одну и ту же каноническую форму `null`, поэтому отбрасываются и настоящие изменения.
Проверено зондом (слева — атрибуты после `ApplyOps`, справа — `changed`):

```
append tags <- null       attrs={"tags":["a",null]}   changed=[]   state_hash: 282127c1… → c3949b53…
set died_at=null (нет)    attrs={"died_at":null,…}    changed=[]   state_hash: 6e49be7d… → bc909248…
remove killed_by (=null)  attrs={"hp":10}             changed=[]   state_hash: 0d03b8df… → 6e49be7d…
```

Дальше по цепочке: `Commit` при пустом `changed` не двигает `Version` (`entity.go:188-190`), факт
`entity.updated` уходит с `changed: []`, read-model и `--audit` не узнают об изменении, а
`recovery_state_identical` на такой сущности честно скажет «не идентично». Это противоречит и
гарантии C-02 («`version` строго +1 на сущность при непустом `changed[]`»), и собственному
обоснованию отклонения 2 dev-log.

Как чинить: различать «значение» и «существование». `changeTracker.record` должен принимать
`oldExists bool` (у `readPath` он уже есть — `applySet`/`applyInc` его выбрасывают через `_`),
`changes()` — отбрасывать запись только при `oldExists == newExists && sameCanonical(old, new)`.
Для `applyAppend` проще: если элемент реально добавлен, это никогда не no-op — записывать без
проверки. Для `applyRemove` без значения — то же, ключ действительно исчез.

Тест, который стоит завести вместе с правкой (он же закрывает весь класс): property на паре
«`len(changed) > 0`» ⟺ «`StateHash` до ≠ `StateHash` после» по табличке из десятка операций,
включая три строки выше. Сейчас именно этой связки в `hash_test.go` и `ops_test.go` нет, хотя
обе половины по отдельности проверены.

**M-2. Индексный сегмент по отсутствующему или не-`[]any` контейнеру молча создаёт объект;
тест грамматики этого не ловит.**
`shared/entity/path.go:138-141` (ветка `default` в `setIn`), тест
`shared/entity/ops_test.go:447` (`TestPathGrammarMatchesJSONPath`).

```
set members[0]="x" при отсутствующем members   →  attrs={"members":{"0":"x"}}
set tags[0]="z"    при tags = []string{"a","b"} →  attrs={"tags":{"0":"z"}}   (список потерян целиком)
```

Первый случай — ровно тот дефект, из-за которого, по комментарию `path.go:16-18`, половина
грамматики и была переписана здесь вместо `jsonpath.Accessor.Set`. Второй — молчаливая потеря
данных: `asAnySlice` умеет нормализовать `[]string` (и `applyAppend` этим пользуется), а `setIn`
про него не знает и уходит в `default`.

Оба случая противоречат решению 7 dev-log и строке README «индекса нет в списке → `invalid_op`»:
индекс за пределами **существующего** `[]any` даёт `ReasonBadIndex`, а индекс по пустоте —
объект.

Почему тест не помогает: `jsonpath.navigate` (`shared/jsonpath/accessor.go:95-134`) на
`map[string]any` разрешает сегмент `"0"` как обычный ключ. Поэтому после записи
`{"members":{"0":"x"}}` чтение `members[0]` возвращает `"x"`, и тест зелёный. Обе половины
расходятся с грамматикой согласованно — а тест сверяет их только друг с другом, не с ожидаемой
формой данных.

Как чинить:

- в `default`-ветке `setIn` — если `head` разбирается как неотрицательное целое, возвращать
  `errIndexOutOfRange` (список растит только `append`, ровно как решено в п. 7);
- перед спуском нормализовать не-`[]any` срезы через `asAnySlice`;
- в `TestPathGrammarMatchesJSONPath` добавить кейсы `missing[0]` и `tags[0]` над `[]string` и
  проверять **тип контейнера** (`attrs["members"]` должен быть `[]any`), а не только то, что
  `jsonpath` что-то прочитал по тому же пути. Иначе тест продолжит подтверждать согласованность
  двух одинаковых ошибок.

**M-3. `remove` элемента списка по индексу пишет `changed` с `new: null`.**
`shared/entity/ops.go:243-253`.

```
remove inventory[0]   attrs={"inventory":[{"item_id":"i2"}]}
                      changed=[{"path":"inventory[0]","old":{"item_id":"i1"},"new":null}]
```

Потребитель, применяющий `changed[]` буквально (а это заявленный способ построения read-model и
пересчёта хэша в `--audit`, C-02 и §3.3), запишет `null` в нулевой элемент вместо сдвига списка и
разойдётся с состоянием State. Решение 6 dev-log эту же проблему для формы `remove` со значением
решило правильно — путь списка, `old`/`new` — список до и после; индексная форма проехала мимо.

Как чинить: если последний токен пути — индекс, записывать изменение по пути родительского
списка со списками до и после (одна ветка, повторяющая уже написанную ниже логику), либо явно
отвергать индексный путь у `remove` без значения. Первое предпочтительнее — иначе `remove`
теряет единственный способ убрать элемент, у которого нет `item_id`/`player_id`/`npc_id`.

#### Minor

**Mi-1. `append` + `remove` того же элемента в одном предложении дают фантомную запись и лишний
рост версии.** `ops.go:359-389`.

```
ops: append inventory <- {item_id:i1}; remove inventory {item_id:i1}
attrs={"inventory":[]}
changed=[{"path":"inventory[0]","old":null,"new":{"item_id":"i1"}},
         {"path":"inventory","old":[{"item_id":"i1"}],"new":[]}]
```

Состояние вернулось туда, откуда вышло, но `changed` непуст → `Commit` двигает `Version`, а факт
сообщает об `append`, которого в итоговом состоянии нет. §3.2 требует «один элемент на конечный
путь… no-op исключается». Корень тот же, что у M-1/M-3: трекер ключуется текстом пути, а не
итоговым состоянием. Минимальная правка в рамках задачи — при записи изменения по пути списка
удалять из трекера ранее записанные пути-элементы этого списка (`<path>[i]`).

**Mi-2. Нет golden-значения `CanonicalJSON`/`StateHash`.** `hash_test.go:14`, `hash_test.go:101`.
Стабильность «между запусками» из DoD пинится 64 пересборками карты внутри одного процесса —
это проверяет независимость от порядка итерации, но не от изменения самого кодировщика. Любая
будущая правка `writeCanonical` (добавили поле, поменяли формат числа) молча обесценит все
сохранённые `snapshot.state_hash` и не уронит ни один тест. Нужен литерал: фиксированная сущность
→ ожидаемая строка `{"attributes":…}` и ожидаемый `sha256:<hex>`.

**Mi-3. `json.Number` в атрибутах попадает в канонический JSON дословно.** `hash.go:74`.
`{"hp":1e2}`, прочитанное декодером с `UseNumber`, даёт `{"attributes":{"hp":1e2},…}` — с
экспонентой, вопреки §3.3 «числа как int64/float64 без экспоненты»; `10` и `10.0` из такого
декодера тоже разойдутся. Сейчас недостижимо (`UseNumber` в репозитории не используется —
проверено `grep`), но это одна строка в будущем декодере шины, и она тихо расколет хэши.
Чинится приведением `json.Number` к `int64`/`float64` и передачей в общий писатель.

**Mi-4. Каноническая форма зависит от HTML-экранирования `encoding/json`.** `hash.go:162-171`.
Символы `<`, `>` и `&` — и в значениях, и в ключах — уходят в каноническую форму
шестизначными escape-последовательностями (`u003c`, `u003e`, `u0026` после обратной косой),
потому что `writeCanonicalString` зовёт `json.Marshal`, а тот по умолчанию экранирует HTML.
В Go это детерминированно, но §3.3 — контракт
между реализациями (комментарий `hash.go:25-26` прямо говорит «одно правило, которое сможет
воспроизвести вторая реализация»), а такого правила ни в §3.3, ни в README нет. Либо
`json.Encoder` + `SetEscapeHTML(false)`, либо явная строка в README и запрос на уточнение §3.3.

**Mi-5. Пути `a[]` и `a[x]` не отвергаются.** `path.go:47-57` + `setIn`.
`set a[] = 1` → `{"a":{"":1}}`, `set a[x] = 1` → `{"a":{"x":1}}`. Грамматика §3.2 — `a.b[0].c`;
скобки с неразбираемым содержимым — тот же malformed path, что и незакрытая `[`, и должны давать
`ReasonBadPath`. Сейчас `[` без `]` отвергается, а `[]`/`[x]` — нет.

**Mi-6. Экспортированные изменяемые срезы.** `ops.go:101` (`ReservedPaths`), `types.go:19`
(`Types`), `types.go:39` (`TerminalStatuses`). Любой потребитель может их дописать, укоротить или
переупорядочить — и изменить проверку `invalid_op` глобально; при этом `slices.Contains` по ним
читается из воркеров всех миров, то есть конкурентная запись извне — гонка (проверить `-race` в
этом окружении нельзя, вывод из кода). Предпочтительнее `func IsReservedPath(path string) bool` и
`func EntityTypes() []string` (копия) при неэкспортируемых срезах; минимум — строка в
док-комментарии «не изменять».

**Mi-7. `Commit` забирает карту вызывающего без копии.** `entity.go:187`. Зонд: после
`e.Commit(attrs, …)` запись `attrs["hp"] = 999` меняет `e.Attributes`. В штатном потоке безопасно
(`ApplyOps` возвращает свежую копию), но `overlayView` из §4.5 п. 8 держит те же копии между
шагами 7 и 10, а пакет обещает в док-комментарии обратное — «никакой записи за спиной». Либо
клонировать, либо явной строкой в комментарии `Commit` зафиксировать передачу владения.

**Mi-8. Геттеры покрывают не все атрибуты `data-model.md` §3** (DoD требует «всех»):
нет `Epoch()` при существующей константе `AttrEpoch` (`types.go:133`, §3.1); нет `spawned_by`
(§3.4) — ни константы, ни геттера; нет `opened_by_event_id`/`closed_by_event_id` (§3.7); нет
`last_session_ended_at` (§3.3). Последний помечен в §3.3 как «может жить в game-service» — по
нему достаточно строки в README; остальные три стоит добавить либо явно перечислить как
намеренно опущенные (как это уже сделано для `city`/`item`/`actor` в `types.go:6-7`).

**Mi-9. Две разные шкалы названы одним набором констант.** `types.go:85-90`.
`ParticipationActive/Idle/OutOfCombat` документированы как «C-03 `Actor.Participation`,
`data-model.md` §3.6», но §3.6 для `members[].participation` перечисляет только `active|idle`, а
`out_of_combat` приходит из C-03. При этом enum состояния участника встречи §3.7
(`in_combat|out_of_combat|idle|dead`) констант не имеет вовсе, хотя `Participant.State` — поле
структуры этого же пакета (`attrs.go:346-351`). Либо развести две шкалы двумя наборами констант,
либо уточнить комментарий, что набор — надмножество и относится к C-03.

**Mi-10. Обратное направление «схема → модель» проверено на одном типе из пяти.**
`schema_test.go:199` разбирает только `entity.update.proposed`. Вторая входящая для State —
`entity.create.proposed` (payload → `attributes` → `New`) — в обратную сторону не проверяется, а
именно она несёт `attributes` целиком и именно на ней в T-016 поедут фикстуры. Прямое
направление (модель → payload) закрыто по всем пяти типам, это в порядке.

**Mi-11. `Commit` при пустом `changed` всё равно дописывает `HistoryEntry` с той же версией.**
`entity.go:195-199`. Три холостых хода подряд дают три записи с `version: 1`, причём
`SetFactEventID` штампует только последнюю (`entity.go:211-213`), и предыдущие навсегда остаются
с `event_id: ""`. Дословно это соответствует §4.5 п. 10 («`History` append» без условия), поэтому
как дефект не выписываю, но следствия неприятные: `history` перестаёт быть монотонной по
`version`, лимит 50 выжигается ходами без изменений (а UC-011 E2 — «HP уже max, ход засчитан» —
это штатный сценарий отдыха), и пустой `event_id` перестаёт быть диагностическим признаком.
**Вопрос к architect#1**: нужна ли запись в `history` для хода без изменений, и если нужна — что
писать в `event_id`.

#### Nit

**N-1. `TestInvalidOpBecomesAValidRejection` проверяет не то, что заявляет.**
`schema_test.go:164-171`. Комментарий обещает «что два enum-а сходятся», а тело подставляет в
payload строковый литерал `"invalid_op"` и валидирует его схемой — отображения
`ErrInvalidOp` → `reason` в пакете нет, и тест его не касается. Либо переформулировать
комментарий, либо (полезнее) завести в пакете отображение ошибки в причину и проверить его — но
это уже граница с политикой State, так что вопрос к architect#1.

**N-2. Док-комментарий `Scope()` отстал от решения оркестратора.** `attrs.go:164-166` пишет про
«сокращение `solo:{id}`, которое пишут фикстуры». По решению от 2026-09-10 (п. 2) фикстуры T-016
пишут объект, а строка остаётся только сокращением блупринтов. Строку стоит поправить сейчас,
пока T-016 на неё не сослался.

**N-3. `StatusTransitionAllowed(x, x) == false` не объяснён.** `types.go:59-67`. Поведение
осознанное, но State, спросив разрешение перед `set status=alive` над живым персонажем, получит
запрет там, где `ApplyOps` даёт честный no-op. Достаточно строки в док-комментарии.

**N-4. `ApplyOps(nil, …)` и `CanonicalJSON(nil)` паникуют**, тогда как `Clone(nil)` и `StateHash`
с `nil` в срезе аккуратно возвращают `nil`/пропускают. Ноль стоимости привести к одному поведению
или зафиксировать «не nil» в док-комментарии.

**N-5. Проверка зарезервированных путей смотрит только на первый сегмент.** `ops.go:142`.
`set stats.version = 9` и `set a._secret = 9` проходят. §3.2 говорит «любой путь с ведущим `_`» —
прочтение «ведущий сегмент» защитимо, и корневой `_intent` (ADR-013) закрыт тестом. Но `_`-ключи
задуманы как служебные на любом уровне. **Вопрос к architect#1** при ревизии §3.2 — заодно с
подтверждением п. 4 решения по `name`.

### Отдельно проверено и признано верным

- **Часов в пакете нет.** `time.Now`/`time.Since` не встречаются ни в одном `.go`-файле;
  `New`, `Commit`, `SetFactEventID` берут время аргументом. Исключение `forbidigo` в
  `.golangci.yml` снято именно удалением строки, а не переносом в `nolint` — заплаток в пакете
  нет ни одной. ОВ-12 закрыт корректно.
- **`ApplyOps` действительно не трогает сущность.** `cloneAttrs` через `jsonpath.Clone`, ошибка
  возвращает `nil, nil, err` без частичного результата; тесты `ops_test.go:60` и `ops_test.go:430`
  это держат. Первая же плохая операция останавливает весь набор — совпадает с §4.5 п. 1
  («ошибка формата ops → `rejected` на всё предложение»).
- **Оптимистичная версия.** `CheckVersion(nil)` — не блокировка (правило «`expected_version`
  обязателен для `hp`/`status`/`inventory`/`position`» остаётся за State, и это верно: ADR-013
  п. 1 адресован писателю, не модели). `expected_version: 0` из схемы (`minimum: 0`) даёт
  конфликт против `version: 1` — правильно, версии начинаются с единицы.
- **Терминальные статусы и таблица переходов.** `alive → abandoned` разрешён, выход из
  `dead`/`abandoned`/`ascended_final` запрещён — совпадает с C-02 v1.2 и inv-09. `IsTerminal()`
  читает атрибут `status`, которого у `group`/`encounter` нет, поэтому для них он ложен — ровно
  исключение §4.5 п. 5.
- **Хэш исключает то, что должен.** `updated_at`, `history`, `last_change`, `last_event_id`, а
  также `name` и `world_id` (README §«Хэш» это честно называет, §3.3 — только первые четыре;
  README здесь точнее документа). Тест `hash_test.go:38` проверяет все шесть.
- **Сортировка на всех уровнях, включая верхний**, и `10` = `10.0` в канонической форме — обе
  вещи закрыты тестами (`hash_test.go:14`, `hash_test.go:66`) и соответствуют решениям
  оркестратора 3 и 6.
- **Клэмп `hp`.** Работает и по индексному пути (`inc members[0].hp -100` → `hp: 0` при
  `hp_max: 10`), сосед `hp_max` берётся по тому же родителю; без `hp_max` остаётся только нижняя
  граница — это ровно то, что написано в §3.2 и в решении 9 dev-log.
- **`omitempty` у `Op.Value`.** Для интерфейсного поля отбрасывается только `nil`, поэтому
  `value: false` и `value: 0` доезжают до провода; тест `ops_test.go:478` это фиксирует — ловушка
  неочевидная и закрыта правильно.
- **Дедуп `append` по `item_id`/`player_id`/`npc_id`** и его отсутствие для скаляров (список
  тегов — это список) — соответствует §3.2 и закрыто двумя тестами.
- **Границы владения.** Изменения только в `shared/entity/**`, `.golangci.yml` и `dev-log.md`;
  файлы T-010 не тронуты, пересечений с параллельной задачей нет.
- **Согласование enum'ов `cause` и статусов со схемами.** Расхождений нет: `cause` в пакете
  вообще не константа (и правильно — это словарь `OwnershipRules`/схемы, а не модели), а enum
  статусов ни в одной схеме `entity.*` не объявлен, поэтому сверять нечего. `entity.updated`
  принимает `changed[]` с обязательными `path`/`new` и необязательным `old` — структура `Change`
  пишет `old` всегда (`null` при отсутствии), схема это допускает (`"old": true`).

### Предложения в бэклог

1. **Свести обе половины грамматики путей в `shared/jsonpath`** (`Set`/`Delete` с индексами) —
   предложение самого разработчика, поддерживаю, но с уточнением: чинить надо **обе** половины.
   Читающая половина (`navigate`) сегодня разрешает `[0]` как ключ `"0"` на map, и пока это так,
   никакой тест согласованности не поймает M-2. Область — EPIC-002.
2. **Property-тест «`changed[]` ⟺ сдвиг `state_hash`»** как постоянный страж класса ошибок M-1
   (и, шире, как исполняемая формулировка гарантии C-02). Уместно завести вместе с правкой M-1 и
   унаследовать в `internal/state`.
3. **Константы `cause`** (`combat|rest|move|loot|spawn|tick|group|create|bootstrap|forget|leave|flee|death|resolve|init|author`)
   сейчас живут строковыми литералами в `shared/contracts/ownership.go` и в двух схемах. Опечатка
   в `LastChange.Cause` ловится только валидацией при публикации. Место — `shared/contracts`, не
   `shared/entity`.
4. **Сверка C-03 с построенной моделью — до старта EPIC-002.** `contracts.md` C-03 объявляет
   `func ChangesFor(...) []entity.Change` с комментарием «ops для `entity.update.proposed`», но
   после T-011 `entity.Change` — это `{path, old, new}` (результат применения), а ops — это
   `entity.Op`. Похоже на опечатку контракта: должно быть `[]entity.Op`. Там же
   `ActorFromEntity(e entity.Entity)` принимает `Entity` **по значению**, тогда как вся модель
   построена на указателях (`*Entity`), а копия по значению всё равно разделяет карту и срезы.
   Обе строки — к system-architect, до того как EPIC-002 начнёт по ним писать.
5. **Расхождение `state-and-mechanics.md` §4.10 и `data-model.md` §3.2 по атрибутам региона**
   формально закрыто решением оркестратора (канон — `data-model` + `encounter_chance`), но сам
   текст §4.10 ещё говорит `npcs: []`. Пока правка не внесена, фикстура региона, написанная по
   §4.10, окажется невидимой для `NPCIDs()`. Напоминание architect#1 — до T-016.

### Риски и допущения

- **`-race` в окружении недоступен**, вывод о гонках сделан чтением. Пакет не держит ни
  изменяемого глобального состояния, ни горутин, ни каналов; единственная поверхность —
  экспортированные срезы `ReservedPaths`/`Types`/`TerminalStatuses` (Mi-6), гонка по которым
  требует записи со стороны потребителя. Все функции пакета чистые в том смысле, что пишут только
  в переданные им или созданные ими структуры — за одним исключением, вынесенным в Mi-7.
- **Замечания M-1, M-2, M-3, Mi-1, Mi-3, Mi-4, Mi-7 получены прогоном зонда**, а не чтением:
  отдельный модуль в каталоге скретчпада с `replace` на репозиторий, шестнадцать сценариев.
  Рабочее дерево не изменялось, тестовых файлов в `shared/entity` не добавлялось; вывод зонда
  воспроизводится в двух независимых прогонах побайтово.
- **`schema_test.go` компилирует схемы напрямую из `schemas.FS`, минуя `shared/contracts`.**
  Это осознанно (сказано в комментарии теста) и на T-011 не влияет, но означает, что связка
  «модель ↔ реестр» (политики, топики, `Validate`) этими тестами не проверяется — она остаётся на
  T-010 и T-014.
- **Допущение по `Attributes`.** Разбор M-2 исходит из того, что списки в атрибутах — `[]any`
  (так приходит с провода и так дают фикстуры-JSON). Ветка с `[]string` достижима только из
  Go-кода — тестов, `mechanics.Stats`, будущего `bootstrap`; там потеря списка будет молчаливой,
  поэтому замечание я не понижал.
- **Расширения сверх §3** (`New`, `Commit`, `CheckVersion`, `ChangeSet`/`Propose`,
  `StatusTransitionAllowed`, геттеры) сигнатуры §3 не ломают, но расширяют публичную поверхность
  контрактного пакета (`tasks.md`: `shared/entity` меняется через system-architect с пометкой
  `contract-change`). Формально их принимает architect#1 — сверка сигнатур §3 заявлена в DoD как
  его работа. С моей стороны расхождений с §3 и C-02 в них нет.
- Кроме этой записи в `review.md` ничего не правил и не коммитил.

---

## T-012 · ревью #1 · 2026-09-09 · code-reviewer#1 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `fd93a6a`, изменения в индексе (коммита нет).
В границах — только файлы T-012 (F-7):

- `.github/workflows/go.yml` (новый), `.github/dependabot.yml`, `.github/CODEOWNERS`,
  удаление `.github/workflows/validate-blueprints.yml`;
- `scripts/coverage-gate.sh`;
- `cmd/mvctl/internal/privacy/**` (`privacy.go`, `scanner.go`, `privacy_test.go`,
  `scanner_test.go`) и регистрация команды в `cmd/mvctl/main.go` / `main_test.go`;
- `Makefile` (цели `test`, `privacy-scan`, `ci`), `.gitleaks.toml` (новый allowlist);
- запись «developer#1 · T-012» в `dev-log.md`.

**Вне границ** (пишет developer#2 параллельно, не смотрел и находок оттуда не включал):
`scripts/llm-bench.ps1`, `scripts/llm-bench.sh`, `ops/metrics/**`, `ops/models.txt`,
`testdata/bench/prompts.jsonl` — T-013.

Основания: `tasks.md` §1 (общий DoD) и раздел T-012; `architecture/infrastructure.md` v0.3
§3.1, §3.1.1, §3.2, §2.4, §4.5 п. 1, §12; ADR-010 + доп. п. 1, 3, 5; ADR-021;
`architecture/threat-model.md` T-15, T-16, T-26, T-31, T-35, T-36 / SEC-01, SEC-23, SEC-25,
NFR-041, NFR-062, NFR-063, NFR-071; `journal.md` — решения оркестратора от 2026-09-09
(ОВ-8, ОВ-9, `ci-harness` в `MV_CORE_ADMIN_CLIENTS`) и от 2026-09-10 (ОВ-30, ОВ-47, ОВ-48,
пять решений по ОВ T-012).

### Вердикт

**Вернуть.** Critical: 1 · Major: 3 · Minor: 9 · Nit: 6.

Workflow сделан заметно выше обычного уровня: девять пинов действий по SHA сверены с реальными
тегами через GitHub API — **все девять совпали** (аннотированный тег `golangci-lint-action@v9.3.0`
разыменован верно: `d583c34f…` → `ba0d7d2e…`); `permissions` объявлены на workflow и сужены
пожобно; `concurrency` с отменой есть; `pull_request_target` нет; ни один job не пишет в
репозиторий; все `push: false`; `continue-on-error` нет нигде; версии инструментов и образов
берутся из `build/versions.env`, литералов версий в шагах нет; `actionlint` — 0 замечаний;
`golangci-lint run ./...` — 0 issues; локальная эмуляция `contracts`, `compose-lint`, `e2e`
и обоих шагов gitleaks зелёная. Сканер `privacy` не печатает найденное значение целиком, и на
это есть отдельный тест в обеих формах вывода. Записи в `dev-log.md` — образцовые: почти всё,
что я перепроверял, там уже описано с обоснованием.

Возврат — из-за одного Critical и трёх Major, и все четыре бьют ровно в цель задачи: **CI в
текущем виде не может стать зелёным**, а два ослабления касаются контура секретов.

1. `scripts/coverage-gate.sh` лежит в индексе с режимом `100644` — шаг job `unit` падает
   с `Permission denied` на каждом прогоне (воспроизведено в Linux-контейнере, exit 126).
2. Решение оркестратора «сузить `push` до `main` и `integration/**`, внести в этой же задаче»
   не внесено.
3. Job `security` на событии `pull_request` обращается к API pull request'а, на который у
   объявленных `permissions` нет прав.
4. Новый `[[allowlists]]` в `.gitleaks.toml` прямо противоречит `infrastructure.md` §4.5 п. 1
   («не расширение allowlist», T-16), причём fingerprint ровно этой находки **уже лежит** в
   `.gitleaksignore` и не срабатывает только из-за абсолютного пути в шаге job'а.

Все четыре — правки в пределах нескольких строк, пересмотра дизайна ни одна не требует.

### Что проверено прогоном (машина владельца, `GOFLAGS=-buildvcs=false`, `-race` недоступен)

| Проверка | Результат |
|---|---|
| `actionlint .github/workflows/go.yml` (v1.7.12) | 0 замечаний |
| `go build ./...`, `go vet ./...` | ok, ok |
| `go test -short -count=1 -coverprofile=coverage.out ./...` | все пакеты `ok`; `cmd/mvctl/internal/privacy` — **86,8 %** |
| `golangci-lint run ./...` | **0 issues** |
| `gofmt -l cmd/mvctl/internal/privacy cmd/mvctl/main.go cmd/mvctl/main_test.go` | пусто |
| `go mod verify` | `all modules verified` |
| `go mod tidy -diff` | exit 1 на Windows (CRLF в рабочей копии; `git ls-files --eol` → `i/lf w/crlf`); в индексе LF, на Linux-раннере шаг зелёный — подтверждает §5 dev-log |
| Пины действий: 9 из 9 через `api.github.com/.../git/ref/tags/<tag>` | **все совпали** (см. таблицу ниже) |
| `scripts/coverage-gate.sh` — 6 сценариев | 5 пакетов «does not exist yet» → 0; `cmd/mvctl/internal` 92,6 % при пороге 60 → 0; при пороге 99 → 1; без аргументов → 2; нечисловой порог → 2; отсутствующий профиль → 2 |
| Профиль покрытия содержит пакеты **без тестов** (`shared/agent/tools`, `eventbus/examples`) | да, 36 и 27 строк — значит новый `internal/state` без единого теста даст 0 % и порог сработает, а не «no statements» |
| эмуляция job `contracts` | `65 types, 8 topics, 58 schema files`; `69 variables`; `TestSchemasValid` ok |
| эмуляция job `e2e` (`go test -tags e2e ./...`) | ok |
| компиляция тегом `integration` (`go vet -tags integration ./...`) | ok |
| эмуляция job `compose-lint` | `config -q` ok; `compose-lint.sh` — `15 services, 6 rules`; все четыре `bad-*.yml` отвергнуты |
| `go run ./cmd/mvctl privacy scan testdata/` и `--json` | `no external identifiers … (6 files read)`, exit 0; в `--json` полей со значениями находок нет |
| `gitleaks git --staged --redact .` | `no leaks found` |
| `gitleaks dir` по содержимому индекса (`git checkout-index` во временный каталог) | 0 находок с новым allowlist; **1 находка без него при абсолютном пути и 0 находок без него при относительном `.`** (см. M-4) |
| `zricethezav/gitleaks:v8.30.1` в Docker Hub | тег существует (обновлён 2026-03-21) |
| `gitleaks detect` (то, что запускает `gitleaks-action`) в v8.30.1 | подкоманда жива |
| режим файла в индексе | `scripts/compose-lint.sh` — `100755`; **`scripts/coverage-gate.sh` — `100644`** (см. C-1) |
| воспроизведение C-1 | `docker run bash:5 … chmod 644 && ./cg.sh` → `Permission denied`, exit **126** |
| `pull_request_target` в `.github/workflows/**` | отсутствует |
| зонд сканера `privacy` (12 форм внешнего ID) | 4 формы пропущены (см. Mi-3, Mi-4) |

Сверка пинов (2026-09-09, `git/ref/tags`):

| Действие | Тег | SHA в файле | API |
|---|---|---|---|
| `actions/checkout` | v7.0.1 | `3d3c42e5…` | совпал |
| `actions/setup-go` | v7.0.0 | `b7ad1dad…` | совпал |
| `actions/upload-artifact` | v7.0.1 | `043fb46d…` | совпал |
| `docker/setup-buildx-action` | v4.3.0 | `37fe6310…` | совпал |
| `docker/build-push-action` | v7.3.0 | `53b7df96…` | совпал |
| `golangci/golangci-lint-action` | v9.3.0 | `ba0d7d2e…` | тег аннотированный `d583c34f…` → объект `ba0d7d2e…`, совпал |
| `gitleaks/gitleaks-action` | v3.0.0 | `e0c47f4f…` | совпал |
| `golang/govulncheck-action` | v1.1.0 | `032d4551…` | совпал |
| `hadolint/hadolint-action` | v3.5.0 | `06be81ba…` | совпал |

### Замечания

#### Critical

**C-1. `scripts/coverage-gate.sh` не исполняемый — job `unit` падает на каждом прогоне.**
`git ls-files -s scripts/coverage-gate.sh` → `100644` (для сравнения, `scripts/compose-lint.sh`
из T-008 — `100755`). Оба вызова — и шаг job'а

```yaml
- name: Coverage floor of the core packages
  run: scripts/coverage-gate.sh 60 internal/state …
```

и рецепт `Makefile`

```make
test:
	@go test -short -race -count=1 -coverprofile=coverage.out ./...
	scripts/coverage-gate.sh 60 internal/state …
```

запускают файл **как программу**, а не через интерпретатор. `actions/checkout` восстанавливает
режим из дерева, и на `ubuntu-latest` шаг завершается `Permission denied` (exit 126).
Локально этого не видно: Git for Windows игнорирует бит исполнения, а Git Bash запускает файл
по shebang. Воспроизведено в контейнере `bash:5` на файле из индекса (`git show :scripts/…`).
Следствие: DoD задачи («PR → все шесть job'ов зелёные») и общий DoD §1 п. 8 недостижимы,
а `make test` / `make ci` сломаны на Linux и в WSL.
**Как исправить**: `git update-index --chmod=+x scripts/coverage-gate.sh` (бит хранится в дереве
и от `core.fileMode` рабочей копии не зависит). Для страховки — вызывать
`bash scripts/coverage-gate.sh …` в обоих местах; шаг `The negative fixtures stay rejected` в
job `compose-lint` вызывает `scripts/compose-lint.sh` так же и работает только потому, что тот
файл `100755`. Заодно стоит завести это в критерий: `git ls-files -s scripts/*.sh` — все `100755`.

#### Major

**M-2. Решение оркестратора о сужении `push` не внесено.** `journal.md`, 2026-09-10, «Решения
оркестратора по ОВ T-012», п. 1: «сузить триггер `push` до `main` и `integration/**` (ветки
эпиков покрываются триггером `pull_request`) … **внести в этой же задаче**». В
`.github/workflows/go.yml` осталось:

```yaml
on:
  push:
    branches: [main, 'integration/**', 'epic/**', feature/agent-gm-core]
```

Группы `concurrency` у `push` (`refs/heads/epic/…`) и у `pull_request` (`refs/pull/N/merge`)
разные, поэтому один и тот же коммит считается дважды: ~20 мин вместо ~10 при лимите
2 000 мин/мес (U-9, §12). Дев-лог §4 п. 3 и §8 п. 1 честно фиксирует, что сделано «по документу»
и что решение за оркестратором — но решение уже принято и относится к этой задаче.
**Как исправить**: оставить в `push.branches` только `main` и `'integration/**'`; `epic/**` и
`feature/agent-gm-core` убрать из `push`, в `pull_request.branches` оставить как есть (там они
целевые ветки PR, а не исходные, — сужать не нужно). Фильтр `paths` только на `push` (п. 2
решения) — **внесён, замечаний нет**, обоснование в комментарии файла корректное: `paths` с
отрицаниями вместо `paths-ignore` — единственный способ вернуть `blueprints/**/*.md`, и порядок
шаблонов («последний выигрывает») соблюдён.

**M-3. Job `security` падает на каждом `pull_request` из-за отсутствующего права
`pull-requests`.** У job'а объявлено

```yaml
permissions:
  contents: read
  security-events: write
```

Объявление блока обнуляет все неперечисленные права, то есть `pull-requests: none`. Разобрал
`dist/index.js` действия по запинованному SHA `e0c47f4f…`: функция `ScanPullRequest` **до**
сканирования и **без** try/catch делает

```js
let commits = await octokit.request(
  "GET /repos/{owner}/{repo}/pulls/{pull_number}/commits", …);
```

Эндпоинт pull request'ов приватного репозитория требует прав «Pull requests: read»; при `none`
это 403, необработанный reject и падение шага — на каждом PR, то есть ровно в сценарии DoD.
Отдельно: `GITLEAKS_ENABLE_COMMENTS` в шаге не выставлен, а по умолчанию `true`, и при находках
действие пишет review-комментарии (`pulls.createReviewComment`) — для этого нужен уже
`pull-requests: write`; сам вызов обёрнут в `try/catch` с `core.warning`, но предшествующий ему
`GET …/pulls/{pull_number}/comments` — нет.
**Как исправить**: добавить job'у `pull-requests: read` и выставить в шаге
`GITLEAKS_ENABLE_COMMENTS: 'false'` (комментарии на PR при одном человеке-владельце ничего не
дают, а требование `write` в job'е с секретами — лишнее расширение прав). Проверить на первом
реальном PR: чек должен быть красным из-за находки, а не из-за 403.

**M-4. Новый `[[allowlists]]` в `.gitleaks.toml` — ослабление вопреки прямому правилу, и оно
не нужно.** `infrastructure.md` §4.5 п. 1 (и §0 п. 6, §11 строка T-16, threat-model T-16):
«allowlist **только** `*.example`… Точечные ложные срабатывания — inline `# gitleaks:allow`
или fingerprint в `.gitleaksignore`, **не расширение allowlist**». Добавлен третий глобальный
allowlist с `regexTarget = "secret"`, действующий на **все** правила и **все** файлы.
При этом fingerprint именно этой находки **уже есть** в `.gitleaksignore` (две строки:
`…:swarm-llm-laws.md:generic-api-key:504` в форме с коммитом и без).

Проверил, почему он не срабатывает. Шаг job'а передаёт gitleaks **абсолютный** путь:

```yaml
run: |
  docker run --rm -v "$PWD:/repo:ro" -w /repo \
    "zricethezav/gitleaks:${GITLEAKS_VERSION}" \
    dir --no-banner --redact --config=/repo/.gitleaks.toml /repo
```

и fingerprint находки становится `/repo/Docs/dev-team/architecture/…:generic-api-key:504`,
то есть с относительной строкой в `.gitleaksignore` не совпадает. Прогон на содержимом индекса
(gitleaks v8.30.1, allowlist T-012 вырезан):

- `gitleaks dir --config=<abs>/.gitleaks.toml <abs>` → **leaks found: 1**;
- `cd <abs> && gitleaks dir --config=.gitleaks.toml .` → **no leaks found**.

То есть достаточно относительного пути — и allowlist не нужен вовсе.
**Как исправить**: в шаге заменить аргументы на `--config=.gitleaks.toml .` (рабочий каталог
контейнера уже `/repo`), а блок allowlist из `.gitleaks.toml` убрать; `.gitleaks.toml` вернётся
к состоянию, которое задаёт F-1. Если по какой-то причине абсолютный путь нужен — вести
исключение в `.gitleaksignore` вторым fingerprint'ом, а не глобальным allowlist. Само правило
узкое (значение целиком — имя переменной проекта), реальный `sk-…` и токен бота им не гасятся —
это я проверил, — но T-16 запрещает не широту, а сам способ.

#### Minor

**Mi-1. `security-events: write` не используется ни одним шагом.** Комментарий в шапке файла
говорит «only `security` asks for more, and only to upload SARIF», но SARIF никуда не
выгружается: `gitleaks-action` пишет `results.sarif` в **артефакт workflow**
(`artifactClient.uploadArtifact`), и артефакт к тому же отключён
(`GITLEAKS_ENABLE_UPLOAD_ARTIFACT: 'false'`); обращений к code-scanning API в коде действия нет
(проверил — все вхождения `code-scanning` в бандле принадлежат карте эндпоинтов octokit, ни
одного в коде самого действия). `govulncheck-action` по `action.yml` печатает текст и ничего не
загружает. Право выдано вхолостую и противоречит правилу минимальности, объявленному в этом же
файле. **Как исправить**: либо убрать `security-events: write`, либо добавить настоящий шаг
`github/codeql-action/upload-sarif` (тогда придётся вернуть `GITLEAKS_ENABLE_UPLOAD_ARTIFACT`
и следить, чтобы `--redact` оставался — он там есть).

**Mi-2. Job `integration` не передаёт `MINIO_COMMIT`.** В `build/versions.env` пин
`MINIO_COMMIT=9e49d5e7a648`, в `build/minio.Dockerfile` он объявлен как `ARG MINIO_COMMIT=…`
со значением по умолчанию и fail-closed проверкой `git rev-parse` (замечание Mi-5 ревью T-004).
`build-args` в шаге перечисляет только `MINIO_TAG`, `MINIO_REPO`, `MINIO_BUILDER_IMAGE`. Пока
дефолт в Dockerfile совпадает с `versions.env` — совпадает; после первого же обновления пина
только в `versions.env` CI молча соберёт другой коммит, а `versions.env` перестанет быть
источником истины (NFR-071, ровно то, ради чего шаг «Read the pinned versions» и существует).
**Как исправить**: добавить строку `MINIO_COMMIT=${{ env.MINIO_COMMIT }}` в `build-args`.

**Mi-3. Сканер `privacy` не видит каноническую форму Telegram-апдейта.** Правило
`external_id` требует ключ из списка **непосредственно** перед числом (`key: 123`), поэтому
вложенные формы пропускаются. Зонд (воспроизводится):

```
{"from":{"id":482913776,"username":"vasya"}}   → поймано только username
{"chat":{"id":482913776}}                      → не поймано
{"user":{"id":"482913776"}}                    → не поймано
{"id":482913776}                               → не поймано
update_id: 482913776                           → не поймано
```

Именно так выглядит `Update` в Telegram Bot API, и именно записи сессий (`testdata/recordings/`,
T-018 и далее) — то, ради чего job и заведён (SEC-01, T-06/T-08, ADR-010 доп. п. 1). Сегодня
пропуск безвреден — записей ещё нет; к моменту их появления шлюз уже будет считаться закрытым.
Отдельно: `update_id` есть в `shared/logging.sensitiveKeys` (решение по Mi-3 ревью T-007), а в
сканере его нет.
**Как исправить** (дёшево, в пределах правила): добавить `update_id` в альтернативу ключей и
одно правило на вложенную форму, например
`(?is)"(?:from|chat|user|sender)"\s*:\s*\{[^{}]*?"id"\s*:\s*"?(\d{5,15})`. Полный структурный
разбор JSON/JSONL — законно в T-139 (EPIC-005), здесь не требую.

**Mi-4. Расхождение с `shared/logging` при заявленной идентичности правил.** Комментарий
`scanner.go` говорит «the same rules as the NFR-041 test», но:

- токен бота: сканер `\b\d{8,10}:…`, `shared/logging` `\b(?:bot)?\d{6,}:…`. Проверено — сканер
  **не видит** ни `4829137:AAH…` (7 цифр), ни `12345678901:AAH…` (11 цифр); логгер видит оба.
  Идентификаторы ботов уже бывают 10-значными и растут;
- ключ провайдера: сканер `sk-…{16,}`, логгер `sk-…{8,}` — `sk-abcdefgh12345678` поймано,
  `sk-abcdefgh` нет;
- список чувствительных ключей продублирован третий раз (схемы, `shared/logging`, сканер) и уже
  разошёлся (`update_id`, `text`).

**Как исправить**: либо взять один экспортируемый источник ключей (в `shared/logging` он уже
есть и покрыт тестами — это тот самый «переиспользуемый код»), либо выровнять диапазоны
(`\d{6,}:`, `sk-…{8,}`) и снять из комментария утверждение о тождественности правил.

**Mi-5. Решение ОВ-30 не выполнено и не вынесено в открытые вопросы.** `journal.md`,
2026-09-10: «ОВ-30: сведение переменных версии (`main.version` в Dockerfile vs
`logging.Version`) — **в T-012 (CI)** добавить второй `-X` либо свести к одной».
`Makefile` уже штампует обе (`LDFLAGS := -X main.version=… -X multiverse-core.io/shared/logging.Version=…`),
а `build/Dockerfile` — только `-X main.version=${VERSION}`; job `image` передаёт `VERSION`, и в
образе `logging.Version` остаётся `dev`, то есть каждая строка лога контейнера уезжает с
`version=dev` (NFR-041, §7.1). В §8 dev-log этого пункта нет.
**Как исправить**: добавить второй `-X` в `build/Dockerfile` (файл T-004, `build/**` — владение
EPIC-001, но правка общего файла — через tech-lead#1) либо зафиксировать перенос решения в
`dev-log.md` явным ОВ.

**Mi-6. `ci-harness` в `MV_CORE_ADMIN_CLIENTS` не добавлен.** `journal.md`, 2026-09-09 (проверка
итерации 2 T-003): «Умолчание `MV_CORE_ADMIN_CLIENTS=operator` — fail-closed: добавить
`ci-harness` (**T-012**) и `mvctl` (T-010) в их DoD». `mvctl` внесён (`shared/env/vars.go`:
`operator,mvctl`), `ci-harness` — нигде: `.github/ci.env` строка 60 = `operator`,
`.env.example` = `operator,mvctl`. В `dev-log.md` пункт не упомянут.
**Как исправить**: определить место (значение по умолчанию в `shared/env/vars.go` — контрактный
файл, через system-architect; либо `MV_CORE_ADMIN_CLIENTS` в окружении job `e2e`, что дешевле и
локально к T-012) и зафиксировать выбор в dev-log. Пока сценарии e2e не существуют (T-018),
это не ломает ничего — но обязательство висит на этой задаче.

**Mi-7. `make secrets-scan` по-прежнему сканирует рабочую копию.** Решение оркестратора от
2026-09-10, п. 4: «`make secrets-scan` должен сканировать содержимое индекса, а не рабочую копию
(ОВ-8) — доделать здесь же, если дёшево, иначе в T-019». Цель не менялась
(`gitleaks dir --no-banner --redact .`), и на машине владельца она красная (6 находок в
неотслеживаемых `.env`, `build/.legacy-src/`, `.claude/worktrees/`) — то есть общий DoD §1 п. 4
формально не показывается зелёным ни на одной задаче. Это дёшево и это тот же корень, что M-4:
`git checkout-index -a --prefix=<tmp>/ && (cd <tmp> && gitleaks dir --config=… .)` — ровно то,
чем я проверял M-4, и заодно относительный путь чинит `.gitleaksignore`.
**Как исправить**: либо доделать здесь (`Makefile` — общий файл, через tech-lead#1), либо
явно перенести в T-019 записью в dev-log; молча оставлять нельзя, п. 4 решения адресован задаче.

**Mi-8. `coverage-gate.sh` не отличает отсутствующий пакет от несобирающегося.**

```bash
if ! go list "./${pkg}/..." >/dev/null 2>&1; then
  printf 'coverage-gate: %s does not exist yet — skipped\n' "$pkg"
  continue
fi
```

`go list` возвращает ненулевой код и когда каталога нет, и когда пакет не компилируется, и
когда сломан сам модуль. Сообщение в обоих случаях — «does not exist yet», а код выхода — 0.
В job `unit` это прикрыто предшествующим `go build ./...`, но `make test` вызывается и сам по
себе, и там порог молча выключится на сломанном пакете.
**Как исправить**: проверять наличие каталога отдельно (`[ -d "$pkg" ] || { warn; continue; }`),
а ненулевой `go list` при существующем каталоге считать ошибкой (`exit 2`).

**Mi-9. `privacy scan` смешивает «нашёл» и «не смог прочитать».** В `runScan` любая ошибка
обхода, кроме `fs.ErrNotExist`, отображается в `cli.ExitFindings` (1) — тот же код, что и
«найдены идентификаторы». Собственный doc-комментарий пакета обещает «0 / 1 (находки) / 2
(ошибка)». CI по коду выхода не отличит утечку от нечитаемого файла, а `Scan` при ошибке
возвращает **частичный** результат (обход прерывается на первой ошибке), то есть «1 находка»
может означать «дальше не смотрели».
**Как исправить**: для ошибок ввода-вывода вернуть отдельный код ошибки и сказать в тексте, что
скан неполный; либо не прерывать обход, а копить нечитаемые пути в `Result.Skipped` и падать в
конце с явным сообщением.

#### Nit

**N-1.** `GITLEAKS_CONFIG: .gitleaks.toml` в шаге `gitleaks-action` действием **не читается**:
в бандле v3.0.0 есть только `GITLEAKS_VERSION`, `GITLEAKS_LICENSE`, `GITLEAKS_ENABLE_COMMENTS`,
`GITLEAKS_ENABLE_SUMMARY`, `GITLEAKS_ENABLE_UPLOAD_ARTIFACT`, `GITLEAKS_NOTIFY_USER_LIST`,
`GITLEAKS_ERROR`. Работает оно только потому, что переменную понимает сам бинарь gitleaks (и
`.gitleaks.toml` в корне он нашёл бы и без неё). Вреда нет; стоит поправить комментарий, чтобы
следующий читатель не искал вход, которого у действия нет.

**N-2.** `govulncheck-action` внутри делает `go install golang.org/x/vuln/cmd/govulncheck@latest`
и вызывает `setup-go` с `go-version-input: 'stable'` **вместе** с переданным `go-version-file`;
setup-go в таком сочетании берёт `go-version` и игнорирует файл. То есть шаг идёт не под
запинованным тулчейном, а единственный инструмент во всём workflow остаётся без пина — против
§2.4 (`golang.org/x/vuln v1.8.0`) и против правила «никаких версий помимо `versions.env`».
В dev-log §9 это записано как допущение — верно; как альтернатива на будущее:
`go run golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION} ./...` вместо действия.

**N-3.** Шаг `docker compose … config -q` в job `compose-lint` дублирует то, что
`scripts/compose-lint.sh` делает сам теми же `--env-file` (`COMPOSE_LINT_ENV_FILES` по умолчанию
`build/versions.env .github/ci.env`). Быстрый fail полезен, но это второе место, где список
env-файлов надо держать синхронным.

**N-4.** Строка `/services/_archive/` без владельца в `CODEOWNERS` — приём рабочий, но
синтаксический контроль GitHub на такие строки исторически ругался. Проверить после первого
push в Settings → Code owners (там показываются ошибки файла); если не примет — заменить
комментарием.

**N-5.** `TestScanTheShippedTestdata` сканирует `testdata/` репозитория, то есть привязывает
unit-тесты пакета `privacy` к файлам чужих задач (сейчас — `testdata/bench/prompts.jsonl`,
T-013). Замысел («фикстура из другой задачи краснит job `unit` своего же PR») правильный, но
следствие — красный `unit` у задачи, которая пакета не касалась. Стоит упомянуть в
migration-guide (T-019), чтобы находка читалась однозначно.

**N-6.** `dependabot.yml`, экосистема `docker` с `directory: /build` рассчитывает на то, что
Dependabot подберёт `Dockerfile`, `minio.Dockerfile` и `legacy.Dockerfile`. Проверить на первом
прогоне, что взяты все три (в логе Dependabot видно, какие файлы разобраны).

### Что проверено отдельно и замечаний не вызвало

- **Полнота состава job'ов против §3.1**: шесть блокирующих + `image` — на месте, имена совпадают
  с required checks §3.2 дословно, `timeout-minutes` явные у всех семи, `runs-on: ubuntu-latest`
  везде. Два отсутствующих шага job `contracts` (`mvctl blueprint validate blueprints/` и
  `api/*.openapi.yaml`) — законные: первый по решению ОВ-48 возвращается в EPIC-003 (A4/T-204),
  второй — в EPIC-004; оба зафиксированы комментарием прямо в job'е и в `Makefile`. Каталога
  `blueprints/` в дереве ещё нет.
- **Удаление `validate-blueprints.yml`** ничего не теряет: он гонял `go test ./shared/agent/...`
  (в том числе `TestBlueprint*`), а эти тесты живут и запускаются job'ом `unit` через
  `go test -short ./...` — `testing.Short()`/`t.Skip` в `shared/agent/*_test.go` нет, проверил;
  остальное в нём ссылалось на `configs/gm_*.yaml` (архив, T-002), `go-version: '1.24'` и
  `go mod tidy` в CI (прямо запрещён T-35). Кто вернёт проверку блупринтов: строка
  `mvctl blueprint validate blueprints/` в job `contracts` — **EPIC-003, A4/T-204** (ОВ-48).
- **Секреты в логах**: `gitleaks-action` запускает бинарь с `--redact` (видно в бандле),
  шаг `gitleaks dir` — тоже; `privacy scan` печатает только `Redact` (два символа + длина),
  на что есть тест в обеих формах вывода; `Result.Findings` помечено `json:"-"` и в
  `details` не попадает — проверил на реальном выводе `--json`.
- **Запись в репозиторий из job'ов**: нет ни одной; `push: false` у обоих `build-push-action`,
  `go mod tidy -diff` вместо `tidy`, `pull_request_target` отсутствует.
- **`concurrency`** объявлен на workflow, группы `qwen-*.yml` с `ci-${{ github.ref }}` не
  пересекаются (проверил).
- **`continue-on-error`** нет нигде; `govulncheck` блокирующий — условие ADR-010 доп. п. 5
  выполнено (`go.mod`: `go 1.26`, `toolchain go1.26.8`), снятие обосновано.
- **Кэш**: `setup-go` с `cache: true` в четырёх job'ах, `type=gha` у обеих сборок образов,
  отдельного `actions/cache` нет — верно, он был бы вторым кэшем того же каталога.
- **Шаг «Read the pinned versions»**: замена `cat` на `grep -E '^[A-Z][A-Z0-9_]*='` обязательна
  (раннер падает на строках-комментариях `$GITHUB_ENV`), смысл шага сохранён.
- **`CODEOWNERS`**: `* @alekseizabelin1985-spec` плюс все девять путей DoD (`blueprints`, `laws`,
  `config`, `schemas`, `shared`, `build`, `.github`, `docker-compose.yml`, `Makefile`) — совпадает
  со списком §3.2 построчно.
- **`dependabot.yml`** против §2.4: `gomod` еженедельно с группировкой minor/patch,
  `github-actions` еженедельно (обновляет SHA-пины), `docker` ежемесячно по `/build`,
  оговорка про невидимый Dependabot'у `versions.env` — всё на месте; `reviewers:` не используется
  (поле устарело в пользу CODEOWNERS) — верно.
- **Порог покрытия**: считается по профилю, а не по `go tool cover -func` — обоснование в шапке
  скрипта корректное (невзвешенное среднее по функциям ≠ покрытие пакета); арифметика проверена
  на реальном профиле (`cmd/mvctl/internal` 475/513 = 92,6 %). Отдельно проверил главный риск
  такого подхода: пакеты **без тестов** в профиле присутствуют, значит новый контекст без единого
  теста даст 0 %, а не «no statements — skipped».
- **`Makefile`**: `.ONESHELL` + `@` на первой строке рецепта гасит эхо всего рецепта, поэтому
  добавленные строки в `test` не шумят; `privacy-scan` включён в `ci` — верно.
- **Границы владения**: тронуты только файлы T-012 плюс общие `cmd/mvctl/main.go`,
  `cmd/mvctl/main_test.go`, `Makefile`, `.gitleaks.toml`. Все они — «только через tech-lead#1»
  (`tasks.md` §0); developer#1 это отметил (§8 п. 5 dev-log) — нужна явная отметка тимлида при
  приёмке. Файлы T-013 не затронуты (принято по dev-log, из индекса авторство не различимо).
- **Тесты `privacy`**: покрывают обе стороны таблицы (что ловим / что не ловим), место находки,
  бинарные файлы, отсутствующий корень, коды выхода, машиночитаемый JSON и нераскрытие значения;
  тавтологии не нашёл, детерминированы (два прогона `-count=1` идентичны), внешних зависимостей
  и таймингов нет.

### DoD по пунктам

| Пункт | Статус |
|---|---|
| Общий §1.1 `make lint`, `gofmt` | **выполнен** (`golangci-lint run ./...` — 0 issues; `gofmt -l` пусто) |
| Общий §1.2 build/vet/test | **выполнен**, кроме `-race` (нет gcc на машине; решение ОВ-5 — только в CI) |
| Общий §1.3 тесты в той же задаче | **выполнен** (`privacy` 86,8 %; у `coverage-gate.sh` собственных тестов нет — проверен шестью сценариями вручную, для bash приемлемо) |
| Общий §1.4 `make secrets-scan` 0 находок | **не выполнен**: цель красная на рабочей копии (Mi-7); `gitleaks git --staged` и `gitleaks dir` по содержимому индекса — 0 |
| Общий §1.5 `dev-log.md` | **выполнен**, качество высокое; не хватает ОВ по Mi-5 и Mi-6 |
| Общий §1.6 поведение по критериям | **выполнен** в части, проверяемой локально |
| Общий §1.7 карта владения / контракты | **выполнен с оговоркой**: четыре общих файла — через tech-lead#1, отметка нужна при приёмке |
| Общий §1.8 CI зелёный на PR | **не выполнен** — C-1 (и M-3) не дают job'ам стать зелёными |
| T-012: шесть job'ов зелёные ≤ 10 мин | **не выполнен** (C-1, M-3); оценка ≤ 10 мин правдоподобна, но проверяема только на GitHub |
| T-012: второй прогон `integration` из кэша `type=gha` ≤ 1 мин | **не проверяемо здесь** (push нельзя); конфигурация кэша корректная |
| T-012: `CODEOWNERS` покрывает девять путей | **выполнен** |
| T-012: branch protection + скриншот/лог | **не выполнен и не может быть**: настройки репозитория — действие пользователя (решение оркестратора п. 5); инструкции написаны в §7 dev-log и продублированы комментарием в `CODEOWNERS` — это правильное закрытие пункта |

### Предложения в бэклог

1. **T-35 остаётся открытым.** `qwen-*.yml` (5 файлов) по-прежнему используют
   `QwenLM/qwen-code-action@v1  # ratchet:exclude` без пина по SHA и с
   `issues: write` / `pull-requests: write` на тексте issue/PR (EP-10 в threat-model). §3.2 прямо
   запрещает трогать их в T-012, поэтому это не замечание — но SEC-25 закрыт не полностью, и
   пункт «CI зелёный; ревью workflow» в SEC-25 честнее считать частичным. Нужна отдельная задача:
   пин по SHA (у действия его может не быть — тогда форк или удаление workflow), аудит условия
   `author_association`, сужение прав.
2. **`.gitattributes`: `go.mod text eol=lf`, `go.sum text eol=lf`** (предложение самого
   разработчика, §8 п. 3) — без этого `go mod tidy -diff` на Windows всегда «грязный», и локальный
   эквивалент job `unit` непроверяем. Файл F-1, отдельная задача.
3. **Критерий «все `scripts/*.sh` — `100755` в индексе»** в общий DoD §1 или в pre-commit:
   C-1 — второй по счёту случай, когда Windows-разработка расходится с Linux-раннером (первый —
   CRLF в `go.sum`). Дешёвая проверка: `git ls-files -s scripts | grep -v ^100755`.
4. **Структурный разбор JSON/JSONL в `privacy scan`** и единый источник списка чувствительных
   ключей (`shared/logging`) — EPIC-005 T-139, если Mi-3/Mi-4 закрываются здесь только
   регулярками.
5. **Генерация `redpanda-init.sh` из `contracts topics --format=rpk`** (ОВ-49) — остаётся в
   бэклоге devops, в T-012 обоснованно не делалось.
6. **hadolint: понизить порог до `warning`** после правки `build/minio.Dockerfile`
   (DL4006/DL3062/DL3025/DL3066) — уже направлено оркестратором в T-019; отмечаю, что сегодняшний
   `failure-threshold: error` — обоснованный компромисс, а не послабление: hadolint остаётся
   блокирующим, просто на другом пороге.

### Риски и допущения

- **Реальный прогон CI недоступен** (push нельзя): всё оценивалось статически и локальной
  эмуляцией шагов. C-1 воспроизведён в Linux-контейнере на файле **из индекса** — это
  доказательство, а не гипотеза. M-3 выведен чтением запинованного `dist/index.js` действия и
  модели прав GitHub App; на 403 он подтвердится только первым реальным PR, поэтому формулировка
  замечания содержит и второй, независимый повод к правке (`GITLEAKS_ENABLE_COMMENTS`).
- **`-race` в окружении недоступен** (нет gcc). Гонок в коде задачи не искал прогоном: пакет
  `privacy` без горутин, каналов и изменяемого глобального состояния; `rules`,
  `placeholderWords`, `fixtureNames` — глобальные, но только читаются.
- **Поведение `gitleaks-action` v3 на приватном личном репозитории без `GITLEAKS_LICENSE`**
  проверяемо только на GitHub. Код действия определяет тип владельца через `GET /users/{username}`
  и для `User` пропускает валидацию лицензии — допущение U-9 подтверждается чтением бандла.
- **Пины действий сверены на 2026-09-09**; SHA неизменны по определению, но сами теги
  мутабельны — если тег переставят, сверка «SHA ↔ версия в комментарии» разъедется. Дальше их
  ведёт Dependabot, и это правильно.
- **Оценка «≤ 10 мин»** относится к параллельным job'ам с прогретым кэшем; первый прогон
  `integration` дольше на сборку MinIO. Проверить после первого зелёного прогона.
- **Авторство файлов T-013 в индексе неразличимо**: то, что developer#1 их не трогал, принято по
  `dev-log.md`.
- Кроме этой записи в `review.md` ничего не правил и не коммитил; временные файлы зонда —
  в каталоге скретчпада, рабочее дерево не изменялось.

---

## T-014 · ревью #1 · 2026-09-09 · code-reviewer#2 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `fd93a6a`, ревьюировались **изменения в индексе**.
В область входят только файлы T-014:

| Действие | Путь |
|---|---|
| A | `shared/testkit/{testkit.go,versions.go,containers.go,testkit_test.go,versions_test.go}` |
| A | `shared/testkit/membus/{membus.go,membus_test.go}` |
| A | `shared/testkit/contract/{contract.go,membus_test.go,redpanda_integration_test.go}` |
| M | `shared/eventbus/kafka.go` (обрезка `ReadRange` по `End`; `closeReader` в `Subscribe`) |
| M | `go.mod` (`github.com/moby/moby/api` indirect → direct) |
| M | `Docs/dev-team/epics/EPIC-001-foundation/dev-log.md` (запись T-014) |

Файлы T-012 (`.github/**`, `cmd/mvctl/internal/privacy/**`, `scripts/coverage-gate.sh`,
`Makefile`, `.gitleaks.toml`, `shared/env/vars.go`, `build/Dockerfile`) по указанию оркестратора
не ревьюировались — по ним идёт итерация 2 другого разработчика.

Основание: `tasks.md` §1 (общий DoD) и §6 (T-014, критерий ворот), `dev-log.md` (запись
developer#2 · T-014), `contracts.md` v0.4 C-01 v1.1, `components/foundation.md` v0.2 §5.3 и §9,
ADR-007 п. 4–6, ADR-010 п. 4 и п. 5, `journal.md` (решения оркестратора по ОВ T-005 и T-014:
retry ×3 = 4 вызова и `Attempts=4`; `Subscribe` при штатной остановке возвращает `nil`;
`SkipValidateOnRead` fail-closed; мягкий режим `membus` принят).

### Вердикт

**Вернуть.** Critical: 0 · Major: 2 · Minor: 7 · Nit: 5.

Сделанное — сильная работа: набор из 15 проверок реально держит контракт (проверено пятью
мутациями, см. ниже), три вскрытых расхождения починены по существу, а не подогнаны, правки
`kafka.go` корректны, и обе закрыты падающими проверками. Но **ворота волны 1 закрытыми считать
пока нельзя**: заглушка сегодня расходится с адаптером в отмене подписки при непустом хвосте
(Major-1, расхождение №4 — воспроизведено сравнительным замером на обеих реализациях), а
`Bus.Close()` — третий метод интерфейса C-01 и та самая семантика, из-за которой чинилось
расхождение №3, — набором не проверяется вообще (Major-2, доказано мутацией). Объём итерации 2
небольшой: две правки в `membus.go` (по 1–3 строки), одна ручка в `Target` и два-три новых кейса.
Уже проверенная часть контракта переделки не требует.

### Что проверено прогоном (машина владельца, `GOFLAGS=-buildvcs=false`, Docker Desktop 29.6.1)

| Проверка | Результат |
|---|---|
| `go build ./... && go vet ./...` | зелёные |
| `go test -short -count=1 ./...` | зелёные, 24 пакета `ok` |
| `go test -tags integration -count=1 ./shared/testkit/... ./shared/eventbus/...` | **три прогона подряд** зелёные (17,2 с / 16,6 с / 12,4 с), флака нет |
| contract на Redpanda (подробно) | 15/15 `PASS`, весь набор 15,5 с; `PublishOfOneEventIsNotBatched` — 3,0 мс при бюджете 300 мс |
| Покрытие | `testkit` 74,0 % (в `dev-log` 81,1 % — с тегом `integration`; `containers.go` в unit-режиме не компилируется), `membus` 83,6 %, `contract` 81,3 %, `eventbus` 75,2 % — все ≥ 60 % |
| `golangci-lint run ./...` (v2.13.2) и `--build-tags=integration,e2e` | 0 issues (теги `integration`/`e2e` и так стоят в `.golangci.yml` `run.build-tags`) |
| `gofmt -l shared/testkit shared/eventbus` | пусто |
| `go mod tidy` | диффа `go.mod`/`go.sum` нет — перевод `moby/moby/api` в direct совпадает с тем, что считает верным `tidy` |
| `gitleaks git --staged --redact .` | `no leaks found` |
| `-race` | не запускался (недоступен в окружении; общий DoD §1 п. 2 — только в CI job `unit`) |

### Мутационная проверка (файлы восстановлены, md5 сверены с резервными копиями)

| # | Мутация | Ожидание | Факт |
|---|---|---|---|
| 1 | `membus.go`: курсор группы не сохраняется между подписками (`b.group` всегда отдаёт новый) | падение | `AGroupResumesFromItsCursor` — «received "before", want "after"» |
| 2 | `membus.go`: `delivery()` собирается с `SkipValidateOnRead: true` | падение | `InvalidOnReadGoesToDeadLetters` (таймаут ожидания DLQ) и `UndecodableMessageGoesToDeadLetters` (обработчик вызван 1 раз вместо 0) |
| 3 | `eventbus/delivery.go`: `attempt <= len(backoff)` → `attempt < len(backoff)` | падение | `RetriesThenDeadLetter` — «called 3 times, want 4» (ловится и на `membus`, т.е. правила действительно общие) |
| 4 | `kafka.go`: снята обрезка `to` по `End` | падение на Redpanda | `JournalStopsAtTheEndOfTheJournal` — 60,05 с и «waited for the context to expire» |
| 5 | `kafka.go`: `defer k.closeReader(reader)` → `defer k.untrack(reader)` | падение/флак на Redpanda | `AGroupResumesFromItsCursor` — таймаут 60 с на ожидании 1 события |
| 6 | `membus.go`: `case <-b.done: return nil` → `return eventbus.ErrClosed` (обе точки) | **должно падать, но не падает** | `TestBusContractOnMembus` зелёный (15/15); валится только локальный `membus_test.go:TestCloseReleasesReaders` → см. Major-2 |

Подгонки теста под реализацию не обнаружено: проверки 1–5 бьют по поведению, а не по сигнатурам,
и мутация в общем `eventbus/delivery.go` валит именно `membus`-половину — значит второго набора
правил в заглушке действительно нет.

### Замечания

**Major-1 · `shared/testkit/membus/membus.go:200-235` (`Subscribe`), `:280-310` (`Tail`),
`:245-272` (`ReadRange`) — заглушка не замечает отмену контекста, пока в топике есть непрочитанное.**
`ctx.Err()` проверяется только (а) когда читатель припаркован на конце лога (`:216`, `:293`) и
(б) когда доставка вернула ошибку (`:228`). `Delivery.Deliver` возвращает `ctx.Err()` лишь если
обработчик сам вернул ошибку, поэтому успешный обработчик означает: отменённая подписка
дочитывает **весь** хвост. Сравнительный замер (обе реализации, один и тот же зонд: 100 событий
в топике, обработчик 50 мс, `cancel()` через 1,5 с):

```
membus: calls at cancel = 30, after Subscribe returned = 100 (backlog 100), err = <nil>
kafka:  calls at cancel = 26, after Subscribe returned =  26 (backlog 100), err = <nil>
```

Это расхождение заглушки с адаптером ровно того класса, ради которого существуют ворота: тест
потребителя или харнесс, написанный по образцу «опубликовать N, подождать, отменить, проверить,
что обработано N», зелёный на `membus` и красный на брокере. `foundation.md` §9 требует от
`membus` воспроизводить семантику адаптера.
*Как исправить*: в начале каждой итерации цикла `Subscribe`/`Tail` — `if ctx.Err() != nil
{ return nil }`, в `ReadRange` — `if ctx.Err() != nil { return next, nil }`; и кейс в `contract.go`:
опубликовать заведомо больший хвост, чем успеет обработчик, отменить, убедиться, что после
возврата `Subscribe` число вызовов перестало расти и заметно меньше длины хвоста.

**Major-2 · `shared/testkit/contract/contract.go:88-107` — `Bus.Close()` не проверяется ни на
одной реализации.** `Close` — третий метод интерфейса `Bus` в C-01, его зовёт каждый контекст
платформы при остановке (`runtime.Deps`), и именно на его семантике сорвалось расхождение №3 из
`dev-log` («`Close` у membus подвешивал читателей и возвращал не тот результат»). Сегодня это
расхождение держит **только** локальный тест заглушки `membus/membus_test.go:171`
(`TestCloseReleasesReaders`) — то есть тест, который по определению не может сравнить заглушку с
брокером. Мутация 6 это доказывает: возврат `eventbus.ErrClosed` вместо `nil` оставляет весь
contract-набор зелёным. Для kafka цепочка `Close → FetchMessage → io.ErrClosedPipe → stopped →
nil` на живом брокере не проверяется вовсе — она обоснована чтением кода, но не прогоном.
*Как исправить*: добавить в `Target` необязательную ручку `Close func() error` (для redpanda —
`bus.Close`; кейс запускается последним, после него таргет не используется) и кейс: подписка
работает → `Close()` → `Subscribe` вернул `nil` → `Publish` вернул `ErrClosed`. Если делить один
брокер на все кейсы мешает — поднять для этого кейса вторую шину над тем же брокером
(`NewKafka` дёшев), второй контейнер не нужен.

**Minor-1 · `contract.go:770-773` — `stop()` молча выбрасывает то, что вернул `Subscribe`.**
Комментарий обещает «reports what Subscribe returned», код делает `<-s.done` и теряет значение.
Из-за этого любой отказ подписки (кроме кейса `SubscribeReturnsNilOnAnOrderlyStop`) виден не как
внятная ошибка, а как 60-секундный таймаут ожидания событий.
*Как исправить*: `stop(t *testing.T)` с `if err := <-s.done; err != nil { t.Errorf(...) }` —
заодно требование «штатная остановка возвращает nil» распространится на все 15 кейсов, а не на один.

**Minor-2 · перевыдача некоммитнутого события (`At-least-once` из «Гарантий» C-01) не
проверяется.** Ни один кейс не показывает, что событие, доставка которого прервана отменой
(`Delivery.Deliver` вернул `ctx.Err()`, офсет не коммитится / курсор группы не двигается), приходит
второй подписке той же группы. Механизмы у реализаций разные (`CommitMessages` против `g.next`), а
на этом держится восстановление контекста после падения.
*Как исправить*: кейс «обработчик виснет на первом событии → отмена → новая подписка той же
группы получает то же событие снова».

**Minor-3 · `--chaos=duplicate` в contract-тесте не используется.** DoD T-014 называет дубли
именно через флаг хаоса; в наборе (`contract.go:415-420`) дубль делается двойным `Publish` одного
и того же события. Наблюдаемо это почти то же самое, но ручки хаоса в `Target` нет вовсе, и на
kafka-таргете режим недостижим — то есть заявленные в C-01 «дубли по `--chaos=duplicate`»
проверены только юнит-тестом заглушки (`membus_test.go:TestChaosDuplicateDeliversTwice`).
*Как исправить*: либо ручка `Target.WithChaos`, либо явная строка в `dev-log`, что дубль
моделируется повторной публикацией, а флаг остаётся уделом unit-тестов.

**Minor-4 · отклонение от ADR-010 п. 4 / `foundation.md` §9 по хаосу не записано в `dev-log`.**
ADR-010 п. 4 требует `--chaos=duplicate` с переизданием **5 %** событий (NFR-013) и
`--chaos=reorder-topics` (проверка ожидания по `correlation_id`); реализовано
`Chaos{Duplicate bool}` (`membus.go:39-46`), `ReorderTopics` отсутствует. Детерминированный `bool`
для тестов, скорее всего, лучше 5 % случайности — но это отклонение, а раздел «Отклонения от
дизайна» `dev-log` §4 перечисляет три других и об этом молчит (общий DoD §1 п. 5).
*Как исправить*: записать отклонение и назвать задачу-потребителя для `ReorderTopics` (ожидание по
`correlation_id` понадобится EPIC-003/EPIC-005).

**Minor-5 · усечение большого тела в dead letter на живом брокере не проверяется.** Решение Mi-3
по ревью T-005 (`MaxDeadLetterRaw = 512 КиБ`, `RawTruncated`) существует ради транспорта: dead
letter больше `max.message.bytes` (1 МиБ по умолчанию) не запишется, офсет не закоммитится, и топик
встанет на том самом сообщении, ради которого DLQ и заведён. Юнит-тест
`eventbus/delivery_test.go:237` проверяет обрезку в памяти; на брокере — ничего.
*Как исправить*: кейс — `Append` тела ~600 КиБ в `analytics_events`, ожидание dead letter с
`RawTruncated=true`, затем проверка, что следующее событие того же топика обработано.

**Minor-6 · валидация при чтении проверена только во включённом положении.** Fail-closed по
умолчанию держится хорошо и не случайно: оба таргета строятся с нулевым `SkipValidateOnRead`
(`contract/membus_test.go:38`, `redpanda_integration_test.go:39`), и мутация 2 это подтверждает.
Но обратной стороны нет: с `SkipValidateOnRead=true` событие обязано дойти до обработчика, и
никто этого не проверяет — значит перепутанный знак при передаче `!MV_BUS_VALIDATE_ON_READ`
(её обязаны делать T-007/T-010/T-018) contract-тест не поймает.
*Как исправить*: кейс на второй шине с `SkipValidateOnRead=true`: невалидный payload доходит до
обработчика и в `dead_letters` не попадает.

**Minor-7 · два известных расхождения не покрыты и нигде не записаны.**
(а) *Несуществующий топик*: `membus` отдаёт `ErrNoTopic` (`membus.go:339`), kafka — ошибку дозвона
в `End` и молчаливое ожидание в `Subscribe`. (б) *Начало журнала*: у `membus` офсет 0 существует
всегда, у брокера с retention 30/90/180 дн. (`infrastructure.md` §10) `from` может оказаться
меньше log start offset — `ReadRange` тогда отдаст события с бо́льшими офсетами и вернёт
`next = msg.Offset+1`, не сообщив о пропуске, а `membus` в той же ситуации отдаст всё с `from`.
Для MVP-1 это не срочно (окно retention велико), но это ровно та ложь заглушки, которую ворота
должны ловить.
*Как исправить*: минимум — раздел «известные расхождения `membus` ↔ kafka» в `dev-log`/README
пакета; по (б) — заявка architect#1 при ревизии C-01 (поведение `ReadRange` левее начала журнала
контрактом не определено).

**Nit-1 · `membus.Records`/`DeadLetters` после `Close()` возвращают `ErrClosed`** (через
`topic()`, `membus.go:339`): естественный порядок «закрыть шину → посмотреть, что осталось в
`dead_letters`» не работает. Читающим методам проверка `closed` не нужна.

**Nit-2 · `testkit.Deterministic` (`testkit.go:55-64`) в `t.Cleanup` сбрасывает реестр
(`eventbus.SetRegistry(nil)`), которого не ставил.** Пакет, установивший реестр до вызова, тихо
его теряет. Либо не трогать, либо назвать это в доке функции.

**Nit-3 · `topic.at` (`membus.go:405`) отдаёт срез из лога без копии.** `Append` тело копирует,
`at` — нет; обработчик, изменивший `body`, испортит журнал. Сегодня безопасно (`Delivery`
разбирает байты в новый `Event`), но заглушкой пользуются чужие тесты.

**Nit-4 · `Retries` объявлен константой, а соответствие ему `Backoff` таргета не проверяется**
(`contract.go:32-36` — «Every target must be built with a backoff of this length» только
комментарием). Одна проверка длины через ручку таргета убрала бы возможность получить зелёный
набор при неверно настроенной шине.

**Nit-5 · `Timeout = 60 s` на каждый из 15 кейсов** (`contract.go:71`): красный прогон может
занять до 15 минут при бюджете CI ≤ 10 мин на все job'ы (ADR-010 п. 5). Стоит либо снизить до
20–30 с (сейчас самый долгий счастливый путь — 0,7 с), либо задать `-timeout` job'у `integration`.

### Отдельно проверено и признано верным

1. **Обрезка `ReadRange` по `End` (`kafka.go:184-210`).** Границы разобраны правильно: ранний
   выход `to <= from` **до** round-trip за `End`; повторная проверка `to <= from` после обрезки
   (пустой интервал и старт за концом журнала ничего не читают и не ошибаются); `from < 0` —
   ошибка. Гонка с растущим хвостом: `End` снимается один раз, события, пришедшие во время чтения,
   в этот вызов не попадают, и `ReadRange` честно возвращает `next < to` — семантика та же у
   `membus` (`t.length()` тоже снимается один раз), а слежение остаётся за `Tail`. Проверено
   мутацией 4.
2. **Закрытие reader'а (`kafka.go:150-158`, `closeReader:369`).** Двойное закрытие безопасно:
   `kafka.Reader.Close()` (kafka-go v0.4.51) держит `closed` под мьютексом, `close(r.msgs)` делает
   один раз и всегда возвращает `nil`, поэтому пара «`Kafka.Close()` закрыл + `defer closeReader`»
   не паникует и не портит агрегат ошибок. Отмена контекста и штатная остановка по-прежнему
   проходят через `stopped()` и дают `nil` (кейс 14 зелёный на брокере). Утечка горутин и членства
   в consumer group закрыта; мутация 5 подтверждает, что без правки набор краснеет.
3. **`membus` действительно переиспользует `Route`/`Delivery`**, второго набора правил нет:
   мутация 3 в общем `eventbus/delivery.go` валит `membus`-половину набора, мутация 2 показывает,
   что валидация при чтении идёт через ту же `Delivery`. Курсоры групп: ключ `topic\x00group`,
   курсор удерживается на время доставки (одна группа — одно событие за раз, как на топике из одной
   партиции; тест `TestOneGroupHandlesEachEventOnce`), при ошибке доставки не двигается.
4. **Мягкий режим `membus` без списка топиков** (`membus.go:109`) — принят оркестратором (решение
   по ОВ T-014 п. 5), явно описан в доке `Config.Topics` и закрыт двумя тестами
   (`TestUnknownTopicIsRefused`, `TestWithoutATopicListTopicsAppearOnFirstUse`); contract-тест
   всегда передаёт `contracts.Topics()`. Опасность ограничена тем, что режим включается только
   пустым списком, а `dead_letters` при непустом списке досоздаётся принудительно — правильное
   решение, иначе неудачная запись DLQ вешала бы топик.
5. **`Versions()`** (`versions.go`): один парсер с `.env.example` (`env.ParseExample`), поэтому
   расхождения правил комментирования и кавычек между compose/Makefile/тестами не будет; карта
   отдаётся копией; `Version` даёт `ErrNoVersion` и на отсутствующее имя, и на объявленное пустым
   (`CHROMA_IMAGE`) — верно, пустое имя образа хуже отсутствующего; `RepoRoot` идёт по
   `runtime.Caller` + `go.mod`, а не по `../..`. Устойчивость к формату проверена на текущем файле:
   строки-присваивания внутри комментариев (`#  (COMPOSE_ENV_FILES=.env,build/versions.env; …)`)
   отсекаются по `Entry.Commented`, пустой результат — ошибка. Тест-«часовой»
   (`versions_test.go:17`) сверяет результат со вторым, независимым парсером — не тавтология.
6. **`containers.go`**: образ только из пина, `auto_create_topics_enabled=false` как в compose,
   топики создаются явно с одной партицией, ожидание — не «порт слушает», а ответ на metadata,
   `Container()` отдан под `testcontainers.CleanupContainer`, ошибки старта завершают контейнер
   через `errors.Join(err, Terminate)`. Отклонение «только Redpanda» согласовано оркестратором.
7. **Офсето-относительная конструкция набора** (свой мир, своя группа, `End` как база в каждом
   кейсе) — то, что делает один файл пригодным и для чистой заглушки, и для брокера с историей
   предыдущих кейсов. Это правильное решение, а не упрощение.
8. **`go.mod`**: `moby/moby/api` используется только в файле с тегом `integration`
   (`containers.go`), перевод в direct совпадает с решением `go mod tidy` (диффа нет), версия не
   менялась, `go.sum` не трогался.

### DoD по пунктам

| Пункт | Результат |
|---|---|
| T-014: contract-тест на **обеих** реализациях — порядок | **выполнен** (кейс 1, `Position` там же) |
| T-014: at-least-once (дубли по `--chaos=duplicate`) | **выполнен по существу, но не по букве** — дубль моделируется повторной публикацией, флаг хаоса в наборе не используется (Minor-3); перевыдача некоммитнутого события не проверяется (Minor-2) |
| T-014: retry ×3 → `dead_letters` | **выполнен** (4 вызова, `Attempts=4`, `Error`, `FailedAt`, «топик не встал»), совпадает с решением оркестратора от 2026-09-09 |
| T-014: `ReadRange` строго по возрастанию | **выполнен** (без дыр, под-диапазон, пустой диапазон, остановка на конце журнала, старт за концом) |
| T-014: `End` монотонен | **выполнен** (+1 на событие, чтение не двигает) |
| T-014: `Tail` живёт до `ctx.Done()` | **выполнен** (офсеты по возрастанию, возврат `nil`) |
| T-014: `PositionFromContext` | **выполнен** (в `Subscribe`, `ReadRange`, `Tail`) |
| T-014: невалидное при чтении → DLQ без вызова handler | **выполнен** (плюс недекодируемое тело с `raw`); не закрыты обратная сторона флага (Minor-6) и усечение большого тела (Minor-5) |
| T-014: `make test-integration` локально | **выполнен** (эквивалент `go test -tags integration` — три зелёных прогона; сама цель Makefile — область T-012, не проверялась) |
| T-014: `testkit.Versions()` = пины файла, тест на расхождение | **выполнен** |
| T-014: **ворота** — зелёный contract-тест | **не выполнен полностью**: набор зелёный, но двух классов расхождений он не держит (Major-1 — расхождение существует прямо сейчас; Major-2 — `Close` не покрыт) |
| Общий §1.1 lint / `gofmt` | **выполнен** (0 issues, в том числе с тегами) |
| Общий §1.2 build / vet / test | **выполнен**, кроме `-race` (недоступен локально; в CI job `unit`) |
| Общий §1.3 тесты в той же задаче | **выполнен** (покрытие 74–84 % по пакетам) |
| Общий §1.4 gitleaks | **выполнен** (`git --staged --redact` — 0) |
| Общий §1.5 `dev-log.md` | **выполнен**, качество высокое (замеры, мутации, риски); не хватает отклонения по хаосу (Minor-4) и списка известных расхождений (Minor-7) |
| Общий §1.6 поведение по критериям | **выполнен** в проверяемой локально части |
| Общий §1.7 карта владения | **выполнен**: `shared/testkit/**` — EPIC-001; `shared/eventbus/**` — общий код, помечен `contract-change` (решение оркестратора по ОВ T-014 п. 2); добавление зависимости в `go.mod` разрешено «любой командой с пометкой в отчёте», и пометка есть |
| Общий §1.8 CI зелёный | вне области (job'ы — T-012, итерация 2) |

### Предложения в бэклог

1. **`depguard`-правило на `shared/testkit`.** Дока пакета утверждает «Nothing under `cmd/` or
   `internal/` may depend on it», `ownership.md` §1 говорит про исключение по файлу для
   `cmd/multiverse/fake_contexts.go`, но в `.golangci.yml` правила нет — утверждение не
   выполняется линтером. Владелец `.golangci.yml` (T-012/T-018).
2. **Вариант `ReadRange` с уже известным `End`** (ОВ-1 разработчика, принято оркестратором в
   бэклог EPIC-002) — подтверждаю: лишний round-trip заметен только при чтении журнала мелкими
   окнами в цикле.
3. **`End` без ctx-таймаута неотменяем**: `conn.ReadLastOffset()` идёт без дедлайна, если у
   контекста его нет (`kafka.go:246-258`); теперь этим свойством пользуется и `ReadRange`. Для
   вызывающих, отменяющих контекст без дедлайна, это потенциальное залипание на подвисшем брокере.
   Правка не в этой задаче — в ревизии C-01/`kafka.go`.
4. **Проверка задержки публикации — единственная перформанс-проверка внутри функционального
   набора.** Запас двадцатикратный (3,0 мс при 300 мс), но на загруженном раннере это первый
   кандидат на флак. Стоит либо вынести в `Benchmark`/отдельный тег, либо оставить `t.Logf` без
   утверждения, а бюджет держать в `ops/metrics`.
5. **Стартеры MinIO/Qdrant/Neo4j в `containers.go`** — по решению оркестратора их пишет
   задача-потребитель (EPIC-005); зафиксировать это в `foundation.md` §9, чтобы отклонение не
   всплывало на каждом ревью.

### Риски и допущения

- **`-race` не прогонялся** (недоступен в окружении). Разделяемое состояние заглушки закрыто
  мьютексами (`topic.mu`, `group.mu`, `Bus.mu`), `done` только закрывается; в contract-наборе общее
  состояние — `subscription.seen` под мьютексом и `atomic.Int64`. Слабое место — пакетные
  `SetIDSource`/`SetClock`/`SetRegistry`, которые `newRun` переустанавливает на каждый кейс, пока
  подписки предыдущих кейсов ещё не гарантированно завершены: кейсы останавливают подписки
  `defer sub.stop()`, но `stop()` не проверяет ошибку (Minor-1). Под `-race` в CI это стоит
  посмотреть первым.
- **Расхождение Major-1 воспроизведено зондом**, который я писал сам (временные `_test.go` в
  `shared/testkit/membus` и `shared/testkit/contract`, оба удалены; рабочее дерево сверено по
  `git status` и md5). Цифры выше — из одного прогона на этой машине; сам факт (100 против 26 из
  100) от тайминга не зависит.
- **Мутации 4 и 5 стоят по ~65 с каждая** (60 с — таймаут кейса) и прогонялись по одному разу: по
  мутации 5 `dev-log` сообщает о флаке (3,3 с → 60 с), у меня был устойчивый таймаут. Вывод
  «правка нужна» это подтверждает независимо от того, флак это или отказ.
- **Три зелёных прогона `integration` подряд** — достаточное, но не исчерпывающее свидетельство
  отсутствия флака: общий брокер на 15 кейсов и ожидания по опросу оставляют место для редких
  гонок на медленной машине. Первый прогон в CI стоит смотреть отдельно.
- **Область ревью ограничена файлами T-014**: `.github/**`, `Makefile`, `scripts/coverage-gate.sh`
  и прочее из T-012 не открывалось; выводы про `make test-integration` и job `integration` сделаны
  по эквивалентной команде `go test -tags integration`, а не по цели Makefile.
- Кроме этой записи в `review.md` ничего не правил и не коммитил. Мутации выполнялись поверх
  рабочей копии с резервными копиями в скретчпаде; после каждой файл восстанавливался, контрольные
  суммы `shared/testkit/membus/membus.go`, `shared/eventbus/kafka.go`, `shared/eventbus/delivery.go`
  совпадают с исходными.

---

## T-015 · ревью #1 · 2026-09-09 · code-reviewer#1 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, ревьюировались **изменения в индексе и рабочем дереве** по
файлам T-015:

| Действие | Путь |
|---|---|
| A | `internal/mechanics/{types,formula,rng,rules,invariants,actor,dice_event,resolve,target,changes}.go` |
| A | `internal/mechanics/{rules,formula,rng,actor,dice_event}_test.go` |
| A | `rules/dark-forest.yaml` |
| AM | `internal/mechanics/{dice_event.go,dice_event_test.go}` — поверх задачи лежит правка оркестратора (seed строкой) |
| M | `schemas/events/dice.rolled.v1.json`, `shared/contracts/validate_test.go`, `analysis/api-contracts.md` §2.3.5 — та же правка оркестратора, проверялась на полноту |
| M | `Docs/dev-team/epics/EPIC-001-foundation/dev-log.md` (запись T-015) — читалась, не правилась |

Вне области (по указанию оркестратора): `shared/testkit/**` и `shared/eventbus/**` — итерация 2
T-014 другого разработчика; `.github/**`, `Makefile`, `cmd/mvctl/**`, `ops/**`, `scripts/**` — T-012
и T-013.

Основание: `tasks.md` §1 (общий DoD) и §6 (T-015); `components/state-and-mechanics.md` v0.2 §2,
§5.1–§5.7; `contracts.md` v0.4 C-03 v1.1, C-02 v1.2; ADR-001 п. 3 и доп. п. 2, ADR-003 п. 5,
ADR-012 п. 1–6; `requirements/domain-review.md` §3.3 и `prd.md` приложение A; `analysis/data-model.md`
§6.3, §7.3; схемы `schemas/events/{dice.rolled,combat.decided,_common}.v1.json`; `plan/ownership.md`
§1; `journal.md` (последние пять записей, включая решение оркестратора по дефекту seed).

### Вердикт

**Вернуть.** Critical: 0 · Major: 1 · Minor: 7 · Nit: 4.

По существу это очень крепкая работа. Золотые числа сверены с приложением A построчно и совпадают
все; валидация `Load` ловит 34 негативных кейса и называет **конкретный ключ**, а не «файл плохой»;
из шестнадцати мутаций, которые я внёс, четырнадцать умерли — включая сдвиг границы крита, знак в
уроне, порог бегства, порядок байт в seed, потерю верхней границы `ClampHP` и проглоченную ошибку
валидации. Пакет не импортирует ни один `internal/*`, не читает часы и не ходит в сеть; после
`Load` ничего не мутируется, и детерминизм устоял под 16 горутинами, двумя `*Rules` из одного файла
и обратным порядком вызовов.

Возврат — из-за одной дыры, и она ровно в том свойстве, ради которого задача делалась. **Поток
генератора ничем не закреплён**: замена `rand.NewPCG(seed, 0)` на `rand.NewPCG(0, seed)` проходит
весь набор зелёным (Major-1). Шесть векторов `Seed` пиннят адрес броска, но не сам бросок, а
`TestRollNdMPlusK` выводит ожидание из `NewRNG(Seed(...))` — той же функции, которую проверяет, и
поэтому такую подмену не увидит по построению. Цена ошибки — все записанные прогоны проекта разом
(`events_hash_match`, `dice_rolled_new = 0` из C-14). Правка — таблица на десяток строк, числа
ниже. Остальное — Minor и Nit, переделки не требуют.

### Что проверено экспериментом

Окружение: машина владельца, `GOFLAGS=-buildvcs=false`, Go 1.26, golangci-lint v2.13.2, `-race`
недоступен.

| Проверка | Команда | Результат |
|---|---|---|
| Сборка и `vet` | `go build ./... && go vet ./internal/mechanics/...` | чисто |
| Тесты и покрытие | `go test -count=1 -cover ./internal/mechanics/...` | `ok`, **94,9 %** при пороге 60 % |
| Линтер | `golangci-lint run ./internal/mechanics/...` | **0 issues** |
| Формат | `gofmt -l internal/mechanics/ rules/` | пусто |
| Границы ADR-001 | правило `internal-mechanics` в `.golangci.yml`; импорты не-тестовых файлов | только `shared/entity`, `gopkg.in/yaml.v3`, stdlib; `shared/{eventbus,contracts}` — только в `_test.go` |
| Векторы `Seed` вне Go | независимый пересчёт на Python: `sha256(eid+":"+i)[:8]` big endian | все 6 совпали до бита |
| Золотые числа | ручная сверка с `prd.md` приложение A и `domain-review.md` §3.3 | совпали все: 10/2/12/d6/2 и 10/3/11/d4/null, крит 20 ×2, фамбл 1, `10 + living_enemies`, `hp_max`, порядок целей, трофей, `60s`/`2`, 10 инвариантов. Математика приложения A сходится: `d20+2 >= 11` и `d20+3 >= 12` — оба 12/20 = 0,60 |
| Детерминизм под нагрузкой | зонд: 200 причин × 4 индекса, эталон последовательно; затем 16 горутин, обратный порядок, попеременно два `*Rules` из одного файла | 0 расхождений на 12 800 сверках, трижды подряд |
| Независимость от порядка | `RollCheck` до и после 500 посторонних бросков `5d8+3` | `Roll` и `CheckResult` совпадают поле в поле |
| Повторный `Load` | два `Load` одного файла, сверка `Stats` и `Document()` | содержимое совпадает (о `Flee *int` — Minor-2) |
| Враждебный YAML | 13 входов: два документа, алиас-бомба 9^4, вложенность 500, табы, бинарь, `null` во всех ключах, списки вместо карт, строка вместо числа, 20-значный `idle_after_missed`, отрицательные `crit_natural`/`timeout` | **ни одной паники, ни одного зависания**, все сообщения читаемы и называют ключ или строку YAML. Единственный принятый вход — «два документа» (Minor-5) |
| Мутации | 16 точечных, каждая откатывалась | 14 убито, 2 выжили (Major-1, Minor-6) |

Таблица мутаций (файл восстанавливался после каждой; контрольные суммы всех девяти не-тестовых
файлов сверены с индексом после серии):

| № | Мутация | Итог |
|---|---|---|
| 1 | `Critical`: `== crit_natural` → `>= crit_natural-1` (крит с 19) | убита `TestHitTable` |
| 2 | `Damage`: `rolled *= CritMultiplier` → `rolled -= …` (знак) | убита `TestDamageTable` |
| 3 | `CheckExpr.Threshold`: `total += v` → `total -= v` (порог бегства) | убита `TestFleeTable`, `TestFleeOutcomeFitsCombatDecided` |
| 4 | `Seed`: `BigEndian` → `LittleEndian` | убита `TestSeedVectors` |
| 5 | `ClampHP`: `min(max(hp,0),hpMax)` → `max(hp,0)` | убита `TestClampHP`, `TestRestore` |
| 6 | `compileEntities`: отбросить ошибку `hp_max < 1` | убита `TestLoadRejects/hp_max_below_one` |
| 7 | `DiceExpr.Roll`: `natural` = последний кубик вместо первого | убита |
| 8 | `DiceExpr.Roll`: потерять `+ d.Modifier` | убита `TestDiceRollStaysInRange` |
| 9 | `Fumble` читает грань крита | убита `TestHitTable` |
| 10 | `LoadBytes`: проглотить ошибку `compile()` | убита `TestLoadRejects` (2 кейса) |
| 11 | `Excluded`: игнорировать `participation` | убита `TestExcluded` |
| 12 | **`NewRNG`: `PCG(seed, 0)` → `PCG(0, seed)`** | **выжила — Major-1** |
| 13 | `Seed`: убрать разделитель `":"` | убита `TestSeedVectors` |
| 14 | `DiceRolledPayload`: seed шестнадцатеричной строкой | убита (схема) |
| 15 | `DiceRolledPayload`: `index` ← `result` | убита |
| 16 | **`Roll.Formula`: `dice.String()` → сырая строка аргумента** | **выжила — Minor-6** |

### Замечания

#### Major-1 · `internal/mechanics/rng.go:39` — поток RNG не закреплён ни одним тестом

`func NewRNG(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 0)) }`

Воспроизведение: заменить на `rand.NewPCG(0, seed)` → `go test -count=1 ./internal/mechanics/...`
даёт `ok` (94,9 % покрытия, все 40 функций зелёные). То же будет при `rand.NewChaCha8`, при другом
методе генератора внутри `DiceExpr.Roll` и при смене алгоритма PCG в будущей версии Go.

Почему это существенно. Комментарий над функцией сам формулирует гарантию: «a run recorded today
replays on the next toolchain». На ней держатся ADR-003 п. 4–5 (record-replay),
`state-and-mechanics.md` §5.6 (`dice_rolled_new = 0`, `events_hash_match`) и NFR-060/061. Векторы
`Seed` (`rng_test.go:15`) пиннят **адрес** броска — SHA-256 и порядок байт; сам бросок не пиннит
ничто. `TestRollNdMPlusK` (`rng_test.go:174`) сверяет результат с ручной прокруткой
`NewRNG(Seed(cause, 0))` — это проверка внутренней согласованности `DiceExpr.Roll` с генератором, а
не проверка генератора: при подмене потока обе стороны равенства меняются одинаково.

Правка: добавить в `rng_test.go` таблицу золотых бросков рядом с `TestSeedVectors`, с
комментарием «эти числа — контракт записи; их изменение обесценивает все записанные прогоны».
Значения на текущей реализации (получены зондом на этой ветке):

```go
{cause: "01JC0000000000000000000000", index: 0, formula: "d20",   result: 3, natural: 3},
{cause: "01JC0000000000000000000000", index: 1, formula: "d6",    result: 4, natural: 4},
{cause: "01JC0000000000000000000000", index: 2, formula: "d20",   result: 6, natural: 6},
{cause: "01JC0000000000000000000000", index: 3, formula: "d4",    result: 2, natural: 2},
{cause: "player.attacked",            index: 0, formula: "3d6+2", result: 9, natural: 3},
```

Индекс 2 полезен отдельно: его seed (`1898231145079728858`) в векторах `Seed` не встречается, то
есть строка проверяет и `Seed`, и поток разом. Достаточно этих пяти строк; отдельного теста на «PCG
именно с (seed, 0)» не нужно — таблица закрывает любую подмену алгоритма.

#### Minor-1 · `internal/mechanics/changes.go:16` — сигнатура `ChangesFor` не совпадает ни с C-03, ни с §5.1, а в эскалации названа формой §5.1

- C-03: `func ChangesFor(o Outcome, a Action, actors map[string]*Actor, causeEventID string) []entity.Change`
- §5.1: `func ChangesFor(a Action, o Outcome, attacker, target *Actor, factEventID string) []ProposedChange`
- реализовано: `… ([]ProposedChange, error)`

Порядок и типы аргументов — по §5.1, но **возвращаемых значений два, а не одно**. `dev-log.md` §6
приводит реализованную форму точно и объясняет причину («вернуть ошибку без канала ошибки нельзя»),
но столбец назван «§5.1 (реализовано)», а запись в `journal.md` говорит просто «реализованы формы
§5.1». Если system-architect приведёт C-03 к §5.1 буквально, контракт станет `[]ProposedChange` — и
код разойдётся уже со свежеисправленным контрактом, а потребитель EPIC-003, собранный по §5.1, не
скомпилируется.

Правка (кода трогать не нужно): в отчёте оркестратору и в `dev-log.md` §6/§10 п. 1 указать, что это
**третья форма**, и сформулировать вопрос архитектору как «привести C-03 **и** §5.1 к
`([]ProposedChange, error)`», а не «привести C-03 к §5.1». По существу канал ошибки нужен и после
T-053: `ChangesFor` может получить исход, несовместимый с действием (`Loot` при живой цели,
`Success != nil` у `attack`), и молча вернуть пустой набор ops хуже, чем отказать.

#### Minor-2 · `internal/mechanics/rules.go:589` (+ `rules.go:87`) — `Document()` обещает копию, но течёт через `StatsDoc.Flee *int`

`maps.Clone` копирует карту, но значения `StatsDoc` несут тот же указатель `Flee *int`.

Воспроизведение (зонд):

```
flee before=2 after mutating the returned copy=99
```

то есть `*doc.Entities["player"].Flee = 99` меняет и то, что вернёт следующий `r.Document()`.
Комментарий над функцией утверждает обратное: «It is a copy: the maps of a loaded rule set stay the
rule set's own». `TestLootAndDocumentAreCopies` (`rules_test.go:122`) проверяет карты и срезы, но не
указатель внутри значения. Практического вреда сегодня нет — `Rules.Stats` считается на `Load` и не
меняется, — но обещание в комментарии не выполняется.

Правка: в `Document()` продублировать `Flee`
(`if s.Flee != nil { v := *s.Flee; s.Flee = &v; doc.Entities[k] = s }`) и добавить кейс в
`TestLootAndDocumentAreCopies`.

#### Minor-3 · `internal/mechanics/rules.go:302` и `rules.go:464` — `Load` называет непредсказуемый ключ, когда сломан не один

`compileEntities` и `compileLoot` идут по карте Go, поэтому при двух битых записях выбор
обвиняемого зависит от рандомизации итерации.

Воспроизведение: файл с `hp_max: 0` **и** у `player`, и у `wolf`, 200 одинаковых `LoadBytes`:

```
distinct error messages over 200 identical Loads: 2 ->
  entities.player.hp_max: … : 177
  entities.wolf.hp_max:   … :  23
```

Пакет декларирует отсутствие зависимости от порядка карт и глобального состояния; здесь она есть, и
падение в CI по такому файлу невоспроизводимо у автора правил.

Правка: обходить `slices.Sorted(maps.Keys(r.doc.Entities))` и то же для `Loot`. Тест: два битых
kind → `ConfigError.Path` один и тот же на 100 прогонах.

#### Minor-4 · `internal/mechanics/formula.go:98` — `ParseDice` не ограничивает число кубов и граней

`count < 1` и `sides < 2` — единственные границы (`formula.go:135`, `:138`). Сверху — ничего.

Воспроизведение (зонд): `dmg: 200000000d6` в `rules/dark-forest.yaml` →

```
Load -> err=<nil>
Stats(player).Dmg = "200000000d6"
one damage roll took 466ms, result=700009577
```

При `10^12` кубов один бросок — десятки минут внутри хода; при `~10^18` переполняется `Max()`.
Это противоречит и обещанию файла («Every complaint is raised here rather than during a fight»), и
мотиву узкой грамматики в ADR-012 (строка 17: «широкая поверхность для ошибок/инъекций из
блупринтов»): `Rules.Roll` документирован как вход для фоновых таблиц GM региона, а их формулы
приходят из блупринтов, а не из файла под ревью.

Правка: в `ParseDice` — верхние границы уровня «здравого смысла» (`count <= 100`, `sides <= 1000`) с
тем же `ConfigError`, и по кейсу на каждую в `TestParseDiceRejects`.

#### Minor-5 · `internal/mechanics/rules.go:244` — второй YAML-документ проглатывается молча

`yaml.Decoder.Decode` читает первый документ и останавливается. Воспроизведение: настоящий
`rules/dark-forest.yaml` плюс `\n---\nschema_version: 99\n` загружается без ошибки. Файл, в котором
редактор оставил `---`, потеряет половину правил без единого сообщения — при том что вся остальная
валидация построена на «неизвестный ключ = опечатка = ошибка».

Правка: после успешного `Decode` вызвать его второй раз в пустышку и требовать `io.EOF`; кейс в
`TestLoadRejectsGarbage`.

#### Minor-6 · `internal/mechanics/rng.go:68` — канонизация `Roll.Formula` не закреплена

Мутация 16 (`Formula: dice.String()` → `Formula: formula`) проходит весь набор: все формулы в
тестах уже записаны в канонической форме. Между тем `formula` уходит на провод
(`dice.rolled.roll.formula`) и участвует в сверке записи с прогоном. В самом репозитории две
конвенции уже расходятся: фикстуры `shared/contracts/validate_test.go:72,207` пишут `"1d20"` и
`"1d8+2"`, а `DiceExpr.String()` отрендерит их как `"d20"` и `"d8+2"`.

Правка: один assert — `r.Roll(cause, 0, "1d20", PurposeHit).Formula == "d20"` — и заодно решить,
какую форму считать канонической на проводе (предлагаю форму `DiceExpr.String()`, раз
`DiceRolledPayload` — единственный конструктор; тогда фикстуры контрактов стоит привести к ней при
следующей правке C-01).

#### Minor-7 · полнота правки оркестратора по `dice.rolled.roll.seed`

Исполняемая часть правки **полна и верна** — проверено:

- `schemas/events/dice.rolled.v1.json:13` — `"type": "string"`, шаблон `^(0|[1-9][0-9]{0,19})$`;
- `internal/mechanics/dice_event.go:22` — `strconv.FormatUint(roll.Seed, 10)`;
- три фикстуры `shared/contracts/validate_test.go:72,207,288` — строки;
- `analysis/api-contracts.md` §2.3.5 — «на проводе `seed` — десятичная строка» с обоснованием;
- новый блок в `TestDiceRolledPayloadValidates` действительно проверяет обобщённый разбор
  (`map[string]any` → `.(string)`), а не только типизированный;
- по репозиторию не осталось ни одного числового `seed` у `dice.rolled` (`grep` по `shared/`,
  `schemas/`, `testdata/`, `cmd/`, `Docs/dev-team/analysis/`);
- шаблон корректно отвергает ведущий ноль, знак, пустую строку и дробь.

Не доведено до конца в документах:

1. `analysis/data-model.md:420` (§7.3 DiceRoll) по-прежнему описывает `roll{index, formula, seed,
   result, natural}` без указания типа на проводе — единственное место, где читатель ищет тип поля,
   и оно теперь молчит. Одна строка: «`seed` — десятичная строка (uint64 не переживает разбор в
   `map[string]any`)».
2. `dev-log.md` §7 записи T-015 утверждает «Реализовано **по схеме**» (то есть числом), а §10 п. 3
   держит вопрос «строка или число» открытым. И то и другое устарело в тот момент, когда оркестратор
   внёс правку. Файл общий и не мой — прошу developer#1 закрыть п. 3 ссылкой на запись в
   `journal.md` и поправить §7.

### Nit

1. `schemas/events/dice.rolled.v1.json:13` — шаблон `^(0|[1-9][0-9]{0,19})$` допускает 20-значные
   числа выше `2^64-1` (`99999999999999999999`). Регуляркой точную границу uint64 задавать некрасиво,
   а единственный издатель — `strconv.FormatUint`; оставить как есть, но знать о зазоре.
2. `internal/mechanics/dice_event.go:22` — `DiceRolledPayload` не проверяет `roller`: при пустом
   `entity.Ref` соберётся payload, который схема отвергнет (`minLength: 1`), и ошибка вылезет на
   публикации, а не в точке вызова. Для «единственного конструктора» дешевле отказать сразу.
3. `internal/mechanics/rng.go:49` и `:79` — `Rules.Roll`/`RollCheck` не используют получатель; вызов
   на `nil *Rules` пройдёт. Безвредно, но либо функция пакета, либо строка в комментарии.
4. Дрейф документов, не кода (в отчёт архитектору, не в правку разработчику): `data-model.md` §6.3
   пишет `flee{formula: …}`, а §5.2 и файл — `flee{check: …}`; `domain-review.md:126` говорит, что
   успешное бегство ведёт в `outside:{region}`, тогда как `prd.md` приложение A и §5.2 — в
   `outside:{world}`/`{world_id}`. Код следует PRD и §5.2, то есть **правильной** версии; расходятся
   между собой документы.

### Что проверено и подтверждено

- **Золотые числа v0.1** — сверены с `prd.md` приложением A и `domain-review.md` §3.3 вручную, по
  каждому числу: статы обоих видов, крит/фамбл и множитель, формулы попадания и бегства, `on_fail`,
  `success_position`, `rest.restore`, порядок и исключения `npc_target`, трофей, `round{60s, 2}`,
  все десять инвариантов. Расхождений нет. Заявленная калибровка (0,60 у обеих сторон) из этих
  чисел выводится.
- **Формат `rules/dark-forest.yaml`** — ключ в ключ с `state-and-mechanics.md` §5.2, включая
  комментарии со ссылками на FR/DR. Файл читается как документ правил, а не как конфиг.
- **Валидация `Load`** — все шесть требований §5.2 закрыты (формулы разбираются, `damage_formula`
  ссылается на существующий атрибут или валидное выражение, `hp_max >= 1` и `def >= 1`, `loot` — на
  известные kind, `invariants` — только известные id, неизвестные ключи — ошибка), плюс сверх того
  `atk >= 0`, неотрицательность урона по `DiceExpr.Min()`, грани крита/фамбла внутри кубика и их
  несовпадение, `crit_multiplier >= 1`, плейсхолдеры позиции, повторы в `order` и `invariants`,
  положительность `round.timeout`. `KnownFields(true)` ловит опечатку **на любом уровне**, а не
  только на верхнем, — это строже, чем просил §5.2.
- **Диагностика ошибок** — `ConfigError{Path, Reason}` с точечным путём; `errors.Is(err,
  ErrInvalidRules)` истинно для всех; тесты сверяют не «отказал», а **какой ключ** назван (34 кейса).
  `Load` добавляет имя файла, `os.ErrNotExist` пробрасывается. На 13 враждебных входах — ни паники,
  ни зависания.
- **Мини-грамматика** — ровно §5.3, без `==`, `in`, скобок и вложенности; идентификаторы слева
  только от актора, справа от цели и контекста; неизвестный идентификатор — ошибка, а не ноль
  (проверено на волке без `flee`). Round-trip `String()` → `ParseDice` сохраняет выражение.
- **RNG** — совпадает с ADR-003 п. 5 и ADR-012 п. 3 буквально; шесть векторов `Seed` пересчитаны
  вне Go; разделение индексов (1000 без коллизий, не менее 16 бит между соседями) и причин (1000 без
  коллизий); броски воспроизводимы после тысячи посторонних, в обратном порядке, на втором `*Rules`
  и из 16 горутин; распределение d20 покрывает все грани со средним около 10,5; `NdM+K` — на одном
  генераторе, `Natural` — первый кубик; `Roll` отказывается бросать без причины, при отрицательном
  индексе, при неизвестном purpose и при неразбираемой формуле.
- **Заглушки** — `Resolve` и `ChangesFor` явно возвращают `ErrNotImplemented` (`errors.Is`),
  `Invariants()` даёт 10 записей с `Check == nil` и непустым `Where`, порядок совпадает с реестром.
  `TestStubs` проверяет и то, что заглушки не мутируют переданных акторов. `NPCTarget` возвращает
  `nil` — не «явный `ErrNotImplemented`» из DoD, но иначе и нельзя: в сигнатуре C-03/§5.1 канала
  ошибки нет; отклонение объяснено в коде и в `dev-log`, вынесено вопросом архитектору (см. ниже).
- **Границы и чистота** — ни одного импорта `internal/*`; `shared/{eventbus,contracts}` — только в
  тестах; ни `time.Now`, ни `os.Getenv`, ни сети; единственный I/O — `os.ReadFile` в `Load`, как и
  предписано §5.1. Правило `internal-mechanics` в `.golangci.yml` это же и стережёт.
- **Стыковка со схемами** — `DiceRolledPayload` валиден против `dice.rolled.v1.json` через
  `contracts.Validate` на полном событии для всех семи purposes; `roller` укладывается в
  `_common.json#/$defs/EntityWithName` (`name` там необязателен); `Outcome` полностью покрывает
  `combat.decided.outcome` и блок `hp` и для атаки, и для бегства; `rules_version` в событии берётся
  из `Load`, а не из константы.
- **Тесты не подогнаны под реализацию** — таблицы попаданий, урона, бегства и клампа записаны
  ожидаемыми числами, а не пересчётом формулы; негативные кейсы валидации ломают **настоящий** файл
  правил и падают с внятным «фикстура больше не содержит …: чините тест, а не правила»; 14 из 16
  мутаций пойманы. Исключения — Major-1 и Minor-6.
- **Владение** — тронуты только `internal/mechanics/**`, `rules/dark-forest.yaml` и запись в
  `dev-log.md`; по `ownership.md` §1 это строка EPIC-002, создание в EPIC-001 F-10 разрешено
  дополнением после G2 в `state-and-mechanics.md` §2. Девять файлов вместо шести из задачи — по
  структуре §2, отклонение объяснено в `dev-log`; принимается.
- **DoD T-015** — золотые числа выполнено, негативные тесты валидации выполнено, векторы `Seed`
  выполнено, `NdM+K` на одном RNG выполнено, `Resolve` → `ErrNotImplemented` выполнено, покрытие
  94,9 % при пороге 60 % выполнено, `make lint` и `gofmt` выполнено. `-race` из §1.2 общего DoD не
  прогонялся — недоступен в окружении; для этого пакета риск близок к нулю (после `Load` состояние
  неизменяемо, горутин нет, генератор создаётся на бросок), что я дополнительно подтвердил зондом на
  16 горутинах.

### Рекомендация системному архитектору по четырём сигнатурам

Реализованы формы §5.1. По существу §5.1 права во всех четырёх случаях, и правильное направление —
**править C-03, а не код**:

| Функция | Рекомендация | Почему по существу |
|---|---|---|
| `Roll` | C-03 → `(Roll, error)` | формула и purpose приходят снаружи (блупринт, фоновая таблица) и бывают невалидны; альтернатива — паника или тихий пустой бросок, а бросок, который нельзя воспроизвести, не должен попадать в журнал |
| `ActorFromEntity` | C-03 → `(e, enc *entity.Entity) (*Actor, error)` | C-03 v1.1 сам добавил `Participation` и `LastDamager`, а это факты **встречи**: из одной сущности игрока их не прочитать. C-03 противоречит здесь сам себе. Отдельно стоит обсудить возврат значением, а не указателем: `Actor` мал, а `*Actor` приглашает ровно ту мутацию, которую §5.1 запрещает |
| `ChangesFor` | и C-03, и §5.1 → `(a Action, o Outcome, attacker, target *Actor, factEventID string) ([]ProposedChange, error)` | `entity.Change{Path,Old,New}` из C-03 — это **факт** `entity.updated`, а не ops предложения (C-02), тут §5.1 однозначно прав; но канал ошибки нужен и после T-053 (несовместимый с действием исход), а его нет ни в одном из двух документов — см. Minor-1 |
| `DiceRolledPayload` | C-03 → `(roll Roll, roller entity.Ref)` | `dice.rolled.roller` — `EntityWithName` с обязательным `entity{id,type}`; из строки такой объект не собрать. Схема исполняема, C-03 — нет |

Пятое, вне четвёрки: **`NPCTarget` без канала ошибки**. Сегодня `nil` заглушки неотличим от
штатного «кусать некого» (UC-008 A2), и единственная защита — комментарий в `target.go` и
`TestStubs`. Риск до T-053 ограничен (единственный потребитель — `FixedMechanics` из T-017), но
закрывается даром: рекомендую C-03 v1.2 →
`NPCTarget(npc *Actor, candidates []*Actor) (*Actor, error)`. Если архитектор предпочтёт сохранить
форму v1 — тогда стоит зафиксировать в C-03 явным предложением, что `nil` означает «целей нет» и
никогда «не реализовано», а заглушка обязана жить в `testkit`, а не в `internal/mechanics`.

### Что нужно для приёмки (итерация 2)

1. Major-1 — таблица золотых бросков в `rng_test.go` (числа выше).
2. Minor-1 — переформулировать эскалацию по `ChangesFor` (правка `dev-log` и отчёта, код не трогать).
3. Minor-2…Minor-6 — по желанию разработчика в этой же итерации; каждая на 1–5 строк кода и один
   тест. Ни одна не блокирует.
4. Minor-7 п. 1 — строка в `data-model.md` §7.3 (файл аналитика, через оркестратора).

### Риски и допущения ревью

- **`-race` недоступен**, поэтому вывод о безопасности при конкурентном доступе сделан по чтению
  (после `Load` `Rules` не мутируется, горутин и глобального состояния в пакете нет, `NewRNG` — на
  каждый бросок) плюс зонд на 16 горутинах без единого расхождения на 12 800 сверках. Это сильное,
  но не исчерпывающее свидетельство: детектор гонок увидел бы то, чего не видит сверка результатов.
  Первый прогон в CI с `-race` стоит посмотреть отдельно.
- **Числа мутаций — с одной машины и одного прогона** на мутацию. Для выживших (12 и 16) это
  неважно: выживание доказывается одним зелёным прогоном, а не статистикой.
- **Непредсказуемость обвиняемого ключа (Minor-3)** измерена на 200 прогонах одного входа;
  соотношение 177/23 — свойство рандомизации карт Go, а не воспроизводимая пропорция.
- **Зонды писались мной** как временные `_test.go` в `internal/mechanics` и удалены; рабочее дерево
  сверено: md5 всех девяти не-тестовых файлов совпадает с индексом, `go test` зелёный,
  `golangci-lint` — 0 issues, `git status` по `internal/mechanics/` и `rules/` не содержит
  посторонних файлов. **Важно**: в записи T-014 `dev-log.md` (строки 4070 и 4075) developer#2
  зафиксировал «красный `internal/mechanics` из-за `zz_reviewtmp*_test.go`» и «2 issues в
  `rules.go`» — это были мои зонды и мои мутации, попавшие в его прогон; к качеству T-014 и T-015
  отношения не имеют, обе записи стоит считать снятыми.
- **Область ревью ограничена файлами T-015** и правкой seed. `shared/testkit/**`,
  `shared/eventbus/**` и файлы T-012/T-013 не открывались; вывод о полноте правки seed сделан по
  `grep` по всему репозиторию за вычетом `services/_archive/**` и `.claude/worktrees/**`.
- **Кроме этой записи в `review.md` я ничего не правил и не коммитил.** Мутации выполнялись поверх
  рабочей копии с резервными копиями и восстановлением после каждой.

---

## T-014 · ревью #2 (итерация 2) · 2026-09-10 · code-reviewer#2 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, база `fd93a6a`, ревьюировались **изменения в индексе** по итерации 2:
`shared/testkit/**` (`testkit.go`, `versions.go`, `containers.go`, `membus/membus.go`,
`contract/{contract.go,membus_test.go,redpanda_integration_test.go}`) и `shared/eventbus/kafka.go`
(общий код), плюс запись `dev-log.md` «developer#2 · T-014 · итерация 2».
`internal/mechanics/**`, `rules/**` (T-015, параллельное ревью code-reviewer#1, включая его
временные зонды `zz_reviewtmp*_test.go`) и файлы T-012 не открывались; общемодульные прогоны я не
использую как аргумент по T-014.

Основание: `tasks.md` §1 и §6 (T-014, критерий ворот), `contracts.md` v0.4 C-01 v1.1 («Реализации:
kafka-go, `membus` — **одна семантика**, общий contract-тест»), ADR-010 п. 4–5, `foundation.md` §9,
`journal.md` — решения оркестратора по ОВ T-014 итерации 2 от 2026-09-10 (SetChaos оставить;
`kafka.go` пометить `contract-change`; `ReadRange` левее начала журнала и **закрытие шины под
работающим `Tail`** — заявки architect#1 в бридж-блок волны 1; `ReorderTopics` — EPIC-003). Решённое
оркестратором повторно не поднимаю.

### Вердикт

**Вернуть. Ворота волны 1 закрытыми считать нельзя.** Critical: 0 · Major: 3 · Minor: 5 · Nit: 2.

По существу итерация сделана правильно: Major-1 ревью #1 закрыт не декларацией, а поведением (обе
реализации печатают «3 of 40», мутация возвращает «40 of 40»), кейс `Close` написан и **нашёл
настоящий дефект общего кода** — зависание `Kafka.Close()` между обработчиком и коммитом; это
сильный результат, и правка `kafka.go` разобрана мной отдельно ниже и признана верной.

Ворота не закрыты по другой причине: **три заявленных якоря набора не держат**, и это проверено
мутациями, а не рассуждением.

1. Мутация, воспроизводящая состояние `kafka.go` **до** правки, роняет набор только в 4 прогонах
   из 8 — самое рискованное изменение итерации (общий код, все подписки платформы) удерживается
   монеткой (Major-1).
2. Мутация 6 ревью #1 («`membus` возвращает `ErrClosed` вместо `nil` из закрытой подписки») в своей
   **припаркованной** половине по-прежнему оставляет весь набор зелёным — то есть самый частый
   реальный сценарий остановки (простаивающий потребитель + `Close`) не закрыт (Major-2).
3. Кейс `DedupDropsTheRepeatedDelivery` проходит, **если дубля не было вовсе** — на обеих
   реализациях (мутации I и J). Пункт DoD T-014 «at-least-once, дубли по `--chaos=duplicate`»
   набором фактически не держится (Major-3).

Объём исправлений — тестовый, порядка 20–30 строк в `contract.go` (плюс 3 строки в `membus.go` по
Minor-3). Реализацию, кроме этих трёх строк, править не нужно.

### Что проверено экспериментом (машина владельца, `GOFLAGS=-buildvcs=false`, Docker 29.6.1, `-race` недоступен)

| Проверка | Результат |
|---|---|
| `go build ./...`, `go vet ./shared/...`, `go vet -tags integration ./shared/...` | зелёные |
| `gofmt -l shared/testkit shared/eventbus` | пусто |
| `golangci-lint run ./shared/...` и `--build-tags=integration,e2e` | **0 issues** в обоих режимах |
| Contract на membus | **20/20 PASS**, 0,96 с (заявлено 0,90 с) — сходится |
| Contract на Redpanda | **20/20 PASS**, набор 8,25 с, пакет 8,9 с (заявлено 7,7–10,4 с) — сходится; обе реализации печатают «3 of 40» |
| `go test -tags integration ./shared/testkit/... ./shared/eventbus/...` | зелёные **два прогона подряд** (13,4 с и 12,7 с) |
| Покрытие (unit-режим) | `testkit` 74,0 %, `membus` 83,8 %, `eventbus` 75,4 %, `contract` 81,0 % (заявлено 81,2 %) — все ≥ 60 %, цифры честные |
| Красный прогон (замер) | одиночный кейс, падающий по таймауту, — 15,0–15,2 с; пакет с одним таким кейсом — 16,4 с (membus) и 23,1–25,1 с (redpanda) |
| Медленный раннер (эмуляция: `GOMAXPROCS=1` + 16 CPU-нагрузчиков на 32 ядрах) | набор на Redpanda **зелёный**, 113 с; но `AnUncommittedEventIsDeliveredAgain` — 18,8 с, `RetriesThenDeadLetter` — 12,7 с, `InvalidOnRead` — 9,5 с (см. Minor-5) |
| Свои зонды | удалены, `git status` по `shared/**` чист, md5 `membus.go` `4af70d8e…`, `kafka.go` `a5e77309…`, `delivery.go` `56230b56…`, `contract.go` `28f8e2f4…` совпадают с исходными |

**Мутации** (после каждой файл восстанавливался, md5 сверены):

| # | Мутация | Ожидание | Факт |
|---|---|---|---|
| A1 | `membus.go:278` — снят `stopping` в начале цикла `Subscribe` | падение | **падает**: `CancellingASubscriptionStopsItOnTheBacklog` («40 of 40») и `CloseStopsTheSubscriptions…` («40 of 40») |
| A2 | `membus.go:334` — снят `stopping` в `ReadRange` | падение | **набор зелёный** → точка не закрыта (Minor-1) |
| A3 | `membus.go:366` — снят `stopping` в `Tail` | падение | **набор зелёный** → точка не закрыта (Minor-1) |
| B1 | `membus.go:292` — припаркованная подписка на `Close` возвращает `ErrClosed` вместо `nil` (= мутация 6 ревью #1) | падение | **набор зелёный** → Major-2 |
| B2 | `membus.go:278` — подписка с непустым хвостом на `Close` возвращает `ErrClosed` | падение | **падает**: `CloseStopsTheSubscriptions…` «returned eventbus: bus is closed, want nil» |
| C | `membus.go:305` — курсор двигается и при прерванной доставке | падение | **падает**: `AnUncommittedEventIsDeliveredAgain`, таймаут 15 с |
| D | `membus.go:147` — `Lenient()` не выставляет флаг (перевёрнутый знак `!MV_BUS_VALIDATE_ON_READ`) | падение | **падает**: `WithoutValidationOnRead…`, 15 с |
| E | `eventbus/delivery.go` — `truncateRaw` отдаёт тело целиком | падение на обеих | **падает на обеих**: `ABigUndecodableBodyIsTruncated…` (membus 16,4 с, redpanda 23,8 с) |
| F | `membus.go:292` — припаркованная подписка на отмену возвращает `ctx.Err()` | внятные ошибки | **10 кейсов** сообщают «the subscription returned context canceled, want nil» — правка Minor-1 ревью #1 работает |
| H | `eventbus/kafka.go:173-176` — снято наследование контекста цикла от `closing` (состояние **до** правки) | падение на брокере | **падает 4 прогона из 8** (1 одиночный + 7 полных): «Close left the subscription running after 15s»; 4 прогона зелёные → Major-1 |
| I | `membus.go:157` — `SetChaos` ничего не делает (дубля нет) | падение | **набор зелёный** → Major-3 |
| J | `contract/redpanda_integration_test.go` — `Duplicate` публикует один раз | падение | **набор зелёный** → Major-3 (дыра на обеих половинах) |

Мутации A1, B2, C, D, F, I бьют по заглушке и поэтому честно валят только `membus`-половину; E — в
общем коде и валит обе; H и J — в kafka-половине. Подгонки под реализацию в новых кейсах не
обнаружено: все пять новых проверок бьют по поведению, а не по сигнатурам.

**Сравнительные замеры (мой зонд, один и тот же код на обеих реализациях; 40 событий в топике,
обработчик 25 мс, действие после третьего вызова):**

```
Close   Subscribe:  membus 3 of 40   |  redpanda  3 of 40    (кейс набора, сходится)
Close   ReadRange:  membus 3 of 40   |  redpanda 40 of 40    (расхождение, Minor-2)
Close   Tail:       membus 3 of 40   |  redpanda 40 of 40    (расхождение, Minor-2)
cancel  ReadRange:  membus 3 of 40   |  redpanda  3 of 40    (совпадает)
cancel  Tail:       membus 3 of 40   |  redpanda  4 of 40    (совпадает)
```

### Замечания

**Major-1 · `shared/testkit/contract/contract.go:883-926` (`closeStopsEverything`) — кейс, ради
которого правился общий `kafka.go`, ловит доправочное состояние примерно в половине прогонов.**
Мутация H (снять `kafka.go:173-176` — наследование контекста цикла от `closing`, то есть ровно код
до правки) даёт: одиночный прогон кейса и полные прогоны 1, 3, 4 — **зелёные**, полные прогоны
2, 5, 6, 7 — красные с «Close left the subscription running after 15s». Итого 4 падения из 8. В
`dev-log` §4 заявлено «4 прогона из 4». Причина разброса видна из механизма дефекта: зависает только
подписка, застигнутая **между обработчиком и `CommitMessages`**, а кейс закрывает шину в произвольный
момент внутри обработчика (`waitFor(calls>=3)` → `Close()`), то есть попадание в окно коммита
случайно. Цена: откат или регрессия самого рискованного изменения итерации (общий код, все подписки
платформы) проходит CI примерно в половине случаев; другого якоря нет — в `shared/eventbus`
интеграционных тестов не существует вовсе (`grep 'go:build integration' shared/` даёт только
`objstore` и `testkit/contract`).
*Как исправить*: сузить окно — обработчик сигналит **последней строкой** («сейчас вернусь»), кейс
закрывает шину по этому сигналу, и тогда цикл почти наверняка в коммите; либо (лучше) отдельный
интеграционный тест адаптера в `shared/eventbus` (свой брокер, N повторов подряд) — дефект живёт
там, а не в контракте, и контрактному набору не место в роли единственного сторожа реализации.

**Major-2 · `shared/testkit/membus/membus.go:292` и `:375` — закрытие шины под **припаркованной**
подпиской не закрыто ни одним кейсом.** Мутация B1 (в припаркованном `select`
`case <-b.st.done: return nil` → `return eventbus.ErrClosed`) оставляет весь набор зелёным — это
дословно мутация 6 ревью #1, которой обосновывался Major-2, а `dev-log` §4 объявляет её закрытой
(мутация B). Закрыта на самом деле другая ветка: выход через `stopping` при непустом хвосте (моя
мутация B2 действительно валит кейс). Разница не академическая: в бою подписка контекста платформы
почти всегда **простаивает на конце лога** в момент `runtime.Deps.Stop`, то есть непокрытой осталась
самая частая форма штатной остановки; сегодня обе реализации ведут себя верно (обе дают `nil`), но
регресс любой из них ворота не заметят.
*Как исправить*: в `closeStopsEverything` держать две подписки одной шины — одну на хвосте (как
сейчас), вторую простаивающую на другом топике — и требовать `nil` от обеих после одного `Close`;
это закрывает обе ветки одним закрытием и не требует второго таргета.

**Major-3 · `shared/testkit/contract/contract.go:443-480` (`DedupDropsTheRepeatedDelivery`) — кейс
проходит, даже если дубля не было.** Обработчик считает только уникальные `event.id` (окно
`testkit.NewDedup`), а утверждение — `len(handled)==2 && handled[0]==repeated.ID &&
handled[1]==marker.ID`; при одной доставке «повторного» события оно выполняется точно так же.
Проверено мутациями на обеих половинах: `SetChaos` — no-op (I) и `Duplicate` на kafka публикует один
раз (J) — набор зелёный в обоих случаях. Значит пункт DoD T-014 «at-least-once (дубли по
`--chaos=duplicate`)» набором не держится, а ручка `Target.Duplicate`, добавленная по Minor-3 ревью
#1, ничего не доказывает: она может перестать дублировать, и ворота промолчат. (Ревью #1 этой дыры
тоже не увидело — засчитывать её как регресс итерации 2 неправильно, но закрывать надо здесь.)
*Как исправить*: считать в кейсе **все** доставки `repeated.ID` до дедупа (отдельный `atomic.Int64`
рядом с окном) и требовать `≥ 2` — три строки; дедуп при этом проверяется тем же утверждением, что
и сейчас.

**Minor-1 · `membus.go:334` (`ReadRange`) и `:366` (`Tail`) — две из трёх точек предиката `stopping`
не закрыты набором.** Мутации A2 и A3 (снять предикат) оставляют набор зелёным: ни один кейс не
отменяет `Tail`/`ReadRange` при непустом хвосте — `JournalTailFollows…` отменяет припаркованный
`Tail`, `JournalReadsByIncreasingOffset` не отменяет ничего. Поведение сегодня верное и совпадает с
адаптером (мой замер: отмена — 3 of 40 против 3–4 of 40), но это ровно тот класс, из-за которого
возвращалась итерация 1, только в журнальных методах.
*Как исправить*: один кейс на журнал — 40 событий, обработчик 25 мс, отмена после третьего,
утверждение «после возврата `ReadRange`/`Tail` число вызовов не растёт и заметно меньше хвоста»;
структура уже есть в `cancellingStopsOnTheBacklog`, нужен параметр «каким методом читаем».

**Minor-2 · расхождение под `Close` в `Tail`/`ReadRange` существует, а в `dev-log` ОВ-5 записано как
«дефекта там нет».** Мой замер: `membus` — 3 of 40, redpanda — **40 of 40** (закрытый kafka-читатель
отдаёт уже выбранный буфер до конца; предел — `QueueCapacity`, по умолчанию 100 записей), возврат
`nil` в обоих случаях, 24 мс против 966 мс. То есть заглушка обещает «`Close` останавливает чтение
немедленно», а адаптер обрабатывает ещё до сотни событий — с побочными эффектами обработчика.
Решение оркестратора (заявка architect#1 в бридж-блок волны 1) **не переоткрываю**; прошу внести в
заявку эти цифры, потому что решение принималось на утверждении, что расхождения нет. Если C-01
определит поведение как «остановиться», правка — те же три строки (наследование `closing` в
`Tail`/`ReadRange`), которые автор сам и предлагает.

**Minor-3 · `membus.go:305`, `:343`, `:383` — после ошибки доставки проверяется только `ctx.Err()`,
а не `stopping(ctx)`; при `Close` под падающим обработчиком `Subscribe` возвращает ошибку, а kafka —
`nil`.** Замер (обработчик всегда падает по 100 мс, `Close` через 250 мс): `membus` →
`eventbus: write dead letter for player.looked: eventbus: bus is closed (cause: …)`, 4 вызова
обработчика; kafka → `<nil>`, 3 вызова (отменённый `closing`-контекст прерывает ретраи). Правило
C-01 «штатная остановка возвращает `nil`» нарушает заглушка, набором это не ловится. Направление
лжи безопасное (ложный красный в чужих тестах, а не ложный зелёный), поэтому Minor.
*Как исправить*: в трёх местах заменить `if ctx.Err() != nil` на `if b.stopping(ctx)`; кейс — тот же
`closeStopsEverything`, но с падающим обработчиком.

**Minor-4 · обработчик, игнорирующий отмену и вернувший `nil`: `membus` коммитит всегда, kafka — как
повезёт.** Замер (обработчик спит 300 мс и возвращает `nil`, отмена через 50 мс, затем новая
подписка той же группы): `membus` — событие **не** перевыдаётся ни разу; kafka — перевыдаётся в
**3 прогонах из 5**. Причина в адаптере: `reader.CommitMessages(ctx, …)` вызывается с уже отменённым
контекстом, а `select` в kafka-go при двух готовых ветках выбирает случайно. Формально обе стороны
укладываются в at-least-once, но `dev-log` §10 утверждает «обработчик, возвращающий на отмене `nil`,
будет закоммичен — и это правильно», и это верно только для заглушки: тест потребителя
«остановились в середине события — дубля после рестарта нет» зелёный на `membus` и красный примерно
через раз на брокере. Детерминированного кейса на это не написать, пока коммит идёт по отменяемому
контексту.
*Как исправить*: строкой в «известные расхождения» `dev-log` §5 (обязательно) и заявкой в бэклог по
общему коду: коммит уже обработанного события — через `context.WithoutCancel(ctx)` с небольшим
дедлайном, тогда «обработано → закоммичено» станет детерминированным и совпадёт с заглушкой.

**Minor-5 · запас таймаута 15 с на медленном раннере — не двадцатикратный, а полутора-двукратный.**
Под эмуляцией медленного раннера (`GOMAXPROCS=1` + 16 нагрузчиков) набор на Redpanda зелёный, но
`AnUncommittedEventIsDeliveredAgain` занял 18,8 с суммарно (кейс складывается из двух ожиданий по
15 с — поэтому и прошёл), `RetriesThenDeadLetterAndTheStreamMovesOn` — 12,7 с, `InvalidOnRead…` —
9,5 с, `DedupDropsTheRepeatedDelivery` — 9,0 с, `ABigUndecodableBody…` — 8,2 с. На холодный старт CI
15 с при этом хватает **с запасом и по другой причине**: вытягивание образа и ожидание порта живут
вне кейсов (`containers.go:41` — свои 90 с), а внутри кейса холодным остаётся только вступление в
consumer group (0,2–0,3 с на тёплой машине). Вывод: снижение 60 → 15 с правильное, но заявленный в
`dev-log` §7 «двадцатикратный запас» относится к ненагруженной машине; первый прогон на
ubuntu-latest (2–4 vCPU, там же Docker) стоит посмотреть отдельно, и первым кандидатом на флак
считать `AnUncommittedEventIsDeliveredAgain`, а не `PublishOfOneEventIsNotBatched`.
Бюджет красного прогона: 20 × 15 с = 5 минут **на одну реализацию**, а `integration` гоняет обе
половины одним пакетом — худший красный ~10 минут; в `go test -timeout 15m` и `timeout-minutes: 25`
(файл T-012) это укладывается, в «≤ 10 мин на job» ADR-010 п. 5 — впритык.

**Nit-1 · после `Close` читающая половина расходится**: kafka `End` → `4, <nil>`, `ReadRange` с
пустым интервалом → `5, <nil>`, `membus` → `ErrClosed` в обоих случаях (`membus.go:432` через
`topic()`). Это семейство Nit-1 ревью #1, отправленного оркестратором в бэклог; фиксирую только
факт, что новый кейс `Close` сделал эту область достижимой для потребителей.

**Nit-2 · `contract.go:575-579` — утверждение кейса отмены слишком широкое**: «обработано меньше 40»
пропустит и 39 из 40. Сегодняшнее расхождение было тотальным (40 из 40), так что кейс работает, но
граница вида `< backlog/2` стоила бы одной строки.

### Статус замечаний ревью #1

| # | Статус | Чем доказано |
|---|---|---|
| Major-1 (отмена не замечается при непустом хвосте) | **закрыто** | обе реализации дают «3 of 40» в кейсе (мои прогоны обеих половин); мутация A1 возвращает «40 of 40» и валит два кейса |
| Major-2 (`Close` не покрыт) | **закрыто частично** | кейс `Close` есть и держит ветку «хвост» (мутация B2 валит набор), но ветка «припаркованная подписка» — мутация 6 ревью #1 — по-прежнему оставляет набор зелёным (мутация B1) → Major-2 этого ревью |
| Minor-1 (`stop()` выбрасывал результат `Subscribe`) | **закрыто** | `stop(t)` с `t.Errorf`; мутация F даёт 10 внятных сообщений вместо таймаутов |
| Minor-2 (перевыдача некоммитнутого) | **закрыто** | кейс `AnUncommittedEventIsDeliveredAgain` зелёный на обеих; мутация C валит его (15 с) |
| Minor-3 (`--chaos=duplicate` в наборе) | **закрыто формально, по существу нет** | ручка `Target.Duplicate` появилась и на `membus` включает `Chaos{Duplicate:true}`, но кейс проходит и без дубля (мутации I и J) → Major-3 этого ревью |
| Minor-4 (отклонение по хаосу не записано) | **закрыто** | `dev-log` §6 п. 4–5: `bool` вместо 5 %, `ReorderTopics` отсутствует, потребитель назван (решение оркестратора — EPIC-003) |
| Minor-5 (усечение большого тела на брокере) | **закрыто** | кейс `ABigUndecodableBodyIsTruncated…` зелёный на обеих; мутация E валит **обе** реализации |
| Minor-6 (обратная сторона флага валидации) | **закрыто** | кейс `WithoutValidationOnRead…` + `Bus.Lenient()`; мутация D валит его |
| Minor-7 (известные расхождения записаны) | **закрыто**, требует дополнения | `dev-log` §5 перечисляет два; по итогам этого ревью туда же просятся Minor-2 и Minor-4 (Close под `Tail`/`ReadRange`; коммит обработчика, игнорирующего отмену) |
| Nit-1…Nit-4 | в бэклоге по решению оркестратора | не поднимаю; Nit-1 выше — только факт, не требование |
| Nit-5 (таймаут 60 с) | **закрыто** | `Timeout = 15 s` (`contract.go:92`); мои замеры красного кейса — 15,0–15,2 с; оговорка — Minor-5 |

### Отдельно проверено и признано верным

1. **Правка `shared/eventbus/kafka.go` (контекст `closing`) — как изменение общего кода.** Разбор и
   прогоны: (а) отмена **до** закрытия читателей (`kafka.go:327-333`) — порядок верный, иначе цикл,
   стоящий в коммите, ничем не освобождается; (б) подписка, застигнутая между обработчиком и
   коммитом, завершается: `Subscribe` вернул `nil` через 349 мс после `Close`; (в) **событие,
   обработанное но не закоммитнутое на момент `Close`, не теряется** — новая шина той же группы
   получает его снова (проверено на живом брокере); (г) повторный `Close` безопасен и возвращает
   `nil` (`k.closed` под мьютексом, `kafka.Reader.Close` идемпотентен); (д) `Close` на шине без
   подписок не ломает остальное: `Publish` → `ErrClosed`, `Subscribe`/`Tail`/`ReadRange` →
   `ErrClosed` через `track` (`kafka.go:427`); (е) гонки «`Close` во время `Publish`» не видно:
   30 тесных прогонов с общим стартовым барьером — 30 из 30 `ErrClosed`, ни паники, ни зависания
   (`writer()` берёт тот же мьютекс, что и `Close`); (ж) `Tail` под `Close` не подвешивается (`nil`
   через 28 мс), `ReadRange` — `nil` через 1,0 с (это не зависание, а дочитывание буфера, см.
   Minor-2). Утечки нет: `closing` — поле шины, `AfterFunc` снимается `defer`. Замечаний по самой
   правке нет; пометка `contract-change` при коммите обязательна (решение оркестратора от
   2026-09-10).
2. **`Bus.Lenient()` как вид на ту же шину** — решение верное: второй `membus` дал бы пустой лог, а
   вынос изменяемого состояния в `*state` делает «шина и её виды — один транспорт» явным; копия
   `Bus` корректна, потому что мьютекса в `Bus` больше нет (иначе поймал бы `go vet`). Мутация D
   показывает, что кейс проверяет флаг, а не наличие ручки.
3. **`Target` вырос честно**: `Duplicate`/`Lenient` обязательны (`Run` падает без них), `Close`
   необязателен и кейс пропускается через `t.Skip` — таргет, обязанный пережить набор, остаётся
   возможным. Порядок кейсов (большое тело предпоследним, `Close` последним) обоснован и проверен.
4. **Цифры `dev-log` §8 честные**: 20/20 на обеих, membus 0,90 → у меня 0,96 с, Redpanda 7,7–10,4 →
   у меня 8,25 с, покрытие 74,0/83,8/75,4 % совпало, `contract` 81,2 → 81,0 %. Два зелёных прогона
   `integration` подряд воспроизвёл.

### Предложения в бэклог

1. **Интеграционный тест адаптера в `shared/eventbus`** (свой брокер через `testkit.StartRedpanda`,
   N повторов сценария «`Close` в момент коммита»): сегодня у общего кода нет ни одного
   собственного теста на живом брокере, и роль сторожа реализации навязана контрактному набору.
2. **Коммит уже обработанного события — через `context.WithoutCancel` с дедлайном** (Minor-4):
   уберёт недетерминированность «обработано → закоммичено» на отмене и сведёт поведение адаптера с
   заглушкой. Общий код, `contract-change`, владелец — system-architect.
3. **Параметризовать кейс отмены по методу чтения** (Minor-1) — один кейс закроет `Subscribe`,
   `Tail` и `ReadRange` вместо трёх.
4. **Дать двум кейсам с хвостом собственный топик** (риск из `dev-log` §10): `player_events` растёт
   на 80 событий за прогон, и все последующие подписки на брокере их перечитывают.
5. Подтверждаю предложения ревью #1, не снятые оркестратором: `depguard` на `shared/testkit`,
   вариант `ReadRange` с уже известным `End`, дедлайн для `End`.

### Риски и допущения

- **`-race` не прогонялся** (недоступен в окружении). Новое разделяемое состояние — `state.chaos`
  под общим мьютексом и `atomic.Int64` в новых кейсах; `Lenient()` копирует структуру без мьютекса.
  Под `-race` в CI первым стоит смотреть `SetChaos` на работающей шине и новые кейсы с хвостом.
- **Мутация H прогонялась 8 раз** (1 одиночный кейс + 7 полных наборов); соотношение 4/8 — оценка,
  а не константа: на более медленной машине окно коммита шире и падений может быть больше. Вывод
  «якорь ненадёжен» от этого не меняется — надёжный якорь обязан падать всегда.
- **Замер Minor-4 (3 перевыдачи из 5)** — статистика пяти прогонов на одной машине; природа явления
  (случайный выбор ветки `select` в kafka-go при отменённом контексте) от машины не зависит.
- **Эмуляция медленного раннера** (`GOMAXPROCS=1` + нагрузка) — не то же самое, что раннер GitHub с
  2–4 vCPU и Docker в том же контейнере: у меня узкое место — CPU процесса теста, у раннера ещё и
  диск с сетью брокера. Цифры Minor-5 — ориентир, а не прогноз.
- **Область ревью — только файлы T-014.** Файлы T-012 (`.github/workflows/go.yml`, `Makefile`)
  открывались только на чтение ради вопроса о бюджете красного прогона; замечаний по ним нет.
  Параллельные правки code-reviewer#1 в `internal/mechanics/**` и незакоммиченное изменение
  `shared/contracts/validate_test.go` на момент моих прогонов на измерения не влияли (тестовый файл
  чужого пакета).
- **Кроме этой записи в `review.md` ничего не правил и не коммитил.** Все зонды (`zz_cr2probe*`) и
  мутации откачены, рабочее дерево по `shared/**` сверено с индексом (`git diff` пуст), md5 четырёх
  затронутых файлов совпадают с исходными.

---

## T-015 · ревью #2 (итерация 2) · 2026-09-10 · code-reviewer#1 (TEAM-1)

### Границы

Проверялись только правки итерации 2 и регрессия от них: `internal/mechanics/**`, `rules/**`,
правка seed (`schemas/events/dice.rolled.v1.json`, `shared/contracts/validate_test.go`) и запись
`dev-log.md` «T-015 · итерация 2». `shared/testkit/**` и `shared/eventbus/**` — область
code-reviewer#2 (T-014), не открывались; общемодульный `golangci-lint run ./...` не использовался
как критерий из-за чужих зондов `zz_cr2probe*` в тех пакетах — линтер запускался по
`./internal/mechanics/...` и `./shared/contracts/...`.

Решения оркестратора от 2026-09-10 (четыре сигнатуры C-03 правит architect#1, код T-015 не
переделывается; `-race` — в CI) приняты и повторно не поднимаются.

### Вердикт

**Принять.** Critical: 0 · Major: 0 · Minor: 1 · Nit: 1.

Major-1 закрыт по существу, а не формально. Пять строк `TestGoldenRolls` — литеральные числа;
я пересчитал их **третьим путём** (свой скрипт вне Go: SHA-256 big endian, PCG-DXSM со
128-битным LCG и посевом `(seed, 0)`, отбор грани через `uint64n` с маской для степени двойки и
порогом `-n % n`) — совпали все пять строк по seed, результату и натуральной грани, плюс все шесть
векторов `TestSeedVectors`. Пять подмен из ревью #1 роняют именно `TestGoldenRolls`, причём две из
них — те самые, что в ревью #1 выживали, — сегодня не убивает ничто, кроме новой таблицы. Таблица
читается: изменение одной цифры в каждом из четырёх столбцов ожиданий роняет тест с внятным
сообщением. Minor-2…Minor-7 закрыты, каждый подтверждён экспериментом, а не чтением.

Единственное новое замечание — Minor и не блокирующее: константный терм проверки остался
единственным числом грамматики без верхней границы и молча переполняется. Это соседняя ветка того
же дерева, которую автор закрыл для модификатора кубов; предлагаю в бэклог.

### Статус замечаний ревью #1

| № | Замечание | Статус | Чем доказано |
|---|---|---|---|
| **Major-1** | поток RNG не закреплён | **закрыто** | `TestGoldenRolls`, 5 литеральных строк. Числа сверены моим независимым пересчётом вне Go (все 5 строк + 6 векторов `Seed`, 0 расхождений). Мутация `NewPCG(seed, 0)` → `NewPCG(0, seed)`: **все 5 подтестов падают**, и `TestGoldenRolls` — единственный упавший тест во всём пакете, то есть ловит именно он |
| Minor-1 | эскалация по `ChangesFor` названа формой §5.1 | **закрыто** | `dev-log.md` §6 «Уточнение итерации 2»: форма названа **третьей**, запрос переформулирован как «привести к `([]ProposedChange, error)` и C-03, и §5.1»; §10 п. 1 закрыт ссылкой на решение оркестратора. Код не тронут — верно |
| Minor-2 | `Document()` течёт через `StatsDoc.Flee *int` | **закрыто** | Зонд: правка `*Flee`, `Loot["wolf"][0].Name`, `Invariants[0]`, `Attack.Rolls[0]`, `Flee.Rolls[0]`, `NPCTarget.Order[0]`, `NPCTarget.Exclude[0]` и вставка ключей `entities.ghost`/`loot.ghost` в возвращённый документ — следующий `Document()` отдаёт `flee=2`, «волчья шкура», `inv-01`, `hit`, `flee`, `last_damager`, `idle`, вставленных ключей не видит. Аксессоры `Loot()` и `Invariants()` тоже копируют. Все ссылочные поля `RulesDocument` покрыты — сверял по определению типа, а не по списку из замечания |
| Minor-3 | `Load` называет непредсказуемый ключ | **закрыто** | Зонд: `hp_max: 0` у обоих видов, 200 одинаковых `LoadBytes` → **1** различное сообщение (было 2 в пропорции 177/23), `entities.player.hp_max`. То же для `loot` с двумя неизвестными kind → 1 сообщение, `loot.aardvark` (первый по алфавиту). Все итерации карт в `compile*` идут через `slices.Sorted(maps.Keys(...))`; `Kinds()` тоже сортирует |
| Minor-4 | нет верхней границы кубов/граней | **закрыто, расширено автором** | См. отдельный разбор ниже |
| Minor-5 | второй документ YAML проглатывается | **закрыто** | Зонд: второй документ с правилами, пустой второй документ, мусор после `---` и **одинокий `---` в конце** (мой четвёртый хвост, в тестах автора его нет) — все четыре отвергнуты с `ErrInvalidRules`. `---` в **начале** файла принимается — это корректный YAML-документ с явным маркером начала, не дефект |
| Minor-6 | канонизация `Roll.Formula` не закреплена | **закрыто** | Строка таблицы `1d6` → на проводе `d6`. Мутация `Formula: dice.String()` → `Formula: formula` роняет ровно этот подтест и ничего больше во всём пакете |
| Minor-7 | документы отстали от правки seed | **закрыто** | `data-model.md:420` теперь называет тип: «десятичная строка uint64 — число JSON не переживает разбор в `map[string]any`». `dev-log.md` §7 переписан, §10 п. 3 закрыт |
| Nit 1–4 | зазор шаблона uint64, `roller` в `DiceRolledPayload`, получатель у `Roll`, дрейф документов | **осознанно не тронуты** | §7 записи итерации 2 объясняет по каждому; Nit 2 и 3 — правка поверхности C-03, её делает architect#1. Возражений нет |

### Major-1 — разбор, чем закрыт

**1. Числа сверены третьим путём.** В ревью #1 числа получены моим зондом на Go, автором — своим
скриптом. Сегодня я пересчитал их заново, реализовав определения, а не вызвав библиотеку:

```
OK 01JC0000000000000000000000#0 1d20: seed=5566502161584001210  result=3 natural=3
OK 01JC0000000000000000000000#1 1d6 : seed=14095709468665957854 result=4 natural=4
OK 01JC0000000000000000000000#2 1d20: seed=1898231145079728858  result=6 natural=6
OK 01JC0000000000000000000000#3 1d4 : seed=1256483486913892533  result=2 natural=2
OK player.attacked#0 3d6+2         : seed=11954706955398637122 result=9 natural=3
+ все 6 векторов TestSeedVectors    MISMATCHES: 0
```

Строка индекса 2 действительно берёт seed, которого нет ни в одном другом тесте. Строка `3d6+2`
проверяет порядок трёх кубов на одном генераторе и модификатор.

**2. Числа — литералы, не пересчёт.** В теле `TestGoldenRolls` нет ни `Seed(...)`, ни `NewRNG(...)`,
ни `DiceExpr.String()`: все ожидания записаны цифрами и строками в таблице. Единственная
производная — `strconv.FormatUint(c.wantSeed, 10)` для сверки провода, и она выводится из
**литерального** `wantSeed`, а не из результата вызова; при `FormatUint(..., 16)` в
`DiceRolledPayload` сверка расходится (подмена 4).

**3. Падает именно `TestGoldenRolls`.** Для каждой подмены прогонялись и `-run TestGoldenRolls`, и
весь пакет:

| Подмена | `-run TestGoldenRolls` | Кто ещё упал во всём пакете |
|---|---|---|
| `rand.NewPCG(seed, 0)` → `(0, seed)` | FAIL, все 5 строк | **никто** — таблица единственная защита |
| `Seed`: убран разделитель `":"` | FAIL, все 5 строк | `TestSeedVectors` |
| `Seed`: `BigEndian` → `LittleEndian` | FAIL, все 5 строк | `TestSeedVectors` |
| `DiceRolledPayload`: `FormatUint(…, 10)` → `16` | FAIL, все 5 строк | `TestDiceRolledPayloadValidates`, `…EveryPurpose` (7 подтестов) |
| `Roll`: `dice.String()` → `formula` | FAIL, строка `1d6` | **никто** |

**4. Таблица читается, а не украшает.** Шестая проверка — мутации самой таблицы, по одной цифре на
столбец:

```
result  3 → 4   FAIL: d20 rolled 3 (natural 3), recorded 4 (natural 3): the stream of the generator changed
seed  …58 → 59  FAIL: seed 1898231145079728858, recorded 1898231145079728859 (+ строка провода)
natural 3 → 4   FAIL: 3d6+2 rolled 9 (natural 3), recorded 9 (natural 4)
wire  d6 → 1d6  FAIL: formula on the wire "d6", recorded "1d6"
```

Все четыре столбца ожиданий проверяются, ни один не мёртв. Сообщения называют, что именно
разошлось, и комментарий над тестом прямо запрещает перебазирование («That is a decision about
stored data, never a test to be re-baselined») — это ровно тот текст, которого не хватало.

### Minor-4 — граница 1000: разбор границ и оценка расширения

Границы (мой зонд, `ParseDice`):

| Вход | Итог |
|---|---|
| `999d999+999`, `999d999-999`, `1000d1000+1000`, `1000d1000-1000`, `1d1000`, `d6+1000`, `d6-1000`, `d2` | принято |
| `1001d6`, `1000d1001`, `1001d1001`, `d1001`, `d6+1001`, `d6-1001` | отвергнуто |
| `0d6`, `-2d6`, `d0`, `d1` | отвергнуто (прежние нижние границы целы) |
| `9223372036854775807d6`, `d9223372036854775807`, `d6+9223372036854775807` | отвергнуто |

Ошибки по существу: называют выражение, **фактическое** число и предел
(`dice expression "1001d6": 1001 dice, at most 1000: a roll has to finish inside a turn`), а не
«неверная формула».

Граница держится на `Load`, а не только в `ParseDice`, — проверено подстановкой в настоящий файл:
`entities.player.dmg` (`1001d6`, `d1001`, `d6+1001`), `rest.restore` (`1001d6`) и кубик проверки
(`attack.hit: "1001d20 + atk >= def"`) отвергаются с точечным путём. `1000d1000` в `dmg`
принимается — граница ровно там, где заявлена.

Стоимость самой широкой **разрешённой** формулы: 5000 бросков `1000d1000+1000` за 13,6 мс
(≈2,7 мкс на бросок) против 466 мс за один бросок `200000000d6` до правки. Обоснование «Стоимость»
из записи итерации 2 подтверждается замером.

Регрессии по правилам нет: единственные формулы репозитория — `d20`, `d6`, `d4` (`rules/` и
`testdata/`), ни одна не задета; `rules/dark-forest.yaml` грузится, все тесты золотых чисел зелёные.
Грамматике ADR-012 / §5.3 (`dice := [INT] 'd' INT [ ('+'|'-') INT ]`) сужение не противоречит:
документ границ `INT` не задаёт, а примеры документа (`d20`, `2d6`, `d6+1`) в границы попадают.

**Ограничение модификатора (сверх замечания) — оправдано, грамматику вредно не сужает.** Мотив в
коде честный и проверяемый: `d6+9223372036854775807` разбирался и переполнял `Max()` при сложении,
то есть это тот же класс дефекта, что и число кубов, а не вкусовое ужесточение. Ни одна формула
v0.1 не имеет модификатора больше `+2`, и порог 1000 оставляет три порядка запаса. Единственное
последствие для будущего — мир с HP в тысячах потребует поднять константу; это правка одной строки
и осознанное решение v0.1, а не долг. Перенос числа в §5.3 как контракта для блупринтов EPIC-003
уже записан оркестратором в `journal.md` — повторно не поднимаю.

### Новые замечания

#### Minor-8 · `internal/mechanics/formula.go`, `parseTerm` — константный терм проверки не ограничен и молча переполняется

Автор закрыл верхнюю границу для `count`, `sides` и `Modifier` — трёх из четырёх чисел грамматики.
Четвёртое, `term := IDENT | INT`, осталось без границы, и это то же переполнение на расстоянии
одного токена:

```
ParseCheck  "d20 + 9223372036854775807 >= def"            → ok, Eval: total = -9223372036854775797
ParseCheck  "d20 >= 9223372036854775807 + living_enemies" → ok, Eval: threshold = -9223372036854775808, OK = true
Load        attack.hit: "d20 + 9223372036854775807 >= def"        → err = <nil>
Load        attack.hit: "d20 + atk >= def + 9223372036854775807"  → err = <nil>
```

Второй случай неприятнее первого: проверка, которая должна была стать невыполнимой, становится
**всегда истинной**, и `Load` пропускает её молча — при том что весь файл построен на «Every
complaint is raised here rather than during a fight» и на «неизвестный ключ = опечатка = ошибка».

Почему не Major: `ParseCheck` вызывается только из `compileCheck` на `Load`, то есть выражение
проверки приходит из файла правил в репозитории, который человек ревьюит, а не из блупринта
(`Rules.Roll` принимает только `ParseDice`). Сегодняшний файл корректен, поведение не сломано.

Правка (две строки и кейс): в `parseTerm` после `parseIntStrict` отвергать `|n| > maxDiceModifier`
(или отдельной константой `maxTermConst` с тем же значением) с тем же текстом «at most 1000 either
way»; кейс в отказах `ParseCheck` и кейс `Load` на `attack.hit`. Замечание вне объёма итерации 2 —
предлагаю в бэклог, не как условие приёмки.

### Nit

1. `internal/mechanics/rng_test.go` — строка `3d6+2` в `TestGoldenRolls` бросается с `PurposeHit`,
   хотя `3d6+2` — очевидная формула урона. На проверку не влияет (purpose в ожиданиях не участвует),
   но читателю таблицы, которая объявлена «контрактом записи», такая строка задаёт лишний вопрос.
   `PurposeDamage` читалось бы ровнее.

### Что проверено экспериментом

Окружение: машина владельца, `GOFLAGS=-buildvcs=false`, Go 1.26, golangci-lint v2.13.2, `-race`
недоступен (решение ОВ-5: в CI).

| Проверка | Команда / зонд | Результат |
|---|---|---|
| Сборка, `vet`, формат | `go build ./...`, `go vet ./internal/mechanics/...`, `gofmt -l internal/mechanics/ rules/` | чисто |
| Тесты и покрытие | `go test -count=1 -cover ./internal/mechanics/...` | `ok`, **94,9 %** — не ниже итерации 1 |
| Тесты контрактов | `go test -count=1 -cover ./shared/contracts/...` | `ok`, 81,7 % |
| Линтер по своей области | `golangci-lint run ./internal/mechanics/... ./shared/contracts/...` | **0 issues** |
| Золотые числа, третий путь | свой скрипт вне Go: SHA-256 BE + PCG-DXSM `(seed, 0)` + `uint64n` (маска / порог `-n % n`) | 5 строк таблицы и 6 векторов `Seed` — **0 расхождений** |
| Пять подмен ревьюера | каждая применялась и откатывалась, прогон `-run TestGoldenRolls` и всего пакета | все пять убиты; две (поток PCG, канонизация формулы) убивает **только** новая таблица |
| Шестая проверка — читается ли таблица | по одной цифре в каждом из четырёх столбцов ожиданий | все четыре мутации роняют тест |
| Minor-2 | зонд: правка семи ссылочных полей возвращённого документа и вставка ключей, затем повторный `Document()`; плюс `Loot()`, `Invariants()` | утечек нет ни по одному полю |
| Minor-3 | зонд: два битых `hp_max`, 200 `LoadBytes`; два неизвестных kind в `loot`, 200 `LoadBytes` | по одному различному сообщению в обоих случаях (было 2) |
| Minor-4 границы | зонд: 20 выражений вокруг 999/1000/1001, 0, отрицательных, `d1`, `int64` max; плюс 7 подстановок в настоящий файл через `Load` | границы ровно на 1000, нижние целы, `Load` их применяет для `dmg`, `rest.restore` и кубика проверки |
| Minor-4 стоимость | 5000 бросков `1000d1000+1000` | 13,6 мс всего (≈2,7 мкс/бросок) против 466 мс за один бросок до правки |
| Minor-5 | 5 хвостов, включая одинокий `---` и `---` в начале файла | 4 отвергнуты, ведущий маркер принят (корректно) |
| Новое замечание Minor-8 | зонд: 3 выражения `ParseCheck` + `Eval`, 2 подстановки в `attack.hit` через `Load` | константа `int64` max принимается и переполняет сумму; `Load` молчит |
| Детерминизм (регрессия) | зонд: 200 причин × 4 индекса, `3d6+2`, сверка со вторым `*Rules` из того же файла | 800 сверок, **0 расхождений** |
| Золотые числа правил | `rules/dark-forest.yaml` против `prd.md` приложение A построчно | совпадают все: 10/2/12/d6/2 и 10/3/11/d4/null, крит 20 ×2, фамбл 1, `10 + living_enemies`, `hp_max`, порядок и исключения целей, трофей, `60s`/`2`, 10 инвариантов. По существу файл в итерации 2 не менялся |
| Новые тесты не тавтологичны | чтение `TestLoadNamesTheSameKeyEveryTime`, `TestLoadRejectsASecondDocument`, `TestLootAndDocumentAreCopies`, `TestDiceBounds` | сверяют конкретный `ConfigError.Path` и конкретные значения; сохранена защита «фикстура больше не содержит …: чините тест, а не правила» |
| Чистота дерева | md5 всех файлов области до и после серии; `git diff` по области; `git status --untracked-files=all` | md5 совпали, `git diff` пуст, посторонних файлов нет |

### Открытые вопросы

Своих нет. Оба вопроса разработчика из §9 записи итерации 2 (канонизация формулы в фикстурах
`shared/contracts/validate_test.go`; перенос предела 1000 в §5.3) оркестратор уже принял в
бридж-блок волны 1 — подтверждаю обе формулировки, добавить нечего.

### Риски и допущения ревью

- **`-race` не прогонялся** — недоступен, по решению ОВ-5 живёт в CI. Вывод о безопасности прежний:
  после `Load` состояние неизменяемо, горутин и глобального состояния нет, генератор создаётся на
  бросок; сверка 800 бросков на двух `*Rules` расхождений не дала. Детектор гонок увидел бы то,
  чего сверка результатов не видит, — первый прогон в CI стоит посмотреть отдельно.
- **Золотые числа проверены на одной платформе** (windows/amd64, Go 1.26). Мой пересчёт вне Go
  моделирует 64-битную ветку `uint64n`; на 32-битной сборке `math/rand/v2` берёт путь `uint32n`, и
  поток для граней, помещающихся в `uint32`, может отличаться. Для проекта это не риск (сборка
  `GOARCH=amd64`, distroless), но если когда-нибудь появится 32-битная цель, `TestGoldenRolls`
  покраснеет **правильно** — и это будет разговор о записанных прогонах, а не о тесте.
- **Таблица закрепляет пять адресов, а не всю поверхность.** У `RollCheck` собственной золотой
  строки нет; он привязан к `Roll` тестом `TestRollCheck` (тот же адрес → тот же результат и seed),
  поэтому подмена потока валит и его через таблицу. Считаю достаточным, отдельной строки не прошу.
- **Minor-8 не проверялся дальше показанного:** сегодняшний файл правил корректен, все тесты
  зелёные. Замечание — о молчании валидатора, а не о поломке.
- **Область ограничена файлами T-015** и правкой seed. `shared/testkit/**`, `shared/eventbus/**` и
  файлы T-012/T-013 не открывались; общемодульный `golangci-lint run ./...` намеренно не
  использовался как критерий из-за чужих зондов `zz_cr2probe*`.
- **Кроме этой записи в `review.md` ничего не правил и не коммитил.** Зонды (`zz_cr1probe*`) и все
  девять мутаций откачены; md5 файлов области совпадают с состоянием до серии, `git diff` по
  области пуст, посторонних отслеживаемых и неотслеживаемых файлов нет.

---

## T-016 · ревью #1 · 2026-09-10 · code-reviewer#1 (TEAM-1)

**Область:** `testdata/fixtures/**` (7 файлов) и `test/fixtures/fixtures_test.go`.
Файлы T-014 (`shared/testkit/**`, `shared/eventbus/**`) не открывались — там живые зонды
code-reviewer#2. `internal/mechanics` и `rules/dark-forest.yaml` (T-015, принята) открывались
только на чтение и один раз как временный зонд с немедленным откатом (см. «Что проверено
экспериментом», Э-1).

**Вердикт: принять.** Critical 0 · Major 0 · Minor 2 · Nit 5.

Заявление автора «производные числа выводятся, а не переписаны» проверено буквально и
подтверждается. В теле теста нет ни одной константы, повторяющей правила или фикстуру:
`grep -nE '\b(10|12|11|d6|d4|0\.25|4531)\b' test/fixtures/fixtures_test.go` даёт только строки
комментариев и ссылок на разделы. Сверка статов действительно идёт **целиком** структурой
`mechanics.Actor`, а не по пяти полям, и это доказано экспериментом, а не чтением.

### Что подтверждено

- **Статы и трофей.** `TestCombatStatsComeFromTheRules` сравнивает `*ActorFromEntity(e, nil)` с
  `Rules.Stats(kind)` через `reflect.DeepEqual` целиком: под сверку попадают не только
  `hp_max/atk/def/dmg/flee`, но и `HP`, `Type`, `Status`, `Participation`, `LastDamager` — то есть
  фикстура обязана иметь `hp == hp_max`, `status: alive` и пустую боевую роль. Трофей выводится из
  `Rules.Loot(kind)`, а не сверяется по имени.
- **Проекции региона** (`npc_ids`, `players_present`) выводятся из `position` сущностей, а не
  утверждаются списком (`TestRegionProjectionsFollowFromPositions`).
- **`state_hash` считается тестом независимо** — `entity.StateHash(entities)` по шести сущностям,
  и отдельно пересчитывается по `entities` объекта снапшота. Проверено в обе стороны (Э-3).
- **`size_bytes`, `entities_count`, `applied_proposals`, `snapshot.id`, `laws_version`,
  `rules_version`** выводятся из файла снапшота, длины набора, шаблона §4.10, `seq`, сущности мира
  и `Rules.Version` соответственно.
- **Схема осталась закрытой.** `schemas/events/snapshot.created.v1.json` на ветке не менялся
  (`git log` по файлу — последний коммит `0c92f2a`, T-006), `additionalProperties: false` на обоих
  уровнях, `required` — все восемь полей. Решение оркестратора п. 1 соблюдено: событие строится
  снятием трёх полей указателя, схема не расширена.
- **Соответствие решениям оркестратора по шести ОВ T-016** — противоречий нет: `scope` объектом,
  атрибуты региона по `data-model.md` §3.2, объект снапшота присутствует, `encounter_chance`
  проверяется только формой, имя мира на месте, `blueprint_ref` не закреплён равенством.
- **Тест действительно исполняется в CI:** job `unit` — `go test -short -race ./...`, пакет
  `multiverse-core.io/test/fixtures` попадает в `./...` и не спрятан за `-short`/тегом сборки.

### Замечания по серьёзности

#### Critical — нет

#### Major — нет

#### Minor-1. `snapshot.key` — единственное производное число, оставленное литералом

`test/fixtures/fixtures_test.go:47` (`snapshotK`), `:483`.

`state-and-mechanics.md` §4.3 задаёт ключ снапшота как `state/{YYYYMMDDTHHMMSSZ}-{seq:06d}.json`,
то есть `key` полностью выводится из `taken_at` и `seq`. Тест выводит из `seq` соседнее поле
`id` (`fmt.Sprintf("state:%s:%06d", worldID, s.Seq)`, строка 480), но `key` сверяет с
рукописной константой `snapshotK`.

Последствие показано мутацией (Э-4/M17): если сдвинуть `taken_at` **и** `written_at` в указателе и
в объекте на `2026-03-05T12:34:56Z`, оставив ключ и имя файла прежними, набор остаётся **зелёным** —
хотя фикстура стала внутренне несогласованной: указатель говорит «снимок сделан 5 марта», а имя
объекта — «1 января». Для эталона, по которому EPIC-002 будет писать `ListSnapshots` (сортировка по
`seq`, разобранному из ключа), это ровно тот дрейф, ради ловли которого задача и существует.

Как исправить: рядом с проверкой `id` вывести и ключ —
`fmt.Sprintf("state/%s-%06d.json", s.TakenAt.UTC().Format("20060102T150405Z"), s.Seq)` — и сверить
с `s.Key`; заодно проверить, что файл с таким именем существует
(`os.Stat(filepath.Join(snapshotDir(), filepath.Base(s.Key)))`), а `loadSnapshot` читать по
`pointer.Snapshot.Key`, а не по константе. Константу `snapshotK` тогда можно убрать совсем.

#### Minor-2. `size_bytes` эталона расходится с байтами на диске при выкладке на Windows

`testdata/fixtures/snapshots/state/latest.json:15`, `test/fixtures/fixtures_test.go:61` (`readLF`).

Нормализация CRLF→LF работает — это проверено (Э-5): после `git checkout` с
`core.autocrlf=true` объект снапшота лежит на диске как **4686 байт**, указатель говорит **4531**,
и весь набор остаётся зелёным. Претензии к тесту нет.

Претензия к артефакту. `.gitattributes` даёт `eol=lf` для `*.go`, `*.sh`, `*.yml`, `*.yaml`,
`*.toml` и `Makefile`, но **не для `*.json`**. По C-14 v1.1 и §4.4 эта фикстура — «эталон для
потребителей read-model»: потребитель читает указатель, читает объект по `key` и сверяет. Такой
потребитель (`FakeState.Seed`, `Harness`, e2e T-018, будущий `mvctl world init`) не вызывает
`readLF` — он читает файл как есть, и на Windows увидит объект на 155 байт длиннее, чем обещает
указатель. Знание о нормализации живёт сегодня только внутри одного теста.

Автор в dev-log §5 сознательно отказался просить строку в общий файл («правка общего файла ради
того, что решается двумя строками в тесте»). Аргумент понятен, но решает он задачу теста, а не
задачу эталона. Как исправить (одна строка, файл tech-lead#1):
`testdata/fixtures/** text eol=lf` в `.gitattributes`. `readLF` при этом остаётся — как страховка
для тех, кто клонировал раньше. Если tech-lead#1 правку не даёт — записать ограничение прямо в
`testdata/fixtures/README.md` («`size_bytes` — длина в LF-форме; на Windows-выкладке файл длиннее»),
чтобы потребитель не считал эталон повреждённым.

#### Nit-1. Словарь `weather` закрыт тестом, хотя по документу он открыт

`test/fixtures/fixtures_test.go:379–381`. `data-model.md` §3.1 пишет `weather` как
`clear, cloudy, rain, storm, fog…` с явным многоточием и пометкой «словарь блупринта». Тест
утверждает членство в закрытом списке из пяти значений. Когда EPIC-003 напишет блупринт со
`snow`, покраснеет тест фикстур, а не блупринт. `time_of_day` перечислен в §3.1 без многоточия —
там закрытый список уместен. Предложение: для `weather` оставить проверку «непустая строка из
нижнего регистра» либо перенести словарь в место, где он объявлен один раз.

#### Nit-2. `decodeStrict` в `TestPointerYieldsAValidSnapshotCreated` не строгий

`test/fixtures/fixtures_test.go:596`. `dec.DisallowUnknownFields()` не действует при разборе в
`map[string]any` — неизвестные ключи в карту просто попадают. Строгость здесь берётся из соседнего
теста, который разбирает тот же файл в структуру; сам вызов имени не оправдывает. Читателю проще,
если это будет `json.Unmarshal(readLF(...), &pointer)` с комментарием «строгость обеспечивает
`TestLatestPointerDescribesSnapshotZero`».

#### Nit-3. Длина имени персонажа не проверяется

`test/fixtures/fixtures_test.go:162`. `data-model.md` §3.3 задаёт для `player` ограничение
`2–32 символа`; тест проверяет только `Name != ""`. Ограничение дешёвое и относится ровно к тем
трём сущностям, которые фикстура и создаёт.

#### Nit-4. Тихий пропуск сверки `laws_version`

`test/fixtures/fixtures_test.go:507`. `if world := byID(entities, worldID); world != nil { … }` —
при отсутствии сущности мира сверка `laws_version` указателя молча не выполняется. Отсутствие мира
поймает `TestFixtureSetIsTheWorldBootstrapCreates`, так что дыры нет, но `t.Fatal` в `else`
дешевле, чем условие, которое читается как «если повезёт».

#### Nit-5. `test/fixtures/**` нет ни в «Файлах» T-016, ни в карте владения

`tasks.md` T-016 объявляет «Файлы: `testdata/fixtures/**`», `ownership.md` §1 — строку
`testdata/fixtures/**`. Тест физически не может лежать в `testdata/` (инструменты Go его
игнорируют), решение о размещении принято и записано в `journal.md`, так что нарушения нет —
но обе таблицы стоит дописать строкой `test/**` (владелец EPIC-001 → общие), иначе карта владения
перестаёт совпадать с деревом уже на волне 0.7, когда туда придёт `test/e2e/stubs_v0_test.go`.

### Что проверено экспериментом

Всё ниже — прогоны на рабочем дереве с немедленным откатом; `git diff` по области после серии пуст,
индекс не трогался, посторонних файлов нет.

**Э-1. «Сверка целиком падает при добавлении нового стата» — подтверждено, двумя путями.**

- *Путь A (только YAML).* `entities.player: {… , spd: 5}` в `rules/dark-forest.yaml` →
  `mechanics.LoadBytes` с `KnownFields(true)` отвергает файл, падают четыре теста набора с
  `load rules: … not a rules document: yaml: unmarshal errors`. Это ловит строгий загрузчик T-015,
  а не сверка — поэтому проверка не остановлена здесь.
- *Путь B (стат добавлен по-настоящему).* Временный зонд: поле `Spd int` в `mechanics.Actor`
  (`types.go`), `Spd int` с тегом `yaml:"spd"` в `StatsDoc` (`rules.go`), `Spd: s.Spd` в
  `compileStats`, `spd: 5` в `entities.player`. Результат — падают ровно три подтеста
  `TestCombatStatsComeFromTheRules/player-{A,B,C}` на `reflect.DeepEqual` (у волка `spd` нет, он
  и не падает). Значит заявление автора верно: новый стат правил не проедет мимо фикстур.
  Все три файла зонда откачены `git checkout --`, `grep -c Spd` → 0/0, `grep -c spd` в правилах → 0.

**Э-2. Мутации фикстур — 16 подмен, непойманных нет.** Каждая роняет свой подтест с внятным
сообщением (в скобках — что именно сказал тест):

| # | Подмена | Результат |
|---|---|---|
| M1 | `atk` игрока `2 → 3` | `TestCombatStatsComeFromTheRules/player-A` («fixture Atk:3 / rules Atk:2») + `state_hash` + расхождение с объектом |
| M2 | трофей волка `wolf-pelt → wolf-hide` | `TestNPCLootComesFromTheRules` («fixture … rules …») + `state_hash` + объект |
| M3 | `position` игрока `outside:… → dark-forest-01` | `TestRegionProjectionsFollowFromPositions` («players_present [], but the characters standing in … are [player-A]») + `TestPlayersStartOutsideInTheirOwnScope` |
| M4 | порядок сущностей в файле (`A,B,C → B,A,C`) | `TestFixtureSetIsTheWorldBootstrapCreates` (полный список have/want) + `applied_proposals`. `state_hash` намеренно **не** сдвигается — `entity.StateHash` сортирует сам; это верно |
| M5 | последний символ `state_hash` в указателе | `TestLatestPointerDescribesSnapshotZero` (file/recomputed рядом) + сверка блоков + сверка с объектом |
| M6 | `size_bytes 4531 → 4532` | «size_bytes 4532, but state/…json is 4531 bytes» |
| M7 | `entities_count 6 → 5` | «entities_count 5, but the fixtures hold 6 entities» |
| M8 | `laws_version v1 → v2` в указателе | «laws_version "v2", but the world entity says "v1"» |
| M9 | `scope` сокращением-строкой `"solo:player-A"` (в обоих файлах) | «scope is stored as string: an entity carries the object {id, type}» — именно тот подтест, ради которого он написан |
| M10 | лишний ключ в блоке `snapshot` указателя | строгий разбор («unknown field "extra_key"») + `TestPointerYieldsAValidSnapshotCreated` (схема закрыта) |
| M11a–c | удаление `state_hash` / `cursor` / `size_bytes` из указателя | по каждому: свой assert **и** отказ схемы `snapshot.created` |
| M12 | удаление `reason` (поле только указателя) | «latest.json lost reason: §4.4 keeps it in the pointer» + «reason "", want bootstrap» |
| M13 | `region_id` волка → несуществующий регион | `state_hash` (значение выведено из атрибутов) |
| M16 | лишний ключ в блоке `snapshot` объекта | строгий разбор объекта |
| M18 | `npc_ids → []` | «npc_ids [], but the NPCs standing in … are [wolf-alpha]» |
| M19 | переименован один `proposal_id` | `applied_proposals` (object/want) + `size_bytes` |

**Э-3. `state_hash` считается независимо — проверено в обе стороны.** Правка любого атрибута
сущности (M1, M2, M3, M13, M18) двигает **пересчитанный** хэш при неизменном файле → тест требует
править файл; правка хэша в файле при неизменных сущностях (M5) → тест требует вернуть. Значение
`sha256:0046b8…` не переписано из вывода, а воспроизводится `entity.StateHash` на каждом прогоне.

**Э-4 (Minor-1). Непойманная мутация — одна.** `taken_at` и `written_at` сдвинуты на
`2026-03-05T12:34:56Z` в указателе и объекте, ключ и имя файла оставлены прежними →
`ok multiverse-core.io/test/fixtures`. Разбор — в Minor-1.

**Э-5 (пункт «перенос строк»). Работает; `size_bytes` совпадает в обеих формах.**
Исходное дерево: `latest.json` 631 Б, объект 4531 Б — LF. После `git checkout` с
`core.autocrlf=true` те же файлы на диске: 651 Б и **4686 Б** (155 CR-байт), проверено
`perl -0777 -ne 'print scalar(()=/\r/g)'`. Набор при этом зелёный: `readLF` нормализует, и
`size_bytes: 4531` сверяется с LF-длиной. На Linux в CI файлы и так LF — то же число. Значит
`size_bytes` от машины не зависит (это и требовалось), но байты на диске зависят — Minor-2.

**Э-6. Privacy — зелёный, и по DoD «нет реальных имён» претензий нет.**
`go run ./cmd/mvctl privacy scan testdata/` → `no external identifiers in testdata/ (13 files read)`,
код 0. Отдельно проверил, что сканер этого и не мог поймать: по `scanner.go` правила ищут токены,
числовые внешние ID, `@handle`, значения за ключами `user_name/first_name/last_name/nick_name` и
адреса — атрибут `name` в его поле зрения не попадает, и кириллица под `[A-Za-z]` не подходит.
Поэтому оценил вручную: «Вася» (`player-A`), «Лена» (`player-B`), «Олег» (`player-C`),
«Альфа-волк» — дословно фикстуры требований (`requirements/user-stories.md`, строки 25–27, блок
«Общие для MVP-1 фикстуры»; `prd.md` строка 154). Это канон спецификации, а не персона: имена без
фамилий, не связаны ни с одним внешним идентификатором, все три персонажа — `actor_kind: ci`.
DoD «нет реальных имён» выполнен.

**Э-7. Пункт 6 — событие строится снятием трёх полей, схема закрыта.** Удаление любого из восьми
полей payload (`state_hash`, `cursor`, `size_bytes` — M11a–c) роняет `contracts.Validate`; удаление
любого из трёх полей-только-указателя (`reason` — M12; `rules_version`, `entities_count` —
тот же цикл на строках 603–608) роняет явную проверку «latest.json lost X». Схема
`snapshot.created.v1.json` на ветке не менялась.

**Э-8. Размещение теста и границы импортов — не ломает.**
`golangci-lint run ./test/...` → `0 issues`; `go build ./...`, `go vet ./...`, `gofmt -l test/` —
чисто. Пакет состоит только из `_test.go`, `go build ./...` его не собирает. `depguard` в
`.golangci.yml` описывает `**/shared/**` и `**/internal/**`; `test/**` не покрыт **ни одним**
правилом, поэтому импорт `internal/mechanics` оттуда разрешён — но разрешён по умолчанию, а не по
объявлению (см. «Предложения в бэклог»). Полный `go test -short -count=1 ./...` — 22 пакета `ok`,
включая `multiverse-core.io/test/fixtures`; регрессии в чужих пакетах нет.
`golangci-lint run ./...` по всему модулю как критерий не использовался — там живут зонды T-014.

### Предложения в бэклог (не требую в этой задаче)

1. **Правило `depguard` для `test/**`.** Сегодня каталог не покрыт ни одним правилом, и ловушка
   `internal-unlisted` до него не достаёт. Пока там один тест фикстур — это незаметно; с приходом
   `test/e2e/` (T-018) отсутствие границы станет настоящей дырой: e2e сможет импортировать любой
   контекст напрямую в обход шины. Нужно объявить намерение явно (разрешить `internal/*` тестам —
   и запретить остальное). Файл `.golangci.yml` — tech-lead#1.
2. **Сообщение об ошибке схемы.** `contracts.Validate` печатает
   `jsonschema validation failed with '…snapshot.created.v1.json#'` без указания поля и пути.
   При отладке фикстуры это заставляет гадать; причины у библиотеки есть, их стоит развернуть.
   Пакет `shared/contracts` — вне T-016.
3. **Один источник словарей `data-model.md` §3.1** (`weather`, `time_of_day`, `status`,
   `actor_kind`): сейчас они переписываются в каждый проверяющий тест. Кандидат — константы рядом
   с `shared/entity`.

### Риски и допущения

- **Линия связи «правила ↔ фикстуры» держится одним тестом.** Это отмечено и автором. После
  EPIC-002 (`internal/state/bootstrap_test.go`) и EPIC-003 (`npc_table.stats_ref`) проверок станет
  три, и они должны остаться согласованными; пока `test/fixtures` — единственный якорь.
- **`applied_proposals` и шаблон `bootstrap:{world}:{type}/{id}`** взяты из §4.10 буквально. Если
  EPIC-002 при реализации `Bootstrap` выберет другой шаблон, фикстура и тест правятся вместе —
  это не дефект, а зафиксированная связь.
- **Строки в рабочем дереве.** В ходе мутаций пять файлов фикстур откатывались через
  `git checkout --`, и при `core.autocrlf=true` вернулись в CRLF-форме (`region.json`, `npc.json`,
  `players.json`, `latest.json`, объект снапшота). Это ровно та форма, которую даёт штатный
  checkout на этой машине; **содержимого это не меняет**: `git diff` и `git diff --cached` по
  области пусты, индекс не трогался, набор зелёный в обеих формах (Э-5). Вернуть их в LF я не мог —
  правка файлов автора мне запрещена. Если хочется байт-в-байт исходное состояние, достаточно
  `git checkout` при `core.autocrlf=input` либо строки из Minor-2.
- **Зонд в `internal/mechanics` (Э-1, путь B) откачен полностью**: `types.go` и `rules.go`
  вернулись с `eol=lf` из `.gitattributes`, `git diff` по ним пуст, `grep -c Spd` → 0.
- **`-race` не прогонялся** (недоступен, как в T-014/T-015). Для теста без горутин это не пробел.
- **Кроме этой записи в `review.md` ничего не правил и не коммитил.** Файлы T-014
  (`shared/testkit/**`, `shared/eventbus/**`) не открывались; `dev-log.md` не трогал.

---

## T-014 · ревью #3 (итерация 3) · 2026-09-10 · code-reviewer#2 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, база `fd93a6a`, ревьюировались **изменения в индексе** по
итерации 3 в моей области: `shared/testkit/**` (`contract/contract.go`, `membus/membus.go`) и
`shared/eventbus/**` (в этой итерации не менялся — сверено по md5 `a5e77309…` с состоянием,
принятым в ревью #2). Запись `dev-log.md` «developer#2 · T-014 · итерация 3» прочитана целиком,
`dev-log.md` не правился. `internal/mechanics/**` и `rules/**` (T-015 принята) не открывались.
Решения оркестратора от 2026-09-10 по обоим ОВ итерации 3 (Minor-3 без якоря → T-395;
последовательное закрытие читателей → заявка system-architect) приняты и **не переоткрываются**.

Состояние дерева на момент прогонов совпадает с заявленным в `dev-log` §7:
`membus.go` `4b0a9a43…`, `membus_test.go` (contract) `3249e654…`,
`redpanda_integration_test.go` `c030a109…`, `contract.go` `8a281e57…`, `kafka.go` `a5e77309…`.

### Вердикт

**Вернуть. Ворота волны 1 закрытыми считать нельзя.** Critical: 0 · Major: 1 · Minor: 0 · Nit: 3.

Пять из шести пунктов итерации закрыты по существу и доказаны мутациями — Major-2 и Major-3
ревью #2 держатся жёстко (B1 — 8 красных из 8, B2 — 3 из 3, I — 3 из 3, J — 2 из 2), правка
`membus.go` по Minor-3 штатное поведение заглушки не изменила, цифры `dev-log` §10 сошлись
до десятой доли процента, набор на исправном коде не флакует ни разу за 33 прогона, в том числе
под нагрузкой. Риск ложного красного, который автор сам назвал главным (второе утверждение кейса
`Close`), **не подтвердился**: 0 ложных красных из 33.

Ворота остаются открытыми по одной причине — той же, что и в ревью #2: **якорь мутации H не
детерминирован**. Заявленные «8 прогонов из 8» я воспроизвести не смог: 35 полных прогонов под
мутацией H дали **6 зелёных** (17 %). Зелёные — не случайный шум, а прямое следствие конструкции
утверждения: допуск «одно событие в полёте» стоит 25 мс работы обработчика, а `Close` на брокере
длится от 20,6 мс до 193 мс, и в быстрой половине этого разброса мутант укладывается в допуск.
Механизм подтверждён зондом: все шесть зелёных прогонов имели `Close` ≤ 51,6 мс и
`after = before + 1`, все красные — `Close` ≥ 61 мс (`after ≥ before + 2`) либо зависание.

Это ровно то замечание, из-за которого задача возвращалась в прошлый раз, и цена та же: откат или
регрессия правки общего кода `kafka.go` (все подписки платформы) проходит CI примерно в одном
прогоне из шести. Объём исправления — тестовый, порядка 5–10 строк в одном кейсе (варианты ниже).

### Статус замечаний ревью #2

| # | Статус | Чем доказано |
|---|---|---|
| Major-1 (якорь мутации H — «монетка») | **не закрыт** | 35 прогонов под H: 29 красных, **6 зелёных**; заявленные 8 из 8 не воспроизводятся. Зонд показывает механизм: зелёные — при `Close` ≤ 51,6 мс, `after = before+1` (внутри допуска). См. Major-1 ниже |
| Major-2 (`Close` под припаркованной подпиской) | **закрыто** | две подписки в кейсе; мутация B1 (`membus.go:299`, припаркованная ветка → `ErrClosed`) — **8 красных из 8**, сообщение адресное: «Subscribe returned … for the subscription parked at the end of its topic»; ветка хвоста (B2) — 3 из 3 |
| Major-3 (дедуп проходит без дубля) | **закрыто** | счёт всех доставок `repeated.ID` до окна, требование `≥ 2`; мутация I (`SetChaos` — no-op) — 3 из 3 красных на `membus`, мутация J (`Duplicate` публикует один раз) — 2 из 2 на Redpanda |
| Minor-1 (`stopping` в `ReadRange`/`Tail`) | **закрыто решением оркестратора** | вынесено вместе с ОВ-5 (закрытие шины под `Tail`/`ReadRange`) в заявку architect#1; проверил, что мутация «Tail, припаркованная ветка → `ErrClosed`» по-прежнему оставляет набор зелёным (3 из 3) — это следствие того же неопределённого места C-01, новым требованием не делаю |
| Minor-2 (цифры расхождения под `Close` в заявку) | **закрыто** | `dev-log` §8 п. 3: 3 из 40 против 40 из 40, утверждение «дефекта там нет» снято |
| Minor-3 (`stopping` в путях dead letter) | **закрыто решением оркестратора** | правка внесена (три места), якоря нет — подтвердил мутацией M3 (откат всех трёх строк): набор зелёный 2 из 2; форма кейса выписана в `dev-log` §4, задача T-395 создана. Проверил, что правка ничего не сломала: `stopping(ctx)` в штатной работе тождественна `ctx.Err() != nil`, набор 22 зелёных прогона, unit-тесты `membus` зелёные |
| Minor-4 (коммит на отменённом контексте) | **закрыто** | `dev-log` §8 п. 4 + заявка `contract-change` в бридж-блок |
| Minor-5 (запас таймаута) | **закрыто** | срок кейса перевыдачи — один на два ожидания, `2 * Timeout`; бюджет красного прогона не вырос — проверено конструкцией (было 15 + 15 в том же кейсе) |
| Nit-1 (чтение после `Close`) | в бэклоге | не поднимаю |
| Nit-2 (граница кейса отмены) | **закрыто** | `atReturn >= backlog/2`; в кейсе `Close` граница строже — «не более одного сверх входа в `Close`» |

### Новые замечания

**Major-1 (повтор) · `shared/testkit/contract/contract.go:1008-1011` — допуск «одно событие в
полёте» стоит 25 мс, и мутант в него укладывается всякий раз, когда `Close` быстрее.**
Мутация H (снять наследование контекста цикла от `closing` в `kafka.go:172-175`, то есть состояние
кода до правки итерации 2) на моей машине даёт **6 зелёных прогонов из 35**. Зонд в кейсе
(логировал длительность `Close`, `before` и `after`) показывает, что это не шум:

```
мутация H, зелёные:  Close 20,6 мс → after=4 | Close 39,1 мс → after=4 | Close 51,6 мс → after=4
мутация H, красные:  Close 61,1 мс → after=5 | Close 79,5 мс → after=6 | Close 96,8 мс → after=6
                     + 7 прогонов «Close left the subscription on the backlog running after 15s»
исправный код:       Close 41–90,6 мс (6 прогонов) и 110–193 мс под нагрузкой (4) → after=before=3
```

Условие выживания мутанта выписывается точно: обречённый цикл разбирает хвост со скоростью
`perEvent = 25 мс` на событие всё то время, пока идёт `Close`; если `Close` укладывается примерно
в одно событие, прибавка равна допуску — и набор зелёный. Длительность `Close` на живом брокере я
намерил в диапазоне **20,6…193 мс** (в `dev-log` §1 заявлено 72–111 мс — это узкая выборка), то
есть порог допуска лежит внутри рабочего разброса, а не за его пределами. Вторая половина кейса
(зависание на коммите) остаётся вероятностной по той же причине, что и в ревью #2.

Отдельно: цифра «8 из 8» в `dev-log` не выглядит подгонкой — при вероятности обнаружения ~0,83
серия из восьми красных вполне обычна (≈22 %). Дело не в честности замера, а в его мощности:
восемь прогонов не отличают 1,0 от 0,83, а разница между ними — это как раз «якорь» против
«монетки с четырьмя гранями».

*Как исправить (любой из вариантов, оба тестовые):*
1. **Сдвинуть порог на порядок ниже наблюдаемого минимума `Close`.** Пауза обработчика после
   сигнального события — не 25 мс, а 2–3 мс (`perEvent` остаётся 25 мс только для первых
   `signalOn` событий, чтобы сигнал заведомо пришёлся на непустой хвост). Допуск «+1 событие»
   сохраняется, значит риск ложного красного не растёт, а мутанту для выживания нужен `Close`
   быстрее ~3 мс — в 7 раз быстрее самого быстрого из моих замеров. Хвост при этом стоит
   увеличить (200–400 событий), чтобы обработчик не догнал конец топика до сигнала.
2. **Убрать зависимость от длительности `Close` совсем**: обработчик сигнального события после
   сигнала блокируется до тех пор, пока кейс не отпустит его — уже после возврата `Close`; тогда
   утверждение становится точным равенством `after == before` без временного допуска. Требует
   проверки, что `kafka.Reader.Close()` не ждёт работающий обработчик (по моим замерам `Close`
   возвращается за 20–193 мс, пока цикл занят, — похоже, не ждёт, но это надо подтвердить).
3. **Признать, что контрактный набор не может быть якорем реализации, и перенести якорь в T-394**
   (собственный интеграционный тест адаптера в `shared/eventbus`): там сценарий «`Close` в момент
   коммита» повторяется N раз со свежей шиной, и при N = 20 вероятность пропустить откат падает до
   ~0,17²⁰. Тогда из `dev-log` надо убрать утверждение, что кейс `Close` держит правку `kafka.go`,
   и назвать T-394 блокирующей для ворот, а не «волной 1 рядом».

Мой выбор — вариант 1 как минимальный на итерацию 4 плюс вариант 3 как постоянный якорь.

**Nit-1 · `contract.go:1190-1197` (`stop`) — ожидание `<-s.done` ничем не ограничено.**
Десять кейсов останавливают подписку через `stop(t)`; если `Subscribe` зависнет (ровно тот класс
дефекта, ради которого ворота и существуют), кейс не упадёт по своему 15-секундному сроку, а
провисит до `go test -timeout 15m`. Это не меняет вывод по бюджету (`timeout-minutes: 25`
переживает), но означает, что «худший красный прогон = 20 × Timeout» верно только для тайм-аутов
ожидания событий, а не для зависшей подписки.
*Как исправить*: `select` с `testkit.After(Timeout)` в `stop`, как уже сделано в кейсе `Close`.

**Nit-2 · `contract.go:924` — припаркованная подписка берёт ту же группу, что и подписка на
хвосте.** Обе зовут `Subscribe(..., r.group, ...)` на разных топиках. На заглушке курсор ключуется
парой «топик + группа», и это безразлично; на брокере это два члена одной consumer group с разными
подписками, то есть вход второго вызывает ребалансировку первой. Работает (11 зелёных прогонов из
11, из них 4 под нагрузкой), но связь кейсу не нужна: `r.group + "-parked"` убирает её одной
строкой.

**Nit-3 · `dev-log` §12 — «первый кандидат на флак по-прежнему `AnUncommittedEventIsDeliveredAgain`»
разошлось с замером после перекройки кейса.** Под нагрузкой (`GOMAXPROCS=1` + 16 нагрузчиков)
самый долгий кейс набора теперь `CloseStopsTheSubscriptionsAndRefusesToPublish` — 5,02 / 5,17 /
6,09 / 6,30 с в четырёх прогонах (кейс перевыдачи в тройку самых долгих не попал ни разу, < 2,1 с).
Причина понятна: две подписки, вход в группу дважды и ребалансировка (см. Nit-2). Запас до
`Timeout` = 15 с остаётся, но кандидат на первый флак в CI сменился, и в записи это стоит поправить.

### Что проверено экспериментом (машина владельца, `GOFLAGS=-buildvcs=false`, Docker 29.6.1, Redpanda v26.1.17, `-race` недоступен)

**1. Якорь мутации H — своей мутацией, 35 полных прогонов + 6 одиночных.**
Мутация: удалены `kafka.go:172-175` (`ctx, cancel := context.WithCancel(ctx)`,
`context.AfterFunc(k.closing, cancel)` и оба `defer`) — состояние до правки итерации 2.

| Форма прогона | Прогонов | Красных | Зелёных |
|---|---|---|---|
| `-run TestBusContractOnRedpanda`, серия A | 10 | 7 | **3** |
| то же, с моим зондом (полный набор) | 6 | 3 | **3** |
| весь пакет, как гоняет CI (`go test -tags integration ./shared/testkit/contract/`) | 13 | 13 | 0 |
| `-run TestBusContractOnRedpanda`, серия B (позже) | 6 | 6 | 0 |
| одиночный кейс `-run …/CloseStops…` | 6 | 6 | 0 |
| **итого полных** | **35** | **29** | **6 (17 %)** |

Зелёные пришлись на один участок сессии и объясняются не формой запуска, а быстрым `Close`
(20,6–51,6 мс против 61–97 мс в красных). Серии «13 из 13» и «6 из 6» не опровергают вывод:
они показывают, что обнаружение зависит от состояния машины, а не от кода.

**2. Ложные красные на исправном коде — не найдены.**

| Прогон | Прогонов | Результат |
|---|---|---|
| `membus`, набор целиком | 12 | 12 зелёных, 1,40–1,58 с |
| `membus`, `GOMAXPROCS=1` + 16 нагрузчиков | 10 | 10 зелёных, 1,48–1,61 с, `before=after=4` во всех |
| Redpanda, набор целиком | 7 | 7 зелёных, 11,0–16,9 с, `before=after=3` во всех |
| Redpanda, `GOMAXPROCS=1` + 16 нагрузчиков | 4 | 4 зелёных, 35,3–39,7 с, `Close` 110–193 мс, `before=after=3` |
| `-tags integration ./shared/testkit/... ./shared/eventbus/...` | 2 подряд | зелёные, 14,2 с и 15,3 с |
| `go test -short -count=1 ./...` | 1 | 22 пакета `ok`, 5,7 с |

Заявленный автором риск (25 мс между чтением счётчика и остановкой читателей) в 33 прогонах, в том
числе под нагрузкой, не реализовался ни разу: `after` не превысил `before` ни в одном прогоне.

**3. Мутации по Major-2, Major-3 и правке Minor-3** (после каждой файл восстановлен из копии, md5
сверены):

| # | Мутация | Прогонов | Красных | Чем упало |
|---|---|---|---|---|
| B1 | `membus.go:299` — припаркованная ветка `Subscribe` возвращает `ErrClosed` (мутация 6 ревью #1, дважды оставлявшая набор зелёным) | 8 | **8** | «Subscribe returned eventbus: bus is closed for the subscription parked at the end of its topic, want nil» |
| B2 | `membus.go:284` — `stopping` в начале цикла → только `ctx.Err()` | 3 | **3** | «the handler was called 37 more times … (40 of the 40 events of the backlog in all)» |
| I | `membus.SetChaos` — no-op | 3 | **3** | «the repeated event … was delivered 1 time(s)» |
| J | `redpanda_integration_test.go` — `Duplicate` публикует один раз | 2 | **2** | то же сообщение для `redpanda` |
| B1T | `membus.go` — припаркованная ветка **`Tail`** возвращает `ErrClosed` | 3 | 0 | набор зелёный — та же неопределённость C-01, что и ОВ-5 (решение оркестратора, не поднимаю) |
| M3 | откат всех трёх `stopping` в путях dead letter | 2 | 0 | набор зелёный — совпадает с §4 `dev-log`, решение оркестратора (T-395) |

**4. Правка `membus.go` (три `stopping`) ничего не сломала.** `stopping(ctx)` в штатной работе
тождественна `ctx.Err() != nil` (второй ветвью служит закрытый `b.st.done`), различие проявляется
только после `Close`. Подтверждено прогонами: набор на `membus` 22 из 22 зелёных, unit-тесты
`shared/testkit/membus` зелёные, `go test -short ./...` — 22 пакета `ok`, `go vet` в обоих режимах
тегов чист.

**5. Честность цифр `dev-log` §10.**

| Заявлено | Намерено мной |
|---|---|
| покрытие `contract` 81,4 % | **81,4 %** |
| `testkit` 74,0 %, `membus` 83,8 %, `eventbus` 75,4 % | те же |
| 20/20 на обеих реализациях | 20/20 и 20/20 |
| `integration` зелёный дважды подряд | зелёный дважды (14,2 с и 15,3 с) |
| `golangci-lint run ./shared/...` и `--build-tags integration,e2e` — 0 issues | **0 issues** в обоих режимах (v2.13.2) |
| `gofmt -l shared/` пусто | пусто |
| «бюджет красного прогона не вырос» | верно: кейс перевыдачи и раньше стоил 15 + 15 с |

Бюджет худшего красного пересчитан: 18 кейсов × 15 с + 30 с (перевыдача) + 30 с (`Close`: сигнал
15 с + первое ожидание возврата 15 с) ≈ **5,5 мин на реализацию**, обе половины идут одним пакетом
→ **≈ 11 мин**. Укладывается в `go test -timeout 15m` и `timeout-minutes: 25`
(`.github/workflows/go.yml:146,181`), по-прежнему выше ориентира ADR-010 п. 5 «≤ 10 мин на job»
(замечание ревью #2 не переоткрываю, менять не прошу). Оговорка — Nit-1: зависшая подписка стоит
не 15 с, а полного `-timeout`.

**6. Новых дыр от перекройки кейса `Close` не нашёл**, кроме перечисленных Nit-2 и Nit-3:
припаркованная подписка действительно припаркована к моменту `Close` (маркер дождался обработки,
дальше идёт секунда работы с хвостом), обе ветки требуют `nil` и обе закреплены мутациями (B1 и
B2), `Publish → ErrClosed` после `Close` на месте, порядок кейсов сохранён (`Close` последний),
`t.Skip` для таргета без `Close` не тронут.

### Зонды и чистота дерева

Мои зонды — временные `t.Logf` в `closeStopsEverything` (длительность `Close`, `before`/`after`) и
шесть мутаций — удалены, файлы восстановлены из резервных копий, md5 сверены:
`contract.go` `8a281e57…`, `membus.go` `4b0a9a43…`, `kafka.go` `a5e77309…`,
`redpanda_integration_test.go` `c030a109…`, `membus_test.go` (contract) `3249e654…`.
`git diff -- shared/` пуст, неотслеживаемых файлов в `shared/` и `internal/` нет. Кроме этой записи
в `review.md` ничего не правил и не коммитил.

### Риски и допущения

- **Соотношение 6 зелёных из 35 — оценка, а не константа.** Оно зависит от того, как быстро
  `k.Close()` покидает consumer group на конкретной машине. На раннере GitHub (2–4 vCPU, Docker
  там же) `Close`, скорее всего, медленнее, и мутант будет ловиться чаще — но «скорее всего» и есть
  предмет замечания: якорь обязан падать всегда, а не чаще.
- **`-race` не прогонялся** (недоступен). Новое разделяемое состояние итерации 3 — канал
  `returning` и `atomic.Int64` в двух кейсах; под `-race` в CI смотреть на них.
- **Эмуляция медленного раннера** (`GOMAXPROCS=1` + 16 нагрузчиков на 32 ядрах) не воспроизводит
  раннер GitHub: у меня узкое место — CPU процесса теста, у раннера ещё и диск с сетью брокера.
- **Область — только файлы T-014 в `shared/testkit` и `shared/eventbus`.**
  `.github/workflows/go.yml` открывался на чтение ради бюджета красного прогона; замечаний по нему
  нет. Параллельные изменения других агентов (`test/fixtures`, `testdata/fixtures`,
  `internal/mechanics`) на мои прогоны не влияли: пакеты независимы, `go test -short ./...`
  зелёный.

---

## T-017 · ревью #1 · 2026-09-10 · code-reviewer#1

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `fd93a6a`, коммита по T-017 нет — ревьюировались изменения
в индексе: `shared/testkit/state/**` (8 файлов), `shared/testkit/mechanics/**` (3 файла),
3 305 добавленных строк, плюс правка `.golangci.yml`, сделанная оркестратором (правило
`shared-testkit-mechanics`).

**Вне области и не рассматривалось**: `shared/testkit/{contract,membus}/**` и `shared/eventbus/kafka.go`
(T-014, developer#2, в работе), `testdata/fixtures/**` и `test/fixtures` (T-016), `internal/mechanics`
и `rules/` (T-015), `.gitattributes`, `Docs/dev-team/*` кроме этой записи. `dev-log.md` не правился.

Основание: `tasks.md` T-017 (описание, дополнение «сведение 3», DoD) и §1 (общий DoD);
`design.md` §5, §5.1, §11; `state-and-mechanics.md` §4.5, §5.1, §7.1; `contracts.md` v0.4 —
C-02 v1.2, C-03 v1.1, C-14 (уточнение v0.4), §0, §17; `consolidation.md` §14.1 (TL2-6, З-2);
`schemas/events/{entity.create.proposed,entity.update.proposed,entity.created,entity.updated,entity.update.rejected,analytics.replay.completed,snapshot.created}.v1.json`;
запись T-017 в `dev-log.md`; три последние записи `journal.md`.

### Вердикт

**Вернуть.** Critical 0 · **Major 1** · Minor 5 · Nit 3.

Работа сделана по существу правильно: подмена C-03 доказана компилятором и прогоном одного
потребителя на обеих реализациях, матрица из пяти причин держится мутациями поштучно, `atomic=true`
не двигает мир вообще, сигнал старта валидируется реестром, а не тестом. Возврат — из-за одного
места, где заглушка молча издаёт факты, которые контракт запрещает (Major-1), и потому что три
обещания («указатель после объекта», «сигнал после подписки», «`mode=recovery`») в тестах не
закреплены и переживают мутацию.

### Замечания по серьёзности

#### Major-1 · `shared/testkit/state/apply.go:169-205`, `:237-242` — два набора изменений на одну сущность в одном предложении: первый молча теряется, версия не двигается, фактов два

`evaluate` для каждого `changes[i]` читает **оригинал** (`apply.go:242` — `current, ok := s.entities[ref.ID]`),
а не результат предыдущего набора; `changedBy` (`apply.go:179`) индексируется по `id`, поэтому
второй набор затирает первый; в цикле фиксации (`apply.go:191-202`) в мир кладутся обе копии, и
последняя выигрывает. Обе копии клонированы из одного оригинала, поэтому `Commit` даёт им **одну и
ту же версию**, и на шину уходят два `entity.updated` для одной сущности с равной `version` и
противоречивыми `changed[].old`.

Доказано зондом (файл удалён): пакет `atomic=true` с двумя наборами по `player-A`
(`inc hp -1` и `inc hp -2`, `hp` = 10, `version` = 1) дал
`hp=8 version=2 facts=2 fact versions=[2 2] refusals=0` — вместо 7 при последовательном применении
или одного отказа. Первый факт объявляет `hp: 10 → 9`, второй `hp: 10 → 8`; проекция потребителя,
сверяющая `old` со своим значением, увидит расхождение уже на втором факте.

Это нарушает C-02 («`version` строго +1 на сущность при непустом `changed[]`») и `state-and-mechanics.md`
§4.5 п. 12 («по одному `entity.updated` **на сущность**»). Предложение проходит реестр: в
`schemas/events/entity.update.proposed.v1.json` у `changes` нет ограничения уникальности —
проверено зондом `contracts.Validate` («the registry accepts two change sets for one entity»),
то есть ветка достижима через шину, а не только через прямой `Apply`.

Как чинить (любой из двух, оба дешёвые):

1. отказ `invalid_op`, если `changes[]` называет один `entity.id` дважды — v0 честно говорит
   «сначала слей операции в один набор», и это ровно та причина, которую §4.5 п. 1 даёт формату;
2. либо применять наборы последовательно на накопленных копиях (`copies map[string]*entity.Entity`
   вместо чтения `s.entities` в `evaluate`) и издавать один факт на сущность.

Плюс тест: пакет из двух наборов по одной сущности → либо один отказ, либо один факт с `version+1`.

#### Minor-1 · `shared/testkit/state/snapshot.go:150-174`, `snapshot_test.go:29-37` — порядок «объект, потом указатель» заявлен именем теста, но не проверяется

Мутация «поменять местами `Put(meta.Key, body)` и `Put(PointerKey, pointerBody)`» оставляет пакет
зелёным (`ok multiverse-core.io/shared/testkit/state 0.578s`), хотя `TestSnapshotWritesTheObjectAndThenThePointer`
в комментарии обещает именно эту гарантию C-14 («указатель пишется после успешного PUT объекта»),
а тест проверяет только, что оба файла есть и указатель называет свежий ключ. Заглушка, пишущая
указатель первым, отдаёт потребителю висячую ссылку.

Как чинить: `Config.Store` — интерфейс `objstore.Client`, поэтому в тесте достаточно декоратора,
записывающего порядок ключей в `Put`, и утверждения `keys == [meta.Key, PointerKey]`.

#### Minor-2 · `shared/testkit/state/state.go:236-264` — «подписался → просигналил» не закреплено тестом и не гарантировано кодом

Мутация «перенести `announceRecovery` перед запуском горутины подписки» зелёная. Сам код тоже не
даёт заявленного порядка: `close(ready)` (`state.go:250`) выполняется **до** вызова
`bus.Subscribe` (`state.go:251`), поэтому `Start` возвращает управление и публикует сигнал, когда
подписка ещё может быть не зарегистрирована.

Практического ущерба на `membus` нет — проверено зондом: предложение, опубликованное **до** `Start`,
всё равно применяется (группа читает с первого офсета). Но DoD и комментарий утверждают порядок,
которого в коде нет; на шине, где новая группа встаёт в конец журнала, потребитель, публикующий
предложение сразу по сигналу, потерял бы первое.

Как чинить: не обещать в комментарии того, что не проверяется. Закрепить то, что действительно
держит контракт («ничего, опубликованное до `Start`, не теряется») — тестом вроде зонда выше, и
переформулировать комментарий `state.go:230-235`.

#### Minor-3 · `shared/testkit/state/state_test.go:191` и `state.go:314` — `mode` сверяется с собственной константой

`if mode, _ := payload.GetString("mode"); mode != state.ReplayModeRecovery` — утверждение
тавтологично: мутация `const ReplayModeRecovery = "test"` оставляет пакет зелёным
(`ok … 0.683s`), потому что схема (`analytics.replay.completed.v1.json`, `enum: [recovery, test]`)
допускает оба значения, а тест сравнивает константу с самой собой. При этом потребители ждут
именно `mode=recovery` (C-14 v0.4, `MV_SWARM_REPLAY_WAIT` → `/health degraded {state_replay: missing}`).

Как чинить: сверять с литералом `"recovery"` — контрактное значение, а не имя константы.

#### Minor-4 · `shared/testkit/state/apply.go:29-43` — предложения чужого мира применяются к своему

`Apply` не смотрит на `world.entity.id` события, хотя настоящий State — воркер **на мир**
(`state-and-mechanics.md` §7.1, участник «state (worker dark-forest-world)»). Зонд: предложение с
`world = some-other-world` по `player-A` применено (`hp=5`), а факт ушёл под чужим миром
(`fact worlds=[some-other-world]`). Для e2e T-018 с одним миром безвредно; два мира на одной
`membus` дадут перекрёстное применение и факты, которые проекция другого мира примет за свои.

Как чинить: в `Apply` пропускать (лог `debug`) предложения, у которых `world.entity.id` ≠ `Config.WorldID`.

#### Minor-5 · `shared/testkit/state/apply.go:256-262` — у перехода `alive → abandoned` не проверяется ни `cause`, ни отсутствие `meta.agent`; в отчёте названа только половина зазора

Зонд: предложение от `source=swarm` с `meta.agent{level: scope}` и `cause=combat`, ставящее
`status=abandoned`, принято (`status="abandoned", facts=1, refusals=0`), и факт ушёл с
`cause: combat`. C-02 v1.2 и З-2 говорят, что переход публикует **только** gateway и **только**
`cause=forget`. Проверка источника — это `level_violation`, вне матрицы v0, и dev-log §3 её честно
называет; про `cause` не сказано ни в dev-log, ни в комментарии пакета, а потребитель (рой), по
ошибке предложивший `abandoned` с `cause=combat`, на заглушке пройдёт и на настоящем State упрётся.

Отдельно: `entity.StatusTransitionAllowed` (`shared/entity/types.go:57`), добавленный T-011 ровно
под это правило и документирующий развилку «`dead_entity` или `level_violation` — решает State»,
заглушкой не используется (`grep` даёт только определение и тесты `shared/entity`).

Как чинить (документарно, без расширения матрицы): назвать зазор в `dev-log.md` §3 и в
doc-комментарии `apply.go` рядом с проверкой терминального статуса — «`cause` и издатель не
проверяются, это `level_violation` настоящего State».

#### Nit-1 · противоречие документов о месте двойника закрыто наполовину

`design.md` §5 и `tasks.md` T-017 говорят `shared/testkit/mechanics`, `contracts.md` §17 —
`testkit/state.FixedMechanics`, а `ownership.md` §1 перечисляет подпакеты владельцев как
`shared/testkit/state`, `…/swarm`, `…/gateway` и `testkit/mechanics` не называет вовсе. Dev-log §5
п. 1 ссылается на «задачу и карту владения» — карта выбранный путь не подтверждает. Выбор пути
правильный (см. раздел о границе импортов), но в открытые вопросы надо добавить `ownership.md` §1
рядом с `contracts.md` §17.

#### Nit-2 · `shared/testkit/mechanics/fixed.go:89-94` — таблица исходов не закреплена ни одним золотым вектором

Мутация «повернуть таблицу» (те же доли, другой порядок строк) зелёная: и утверждения, и реализация
читают один и тот же `slot`, а доли при повороте сохраняются. Формально дизайн фиксирует только
доли (60/10/10/20), и мутация «5 попаданий / 3 промаха» краснеет, так что дыры в требованиях нет.
Но записанный сценарий T-018 («solo-30», 30 `narrative.output`) при перестановке строк молча
станет другим. Достаточно одной пары в тесте: `Verdict("ev-0", 0) == "…"`.

#### Nit-3 · `shared/testkit/state/consumer_test.go:154-157` — семь причин C-02 записаны литералом

Список контрактных причин продублирован в тесте, тогда как он есть в схеме
(`entity.update.rejected.v1.json`, `reason.enum`) и доступен через `contracts.Lookup(...).Schema`.
При расширении enum список разъедется молча.

### Что проверено экспериментом

Все прогоны — `GOFLAGS=-buildvcs=false`, Go 1.26.8, `-race` недоступен.

**Базовый прогон.** `go test -short -count=1 ./...` — зелёный целиком, включая `shared/testkit/contract`.
`go test -short -count=1 -cover ./shared/testkit/{state,mechanics}/...` → **87,2 %** и **93,2 %**
(цифры dev-log подтверждены). `golangci-lint run ./...` (v2.13.2) — **0 issues**; `gofmt -l` по обоим
каталогам пуст.

**1. Подмена C-03 доказана компилятором.** Интерфейс `Mechanics` объявлен на стороне потребителя
(`shared/testkit/mechanics/consumer_test.go:23-30`, пакет `mechanics_test`), а не в пакете заглушки;
оба утверждения времени сборки на месте (`:34-35`). Расхождение проверено двумя мутациями:
`Stats(kind string)` → `Stats(kind string, _ int)` даёт
`*FixedMechanics does not implement Mechanics (wrong type for method Stats)`; `Resolve`, потерявший
`[]mech.Roll`, ломает сборку самого пакета. `TestOneConsumerDrivesBothImplementations` действительно
гоняет одну функцию `encounterTurn` на заглушке и на `mech.Load(rules/dark-forest.yaml)` — на второй
приходит `ErrNotImplemented`. Набор методов покрывает `Resolve/NPCTarget/Roll/Stats/Invariants`
(`design.md` §5).

**Тест-потребитель C-02.** Проекция (`shared/testkit/state/consumer_test.go:30-63`) построена только
на типах реестра и `event.Path()`, ни одного имени заглушки в её теле нет; импорт
`shared/testkit/state` в файле есть и неизбежен (внешний тест-пакет, утверждения ссылаются на
`state.ReasonVersionConflict`) — формулировку dev-log «ни одним именем не упоминает `testkit`»
читаю как относящуюся к проекции; так оно и есть.

**2. Матрица отказов — по мутации на причину, каждая красит ожидаемый тест.**

| Мутация | Результат |
|---|---|
| `IsTerminal()` выключен | FAIL `TestRejectionMatrix/dead_entity`, `TestForgetAbandonsALivingCharacterOnce`, `TestForgetOverADeadCharacterIsRefused` |
| `abandoned` перестал быть терминальным (`dead`/`ascended_final` остались) | FAIL `TestForgetAbandonsALivingCharacterOnce` (повторный `/forget` прошёл) |
| `CheckVersion` не вызывается | FAIL `TestRejectionMatrix/version_conflict`, `TestARefusedProposalIsNotRemembered`, `TestAConsumerBuildsItsProjectionFromTheStub` |
| `unknown_entity` → `invalid_op` | FAIL `TestRejectionMatrix/unknown_entity` |
| проверка существования при создании выключена | FAIL `TestRejectionMatrix/duplicate_entity` |
| ошибка `ApplyOps` → `dead_entity` вместо `invalid_op` | FAIL `TestRejectionMatrix/invalid_op` |

Переход `alive → abandoned` проверен отдельно: из `alive` — один `entity.updated`
`changed:[{status, alive, abandoned}] cause=forget` и ноль `narrative.output`; повтор и `dead` —
`rejected reason=dead_entity` (тесты `apply_test.go:129-184`, обе мутации выше их валят).
Исключение «четыре пути трупа» (§4.5 п. 5) реализовано и покрыто.

**3. `atomic=true` откатывает пакет целиком.** Мутация «класть копию в мир по мере обхода»
(`s.entities[copyOf.ID] = copyOf` внутри цикла) → FAIL `TestAtomicPackageIsAllOrNothing`. Тест
устроен правильно: два годных набора идут **до** сломанного, сверяется `StateHash` до и после,
`facts == 0` и `refusals == 1`. «Ровно один отказ» держится структурно — `break` на первой
проблеме (`apply.go:173-176`). Порядок фактов по возрастанию `id` закреплён: инверсия компаратора
→ FAIL `TestFactsComeOutInIdentifierOrder`.

**4. Протокол старта валидируется реестром, а не тестом.** Мутация «подменить обязательное поле
`incomplete_record` на неизвестное `unexpected_field`» валит пять тестов сразу, включая
`TestStartTwiceIsRefused`, — то есть событие не публикуется вовсе: `membus` проверяет схему на
записи, `Start` возвращает ошибку. `TestStartAnnouncesRecovery` дополнительно сверяет
`source ∈ Spec.Publishers` и `causation_id == ""`. Найденные зазоры — Minor-2 (порядок) и Minor-3
(`mode`).

**6. Детерминизм и согласованность кости.** `TestResolveIsDeterministic` сравнивает два независимо
загруженных набора правил — исход и броски совпадают. Доли: мутация «5 попаданий / 3 промаха» →
FAIL `TestOutcomeTableHasTheSharesOfTheDesign`; поворот таблицы (доли те же) — зелёный (Nit-2).
Согласованность натуральной кости с вердиктом закреплена: обмен формул `naturalFor` для попадания
и промаха → FAIL `TestResolveAttackFollowsTheTable/{hit,miss}`; игнор `crit_multiplier` → FAIL
`TestCriticalDoublesTheDamage`; бросок урона по индексу попадания вместо `idx+1` → FAIL
`.../{hit,critical}`; `NPCTarget`, перестающий отсеивать `Excluded`, → FAIL
`TestNPCTargetIsTheFirstLivingByID`.

**7. Подгонки тестов под реализацию не нашёл, кроме перечисленного.** Переименование
`Component = "state"` → `"swarm"` краснеет (`TestSnapshotWritesTheObjectAndThenThePointer`).
Не закреплены: порядок записи объекта и указателя (Minor-1), порядок «подписка → сигнал» (Minor-2),
значение `mode` (Minor-3), порядок строк таблицы (Nit-2).

**Зонды и чистота дерева.** Все зонды удалены: пять временных `zz_probe*.go` (в `shared/objstore`,
`shared/testkit/{mechanics,state}`, `internal/{state,gateway}`), два временных `*_test.go` в
`shared/testkit/state`, каталоги `internal/state` и `internal/gateway` (созданы под зонд) удалены —
в `internal/` снова только `mechanics`. Копия конфига линтера для проверки рекомендации лежала вне
репозитория. `git diff -- shared/testkit/state shared/testkit/mechanics .golangci.yml` пуст,
неотслеживаемых файлов в `shared/` и `internal/` нет, состав `git status` совпадает со снимком на
начало ревью. Кроме этой записи в `review.md` ничего не правил и не коммитил.

### Оценка правки границы импортов (`.golangci.yml`)

**Правка верна и минимальна, граница для остальных `shared/*` цела.** Проверено зондами
(файлы удалены):

| Зонд | Ответ линтера |
|---|---|
| `shared/objstore` → `internal/mechanics` | отвергнут правилом `shared`, текст прежний |
| `shared/testkit/state` → `internal/mechanics` | отвергнут правилом `shared` |
| `shared/testkit/mechanics` → `internal/state` | отвергнут правилом `shared-testkit-mechanics` |
| `shared/testkit/mechanics` → `internal/gateway` | отвергнут правилом `shared-testkit-mechanics` |
| `shared/testkit/mechanics` → `internal/mechanics` (как есть) | 0 issues |

То есть исключение по каталогу не превратилось в дыру в слое: двойник видит ровно один контекст.
Выбор каталога (`shared/testkit/mechanics`, а не `shared/testkit/state`, как в `contracts.md` §17)
для границы **строго лучше**: во втором случае исключение накрыло бы и `FakeState`, и дыра была бы
шире на целый пакет.

**Остаточный риск из журнала подтверждён экспериментально и достижим.** Файл
`internal/gateway/zz_probe.go` (production, не `_test.go`), импортирующий
`shared/testkit/mechanics` и вызывающий `m.Stats("wolf")`, даёт **0 issues**, тогда как прямой
импорт `internal/mechanics` из того же файла отвергается правилом `internal-gateway`. Контекст
действительно дотягивается до чужой библиотеки через двойник, и линтер молчит.

**Рекомендация архитектору: защита нужна, и она уже предписана.** `ownership.md` §1 (строка про
`cmd/multiverse/fake_contexts.go`) говорит: это «**единственный** импорт `shared/testkit` в
production-бинарнике (`depguard`-исключение по файлу)» — значит запрет подразумевался, просто не
записан. Выражается он штатными средствами, без переноса пакетов и без правки C-03:

```yaml
# в linters.settings.depguard.rules
no-testkit-in-production:
  files:
    - "**/internal/**"
    - "**/cmd/**"
    - "!**/cmd/multiverse/fake_contexts.go"
  deny:
    - pkg: multiverse-core.io/shared/testkit
      desc: "тестовые двойники — для тестов (ownership.md §1)"

# в issues.exclusions.rules — тестов запрет не касается
- path: _test\.go$
  linters: [depguard]
  text: multiverse-core.io/shared/testkit
```

Проверено на копии конфига (в репозитории `.golangci.yml` не менялся):
`golangci-lint run --config <копия> ./internal/... ./shared/... ./cmd/...` даёт ровно одну находку —
production-зонд в `internal/gateway`; тестовый файл того же пакета, импортирующий тот же двойник,
исключением снят; на остальном дереве находок нет. То есть возражение «depguard не умеет отличать
тестовые файлы» снимается парой «правило + исключение по пути», как уже сделано для
`shared/(clock|env)` и `forbidigo`.

Кому: architect#1 (ратификация, ADR-001 доп. пункт), tech-lead#1 + devops (правка файла).
Замечанием к T-017 это не считаю — файл вне владения исполнителя.

### Открытые вопросы (в бридж-блок волны 1)

1. Шесть вопросов dev-log §6 подтверждаю, к п. 3 добавить `ownership.md` §1 (Nit-1).
2. Нужно ли State отказывать, когда `changes[]` называет одну сущность дважды, и какой причиной
   (`invalid_op` или новая) — вопрос не только заглушки: T-056 столкнётся с тем же (Major-1).
3. Схема `entity.update.proposed.v1.json` не запрещает повтор сущности в `changes[]`; если ответ на
   п. 2 — «отказывать», дешевле закрыть это в схеме, а не в каждой реализации.

### Риски и допущения

- **`-race` не прогонялся** (недоступен). Новое разделяемое состояние: `FakeState.mu` вокруг
  `entities`/`cursor`/`applied`, `subscription sync.WaitGroup` и `subErr`. Публикация фактов идёт
  вне замка (`apply.go:208-225`), а `SetFactEventID` — под замком, но по указателю, уже видимому
  через `Get`; при одном подписчике это безопасно, под `-race` в CI посмотреть.
- **`shared/testkit/{contract,membus}` не открывались** (T-014, developer#2). Поэтому утверждение
  «группа читает с первого офсета» проверялось поведенчески (зонд с публикацией до `Start`), а не
  по коду `membus`.
- Заявленные dev-log цифры (покрытие, число подмен) проверялись выборочно: покрытие пересчитано,
  из двенадцати заявленных подмен воспроизведено шесть по матрице отказов и шесть по механике —
  все ведут себя как заявлено.
- Оценка Major-1 опирается на то, что `changes[]` с повтором сущности проходит реестр (проверено) и
  что настоящий State по §4.5 п. 12 обязан издавать один факт на сущность. Если архитектор решит,
  что такое предложение невозможно по построению у всех издателей, замечание падает до Minor
  (тогда всё равно нужен отказ, а не молчаливая потеря).

## T-017 · ревью #2 (итерация 2) · 2026-09-10 · code-reviewer#1

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `fd93a6a`, коммита по T-017 по-прежнему нет — ревьюировались
изменения в индексе: `shared/testkit/state/**` (8 файлов) и `shared/testkit/mechanics/**` (3 файла),
3 760 добавленных строк против 3 305 на ревью #1. Итерация повторная, поэтому проверялись только
исправления по восьми замечаниям ревью #1 и регрессия от них.

**Вне области и не открывалось:** `shared/testkit/{contract,membus}/**` (T-014, developer#2 в
работе) — про `membus` в этой записи есть только утверждения, полученные поведенчески (проверка
типа и прогон), не чтением его кода. `shared/eventbus/**` читался, но не правился. `dev-log.md`,
`journal.md`, `ownership.md`, `.golangci.yml` не правились.

Основание: раздел «T-017 · ревью #1» выше; запись `dev-log.md` «developer#1 · T-017 · итерация 2 по
ревью #1»; три последние записи `journal.md` (решения оркестратора приняты и не переоткрываются);
`shared/eventbus/{bus.go,kafka.go}`; `shared/entity/types.go`;
`schemas/events/entity.update.rejected.v1.json`.

### Вердикт

**Принять.** Critical 0 · **Major 0** · Minor 3 · Nit 2.

Major-1 закрыт и закрыт правильно: повтор сущности в одном предложении отвергается ровно одним
`invalid_op` до взятия замка, в обоих режимах, мир не двигается, фактов ноль, предложение не
запоминается — и законный пакет из трёх разных сущностей по-прежнему проходит целиком. Minor-1,
Minor-3, Minor-4, Minor-5, Nit-2, Nit-3 закрыты и держатся мутациями. Ни одна из шести мутаций
ревью #1 не позеленела — прежние тесты не ослабли, объём проверок вырос.

Minor-2 закрыт **не буквально и не полностью**, и это осознанное отклонение, а не пропуск: строгий
порядок «группа зарегистрирована → сигнал» через `eventbus.Bus` действительно невыразим, решение
хуже не сделало, а грубый порядок теперь закреплён. Но два обещания итерации 2 вокруг него не
подтвердились экспериментом (новые Minor-6 и Minor-7), и движение курсора над чужим миром не
закреплено ни одним тестом (Minor-8). Ни одно из трёх не ломает поведение и не отклоняется от
дизайна, поэтому возврата нет — правки уходят в довесок к T-018 или в бэклог.

### Статус замечаний ревью #1

| Замечание | Статус | Чем доказано |
|---|---|---|
| **Major-1** · два набора на одну сущность | **закрыто** | мой зонд, повтор прежнего: `atomic` и не-`atomic` — `hp 10→10, version 1→1, facts=0, refusals=1 (invalid_op, player-A), hash-moved=false, applied=[]`. Обратная проверка: пакет из трёх разных сущностей — `facts=3`, версии 2/2/2, отказов 0. Мутация «снять `repeatedEntity`» валит `TestOneEntityNamedTwiceIsRefused/{atomic,loose}` |
| **Minor-1** · порядок «объект, потом указатель» | **закрыто** | декоратор `putRecorder` (`snapshot_test.go:133-172`) смотрит на порядок ключей изнутри хранилища. Мутация «переставить два `Put` в `snapshot.go`» валит `TestTheObjectIsWrittenBeforeThePointer` (на ревью #1 та же мутация была зелёной) |
| **Minor-2** · «подписался → просигналил» | **закрыто частично, отклонение обосновано** | см. отдельный раздел ниже. Мутация ревью #1 («анонс перед запуском горутины») теперь красит тест 30 раз из 30; мутация «не ждать отчёта шины» — 2 раза из 100 (Minor-6) |
| **Minor-3** · `mode` сверялся с собственной константой | **закрыто** | `state_test.go:214-222`: сравнение с литералом `"recovery"` плюс отдельная строка про саму константу. Мутация `ReplayModeRecovery = "test"` валит `TestStartAnnouncesRecovery` |
| **Minor-4** · предложения чужого мира | **закрыто** (курсор — с оговоркой) | `apply.go:41-45`. Зонды: `update` и `create` чужого мира — `facts=0 refusals=0`, сущность в мир не попала, `StateHash` не сдвинулся, свой мир после этого работает (`hp=9`, факт один). Курсор двигается: после сообщения с `offset=41` снимок несёт `Cursor: map[system_events:42]`. Мутация «снять фильтр» валит `TestAProposalOfAnotherWorldIsPassedOver`. Но движение курсора тестом не закреплено — Minor-8 |
| **Minor-5** · переход в `abandoned` | **закрыто, обе половины** | `apply.go:301-302, 326-358`. Зонд по шести случаям: `abandoned+forget` → принят; `abandoned+combat` и `abandoned+death` → `invalid_op`, статус `alive`; `dead+combat` → принят (матрица разрешает); `"sleeping"` и `""` → `invalid_op`. Мутации разделены и красят разные подтесты: снять проверку `cause` → `.../abandoned_with_another_cause`; снять `entity.StatusTransitionAllowed` → `.../a_status_the_matrix_does_not_know`. **Зазор по издателю честный, а не замаскированный**: зонд «рой (`core/swarm`) с `meta.agent{level:scope}` и `cause=forget`» → `status="abandoned", facts=1, refusals=0`, ровно как написано в doc-комментарии `statusRefusal` и в dev-log §1 |
| **Nit-1** · документы о месте двойника | **закрыто оркестратором** | `ownership.md` §1 дополнена подпакетом `…/mechanics` |
| **Nit-2** · золотой вектор таблицы исходов | **закрыто** | `fixed_test.go:112-138`, двенадцать литеральных пар. Три перестановки, сохраняющие доли 60/10/10/20, все валят `TestOutcomeTableIsTheOneWrittenDown` и **не** валят тест долей: поворот на строку, обмен `critical`↔`fumble`, обмен строки попадания со строкой промаха |
| **Nit-3** · семь причин литералом | **закрыто** | `consumer_test.go:153-187` читает `reason.enum` из скомпилированной схемы реестра; проверено, что `level_violation`/`law_violation` есть в контракте и нет в матрице заглушки |

### Оценка решения по Minor-2

**1. Утверждение «строгий порядок средствами C-01 недостижим» — верно наполовину.**

Верно то, что через `eventbus.Bus` момент регистрации группы назвать нечем: `Subscribe`
(`bus.go:40`) блокируется до конца подписки, а в `Kafka.Subscribe` (`kafka.go:140-200`) группа
регистрируется внутри `reader.FetchMessage`, то есть внутри невернувшегося вызова. Здесь автор прав,
и никакая перестановка внутри `state.go` этого не меняет.

Неверно то, что этим C-01 исчерпывается. У C-01 есть вторая половина — `Journal`
(`bus.go:47-56`: `End`, `Tail`, `ReadRange`), и её реализуют **обе** шины (зонд:
`membus is a Journal: true; *eventbus.Kafka is a Journal: true`). Последовательность «взять
`End(system_events)` (или просто 0) → опубликовать сигнал → `Tail` с этого офсета» окна потери не
имеет вовсе и не требует ни новой шины, ни правки C-01: `Tail` читает с закреплённого числа, когда
бы он ни стартовал. Заглушка при этом ничего не теряет, потому что курсор она и так ведёт сама
(`apply.go:30-34` → `Cursor` в снимке), а не полагается на коммиты группы. Цена — отказ от
потребительской группы (иное имя очереди `dead_letters`, иной учёт повторов), поэтому это решение
уровня дизайна, а не правка в рамках задачи. Автор этот вариант в dev-log §4 п. 1 называет и
отводит — отвод разумный, но формулировка «недостижимо средствами C-01» его переоценивает:
правильная формулировка — «невыразимо через `Bus`».

**2. Опциональный интерфейс проблему не прячет, но делает допущение невидимым.**

`readySubscriber` (`state.go:291-298`) не реализует **никто** в дереве — ни `membus`, ни
`*eventbus.Kafka` (зонд: обе проверки типа дают `false`). Единственная реализация — двойник
`readyBus` в самом тесте. То есть ветка `state.go:304-306` в продуктовом пути мертва, а работает
всегда `close(ready)` перед `Subscribe` (`state.go:307-308`) — ровно то, что было на ревью #1.
Прятанья тут нет: комментарий `state.go:242-249` говорит это вслух, и тест
`TestNothingPublishedBeforeStartIsLost` закрепляет ту гарантию, которая на самом деле держит.
Опасность другая и тихая: выбор ветки делается проверкой типа, компилятор в нём не участвует, и
если `membus` (T-014, в работе) заведёт метод с этим именем и другой сигнатурой — заглушка молча
останется на прежней ветке, ни один тест не покраснеет. Это Nit-4 ниже.

**3. Хуже не стало.** На `membus` поведение побайтово прежнее. Грубый порядок, который на ревью #1
не держался вовсе, теперь закреплён детерминированно: мутация «анонс перед запуском горутины»
краснеет 30 раз из 30. Новых разделяемых данных нет: `subErr` пишется и читается под `s.mu`,
`ready`/`stopped` — одноразовые каналы. Гонки не добавлено (под `-race` не проверялось — недоступен).

**4. Практический риск сегодня близок к нулю, и это довод в заявке.** Адаптер Kafka подписывается
с `StartOffset: kafka.FirstOffset` (`kafka.go:154`), то есть «новая группа читает с первого офсета»
— свойство не `membus`-специфичное, как сказано в комментарии `state.go:246-248`, а общее для обеих
реализаций C-01. Зазор открывается только у будущего адаптера, который встанет в конец журнала.

**5. Что должно попасть в заявку архитектору.** Предложение автора «добавить канал готовности в
`Bus.Subscribe`» поддерживаю как вариант, но не как единственный: развилка тройная, и самый дешёвый
вариант в ней не тот.

- **(в) Признать «новая группа читает с первого офсета» гарантией C-01 и закрепить её
  контракт-тестом T-014.** Обе реализации ей уже удовлетворяют (`kafka.go:154`; для `membus`
  проверено поведенчески). Тогда рукопожатие на старте не нужно никому, а `readySubscriber` из
  `testkit/state` удаляется вместе с зазором. Цена — один контракт-тест. **Рекомендую как первый
  шаг.**
- **(б) Если брокер без этого свойства всё же появится** — поднять `ReadySubscriber` в
  `shared/eventbus` как штатный опциональный интерфейс (одно место, обе шины могут его принять), а
  локальный `readySubscriber` заглушки заменить на него.
- **(а) Менять сигнатуру `Bus.Subscribe`** — самое дорогое (все реализации и все вызовы) и, пока
  верно (в), не нужное.

Кому: architect#1 (выбор варианта, C-01), tech-lead#1 + developer#2 (контракт-тест в T-014, если
выбран вариант «в»).

### Новые замечания

#### Minor-6 · `shared/testkit/state/state_test.go:284-307` — тест порядка ловит собственную мутацию 2 раза из 100

Мутация, названная в таблице dev-log §2 («сигнал без ожидания отчёта шины» — снять `select` в
`Start`, `state.go:278-287`), красит `TestStartSubscribesBeforeItAnnounces` **2 раза из 100**
прогонов; остальные 98 зелёные. На исправном коде тест зелёный 100 из 100, ложных красных нет.
Причина: `readyBus.SubscribeReady` записывает `"subscribe"` первой же строкой в своей горутине, а
`announceRecovery` в основной успевает сделать хэш мира и маршалинг, поэтому даже без ожидания
горутина обычно записывается первой. То есть заявленное доказательство закрытия Minor-2 держится
на удаче планировщика. Более грубая мутация (анонс перед запуском горутины) краснеет 30 из 30 —
что-то тест всё же пинит, но не то, что назван пинить.

Как чинить (детерминированно, без пауз): дать `readyBus` ворота — записать `"subscribe"`, затем
ждать канала, который открывает тест; тест убеждается, что `Start` до открытия ворот не вернулся, и
только потом сверяет порядок. Тогда порядок держится построением, а не скоростью машины.

#### Minor-7 · `shared/testkit/state/state.go:303-309` — «`Start` возвращает ошибку подписки вместо сигнала» не выполняется ни на одной существующей шине

`dev-log.md` §1 (Minor-2) утверждает без оговорок: «возвращает ошибку подписки вместо сигнала, если
та упала сразу». Зонд: обычная `eventbus.Bus` (без `SubscribeReady`), чей `Subscribe` немедленно
возвращает ошибку, 300 прогонов — **`Start` сообщил об ошибке 0 раз из 300, сигнал восстановления
ушёл 300 раз из 300**. Причина структурная: в запасной ветке `close(ready)` выполняется **до**
`Subscribe` (`state.go:307-308`), поэтому `select` в `Start` всегда выбирает `ready`. Обещание
выполняется только на шине, реализующей `readySubscriber`, а такой в дереве нет (см. Nit-4).

Поведение при этом не хуже прежнего — на ревью #1 было то же самое, — но в dev-log записана
гарантия, которой нет, и это ровно тот класс дефекта, из-за которого возникло Minor-2.

Как чинить: проще и честнее всего — сузить формулировку в dev-log до «на шине, которая умеет
отчитаться». Настоящее исправление запасной ветки без правки C-01 не выражается (см. раздел выше),
поэтому требовать его в рамках задачи не считаю правильным.

#### Minor-8 · `shared/testkit/state/apply.go:30-34` — движение курсора над сообщением чужого мира не закреплено тестом

Код правильный (зонд: после сообщения чужого мира с `offset=41` снимок несёт
`Cursor: map[system_events:42]`), и dev-log это обещает («Курсор при этом двигается: сообщение
прочитано»). Но мутация «перенести обновление курсора ниже фильтра чужого мира» оставляет **весь
пакет зелёным**. Ущерб от такой регрессии умеренный (у подписки офсеты коммитит шина, а курсор
заглушки уходит только в снимок, и отставший курсор даёт read-model лишний повтор, а не вечное
перечитывание) — потому Minor, а не Major.

Как чинить: одна строка в `TestAProposalOfAnotherWorldIsPassedOver` — подать событие через
`eventbus.ContextWithPosition` и сверить `Cursor` снимка.

#### Nit-4 · `shared/testkit/state/state.go:291-306` — выбор ветки по проверке типа, допущение невидимо компилятору

`readySubscriber` не реализован ни одним типом дерева (проверено обеими проверками типа). Если
`membus` заведёт `SubscribeReady` с другой сигнатурой или с другой семантикой, заглушка молча
останется на запасной ветке. Достаточно теста-утверждения: «`membus` о готовности не отчитывается;
когда это перестанет быть правдой, запасную ветку и `readySubscriber` надо удалить» — допущение
становится видимым и падает само, когда устареет.

#### Nit-5 · `shared/testkit/state/state.go:238-241` — комментарий приписывает платформе брокер, которого у неё нет

«on a broker where a new consumer group starts at the end of the journal…» — у адаптера Kafka этой
платформы `StartOffset: kafka.FirstOffset` (`kafka.go:154`), то есть свойство «новая группа читает
с первого офсета» держат **обе** реализации C-01, а не только `membus`, как сказано ниже по тексту
(`state.go:246-248`). Формулировка занижает то, что уже гарантировано, и завышает остаточный риск;
поправить стоит вместе с решением архитектора по варианту «в».

### Что проверено экспериментом

Все прогоны — `GOFLAGS=-buildvcs=false`, Go 1.26.8, golangci-lint 2.13.2, `-race` недоступен.

**Базовый прогон.** `go build ./...`, `go vet` по обоим каталогам, `go test -short -count=1 ./...`
— зелёные целиком. Покрытие `shared/testkit/state` **87,3 %**, `shared/testkit/mechanics`
**93,2 %** — цифры dev-log подтверждены до десятой. `golangci-lint run ./shared/testkit/...` —
**0 issues**, `gofmt -l` по обоим каталогам пуст. Инвентарь: 99 запусков тестов (с подтестами) в
двух пакетах, 99 зелёных.

**Мои зонды (все удалены).**

| Зонд | Ответ |
|---|---|
| Major-1, `atomic=true` и `atomic=false`, два набора по `player-A` | `hp 10→10, version 1→1, facts=0, refusals=1 (invalid_op, player-A), hash-moved=false, applied=[]` |
| Major-1, пакет «годный набор + два по одной сущности» | обе ветви: `facts=0 refusals=1`, мир не сдвинулся — пакет отвергается целиком, как неверно сформированный |
| Обратная проверка: три разные сущности, оба режима | `facts=3 refusals=0 applied=[p-legal]`, версии 2/2/2, `hp` 9/8/7 — законный пакет проходит целиком |
| Minor-4, `update` чужого мира с `Position{offset:41}` | `facts=0 refusals=0`, `Cursor: map[system_events:42]`, свой мир после этого работает |
| Minor-4, `create` чужого мира | сущность в этот мир не попала, `entity.created` не издан, отказ не издан |
| Minor-5, шесть переходов статуса | `abandoned+forget` принят; `abandoned+combat`, `abandoned+death`, `"sleeping"`, `""` → `invalid_op`, статус не сдвинут; `dead+combat` принят |
| Minor-5, зазор по издателю: `core/swarm` + `meta.agent` + `cause=forget` | принят (`status="abandoned" facts=1 refusals=0`) — зазор ровно тот, что назван в коде и в dev-log |
| Minor-2, кто реализует `SubscribeReady` | `membus`: нет; `*eventbus.Kafka`: нет |
| Minor-2, кто реализует `eventbus.Journal` | `membus`: да; `*eventbus.Kafka`: да |
| Minor-7, обычная шина с падающим `Subscribe`, 300 прогонов | `Start` сообщил об ошибке 0/300, сигнал ушёл 300/300 |

**Мутации итерации 2 (все откачены, файлы сверены по md5 с копиями до мутаций).**

| Мутация | Результат |
|---|---|
| снять проверку `repeatedEntity` | FAIL `TestOneEntityNamedTwiceIsRefused/{atomic,loose}` |
| снять фильтр чужого мира | FAIL `TestAProposalOfAnotherWorldIsPassedOver` |
| перенести обновление курсора ниже фильтра чужого мира | **зелено** (Minor-8) |
| снять проверку `cause != forget` | FAIL `TestAbandonedNeedsTheCauseOfForget/abandoned_with_another_cause` |
| снять `entity.StatusTransitionAllowed` | FAIL `.../a_status_the_matrix_does_not_know` |
| `ReplayModeRecovery = "test"` | FAIL `TestStartAnnouncesRecovery` |
| переставить `Put` указателя перед `Put` объекта | FAIL `TestTheObjectIsWrittenBeforeThePointer` |
| снять `select` в `Start` (не ждать отчёта), 100 прогонов | FAIL **2 из 100** (Minor-6) |
| анонс перед запуском горутины подписки, 30 прогонов | FAIL 30 из 30 |
| всегда идти по запасной ветке (`readySubscriber` выключен), 25 прогонов | FAIL 25 из 25 `TestStartSubscribesBeforeItAnnounces` |
| таблица исходов: поворот на строку | FAIL `TestOutcomeTableIsTheOneWrittenDown`, тест долей зелёный |
| таблица исходов: обмен `critical`↔`fumble` | FAIL то же, тест долей зелёный |
| таблица исходов: обмен строки попадания со строкой промаха | FAIL то же, тест долей зелёный |

**Регрессия от рефакторинга тестовых помощников — не нашёл.** Шесть мутаций ревью #1 прогнаны
заново, все по-прежнему красят те же тесты: `IsTerminal` выключен → `TestRejectionMatrix/dead_entity`,
`TestForgetAbandonsALivingCharacterOnce`, `TestForgetOverADeadCharacterIsRefused`; `CheckVersion` не
вызывается → `TestRejectionMatrix/version_conflict`, `TestARefusedProposalIsNotRemembered`,
`TestAConsumerBuildsItsProjectionFromTheStub`; запись копии в мир по ходу обхода →
`TestAtomicPackageIsAllOrNothing`; инверсия компаратора → `TestFactsComeOutInIdentifierOrder`.
Подмена C-03 по-прежнему держится компилятором: дрейф `Stats(kind string)` →
`Stats(kind string, _ int)` даёт `*FixedMechanics does not implement Mechanics (wrong type for
method Stats)`. Прежние тесты не переписаны в сторону ослабления; объём проверок вырос с 3 305 до
3 760 строк, ни одно утверждение ревью #1 не снято.

**Проверка обоснования, а не только кода.** `entity.update.rejected.v1.json` действительно закрыт:
`details` — `additionalProperties: false` с тремя полями (`expected_version`, `actual_version`,
`invariant_id`), так что подробность «слейте наборы» выразить в событии нечем и место ей в логе —
объяснение автора верно. `entity.StatusTransitionAllowed` (`shared/entity/types.go:59-67`)
задействован по назначению и на статусах сущностей v0 (только `alive/dead/abandoned/ascended_final`,
`data-model.md` §3.3 и §3.4) лишних отказов не даёт: `""→alive` матрица разрешает, `alive→""`
отвергает.

**Зонды и чистота дерева.** Оба зондовых файла (`shared/testkit/state/zz_review2_probe_test.go`,
`…/zz_review2_ready_test.go`) удалены, копии продуктовых файлов удалены. Контрольные суммы
одиннадцати файлов области совпадают со снятыми до мутаций; `git diff -- shared/testkit/state
shared/testkit/mechanics` пуст, неотслеживаемых файлов в `shared/` и `internal/` нет, состав
`git status` совпадает со снимком на начало ревью. Кроме этой записи в `review.md` ничего не правил
и не коммитил.

### Открытые вопросы (в бридж-блок волны 1)

1. Развилка по C-01 (см. раздел про Minor-2): признать «новая группа читает с первого офсета»
   гарантией C-01 и закрепить контракт-тестом T-014 **(рекомендую)** / поднять `ReadySubscriber` в
   `shared/eventbus` / менять сигнатуру `Bus.Subscribe`. Формулировку dev-log «недостижимо
   средствами C-01» заменить на «невыразимо через `Bus`»: журнальная половина C-01 (`End`+`Tail`)
   это умеет, и обе шины её реализуют.
2. Ратификация правила «одна сущность — один набор изменений» (открытый вопрос ревью #1, п. 2)
   остаётся за архитектором; ответ автора на п. 3 («схемой не выразить — `uniqueItems` сравнивает
   элементы целиком») принимаю, пункт закрыт отрицательно.
3. Потребитель отличает «повтор сущности» от прочих `invalid_op` только по логу State: `details`
   закрыт схемой. Это тот же корень, что и уже открытый вопрос про `details.batch_size` (§4.5 п. 9)
   — решать их стоит вместе: либо открыть `details` для `rule`/`batch_size`, либо признать, что
   `invalid_op` неразличим по событию.
4. Шесть открытых вопросов итерации 1 и п. 3 ревью #1 (`ownership.md` §1) — в силе; Nit-1 закрыт
   оркестратором.

### Предложения в бэклог

- Minor-6 и Minor-8 — две тестовые правки на десяток строк, естественное место им T-018 (тот же
  каталог, тот же исполнитель) или отдельная мелкая задача перед T-056.
- Nit-4: тест-утверждение «`membus` о готовности не отчитывается» — вместе с решением по п. 1
  открытых вопросов; при варианте «в» и `readySubscriber`, и это утверждение удаляются целиком.

### Риски и допущения

- **`-race` не прогонялся** (недоступен). Новое разделяемое состояние итерации 2 — `subErr` под
  `s.mu` и два одноразовых канала `ready`/`stopped`; данных, разделяемых без замка, я не нашёл, но
  под `-race` в CI на это стоит посмотреть вместе с остатком от ревью #1 (`SetFactEventID` по
  указателю, уже видимому через `Get`).
- **`shared/testkit/{contract,membus}` не открывались** (T-014, developer#2 в работе). Все
  утверждения про `membus` в этой записи — поведенческие (проверка типа и прогон), не по коду.
  Если T-014 изменит поведение группы на первом офсете, вывод по Minor-2 и Nit-5 надо пересмотреть.
- **Числа Minor-6 и Minor-7 — статистика на одной машине** (Windows, Go 1.26.8, без `-race`, без
  нагрузки). Порядок величины («почти никогда» против «всегда») от машины не зависит, точные
  2/100 и 0/300 — зависят.
- **Базы для сравнения с итерацией 1 нет**: коммита по T-017 по-прежнему не существует, поэтому
  утверждение автора «`snapshot.go` не менялся» я подтвердить не могу — проверял не отсутствие
  правки, а правильность порядка записи и то, что он теперь закреплён тестом.
- Вердикт «принять» опирается на то, что три Minor этой записи документарно-тестового свойства и не
  меняют поведения заглушки. Если оркестратор считает, что незакреплённое обещание в dev-log
  (Minor-7) должно быть исправлено до слияния, это правка одного абзаца, а не итерация.

---

## T-014 · ревью #4 (итерация 4) · 2026-09-10 · code-reviewer#2 (TEAM-1)

### Границы ревью

Ветка `epic/EPIC-001-foundation`, ревьюировались изменения в индексе по итерации 4 в моей области:
`shared/testkit/contract/**`, `shared/testkit/membus/**`, `shared/eventbus/**`. Запись `dev-log.md`
«developer#2 · T-014 · итерация 4 по ревью #3» прочитана целиком; `dev-log.md` не правился. Три
последние записи `journal.md` прочитаны. `shared/testkit/state/**` и `shared/testkit/mechanics/**`
(T-017, повторное ревью code-reviewer#1, там живые зонды) **не открывались**, тесты этих пакетов не
запускались, замечаний по ним нет; неотслеживаемого `zz_review2_probe_test.go` в дереве на момент
моих прогонов не было — не создавал и не удалял.

Состояние дерева до и после всех моих прогонов совпадает с заявленным в `dev-log` §7 (md5):
`contract.go` `9647f682…`, `kafka.go` `a5e77309…`, `membus.go` `4b0a9a43…`,
`membus_test.go` (contract) `3249e654…`, `redpanda_integration_test.go` `c030a109…`.
Все мои мутации и зонды сняты, `git diff` по этим каталогам пуст.

Окружение прогонов: Windows 11, Go 1.26.8, 32 ядра, `GOFLAGS=-buildvcs=false`, Docker 29.6.1,
Redpanda в testcontainers, `-race` недоступен. Форма прогона — та же, что в CI:
`go test -tags integration -count=1 -timeout 15m ./shared/testkit/contract/` (обе реализации в одном
прогоне). «Под нагрузкой» = `GOMAXPROCS=1` + 16 нагрузчиков.

### Вердикт

**Принять. Ворота волны 1 со стороны T-014 закрыты.** Critical: 0 · Major: 0 · Minor: 1 · Nit: 2.

Замечание, из-за которого задача возвращалась трижды, закрыто и доказано прогонами: **27 красных из
27 под мутацией H**, из них 5 под нагрузкой, ни одного зелёного. Красит ровно тот кейс, ради
которого правка делалась, — `CloseStopsTheSubscriptionsAndRefusesToPublish` на таргете `redpanda`, —
и только он: соседние кейсы (в том числе `CancellingASubscriptionStopsItOnTheBacklog` и
`AnUncommittedEventIsDeliveredAgain`) зелёные во всех 27 прогонах. Ложных красных на исправном коде
нет: **21 зелёный прогон из 21**, 6 из них под нагрузкой. Мутации B1 и B2 держатся (4 из 4 и 4 из 4).
Цифры `dev-log` §7 сошлись: покрытие `contract` 81,9 %, `testkit` 74,0 %, `membus` 83,8 %,
`eventbus` 75,4 %, `golangci-lint` v2.13.2 — 0 issues в обоих режимах тегов, `gofmt` пусто,
`go vet -tags integration` чист.

Отдельной строкой, для журнала: **ворота волны 1 со стороны T-014 закрыты — якорь мутации H красен
в 27 прогонах из 27, ложных красных 0 в 21 прогоне, вероятностная половина якоря измерена и
совпала с 1/2 на подписку.**

### Оценка разбора механизма

Разбор исполнителя проверен по исходнику `kafka-go` v0.4.51 и по `shared/eventbus/kafka.go`, а
затем — измерением. **Разбор верен по всем трём утверждениям**, и он точнее того, что дал
оркестратор.

**1. «Цикл упирается в коммит, а не в чтение» — верно.** Порядок в `Subscribe`
(`kafka.go:185-203`) — `FetchMessage → deliver → CommitMessages`; разблокированный обработчик
возвращает управление ровно перед коммитом, а не перед чтением. Предпосылка оркестратора («`Close`
не ждёт горутин подписок») тоже верна: `Reader.Close` (`reader.go:757-777`) отменяет свои
контексты, ждёт `r.join.Wait()` и закрывает `r.msgs` — про обработчик вызывающей стороны он не
знает. Но вывод «мутант дочитает буфер читателя» неверен: до буфера дело не доходит.

**2. «Ровно 1/2 на подписку» — верно, и это подтверждено не только чтением кода, но и
распределением.** Цепочка по коду:
`commitLoopImmediate` (`reader.go:193-225`) на `ctx.Done()` разбирает `r.commits` до пустоты,
отвечает всем `errch` и выходит; `Close` дожидается именно его через `r.join.Wait()`. После этого
`CommitMessages` (`reader.go:878-901`) входит в `select` из трёх ветвей: `r.commits <- creq`
(канал буферизован на `QueueCapacity` — `reader.go:679,705`; в конфиге адаптера не задан, значит
100, и он пуст → ветвь готова), `<-ctx.Done()` (у мутанта не готова) и `<-r.stctx.Done()`
(`r.stctx` отменён `r.stop()` внутри `Close` — `reader.go:700,706,766` → ветвь готова). Две готовые
ветви ⇒ равновероятный выбор. Первая ведёт в вечное ожидание, потому что `useSyncCommits()`
(`reader.go:109`) истинно при `CommitInterval: 0` (`kafka.go:156`) и второй `select` ждёт `errch`,
на который отвечать уже некому; вторая даёт `io.ErrClosedPipe`, который `stopped`
(`kafka.go:455-457`) считает штатной остановкой, и `Subscribe` возвращает `nil`.
**Измерение.** В 27 прогонах под мутацией H кейс сообщает о зависании первой по порядку проверки
подписки (`parked`, `a`, `b`, `c`). Наблюдалось: `a` — 14, `b` — 8, `c` — 2, без зависания — 3.
Ожидание при p = 1/2 на подписку: 13,5 / 6,75 / 3,375 / 3,375. Совпадение практически точное;
гипотеза «1/2» подтверждается, гипотеза «0,83», из которой исходило ревью #3, — нет.

**3. «Настоящая причина прежних зелёных — последовательное закрытие читателей» — верно.**
`Kafka.Close` (`kafka.go:311-341`) снимает снимок карты `k.readers` (порядок обхода карты
случаен) и закрывает читателей подряд уже после `k.stopReaders()`; у мутанта `stopReaders` ни на
что не влияет, поэтому цикл живёт ровно до закрытия **своего** читателя. Прямое подтверждение
моим зондом на длительности закрытий (см. «Что проверено экспериментом», п. 3): на **исправном**
коде первое закрытие стоит 5,5–43,5 мс, а все последующие — 0 с, потому что циклы, получив отмену,
закрывают своих читателей сами через `defer k.closeReader`, и `Close` добивает уже закрытых. На
мутанте закрытия идут одно за другим по-настоящему.

**Что в разборе стоит поправить (Nit-1).** Формулировка «требование сведено к: закрытие одного
kafka-читателя длится дольше двух событий обработчика, то есть дольше ~9 мс» — не то, что
гарантируется конструкцией, и по нижней границе неверна. Одиночные закрытия у меня опускались до
**5,5 мс** (28 замеров: 5,5 / 5,9 / 6,2 / 6,4 / 7,4 / 8,4 / 9,4 … 91,4 мс), то есть бывают короче
двух событий (~11,6 мс при измеренных ~5,8 мс на событие под мутацией). Гарантируется другое и
большее: у **последней** закрываемой подписки хвоста впереди не одно закрытие, а сумма как минимум
двух, и эта сумма в шести прямых замерах составила **55,0 / 73,7 / 74,2 / 85,0 / 96,5 / 118,4 мс**,
то есть от 9 до 20 событий при допуске в одно. Вывод исполнителя от этого только крепче — запас не
двукратный, а девятикратный, — но записан он должен быть как «сумма закрытий впереди последней
подписки хвоста», иначе следующий, кто станет упрощать кейс, снимет не тот запас.

### Статус замечаний ревью #3

| # | Статус | Чем доказано |
|---|---|---|
| **Major-1** (якорь мутации H — «монетка»: 6 зелёных из 35) | **закрыт** | своей мутацией H, 27 полных прогонов (6 + 16 обычных, 5 под нагрузкой) — **27 красных, 0 зелёных**. Механизм зелёных из ревью #3 объяснён и устранён: он был в том, что единственный читатель хвоста мог закрыться первым; с тремя подписками этого случиться не может. Остаточный риск оценён численно ниже |
| **Nit-1** (`stop` ждёт `<-s.done` без срока) | **закрыто** | `contract.go:1287-1294`: `select` с `testkit.After(Timeout)`, отчёт через `t.Errorf` — зависшая подписка стоит кейсу 15 с, а не `-timeout` всего прогона |
| **Nit-2** (припаркованная подписка в общей группе) | **закрыто** | `contract.go:932`: группа `r.group+"-parked"`, подписки хвоста — `-a`, `-b`, `-c`; общий `start` принимает группу параметром. Побочный выигрыш подтверждён: пакет на брокере 9,1–15 с против 13,3 с в итерации 3 |
| **Nit-3** (кандидат на первый флак разошёлся с замером) | **закрыто** | правка внесена в запись итерации 3; мой замер под нагрузкой подтверждает: самый долгий кейс — `Close`, 6,11–7,87 с, следующий `RetriesThenDeadLetter…` 2,45–4,39 с |
| Nit-1 ревью #2 (чтение после `Close`) | в бэклоге, не поднимаю | — |
| Minor-1/2, Minor-3, ОВ-5 ревью #3 | закрыты решением оркестратора (T-395, заявка architect#1) | не переоткрываю |

Мутации Major-2 и Major-3 ревью #2 после перекройки кейса держатся: **B1** (`membus.go:299`,
припаркованная ветка → `ErrClosed`) — 4 красных из 4, сообщение адресное («Subscribe returned
eventbus: bus is closed for the subscription parked at the end of its topic, want nil»);
**B2** (`membus.go:284`, `stopping` → `ctx.Err()`) — 4 красных из 4, «subscription a/b/c handled 280
more events … (300 of the 300 events of the backlog in all)» по всем трём подпискам.

### Оценка трёх отклонений от решения оркестратора

**1. Обработчик отпускается перед `Close`, а не после его возврата — принято, отклонение
усиливает якорь.** В варианте оркестратора к моменту разблокировки все читатели уже закрыты:
цикл либо встаёт на коммите, либо получает `io.ErrClosedPipe`, возвращается в `FetchMessage`,
получает `io.EOF` и штатно завершается. Счётчик при этом вырасти не может **вообще**, то есть
детектор «счёт» исчезает и остаётся только 1/2 на подписку (7/8 на трёх). В принятом варианте
работают оба детектора, и это видно в моих прогонах: 24 красных из 27 пришли зависанием, 3 —
счётом, причём в каждом из этих трёх допуск перешагнули минимум две подписки из трёх.

**2. Допуск `slack = 1` оставить. Строку `slack = 0` НЕ вносить.** Проверено прямым прогоном:
с `slack = 0` на исправном коде я получил **ложный красный в 1 прогоне из 8** — дельта 1 у двух
подписок из трёх, и не на брокере, а на **`membus`** (`--- FAIL: TestBusContractOnMembus/CloseStops…`,
0,08 с). Механизм понятен: у заглушки между `close(release)` и отменой нет сетевого коммита,
который на брокере съедает этот зазор. Замер исполнителя «0 из 64» воспроизводится (в моих 84
измерениях 82 нуля, максимум дельты — 1), но эти два измерения показывают, что допуск несёт
нагрузку, а не вкус. Отклонение принято, вопрос закрыт.

**3. Запасной вариант применён вместе с основным — принято.** Это не «на всякий случай»:
пауза 3 мс и хвост 300 лечат порог «`Close` быстрее одного события», три подписки лечат слепой
участок «мой читатель закрыт первым»; ни одно из двух не покрывает другое. Численно: без паузы
порог был ~25 мс при закрытиях от 5,5 мс; без третьей подписки кейс опирался бы на одну
оставшуюся — в моих прогонах слепой участок наблюдался прямо (в прогоне серии A подписка `b` дала
≤ +1, тогда как `a` дала +13 и `c` +2).

### Новые замечания

**Minor-1 · `dev-log` §6 — бюджет худшего красного прогона занижен примерно на четверть, и запас
до `-timeout 15m` фактически исчерпан.** Счёт «18 кейсов × 15 с» считает по одному сроку на кейс,
а сроки в наборе не по одному: `stop` теперь тоже ограничен `Timeout` и сообщает через `t.Errorf`,
то есть **не прерывает кейс**, поэтому `defer sub.stop(t)` добавляет 15 с поверх упавшего
ожидания. По разбору всех ожиданий пакета (`contract.go`: 13 кейсов с собственным сроком, из них
`groupResumesFromItsCursor`, `uncommittedIsDeliveredAgain`, `bigUndecodableBodyIsTruncated` и
`closeStopsEverything` держат по три независимых срока) худший красный ≈ **7,25 мин на реализацию**
→ **≈ 14,5 мин на пакет**, при `-timeout 15m` на бинарь пакета
(`.github/workflows/go.yml:181`). Это не прячет дефект — упёршийся в `-timeout` прогон падает
громко, — но диагностика в этом случае будет не сообщением кейса, а паникой рантайма, и
`timeout-minutes: 25` тут ни при чём.
*Как исправить (любое, файл вне владения T-014 — в заявку/бэклог):* поднять `-timeout` для job
`integration` до 20m, либо снизить `Timeout` контрактного набора до 10 с (запас до наблюдаемых
максимумов — 6,11–7,87 с под нагрузкой — остаётся, но сжимается), либо просто записать в `dev-log`
верную цифру и признать её осознанной. Ворота не блокирует.

**Nit-1 · `dev-log` §2 п. 1 и §10 — «требование сведено к: закрытие одного читателя дольше двух
событий обработчика (~9 мс)».** Формулировка по нижней границе неверна и занижает собственный
результат: см. «Оценка разбора механизма», п. 3. Предлагаемая замена: «сумма закрытий читателей,
идущих впереди последней закрываемой подписки хвоста (не менее двух), дольше двух событий
обработчика; измерено 55–118 мс против ~11,6 мс».

**Nit-2 · `dev-log` §2 п. 4 — «0 дополнительных вызовов в 64 измерениях из 64» стоит дополнить
контрпримером.** У меня 84 измерения дали 82 нуля и **два** значения 1 — оба в одном прогоне на
`membus`. Цифра исполнителя честна, но в таком виде читается как «допуск не нужен», и следующий,
кто станет чистить кейс, снимет `slack`. Одна строка в записи («на `membus` дельта 1 наблюдалась,
1 прогон из 8 с `slack = 0` красный») закрывает вопрос навсегда.

### Что проверено экспериментом

**1. Якорь мутации H — своей мутацией, 27 полных прогонов (требовалось ≥ 20).**
Мутация: в `shared/eventbus/kafka.go` строки 172-175 (`ctx, cancel := context.WithCancel(ctx)`,
`defer cancel()`, `context.AfterFunc(k.closing, cancel)`, `defer stopWatchingClose()`) заменены на
`_ = k.closing` — состояние кода до правки итерации 2. md5 под мутацией `acf28fa5…`, после снятия
`a5e77309…`.

| Форма прогона | Прогонов | Красных | Зелёных |
|---|---|---|---|
| пакет целиком, как гоняет CI, серия A | 6 | **6** | 0 |
| то же, серия B | 16 | **16** | 0 |
| то же, `GOMAXPROCS=1` + 16 нагрузчиков | 5 | **5** | 0 |
| **итого** | **27** | **27** | **0** |

**Красит именно кейс `Close`, а не соседний.** Во всех 27 прогонах падал ровно один подтест —
`TestBusContractOnRedpanda/CloseStopsTheSubscriptionsAndRefusesToPublish`, и только на таргете
`redpanda` (мутация в `kafka.go`, `membus` её не видит — как и должно быть). Ни
`CancellingASubscriptionStopsItOnTheBacklog`, ни `AnUncommittedEventIsDeliveredAgain`, ни
`SubscribeReturnsNilOnAnOrderlyStop` не покраснели ни разу.

Чем красит: **24 прогона — зависанием** («Close left the subscription … running after 15s», кейс
15,3–24,0 с), **3 прогона — счётом** (кейс 0,40–0,47 с): `a +13 / c +2`, `a +9 / c +4`,
`a +9 / b +11 / c +14` при допуске 1. Обе половины якоря сработали, ни одного прогона, где не
сработала ни одна.

**2. Ложные красные на исправном коде — не найдены, 21 прогон (требовалось ≥ 15).**

| Форма прогона | Прогонов | Результат |
|---|---|---|
| пакет `contract` целиком | 12 | 12 зелёных, 11–15 с |
| то же, `GOMAXPROCS=1` + 16 нагрузчиков | 6 | **6 зелёных**, 41–52 с |
| базовый прогон до мутаций | 1 | зелёный, 10,1 с |
| `./shared/testkit/contract/ ./shared/testkit/membus/ ./shared/eventbus/...`, дважды подряд | 2 | зелёные, `contract` 9,8 с и 9,1 с |
| **итого** | **21** | **21 зелёный, 0 красных** |

Самый долгий кейс набора под нагрузкой — `Close`: 6,11 / 6,70 / 6,90 / 6,96 / 7,56 / 7,87 с
(следующий — `RetriesThenDeadLetterAndTheStreamMovesOn`, 2,45–4,39 с). Это совпадает с
`dev-log` §5 (5,75–8,66 с). **Риск флака в CI оцениваю как низкий, несмотря на подорожание
кейса**, и вот почему: 300 публикаций делаются **до** взятия срока (`deadline` берётся строкой
1005, после цикла публикаций), поэтому дорогая часть кейса не лежит ни под одним 15-секундным
бюджетом; под бюджетом остаются `parked.wait` (одно событие), общий срок на три сигнала (подписки
к этому моменту уже разобрали 20 событий каждая) и возврат подписок после `Close` (на исправном
коде мгновенный). Запас на каждом из них — не «6 с из 15», а «доли секунды из 15».

**3. Разбор механизма — зондом на длительности закрытия читателей** (`Kafka.Close`, лог времени
каждого `r.Close()` и накопленной суммы; 6 прогонов под мутацией H, 3 на исправном коде;
`go vet` и компиляция под тегом `integration` проверялись, зонд снят, md5 восстановлен).

- **Исправный код:** первое закрытие 5,5 / 31,3 / 43,5 мс, **все последующие — 0 с**. Это прямое
  доказательство того, что при отмене циклы закрывают своих читателей сами (`defer k.closeReader`),
  а `Close` добивает уже закрытых.
- **Мутант:** закрытия идут по-настоящему, одиночное — **5,5…91,4 мс** (28 замеров), накопленная
  сумма перед закрытием последнего читателя хвоста — **55,0 / 73,7 / 74,2 / 85,0 / 96,5 / 118,4 мс**.
  Стоимость события обработчика под мутацией ~5,8 мс (22 события за 128 мс).
- **Оценка остаточного риска зелёного прогона.** Зелёный требует одновременно: (а) ни одна из трёх
  подписок не встала на коммите — p = 1/8 по измеренному 1/2 на подписку, и (б) последняя
  закрываемая подписка хвоста не успела двух событий, то есть накопленная сумма закрытий впереди
  неё < ~11,6 мс. В шести прямых замерах эта сумма ни разу не опускалась ниже 55 мс. Отсюда
  P(зелёный) заметно ниже 1 %, против 17 % в итерации 3. Это оценка, а не гарантия (см. риски).

**4. Мутации B1 и B2 — своими прогонами.**

| # | Мутация | Прогонов | Красных | Чем упало |
|---|---|---|---|---|
| B1 | `membus.go:299` — припаркованная ветка `Subscribe` → `ErrClosed` | 4 | **4** | «Subscribe returned eventbus: bus is closed for the subscription parked at the end of its topic, want nil», кейс 0,08 с |
| B2 | `membus.go:284` — `stopping(ctx)` → `ctx.Err() != nil` | 4 | **4** | «subscription a/b/c handled 280 more events … (300 of the 300 events of the backlog in all)», все три подписки |

После каждой серии `membus.go` восстановлен, md5 `4b0a9a43…` сверен.

**5. Допуск `slack` — прогоном с `slack = 0` и логом фактических дельт.**

| Форма | Прогонов | Результат |
|---|---|---|
| `slack = 0`, обычный прогон | 8 | **1 красный** (`membus`, дельты `a=1`, `b=1`, `c=0`), 7 зелёных |
| `slack = 0`, `GOMAXPROCS=1` + 16 нагрузчиков | 6 | 6 зелёных, все дельты 0 |
| **итого измерений** | **84** | 82 нуля, 2 единицы, максимум 1 |

Вывод — в отклонении 2 выше. Зонд снят, `contract.go` восстановлен, md5 `9647f682…`.

**6. Честность цифр `dev-log` §7.**

| Заявлено | Намерено мной |
|---|---|
| покрытие `contract` **81,9 %** | **81,9 %** |
| `testkit` 74,0 %, `membus` 83,8 %, `eventbus` 75,4 % | те же |
| `golangci-lint run ./shared/...` — 0 issues | **0 issues** на моей области (v2.13.2) |
| `--build-tags integration,e2e` — 0 issues | **0 issues** |
| `gofmt -l shared/` пусто | пусто |
| `go vet`, `go vet -tags integration` чисто | чисто |
| набор стал быстрее (9,6–12,7 с) | 9,1–15,0 с, медиана ~12 с; против 13,3 с итерации 3 — подтверждаю |
| md5 пяти файлов | все пять совпали, до и после моих прогонов |

**7. Бюджет красного прогона — пересчитан, см. Minor-1.** Расхождение с `dev-log` §6: 14,5 мин
против заявленных 11,5 мин на пакет, при `-timeout 15m` (`.github/workflows/go.yml:181`) и
`timeout-minutes: 25` (там же, :146). Вывод «укладывается» сохраняется только формально.

### Зонды и чистота дерева

Мои зонды — временный `t.Logf` дельт и `slack = 0` в `closeStopsEverything`, `fmt.Fprintf` времени
закрытия читателей в `Kafka.Close`, три мутации (H, B1, B2) — удалены, файлы восстановлены из
резервных копий, md5 сверены: `contract.go` `9647f682…`, `kafka.go` `a5e77309…`,
`membus.go` `4b0a9a43…`, `membus_test.go` `3249e654…`, `redpanda_integration_test.go` `c030a109…`.
`git diff` по `shared/eventbus`, `shared/testkit/contract`, `shared/testkit/membus` пуст,
неотслеживаемых файлов в этих каталогах нет. Кроме этой записи в `review.md` ничего не правил и не
коммитил. Каталоги T-017 (`shared/testkit/{state,mechanics}`) не открывались и тестами не
запускались — там живые зонды code-reviewer#1.

### Что остаётся хвостом волны 1 (не блокирует ворота)

- **Заявка system-architect (бридж-блок волны 1):** расширение C-01 — «контекст обработчика
  отменяется при `Close`». Это единственный кандидат на строго детерминированное утверждение,
  отличающее исправный код от мутанта H без обращения ко времени; сегодня такого утверждения в
  границах C-01 не существует, и исполнитель честно не заявляет «детерминирован по построению».
  Пока его нет, якорь остаётся вероятностным с оценкой P(зелёный) < 1 %.
- **T-395** (кейс на `stopping` в путях dead letter — правка внесена, якоря нет) и
  **T-394** (собственный интеграционный тест адаптера в `shared/eventbus`): переносить якорь в
  T-394 не потребовалось, но как второй, независимый от контрактного набора якорь он по-прежнему
  полезен — предлагаю оставить в волне 1 рядом, не блокирующим.
- **Minor-1 этой записи** — `-timeout` job `integration` или `Timeout` набора (заявка, файл вне
  владения T-014).
- **ОВ-5 / C-01** (закрытие шины под `Tail`/`ReadRange`) — решение оркестратора от 2026-09-10,
  не переоткрываю; мутация «Tail, припаркованная ветка → `ErrClosed`» набор по-прежнему не красит.
- **`-race` ни разу не прогонялся** ни в одной из четырёх итераций. Новое разделяемое состояние
  итерации 4 — канал `release` (только закрывается) и `atomic.Int64` в трёх `backlogReader`. При
  этом job `unit` (`go test -short -race`) этот кейс не затрагивает, а job `integration` идёт без
  `-race`, то есть кейс `Close` под детектором гонок не окажется вообще. Заявка в бэклог, а не
  замечание к T-014.
- **Ворота волны 1 целиком** дополнительно зависят от T-017 (`shared/testkit/{state,mechanics}`,
  повторное ревью code-reviewer#1) — эта запись закрывает только сторону T-014.

### Риски и допущения

- **Все прогоны — на машине владельца** (32 ядра, брокер в Docker там же). Раннер GitHub
  (2–4 vCPU) эмуляцией `GOMAXPROCS=1` + нагрузчики не воспроизводится. Направление сноса при этом
  в пользу якоря: закрытие читателя — сетевой выход из consumer group, на медленном раннере оно
  только длиннее, а значит счётчик мутанта только больше.
- **P(зелёный) < 1 % — оценка по 6 замерам суммы закрытий и по распределению 27 прогонов, а не
  константа.** Утверждать «якорь детерминирован по построению» я, как и исполнитель, не буду; это и
  есть содержание заявки архитектору выше. Разница с ревью #3 — не в риторике: там зелёные
  наблюдались (6 из 35), здесь не наблюдались ни разу в 27, и известен механизм, который их
  исключает.
- **Мутация H — одна.** Она возвращает код в состояние до правки итерации 2 и потому проверяет
  ровно то, ради чего кейс писался. Другие способы сломать `Close` (например, порядок
  `stopReaders` относительно закрытия читателей) я не мутировал: за границами замечания ревью #3.
- **Область — только `shared/testkit/{contract,membus}` и `shared/eventbus`.**
  `.github/workflows/go.yml` открывался на чтение ради Minor-1. `go test -short ./...` я намеренно
  **не** запускал, чтобы не смешивать результат с живыми зондами T-017 в
  `shared/testkit/state/**`; цифру «24 пакета ok» из `dev-log` §7 не подтверждаю и не оспариваю.

---

## T-018 · ревью #1 · 2026-09-10 · code-reviewer#1

### Границы ревью

Ветка `epic/EPIC-001-foundation`, HEAD `fd93a6a`, коммита по T-018 нет — ревьюировались изменения
в индексе: `shared/testkit/gateway/**` (3 файла), `shared/testkit/swarm/**` (4 файла),
`test/e2e/**` (3 файла) и пять правок по ревью #2 T-017 в `shared/testkit/state/**`.

**Вне области и не открывалось:** `shared/testkit/{contract,membus}/**` и `shared/eventbus/**`
(T-014, code-reviewer#2 держит там живые зонды — `ZZPROBE`, `time.Since` в `kafka.go`);
общемодульный `golangci-lint run ./...` красный из-за них, к T-018 это отношения не имеет.
`internal/mechanics/**`, `rules/**`, `testdata/**`, `shared/contracts/**` читались, не правились.
`dev-log.md`, `tasks.md`, `journal.md`, `ownership.md`, `.golangci.yml` не правились. Единственный
файл на запись — эта запись в `review.md`.

Основание: раздел T-018 в `tasks.md` (в первую очередь **решение оркестратора от 2026-09-11**:
`WithEncounterStub` не делается, `Attack`/`Flee` не делаются, боевой сценарий и число 30
переносятся в I1-α); раздел «T-017 · ревью #2» выше; записи `dev-log.md` «developer#1 · T-018»
и правка записи «developer#1 · T-017 · итерация 2»; пять последних записей `journal.md` (решения
приняты, не переоткрываю); `design.md` §5, §5.1, §10, §12; `contracts.md` §0, C-04, C-05 v1.1, §17;
`schemas/events/**`.

### Вердикт

**Принять.** Critical 0 · **Major 0** · Minor 3 · Nit 4.

Задача сделана в сокращённом составе ровно по решению оркестратора, и сокращение проведено честно:
`Attack`/`Flee`, `solo-30`, `WithEncounterStub` и `kind=turn/death/world_event` не «заглушены
пустышками», а отсутствуют, причём отсутствие каждого названо вслух в коде и — что важнее —
закреплено тестом (`Script("solo-30")` возвращает ошибку, называющую причину;
`TestTheTableOfV0IsTheWholeTable` падает при появлении `combat.decided` в таблице). Главное
свойство задачи — «ожидания сквозного теста выводятся из скрипта, а не записаны литералом» —
подтверждено четырьмя мутациями и держится: тест реагирует на пропажу шага, на подмену порядка и
на пропажу предложения и остаётся зелёным, когда скрипт честно растёт или сжимается.

Три Minor — один класс: **утверждение шире, чем то, что закреплено тестом**. Тот же класс, из-за
которого в T-017 появились Minor-6 и Minor-7; здесь он не блокирует, потому что во всех трёх
случаях сама реализация верна (проверено зондами), неверна только запись о ней или охват теста.

### Статус пяти замечаний ревью #2 T-017

| Замечание | Статус | Чем подтверждено |
|---|---|---|
| **Minor-6** (ворота теста порядка) | **закрыто** | мутация «снять `select` в `Start`» — **60 из 60 красных** (было 2 из 100 на моей машине); исправный код — **0 из 140** (60 подряд + 80 под восьмикратной нагрузкой); сообщение точное |
| **Minor-7** (гарантия, которой нет) | **закрыто сужением; новая формулировка честная** | зонд: обычная `eventbus.Bus` с немедленно падающим `Subscribe`, 60 прогонов — `Start` сообщил **0 из 60**, `Wait` — **60 из 60**; ровно то, что теперь написано в `dev-log` и в doc-комментарии `Start` |
| **Minor-8** (курсор над чужим миром) | **закрыто** | мутация «перенести обновление курсора ниже фильтра» — **30 из 30 красных**, `cursor 0, want 42` |
| **Nit-4** (допущение о проверке типа) | **закрыто** | рефлексия по имени метода на месте; дрейф сигнатуры `readySubscriber` вообще не компилируется (проверено); а если заглушка молча уходит на запасную ветку — `TestStartSubscribesBeforeItAnnounces` красный 20 из 20 |
| **Nit-5** (комментарий о брокере) | **закрыто** | формулировка перечисляет обе реализации C-01; `kafka.go` в этот раз не открывал (чужая область) — принимаю по своему же чтению в ревью #2 |

**Minor-6 разобран отдельно, потому что заявка была сильной.** Ворота построены правильно: они не
синхронизация, а *задержка*, и держат подписку, пока тест не откроет. При исправном коде `Start`
за воротами не проходит вообще, сколько бы тест ни ждал, — поэтому `gateWindow = 250 мс`
ограничивает только время, за которое **сломанный** код обязан себя выдать, и на медленной машине
не превращается в ложный красный. Единственный канал ложного красного, который я нашёл, —
`waitFor` (3 с на то, чтобы горутина подписки записала первый шаг); под восьмикратной нагрузкой
он не сработал ни разу из 80.

**Тест не превратился в проверку самих ворот.** Проверено мутацией «заглушка молча уходит на
запасную ветку» (`ok && false` в `subscribe`): ворота в этом случае вообще не участвуют, а тест
красный 20 из 20 с сообщением «`Start returned` while the subscription was still being made».
То есть пинится поведение `Start`, а не двойник.

### Замечания по серьёзности

#### Minor-1 · `shared/testkit/gateway/harness_test.go:242-286` — «изменение, о котором харнесс не слышал, даёт `version_conflict`» не закреплено ни одним тестом

Запись `journal.md` от 2026-09-11 говорит без оговорок: «Прямое следствие **закреплено тестом** —
изменение, о котором харнесс ещё не услышал, даёт `version_conflict`». Слово `version_conflict`
в `shared/testkit/gateway/**` встречается **один раз — в комментарии** (`harness_test.go:252`).
Единственный тест рядом, `TestRestPinsTheVersionItSawLast`, делает прямо противоположное: он
`waitFor`-ом **дожидается**, пока харнесс услышит о ране, и тем самым закрывает окно, в котором
свойство и проявляется. Никакой отказ в этом пакете не ожидается и не проверяется.

Само свойство при этом есть, и код верен — зонд (удалён):

- **глухой харнесс** (подписка отменена после создания персонажа, рана нанесена в обход, затем
  `Rest`): `Rest` вернул `no answer to proposal gw-rest-player-A-2 within 300ms`, а на шине лежит
  `entity.update.rejected reason=version_conflict` — State отказал ровно так, как обещано;
- **слушающий харнесс без `waitFor`** (та самая снятая строка), 50 прогонов: `version_conflict`
  **50 из 50**, «прошло» 0 из 50.

То есть свойство воспроизводимо и его легко закрепить, но сегодня оно живёт только в комментарии.
Ущерб умеренный (заглушка v0, потребитель — тесты волны 1), потому Minor, а не Major.

Как чинить (5–8 строк): отдельный тест — остановить подписку харнесса (`cancel` + `Wait`), ранить
персонажа через `fake.Apply`, вызвать `Rest` и утверждать, что на `system_events` появился
`entity.update.rejected` с `reason = state.ReasonVersionConflict` под `proposal_id` харнесса.
Комментарий `harness_test.go:250-254` при этом становится ссылкой на тест, а не самостоятельным
обещанием.

#### Minor-2 · `shared/testkit/swarm/narrator_test.go:33-57` — «вся таблица» не проверяет, что таблица вся

Решение «подписку на `combat.decided` не объявлять» закреплено тестом с обеих сторон, и это
проверено:

- тип **появился** в таблице (`"combat.decided": "turn"`) → красный, сообщение
  «v0 answers combat.decided with "turn"; nothing in Phase 1 publishes it to the stub»;
- тип **пропал** из таблицы (снял `player.looked`) → красный, «player.looked is answered with ""
  (false), want "entry"» (плюс ещё два красных теста в пакете).

Обе стороны держатся. Но тест проверяет **список из семи поимённо названных типов**, а не размер
таблицы. Мутация «добавить в `narrated` тип, которого в списке нет» — `"npc.moved": KindEntry`
(тип EPIC-003, к которому заглушка отношения не имеет) — оставляет **весь пакет зелёным, и e2e
тоже зелёным**. Имя теста (`TheTableOfV0IsTheWholeTable`) и формулировка `journal.md` («падает
при появлении типа в таблице») шире того, что тест делает.

Как чинить (2 строки): либо сверять по всему реестру (`KindFor` для каждого типа
`contracts.Types()`: ровно три отвечают, остальные — нет), либо экспортировать длину таблицы и
сверять с длиной `tells`. Первое лучше: список «запрещённых» типов тогда не надо поддерживать
руками.

#### Minor-3 · `test/e2e/stubs_v0_test.go:94-98, 237-240, 442` — сломанное событие превращается в десятисекундный таймаут, который ничего не называет

Проверка «сломай одно событие — тест ловит» проведена: мутация «нарратор не кладёт `laws_version`»
(поле обязательно по схеме `narrative.output`) — тест **красный**, это правильно. Но:

| Порядок проверок | Время до красного | Что сказано |
|---|---|---|
| как сдано | **10,04 с** | `timed out waiting for the narratives of the scenario` |
| `assertNothingWasDeadLettered` поднят выше `assertNarratives` | **0,04 с** | `dead letter: … Error: testkit/swarm: publish narrative.output (entry): eventbus: validate narrative.output: contracts: invalid payload narrative.output: jsonschema validation failed …` — с идентификатором события-причины |

То есть точная диагностика **уже лежит на шине** (в `dead_letters`), но проверка, которая её
читает, стоит последней и до неё не доходит: `waitFor` делает `t.Fatalf`. Сообщение таймаута не
называет даже счёт («ждал 6, вижу 3»). Для теста, который в CI пойдёт под тегом `e2e`, это
означает 10 секунд и строку, по которой нельзя понять, что случилось.

Как чинить (одно из двух, обе — одна-две строки): либо проверять недоставленные **до** ожидания
нарративов (порядок «сначала то, что уже произошло, потом то, чего ждём»), либо печатать в
сообщении таймаута счёт и содержимое `dead_letters`.

### Nit

- **Nit-1 · `shared/testkit/gateway/harness.go:437-438`.** Doc-комментарий `move` обещает «the
  position **and the scope** of the character, in one atomic package», а `scope` в предложение не
  кладётся — это осознанное отклонение, описанное в `dev-log` §2.2 и вынесенное в открытый вопрос
  №2. Комментарий надо привести к коду, иначе следующий читатель поверит комментарию.
- **Nit-2 · `shared/testkit/swarm/narrator.go:73-77`.** `DefaultLawsVersion = "v1"` описан как
  значение, которое «приходит из фикстуры мира» (`testdata/fixtures/world.json` действительно
  несёт `laws_version: v1`), но код фикстуру не читает и ничто эти два места не связывает. Если
  фикстура поменяет версию законов, нарратив молча уедет, и ни один тест этого не заметит.
  Лечится одной строкой в тесте (сверить константу с атрибутом фикстуры) — или чтением атрибута.
- **Nit-3 · `test/e2e/empty_world_test.go:35, 62, 134-145`.** Процесс, который умирает на старте,
  стоит набору полные 60 с: `probe` опрашивает порт и за жизнью ребёнка не следит. Замер: с
  заведомо неверными флагами тест краснеет за **63,4 с**, хотя процесс вышел с кодом 2 через
  миллисекунды и его stderr (`multiverse: --bus=memory requires --contexts=all`) тест исправно
  сохранил и напечатал. Лечится каналом от `cmd.Wait()` в `select` опроса — 0,1 с вместо минуты.
  Гонку за портом (окно между закрытием слушателя и `bind` процесса) подтвердить не удалось:
  5 прогонов подряд зелёные; риск честно назван в `dev-log` §2.9.
- **Nit-4 · `dev-log.md` §2.1 и заголовок записи.** Заголовок говорит «**три** правки по ревью #2
  T-017», в §1 их пять (Minor-6…8, Nit-4, Nit-5). Замеренные мною количества тестов:
  `gateway` — **16** тестов / 33 запуска (в §2.1 «15 / 24»), `swarm` — **17** (в §2.1 «16»),
  `template` — 5 (совпадает). Цифры покрытия совпали до десятой.

### Что проверено экспериментом

Все прогоны — машина владельца, Go 1.26.8, `GOFLAGS=-buildvcs=false`, golangci-lint 2.13.2,
`-race` недоступен. Все зонды и мутации сняты, дерево сверено (`git diff` по
`shared/testkit`/`test/e2e` пуст, `grep ZZPROBE` по ним — 0).

**Базовый прогон.** `go build ./...`, `go vet` по четырём каталогам и `go vet -tags e2e ./test/...`
— зелёные. `go test -short -count=1 ./shared/... ./test/...` — зелёные целиком.
`go test -tags e2e -count=3` — зелёные. Покрытие: `gateway` **89,5 %**, `swarm` **84,0 %**,
`swarm/template` **93,8 %**, `state` **87,3 %** — цифры `dev-log` подтверждены до десятой.
`golangci-lint run` по `shared/testkit/{gateway,swarm,state}/... ./test/...` — **0 issues**,
с `--build-tags e2e` — тоже **0 issues**; `gofmt -l` пуст. Сквозной тест публикует **46 событий**
всех топиков, все валидны по реестру, `dead_letters` пуст — цифра `dev-log` подтверждена.

**Мутации и зонды (все сняты).**

| № | Мутация / зонд | Ответ |
|---|---|---|
| A | `state.go`: снять `select` в `Start` (сигнал без ожидания отчёта шины) | **60 из 60 красных**, «publish analytics.replay.completed while the subscription was still being made» |
| — | исправный код, тот же тест, 60 подряд + 80 под восьмикратной нагрузкой | **0 из 140 красных** |
| B | `state.go`: `subscribe` всегда уходит на запасную ветку (`ok && false`) | **20 из 20 красных**, «Start returned…» — тест пинит `Start`, а не ворота |
| E | `readySubscriber`: дрейф сигнатуры | не компилируется — направление закрыто компилятором |
| F | `apply.go`: перенести обновление курсора ниже фильтра чужого мира | **30 из 30 красных**, «cursor 0, want 42» |
| Z | зонд Minor-7: обычная `Bus` с падающим `Subscribe`, 60 прогонов | `Start` — **0 из 60**, `Wait` — **60 из 60** |
| G1 | `scenario.go`: **добавить** шаг в `visit` (ещё один `look`) | e2e **зелёный**, событий 46 → **52** (+3 действия, +3 нарратива) — ожидания выводятся, литералов нет |
| G2 | `Scenario`: бегунок **роняет последний шаг** скрипта | **красный**: «14 player actions on the bus, the script has 15» |
| G3 | `Scenario`: бегунок **меняет порядок** `look`/`say` внутри каждого визита | **красный**, шесть строк вида «action 2 is {player.said player-A}, the script says {player.looked player-A}» |
| L | `harness.go`: `Rest` не публикует предложение | **красный**: «6 entity.updated on the bus, the script asks for 9» |
| H3 | `narrated`: **появился** `combat.decided → turn` | **красный** (решение по `combat.decided` закреплено) |
| H1 | `narrated`: **пропал** `player.looked` | **красный** (3 теста); e2e при этом зелёный — правильно, ожидания выведены |
| H4 | `narrated`: появился `npc.moved → entry` (типа нет в списке теста) | **всё зелёное** → Minor-2 |
| I1 | нарратор не кладёт `laws_version` (событие невалидно по схеме) | **красный за 10,04 с**, «timed out waiting for the narratives…» |
| I2 | то же + проверка недоставленных поднята выше ожидания нарративов | **красный за 0,04 с** с полной ошибкой схемы и id причины → Minor-3 |
| J | `fieldsFor`: здоровье как константа `10/10` | `TestTheTextCarriesWhatStateSaid` **красный** — подстановка здоровья настоящая, из проекции по фактам; e2e при этом зелёный (текстов не проверяет) |
| M | 15 прогонов e2e, сверка множества текстов нарративов | **15 из 15 одинаковы**; при этом идентификатор одного и того же нарратива между прогонами гулял (`e2e-26` / `e2e-27`) — см. риски |
| K | «пустой мир» с заведомо неверными флагами | красный за **63,4 с**; stderr процесса сохранён и напечатан → Nit-3 |
| — | «пустой мир», 5 прогонов подряд | **0 из 5 красных** |

**Отдельно проверено по DoD и контрактам.**

- **Ожидание ответа State** (`harness.go:395-419`): тихого зависания нет — `awaitProposal` держит
  и `ctx.Done()`, и срок (`DefaultTimeout = 5 с`), и сообщение называет `proposal_id` и срок;
  `TestAnActionGivesUpWhenNobodyAnswers` пинит это с 50 мс. Отказ State приходит **ошибкой**
  действия с названной причиной (`TestARefusalStopsTheAction`, `duplicate_entity`), поэтому падение
  сценария в e2e не превращается в таймаут: `Scenario` возвращает «step N (rest player-A): …
  refused: …» немедленно.
- **Конверт всего, что публикует харнесс**: `source=testkit/gateway`, `actor_kind=ci`,
  `correlation_id = id`, `meta.agent` отсутствует — проверяется и в unit
  (`TestEverythingTheHarnessPublishesIsValid`, включая `Spec.Publishers`), и в e2e.
  `Outside(worldID)` = `outside:{world_id}` — совпадает с конвенцией `data-model.md`.
- **Нарратив против C-05**: `kind`, `generated_by=template`, `locale`, `laws_version`,
  `filter{applied,status,filter_version}`, `narrative_event_id = id`, `based_on[0]` = причина,
  `meta.agent{fake-narrator, task, 0.0.0}`, непустой `recipients[]` — все проверены в e2e
  поэлементно; пустой раунд не издаётся (`recipients.minItems = 1`).
- **Выбор шаблона по хэшу**: `template.pick` — FNV-1a от `ev.ID`, без карт и без случайности;
  `TestTheSameEventAlwaysGetsTheSameText` даёт 50 повторов и отдельно проверяет, что разные
  события не схлопываются в один текст. Воспроизводимость по одному и тому же журналу —
  по построению.
- **Прежние тесты T-017 не ослабли**: пакет `state` — 31 тест / 44 запуска, все зелёные; ни одна
  из трёх мутаций по прежним замечаниям (A, B, F) не позеленела; `TestOneEntityNamedTwiceIsRefused`
  (Major-1 ревью #2) и матрица отказов на месте.

### Открытые вопросы

1. **`cmd/multiverse/fake_contexts.go` и `MV_SWARM_FAKE` в дереве отсутствуют**, при этом
   `ownership.md` §1 называет этот файл задачей **EPIC-001 (F-2/F-10)**, `contracts.md` §0 и §17
   монтируют фейки в `core` именно этим хуком, а `.golangci.yml:76` уже держит для него именное
   исключение `depguard`. В DoD T-018 хука нет, поэтому я его не требую. Вопрос оркестратору:
   хук — остаток волны 0 (тогда нужна задача до T-020) или он приходит вместе с `FakeEncounter`
   (EPIC-003 C4)? Ответ стоит записать, иначе на приёмке волны 0 он всплывёт как «не сделано».
2. **Открытые вопросы исполнителя №1, 2, 5** (`player.looked` — `entry` или `turn`; `scope` в
   предложении при движении; кто заводит `kind=turn/death/world_event`) поставлены верно и
   адресованы правильно; №1 уже решён оркестратором в пользу `entry` — правка C-05 остаётся за
   architect#1 (бридж-блок волны 1).
3. **Развилка `readySubscriber` / гарантия «новая группа читает с первого офсета» в C-01**
   (открытый вопрос T-017) остаётся открытой. Nit-4 сделал допущение видимым, но развилку не
   закрыл; это по-прежнему заявка к архитектору, а не долг разработчика.

### Предложения в бэклог (не требую в этой задаче)

- Неблокирующий режим `Harness` для нагрузочного переиспользования в EPIC-004 (ограничение v0
  названо исполнителем в `dev-log` §2.8 п. 3 — согласен, это заявка, а не дефект).
- `assertEverythingIsValid` при валидирующей на публикации шине проверить может только то, что
  шина уже проверила; ценность у неё — счёт и охват топиков, на которые никто не подписан. Тест
  сам это признаёт в комментарии; если в волне 1 появится издатель мимо `Publish`, ценность
  вернётся.
- Ожидание нарративов по стенным часам (10 с) заменить на детерминированную точку, когда у
  `membus` появится способ сказать «всё разослано» (сегодня его нет — это T-014, чужая область).

### Риски и допущения

- **`-race` не прогонялся** (недоступен на этой машине). Новое разделяемое состояние — `Harness.mu`
  (версии, позиции, `settled`, пересоздаваемый под замком канал `changed`) и `FakeNarrator.mu`
  (проекция персонажей); `WithLog`/`WithTimeout`/`WithLaws` замка не берут намеренно и вызываются
  до `Start` — это сказано в их doc-комментариях. Чтение кода расхождений не показало
  (`record`/`Version`/`Position` целиком под замком, `changed` закрывается и заменяется под тем же
  замком), но утверждать «гонок нет» на основании чтения я не буду. Под `-race` смотреть сюда
  первым делом.
- **Идентификаторы событий в сквозном тесте между прогонами не стабильны.** Замер M: один и тот же
  нарратив получил `e2e-26` в одном прогоне и `e2e-27` в другом — детерминированный источник id
  общий, а публикуют харнесс, State и нарратор из разных горутин. Тексты при этом совпали 15 раз
  из 15 (индекс шаблона по модулю уцелел), и сегодня это ничем не грозит: текстов тест не
  проверяет. Но тот, кто в I1-α захочет закрепить конкретный текст в e2e, обязан сначала закрепить
  идентификатор — иначе получит флак, объяснимый только этим замером.
- **Все прогоны — на машине владельца** (32 ядра). Раннер GitHub (2–4 vCPU) не эмулировался;
  направление сноса для ворот Minor-6 при этом безопасное (медленная машина только удлиняет
  ожидание сломанного кода, а исправный за воротами не проходит вовсе), а вот 60-секундный
  бюджет «пустого мира» и 10-секундное ожидание нарративов на медленном раннере — первые
  кандидаты в долгие красные.
- **`kafka.go` и `membus.go` в этот раз не открывались** (чужая область с живыми зондами).
  Утверждения Nit-5 про `StartOffset: kafka.FirstOffset` приняты по своему же чтению в ревью #2,
  заново не подтверждались.

---

## T-019 · приёмка tech-lead#1 · 2026-09-10

### Границы ревью

Только документы: `README.md`, `CLAUDE.md`, `AGENTS.md`, `Docs/ops/runbook.md`,
`services/_archive/README.md`, поле `stack` в `.dev-team.json`. Код не открывался на предмет
правок: `shared/testkit/**` и `test/e2e/**` в это же время ревьюит code-reviewer#1 (T-018).
Исходники читались только как источник истины для проверки утверждений документа.
Оцениваются и правки оркестратора, внесённые после сдачи задачи (восемь ссылок `docs/` → `Docs/`,
удаление профиля `bot` из набора по умолчанию в `.env.example`).

Метод: не «читается ли текст», а «истинно ли каждое утверждение». Команды прогонялись; цели
`make` сверялись с `Makefile` построчно; подкоманды — запуском собранных бинарников.

### Вердикт

**ВЕРНУТЬ** — Critical 0, Major 2, Minor 4, Nit 6.

Работа сделана добросовестно и по существу: устаревшая картина (пятнадцать сервисов, `go.work`,
Chroma/Timescale/Redis как основной стек, цели `make build-service`/`build-all`/`run SERVICE=`,
ссылки на несуществующие `services/<имя>/AGENTS.md`) вычищена полностью — поиск по всем шести
файлам показывает только отрицательные упоминания («`go.work` больше нет», «целей … больше нет»).
Таблица статусов сходится с деревом до последнего каталога. Ни одной несуществующей цели `make`
и ни одной несуществующей подкоманды `mvctl`/`multiverse` в документах нет.

Возврат — из-за двух утверждений, которые сегодня ложны, причём одно из них (Ma-1) отменяет
центральное обещание документа «запуск за 5 команд» и пункт готовности задачи «команды из README
выполняются на чистой машине».

### Замечания по серьёзности

#### Major

**Ma-1. `README.md:33–45` («Запуск за 5 команд») и `Docs/ops/runbook.md:33–35` — `make up` на
чистой машине падает, не запустив ни одного контейнера.** Причина не в профилях: `docker compose`
интерполирует **весь** файл до того, как отфильтровать сервисы по профилям, а в
`docker-compose.yml` есть две обязательные переменные (`${VAR:?}`) у сервисов вне набора по
умолчанию — `MV_TELEGRAM_BOT_TOKEN` (сервис `telegram-bot`, профиль `bot`; в `.env.example`
пуст) и `CHROMA_IMAGE` (сервис `chromadb`, профиль `legacy`; в `build/versions.env` намеренно
пуст, решение D-3/T-008). Доказательство (окружение = заполненный `.env` минус эти две
переменные, набор профилей — новый по умолчанию):

```
$ export MINIO_ROOT_USER=u MINIO_ROOT_PASSWORD=pppppppppppppppp MV_MINIO_ACCESS_KEY=a \
         MV_MINIO_SECRET_KEY=s NEO4J_PASSWORD=n MV_NEO4J_PASSWORD=n \
         COMPOSE_PROFILES=memory COMPOSE_ENV_FILES=build/versions.env
$ docker compose config -q
error while interpolating services.telegram-bot.environment.MV_TELEGRAM_BOT_TOKEN:
  required variable MV_TELEGRAM_BOT_TOKEN is missing a value: set MV_TELEGRAM_BOT_TOKEN in .env
$ docker compose up -d --dry-run
error while interpolating services.chromadb.image:
  required variable CHROMA_IMAGE is missing a value: set CHROMA_IMAGE in build/versions.env …
```
(порядок двух ошибок между прогонами меняется — обе блокирующие; `docker compose version` = v5.2.0,
демон 29.6.1; с `--env-file .github/ci.env` та же команда проходит и печатает ровно
`core gateway memory minio minio-init neo4j qdrant redpanda redpanda-init`).

Следствия: (1) обещание «5 команд» ложно — четвёртая команда не работает; (2) правка оркестратора
в `.env.example` (убран `bot` из `COMPOSE_PROFILES`) **проблему не решает** — интерполяция от
профиля не зависит, и запись в журнале «`make up` с новым набором по умолчанию поднимает только
существующее» верна лишь как описание состава, но не как факт запуска; (3) под тем же ударом
пункт готовности эпика в T-020 («`make up` поднимает инфраструктуру и пустые контексты»).

Владение: `docker-compose.yml`, `.env.example`, `build/versions.env` — tech-lead#1 / devops, не
tech-writer. Поэтому **исправление документа тут не требуется**, требуется решение по compose:
`:?` не умеет означать «обязательна, когда её профиль активен», а именно это имелось в виду в
T-008. Отдельная задача волны 1 (запрос оркестратору на её заведение — см. «Открытые вопросы»);
приёмка T-019 и стендовая проверка «5 команд» человеком — **после** неё, иначе проверять нечего.

**Ma-2. `README.md:53–57` — врезка про профиль `bot` описывает файл, которого больше нет.**
Текст утверждает: «`.env.example` по умолчанию перечисляет его в `COMPOSE_PROFILES=memory,bot`…
Уберите `bot` из `COMPOSE_PROFILES`». Факт после правки оркестратора
(`.env.example:13–16`): `COMPOSE_PROFILES=memory`, а над строкой — комментарий, почему `bot` в
наборе не стоит. То есть README (а) утверждает про содержимое файла в репозитории неправду,
(б) требует от читателя действия, которое уже сделано, (в) косвенно ломает и комментарий в
блоке команд (`README.md:38–39` «`COMPOSE_PROFILES` оставить memory … см. примечание ниже» —
примечание ниже теперь противоречит самому себе). Проверено: `grep -n COMPOSE_PROFILES .env.example`.
Правка на три строки: врезку переписать как «профиль `bot` в наборе по умолчанию отсутствует,
потому что `cmd/telegram-bot` появится в EPIC-004; добавлять его в `COMPOSE_PROFILES` до этого
не нужно». Это единственная правка, которая возвращается исполнителю.

#### Minor

**Mi-1. `Docs/ops/runbook.md:145` — команда `mvctl session-report` не существует и никогда не
существовала под этим именем.** Проверено запуском: `mvctl session-report` →
`mvctl: unknown command "session-report"`, код 2. В реестре (`cmd/mvctl/main.go`, вывод
`mvctl --help`) зарезервировано имя `report` («session and audit reports as CSV [reserved,
EPIC-005]»), и сам runbook в шапке (строка 13) перечисляет именно `report`. Файл противоречит
сам себе; имя пришло из `infrastructure.md` §9.4/§9.7, где оно тоже неточно. Заменить на
`mvctl report`; `mvctl llm usage` в той же строке допустимо (подкоманда зарезервированного
`llm`), но лучше — `mvctl llm`.

**Mi-2. `services/_archive/README.md:12–16` — правило устарело и теперь ложно.** Текст: «Исключение
архива из `.golangci.yml` (T-003), `.dockerignore` и `Makefile`/`docker-compose.yml` (T-004,
T-008) и `CODEOWNERS` (T-012) — задачи волны 0: **этих файлов ещё нет** либо они ссылаются на
исходные (перенесённые) пути». Все файлы существуют и исключения в них есть:
`.golangci.yml:217,274` (`services/_archive`), `.dockerignore:46` (`services/`),
`.github/CODEOWNERS:17,40`. Задача T-019 этот файл трогала (добавлен раздел «Смотрите также»),
привести абзац в настоящее время — работа на одну строку.

**Mi-3. `README.md:18–31` («Требования») не покрывает того, что README же и требует.** Пункт
«Участие в разработке» (строка 161) требует зелёный `make ci` до PR, а `make ci` —
это `lint test contracts secrets-scan privacy-scan vuln compose-lint test-e2e`, то есть нужны
`golangci-lint` (пин `GOLANGCI_LINT_VERSION=v2.13.2` в `build/versions.env`), `govulncheck`,
`gitleaks` (в README помечен «опционально») и `pre-commit` для хука из T-001. В списке — только
`make`, `jq`, `age`, опционально `gitleaks`. На этой машине первые три установлены
(`golangci-lint 2.13.2`, `gitleaks`, `govulncheck`), `pre-commit` — нет; на чистой машине не
будет ни одного. Добавить строку «инструменты разработчика» со ссылкой на `build/versions.env`.

**Mi-4. `CLAUDE.md` и `AGENTS.md` называют только один из четырёх запретов линтера.**
`AGENTS.md:45–47` и разделы про конфигурацию описывают запрет `os.Getenv`/`os.LookupEnv` вне
`shared/env`, но `forbidigo` в `.golangci.yml` запрещает ещё `time.Now`, `time.After/Tick/
NewTimer/NewTicker/Since/Until` (только через `shared/clock`, иначе replay не воспроизводим) и
`log.Print*`/`Fatal*`/`Panic*` (только `slog`). Это файлы-инструкции для агентов: агент, который
напишет `time.Now()` по CLAUDE.md, узнает о правиле только от красного `make lint`. Одна строка
рядом с существующей.

#### Nit

1. `README.md:47` — набор по умолчанию перечислен как `redpanda, minio, gateway, core`; фактически
   `docker compose config --services` без профилей даёт ещё `redpanda-init` и `minio-init`
   (runbook про них говорит, README — нет).
2. `README.md:121` — в строке «Архив» перечислены `shared/{…}`, `fake_deps/`, `test_minio.go`,
   «старые `Dockerfile`», но не `configs/gm_*.yaml` (6 файлов) и не `shared/agent/tools/*` +
   `filter.go`; в `CLAUDE.md`/`AGENTS.md` та же строка честно заканчивается «и др.». Либо «и др.»,
   либо полный список.
3. `Docs/ops/runbook.md:147` — «`ops/metrics/` содержит только материалы замера LLM (`baseline.md`,
   `bench-matrix.json`)»: в каталоге есть ещё `README.md`, а `baseline.md` — **шаблон** («Статус:
   ШАБЛОН, заполняется после прогона на стенде», T-013 в волну 1). Точнее: «заготовки замера».
4. `README.md:28` — `FiloSottile.age` описан как «шифрование бэкапа `links.db`»; `age` сегодня не
   вызывает ни одна цель `Makefile` и ни один скрипт, объявлена только переменная
   `MV_BACKUP_AGE_RECIPIENT` (`shared/env/vars.go:62`), а сам `links.db` появится в EPIC-004.
   Пометить «понадобится с EPIC-004», как это сделано для остальных «зарезервированных» вещей.
5. `README.md:21` — «Docker Desktop (Compose v2, `docker compose`, не `docker-compose`)»: на стенде
   владельца `docker compose version` = **v5.2.0**. Формулировка про «v2» читается как требование
   версии; имелось в виду «плагин, а не бинарник v1».
6. `README.md:99` — `build/` описан как «Dockerfile платформы, minio.Dockerfile, versions.env»;
   в каталоге ещё `legacy.Dockerfile`, `minio-init.sh`, `redpanda-init.sh`.
7. `.dev-team.json.stack` — единственное поле артефактов, написанное по-английски при
   `"language": "ru"` (предыдущее значение тоже было английским, так что это не регресс).

### Что проверено запуском

| Что | Команда | Результат |
|---|---|---|
| Реестр команд `mvctl` против README/CLAUDE/AGENTS/runbook | `go run ./cmd/mvctl --help` | 15 команд; реализованы `contracts`, `env`, `storage`, `privacy`, `version`; зарезервированы `world`, `blueprint`, `laws`, `record`, `golden`, `llm`, `memory`, `report`, `trace` — ровно так, как написано в документах |
| «Зарезервированное» действительно отсутствует | `mvctl world init`, `mvctl memory rebuild`, `mvctl report` | `not implemented in this build (EPIC-002 / EPIC-005)`, код 2 — читатель не примет за рабочее |
| То же для `multiverse db` | `multiverse db backup`, `db check` | `no database is compiled into this process yet (EPIC-004, ADR-019)`, код 1 — совпадает с формулировкой runbook §4 |
| `mvctl session-report` (runbook:145) | `mvctl session-report` | `unknown command`, код 2 → Mi-1 |
| Флаги `cmd/multiverse` против карты пакетов | `multiverse --help` | `--bus/--contexts/--mode/--recording/--id-source` — как в README:71 и CLAUDE:35 |
| Обещание CLAUDE.md:132 | `MV_CORE_ADDR=127.0.0.1:8097 multiverse --contexts=all --bus=memory` + `curl /health` | 200, `{"status":"ok"}` со всеми семью контекстами (`state,laws,mechanics,llm,swarm,gateway,memory`) — заглушки отвечают ровно как описано |
| `make contracts` по частям (make на машине не установлен) | `mvctl contracts check`; `mvctl env check` | `65 types, 8 topics, 58 schema files checked`, код 0; `69 variables declared, compared with .env.example`, код 0 |
| Общий DoD §1 (линтер) | `golangci-lint run` | `0 issues`, код 0 |
| Состав compose по умолчанию | `docker compose --env-file build/versions.env --env-file .github/ci.env config --services` | `core gateway memory minio minio-init neo4j qdrant redpanda redpanda-init` — только существующее, `telegram-bot` отсутствует (правка оркестратора по составу верна) |
| Тот же запуск без `ci.env` (= чистая машина) | `docker compose config -q`, `docker compose up -d --dry-run` | падение на интерполяции → **Ma-1** |
| Ссылки на runbook в старом регистре | `grep -n "docs/" <шесть файлов> \| grep -v "Docs/"` | пусто; в индексе файл записан как `Docs/ops/runbook.md` — правка оркестратора верна |
| Все относительные ссылки шести файлов | обход `](…)` с проверкой существования | ни одной битой; якоря `#статус-кода-вне-единого-модуля`, `#статус-кода-в-services` соответствуют заголовкам |
| Следы старой картины | `grep` по `go.work`, `build-service`, `build-all`, `run SERVICE`, `logs-service`, `Chroma`, `Timescale`, `Redis`, `services/*/AGENTS.md`, «15 сервисов» | все вхождения — отрицательные («больше нет», «не используется», «только профиль `legacy`»); `find services -name AGENTS.md` → пусто, как и сказано в `AGENTS.md:131` |
| Таблица статусов против дерева | `ls services/`, `ls services/*/FROZEN.md`, `find services/_archive -name ARCHIVED.md`, `go list ./...` | 13 каталогов = 3 источника переписывания + 2 legacy + 8 замороженных (`FROZEN.md` ровно 8); архив — 15 `ARCHIVED.md`; `go list ./...` не содержит ни `services/`, ни `_archive`; у каждого каталога свой `go.mod` — как написано |
| Каждая цель `make` из README/CLAUDE/AGENTS/runbook | сверка с `Makefile` (`make` на машине не установлен, `winget install ezwinports.make` из README как раз про это) | все существуют: `build lint test test-integration test-e2e contracts secrets-scan ci ci-full image minio-image up down reset logs health llm-up llm-down llm-health backup restore archive-legacy help`; описания («`up` = `compose up -d --wait` + `health` не строго по LLM», «`backup` останавливает `core`/`redpanda` и исключает `prompts-*`», «`archive-legacy` → `backups/legacy-<date>/`», `ROUTER=1`) совпадают с рецептами |
| Числа в runbook | `scripts/llm-server.ps1:219–233`, `llm-server.sh:204–218` | «до 180 с», `ops/llm-server.pid`, `ops/llm-server.log` — верно |
| Поле `stack` | `go.mod`, `build/versions.env`, `docker-compose.yml` | Go 1.26 (`toolchain go1.26.8`, `GO_VERSION=1.26.8`), единый модуль, Redpanda, MinIO из исходников, Qdrant+Neo4j, Chroma только в профиле `legacy`, llama-server нативно + Ollama по профилю `gpu`, Timescale/Redis отсутствуют — соответствует факту |
| Карточки процессов runbook §8 | `docker-compose.yml` | команды и порты сходятся, включая порядок контекстов `core` = `state,mechanics,laws,llm,swarm` (в `infrastructure.md` §9.8 порядок другой — runbook правильно следует коду, а не документу) |

### Открытые вопросы (оркестратору)

1. **Задача на compose по Ma-1** (моё владение, не исполнителя): решить, как выразить «переменная
   обязательна, когда активен её профиль». Варианты: (а) `${MV_TELEGRAM_BOT_TOKEN:-}` +
   `${CHROMA_IMAGE:-}` и громкая проверка в точке использования (бот — EPIC-004, chroma — при
   подъёме профиля `legacy` на стенде); (б) оставить `:?` и заставить `make up` подмешивать
   `--env-file` с заглушками — хуже, тихо ломает смысл «громкого» отказа; (в) вынести оба сервиса
   в `docker-compose.bot.yml`/`docker-compose.legacy.yml` и подключать их из `Makefile` только
   когда профиль назван — дороже, но единственный способ сохранить `:?` буквально. Прошу завести
   задачу волны 1 (счётчик 396 → 397) до T-020: пункт готовности эпика («`make up` поднимает
   инфраструктуру») сегодня не выполним.
2. **Устаревшие документы вне списка файлов T-019** — ровно та болезнь, ради которой задача
   и заводилась, но по другим адресам: `QWEN.md` (723 строки; таблица стека с ChromaDB и
   TimescaleDB, цели `make build-service SERVICE=`, `make run SERVICE=`, таблица пятнадцати
   сервисов с портами) — это файл-инструкция для агента того же класса, что `CLAUDE.md`;
   плюс `AI_AGENT_INSTRUCTIONS.md`, `README_LIVING_WORLDS.md`, `AUTOMATION-SETUP.md` и шесть
   файлов `docs/LIVING_WORLDS_*.md`. Расширять объём T-019 задним числом не буду — прошу
   отдельную задачу.
3. **Каталог `docs/` в индексе всё-таки есть** — вопреки записи в журнале от 2026-09-11
   («отдельного `docs/` в индексе нет»). Факт: `git ls-files | grep -c '^docs/'` = **6**,
   `'^Docs/'` = **83**, сумма и даёт те самые 89. Реестр Git регистр хранит, `core.ignorecase=true`
   лишь скрывает это на машине владельца: на сборщике с чувствительной ФС появятся два каталога.
   Вывод для ссылок из Ma/Mi не меняется (`Docs/ops/runbook.md` записан именно так и проверен),
   но фразу журнала стоит поправить, а судьбу шести файлов решить вместе с п. 2.
4. **U-11 (каталоги среды разработки в индексе)** пунктом `tasks.md` был поручен tech-writer
   в T-019; оркестратор уже вынес его в T-396. Подтверждаю решение: это не документарная работа.
   В приёмке T-019 этот пункт не проверялся и на вердикт не влияет.

### Риски и допущения

- **`make` на машине не установлен** (`which make` → пусто, в `winget`-ссылках README как раз
  `ezwinports.make`), поэтому ни одна цель целиком не прогонялась: содержимое целей сверялось
  чтением `Makefile` и прогоном их составных частей (`mvctl contracts check`, `mvctl env check`,
  `golangci-lint run`). Утверждения вида «`make up` делает то, что написано» проверены на уровне
  рецепта и на уровне `docker compose`, но не на уровне `make`.
- **Ничего, что поднимает контейнеры или трогает GPU, не запускалось** (`make up`, `llm-up`,
  `minio-image`, `test-integration`). Ma-1 получен `--dry-run`/`config`, то есть до создания
  контейнеров; вывод «падает раньше, чем что-то стартует» опирается на то, что `compose` загружает
  и интерполирует модель до фильтрации по профилям — это и подтверждено двумя разными командами.
- **Стендовый пункт задачи «5 команд на чистой машине» человеком не выполнялся** и, пока живёт
  Ma-1, выполняться не должен — он гарантированно красный.
- **Прогон на машине владельца**, Windows 11 + Git Bash, `core.ignorecase=true`. Всё, что связано
  с регистром путей (`Docs/` против `docs/`), здесь принципиально не воспроизводится и проверялось
  по индексу Git, а не по файловой системе.
- Тексты `infrastructure.md` §9 и `epics.md` со словом «`docs/`» я не трогал: их правка — за
  architect#1 в бридж-блоке волны 1 (решение оркестратора), и на вердикт по T-019 она не влияет.

---

## T-019 · приёмка #2 (итерации 2–3) · 2026-09-10 · tech-lead#1

### Границы и метод

Область та же: `README.md`, `CLAUDE.md`, `AGENTS.md`, `Docs/ops/runbook.md`,
`services/_archive/README.md`, поле `stack` в `.dev-team.json`. Файлы T-397
(`docker-compose*.yml`, `Makefile`, `.env.example`, `scripts/compose-lint.sh`,
`testdata/compose-lint/**`, `.github/**`) читались **только как источник истины**; их
реализацию ревьюит code-reviewer#2, замечаний по ней здесь нет.

Метод не менялся: каждое утверждение проверялось своим прогоном или поиском по файлу
истины, отчёт исполнителя как доказательство не принимался. GNU make на машине
по-прежнему нет — цели проверялись чтением рецептов и прогоном их составных частей
(`docker compose ... config`, `mvctl`, `golangci-lint`); это известный пробел владельца,
он записан в журнал и в шапку runbook, и дефектом задачи не считается. Контейнеры не
поднимались, GPU не трогался.

### Вердикт

**ВЕРНУТЬ** (итерация 4) — Critical 0, Major 2, Minor 5, Nit 5.

Обе прежние Major и все четыре Minor закрыты по существу, шесть из семи мелких — тоже;
перечень запретов линтера сверен с `.golangci.yml` дословно и сходится. Возврат — из-за
одного файла, который в итерации 3 остался вне правки: `CLAUDE.md` описывает раскладку
профилей в том виде, в каком она была **до** T-397, и продолжает перечислять обязательные
переменные списком (неполным) вместо правила. Это тот же класс ошибки, за который
возвращались Ma-1/Ma-2, и в том файле, который читают агенты следующих эпиков.

Объём итерации 4 заморожен: N-1…N-7 ниже плюс запись в `dev-log.md`. Ничего из уже
закрытого перепроверять не нужно — при повторной сдаче я сверяю только их.

### Статус прежних замечаний

| # | Статус | Чем доказано |
|---|---|---|
| **Ma-1** (`make up` падал на интерполяции до старта контейнеров) | **закрыто** (T-397) | Собрана «чистая машина» из `.env.example` (заполнены только помеченные `[required]`) + `build/versions.env`. `docker compose -f docker-compose.yml config --services` при `COMPOSE_PROFILES=` даёт ровно `core gateway minio minio-init redpanda redpanda-init` без единой ошибки интерполяции; с `COMPOSE_PROFILES=memory` добавляются `memory neo4j qdrant`. Прежние две ошибки (`MV_TELEGRAM_BOT_TOKEN`, `CHROMA_IMAGE`) не воспроизводятся. Громкий отказ жив и переехал: `-f docker-compose.bot.yml --profile bot` → `required variable MV_TELEGRAM_BOT_TOKEN … (D-10)`; `-f docker-compose.legacy.yml --profile legacy` → `required variable CHROMA_IMAGE … (T-008, D-3)`. Документная часть — в README и runbook верна, в `CLAUDE.md` нет (N-1) |
| **Ma-2** (врезка README про профиль `bot` описывала прежний `.env.example`) | **закрыто** | `README.md:63–75` переписан целиком. Сверено с файлом: `.env.example:33–36` — `COMPOSE_PROFILES=memory`, над строкой комментарий, почему `bot` в наборе не стоит; врезка больше не требует от читателя действия, которое уже сделано, и не противоречит блоку команд выше |
| **Mi-1** (`mvctl session-report`) | **закрыто** | `Docs/ops/runbook.md:182` — `mvctl report`, `mvctl llm`. Прогон: `mvctl report` → `not implemented in this build (EPIC-005)`, код 2; `mvctl llm usage` → то же; `mvctl session-report` → `unknown command`, код 2. `grep session-report` по шести файлам — пусто. Шапка runbook (строки 13–14) перечисляет девять зарезервированных имён — совпадает с `mvctl --help` |
| **Mi-2** (абзац архива про несуществующие файлы исключения) | **закрыто** | `services/_archive/README.md:12–18` переписан на факт, и каждая ссылка проверена: `.golangci.yml:217,274` = `services/_archive` + `services/`; `.dockerignore:46` = `services/`; `.github/CODEOWNERS:40` = `/services/_archive/`, комментарий на строке 17; `grep _archive Makefile docker-compose.yml` — пусто (утверждение «не упоминают вовсе» верно); в `.gitleaks.toml`/`.gitleaksignore` архива нет (сказано «не исключён» — верно); `.pre-commit-config.yaml` — пять `exclude: '^services/_archive/'` |
| **Mi-3** (требования не покрывают того, что нужно `make ci`) | **закрыто частично → N-4** | Строка про инструменты добавлена (`README.md:25–30`), три пина совпадают с `build/versions.env`. Но три утверждения в ней неверны или неполны — вынесено в N-4 |
| **Mi-4** (один запрет линтера из четырёх) | **закрыто** | `CLAUDE.md:138–148` — таблица из четырёх строк, раздела в этом файле раньше не было вовсе; `AGENTS.md:45–53` — три абзаца. Сверено дословно с `.golangci.yml:201–210`: семейства (`os.Getenv/LookupEnv/Environ/ExpandEnv`; `time.Now`; `time.After/Tick/NewTimer/NewTicker/Since/Until`; `log.Print*/Fatal*/Panic*`), состав функций и тексты причин (манифест и `.env.example` NFR-074; воспроизводимость replay; настенное время таймеров; только структурные JSON-логи) совпадают со всеми четырьмя `msg` |
| Мелкое 1 (init-контейнеры) | **закрыто** | `README.md:55–56`, `runbook:47–49`. `config --services` без профилей возвращает `redpanda-init`/`minio-init` — оба теперь названы |
| Мелкое 2 (неполный список архива) | **закрыто** | `README.md:141` дополнен `configs/gm_*.yaml` (6 файлов), `shared/agent/tools/*` (кроме `registry.go`) и `filter.go`. Сверено со строками 47–64 и 75 таблицы `services/_archive/README.md` — совпадает |
| Мелкое 3 (`ops/metrics/`) | **закрыто** | `runbook:184–185` — «заготовки замера», перечислены `README.md`, `bench-matrix.json`, `baseline.md` со статусом ШАБЛОН. `ls ops/metrics/` = ровно эти три файла; `baseline.md` начинается со строки «Статус: ШАБЛОН, заполняется после прогона на стенде» |
| Мелкое 4 (`age`) | **закрыто** | `README.md:35–36` — «сегодня не вызывается ни одной целью Makefile, понадобится с EPIC-004». `grep` по `Makefile` и `scripts/` подтверждает: ни одна цель `age` не вызывает |
| Мелкое 5 (Compose «v2») | **закрыто** | `README.md:21–22` — «плагин Docker CLI, а не отдельный бинарник `docker-compose` v1; версия плагина на стенде владельца — v5.2.0» |
| Мелкое 6 (`build/`) | **закрыто в README, не закрыто в двух других** | `README.md:117–118` перечисляет шесть файлов — `ls build/` даёт ровно их. `CLAUDE.md` и `AGENTS.md` сохранили прежний неполный перечень без «и др.» → Nit 2 |
| Мелкое 7 (`stack` по-английски при `language: ru`) | **не менялось, принято** | Значение поля проверено и фактически верно (Go 1.26, единый модуль, Redpanda, MinIO из исходников, Qdrant+Neo4j, Chroma только в профиле `legacy`, llama-server + опционально Ollama). Язык поля — не регресс, на вердикт не влияет |

### Новые замечания

#### Major

**N-1. `CLAUDE.md` (карта каталогов, строка `docker-compose.yml`; раздел «Ключевые файлы»)
описывает раскладку профилей до T-397.**
В карте: `docker-compose.yml  # профили: (default) / memory / gpu / bot / dev / legacy`;
в «Ключевых файлах»: «`docker-compose.yml` — полный стек инфраструктуры и профили».
Оба утверждения сегодня ложны: сервисы профилей `bot` и `legacy` вынесены в
`docker-compose.bot.yml` и `docker-compose.legacy.yml` (проверено `grep -E '^  [a-z-]+:|profiles:'`
по трём файлам: `telegram-bot` — только в `.bot.yml`; `chromadb`, `semantic-memory`,
`narrative-orchestrator` — только в `.legacy.yml`), и ни одного из двух новых файлов
`CLAUDE.md` не упоминает вовсе. `README.md:119–120` в итерации 3 поправлен правильно —
`CLAUDE.md` остался на прежней картине.

Почему Major, а не Minor: это файл-инструкция, по которому агент EPIC-002…EPIC-005 строит
представление о репозитории; ровно за такое утверждение («врезка описывает файл, которого
больше нет») возвращалось Ma-2. Агент, добавляющий сервис профиля в `docker-compose.yml`
по этой карте, воспроизводит дефект, который T-397 только что закрыл, — его поймает
правило 7 линтера, но карта ведёт в обратную сторону.

Правка: в карте — `docker-compose.yml  # профили: (default) / memory / gpu / dev`, ниже
добавить `docker-compose.bot.yml` и `docker-compose.legacy.yml` с пометкой «подключает
Makefile по `PROFILES=…`»; в «Ключевых файлах» снять слово «полный» и сослаться на врезку
README.

**N-2 (процесс). Записи о T-019 в `dev-log.md` нет ни одной — ни за первую сдачу,
ни за итерации 2 и 3.** Общий DoD §1 п. 5 требует запись с исполнителем, задачей, что
сделано, отклонениями и открытыми вопросами. Проверено: `grep -c 'tech-writer ·'` по
`dev-log.md` = 0; последний раздел файла — `developer#1 · T-018`; заголовка `T-019`
в `Docs/dev-team/**` нет нигде, кроме `review.md`. Ход работы восстанавливается только по
трём записям `journal.md` от 2026-09-11 — это журнал оркестратора, а не журнал задачи, и
он не содержит ни списка правок, ни того, чем каждая подтверждена.

Достаточно одной записи за все три итерации: что изменено пофайлово, чем каждое утверждение
подтверждено, что осталось за владельцем (стендовая проверка «5 команд»).

Заодно моя часть: статус задачи не проставлен — `tasks.md:30` (`| F-9 | T-019 | todo |`)
и `tasks.md:275` (`Статус: todo`); проставлю при приёмке.

#### Minor

**N-3. `README.md:74–75` и `Docs/ops/runbook.md:67–69`: «Профиль `legacy` требует
`make legacy-src` перед первым `make up PROFILES=legacy`» — Makefile делает это сам.**
`Makefile:217`: `up: $(if $(filter legacy,$(ACTIVE_PROFILE_LIST)),legacy-src)` — цель
`legacy-src` стоит предпосылкой `up` и выполняется автоматически, как только `legacy`
попал в активный набор (через `PROFILES=` или `COMPOSE_PROFILES` в `.env`). Шапка
`docker-compose.legacy.yml:19–20` говорит то же самое: «`make up PROFILES=legacy` →
`make legacy-src` runs first». То есть оба документа предписывают лишний ручной шаг и
расходятся с двумя источниками истины сразу.

Правка — не вычёркиванием, а по факту: «`make up PROFILES=legacy` сам сначала выполняет
`legacy-src`, а он делает `rm -rf build/.legacy-src` и `git archive` из `LEGACY_SRC_REF`,
поэтому первый запуск профиля дольше обычного».

**N-4. `README.md:25–30` («Инструменты разработчика для `make ci`») — три неточности в
строке, добавленной по Mi-3.**
1. `pre-commit` в `make ci` не участвует: цель — `lint test contracts secrets-scan
   privacy-scan vuln compose-lint test-e2e` (`Makefile:191`), ни один рецепт `pre-commit`
   не вызывает. Он нужен для хука из T-001, а не для `make ci` — как и было
   сформулировано в исходном Mi-3.
2. «в CI все четыре ставятся автоматически» — неверно: в `.github/workflows/go.yml`
   ставятся `golangci-lint`, `gitleaks` и `govulncheck`; `pre-commit` не упоминается
   (`grep pre-commit .github/workflows/*.yml` — пусто).
3. Не назван инструмент, без которого `make ci` действительно красный: `python3`.
   `make ci` → `compose-lint` → `scripts/compose-lint.sh:98–100` завершается с
   `compose-lint: python3 is required to read the compose model`. То есть исходный пробел
   Mi-3 («список требований не покрывает того, что README же и требует») закрыт не до конца.

**N-5. Предупреждение о непрогнанных целях `make` стоит только в runbook.**
`Docs/ops/runbook.md:19–26` — врезка на месте, до раздела 1, текст честный и конкретный
(«ни один участник ни разу не выполнил ни одной цели `make`», «CI цели `make` тоже не
вызывает», что сделать владельцу). Это правильное место. Но первый запуск оператор делает
по `README.md` «Запуск за 5 команд» (строки 41–53), и там ни предупреждения, ни ссылки на
него нет: указатель на runbook стоит **после** блока команд (строка 77) и обещает
«полный порядок… диагностику», а не «эти команды никто не выполнял». Требование «прочтут
до первого запуска» для читателя README не выполнено.

Правка: одна строка-врезка над блоком команд README со ссылкой на врезку runbook.

**N-6. `CLAUDE.md` («Рабочий процесс разработки», п. 2) — обязательные переменные всё ещё
списком, и список неполон.**
«`cp .env.example .env`, заполнить обязательные переменные (`MINIO_ROOT_*`, `MV_MINIO_*`,
`NEO4J_PASSWORD` при профиле `memory`)». В `.env.example` пометок `[required]` шесть
(строки 48, 50, 52, 71, 73, 140); перечисленное покрывает пять, пропущен
**`MV_NEO4J_PASSWORD`** (`.env.example:140`, «`[required]` секрет; = `NEO4J_PASSWORD`»).
Итерация 3 заменила список правилом в `README.md:44–47` и `runbook:30–32` — в `CLAUDE.md`
он остался и уже разошёлся с файлом. Правило («заполнить всё, что помечено `[required]` в
комментарии над переменной») нужно и здесь: оно не устареет при следующей правке
`.env.example` и совпадает с тем, что проверяет правило 7 линтера.

**N-7. `Docs/ops/runbook.md:164–171` (ротация токена бота) — замечание исполнителя
подтверждаю, но опасна в нём не та часть, на которую он указал.**
Шаг `make up PROFILES=bot` действительно не перезапускает один контейнер, а сводит весь
набор по умолчанию (`docker compose up -d --wait` + `make health`); сам по себе это
идемпотентный сход — на живом стеке пересоздаётся только контейнер с изменившейся
конфигурацией, то есть бот. Это неточность формулировки, не более.

Реальная цена в другом: `PROFILES=` не добавляется к активному набору, а **замещает** его
(`Makefile:57`: `ACTIVE_PROFILES := $(if $(PROFILES),$(PROFILES),…)`). Оператор,
державший `COMPOSE_PROFILES=memory` в `.env`, после `make up PROFILES=bot` получает стек,
поднятый одним набором файлов, а `make down` (без `PROFILES=`) — другим; §2 того же
runbook прямо предупреждает, что тогда сервисы профиля не остановятся. То есть раздел 6
выдаёт команду, которая ломает правило раздела 2.

Правка: `make up PROFILES=<обычный набор>,bot` (например `memory,bot`) и оговорка, что
`make up` сводит весь стек, а минимальное действие —
`docker compose -f docker-compose.yml -f docker-compose.bot.yml up -d telegram-bot`.
Раздел помечен «не применимо на текущем этапе» (бот в EPIC-004), поэтому Minor, а не Major.
Заодно: во фразе «бинарник `docker compose` без `-f` на этот файл не видит» слово
«бинарник» лишнее.

#### Nit

1. `CLAUDE.md` («Команды сборки/линта/теста»): «`make secrets-scan` — gitleaks по диапазону
   ветки + **рабочей копии**». Второй прогон цели читает **содержимое индекса**, а не
   рабочую копию: `Makefile:157–168` делает `git checkout-index -a --prefix=` во временный
   каталог и сканирует его, и там же комментарий, почему именно так (решение ОВ-8:
   нетронутый `.env` владельца, `build/.legacy-src/` и рабочие каталоги агентов держали
   цель красной). Формулировка пришла из общего DoD §1 п. 4, но чинить её нужно в
   документе, а не наоборот.
2. `CLAUDE.md` и `AGENTS.md` (карты каталогов): `build/` описан как «Dockerfile платформы,
   `minio.Dockerfile`, `versions.env`», без «и др.», хотя в каталоге ещё
   `legacy.Dockerfile`, `minio-init.sh`, `redpanda-init.sh`. В `README.md` это исправлено
   (мелкое 6) — правка одного файла из трёх развела их между собой.
3. `README.md:91`, `CLAUDE.md` (карта `cmd/mvctl`), `AGENTS.md` (карта каталогов) — среди
   реализованных подкоманд перечислены `contracts, env, storage, privacy`; `version`
   реализована и работает (`mvctl --help`: 15 команд, из них зарезервировано 9), но не
   названа нигде.
4. `services/_archive/README.md:16` — «`Makefile` и `docker-compose.yml` архив не упоминают
   вовсе» остаётся верным как написано, но compose-файлов теперь три, и
   `docker-compose.legacy.yml:6,33` ссылается на `services/_archive/**` (в комментариях).
   Точнее: «ни один рецепт `Makefile` и ни один сервис compose архив не собирают».
5. `.dev-team.json.stack` — по-прежнему по-английски при `"language": "ru"`; содержимое
   верно. Как и в приёмке #1, на вердикт не влияет.

### Передать оркестратору (вне моего владения)

1. **`.env.example:20–21`** содержит перекрёстную ссылку «(это те же шесть, что перечисляет
   README)» — но README после итерации 3 шести переменных не перечисляет, там правило.
   Ссылка повисла. Файл принадлежит T-397 и его ревьюеру (code-reviewer#2); правка — снять
   скобку или заменить на «см. правило в README». Замечаний по самой реализации T-397 у
   меня нет.
2. **DoD T-019 «команды из README выполняются на чистой машине владельца (проверка
   человеком)»** остаётся невыполненным: GNU make не установлен. Пункт вынесен владельцу
   записью в `journal.md` от 2026-09-11 и врезкой `runbook:19–26`. Документы непроверенное
   за проверенное не выдают, поэтому на вердикт по документам пункт не влияет, но **закрыть
   T-020 (приёмка волны 0) до его выполнения нельзя** — это и есть недостающая часть
   готовности эпика.

### Что проверено запуском

| Что | Команда | Результат |
|---|---|---|
| Ma-1 закрыт: «чистая машина» интерполируется | синтезирован `clean.env` из `.env.example` (заполнены **только** помеченные `[required]`, значением-заглушкой), затем `COMPOSE_PROFILES= docker compose --env-file clean.env --env-file build/versions.env -f docker-compose.yml config --services` | `core gateway minio minio-init redpanda redpanda-init`, ошибок интерполяции нет — ровно тот состав, что обещает `README:55–56` |
| Тот же прогон с профилем | `COMPOSE_PROFILES=memory … config --services` | добавляются `memory neo4j qdrant` — совпадает с `README:57` и `runbook:47–49` |
| Громкий отказ переехал, а не исчез (`bot`) | `… -f docker-compose.yml -f docker-compose.bot.yml --profile bot config -q` | `required variable MV_TELEGRAM_BOT_TOKEN is missing a value: … the profile bot cannot run without it (D-10)` |
| Громкий отказ переехал, а не исчез (`legacy`) | `… -f docker-compose.yml -f docker-compose.legacy.yml --profile legacy config -q` | `required variable CHROMA_IMAGE … (T-008, D-3)` |
| Утверждение документов про голый вызов compose | `COMPOSE_PROFILES= docker compose … -f docker-compose.yml --profile bot config --services` | тот же состав из шести сервисов, `telegram-bot` отсутствует, ошибки нет — «молча поднимет стек без бота» подтверждено буквально |
| Какой файл когда подключает Makefile | чтение `Makefile:44–68` (`DOTENV_PROFILES` → `ACTIVE_PROFILES` → `COMPOSE_FILES`) + эмуляция его `sed`-разбора `.env` оболочкой | `-f docker-compose.bot.yml` / `-f docker-compose.legacy.yml` добавляются по `filter` активного набора; `PROFILES=` **замещает** `COMPOSE_PROFILES`, а не дополняет → N-7 |
| Снятие стека тем же набором | `Makefile:236–238` (`down: @$(COMPOSE) down`), `reset`, `logs` | все идут через ту же переменную `COMPOSE` → утверждение `runbook:74–79` верно: `make up PROFILES=…` требует того же `PROFILES=` у `make down` |
| Правило про обязательные переменные | `grep -n required .env.example` | шесть пометок `[required]` (строки 48, 50, 52, 71, 73, 140) + объяснение в шапке (строка 20); README и runbook описывают правило, `CLAUDE.md` — список из пяти → N-6 |
| Запреты линтера, дословно | `sed -n '201,210p' .golangci.yml` против `CLAUDE.md:138–148` и `AGENTS.md:45–53` | четыре пары `pattern`/`msg` — совпадают все четыре, включая состав функций и тексты причин |
| Реестр команд `mvctl` | `go run ./cmd/mvctl --help` | 15 команд; реализованы `contracts env storage privacy version`, зарезервированы девять — совпадает с `README:91–93`, `CLAUDE.md`, `AGENTS.md`, `runbook:13–14` |
| Mi-1 по факту | `mvctl report`, `mvctl llm usage`, `mvctl session-report` | `not implemented … (EPIC-005)` код 2; то же; `unknown command "session-report"` код 2 |
| Каждая цель `make` из четырёх документов | сверка с `.PHONY`-списком `Makefile` | упомянуты `build lint test test-integration test-e2e contracts secrets-scan compose-lint ci ci-full image minio-image up legacy-src down reset logs health llm-up llm-down llm-health backup restore archive-legacy help` — все существуют; `build-service`, `build-all`, `run SERVICE=`, `logs-service` встречаются только в отрицательных формулировках (`CLAUDE.md:233`, `AGENTS.md:34–37`) |
| Содержание описаний целей | чтение рецептов `Makefile:121–208`, `216–299` | `build` = `go build -ldflags … -o bin/ ./cmd/...` (в `cmd/` ровно `multiverse` и `mvctl`); `test` = `-short -race -count=1` + `coverage-gate.sh 60`; `contracts` = `contracts check` + `env check` + `TestSchemasValid`; `ci` = восемь целей; `up` = `up -d --wait` + `health LLM_STRICT=0`; `backup` останавливает `core`/`redpanda` и исключает `prompts-*`; `health` печатает `gateway/memory/telegram-bot`, `core` — изнутри контейнера, плюс LLM и возраст бэкапа — описания в документах сходятся |
| Карточки процессов runbook §8 | `grep 'contexts=\|ports:' docker-compose.yml` | `gateway` = `--contexts=gateway`, `127.0.0.1:8088`; `core` = `--contexts=state,mechanics,laws,llm,swarm`, портов не публикует; `memory` = `--contexts=memory`, `127.0.0.1:8082` — совпадает построчно |
| Утверждения архива (Mi-2) | `sed -n '216,219p;272,276p' .golangci.yml`; `grep -n services/ .dockerignore`; `sed -n '17p;40p' .github/CODEOWNERS`; `grep -n _archive Makefile docker-compose*.yml .gitleaks* .pre-commit-config.yaml` | все шесть ссылок точны по номерам строк; `Makefile`/`docker-compose.yml` — пусто; `gitleaks` архив не исключает; `pre-commit` — пять `exclude` |
| Состав каталогов, названных в документах | `ls build/`, `ls ops/metrics/`, `ls cmd/`, `head -3 ops/metrics/baseline.md` | `build/` = шесть файлов (README точен, `CLAUDE.md`/`AGENTS.md` неполны → Nit 2); `ops/metrics/` = три файла, `baseline.md` = ШАБЛОН (runbook точен); `cmd/` = `multiverse`, `mvctl` |
| Инструменты, нужные `make ci` | `grep -n 'command -v\|python3' scripts/compose-lint.sh`; `grep -n 'pre-commit\|golangci\|gitleaks\|govulncheck' .github/workflows/*.yml` | `compose-lint` жёстко требует `python3` (иначе выход с сообщением) и `docker compose`; в CI ставятся три инструмента из четырёх, `pre-commit` — ни разу → N-4 |
| Все относительные ссылки шести файлов | обход `](…)` с проверкой существования цели | ни одной битой; регистр `Docs/` сохранён |
| Следы прежней картины | `grep` по `go.work`, `Timescale`, `Redis`, `Chroma`, «15 сервисов» | все вхождения отрицательные или профильные (`Chroma` — только «профиль `legacy`» и «раньше валил `make up`»); положительных утверждений старой картины не осталось |
| `dev-log.md` (общий DoD §1 п. 5) | `grep -c 'tech-writer ·'`, список заголовков `^## ` | 0 записей; последний раздел — `developer#1 · T-018` → N-2 |
| Линтер по общему DoD | `golangci-lint run` | `0 issues`, код 0 (документы Go-код не трогают; прогон контрольный) |

### Риски и допущения

- **GNU make не установлен** — ни одна цель не исполнялась. Всё, что сказано выше про
  `make up`/`make down`/`make ci`, проверено на уровне рецепта (чтение `Makefile`) и на
  уровне их составных частей (`docker compose … config`, `mvctl`, `golangci-lint`), но не
  на уровне `make`. Вычисление `ACTIVE_PROFILES` и подстановка `-f` проверены эмуляцией
  того же `sed`/`filter` оболочкой. Это известный пробел владельца, а не дефект задачи;
  документы его не скрывают (о месте предупреждения — N-5).
- **Контейнеры не поднимались, GPU не трогался.** Вывод «`make up` на чистой машине больше
  не падает до старта контейнеров» получен из `docker compose config`, то есть до создания
  чего-либо; он опирается на то, что compose интерполирует модель целиком до фильтрации по
  профилям — это и есть механизм, который отказал в Ma-1 и который теперь обойдён.
- **«Чистая машина» синтезирована**, а не взята с чистой машины: заполнены ровно шесть
  переменных с пометкой `[required]`, всё остальное — как в `.env.example`. Это
  воспроизводит тот же сценарий, который проверяет правило 7 линтера, но независимо от него
  (собственный скрипт, а не `scripts/compose-lint.sh`).
- **Файлы T-397 не оценивались по существу** — только как источник истины. Единственное
  замечание, их касающееся (повисшая ссылка в `.env.example`), вынесено оркестратору, а не
  записано как дефект задачи.
- **`.env` владельца не читался** (кроме проверки, что строки `COMPOSE_PROFILES` в нём нет);
  секреты не открывались.

---

## T-397 · ревью #1 · 2026-09-11 · code-reviewer#2

### Границы ревью

`docker-compose.yml`, новые `docker-compose.bot.yml` и `docker-compose.legacy.yml`, `Makefile`,
`.env.example`, `scripts/compose-lint.sh`, `testdata/compose-lint/**`, `.github/ci.env`;
`.github/workflows/go.yml` — только чтением. Документы (`README.md`, `CLAUDE.md`, `AGENTS.md`,
`Docs/ops/runbook.md`, `services/_archive/README.md`) не рассматривались: их в это же время правит
tech-writer по T-019.

Метод: состояние «до» доставалось из индекса (`git show :docker-compose.yml`, `git show
:.env.example`) и прогонялось теми же командами, что и состояние «после», — вердикт про закрытый
дефект опирается на разницу двух прогонов, а не на текст задачи. Решения оркестратора по обоим
открытым вопросам T-397 (голый `docker compose --profile bot` не поддерживается; правило 7
разрешает обязательную переменную профильного сервиса, если она помечена заполняемой) приняты как
данность и не переоткрываются.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 3, Nit 4.

Оба заявленных дефекта закрыты, и оба — не на слово: старые файлы из индекса на чистом окружении
падают, новые проходят; набор по умолчанию — ровно девять сервисов; полностью разрешённая модель
до и после переноса совпадает дословно, кроме одного нового служебного ключа `x-legacy-logging`.
Второй дефект воспроизведён в обе стороны: старый `.env.example` подставлял в `MINIO_ROOT_USER`
текст собственного комментария, а `set -a; . ./.env` читал ту же строку как пустую.

Три Minor не блокируют: один — реальная, но узкая расходимость разбора `.env` между `Makefile` и
compose (лечится однострочной правкой), два других — про артефакты и архитектурный документ, и
правит их не исполнитель.

**Отдельно и громко: правки `Makefile` не исполнялись ничем.** GNU make на машине нет
(`which make` пуст; в WSL только дистрибутив `docker-desktop` — busybox без make), CI цели `make`
не вызывает. Всё, что написано ниже про `Makefile`, — вычитка глазами плюс эмуляция оболочкой;
это записано в «Что осталось непроверенным» и остаётся долгом владельца.

### Замечания по серьёзности

#### Critical

Нет.

#### Major

Нет.

#### Minor

**Mi-1. `Makefile:57` — самодельный разбор `.env` расходится с разбором compose в трёх
воспроизводимых случаях; во всех трёх `make` подключает МЕНЬШЕ файлов, чем ожидает compose, и
профиль тихо исчезает.** `DOTENV_PROFILES` берёт `head -n 1` и требует, чтобы ключ стоял в самом
начале строки. Разбор dotenv у compose: последнее вхождение ключа побеждает, префикс `export`
допустим, ведущие пробелы допустимы. Проверено на одном и том же файле (слева — конвейер `sed` из
`Makefile`, справа — `docker compose --env-file … config`):

```
.env содержит COMPOSE_PROFILES дважды (memory, затем memory,bot)
  make → [memory]            compose → [memory,bot]
.env содержит `export COMPOSE_PROFILES=memory,bot`
  make → []                  compose → [memory,bot]
.env содержит `  COMPOSE_PROFILES=memory,bot` (два пробела в начале)
  make → []                  compose → [memory,bot]
```

Последствие ровно то, которое задача старалась убрать: compose считает профиль `bot` активным,
`docker-compose.bot.yml` не подключён, `make up` молча поднимает стек без бота и без единого
сообщения. Вероятность средняя — дубль ключа появляется при ручной правке `.env`, `export`
напрашивается из-за соседнего `set -a; . ./.env` в этом же `Makefile`.

Как исправить (одна строка): `head -n 1` → `tail -n 1`, и шаблон
`s/^COMPOSE_PROFILES=//p` → `s/^[[:space:]]*\(export[[:space:]][[:space:]]*\)\?COMPOSE_PROFILES=//p`.
Значения с кавычками, инлайн-комментарием и CRLF конвейер уже разбирает так же, как compose, —
проверено (см. таблицу), их трогать не нужно.

**Mi-2. `Docs/dev-team/architecture/infrastructure.md` описывает топологию, которой больше нет, и
— хуже — §4.2 хранит эталон `.env.example` ровно в запрещённом формате.** §1.3 говорит о `bot` и
`legacy` внутри общего файла, §3.1.1 — о шести правилах линтера, §2.2 — о прежнем `COMPOSE`. Но
самое опасное — блок §4.2 (строки 402–512), где целевой `.env.example` приведён в виде
`MINIO_ROOT_USER=                     # обязательна; не minioadmin`,
`MV_TELEGRAM_BOT_TOKEN=               # секрет; выдаёт @BotFather`: это дословно тот формат,
который T-397 объявил дефектом. Документ — источник истины для следующего исполнителя, и рецидив
из него будет выглядеть как «сделал по дизайну». Смягчение есть: правило 7 такой файл отвергнет
(проверено фикстурой). Владелец файла — tech-lead#1/system-architect, исполнителю T-397 править
не нужно; требуется задача на синхронизацию §1.3, §2.2, §3.1.1, §4.2.

**Mi-3. У T-397 нет записи в `dev-log.md` эпика и карточки в `tasks.md`.** `grep -c T-397` даёт 0 в
`Docs/dev-team/epics/EPIC-001-foundation/dev-log.md` и 0 в `tasks.md`; задача существует только в
`journal.md` (6 записей) и `Docs/dev-team/state.js` (статус `review`, исполнитель
`devops-engineer`). Из-за этого у ревью нет ни формулировки критериев приёмки, ни таблицы
самопроверки автора — их пришлось собирать из журнала и постановки. Записи журнала подробные и
честные (включая признание пробела с `make`), так что это не сокрытие, а незакрытый артефакт:
нужна запись в `dev-log.md` по шаблону и карточка задачи.

#### Nit

1. `.env.example:22–24` — «пометка обязательности… это те же шесть, что перечисляет README».
   README переменные не перечисляет, а отсылает обратно к пометке (`README.md:44`:
   «заполнить переменные, отмеченные `[required]`»). Ссылка кольцевая; либо убрать «что перечисляет
   README», либо перечислить шесть имён здесь. Шесть — верно: `MINIO_ROOT_USER`,
   `MINIO_ROOT_PASSWORD`, `NEO4J_PASSWORD`, `MV_MINIO_ACCESS_KEY`, `MV_MINIO_SECRET_KEY`,
   `MV_NEO4J_PASSWORD`, и ровно они закрывают все `${VAR:?}` базового файла, не приходящие из
   `build/versions.env` (проверено перечислением).
2. `scripts/compose-lint.sh:79` — `-h` печатает `sed -n '2,60p'`, то есть на две строки больше
   шапки: в конце справки появляется `set -euo pipefail`. Было `2,50p`, стало `2,60p`; нужно
   `2,58p`.
3. Правило 7 считает пометкой `[required]` только НЕПОСРЕДСТВЕННО предшествующую строку
   комментария (переменная `comment` перезаписывается на каждой строке). Сейчас это безопасно (у
   всех шести пометка стоит вплотную), и промах приводит к громкому падению, а не к тихому
   пропуску, но у `MV_TELEGRAM_BOT_TOKEN` уже сегодня блок из трёх строк комментария — если такая
   переменная когда-нибудь станет `[required]`, метка потеряется. Одна строка в шапке
   `.env.example`: «пометка — в строке, непосредственно предшествующей переменной».
4. `docker-compose.bot.yml`, `docker-compose.legacy.yml` и обе новые фикстуры лежат в рабочем
   дереве с CRLF при `.gitattributes: *.yml text eol=lf` и хуке `mixed-line-ending --fix=lf`
   (`.pre-commit-config.yaml:30–32`). Первый `git commit` эти файлы перепишет хуком — тот самый
   шум «файлы изменены хуком», который уже отмечен в журнале 2026-09-11. Это общее состояние
   дерева (у `docker-compose.yml` то же), но новые файлы стоит сохранить в LF заранее.

### Что проверено экспериментом

| Что | Команда | Результат |
|---|---|---|
| Исходный дефект (состояние «до») | `git show :docker-compose.yml > /tmp/before.yml`; `docker compose -f /tmp/before.yml --env-file <копия старого .env.example с заполненными «обязательна»> --env-file build/versions.env config -q` | `error while interpolating services.chromadb.image: required variable CHROMA_IMAGE…`, код 1 — дефект T-019 Ma-1 воспроизведён |
| Исходный дефект закрыт (состояние «после») | `docker compose --env-file /tmp/clean.env --env-file build/versions.env config -q` | код 0 |
| «Чистая машина» собрана независимо от линтера | `awk` по `.env.example`: заполнены только переменные, у которых в строке ВЫШЕ стоит `[required]` | заполнено ровно 6: `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`, `NEO4J_PASSWORD`, `MV_MINIO_ACCESS_KEY`, `MV_MINIO_SECRET_KEY`, `MV_NEO4J_PASSWORD` |
| Набор по умолчанию | `docker compose --env-file /tmp/clean.env --env-file build/versions.env config --services` | ровно 9: `core gateway memory minio minio-init neo4j qdrant redpanda redpanda-init` |
| Запуск доходит до создания контейнеров | `docker compose --env-file /tmp/clean.env --env-file build/versions.env -p mv-review-dryrun up -d --dry-run` | интерполяция пройдена, dry-run дошёл до `Started`/`Healthy` всей инфраструктуры; `docker ps -a` / `volume ls` / `network ls` по метке проекта — пусто, ничего не создано |
| Второй дефект: что было | `docker compose -f docker-compose.yml --env-file <старый .env.example дословно> … config` | `MINIO_ROOT_USER: '# обязательна; не minioadmin'`, `MINIO_ROOT_PASSWORD: '# обязательна, >= 16 символов; секрет'`, `NEO4J_AUTH: neo4j/# обязательна при профиле memory…` — MinIO стартовал бы с логином-комментарием, `${VAR:?}` молчал |
| Второй дефект: расхождение с оболочкой | `set -a; . <тот же файл>; echo "[$MINIO_ROOT_USER]"` | `[]` — compose и `make` читают одну строку по-разному, как и записано в журнале |
| Второй дефект: что стало | `grep -nE '^[A-Za-z_][A-Za-z0-9_]*=[[:space:]]*#' .env.example` | ни одного совпадения; в версии из индекса — 13, и это ровно те 13, что перечислил автор |
| Не осталось ли ещё одной такой переменной | тот же `grep` по `build/versions.env` и `.github/ci.env`; полный список пустых переменных `.env.example` (15 шт.) просмотрен глазами | чисто во всех трёх файлах |
| Громкий отказ профиля `bot` | `docker compose -f docker-compose.yml -f docker-compose.bot.yml --profile bot --env-file /tmp/clean.env … config -q` | `required variable MV_TELEGRAM_BOT_TOKEN is missing a value: … the profile 'bot' cannot run without it (D-10)`, код 1 |
| Громкий отказ профиля `legacy` | то же с `docker-compose.legacy.yml --profile legacy` | `required variable CHROMA_IMAGE … the profile 'legacy' cannot run without it`, код 1 |
| Ничего не потеряно при переносе | `docker compose --project-directory . -f <старый файл> --env-file build/versions.env --env-file .github/ci.env --profile gpu --profile memory --profile dev --profile legacy --profile bot config` против того же по трём новым файлам, `diff` | различие ровно одно: в новой модели появился ключ верхнего уровня `x-legacy-logging` (`50m × 5`). Сервисы, порты, тома, `depends_on`, `logging`, окружение, `x-bot-logging` (`10m × 3`) — совпадают дословно |
| Тома | `config --volumes` до и после | совпадают; `chroma_data` объявлен в файле профиля так же, как был объявлен в общем файле |
| Линтер на действующих файлах | `scripts/compose-lint.sh` | `ok — 15 services in 3 file(s), 7 rules (profiles resolved: bot, dev, gpu, legacy, memory)`, код 0 |
| Каждая фикстура отвергнута СВОИМ правилом | `scripts/compose-lint.sh -f testdata/compose-lint/bad-*.yml` по одной, с чтением текста | `bad-anchor` → правило 3 (2 нарушения), `bad-env-list` → правило 3, `bad-latest` → правило 1, `bad-port` → правило 2, `bad-required-outside-default` → **правило 7** (`MV_TELEGRAM_BOT_TOKEN`), `bad-env-example-comment` → **правило 7**, половина «формат `.env`» (`…:5: MINIO_ROOT_USER is empty and carries an inline comment`). Правило 7 выполняется первым и старые фикстуры не перехватывает |
| Правило 7 переживает добавление сервиса в профиль | копия `docker-compose.yml` + сервис `profiles: ["dev"]` с `${MV_NEW_SERVICE_KEY:?}` → `scripts/compose-lint.sh -f …` | правило 7 срабатывает и называет переменную. Тот же сервис с `${NEO4J_PASSWORD:?}` (переменная помечена `[required]`) — `ok`, то есть решение оркестратора по ОВ 2 реализовано именно так, как принято |
| Шаг CI «docker compose config» дословно | `docker compose --env-file build/versions.env --env-file .github/ci.env config -q` | код 0 |
| Шаг CI «фикстуры остаются отвергнутыми» дословно | цикл `for bad in testdata/compose-lint/bad-*.yml` из `.github/workflows/go.yml:356–362` | все шесть отвергнуты, цикл завершается 0 |
| Job `contracts` не сломан переформатированием примера | `go run ./cmd/mvctl env check`; `go test ./shared/env/... ./cmd/mvctl/...` | `env check: 69 variables declared, compared with .env.example`, код 0; все пакеты `ok` |
| `Makefile`: активный набор профилей (эмуляция оболочкой) | скрипт, повторяющий `$(if …)`, `$(subst …)`, `$(filter …)`, `$(wildcard …)` и конвейер `sed` | пусто → только базовый файл; `COMPOSE_PROFILES=bot` из окружения → `+bot.yml`; `.env` с инлайн-комментарием `memory   # набор профилей: memory,gpu,bot,dev,legacy` → **`memory`, файлы профилей НЕ подключаются** (без обрезки комментария `$(filter bot,…)` поймал бы слово `bot` из самого комментария); `PROFILES=bot` → `+bot.yml --profile bot`; `PROFILES=memory,legacy` → `+legacy.yml`, предпосылка `legacy-src` есть; `PROFILES=bogus` → только базовый файл, `--profile bogus`, compose молчит; `.env` = `memory,legacy` без `PROFILES` → `+legacy.yml` и `legacy-src` |
| Экранирование `\#` в `$(shell sed …)` | обе возможные трактовки GNU make (обратная косая сохраняется внутри вызова функции / снимается) прогнаны как два отдельных конвейера `sed` | результат идентичен (`memory`), предупреждений GNU sed нет — экранирование безопасно при любой трактовке; `$$` → `$` в конвейере даёт якоря конца строки, как и задумано |
| Разбор `.env` в `Makefile` против разбора compose | 7 форм строки `COMPOSE_PROFILES=…` через `docker compose --env-file … config` и через конвейер `sed` | совпадение на: инлайн-комментарий, комментарий без пробела, двойные кавычки, одинарные кавычки, хвостовые пробелы, CRLF, пустое значение. Расхождение на трёх формах — Mi-1 |
| Локальный файл переопределения | `$(wildcard docker-compose.override.yml)`, `ls docker-compose.override.y*ml` | в дереве его нет; логика верна и добавляет файл последним. Оговорка — в «Рисках» |
| Снимает ли `down` контейнеры профиля | изолированный проект `mvrevorphan` на `alpine:3.22`, контейнеры только СОЗДАНЫ (`create`, не `up`): форма «после» (`-f base -f extra`, затем `down` только с `base`) и форма «до» (один файл с обоими сервисами, `down` без `--profile`) | **обе формы ведут себя одинаково: контейнер профиля остаётся**. То есть это не регресс T-397, а поведение compose по профилям; `down --remove-orphans` в форме «после» снимает его, симметричный `down` с файлом и `--profile` — тоже. То же и для томов: `down -v` не удаляет том профиля ни до, ни после. Всё созданное удалено, чужой проект `deployments` не тронут |
| `git grep` в правиле 3 видит новые файлы | `git ls-files --others --exclude-standard -- 'docker-compose*.yml'` | шаблон матчит `docker-compose.bot.yml` и `docker-compose.legacy.yml`; пока файлы не в индексе, `git grep` их не читает — это уйдёт с коммитом |

### Что осталось непроверенным и почему

- **Ни одна цель `make` не выполнена — на машине нет GNU make.** `which make` пуст; в WSL
  единственный дистрибутив `docker-desktop` (busybox, `command -v make` → пусто); ставить make я
  не вправе. Всё про `Makefile` в этом ревью — вычитка и эмуляция оболочкой, включая ключевые
  места: `\#` и `$$` в `$(shell sed …)`, `$(filter …)` по списку профилей, предпосылка
  `legacy-src`, порядок `-f`. Не проверено исполнением: раскрытие `$(if …)`/`$(wildcard …)` самим
  make, порядок вычисления `:=` при `include build/versions.env`, поведение `$(shell)` при
  отсутствующем `.env`. Ожидаемый долг владельца (совпадает с записью в журнале): `make -n up`,
  `make -n up PROFILES=bot`, `make -n up PROFILES=memory,legacy`, `make compose-lint`, затем
  реальный `make up` на чистой машине.
- **Контейнеры проекта не поднимались** (запрет постановки): всё получено через `config`,
  `config --services` и `up --dry-run`. Единственное исключение — изолированный throwaway-проект
  на `alpine:3.22` для вопроса про `down`, где контейнеры были только СОЗДАНЫ (`create`), ни один
  не запускался; всё удалено, метки проверены.
- **`make up` на по-настоящему чистой машине** (пустой Docker, нет томов, нет образа
  `multiverse-core:dev`) не выполнялся: `up --dry-run` доходит до создания контейнеров, но не
  доказывает, что сервисы стартуют и становятся healthy. Пункт «5 команд» по-прежнему за стендом.
- **CI на GitHub не запускался**: шаги job `compose-lint` выполнены дословно локально
  (`docker compose config -q`, `scripts/compose-lint.sh`, цикл по фикстурам); `hadolint` и
  остальные job не трогались — их T-397 не менял, `.github/workflows/go.yml` в этой задаче не
  правился вовсе.
- **Профиль `legacy` целиком** (`make legacy-src`, сборка образов из `LEGACY_SRC_REF`,
  жизнеспособность профиля — ОВ-41) не проверялся: он и до задачи не был проверен, а `CHROMA_IMAGE`
  намеренно пуст (D-3). Проверено только то, что перенос ничего не изменил в описании сервисов.
- **Windows-специфика**: прогон на Windows 11 + Git Bash, `docker compose` v5.2.0, демон 29.6.1.
  Поведение `sed`/`awk` из состава Git for Windows считалось эквивалентным GNU-варианту в Linux-CI;
  для использованных выражений это верно, но на CI не перепроверялось.

### Открытые вопросы (оркестратору)

1. Кому и когда заводить задачу на синхронизацию `infrastructure.md` §1.3, §2.2, §3.1.1 и особенно
   §4.2 (Mi-2)? Пока §4.2 показывает запрещённый формат, следующий исполнитель воспроизведёт
   второй дефект «по дизайну». Файл — не devops.
2. Нужна ли запись T-397 в `dev-log.md` эпика и карточка в `tasks.md` задним числом (Mi-3), или
   для задач, рождённых из ревью, достаточно журнала? Сейчас правило разное для T-0NN и T-397.

### Предложения в бэклог (в этой задаче не требую)

- `make down` и `make reset` покрывают только файлы активного набора профилей; контейнер и том
  профиля, поднятого другим набором, остаются (проверено — поведение то же и до T-397). Дёшево
  закрывается `--remove-orphans` в цели `down`; для `chroma_data` §5.5 хочет ровно текущего
  поведения, но текст приглашения `reset` («Delete every volume of multiverse?») этого не отражает.
- Линтер не проверяет, что каждый существующий `docker-compose.*.yml` подключается из `Makefile`.
  Забытая строка `COMPOSE_FILES +=` даст тихий стек без сервисов профиля — тот же класс тишины,
  что и голый `docker compose --profile bot`.
- `Docs/dev-team/plan/ownership.md` и `.github/CODEOWNERS` перечисляют `docker-compose.yml`
  поимённо; после T-397 стоит написать `docker-compose*.yml`, иначе правило потеряется на новых
  файлах (в CODEOWNERS сейчас спасает `*`).
- Шапка `.env.example` объявляет запрет инлайн-комментария после пустого значения и для
  `build/versions.env`, и для `.github/ci.env`, но правило 7 проверяет только `.env.example`
  (оба файла сейчас чисты — проверено). Проверку легко распространить тем же кодом.

### Риски и допущения

- **Главный риск задачи не в коде, а в непроверенности `Makefile`.** Логика вычисления активного
  набора профилей выглядит корректной и на семи сценариях эмуляции даёт ожидаемое, но между
  эмуляцией и GNU make остаётся зазор, который закрывает только установка make. Вердикт «принять»
  относится к compose-файлам, `.env.example`, линтеру и фикстурам, проверенным исполнением;
  по `Makefile` он опирается на вычитку.
- Вариант (в) переносит цену с «падает у всех» на «молчит у того, кто зовёт `docker compose`
  мимо `make`». Это принятое решение оркестратора, оно задокументировано в шапках обоих новых
  файлов и в `.env.example`. Остаточный риск: `make` объявлен единственной точкой входа оператора,
  а `make` на машине владельца пока нет — то есть поддерживаемого пути запуска профилей `bot` и
  `legacy` сегодня физически не существует. Это усиливает срочность установки make, а не меняет
  вердикт.
- Совпадение разрешённых моделей «до/после» получено с `--env-file .github/ci.env`, то есть с
  фиктивными значениями. Для сравнения состава это корректно (интерполяция от профилей не зависит),
  но оно не доказывает, что значения на стенде совпадут с прежними.
- Ответ про `down` получен на изолированном проекте из двух контейнеров `alpine`, а не на реальном
  стеке: перенос вывода на `telegram-bot` опирается на то, что механизм (метка проекта +
  `--remove-orphans`) от содержимого сервиса не зависит.
- Дерево после ревью не изменено, кроме этого файла: правки не вносились, временные файлы жили в
  каталоге вне репозитория и удалены, throwaway-объекты Docker удалены и проверены по меткам.

## T-019 · приёмка #3 (итерация 4) · 2026-09-10 · tech-lead#1

### Границы и метод

Объём был заморожен приёмкой #2: N-1…N-7, четыре мелких (Nit 1–4) и запись в
`dev-log.md`. Сверялись только они плюс сплошной просмотр изменённых кусков итерации 4
(`git diff` рабочей копии против индекса — индекс держит состояние итерации 3). Закрытое
ранее не перепроверялось. Nit-5 (`.dev-team.json.stack` по-английски) на вердикт не влияет,
как и в двух прежних приёмках.

Метод прежний: каждое утверждение проверялось своим прогоном или чтением файла-истины;
отчёт исполнителя доказательством не считался. GNU make не установлен — цели читались
и проверялись их составными частями (`docker compose config`, `scripts/compose-lint.sh`,
`go run ./cmd/mvctl`); это известный пробел владельца, не дефект задачи. Контейнеры не
поднимались.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 2, Nit 4.

Все семь пунктов заморозки и все четыре мелких закрыты по существу и подтверждены
независимо. Остаток — две Minor, обе появились в правках итерации 4 и обе однострочные:
устаревшие ссылки на номера строк `Makefile` (следствие того, что оркестратор поправил
разбор профилей ПОСЛЕ моей приёмки #2, а исполнитель взял номера из текста ревью, не
пересверив) и переширокая оговорка в `services/_archive/README.md`. Ни одна из них не
велит читателю сделать неверное действие — в отличие от N-3/N-7, за которые возвращалось.
Возвращать задачу пятый раз за три строки, две из которых испортил я сам, неправильно:
исправления вынесены остатком (см. ниже) и являются предусловием T-020.

**Остаток задачи, не закрываемый этой приёмкой**: DoD «команды из README выполняются на
чистой машине владельца (проверка человеком)» — не выполнен, потому что `make` не
установлен ни на одной машине разработки. Решение приёмки #2 подтверждаю: это условие
T-020, а не T-019; документы факт не скрывают (врезки в README и runbook).

### Статус семи пунктов заморозки

| # | Статус | Чем доказано |
|---|---|---|
| N-1 (Major) | **закрыт** | `CLAUDE.md:64–66` называет оба новых файла, `:245–247` — «стек по умолчанию и профили `memory`/`gpu`/`dev`, `bot`/`legacy` — в своих файлах, Makefile подключает по `PROFILES=`». Сверено прогоном: `docker compose --profile legacy --profile bot config --services` по одному `docker-compose.yml` даёт 7 сервисов (`core, gateway, minio, minio-init, neo4j, redpanda, redpanda-init`), с тремя файлами — 11 (+`telegram-bot`, `chromadb`, `narrative-orchestrator`, `semantic-memory`). Совпадает с шапками всех трёх файлов и с `Makefile:68–70` |
| N-2 (Major, процесс) | **закрыт** | Запись `## tech-writer · T-019 · F-9 … · 2026-09-10` добавлена; диффом подтверждено, что это единственный кусок (`@@ -5798,3 +5798,101 @@`) — чужие разделы не тронуты. Охватывает все четыре итерации: что переписано, что убрано как устаревшее, три расхождения кода и документов, проверки, что осталось за владельцем |
| N-3 (Minor) | **закрыт** | `README.md:85–89` и `runbook:67–73` больше не требуют ручного `make legacy-src`; сказано, что `up` выполняет её сам и почему первый запуск дольше. Сходится с `Makefile:224` (`up: $(if $(filter legacy,$(ACTIVE_PROFILE_LIST)),legacy-src)`), с рецептом `legacy-src` (`Makefile:229–240`) и с шапкой `docker-compose.legacy.yml:19–20`. Дефект — номер строки, см. Nov-1 |
| N-4 (Minor) | **закрыт** | `README.md:25–34`: состав цели процитирован дословно и совпадает с `Makefile:198`; `pre-commit` объявлен не участвующим в `make ci` (по рецептам — ноль вызовов); «в CI ставятся только три» подтверждено `.github/workflows/go.yml` (`golangci-lint-action` с `version: ${{ env.GOLANGCI_LINT_VERSION }}`, `gitleaks-action`, `govulncheck-action`; `pre-commit` не встречается); `python3` назван, текст ошибки совпадает с `scripts/compose-lint.sh:100` дословно; пины `v2.13.2`/`v8.30.1` сверены с `build/versions.env:107–108`. Уточнение — Nit-2 ниже |
| N-5 (Minor) | **закрыт** | Врезка `README.md:47–52` стоит ДО блока команд (`:54`), читатель видит её раньше `cp .env.example .env`. Заголовок ссылки («Порядок ниже не прогнан целиком») дословно совпадает с врезкой `runbook:19–26`. Утверждение «CI цели `make` не вызывает» проверено: в `.github/workflows/go.yml` слово `make` встречается только в комментариях |
| N-6 (Minor) | **закрыт** | `CLAUDE.md:231–233` — правило («всё, что помечено `[required]` в комментарии над переменной»), формулировка совпадает с `README.md:55–56` и `runbook:30–32`. «Сейчас шесть переменных» верно: пометок `[required]` над переменными в `.env.example` шесть (строки 50, 52, 54, 73, 75, 142; седьмое вхождение — объяснение самой пометки на строке 20). Нумерация уехала после правки `.env.example` оркестратором — в текстах документов номеров строк нет, и это правильно |
| N-7 (Minor) | **закрыт** | `runbook:173–189` переписан: сказано, что `PROFILES=` замещает набор, дано два верных пути — `make up PROFILES=<обычный набор>,bot` и точечный `docker compose -f docker-compose.yml -f docker-compose.bot.yml up -d telegram-bot`. Сходится с `Makefile:65` (`ACTIVE_PROFILES`), `:44` (`PROFILE_ARGS` строится только из `PROFILES`) и с правилом §2 (`runbook:77–82`). Слово «бинарник» убрано. Дефект — номер строки, см. Nov-1 |

### Четыре мелких

| # | Статус | Чем доказано |
|---|---|---|
| Nit-1 приёмки #2 (`make secrets-scan`) | **закрыт** | `CLAUDE.md:122` — «по диапазону ветки + содержимому индекса (не рабочей копии)»; сходится с `Makefile:163–174` (`git checkout-index -a --prefix=`, решение ОВ-8) |
| Nit-2 приёмки #2 (состав `build/`) | **закрыт** | `CLAUDE.md:62–63` и `AGENTS.md:122–123` перечисляют `Dockerfile`, `legacy.Dockerfile`, `minio.Dockerfile`, `minio-init.sh`, `redpanda-init.sh`, `versions.env` — это ровно `ls build/`; все три файла-инструкции теперь совпадают между собой |
| Nit-3 приёмки #2 (`mvctl version`) | **закрыт** | Добавлена в README, `CLAUDE.md`, `AGENTS.md`. `go run ./cmd/mvctl --help`: реализованы `contracts, env, privacy, storage, version`, зарезервированы девять — списки в документах совпадают дословно |
| Nit-4 приёмки #2 (архив и три compose-файла) | **закрыт по сути, с переширокой оговоркой** | `services/_archive/README.md:16–21` — «ни один рецепт `Makefile` и ни один сервис ни одного из трёх compose-файлов архив не собирает»; ссылки `docker-compose.legacy.yml:6,33` названы комментариями, оба номера проверены и верны. Оговорка «и не собирают `services/*`» — неверна, см. Nov-2 |

### Новые замечания

#### Minor

**Nov-1. Три ссылки на номера строк `Makefile` в живых документах указывают не туда.**
`README.md:86` и `runbook:68` ссылаются на `Makefile:217` как на строку цели `up` — там
`--build-arg MINIO_BUILDER_IMAGE=…`; цель `up` теперь на строке 224. `runbook:173`
ссылается на `Makefile:57` как на определение `ACTIVE_PROFILES` — там строка `#`;
определение на строке 65. Те же устаревшие номера в записи `dev-log.md` (`Makefile:57`,
`:61-63`, `:191`, `:217`, `:222`); для журнала это менее важно — он фиксирует состояние
на момент работы, но и там лучше поправить одной правкой.

Причина названа честно: номера пришли из моего же текста приёмки #2 и были верны тогда;
между приёмкой и итерацией 4 оркестратор переписал разбор набора профилей
(`Makefile:58–64`, Mi-1 ревью T-397), и файл сдвинулся на семь строк. Проверить было
можно — в момент сдачи итерации 4 в индексе уже лежал новый `Makefile` (`git show :Makefile`:
`ACTIVE_PROFILES` = 65, `ci` = 198, `up` = 224).

Правка (одна строка в каждом месте): ссылаться на **имя цели/переменной**, а не на номер
строки — «цель `up` объявляет `legacy-src` предпосылкой», «переменная `ACTIVE_PROFILES` в
`Makefile`». Тогда ссылка не сгниёт при следующей правке рецептов; этот класс дефекта за
три приёмки повторяется уже второй раз.

**Nov-2. `services/_archive/README.md:18` — «все они вне единого модуля и не собирают
`services/*`» неверно для профиля `legacy`.**
Заголовочное утверждение («архив не собирает ни один рецепт и ни один сервис compose»)
верно и проверено. Но обосновывающая оговорка ложна: `make legacy-src` (`Makefile:238`)
экспортирует `services/narrative-orchestrator` и `services/semantic-memory` из
`LEGACY_SRC_REF`, а `docker-compose.legacy.yml:71–75,115–119` именно их и собирает
(`context: build/.legacy-src`, `dockerfile: ../legacy.Dockerfile`). То есть `services/*`
собирается — просто не из рабочего дерева и не из архива. В прежней редакции оговорка
относилась к `docker-compose.yml`, где она верна; итерация 4 распространила её на файл,
где она неверна.

Правка: «…архив не собирает. Профиль `legacy` собирает `services/narrative-orchestrator`
и `services/semantic-memory`, но не из рабочего дерева, а из экспорта `LEGACY_SRC_REF` в
`build/.legacy-src` (`make legacy-src`, `build/legacy.Dockerfile`); `services/_archive/**`
не собирает никто, поэтому явное исключение не требовалось».

#### Nit

1. `dev-log.md` (раздел «Проверки»): «`go run ./cmd/mvctl --help` (15 команд)» — команд
   четырнадцать (5 реализованных + 9 зарезервированных). Число пришло из моей приёмки #2,
   там же и ошибка; списки команд в самих документах при этом верны.
2. `README.md:29–30`: «без `python3` `compose-lint` завершается ошибкой» — строго говоря,
   `scripts/compose-lint.sh:98` берёт `command -v python3 || command -v python`, то есть
   падает, только если нет ни того ни другого. Уточнение на два слова; на действие
   оператора не влияет.
3. `CLAUDE.md:65–66,246–247`: «Makefile подключает их по `PROFILES=…`» — не сказано, что
   второй путь (`COMPOSE_PROFILES` в `.env`) работает так же; README и runbook называют оба.
   После правки оркестратора именно путь через `.env` стал надёжным (дубль ключа, `export`,
   ведущие пробелы), а `.env.example:38` ставит `COMPOSE_PROFILES=memory` по умолчанию.
4. `CLAUDE.md:64` — карта говорит «`docker-compose.yml` # профили: (default) / memory / gpu
   / dev»; формулировку диктовал я, и она опускает `neo4j`, который остаётся в этом файле с
   `profiles: ["memory","legacy"]` (доказано прогоном: `--profile legacy` по одному файлу
   даёт `neo4j`). Точнее — «…/ dev (+ `neo4j` профилей `memory`/`legacy`)». Nit, потому что
   исключение подробно описано в шапке `docker-compose.legacy.yml:26–29`.

### Влияние правки `Makefile` оркестратором на тексты документов

Проверено отдельно. Смысловых расхождений правка не создала: все документы говорят
«`PROFILES=…` или `COMPOSE_PROFILES` в `.env`», и именно эти два пути после правки работают
одинаково (раньше `make` в трёх формах записи видел меньше файлов, чем compose).
Единственное следствие для текстов — сдвиг номеров строк, то есть Nov-1; плюс Nit-3:
путь через `.env` теперь стоит называть везде, где называется `PROFILES=`.

### Что проверено запуском

| Что | Команда | Результат |
|---|---|---|
| Раскладка профилей по файлам (N-1) | `docker compose --env-file build/versions.env --env-file .github/ci.env --profile legacy --profile bot config --services` и то же с `-f` всех трёх файлов | 7 сервисов против 11; разница — ровно `telegram-bot`, `chromadb`, `narrative-orchestrator`, `semantic-memory`; `neo4j` в обоих случаях |
| Линтер композиции (N-4, Nit-4) | `bash scripts/compose-lint.sh` | `ok — 15 services in 3 file(s), 7 rules (profiles resolved: bot, dev, gpu, legacy, memory)`, код 0 |
| Реализованные подкоманды (Nit-3) | `go run ./cmd/mvctl --help` | 14 команд: реализованы `contracts, env, privacy, storage, version`; девять помечены `[reserved, EPIC-00N]` |
| Версия плагина compose (README:22) | `docker compose version` | `Docker Compose version v5.2.0` — цифра в README верна |
| `python3` на машине владельца (N-4) | `command -v python3`, запуск однострочника | путь через `WindowsApps`, но это настоящий интерпретатор (код 0) — ложной тревоги про Store-заглушку нет |
| Номера строк `Makefile` (Nov-1) | `grep -n` по рабочей копии и `git show :Makefile` | 224 (`up`) / 65 (`ACTIVE_PROFILES`) / 198 (`ci`) — в индексе и в дереве одинаково |
| Целостность записи в `dev-log.md` (N-2) | `git diff -- …/dev-log.md` | один кусок `@@ -5798,3 +5798,101 @@`, только добавления в конец файла |
| Ссылки пяти документов области | обход всех относительных ссылок с проверкой существования цели | битых нет |
| Состав `build/`, пины, состав `ci` | `ls build/`, `grep` по `build/versions.env`, `Makefile:198` | совпадают с текстами README / `CLAUDE.md` / `AGENTS.md` |

Не проверялось и почему: ни одна цель `make` целиком — GNU make не установлен; контейнеры
не поднимались — по указанию оркестратора; профиль `legacy` не собирался — `CHROMA_IMAGE`
намеренно пуст (D-3).

### Остаток и передача

1. Nov-1 и Nov-2 — три строки в `README.md`, `Docs/ops/runbook.md`,
   `services/_archive/README.md` (плюс необязательная правка номеров в `dev-log.md`).
   Исполнитель — tech-writer, размер — минуты. **Предусловие T-020**: волна сливается в
   `integration/mvp-1`, и эти строки уедут в историю. Карточку заводит оркестратор — раздел
   задач волны 1 веду не я.
2. Стендовый прогон «5 команд» владельцем — условие T-020, не T-019 (решение приёмки #2
   подтверждено).

---

## Приёмка волны 0 (T-020) · 2026-09-11 · tech-lead#1 (TEAM-1)

### Границы и метод

Принимаю **волну целиком**, а не отдельную задачу: состав T-001…T-019 плюс T-397
(выполнена до T-020). Полные прогоны сборки, линтера, коротких, сквозных и
интеграционных тестов сделал оркестратор перед приёмкой (`journal.md`, запись
«ПОЛНАЯ ПРОВЕРКА ВЕТКИ»); я на них опираюсь и **не повторяю**. Своими руками
перепроверил то, от чего зависит вердикт: сборка модуля, тесты ядра волны и
сквозные, `mvctl contracts check`, состав артефактов заглушек и фикстур, состояние
рабочего дерева, выборочная сверка утверждений в журналах с кодом. Ничего не
правил, не коммитил, не сливал и тега не ставил — это действия владельца.

Что прогнал сам (2026-09-11, ветка `epic/EPIC-001-foundation`):

- `go build ./...` — чисто.
- `go test -short -count=1 ./shared/testkit/... ./internal/mechanics/... ./cmd/...`
  — 16 пакетов `ok`.
- `go test -tags e2e -count=1 ./test/...` — `test/e2e` и `test/fixtures` зелёные.
- `go run ./cmd/mvctl contracts check` — 65 типов, 8 топиков, 58 файлов схем.
- состав: 8 файлов `FROZEN.md`; `services/_archive/` с индексом; шесть подпакетов
  `shared/testkit/{contract,gateway,mechanics,membus,state,swarm}`; фикстуры мира и
  снапшот seq 0; `rules/dark-forest.yaml`; три compose-файла; семь правил
  `compose-lint` и семь фикстур; семь заданий в `.github/workflows/go.yml`; правило
  `no-testkit-in-production` и именное исключение по `cmd/multiverse/fake_contexts.go`
  в `.golangci.yml`.

Не проверял и проверить не могу: ни одной цели `make` (GNU make на машине нет) и
стендовый прогон команд README на чистой машине.

### Вердикт

**Волна 0 принята с условиями.** Инженерное содержание волны сделано и доказано;
ворота волны 1 (T-014) закрыты ревьюером, T-017 и T-018 приняты. Условия —
в разделе «Незакрытые условия»: они не про качество кода, а про действия владельца
и про гигиену артефактов, которую нельзя закрыть агентом.

Начинать волну 1 можно **сейчас**, не дожидаясь стендового прогона: бридж-блок
(T-214, T-215, T-219, T-220, T-255) не зависит ни от `make`, ни от живого стенда —
он работает на `membus`, фикстурах и заглушках, которые волна 0 поставила.

### 1. Полнота волны

| Задача | Факт в дереве | Приёмка |
|---|---|---|
| T-001 F-1 | `.gitleaks.toml`, `.pre-commit-config.yaml`, `.gitignore`, `.gitattributes`, `.env.example`, плейсхолдер ключа | ревью #1 — принять, коммит `04a3a15` |
| T-002 F-3 | `services/_archive/**` (индекс + `ARCHIVED.md`), 8 `FROZEN.md`, `Docs/archive/` | ревью #1 — принять, коммит `5a20bb8` |
| T-003 F-2 | единый модуль, `cmd/multiverse` с 7 контекстами-заглушками, `shared/{runtime,clock}`, `.golangci.yml`, `build/Dockerfile` | итерация 2, проверено оркестратором |
| T-004 F-6a | `build/versions.env`, `build/minio.Dockerfile`, ядро compose, `.dockerignore` | ревью #1 — принять + правки безопасности |
| T-005 F-4a | конверт C-01 v1.1, `Bus`/`Journal`/`Dedup`, kafka-адаптер, DLQ | итерация 2, покрытие 76,1 % |
| T-006 F-4b-1 | `shared/contracts`, `_common`/`_envelope`, схемы блоков «а» и «б» | итерация 2 |
| T-007 F-5 | `shared/env` (манифест), `shared/objstore`, `shared/logging` | итерация 2 |
| T-008 F-6b | 5 профилей, `redpanda-init`/`minio-init`, Makefile, `compose-lint` | итерация 2, проверено на живых контейнерах |
| T-009 F-4b-2 | 31 схема блока «в», политики топиков, `Spec.Reserved` | правка M-1 внесена оркестратором |
| T-010 F-4c | `cmd/mvctl`: `contracts check/topics`, `env check`, `storage init`, `privacy scan`, `version`, 10 зарезервированных имён | ревью #1 — принять + доработка |
| T-011 F-10a | `shared/entity` v2: ops, канонический JSON, `state_hash`, статусы | итерация 2 (три Major закрыты) |
| T-012 F-7 | 7 заданий CI, `dependabot.yml`, `CODEOWNERS`, `coverage-gate.sh` | итерация 2 (Critical-1 закрыт) |
| T-013 F-8 | **частично**: `prompts.jsonl`, `bench-matrix.json`, `llm-bench.sh/.ps1`, `baseline.md` — **шаблон**, замер не сделан | оснастка принята оркестратором; замер уходит в волну 1 |
| T-014 F-5t | `testkit` ядро, `membus`, контрактный набор 20 кейсов на двух реализациях | ревью #4 — принять, **ворота закрыты** |
| T-015 F-10b | `internal/mechanics` + `rules/dark-forest.yaml` v0.1, золотая таблица бросков | ревью #2 — принять + Minor-8 |
| T-016 F-10c | фикстуры «Тёмного леса», указатель и объект снапшота seq 0, тест в `test/fixtures` | ревью #1 — принять, оба Minor закрыты |
| T-017 F-10d | `testkit/state.FakeState` v0, `testkit/mechanics.FixedMechanics` | ревью #2 — принять |
| T-018 F-10e | `testkit/gateway.Harness` v0, `testkit/swarm.FakeNarrator` v0 + `template/ru.go`, e2e «заглушки v0» | ревью #1 — принять, доводка проверена |
| T-019 F-9 | README, CLAUDE.md, AGENTS.md, runbook, `stack` | приёмка #3 — принять |
| T-397 | `docker-compose.bot.yml`/`.legacy.yml`, правило 7 линтера, формат `.env.example` | ревью #1 — принять |

**Формально закрытых задач не нашёл.** Два уточнения к «полноте»:

1. **T-013 закрыта наполовину по существу дела, и это честно записано.** `ops/metrics/baseline.md`
   существует, но это шаблон с прочерками и явной пометкой «заполняется после прогона на стенде».
   DoD T-020 такой исход допускает («`baseline.md` есть **или** зафиксирован перенос T-013»), но
   перенос зафиксирован только в `journal.md`. Карточки на замер в разделе задач волны 1 нет —
   ровно тот дефект, ради которого раздел и заводили.
2. **T-018 сдана в сокращённом составе** (нет `Attack`/`Flee`, боевого сценария и видов нарратива
   `turn`/`death`/`world_event`). Сокращение — решение оркестратора, оно обосновано и каждое
   отсутствие закреплено тестом. Но следствие для волны 1 никем не подхвачено, см. раздел 2 п. 1.

### 2. Годность для волны 1

Что волна 1 ждёт по `roadmap.md` (веха M0+) и что реально есть:

| Ожидание | Состояние |
|---|---|
| Контракт событий и его якорь | **есть**: реестр 65 типов / 8 топиков / 58 схем, контрактный набор 20 кейсов зелёный на `membus` и на живой Redpanda |
| Механика как данные | **есть**: `internal/mechanics` (типы C-03, `Load`, грамматика формул, RNG с золотой таблицей), `rules/dark-forest.yaml` v0.1; `Resolve`/`ChangesFor`/`NPCTarget`/`Invariant.Check` — заглушки для EPIC-002, как и планировалось |
| Фикстуры мира | **есть**: 6 сущностей, указатель и объект снапшота seq 0, тест выводит производные числа, а не повторяет их |
| Заглушка состояния | **есть**: `FakeState` v0 + `FixedMechanics`, подмена C-03 держится компилятором |
| Заглушка шлюза | **есть частично**: `Harness` v0 умеет создание персонажа, вход/выход, `look`, `say`, `rest`; **боевых действий нет** |
| Заглушка рассказчика | **есть частично**: `FakeNarrator` v0 даёт `entry` и `round`; `turn`/`death`/`world_event` отсутствуют осознанно |
| Инфраструктура запуска | **есть**: три compose-файла, Makefile, CI из 7 заданий, `mvctl`; **не исполнена ни одна цель `make`** |

Три конкретных места, где бридж-блок упрётся, если ничего не сделать:

1. **T-219 (`FakeEncounter`) не сможет выполнить свой DoD «e2e `solo-30` … 30 ходов».**
   Сценарий боя нужно кому-то издавать: `FakeEncounter` подписан на `player.attacked` и
   `player.flee_attempted`, а `Harness` v0 таких действий не имеет (методы —
   `CreatePlayer`, `Enter`, `Leave`, `Look`, `Say`, `Rest`). По `ownership.md` §1
   `shared/testkit/gateway` принадлежит EPIC-004, чьи задачи в последовательном порядке
   идут **после** бридж-блока. Владельца у правки сегодня нет.
   **Решение тимлида**: расширение `Harness` v0 на `Attack`/`Flee` (+ шаг сценария) —
   отдельная задача бридж-блока размера S, исполняется в паре с T-219, путь владения
   EPIC-004 открывается разово через tech-lead#1 (та же процедура, что у T-255 с
   `cmd/multiverse/fake_contexts.go`). Заводит карточку оркестратор; без неё DoD T-219
   придётся ослаблять, а это ровно тот случай, когда «зелёный e2e» перестаёт что-либо
   значить.
2. **T-220 придёт с видом нарратива, противоречащим волне 0.** Задача T-220 велит отвечать
   на `player.looked` видом `turn` (по C-05), а волна 0 по решению оркестратора сделала
   `entry` и закрепила это тестом. Пока architect#1 не поправит C-05, исполнитель T-220
   либо сломает тест волны 0, либо переоткроет вопрос. Правка C-05 — предусловие T-220,
   а не «когда-нибудь в бридж-блоке».
3. **C-03 в `contracts.md` до сих пор описывает четыре сигнатуры не так, как они
   реализованы** (`ChangesFor` без канала ошибки и с `[]entity.Change`, `DiceRolledPayload(roll,
   roller string)`, `ActorFromEntity(entity.Entity) (Actor, error)`, `NPCTarget(...) *Actor`);
   §17 там же называет двойник механики в `shared/testkit/state`, а он в
   `shared/testkit/mechanics`. Решение «править контракт, а не код» принято, но не исполнено.
   Важно: `NPCTarget` — единственный пункт, где после правки контракта придётся тронуть и
   **код** (`internal/mechanics/target.go` возвращает `*Actor` без ошибки); это работа
   EPIC-002 T-053/T-054, не бридж-блока.

Отдельно: `MV_SWARM_FAKE` в манифесте `shared/env` и в `.env.example` **не объявлен**
(исключение `depguard` по `cmd/multiverse/fake_contexts.go` — уже есть). T-255 это
предусматривает, но правка `shared/env` идёт через tech-lead#1 — предупреждён.

### 3. Честность артефактов (выборочная сверка)

За волну нашлось несколько утверждений, которых не держал ни один тест; проверил, что
исправления на месте — каждое **по коду**, а не по записи о правке:

- `dice.rolled.roll.seed` — в схеме `string` с шаблоном `^(0|[1-9][0-9]{0,19})$`,
  в коде `strconv.FormatUint`. Дефект контракта закрыт.
- `Harness`: «изменение, о котором харнесс не слышал, даёт `version_conflict`» — тест есть
  (`TestAChangeTheHarnessNeverHeardOfIsRefused`), запись о нём была исправлена автором вслух.
- T-017 Minor-7: формулировка про ошибку подписки в `Start` сужена — в `dev-log.md` прямо
  сказано, что свойство есть только у шины, реализующей интерфейс готовности, и такой в
  дереве нет.
- T-014 Nit-1 и Nit-2: обе пометки оркестратора на месте, в том числе контрпример ревьюера
  «82 нуля и две единицы из 84» рядом с прежним «0 из 64».
- `CLAUDE.md` действительно переписан (единый модуль, оба новых compose-файла названы) —
  это был Major приёмки #2 T-019.
- `.env.example`: инлайн-комментариев после пустого значения не осталось ни одного.

**Одно место осталось.** В записи `dev-log.md` «T-014 · итерация 3» стоит вывод «вместе
они дают 8 из 8», который ревью #3 прямо опровергло замером (6 зелёных из 35). Запись
итерации 4 разбирает механизм заново и приходит к другому решению, но в самой итерации 3
пометки нет, хотя в этом же файле у более мелких утверждений такие пометки есть.
Не блокирует ничего; правка — одна строка, отдаю в T-394 (там же живёт второй якорь).

### 4. Долги, уходящие в волну 1

**Заведённые карточки** (`tasks.md`, раздел задач волны 1): T-394 (интеграционные тесты
адаптера Kafka), T-395 (кейс «Close под падающим обработчиком»), T-396 (U-11, каталоги
среды разработки в индексе), T-398 (раскол `docs/` и `Docs/`, устаревшие файлы-инструкции),
T-399 (`infrastructure.md` под топологию T-397). T-397 закрыта.

**Заявки архитектору** (бридж-блок, до или параллельно с T-214/T-215):

1. Расширение C-01: «контекст обработчика отменяется при `Close`» — без этого строго
   детерминированного якоря мутации H в границах контракта не существует.
2. C-03 v1.2: четыре сигнатуры (`Roll`, `ActorFromEntity`, `ChangesFor`, `DiceRolledPayload`)
   и `NPCTarget` с каналом ошибки; §17 — имя подпакета двойника механики.
3. C-01: признать гарантией «новая группа читает с первого офсета» (свойство обеих
   реализаций) и закрепить контрактным тестом — тогда `readySubscriber` удаляется целиком.
4. C-05: вид нарратива на `player.looked` — `entry`, а не `turn` (предусловие T-220).
5. C-04: нужен ли `scope` в предложении при движении (правка EPIC-004 T-301).
6. `infrastructure.md`: топология трёх compose-файлов (это и есть T-399), плюс §4.4/§4.10
   `state-and-mechanics.md` по объекту снапшота и `size_bytes`, правило «одна сущность —
   один набор изменений» (§4.5), предел 1000 в грамматике формул (§5.3).
7. Ратификация ADR-001: правило `no-testkit-in-production` и исключение
   `shared-testkit-mechanics` в `.golangci.yml`.

**Непокрытые расхождения реализаций шины** (записаны в `dev-log.md` как известные):
`Close` под работающим `Tail`/`ReadRange` (`membus` 3 из 40, Redpanda 40 из 40);
`Close` под падающим обработчиком (T-395); обработчик, игнорирующий отмену
(`membus` коммитит всегда, kafka даёт 3 перевыдачи из 5). Кейсов нет сознательно: набор
не может закреплять расхождение, пока C-01 не скажет, какое поведение правильное. Это
делает заявки 1 и 3 не «бумажной» работой, а условием, чтобы у трёх дыр появился владелец.

**Детектора гонок нет там, где он нужнее всего**: задание `unit` идёт с `-short`,
`integration` — без `-race`, поэтому кейс `Close` и контрактный набор на брокере не
попадают **ни под одно** задание CI с `-race`. Задачу заводить оркестратору; по объёму
это S (отдельное задание либо `-race` у `integration` с пересмотром бюджета времени).

**Долги без карточек, которые надо завести** (иначе повторяем ошибку, ради которой
раздел волны 1 и появился):

- замер LLM на стенде (остаток T-013) и заполнение `ops/metrics/baseline.md`;
- стендовый прогон команд README на чистой машине (остаток T-019);
- расширение `Harness` v0 на `Attack`/`Flee` (раздел 2 п. 1);
- задание CI с детектором гонок для кейсов жизненного цикла шины;
- три строки Nov-1/Nov-2 из приёмки #3 T-019 (README, runbook, `services/_archive/README.md`)
  и пометка к записи итерации 3 T-014 — попутно в T-394/T-398.

### 5. Незакрытые условия приёмки

1. **Стендовый прогон владельцем.** GNU make на машине не установлен; ни одна цель `make`
   не исполнялась ни разу, включая правки Makefile из T-397 (разбор `.env`, подключение
   файлов профиля) — они проверены только эмуляцией оболочкой. CI цели `make` не вызывает,
   то есть автоматической страховки нет тоже. Нужно: `make -n up PROFILES=bot`,
   `make -n up PROFILES=memory,legacy`, `make compose-lint`, затем настоящие `make ci` и
   `make up` на чистой машине с `/health` контекстов `core|gateway|memory`.
   **Без этого не закрываются три критерия готовности эпика** (`epics.md` §6 / `roadmap.md`
   M0): «`make ci` зелёный», «`make up` поднимает инфраструктуру и пустые контексты» и
   «команды README выполняются на чистой машине».
2. **Коммит владельца** (`commits=ask`), затем слияние в `integration/mvp-1` и тег
   `mvp-1/wave-0`. Критерий «тег проставлен» до этого не закрывается по определению.
3. **`git status` не чист.** В рабочем дереве 15 неиндексированных `go.mod`/`go.sum`
   legacy-сервисов — след `go work sync` по рабочему пространству, которого больше нет
   (решение по ним было отложено до T-020). **Решение тимлида: откатить, а не коммитить.**
   Модули вне сборки (`go build ./...` их не видит), правка ничего не чинит, а в замороженных
   и архивных сервисах меняет файлы, которые должны остаться такими же, какими были в
   последнем рабочем коммите. Если владелец предпочтёт сохранить — только отдельным коммитом
   `chore`, вне коммита волны. Каталоги `.claude/**`, `.mcp.json`, `.qwen/**` — территория
   T-396, `Docs/user-stories/**` — наследие проекта, к волне отношения не имеет.
4. **У T-397 нет записи в `dev-log.md`** — общий DoD §1 п. 5 требует её явно. Ход работы
   восстанавливается только по `journal.md` и `review.md`. Ровно то же замечание волна уже
   получала по T-019 (N-2 приёмки #2). Запись — за исполнителем задачи.

Пункты 1 и 2 — действия владельца, пункты 3 и 4 — минуты работы, но до слияния.

### 6. Оценка процесса (в ретроспективу)

**Что было оправданно.**

- **Четыре ревью T-014.** Это ворота волны, и предметом спора была не реализация, а сила
  доказательства: держит ли набор правку `kafka.go`. По дороге найден настоящий дефект
  общего кода (`Close` навсегда подвешивал подписку между обработчиком и коммитом) —
  штатная остановка процесса не завершалась. Три возврата подряд дали то, ради чего волна
  и затевалась: якорь, который краснеет 27 прогонов из 27 при 0 ложных из 21.
- **Две итерации T-015 и T-017.** Обе — про класс ошибок «тест выводит ожидание из той же
  функции, которую проверяет» (RNG) и «молчаливая потеря данных» (второй набор изменений
  на сущность). Оба дефекта пережили бы волну и всплыли в EPIC-002.
- **Разбор механизма исполнителем против разбора оркестратора** (T-014 итерация 4): решение
  оркестратора оказалось неверным по существу, исполнитель это доказал зондом и отклонился
  с обоснованием, ревьюер отклонения принял. Это правильная работа команды, а не сбой.

**Что говорит о недостатке в постановке задач — мой счёт.**

1. **DoD описывал состав, а не доказательство.** «Contract-тест зелёный на обеих
   реализациях» — это про цвет, а не про способность ловить регрессию; критерий силы якоря
   («каждый кейс доказан мутацией», «якорь красен N из N прогонов») изобретался по ходу трёх
   ревью. Для задач, которые ставят якорь под общий код, этот критерий обязан быть в DoD
   **с самого начала** — писать его тимлиду, а не выводить ревьюеру.
2. **Для детерминированных алгоритмов не был потребован золотой вектор, посчитанный вне
   реализации.** Ровно это стоило T-015 одной итерации, и это же дешевле всего описать
   одной строкой DoD.
3. **Документарную задачу поставили параллельно правке того, что она документирует.**
   T-019 шла одновременно с T-397, которая переставила топологию compose, `.env.example` и
   Makefile, и ещё оркестратор правил README по ходу. Две из четырёх итераций — про то, что
   документ отстал от дерева, а не про качество текста. Правило на будущее: поверхность,
   которую описывает документарная задача, должна быть заморожена до её старта.
4. **Пункт DoD «dev-log заполнен» не проверялся на входе в приёмку.** Дважды (T-019, T-397)
   записи не оказалось. Это проверка на десять секунд, и она должна быть первой — до чтения
   кода.
5. **«Заглушки v0» ставились без матрицы отказов.** Задача перечисляла, что двойник делает,
   и не перечисляла, что он обязан отвергнуть; Major-1 T-017 — прямое следствие.
6. **Общее рабочее дерево на двух агентов** дало ложные срабатывания `pre-commit` и —
   опаснее — новые файлы T-397, оставшиеся вне индекса. Правило «после задачи, создающей
   файлы, смотреть `git status --untracked=all`» уже записано; его надо перенести в общий
   DoD, а не держать в журнале.

Возвраты в волне: 13 задач из 20 возвращались хотя бы раз, и **в каждом возврате был
настоящий Major или Critical**. Читать это как «плохо работают разработчики» — неверно;
это цена постановки, где доказательство не входило в задание.

### 7. Решения тимлида, принятые этой приёмкой

1. Имя ветки EPIC-002 фиксируется как **`epic/EPIC-002-state-mechanics`** (как каталог
   `epics/`); строку `teams.md` §3.2 (`epic/EPIC-002-state`) правит владелец файла плана —
   правка вне моего мандата на эту сессию, передаю оркестратору.
2. Передача владения подпакетами `testkit/{state,mechanics,gateway,swarm}` поставщикам
   **уже записана** в `ownership.md` §1 (v0.3/v0.4), отдельной правки не требуется.
3. Неиндексированные `go.mod` legacy-сервисов — откатить, в коммит волны не включать (§5 п. 3).
4. T-018 принимаю в сокращённом составе; недостающие боевые действия харнесса — задача
   бридж-блока, а не долг T-018 (§2 п. 1).
5. Волна 1 стартует бридж-блоком, не дожидаясь стенда (§8).

### 8. Рекомендация по старту волны 1

Порядок первого блока: **заявки архитектору 2 и 4** (C-03 и C-05 — предусловия T-219/T-220,
полдня работы architect#1) → **T-214** (схемы роя, тиков, мира, законов) → **T-215** (схемы
LLM и нарратива) → **T-219** `FakeEncounter` вместе с новой задачей на `Attack`/`Flee`
в `Harness` → **T-220** `FakeNarrator` → **T-255** хук `MV_SWARM_FAKE` (вместе с объявлением
переменной в `shared/env` и `.env.example` через tech-lead#1). T-394 и T-395 можно вести
вторым слотом параллельно — они ни от чего в бридж-блоке не зависят и закрывают самое тонкое
место волны 0.

## T-404 · ревью #1 · 2026-09-10 · code-reviewer#2

### Границы ревью

`shared/env/vars.go` (блок LLM), `scripts/llm-server.ps1`, `scripts/llm-server.sh`,
`.env.example`, раздел 3 «LLM» в `Docs/ops/runbook.md`. Только чтением: `Makefile`
(цели `llm-*`, `health`), `cmd/mvctl/internal/env/env.go`, `shared/env/*` целиком с
тестами, `scripts/llm-bench.*`, `Docs/dev-team/architecture/infrastructure.md`,
ADR-005 с дополнениями 1 и 2, карточка T-404 и восемь последних записей журнала.
`.env` владельца не открывался ни разу: всё, что требовало его значений, запускалось
через `make`, который сам делает `set -a; . ./.env`.

Метод: сначала прогоны на живом стенде владельца (LLM на `127.0.0.1:8888`, стек поднят),
затем матрица паритета — обе реализации запускались на ОДНОМ наборе значений
`MV_LLM_PROVIDER`/`MV_LLM_URL`/`MV_LLM_API_KEY`, вывод и код возврата сравнивались
построчно. Действие `up` проверялось не на стенде, а в изолированной копии дерева
(`scripts/`, `build/versions.env`, `ops/`) с подставным «llama-server», который
записывает свои аргументы и живёт 40 с: так виден настоящий `--port`, и ни один процесс
владельца не затронут. Живой процесс LLM не останавливался, `llm-down` на рабочем дереве
не запускался, контейнеры не перезапускались.

### Вердикт

**ВЕРНУТЬ** — Critical 1, Major 8, Minor 12, Nit 3.

Главное заявленное сделано и подтверждено исполнением: порт `--port` действительно
выводится из `MV_LLM_URL` в обеих реализациях (снято с реальной командной строки
запущенного процесса), проба с хоста подменяет только `host.docker.internal`, старая
`MV_LLM_PORT=1234` игнорируется обеими, `make llm-health` и строгий `make health` на
стенде дают 0, манифест — 68 переменных, `mvctl env check` без `--env` — 0, ключ в списке
процессов не появляется. Возврат — не из-за этого.

Возврат из-за трёх вещей. Первая: пустое значение `MV_LLM_URL` PowerShell-реализация
считает «адреса нет, проверять нечего» и выходит с **кодом 0** — строгий `make health`
становится зелёным при полностью ненастроенном LLM (C-1). Вторая: утверждение
«расхождение действительно невозможно» верно для переменной, но не для правила вывода
адреса — правило размножено на четыре реализации с тремя разными поведениями, и на этом
же стенде `make bench` умрёт с ложным «the LLM process is not running» при живом сервере
(M-1). Третья: две реализации на одном входе расходятся не в мелочах — вплоть до
противоположных вердиктов о живом сервере (M-2…M-5).

Отдельно: **записи T-404 в `dev-log.md` нет вовсе** (M-8) — по журналу это третий такой
случай за эпик. Поэтому ни одно отклонение от карточки (например, незакрытый п. 2 про
способ закрепления версии рантайма) не обосновано нигде, кроме отчёта исполнителя.

### Замечания по серьёзности

#### Critical

**C-1. `scripts/llm-server.ps1:88` — пустой `MV_LLM_URL` даёт зелёную проверку здоровья.**
В PowerShell `??` реагирует только на `$null`, но не на пустую строку, поэтому
`($env:MV_LLM_URL ?? 'http://127.0.0.1:1234')` при `MV_LLM_URL=` (строка есть, значение
пустое — обычный результат «стёр значение, оставил ключ») даёт `''`. Дальше
`if ($endpoint)` ложно, `Show-Health` печатает `Write-NotStartable` и **возвращает 0**.
В bash `${MV_LLM_URL:-…}` подставляет значение по умолчанию, и поведение противоположное.
Замер, одинаковый вход:

```
MV_LLM_PROVIDER=openai_compat MV_LLM_URL=  (пусто)
  sh : llm: probing http://127.0.0.1:1234 (from MV_LLM_URL=http://127.0.0.1:1234)
       llm: http://127.0.0.1:1234 unreachable — the process is not running   exit=1
  ps1: llm: MV_LLM_URL= is not an address of this machine — provider
       openai_compat is not started or stopped from here                     exit=0
```

Последствие: `make llm-health` = 0 и строгий `make health` = 0 при ненастроенном LLM, на
Windows — то есть на основной машине проекта. Это тот же класс дефекта, что ложный FAIL
ядра и ложная «мёртвая LLM» из T-403, только в обратную сторону, а обратная хуже: ложный
отказ замечают сразу, ложное «ок» — никогда. `env.Validate` здесь не поможет:
`MV_LLM_URL` не `Required()` и имеет значение по умолчанию.

Как исправить: один хелпер на весь скрипт вместо `??` —
`function Env-Or([string]$n,[string]$d){ $v=[Environment]::GetEnvironmentVariable($n); if
([string]::IsNullOrWhiteSpace($v)) { $d } else { $v } }` — и применить его ко ВСЕМ
чтениям `$env:` (тот же корень у Mi-9 и Mi-10). Плюс: при пустом адресе у провайдера,
которому адрес положен, `health` обязан возвращать ненулевой код, как это уже сделано
для `ollama` без `MV_OLLAMA_URL`.

#### Major

**M-1. Источник адреса один, а правило вывода адреса — нет: `scripts/llm-bench.sh:638-664`
и `scripts/llm-bench.ps1:249-283` считают адрес пробы из того же `MV_LLM_URL`, но по
другим правилам, и на этом стенде дают ложный вердикт.** Что реально делают четыре места:

| | подмена `host.docker.internal` | срезание хвоста `/v1` | откат с `/health` на `/v1/models` |
|---|---|---|---|
| `llm-server.sh` / `.ps1` | **да** (T-404) | нет | **да** (T-403 → T-404) |
| `llm-bench.sh` / `.ps1` | нет | **да** | нет |

На стенде владельца `MV_LLM_URL=http://host.docker.internal:8888`, а у сервера нет
`/health` — значит `make bench` попадает в обе ямы сразу. Замер: `curl --max-time 5
http://host.docker.internal:8888/v1/models` с хоста = `000` (имя с хоста не резолвится),
`http://127.0.0.1:8888/health` = 404. Bench на этом входе печатает
`no answer from http://host.docker.internal:8888/health — the LLM process is not running.
Start it: make llm-up` — ровно то ложное утверждение, ради устранения которого затевалась
задача, и совет в нём вредный (сервер работает; `make llm-up` в этом состоянии сделает
то, что описано в M-6). Задача заявлена как «адрес имеет один источник истины» — с
переменной так и есть, но правило «как из переменной получить адрес пробы» размножено.

Как исправить: вынести вывод адреса в одно место на реализацию (для bash — функция в
`scripts/lib/llm-endpoint.sh`, подключаемая обоими сценариями; для PowerShell — общий
модуль), чтобы правило было ровно одно. Если это тянет правку `llm-bench.*` за пределы
задачи — минимум завести карточку и записать в dev-log, что расхождение известно; молча
оставлять нельзя, потому что это и есть тот самый дефект.

**M-2. `scripts/llm-server.ps1:150` — PowerShell-проба ходит через прокси, bash-проба нет;
о готовности LLM может отвечать прокси.** `Invoke-WebRequest` берёт системный прокси и
переменные окружения, `curl` в mingw уважает только `http_proxy` в нижнем регистре. На
машине владельца задан `HTTP_PROXY=http://127.0.0.1:10808`. Замер, одинаковый вход
`MV_LLM_URL=http://[::1]:8888` (на `::1` не слушает никто, `netstat` показывает только
`127.0.0.1:8888`):

```
  sh : llm: http://[::1]:8888 unreachable — the process is not running          exit=1
  ps1: llm: http://[::1]:8888/health = 503 loading (the model is still being read)  exit=1
```

`503` пришёл от прокси. Проверено прямо:
`[System.Net.WebRequest]::DefaultWebProxy.GetProxy('http://[::1]:8888/health')` →
`http://127.0.0.1:10808/`. То есть проверка здоровья способна доложить «модель грузится»,
когда по адресу вообще ничего нет, и наоборот. Как исправить: `-NoProxy` у
`Invoke-WebRequest`/`Invoke-RestMethod` и `--noproxy` у `curl` — одинаково в обеих
реализациях и с записью решения: платформа на Go по умолчанию УВАЖАЕТ `HTTP_PROXY`
(`http.ProxyFromEnvironment`), так что «как ходит проба» и «как ходит платформа» надо
свести сознательно, а не по умолчанию инструмента.

**M-3. `scripts/llm-server.ps1:99-119` — IPv6-литерал ломает классификацию адреса.**
`[uri]'http://[::1]:8888'` даёт `.Host` = `[::1]` (со скобками), а списки в скрипте
содержат `'::1'`. Следствия для loopback-адреса: `$startable` ложно (`up` откажется
запускать локальный сервер), блок пина сборки и VRAM не печатается, и — главное —
`Write-CloudWarning` печатает `MV_LLM_URL points outside this machine and
MV_LLM_CLOUD_ENABLED is not true`, прямо противореча ADR-005 доп. 2 п. 3, где loopback
исключён из определения облака. В `.sh` скобочная форма разобрана аккуратно и `[::1]`
считается локальным — реализации расходятся и здесь (замер см. в M-2). Как исправить:
сравнивать по `$uri.DnsSafeHost` (даёт `::1` без скобок) либо добавить `'[::1]'` в оба
списка `.ps1`; в `.sh` для симметрии добавить `::1` без скобок.

**M-4. Один вход — противоположные вердикты о живом сервере: `MV_LLM_URL` с userinfo.**
`.sh` разбирает `scheme://host[:port]` вручную и не отделяет `user:pass@`, поэтому
`user:s3cr3t@127.0.0.1` становится «хостом»; `[uri]` в PowerShell userinfo отбрасывает.
Замер на живом сервере владельца:

```
MV_LLM_URL=http://user:s3cr3t@127.0.0.1:8888
  sh : llm: probing http://user:s3cr3t@127.0.0.1:8888 (from MV_LLM_URL=…)
       llm: http://user:s3cr3t@127.0.0.1:8888/health = 404
       llm: WARNING MV_LLM_URL points outside this machine …            exit=1
  ps1: llm: probing http://127.0.0.1:8888 (from MV_LLM_URL=…)
       llm: …/v1/models = 200 ok … llm: VRAM …                          exit=0
```

Три беды сразу: противоположные коды возврата на одном входе; `.sh` печатает пароль в
консоль и в любой перехваченный лог; `.sh` считает loopback «вне этой машины» и потому
пропускает пин и VRAM и врёт про облачный гейт. Как исправить: в `.sh` срезать всё до
последнего `@` в `hostport` перед разбором хоста и порта, а строку `probing` печатать по
восстановленному адресу без userinfo.

**M-5. `scripts/llm-server.sh:296` — список моделей содержит ровно ОДНУ модель, последнюю.**
`sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'` жадный, а ответ
`/v1/models` приходит одной строкой, поэтому `.*"id"` доезжает до последнего вхождения.
Замер на подставном сервере с тремя моделями и на живом сервере владельца:

```
подставной (model-alpha, model-beta, model-gamma):  sh → «model-gamma»   ps1 → все три
живой сервер владельца (22 модели):                 sh → «comfy»         ps1 → все 22
```

Строка `sed` унаследована, но T-404 назначил список моделей ЗАМЕНОЙ пина версии для
не-llama.cpp рантайма («the way to pin the version of a remote runtime is the model of
/v1/models»), и runbook на него же отправляет при смене модели («новое имя должно быть в
списке»). Значит теперь это не косметика: на Linux-стенде обе процедуры дают неверный
ответ. Как исправить: `grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | sed
's/.*"\(.*\)"/\1/'` (или `tr ',' '\n'` перед `sed`), плюс проверка на подставном ответе
с тремя моделями.

**M-6. `up` не видит чужой сервер на своём адресе: запускает второй процесс и рапортует
об успехе чужим ответом.** Обе реализации решают «уже запущен» только по
`ops/llm-server.pid`; если файла нет, а адрес отвечает, скрипт стартует ещё один процесс
и затем печатает `= 200 (pid N)`, где `200` пришёл от чужого сервера, а `pid` — от
своего. Замер в изолированной копии (подставной «сервер» слушает 18899, подставной
«llama-server» не слушает ничего):

```
llm: starting …/fakebin.exe on 0.0.0.0:18899 (single mode); the platform calls it at http://127.0.0.1:18899
llm: http://127.0.0.1:18899/v1/models = 200 (pid 81554, log …/ops/llm-server.log)
llm: first call 140 ms                                                   exit=0
```

Случай не умозрительный: на стенде владельца LLM поднята руками, `ops/` содержит только
`metrics` и `models.txt`, pid-файла нет — то есть `make llm-up` сегодня сделает именно
это, а следующий `make llm-down` убьёт записанный pid (в лучшем случае — собственный
мёртвый процесс, в худшем — переиспользованный системой номер). И это прямо противоречит
подсказке самой цели: «a second call is a no-op». Как исправить: до запуска выполнить
пробу; если адрес отвечает 200/503, а pid-файла нет — сказать «по этому адресу уже
кто-то отвечает, и это не наш процесс» и не стартовать второй.

**M-7. `Docs/dev-team/architecture/infrastructure.md` — документ, на который ссылаются
шапки обоих скриптов и манифест, всё ещё предписывает удалённую переменную.** Три места:
§4.2, строка `MV_LLM_PORT=1234` в эталонном листинге `.env`; §6.2, скелет скрипта
`$port = [int]($env:MV_LLM_PORT ?? 1234)` и `'--port', $port`; §6.3, строка таблицы
«`--host 127.0.0.1 --port 1234` | **да**, из `MV_LLM_HOST`/`MV_LLM_PORT`». Пока это так,
следующий исполнитель восстановит переменную «по документу», и обоснование у него будет
формально верное. Задача числится за architect#1 + devops-engineer, так что правка
внутри задачи, а не за её пределами.

**M-8. Записи T-404 в `Docs/dev-team/epics/EPIC-001-foundation/dev-log.md` нет.** Проверено:
`grep -n "T-40" dev-log.md` не находит ничего, последняя запись — `## devops-engineer ·
T-397`. Следствие для ревью: ни одно отклонение не обосновано в дереве. В частности,
п. 2 карточки («сверка закреплённой сборки неприменима к другому провайдеру; нужен другой
способ закрепить версию среды выполнения») по коду не закрыт для случая, который на
стенде и есть: адрес локальный, сервер — не llama.cpp, и `make llm-health` по-прежнему
печатает `pinned build b10441; MV_LLM_BIN is not set, cannot compare` — дословно тот
симптом, который карточка называет проблемой. Из кода не видно, отложено это осознанно
или потеряно.

#### Minor

**Mi-1. Указание об отменённой переменной до оператора не доходит.** `sharedenv.Deprecated`
вызывается только из `cmd/mvctl/internal/env/env.go:114`, то есть только при
`mvctl env check --env`; в `Makefile:170` цель `contracts` зовёт `go run ./cmd/mvctl env
check` БЕЗ `--env`, а `env.Validate` про отменённые переменные не знает вовсе. Значит
оператор со старым `.env` не услышит ничего от `make up`, `make ci`, `make health` и
`make llm-health` — только если сам догадается загрузить `.env` и вызвать команду с
флагом, о котором ему никто не сказал. Замер: `MV_LLM_PORT=1234 mvctl env check --env` →
`[deprecated] MV_LLM_PORT: retired and still set … — remove it from the environment`,
код 1; `mvctl env check` без флага → 68 переменных, код 0. Как исправить дёшево: в
`$(call llm,…)` после `load_dotenv` печатать одну строку, если `MV_LLM_PORT` непуст, —
у скриптов `.env` уже загружен, и это ровно то место, где оператор ждёт объяснения.

**Mi-2. Ответ на открытый вопрос №1 (адрес с `/v1`): риск не теоретический, воспроизведён,
и в проекте уже есть готовое правило.** Замер на подставном сервере,
`MV_LLM_URL=http://127.0.0.1:18899/v1`; сервер записал полученные пути:

```
REQ GET /v1/health      REQ GET /v1/v1/models        обе реализации, exit=1
```

То есть канонический для облачных вендоров адрес `…/v1` даёт ложное «LLM мертва».
Реализации согласованы между собой — потому Minor, а не Major, — и облачного клиента
сегодня ещё нет (EPIC-003). Готовое правило лежит рядом: `llm-bench.sh:643` делает
`base=${url%/v1}`, `llm-bench.ps1:255` — то же. Рекомендация: перенести правило в
`llm-server.*` (закрывается вместе с M-1) и одной строкой записать в `.env.example` и в
runbook, что в переменной — базовый адрес БЕЗ `/v1`.

**Mi-3. Ключ с кавычкой или обратной косой в `.sh` уходит на сервер молча испорченным.**
Формат конфига curl разбирает кавычки и escape-последовательности, а ключ подставляется в
него как есть. Замер (подставной сервер печатает полученный заголовок):

```
MV_LLM_API_KEY='quo"te KEY'     sh → Authorization: Bearer quo          (обрезан)
MV_LLM_API_KEY='back\slashKEY'  sh → Authorization: Bearer backslashKEY (съеден \)
MV_LLM_API_KEY='sp ace KEY'     sh → Bearer sp ace KEY   ok
MV_LLM_API_KEY='ha#sh KEY'      sh → Bearer ha#sh KEY    ok
те же ключи в ps1: уходят дословно, кроме кавычки — см. Mi-4 и Mi-6
```

Оператор увидит 401 и будет чинить не то. Как исправить: экранировать `\` и `"` перед
подстановкой (`v=${MV_LLM_API_KEY//\\/\\\\}; v=${v//\"/\\\"}`) — сам способ передачи
(конфиг на stdin) правильный и проверен, см. «Что проверено экспериментом» п. 5.

**Mi-4. Путь утечки секрета в консоль: `scripts/llm-server.ps1:245` и `:437` печатают
`$_.Exception.Message`, а .NET вкладывает в это сообщение полное значение заголовка.**
Замер: `Invoke-WebRequest … -Headers @{Authorization='Bearer quo"te KEY'}` → исключение
`The format of value 'Bearer quo"te KEY' is invalid.` Через `llm: warm-up call failed
($($_.Exception.Message))` это уходит в консоль и в перехваченный вывод, что запрещено
ADR-009 п. 4 (значение секрета не появляется ни в ошибке, ни в логе). Как исправить: в
обоих `catch` печатать тип исключения и код состояния, а не текст, либо прогонять текст
через замену значения `MV_LLM_API_KEY` на `***`.

**Mi-5. Локализованные сообщения .NET выходят кракозябрами — дефект 3 из T-403 вернулся
через тексты исключений.** Замер (прогон из M-6 и разбор битого pid-файла):

```
llm: warm-up call failed (��� ���ﭨ� �⪫��� �� 㪠�뢠�� �� �ᯥ譮� �믮������: 404 …)
InvalidArgument: … ���������� �८�ࠧ����� ���祭�� "not-a-pid" � ⨯ "System.Int32"
```

Строки скрипта переведены на английский правильно, но текст исключения приходит из
локализованного .NET. Как исправить: в шапке скрипта поставить
`[Threading.Thread]::CurrentThread.CurrentUICulture = 'en-US'` либо не печатать текст
исключения вовсе (см. Mi-4).

**Mi-6. `scripts/llm-server.ps1:152` — `catch { return 0 }` превращает ЛЮБУЮ ошибку в
«процесс не запущен».** Замер: живой сервер, `MV_LLM_API_KEY='quo"te KEY'` →
`llm: http://127.0.0.1:18899 unreachable — the process is not running (make llm-up)`,
хотя сервер отвечает, а сломан заголовок. Туда же попадут ошибка TLS, отказ прокси,
неразрешимое имя. Совет «сделайте make llm-up» неверен во всех этих случаях. Как
исправить: отличать «соединение не состоялось» от «запрос не построен» — при
не-`HttpRequestException` печатать тип ошибки отдельной строкой.

**Mi-7. Прогревочный вызов: `.sh` считает успехом любой HTTP-код.** `curl` вызван без
`--fail`, поэтому 404 от сервера с другим набором точек — «успех». Замер, один и тот же
подставной сервер, отвечающий 404 на `/v1/chat/completions`:

```
sh : llm: first call 140 ms
ps1: llm: warm-up call failed (…404 Not Found…); the server is up, check the blueprint model name
```

Как исправить: `curl --fail-with-body` (или проверка `%{http_code}`), чтобы «первый
вызов» означал одно и то же в обеих реализациях.

**Mi-8. Битый `ops/llm-server.pid` роняет `down` в PowerShell.** Замер, файл содержит
`not-a-pid`: `.sh` → `no live server in ops/llm-server.pid — nothing to stop`, файл
удалён, код 0; `.ps1` → исключение `[int]"not-a-pid"`, код 1, файл остался. Это путь
восстановления после сбоя — самое неподходящее место для падения. Как исправить:
`[int]::TryParse($recorded, [ref]$id)`, при неудаче удалить файл и сказать об этом.

**Mi-9. Пустые числовые переменные дают в PowerShell 0, в bash — значение по умолчанию.**
`[int]''` в PS7 = 0 (проверено: `health` с `MV_LLM_NUM_CTX=` и `MV_LLM_SLOTS=` отработал
без ошибки), значит `up` соберёт `--ctx-size 0 --parallel 0`, тогда как `.sh` подставит
8192 и 1. Корень тот же, что у C-1.

**Mi-10. Пустой `MV_LLM_PROVIDER` — разные ветки.** `.sh` читает его как `openai_compat`
(значение по умолчанию манифеста), `.ps1` — как пустую строку: адрес берётся из
`MV_LLM_URL`, но `$startable` ложно, пин и VRAM не печатаются, а `up` откажет с текстом
`MV_LLM_URL=… is not an address of this machine`, называющим неверную причину. Корень
тот же, что у C-1.

**Mi-11. URL без схемы.** `.sh` принимает `127.0.0.1:8888` и работает, `.ps1` отвергает
сырым `Write-Error` с распечаткой строки исходника (и с кракозябрами в кириллическом
пути). Правило надо выбрать одно и оформить сообщением, а не error record.

**Mi-12. Предупреждение SEC-15 предупреждает — и делает.** Новая проверка (её раньше не
было: в прежнем `.sh` `MV_LLM_HOST` подставлялся ещё и в адрес пробы) срабатывает верно —
при `MV_LLM_HOST=0.0.0.0` обе реализации печатают WARNING. Но дальше обе передают
`--host 0.0.0.0` и поднимают сервер на всех интерфейсах; при культуре fail-closed (SEC-14)
это стоит либо запретить с явным флагом-исключением, либо записать в ADR, что
предупреждения достаточно. Мелочи паритета там же: `.sh` пишет предупреждение в stderr,
`.ps1` — в stdout; список loopback в `.ps1` не содержит `[::1]`, в `.sh` содержит.

#### Nit

**N-1.** Порядок аргументов сервера различается (`-m` до `--no-webui` в `.ps1`, после —
в `.sh`); на поведение не влияет, но мешает сличать логи двух стендов.

**N-2.** `§` и длинное тире в выводе `.ps1` транслитерируются консолью
(`infrastructure.md 6.3`). Безвредно, но если текст выводился на английский ради
читаемости — стоит и здесь обойтись ASCII.

**N-3.** Комментарий к `MV_LLM_URL` в `.env.example` занял пять строк; одной строки про
«базовый адрес без `/v1`» (Mi-2) в нём как раз не хватает.

### Паритет двух реализаций

Прогон обеих на одном входе, действие `health`, если не указано иное. «=» — сходится.

| Вход | `.sh` | `.ps1` | |
|---|---|---|---|
| `openai_compat`, `http://127.0.0.1:8888` (живой) | 200 ok, exit 0 | 200 ok, exit 0 | = кроме списка моделей (M-5) |
| `openai_compat`, `http://host.docker.internal:8888` | проба 127.0.0.1, 200, 0 | то же | = |
| `openai_compat`, `http://127.0.0.1:9` | unreachable, 1 | unreachable, 1 | = |
| `openai_compat`, `http://10.1.2.3:8888` | unreachable, 1 | unreachable, 1 | = (гейт молчит — верно) |
| `openai_compat`, `http://[::1]:8888` | unreachable, 1 | **503 loading от прокси** + ложный WARNING про облако | M-2, M-3 |
| `openai_compat`, `http://user:s3cr3t@127.0.0.1:8888` | 404, exit **1**, пароль в выводе | 200 ok, exit **0** | M-4 |
| `openai_compat`, `` (пусто) | default 1234, unreachable, 1 | «нечего проверять», **0** | C-1 |
| `openai_compat`, `127.0.0.1:8888` (без схемы) | 200 ok, 0 | error record, 1 | Mi-11 |
| `openai_compat`, `http://127.0.0.1:18899/v1` | `/v1/v1/models`, 1 | `/v1/v1/models`, 1 | = (Mi-2) |
| провайдер пустой | как `openai_compat` | адрес читает, но «не наша машина» | Mi-10 |
| `ollama` + `MV_OLLAMA_URL` | проба по `MV_OLLAMA_URL`, 0 | то же | = |
| `ollama` без `MV_OLLAMA_URL` | две строки, exit 1 | две строки, exit 1 | = (в `.sh` вторая в stderr) |
| `anthropic` | «не отсюда», 0 | «не отсюда», 0 | = |
| `recorded` / `fake` | «внутри процесса», 0 | то же, 0 | = |
| `MV_LLM_PORT=1234` при `MV_LLM_URL` на 8888 | игнорируется | игнорируется | = |
| `up`: аргументы сервера | `--port 18899 --ctx-size 16384 --parallel 2 …` | те же | = (N-1) |
| `up`: `MV_LLM_HOST=0.0.0.0` | WARNING в stderr | WARNING в stdout | Mi-12 |
| `up`: прогрев, сервер отвечает 404 | «first call 140 ms» | «warm-up call failed» | Mi-7 |
| `down`: нет pid-файла | nothing to stop, 0 | nothing to stop, 0 | = |
| `down`: устаревший pid 999999 | nothing to stop, 0, файл удалён | то же | = |
| `down`: `not-a-pid` в файле | чистит, 0 | исключение, 1, файл остался | Mi-8 |
| `down`: облачный адрес + живой pid | «не отсюда», pid-файл не тронут | то же | = |
| ключ с пробелом / `#` | уходит дословно | уходит дословно | = |
| ключ с `"` | молча обрезан | «unreachable», 1 | Mi-3, Mi-6 |
| ключ с `\` | `\` съеден | дословно | Mi-3 |

### Что проверено экспериментом

Все прогоны мои, на машине владельца, 2026-09-10.

1. **Порт выведен из `MV_LLM_URL` — снято с настоящей командной строки процесса**, а не с
   эха скрипта. В изолированной копии дерева, `MV_LLM_URL=http://127.0.0.1:18899`,
   `MV_LLM_NUM_CTX=8192`, `MV_LLM_SLOTS=2`; `Get-CimInstance Win32_Process` во время
   работы обеих реализаций даёт `fakebin.exe --host 0.0.0.0 --port 18899 --ctx-size 16384
   --parallel 2 …`. Главное утверждение задачи подтверждено для ОБЕИХ реализаций.
2. **Подмена только `host.docker.internal`.** `MV_LLM_URL=http://host.docker.internal:8888`
   → `probing http://127.0.0.1:8888 (from MV_LLM_URL=http://host.docker.internal:8888)`;
   `http://10.1.2.3:8888` проверяется как есть и без предупреждения про облако (RFC1918
   исключён верно, ADR-005 доп. 2 п. 3); `https://api.example.org` — «не отсюда» плюс
   предупреждение про закрытый гейт. Имя `host.docker.internal` с хоста действительно не
   резолвится: `curl` = `000`.
3. **Старая `MV_LLM_PORT=1234` при `MV_LLM_URL` на 8888 игнорируется обеими** — обе
   печатают `probing http://127.0.0.1:8888`.
4. **Отмена переменной объявлена и работает механически**: `MV_LLM_PORT=1234 mvctl env
   check --env` → `[deprecated] … remove it from the environment`, код 1. CI-режим не
   сломан: `mvctl env check` — `68 variables declared`, код 0. Сверка с примером
   двусторонняя; регрессия закреплена существующим тестом (`example_test.go:161`
   запрещает отменённые имена в `.env.example`). Ограничение — Mi-1.
5. **Ключ не попадает в список процессов.** Проба против «чёрной дыры»
   (`http://10.255.255.1:8888`, 5 с ожидания) с `MV_LLM_API_KEY=TOPSECRETKEY42`;
   одновременный `Get-CimInstance Win32_Process -Filter "name='curl.exe'"` даёт
   `curl.exe -s -K - -o nul -w %{http_code} --max-time 5 http://10.255.255.1:8888/health`
   — ключа нет; в выводе скрипта его тоже нет (`grep -c` = 0). Заявление автора
   подтверждено. Порча ключа спецсимволами — отдельно, Mi-3.
6. **Откат T-403 перенесён в `.sh` и работает на живом сервере**: `/health` = 404 →
   суждение по `/v1/models` = 200, и строка вывода прямо это называет. Обе реализации
   ведут себя одинаково; расширенный список кодов (401/403/404/405/501) в них совпадает
   дословно.
7. **Цели `make` на живом стенде**: `make llm-health` — код 0, адрес 8888, источник
   назван; строгий `make health` (`LLM_STRICT` по умолчанию 1) — код 0, `gateway ok`,
   `memory ok`, `core ok`, LLM 200.
8. **Не сломано то, что работало**: `go build ./...` — 0; `go vet ./...` — 0;
   `go test -short ./...` — 0 (ни одного FAIL); `golangci-lint run ./...` — `0 issues`;
   `gofmt -l ./cmd ./shared ./internal` — пусто; `mvctl env check` — 0.
9. **`up` и `down` проверялись только в изолированной копии** с подставным бинарником;
   живой процесс LLM владельца не останавливался, `make llm-down` на рабочем дереве не
   запускался, контейнеры не трогались, `nvidia-smi` вызывался только на чтение (его
   зовёт сам скрипт).

### Чего проверка не покрывает

- Поведение на настоящем Linux-стенде: `.sh` прогонялся под Git Bash на Windows. Разбор
  URL, коды возврата и работа с ключом от этого не зависят, но `nohup` и сигналы в ветке
  `up` на Linux не проверялись.
- Настоящий облачный эндпоинт не опрашивался: вместо него недостижимые адреса и
  подставной сервер. Утверждение про `/v1/v1/…` (Mi-2) доказано подставным сервером,
  печатающим полученные пути, а не запросом к вендору.
- `make bench` не запускался (тратит GPU владельца); вывод M-1 сделан из кода
  `llm-bench.*` плюс два замера: имя `host.docker.internal` с хоста не резолвится, и у
  сервера владельца нет `/health`.
- Дефект M-6 воспроизведён подставным сервером на 18899; на живом 8888 он намеренно НЕ
  воспроизводился.

### Ответы на три открытых вопроса автора

**1. Адрес с `/v1` у облачных вендоров — риск `/v1/v1/…`.** Не риск, а факт: воспроизведён
на подставном сервере, обе реализации запрашивают `/v1/health` и `/v1/v1/models` и
выносят «мертва» живому эндпоинту (Mi-2). Отвечать на вопрос заново не нужно — в проекте
уже принято решение и написан код: `llm-bench.sh:643` и `llm-bench.ps1:255` срезают
хвост `/v1`, потому что «`http://host:1234` и `http://host:11434/v1` — оба законные
значения переменной». Рекомендация: то же правило в `llm-server.*`, и одна строка в
`.env.example`, что переменная хранит базовый адрес.

**2. Правило «запускается отсюда» для адреса локальной подсети.** Текущее разделение
верное, ему не хватает только записи. `startable` отвечает на вопрос «этот процесс наш»
и потому включает лишь loopback, `0.0.0.0` и `host.docker.internal`; облачный гейт
отвечает на другой вопрос — «адрес за пределами доверенной сети» — и потому исключает
ещё и RFC1918, как предписано ADR-005 доп. 2 п. 3. Проверено: `http://10.1.2.3:8888` —
«не отсюда», без предупреждения о гейте; `https://api.example.org` — «не отсюда» и с
предупреждением. Случай «сервер на этой машине, но опубликован в подсеть» под запретом
SEC-15, так что отказ его запускать — не пробел, а следствие. Рекомендация: добавить в
таблицу runbook строку про адрес локальной подсети (`llm-up` — «не отсюда», гейт молчит),
чтобы правило не приходилось выводить из кода.

**3. У предупреждения о закрытом гейте нет кода возврата — и это ошибка.** Читается из
кода: `cloud_warning` печатает предупреждение, а код возврата определяет только проба
(`[ "$llm_code" = 200 ] || exit 1`, `if ($code -ne 200) { return 1 }`). Значит облачный
эндпоинт, который отвечает 200 при `MV_LLM_CLOUD_ENABLED` не `true`, даёт `make
llm-health` = 0 и строгий `make health` = 0, хотя провайдер по ADR-005 доп. 2 п. 3
откажется стартовать на этом адресе. Проверка отвечает на вопрос «сможет ли платформа
пользоваться LLM», а не «отвечает ли эндпоинт», поэтому здесь нужен ненулевой код — это
тот же ложно-зелёный, что C-1, только по другой причине. Замером не подтверждено:
публичного адреса, отвечающего 200, у ревью не было; вывод сделан из кода обеих
реализаций.

## T-404 · ревью #2 (итерация 2) · 2026-09-11 · code-reviewer#2

### Границы и метод

Оценивались: `scripts/lib/llm-endpoint.sh`, `scripts/lib/LlmEndpoint.psm1`,
`scripts/llm-server.{sh,ps1}`, `scripts/llm-bench.{sh,ps1}`, блок LLM в
`shared/env/vars.go`, `.env.example`, раздел 3 «LLM» в `Docs/ops/runbook.md`,
запись T-404 в `dev-log.md`. `Docs/dev-team/architecture/infrastructure.md` НЕ
оценивался (M-7 закреплён за architect#1 в T-399, решение оркестратора).
Только чтением: `Makefile`, `shared/env/env.go`, `cmd/mvctl/internal/env`,
`ops/metrics/bench-matrix.json`, восемь последних записей журнала. `.env`
владельца не открывался: всё, что требовало его значений, запускалось через
`make`, который сам делает `set -a; . ./.env`.

Стенд паритета собран заново и с нуля, чужой не использовался и не читался.
60 входов для `llm-server.*` прогонялись ОДНОВРЕМЕННО обеими реализациями с
одним и тем же набором переменных (все `MV_LLM_*` явно сбрасывались перед
каждым прогоном), stdout+stderr и код возврата сравнивались построчно;
нормализовались только пути дерева, pid и миллисекунды. Подставной сервер —
Python-заглушка, которая ЗАПИСЫВАЕТ полученные пути и заголовки: `404` на
`/health`, `200` на `/v1/models` — форма ответа сервера владельца. Действия
`up`/`down` выполнялись ТОЛЬКО в изолированной копии дерева с подставным
«llama-server» (Go, пишет свой argv и умеет слушать порт). `make bench` не
запускался; вместо него `scripts/llm-bench.{sh,ps1}` прогонялись в отдельной
копии дерева против подставного сервера, с выводом отчётов вне репозитория.

### ⚠ Происшествие на стенде владельца (сообщается первым, потому что важнее выводов)

**Живой сервер LLM на `127.0.0.1:8888` в ходе этого ревью остановлен и на момент
сдачи отчёта НЕ работает.** Это моя ошибка исполнения, а не дефект кода: проверяя
замечание про PID-файл, я записал в `ops/llm-server.pid` изолированной копии
номер процесса, который считал своим «подопытным» (`python -c sleep`), но
подопытный оказался `python3.13.exe`, а выбранный по имени `python.exe` —
процессом из окружения владельца. `scripts/llm-server.ps1` при `up` этот номер
прочитал и выполнил `Stop-Process -Force`; через несколько секунд перестал
слушать и порт 8888 (`unsloth-studio` как процесс жив, слушателя нет).
Сервер поднимался владельцем вручную, `MV_LLM_BIN` не задан, поэтому поднять
его обратно обвязкой нельзя и я этого не делал: **требуется ручной перезапуск
владельцем**. Прогоны против живого сервера, сделанные ДО происшествия
(строки 1–5, 14, 45 таблицы паритета, полный список из 22 моделей), в отчёте
помечены отдельно; строгий `make health` в зелёном состоянии в этой итерации
проверить уже не удалось — проверено только то, что он краснеет.
Побочное следствие: это же происшествие является натурным подтверждением
замечания M-5 ниже.

### Вердикт

**ВЕРНУТЬ** — Critical 0, Major 5, Minor 12, Nit 4.

Блокирующее C-1 итерации 1 закрыто по-настоящему, и это подтверждено
исполнением, а не чтением: пустое значение, отсутствующее значение, значение из
пробелов дают в ОБЕИХ реализациях один и тот же текст и код 1, `make llm-health`
и строгий `make health` при этом краснеют. Главное заявленное — «правило одно на
реализацию» — тоже сделано: ни один из четырёх скриптов больше не выводит адрес
сам (проверено `grep`-ом и исполнением), и замер на самом злом входе
(`host.docker.internal` + хвост `/v1` + сервер без `/health`) доходит до конца
и пишет отчёт, ни одного `/v1/v1/…` подставной сервер не видел. Требование
владельца по `/v1` реализовано именно так, как записано в журнале: обе формы
принимаются, проверка здоровья идёт по базовому адресу, `/v1` дописывается
только там, где точка действительно под ним. `/v1` в СЕРЕДИНЕ пути не трогается.

Возврат — не за это. Возврат за пять вещей, каждая воспроизведена прогоном:
ручной разбор адреса в PowerShell сравнивает строки ПО ПРАВИЛАМ КУЛЬТУРЫ, и на
одном входе две реализации дают противоположные коды возврата (M-1); признак
«запрос не состоялся» в bash теряется в подоболочке, то есть исправление Mi-6
в `.sh` не работает вовсе (M-2); ключ с управляющим символом в PowerShell молча
обрезается и расщепляет заголовок, а в bash роняет запрос — снова
противоположные вердикты на одном входе (M-3); «неустановленная переменная —
это не конфигурация» противоречит явному решению `shared/env` и оставляет для
случая «переменной нет» ДВА источника истины, платформенный и обвязочный (M-4);
и PID-файлу по-прежнему верят без единой проверки, что записанный номер — наш
процесс (M-5, натурно подтверждено выше).

### Статус прежних замечаний

Каждая строка — результат прогона, а не чтения кода, если не сказано иное.

| № | Что было | Статус | Чем подтверждено |
|---|---|---|---|
| **C-1** | пустой `MV_LLM_URL` → код 0 в PowerShell | **закрыто** | входы «пусто», «не установлена», «одни пробелы»: обе реализации печатают один текст, обе `exit 1`; `make llm-health` = 1, строгий `make health` = 2, `LLM_STRICT=0` = 0 |
| **M-1** | правило вывода адреса в четырёх местах | **закрыто** | `grep` по четырём скриптам: собственного вывода адреса нет ни в одном, остались только дописывания `/v1/models` и `/v1/chat/completions` к базе из модуля; `llm-bench.sh` и `.ps1` на входе `http://host.docker.internal:18993/v1` проходят шлюз, пишут CSV и отчёт, подставной сервер видит `GET /health`, `GET /v1/models`, `POST /v1/chat/completions` и ни одного `/v1/v1/…` |
| **M-2** | проба ходит через прокси в PowerShell | **закрыто** | при `HTTP_PROXY=HTTPS_PROXY=http_proxy=http://127.0.0.1:9` (заведомо мёртвый прокси) обе реализации отвечают `200 ok` по подставному серверу |
| **M-3** | `[::1]` объявлялся облаком | **закрыто** | вход `http://[::1]:8888`: обе печатают `unreachable — the process is not running (make llm-up)`, то есть считают адрес НАШИМ; предупреждения про облако нет ни в одной |
| **M-4** | userinfo: противоположные вердикты, пароль в консоли | **закрыто** | `http://user:s3cr3t@127.0.0.1:8888`: обе печатают `http://***@127.0.0.1:8888`, обе `exit 0`, `grep` по выводу пароля не находит. Остался побочный вопрос — см. Mi-4 |
| **M-5** | список моделей схлопывался до последней | **закрыто** | на живом сервере (до происшествия) обе печатают одинаковые 22 имени, начиная с `unsloth/Qwen3.6` и заканчивая `comfy`; на подставном — все три |
| **M-6** | `up` поднимал второй сервер на занятый адрес | **закрыто** | в изолированной копии, подставной сервер на 18991, PID-файла нет: обе печатают `… already answers 200 and ops/llm-server.pid records no process of ours — refusing to start a second server …`, обе `exit 1` |
| **M-7** | `infrastructure.md` предписывает удалённую переменную | **не оценивалось** | за architect#1 в T-399 (решение оркестратора) |
| **M-8** | записи T-404 в `dev-log.md` нет | **закрыто** | запись есть, покрывает обе итерации, отклонения названы поимённо с причинами; расхождений записи с кодом не нашёл |
| **Mi-1** | про отменённую `MV_LLM_PORT` оператору никто не говорит | **закрыто** | `MV_LLM_PORT=1234` + рабочий адрес: обе первой строкой печатают `llm: MV_LLM_PORT=1234 is retired and ignored — the port comes from MV_LLM_URL (T-404); remove the line from .env`; `mvctl env check --env` по-прежнему даёт `[deprecated] … remove it from the environment`, код 1 |
| **Mi-2** | адрес с `/v1` давал `/v1/v1/…` | **закрыто** | четыре формы (`…:18991`, `…/v1`, `…/v1/`, `…//v1`) — подставной сервер видит ровно `GET /health` и `GET /v1/models`; форма с `/v1` дополнительно печатает объясняющую строку. `/v1` в середине (`…/v1/proxy`) НЕ срезается: запросы идут в `/v1/proxy/health` и `/v1/proxy/v1/models` — это верно |
| **Mi-3** | ключ с `"` и `\` уходил испорченным | **закрыто (частично)** | ключи `sp ace KEY`, `quo"te KEY`, `back\slashKEY`, `ha#sh KEY`, `a=b;c KEY` и ключ длиной 8000 символов доходят до сервера ДОСЛОВНО и одинаково в обеих (снято с заголовка на подставном сервере). Управляющие символы — нет, см. M-3 |
| **Mi-4** | текст исключения .NET печатал ключ | **закрыто** | текста исключения в выводе нет ни в одном пути; `grep` по выводу и по списку процессов ключа не находит. Исключение — падение импорта самого модуля, см. Mi-3 |
| **Mi-5** | кракозябры из локализованного .NET | **закрыто** | в 60 прогонах ни одной битой строки; тире и `§` выводятся корректно. Кроме падения импорта модуля (Mi-3) |
| **Mi-6** | `catch { return 0 }` = «процесс не запущен» | **НЕ закрыто в `.sh`** | см. M-2. В `.ps1` работает |
| **Mi-7** | прогрев: `.sh` считал успехом любой код | **закрыто в `llm-server.*`, НЕ закрыто в `llm-bench.ps1`** | см. Mi-5 |
| **Mi-8** | битый PID-файл ронял `down` в PowerShell | **закрыто** | файл `not-a-pid`: обе печатают две одинаковые строки, обе `exit 0`, файл удалён; устаревший `999999` — тоже одинаково |
| **Mi-9** | пустые числовые переменные | **закрыто** | `MV_LLM_NUM_CTX=` и `MV_LLM_SLOTS=` → обе собирают `--ctx-size 8192 --parallel 1` (argv снят с подставного бинарника) |
| **Mi-10** | пустой `MV_LLM_PROVIDER` | **закрыто** | пустой провайдер обе читают как `openai_compat`, вывод совпадает дословно |
| **Mi-11** | адрес без схемы | **закрыто** | `127.0.0.1:8888` отвергают обе одним текстом, `exit 1` |
| **Mi-12** | SEC-15 предупреждал и всё равно делал | **закрыто** | `MV_LLM_HOST=0.0.0.0` и `MV_LLM_HOST=192.168.1.5`: обе отказываются, один текст, `exit 1`, подставной бинарник не запускается ни разу; `127.0.0.1`, `localhost`, `::1`, `[::1]` разрешены |
| **N-1** | порядок аргументов сервера | **закрыто** | argv двух реализаций совпал посимвольно: `--host 127.0.0.1 --port 18995 --ctx-size 16384 --parallel 2 … -m D:/Models/Qwen3-27B.gguf --no-webui` |
| **N-2** | `§` и тире транслитерировались | **закрыто** | см. Mi-5 |
| **N-3** | комментарий к `MV_LLM_URL` в `.env.example` | **закрыто по букве, спорно по смыслу** | строка есть; формулировка — см. Mi-8 |

Итог по прежним: из 24 замечаний закрыты 22, одно вне области (M-7), одно
(Mi-6) закрыто лишь в одной реализации из двух, ещё одно (Mi-7) — лишь в двух
скриптах из четырёх.

### Новые замечания

#### Critical

Нет. Ложно-зелёного класса C-1 в этой итерации не нашёл: ни один вход из 60 не
дал «зелёную проверку при неработающей LLM». Обратное — красная проверка при
работающем эндпоинте — есть, см. M-1.

#### Major

**M-1. `scripts/lib/LlmEndpoint.psm1:144, 159, 193, 219, 255` — ручной разбор
адреса сравнивает строки по правилам КУЛЬТУРЫ, а bash сравнивает байты; на одном
входе две реализации дают противоположные коды возврата.**
`String.EndsWith`, `StartsWith` и `IndexOf(string)` в .NET по умолчанию
культурно-зависимы, и «игнорируемые» символы Unicode (мягкий перенос `U+00AD`,
нулевой пробел `U+200B`) в таком сравнении НЕ УЧАСТВУЮТ. Три замера, вход один и
тот же:

```
MV_LLM_URL=http://127.0.0.1:8888/v1<U+00AD>
  sh : probing http://127.0.0.1:8888/v1<U+00AD> … /v1<U+00AD>/health = 404      exit=1
  ps1: probing http://127.0.0.1:8888 … ends with /v1 … /v1/models = 200 ok      exit=0

MV_LLM_URL=http://127.0.0.1:8888/v1<U+200B>      то же самое: exit 1 против 0

MV_LLM_URL=http<U+00AD>://127.0.0.1:8888
  sh : MV_LLM_URL=… uses scheme 'http<U+00AD>'; only http and https …           exit=1
  ps1: probing http<U+00AD>://127.0.0.1:8888 … unreachable — the process is not
       running (make llm-up)                                                    exit=1
```

Первые два — ровно тот дефект, ради которого писался общий модуль: один вход,
разные вердикты, причём PowerShell (реализация основной машины проекта) выдаёт
ЗЕЛЁНЫЙ по адресу, которого в переменной нет. Третий — та же причина с другой
стороны: `IndexOf('://')` находит схему там, где её нет, и обвязка советует
`make llm-up` строке, которая адресом не является. Невидимый символ в URL — не
экзотика, а обычный результат копирования адреса из документации вендора, из PDF
или из вики.

Шапка модуля обещает: «Every message printed from here exists in both, character
for character». Здесь это не выполняется по построению: две реализации выполняют
РАЗНЫЕ алгоритмы сравнения строк, и ни один стенд паритета этого не поймает,
пока в наборе входов нет невидимых символов.

Как исправить (дёшево и закрывает весь класс сразу): (1) все сравнения в
`Convert-LlmUrl` перевести на ординальные —
`EndsWith('/v1', [StringComparison]::Ordinal)`,
`IndexOf('://', [StringComparison]::Ordinal)`,
`StartsWith('[', [StringComparison]::Ordinal)`; (2) в ОБЕИХ реализациях
отвергать значение, содержащее символы вне печатного ASCII, одним сообщением —
платформенный HTTP-клиент такую строку всё равно закодирует не так, как ожидает
оператор, а «обвязка не должна работать там, где ломается платформа» — уже
принятый в этом же файле критерий (комментарий про Mi-11).

**M-2. `scripts/lib/llm-endpoint.sh:421-432, 452` — `LLM_PROBE_FAULT`
выставляется ВНУТРИ подстановки команд и до вызывающего не доходит никогда;
исправление Mi-6 в bash не работает.**
`llm_probe_code` печатает код и потому вызывается как `$(llm_probe_code …)` —
это подоболочка, и присваивание `LLM_PROBE_FAULT=1` умирает вместе с ней. Строкой
ниже `llm_health_probe` делает `LLM_HEALTH_FAULT=$LLM_PROBE_FAULT`, читая
неизменную инициализацию `0`. Замер напрямую по модулю:

```
. scripts/lib/llm-endpoint.sh
llm_health_probe "https://127.0.0.1:18991"     # TLS к простому HTTP-порту
  → CODE=000 FAULT=0 PROBE_FAULT=0             # curl при этом вышел с 35
```

Тот же вход через скрипты:

```
MV_LLM_URL=https://127.0.0.1:18991
  sh : … unreachable — the process is not running (make llm-up)           exit=1
  ps1: … unreachable — the process is not running (make llm-up)
       llm: the request itself failed — this is not "the process is not
            running"; check the address, the key and TLS                  exit=1
```

В `.sh` строка-диагностика мертва целиком: сюда попадают обрыв TLS, отказ прокси,
непостроенный запрос — все случаи, в которых совет «сделайте `make llm-up`»
неверен, ради чего Mi-6 и заводилось. Второй вход с тем же расхождением:
`http://a[b:8888`.

Как исправить: сделать `llm_probe_code` не печатающей, а присваивающей
(`LLM_PROBE_CODE`, как уже сделано для `LLM_HEALTH_CODE`) — тогда подоболочка
исчезает вовсе; либо возвращать признак вторым полем печатаемого значения. И
закрепить стендом T-405 одной строкой: «FAULT выставлен» — проверяемое свойство.

**M-3. `scripts/lib/LlmEndpoint.psm1:409` (`-SkipHeaderValidation`) против
`scripts/lib/llm-endpoint.sh:397-408` — ключ с управляющим символом в PowerShell
молча обрезается и расщепляет заголовок, а в bash роняет запрос.**
`-SkipHeaderValidation` добавлен в этой итерации как обход Mi-3/Mi-4 и выключает
ровно ту проверку .NET, которая запрещает CR/LF в значении заголовка. Замер,
подставной сервер печатает ВСЕ полученные заголовки:

```
MV_LLM_API_KEY = "AB\r\nX-Injected: yes\r\nCD"
  ps1 → HEADERS={'Host': …, 'Authorization': 'Bearer AB', 'X-Injected': 'yes'}
        llm: … /v1/models = 200 ok                                        exit=0
  sh  → curl: option -K: found an unknown config option (exit 2)
        llm: … unreachable — the process is not running (make llm-up)     exit=1
```

Три следствия. Первое: секрет уходит на сервер ОБРЕЗАННЫМ и молча — оператор
увидит 401 и пойдёт чинить не то, то есть ровно симптом Mi-3, который числится
закрытым. Второе: в запрос попадает произвольный дополнительный заголовок,
собранный из хвоста значения переменной, — удалённый эндпоинт получает кусок
того, что оператор считал секретом, в чужом поле. Третье: снова противоположные
коды возврата на одном входе. Модель угроз мягкая (значение приходит из `.env`
владельца, а не от внешнего лица), поэтому не Critical; но `.env` с CRLF или
ключ, склеенный из двух строк, — бытовая ситуация Windows-машины.

Как исправить: в ОБЕИХ реализациях отвергать ключ с управляющими символами одной
строкой и одним текстом (после `llm_trim`/`.Trim()` в значении не должно
остаться ничего с кодом < 0x20). `-SkipHeaderValidation` при этом можно оставить:
он нужен для кавычки, и только для неё.

**M-4. `scripts/lib/llm-endpoint.sh:336` и `LlmEndpoint.psm1:347` против
`shared/env/vars.go:108` и `shared/env/env.go:320` — «переменной нет» обвязка и
платформа понимают ПО-РАЗНОМУ, то есть для этого случая источников истины
по-прежнему два.**
Решение итерации 2 в части ПУСТОГО значения верное, и я его поддерживаю:
`shared/env` прямо говорит, что пустое значение перебивает умолчание, значит
адреса нет и у платформы, и краснеть обязаны обе стороны. В части
НЕУСТАНОВЛЕННОГО — нет. `shared/env/env.go:320` содержит явное, названное прошлым
ревью решение: «Unset and set-to-empty are therefore different things» (T-007
Mi-6), и `LLMURL.String()` при отсутствующей переменной возвращает объявленное
умолчание `http://127.0.0.1:1234`. Проверено исполнением (временный тест в
`shared/env`, после прогона удалён, дерево чистое): переменной нет — платформа
получает `http://127.0.0.1:1234`, обвязка печатает «has no value — the platform
has no LLM address» и краснеет.

Значит при удалённой из `.env` строке платформа молча пойдёт стучаться на 1234
(где на этой машине может слушать что угодно), а `make llm-health` скажет «адреса
нет». Это не мелочь формулировки, а тот же расход двух источников истины, ради
устранения которого заведена задача, — только в другом месте. Довод dev-log
«Windows не сохраняет разницу между "не установлена" и "установлена пустой" через
границу процесса» я проверил и подтверждаю как факт о платформе: он объясняет,
почему нельзя РАЗЛИЧАТЬ эти случаи в скриптах, но не объясняет, почему манифест
продолжает подставлять умолчание там, где обвязка отказывает.

Как исправить: выбрать одно и записать. Дешевле — снять умолчание у `MV_LLM_URL`
и объявить её `Required()`: тогда обе стороны краснеют одинаково, а
`.env.example` остаётся единственным местом, где адрес написан. Альтернатива —
оставить умолчание и подставлять его в обвязке при отсутствующей переменной,
приняв, что на Windows это неотличимо от пустой. Решение за оркестратором; молча
оставлять расхождение нельзя — оно и есть предмет задачи.

**M-5. `scripts/llm-server.ps1:317-324` и `scripts/llm-server.sh:313-320` —
номеру из `ops/llm-server.pid` верят без единой проверки, что это наш процесс;
`up` и `down` убивают то, чем этот номер оказался.**
M-6 итерации 1 закрыт наполовину: «по адресу уже кто-то отвечает» теперь
проверяется, «записанный номер действительно наш» — нет. Замер (изолированная
копия, посторонний живой процесс, его номер положен в PID-файл, адрес не
отвечает):

```
llm: pid <N> is recorded but http://127.0.0.1:18996 does not answer; restarting
→ Stop-Process -Id <N> -Force → посторонний процесс убит
```

Случай не умозрительный: PID-файл переживает перезагрузку, а Windows
переиспользует номера процессов агрессивно. Натурное подтверждение — происшествие
в начале раздела: именно этот код остановил живой процесс окружения владельца. В
`.sh` то же самое (`kill -9 "$pid"`) и по той же логике.

Как исправить: перед `Stop-Process`/`kill` сверить, что процесс наш, — в
PowerShell `$proc.Path` против `MV_LLM_BIN`, в bash `readlink /proc/$pid/exe`
либо `ps -p "$pid" -o comm=`; при несовпадении сказать «PID-файл описывает чужой
процесс — игнорирую», удалить файл и не убивать. Дополнительно стоит писать в
PID-файл вторую строку с путём бинарника и временем старта: тогда сверка не
зависит от того, жив ли ещё образ на диске.

#### Minor

**Mi-1. `scripts/llm-server.sh:165` и `.ps1:181` — совет `make llm-up` даётся
там, где соседняя строка уже объяснила, что запустить отсюда нечем.** Снято с
живого стенда владельца (`make llm-health` после остановки сервера):

```
llm: http://127.0.0.1:8888 unreachable — the process is not running (make llm-up)
llm: MV_LLM_BIN is not set — this runtime was not started from here, …
```

`make llm-up` в этом состоянии завершится с «MV_LLM_BIN is not set». Две соседние
строки противоречат друг другу. Как исправить: при незаданном `MV_LLM_BIN`
убирать `(make llm-up)` и говорить «поднимите рантайм тем же способом, каким он
поднимался раньше; `make llm-up` требует `MV_LLM_BIN`».

**Mi-2. `scripts/llm-server.sh:205` и `.ps1:226` — «the version of any other
runtime is the model list above» печатается, когда никакого списка выше нет.**
Список моделей выводится только при коде 200; при недостижимом эндпоинте строка
ссылается в пустоту (см. вывод в Mi-1 — это живой стенд). Как исправить: печатать
хвост про список моделей только если он был напечатан.

**Mi-3. Падение импорта общего модуля не диагностируется, а на кириллическом пути
выводится кракозябрами.** Изолированная копия без `scripts/lib`:

```
ps1  → ImportModule: …\llm-server.ps1:81  + подчёркивание исходной строки
       "…\lib\LlmEndpoint.psm1" �� �� ����㦥�, ⠪ ��� …     exit=1
sh   → llm-server.sh: line 42: …/scripts/lib/llm-endpoint.sh: No such file …  exit=1
```

Коды возврата верные (проверены все четыре скрипта: 1, 1, 1, 1) — ложного
зелёного нет, поэтому Minor. Но `CurrentUICulture='en-US'` и UTF-8 задаются
ВНУТРИ модуля, значит на единственном пути, где модуль не загрузился, оба
исправления (Mi-4, Mi-5, N-2) не действуют. Как исправить: до `Import-Module` /
до `.` проверить наличие файла и напечатать одну английскую строку с его именем
и с тем, что это единственный источник правила разбора адреса.

**Mi-4. userinfo не просто маскируется — он молча ВЫБРАСЫВАЕТСЯ, и проба идёт без
учётных данных.** `http://user:s3cr3t@127.0.0.1:8888` обе реализации проверяют
как `http://127.0.0.1:8888` без заголовка авторизации. Для эндпоинта, который
именно так и настроен, это ложный 401/красный, а `net/http` в Go при userinfo в
URL подставляет Basic-заголовок сам — то есть платформа и проба разойдутся. Как
исправить: либо отвергать userinfo явным сообщением («ключ живёт в
`MV_LLM_API_KEY`, а не в адресе»), либо использовать его как Basic-авторизацию;
любой вариант лучше молчаливого выбрасывания.

**Mi-5. `scripts/llm-bench.ps1:328-335` против `scripts/llm-bench.sh:703` —
прогревочный вызов замера: `.sh` докладывает не-200, `.ps1` молчит.** Это Mi-7
итерации 1, перенесённое в `llm-server.*` и не перенесённое в замер. Замер на
подставном сервере, отвечающем 404 на `/v1/chat/completions`:

```
bench.sh  → bench: warm-up call answered 404 (first_call_ms is still recorded)
bench.ps1 → (ни одной строки)
```

Причина та же, что была в `llm-server.ps1`: `-SkipHttpErrorCheck` не бросает
исключение, а `catch` — единственное место, где что-то печатается. Как исправить:
судить по `$response.StatusCode`, тем же текстом, что в `.sh`.

**Mi-6. `scripts/llm-bench.sh:818` и `.ps1:573` — в отчёт замера теперь пишется
адрес ПРОБЫ, а не адрес, как он настроен.** Было `url=<значение переменной без
/v1>`, стало `url=$base`, то есть `host.docker.internal` уже заменён на
`127.0.0.1`. Отчёты замера сравниваются между стендами и датами; из нового поля
не восстановить, что было настроено. Как исправить: писать оба —
`url` (как настроено, из `LLM_EP_RAW`) и `probe_url`.

**Mi-7. `scripts/llm-server.sh:409-418` против `.ps1:408-419` — ожидание старта
устроено по-разному, а сообщение об истечении в обеих одно и то же («180 s»).**
В `.sh` это 90 итераций «проба + `sleep 2`», и каждая проба может стоить до 10 с
(две точки по 5 с) — на недоступном адресе цикл живёт не 180 с, а до ~18 минут; в
`.ps1` это честный дедлайн 180 с. Как исправить: в `.sh` считать дедлайн от
`date +%s`, как в `.ps1`.

**Mi-8. `.env.example:99-105` — формулировка возвращает оператору работу, которую
владелец распорядился делать коду.** Написано: «БАЗОВЫЙ адрес, БЕЗ хвоста /v1»;
уточнение владельца (журнал 2026-09-11) прямо заменяет эту формулировку на
«принимать ОБЕ формы, какую бы ни дала документация вендора». Код делает верно
(проверено на четырёх формах), текст — нет: оператор, прочитав это, пойдёт
править адрес руками. Как исправить: «можно указывать как с хвостом `/v1`, так и
без — обвязка приведёт значение к базовому сама; проверка здоровья идёт НЕ под
`/v1`». Тот же текст стоит поправить в `Docs/ops/runbook.md` §3 (первый пункт
списка «Что именно делает правило») и в `shared/env/vars.go:104-113`.

**Mi-9. `scripts/lib/llm-endpoint.sh:196` против `LlmEndpoint.psm1:203` — на одном
входе разные сообщения.**

```
MV_LLM_URL=http://[::1]x
  sh : … looks like a bare IPv6 literal; bracket it: http://[::1]:8888
  ps1: … has trailing characters after the IPv6 literal
```

Коды возврата совпадают (1), поэтому Minor, но это прямое нарушение записанного в
шапке обоих файлов обещания про «character for character». Как исправить: в
`.sh` добавить ту же ветку «мусор после литерала», что в `.ps1`.

**Mi-10. `scripts/lib/llm-endpoint.sh:62-69` и `LlmEndpoint.psm1:78-83` —
редакция секрета вырезает подстроку без оглядки на смысл.** При
`MV_LLM_API_KEY=8888` обе реализации печатают
`llm: probing http://127.0.0.1:*** (from MV_LLM_URL=http://127.0.0.1:***)` —
диагностика становится нечитаемой ровно тогда, когда она нужна. Паритет
сохранён, поэтому Minor. Как исправить: не редактировать значения короче
разумного порога (или редактировать только по границам слов), а короткое
значение `MV_LLM_API_KEY` отдельно называть подозрительным.

**Mi-11. Значение `MV_LLM_PROVIDER` вне списка манифеста скрипты принимают
молча.** `MV_LLM_PROVIDER=zzz` + рабочий адрес: обе реализации доходят до
`200 ok` и `exit 0`, но `Startable` ложно, поэтому исчезают пин и VRAM, а `up`
скажет «не запускается отсюда» с неверной причиной. `mvctl env check --env` то же
значение отвергает (`OneOf`). Как исправить: сверять провайдера с тем же списком
и отвергать неизвестный одной строкой в обеих реализациях.

**Mi-12. `MV_LLM_URL` без порта даёт `--port 80`.** `http://127.0.0.1` — обе
реализации выводят порт по умолчанию схемы и (в `up`) отдадут `llama-server`
`--port 80`. Умолчание манифеста (`…:1234`) при этом не участвует. Поведение
одинаковое, вреда на стенде нет, но «порт не указан» и «порт 80» — разные
намерения оператора. Как исправить: требовать явный порт для локального
`openai_compat` либо сказать в сообщении, какой порт выведен и откуда.

#### Nit

**N-1.** На успешном пути `/v1/models` запрашивается дважды — один раз для
отката проверки здоровья, второй для списка моделей (видно в журнале подставного
сервера: `1×/health`, `2×/v1/models`). Можно переиспользовать тело первого
ответа.

**N-2.** В таблице замера `.sh` печатает доли (`0.0`), `.ps1` — целые (`0`);
автор это знает и указал в dev-log §8. Оставить как есть допустимо, но тогда
это стоит закрепить в T-405 как сознательное исключение, иначе следующий стенд
паритета будет ловить его каждый раз.

**N-3.** JSON-отчёты замера отличаются типами полей: `.sh` пишет `"num_ctx": "8192"`
и `"http": "200"` строками, `.ps1` — числами, и в `.sh` есть поле `index`,
которого нет в `.ps1`. Дефект унаследованный (генераторы отчёта разные), но
отчёты предназначены для сравнения между стендами.

**N-4.** `MV_LLM_URL=http://127.0.0.1:8888/%2e%2e/v1`, `…//v1`, `http://LOCALHOST.:8888`,
`http://[fe80::1%25eth0]:8888`, `http://127.0.0.1:8888@10.1.2.3:8888` разбираются
одинаково в обеих реализациях — это не замечание, а подтверждение: остальной
ручной разбор держится. Единственная его слабость — культурные сравнения (M-1).

### Ваша таблица паритета

Мой стенд, собранный заново; чужой не использовался. Действие `health`, если не
указано иное. Всего 69 входов для `llm-server.*`, 10 прогонов `up`/`down` в
изолированной копии и 4 прогона замера. «=» — вывод и код возврата совпали
посимвольно после нормализации путей, pid и миллисекунд.

| Вход | `.sh` | `.ps1` | |
|---|---|---|---|
| `http://127.0.0.1:8888` (живой сервер, до происшествия) | 200 ok по `/v1/models`, 22 модели, 0 | то же, те же 22 | = |
| `HTTP://127.0.0.1:8888` (схема в верхнем регистре) | 200 ok, 0 | 200 ok, 0 | = |
| `http://127.0.0.1:8888/v1` | срез хвоста, `/health` + `/v1/models`, 0 | то же | = |
| `http://127.0.0.1:8888/v1/` | то же, 0 | то же | = |
| `http://127.0.0.1:8888//v1` | то же, 0 | то же | = |
| `http://127.0.0.1:8888/V1` (регистр) | не срезает, 1 | не срезает, 1 | = |
| `http://host.docker.internal:8888` (живой) | проба 127.0.0.1, 200, 0 | то же | = |
| `http://127.0.0.1:18991/api/openai` (несколько сегментов) | `/api/openai/health`, `/api/openai/v1/models`, 0 | то же | = |
| `http://127.0.0.1:18991/v1/proxy` (`/v1` в СЕРЕДИНЕ) | не срезает: `/v1/proxy/health`, `/v1/proxy/v1/models`, 0 | то же | = |
| пусто | «has no value …», 1 | то же, 1 | = (C-1) |
| переменной нет | «has no value …», 1 | то же, 1 | = по выводу, но ≠ платформе (M-4) |
| одни пробелы | «has no value …», 1 | то же, 1 | = |
| `127.0.0.1:8888` (без схемы) | «has no scheme …», 1 | то же, 1 | = (Mi-11) |
| `ftp://127.0.0.1:8888` | «uses scheme 'ftp' …», 1 | то же, 1 | = |
| `http://user:s3cr3t@127.0.0.1:8888` (живой) | `http://***@…`, 200, 0 | то же, 0 | = (M-4 закрыт; Mi-4) |
| `http://@127.0.0.1:8888` | 200, 0 | 200, 0 | = |
| `http://127.0.0.1:8888@10.1.2.3:8888` | хост `10.1.2.3`, 1 | то же, 1 | = |
| `http://[::1]:8888` | «our process», без облачного WARNING, 1 | то же, 1 | = (M-3 закрыт) |
| `http://[::1]` | 1 | 1 | = |
| `http://::1:8888` (без скобок) | «bare IPv6 literal …», 1 | то же, 1 | = |
| `http://[::1]x` | «bare IPv6 literal …», 1 | «trailing characters …», 1 | **Mi-9** |
| `http://[fe80::1%25eth0]:8888` | 1 | 1 | = |
| `http://127.0.0.1` (порт по умолчанию) | проба без порта, `--port 80`, 1 | то же, 1 | = (Mi-12) |
| `http://127.0.0.1:08888` (ведущий ноль) | 200, 0 | 200, 0 | = |
| `http://127.0.0.1:0888` | 1 | 1 | = |
| `http://127.0.0.1:0` / `:99999` / `:abc` | 1 | 1 | = |
| `http:///v1` / `http://:8888` | «has no host», 1 | то же, 1 | = |
| `http://127.0.0.1:8888/?x=1` | «query or a fragment», 1 | то же, 1 | = |
| `http://127.0.0.1:8888/` + путь 3000 символов | 1 | 1 | = |
| `http://LOCALHOST.:8888` | 1 | 1 | = |
| `http://http://127.0.0.1:8888` | 1 | 1 | = |
| `http://127.0.0.1:8888/%2e%2e/v1` | 1 | 1 | = |
| хвост `U+00A0` (NBSP) | обрезан, 200, 0 | обрезан, 200, 0 | = |
| `U+00A0` в середине пути | 1 | 1 | = |
| **`…/v1` + `U+00AD`** | `/v1<SHY>/health` = 404, **1** | срез `/v1`, 200 ok, **0** | **M-1** |
| **`…/v1` + `U+200B`** | 404, **1** | 200 ok, **0** | **M-1** |
| **`http<U+00AD>://…`** | «uses scheme 'http<SHY>'», 1 | проба по битой строке, «process is not running», 1 | **M-1** |
| **`https://127.0.0.1:18991` (TLS к HTTP-порту)** | «not running», без строки про сбой запроса | «not running» + «the request itself failed …» | **M-2** |
| **`http://a[b:8888`** | без строки про сбой запроса | со строкой | **M-2** |
| `MV_LLM_PROVIDER=` (пусто) | как `openai_compat`, 0 | то же | = (Mi-10) |
| `MV_LLM_PROVIDER=zzz` | 200 ok, 0, без пина и VRAM | то же | = (Mi-11) |
| `anthropic` / `recorded` / `fake` | «не отсюда», 0 | то же, 0 | = |
| `ollama` + `MV_OLLAMA_URL` | проба по `MV_OLLAMA_URL`, 0 | то же | = |
| `ollama` без `MV_OLLAMA_URL` | одна строка, 1 | та же строка, 1 | = |
| `MV_LLM_PORT=1234` при живом 8888 | строка про отменённую переменную, 200, 0 | то же | = (Mi-1) |
| `http://10.1.2.3:8888` (RFC1918) | «не отсюда», гейт молчит, 1 | то же | = |
| `http://localtest.me:18991`, гейт закрыт | 200 + WARNING + «reported as a failure», **1** | то же, **1** | = (решение оркестратора п. 3) |
| то же, `MV_LLM_CLOUD_ENABLED=true` | 200, 0 | 200, 0 | = |
| `HTTP_PROXY`/`http_proxy` на мёртвый прокси | 200 ok, 0 | 200 ok, 0 | = (M-2 итерации 1) |
| ключ: пробел / `"` / `\` / `#` / `=;` / 8000 символов | доходит дословно | доходит дословно | = (Mi-3) |
| **ключ с `\r\n`** | curl не построил запрос → «not running», **1** | `Bearer AB` + чужой заголовок `X-Injected`, **0** | **M-3** |
| `MV_LLM_API_KEY=8888` (вырожденный) | `http://127.0.0.1:***` | то же | = (Mi-10) |
| `up`: `MV_LLM_HOST=0.0.0.0` | отказ, 1, бинарник не запущен | то же | = (Mi-12, SEC-15) |
| `up`: `MV_LLM_HOST=192.168.1.5` | отказ, 1 | то же | = |
| `up`: адрес занят чужим сервером, PID-файла нет | отказ, 1 | то же | = (M-6) |
| `up`: свободный адрес, полный старт | argv, 200, «first call N ms», 0 | argv совпал посимвольно | = (N-1) |
| `up`: `MV_LLM_NUM_CTX=` / `MV_LLM_SLOTS=` | `--ctx-size 8192 --parallel 1` | то же | = (Mi-9) |
| `up`: PID-файл с чужим ЖИВЫМ номером | `kill -9` чужого | `Stop-Process -Force` чужого | = по паритету, **M-5** по сути |
| `down`: PID-файла нет | «nothing to stop», 0 | то же | = |
| `down`: `not-a-pid` | две строки, 0, файл удалён | то же | = (Mi-8) |
| `down`: устаревший `999999` | 0, файл удалён | то же | = |
| `down`: облачный адрес | «не отсюда», 0 | то же | = |
| `down`: пустой адрес | ошибка, 1 | то же, 1 | = |
| замер: `http://host.docker.internal:18993/v1` (все три ямы) | дошёл до конца, CSV + отчёт | то же | = (M-1 итерации 1 закрыт) |
| **замер: прогрев отвечает 404** | «warm-up call answered 404» | молчит | **Mi-5** |
| замер: поля отчёта | `"8192"`, `"200"`, есть `index` | `8192`, `200`, нет `index` | N-3 |

Расхождений: **6 по существу** (M-1 в трёх формах, M-2 в двух, M-3, Mi-9, Mi-5
замера) на 83 сравнения. Отдельно отмечу вход, который расхождением НЕ является:
`MV_LLM_URL=http://127.0.0.1:8888\v1` даёт разные вердикты, но потому, что
обратная косая превращается в прямую на границе bash → pwsh (проверено: PowerShell
получает уже `/v1`, символ 47). Это свойство окружения, а не кода; в стенд T-405
такой вход брать нельзя — он будет мигать.

### Что проверено экспериментом

Все прогоны мои, на машине владельца. `.env` не открывался; всё, что требовало
его значений, шло через `make`.

1. **Правило вывода адреса действительно одно.** `grep` по четырём скриптам:
   собственного разбора `MV_LLM_URL`/`MV_OLLAMA_URL`, подмены
   `host.docker.internal` и среза `/v1` в них не осталось; остались только
   дописывания `/v1/models` и `/v1/chat/completions` к базе, которую вернул
   модуль. Заявление автора подтверждено.
2. **Требование владельца по `/v1` — на подставном сервере, который ЗАПИСЫВАЕТ
   полученные пути.** Формы `…:18991`, `…:18991/v1`, `…:18991/v1/`,
   `…:18991//v1` дают ровно `GET /health` и `GET /v1/models`; `/v1/v1/…` не
   появился ни разу. Проверка здоровья идёт по базовому адресу, а не под `/v1`, —
   именно то, что записано в журнале как «техническая тонкость». Форма с `/v1` в
   СЕРЕДИНЕ (`…/v1/proxy`) не трогается: `/v1/proxy/health`, `/v1/proxy/v1/models`.
   На живом сервере (до происшествия) три формы дали одинаковые 200 и одинаковый
   список из 22 моделей.
3. **C-1 закрыт.** Пусто, «не установлена», «одни пробелы» — один текст и код 1 в
   обеих реализациях. `make llm-health` = 1; строгий `make health` = 2 (LLM
   красная, `gateway/memory/core ok`); `make health LLM_STRICT=0` = 0 с
   предупреждением. Зелёного состояния строгого `health` в этой итерации показать
   не могу — см. происшествие.
4. **Ключ не утекает.** Проба против «чёрной дыры» `http://10.255.255.1:8888` с
   `MV_LLM_API_KEY=TOPSECRETKEY42`, одновременный снимок командных строк ВСЕХ
   процессов (`Get-CimInstance Win32_Process`): у `curl.exe` командная строка
   `-s --noproxy * -K - -o nul -w %{http_code} --max-time 5 http://…/health` —
   ключа нет; в выводе обеих реализаций ключа нет; в отчётах и CSV замера
   (`grep -rl` по каталогу отчётов) ключа нет; в `llm-bench.sh` аргумент
   `-H "Authorization: …"` действительно исчез. Проверены все четыре скрипта.
   Единственный путь порчи, который остался, — управляющий символ (M-3).
5. **M-6 закрыт и не даёт ложных срабатываний.** Отказ при занятом адресе
   срабатывает только когда PID-файла нет И адрес отвечает; при живом PID-файле и
   отвечающем адресе — «already running … nothing to do»; при живом PID-файле и
   молчащем адресе — перезапуск. Всё в изолированной копии.
6. **SEC-15 не ломает документированный сценарий контейнера.** Проверено прямо:
   заглушка, слушающая ТОЛЬКО `127.0.0.1:18991`, доступна из работающего
   контейнера стека по имени `host.docker.internal` (`docker exec … /multiverse
   health --url http://host.docker.internal:18991/v1/models` — заглушка увидела
   запрос). То есть требование «только loopback» и обращение из контейнеров на
   этой машине совместимы. Контейнеры не перезапускались, `docker exec` — только
   чтение.
7. **Замер больше не врёт.** `llm-bench.sh` и `llm-bench.ps1` на входе
   `http://host.docker.internal:18993/v1` (алиас + хвост `/v1` + сервер без
   `/health`) проходят шлюз, прогоняют все три фазы и пишут CSV и JSON; шлюз
   `no answer … the LLM process is not running` не срабатывает. `make bench` не
   запускался, GPU не занимался, отчёты писались вне репозитория.
8. **Не сломано то, что работало.** `go build ./...` — 0; `go vet ./...` — 0;
   `go test -short ./...` — 0, ни одного FAIL; `golangci-lint run ./...` —
   `0 issues`; `gofmt -l ./cmd ./shared ./internal` — пусто; `mvctl env check` —
   `68 variables declared`, код 0; `MV_LLM_PORT=1234 mvctl env check --env` —
   `[deprecated] MV_LLM_PORT: retired and still set …`, код 1. Рабочее дерево
   после ревью не изменено (`git status` совпадает с состоянием до).

### Чего проверка не покрывает

- Зелёный строгий `make health` и повторная проверка списка моделей на живом
  сервере — сервер остановлен по моей ошибке (см. начало раздела).
- `.sh` прогонялся под Git Bash на Windows; `nohup`, сигналы и ветка `up` на
  настоящем Linux-стенде не проверялись. Отдельно не проверено, достижим ли
  loopback-сервер из контейнера на Linux-стенде, где `host.docker.internal`
  разрешается через `host-gateway`: на Docker Desktop (эта машина) достижим,
  на Linux это другой механизм. Если ответ «нет», отказ SEC-15 и обращение из
  контейнера на Linux несовместимы — вопрос к архитектору, не к этой задаче.
- Настоящий облачный эндпоинт не опрашивался: гейт проверен именем
  `localtest.me`, которое публично разрешается в `127.0.0.1`, поэтому
  классификация считает адрес нелокальным, а отвечает подставной сервер.
- `make bench` на живом сервере не запускался (GPU владельца).
- Долгий путь ожидания старта (`Mi-7`) выведен из кода, а не измерен: ждать
  18 минут ради подтверждения я не стал.

### Ответ на вопрос автора об отходе (пустое и неустановленное — одинаково)

Согласен наполовину, и это Major M-4. Пустое значение — ошибка: так говорит
`shared/env` («an empty value overrides the default»), значит адреса нет и у
платформы, и обе стороны обязаны краснеть одинаково. Неустановленное — не то же
самое: `shared/env/env.go:320` содержит явное решение прошлого ревью, что
«unset» и «set to empty» различаются, и `LLMURL.String()` при отсутствующей
переменной отдаёт `http://127.0.0.1:1234`. Довод про Windows верен как факт (я
его проверил), но он обосновывает лишь невозможность РАЗЛИЧИТЬ эти случаи в
скриптах; он не обосновывает, почему манифест продолжает подставлять умолчание
там, где обвязка отказывает. Пока это так, для случая «строки в `.env` нет»
источников истины два, а задача называется «адрес имеет один источник».
Умолчание манифеста при этом не «ломается» — оно просто перестаёт быть
достижимым через обвязку, и честнее его снять (`Required()`), чем оставить
недостижимым.

---

## T-400 · ревью #1 · 2026-09-11 · code-reviewer#2

### Границы ревью

`shared/testkit/gateway/{harness.go,scenario.go,combat_test.go,harness_test.go}` — изменения
из индекса. Задача ревьюировалась **вместе с T-219** (`shared/testkit/swarm`): по указанию
оркестратора это один боевой путь, и порознь он не проверяется. Замечания по заглушке боя —
в разделе «T-219 · ревью #1» файла `epics/EPIC-003-swarm-llm-laws/review.md`.

Читалось как данность и не переоткрывалось: решения оркестратора от 2026-09-11 —
«одно действие — ровно одно атомарное предложение» (вариант «б»), «правило линтера вместо
директивы подавления», «уровень агента при создании встречи правит документ». Контракты
C-02 v1.3, C-03 v1.2, C-04 v1.2, C-05 v1.2, `state-and-mechanics.md` v0.3, `internal/mechanics/**`,
`shared/testkit/{state,mechanics}/**` — чтением.

**Метод: боевой путь собран целиком и прогнан.** В дереве нет ни одного теста, который сводит
харнесс T-400 с заглушкой T-219: `combat_test.go` проверяет харнесс против собственной подставной
встречи, тесты `shared/testkit/swarm` публикуют действия игрока прямо в шину, а `test/e2e`
гоняет `solo-visit`/`party-visit` **без боя**. Поэтому был собран временный зонд
(`membus` + `state.FakeState` + `gateway.Harness` + `swarm.FakeEncounter` на одной шине),
прогнан 60 раз подряд и удалён; дерево сверено (`git diff` по `shared/`, `internal/`, `test/`,
`cmd/`, `.golangci.yml` — пусто).

### Вердикт

**ВЕРНУТЬ** — Critical 0, Major 3, Minor 2, Nit 1.

Собственный код харнесса написан правильно и по решению оркестратора: ожидание факта, а не
решения боя, слот, открытый до публикации, разность множеств вместо порядка прихода — всё
это выдержало мутации и прогон. Возвращаю не за это.

Возвращаю за то, что **поставленный задачей сценарий `solo-skirmish` не проходит против
единственного, кто в дереве разрешает бой**: 25 прогонов из 60 падают по таймауту, на всех трёх
боевых шагах. Причина обрыва — в T-219, но следствие принадлежит и T-400: задача отдала скрипт,
который против своего же контрагента не работает, и её тесты этого не видят, потому что подставная
встреча в `combat_test.go` никогда не промахивается и никогда не заканчивает встречу.
Плюс два места, где свойство, на котором держится вся гарантия задачи, ничем не держится:
непрочитанный пакет даёт молчаливый успех, а мутация «возвращаться по первому факту» проходит
набор зелёным.

### Замечания по серьёзности

#### Critical

Нет. Обрыв боевого пути числится Critical за T-219: не публикует заглушка.

#### Major

**Ma-1. `scenario.go:135-148` — `ScenarioSkirmish` не имеет ветки «встреча закончилась», и против
`FakeEncounter` падает в 42 % прогонов.** Скрипт — это `attack, attack, flee` подряд. Встреча же
закрывается, как только волк или игрок упал, а после `encounter.ended` заглушка на действия не
отвечает **ничем**: `fightOf` возвращает «не в бою» и выходит с `nil`. Харнессу нечего ждать, и он
ждёт весь таймаут.

Прогон зонда, 60 запусков `solo-skirmish` на полном пути:

```
runs=60 ok=35
timeout: step 4 (attack player-A)  x8
timeout: step 5 (attack player-A)  x9
timeout: step 6 (flee  player-A)   x8
```

Прицельно и детерминированно (волк посажен на 1 hp, первый же удар его валит):

```
encounter.ended reason=npc_dead
attack after encounter.ended -> nobody resolved the attack of player-A on wolf-alpha within 2s (waited 2s)
flee   after encounter.ended -> nobody resolved the flight of player-A within 2s  (waited 2.001s)
```

И второй край — упал игрок (игрок на 1 hp, волк на 40):

```
encounter.ended reason=players_out after swing 1
State says player-A is dead
say   -> <nil>
rest  -> proposal gw-rest-player-A-3 refused: dead_entity
leave -> proposal gw-move-player-A-4 refused: dead_entity
```

То есть даже после починки T-219 скрипт остаётся неисполнимым: за боем в нём идут `rest` и `leave`,
а State их у трупа отвергает.

Как исправить. Таблица шагов здесь не поможет — сколько ходов переживёт волк, решает таблица
механики, и это записано в собственном отклонении 1 задачи. Нужен явный признак у шага либо у
харнесса: «шаг допустим, только пока встреча жива». Минимально: `Harness` подписывается на
`world_events` и помнит `encounter.ended` по scope; `Attack`/`Flee` вне живой встречи отказывают
**мгновенно и по причине** («the fight of player-A is over: encounter.ended reason=npc_dead»),
а не по таймауту; `Scenario` для `solo-skirmish` пропускает оставшиеся боевые шаги и не ведёт
мёртвого персонажа дальше по скрипту. Это ровно та экономия таймаута, которой уже обоснованы
проверки в `opponent` и в `Attack` — здесь её не хватает на один случай.

**Ma-2. `harness.go:150-166` (`reaction.done`) — предложение, из которого харнесс не смог прочитать
ни одной сущности, засчитывается как «мир пришёл в покой»: действие возвращает успех, не увидев ни
одного факта.** `proposedChange` всегда возвращает непустую карту (`make(...)`), поэтому
`named != nil` означает лишь «предложение увидено». Дальше цикл `for id := range r.named` по пустой
карте не делает ни одного витка, и `done()` = `true`.

Гарантия задачи ломается в сторону молчаливого «зелено», хотя doc-комментарий `awaitReaction`
обещает обратное: «An encounter that breaks it makes every run of the scenario fail here, with the
message below — never one run in ten». Зонд (пакет отдан в `Observe` напрямую, публикации нет):

```
Attack returned err=<nil> after a package that named nobody: no fact of State was ever seen
```

Сегодня это недостижимо через шину — схема C-02 требует `changes[].entity.entity.id`, — но
именно поэтому и стоит закрыть сейчас: путь чтения (`changes[i].entity.entity.id`) харнесс
выбирает сам, ревизия C-02 его сместит, и отказ станет молчаливым успехом, а не падением.

Как исправить: в `expect` не заполнять слот пустым набором, а записать отказ —
`if len(named) == 0 { r.refusal = "the proposal " + proposalID + " named no entity the harness
can read" }`. Тогда та же ветка `done()` → `refusal` вернёт ошибку с текстом.

**Ma-3. Свойство «ждать факт по КАЖДОЙ названной сущности» не держится ни одним тестом.**
Автор сам называет его главным (запись T-400 в `dev-log.md`, §1, «Почему не „первый попавшийся
факт“»), но мутации 1 и 2 из его таблицы бьют по другим точкам: «не ждать вовсе» и «вернуться,
как только предложение увидено». Мутация ровно в названную точку не ставилась. Поставил:

```go
// reaction.done(): вместо обхода r.named
return len(r.answered) > 0
```

Результат: `go test ./shared/testkit/gateway/` — **зелено**; `go test -tags e2e ./test/e2e/` —
**зелено**. Мутация ловится только повторами: при `-count=25` набор краснеет дважды
(`combat_test.go:89: the harness thinks wolf-alpha is at version 0, State says 2`), при
`-count=10` сквозного — не краснеет вовсе. То есть страж свойства сам является гонкой: он падает,
только когда второй факт случайно опоздал.

Как исправить: сделать проверку детерминированной, а не вероятностной. Подставная встреча
в `combat_test.go` уже называет двоих; достаточно дать ей режим, в котором второй факт заведомо
приходит позже (`state.FakeState` применяет пакет одним куском, поэтому задержку надо ставить на
стороне подставной встречи), и утверждать после возврата `Attack`: у ОБЕИХ названных сущностей
версия харнесса равна версии State. Сейчас `TestAttackPublishesTheActionAndWaitsForTheChange` это
утверждает, но выигрывает гонку примерно в девяти случаях из десяти.

#### Minor

**Mi-1. `harness.go:308-315` — второй пакет на одно действие проглатывается молча.** Ветка
`r.named != nil` выходит без единой записи в лог. Решение оркестратора («до правки контракта
харнесс вправе возвращаться по первому») этот выбор разрешает, но не требует делать его невидимым.
Проверено зондом: резолвер, отвечающий на удар двумя законными пакетами, даёт

```
Attack returned. State still has player-A at hp=10
(the second package of the SAME action has not been applied yet)
```

— действие отчитывается о разрешении, пока половина изменения ещё не опубликована, и в журнале об
этом ни строки. Как исправить: `h.log.Warn("a second proposal for one action was ignored",
"proposal_id", …, "correlation_id", …)` — одна строка, и будущая ревизия C-04/C-05 перестанет быть
тихой.

**Mi-2. `harness.go:83` — `DefaultTimeout = 5s` умножается на число боевых шагов.** Скрипт
`solo-skirmish` при обрыве пути стоит 15 с настенного времени за прогон; зонд на 60 прогонов занял
125 с почти целиком из-за этого. Для CI это заметно. Умолчание менять не надо (для настоящего State
5 с осмысленны) — достаточно записать в doc-комментарии `WithTimeout` рекомендацию для сквозных
наборов и, после починки Ma-1, отказывать мгновенно там, где ответа заведомо не будет.

#### Nit

**N-1. `harness.go:466, 501` — `Attack` требует, чтобы харнесс уже слышал о персонаже
(`h.Version(playerID)`), но о цели не требует ничего.** Логика верная (о NPC харнесс версий не
ведёт), однако комментарий `opponent` объясняет только «почему не игрок», и следующий читатель
будет искать симметричную проверку. Одна строка в комментарии.

### Что проверено экспериментом

| Что | Как | Итог |
|---|---|---|
| Боевой путь целиком | зонд `membus`+`FakeState`+`Harness`+`FakeEncounter`, 60 прогонов `solo-skirmish` | **35 ok / 25 таймаутов**, падают все три боевых шага |
| Действие после конца встречи | волк на 1 hp, удар после `encounter.ended reason=npc_dead` | таймаут у `Attack` и у `Flee` |
| Игрок погиб в бою | игрок 1 hp, волк 40 hp | `rest` и `leave` отвергнуты с `dead_entity` |
| Порядок прихода фактов | мутация в `changes.sets()` — обратный порядок наборов | **харнесс не заметил**: `gateway` и `swarm` зелёные. Свойство «порядок не важен» подтверждено |
| Два пакета на одно действие | подставной резолвер, два законных пакета | харнесс возвращается по первому, молча (Mi-1) |
| Предложение отклонено | `TestARefusedChangeOfTheFightStopsTheAction`, режим `encounterStale` | ошибка называет `version_conflict` — сильное утверждение, не «ошибка есть» |
| Возврат по первому факту | мутация `done()` → `len(answered) > 0` | `-count=1` зелено; `-count=25` красное дважды (Ma-3) |
| Пакет, не назвавший никого | `Observe` напрямую | `Attack` вернул `nil` без единого факта (Ma-2) |
| Слабые утверждения | вычитка `combat_test.go` | **не найдено**: `TestAFightNeedsSomebodyToFight` и `TestAnActionNobodyResolvesGivesUp` проверяют текст причины, а не наличие ошибки. Замечание автора о собственной слабой проверке закрыто по делу |
| Сборка, `vet`, короткие тесты | `go build ./...`, `go vet`, `go test -short -count=1 ./...` | зелено |
| Сквозные тесты | `go test -tags e2e -count=1 ./test/...` | зелено (боя в них нет) |
| Линтер | `golangci-lint run ./...` | 0 issues |
| Контракты | `go run ./cmd/mvctl contracts check` | 65 типов, 8 топиков, 58 схем — ок |
| Детектор гонок | `go test -race` | **не запускался**: нет gcc, cgo недоступен. То же ограничение, что в T-018/T-401 |

### Что осталось непроверенным

- `-race` по боевому пути. Это предмет T-401 и долг владельца, а не исполнителя.
- Поведение против настоящего `mechanics.Rules.Resolve` (T-053) — его ещё нет; всё мерялось на
  `FixedMechanics` (60 % попадание, 10 % крит, 10 % провал, 20 % промах).
- Настоящий шлюз по HTTP: харнесс ждёт собственного факта, чего шлюзу нельзя. Записано автором
  как известное ограничение v0 — согласен, не блокирует.

### Предложения в бэклог

1. Тест-«часовой» на пару задач: один сквозной набор, где харнесс T-400 и заглушка T-219 стоят на
   одной шине. Сегодня их нет ни в одном пакете, и обе задачи были зелёными врозь.
2. Детерминированный «медленный State» в `testkit` — подставной применитель, публикующий факты
   пакета с управляемой задержкой. Без него свойства вида «дождался всех» проверяются гонкой.

---

## T-400 · ревью #2 (итерации 2 и 3) · 2026-09-11 · code-reviewer#2

### Границы ревью

`shared/testkit/gateway/{harness.go,scenario.go,combat_test.go,harness_test.go,reaction_test.go,stand_test.go}`
в коммите `3c628cb`. Ревьюировалось **вместе с T-219** (раздел «T-219 · ревью #2» в
`epics/EPIC-003-swarm-llm-laws/review.md`). Повторное ревью: проверены исправления по ревью #1,
повтор при конфликте версий (итерация 3) и регрессия от них.

Как данность, не переоткрывалось: решения оркестратора от 2026-09-11 — отказ на стороне харнесса,
адаптивный сценарий, «одно действие — один пакет», повтор при конфликте версий, «незакрытая
мутация стенда остаётся страховкой, второго двойника боя не строить».

**Метод.** Стенд — 120 своих прогонов (1920 боёв). Плюс два временных зонда: в `shared/testkit/swarm`
(S1…S5) и в `shared/testkit/gateway` (G1, G2, G3 и G5 — инварианты на 160 боях). Зонды удалены,
дерево сверено (`git diff HEAD -- shared internal test cmd schemas .golangci.yml` — пусто,
`zz`-файлов нет).

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 2, Nit 0.

Все три Major ревью #1 закрыты, и боевой путь сходится: 0 красных на 130 прогонах против 25 из 60
в ревью #1. Разделение «гонка — дефект» сделано верно: конфликт версий продлевает ожидание, четыре
остальные причины кончают его сразу, срок не продлевается. Против настоящих `FakeState` и
`FakeEncounter` сдавшийся издатель ловится сроком с текстом, отличным от «никто не ответил».
Оба новых замечания — про текст и срок на краях, которых сценарии дерева не касаются.

### Стенд: мои прогоны

| Серия | Прогонов (боёв) | Красных |
|---|---|---|
| `go test -count=60 -run TestTheHarnessAndTheEncounterOfTheSwarmOnOneBus`, `GOMAXPROCS` по умолчанию | 60 (960) | **0** |
| то же, `-cpu=1,2,4 -count=20` | 60 (960) | **0** |
| зонд G5 (сценарий `solo-skirmish` + инварианты), `-count=10` | 10 (160) | **0** |

G5 проверял каждый прогон:
- у каждого `player.attacked`/`player.flee_attempted` ровно один `proposal_id` среди предложений
  `testkit/swarm`, и он применён (есть факт);
- `combat.decided` на корреляцию — не больше двух;
- позиция персонажа у харнесса совпадает с позицией у State.

Нарушений 0. Гонка встретилась 1 раз на 160 боёв (`prop-zz5-10-11`: предложен 2 раза, 1 конфликт,
применён) — повтор её пережил.

### Статус замечаний ревью #1

| Замечание | Статус | Доказательство |
|---|---|---|
| Ma-1 (сценарий без ветки «встреча закончилась») | **закрыто** | `stillFighting` отказывает сразу после `encounter.ended` и над трупом; шаги `When`/`Turns`; 130 прогонов без таймаута |
| Ma-2 (пакет без сущностей — молчаливый успех) | **закрыто** | `expect` ставит отказ «named no entity»; `TestAChangeThatNamesNobodyIsNotAnAnswer` читает текст |
| Ma-3 (свойство «факт по каждой сущности» без стража) | **закрыто** | `reaction_test.go` без State: ответчик публикует ровно запрошенные факты, таймингов нет. Мутацию автора поштучно не перепроверял — детерминированность видна по построению |
| Mi-1 (второй пакет молча) | **закрыто** | `Warn` «a second proposal for one action was ignored»; тест читает журнал |
| Mi-2 (таймаут × число шагов) | **закрыто** | абзац в `WithTimeout`; удар после конца встречи больше не стоит срока |
| N-1 (`opponent` без симметричной проверки) | **закрыто** | абзац в комментарии `opponent` |

### Повтор при конфликте — что проверено

| Свойство | Как | Итог |
|---|---|---|
| конфликт продлевает ожидание, 4 остальные причины кончают сразу | `TestAPackageRefusedForAVersionConflictIsWaitedThroughItsRetry`, `TestARefusalThatIsNotARaceEndsTheWaitAtOnce` (текст причины, а не «ошибка есть») — прочитано, прогнано | держится |
| повтор под тем же `proposal_id` — не второй пакет; ожидаемый состав — из последнего предложения | `expect` (`again`); G5 — один `proposal_id` на действие | держится |
| срок не продлевается | `awaitReaction`: срок заводится один раз до цикла (`harness.go:990-992`), ветка конфликта никого не будит | держится |
| сдавшийся издатель ловится сроком, текст отличен от «никто не ответил» | **G3**: настоящие `FakeState` и `FakeEncounter`; у заглушки версия волка навсегда неверна (посеяна 7) | 3 отказа, 3 предложения, конец за **2,001 с** при сроке 2 с: «…lost the version race and never came back as a fact: this is a publisher that ran out of retries and gave up, not silence on the bus (version conflicts: 3, entities with a fact: 0 of 2)» |
| дублирующая доставка отказа на стороне харнесса | чтение `record` | безвредна: `conflicts` растёт, ожидание не кончается и никого не будит |

### Новые замечания

#### Minor

**Mi-1 (новое). `harness.go:546-557` (`record`) — конфликт засчитывается действию без сверки
`proposal_id`; итог — неверный диагноз на сроке.** Зонд G2: на корреляцию действия приходит только
отказ `version_conflict` чужого пакета `foreign-1`, своего пакета нет вовсе:

```
nobody resolved the attack of player-A on wolf-alpha within 200ms: the change proposed for it ()
lost the version race and never came back as a fact: this is a publisher that ran out of retries
and gave up, not silence on the bus (version conflicts: 1, entities with a fact: 0 of 0)
```

Текст винит издателя в исчерпании повторов по пакету с пустым идентификатором, хотя на действие
не ответил никто. Это та самая путаница «читатель ищет не там», от которой `progress` и должен
защищать. Автор назвал это допущением (§10 записи итерации 3), но оно меняет не только ожидание,
а ещё и диагноз. Как исправить: считать конфликт только при
`r.named != nil && proposalID == r.proposalID`; иначе — строка `Info` «конфликт чужого пакета под
корреляцией действия», `conflicts` не трогать. Тест — случай G2 с ожиданием «none arrived».

**Mi-2 (новое). `harness.go:893-904` (`stillFighting`) — «действие без активной встречи отказывает
немедленно» верно только для встречи, о конце которой харнесс знает. Персонаж, который ни в какую
встречу не входил, ждёт полный срок.** Зонд G1 на стенде (настоящие `FakeState` и `FakeEncounter`),
`CreatePlayer`, затем удар и бегство без `Enter`:

```
attack with no encounter: after 2.001s: ... within 2s: an encounter answers every action it takes
  with one proposal of change, and none arrived
flee   with no encounter: after 2.001s: ... none arrived
```

Принцип «незнание — не факт» верен для `encounter.started`: тот идёт по другому топику. Но есть знание,
которое ни с каким топиком не пересекается, — собственная карта `positions`. Персонаж, которого
харнесс сам держит в `Outside(world)`, в бою быть не может. Как исправить: в `stillFighting`
отказывать сразу, если `positionOf(player) == Outside(h.worldID)`, с текстом «…is outside every
region, and no encounter answers it». Либо, если это сознательно, сузить формулировку карточки и
doc-комментария `awaitReaction` до «после конца встречи». Не блокирует: сценарии дерева так не
пишут, а ошибка есть и называет срок.

### Что проверено экспериментом

| Что | Как | Итог |
|---|---|---|
| Боевой путь | стенд, 120 прогонов при разном `GOMAXPROCS` | **0 красных** |
| Одно действие — один ответ | G5, 160 боёв | 0 нарушений, 1 гонка пережита |
| Сдавшийся издатель | G3, сквозной | ловится сроком, текст верный |
| Чужой конфликт под корреляцией действия | G2 | неверный диагноз (Mi-1) |
| Действие без встречи вообще | G1 | полный срок (Mi-2) |
| Сборка, `vet`, короткие тесты | `go build ./...`, `go vet ./...`, `go test -short -count=1 ./...` | зелено, все пакеты `ok` |
| Сквозные тесты | `go test -tags e2e -count=1 ./test/...` | `ok` (боя в них по-прежнему нет — его роль играет стенд) |
| Линтер | `golangci-lint run ./...` (2.13.2) | 0 issues |
| Контракты | `go run ./cmd/mvctl contracts check` | 65 типов, 8 топиков, 58 схем |
| Покрытие | `go test -cover` | `shared/testkit/gateway` **92,0 %** |
| Детектор гонок | `-race` | **не запускался**: нет cgo (T-401) |

### Что не перепроверялось

- Матрица «23 / 31 / 33 / 45 из 45» — историческая: для неё нужен код до правок. Не воспроизводилась,
  её вывод подтверждён текущим замером (0 красных).
- Мутации автора поштучно. Вместо них — свои зонды в стык.
- Незакрытая мутация `standRefusals` («конфликт обязан вернуться фактом») принята решением
  оркестратора. По чтению подтверждаю, что единственный издатель стенда повторяет всегда, поэтому
  случая «конфликт без возврата» на стенде нет. Свойство держат модульные тесты харнесса и G3.

### Предложения в бэклог

1. C-05 (T-409): вместе с правилом повтора записать, что конфликт относится к **пакету** по
   `proposal_id`, а не к действию по корреляции. Одна строка, и Mi-1 не повторится у настоящего шлюза.
2. Язык сценариев `Run`/`Step.When/Turns` — в `design.md` §5 (открытый вопрос автора, architect#1).

## T-411 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Изменения из индекса: `docker-compose.yml` (три строки списков клиентов, шапка — правило 8),
`.env.example` (+7 строк комментария), `scripts/compose-lint.sh` (правило 8, расширение правила 3,
справка), `Makefile` (строка `make help`), `testdata/compose-lint/` (четыре фикстуры, README).
Контекст — карточка T-411, запись T-411 в `dev-log.md`, две последние записи `journal.md`,
`shared/env/env.go:313-353`, `shared/env/vars.go`, `shared/runtime/http.go` (`AdminOnly`).
Решения оркестратора по открытым вопросам T-411 — 26 совпадающих умолчаний остаются, адреса сети
и `OLLAMA_*` уходят архитектору, заведена T-412 — приняты как данность и не переоткрывались.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 3, Nit 3.

Главное заявление подтверждено настоящим запуском контейнера, а не только `config`. Ключ без
значения при молчащем `.env` не приходит процессу, и Go-процесс получает умолчание манифеста.
Пустая строка даёт «задано пустым», то есть «никого», как и задумано в `env.go:317-321`. Правило 8
на композиции из HEAD даёт ровно три нарушения — эти три строки. Каждая новая фикстура отвергается
своим правилом: это проверено мутацией правила. Все обязательные прогоны зелёные.

Все три Minor — дыры в охране, дефекта в текущей композиции нет. Mi-1 и Mi-2 советую закрыть в этой
же задаче: это около десяти строк линтера и две фикстуры. Возврата они не требуют.

### Замечания

#### Critical / Major

Нет.

#### Minor

**Mi-1. `scripts/compose-lint.sh:560-571`, 588: исключение «адрес сервиса сети» принимает голое имя
сервиса для любой переменной, в том числе для списка клиентов.** `names_services` считает адресом
любой элемент, чей хост — имя сервиса. Порт и схема для этого не нужны
(`item.rpartition(":")[0] or item`). `telegram-bot` — одновременно сервис
(`docker-compose.bot.yml:42`) и настоящий идентификатор клиента. Поэтому именно класс T-411
(композиция подставляет свой, более узкий список) проходит линтер зелёным. Зонд
`e7-service-name-allowlist.yml`:

```
MV_GATEWAY_CLIENT_IDS: ${MV_GATEWAY_CLIENT_IDS:-telegram-bot}
MV_CORE_ADMIN_CLIENTS: ${MV_CORE_ADMIN_CLIENTS:-core}
MV_WORLD_ID: ${MV_WORLD_ID:-core}
-> compose-lint: ok — 3 services in 1 file(s), 8 rules
```

*Как исправить:* применять исключение только к переменным, чьё умолчание в манифесте — адрес
(`host:port` или `scheme://`), и требовать от элемента ту же форму: порт или схему. Второй вариант —
явный набор сетевых переменных в линтере (их шесть, они перечислены в dev-log п. 3). Добавить
«плохую» фикстуру со списком клиентов, где стоит имя сервиса.

**Mi-2. `scripts/compose-lint.sh:557`: правило 8 не видит `${MV_X}` и `$MV_X` без умолчания, хотя
шапка (`:48-49`) и комментарий в композиции называют именно эту форму ловушкой.** Если у манифеста
умолчание непустое, эта форма — третий источник значения: пустая строка, а для списка допуска это
«никого». `${MV_X:-}` правило ловит (зонд `e13` отвергнут), а `${MV_X}` — нет. Зонд
`e8-nodefault.yml`: `MV_CORE_ADMIN_CLIENTS: ${MV_CORE_ADMIN_CLIENTS}` и
`MV_GATEWAY_CLIENT_IDS: $MV_GATEWAY_CLIENT_IDS` дают `compose-lint: ok`. Что при этом получает
процесс, показал запуск (случай `none`, переменная `MV_Y: ${MV_Y}`): в контейнере `MV_Y=`, в Go
`LookupEnv(MV_Y) = "", present=true`. Это ровно та «наивная правка», от которой предостерегает
dev-log п. 2: она закрыла бы шлюз и админку всем, у кого нет строки.
*Как исправить:* для `MV_*` с непустым умолчанием манифеста отвергать `${MV_X}` и `$MV_X` без
модификатора (исключая `$$`) с тем же советом: «ключ без значения». Добавить фикстуру.

**Mi-3. `.env.example:108-115, 128`: для prod не сказано, что без строки все три списка включают
`ci-harness`.** Правка намеренно расширила то, что получает контейнер без строк. Раньше это было
`operator` для `/v1/admin/*` и `mvctl` для `X-Actor-Kind`, теперь — список манифеста с `ci-harness`
и `mvctl`. Исполнитель назвал этот риск в dev-log п. 7, но оператору об этом говорит только старая
строчная пометка у `MV_GATEWAY_CLIENT_IDS` («в prod без ci-harness»). У
`MV_GATEWAY_ACTOR_KIND_CLIENTS` и `MV_CORE_ADMIN_CLIENTS` про prod ничего нет, и новый блок тоже
молчит. Почему Minor, а не Major: `AdminOnly` (`shared/runtime/http.go:110-116`) сверяет только
заявленный заголовок `X-Client-Id`, так что тот, кто достучался до порта (он на loopback), может
назваться и `operator`. Новой возможности у вызывающего не появляется. Страдает точность документа
для оператора.
*Как исправить:* дописать в блок `.env.example:108-113` одну строку: «в prod строки держать всегда и
убрать `ci-harness` из всех трёх списков».

#### Nit

**N-1. `scripts/compose-lint.sh:557`: `DEFAULTED` срабатывает на экранированное `$${MV_X:-d}`, а на
вложенной подстановке выдаёт искажённое сообщение.** Зонд `e9-escaped.yml`: `echo $${MV_LOG_LEVEL:-debug}`
в `command` compose оставляет буквально, а правило 8 его отвергает. Зонды `e5`/`e5b`
(`${MV_LOG_LEVEL:-${MV_LOG_FORMAT:-text}}`) дают сообщение «defaults to `'${MV_LOG_FORMAT:-text'`»,
а внутреннее умолчание отдельно не проверяется. Оба случая громкие, молчаливого пропуска нет.
*Исправить:* `(?<!\$)\$\{…`; про вложенность — строка в шапке.

**N-2. `scripts/compose-lint.sh:545-551`: разбор манифеста.** Прогнал регулярку на образцах:
- многострочный `Declare` (имя и умолчание на следующих строках) разбирается верно, так что
  комментарий «on one line» и риск в dev-log п. 7 строже, чем есть на деле;
- константа, `` `raw` ``, экранированная кавычка и `DeclareExternal("MV_…")` дают «not declared» —
  громко;
- конкатенация даёт обрезанное умолчание и громкое расхождение;
- единственный тихий случай — закомментированный `// Declare("MV_A", "old", …)` **после** настоящего:
  `dict` берёт последнее совпадение и сравнивает с `old`. Сейчас таких строк в `vars.go` нет.

*Исправить:* убирать строки `//` до `findall` и поправить комментарий.

**N-3. Правило 3: ключ в кавычках не виден ни `BARE`, ни `ENTRY`.** Зонд `e10-quoted-secret.yml`
(`"MINIO_ROOT_USER":` без значения) даёт `compose-lint: ok`. Для `ENTRY` класс существовал и до
T-411, поэтому он в бэклоге, а не в задаче.

### Что проверено экспериментом

Все зонды лежали в скратче вне репозитория. Контейнеры зонда шли отдельным проектом `t411probe`
на `alpine:3.22` с `--pull never` и после каждого случая снимались `down`: по `docker ps -a` и
`docker network ls` их не осталось. Стенд владельца не трогался. Разрешённые значения печатались
только по трём ключам.

- **Семантика, три случая плюс два.** Композиция `MV_X:` / `MV_Y: ${MV_Y}` / `MV_Z: ${MV_Z:-}`,
  внутри — статический Go-зонд (`os.LookupEnv`) и `env` оболочки.
  - Строки нет: `config` → `MV_X: null`, в `Config.Env` лежит голое `MV_X` без `=`. Контейнер
    **стартует**, оболочка и Go его не видят (`present=false`), значит, действует умолчание
    манифеста. `MV_Y`/`MV_Z` → `""`, `present=true`.
  - `MV_X=`: `present=true`, значение `""`. `List()` даёт пустой список, это «никого»:
    `TestEmptyValueOverridesTheDefault`, `TestAdminOnlyAdmitsNobodyWhenTheAllowListIsEmptied` —
    PASS.
  - `MV_X=a,b`: `"a,b"`.
  - Строчный комментарий после пустого значения: `'# comment'`. Этот класс описан ещё правилом 7, к
    T-411 не относится.
  - Переменная в оболочке при молчащем файле: значение из оболочки, как и было у `${…}`.
- **Стенд владельца** (`COMPOSE_ENV_FILES=.env,build/versions.env`, только три ключа): gateway
  `telegram-bot,ci-harness,mvctl` и `ci-harness,mvctl`, core `operator,mvctl,ci-harness`, то есть
  значения манифеста. `up --dry-run --no-build` пересоздаёт core, gateway и memory, но **точно так
  же** и по `docker-compose.yml` из HEAD. Пересоздание вызвано не T-411, утверждение исполнителя
  подтверждено.
- **Правило 8 на композиции до правки** (`git show HEAD:docker-compose.yml` + bot + legacy): ровно
  3 нарушения, строки 236, 237, 272.
- **Мутации.** С отключённым правилом 8 три фикстуры `bad-default-*` становятся зелёными, а
  `bad-secret-passthrough` остаётся красной по правилу 3. С отключённой проверкой `BARE`
  `bad-secret-passthrough` зеленеет, три `bad-default-*` остаются красными по правилу 8.
  Одиночный прогон каждой фикстуры даёт ровно одно нарушение своего правила.
- **Пограничные случаи правила 8:**

  | Зонд | Результат |
  |---|---|
  | список сервис + не сервис (`e1`) | отвергнут, верно |
  | `http://core:8090`, `neo4j://core:7687`, `core:9092,gateway:9092` (`e2`) | пропущен, верно |
  | `${MV_X-d}` (`e3`) | отвергнут; совпадающий `${MV_LOG_FORMAT-json}` пропущен |
  | `${MV_X:?msg - с дефисом}` (`e4`) | пропущен, это дело правил 3/7 |
  | вложенная подстановка (`e5`, `e5b`) | N-1 |
  | `DeclareDeprecated` (`e6`, `MV_GATEWAY_ADDR`) | отвергнут как retired |
  | голое имя сервиса у списка (`e7`) | **пропущен — Mi-1** |
  | `${MV_X}` / `$MV_X` (`e8`) | **пропущен — Mi-2** |
  | `$${…}` (`e9`) | ложное срабатывание — N-1 |
  | `${MV_X:-}` (`e13`) | отвергнут, верно |
  | ключ в кавычках со своим умолчанием (`e15`) | отвергнут, верно |

- **Правило 3.** Все три файла композиции проходят, то есть законные строки не отвергаются.
  `- MINIO_ROOT_USER` в списке (`e11`) отвергнут. `MINIO_ROOT_USER: ~` (`e12`) ловит прежняя ветка
  «литерал». Хорошие формы для списков — `MV_X:`, `MV_X: null`, `- MV_X` (`e14`) — проходят.
  Ключ в кавычках — N-3.
- **Обязательные прогоны:**
  - `make compose-lint`: ok, 15 сервисов, 8 правил, 10 из 10 «плохих» фикстур отвергнуты;
  - `go run ./cmd/mvctl env check`: 66 переменных, код 0;
  - `docker compose config -q` с настройками владельца: 0;
  - `go build ./... && go vet ./... && go test -short -count=1 ./...`: зелёные;
  - `golangci-lint run ./...`: 0 issues;
  - `gitleaks git --staged --redact .`: no leaks found.

### Предложения в бэклог

1. `shared/runtime/http.go:104`: в doc-комментарии `AdminOnly` написано «"operator" when unset», а
   умолчание манифеста — `operator,mvctl,ci-harness`. Устарело ещё до T-411, но теперь это верно и
   для контейнера. Поправить в проходе по устаревшим именам.
2. Правило 3: ключи в кавычках (N-3), для `ENTRY` и `BARE` одинаково.

## T-411 · ревью #2 · 2026-09-11 · code-reviewer#1

### Границы ревью

Итерация 2 по ревью #1: исправления Mi-1, Mi-2, Mi-3, N-2 и регрессия от них. Прочитаны раздел
«T-411 · ревью #1», запись «T-411 · итерация 2» в `dev-log.md`, раздел «Итерация 2» карточки, две
последние записи `journal.md`. Дифф из индекса: `scripts/compose-lint.sh`, `.env.example`,
`docker-compose.yml`, `Makefile`, `testdata/compose-lint/` (14 «плохих» фикстур, README).
`shared/testkit/swarm` (T-220, работа developer#3) исключён по указанию оркестратора. N-1 и N-3 —
бэклог, не переоткрывались.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 3 (новые).

Все четыре исправления подтверждены своими прогонами. Зонд e7 даёт 3 нарушения, e8 — 2. Из 16
пограничных зондов ревью #1 отличается от ожидаемого только то, что ушло в бэклог (N-1: e5, e5b,
e9; N-3: e10). Каждый снятый мутантом признак зеленит ровно свою фикстуру. Настоящие файлы
композиции и законные формы (пустое умолчание манифеста, не-`MV_`, `$$`) новое условие Mi-2 не
краснит. Три новых Nit — границы эвристик. Все они либо громкие, либо требуют экзотической записи;
в текущей композиции ни одна не проявляется.

### Статус замечаний ревью #1

| № | Статус | Доказательство |
|---|---|---|
| Mi-1 | закрыто | e7 → 3 нарушения правила 8; мутант A (снята развязка `is_address(manifest[name])`, `:627`) зеленит только `bad-default-not-an-address-variable.yml`, мутант B (снята форма элемента, `:594`) — только `bad-default-address-without-port.yml`; настоящие файлы ok под обоими |
| Mi-2 | закрыто | e8 → 2 нарушения; мутант C (из `UNMODIFIED` `:579` убрана ветка `${MV_X}`) зеленит только `bad-nodefault-braced.yml`; мутант D (снято исключение для пустого умолчания) не зеленит ни одной фикстуры, но краснит законный зонд g1 (`${MV_MEMORY_URL}`, `$MV_BACKUP_AGE_RECIPIENT`) — исключение нужно и работает |
| Mi-3 | закрыто | `.env.example:115-116`: «В PROD: все три строки держать ВСЕГДА и убрать ci-harness из всех трёх» |
| N-2 | закрыто (по чтению кода) | строки `//` выбрасываются до `findall` (`manifest_src`); комментарий описывает фактическое поведение; мутацию N-2 сам не ставил, у исполнителя она в dev-log п. 5 |
| N-1 | бэклог, без изменений | e5, e5b — отвергнуты с искажённым сообщением; e9 (`$${MV_LOG_LEVEL:-debug}`) — по-прежнему ложное срабатывание |
| N-3 | бэклог, без изменений | e10 (`"MINIO_ROOT_USER":`) — по-прежнему `ok` |

### Замечания

#### Critical / Major / Minor

Нет.

#### Nit

**N-4. `scripts/compose-lint.sh:581, 594, 627`: граница Mi-1 «адресная переменная — по форме
умолчания» ошибается в обе стороны, и форма элемента не сверяется с формой переменной.**
- Мягкая сторона. `MV_CORE_ADDR` — адрес прослушивания, а не адрес сервиса сети, но его умолчание
  `127.0.0.1:8090` имеет адресную форму. Зонд h1 `${MV_CORE_ADDR:-core:8090}` → `ok`.
- Форма элемента. Требуется порт **или** схема, а не та форма, что у переменной. Зонд h5:
  `MV_MINIO_ENDPOINT:-http://memory:9000` (манифест: «host:port without a scheme»),
  `MV_KAFKA_BROKERS:-kafka://core:9092`, `MV_CORE_URL:-gateway:8090` (URL без схемы) — все `ok`.
- Строгая сторона. У `MV_MEMORY_URL`, `MV_LLM_URL` и `MV_OLLAMA_URL` умолчание пустое,
  у `MV_LLM_HOST` — `127.0.0.1` без порта, у `MV_TELEGRAM_HEALTH_ADDR` — `:8089` без хоста. Поэтому
  для них исключения нет. Зонды h2 (`MV_MEMORY_URL:-http://memory:8082`), h3 и h4 отвергнуты.

Почему Nit: строгая сторона громкая. Мягкая даёт значение, которое падает при старте или при первом
вызове: у MinIO — ошибка разбора endpoint, у core — красный healthcheck на loopback. Такую запись
надо написать специально, T-408 её как раз убрал. Исполнитель сам назвал это допущением
(«граница по форме записи, а не по смыслу», journal).
*Исправить (бэклог):* требовать от элемента ту же форму, что у умолчания манифеста (схема ↔ схема,
`host:port` ↔ `host:port`). Либо заменить эвристику явным набором из шести сетевых переменных
(dev-log T-411 п. 3), тогда `MV_CORE_ADDR` в него не попадёт.

**N-5. `scripts/compose-lint.sh:620, 638`: условие Mi-2 читает строчный комментарий YAML, а его
совет не подходит вне `environment`.**
- g6 `MV_CORE_ADMIN_CLIENTS:   # not ${MV_CORE_ADMIN_CLIENTS}, see T-411` отвергнут: пропускаются
  только строки, которые начинаются с `#`, а compose комментарий не интерполирует. Отказ ложный,
  но громкий.
- g3 (`SOME_URL: "http://${MV_WORLD_ID}:8080"`) и g4 (`${MV_WORLD_ID}` в `command:`,
  `$MV_LOG_LEVEL` в `entrypoint:`) отвергнуты. По существу это верно: при молчащем `.env` compose
  подставит туда пустую строку, а манифест там не действует. Но сообщение говорит «the process gets
  an EMPTY MV_WORLD_ID… Pass it through as a key with no value», хотя процесс получает `SOME_URL`
  или аргумент командной строки, и ключ без значения здесь невозможен.

*Исправить (бэклог, вместе с N-1):* отрезать ` #…` вне кавычек до разбора. Для вхождения не в
позиции значения `MV_X:` сообщение должно советовать `${MV_X:-<умолчание манифеста>}` или `:?`.

**N-6. `scripts/compose-lint.sh:570, 579`: две формы уходят мимо обоих шаблонов.**
- g7: `${MV_CORE_ADMIN_CLIENTS:+x}` и `${MV_GATEWAY_CLIENT_IDS+x}` → `ok`. При молчащем `.env`
  compose подставляет пустую строку, то есть ровно тот исход, от которого закрыт Mi-2.
- g8: `$$$MV_CORE_ADMIN_CLIENTS` → `ok`. Это `$$` плюс интерполяция, а lookbehind `(?<!\$)`
  считает её экранированной.

Обе записи для списка допуска экзотичны, поэтому Nit. *Исправить (бэклог, вместе с N-1):*
`DEFAULTED`/`UNMODIFIED` должны знать `:+`/`+`. Экранирование считать по чётности подряд идущих `$`.

### Что проверено экспериментом

Зонды и мутанты лежали в каталоге из `mktemp -d` (`/tmp/tmp.LyCR7UPM6U`). Он удалён по точному пути,
его отсутствие проверено. Контейнеры и compose-проекты не создавались: линтер вызывает только
`docker compose config`, и `docker ps -a` пуст по `probe`. Стенд владельца и LLM не трогались.

- **Зонды ревью #1 (16).** Результаты e1–e15 и e5b совпадают с ожидаемыми:
  - отвергнуты e1, e3, e6, e13, e15 (правило 8), e11 и e12 (правило 3);
  - пропущены e2, e3ok, e4, e14 и e14b;
  - e7 → **3** нарушения, e8 → **2**;
  - N-1: e5, e5b и e9; N-3: e10.
- **Mi-2, законные формы.**
  - g1: пустое умолчание манифеста → ok.
  - g2: не-`MV_` (`${FOO_X}`, `$BAR_X`, `${COMPOSE_PROJECT_NAME}`) → ok.
  - g5: `$$MV_LOG_LEVEL`, `$${MV_LOG_FORMAT}` в `command` → ok.
  - `DeclareExternal`: в `shared/env/infra.go` нет ни одного имени `MV_*`, только `COMPOSE_*`,
    `MINIO_ROOT_*`, `NEO4J_PASSWORD`, `OLLAMA_*`. Шаблоны берут только `MV_`, так что конфликта нет.
    Гипотетическое `DeclareExternal("MV_…")` дало бы громкое «not declared».
  - g3 и g4 (строка, `command`/`entrypoint`) → отвергнуты, по существу верно (N-5).
  - Все три файла композиции → ok.
- **Граница Mi-1** (h1–h6) — N-4. h6 (`http://telegram-bot` без порта у URL-переменной,
  `memory:0`) → ok, это допустимая форма адреса.
- **Правило 3, регрессия.** g9 `MINIO_ROOT_USER:   # from .env` → отвергнут правилом 3, верно.
- **Мутации (свои, 4).** Каждая — одна изменённая строка в копии скрипта, `repo_root` указывает на
  дерево. Под немутированной копией: настоящие файлы ok, ни одна из 14 фикстур не зеленеет.

  | Мутант | Зеленеет | Настоящие файлы |
  |---|---|---|
  | A: снята развязка по адресу манифеста (`:627`) | `bad-default-not-an-address-variable.yml` | ok |
  | B: снята форма элемента (`:594`) | `bad-default-address-without-port.yml` | ok |
  | C: из `UNMODIFIED` убрана ветка `${MV_X}` | `bad-nodefault-braced.yml` | ok |
  | D: снято исключение для пустого умолчания | ни одна; краснеет законный g1 | ok |

- **Обязательные прогоны:**
  - `make compose-lint`: ok, 15 сервисов, 3 файла, 8 правил, 14 из 14 «плохих» фикстур отвергнуты,
    код 0;
  - `go run ./cmd/mvctl env check`: 66 переменных, код 0;
  - `docker compose config -q` с `COMPOSE_ENV_FILES=.env,build/versions.env`: 0. Три ключа: gateway
    `telegram-bot,ci-harness,mvctl` / `ci-harness,mvctl`, core `operator,mvctl,ci-harness`;
  - `go build ./... && go vet ./...`: зелёные;
  - `go test -short -count=1` без `shared/testkit/swarm`: код 0, 25 пакетов ok;
  - `golangci-lint run ./...`: 0 issues;
  - `gitleaks git --staged --redact .`: no leaks found.

### Предложения в бэклог

1. Добавить к N-1 (правило 8, разбор подстановок): N-5 (строчный комментарий, сообщение вне
   `environment`) и N-6 (`:+`/`+`, нечётное число `$`). Всё это одна правка шаблонов.
2. N-4: форма элемента должна совпадать с формой умолчания манифеста, либо эвристику заменить явным
   набором сетевых переменных. Сделать это при следующем касании правила 8 или вместе с T-412
   (адреса сети у архитектора).

## T-410 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Изменения из индекса: `cmd/multiverse/{serve.go, bus.go, bus_memory.go, serve_test.go}`,
`shared/runtime/{runtime.go, http.go}` (только комментарии), `shared/contracts/ownership{,_test}.go`
(только комментарии), `shared/env/vars.go` (только комментарий), `.golangci.yml` (правило
`cmd-multiverse-bus-memory`, исключение в `no-testkit-in-production`).

Контекст: карточка T-410 и её «Выполнение», запись T-410 в `dev-log.md`; `contracts.md` — правило
приоритета, C-01 v1.3 (`Deps`), гарантии v1.2 (ADR-022, ADR-023), блок «Заглушка»; ADR-023, ADR-025;
`plan/ownership.md` §1; записи `journal.md` про T-408; `components/foundation.md` §2.
`shared/testkit/swarm/` (T-220, ревьюит code-reviewer#2) не открывался.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 4, Nit 2.

Главные заявления подтверждены моими прогонами:
- все девять полей `Deps` заполнены; `Bus` и `Journal` — один объект;
- вид транспорта берётся из `opts.bus` (источник T-408), строка старта называет источник;
- флаг `MV_BUS_VALIDATE_ON_READ` доходит до обоих транспортов (у kafka — моим зондом, см. Mi-2);
- шина закрывается последней на всех путях выхода через `return`.

Порядок ADR-023 проверен мутантами, которые переставляют и снимают `defer`, и зондами на пяти путях.
Живой запуск `--contexts=all --bus=memory`: `/health` ok по семи контекстам.

Все четыре Minor — пробелы в охране и в тексте, дефекта поведения нет. Mi-1 и Mi-3 советую закрыть
в этой же задаче: это один тест и один символ `$`. Возврата они не требуют.

### Замечания

#### Critical / Major

Нет.

#### Minor

**Mi-1. `cmd/multiverse/serve.go:293` (`release()`) и `:199` (`run(ctx, stop)`): что повторный
сигнал убивает процесс, не закреплено ни одним тестом.** Чтением кода поведение прежнее. `serve`
регистрирует `NotifyContext`, `run` вызывает `release` (это прежний `stop`) сразу после ожидания,
до `srv.Stop` и `StopAll`. Регистрация сдвинулась раньше `openBus`/`NewHTTP`, но между ними ничего
не блокирует. Самое рискованное место разделения при этом без охраны:
- мутант R1 (строка `release()` удалена) проходит все тесты задачи: `runUntilStarted` передаёт как
  `release` тот же `cancel`, который уже вызван;
- мутант R5 (`serve` передаёт `func() {}` вместо `stop`) тоже выживает.

R1 убивает мой зонд `TestProbeReleaseComesBeforeStop` (и `TestProbeCancelledBeforeRun`): `release`
пишется в ленту, ожидается `start first|release|stop first|bus closed`.

*Как исправить:* добавить такой тест в `serve_test.go`. R5 без настоящего сигнала (на Windows его не
послать) не поймать. Достаточно комментария у вызова в `serve`, что `stop` передаётся ради второго
сигнала; либо вынести `signal.NotifyContext` в функцию, возвращающую пару `(ctx, release)`, и
проверить её отдельно.

**Mi-2. `cmd/multiverse/bus.go:44` — M7 наблюдаем без брокера.** Утверждение dev-log «поля
`eventbus.Kafka` неэкспортируемые, без брокера не наблюдаемо» неверно. Неэкспортируемое поле
`bool` читается через `reflect`: `Bool()` разрешён, запрещены только `Interface()`/`Set`. Мой зонд
`TestProbeKafkaGetsValidateOnRead`:

```go
got := reflect.ValueOf(b.(*eventbus.Kafka)).Elem().FieldByName("skipValidateOnRead").Bool()
```

Он зелёный на исходном коде (по умолчанию `false`, при `MV_BUS_VALIDATE_ON_READ=false` — `true`) и
красный на мутанте R4 (= M7, `SkipValidateOnRead: validate`).

*Как исправить (любое из двух):* (а) вставить этот тест как есть; (б) чище — вынести сборку
`eventbus.KafkaConfig` в функцию `kafkaConfig(reg, validate, timers, log)` и проверить её поля без
`reflect`. Интеграционный тест из бэклога (п. 3 карточки) тогда не нужен ради этого флага.

**Mi-3. `.golangci.yml:93-94` — исключение открывает префикс `shared/testkit/membus*`, а не «один
пакет».** depguard сравнивает `allow` по префиксу строки. Мутанты в экспорте индекса
(`git checkout-index` во временный каталог):

| # | Мутант | Результат |
|---|---|---|
| L1 | `bus_memory.go` импортирует `shared/testkit/state` | красный, `cmd-multiverse-bus-memory` |
| L2 | другой файл `cmd/multiverse` импортирует `membus` | красный, `no-testkit-in-production` |
| L3 | `cmd/mvctl/bus_memory.go` импортирует `membus` | красный, `no-testkit-in-production` |
| L4 | создан пакет `shared/testkit/membusx`, `bus_memory.go` импортирует его | **зелёный, 0 issues** |
| L4b | то же, `allow` записан как `multiverse-core.io/shared/testkit/membus$` | красный |
| L4c | `allow` с `$`, исходный `bus_memory.go` | зелёный |

Сегодня такого пакета нет, но будущий `membus/chaos` или `membusfake` пройдёт молча. Комментарий
правила при этом обещает, что «the file cannot become a door for any other double».

*Как исправить:* `- multiverse-core.io/shared/testkit/membus$`. Проверено: L4b ловит, L4c чистый.

**Mi-4. `cmd/multiverse/bus_memory.go:17-19`, `.golangci.yml:81`, `:87-88` — новые комментарии
описывают несуществующий `fake_contexts.go` как действующий.** Файла `cmd/multiverse/fake_contexts.go`
нет в дереве и не было ни в одной ветке (`git log --all -- cmd/multiverse/fake_contexts.go` пуст).
`FakeContext` и `MV_SWARM_FAKE` не регистрирует и не объявляет никто
(`grep` по `cmd/`, `shared/env/`). Что написано:
- `bus_memory.go`: «It is not cmd/multiverse/fake_contexts.go, which registers a FAKE context and
  leaves with EPIC-003 I1» — в настоящем времени;
- `.golangci.yml:87-88`: то же;
- сообщение `desc` правила `no-testkit-in-production` (`:81`): «ownership.md §1 names
  cmd/multiverse/fake_contexts.go as the single exception». Задача добавила второе исключение в тот
  же `files`, но сообщение не поправила. Нарушитель теперь читает про единственное исключение,
  которого нет, — это видно в выводе мутантов L2 и L3 выше.

Факт, важный для решения архитектора: сегодня `bus_memory.go` — **единственный** импорт
`shared/testkit` в бинарнике.

*Как исправить:* в обоих комментариях говорить о `fake_contexts.go` как о запланированном хуке
(«будет», `ownership.md` §1); `desc` переписать без слова «single», например «exceptions:
cmd/multiverse/fake_contexts.go (planned, EPIC-003 I1) and cmd/multiverse/bus_memory.go
(membus, T-410)».

#### Nit

**N-1. `cmd/multiverse/serve.go:222-223` («the bus is last on EVERY path out») и `:247-249` — при
панике в `Start` это неверно.** `runtime.StartAll` не делает `recover`. Отложенный `closeBus`
выполняется при раскрутке, пока уже запущенный контекст не остановлен. Зонд
`TestProbePanicInStart`: `recovered=start boom timeline: start first|bus closed` — без `stop first`.
Процесс после этого всё равно падает, поэтому вреда нет. Поведение `StartAll` к задаче не относится.
*Как исправить:* в комментарии «every return path», а `recover` в `StartAll` (панику превращать в
ошибку и останавливать запущенные) — в бэклог `shared/runtime`.

**N-2. `shared/runtime/http.go:105` (124 символа) и `shared/env/vars.go:143-145` — комментарии
после правки не переформатированы.** В `http.go` одна строка стала вдвое длиннее соседних. В
`vars.go` строка обрывается на «reduce», продолжение — на следующей. Смысл верный: оба файла
существуют — `scripts/lib/llm-endpoint.sh`, `scripts/lib/LlmEndpoint.psm1`; умолчание
`env.CoreAdminClients` = `operator,mvctl,ci-harness`. Остальные новые комментарии сверены с кодом и
контрактом и верны:
- `runtime.go`: девять полей, нет `Store`/`Env`, `objstore.New` существует, C-01 v1.3;
- `ownership.go`: ADR-025, C-02 v1.4;
- `bus.go`, `serve.go`: источник T-408, SEC-16, ADR-023 п. 4.

### Что проверено экспериментом

Зонды и мутанты Go — через `go test -overlay` из каталога `mktemp -d`. Мутанты depguard — в
экспорте индекса (`git checkout-index -a --prefix=`) в том же каталоге. Каталог удалён по точному
пути; рабочее дерево не менялось (`git status` до и после — 159 строк, непроиндексированных правок
в `cmd/`, `shared/`, `.golangci.yml` нет).

| # | Мутант / зонд | Тесты задачи | Мои зонды |
|---|---|---|---|
| R1 | удалён `release()` в `run` | **выжил** | красный: `ReleaseComesBeforeStop`, `CancelledBeforeRun` |
| R2 | `defer closeBus` снят, `closeBus` сразу после `release()` (до `StopAll`) | красный: `ClosesTheBusAfterEveryContextStopped`, `ClosesTheBusLastWhenTheStartFails` | красный: лента `start…|bus closed|stop second: journal: eventbus: bus is closed|…` |
| R3 | `defer` снят, `closeBus` только перед `return serveErr` | красный: `…LastWhenTheStartFails` (нормальный путь зелёный — оба теста нужны) | — |
| R4 | = M7, kafka получает `validate` вместо `!validate` | **выжил** | красный: `KafkaGetsValidateOnRead` |
| R5 | `serve` передаёт `func() {}` вместо `stop` | **выжил** | выжил (без сигнала не наблюдаем) |

Пути выхода, не покрытые тестами задачи, — мои зонды на исходном коде:
- ошибка в `Stop` у одного контекста: `start first|start second|stop second|stop first|bus closed`,
  ошибка только в логе — верно;
- контекст отменён до `run`: `start first|release|stop first|bus closed`, `err=nil` — верно;
- ошибка создания шины: `err=bus memory (--bus): no broker`, закрывать нечего, лента пуста — верно,
  источник назван;
- `--bus=kafka` при `MV_KAFKA_BROKERS=""`: `bus kafka (MV_BUS): eventbus: kafka: no brokers
  configured` — сообщение полезное;
- паника в `Start`: `start first|bus closed` — см. N-1.

Совместимость с T-408:
- `TestModeAndBusComeFromTheManifest`, `TestFlagOverridesTheManifest`,
  `TestRefusalNamesTheSourceOfTheValue`, `TestRetiredBusValueIsRefusedByName` — зелёные;
- `openBus` получает `opts.bus` и больше ничего; второго источника нет;
- compose передаёт `MV_BUS=kafka` и `MV_KAFKA_BROKERS=redpanda:9092` из общего якоря, поэтому путь
  kafka в стеке не ломается (`NewKafka` к брокеру при создании не обращается).

Живой запуск: бинарник собран во временный каталог, `MV_CORE_ADDR=127.0.0.1:55731 multiverse.exe
--contexts=all --bus=memory`.
- `/health`: `ok`, семь контекстов `ok`; `multiverse health --url …` — `ok`, код 0.
- Строка старта: `mode: live (MV_MODE), bus: memory (--bus)`.
- Остановка: процесс найден по слушающему порту (PID 41012). Перед `Stop-Process` сверены путь exe
  (мой временный каталог) и командная строка (`--contexts=all --bus=memory`). После этого порт
  свободен. Стек владельца и LLM не трогались.

Обязательные прогоны:
- `go build ./... && go vet ./...`: зелёные;
- `go test -short -count=1 $(go list ./... | grep -v shared/testkit/swarm)`: код 0;
- `go test -tags e2e -count=1 ./test/...`: `test/e2e` ok, `test/fixtures` ok;
- `golangci-lint run ./...`: 0 issues;
- `mvctl contracts check`: 65 типов, 8 топиков, 58 схем, код 0; `mvctl env check`: 66 переменных, код 0;
- `gitleaks git --staged --redact .`: no leaks found.

`-race` не запускался (T-401).

### Предложения в бэклог

1. `shared/runtime.StartAll`: `recover` вокруг `Start` — паника становится ошибкой, запущенные
   контексты останавливаются, шина закрывается после них (N-1).
2. Решение архитектора «узкое исключение или `membus` в `shared/eventbus`». С точки зрения ревью
   исключение приемлемо как временное: одна точка, линтер держит L1–L3, после Mi-3 — и L4. Но
   C-01 «Заглушка» называет `membus` заглушкой для потребителей, а `foundation.md` §2 делает его
   транспортом режима бинарника. Это два статуса одного пакета, и развести их должен контракт, а не
   `.golangci.yml`. Вместе с решением поправить `ownership.md` §1 («единственный импорт»).
3. Пункты 1, 2 и 4 бэклога карточки поддерживаю. Пункт 3 (интеграционный тест ради M7) снимается,
   если закрыть Mi-2.

## T-410 · ревью #2 · 2026-09-11 · code-reviewer#1

### Границы ревью

Итерация 2 по ревью #1: Mi-1…Mi-4 и N-2 закрыты кодом, N-1 закрыт только текстом (`recover` в
`StartAll` оркестратор отправил в бэклог). Проверены исправления и регрессия от них. Файлы в индексе:
`cmd/multiverse/{serve.go, bus.go, bus_memory.go, serve_test.go}`, `.golangci.yml`,
`shared/runtime/http.go`, `shared/env/vars.go`. Контекст:
- «Итерация 2» в карточке и в `dev-log.md`;
- последние записи `journal.md`;
- `serve.go` из HEAD — чтобы сверить обработку сигналов до задачи;
- заголовок T-255 и `plan/ownership.md` §1 — чтобы сверить Mi-4.

`shared/testkit/swarm/` (T-220, code-reviewer#2) не открывался.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 1.

Все пять пунктов ревью #1 закрыты, каждое закрытие подтверждено моим мутантом или зондом. Регрессии
в обработке сигналов и в коде выхода нет — проверено живым CTRL_BREAK на своём бинарнике.

### Статус замечаний ревью #1

| # | Статус | Доказательство |
|---|---|---|
| Mi-1 | закрыто | R1, R5 и новый R6 (`release()` после `StopAll`) красные на тестах задачи; зонд с настоящим двойным сигналом — ниже |
| Mi-2 | закрыто | M7 и M7b красные на `TestKafkaConfigCarriesTheManifest`; остаток M7c — N-3 |
| Mi-3 | закрыто | L4 (`membusx`) красный, контроль без `$` — 0 issues, исходный файл — 0 issues |
| Mi-4 | закрыто | `desc` и оба комментария сверены с фактами, исключение не шире одного файла (L5, L6) |
| N-1 | закрыто текстом | `serve.go:228-239` — «every path out through a return», паника оговорена верно; `recover` — бэклог оркестратора |
| N-2 | закрыто | `http.go:101-109`, `vars.go:143-149` переформатированы, смысл прежний |

**Mi-4 подробно.** Каждое утверждение `desc` правила `no-testkit-in-production` (`.golangci.yml:81`)
и комментариев (`:83-93`, `bus_memory.go:14-21`) сверено с источником:
- `fake_contexts.go` «planned by T-255» — заголовок T-255: «Хук `cmd/multiverse/fake_contexts.go`
  (`MV_SWARM_FAKE`)»;
- «leaves with EPIC-003 I1» — `ownership.md` §1, строка 16: «удаляется при слиянии EPIC-003 I1»;
- «сегодня `bus_memory.go` — единственный импорт `testkit`» — в `cmd/multiverse` нет
  `fake_contexts.go`, L0 чистый;
- слова «single» в `desc` больше нет.

Ложных утверждений не найдено. `ownership.md` §1 по-прежнему называет `fake_contexts.go`
«единственным импортом», но это документ архитектора, пункт 2 бэклога ревью #1.

### Новые замечания

#### Critical / Major / Minor

Нет.

#### Nit

**N-3. `cmd/multiverse/bus.go:43` — выживает M7c, хотя поймать его дёшево.** Мутант подменяет флаг
в самой строке вызова: `cfg.SkipValidateOnRead = false; return eventbus.NewKafka(cfg)`. Тесты задачи
его пропускают. Утверждение dev-log верно: без брокера его ловит только `reflect`. Но дорого это не
стоит: мой зонд `TestProbeKafkaAdapterSkip` — это `openBus(busKafka, …)` и чтение поля через
`reflect.ValueOf(b.(*eventbus.Kafka)).Elem().FieldByName("skipValidateOnRead").Bool()` для значений
`""/false/true`. На исходном коде он зелёный, на M7c красный: `value "false": adapter skip=false
want true`.

Для временного решения хватит и одной строки без теста: между проверенной `kafkaConfig` и адаптером
нет логики. Возврата не требует.

*Как исправить (по желанию, можно в бэклог):* добавить эти две строки с `reflect` в
`TestOpenBusBuildsTheTransportTheValueNames`, где адаптер kafka уже создаётся. Либо закрыть
интеграционным тестом `openBus(kafka)`, когда он появится.

### Регрессия сигналов и кода выхода (пункт 2 задания)

Чтение кода, `serve` в HEAD против индекса:
- до задачи: `NotifyContext` → `defer stop()` → `StartAll` → … ожидание → `stop()` → `srv.Stop` →
  `StopAll`;
- после задачи: `withSignals` делает `NotifyContext` → `defer stop()` → `fn(ctx, stop)`, а `run`
  вызывает `release()` (тот же `stop`) сразу после ожидания, до `srv.Stop`.

Последовательность та же. Единственный сдвиг: регистрация теперь раньше `openBus`/`NewHTTP`.
Сигнал в этом окне в несколько миллисекунд теперь ведёт к штатной остановке, а прежде убивал
процесс обработчиком по умолчанию. Это безвредно. `runServe` не менялся: 0 — успех, 1 — ошибка
`serve`, 2 — ошибка разбора.

Эксперимент (Windows, CTRL_BREAK доходит до Go как `os.Interrupt` = `syscall.SIGINT`; дочерний процесс
в своей группе, `CREATE_NEW_PROCESS_GROUP`, `GenerateConsoleCtrlEvent` только в эту группу):

| Проверка | Исходный код | Мутант |
|---|---|---|
| зонд `withSignals`: первый сигнал → `first`, `release()`, второй сигнал через 300 мс | `["ready" "first"]`, выход `0xC000013A` (убит вторым сигналом) | R5 (`release` пустой): `["ready" "first" "survived" …]`, выход 0 — зонд красный |
| живой бинарник `--contexts=all --bus=memory`, один CTRL_BREAK | выход **0**, штатно, порт освобождён | R5b (`serve` обходит `withSignals`): выход `0xC000013A`, убит без остановки |

R5b тесты задачи и зонды пропускают: он виден только на живом процессе. Это одна строка композиции
в `serve`, отдельного теста она не стоит, см. «Предложения в бэклог», п. 2.

### Что проверено экспериментом

Мутанты и зонды Go — через `go test -overlay` из каталога `mktemp -d`
(`/tmp/tmp.njkGD6sBxr`). Мутанты depguard — в выгрузке индекса (`git checkout-index -a`) в том же
каталоге. Каталог удалён по точному пути. Непроиндексированных правок в `cmd/`, `shared/`,
`.golangci.yml` нет.

Хеш `git status --porcelain` за время ревью изменился при том же числе строк (162). Сменились только
две строки: `EPIC-003-swarm-llm-laws/review.md` (`M`→`MM`) и `tasks/T-220.md` (`A`→`AM`), обе
записаны в 13:33:54–13:34:01. Это артефакты code-reviewer#2, мои действия их не касались.

| # | Мутант | Тесты задачи | Мой зонд |
|---|---|---|---|
| R1 | удалён `release()` в `run` | красный: `ServeReleasesTheSignalsBeforeStopping` | — |
| R5 | `withSignals` отдаёт `func() {}` | красный: `WithSignalsHandsOverTheRealRelease` | красный: `ProbeSecondSignalKills` |
| R5b | `serve` вызывает `run` без `withSignals` | выжил | выжил в unit, ловит живой CTRL_BREAK |
| R6 | `release()` перенесён после `StopAll` | красный: лента `start first|stop first|release|bus closed` | — |
| M7 | kafka: `SkipValidateOnRead: false && !validate` | красный: `KafkaConfigCarriesTheManifest` | — |
| M7b | kafka: `SkipValidateOnRead: validate` | красный (все три подслучая) | — |
| M7c | `cfg.SkipValidateOnRead = false` перед `NewKafka(cfg)` | выжил | красный: `ProbeKafkaAdapterSkip` (N-3) |
| L0 | исходная выгрузка | 0 issues | — |
| L4 | `shared/testkit/membusx`, импорт из `bus_memory.go` | красный: `cmd-multiverse-bus-memory` | — |
| L4-контроль | то же, `allow` без `$` | 0 issues | — |
| L1 | `bus_memory.go` импортирует `testkit/state` | красный: `cmd-multiverse-bus-memory` | — |
| L2 | `contexts.go` импортирует `membus` | красный: `no-testkit-in-production`, новое `desc` в выводе | — |
| L5 | `cmd/multiverse/fake_contexts.go` импортирует `testkit/state` | 0 issues (исключение работает) | — |
| L6 | `cmd/multiverse/fake_contexts2.go` — то же | красный: `no-testkit-in-production` (исключение не шире файла) | — |

Живой запуск: бинарник собран во временный каталог, запущен из моего harness с
`MV_CORE_ADDR=127.0.0.1:56140`, `--contexts=all --bus=memory`.
- Строка старта: `mode: live (MV_MODE), bus: memory (--bus)`.
- `/health` 200, `ok` по семи контекстам; `multiverse health --url …` — `ok`, код 0.
- Опознание: PID слушателя по `netstat` (19964) совпал с PID дочернего процесса harness, путь exe —
  мой временный каталог, командная строка — моя.
- Остановка: CTRL_BREAK только в группу этого PID. Выход 0, в логе ошибок нет, порт после выхода
  отказывает в соединении.

Мутант R5b — отдельный процесс (PID 43168, порт 56144), остановлен так же. Стек владельца и
LLM (127.0.0.1:8888) не трогались.

Обязательные прогоны:
- `go build ./... && go vet ./...`: зелёные;
- `go test -short -count=1 $(go list ./... | grep -v shared/testkit/swarm)`: код 0;
- `go test -tags e2e -count=1 ./test/...`: `test/e2e` ok, `test/fixtures` ok;
- `golangci-lint run ./...` (v2.13.2): 0 issues;
- `mvctl contracts check`: 65 типов, 8 топиков, 58 схем, код 0; `mvctl env check`: 66 переменных,
  код 0;
- gitleaks: `gitleaks git --staged --redact .` — no leaks; диапазон ветки
  `integration/mvp-1..HEAD` — no leaks. Скан содержимого индекса (вторая половина
  `make secrets-scan`) — **1 находка, не в файлах T-410**, см. «Риски».

`-race` не запускался (T-401).

### Риски (вне задачи, оркестратору)

1. **`make secrets-scan` (а с ним `make ci`) сейчас красный на содержимом индекса.** Находка
   `generic-api-key` в `Docs/dev-team/architecture/components/swarm-llm-laws.md:511` — давний ложный
   срабатыватель, строка таблицы провайдеров §9.1 (`POST {…}/api/chat`, коммит `be33e93`).
   `.gitleaksignore:9-10` держит его отпечаток на строке `:504`. Проиндексированные правки документа
   (hunk `@@ -2,0 +3,2` и `@@ -156 +158,6`) сдвинули строку на 511, и отпечаток перестал совпадать.
   `gitleaks git --staged` этого не видит: сама строка не менялась. Исправление — обновить
   отпечатки в `.gitleaksignore` (`:504` → `:511`), силами того, кто владеет правкой документа, до
   коммита волны.
2. R5b виден только на живом процессе, см. выше.

### Предложения в бэклог

1. N-3 — две строки с `reflect` в `TestOpenBusBuildsTheTransportTheValueNames` либо интеграционный
   тест `openBus(kafka)`.
2. Дымовой тест сигналов под Windows: запуск бинарника в своей группе процессов, CTRL_BREAK, ожидание
   кода 0 и свободного порта. Мой harness — около 80 строк без внешних зависимостей. Он же ловит R5b
   и любую будущую поломку штатной остановки в `serve`. Место — `test/e2e` или `make health`-подобная
   цель.
3. Соседние правила depguard `shared-testkit-mechanics` и `shared-testkit-swarm` держат
   `allow: multiverse-core.io/internal/mechanics` без `$` — тот же класс префикса, что Mi-3. Для них
   это не проверено экспериментом, вывод сделан по L4-контролю. Если подпакеты `internal/mechanics/…`
   двойнику не нужны, стоит поставить `$` вне T-410.

## T-417 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Задача T-417 «C-01 v1.4 в коде: двухшаговый Dedup и id события из причины», метка `contract-change`.
Проверен дифф индекса: `shared/eventbus/{dedup.go, types.go, cause_id.go, cause_id_test.go,
dedup_twostep_test.go, README.md}` и `shared/testkit/contract/contract.go`. Что прочитано до диффа:
- `### T-417` в `tasks.md` и карточка с «Выполнением»;
- `dev-log.md`, раздел «developer#1 · T-417»;
- ADR-027;
- `contracts.md` v0.7: C-01 v1.4 и C-14 v1.2;
- `journal.md` за 2026-09-11: ревью #1–#2 T-220, T-416, приёмка T-416.

Всё запускалось в изолированной копии из экспорта индекса (`git checkout-index -a`). Копия по `.go`
файлам `shared/eventbus` и `shared/testkit/contract` побайтно совпадает с рабочим деревом, если не
считать CR. Файлы T-419 в индекс не попали, поэтому в копии собрался весь модуль, и прогоны идут по
всему модулю, а не только по двум пакетам. Каталоги T-419 не открывались и не оценивались. Грэп
вызывающих `DeriveOption` прошёл по рабочему дереву и захватил их файлы; замечаний по ним нет.
Интеграционные тесты с Redpanda не запускались, `-race` недоступен (T-401).

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 2, Nit 1.

Оба механизма соответствуют ADR-027 и C-01 v1.4. Кодирование имени UUIDv5 однозначно: коллизий нет,
golden-значения автора совпали с моим независимым расчётом. Байты `Derive` без опции равны байтам
HEAD. Прежние тесты зелёные без правок. Оба Minor касаются того, насколько тесты и текст удерживают
сделанное, а не поведения кода.

### Замечания

#### Critical / Major

Нет.

#### Minor

**Mi-1. `shared/eventbus/cause_id_test.go:213-218` — golden закрепляет кодирование только для одной
ASCII-части. Два моих мутанта кодирования выживают на всех тестах пакета.**
- K1: длина в рунах вместо байтов, `utf8.RuneCountInString(component)` вместо `len(component)`.
- K2: опция передаёт в `causeID` только первую часть, `parts[:min(1, len(parts))]`.

Причины у них разные:
- обе golden-строки — одна ASCII-часть, а на ASCII длина в байтах и в рунах совпадает;
- `TestWithCauseIDDoesNotGlueNeighbouringComponents` зовёт `causeID` напрямую, мимо опции;
- в таблице применимости у каждой строки не больше одной части.

Автор пишет, что кодирование закреплено golden-тестом, а ADR-027 считает кодирование частью контракта
replay (NFR-061). Сегодня удержана только ASCII-ветка с одной частью. Id сущностей и виды нарратива
пока ASCII, поэтому ущерба сейчас нет. Но переписывание `causeID` на руны или потеря частей в
опции прошли бы зелёными, и журналы с не-ASCII частями перестали бы воспроизводиться побайтно.

*Как исправить:* добавить в `TestWithCauseIDKeepsItsEncoding` строки, которые идут через `Derive(…,
WithCauseID(…))`: несколько частей, пустая часть, без частей, не-ASCII в причине, типе и частях. Мои
значения — Python `hashlib.sha1(ns.bytes + name)` с версией 5, для ASCII сверено с `uuid.uuid5`;
в Go совпали на коде индекса:

| причина | тип | parts | id |
|---|---|---|---|
| `ev-1` | `t` | `["12:ab","3:x"]` | `7d98e0bb-e5f9-56ca-97eb-3d4c1fe8c862` |
| `ev-1` | `t` | `["a",""]` | `ac52f5dc-7bd0-5e72-8c5d-3b38556ce5e2` |
| `ev-1` | `t` | нет | `056337ca-da42-50be-8cf1-f2e138401948` |
| `ev-1` | `narrative.output` | `["смерть"]` | `41f7097b-ef70-520d-a05d-53a503efcdc9` |
| `ев-1` | `тип` | `["ё","0"]` | `e970d6c2-cf82-584a-a3d4-b4780581db6e` |

K1 краснеет на двух не-ASCII строках, K2 — на четырёх строках с двумя частями.

**Mi-2. `shared/eventbus/cause_id.go:33-34` и README («В `NewRoot` опция паникует») называют паникой
только `NewRoot`, но `Derive` от конверта без id паникует так же, и из обработчика до этой паники
можно дойти по данным.**

Сама паника по делу. ADR-027 п. 1 и C-01 v1.4 прямо требуют её в конструкторе. У `Derive` нет
канала ошибки: сигнатура возвращает только `Event`, других ошибок `Derive` не сообщает. Возврат
ошибки — несовместимая правка C-01, и для задачи он не нужен.

Неполнота в том, что паника на конверте без id — расширение автора сверх ADR. Оно покрыто подтестом
`TestWithCauseIDPanicsOnARootEvent/Derive from an envelope without an id`, но в документации его нет.
И до него можно дойти по данным:
- в `shared/eventbus`, `shared/testkit/membus`, `shared/runtime` и `cmd/multiverse` нет ни одного
  `recover()` (грэп);
- зонд `TestProbeT417PanicEscapesDelivery`: `Delivery{SkipValidateOnRead: true}`, обработчик делает
  `Derive(ev, …, WithCauseID("turn"))` над конвертом без id. Паника вышла из `Deliver` наружу с
  текстом «eventbus: WithCauseID on "narrative.output", an event without a cause…». В процессе
  ADR-001 это падение всех контекстов разом;
- с валидацией при чтении (по умолчанию) тот же конверт уходит в `dead_letters` (`parked=1`), паники
  нет.

Путь по данным открыт только при `MV_BUS_VALIDATE_ON_READ=false`.

*Как исправить:* в godoc `WithCauseID` и в README рядом с `NewRoot` назвать и `Derive` от конверта
без id, с оговоркой, что при выключенной валидации при чтении такой конверт доходит до обработчика.
Вызывающим (T-229, T-230, T-232) — в бэклог.

#### Nit

**N-1. `shared/testkit/contract/contract.go:519-523` — `markerHandled <- struct{}{}` пишет в канал
ёмкостью 1 блокирующей отправкой.** Соседние случаи того же файла пишут неблокирующе
(`contract.go:587-595`, `select { case reached <- …: default: }`). На Redpanda доставка
at-least-once, и если маркер придёт второй раз, обработчик повиснет на отправке, а `sub.stop`
будет ждать его до таймаута теста. На `membus` маркер приходит один раз. Запуском это не
подтверждено: брокер по заданию не поднимался, вывод сделан по коду и контракту C-01.
*Как исправить:* `select { case markerHandled <- struct{}{}: default: }`. Возврата не требует.

### Проверка заявлений автора

| Заявление | Итог | Доказательство |
|---|---|---|
| `Has` только спрашивает, пустой id — `false` | подтверждено | тесты задачи; K6 (`Has` всегда `false`) — 5 тестов пакета и contract `TwoStep…` на `membus` красные: «side effect ran 2 time(s)» |
| `Add` = `Seen`, общий `remember` под мьютексом | подтверждено | чтение `dedup.go`; `TestDedupAddEvictsLikeSeen` |
| `IDs`/`Restore` для окна, заполненного любым путём | подтверждено | `TestDedupTwoStepSurvivesASnapshot`, `TestDedupTwoStepAndSeenShareOneWindow` |
| UUIDv5 от причины, типа и частей; пространство имён — UUIDv5 от URL | подтверждено | Python: `NS 46a1a952-5297-584b-9430-d9895b6db112`, оба golden совпали |
| Префикс длины вместо разделителя | подтверждено, отклонение обосновано | кодирование беспрефиксное, значит однозначно; перебор 248 820 кортежей — коллизий 0 |
| Без причины — паника с именем опции | подтверждено | K5 красный; зонд выше (Mi-2) |
| С опцией генератор не вызывается | подтверждено | `TestWithCauseIDDoesNotConsumeTheGenerator` |
| Без опции `Derive` ведёт себя как прежде | подтверждено побайтно | зонд `DeriveBytes` на `types.go` индекса и HEAD — 6 событий совпали; K4 красный на `TestDeriveWithoutTheOptionTakesTheNextGeneratorID` |
| Прежние `dedup`, `types`, `fixtures` зелёные без правок | подтверждено | `git diff --cached --stat`: файлы не тронуты; `go test` ok |

Пункты задания:
- **(3) опции, читающие `e.ID`.** Грэп по `DeriveOption` и `func(e *Event)`: в пакете опции
  `WithAgent`, `WithScope`, `WithWorld`, `WithRelations`, `WithGMPath`, `WithReplay`,
  `WithTimestamp`, и ни одна не читает `e.ID`. Вызывающие в `test/fixtures` и `contract.go` передают
  только `WithAgent`. Поведение изменилось в одном месте: опция, которая сама ставит id, больше не
  расходует номер генератора. Таких опций нет.
- **(4) `Has` без освежения.** Это буква ADR-027 п. 3 («спрашивает и окно не меняет»). Зонд
  `HasAgingVsSeen` (ёмкость 3) показывает разницу: второй дубль после одного нового события проходит
  через `Has` и гасится через `Seen`. Чтобы это сработало в бою, нужны два дубля одного события,
  разнесённые на ёмкость окна (10 000). Одиночный поздний дубль не гасит и `Seen`. Для окна «уже
  ответил» это безвредно, замечания нет.

### Что проверено экспериментом

Всё — в изолированной копии индекса.
- `go build ./...`, `go vet ./...`, `go vet -tags integration ./shared/testkit/...` — зелёные.
- `go test -short -count=1 ./...` — 27 пакетов ok, код 0.
- `go test -tags e2e -count=1 ./test/...` — `test/e2e` ok, `test/fixtures` ok.
- `golangci-lint run ./shared/eventbus/... ./shared/testkit/contract/...` — 0 issues, с
  `--build-tags integration` тоже 0 issues.
- `go run ./cmd/mvctl contracts check` — 65 типов, 8 топиков, 58 схем, код 0.
- Мутанты через `go test -overlay`:

| # | Мутация | Итог |
|---|---|---|
| K1 | длина в рунах | **выжил** на тестах пакета; мой golden-зонд красный (Mi-1) |
| K2 | опция берёт только первую часть | **выжил** на тестах пакета; мой golden-зонд красный (Mi-1) |
| K3 | причина берётся из `CorrelationID` | красный: `TestWithCauseIDApplicability` |
| K4 | генератор вызывается только без опций | красный: `TestDeriveWithoutTheOptionTakesTheNextGeneratorID` |
| K5 | паника по `e.ID != ""` вместо пустой причины | красный: `TestWithCauseIDPanicsOnARootEvent` |
| K6 | `Has` всегда `false` | красный: 5 тестов пакета и contract `TwoStep…` на `membus` |

- Зонды: golden Python ↔ Go (9 строк), перебор коллизий, паника через `Delivery`, старение через
  `Has`, байты `Derive` HEAD ↔ индекс. Каталоги копии, мутантов и Python удалены по точным путям.

### Предложения в бэклог

1. Mi-1 — строки golden через опцию (значения выше).
2. T-229, T-230, T-232: тест вызывающего на конверт без id при выключенной валидации при чтении или
   явная проверка `ev.ID != ""` до `Derive(…, WithCauseID)`. Отдельно — нужен ли `recover` в
   `Delivery`, решает архитектор: сегодня паника любого обработчика роняет процесс.
3. ADR-027 п. 1 — одной строкой заменить «разделитель, которого нет в id» на «префикс длины у каждой
   части», как предлагает автор.
4. Прогон contract-теста с новым случаем на Redpanda (T-394) — вместе с N-1.

## T-414 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Задача XS: `serve` — явная подкоманда и синоним запуска без подкоманды (путь (а) DoD). Файлы в индексе:
`cmd/multiverse/{main.go, serve.go, dispatch_test.go}`, `CLAUDE.md`, `README.md`. Неиндексированных
правок в этих файлах нет. Контекст: `### T-414` в `tasks.md`, карточка, запись developer#1 в `dev-log.md`,
журнал про T-408 (`fs.Visit`) и T-410 (`withSignals`, `process.run`), мой раздел «T-410 · ревью #2»
(мутанты R1, R5, R5b). В `shared/testkit/{swarm,gateway}/` и `schemas/` (итерация 2 T-419,
developer#3) не заходил.

Незаконченная правка T-419 ломает `go vet ./...` в рабочем дереве:
`shared/testkit/swarm/window_test.go:10:7: undefined: newWindow`. К T-414 это отношения не имеет. Поэтому
все прогоны сделаны в изолированной копии из экспорта индекса (`git checkout-index -a`); в индексе
сборка T-419 целая.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 3.

Заявления автора подтверждены моими прогонами:
- обе формы проходят один `runServe`;
- форма без подкоманды у compose, образа, `make replay` и e2e не сломана;
- неизвестное слово даёт код 2 и перечень подкоманд;
- перекрытие манифеста флагом и строка старта одинаковы в обоих написаниях.

Ни одна форма, работавшая на HEAD, не стала отказом. Обработка сигналов T-410 не изменилась: мутанты R1
и R5 красные.

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| N-1 | Nit | `cmd/multiverse/serve.go:91-92` | `multiverse --contexts=all serve` → `unexpected argument "serve"`, код 2, без подсказки. Проверено на HEAD и на новом бинарнике: текст одинаковый. Предложение автора №1 | Если `fs.Arg(0)` входит в `subcommands`, дописать к отказу «подкоманда пишется первой: multiverse serve --contexts=…». Тест — один случай в `TestUnknownSubcommand…` или рядом. Примерно 5 строк |
| N-2 | Nit | `cmd/multiverse/serve.go:79-80` | `multiverse -h` печатает `Usage of multiverse:` и флаги `serve`; про `health`, `db`, `version` не говорит. Узнать о них можно только из отказа на неизвестное слово. Предложение автора №2 | `fs.Usage` с первой строкой `usage: multiverse [serve] [flags] \| health \| db \| version`; перечень — из того же среза `subcommands`, чтобы не завести второй список. Тест: вывод `-h` содержит все четыре слова |
| N-3 | Nit | `cmd/multiverse/main.go:31` | Мутант N4: `run` передаёт в `dispatch` пустой запуск вместо `serve`. Юнит-тесты его пропускают, потому что `dispatch_test.go` подменяет `start`. Ловит только e2e, и то по сроку, через 64 с. Это тот же класс, что R5b T-410. e2e входит в `make ci` и в CI, так что покрытие есть | Отдельного теста не требую. Быстрое падение пробы e2e при вышедшем процессе уже стоит в T-415 (п. «проба e2e») |

Оценка N-1 и N-2 для оркестратора. Оба дешёвые: примерно 5 и 10 строк плюс по тесту. Оба в теме задачи
(найти подкоманду), закрыть в этой задаче разумно. Поведение они не ломают, поэтому на вердикт не
влияют.

Вне задачи, уже названо автором: `Docs/dev-team/architecture/diagrams/c4-component-foundation.md:16`
говорит «подкоманды health и db», без `serve` и `version`. Это документ архитектора, для T-420.

### Что проверено экспериментом

Копия индекса и бинарники лежали в scratchpad; каталог удалён по точному пути. Бинарник HEAD собран из
той же копии через `-overlay` с `main.go` и `serve.go` из `git show HEAD:`.

**Ключи overlay.** Go разворачивает короткое имя `C:\Users\CD86~1` в кириллический путь (`go env GOMOD`
даёт `C:\Users\Алексей\...`). Поэтому абсолютные ключи с коротким именем молча не совпадают: контрольный
мутант с синтаксической ошибкой собрался с кодом 0. С ключами относительно рабочего каталога контроль
красный (`syntax error`). Все мутанты ниже прогнаны с относительными ключами, и в каждом прогоне
проверено, что файл мутанта отличается от исходного.

**Инцидент.** Первая сборка «HEAD» попала под эту ловушку и оказалась новым кодом. Мой вызов
`head.exe serve --contexts=all` без `MV_CORE_ADDR` поднял процесс на 127.0.0.1:8090, шина kafka по
манифесту. Проработал около 5 минут. Опознан по PID 60120, пути exe в моём scratchpad и командной строке,
остановлен, порт свободен. Влияние на стек владельца, по проверке, нулевое:
- `core` публикует 8090 только внутри контейнера (`8090/tcp`, без проброса на хост), так что запросы с
  хоста к нему не перехватывались;
- `NewKafka` не соединяется с брокером при создании (`shared/eventbus/kafka.go:95-119`, писатели
  заводятся лениво);
- контексты — заглушки, они не публикуют и не подписываются.

После этого все запуски — только с `MV_CORE_ADDR` на свободном порту, `MV_BUS=memory` и сроком 4 с.

**Разбор первого слова, HEAD против нового кода.** Код выхода 124 — процесс поднялся и был снят по сроку
4 с.

| Аргументы | HEAD | Новый |
|---|---|---|
| (нет) | 2, `--contexts is required` | 2, то же |
| `serve --contexts=all` | 2, `unexpected argument "serve"` | 124, поднялся |
| `--contexts all` (через пробел) | 124 | 124 |
| `-contexts=all` | 124 | 124 |
| `serve -contexts=all`, `serve --contexts all` | 2 | 124 |
| `-- --contexts=all` | 2, `unexpected argument "--contexts=all"` | 2, то же |
| `serve -- --contexts=all` | 2, `…"serve"` | 2, `…"--contexts=all"` |
| `""` и `"" --contexts=all` | 2, `unexpected argument ""` | 2, `unknown subcommand ""` с перечнем |
| `--contexts serve` (`serve` — значение флага) | 2, `MV_BUS=memory requires --contexts=all, got "serve"` | 2, то же |
| `--mode serve --contexts=all` | 2, `--mode="serve": expected live or replay` | 2, то же |
| `--contexts=all serve`, `serve serve` | 2, `unexpected argument "serve"` | 2, то же (N-1) |
| `-h` | 0, флаги serve | 0, то же (N-2) |
| `serve -h` | 2 | 0 |
| `help` | 2, `unexpected argument` | 2, перечень |
| `-` | 2, `unexpected argument "-"` | 2, то же |
| `version extra` | 0, `dev` | 0, `dev` |

Итог:
- всё, что на HEAD поднимало процесс, поднимает его и теперь;
- слово на месте значения флага идёт в значение, в подкоманду не превращается;
- в обе стороны сменились только тексты отказов, коды выхода те же.

**Прогоны в копии индекса.**
- `go build ./...` и `go vet ./...` — зелёные;
- `gofmt -l cmd/multiverse` — пусто;
- `go test -short -count=1 ./cmd/...` — ok, 7 пакетов;
- `go test -tags e2e -count=1 ./test/...` — `test/e2e` ok (3.7 с), `test/fixtures` ok;
- `golangci-lint run ./cmd/...` — 0 issues;
- `gitleaks git --staged --redact .` в репозитории — no leaks found, код 0.

**Compose.** `COMPOSE_ENV_FILES=.env,build/versions.env docker compose config --format json`, напечатано
только поле `command`:

| Сервис | `command` |
|---|---|
| `core` | `['--contexts=state,mechanics,laws,llm,swarm']` |
| `gateway` | `['--contexts=gateway']` |
| `memory` | `['--contexts=memory']` |

`entrypoint` не переопределён, действует `ENTRYPOINT ["/multiverse"]` (`build/Dockerfile:42`). Команда
начинается с флага, значит идёт по ветке `HasPrefix(args[0], "-")`. Контейнеры не запускались.

**Живой запуск** (`new.exe`, 127.0.0.1:18415).

| Команда | `/health` | `multiverse health` | Строка старта |
|---|---|---|---|
| `serve --contexts=all --bus=memory` | ok, семь контекстов | 0 | `mode: live (MV_MODE), bus: memory (--bus)` |
| `--contexts=all --bus=memory` | ok, семь контекстов | 0 | та же |
| `MV_MODE=replay`, `serve … --mode=live` | ok | 0 | `mode: live (--mode)` |
| `helth --url x` | — | — | код 2, `unknown subcommand "helth": expected serve, health, db or version, …` |

Каждый слушатель перед остановкой сверен по PID, пути exe и командной строке (PID 6724, 39048, 67948).
После прогона порт свободен.

**Мутанты.** Таблица автора (M1–M9) не повторялась.

| # | Мутант | Результат |
|---|---|---|
| C | контроль: `main.go` с синтаксической ошибкой (юнит и e2e через `GOFLAGS=-overlay`) | красный: overlay дошёл и до дочернего `go build` в e2e |
| R1 | T-410: удалён `release()` в `process.run` | красный: `TestServeReleasesTheSignalsBeforeStopping` |
| R5 | T-410: `withSignals` отдаёт `func() {}` вместо `stop` | красный: `TestWithSignalsHandsOverTheRealRelease` |
| N2 | отказ на неизвестное слово пишется в stdout | красный: `TestUnknownSubcommandIsRefusedWithTheKnownOnes` (3 подслучая) |
| N3 | форма без подкоманды теряет первый флаг (`args[min(1,len(args)):]`) | красный: `…AreOneCommand` (4 подслучая), `…RefuseAlike` |
| N4 | `run` передаёт пустой запуск вместо `serve` | юнит — выжил; e2e — красный (`no answer … within 1m0s`, 64 с), N-3 |

**Документы.**
- CLAUDE.md: карта каталогов и `go run ./cmd/multiverse serve --contexts=all --bus=memory` — форма
  рабочая, живой запуск выше.
- README: карта каталогов, `serve` можно опустить — верно.
- AGENTS.md:104 (`serve/health/db`) теперь верен.
- Грэп `serve` и вызовов бинарника:
  - `Docs/ops/runbook.md:53` и `Makefile:298` — `/multiverse health`, рабочая форма;
  - `Docs/ops/runbook.md:384` и `Makefile:428` (`make replay`) — без подкоманды, рабочая форма;
  - `build/Dockerfile:41-42` — `ENTRYPOINT` и пример healthcheck `health --url`;
  - `.github/` — вызовов бинарника нет.

  Неработающей формы не осталось.

### Предложения в бэклог

1. N-1 и N-2 — если оркестратор не закроет их в этой задаче.
2. Ловушку overlay с коротким именем пути (`CD86~1` → кириллица) описать в руководстве по мутантам или в
   памяти команды. С абсолютным ключом прогон молча становится пустым, и контрольный мутант с
   синтаксической ошибкой должен быть обязательным шагом каждого прогона мутантов.

## T-418 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Задача T-418 «Перенос membus в shared/eventbus/membus», метка `contract-change`. Проверен дифф индекса
`git diff --cached -M`: 33 пути. Код и конфиги:
- `.golangci.yml`;
- `cmd/multiverse/{bus_memory.go, fake_contexts_test.go, serve_test.go}`;
- `shared/eventbus/membus/{membus.go, membus_test.go}` (R097, R099);
- `shared/testkit/contract/{contract.go, membus_test.go}`;
- 9 тестов `shared/testkit/{gateway,state,swarm}`;
- `test/e2e/stubs_v0_test.go`, `test/fixtures/events_test.go`.

Документы: `CLAUDE.md`, `AGENTS.md`, `README.md`, `shared/eventbus/README.md`, `components/foundation.md`
§1 и §9, `testing/strategy.md`.

Что прочитано до диффа:
- `### T-418` в `tasks.md` и карточка с «Выполнением» и «Добавкой до ревью»;
- `dev-log.md`, раздел «developer#1 · T-418»;
- `journal.md`: T-410 (решения, ревью #1–#2, приёмка), T-416 и её приёмка, T-425 и её приёмка, записи
  T-418 и решение оркестратора о расширении правила;
- `contracts.md` v0.8: C-01 (v1.4, v1.5) и блок «Заглушка для потребителей» C-01.

Всё запускалось в изолированной копии из экспорта индекса (`git checkout-index -a` в каталог из
`mktemp -d`). Для сравнения с HEAD — вторая копия из `git archive HEAD`. Обе удалены по точному пути.
Рабочее дерево не менялось, файлы T-413 (`shared/env`, `scripts/compose-lint.sh`, `testdata/compose-lint/`,
`.env.example`, `infrastructure.md`) не открывались. `-overlay` не использовался: мутанты правились в
файлах копии и восстанавливались из резервной копии со сверкой `cmp`. Redpanda и интеграционные тесты не
запускались, `-race` недоступен (T-401). Стек и `:8888` не трогались.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 2.

Перенос чистый. Git видит rename, а не delete+add. API `membus` не изменился: `go doc -all` совпадает
до байта во всём, кроме doc-комментария пакета. Код `membus.go` без комментариев совпадает с HEAD. Старый
путь в коде, конфигах и сборке не остался. Бинарник изменился только путём одного пакета, `testkit` в
нём по-прежнему только через хук T-255. Правило depguard ловит то, что должно ловить, и не ловит законные
импорты в тестах, в `test/e2e` и в `shared/testkit/*`. Оба Nit касаются документации.

### Замечания

#### Critical / Major / Minor

Нет.

#### Nit

**N-1. `CLAUDE.md:229-230` — охват правила назван не полностью.** Текст: «Production-код (`cmd/**`,
`internal/**`) `shared/testkit` не импортирует». После добавки до ревью правило охватывает и
`shared/eventbus/**`, так говорит `desc` в `.golangci.yml:87` и мутант M1 ниже. Утверждение не
ложное, но неполное, а CLAUDE.md читают агенты как инструкцию.
*Как исправить:* «Production-код (`cmd/**`, `internal/**`, `shared/eventbus/**`, включая `membus`)».

**N-2. `.golangci.yml:91` — строка комментария 105 символов.** Соседние строки блока 75–78 символов.
Это след склейки после удаления абзаца про `bus_memory.go`.
*Как исправить:* перенести `MV_SWARM_FAKE=true mounts FakeEncounter and` на следующую строку.

### Что проверено экспериментом

**Заявления автора.**

| Заявление | Проверка | Результат |
|---|---|---|
| `git mv` сохраняет историю | `git diff --cached -M --name-status` | `R097 membus.go`, `R099 membus_test.go`: rename при пороге по умолчанию 50 %, тот же механизм, что у `--follow` |
| импортёров 15, все обновлены | дифф: `bus_memory.go` + 14 тестов (+ собственный тест пакета) | 15 совпадает |
| старый путь не остался | `git grep --cached "testkit/membus" -- ':!Docs'` | пусто (exit 1): `.golangci.yml`, `Makefile`, `.github/`, `build/`, `services/*`, `internal/*` |
| API не изменился | `go doc -all` HEAD vs индекс; код без строк `//` | FUNCTIONS/TYPES совпадают; не-комментарийный код идентичен; тест пакета отличается только строкой импорта |
| contract-тест с нового пути | `go test -short -count=1 -run TestBusContractOnMembus -v ./shared/testkit/contract/` | 21 PASS, 0 FAIL, `TwoStepDedupRemembersOnlyAfterTheSideEffect` в их числе |
| в бинарнике testkit только через хук | `go list -deps ./cmd/multiverse`, HEAD vs индекс | разница — ровно `-shared/testkit/membus +shared/eventbus/membus`; `testkit/{swarm,swarm/template,state,mechanics}` как в HEAD; из не-testkit пакетов testkit импортирует только `cmd/multiverse` (`testkit/swarm`, хук T-255) |

**Прогоны в копии индекса.**
- `go build ./... && go vet ./...` — 0.
- `go test -short -count=1 ./...` — 27 пакетов ok, 0 FAIL, в том числе `shared/eventbus/membus`.
- `go test -tags e2e -count=1 ./test/...` — `test/e2e` ok (4.8 с), `test/fixtures` ok.
- `go vet -tags integration ./shared/testkit/... ./shared/eventbus/...` — 0; `go vet -tags e2e ./test/...` — 0.
- `golangci-lint run ./...` (v2.13.2) — 0 issues.
- `go run ./cmd/mvctl contracts check` — 65 types, 8 topics, 58 schema files, 0.
- Живой запуск бинарника из копии: `MV_CORE_ADDR=127.0.0.1:18941 mv418.exe serve --contexts=all
  --bus=memory` (порт перед этим свободен). `/health`:
  `{"status":"ok","details":{"contexts":{…семь…:"ok"}}}`. Строка старта: `bus: memory (--bus)`. Слушатель
  PID 56148 перед остановкой сверен по пути exe (`scratchpad\mv418.exe`) и командной строке, остановлен
  только он. После — `:18941` не слушается, `mv418.exe` в системе нет.

**Мутанты depguard** (линтер по затронутым пакетам копии).

| # | Мутант | Результат |
|---|---|---|
| M0 | контроль: `func (` в конец `membus.go` | красный, typecheck, код 7 |
| M1 | `_ "…/shared/testkit"` в `membus.go` | depguard `no-testkit-in-production`, 1 |
| M1-был | M1 под `.golangci.yml` из HEAD | depguard 0 — до добавки проходило молча |
| M2 | тот же импорт в `membus/mutant_test.go` (`package membus`) | 0 — тесты не задеты |
| M3 | `bus_memory.go` на старый путь `shared/testkit/membus` (пакет восстановлен в копии) | depguard `no-testkit-in-production`, 1 |
| M3-был | M3 под `.golangci.yml` из HEAD | 0 — прежнее исключение пропускало |
| M4 | `_ "…/shared/testkit"` в `internal/mechanics/actor.go` | depguard, 1 — прежние globs не сломаны |
| M5 | `_ "…/shared/testkit/state"` в `fake_contexts.go` | depguard `cmd-multiverse-fake-contexts`, 1 — хук по-прежнему узкий |
| M6 | `_ "…/shared/testkit"` в не-тестовом `test/e2e/doc.go` | 0 — правило не шире нужного |

В M1, M4 и M5 кроме depguard по 2 находки формата: вставленный импорт стоит не в своей группе. Это
артефакт мутанта, не задачи. Законные импорты `testkit`, которые чистый прогон линтера пропускает:
`shared/eventbus/membus/membus_test.go`, `test/e2e/{empty_world,stubs_v0}_test.go`, не-тестовые
`shared/testkit/contract/contract.go` и `shared/testkit/swarm/{fake_context,fake_narrator}.go`. После
восстановления каждый файл сверен `cmp`, итоговый `golangci-lint run ./...` — 0 issues.

**Документы.** Карты каталогов в `CLAUDE.md`, `AGENTS.md`, `README.md`, абзац `shared/eventbus/README.md`,
§1 и §9 `foundation.md`, `strategy.md:90` и `:368` соответствуют дереву. Оставшиеся упоминания
`shared/testkit/membus` в `Docs/` — это `contracts.md:191` и `:577`, `ownership.md`, `api-contracts.md:259`,
`ADR-010:30`, `c4-component-foundation.md`, `EPIC-003/design.md:142`, `infrastructure.md:1138` и записи
прошлого. Все они уже собраны в T-428 и в это ревью не входят.

### Предложения в бэклог

1. N-1 можно закрыть в T-428 вместе с остальными документами, если оркестратор не закроет его в этой
   задаче.
2. Для памяти команды о живых запусках: в Git Bash `cd … && exe … &` даёт в `$!` подоболочку bash, а не
   exe. Её winpid — не слушатель. Слушателя опознавать по `netstat -ano` на своём порту, затем по пути exe
   и командной строке. В этом ревью первая попытка остановки попала в собственную подоболочку. Процесс
   сверен и остановлен вторым шагом, чужое не задето.

## T-426 · ревью #1 · 2026-09-11 · code-reviewer#2

### Границы ревью

Задача T-426 «Перехват паники обработчика в `eventbus.Delivery` (C-01 v1.5)», метка `contract-change`.
Проверен дифф индекса по путям задачи:
- `shared/eventbus/{delivery.go, registry.go, bus.go, README.md}`;
- тесты `shared/eventbus/{delivery_test.go, kafka_test.go}` и `shared/eventbus/membus/membus_test.go`;
- `shared/testkit/contract/contract.go`;
- `Docs/dev-team/architecture/components/foundation.md` §5.

Что прочитано до диффа:
- `### T-426` в `tasks.md` и карточка с «Выполнением»;
- запись developer#1 · T-426 в `dev-log.md`;
- `contracts.md` v0.8: C-01 v1.5, «Гарантии», блок «Паника обработчика» и история;
- ADR-007 п. 6;
- `journal.md`: решение по `recover` после T-417, T-425 и её приёмка, T-418, запуск и сдача T-426.

Для оценки дополнительно прочитаны: циклы чтения `kafka.go` и `membus.go`, `logging.BusMiddleware`,
определение `service_panics` (`metrics.md:114`) и `Reader.CommitMessages` в kafka-go v0.4.49.

Всё запускалось в изолированной копии из экспорта индекса (`git checkout-index -a` в каталог из
`mktemp -d` в scratchpad). Копия удалена по точному пути. Рабочее дерево не менялось, файлы T-413 не
открывались. `-overlay` не использовался: мутанты правились в файле копии, каждая правка сверялась `cmp`
с резервной копией, которая сама сверена `cmp` с `git show :shared/eventbus/delivery.go`. Redpanda и
интеграционные тесты не запускались, `-race` недоступен (T-401). Стек и LLM не трогались.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 2, Nit 2.

Перехват стоит ровно там, где требует C-01 v1.5. Обе реализации зовут обработчик только через
`Delivery.Deliver`: `kafka.go:193/252/282` и `membus.go:307/350/390` → `deliver` → `Deliver`/`DeliverRaw`.
Других вызовов обработчика и других конструкторов `Delivery` в дереве нет. Паника не повторяется,
паркуется с номером вызова, лог `Error` со стеком обработчика. Офсет или курсор сдвигаются только после
`nil`. Все заявления автора подтверждены прогонами, кроме одного: парковка паники при остановке верна,
но тестом не закреплена (Mi-2). Нашлась одна дыра в самом перехвате (Mi-1). Путь к ней надуманный, и
падение в этом случае не хуже, чем было до задачи. Поэтому это Minor, а не Major.

### Замечания

#### Critical / Major

Нет.

#### Minor

**Mi-1. `delivery.go:236-237`, `:248`, `:261` — вложенная паника форматирования уходит из `call` и
роняет процесс.** Значение паники форматируется внутри отложенной функции уже после `recover`: `%w`/`%v`
в `handlerPanic` и `fmt.Sprint` в `logPanic`. Паникующий `String()`/`Error()` первого уровня `fmt` ловит
сам, и это безопасно. Но если метод паникует значением, которое при печати тоже паникует, `fmt`
(`catchPanic`, ветка `p.panicking`) паникует повторно. Новая паника выходит из отложенной функции
наружу, мимо `Deliver`. Нет ни записи в `dead_letters`, ни строки `Error`: `logPanic` стоит после
`handlerPanic`. Событие не закоммичено, то есть тот самый цикл рестартов, против которого заведён
C-01 v1.5.
*Доказательство* — зонд `TestProbeT426PanicValueFormatting` в копии, `Deliver` обёрнут в `recover`
теста:

| Значение паники | Результат |
|---|---|
| `type S struct{}; func (S) String() string { panic(S{}) }` | **паника вышла из `Deliver`**, писем 0 |
| то же для `Error()` (ветка `%w`) | **паника вышла из `Deliver`**, писем 0 |
| `String()` паникует строкой | припарковано: `eventbus: handler panic: %!v(PANIC=String method: stringer broke)` |
| nil-указатель со `String()` | припарковано: `eventbus: handler panic: <nil>` |

*Как исправить:* превращать значение в текст один раз, в помощнике со своим `recover`, например
`panicText(v any) (s string)`. При вложенной панике он возвращает `fmt.Sprintf("%T", v)` плюс пометку.
Этот текст идёт и в ошибку, и в поле `panic` лога. Чтобы ошибка-значение осталась доступной для
`errors.Is`, её можно оборачивать своим типом с `Unwrap()` и готовым текстом, а не через `%w`: вызов
`fmt.Errorf("%w", v)` вызывает `v.Error()`. Тест — два случая из таблицы: одно письмо, `nil` из
`Deliver`, строка `Error` есть. `debug.Stack()` безопасен и может остаться, где стоит.

**Mi-2. `delivery.go:112-117` — «паника при остановке паркуется» не закреплена тестом.** Карточка и
dev-log называют это решением: `ctx.Err()` для паники не проверяется. По C-01 v1.5 п. 1–2 решение
верное: паника не повторяется и паркуется сразу, исключения для остановки контракт не делает. Обычная
ошибка при остановке возвращает `ctx.Err()` без парковки, и эта разница — главное, что здесь можно
сломать при будущей правке. Мутант ME (`if ctx.Err() != nil { return ctx.Err() }` в начале ветки
`panicked`) выживает: `shared/eventbus`, `membus` и contract-тест — все ok.
*Как исправить:* тест в `delivery_test.go`: обработчик вызывает `cancel()` своего контекста и
паникует → `Deliver` возвращает `nil`, одно письмо, `attempts=1`, один вызов. Зонд
`TestProbeT426PanicDuringShutdownIsParkedOnce` в копии зелёный на коде задачи, то есть поведение есть,
не хватает только закрепления. Для контраста зонд `…ErrorDuringShutdownIsNotParked` подтверждает, что
обычная ошибка при остановке даёт `context.Canceled` и писем 0.

#### Nit

**N-1. Карточка T-426, «Что сделано», довод «иначе после рестарта она пришла бы снова» верен для
`membus`, но не всегда для kafka.** В kafka `Subscribe` письмо пишется под `WithoutCancel`, а коммит
`reader.CommitMessages(ctx, msg)` (`kafka.go:199`) идёт с уже отменённым `ctx`. В kafka-go v0.4.49
(`reader.go:893-910`) оба `select` выбирают между готовой отправкой и `ctx.Done()` случайно. Коммит
может не пройти, `stopped` вернёт `nil`, и после рестарта событие придёт снова. Обработчик запаникует
второй раз, в `dead_letters` будет второе письмо с тем же `original.id`, затем коммит. Цикла рестартов
нет, так что C-01 соблюдён: это обычный at-least-once. Тот же класс уже есть у отказа валидации при
остановке. Проверено чтением, без брокера.
*Как исправить:* ничего в коде. В README (пункт «Паника обработчика перехватывается») или в
T-394 одной фразой: письма в `dead_letters` при остановке могут повториться, разбор идёт по
`original.id`.

**N-2. `delivery.go:57-61` — после правки doc-комментария `Delivery` осталась короткая строка «what makes
the kafka».** Строки соседних абзацев — около 78 символов, эта — 20.
*Как исправить:* склеить со следующей строкой.

### Что проверено экспериментом

**Заявления автора и вопросы поручения.**

| Заявление / вопрос | Проверка | Результат |
|---|---|---|
| `recover` в отложенной функции, стек видит кадры обработчика | `TestDeliverLogsAPanicAsAnErrorWithItsStack` (кадр `panicInAHandler`); мутант MC | подтверждено |
| `panic(nil)` паркуется | подкейс `a_nil`: `panic called with nil argument` (`go 1.26` в `go.mod`, `panicnil` выключен) | подтверждено |
| `runtime.Goexit` — не паника | зонд `TestProbeT426GoexitIsNotAPanic`: горутина завершилась, `Deliver` не вернул, писем 0 | подтверждено |
| без повтора, `attempts` = номер вызова | зонды: две ошибки, затем паника → `calls=3`, `attempts=3`; три ошибки, затем паника на последнем вызове → `attempts=4` | подтверждено |
| паника при остановке паркуется | зонд: `cancel()` + паника → `nil`, одно письмо | подтверждено; тестом не закреплено (Mi-2) |
| дубль письма при остановке | membus: нет, курсор сдвигается по `nil`. kafka: возможен, см. N-1 | чтение кода |
| текст ошибки, `%w` для ошибки-значения, стека нет в письме | тесты задачи; мутанты MC и MD | подтверждено |
| лог `Error`, поля | тест лога; `service_panics` = `level=error` без `handled=true` (`metrics.md:114`) — строка засчитывается | подтверждено |
| коммит только после записи; сбой записи → ошибка | `TestDeliverReportsAFailingSinkAfterAPanic`; `membus.go:307-310`, `kafka.go:193-199` | подтверждено |
| паника внутри `call` не уходит наружу | зонд по значениям паники | **опровергнуто для вложенной паники форматирования (Mi-1)** |
| обход `Deliver` в `Tail`/`ReadRange`/`DeliverRaw` | grep `h(`/`handler(`/`.Deliver(` по `shared/`; `DeliverRaw` обработчик не принимает | обхода нет |
| свои `recover` в обработчиках | grep `recover()` по `cmd/ internal/ shared/ test/` | только `delivery.go` и четыре теста (`env`, `runtime`, `cause_id`, `cli`), в обработчиках нет; `BusMiddleware` без `defer`, на панике ничего не пишет |
| `errors.Is(err, ErrHandlerPanic)` при сбое записи | зонд: `Is(ErrHandlerPanic)=false`, `Is(errSink)=true` | как заявлено; оценка — в бэклоге |

**Прогоны в копии индекса** (go1.26.8 windows/amd64).
- `go build ./... && go vet ./...` — 0.
- `go vet -tags integration ./shared/testkit/... ./shared/eventbus/...` — 0.
- `go test -short -count=1 ./...` — все пакеты ok, 0 FAIL.
- `go test -tags e2e -count=1 ./test/...` (цель `make test-e2e`) — `test/e2e` ok (5.9 с), `test/fixtures` ok.
- `golangci-lint run ./...` (v2.13.2) — 0 issues.
- `go run ./cmd/mvctl contracts check` — 65 types, 8 topics, 58 schema files, 0.
- `TestBusContractOnMembus` — 22 PASS, 0 FAIL/SKIP, `AHandlerPanicIsParkedWithoutRetry` в их числе.

**Мутанты** `delivery.go`, прогон `go test -short -count=1 ./shared/eventbus/... ./shared/testkit/contract/...`.

| # | Мутант | Результат |
|---|---|---|
| M0 | контроль: `func broken( {` после `package` | красный, сборка |
| MA | перехват только в `Subscribe`: в `call` при `Consumer` с префиксом `journal.` обработчик зовётся без `recover` | красный: `TestReadRangeParksAHandlerPanicAndReadsOn`, contract `AHandlerPanicIsParkedWithoutRetry`, процесс падает с паникой |
| MB | `attempts` всегда 1 | красный: `TestDeliverParksAPanicAfterAFailedAttemptWithItsCallNumber` |
| MC | стек в письме: `fmt.Errorf("%w %s", handlerPanic(r), debug.Stack())` | красный: 3 подкейса `TestDeliverParksAPanicAtOnceWithoutRetrying`, contract-кейс |
| MD | ошибка-значение через `%v` вместо `%w` | красный: `TestHandlerPanicKeepsAnErrorValueMatchable` |
| ME | паника при остановке возвращает `ctx.Err()` вместо парковки | **выжил** — Mi-2 |

Первая попытка MC не собралась: при подстановке perl сломал экранирование `\n`. Мутант повторён с `%w %s`.
После каждого мутанта файл восстановлен и сверен `cmp`; итоговый прогон — ok. Зонд лежал в копии
отдельным файлом и вынесен из пакета до мутантов.

### Предложения в бэклог

1. **`%w` для причины в ошибке `deadLetter` (предложение автора) сейчас не нужно.** Ни один вызывающий
   не разбирает ошибку `Deliver`: циклы `kafka.go` и `membus.go` только возвращают её. Если понадобится,
   нельзя просто заменить `(cause: %v)` на `%w`. `stopped` в `kafka.go:455-456` считает остановкой любой
   `errors.Is(err, io.EOF)`. Обработчик, чья ошибка оборачивает `io.EOF`, при сбое записи в `dead_letters`
   тогда молча завершил бы `Subscribe` с `nil` вместо ошибки. Если различать панику всё же нужно,
   заворачивать только `ErrHandlerPanic`, а не всю причину, и держать это тестом с `io.EOF` в причине.
2. N-1 — одной фразой в T-394 (прогон contract-кейса на Redpanda), там же проверить повтор письма при
   остановке на брокере.
3. Вне задачи, старое поведение: `runtime.Goexit` в обработчике (например, `t.FailNow` в тесте
   потребителя) под `membus.Subscribe` оставляет `g.mu` захваченным (`membus.go:291-309`), и группа
   зависает. Перехват паники этого не меняет. Для памяти команды: в обработчиках тестов использовать
   `t.Error` и канал, а не `t.Fatal`.

## T-413 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Изменения из индекса: `scripts/compose-lint.sh`, `testdata/compose-lint/` (34 фикстуры и README),
`shared/env/{infra.go, env.go, env_test.go, vars.go}`, `.env.example`, `Makefile`,
`.github/workflows/go.yml`, `docker-compose.yml` (только шапка), `infrastructure.md`. Контекст:
- раздел `### T-413` в `tasks.md` и карточка с «Выполнением»;
- запись T-413 в `dev-log.md`;
- мои разделы «T-411 · ревью #1» и «T-411 · ревью #2»;
- `contracts.md` §16 п. 5;
- записи журнала про T-411, T-412, T-413 и T-416.

`shared/eventbus/` (T-426, работа developer#1) не открывался. Всё проверялось в изолированной копии из
экспорта индекса (`git checkout-index` в каталог из `mktemp -d` плюс `git init`, чтобы работал `git grep`
правила 3). Копия удалена по точному пути. Стенд владельца: только `config` и `up --dry-run`, печатались
коды и хеши.

### Вердикт

**ВЕРНУТЬ** — Critical 0, Major 1, Minor 3, Nit 3.

Большая часть заявлений автора подтверждена моими прогонами:
- все пять Nit двух ревью T-411 закрыты, находка приёмки T-411 закрыта;
- явный набор сетевых переменных и сверка формы работают;
- умолчания `OLLAMA_*` сверены с композицией в обе стороны: мутант Go краснит тест, дрейф `infra.go`
  краснит линтер;
- хеши модели владельца совпадают с заявленными.

Возврат — из-за одной регрессии. Новый разбор ключа (`MAPPED`) не допускает пробела перед двоеточием,
поэтому правило 3 молча пропускает литеральный пароль и голый обязательный секрет в записи
`KEY : value`. Эту запись YAML и compose принимают, а старый линтер её отвергал. Правка — два символа в
регулярке и одна фикстура. Мутант с этой правкой (M6 ниже) даёт чистые настоящие файлы и 34 из 34
фикстур.

### Статус N-1…N-6 и находок приёмок

| № | Статус | Доказательство |
|---|---|---|
| N-1 | закрыто | e9 `echo $${MV_LOG_LEVEL:-debug}` → ok. e5 даёт два отказа: «falls back to another variable» и «MV_LOG_FORMAT defaults to 'text'», сообщение неискажённое. e5b (в `command`) → отвергнут |
| N-2 | закрыто в T-411 | — |
| N-3 | закрыто | e10 `"MINIO_ROOT_USER":` → правило 3 (HEAD: ok) |
| N-4 | закрыто | h1 `MV_CORE_ADDR:-core:8090` → отказ со своей причиной (T-408). h5 (`http://` у `MV_MINIO_ENDPOINT`, `kafka://` у брокеров, `gateway:8090` у `MV_CORE_URL`) → 3 отказа формы. h2 (`MV_MEMORY_URL`), h3, h4 → отказ, у h4 с подсказкой про набор. h6 и e2 → ok. В `good-network-set` `MV_LLM_URL` и `MV_TELEGRAM_HEALTH_ADDR` проходят |
| N-5 | закрыто | g6 `MV_CORE_ADMIN_CLIENTS:   # not ${…}` → ok. g3 и g4 → отказ с советом «repeat the manifest's default … or `:?`» вместо «ключ без значения» |
| N-6 | закрыто | g7 → 2 отказа, g8 `$$$MV_…` → отказ. `$$$$MV_…` и `$$$${…}` → ok, как у compose |
| приёмка T-411 (правило фикстуры) | закрыто, с Mi-3 | ST1 (чужое правило), ST2 (чужой `expect-text`), ST3 (нет `expect-rule`), ST4 (`expect-rule: 3 (secrets)`) и ST7 («хорошая», нарушающая правило) — код 1. Контроль ST0 — 0 |
| приёмка T-412 (`[required]` у `MV_LLM_URL` и проверка) | закрыто | пометка стоит прямо над `MV_LLM_URL=`; n10 `${MV_WORLD_ID?x}` в первом файле → правило 7; мутант M17 у автора. Про вложенное `:?` — Mi-2 |
| T-412 (описание `COMPOSE_ENV_FILES`) | закрыто | вариант (а), текст верен |

Прочие законные формы ревью T-411 по-прежнему проходят: e2, e3ok, e4, e14 (`MV_X:`, `MV_X: null`,
`- MV_X`), g1 (пустое умолчание манифеста), g2b, g5, h6. **Исключение — g2
`${COMPOSE_PROJECT_NAME}`**: в ревью #2 T-411 он проходил, теперь отвергнут (Mi-1).

### Замечания

#### Critical

Нет.

#### Major

**Ma-1. `scripts/compose-lint.sh:511` (`MAPPED`): правило 3 перестало видеть ключ с пробелом перед
двоеточием. Это регрессия проверки SEC-14 относительно HEAD.** `MAPPED` требует `NAME:` вплотную. Старые
`ENTRY` и `BARE` допускали `\s*` перед `[:=]`. Зонды:

```
MINIO_ROOT_PASSWORD : hunter2-literal   -> T-413: ok      HEAD: [rule 3] carries the literal
MINIO_ROOT_USER :                       -> T-413: ok      HEAD: [rule 3] passed through with no value
OLLAMA_KEEP_ALIVE :                     -> T-413: ok      (новая проверка правила 8 тоже слепа)
```

Эталон compose v5.2: `A7 : spaced` → в модели `"A7": "spaced"`, то есть compose читает это как ключ.
Выравнивание двоеточий в столбик — встречающийся стиль, так что литеральный пароль проходит защитное
правило без единого слова. Почему Major, а не Minor, как дыры в охране T-411. Здесь это не новая
граница новой проверки: правка убирает защиту, которая работала, и делает это молча. В нынешних файлах
композиции такой записи нет.
*Как исправить:* `(["']?)([A-Za-z_][A-Za-z0-9_]*)\1[ \t]*:` в `MAPPED` (мутант M6 ниже: настоящие файлы
ok, 34 из 34 фикстур, n01/n01b → правило 3, n01c → правило 8). Добавить «плохую» фикстуру
`bad-secret-spaced-colon.yml` (`# expect-rule: 3`).

#### Minor

**Mi-1. `scripts/compose-lint.sh:850-855`: сверка с `DeclareExternal` охватывает все сторонние переменные
с умолчанием, а не только `OLLAMA_*`. В итоге отвергается законная форма `${COMPOSE_PROJECT_NAME}`.**
Контракт §16 п. 5 говорит о «сторонних переменных с умолчанием платформы (`OLLAMA_*`)». Но `external`
собирает все `DeclareExternal` с непустым умолчанием, и туда попадают `COMPOSE_PROJECT_NAME`,
`COMPOSE_PROFILES` и `COMPOSE_ENV_FILES`. Зонд g2 (`PROJ: ${COMPOSE_PROJECT_NAME}`) в ревью #2 T-411
проходил, теперь отвергнут. Эталон compose: без всякого окружения `${COMPOSE_PROJECT_NAME}` → `'gt'`,
имя проекта. Compose задаёт эту переменную сам, пустой она не бывает. Текст отказа тоже неприменим: он
повторяет Mi-2 («the process gets an EMPTY … for an allow-list that is nobody», «shared/env reads
set-to-empty») про переменную, которую ни один процесс платформы не читает. Громко, в нынешних файлах не
проявляется.
*Как исправить:* ограничить `external` именами `OLLAMA_*` или явным списком из контракта (мутант M5:
настоящие файлы и 34 фикстуры ok, g2 → ok). Для `${OLLAMA_X}` без умолчания нужен свой текст отказа
(«контейнер получит пустое значение вместо умолчания `infra.go`»). В `good-*` добавить строку
`${COMPOSE_PROJECT_NAME}`.

**Mi-2. Два новых условия без фикстуры: их мутанты выживают на всех 34 фикстурах. DoD задачи требует
фикстуру и мутант на каждое новое условие.**
- `strip_comment` учитывает кавычки (`:495`). Мутант M1 (`elif False:` вместо ветки кавычки) даёт
  чистые настоящие файлы и 34 фикстуры ok. Зонд q1 `SOME: "x #${MV_CORE_ADMIN_CLIENTS}"` под мутантом
  проходит молча, а compose `#` в кавычках интерполирует (эталон A9).
- Вторая половина правила 7 обходит вложенные подстановки, `walk(...)` (`:800`). Мутант M2 (обход без
  `walk`) даёт то же: ok. Зонд q2 `FOO: ${FOO_UNSET:-${MV_WORLD_ID:?need it}}` под мутантом проходит,
  без мутанта отвергается правилом 7.

*Как исправить:* «плохие» фикстуры по образцу q1 (`# expect-rule: 8`) и q2 (`# expect-rule: 7`, с парным
`.env`, где у переменной нет `[required]`).

**Mi-3. `scripts/compose-lint.sh:137-139, 174-175`: `--fixtures` на пустом или несуществующем каталоге
завершается с кодом 0 и ничего не печатает.** Это ST8: `COMPOSE_LINT_FIXTURES` на пустой каталог даёт
`rc=0` и ни одной строки. Правка `[ -e ] || continue`, закрывшая падение через `set -e`, сделала пустой
прогон зелёным. Переезд или переименование `testdata/compose-lint`, как и опечатка в переменной,
выключают самопроверку в CI и в `make` без следа. Это ровно то, от чего предостерегает шапка цели: «a rule
that quietly stopped firing looks exactly like a clean file».
*Как исправить:* считать «плохие» и «хорошие» фикстуры и выходить с ненулевым кодом, если какой-то из
счётчиков равен нулю. В итоговой строке печатать оба числа («29 bad, 5 good»).

#### Nit

**N-1. Разбор «как у compose» остаётся построчным, и на уровне YAML есть формы, которые compose
интерполирует, а линтер не видит.** Все расхождения молчаливые. Каждое подтверждено эталоном `docker
compose config` v5.2, и все формы экзотические:

| Форма | compose | линтер |
|---|---|---|
| `"\x24{MV_CORE_ADMIN_CLIENTS}"`: escape YAML в двойных кавычках (n03) | подставляет (`A3` → `pval`) | ok |
| `'it''s # ${MV_CORE_ADMIN_CLIENTS}'`: `''` в одинарных кавычках (n05) | `it's # pval` | ok: `''` закрывает кавычку, дальше `#` считается комментарием |
| `#` в строке блочного скаляра `\|` (n04b) | `# pval` | ok: срезано как комментарий |
| `#` в продолжении многострочной строки в кавычках (n13) | `first # pval` | ok |
| `environment: {MINIO_ROOT_PASSWORD: hunter2}`: поток-маппинг, правило 3 (n02) | литерал в контейнере | ok (HEAD тоже ok, класс старый) |

Автор пишет, что «блочные скаляры не разбираются». Это неточно: строки `|` и `>` читаются. n04a (`echo
${MV_CORE_ADMIN_CLIENTS}` в `|`) и n04c (`>`) отвергаются верно. Не видны только строки, начинающиеся с
`#`. Правку формулировки внести в карточку.
Обратные расхождения, громкие: `${MV_LLM_URL:?need ${MV_WORLD_ID}}` (n09) отвергается правилом 8. Но
compose вычисляет сообщение `:?` и умолчание `:-` лениво, только когда они нужны (эталоны gt2 и gt3 дают
0 при заданной внешней переменной и незаданной внутренней `:?`). `${MV_X:-$${Y}}` линтер закрывает на
первой `}`, а compose — на второй: его счётчик скобок не пропускает `$$`. Отказ громкий, сообщение слегка
искажено.

**N-2. Самопроверка фикстур: три мелочи.**
- ST5: две строки `expect-rule` (`3`, затем `8`) — берётся первая, вторая молча игнорируется, код 0.
- ST6: `bad-x.yaml` пропускается без слова, потому что glob знает только `.yml`.
- ST2: после строки о неверной причине печатается ещё и «rejected by rule 3, as it must be». Код
  верный (1), вывод противоречит сам себе.

*Исправить:* несколько строк `expect-rule` — отказ. `bad-*.yaml` — отказ. Строку «as it must be» печатать
только при полном совпадении.

**N-3. Скрипт делает `cd "$repo_root"`, поэтому `-f` с путём, относительным к другому каталогу, даёт
вводящий в заблуждение отказ правила 7** («does not interpolate on a clean machine»). Сам на это
наступил при зондах. Было и до T-413. *Исправить:* превращать пути `-f` в абсолютные до `cd` либо
проверять `[ -f ]` с внятным сообщением.

### Что проверено экспериментом

Все зонды, эталоны compose и мутанты лежали в копии из `mktemp -d`. Мутанты — копиями скрипта в
`scripts/` копии, сгенерированными с проверкой «якорь найден ровно один раз». Каталог удалён по точному
пути. Контейнеры не создавались.

- **Зонды T-411** (e1–e15, e3ok, e5b, g1–g9, g2b, h1–h6), прогон с абсолютными путями. Все результаты
  совпадают с ожидаемыми после T-413, кроме g2 (Mi-1). Таблица статусов — выше.
- **Эталон compose v5.2** (`config --format json` на своём файле со своим `.env`): `$$$${P}` → `$${P}`;
  `${P:-$${Q}}` → значение `P`; `${UNSET:-$${Q}}` → `${Q}`; `\x24{P}` → значение `P`; `'it''s # ${P}'` →
  `it's # pval`; `A7 : spaced` → ключ `A7`; `$$$P` → `$pval`; многострочная строка в кавычках и `#` в `|`
  → подставлено; `>` → подставлено; `${UNSET:-a}b}` → `ab}`; вложенные `:?` в сообщении и в умолчании
  вычисляются лениво (gt2, gt3 → 0); `${COMPOSE_PROJECT_NAME}` → имя проекта.
  `config --no-interpolate` отдаёт те же строки после разбора YAML, но без подстановки (`\x24{P}` →
  `${P}`, блочный скаляр целиком). Это основа для предложения 1.
- **Регрессия правила 3.** HEAD-линтер (`git show HEAD:scripts/compose-lint.sh`) на n01/n01b → правило 3,
  на n01c, n02 и e10 → ok. Новый линтер на n01, n01b, n01c, n02 → ok, на e10 → правило 3.
- **Правило 7 и `[required]` (вопрос 3).** Граница «только файл, загружаемый всегда» совпадает с T-397.
  Токен бота в `.env.example:238-243` осознанно без пометки: там объяснено, почему. `:?` в
  `docker-compose.bot.yml:63` несёт внятный текст и срабатывает у того, кто поднял профиль `bot`. Если
  пометить токен `[required]`, README велел бы заполнять его каждому, а это ровно то, что убрал T-397.
  Замечания нет.
- **Самопроверка фикстур:** ST0…ST8 — выше и в Mi-3, N-2.
- **Мутанты линтера (свои, 9).** Каждый прогнан на настоящих файлах, на `--fixtures` и на шести зондах.

  | Мутант | Настоящие | `--fixtures` | Итог |
  |---|---|---|---|
  | M0 контроль: синтаксическая ошибка Python | красные | 29 «no rule at all» и 5 «хороших» отвергнуты | контроль работает |
  | M1 `strip_comment` без учёта кавычек | ok | ok | **выжил** — Mi-2, ловит только q1 |
  | M2 правило 7 без `walk` | ok | ok | **выжил** — Mi-2, ловит только q2 |
  | M3 `op == ":+"` вместо `endswith("+")` | ok | `bad-alternate-plain` — причина | убит |
  | M4 расхождение умолчания `DeclareExternal` не отказ | ok | `bad-ollama-default` зелёная | убит |
  | M5 `external` только `OLLAMA_*` (кандидат правки Mi-1) | ok | ok | g2 → ok, остальное без изменений |
  | M6 `[ \t]*` перед `:` в `MAPPED` (кандидат правки Ma-1) | ok | ok | n01, n01b → правило 3; n01c → правило 8 |
  | M9 тождественный | ok | ok | чистый |

  Восемь мутантов (M0–M6 и M9) шли параллельно: 144 с на все, то есть около 280 запусков линтера.
- **Мутанты Go** (`go test -overlay`, ключ относительный `shared/env/infra.go`, контроль первым): G0
  синтаксис → build failed; G1 `OLLAMA_KV_CACHE_TYPE=q8_0` → FAIL `TestOllamaBlockCarriesTheSettingsOfTheStandAsDefaults`;
  G2 `OLLAMA_ORIGINS=http://127.0.0.1,http://evil.example` → FAIL. Дрейф G1, положенный на диск копии, →
  правило 8 на `docker-compose.yml:434` («defaults to 'f16' here and to 'q8_0'»). Файл восстановлен,
  `git diff --quiet` → 0.
- **`OLLAMA_*` (вопрос 5).** Умолчания `infra.go` совпадают с `infrastructure.md` §4.2 п. 2 и §6.5
  (стр. 987) и с блоком `docker-compose.yml:430-436`. `OLLAMA_ORIGINS` — только loopback, без `*`;
  правило 5 и тест проверяют это отдельно. У владельца, профиль `gpu`: sha256 блока `OLLAMA_*` сервиса
  `ollama` — `5f368c5b551a10dd`, модель — `ddc89274a7f7e5b8`, совпадает с dev-log.
- **Время (вопрос 6).** `make compose-lint` в рабочем дереве — 68 с, код 0. Разбивка: основной прогон —
  1,9 с, одна фикстура — 1,9 с (два вызова `docker compose` по ~0,6 с плюс Python), 34 фикстуры идут
  подряд. В pre-commit линтера нет: `.pre-commit-config.yaml` — gitleaks, golangci-lint и хуки
  pre-commit-hooks; в `.claude/hooks` его тоже нет. В CI это отдельный шаг задания `compose-lint`, плюс
  около минуты. Для CI и `make ci` приемлемо, замечания нет; ускорение — предложение 2.
- **Обязательные прогоны:**
  - `make compose-lint`: код 0, 15 сервисов в 3 файлах, 29 из 29 «плохих» отвергнуты своими правилами,
    5 из 5 «хороших» прошли;
  - `go run ./cmd/mvctl env check`: 67 переменных, код 0;
  - `go build ./... && go vet ./...`: ok;
  - `go test -short -count=1 ./shared/env/... ./cmd/mvctl/...`: 7 пакетов ok;
  - `golangci-lint run ./shared/env/...`: 0 issues;
  - у владельца, `COMPOSE_ENV_FILES=.env,build/versions.env`: `config -q` → 0. Хеши `config --format
    json`: без профиля `0870f33d758b0688`, `gpu` `ddc89274a7f7e5b8`, `gpu+memory+dev` `80c0aac4ebfcbcd7`,
    все три равны заявленным. `up --dry-run --no-build --pull never` → 0;
  - `gitleaks git --staged --redact .`: no leaks found.

### Предложения в бэклог

1. Брать значения для правил 3, 7 и 8 из `docker compose config --no-interpolate --format json`, а не из
   строк файла. Это разом снимает класс N-1 (комментарии, кавычки, escape YAML, блочные скаляры,
   поток-маппинги, `KEY :`). Номера строк для сообщений можно искать по тексту значения.
2. Ускорить `--fixtures`: гонять фикстуры параллельно (`xargs -P "$(nproc)"` или фоновые задания с
   `wait`, вывод собирать по файлам). Мой параллельный прогон мутантов показывает, что `docker compose
   config` параллель переносит. Ожидаемо около 10 с вместо 65.

## T-426 · ревью #2 · 2026-09-11 · code-reviewer#2

### Границы ревью

Повторное ревью T-426 после итерации 2 (developer#1): проверены только исправления по ревью #1 и регрессия
от них. Дифф индекса: `shared/eventbus/{delivery.go, delivery_test.go, README.md}`. Прочитаны: свой раздел
«T-426 · ревью #1», «Итерация 2» в карточке, запись итерации 2 в `dev-log.md`. Для вопроса о цепочке
ошибки дополнительно прочитаны `stopped` и циклы чтения `kafka.go`, `stopping` и циклы `membus.go`,
конструкторы `Delivery` в обеих реализациях, все `errors.Is`/`errors.As` по `shared/`, `cmd/`, `internal/`.

Всё запускалось в изолированной копии из экспорта индекса (`git checkout-index -a` в каталог из
`mktemp -d` в scratchpad). Копия удалена по точному пути. Рабочее дерево не менялось, файлы T-413 не
открывались. Мутанты правились в файле копии. Резервная копия сверена `cmp` с `git show
:shared/eventbus/delivery.go`, после каждого мутанта файл восстановлен и сверен `cmp` с индексом. Redpanda
и интеграционные тесты не запускались, `-race` недоступен (T-401).

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 0.

Все четыре замечания ревью #1 закрыты и подтверждены прогонами. Перехват паники выдерживает все
найденные мной способы уйти наружу, связанные со значением паники. `Unwrap() []error` у `panicError` не
открывает нового пути к `stopped` в kafka: панику, принятую за остановку, получить можно, но только в
ветке `DLQ == nil`, недостижимой в обеих шинах. Ограничение для T-415 — в бэклоге.

### Статус замечаний ревью #1

| # | Замечание | Статус | Доказательство |
|---|---|---|---|
| Mi-1 | вложенная паника форматирования уходит из `call` | **закрыто** | зонд ревью #1 повторён точно, 4 из 4 паркуются (таблица ниже); мутанты MH (автор) и MR (мой) красные |
| Mi-2 | парковка паники при остановке не закреплена | **закрыто** | `TestDeliverParksAPanicDuringShutdown` (`delivery_test.go:628`): `nil`, один вызов, одно письмо, `attempts=1`; мутант ME теперь красный |
| N-1 | повтор письма при остановке в kafka | **закрыто** | `README.md:206-209`: причина (коммит под отменённым контекстом), разбор по `original.id`, в `membus` повтора нет |
| N-2 | висячая строка в doc-комментарии `Delivery` | **закрыто** | `delivery.go:57-61`: строки склеены, длина ровная |

### Замечания

Нет.

### Что проверено экспериментом

**Зонд значений паники** (`TestProbeR2PanicValues`, в копии). Для каждого значения: `Deliver` обёрнут в
`recover` теста, логгер записывает все записи. Итог для всех 11 значений одинаковый: наружу не ушло,
`Deliver` вернул `nil`, одно письмо, одна запись уровня `ERROR`.

| Значение паники | `dl.Error` / поле `panic` |
|---|---|
| R1 `String()` паникует собой (случай ревью #1) | `eventbus: handler panic: eventbus.r2S (printing the value panicked)` |
| R2 `Error()` паникует собой (случай ревью #1) | `… eventbus.r2E (printing the value panicked)` |
| R3 `String()` паникует строкой (случай ревью #1) | `… %!v(PANIC=String method: stringer broke)` |
| R4 nil-указатель со `String()` (случай ревью #1) | `… <nil>` |
| X1 `Format` паникует значением со вложенной паникой | `… eventbus.r2Fmt (printing the value panicked)` |
| X2 `Error()` паникует паникующим `Stringer` | `… eventbus.r2ErrPanicsStringer (printing …)` |
| X3 экзотический `%T`: generic `r2Gen[func(chan<- map[string]any)]` с паникующим `String()` | `… eventbus.r2Gen[func(chan<- map[string]interface {})] (printing …)` |
| X4 паникующий `LogValue()` | `… log valuer`: значение идёт в лог текстом, `LogValue` не вызывается. Через настоящий `slog.JSONHandler` — то же |
| X5 циклический `Unwrap` (`Unwrap()` возвращает себя) | `… cyclic`: `Deliver` цепочку не обходит |
| X6 `io.EOF` / X7 `fmt.Errorf("read: %w", io.EOF)` | `… EOF` / `… read: EOF`: припарковано, не принято за остановку |

`%T` на фолбэке паниковать не может: он печатает `reflect.TypeOf`, методы значения не вызывает. X3 это
подтверждает.

**Цепочка ошибки и `stopped`** (`TestProbeR2StoppedSeesThePanicValue`, «живой» контекст).

| Путь | `stopped(ctx, err)` |
|---|---|
| паника `io.EOF` / обёрнутый `io.EOF` / `io.ErrClosedPipe`, сбой записи в `dead_letters` | `false`: причина идёт через `(cause: %v)` (`delivery.go:188`), цепочка её не несёт |
| то же, `DLQ == nil` (`delivery.go:172`, причина через `%w`) | `true`, но ветка недостижима: `kafka.go:346-355` и `membus.go:425-434` всегда задают `DLQ` |
| до T-426: обработчик *возвращает* `io.EOF`, `DLQ == nil` | `true`: тот же класс уже был, паника в него лишь добавилась |
| гипотетически T-415: `… %w (cause: %w)` | `true` для всех трёх значений (см. бэклог) |
| циклический `Unwrap`: `errors.Is(pe, ErrHandlerPanic)` | `true`, завершается: первый элемент `[]error` совпадает сразу |
| циклический `Unwrap`: `errors.Is(pe, io.EOF)`; `stopped` на ветке `DLQ == nil` | **виснет** (таймаут 1 с). При сбое записи `stopped` завершается (`false`) |

Grep `errors.Is`/`errors.As` по `shared/`, `cmd/`, `internal/`: ошибку `Deliver` разбирает только `stopped`
(`kafka.go:456`). `membus` решает по `stopping(ctx)` (`membus.go:192`), без разбора ошибки. `logging`,
`cmd/multiverse` и заглушки testkit ошибку доставки не разбирают. Пути, по которому ошибка паники сейчас
доходит до `errors.Is`, в дереве нет.

**Прочие пути наружу** (не находки этой задачи):
- паникующий логгер (`slog.Handler.Handle` паникует): паника уходит из `call`, писем 0. Старый путь `Warn`
  для обычной ошибки ведёт себя так же. Это класс «сломан сам логгер процесса», и `recover` в `Delivery`
  его не закрывал ни до задачи, ни после;
- огромный текст: паника строкой 4 МиБ даёт `len(dl.Error)=4194329` и поле `panic` 4 МиБ. Обычная ошибка
  обработчика той же длины даёт 4194304: длина `Error` не ограничена и раньше. В `membus` письмо
  пишется. В kafka письмо больше предела брокера на запись не пройдёт, `Deliver` вернёт ошибку, офсет не
  закоммитится. Это тот же класс, что у обычной ошибки. Бэклог.

**Прогоны в копии индекса** (go1.26.8 windows/amd64).
- `go build ./... && go vet ./...` — 0.
- `go vet -tags integration ./shared/testkit/... ./shared/eventbus/...` — 0.
- `go test -short -count=1 ./...` — все 27 пакетов с тестами ok, 0 FAIL.
- `go test -tags e2e -count=1 -timeout 10m ./test/...` — `test/e2e` ok (5.8 с), `test/fixtures` ok.
- `golangci-lint run ./...` (v2.13.2) — 0 issues.
- `go run ./cmd/mvctl contracts check` — 65 types, 8 topics, 58 schema files, 0.
- `TestBusContractOnMembus` — 22 PASS, 0 FAIL/SKIP, `AHandlerPanicIsParkedWithoutRetry` в их числе.

**Мутанты** `delivery.go`, прогон `go test -short -count=1 ./shared/eventbus/... ./shared/testkit/contract/...`.
Перед прогоном зонд вынесен из пакета.

| # | Мутант | Результат |
|---|---|---|
| M0 | контроль: `func broken( {` в конце файла | красный, сборка (4 пакета) |
| ME | в начале ветки `if panicked` (`:111`): `if ctx.Err() != nil { return ctx.Err() }` | красный: `TestDeliverParksAPanicDuringShutdown` (в ревью #1 выживал) |
| MU | `Unwrap` возвращает только `ErrHandlerPanic`, без значения | красный: `TestHandlerPanicKeepsAnErrorValueMatchable` |
| MR | `panicError.Error()` снова печатает значение (`e.cause.Error()`), а не готовый текст | красный: `TestDeliverParksAPanicWhoseValueCannotBePrinted/Error_panics` |

После каждого мутанта `delivery.go` восстановлен, `cmp` с индексом совпал. Итоговый прогон зелёный.

### Предложения в бэклог

1. **T-415, `%w` для причины при сбое записи.** Предостережение ревью #1 теперь шире. `panicError.Unwrap()
   []error` делает значение паники видимым для `errors.Is`. Если заменить `(cause: %v)` на `%w`, паника
   значением `io.EOF` или `io.ErrClosedPipe` (и ошибка, их оборачивающая) при сбое записи в `dead_letters`
   даст `stopped=true`, и kafka-`Subscribe` молча вернёт `nil` при живом контексте. Зонд: гипотетическое
   `… %w (cause: %w)` даёт `true` для всех трёх значений. Паника с циклическим `Unwrap` повесит `stopped`.
   Если различать панику нужно, заворачивать только `ErrHandlerPanic` (например, `%w` для него и `%v`
   для причины). Альтернатива — сузить `stopped`: смотреть на ошибку ридера или коммита, а не на ошибку
   доставки. В DoD T-415 — тест с паникой значением `io.EOF` при сбое записи.
2. Ограничить длину `DeadLetter.Error` (по аналогии с `MaxDeadLetterRaw`) и поля `panic` в логе. Касается
   и обычной ошибки обработчика, это не регрессия T-426. Нужно до T-394: на Redpanda письмо больше предела
   брокера не запишется, и событие не будет закоммичено.
3. Ветка `DLQ == nil` в `deadLetter` (`delivery.go:172`) оборачивает причину через `%w`. Сейчас она
   недостижима из шин. Если её когда-нибудь станет достигать цикл kafka, действует п. 1.

## T-413 · ревью #2 · 2026-09-11 · code-reviewer#1

### Границы ревью

Повторное ревью после итерации 2 (devops-engineer#1). Проверены только исправления по ревью #1 и регрессия
от них. Дифф индекса: `scripts/compose-lint.sh`, `testdata/compose-lint/` (41 фикстура, два парных `.env`,
README), `shared/env/`, `.env.example`, `Makefile`, `.github/workflows/go.yml`, `infrastructure.md` §3.1.1.
Вне области задачи в индексе ничего нет. Прочитаны: свой раздел «T-413 · ревью #1», «Итерация 2 (по
ревью #1)» в карточке, записи T-413 в конце `dev-log.md`. N-1, N-3, параллельный прогон фикстур и литерал
`OLLAMA_*` оркестратор перенёс в T-429, здесь они не проверялись.

Всё запускалось в изолированной копии из экспорта индекса (`git checkout-index -a` в каталог из `mktemp -d`
в scratchpad, плюс `git init`, чтобы работал `git grep` правила 3). Мутанты — копиями скрипта в `scripts/`
копии, генератор проверял, что якорь найден ровно один раз. Копия удалена по точному пути. Рабочее дерево
не менялось: `git diff --quiet` по файлам T-413 → 0. Стенд владельца: только `config` и `up --dry-run`,
печатались коды и хеши, `.env` не открывался.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 2.

Ma-1 и все три Minor закрыты, это подтверждено повтором зондов ревью #1 и своими мутантами. Законные формы
ревью T-411 проходят, `${COMPOSE_PROJECT_NAME}` снова законна. Самопроверка отказывает во всех подложенных
случаях с понятным текстом. Хеши модели владельца совпадают. Два Nit касаются силы фикстур: код верный, но
два правдоподобных ослабления выживают на фикстурах. Правка — по строке-две в фикстурах, можно внести при
коммите.

### Статус замечаний ревью #1

| № | Статус | Доказательство |
|---|---|---|
| Ma-1 | **закрыто** | Зонды ревью #1 повторены точно: `MINIO_ROOT_PASSWORD : hunter2-literal` → правило 3 «carries the literal»; `MINIO_ROOT_USER :` → правило 3 «passed through with no value»; `OLLAMA_KEEP_ALIVE :` → правило 8. Дополнительно: несколько пробелов, табуляция, `"MINIO_ROOT_PASSWORD" :`, `MV_LLM_API_KEY : sk-…`, `MINIO_ROOT_USER      :`, `OLLAMA_KEEP_ALIVE    :` — всё отвергается своим правилом. Остальные места с шаблоном ключа проверены грепом и зондами. `LISTED` (`- KEY =v`, в том числе в кавычках) — compose v5.2 отказывает сам («declared with a trailing space»), линтер громко краснеет правилом 7. `raw_images` при `image :` у стороннего образа (qdrant) — громкий ложный отказ правила 1, у HEAD так же; у собственного образа (`multiverse-core/…`) строка источника не нужна. `TOP_LEVEL`/`BLOCK` дают только имя места в сообщении, правило 5 `OLLAMA_HOST\s*[:=]` пробел допускает. Молчащих мест нет |
| Mi-1 | **закрыто** | `STACK: ${COMPOSE_PROJECT_NAME}` и `PROF: ${COMPOSE_PROFILES:-whatever}` → ok. Проверка `OLLAMA_*` не выключилась: `OLLAMA_NUM_PARALLEL: ${OLLAMA_NUM_PARALLEL:-4}` → правило 8 «defaults to '4' here and to '1'», то же с `KEY :` и для `OLLAMA_ORIGINS:-http://127.0.0.1`. Свой мутант r4 (`EXTERNAL_PREFIX = "OLLAMA_K"`) убит двумя фикстурами (`bad-ollama-default`, `bad-ollama-nodefault`). Из сверки выпали только `COMPOSE_*`: у `MINIO_ROOT_*`/`NEO4J_PASSWORD` умолчание пустое, они выпадали и раньше |
| Mi-2 | **закрыто** (с N-2 ниже) | q1 `SOME: "x #${MV_CORE_ADMIN_CLIENTS}"` → правило 8, q2 `FOO: ${FOO_UNSET:-${MV_WORLD_ID:?need it}}` с `.env.example` → правило 7. Фикстуры `bad-comment-in-quotes`, `bad-required-nested` на месте |
| Mi-3 | **закрыто** | Пустой каталог → 1, «holds 0 bad and 0 good fixtures». Нет каталога → 1, «does not exist». Только «плохая» или только «хорошая» → 1, «holds 1 bad and 0 good» / «0 bad and 1 good». Свой мутант r2 (`\|\|` → `&&` в проверке нулей) эти два случая зеленит: проверка работает |
| N-2 ревью #1 | **закрыто** | Две строки `expect-rule` → 1, «has 2 '# expect-rule:' lines». `bad-x.yaml` и `good-x.yaml` → 1, «is not read — fixtures end in .yml». Чужая `expect-text` → 1, строки «as it must be» для этой фикстуры нет. Контроль (одна верная «плохая» и одна «хорошая») → 0, «fixtures ok — 1 bad, 1 good» |
| N-1, N-3 ревью #1 | переданы в T-429 | формулировка про блочные скаляры в карточке уточнена верно |

### Замечания

#### Critical, Major, Minor

Нет.

#### Nit

**N-1. `testdata/compose-lint/bad-secret-spaced-colon.yml:14-15`, `bad-ollama-spaced-colon.yml:13`:
«плохие» фикстуры Ma-1 проверяют ровно один пробел, а выравнивание в столбик — это несколько.** Мутант r1
(`\1[ \t]*:` → `\1[ \t]?:` в `MAPPED`, `compose-lint.sh:544`): настоящие файлы и 41 фикстура ok, но зонды
`MINIO_ROOT_PASSWORD      : hunter2`, `MINIO_ROOT_USER      :` и `OLLAMA_KEEP_ALIVE    :` проходят молча. Это
та самая форма, из-за которой Ma-1 был Major. «Хорошая» `good-spaced-colon.yml` выравнивает несколькими
пробелами, но на «хорошей» ослабление не видно: невидимый ключ не нарушает ничего.
*Как исправить:* в `bad-secret-spaced-colon.yml` одну из двух строк выровнять несколькими пробелами (обе
`expect-text` уже различают `USER` и `PASSWORD`, так что r1 потеряет одну из них), в
`bad-ollama-spaced-colon.yml` — несколько пробелов.

**N-2. `scripts/compose-lint.sh:524` (`strip_comment`): ветка одинарной кавычки без фикстуры.** Мутант r5
(`c in "\"'"` → `c in "\""`): настоящие файлы и 41 фикстура ok, зонд `SOME: 'x #${MV_CORE_ADMIN_CLIENTS}'`
проходит молча, хотя без мутанта отвергается правилом 8. `bad-comment-in-quotes.yml` покрывает только
двойные кавычки.
*Как исправить:* отдельная «плохая» фикстура с одинарными кавычками (`# expect-rule: 8`). Строку в ту же
фикстуру добавлять бесполезно: двойные кавычки дадут тот же текст отказа, и мутант выживет.

### Что проверено экспериментом

- **Зонды** (17 файлов, прогон с абсолютными путями): Ma-1 ×9, Mi-1 ×3 плюс две законные формы
  `COMPOSE_*`, q1, q1 в одинарных кавычках, q2. Законные формы ревью T-411 одним файлом — `MV_X:`,
  `MV_X: null`, `- MV_X`, `${MV_MEMORY_URL:-}`, `${COMPOSE_PROJECT_NAME}`, `MV_CORE_URL:-http://core:8090`,
  `MV_WORLD_ID : ${MV_WORLD_ID:-dark-forest-world}`, ключ без значения с хвостовым комментарием, `$${MV_X:-d}`
  в `command` → ok.
- **Самопроверка** (`COMPOSE_LINT_FIXTURES` на свои каталоги): 9 случаев, результаты — в таблице выше.
- **Мутанты линтера (свои, 7)** — на настоящих файлах, на `--fixtures`, на 17 зондах и на двух ST:

  | Мутант | Настоящие | `--fixtures` | Итог |
  |---|---|---|---|
  | m0 контроль: синтаксическая ошибка во второй Python-половине | красные | 1: 32 «no rule at all» (две фикстуры правила 7 краснеют в bash-половине), 7 «хороших» отвергнуты; все зонды 1 | контроль работает |
  | r1 `[ \t]*` → `[ \t]?` в `MAPPED` | ok | ok | **выжил** — N-1; молчат 3 зонда с несколькими пробелами |
  | r2 проверка нулей `\|\|` → `&&` | ok | ok | ST «только плохие» и «только хорошие» → 0: убит самопроверкой |
  | r3 расхождение умолчания `OLLAMA_*` идёт в ветку «вне набора сетевых» | ok | ok | выжил: отказ громкий, совет верный, но причина неприменима («not in the explicit set of network addresses»). Ветка итерации 1, не исправление этой итерации — не замечание |
  | r4 `EXTERNAL_PREFIX = "OLLAMA_K"` | ok | 1: `bad-ollama-default`, `bad-ollama-nodefault` зелёные | убит |
  | r5 `strip_comment` не видит `'` | ok | ok | **выжил** — N-2; молчит только q1 в одинарных кавычках |
  | r9 тождественный | ok | ok | чистый |

  Мутанты шли параллельно, около 7 минут на все.
- **Вне повторного ревью, для T-429:** `MV_LLM_API_KEY:` с литералом на следующей строке (многострочный
  plain-скаляр) проходит молча — и у нового линтера, и у HEAD (`git show HEAD:scripts/compose-lint.sh`).
  Класс N-1 ревью #1 (разбор на уровне YAML), не регрессия.
- **Обязательные прогоны** (go1.26.8 windows/amd64):
  - `make compose-lint` → 0 за 81 с: 15 сервисов в 3 файлах, «fixtures ok — 34 bad, 7 good»;
  - `go run ./cmd/mvctl env check` → 0, 67 переменных;
  - `go build ./... && go vet ./...` → 0;
  - `go test -short -count=1 ./shared/env/... ./cmd/mvctl/...` → 7 пакетов ok;
  - `golangci-lint run ./shared/env/...` → 0 issues;
  - у владельца, `COMPOSE_ENV_FILES=.env,build/versions.env`: `config -q` → 0, хеши `config --format json`:
    без профиля `0870f33d758b0688`, `gpu` `ddc89274a7f7e5b8`, `gpu+memory+dev` `80c0aac4ebfcbcd7` — равны
    заявленным; `up --dry-run --no-build --pull never` → 0;
  - `gitleaks git --staged --redact .` → no leaks found.

### Предложения в бэклог

1. В T-429: добавить к списку расхождений на уровне YAML многострочный plain-скаляр (`KEY:` и значение на
   следующей строке) — для секретов вне `MUST_BE_REQUIRED` это молчаливый литерал.
2. В T-429 (по желанию): в ветке расхождения умолчания для `OLLAMA_*` мутант r3 выживает, потому что
   `expect-text` у `bad-ollama-default` совпадает с общей частью обоих текстов. Добавить `expect-text` с
   «contracts.md §16 p. 5».

## T-428 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Незакоммиченные правки в `.worktrees/EPIC-001`, ветка `task/T-428-docs-after-membus`: 12 файлов в `Docs/`,
`git diff --stat`: +59/−17, других файлов нет. Прочитаны раздел «### T-428» в `tasks.md`, раздел выполнения в
карточке, запись tech-writer#1 в `dev-log.md`. Факт сверен по дереву: `shared/eventbus/membus/` есть,
`shared/testkit/membus` нет; не-тестовый импорт `shared/testkit` в `cmd/multiverse` — только
`fake_contexts.go` (`testkit/swarm`); `.golangci.yml`: `no-testkit-in-production` с исключением одного
`fake_contexts.go`, `cmd-multiverse-fake-contexts` — ровно `shared/testkit/swarm$`; `testkit.Dedup` — псевдоним
(`shared/testkit/testkit.go:24`); `shared/testkit/contract` гоняет `membus` (`membus_test.go`) и Redpanda.

Свой грэп по `Docs/`: `testkit/membus`, `testkit.membus`, `testkit/{…membus…}`, `testkit` и `membus` в одной
строке, «до переезда», «после переезда», «переезжает», «импортов два», `bus_memory`, «единственный импорт».
Записями прошлого считаю `dev-log.md`, `review.md`, `journal.md`, карточки задач, ADR с датой, сводки
изменений версий, `consolidation.md`, `decomposition-review.md` (своя пометка tech-lead#1 в строке 6),
§14 `gateway-and-bot.md` (в заголовке «история»), закрытый M0 в `roadmap.md:53`, чек-лист F-5t.

### Вердикт

**ВЕРНУТЬ** — Critical 0, Major 1, Minor 4, Nit 2.

Замены из списка задачи верны. Путь `shared/eventbus/membus`, «вторая реализация C-01, а не заглушка» и
«единственный импорт `shared/testkit` в бинарнике — хук T-255» совпадают с деревом и `.golangci.yml`. Записи
прошлого не переписаны, только помечены. Спорные решения (а)–(в) приняты, разбор ниже. Вернуть из-за полноты:
DoD проверялся только литеральным грэпом `testkit/membus`. Он пропустил две строки действующего текста, которые
по-прежнему кладут `membus` в `testkit`, и одна из них противоречит соседней строке того же дерева.

### Спорные решения исполнителя

- **(а) `plan/decomposition-review.md` — запись прошлого.** Верно: в строке 6 пометка tech-lead#1 «Это запись
  ревью на 2026-09-09, текст не переписывается». Строки 86, 108 и 149 с `membus` в `testkit` тоже часть этой
  записи, их не трогать правильно.
- **(б) mermaid в `c4-component-foundation.md`.** Синтаксис цел: скобки и кавычки в строках 22, 31 и 50 парные,
  внутри описаний нет двойных кавычек, узлы `tk` и `eb` в `Rel` объявлены. Смысл тоже держится. «Что здесь
  будущее» больше не обещает переезд. Строка 7 «Расхождений» говорит то же, что диаграмма. Новый текст `Rel`
  совпадает с кодом: `contract` импортирует `eventbus`, его тесты гоняют `membus` и Redpanda. Про авторство
  строки 7 — Mi-3.
- **(в) Документы system-architect и tech-lead.** Все правки фактологические, ни одно решение не изменено:
  приведён только путь и время глагола («переезжает» → «переехал»). Версии документов при этом не отмечены — Mi-4.

### Замечания

#### Major

**Ma-1. Действующий текст по-прежнему кладёт `membus` в `shared/testkit`, а литеральный грэп DoD этого не
видит.**
- `architecture/components/foundation.md:36` — дерево §1: `testkit/  membus, Dedup-псевдоним, harness-каркас,
  фикстуры…`. Строкой 27 выше в том же дереве `membus/` уже стоит под `eventbus/` (правка T-418), так что §1
  противоречит сам себе. T-418 передал «остальные документы архитектора» в T-428 (`T-418.md:70`).
- `epics/EPIC-004-gateway-bot/design.md:127` — «читаем, но не меняем: … `shared/testkit/{membus,state,swarm}`».
  Это действующий дизайн эпика, который ещё не стартовал, и в нём старый путь импорта. Та же категория, что и
  `EPIC-003/design.md:142` из списка задачи, но фигурные скобки прячут строку от грэпа `testkit/membus`.

*Как исправить:* `foundation.md:36` — убрать `membus` из строки `testkit/`, по образцу компонента `testkit` на
диаграмме (`contract`-тест шины, `Dedup`-псевдоним, `Versions()`, `containers`, фикстуры; фейки — в
подпакетах). `EPIC-004/design.md:127` — `shared/eventbus/membus`, `shared/testkit/{state,swarm}`. В карточке в
«Итоговый грэп» добавить расширенный грэп (`testkit/\{`, `testkit` и `membus` в одной строке) и его разбор.

#### Minor

**Mi-1. Критерий готовности EPIC-001 «`testkit` содержит membus + v0 заглушек» в действующих документах:
`plan/epics.md:52`, `architecture/components/foundation.md:367`, `epics/EPIC-001-foundation/design.md:165`.**
После T-418 он буквально не выполним, а приёмка EPIC-001 ещё впереди. Менять формулировку критерия — решение
tech-lead#1, поэтому только пометка.
*Как исправить:* рядом с фразой — «(с T-418 `membus` — в `shared/eventbus/membus`)». `roadmap.md:53` — M0 закрыт,
это запись прошлого, не трогать.

**Mi-2. `architecture/contracts.md:190`: «на момент переезда — 14 импортёров».** Исполнитель превратил
плановое число T-416 в утверждение о факте, а T-418 насчитал 15: 14 тестов и `bus_memory.go`
(`T-418.md:35`, «Отклонения»).
*Как исправить:* «на момент переезда — 15 импортёров: 14 тестов и `cmd/multiverse/bus_memory.go`» или убрать
число.

**Mi-3. Пометки, написанные T-428, подписаны T-418: `architecture/adr/ADR-001-modular-monolith-topology.md:51`
(«Пометка (T-418, 2026-09-11)») и `architecture/diagrams/c4-component-foundation.md:77` («закрыто (T-418…)» —
«диаграмма выше поправлена»).** T-418 диаграмму не правил. По образцу соседних записей (пометка T-420 в п. 8,
пометка T-416 в `decomposition-review.md:6`, строка 6 «закрыто (T-425)») подписывается задача, которая сделала
правку.
*Как исправить:* «Пометка (T-428, 2026-09-11; переезд — T-418)», «закрыто (T-428, 2026-09-11; переезд — T-418)».

**Mi-4. Правки в версионируемых документах не отмечены в их истории: `architecture/contracts.md` (C-01, «История
изменений», строка 194) и `plan/ownership.md` (шапка v0.5, строка 3).** Для таких текстовых правок в C-01 есть
образец: «v1.3a — … правка текста, Go-API не менялся» (T-410). Без отметки читатель шапки `ownership.md` v0.5
видит «переезжает… до переезда», а строки 10, 15 и 16 говорят «с T-418».
*Как исправить:* C-01 — «v1.4a — 2026-09-11 (T-428): «Заглушка» и §17 приведены к T-418; решения и Go-API не
менялись». `ownership.md` — такая же одна фраза в шапке.

#### Nit

**N-1. `plan/ownership.md:16`: «Между T-410 и T-418 импортов `shared/testkit` в бинарнике было два».** Хук
`fake_contexts.go` добавил T-255 уже после T-410 (`T-410.md:50`: «`fake_contexts.go` ещё нет»), так что точнее
«между T-255 и T-418». Та же неточность есть в пометке T-420 в `ADR-001:47` — это запись прошлого, её не трогать.

**N-2. `ADR-001-modular-monolith-topology.md:51`: «Текст пункта — запись своего времени».** Пометка стоит над
всем дополнением, и о `membus` там три пункта (1–3); п. 4 о другом. Лучше «пункты 1–3 ниже — запись своего
времени».

### Гигиена

- `git diff --stat` — ровно 12 файлов из отчёта, случайных перезаписей нет (`numstat` от 1/1 до 35/1).
- Окончания строк: в рабочей копии у всех 12 файлов CRLF на каждой строке (`core.autocrlf=true`,
  `* text=auto`), смешения нет.
- Конец файла: у всех 12 ровно один перевод строки, пустой строки в конце нет.

### Предложения в бэклог

1. `AGENTS.md:118` (вне `Docs/`, вне области T-428): `shared/testkit/  # membus, contract-тест шины…` — старое
   место. При этом `T-418.md:23,35` пишет, что карта `AGENTS.md` поправлена. Строку поправить отдельной мелкой
   правкой.
2. System-architect: `c4-component-foundation.md:23` — `OwnershipRules - статичная копия таблицы владения`
   противоречит ADR-025: это единственная истина, копии нет (T-416). Не касается T-428.
3. System-architect, по желанию: заголовок C-01 «### Заглушка для потребителей» (`contracts.md:189`) не
   совпадает с текстом под ним («вторая реализация, а не двойник»).

## T-428 · ревью #2 · 2026-09-11 · code-reviewer#1

### Границы ревью

Повторное ревью после итерации 2 (tech-writer#1). Проверены только исправления по ревью #1 и регрессия от
них. Незакоммиченные правки в `.worktrees/EPIC-001`, ветка `task/T-428-docs-after-membus`: 18 файлов, среди
них `AGENTS.md` (вне `Docs/`, по предложению 1 ревью #1) и этот `review.md` (моя запись #1). Прочитаны
«Выполнение — итерация 2» и «Итоговый грэп, итерация 2» в карточке, запись итерации 2 в `dev-log.md`, дифф
по словам всех повторно правленных файлов. Расширенный грэп ревью #1 повторён: строки, где `testkit` и
`membus` стоят рядом, без уже помеченных.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 1, Nit 2.

Все семь замечаний ревью #1 закрыты. Расширенный грэп заявлений «`membus` лежит в `shared/testkit`» в
действующем тексте больше не находит. Остались три вида строк: записи прошлого, помеченные строки и строки,
где `membus` — шина, в которую публикует фейк, а не место пакета. Правки итерации 2 фактологичны. Решения
владельцев документов не изменены.

### Статус замечаний ревью #1

| № | Статус | Доказательство |
|---|---|---|
| Ma-1 | **закрыто** | `foundation.md:36`: в строке `testkit/` нет `membus`, есть `contract`-тест шины и отсылка «`membus` — под `eventbus/` выше (T-418)». §1 больше не противоречит строке 27. `EPIC-004/design.md:127`: `shared/eventbus/membus`, `shared/testkit/{state,swarm}`. В карточке есть расширенный грэп с разбором по категориям |
| Mi-1 | **закрыто** | Пометка «(с T-418 `membus` — в `shared/eventbus/membus`)» стоит у критерия в `plan/epics.md:52`, `foundation.md:367`, `EPIC-001/design.md:165`. Формулировка критерия не изменена. По решению оркестратора так же помечены `plan/epics.md:49,150`: там описан объём волны 0, и строки не противоречат строке 52 |
| Mi-2 | **закрыто** | `contracts.md:190`: «на момент переезда — 15 импортёров: 14 тестов и `cmd/multiverse/bus_memory.go`» — совпадает с `T-418.md:35` |
| Mi-3 | **закрыто** | `ADR-001…:51` — «Пометка (T-428, 2026-09-11; переезд — T-418)». `c4-component-foundation.md:77` — «закрыто (T-428, 2026-09-11; переезд — T-418)» |
| Mi-4 | **закрыто** (с Mi-1 ниже) | C-01: версия в заголовке блока (`contracts.md:142`) и запись в «Истории изменений» (`:194`): «решения и Go-API не менялись». `ownership.md:3`: фраза «Правка T-428 … редакционная правка текста, без изменения решений» |
| N-1 | **закрыто** | `ownership.md:16`: «Между T-255 и T-418» |
| N-2 | **закрыто** | `ADR-001…:51`: «Пункты 1–3 ниже — запись своего времени, не переписываются» |
| Бэклог 1 | **сделано** | `AGENTS.md:118`: `shared/testkit/  # contract-тест шины, фейки для тестов/e2e (membus — в shared/eventbus/)`. Совпадает с деревом (`shared/eventbus/membus/` есть, `shared/testkit/membus` нет) и с картой каталогов в `CLAUDE.md`. Строка 108 того же дерева уже называет `membus/` под `eventbus/`, противоречия нет |
| Бэклог 2–3 | переданы | в «Предложения в бэклог» карточки, область system-architect |

### Замечания

#### Critical, Major

Нет.

#### Minor

**Mi-1. `architecture/contracts.md:142,194`: номер «v1.4a» для редакционной правки, сделанной после v1.5.**
По образцу v1.3a буква означает редакционную правку текущей на тот момент версии: v1.3a шла после v1.3 и до
v1.4. Сейчас текущая версия C-01 — v1.5, а «v1.4a» по номеру выглядит как правка до неё. Два места ещё и
расходятся в порядке. В перечне версий заголовка (`:142`) она стоит между v1.4 и v1.5, то есть раньше v1.5.
В «Истории изменений» (`:194`) — после v1.5, последней записью. Читатель последней строки истории может
решить, что текущая версия — «v1.4a», а ссылки «C-01 v1.5» в других местах покажутся устаревшими. Довод
исполнителя «правился текст блока v1.4» понятен, но номер версии обозначает состояние всего контракта, а не
возраст правленого абзаца.
*Как исправить:* «v1.5a — 2026-09-11 (T-428): «Заглушка» и §17 приведены к T-418; решения и Go-API не
менялись». В заголовке — после v1.5. Не блокирует: история не теряется, Go-API не менялся.

#### Nit

**N-1. `plan/epics.md:49,150`: две скобки подряд — «(membus, contract-тест шины, fakes-каркас) (с T-418…)» и
«(обязателен до волны 1 — …) (с T-418…)».** Читается хуже, чем пометка в той же скобке через «;» или после
точки с запятой.

**N-2. Карточка, «Итоговый грэп, итерация 2», категория 2: `foundation.md:343` отнесён к «`membus` —
цель теста».** Узел mermaid `F5t[F-5 testkit: membus, contract-тест шины, фикстуры]` говорит, что `membus` —
часть `testkit`. Это план задач волны 0 (§12), то есть запись прошлого, категория 3, а не 2. Помечать строку
не обязательно. Но по тому же правилу оркестратора, по которому помечены `epics.md:49,150`, соседние строки
§12 (`:343` и помеченная `:367`) теперь говорят по-разному. Если помечать, то пометкой под диаграммой, а не
в метке узла.

### Гигиена

- Изменено ровно 18 файлов (`git status --porcelain`), `git diff --stat`: +227/−29, случайных перезаписей нет.
- Окончания строк: у всех 18 файлов CRLF на каждой строке (число `\r$` равно числу строк), смешения нет.
- Конец файла: у всех 18 ровно один перевод строки, пустой строки в конце нет.

### Предложения в бэклог

Новых нет. Пункты 2–3 ревью #1 записаны в карточке для system-architect.

## T-429 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Незакоммиченные правки в `.worktrees/T-429`, ветка `task/T-429-compose-lint-config`: 7 изменённых файлов
(`scripts/compose-lint.sh`, `testdata/compose-lint/README.md`, `bad-ollama-default.yml`, `.github/workflows/go.yml`,
`infrastructure.md` §3.1.1, `dev-log.md`, карточка) и 8 новых фикстур. Вне области задачи правок нет: файлы
композиции, `build/versions.env`, `.github/ci.env`, `.env.example`, `shared/`, `Makefile` не изменены. Прочитаны
карточка T-429 (причина, DoD, раздел выполнения), свои разделы «T-413 · ревью #1» и «#2», запись T-429 в
`dev-log.md`. После точки ветвления эпик принял T-415. Из файлов T-429 он трогает только `dev-log.md` и этот
`review.md`, в обоих — дописывание в конец, так что конфликт при слиянии тривиален.

Зонды и мутанты шли в копии рабочей папки, в своём подкаталоге scratch (`git init` для `git grep`), без
`-overlay`. Копия удалена по точному пути. Модель владельца читалась только через `config`, печатались коды и
хеши, `.env` не открывался. Решения оркестратора (время ~22 с, литерал `OLLAMA_*` передан архитектору) —
не замечания.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 1, Nit 5.

Переход сделан полно. У правил 3, 7, 8 и у источника образа правила 1 не осталось построчного разбора YAML,
класс N-1 ревью #1 T-413 закрыт целиком. Каждая форма из списка — фикстура, и старый линтер ошибается на них так,
как заявлено. Интерполяция в скрипте совпала с compose v5.2 на всех моих зондах. Сырое чтение не тянет значения
ни из окружения процесса, ни из `env_file`. Minor: ослабленную проверку источника образа правила 1 не видит ни
одна фикстура, а этот путь задача как раз перевела на новое чтение. Правка — одна-две фикстуры, её можно внести
при коммите.

### Проверка по пунктам задания

1. **Полнота.** Правила 3, 7 (вторая половина), 8 и источник образа правила 1 читают `raw_models`
   (`compose-lint.sh:641`, `:846-851`, `:935`, `:1065-1067`, `:1350`). `strip_comment`, `unquote`, `MAPPED`,
   `entry`, `TOP_LEVEL`/`BLOCK` удалены. Что осталось на тексте и почему это не тот же класс:
   - правило 5, литерал `OLLAMA_HOST=0.0.0.0` «нигде в файле» (`:995-997`), — дополнение к проверке по модели
     (`:985-994`), которая читает `environment` уже после compose. Обойти его тем же классом можно только
     надуманно (`\x30.0.0.0` внутри `command`), замечанием не считаю;
   - правило 4, строки `env_file` (`:974-980`), дублирует модель: интерполированный `config` вливает `env_file`
     в `environment` (проверено зондом), и `TOKEN in env` срабатывает раньше;
   - `git grep minioadmin` — намеренно по тексту дерева;
   - `.env.example` и `build/versions.env` разбирает Python, но это dotenv, а не YAML; вне задачи.

   Правила 2, 4, 5 (модельная часть) и 6 читают интерполированную модель, то есть значения после YAML, и класс
   их не касается. Нормализация полей в сыром чтении подстановку не разрывает (13 зондов). `command` и
   `entrypoint`, записанные строкой, строкой и остаются, без shell-разбиения. `healthcheck.test`, `labels`,
   `extra_hosts`, `ports` — строки. `volumes` переводится в длинную форму, но `source` содержит `${…}`
   целиком. Вердикты старого и нового линтера на этих зондах совпали. Новый вдобавок ловит
   `MV_LLM_API_KEY: 1234567890`, который HEAD пропускал.
2. **Интерполяция.** 12 форм в одном файле: интерполированный `config` compose v5.2.0 сравнивался с разбором
   `interpolations()`. Результаты: `$$$X` → `$` и X; `${U:-$${Y}}` → литерал `${Y}`, закрывается на второй `}`;
   `${U:-a}b}` → `ab}`; `${U:-{"a":1}}` → `{"a":1}`. Для `${S:+x${S2}}`, `${U+$${Q}}`, `${U:-$S}`,
   `${U:-${S:-z}}`, `${S-$$}` и `a$${S}b${S}` границы и вложенность у скрипта те же. Ленивость `:?`/`?`: при
   заданной S compose пропускает `${S:?need ${U2:?inner}}` и `${S?m ${U3?x}}`. При пустой внешней переменной
   compose вычисляет сообщение и отказывает, причём по внутренней переменной, если пуста и она. Отказ случился
   бы в любом случае, так что не проверять сообщение — верно.
3. **Безопасность.** Сырое чтение файла с `MV_LLM_API_KEY:` (без значения) при той же переменной в окружении
   процесса и с `env_file`, где лежат маркеры: маркеров в выводе 0, ключ — `null`, `env_file` остаётся путём.
   Интерполированный `config` того же файла выдаёт оба маркера, так было и раньше, это правила 1–6.
   `COMPOSE_PROFILES`, `COMPOSE_FILE`, `COMPOSE_ENV_FILES`, `COMPOSE_PROJECT_NAME` в окружении не меняют вывод на
   настоящих файлах. Остаток — не T-429: окружение процесса по-прежнему перекрывает `--env-file` в модели и в
   «чистой машине» правила 7. Зонд: `MV_TELEGRAM_BOT_TOKEN`, выставленный в оболочке, делает первую половину
   правила 7 зелёной на `bad-required-outside-default`, отказ даёт только вторая. См. бэклог.
4. **`--profile '*'` и `--no-consistency`.** На v5.2 сырое чтение `docker-compose.yml` отдаёт 11 сервисов без
   профиля, с `gpu` и с `'*'`. Интерполированное — 6 без профиля и 11 с `'*'`. Эквивалентность m06
   подтверждаю. Мой k5 (без `--no-consistency`) на v5.2 тоже эквивалентен. Защита `:653-663` работает так:
   версия, которая фильтрует сырое чтение и не знает `'*'`, потеряет именно сервисы с профилями. Модель
   собрана с пятью профилями явно, разность окажется непустой, и скрипт выйдет с кодом 2 и текстом. Версия,
   которая не знает флага, отказывает уже в `config`, и скрипт выходит с кодом 1 и сообщением «cannot read … as
   written». Молчаливого пропуска по профилям я не вижу. Проверить другую версию здесь было нечем — см. риски.
5. **`-f`.** Несуществующий файл, каталог, `-f` без значения, второй `-f` на несуществующий файл → код 2 с
   именем. Относительный путь из подкаталога (`../scripts/…` и `compose-lint/x.yml`), абсолютный POSIX-путь вне
   репозитория, абсолютный путь Windows с `\`, относительный путь из каталога вне репозитория → 0.
6. **Параллельный `--fixtures`.** Набор из 7 фикстур, проваленных по-разному: не то правило, чужой
   `expect-text`, два правила, «хорошая» с отказом. При `COMPOSE_LINT_JOBS=1/4/32` — код 1, 25 строк, вывод
   побайтно одинаков. Временный каталог у каждого запуска свой (`mktemp -d`), общих файлов нет. Из прежних
   фикстур изменена только `bad-ollama-default.yml` (добавлен `expect-text`), остальные 35/7 проходят без
   правок.
7. **Хеши модели владельца** (`--project-directory` основной папки, его `.env` и `build/versions.env`): без
   профиля `0870f33d758b0688`, `gpu` — `ddc89274a7f7e5b8`, `gpu+memory+dev` — `80c0aac4ebfcbcd7`, `config -q` → 0.
   Совпадают с заявленными. Файлы композиции в T-429 и в основной папке побайтно одинаковы.

### Замечания

#### Critical, Major

Нет.

#### Minor

**Mi-1. `scripts/compose-lint.sh:846-876`, `testdata/compose-lint/`: проверку «сторонний образ берётся из
`build/versions.env`» не закрепляет ни одна фикстура.** Мутант k3 (`if not used & set(versions):` →
`if False and …`): настоящие файлы ok, 42/8 ok. Задача перевела источник этой проверки на сырое чтение, и
шапка (`:842-845`) заявляет, что `image :` и образ из якоря теперь находятся так же, как у compose. Но ни
«плохой», ни «хорошей» фикстуры на это нет: у правила 1 одна фикстура, `bad-latest.yml`, и она про тег. Пробел
был и у HEAD, но код правлен именно в этой задаче.
*Как исправить:* `bad-image-unpinned.yml` — сторонний образ литералом с тегом, `expect-rule: 1`,
`expect-text: does not come from build/versions.env`. Плюс «хорошая» фикстура: сторонний образ из переменной
`build/versions.env`, записанный как `image :` или через якорь.

#### Nit

**N-1. `:789-796` (`text_of`): числовой литерал секрета без фикстуры.** Поведение новое: `MV_LLM_API_KEY:
1234567890` → правило 3, а HEAD пропускал молча. Мутант k1 (`return str(value)` → `return None`) выживает на 42/8:
число снова читается как ключ без значения, и для секрета вне `MUST_BE_REQUIRED` это тишина.
*Как исправить:* отдельная фикстура `bad-secret-number.yml` с `expect-text: carries the literal '1234567890'` —
этот текст отличает её от фикстур со строковым литералом.

**N-2. `:675-688` (`note`/`flush`): схлопывание находки якоря без фикстуры.** Мутант k2 (каждое место —
отдельный отказ): 42/8 ok. При этом поведение заявлено в карточке и шапке.
*Как исправить:* в `bad-anchor.yml` добавить `expect-text` с двумя местами через «, » — в том виде, в каком их
выводит `flush`.

**N-3. `:330-343`: ложная строка самопроверки.** Если первую «хорошую» фикстуру линтер отверг и из корня,
самопроверка всё равно пишет «passes from the repository root but not as -f … from its own directory» — это
неправда (зонд с «хорошей» фикстурой, которая не проходит).
*Как исправить:* выводить проверку относительного пути, только если прогон той же фикстуры из корня прошёл,
либо брать для неё первую прошедшую фикстуру.

**N-4. `:1367` и совет `fix_for` (`:1221-1223`): «pass it through as a key with no value» получают и записи вне
`environment`** — элемент `command` вида `CLIENTS=${MV_X}`, `labels`, `build.args`. Шапка (`:100-102`, N-5
T-411) обещает для `command` другой совет. В блочной форме у HEAD так же, так что это не регрессия. Новое чтение
лишь распространило совет на поток-формы: у HEAD `["run", "CLIENTS=${…}"]` и `labels: {…}` получали «part of a
larger string». Вердикт верен, неверен только совет.
*Как исправить:* `whole` ставить только тогда, когда `item.where` лежит в `environment` сервиса.

**N-5. `:79-81`, шапка: оборванная строка** «…`--profile '*'`: every value / after YAML has / parsed it…».
После вставки флага абзац не переформатирован.

### Мутанты (свои, 6; в копиях, каждый — на настоящих файлах и на `--fixtures`)

| Мутант | Настоящие | `--fixtures` | Итог |
|---|---|---|---|
| k0 контроль: синтаксическая ошибка во второй Python-половине | 1 | 1: «no rule at all» у «плохих», «хорошие» отвергнуты | контроль работает |
| k1 `text_of`: число → `None` | 0 | 0, 42/8 | **выжил** — N-1 |
| k2 без схлопывания мест якоря | 0 | 0, 42/8 | **выжил** — N-2 |
| k3 правило 1 без проверки источника образа | 0 | 0, 42/8 | **выжил** — Mi-1 |
| k4 закрывающая `}` без счёта вложенных `${` | 0 | 1: `bad-default-escaped-brace`, `bad-default-nested-inner`, `bad-default-nested-outer`, `bad-required-nested` | убит |
| k5 сырое чтение без `--no-consistency` | 0 | 0, 42/8 | эквивалентен на v5.2 (как m06) |

### Замечание процесса (не код)

По сведениям оркестратора, исполнитель выполнил `rm -rf /tmp/tmp.*` в общем Temp. Это нарушение правила
«удалять только по точному пути своего каталога»: шаблон задевает временные каталоги других агентов и
процессов, в том числе параллельные прогоны самого линтера, который создаёт каталоги через `mktemp -d`. В
карточке (строка 107) написано «Копии удалены по точному пути», что с этим расходится. Сам я факт не проверял:
в файлах задачи следов нет. *Как исправить:* указать факт в карточке; впредь удалять только каталог, путь
которого сохранён в переменной при создании.

### Обязательные прогоны

- `make compose-lint` → 0 за 23,6 с (docker compose v5.2.0): 15 сервисов в 3 файлах, «fixtures ok — 42 bad,
  8 good», строка про относительный путь.
- `go run ./cmd/mvctl env check` → 0, 67 переменных.
- `bash -n scripts/compose-lint.sh` → ok. shellcheck не установлен.
- `gitleaks dir --redact` по скрипту и `testdata/compose-lint` → no leaks found.
- `git ls-files --eol`: скрипт и фикстуры — LF по `.gitattributes`.

### Риски и допущения

- Версия compose на раннере не пинуется. Проверена только v5.2.0 (машина владельца). На другой версии
  `--no-interpolate` может вести себя иначе: например, заполнить ключ без значения из окружения или влить
  `env_file`. Тогда правило 3 может громко отказать или процитировать значение в отказе. В нынешних файлах
  `env_file` нет, а окружение CI — это фиктивный `.github/ci.env` и пины. Худший исход, который я вижу, —
  красное задание. Шаг `docker compose version` в CI позволит сверить версию по первому логу.

### Предложения в бэклог

1. Изолировать интерполированную модель и «чистую машину» правила 7 от окружения процесса: запускать их с
   `env -u` для имён из `.env.example` и `build/versions.env`. Сейчас переменная, экспортированная в оболочке
   оператора, делает первую половину правила 7 зелёной. Вторая половина прикрывает не всё: пины
   `build/versions.env` из неё исключены (`:1071`), то есть случай пустого `CHROMA_IMAGE` (D-3) виден только
   первой.
2. N-4 — если его не исправят в этой задаче.

## T-429 · ревью #2 · 2026-09-11 · code-reviewer#1

### Границы ревью

Повторное ревью после итерации 2 (devops-engineer#1). Проверены только исправления по ревью #1 и регрессия от
них. Незакоммиченные правки в `.worktrees/T-429`: к файлам итерации 1 добавились `bad-anchor.yml` (изменён) и
четыре новые фикстуры — `bad-image-unpinned`, `good-image-pinned`, `bad-secret-number`,
`bad-nodefault-command-entry`. Прочитаны раздел «Итерация 2» в карточке, запись итерации 2 в `dev-log.md`,
изменённые места скрипта (`Item.parent`, `in_env` `:1373-1381`, самопроверка `:330-350`, шапка `:74-102`) и
строки README фикстур. Зонды и мутанты шли в каталогах, созданных `mktemp -d` в своём подкаталоге scratch.
Каждый каталог удалён по сохранённому пути, после удаления их 0.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 2.

Mi-1 и N-1…N-5 закрыты. Изменение поведения N-4 проверено зондами по всем формам записи: совет «ключ без
значения» остался у настоящих записей `environment`, а у `command`, `labels` и `build.args` он заменён верным
советом. Других регрессий в текстах отказов не нашёл. Оба новых Nit касаются силы фикстур, код верный.

### Статус замечаний ревью #1

| № | Статус | Доказательство |
|---|---|---|
| Mi-1 | **закрыто** | `bad-image-unpinned.yml`: образ из якоря и образ `image :`, оба `expect-text` называют сервис. `good-image-pinned.yml`: те же формы с пинами. Мутант k3 исполнителя убит, в README есть строки для обеих фикстур |
| N-1 | **закрыто** | `bad-secret-number.yml`, `expect-text: carries the literal '1234567890'`. k1 убит |
| N-2 | **закрыто** (с N-2 ниже) | Отказ на `bad-anchor`: `services.core.environment.NEO4J_PASSWORD, … x-extra-env.NEO4J_PASSWORD` — одна находка, два места. k2 убит |
| N-3 | **закрыто** | Зонд: одна «плохая» фикстура и «хорошая», которая не проходит, → код 1 и строка «was not checked … it does not pass from the repository root either (see above)». С исправной «хорошей» → «passed as -f … from its own directory», код 0. Индекс корневого прогона (`${#bads[@]}`) верен: запуск идёт по `bads`, затем по `goods` |
| N-4 | **закрыто** (с N-1 ниже) | Зонды, 13 файлов, у каждого места смотрел совет. «Pass it through as a key with no value»: `environment` сервиса в виде map, list и поток-формы; якорь `x-*` на верхнем уровне, влитый через `<<`; вложенный `x-common.environment`; якорь-список `environment: *envlist`; расхождение умолчания `${MV_LOG_LEVEL:-debug}` в `environment`. У якорей находка схлопнута в одну с двумя местами. «Part of a larger string»: `command` в блочной и поток-форме, `labels` в блочной форме, `build.args` списком, и `NOTE: "x ${MV_X}"` внутри `environment`, как и раньше. Совет для `OLLAMA_*` без значения вне `environment` не изменился (`write ${OLLAMA_X:-d}`). Мой мутант j2 (`in_env` без `.environment`) убит фикстурой `bad-nodefault-bare`. Регрессий текста не нашёл |
| N-5 | **закрыто** | Абзац переформатирован. У `:85` осталась одна короткая строка, это не замечание |
| Процесс | **закрыто** | Карточка, «Удаление временных файлов — поправка»: факт `rm -rf /tmp/tmp.*` признан, прежняя запись названа неправдой, порядок работы на будущее описан |
| Бэклог п. 1 | **передан** | В карточке, раздел «Бэклог» |

### Замечания

#### Critical, Major, Minor

Нет.

#### Nit

**N-1. `scripts/compose-lint.sh:1378-1379`: ветку `x-*` условия `in_env` не закрепляет ни одна фикстура.** Мой
мутант j1 (ветка `x-*` → `False`): настоящие файлы ok, 45/9 ok. Без этой ветки находка в якоре, влитом в
`environment`, раскалывается на две. Место в сервисе получает совет «ключ без значения», место в якоре — совет
длинной строки. Вердикт остаётся красным, но это то самое расхождение совета, которое закрывал N-4.
*Как исправить:* «плохая» фикстура правила 8 с `${MV_X}` в якоре `x-*` на верхнем уровне, который влит в
`environment`. `expect-text` — место сервиса с запятой (`environment.MV_CORE_ADMIN_CLIENTS,`): запятая
появляется, только если оба места получили один и тот же текст.
Попутно, без правки: якорь `x-*`, который целиком служит `labels` (`labels: *labels`), считается записью
`environment`. Место в якоре получает совет «ключ без значения», а место в сервисе — совет длинной строки.
Выходят две находки вместо одной. Форма надуманная, её нет ни в файлах, ни в планах.

**N-2. `testdata/compose-lint/bad-anchor.yml`, `expect-text: services.core.environment.NEO4J_PASSWORD,` зависит
от порядка мест.** Порядок задаёт compose: на v5.2 ключи верхнего уровня идут как `name`, `networks`, `services`,
`x-*` (проверено на `bad-anchor` и на своём зонде), поэтому место в сервисе всегда первое. На версии, которая
выведет `x-*` раньше, фикстура покраснеет ложно, но громко, а не молча. Молчаливого пропуска нет.
*Как исправить (по желанию):* `expect-text: NEO4J_PASSWORD, testdata/` — после схлопывания эта подстрока есть
при любом порядке, а без схлопывания (k2) её нет.

### Мутанты (свои, 4; в копии, каждый — на настоящих файлах и на `--fixtures`)

| Мутант | Настоящие | `--fixtures` | Итог |
|---|---|---|---|
| j0 контроль: синтаксическая ошибка во второй Python-половине | 1 | 1: «no rule at all» у «плохих» | контроль работает |
| j1 `in_env` без ветки `x-*` | 0 | 0, 45/9 | **выжил** — N-1 |
| j2 `in_env` без `.environment` | 0 | 1: `bad-nodefault-bare` — не та причина | убит |
| j3 индекс корневого прогона N-3 | — | — | недействителен: мутация задела только проверку существования файла, чтение шло из верного файла. N-3 проверен зондом (выше) |

### Обязательные прогоны

- `make compose-lint` → 0 за 20,8 с: 15 сервисов в 3 файлах, «fixtures ok — 45 bad, 9 good».
- Хеши модели владельца: без профиля `0870f33d758b0688`, `gpu` — `ddc89274a7f7e5b8`, `gpu+memory+dev` —
  `80c0aac4ebfcbcd7`. Совпадают с прежними.
- `go run ./cmd/mvctl env check` → 0, 67 переменных. `bash -n` → ok.
- `gitleaks dir --redact` по `testdata/compose-lint` и скрипту → no leaks found.

### Предложения в бэклог

Новых нет. Изоляция от окружения процесса записана в карточке.
## T-415 · ревью #1 · 2026-09-11 · code-reviewer#2

### Границы ревью

Ревью #1 задачи T-415 (S). Работа лежит незакоммиченной в `.worktrees/T-415` (ветка `task/T-415-silent-paths` от
`epic/EPIC-001-foundation`, HEAD `72a24ec`): 9 изменённых и 6 новых файлов, всё в области задачи. Прочитаны: раздел
«### T-415» в `tasks.md`, карточка T-415 (выполнение, отклонения, запись для dev-log, бэклог), ADR-023 п. 4, C-01 в
`contracts.md` (строки 156 и 169: `recover` в `StartAll` там уже предусмотрен), `ownership.md` (`shared/runtime` —
общий код), ревью-источники T-410 (N-1, N-3, M7c, R5b), T-220 (Mi-1 нарратора, EPIC-003), T-426 #1/#2. Пункт про
`.claude/hooks/go-fmt.ps1` исключён оркестратором и не проверялся.

Прогоны и мутанты — в копии рабочей папки (`git ls-files -co --exclude-standard` → подкаталог scratchpad), флак —
в выгрузке HEAD (`git archive HEAD`). Способ `-overlay` не применялся. После каждого мутанта файл восстановлен и сверен
`cmp` с рабочей папкой, итоговый `diff -rq` пуст. Копии удалены по точному пути. В рабочей папке T-415 менялись
только этот раздел и строка ревью в карточке. Живые процессы запускали только e2e-тесты: `--bus=memory`, порт от
`freeAddress`, сигнал — в группу дочернего процесса. Redpanda, контейнеры и LLM не трогались. `-race` недоступен (нет cgo).

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 1, Nit 4.

Все семь пунктов сделаны, на каждый есть тест, и тест краснеет на своём мутанте. Это подтверждено моими прогонами:
мутанты автора повторены, добавлены свои. Экспортируемый API не менялся ни в `shared/runtime`, ни в
`shared/eventbus`: добавлены только неэкспортируемые `start`, `panicText`, `deliverNext`, сигнатуры `StartAll` и
`Subscribe` прежние. Новое поведение `StartAll` (паника превращается в ошибку старта) C-01 уже описывает
(`contracts.md:156`, `:169`: «там `recover` даёт `runtime.StartAll`, T-415»), поэтому это **не contract-change** —
оценку оркестратора подтверждаю. Mi-1 — ложный след отказа публикации у `FakeEncounter` при штатной остановке. Его
лучше закрыть до коммита: правка на две строки плюс тест.

### Замечания

#### Minor

**Mi-1. `shared/testkit/swarm/fake_encounter.go:1532-1538` (`refusedPublication`): публикация, прерванная остановкой,
записывается как отказ шины.** Штатная остановка по ADR-023 п. 4 идёт через отмену контекста вызывающего:
`FakeContext.Stop` зовёт `cancel()`, затем `Wait()` (`fake_context.go:184-185`). Если в этот момент обработчик
публикует, `membus.Publish` возвращает `context.Canceled` (`membus.go:206`). `refusedPublication` без разбора пишет
`ERROR «publication refused by the bus»` и кладёт ошибку в липкий `failure`. Тогда `Wait()`, а за ним
`FakeContext.Stop`, возвращают `testkit/swarm: publish …: context canceled`, а `serve` пишет в лог строку `stop`
уровня `ERROR`. Doc-комментарии `Wait` (`:562-563`) и `FakeContext.Stop` (`fake_context.go:171-173`) обещают на
чистой остановке `nil`. До T-415 этот путь молчал правильно: `Delivery` при отменённом контексте
возвращала ошибку, `membus` превращала её в `nil`, `subErr` не трогался.
Зонд P1 (в копии): `openFight`, затем `Act` с отменённым контекстом → `Act = testkit/swarm: publish dice.rolled:
context canceled`, `Err() = то же`, в логе `publication refused by the bus`.
*Как исправить:* в `refusedPublication` передавать `ctx` и при `ctx.Err() != nil` не звать `e.fail`, а
писать строку уровня `Debug`/`Info` («publication interrupted by the stop») и возвращать ошибку как раньше. Тест:
`Act` с отменённым контекстом → `Err() == nil`, в логе нет `ERROR`. Нарратор (`fake_narrator.go:815-817`) на отмене
тоже пишет `ERROR`, но в `Err()` ничего не кладёт; его правка вне задачи (бэклог EPIC-003).

#### Nit

**N-1. `fake_encounter.go:1327` — ветка `encounter.ended` в `announce` без теста.** Мутант MA (в этой ветке
`refusedPublication` заменён на прежний `fmt.Errorf`) выживает на `shared/testkit/swarm` и `cmd/multiverse`. Тесты
покрывают `publish` (`combat.decided`) и `encounter.started`, но не третий путь. *Как исправить:* третий кейс в
`fake_encounter_failure_test.go` — отказ `encounter.ended` на факте закрывающего пакета.

**N-2. `cmd/multiverse/serve_test.go:333-338` — проверка поля через `reflect` ловит переименование, но не смену
типа.** При переименовании поля сообщение понятное: «has no field skipValidateOnRead any more: follow the rename
here». Если поле станет не `bool` (например, уйдёт в структуру настроек), `field.Bool()` упадёт паникой `reflect:
call of reflect.Value.Bool on … Value` без подсказки. *Как исправить:* перед `Bool()` проверить
`field.Kind() != reflect.Bool` → `t.Fatalf` с тем же советом. Сам выбор `reflect` верный: экспортируемый геттер у
`eventbus.Kafka` был бы contract-change.

**N-3. `test/e2e/lifecycle_test.go:62` — сигнал отправляется без проверки, что процесс ещё жив.** Если дочерний
процесс умер между ответом `/health` и `interrupt`, `GenerateConsoleCtrlEvent` получит id группы, лидера которой
уже нет. Задеть чужие процессы при живом дочернем тест не может: группа создана `CREATE_NEW_PROCESS_GROUP`, её id
равен pid дочернего, событие идёт только в неё. Прогон дал PASS, не SKIP. *Как исправить:* перед `interrupt`
вставить `select { case <-proc.exited: t.Fatalf(…код выхода…); default: }` — дёшево и снимает вопрос целиком.

**N-4. `.golangci.yml:156` (`internal-state`), `:179-181` (`internal-swarm`) — та же дыра по префиксу, её проще
закрыть сейчас, чем держать в бэклоге.** Мутант: пакет `internal/mechanicsx` и импорт из поддельного `internal/state`
при текущем конфиге проходят. С `internal/mechanics$` в `internal-state` получаем `depguard` «not allowed from list
'internal-state'». Цена нулевая: пакетов `internal/state` и `internal/swarm` ещё нет, у `internal/mechanics` нет
подпакетов, ломать нечего. У `internal-swarm` так же открыты `internal/laws` и `internal/llm`. Если контексту позже
понадобятся подпакеты, их добавят парой `…/mechanics$` + `…/mechanics/` осознанно. Правка в том же файле, что уже
меняет T-415. Если оркестратор не хочет расширять задачу — оставить в бэклоге, как предложил автор.

### Ответы на вопросы поручения

1. **`StartAll`.** Паника в `Routes` и в `Start` превращается в ошибку `start context <имя>: panic: <значение>`.
   Запущенные до неё контексты останавливаются в обратном порядке, упавший не останавливается: мутант MS
   (`contexts[:i+1]`) краснеет, двойного `Stop` нет — `run` после ошибки `StartAll` только возвращает её, а
   `closeBus` отложен первым. Лента `start first|stop first|bus closed` закреплена тестом `serve_test`. Стек только
   в логе: мутант MK (стек пустой) краснеет, тест проверяет, что в ошибке нет `goroutine`. `panic(nil)` и
   непечатаемое значение покрыты. **Паника в `Stop` во время отката** (зонд P2): уходит из `StartAll`, контексты
   до паникующего не останавливаются (`stopped=` пусто), `closeBus` в `run` выполняется при раскрутке. Это не
   регрессия: `StopAll` на обычной остановке ведёт себя так же, и задача про панику в `Start`. Бэклог п. 2.
   `runtime.Goexit` в `Start` (только в тестах) `recover` не видит — так устроен Go, замечанием не считаю.
2. **`membus.deliverNext`.** Единственный `defer g.mu.Unlock()`, явных `Unlock` нет, двойного снятия нет. `Close`
   берёт только `st.mu` и `g.mu` не трогает, как и раньше. Порядок прежний: `changed` берётся под блокировкой до
   `t.at`, курсор двигается только после учтённой доставки. После `Goexit` курсор стоит, событие получает следующая
   подписка группы (at-least-once, C-01). Публичный API `shared/eventbus` не менялся, contract-change нет. Мутант
   MG (явные `Unlock` без `defer`) → «the group is stuck» за 5 с.
3. **`FakeEncounter`.** Правка минимальная: отдельный `failMu` (порядок блокировок `mu → failMu`, обратного пути
   нет), три точки через `refusedPublication`. Липкий `Err()` → `/health fail` до перезапуска — для двойника
   приемлемо и честно, пакет мог потеряться. Отличие от нарратора осознанное: у нарратора после итерации 2 T-220
   запоминание идёт после публикации, повтор восстанавливает ход, в `Err()` ничего нет. У встречи запоминание
   раннее (иначе повтор бросит кости второй раз), поэтому след и нужен. Правку принятого файла EPIC-003 подтверждает
   tech-lead#2 — автор это отметил. Липкость нужно ограничить Mi-1: сейчас она срабатывает и на штатной остановке.
4. **e2e.** Сигнальный тест чужие процессы задеть не может (см. N-3). Во флаки по порту не верю:
   `freeAddress` — прежний приём с известным коротким окном, финальный `DialTimeout` после выхода получает отказ
   соединения. Проба с `exited` отказывает быстро: `TestTheProbeGivesUpOnAProcessThatExited` — 3,8 с вместе со
   сборкой; мутант MP (без ветки `exited`) — красный, но за 17,9 с. `p.err` и `ProcessState` читаются после
   `<-exited`, гонки нет.
5. **depguard `$`.** Мутант ML: `internal/mechanicsx` из обоих двойников → 2 × `depguard` со своими `desc`. Контроль
   (без `$`) — 0 issues. По `internal-state`/`internal-swarm` см. N-4.
6. **M7c через `reflect`.** Мутант M7c → `value "false": … = false, want true`. Хрупкость — N-2.
7. **Сторож `%w`.** Мутант MW (`(cause: %v)` → `%w`) краснеет на всех четырёх подкейсах с понятным текстом
   («the kafka Subscribe would return nil and drop the consumer»). Предостережение ревью #2 T-426 закреплено.

### Что проверено экспериментом

| # | Мутант | Результат |
|---|---|---|
| M0 | контроль: `func broken( {` в конце `lifecycle.go` | красный, сборка |
| MR | `r := recover()` → `var r any` | красный: `runtime` («StartAll let the panic out»), лента `serve_test` |
| MS | откат `contexts[:i]` → `contexts[:i+1]` | красный: «stopped [mechanics state], want [state]» |
| MK | стек в логе пустой | красный: «the panic left no error with its stack in the log» |
| MG | `deliverNext` без `defer`, явные `Unlock` | красный: «the group is stuck» (5 с) |
| MW | `(cause: %v)` → `(cause: %w)` | красный, 4 из 4 подкейсов |
| M7c | `cfg.SkipValidateOnRead = false` перед `NewKafka` | красный |
| ME | без `e.fail(wrapped)` | красный: оба теста следа |
| MA | ветка `encounter.ended` без `refusedPublication` | **выжил** — N-1 |
| MP | проба без ветки `exited` | красный, 17,9 с |
| R5b | `serve` зовёт `run` без `withSignals` | e2e красный: `exit code 3221225786 (0xc000013a)`; юнит-тесты `cmd/multiverse` зелёные (как и заявлено) |
| ML | `internal/mechanicsx` из двух двойников | 2 × depguard; контроль без `$` — 0 issues |
| ML-s | то же из `internal/state`, текущий конфиг / с `$` | проходит / depguard `internal-state` — N-4 |

Зонды: P1 (`Act` с отменённым контекстом → липкий `Err`, `ERROR` в логе) — Mi-1; P2 (паника в `Stop` при откате
уходит наружу, `stopped` пуст) — бэклог п. 2.

**Флак `TestTheFactsOfStateMoveTheVersionTheStubProposesAgainst`** воспроизведён на выгрузке HEAD без правок T-415:
`-count=40` → `index out of range [-1]`, `fake_encounter_test.go:853`. К T-415 отношения не имеет: тест не менялся,
T-415 не трогает порядок публикаций на успешном пути. Причина в самом тесте (`:852`): `settled` истинно при
`decided > before` и `len(AppliedProposals()) >= len(proposals)` = `0 >= 0`, когда решение уже опубликовано, а
предложение ещё нет. Затем `proposals[len(proposals)-1]` падает. Правка — добавить в `settled` условие
`len(proposals) > 0`. Файл EPIC-003, бэклог п. 1.

**Прогоны в копии** (go1.26.8 windows/amd64, golangci-lint 2.13.2):
- `go build ./... && go vet ./... && go vet -tags e2e ./test/...` — 0;
- `go test -short -count=1 ./...` — 27 пакетов ok, 0 FAIL (флак на этом прогоне не выпал);
- `go test -tags e2e -count=1 ./test/...` — `test/e2e` ok (13 с): 4 теста PASS, `TestTheProcessStopsCleanlyOnASignal`
  выполнен (PASS, не SKIP); `test/fixtures` ok;
- `golangci-lint run ./...` — 0 issues.

### Предложения в бэклог

1. EPIC-003 (tech-lead#2): флак `fake_encounter_test.go:852-853` — `len(proposals) > 0` в `settled`.
2. system-architect (`shared/runtime`): паника в `Stop` — и при откате `StartAll`, и в `StopAll` на обычной
   остановке — уходит наружу, оставшиеся контексты не останавливаются, шина закрывается при раскрутке. Если это
   важно для ADR-023 п. 4, нужен `recover` в `StopAll` по образцу `start`: ошибка `stop context <имя>: panic: …`,
   стек в лог, остальные контексты продолжают останавливаться.
3. EPIC-003: нарратор на отменённом контексте пишет `ERROR «narrative not published»` (`fake_narrator.go:815-817`) —
   тот же ложный след, что в Mi-1, только в логе.
4. EPIC-003: позднее запоминание у `FakeEncounter` вместе с идемпотентностью полуопубликованного хода (п. 2 бэклога
   автора) — подтверждаю.

## T-415 · ревью #2 · 2026-09-11 · code-reviewer#2

### Границы ревью

Повторное ревью после итерации 2 (developer#1). Проверены только исправления по ревью #1 (Mi-1, N-1…N-4) и
флак EPIC-003, который оркестратор включил в итерацию, плюс регрессия от них. Работа по-прежнему незакоммичена в
`.worktrees/T-415`, по сравнению с итерацией 1 добавился один изменённый файл, `fake_encounter_test.go`. Прочитаны:
свой раздел «T-415 · ревью #1», раздел «Итерация 2» и запись для dev-log в карточке, дифф `.golangci.yml`,
`fake_encounter.go` (`refusedPublication`), `fake_encounter_failure_test.go`, `fake_encounter_test.go`, `serve_test.go`,
`lifecycle_test.go`, для вопроса о гонке в Mi-1 — `Deliver` в `shared/eventbus/delivery.go`.

Прогоны и мутанты — в новой копии рабочей папки (`git ls-files -co --exclude-standard` → scratchpad), серии флака на
HEAD — в выгрузке `git archive HEAD`. Первым шёл контрольный мутант. После каждого мутанта файл восстановлен и сверен
`cmp` с рабочей папкой, итоговый `diff -rq` пуст, копии удалены по точному пути. В рабочей папке менялись только этот
раздел и строка в карточке. `.worktrees/T-429` и основная папка не открывались.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 0.

Все пять замечаний ревью #1 закрыты, каждое подтверждено моим мутантом или повтором зонда. Правка флака строже
поручения, и это правильно: что проверяет тест, она не ослабляет, а гонку ожидания закрывает с обеих сторон. Гонка
в Mi-1 существует (зонд P3), но для двойника безвредна: событие остаётся незакоммиченным и приходит снова.

### Статус замечаний ревью #1

| # | Статус | Доказательство |
|---|---|---|
| Mi-1 | **закрыто** | `fake_encounter.go:1538-1549`: при `ctx.Err() != nil` — строка `Info` «publication interrupted by the stop», без `e.fail`, ошибка возвращается обработчику. Зонд P1 ревью #1 теперь даёт `Err() = nil` и ни одного `ERROR`. Тест `TestAPublicationCutShortByTheStopLeavesNoTrace` покрывает `Act` и `Observe` (announce). Мой мутант MI (ветка → `if false`) красный на обоих подкейсах: «Err() = … context canceled … want nil» |
| N-1 | **закрыто** | `TestARefusedEndLeavesATrace`; мутант MA ревью #1 (прежний `fmt.Errorf` в ветке `ended`) теперь красный: «Err() = <nil>, want the refusal of encounter.ended» |
| N-2 | **закрыто** | `serve_test.go:337-339`: `field.Kind() != reflect.Bool` → `t.Fatalf` с советом до вызова `Bool()`. Проверено чтением, мутант автора с `int` понятен |
| N-3 | **закрыто** | `lifecycle_test.go:62-68`: `select` по `proc.exited` → `t.Fatalf` до `interrupt`. Сигнальный тест выполнен (PASS, не SKIP) |
| N-4 | **закрыто** | `.golangci.yml`: `internal/mechanics$` у `internal-state`; `internal/mechanics$`, `internal/laws$`, `internal/llm$` у `internal-swarm`. Мутант: `internal/{mechanics,laws,llm}x` из поддельных `internal/swarm` и `internal/state` → 4 × depguard со своими `desc`. Контроль (все `$` сняты) — 0 issues |

### Флак `TestTheFactsOfStateMoveTheVersionTheStubProposesAgainst` (EPIC-003)

**Не ослабляет ли правка тест.** Нет. Что тест доказывает, решают итоговые проверки
(`fake_encounter_test.go:867-891`): State не отказал ни одному предложению (`TypeRejected` нет, значит, нет
version_conflict), бой закончился, версия волка ≥ 2, волк ранен, `dead_letters` пусты. Правка их не трогает.
Условие ожидания (`:841`, `:858-859`) только упорядочивает удары: следующий удар уходит, когда предложение этого
обмена опубликовано и применено. Это и есть сценарий теста — «следующее предложение делается против версии, которую
оставили факты». Раньше ожидание могло кончиться раньше срока. Строгое условие к тому же даёт `closes()` именно
предложение текущего обмена, а не прошлого. Обмен, который решён, но ничего не предложил, строгое условие не спрячет:
тест упадёт громко, по сроку `waitFor`.

**Статистика (мои серии, `-short -count=40 -run '…$'`):**

| Вариант | Серии | Красные |
|---|---|---|
| HEAD (выгрузка, без правки) | 2 | **2** (`index out of range [-1]`; серия обрывается на первой панике) |
| итерация 2 (строгое условие) | 4 | **0** из 160 прогонов |
| мутант MF: условие HEAD в копии (переменная оставлена через `_ =`) | 2 | **2** — серия ловит флак |
| мутант MF2: вариант `len(proposals) > 0` | 4 | 0 — таймаут автора (1 из 6) у меня не воспроизвёлся. Ранний выход по уже применённым прошлым предложениям виден из кода, строгое условие закрывает его логически, а не статистически |

С автором сходится: HEAD красный, правка — 0 красных. Правка файла EPIC-003 записана в карточке как
подтверждённая tech-lead#2.

### Mi-1: гонка «настоящий отказ в момент отмены»

Зонд P3: шина отказывает публикации `dice.rolled` по своей причине, а контекст обработчика в этот момент уже
отменён. Итог — `Act` = отказ шины (`errors.Is(err, errPublishRefused)`), `Err() = nil`, в логе `Info`
«publication interrupted by the stop» с текстом отказа, `ERROR` нет. Настоящий отказ при этом не попадает в
`Err()`. **Для двойника это приемлемо**, и потери здесь нет:
- `Delivery.Deliver` при ошибке обработчика и отменённом контексте возвращает `ctx.Err()` и не паркует
  (`delivery.go:121-125`). `membus` не двигает курсор, kafka не коммитит, событие приходит в следующем запуске.
  `acted` живёт в памяти, поэтому новый запуск ответит заново.
- `Err()` читает только `/health` и `Wait`/`Stop`, а после `Stop` `/health` у `FakeContext` и так `degraded`.
- Текст отказа остаётся в логе.
Более строгий вариант — `errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)` вместе с
`ctx.Err() != nil`. Он имеет смысл для настоящего агента, где `Err()` будет значить больше, и лежит в бэклоге EPIC-003
(п. 1 ниже), а не в замечаниях.

### Что проверено экспериментом

| # | Мутант | Результат |
|---|---|---|
| M0 | контроль: `func broken( {` в конце `fake_encounter.go` | красный, сборка |
| MI | ветка остановки в `refusedPublication` выключена | красный, 2 подкейса |
| MA | ветка `ended` без `refusedPublication` (выживал в ревью #1) | красный |
| MF | условие ожидания флака возвращено к HEAD | красный, 2 из 2 серий |
| MF2 | условие `len(proposals) > 0` | зелёный, 0 из 4 серий (см. выше) |
| N4 | `internal/{mechanics,laws,llm}x` из `internal/swarm`, `internal/state` | 4 × depguard; контроль без `$` — 0 issues |

Зонд P3 — выше. Прогоны в копии (go1.26.8 windows/amd64, golangci-lint 2.13.2):
- `go build ./... && go vet ./... && go vet -tags e2e ./test/...` — 0;
- `go test -short -count=1 ./...` — все пакеты ok, 0 FAIL;
- `go test -tags e2e -count=1 ./test/e2e/` — ok (12 с), 4 теста PASS, сигнальный выполнен;
- `golangci-lint run ./...` — 0 issues.

### Предложения в бэклог

1. EPIC-003 (T-229, настоящий агент встречи): исключение остановки сузить до ошибок отмены
   (`errors.Is(err, context.Canceled/DeadlineExceeded)`), чтобы настоящий отказ шины в момент отмены попадал в
   `Err()`. Для двойника не требуется.
2. Пункты 2–4 бэклога ревью #1 (`recover` в `StopAll`; `ERROR` нарратора на отмене; позднее запоминание) остаются
   в силе, в итерацию не входили.

## T-401 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Незакоммиченные правки в `.worktrees/T-401`, ветка `task/T-401-ci-race-detector`, база — `epic/EPIC-001-foundation`
(`9fc6c7c`). Пять файлов: `.github/workflows/go.yml`, `Makefile`, `architecture/infrastructure.md` (§2.2, §3.1),
карточка, `dev-log.md`. Go-кода в изменениях нет: `git diff --name-only | grep '\.go$'` пуст. Прочитаны раздел
«### T-401» индекса `tasks.md`, раздел выполнения карточки, ADR-010, §2.2 и §3.1 `infrastructure.md`, а также код,
на который ссылается задание: `test/e2e/*`, `cmd/multiverse/main.go`, `shared/eventbus/membus`,
`shared/testkit/{contract,gateway,swarm}`.

Детектор гонок на машине не запускается (`go env CGO_ENABLED` = 0, gcc нет, ОВ-5). CI отсюда не запустить.
Поэтому проверка шла чтением, через `actionlint`, `yaml.safe_load`, `make -n` и настоящие цели без детектора.
Контейнеры, стек и LLM не трогались, `.env` не открывался. Решения оркестратора — не замечания: `-count=3` стоит до
первого замера, внести `race` в required checks — дело владельца.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 1, Nit 5.

Задание `race` и `-race` у `integration` сделаны верно. cgo включён там, где нужен. Все места, названные в индексе
и в T-415/T-426, лежат в пакетах списка. Дочерний процесс e2e действительно собирается с детектором, и код 66
ловится. Имена существующих заданий не тронуты. Minor — список пакетов записан в двух местах, и расхождение
никто не проверяет, хотя способ держать его в одном месте есть. Остальное — Nit.

### Проверка по пунктам задания

1. **Задание `race`.**
   - `CGO_ENABLED: '1'` задан на уровне задания (`go.yml:261-263`), так что его получают оба шага, включая
     `go build` внутри e2e. У `integration` он задан на уровне шага (`go.yml:195`), а шаг там с детектором один.
   - Места, которые называет задание, найдены в коде:
     - `deliverNext` и `SetChaos` — `shared/eventbus/membus/membus.go:320`, `:162`;
     - контракт на membus — `shared/testkit/contract/membus_test.go:35`;
     - `Harness.mu` — `shared/testkit/gateway/harness.go:142`;
     - проекция рассказчика — `shared/testkit/swarm/fake_narrator.go:201`;
     - `failMu` — `shared/testkit/swarm/fake_encounter.go:248`, `:554`, `:574`.

     Всё это покрывают `./shared/eventbus/...` и `./shared/testkit/...`. Кроме `test/e2e`, тег `e2e` не стоит ни у
     одного файла, поэтому `./test/e2e/...` в `race` равно `./...` в задании `e2e`.
   - **Дочерний процесс собран с детектором.** `build()` вызывает `go build` изнутри теста
     (`test/e2e/empty_world_test.go:81`), без `cmd.Env`, то есть с окружением теста. `GOFLAGS=-race` и
     `CGO_ENABLED=1` доходят до сборки дочернего процесса. `-race` в командной строке и в `GOFLAGS` одновременно
     допустим.
   - **Код 66 ловится.** `main` завершается через `os.Exit(run(...))` (`cmd/multiverse/main.go:25`). Если гонка
     найдена, среда выполнения при `os.Exit(0)` заменяет код на 66. Кейс остановки по сигналу требует 0
     (`test/e2e/lifecycle_test.go:78`) и при падении печатает вывод процесса (`empty_world_test.go:170-172`).
     Подкоманда `multiverse health` (`empty_world_test.go:63-66`) тоже проверяет код выхода.
   - **Ограничение описано в карточке честно.** В `empty_world` процесс убивается отменой контекста, поэтому гонка в
     нём попадает только в вывод, а тест вывод не читает. Предложение — в бэклоге, п. 1.
   - `stubs_v0` работает внутри процесса теста (`membus.New`, `gateway.NewHarness`), и его покрывает `-race`
     тестового бинарника.
2. **`integration` с `-race`.**
   - `-timeout 20m` ограничивает каждый тестовый бинарник, а не шаг. `timeout-minutes: 30` задания = холодная
     сборка MinIO (~5 мин) + setup-go и компиляция с детектором (~2–3 мин) + 20 мин самого долгого пакета. Цифры
     согласованы, запас небольшой (N-4).
   - Юнит-тесты прогоняются повторно, но не бессмысленно. `-tags integration ./...` гонял юнит-тесты и до T-401,
     только без детектора. Сузить список нельзя без того же риска, от которого задание отказалось в `race`:
     новый тест за тегом, не попавший в список, молча выпадет. Цена повтора — CPU на критическом пути. Её покажет
     первый замер (N-4).
3. **Makefile.**
   - Признак cgo — существующий `RACE` (`Makefile:147`): `go env CGO_ENABLED` = 1 **и** `go env CC` найден в
     `PATH`. Эти два условия не противоречат друг другу: Go 1.20+ сам ставит `CGO_ENABLED=0`, когда компилятора нет.
   - Проверено на этой машине (make 4.4, ezwinports):
     - `make test-race` — отказ с объяснением, код рецепта 1, make — 2;
     - `make test-race RACE_OPTIONAL=1` — `SKIPPED`, 0;
     - в `make -n ci` условие раскрыто как `if [ -n "1" ]`, в `make -n ci RACE_OPTIONAL=` — как `if [ -n "" ]`;
     - `make -n deploy` наследует пропуск через `ci`;
     - `make -n test-race RACE=-race` печатает те же две команды, что CI;
     - `make -n test-integration` печатает «WITHOUT the race detector» и `-timeout 20m`.
   - На Windows `make ci` из-за `test-race` не ломается.
   - Список пакетов записан дважды и не сверяется — Mi-1.
4. **Существующие задания.**
   - `yaml.safe_load` — восемь заданий: `unit, integration, e2e, race, contracts, security, compose-lint, image`.
   - Имена и `name:` прежних заданий не изменены, `e2e` без флага.
   - `actionlint` (из `~/go/bin`) по `go.yml` — 0 замечаний.
   - Шапка `go.yml:6-11` и §3.1 `infrastructure.md` (строка required checks) обновлены. Устаревшее осталось в
     `plan/roadmap.md` — N-5.
5. **Флак `fight-09`.** Правками T-401 он вызван быть не может: в изменениях нет ни одного `.go`-файла. Тест
   `shared/testkit/gateway/stand_test.go` работает с `standTimeout = 2s` (`:48`), а `race` гоняет пакет трижды под
   детектором на двух ядрах. Поэтому новое задание, скорее всего, будет ловить этот флак чаще, чем `unit`. Это
   риск для required check, а не дефект задачи (см. «Риски»).

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-1 | Minor | `.github/workflows/go.yml:280-285`, `Makefile:183` | Список пакетов `race` записан дважды (третий раз — в §2.2/§3.1 `infrastructure.md`). Защищает его только комментарий «change both together». Если пакет добавить в одно место, CI и локальный прогон молча разойдутся, и никто этого не увидит. А один источник здесь возможен | На выбор: (а) шаг CI вызывает `make test-race RACE_COUNT=3`. GNU make на `ubuntu-latest` есть, Makefile без `.env` работает, а строгий режим (без `RACE_OPTIONAL`) без cgo откажет громко — на раннере это и нужно. (б) Если оставить правило «CI не вызывает make», добавить в `race` шаг-сторож: `make -n test-race RACE=-race` должен содержать строку `go test` из `go.yml`, иначе шаг падает. В любом случае перечислить два места в комментарии |
| N-1 | Nit | `Makefile:190` | `RACE_OPTIONAL` берётся из окружения: `RACE_OPTIONAL=1 make test-race` → `SKIPPED`, 0 (проверено). Если переменная экспортирована в оболочке, прямой вызов тоже становится мягким. `SKIPPED` при этом печатается, так что обмана нет | Рядом с `RACE_COUNT` добавить `RACE_OPTIONAL :=`. Присваивание в файле перекрывает окружение, `ci: RACE_OPTIONAL := 1` и командная строка работают как раньше |
| N-2 | Nit | `Makefile:189` | Причина всегда называет обе части: «CGO_ENABLED=0, CC=gcc not on PATH». При `go env -w CGO_ENABLED=0` и установленном gcc вторая половина неправда | Печатать только ту часть условия, которая не выполнилась |
| N-3 | Nit | `.github/workflows/go.yml:260` | `timeout-minutes: 20` меньше суммы настройки, компиляции с детектором и двух шагов по `-timeout 10m`. В красном прогоне, где первый шаг упирается в свой предел, задание будет убито раньше, чем второй шаг что-то сообщит. Кроме того, `-timeout 10m` при `-count=3` один на все три прохода пакета | После первого замера — `timeout-minutes: 25` либо шаги с явным `timeout-minutes` каждый. Пока «оценка, не замер» — допустимо |
| N-4 | Nit | `.github/workflows/go.yml:211`, `infrastructure.md` §3.1 | Юнит-тесты внутри `integration` теперь идут под детектором на критическом пути. Оценка «6–10 мин» — у самой границы «суммарно ≤ 10 мин» из §3.1 | После первого прогона вписать замер в §3.1. Если `integration` выйдет за 10 мин — отдельная задача: сузить набор так, чтобы тест за тегом не мог выпасть молча (например, через `go list -tags integration` по файлам с тегом) |
| N-5 | Nit | `Docs/dev-team/plan/roadmap.md:142` | Там всё ещё «required checks — шесть job'ов». Файл вне правок задачи | Поправить вместе с Mi-1 или передать ведущему план. Историческую DoD T-012 в `tasks.md:214` не трогать |

### Предложения в бэклог

1. `test/e2e`: `launch` при очистке ищет в выводе процесса `WARNING: DATA RACE` и валит тест. Тогда гонка в
   процессе, убитом отменой контекста (`empty_world`), тоже будет видна.
2. Флак `TestTheHarnessAndTheEncounterOfTheSwarmOnOneBus/fight-NN`: нужна отдельная задача владельцу
   `shared/testkit/gateway` через tech-lead#1, как предлагает исполнитель. Это надо сделать до того, как `race`
   станет required check.
3. Когда у `cmd/multiverse` (сборка контекстов, сигналы) или у целевого роя появятся конкурентные тесты — добавить
   пакеты в список `race`. `shared/agent` (каркас) в список сейчас не входит, и это верно.
4. Цифры первого прогона `race` и `integration` вписать в §3.1 вместо оценок.

### Риски и допущения

- Всё сказанное о поведении под `-race` установлено чтением: детектор здесь не запускался ни разу. Единственное
  доказательство того, что он ловит, — контрольный мутант на раннере (карточка, «Как проверить», п. 3).
- `race` будет чаще других заданий краснеть на флаке `fight-NN`. Пока задание не required, это шум. Когда станет
  required — будет блокировать мерж.
- Правки `artifactsDir` в ветке задачи (карточка, `dev-log.md`, `infrastructure.md`) идут по раскладке, которую
  задаёт оркестратор, и замечанием не считаются.

## T-401 · ревью #2 · 2026-09-11 · code-reviewer#1

### Границы ревью

Итерация 2 исполнителя: незакоммиченные правки в `.worktrees/T-401`, раздел «Итерация 2» карточки, запись в
`dev-log.md`. Проверены закрытие замечаний ревью #1 и регрессия от новых правок, прежде всего перенос шага
`race` на `make test-race`. К пяти файлам итерации 1 добавился `plan/roadmap.md`. Go-кода в изменениях по-прежнему
нет. Детектор гонок и make 4.3 здесь недоступны: cgo нет, в WSL только `docker-desktop`. Поэтому поведение на
раннере установлено чтением Makefile целиком — make разбирает весь файл, даже когда вызвана одна цель. Локально
цели проверены на make 4.4.1 без детектора. Контейнеры и `.env` не трогались.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 2.

Mi-1, N-1, N-2, N-3, N-5 закрыты. N-4 по своей природе ждёт первого прогона на раннере, и это записано в карточке.
Перенос на `make test-race` регрессии не дал. На раннере цель видит cgo и ложно не отказывает. Возможностей
make 4.4 она не использует. Логика `RACE_OPTIONAL` от вызова из CI не зависит. `-race` доходит до сборки
дочернего процесса e2e. Два новых Nit — о тексте, а не о поведении.

### Закрытие замечаний ревью #1

| # | Было | Стало | Итог |
|---|---|---|---|
| Mi-1 | список пакетов в `go.yml` и Makefile | шаг `race` — `run: make test-race` (`go.yml:285-289`); список только в `RACE_PKGS` (`Makefile:193`); в `go.yml` списка нет (`yaml.safe_load`: у `race` один шаг `make test-race`); §2.2/§3.1 ссылаются на `RACE_PKGS` | закрыто |
| N-1 | `RACE_OPTIONAL` из окружения | `RACE_OPTIONAL :=` (`Makefile:195`). `RACE_OPTIONAL=1 make test-race` → отказ, код 2; `make test-race RACE_OPTIONAL=1` → `SKIPPED`, 0 | закрыто |
| N-2 | причина называла обе половины | без gcc → «the C compiler CC=gcc is not on PATH…»; `CGO_ENABLED=0 CC=bash` → «CGO_ENABLED=0 although bash is on PATH…», код 2 | закрыто |
| N-3 | 20 мин задания | `timeout-minutes: 25` (`go.yml:273`), в комментарии объяснено, что `-timeout` действует на бинарник вместе со всеми проходами `-count` | закрыто |
| N-4 | замер `integration` | пункт «сделать после первого прогона» в карточке | перенесено обоснованно |
| N-5 | `roadmap.md:142` «шесть job'ов» | восемь заданий поимённо, обязательные выбирает владелец | закрыто |

### Особые пункты оркестратора

1. **cgo на раннере и ложный отказ.** `CGO_ENABLED: '1'` задан на уровне задания (`go.yml:274-277`) и попадает в
   окружение `make`. `RACE` (`Makefile:147`) вычисляется при разборе как `go env CGO_ENABLED` = `1` **и**
   `command -v "$(go env CC)"`. На linux без `CC` в окружении `go env CC` даёт `gcc`, а на `ubuntu-latest` gcc
   есть. `go` в `PATH` кладёт setup-go через `GITHUB_PATH`, его видят и `make`, и `$(shell)`. Результат —
   `RACE=-race`, ветка отказа не выполняется.

   Ложного отказа из-за конвейера `go env | grep -q` нет. `go env` пишет два байта одним вызовом до того, как
   `grep` может закрыть канал, так что SIGPIPE исключён. Даже если бы признак ошибся, строгий режим отказал бы
   громко (код 2 и причина в логе), а не пропустил бы проверку.

   Разбор остального Makefile на раннере безопасен:
   - `include build/versions.env` — файл в дереве;
   - `$(shell git rev-parse …)` — работает и на PR-коммите слияния;
   - `DOTENV_PROFILES` — `sed` по отсутствующему `.env` с `2>/dev/null`, код `$(shell)` не учитывается;
   - `uname`;
   - ни одного `$(error …)`.
2. **Возможности make 4.4.** Makefile прочитан целиком, плюс грэп по `.WAIT`, `--shuffle`, `$(let`, `$(intcmp`,
   `.NOTINTERMEDIATE`, `.EXTRA_PREREQS`, `$(file`, `MAKE_VERSION`: вхождений нет. Цель и файл используют только
   старые средства:
   - `.ONESHELL`, `.SHELLFLAGS` — с 3.82;
   - значения для цели, `export VAR :=`, `define`;
   - функции `if`, `foreach`, `subst`, `filter`, `findstring`, `wildcard`, `call`, `shell`.

   Единственное различие 4.3 и 4.4, которое касается этого места: 4.4 передаёт экспортированные переменные make
   в окружение `$(shell)`, а 4.3 — нет. Для `RACE` это не важно: `CGO_ENABLED` и `PATH` приходят из настоящего
   окружения задания, а не из `export` Makefile. Что шапка файла по-прежнему называет 4.4 — N-6.
3. **`RACE_OPTIONAL` при вызове из CI.** CI вызывает `make test-race` напрямую, без `ci`, так что значение для
   цели `ci: RACE_OPTIONAL := 1` не участвует. Присваивание в файле перекрывает окружение, если не задан
   `make -e`, а Actions не ставит ни `RACE_OPTIONAL`, ни `MAKEFLAGS`. В итоге на раннере режим всегда строгий.
   Локально поведение `ci` не изменилось:
   - `make -n ci`, `make -n ci-full`, `make -n deploy` — условие раскрыто как `-n "1"`;
   - `RACE_OPTIONAL= make -n ci` — тоже `-n "1"`, окружение не мешает;
   - `make -n ci RACE_OPTIONAL=` — `-n ""`.
4. **`-race` для e2e через make.** `export GOFLAGS := -buildvcs=false` (`Makefile:31`) передаёт `GOFLAGS` в
   окружение рецепта в любой версии make, так что под `set -u` `$$GOFLAGS` определена. Строка
   `GOFLAGS="$$GOFLAGS -race" go test …` (`Makefile:220`) даёт `GOFLAGS=-buildvcs=false -race`. Сборка
   дочернего процесса (`test/e2e/empty_world_test.go:81`, без `cmd.Env`) наследует эту переменную вместе с
   `CGO_ENABLED=1` задания. Повтор `-buildvcs=false` в `GOFLAGS` и в аргументах `go build` допустим. Код 66 —
   как в ревью #1: `os.Exit(run(...))`, кейс сигнала требует 0. `make -n test-race RACE=-race` печатает обе
   команды. Если первый `go test` падает, `-e` останавливает рецепт до e2e. Раньше было так же: шаги задания
   останавливались на первом упавшем.
5. **Прочее.**
   - `actionlint` — 0 замечаний; `yaml.safe_load` — восемь заданий, имена прежние.
   - Шаг один, поэтому отдельные `timeout-minutes` у шагов не нужны.
   - Флак `fight-NN` исполнитель не трогал; в карточке он отнесён к T-433. В индексах `tasks.md` веток T-401 и
     EPIC-001 такой задачи нет — вопрос оркестратору.

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| N-6 | Nit | `Makefile:8` | Шапка требует «GNU make 4.4», а CI теперь исполняет Makefile на `ubuntu-latest` с make 4.3. Сейчас 4.4 не нужна (п. 2 выше), но тот, кто добавит `.WAIT`, `$(let)` или другую возможность 4.4, по шапке не узнает, что сломает задание `race` | В шапке написать «GNU make 4.3+ (CI: `ubuntu-latest`, 4.3; Windows: ezwinports 4.4)». Риск из бэклога карточки (п. 4) этим снимается |
| N-7 | Nit | `Makefile:219` | Строка лога печатает `GOFLAGS=-race`, а на деле `GOFLAGS=-buildvcs=false -race`. Смысл печати — показать, с чем прогон шёл на самом деле | Печатать настоящее значение, например `echo "test-race: GOFLAGS=\"$$GOFLAGS -race\" go test …"` |

### Риски и допущения

- Поведение make 4.3 установлено чтением: запустить 4.3 здесь нечем. Если чтение ошиблось, первый прогон `race`
  упадёт громко на разборе или отказе, а не пройдёт молча.
- Детектор по-прежнему не запускался. Доказательство, что он ловит, — контрольный мутант на раннере (карточка,
  «Как проверить», п. 3). Лог шага теперь печатает команды, так что `-race` и список видны.
- Как и в ревью #1: пока флак `fight-NN` не исправлен, `race` в required checks будет блокировать мерж на шуме.
## T-431 · ревью #1 · 2026-09-11 · code-reviewer#2

### Границы ревью

Ревью #1 задачи T-431 (S), только документы. Работа лежит незакоммиченной в `.worktrees/T-431` (ветка
`task/T-431-contracts-revision-3` от эпика после `23c845d`): 7 изменённых файлов и новая карточка `tasks/T-431.md`.
Прочитаны карточка и раздел «### T-431» в `tasks.md`, дифф целиком: `contracts.md` v0.9 (C-01 v1.6, C-05 v1.6,
§16 п. 5, истории), ADR-027 «Уточнение исполнения 2», ADR-028 «Уточнение исполнения», design EPIC-003 §14.5 и
changelog 0.3a, `c4-component-foundation.md`, design EPIC-002 `:62`, разделы T-428/T-429/T-430/T-431 индекса.
Для сверки прочитаны КД §5.1 (`swarm-llm-laws.md:378-389`), задачи EPIC-003 T-224, T-225, T-227, T-228, T-230,
T-232, T-236 и T-421, а также код: `shared/runtime/lifecycle.go`, `shared/eventbus/delivery.go` (`deadLetter`,
`panicError`), `kafka.go` (`stopped`), `panic_stop_test.go`, `cmd/multiverse/serve.go`, двойники `testkit/swarm`.
Грэп `StopAll(` прошёл по всем `*.go` дерева. Код не запускался: в задаче его нет. В рабочей папке T-431 менялись
только этот раздел и строка ревью в карточке.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 10, Nit 3.

Решения согласованы между собой и с ADR-026/027/028, C-05, C-14 и NFR-061. Работающий код C-01 v1.6 не ломает.
Единственные документы, которым она противоречит, — КД §5.1 (шаги 1 и 5) и T-224. Они названы, и у правки есть
адресаты (см. Mi-10). Довод против `%w` проверен по коду и верен. Все замечания — точность текста, до коммита их
можно снять за полчаса. Mi-1…Mi-3 и Mi-6 — правки в одну строку, их лучше сделать в этой же итерации.

### Замечания

#### Critical, Major

Нет.

#### Minor

**Mi-1. Шапки версий не подняты: `contracts.md:3-4`, design EPIC-003 `:3`.** В `contracts.md:3` стоит «Версия 0.8 ·
… system-architect#1 (v0.7–v0.8)», хотя в той же строке уже записана v0.9. «Основание» (`:4`) перечисляет
«ADR-022…ADR-027», а C-05 v1.6 опирается на ADR-028. В design EPIC-003 `:3` — «Версия 0.3», хотя changelog
заканчивается записью 0.3a. Это тот же класс, что Mi-1 ревью #2 T-428: по первой строке читатель решит, что ссылки
«v0.9» и «0.3a» указывают на будущее.
*Как исправить:* «Версия 0.9 · … (v0.7–v0.9)», «ADR-022…ADR-028»; в design — «Версия 0.3a» и короткая фраза о 0.3a
в шапке.

**Mi-2. `contracts.md:190`: пометка «(v1.3a, T-410: правка текста, а не контракта — Go-API `Deps` не менялся…)» ушла
за вставку v1.6.** Раньше пометка стояла сразу после фразы про владение транспортом. Теперь она идёт после
«*Почему:*» абзаца v1.6, где Go-API `StopAll` как раз меняется (`contract-change`). Строка читается как
«Go-API не менялся» в адрес v1.6.
*Как исправить:* вернуть пометку за «…в `runtime.StartAll` (T-415).», перед «**v1.6 (T-431, код — T-430).**».

**Mi-3. `contracts.md:179`: общие правила `Dedup` оказались внутри подпункта «Посредник доставки».** «Предусловие двух
шагов — обработчик не вызывается конкурентно…» и «Своё окно без `IDs`/`Restore` у stateful-потребителя —
дефект» раньше завершали пункт «Дедуп». Теперь они стоят последним подпунктом посредника и читаются как условия,
которые касаются только его.
*Как исправить:* перенести обе фразы в конец основного абзаца «Дедуп» (`:174`), в подпункте оставить только
синхронность.

**Mi-4. T-430: назван не весь список мест вызова `StopAll`.** В production вызовов три, и это названо верно:
`lifecycle.go:26`, `serve.go:340`, `serve.go:349`. В `test/` и `shared/testkit` вызовов нет. Не назван четвёртый,
тестовый: `shared/runtime/runtime_test.go:320`. Без правки он не соберётся. Список файлов в `tasks.md:639`
(«трогает `lifecycle.go`, `delivery.go` и вызов в `serve.go`») его не включает, и `contracts.md:24` («три вызова»)
тоже.
*Как исправить:* дописать `shared/runtime/runtime_test.go:320` в раздел T-430 и в строку таблицы v0.9.

**Mi-5. `tasks.md:617`, C-01 v1.6 (рантайм, `contracts.md:189`): `errors.Join` делает ошибку `StartAll`
многострочной.** Эту ошибку `serve.go:62` печатает одной строкой: `Fprintln(stderr, "multiverse:", err)`. `Join`
разделяет части переводом строки. Комментарий `lifecycle.go:34-37` прямо исходит из того, что ошибка «ends up in
a line of stderr». Решение формат не задаёт, и DoD его не проверяет.
*Как исправить:* в решении T-430 выбрать форму. Либо ошибки `Stop` дописываются в ту же строку (`; stop context
…`), либо многострочность принимается явно, и тест отката проверяет вывод `serve`.

**Mi-6. `tasks.md:623`: «ошибка «нет приёмника» через `%w`» — такого значения ошибки нет.** Сейчас
`delivery.go:172` — это `fmt.Errorf("…no dead letter sink for %s: %w", ev.Type, cause)`, и `%w` там оборачивает
как раз причину. Оборачивать «ошибку «нет приёмника»» можно, только если завести значение. Экспортируемый sentinel —
это изменение Go-API `shared/eventbus`, а таблица v0.9 (`contracts.md:23`) и `:205` обещают «Go-API не меняется».
*Как исправить:* написать форму прямо: `fmt.Errorf("eventbus: no dead letter sink for %s (cause: %v)", …)` без
`%w`, либо неэкспортируемый sentinel.

**Mi-7. `tasks.md:636`: DoD T-430 требует «README `shared/runtime`», а такого файла нет.** Пункт в таком виде не
проверить: исполнитель либо создаст файл, которого никто не планировал, либо пропустит пункт.
*Как исправить:* указать существующее место (doc-комментарий пакета или `StopAll`) либо написать «создать
`shared/runtime/README.md`».

**Mi-8. `contracts.md:626`: фраза «необъявленные `OLLAMA_*` … стережёт правило 5 (SEC-15)» шире самого правила.** Правило 5
(`infrastructure.md:328`) проверяет из необъявленных только значение `OLLAMA_HOST`. `OLLAMA_ORIGINS` объявлена.
Необъявленную `OLLAMA_DEBUG: 1` не проверяет ни одно правило. Решение (б) это не задевает. Объявленных без
умолчания `OLLAMA_*` нет: у всех шести в `infra.go:62-77` есть умолчание. Но исполнитель T-432 по этой фразе
может начать расширять правило 5.
*Как исправить:* «правило 8 их не трогает; правило 5 проверяет из них только `OLLAMA_HOST` (SEC-15), остальные не
проверяются: через `.env` платформы они не настраиваются».

**Mi-9. Design EPIC-003 §14.5 п. 1 (`:487-489`) и C-01 «Посредник» (`contracts.md:175-178`): правило посредника
неполно в трёх местах.**
- (а) Перечень шагов, которые повтор выполнит второй раз, неполон. В КД §5.1 шаг 3 есть ещё `ScopeIndex.Apply` и
  `presence.Observe` (`swarm-llm-laws.md:384`). Шаги 6–7 перечень не упоминает вовсе.
- (б) Идемпотентность `journal.Observe` и `budget.Observe` «по id события» подана как факт, но её нет ни в КД, ни
  в DoD T-224/T-227. Повтор `llm.output` дважды учтётся в `BackgroundBudget` и сдвинет `lod_allowed` в живом прогоне.
- (в) Не сказано, что делать с остальными получателями после ошибки одного. КД §5.1 шаг 5 доставляет «параллельно
  между агентами». Если Router остановится на первой ошибке, а событие после трёх повторов уйдёт в `dead_letters`,
  его не получат и невиновные получатели. Это тот же класс, что §14.5 п. 3 снимает для действий.

*Как исправить:* перечислить все проекции шага 3. Идемпотентность по id записать требованием в «Кому»: строка DoD
T-224, для бюджета — T-227. Добавить фразу «ошибка одного получателя не отменяет доставку остальным; шине
возвращается одна из ошибок».

**Mi-10. Передача в EPIC-003 записана только в design §14.5, в индекс EPIC-003 она не попала.** T-224
(`epics/EPIC-003-swarm-llm-laws/tasks.md:377`) по-прежнему реализует шаг 1 КД §5.1: `eventbus.Dedup` до доставки.
Это прямо противоречит C-01 v1.6. Строки DoD для T-224, T-228, T-232 и T-236 существуют только как «Кому» в
design `:490` и `:505`. В T-416 и T-425 system-architect вписывал такие строки прямо в DoD EPIC-003 (T-232,
`tasks.md:591`, пометки «(T-416, …)» и «(T-425, …)»). Сейчас ничего не сломано: все задачи в статусе `todo`. Но
исполнитель T-224 читает свою карточку, а не §14.5.
*Как исправить:* вписать строки с пометкой «(T-431, 2026-09-11; design §14.5)» либо поручить это tech-lead#2 и
architect#2 до старта T-224 (решение оркестратора). Минимум — отсылка в «Что» у T-224: «шаг 1 — по C-01 v1.6,
design §14.5 п. 1».

#### Nit

**N-1. `tasks/T-431.md`: у карточки окончания строк LF (0 CR на 58 LF), а все файлы задачи и соседние карточки
(`T-428.md`) — в CRLF.** Смешения внутри файла нет. В репозиторий файл уйдёт так же (`* text=auto`).

**N-2. `ADR-025:29`: ссылка «(C-01, «Заглушка»)» ведёт на переименованный заголовок.** Пометка под новым
заголовком (`contracts.md:229`) выручает, но действующий ADR лучше поправить. `ADR-001:55` — запись своего времени,
её не трогать.

**N-3. `design.md:487`: «первую ошибку получателя» при параллельной доставке между агентами недетерминирована.** C-01
говорит просто «ошибку получателя». Слово «первую» лучше снять или сказать «любую» (см. Mi-9 (в)).

### Ответы на вопросы поручения

1. **Согласованность.** Противоречий не нашёл.
   - Посредник (C-01 v1.6, ADR-027 УИ2 п. 6) продолжает ADR-027 п. 3–4 и не задевает ADR-026.
   - «Граница ответа — пакет» (§14.5 п. 2) опирается на C-05 п. 5, а дописывание конца после рестарта — на ADR-026
     п. 10 и C-14 v1.2.
   - Отложенные действия (§14.5 п. 3; C-05 v1.6 п. 4) выполняют требование C-05 v1.5 «действие не теряется».
   - «Открытие один раз» (ADR-028 УИ п. 1) согласовано с ADR-026 УИ п. 4 (NPC занят с предложения).
   - NFR-061: запись в окно после доставки не меняет байты событий. Запись «открытие в полёте» лежит в снапшоте.
     Условие пересмотра TTL сформулировано по тикам, а не по настенному времени.
   - Работающий код v1.6 не ломает. Ни у `Delivery`, ни у membus, ни у kafka своего окна нет. `FakeNarrator` уже на
     `Has`/`Add` (`fake_narrator.go:340-346`, `:418-436`). `FakeEncounter` остаётся на `Seen` (`fake_encounter.go:647`),
     и новой формулировке C-01 это не противоречит: двойник на повторе бросает кости заново, его эффект не
     идемпотентен.
   - Утверждения ADR-028 УИ сверены. `reason=error` есть в enum схемы (`agent.stopped.v1.json:9`).
     `encounter.child_blueprint` описан в `api-contracts.md`, на который ссылается C-11. Id встречи у двойника —
     `"encounter-" + ev.ID` (`fake_encounter.go:732`).
2. **T-430.**
   - Места вызова — Mi-4.
   - Отказ от `%w` обоснован верно. `stopped` (`kafka.go:455-456`) принимает `io.EOF` и `io.ErrClosedPipe` в любом
     месте цепочки, а `panicError.Unwrap` отдаёт и `ErrHandlerPanic`, и значение паники, если это ошибка
     (`delivery.go:265-273`). `%w` для причины протащил бы `io.EOF` в цепочку. Сторож `panic_stop_test.go` держит
     четыре таких случая.
   - Запасной путь «только `ErrHandlerPanic`, без значения» безопасен.
   - DoD проверяем, кроме пунктов Mi-5 (формат строки) и Mi-7 (README).
3. **§16 п. 5.** Формулировка однозначна: литерал любой формы (отображение или элемент списка) при любом значении,
   единственная допустимая форма названа. Объявленных без умолчания `OLLAMA_*` сейчас нет. Необъявленные правило 8
   не задевает. Неточность про правило 5 — Mi-8. **Для приёмки:** оркестратор вынес литерал `OLLAMA_*` в отдельную
   задачу T-432 (S). Строку DoD из раздела T-429 (`tasks.md:591-601`) нужно перенести в T-432, а ссылки «Правка —
   T-429» перенаправить на T-432: `contracts.md:28`, `:627`, `T-431.md:43`. Это делает приёмка, замечанием к
   исполнителю не считаю.
4. **Истории версий.** Записи есть и точны: сводка v0.9 (`contracts.md:9-29`), шапка C-01 и история v1.6 (`:161`,
   `:233-239`), шапка C-05 и история v1.6, ADR-027 УИ2 (п. 5–7 продолжают п. 1–4 УИ1), ADR-028 УИ, design 0.3a.
   Номера монотонны: C-01 v1.5a → v1.6, C-05 v1.5 → v1.6. Буква у design 0.3a — правка текущей версии 0.3, это
   соответствует уроку T-428. Шапки не подняты — Mi-1.
5. **Редакционные из T-428** сделаны: `c4-component-foundation.md:23`, design EPIC-002 `:62`, заголовок C-01
   (`contracts.md:228-229`). Раздел T-428 отмечен (`tasks.md:585`, «Выполнено в T-431»).
6. **Гигиена.**
   - `git diff --stat`: 7 файлов, +197/−17. С `--ignore-all-space` то же самое, случайных перезаписей нет.
   - CRLF: у всех 7 файлов число CR равно числу LF. Карточка — N-1.
   - Конец файла: у всех ровно один перевод строки.
   - Mermaid: в `c4-component-foundation.md` изменилась только строка описания `Component(ct, …)` внутри кавычек.
     Скобки внутри кавычек там уже встречаются (`eb`), синтаксис не нарушен.

### Предложения в бэклог

1. EPIC-003 (tech-lead#2, architect#2): строки DoD по design §14.5 п. 1 и п. 4 — T-224, T-227 (бюджет), T-228,
   T-232, T-236 и КД §5.1 шаги 1, 3, 5, если оркестратор не решит по Mi-10 иначе.
2. EPIC-001: перенос строки DoD литерала `OLLAMA_*` из T-429 в T-432 (см. ответ 3).

## T-431 · ревью #2 · 2026-09-11 · code-reviewer#2

### Границы ревью

Повторное ревью после итерации 2 (system-architect#1). Проверены только исправления по ревью #1 и регрессия от
них, а также то, что появилось по решениям оркестратора: строки DoD в индексе EPIC-003, КД §5.1 и T-432. Работа
по-прежнему незакоммичена в `.worktrees/T-431`: 11 изменённых файлов и карточка. К файлам итерации 1 добавились
`ADR-025`, `components/swarm-llm-laws.md` и `epics/EPIC-003-swarm-llm-laws/tasks.md`. Прочитаны: «Итерация 2» в
карточке, дифф всех файлов относительно HEAD, разделы T-429, T-430, T-432 индекса EPIC-001, КД §5.1 и §7.2, design
EPIC-003 §14.5. Для особых вопросов сверены `cmd/multiverse/serve.go` (doc `run`, печать ошибки, `:50-64`,
`:255-264`), `shared/runtime/lifecycle.go` и проверки текста ошибки `StartAll` в тестах. Код не запускался. Менялись
только этот раздел и строка ревью в карточке.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 1, Nit 3.

Все 13 замечаний ревью #1 закрыты, и регрессий от правок нет. Новое замечание одно (R2-Mi-1): требование
«повтор `tick.fired` гасится по `tick_seq`» из §14.5 п. 1 не согласовано с `tick.aborted` из КД §7.2, и строки DoD
у него нет. Без исправления тик, уже объявленный прерванным, выполнится на повторе шины. Правка на две строки,
лучше сделать до коммита. Остальное — Nit.

### Статус замечаний ревью #1

| № | Статус | Доказательство |
|---|---|---|
| Mi-1 | **закрыто** | `contracts.md:3` — «Версия 0.9 … (v0.7–v0.9)», `:4` — «ADR-022…ADR-028»; design EPIC-003 `:3` — «Версия 0.3a … 0.3a — system-architect#1» |
| Mi-2 | **закрыто** | в C-01 пометка «(v1.3a, T-410…)» снова стоит перед «**v1.6 (T-431, код — T-430).**» (порядок проверен грэпом) |
| Mi-3 | **закрыто** | «Предусловие двух шагов…» и «Своё окно без `IDs`/`Restore`…» завершают основной абзац «Дедуп». В подпункте «Посредник» их нет |
| Mi-4 | **закрыто** | `tasks.md` T-430 и `contracts.md:24` называют `lifecycle.go:26`, `serve.go:340`, `serve.go:349` и `runtime_test.go:320`. В «Исполнителе» T-430 добавлен `runtime_test.go` |
| Mi-5 | **закрыто** | C-01 v1.6 и T-430: `start context <имя>: <причина>; stop context <имя>: <причина>`, `%w` у ошибки старта; тест «перевода строки нет». Разбор — ниже |
| Mi-6 | **закрыто** | T-430: `fmt.Errorf("eventbus: no dead letter sink for %s (cause: %v)", ev.Type, cause)`, «без `%w` и без нового значения ошибки, Go-API не меняется» |
| Mi-7 | **закрыто** | DoD T-430: doc-комментарии `StopAll`/`StartAll` в `lifecycle.go`, «README … не заводится» |
| Mi-8 | **закрыто** | `contracts.md:627` и T-432: правило 5 проверяет только `OLLAMA_HOST`, остальные (`OLLAMA_DEBUG`) не проверяет никто, T-432 правило 5 не расширяет |
| Mi-9 | **закрыто** (новое по шагу 6 — R2-Mi-1) | (а) design `:489-494` — шаги 3–7 с требованием к каждому, в КД §5.1 шаг 3 — к каждой проекции; (б) `budget.Observe` — строка T-227; (в) C-01 «Посредник» и §14.5: ошибка одного получателя не отменяет доставку остальным |
| Mi-10 | **закрыто** | строки «(T-431, 2026-09-11)» в T-224, T-227, T-228, T-232, T-236. «Что» у T-224 — шаги 1–4 и 6–8. КД §5.1 — шаги 1, 3, 5, новый шаг 8 и абзац «Почему `Add` после доставки» |
| N-1 | **закрыто** | карточка в CRLF: 81 CR на 81 LF |
| N-2 | **закрыто** | `ADR-025:29` — «(C-01, «Реализации для потребителей»; до T-431 — «Заглушка»)» |
| N-3 | **закрыто** | «первую ошибку» снято, теперь «любая из ошибок» |
| перенос в T-432 | **сделано** | строки DoD в T-429 нет, есть отсылка; `contracts.md:28`, `:628`, `T-431.md:43` ведут на T-432 |

### Замечания

#### Critical, Major

Нет.

#### Minor

**R2-Mi-1. Design EPIC-003 `:492` (шаг 6) против КД §7.2 (`swarm-llm-laws.md:454`): тик, объявленный прерванным,
выполнится на повторе. Строки DoD у требования нет.**
- §14.5 п. 1 требует: «повтор `tick.fired` агент-адресат гасит по `tick_seq` — тик с номером не больше последнего
  выполненного не выполняется».
- КД §7.2 при ошибке выполнения тика после `tick.fired` публикует `tick.aborted` (паника роли перехвачена,
  `handled=false`).
- По новому шагу 8 (КД `:389`) и строке T-228 эта ошибка теперь уходит из `Route` шине, и шина повторит
  `tick.fired`. Тик не был выполнен, поэтому по формулировке шага 6 повтор его выполнит. В журнале окажется
  `tick.aborted`, а за ним — результаты того же тика. При догоне это читается иначе, чем в живом прогоне: тик с
  `tick.aborted` считается прерванным.
- Требование шага 6 не попало ни в одну строку DoD. Строка T-227 касается только `budget.Observe`, строка T-224 —
  только проекций шага 3, а ветку `tick.fired` реализует T-224 (шаги 6–7).

*Как исправить:* в §14.5 п. 1 (шаг 6) написать «не больше последнего выполненного **или прерванного
(`tick.aborted`)**» либо «после `tick.aborted` `HandleTick` возвращает `nil`: тик закрыт». Добавить строку
«(T-431, …)» в T-224 или T-227. Тест: ошибка тика → `tick.aborted` → повтор `tick.fired` тик не выполняет, и
второго `tick.aborted` нет.

#### Nit

**R2-N-1. КД `:389` (шаг 8): «только если шаги 4–7 прошли без ошибки», а design `:489` говорит, что повтор выполняет
шаги 3–7.** Шаг 3 из условия выпал. Сегодня проекции ошибок не возвращают, но формулировки лучше свести: «шаги
3–7».

**R2-N-2. «Шине возвращается любая из ошибок» (C-01 «Посредник», design `:487`, КД шаг 5, строка T-224).** Для
NFR-061 это неважно. Какую ошибку вернул Router, не влияет ни на опубликованные события, ни на число попыток, ни на
порядок. Догон идёт через `Journal`, а там нет ни повторов, ни `dead_letters`. Между прогонами будет отличаться
только текст поля `error` в `dead_letters` и в логе. Если захочется воспроизводимого текста (тест T-224 с двумя
ошибками), дешёвый вариант — «ошибка первого по порядку `Recipients(ev)`». Порядок получателей детерминирован.

**R2-N-3. Строка T-227 кладёт учтённые id окна бюджета в снапшот «(T-236)», а в DoD T-236 такой строки нет.** Там
только «открытие в полёте». Можно дописать в ту же строку T-236 «и id окна `BackgroundBudget`». Строки в changelog
индекса EPIC-003 нет. Прецедента тоже нет: T-416 и T-425 её не добавляли, поэтому это не замечание.

### Ответы на особые вопросы

1. **Mi-5, формат «; ».** Цепочка не теряется. Ошибка старта остаётся под `%w`, поэтому `errors.Is`/`errors.As`
   для неё работают, как раньше. Ошибки `Stop` входят текстом, и для них различение не требуется. Действующие
   проверки текста — `strings.Contains` (`serve_test.go:276`, `runtime_test.go:288`) и префикс
   (`fake_contexts_test.go:140`). Дописанный хвост их не ломает. Doc `run` в `serve.go:255-264` говорит о порядке
   остановки, а не о формате, поэтому противоречия нет. Комментарий `start` (`lifecycle.go:34-37`, «ends up in a
   line of stderr») с решением согласован. Оговорка — в «Рисках».
2. **Mi-9, «любая из ошибок».** См. R2-N-2: для replay неважно.
3. **КД §5.1 шаг 8** согласован с C-01 v1.6: `Add` только после всех получателей без ошибки, отмена при остановке
   считается ошибкой. С design §14.1 п. 4–6 тоже: повтор шины и повторная доставка после рестарта доходят до
   агента, счётчик предела растёт, после предела событие уходит в `dead_letters`. С §14.5 п. 3 тоже: отложенное
   действие возвращает `nil`, и Router его запоминает. Исключение — тик, R2-Mi-1.
4. **T-432** (`tasks.md:647`). Заголовок в формате соседних: `### T-432: … · Размер: S · Статус: todo · Волна 1
   (бэклог)`. DoD проверяем: три фикстуры с `expect-text`, рабочая композиция проходит `make compose-lint`, одна
   фраза в `infrastructure.md` §3.1.1 п. 8. Порядок относительно T-429 записан.

### Гигиена

- Изменено 11 файлов и новая карточка. `git diff --stat`: +412/−24. С `--ignore-all-space` то же самое, перезаписей
  нет.
- CRLF: у всех файлов число CR равно числу LF, у карточки тоже (81/81).
- Конец файла: у всех ровно один перевод строки.
- `review.md`: удалённых строк нет, раздел ревью #1 на месте. Диаграммы в итерации 2 не менялись.

### Предложения в бэклог

Новых нет. Пункт 1 ревью #1 выполнен в итерации 2, пункт 2 — тоже.

## T-430 · ревью #1 · 2026-09-11 · code-reviewer#2

### Границы ревью

Ревью исполнения решения T-431 (C-01 v1.6, контракты v0.9). Работа не закоммичена и лежит в `.worktrees/T-430`,
ветка `task/T-430-panic-in-stop` от эпика `e3d2ad2`. Изменено семь файлов (+351/−31 с `--ignore-all-space`,
перезаписей нет), добавлена новая карточка `tasks/T-430.md`. Прочитаны: раздел «### T-430» индекса (`tasks.md:597-632`),
карточка, C-01 v1.6 (`contracts.md:21-24`, `:184-209`), дифф целиком, `lifecycle.go` целиком,
`serve.go:48-66`, `:250-354`, `delivery.go:167-191`, `fake_contexts.go:39-68`, `testkit/swarm/fake_context.go:160-215`.
Коммитов вне ветки нет. Изменения в `artifactsDir` — только карточка и dev-log, как положено исполнителю.
Менялись только этот раздел и секция «Ревью» в карточке.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 1, Nit 1.

Решение T-431 исполнено без отклонений. Паника в `Stop` больше не прерывает остановку остальных контекстов, шина
закрывается последней на всех путях. Мои мутанты красные, сторож держит ветку `DLQ == nil`. Mi-1 касается точности
doc-комментария и формулировки риска в карточке. Её лучше поправить до коммита, одной правкой текста, код не меняется.

### Замечания

#### Critical, Major

Нет.

#### Minor

**Mi-1. `shared/runtime/lifecycle.go:27-28`: doc `StartAll` обещает одну строку при любой ошибке, а код держит её
одной только для паники. Контрпример уже есть в дереве.**
- Комментарий начинается утверждением «The error of a failed start is one line». Экранирование (`:92`, `:97`)
  применяется только к тексту паники. Ошибки, которые `Start`/`Stop` возвращают сами, входят в строку как есть.
  Абзац `:35-37` («no panic value can break the line») точен, спорна только первая фраза.
- Контрпример: `shared/testkit/swarm/fake_context.go:185` — `FakeContext.Stop` возвращает
  `errors.Join(encounter.Wait(), narrator.Wait())`. Это контекст `swarm` при `MV_SWARM_FAKE=true`
  (`cmd/multiverse/fake_contexts.go:45`), то есть production-путь через хук T-255, а не только тесты. Если обе
  подписки заглушки упали сами до отката, хвост `; stop context swarm: <a>\n<b>` разорвёт строку stderr
  (`serve.go:62`).
- Отсюда неточность в карточке: риск 1 говорит «Сейчас таких контекстов нет (все — заглушки)». Это неверно.
- Серьёзность низкая. На откате `Stop` сразу отменяет подписки, а отменённая подписка возвращает `nil` (C-01).
  Перевод строки появляется, только если обе подписки упали сами в окне старта. На обычной остановке ошибки `Stop`
  уходят в slog (`serve.go:342-344`), и JSON-обработчик экранирует их сам. Поэтому это Minor: комментарий обещает
  больше, чем есть, а п. 4 DoD — точность doc-комментариев.

*Как исправить:*
1. Первую фразу сделать точной. Например: «The process prints the error of a failed start as one line of stderr. A
   panic can never break that line (below); an error a context returns itself goes in as it is.»
2. В риске 1 карточки назвать контрпример `FakeContext.Stop`.
3. Вопрос «экранировать ли всё» уже ушёл к system-architect (риск 1 карточки) — см. «Ответы», п. 3а.

#### Nit

**N-1. Обратная косая черта не экранируется: литерал `a\nb` и `a<LF>b` дают одинаковую строку ошибки (риск 2
карточки).** Действий не нужно. Значение как есть и стек лежат в поле лога `panic`, так что различить можно. Строгий
вариант (`\` → `\\`) меняет формат из C-01 v1.6, где названы только `\n` и `\r`. Это решение архитектора, а выгода
близка к нулю. Остальные «переводы строк» Unicode (NEL, U+2028) терминалы и сборщики логов строками не считают.

### Ответы на вопросы поручения

1. **Соответствие решению T-431.**
   - Сигнатура: `StopAll(ctx, contexts, log *slog.Logger) []error` (`lifecycle.go:123`).
   - Места вызова (грэп `StopAll(` по всем `*.go`, включая `test/`, `shared/testkit`, `services/`):
     - production: `lifecycle.go:42` (`deps.Log`), `serve.go:342` и `:351` (`log` процесса);
     - тесты: `runtime_test.go:320` (`nil`), `:416`, `:450`;
     - в `shared/testkit` и `test/` вызовов нет.
   - Формат: `stop context <имя>: panic: <значение>` (`:92` + `:127`).
   - Хвост отката: «; » перед каждой ошибкой, в порядке отката (`:44-49`).
   - Экранирование `\n`/`\r` — только текста паники в ошибке (`:92`), поле лога `panic` пишется как есть (`:89`).
   - `%w` у ошибки старта, хвост — текстом: тест `errors.Is` = true на старт и false на `Stop`.
   - `delivery.go:172`: `(cause: %v)` дословно по решению, Go-API `shared/eventbus` не менялся.
2. **Порядок ADR-023.**
   - `stop` (`:70-77`) ловит панику на каждом контексте отдельно, поэтому цикл `StopAll` доходит до конца.
   - Откат `StartAll` идёт через тот же `StopAll`.
   - Шину закрывает только отложенный `closeBus` (`serve.go:293`), после `return` из `run`.
   - Двойного `Stop` нет: неудачный старт — `return` на `:301` без `shutdown`; `shutdown` — только при сбое `listen`
     (`:304`); обычный путь — один `StopAll` (`:342`).
   - `nil` логгер: проверка на `:86` и отдельный тест.
   - Контекст, чей `Start` упал, не останавливается, как и в T-415. Это не новость T-430.
3. **Риски исполнителя.**
   - (а) Minor, см. Mi-1. Не блокирует: вероятность низкая, последствие — строка stderr, разбитая на несколько.
     Для архитектора: дешевле всего экранировать на границе вывода — один `strings.Replacer` над `err.Error()` в
     `runServe` (`serve.go:58`, `:62`). Цепочка не трогается, нового типа ошибки нет, покрываются любые причины.
     Тогда экранирование в `recovered` остаётся ради лога и вызывающих, кроме `serve`. Вариант
     «обёртка `oneLineError{err}` с `Unwrap`» тоже около десяти строк, но экранирует только то, что прошло через
     `StartAll`.
   - (б) Nit, см. N-1.
4. **Тесты.**
   - Лента serve (`serve_test.go:242`) — точная последовательность `start first|start second|stop second|stop first|bus closed`,
     в логе процесса — ошибка и стек.
   - Кейс отката (`:277`) — полная строка `start context second: boom; stop context first: panic: stuck`.
   - Многострочная паника: LF и CRLF, в `Start` и в `Stop`, точная строка и значение в логе как есть.
   - Сторож: 4 причины × 2 приёмника = 8 подкейсов. Под MD (`%w` в ветке `DLQ == nil`) краснеют ровно 4 подкейса
     «no sink», 4 «the sink fails» остаются зелёными. Так и должно быть: мутант трогает только эту ветку.
   - Мои мутанты — в копии в scratch, не через `-overlay`. Каждый проверен на применение, после каждого файл
     восстановлен побайтно. Базовый прогон до и после зелёный, копия удалена по сохранённому пути. Все красные:

     | Мутант | Правка | Что покраснело |
     |---|---|---|
     | M0 (контрольный) | синтаксическая ошибка | сборка |
     | MS | `recover()` → `any(nil)` только в `stop` | «StopAll let the panic out», пакет `cmd/multiverse` падает паникой `boom` |
     | MRL (свой) | откат `StartAll` передаёт `nil` вместо `deps.Log` | «log = 0 records», «log = 1 records, want one per panic» |
     | MCR (свой) | в `oneLine` нет пары `\r` | подкейсы CRLF в двух тестах |
     | MD | см. выше | 4 подкейса «no sink» |

     Ни один мутант не выжил.
   - Тесты детерминированы. `records` читается в той же горутине. `logged` в тесте serve читается после получения
     результата `run` через канал, что даёт happens-before. Проверить это гонщиком нельзя: `-race` недоступен, нет cgo.
5. **Doc-комментарии.**
   - `StopAll` (`:113-122`) точен.
   - Фраза в `serve.go:264-266` точна.
   - `StartAll` — кроме первой фразы (Mi-1) — точен: откат через `StopAll` с `deps.Log`, формат, цепочка, экранирование.
   - Строки `lifecycle.go:26`, `serve.go:340/349` в C-01 и индексе — ссылки на состояние до правки. Это история
     решения, замечанием не считаю.

### Прогоны (go1.26, windows/amd64; `-race` недоступен — нет cgo)

- `go build ./... && go vet ./... && go vet -tags e2e ./test/...` — 0.
- `go test -short -count=1 ./shared/... ./cmd/...` — все 28 пакетов с тестами ok.
- `go test -tags e2e -count=1 ./test/...` — `test/e2e` ok (14.1 с), `test/fixtures` ok.
- `golangci-lint run ./...` — 0 issues. `gofmt -l shared cmd test` — пусто.
- Живой запуск не нужен: лента serve гоняет настоящий `run` на `--bus=memory` и порту `127.0.0.1:0`.

### Гигиена

- Go-файлы в LF.
- Карточка (56/56), dev-log и `review.md` в CRLF, число CR равно числу LF, в конце ровно один перевод строки.
- Метка dev-log `<!-- dev-log T-430 -->` — в формате соседних записей.

### Предложения в бэклог

1. Для system-architect: одна строка stderr при любой причине отказа — экранирование на границе `runServe` (Mi-1,
   «Ответы», п. 3а). Пока нет доменных контекстов, срочности нет. Это стоит решить до первого настоящего контекста
   EPIC-002/003, чей `Stop` соберёт ошибки через `errors.Join`.

## T-402 · ревью #1 · 2026-09-12 · code-reviewer#2

### Границы ревью

Задача — данные замера и документ, кода нет. Работа не закоммичена и лежит в `.worktrees/T-402`, ветка
`task/T-402-llm-bench-baseline`. Коммитов в ветке нет (`git log epic/EPIC-001-foundation..HEAD` пуст). Эпик с тех пор
ушёл вперёд (`48b88fd`). Изменены `ops/metrics/baseline.md`, карточка `tasks/T-402.md` и `dev-log.md`, добавлены
`ops/metrics/bench-20260911-2327.csv`, `bench-E-20260911-2327.json`, `bench-20260911-2353.csv` и
`bench-Q36-A3B-20260911-2353.json`. `bench-matrix.json` и скрипты не менялись. Прочитаны: раздел «### T-402» индекса
(`tasks.md:445-447`), `infrastructure.md` §6.4 (`:863-989`), `bench-matrix.json`, шапка и логика `scripts/llm-bench.sh`,
прогрев в `scripts/llm-bench.ps1:341-367`, дифф целиком, оба JSON-отчёта целиком.

Проверки стенда — только чтение: `GET :8888/health`, `/props`, `/v1/models` по одному разу и `netstat` без командных
строк. Замер не повторялся, серверы не трогались.

### Вердикт

**ВЕРНУТЬ** — Critical 1, Major 1, Minor 4, Nit 2.

Числа честные: все шесть ячеек пересчитаны по `requests[]`, совпало всё. Вердикты по порогам верны, пометка «один
прогон» стоит везде. Отклонения от методики названы все пять: копия матрицы, пин билда в CSV, недействительный B2,
контекст вне ячеек, неизвестный KV. Справочное сравнение не выдаёт себя за решение U-2 (§2.5, §5). Строки «не
замерено» — с причинами. Вернуть приходится по двум пунктам. В JSON-отчётах лежит путь профиля пользователя (C-1).
Главный вывод «p95 определяет роутер, а не модель» подан как установленный факт, хотя второй замер в том же документе
ему противоречит (M-1).

### Замечания

#### Critical

**C-1. `ops/metrics/bench-E-20260911-2327.json:20`, `ops/metrics/bench-Q36-A3B-20260911-2353.json:20`: в `meta.matrix`
лежит абсолютный путь временного каталога в профиле пользователя.**
- Значение: `C:/Users/CD86~1/AppData/Local/Temp/claude/<слаг проекта>/<UUID сессии>/scratchpad/t402*.…/bench-matrix.*.json`.
  Это путь профиля пользователя, инструмент и идентификатор сессии — лишний персональный путь сверх id модели.
  Файлы предназначены для репозитория.
- Ключей, заголовков авторизации, токенов и содержимого `.env` нет. Поиск по четырём файлам: `bearer`,
  `authoriz`, `api_key`, `sk-`, `token=`, `password`, `secret`. Текстов ответов в `requests[]` нет. `url` — это
  `host.docker.internal:8888`. Путь `D:\Models\…\*.gguf` в CSV и JSON — id модели, он допустим.
- Причина в скрипте. `scripts/llm-bench.sh:834` пишет `matrix=${matrix#"$repo_root/"}`, и путь вне репозитория
  остаётся абсолютным. Исполнитель этого не заметил.

*Как исправить:* в обоих JSON заменить значение `meta.matrix` нейтральным, например `"scratch copy of
ops/metrics/bench-matrix.json: configs.E.tick/narrative = Qwen3.8 (baseline.md §1)"`, а для Q36 — ссылку на §2.5.
В `baseline.md` §1 и §2.5 одной фразой отметить, что поле отредактировано вручную и почему. Абзацы, где сказано, что
в `meta.matrix` лежит путь временного каталога (`baseline.md:53-54`), поправить так же. Правка скрипта — в бэклог
(ниже, п. 2).

#### Major

**M-1. `baseline.md:118-121`, `:225-226`, `:240`, `:251-255`, `:267-269`, `:288-289`; `tasks/T-402.md:56-59`:
причина p95 у E («роутер, не модель») подана как установленная, хотя данные прогона Q36 ей противоречат.**
- Для E довод один: диагностика вне методики, 5 + 5 запросов. Через роутер «необъяснённое» время было 5,3–5,6 с
  в 2 из 5 запросов, напрямую — 0,12–0,31 с в 5 из 5. Сырые числа не сохранены нигде, кроме текста.
- Прогон Q36 шёл по тому же прямому пути в `llama-server`, без роутера. По `requests[]` у него «прочее»
  (латентность − `predicted_ms`) больше 2 с в **11 из 30** запросов: 2199–2801 мс; tick — 1 из 10,
  phase2 — 5 из 10, group3 — 5 из 10. Короткая проверка после прогона этого не воспроизвела: 0,13–0,24 с (§2.5).
  Это ровно тот тип проверки, на котором держится вывод о роутере для E.
- Отсюда: «напрямую в `llama-server` накладные 0,12–0,31 с» верно для одиночных запросов после прогона. Для
  последовательной нагрузки внутри прогона это не доказано, и в том же документе есть обратный пример. Какая доля
  5,7–6,3 с у E приходится на роутер, а какая на сервер, не установлено. Отсюда же оценка «~2,7 с без роутера»
  (`:253-255`) опирается на непроверенную посылку.
- Почему Major, а не Minor: этим выводом объяснены `fail` E в §3 и «Главный вывод» карточки. По ним architect#1
  будет читать прогон 1 и решать, нужны ли C/A. Решения U-2 это не меняет — оно и так не принято.

*Как исправить:* переформулировать как гипотезу с данными за и против. За: 2 из 5 через роутер против 0 из 5
напрямую. Против: 11 из 30 внутри прогона Q36 напрямую, не воспроизведено после. Итог: доля роутера в хвосте E не
установлена, проверят прогоны 2–3 E на чистом llama.cpp. §3 (NFR-002, «Честный вердикт»), §4 пп. 1 и 6(а) и
«Главный вывод» карточки привести к той же формулировке. Числа обеих диагностик (запрос, путь, латентность,
`prompt_ms`, `predicted_ms`) сохранить рядом с отчётами — одним CSV без путей — либо явно написать, что они не
сохранены.

#### Minor

**Mi-1. `baseline.md:185-191`, `tasks/T-402.md:103-104`: у Q36 «хвост» — это половина нарративных запросов, и
гипотеза прогрева не сходится с порядком запросов.**
- phase2: «прочее» больше 2 с у 5 из 10, в том числе у медианного запроса (2941 мс, из них ~2,3 с не генерация).
  group3 — 5 из 10. Это не хвост, это p50.
- Гипотеза «первые запросы новых форм прогревали сервер» против порядка. Фазы шли tick → phase2 → group3. В tick
  медленный только последний запрос (idx 10). В phase2 медленные idx 1, 2, 8, 9, 10, в group3 — idx 4, 5, 6, 9, 10.
  Под конец их не меньше, чем в начале.

*Как исправить:* написать «у половины запросов phase2 и group3, включая медиану». Гипотезу прогрева убрать либо
пометить как не подтверждённую порядком. Без `timings.prompt_ms` в отчёте причину не установить — это п. 2
бэклога исполнителя.

**Mi-2. `baseline.md:202`, `:192-193`: «`latin` ≤ 0,006» — это среднее по ячейке, а не верхняя граница.**
- Максимум по ответу — 0,0341 (`df-02…group3`), ещё 0,0238 (`df-10…group3`). Порог NFR-090 — на ответ (≤ 0,10),
  так что вердикт `pass` остаётся.
- «Отдельные латинские буквы в тексте» по артефактам не проверить: тексты ответов не сохраняются.

*Как исправить:* «среднее 0,006, максимум по ответу 0,034». Фразу о характере латиницы убрать либо указать, откуда
она (просмотр ответов во время прогона).

**Mi-3. `baseline.md:34`, `:36`, `:293`: в документе репозитория лишние сведения о машине владельца.**
- Перечень программ рабочего стола: Word, Steam, Razer, клиенты видеосвязи и удалённого рабочего стола, `claude.exe`,
  антивирус `avpui.exe`.
- Каталог бинарника `D:\Models\llama\llama\`.
- Для замера важно число графических процессов и то, что вычислительный процесс один. Имена программ и путь к
  бинарнику ничего не объясняют в числах.

*Как исправить:* «~35 графических процессов рабочего стола (браузер, офис, мессенджеры и т. п.); вычислительный —
только `llama-server`». Путь к `.exe` убрать, достаточно «`llama-server.exe`, b10878». Про антивирус — «антивирус
в списке процессов GPU».

**Mi-4. `baseline.md:348-356` против `tasks/T-402.md:35-37`: инструкция «Как повторить» не совпадает с тем, как
прогон был сделан.**
- §6: «из корня основной папки … `--out-dir ops/metrics`». Так файлы легли бы в `ops/metrics` основной папки.
  Их там нет, они в папке задачи. Карточка говорит «`--out-dir` → `ops/metrics` этой папки».

*Как исправить:* указать фактический корень и `--out-dir`. Для Q36 — одной строкой, чем копия матрицы отличается
(`configs.Q36-A3B` = копия E, модель = id из `/v1/models`, `primary: false`).

#### Nit

**N-1. `baseline.md:256-257`, `:333-334`: два утверждения устарели после продолжения.**
- «Вопрос … вынесен владельцу» — владелец уже ответил (§4 п. 7).
- «Стенд — b10840» — после 23:50 стенд на b10878.

Уточнить обе фразы.

**N-2. `dev-log.md`: пять новых строк записи T-402 в LF внутри файла в CRLF.** 8128 LF против 8123 CR. Git
нормализует при коммите (`core.autocrlf=true`), в репозитории это не останется. У соседних записей рабочая копия
однородна.

### Дефект прогрева `llm-bench.sh`

- Описан достаточно, чтобы воспроизвести: условие (id — путь Windows, `llama-server` без `--alias`), следствие
  (HTTP 500, `first_call_ms` недействителен), причина (JSON собирается строкой).
- Воспроизвёл без сервера. Строка тела из `scripts/llm-bench.sh:713` с id
  `D:\Models\unsloth\Qwen3.6\…gguf` не разбирается как JSON: `Invalid \escape: line 1 column 13`. Сломает её и `"`
  в id.
- Двойник `scripts/llm-bench.ps1:341-347` собирает тело через `ConvertTo-Json`, дефекта там нет. Значит, это ещё и
  расхождение двух скриптов, а шапка `llm-bench.sh:16-19` называет такое находкой ревью.
- Скрипт при не-200 всё равно пишет `first_call_ms` (`:715`) без пометки в CSV. Недействительность видна только
  из `baseline.md`.

**Оценка: отдельная задача (S, devops-engineer; паритет — область T-405), не часть T-402.** T-402 — замер и документ.
Правка скрипта требует своего ревью. Замешанная сюда, она сделала бы прогон 1 результатом «не того» скрипта.
Приоритет выше, чем у бэклога: по моей пробе `:8888` уже отдаёт Qwen3.8-27B под id-путём
(`D:\Models\unsloth\Qwen3.8\Qwen3.8-27B-UD-Q3_K_XL.gguf`, alias тот же). Прогоны 2–3 E на этом стенде упрутся в тот
же 500 на прогреве. Кроме того, имя `configs.E` в матрице не совпадёт с id, и без копии матрицы скрипт пропустит все
фазы. Задачу стоит закрыть до сигнала владельца на прогоны 2–3.

### Что проверено и сошлось

- Пересчёт по `requests[]` (правило nearest-rank скрипта): E — tick 3213/9803/44,5, phase2 2996/7602/48,1,
  group3 5021/9753/47,7. Q36 — 2098/3182/75,2, 2941/3710/115,3, 3665/4244/110,8. `valid_json`, `lang_pass` — 1,0.
  Доля кэша — 0,35/0,37/0,32. Совпадает с CSV, JSON и таблицами `baseline.md`.
- Разбор латентности E (`baseline.md:99-107`) сошёлся до единиц: «прочее» 633–1277 / 615–1176 / 825–1340; всплески
  4, 1 и 4 из 10 (5669–6225, 5679, 5885–6259); средние 86/94/146 токенов. «9 из 10 ≤ 3508» верно.
- Q36: генерация 0,46–2,33 / 0,58–1,03 / 1,24–1,91 с; «прочее» 0,32–2,8 с. Потолок `max_tokens` у phase2 — 320
  (`prompts.jsonl`), так что расчёт «~2,9 с при 110 ток/с, ~6,7 с при 48» верен. Отношение tps 2,32–2,40 верно.
- Вердикты: E phase2 p95 7602 > 5000 → `fail` верно, Q36 3710 ≤ 5000, доли 1,0, ошибок 0 → `pass` верно, группа
  справочно (`*`).
- Стенд, чтение: `:8888` слушает только `127.0.0.1` — верно. `/health` 200, `/props`: `build_info` b10878-4850c7727,
  4 слота × 200 192 — сходится с §1. Модель на момент ревью уже другая (см. выше). Строки §1 до 23:50 (роутер,
  `:62450`, b10840) проверить уже нельзя: роутер снят.

### Гигиена

- Карточка и `review.md` в CRLF. `baseline.md`, CSV и JSON в LF, как и в репозитории.
- Метка `<!-- dev-log T-402 -->` — в формате соседних.
- Изменения в `artifactsDir` — только карточка и dev-log, как положено исполнителю.

### Вне границ ревью: файлы, появившиеся во время ревью

Во время ревью в папке задачи появились два неотслеживаемых файла: `ops/metrics/bench-20260912-0003.csv` и
`bench-E-20260912-0003.json`, время записи 2026-09-12 00:04:48. Судя по содержимому, это прогон E на текущем стенде:
id-путь Qwen3.8, 200 192, напрямую в `llama-server`. phase2 — p95 2585, `pass`; tps 48,0. Я их не создавал: мои
запросы к стенду были только `GET`. В `baseline.md`, карточке и dev-log их нет. Ревью они не проходили.

Уже видно:
- `meta.matrix` — тот же путь профиля (C-1 распространяется и на них).
- `first_call_ms` = 130 при id-пути — вероятно, тот же 500 на прогреве.
- Если прогон войдёт в T-402, его данные — довод для M-1: хвоста ~6 с напрямую нет.

Нужна итерация 2 с этими файлами в границах.

### Предложения в бэклог

1. Отдельная задача: прогрев `llm-bench.sh` через python-помощник (см. раздел выше) и пустой `first_call_ms` при
   не-200 в обоих скриптах. До прогонов 2–3 E.
2. `llm-bench.sh:834` и двойник: путь матрицы вне репозитория писать как имя файла плюс `sha256` содержимого, а не
   абсолютным путём. Причина C-1; хеш заодно делает копию матрицы проверяемой.
3. Поддержать п. 2 бэклога исполнителя (`timings.prompt_ms` в `requests[]`) как условие прогонов 2–3: без него M-1
   и Mi-1 снова останутся гипотезой.
4. П. 3 бэклога исполнителя (алиас модели в матрице) касается теперь и чистого `llama-server` без `--alias`, а не
   только роутера.

## T-402 · ревью #2 · 2026-09-12 · code-reviewer#2

### Границы ревью

Итерация 2 по ревью #1 и прогоны 2–3 E. Прогон 2 сделан devops-engineer#1 (00:03), прогон 3 — оркестратором (00:08,
после перезапуска сервера владельцем). По ответу оркестратора оба входят в T-402. Работа не закоммичена, коммитов в
ветке нет. В `ops/metrics` добавились `bench-20260912-0003.csv`, `bench-E-20260912-0003.json`,
`bench-20260912-0008.csv`, `bench-E-20260912-0008.json` и `diag-T402-20260911.csv`. Переписаны `baseline.md`, карточка
(раздел «Итерация 2», «Главный вывод») и dev-log. `bench-matrix.json` и скрипты не менялись.

Проверены закрытия C-1, M-1, Mi-1…Mi-4, N-1 и N-2, а также новые данные прогонов 2–3. К стенду — один `GET /props`
(первая попытка упала на моём экранировании в оболочке, до стенда не дошла).
## T-433 · ревью #1 · 2026-09-11 · code-reviewer#2

### Границы ревью

Флак `fight-NN` стенда `TestTheHarnessAndTheEncounterOfTheSwarmOnOneBus`. Работа не закоммичена и лежит в `.worktrees/T-433`,
ветка `task/T-433-stand-fight-flake` от эпика `06c9039`. Коммитов в ветке нет. Эпик с тех пор ушёл на `9455016`
(chore security), файлов задачи он не трогает. Изменены `harness.go`, `stand_test.go`, `end_by_fact_test.go` и
dev-log, добавлена карточка `tasks/T-433.md`. Из `artifactsDir` исполнитель менял только карточку и dev-log.
Прочитаны: раздел «### T-433» индекса (`tasks.md:673-687`), карточка, дифф целиком, `harness.go` целиком,
`scenario.go:275-323`, `membus.go:262-335`, `fake_encounter.go:742-789` и `:1903-1918`,
`fake_narrator_stand_test.go:85-145`, хелперы `reaction_test.go:344-418` и `harness_test.go:791-804`.
Менялись только этот раздел и секция «Ревью» карточки.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 1, Nit 2.

Причина найдена и доказана воспроизведением. Порядок доставки между топиками (C-01) даёт два проявления, и оба
исправлены по существу: стенд ждёт асинхронный старт по контракту, а харнесс будит ожидание, которое поздний старт
раньше оставлял висеть. `standTimeout` не менялся. MS и MH у меня красные, серии зелёные. Mi-1 — остаточное узкое
окно того же класса. Оно было и до задачи, правка его не создала. Рекомендую закрыть в итерации 2 этой же задачи:
пять строк, до того как `race` станет required check. Не блокирует.

### Замечания

#### Critical, Major

Нет.

#### Minor

**Mi-1. `shared/testkit/gateway/harness.go:1009-1029`: проверка «бой окончен» и регистрация ожидания стоят под
разными захватами мьютекса. Старт или конец боя, пришедший между ними, ожидания не будит.**
- В `Flee` цепочка такая: `stillFighting` (`:1009`, снимает `h.mu`), затем сборка события `h.root` (`:1012`), затем
  регистрация `h.awaited[correlation]` в `resolved` (`:1027-1029`). Если `opened` (`:539-541`) или `end` (`:587`)
  выполнится в этом окне, `stopWaits` ожидания не найдёт: его ещё нет. Ожидание, зарегистрированное позже, никто не
  разбудит, а двойник на действие вне боя молчит (`fake_encounter.go:1911-1915`). Итог — дедлайн 2 с и тот же текст,
  что у второго проявления: «nobody resolved the flight of player-A».
- Для `end` окно было и до T-433. Новая ветка `opened` его унаследовала. Окно — микросекунды (сборка события), а не
  весь бой, поэтому флак стал на порядки реже, но из класса, который закрывает задача, не ушёл. Под `-race` на двух
  ядрах раннера окно шире.
- Доказательство. В копии в scratch: MT — 100 мс сна между `:1011` и `:1012`, плюс тест «факт конца → `Flee` в
  горутине → через 20 мс старт». Результат: красный 3/3, «nobody resolved the flight of player-A within 200ms». Тот
  же тест без сна — зелёный 3/3, `ErrFightOver`.

*Как исправить:* в `resolved` перепроверить бой под тем же захватом, под которым регистрируется ожидание:

```go
h.mu.Lock()
if f := h.fights[playerID]; f != nil && f.ended != "" {
	h.mu.Unlock()
	return fmt.Errorf("testkit/gateway: %s: the encounter %s ended (%s): %w", what, f.id, f.ended, ErrFightOver)
}
h.awaited[correlation] = &reaction{player: playerID}
h.mu.Unlock()
```

Тогда `opened`/`end` либо прошли раньше регистрации (перепроверка это видит), либо позже (`stopWaits` видит
ожидание). Проверено в scratch: MT с этой правкой зелёный 3/3, пакеты `gateway` и `swarm` целиком зелёные. Статус
персонажа можно перепроверить там же. Детерминированный unit-тест без инъекции задержки не пишется, поэтому
достаточно doc-комментария у `resolved`: почему проверка повторяется под замком. `stillFighting` остаётся ради
текста ошибки до публикации.

#### Nit

**N-1. `tasks/T-433.md` в LF (0 CR на 88 LF).** Соседние карточки, dev-log и `review.md` в CRLF (T-430 — 119/119,
T-401 — 188/188). Привести к CRLF при коммите.

**N-2. MS детерминирован практически, а не строго.** После `release()` (`stand_test.go:113`) горутина харнесса
соревнуется с немедленным `h.Fight`. Главная горутина на практике выигрывает: у меня 6 из 16, у исполнителя 5 из 16.
Число равно числу боёв со смертью персонажа и меняется от прогона. Для воспроизведения этого достаточно, действий не
нужно. MH детерминирован строго: ворота открываются после регистрации ожидания.

### Ответы на вопросы поручения

1. **Доказательство причины.**
   - `lateWorld` (`stand_test.go:246-274`) держит обработчик только группы харнесса на `world_events`.
   - membus держит курсор группы на время доставки (`membus.go:320-335`). Поэтому порядок внутри топика сохраняется,
     а группы State и двойника и другие топики не затронуты.
   - У `Delivery.Deliver` нет тайм-аута обработчика: удерживаемое событие не уходит в DLQ.
   - Поведение шины обёртка не подменяет: события не теряются, не переставляются внутри топика и не дублируются.
     Добавлена только задержка одного топика относительно другого, а её C-01 допускает.
   - На `ctx.Done` обработчик возвращает `nil` без вызова `h`, то есть событие засчитывается без обработки. Это
     бывает только при teardown и на проверки не влияет.
   - Оба проявления воспроизводятся: MS даёт «never heard» (бои со смертью персонажа), MH даёт тайм-аут бегства
     (бои со смертью волка) и красный unit.
   - Вывод «проявление зависит от исхода, исход не закреплён за номером» согласуется с моими прогонами.
2. **Правка `harness.go`.**
   - `opened` работает под `h.mu` (`:523-524`). `ended` читается один раз, `stopWaits` вызывается под тем же захватом.
   - Пробуждение — один `close`+замена `h.changed` на вызов (`:605-608`). `awaitReaction` снимает `wake` под тем же
     мьютексом, что и состояние (`:1079-1083`). Поэтому среди уже ждущих пропущенного пробуждения нет, двойного
     `close` тоже: канал заменяется под замком.
   - Дубликат `encounter.started` (at-least-once) повторно ставит `over` тем же ожиданиям, это идемпотентно.
   - Исключения согласованы с `end`. `correlation = ""` ни с чем не совпадает. У действия, закрывшего бой,
     `named != nil`: предложение стоит раньше факта на том же `system_events`, и его разбирает одна горутина подписки.
   - Конец раньше старта может прийти только из факта: `encounter.ended` идёт по `world_events` после
     `encounter.started` (один публикатор, один топик). Рассуждение doc-комментария `:508-515` верно.
   - Остаток — окно Mi-1: оно до регистрации ожидания, а не среди ждущих.
   - Граница исполнителя (поздний старт старого боя при ожидании в новом) в MVP-1 недостижима. В `ScenarioSkirmish`
     один вход, удары и одно бегство (`scenario.go:171-201`), в visit/party боя нет. Кроме того, `opened`
     перезаписывал `fights[who]` старым оконченным боем и до T-433, так что следующее действие и так получало ложный
     `ErrFightOver` от `stillFighting`. Новой ошибки правка не вводит. С п. 2 бэклога карточки согласен: защита
     сейчас не нужна.
3. **Стенд.**
   - `heardOfTheFight` (`:217-230`) — ожидание по контракту (doc `Fight`, `harness.go:760-765`). Оно ограничено
     `standTimeout`, текст утверждения тот же (`:124-125`).
   - Другие синхронные вопросы к харнессу после `Run` — `h.Version`/`h.Status` (`:141`, `:146`). Их отвечают факты
     `system_events`, того же топика, на котором `Run` ждёт ответ на каждое действие, а не `world_events`. Это не тот
     класс.
   - Число стартов (`:127`) проверяется по журналу, а не по харнессу.
   - Близнец (`fake_narrator_stand_test.go`): `h.Fight` там нет, старты считаются по журналу, нарративы ждёт `waitFor`.
     Второе проявление грозило его `Run` так же и закрыто правкой харнесса.
4. **Владение.**
   - Экспортируемый API `Harness` не менялся: `stopWaits` неэкспортируемый, `lateWorld` и `heardOfTheFight` — в
     `_test`-пакете.
   - `shared/testkit/swarm`, `shared/eventbus` и контракты не менялись, подтверждение tech-lead#2 не нужно.
   - Путь в `shared/testkit/gateway` — через tech-lead#1, как указано в карточке.
5. **Мои прогоны и мутанты** — ниже.

### Мутанты

Копия в scratch (`git ls-files -co --exclude-standard` без `services/`, `mktemp -d`), не `-overlay`. Эталон трёх
файлов лежит отдельно. Базовый прогон копии зелёный. Каждый мутант проверен на применение, после каждого файлы
восстановлены, и `cmp` с эталоном совпал. Копия удалена по сохранённому пути.

| Мутант | Правка | Результат |
|---|---|---|
| M0 (контрольный) | `func broken( {` в конце `stand_test.go` | красный: сборка (`setup failed`) |
| MS | `heardOfTheFight` → немедленный `h.Fight` | красный: `TestTheStandWhen…`, 6 боёв из 16, «never heard» |
| MH | `h.stopWaits(late, ended, "")` → `_ = ended` | красный: 10 боёв из 16 (тайм-аут бегства 2 с) и unit (200 мс) |
| MT (свой) | 100 мс сна в `Flee` между `stillFighting` и `h.root` + scratch-тест окна | красный 3/3 (Mi-1); без сна зелёный 3/3; с правкой из Mi-1 зелёный 3/3 |

Основной стенд на MS и MH в одиночном прогоне зелёный, как и должно быть: без нагрузки флак не ловится (T-401).

### Прогоны (go1.26.8, windows/amd64; `-race` недоступен — нет cgo)

- Две серии подряд `go test -count=5 -timeout 20m` по `RACE_PKGS` (`Makefile:194`): 11 пакетов ok в каждой,
  `shared/testkit/gateway` — 15.4 и 15.8 с. Мутанты шли параллельно со второй серией, то есть под нагрузкой.
- `go test -short -count=1 ./...`: 27 пакетов ok, 0 FAIL.
- `golangci-lint run ./...` (2.13.2): 0 issues. `gofmt -l shared/testkit/gateway` — пусто.
- e2e не запускал: T-433 его не затрагивает, у исполнителя e2e ok.

### Гигиена

- Go-файлы в LF. Запись dev-log в CRLF (8126/8126), метка `<!-- dev-log T-433 -->` в формате соседних записей.
- Карточка в LF — см. N-1.

### Предложения в бэклог

1. Если Mi-1 не войдёт в итерацию 2 — отдельная задача до required check `race`: перепроверка боя под замком
   регистрации в `resolved`, закрывает окно и для `end`.
2. С п. 1 и 2 бэклога карточки согласен (источник id по публикатору; помнить у ожидания, в какой бой ушло действие).
## T-432 · ревью #1 · 2026-09-11 · code-reviewer#1

### Границы ревью

Незакоммиченные правки в `.worktrees/T-432`, ветка `task/T-432-ollama-literal` от эпика `ae3eb0a`. Изменены
`scripts/compose-lint.sh` (шапка правила 8 и проверка записи, `:1376-1395`), `testdata/compose-lint/README.md`,
`infrastructure.md` §3.1.1 п. 8 (`:333`) и `dev-log.md`. Новые файлы: четыре bad-фикстуры, одна good и карточка
`tasks/T-432.md`. Прочитаны раздел «### T-432» индекса (`tasks.md:653-669`), `contracts.md` §16 п. 5 (`:620-628`),
`shared/env/infra.go:62-77`, блок `ollama` в `docker-compose.yml:425-436` и весь путь значения в скрипте:
`interpolations`, `scalars`, `text_of`, `check`, раннер `--fixtures`. Файлы композиции, `build/versions.env`,
`.github/ci.env` и `.env.example` не менялись (`git diff --quiet ae3eb0a` → 0). Контейнеры, стек и LLM не
трогались, `.env` не открывался, из `docker compose config` печатались только коды и хеши. Решения оркестратора —
не замечания: `secrets-scan` красный из-за `.gitleaksignore` (чинится на эпике); отказ для значения из другой
переменной остаётся.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 2, Nit 1.

Проверка сделана верно. Литерал отвергается в отображении и в элементе списка, в том числе литерал, равный
умолчанию, а также `$${…}`. Отказ называет единственную форму с умолчанием `infra.go`. Необъявленные `OLLAMA_*`
и правило 5 не затронуты, хеши модели владельца прежние. Две претензии — Minor. Условие «ничего, кроме своей
подстановки» не закреплено ни одной фикстурой: мой мутант m2 выживает. Кроме того, фраза «единственная допустимая
форма» шире того, что проверяет правило 8.

### Проверка DoD и вопросов задания

1. **Фикстуры.** Три обязательные на месте: `bad-ollama-literal` (отображение, `4` ≠ `1`), `bad-ollama-literal-list`
   (`- OLLAMA_KV_CACHE_TYPE=q8_0`), `bad-ollama-literal-default` (`2` = умолчанию). Четвёртая — `bad-ollama-foreign`.
   У каждой два `expect-text`: причина отказа и форма `${OLLAMA_X:-<умолчание>}, the default of shared/env/infra.go`.
   Раннер требует, чтобы сработало ровно одно правило (`compose-lint.sh:289`), и все четыре дают только правило 8.
   У `bad-ollama-foreign` умолчание `MV_LLM_SLOTS` равно `1` (`vars.go:223`), так что `check` не добавляет второго
   отказа. `good-ollama-list` закрепляет допустимую форму в элементах списка и необъявленный `OLLAMA_DEBUG=0`.
2. **Рабочая композиция и CI.** `make -o secrets-scan ci` в `.worktrees/T-432` → 0 за 57 с. Внутри: lint «0 issues»,
   `test-race` SKIPPED (ОВ-5), `env check` (67 переменных), govulncheck «affected by 0», «compose-lint: ok — 15
   services in 3 file(s)», «fixtures ok — 50 bad, 10 good», e2e ok.
3. **Хеши владельца.** `--project-directory` основной папки, её `.env` и `build/versions.env`, `-f` из основной
   папки и из T-432: `config -q` → 0. Без профиля — `0870f33d758b0688`, `gpu` — `ddc89274a7f7e5b8`,
   `gpu+memory+dev` — `80c0aac4ebfcbcd7`. У обеих папок одинаковые, совпадают с заявленными.
4. **Фраза в §3.1.1 п. 8** (`infrastructure.md:333`) есть. Она покрывает литерал в обеих записях, литерал, равный
   умолчанию, и значение из другой переменной («как и значение из другой переменной»). Отказ называет форму,
   необъявленные `OLLAMA_*` правило не трогает. **Решение оркестратора о чужой переменной документы покрывают.**
   В §3.1.1 оно названо прямо, а `contracts.md:622` («Допустима одна форма — `${OLLAMA_X:-…}`») покрывает его по
   смыслу. Прямо чужая переменная в контракте не названа — это предложение в бэклог, п. 1, а не замечание.
5. **Необъявленные `OLLAMA_*` и правило 5.** `external` строится только из `DeclareExternal` с непустым умолчанием
   и префиксом `OLLAMA_` (`compose-lint.sh:1140-1145`). Прогон `OLLAMA_DEBUG: 1` + `OLLAMA_HOST: 127.0.0.1:11434`
   → 0. Правило 5 в diff не менялось.
6. **`$${…}` — литерал.** `interpolations()` пропускает `$$`. `OLLAMA_NUM_PARALLEL: $${OLLAMA_NUM_PARALLEL:-1}` →
   «carries the literal '$${OLLAMA_NUM_PARALLEL:-1}'». Верно: контейнер получает текст.
7. **Ключ `OLLAMA_*` в любом месте композиции (риск исполнителя).** Проверено: `labels`, `build.args`, элемент
   `command` (`"OLLAMA_NUM_PARALLEL=4"`), якорь `x-…` со слиянием `<<:` (одно сообщение на два места). Во всех
   случаях — отказ правила 8. **Приемлемо.** Область та же, что у существующей проверки «ключ без значения»
   (`:1367`) и у правила 3 («wherever they sit»). `OLLAMA_X=4` в `command` и в якоре действительно перебивает
   `.env`. Ложное срабатывание возможно только на метке `OLLAMA_*` — это надуманный случай, и отказ громкий.
8. **`${OLLAMA_X:?…}` отдан правилу 7 — верно лишь для файла, который загружается всегда.** `OLLAMA_*` в
   `.env.example` закомментированы (`:76-81`), поэтому `${OLLAMA_NUM_PARALLEL:?…}` и `${…?…}` в
   `docker-compose.yml` падают по первой половине правила 7 (проверено, отказ `[rule 7]`). Но файлы профилей
   (`bot`, `legacy`) правило 7 не проверяет (§3.1.1 п. 7), и там `:?` проходит молча. Проходит и
   `${OLLAMA_X-1}` без двоеточия — где угодно. Класс T-408 это не порождает: значение `.env` в контейнер доходит, а
   `:?` отказывает громко. Но формулировка «единственная допустимая форма» теперь стоит в отказе и в §3.1.1, а
   правило 8 допускает три формы. См. Mi-2.
9. **Прочие краевые случаи.** Проверено: пустое значение `- OLLAMA_KEEP_ALIVE=` → «literal ''»; `true` →
   «literal 'true'». Для `$OLLAMA_X` и `${OLLAMA_X:-${MV_LLM_SLOTS:-1}}` — по одному отказу, из `check`, без
   двойного срабатывания.

### Мутанты (копия папки в своём подкаталоге scratch, `git init`, по очереди, контрольный первым)

| # | Мутант | Настоящие | `--fixtures` | Итог |
|---|---|---|---|---|
| m0 | контроль: синтаксическая ошибка во второй Python-половине | 1 | 1, «no rule at all» у «плохих» | контроль работает |
| m1 | повтор p4 исполнителя: условие `if not own:` | 0 | 1, `bad-ollama-foreign` «passed the linter» | убит |
| m2 | свой: из условия убрано `own[0].text == item.value` | 0 | **0, «50 bad, 10 good»** | **выжил** → Mi-1 |
| m9 | тождественный | 0 | 0, «50 bad, 10 good» | чистый |

m2 меняет поведение: `OLLAMA_KEEP_ALIVE: "${OLLAMA_KEEP_ALIVE:--1}m"` без мутанта — отказ, с мутантом — «ok».
После прогона копия удалена по сохранённому пути.

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| Mi-1 | Minor | `scripts/compose-lint.sh:1385`, `testdata/compose-lint/` | Третья часть условия, `own[0].text == item.value` (своя подстановка, но в окружении текста), не закреплена ни одной фикстурой: мутант m2 проходит «50 bad, 10 good». Именно она отличает «ровно своя подстановка» от «есть своя подстановка». Без неё `"${OLLAMA_KEEP_ALIVE:--1}m"` пройдёт незамеченным | Добавить `bad-ollama-wrapped.yml`, например `OLLAMA_KEEP_ALIVE: "${OLLAMA_KEEP_ALIVE:--1}m"`, с `expect-text` отказа и формы. Строку — в README фикстур. Вместе с N-1 |
| Mi-2 | Minor | `scripts/compose-lint.sh:1261-1262`, `:1305`; `infrastructure.md:333`; текст отказа `:1393` | Отказ и §3.1.1 п. 8 говорят «the one form allowed / единственная допустимая форма `${OLLAMA_X:-d}`», но правило 8 пропускает `${OLLAMA_X-d}` (везде) и `${OLLAMA_X:?…}`/`${…?…}` (в файлах профилей правило 7 не действует). Это было и до T-432, но теперь документ и отказ заявляют больше, чем проверяется. Вариант с `:?` для `OLLAMA_*` контракт v0.7 отверг прямо (`contracts.md:621`) | На выбор оркестратора: (а) в `check` для `name in external` отвергать `op` из `-`, `?`, `:?` с той же формой в тексте. Фикстура нужна на `${OLLAMA_X-d}`. Для `:?` отдельной фикстуры не будет: единственный `-f` раннера — это файл, который загружается всегда, и правило 7 сработает рядом с правилом 8, а раннер требует ровно одно правило (`compose-lint.sh:289`). Этот случай — разовым прогоном или мутантом, с записью в карточке; (б) вынести в бэклог, а в §3.1.1 п. 8 и в тексте отказа до тех пор писать «форма `${OLLAMA_X:-d}`» без «единственная». Блокером не считаю |
| N-1 | Nit | `scripts/compose-lint.sh:1386-1387` | Для своей подстановки в окружении текста (`"${OLLAMA_KEEP_ALIVE:--1}m"`) отказ говорит «takes its value from …, not from OLLAMA_KEEP_ALIVE itself». Это неправда: значение как раз из неё, только с хвостом | Третья ветка текста: если среди `own` есть `item.key` — «wraps ${…} in other text; the value must be the interpolation alone». Закрепить фикстурой из Mi-1 |

### Предложения в бэклог

1. system-architect, при следующей ревизии контрактов: в `contracts.md` §16 п. 5 (v0.9) прямо назвать значение из
   другой переменной (`OLLAMA_NUM_PARALLEL: ${MV_LLM_SLOTS:-1}`) нарушением наравне с литералом. Сейчас это
   выводится из «допустима одна форма», а §3.1.1 уже говорит прямо.
2. Если Mi-2 уйдёт в бэклог — отдельная S-задача: для `OLLAMA_*` правило 8 принимает ровно `:-`, фикстуры на `-` и
   `:?`.

### Риски и допущения

- Метки `OLLAMA_*` в `labels` будут отвергаться с текстом про контейнер. Такой метки в композиции нет, и
  появиться ей незачем — принимаю как цену единой области проверки.
- `make ci` целиком красный из-за `secrets-scan` (`.gitleaksignore`, T-431) — не T-432. Проверено
  `make -o secrets-scan ci`. Эпик станет зелёным после коммита оркестратора.
- Правки `artifactsDir` в ветке задачи (карточка, `dev-log.md`, README фикстур, `infrastructure.md`) идут по
  раскладке оркестратора и замечанием не считаются.

## T-432 · ревью #2 · 2026-09-11 · code-reviewer#1

### Границы ревью

Итерация 2 — незакоммиченные правки в `.worktrees/T-432`, база прежняя (`ae3eb0a`). В `scripts/compose-lint.sh`
изменены шапка правила 8 (`:59-68`), ветка `-` в `check` (`:1267-1279`) и три ветки текста отказа
(`:1400-1422`). Изменены также `infrastructure.md:333` и README фикстур. Новые фикстуры — `bad-ollama-wrapped`
и `bad-ollama-plain-dash`. `expect-text` четырёх прежних фикстур переведены на новую фразу. Раздел «Итерация 2»
карточки прочитан. Проверены закрытие Mi-1, Mi-2 и N-1 ревью #1, а также регрессия от этих правок. Решение
оркестратора по Mi-2 — не замечание: вариант (а) частично, `?`/`:?` отданы правилу 7, пробел в файлах профилей
вынесен в бэклог. Контейнеры, стек и LLM не трогались, `.env` не открывался, из `docker compose config`
печатались только коды и хеши.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 4.

Mi-1, Mi-2 (в объёме решения оркестратора) и N-1 закрыты. Мутанты m2 и на ветку `-` убиты. У MV_* поведение
побайтно прежнее, ложных отказов на рабочей композиции нет. Остались Nit: остатки «one allowed form» вне
отказа и §3.1.1, два неточных текста в редких ветках и оборот в шапке и §3.1.1.

### Закрытие ревью #1

| # | Статус | Чем подтверждено |
|---|---|---|
| Mi-1 | **закрыто** | `bad-ollama-wrapped.yml`. Мутант n1 (= m2, из условия `:1402` убрано `own[0].text == item.value`): «bad-ollama-wrapped broke rule 8 but passed the linter» |
| Mi-2 | **закрыто в объёме решения** | `${OLLAMA_X-d}` отвергается (`:1267`), фикстура `bad-ollama-plain-dash`. Мутант n2 (ветка выключена) убит на ней. Отказ больше не обещает «the one form»: «rule 8 accepts `${OLLAMA_X:-d}` … (a requirement with `:?` is rule 7's)». §3.1.1 п. 8 и шапка прямо говорят, что правило 7 файлы профилей не читает. Пробел в профилях — в бэклоге карточки, п. 1 |
| N-1 | **закрыто** | Третья ветка «wraps … in other text …; the value must be the interpolation alone» (`:1411`), три `expect-text` в `bad-ollama-wrapped` |

### Проверки

1. **`make -o secrets-scan ci`** в T-432 → 0 за 55 с. Внутри: lint «0 issues», `test-race` SKIPPED (ОВ-5),
   `env check` (67 переменных), govulncheck «affected by 0», «compose-lint: ok — 15 services in 3 file(s)»,
   «fixtures ok — 52 bad, 10 good», e2e ok. Сам `secrets-scan` в этой ветке не запускался: исправление
   `9455016` лежит на эпике, а ветка от `ae3eb0a`.
2. **Хеши владельца** (`--project-directory` основной папки, её `.env` и `build/versions.env`, `-f` из T-432):
   `config -q` → 0, `0870f33d758b0688` / `ddc89274a7f7e5b8` / `80c0aac4ebfcbcd7` — прежние. Файлы композиции,
   `build/versions.env`, `.github/ci.env`, `.env.example` и `shared/` равны `ae3eb0a`.
3. **MV_* не задеты.** Прогнал одни и те же файлы старым скриптом (`ae3eb0a`) и новым:
   - `${MV_LLM_SLOTS-1}` → 0 у обоих;
   - `${MV_LLM_SLOTS-2}` → один и тот же отказ «defaults to '2'»;
   - `${MV_KAFKA_BROKERS-redpanda:9092}` → один и тот же отказ;
   - `${MV_LLM_SLOTS-1}` в `command` → 0 у обоих;
   - `${MV_MEMORY_URL-}` → 0 у обоих.

   Вывод побайтно совпадает. Формы `${MV_X-d}` нет ни в композиции, ни в фикстурах. Поэтому мутант n3
   (ветка `-` расширена на MV_*) выживает: «52 bad, 10 good», рабочая композиция чистая. Это ожидаемо — задача
   MV_* не расширяет. Замечанием не считаю, см. бэклог, п. 1.
4. **Ложных отказов нет.** Рабочая композиция → 0. `good-ollama-list` и мой файл
   `- OLLAMA_NUM_PARALLEL=${OLLAMA_NUM_PARALLEL:-1}` + `- OLLAMA_DEBUG=1` → 0 у обоих скриптов. Новые отказы по
   сравнению со старым скриптом появились только там, где их требует решение: `-` в `environment` и в
   `command`, своя подстановка с хвостом (`"${…--1}m"`, `"${MV_LLM_SLOTS:-1}${OLLAMA_NUM_PARALLEL:-1}"`).
   `${OLLAMA_X:?…}` в основном файле — по-прежнему только правило 7.
5. **Тексты отказов по веткам.**
   - литерал — «carries the literal '4': .env cannot override it … whatever the value» — точно;
   - чужая переменная — «takes its value from '${MV_LLM_SLOTS:-1}', not from OLLAMA_NUM_PARALLEL itself: .env
     cannot override it» — точно. Исключение одно: своя подстановка вложена в умолчание чужой (N-2);
   - своя с хвостом — «wraps ${…} in other text (…): … reaches the container with compose's text around it;
     the value must be the interpolation alone» — точно, в том числе когда чужая подстановка стоит перед своей;
   - форма `-` — «falls back to the default only when … is unset: a .env line `X=` hands the container an EMPTY
     X instead of the default of infra.go ('1'); write ${X:-1}» — точно, пока `d` равно умолчанию. При другом
     `d` расхождение умолчаний больше не называется (N-3).

   Если в одной записи и хвост, и `-`, выходят два отказа правила 8. Оба верны, и раннер это допускает.

### Мутанты (копия папки в своём подкаталоге scratch, `git init`, якорь найден ровно один раз, по очереди, контрольный первым)

| # | Мутант | Настоящие | `--fixtures` | Итог |
|---|---|---|---|---|
| n0 | контроль: синтаксическая ошибка во второй Python-половине | 1 | 1, «no rule at all» у «плохих» | контроль работает |
| n1 | m2 ревью #1: из `:1402` убрано `own[0].text == item.value` | 0 | 1, `bad-ollama-wrapped` «passed the linter» | убит |
| n2 | ветка `-` выключена (`if False and op == "-" …`) | 0 | 1, `bad-ollama-plain-dash` «passed the linter» | убит |
| n3 | ветка `-` расширена на MV_* (`if op == "-":`) | 0 | 0, «52 bad, 10 good» | выжил — ожидаемо, см. п. 3 |
| n9 | тождественный | 0 | 0, «52 bad, 10 good» | чистый |

После прогона копия удалена по сохранённому пути.

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| N-1 | Nit | `scripts/compose-lint.sh:1399`; `testdata/compose-lint/bad-ollama-foreign.yml:1`, `:5`; `good-ollama-list.yml:2`; `testdata/compose-lint/README.md:73`, `:98`; `compose-lint.sh:62-63` | Отказ и §3.1.1 от «единственной формы» отказались, а комментарий, шапки двух фикстур и две строки README по-прежнему пишут «the one allowed form» / «one form only». В шапке правила 8 оборот «.env could then never set it as set» не читается | Заменить на «the form rule 8 accepts» / «`${OLLAMA_X:-d}`». В шапке — «.env could then not set it, or not as set» |
| N-2 | Nit | `scripts/compose-lint.sh:1416` | Если своя подстановка вложена в умолчание чужой (`${MV_LLM_SLOTS:-${OLLAMA_NUM_PARALLEL:-1}}`), отказ говорит «never reaches the container, whatever the value». Это неправда: при молчащем `MV_LLM_SLOTS` значение `.env` доходит. Вторым отказом ту же запись и так ловит `check` («falls back to another variable») | Искать `mine` и во вложенных (`walk(own)`), либо в ветке «чужая» писать «.env reaches it only through another variable». Случай надуманный, фикстура не нужна |
| N-3 | Nit | `scripts/compose-lint.sh:1267-1279` | Ветка `-` делает `return` раньше сверки умолчания. У `${OLLAMA_NUM_PARALLEL-4}` отказ больше не говорит, что `4` ≠ `1` (старый скрипт говорил). У `${OLLAMA_X-}` «falls back to the default» относится к пустому умолчанию. Совет `write ${X:-1}` при этом верный, так что оператор всё равно придёт к правильной форме | Если `ref.arg != default` и `ref.inner` пуст — дописать «, and its default {arg!r} is not the {default!r} of infra.go» |
| N-4 | Nit | `Docs/dev-team/architecture/infrastructure.md:333` | «…или своя подстановка с окружающим текстом: `.env` его не переопределит или дойдёт не тем значением» — у второй половины нет подлежащего, а «его» рассогласовано со словом «подстановка» | «…: `.env` такое значение не переопределит, а в случае текста вокруг — значение `.env` дойдёт не таким, каким задано» |

### Предложения в бэклог

1. MV_* с `-` без двоеточия (`${MV_X-d}`): строка `MV_X=` в `.env` отдаёт процессу пустое значение. `shared/env`
   считает пустое значение заданным, и для списка допуска это «никого» — тот же довод, что Mi-2 T-411 для
   `${MV_X}`. Отдельной задачей решить, отвергать ли `-` и для MV_*. Тогда же закрепить решение фикстурой: сейчас
   его не держит ничего (мутант n3 выживает).
2. Пункты 1–2 бэклога карточки (профили bot/legacy для `:?`, прямая формулировка в `contracts.md` §16 п. 5)
   поддерживаю.

### Риски и допущения

- Новые отказы в `command` (`"OLLAMA_X=${OLLAMA_X-1} ollama serve"`) — та же область проверки, что принята в
  ревью #1. В рабочей композиции таких строк нет.
- `secrets-scan` в ветке T-432 будет красным, пока ветку не синхронизируют с эпиком (`9455016`). К правкам
  задачи это не относится.

## T-406 · ревью #1 · 2026-09-12 · code-reviewer#1

### Границы ревью

Незакоммиченные правки в `.worktrees/T-406`, ветка `task/T-406-drop-ready-subscriber` от эпика `48b88fd`; эпик с тех
пор не сдвигался, так что diff задачи равен рабочей копии. Изменены `shared/testkit/state/state.go`, `state_test.go`,
`shared/testkit/contract/contract.go`, карточка `tasks/T-406.md` и `dev-log.md` (только дописан раздел). Прочитаны
раздел «### T-406» индекса (`tasks.md:469-473`), ADR-022, `contracts.md` C-01 «Старт подписки (v1.2)» (`:220`),
`kafka.go:140-261` и `:402-418` (`Subscribe` с `FirstOffset`, `ReadRange` → `journalReader` → `SetOffset(from)`),
`membus.go:271-423` и `:489-505`, обе цели набора (`membus_test.go`, `redpanda_integration_test.go`), помощники
набора (`contract.go:1337-1585`), а также `kafka-go v0.4.51` `reader.go:1025-1051` и `:1382-1397`. Redpanda,
контейнеры и LLM не запускались, `.env` не открывался. Метка `[contract-change]` — решение оркестратора, не
замечание.

### Вердикт

**ПРИНЯТЬ** — Critical 0, Major 0, Minor 0, Nit 3.

Все замечания ревью #1 закрыты, регрессий нет. Числа прогонов 2–3 сходятся с `requests[]`. Прогоны честно помечены:
ручной прогрев, недействительный `first_call_ms`, контекст вне ячеек, неизвестный KV. Вердикт «2 `pass` из 2
зачётных» нигде не выдан за решение U-2.

### Закрытие замечаний ревью #1

| № | Закрыто | Чем проверено |
|---|---|---|
| C-1 | да | Грэп по всем девяти файлам замера (четыре CSV, четыре JSON, `diag-T402-20260911.csv`) и `baseline.md`: `Users`, `AppData`, `Temp`, `scratch`, `CD86`, UUID сессии и шаблон UUID, `bearer`, `authoriz`, `api_key`, `sk-`, `token=`, `password`, `secret`. Совпадения только `temp` в профиле сэмплинга (`baseline.md:45`) и слово «искажений» (`:359`) — ложные. Все четыре `meta.matrix` — нейтральное описание. JSON разбираются. «Изменена ровно одна строка» подтверждено по размеру: у двух JSON, чей исходный размер я знал из ревью #1, новый размер совпал до байта с «старый − старое значение + новое значение». `cells` и `requests` дают те же числа, что в ревью #1. Ручная правка отмечена в `baseline.md:75-83`. |
| M-1 | да | `baseline.md:162-188`: гипотеза, доводы «за» и «против», итог «не установлено», для U-2 не важно. §3 и §4 п. 6(а) (`:360-361`) и «Главный вывод» карточки (`T-402.md:56-63`) приведены к той же формулировке. Сырые числа в `diag-T402-20260911.csv`, 16 строк, без путей. Сверка с текстом: через роутер «необъяснённое» 5340 и 5634 в 2 из 5, напрямую 117–307, после Q36 128–238 мс (текст — «0,13–0,24 с»), вызов на 1 токен 539 мс. Всё совпадает. |
| Mi-1 | да | `baseline.md:248-262`: 11 из 30, индексы медленных запросов совпали с моими, медиана названа. Гипотеза прогрева отклонена с двумя доводами. |
| Mi-2 | да | `:263-266`, `:275`: среднее 0,006 и максимум 0,034. Фраза о характере латиницы убрана. |
| Mi-3 | да | §1 `:36`: перечень программ, путь к бинарнику и имя антивируса убраны. |
| Mi-4 | да | §6 `:421-442`: корень для `.env`, скрипт и `--out-dir` из папки задачи, таблица отличий копий матрицы. |
| N-1 | да | «Вынесен владельцу» и «стенд — b10840» ушли, §1 разбит по трём периодам. |
| N-2 | да | `dev-log.md`: 8130 LF и 8130 CR. |

### Прогоны 2–3: пересчёт и пометки

- **Пересчёт** по `requests[]`, правило nearest-rank скрипта.
  - Прогон 2: tick 3158 / 4968 / 44,5; phase2 2422 / 2585 / 48,0; group3 3900 / 4382 / 47,6.
  - Прогон 3: 3236 / 6195 / 42,0; 2562 / 2916 / 46,4; 4213 / 4772 / 45,6.
  - `valid_json` и `lang_pass` 1,0, ошибок 0.
  - CSV и `cells` JSON совпадают по всем колонкам у всех четырёх прогонов.
- **Разбор латентности** (`:147-152`) сошёлся: генерация, «прочее» 442–824 мс, 0 из 60 выше 2 с, средние и максимумы
  `completion_tokens`.
- **Прочие числа** сошлись:
  - латиница — максимум 0,0269 (прогон 2, group3);
  - «9 из 30 выше 5,6 с» в прогоне 1;
  - отношение tps Q36/E 2,40–2,48;
  - максимум ответа `phase2` 112 токенов.
- **Пометки** на месте. Ручной прогрев 698 и 570 мс и недействительный `first_call_ms` (130 и 127) — `:130-137` и §6.
  Контекст вне ячеек — сноска ¹, §3, §5. KV «не известно» — §1 и в каждой строке. Выборка VRAM в прогоне 3 честно
  названа снимком.
- **Вердикт** «2 `pass` из 2 зачётных, U-2 ждёт прогона 4» (`:323-326`, §5 `:380-386`, `T-402.md:133-134`) подан как
  промежуточное состояние. Базовая конфигурация в §5 — «—». Отдельно вынесено, что architect#1 решает, считать ли
  ячейку вне матрицы ячейкой решения. Незачёт прогона 1 оформлен как решение оркестратора.
- **Стенд** (`GET /props`): b10878-4850c7727, 4 слота × 200 192, `Qwen3.8-27B-UD-Q3_K_XL.gguf` — совпадает с §1,
  период 3.

### Замечания

#### Critical, Major, Minor

Нет.

#### Nit

**N-1. `baseline.md:156-158`: «p95 каждой фазы даёт самый длинный ответ, а не накладные» — для `phase2` прогона 2 это
не так.**
- p95-запрос (`df-03`, 2585 мс) — 92 токена при максимуме фазы 97. Его генерация, 1908 мс, тоже не самая длинная
  (самая длинная — 2010).
- Зато у него самые большие в фазе накладные, 677 мс. Вывод «многосекундных накладных нет» верен, неточна только
  формулировка.
- Для остальных трёх ячеек (tick в обоих прогонах, phase2 прогона 3) утверждение верно.

Как исправить: «p95 = генерация самого длинного или почти самого длинного ответа плюс ≤ 0,8 с».

**N-2. `baseline.md:328-329`: «до ~6,7–7,6 с одной генерации» при потолке `phase2` посчитано по tps тика (42,0).**
Tps `phase2` — 46,4–48,1, то есть ~6,7–6,9 с. §2.5 (`:296-297`) даёт «~6,7–7 с», и числа расходятся.

**N-3. `baseline.md:98`, `:162-169`: две мелкие неполноты.**
- У прогона 3 не указано время окончания («00:08:54–…»). По времени записи CSV это 00:10:36.
- Не определено, что `unexplained_ms` в `diag-T402-20260911.csv` — это латентность − `prompt_ms` − `predicted_ms`, а
  «прочее» в таблицах — латентность − `predicted_ms`. Довод «против» 1 сравнивает одно с другим. Вывод от этого не
  меняется: `prompt_ms` там 0,1–0,5 с.

### Гигиена

- `dev-log.md`, карточка и `review.md` в CRLF, число CR равно числу LF.
- `baseline.md` и `diag-T402-20260911.csv` в LF, как CSV и JSON скрипта.
- Изменения в `artifactsDir` — только карточка и dev-log.
- Запись итерации 1 в dev-log сохранила старую формулировку о роутере. Журнал только дополняется, поправку несёт
  запись итерации 2 — замечанием не считаю.

### Предложения в бэклог

Новых нет. Пп. 1–2 ревью #1 ушли в T-434 (решение оркестратора). П. 3 (`timings.prompt_ms`) стоит проверить на
включение в T-434: без него причина всплесков Q36 и прогона 1 так и останется гипотезой (`baseline.md:187-188`).

### Вне границ итерации 2: файлы прогона 4 E

По сообщению оркестратора, во время ревью сделан прогон 4 E: `ops/metrics/bench-20260912-0020.csv` и
`bench-E-20260912-0020.json`, старт 00:20:02, сервер перезапущен владельцем в 00:19:00. В `baseline.md` его пока нет,
его внесёт итерация 3, поэтому замечанием это не считаю. Проверил только сами файлы.
- **Утечки.** Грэп по тем же шаблонам, что для C-1, пуст. `meta.matrix` — то же нейтральное описание.
- **Числа.** Пересчёт по `requests[]` совпал с `cells`, CSV совпал с JSON по всем колонкам.
  - tick 2773 / 4488 / 40,3;
  - phase2 2590 / 3533 / 42,3 — `pass`;
  - group3 3980 / 4720 / 42,8 — `pass*`;
  - `valid_json` и `lang_pass` 1,0, ошибок 0;
  - «прочее» 486–854 мс, 0 из 30 выше 2 с.
- **Пометки для итерации 3.** Вносить с теми же пометками, что прогоны 2–3.
  - `first_call_ms` = 128 недействителен, ручной тёплый вызов 536 мс.
  - `num_ctx` 200 192 вне ячеек, KV `unknown`.
  - tps `phase2` 42,3 — ниже, чем в прогонах 2–3. После этого прогона нижняя граница в N-2 (~7,6 с при потолке 320
    токенов) становится верной и для `phase2`.
`readySubscriber` удалён полностью. `Start` стал проще и не хуже прежнего по гонкам. Новый кейс детерминирован, от
порядка кейсов не зависит и закрывает дыру, которую прежний набор не видел: мой повтор M3 валит только его.

### Проверка по пунктам задания

1. **Удаление.** `git grep` по дереву вне `Docs/dev-team` (и по `services/`) на `readySubscriber`, `SubscribeReady`,
   `readyReporter`, `readyBus`, `racedTheGate`, `gateWindow`, `TestStartSubscribesBeforeItAnnounces` и
   `TestTheBusOfTheTestsDoesNotReportReadiness` — пусто. Остались только упоминания в ADR-022, `contracts.md`,
   индексе и карточке, то есть история.
   `Start` (`state.go:248-269`): горутина регистрируется в `subscription` до `go`, выходит, когда `Subscribe`
   возвращается по `ctx` (обе шины возвращают `nil`), а `Wait` её дожидается. Утечки нет. Если `announceRecovery`
   падает, подписка живёт до отмены `ctx` — так было и раньше, и все вызывающие (`stand_test.go:99`,
   `harness_test.go`, `fake_contexts_test.go`) отменяют `ctx` и зовут `Wait`. Ошибка подписки пишется в `subErr` под
   `mu` и в лог, `Wait` её отдаёт — не теряется. Прежде в запасной ветке `ready` закрывался до `Subscribe`, так что
   до `Start` ошибка доходила только как исход гонки в `select`. Поведение фактически прежнее, пропала лишь
   случайность. На ошибку подписки из `Start` никто не опирался: `cmd/multiverse` использует `swarm.FakeContext`, а
   не `FakeState.Start`.
2. **Отклонение (удалён и `TestStartSubscribesBeforeItAnnounces`) — оправдано.** Тест держался на двойнике с
   `SubscribeReady` и после удаления интерфейса был бы красным по построению. Переписать его на порядок вызовов
   `Subscribe`/`Publish` без рукопожатия нельзя: `Subscribe` зовётся из горутины, порядок не гарантирован, и тест
   стал бы нестабильным. ADR-022 п. 2 прямо снимает с порядка роль защиты. Гарантию «ничего опубликованное до
   `Start` не теряется» держат две проверки. Со стороны заглушки — `TestNothingPublishedBeforeStartIsLost`:
   мутант M2 исполнителя (группа с конца) его валит. Со стороны шины — новый кейс: его валят M1, M2 и M3.
3. **Новый кейс `ANewGroupStartsAtTheFirstOffset` (`contract.go:456-521`).**
   - *Детерминированность.* Тайминга нет — только `waitFor` по условию с общим `Timeout`. Группа и мир свои
     (`runSeq`). `first` и `ours` защищены `mu`. Порядок `defer` верный: `mu.Unlock` раньше `sub.stop`, взаимной
     блокировки при `Fatalf` нет.
   - *Порядок кейсов.* Кейс не зависит от места в наборе, и это проверено в копии. Первым — зелёный (`first = begins
     = 0`). Перед `Close`, то есть после всех `Append` с битыми телами, — тоже зелёный: `ReadRange` и новая группа
     идут через один и тот же `Delivery` и одинаково пропускают непрочитанные тела, мимо обработчика.
   - *kafka: `SetOffset(0)` против `FirstOffset`.* Риск исполнителя про retention снят кодом библиотеки. На
     `OffsetOutOfRange` читатель с явным офсетом делает `offset < first → offset = first` (`kafka-go` `reader.go:1393-1397`).
     Значит, `ReadRange` от 0 на урезанном топике отдаёт первый доступный офсет. Новая группа с `FirstOffset` (-2)
     начинает с того же начала лога, так что `first == begins` держится и на брокере с удалённым началом.
     Остаётся только гонка retention между двумя чтениями — на testcontainers её нет.
   - *Объём и `Timeout` на Redpanda.* «Несколько сотен событий» — завышено. На свежем брокере testcontainers к
     кейсу №10 в `player_events` 17 событий: 5+4+3+1+2+2, тот же набор, что даёт «began at offset 17» в M3 на membus.
     Плюс 3 своих. Это 20 синхронных коммитов группы и одно вступление в группу (его и так платит каждый кейс с
     подпиской) — далеко от 15 с. Смежность `base…base+2` требует, чтобы в топик никто не писал параллельно. Это то
     же допущение, что у `JournalReadsByIncreasingOffset`, а кейсы идут последовательно. Ложного красного на Redpanda
     не ожидаю. Прогон — с T-394.
4. **Мутанты** — в копии рабочей папки в scratch (`mktemp -d`, `git ls-files -co --exclude-standard` без
   `services/`, без `-overlay`). Прогон: `go test -short -count=1 ./shared/testkit/contract/ ./shared/testkit/state/`.
   Файл восстанавливался после каждого мутанта, в конце — `cmp` с рабочей папкой. Копия удалена по сохранённому пути.

   | # | Мутант (`membus.go`, если не сказано иное) | Результат |
   |---|---|---|
   | q9 | тождественный | оба пакета ok |
   | q0 | контроль: `func broken( {` в конце файла | красный: сборка обоих пакетов |
   | q3 | = M3: новая группа начинает за 3 записи до конца (флаг `started` в `group`, сдвиг курсора в `Subscribe`) | **красный только `ANewGroupStartsAtTheFirstOffset`** («began at offset 17, but player_events begins at 0»); остальные кейсы набора и `testkit/state` зелёные — M3 исполнителя подтверждён |
   | q4 | то же, за 5 записей до конца | красный только новый кейс («began at offset 15») |
   | o1 | `contract.go`: кейс первым в наборе | зелёный |
   | o2 | `contract.go`: кейс перед `Close`, после всех `Append` | зелёный |

5. **Прогоны** (go1.26.8 windows/amd64, golangci-lint 2.13.2). `go build ./... && go vet ./... && go vet -tags
   integration ./shared/testkit/contract/ && go vet -tags e2e ./test/...` → 0. `gofmt -l shared cmd test` — пусто.
   `go test -short -count=1 ./...` → 27 пакетов ok. `golangci-lint run ./...` → 0 issues, depguard чист: у `state` в
   тесте ушли только `reflect` и `sync`, у `contract.go` импорты прежние. `-race` недоступен (нет cgo).

### Замечания

| # | Серьёзность | Файл:строка | Что не так | Как исправить |
|---|---|---|---|---|
| N-1 | Nit | `shared/testkit/state/state.go:233-234` | «Start subscribes to system_events and announces the end of recovery, in that order». `Subscribe` теперь зовётся из горутины, и сигнал вполне может уйти раньше, чем она дойдёт до шины. Слова «in that order» обещают порядок, которого код не даёт. Абзац ниже («a habit rather than a safeguard») это смягчает, но первая строка doc-комментария читается как гарантия | «Start starts the subscription to system_events and announces the end of recovery without waiting for the subscription to join its group» — и убрать «the order above» во втором абзаце |
| N-2 | Nit | `shared/testkit/contract/contract.go:470` | `ReadRange` от 0 идёт под `t.Context()` без срока. Если брокер завис, кейс стоит весь `-timeout` прогона, а не `Timeout`, — ровно то, от чего `subscription.stop` защищён (ревью #3 T-014, Nit-1) и от чего `JournalStopsAtTheEndOfTheJournal` берёт `WithTimeout`. У `JournalReadsByIncreasingOffset` та же привычка, но новый кейс единственный читает весь топик | `ctx, cancel := context.WithTimeout(t.Context(), Timeout); defer cancel()` и передать `ctx` в `ReadRange` |
| N-3 | Nit | `tasks/T-406.md:62`, `:66` | Бэклог п. 1: «несколько сотен событий прежних кейсов» — на свежем брокере их 17. Риски: «поведение `SetOffset(0)` … не проверялось» — оно определено `kafka-go` `reader.go:1393-1397`: чтение ниже начала лога перескакивает на первый офсет, и `first == begins` держится. Риск закрыт, а в карточке он записан открытым | Исполнителю при следующей правке карточки: «17 событий + 3 своих» и «`SetOffset(0)` ниже начала лога kafka-go переводит на первый офсет (`reader.go:1393-1397`); `first == begins` держится и после retention» |

### Предложения в бэклог

1. П. 2 бэклога карточки (ловить `auto.offset.reset` в линтере композиции, если такая настройка появится) —
   поддерживаю как наблюдение. Сегодня условие пересмотра ADR-022 п. 4 держит только
   `StartOffset: kafka.FirstOffset` в `kafka.go:154`, и держит его теперь новый кейс набора на Redpanda.

### Риски и допущения

- Половина набора на Redpanda с новым кейсом не прогонялась: только `go vet -tags integration`. Вывод «ложного
  красного не будет» основан на чтении `kafka.go` и `kafka-go v0.4.51`, а не на прогоне. Подтверждение — T-394.
- Вывод про retention опирается на ветку `readLoop` в `kafka-go v0.4.51`. При смене версии библиотеки его стоит
  перепроверить, но кейс сам покраснеет, если поведение разойдётся.
