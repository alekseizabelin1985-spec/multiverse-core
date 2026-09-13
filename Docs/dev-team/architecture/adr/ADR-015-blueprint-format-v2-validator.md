# ADR-015: Формат блупринта v2 (frontmatter + секции) и единый валидатор с окружением

Статус: предложено (к ревью system-architect#1) · Дата: 2026-09-09 · Автор: architect#2 (TEAM-2, EPIC-003) · Уровень: реализация блока
Связи: ADR-002 п. 6, ADR-007 п. 2; C-11 (`api-contracts.md` §3.1–3.3 + расширение `group-narrator`); FR-090…FR-092, FR-120, FR-124, NFR-071, NFR-083; UC-029; `components/swarm-llm-laws.md` §13.

## Контекст

As-is `shared/agent`: `AgentBlueprint` без `level/role/scope_binding/allowed_event_types/...`; `md_parser.go` ищет YAML внутри ```` ```yaml ```` и умеет frontmatter, но промпты вытаскивает из код-блоков `phase1_prompt`; `blueprint_validator.go` требует `tools` (≥ 1) и `llm.model`, знает типы `game-master|narrator|...`, не проверяет ссылки; `examples/domain-dark-forest.md` — чистый YAML c `qwen:7b`. Контракт C-11 задаёт поля и правила §3.2; формат парсера и способ проверки ссылок (реестр типов, файлы, инструменты) — за архитектором блока. Тот же валидатор должен работать в рантайме и в `mvctl blueprint validate` (EPIC-005).

## Рассмотренные варианты

| Вопрос | Вариант | Плюсы | Минусы |
|---|---|---|---|
| Формат | чистый YAML с многострочными промптами | один парсер | промпты и канон в YAML-строках плохо читаются автором; `## canon` как список неудобен |
| | **Markdown: YAML-frontmatter + секции `## system/phase1/phase2/tick/canon/description`; чистый YAML допустим** | автор пишет текст как текст; конфиг строгий; соответствует C-11 | два входных формата (один парсер с двумя фронтами) |
| Совместимость типов | новая структура `BlueprintV2` рядом со старой | не ломает мосты narrative-orchestrator | мосты удаляются (ADR-002); два типа — путаница |
| | **расширить `AgentBlueprint` на месте**, старые поля оставить, `type` — псевдоним `role` | один тип; старые тесты парсера переписываются точечно | поля as-is (`llm.model` плоский) объявляются устаревшими: `llm.model` → `llm.phase2.model` при загрузке с предупреждением |
| Проверка ссылок | валидатор читает реестр/файлы сам | автономно | `shared/agent` получил бы зависимости на `contracts` и ФС проекта |
| | **`ValidationEnv` передаётся вызывающим** (рантайм — из `contracts`/ФС/`mechanics.Invariants()`, CLI — так же) | `shared/agent` без зависимостей; один код в двух местах | вызывающий обязан собрать окружение (небольшой хелпер `agent.EnvFromProject(root)`) |
| Неизвестные ключи | игнорировать | лояльно | опечатка `alowed_event_types` молча отключает белый список |
| | **ошибка (`KnownFields(true)`)** | ловит опечатки | требует полного описания полей (есть) |
| Белые списки уровней | только в блупринтах | гибко | автор может выдать региону `player.*` |
| | **реестр уровней в `shared/agent/levels.go`, блупринт ⊂ реестра**; тот же реестр — в `contracts.OwnershipRules` (C-02) | BR-16 не обходится данными; State и страж согласованы | новая роль = правка реестра (ожидаемо: роль — код) |

## Решение

1. **Формат**: файл `*.md` = YAML-frontmatter (`---`…`---`) + секции второго уровня `## system`, `## phase1`, `## phase2`, `## tick`, `## canon` (маркированный список → `[]string`), `## description`; прочие секции — ошибка. Файл `*.yaml|*.yml` — те же поля, промпты под ключом `prompts:`. `yaml.v3` с `KnownFields(true)`. `content_hash = SHA-256(нормализованный YAML + секции)`; `blueprint_version` — из `version` (semver).
2. **Тип** `AgentBlueprint` расширяется на месте (структура — компонентный документ §13.1). Плоские `llm.model/temperature/max_tokens/schema` считаются устаревшими: при наличии переносятся в `llm.phase2` с предупреждением; `type` = псевдоним `role`.
3. **Валидатор** `Validate(bp, env) []Issue{File, Field, Reason, Severity}`; правила — §13.2 компонентного документа (14 правил: обязательные поля по уровню, согласование `level↔role↔scope_binding`, `parent`, `trigger` (`timer` ⇒ `intervals`, `event` ⇒ `event_name` матчит реестр, glob допустим), `ttl` у `global/domain` — warning, `max_instances=1`, модели на используемые фазы и запрет `qwen:7b/72b`, `allowed_event_types` ⊂ реестр ∩ уровень, `owned_entity_types` ⊂ уровень, `tools` ⊂ реестр (в MVP-1 пуст), `laws_ref/rules_ref/absolute_limits_ref` существуют, обязательные `npc_table/respawn_ttl/encounter/background_events/description` у `domain`, `budget/invariants` у `global`, плейсхолдеры из словаря, `monitor/object` — info «reserved»). `ValidationEnv{EventTypes, Blueprints, FileExists, Tools, Schemas, Invariants}` собирает вызывающий; хелпер `agent.EnvFromProject(root, contracts.Types(), invariants)`.
   **Уточнение 2026-09-13 (T-459, по коду T-202 и ADR-025):** `ValidationEnv{EventTypes, OwnedEntityTypes, Blueprints, FileExists, Tools, Schemas, Invariants, Models}`; хелпер — `agent.EnvFromProject(root, eventTypes, ownedEntityTypes, invariants, models)`. Типы событий и строки владения передаёт вызывающий: `shared/agent` не импортирует `shared/contracts`. `Issue` получает стабильный код находки `Code` — по нему рантайм понижает правило 7а до `warning` (КД §13.2, решение 1). Форма `EnvFromProject(root, contracts.Types(), invariants)` выше устарела.
4. **Реестр уровней** `shared/agent/levels.go`: `AllowedEventTypes(level, role)`, `OwnedEntityTypes(level, role)` по таблице §2.4 api-contracts / `data-model.md` §4; `monitor`/`object` присутствуют. Экспортируется в `contracts.OwnershipRules` (владелец C-02 — EPIC-002; передача — по контракту).
   **Устарело 2026-09-11 (ADR-025, T-409) в части «экспортируется»:** единственная истина таблицы владения — `shared/contracts/ownership.go`, а не `levels.go`. `levels.go` держит `AllowedEventTypes`; вид «типы сущностей уровня» валидатор получает через `ValidationEnv`, собранный вызывающим из `contracts.OwnershipRules` (так же, как реестр типов в п. 3). Почему: файла `levels.go` в дереве нет, а копия в `shared/contracts` уже существует и проверена; в таблице есть строки без агента (`gateway`, `author`, `system`); экспорт при сборке развернул бы зависимость фундамента на пакет роя.
   **Уточнение 2026-09-13 (T-459):** источник `AllowedEventTypes` — белые списки `api-contracts.md` §2.4; в `data-model.md` §4 только владение сущностями. Роль `city-gm` в MVP-1 резервная: список пуст, `info: reserved role, spawn disabled` (КД §13.2, решение 2).
5. **Загрузка**: `BlueprintRegistry.LoadDir` парсит все файлы, валидирует, невалидные не активирует (`/health degraded {blueprints}`), дубликаты `name` — ошибка. Горячая перезагрузка (I2, Should): наблюдение каталога, новая версия применяется к следующему спавну и к timer-агентам на следующем тике; `agent.blueprint_reloaded {blueprint, version_from, version_to, content_hash}`.
6. **Удаляется**: `md_parser.go`, `blueprint_loader.go`, `blueprint_validator.go` (+тесты), `helpers.go` (`DefaultBlueprintFactory`, `TTLManager`), `examples/`; тестовые блупринты — `shared/agent/testdata/blueprints/{valid,invalid}`.

## Последствия

- Позитивные: автор пишет регион как документ; ошибки валидатора — «файл, поле, причина»; BR-16 не обходится данными; один валидатор для CLI и рантайма (C-11 гарантия).
- Негативные: старые `configs/gm_*.yaml` и `examples/domain-dark-forest.md` не совместимы (переписываются вручную — 5 файлов); тесты `agent_test.go`/`blueprint_validator_test.go`/`e2e_dark_forest_test.go` переписываются.
- Что придётся сделать: `shared/agent/{blueprint,parser,validator,levels,placeholders}.go`; пять блупринтов; запрос к C-11 о glob в `trigger.event_name` и поле `immediate_broadcast[]` (компонентный документ §8.2) — в отчёте.
