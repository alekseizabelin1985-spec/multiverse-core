# Runbook — эксплуатация Multiverse-Core

Источник: `Docs/dev-team/architecture/infrastructure.md` §9 (сведено к фактическому
состоянию репозитория на момент EPIC-001 «Фундамент», подволна 0.7). Аудитория —
оператор стенда (сейчас — владелец проекта, единственная машина: Windows 11,
Docker Desktop, GPU для LLM).

> **Стадия проекта.** Контексты `state`, `mechanics`, `swarm`, `llm`, `laws`,
> `gateway`, `memory` сейчас — заглушки (`cmd/multiverse/contexts.go`), отвечающие
> `/health: ok` без домена. Команды ниже поднимают и проверяют инфраструктуру и
> процессы платформы; игровых сценариев (`mvctl world init`, сессии игрока) на этом
> этапе ещё нет — они появляются по мере эпиков EPIC-002…EPIC-005. Команды `mvctl`
> и `multiverse db`, которых в этом разделе нет (`world`, `blueprint`, `laws`,
> `record`, `golden`, `llm`, `memory`, `report`, `trace`, `db backup|check`),
> зарезервированы в реестре (`cmd/mvctl/main.go`, `cmd/multiverse/db.go`) и
> возвращают код ошибки с именем владеющего эпика — это ожидаемое поведение, не
> баг.

> **Порядок ниже не прогнан целиком.** GNU make не установлен ни на одной
> машине, где шла разработка T-001…T-398 — ни один участник ни разу не
> выполнил ни одной цели `make` по-настоящему; правки `Makefile` проверялись
> построчной эмуляцией оболочки (см. `Docs/dev-team/journal.md`, записи по
> T-397). CI цели `make` тоже не вызывает. Владелец — первый, кто прогоняет
> этот раздел на реальной чистой машине: установить `make` (README, раздел
> «Требования»), затем `make -n up`, `make compose-lint`, и только потом
> настоящий `make up`.

## 1. Запуск с нуля (после клонирования)

1. `cp .env.example .env`; заполнить переменные, отмеченные `[required]` в
   комментарии над ними в `.env.example` — это обязательный минимум для
   активного набора профилей (`COMPOSE_PROFILES` в примере — `memory`).
   **Пустую переменную комментировать только строкой выше, никогда после `=`**:
   dotenv-парсер `docker compose` обрезает хвостовой комментарий лишь у
   НЕПУСТОГО значения, а у пустого берёт его текстом значения — переменная
   перестаёт быть пустой, `${VAR:?}` молчит, и сервис стартует с мусорным
   значением, хотя `make` (`set -a; . ./.env`) читает ту же строку как пустую
   (T-397). Оставить `COMPOSE_PROFILES=memory` — профиль `bot` пока не
   запускается (бинарник `cmd/telegram-bot` появится в EPIC-004); про профили
   `bot`/`legacy` — ниже.
2. **LLM (опционально).** Проверить в `.env`: `MV_LLM_PROVIDER=openai_compat`,
   `MV_LLM_URL`, `MV_LLM_BIN`, `MV_LLM_MODEL_FILE`, `MV_LLM_MODELS_DIR`; файл
   модели (`.gguf`) должен лежать по указанному пути. `make llm-up` → дождаться
   `/health = 200` → `make llm-health` (печатает `/v1/models` и билд). Без LLM
   платформа тоже стартует — нарратив деградирует (`/health.llm = unavailable`,
   FR-080), это не авария.
3. `make minio-image` (первый раз ≈ 5 минут) → `make up` (поднимает
   `redpanda`, `minio`, `gateway`, `core` плюс профили из `.env`; init-контейнеры
   создают топики и бакеты) → `make health`.
4. Проверка вручную: `curl http://127.0.0.1:8088/health` (gateway),
   `curl http://127.0.0.1:8082/health` (memory, если профиль `memory` включён).
   Порт `core` (`:8090`) наружу не публикуется — проверяется изнутри контейнера:
   `docker compose exec core /multiverse health --url http://127.0.0.1:8090/health`.

### Профили

Профили `bot` и `legacy` описаны в собственных compose-файлах
(`docker-compose.bot.yml`, `docker-compose.legacy.yml`) и подключаются
Makefile'ом, только когда профиль реально запрошен: `make up PROFILES=bot`,
`make up PROFILES=memory,legacy` или `COMPOSE_PROFILES=...` в `.env` (Makefile
сам добавляет `-f` нужного файла по активному набору). Причина — `docker
compose` интерполирует файл целиком ДО отбора по профилям, и обязательные
переменные этих сервисов (токен бота, образ Chroma) раньше валили `make up`
всем подряд, включая тех, кто эти профили не поднимает (T-397). **Голый
`docker compose --profile bot up` без Makefile эти файлы не видит и
официально не поддерживается — он молча поднимет стек без бота**; это
решение оркестратора, а не пробел. `make up PROFILES=legacy` сам сначала
выполняет `legacy-src` (цель `up` объявляет её своей предпосылкой, срабатывает,
как только `legacy` попал в активный набор) — она делает `rm -rf
build/.legacy-src` и `git archive` из `LEGACY_SRC_REF`, поэтому первый запуск
профиля дольше обычного; отдельно вызывать `make legacy-src` не нужно. Профиль
`bot` сегодня всё равно не запускает контейнер — `cmd/telegram-bot` появится в
EPIC-004.

## 2. Остановка / перезапуск

- `make down` — останавливает контейнеры, тома сохраняются; это единственная
  цель остановки для любого набора профилей — она подключает тот же набор
  compose-файлов, что и `make up` (см. «Профили» выше). Если стек поднимали с
  `PROFILES=...`, передайте то же значение и в `make down` (либо держите
  `COMPOSE_PROFILES` в `.env` неизменным между `up` и `down`), иначе Makefile
  не подключит файл профиля и не остановит его сервисы.
- Перезапуск одного процесса: `docker compose restart core` (или `gateway`,
  `memory`).
- `make reset` — **удаляет тома**; требует свежего `make backup` и подтверждения
  вводом `yes`.

## 3. LLM: старт, проверка, смена модели

**Старт.** `make llm-up` (single-режим) или `make llm-up ROUTER=1` (router,
несколько моделей). Скрипт (`scripts/llm-server.ps1` на Windows,
`scripts/llm-server.sh` в WSL/Linux — Makefile сам выбирает по наличию `pwsh`)
пишет PID в `ops/llm-server.pid` и ждёт `GET $MV_LLM_URL/health = 200` (до 180 с;
`503 Loading model` по пути — норма).

**Проверка.** `make llm-health` — код `/health`, список `GET /v1/models`, билд
llama.cpp, `nvidia-smi` (used/total VRAM).

**Стоп.** `make llm-down` (по PID-файлу `ops/llm-server.pid`).

**Смена модели.** Положить `.gguf` в `MV_LLM_MODELS_DIR` → `make llm-down` →
поправить `MV_LLM_MODEL_FILE` в `.env` (single-режим) или включить `ROUTER=1`
(модель тогда выбирается полем `model` в блупринте, когда блупринты появятся,
EPIC-003) → `make llm-up` → `make llm-health` (новое имя должно быть в
`/v1/models`) → обновить `LLM_MODEL_DEFAULT` в `build/versions.env`.

**Обновление билда llama.cpp.** Отдельной задачей: скачать релиз → `make llm-down`
→ заменить бинарник (старый сохранить) → `llama-server --version` → обновить
`LLAMACPP_BUILD` в `build/versions.env` → `make llm-up` → `make llm-health` →
`make test-e2e` (золотой прогон на записях, когда они появятся, EPIC-003) — при
регрессии вернуть прежний бинарник.

**Диагностика:**

| Симптом | Что смотреть |
|---|---|
| `loading` дольше 3 мин | размер модели vs VRAM (`nvidia-smi`); хватает ли RAM при `mlock` |
| `unavailable` с хоста | процесс жив? `ops/llm-server.pid`, `curl http://127.0.0.1:1234/health` |
| `unavailable` только из контейнера | `host.docker.internal` (у `core` есть `extra_hosts: host.docker.internal:host-gateway`), правило файрвола Windows для `llama-server.exe` |
| `degraded(model_not_resident)` | `GET /v1/models` vs ожидаемое имя модели |

Логи — `ops/llm-server.log` (перезаписывается при рестарте; `--verbose` не
включаем).

## 4. Резервное копирование и восстановление

Реализовано целями Makefile (`build/versions.env` задаёт образ `ALPINE_IMAGE` для
служебных контейнеров):

```bash
make backup                 # тома minio-data и redpanda-data -> $BACKUP_DIR (по умолчанию ~/multiverse-backups)
make restore FILE=<архив>   # восстановить один из них: make restore FILE=minio-<date>.tgz
make archive-legacy         # разово: as-is тома (minio_data, redpanda_data) -> backups/legacy-<date>/
```

`make backup` останавливает `core`/`redpanda` на время снятия архива каждого тома
и запускает их обратно; бакет `prompts-*` исключается из бэкапа MinIO намеренно
(содержит тексты игроков, отдельный жизненный цикл ILM 30 дней). `make reset`
отказывается работать без свежего архива в `$BACKUP_DIR`.

**Восстановление после падения (тома целы):** `make up` → `make health`; в логе
`core` — попытка восстановления по журналу (контексты пока заглушки, поэтому
сейчас это просто повторный старт без данных для восстановления).

**Восстановление на чистой машине:** установить Docker Desktop, `git clone`,
`.env` из менеджера паролей владельца → `make minio-image` → `make restore
FILE=minio-<date>.tgz` → `make restore FILE=redpanda-<date>.tgz` → `make llm-up`
(если нужен) → `make up` → `make health`.

**Онлайн-бэкап SQLite (`links.db`/`gateway.db`) и восстановление памяти
(Qdrant/Neo4j) через `mvctl`/`multiverse db`** — реализуются вместе с
`internal/gateway` (EPIC-004) и `internal/memory` (EPIC-005); сейчас
`multiverse db backup|check` возвращает ошибку «нет базы данных в этом процессе»
— это ожидаемо, компонентов ещё нет.

## 5. Обновление версий инфраструктуры

Одна версия за раз: изменить пин в **`build/versions.env`** (единственное место —
его читают `docker-compose.yml`, `Makefile`, `shared/testkit.Versions()`) →
`make backup` → `docker compose pull <сервис>` (или `make minio-image` для MinIO)
→ `docker compose up -d <сервис>` → `make health` → `make test-integration`.
llama.cpp — по процедуре раздела 3 «Обновление билда» (сервер вне compose,
`docker compose pull` его не касается). Ежемесячно — сверять `build/versions.env`
с Docker Hub и релизами `ggml-org/llama.cpp` вручную (Dependabot этот файл не
видит).

## 6. Токен Telegram-бота

Не применимо на текущем этапе — `cmd/telegram-bot` ещё не реализован (EPIC-004).
Когда бот появится: `@BotFather` → `/revoke` → обновить `MV_TELEGRAM_BOT_TOKEN`
в `.env`.

**`PROFILES=` замещает активный набор, а не дополняет его** (переменная `ACTIVE_PROFILES` в `Makefile`:
`ACTIVE_PROFILES := $(if $(PROFILES),$(PROFILES),…)`). Оператор с
`COMPOSE_PROFILES=memory` в `.env` НЕ должен свести стек командой `make up
PROFILES=bot` — так `up` подключит другой набор compose-файлов, чем текущий
`down`, и правило раздела 2 («тот же `PROFILES=` у `up` и `down`») будет
нарушено. Правильно:

- свести весь стек с добавленным профилем бота — `make up
  PROFILES=memory,bot` (перечислить весь обычный набор оператора + `bot`);
- либо не трогать остальной стек и поднять только контейнер бота — прямой
  вызов compose с обоими файлами: `docker compose -f docker-compose.yml -f
  docker-compose.bot.yml up -d telegram-bot` (сервис `telegram-bot` живёт
  только в `docker-compose.bot.yml`, `docker compose` без `-f` на этот файл
  не видит).

Проверка: `docker compose -f docker-compose.yml -f docker-compose.bot.yml
logs --tail=50 telegram-bot` (токен в логе должен быть отредактирован).

## 7. Ежедневный / еженедельный чек оператора

- Перед сессией (если нужен LLM): `make llm-up` — процесс не служба, после
  перезагрузки Windows сам не поднимется.
- Регулярно: `make health` (ненулевой код — есть проблема); `make backup`
  (еженедельно или перед важными изменениями); `docker system df`.
- Ежемесячно: сверить `build/versions.env` с реестрами образов и релизами
  llama.cpp (раздел 5).

Автоматизированные отчёты (`mvctl report`, `mvctl llm`, инциденты
в `ops/metrics/incidents.csv`) появляются вместе с EPIC-005 (005-ops); сейчас
`ops/metrics/` содержит только заготовки замера LLM из задачи F-8: `README.md`,
`bench-matrix.json` и `baseline.md` (статус — ШАБЛОН, заполняется после прогона на стенде).

## 8. Карточки процессов

| | `gateway` | `core` | `memory` (профиль `memory`) |
|---|---|---|---|
| Команда | `/multiverse --contexts=gateway` | `/multiverse --contexts=state,mechanics,laws,llm,swarm` | `/multiverse --contexts=memory` |
| Порт / health | `127.0.0.1:8088/health` | `127.0.0.1:8090/health` (не публикуется наружу; проверка изнутри контейнера) | `127.0.0.1:8082/health` |
| Данные сейчас | нет (том `gateway-data` зарезервирован, `internal/gateway` ещё не реализован) | нет домена (заглушка); при появлении `internal/state` — MinIO (`entities-*`, `snapshots-*`) | нет домена (заглушка); при появлении `internal/memory` — Qdrant/Neo4j |
| Старт / стоп | `docker compose up -d gateway` / `stop gateway` | `docker compose restart core` | `docker compose up -d memory` (требует профиль `memory`) |
| Логи | `make logs SERVICE=gateway` | `make logs SERVICE=core` | `make logs SERVICE=memory` |
| Типичный сбой | контейнер не стартует → проверить `.env` (обязательные `MV_MINIO_*`) | `llm=unavailable` → раздел 3 | `qdrant`/`neo4j` не healthy → `docker compose logs qdrant neo4j` |

Пятая карточка — **`llama-server`** (нативный процесс, не сервис compose):

| | `llama-server` |
|---|---|
| Команда | `make llm-up` (см. раздел 3) |
| Порт / health | `127.0.0.1:1234`: `GET /health` (200 `ok` / 503 `loading`), `GET /v1/models` |
| Данные | файл модели `.gguf` в `MV_LLM_MODELS_DIR` |
| Старт / стоп | `make llm-up` / `make llm-down`; `make up`/`make down` его не трогают |
| Логи | `ops/llm-server.log` |
| Секреты | нет — локальный сервер работает без `--api-key` |
