# Фикстуры мира «Тёмный лес»

Шесть сущностей MVP-1 и указатель на снапшот seq 0 — единственный способ
создать мир на MVP-1 (`architecture/components/state-and-mechanics.md` §4.10,
решение G2). Отсюда читают `internal/state.Bootstrap`,
`mvctl world init --fixtures`, `testkit/state.FakeState.Seed` и
`testkit/gateway.Harness`.

Владение: EPIC-001 создаёт (T-016), дальше файлы общие
(`plan/ownership.md` §1): **добавлять** сущности может любая команда, **менять
существующие** — с уведомлением tech-lead#1. Сверку выполняет
`test/fixtures/fixtures_test.go`; после любой правки прогнать
`go test ./test/...` и `mvctl privacy scan testdata/`.

## Формат

| Файл | Содержимое |
|---|---|
| `world.json` | `dark-forest-world` |
| `region.json` | `dark-forest-01` «Тёмный лес» |
| `npc.json` | `wolf-alpha` «Альфа-волк» |
| `players.json` | `player-A`, `player-B`, `player-C` |
| `snapshots/state/latest.json` | указатель на снапшот seq 0 (C-14 v1.1, §4.4) |
| `snapshots/state/20260101T000000Z-000000.json` | сам объект снапшота, на который смотрит указатель |

Каждый файл сущностей — **массив** объектов `shared/entity.Entity` (одна форма
в памяти, на диске и внутри снапшота), даже если сущность одна. Порядок файлов
= порядок создания: мир → регион → NPC → игроки.

Сущность в фикстуре — это состояние сразу после принятого
`entity.create.proposed`: `version: 1`, `created_at == updated_at`, без
`last_event_id`, `last_change` и `history` (их пишет State, когда публикует
факт). `scope` хранится объектом `{id, type}`, а не сокращением `solo:{id}`.

## Откуда берутся числа

- `hp`, `hp_max`, `atk`, `def`, `dmg`, `flee` и таблица `loot` — из
  `rules/dark-forest.yaml` (`mechanics.Rules.Stats`, `Rules.Loot`). В фикстурах
  они продублированы намеренно: сущность — истина о состоянии, правила — истина
  о формулах (§4.10). Правится файл правил → правятся фикстуры, иначе тест
  красный.
- `npc_ids`, `players_present` региона — проекции позиций сущностей.
- `state_hash`, `size_bytes`, `entities_count`, `applied_proposals` указателя —
  вычисляются из самих сущностей и файла снапшота; тест пересчитывает их.
- Идентификаторы, имена, погода, `respawn_ttl: 24h`, `encounter_chance` —
  авторский контент. `encounter_chance = 0.25` — предварительное значение:
  владелец числа — блупринт `domain-dark-forest` (EPIC-003, `encounter.chance`),
  тест проверяет только, что это вероятность в `(0, 1]`.

## Приватность

Персонажи `player-A/B/C` — сессии `actor_kind: ci`. Внешних идентификаторов,
имён пользователей и адресов в этом дереве быть не должно: проверяет
`mvctl privacy scan testdata/` (job `security` в CI, SEC-01, SEC-23, NFR-041).
