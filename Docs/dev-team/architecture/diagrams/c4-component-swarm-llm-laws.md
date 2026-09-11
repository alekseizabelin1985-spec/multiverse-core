# C4 уровень 3 — рой, шлюз модели и законы (EPIC-003)

**Вопрос читателя: кто решает, что происходит в мире, кто разговаривает с моделью и что стоит между моделью и журналом?**

Дата: 2026-09-11 · system-architect#1
Иллюстрирует: [`../components/swarm-llm-laws.md`](../components/swarm-llm-laws.md) §1.1–§9, [`../contracts.md`](../contracts.md) C-05, C-06, C-07, C-11, C-12, C-15, ADR-002 (рой на одной архитектуре), ADR-005 (провайдеры), ADR-014 (две очереди), ADR-016 (парсер), ADR-017 (страж).
Проверено по дереву: `shared/agent/*.go`, `shared/testkit/swarm/*.go`, `shared/contracts/registry.go`, `services/_archive/shared/agent/`.

**Предупреждение читателю: каталогов `internal/swarm`, `internal/llm`, `internal/laws` в дереве нет.** Ниже нарисован целевой блок с явной пометкой каждого компонента; всё, что сегодня работает от его имени, вынесено в отдельный контур справа.

```mermaid
C4Component
    title core — компоненты роя, шлюза и законов; целевое слева, существующее справа

    Container_Boundary(sw, "internal/swarm — БУДУЩЕЕ") {
        Component(rt, "Runtime", "Go", "подписки на player, world, game, system events; дедуп по event.id; догон с курсора снапшота; выход из replay в live по Journal.End")
        Component(reg, "BlueprintRegistry", "Go", "загрузка blueprints/*.md, content_hash, горячая перезагрузка")
        Component(router, "Router", "Go", "спавн по trigger и scope_binding; доставка по уровням вверх и вниз; динамический родитель персонального GM")
        Component(lc, "Lifecycle", "Go", "spawn, stop, TTL; события agent.spawned, agent.stopped, agent.child_resolved, agent.spawn_rejected")
        Component(sched, "Scheduler", "Go", "две очереди: interactive и background; воркер фона уступает; тики по расписанию; бюджет вызовов в час на мир; tick.fired ДО выполнения тика")
        Component(pipe, "Pipeline + roles", "Go", "Phase 1 по правилам, Phase 2 нарратив; роли global-gm, region-gm, encounter, personal-gm, group-narrator; Emitter с белыми списками типов и типов сущностей")
        Component(ctxb, "context", "Go", "WorldView как проекция фактов; окно журнала и индекс фона как ШТАТНАЯ деградация; MemoryClient с таймаутом 500 мс")
        Component(snap, "Snapshot", "Go", "снапшот рантайма: живые агенты, расписание, бюджет, курсоры")
        Component(admin, "Admin routes", "Go", "GET /v1/admin/agents, POST /v1/admin/agents/{id}/tick, вклад в /health процесса")
    }

    Container_Boundary(llm, "internal/llm — БУДУЩЕЕ") {
        Component(gwm, "Gateway", "Go", "Generate: бюджет, провайдер и повторы, парсер, проверка языка, фильтр категории a, ЗАПИСЬ, страж - именно в этом порядке")
        Component(prov, "providers", "Go", "openai_compat по умолчанию для llama-server, ollama вторым, recorded для replay, fake для тестов, anthropic - будущее")
        Component(parser, "parser", "Go", "восстановление JSON из ответа и проверка по JSON Schema 2020-12")
        Component(filt, "filter", "Go", "фильтр категории a, fail-closed: упал - значит не пропустил")
        Component(guard, "guardian", "Go", "неизвестная сущность, посягательство на волю игрока, нарушение уровня, нарушение закона; enum причин общий со схемой llm.output.rejected")
        Component(prompt, "prompt", "Go", "секции промпта, экранирование текста игрока и фактов памяти, prompt_hash")
        Component(bud, "budget и usage", "Go", "скользящие окна по миру, уровню, фазе, провайдеру; стоимость облака")
    }

    Container_Boundary(lw, "internal/laws — БУДУЩЕЕ") {
        Component(lsvc, "Laws", "Go", "Current и Get, загрузка laws/*.yaml, подписка на world.laws.changed, счётчик напряжения, Bump")
    }

    Container_Boundary(now, "Что работает от их имени СЕГОДНЯ") {
        Component(fe, "testkit/swarm.FakeEncounter", "Go, написан", "Phase 1 боя без роя: на вход в регион создаёт встречу, на удар и бегство считает исход, публикует dice.rolled, combat.decided, один атомарный entity.update.proposed, encounter.ended")
        Component(fn, "testkit/swarm.FakeNarrator", "Go, написан", "Phase 2 на шаблонах: narrative.output вида entry на вход и осмотр, вида round на закрытие раунда. Видов turn, death, world_event НЕТ")
        Component(fc, "testkit/swarm.FakeContext", "Go, написан", "контекст роя для монтирования двойников")
        Component(ag, "shared/agent", "Go, as-is", "не типы роя, а НЕПОДКЛЮЧЁННАЯ вторая архитектура GM: router, lifecycle, pipeline, worker_pool, state_manager, md_parser, blueprint_loader, lod, tools/registry")
    }

    Component_Ext(mech, "internal/mechanics", "EPIC-002", "Resolve, NPCTarget, Seed, DiceRolledPayload, реестр инвариантов")
    ContainerQueue(bus, "Redpanda", "C-01", "читает player, world, game, system; публикует world, game, system, narrative_output, llm_records")
    ContainerDb(minio, "MinIO", "objstore", "snapshots-{world}/swarm, prompts-{world} по флагу")
    Container_Ext(llama, "llama-server", "нативно, 127.0.0.1:1234", "chat/completions с грамматикой по JSON-схеме, embeddings, models, health")
    Container_Ext(memc, "memory", "HTTP C-09", "контекст области и сводка отсутствия")

    Rel(rt, router, "событие прошло дедуп")
    Rel(router, lc, "ensure или stop экземпляра агента")
    Rel(router, pipe, "HandleEvent для экземпляра")
    Rel(sched, pipe, "RunTick и задания модели")
    Rel(pipe, mech, "Resolve и NPCTarget - Go-вызов, не шина")
    Rel(pipe, lsvc, "Current для laws_version в промпте")
    Rel(pipe, ctxb, "сборка контекста агента")
    Rel(pipe, gwm, "Generate")
    Rel(ctxb, memc, "контекст, при ошибке - окно журнала")
    Rel(gwm, bud, "разрешение и учёт")
    Rel(gwm, prov, "запрос к модели")
    Rel(gwm, parser, "разбор и проверка по схеме")
    Rel(gwm, filt, "проверка текста")
    Rel(gwm, guard, "проверка элементов ответа")
    Rel(gwm, prompt, "сборка промпта и его хэш")
    Rel(guard, lsvc, "актуальность версии законов")
    Rel(prov, llama, "HTTP")
    Rel(gwm, bus, "llm.output ДО использования ответа, llm.output.rejected, content.incident.recorded")
    Rel(reg, ag, "ЦЕЛЕВОЕ: типы, разбор блупринта, валидатор, реестр уровней")
    Rel(snap, minio, "снапшот роя и указатель")
    Rel(rt, bus, "Subscribe и Publish")
    Rel(fe, mech, "Resolve через интерфейс, сегодня подменён FixedMechanics")
    Rel(fe, bus, "публикует типы подменяемого контракта, source = testkit/swarm")
    Rel(fn, bus, "narrative.output")
```

## Порядок в шлюзе, который эту диаграмму и делает полезной

бюджет → провайдер и повторы → парсер → язык → фильтр → **запись `llm.output`** → страж. Запись идёт **до** того, как ответ использован: иначе replay не воспроизведёт решение. При карантине фильтра и при падении фильтра `response_raw` не пишется вовсе.

## Что здесь будущее

Всё, кроме контура «Что работает от их имени сегодня». Ни одного файла `internal/swarm`, `internal/llm`, `internal/laws` в дереве нет. Нет и данных, на которых они должны работать: каталогов `blueprints/`, `laws/`, `schemas/agent/`, `config/absolute-limits.yaml`.

Зарегистрированы, но не имеют издателя (помечены `Reserved` в реестре): `world.law_breach.proposed | rejected | applied | review_decided | rolled_back` — механика пробоя это эпик E-B, в MVP-1 живёт только контракт.

## Расхождения с деревом

**Сверка T-409 (2026-09-11, architect#1).** «Закрыто» — документ приведён к дереву; «код» — прав контракт, названа задача; «открыто» — оставлено намеренно.

| # | Статус | Что сделано |
|---|---|---|
| 1 | закрыто | `swarm-llm-laws.md` §2 и шапка: раскладка `shared/agent` помечена как целевая, as-is состав перечислен, переписывание — T-201/T-202 |
| 2 | закрыто | ADR-025: истина — `shared/contracts/ownership.go`, тест равенства не нужен; `levels.go` — производный вид (C-02 v1.4, §16 п. 6) |
| 3 | закрыто (T-220; сверка T-416) | `FakeNarrator` отвечает на все шесть поводов C-05 — шесть поводов на пять видов, `entry` покрывает вход и осмотр; соло-удар порождает `turn`, смерть — `death`, начало встречи — `world_event`. C-05 v1.4 «Заглушка» переписан по T-220; явный признак конца обмена (`combat.decided.exchange`) и порядок смерти после удара — C-05 v1.4 п. 7–8 |
| 4 | закрыто | `swarm-llm-laws.md` C4 §1.1, §2, §9.1, §14: `openai_compat` — провайдер по умолчанию |
| 5 | закрыто | `foundation.md` §1: правило «`shared/*` не импортирует `internal/*`» записано с двумя именованными исключениями линтера |

Формулировки, записанные при рисовании (до сверки):

1. **`shared/agent` — это не то, что описывает документ.** `components/swarm-llm-laws.md` §2 перечисляет целевые файлы `types.go`, `blueprint.go`, `parser.go`, `validator.go`, `levels.go`, `placeholders.go` и список удаляемых. В дереве ровно наоборот: все «удаляемые» на месте (`router.go`, `lifecycle.go`, `pipeline.go`, `worker_pool.go`, `state_manager.go`, `md_parser.go`, `blueprint_loader.go`, `helpers.go`, `interfaces.go`), ни одного целевого нет. В архив (`services/_archive/shared/agent/`) уехали только `filter.go`, `tools/*_tool.go`, `adapter.go` и e2e-тест.
2. **`shared/agent/levels.go` отсутствует**, хотя `contracts.md` §16 п. 6 объявляет его единственной истиной таблицы владения, а тест равенства с `shared/contracts/ownership.go` — блокирующим merge. Сегодня блокировать нечего.
3. **`FakeNarrator` v0 уже́ контракта.** `contracts.md` C-05 «Заглушка» перечисляет шесть поводов для нарратива: `player.entered_region`, `player.looked`, `encounter.started`, последний `combat.decided` цикла, `entity.updated status=dead`, `round.closed`. В коде таблица `narrated` знает три: вход, осмотр, закрытие раунда. Практическое следствие видно на [`seq-combat-solo.md`](seq-combat-solo.md): **соло-удар сегодня не порождает никакого нарратива**, и e2e это не ловит, потому что ожидания выводятся из той же таблицы.
4. **Провайдер `openai_compat` в документе помечен как «E-H, компилируется, включается флагом»** (`components/swarm-llm-laws.md` §2), а по U-8 и C-15 v1.1 он основной. Документ отстал от решения; в дереве нет ни того, ни другого.
5. **Двойник встречи знает про механику больше, чем ему полагается по границам.** Импорт `internal/mechanics` из `shared/**` разрешён именно двум подпакетам (`shared/testkit/mechanics`, `shared/testkit/swarm`) именованными правилами `.golangci.yml`. Это осознанное исключение — двойник обязан говорить типами C-03, — но правило зависимостей `shared/* не импортирует internal/*` из `components/foundation.md` §1 записано без него.
