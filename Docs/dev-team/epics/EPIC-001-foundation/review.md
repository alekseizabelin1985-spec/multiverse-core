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
