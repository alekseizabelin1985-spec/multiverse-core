# C4 уровень 3 — вход игрока и бот (EPIC-004)

**Вопрос читателя: через что проходит действие игрока до шины, что при этом хранится и где именно лежат персональные данные?**

Дата: 2026-09-11 · system-architect#1
Иллюстрирует: [`../components/gateway-and-bot.md`](../components/gateway-and-bot.md) §2, §4, §7–§10, [`../contracts.md`](../contracts.md) C-04, C-08, C-10, C-14, ADR-006 (доставка), ADR-009 (персональные данные), ADR-018 (библиотека бота), ADR-019 (SQLite и миграции), ADR-020 (координатор раундов).
Проверено по дереву: `cmd/` (каталога `telegram-bot` нет), `internal/` (каталога `gateway` нет), `docker-compose.bot.yml`, `shared/testkit/gateway/`.

**Предупреждение читателю: в дереве нет ни `internal/gateway`, ни `cmd/telegram-bot`.** Диаграмма показывает целевую форму блока и явно называет то единственное, что существует, — двойник `shared/testkit/gateway`.

```mermaid
C4Component
    title gateway и telegram-bot — компоненты; в дереве существует только двойник

    Container_Boundary(bot, "cmd/telegram-bot — БУДУЩЕЕ") {
        Component(poll, "poller", "Go, go-telegram/bot", "long polling с таймаутом 25 с; только личные чаты; allowlist Telegram user id ДО любого вызова gateway")
        Component(cmds, "commands", "Go", "разбор словаря команд, клавиатуры, уведомление об ИИ и 18+, предупреждение об облаке раз за сессию диалога")
        Component(deliv, "delivery", "Go", "long-poll доставок и ack; отправка plain text без parse_mode")
    }

    Container_Boundary(gw, "internal/gateway — БУДУЩЕЕ") {
        Component(api, "api", "Go, net/http ServeMux", "маршруты v1, DTO, коды ошибок, middleware: список клиентов, actor_kind, лимит тела 64 КиБ, rate limit, JSON-ошибки, request_id")
        Component(links, "links", "Go + links.db", "ЕДИНСТВЕННОЕ место с внешним идентификатором: Link, суррогат link_id, согласие, привязка персонажа, forget с немедленным checkpoint и vacuum, маршрут доставки по player_id")
        Component(actions, "actions", "Go + таблица ключей", "словарь и предусловия, идемпотентность action_key, rate limit 30 в минуту, публикация player.* и entity.update.proposed")
        Component(groups, "groups", "Go", "создание, вход, выход, правила лидера, group.*, атомарные предложения")
        Component(rounds, "rounds", "Go + таблица раундов", "координатор раунда группы: открытие, таймаут через Clock, авто-защита молчавших, idle, явное закрытие для ci")
        Component(session, "session", "Go + таблица сессий", "сессии области, простой 30 минут, парные analytics.session.*")
        Component(turns, "turns", "Go + таблица ходов", "жизненный цикл хода до нарратива или таймаута, analytics.turn.completed")
        Component(rm, "readmodel", "Go, в памяти", "проекция мира из entity.created и entity.updated; старт от snapshots-{world}/state/latest.json; ожидание фактов по correlation_id")
        Component(outbox, "outbox", "Go + таблица доставок", "Delivery: постановка из narrative.output, combat.decided, entity.updated, group.*; лизинг и ack; голова очереди на игрока; повтор через 30 с; срок 24 часа; сброс при forget")
        Component(cons, "consumer", "Go, eventbus", "подписки на system, game, world events и narrative_output; дедуп по event.id; курсоры; диспетчер")
        Component(snap, "snapshot", "Go, objstore", "snapshot.created component=gateway: курсоры и хэш проекции")
        Component(st, "store", "Go, modernc.org/sqlite + goose", "две базы, PRAGMA, миграции из встроенной файловой системы, без Down")
        Component(cl, "client", "Go", "клиент C-08, один на бота и на харнесс CI")
    }

    Container_Boundary(now, "Что существует сегодня") {
        Component(harn, "testkit/gateway.Harness", "Go, написан", "издатель player.* и entity.*.proposed от имени фикстурных персонажей; ждёт факт по каждой названной сущности; знает про встречу ровно столько, чтобы отличить конец боя от тишины. HTTP, сессий, раундов и идемпотентности НЕТ")
        Component(scen, "testkit/gateway.Scenario", "Go, написан", "сценарии visit и skirmish: шаги, условия, адаптивная длина боя")
    }

    ContainerDb(ldb, "links.db", "SQLite, отдельный файл, права 0600 — БУДУЩЕЕ", "Link и CharacterRequest; отдельная политика бэкапа; физическое удаление по forget")
    ContainerDb(gdb, "gateway.db", "SQLite WAL — БУДУЩЕЕ", "сессии, ходы, раунды, ключи идемпотентности, доставки, курсоры; НИ ОДНОГО поля с внешним идентификатором")
    ContainerQueue(bus, "Redpanda", "C-01", "пишет player, game, system, analytics; читает system, game, world, narrative_output")
    ContainerDb(minio, "MinIO", "objstore", "чтение state/latest.json, запись snapshots-{world}/gateway")
    System_Ext(core, "core admin", "C-06, MV_CORE_URL", "обратный прокси /v1/admin/* только для ci и operator")

    Rel(poll, cmds, "обновление из Telegram")
    Rel(cmds, cl, "HTTP v1")
    Rel(deliv, cl, "long-poll доставок и ack")
    Rel(cl, api, "C-08")
    Rel(api, links, "resolve, consent, forget, привязка персонажа")
    Rel(api, actions, "POST actions")
    Rel(api, groups, "операции группы")
    Rel(api, rounds, "закрытие раунда для ci")
    Rel(api, outbox, "выдача доставок и ack")
    Rel(api, rm, "статус игрока, группа, миры")
    Rel(api, core, "прокси admin")
    Rel(actions, rm, "предусловия действия")
    Rel(actions, session, "открыть или продолжить сессию")
    Rel(actions, turns, "зарегистрировать ход")
    Rel(actions, rounds, "принять действие в области группы")
    Rel(actions, bus, "player.* и entity.*.proposed")
    Rel(groups, bus, "group.* и атомарные предложения")
    Rel(rounds, bus, "round.opened, player.defended по таймауту, round.closed")
    Rel(session, bus, "analytics.session.*")
    Rel(turns, bus, "analytics.turn.completed")
    Rel(cons, bus, "Subscribe")
    Rel(cons, rm, "применение фактов")
    Rel(cons, outbox, "постановка доставки")
    Rel(cons, turns, "отметки механики и нарратива")
    Rel(outbox, links, "маршрут по player_id ТОЛЬКО в момент выдачи")
    Rel(links, ldb, "SQL")
    Rel(outbox, gdb, "SQL")
    Rel(session, gdb, "SQL")
    Rel(turns, gdb, "SQL")
    Rel(rounds, gdb, "SQL")
    Rel(st, ldb, "миграции")
    Rel(st, gdb, "миграции")
    Rel(rm, minio, "старт проекции")
    Rel(snap, minio, "запись снапшота")
    Rel(harn, bus, "player.* и entity.*.proposed, source = testkit/gateway")
    Rel(scen, harn, "шаги сценария")
```

## Правило, ради которого блок разделён именно так

Внешний идентификатор существует ровно в двух местах: в памяти бота (`chat_id` для отправки) и в `links.db`. Компонент `outbox` узнаёт маршрут у `links` **в момент выдачи доставки** и нигде его не сохраняет. Всё остальное — шина, объектное хранилище, индексы, логи, `gateway.db` — знает только `player_id` и `link_id`. Тест NFR-041 сканирует ровно эти хранилища.

## Что здесь будущее

Всё, кроме контура «Что существует сегодня». Нет: каталога `internal/gateway` и его миграций, каталога `cmd/telegram-bot`, файлов `links.db` и `gateway.db`, спецификации `api/gateway.openapi.yaml`, клиента `internal/gateway/client`. Сервис профиля `bot` в `docker-compose.bot.yml` описан целиком и стартует в тот день, когда бинарник появится, — по замыслу, файл менять не придётся.

## Расхождения с деревом

1. **Двойник шлюза не заглушает HTTP.** `contracts.md` §17 обещает потребителям `testkit/gateway.FakeGateway` — HTTP-заглушку для разработки бота. В дереве её нет: есть `Harness`, который публикует события напрямую в шину. Разработка бота на сегодня ничем не разблокирована.
2. **Отказ действия живёт на стороне харнесса, а не встречи.** По решению оркестратора от 2026-09-11 (Critical-2 ревью T-219) харнесс сам отличает «встреча закончилась» от «никто не слушает» по событиям жизненного цикла `encounter.*`, а заглушка встречи явного отказа не публикует. Настоящий gateway унаследует этот вопрос: C-05 молчит о действии, которое встреча не принимает.
3. **`MV_GATEWAY_ADDR` объявлен, но не читается.** Процесс берёт адрес HTTP-сервера из `MV_CORE_ADDR` независимо от роли; compose дублирует туда значение. До появления `internal/gateway` это работает, но `components/gateway-and-bot.md` §11.3 называет переменную так, будто её читает код.
4. **Имя переменной списка клиентов.** Манифест `shared/env/vars.go` и compose знают `MV_GATEWAY_CLIENT_IDS`, контракт C-08 говорит `MV_GATEWAY_CLIENTS`. Прав манифест — он единственный, кого читает `mvctl env check`; правится C-08. Заодно расходятся значения по умолчанию: манифест даёт `telegram-bot,ci-harness,mvctl`, compose — `telegram-bot,mvctl`, то есть в контейнере харнесс CI по умолчанию не допущен, а при запуске без compose — допущен.
