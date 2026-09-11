# 🧩 GM Configuration Specification

> Все GM — одна и та же бинарная реализация. Поведение задаётся **исключительно параметрами запуска и конфигурационным файлом**.  
> **Стиль повествования НЕ задаётся в конфигурации — это прерогатива LLM.**

---

## 🔧 Параметры запуска

| Параметр | Обязательный | Пример |
|---------|--------------|--------|
| `--scope_id` | ✅ | `player:123` |
| `--config` | ✅ | `gm_config_player.yaml` |
| `--event_bus` | ❌ | `localhost:9092` |
| `--minio_endpoint` | ❌ | `http://minio:9000` |

---

## 📁 Конфигурационный файл (YAML)

    # Обязательные поля
    scope_type: "player"
    focus_entities: ["{{.player_id}}"]

    # Контекстное окно
    time_window: "2m"
    context_depth:
      canon: 1
      history: 3
      entities: 2

    # Что включать в промт
    include:
      world_facts: false
      entity_emotions: true
      location_details: true
      temporal_context: true

    # Поведение обработки
    triggers:
      time_interval_ms: 10000
      max_events: 50
      narrative_triggers:
        - "combat.start"
        - "player.entered_boss_room"

    # Снапшоты
    snapshot:
      interval_events: 10
      interval_ms: 30000
      minio_path: "gnue/gm-snapshots/v1"

> ⚠️ **Поле `narrative_style` удалено**. GM не указывает стиль — только предоставляет факты.

---

## 🧪 Рекомендуемые профили

| Профиль | `scope_type` | `focus_entities` | `include.world_facts` |
|--------|--------------|------------------|------------------------|
| **GM: World** | `world` | `[]` | `true` |
| **GM: Region** | `region` | `[]` | `true` |
| **GM: Player** | `player` | `["{{.player_id}}"]` | `false` |
